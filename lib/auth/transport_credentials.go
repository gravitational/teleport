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

package auth

import (
	"context"
	"crypto/tls"
	"io"
	"net"

	"github.com/gravitational/trace"
	"google.golang.org/grpc/credentials"

	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/authz"
)

// UserGetter is responsible for building an authenticated user based on TLS metadata
type UserGetter interface {
	GetUser(connState tls.ConnectionState) (authz.IdentityGetter, error)
}

// ConnectionIdentity contains the identifying properties of a
// client connection required to enforce connection limits.
type ConnectionIdentity struct {
	// Username is the name of the user
	Username string
	// MaxConnections the upper limit to number of open connections for a user
	MaxConnections int64
	// LocalAddr is the local address for the connection
	LocalAddr string
	// RemoteAddr is the remote address for the connection
	RemoteAddr string
	// UserMetadata contains metadata for a user
	UserMetadata apievents.UserMetadata
}

// ConnectionEnforcer limits incoming connections based on
// max connection settings.
type ConnectionEnforcer interface {
	EnforceConnectionLimits(ctx context.Context, identity ConnectionIdentity, closers ...io.Closer) (context.Context, error)
}

// TransportCredentialsConfig configures the behavior that occurs
// during the server handshake by the TransportCredentials
type TransportCredentialsConfig struct {
	// TransportCredentials provide the credentials that are used to perform the TLS
	// server and client handshakes as well as the [credentials.ProtocolInfo]. This
	// **MUST** not be nil, and it must have its [credentials.ProtocolInfo.SecurityProtocol]
	// equal to "tls".
	TransportCredentials credentials.TransportCredentials
	// UserGetter constructs the clients' [tlsca.Identity] from the [tls.ConnectionState]
	// that is received from the TLS handshake. This
	UserGetter UserGetter
	// Authorizer prevents any connections from being established if the user is not
	// authorized due to locks, private key policy, device trust, etc. If not set
	// then no authorization is performed.
	Authorizer authz.Authorizer
	// Enforcer prevents any connections from being established if the user would
	// exceed their configured max connection limit. Any connections that are
	// permitted may be terminated if there is an issue determining if the number
	// of active connections is within the limit. If not set then no connection
	// limits are enforced.
	Enforcer ConnectionEnforcer
}

// Check validates that the configuration is valid for use and
// that all supplied parameters are set accordingly.
func (c *TransportCredentialsConfig) Check() error {
	switch {
	case c.TransportCredentials == nil:
		return trace.BadParameter("parameter TransportCredentials required")
	case c.TransportCredentials.Info().SecurityProtocol != "tls":
		return trace.BadParameter("the TransportCredentials must be a tls security protocol, got %s", c.TransportCredentials.Info().SecurityProtocol)
	case c.UserGetter == nil:
		return trace.BadParameter("parameter UserGetter required")
	case c.Authorizer != nil && c.UserGetter == nil:
		return trace.BadParameter("a UserGetter is required to use validate identities with an Authorizer")
	case c.Enforcer != nil && c.Authorizer == nil:
		return trace.BadParameter("both a UserGetter and an Authorizer are required to enforce connection limits with an Enforcer")
	default:
		return nil
	}
}

// TransportCredentials is a [credentials.TransportCredentials] that
// enforces mTLS and retrieves the [IdentityGetter] for use by middleware
// to perform authorization.
type TransportCredentials struct {
	credentials.TransportCredentials

	userGetter UserGetter
	authorizer authz.Authorizer
	enforcer   ConnectionEnforcer
}

