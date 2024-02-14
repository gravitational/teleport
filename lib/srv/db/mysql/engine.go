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

package mysql

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/go-mysql-org/go-mysql/client"
	"github.com/go-mysql-org/go-mysql/mysql"
	"github.com/go-mysql-org/go-mysql/packet"
	"github.com/go-mysql-org/go-mysql/server"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/db/cloud"
	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/gravitational/teleport/lib/srv/db/common/role"
	"github.com/gravitational/teleport/lib/srv/db/mysql/protocol"
	"github.com/gravitational/teleport/lib/utils"
)

// NewEngine create new MySQL engine.
func NewEngine(ec common.EngineConfig) common.Engine {
	return &Engine{
		EngineConfig: ec,
	}
}

// Engine implements the MySQL database service that accepts client
// connections coming over reverse tunnel from the proxy and proxies
// them between the proxy and the MySQL database instance.
//
// Implements common.Engine.
type Engine struct {
	// EngineConfig is the common database engine configuration.
	common.EngineConfig
	// proxyConn is a client connection.
	proxyConn server.Conn
}

// InitializeConnection initializes the engine with client connection.
func (e *Engine) InitializeConnection(clientConn net.Conn, _ *common.Session) error {
	// Make server conn to get access to protocol's WriteOK/WriteError methods.
	e.proxyConn = server.Conn{Conn: packet.NewConn(clientConn)}
	return nil
}

// SendError sends an error to connected client in the MySQL understandable format.
func (e *Engine) SendError(err error) {
	if writeErr := e.proxyConn.WriteError(trace.Unwrap(err)); writeErr != nil {
		e.Log.WithError(writeErr).Debugf("Failed to send error %q to MySQL client.", err)
	}
}

