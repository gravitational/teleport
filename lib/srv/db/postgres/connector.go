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
	"fmt"
	"log/slog"
	"time"

	"github.com/gravitational/trace"
	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v4"

	"github.com/gravitational/teleport/api/types"
	libcloud "github.com/gravitational/teleport/lib/cloud"
	"github.com/gravitational/teleport/lib/srv/db/cloud"
	"github.com/gravitational/teleport/lib/srv/db/common"
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

func (c *connector) getConnectConfig(ctx context.Context) (*pgconn.Config, error) {
	// The driver requires the config to be built by parsing the connection
	// string so parse the basic template and then fill in the rest of
	// parameters such as TLS configuration.
	config, err := pgconn.ParseConfig(fmt.Sprintf("postgres://%s", c.database.GetURI()))
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
