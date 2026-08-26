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

package postgres

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/client/db/profile"
)

// TestConnString verifies creating Postgres connection string from profile.
func TestConnString(t *testing.T) {
	const (
		host     = "localhost"
		port     = 5432
		caPath   = "/tmp/ca"
		certPath = "/tmp/cert"
		keyPath  = "/tmp/key"
	)

	tests := []struct {
		name     string
		user     string
		database string
		insecure bool
		out      string
	}{
		{
			name: "default settings",
			out:  "postgres://localhost:5432?sslrootcert=/tmp/ca&sslcert=/tmp/cert&sslkey=/tmp/key&sslmode=verify-full",
		},
		{
			name:     "insecure",
			insecure: true,
			out:      "postgres://localhost:5432?sslrootcert=/tmp/ca&sslcert=/tmp/cert&sslkey=/tmp/key&sslmode=verify-ca",
		},
		{
			name: "user set",
			user: "alice",
			out:  "postgres://alice@localhost:5432?sslrootcert=/tmp/ca&sslcert=/tmp/cert&sslkey=/tmp/key&sslmode=verify-full",
		},
		{
			name: "user with special characters",
			user: "postgres@google-project-id.iam",
			out:  "postgres://postgres%40google-project-id.iam@localhost:5432?sslrootcert=/tmp/ca&sslcert=/tmp/cert&sslkey=/tmp/key&sslmode=verify-full",
		},
		{
			name:     "database set",
			database: "test",
			out:      "postgres://localhost:5432/test?sslrootcert=/tmp/ca&sslcert=/tmp/cert&sslkey=/tmp/key&sslmode=verify-full",
		},
		{
			name:     "user and database set",
			user:     "alice",
			database: "test",
			out:      "postgres://alice@localhost:5432/test?sslrootcert=/tmp/ca&sslcert=/tmp/cert&sslkey=/tmp/key&sslmode=verify-full",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.Equal(t, test.out, GetConnString(&profile.ConnectProfile{
				Host:       host,
				Port:       port,
				User:       test.user,
				Database:   test.database,
				Insecure:   test.insecure,
				CACertPath: caPath,
				CertPath:   certPath,
				KeyPath:    keyPath,
			}, false, false))
		})
	}
}
