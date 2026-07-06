// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package upstreamtls

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
)

func TestNewTLSCertPool(t *testing.T) {
	t.Parallel()
	const clusterName = "test-cluster"

	_, inlineCAPEM, err := tlsca.GenerateSelfSignedCA(
		pkix.Name{Organization: []string{"inline-ca"}}, nil, time.Hour,
	)
	require.NoError(t, err)
	_, spiffeCAPEM1, err := tlsca.GenerateSelfSignedCA(
		pkix.Name{Organization: []string{"spiffe-ca-1"}}, nil, time.Hour,
	)
	require.NoError(t, err)
	_, spiffeCAPEM2, err := tlsca.GenerateSelfSignedCA(
		pkix.Name{Organization: []string{"spiffe-ca-2"}}, nil, time.Hour,
	)
	require.NoError(t, err)

	spiffeCAID := types.CertAuthID{Type: types.SPIFFECA, DomainName: clusterName}

	for name, tc := range map[string]struct {
		cas          []string
		getter       AccessPoint
		expectedErr  require.ErrorAssertionFunc
		expectedPool require.ValueAssertionFunc
	}{
		"empty input returns nil pool": {
			cas:          nil,
			getter:       &fakeAccessPoint{},
			expectedErr:  require.NoError,
			expectedPool: require.Nil,
		},
		"inline PEM only": {
			cas:         []string{string(inlineCAPEM)},
			getter:      &fakeAccessPoint{},
			expectedErr: require.NoError,
			expectedPool: func(tt require.TestingT, i1 any, i2 ...any) {
				expected := x509.NewCertPool()
				require.True(tt, expected.AppendCertsFromPEM(inlineCAPEM), i2...)

				generatedPool, _ := i1.(*x509.CertPool)
				require.True(tt, generatedPool.Equal(expected), "cert pool contents differ")
			},
		},
		"alias resolves to single key pair": {
			cas: []string{types.AppTLSInternalCAWorkloadIdentity},
			getter: &fakeAccessPoint{cas: map[types.CertAuthID]types.CertAuthority{
				spiffeCAID: newSPIFFECertAuthority(t, clusterName, spiffeCAPEM1),
			}},
			expectedErr: require.NoError,
			expectedPool: func(tt require.TestingT, i1 any, i2 ...any) {
				expected := x509.NewCertPool()
				require.True(tt, expected.AppendCertsFromPEM(spiffeCAPEM1), i2...)

				generatedPool, _ := i1.(*x509.CertPool)
				require.True(tt, generatedPool.Equal(expected), "cert pool contents differ")
			},
		},
		"alias resolves to multiple key pairs (rotation)": {
			cas: []string{types.AppTLSInternalCAWorkloadIdentity},
			getter: &fakeAccessPoint{cas: map[types.CertAuthID]types.CertAuthority{
				spiffeCAID: newSPIFFECertAuthority(t, clusterName, spiffeCAPEM1, spiffeCAPEM2),
			}},
			expectedErr: require.NoError,
			expectedPool: func(tt require.TestingT, i1 any, i2 ...any) {
				expected := x509.NewCertPool()
				require.True(tt, expected.AppendCertsFromPEM(spiffeCAPEM1))
				require.True(tt, expected.AppendCertsFromPEM(spiffeCAPEM2))

				generatedPool, _ := i1.(*x509.CertPool)
				require.True(tt, generatedPool.Equal(expected), "cert pool contents differ")
			},
		},
		"alias resolves but CA has no TLS key pairs": {
			cas: []string{types.AppTLSInternalCAWorkloadIdentity},
			getter: &fakeAccessPoint{cas: map[types.CertAuthID]types.CertAuthority{
				spiffeCAID: newSPIFFECertAuthority(t, clusterName),
			}},
			expectedErr: require.NoError,
			expectedPool: func(tt require.TestingT, i1 any, i2 ...any) {
				generatedPool, _ := i1.(*x509.CertPool)
				require.NotNil(t, generatedPool)
				require.True(tt, generatedPool.Equal(x509.NewCertPool()), "expected empty cert pool")
			},
		},
		"alias getter error is propagated": {
			cas:          []string{types.AppTLSInternalCAWorkloadIdentity},
			getter:       &fakeAccessPoint{err: trace.NotFound("no SPIFFE CA")},
			expectedErr:  require.Error,
			expectedPool: require.Nil,
		},
		"inline PEM and alias combined": {
			cas: []string{string(inlineCAPEM), types.AppTLSInternalCAWorkloadIdentity},
			getter: &fakeAccessPoint{cas: map[types.CertAuthID]types.CertAuthority{
				spiffeCAID: newSPIFFECertAuthority(t, clusterName, spiffeCAPEM1),
			}},
			expectedErr: require.NoError,
			expectedPool: func(tt require.TestingT, i1 any, i2 ...any) {
				expected := x509.NewCertPool()
				require.True(tt, expected.AppendCertsFromPEM(inlineCAPEM))
				require.True(tt, expected.AppendCertsFromPEM(spiffeCAPEM1))

				generatedPool, _ := i1.(*x509.CertPool)
				require.True(tt, generatedPool.Equal(expected), "cert pool contents differ")
			},
		},
		"malformed PEM returns error": {
			cas:          []string{"not a pem"},
			getter:       &fakeAccessPoint{},
			expectedErr:  require.Error,
			expectedPool: require.Nil,
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			pool, err := newTLSCertPool(context.Background(), logtest.NewLogger(), tc.getter, clusterName, tc.cas)
			tc.expectedErr(t, err)
			tc.expectedPool(t, pool)
		})
	}
}

