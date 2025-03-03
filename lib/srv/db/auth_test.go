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
	"crypto/tls"
	"log/slog"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	elasticache "github.com/aws/aws-sdk-go-v2/service/elasticache"
	ectypes "github.com/aws/aws-sdk-go-v2/service/elasticache/types"
	"github.com/aws/aws-sdk-go-v2/service/memorydb"
	memorydbtypes "github.com/aws/aws-sdk-go-v2/service/memorydb/types"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/cloud/mocks"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/gravitational/teleport/lib/srv/db/redis"
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
		withSpanner("spanner-correct-token", cloudSpannerAuthToken),
		withSpanner("spanner-incorrect-token", "xyz123"),
	}
	databases := make([]types.Database, 0, len(withDBs))
	for _, withDB := range withDBs {
		databases = append(databases, withDB(t, ctx, testCtx))
	}
	ecMock := &mocks.ElastiCacheClient{}
	elastiCacheIAMUser := ectypes.User{
		UserId:         aws.String("default"),
		Authentication: &ectypes.Authentication{Type: ectypes.AuthenticationTypeIam},
	}
	ecMock.AddMockUser(elastiCacheIAMUser, nil)

	memorydbMock := &mocks.MemoryDBClient{}
	memorydbIAMUser := memorydbtypes.User{
		Name:           aws.String("default"),
		Authentication: &memorydbtypes.Authentication{Type: memorydbtypes.AuthenticationTypeIam},
	}
	memorydbMock.AddMockUser(memorydbIAMUser, nil)
	testCtx.server = testCtx.setupDatabaseServer(ctx, t, agentParams{
		Databases: databases,
		GetEngineFn: func(db types.Database, conf common.EngineConfig) (common.Engine, error) {
			if db.GetProtocol() != defaults.ProtocolRedis {
				return common.GetEngine(db, conf)
			}
			if err := conf.CheckAndSetDefaults(); err != nil {
				return nil, trace.Wrap(err)
			}
			conf.AWSConfigProvider = &mocks.AWSConfigProvider{}
			return &redis.Engine{
				EngineConfig: conf,
				AWSClients: fakeRedisAWSClients{
					ecClient:  ecMock,
					mdbClient: memorydbMock,
				},
			}, nil
		},
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
		{
			desc:     "correct Spanner auth token",
			service:  "spanner-correct-token",
			protocol: defaults.ProtocolSpanner,
		},
		{
			desc:     "incorrect Spanner auth token",
			service:  "spanner-incorrect-token",
			protocol: defaults.ProtocolSpanner,
			err:      "invalid RPC auth token",
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
			case defaults.ProtocolSpanner:
				clt, localProxy, err := testCtx.spannerClient(ctx, "alice", test.service, "admin", "somedb")
				// Teleport doesn't actually try to fetch a token until an RPC
				// is received, so it shouldn't fail after connecting.
				require.NoError(t, err)
				t.Cleanup(func() {
					// Disconnect.
					clt.Close()
					_ = localProxy.Close()
				})
				_, err = pingSpanner(ctx, clt, 123)
				if test.err != "" {
					require.Error(t, err)
					require.Contains(t, err.Error(), test.err)
				} else {
					require.NoError(t, err)
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
	realAuth common.Auth
	// Logger is used for logging.
	*slog.Logger
}

func newTestAuth(ac common.AuthConfig) (*testAuth, error) {
	auth, err := common.NewAuth(ac)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &testAuth{
		realAuth: auth,
		Logger:   slog.With(teleport.ComponentKey, "auth:test"),
	}, nil
}

var _ common.Auth = (*testAuth)(nil)

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
	// cloudSpannerAuthToken is a mock Cloud Spanner IAM auth token.
	cloudSpannerAuthToken = "cloud-spanner-auth-token"
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

type fakeTokenSource struct {
	*slog.Logger

	token string
	exp   time.Time
}

func (f *fakeTokenSource) Token() (*oauth2.Token, error) {
	f.InfoContext(context.Background(), "Generating Cloud Spanner auth token source")
	return &oauth2.Token{
		Expiry:      f.exp,
		AccessToken: f.token,
	}, nil
}

func (a *testAuth) GetRDSAuthToken(ctx context.Context, database types.Database, databaseUser string) (string, error) {
	a.InfoContext(ctx, "Generating RDS auth token.",
		"database", database,
		"database_user", databaseUser,
	)
	return rdsAuthToken, nil
}

func (a *testAuth) GetRedshiftAuthToken(ctx context.Context, database types.Database, databaseUser string, databaseName string) (string, string, error) {
	a.InfoContext(ctx, "Generating Redshift auth token",
		"database", database,
		"database_user", databaseUser,
		"database_name", databaseName,
	)
	return redshiftAuthUser, redshiftAuthToken, nil
}

func (a *testAuth) GetRedshiftServerlessAuthToken(ctx context.Context, database types.Database, databaseUser string, databaseName string) (string, string, error) {
	return "", "", trace.NotImplemented("GetRedshiftServerlessAuthToken is not implemented")
}

func (a *testAuth) GetElastiCacheRedisToken(ctx context.Context, database types.Database, databaseUser string) (string, error) {
	return elastiCacheRedisToken, nil
}

func (a *testAuth) GetMemoryDBToken(ctx context.Context, database types.Database, databaseUser string) (string, error) {
	return memorydbToken, nil
}

func (a *testAuth) GetCloudSQLAuthToken(ctx context.Context, databaseUser string) (string, error) {
	a.InfoContext(ctx, "Generating Cloud SQL auth token", "database_user", databaseUser)
	return cloudSQLAuthToken, nil
}

func (a *testAuth) GetSpannerTokenSource(ctx context.Context, databaseUser string) (oauth2.TokenSource, error) {
	return &fakeTokenSource{
		token:  cloudSpannerAuthToken,
		Logger: a.Logger.With("database_user", databaseUser),
	}, nil
}

func (a *testAuth) GetCloudSQLPassword(ctx context.Context, database types.Database, databaseUser string) (string, error) {
	a.InfoContext(ctx, "Generating Cloud SQL password",
		"database", database,
		"database_user", databaseUser,
	)
	return cloudSQLPassword, nil
}

func (a *testAuth) GetAzureAccessToken(ctx context.Context) (string, error) {
	a.InfoContext(ctx, "Generating Azure access token")
	return azureAccessToken, nil
}

func (a *testAuth) GetAzureCacheForRedisToken(ctx context.Context, database types.Database) (string, error) {
	a.InfoContext(ctx, "Generating Azure Redis token", "database", database)
	return azureRedisToken, nil
}

func (a *testAuth) GetTLSConfig(ctx context.Context, expiry time.Time, database types.Database, databaseUser string) (*tls.Config, error) {
	return a.realAuth.GetTLSConfig(ctx, expiry, database, databaseUser)
}

func (a *testAuth) GetAuthPreference(ctx context.Context) (types.AuthPreference, error) {
	return a.realAuth.GetAuthPreference(ctx)
}

func (a *testAuth) GetAzureIdentityResourceID(ctx context.Context, identityName string) (string, error) {
	return a.realAuth.GetAzureIdentityResourceID(ctx, identityName)
}

func (a *testAuth) GetAWSIAMCreds(ctx context.Context, database types.Database, databaseUser string) (string, string, string, error) {
	a.InfoContext(ctx, "Generating AWS IAM credentials",
		"database", database,
		"database_user", databaseUser,
	)
	return atlasAuthUser, atlasAuthToken, atlasAuthSessionToken, nil
}

func (a *testAuth) GenerateDatabaseClientKey(ctx context.Context) (*keys.PrivateKey, error) {
	key, err := keys.ParsePrivateKey(fixtures.PEMBytes["rsa"])
	return key, trace.Wrap(err)
}

func (a *testAuth) WithLogger(getUpdatedLogger func(*slog.Logger) *slog.Logger) common.Auth {
	return &testAuth{
		realAuth: a.realAuth,
		Logger:   a.Logger,
	}
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

type fakeRedisAWSClients struct {
	mdbClient redis.MemoryDBClient
	ecClient  redis.ElastiCacheClient
}

func (f fakeRedisAWSClients) GetElastiCacheClient(cfg aws.Config, optFns ...func(*elasticache.Options)) redis.ElastiCacheClient {
	return f.ecClient
}

func (f fakeRedisAWSClients) GetMemoryDBClient(cfg aws.Config, optFns ...func(*memorydb.Options)) redis.MemoryDBClient {
	return f.mdbClient
}
