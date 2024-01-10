/*
Copyright 2017 Gravitational, Inc.

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
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/http/httpproxy"
)

func TestGetProxyAddress(t *testing.T) {
	type env struct {
		name string
		val  string
	}
	var tests = []struct {
		info       string
		env        []env
		targetAddr string
		proxyAddr  string
	}{
		{
			info:       "valid, can be raw host:port",
			env:        []env{{name: "http_proxy", val: "proxy:1234"}},
			proxyAddr:  "proxy:1234",
			targetAddr: "192.168.1.1:3030",
		},
		{
			info:       "valid, raw host:port works for https",
			env:        []env{{name: "HTTPS_PROXY", val: "proxy:1234"}},
			proxyAddr:  "proxy:1234",
			targetAddr: "192.168.1.1:3030",
		},
		{
			info:       "valid, correct full url",
			env:        []env{{name: "https_proxy", val: "https://proxy:1234"}},
			proxyAddr:  "proxy:1234",
			targetAddr: "192.168.1.1:3030",
		},
		{
			info:       "valid, http endpoint can be set in https_proxy",
			env:        []env{{name: "https_proxy", val: "http://proxy:1234"}},
			proxyAddr:  "proxy:1234",
			targetAddr: "192.168.1.1:3030",
		},
		{
			info:       "valid, socks5 endpoint can be set in https_proxy",
			env:        []env{{name: "https_proxy", val: "socks5://proxy:1234"}},
			proxyAddr:  "proxy:1234",
			targetAddr: "192.168.1.1:3030",
		},
		{
			info: "valid, http endpoint can be set in https_proxy, but no_proxy override matches domain",
			env: []env{
				{name: "https_proxy", val: "http://proxy:1234"},
				{name: "no_proxy", val: "proxy"}},
			proxyAddr:  "",
			targetAddr: "proxy:1234",
		},
		{
			info: "valid, http endpoint can be set in https_proxy, but no_proxy override matches ip",
			env: []env{
				{name: "https_proxy", val: "http://proxy:1234"},
				{name: "no_proxy", val: "192.168.1.1"}},
			proxyAddr:  "",
			targetAddr: "192.168.1.1:1234",
		},
		{
			info: "valid, http endpoint can be set in https_proxy, but no_proxy override matches subdomain",
			env: []env{
				{name: "https_proxy", val: "http://proxy:1234"},
				{name: "no_proxy", val: ".example.com"}},
			proxyAddr:  "",
			targetAddr: "bla.example.com:1234",
		},
		{
			info: "valid, no_proxy blocks matching port",
			env: []env{
				{name: "https_proxy", val: "proxy:9999"},
				{name: "no_proxy", val: "example.com:1234"},
			},
			proxyAddr:  "",
			targetAddr: "example.com:1234",
		},
		{
			info: "valid, no_proxy matches host but not port",
			env: []env{
				{name: "https_proxy", val: "proxy:9999"},
				{name: "no_proxy", val: "example.com:1234"},
			},
			proxyAddr:  "proxy:9999",
			targetAddr: "example.com:5678",
		},
	}

	// used to augment test cases with auth credentials
	authTests := []struct {
		info     string
		user     string
		password string
	}{
		{info: "no credentials", user: "", password: ""},
		{info: "plain password", user: "alice", password: "password"},
		{info: "special characters in password", user: "alice", password: " !@#$%^&*()_+-=[]{};:,.<>/?`~\"\\ abc123"},
	}

	for i, tt := range tests {
		for j, authTest := range authTests {
			t.Run(fmt.Sprintf("%v %v: %v with %v", i, j, tt.info, authTest.info), func(t *testing.T) {
				for _, env := range tt.env {
					switch strings.ToLower(env.name) {
					case "http_proxy", "https_proxy":
						// add auth test credentials into http(s)_proxy env vars
						val, err := buildProxyAddr(env.val, authTest.user, authTest.password)
						require.NoError(t, err)
						t.Setenv(env.name, val)
					case "no_proxy":
						t.Setenv(env.name, env.val)
					}
				}
				p := GetProxyURL(tt.targetAddr)

				// is a proxy expected?
				if tt.proxyAddr == "" {
					require.Nil(t, p)
					return
				}
				require.NotNil(t, p)
				require.Equal(t, tt.proxyAddr, p.Host)

				// are auth credentials expected?
				if authTest.user == "" && authTest.password == "" {
					require.Nil(t, p.User)
					return
				}
				require.NotNil(t, p.User)
				require.Equal(t, authTest.user, p.User.Username())
				password, _ := p.User.Password()
				require.Equal(t, authTest.password, password)
			})
		}
	}
}

func buildProxyAddr(addr, user, pass string) (string, error) {
	if user == "" && pass == "" {
		return addr, nil
	}
	userInfo := url.UserPassword(user, pass)
	if strings.HasPrefix(addr, "http") || strings.HasPrefix(addr, "socks5") {
		u, err := url.Parse(addr)
		if err != nil {
			return "", trace.Wrap(err)
		}
		u.User = userInfo
		return u.String(), nil
	}
	return fmt.Sprintf("%v@%v", userInfo.String(), addr), nil
}

func TestProxyAwareRoundTripper(t *testing.T) {
	t.Setenv("HTTP_PROXY", "http://localhost:8888")
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
		Proxy: func(req *http.Request) (*url.URL, error) {
			return httpproxy.FromEnvironment().ProxyFunc()(req.URL)
		},
	}
	rt := NewHTTPRoundTripper(transport, nil)
	req, err := http.NewRequest(http.MethodGet, "https://localhost:9999", nil)
	require.NoError(t, err)
	// Don't care about response, only if the scheme changed.
	//nolint:bodyclose // resp should be nil, so there will be no body to close.
	_, err = rt.RoundTrip(req)
	require.Error(t, err)
	require.Equal(t, "http", req.URL.Scheme)
}

// TestHttpRoundTripperDowngrade tests that the round tripper downgrades https requests to http
// when HTTP_PROXY is set to "http://localhost:*" (i.e. there's an http proxy running on localhost).
func TestHttpRoundTripperDowngrade(t *testing.T) {
	testCases := []struct {
		desc           string
		setHTTPProxy   bool
		shouldHitProxy bool
	}{
		{
			desc:           "hits http proxy if insecure and localhost http proxy is set",
			setHTTPProxy:   true,
			shouldHitProxy: true,
		},
		{
			desc:           "does not hit http proxy if insecure and localhost http proxy is not set",
			setHTTPProxy:   false,
			shouldHitProxy: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			newHandler := func(runningAtProxy bool, wasHit *bool) http.HandlerFunc {
				return func(w http.ResponseWriter, r *http.Request) {
					*wasHit = true
					if tc.shouldHitProxy {
						// If the request should hit the proxy, then:
						// - this handler is running at the proxy, and
						// - the scheme should be http.
						require.True(t, runningAtProxy)
						require.Equal(t, "http", r.URL.Scheme)
					}
					w.WriteHeader(http.StatusOK)
				}
			}

			// Start localhost http proxy.
			runningAtProxy := true
			loopback := true
			https := false
			httpProxyWasHit := false
			httpProxy, err := newServer(newHandler(runningAtProxy, &httpProxyWasHit), loopback, https)
			require.NoError(t, err)
			defer httpProxy.Close()

			// Start non-localhost https server.
			runningAtProxy = false
			loopback = false
			https = true
			httpsSrvWasHit := false
			httpsSrv, err := newServer(newHandler(runningAtProxy, &httpsSrvWasHit), loopback, https)
			require.NoError(t, err)
			defer httpsSrv.Close()

			if tc.setHTTPProxy {
				// url.Parse won't correctly parse an absolute URL without a scheme.
				u, err := url.Parse("http://" + httpProxy.Listener.Addr().String())
				require.NoError(t, err)
				_, port, err := net.SplitHostPort(u.Host)
				require.NoError(t, err)

				// Set HTTP_PROXY to "http://localhost:*".
				t.Setenv("HTTP_PROXY", fmt.Sprintf("http://localhost:%s", port))
			}

			clt := newClient(t, nil)

			// Perform any request.
			// Set addr to the https server. If HTTP_PROXY was set above,
			// the http proxy should be hit regardless.
			addr := httpsSrv.Listener.Addr().String()
			request(t, clt, addr)

			// Validate that the correct server was hit.
			require.Equal(t, tc.shouldHitProxy, httpProxyWasHit)
			require.Equal(t, !tc.shouldHitProxy, httpsSrvWasHit)
		})
	}
}

// TestHttpRoundTripperExtraHeaders tests that the round tripper adds the extra headers set.
func TestHttpRoundTripperExtraHeaders(t *testing.T) {
	testCases := []struct {
		desc          string
		extraHeaders  map[string]string
		expectHeaders func(*testing.T, http.Header)
	}{
		{
			desc: "extra headers are added",
			extraHeaders: map[string]string{
				"header1": "value1",
				"header2": "value2",
			},
			expectHeaders: func(t *testing.T, headers http.Header) {
				require.Equal(t, []string{"value1"}, headers.Values("header1"))
				require.Equal(t, []string{"value2"}, headers.Values("header2"))
			},
		},
		{
			desc: "extra headers do not overwrite existing headers",
			extraHeaders: map[string]string{
				"header1":      "value1",
				"Content-Type": "value2",
			},
			expectHeaders: func(t *testing.T, headers http.Header) {
				require.Equal(t, []string{"value1"}, headers.Values("header1"))
				require.Equal(t, []string{"application/json", "value2"}, headers.Values("Content-Type"))
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			var handler http.HandlerFunc = func(w http.ResponseWriter, r *http.Request) {
				tc.expectHeaders(t, r.Header)
				w.WriteHeader(http.StatusOK)
			}

			// Start localhost https server.
			loopback := true
			tls := true
			httpsSrv, err := newServer(handler, loopback, tls)
			require.NoError(t, err)
			defer httpsSrv.Close()

			clt := newClient(t, tc.extraHeaders)

			// Perform any request.
			// Set the address to the localhost https server.
			addr := httpsSrv.Listener.Addr().String()
			request(t, clt, addr)
		})
	}
}

// newServer starts a new server that:
// - runs TLS if `https`
// - uses a loopback listener if `loopback`
func newServer(handler http.HandlerFunc, loopback bool, https bool) (*httptest.Server, error) {
	srv := httptest.NewUnstartedServer(handler)

	if !loopback {
		// Replace the test-supplied loopback listener with the first available
		// non-loopback address.
		srv.Listener.Close()
		l, err := net.Listen("tcp", "0.0.0.0:0")
		if err != nil {
			return nil, err
		}
		srv.Listener = l
	}

	if https {
		srv.StartTLS()
	} else {
		srv.Start()
	}
	return srv, nil
}

// newClient creates a new https roundtrip client.
func newClient(t *testing.T, extraHeaders map[string]string) *http.Client {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			// Setting insecure ensures that https requests succeed.
			InsecureSkipVerify: true,
		},
		Proxy: func(req *http.Request) (*url.URL, error) {
			return httpproxy.FromEnvironment().ProxyFunc()(req.URL)
		},
	}
	return &http.Client{
		Transport: NewHTTPRoundTripper(transport, extraHeaders),
	}
}

// request perform a POST request.
func request(t *testing.T, clt *http.Client, addr string) {
	url := "https://" + addr + "/v1/content"
	resp, err := clt.Post(url, "application/json", nil)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
}

func TestParse(t *testing.T) {
	successTests := []struct {
		name, addr, scheme, host, path string
	}{
		{name: "scheme-host-port", addr: "http://example.com:8080", scheme: "http", host: "example.com:8080", path: ""},
		{name: "host-port", addr: "example.com:8080", scheme: "", host: "example.com:8080", path: ""},
		{name: "scheme-ip4-port", addr: "http://127.0.0.1:8080", scheme: "http", host: "127.0.0.1:8080", path: ""},
		{name: "ip4-port", addr: "127.0.0.1:8080", scheme: "", host: "127.0.0.1:8080", path: ""},
		{name: "scheme-ip6-port", addr: "http://[::1]:8080", scheme: "http", host: "[::1]:8080", path: ""},
		{name: "ip6-port", addr: "[::1]:8080", scheme: "", host: "[::1]:8080"},
		{name: "host/path", addr: "example.com/path/to/somewhere", scheme: "", host: "example.com", path: "/path/to/somewhere"},
	}
	for _, tc := range successTests {
		t.Run(fmt.Sprintf("should parse: %s", tc.name), func(t *testing.T) {
			u, err := ParseURL(tc.addr)
			require.NoError(t, err)
			errMsg := fmt.Sprintf("(%v, %v, %v)", u.Scheme, u.Host, u.Path)
			require.Equal(t, tc.scheme, u.Scheme, errMsg)
			require.Equal(t, tc.host, u.Host, errMsg)
			require.Equal(t, tc.path, u.Path)
		})
	}

	failTests := []struct {
		name, addr string
	}{
		{name: "invalid char in host without scheme", addr: "bad addr"},
	}
	for _, tc := range failTests {
		t.Run(fmt.Sprintf("should not parse: %s", tc.name), func(t *testing.T) {
			u, err := ParseURL(tc.addr)
			require.Error(t, err, u)
		})
	}

	t.Run("empty addr", func(t *testing.T) {
		u, err := ParseURL("")
		require.NoError(t, err)
		require.Nil(t, u)
	})
}
