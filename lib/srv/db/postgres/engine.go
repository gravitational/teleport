/*
Copyright 2020-2021 Gravitational, Inc.

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
	"errors"
	"fmt"
	"net"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/db/cloud"
	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/gravitational/teleport/lib/srv/db/common/role"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/jackc/pgconn"
	"github.com/jackc/pgproto3/v2"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"
)

// Engine implements the Postgres database service that accepts client
// connections coming over reverse tunnel from the proxy and proxies
// them between the proxy and the Postgres database instance.
//
// Implements common.Engine.
type Engine struct {
	// Auth handles database access authentication.
	Auth common.Auth
	// Audit emits database access audit events.
	Audit common.Audit
	// Context is the database server close context.
	Context context.Context
	// Clock is the clock interface.
	Clock clockwork.Clock
	// CloudClients provides access to cloud API clients.
	CloudClients common.CloudClients
	// Log is used for logging.
	Log logrus.FieldLogger
	// client is a client connection.
	client *pgproto3.Backend
}

// InitializeConnection initializes the client connection.
func (e *Engine) InitializeConnection(clientConn net.Conn, sessionCtx *common.Session) error {
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
	// Now we know which database/username the user is connecting to, so
	// perform an authorization check.
	err := e.checkAccess(ctx, sessionCtx)
	if err != nil {
		return trace.Wrap(err)
	}
	// This is where we connect to the actual Postgres database.
	server, hijackedConn, err := e.connect(ctx, sessionCtx)
	if err != nil {
		return trace.Wrap(err)
	}
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
	// Now launch the message exchange relaying all intercepted messages b/w
	// the client (psql or other Postgres client) and the server (database).
	clientErrCh := make(chan error, 1)
	serverErrCh := make(chan error, 1)
	go e.receiveFromClient(e.client, server, clientErrCh, sessionCtx)
	go e.receiveFromServer(server, e.client, serverConn, serverErrCh, sessionCtx)
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
	e.Log.Debugf("Received startup message: %#v.", startupMessageI)
	startupMessage, ok := startupMessageI.(*pgproto3.StartupMessage)
	if !ok {
		return trace.BadParameter("expected *pgproto3.StartupMessage, got %T", startupMessageI)
	}
	// Pass startup parameters received from the client along (this is how the
	// client sets default date style format for example), but remove database
	// name and user from them.
	for key, value := range startupMessage.Parameters {
		switch key {
		case "database":
			sessionCtx.DatabaseName = value
		case "user":
			sessionCtx.DatabaseUser = value
		default:
			sessionCtx.StartupParameters[key] = value
		}
	}
	return nil
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

	dbRoleMatchers := role.DatabaseRoleMatchers(
		sessionCtx.Database.GetProtocol(),
		sessionCtx.DatabaseUser,
		sessionCtx.DatabaseName,
	)
	err = sessionCtx.Checker.CheckAccess(
		sessionCtx.Database,
		mfaParams,
		dbRoleMatchers...,
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
		if trace.IsAccessDenied(common.ConvertError(err)) && sessionCtx.Database.IsRDS() {
			return nil, nil, trace.AccessDenied(`Could not connect to database:

  %v

Make sure that Postgres user %q has "rds_iam" role and Teleport database
agent's IAM policy has "rds-connect" permissions (note that IAM changes may
take a few minutes to propagate):

%v
`, common.ConvertError(err), sessionCtx.DatabaseUser, sessionCtx.Database.GetIAMPolicy())
		}
		return nil, nil, trace.Wrap(err)
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
	for {
		message, err := client.Receive()
		if err != nil {
			log.WithError(err).Errorf("Failed to receive message from client.")
			clientErrCh <- err
			return
		}
		log.Debugf("Received client message: %#v.", message)
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

// receiveFromServer receives messages from the provided frontend (which
// is connected to the database instance) and relays them back to the psql
// or other client via the provided backend.
func (e *Engine) receiveFromServer(server *pgproto3.Frontend, client *pgproto3.Backend, serverConn *pgconn.PgConn, serverErrCh chan<- error, sessionCtx *common.Session) {
	log := e.Log.WithField("from", "server")
	defer log.Debug("Stop receiving from server.")
	for {
		message, err := server.Receive()
		if err != nil {
			if serverConn.IsClosed() {
				log.Debug("Server connection closed.")
				serverErrCh <- nil
				return
			}
			log.WithError(err).Errorf("Failed to receive message from server.")
			serverErrCh <- err
			return
		}
		log.Debugf("Received server message: %#v.", message)
		// This is where we would plug in custom logic for particular
		// messages received from the Postgres server (i.e. emitting
		// an audit event), but for now just pass them along back to
		// the client.
		err = client.Send(message)
		if err != nil {
			log.WithError(err).Error("Failed to send message to client.")
			serverErrCh <- err
			return
		}
	}
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
	case types.DatabaseTypeRDS:
		config.Password, err = e.Auth.GetRDSAuthToken(sessionCtx)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	case types.DatabaseTypeRedshift:
		config.User, config.Password, err = e.Auth.GetRedshiftAuthToken(sessionCtx)
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
		// Azure requires database login to be <user>@<server-name> e.g.
		// alice@postgres-server-name.
		config.User = fmt.Sprintf("%v@%v", config.User, sessionCtx.Database.GetAzure().Name)
	}
	return config, nil
}

// formatParameters converts parameters from the Postgres wire message into
// their string representations for including in the audit log.
func formatParameters(parameters [][]byte, formatCodes []int16) (formatted []string) {
	// Each parameter can be either a text or a binary which is determined
	// by "parameter format codes" in the Bind message (0 - text, 1 - binary).
	//
	// Be a bit paranoid and make sure that number of format codes matches the
	// number of parameters, or there are no format codes in which case all
	// parameters will be text.
	if len(formatCodes) != 0 && len(formatCodes) != len(parameters) {
		logrus.Warnf("Postgres parameter format codes and parameters don't match: %#v %#v.",
			parameters, formatCodes)
		return formatted
	}
	for i, p := range parameters {
		// According to Bind message documentation, if there are no parameter
		// format codes, it may mean that either there are no parameters, or
		// that all parameters use default text format.
		if len(formatCodes) == 0 {
			formatted = append(formatted, string(p))
			continue
		}
		switch formatCodes[i] {
		case parameterFormatCodeText:
			// Text parameters can just be converted to their string
			// representation.
			formatted = append(formatted, string(p))
		case parameterFormatCodeBinary:
			// For binary parameters, just put a placeholder to avoid
			// spamming the audit log with unreadable info.
			formatted = append(formatted, "<binary>")
		default:
			// Should never happen but...
			logrus.Warnf("Unknown Postgres parameter format code: %#v.",
				formatCodes[i])
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
