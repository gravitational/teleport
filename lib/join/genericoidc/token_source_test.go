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

package genericoidc

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

func TestGetIDTokenFromEnvironment(t *testing.T) {
	t.Parallel()

	fakeGetEnv := func(expectKey, env string) envGetter {
		return func(key string) string {
			if key == expectKey {
				return env
			}

			return ""
		}
	}

	const keyName = "OIDC_TEST"

	tests := []struct {
		name        string
		getEnv      envGetter
		wantString  string
		assertError require.ErrorAssertionFunc
	}{
		{
			name:        "success",
			getEnv:      fakeGetEnv(keyName, "foobarbizz"),
			wantString:  "foobarbizz",
			assertError: require.NoError,
		},
		{
			name:   "unset",
			getEnv: fakeGetEnv(keyName, ""),
			assertError: func(t require.TestingT, err error, i ...any) {
				require.True(t, trace.IsBadParameter(err))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := NewIDTokenSource(tt.getEnv, DefaultCommandRunner, DefaultHTTPRequester)
			str, err := v.GetIDTokenFromEnvironment(keyName)
			require.Equal(t, tt.wantString, str)
			tt.assertError(t, err)
		})
	}
}

func TestGetIDTokenFromCommand(t *testing.T) {
	t.Parallel()

	// fakeRunner produces command runners that simulate a delay and return a
	// set value; fast when run under synctest.
	fakeRunner := func(delay time.Duration, ret func() ([]byte, error)) commandRunner {
		return func(ctx context.Context, command ...string) ([]byte, error) {
			select {
			case <-time.After(delay):
				return ret()
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
	}

	tests := []struct {
		name        string
		runner      commandRunner
		timeout     time.Duration
		wantString  string
		assertError require.ErrorAssertionFunc
	}{
		{
			name:    "success",
			timeout: time.Minute,
			runner: fakeRunner(time.Second, func() ([]byte, error) {
				return []byte("foo"), nil
			}),
			wantString:  "foo",
			assertError: require.NoError,
		},
		{
			name:    "error",
			timeout: time.Minute,
			runner: fakeRunner(time.Second, func() ([]byte, error) {
				return nil, errors.New("oops")
			}),
			assertError: func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "oops")
			},
		},
		{
			name:    "timeout",
			timeout: time.Minute,
			runner: fakeRunner(time.Hour, func() ([]byte, error) {
				return []byte("impossible"), nil
			}),
			assertError: func(t require.TestingT, err error, i ...any) {
				require.ErrorIs(t, err, context.DeadlineExceeded)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				v := NewIDTokenSource(os.Getenv, tt.runner, DefaultHTTPRequester)

				str, err := v.GetIDTokenFromCommand(t.Context(), tt.timeout, "foo")
				require.Equal(t, tt.wantString, str)
				tt.assertError(t, err)
			})
		})
	}
}

func TestGetIDTokenFromHTTPEndpoint_Timeout(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		source := NewIDTokenSource(os.Getenv, DefaultCommandRunner, func(ctx context.Context, _ JWTFromHTTPEndpointParams) ([]byte, error) {
			<-ctx.Done()
			return nil, ctx.Err()
		})

		start := time.Now()
		token, err := source.GetIDTokenFromHTTPEndpoint(t.Context(), time.Minute, JWTFromHTTPEndpointParams{
			Request: HTTPRequestParams{
				URL: "http://example.com",
			},
		})
		require.ErrorIs(t, err, context.DeadlineExceeded)
		require.Empty(t, token)
		require.Equal(t, time.Minute, time.Since(start))
	})
}

func TestIsEndpointAllowed(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name              string
		endpoint          string
		insecureAllowHTTP bool
		errCheck          require.ErrorAssertionFunc
	}{
		{
			name:     "HTTPS",
			endpoint: "https://example.com/token",
			errCheck: require.NoError,
		},
		{
			name:              "HTTPS with opt-in",
			endpoint:          "https://example.com/token",
			insecureAllowHTTP: true,
			errCheck:          require.NoError,
		},
		{
			name:     "metadata HTTP",
			endpoint: "http://169.254.169.254/token",
			errCheck: require.NoError,
		},
		{
			name:     "metadata HTTP with port",
			endpoint: "http://169.254.169.254:8080/token",
			errCheck: require.NoError,
		},
		{
			name:     "loopback HTTP",
			endpoint: "http://127.0.0.1/token",
			errCheck: require.NoError,
		},
		{
			name:     "loopback HTTP with port",
			endpoint: "http://127.0.0.1:8080/token",
			errCheck: require.NoError,
		},
		{
			name:     "other HTTP host",
			endpoint: "http://example.com/token",
			errCheck: require.Error,
		},
		{
			name:              "other HTTP host with opt-in",
			endpoint:          "http://example.com/token",
			insecureAllowHTTP: true,
			errCheck:          require.NoError,
		},
		{
			name:     "metadata lookalike hostname",
			endpoint: "http://169.254.169.254.example.com/token",
			errCheck: require.Error,
		},
		{
			name:     "loopback lookalike hostname",
			endpoint: "http://127.0.0.1.example.com/token",
			errCheck: require.Error,
		},
		{
			name:     "allowlisted IP in userinfo",
			endpoint: "http://127.0.0.1@example.com/token",
			errCheck: require.Error,
		},
		{
			name:     "unsupported scheme",
			endpoint: "ftp://127.0.0.1/token",
			errCheck: require.Error,
		},
		{
			name:              "unsupported scheme with opt-in",
			endpoint:          "ftp://127.0.0.1/token",
			insecureAllowHTTP: true,
			errCheck:          require.Error,
		},
		{
			name:              "missing scheme with opt-in",
			endpoint:          "//127.0.0.1/token",
			insecureAllowHTTP: true,
			errCheck:          require.Error,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			endpoint, err := url.Parse(tt.endpoint)
			require.NoError(t, err)
			tt.errCheck(t, isEndpointAllowed(endpoint, tt.insecureAllowHTTP))
		})
	}
}

