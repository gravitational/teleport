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
	"crypto"
	"crypto/x509/pkix"
	"errors"
	"net/url"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/tlsca"
	commonclient "github.com/gravitational/teleport/tool/tctl/common/client"
)

// testCA bundles a self-signed CA and a clock; it mints Access Graph TLS
// certs against an arbitrary public key for the validation tests below.
type testCA struct {
	ca    *tlsca.CertAuthority
	clock clockwork.Clock
}

func newTestCA(t *testing.T) *testCA {
	t.Helper()
	caKey, err := cryptosuites.GenerateKeyWithAlgorithm(cryptosuites.ECDSAP256)
	require.NoError(t, err)
	caCertPEM, err := tlsca.GenerateSelfSignedCAWithSigner(
		caKey,
		pkix.Name{CommonName: "test-ca", Organization: []string{"test"}},
		nil,
		time.Hour,
	)
	require.NoError(t, err)
	ca, err := tlsca.FromCertAndSigner(caCertPEM, caKey)
	require.NoError(t, err)
	return &testCA{ca: ca, clock: clockwork.NewRealClock()}
}

func (c *testCA) signAccessGraphCert(t *testing.T, pub crypto.PublicKey, ttl time.Duration) []byte {
	t.Helper()
	identity := tlsca.Identity{Username: "alice", Groups: []string{"access"}}
	subject, err := identity.Subject()
	require.NoError(t, err)
	cert, err := c.ca.GenerateCertificate(tlsca.CertificateRequest{
		Clock:     c.clock,
		PublicKey: pub,
		Subject:   subject,
		NotAfter:  c.clock.Now().Add(ttl),
	})
	require.NoError(t, err)
	return cert
}

// newTestKeyRing returns a KeyRing populated with a fresh TLS private key —
// enough for the validation helpers, which only inspect AccessGraphTLSCert
// and TLSPrivateKey.
func newTestKeyRing(t *testing.T) *client.KeyRing {
	t.Helper()
	tlsKey, err := cryptosuites.GenerateKeyWithAlgorithm(cryptosuites.ECDSAP256)
	require.NoError(t, err)
	tlsPriv, err := keys.NewPrivateKey(tlsKey)
	require.NoError(t, err)
	return &client.KeyRing{
		KeyRingIndex:  client.KeyRingIndex{Username: "alice", ClusterName: "test", ProxyHost: "proxy"},
		TLSPrivateKey: tlsPriv,
	}
}

// withTeleportTLSCert attaches a Teleport TLS cert to the keyring, signed by
// the test CA — required for issueAccessGraphCert, which derives the
// requested NotAfter from the keyring's existing Teleport cert.
func (c *testCA) withTeleportTLSCert(t *testing.T, kr *client.KeyRing, ttl time.Duration) *client.KeyRing {
	t.Helper()
	identity := tlsca.Identity{Username: kr.Username, Groups: []string{"access"}}
	subject, err := identity.Subject()
	require.NoError(t, err)
	cert, err := c.ca.GenerateCertificate(tlsca.CertificateRequest{
		Clock:     c.clock,
		PublicKey: kr.TLSPrivateKey.Public(),
		Subject:   subject,
		NotAfter:  c.clock.Now().Add(ttl),
	})
	require.NoError(t, err)
	kr.TLSCert = cert
	return kr
}

// mockAuthClient embeds *authclient.Client so unimplemented methods
// compile away; we only override what the tests touch. Mirrors mockClient
// in auth_command_test.go.
type mockAuthClient struct {
	*authclient.Client

	generate func(ctx context.Context, req proto.UserCertsRequest) (*proto.Certs, error)
	// ping defaults to a licensed-and-enabled response so the
	// precondition gate is a no-op for tests that don't care.
	ping func(ctx context.Context) (proto.PingResponse, error)

	gotReq *proto.UserCertsRequest
}

func (m *mockAuthClient) GenerateUserCerts(ctx context.Context, req proto.UserCertsRequest) (*proto.Certs, error) {
	m.gotReq = &req
	if m.generate == nil {
		return nil, errors.New("GenerateUserCerts not stubbed")
	}
	return m.generate(ctx, req)
}

func (m *mockAuthClient) Ping(ctx context.Context) (proto.PingResponse, error) {
	if m.ping != nil {
		return m.ping(ctx)
	}
	return pingResponseAccessGraphReady(), nil
}

// pingResponseAccessGraphReady is the canonical "licensed and enabled"
// PingResponse used as the default mock response.
func pingResponseAccessGraphReady() proto.PingResponse {
	return proto.PingResponse{
		ServerFeatures: &proto.Features{
			AccessGraph: true,
			Entitlements: map[string]*proto.EntitlementInfo{
				string(entitlements.Policy): {Enabled: true},
			},
		},
	}
}

