/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package accessgraph

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/utils/keys"
	accessgraph "github.com/gravitational/teleport/lib/accessgraph/apiclient"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/utils/cert"
)

// newClientTestKeyRing builds a KeyRing with a self-signed cert for access graph client testing.
func newClientTestKeyRing(t *testing.T, proxyHost string) *client.KeyRing {
	t.Helper()
	creds, err := cert.GenerateSelfSignedCert([]string{proxyHost}, nil, nil, nil)
	require.NoError(t, err)
	priv, err := keys.ParsePrivateKey(creds.PrivateKey)
	require.NoError(t, err)
	return &client.KeyRing{
		KeyRingIndex:       client.KeyRingIndex{Username: "alice", ProxyHost: proxyHost},
		TLSPrivateKey:      priv,
		AccessGraphTLSCert: creds.Cert,
	}
}

// TestCheckResponse_Success covers the no-error paths.
func TestCheckResponse_Success(t *testing.T) {
	t.Parallel()
	require.NoError(t, checkResponse(200, []byte(`{"items":[]}`)))
	require.NoError(t, checkResponse(399, nil))
}

// TestCheckResponse_AGBadRequest covers the AG-native error envelope —
// the only branch we own. Anything else is delegated to [trace.ReadError].
func TestCheckResponse_AGBadRequest(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		statusCode  int
		body        string
		wantStatus  int
		wantMessage string
	}{
		{
			name:        "400 with AG BadRequest envelope",
			statusCode:  400,
			body:        `{"message": "missing required field 'kind'"}`,
			wantStatus:  400,
			wantMessage: "missing required field 'kind'",
		},
		{
			// Constructed body that satisfies both shapes; the AG
			// envelope must take precedence
			name:        "AG envelope wins over teleport envelope",
			statusCode:  400,
			body:        `{"message": "ag-native", "error": {"message": "teleport-envelope"}}`,
			wantStatus:  400,
			wantMessage: "ag-native",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := checkResponse(tt.statusCode, []byte(tt.body))
			var agErr *apiResponseError
			require.ErrorAs(t, err, &agErr, "want *apiResponseError, got %T", err)
			require.Equal(t, tt.wantStatus, agErr.StatusCode)
			require.Equal(t, tt.wantMessage, agErr.Message)
		})
	}
}

// TestCheckResponse_DelegatesToTraceReadError verifies that bodies the
// AG envelope can't claim are passed through to [trace.ReadError]
func TestCheckResponse_DelegatesToTraceReadError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		statusCode int
		body       string
		wantCheck  func(error) bool
	}{
		{
			name:       "403 → IsAccessDenied",
			statusCode: 403,
			body:       `{"error": {"message": "feature not enabled"}}`,
			wantCheck:  trace.IsAccessDenied,
		},
		{
			name:       "404 → IsNotFound",
			statusCode: 404,
			body:       `{"error": {"message": "no such resource"}}`,
			wantCheck:  trace.IsNotFound,
		},
		{
			name:       "400 → IsBadParameter",
			statusCode: 400,
			body:       `{"error": {"message": "bad input"}}`,
			wantCheck:  trace.IsBadParameter,
		},
		{
			// JSON that parses but matches neither the AG envelope
			// (top-level "message") nor the Teleport envelope
			// ("error.message"). Must delegate to trace.ReadError,
			// not surface as *apiResponseError with an empty message.
			name:       "JSON without message or error fields",
			statusCode: 400,
			body:       `{"foo":"bar"}`,
			wantCheck:  trace.IsBadParameter,
		},
		{
			name:       "non-JSON body still returns a non-nil error",
			statusCode: 502,
			body:       "upstream connection refused",
			wantCheck:  func(err error) bool { return err != nil },
		},
		{
			name:       "empty body still returns a non-nil error",
			statusCode: 418,
			body:       "",
			wantCheck:  func(err error) bool { return err != nil },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := checkResponse(tt.statusCode, []byte(tt.body))
			require.True(t, tt.wantCheck(err), "wantCheck failed for err=%v", err)
			// And specifically NOT *apiResponseError, which is
			// reserved for the AG-native envelope.
			var agErr *apiResponseError
			require.NotErrorAs(t, err, &agErr, "must not be *apiResponseError, got %v", err)
		})
	}
}

// fakeResponse is a minimal accessGraphResponse used to drive doRequest
// without standing up a real HTTP exchange.
type fakeResponse struct {
	statusCode int
	body       []byte
}

func (f fakeResponse) StatusCode() int { return f.statusCode }
func (f fakeResponse) Bytes() []byte   { return f.body }

func TestDoRequest(t *testing.T) {
	t.Parallel()

	t.Run("transport error is wrapped, response zeroed", func(t *testing.T) {
		t.Parallel()
		want := errors.New("dial tcp: connection refused")
		got, err := doRequest(fakeResponse{statusCode: 200}, want)
		require.ErrorIs(t, err, want)
		require.Equal(t, fakeResponse{}, got, "response must be zero-valued on transport error")
	})

	t.Run("HTTP error becomes apiResponseError, response zeroed", func(t *testing.T) {
		t.Parallel()
		got, err := doRequest(
			fakeResponse{statusCode: 500, body: []byte(`{"message":"boom"}`)},
			nil,
		)
		var agErr *apiResponseError
		require.ErrorAs(t, err, &agErr, "want *apiResponseError, got %T", err)
		require.Equal(t, 500, agErr.StatusCode)
		require.Equal(t, "boom", agErr.Message)
		require.Equal(t, fakeResponse{}, got, "response must be zero-valued on HTTP error")
	})

	t.Run("success returns the original response unchanged", func(t *testing.T) {
		t.Parallel()
		in := fakeResponse{statusCode: 200, body: []byte(`{"items":[]}`)}
		got, err := doRequest(in, nil)
		require.NoError(t, err)
		require.Equal(t, in, got)
	})
}

