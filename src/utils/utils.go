package utils

import (
	"errors"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

func Timeout() time.Duration {
	envTimeout := os.Getenv("TIMEOUT")
	if len(envTimeout) != 0 {
		etimeout, _ := strconv.Atoi(envTimeout)
		return time.Duration(etimeout)
	}
	return time.Second * 10

}

func Region() string {
	envRegion := os.Getenv("REGION")
	if len(envRegion) != 0 {
		return envRegion
	}
	return "us-east-1"
}

func Port() string {
	envPort := os.Getenv("PORT")
	if len(envPort) != 0 {
		return envPort
	}
	return "8080"
}

func BucketName() (string, error) {
	envBucket := os.Getenv("BUCKET")
	if len(envBucket) != 0 {
		return envBucket, nil
	}
	return "", errors.New("No bucket name provided")
}

// Request.RemoteAddress contains port, which we want to remove i.e.:
//"[::1]:1234" => "[::1]"
func ipAddrFromRemoteAddr(s string) string {
	idx := strings.LastIndex(s, ":")
	if idx == -1 {
		return s
	}
	return s[:idx]
}

// requestGetRemoteAddress returns ip address of the client making the request,
// taking into account http proxies

func RequestGetRemoteAddress(r *http.Request) string {
	hdr := r.Header
	hdrRealIP := hdr.Get("X-Real-Ip")
	hdrForwardedFor := hdr.Get("X-Forwarded-For")
	if hdrRealIP == "" && hdrForwardedFor == "" {
		return ipAddrFromRemoteAddr(r.RemoteAddr)
	}
	if hdrForwardedFor != "" {
		// X-Forwarded-For is potentially a list of addresses separated with ","
		parts := strings.Split(hdrForwardedFor, ",")
		for i, p := range parts {
			parts[i] = strings.TrimSpace(p)
		}
		return parts[0]
	}
	return hdrRealIP
}
