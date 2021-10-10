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

package db

import (
	"context"
	"testing"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/srv/db/common"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

// TestAuthTokens verifies that proper IAM auth tokens are used when connecting
// to cloud databases such as RDS, Redshift, Cloud SQL.
func TestAuthTokens(t *testing.T) {
	ctx := context.Background()
	testCtx := setupTestContext(ctx, t,
		withRDSPostgres("postgres-rds-correct-token", rdsAuthToken),
		withRDSPostgres("postgres-rds-incorrect-token", "qwe123"),
		withRedshiftPostgres("postgres-redshift-correct-token", redshiftAuthToken),
		withRedshiftPostgres("postgres-redshift-incorrect-token", "qwe123"),
		withCloudSQLPostgres("postgres-cloudsql-correct-token", cloudSQLAuthToken),
		withCloudSQLPostgres("postgres-cloudsql-incorrect-token", "qwe123"),
		withRDSMySQL("mysql-rds-correct-token", "root", rdsAuthToken),
		withRDSMySQL("mysql-rds-incorrect-token", "root", "qwe123"),
		withCloudSQLMySQL("mysql-cloudsql-correct-token", "root", cloudSQLPassword),
		withCloudSQLMySQL("mysql-cloudsql-incorrect-token", "root", "qwe123"))
	go testCtx.startHandlingConnections()

	testCtx.createUserAndRole(ctx, t, "alice", "admin", []string{types.Wildcard}, []string{types.Wildcard})

	tests := []struct {
		desc     string
		service  string
		protocol string
		err      string
	}{
		{
			desc:     "correct Postgres RDS IAM auth token",
			service:  "postgres-rds-correct-token",
			protocol: defaults.ProtocolPostgres,
		},
		{
			desc:     "incorrect Postgres RDS IAM auth token",
			service:  "postgres-rds-incorrect-token",
			protocol: defaults.ProtocolPostgres,
			err:      "rds-db:connect", // Make sure we print example RDS IAM policy.
		},
		{
			desc:     "correct Postgres Redshift IAM auth token",
			service:  "postgres-redshift-correct-token",
			protocol: defaults.ProtocolPostgres,
		},
		{
			desc:     "incorrect Postgres Redshift IAM auth token",
			service:  "postgres-redshift-incorrect-token",
			protocol: defaults.ProtocolPostgres,
			err:      "invalid auth token",
		},
		{
			desc:     "correct Postgres Cloud SQL IAM auth token",
			service:  "postgres-cloudsql-correct-token",
			protocol: defaults.ProtocolPostgres,
		},
		{
			desc:     "incorrect Postgres Cloud SQL IAM auth token",
			service:  "postgres-cloudsql-incorrect-token",
			protocol: defaults.ProtocolPostgres,
			err:      "invalid auth token",
		},
		{
			desc:     "correct MySQL RDS IAM auth token",
			service:  "mysql-rds-correct-token",
			protocol: defaults.ProtocolMySQL,
		},
		{
			desc:     "incorrect MySQL RDS IAM auth token",
			service:  "mysql-rds-incorrect-token",
			protocol: defaults.ProtocolMySQL,
			err:      "rds-db:connect", // Make sure we print example RDS IAM policy.
		},
		{
			desc:     "correct MySQL Cloud SQL IAM auth token",
			service:  "mysql-cloudsql-correct-token",
			protocol: defaults.ProtocolMySQL,
		},
		{
			desc:     "incorrect MySQL Cloud SQL IAM auth token",
			service:  "mysql-cloudsql-incorrect-token",
			protocol: defaults.ProtocolMySQL,
			err:      "Access denied for user",
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			switch test.protocol {
			case defaults.ProtocolPostgres:
				conn, err := testCtx.postgresClient(ctx, "alice", test.service, "postgres", "postgres")
				if test.err != "" {
					require.Error(t, err)
					require.Contains(t, err.Error(), test.err)
				} else {
					require.NoError(t, err)
					require.NoError(t, conn.Close(ctx))
				}
			case defaults.ProtocolMySQL:
				conn, err := testCtx.mysqlClient("alice", test.service, "root")
				if test.err != "" {
					require.Error(t, err)
					require.Contains(t, err.Error(), test.err)
				} else {
					require.NoError(t, err)
					require.NoError(t, conn.Close())
				}
			default:
				t.Fatalf("unrecognized database protocol in test: %q", test.protocol)
			}
		})
	}
}

// testAuth mocks cloud provider auth tokens generation for use in tests.
type testAuth struct {
	// Auth is the wrapped "real" auth that handles everything except for
	// cloud auth tokens generation.
	common.Auth
	// FieldLogger is used for logging.
	logrus.FieldLogger
}

func newTestAuth(ac common.AuthConfig) (*testAuth, error) {
	auth, err := common.NewAuth(ac)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &testAuth{
		Auth:        auth,
		FieldLogger: logrus.WithField(trace.Component, "auth:test"),
	}, nil
}

const (
	// rdsAuthToken is a mock RDS IAM auth token.
	rdsAuthToken = "rds-auth-token"
	// redshiftAuthUser is a mock Redshift IAM auth user.
	redshiftAuthUser = "redshift-auth-user"
	// redshiftAuthToken is a mock Redshift IAM auth token.
	redshiftAuthToken = "redshift-auth-token"
	// cloudSQLAuthToken is a mock Cloud SQL IAM auth token.
	cloudSQLAuthToken = "cloudsql-auth-token"
	// cloudSQLPassword is a mock Cloud SQL user password.
	cloudSQLPassword = "cloudsql-password"
)

// GetRDSAuthToken generates RDS/Aurora auth token.
func (a *testAuth) GetRDSAuthToken(sessionCtx *common.Session) (string, error) {
	a.Infof("Generating RDS auth token for %v.", sessionCtx)
	return rdsAuthToken, nil
}

// GetRedshiftAuthToken generates Redshift auth token.
func (a *testAuth) GetRedshiftAuthToken(sessionCtx *common.Session) (string, string, error) {
	a.Infof("Generating Redshift auth token for %v.", sessionCtx)
	return redshiftAuthUser, redshiftAuthToken, nil
}

// GetCloudSQLAuthToken generates Cloud SQL auth token.
func (a *testAuth) GetCloudSQLAuthToken(ctx context.Context, sessionCtx *common.Session) (string, error) {
	a.Infof("Generating Cloud SQL auth token for %v.", sessionCtx)
	return cloudSQLAuthToken, nil
}

// GetCloudSQLPassword generates Cloud SQL user password.
func (a *testAuth) GetCloudSQLPassword(ctx context.Context, sessionCtx *common.Session) (string, error) {
	a.Infof("Generating Cloud SQL user password %v.", sessionCtx)
	return cloudSQLPassword, nil
}
