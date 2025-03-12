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

package database

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/gravitational/trace"
	"github.com/jackc/pgconn"
	"github.com/jackc/pgerrcode"

	"github.com/gravitational/teleport/lib/defaults"
)

const (
	// A simple query to execute when running the Ping request
	selectOneQuery = "select 1;"
)

// PostgresPinger implements the DatabasePinger interface for the Postgres protocol
type PostgresPinger struct{}

// Ping connects to the database and issues a basic select statement to validate the connection.
func (p *PostgresPinger) Ping(ctx context.Context, params PingParams) error {
	if err := params.CheckAndSetDefaults(defaults.ProtocolPostgres); err != nil {
		return trace.Wrap(err)
	}

	pgconnConfig, err := pgconn.ParseConfig(
		fmt.Sprintf("postgres://%s@%s:%d/%s",
			params.Username,
			params.Host,
			params.Port,
			params.DatabaseName,
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
			slog.InfoContext(context.Background(), "failed to close connection in PostgresPinger.Ping", "error", err)
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

// IsConnectionRefusedError checks whether the error is of type connection refused.
func (p *PostgresPinger) IsConnectionRefusedError(err error) bool {
	if err == nil {
		return false
	}

	return strings.Contains(err.Error(), "connection refused (SQLSTATE")
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
