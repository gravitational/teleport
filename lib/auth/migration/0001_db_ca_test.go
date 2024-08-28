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

package migration

import (
	"context"
	"crypto/x509/pkix"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/tlsca"
)

func TestDBAuthorityDown(t *testing.T) {
	b, err := memory.New(memory.Config{EventsOff: true})
	require.NoError(t, err)
	svc := local.NewCAService(b)

	migration := createDBAuthority{}

	ctx := context.Background()

	// create a CA for all types except DB
	for _, caType := range types.CertAuthTypes {
		if caType == types.DatabaseCA {
			continue
		}
		require.NoError(t, svc.CreateCertAuthority(ctx, newCertAuthority(t, uuid.NewString(), caType)))
	}

	t.Run("down succeeds when no db cas exists", func(t *testing.T) {
		require.NoError(t, migration.Down(ctx, b))

		// validate all other CA types still exist
		for _, caType := range types.CertAuthTypes {
			if caType == types.DatabaseCA {
				continue
			}

			cas, err := svc.GetCertAuthorities(ctx, caType, false)
			require.NoError(t, err)
			require.Len(t, cas, 1)
		}
	})

	t.Run("down removes db cas", func(t *testing.T) {
		require.NoError(t, svc.CreateCertAuthority(ctx, newCertAuthority(t, uuid.NewString(), types.DatabaseCA)))

		require.NoError(t, migration.Down(ctx, b))

		// validate all other CA types still exist
		for _, caType := range types.CertAuthTypes {
			cas, err := svc.GetCertAuthorities(ctx, caType, false)
			require.NoError(t, err)
			if caType == types.DatabaseCA {
				require.Empty(t, cas)
			} else {
				require.Len(t, cas, 1)
			}
		}
	})
}

func newCertAuthority(t *testing.T, name string, caType types.CertAuthType) types.CertAuthority {
	ta := testauthority.New()
	priv, pub, err := ta.GenerateKeyPair()
	require.NoError(t, err)

	// CA for cluster1 with 1 key pair.
	key, cert, err := tlsca.GenerateSelfSignedCA(pkix.Name{CommonName: name}, nil, time.Minute)
	require.NoError(t, err)

	jwtPub, jwtPriv, err := ta.GenerateJWT()
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
			JWT: []*types.JWTKeyPair{{
				PublicKey:  jwtPub,
				PrivateKey: jwtPriv,
			}},
		},
	})
	require.NoError(t, err)
	return ca
}

