package pgbk

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/gravitational/trace"
	"github.com/jackc/pgconn"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgtype"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/lib/backend"
)

// connectPostgres will open a single connection to the "postgres" database in
// the database cluster specified in poolConfig.
func connectPostgres(ctx context.Context, poolConfig *pgxpool.Config) (*pgx.Conn, error) {
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

func ensureDatabase(ctx context.Context, poolConfig *pgxpool.Config, log logrus.FieldLogger) {
	pgConn, err := connectPostgres(ctx, poolConfig)
	if err != nil {
		log.WithError(err).Warn("Failed to connect to the \"postgres\" database.")
		return
	}

	// this will error out if the encoding of template1 is not UTF8; in such
	// cases, the database creation should probably be done manually anyway
	createDB := fmt.Sprintf("CREATE DATABASE \"%v\" ENCODING UTF8", poolConfig.ConnConfig.Database)
	if _, err := pgConn.Exec(ctx, createDB); err != nil && !isCode(err, pgerrcode.DuplicateDatabase) {
		// CREATE will check permissions first and we may not have CREATEDB
		// privileges in more hardened setups; the subsequent connection
		// will fail immediately if we can't connect, anyway, so we can log
		// permission errors at debug level here.
		if isCode(err, pgerrcode.InsufficientPrivilege) {
			log.WithError(err).Debug("Error creating database.")
		} else {
			log.WithError(err).Warn("Error creating database.")
		}
	}
	if err := pgConn.Close(ctx); err != nil {
		log.WithError(err).Warn("Error closing connection to the \"postgres\" database.")
	}
}

// isCode checks if the passed error is a Postgres error with the given code.
func isCode(err error, code string) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == code
}

func toPgTime(t time.Time) pgtype.Timestamp {
	if t.IsZero() {
		return pgtype.Timestamp{Status: pgtype.Null}
	}
	return pgtype.Timestamp{Time: t, Status: pgtype.Present}
}

func newLease(i backend.Item) *backend.Lease {
	var lease backend.Lease
	if !i.Expires.IsZero() {
		lease.Key = i.Key
	}
	return &lease
}

func deleteExpired(ctx context.Context, tx pgx.Tx, key []byte) error {
	_, err := tx.Exec(ctx,
		"DELETE FROM kv WHERE key = $1 AND expires IS NOT NULL AND expires <= $2",
		key, time.Now().UTC())
	return trace.Wrap(err)
}
