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

package postgres

import (
	"context"
	"crypto/tls"
	"io"
	"net"
	"strings"

	"github.com/gravitational/teleport/lib/auth"

	"github.com/jackc/pgproto3/v2"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

// Proxy proxies connections from Postgres clients to database services
// over reverse tunnel. It runs inside Teleport proxy service.
//
// Implements db.DatabaseProxy.
type Proxy struct {
	// TLSConfig is the proxy TLS configuration.
	TLSConfig *tls.Config
	// Middleware is the auth middleware.
	Middleware *auth.Middleware
	// ConnectToSite is used to connect to remote database server over reverse tunnel.
	ConnectToSite func(context.Context) (net.Conn, error)
	// Log is used for logging.
	Log logrus.FieldLogger
}

// HandleConnection accepts connection from a Postgres client, authenticates
// it and proxies it to an appropriate database service.
func (p *Proxy) HandleConnection(ctx context.Context, clientConn net.Conn) (err error) {
	startupMessage, tlsConn, backend, err := p.handleStartup(ctx, clientConn)
	if err != nil {
		return trace.Wrap(err)
	}
	defer func() {
		if err != nil {
			if err := backend.Send(toErrorResponse(err)); err != nil {
				p.Log.WithError(err).Warn("Failed to send error to backend.")
			}
		}
	}()
	ctx, err = p.Middleware.WrapContext(ctx, tlsConn)
	if err != nil {
		return trace.Wrap(err)
	}
	siteConn, err := p.ConnectToSite(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	defer siteConn.Close()
	err = p.proxyToSite(ctx, tlsConn, siteConn, startupMessage)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// handleStartup handles the initial protocol exchange between the Postgres
// client (e.g. psql) and this proxy.
//
// Returns the startup message that contains initial connect parameters and
// the upgraded TLS connection.
func (p *Proxy) handleStartup(ctx context.Context, clientConn net.Conn) (*pgproto3.StartupMessage, *tls.Conn, *pgproto3.Backend, error) {
	// Backend acts as a server for the Postgres wire protocol.
	backend := pgproto3.NewBackend(pgproto3.NewChunkReader(clientConn), clientConn)
	startupMessage, err := backend.ReceiveStartupMessage()
	if err != nil {
		return nil, nil, nil, trace.Wrap(err)
	}
	p.Log.Debugf("Received startup message: %#v.", startupMessage)
	// When initiating an encrypted connection, psql will first check with
	// the server whether it supports TLS by sending an SSLRequest message.
	//
	// Once the server has indicated the support (by sending 'S' in reply),
	// it will send a StartupMessage with the connection parameters such as
	// user name, database name, etc.
	//
	// https://www.postgresql.org/docs/13/protocol-flow.html#id-1.10.5.7.11
	switch m := startupMessage.(type) {
	case *pgproto3.SSLRequest:
		// Send 'S' back to indicate TLS support to the client.
		_, err := clientConn.Write([]byte("S"))
		if err != nil {
			return nil, nil, nil, trace.Wrap(err)
		}
		// Upgrade the connection to TLS and wait for the next message
		// which should be of the StartupMessage type.
		clientConn = tls.Server(clientConn, p.TLSConfig)
		return p.handleStartup(ctx, clientConn)
	case *pgproto3.StartupMessage:
		// TLS connection between the client and this proxy has been
		// established, just return the startup message.
		tlsConn, ok := clientConn.(*tls.Conn)
		if !ok {
			return nil, nil, nil, trace.BadParameter(
				"expected tls connection, got %T", clientConn)
		}
		return m, tlsConn, backend, nil
	}
	return nil, nil, nil, trace.BadParameter(
		"unsupported startup message: %#v", startupMessage)
}

// proxyToSite starts proxying all traffic received from Postgres client
// between this proxy and Teleport database service over reverse tunnel.
func (p *Proxy) proxyToSite(ctx context.Context, clientConn, siteConn net.Conn, startupMessage *pgproto3.StartupMessage) (retErr error) {
	// Frontend acts as a client for the Postgres wire protocol.
	frontend := pgproto3.NewFrontend(pgproto3.NewChunkReader(siteConn), siteConn)
	// Pass the startup message along to the Teleport database server.
	err := frontend.Send(startupMessage)
	if err != nil {
		return trace.Wrap(err)
	}
	errCh := make(chan error, 2)
	go func() {
		defer p.Log.Debug("Stop proxying from client to site.")
		defer siteConn.Close()
		defer clientConn.Close()
		_, err := io.Copy(siteConn, clientConn)
		errCh <- err
	}()
	go func() {
		defer p.Log.Debug("Stop proxying from site to client.")
		defer siteConn.Close()
		defer clientConn.Close()
		_, err := io.Copy(clientConn, siteConn)
		errCh <- err
	}()
	var errs []error
	for i := 0; i < 2; i++ {
		select {
		case err := <-errCh:
			if err != nil && err != io.EOF && !strings.Contains(err.Error(), "use of closed network connection") {
				p.Log.WithError(err).Warn("Connection problem.")
				errs = append(errs, err)
			}
		case <-ctx.Done():
			return trace.ConnectionProblem(nil, "context is closing")
		}
	}
	return trace.NewAggregate(errs...)
}
