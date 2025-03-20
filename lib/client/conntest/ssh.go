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

package conntest

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/api/utils/sshutils"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/connectmycomputer"
	"github.com/gravitational/teleport/lib/cryptosuites"
	libsshutils "github.com/gravitational/teleport/lib/sshutils"
)

// SSHConnectionTesterConfig has the necessary fields to create a new SSHConnectionTester.
type SSHConnectionTesterConfig struct {
	// UserClient is an auth client that has a User's identity.
	// This is the user that is running the SSH Connection Test.
	UserClient authclient.ClientI

	// ProxyHostPort is the proxy to use in the `--proxy` format (host:webPort,sshPort)
	ProxyHostPort string

	// TLSRoutingEnabled indicates that proxy supports ALPN SNI server where
	// all proxy services are exposed on a single TLS listener (Proxy Web Listener).
	TLSRoutingEnabled bool
}

// SSHConnectionTester implements the ConnectionTester interface for Testing SSH access
type SSHConnectionTester struct {
	cfg          SSHConnectionTesterConfig
	webProxyAddr string
	sshProxyAddr string
}

// NewSSHConnectionTester creates a new SSHConnectionTester
func NewSSHConnectionTester(cfg SSHConnectionTesterConfig) (*SSHConnectionTester, error) {
	parsedProxyHostAddr, err := client.ParseProxyHost(cfg.ProxyHostPort)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &SSHConnectionTester{
		cfg:          cfg,
		webProxyAddr: parsedProxyHostAddr.WebProxyAddr,
		sshProxyAddr: parsedProxyHostAddr.SSHProxyAddr,
	}, nil
}

