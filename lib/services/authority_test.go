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

package services_test

import (
	"bytes"
	"crypto/x509/pkix"
	"testing"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/testauthority"
	. "github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

func TestCertPoolFromCertAuthorities(t *testing.T) {
	// CA for cluster1 with 1 key pair.
	key, cert, err := tlsca.GenerateSelfSignedCA(pkix.Name{CommonName: "cluster1"}, nil, time.Minute)
	require.NoError(t, err)
	ca1, err := types.NewCertAuthority(types.CertAuthoritySpecV2{
		Type:        types.HostCA,
		ClusterName: "cluster1",
		ActiveKeys: types.CAKeySet{
			TLS: []*types.TLSKeyPair{{
				Cert: cert,
				Key:  key,
			}},
		},
	})
	require.NoError(t, err)

	// CA for cluster2 with 2 key pairs.
	key, cert, err = tlsca.GenerateSelfSignedCA(pkix.Name{CommonName: "cluster2"}, nil, time.Minute)
	require.NoError(t, err)
	key2, cert2, err := tlsca.GenerateSelfSignedCA(pkix.Name{CommonName: "cluster2"}, nil, time.Minute)
	require.NoError(t, err)
	ca2, err := types.NewCertAuthority(types.CertAuthoritySpecV2{
		Type:        types.HostCA,
		ClusterName: "cluster2",
		ActiveKeys: types.CAKeySet{
			TLS: []*types.TLSKeyPair{
				{
					Cert: cert,
					Key:  key,
				},
				{
					Cert: cert2,
					Key:  key2,
				},
			},
		},
	})
	require.NoError(t, err)

	t.Run("ca1 with 1 cert", func(t *testing.T) {
		pool, count, err := CertPoolFromCertAuthorities([]types.CertAuthority{ca1})
		require.NotNil(t, pool)
		require.NoError(t, err)
		require.Equal(t, 1, count)
	})
	t.Run("ca2 with 2 certs", func(t *testing.T) {
		pool, count, err := CertPoolFromCertAuthorities([]types.CertAuthority{ca2})
		require.NotNil(t, pool)
		require.NoError(t, err)
		require.Equal(t, 2, count)
	})

	t.Run("ca1 + ca2 with 3 certs total", func(t *testing.T) {
		pool, count, err := CertPoolFromCertAuthorities([]types.CertAuthority{ca1, ca2})
		require.NotNil(t, pool)
		require.NoError(t, err)
		require.Equal(t, 3, count)
	})
}

func TestCertAuthorityEquivalence(t *testing.T) {
	// CA for cluster1 with 1 key pair.
	key, cert, err := tlsca.GenerateSelfSignedCA(pkix.Name{CommonName: "cluster1"}, nil, time.Minute)
	require.NoError(t, err)
	ca1, err := types.NewCertAuthority(types.CertAuthoritySpecV2{
		Type:        types.HostCA,
		ClusterName: "cluster1",
		ActiveKeys: types.CAKeySet{
			TLS: []*types.TLSKeyPair{{
				Cert: cert,
				Key:  key,
			}},
		},
	})
	require.NoError(t, err)

	// CA for cluster2 with 2 key pairs.
	key, cert, err = tlsca.GenerateSelfSignedCA(pkix.Name{CommonName: "cluster2"}, nil, time.Minute)
	require.NoError(t, err)
	key2, cert2, err := tlsca.GenerateSelfSignedCA(pkix.Name{CommonName: "cluster2"}, nil, time.Minute)
	require.NoError(t, err)
	ca2, err := types.NewCertAuthority(types.CertAuthoritySpecV2{
		Type:        types.HostCA,
		ClusterName: "cluster2",
		ActiveKeys: types.CAKeySet{
			TLS: []*types.TLSKeyPair{
				{
					Cert: cert,
					Key:  key,
				},
				{
					Cert: cert2,
					Key:  key2,
				},
			},
		},
	})
	require.NoError(t, err)

	// different CAs are different
	require.False(t, ca1.IsEqual(ca2))

	// two copies of same CA are equivalent
	require.True(t, ca1.IsEqual(ca1.Clone()))

	// CAs with same name but different details are different
	ca1mod := ca1.Clone()
	ca1mod.AddRole("some-new-role")
	require.False(t, ca1.IsEqual(ca1mod))
}

func TestCertAuthorityUTCUnmarshal(t *testing.T) {
	t.Parallel()

	_, pub, err := testauthority.GenerateKeyPair()
	require.NoError(t, err)
	_, cert, err := tlsca.GenerateSelfSignedCA(pkix.Name{CommonName: "clustername"}, nil, time.Hour)
	require.NoError(t, err)

	caLocal, err := types.NewCertAuthority(types.CertAuthoritySpecV2{
		Type:        types.HostCA,
		ClusterName: "clustername",
		ActiveKeys: types.CAKeySet{
			SSH: []*types.SSHKeyPair{{PublicKey: pub}},
			TLS: []*types.TLSKeyPair{{Cert: cert}},
		},
		Rotation: &types.Rotation{
			LastRotated: time.Now().In(time.FixedZone("not UTC", 2*60*60)),
		},
	})
	require.NoError(t, err)

	_, offset := caLocal.GetRotation().LastRotated.Zone()
	require.NotZero(t, offset)

	item, err := utils.FastMarshal(caLocal)
	require.NoError(t, err)
	require.Contains(t, string(item), "+02:00\"")
	caUTC, err := UnmarshalCertAuthority(item)
	require.NoError(t, err)

	_, offset = caUTC.GetRotation().LastRotated.Zone()
	require.Zero(t, offset)

	// see https://github.com/gogo/protobuf/issues/519
	require.NotPanics(t, func() { caUTC.Clone() })

	require.True(t, caLocal.IsEqual(caUTC))
}

func TestValidateCertAuthority(t *testing.T) {
	t.Parallel()

	clone := func(spec *types.CertAuthoritySpecV2) *types.CertAuthoritySpecV2 {
		return proto.Clone(spec).(*types.CertAuthoritySpecV2)
	}

	const clusterName = "zarquon"
	keyPEM, certPEM, err := tlsca.GenerateSelfSignedCA(
		pkix.Name{CommonName: clusterName},
		nil,         /* dnsNames */
		time.Minute, /* ttl */
	)
	require.NoError(t, err)

	jwtPubPEM, jwtPrivPEM, err := testauthority.GenerateJWT()
	require.NoError(t, err)

	winCA := &types.CertAuthoritySpecV2{
		Type:        types.WindowsCA,
		ClusterName: clusterName,
		ActiveKeys: types.CAKeySet{
			TLS: []*types.TLSKeyPair{
				{
					Cert:    certPEM,
					Key:     keyPEM,
					KeyType: types.PrivateKeyType_RAW,
				},
			},
		},
	}

	samlIDPCA := &types.CertAuthoritySpecV2{
		Type:        types.SAMLIDPCA,
		ClusterName: clusterName,
		ActiveKeys: types.CAKeySet{
			TLS: []*types.TLSKeyPair{
				{
					// OK to share cert/keys for this test, the CAs are tested
					// independently.
					Cert:    certPEM,
					Key:     keyPEM,
					KeyType: types.PrivateKeyType_RAW,
				},
			},
		},
	}

	oidcIDPCA := &types.CertAuthoritySpecV2{
		Type:        types.OIDCIdPCA,
		ClusterName: clusterName,
		ActiveKeys: types.CAKeySet{
			JWT: []*types.JWTKeyPair{
				{
					PublicKey:      jwtPubPEM,
					PrivateKey:     jwtPrivPEM,
					PrivateKeyType: types.PrivateKeyType_RAW,
				},
			},
		},
	}

	// Add your new CA to the table below!
	// If you are using checkTLSKeys() you don't need to duplicate every test case
	// of WindowsCA, just add a single test that exercises that function call.

	tests := []struct {
		name    string
		spec    *types.CertAuthoritySpecV2
		wantErr string
	}{
		// WindowsCA.
		{
			name: "valid WindowsCA",
			spec: winCA,
		},
		{
			name: "valid WindowsCA (Cert only)",
			spec: func() *types.CertAuthoritySpecV2 {
				spec := clone(winCA)
				spec.ActiveKeys.TLS = []*types.TLSKeyPair{
					{Cert: certPEM},
				}
				return spec
			}(),
		},
		{
			name: "valid WindowsCA (multiple active keys)",
			spec: func() *types.CertAuthoritySpecV2 {
				key2, cert2, err := tlsca.GenerateSelfSignedCA(
					pkix.Name{CommonName: clusterName},
					nil,         /* dnsNames */
					time.Minute, /* ttl */
				)
				require.NoError(t, err)

				spec := clone(winCA)
				spec.ActiveKeys.TLS = append(spec.ActiveKeys.TLS, &types.TLSKeyPair{
					Cert: cert2,
					Key:  key2,
				})
				return spec
			}(),
		},
		{
			name: "valid WindowsCA (non-RAW private key)",
			spec: func() *types.CertAuthoritySpecV2 {
				spec := clone(winCA)
				spec.ActiveKeys.TLS = []*types.TLSKeyPair{
					{
						Cert:    certPEM,
						Key:     []byte(`ceci n'est pas a RAW key`),
						KeyType: types.PrivateKeyType_PKCS11,
					},
				}
				return spec
			}(),
		},
		{
			name: "WindowsCA empty TLS keys",
			spec: func() *types.CertAuthoritySpecV2 {
				spec := clone(winCA)
				spec.ActiveKeys.TLS = nil
				return spec
			}(),
			wantErr: "missing TLS",
		},
		{
			name: "WindowsCA TLS key invalid",
			spec: func() *types.CertAuthoritySpecV2 {
				spec := clone(winCA)
				spec.ActiveKeys.TLS = []*types.TLSKeyPair{
					{
						Cert: certPEM,
						Key:  []byte("ceci n'est pas a private key"),
					},
				}
				return spec
			}(),
			wantErr: "private key and certificate",
		},
		{
			name: "WindowsCA TLS cert invalid (Cert and Key)",
			spec: func() *types.CertAuthoritySpecV2 {
				spec := clone(winCA)
				spec.ActiveKeys.TLS = []*types.TLSKeyPair{
					{
						Cert: []byte("ceci n'est pas a certificate"),
						Key:  keyPEM,
					},
				}
				return spec
			}(),
			wantErr: "private key and certificate",
		},
		{
			name: "WindowsCA TLS cert invalid (Cert only)",
			spec: func() *types.CertAuthoritySpecV2 {
				spec := clone(winCA)
				spec.ActiveKeys.TLS = []*types.TLSKeyPair{
					{
						Cert: []byte("ceci n'est pas a certificate"),
					},
				}
				return spec
			}(),
			wantErr: "certificate",
		},

		// SAMLIDPCA.
		{
			name: "valid SAMLIDPCA",
			spec: samlIDPCA,
		},
		{
			// WindowsCA already covers all corner-cases of checkTLSKeys.
			name: "SAMLIDPCA invalid ActiveKeys",
			spec: func() *types.CertAuthoritySpecV2 {
				spec := clone(samlIDPCA)
				spec.ActiveKeys = types.CAKeySet{}
				return spec
			}(),
			wantErr: "missing TLS",
		},

		// OIDCIdPCA.
		{
			name: "valid OIDCIdPCA",
			spec: oidcIDPCA,
		},
		{
			name: "valid OIDCIdPCA (multiple active keys)",
			spec: func() *types.CertAuthoritySpecV2 {
				// GenerateJWT() returns hard-coded keys, so this set is the same as
				// jwtPubPEM/jwtPrivPEM.
				// This is OK for this test and we retain the call to GenerateJWT() in
				// case it ever returns distinct keys.
				pub2, priv2, err := testauthority.GenerateJWT()
				require.NoError(t, err)

				spec := clone(oidcIDPCA)
				spec.ActiveKeys.JWT = append(spec.ActiveKeys.JWT, &types.JWTKeyPair{
					PublicKey:      pub2,
					PrivateKey:     priv2,
					PrivateKeyType: types.PrivateKeyType_RAW,
				})
				return spec
			}(),
		},
		{
			name: "valid OIDCIdPCA (empty private key)",
			spec: func() *types.CertAuthoritySpecV2 {
				spec := clone(oidcIDPCA)
				spec.ActiveKeys.JWT = []*types.JWTKeyPair{
					{
						PublicKey: jwtPubPEM,
					},
				}
				return spec
			}(),
		},
		{
			name: "OIDCIdPCA JWT PrivateKey invalid",
			spec: func() *types.CertAuthoritySpecV2 {
				spec := clone(oidcIDPCA)
				spec.ActiveKeys.JWT = []*types.JWTKeyPair{
					{
						PublicKey:      jwtPubPEM,
						PrivateKey:     []byte(`ceci n'est pas a private key`),
						PrivateKeyType: types.PrivateKeyType_RAW,
					},
				}
				return spec
			}(),
			wantErr: "private key",
		},
		{
			name: "OIDCIdPCA JWT PublicKey invalid",
			spec: func() *types.CertAuthoritySpecV2 {
				spec := clone(oidcIDPCA)
				spec.ActiveKeys.JWT = []*types.JWTKeyPair{
					{
						PublicKey:      []byte(`ceci n'est pas a public key`),
						PrivateKey:     jwtPrivPEM,
						PrivateKeyType: types.PrivateKeyType_RAW,
					},
				}
				return spec
			}(),
			wantErr: "public key",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			ca, err := types.NewCertAuthority(*test.spec)
			require.NoError(t, err)

			err = ValidateCertAuthority(ca)
			if test.wantErr != "" {
				assert.ErrorContains(t, err, test.wantErr, "ValidateCertAuthority")
				return
			}
			assert.NoError(t, err, "ValidateCertAuthority")
		})
	}
}

