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
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/cloud/gcp"
	"github.com/gravitational/teleport/lib/cloud/mocks"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/srv/db/common"
)

type fakeAuth struct {
	common.Auth
}

func (a fakeAuth) GetCloudSQLAuthToken(ctx context.Context, databaseUser string) (string, error) {
	if !isDBUserFullGCPServerAccountID(databaseUser) {
		return "", trace.BadParameter("database user must be a service account")
	}
	return "iam-auth-token", nil
}

func (a fakeAuth) GetCloudSQLPassword(ctx context.Context, database types.Database, databaseUser string) (string, error) {
	if isDBUserFullGCPServerAccountID(databaseUser) {
		return "", trace.BadParameter("database user must not be a service account")
	}
	return "one-time-password", nil
}

func (a fakeAuth) WithLogger(getUpdatedLogger func(*slog.Logger) *slog.Logger) common.Auth {
	if a.Auth != nil {
		return a.Auth.WithLogger(getUpdatedLogger)
	}
	return a
}

func Test_getGCPUserAndPassword(t *testing.T) {
	ctx := context.Background()
	authClient := makeAuthClient(t)
	db := makeGCPMySQLDatabase(t)

	tests := []struct {
		name              string
		inputDatabaseUser string
		mockGCPClient     gcp.SQLAdminClient
		wantDatabaseUser  string
		wantPassword      string
		wantError         bool
	}{
		{
			name:              "iam auth with full service account",
			inputDatabaseUser: "iam-auth-user@project-id.iam.gserviceaccount.com",
			wantDatabaseUser:  "iam-auth-user",
			wantPassword:      "iam-auth-token",
		},
		{
			name:              "iam auth with short service account",
			inputDatabaseUser: "iam-auth-user@project-id.iam",
			wantError:         true,
		},
		{
			name:              "iam auth with CLOUD_IAM_SERVICE_ACCOUNT user",
			inputDatabaseUser: "iam-auth-user",
			mockGCPClient: &mocks.GCPSQLAdminClientMock{
				DatabaseUser: makeGCPDatabaseUser("iam-auth-user", "CLOUD_IAM_SERVICE_ACCOUNT"),
			},
			wantDatabaseUser: "iam-auth-user",
			wantPassword:     "iam-auth-token",
		},
		{
			name:              "iam auth with CLOUD_IAM_GROUP_SERVICE_ACCOUNT user",
			inputDatabaseUser: "iam-auth-user",
			mockGCPClient: &mocks.GCPSQLAdminClientMock{
				DatabaseUser: makeGCPDatabaseUser("iam-auth-user", "CLOUD_IAM_GROUP_SERVICE_ACCOUNT"),
			},
			wantDatabaseUser: "iam-auth-user",
			wantPassword:     "iam-auth-token",
		},
		{
			name:              "password auth without GetUser permission",
			inputDatabaseUser: "some-user",
			mockGCPClient:     &mocks.GCPSQLAdminClientMock{
				// Default no permission to GetUser,
			},
			wantDatabaseUser: "some-user",
			wantPassword:     "one-time-password",
		},
		{
			name:              "password auth with BUILT_IN user",
			inputDatabaseUser: "password-user",
			mockGCPClient: &mocks.GCPSQLAdminClientMock{
				DatabaseUser: makeGCPDatabaseUser("password-user", "BUILT_IN"),
			},
			wantDatabaseUser: "password-user",
			wantPassword:     "one-time-password",
		},
		{
			name:              "password auth with empty user type",
			inputDatabaseUser: "password-user",
			mockGCPClient: &mocks.GCPSQLAdminClientMock{
				DatabaseUser: makeGCPDatabaseUser("password-user", ""),
			},
			wantDatabaseUser: "password-user",
			wantPassword:     "one-time-password",
		},
		{
			name:              "unsupported user type",
			inputDatabaseUser: "some-user",
			mockGCPClient: &mocks.GCPSQLAdminClientMock{
				DatabaseUser: makeGCPDatabaseUser("some-user", "CLOUD_IAM_USER"),
			},
			wantError: true,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			sessionCtx := &common.Session{
				Database:     db,
				DatabaseUser: test.inputDatabaseUser,
				ID:           "00000000-0000AAAA-0000BBBB-0000CCCC",
			}

			engine := NewEngine(common.EngineConfig{
				Auth:       &fakeAuth{},
				AuthClient: authClient,
				Context:    ctx,
				Clock:      clockwork.NewRealClock(),
				Log:        slog.Default(),
			}).(*Engine)

			databaseUser, password, err := engine.getGCPUserAndPassword(ctx, sessionCtx, test.mockGCPClient)
			if test.wantError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, test.wantDatabaseUser, databaseUser)
				require.Equal(t, test.wantPassword, password)
			}
		})
	}
}

func makeAuthClient(t *testing.T) *authclient.Client {
	t.Helper()

	authServer, err := auth.NewTestAuthServer(auth.TestAuthServerConfig{
		ClusterName: "mysql-test",
		Dir:         t.TempDir(),
	})
	require.NoError(t, err)
	t.Cleanup(func() { authServer.Close() })

	tlsServer, err := authServer.NewTestTLSServer()
	require.NoError(t, err)
	t.Cleanup(func() { tlsServer.Close() })

	authClient, err := tlsServer.NewClient(auth.TestServerID(types.RoleDatabase, "mysql-test"))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, authClient.Close()) })

	return authClient
}

func makeGCPMySQLDatabase(t *testing.T) types.Database {
	t.Helper()

	database, err := types.NewDatabaseV3(types.Metadata{
		Name: "gcp-mysql",
	}, types.DatabaseSpecV3{
		Protocol: defaults.ProtocolMySQL,
		URI:      "localhost:3306",
		GCP: types.GCPCloudSQL{
			ProjectID:  "project-1",
			InstanceID: "instance-1",
		},
	})
	require.NoError(t, err)
	return database
}

func makeGCPDatabaseUser(name, userType string) *sqladmin.User {
	return &sqladmin.User{
		Name:     name,
		Host:     "%",
		Type:     userType,
		Instance: "instance-1",
	}
}
