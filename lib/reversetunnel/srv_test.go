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

package reversetunnel

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
)

func TestServerKeyAuth(t *testing.T) {
	ta := testauthority.New()
	priv, pub, err := ta.GenerateKeyPair()
	require.NoError(t, err)
	caSigner, err := ssh.ParsePrivateKey(priv)
	require.NoError(t, err)

	ca, err := types.NewCertAuthority(types.CertAuthoritySpecV2{
		Type:        types.HostCA,
		ClusterName: "cluster-name",
		ActiveKeys: types.CAKeySet{
			SSH: []*types.SSHKeyPair{{
				PrivateKey:     priv,
				PrivateKeyType: types.PrivateKeyType_RAW,
				PublicKey:      pub,
			}},
		},
	})
	require.NoError(t, err)

	s := &server{
		log: utils.NewLoggerForTests(),
		localAccessPoint: mockAccessPoint{
			ca: ca,
		},
	}
	con := mockSSHConnMetadata{}
	tests := []struct {
		desc           string
		key            ssh.PublicKey
		wantExtensions map[string]string
		wantErr        require.ErrorAssertionFunc
	}{
		{
			desc: "host cert",
			key: func() ssh.PublicKey {
				rawCert, err := ta.GenerateHostCert(services.HostCertParams{
					CASigner:      caSigner,
					PublicHostKey: pub,
					HostID:        "host-id",
					NodeName:      con.User(),
					ClusterName:   "host-cluster-name",
					Role:          types.RoleNode,
				})
				require.NoError(t, err)
				key, _, _, _, err := ssh.ParseAuthorizedKey(rawCert)
				require.NoError(t, err)
				return key
			}(),
			wantExtensions: map[string]string{
				extHost:              con.User(),
				utils.ExtIntCertType: utils.ExtIntCertTypeHost,
				extCertRole:          string(types.RoleNode),
				extAuthority:         "host-cluster-name",
			},
			wantErr: require.NoError,
		},
		{
			desc: "user cert",
			key: func() ssh.PublicKey {
				rawCert, err := ta.GenerateUserCert(services.UserCertParams{
					CASigner:          caSigner,
					PublicUserKey:     pub,
					Username:          con.User(),
					AllowedLogins:     []string{con.User()},
					Roles:             []string{"dev", "admin"},
					RouteToCluster:    "user-cluster-name",
					CertificateFormat: constants.CertificateFormatStandard,
					TTL:               time.Minute,
				})
				require.NoError(t, err)
				key, _, _, _, err := ssh.ParseAuthorizedKey(rawCert)
				require.NoError(t, err)
				return key
			}(),
			wantExtensions: map[string]string{
				extHost:              con.User(),
				utils.ExtIntCertType: utils.ExtIntCertTypeUser,
				extCertRole:          "dev",
				extAuthority:         "user-cluster-name",
			},
			wantErr: require.NoError,
		},
		{
			desc: "not a cert",
			key: func() ssh.PublicKey {
				key, _, _, _, err := ssh.ParseAuthorizedKey(pub)
				require.NoError(t, err)
				return key
			}(),
			wantErr: require.Error,
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			perm, err := s.keyAuth(con, tt.key)
			tt.wantErr(t, err)
			if err == nil {
				require.Empty(t, cmp.Diff(perm, &ssh.Permissions{Extensions: tt.wantExtensions}))
			}
		})
	}
}

type mockSSHConnMetadata struct {
	ssh.ConnMetadata
}

func (mockSSHConnMetadata) User() string         { return "conn-user" }
func (mockSSHConnMetadata) RemoteAddr() net.Addr { return &net.TCPAddr{} }

type mockAccessPoint struct {
	auth.ProxyAccessPoint
	ca types.CertAuthority
}

func (ap mockAccessPoint) GetCertAuthority(ctx context.Context, id types.CertAuthID, loadKeys bool, opts ...services.MarshalOption) (types.CertAuthority, error) {
	return ap.ca, nil
}

func TestCreateRemoteAccessPoint(t *testing.T) {
	cases := []struct {
		name           string
		version        string
		assertion      require.ErrorAssertionFunc
		oldRemoteProxy bool
	}{
		{
			name:      "invalid version",
			assertion: require.Error,
		},
		{
			name:      "remote running 9.0.0",
			assertion: require.NoError,
			version:   "9.0.0",
		},
		{
			name:      "remote running 8.0.0",
			assertion: require.NoError,
			version:   "8.0.0",
		},
		{
			name:           "remote running 7.0.0",
			assertion:      require.NoError,
			version:        "7.0.0",
			oldRemoteProxy: true,
		},
		{
			name:           "remote running 6.0.0",
			assertion:      require.NoError,
			version:        "6.0.0",
			oldRemoteProxy: true,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			newProxyFn := func(clt auth.ClientI, cacheName []string) (auth.RemoteProxyAccessPoint, error) {
				if tt.oldRemoteProxy {
					return nil, errors.New("expected to create an old remote proxy")
				}

				return nil, nil
			}

			oldProxyFn := func(clt auth.ClientI, cacheName []string) (auth.RemoteProxyAccessPoint, error) {
				if !tt.oldRemoteProxy {
					return nil, errors.New("expected to create an new remote proxy")
				}

				return nil, nil
			}

			clt := &mockAuthClient{}
			srv := &server{
				log: utils.NewLoggerForTests(),
				Config: Config{
					NewCachingAccessPoint:         newProxyFn,
					NewCachingAccessPointOldProxy: oldProxyFn,
				},
			}
			_, err := createRemoteAccessPoint(srv, clt, tt.version, "test")
			tt.assertion(t, err)
		})
	}
}