func TestIssueClientCertificates(t *testing.T) {
	t.Parallel()

	const userCert = "user-cert-raw"
	certExpiry := time.Unix(1700000000, 0).UTC()

	for name, tc := range map[string]struct {
		getUserCert    func() ([]byte, error)
		clt            *fakeIssuanceClient
		expectedErr    require.ErrorAssertionFunc
		expectedCerts  require.ValueAssertionFunc
		expectedExpiry require.ValueAssertionFunc
	}{
		"success": {
			getUserCert: func() ([]byte, error) { return []byte(userCert), nil },
			clt: &fakeIssuanceClient{
				resp: x509SVIDResponse(5*time.Minute, certExpiry, []byte("leaf"), []byte("intermediate")),
			},
			expectedErr: require.NoError,
			expectedCerts: func(tt require.TestingT, i1 any, i2 ...any) {
				require.IsType(tt, &tls.Certificate{}, i1, i2...)
				cert, _ := i1.(*tls.Certificate)
				require.Equal(t, [][]byte{[]byte("leaf"), []byte("intermediate")}, cert.Certificate)
				require.NotNil(t, cert.PrivateKey)
			},
			expectedExpiry: func(tt require.TestingT, i1 any, i2 ...any) {
				require.IsType(tt, time.Time{}, i1, i2...)
				notAfter, _ := i1.(time.Time)
				require.Equal(t, certExpiry, notAfter.UTC(), i2...)
			},
		},
		"get user cert error is propagated": {
			getUserCert:    func() ([]byte, error) { return nil, trace.AccessDenied("no cert") },
			clt:            &fakeIssuanceClient{resp: x509SVIDResponse(5*time.Minute, certExpiry, []byte("leaf"))},
			expectedErr:    require.Error,
			expectedCerts:  require.Nil,
			expectedExpiry: require.Zero,
		},
		"issuance error is propagated": {
			getUserCert:    func() ([]byte, error) { return []byte(userCert), nil },
			clt:            &fakeIssuanceClient{err: trace.ConnectionProblem(nil, "boom")},
			expectedErr:    require.Error,
			expectedCerts:  require.Nil,
			expectedExpiry: require.Zero,
		},
		"missing svid is rejected": {
			getUserCert: func() ([]byte, error) { return []byte(userCert), nil },
			clt: &fakeIssuanceClient{
				// Response with no X.509 SVID credential.
				resp: workloadidentityv1pb.IssueTeleportWorkloadIdentityResponse_builder{}.Build(),
			},
			expectedErr:    require.Error,
			expectedCerts:  require.Nil,
			expectedExpiry: require.Zero,
		},
		"empty svid cert is rejected": {
			getUserCert:    func() ([]byte, error) { return []byte(userCert), nil },
			clt:            &fakeIssuanceClient{resp: x509SVIDResponse(5*time.Minute, certExpiry, nil)},
			expectedErr:    require.Error,
			expectedCerts:  require.Nil,
			expectedExpiry: require.Zero,
		},
		"missing credential expiry is rejected": {
			getUserCert: func() ([]byte, error) { return []byte(userCert), nil },
			clt: &fakeIssuanceClient{
				// ExpiresAt absent. Note that AsTime() on a nil timestamp
				// returns the Unix epoch, not the zero time.Time.
				resp: workloadidentityv1pb.IssueTeleportWorkloadIdentityResponse_builder{
					Credential: workloadidentityv1pb.Credential_builder{
						X509Svid: workloadidentityv1pb.X509SVIDCredential_builder{
							Cert: []byte("leaf"),
						}.Build(),
					}.Build(),
				}.Build(),
			},
			expectedErr:    require.Error,
			expectedCerts:  require.Nil,
			expectedExpiry: require.Zero,
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			cert, notAfter, err := issueClientCertificate(t.Context(), Options{
				AccessPoint:                  &fakeAccessPoint{},
				ClusterName:                  "root.example.com",
				WorkloadIdentityClientGetter: fakeWorkloadIdentityClientGetter{clt: tc.clt},
				GetUserCertFunc:              tc.getUserCert,
			})
			tc.expectedErr(t, err)
			tc.expectedCerts(t, cert)
			tc.expectedExpiry(t, notAfter)
		})
	}
}

