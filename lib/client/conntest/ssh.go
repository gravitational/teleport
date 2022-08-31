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

package conntest

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/sshutils"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/client"
	libsshutils "github.com/gravitational/teleport/lib/sshutils"
)

// SSHConnectionTester implements the ConnectionTester interface for Testing SSH access
type SSHConnectionTester struct {
	userClt  auth.ClientI
	proxyClt auth.ClientI
}

// NewSSHConnectionTester creates a new SSHConnectionTester
func NewSSHConnectionTester(userClt auth.ClientI, proxyClt auth.ClientI) *SSHConnectionTester {
	return &SSHConnectionTester{
		userClt:  userClt,
		proxyClt: proxyClt,
	}
}

// TestConnection tests an SSH Connection to the target Node using
//  - the provided client
//  - resource name
//  - principal / linux user
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

	if err := s.userClt.CreateConnectionDiagnostic(ctx, connectionDiagnostic); err != nil {
		return nil, trace.Wrap(err)
	}

	key, err := client.GenerateRSAKey()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	currentUser, err := s.userClt.GetCurrentUser(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	certs, err := s.userClt.GenerateUserCerts(ctx, proto.UserCertsRequest{
		PublicKey:              key.MarshalSSHPublicKey(),
		Username:               currentUser.GetName(),
		Expires:                time.Now().Add(time.Minute).UTC(),
		ConnectionDiagnosticID: connectionDiagnosticID,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	key.Cert = certs.SSH
	key.TLSCert = certs.TLS

	certAuths, err := s.userClt.GetCertAuthorities(ctx, types.HostCA, false /* loadKeys */)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	hostkeyCallback, err := hostKeyCallbackFromCAs(certAuths)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	key.TrustedCA = auth.AuthoritiesToTrustedCerts(certAuths)

	keyAuthMethod, err := key.AsAuthMethod()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	clusterName, err := s.userClt.GetClusterName()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	clientConfTLS, err := key.TeleportClientTLSConfig(nil, []string{clusterName.GetClusterName()})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	host := req.ResourceName
	hostPort := 0
	tlsRoutingEnabled := req.TLSRoutingEnabled

	parsedProxyHostAddr, err := client.ParseProxyHost(req.ProxyHostPort)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	webProxyAddr := parsedProxyHostAddr.WebProxyAddr
	sshProxyAddr := parsedProxyHostAddr.SSHProxyAddr

	key.KeyIndex = client.KeyIndex{
		Username:    req.SSHPrincipal,
		ProxyHost:   webProxyAddr,
		ClusterName: clusterName.GetClusterName(),
	}

	if !tlsRoutingEnabled {
		node, err := s.proxyClt.GetNode(ctx, defaults.Namespace, req.ResourceName)
		if err != nil {
			if trace.IsNotFound(err) {
				connDiag, err := s.userClt.AppendDiagnosticTrace(ctx, connectionDiagnosticID, types.NewTraceDiagnosticConnection(
					types.ConnectionDiagnosticTrace_RBAC_NODE,
					"Node not found. Ensure the Node exists and your role allows you to access it.",
					err,
				))
				if err != nil {
					return nil, trace.Wrap(err)
				}

				return connDiag, nil
			}
			return nil, trace.Wrap(err)
		}

		addrParts := strings.Split(node.GetAddr(), ":")
		if len(addrParts) != 2 {
			return nil, trace.BadParameter("invalid node address: %v", node.GetAddr())
		}

		host = addrParts[0]

		hostPort, err = strconv.Atoi(addrParts[1])
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	processStdout := &bytes.Buffer{}

	clientConf := client.MakeDefaultConfig()
	clientConf.AddKeysToAgent = client.AddKeysToAgentNo
	clientConf.AuthMethods = []ssh.AuthMethod{keyAuthMethod}
	clientConf.Host = host
	clientConf.HostPort = hostPort
	clientConf.HostKeyCallback = hostkeyCallback
	clientConf.HostLogin = req.SSHPrincipal
	clientConf.SkipLocalAuth = true
	clientConf.SSHProxyAddr = sshProxyAddr
	clientConf.Stderr = io.Discard
	clientConf.Stdin = &bytes.Buffer{}
	clientConf.Stdout = processStdout
	clientConf.TLS = clientConfTLS
	clientConf.TLSRoutingEnabled = tlsRoutingEnabled
	clientConf.UseKeyPrincipals = true
	clientConf.Username = currentUser.GetName()
	clientConf.WebProxyAddr = webProxyAddr

	tc, err := client.NewClient(clientConf)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ctxWithTimeout, cancelFunc := context.WithTimeout(ctx, req.DialTimeout)
	defer cancelFunc()

	if err := tc.SSH(ctxWithTimeout, []string{"whoami"}, false); err != nil {
		return s.handleErrFromSSH(ctx, connectionDiagnosticID, req.SSHPrincipal, err, processStdout)
	}

	connDiag, err := s.userClt.AppendDiagnosticTrace(ctx, connectionDiagnosticID, types.NewTraceDiagnosticConnection(
		types.ConnectionDiagnosticTrace_NODE_PRINCIPAL,
		fmt.Sprintf("%q user exists in target node", req.SSHPrincipal),
		nil,
	))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	connDiag.SetMessage(types.DiagnosticMessageSuccess)
	connDiag.SetSuccess(true)

	if err := s.userClt.UpdateConnectionDiagnostic(ctx, connDiag); err != nil {
		return nil, trace.Wrap(err)
	}

	return connDiag, nil
}

func (s SSHConnectionTester) handleErrFromSSH(ctx context.Context, connectionDiagnosticID string, sshPrincipal string, sshError error, processStdout *bytes.Buffer) (types.ConnectionDiagnostic, error) {
	if trace.IsConnectionProblem(sshError) {
		connDiag, err := s.userClt.AppendDiagnosticTrace(ctx, connectionDiagnosticID, types.NewTraceDiagnosticConnection(
			types.ConnectionDiagnosticTrace_CONNECTIVITY,
			`Failed to connect to the Node. Ensure teleport service is running using "systemctl status teleport".`,
			sshError,
		))
		if err != nil {
			return nil, trace.Wrap(err)
		}

		return connDiag, nil
	}

	processStdoutString := strings.TrimSpace(processStdout.String())
	if strings.HasPrefix(processStdoutString, "Failed to launch: user: unknown user") {
		connDiag, err := s.userClt.AppendDiagnosticTrace(ctx, connectionDiagnosticID, types.NewTraceDiagnosticConnection(
			types.ConnectionDiagnosticTrace_NODE_PRINCIPAL,
			fmt.Sprintf("Invalid user. Please ensure the principal %q is a valid Linux login in the target node. Output from Node: %v", sshPrincipal, processStdoutString),
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
		connDiag, err := s.userClt.GetConnectionDiagnostic(ctx, connectionDiagnosticID)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		return connDiag, nil
	}

	connDiag, err := s.userClt.AppendDiagnosticTrace(ctx, connectionDiagnosticID, types.NewTraceDiagnosticConnection(
		types.ConnectionDiagnosticTrace_UNKNOWN_ERROR,
		"Unknown error.",
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
