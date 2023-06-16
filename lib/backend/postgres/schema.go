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

import "fmt"

// schemaVersion defines the current schema version.
// Increment this value when adding a new migration.
const schemaVersion = 1

// getMigration returns migration SQL for a schema version.
func getMigration(version int) string {
	switch version {
	case 1:
		return migrateV1
		// case 2:
		//   return migrateV2
	}
	panic(fmt.Sprintf("migration version not implemented: %v", version))
}

// migrateV1 is the baseline schema.
//
// Keys are stored as BYTEA to avoid collation ordering.
// When debugging, convert the key to a readable value using:
//   SELECT encode(key, 'escape') FROM lease;
const migrateV1 = `
	CREATE TABLE item (
		key BYTEA NOT NULL,
		id BIGINT NOT NULL,
		value BYTEA NOT NULL,
		CONSTRAINT item_pk PRIMARY KEY (key,id)
	);

	CREATE TABLE lease (
		key BYTEA NOT NULL,
		id BIGINT NOT NULL,
		expires TIMESTAMPTZ,
		CONSTRAINT lease_pk PRIMARY KEY (key)
	);
	CREATE INDEX lease_expires ON lease (expires);

	CREATE TABLE event (
		eventid BIGINT NOT NULL,
		created TIMESTAMPTZ NOT NULL,
		key BYTEA NOT NULL,
		id BIGINT NOT NULL,
		type SMALLINT NOT NULL,
		CONSTRAINT event_pk PRIMARY KEY (eventid)
	);
`
