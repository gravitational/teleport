// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package postgres

import (
	"context"
	"log/slog"
	"testing"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/stretchr/testify/require"
)

func TestFixGCPUsername(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name     string
		spec     types.DatabaseSpecV3
		user     string
		wantUser string
	}{
		{
			name: "alloydb",
			spec: types.DatabaseSpecV3{
				Protocol: types.DatabaseProtocolPostgreSQL,
				URI:      "alloydb://projects/my-project-123456/locations/europe-west1/clusters/my-cluster/instances/my-instance",
			},
			user:     "testuser",
			wantUser: "testuser@my-project-123456.iam",
		},
		{
			name: "cloudsql",
			spec: types.DatabaseSpecV3{
				Protocol: types.DatabaseProtocolPostgreSQL,
				URI:      "localhost:5432",
				GCP: types.GCPCloudSQL{
					ProjectID:  "my-project-123456",
					InstanceID: "instance-123",
				},
			},
			user:     "testuser",
			wantUser: "testuser@my-project-123456.iam",
		},
		{
			name: "nongcp",
			spec: types.DatabaseSpecV3{
				Protocol: types.DatabaseProtocolPostgreSQL,
				URI:      "localhost:12345",
			},
			user:     "testuser",
			wantUser: "testuser",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			db, err := types.NewDatabaseV3(types.Metadata{Name: "mydb"}, test.spec)
			require.NoError(t, err)

			sessionCtx := &common.Session{
				Database:     db,
				DatabaseUser: test.user,
			}

			err = fixGCPUsername(context.Background(), slog.Default(), sessionCtx)
			require.NoError(t, err)

			require.Equal(t, test.wantUser, sessionCtx.DatabaseUser)
		})
	}
}
