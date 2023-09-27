// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package db

import (
	"context"
	"fmt"

	"github.com/gravitational/trace"
	"github.com/jackc/pgconn"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/benchmark"
	"github.com/gravitational/teleport/lib/client"
)

const (
	// postgresPingQuery is a noop query used to verify the connection is
	// working.
	postgresPingQuery = "SELECT 1;"
)

// PostgresBenchmark is a benchmark suite that connects to a PostgreSQL database
// (directly or through Teleport) and issues a ping query.
type PostgresBenchmark struct {
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
	// connConfig is a configuration to directly connect to a PostgreSQL
	// database (without Teleport).
	connConfig *pgconn.Config
}

func (p *PostgresBenchmark) CheckAndSetDefaults() error {
	if p.DBService == "" {
		return trace.BadParameter("database or direct database URI must be provided")
	}

	if directConfig, err := pgconn.ParseConfig(p.DBService); err == nil {
		p.connConfig = directConfig
	}

	if p.connConfig == nil && (p.DBUser == "" || p.DBName == "") {
		return trace.BadParameter("must provide and database name and user")
	}

	return nil
}

// BenchBuilder returns a WorkloadFunc for the given benchmark suite.
func (p *PostgresBenchmark) BenchBuilder(ctx context.Context, tc *client.TeleportClient) (benchmark.WorkloadFunc, error) {
	if err := p.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	config, err := p.buildConnectionConfig(ctx, tc)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return func(ctx context.Context) error {
		conn, err := pgconn.ConnectConfig(ctx, config)
		if err != nil {
			return trace.Wrap(err)
		}
		defer conn.Close(ctx)

		_, err = conn.Exec(ctx, postgresPingQuery).ReadAll()
		return trace.Wrap(err)
	}, nil
}

// buildConnectionConfig generates a connect configuration for the database.
func (p *PostgresBenchmark) buildConnectionConfig(ctx context.Context, tc *client.TeleportClient) (*pgconn.Config, error) {
	if p.connConfig != nil {
		return p.connConfig, nil
	}

	database, err := getDatabase(ctx, tc, p.DBService, types.DatabaseProtocolPostgreSQL)
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

	config, err := pgconn.ParseConfig(fmt.Sprintf("postgres://%v/?sslmode=verify-full", lp.GetAddr()))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	config.User = p.DBUser
	config.Database = p.DBName
	return config, nil
}