func TestNewGetClientCertFunc(t *testing.T) {
	t.Parallel()

	managedAppTLS := &types.AppTLS{ClientCertMode: types.AppClientCertModeManaged}
	newOptions := func(clt *fakeIssuanceClient) Options {
		return Options{
			Clock:                        clockwork.NewRealClock(),
			AccessPoint:                  &fakeAccessPoint{},
			ClusterName:                  "root.example.com",
			WorkloadIdentityClientGetter: fakeWorkloadIdentityClientGetter{clt: clt},
			GetUserCertFunc:              func() ([]byte, error) { return []byte("user-cert-raw"), nil },
		}
	}
	// CRI that accepts the ECDSA P-256 keys generated for AppClientCATLS.
	supportedCRI := &tls.CertificateRequestInfo{
		Version:          tls.VersionTLS13,
		SignatureSchemes: []tls.SignatureScheme{tls.ECDSAWithP256AndSHA256},
	}

	t.Run("non-managed mode returns nil func", func(t *testing.T) {
		t.Parallel()
		fn := newGetClientCertFunc(logtest.NewLogger(), newOptions(&fakeIssuanceClient{}), &types.AppTLS{})
		require.Nil(t, fn)
	})

	t.Run("certificate is issued on demand, not at construction", func(t *testing.T) {
		t.Parallel()
		clt := &fakeIssuanceClient{resp: x509SVIDResponse(5*time.Minute, time.Now().Add(time.Hour), []byte("leaf"))}
		fn := newGetClientCertFunc(logtest.NewLogger(), newOptions(clt), managedAppTLS)
		require.NotNil(t, fn)
		require.Zero(t, clt.calls.Load())

		cert, err := fn(supportedCRI)
		require.NoError(t, err)
		require.Equal(t, [][]byte{[]byte("leaf")}, cert.Certificate)
		require.Equal(t, int32(1), clt.calls.Load())
	})

	t.Run("valid certificate is reused", func(t *testing.T) {
		t.Parallel()
		clt := &fakeIssuanceClient{resp: x509SVIDResponse(5*time.Minute, time.Now().Add(time.Hour), []byte("leaf"))}
		fn := newGetClientCertFunc(logtest.NewLogger(), newOptions(clt), managedAppTLS)

		first, err := fn(supportedCRI)
		require.NoError(t, err)
		second, err := fn(supportedCRI)
		require.NoError(t, err)
		require.Same(t, first, second)
		require.Equal(t, int32(1), clt.calls.Load())
	})

	t.Run("expired certificate is reissued", func(t *testing.T) {
		t.Parallel()
		clt := &fakeIssuanceClient{resp: x509SVIDResponse(5*time.Minute, time.Now().Add(-time.Minute), []byte("leaf"))}
		fn := newGetClientCertFunc(logtest.NewLogger(), newOptions(clt), managedAppTLS)

		_, err := fn(supportedCRI)
		require.NoError(t, err)
		_, err = fn(supportedCRI)
		require.NoError(t, err)
		require.Equal(t, int32(2), clt.calls.Load())
	})

	t.Run("certificate expiring within the safety margin is reissued", func(t *testing.T) {
		t.Parallel()
		// Certificate is still valid, but its expiry falls inside the safety
		// margin, so it must not be served from the cache.
		clt := &fakeIssuanceClient{resp: x509SVIDResponse(5*time.Minute, time.Now().Add(clientCertExpiryMargin/2), []byte("leaf"))}
		fn := newGetClientCertFunc(logtest.NewLogger(), newOptions(clt), managedAppTLS)

		_, err := fn(supportedCRI)
		require.NoError(t, err)
		_, err = fn(supportedCRI)
		require.NoError(t, err)
		require.Equal(t, int32(2), clt.calls.Load())
	})

	t.Run("renewal failure on an expiring cached cert returns an error", func(t *testing.T) {
		t.Parallel()
		// First issuance succeeds, but the cert already sits within the safety
		// margin, so the next handshake must renew it rather than reuse it.
		clt := &fakeIssuanceClient{resp: x509SVIDResponse(5*time.Minute, time.Now().Add(clientCertExpiryMargin/2), []byte("leaf"))}
		fn := newGetClientCertFunc(logtest.NewLogger(), newOptions(clt), managedAppTLS)

		cert, err := fn(supportedCRI)
		require.NoError(t, err)
		require.Equal(t, [][]byte{[]byte("leaf")}, cert.Certificate)
		require.Equal(t, int32(1), clt.calls.Load())

		// Renewal fails. The cached cert is within the margin (effectively
		// expired), so the handshake must fail instead of presenting a stale
		// certificate.
		clt.err = trace.ConnectionProblem(nil, "boom")
		_, err = fn(supportedCRI)
		require.Error(t, err)
		require.Equal(t, int32(2), clt.calls.Load())
	})

	t.Run("issuance failures are returned and retried, not cached", func(t *testing.T) {
		t.Parallel()
		clt := &fakeIssuanceClient{err: trace.ConnectionProblem(nil, "boom")}
		fn := newGetClientCertFunc(logtest.NewLogger(), newOptions(clt), managedAppTLS)

		_, err := fn(supportedCRI)
		require.Error(t, err)
		_, err = fn(supportedCRI)
		require.Error(t, err)
		require.Equal(t, int32(2), clt.calls.Load())

		// Recovery: once issuance succeeds again, the handshake gets a cert.
		clt.err = nil
		clt.resp = x509SVIDResponse(5*time.Minute, time.Now().Add(time.Hour), []byte("leaf"))
		cert, err := fn(supportedCRI)
		require.NoError(t, err)
		require.Equal(t, [][]byte{[]byte("leaf")}, cert.Certificate)
	})

	t.Run("unsupported certificate sends empty certificate", func(t *testing.T) {
		t.Parallel()
		clt := &fakeIssuanceClient{resp: x509SVIDResponse(5*time.Minute, time.Now().Add(time.Hour), []byte("leaf"))}
		fn := newGetClientCertFunc(logtest.NewLogger(), newOptions(clt), managedAppTLS)

		// AcceptableCAs forces SupportsCertificate to inspect the chain, which
		// rejects the cert (mimicking a server requiring a different CA).
		cert, err := fn(&tls.CertificateRequestInfo{
			Version:          tls.VersionTLS13,
			SignatureSchemes: []tls.SignatureScheme{tls.ECDSAWithP256AndSHA256},
			AcceptableCAs:    [][]byte{[]byte("other-ca")},
		})
		require.NoError(t, err)
		require.NotNil(t, cert)
		// Empty certificate tells the TLS stack to send no client certificate.
		require.Empty(t, cert.Certificate)
	})

	t.Run("concurrent handshakes issue a single certificate", func(t *testing.T) {
		t.Parallel()
		clt := &fakeIssuanceClient{resp: x509SVIDResponse(5*time.Minute, time.Now().Add(time.Hour), []byte("leaf"))}
		fn := newGetClientCertFunc(logtest.NewLogger(), newOptions(clt), managedAppTLS)

		var wg sync.WaitGroup
		for range 10 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				cert, err := fn(supportedCRI)
				assert.NoError(t, err)
				assert.NotEmpty(t, cert.Certificate)
			}()
		}
		wg.Wait()
		require.Equal(t, int32(1), clt.calls.Load())
	})
}