func TestDBAuthorityUp(t *testing.T) {
	fakeCA := &types.CertAuthorityV2{
		Kind: types.KindCertAuthority,
		Spec: types.CertAuthoritySpecV2{
			ActiveKeys: types.CAKeySet{
				TLS: []*types.TLSKeyPair{
					{
						Cert: []byte{0, 1, 2},
						Key:  []byte{3, 4, 5},
					},
				},
			},
		},
	}

	rootDB := types.CertAuthID{
		Type:       types.DatabaseCA,
		DomainName: "root",
	}

	rootHost := types.CertAuthID{
		Type:       types.HostCA,
		DomainName: "root",
	}

	leaf1DB := types.CertAuthID{
		Type:       types.DatabaseCA,
		DomainName: "leaf1",
	}

	leaf1Host := types.CertAuthID{
		Type:       types.HostCA,
		DomainName: "leaf1",
	}

	leaf2DB := types.CertAuthID{
		Type:       types.DatabaseCA,
		DomainName: "leaf2",
	}

	leaf2Host := types.CertAuthID{
		Type:       types.HostCA,
		DomainName: "leaf2",
	}

	clusterName := func(name string) types.ClusterName {
		return &types.ClusterNameV2{
			Kind: types.KindClusterName,
			Spec: types.ClusterNameSpecV2{
				ClusterName: name,
				ClusterID:   "test",
			},
		}
	}

	cases := []struct {
		name         string
		fakeTrust    *fakeTrust
		assertion    require.ErrorAssertionFunc
		validateFunc func(t *testing.T, created []types.CertAuthority)
	}{
		{
			name: "db cas created from host cas",
			fakeTrust: &fakeTrust{
				authorities: map[types.CertAuthID]types.CertAuthority{
					rootHost:  fakeCA,
					leaf1Host: fakeCA,
					leaf1DB:   fakeCA,
					leaf2Host: fakeCA,
				},
				clusters: []types.TrustedCluster{
					&types.TrustedClusterV2{
						Kind:     types.KindTrustedCluster,
						Metadata: types.Metadata{Name: "leaf1"},
						Spec: types.TrustedClusterSpecV2{
							Enabled: true,
						},
					}, &types.TrustedClusterV2{
						Kind:     types.KindTrustedCluster,
						Metadata: types.Metadata{Name: "leaf2"},
						Spec:     types.TrustedClusterSpecV2{},
					},
				},
			},
			assertion: require.NoError,
			validateFunc: func(t *testing.T, created []types.CertAuthority) {
				require.Len(t, created, 2)
				require.Equal(t, types.DatabaseCA, created[0].GetType())
				require.Equal(t, "root", created[0].GetClusterName())
				require.Equal(t, types.DatabaseCA, created[1].GetType())
				require.Equal(t, "leaf2", created[1].GetClusterName())
			},
		},
		{
			name: "db certs already exists",
			fakeTrust: &fakeTrust{
				authorities: map[types.CertAuthID]types.CertAuthority{
					rootDB:  fakeCA,
					leaf2DB: fakeCA,
					leaf1DB: fakeCA,
				},
				clusters: []types.TrustedCluster{
					&types.TrustedClusterV2{
						Kind:     types.KindTrustedCluster,
						Metadata: types.Metadata{Name: "leaf1"},
						Spec: types.TrustedClusterSpecV2{
							Enabled: true,
						},
					}, &types.TrustedClusterV2{
						Kind:     types.KindTrustedCluster,
						Metadata: types.Metadata{Name: "leaf2"},
						Spec:     types.TrustedClusterSpecV2{},
					},
				},
			},
			assertion: require.NoError,
			validateFunc: func(t *testing.T, created []types.CertAuthority) {
				require.Empty(t, created)
			},
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			b, err := memory.New(memory.Config{EventsOff: true})
			require.NoError(t, err)

			test.fakeTrust.Trust = local.NewCAService(b)

			migration := createDBAuthority{
				trustServiceFn: func(b backend.Backend) services.Trust {
					return test.fakeTrust
				},
				configServiceFn: func(b backend.Backend) (services.ClusterConfiguration, error) {
					svc, err := local.NewClusterConfigurationService(b)
					if err != nil {
						return nil, trace.Wrap(err)
					}
					return fakeConfig{
						ClusterConfiguration: svc,
						clusterName:          clusterName("root"),
					}, nil
				},
			}

			test.assertion(t, migration.Up(context.Background(), b))
			test.validateFunc(t, test.fakeTrust.casCreated())
		})
	}
}

type fakeConfig struct {
	services.ClusterConfiguration
	clusterName types.ClusterName
}

func (f fakeConfig) GetClusterName(opts ...services.MarshalOption) (types.ClusterName, error) {
	return f.clusterName, nil
}

type fakeTrust struct {
	services.Trust

	authorities map[types.CertAuthID]types.CertAuthority

	clusters []types.TrustedCluster
	mu       sync.Mutex
	created  []types.CertAuthority
}

func (f *fakeTrust) casCreated() []types.CertAuthority {
	f.mu.Lock()
	defer f.mu.Unlock()

	cas := make([]types.CertAuthority, len(f.created))
	copy(cas, f.created)

	return cas
}

func (f *fakeTrust) CreateCertAuthority(ctx context.Context, ca types.CertAuthority) error {
	f.mu.Lock()
	f.created = append(f.created, ca)
	f.mu.Unlock()

	return nil
}

func (f *fakeTrust) GetCertAuthority(ctx context.Context, id types.CertAuthID, loadKeys bool) (types.CertAuthority, error) {
	ca, ok := f.authorities[id]
	if !ok {
		return nil, trace.NotFound("no authority matching %s present", id)
	}

	return ca, nil
}

func (f *fakeTrust) GetTrustedClusters(ctx context.Context) ([]types.TrustedCluster, error) {
	return f.clusters, nil
}