// HandleConnection processes the connection from MySQL proxy coming
// over reverse tunnel.
//
// It handles all necessary startup actions, authorization and acts as a
// middleman between the proxy and the database intercepting and interpreting
// all messages i.e. doing protocol parsing.
func (e *Engine) HandleConnection(ctx context.Context, sessionCtx *common.Session) error {
	observe := common.GetConnectionSetupTimeObserver(sessionCtx.Database)

	// Perform authorization checks.
	err := e.checkAccess(ctx, sessionCtx)
	if err != nil {
		return trace.Wrap(err)
	}

	// Automatically create the database user if needed.
	cancelAutoUserLease, err := e.GetUserProvisioner(e).Activate(ctx, sessionCtx)
	if err != nil {
		return trace.Wrap(err)
	}
	defer func() {
		err := e.GetUserProvisioner(e).Teardown(ctx, sessionCtx)
		if err != nil {
			e.Log.WithError(err).Error("Failed to teardown the user.")
		}
	}()

	// Establish connection to the MySQL server.
	serverConn, err := e.connect(ctx, sessionCtx)
	if err != nil {
		defer cancelAutoUserLease()
		if trace.IsLimitExceeded(err) {
			return trace.LimitExceeded("could not connect to the database, please try again later")
		}
		return trace.Wrap(err)
	}
	defer func() {
		err := serverConn.Close()
		if err != nil {
			e.Log.WithError(err).Error("Failed to close connection to MySQL server.")
		}
	}()

	// Release the auto-users semaphore now that we've successfully connected.
	cancelAutoUserLease()

	// Internally, updateServerVersion() updates databases only when database version
	// is not set, or it has changed since previous call.
	if err := e.updateServerVersion(sessionCtx, serverConn); err != nil {
		// Log but do not fail connection if the version update fails.
		e.Log.WithError(err).Warnf("Failed to update the MySQL server version.")

	}

	// Send back OK packet to indicate auth/connect success. At this point
	// the original client should consider the connection phase completed.
	err = e.proxyConn.WriteOK(nil)
	if err != nil {
		return trace.Wrap(err)
	}

	e.Audit.OnSessionStart(e.Context, sessionCtx, nil)
	defer e.Audit.OnSessionEnd(e.Context, sessionCtx)

	observe()

	// Copy between the connections.
	clientErrCh := make(chan error, 1)
	serverErrCh := make(chan error, 1)
	go e.receiveFromClient(e.proxyConn.Conn, serverConn, clientErrCh, sessionCtx)
	go e.receiveFromServer(serverConn, e.proxyConn.Conn, serverErrCh, sessionCtx)
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

// updateServerVersion updates the server runtime version if the version reported by the database is different from
// the version in status configuration.
func (e *Engine) updateServerVersion(sessionCtx *common.Session, serverConn *client.Conn) error {
	serverVersion := serverConn.GetServerVersion()
	statusVersion := sessionCtx.Database.GetMySQLServerVersion()
	// Update only when needed
	if serverVersion != "" && serverVersion != statusVersion {
		sessionCtx.Database.SetMySQLServerVersion(serverVersion)
	}

	return nil
}

// checkAccess does authorization check for MySQL connection about to be established.
func (e *Engine) checkAccess(ctx context.Context, sessionCtx *common.Session) error {
	if err := sessionCtx.CheckUsernameForAutoUserProvisioning(); err != nil {
		return trace.Wrap(err)
	}

	authPref, err := e.Auth.GetAuthPreference(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	state := sessionCtx.GetAccessState(authPref)
	dbRoleMatchers := role.GetDatabaseRoleMatchers(role.RoleMatchersConfig{
		Database:       sessionCtx.Database,
		DatabaseUser:   sessionCtx.DatabaseUser,
		DatabaseName:   sessionCtx.DatabaseName,
		AutoCreateUser: sessionCtx.AutoCreateUserMode.IsEnabled(),
	})
	err = sessionCtx.Checker.CheckAccess(
		sessionCtx.Database,
		state,
		dbRoleMatchers...,
	)
	if err != nil {
		e.Audit.OnSessionStart(e.Context, sessionCtx, err)
		return trace.Wrap(err)
	}
	return nil
}

// connect establishes connection to MySQL database.
func (e *Engine) connect(ctx context.Context, sessionCtx *common.Session) (*client.Conn, error) {
	tlsConfig, err := e.Auth.GetTLSConfig(ctx, sessionCtx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	user := sessionCtx.DatabaseUser
	connectOpt := func(conn *client.Conn) {
		conn.SetTLSConfig(tlsConfig)
	}

	var dialer client.Dialer
	var password string
	switch {
	case sessionCtx.Database.IsRDS(), sessionCtx.Database.IsRDSProxy():
		password, err = e.Auth.GetRDSAuthToken(ctx, sessionCtx)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	case sessionCtx.Database.IsCloudSQL():
		// Get the client once for subsequent calls (it acquires a read lock).
		gcpClient, err := e.CloudClients.GetGCPSQLAdminClient(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		user, password, err = e.getGCPUserAndPassword(ctx, sessionCtx, gcpClient)
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
		// the instance requires SSL. Also use a TLS dialer instead of
		// the default net dialer when GCP requires SSL.
		if requireSSL {
			err = cloud.AppendGCPClientCert(ctx, sessionCtx, gcpClient, tlsConfig)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			connectOpt = func(*client.Conn) {}
			dialer = newGCPTLSDialer(tlsConfig)
		}
	case sessionCtx.Database.IsAzure():
		password, err = e.Auth.GetAzureAccessToken(ctx, sessionCtx)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		user = services.MakeAzureDatabaseLoginUsername(sessionCtx.Database, user)
	}

	// Use default net dialer unless it is already initialized.
	if dialer == nil {
		var nd net.Dialer
		dialer = nd.DialContext
	}

	// TODO(r0mant): Set CLIENT_INTERACTIVE flag on the client?
	conn, err := client.ConnectWithDialer(ctx, "tcp", sessionCtx.Database.GetURI(),
		user,
		password,
		sessionCtx.DatabaseName,
		dialer,
		connectOpt,
		// client-set capabilities only.
		// TODO(smallinsky) Forward "real" capabilities from mysql client to mysql server.
		withClientCapabilities(
			mysql.CLIENT_MULTI_RESULTS,
			mysql.CLIENT_MULTI_STATEMENTS,
		),
	)
	if err != nil {
		return nil, common.ConvertConnectError(err, sessionCtx)
	}
	return conn, nil
}

func withClientCapabilities(caps ...uint32) func(conn *client.Conn) {
	return func(conn *client.Conn) {
		for _, cap := range caps {
			conn.SetCapability(cap)
		}
	}
}

// receiveFromClient relays protocol messages received from MySQL client
// to MySQL database.
func (e *Engine) receiveFromClient(clientConn, serverConn net.Conn, clientErrCh chan<- error, sessionCtx *common.Session) {
	log := e.Log.WithFields(logrus.Fields{
		"from":   "client",
		"client": clientConn.RemoteAddr(),
		"server": serverConn.RemoteAddr(),
	})
	defer func() {
		log.Debug("Stop receiving from client.")
		close(clientErrCh)
	}()

	msgFromClient := common.GetMessagesFromClientMetric(sessionCtx.Database)

	for {
		packet, err := protocol.ParsePacket(clientConn)
		if err != nil {
			if utils.IsOKNetworkError(err) {
				log.Debug("Client connection closed.")
				return
			}
			log.WithError(err).Error("Failed to read client packet.")
			clientErrCh <- err
			return
		}

		msgFromClient.Inc()

		switch pkt := packet.(type) {
		case *protocol.Query:
			e.Audit.OnQuery(e.Context, sessionCtx, common.Query{Query: pkt.Query()})
		case *protocol.ChangeUser:
			// MySQL protocol includes COM_CHANGE_USER command that allows to
			// re-authenticate connection as a different user. It is not
			// supported by mysql shell and most drivers but some drivers do
			// provide it.
			//
			// We do not want to allow changing the connection user and instead
			// force users to go through normal reconnect flow so log the
			// attempt and close the client connection.
			log.Warnf("Rejecting attempt to change user to %q for session %v.", pkt.User(), sessionCtx)
			return
		case *protocol.Quit:
			return

		case *protocol.InitDB:
			// Update DatabaseName when switching to another so the audit logs
			// are up to date. E.g.:
			// mysql> use foo;
			// mysql> select * from users;
			sessionCtx.DatabaseName = pkt.SchemaName()

			e.Audit.EmitEvent(e.Context, makeInitDBEvent(sessionCtx, pkt))
		case *protocol.CreateDB:
			e.Audit.EmitEvent(e.Context, makeCreateDBEvent(sessionCtx, pkt))
		case *protocol.DropDB:
			e.Audit.EmitEvent(e.Context, makeDropDBEvent(sessionCtx, pkt))
		case *protocol.ShutDown:
			e.Audit.EmitEvent(e.Context, makeShutDownEvent(sessionCtx, pkt))
		case *protocol.ProcessKill:
			e.Audit.EmitEvent(e.Context, makeProcessKillEvent(sessionCtx, pkt))
		case *protocol.Debug:
			e.Audit.EmitEvent(e.Context, makeDebugEvent(sessionCtx, pkt))
		case *protocol.Refresh:
			e.Audit.EmitEvent(e.Context, makeRefreshEvent(sessionCtx, pkt))

		case *protocol.StatementPreparePacket:
			e.Audit.EmitEvent(e.Context, makeStatementPrepareEvent(sessionCtx, pkt))
		case *protocol.StatementExecutePacket:
			// TODO(greedy52) Number of parameters is required to parse
			// parameters out of the packet. Parameter definitions are required
			// to properly format the parameters for including in the audit
			// log. Both number of parameters and parameter definitions can be
			// obtained from the response of COM_STMT_PREPARE.
			e.Audit.EmitEvent(e.Context, makeStatementExecuteEvent(sessionCtx, pkt))
		case *protocol.StatementSendLongDataPacket:
			e.Audit.EmitEvent(e.Context, makeStatementSendLongDataEvent(sessionCtx, pkt))
		case *protocol.StatementClosePacket:
			e.Audit.EmitEvent(e.Context, makeStatementCloseEvent(sessionCtx, pkt))
		case *protocol.StatementResetPacket:
			e.Audit.EmitEvent(e.Context, makeStatementResetEvent(sessionCtx, pkt))
		case *protocol.StatementFetchPacket:
			e.Audit.EmitEvent(e.Context, makeStatementFetchEvent(sessionCtx, pkt))
		case *protocol.StatementBulkExecutePacket:
			// TODO(greedy52) Number of parameters and parameter definitions
			// are required. See above comments for StatementExecutePacket.
			e.Audit.EmitEvent(e.Context, makeStatementBulkExecuteEvent(sessionCtx, pkt))
		}
		_, err = protocol.WritePacket(packet.Bytes(), serverConn)
		if err != nil {
			log.WithError(err).Error("Failed to write server packet.")
			clientErrCh <- err
			return
		}
	}
}

// receiveFromServer relays protocol messages received from MySQL database
// to MySQL client.
func (e *Engine) receiveFromServer(serverConn, clientConn net.Conn, serverErrCh chan<- error, sessionCtx *common.Session) {
	log := e.Log.WithFields(logrus.Fields{
		"from":   "server",
		"client": clientConn.RemoteAddr(),
		"server": serverConn.RemoteAddr(),
	})
	messagesCounter := common.GetMessagesFromServerMetric(sessionCtx.Database)

	// parse and count the messages from the server in a separate goroutine,
	// operating on a copy of the server message stream. the copy is arranged below.
	copyReader, copyWriter := io.Pipe()
	defer copyWriter.Close()

	go func() {
		defer copyReader.Close()

		var count int64
		defer func() {
			log.WithField("parsed_total", count).Debug("Stopped parsing messages from server.")
		}()

		for {
			_, _, err := protocol.ReadPacket(copyReader)
			if err != nil {
				return
			}

			count += 1
			messagesCounter.Inc()
		}
	}()

	// the messages are ultimately copied from serverConn to clientConn,
	// but a copy of that message stream is written to a synchronous pipe,
	// which is read by the analysis goroutine above.
	total, err := io.Copy(clientConn, io.TeeReader(serverConn, copyWriter))
	if err != nil {
		if utils.IsOKNetworkError(err) {
			log.Debug("Server connection closed.")
		} else {
			log.WithError(err).Warn("Server -> Client copy finished with unexpected error.")
		}
	}

	log.Debugf("Stopped receiving from server. Transferred %v bytes.", total)
	serverErrCh <- trace.Wrap(err)
}

// makeAcquireSemaphoreConfig builds parameters for acquiring a semaphore
// for connecting to a MySQL Cloud SQL instance for this session.
func (e *Engine) makeAcquireSemaphoreConfig(sessionCtx *common.Session) services.AcquireSemaphoreWithRetryConfig {
	return services.AcquireSemaphoreWithRetryConfig{
		Service: e.AuthClient,
		// The semaphore will serialize connections to the database as specific
		// user. If we fail to release the lock for some reason, it will expire
		// in a minute anyway.
		Request: types.AcquireSemaphoreRequest{
			SemaphoreKind: "gcp-mysql-token",
			SemaphoreName: fmt.Sprintf("%v-%v", sessionCtx.Database.GetName(), sessionCtx.DatabaseUser),
			MaxLeases:     1,
			Expires:       e.Clock.Now().Add(time.Minute),
		},
		// If multiple connections are being established simultaneously to the
		// same database as the same user, retry for a few seconds.
		Retry: retryutils.LinearConfig{
			Step:  time.Second,
			Max:   time.Second,
			Clock: e.Clock,
		},
	}
}

// newGCPTLSDialer returns a TLS dialer configured to connect to the Cloud Proxy
// port rather than the default MySQL port.
func newGCPTLSDialer(tlsConfig *tls.Config) client.Dialer {
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		// Workaround issue generating ephemeral certificates for secure connections
		// by creating a TLS connection to the Cloud Proxy port overriding the
		// MySQL client's connection. MySQL on the default port does not trust
		// the ephemeral certificate's CA but Cloud Proxy does.
		host, port, err := net.SplitHostPort(address)
		if err == nil && port == gcpSQLListenPort {
			address = net.JoinHostPort(host, gcpSQLProxyListenPort)
		}
		tlsDialer := tls.Dialer{Config: tlsConfig}
		return tlsDialer.DialContext(ctx, network, address)
	}
}

// FetchMySQLVersion connects to MySQL database and tries to read the handshake packet and return the version.
// In case of error the message returned by the database is propagated in returned error.
func FetchMySQLVersion(ctx context.Context, database types.Database) (string, error) {
	var dialer client.Dialer

	if database.IsCloudSQL() {
		dialer = newGCPTLSDialer(&tls.Config{})
	} else {
		var nd net.Dialer
		dialer = nd.DialContext
	}

	return protocol.FetchMySQLVersionInternal(ctx, dialer, database.GetURI())
}

const (
	// gcpSQLListenPort is the port used by Cloud SQL MySQL instances.
	gcpSQLListenPort = "3306"
	// gcpSQLProxyListenPort is the port used by Cloud Proxy for MySQL instances.
	gcpSQLProxyListenPort = "3307"
)