// TestAccessGraphClient_AgainstMockServer exercises client and helpers using
// an httptest server.
func TestAccessGraphClient_AgainstMockServer(t *testing.T) {
	t.Parallel()

	// Use a non-IP ProxyHost so the TLS client actually sends SNI —
	// per RFC 6066, IP literals don't get SNI. We use `example.com`
	// the certs that httptest generate include `example.com` as a valid DNS name
	const proxyHost = "example.com"
	keyRing := newClientTestKeyRing(t, proxyHost)

	const aliasesPath = "/v1/enterprise/accessgraph/graph/account-aliases"

	type assertion func(t *testing.T, err error)
	wantSuccess := func() assertion {
		return func(t *testing.T, err error) { require.NoError(t, err) }
	}
	wantAGError := func(status int, msg string) assertion {
		return func(t *testing.T, err error) {
			var agErr *apiResponseError
			require.ErrorAs(t, err, &agErr, "want *apiResponseError, got %T", err)
			require.Equal(t, status, agErr.StatusCode)
			require.Equal(t, msg, agErr.Message)
		}
	}
	wantTraceCheck := func(check func(error) bool) assertion {
		return func(t *testing.T, err error) {
			require.True(t, check(err), "trace check failed for err=%v", err)
			var agErr *apiResponseError
			require.NotErrorAs(t, err, &agErr, "teleport-envelope errors must not surface as *apiResponseError")
		}
	}

	tests := []struct {
		name    string
		handler http.HandlerFunc
		assert  assertion
	}{
		{
			name: "200 with empty alias list → no error",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(200)
				_, _ = w.Write([]byte(`[]`))
			},
			assert: wantSuccess(),
		},
		{
			name: "400 with AG BadRequest body",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(400)
				_, _ = w.Write([]byte(`{"message":"invalid filter"}`))
			},
			assert: wantAGError(400, "invalid filter"),
		},
		{
			// Simulates teleport proxy error returning a teleport [trace.Error]
			name: "403 with teleport proxy envelope → typed trace error",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(403)
				_, _ = w.Write([]byte(`{"error":{"message":"access graph is not enabled"}}`))
			},
			assert: wantTraceCheck(trace.IsAccessDenied),
		},
		{
			name: "503 with empty body → non-nil trace error",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(503)
			},
			assert: wantTraceCheck(func(err error) bool { return err != nil }),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var (
				gotPath string
				gotSNI  string
			)
			srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotPath = r.URL.Path
				tt.handler(w, r)
			}))

			// Observe the SNI the client sends, which must match the keyring's ProxyHost.
			srv.TLS = &tls.Config{
				GetCertificate: func(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
					gotSNI = hello.ServerName
					return nil, nil
				},
			}
			srv.StartTLS()
			t.Cleanup(srv.Close)

			// Manually construct the http client so we can inject the RootCAs needed to verify the cert
			httpClient, err := newAccessGraphHTTPClient(context.Background(), srv.Listener.Addr().String(), keyRing)
			require.NoError(t, err)
			tr := httpClient.Transport.(*http.Transport)
			// httptest's cert is signed by a private CA, so we need to add it to the client's RootCAs
			tr.TLSClientConfig.RootCAs = x509.NewCertPool()
			tr.TLSClientConfig.RootCAs.AddCert(srv.Certificate())

			// Construct the client with the test HTTP client, then do a request to verify the full stack,
			baseURL := (&url.URL{
				Scheme: "https",
				Host:   srv.Listener.Addr().String(),
				Path:   accessGraphAPIPath,
			}).String()
			ag, err := accessgraph.NewClientWithResponses(
				baseURL,
				accessgraph.WithHTTPClient(httpClient),
			)
			require.NoError(t, err)

			_, err = doRequest(ag.GetAccountAliasesWithResponse(context.Background()))
			tt.assert(t, err)
			require.Equal(t, aliasesPath, gotPath, "client must hit the AG-mounted path")
			require.Equal(t, proxyHost, gotSNI, "client must propagate keyring ProxyHost as SNI")
		})
	}
}

// TestNewAccessGraphClient_InputValidation covers the cheap up-front
// guards that don't require a real keyring.
func TestNewAccessGraphClient_InputValidation(t *testing.T) {
	t.Parallel()

	t.Run("empty proxy address", func(t *testing.T) {
		t.Parallel()
		_, err := newAccessGraphClient(context.Background(), "", &client.KeyRing{})
		require.Error(t, err)
		require.Contains(t, err.Error(), "proxy address")
	})

	t.Run("nil keyring", func(t *testing.T) {
		t.Parallel()
		_, err := newAccessGraphClient(context.Background(), "proxy.example.com:443", nil)
		require.Error(t, err)
		require.True(t, strings.Contains(err.Error(), "key ring"), "got %v", err)
	})
}

// TestNewAccessGraphClient_Success covers the constructor happy path
// and the proxy-address normalization performed by utils.ParseAddr.

func TestNewAccessGraphClient_Success(t *testing.T) {
	t.Parallel()

	keyRing := newClientTestKeyRing(t, "proxy.example.com")

	tests := []struct {
		name      string
		proxyAddr string
		wantErr   string // empty → success
	}{
		{"bare host", "proxy.example.com", ""},
		{"host:port", "proxy.example.com:443", ""},
		{"normalizes scheme", "https://proxy.example.com:443", ""},
		{"strips trailing path", "https://proxy.example.com/anything", ""},
		{"unsupported scheme", "ftp://proxy.example.com", "unsupported scheme"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c, err := newAccessGraphClient(context.Background(), tt.proxyAddr, keyRing)
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, c)
		})
	}
}