// TestConnection tests an SSH Connection to the target Node using
//   - the provided client
//   - resource name
//   - principal / linux user
//
// A new ConnectionDiagnostic is created and used to store the traces as it goes through the checkpoints
// To set up the SSH client, it will generate a new cert and inject the ConnectionDiagnosticID
//   - add a trace of whether the SSH Node was reachable
//   - SSH Node receives the cert and extracts the ConnectionDiagnostiID
//   - the SSH Node will append a trace indicating if the has access (RBAC)
//   - the SSH Node will append a trace indicating if the requested principal is valid for the target Node
func (s *SSHConnectionTester) TestConnection(ctx context.Context, req TestConnectionRequest) (types.ConnectionDiagnostic, error) {
	if req.ResourceKind != types.KindNode {
		return nil, trace.BadParameter("invalid value for ResourceKind, expected %q got %q", types.KindNode, req.ResourceKind)
	}

	connectionDiagnosticID := uuid.NewString()
	connectionDiagnostic, err := types.NewConnectionDiagnosticV1(connectionDiagnosticID, map[string]string{},
		types.ConnectionDiagnosticSpecV1{
			// We start with a failed state so that we don't need an extra update when returning non-happy paths.
			// For the happy path, we do update the Message to Success.
			Message: types.DiagnosticMessageFailed,
		})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.cfg.UserClient.CreateConnectionDiagnostic(ctx, connectionDiagnostic); err != nil {
		return nil, trace.Wrap(err)
	}

	key, err := cryptosuites.GenerateKey(ctx,
		cryptosuites.GetCurrentSuiteFromAuthPreference(s.cfg.UserClient),
		cryptosuites.UserSSH)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	privateKey, err := keys.NewSoftwarePrivateKey(key)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	keyRing := client.NewKeyRing(privateKey, privateKey)

	tlsPub, err := keyRing.TLSPrivateKey.MarshalTLSPublicKey()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	currentUser, err := s.cfg.UserClient.GetCurrentUser(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	mfaResponse, err := req.MFAResponse.GetOptionalMFAResponseProtoReq()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	certs, err := s.cfg.UserClient.GenerateUserCerts(ctx, proto.UserCertsRequest{
		SSHPublicKey:           keyRing.SSHPrivateKey.MarshalSSHPublicKey(),
		TLSPublicKey:           tlsPub,
		Username:               currentUser.GetName(),
		Expires:                time.Now().Add(time.Minute).UTC(),
		ConnectionDiagnosticID: connectionDiagnosticID,
		MFAResponse:            mfaResponse,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	keyRing.Cert = certs.SSH
	keyRing.TLSCert = certs.TLS

	certAuths, err := s.cfg.UserClient.GetCertAuthorities(ctx, types.HostCA, false /* loadKeys */)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	hostkeyCallback, err := hostKeyCallbackFromCAs(certAuths)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	keyRing.TrustedCerts = authclient.AuthoritiesToTrustedCerts(certAuths)

	keyAuthMethod, err := keyRing.AsAuthMethod()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	clusterName, err := s.cfg.UserClient.GetClusterName(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	clientConfTLS, err := keyRing.TeleportClientTLSConfig(nil, []string{clusterName.GetClusterName()})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	keyRing.KeyRingIndex = client.KeyRingIndex{
		Username:    req.SSHPrincipal,
		ProxyHost:   s.webProxyAddr,
		ClusterName: clusterName.GetClusterName(),
	}

	processStdout := &bytes.Buffer{}

	clientConf := client.MakeDefaultConfig()
	clientConf.AddKeysToAgent = client.AddKeysToAgentNo
	clientConf.AuthMethods = []ssh.AuthMethod{keyAuthMethod}
	clientConf.Host = req.ResourceName
	clientConf.HostKeyCallback = hostkeyCallback
	clientConf.HostLogin = req.SSHPrincipal
	clientConf.NonInteractive = true
	clientConf.SSHProxyAddr = s.sshProxyAddr
	clientConf.Stderr = io.Discard
	clientConf.Stdin = &bytes.Buffer{}
	clientConf.Stdout = processStdout
	clientConf.TLS = clientConfTLS
	clientConf.TLSRoutingEnabled = s.cfg.TLSRoutingEnabled
	clientConf.Username = currentUser.GetName()
	clientConf.WebProxyAddr = s.webProxyAddr
	clientConf.SiteName = clusterName.GetClusterName()

	tc, err := client.NewClient(clientConf)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ctxWithTimeout, cancelFunc := context.WithTimeout(ctx, req.DialTimeout)
	defer cancelFunc()

	if err := tc.SSH(ctxWithTimeout, []string{"whoami"}); err != nil {
		return s.handleErrFromSSH(ctx, connectionDiagnosticID, req.SSHPrincipal, err, processStdout, currentUser, req)
	}

	connDiag, err := s.cfg.UserClient.AppendDiagnosticTrace(ctx, connectionDiagnosticID, types.NewTraceDiagnosticConnection(
		types.ConnectionDiagnosticTrace_NODE_PRINCIPAL,
		fmt.Sprintf("%q user exists in target node", req.SSHPrincipal),
		nil,
	))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	connDiag.SetMessage(types.DiagnosticMessageSuccess)
	connDiag.SetSuccess(true)

	if err := s.cfg.UserClient.UpdateConnectionDiagnostic(ctx, connDiag); err != nil {
		return nil, trace.Wrap(err)
	}

	return connDiag, nil
}

func (s SSHConnectionTester) handleErrFromSSH(ctx context.Context, connectionDiagnosticID string,
	sshPrincipal string, sshError error, processStdout *bytes.Buffer, currentUser types.User, req TestConnectionRequest) (types.ConnectionDiagnostic, error) {
	isConnectMyComputerNode := req.SSHNodeSetupMethod == SSHNodeSetupMethodConnectMyComputer

	if trace.IsConnectionProblem(sshError) {
		var statusCommand string
		if req.SSHNodeOS == constants.DarwinOS {
			statusCommand = "launchctl print 'system/Teleport Service'"
		} else {
			statusCommand = "systemctl status teleport"
		}

		message := fmt.Sprintf(`Failed to connect to the Node. Ensure teleport service is running using "%s".`, statusCommand)
		if isConnectMyComputerNode {
			message = "Failed to connect to the Node. Open the Connect My Computer tab in Teleport Connect and make sure that the agent is running."
		}

		connDiag, err := s.cfg.UserClient.AppendDiagnosticTrace(ctx, connectionDiagnosticID, types.NewTraceDiagnosticConnection(
			types.ConnectionDiagnosticTrace_CONNECTIVITY,
			message,
			sshError,
		))
		if err != nil {
			return nil, trace.Wrap(err)
		}

		return connDiag, nil
	}

	processStdoutString := strings.TrimSpace(processStdout.String())
	// If the selected principal does not exist on the node, attempting to connect emits:
	// "Failed to launch: user: lookup username <principal>: no such file or directory."
	isUsernameLookupFail := strings.HasPrefix(processStdoutString, "Failed to launch: user:")
	// Connect My Computer runs the agent as non-root. When attempting to connect as another system
	// user that is not the same as the user who runs the agent, the emitted error is "Failed to
	// launch: fork/exec <conn.User shell>: operation not permitted."
	isForkExecOperationNotPermitted := strings.HasPrefix(processStdoutString, "Failed to launch: fork/exec") &&
		strings.Contains(processStdoutString, "operation not permitted")
	// "operation not permitted" is handled only for the Connect My Computer case as we assume that
	// regular SSH nodes are started as root and are unlikely to run into this error.
	isInvalidNodePrincipal := isUsernameLookupFail || (isConnectMyComputerNode && isForkExecOperationNotPermitted)

	if isInvalidNodePrincipal {
		message := fmt.Sprintf("Invalid user. Please ensure the principal %q is a valid login in the target node. Output from Node: %v",
			sshPrincipal, processStdoutString)
		if isConnectMyComputerNode {
			connectMyComputerRoleName := connectmycomputer.GetRoleNameForUser(currentUser.GetName())
			message = "Invalid user."
			outputFromAgent := fmt.Sprintf("Output from the Connect My Computer agent: %v", processStdoutString)
			retrySetupInstructions := "reload this page, pick Connect My Computer again, then in Teleport Connect " +
				"remove the Connect My Computer agent and start Connect My Computer setup again."

			var detailedMessage string
			if req.SSHPrincipalSelectionMode == SSHPrincipalSelectionModeManual {
				detailedMessage = "You probably picked a login which does not match the system user " +
					"that is running Teleport Connect. Pick the correct login and try again.\n\n" +
					"If the list of logins does not include the correct login for this node, " +
					retrySetupInstructions + "\n\n" + outputFromAgent
			} else {
				detailedMessage = fmt.Sprintf("The role %q includes only the login %q and %q is not a valid principal for this node. ",
					connectMyComputerRoleName, sshPrincipal, sshPrincipal) +
					"To fix this problem, " + retrySetupInstructions + "\n\n" + outputFromAgent
			}

			// The wrapping here is done so that the detailed message will be shown under "Show details"
			// and not as one of the main points of the connection test.
			sshError = trace.Wrap(sshError, detailedMessage)
		}

		connDiag, err := s.cfg.UserClient.AppendDiagnosticTrace(ctx, connectionDiagnosticID, types.NewTraceDiagnosticConnection(
			types.ConnectionDiagnosticTrace_NODE_PRINCIPAL,
			message,
			sshError,
		))
		if err != nil {
			return nil, trace.Wrap(err)
		}

		return connDiag, nil
	}

	// This happens when the principal is not part of the allowed ones.
	// A trace was already added by the Node and, here, we just return the diagnostic.
	if trace.IsAccessDenied(sshError) {
		connDiag, err := s.cfg.UserClient.GetConnectionDiagnostic(ctx, connectionDiagnosticID)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		return connDiag, nil
	}

	connDiag, err := s.cfg.UserClient.AppendDiagnosticTrace(ctx, connectionDiagnosticID, types.NewTraceDiagnosticConnection(
		types.ConnectionDiagnosticTrace_UNKNOWN_ERROR,
		fmt.Sprintf("Unknown error. %s", processStdoutString),
		sshError,
	))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return connDiag, nil
}

func hostKeyCallbackFromCAs(certAuths []types.CertAuthority) (ssh.HostKeyCallback, error) {
	var certPublicKeys []ssh.PublicKey
	for _, ca := range certAuths {
		caCheckers, err := libsshutils.GetCheckers(ca)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		certPublicKeys = append(certPublicKeys, caCheckers...)
	}

	hostKeyCallback, err := sshutils.NewHostKeyCallback(sshutils.HostKeyCallbackConfig{
		GetHostCheckers: func() ([]ssh.PublicKey, error) {
			return certPublicKeys, nil
		},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return hostKeyCallback, nil
}
