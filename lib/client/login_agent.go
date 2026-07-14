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

	hardwarekeyagentv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/hardwarekeyagent/v1"
	pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/loginagent/v1"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/api/utils/keys/hardwarekeyagent"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/localagent"
)

// LoginAgentDirEnvVar is the name of the environment variable that will be used
// to configure the Machine ID login agent.
const LoginAgentDirEnvVar = "TELEPORT_LOGIN_AGENT_DIR"

// loginWithMachineIDAgent logs the user in non-interactively using the Machine
// ID login agent.
func (tc *TeleportClient) loginWithMachineIDAgent(agentDir string) SSHLoginFunc {
	return func(ctx context.Context, keyRing *KeyRing) (*authclient.CLILoginResponse, error) {
		conn, err := localagent.NewClient(agentDir)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		defer conn.Close()

		rsp, err := pb.NewLoginAgentServiceClient(conn).
			Login(ctx, pb.LoginRequest_builder{
				RouteToCluster:    tc.SiteName,
				KubernetesCluster: tc.KubernetesCluster,
			}.Build())
		if err != nil {
			return nil, trace.Wrap(err)
		}

		// Login agent also implements the Hardware Key Agent service so we can
		// keep the private key material in-memory.
		hwkService := hardwarekeyagent.NewService(
			hardwarekeyagentv1.NewHardwareKeyAgentServiceClient(conn),
			nil, /* fallbackService */
		)

		// Overwrite the key ring's private key. The other fields (e.g. Username,
		// Cert) are set by SSHLogin.
		privKey, err := keys.ParsePrivateKey(rsp.GetPrivateKey(), keys.WithHardwareKeyService(hwkService))
		if err != nil {
			return nil, trace.Wrap(err, "parsing private key")
		}
		keyRing.SSHPrivateKey = privKey
		keyRing.TLSPrivateKey = privKey

		// Reset outdated fields.
		keyRing.KubeTLSCredentials = make(map[string]TLSCredential)
		keyRing.AppTLSCredentials = make(map[string]TLSCredential)
		keyRing.DBTLSCredentials = make(map[string]TLSCredential)
		keyRing.WindowsDesktopTLSCredentials = make(map[string]TLSCredential)

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