// fakeAccessPoint is an accessPoint backed by an in-memory map.
type fakeAccessPoint struct {
	cas map[types.CertAuthID]types.CertAuthority
	err error
}

func (f *fakeAccessPoint) GetCertAuthority(_ context.Context, id types.CertAuthID, _ bool) (types.CertAuthority, error) {
	if f.err != nil {
		return nil, f.err
	}
	ca, ok := f.cas[id]
	if !ok {
		return nil, trace.NotFound("ca %v not found", id)
	}
	return ca, nil
}

func (f *fakeAccessPoint) GetAuthPreference(context.Context) (types.AuthPreference, error) {
	return types.DefaultAuthPreference(), nil
}

// newSPIFFECertAuthority builds a SPIFFE CertAuthority whose ActiveKeys.TLS
// holds one entry per provided cert PEM.
func newSPIFFECertAuthority(t *testing.T, clusterName string, certPEMs ...[]byte) types.CertAuthority {
	t.Helper()

	keyPairs := make([]*types.TLSKeyPair, 0, len(certPEMs))
	for _, certPEM := range certPEMs {
		keyPairs = append(keyPairs, &types.TLSKeyPair{Cert: certPEM})
	}
	ca, err := types.NewCertAuthority(types.CertAuthoritySpecV2{
		Type:        types.SPIFFECA,
		ClusterName: clusterName,
		ActiveKeys:  types.CAKeySet{TLS: keyPairs},
	})
	require.NoError(t, err)
	return ca
}

