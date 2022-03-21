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

package sqlserver

import (
	"context"
	"io"
	"net"

	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/gravitational/teleport/lib/srv/db/sqlserver/protocol"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
)

func init() {
	common.RegisterEngine(newEngine, defaults.ProtocolSQLServer)
}

func newEngine(ec common.EngineConfig) common.Engine {
	return &Engine{
		EngineConfig: ec,
		Connector: &connector{
			Auth: ec.Auth,
		},
	}
}

// Engine handles connections from SQL Server clients coming from Teleport
// proxy over reverse tunnel.
type Engine struct {
	// EngineConfig is the common database engine configuration.
	common.EngineConfig
	// Connector allows to override SQL Server connection logic. Used in tests.
	Connector Connector
	// clientConn is the SQL Server client connection.
	clientConn net.Conn
}

// InitializeConnection initializes the client connection.
func (e *Engine) InitializeConnection(clientConn net.Conn, _ *common.Session) error {
	e.clientConn = clientConn
	return nil
}

// SendError sends an error to SQL Server client.
func (e *Engine) SendError(err error) {
	if err != nil && !utils.IsOKNetworkError(err) {
		if errSend := protocol.WriteErrorResponse(e.clientConn, err); errSend != nil {
			e.Log.WithError(errSend).Warnf("Failed to send error to client: %v.", err)
		}
	}
}

// HandleConnection authorizes the incoming client connection, connects to the
// target SQL Server server and starts proxying messages between client/server.
func (e *Engine) HandleConnection(ctx context.Context, sessionCtx *common.Session) error {
	// Pre-Login packet was handled on the Proxy. Now we expect the client to
	// send us a Login7 packet that contains username/database information and
	// other connection options.
	packet, err := e.handleLogin7(sessionCtx)
	if err != nil {
		return trace.Wrap(err)
	}

	// Run authorization check.
	err = e.checkAccess(ctx, sessionCtx)
	if err != nil {
		return trace.Wrap(err)
	}

	// Connect to the target SQL Server instance.
	serverConn, serverFlags, err := e.Connector.Connect(ctx, sessionCtx, packet)
	if err != nil {
		return trace.Wrap(err)
	}
	defer serverConn.Close()

	// Pass all flags returned by server during login back to the client.
	err = protocol.WriteStreamResponse(e.clientConn, serverFlags)
	if err != nil {
		return trace.Wrap(err)
	}

	// Start proxying packets between client and server.
	err = e.proxy(ctx, serverConn)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// handleLogin7 processes Login7 packet received from the client.
//
// Login7 packet contains database user, database name and various login
// options that we pass to the target SQL Server.
func (e *Engine) handleLogin7(sessionCtx *common.Session) (*protocol.Login7Packet, error) {
	pkt, err := protocol.ReadLogin7Packet(e.clientConn)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	sessionCtx.DatabaseUser = pkt.Username()
	if pkt.Database() != "" {
		sessionCtx.DatabaseName = pkt.Database()
	}

	return pkt, nil
}

func (e *Engine) checkAccess(ctx context.Context, sessionCtx *common.Session) error {
	ap, err := e.Auth.GetAuthPreference(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	mfaParams := services.AccessMFAParams{
		Verified:       sessionCtx.Identity.MFAVerified != "",
		AlwaysRequired: ap.GetRequireSessionMFA(),
	}

	err = sessionCtx.Checker.CheckAccess(sessionCtx.Database, mfaParams,
		&services.DatabaseUserMatcher{
			User: sessionCtx.DatabaseUser,
		})
	if err != nil {
		e.Audit.OnSessionStart(e.Context, sessionCtx, err)
		return trace.Wrap(err)
	}

	return nil
}

// proxy proxies all traffic between the client and server connections.
func (e *Engine) proxy(ctx context.Context, serverConn io.ReadWriteCloser) error {
	errCh := make(chan error, 2)

	go func() {
		defer serverConn.Close()
		_, err := io.Copy(serverConn, e.clientConn)
		errCh <- err
	}()

	go func() {
		defer serverConn.Close()
		_, err := io.Copy(e.clientConn, serverConn)
		errCh <- err
	}()

	var errs []error
	for i := 0; i < 2; i++ {
		select {
		case err := <-errCh:
			if err != nil && !utils.IsOKNetworkError(err) {
				errs = append(errs, err)
			}
		case <-ctx.Done():
			return trace.Wrap(ctx.Err())
		}
	}

	return trace.NewAggregate(errs...)
}
