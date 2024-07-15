/*
Copyright 2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package utils

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/gravitational/trace"
	"golang.org/x/net/http/httpproxy"
)

// GetProxyURL gets the HTTP proxy address to use for a given address, if any.
func GetProxyURL(dialAddr string) *url.URL {
	addrURL, err := ParseURL(dialAddr)
	if err != nil || addrURL == nil {
		return nil
	}

	proxyFunc := httpproxy.FromEnvironment().ProxyFunc()
	if addrURL.Scheme != "" {
		proxyURL, err := proxyFunc(addrURL)
		if err != nil {
			return nil
		}
		return proxyURL
	}

	for _, scheme := range []string{"https", "http"} {
		addrURL.Scheme = scheme
		proxyURL, err := proxyFunc(addrURL)
		if err == nil && proxyURL != nil {
			return proxyURL
		}
	}

	return nil
}

// ParseURL parses an absolute URL. Unlike url.Parse, absolute URLs without a scheme are allowed.
func ParseURL(addr string) (*url.URL, error) {
	if addr == "" {
		return nil, nil
	}
	addrURL, err := url.Parse(addr)
	if err == nil && addrURL.Host != "" {
		return addrURL, nil
	}

	// url.Parse won't correctly parse an absolute URL without a scheme, so try again with a scheme.
	addrURL, err2 := url.Parse("http://" + addr)
	if err2 != nil {
		return nil, trace.NewAggregate(err, err2)
	}
	addrURL.Scheme = ""
	return addrURL, nil
}

// HTTPRoundTripper is a wrapper for http.Transport that
// - adds extra HTTP headers to all requests, and
// - downgrades requests to plain HTTP when proxy is at localhost and the wrapped http.Transport has TLSClientConfig.InsecureSkipVerify set to true.
type HTTPRoundTripper struct {
	*http.Transport
	// extraHeaders is a map of extra HTTP headers to be included in requests.
	extraHeaders map[string]string
	// isProxyHTTPLocalhost indicates that the HTTP_PROXY is at "http://localhost"
	isProxyHTTPLocalhost bool
}

// NewHTTPRoundTripper creates a new initialized HTTP roundtripper.
func NewHTTPRoundTripper(transport *http.Transport, extraHeaders map[string]string) *HTTPRoundTripper {
	proxyConfig := httpproxy.FromEnvironment()
	return &HTTPRoundTripper{
		Transport:            transport,
		extraHeaders:         extraHeaders,
		isProxyHTTPLocalhost: strings.HasPrefix(proxyConfig.HTTPProxy, "http://localhost"),
	}
}

// CloseIdleConnections forwards closing of idle connections on to the wrapped
// transport. This is required to ensure that the underlying [http.Transport] has
// its idle connections closed per the [http.Client] docs:
//
//	> If the Client's Transport does not have a CloseIdleConnections method
//	> then this method does nothing.
func (rt *HTTPRoundTripper) CloseIdleConnections() {
	rt.Transport.CloseIdleConnections()
}

// RoundTrip executes a single HTTP transaction. Part of the RoundTripper interface.
func (rt *HTTPRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	// Add extra HTTP headers.
	for header, v := range rt.extraHeaders {
		req.Header.Add(header, v)
	}

	// Use plain HTTP if proxying via http://localhost in insecure mode.
	tlsConfig := rt.Transport.TLSClientConfig
	if rt.isProxyHTTPLocalhost && tlsConfig != nil && tlsConfig.InsecureSkipVerify {
		req.URL.Scheme = "http"
	}

	return rt.Transport.RoundTrip(req)
}