type fakeIssuanceClient struct {
	workloadidentityv1pb.WorkloadIdentityIssuanceServiceClient

	calls atomic.Int32
	resp  *workloadidentityv1pb.IssueTeleportWorkloadIdentityResponse
	err   error
}

func (f *fakeIssuanceClient) IssueTeleportWorkloadIdentity(_ context.Context, in *workloadidentityv1pb.IssueTeleportWorkloadIdentityRequest, _ ...grpc.CallOption) (*workloadidentityv1pb.IssueTeleportWorkloadIdentityResponse, error) {
	f.calls.Add(1)
	if f.err != nil {
		return nil, f.err
	}
	return f.resp, nil
}

type fakeWorkloadIdentityClientGetter struct {
	clt workloadidentityv1pb.WorkloadIdentityIssuanceServiceClient
}

func (f fakeWorkloadIdentityClientGetter) WorkloadIdentityIssuanceClient() workloadidentityv1pb.WorkloadIdentityIssuanceServiceClient {
	return f.clt
}

// x509SVIDResponse builds an issuance response carrying an X.509 SVID with the
// provided expiry, leaf certificate, and chain.
func x509SVIDResponse(ttl time.Duration, expiresAt time.Time, cert []byte, chain ...[]byte) *workloadidentityv1pb.IssueTeleportWorkloadIdentityResponse {
	return workloadidentityv1pb.IssueTeleportWorkloadIdentityResponse_builder{
		Credential: workloadidentityv1pb.Credential_builder{
			Ttl:       durationpb.New(ttl),
			ExpiresAt: timestamppb.New(expiresAt),
			X509Svid: workloadidentityv1pb.X509SVIDCredential_builder{
				Cert:  cert,
				Chain: chain,
			}.Build(),
		}.Build(),
	}.Build()
}