func BenchmarkCertAuthoritiesEquivalent(b *testing.B) {
	ca1, err := types.NewCertAuthority(types.CertAuthoritySpecV2{
		Type:        types.HostCA,
		ClusterName: "cluster1",
		ActiveKeys: types.CAKeySet{
			TLS: []*types.TLSKeyPair{{
				Cert: bytes.Repeat([]byte{49}, 1600),
				Key:  bytes.Repeat([]byte{49}, 1200),
			}},
		},
	})
	require.NoError(b, err)

	// ca2 is a clone of ca1.
	ca2 := ca1.Clone()

	// ca3 has different cert bytes from ca1.
	ca3, err := types.NewCertAuthority(types.CertAuthoritySpecV2{
		Type:        types.HostCA,
		ClusterName: "cluster1",
		ActiveKeys: types.CAKeySet{
			TLS: []*types.TLSKeyPair{{
				Cert: bytes.Repeat([]byte{49}, 1666),
				Key:  bytes.Repeat([]byte{49}, 1222),
			}},
		},
	})
	require.NoError(b, err)

	b.Run("true", func(b *testing.B) {
		for b.Loop() {
			require.True(b, ca1.IsEqual(ca2))
		}
	})

	b.Run("false", func(b *testing.B) {
		for b.Loop() {
			require.False(b, ca1.IsEqual(ca3))
		}
	})
}
