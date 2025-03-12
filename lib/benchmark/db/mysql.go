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

package db

import (
	"context"
	"net/url"
	"strings"

	mysqlclient "github.com/go-mysql-org/go-mysql/client"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/benchmark"
	"github.com/gravitational/teleport/lib/client"
)

const (
	// mysqlProtocolScheme is the URL scheme for MySQL databases.
	mysqlProtocolScheme = "mysql"
)

// MySQLBenchmark is a benchmark suite that connects to a MySQL database
// (directly or through Teleport) and issues a ping command.
type MySQLBenchmark struct {
	// DBService database service name of the target database. Can be a Teleport
	// database or a direct URI.
	DBService string
	// DBUser database user used to connect to the target database.
	DBUser string
	// DBName database name where the benchmark queries are going to be
	// executed.
	DBName string
	// InsecureSkipVerify bypasses verification of TLS certificate.
	InsecureSkipVerify bool
	// connOptions are the MySQL connection options.
	connOptions *mysqlConnOptions
}

func (p *MySQLBenchmark) CheckAndSetDefaults() error {
	if p.DBService == "" {
		return trace.BadParameter("database or direct database URI must be provided")
	}

	if strings.Contains(p.DBService, "://") {
		var err error
		p.connOptions, err = parseMySQLConnString(p.DBService)
		return trace.Wrap(err)
	}

	if p.DBUser == "" {
		return trace.BadParameter("must provide database user")
	}

	return nil
}

// BenchBuilder returns a WorkloadFunc for the given benchmark suite.
func (p *MySQLBenchmark) BenchBuilder(ctx context.Context, tc *client.TeleportClient) (benchmark.WorkloadFunc, error) {
	if err := p.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	config, err := p.buildConnectionConfig(ctx, tc)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return func(ctx context.Context) error {
		conn, err := mysqlclient.Connect(config.host, config.username, config.password, config.database)
		if err != nil {
			return trace.Wrap(err)
		}

		defer conn.Quit()
		return trace.Wrap(conn.Ping())
	}, nil
}

// mysqlConnOptions defines connection configuration for MySQL database.
type mysqlConnOptions struct {
	host     string
	username string
	password string
	database string
}

// buildConnectionConfig generates a connect configuration for the database.
func (p *MySQLBenchmark) buildConnectionConfig(ctx context.Context, tc *client.TeleportClient) (*mysqlConnOptions, error) {
	if p.connOptions != nil {
		return p.connOptions, nil
	}

	database, err := getDatabase(ctx, tc, p.DBService, types.DatabaseProtocolMySQL)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	dbCert, err := retrieveDatabaseCertificates(ctx, tc, database, p.DBUser, p.DBName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	lp, err := startLocalProxy(ctx, p.InsecureSkipVerify, tc, database.GetProtocol(), dbCert)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &mysqlConnOptions{
		host:     lp.GetAddr(),
		username: p.DBUser,
		database: p.DBName,
	}, nil
}

// parseMySQLConnString parses a MySQL URI into connection options.
// URI Format:
// [scheme://][user[:[password]]@]host[:port][/schema][?attribute1=value1&attribute2=value2...
//
// Reference: https://dev.mysql.com/doc/refman/8.0/en/connecting-using-uri-or-key-value-pairs.html
func parseMySQLConnString(connString string) (*mysqlConnOptions, error) {
	parsed, err := url.Parse(connString)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if parsed.Scheme != mysqlProtocolScheme {
		return nil, trace.BadParameter("scheme %q not supported. Please use %q", parsed.Scheme, mysqlProtocolScheme)
	}

	password, _ := parsed.User.Password()
	return &mysqlConnOptions{
		host:     parsed.Host,
		username: parsed.User.Username(),
		password: password,
		database: strings.TrimPrefix(parsed.Path, "/"),
	}, nil
}
