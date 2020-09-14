package reversetunnel

import (
	"net"
	"testing"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils/testlog"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"golang.org/x/crypto/ssh"
)

func TestServerKeyAuth(t *testing.T) {
	ca := testauthority.New()
	priv, pub, err := ca.GenerateKeyPair("")
	assert.NoError(t, err)

	s := &server{
		Entry: testlog.FailureOnly(t),
		localAccessPoint: mockAccessPoint{ca: services.NewCertAuthority(
			services.HostCA,
			"cluster-name",
			[][]byte{priv},
			[][]byte{pub},
			nil,
			services.CertAuthoritySpecV2_RSA_SHA2_256,
		)},
	}
	con := mockSSHConnMetadata{}
	tests := []struct {
		desc           string
		key            ssh.PublicKey
		wantExtensions map[string]string
		wantErr        assert.ErrorAssertionFunc
	}{
		{
			desc: "host cert",
			key: func() ssh.PublicKey {
				rawCert, err := ca.GenerateHostCert(services.HostCertParams{
					PrivateCASigningKey: priv,
					CASigningAlg:        defaults.CASignatureAlgorithm,
					PublicHostKey:       pub,
					HostID:              "host-id",
					NodeName:            con.User(),
					ClusterName:         "host-cluster-name",
					Roles:               teleport.Roles{teleport.RoleNode},
				})
				assert.NoError(t, err)
				key, _, _, _, err := ssh.ParseAuthorizedKey(rawCert)
				assert.NoError(t, err)
				return key
			}(),
			wantExtensions: map[string]string{
				extHost:      con.User(),
				extCertType:  extCertTypeHost,
				extCertRole:  string(teleport.RoleNode),
				extAuthority: "host-cluster-name",
			},
			wantErr: assert.NoError,
		},
		{
			desc: "user cert",
			key: func() ssh.PublicKey {
				rawCert, err := ca.GenerateUserCert(services.UserCertParams{
					PrivateCASigningKey: priv,
					CASigningAlg:        defaults.CASignatureAlgorithm,
					PublicUserKey:       pub,
					Username:            con.User(),
					AllowedLogins:       []string{con.User()},
					Roles:               []string{"dev", "admin"},
					RouteToCluster:      "user-cluster-name",
					CertificateFormat:   teleport.CertificateFormatStandard,
				})
				assert.NoError(t, err)
				key, _, _, _, err := ssh.ParseAuthorizedKey(rawCert)
				assert.NoError(t, err)
				return key
			}(),
			wantExtensions: map[string]string{
				extHost:      con.User(),
				extCertType:  extCertTypeUser,
				extCertRole:  "dev",
				extAuthority: "user-cluster-name",
			},
			wantErr: assert.NoError,
		},
		{
			desc: "not a cert",
			key: func() ssh.PublicKey {
				key, _, _, _, err := ssh.ParseAuthorizedKey(pub)
				assert.NoError(t, err)
				return key
			}(),
			wantErr: assert.Error,
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			perm, err := s.keyAuth(con, tt.key)
			tt.wantErr(t, err)
			if err == nil {
				assert.Empty(t, cmp.Diff(perm, &ssh.Permissions{Extensions: tt.wantExtensions}))
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

func (ap mockAccessPoint) GetCertAuthority(id services.CertAuthID, loadKeys bool, opts ...services.MarshalOption) (services.CertAuthority, error) {
	return ap.ca, nil
}
