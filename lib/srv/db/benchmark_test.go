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

package db

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

/*
$ go test ./lib/srv/db -bench=. -run=^$ -benchtime=3x
goos: darwin
goarch: arm64
pkg: github.com/gravitational/teleport/lib/srv/db
BenchmarkPostgresReadLargeTable/size=11-10         	       3	 286618514 ns/op
BenchmarkPostgresReadLargeTable/size=20-10         	       3	 253457917 ns/op
BenchmarkPostgresReadLargeTable/size=100-10        	       3	 222804292 ns/op
BenchmarkPostgresReadLargeTable/size=1000-10       	       3	 216612764 ns/op
BenchmarkPostgresReadLargeTable/size=2000-10       	       3	 214121861 ns/op
BenchmarkPostgresReadLargeTable/size=8000-10       	       3	 215046472 ns/op
*/
// BenchmarkPostgresReadLargeTable is a benchmark for read-heavy usage of Postgres.
// Depending on the message size we may get different performance, due to the way the respective engine is written.
func BenchmarkPostgresReadLargeTable(b *testing.B) {
	b.StopTimer()
	ctx := context.Background()
	testCtx := setupTestContext(ctx, b, withSelfHostedPostgres("postgres", func(db *types.DatabaseV3) {
		db.SetStaticLabels(map[string]string{"foo": "bar"})
	}))
	go testCtx.startHandlingConnections()

	user := "alice"
	role := "admin"
	allow := []string{types.Wildcard}

	// Create user/role with the requested permissions.
	testCtx.createUserAndRole(ctx, b, user, role, allow, allow)
	for _, messageSize := range []int{11, 20, 100, 1000, 2000, 8000} {

		// connect to the database
		pgConn, err := testCtx.postgresClient(ctx, user, "postgres", "postgres", "postgres")
		require.NoError(b, err)

		// total bytes to be transmitted, approximate.
		const totalBytes = 100 * 1024 * 1024
		// calculate the number of messages required to reach totalBytes of transferred data.
		rowCount := totalBytes / messageSize

		// run first query without timer. the server will cache the message.
		_, err = pgConn.Exec(ctx, fmt.Sprintf("SELECT * FROM bench_%v LIMIT %v", messageSize, rowCount)).ReadAll()
		require.NoError(b, err)

		b.Run(fmt.Sprintf("size=%v", messageSize), func(b *testing.B) {
			for b.Loop() {
				// Execute a query, count results.
				q := pgConn.Exec(ctx, fmt.Sprintf("SELECT * FROM bench_%v LIMIT %v", messageSize, rowCount))

				// pgConn.Exec can potentially execute multiple SQL queries.
				// the outer loop is a query loop, the inner loop is for individual results.
				rows := 0
				for q.NextResult() {
					rr := q.ResultReader()
					for rr.NextRow() {
						rows++
					}
				}

				require.NoError(b, q.Close())
				require.Equal(b, rowCount, rows)
			}
		})

		// Disconnect.
		err = pgConn.Close(ctx)
		require.NoError(b, err)
	}
}
