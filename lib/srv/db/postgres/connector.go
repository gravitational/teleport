// Teleport
// Copyright (C) 2024 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package postgres

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/gravitational/trace"
	"github.com/jackc/pgconn"
	"github.com/jackc/pgproto3/v2"
	"github.com/jackc/pgx/v4"

	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	gcputils "github.com/gravitational/teleport/api/utils/gcp"
	libcloud "github.com/gravitational/teleport/lib/cloud"
	"github.com/gravitational/teleport/lib/srv/db/cloud"
	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/gravitational/teleport/lib/srv/db/endpoints"
	discoverycommon "github.com/gravitational/teleport/lib/srv/discovery/common"
)

type connector struct {
	auth       common.Auth
	gcpClients libcloud.GCPClients
	log        *slog.Logger

	certExpiry    time.Time
	database      types.Database
	databaseUser  string
	databaseName  string
	startupParams map[string]string
}

// NewEndpointsResolver returns a health check target endpoint resolver.
func NewEndpointsResolver(_ context.Context, db types.Database, config endpoints.ResolverBuilderConfig) (endpoints.Resolver, error) {
	// special handling for AlloyDB
	if db.GetType() == types.DatabaseTypeAlloyDB {
		return newAlloyDBEndpointsResolver(db, config.GCPClients)
	}

	return newEndpointsResolver(db.GetURI())
}

