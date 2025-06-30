// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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

package vnet

import (
	"context"
	"crypto/rand"
	"fmt"
	"net"
	"strings"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"

	tracessh "github.com/gravitational/teleport/api/observability/tracing/ssh"
	"github.com/gravitational/teleport/api/utils/sshutils"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/utils"
)

// sshHandler handles incoming VNet SSH connections.
type sshHandler struct {
	cfg sshHandlerConfig
}

type sshHandlerConfig struct {
	sshProvider *sshProvider
	target      dialTarget
}

func newSSHHandler(cfg sshHandlerConfig) *sshHandler {
	return &sshHandler{
		cfg: cfg,
	}
}

// handleTCPConnector handles an incoming TCP connection from VNet and proxies
// the connection to a target SSH node.
func (h *sshHandler) handleTCPConnector(ctx context.Context, localPort uint16, connector func() (net.Conn, error)) error {
	if localPort != 22 {
		return trace.BadParameter("SSH is only handled on port 22")
	}
	agent := newSSHAgent()
	targetConn, err := h.cfg.sshProvider.dial(ctx, h.cfg.target, agent)
	if err != nil {
		return trace.Wrap(err)
	}
	defer targetConn.Close()
	return trace.Wrap(h.handleTCPConnectorWithTargetConn(ctx, connector, targetConn, agent))
}

// handleTCPConnectorWithTargetTCPConn handles an incoming TCP connection from
// VNet when a TCP connection to the target host has already been established.
func (h *sshHandler) handleTCPConnectorWithTargetConn(
	ctx context.Context,
	connector func() (net.Conn, error),
	targetConn net.Conn,
	agent *sshAgent,
) error {
	target := h.cfg.target
	hostCert, err := newHostCert(target.fqdn, h.cfg.sshProvider.hostCASigner)
	if err != nil {
		return trace.Wrap(err)
	}

	localConn, err := connector()
	if err != nil {
		return trace.Wrap(err)
	}
	defer localConn.Close()

	var (
		clientConn       *sshConn
		clientConnErr    error
		initiatedSSHConn bool
	)
	serverConfig := &ssh.ServerConfig{
		// We attempt to initiate an SSH connection with the target server in
		// PublicKeyCallback in order to fail the SSH authentication phase with
		// the client if SSH authentication to the target fails. Otherwise, when
		// connection to an SSH node the user is not allowed to access, they
		// would just see an succesfull SSH handshake and then an immediately
		// closed connection.
		//
		// TODO(nklaassen): if https://github.com/golang/go/issues/70795 ever
		// gets implemented we should do this in VerifiedPublicKeyCallback
		// instead.
		PublicKeyCallback: func(conn ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			if !sshutils.KeysEqual(h.cfg.sshProvider.trustedUserPublicKey, key) {
				return nil, trace.AccessDenied("client public key is not trusted")
			}
			// Make sure to only initiate the SSH connection once in case
			// PublicKeyCallback is called multiple times.
			if initiatedSSHConn {
				return nil, clientConnErr
			}
			initiatedSSHConn = true
			clientConn, clientConnErr = h.initiateSSHConn(ctx, targetConn, conn.User(), agent)
			if clientConnErr != nil {
				// Attempt to send a friendlier errer message if we failed to
				// initiate the SSH connection to the target by sending an auth
				// banner message.
				if utils.IsHandshakeFailedError(clientConnErr) {
					// We don't have much real information about the error in
					// this case, this is the same message tsh prints.
					return nil, &ssh.BannerError{
						Err:     clientConnErr,
						Message: formatBannerMessage(fmt.Sprintf("access denied to %s connecting to %s", conn.User(), target.hostname)),
					}
				}
				return nil, &ssh.BannerError{
					Err:     clientConnErr,
					Message: formatBannerMessage(trace.UserMessage(clientConnErr)),
				}
			}
			return nil, nil
		},
	}
	serverConfig.AddHostKey(hostCert)

	serverConn, serverChans, serverReqs, err := ssh.NewServerConn(localConn, serverConfig)
	if err != nil {
		// Make sure to close the client conn if we already accepted it.
		if clientConn != nil {
			clientConn.Close()
		}
		return trace.Wrap(err, "accepting incoming SSH connection")
	}
	log.DebugContext(ctx, "Accepted incoming SSH connection",
		"profile", target.profile,
		"cluster", target.cluster,
		"host", target.hostname,
		"user", serverConn.User(),
	)

	// proxySSHConnection transparently proxies the SSH connection from the
	// client to the target. It will handle closing the connections before it
	// returns.
	proxySSHConnection(ctx,
		sshConn{
			conn:  serverConn,
			chans: serverChans,
			reqs:  serverReqs,
		},
		*clientConn,
	)
	return nil
}

func (h *sshHandler) initiateSSHConn(ctx context.Context, targetConn net.Conn, user string, agent *sshAgent) (*sshConn, error) {
	target := h.cfg.target
	clientConfig, err := h.cfg.sshProvider.sessionSSHConfig(ctx, target, user, agent)
	if err != nil {
		return nil, trace.Wrap(err, "building SSH client config")
	}
	clientConn, clientChans, clientReqs, err := tracessh.NewClientConn(ctx, targetConn, target.addr, clientConfig)
	if err != nil {
		return nil, trace.Wrap(err, "initiating SSH connection to %s@%s", user, target.addr)
	}
	log.DebugContext(ctx, "Initiated SSH connection to target", "root_cluster", target.rootCluster,
		"leaf_cluster", target.leafCluster, "host", target.addr)
	return &sshConn{
		conn:  clientConn,
		chans: clientChans,
		reqs:  clientReqs,
	}, nil
}

func newHostCert(fqdn string, ca ssh.Signer) (ssh.Signer, error) {
	// If the user typed "ssh host.com" or "ssh host.com." our DNS handler will
	// only see the fully-qualified variant with the trailing "." but the SSH
	// client treats them differently, we need both in the principals if we want
	// the cert to be trusted in both cases.
	validPrincipals := []string{
		fqdn,
		strings.TrimSuffix(fqdn, "."),
	}
	// We generate an ephemeral key for every connection, Ed25519 is fast and
	// well supported.
	hostKey, err := cryptosuites.GenerateKeyWithAlgorithm(cryptosuites.Ed25519)
	if err != nil {
		return nil, trace.Wrap(err, "generating SSH host key")
	}
	hostSigner, err := ssh.NewSignerFromSigner(hostKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	cert := &ssh.Certificate{
		Key:             hostSigner.PublicKey(),
		Serial:          1,
		CertType:        ssh.HostCert,
		ValidPrincipals: validPrincipals,
		// This cert will only ever be used to handle this one SSH connection,
		// the private key is held only in memory, the issuing CA is regenerated
		// every time this process restarts and will only be trusted on this one
		// host. The expiry doesn't matter.
		ValidBefore: ssh.CertTimeInfinity,
	}
	if err := cert.SignCert(rand.Reader, ca); err != nil {
		return nil, trace.Wrap(err, "signing SSH host cert")
	}
	certSigner, err := ssh.NewCertSigner(cert, hostSigner)
	return certSigner, trace.Wrap(err)
}

func formatBannerMessage(msg string) string {
	return "VNet: " + msg + "\n"
}
