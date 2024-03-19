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
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/elasticache"
	"github.com/aws/aws-sdk-go/service/memorydb"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud/mocks"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/srv/db/common"
)

// TestAuthTokens verifies that proper IAM auth tokens are used when connecting
// to cloud databases such as RDS, Redshift, Cloud SQL.
func TestAuthTokens(t *testing.T) {
	ctx := context.Background()
	testCtx := setupTestContext(ctx, t)
	withDBs := []withDatabaseOption{
		withRDSPostgres("postgres-rds-correct-token", rdsAuthToken),
		withRDSPostgres("postgres-rds-incorrect-token", "qwe123"),
		withRedshiftPostgres("postgres-redshift-correct-token", redshiftAuthToken),
		withRedshiftPostgres("postgres-redshift-incorrect-token", "qwe123"),
		withCloudSQLPostgres("postgres-cloudsql-correct-token", cloudSQLAuthToken),
		withCloudSQLPostgres("postgres-cloudsql-incorrect-token", "qwe123"),
		withAzurePostgres("postgres-azure-correct-token", azureAccessToken),
		withAzurePostgres("postgres-azure-incorrect-token", "qwe123"),
		withRDSMySQL("mysql-rds-correct-token", "root", rdsAuthToken),
		withRDSMySQL("mysql-rds-incorrect-token", "root", "qwe123"),
		withCloudSQLMySQL("mysql-cloudsql-correct-token", "root", cloudSQLPassword),
		withCloudSQLMySQL("mysql-cloudsql-incorrect-token", "root", "qwe123"),
		withAzureMySQL("mysql-azure-correct-token", "root", azureAccessToken),
		withAzureMySQL("mysql-azure-incorrect-token", "root", "qwe123"),
		withAzureRedis("redis-azure-correct-token", azureRedisToken),
		withAzureRedis("redis-azure-incorrect-token", "qwe123"),
		withElastiCacheRedis("redis-elasticache-correct-token", elastiCacheRedisToken, "7.0.0"),
		withElastiCacheRedis("redis-elasticache-incorrect-token", "qwe123", "7.0.0"),
		withMemoryDBRedis("redis-memorydb-correct-token", memorydbToken, "7.0"),
		withMemoryDBRedis("redis-memorydb-incorrect-token", "qwe123", "7.0"),
	}
	databases := make([]types.Database, 0, len(withDBs))
	for _, withDB := range withDBs {
		databases = append(databases, withDB(t, ctx, testCtx))
	}
	ecMock := &mocks.ElastiCacheMock{}
	elastiCacheIAMUser := &elasticache.User{
		UserId:         aws.String("default"),
		Authentication: &elasticache.Authentication{Type: aws.String("iam")},
	}
	ecMock.AddMockUser(elastiCacheIAMUser, nil)
	memorydbMock := &mocks.MemoryDBMock{}
	memorydbIAMUser := &memorydb.User{
		Name:           aws.String("default"),
		Authentication: &memorydb.Authentication{Type: aws.String("iam")},
	}
	memorydbMock.AddMockUser(memorydbIAMUser, nil)
	testCtx.server = testCtx.setupDatabaseServer(ctx, t, agentParams{
		Databases:   databases,
		ElastiCache: ecMock,
		MemoryDB:    memorydbMock,
	})
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
			// Make sure we print example RDS IAM policy.
			err: "arn:aws:rds-db:us-east-1:{account_id}:dbuser:{resource_id}",
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
			// Make sure we print example RDS IAM policy.
			err: "arn:aws:rds-db:us-east-1:{account_id}:dbuser:{resource_id}",
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
		{
			desc:     "correct Azure Redis auth token",
			service:  "redis-azure-correct-token",
			protocol: defaults.ProtocolRedis,
		},
		{
			desc:     "incorrect Azure Redis auth token",
			service:  "redis-azure-incorrect-token",
			protocol: defaults.ProtocolRedis,
			err:      "WRONGPASS invalid username-password pair",
		},
		{
			desc:     "correct ElastiCache Redis auth token",
			service:  "redis-elasticache-correct-token",
			protocol: defaults.ProtocolRedis,
		},
		{
			desc:     "incorrect ElastiCache Redis auth token",
			service:  "redis-elasticache-incorrect-token",
			protocol: defaults.ProtocolRedis,
			// Make sure we print a user-friendly IAM auth error.
			err: "Make sure that IAM auth is enabled",
		},
		{
			desc:     "correct MemoryDB auth token",
			service:  "redis-memorydb-correct-token",
			protocol: defaults.ProtocolRedis,
		},
		{
			desc:     "incorrect MemoryDB auth token",
			service:  "redis-memorydb-incorrect-token",
			protocol: defaults.ProtocolRedis,
			// Make sure we print a user-friendly IAM auth error.
			err: "Make sure that IAM auth is enabled",
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
			case defaults.ProtocolRedis:
				conn, err := testCtx.redisClient(ctx, "alice", test.service, "default")
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
		FieldLogger: logrus.WithField(teleport.ComponentKey, "auth:test"),
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
	// azureAccessToken is a mock Azure access token.
	azureAccessToken = "azure-access-token"
	// azureRedisToken is a mock Azure Redis token.
	azureRedisToken = "azure-redis-token"
	// elastiCacheRedisToken is a mock ElastiCache Redis token.
	elastiCacheRedisToken = "elasticache-redis-token"
	// memorydbToken is a mock MemoryDB auth token.
	memorydbToken = "memorydb-token"
	// atlasAuthUser is a mock Mongo Atlas IAM auth user.
	atlasAuthUser = "arn:aws:iam::111111111111:role/alice"
	// atlasAuthToken is a mock Mongo Atlas IAM auth token.
	atlasAuthToken = "atlas-auth-token"
	// atlasAuthSessionToken is a mock Mongo Atlas IAM auth session token.
	atlasAuthSessionToken = "atlas-session-token"
)

// GetRDSAuthToken generates RDS/Aurora auth token.
func (a *testAuth) GetRDSAuthToken(ctx context.Context, sessionCtx *common.Session) (string, error) {
	a.Infof("Generating RDS auth token for %v.", sessionCtx)
	return rdsAuthToken, nil
}

// GetRedshiftAuthToken generates Redshift auth token.
func (a *testAuth) GetRedshiftAuthToken(ctx context.Context, sessionCtx *common.Session) (string, string, error) {
	a.Infof("Generating Redshift auth token for %v.", sessionCtx)
	return redshiftAuthUser, redshiftAuthToken, nil
}

func (a *testAuth) GetRedshiftServerlessAuthToken(ctx context.Context, sessionCtx *common.Session) (string, string, error) {
	return "", "", trace.NotImplemented("GetRedshiftServerlessAuthToken is not implemented")
}

func (a *testAuth) GetElastiCacheRedisToken(ctx context.Context, sessionCtx *common.Session) (string, error) {
	return elastiCacheRedisToken, nil
}

func (a *testAuth) GetMemoryDBToken(ctx context.Context, sessionCtx *common.Session) (string, error) {
	return memorydbToken, nil
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

// GetAzureAccessToken generates Azure access token.
func (a *testAuth) GetAzureAccessToken(ctx context.Context, sessionCtx *common.Session) (string, error) {
	a.Infof("Generating Azure access token for %v.", sessionCtx)
	return azureAccessToken, nil
}

// GetAzureCacheForRedisToken retrieves auth token for Azure Cache for Redis.
func (a *testAuth) GetAzureCacheForRedisToken(ctx context.Context, sessionCtx *common.Session) (string, error) {
	a.Infof("Generating Azure Redis token for %v.", sessionCtx)
	return azureRedisToken, nil
}

// GetAWSIAMCreds returns the AWS IAM credentials, including access key, secret
// access key and session token.
func (a *testAuth) GetAWSIAMCreds(ctx context.Context, sessionCtx *common.Session) (string, string, string, error) {
	a.Infof("Generating AWS IAM credentials for %v.", sessionCtx)
	return atlasAuthUser, atlasAuthToken, atlasAuthSessionToken, nil
}

func TestMongoDBAtlas(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	testCtx := setupTestContext(ctx, t,
		withAtlasMongo("iam-auth", atlasAuthUser, atlasAuthSessionToken),
		withAtlasMongo("certs-auth", "", ""),
	)
	go testCtx.startHandlingConnections()
	testCtx.createUserAndRole(ctx, t, "alice", "admin", []string{types.Wildcard}, []string{types.Wildcard})

	for name, tt := range map[string]struct {
		service   string
		dbUser    string
		expectErr require.ErrorAssertionFunc
	}{
		"authenticates with arn username": {
			service:   "iam-auth",
			dbUser:    "arn:aws:iam::111111111111:role/alice",
			expectErr: require.NoError,
		},
		"disabled iam authentication": {
			service:   "certs-auth",
			dbUser:    "arn:aws:iam::111111111111:role/alice",
			expectErr: require.Error,
		},
		"partial arn authentication": {
			service:   "iam-auth",
			dbUser:    "role/alice",
			expectErr: require.NoError,
		},
		"IAM user arn": {
			service:   "iam-auth",
			dbUser:    "arn:aws:iam::111111111111:user/alice",
			expectErr: require.Error,
		},
		"certs authentication": {
			service:   "certs-auth",
			dbUser:    "alice",
			expectErr: require.NoError,
		},
	} {
		t.Run(name, func(t *testing.T) {
			conn, err := testCtx.mongoClient(ctx, "alice", tt.service, tt.dbUser)
			tt.expectErr(t, err)
			if err != nil {
				require.NoError(t, conn.Disconnect(ctx))
			}
		})
	}
}
