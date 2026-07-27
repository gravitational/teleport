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

package azure

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/jwt"
	"github.com/gravitational/teleport/lib/srv/app/common"
	"github.com/gravitational/teleport/lib/tlsca"
)

type streamingTestBody struct {
	remaining int64
	started   *atomic.Bool
	readEarly *atomic.Bool
}

func (b *streamingTestBody) Read(p []byte) (int, error) {
	if !b.started.Load() {
		b.readEarly.Store(true)
	}
	if b.remaining == 0 {
		return 0, io.EOF
	}
	if int64(len(p)) > b.remaining {
		p = p[:b.remaining]
	}
	for i := range p {
		p[i] = 'x'
	}
	b.remaining -= int64(len(p))
	return len(p), nil
}

func (b *streamingTestBody) Close() error { return nil }

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

type noopAudit struct{}

func (noopAudit) OnSessionStart(context.Context, string, *tlsca.Identity, types.Application) error {
	return nil
}

func (noopAudit) OnSessionEnd(context.Context, string, *tlsca.Identity, types.Application) error {
	return nil
}

func (noopAudit) OnSessionChunk(context.Context, string, string, *tlsca.Identity, types.Application) error {
	return nil
}

func (noopAudit) OnRequest(context.Context, *common.SessionContext, *http.Request, uint32, *common.AWSResolvedEndpoint) error {
	return nil
}

func (noopAudit) OnDynamoDBRequest(context.Context, *common.SessionContext, *http.Request, uint32, *common.AWSResolvedEndpoint) error {
	return nil
}

func (noopAudit) OnLLMRequest(context.Context, *common.SessionContext, *http.Request, common.LLMRequest, common.LLMResponse) error {
	return nil
}

func (noopAudit) EmitEvent(context.Context, apievents.AuditEvent) error {
	return nil
}

func TestForwarder_streamsLargeRequestBody(t *testing.T) {
	t.Parallel()

	clientKey, err := cryptosuites.GenerateKeyWithAlgorithm(cryptosuites.ECDSAP256)
	require.NoError(t, err)

	clock := clockwork.NewRealClock()
	jwtKey, err := jwt.New(&jwt.Config{
		Clock:       clock,
		PrivateKey:  clientKey,
		ClusterName: types.TeleportAzureMSIEndpoint,
	})
	require.NoError(t, err)

	const resource = "https://storage.azure.com/"
	token, err := jwtKey.SignAzureToken(jwt.AzureTokenClaims{
		TenantID: "tenant",
		Resource: resource,
	})
	require.NoError(t, err)

	var roundTripStarted atomic.Bool
	var readBeforeRoundTrip atomic.Bool
	wantSize := int64(teleport.MaxHTTPRequestSize + 1)

	hnd, err := newAzureHandler(t.Context(), HandlerConfig{
		Clock: clock,
		RoundTripper: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			roundTripStarted.Store(true)

			require.Equal(t, "https", req.URL.Scheme)
			require.Equal(t, "account.blob.core.windows.net", req.URL.Host)
			require.Equal(t, "account.blob.core.windows.net", req.Host)
			require.Equal(t, "Bearer real-azure-token", req.Header.Get("Authorization"))

			n, err := io.Copy(io.Discard, req.Body)
			require.NoError(t, err)
			require.Equal(t, wantSize, n)

			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("ok")),
				Header:     make(http.Header),
			}, nil
		}),
		getAccessToken: func(ctx context.Context, managedIdentity string, scope string) (*azcore.AccessToken, error) {
			if managedIdentity != "azure-identity" {
				return nil, trace.BadParameter("wrong managed identity %q", managedIdentity)
			}
			if scope != resource {
				return nil, trace.BadParameter("wrong scope %q", scope)
			}
			return &azcore.AccessToken{Token: "real-azure-token"}, nil
		},
	})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPut, "https://teleport.local/container/blob", &streamingTestBody{
		remaining: wantSize,
		started:   &roundTripStarted,
		readEarly: &readBeforeRoundTrip,
	})
	req.Header.Set("X-Forwarded-Host", "account.blob.core.windows.net")
	req.Header.Set("Authorization", "Bearer "+token)
	req.TLS = &tls.ConnectionState{
		PeerCertificates: []*x509.Certificate{{PublicKey: clientKey.Public()}},
	}

	app, err := types.NewAppV3(types.Metadata{Name: "azure"}, types.AppSpecV3{Cloud: types.CloudAzure})
	require.NoError(t, err)

	req = common.WithSessionContext(req, &common.SessionContext{
		Identity: &tlsca.Identity{
			Username: "alice",
			RouteToApp: tlsca.RouteToApp{
				AzureIdentity: "azure-identity",
			},
		},
		App:   app,
		Audit: noopAudit{},
	})

	rec := httptest.NewRecorder()
	hnd.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.True(t, roundTripStarted.Load())
	require.False(t, readBeforeRoundTrip.Load(), "request body was read before forwarding")
}

