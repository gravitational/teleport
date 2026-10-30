// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package client

import (
	"context"

	"github.com/gravitational/trace"

	pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/loginagent/v1"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/client/credentialencryption"
	"github.com/gravitational/teleport/lib/ipc"
)

// LoginAgentDirEnvVar is the name of the environment variable that will be used
// to configure the Machine ID login agent.
const LoginAgentDirEnvVar = "TELEPORT_LOGIN_AGENT_DIR"

// loginWithMachineIDAgent logs the user in non-interactively using the Machine
// ID login agent.
func (tc *TeleportClient) loginWithMachineIDAgent(agentDir string) SSHLoginFunc {
	// If the cluster requires hardware-backed private keys, GetNewLoginKeyRing
	// will try to create a local PIV key before the login function is called,
	// which (A) is unnecessary as the login agent manages its own private key
	// material and (B) will likely fail because the login agent is typically
	// used in situations where there is no YubiKey, etc.
	tc.PrivateKeyPolicy = keys.PrivateKeyPolicyNone

	return func(ctx context.Context, keyRing *KeyRing) (*authclient.CLILoginResponse, error) {
		log.DebugContext(ctx, "Logging in using the Machine ID login agent")

		conn, err := ipc.NewClient(agentDir)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		defer conn.Close()

		rsp, err := pb.NewLoginAgentServiceClient(conn).
			Login(ctx, pb.LoginRequest_builder{
				RouteToCluster:    tc.SiteName,
				KubernetesCluster: tc.KubernetesCluster,
				Scope:             tc.Scope,
			}.Build())
		if err != nil {
			return nil, trace.Wrap(err)
		}

		// Overwrite the key ring's private key. The other fields (e.g. Username,
		// Cert) are set by SSHLogin.
		privKey, err := keys.ParsePrivateKey(
			rsp.GetPrivateKey(),
			keys.WithHardwareKeyService(tc.ClientStore.HardwareKeyService),
		)
		if err != nil {
			return nil, trace.Wrap(err, "parsing private key")
		}
		keyRing.SSHPrivateKey = privKey
		keyRing.TLSPrivateKey = privKey

		// Set EncryptionPrivateKey to nil if nothing from login response.
		keyRing.EncryptionPrivateKey = nil
		if encryptionKeyPEM := rsp.GetEncryptionKey(); len(encryptionKeyPEM) > 0 {
			keyRing.EncryptionPrivateKey, err = credentialencryption.ParsePrivateKey(
				encryptionKeyPEM,
				credentialencryption.WithHardwareKeyService(tc.ClientStore.HardwareKeyService),
			)
			if err != nil {
				return nil, trace.Wrap(err, "parsing session encryption key")
			}
		}

		hostSigners := make([]authclient.TrustedCerts, len(rsp.GetHostSigners()))
		for idx, hs := range rsp.GetHostSigners() {
			hostSigners[idx] = authclient.TrustedCerts{
				ClusterName:     hs.GetClusterName(),
				AuthorizedKeys:  hs.GetSshAuthorizedKeys(),
				TLSCertificates: hs.GetTlsCaCerts(),
			}
		}

		return &authclient.CLILoginResponse{
			Username:    rsp.GetUsername(),
			Cert:        rsp.GetSshCert(),
			TLSCert:     rsp.GetTlsCert(),
			HostSigners: hostSigners,
		}, nil
	}
}
