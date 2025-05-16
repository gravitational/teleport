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
	require.False(t, CertAuthoritiesEquivalent(ca1, ca2))

	// two copies of same CA are equivalent
	require.True(t, CertAuthoritiesEquivalent(ca1, ca1.Clone()))

	// CAs with same name but different details are different
	ca1mod := ca1.Clone()
	ca1mod.AddRole("some-new-role")
	require.False(t, CertAuthoritiesEquivalent(ca1, ca1mod))
}

func TestCertAuthorityUTCUnmarshal(t *testing.T) {
	t.Parallel()

	_, pub, err := testauthority.New().GenerateKeyPair()
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

	require.True(t, CertAuthoritiesEquivalent(caLocal, caUTC))
}

func TestCheckSAMLIDPCA(t *testing.T) {
	// Create testing CA.
	key, cert, err := tlsca.GenerateSelfSignedCA(pkix.Name{CommonName: "cluster1"}, nil, time.Minute)
	require.NoError(t, err)

	tests := []struct {
		name             string
		keyset           types.CAKeySet
		errAssertionFunc require.ErrorAssertionFunc
	}{
		{
			name:             "no active keys",
			keyset:           types.CAKeySet{},
			errAssertionFunc: require.Error,
		},
		{
			name: "multiple active keys",
			keyset: types.CAKeySet{
				TLS: []*types.TLSKeyPair{{
					Cert: cert,
					Key:  key,
				}, {
					Cert: cert,
					Key:  key,
				}},
			},
			errAssertionFunc: require.NoError,
		},
		{
			name: "empty key",
			keyset: types.CAKeySet{
				TLS: []*types.TLSKeyPair{{
					Cert: cert,
					Key:  []byte{},
				}},
			},
			errAssertionFunc: require.NoError,
		},
		{
			name: "unparseable key",
			keyset: types.CAKeySet{
				TLS: []*types.TLSKeyPair{{
					Cert: cert,
					Key:  bytes.Repeat([]byte{49}, 1222),
				}},
			},
			errAssertionFunc: require.Error,
		},
		{
			name: "unparseable cert",
			keyset: types.CAKeySet{
				TLS: []*types.TLSKeyPair{{
					Cert: bytes.Repeat([]byte{49}, 1222),
					Key:  key,
				}},
			},
			errAssertionFunc: require.Error,
		},
		{
			name: "valid CA",
			keyset: types.CAKeySet{
				TLS: []*types.TLSKeyPair{{
					Cert: cert,
					Key:  key,
				}},
			},
			errAssertionFunc: require.NoError,
		},
		{
			name: "don't validate non-raw private keys",
			keyset: types.CAKeySet{
				TLS: []*types.TLSKeyPair{{
					Cert:    cert,
					Key:     bytes.Repeat([]byte{49}, 1222),
					KeyType: types.PrivateKeyType_PKCS11,
				}},
			},
			errAssertionFunc: require.NoError,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ca, err := types.NewCertAuthority(types.CertAuthoritySpecV2{
				Type:        types.SAMLIDPCA,
				ClusterName: "cluster1",
				ActiveKeys:  test.keyset,
			})
			require.NoError(t, err)
			test.errAssertionFunc(t, ValidateCertAuthority(ca))
		})
	}
}

func TestCheckOIDCIdP(t *testing.T) {
	ta := testauthority.New()

	pub, priv, err := ta.GenerateJWT()
	require.NoError(t, err)

	pub2, priv2, err := ta.GenerateJWT()
	require.NoError(t, err)

	tests := []struct {
		name             string
		keyset           types.CAKeySet
		errAssertionFunc require.ErrorAssertionFunc
	}{
		{
			name:             "no active keys",
			keyset:           types.CAKeySet{},
			errAssertionFunc: require.Error,
		},
		{
			name: "multiple active keys",
			keyset: types.CAKeySet{
				JWT: []*types.JWTKeyPair{
					{
						PublicKey:  pub,
						PrivateKey: priv,
					},
					{
						PublicKey:  pub2,
						PrivateKey: priv2,
					},
				},
			},
			errAssertionFunc: require.NoError,
		},
		{
			name: "empty private key",
			keyset: types.CAKeySet{
				JWT: []*types.JWTKeyPair{{
					PublicKey:  pub,
					PrivateKey: []byte{},
				}},
			},
			errAssertionFunc: require.NoError,
		},
		{
			name: "unparseable private key",
			keyset: types.CAKeySet{
				JWT: []*types.JWTKeyPair{{
					PublicKey:  pub,
					PrivateKey: bytes.Repeat([]byte{49}, 1222),
				}},
			},
			errAssertionFunc: require.Error,
		},
		{
			name: "unparseable public key",
			keyset: types.CAKeySet{
				JWT: []*types.JWTKeyPair{{
					PublicKey:  bytes.Repeat([]byte{49}, 1222),
					PrivateKey: priv,
				}},
			},
			errAssertionFunc: require.Error,
		},
		{
			name: "valid key pair",
			keyset: types.CAKeySet{
				JWT: []*types.JWTKeyPair{{
					PublicKey:  pub,
					PrivateKey: priv,
				}},
			},
			errAssertionFunc: require.NoError,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ca, err := types.NewCertAuthority(types.CertAuthoritySpecV2{
				Type:        types.OIDCIdPCA,
				ClusterName: "cluster1",
				ActiveKeys:  test.keyset,
			})
			require.NoError(t, err)
			test.errAssertionFunc(t, ValidateCertAuthority(ca))
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
		for i := 0; i < b.N; i++ {
			require.True(b, CertAuthoritiesEquivalent(ca1, ca2))
		}
	})

	b.Run("false", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			require.False(b, CertAuthoritiesEquivalent(ca1, ca3))
		}
	})
}
