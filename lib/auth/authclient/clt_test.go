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

package authclient

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"net"
	"net/http"
	"testing"
	"testing/synctest"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/test/bufconn"

	"github.com/gravitational/teleport/api/breaker"
	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/fixtures"
)

func fakeCA(t *testing.T, caType types.CertAuthType) types.CertAuthority {
	t.Helper()
	ca, err := types.NewCertAuthority(types.CertAuthoritySpecV2{
		Type:        caType,
		ClusterName: "fizz-buzz",
		ActiveKeys: types.CAKeySet{
			TLS: []*types.TLSKeyPair{
				{
					Cert: []byte(fixtures.TLSCACertPEM),
					Key:  []byte(fixtures.TLSCAKeyPEM),
				},
			},
			SSH: []*types.SSHKeyPair{
				// Two of these to ensure that both are written to known hosts
				{
					PrivateKey: []byte(fixtures.SSHCAPrivateKey),
					PublicKey:  []byte(fixtures.SSHCAPublicKey),
				},
				{
					PrivateKey: []byte(fixtures.SSHCAPrivateKey),
					PublicKey:  []byte(fixtures.SSHCAPublicKey),
				},
			},
		},
	})
	require.NoError(t, err)
	return ca
}

func TestValidateTrustedClusterRequestProto(t *testing.T) {
	native := &ValidateTrustedClusterRequest{
		Token:           "fizz-buzz",
		TeleportVersion: "v19.0.0",
		CAs: []types.CertAuthority{
			fakeCA(t, types.HostCA),
			fakeCA(t, types.UserCA),
		},
	}
	proto, err := native.ToProto()
	require.NoError(t, err)
	backToNative := ValidateTrustedClusterRequestFromProto(proto)
	require.Empty(t, cmp.Diff(native, backToNative))
}

func TestValidateTrustedClusterResponseProto(t *testing.T) {
	native := &ValidateTrustedClusterResponse{
		CAs: []types.CertAuthority{
			fakeCA(t, types.HostCA),
			fakeCA(t, types.UserCA),
		},
	}
	proto, err := native.ToProto()
	require.NoError(t, err)
	backToNative := ValidateTrustedClusterResponseFromProto(proto)
	require.Empty(t, cmp.Diff(native, backToNative))
}

func TestHTTPCircuitBreaker(t *testing.T) {
	synctest.Test(t, synctestHTTPCircuitBreaker)
}
func synctestHTTPCircuitBreaker(t *testing.T) {
	srv := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "this hit the server", 500)
		}),
		TLSConfig: &tls.Config{
			Certificates: []tls.Certificate{requireSnakeoilCert(t)},
		},
	}
	listener := bufconn.Listen(100)

	serveErr := make(chan error, 1)
	go func() {
		serveErr <- srv.ServeTLS(listener, "", "")
	}()
	defer func() {
		require.ErrorIs(t, <-serveErr, http.ErrServerClosed)
	}()

	defer func() {
		require.NoError(t, srv.Close())
	}()

	clt, err := NewClient(client.Config{
		Dialer: client.ContextDialerFunc(func(ctx context.Context, network, addr string) (net.Conn, error) {
			return listener.DialContext(ctx)
		}),
		Credentials: []client.Credentials{
			client.LoadTLS(&tls.Config{InsecureSkipVerify: true}),
		},
		CircuitBreakerConfig: breaker.Config{
			Interval:     time.Hour,
			Trip:         breaker.StaticTripper(true),
			Recover:      breaker.StaticTripper(true),
			IsSuccessful: func(v any, err error) bool { return false },
		},
		DialInBackground: true,
	})
	require.NoError(t, err)
	defer clt.Close()

	_, err = clt.HTTPClient.Get(t.Context(), clt.HTTPClient.Endpoint(), nil)
	require.ErrorContains(t, err, "this hit the server")

	// the breaker should be tripped now, unlike what a default circuit breaker would do

	_, err = clt.HTTPClient.Get(t.Context(), clt.HTTPClient.Endpoint(), nil)
	require.ErrorAs(t, err, new(*trace.ConnectionProblemError))
	require.ErrorContains(t, err, "Unable to communicate with the Teleport Auth Service")
}

func requireSnakeoilCert(t testing.TB) tls.Certificate {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	cert := &x509.Certificate{}
	certDER, err := x509.CreateCertificate(rand.Reader, cert, cert, key.Public(), key)
	require.NoError(t, err)
	return tls.Certificate{
		Certificate: [][]byte{certDER},
		PrivateKey:  key,
	}
}
