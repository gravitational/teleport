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
	"bytes"
	"context"
	"crypto/tls"
	"database/sql"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/ClickHouse/ch-go"
	cqlclient "github.com/datastax/go-cassandra-native-protocol/client"
	elastic "github.com/elastic/go-elasticsearch/v8"
	mysqlclient "github.com/go-mysql-org/go-mysql/client"
	mysqllib "github.com/go-mysql-org/go-mysql/mysql"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jackc/pgconn"
	"github.com/jonboulle/clockwork"
	mssql "github.com/microsoft/go-mssqldb"
	opensearchclt "github.com/opensearch-project/opensearch-go/v2"
	goredis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	sqladmin "google.golang.org/api/sqladmin/v1beta4"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/authz"
	clients "github.com/gravitational/teleport/lib/cloud"
	"github.com/gravitational/teleport/lib/cloud/awsconfig"
	"github.com/gravitational/teleport/lib/cloud/mocks"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/defaults"
	libevents "github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/eventstest"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/inventory"
	"github.com/gravitational/teleport/lib/limiter"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/multiplexer"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv"
	"github.com/gravitational/teleport/lib/srv/alpnproxy"
	alpncommon "github.com/gravitational/teleport/lib/srv/alpnproxy/common"
	"github.com/gravitational/teleport/lib/srv/db/cassandra"
	"github.com/gravitational/teleport/lib/srv/db/clickhouse"
	"github.com/gravitational/teleport/lib/srv/db/cloud"
	"github.com/gravitational/teleport/lib/srv/db/common"
	dbconnect "github.com/gravitational/teleport/lib/srv/db/common/connect"
	"github.com/gravitational/teleport/lib/srv/db/dynamodb"
	"github.com/gravitational/teleport/lib/srv/db/elasticsearch"
	"github.com/gravitational/teleport/lib/srv/db/mongodb"
	"github.com/gravitational/teleport/lib/srv/db/mongodb/protocol"
	"github.com/gravitational/teleport/lib/srv/db/mysql"
	"github.com/gravitational/teleport/lib/srv/db/objects"
	"github.com/gravitational/teleport/lib/srv/db/opensearch"
	"github.com/gravitational/teleport/lib/srv/db/postgres"
	"github.com/gravitational/teleport/lib/srv/db/redis"
	redisprotocol "github.com/gravitational/teleport/lib/srv/db/redis/protocol"
	"github.com/gravitational/teleport/lib/srv/db/snowflake"
	"github.com/gravitational/teleport/lib/srv/db/spanner"
	"github.com/gravitational/teleport/lib/srv/db/sqlserver"
	"github.com/gravitational/teleport/lib/srv/discovery/fetchers/db"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/cert"
)

func TestMain(m *testing.M) {
	utils.InitLoggerForTests()
	cryptosuites.PrecomputeRSATestKeys(m)
	modules.SetInsecureTestMode(true)
	registerTestSnowflakeEngine()
	registerTestElasticsearchEngine()
	registerTestSQLServerEngine()
	os.Exit(m.Run())
}

