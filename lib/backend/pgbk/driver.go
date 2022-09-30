// Copyright 2022 Gravitational, Inc
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

package pgbk

import "github.com/gravitational/trace"

const (
	// MaxDatabaseNameLength is the maximum PostgreSQL identifier length.
	// https://www.postgresql.org/docs/14/sql-syntax-lexical.html#SQL-SYNTAX-IDENTIFIERS
	MaxDatabaseNameLength = 63
)

const (
	// BackendName is the name of this backend.
	BackendName = "postgres"
	// AlternativeName is another name of this backend.
	AlternativeName = "cockroachdb"
)

// GetName returns BackendName (postgres).
func GetName() string {
	return BackendName
}

// ValidateDatabaseName returns true when name contains only alphanumeric and/or
// underscore/dollar characters, the first character is not a digit, and the
// name's length is less than MaxDatabaseNameLength (63 bytes).
func ValidateDatabaseName(name string) error {
	if MaxDatabaseNameLength <= len(name) {
		return trace.BadParameter("invalid PostgreSQL database name, length exceeds %d bytes. See https://www.postgresql.org/docs/14/sql-syntax-lexical.html.", MaxDatabaseNameLength)
	}
	for i, r := range name {
		switch {
		case 'A' <= r && r <= 'Z', 'a' <= r && r <= 'z', r == '_':
		case i > 0 && (r == '$' || '0' <= r && r <= '9'):
		default:
			return trace.BadParameter("invalid PostgreSQL database name: %v. See https://www.postgresql.org/docs/14/sql-syntax-lexical.html.", name)
		}
	}
	return nil
}

