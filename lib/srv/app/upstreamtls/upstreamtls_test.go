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
	"crypto/x509"
	"crypto/x509/pkix"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

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
		getter       CertificateAuthorityGetter
		expectedErr  require.ErrorAssertionFunc
		expectedPool require.ValueAssertionFunc
	}{
		"empty input returns nil pool": {
			cas:          nil,
			getter:       &fakeCAGetter{},
			expectedErr:  require.NoError,
			expectedPool: require.Nil,
		},
		"inline PEM only": {
			cas:         []string{string(inlineCAPEM)},
			getter:      &fakeCAGetter{},
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
			getter: &fakeCAGetter{cas: map[types.CertAuthID]types.CertAuthority{
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
			getter: &fakeCAGetter{cas: map[types.CertAuthID]types.CertAuthority{
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
			getter: &fakeCAGetter{cas: map[types.CertAuthID]types.CertAuthority{
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
			getter:       &fakeCAGetter{err: trace.NotFound("no SPIFFE CA")},
			expectedErr:  require.Error,
			expectedPool: require.Nil,
		},
		"inline PEM and alias combined": {
			cas: []string{string(inlineCAPEM), types.AppTLSInternalCAWorkloadIdentity},
			getter: &fakeCAGetter{cas: map[types.CertAuthID]types.CertAuthority{
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
			getter:       &fakeCAGetter{},
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

// fakeCAGetter is a CertificateAuthorityGetter backed by an in-memory map.
type fakeCAGetter struct {
	cas map[types.CertAuthID]types.CertAuthority
	err error
}

func (f *fakeCAGetter) GetCertAuthority(_ context.Context, id types.CertAuthID, _ bool) (types.CertAuthority, error) {
	if f.err != nil {
		return nil, f.err
	}
	ca, ok := f.cas[id]
	if !ok {
		return nil, trace.NotFound("ca %v not found", id)
	}
	return ca, nil
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
