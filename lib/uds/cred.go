/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package uds

import (
	"context"
	"net"

	"github.com/gravitational/trace"
	"google.golang.org/grpc/credentials"
)

type transportCredentials struct {
	wrapped credentials.TransportCredentials
}

// AuthInfo is a wrapper around the credentials.AuthInfo interface that also
// includes information about the peer connected to the UDS.
type AuthInfo struct {
	// Wrapped contains the wrapped credentials.AuthInfo.
	Wrapped credentials.AuthInfo
	// Creds contains information about the peer connected to the UDS.
	// It is nil if the peer is not connected via UDS.
	Creds *Creds
}

// AuthType implements the credentials.AuthInfo interface.
// The call is delegated to the wrapped credentials.AuthInfo.
func (a AuthInfo) AuthType() string {
	return a.Wrapped.AuthType()
}

// NewTransportCredentials returns a new credentials.TransportCredentials that
// wraps the provided credentials.TransportCredentials. This wrapper adds
// information about the peer connected to the AuthInfo via UDS if available.
func NewTransportCredentials(
	wrapped credentials.TransportCredentials,
) credentials.TransportCredentials {
	return &transportCredentials{
		wrapped: wrapped,
	}
}

// ClientHandshake implements the credentials.TransportCredentials interface.
// The call is delegated to the wrapped credentials.TransportCredentials.
func (c *transportCredentials) ClientHandshake(
	ctx context.Context, authority string, conn net.Conn,
) (net.Conn, credentials.AuthInfo, error) {
	return c.wrapped.ClientHandshake(ctx, authority, conn)
}

// ServerHandshake implements the credentials.TransportCredentials interface.
// The call is first delegated to the wrapped credentials.TransportCredentials,
// if this succeeds, the credentials of the peer connected via UDS are extracted
// and returned in a wrapped AuthInfo.
func (c *transportCredentials) ServerHandshake(
	conn net.Conn,
) (net.Conn, credentials.AuthInfo, error) {
	wrappedConn, wrappedAuthInfo, err := c.wrapped.ServerHandshake(conn)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	var creds *Creds
	if udsConn, ok := conn.(*net.UnixConn); ok {
		creds, err = getCreds(udsConn)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}
	}

	return wrappedConn, AuthInfo{
		Wrapped: wrappedAuthInfo,
		Creds:   creds,
	}, nil
}

// Info implements the credentials.TransportCredentials interface. This call
// is delegated to the wrapped credentials.TransportCredentials.
func (c *transportCredentials) Info() credentials.ProtocolInfo {
	return c.wrapped.Info()
}

// Clone implements the credentials.TransportCredentials interface. It returns
// a copy of the TransportCredentials.
func (c *transportCredentials) Clone() credentials.TransportCredentials {
	return &transportCredentials{
		wrapped: c.wrapped.Clone(),
	}
}

// OverrideServerName implements the credentials.TransportCredentials interface.
// This call is delegated to the wrapped credentials.TransportCredentials.
func (c *transportCredentials) OverrideServerName(sn string) error {
	//nolint:staticcheck // SA1019. Kept for backward compatibility.
	return c.wrapped.OverrideServerName(sn)
}

// Creds contains information about the peer connected to the UDS.
type Creds struct {
	// PID is the process ID of the peer.
	PID int
	// UID is the ID of the user that the peer process is running as.
	UID int
	// GID is the ID of the primary group that the peer process is running as.
	GID int
}

// GetCreds returns information about the peer connected to the UDS. It must
// be passed a net.Conn which encapsulates a *net.UnixConn.
func GetCreds(conn net.Conn) (*Creds, error) {
	udsConn, ok := conn.(*net.UnixConn)
	if !ok {
		return nil, trace.BadParameter("requires *UnixConn, got %T", conn)
	}

	return getCreds(udsConn)
}
