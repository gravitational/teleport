/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package dbcmd

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/observability/tracing"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

func TestCLICommandBuilderGetExecCommand(t *testing.T) {
	fakeExec := &fakeExec{
		execOutput: map[string][]byte{
			"psql":  []byte(""),
			"mysql": []byte(""),
		},
	}

	conf := &client.Config{
		ClientStore:  client.NewFSClientStore(t.TempDir()),
		Host:         "localhost",
		WebProxyAddr: "proxy.example.com",
		SiteName:     "db.example.com",
		Tracer:       tracing.NoopProvider().Tracer("test"),
	}

	tc, err := client.NewClient(conf)
	require.NoError(t, err)

	profile := &client.ProfileStatus{
		Name:     "example.com",
		Username: "bob",
		Dir:      "/tmp",
		Cluster:  "example.com",
	}

	tests := []struct {
		name     string
		opts     []ConnectCommandFunc
		protocol string
		cmd      []string
		wantErr  bool
	}{
		{
			name:     "not authenticated tunnel",
			protocol: defaults.ProtocolPostgres,
			wantErr:  true,
		},
		{
			name:     "unsupported protocol",
			protocol: defaults.ProtocolDynamoDB,
			opts:     []ConnectCommandFunc{WithNoTLS()},
			wantErr:  true,
		},
		{
			name:     "postgres",
			protocol: defaults.ProtocolPostgres,
			opts:     []ConnectCommandFunc{WithNoTLS()},
			cmd:      []string{"psql", "postgres://db-user@localhost:12345/db-name", "-c", "select 1"},
			wantErr:  false,
		},
		{
			name:     "mysql",
			protocol: defaults.ProtocolMySQL,
			opts:     []ConnectCommandFunc{WithNoTLS()},
			cmd:      []string{"mysql", "--user", "db-user", "--database", "db-name", "--port", "12345", "--host", "localhost", "--protocol", "TCP", "-e", "select 1"},
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			database := tlsca.RouteToDatabase{
				Protocol:    tt.protocol,
				Database:    "db-name",
				Username:    "db-user",
				ServiceName: "db-service",
			}

			opts := append([]ConnectCommandFunc{
				WithLocalProxy("localhost", 12345, ""),
				WithExecer(fakeExec),
			}, tt.opts...)

			c := NewCmdBuilder(tc, profile, database, "root", opts...)
			c.uid = utils.NewFakeUID()
			got, err := c.GetExecCommand(context.Background(), "select 1")
			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.cmd, got.Args)
		})
	}
}
