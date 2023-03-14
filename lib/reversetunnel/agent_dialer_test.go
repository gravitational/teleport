/*
Copyright 2022 Gravitational, Inc.

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
	"crypto/rand"
	"crypto/rsa"
	"testing"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/api/types"
	apisshutils "github.com/gravitational/teleport/api/utils/sshutils"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/utils"
)

// TestAgentCertChecker validates that reverse tunnel agents properly validate
// SSH host certificates.
func TestAgentCertChecker(t *testing.T) {
	handler := sshutils.NewChanHandlerFunc(func(_ context.Context, ccx *sshutils.ConnectionContext, nch ssh.NewChannel) {
		ch, _, err := nch.Accept()
		require.NoError(t, err)
		require.NoError(t, ch.Close())
	})

	ca, err := apisshutils.MakeTestSSHCA()
	require.NoError(t, err)

	tests := []struct {
		name       string
		signerFunc func(ssh.Signer) (ssh.Signer, error)
		requireErr require.ErrorAssertionFunc
	}{
		{
			"Ensure valid host certificate is accepted.",
			apisshutils.MakeRealHostCert,
			require.NoError,
		},
		{
			"Ensure invalid host certificate is rejected.",
			apisshutils.MakeSpoofedHostCert,
			require.Error,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cert, err := tc.signerFunc(ca)
			require.NoError(t, err)

			sshServer, err := sshutils.NewServer(
				"test",
				utils.NetAddr{AddrNetwork: "tcp", Addr: "localhost:0"},
				handler,
				[]ssh.Signer{cert},
				sshutils.AuthMethods{NoClient: true},
				sshutils.SetInsecureSkipHostValidation(),
			)
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, sshServer.Close()) })
			require.NoError(t, sshServer.Start())

			priv, err := rsa.GenerateKey(rand.Reader, 2048)
			require.NoError(t, err)

			signer, err := ssh.NewSignerFromKey(priv)
			require.NoError(t, err)

			dialer := agentDialer{
				client:      &fakeClient{caKey: ca.PublicKey()},
				authMethods: []ssh.AuthMethod{ssh.PublicKeys(signer)},
				log:         logrus.New(),
			}

			_, err = dialer.DialContext(context.Background(), *utils.MustParseAddr(sshServer.Addr()))
			tc.requireErr(t, err)
		})
	}
}

type fakeClient struct {
	auth.AccessCache
	caKey ssh.PublicKey
}

func (fc *fakeClient) GetCertAuthorities(ctx context.Context, caType types.CertAuthType, loadKeys bool) ([]types.CertAuthority, error) {
	ca, err := types.NewCertAuthority(types.CertAuthoritySpecV2{
		Type:        types.HostCA,
		ClusterName: "example.com",
		ActiveKeys: types.CAKeySet{
			SSH: []*types.SSHKeyPair{
				{
					PublicKey: ssh.MarshalAuthorizedKey(fc.caKey),
				},
			},
		},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return []types.CertAuthority{ca}, nil
}

func (fc *fakeClient) GetClusterNetworkingConfig(ctx context.Context, opts ...services.MarshalOption) (types.ClusterNetworkingConfig, error) {
	return types.NewClusterNetworkingConfigFromConfigFile(types.ClusterNetworkingConfigSpecV2{})
}