func TestIssueAndStoreAccessGraphCert(t *testing.T) {
	t.Parallel()
	ca := newTestCA(t)

	keyRing := ca.withTeleportTLSCert(t, newTestKeyRing(t), time.Hour)
	store := client.NewMemClientStore()
	creds := &accessGraphCredentials{
		proxyAddr:   "proxy.example.com:443",
		clientStore: store,
		keyRing:     keyRing,
	}

	mock := &mockAuthClient{
		generate: func(ctx context.Context, req proto.UserCertsRequest) (*proto.Certs, error) {
			pub, err := keys.ParsePublicKey(req.TLSPublicKey)
			require.NoError(t, err)
			return &proto.Certs{
				TLS: ca.signAccessGraphCert(t, pub, time.Hour),
			}, nil
		},
	}

	require.NoError(t, issueAndStoreAccessGraphCert(context.Background(), creds, mock))

	// Auth client received the expected request.
	require.NotNil(t, mock.gotReq)
	require.Equal(t, "alice", mock.gotReq.Username)
	require.Equal(t, proto.UserCertsRequest_AccessGraphAPI, mock.gotReq.Usage)

	// The new cert is on the in-memory keyring and validates cleanly.
	require.NotEmpty(t, keyRing.AccessGraphTLSCert)
	require.True(t, validateAccessGraphCert(context.Background(), keyRing))

	// And it was persisted in the client store under the resolved cluster name.
	stored, err := store.GetKeyRing(client.KeyRingIndex{
		ProxyHost:   keyRing.ProxyHost,
		Username:    keyRing.Username,
		ClusterName: keyRing.ClusterName,
	})
	require.NoError(t, err)
	require.Equal(t, keyRing.AccessGraphTLSCert, stored.AccessGraphTLSCert)
}

func TestEnsureAccessGraphCert_ReuseSkipsClientInit(t *testing.T) {
	t.Parallel()
	ca := newTestCA(t)

	keyRing := newTestKeyRing(t)
	keyRing.AccessGraphTLSCert = ca.signAccessGraphCert(t, keyRing.TLSPrivateKey.Public(), time.Hour)

	creds := &accessGraphCredentials{
		proxyAddr:   "proxy.example.com:443",
		clientStore: client.NewMemClientStore(),
		keyRing:     keyRing,
	}

	// If the existing cert validates, ensureAccessGraphCert must not
	// initialize the (expensive) auth client. We track that by counting
	// how many times the InitFunc closure is called.
	var calls int
	clientFunc := commonclient.InitFunc(func(ctx context.Context) (*authclient.Client, func(context.Context), error) {
		calls++
		return nil, func(context.Context) {}, errors.New("InitFunc should not be called when reusing a valid cert")
	})

	require.NoError(t, ensureAccessGraphCert(context.Background(), creds, clientFunc))
	require.Zero(t, calls, "InitFunc must not be called when the existing cert is valid")
}

