//go:build unix

/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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
package regular

import (
	"context"
	"os/user"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	tracessh "github.com/gravitational/teleport/api/observability/tracing/ssh"
	"github.com/gravitational/teleport/lib/utils/host"
	"github.com/gravitational/teleport/lib/utils/testutils"
)

// BenchmarkRootExecCommand measures performance of running multiple exec requests
// over a single ssh connection. The same test is run with and without host user
// creation support to catch any performance degradation caused by user provisioning.
func BenchmarkRootExecCommand(b *testing.B) {
	testutils.RequireRoot(b)

	b.ReportAllocs()

	cases := []struct {
		name       string
		createUser bool
	}{
		{
			name: "no user creation",
		},
		{
			name:       "with user creation",
			createUser: true,
		},
	}

	for _, test := range cases {
		b.Run(test.name, func(b *testing.B) {
			var opts []ServerOption
			if test.createUser {
				opts = []ServerOption{SetCreateHostUser(true)}
			}

			f := newFixtureWithoutDiskBasedLogging(b, opts...)

			for b.Loop() {
				username := f.user
				if test.createUser {
					username = testutils.GenerateLocalUsername(b)
					b.Cleanup(func() { _, _ = host.UserDel(username) })
				}

				_, err := newUpack(f.testSrv, username, []string{username, f.user}, wildcardAllow)
				require.NoError(b, err)

				clt := f.newSSHClient(context.Background(), b, &user.User{Username: username})

				executeCommand(b, clt, "uptime", 10)
			}
		})
	}
}

func executeCommand(tb testing.TB, clt *tracessh.Client, command string, executions int) {
	tb.Helper()

	var wg sync.WaitGroup
	for i := 0; i < executions; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			ctx := context.Background()

			se, err := clt.NewSession(ctx)
			assert.NoError(tb, err)
			defer se.Close()

			assert.NoError(tb, se.Run(ctx, command))
		}()
	}

	wg.Wait()
}
