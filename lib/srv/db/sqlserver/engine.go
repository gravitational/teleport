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

package sqlserver

import (
	"bytes"
	"context"
	"io"
	"net"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types/events"
	libevents "github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/gravitational/teleport/lib/srv/db/common/kerberos"
	"github.com/gravitational/teleport/lib/srv/db/sqlserver/protocol"
	"github.com/gravitational/teleport/lib/utils"
)

// NewEngine create new SQL Server engine.
func NewEngine(ec common.EngineConfig) common.Engine {
	return &Engine{
		EngineConfig: ec,
		Connector: &connector{
			DBAuth: ec.Auth,

			kerberos: kerberos.NewClientProvider(ec.AuthClient, ec.DataDir),
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
			e.Log.WarnContext(e.Context, "Failed to send error to client.", "engine_error", err, "send_error", errSend)
		}
	}
}

// HandleConnection authorizes the incoming client connection, connects to the
// target SQL Server server and starts proxying messages between client/server.
func (e *Engine) HandleConnection(ctx context.Context, sessionCtx *common.Session) error {
	observe := common.GetConnectionSetupTimeObserver(sessionCtx.Database)

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

	observe()

	clientErrCh := make(chan error, 1)
	serverErrCh := make(chan error, 1)
	go e.receiveFromClient(e.clientConn, serverConn, clientErrCh, sessionCtx)
	go e.receiveFromServer(serverConn, e.clientConn, serverErrCh)

	select {
	case err := <-clientErrCh:
		e.Log.DebugContext(e.Context, "Client done.", "error", err)
	case err := <-serverErrCh:
		e.Log.DebugContext(e.Context, "Server done.", "error", err)
	case <-ctx.Done():
		e.Log.DebugContext(e.Context, "Context canceled.")
	}

	return nil
}

// receiveFromClient relays protocol messages received from  SQL Server client
// to SQL Server database.
func (e *Engine) receiveFromClient(clientConn, serverConn io.ReadWriteCloser, clientErrCh chan<- error, sessionCtx *common.Session) {
	defer func() {
		if r := recover(); r != nil {
			e.Log.ErrorContext(e.Context, "Recovered while handling DB connection", "recover", r)
			err := trace.BadParameter("failed to handle client connection")
			e.SendError(err)
		}
		serverConn.Close()
		e.Log.DebugContext(e.Context, "Stop receiving from client.")
		close(clientErrCh)
	}()

	msgFromClient := common.GetMessagesFromClientMetric(sessionCtx.Database)
	// initialPacketHeader and chunkData are used to accumulate chunked packets
	// to build a single packet with full contents for auditing.
	var initialPacketHeader protocol.PacketHeader
	var chunkData bytes.Buffer

	for {
		p, err := protocol.ReadPacket(clientConn)
		if err != nil {
			if utils.IsOKNetworkError(err) {
				e.Log.DebugContext(e.Context, "Client connection closed.")
				return
			}
			e.Log.ErrorContext(e.Context, "Failed to read client packet.", "error", err)
			clientErrCh <- err
			return
		}
		msgFromClient.Inc()

		// Audit events are going to be emitted only on final messages, this way
		// the packet parsing can be complete and provide the query/RPC
		// contents.
		if protocol.IsFinalPacket(p) {
			sqlPacket, err := e.toSQLPacket(initialPacketHeader, p, &chunkData)
			switch {
			case err != nil:
				e.Log.ErrorContext(e.Context, "Failed to parse SQLServer packet.", "error", err)
				e.emitMalformedPacket(e.Context, sessionCtx, p)
			default:
				e.auditPacket(e.Context, sessionCtx, sqlPacket)
			}
		} else {
			if chunkData.Len() == 0 {
				initialPacketHeader = p.Header()
			}

			chunkData.Write(p.Data())
		}

		_, err = serverConn.Write(p.Bytes())
		if err != nil {
			e.Log.ErrorContext(e.Context, "Failed to write server packet.", "error", err)
			clientErrCh <- err
			return
		}
	}
}

// toSQLPacket Parses a regular (self-contained) or chunked packet into an SQL
// packet (used for auditing).
func (e *Engine) toSQLPacket(header protocol.PacketHeader, packet *protocol.BasicPacket, chunks *bytes.Buffer) (protocol.Packet, error) {
	if chunks.Len() > 0 {
		defer chunks.Reset()
		chunks.Write(packet.Data())
		// We're safe to "read" chunk using `Bytes()` function because the
		// packet processing copies the packet contents.
		packetData := chunks.Bytes()

		var err error
		// The final chucked packet header must be the first packet header.
		packet, err = protocol.NewBasicPacket(header, packetData)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	return protocol.ToSQLPacket(packet)
}

// receiveFromServer relays protocol messages received from MySQL database
// to MySQL client.
func (e *Engine) receiveFromServer(serverConn, clientConn io.ReadWriteCloser, serverErrCh chan<- error) {
	// Note: we don't increment common.GetMessagesFromServerMetric here because messages are not parsed, so we cannot count them.
	// The total bytes written is still available as a metric.
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
	authPref, err := e.Auth.GetAuthPreference(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	state := sessionCtx.GetAccessState(authPref)
	err = sessionCtx.Checker.CheckAccess(sessionCtx.Database, state,
		services.NewDatabaseUserMatcher(sessionCtx.Database, sessionCtx.DatabaseUser),
	)
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
