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
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/defaults"
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
	connectionDiagnosticID := uuid.NewString()
	connectionDiagnostic, err := types.NewConnectionDiagnosticV1(connectionDiagnosticID, map[string]string{},
		types.ConnectionDiagnosticSpecV1{
			Message: types.DiagnosticMessageFailed,
		})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.clt.CreateConnectionDiagnostic(ctx, connectionDiagnostic); err != nil {
		return nil, trace.Wrap(err)
	}

	sshNode, err := s.clt.GetNode(ctx, defaults.Namespace, req.ResourceName)
	if err != nil {
		connDiag, err := s.clt.AppendTraceConnectionDiagnostic(ctx, connectionDiagnosticID, types.NewFailedTraceConnectionDiagnostic(
			uuid.NewString(),
			types.DiagnosticTraceTypeRBACNode,
			"Node not found. Ensure the Node exists and your role allows you to access it.",
			err,
		))
		if err != nil {
			return nil, trace.Wrap(err)
		}

		return connDiag, nil
	}

	_, err = s.clt.AppendTraceConnectionDiagnostic(ctx, connectionDiagnosticID, types.NewSuccessTraceConnectionDiagnostic(
		uuid.NewString(),
		types.DiagnosticTraceTypeRBACNode,
		"Resource exists.",
	))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	sshNodeAddr := sshNode.GetAddr()

	key, err := client.NewKey()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	currentUser, err := s.clt.GetCurrentUser(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	certs, err := s.clt.GenerateUserCerts(ctx, proto.UserCertsRequest{
		NodeName:               sshNode.GetName(),
		PublicKey:              key.Pub,
		Username:               currentUser.GetName(),
		Expires:                time.Now().Add(time.Minute).UTC(),
		Format:                 constants.CertificateFormatStandard,
		Usage:                  proto.UserCertsRequest_SSH,
		ConnectionDiagnosticID: connectionDiagnosticID,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	key.Cert = certs.SSH

	// get certificate authorities
	certAuths, err := s.clt.GetCertAuthorities(ctx, types.HostCA, false)
	if err != nil {
		return nil, trace.Wrap(err)
	}

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

	sshClientConfig, err := key.ProxyClientSSHConfig(nil, sshNodeAddr)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	sshClientConfig.HostKeyCallback = hostKeyCallback
	sshClientConfig.User = req.SSHPrincipal

	dialCtx, cancelFunc := context.WithTimeout(ctx, req.DialTimeout)
	defer cancelFunc()

	var dialer net.Dialer
	conn, err := dialer.DialContext(dialCtx, "tcp", sshNodeAddr)
	if err != nil {
		connDiag, err := s.clt.AppendTraceConnectionDiagnostic(ctx, connectionDiagnosticID, types.NewFailedTraceConnectionDiagnostic(
			uuid.NewString(),
			types.DiagnosticTraceTypeConnectivity,
			"Failed to access the host. Please ensure it's network reachable.",
			err,
		))
		if err != nil {
			return nil, trace.Wrap(err)
		}

		return connDiag, nil
	}
	defer conn.Close()

	_, err = s.clt.AppendTraceConnectionDiagnostic(ctx, connectionDiagnosticID, types.NewSuccessTraceConnectionDiagnostic(
		uuid.NewString(),
		types.DiagnosticTraceTypeConnectivity,
		"Host is alive and reachable.",
	))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	sshConn, sshNewChannel, sshReq, err := ssh.NewClientConn(conn, sshNodeAddr, sshClientConfig)
	if err != nil {
		connDiag, err := s.clt.AppendTraceConnectionDiagnostic(ctx, connectionDiagnosticID, types.NewFailedTraceConnectionDiagnostic(
			uuid.NewString(),
			types.DiagnosticTraceTypeNodeSSHServer,
			"Failed to open SSH connection. Please ensure Teleport is running.",
			err,
		))
		if err != nil {
			return nil, trace.Wrap(err)
		}

		return connDiag, nil
	}

	_, err = s.clt.AppendTraceConnectionDiagnostic(ctx, connectionDiagnosticID, types.NewSuccessTraceConnectionDiagnostic(
		uuid.NewString(),
		types.DiagnosticTraceTypeNodeSSHServer,
		"Established an SSH connection.",
	))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	sshClient := ssh.NewClient(sshConn, sshNewChannel, sshReq)

	sshSession, err := sshClient.NewSession()
	if err != nil {
		connDiag, err := s.clt.AppendTraceConnectionDiagnostic(ctx, connectionDiagnosticID, types.NewFailedTraceConnectionDiagnostic(
			uuid.NewString(),
			types.DiagnosticTraceTypeNodeSSHSession,
			"Failed to create a new SSH Session. Please ensure Teleport is running.",
			err,
		))
		if err != nil {
			return nil, trace.Wrap(err)
		}

		return connDiag, nil
	}
	defer sshSession.Close()

	_, err = s.clt.AppendTraceConnectionDiagnostic(ctx, connectionDiagnosticID, types.NewSuccessTraceConnectionDiagnostic(
		uuid.NewString(),
		types.DiagnosticTraceTypeNodeSSHSession,
		"Created an SSH session.",
	))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	output, err := sshSession.CombinedOutput("whoami")
	if err != nil {
		connDiag, err := s.clt.AppendTraceConnectionDiagnostic(ctx, connectionDiagnosticID, types.NewFailedTraceConnectionDiagnostic(
			uuid.NewString(),
			types.DiagnosticTraceTypeNodePrincipal,
			fmt.Sprintf("Failed to query the current user in the target node. Please ensure the principal %q is a valid Linux login in the target node. Output from Node: %s", sshClientConfig.User, strings.TrimSpace(string(output))),
			err,
		))
		if err != nil {
			return nil, trace.Wrap(err)
		}

		return connDiag, nil
	}
	whoamiOutput := strings.TrimSpace(string(output))

	connDiag, err := s.clt.AppendTraceConnectionDiagnostic(ctx, connectionDiagnosticID, types.NewSuccessTraceConnectionDiagnostic(
		uuid.NewString(),
		types.DiagnosticTraceTypeNodePrincipal,
		fmt.Sprintf("%q user exists in target node", whoamiOutput),
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
