/*
Copyright 2020 Gravitational, Inc.

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
	"errors"
	"io"
	"net"

	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/srv/db/mysql/protocol"

	"github.com/siddontang/go-mysql/mysql"
	"github.com/siddontang/go-mysql/server"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

// Proxy proxies connections from MySQL clients to database services
// over reverse tunnel. It runs inside Teleport proxy service.
//
// Implements db.DatabaseProxy.
type Proxy struct {
	// TLSConfig is the proxy TLS configuration.
	TLSConfig *tls.Config
	// Middleware is the auth middleware.
	Middleware *auth.Middleware
	// ConnectToSite is used to connect to remote database server over reverse tunnel.
	ConnectToSite func(context.Context, string, string) (net.Conn, error)
	// ProxyToSite starts proxying between client and site connections.
	ProxyToSite func(ctx context.Context, clientConn, siteConn io.ReadWriteCloser) error
	// Log is used for logging.
	Log logrus.FieldLogger
}

// HandleConnection accepts connection from a Postgres client, authenticates
// it and proxies it to an appropriate database service.
func (p *Proxy) HandleConnection(ctx context.Context, clientConn net.Conn) (err error) {
	server := p.makeServer(clientConn)
	// Perform first part of the handshake, up to the point where client sends
	// us certificate and connection upgrades to TLS.
	tlsConn, err := p.performHandshake(server)
	if err != nil {
		return trace.Wrap(err)
	}
	ctx, err = p.Middleware.WrapContext(ctx, tlsConn)
	if err != nil {
		return trace.Wrap(err)
	}
	siteConn, err := p.ConnectToSite(ctx, server.GetUser(), server.GetDatabase())
	if err != nil {
		return trace.Wrap(err)
	}
	defer siteConn.Close()
	// Before replying OK to the client which would make the client consider
	// auth completed, wait for OK packet from db service indicating auth
	// success.
	err = p.waitForOK(server, siteConn)
	if err != nil {
		return trace.Wrap(err)
	}
	// Auth has completed, the client enters command phase, start proxying
	// all messages back-and-forth.
	err = p.ProxyToSite(ctx, tlsConn, siteConn)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

type credentialProvider struct{}

func (p *credentialProvider) CheckUsername(_ string) (bool, error)         { return true, nil }
func (p *credentialProvider) GetCredential(_ string) (string, bool, error) { return "", true, nil }

// makeServer creates a MySQL server from the accepted client connection that
// provides access to various parts of the handshake.
func (p *Proxy) makeServer(clientConn net.Conn) *server.Conn {
	// TODO(r0mant): Add CLIENT_NO_SCHEMA and CLIENT_DEPRECATE_EOF server capabilities.
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
func (p *Proxy) performHandshake(server *server.Conn) (*tls.Conn, error) {
	err := server.WriteInitialHandshake()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	err = server.ReadHandshakeResponse()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// First part of the handshake completed and the connection has been
	// upgraded to TLS so now we can look at the client certificate and
	// see which database service to route the connection to.
	tlsConn, ok := server.Conn.Conn.(*tls.Conn)
	if !ok {
		return nil, trace.BadParameter("expected tls connection")
	}
	return tlsConn, nil
}

// waitForOK waits for OK_PACKET from the database service (siteConn)
// which indicates that auth on the other side completed succesfully.
func (p *Proxy) waitForOK(server *server.Conn, siteConn net.Conn) error {
	packet, err := protocol.ReadPacket(siteConn)
	if err != nil {
		return trace.Wrap(err)
	}
	p.Log.Debugf("Received packet: %s", packet)
	switch packet[4] {
	case mysql.OK_HEADER:
		err = server.WriteOK(nil)
		if err != nil {
			return trace.Wrap(err)
		}
	case mysql.ERR_HEADER:
		err = server.WriteError(errors.New(string(packet[7:])))
		if err != nil {
			return trace.Wrap(err)
		}
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
