package forward

import (
	"crypto/tls"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRewrite(t *testing.T) {
	tests := []struct {
		name               string
		trustForwardHeader bool
		incomingHeader     http.Header
		hostname           string
		remoteAddr         string
		host               string
		url                string
		ssl                bool
		expectedHeaders    http.Header
	}{
		{
			name:               "trust forward header, no hr hostname, no port, http",
			trustForwardHeader: true,
			remoteAddr:         "127.0.0.1:12345",
			host:               "localhost",
			url:                "http://localhost",
			expectedHeaders: http.Header{
				XForwardedFor:    []string{"127.0.0.1"},
				XForwardedHost:   []string{"localhost"},
				XForwardedServer: nil,
				XForwardedSSL:    []string{"off"},
				XForwardedPort:   []string{"80"},
			},
		},
		{
			name:               "trust forward header, hr hostname, no port, http",
			trustForwardHeader: true,
			hostname:           "hr-host",
			remoteAddr:         "127.0.0.1:12345",
			host:               "localhost",
			url:                "http://localhost",
			expectedHeaders: http.Header{
				XForwardedFor:    []string{"127.0.0.1"},
				XForwardedHost:   []string{"localhost"},
				XForwardedServer: []string{"hr-host"},
				XForwardedSSL:    []string{"off"},
				XForwardedPort:   []string{"80"},
			},
		},
		{
			name:               "don't trust forward header, hr hostname, no port, http",
			trustForwardHeader: false,
			hostname:           "hr-host",
			remoteAddr:         "127.0.0.1:12345",
			host:               "localhost",
			url:                "http://localhost",
			expectedHeaders: http.Header{
				XForwardedFor:    []string{"127.0.0.1"},
				XForwardedHost:   []string{"localhost"},
				XForwardedServer: []string{"hr-host"},
				XForwardedSSL:    []string{"off"},
				XForwardedPort:   []string{"80"},
			},
		},
		{
			name:               "trust forward header, hr hostname, no port, https",
			trustForwardHeader: true,
			hostname:           "hr-host",
			remoteAddr:         "127.0.0.1:12345",
			host:               "localhost",
			url:                "https://localhost",
			ssl:                true,
			expectedHeaders: http.Header{
				XForwardedFor:    []string{"127.0.0.1"},
				XForwardedHost:   []string{"localhost"},
				XForwardedServer: []string{"hr-host"},
				XForwardedSSL:    []string{"on"},
				XForwardedPort:   []string{"443"},
			},
		},
		{
			name:               "trust forward header, hr hostname, set proto, no port, https",
			trustForwardHeader: true,
			incomingHeader: http.Header{
				XForwardedProto: []string{"https"},
			},
			hostname:   "hr-host",
			remoteAddr: "127.0.0.1:12345",
			host:       "localhost",
			url:        "https://localhost",
			ssl:        true,
			expectedHeaders: http.Header{
				XForwardedFor:    []string{"127.0.0.1"},
				XForwardedHost:   []string{"localhost"},
				XForwardedServer: []string{"hr-host"},
				XForwardedSSL:    []string{"on"},
				XForwardedPort:   []string{"443"},
			},
		},
		{
			name:               "trust forward header, hr hostname, port, https",
			trustForwardHeader: true,
			hostname:           "hr-host",
			remoteAddr:         "127.0.0.1:12345",
			host:               "localhost",
			url:                "https://localhost:23456",
			ssl:                true,
			expectedHeaders: http.Header{
				XForwardedFor:    []string{"127.0.0.1"},
				XForwardedHost:   []string{"localhost"},
				XForwardedServer: []string{"hr-host"},
				XForwardedSSL:    []string{"on"},
				XForwardedPort:   []string{"23456"},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			url, err := url.Parse(test.url)
			require.NoError(t, err)
			req := &http.Request{
				RemoteAddr: test.remoteAddr,
				URL:        url,
				Host:       test.host,
				Header:     http.Header{},
			}

			if test.incomingHeader != nil {
				req.Header = test.incomingHeader
			}

			if test.ssl {
				req.TLS = &tls.ConnectionState{}
			}

			hr := &HeaderRewriter{
				TrustForwardHeader: test.trustForwardHeader,
				Hostname:           test.hostname,
			}
			hr.Rewrite(req)
			for expectedHeaderName, expectedHeaderValues := range test.expectedHeaders {
				assert.Equal(t, expectedHeaderValues, req.Header.Values(expectedHeaderName), "%s did not match", expectedHeaderName)
			}
		})
	}
}
