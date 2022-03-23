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
func GetProxyAddress(addr string) *url.URL {
	addrURL, err := parse(addr)
	if err != nil {
		return nil
	}
	proxyFunc := httpproxy.FromEnvironment().ProxyFunc()
	for _, scheme := range []string{"https", "http"} {
		addrURL.Scheme = scheme
		proxyURL, err := proxyFunc(addrURL)
		if err == nil && proxyURL != nil {
			return proxyURL
		}
	}

	return nil
}

// parse will extract the host:port of the proxy to dial to. If the
// value is not prefixed by "http", then it will prepend "http" and try.
func parse(addr string) (*url.URL, error) {
	proxyurl, err := url.Parse(addr)
	if err != nil || !strings.HasPrefix(proxyurl.Scheme, "http") {
		proxyurl, err = url.Parse("http://" + addr)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	return proxyurl, nil
}

// ProxyAwareRoundTripper is a wrapper for http.Transport that can modify roundtrips as needed.
type ProxyAwareRoundTripper struct {
	http.Transport
}

func (rt *ProxyAwareRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	tlsConfig := rt.Transport.TLSClientConfig
	if tlsConfig == nil {
		return rt.Transport.RoundTrip(req)
	}
	httpProxy := httpproxy.FromEnvironment().HTTPProxy
	if httpProxy == "" {
		return rt.Transport.RoundTrip(req)
	}
	httpProxyURL, err := parse(httpProxy)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// Use plain HTTP if proxying via http://localhost in insecure mode.
	if tlsConfig.InsecureSkipVerify &&
		httpProxyURL.Scheme == "http" &&
		httpProxyURL.Hostname() == "localhost" {
		req.URL.Scheme = "http"
	}
	return rt.Transport.RoundTrip(req)
}
