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

package database

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/gravitational/trace"
	"github.com/jackc/pgconn"
	"github.com/jackc/pgerrcode"
	"github.com/sirupsen/logrus"
)

const (
	// A simple query to execute when running the Ping request
	selectOneQuery = "select 1;"
)

// PostgresPinger implements the DatabasePinger interface for the Postgres protocol
type PostgresPinger struct{}

// Ping connects to the database and issues a basic select statement to validate the connection.
func (p *PostgresPinger) Ping(ctx context.Context, ping PingParams) error {
	if err := ping.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	pgconnConfig, err := pgconn.ParseConfig(
		fmt.Sprintf("postgres://%s@%s:%d/%s",
			ping.Username,
			ping.Host,
			ping.Port,
			ping.Database,
		),
	)
	if err != nil {
		return trace.Wrap(err)
	}

	conn, err := pgconn.ConnectConfig(ctx, pgconnConfig)
	if err != nil {
		return trace.Wrap(err)
	}

	defer func() {
		if err := conn.Close(ctx); err != nil {
			logrus.WithError(err).Info("failed to close connection in PostgresPinger.Ping")
		}
	}()

	result, err := conn.Exec(ctx, selectOneQuery).ReadAll()
	if err != nil {
		return trace.Wrap(err)
	}

	if len(result) != 1 {
		return trace.BadParameter("unexpected length for result: %+v", result)
	}

	return nil
}

// IsConnectionRefusedError checks whether the error is of type invalid database user.
// This can happen when the user doesn't exist.
func (p *PostgresPinger) IsConnectionRefusedError(err error) bool {
	if err == nil {
		return false
	}

	return strings.Contains(err.Error(), "connection refused (SQLSTATE )")
}

// IsInvalidDatabaseUserError checks whether the error is of type invalid database user.
// This can happen when the user doesn't exist.
func (p *PostgresPinger) IsInvalidDatabaseUserError(err error) bool {
	var pge *pgconn.PgError
	if errors.As(err, &pge) {
		if pge.SQLState() == pgerrcode.InvalidAuthorizationSpecification {
			return true
		}
	}

	return false
}

// IsInvalidDatabaseNameError checks whether the error is of type invalid database name.
// This can happen when the database doesn't exist.
func (p *PostgresPinger) IsInvalidDatabaseNameError(err error) bool {
	var pge *pgconn.PgError
	if errors.As(err, &pge) {
		if pge.SQLState() == pgerrcode.InvalidCatalogName {
			return true
		}
	}

	return false
}
