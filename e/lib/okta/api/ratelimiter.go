package oktaapi

import (
	"net/http"

	"github.com/gravitational/trace"
	"golang.org/x/time/rate"
)

// rateLimitingHTTPTransport will only perform HTTP requests after waiting the
// for the rate limiter.
type rateLimitingHTTPTransport struct {
	delegate    *http.Transport
	rateLimiter *rate.Limiter
}

func (r *rateLimitingHTTPTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Before issuing any HTTP request, wait to ensure we only issue the number of
	// requests the rate limiter allows.
	if err := r.rateLimiter.Wait(req.Context()); err != nil {
		return nil, trace.Wrap(err)
	}

	return r.delegate.RoundTrip(req)
}

func (r *rateLimitingHTTPTransport) CloseIdleConnections() {
	r.delegate.CloseIdleConnections()
}
