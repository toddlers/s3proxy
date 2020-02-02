// Usage: curl localhost:8080/getConfig?key="<KEY_NAME>"
package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/toddlers/s3proxy/utils"
)

type s3Proxy struct {
	bucket  string
	timeout time.Duration
	port    string
	region  string
}

var (
	Trace   *log.Logger
	Info    *log.Logger
	Warning *log.Logger
	Error   *log.Logger
)

func init() {
	Trace = log.New(ioutil.Discard, "TRACE: ", log.Ldate|log.Ltime|log.Lshortfile)
	Info = log.New(os.Stdout, "INFO: ", log.Ldate|log.Ltime|log.Lshortfile)
	Warning = log.New(os.Stdout, "WARNING: ", log.Ldate|log.Ltime|log.Lshortfile)
	Error = log.New(os.Stdout, "ERROR: ", log.Ldate|log.Ltime|log.Lshortfile)
}

func check(err error) {
	if err != nil {
		if awsErr, ok := err.(awserr.Error); ok {
			// Get error details
			Error.Println("Error:", awsErr.Code(), awsErr.Message())
		}
	}
}
func getDefaults() (*s3Proxy, error) {
	return &s3Proxy{
		region:  utils.Region(),
		port:    utils.Port(),
		timeout: utils.Timeout(),
	}, nil
}

func getSession(region string) (*session.Session, error) {
	return session.NewSession(&aws.Config{Region: aws.String(region)})
}

func getObject(w http.ResponseWriter, r *http.Request, s3p *s3Proxy) {
	keyName := r.FormValue("key")

	sess, err := getSession(s3p.region)
	if err != nil {
		fmt.Fprintf(os.Stderr, "not able to create an aws sessions, %v\n", err)
		check(err)
		return
	}

	svc := s3.New(sess)

	// Create a context with a timeout that will abort the download
	// if it takes more than the passed in timeout
	ctx := context.Background()

	var cancelFn func()

	if s3p.timeout > 0 {
		ctx, cancelFn = context.WithTimeout(ctx, s3p.timeout)
	}

	// Ensure the context is canceled to prevent leaking
	defer cancelFn()

	// Downloads the object to s3. The context will interrupt the request if
	// the timeout expires

	start := time.Now().UTC()
	result, err := svc.GetObjectWithContext(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s3p.bucket),
		Key:    aws.String(keyName),
	})
	end := time.Now().UTC()
	delta := end.Sub(start)

	if err != nil {
		if aerr, ok := err.(awserr.Error); ok && aerr.Code() == request.CanceledErrorCode {
			//if the sdk can determine the request or retry delay was canceled
			// by a context the CanceledErrorCode error code will be returned
			fmt.Fprintf(os.Stderr, "download canceled due to timeout, %v\n", err)
			w.WriteHeader(http.StatusRequestTimeout)
			return
		}

		fmt.Fprintf(os.Stderr, "failed to download the object, %v\n", err)
		w.WriteHeader(http.StatusBadGateway)
		return

	}
	body, err := ioutil.ReadAll(result.Body)

	if err != nil {
		Error.Println("Error redaing http response body : ", err)
	}

	ip := utils.RequestGetRemoteAddress(r)

	contentLength := w.Header().Get("Content-Length")

	if len(contentLength) == 0 {
		contentLength = "cached"
	}
	contentLength = fmt.Sprintf("%s bytes", contentLength)

	w.Write(body)

	Info.Printf("%s - %s - %s - %s - %v - %s", start.Format(time.RFC3339), ip, r.Method, r.URL.Path, delta, contentLength)

}

func makeS3Proxy() (*s3Proxy, error) {

	bucketName, bucketError := utils.BucketName()

	if bucketError != nil {
		return nil, bucketError
	}
	Info.Println("Bucket Name : ", bucketName)
	s3p, err := getDefaults()
	if err != nil {
		return nil, err
	}
	s3p.bucket = bucketName
	return s3p, nil
}

func main() {
	s3p, err := makeS3Proxy()
	if err != nil {
		Error.Println(err)
		os.Exit(1)
	}

	Info.Println("Timeout Configured : ", s3p.timeout)
	Info.Println("AWS Region Name : ", s3p.region)

	Info.Printf("S3 Proxy Server Listening on: %s\n", s3p.port)

	http.HandleFunc("/getObject", func(w http.ResponseWriter, r *http.Request) {
		getObject(w, r, s3p)
	})

	Error.Println(http.ListenAndServe(":"+s3p.port, nil))
}
