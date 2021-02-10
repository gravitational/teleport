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
	"net"
	"testing"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils/testlog"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
)

func TestServerKeyAuth(t *testing.T) {
	ca := testauthority.New()
	priv, pub, err := ca.GenerateKeyPair("")
	require.NoError(t, err)

	s := &server{
		log: testlog.FailureOnly(t),
		localAccessPoint: mockAccessPoint{
			ca: types.NewCertAuthority(types.CertAuthoritySpecV2{
				Type:         services.HostCA,
				ClusterName:  "cluster-name",
				SigningKeys:  [][]byte{priv},
				CheckingKeys: [][]byte{pub},
				Roles:        nil,
				SigningAlg:   services.CertAuthoritySpecV2_RSA_SHA2_256,
			}),
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
				rawCert, err := ca.GenerateHostCert(auth.HostCertParams{
					PrivateCASigningKey: priv,
					CASigningAlg:        defaults.CASignatureAlgorithm,
					PublicHostKey:       pub,
					HostID:              "host-id",
					NodeName:            con.User(),
					ClusterName:         "host-cluster-name",
					Roles:               teleport.Roles{teleport.RoleNode},
				})
				require.NoError(t, err)
				key, _, _, _, err := ssh.ParseAuthorizedKey(rawCert)
				require.NoError(t, err)
				return key
			}(),
			wantExtensions: map[string]string{
				extHost:      con.User(),
				extCertType:  extCertTypeHost,
				extCertRole:  string(teleport.RoleNode),
				extAuthority: "host-cluster-name",
			},
			wantErr: require.NoError,
		},
		{
			desc: "user cert",
			key: func() ssh.PublicKey {
				rawCert, err := ca.GenerateUserCert(auth.UserCertParams{
					PrivateCASigningKey: priv,
					CASigningAlg:        defaults.CASignatureAlgorithm,
					PublicUserKey:       pub,
					Username:            con.User(),
					AllowedLogins:       []string{con.User()},
					Roles:               []string{"dev", "admin"},
					RouteToCluster:      "user-cluster-name",
					CertificateFormat:   teleport.CertificateFormatStandard,
					TTL:                 time.Minute,
				})
				require.NoError(t, err)
				key, _, _, _, err := ssh.ParseAuthorizedKey(rawCert)
				require.NoError(t, err)
				return key
			}(),
			wantExtensions: map[string]string{
				extHost:      con.User(),
				extCertType:  extCertTypeUser,
				extCertRole:  "dev",
				extAuthority: "user-cluster-name",
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
	auth.AccessPoint
	ca services.CertAuthority
}

func (ap mockAccessPoint) GetCertAuthority(id services.CertAuthID, loadKeys bool, opts ...auth.MarshalOption) (services.CertAuthority, error) {
	return ap.ca, nil
}
