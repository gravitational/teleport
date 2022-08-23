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
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/sshutils"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/client"
	libsshutils "github.com/gravitational/teleport/lib/sshutils"
)

// SSHConnectionTester implements the ConnectionTester interface for Testing SSH access
type SSHConnectionTester struct {
	clt auth.ClientI
}

// NewSSHConnectionTester creates a new SSHConnectionTester
func NewSSHConnectionTester(clt auth.ClientI) *SSHConnectionTester {
	return &SSHConnectionTester{
		clt: clt,
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
			Message: types.DiagnosticMessageFailed,
		})
	if err != nil {
		return nil, trace.Wrap(err, "new conn diag")
	}

	if err := s.clt.CreateConnectionDiagnostic(ctx, connectionDiagnostic); err != nil {
		return nil, trace.Wrap(err, "create conn diag")
	}

	key, err := client.GenerateRSAKey()
	if err != nil {
		return nil, trace.Wrap(err, "generate rsa key")
	}

	currentUser, err := s.clt.GetCurrentUser(ctx)
	if err != nil {
		return nil, trace.Wrap(err, "current user")
	}

	certs, err := s.clt.GenerateUserCerts(ctx, proto.UserCertsRequest{
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

	certAuths, err := s.clt.GetCertAuthorities(ctx, types.HostCA, false /* loadKeys */)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	hostkeyCallback, err := hostkeyCallbackFromCAs(certAuths)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	key.TrustedCA = auth.AuthoritiesToTrustedCerts(certAuths)

	keyAuthMethod, err := key.AsAuthMethod()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	clusterName, err := s.clt.GetClusterName()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	clientConfTLS, err := key.TeleportClientTLSConfig(nil, []string{clusterName.GetClusterName()})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	host := req.ResourceName
	tlsRoutingEnabled := true

	webProxyAddr, sshProxyAddr, err := s.proxyAddrs()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	key.KeyIndex = client.KeyIndex{
		Username:    req.SSHPrincipal,
		ProxyHost:   webProxyAddr,
		ClusterName: clusterName.GetClusterName(),
	}

	processStdout := &bytes.Buffer{}

	clientConf := client.MakeDefaultConfig()
	clientConf.AuthMethods = []ssh.AuthMethod{keyAuthMethod}
	clientConf.Host = host
	clientConf.HostKeyCallback = hostkeyCallback
	clientConf.HostLogin = req.SSHPrincipal
	clientConf.SkipLocalAuth = true
	clientConf.SSHProxyAddr = sshProxyAddr // TODO(marco): remove the next line?
	clientConf.SSHProxyAddr = webProxyAddr
	clientConf.Stderr = io.Discard
	clientConf.Stdin = &bytes.Buffer{}
	clientConf.Stdout = processStdout
	clientConf.TLS = clientConfTLS
	clientConf.TLSRoutingEnabled = tlsRoutingEnabled
	clientConf.UseKeyPrincipals = true
	clientConf.Username = currentUser.GetName()
	clientConf.WebProxyAddr = webProxyAddr
	clientConf.AddKeysToAgent = client.AddKeysToAgentNo

	teleClient, err := client.NewClient(clientConf)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ctxWithTimeout, cancelFunc := context.WithTimeout(ctx, req.DialTimeout)
	defer cancelFunc()

	if err := teleClient.SSH(ctxWithTimeout, []string{"whoami"}, false); err != nil {
		return s.handleErrFromSSH(ctx, connectionDiagnosticID, req.SSHPrincipal, err, processStdout)
	}

	connDiag, err := s.clt.AppendDiagnosticTrace(ctx, connectionDiagnosticID, types.NewSuccessTraceConnectionDiagnostic(
		types.ConnectionDiagnosticTrace_NODE_PRINCIPAL,
		fmt.Sprintf("%q user exists in target node", req.SSHPrincipal),
	))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	connDiag.SetMessage(types.DiagnosticMessageSuccess)
	connDiag.SetSuccess(true)

	if err := s.clt.UpdateConnectionDiagnostic(ctx, connDiag); err != nil {
		return nil, trace.Wrap(err)
	}

	return connDiag, nil
}

func (s SSHConnectionTester) proxyAddrs() (webProxyAddr string, sshProxyAddr string, err error) {
	proxies, err := s.clt.GetProxies()
	if err != nil {
		return "", "", trace.Wrap(err)
	}

	if len(proxies) == 0 {
		return "", "", trace.NotFound("cluster has no proxies")
	}

	parsedAddrs, err := client.ParseProxyHost(proxies[0].GetAddr())
	if err != nil {
		return "", "", trace.Wrap(err)
	}

	return parsedAddrs.WebProxyAddr, parsedAddrs.SSHProxyAddr, nil
}

func (s SSHConnectionTester) handleErrFromSSH(ctx context.Context, connectionDiagnosticID string, sshPrincipal string, sshError error, processStdout *bytes.Buffer) (types.ConnectionDiagnostic, error) {
	// Either the node doesn't exist or the user doesn't have access to it.
	if trace.IsNotFound(sshError) {
		connDiag, err := s.clt.AppendDiagnosticTrace(ctx, connectionDiagnosticID, types.NewFailedTraceConnectionDiagnostic(
			types.ConnectionDiagnosticTrace_RBAC_NODE,
			"Node not found. Ensure the Node exists and your role allows you to access it.",
			sshError,
		))
		if err != nil {
			return nil, trace.Wrap(err)
		}

		return connDiag, nil
	}

	if trace.IsConnectionProblem(sshError) {
		connDiag, err := s.clt.AppendDiagnosticTrace(ctx, connectionDiagnosticID, types.NewFailedTraceConnectionDiagnostic(
			types.ConnectionDiagnosticTrace_CONNECTIVITY,
			"Failed to connect to the Node. Ensure teleport is running.",
			sshError,
		))
		if err != nil {
			return nil, trace.Wrap(err)
		}

		return connDiag, nil
	}

	processStdoutString := strings.TrimSpace(processStdout.String())

	if strings.Contains(sshError.Error(), "Process exited with status 255") && strings.HasPrefix(processStdoutString, "Failed to launch: user: unknown user") {
		connDiag, err := s.clt.AppendDiagnosticTrace(ctx, connectionDiagnosticID, types.NewFailedTraceConnectionDiagnostic(
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
		connDiag, err := s.clt.GetConnectionDiagnostic(ctx, connectionDiagnosticID)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		return connDiag, nil
	}

	connDiag, err := s.clt.AppendDiagnosticTrace(ctx, connectionDiagnosticID, types.NewFailedTraceConnectionDiagnostic(
		types.ConnectionDiagnosticTrace_UNKNOWN_ERROR,
		"Unknown error.",
		sshError,
	))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return connDiag, nil
}

func hostkeyCallbackFromCAs(certAuths []types.CertAuthority) (ssh.HostKeyCallback, error) {
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
