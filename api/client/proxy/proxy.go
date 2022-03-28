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

package proxy

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/gravitational/trace"
	"golang.org/x/net/http/httpproxy"
)

// GetProxyAddress gets the HTTP proxy address to use for a given address, if any.
func GetProxyAddress(dialAddr string) *url.URL {
	addrURL, err := parse(dialAddr)
	if err != nil {
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

// parse parses a URL. If the address does not have a scheme, it will prepend "http" and try.
func parse(addr string) (*url.URL, error) {
	proxyurl, err := url.Parse(addr)
	if err != nil || !strings.HasPrefix(proxyurl.Scheme, "http") {
		proxyurl, err = url.Parse("http://" + addr)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		proxyurl.Scheme = ""
	}

	return proxyurl, nil
}

// HTTPFallbackRoundTripper is a wrapper for http.Transport that downgrades requests
// to plain HTTP when using a plain HTTP proxy at localhost.
type HTTPFallbackRoundTripper struct {
	*http.Transport
	isProxyHTTPLocalhost bool
}

func NewHTTPFallbackRoundTripper(transport *http.Transport, insecure bool) *HTTPFallbackRoundTripper {
	proxyConfig := httpproxy.FromEnvironment()
	rt := HTTPFallbackRoundTripper{
		Transport:            transport,
		isProxyHTTPLocalhost: strings.HasPrefix(proxyConfig.HTTPProxy, "http://localhost"),
	}
	if rt.TLSClientConfig != nil {
		rt.TLSClientConfig.InsecureSkipVerify = insecure
	}
	return &rt
}

// RoundTrip executes a single HTTP transaction. Part of the RoundTripper interface.
func (rt *HTTPFallbackRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	tlsConfig := rt.Transport.TLSClientConfig
	// Use plain HTTP if proxying via http://localhost in insecure mode.
	if rt.isProxyHTTPLocalhost && tlsConfig != nil && tlsConfig.InsecureSkipVerify {
		req.URL.Scheme = "http"
	}
	return rt.Transport.RoundTrip(req)
}