func TestForwarder_getToken(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name string

		setup func(t *testing.T) (context.Context, HandlerConfig)

		managedIdentity string
		scope           string

		wantToken *azcore.AccessToken
		checkErr  require.ErrorAssertionFunc
	}

	tests := []testCase{
		{
			name: "base case",
			setup: func(t *testing.T) (context.Context, HandlerConfig) {
				return t.Context(), HandlerConfig{
					getAccessToken: func(ctx context.Context, managedIdentity string, scope string) (*azcore.AccessToken, error) {
						if managedIdentity != "MY_IDENTITY" {
							return nil, trace.BadParameter("wrong managedIdentity")
						}
						if scope != "MY_SCOPE" {
							return nil, trace.BadParameter("wrong scope")
						}
						return &azcore.AccessToken{Token: "foobar"}, nil
					},
				}
			},
			managedIdentity: "MY_IDENTITY",
			scope:           "MY_SCOPE",
			wantToken:       &azcore.AccessToken{Token: "foobar"},
			checkErr:        require.NoError,
		},
		{
			name: "timeout",
			setup: func(t *testing.T) (context.Context, HandlerConfig) {
				clock := clockwork.NewFakeClock()
				return t.Context(), HandlerConfig{
					Clock: clock,
					getAccessToken: func(ctx context.Context, managedIdentity string, scope string) (*azcore.AccessToken, error) {
						// advance time by getTokenTimeout
						clock.Advance(getTokenTimeout)

						// after the test is done unblock the sleep() below.
						t.Cleanup(func() {
							clock.Advance(getTokenTimeout * 2)
						})

						// block for 2*getTokenTimeout; this call won't return before Cleanup() phase.
						clock.Sleep(getTokenTimeout * 2)

						return &azcore.AccessToken{Token: "foobar"}, nil
					},
				}
			},
			checkErr: func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "timeout waiting for access token for 5s")
				require.ErrorIs(t, err, context.DeadlineExceeded)
			},
		},
		{
			name: "non-timeout error",
			setup: func(t *testing.T) (context.Context, HandlerConfig) {
				return t.Context(), HandlerConfig{
					getAccessToken: func(ctx context.Context, managedIdentity string, scope string) (*azcore.AccessToken, error) {
						return nil, trace.BadParameter("bad param foo")
					},
				}
			},
			checkErr: func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "bad param foo")
				require.True(t, trace.IsBadParameter(err))
			},
		},
		{
			name: "context cancel",
			setup: func(t *testing.T) (context.Context, HandlerConfig) {
				testCaseContext, testCaseContextCancel := context.WithCancel(t.Context())

				return testCaseContext, HandlerConfig{
					getAccessToken: func(ctx context.Context, managedIdentity string, scope string) (*azcore.AccessToken, error) {
						testCaseContextCancel()

						// Block until test completes to ensure ctx.Done() is selected.
						// This prevents a race where both ctx.Done() and resultChan are ready
						// simultaneously and select picks resultChan non-deterministically.
						select {
						case <-t.Context().Done():
						case <-time.After(3 * time.Second):
							// Timeout to prevent test hanging if getToken has a bug
						}
						return nil, trace.BadParameter("bad param foo")
					},
				}
			},
			checkErr: func(t require.TestingT, err error, i ...any) {
				require.ErrorIs(t, err, context.Canceled)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, config := tt.setup(t)

			hnd, err := newAzureHandler(ctx, config)
			require.NoError(t, err)

			token, err := hnd.getToken(ctx, tt.managedIdentity, tt.scope)

			require.Equal(t, tt.wantToken, token)
			tt.checkErr(t, err)
		})
	}
}

func TestForwarder_getToken_cache(t *testing.T) {
	ctx := context.Background()

	clock := clockwork.NewFakeClock()

	calls := 0
	hnd, err := newAzureHandler(ctx, HandlerConfig{
		Clock: clock,
		getAccessToken: func(ctx context.Context, managedIdentity string, scope string) (*azcore.AccessToken, error) {
			calls++
			return &azcore.AccessToken{Token: "OK"}, nil
		},
	})
	require.NoError(t, err)

	// first call goes through
	_, err = hnd.getToken(ctx, "", "")
	require.NoError(t, err)
	require.Equal(t, 1, calls)

	// second call is cached
	_, err = hnd.getToken(ctx, "", "")
	require.NoError(t, err)
	require.Equal(t, 1, calls)

	// advance past cache expiry
	clock.Advance(time.Second * 60 * 2)

	// third call goes through
	_, err = hnd.getToken(ctx, "", "")
	require.NoError(t, err)
	require.Equal(t, 2, calls)
}