func TestResolveAccessGraphCredentials(t *testing.T) {
	t.Parallel()

	const (
		proxyHost = "proxy.example.com"
		username  = "alice"
		cluster   = "test-cluster"
	)

	// Fully populated profile pointing at a real keyring stored in an
	// in-memory client store.
	ca := newTestCA(t)
	newStoreAndProfile := func(t *testing.T) (*client.Store, *client.ProfileStatus) {
		t.Helper()
		store := client.NewMemClientStore()
		kr := ca.withTeleportTLSCert(t, newTestKeyRing(t), time.Hour)
		kr.KeyRingIndex = client.KeyRingIndex{
			ProxyHost:   proxyHost,
			Username:    username,
			ClusterName: cluster,
		}
		require.NoError(t, store.AddKeyRing(kr))
		profile := &client.ProfileStatus{
			Name:     proxyHost,
			Username: username,
			Cluster:  cluster,
			ProxyURL: url.URL{Scheme: "https", Host: proxyHost + ":443"},
		}
		return store, profile
	}

	t.Run("happy path", func(t *testing.T) {
		t.Parallel()
		store, profile := newStoreAndProfile(t)

		creds, err := resolveAccessGraphCredentials(context.Background(), store, profile)
		require.NoError(t, err)
		require.Equal(t, proxyHost+":443", creds.proxyAddr)
		require.Same(t, store, creds.clientStore)
		require.NotNil(t, creds.keyRing)
		require.Equal(t, username, creds.keyRing.Username)
		require.Equal(t, cluster, creds.keyRing.ClusterName)
	})

	t.Run("missing proxy URL host", func(t *testing.T) {
		t.Parallel()
		store, profile := newStoreAndProfile(t)
		profile.ProxyURL = url.URL{}

		_, err := resolveAccessGraphCredentials(context.Background(), store, profile)
		require.True(t, trace.IsNotFound(err), "expected NotFound, got %v", err)
	})

	t.Run("keyring not in store", func(t *testing.T) {
		t.Parallel()
		// Empty store — profile points at a keyring that doesn't exist.
		store := client.NewMemClientStore()
		profile := &client.ProfileStatus{
			Name:     proxyHost,
			Username: username,
			Cluster:  cluster,
			ProxyURL: url.URL{Scheme: "https", Host: proxyHost + ":443"},
		}

		_, err := resolveAccessGraphCredentials(context.Background(), store, profile)
		require.Error(t, err)
		require.True(t, trace.IsNotFound(err), "expected NotFound from GetKeyRing, got %v", err)
	})

	t.Run("uses profile.Name as ProxyHost", func(t *testing.T) {
		t.Parallel()
		// Verify the index uses profile.Name (host-only, profile filename)
		// rather than profile.ProxyURL.Host (which often includes a port).
		// A keyring stored under "host:port" would not be found.
		store := client.NewMemClientStore()
		kr := ca.withTeleportTLSCert(t, newTestKeyRing(t), time.Hour)
		kr.KeyRingIndex = client.KeyRingIndex{
			ProxyHost:   proxyHost + ":443", // wrong: includes port
			Username:    username,
			ClusterName: cluster,
		}
		require.NoError(t, store.AddKeyRing(kr))
		profile := &client.ProfileStatus{
			Name:     proxyHost,
			Username: username,
			Cluster:  cluster,
			ProxyURL: url.URL{Scheme: "https", Host: proxyHost + ":443"},
		}

		_, err := resolveAccessGraphCredentials(context.Background(), store, profile)
		require.Error(t, err, "lookup should miss when profile.Name disagrees with the stored ProxyHost")
	})
}

func TestEnsureAccessGraphCert_ReissuePathInvokesClient(t *testing.T) {
	t.Parallel()

	// Empty AccessGraphTLSCert forces re-issue. We inject an InitFunc that
	// returns an error so we can confirm the dispatch reached it without
	// needing a working *authclient.Client.
	creds := &accessGraphCredentials{
		proxyAddr:   "proxy.example.com:443",
		clientStore: client.NewMemClientStore(),
		keyRing:     newTestKeyRing(t),
	}

	sentinel := errors.New("init invoked")
	var calls int
	clientFunc := commonclient.InitFunc(func(ctx context.Context) (*authclient.Client, func(context.Context), error) {
		calls++
		return nil, nil, sentinel
	})

	err := ensureAccessGraphCert(context.Background(), creds, clientFunc)
	require.ErrorIs(t, err, sentinel)
	require.Equal(t, 1, calls)
}

func TestValidateAccessGraphCert(t *testing.T) {
	t.Parallel()
	ca := newTestCA(t)

	tests := []struct {
		name  string
		setup func(t *testing.T) *client.KeyRing
		want  bool
	}{
		{
			name: "no cert in keyring",
			setup: func(t *testing.T) *client.KeyRing {
				return newTestKeyRing(t)
			},
			want: false,
		},
		{
			name: "malformed cert bytes",
			setup: func(t *testing.T) *client.KeyRing {
				kr := newTestKeyRing(t)
				kr.AccessGraphTLSCert = []byte("not a certificate")
				return kr
			},
			want: false,
		},
		{
			name: "expired cert",
			setup: func(t *testing.T) *client.KeyRing {
				kr := newTestKeyRing(t)
				kr.AccessGraphTLSCert = ca.signAccessGraphCert(t, kr.TLSPrivateKey.Public(), -time.Minute)
				return kr
			},
			want: false,
		},
		{
			name: "cert public key does not match keyring private key",
			setup: func(t *testing.T) *client.KeyRing {
				kr := newTestKeyRing(t)
				otherKey, err := cryptosuites.GenerateKeyWithAlgorithm(cryptosuites.ECDSAP256)
				require.NoError(t, err)
				kr.AccessGraphTLSCert = ca.signAccessGraphCert(t, otherKey.Public(), time.Hour)
				return kr
			},
			want: false,
		},
		{
			name: "valid cert bound to keyring private key",
			setup: func(t *testing.T) *client.KeyRing {
				kr := newTestKeyRing(t)
				kr.AccessGraphTLSCert = ca.signAccessGraphCert(t, kr.TLSPrivateKey.Public(), time.Hour)
				return kr
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			kr := tt.setup(t)
			require.Equal(t, tt.want, validateAccessGraphCert(context.Background(), kr))
		})
	}
}

