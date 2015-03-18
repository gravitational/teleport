package web

import (
	"net/http"
	"strings"
	"time"

	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/codahale/lunk"
)

const (
	// HeaderEventID is the name of the HTTP header by which the root and
	// event IDs are passed along.
	HeaderEventID = "Event-ID"
)

// SetRequestEventID sets the Event-ID header on the request.
func SetRequestEventID(r *http.Request, e lunk.EventID) {
	r.Header.Set(HeaderEventID, e.String())
}

// GetRequestEventID returns the EventID for the request, nil if no Event-ID was
// provided, or an error if the value was unparseable.
func GetRequestEventID(r *http.Request) (*lunk.EventID, error) {
	s := r.Header.Get(HeaderEventID)
	if s == "" {
		return nil, nil
	}
	return lunk.ParseEventID(s)
}

var (
	// RedactedHeaders is a slice of header names whose values should be
	// entirely redacted from logs.
	RedactedHeaders = []string{"Authorization"}
)

// HTTPRequest returns an event which records various aspects of an HTTP request.
// The returned value is incomplete, and should have the response status, size,
// and the elapsed time set before being logged.
func HTTPRequest(r *http.Request) *HTTPRequestEvent {
	return &HTTPRequestEvent{
		Method:        r.Method,
		URI:           r.RequestURI,
		Proto:         r.Proto,
		Headers:       redactHeaders(r),
		Host:          r.Host,
		RemoteAddr:    r.RemoteAddr,
		ContentLength: r.ContentLength,
	}
}

// BUG(1.3): HTTPRequestEvent does not record whether the request was over TLS.
// BUG(1.3): HTTPRequestEvent does not record the identity of the TLS peer.

// HTTPRequestEvent records
type HTTPRequestEvent struct {
	Method        string            `lunk:"method"`
	URI           string            `lunk:"uri"`
	Proto         string            `lunk:"proto"`
	Headers       map[string]string `lunk:"headers"`
	Host          string            `lunk:"host"`
	RemoteAddr    string            `lunk:"remote_addr"`
	ContentLength int64             `lunk:"content_length"`
	Status        int               `lunk:"status"`
	Elapsed       time.Duration     `lunk:"elapsed"`
}

// Schema returns the constant "httprequest".
func (HTTPRequestEvent) Schema() string {
	return "httprequest"
}

var (
	redacted = []string{"REDACTED"}
)

func redactHeaders(r *http.Request) map[string]string {
	h := make(http.Header, len(r.Header)+len(r.Trailer))
	for k, v := range r.Header {
		if isRedacted(k) {
			h[k] = redacted
		} else {
			h[k] = v
		}
	}
	for k, v := range r.Trailer {
		if isRedacted(k) {
			h[k] = redacted
		} else {
			h[k] = append(h[k], v...)
		}
	}

	m := make(map[string]string, len(h))
	for k, v := range h {
		m[strings.ToLower(k)] = strings.Join(v, ",")
	}
	return m
}

func isRedacted(name string) bool {
	for _, v := range RedactedHeaders {
		if strings.EqualFold(name, v) {
			return true
		}
	}
	return false
}
