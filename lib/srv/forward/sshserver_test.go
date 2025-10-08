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

package forward

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"net"
	"os/user"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/utils/keys"
	apisshutils "github.com/gravitational/teleport/api/utils/sshutils"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
)

func TestSignersWithSHA1Fallback(t *testing.T) {
	assertSHA2Signer := func(t *testing.T, signer ssh.Signer) {
		require.Equal(t, ssh.CertAlgoRSAv01, signer.PublicKey().Type())

		sha2AlgSigner, ok := signer.(ssh.AlgorithmSigner)
		require.True(t, ok)

		data := make([]byte, 32)
		// This is how x/crypto signs SSH certificates.
		sig, err := sha2AlgSigner.SignWithAlgorithm(rand.Reader, data, ssh.KeyAlgoRSASHA512)
		require.NoError(t, err)
		require.Equal(t, ssh.KeyAlgoRSASHA512, sig.Format)
	}

	assertSHA1Signer := func(t *testing.T, signer ssh.Signer) {
		require.Equal(t, ssh.CertAlgoRSAv01, signer.PublicKey().Type())

		// We should not be able to cast the signer to ssh.AlgorithmSigner.
		// Otherwise, x/crypto will use SHA-2-512 for signing.
		_, ok := signer.(ssh.AlgorithmSigner)
		require.False(t, ok)

		data := make([]byte, 32)
		sig, err := signer.Sign(rand.Reader, data)
		require.NoError(t, err)
		require.Equal(t, ssh.KeyAlgoRSA, sig.Format)
	}

	tests := []struct {
		name      string
		signersCb func(t *testing.T) []ssh.Signer
		want      func(t *testing.T, got []ssh.Signer)
	}{
		{
			name: "RSA host certificate",
			signersCb: func(t *testing.T) []ssh.Signer {
				caSigner, err := apisshutils.MakeTestSSHCA()
				require.NoError(t, err)
				hostKey, err := keys.ParsePrivateKey(fixtures.PEMBytes["rsa"])
				require.NoError(t, err)
				hostCert, err := apisshutils.MakeRealHostCertWithKey(hostKey.Signer, caSigner)
				require.NoError(t, err)
				return []ssh.Signer{hostCert}
			},
			want: func(t *testing.T, signers []ssh.Signer) {
				// We expect 2 certificates, order matters.
				require.Len(t, signers, 2)
				assertSHA2Signer(t, signers[0])
				assertSHA1Signer(t, signers[1])
			},
		},
		{
			name: "RSA host public key",
			signersCb: func(t *testing.T) []ssh.Signer {
				hostKey, err := keys.ParsePrivateKey(fixtures.PEMBytes["rsa"])
				require.NoError(t, err)
				hostSigner, err := ssh.NewSignerFromSigner(hostKey.Signer)
				require.NoError(t, err)
				return []ssh.Signer{hostSigner}
			},
			want: func(t *testing.T, signers []ssh.Signer) {
				// public key should not be copied
				require.Len(t, signers, 1)
				require.Equal(t, ssh.KeyAlgoRSA, signers[0].PublicKey().Type())
			},
		},
		{
			name: "Ed25519 host certificate",
			signersCb: func(t *testing.T) []ssh.Signer {
				caSigner, err := apisshutils.MakeTestSSHCA()
				require.NoError(t, err)
				hostCert, err := apisshutils.MakeRealHostCert(caSigner)
				require.NoError(t, err)
				return []ssh.Signer{hostCert}
			},
			want: func(t *testing.T, signers []ssh.Signer) {
				require.Len(t, signers, 1)
				require.Equal(t, ssh.CertAlgoED25519v01, signers[0].PublicKey().Type())
			},
		},
		{
			name: "Ed25519 host key",
			signersCb: func(t *testing.T) []ssh.Signer {
				_, hostKey, err := ed25519.GenerateKey(rand.Reader)
				require.NoError(t, err)
				hostSigner, err := ssh.NewSignerFromSigner(hostKey)
				require.NoError(t, err)
				return []ssh.Signer{hostSigner}
			},
			want: func(t *testing.T, signers []ssh.Signer) {
				require.Len(t, signers, 1)
				require.Equal(t, ssh.KeyAlgoED25519, signers[0].PublicKey().Type())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			getSignersFn := signersWithSHA1Fallback(tt.signersCb(t))
			signers, err := getSignersFn()
			require.NoError(t, err)
			tt.want(t, signers)
		})
	}
}

type newChannelMock struct {
	channelType string
	accepted    atomic.Bool
	rejected    atomic.Bool
}

func (n *newChannelMock) Accept() (ssh.Channel, <-chan *ssh.Request, error) {
	n.accepted.Store(true)
	return nil, nil, errors.New("mock channel")
}

