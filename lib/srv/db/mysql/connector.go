/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"time"

	"github.com/go-mysql-org/go-mysql/client"
	"github.com/go-mysql-org/go-mysql/mysql"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"golang.org/x/time/rate"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/srv/db/common"
	discoverycommon "github.com/gravitational/teleport/lib/srv/discovery/common"
)

func newConnector(cfg connectorConfig) (*connector, error) {
	if err := cfg.checkAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &connector{
		connectorConfig: cfg,
		gcpAuth: &gcpAuth{
			auth:         cfg.auth,
			authClient:   cfg.authClient,
			clients:      cfg.gcpClients,
			clock:        cfg.clock,
			database:     cfg.database,
			databaseUser: cfg.databaseUser,
			log:          cfg.log,
			// avoid checking the ssl mode more than once every 30 minutes
			sometimes: &rate.Sometimes{Interval: 30 * time.Minute},
		},
	}, nil
}

type connectorConfig struct {
	auth       common.Auth
	authClient *authclient.Client
	clock      clockwork.Clock
	gcpClients gcpClients
	log        *slog.Logger

	database     types.Database
	databaseName string
	databaseUser string
}

func (cfg *connectorConfig) checkAndSetDefaults() error {
	if cfg.auth == nil {
		return trace.BadParameter("missing auth")
	}
	if cfg.authClient == nil {
		return trace.BadParameter("missing authClient")
	}
	if cfg.gcpClients == nil {
		return trace.BadParameter("missing gcpClients")
	}
	if cfg.database == nil {
		return trace.BadParameter("missing database")
	}
	if cfg.databaseUser == "" {
		return trace.BadParameter("missing databaseUser")
	}

	if cfg.clock == nil {
		cfg.clock = clockwork.NewRealClock()
	}
	if cfg.log == nil {
		cfg.log = slog.Default()
	}
	return nil
}

// connector connects to MySQL databases.
type connector struct {
	connectorConfig

	gcpAuth *gcpAuth
}

