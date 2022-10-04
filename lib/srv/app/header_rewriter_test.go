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

package app

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"testing"

	"github.com/gravitational/oxy/forward"
	"github.com/gravitational/teleport/lib/srv/app/common"
	"github.com/stretchr/testify/assert"
)

func mustParseURL(urlString string) *url.URL {
	url, err := url.Parse(urlString)
	if err != nil {
		panic(fmt.Sprintf("error parsing URL: %v", err))
	}

	return url
}

func TestHeaderRewriter(t *testing.T) {
	tests := []struct {
		name               string
		req                *http.Request
		expectedSSLHeader  string
		expectedPortHeader string
	}{
		{
			name: "http, no port specified",
			req: &http.Request{
				URL:    mustParseURL("http://testhost"),
				Header: http.Header{},
			},
			expectedSSLHeader:  sslOff,
			expectedPortHeader: "80",
		},
		{
			name: "http, port specified",
			req: &http.Request{
				URL:    mustParseURL("http://testhost:12345"),
				Header: http.Header{},
			},
			expectedSSLHeader:  sslOff,
			expectedPortHeader: "12345",
		},
		{
			name: "https, no port specified",
			req: &http.Request{
				URL:    mustParseURL("https://testhost"),
				Header: http.Header{},
				TLS:    &tls.ConnectionState{},
			},
			expectedSSLHeader:  sslOn,
			expectedPortHeader: "443",
		},
		{
			name: "https, port specified",
			req: &http.Request{
				URL:    mustParseURL("https://testhost:12345"),
				Header: http.Header{},
				TLS:    &tls.ConnectionState{},
			},
			expectedSSLHeader:  sslOn,
			expectedPortHeader: "12345",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			hr := headerRewriter{
				delegate: &forward.HeaderRewriter{},
			}

			hr.Rewrite(test.req)

			assert.Equal(t, test.req.Header.Get(common.XForwardedSSL), test.expectedSSLHeader)
			assert.Equal(t, test.req.Header.Get(common.XForwardedPort), test.expectedPortHeader)
		})
	}
}