func (n *newChannelMock) Reject(reason ssh.RejectionReason, message string) error {
	n.rejected.Store(true)
	return nil
}

func (n *newChannelMock) ChannelType() string {
	return n.channelType
}

func (n *newChannelMock) ExtraData() []byte {
	return ssh.Marshal(sshutils.DirectTCPIPReq{
		Host:     "localhost",
		Port:     0,
		Orig:     "localhost",
		OrigPort: 0,
	})
}

// TestDirectTCPIP ensures that ssh client using SessionJoinPrincipal as Login
// cannot connect using "direct-tcpip" on forward mode.
//
// Forward requires a lot of dependencies and we don't have top level tests
// yet here. If we add it in future, test should be rework to use public methods
// instead of internals.
func TestDirectTCPIP(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	cases := []struct {
		name           string
		login          string
		expectAccepted bool
		expectRejected bool
	}{
		{
			name:           "join principal rejected",
			login:          teleport.SSHSessionJoinPrincipal,
			expectAccepted: false,
			expectRejected: true,
		},
		{
			name: "user allowed",
			login: func() string {
				u, err := user.Current()
				require.NoError(t, err)
				return u.Username
			}(),
			expectAccepted: true,
			// expectRejected is set to true because we are using mock channel
			// which return errors on accept.
			expectRejected: true,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := Server{
				logger:          logtest.NewLogger(),
				identityContext: srv.IdentityContext{Login: tt.login},
			}

			nch := &newChannelMock{channelType: teleport.ChanDirectTCPIP}
			s.handleChannel(ctx, nch)
			require.Equal(t, tt.expectRejected, nch.rejected.Load())
			require.Equal(t, tt.expectAccepted, nch.accepted.Load())
		})
	}
}

func TestCheckTCPIPForward(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		login  string
		assert require.ErrorAssertionFunc
	}{
		{
			name:   "join principal rejected",
			login:  teleport.SSHSessionJoinPrincipal,
			assert: require.Error,
		},
		{
			name:   "user accepted",
			login:  "test-user",
			assert: require.NoError,
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := Server{
				logger:          logtest.NewLogger(),
				identityContext: srv.IdentityContext{Login: tt.login},
				targetServer:    &types.ServerV2{},
			}
			err := s.checkTCPIPForwardRequest(context.Background(),
				&ssh.Request{
					Type:      teleport.TCPIPForwardRequest,
					WantReply: false,
					Payload: ssh.Marshal(sshutils.TCPIPForwardReq{
						Addr: "localhost",
						Port: 0,
					}),
				})
			tt.assert(t, err)
		})
	}
}

// TODO(atburke): Add test for handleForwardedTCPIPRequest once we have
// infrastructure for higher-level tests here.

func TestEventMetadata(t *testing.T) {
	nodeID := uuid.NewString()
	proxyID := uuid.NewString()

	for _, tt := range []struct {
		name           string
		subkind        string
		spec           types.ServerSpecV2
		labels         map[string]string
		expectMetadata events.ServerMetadata
	}{
		{
			name: "tunnel node",
			labels: map[string]string{
				"stcLabel": "stcResult",
			},
			spec: types.ServerSpecV2{
				Addr: "127.0.0.1:3022",
				CmdLabels: map[string]types.CommandLabelV2{
					"cmdLabel": {Result: "cmdResult"},
				},
				Hostname:  "server01",
				UseTunnel: true,
			},
			expectMetadata: events.ServerMetadata{
				ServerVersion:   teleport.Version,
				ServerID:        nodeID,
				ServerNamespace: apidefaults.Namespace,
				ServerAddr:      "",
				ServerHostname:  "server01",
				ServerLabels: map[string]string{
					"stcLabel": "stcResult",
					"cmdLabel": "cmdResult",
				},
				ServerSubKind: types.SubKindTeleportNode,
				ForwardedBy:   proxyID,
			},
		}, {
			name: "tunnel node",
			labels: map[string]string{
				"stcLabel": "stcResult",
			},
			spec: types.ServerSpecV2{
				Addr: "127.0.0.1:3022",
				CmdLabels: map[string]types.CommandLabelV2{
					"cmdLabel": {Result: "cmdResult"},
				},
				Hostname: "server01",
			},
			expectMetadata: events.ServerMetadata{
				ServerVersion:   teleport.Version,
				ServerID:        nodeID,
				ServerNamespace: apidefaults.Namespace,
				ServerAddr:      "127.0.0.1:3022",
				ServerHostname:  "server01",
				ServerLabels: map[string]string{
					"stcLabel": "stcResult",
					"cmdLabel": "cmdResult",
				},
				ServerSubKind: types.SubKindTeleportNode,
				ForwardedBy:   proxyID,
			},
		}, {
			name:    "agentless node",
			subkind: types.SubKindOpenSSHNode,
			labels: map[string]string{
				"stcLabel": "stcResult",
			},
			spec: types.ServerSpecV2{
				Addr:     "openssh.example.com:22",
				Hostname: "agentless-host",
			},
			expectMetadata: events.ServerMetadata{
				ServerVersion:   teleport.Version,
				ServerID:        nodeID,
				ServerNamespace: apidefaults.Namespace,
				ServerAddr:      "openssh.example.com:22",
				ServerHostname:  "agentless-host",
				ServerLabels: map[string]string{
					"stcLabel": "stcResult",
				},
				ServerSubKind: types.SubKindOpenSSHNode,
				ForwardedBy:   proxyID,
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			targetServer, err := types.NewNode(nodeID, tt.subkind, tt.spec, tt.labels)
			require.NoError(t, err)

			forwardSrv := &Server{
				proxyUUID:    proxyID,
				targetServer: targetServer,
			}

			require.EqualValues(t, tt.expectMetadata, forwardSrv.EventMetadata())
		})
	}
}

