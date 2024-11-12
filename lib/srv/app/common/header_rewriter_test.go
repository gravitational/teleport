/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package common

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/gravitational/teleport/lib/httplib/reverseproxy"
)

func mustParseURL(urlString string) *url.URL {
	url, err := url.Parse(urlString)
	if err != nil {
		panic(fmt.Sprintf("error parsing URL: %v", err))
	}

	return url
}

type testDelegate struct {
	header string
	value  string
}

func newTestDelegate(header, value string) *testDelegate {
	return &testDelegate{
		header: header,
		value:  value,
	}
}

func (t *testDelegate) Rewrite(req *httputil.ProxyRequest) {
	req.Out.Header.Set(t.header, t.value)
}

func TestHeaderRewriter(t *testing.T) {
	tests := []struct {
		name               string
		req                *http.Request
		extraDelegates     []reverseproxy.Rewriter
		expectedHeaders    http.Header
		expectedSSLHeader  string
		expectedPortHeader string
	}{
		{
			name: "http, no port specified",
			req: &http.Request{
				Host:   "testhost",
				URL:    mustParseURL("http://testhost"),
				Header: http.Header{},
			},
			expectedHeaders: http.Header{
				XForwardedSSL:               []string{sslOff},
				reverseproxy.XForwardedPort: []string{"80"},
			},
		},
		{
			name: "http, port specified",
			req: &http.Request{
				Host:   "testhost:12345",
				URL:    mustParseURL("http://testhost:12345"),
				Header: http.Header{},
			},
			expectedHeaders: http.Header{
				XForwardedSSL:               []string{sslOff},
				reverseproxy.XForwardedPort: []string{"12345"},
			},
		},
		{
			name: "https, no port specified",
			req: &http.Request{
				Host:   "testhost",
				URL:    mustParseURL("https://testhost"),
				Header: http.Header{},
				TLS:    &tls.ConnectionState{},
			},
			expectedHeaders: http.Header{
				XForwardedSSL:               []string{sslOn},
				reverseproxy.XForwardedPort: []string{"443"},
			},
		},
		{
			name: "https, port specified, extra delegates",
			req: &http.Request{
				Host:   "testhost:12345",
				URL:    mustParseURL("https://testhost:12345"),
				Header: http.Header{},
				TLS:    &tls.ConnectionState{},
			},
			extraDelegates: []reverseproxy.Rewriter{
				newTestDelegate("test-1", "value-1"),
				newTestDelegate("test-2", "value-2"),
			},
			expectedHeaders: http.Header{
				XForwardedSSL:               []string{sslOn},
				reverseproxy.XForwardedPort: []string{"12345"},
				"test-1":                    []string{"value-1"},
				"test-2":                    []string{"value-2"},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			delegates := []reverseproxy.Rewriter{&reverseproxy.HeaderRewriter{}}
			delegates = append(delegates, test.extraDelegates...)
			hr := NewHeaderRewriter(delegates...)

			// replicate net/http/httputil.ReverseProxy stripping
			// forwarding headers from the outbound request
			outReq := test.req.Clone(test.req.Context())
			outReq.Header.Del("Forwarded")
			outReq.Header.Del(reverseproxy.XForwardedFor)
			outReq.Header.Del(reverseproxy.XForwardedHost)
			outReq.Header.Del(reverseproxy.XForwardedProto)

			hr.Rewrite(&httputil.ProxyRequest{
				In:  test.req,
				Out: outReq,
			})

			for header, value := range test.expectedHeaders {
				assert.Equal(t, outReq.Header.Get(header), value[0])
			}
		})
	}
}
