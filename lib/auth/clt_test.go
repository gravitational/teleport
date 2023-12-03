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

package auth

import (
	"context"
	"crypto/tls"
	"crypto/x509/pkix"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/breaker"
	apiclient "github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/tlsca"
)

func TestClient_DialTimeout(t *testing.T) {
	t.Parallel()
	cases := []struct {
		desc    string
		timeout time.Duration
	}{
		{
			desc:    "dial timeout set to valid value",
			timeout: 500 * time.Millisecond,
		},
		{
			desc:    "defaults prevent infinite timeout",
			timeout: 0,
		},
	}

	for _, tt := range cases {
		t.Run(tt.desc, func(t *testing.T) {
			tt := tt
			t.Parallel()

			// create a client that will attempt to connect to a blackholed address. The address is reserved
			// for benchmarking by RFC 6890.
			cfg := apiclient.Config{
				DialTimeout: tt.timeout,
				Addrs:       []string{"198.18.0.254:1234"},
				Credentials: []apiclient.Credentials{
					apiclient.LoadTLS(&tls.Config{}),
				},
				CircuitBreakerConfig: breaker.NoopBreakerConfig(),
			}
			clt, err := NewClient(cfg)
			require.NoError(t, err)

			// call this so that the DialTimeout gets updated, if necessary, so that we know how long to
			// wait before failing this test
			require.NoError(t, cfg.CheckAndSetDefaults())

			errChan := make(chan error, 1)
			go func() {
				// try to create a session - this will timeout after the DialTimeout threshold is exceeded
				_, err := clt.CreateSessionTracker(context.Background(), &types.SessionTrackerV1{})
				errChan <- err
			}()

			select {
			case err := <-errChan:
				require.Error(t, err)
			case <-time.After(cfg.DialTimeout + (cfg.DialTimeout / 2)):
				t.Fatal("Timed out waiting for dial to complete")
			}
		})
	}
}

func TestClient_RequestTimeout(t *testing.T) {
	t.Parallel()

	testDone := make(chan struct{})
	sawRoot := make(chan bool, 1)
	sawSlow := make(chan bool, 1)

	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v2/authorities/host/rotate/external":
			sawRoot <- true
			http.Redirect(w, r, "/slow", http.StatusFound)
		case "/slow":
			sawSlow <- true
			w.Write([]byte("Hello"))
			w.(http.Flusher).Flush()
			<-testDone
		}
	}))
	t.Cleanup(func() {
		close(testDone)
		srv.Close()
	})

	srv.TLS = &tls.Config{InsecureSkipVerify: true}

	cfg := apiclient.Config{
		Addrs: []string{srv.Listener.Addr().String()},
		Credentials: []apiclient.Credentials{
			apiclient.LoadTLS(srv.TLS),
		},
		CircuitBreakerConfig: breaker.NoopBreakerConfig(),
	}
	clt, err := NewClient(cfg)
	require.NoError(t, err)

	srv.StartTLS()

	ca := newCertAuthority(t, "test", types.HostCA)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	t.Cleanup(cancel)

	err = clt.RotateExternalCertAuthority(ctx, ca)
	require.ErrorIs(t, trace.Unwrap(err), context.DeadlineExceeded)

	close(sawRoot)
	require.True(t, <-sawRoot, "handler never got /v2/authorities/host/rotate/external request")

	close(sawSlow)
	require.True(t, <-sawSlow, "handler never got /slow request")
}

func newCertAuthority(t *testing.T, name string, caType types.CertAuthType) types.CertAuthority {
	ta := testauthority.New()
	priv, pub, err := ta.GenerateKeyPair()
	require.NoError(t, err)

	// CA for cluster1 with 1 key pair.
	key, cert, err := tlsca.GenerateSelfSignedCA(pkix.Name{CommonName: name}, nil, time.Minute)
	require.NoError(t, err)

	ca, err := types.NewCertAuthority(types.CertAuthoritySpecV2{
		Type:        caType,
		ClusterName: name,
		ActiveKeys: types.CAKeySet{
			SSH: []*types.SSHKeyPair{{
				PrivateKey:     priv,
				PrivateKeyType: types.PrivateKeyType_RAW,
				PublicKey:      pub,
			}},
			TLS: []*types.TLSKeyPair{{
				Cert: cert,
				Key:  key,
			}},
		},
	})
	require.NoError(t, err)
	return ca
}