// TestAccessPostgres verifies access scenarios to a Postgres database based
// on the configured RBAC rules.
func TestAccessPostgres(t *testing.T) {
	ctx := context.Background()
	testCtx := setupTestContext(ctx, t, withSelfHostedPostgres("postgres", func(db *types.DatabaseV3) {
		db.SetStaticLabels(map[string]string{"foo": "bar"})
	}))
	go testCtx.startHandlingConnections()

	dynamicDBLabels := types.Labels{"echo": {"test"}}
	staticDBLabels := types.Labels{"foo": {"bar"}}
	tests := []struct {
		desc          string
		user          string
		role          string
		allowDbNames  []string
		allowDbUsers  []string
		extraRoleOpts []roleOptFn
		dbName        string
		dbUser        string
		err           string
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
		{
			desc:         "access allowed to specific user/database by static label",
			user:         "alice",
			role:         "admin",
			allowDbNames: []string{"metrics"},
			allowDbUsers: []string{"alice"},
			// The default test role created has wildcard labels allowed.
			// This tests that specific allowed database labels matching the
			// test database's static labels allows access.
			extraRoleOpts: []roleOptFn{withAllowedDBLabels(staticDBLabels)},
			dbName:        "metrics",
			dbUser:        "alice",
		},
		{
			desc:         "access allowed to specific user/database by dynamic label",
			user:         "alice",
			role:         "admin",
			allowDbNames: []string{"metrics"},
			allowDbUsers: []string{"alice"},
			// The default test role created has wildcard labels allowed.
			// This tests that specific allowed database labels matching the
			// test database's dynamic labels allows access, to ensure
			// that RBAC checks against dynamic labels are working.
			extraRoleOpts: []roleOptFn{withAllowedDBLabels(dynamicDBLabels)},
			dbName:        "metrics",
			dbUser:        "alice",
		},
		{
			desc:         "access denied by dynamic label",
			user:         "alice",
			role:         "admin",
			allowDbNames: []string{"metrics"},
			allowDbUsers: []string{"alice"},
			// The default test role created has wildcard labels allowed.
			// This tests that specific denied database labels matching the
			// test database's dynamic labels denies access, to ensure
			// that RBAC checks against dynamic labels are working.
			extraRoleOpts: []roleOptFn{withDeniedDBLabels(dynamicDBLabels)},
			dbName:        "metrics",
			dbUser:        "alice",
			err:           "access to db denied",
		},
		{
			desc:         "empty username is not allowed",
			user:         "alice",
			role:         "admin",
			allowDbNames: []string{types.Wildcard},
			allowDbUsers: []string{"postgres"},
			dbName:       "postgres",
			dbUser:       "",
			err:          "user name must not be empty",
		},
		{
			desc:         "empty DB name is set to allowed user name",
			user:         "alice",
			role:         "admin",
			allowDbNames: []string{"postgres"},
			allowDbUsers: []string{"postgres"},
			dbName:       "",
			dbUser:       "postgres",
		},
		{
			desc:         "empty DB name is set to disallowed user name",
			user:         "alice",
			role:         "admin",
			allowDbNames: []string{"non-existent"},
			allowDbUsers: []string{"postgres"},
			dbName:       "",
			dbUser:       "postgres",
			err:          "access to db denied",
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			// Create user/role with the requested permissions.
			testCtx.createUserAndRole(ctx, t, test.user, test.role,
				test.allowDbUsers, test.allowDbNames, test.extraRoleOpts...)

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

func TestMySQLServerVersionUpdateOnConnection(t *testing.T) {
	ctx := context.Background()
	testCtx := setupTestContext(
		ctx,
		t,
		withSelfHostedMySQL("mysql",
			// Set an older version in DB spec.
			withMySQLServerVersionInDBSpec("6.6.6-before"),
			// Set a newer version in TestServer.
			withMySQLServerVersion("8.8.8-after"),
		),
	)
	go testCtx.startHandlingConnections()

	// Confirm the server version configured in the spec.
	db, err := testCtx.server.getProxiedDatabase("mysql")
	require.NoError(t, err)
	require.Equal(t, "6.6.6-before", db.GetMySQLServerVersion())

	// Connect.
	testCtx.createUserAndRole(ctx, t, "alice", "admin", []string{"alice"}, []string{types.Wildcard})
	mysqlConn, err := testCtx.mysqlClient("alice", "mysql", "alice")
	require.NoError(t, err)
	defer mysqlConn.Close()
	_, err = mysqlConn.Execute("select 1")
	require.NoError(t, err)

	// Check if proxied database is updated.
	updatedDB, err := testCtx.server.getProxiedDatabase("mysql")
	require.NoError(t, err)
	require.Equal(t, "8.8.8-after", updatedDB.GetMySQLServerVersion())
}

// TestAccessRedis verifies access scenarios to a Redis database based
// on the configured RBAC rules.
func TestAccessRedis(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	testCtx := setupTestContext(ctx, t,
		withSelfHostedRedis("redis"),
		withAzureRedis("azure-redis", azureRedisToken))
	go testCtx.startHandlingConnections()

	tests := []struct {
		// desc is the test case description.
		desc string
		// dbService is the name of the database service to connect to.
		dbService string
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
			dbService:    "redis",
			user:         "alice",
			role:         "admin",
			allowDbUsers: []string{types.Wildcard},
			dbUser:       "root",
		},
		{
			desc:         "has access to nothing",
			dbService:    "redis",
			user:         "alice",
			role:         "admin",
			allowDbUsers: []string{},
			dbUser:       "root",
			err:          "access to db denied",
		},
		{
			desc:         "access allowed to specific user",
			dbService:    "redis",
			user:         "alice",
			role:         "admin",
			allowDbUsers: []string{"alice"},
			dbUser:       "alice",
		},
		{
			desc:         "access denied to specific user",
			dbService:    "redis",
			user:         "alice",
			role:         "admin",
			allowDbUsers: []string{"alice"},
			dbUser:       "root",
			err:          "access to db denied",
		},
		{
			desc:         "azure access allowed to default user",
			dbService:    "azure-redis",
			user:         "alice",
			role:         "admin",
			allowDbUsers: []string{"default"},
			dbUser:       "default",
		},
		{
			desc:         "azure access denied to non-default user",
			dbService:    "azure-redis",
			user:         "alice",
			role:         "admin",
			allowDbUsers: []string{"alice"},
			dbUser:       "alice",
			err:          "access denied to non-default db user",
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			// Create user/role with the requested permissions.
			testCtx.createUserAndRole(ctx, t, test.user, test.role, test.allowDbUsers, []string{types.Wildcard})

			// Try to connect to the database as this user.
			redisClient, err := testCtx.redisClient(ctx, test.user, test.dbService, test.dbUser)
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

// TestRedisCommandDocs is a regression test to verify if simple/status strings
// (+<string>) are returned to Redis client for "COMMAND DOCS" command.
//
// "redis-cli" expects command info flags in simple/status strings (+<string>),
// not regular ones ($<string>). An error is thrown by "redis-cli" if +<string>
// is not received.
func TestRedisCommandDocs(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	testCtx := setupTestContext(ctx, t, withSelfHostedRedis("redis"))
	go testCtx.startHandlingConnections()

	// Create user/role with the requested permissions.
	testCtx.createUserAndRole(ctx, t, "alice", "admin", []string{types.Wildcard}, []string{types.Wildcard})

	// Try to connect to the database as this user.
	redisClient, err := testCtx.redisClient(ctx, "alice", "redis", "default")
	require.NoError(t, err)

	// Miniredis returns a sample command result:
	// https://github.com/alicebob/miniredis/blob/master/cmd_command.go
	//
	// Unlike "redis-cli", redisClient.Command accepts any kind of string
	// regardless whether the flag is in +<flag> or in $<flag>. This should
	// always succeeds but it will be used as a reference to test the raw
	// output.
	parsedCommandInfos := redisClient.Command(ctx)
	require.NoError(t, parsedCommandInfos.Err())

	// Capture the raw bytes.
	rawCommandInfos := goredis.NewCmd(ctx, "COMMAND", "DOCS")
	require.NoError(t, redisClient.Process(ctx, rawCommandInfos))
	rawCommandInfosBuffer := &bytes.Buffer{}
	require.NoError(t, redisprotocol.WriteCmd(goredis.NewWriter(rawCommandInfosBuffer), rawCommandInfos.Val()))

	// Loop each flag to make sure +<flag> is written to the raw bytes.
	rawCommandInfosOutput := rawCommandInfosBuffer.String()
	var flagsVerified int
	for _, commandInfo := range parsedCommandInfos.Val() {
		for _, flag := range commandInfo.Flags {
			require.Contains(t, rawCommandInfosOutput, "+"+flag)
			flagsVerified += 1
		}
	}
	// Just to make sure miniredis is returning a command info with some flags.
	require.Greater(t, flagsVerified, 0)
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
	certPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: ephemeralCert.Certificate[0],
	})

	// Setup database servers for Postgres and MySQL with a mock GCP API that
	// will require SSL and return the ephemeral certificate created above.
	testCtx.server = testCtx.setupDatabaseServer(ctx, t, agentParams{
		Databases: []types.Database{
			withCloudSQLPostgres("postgres", cloudSQLAuthToken)(t, ctx, testCtx),
			withCloudSQLMySQLTLS("mysql", user, cloudSQLPassword)(t, ctx, testCtx),
		},
		GCPSQL: &mocks.GCPSQLAdminClientMock{
			EphemeralCert: string(certPEM),
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

func registerTestSQLServerEngine() {
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
			name: "current server",
			opts: []mongodb.TestServerOption{
				mongodb.TestServerWireVersion(protocol.OpmsgWireVersion),
			},
		},
		{
			name: "mongodb 3.6 server",
			opts: []mongodb.TestServerOption{
				mongodb.TestServerWireVersion(6),
			},
		},
	}

	clientOpts := []struct {
		name string
		opts *options.ClientOptions
	}{
		{
			name: "client without compression",
			opts: options.Client().
				// Add extra time so the test won't time out when running in parallel.
				SetServerSelectionTimeout(10 * time.Second),
		},
		{
			name: "client with compression",
			opts: options.Client().
				// Add extra time so the test won't time out when running in parallel.
				SetServerSelectionTimeout(10 * time.Second).
				SetCompressors([]string{"zlib"}),
		},
	}

	// Execute each scenario on both modern and legacy Mongo servers
	// to make sure legacy messages are also subject to RBAC.
	for _, test := range tests {
		test := test
		t.Run(fmt.Sprintf("%v", test.desc), func(t *testing.T) {
			t.Parallel()

			for _, serverOpt := range serverOpts {
				testCtx := setupTestContext(ctx, t, withSelfHostedMongo("mongo", serverOpt.opts...))
				go testCtx.startHandlingConnections()

				// Create user/role with the requested permissions.
				testCtx.createUserAndRole(ctx, t, test.user, test.role, test.allowDbUsers, test.allowDbNames)

				for _, clientOpt := range clientOpts {
					clientOpt := clientOpt

					t.Run(fmt.Sprintf("%v/%v", serverOpt.name, clientOpt.name), func(t *testing.T) {
						t.Parallel()

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
		})
	}
}

func TestMongoDBMaxMessageSize(t *testing.T) {
	ctx := context.Background()

	for name, tt := range map[string]struct {
		maxMessageSize     uint32
		messageSize        int
		expectedQueryError bool
	}{
		"default message size": {
			messageSize: 400,
		},
		"message size exceeded": {
			// Set a value that will enable handshake message to complete
			// successfully.
			maxMessageSize:     400,
			messageSize:        500,
			expectedQueryError: true,
		},
	} {
		t.Run(name, func(t *testing.T) {
			tt := tt
			t.Parallel()

			testCtx := setupTestContext(ctx, t, withSelfHostedMongo("mongo", mongodb.TestServerMaxMessageSize(tt.maxMessageSize)))
			testCtx.createUserAndRole(ctx, t, "alice", "admin", []string{"admin"}, []string{"admin"})
			go testCtx.startHandlingConnections()

			mongoClient, err := testCtx.mongoClient(ctx, "alice", "mongo", "admin")
			require.NoError(t, err)
			defer mongoClient.Disconnect(ctx)

			_, err = mongoClient.Database("admin").Collection("test").Find(ctx, bson.M{"largevalue": make([]byte, tt.messageSize)})
			if tt.expectedQueryError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}

// TestAccessDisabled makes sure database access can be disabled via modules.
func TestAccessDisabled(t *testing.T) {
	modules.SetTestModules(t, &modules.TestModules{
		TestFeatures: modules.Features{
			Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
				entitlements.DB: {Enabled: false},
			},
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
	require.Equal(t, "123", getResult.Val())

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
			// Create a synchronization channel between publisher and subscriber
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
			if err != nil && !errors.Is(err, goredis.Nil) {
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
			if errors.Is(err, goredis.TxFailedErr) {
				// Optimistic lock lost. Retry.
				continue
			}
			// Return any other error.
			return err
		}

		return errors.New("increment reached maximum number of retries")
	}

	var wg sync.WaitGroup
	// use just 2 concurrent connections as we want to test our proxy/protocol behavior not Redis concurrency.
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
	authClient     *authclient.Client
	proxyServer    *ProxyServer
	mux            *multiplexer.Mux
	mysqlListener  net.Listener
	webListener    *multiplexer.WebListener
	fakeRemoteSite *reversetunnelclient.FakeRemoteSite
	server         *Server
	emitter        *eventstest.ChannelEmitter
	databaseCA     types.CertAuthority
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
	// snowflake is a collection of Snowflake databases the test uses.
	snowflake map[string]testSnowflake
	// cassandra is a collection of Cassandra databases the test uses.
	cassandra map[string]testCassandra
	// elasticsearch is a collection of Elasticsearch databases the test uses.
	elasticsearch map[string]testElasticsearch
	// opensearch is a collection of OpenSearch databases the test uses.
	opensearch map[string]testOpenSearch
	// dynamodb is a collection of DynamoDB databases the test uses.
	dynamodb map[string]testDynamoDB
	// clickHouse is a collection of ClickHouse databases the test uses.
	clickHouse map[string]testClickHouse
	// spanner is a collection of Spanner databases the test uses.
	spanner map[string]testSpannerDB

	// clock to override clock in tests.
	clock *clockwork.FakeClock
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
	// db is the test SQLServer database server.
	db *sqlserver.TestServer
	// resource is the resource representing this SQL Server database
	resource types.Database
}

type testSnowflake struct {
	// db is the test Snowflake database server.
	db *snowflake.TestServer
	// resource is the resource representing this Snowflake database.
	resource types.Database
}

// testCassandra represents a single proxied Cassandra database.
type testCassandra struct {
	// db is the test Cassandra database server.
	db *cassandra.TestServer
	// resource is the resource representing this Cassandra database.
	resource types.Database
}

type testClickHouse struct {
	// db is the test Clickhouse database server.
	db *clickhouse.TestServer
	// resource is the resource representing this ClickHouse database.
	resource types.Database
}

type testElasticsearch struct {
	// db is the test elasticsearch database server.
	db *elasticsearch.TestServer
	// resource is the resource representing this elasticsearch database.
	resource types.Database
}

type testOpenSearch struct {
	// db is the test OpenSearch database server.
	db *opensearch.TestServer
	// resource is the resource representing this OpenSearch database.
	resource types.Database
}

// testDynamoDB represents a single proxied DynamoDB database.
type testDynamoDB struct {
	// db is the test Dynamodb database server.
	db *dynamodb.TestServer
	// resource is the resource representing this DynamoDB database.
	resource types.Database
}

// testSpannerDB represents a single proxied Spanner database.
type testSpannerDB struct {
	// db is the test Spanner database server.
	db *spanner.TestServer
	// resource is the resource representing this Spanner database.
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
func (c *testContext) postgresClient(ctx context.Context, teleportUser, dbService, dbUser, dbName string, opts ...common.ClientOption) (*pgconn.PgConn, error) {
	return c.postgresClientWithAddr(ctx, c.mux.DB().Addr().String(), teleportUser, dbService, dbUser, dbName, opts...)
}

// postgresClientWithAddr is like postgresClient but allows to override connection address.
func (c *testContext) postgresClientWithAddr(ctx context.Context, address, teleportUser, dbService, dbUser, dbName string, opts ...common.ClientOption) (*pgconn.PgConn, error) {
	cfg := common.TestClientConfig{
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
	}

	for _, opt := range opts {
		opt(&cfg)
	}

	return postgres.MakeTestClient(ctx, cfg)
}

// postgresClientLocalProxy connects to test Postgres through local ALPN proxy.
func (c *testContext) postgresClientLocalProxy(ctx context.Context, teleportUser, dbService, dbUser, dbName string) (*pgconn.PgConn, *alpnproxy.LocalProxy, error) {
	route := tlsca.RouteToDatabase{
		ServiceName: dbService,
		Protocol:    defaults.ProtocolPostgres,
		Username:    dbUser,
		Database:    dbName,
	}

	// Start local proxy which client will connect to.
	proxy, err := c.startLocalALPNProxy(ctx, c.webListener.Addr().String(), teleportUser, route)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	// Client connects to the local proxy without TLS.
	conn, err := pgconn.Connect(ctx, fmt.Sprintf("postgres://%v@%v/%v", dbUser, proxy.GetAddr(), dbName))
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	return conn, proxy, nil
}

// mysqlClient connects to test MySQL through database access as a specified
// Teleport user and database account.
func (c *testContext) mysqlClient(teleportUser, dbService, dbUser string) (mysql.TestClientConn, error) {
	return c.mysqlClientWithAddr(c.mysqlListener.Addr().String(), teleportUser, dbService, dbUser)
}

// mysqlClientWithAddr is like mysqlClient but allows to override connection address.
func (c *testContext) mysqlClientWithAddr(address, teleportUser, dbService, dbUser string) (mysql.TestClientConn, error) {
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

// mysqlClientLocalProxy connects to test MySQL through local ALPN proxy.
func (c *testContext) mysqlClientLocalProxy(ctx context.Context, teleportUser, dbService, dbUser string) (mysql.TestClientConn, *alpnproxy.LocalProxy, error) {
	route := tlsca.RouteToDatabase{
		ServiceName: dbService,
		Protocol:    defaults.ProtocolMySQL,
		Username:    dbUser,
	}

	// Start local proxy which client will connect to.
	proxy, err := c.startLocalALPNProxy(ctx, c.webListener.Addr().String(), teleportUser, route)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	// Client connects to the local proxy without TLS.
	conn, err := mysqlclient.Connect(proxy.GetAddr(), dbUser, "", "")
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	return conn, proxy, nil
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

// mongoClientLocalProxy connects to test MongoDB through local ALPN proxy.
func (c *testContext) mongoClientLocalProxy(ctx context.Context, teleportUser, dbService, dbUser string) (*mongo.Client, *alpnproxy.LocalProxy, error) {
	route := tlsca.RouteToDatabase{
		ServiceName: dbService,
		Protocol:    defaults.ProtocolMongoDB,
		Username:    dbUser,
	}

	// Start local proxy which client will connect to.
	proxy, err := c.startLocalALPNProxy(ctx, c.webListener.Addr().String(), teleportUser, route)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	// Client connects to the local proxy without TLS.
	client, err := mongo.Connect(ctx, options.Client().
		ApplyURI("mongodb://"+proxy.GetAddr()).
		SetHeartbeatInterval(500*time.Millisecond).
		SetServerSelectionTimeout(5*time.Second))
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	// Ping to make sure it connected successfully.
	errPing := client.Ping(ctx, nil)
	if errPing != nil {
		if err := client.Disconnect(ctx); err != nil {
			return nil, nil, trace.NewAggregate(errPing, err)
		}
		return nil, nil, trace.Wrap(errPing)
	}

	return client, proxy, nil
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

// redisClientLocalProxy connects to test Redis through local ALPN proxy.
func (c *testContext) redisClientLocalProxy(ctx context.Context, teleportUser, dbService, dbUser string) (*redis.Client, *alpnproxy.LocalProxy, error) {
	route := tlsca.RouteToDatabase{
		ServiceName: dbService,
		Protocol:    defaults.ProtocolRedis,
		Username:    dbUser,
	}

	// Start local proxy which client will connect to.
	proxy, err := c.startLocalALPNProxy(ctx, c.webListener.Addr().String(), teleportUser, route)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	// Client connects to the local proxy without TLS.
	client := goredis.NewClient(&goredis.Options{
		Addr: proxy.GetAddr(),
	})

	// Ping to make sure connection is successful.
	errPing := client.Ping(ctx).Err()
	if errPing != nil {
		if err := client.Close(); err != nil {
			return nil, nil, trace.NewAggregate(errPing, err)
		}
		return nil, nil, trace.Wrap(errPing)
	}

	return client, proxy, nil
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

// clickHouseNativeClient connects to the specified ClickHouse Server address.
func (c *testContext) clickHouseNativeClient(ctx context.Context, teleportUser, dbService, dbUser, dbName string) (*ch.Client, *alpnproxy.LocalProxy, error) {
	proxy, route, err := c.startLocalProxy(ctx, teleportUser, dbService, dbUser, dbName)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	client, err := clickhouse.MakeNativeTestClient(ctx, common.TestClientConfig{
		AuthClient:      c.authClient,
		AuthServer:      c.authServer,
		Address:         proxy.GetAddr(),
		Cluster:         c.clusterName,
		Username:        teleportUser,
		RouteToDatabase: route,
	})
	if err != nil {
		proxy.Close()
		return nil, nil, trace.Wrap(err)
	}
	return client, proxy, nil
}

// clickHouseHTTPClient connects to the specified ClickHouse Server address.
func (c *testContext) clickHouseHTTPClient(ctx context.Context, teleportUser, dbService, dbUser, dbName string) (*sql.DB, *alpnproxy.LocalProxy, error) {
	proxy, route, err := c.startLocalProxy(ctx, teleportUser, dbService, dbUser, dbName)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	client, err := clickhouse.MakeDBTestClient(ctx, common.TestClientConfig{
		AuthClient:      c.authClient,
		AuthServer:      c.authServer,
		Address:         proxy.GetAddr(),
		Cluster:         c.clusterName,
		Username:        teleportUser,
		RouteToDatabase: route,
	})
	if err != nil {
		proxy.Close()
		return nil, nil, trace.Wrap(err)
	}
	return client, proxy, nil
}

func (c *testContext) startLocalProxy(ctx context.Context, teleportUser, dbService, dbUser, dbName string) (*alpnproxy.LocalProxy, tlsca.RouteToDatabase, error) {
	route := tlsca.RouteToDatabase{
		ServiceName: dbService,
		Protocol:    defaults.ProtocolClickHouseHTTP,
		Username:    dbUser,
		Database:    dbName,
	}

	proxy, err := c.startLocalALPNProxy(ctx, c.webListener.Addr().String(), teleportUser, route)
	if err != nil {
		return nil, tlsca.RouteToDatabase{}, trace.Wrap(err)
	}
	return proxy, route, nil
}

// cassandraClient connects to test Cassandra through database access as a specified Teleport user and database account.
func (c *testContext) cassandraClient(ctx context.Context, teleportUser, dbService, dbUser string, opts ...cassandra.ClientOptions) (*cassandra.Session, error) {
	return c.cassandraClientWithAddr(ctx, c.webListener.Addr().String(), teleportUser, dbService, dbUser, opts...)
}

// cassandraRawClient connects to test Cassandra through using a raw connection that
// allows to send/receive a native Cassandra protocol frames.
func (c *testContext) cassandraRawClient(ctx context.Context, teleportUser, dbService, dbUser string, opts ...cassandra.ClientOptions) (*cqlclient.CqlClientConnection, error) {
	options := cassandra.ClientOptionsParams{
		Username: "cassandra",
	}
	for _, opt := range opts {
		opt(&options)
	}
	cc := cqlclient.NewCqlClient(c.webListener.Addr().String(), &cqlclient.AuthCredentials{
		Username: options.Username,
		Password: "cassandra",
	})
	cc.ReadTimeout = time.Hour
	cc.ConnectTimeout = time.Hour
	tlsConfig, err := common.MakeTestClientTLSConfig(common.TestClientConfig{
		AuthClient: c.authClient,
		AuthServer: c.authServer,
		Address:    c.webListener.Addr().String(),
		Cluster:    c.clusterName,
		Username:   teleportUser,
		RouteToDatabase: tlsca.RouteToDatabase{
			ServiceName: dbService,
			Protocol:    defaults.ProtocolCassandra,
			Username:    dbUser,
		},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	cc.TLSConfig = tlsConfig
	pp, err := cc.Connect(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return pp, nil
}

// cassandraClientWithAddr is like cassandraClient but allows overriding connection address.
func (c *testContext) cassandraClientWithAddr(ctx context.Context, proxyAddress, teleportUser, dbService, dbUser string, opts ...cassandra.ClientOptions) (*cassandra.Session, error) {
	return cassandra.MakeTestClient(ctx, common.TestClientConfig{
		AuthClient: c.authClient,
		AuthServer: c.authServer,
		Address:    proxyAddress,
		Cluster:    c.clusterName,
		Username:   teleportUser,
		RouteToDatabase: tlsca.RouteToDatabase{
			ServiceName: dbService,
			Protocol:    defaults.ProtocolCassandra,
			Username:    dbUser,
		},
	}, opts...)
}

// startLocalALPNProxy starts local ALPN proxy for the specified database.
func (c *testContext) startLocalALPNProxy(ctx context.Context, proxyAddr, teleportUser string, route tlsca.RouteToDatabase) (*alpnproxy.LocalProxy, error) {
	key, err := keys.ParsePrivateKey(fixtures.PEMBytes["rsa"])
	if err != nil {
		return nil, trace.Wrap(err)
	}

	publicKeyPEM, err := keys.MarshalPublicKey(key.Public())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	clientCert, err := c.authServer.GenerateDatabaseTestCert(
		auth.DatabaseTestCertRequest{
			PublicKey:       publicKeyPEM,
			Cluster:         c.clusterName,
			Username:        teleportUser,
			RouteToDatabase: route,
		})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	tlsCert, err := key.TLSCertificate(clientCert)
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
		Protocols:          []alpncommon.Protocol{proto},
		InsecureSkipVerify: true,
		Listener:           listener,
		ParentContext:      ctx,
		Cert:               tlsCert,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	go proxy.Start(ctx)

	return proxy, nil
}

// snowflakeClient returns a Snowflake test DB client.
func (c *testContext) snowflakeClient(ctx context.Context, teleportUser, dbService, dbUser, dbName string) (*sql.DB, *alpnproxy.LocalProxy, error) {
	route := tlsca.RouteToDatabase{
		ServiceName: dbService,
		Protocol:    defaults.ProtocolSnowflake,
		Username:    dbUser,
		Database:    dbName,
	}

	proxy, err := c.startLocalALPNProxy(ctx, c.webListener.Addr().String(), teleportUser, route)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	db, err := snowflake.MakeTestClient(ctx, common.TestClientConfig{
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

	return db, proxy, nil
}

// elasticsearchClient returns an Elasticsearch test DB client.
func (c *testContext) elasticsearchClient(ctx context.Context, teleportUser, dbService, dbUser string) (*elastic.Client, *alpnproxy.LocalProxy, error) {
	route := tlsca.RouteToDatabase{
		ServiceName: dbService,
		Protocol:    defaults.ProtocolElasticsearch,
		Username:    dbUser,
	}

	proxy, err := c.startLocalALPNProxy(ctx, c.webListener.Addr().String(), teleportUser, route)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	db, err := elasticsearch.MakeTestClient(ctx, common.TestClientConfig{
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

	return db, proxy, nil
}

// openSearchClient returns an OpenSearch test DB client.
func (c *testContext) openSearchClient(ctx context.Context, teleportUser, dbService, dbUser string) (*opensearchclt.Client, *alpnproxy.LocalProxy, error) {
	route := tlsca.RouteToDatabase{
		ServiceName: dbService,
		Protocol:    defaults.ProtocolOpenSearch,
		Username:    dbUser,
	}

	proxy, err := c.startLocalALPNProxy(ctx, c.webListener.Addr().String(), teleportUser, route)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	db, err := opensearch.MakeTestClient(ctx, common.TestClientConfig{
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

	return db, proxy, nil
}

// dynamodbClient returns a DynamoDB test client.
func (c *testContext) dynamodbClient(ctx context.Context, teleportUser, dbService, dbUser string) (*dynamodb.Client, *alpnproxy.LocalProxy, error) {
	route := tlsca.RouteToDatabase{
		ServiceName: dbService,
		Protocol:    defaults.ProtocolDynamoDB,
		Username:    dbUser,
	}

	proxy, err := c.startLocalALPNProxy(ctx, c.webListener.Addr().String(), teleportUser, route)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	db, err := dynamodb.MakeTestClient(ctx, common.TestClientConfig{
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

	return db, proxy, nil
}

func (c *testContext) spannerClient(ctx context.Context, teleportUser, dbService, dbUser, dbName string) (*spanner.SpannerTestClient, *alpnproxy.LocalProxy, error) {
	route := tlsca.RouteToDatabase{
		ServiceName: dbService,
		Protocol:    defaults.ProtocolSpanner,
		Username:    dbUser,
		Database:    dbName,
	}

	proxy, err := c.startLocalALPNProxy(ctx, c.webListener.Addr().String(), teleportUser, route)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	clt, err := spanner.MakeTestClient(ctx, common.TestClientConfig{
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

	return clt, proxy, nil
}

type roleOptFn func(types.Role)

func withDeniedDatabaseUsers(users ...string) roleOptFn {
	return func(role types.Role) {
		role.SetDatabaseUsers(types.Deny, users)
	}
}

func withDeniedDatabaseNames(names ...string) roleOptFn {
	return func(role types.Role) {
		role.SetDatabaseNames(types.Deny, names)
	}
}

func withAllowedDBLabels(labels types.Labels) roleOptFn {
	return func(role types.Role) {
		role.SetDatabaseLabels(types.Allow, labels)
	}
}

func withDeniedDBLabels(labels types.Labels) roleOptFn {
	return func(role types.Role) {
		role.SetDatabaseLabels(types.Deny, labels)
	}
}

func withClientIdleTimeout(clientIdleTimeout time.Duration) roleOptFn {
	return func(role types.Role) {
		opts := role.GetOptions()
		opts.ClientIdleTimeout = types.NewDuration(clientIdleTimeout)
		role.SetOptions(opts)
	}
}

// createUserAndRole creates Teleport user and role with specified names
// and allowed database users/names properties.
func (c *testContext) createUserAndRole(ctx context.Context, t testing.TB, userName, roleName string, dbUsers, dbNames []string, roleOpts ...roleOptFn) (types.User, types.Role) {
	user, role, err := auth.CreateUserAndRole(c.tlsServer.Auth(), userName, []string{roleName}, nil)
	require.NoError(t, err)
	role.SetDatabaseUsers(types.Allow, dbUsers)
	role.SetDatabaseNames(types.Allow, dbNames)
	for _, roleOpt := range roleOpts {
		roleOpt(role)
	}
	role, err = c.tlsServer.Auth().UpsertRole(ctx, role)
	require.NoError(t, err)
	return user, role
}

// makeTLSConfig returns tls configuration for the test's tls listener.
func (c *testContext) makeTLSConfig(t testing.TB) *tls.Config {
	creds, err := cert.GenerateSelfSignedCert([]string{"localhost"}, nil)
	require.NoError(t, err)
	cert, err := tls.X509KeyPair(creds.Cert, creds.PrivateKey)
	require.NoError(t, err)
	conf := utils.TLSConfig(nil)
	conf.Certificates = append(conf.Certificates, cert)
	conf.ClientAuth = tls.VerifyClientCertIfGiven
	conf.ClientCAs, _, _, err = authclient.DefaultClientCertPool(context.Background(), c.authServer, c.clusterName)
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
	SetShuffleFunc(dbconnect.ShuffleSort)
}

func setupTestContext(ctx context.Context, t testing.TB, withDatabases ...withDatabaseOption) *testContext {
	testCtx := &testContext{
		clusterName:   "root.example.com",
		hostID:        uuid.New().String(),
		postgres:      make(map[string]testPostgres),
		mysql:         make(map[string]testMySQL),
		mongo:         make(map[string]testMongoDB),
		redis:         make(map[string]testRedis),
		sqlServer:     make(map[string]testSQLServer),
		snowflake:     make(map[string]testSnowflake),
		elasticsearch: make(map[string]testElasticsearch),
		opensearch:    make(map[string]testOpenSearch),
		cassandra:     make(map[string]testCassandra),
		dynamodb:      make(map[string]testDynamoDB),
		clickHouse:    make(map[string]testClickHouse),
		spanner:       make(map[string]testSpannerDB),
		clock:         clockwork.NewFakeClockAt(time.Now()),
	}
	t.Cleanup(func() { testCtx.Close() })

	// Create and start test auth server.
	authServer, err := auth.NewTestAuthServer(auth.TestAuthServerConfig{
		Clock:       testCtx.clock,
		ClusterName: testCtx.clusterName,
		AuthPreferenceSpec: &types.AuthPreferenceSpecV2{
			SignatureAlgorithmSuite: types.SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_BALANCED_V1,
		},
		Dir: t.TempDir(),
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
		ID:       "test",
		Listener: listener,
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
	_, err = authServer.AuthServer.UpsertSessionRecordingConfig(ctx, recConfig)
	require.NoError(t, err)

	// Auth client for database service.
	testCtx.authClient, err = testCtx.tlsServer.NewClient(auth.TestServerID(types.RoleDatabase, testCtx.hostID))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, testCtx.authClient.Close()) })

	testCtx.databaseCA, err = testCtx.authClient.GetCertAuthority(ctx, types.CertAuthID{Type: types.DatabaseCA, DomainName: testCtx.clusterName}, false)
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
	proxyAuthorizer, err := authz.NewAuthorizer(authz.AuthorizerOpts{
		ClusterName: testCtx.clusterName,
		AccessPoint: proxyAuthClient,
		LockWatcher: proxyLockWatcher,
	})
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
	testCtx.fakeRemoteSite = reversetunnelclient.NewFakeRemoteSite(testCtx.clusterName, proxyAuthClient)
	t.Cleanup(func() { require.NoError(t, testCtx.fakeRemoteSite.Close()) })
	tunnel := &reversetunnelclient.FakeServer{
		Sites: []reversetunnelclient.RemoteSite{
			testCtx.fakeRemoteSite,
		},
	}
	// Empty config means no limit.
	connLimiter, err := limiter.NewLimiter(limiter.Config{})
	require.NoError(t, err)

	// Create test audit events emitter.
	// NOTE(gavin): this emitter is just a buffered channel and it will block if
	// a test does not consume the events, which can make your test fail
	// mysteriously with a timeout.
	testCtx.emitter = eventstest.NewChannelEmitter(100)

	connMonitor, err := srv.NewConnectionMonitor(srv.ConnectionMonitorConfig{
		AccessPoint:    proxyAuthClient,
		LockWatcher:    proxyLockWatcher,
		Clock:          testCtx.clock,
		ServerID:       testCtx.hostID,
		Emitter:        testCtx.emitter,
		EmitterContext: ctx,
		Logger:         utils.NewSlogLoggerForTests(),
	})
	require.NoError(t, err)

	// Create database proxy server.
	testCtx.proxyServer, err = NewProxyServer(ctx, ProxyServerConfig{
		AuthClient:        proxyAuthClient,
		AccessPoint:       proxyAuthClient,
		Authorizer:        proxyAuthorizer,
		Tunnel:            tunnel,
		TLSConfig:         tlsConfig,
		Limiter:           connLimiter,
		ConnectionMonitor: connMonitor,
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
	GetServerInfoFn func(database types.Database) func(context.Context) (*types.DatabaseServerV3, error)
	// OnReconcile sets database resource reconciliation callback.
	OnReconcile func(types.Databases)
	// NoStart indicates server should not be started.
	NoStart bool
	// GCPSQL defines the GCP Cloud SQL mock to use for GCP API calls.
	GCPSQL *mocks.GCPSQLAdminClientMock
	// OnHeartbeat defines a heartbeat function that generates heartbeat events.
	OnHeartbeat func(error)
	// CADownloader defines the CA downloader.
	CADownloader CADownloader
	// CloudClients is the cloud API clients for database service.
	CloudClients clients.Clients
	// AWSConfigProvider provides [aws.Config] for AWS SDK service clients.
	AWSConfigProvider awsconfig.Provider
	// AWSDatabaseFetcherFactory provides AWS database fetchers
	AWSDatabaseFetcherFactory *db.AWSFetcherFactory
	// AWSMatchers is a list of AWS databases matchers.
	AWSMatchers []types.AWSMatcher
	// AzureMatchers is a list of Azure databases matchers.
	AzureMatchers []types.AzureMatcher
	// discoveryResourceChecker performs some pre-checks when creating databases
	// discovered by the discovery service.
	DiscoveryResourceChecker cloud.DiscoveryResourceChecker
	// Recorder is the recorder used on sessions.
	Recorder libevents.SessionRecorder
	// GetEngineFn can be used to override the engine created in tests.
	GetEngineFn func(types.Database, common.EngineConfig) (common.Engine, error)
	// DatabaseObjects is used to override the db object importer in tests.
	DatabaseObjects objects.Objects
}

func (p *agentParams) setDefaults(c *testContext) {
	if p.HostID == "" {
		p.HostID = c.hostID
	}
	if p.GCPSQL == nil {
		p.GCPSQL = &mocks.GCPSQLAdminClientMock{
			DatabaseInstance: &sqladmin.DatabaseInstance{
				Settings: &sqladmin.Settings{
					IpConfiguration: &sqladmin.IpConfiguration{
						RequireSsl: false,
					},
				},
			},
		}
	}
	if p.CADownloader == nil {
		p.CADownloader = &fakeDownloader{
			cert: []byte(fixtures.TLSCACertPEM),
		}
	}

	if p.CloudClients == nil {
		p.CloudClients = &clients.TestCloudClients{
			GCPSQL: p.GCPSQL,
		}
	}
	if p.AWSConfigProvider == nil {
		p.AWSConfigProvider = &mocks.AWSConfigProvider{Err: trace.AccessDenied("AWS SDK clients are disabled for tests by default")}
	}

	if p.DiscoveryResourceChecker == nil {
		p.DiscoveryResourceChecker = &fakeDiscoveryResourceChecker{}
	}
	if p.DatabaseObjects == nil {
		// disables the real object importer by default, to avoid unexpected
		// db admin connection attempts during tests.
		p.DatabaseObjects = fakeObjectsImporter{}
	}
}

func (c *testContext) setupDatabaseServer(ctx context.Context, t testing.TB, p agentParams) *Server {
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
	dbAuthorizer, err := authz.NewAuthorizer(authz.AuthorizerOpts{
		ClusterName: c.clusterName,
		AccessPoint: c.authClient,
		LockWatcher: lockWatcher,
	})
	require.NoError(t, err)

	// Create test database auth tokens generator.
	testAuth, err := newTestAuth(common.AuthConfig{
		AuthClient:        c.authClient,
		AccessPoint:       c.authClient,
		Clients:           &clients.TestCloudClients{},
		Clock:             c.clock,
		AWSConfigProvider: p.AWSConfigProvider,
	})
	require.NoError(t, err)

	// Create default limiter.
	connLimiter, err := limiter.NewLimiter(limiter.Config{})
	require.NoError(t, err)

	connMonitor, err := srv.NewConnectionMonitor(srv.ConnectionMonitorConfig{
		AccessPoint:    c.authClient,
		LockWatcher:    lockWatcher,
		Clock:          c.clock,
		ServerID:       p.HostID,
		Emitter:        c.emitter,
		EmitterContext: context.Background(),
		Logger:         utils.NewSlogLoggerForTests(),
	})
	require.NoError(t, err)

	if p.Recorder == nil {
		p.Recorder = libevents.NewDiscardRecorder()
	}

	// Auth client for this database server identity.
	clt, err := c.tlsServer.NewClient(auth.TestServerID(types.RoleDatabase, p.HostID))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, clt.Close()) })

	inventoryHandle := inventory.NewDownstreamHandle(clt.InventoryControlStream, proto.UpstreamInventoryHello{
		ServerID: p.HostID,
		Version:  teleport.Version,
		Services: []types.SystemRole{types.RoleDatabase},
		Hostname: "test",
	})

	// Create database server agent itself.
	server, err := New(ctx, Config{
		Clock:            c.clock,
		DataDir:          t.TempDir(),
		AuthClient:       c.authClient,
		AccessPoint:      c.authClient,
		Authorizer:       dbAuthorizer,
		Hostname:         constants.APIDomain,
		HostID:           p.HostID,
		TLSConfig:        tlsConfig,
		Limiter:          connLimiter,
		Auth:             testAuth,
		Emitter:          c.emitter,
		Databases:        p.Databases,
		OnHeartbeat:      p.OnHeartbeat,
		ResourceMatchers: p.ResourceMatchers,
		GetServerInfoFn:  p.GetServerInfoFn,
		GetRotation: func(types.SystemRole) (*types.Rotation, error) {
			return &types.Rotation{}, nil
		},
		NewAudit: func(cfg common.AuditConfig) (common.Audit, error) {
			// Use the same audit logger implementation but substitute the
			// underlying emitter so events can be tracked in tests.
			return common.NewAudit(common.AuditConfig{
				Emitter:  c.emitter,
				Recorder: libevents.WithNoOpPreparer(p.Recorder),
				Database: cfg.Database,
				Clock:    c.clock,
			})
		},
		CADownloader:              p.CADownloader,
		OnReconcile:               p.OnReconcile,
		DatabaseObjects:           p.DatabaseObjects,
		ConnectionMonitor:         connMonitor,
		CloudClients:              p.CloudClients,
		AWSConfigProvider:         p.AWSConfigProvider,
		AWSDatabaseFetcherFactory: p.AWSDatabaseFetcherFactory,
		AWSMatchers:               p.AWSMatchers,
		AzureMatchers:             p.AzureMatchers,
		ShutdownPollPeriod:        100 * time.Millisecond,
		InventoryHandle:           inventoryHandle,
		discoveryResourceChecker:  p.DiscoveryResourceChecker,
		getEngineFn:               p.GetEngineFn,
	})
	require.NoError(t, err)

	if !p.NoStart {
		require.NoError(t, server.Start(ctx))

		// Explicitly send a heartbeat for any statically defined dbs.
		for _, db := range p.Databases {
			select {
			case sender := <-inventoryHandle.Sender():
				dbServer, err := server.getServerInfo(ctx, db)
				require.NoError(t, err)
				require.NoError(t, sender.Send(ctx, proto.InventoryHeartbeat{
					DatabaseServer: dbServer,
				}))
			case <-time.After(20 * time.Second):
				t.Fatal("timed out waiting for inventory handle sender")
			}
		}
	}

	return server
}

// TestAccessClickHouse verifies access scenarios to a ClickHouse database.
func TestAccessClickHouse(t *testing.T) {
	const (
		aliceUser = "alice"
		adminRole = "admin"
	)

	ctx := context.Background()
	testCtx := setupTestContext(ctx, t,
		withClickhouseHTTP(defaults.ProtocolClickHouseHTTP),
		withClickhouseNative(defaults.ProtocolClickHouse),
	)
	go testCtx.startHandlingConnections()

	tests := []struct {
		desc         string
		allowDbUsers []string
		dbUser       string
		err          string
	}{
		{
			desc:         "has access to all database users",
			allowDbUsers: []string{types.Wildcard},
			dbUser:       "root",
		},
		{
			desc:         "has access to nothing",
			allowDbUsers: []string{},
			dbUser:       "root",
			err:          "access to db denied",
		},
		{
			desc:         "access allowed to specific user",
			allowDbUsers: []string{aliceUser},
			dbUser:       aliceUser,
		},
		{
			desc:         "access denied to specific user",
			allowDbUsers: []string{aliceUser},
			dbUser:       "root",
			err:          "access to db denied",
		},
	}

	type connectFunc func(ctx context.Context, teleportUser, dbService, dbUser, dbName string) (io.Closer, io.Closer, error)
	connectMap := map[string]connectFunc{
		defaults.ProtocolClickHouseHTTP: func(ctx context.Context, teleportUser, dbService, dbUser, dbName string) (io.Closer, io.Closer, error) {
			return testCtx.clickHouseHTTPClient(ctx, teleportUser, defaults.ProtocolClickHouseHTTP, dbUser, dbName)
		},
		defaults.ProtocolClickHouse: func(ctx context.Context, teleportUser, dbService, dbUser, dbName string) (io.Closer, io.Closer, error) {
			return testCtx.clickHouseNativeClient(ctx, teleportUser, defaults.ProtocolClickHouse, dbUser, dbName)
		},
	}

	for _, test := range tests {
		for _, protocol := range []string{defaults.ProtocolClickHouse, defaults.ProtocolClickHouseHTTP} {
			t.Run(protocol, func(t *testing.T) {
				t.Run(test.desc, func(t *testing.T) {
					// Create user/role with the requested permissions.
					testCtx.createUserAndRole(ctx, t, aliceUser, adminRole, test.allowDbUsers, []string{types.Wildcard})

					connectCall, ok := connectMap[protocol]
					require.True(t, ok)

					conn, proxy, err := connectCall(ctx, aliceUser, protocol, test.dbUser, "master")
					if test.err != "" {
						require.Error(t, err)
						// Error message propagation is only implemented for HTTP Clickhouse protocol.
						if protocol != defaults.ProtocolClickHouse {
							require.Contains(t, err.Error(), test.err)
						}
						return
					}
					require.NoError(t, err)

					// Close connection and proxy.
					t.Cleanup(func() {
						require.NoError(t, conn.Close())
						require.NoError(t, proxy.Close())
					})
				})
			})
		}
	}
}

type withDatabaseOption func(t testing.TB, ctx context.Context, testCtx *testContext) types.Database

type databaseOption func(*types.DatabaseV3)

func withSelfHostedPostgres(name string, dbOpts ...databaseOption) withDatabaseOption {
	return func(t testing.TB, ctx context.Context, testCtx *testContext) types.Database {
		postgresServer, err := postgres.NewTestServer(common.TestServerConfig{
			Name:       name,
			AuthClient: testCtx.authClient,
			ClientAuth: tls.RequireAndVerifyClientCert,
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
		for _, dbOpt := range dbOpts {
			dbOpt(database)
		}
		testCtx.postgres[name] = testPostgres{
			db:       postgresServer,
			resource: database,
		}
		return database
	}
}

func withSelfHostedPostgresUsers(name string, users []string, dbOpts ...databaseOption) withDatabaseOption {
	return func(t testing.TB, ctx context.Context, testCtx *testContext) types.Database {
		postgresServer, err := postgres.NewTestServer(common.TestServerConfig{
			Name:         name,
			AuthClient:   testCtx.authClient,
			ClientAuth:   tls.RequireAndVerifyClientCert,
			AllowAnyUser: false,
			Users:        users,
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
		for _, dbOpt := range dbOpts {
			dbOpt(database)
		}
		testCtx.postgres[name] = testPostgres{
			db:       postgresServer,
			resource: database,
		}
		return database
	}
}

func withRDSPostgres(name, authToken string) withDatabaseOption {
	return func(t testing.TB, ctx context.Context, testCtx *testContext) types.Database {
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
			TLS: types.DatabaseTLS{
				CACert: string(testCtx.databaseCA.GetActiveKeys().TLS[0].Cert),
			},
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
	return func(t testing.TB, ctx context.Context, testCtx *testContext) types.Database {
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
			TLS: types.DatabaseTLS{
				CACert: string(testCtx.databaseCA.GetActiveKeys().TLS[0].Cert),
			},
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
	return func(t testing.TB, ctx context.Context, testCtx *testContext) types.Database {
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
			TLS: types.DatabaseTLS{
				CACert: string(testCtx.databaseCA.GetActiveKeys().TLS[0].Cert),
			},
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
	return func(t testing.TB, ctx context.Context, testCtx *testContext) types.Database {
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
			TLS: types.DatabaseTLS{
				CACert: string(testCtx.databaseCA.GetActiveKeys().TLS[0].Cert),
			},
		})
		require.NoError(t, err)
		testCtx.postgres[name] = testPostgres{
			db:       postgresServer,
			resource: database,
		}
		return database
	}
}

type selfHostedMySQLOptions struct {
	serverOptions   []mysql.TestServerOption
	databaseOptions []databaseOption
}

type selfHostedMySQLOption func(*selfHostedMySQLOptions)

func withMySQLServerVersion(version string) selfHostedMySQLOption {
	return func(opts *selfHostedMySQLOptions) {
		opts.serverOptions = append(opts.serverOptions, mysql.WithServerVersion(version))
	}
}

func withMySQLServerVersionInDBSpec(version string) selfHostedMySQLOption {
	return func(opts *selfHostedMySQLOptions) {
		opts.databaseOptions = append(opts.databaseOptions, func(db *types.DatabaseV3) {
			db.Spec.MySQL.ServerVersion = version
		})
	}
}

func withMySQLAdminUser(username string) selfHostedMySQLOption {
	return func(opts *selfHostedMySQLOptions) {
		opts.databaseOptions = append(opts.databaseOptions, func(db *types.DatabaseV3) {
			db.Spec.AdminUser = &types.DatabaseAdminUser{
				Name: username,
			}
		})
	}
}

func withSelfHostedMySQL(name string, applyOpts ...selfHostedMySQLOption) withDatabaseOption {
	return func(t testing.TB, ctx context.Context, testCtx *testContext) types.Database {
		opts := selfHostedMySQLOptions{}
		for _, applyOpt := range applyOpts {
			applyOpt(&opts)
		}

		mysqlServer, err := mysql.NewTestServer(common.TestServerConfig{
			Name:       name,
			AuthClient: testCtx.authClient,
			ClientAuth: tls.RequireAndVerifyClientCert,
		}, opts.serverOptions...)
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

		for _, applyDatabaseOpt := range opts.databaseOptions {
			applyDatabaseOpt(database)
		}

		testCtx.mysql[name] = testMySQL{
			db:       mysqlServer,
			resource: database,
		}
		return database
	}
}

func withRDSMySQL(name, authUser, authToken string) withDatabaseOption {
	return func(t testing.TB, ctx context.Context, testCtx *testContext) types.Database {
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
			CACert: string(testCtx.databaseCA.GetActiveKeys().TLS[0].Cert),
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
	return func(t testing.TB, ctx context.Context, testCtx *testContext) types.Database {
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
			CACert: string(testCtx.databaseCA.GetActiveKeys().TLS[0].Cert),
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
	return func(t testing.TB, ctx context.Context, testCtx *testContext) types.Database {
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
			CACert: string(testCtx.databaseCA.GetActiveKeys().TLS[0].Cert),
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
	return func(t testing.TB, ctx context.Context, testCtx *testContext) types.Database {
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
			CACert: string(testCtx.databaseCA.GetActiveKeys().TLS[0].Cert),
		})
		require.NoError(t, err)
		testCtx.mysql[name] = testMySQL{
			db:       mysqlServer,
			resource: database,
		}
		return database
	}
}

func withAtlasMongo(name, authUser, authSession string) withDatabaseOption {
	return func(t testing.TB, ctx context.Context, testCtx *testContext) types.Database {
		mongoServer, err := mongodb.NewTestServer(common.TestServerConfig{
			Name:       name,
			AuthClient: testCtx.authClient,
			AuthUser:   authUser,
			AuthToken:  authSession,
		})
		require.NoError(t, err)
		go mongoServer.Serve()
		t.Cleanup(func() { mongoServer.Close() })
		database, err := types.NewDatabaseV3(types.Metadata{
			Name: name,
		}, types.DatabaseSpecV3{
			Protocol:      defaults.ProtocolMongoDB,
			URI:           net.JoinHostPort("localhost", mongoServer.Port()),
			DynamicLabels: dynamicLabels,
			AWS: types.AWS{
				AccountID: "000000000000",
			},
			MongoAtlas: types.MongoAtlas{
				Name: "test",
			},
			CACert: string(testCtx.databaseCA.GetActiveKeys().TLS[0].Cert),
		})
		require.NoError(t, err)
		testCtx.mongo[name] = testMongoDB{
			db:       mongoServer,
			resource: database,
		}
		return database
	}
}

func withSelfHostedMongo(name string, opts ...mongodb.TestServerOption) withDatabaseOption {
	return withSelfHostedMongoWithAdminUser(name, "", opts...)
}

func withSelfHostedMongoWithAdminUser(name, adminUsername string, opts ...mongodb.TestServerOption) withDatabaseOption {
	return func(t testing.TB, ctx context.Context, testCtx *testContext) types.Database {
		mongoServer, err := mongodb.NewTestServer(common.TestServerConfig{
			Name:       name,
			AuthClient: testCtx.authClient,
			ClientAuth: tls.RequireAndVerifyClientCert,
		}, opts...)
		require.NoError(t, err)
		go mongoServer.Serve()
		t.Cleanup(func() { mongoServer.Close() })

		var adminUser *types.DatabaseAdminUser
		if adminUsername != "" {
			adminUser = &types.DatabaseAdminUser{
				Name: adminUsername,
			}
		}

		database, err := types.NewDatabaseV3(types.Metadata{
			Name: name,
		}, types.DatabaseSpecV3{
			Protocol:      defaults.ProtocolMongoDB,
			URI:           net.JoinHostPort("localhost", mongoServer.Port()),
			DynamicLabels: dynamicLabels,
			AdminUser:     adminUser,
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
	return func(t testing.TB, ctx context.Context, testCtx *testContext) types.Database {
		redisServer, err := redis.NewTestServer(t, common.TestServerConfig{
			Name:       name,
			AuthClient: testCtx.authClient,
			ClientAuth: tls.RequireAndVerifyClientCert,
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
	return func(t testing.TB, ctx context.Context, testCtx *testContext) types.Database {
		sqlServer, err := sqlserver.NewTestServer(common.TestServerConfig{
			Name:       name,
			AuthClient: testCtx.authClient,
		})
		require.NoError(t, err)
		go sqlServer.Serve()
		t.Cleanup(func() { sqlServer.Close() })
		database, err := types.NewDatabaseV3(types.Metadata{
			Name: name,
		}, types.DatabaseSpecV3{
			Protocol: defaults.ProtocolSQLServer,
			URI:      net.JoinHostPort("localhost", sqlServer.Port()),
		})
		require.NoError(t, err)
		testCtx.sqlServer[name] = testSQLServer{
			db:       sqlServer,
			resource: database,
		}
		return database
	}
}

func withClickhouseNative(name string) withDatabaseOption {
	return func(t testing.TB, ctx context.Context, testCtx *testContext) types.Database {
		server, err := clickhouse.NewTestServer(common.TestServerConfig{
			Name:       name,
			AuthClient: testCtx.authClient,
		}, clickhouse.WithClickHouseNativeProtocol())
		require.NoError(t, err)
		go server.Serve()
		t.Cleanup(func() {
			server.Close()
		})
		database, err := types.NewDatabaseV3(types.Metadata{
			Name: name,
		}, types.DatabaseSpecV3{
			Protocol: defaults.ProtocolClickHouse,
			URI:      fmt.Sprintf("clickhouse://%s", net.JoinHostPort("localhost", server.Port())),
		})
		require.NoError(t, err)
		testCtx.clickHouse[name] = testClickHouse{
			db:       server,
			resource: database,
		}
		return database
	}
}

func withClickhouseHTTP(name string) withDatabaseOption {
	return func(t testing.TB, ctx context.Context, testCtx *testContext) types.Database {
		server, err := clickhouse.NewTestServer(common.TestServerConfig{
			Name:       name,
			AuthClient: testCtx.authClient,
		}, clickhouse.WithClickHouseHTTPProtocol())
		require.NoError(t, err)
		go server.Serve()
		t.Cleanup(func() { server.Close() })
		database, err := types.NewDatabaseV3(types.Metadata{
			Name: name,
		}, types.DatabaseSpecV3{
			Protocol: defaults.ProtocolClickHouseHTTP,
			URI:      fmt.Sprintf("https://%s", net.JoinHostPort("localhost", server.Port())),
		})
		require.NoError(t, err)
		testCtx.clickHouse[name] = testClickHouse{
			db:       server,
			resource: database,
		}
		return database
	}
}

func withElastiCacheRedis(name string, token, engineVersion string) withDatabaseOption {
	return func(t testing.TB, ctx context.Context, testCtx *testContext) types.Database {
		redisServer, err := redis.NewTestServer(t, common.TestServerConfig{
			Name:       name,
			AuthClient: testCtx.authClient,
		}, redis.TestServerPassword(token))
		require.NoError(t, err)

		database, err := types.NewDatabaseV3(types.Metadata{
			Name: name,
			Labels: map[string]string{
				"engine-version": engineVersion,
			},
		}, types.DatabaseSpecV3{
			Protocol:      defaults.ProtocolRedis,
			URI:           fmt.Sprintf("rediss://%s", net.JoinHostPort("localhost", redisServer.Port())),
			DynamicLabels: dynamicLabels,
			AWS: types.AWS{
				Region: "us-west-1",
				ElastiCache: types.ElastiCache{
					ReplicationGroupID: "example-cluster",
				},
			},
			// Set CA cert to pass cert validation.
			TLS: types.DatabaseTLS{
				CACert: string(testCtx.databaseCA.GetActiveKeys().TLS[0].Cert),
			},
		})
		require.NoError(t, err)
		testCtx.redis[name] = testRedis{
			db:       redisServer,
			resource: database,
		}
		return database
	}
}

func withMemoryDBRedis(name string, token, engineVersion string) withDatabaseOption {
	return func(t testing.TB, ctx context.Context, testCtx *testContext) types.Database {
		redisServer, err := redis.NewTestServer(t, common.TestServerConfig{
			Name:       name,
			AuthClient: testCtx.authClient,
		}, redis.TestServerPassword(token))
		require.NoError(t, err)

		database, err := types.NewDatabaseV3(types.Metadata{
			Name: name,
			Labels: map[string]string{
				"engine-version": engineVersion,
			},
		}, types.DatabaseSpecV3{
			Protocol:      defaults.ProtocolRedis,
			URI:           fmt.Sprintf("rediss://%s", net.JoinHostPort("localhost", redisServer.Port())),
			DynamicLabels: dynamicLabels,
			AWS: types.AWS{
				Region: "us-west-1",
				MemoryDB: types.MemoryDB{
					ClusterName: "example-cluster",
				},
			},
			// Set CA cert to pass cert validation.
			TLS: types.DatabaseTLS{
				CACert: string(testCtx.databaseCA.GetActiveKeys().TLS[0].Cert),
			},
		})
		require.NoError(t, err)
		testCtx.redis[name] = testRedis{
			db:       redisServer,
			resource: database,
		}
		return database
	}
}

func withAzureRedis(name string, token string) withDatabaseOption {
	return func(t testing.TB, ctx context.Context, testCtx *testContext) types.Database {
		redisServer, err := redis.NewTestServer(t, common.TestServerConfig{
			Name:       name,
			AuthClient: testCtx.authClient,
		}, redis.TestServerPassword(token))
		require.NoError(t, err)

		database, err := types.NewDatabaseV3(types.Metadata{
			Name: name,
		}, types.DatabaseSpecV3{
			Protocol:      defaults.ProtocolRedis,
			URI:           fmt.Sprintf("rediss://%s", net.JoinHostPort("localhost", redisServer.Port())),
			DynamicLabels: dynamicLabels,
			Azure: types.Azure{
				Name:       name,
				ResourceID: "/subscriptions/sub-id/resourceGroups/group-name/providers/Microsoft.Cache/Redis/example-teleport",
			},
			// Set CA cert to pass cert validation.
			TLS: types.DatabaseTLS{
				CACert: string(testCtx.databaseCA.GetActiveKeys().TLS[0].Cert),
			},
		})
		require.NoError(t, err)
		testCtx.redis[name] = testRedis{
			db:       redisServer,
			resource: database,
		}
		return database
	}
}

type fakeDiscoveryResourceChecker struct {
	byName map[string]func(context.Context, types.Database) error
}

func (f *fakeDiscoveryResourceChecker) Check(ctx context.Context, database types.Database) error {
	if len(f.byName) > 0 {
		if check := f.byName[database.GetName()]; check != nil {
			return trace.Wrap(check(ctx, database))
		}
	}
	return nil
}

var dynamicLabels = types.LabelsToV2(map[string]types.CommandLabel{
	"echo": &types.CommandLabelV2{
		Period:  types.NewDuration(time.Second),
		Command: []string{"echo", "test"},
	},
})

// testAWSRegion is the AWS region used in tests.
const testAWSRegion = "us-east-1"

type fakeObjectsImporter struct{}

func (fakeObjectsImporter) StartImporter(ctx context.Context, database types.Database) error {
	return objects.NewErrFetcherDisabled("importer is disabled in fake object importer for test")
}

func (fakeObjectsImporter) StopImporter(databaseName string) error {
	return objects.NewErrFetcherDisabled("importer is disabled in fake object importer for test")
}
