/*
Copyright 2021 Gravitational, Inc.

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
	"testing"

	"github.com/gravitational/teleport/lib/client/db/profile"

	"github.com/stretchr/testify/require"
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
			}))
		})
	}
}