func TestDefaultHTTPRequester_InsecureAllowHTTP(t *testing.T) {
	t.Parallel()

	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		_, _ = w.Write([]byte("header.payload.signature"))
	}))
	t.Cleanup(server.Close)

	endpoint, err := url.Parse(server.URL)
	require.NoError(t, err)
	// Only the literal loopback IP is allowlisted, so localhost requires opt-in.
	endpoint.Host = "localhost:" + endpoint.Port()
	params := JWTFromHTTPEndpointParams{
		Request: HTTPRequestParams{URL: endpoint.String()},
	}
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	token, err := DefaultHTTPRequester(ctx, params)
	require.ErrorContains(t, err, "is not allowed")
	require.Empty(t, token)
	require.Zero(t, requests.Load(), "rejected endpoints must never be contacted")

	params.Request.InsecureAllowHTTP = true
	token, err = DefaultHTTPRequester(ctx, params)
	require.NoError(t, err)
	require.Equal(t, "header.payload.signature", string(token))
	require.Equal(t, int32(1), requests.Load())
}

func TestDefaultHTTPRequester_Request(t *testing.T) {
	t.Parallel()

	requests := make(chan *http.Request, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests <- r
		_, _ = w.Write([]byte("header.payload.signature"))
	}))
	t.Cleanup(server.Close)

	token, err := DefaultHTTPRequester(t.Context(), JWTFromHTTPEndpointParams{
		Request: HTTPRequestParams{
			URL:         server.URL + "/token?audience=old&existing=keep",
			Method:      http.MethodPost,
			QueryParams: map[string]string{"audience": "https://teleport.example.com"},
			Headers:     map[string]string{"Metadata": "true"},
		},
	})
	require.NoError(t, err)
	require.Equal(t, "header.payload.signature", string(token))

	select {
	case req := <-requests:
		require.Equal(t, http.MethodPost, req.Method)
		require.Equal(t, "/token", req.URL.Path)
		require.Equal(t, url.Values{
			"audience": {"https://teleport.example.com"},
			"existing": {"keep"},
		}, req.URL.Query())
		require.Equal(t, "true", req.Header.Get("Metadata"))
	default:
		t.Fatal("missing HTTP request")
	}
}

func TestDefaultHTTPRequester_DoesNotFollowRedirects(t *testing.T) {
	// Supporting redirects could cause the HTTP client to do a request and send possibly sensitive credentials to a different host/endpoint.
	var redirectTargetHits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			http.Redirect(w, r, "/redirected", http.StatusFound)
		case "/redirected":
			redirectTargetHits.Add(1)
			_, _ = w.Write([]byte("header.payload.signature"))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	token, err := DefaultHTTPRequester(t.Context(), JWTFromHTTPEndpointParams{
		Request: HTTPRequestParams{URL: server.URL + "/token"},
	})
	require.ErrorContains(t, err, "invalid status (302 Found)")
	require.Empty(t, token)
	require.Zero(t, redirectTargetHits.Load(), "redirect target must never be requested")
}

func TestDefaultHTTPRequester_Response(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name     string
		body     string
		jsonPath string
		want     string
		wantErr  string
	}{
		{name: "raw token", body: "header.payload.signature", want: "header.payload.signature"},
		{name: "ID token", body: `{"access_token":"access","id_token":"id"}`, jsonPath: "$.id_token", want: "id"},
		{name: "invalid JSONPath", body: `{}`, jsonPath: "$.tokens[", wantErr: "invalid result.json_path expression"},
		{name: "multiple tokens", body: `{"tokens":["first","second"]}`, jsonPath: "$.tokens[*]", wantErr: "must select exactly one value, selected 2"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte(tt.body))
			}))
			t.Cleanup(server.Close)

			token, err := DefaultHTTPRequester(t.Context(), JWTFromHTTPEndpointParams{
				Request: HTTPRequestParams{URL: server.URL},
				Result:  ResponseExtractParams{JSONPath: tt.jsonPath},
			})
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				require.Empty(t, token)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, string(token))
		})
	}
}
