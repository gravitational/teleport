/*
Copyright 2020-2021 Gravitational, Inc.

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
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"os"
	"sync"
	"testing"
	"time"

	mssql "github.com/denisenkom/go-mssqldb"
	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events/eventstest"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/limiter"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/multiplexer"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/alpnproxy"
	alpncommon "github.com/gravitational/teleport/lib/srv/alpnproxy/common"
	"github.com/gravitational/teleport/lib/srv/db/cloud"
	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/gravitational/teleport/lib/srv/db/mongodb"
	"github.com/gravitational/teleport/lib/srv/db/mysql"
	"github.com/gravitational/teleport/lib/srv/db/postgres"
	"github.com/gravitational/teleport/lib/srv/db/redis"
	"github.com/gravitational/teleport/lib/srv/db/sqlserver"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"

	goredis "github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jackc/pgconn"
	"github.com/jonboulle/clockwork"
	mysqlclient "github.com/siddontang/go-mysql/client"
	mysqllib "github.com/siddontang/go-mysql/mysql"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/x/mongo/driver/wiremessage"
	sqladmin "google.golang.org/api/sqladmin/v1beta4"
)

func TestMain(m *testing.M) {
	utils.InitLoggerForTests()
	os.Exit(m.Run())
}

// TestAccessPostgres verifies access scenarios to a Postgres database based
// on the configured RBAC rules.
func TestAccessPostgres(t *testing.T) {
	ctx := context.Background()
	testCtx := setupTestContext(ctx, t, withSelfHostedPostgres("postgres"))
	go testCtx.startHandlingConnections()

	tests := []struct {
		desc         string
		user         string
		role         string
		allowDbNames []string
		allowDbUsers []string
		dbName       string
		dbUser       string
		err          string
	}{
		{
			desc:         "has access to all database names and users",
			user:         "alice",
			role:         "admin",
			allowDbNames: []string{types.Wildcard},
			allowDbUsers: []string{types.Wildcard},
			dbName:       "postgres",
			dbUser:       "postgres",
		},
		{
			desc:         "has access to nothing",
			user:         "alice",
			role:         "admin",
			allowDbNames: []string{},
			allowDbUsers: []string{},
			dbName:       "postgres",
			dbUser:       "postgres",
			err:          "access to db denied",
		},
		{
			desc:         "no access to databases",
			user:         "alice",
			role:         "admin",
			allowDbNames: []string{},
			allowDbUsers: []string{types.Wildcard},
			dbName:       "postgres",
			dbUser:       "postgres",
			err:          "access to db denied",
		},
		{
			desc:         "no access to users",
			user:         "alice",
			role:         "admin",
			allowDbNames: []string{types.Wildcard},
			allowDbUsers: []string{},
			dbName:       "postgres",
			dbUser:       "postgres",
			err:          "access to db denied",
		},
		{
			desc:         "access allowed to specific user/database",
			user:         "alice",
			role:         "admin",
			allowDbNames: []string{"metrics"},
			allowDbUsers: []string{"alice"},
			dbName:       "metrics",
			dbUser:       "alice",
		},
		{
			desc:         "access denied to specific user/database",
			user:         "alice",
			role:         "admin",
			allowDbNames: []string{"metrics"},
			allowDbUsers: []string{"alice"},
			dbName:       "postgres",
			dbUser:       "postgres",
			err:          "access to db denied",
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			// Create user/role with the requested permissions.
			testCtx.createUserAndRole(ctx, t, test.user, test.role, test.allowDbUsers, test.allowDbNames)

			// Try to connect to the database as this user.
			pgConn, err := testCtx.postgresClient(ctx, test.user, "postgres", test.dbUser, test.dbName)
			if test.err != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), test.err)
				return
			}

			require.NoError(t, err)

			// Execute a query.
			result, err := pgConn.Exec(ctx, "select 1").ReadAll()
			require.NoError(t, err)
			require.Equal(t, []*pgconn.Result{postgres.TestQueryResponse}, result)

			// Disconnect.
			err = pgConn.Close(ctx)
			require.NoError(t, err)
		})
	}
}

// TestAccessMySQL verifies access scenarios to a MySQL database based
// on the configured RBAC rules.
func TestAccessMySQL(t *testing.T) {
	ctx := context.Background()
	testCtx := setupTestContext(ctx, t, withSelfHostedMySQL("mysql"))
	go testCtx.startHandlingConnections()

	tests := []struct {
		// desc is the test case description.
		desc string
		// user is the Teleport local user name the test will use.
		user string
		// role is the Teleport role name to create and assign to the user.
		role string
		// allowDbUsers is the role's list of allowed database users.
		allowDbUsers []string
		// dbUser is the database user to simulate connect as.
		dbUser string
		// err is the expected test case error.
		err string
	}{
		{
			desc:         "has access to all database users",
			user:         "alice",
			role:         "admin",
			allowDbUsers: []string{types.Wildcard},
			dbUser:       "root",
		},
		{
			desc:         "has access to nothing",
			user:         "alice",
			role:         "admin",
			allowDbUsers: []string{},
			dbUser:       "root",
			err:          "access to db denied",
		},
		{
			desc:         "access allowed to specific user",
			user:         "alice",
			role:         "admin",
			allowDbUsers: []string{"alice"},
			dbUser:       "alice",
		},
		{
			desc:         "access denied to specific user",
			user:         "alice",
			role:         "admin",
			allowDbUsers: []string{"alice"},
			dbUser:       "root",
			err:          "access to db denied",
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			// Create user/role with the requested permissions.
			testCtx.createUserAndRole(ctx, t, test.user, test.role, test.allowDbUsers, []string{types.Wildcard})

			// Try to connect to the database as this user.
			mysqlConn, err := testCtx.mysqlClient(test.user, "mysql", test.dbUser)
			if test.err != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), test.err)
				return
			}

			require.NoError(t, err)

			// Execute a query.
			result, err := mysqlConn.Execute("select 1")
			require.NoError(t, err)
			require.Equal(t, mysql.TestQueryResponse, result)

			// Disconnect.
			err = mysqlConn.Close()
			require.NoError(t, err)
		})
	}
}

// TestAccessRedis verifies access scenarios to a Redis database based
// on the configured RBAC rules.
func TestAccessRedis(t *testing.T) {
	ctx := context.Background()
	testCtx := setupTestContext(ctx, t, withSelfHostedRedis("redis"))
	go testCtx.startHandlingConnections()

	tests := []struct {
		// desc is the test case description.
		desc string
		// user is the Teleport local username the test will use.
		user string
		// role is the Teleport role name to create and assign to the user.
		role string
		// allowDbUsers is the role's list of allowed database users.
		allowDbUsers []string
		// dbUser is the database user to simulate connect as.
		dbUser string
		// err is the expected test case error.
		err string
	}{
		{
			desc:         "has access to all database users",
			user:         "alice",
			role:         "admin",
			allowDbUsers: []string{types.Wildcard},
			dbUser:       "root",
		},
		{
			desc:         "has access to nothing",
			user:         "alice",
			role:         "admin",
			allowDbUsers: []string{},
			dbUser:       "root",
			err:          "access to db denied",
		},
		{
			desc:         "access allowed to specific user",
			user:         "alice",
			role:         "admin",
			allowDbUsers: []string{"alice"},
			dbUser:       "alice",
		},
		{
			desc:         "access denied to specific user",
			user:         "alice",
			role:         "admin",
			allowDbUsers: []string{"alice"},
			dbUser:       "root",
			err:          "access to db denied",
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			// Create user/role with the requested permissions.
			testCtx.createUserAndRole(ctx, t, test.user, test.role, test.allowDbUsers, []string{types.Wildcard})

			ctx := context.Background()
			// Try to connect to the database as this user.
			redisClient, err := testCtx.redisClient(ctx, test.user, "redis", test.dbUser)
			if test.err != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), test.err)
				return
			}

			require.NoError(t, err)

			// Execute a query.
			result := redisClient.Echo(ctx, "ping")
			require.NoError(t, result.Err())
			require.Equal(t, "ping", result.Val())

			// Disconnect.
			err = redisClient.Close()
			require.NoError(t, err)
		})
	}
}

// TestMySQLBadHandshake verifies MySQL proxy can gracefully handle truncated
// client handshake messages.
func TestMySQLBadHandshake(t *testing.T) {
	ctx := context.Background()
	testCtx := setupTestContext(ctx, t, withSelfHostedMySQL("mysql"))
	go testCtx.startHandlingConnections()

	tests := []struct {
		name string
		data []byte
	}{
		{
			name: "short user name",
			data: []byte{0x8d, 0xae, 0xff, 0x49, 0x0, 0x0, 0x0, 0x1, 0x2d, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x61, 0x6c, 0x69, 0x63, 0x65},
		},
		{
			name: "short db name",
			data: []byte{0x8d, 0xae, 0xff, 0x49, 0x0, 0x0, 0x0, 0x1, 0x2d, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x61, 0x6c, 0x69, 0x63, 0x65, 0x0, 0x14, 0xce, 0x7, 0x50, 0x5d, 0x8c, 0xca, 0x17, 0xda, 0x1b, 0x60, 0xea, 0x9d, 0xa9, 0xc4, 0x7d, 0x83, 0x85, 0xa8, 0x7a, 0x96, 0x71, 0x77, 0x65, 0x31, 0x32},
		},
		{
			name: "short plugin name",
			data: []byte{0x8d, 0xae, 0xff, 0x49, 0x0, 0x0, 0x0, 0x1, 0x2d, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x61, 0x6c, 0x69, 0x63, 0x65, 0x0, 0x14, 0xce, 0x7, 0x50, 0x5d, 0x8c, 0xca, 0x17, 0xda, 0x1b, 0x60, 0xea, 0x9d, 0xa9, 0xc4, 0x7d, 0x83, 0x85, 0xa8, 0x7a, 0x96, 0x71, 0x77, 0x65, 0x31, 0x32, 0x33, 0x0, 0x6d, 0x79, 0x73, 0x71, 0x6c, 0x5f, 0x6e, 0x61, 0x74, 0x69, 0x76, 0x65, 0x5f, 0x70, 0x61, 0x73, 0x73},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Connect to MySQL proxy endpoint.
			conn, err := net.Dial("tcp", testCtx.mysqlListener.Addr().String())
			require.NoError(t, err)

			// Read initial handshake message.
			bytes := make([]byte, 1024)
			_, err = conn.Read(bytes)
			require.NoError(t, err)

			// Prepend header to the packet data.
			packet := append([]byte{
				byte(len(test.data)),
				byte(len(test.data) >> 8),
				byte(len(test.data) >> 16),
				0x1,
			}, test.data...)

			// Write handshake response packet.
			_, err = conn.Write(packet)
			require.NoError(t, err)

			err = conn.Close()
			require.NoError(t, err)
		})
	}
}

// TestAccessMySQLChangeUser verifies that COM_CHANGE_USER command is rejected.
func TestAccessMySQLChangeUser(t *testing.T) {
	ctx := context.Background()
	testCtx := setupTestContext(ctx, t, withSelfHostedMySQL("mysql"))
	go testCtx.startHandlingConnections()

	// Create user/role with the requested permissions.
	testCtx.createUserAndRole(ctx, t, "alice", "admin", []string{"alice"}, []string{types.Wildcard})

	// Connect to the database as this user.
	mysqlConn, err := testCtx.mysqlClient("alice", "mysql", "alice")
	require.NoError(t, err)

	// Send COM_CHANGE_USER command. The driver doesn't support it natively so
	// assemble the raw packet and send it which should be enough to test the
	// rejection logic.
	packet := []byte{
		0x05,                     // Payload length.
		0x00,                     // Payload length cont'd.
		0x00,                     // Payload length cont'd.
		0x00,                     // Sequence number.
		mysqllib.COM_CHANGE_USER, // Command type.
		'b',                      // Null-terminated string with new user name.
		'o',
		'b',
		0x00,
		// There would've been other fields in "real" packet but these will
		// do for the test to detect the command.
	}
	err = mysqlConn.WritePacket(packet)
	require.NoError(t, err)

	// Connection should've been closed so any attempt to use it should fail.
	_, err = mysqlConn.Execute("select 1")
	require.Error(t, err)
}

func TestMySQLCloseConnection(t *testing.T) {
	ctx := context.Background()
	testCtx := setupTestContext(ctx, t, withSelfHostedMySQL("mysql"))
	go testCtx.startHandlingConnections()

	// Create user/role with the requested permissions.
	testCtx.createUserAndRole(ctx, t, "alice", "admin", []string{"alice"}, []string{types.Wildcard})

	// Connect to the database as this user.
	mysqlConn, err := testCtx.mysqlClient("alice", "mysql", "alice")
	require.NoError(t, err)

	_, err = mysqlConn.Execute("select 1")
	require.NoError(t, err)

	// Close connection to DB proxy
	err = mysqlConn.Close()
	require.NoError(t, err)

	// DB proxy should close the DB connection and send COM_QUIT message.
	require.Eventually(t, func() bool {
		return testCtx.mysql["mysql"].db.ConnsClosed()
	}, 2*time.Second, 100*time.Millisecond)
}

// TestAccessRedisAUTHDefaultCmd checks if empty user can log in to Redis as default.
func TestAccessRedisAUTHDefaultCmd(t *testing.T) {
	ctx := context.Background()
	testCtx := setupTestContext(ctx, t, withSelfHostedRedis("redis", redis.TestServerPassword("123")))
	go testCtx.startHandlingConnections()

	// Create user/role with the requested permissions.
	testCtx.createUserAndRole(ctx, t, "alice", "admin", []string{"default"}, []string{types.Wildcard})

	// Connect to the database as this user.
	redisConn, err := testCtx.redisClient(ctx, "alice", "redis", "", redis.SkipPing(true))
	require.NoError(t, err)

	err = redisConn.Process(ctx, goredis.NewCmd(ctx, "AUTH", "123"))
	require.NoError(t, err)

	// Check if we can execute some commands
	resp := redisConn.Echo(ctx, "ping")
	require.NoError(t, resp.Err())
	require.Equal(t, "ping", resp.Val())

	err = redisConn.Close()
	require.NoError(t, err)
}

// TestAccessRedisAUTHCmd checks if AUTH command is verified against Teleport RBAC before is sent to Redis.
func TestAccessRedisAUTHCmd(t *testing.T) {
	ctx := context.Background()
	testCtx := setupTestContext(ctx, t, withSelfHostedRedis("redis"))
	go testCtx.startHandlingConnections()

	// Create user/role with the requested permissions.
	testCtx.createUserAndRole(ctx, t, "alice", "admin", []string{"alice"}, []string{types.Wildcard})

	// Connect to the database as this user.
	redisConn, err := testCtx.redisClient(ctx, "alice", "redis", "alice")
	require.NoError(t, err)

	err = redisConn.Process(ctx, goredis.NewCmd(ctx, "AUTH", "attacker", "secret-password"))
	require.Error(t, err)
	require.Contains(t, err.Error(), "ERR Teleport: failed to authenticate as")

	err = redisConn.Close()
	require.NoError(t, err)
}

// TestAccessMySQLServerPacket verifies some edge-cases related to reading
// wire packets sent by the MySQL server.
func TestAccessMySQLServerPacket(t *testing.T) {
	ctx := context.Background()
	testCtx := setupTestContext(ctx, t, withSelfHostedMySQL("mysql"))
	go testCtx.startHandlingConnections()

	// Create user/role with access permissions.
	testCtx.createUserAndRole(ctx, t, "alice", "admin", []string{"alice"}, []string{types.Wildcard})

	// Connect to the database as this user.
	mysqlConn, err := testCtx.mysqlClient("alice", "mysql", "alice")
	require.NoError(t, err)

	// Execute "show tables" command which will make the test server to reply
	// in a way that previously would cause our packet parsing logic to fail.
	_, err = mysqlConn.Execute("show tables")
	require.NoError(t, err)

	err = mysqlConn.Close()
	require.NoError(t, err)
}

// TestGCPRequireSSL tests connecting to GCP Cloud SQL Postgres and MySQL
// databases with an ephemeral client certificate.
func TestGCPRequireSSL(t *testing.T) {
	ctx := context.Background()
	user := "alice"
	testCtx := setupTestContext(ctx, t)
	testCtx.createUserAndRole(ctx, t, user, "admin", []string{types.Wildcard}, []string{types.Wildcard})

	// Generate ephemeral cert returned from mock GCP API.
	ephemeralCert, err := common.MakeTestClientTLSCert(common.TestClientConfig{
		AuthClient: testCtx.authClient,
		AuthServer: testCtx.authServer,
		Cluster:    testCtx.clusterName,
		Username:   user,
	})
	require.NoError(t, err)

	// Setup database servers for Postgres and MySQL with a mock GCP API that
	// will require SSL and return the ephemeral certificate created above.
	testCtx.server = testCtx.setupDatabaseServer(ctx, t, agentParams{
		Databases: []types.Database{
			withCloudSQLPostgres("postgres", cloudSQLAuthToken)(t, ctx, testCtx),
			withCloudSQLMySQLTLS("mysql", user, cloudSQLPassword)(t, ctx, testCtx),
		},
		GCPSQL: &cloud.GCPSQLAdminClientMock{
			EphemeralCert: ephemeralCert,
			DatabaseInstance: &sqladmin.DatabaseInstance{
				Settings: &sqladmin.Settings{
					IpConfiguration: &sqladmin.IpConfiguration{
						RequireSsl: true,
					},
				},
			},
		},
	})
	go testCtx.startHandlingConnections()

	// Try to connect to postgres.
	pgConn, err := testCtx.postgresClient(ctx, user, "postgres", "postgres", "postgres")
	require.NoError(t, err)

	// Execute a query.
	pgResult, err := pgConn.Exec(ctx, "select 1").ReadAll()
	require.NoError(t, err)
	require.Equal(t, []*pgconn.Result{postgres.TestQueryResponse}, pgResult)

	// Disconnect.
	err = pgConn.Close(ctx)
	require.NoError(t, err)

	// Try to connect to MySQL.
	mysqlConn, err := testCtx.mysqlClient(user, "mysql", user)
	require.NoError(t, err)

	// Execute a query.
	mysqlResult, err := mysqlConn.Execute("select 1")
	require.NoError(t, err)
	require.Equal(t, mysql.TestQueryResponse, mysqlResult)

	// Disconnect.
	err = mysqlConn.Close()
	require.NoError(t, err)
}

func init() {
	// Override SQL Server engine that is used normally with the test one
	// that mocks connection dial and Kerberos auth.
	common.RegisterEngine(newTestSQLServerEngine, defaults.ProtocolSQLServer)
}

func newTestSQLServerEngine(ec common.EngineConfig) common.Engine {
	return &sqlserver.Engine{
		EngineConfig: ec,
		Connector:    &sqlserver.TestConnector{},
	}
}

// TestAccessSQLServer verifies access scenarios to a SQL Server database.
func TestAccessSQLServer(t *testing.T) {
	ctx := context.Background()
	testCtx := setupTestContext(ctx, t, withSQLServer("sqlserver"))
	go testCtx.startHandlingConnections()

	tests := []struct {
		desc         string
		teleportUser string
		teleportRole string
		allowDbUsers []string
		dbUser       string
		err          string
	}{
		{
			desc:         "has access to all database users",
			teleportUser: "alice",
			teleportRole: "admin",
			allowDbUsers: []string{types.Wildcard},
			dbUser:       "root",
		},
		{
			desc:         "has access to nothing",
			teleportUser: "alice",
			teleportRole: "admin",
			allowDbUsers: []string{},
			dbUser:       "root",
			err:          "access to db denied",
		},
		{
			desc:         "access allowed to specific user",
			teleportUser: "alice",
			teleportRole: "admin",
			allowDbUsers: []string{"alice"},
			dbUser:       "alice",
		},
		{
			desc:         "access denied to specific user",
			teleportUser: "alice",
			teleportRole: "admin",
			allowDbUsers: []string{"alice"},
			dbUser:       "root",
			err:          "access to db denied",
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			// Create user/role with the requested permissions.
			testCtx.createUserAndRole(ctx, t, test.teleportUser, test.teleportRole, test.allowDbUsers, []string{types.Wildcard})

			// Try to connect to the database as this user.
			conn, proxy, err := testCtx.sqlServerClient(ctx, test.teleportUser, "sqlserver", test.dbUser, "master")
			if test.err != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), test.err)
				return
			}
			require.NoError(t, err)

			// Close connection and proxy.
			t.Cleanup(func() {
				require.NoError(t, conn.Close())
				require.NoError(t, proxy.Close())
			})
		})
	}
}

// TestAccessMongoDB verifies access scenarios to a MongoDB database based
// on the configured RBAC rules.
func TestAccessMongoDB(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		desc         string
		user         string
		role         string
		allowDbNames []string
		allowDbUsers []string
		dbName       string
		dbUser       string
		connectErr   string
		queryErr     string
	}{
		{
			desc:         "has access to all database names and users",
			user:         "alice",
			role:         "admin",
			allowDbNames: []string{types.Wildcard},
			allowDbUsers: []string{types.Wildcard},
			dbUser:       "admin",
			dbName:       "admin",
			connectErr:   "",
			queryErr:     "",
		},
		{
			desc:         "has access to nothing",
			user:         "alice",
			role:         "admin",
			allowDbNames: []string{},
			allowDbUsers: []string{},
			dbName:       "admin",
			dbUser:       "admin",
			connectErr:   "access to db denied",
			queryErr:     "",
		},
		{
			desc:         "no access to databases",
			user:         "alice",
			role:         "admin",
			allowDbNames: []string{""},
			allowDbUsers: []string{types.Wildcard},
			dbName:       "admin",
			dbUser:       "admin",
			connectErr:   "access to db denied",
			queryErr:     "",
		},
		{
			desc:         "no access to users",
			user:         "alice",
			role:         "admin",
			allowDbNames: []string{types.Wildcard},
			allowDbUsers: []string{},
			dbName:       "admin",
			dbUser:       "admin",
			connectErr:   "access to db denied",
			queryErr:     "",
		},
		{
			desc:         "access allowed to specific user and database",
			user:         "alice",
			role:         "admin",
			allowDbNames: []string{"admin"},
			allowDbUsers: []string{"alice"},
			dbName:       "admin",
			dbUser:       "alice",
			connectErr:   "",
			queryErr:     "",
		},
		{
			desc:         "access denied to specific user and database",
			user:         "alice",
			role:         "admin",
			allowDbNames: []string{"admin"},
			allowDbUsers: []string{"alice"},
			dbName:       "metrics",
			dbUser:       "alice",
			connectErr:   "",
			queryErr:     "access to db denied",
		},
	}

	// Each scenario is executed multiple times with different server/client
	// options to test things like legacy MongoDB servers and clients that
	// use compression.
	serverOpts := []struct {
		name string
		opts []mongodb.TestServerOption
	}{
		{
			name: "new server",
			opts: []mongodb.TestServerOption{},
		},
		{
			name: "old server",
			opts: []mongodb.TestServerOption{
				mongodb.TestServerWireVersion(wiremessage.OpmsgWireVersion - 1),
			},
		},
	}

	clientOpts := []struct {
		name string
		opts *options.ClientOptions
	}{
		{
			name: "client without compression",
			opts: options.Client(),
		},
		{
			name: "client with compression",
			opts: options.Client().SetCompressors([]string{"zlib"}),
		},
	}

	// Execute each scenario on both modern and legacy Mongo servers
	// to make sure legacy messages are also subject to RBAC.
	for _, test := range tests {
		for _, serverOpt := range serverOpts {
			for _, clientOpt := range clientOpts {
				t.Run(fmt.Sprintf("%v/%v/%v", serverOpt.name, clientOpt.name, test.desc), func(t *testing.T) {
					testCtx := setupTestContext(ctx, t, withSelfHostedMongo("mongo", serverOpt.opts...))
					go testCtx.startHandlingConnections()

					// Create user/role with the requested permissions.
					testCtx.createUserAndRole(ctx, t, test.user, test.role, test.allowDbUsers, test.allowDbNames)

					// Try to connect to the database as this user.
					mongoClient, err := testCtx.mongoClient(ctx, test.user, "mongo", test.dbUser, clientOpt.opts)
					t.Cleanup(func() {
						if mongoClient != nil {
							require.NoError(t, mongoClient.Disconnect(ctx))
						}
					})
					if test.connectErr != "" {
						require.Error(t, err)
						require.Contains(t, err.Error(), test.connectErr)
						return
					}
					require.NoError(t, err)

					// Execute a "find" command. Collection name doesn't matter currently.
					records, err := mongoClient.Database(test.dbName).Collection("test").Find(ctx, bson.M{})
					if test.queryErr != "" {
						require.Error(t, err)
						require.Contains(t, err.Error(), test.queryErr)
						return
					}
					require.NoError(t, err)
					require.NoError(t, records.Close(ctx))
				})
			}
		}
	}
}

// TestAccessDisabled makes sure database access can be disabled via modules.
func TestAccessDisabled(t *testing.T) {
	modules.SetTestModules(t, &modules.TestModules{
		TestFeatures: modules.Features{
			DB: false,
		},
	})

	ctx := context.Background()
	testCtx := setupTestContext(ctx, t, withSelfHostedPostgres("postgres"))
	go testCtx.startHandlingConnections()

	userName := "alice"
	roleName := "admin"
	dbUser := "postgres"
	dbName := "postgres"

	// Create user/role with the requested permissions.
	testCtx.createUserAndRole(ctx, t, userName, roleName, []string{types.Wildcard}, []string{types.Wildcard})

	// Try to connect to the database as this user.
	_, err := testCtx.postgresClient(ctx, userName, "postgres", dbUser, dbName)
	require.Error(t, err)
	require.Contains(t, err.Error(), "this Teleport cluster is not licensed for database access")
}

// TestPostgresInjectionDatabase makes sure Postgres connection is not
// susceptible to malicious database name injections.
func TestPostgresInjectionDatabase(t *testing.T) {
	ctx := context.Background()
	testCtx := setupTestContext(ctx, t, withSelfHostedPostgres("postgres"))
	go testCtx.startHandlingConnections()

	postgresServer := testCtx.postgres["postgres"].db

	// Make sure the role allows wildcard database users and names.
	testCtx.createUserAndRole(ctx, t, "alice", "admin", []string{types.Wildcard}, []string{types.Wildcard})

	// Connect and make sure connection parameters are as expected.
	psql, err := testCtx.postgresClient(ctx, "alice", "postgres", "alice", "test&user=bob")
	require.NoError(t, err)

	select {
	case p := <-postgresServer.ParametersCh():
		require.Equal(t, map[string]string{"user": "alice", "database": "test&user=bob"}, p)
	case <-time.After(time.Second):
		t.Fatal("didn't receive startup message parameters after 1s")
	}

	err = psql.Close(ctx)
	require.NoError(t, err)
}

// TestPostgresInjectionUser makes sure Postgres connection is not
// susceptible to malicious user name injections.
func TestPostgresInjectionUser(t *testing.T) {
	ctx := context.Background()
	testCtx := setupTestContext(ctx, t, withSelfHostedPostgres("postgres"))
	go testCtx.startHandlingConnections()

	postgresServer := testCtx.postgres["postgres"].db

	// Make sure the role allows wildcard database users and names.
	testCtx.createUserAndRole(ctx, t, "alice", "admin", []string{types.Wildcard}, []string{types.Wildcard})

	// Construct malicious username that simulates the connection string.
	user := fmt.Sprintf("alice@localhost:%v?database=prod&foo=", postgresServer.Port())

	// Connect and make sure startup parameters are as expected.
	psql, err := testCtx.postgresClient(ctx, "alice", "postgres", user, "test")
	require.NoError(t, err)

	select {
	case p := <-postgresServer.ParametersCh():
		require.Equal(t, map[string]string{"user": user, "database": "test"}, p)
	case <-time.After(time.Second):
		t.Fatal("didn't receive startup message parameters after 1s")
	}

	err = psql.Close(ctx)
	require.NoError(t, err)
}

// TestCompatibilityWithOldAgents verifies that older database agents where
// each database was represented as a DatabaseServer are supported.
//
// DELETE IN 9.0.
func TestCompatibilityWithOldAgents(t *testing.T) {
	ctx := context.Background()
	testCtx := setupTestContext(ctx, t)
	go testCtx.startProxy()

	postgresServer, err := postgres.NewTestServer(common.TestServerConfig{
		Name:       "postgres",
		AuthClient: testCtx.authClient,
	})
	require.NoError(t, err)
	go postgresServer.Serve()
	t.Cleanup(func() { postgresServer.Close() })

	database, err := types.NewDatabaseV3(types.Metadata{
		Name: "postgres",
	}, types.DatabaseSpecV3{
		Protocol: defaults.ProtocolPostgres,
		URI:      net.JoinHostPort("localhost", postgresServer.Port()),
	})
	require.NoError(t, err)
	databaseServer := testCtx.setupDatabaseServer(ctx, t, agentParams{
		Databases: []types.Database{database},
		GetServerInfoFn: func(database types.Database) func() (types.Resource, error) {
			return func() (types.Resource, error) {
				return types.NewDatabaseServerV3(types.Metadata{
					Name: database.GetName(),
				}, types.DatabaseServerSpecV3{
					Protocol: database.GetProtocol(),
					URI:      database.GetURI(),
					HostID:   testCtx.hostID,
					Hostname: constants.APIDomain,
				})
			}
		},
	})
	go func() {
		for conn := range testCtx.fakeRemoteSite.ProxyConn() {
			go databaseServer.HandleConnection(conn)
		}
	}()

	testCtx.createUserAndRole(ctx, t, "alice", "admin", []string{"postgres"}, []string{"postgres"})

	// Make sure we can connect successfully.
	psql, err := testCtx.postgresClient(ctx, "alice", "postgres", "postgres", "postgres")
	require.NoError(t, err)

	err = psql.Close(ctx)
	require.NoError(t, err)
}

func TestRedisGetSet(t *testing.T) {
	ctx := context.Background()
	testCtx := setupTestContext(ctx, t, withSelfHostedRedis("redis"))
	go testCtx.startHandlingConnections()

	// Create user/role with the requested permissions.
	testCtx.createUserAndRole(ctx, t, "alice", "admin", []string{types.Wildcard}, []string{types.Wildcard})

	// Try to connect to the database as this user.
	redisClient, err := testCtx.redisClient(ctx, "alice", "redis", "admin")
	require.NoError(t, err)

	// Execute a query.
	result := redisClient.Set(ctx, "key1", "123", 0)
	require.NoError(t, result.Err())

	getResult := redisClient.Get(ctx, "key1")
	require.NoError(t, getResult.Err())
	require.Equal(t, getResult.Val(), "123")

	// Disconnect.
	err = redisClient.Close()
	require.NoError(t, err)
}

func TestRedisPubSub(t *testing.T) {
	tests := []struct {
		name        string
		subscribeFn func(ctx context.Context, client *redis.Client) *goredis.PubSub
		verifyMsg   func(t *testing.T, msg *goredis.Message)
	}{
		{
			name: "subscribe",
			subscribeFn: func(ctx context.Context, client *redis.Client) *goredis.PubSub {
				return client.Subscribe(ctx, "foo")
			},
			verifyMsg: func(t *testing.T, msg *goredis.Message) {
				require.Equal(t, "foo", msg.Channel)
				require.Equal(t, "bar", msg.Payload)
			},
		},
		{
			name: "psubscribe",
			subscribeFn: func(ctx context.Context, client *redis.Client) *goredis.PubSub {
				return client.PSubscribe(ctx, "fo?")
			},
			verifyMsg: func(t *testing.T, msg *goredis.Message) {
				require.Equal(t, "foo", msg.Channel)
				require.Equal(t, "fo?", msg.Pattern)
				require.Equal(t, "bar", msg.Payload)
			},
		},
	}

	for _, tt := range tests {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			testCtx := setupTestContext(ctx, t, withSelfHostedRedis("redis"))
			go testCtx.startHandlingConnections()

			// Create user/role with the requested permissions.
			testCtx.createUserAndRole(ctx, t, "alice", "admin", []string{types.Wildcard}, []string{types.Wildcard})

			// Try to connect to the database as this user.
			redisClient, err := testCtx.redisClient(ctx, "alice", "redis", "admin")
			require.NoError(t, err)

			var fooSub *goredis.PubSub
			// Create a synchronisation channel between publisher and subscriber
			syncChan := make(chan bool)

			go func() {
				fooSub = tt.subscribeFn(ctx, redisClient)
				// If one of the checks fails the syncChan will be closed. If the main goroutine is waiting for a response
				// it will be unblocked, and it will fail the test.
				defer func() {
					fooSub.Close()
					close(syncChan)
				}()

				event, err := fooSub.Receive(ctx)
				require.NoError(t, err)
				require.IsType(t, &goredis.Subscription{}, event)
				syncChan <- true

				event, err = fooSub.Receive(ctx)
				require.NoError(t, err)

				msg, ok := event.(*goredis.Message)
				require.True(t, ok, "Redis message has a wrong type")
				tt.verifyMsg(t, msg)
				syncChan <- true
			}()

			// Wait for a subscription to be active.
			require.True(t, <-syncChan)
			err = redisClient.Publish(ctx, "foo", "bar").Err()
			require.NoError(t, err)
			// Wait for a message to be received in subscribed goroutine.
			require.True(t, <-syncChan)

			// Check if the connection is still active
			resp := redisClient.Echo(ctx, "ping")
			require.NoError(t, resp.Err())
			require.Equal(t, "ping", resp.Val())

			// Disconnect.
			err = redisClient.Close()
			require.NoError(t, err)
		})
	}
}

func TestRedisPipeline(t *testing.T) {
	ctx := context.Background()
	testCtx := setupTestContext(ctx, t, withSelfHostedRedis("redis"))
	go testCtx.startHandlingConnections()

	// Create user/role with the requested permissions.
	testCtx.createUserAndRole(ctx, t, "alice", "admin", []string{types.Wildcard}, []string{types.Wildcard})

	// Try to connect to the database as this user.
	redisClient, err := testCtx.redisClient(ctx, "alice", "redis", "admin")
	require.NoError(t, err)

	t.Cleanup(func() {
		err := redisClient.Close()
		require.NoError(t, err)
	})

	pipeliner := redisClient.Pipeline()
	t.Cleanup(func() {
		err = pipeliner.Close()
		require.NoError(t, err)
	})

	// Set multiple keys using pipelining.
	for i := 0; i < 10; i++ {
		err := pipeliner.Set(ctx, fmt.Sprintf("foo%d", i), i, 0).Err()
		require.NoError(t, err)
	}

	cmds, err := pipeliner.Exec(ctx)
	require.NoError(t, err)

	for _, cmd := range cmds {
		require.NoError(t, cmd.Err())
	}

	for i := 0; i < 10; i++ {
		err := pipeliner.Get(ctx, fmt.Sprintf("foo%d", i)).Err()
		require.NoError(t, err)
	}

	cmds, err = pipeliner.Exec(ctx)
	require.NoError(t, err)

	for i, cmd := range cmds {
		require.NoError(t, cmd.Err())
		require.Equal(t, fmt.Sprintf("foo%d", i), cmd.Args()[1])
	}
}

func TestRedisTransaction(t *testing.T) {
	ctx := context.Background()
	testCtx := setupTestContext(ctx, t, withSelfHostedRedis("redis"))
	go testCtx.startHandlingConnections()

	// Create user/role with the requested permissions.
	testCtx.createUserAndRole(ctx, t, "alice", "admin", []string{types.Wildcard}, []string{types.Wildcard})

	// Try to connect to the database as this user.
	redisClient, err := testCtx.redisClient(ctx, "alice", "redis", "admin")
	require.NoError(t, err)

	t.Cleanup(func() {
		err := redisClient.Close()
		require.NoError(t, err)
	})

	// Test below has been taken from go-redis documentation and modify: https://pkg.go.dev/github.com/go-redis/redis/v8#Client.Watch
	const maxRetries = 100

	// Increment transactional increments key using GET and SET commands.
	increment := func(key string) error {
		// Transactional function.
		txf := func(tx *goredis.Tx) error {
			// Get current value or zero.
			n, err := tx.Get(ctx, key).Int()
			if err != nil && err != goredis.Nil {
				return err
			}

			// Actual operation (local in optimistic lock).
			n++

			// Operation is committed only if the watched keys remain unchanged.
			_, err = tx.TxPipelined(ctx, func(pipe goredis.Pipeliner) error {
				pipe.Set(ctx, key, n, 0)
				return nil
			})
			return err
		}

		for i := 0; i < maxRetries; i++ {
			err := redisClient.Watch(ctx, txf, key)
			if err == nil {
				// Success.
				return nil
			}
			if err == goredis.TxFailedErr {
				// Optimistic lock lost. Retry.
				continue
			}
			// Return any other error.
			return err
		}

		return errors.New("increment reached maximum number of retries")
	}

	var wg sync.WaitGroup
	// use just 2 concurrent connections as we want to test our proxy/protocol behaviour not Redis concurrency.
	const concurrentConnections = 2

	// Create a channel for potential transaction errors, as testify require package cannot be used from a goroutine.
	asyncErrors := make(chan error, concurrentConnections)
	defer close(asyncErrors)

	for i := 0; i < concurrentConnections; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			if err := increment("counter"); err != nil {
				asyncErrors <- err
			}
		}()
	}
	wg.Wait()

	select {
	case err := <-asyncErrors:
		require.FailNow(t, "failed to increment counter", err)
	default:
	}

	n, err := redisClient.Get(ctx, "counter").Int()

	require.NoError(t, err)
	require.Equal(t, concurrentConnections, n)
}

func TestRedisNil(t *testing.T) {
	ctx := context.Background()
	testCtx := setupTestContext(ctx, t, withSelfHostedRedis("redis"))
	go testCtx.startHandlingConnections()

	// Create user/role with the requested permissions.
	testCtx.createUserAndRole(ctx, t, "alice", "admin", []string{types.Wildcard}, []string{types.Wildcard})

	// Try to connect to the database as this user.
	redisClient, err := testCtx.redisClient(ctx, "alice", "redis", "admin")
	require.NoError(t, err)

	// Get Redis to return nil. It should be parsed correctly as nil, not a Redis error.
	result := redisClient.Get(ctx, "keyDoesntExist")
	require.Equal(t, goredis.Nil, result.Err())

	// Disconnect.
	err = redisClient.Close()
	require.NoError(t, err)
}

type testContext struct {
	hostID         string
	clusterName    string
	tlsServer      *auth.TestTLSServer
	authServer     *auth.Server
	authClient     *auth.Client
	proxyServer    *ProxyServer
	mux            *multiplexer.Mux
	mysqlListener  net.Listener
	webListener    *multiplexer.WebListener
	fakeRemoteSite *reversetunnel.FakeRemoteSite
	server         *Server
	emitter        *eventstest.ChannelEmitter
	hostCA         types.CertAuthority
	// postgres is a collection of Postgres databases the test uses.
	postgres map[string]testPostgres
	// mysql is a collection of MySQL databases the test uses.
	mysql map[string]testMySQL
	// mongo is a collection of MongoDB databases the test uses.
	mongo map[string]testMongoDB
	// redis is a collection of Redis databases the test uses.
	redis map[string]testRedis
	// sqlServer is a collection of SQL Server databases the test uses.
	sqlServer map[string]testSQLServer
	// clock to override clock in tests.
	clock clockwork.FakeClock
}

// testPostgres represents a single proxied Postgres database.
type testPostgres struct {
	// db is the test Postgres database server.
	db *postgres.TestServer
	// resource is the resource representing this Postgres database.
	resource types.Database
}

// testMySQL represents a single proxied MySQL database.
type testMySQL struct {
	// db is the test MySQL database server.
	db *mysql.TestServer
	// resource is the resource representing this MySQL database.
	resource types.Database
}

// testMongoDB represents a single proxied MongoDB database.
type testMongoDB struct {
	// db is the test MongoDB database server.
	db *mongodb.TestServer
	// resource is the resource representing this MongoDB database.
	resource types.Database
}

// testRedis represents a single proxied Redis database.
type testRedis struct {
	// db is the test Redis database server.
	db *redis.TestServer
	// resource is the resource representing this Redis database.
	resource types.Database
}

// testSQLServer represents a single proxied SQL Server database.
type testSQLServer struct {
	// resource is the resource representing this SQL Server database
	resource types.Database
}

// startProxy starts all proxy services required to handle connections.
func (c *testContext) startProxy() {
	// Start multiplexer.
	go c.mux.Serve()
	// Start TLS multiplexer.
	go c.webListener.Serve()
	// Start database proxy server.
	go c.proxyServer.ServePostgres(c.mux.DB())
	// Start MySQL proxy server.
	go c.proxyServer.ServeMySQL(c.mysqlListener)
	// Start database TLS proxy server.
	go c.proxyServer.ServeTLS(c.webListener.DB())
}

// startHandlingConnections starts all services required to handle database
// client connections: multiplexer, proxy server Postgres/MySQL listeners
// and the database service agent.
func (c *testContext) startHandlingConnections() {
	// Start all proxy services.
	c.startProxy()
	// Start handling database client connections on the database server.
	for conn := range c.fakeRemoteSite.ProxyConn() {
		go c.server.HandleConnection(conn)
	}
}

// postgresClient connects to test Postgres through database access as a
// specified Teleport user and database account.
func (c *testContext) postgresClient(ctx context.Context, teleportUser, dbService, dbUser, dbName string) (*pgconn.PgConn, error) {
	return c.postgresClientWithAddr(ctx, c.mux.DB().Addr().String(), teleportUser, dbService, dbUser, dbName)
}

// postgresClientWithAddr is like postgresClient but allows to override connection address.
func (c *testContext) postgresClientWithAddr(ctx context.Context, address, teleportUser, dbService, dbUser, dbName string) (*pgconn.PgConn, error) {
	return postgres.MakeTestClient(ctx, common.TestClientConfig{
		AuthClient: c.authClient,
		AuthServer: c.authServer,
		Address:    address,
		Cluster:    c.clusterName,
		Username:   teleportUser,
		RouteToDatabase: tlsca.RouteToDatabase{
			ServiceName: dbService,
			Protocol:    defaults.ProtocolPostgres,
			Username:    dbUser,
			Database:    dbName,
		},
	})
}

// mysqlClient connects to test MySQL through database access as a specified
// Teleport user and database account.
func (c *testContext) mysqlClient(teleportUser, dbService, dbUser string) (*mysqlclient.Conn, error) {
	return c.mysqlClientWithAddr(c.mysqlListener.Addr().String(), teleportUser, dbService, dbUser)
}

// mysqlClientWithAddr is like mysqlClient but allows to override connection address.
func (c *testContext) mysqlClientWithAddr(address, teleportUser, dbService, dbUser string) (*mysqlclient.Conn, error) {
	return mysql.MakeTestClient(common.TestClientConfig{
		AuthClient: c.authClient,
		AuthServer: c.authServer,
		Address:    address,
		Cluster:    c.clusterName,
		Username:   teleportUser,
		RouteToDatabase: tlsca.RouteToDatabase{
			ServiceName: dbService,
			Protocol:    defaults.ProtocolMySQL,
			Username:    dbUser,
		},
	})
}

// mongoClient connects to test MongoDB through database access as a
// specified Teleport user and database account.
func (c *testContext) mongoClient(ctx context.Context, teleportUser, dbService, dbUser string, opts ...*options.ClientOptions) (*mongo.Client, error) {
	return c.mongoClientWithAddr(ctx, c.webListener.Addr().String(), teleportUser, dbService, dbUser, opts...)
}

// mongoClientWithAddr is like mongoClient but allows overriding connection address.
func (c *testContext) mongoClientWithAddr(ctx context.Context, address, teleportUser, dbService, dbUser string, opts ...*options.ClientOptions) (*mongo.Client, error) {
	return mongodb.MakeTestClient(ctx, common.TestClientConfig{
		AuthClient: c.authClient,
		AuthServer: c.authServer,
		Address:    address,
		Cluster:    c.clusterName,
		Username:   teleportUser,
		RouteToDatabase: tlsca.RouteToDatabase{
			ServiceName: dbService,
			Protocol:    defaults.ProtocolMongoDB,
			Username:    dbUser,
		},
	}, opts...)
}

// redisClient connects to test Redis through database access as a specified Teleport user and database account.
func (c *testContext) redisClient(ctx context.Context, teleportUser, dbService, dbUser string, opts ...redis.ClientOptions) (*redis.Client, error) {
	return c.redisClientWithAddr(ctx, c.webListener.Addr().String(), teleportUser, dbService, dbUser, opts...)
}

// redisClientWithAddr is like redisClient but allows overriding connection address.
func (c *testContext) redisClientWithAddr(ctx context.Context, proxyAddress, teleportUser, dbService, dbUser string, opts ...redis.ClientOptions) (*redis.Client, error) {
	return redis.MakeTestClient(ctx, common.TestClientConfig{
		AuthClient: c.authClient,
		AuthServer: c.authServer,
		Address:    proxyAddress,
		Cluster:    c.clusterName,
		Username:   teleportUser,
		RouteToDatabase: tlsca.RouteToDatabase{
			ServiceName: dbService,
			Protocol:    defaults.ProtocolRedis,
			Username:    dbUser,
		},
	}, opts...)
}

// sqlServerClient connects to the specified SQL Server address.
func (c *testContext) sqlServerClient(ctx context.Context, teleportUser, dbService, dbUser, dbName string) (*mssql.Conn, *alpnproxy.LocalProxy, error) {
	route := tlsca.RouteToDatabase{
		ServiceName: dbService,
		Protocol:    defaults.ProtocolSQLServer,
		Username:    dbUser,
		Database:    dbName,
	}

	// SQL Server clients always connect via the local proxy so start it first.
	proxy, err := c.startLocalALPNProxy(ctx, c.webListener.Addr().String(), teleportUser, route)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	// Client connects to the local proxy.
	client, err := sqlserver.MakeTestClient(ctx, common.TestClientConfig{
		AuthClient:      c.authClient,
		AuthServer:      c.authServer,
		Address:         proxy.GetAddr(),
		Cluster:         c.clusterName,
		Username:        teleportUser,
		RouteToDatabase: route,
	})
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	return client, proxy, nil
}

// startLocalALPNProxy starts local ALPN proxy for the specified database.
func (c *testContext) startLocalALPNProxy(ctx context.Context, proxyAddr, teleportUser string, route tlsca.RouteToDatabase) (*alpnproxy.LocalProxy, error) {
	key, err := client.NewKey()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	clientCert, err := c.authServer.GenerateDatabaseTestCert(
		auth.DatabaseTestCertRequest{
			PublicKey:       key.Pub,
			Cluster:         c.clusterName,
			Username:        teleportUser,
			RouteToDatabase: route,
		})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	tlsCert, err := tls.X509KeyPair(clientCert, key.Priv)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, trace.Wrap(err)
	}

	proto, err := alpncommon.ToALPNProtocol(route.Protocol)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	proxy, err := alpnproxy.NewLocalProxy(alpnproxy.LocalProxyConfig{
		RemoteProxyAddr:    proxyAddr,
		Protocol:           proto,
		InsecureSkipVerify: true,
		Listener:           listener,
		ParentContext:      ctx,
		Certs:              []tls.Certificate{tlsCert},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	go proxy.Start(ctx)

	return proxy, nil
}

// createUserAndRole creates Teleport user and role with specified names
// and allowed database users/names properties.
func (c *testContext) createUserAndRole(ctx context.Context, t *testing.T, userName, roleName string, dbUsers, dbNames []string) (types.User, types.Role) {
	user, role, err := auth.CreateUserAndRole(c.tlsServer.Auth(), userName, []string{roleName})
	require.NoError(t, err)
	role.SetDatabaseUsers(types.Allow, dbUsers)
	role.SetDatabaseNames(types.Allow, dbNames)
	err = c.tlsServer.Auth().UpsertRole(ctx, role)
	require.NoError(t, err)
	return user, role
}

// makeTLSConfig returns tls configuration for the test's tls listener.
func (c *testContext) makeTLSConfig(t *testing.T) *tls.Config {
	creds, err := utils.GenerateSelfSignedCert([]string{"localhost"})
	require.NoError(t, err)
	cert, err := tls.X509KeyPair(creds.Cert, creds.PrivateKey)
	require.NoError(t, err)
	conf := utils.TLSConfig(nil)
	conf.Certificates = append(conf.Certificates, cert)
	conf.ClientAuth = tls.VerifyClientCertIfGiven
	conf.ClientCAs, err = auth.ClientCertPool(c.authServer, c.clusterName)
	require.NoError(t, err)
	return conf
}

// Close closes all resources associated with the test context.
func (c *testContext) Close() error {
	var errors []error
	if c.mux != nil {
		errors = append(errors, c.mux.Close())
	}
	if c.mysqlListener != nil {
		errors = append(errors, c.mysqlListener.Close())
	}
	if c.webListener != nil {
		errors = append(errors, c.webListener.Close())
	}
	if c.server != nil {
		errors = append(errors, c.server.Close())
	}
	return trace.NewAggregate(errors...)
}

func init() {
	// Override database agents shuffle behavior to ensure they're always
	// tried in the same order during tests. Used for HA tests.
	SetShuffleFunc(ShuffleSort)
}

func setupTestContext(ctx context.Context, t *testing.T, withDatabases ...withDatabaseOption) *testContext {
	testCtx := &testContext{
		clusterName: "root.example.com",
		hostID:      uuid.New().String(),
		postgres:    make(map[string]testPostgres),
		mysql:       make(map[string]testMySQL),
		mongo:       make(map[string]testMongoDB),
		redis:       make(map[string]testRedis),
		sqlServer:   make(map[string]testSQLServer),
		clock:       clockwork.NewFakeClockAt(time.Now()),
	}
	t.Cleanup(func() { testCtx.Close() })

	// Create and start test auth server.
	authServer, err := auth.NewTestAuthServer(auth.TestAuthServerConfig{
		Clock:       clockwork.NewFakeClockAt(time.Now()),
		ClusterName: testCtx.clusterName,
		Dir:         t.TempDir(),
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, authServer.Close()) })

	testCtx.tlsServer, err = authServer.NewTestTLSServer()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, testCtx.tlsServer.Close()) })

	testCtx.authServer = testCtx.tlsServer.Auth()

	// Create multiplexer.
	listener, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)
	testCtx.mux, err = multiplexer.New(multiplexer.Config{
		ID:                  "test",
		Listener:            listener,
		EnableProxyProtocol: true,
	})
	require.NoError(t, err)

	// Setup TLS listener.
	testCtx.webListener, err = multiplexer.NewWebListener(multiplexer.WebListenerConfig{
		Listener: tls.NewListener(testCtx.mux.TLS(), testCtx.makeTLSConfig(t)),
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, testCtx.webListener.Close()) })

	// Create MySQL proxy listener.
	testCtx.mysqlListener, err = net.Listen("tcp", "localhost:0")
	require.NoError(t, err)

	// Use sync recording to not involve the uploader.
	recConfig, err := authServer.AuthServer.GetSessionRecordingConfig(ctx)
	require.NoError(t, err)
	recConfig.SetMode(types.RecordAtNodeSync)
	err = authServer.AuthServer.SetSessionRecordingConfig(ctx, recConfig)
	require.NoError(t, err)

	// Auth client for database service.
	testCtx.authClient, err = testCtx.tlsServer.NewClient(auth.TestServerID(types.RoleDatabase, testCtx.hostID))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, testCtx.authClient.Close()) })

	testCtx.hostCA, err = testCtx.authClient.GetCertAuthority(ctx, types.CertAuthID{Type: types.HostCA, DomainName: testCtx.clusterName}, false)
	require.NoError(t, err)

	// Auth client, lock watcher and authorizer for database proxy.
	proxyAuthClient, err := testCtx.tlsServer.NewClient(auth.TestBuiltin(types.RoleProxy))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, proxyAuthClient.Close()) })

	proxyLockWatcher, err := services.NewLockWatcher(ctx, services.LockWatcherConfig{
		ResourceWatcherConfig: services.ResourceWatcherConfig{
			Component: teleport.ComponentProxy,
			Client:    proxyAuthClient,
		},
	})
	require.NoError(t, err)
	proxyAuthorizer, err := auth.NewAuthorizer(testCtx.clusterName, proxyAuthClient, proxyLockWatcher)
	require.NoError(t, err)

	// TLS config for database proxy and database service.
	serverIdentity, err := auth.NewServerIdentity(authServer.AuthServer, testCtx.hostID, types.RoleDatabase)
	require.NoError(t, err)
	tlsConfig, err := serverIdentity.TLSConfig(nil)
	require.NoError(t, err)

	// Set up database servers used by this test.
	var databases []types.Database
	for _, withDatabase := range withDatabases {
		databases = append(databases, withDatabase(t, ctx, testCtx))
	}

	// Establish fake reversetunnel b/w database proxy and database service.
	testCtx.fakeRemoteSite = reversetunnel.NewFakeRemoteSite(testCtx.clusterName, proxyAuthClient)
	t.Cleanup(func() { require.NoError(t, testCtx.fakeRemoteSite.Close()) })
	tunnel := &reversetunnel.FakeServer{
		Sites: []reversetunnel.RemoteSite{
			testCtx.fakeRemoteSite,
		},
	}
	// Empty config means no limit.
	connLimiter, err := limiter.NewLimiter(limiter.Config{})
	require.NoError(t, err)

	// Create test audit events emitter.
	testCtx.emitter = eventstest.NewChannelEmitter(100)

	// Create database proxy server.
	testCtx.proxyServer, err = NewProxyServer(ctx, ProxyServerConfig{
		AuthClient:  proxyAuthClient,
		AccessPoint: proxyAuthClient,
		Authorizer:  proxyAuthorizer,
		Tunnel:      tunnel,
		TLSConfig:   tlsConfig,
		Limiter:     connLimiter,
		Emitter:     testCtx.emitter,
		Clock:       testCtx.clock,
		ServerID:    "proxy-server",
		LockWatcher: proxyLockWatcher,
	})
	require.NoError(t, err)

	// Create database service agent.
	if len(databases) > 0 {
		testCtx.server = testCtx.setupDatabaseServer(ctx, t, agentParams{
			Databases: databases,
		})
	}

	return testCtx
}

// agentParams combines parameters for creating database agent servers in tests.
type agentParams struct {
	// Databases is a list of statically registered databases.
	Databases types.Databases
	// HostID is an optional host id.
	HostID string
	// ResourceMatchers are optional database resource matchers.
	ResourceMatchers []services.ResourceMatcher
	// GetServerInfoFn overrides heartbeat's server info function.
	GetServerInfoFn func(database types.Database) func() (types.Resource, error)
	// OnReconcile sets database resource reconciliation callback.
	OnReconcile func(types.Databases)
	// NoStart indicates server should not be started.
	NoStart bool
	// GCPSQL defines the GCP Cloud SQL mock to use for GCP API calls.
	GCPSQL *cloud.GCPSQLAdminClientMock
	// OnHeartbeat defines a heartbeat function that generates heartbeat events.
	OnHeartbeat func(error)
}

func (p *agentParams) setDefaults(c *testContext) {
	if p.HostID == "" {
		p.HostID = c.hostID
	}
	if p.GCPSQL == nil {
		p.GCPSQL = &cloud.GCPSQLAdminClientMock{
			DatabaseInstance: &sqladmin.DatabaseInstance{
				Settings: &sqladmin.Settings{
					IpConfiguration: &sqladmin.IpConfiguration{
						RequireSsl: false,
					},
				},
			},
		}
	}
}

func (c *testContext) setupDatabaseServer(ctx context.Context, t *testing.T, p agentParams) *Server {
	p.setDefaults(c)

	// Database service credentials.
	serverIdentity, err := auth.NewServerIdentity(c.authServer, p.HostID, types.RoleDatabase)
	require.NoError(t, err)
	tlsConfig, err := serverIdentity.TLSConfig(nil)
	require.NoError(t, err)

	// Lock watcher and authorizer for database service.
	lockWatcher, err := services.NewLockWatcher(ctx, services.LockWatcherConfig{
		ResourceWatcherConfig: services.ResourceWatcherConfig{
			Component: teleport.ComponentDatabase,
			Client:    c.authClient,
		},
	})
	require.NoError(t, err)
	dbAuthorizer, err := auth.NewAuthorizer(c.clusterName, c.authClient, lockWatcher)
	require.NoError(t, err)

	// Create test database auth tokens generator.
	testAuth, err := newTestAuth(common.AuthConfig{
		AuthClient: c.authClient,
		Clients:    &common.TestCloudClients{},
		Clock:      c.clock,
	})
	require.NoError(t, err)

	// Create default limiter.
	connLimiter, err := limiter.NewLimiter(limiter.Config{})
	require.NoError(t, err)

	// Create database server agent itself.
	server, err := New(ctx, Config{
		Clock:            clockwork.NewFakeClockAt(time.Now()),
		DataDir:          t.TempDir(),
		AuthClient:       c.authClient,
		AccessPoint:      c.authClient,
		StreamEmitter:    c.authClient,
		Authorizer:       dbAuthorizer,
		Hostname:         constants.APIDomain,
		HostID:           p.HostID,
		TLSConfig:        tlsConfig,
		Limiter:          connLimiter,
		Auth:             testAuth,
		Databases:        p.Databases,
		OnHeartbeat:      p.OnHeartbeat,
		ResourceMatchers: p.ResourceMatchers,
		GetServerInfoFn:  p.GetServerInfoFn,
		GetRotation: func(types.SystemRole) (*types.Rotation, error) {
			return &types.Rotation{}, nil
		},
		NewAudit: func(common.AuditConfig) (common.Audit, error) {
			// Use the same audit logger implementation but substitute the
			// underlying emitter so events can be tracked in tests.
			return common.NewAudit(common.AuditConfig{
				Emitter: c.emitter,
			})
		},
		CADownloader: &fakeDownloader{
			cert: []byte(fixtures.TLSCACertPEM),
		},
		OnReconcile: p.OnReconcile,
		LockWatcher: lockWatcher,
		CloudClients: &common.TestCloudClients{
			STS:      &cloud.STSMock{},
			RDS:      &cloud.RDSMock{},
			Redshift: &cloud.RedshiftMock{},
			IAM:      &cloud.IAMMock{},
			GCPSQL:   p.GCPSQL,
		},
	})
	require.NoError(t, err)

	if !p.NoStart {
		require.NoError(t, server.Start(ctx))
		require.NoError(t, server.ForceHeartbeat())
	}

	return server
}

type withDatabaseOption func(t *testing.T, ctx context.Context, testCtx *testContext) types.Database

func withSelfHostedPostgres(name string) withDatabaseOption {
	return func(t *testing.T, ctx context.Context, testCtx *testContext) types.Database {
		postgresServer, err := postgres.NewTestServer(common.TestServerConfig{
			Name:       name,
			AuthClient: testCtx.authClient,
		})
		require.NoError(t, err)
		go postgresServer.Serve()
		t.Cleanup(func() { postgresServer.Close() })
		database, err := types.NewDatabaseV3(types.Metadata{
			Name: name,
		}, types.DatabaseSpecV3{
			Protocol:      defaults.ProtocolPostgres,
			URI:           net.JoinHostPort("localhost", postgresServer.Port()),
			DynamicLabels: dynamicLabels,
		})
		require.NoError(t, err)
		testCtx.postgres[name] = testPostgres{
			db:       postgresServer,
			resource: database,
		}
		return database
	}
}

func withRDSPostgres(name, authToken string) withDatabaseOption {
	return func(t *testing.T, ctx context.Context, testCtx *testContext) types.Database {
		postgresServer, err := postgres.NewTestServer(common.TestServerConfig{
			Name:       name,
			AuthClient: testCtx.authClient,
			AuthToken:  authToken,
		})
		require.NoError(t, err)
		go postgresServer.Serve()
		t.Cleanup(func() { postgresServer.Close() })
		database, err := types.NewDatabaseV3(types.Metadata{
			Name: name,
		}, types.DatabaseSpecV3{
			Protocol:      defaults.ProtocolPostgres,
			URI:           net.JoinHostPort("localhost", postgresServer.Port()),
			DynamicLabels: dynamicLabels,
			AWS: types.AWS{
				Region: testAWSRegion,
			},
			// Set CA cert, otherwise we will attempt to download RDS roots.
			CACert: string(testCtx.hostCA.GetActiveKeys().TLS[0].Cert),
		})
		require.NoError(t, err)
		testCtx.postgres[name] = testPostgres{
			db:       postgresServer,
			resource: database,
		}
		return database
	}
}

func withRedshiftPostgres(name, authToken string) withDatabaseOption {
	return func(t *testing.T, ctx context.Context, testCtx *testContext) types.Database {
		postgresServer, err := postgres.NewTestServer(common.TestServerConfig{
			Name:       name,
			AuthClient: testCtx.authClient,
			AuthToken:  authToken,
		})
		require.NoError(t, err)
		go postgresServer.Serve()
		t.Cleanup(func() { postgresServer.Close() })
		database, err := types.NewDatabaseV3(types.Metadata{
			Name: name,
		}, types.DatabaseSpecV3{
			Protocol:      defaults.ProtocolPostgres,
			URI:           net.JoinHostPort("localhost", postgresServer.Port()),
			DynamicLabels: dynamicLabels,
			AWS: types.AWS{
				Region:   testAWSRegion,
				Redshift: types.Redshift{ClusterID: "redshift-cluster-1"},
			},
			// Set CA cert, otherwise we will attempt to download Redshift roots.
			CACert: string(testCtx.hostCA.GetActiveKeys().TLS[0].Cert),
		})
		require.NoError(t, err)
		testCtx.postgres[name] = testPostgres{
			db:       postgresServer,
			resource: database,
		}
		return database
	}
}

func withCloudSQLPostgres(name, authToken string) withDatabaseOption {
	return func(t *testing.T, ctx context.Context, testCtx *testContext) types.Database {
		postgresServer, err := postgres.NewTestServer(common.TestServerConfig{
			Name:       name,
			AuthClient: testCtx.authClient,
			AuthToken:  authToken,
			// Cloud SQL presented certificate must have <project-id>:<instance-id>
			// in its CN.
			CN: "project-1:instance-1",
		})
		require.NoError(t, err)
		go postgresServer.Serve()
		t.Cleanup(func() { postgresServer.Close() })
		database, err := types.NewDatabaseV3(types.Metadata{
			Name: name,
		}, types.DatabaseSpecV3{
			Protocol:      defaults.ProtocolPostgres,
			URI:           net.JoinHostPort("localhost", postgresServer.Port()),
			DynamicLabels: dynamicLabels,
			GCP: types.GCPCloudSQL{
				ProjectID:  "project-1",
				InstanceID: "instance-1",
			},
			// Set CA cert to pass cert validation.
			CACert: string(testCtx.hostCA.GetActiveKeys().TLS[0].Cert),
		})
		require.NoError(t, err)
		testCtx.postgres[name] = testPostgres{
			db:       postgresServer,
			resource: database,
		}
		return database
	}
}

func withAzurePostgres(name, authToken string) withDatabaseOption {
	return func(t *testing.T, ctx context.Context, testCtx *testContext) types.Database {
		postgresServer, err := postgres.NewTestServer(common.TestServerConfig{
			Name:       name,
			AuthClient: testCtx.authClient,
			AuthToken:  authToken,
		})
		require.NoError(t, err)
		go postgresServer.Serve()
		t.Cleanup(func() { postgresServer.Close() })
		database, err := types.NewDatabaseV3(types.Metadata{
			Name: name,
		}, types.DatabaseSpecV3{
			Protocol:      defaults.ProtocolPostgres,
			URI:           net.JoinHostPort("localhost", postgresServer.Port()),
			DynamicLabels: dynamicLabels,
			Azure: types.Azure{
				Name: name,
			},
			// Set CA cert, otherwise we will attempt to download RDS roots.
			CACert: string(testCtx.hostCA.GetActiveKeys().TLS[0].Cert),
		})
		require.NoError(t, err)
		testCtx.postgres[name] = testPostgres{
			db:       postgresServer,
			resource: database,
		}
		return database
	}
}

func withSelfHostedMySQL(name string) withDatabaseOption {
	return func(t *testing.T, ctx context.Context, testCtx *testContext) types.Database {
		mysqlServer, err := mysql.NewTestServer(common.TestServerConfig{
			Name:       name,
			AuthClient: testCtx.authClient,
		})
		require.NoError(t, err)
		go mysqlServer.Serve()
		t.Cleanup(func() {
			require.NoError(t, mysqlServer.Close())
		})
		database, err := types.NewDatabaseV3(types.Metadata{
			Name: name,
		}, types.DatabaseSpecV3{
			Protocol:      defaults.ProtocolMySQL,
			URI:           net.JoinHostPort("localhost", mysqlServer.Port()),
			DynamicLabels: dynamicLabels,
		})
		require.NoError(t, err)
		testCtx.mysql[name] = testMySQL{
			db:       mysqlServer,
			resource: database,
		}
		return database
	}
}

func withRDSMySQL(name, authUser, authToken string) withDatabaseOption {
	return func(t *testing.T, ctx context.Context, testCtx *testContext) types.Database {
		mysqlServer, err := mysql.NewTestServer(common.TestServerConfig{
			Name:       name,
			AuthClient: testCtx.authClient,
			AuthUser:   authUser,
			AuthToken:  authToken,
		})
		require.NoError(t, err)
		go mysqlServer.Serve()
		t.Cleanup(func() { mysqlServer.Close() })
		database, err := types.NewDatabaseV3(types.Metadata{
			Name: name,
		}, types.DatabaseSpecV3{
			Protocol:      defaults.ProtocolMySQL,
			URI:           net.JoinHostPort("localhost", mysqlServer.Port()),
			DynamicLabels: dynamicLabels,
			AWS: types.AWS{
				Region: testAWSRegion,
			},
			// Set CA cert, otherwise we will attempt to download RDS roots.
			CACert: string(testCtx.hostCA.GetActiveKeys().TLS[0].Cert),
		})
		require.NoError(t, err)
		testCtx.mysql[name] = testMySQL{
			db:       mysqlServer,
			resource: database,
		}
		return database
	}
}

func withCloudSQLMySQL(name, authUser, authToken string) withDatabaseOption {
	return func(t *testing.T, ctx context.Context, testCtx *testContext) types.Database {
		mysqlServer, err := mysql.NewTestServer(common.TestServerConfig{
			Name:       name,
			AuthClient: testCtx.authClient,
			AuthUser:   authUser,
			AuthToken:  authToken,
			// Cloud SQL presented certificate must have <project-id>:<instance-id>
			// in its CN.
			CN: "project-1:instance-1",
		})
		require.NoError(t, err)
		go mysqlServer.Serve()
		t.Cleanup(func() { mysqlServer.Close() })
		database, err := types.NewDatabaseV3(types.Metadata{
			Name: name,
		}, types.DatabaseSpecV3{
			Protocol:      defaults.ProtocolMySQL,
			URI:           net.JoinHostPort("localhost", mysqlServer.Port()),
			DynamicLabels: dynamicLabels,
			GCP: types.GCPCloudSQL{
				ProjectID:  "project-1",
				InstanceID: "instance-1",
			},
			// Set CA cert to pass cert validation.
			CACert: string(testCtx.hostCA.GetActiveKeys().TLS[0].Cert),
		})
		require.NoError(t, err)
		testCtx.mysql[name] = testMySQL{
			db:       mysqlServer,
			resource: database,
		}
		return database
	}
}

// withCloudSQLMySQLTLS creates a test MySQL server that simulates GCP Cloud SQL
// and requires client authentication using an ephemeral client certificate.
func withCloudSQLMySQLTLS(name, authUser, authToken string) withDatabaseOption {
	return func(t *testing.T, ctx context.Context, testCtx *testContext) types.Database {
		mysqlServer, err := mysql.NewTestServer(common.TestServerConfig{
			Name:       name,
			AuthClient: testCtx.authClient,
			AuthUser:   authUser,
			AuthToken:  authToken,
			// Cloud SQL presented certificate must have <project-id>:<instance-id>
			// in its CN.
			CN: "project-1:instance-1",
			// Enable TLS listener.
			ListenTLS: true,
		})
		require.NoError(t, err)
		go mysqlServer.Serve()
		t.Cleanup(func() { mysqlServer.Close() })
		database, err := types.NewDatabaseV3(types.Metadata{
			Name: name,
		}, types.DatabaseSpecV3{
			Protocol:      defaults.ProtocolMySQL,
			URI:           net.JoinHostPort("localhost", mysqlServer.Port()),
			DynamicLabels: dynamicLabels,
			GCP: types.GCPCloudSQL{
				ProjectID:  "project-1",
				InstanceID: "instance-1",
			},
			// Set CA cert to pass cert validation.
			CACert: string(testCtx.hostCA.GetActiveKeys().TLS[0].Cert),
		})
		require.NoError(t, err)
		testCtx.mysql[name] = testMySQL{
			db:       mysqlServer,
			resource: database,
		}
		return database
	}
}

func withAzureMySQL(name, authUser, authToken string) withDatabaseOption {
	return func(t *testing.T, ctx context.Context, testCtx *testContext) types.Database {
		mysqlServer, err := mysql.NewTestServer(common.TestServerConfig{
			Name:       name,
			AuthClient: testCtx.authClient,
			AuthUser:   authUser,
			AuthToken:  authToken,
		})
		require.NoError(t, err)
		go mysqlServer.Serve()
		t.Cleanup(func() { mysqlServer.Close() })
		database, err := types.NewDatabaseV3(types.Metadata{
			Name: name,
		}, types.DatabaseSpecV3{
			Protocol:      defaults.ProtocolMySQL,
			URI:           net.JoinHostPort("localhost", mysqlServer.Port()),
			DynamicLabels: dynamicLabels,
			Azure: types.Azure{
				Name: name,
			},
			// Set CA cert, otherwise we will attempt to download RDS roots.
			CACert: string(testCtx.hostCA.GetActiveKeys().TLS[0].Cert),
		})
		require.NoError(t, err)
		testCtx.mysql[name] = testMySQL{
			db:       mysqlServer,
			resource: database,
		}
		return database
	}
}

func withSelfHostedMongo(name string, opts ...mongodb.TestServerOption) withDatabaseOption {
	return func(t *testing.T, ctx context.Context, testCtx *testContext) types.Database {
		mongoServer, err := mongodb.NewTestServer(common.TestServerConfig{
			Name:       name,
			AuthClient: testCtx.authClient,
		}, opts...)
		require.NoError(t, err)
		go mongoServer.Serve()
		t.Cleanup(func() { mongoServer.Close() })
		database, err := types.NewDatabaseV3(types.Metadata{
			Name: name,
		}, types.DatabaseSpecV3{
			Protocol:      defaults.ProtocolMongoDB,
			URI:           net.JoinHostPort("localhost", mongoServer.Port()),
			DynamicLabels: dynamicLabels,
		})
		require.NoError(t, err)
		testCtx.mongo[name] = testMongoDB{
			db:       mongoServer,
			resource: database,
		}
		return database
	}
}

func withSelfHostedRedis(name string, opts ...redis.TestServerOption) withDatabaseOption {
	return func(t *testing.T, ctx context.Context, testCtx *testContext) types.Database {
		redisServer, err := redis.NewTestServer(t, common.TestServerConfig{
			Name:       name,
			AuthClient: testCtx.authClient,
		}, opts...)
		require.NoError(t, err)

		database, err := types.NewDatabaseV3(types.Metadata{
			Name: name,
		}, types.DatabaseSpecV3{
			Protocol:      defaults.ProtocolRedis,
			URI:           fmt.Sprintf("rediss://%s", net.JoinHostPort("localhost", redisServer.Port())),
			DynamicLabels: dynamicLabels,
		})
		require.NoError(t, err)
		testCtx.redis[name] = testRedis{
			db:       redisServer,
			resource: database,
		}
		return database
	}
}

func withSQLServer(name string) withDatabaseOption {
	return func(t *testing.T, ctx context.Context, testCtx *testContext) types.Database {
		database, err := types.NewDatabaseV3(types.Metadata{
			Name: name,
		}, types.DatabaseSpecV3{
			Protocol: defaults.ProtocolSQLServer,
			URI:      "localhost:1433", // URI doesn't matter as tests aren't actually going to dial it.
		})
		require.NoError(t, err)
		testCtx.sqlServer[name] = testSQLServer{
			resource: database,
		}
		return database
	}
}

var dynamicLabels = types.LabelsToV2(map[string]types.CommandLabel{
	"echo": &types.CommandLabelV2{
		Period:  types.NewDuration(time.Second),
		Command: []string{"echo", "test"},
	},
})

// testAWSRegion is the AWS region used in tests.
const testAWSRegion = "us-east-1"
