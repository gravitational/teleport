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

package mysql

import (
	"context"
	"log/slog"
	"testing"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	sqladmin "google.golang.org/api/sqladmin/v1beta4"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/cloud/gcp"
	"github.com/gravitational/teleport/lib/cloud/mocks"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/srv/db/healthchecks"
)

func Test_getTargetAddress(t *testing.T) {
	t.Parallel()
	tests := []struct {
		desc       string
		dbURI      string
		isCloudSQL bool
		clients    fakeGCPClients
		want       string
	}{
		{
			desc:  "default",
			dbURI: "example.com:3306",
			want:  "example.com:3306",
		},
		{
			desc:       "cloudsql db failed to get admin client",
			dbURI:      "example.com:3306",
			isCloudSQL: true,
			clients: fakeGCPClients{
				getClientErr: trace.Errorf("failed to get Cloud SQL admin client"),
			},
			want: "example.com:3306",
		},
		{
			desc:       "cloudsql db requires ssl",
			dbURI:      "example.com:3306",
			isCloudSQL: true,
			clients: fakeGCPClients{
				client: &mocks.GCPSQLAdminClientMock{
					DatabaseInstance: &sqladmin.DatabaseInstance{
						Settings: &sqladmin.Settings{
							IpConfiguration: &sqladmin.IpConfiguration{
								RequireSsl: true,
							},
						},
					},
				},
			},
			want: "example.com:3307",
		},
		{
			desc:       "cloudsql db non-default port is not overridden when ssl is required",
			dbURI:      "example.com:1234",
			isCloudSQL: true,
			clients: fakeGCPClients{
				client: &mocks.GCPSQLAdminClientMock{
					DatabaseInstance: &sqladmin.DatabaseInstance{
						Settings: &sqladmin.Settings{
							IpConfiguration: &sqladmin.IpConfiguration{
								RequireSsl: true,
							},
						},
					},
				},
			},
			want: "example.com:1234",
		},
		{
			desc:       "cloudsql db does not require ssl",
			dbURI:      "example.com:3306",
			isCloudSQL: true,
			clients: fakeGCPClients{
				client: &mocks.GCPSQLAdminClientMock{
					DatabaseInstance: &sqladmin.DatabaseInstance{
						Settings: &sqladmin.Settings{
							IpConfiguration: &sqladmin.IpConfiguration{
								RequireSsl: false,
							},
						},
					},
				},
			},
			want: "example.com:3306",
		},
		{
			desc:       "cloudsql db access denied to check ssl setting ",
			dbURI:      "example.com:3306",
			isCloudSQL: true,
			clients: fakeGCPClients{
				client: &mocks.GCPSQLAdminClientMock{
					GetDatabaseInstanceError: trace.AccessDenied("unauthorized"),
				},
			},
			want: "example.com:3306",
		},
		{
			desc:       "cloudsql db other error when checking ssl setting ",
			dbURI:      "example.com:3306",
			isCloudSQL: true,
			clients: fakeGCPClients{
				client: &mocks.GCPSQLAdminClientMock{
					GetDatabaseInstanceError: trace.NotFound("not found"),
				},
			},
			want: "example.com:3306",
		},
	}
	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			dbSpec := types.DatabaseSpecV3{
				Protocol: defaults.ProtocolMySQL,
				URI:      test.dbURI,
			}
			if test.isCloudSQL {
				dbSpec.GCP = types.GCPCloudSQL{
					ProjectID:  "project-1",
					InstanceID: "instance-1",
				}
			}
			db, err := types.NewDatabaseV3(types.Metadata{
				Name: "test",
			}, dbSpec)
			require.NoError(t, err)
			checker, err := newHealthChecker(healthchecks.HealthCheckerConfig{
				Auth:                  fakeAuth{},
				AuthClient:            &authclient.Client{},
				Clock:                 clockwork.NewRealClock(),
				Database:              db,
				GCPClients:            test.clients,
				Log:                   slog.Default(),
				UpdateProxiedDatabase: func(string, func(types.Database) error) error { return nil },
			})
			require.NoError(t, err)
			got := checker.getTargetAddress(t.Context())
			require.Equal(t, test.want, got)
		})
	}
}

func Test_getHealthCheckDBUser(t *testing.T) {
	rdsDB, err := types.NewDatabaseV3(types.Metadata{
		Name: "rds",
	}, types.DatabaseSpecV3{
		Protocol: defaults.ProtocolMySQL,
		URI:      "aurora-instance-1.abcdefghijklmnop.us-west-1.rds.amazonaws.com:5432",
	})
	require.NoError(t, err)
	cloudSQL := makeGCPMySQLDatabase(t)

	tests := []struct {
		desc     string
		database types.Database
		env      string
		admin    string
		want     string
	}{
		{
			desc:     "rds default",
			database: rdsDB,
			want:     "teleport-healthchecker",
		},
		{
			desc:     "rds with env",
			database: rdsDB,
			env:      "env",
			want:     "env",
		},
		{
			desc:     "rds with admin",
			database: rdsDB,
			admin:    "admin",
			want:     "admin",
		},
		{
			desc:     "rds with env and admin uses admin",
			database: rdsDB,
			env:      "env",
			admin:    "admin",
			want:     "admin",
		},
		{
			desc:     "gcp default",
			database: cloudSQL,
			want:     "teleport-healthchecker@project-1.iam.gserviceaccount.com",
		},
		{
			desc:     "gcp with env",
			database: cloudSQL,
			env:      "env",
			want:     "env@project-1.iam.gserviceaccount.com",
		},
		{
			desc:     "gcp with admin",
			database: cloudSQL,
			admin:    "admin",
			want:     "admin@project-1.iam.gserviceaccount.com",
		},
		{
			desc:     "gcp with env and admin uses admin",
			database: cloudSQL,
			env:      "env",
			admin:    "admin",
			want:     "admin@project-1.iam.gserviceaccount.com",
		},
	}
	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			t.Setenv(healthCheckUserEnvVar, test.env)
			db := test.database.Copy()
			db.Spec.AdminUser = &types.DatabaseAdminUser{Name: test.admin}
			got := getHealthCheckDBUser(db)
			require.Equal(t, test.want, got)
		})
	}
}

type fakeGCPClients struct {
	healthchecks.GCPClients

	getClientErr error
	client       *mocks.GCPSQLAdminClientMock
}

func (f fakeGCPClients) GetSQLAdminClient(context.Context) (gcp.SQLAdminClient, error) {
	return f.client, f.getClientErr
}
