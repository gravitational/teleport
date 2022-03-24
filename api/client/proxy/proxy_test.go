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

package proxy

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"testing"

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

	for i, tt := range tests {
		t.Run(fmt.Sprintf("%v: %v", i, tt.info), func(t *testing.T) {
			for _, env := range tt.env {
				t.Setenv(env.name, env.val)
			}
			p := GetProxyAddress(tt.targetAddr)
			if tt.proxyAddr == "" {
				require.Nil(t, p)
			} else {
				require.Equal(t, tt.proxyAddr, p.Host)
			}
		})
	}
}

func TestProxyAwareRoundTripper(t *testing.T) {
	t.Setenv("HTTP_PROXY", "http://localhost:8888")
	rt := HTTPFallbackRoundTripper{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
			Proxy: func(req *http.Request) (*url.URL, error) {
				return httpproxy.FromEnvironment().ProxyFunc()(req.URL)
			},
		},
	}
	req, err := http.NewRequest(http.MethodGet, "https://localhost:9999", nil)
	require.NoError(t, err)
	// Don't care about response, only if the scheme changed.
	_, err = rt.RoundTrip(req)
	require.Error(t, err)
	require.Equal(t, "http", req.URL.Scheme)
}
