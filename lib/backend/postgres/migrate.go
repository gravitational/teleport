/*
Copyright 2018-2022 Gravitational, Inc.

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

package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/gravitational/trace"
)

// migrate the database to the most recent schema version.
func (db *pgDB) migrate(ctx context.Context) error {
	tx := db.begin(ctx, db.readWriteOpts)

	tx.setTxIsolationLevel(serializableTxIsolationLevel)
	if !tx.migrateTableExists() {
		tx.createMigrateTable()
	}

	dbVersion := tx.getSchemaVersion()
	if tx.err != nil || dbVersion == schemaVersion {
		return tx.err
	}

	// Can't migrate backwards.
	if dbVersion > schemaVersion {
		tx.rollback(trace.BadParameter("cannot migrate from database schema version %v to %v", dbVersion, schemaVersion))
		return tx.err
	}

	// Prepare statement to insert migration records for each new version.
	const query = `INSERT INTO migrate (version) VALUES ($1)`
	insertVersionStmt, err := tx.sqlTx.PrepareContext(tx.ctx, query)
	if tx.rollback(err) {
		return tx.err
	}
	defer insertVersionStmt.Close()

	// Migrate from dbVersion to schemaVersion.
	for version := dbVersion + 1; version <= schemaVersion; version++ {
		_, err = tx.sqlTx.ExecContext(tx.ctx, getMigration(version))
		if tx.rollback(err) {
			return tx.err
		}
		_, err = insertVersionStmt.ExecContext(tx.ctx, version)
		if tx.rollback(err) {
			return tx.err
		}
	}

	return tx.Commit()
}

// setTxIsolationLevel sets the current database's target isolation level to targetLevel.
func (tx *pgTx) setTxIsolationLevel(targetLevel string) {
	if tx.err != nil {
		return
	}

	// Query for database's default isolation level.
	const levelQuery = `SELECT setting FROM pg_settings WHERE name='default_transaction_isolation'`
	var level string
	err := tx.sqlTx.QueryRowContext(tx.ctx, levelQuery).Scan(&level)
	if tx.rollback(err) {
		return
	}

	// Return if we're already set to serializable.
	if level == targetLevel {
		return
	}

	// Query for the database name (needed for the next alter db call). We
	// could've passed the name in from the backend's configuration, but we
	// only need the name once when initializing a new db.
	var dbName string
	err = tx.sqlTx.QueryRowContext(tx.ctx, "SELECT current_catalog").Scan(&dbName)
	if tx.rollback(err) {
		return
	}

	// Change default isolation level for the database.
	const alterQuery = "ALTER DATABASE %s SET DEFAULT_TRANSACTION_ISOLATION TO '%s'"
	_, err = tx.sqlTx.ExecContext(tx.ctx, fmt.Sprintf(alterQuery, dbName, targetLevel))
	if tx.rollback(err) {
		return
	}

	// Change isolation level for the current connection. Changing the database
	// level doesn't change the level for the current connection. This connection
	// is part of a pool, so it may be reused again. This connection should be
	// the only one in the pool at this point.
	const setQuery = "SET DEFAULT_TRANSACTION_ISOLATION = '%s'"
	_, err = tx.sqlTx.ExecContext(tx.ctx, fmt.Sprintf(setQuery, targetLevel))
	tx.rollback(err)
}

// migrateTableExists returns true if the migrate table exists.
// It returns false if the transaction in in an error state.
func (tx *pgTx) migrateTableExists() bool {
	if tx.err != nil {
		return false
	}

	// Using EXISTS always returns a boolean result.
	const query = `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.tables
			WHERE table_schema='public'
			  AND table_name='migrate'
		)`

	var exists bool
	err := tx.sqlTx.QueryRowContext(tx.ctx, query).Scan(&exists)
	if tx.rollback(err) {
		return false
	}
	return exists
}

// createMigrateTable creates a new migrate table.
func (tx *pgTx) createMigrateTable() {
	if tx.err != nil {
		return
	}

	const query = `
		CREATE TABLE migrate (
			version INT PRIMARY KEY NOT NULL,
			created TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`

	_, err := tx.sqlTx.ExecContext(tx.ctx, query)
	tx.rollback(err)
}

// getSchemaVersion returns the database's version of the schema.
// Zero is returned if there are no version records.
func (tx *pgTx) getSchemaVersion() int {
	if tx.err != nil {
		return 0
	}

	const query = `SELECT version FROM migrate ORDER BY version DESC LIMIT 1`
	var version int
	err := tx.sqlTx.QueryRowContext(tx.ctx, query).Scan(&version)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		tx.rollback(err)
		return 0
	}
	return version
}

const serializableTxIsolationLevel = "serializable"
