/*
Copyright 2020 Gravitational, Inc.

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

package auth

import (
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"testing"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/tlsca"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/require"
)

func TestMiddlewareGetUser(t *testing.T) {
	t.Parallel()
	const (
		localClusterName  = "local"
		remoteClusterName = "remote"
	)
	s := newTestServices(t)
	// Set up local cluster name in the backend.
	cn, err := types.NewClusterName(types.ClusterNameSpecV2{
		ClusterName: localClusterName,
	})
	require.NoError(t, err)
	require.NoError(t, s.UpsertClusterName(cn))

	// Helper func for generating fake certs.
	subject := func(id tlsca.Identity) pkix.Name {
		s, err := id.Subject()
		require.NoError(t, err)
		// ExtraNames get moved to Names when generating a real x509 cert.
		// Since we're just mimicking certs in memory, move manually.
		s.Names = s.ExtraNames
		return s
	}
	now := time.Date(2020, time.November, 5, 0, 0, 0, 0, time.UTC)

	var (
		localUserIdentity = tlsca.Identity{
			Username:        "foo",
			Groups:          []string{"devs"},
			TeleportCluster: localClusterName,
			Expires:         now,
		}
		localUserIdentityNoTeleportCluster = tlsca.Identity{
			Username: "foo",
			Groups:   []string{"devs"},
			Expires:  now,
		}
		localSystemRole = tlsca.Identity{
			Username:        "node",
			Groups:          []string{string(teleport.RoleNode)},
			TeleportCluster: localClusterName,
			Expires:         now,
		}
		remoteUserIdentity = tlsca.Identity{
			Username:        "foo",
			Groups:          []string{"devs"},
			TeleportCluster: remoteClusterName,
			Expires:         now,
		}
		remoteUserIdentityNoTeleportCluster = tlsca.Identity{
			Username: "foo",
			Groups:   []string{"devs"},
			Expires:  now,
		}
		remoteSystemRole = tlsca.Identity{
			Username:        "node",
			Groups:          []string{string(teleport.RoleNode)},
			TeleportCluster: remoteClusterName,
			Expires:         now,
		}
	)

	tests := []struct {
		desc      string
		peers     []*x509.Certificate
		wantID    IdentityGetter
		assertErr require.ErrorAssertionFunc
	}{
		{
			desc: "no client cert",
			wantID: BuiltinRole{
				Role:        teleport.RoleNop,
				Username:    string(teleport.RoleNop),
				ClusterName: localClusterName,
				Identity:    tlsca.Identity{},
			},
			assertErr: require.NoError,
		},
		{
			desc: "local user",
			peers: []*x509.Certificate{{
				Subject:  subject(localUserIdentity),
				NotAfter: now,
				Issuer:   pkix.Name{Organization: []string{localClusterName}},
			}},
			wantID: LocalUser{
				Username: localUserIdentity.Username,
				Identity: localUserIdentity,
			},
			assertErr: require.NoError,
		},
		{
			desc: "local user no teleport cluster in cert subject",
			peers: []*x509.Certificate{{
				Subject:  subject(localUserIdentityNoTeleportCluster),
				NotAfter: now,
				Issuer:   pkix.Name{Organization: []string{localClusterName}},
			}},
			wantID: LocalUser{
				Username: localUserIdentity.Username,
				Identity: localUserIdentity,
			},
			assertErr: require.NoError,
		},
		{
			desc: "local system role",
			peers: []*x509.Certificate{{
				Subject:  subject(localSystemRole),
				NotAfter: now,
				Issuer:   pkix.Name{Organization: []string{localClusterName}},
			}},
			wantID: BuiltinRole{
				Username:    localSystemRole.Username,
				Role:        teleport.RoleNode,
				ClusterName: localClusterName,
				Identity:    localSystemRole,
			},
			assertErr: require.NoError,
		},
		{
			desc: "remote user",
			peers: []*x509.Certificate{{
				Subject:  subject(remoteUserIdentity),
				NotAfter: now,
				Issuer:   pkix.Name{Organization: []string{remoteClusterName}},
			}},
			wantID: RemoteUser{
				ClusterName: remoteClusterName,
				Username:    remoteUserIdentity.Username,
				RemoteRoles: remoteUserIdentity.Groups,
				Identity:    remoteUserIdentity,
			},
			assertErr: require.NoError,
		},
		{
			desc: "remote user no teleport cluster in cert subject",
			peers: []*x509.Certificate{{
				Subject:  subject(remoteUserIdentityNoTeleportCluster),
				NotAfter: now,
				Issuer:   pkix.Name{Organization: []string{remoteClusterName}},
			}},
			wantID: RemoteUser{
				ClusterName: remoteClusterName,
				Username:    remoteUserIdentity.Username,
				RemoteRoles: remoteUserIdentity.Groups,
				Identity:    remoteUserIdentity,
			},
			assertErr: require.NoError,
		},
		{
			desc: "remote system role",
			peers: []*x509.Certificate{{
				Subject:  subject(remoteSystemRole),
				NotAfter: now,
				Issuer:   pkix.Name{Organization: []string{remoteClusterName}},
			}},
			wantID: RemoteBuiltinRole{
				Username:    remoteSystemRole.Username,
				Role:        teleport.RoleNode,
				ClusterName: remoteClusterName,
				Identity:    remoteSystemRole,
			},
			assertErr: require.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {

			m := &Middleware{
				AccessPoint: s,
			}

			id, err := m.GetUser(tls.ConnectionState{PeerCertificates: tt.peers})
			tt.assertErr(t, err)
			if err != nil {
				return
			}
			require.Empty(t, cmp.Diff(id, tt.wantID,
				cmpopts.IgnoreFields(BuiltinRole{}, "GetSessionRecordingConfig"),
				cmpopts.EquateEmpty(),
			))
		})
	}
}