// NewTransportCredentials returns a new TransportCredentials
func NewTransportCredentials(cfg TransportCredentialsConfig) (*TransportCredentials, error) {
	if err := cfg.Check(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &TransportCredentials{
		TransportCredentials: cfg.TransportCredentials,
		userGetter:           cfg.UserGetter,
		authorizer:           cfg.Authorizer,
		enforcer:             cfg.Enforcer,
	}, nil
}

// IdentityInfo contains the auth information and identity
// for an authenticated TLS connection. It implements the
// [credentials.AuthInfo] interface and is returned from
// [TransportCredentials.ServerHandshake].
type IdentityInfo struct {
	// TLSInfo contains TLS connection information.
	*credentials.TLSInfo
	// IdentityGetter provides a mechanism to retrieve the
	// identity of the client.
	IdentityGetter authz.IdentityGetter
	// AuthContext contains information about the traits and roles
	// that an identity may have. This will be unset if the
	// [TransportCredentialsConfig.Authorizer] provided to [NewTransportCredentials]
	// was nil.
	AuthContext *authz.Context
	// Conn is the underlying [net.Conn] of the gRPC connection.
	Conn net.Conn
}

// ServerHandshake does the authentication handshake for servers. It returns
// the authenticated connection and the corresponding auth information about
// the connection.
// At minimum the TLS handshake is performed and the identity is built from
// the [tls.ConnectionState]. If the TransportCredentials is configured with
// and Authorizer and ConnectionEnforcer then additional session controls are
// applied before the handshake completes.
func (c *TransportCredentials) ServerHandshake(rawConn net.Conn) (_ net.Conn, _ credentials.AuthInfo, err error) {
	conn, tlsInfo, err := c.performTLSHandshake(rawConn)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	defer func() {
		if err != nil {
			conn.Close()
		}
	}()

	identityGetter, err := c.userGetter.GetUser(tlsInfo.State)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	ctx := context.Background()
	authCtx, err := c.authorize(ctx, conn.RemoteAddr(), identityGetter, &tlsInfo.State)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	if err := c.enforceConnectionLimits(ctx, authCtx, conn); err != nil {
		return nil, nil, trace.Wrap(err)
	}

	return conn, IdentityInfo{
		TLSInfo:        tlsInfo,
		IdentityGetter: identityGetter,
		AuthContext:    authCtx,
		Conn:           conn,
	}, nil
}

// performTLSHandshake does the TLS handshake and validates the
// returned [credentials.AuthInfo] are of type [credentials.TLSInfo].
func (c *TransportCredentials) performTLSHandshake(rawConn net.Conn) (net.Conn, *credentials.TLSInfo, error) {
	conn, info, err := c.TransportCredentials.ServerHandshake(rawConn)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	tlsInfo, ok := info.(credentials.TLSInfo)
	if !ok {
		conn.Close()
		return nil, nil, trace.BadParameter("unexpected type in tls auth info %T", info)
	}

	return conn, &tlsInfo, nil
}

// authorize enforces that the identity is not restricted from connecting due
// to things like locks, private key policy, device trust, etc. If the TransportCredentials
// was not configured to do authorization then this is a noop and will return nil, nil.
func (c *TransportCredentials) authorize(ctx context.Context, remoteAddr net.Addr, identityGetter authz.IdentityGetter, connState *tls.ConnectionState) (*authz.Context, error) {
	if c.authorizer == nil {
		return &authz.Context{
			Identity: identityGetter,
		}, nil
	}

	// construct a context with the keys expected by the Authorizer
	ctx = authz.ContextWithUserCertificate(ctx, certFromConnState(connState))
	ctx = authz.ContextWithClientSrcAddr(ctx, remoteAddr)
	ctx = authz.ContextWithUser(ctx, identityGetter)

	authCtx, err := c.authorizer.Authorize(ctx)
	return authCtx, trace.Wrap(err)
}

// enforceConnectionLimits prevents the identity from exceeding any configured
// connection limits. The provided connection will be closed by the enforcer
// if connectivity to Auth is interrupted. If the TransportCredentials
// was not configured to do connection limiting then this is a noop and will return nil.
func (c *TransportCredentials) enforceConnectionLimits(ctx context.Context, authCtx *authz.Context, conn net.Conn) error {
	if c.enforcer == nil {
		return nil
	}

	if authCtx == nil || authCtx.Checker == nil {
		return trace.BadParameter("unable to determine connection limits without a valid auth context")
	}

	_, err := c.enforcer.EnforceConnectionLimits(
		ctx,
		ConnectionIdentity{
			Username:       authCtx.User.GetName(),
			MaxConnections: authCtx.Checker.MaxConnections(),
			LocalAddr:      conn.LocalAddr().String(),
			RemoteAddr:     conn.RemoteAddr().String(),
			UserMetadata:   authz.ClientUserMetadata(ctx),
		},
		conn,
	)

	return trace.Wrap(err)
}

// Clone makes a copy of this TransportCredentials.
func (c *TransportCredentials) Clone() credentials.TransportCredentials {
	return &TransportCredentials{
		userGetter:           c.userGetter,
		authorizer:           c.authorizer,
		enforcer:             c.enforcer,
		TransportCredentials: c.TransportCredentials.Clone(),
	}
}