// TestCheckAccessGraphSupported asserts the trace error category and that
// each user-visible message names the missing piece and links to the right
// docs.
func TestCheckAccessGraphSupported(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		ping       proto.PingResponse
		pingErr    error // when non-nil, returned in place of ping
		wantErr    func(error) bool
		wantSubstr []string // every substring must appear in err.Error()
	}{
		{
			name: "Policy entitlement enabled → no error",
			ping: pingResponseAccessGraphReady(),
			wantErr: func(err error) bool {
				return err == nil
			},
		},
		{
			// Older clusters set only the legacy Policy submessage.
			name: "legacy Policy submessage enabled → no error",
			ping: proto.PingResponse{
				ServerFeatures: &proto.Features{
					AccessGraph: true,
					Policy:      &proto.PolicyFeature{Enabled: true},
				},
			},
			wantErr: func(err error) bool { return err == nil },
		},
		{
			name: "licensed but feature not enabled → AccessDenied, points at setup docs",
			ping: proto.PingResponse{
				ServerFeatures: &proto.Features{
					AccessGraph: false,
					Entitlements: map[string]*proto.EntitlementInfo{
						string(entitlements.Policy): {Enabled: true},
					},
				},
			},
			wantErr: trace.IsAccessDenied,
			wantSubstr: []string{
				"not configured",
				"access_graph",
				"teleport.yaml",
				accessGraphSetupDocURL,
			},
		},
		{
			name: "Policy entitlement explicitly disabled → AccessDenied",
			ping: proto.PingResponse{
				ServerFeatures: &proto.Features{
					AccessGraph: true,
					Entitlements: map[string]*proto.EntitlementInfo{
						string(entitlements.Policy): {Enabled: false},
					},
				},
			},
			wantErr: trace.IsAccessDenied,
			wantSubstr: []string{
				"Policy",
			},
		},
		{
			name: "entitlements map missing entirely and no legacy Policy → AccessDenied",
			ping: proto.PingResponse{
				ServerFeatures: &proto.Features{AccessGraph: true},
			},
			wantErr: trace.IsAccessDenied,
			wantSubstr: []string{
				"Policy",
			},
		},
		{
			// Regression guard: the `AccessGraph` entitlement key is
			// never populated by the modules code — only `Policy` is.
			name: "AccessGraph-key entitlement on its own is NOT sufficient — Policy is the gate",
			ping: proto.PingResponse{
				ServerFeatures: &proto.Features{
					AccessGraph: true,
					Entitlements: map[string]*proto.EntitlementInfo{
						string(entitlements.AccessGraph): {Enabled: true},
					},
				},
			},
			wantErr: trace.IsAccessDenied,
			wantSubstr: []string{
				"Policy",
			},
		},
		{
			name:    "ping itself fails — error is propagated with context",
			pingErr: errors.New("connection refused"),
			wantErr: func(err error) bool { return err != nil },
			wantSubstr: []string{
				"pinging cluster",
				"connection refused",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			mock := &mockAuthClient{
				ping: func(ctx context.Context) (proto.PingResponse, error) {
					if tt.pingErr != nil {
						return proto.PingResponse{}, tt.pingErr
					}
					return tt.ping, nil
				},
			}
			err := checkAccessGraphSupported(context.Background(), mock)
			require.True(t, tt.wantErr(err), "wantErr predicate failed for err=%v", err)
			if err != nil {
				for _, s := range tt.wantSubstr {
					require.Contains(t, err.Error(), s)
				}
			}
		})
	}
}

// TestCheckAccessGraphSupported_BlocksIssue confirms a failing
// precondition prevents GenerateUserCerts from being called.
func TestCheckAccessGraphSupported_BlocksIssue(t *testing.T) {
	t.Parallel()

	mock := &mockAuthClient{
		ping: func(ctx context.Context) (proto.PingResponse, error) {
			// Unlicensed cluster.
			return proto.PingResponse{
				ServerFeatures: &proto.Features{AccessGraph: true},
			}, nil
		},
		generate: func(ctx context.Context, req proto.UserCertsRequest) (*proto.Certs, error) {
			t.Fatalf("GenerateUserCerts must not be called when precondition check fails")
			return nil, nil
		},
	}

	require.True(t, trace.IsAccessDenied(checkAccessGraphSupported(context.Background(), mock)))
	require.Nil(t, mock.gotReq, "GenerateUserCerts must not have been invoked")
}
