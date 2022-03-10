/*
Copyright 2021 Gravitational, Inc.

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

package mysql

import (
	"context"
	"crypto/tls"
	"net"
	"time"

	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/limiter"
	"github.com/gravitational/teleport/lib/multiplexer"
	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/gravitational/teleport/lib/srv/db/mysql/protocol"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/siddontang/go-mysql/mysql"
	"github.com/siddontang/go-mysql/server"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

// Proxy proxies connections from MySQL clients to database services
// over reverse tunnel. It runs inside Teleport proxy service.
//
// Implements common.Proxy.
type Proxy struct {
	// TLSConfig is the proxy TLS configuration.
	TLSConfig *tls.Config
	// Middleware is the auth middleware.
	Middleware *auth.Middleware
	// Service is used to connect to a remote database service.
	Service common.Service
	// Log is used for logging.
	Log logrus.FieldLogger
	// Limiter limits the number of active connections per client IP.
	Limiter *limiter.Limiter
}

// HandleConnection accepts connection from a MySQL client, authenticates
// it and proxies it to an appropriate database service.
func (p *Proxy) HandleConnection(ctx context.Context, clientConn net.Conn) (err error) {
	// Wrap the client connection in the connection that can detect the protocol
	// by peeking into the first few bytes. This is needed to be able to detect
	// proxy protocol which otherwise would interfere with MySQL protocol.
	conn := multiplexer.NewConn(clientConn)
	server := p.makeServer(conn)
	// If any error happens, make sure to send it back to the client, so it
	// has a chance to close the connection from its side.
	defer func() {
		if r := recover(); r != nil {
			p.Log.Warnf("Recovered in MySQL proxy while handling connection from %v: %v.", clientConn.RemoteAddr(), r)
			err = trace.BadParameter("failed to handle MySQL client connection")
		}
		if err != nil {
			if writeErr := server.WriteError(err); writeErr != nil {
				p.Log.WithError(writeErr).Debugf("Failed to send error %q to MySQL client.", err)
			}
		}
	}()
	// Perform first part of the handshake, up to the point where client sends
	// us certificate and connection upgrades to TLS.
	tlsConn, err := p.performHandshake(conn, server)
	if err != nil {
		return trace.Wrap(err)
	}

	clientIP, err := utils.ClientIPFromConn(clientConn)
	if err != nil {
		return trace.Wrap(err)
	}
	// Apply connection and rate limiting.
	releaseConn, err := p.Limiter.RegisterRequestAndConnection(clientIP)
	if err != nil {
		return trace.Wrap(err)
	}
	defer releaseConn()

	proxyCtx, err := p.Service.Authorize(ctx, tlsConn, common.ConnectParams{
		User:     server.GetUser(),
		Database: server.GetDatabase(),
		ClientIP: clientIP,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	serviceConn, err := p.Service.Connect(ctx, proxyCtx)
	if err != nil {
		return trace.Wrap(err)
	}
	defer serviceConn.Close()
	// Before replying OK to the client which would make the client consider
	// auth completed, wait for OK packet from db service indicating auth
	// success.
	err = p.waitForOK(server, serviceConn)
	if err != nil {
		return trace.Wrap(err)
	}
	// Auth has completed, the client enters command phase, start proxying
	// all messages back-and-forth.
	err = p.Service.Proxy(ctx, proxyCtx, tlsConn, serviceConn)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// credentialProvider is used by MySQL server created below.
//
// It's a no-op because authentication is done via mTLS.
type credentialProvider struct{}

func (p *credentialProvider) CheckUsername(_ string) (bool, error)         { return true, nil }
func (p *credentialProvider) GetCredential(_ string) (string, bool, error) { return "", true, nil }

// makeServer creates a MySQL server from the accepted client connection that
// provides access to various parts of the handshake.
func (p *Proxy) makeServer(clientConn net.Conn) *server.Conn {
	return server.MakeConn(
		clientConn,
		server.NewServer(
			serverVersion,
			mysql.DEFAULT_COLLATION_ID,
			mysql.AUTH_NATIVE_PASSWORD,
			nil,
			p.TLSConfig),
		&credentialProvider{},
		server.EmptyHandler{})
}

// performHandshake performs the initial handshake between MySQL client and
// this server, up to the point where the client sends us a certificate for
// authentication, and returns the upgraded connection.
func (p *Proxy) performHandshake(conn *multiplexer.Conn, server *server.Conn) (*tls.Conn, error) {
	// MySQL protocol is server-initiated which means the client will expect
	// server to send initial handshake message.
	err := server.WriteInitialHandshake()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// See if we need to read the proxy-line which could happen if Teleport
	// is running behind a load balancer with proxy protocol enabled.
	err = p.maybeReadProxyLine(conn)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// Then proceed normally to MySQL handshake.
	err = server.ReadHandshakeResponse()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// First part of the handshake completed and the connection has been
	// upgraded to TLS so now we can look at the client certificate and
	// see which database service to route the connection to.
	tlsConn, ok := server.Conn.Conn.(*tls.Conn)
	if !ok {
		return nil, trace.BadParameter("expected TLS connection")
	}
	return tlsConn, nil
}

// maybeReadProxyLine peeks into the connection to see if instead of regular
// MySQL protocol we were sent a proxy-line. This usually happens when Teleport
// is running behind a load balancer with proxy protocol enabled.
func (p *Proxy) maybeReadProxyLine(conn *multiplexer.Conn) error {
	proto, err := conn.Detect()
	if err != nil {
		return trace.Wrap(err)
	}
	if proto != multiplexer.ProtoProxy {
		return nil
	}
	proxyLine, err := conn.ReadProxyLine()
	if err != nil {
		return trace.Wrap(err)
	}
	p.Log.Debugf("MySQL listener proxy-line: %s.", proxyLine)
	return nil
}

// waitForOK waits for OK_PACKET from the database service which indicates
// that auth on the other side completed successfully.
func (p *Proxy) waitForOK(server *server.Conn, serviceConn net.Conn) error {
	err := serviceConn.SetReadDeadline(time.Now().Add(2 * defaults.DatabaseConnectTimeout))
	if err != nil {
		return trace.Wrap(err)
	}
	packet, err := protocol.ParsePacket(serviceConn)
	if err != nil {
		return trace.Wrap(err)
	}
	err = serviceConn.SetReadDeadline(time.Time{})
	if err != nil {
		return trace.Wrap(err)
	}
	switch p := packet.(type) {
	case *protocol.OK:
		err = server.WriteOK(nil)
		if err != nil {
			return trace.Wrap(err)
		}
	case *protocol.Error:
		err = server.WriteError(p)
		if err != nil {
			return trace.Wrap(err)
		}
	default:
		return trace.BadParameter("expected OK or ERR packet, got %s", packet)
	}
	return nil
}

const (
	// serverVersion is advertised to MySQL clients during handshake.
	//
	// Some clients may refuse to work with older servers (e.g. MySQL
	// Workbench requires > 5.5).
	serverVersion = "8.0.0-Teleport"
)
