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

package postgres

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"

	"github.com/gravitational/trace"
	"github.com/jackc/pgconn"
	"github.com/jackc/pgproto3/v2"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/db/cloud"
	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/gravitational/teleport/lib/srv/db/common/role"
	"github.com/gravitational/teleport/lib/utils"
)

// NewEngine create new Postgres engine.
func NewEngine(ec common.EngineConfig) common.Engine {
	return &Engine{
		EngineConfig: ec,
	}
}

// Engine implements the Postgres database service that accepts client
// connections coming over reverse tunnel from the proxy and proxies
// them between the proxy and the Postgres database instance.
//
// Implements common.Engine.
type Engine struct {
	// EngineConfig is the common database engine configuration.
	common.EngineConfig
	// client is a client connection.
	client *pgproto3.Backend
	// cancelReq is a cancel request saved when a cancel request is received
	// instead of a startup message.
	cancelReq *pgproto3.CancelRequest

	// rawClientConn is raw, unwrapped network connection to the client
	rawClientConn net.Conn
	// rawServerConn is raw, unwrapped network connection to the server
	rawServerConn net.Conn
}

// InitializeConnection initializes the client connection.
func (e *Engine) InitializeConnection(clientConn net.Conn, sessionCtx *common.Session) error {
	e.rawClientConn = clientConn
	e.client = pgproto3.NewBackend(pgproto3.NewChunkReader(clientConn), clientConn)

	// The proxy is supposed to pass a startup message it received from
	// the psql client over to us, so wait for it and extract database
	// and username from it.
	err := e.handleStartup(e.client, sessionCtx)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// SendError sends an error to connected client in a Postgres understandable format.
func (e *Engine) SendError(err error) {
	if err := e.client.Send(toErrorResponse(err)); err != nil && !utils.IsOKNetworkError(err) {
		e.Log.WithError(err).Error("Failed to send error to client.")
	}
}

// toErrorResponse converts the provided error to a Postgres wire protocol
// error message response so the client such as psql can display it
// appropriately.
func toErrorResponse(err error) *pgproto3.ErrorResponse {
	var pgErr *pgconn.PgError
	if !errors.As(trace.Unwrap(err), &pgErr) {
		return &pgproto3.ErrorResponse{
			Message: err.Error(),
		}
	}
	return &pgproto3.ErrorResponse{
		Severity: pgErr.Severity,
		Code:     pgErr.Code,
		Message:  pgErr.Message,
		Detail:   pgErr.Detail,
	}
}

// HandleConnection processes the connection from Postgres proxy coming
// over reverse tunnel.
//
// It handles all necessary startup actions, authorization and acts as a
// middleman between the proxy and the database intercepting and interpreting
// all messages i.e. doing protocol parsing.
func (e *Engine) HandleConnection(ctx context.Context, sessionCtx *common.Session) error {
	observe := common.GetConnectionSetupTimeObserver(sessionCtx.Database)
	// Now we know which database/username the user is connecting to, so
	// perform an authorization check.
	err := e.checkAccess(ctx, sessionCtx)
	if err != nil {
		return trace.Wrap(err)
	}
	if e.cancelReq != nil {
		// Special case when sending a cancel request.
		// Postgres cancel request message flow is unique:
		// 1. No startup message is sent by the client.
		// 2. The server closes the connection without responding to the client.
		return trace.Wrap(e.handleCancelRequest(ctx, sessionCtx))
	}
	// Automatically create the database user if needed.
	cancelAutoUserLease, err := e.GetUserProvisioner(e).Activate(ctx, sessionCtx)
	if err != nil {
		return trace.Wrap(err)
	}
	defer func() {
		err := e.GetUserProvisioner(e).Teardown(ctx, sessionCtx)
		if err != nil {
			e.Log.WithError(err).Error("Failed to teardown auto user.")
		}
	}()
	// This is where we connect to the actual Postgres database.
	server, hijackedConn, err := e.connect(ctx, sessionCtx)
	if err != nil {
		cancelAutoUserLease()
		return trace.Wrap(err)
	}
	e.rawServerConn = hijackedConn.Conn
	// Release the auto-users semaphore now that we've successfully connected.
	cancelAutoUserLease()
	// Upon successful connect, indicate to the Postgres client that startup
	// has been completed, and it can start sending queries.
	err = e.makeClientReady(e.client, hijackedConn)
	if err != nil {
		return trace.Wrap(err)
	}
	// At this point Postgres client should be ready to start sending
	// messages: this is where psql prompt appears on the other side.
	e.Audit.OnSessionStart(e.Context, sessionCtx, nil)
	defer e.Audit.OnSessionEnd(e.Context, sessionCtx)
	// Reconstruct pgconn.PgConn from hijacked connection for easier access
	// to its utility methods (such as Close).
	serverConn, err := pgconn.Construct(hijackedConn)
	if err != nil {
		return trace.Wrap(err)
	}
	defer func() {
		err = serverConn.Close(ctx)
		if err != nil && !utils.IsOKNetworkError(err) {
			e.Log.WithError(err).Error("Failed to close connection.")
		}
	}()

	observe()

	// Now launch the message exchange relaying all intercepted messages b/w
	// the client (psql or other Postgres client) and the server (database).
	clientErrCh := make(chan error, 1)
	serverErrCh := make(chan error, 1)
	go e.receiveFromClient(e.client, server, clientErrCh, sessionCtx)
	go e.receiveFromServer(serverConn, serverErrCh, sessionCtx)
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

// handleStartup receives a startup message from the proxy and updates
// the session context with the connection parameters.
func (e *Engine) handleStartup(client *pgproto3.Backend, sessionCtx *common.Session) error {
	startupMessageI, err := client.ReceiveStartupMessage()
	if err != nil {
		return trace.Wrap(err)
	}
	switch m := startupMessageI.(type) {
	case *pgproto3.StartupMessage:
		e.Log.Debugf("Received startup message: %#v.", m)
		// Pass startup parameters received from the client along (this is how the
		// client sets default date style format for example), but remove database
		// name and user from them.
		for key, value := range m.Parameters {
			switch key {
			case "database":
				sessionCtx.DatabaseName = value
			case "user":
				sessionCtx.DatabaseUser = value
			default:
				sessionCtx.StartupParameters[key] = value
			}
		}
	case *pgproto3.CancelRequest:
		e.Log.Debugf("Received cancel request for PID: %v.", m.ProcessID)
		e.cancelReq = m
	default:
		return trace.BadParameter("unexpected startup message type: %T", startupMessageI)
	}
	return nil
}

func (e *Engine) checkAccess(ctx context.Context, sessionCtx *common.Session) error {
	if err := sessionCtx.CheckUsernameForAutoUserProvisioning(); err != nil {
		return trace.Wrap(err)
	}
	authPref, err := e.Auth.GetAuthPreference(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	matchers := role.GetDatabaseRoleMatchers(role.RoleMatchersConfig{
		Database:       sessionCtx.Database,
		DatabaseUser:   sessionCtx.DatabaseUser,
		DatabaseName:   sessionCtx.DatabaseName,
		AutoCreateUser: sessionCtx.AutoCreateUserMode.IsEnabled(),
	})
	err = sessionCtx.Checker.CheckAccess(
		sessionCtx.Database,
		sessionCtx.GetAccessState(authPref),
		matchers...,
	)
	if err != nil {
		e.Audit.OnSessionStart(e.Context, sessionCtx, err)
		return trace.Wrap(err)
	}
	return nil
}

// connect establishes the connection to the database instance and returns
// the hijacked connection and the frontend, an interface used for message
// exchange with the database.
func (e *Engine) connect(ctx context.Context, sessionCtx *common.Session) (*pgproto3.Frontend, *pgconn.HijackedConn, error) {
	connectConfig, err := e.getConnectConfig(ctx, sessionCtx)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	// TODO(r0mant): Instead of using pgconn to connect, in future it might
	// make sense to reimplement the connect logic which will give us more
	// control over the initial startup and ability to relay authentication
	// messages b/w server and client e.g. to get client's password.
	conn, err := pgconn.ConnectConfig(ctx, connectConfig)
	if err != nil {
		return nil, nil, common.ConvertConnectError(err, sessionCtx)
	}
	// Hijacked connection exposes some internal connection data, such as
	// parameters we'll need to relay back to the client (e.g. database
	// server version).
	hijackedConn, err := conn.Hijack()
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	frontend := pgproto3.NewFrontend(pgproto3.NewChunkReader(hijackedConn.Conn), hijackedConn.Conn)
	return frontend, hijackedConn, nil
}

// makeClientReady indicates to the Postgres client (such as psql) that the
// server is ready to accept messages from it.
func (e *Engine) makeClientReady(client *pgproto3.Backend, hijackedConn *pgconn.HijackedConn) error {
	// AuthenticationOk indicates that the authentication was successful.
	e.Log.Debug("Sending AuthenticationOk.")
	if err := client.Send(&pgproto3.AuthenticationOk{}); err != nil {
		return trace.Wrap(err)
	}
	// BackendKeyData provides secret-key data that the frontend must save
	// if it wants to be able to issue cancel requests later.
	e.Log.Debugf("Sending BackendKeyData: PID=%v.", hijackedConn.PID)
	if err := client.Send(&pgproto3.BackendKeyData{ProcessID: hijackedConn.PID, SecretKey: hijackedConn.SecretKey}); err != nil {
		return trace.Wrap(err)
	}
	// ParameterStatuses contains parameters reported by the server such as
	// server version, relay them back to the client.
	e.Log.Debugf("Sending ParameterStatuses: %v.", hijackedConn.ParameterStatuses)
	for k, v := range hijackedConn.ParameterStatuses {
		if err := client.Send(&pgproto3.ParameterStatus{Name: k, Value: v}); err != nil {
			return trace.Wrap(err)
		}
	}
	// ReadyForQuery indicates that the start-up is completed and the
	// frontend can now issue commands.
	e.Log.Debug("Sending ReadyForQuery")
	if err := client.Send(&pgproto3.ReadyForQuery{TxStatus: 'I'}); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// receiveFromClient receives messages from the provided backend (which
// in turn receives them from psql or other client) and relays them to
// the frontend connected to the database instance.
func (e *Engine) receiveFromClient(client *pgproto3.Backend, server *pgproto3.Frontend, clientErrCh chan<- error, sessionCtx *common.Session) {
	log := e.Log.WithField("from", "client")
	defer log.Debug("Stop receiving from client.")

	ctr := common.GetMessagesFromClientMetric(sessionCtx.Database)

	for {
		message, err := client.Receive()
		if err != nil {
			log.WithError(err).Errorf("Failed to receive message from client.")
			clientErrCh <- err
			return
		}
		log.Tracef("Received client message: %#v.", message)
		ctr.Inc()

		switch msg := message.(type) {
		case *pgproto3.Query:
			e.auditQueryMessage(sessionCtx, msg)
		case *pgproto3.Parse:
			e.auditParseMessage(sessionCtx, msg)
		case *pgproto3.Bind:
			e.auditBindMessage(sessionCtx, msg)
		case *pgproto3.Execute:
			e.auditExecuteMessage(sessionCtx, msg)
		case *pgproto3.Close:
			e.auditCloseMessage(sessionCtx, msg)
		case *pgproto3.FunctionCall:
			e.auditFuncCallMessage(sessionCtx, msg)
		case *pgproto3.Terminate:
			clientErrCh <- nil
			return
		}
		err = server.Send(message)
		if err != nil {
			log.WithError(err).Error("Failed to send message to server.")
			clientErrCh <- err
			return
		}
	}
}

// auditQueryMessage processes Query wire message which indicates that client
// is executing a simple query.
func (e *Engine) auditQueryMessage(session *common.Session, msg *pgproto3.Query) {
	e.Audit.OnQuery(e.Context, session, common.Query{Query: msg.String})
}

// handleParseMesssage processes Parse wire message which indicates start of the
// extended query protocol (prepared statements):
// https://www.postgresql.org/docs/10/protocol-flow.html#PROTOCOL-FLOW-EXT-QUERY
func (e *Engine) auditParseMessage(session *common.Session, msg *pgproto3.Parse) {
	e.Audit.EmitEvent(e.Context, makeParseEvent(session, msg.Name, msg.Query))
}

// auditBindMessage processes Bind wire message which readies existing prepared
// statement for execution into what Postgres calls a "destination portal",
// optionally binding it with parameters (for parameterized queries).
func (e *Engine) auditBindMessage(session *common.Session, msg *pgproto3.Bind) {
	e.Audit.EmitEvent(e.Context, makeBindEvent(session, msg.PreparedStatement,
		msg.DestinationPortal, formatParameters(msg.Parameters,
			msg.ParameterFormatCodes)))
}

// auditExecuteMessage processes Execute wire message which indicates that
// client is executing the previously parsed and bound prepared statement.
func (e *Engine) auditExecuteMessage(session *common.Session, msg *pgproto3.Execute) {
	e.Audit.EmitEvent(e.Context, makeExecuteEvent(session, msg.Portal))
}

// auditCloseMessage processes Close wire message which indicates that client
// is closing a prepared statement or a destination portal.
func (e *Engine) auditCloseMessage(session *common.Session, msg *pgproto3.Close) {
	switch msg.ObjectType {
	case closeTypePreparedStatement:
		e.Audit.EmitEvent(e.Context, makeCloseEvent(session, msg.Name, ""))
	case closeTypeDestinationPortal:
		e.Audit.EmitEvent(e.Context, makeCloseEvent(session, "", msg.Name))
	}
}

// auditFuncCallMessage processes FunctionCall wire message which indicates
// that client is executing a system function.
func (e *Engine) auditFuncCallMessage(session *common.Session, msg *pgproto3.FunctionCall) {
	var formatCodes []int16
	for _, fc := range msg.ArgFormatCodes {
		formatCodes = append(formatCodes, int16(fc))
	}
	e.Audit.EmitEvent(e.Context, makeFuncCallEvent(session, msg.Function,
		formatParameters(msg.Arguments, formatCodes)))
}

// auditUserPermissions calls OnPermissionsUpdate() with appropriate context.
func (e *Engine) auditUserPermissions(session *common.Session, entries []events.DatabasePermissionEntry) {
	e.Audit.OnPermissionsUpdate(e.Context, session, entries)
}

// receiveFromServer receives messages from the provided frontend (which
// is connected to the database instance) and relays them back to the psql
// or other client via the provided backend.
func (e *Engine) receiveFromServer(serverConn *pgconn.PgConn, serverErrCh chan<- error, sessionCtx *common.Session) {
	log := e.Log.WithField("from", "server")
	ctr := common.GetMessagesFromServerMetric(sessionCtx.Database)

	// parse and count the messages from the server in a separate goroutine,
	// operating on a copy of the server message stream. the copy is arranged below.
	copyReader, copyWriter := io.Pipe()
	closeChan := make(chan struct{})

	go func() {
		defer copyReader.Close()
		defer close(closeChan)

		// server will never be used to write to server,
		// which is why we pass io.Discard instead of e.rawServerConn
		server := pgproto3.NewFrontend(pgproto3.NewChunkReader(copyReader), io.Discard)

		var count int64
		defer func() {
			log.WithField("parsed_total", count).Debug("Stopped parsing messages from server.")
		}()

		for {
			message, err := server.Receive()
			if err != nil {
				if serverConn.IsClosed() {
					log.Debug("Server connection closed.")
					return
				}
				log.WithError(err).Error("Failed to receive message from server.")
				return
			}

			count += 1
			ctr.Inc()
			log.Tracef("Received server message: %#v.", message)
		}
	}()

	// the messages are ultimately copied from e.rawServerConn to e.rawClientConn,
	// but a copy of that message stream is written to a synchronous pipe,
	// which is read by the analysis goroutine above.
	total, err := io.Copy(e.rawClientConn, io.TeeReader(e.rawServerConn, copyWriter))
	if err != nil && !trace.IsConnectionProblem(trace.ConvertSystemError(err)) {
		log.WithError(err).Warn("Server -> Client copy finished with unexpected error.")
	}

	// We need to close the writer half of the pipe to notify the analysis
	// goroutine that the connection is done. This will result in the goroutine
	// receiving an io.ErrClosedPipe error, which will cause it to finish its
	// execution. After that, wait until the closeChan is closed to ensure the
	// goroutine is completed, avoiding data races.
	copyWriter.Close()
	<-closeChan

	serverErrCh <- trace.Wrap(err)
	log.Debugf("Stopped receiving from server. Transferred %v bytes.", total)
}

// getConnectConfig returns config that can be used to connect to the
// database instance.
func (e *Engine) getConnectConfig(ctx context.Context, sessionCtx *common.Session) (*pgconn.Config, error) {
	// The driver requires the config to be built by parsing the connection
	// string so parse the basic template and then fill in the rest of
	// parameters such as TLS configuration.
	config, err := pgconn.ParseConfig(fmt.Sprintf("postgres://%s", sessionCtx.Database.GetURI()))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// TLS config will use client certificate for an onprem database or
	// will contain RDS root certificate for RDS/Aurora.
	config.TLSConfig, err = e.Auth.GetTLSConfig(ctx, sessionCtx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	config.User = sessionCtx.DatabaseUser
	config.Database = sessionCtx.DatabaseName
	// Pgconn adds fallbacks to retry connection without TLS if the TLS
	// attempt fails. Reset the fallbacks to avoid retries, otherwise
	// it's impossible to debug TLS connection errors.
	config.Fallbacks = nil
	// Set startup parameters that the client sent us.
	config.RuntimeParams = sessionCtx.StartupParameters
	// AWS RDS/Aurora and GCP Cloud SQL use IAM authentication so request an
	// auth token and use it as a password.
	switch sessionCtx.Database.GetType() {
	case types.DatabaseTypeRDS, types.DatabaseTypeRDSProxy:
		config.Password, err = e.Auth.GetRDSAuthToken(ctx, sessionCtx)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	case types.DatabaseTypeRedshift:
		config.User, config.Password, err = e.Auth.GetRedshiftAuthToken(ctx, sessionCtx)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	case types.DatabaseTypeRedshiftServerless:
		config.User, config.Password, err = e.Auth.GetRedshiftServerlessAuthToken(ctx, sessionCtx)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	case types.DatabaseTypeCloudSQL:
		config.Password, err = e.Auth.GetCloudSQLAuthToken(ctx, sessionCtx)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		// Get the client once for subsequent calls (it acquires a read lock).
		gcpClient, err := e.CloudClients.GetGCPSQLAdminClient(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		// Detect whether the instance is set to require SSL.
		// Fallback to not requiring SSL for access denied errors.
		requireSSL, err := cloud.GetGCPRequireSSL(ctx, sessionCtx, gcpClient)
		if err != nil && !trace.IsAccessDenied(err) {
			return nil, trace.Wrap(err)
		}
		// Create ephemeral certificate and append to TLS config when
		// the instance requires SSL.
		if requireSSL {
			err = cloud.AppendGCPClientCert(ctx, sessionCtx, gcpClient, config.TLSConfig)
			if err != nil {
				return nil, trace.Wrap(err)
			}
		}
	case types.DatabaseTypeAzure:
		config.Password, err = e.Auth.GetAzureAccessToken(ctx, sessionCtx)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		config.User = services.MakeAzureDatabaseLoginUsername(sessionCtx.Database, config.User)
	}
	return config, nil
}

// handleCancelRequest handles a cancel request and returns immediately (closing the connection).
func (e *Engine) handleCancelRequest(ctx context.Context, sessionCtx *common.Session) error {
	config, err := pgconn.ParseConfig(fmt.Sprintf("postgres://%s", sessionCtx.Database.GetURI()))
	if err != nil {
		return trace.Wrap(err)
	}
	tlsConfig, err := e.Auth.GetTLSConfig(ctx, sessionCtx)
	if err != nil {
		return trace.Wrap(err)
	}
	// We can't use pgconn in this case because it always sends a
	// startup message.
	// Instead, use the pgconn config string parser for convenience and dial
	// db host:port ourselves.
	network, address := pgconn.NetworkAddress(config.Host, config.Port)
	if err != nil {
		return trace.Wrap(err)
	}
	dialer := net.Dialer{Timeout: defaults.DefaultIOTimeout}
	conn, err := dialer.DialContext(ctx, network, address)
	if err != nil {
		return common.ConvertConnectError(err, sessionCtx)
	}
	tlsConn, err := startPGWireTLS(conn, tlsConfig)
	if err != nil {
		return common.ConvertConnectError(err, sessionCtx)
	}
	frontend := pgproto3.NewFrontend(pgproto3.NewChunkReader(tlsConn), tlsConn)
	if err = frontend.Send(e.cancelReq); err != nil {
		return trace.Wrap(err)
	}
	response := make([]byte, 1)
	if _, err := tlsConn.Read(response); err != io.EOF {
		// server should close the connection after receiving cancel request.
		return trace.Wrap(err)
	}
	return nil
}

// startPGWireTLS is a helper func that upgrades upstream connection to TLS.
// copied from github.com/jackc/pgconn.startTLS.
func startPGWireTLS(conn net.Conn, tlsConfig *tls.Config) (net.Conn, error) {
	frontend := pgproto3.NewFrontend(pgproto3.NewChunkReader(conn), conn)
	if err := frontend.Send(&pgproto3.SSLRequest{}); err != nil {
		return nil, trace.Wrap(err)
	}
	response := make([]byte, 1)
	if _, err := io.ReadFull(conn, response); err != nil {
		return nil, trace.Wrap(err)
	}
	if response[0] != 'S' {
		return nil, trace.Errorf("server refused TLS connection")
	}
	return tls.Client(conn, tlsConfig), nil
}

// formatParameters converts parameters from the Postgres wire message into
// their string representations for including in the audit log.
func formatParameters(parameters [][]byte, formatCodes []int16) (formatted []string) {
	// Each parameter can be either a text or a binary which is determined
	// by "parameter format codes" in the Bind message (0 - text, 1 - binary).
	//
	// Be a bit paranoid and make sure that number of format codes matches the
	// number of parameters, or there are zero or one format codes.
	// zero format codes applies text format to all params.
	// one format code applies the same format code to all params.
	// https://www.postgresql.org/docs/current/protocol-message-formats.html#PROTOCOL-MESSAGE-FORMATS-BIND
	// https://www.postgresql.org/docs/current/protocol-message-formats.html#PROTOCOL-MESSAGE-FORMATS-FUNCTIONCALL
	if len(formatCodes) > 1 && len(formatCodes) != len(parameters) {
		logrus.Warnf("Postgres parameter format codes and parameters don't match: %#v %#v.",
			parameters, formatCodes)
		return formatted
	}
	for i, p := range parameters {
		// According to Bind message documentation, if there are no parameter
		// format codes, it may mean that either there are no parameters, or
		// that all parameters use default text format.
		var formatCode int16
		switch len(formatCodes) {
		case 0:
			// use default 0 (text) format for all params.
		case 1:
			// apply the same format code to all params.
			formatCode = formatCodes[0]
		default:
			// apply format code corresponding to this param.
			formatCode = formatCodes[i]
		}

		switch formatCode {
		case parameterFormatCodeText:
			// Text parameters can just be converted to their string
			// representation.
			formatted = append(formatted, string(p))
		case parameterFormatCodeBinary:
			// For binary parameters, encode the parameter as a base64 string.
			formatted = append(formatted, base64.StdEncoding.EncodeToString(p))
		default:
			// Should never happen but...
			logrus.Warnf("Unknown Postgres parameter format code: %#v.",
				formatCode)
			formatted = append(formatted, "<unknown>")
		}
	}
	return formatted
}

const (
	// parameterFormatCodeText indicates that this is a text query parameter.
	parameterFormatCodeText = 0
	// parameterFormatCodeBinary indicates that this is a binary query parameter.
	parameterFormatCodeBinary = 1

	// closeTypePreparedStatement indicates that a prepared statement is being
	// closed by the Close message.
	closeTypePreparedStatement = 'S'
	// closeTypeDestinationPortal indicates that a destination portal is being
	// closed by the Close message.
	closeTypeDestinationPortal = 'P'
)
