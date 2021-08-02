/*
Copyright 2021 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package services

import (
	"crypto/x509/pkix"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/tlsca"
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

	// CA for cluster3 with old schema
	key, cert, err = tlsca.GenerateSelfSignedCA(pkix.Name{CommonName: "cluster3"}, nil, time.Minute)
	require.NoError(t, err)
	ca3, err := types.NewCertAuthority(types.CertAuthoritySpecV2{
		Type:        types.HostCA,
		ClusterName: "cluster3",
		TLSKeyPairs: []types.TLSKeyPair{{
			Cert: cert,
			Key:  key,
		}},
	})
	require.NoError(t, err)

	t.Run("ca1 with 1 cert", func(t *testing.T) {
		pool, err := CertPoolFromCertAuthorities([]types.CertAuthority{ca1})
		require.NoError(t, err)
		require.Len(t, pool.Subjects(), 1)
	})
	t.Run("ca2 with 2 certs", func(t *testing.T) {
		pool, err := CertPoolFromCertAuthorities([]types.CertAuthority{ca2})
		require.NoError(t, err)
		require.Len(t, pool.Subjects(), 2)
	})
	t.Run("ca3 with 1 cert", func(t *testing.T) {
		pool, err := CertPoolFromCertAuthorities([]types.CertAuthority{ca3})
		require.NoError(t, err)
		require.Len(t, pool.Subjects(), 1)
	})

	t.Run("ca1 + ca2 + ca3 with 4 certs total", func(t *testing.T) {
		pool, err := CertPoolFromCertAuthorities([]types.CertAuthority{ca1, ca2, ca3})
		require.NoError(t, err)
		require.Len(t, pool.Subjects(), 4)
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

	// CAs that differ *only* by resource ID are equivalent
	ca1modID := ca1.Clone()
	ca1modID.SetResourceID(ca1.GetResourceID() + 1)
	require.True(t, CertAuthoritiesEquivalent(ca1, ca1modID))
}
