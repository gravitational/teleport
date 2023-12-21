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

package reversetunnel

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/sshutils"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
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
		log:    utils.NewLoggerForTests(),
		Config: Config{Clock: clockwork.NewRealClock()},
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

func (ap mockAccessPoint) GetCertAuthority(ctx context.Context, id types.CertAuthID, loadKeys bool) (types.CertAuthority, error) {
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
			name:      "remote running 13.0.0",
			assertion: require.NoError,
			version:   "13.0.0",
		},
		{
			name:           "remote running 12.0.0",
			assertion:      require.NoError,
			version:        "12.0.0",
			oldRemoteProxy: true,
		},
		{
			name:           "remote running 11.0.0",
			assertion:      require.NoError,
			version:        "11.0.0",
			oldRemoteProxy: true,
		},
		{
			name:           "remote running 10.0.0",
			assertion:      require.NoError,
			version:        "10.0.0",
			oldRemoteProxy: true,
		},
		{
			name:           "remote running 9.0.0",
			assertion:      require.NoError,
			version:        "9.0.0",
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

func Test_ParseDialReq(t *testing.T) {
	testCases := []sshutils.DialReq{
		{
			Address:       "TargetAddress",
			ServerID:      "ServerID123",
			ConnType:      types.NodeTunnel,
			ClientSrcAddr: "192.168.1.13:444",
			ClientDstAddr: "192.168.1.14:444",
		},
		{
			Address:       "TargetAddress",
			ServerID:      "ServerID123",
			ConnType:      types.NodeTunnel,
			ClientSrcAddr: "[::1]:444",
			ClientDstAddr: "[::1]:555",
		},
	}

	for _, test := range testCases {
		payload, err := json.Marshal(test)
		require.NoError(t, err)
		require.NotEmpty(t, payload)

		parsed := parseDialReq(payload)
		require.Equal(t, &test, parsed)
	}
}
