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

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/defaults"
	libevents "github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/gravitational/teleport/lib/srv/db/sqlserver/protocol"
	"github.com/gravitational/teleport/lib/utils"
)

func init() {
	common.RegisterEngine(newEngine, defaults.ProtocolSQLServer)
}

func newEngine(ec common.EngineConfig) common.Engine {
	return &Engine{
		EngineConfig: ec,
		Connector: &connector{
			DBAuth:     ec.Auth,
			AuthClient: ec.AuthClient,
			DataDir:    ec.DataDir,
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

	e.Audit.OnSessionStart(e.Context, sessionCtx, nil)
	defer e.Audit.OnSessionEnd(e.Context, sessionCtx)

	clientErrCh := make(chan error, 1)
	serverErrCh := make(chan error, 1)
	go e.receiveFromClient(e.clientConn, serverConn, clientErrCh, sessionCtx)
	go e.receiveFromServer(serverConn, e.clientConn, serverErrCh)

	select {
	case err := <-clientErrCh:
		e.Log.WithError(err).Debug("Client done.")
	case err := <-serverErrCh:
		e.Log.WithError(err).Debug("Server done.")
	case <-ctx.Done():
		e.Log.Debug("Context canceled.")
	}

	return nil
}

// receiveFromClient relays protocol messages received from  SQL Server client
// to SQL Server database.
func (e *Engine) receiveFromClient(clientConn, serverConn io.ReadWriteCloser, clientErrCh chan<- error, sessionCtx *common.Session) {
	defer func() {
		if r := recover(); r != nil {
			e.Log.Warnf("Recovered while handling DB connection %v", r)
			err := trace.BadParameter("failed to handle client connection")
			e.SendError(err)
		}
		serverConn.Close()
		e.Log.Debug("Stop receiving from client.")
		close(clientErrCh)
	}()
	for {
		p, err := protocol.ReadPacket(clientConn)
		if err != nil {
			if utils.IsOKNetworkError(err) {
				e.Log.Debug("Client connection closed.")
				return
			}
			e.Log.WithError(err).Error("Failed to read client packet.")
			clientErrCh <- err
			return
		}

		sqlPacket, err := protocol.ToSQLPacket(p)
		switch {
		case err != nil:
			e.Log.WithError(err).Errorf("Failed to parse SQLServer packet.")
			e.emitMalformedPacket(e.Context, sessionCtx, p)
		default:
			e.auditPacket(e.Context, sessionCtx, sqlPacket)
		}

		_, err = serverConn.Write(p.Bytes())
		if err != nil {
			e.Log.WithError(err).Error("Failed to write server packet.")
			clientErrCh <- err
			return
		}
	}
}

// receiveFromServer relays protocol messages received from MySQL database
// to MySQL client.
func (e *Engine) receiveFromServer(serverConn, clientConn io.ReadWriteCloser, serverErrCh chan<- error) {
	defer clientConn.Close()
	_, err := io.Copy(clientConn, serverConn)
	if err != nil && !utils.IsOKNetworkError(err) {
		serverErrCh <- trace.Wrap(err)
	}
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

	mfaParams := sessionCtx.MFAParams(ap.GetRequireMFAType())
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
func (e *Engine) emitMalformedPacket(ctx context.Context, sessCtx *common.Session, packet protocol.Packet) {
	e.Audit.EmitEvent(ctx, &events.DatabaseSessionMalformedPacket{
		Metadata: common.MakeEventMetadata(sessCtx,
			libevents.DatabaseSessionMalformedPacketEvent,
			libevents.DatabaseSessionMalformedPacketCode,
		),
		UserMetadata:     common.MakeUserMetadata(sessCtx),
		SessionMetadata:  common.MakeSessionMetadata(sessCtx),
		DatabaseMetadata: common.MakeDatabaseMetadata(sessCtx),
		Payload:          packet.Bytes(),
	})
}

func (e *Engine) auditPacket(ctx context.Context, sessCtx *common.Session, packet protocol.Packet) {
	switch t := packet.(type) {
	case *protocol.SQLBatch:
		e.Audit.OnQuery(ctx, sessCtx, common.Query{Query: t.SQLText})
	case *protocol.RPCRequest:
		e.Audit.EmitEvent(ctx, &events.SQLServerRPCRequest{
			Metadata: common.MakeEventMetadata(sessCtx,
				libevents.DatabaseSessionSQLServerRPCRequestEvent,
				libevents.SQLServerRPCRequestCode,
			),
			UserMetadata:     common.MakeUserMetadata(sessCtx),
			SessionMetadata:  common.MakeSessionMetadata(sessCtx),
			DatabaseMetadata: common.MakeDatabaseMetadata(sessCtx),
			Procname:         t.ProcName,
			Parameters:       t.Parameters,
		})
	}
}
