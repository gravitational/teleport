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
	"net"
	"strings"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/api/utils/sshutils"
	"github.com/gravitational/teleport/lib/cryptosuites"
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
	targetConn, err := h.cfg.sshProvider.dial(ctx, h.cfg.target)
	if err != nil {
		return trace.Wrap(err)
	}
	defer targetConn.Close()
	return trace.Wrap(h.handleTCPConnectorWithTargetConn(ctx, localPort, connector, targetConn))
}

// handleTCPConnectorWithTargetTCPConn handles an incoming TCP connection from
// VNet when a TCP connection to the target host has already been established.
func (h *sshHandler) handleTCPConnectorWithTargetConn(
	ctx context.Context,
	localPort uint16,
	connector func() (net.Conn, error),
	targetConn net.Conn,
) error {
	hostCert, err := h.newHostCert(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	localConn, err := connector()
	if err != nil {
		return trace.Wrap(err)
	}
	defer localConn.Close()

	// For now we accept the incoming SSH connection but forwarding to the
	// target is not implemented yet so we immediately close it.
	serverConfig := &ssh.ServerConfig{
		PublicKeyCallback: func(conn ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			if !sshutils.KeysEqual(h.cfg.sshProvider.trustedUserPublicKey, key) {
				return nil, trace.AccessDenied("SSH client public key is not trusted")
			}
			return nil, nil
		},
	}
	serverConfig.AddHostKey(hostCert)
	serverConn, chans, reqs, err := ssh.NewServerConn(localConn, serverConfig)
	if err != nil {
		return trace.Wrap(err, "accepting incoming SSH connection")
	}
	// Immediately close the connection but make sure to drain the channels.
	serverConn.Close()
	go ssh.DiscardRequests(reqs)
	go func() {
		for newChan := range chans {
			_ = newChan.Reject(0, "")
		}
	}()
	target := h.cfg.target
	log.DebugContext(ctx, "Accepted incoming SSH connection",
		"profile", target.profile,
		"cluster", target.cluster,
		"host", target.host,
		"user", serverConn.User(),
	)
	return trace.NotImplemented("VNet SSH connection forwarding is not yet implemented")
}

func (h *sshHandler) newHostCert(ctx context.Context) (ssh.Signer, error) {
	// If the user typed "ssh host.com" or "ssh host.com." our DNS handler will
	// only see the fully-qualified variant with the trailing "." but the SSH
	// client treats them differently, we need both in the principals if we want
	// the cert to be trusted in both cases.
	validPrincipals := []string{
		h.cfg.target.fqdn,
		strings.TrimSuffix(h.cfg.target.fqdn, "."),
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
	if err := cert.SignCert(rand.Reader, h.cfg.sshProvider.hostCASigner); err != nil {
		return nil, trace.Wrap(err, "signing SSH host cert")
	}
	certSigner, err := ssh.NewCertSigner(cert, hostSigner)
	return certSigner, trace.Wrap(err)
}
