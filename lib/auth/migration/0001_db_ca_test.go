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
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
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
		authorities  []types.CertAuthID
		clusters     []types.TrustedCluster
		assertion    require.ErrorAssertionFunc
		validateFunc func(t *testing.T, svc *local.CA)
	}{
		{
			name: "db cas created from host cas",
			authorities: []types.CertAuthID{
				rootHost,
				leaf1Host,
				leaf1DB,
				leaf2Host,
			},
			clusters: []types.TrustedCluster{
				&types.TrustedClusterV2{
					Kind:     types.KindTrustedCluster,
					Metadata: types.Metadata{Name: "leaf1"},
					Spec: types.TrustedClusterSpecV2{
						Enabled: true,
						RoleMap: []types.RoleMapping{
							{
								Local:  []string{teleport.PresetAccessRoleName},
								Remote: teleport.PresetAccessRoleName,
							},
						},
					},
				},
				&types.TrustedClusterV2{
					Kind:     types.KindTrustedCluster,
					Metadata: types.Metadata{Name: "leaf2"},
					Spec: types.TrustedClusterSpecV2{
						RoleMap: []types.RoleMapping{
							{
								Local:  []string{teleport.PresetAccessRoleName},
								Remote: teleport.PresetAccessRoleName,
							},
						},
					},
				},
			},
			assertion: require.NoError,
			validateFunc: func(t *testing.T, svc *local.CA) {
				hostCAs, err := svc.GetCertAuthorities(t.Context(), types.HostCA, false)
				require.NoError(t, err)
				assert.Len(t, hostCAs, 3)

				for _, ca := range hostCAs {
					ca, err := svc.GetCertAuthority(t.Context(), types.CertAuthID{Type: types.DatabaseCA, DomainName: ca.GetClusterName()}, false)
					require.NoError(t, err)
					require.NotNil(t, ca)
				}
			},
		},
		{
			name: "db certs already exists",
			authorities: []types.CertAuthID{
				rootDB,
				leaf2DB,
				leaf1DB,
			},
			clusters: []types.TrustedCluster{
				&types.TrustedClusterV2{
					Kind:     types.KindTrustedCluster,
					Metadata: types.Metadata{Name: "leaf1"},
					Spec: types.TrustedClusterSpecV2{
						Enabled: true,
						RoleMap: []types.RoleMapping{
							{
								Local:  []string{teleport.PresetAccessRoleName},
								Remote: teleport.PresetAccessRoleName,
							},
						},
					},
				},
				&types.TrustedClusterV2{
					Kind:     types.KindTrustedCluster,
					Metadata: types.Metadata{Name: "leaf2"},
					Spec: types.TrustedClusterSpecV2{
						RoleMap: []types.RoleMapping{
							{
								Local:  []string{teleport.PresetAccessRoleName},
								Remote: teleport.PresetAccessRoleName,
							},
						},
					},
				},
			},
			assertion: require.NoError,
			validateFunc: func(t *testing.T, svc *local.CA) {
				cas, err := svc.GetCertAuthorities(t.Context(), types.DatabaseCA, false)
				require.NoError(t, err)
				assert.Len(t, cas, 3)

				hostCAs, err := svc.GetCertAuthorities(t.Context(), types.HostCA, false)
				require.NoError(t, err)
				assert.Empty(t, hostCAs)
			},
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			b, err := memory.New(memory.Config{EventsOff: true})
			require.NoError(t, err)

			svc := local.NewCAService(b)
			for _, id := range test.authorities {
				ca := newCertAuthority(t, id.DomainName, id.Type)
				require.NoError(t, svc.CreateCertAuthority(t.Context(), ca))
			}

			for _, tc := range test.clusters {
				_, err := svc.CreateTrustedCluster(t.Context(), tc, nil)
				require.NoError(t, err)
			}

			migration := createDBAuthority{
				trustServiceFn: func(b backend.Backend) *local.CA {
					return svc
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

			test.assertion(t, migration.Up(t.Context(), b))
			test.validateFunc(t, svc)
		})
	}
}

type fakeConfig struct {
	services.ClusterConfiguration
	clusterName types.ClusterName
}

func (f fakeConfig) GetClusterName(_ context.Context) (types.ClusterName, error) {
	return f.clusterName, nil
}