func TestServerConfigCheckDefaults(t *testing.T) {
	teleportNode, err := types.NewNode("teleport-node", "", types.ServerSpecV2{}, nil)
	require.NoError(t, err)

	openSSHNode, err := types.NewNode("openssh-node", types.SubKindOpenSSHNode, types.ServerSpecV2{
		Addr:     "openssh.example.com:22",
		Hostname: "openssh.example.com",
	}, nil)
	require.NoError(t, err)

	openSSHEICENode, err := types.NewEICENode(types.ServerSpecV2{
		Addr:     "openssheice.example.com:22",
		Hostname: "openssheice.example.com",
		CloudMetadata: &types.CloudMetadata{
			AWS: &types.AWSInfo{
				AccountID:   "123456789012",
				InstanceID:  "i-123456789012",
				Region:      "us-east-1",
				VPCID:       "vpc-abcd",
				SubnetID:    "subnet-123",
				Integration: "teleportdev",
			},
		},
	}, nil)
	require.NoError(t, err)

	for _, tt := range []struct {
		name           string
		modifyCfg      func(c *ServerConfig)
		errorAssertion require.ErrorAssertionFunc
	}{
		{
			name:      "no targetServer",
			modifyCfg: func(c *ServerConfig) {},
			errorAssertion: func(tt require.TestingT, err error, i ...interface{}) {
				require.Error(t, err)
				require.ErrorContains(t, err, "target server is required")
			},
		}, {
			name: "Teleport Node",
			modifyCfg: func(c *ServerConfig) {
				c.TargetServer = teleportNode
				c.UserAgent = &sshutils.AgentChannel{}
			},
			errorAssertion: require.NoError,
		}, {
			name: "Teleport Node no agent",
			modifyCfg: func(c *ServerConfig) {
				c.TargetServer = teleportNode
			},
			errorAssertion: func(tt require.TestingT, err error, i ...interface{}) {
				require.Error(t, err)
				require.ErrorContains(t, err, "user agent required")
			},
		}, {
			name: "OpenSSH Node",
			modifyCfg: func(c *ServerConfig) {
				c.TargetServer = openSSHNode
				c.AgentlessSigner = &sshutils.LegacySHA1Signer{}
			},
			errorAssertion: require.NoError,
		}, {
			name: "OpenSSH Node no signer",
			modifyCfg: func(c *ServerConfig) {
				c.TargetServer = openSSHNode
			},
			errorAssertion: func(tt require.TestingT, err error, i ...interface{}) {
				require.Error(t, err)
				require.ErrorContains(t, err, "agentless signer is required")
			},
		}, {
			name: "OpenSSH EICE Node",
			modifyCfg: func(c *ServerConfig) {
				c.TargetServer = openSSHEICENode
			},
			errorAssertion: require.NoError,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			config := &ServerConfig{
				LocalAuthClient:          &authclient.Client{},
				TargetClusterAccessPoint: &authclient.Client{},
				DataDir:                  "datadir",
				TargetConn:               &net.UnixConn{},
				SrcAddr:                  &net.IPAddr{},
				DstAddr:                  &net.IPAddr{},
				HostCertificate:          &sshutils.LegacySHA1Signer{},
				Clock:                    clockwork.NewFakeClock(),
				Emitter:                  &authclient.Client{},
				LockWatcher:              &services.LockWatcher{},
				EICESigner: func(ctx context.Context, target types.Server, integration types.Integration, login, token string, ap cryptosuites.AuthPreferenceGetter) (ssh.Signer, error) {
					return nil, nil
				},
			}

			tt.modifyCfg(config)

			err := config.CheckDefaults()
			tt.errorAssertion(t, err)
		})
	}

	// UserAgent:                userAgent,
	// AgentlessSigner:          params.AgentlessSigner,
	// TargetServer:    params.TargetServer,

}
