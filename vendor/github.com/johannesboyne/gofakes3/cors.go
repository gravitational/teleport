package gofakes3

import (
	"net/http"
	"strings"
)

var (
	corsHeaders = []string{
		"Accept",
		"Accept-Encoding",
		"Authorization",
		"Content-Disposition",
		"Content-Length",
		"Content-Type",
		"X-Amz-Date",
		"X-Amz-User-Agent",
		"X-CSRF-Token",
		"x-amz-acl",
		"x-amz-content-sha256",
		"x-amz-meta-filename",
		"x-amz-meta-from",
		"x-amz-meta-private",
		"x-amz-meta-to",
		"x-amz-security-token",
	}
	corsHeadersString = strings.Join(corsHeaders, ", ")
)

type withCORS struct {
	r   http.Handler
	log Logger
}

func (s *withCORS) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE, HEAD")
	w.Header().Set("Access-Control-Allow-Headers", corsHeadersString)
	w.Header().Set("Access-Control-Expose-Headers", "ETag")

	if r.Method == "OPTIONS" {
		return
	}

	s.r.ServeHTTP(w, r)
}
