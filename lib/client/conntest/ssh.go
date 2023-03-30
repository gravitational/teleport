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

// SSHConnectionTesterConfig has the necessary fields to create a new SSHConnectionTester.
type SSHConnectionTesterConfig struct {
	// UserClient is an auth client that has a User's identity.
	// This is the user that is running the SSH Connection Test.
	UserClient auth.ClientI

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

	key, err := client.GenerateRSAKey()
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
		PublicKey:              key.MarshalSSHPublicKey(),
		Username:               currentUser.GetName(),
		Expires:                time.Now().Add(time.Minute).UTC(),
		ConnectionDiagnosticID: connectionDiagnosticID,
		MFAResponse:            mfaResponse,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	key.Cert = certs.SSH
	key.TLSCert = certs.TLS

	certAuths, err := s.cfg.UserClient.GetCertAuthorities(ctx, types.HostCA, false /* loadKeys */)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	hostkeyCallback, err := hostKeyCallbackFromCAs(certAuths)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	key.TrustedCerts = auth.AuthoritiesToTrustedCerts(certAuths)

	keyAuthMethod, err := key.AsAuthMethod()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	clusterName, err := s.cfg.UserClient.GetClusterName()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	clientConfTLS, err := key.TeleportClientTLSConfig(nil, []string{clusterName.GetClusterName()})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	key.KeyIndex = client.KeyIndex{
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

	if err := tc.SSH(ctxWithTimeout, []string{"whoami"}, false); err != nil {
		return s.handleErrFromSSH(ctx, connectionDiagnosticID, req.SSHPrincipal, err, processStdout)
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

func (s SSHConnectionTester) handleErrFromSSH(ctx context.Context, connectionDiagnosticID string, sshPrincipal string, sshError error, processStdout *bytes.Buffer) (types.ConnectionDiagnostic, error) {
	if trace.IsConnectionProblem(sshError) {
		connDiag, err := s.cfg.UserClient.AppendDiagnosticTrace(ctx, connectionDiagnosticID, types.NewTraceDiagnosticConnection(
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
	if strings.HasPrefix(processStdoutString, "Failed to launch: user:") {
		connDiag, err := s.cfg.UserClient.AppendDiagnosticTrace(ctx, connectionDiagnosticID, types.NewTraceDiagnosticConnection(
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