func newAlloyDBEndpointsResolver(db types.Database, clients endpoints.GCPClients) (endpoints.Resolver, error) {
	serverPort := strconv.Itoa(alloyDBServerProxyPort)

	info, err := gcputils.ParseAlloyDBConnectionURI(db.GetURI())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if db.GetGCP().AlloyDB.EndpointOverride != "" {
		addr := net.JoinHostPort(db.GetGCP().AlloyDB.EndpointOverride, serverPort)
		return endpoints.ResolverFn(func(context.Context) ([]string, error) {
			return []string{addr}, nil
		}), nil
	}

	resolveFun := endpoints.ResolverFn(func(ctx context.Context) ([]string, error) {
		adminClient, err := clients.GetGCPAlloyDBClient(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		addr, err := adminClient.GetEndpointAddress(ctx, *info, db.GetGCP().AlloyDB.EndpointType)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return []string{net.JoinHostPort(addr, serverPort)}, nil
	})

	return resolveFun, nil
}

func newEndpointsResolver(uri string) (endpoints.Resolver, error) {
	config, err := pgconn.ParseConfig(fmt.Sprintf("postgres://%s", uri))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	addrs := make([]string, 0, len(config.Fallbacks)+1)
	hostPort := net.JoinHostPort(config.Host, strconv.Itoa(int(config.Port)))
	addrs = append(addrs, hostPort)
	for _, fb := range config.Fallbacks {
		hostPort := net.JoinHostPort(fb.Host, strconv.Itoa(int(fb.Port)))
		// pgconn duplicates the host/port in its fallbacks for some reason, so
		// we de-duplicate and preserve the fallback order
		if !slices.Contains(addrs, hostPort) {
			addrs = append(addrs, hostPort)
		}
	}
	return endpoints.ResolverFn(func(context.Context) ([]string, error) {
		return addrs, nil
	}), nil
}

// alloyDBServerProxyPort is the non-configurable port on which the AlloyDB server-side proxy is listening.
const alloyDBServerProxyPort = 5433

func (c *connector) getConnectConfig(ctx context.Context) (*pgconn.Config, error) {
	// The driver requires the config to be built by parsing the connection
	// string so parse the basic template and then fill in the rest of
	// parameters such as TLS configuration.
	dbConfig := fmt.Sprintf("postgres://%s", c.database.GetURI())

	// AlloyDB URI is not parseable by pgconn.ParseConfig. Construct a minimal config for parsing purposes.
	// We will replace it with real hostname later.
	if c.database.GetType() == types.DatabaseTypeAlloyDB {
		dbConfig = fmt.Sprintf("postgres://placeholder:%v", alloyDBServerProxyPort)
	}

	config, err := pgconn.ParseConfig(dbConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// TLS config will use client certificate for an onprem database or
	// will contain RDS root certificate for RDS/Aurora.
	config.TLSConfig, err = c.auth.GetTLSConfig(ctx, c.certExpiry, c.database, c.databaseUser)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	config.User = c.databaseUser
	config.Database = c.databaseName
	// Pgconn adds fallbacks to retry connection without TLS if the TLS
	// attempt fails. Reset the fallbacks to avoid retries, otherwise
	// it's impossible to debug TLS connection errors.
	config.Fallbacks = nil
	// Set startup parameters that the client sent us.
	config.RuntimeParams = c.startupParams
	// AWS RDS/Aurora and GCP Cloud SQL use IAM authentication so request an
	// auth token and use it as a password.
	switch c.database.GetType() {
	case types.DatabaseTypeRDS, types.DatabaseTypeRDSProxy:
		config.Password, err = c.auth.GetRDSAuthToken(ctx, c.database, c.databaseUser)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	case types.DatabaseTypeRedshift:
		config.User, config.Password, err = c.auth.GetRedshiftAuthToken(ctx, c.database, c.databaseUser, c.databaseName)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	case types.DatabaseTypeRedshiftServerless:
		config.User, config.Password, err = c.auth.GetRedshiftServerlessAuthToken(ctx, c.database, c.databaseUser, c.databaseName)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	case types.DatabaseTypeCloudSQL:
		config.Password, err = c.auth.GetCloudSQLAuthToken(ctx, c.databaseUser)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		// Get the client once for subsequent calls (it acquires a read lock).
		gcpClient, err := c.gcpClients.GetGCPSQLAdminClient(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		// Detect whether the instance is set to require SSL.
		// Fallback to not requiring SSL for access denied errors.
		requireSSL, err := cloud.GetGCPRequireSSL(ctx, c.database, gcpClient)
		if err != nil && !trace.IsAccessDenied(err) {
			return nil, trace.Wrap(err)
		}
		// Create ephemeral certificate and append to TLS config when
		// the instance requires SSL.
		if requireSSL {
			err = cloud.AppendGCPClientCert(ctx, &cloud.AppendGCPClientCertRequest{
				GCPClient:   gcpClient,
				GenerateKey: c.auth.GenerateDatabaseClientKey,
				Expiry:      c.certExpiry,
				Database:    c.database,
				TLSConfig:   config.TLSConfig,
			})
			if err != nil {
				return nil, trace.Wrap(err)
			}
		}
	case types.DatabaseTypeAlloyDB:
		token, err := c.auth.GetAlloyDBAuthToken(ctx, c.databaseUser)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		adminClient, err := c.gcpClients.GetGCPAlloyDBClient(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		pkey, err := c.auth.GenerateDatabaseClientKey(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		info, err := gcputils.ParseAlloyDBConnectionURI(c.database.GetURI())
		if err != nil {
			return nil, trace.Wrap(err)
		}

		clientCert, rootCA, err := adminClient.GenerateClientCertificate(ctx, *info, c.certExpiry, pkey)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		// construct custom TLS config suitable for AlloyDB
		rootCAPool := x509.NewCertPool()
		ok := rootCAPool.AppendCertsFromPEM([]byte(rootCA))
		if !ok {
			return nil, trace.BadParameter("failed to parse root certificate")
		}

		dbOpts := c.database.GetGCP().AlloyDB

		if dbOpts.EndpointOverride != "" {
			// respect override
			config.Host = dbOpts.EndpointOverride
			c.log.DebugContext(ctx, "Using database endpoint override", "address", config.Host)
		} else {
			// resolve the database address of particular type (public/private/PSC).
			config.Host, err = adminClient.GetEndpointAddress(ctx, *info, dbOpts.EndpointType)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			c.log.DebugContext(ctx, "Resolved database endpoint address", "address", config.Host)
		}

		tlsConfig := &tls.Config{
			Certificates: []tls.Certificate{*clientCert},
			RootCAs:      rootCAPool,
			ServerName:   config.Host,
			MinVersion:   tls.VersionTLS13,
			// copy from original config; only InsecureSkipVerify for now, we may want to expand this in the future.
			InsecureSkipVerify: config.TLSConfig.InsecureSkipVerify,
		}

		// remove TLS config from pg client config; DialFunc will handle TLS on its own.
		config.TLSConfig = nil
		config.DialFunc = func(ctx context.Context, network, addr string) (net.Conn, error) {
			conn, err := net.DialTimeout(network, addr, defaults.DefaultIOTimeout)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			tlsConn := tls.Client(conn, tlsConfig)
			err = tlsConn.HandshakeContext(ctx)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			err = metadataExchangeAlloyDB(token, tlsConn)
			if err != nil {
				_ = tlsConn.Close() // best effort

				c.log.WarnContext(ctx, "Metadata exchange failed", "err", err)

				// special case for "IAM check failed" error
				if strings.Contains(err.Error(), `IAM check failed`) {
					return nil, trace.AccessDenied(`Could not connect to database:

  %v

Make sure that AlloyDB user %q exists and has the following permissions:
- alloydb.instances.connect
- alloydb.users.login
- serviceusage.services.use

You can create a custom role with these permissions, or grant the following roles:
- Cloud AlloyDB Database User
- Cloud AlloyDB Client
- Service Usage Consumer

Note that IAM changes may take a few minutes to propagate.`, err, c.databaseUser)
				}
				return nil, trace.Wrap(err)
			}
			return tlsConn, nil
		}
	case types.DatabaseTypeAzure:
		config.Password, err = c.auth.GetAzureAccessToken(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		config.User = discoverycommon.MakeAzureDatabaseLoginUsername(c.database, config.User)
	}
	return config, nil
}

// pgxConnect connects to the database using pgx driver which is higher-level
// than pgconn and is easier to use for executing queries.
func (c *connector) pgxConnect(ctx context.Context) (*pgx.Conn, error) {
	config, err := c.getConnectConfig(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	pgxConf, err := pgx.ParseConfig("")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	pgxConf.Config = *config
	c.log.DebugContext(ctx, "Connecting to database", "db_name", config.Database, "db_user", config.User, "host", config.Host)
	return pgx.ConnectConfig(ctx, pgxConf)
}

func (c *connector) sendCancelRequest(ctx context.Context, cancelReq *pgproto3.CancelRequest) error {
	// We can't use pgconn in this case because it always sends a startup message.
	config, err := c.getConnectConfig(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	address := net.JoinHostPort(config.Host, strconv.Itoa(int(config.Port)))

	// allow config.DialFunc override from config
	dialFunc := config.DialFunc
	if dialFunc == nil {
		dialer := net.Dialer{Timeout: defaults.DefaultIOTimeout}
		dialFunc = dialer.DialContext
	}

	c.log.DebugContext(ctx, "Dialing database to cancel request", "address", address)
	conn, err := dialFunc(ctx, "tcp", address)
	if err != nil {
		return trace.Wrap(err)
	}

	// dialFunc may return a TLS connection, in which case the binary PostgreSQL protocol
	// is already tunneled over TLS. Otherwise, negotiate a TLS upgrade before proceeding.
	tlsConn, ok := conn.(*tls.Conn)
	if !ok {
		if config.TLSConfig == nil {
			return trace.BadParameter("TLSConfig missing for non-TLS connection %v (this is a bug)", conn)
		}
		tlsConn, err = startPGWireTLS(conn, config.TLSConfig)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	frontend := pgproto3.NewFrontend(pgproto3.NewChunkReader(tlsConn), tlsConn)
	if err = frontend.Send(cancelReq); err != nil {
		return trace.Wrap(err)
	}

	response := make([]byte, 1)
	if _, err := tlsConn.Read(response); !errors.Is(err, io.EOF) {
		// server should close the connection after receiving cancel request.
		return trace.Wrap(err)
	}

	c.log.DebugContext(ctx, "Cancel request sent successfully")
	return nil
}

// startPGWireTLS is a helper func that upgrades upstream connection to TLS.
// copied from github.com/jackc/pgconn.startTLS.
func startPGWireTLS(conn net.Conn, tlsConfig *tls.Config) (*tls.Conn, error) {
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

// withDefaultDatabase returns a copy of connector with databaseName switched to the default database, if one is available.
func (c *connector) withDefaultDatabase() *connector {
	copied := *c
	if c.database.GetAdminUser().DefaultDatabase != "" {
		copied.databaseName = c.database.GetAdminUser().DefaultDatabase
	}
	return &copied
}

// connectAsAdmin connect to the database from db route as admin user.
// If useDefaultDatabase is true and a default database is configured for the admin user, it will be used instead.
func (c *connector) connectAsAdmin(ctx context.Context) (*pgx.Conn, error) {
	// make a copy to override the database user as well as to clear potential startup params.
	copied := *c
	copied.databaseUser = c.database.GetAdminUser().Name
	copied.startupParams = make(map[string]string)

	conn, err := copied.pgxConnect(ctx)
	return conn, trace.Wrap(err)
}
