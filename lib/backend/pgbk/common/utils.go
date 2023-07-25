// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package pgcommon

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/gravitational/trace"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/api/utils/retryutils"
)

// ConnectPostgres will open a single connection to the "postgres" database in
// the database cluster specified in poolConfig.
func ConnectPostgres(ctx context.Context, poolConfig *pgxpool.Config) (*pgx.Conn, error) {
	connConfig := poolConfig.ConnConfig.Copy()
	connConfig.Database = "postgres"

	if poolConfig.BeforeConnect != nil {
		if err := poolConfig.BeforeConnect(ctx, connConfig); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	conn, err := pgx.ConnectConfig(ctx, connConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if poolConfig.AfterConnect != nil {
		if err := poolConfig.AfterConnect(ctx, conn); err != nil {
			conn.Close(ctx)
			return nil, trace.Wrap(err)
		}
	}

	return conn, nil
}

// TryEnsureDatabase will connect to the "postgres" database and attempt to
// create the database named in the pool's configuration.
func TryEnsureDatabase(ctx context.Context, poolConfig *pgxpool.Config, log logrus.FieldLogger) {
	pgConn, err := ConnectPostgres(ctx, poolConfig)
	if err != nil {
		log.WithError(err).Warn("Failed to connect to the \"postgres\" database.")
		return
	}

	// the database name is not a string but an identifier, so we can't use query parameters for it
	createDB := fmt.Sprintf("CREATE DATABASE \"%v\" TEMPLATE template0 ENCODING UTF8 LC_COLLATE 'C' LC_CTYPE 'C'", poolConfig.ConnConfig.Database)
	if _, err := pgConn.Exec(ctx, createDB, pgx.QueryExecModeExec); err != nil && !IsCode(err, pgerrcode.DuplicateDatabase) {
		// CREATE will check permissions first and we may not have CREATEDB
		// privileges in more hardened setups; the subsequent connection
		// will fail immediately if we can't connect, anyway, so we can log
		// permission errors at debug level here.
		if IsCode(err, pgerrcode.InsufficientPrivilege) {
			log.WithError(err).Debug("Error creating database due to insufficient privileges.")
		} else {
			log.WithError(err).Warn("Error creating database.")
		}
	}
	if err := pgConn.Close(ctx); err != nil {
		log.WithError(err).Warn("Error closing connection to the \"postgres\" database.")
	}
}

// Retry runs the closure potentially more than once, retrying quickly on
// serialization or deadlock errors, and backing off more on other retryable
// errors. It will not retry on network errors or other ambiguous errors after
// any data has been sent.
func Retry[T any](ctx context.Context, log logrus.FieldLogger, f func() (T, error)) (T, error) {
	const idempotent = false
	v, err := retry(ctx, log, idempotent, f)
	return v, trace.Wrap(err)
}

// RetryIdempotent runs the closure potentially more than once, retrying quickly
// on serialization or deadlock errors, and backing off more on other errors. It
// assumes that f is idempotent, so it will retry even in ambiguous situations.
func RetryIdempotent[T any](ctx context.Context, log logrus.FieldLogger, f func() (T, error)) (T, error) {
	const idempotent = true
	v, err := retry(ctx, log, idempotent, f)
	return v, trace.Wrap(err)
}

func retry[T any](ctx context.Context, log logrus.FieldLogger, isIdempotent bool, f func() (T, error)) (T, error) {
	var v T
	var err error
	v, err = f()
	if err == nil {
		return v, nil
	}

	if ctx.Err() != nil {
		var zeroT T
		return zeroT, trace.Wrap(ctx.Err())
	}

	retry, retryErr := retryutils.NewLinear(retryutils.LinearConfig{
		First:  0,
		Step:   100 * time.Millisecond,
		Max:    750 * time.Millisecond,
		Jitter: retryutils.NewHalfJitter(),
	})
	if retryErr != nil {
		var zeroT T
		return zeroT, trace.Wrap(retryErr)
	}

	for i := 1; i < 10; i++ {
		var pgErr *pgconn.PgError
		_ = errors.As(err, &pgErr)

		if pgErr != nil && (pgErr.Code == pgerrcode.SerializationFailure || pgErr.Code == pgerrcode.DeadlockDetected) {
			log.WithError(err).
				WithField("attempt", i).
				Debug("Operation failed due to conflicts, retrying quickly.")
			retry.Reset()
			// the very first attempt gets instant retry on serialization failure
			if i > 1 {
				retry.Inc()
			}
		} else if (isIdempotent && pgErr == nil) || pgconn.SafeToRetry(err) {
			log.WithError(err).
				WithField("attempt", i).
				Debug("Operation failed, retrying.")
			retry.Inc()
		} else {
			// we either know we shouldn't retry (on a database error), or we
			// are not in idempotent mode and we don't know if we should retry
			// (ambiguous error after sending some data)
			var zeroT T
			return zeroT, trace.Wrap(err)
		}

		select {
		case <-retry.After():
		case <-ctx.Done():
			var zeroT T
			return zeroT, trace.Wrap(ctx.Err())
		}

		v, err = f()
		if err == nil {
			return v, nil
		}

		if ctx.Err() != nil {
			var zeroT T
			return zeroT, trace.Wrap(ctx.Err())
		}
	}

	var zeroT T
	return zeroT, trace.LimitExceeded("too many retries, last error: %v", err)
}

// RetryTx runs a closure like [Retry] or [RetryIdempotent], wrapped in
// [pgx.BeginTxFunc].
func RetryTx(
	ctx context.Context,
	log logrus.FieldLogger,
	db interface {
		BeginTx(context.Context, pgx.TxOptions) (pgx.Tx, error)
	},
	txOptions pgx.TxOptions,
	isIdempotent bool,
	f func(tx pgx.Tx) error,
) error {
	_, err := retry(ctx, log, isIdempotent, func() (struct{}, error) {
		return struct{}{}, trace.Wrap(pgx.BeginTxFunc(ctx, db, txOptions, f))
	})
	return trace.Wrap(err)
}

// IsCode checks if the passed error is a Postgres error with the given code.
func IsCode(err error, code string) bool {
	var pgErr *pgconn.PgError
	_ = errors.As(err, &pgErr)
	return pgErr != nil && pgErr.Code == code
}

// SetupAndMigrate sets up the database schema, applying the migrations in the
// schemas slice in order, starting from the first non-applied one. tableName is
// the name of a table used to hold schema version numbers.
func SetupAndMigrate(
	ctx context.Context,
	log logrus.FieldLogger,
	db interface {
		BeginTx(context.Context, pgx.TxOptions) (pgx.Tx, error)
		Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	},
	tableName string,
	schemas []string,
) error {
	var version int32
	var migrateErr error

	// this is split off from the rest because we might not have permissions to
	// CREATE TABLE, which is checked even if the table exists
	if _, err := RetryIdempotent(ctx, log, func() (struct{}, error) {
		_, err := db.Exec(ctx,
			fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %v (
				version integer PRIMARY KEY CHECK (version > 0),
				created timestamptz NOT NULL DEFAULT now()
			)`, tableName), pgx.QueryExecModeExec,
		)
		return struct{}{}, trace.Wrap(err)
	}); err != nil {
		// the very first SELECT in the next transaction will fail, we don't
		// need anything higher than debug here
		log.WithError(err).Debugf("Failed to confirm the existence of the %v table.", tableName)
	}

	const idempotent = true
	if err := RetryTx(ctx, log, db, pgx.TxOptions{
		IsoLevel:   pgx.Serializable,
		AccessMode: pgx.ReadWrite,
	}, idempotent, func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx,
			fmt.Sprintf("SELECT COALESCE(max(version), 0) FROM %v", tableName),
			pgx.QueryExecModeExec,
		).Scan(&version); err != nil {
			return trace.Wrap(err)
		}

		if int(version) > len(schemas) {
			migrateErr = trace.BadParameter("unsupported schema version %v", version)
			// the transaction succeeded, the error is outside of the transaction
			return nil
		}

		if int(version) == len(schemas) {
			return nil
		}

		for _, s := range schemas[version:] {
			if _, err := tx.Exec(ctx, s, pgx.QueryExecModeExec); err != nil {
				return trace.Wrap(err)
			}
		}

		if _, err := tx.Exec(ctx,
			fmt.Sprintf("INSERT INTO %v (version) VALUES ($1)", tableName),
			pgx.QueryExecModeExec, len(schemas),
		); err != nil {
			return trace.Wrap(err)
		}

		return nil
	}); err != nil {
		return trace.Wrap(err)
	}

	if migrateErr != nil {
		return trace.Wrap(migrateErr)
	}

	if int(version) != len(schemas) {
		log.WithFields(logrus.Fields{
			"previous_version": version,
			"current_version":  len(schemas),
		}).Info("Migrated database schema.")
	}

	return nil
}