// connect establishes connection to MySQL database.
func (c *connector) connect(
	ctx context.Context,
	certExpiry time.Time,
	onDial func(context.Context, net.Conn),
) (*client.Conn, error) {
	tlsConfig, err := c.auth.GetTLSConfig(ctx, certExpiry, c.database, c.databaseUser)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	user := c.databaseUser
	connectOpt := func(conn *client.Conn) error {
		conn.SetTLSConfig(tlsConfig)
		return nil
	}

	var dialer client.Dialer
	var password string
	switch {
	case c.database.IsRDS(), c.database.IsRDSProxy():
		password, err = c.auth.GetRDSAuthToken(ctx, c.database, c.databaseUser)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	case c.database.IsCloudSQL():
		user, password, err = c.gcpAuth.getGCPUserAndPassword(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		requireSSL, err := c.gcpAuth.checkSSLRequired(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		// Create ephemeral certificate and append to TLS config when
		// the instance requires SSL. Also use a TLS dialer instead of
		// the default net dialer when GCP requires SSL.
		if requireSSL {
			if err := c.gcpAuth.appendGCPClientCert(ctx, certExpiry, tlsConfig); err != nil {
				return nil, trace.Wrap(err)
			}
			connectOpt = func(*client.Conn) error {
				return nil
			}
			dialer = newGCPTLSDialer(tlsConfig)
		}
	case c.database.IsAzure():
		password, err = c.auth.GetAzureAccessToken(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		user = discoverycommon.MakeAzureDatabaseLoginUsername(c.database, user)
	}

	// Use default net dialer unless it is already initialized.
	if dialer == nil {
		var nd net.Dialer
		dialer = nd.DialContext
	}

	c.log.DebugContext(ctx, "Connecting to database")
	// TODO(r0mant): Set CLIENT_INTERACTIVE flag on the client?
	conn, err := client.ConnectWithDialer(ctx, "tcp", c.database.GetURI(),
		user,
		password,
		c.databaseName,
		func(ctx context.Context, network, address string) (net.Conn, error) {
			conn, err := dialer(ctx, network, address)
			if err != nil {
				return nil, trace.Wrap(newDialError(err))
			}
			if onDial != nil {
				recorder := newRecorderConn(conn)
				onDial(ctx, recorder)
				return recorder.rewind(), nil
			}
			return conn, nil
		},
		connectOpt,
		// client-set capabilities only.
		// TODO(smallinsky) Forward "real" capabilities from mysql client to mysql server.
		withClientCapabilities(
			mysql.CLIENT_MULTI_RESULTS,
			mysql.CLIENT_MULTI_STATEMENTS,
		),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return conn, nil
}

// updateServerVersion updates the server runtime version if the version
// reported by the database is different from the version in status
// configuration.
func updateServerVersion(
	ctx context.Context,
	log *slog.Logger,
	db types.Database,
	serverVersion string,
	updateProxiedDatabase func(string, func(types.Database) error) error,
) error {
	statusVersion := db.GetMySQLServerVersion()

	// Update only when needed.
	if serverVersion == "" || serverVersion == statusVersion {
		return nil
	}

	// Note that db may be a copy of the database cached by database service.
	// Call updateProxiedDatabase to update the original as well.
	db.SetMySQLServerVersion(serverVersion)
	doUpdate := func(db types.Database) error {
		db.SetMySQLServerVersion(serverVersion)
		return nil
	}

	log.DebugContext(ctx, "Updated MySQL server version",
		"old_version", statusVersion,
		"version", serverVersion,
	)
	return trace.Wrap(updateProxiedDatabase(db.GetName(), doUpdate))
}

func newRecorderConn(conn net.Conn) *recorderConn {
	rc := &recorderConn{Conn: conn}
	rc.reset()
	return rc
}

type recorderConn struct {
	net.Conn
	// buf contains the recorded data read since the last reset or rewind.
	buf bytes.Buffer
	// recordingReader is used to limit reads from the conn that are written to the
	// buffer. The recorder enforces a limit to prevent accidental memory
	// exhaustion.
	recordingReader *io.LimitedReader
}

// Read implements [io.Reader]. All reads from the connection are recorded up
// to a fixed size limit. After the limit is reached, all further reads are not
// recorded.
func (rc *recorderConn) Read(p []byte) (int, error) {
	return io.MultiReader(rc.recordingReader, rc.Conn).Read(p)
}

func (rc *recorderConn) reset() {
	const bufferSizeLimit = 1 << 20 // 1MB
	rc.buf.Reset()
	rc.recordingReader = &io.LimitedReader{
		R: io.TeeReader(rc.Conn, &rc.buf),
		N: bufferSizeLimit,
	}
}

// remainingBufferSize returns the remaining size, in bytes, of the current
// recorder buffer.
func (rc *recorderConn) remainingBufferSize() int {
	return int(rc.recordingReader.N)
}

// rewind sets the underlying [net.Conn] to a [bufferedConn] that reads from the
// recorded data first, then the connection itself. After calling rewind the
// recorder can be reused, but it should not be used once recording is no longer
// necessary, since it will continue to buffer reads in memory up to its buffer
// limit.
// The [net.Conn] returned will replay recorded data, but it will not record
// further reads, so it should be used once recording is no longer needed.
func (rc *recorderConn) rewind() net.Conn {
	rc.Conn = newBufferedConn(bytes.NewReader(bytes.Clone(rc.buf.Bytes())), rc.Conn)
	rc.reset()
	return rc.Conn
}

func newBufferedConn(recording io.Reader, conn net.Conn) *bufferedConn {
	return &bufferedConn{
		Conn:      conn,
		recording: recording,
	}
}

type bufferedConn struct {
	net.Conn
	recording io.Reader
}

func (bc *bufferedConn) Read(p []byte) (int, error) {
	return io.MultiReader(bc.recording, bc.Conn).Read(p)
}

// dialError is used to mark errors encountered when attempting to connect
// to the MySQL database.
type dialError struct{ inner error }

// newDialError returns a dialError.
func newDialError(inner error) *dialError {
	if inner == nil {
		return nil
	}
	return &dialError{inner: inner}
}

func (d *dialError) Error() string { return d.inner.Error() }
func (d *dialError) Unwrap() error { return d.inner }

// isDialError checks if the error is from failing to dial the MySQL database.
func isDialError(err error) bool {
	var target *dialError
	return errors.As(err, &target)
}
