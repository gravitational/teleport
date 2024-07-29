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

package peer

import (
	"context"
	"net"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/credentials"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/tlsca"
)

// serverCredentials wraps a [crendentials.TransportCredentials] that
// extends the ServerHandshake to ensure the credentials contain the proxy system role.
type serverCredentials struct {
	credentials.TransportCredentials
}

// newServerCredentials creates new serverCredentials from the given [crendentials.TransportCredentials].
func newServerCredentials(creds credentials.TransportCredentials) *serverCredentials {
	return &serverCredentials{
		TransportCredentials: creds,
	}
}

// ServerHandshake performs the TLS handshake and then verifies that the client
// attempting to connect is a Proxy.
func (c *serverCredentials) ServerHandshake(conn net.Conn) (_ net.Conn, _ credentials.AuthInfo, err error) {
	conn, authInfo, err := c.TransportCredentials.ServerHandshake(conn)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	defer func() {
		if err != nil {
			conn.Close()
		}
	}()

	identity, err := getIdentity(authInfo)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	if err := checkProxyRole(identity); err != nil {
		return nil, nil, trace.Wrap(err)
	}

	return conn, authInfo, nil
}

// clientCredentials wraps a [crendentials.TransportCredentials] that
// extends the ClientHandshake to ensure the credentials contain the proxy system role
// and that connections are established to the expected peer.
type clientCredentials struct {
	credentials.TransportCredentials
	peerID   string
	peerAddr string
	logger   logrus.FieldLogger
}

// newClientCredentials creates new clientCredentials from the given [crendentials.TransportCredentials].
func newClientCredentials(peerID, peerAddr string, logger logrus.FieldLogger, creds credentials.TransportCredentials) *clientCredentials {
	return &clientCredentials{
		TransportCredentials: creds,
		peerID:               peerID,
		peerAddr:             peerAddr,
		logger:               logger,
	}
}

// ClientHandshake performs the TLS handshake and then verifies that the
// server is a Proxy and that it's UUID matches the expected id of the peer.
func (c *clientCredentials) ClientHandshake(ctx context.Context, laddr string, conn net.Conn) (_ net.Conn, _ credentials.AuthInfo, err error) {
	conn, authInfo, err := c.TransportCredentials.ClientHandshake(ctx, laddr, conn)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	defer func() {
		if err != nil {
			conn.Close()
		}
	}()

	identity, err := getIdentity(authInfo)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	if err := checkProxyRole(identity); err != nil {
		return nil, nil, trace.Wrap(err)
	}

	const duplicatePeerMsg = "Detected multiple Proxy Peers with the same public address %q when connecting to Proxy %q which can lead to inconsistent state and problems establishing sessions. For best results ensure that `peer_public_addr` is unique per proxy and not a load balancer."
	if err := validatePeer(c.peerID, identity); err != nil {
		c.logger.Errorf(duplicatePeerMsg, c.peerAddr, c.peerID)
		return nil, nil, trace.Wrap(err)
	}

	return conn, authInfo, nil
}

// getIdentity returns a [tlsca.Identity] that is created from the certificate
// presented during the TLS handshake.
func getIdentity(authInfo credentials.AuthInfo) (*tlsca.Identity, error) {
	tlsInfo, ok := authInfo.(credentials.TLSInfo)
	if !ok {
		return nil, trace.AccessDenied("credentials auth information is missing")
	}

	certs := tlsInfo.State.PeerCertificates
	if len(certs) == 0 {
		return nil, trace.AccessDenied("no peer certificates provided")
	}

	clientCert := certs[0]
	identity, err := tlsca.FromSubject(clientCert.Subject, clientCert.NotAfter)
	return identity, trace.Wrap(err)
}

// checkProxyRole ensures that the [tlsca.identity] is for a [types.RoleProxy].
func checkProxyRole(identity *tlsca.Identity) error {
	for _, role := range identity.Groups {
		if types.SystemRole(role) == types.RoleProxy {
			return nil
		}
	}

	return trace.AccessDenied("proxy system role required")
}

// validatePeer ensures that provided peerID matches the id of
// the peer that was connected to. This prevents client connections
// from being established to an incorrect peer if multiple peers
// share the same address.
func validatePeer(peerID string, identity *tlsca.Identity) error {
	if identity.Username == peerID {
		return nil
	}

	return trace.AccessDenied("connected to unexpected proxy")
}
