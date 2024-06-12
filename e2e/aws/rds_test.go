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

package e2e

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	mysqlclient "github.com/go-mysql-org/go-mysql/client"
	"github.com/go-mysql-org/go-mysql/mysql"
	"github.com/jackc/pgx/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integration/helpers"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

// makeDBTestCluster is a test helper to set up a typical test cluster for
// database e2e tests.
func makeDBTestCluster(t *testing.T, accessRole, discoveryRole, discoveryMatcherType string, opts ...testOptionsFunc) *helpers.TeleInstance {
	t.Helper()
	opts = append([]testOptionsFunc{
		withSingleProxyPort(t),
		withDiscoveryService(t, "db-e2e-test", types.AWSMatcher{
			Types:   []string{discoveryMatcherType},
			Tags:    mustGetDiscoveryMatcherLabels(t),
			Regions: []string{mustGetEnv(t, awsRegionEnv)},
			AssumeRole: &types.AssumeRole{
				RoleARN: discoveryRole,
			},
		}),
		withDatabaseService(t, services.ResourceMatcher{
			Labels: types.Labels{types.Wildcard: {types.Wildcard}},
			AWS: services.ResourceMatcherAWS{
				AssumeRoleARN: accessRole,
			},
		}),
		withFullDatabaseAccessUserRole(t),
	}, opts...)
	return createTeleportCluster(t, opts...)
}

// testRDS tests AWS RDS database discovery and connections.
// Since RDS has many different db engines available, this test groups all
// the engines together into subtests: postgres, mysql, etc.
func testRDS(t *testing.T) {
	t.Parallel()
	// give everything 2 minutes to finish. Realistically it takes ~10-20
	// seconds, but let's be generous to maybe avoid flakey failures.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)

	// use random names so we can test auto provisioning these users with these
	// roles via Teleport, without tests colliding with eachother across
	// parallel test runs.
	autoUserKeep := "auto_keep_" + randASCII(t, 6)
	autoUserDrop := "auto_drop_" + randASCII(t, 6)
	autoRole1 := "auto_role1_" + randASCII(t, 6)
	autoRole2 := "auto_role2_" + randASCII(t, 6)

	accessRole := mustGetEnv(t, rdsAccessRoleEnv)
	discoveryRole := mustGetEnv(t, rdsDiscoveryRoleEnv)
	opts := []testOptionsFunc{
		withUserRole(t, autoUserKeep, "db-auto-user-keeper", makeAutoUserKeepRoleSpec(autoRole1, autoRole2)),
		withUserRole(t, autoUserDrop, "db-auto-user-dropper", makeAutoUserDropRoleSpec(autoRole1, autoRole2)),
	}
	cluster := makeDBTestCluster(t, accessRole, discoveryRole, types.AWSMatcherRDS, opts...)

	t.Run("postgres", func(t *testing.T) {
		t.Parallel()

		// wait for the database to be discovered
		pgDBName := mustGetEnv(t, rdsPostgresInstanceNameEnv)
		waitForDatabases(t, cluster.Process, pgDBName)
		db, err := cluster.Process.GetAuthServer().GetDatabase(ctx, pgDBName)
		require.NoError(t, err)
		adminUser := mustGetDBAdmin(t, db)

		conn := connectAsRDSPostgresAdmin(t, ctx, db.GetAWS().RDS.InstanceID)
		provisionRDSPostgresAutoUsersAdmin(t, ctx, conn, adminUser.Name)

		// create a new schema with tables that can only be accessed if the
		// auto roles are granted by Teleport automatically.
		testSchema := "test_" + randASCII(t, 4)
		_, err = conn.Exec(ctx, fmt.Sprintf("CREATE SCHEMA %q", testSchema))
		require.NoError(t, err)
		testTable := "ctf" // capture the flag :)
		_, err = conn.Exec(ctx, fmt.Sprintf("CREATE TABLE %q.%q ()", testSchema, testTable))
		require.NoError(t, err)

		// create the roles that Teleport will auto assign.
		// role 1 only allows usage of the test schema.
		// role 2 only allows select of the test table in the test schema.
		// a user needs to have both roles to select from the test table.
		_, err = conn.Exec(ctx, fmt.Sprintf("CREATE ROLE %q", autoRole1))
		require.NoError(t, err)
		_, err = conn.Exec(ctx, fmt.Sprintf("CREATE ROLE %q", autoRole2))
		require.NoError(t, err)
		_, err = conn.Exec(ctx, fmt.Sprintf("GRANT USAGE ON SCHEMA %q TO %q", testSchema, autoRole1))
		require.NoError(t, err)
		_, err = conn.Exec(ctx, fmt.Sprintf("GRANT SELECT ON %q.%q TO %q", testSchema, testTable, autoRole2))
		require.NoError(t, err)
		autoRolesQuery := fmt.Sprintf("select 1 from %q.%q", testSchema, testTable)

		t.Cleanup(func() {
			// best effort cleanup everything created for the tests,
			// including the auto drop user in case Teleport fails to do so.
			_, _ = conn.Exec(ctx, fmt.Sprintf("DROP SCHEMA %q CASCADE", testSchema))
			_, _ = conn.Exec(ctx, fmt.Sprintf("DROP ROLE %q", autoRole1))
			_, _ = conn.Exec(ctx, fmt.Sprintf("DROP ROLE %q", autoRole2))
			_, _ = conn.Exec(ctx, fmt.Sprintf("DROP USER %q", autoUserKeep))
			_, _ = conn.Exec(ctx, fmt.Sprintf("DROP USER %q", autoUserDrop))
		})

		var pgxConnMu sync.Mutex
		for name, test := range map[string]struct {
			user            string
			dbUser          string
			query           string
			afterConnTestFn func(t *testing.T)
		}{
			"existing user": {
				user:   hostUser,
				dbUser: adminUser.Name, // admin user already has RDS IAM auth
				query:  "select 1",
			},
			"auto user keep": {
				user:   autoUserKeep,
				dbUser: autoUserKeep,
				query:  autoRolesQuery,
				afterConnTestFn: func(t *testing.T) {
					pgxConnMu.Lock()
					defer pgxConnMu.Unlock()
					waitForPostgresAutoUserDeactivate(t, ctx, conn, autoUserKeep)
				},
			},
			"auto user drop": {
				user:   autoUserDrop,
				dbUser: autoUserDrop,
				query:  autoRolesQuery,
				afterConnTestFn: func(t *testing.T) {
					pgxConnMu.Lock()
					defer pgxConnMu.Unlock()
					waitForPostgresAutoUserDrop(t, ctx, conn, autoUserDrop)
				},
			},
		} {
			test := test
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				t.Run("connect", func(t *testing.T) {
					route := tlsca.RouteToDatabase{
						ServiceName: db.GetName(),
						Protocol:    defaults.ProtocolPostgres,
						Username:    test.dbUser,
						Database:    "postgres",
					}
					t.Run("via proxy", func(t *testing.T) {
						t.Parallel()
						postgresConnTest(t, cluster, test.user, route, test.query)
					})
					t.Run("via local proxy", func(t *testing.T) {
						t.Parallel()
						postgresLocalProxyConnTest(t, cluster, test.user, route, test.query)
					})
				})
				if test.afterConnTestFn != nil {
					test.afterConnTestFn(t)
				}
			})
		}
	})

	t.Run("mysql", func(t *testing.T) {
		t.Parallel()

		// wait for the database to be discovered
		mysqlDBName := mustGetEnv(t, rdsMySQLInstanceNameEnv)
		waitForDatabases(t, cluster.Process, mysqlDBName)
		db, err := cluster.Process.GetAuthServer().GetDatabase(ctx, mysqlDBName)
		require.NoError(t, err)
		adminUser := mustGetDBAdmin(t, db)

		conn := connectAsRDSMySQLAdmin(t, ctx, db.GetAWS().RDS.InstanceID)
		provisionRDSMySQLAutoUsersAdmin(t, ctx, conn, adminUser.Name)

		// create a couple test tables to test role assignment with.
		testTable1 := "teleport.test_" + randASCII(t, 4)
		_, err = conn.Execute(fmt.Sprintf("CREATE TABLE %s (x int)", testTable1))
		require.NoError(t, err)
		testTable2 := "teleport.test_" + randASCII(t, 4)
		_, err = conn.Execute(fmt.Sprintf("CREATE TABLE %s (x int)", testTable2))
		require.NoError(t, err)

		// create the roles that Teleport will auto assign.
		// role 1 only allows SELECT on test table 1.
		// role 2 only allows SELECT on test table 2.
		// a user needs to have both roles to select from a join of the tables.
		_, err = conn.Execute(fmt.Sprintf("CREATE ROLE %q", autoRole1))
		require.NoError(t, err)
		_, err = conn.Execute(fmt.Sprintf("CREATE ROLE %q", autoRole2))
		require.NoError(t, err)
		_, err = conn.Execute(fmt.Sprintf("GRANT SELECT on %s TO %q", testTable1, autoRole1))
		require.NoError(t, err)
		_, err = conn.Execute(fmt.Sprintf("GRANT SELECT on %s TO %q", testTable2, autoRole2))
		require.NoError(t, err)
		autoRolesQuery := fmt.Sprintf("SELECT 1 FROM %s JOIN %s", testTable1, testTable2)

		t.Cleanup(func() {
			// best effort cleanup all the users created for the tests,
			// including the auto drop user in case Teleport fails to do so.
			_, _ = conn.Execute(fmt.Sprintf("DROP TABLE %s", testTable1))
			_, _ = conn.Execute(fmt.Sprintf("DROP TABLE %s", testTable2))
			_, _ = conn.Execute(fmt.Sprintf("DROP ROLE %q", autoRole1))
			_, _ = conn.Execute(fmt.Sprintf("DROP ROLE %q", autoRole2))
			_, _ = conn.Execute(fmt.Sprintf("DROP USER %q", autoUserKeep))
			_, _ = conn.Execute(fmt.Sprintf("DROP USER %q", autoUserDrop))
		})

		for name, test := range map[string]struct {
			user            string
			dbUser          string
			query           string
			afterConnTestFn func(t *testing.T)
		}{
			"existing user": {
				user:   hostUser,
				dbUser: adminUser.Name, // admin user already has RDS IAM auth
				query:  "select 1",
			},
			"auto user keep": {
				user:   autoUserKeep,
				dbUser: autoUserKeep,
				query:  autoRolesQuery,
				afterConnTestFn: func(t *testing.T) {
					waitForMySQLAutoUserDeactivate(t, conn, autoUserKeep)
				},
			},
			"auto user drop": {
				user:   autoUserDrop,
				dbUser: autoUserDrop,
				query:  autoRolesQuery,
				afterConnTestFn: func(t *testing.T) {
					waitForMySQLAutoUserDrop(t, conn, autoUserDrop)
				},
			},
		} {
			test := test
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				route := tlsca.RouteToDatabase{
					ServiceName: mysqlDBName,
					Protocol:    defaults.ProtocolMySQL,
					Username:    test.dbUser,
					Database:    "", // not needed
				}
				t.Run("connect", func(t *testing.T) {
					// run multiple conn tests in parallel to test parallel
					// auto user connections.
					t.Run("via local proxy 1", func(t *testing.T) {
						t.Parallel()
						mysqlLocalProxyConnTest(t, cluster, test.user, route, test.query)
					})
					t.Run("via local proxy 2", func(t *testing.T) {
						t.Parallel()
						mysqlLocalProxyConnTest(t, cluster, test.user, route, test.query)
					})
				})
				if test.afterConnTestFn != nil {
					test.afterConnTestFn(t)
				}
			})
		}
	})

	t.Run("mariadb", func(t *testing.T) {
		t.Parallel()

		// wait for the database to be discovered
		mariaDBName := mustGetEnv(t, rdsMariaDBInstanceNameEnv)
		waitForDatabases(t, cluster.Process, mariaDBName)
		db, err := cluster.Process.GetAuthServer().GetDatabase(ctx, mariaDBName)
		require.NoError(t, err)
		adminUser := mustGetDBAdmin(t, db)

		// connect as the RDS database admin user - not to be confused
		// with Teleport's "db admin user".
		conn := connectAsRDSMySQLAdmin(t, ctx, db.GetAWS().RDS.InstanceID)
		provisionMariaDBAdminUser(t, ctx, conn, adminUser.Name)

		// create a couple test tables to test role assignment with.
		testTable1 := "teleport.test_" + randASCII(t, 4)
		_, err = conn.Execute(fmt.Sprintf("CREATE TABLE %s (x int)", testTable1))
		require.NoError(t, err)
		t.Cleanup(func() {
			_, _ = conn.Execute(fmt.Sprintf("DROP TABLE %s", testTable1))
		})
		testTable2 := "teleport.test_" + randASCII(t, 4)
		_, err = conn.Execute(fmt.Sprintf("CREATE TABLE %s (x int)", testTable2))
		require.NoError(t, err)
		t.Cleanup(func() {
			_, _ = conn.Execute(fmt.Sprintf("DROP TABLE %s", testTable2))
		})

		// create the roles that Teleport will auto assign.
		// role 1 only allows SELECT on test table 1.
		// role 2 only allows SELECT on test table 2.
		// a user needs to have both roles to select from a join of the tables.
		_, err = conn.Execute(fmt.Sprintf("CREATE ROLE %q", autoRole1))
		require.NoError(t, err)
		t.Cleanup(func() {
			_, _ = conn.Execute(fmt.Sprintf("DROP ROLE %q", autoRole1))
		})
		_, err = conn.Execute(fmt.Sprintf("CREATE ROLE %q", autoRole2))
		require.NoError(t, err)
		t.Cleanup(func() {
			_, _ = conn.Execute(fmt.Sprintf("DROP ROLE %q", autoRole2))
		})
		_, err = conn.Execute(fmt.Sprintf("GRANT SELECT on %s TO %q", testTable1, autoRole1))
		require.NoError(t, err)
		_, err = conn.Execute(fmt.Sprintf("GRANT SELECT on %s TO %q", testTable2, autoRole2))
		require.NoError(t, err)

		// db admin needs the admin option for a role to grant others that role.
		_, err = conn.Execute(fmt.Sprintf("GRANT %q TO %q WITH ADMIN OPTION", autoRole1, adminUser.Name))
		require.NoError(t, err)
		_, err = conn.Execute(fmt.Sprintf("GRANT %q TO %q WITH ADMIN OPTION", autoRole2, adminUser.Name))
		require.NoError(t, err)
		autoRolesQuery := fmt.Sprintf("SELECT 1 FROM %s JOIN %s", testTable1, testTable2)

		t.Cleanup(func() {
			// best effort cleanup all the users created by the tests.
			// don't cleanup the admin or test runs will interfere with
			// each other.
			_, _ = conn.Execute(fmt.Sprintf("DROP ROLE %q", "tp-role-"+autoUserKeep))
			_, _ = conn.Execute(fmt.Sprintf("DROP ROLE %q", "tp-role-"+autoUserDrop))
			_, _ = conn.Execute(fmt.Sprintf("DROP USER %q", autoUserKeep))
			_, _ = conn.Execute(fmt.Sprintf("DROP USER %q", autoUserDrop))
			_, _ = conn.Execute("DELETE FROM teleport.user_attributes WHERE USER=?", autoUserKeep)
			_, _ = conn.Execute("DELETE FROM teleport.user_attributes WHERE USER=?", autoUserDrop)
		})

		for name, test := range map[string]struct {
			user            string
			dbUser          string
			query           string
			afterConnTestFn func(t *testing.T)
		}{
			"existing user": {
				user:   hostUser,
				dbUser: adminUser.Name,
				query:  "select 1",
			},
			"auto user keep": {
				user:   autoUserKeep,
				dbUser: autoUserKeep,
				query:  autoRolesQuery,
				afterConnTestFn: func(t *testing.T) {
					waitForMariaDBAutoUserDeactivate(t, conn, autoUserKeep)
				},
			},
			"auto user drop": {
				user:   autoUserDrop,
				dbUser: autoUserDrop,
				query:  autoRolesQuery,
				afterConnTestFn: func(t *testing.T) {
					waitForMariaDBAutoUserDrop(t, conn, autoUserDrop)
				},
			},
		} {
			test := test
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				route := tlsca.RouteToDatabase{
					ServiceName: mariaDBName,
					Protocol:    defaults.ProtocolMySQL,
					Username:    test.dbUser,
					Database:    "", // not needed
				}
				t.Run("connect", func(t *testing.T) {
					// run multiple conn tests in parallel to test parallel
					// auto user connections.
					t.Run("via local proxy 1", func(t *testing.T) {
						t.Parallel()
						mysqlLocalProxyConnTest(t, cluster, test.user, route, test.query)
					})
					t.Run("via local proxy 2", func(t *testing.T) {
						t.Parallel()
						mysqlLocalProxyConnTest(t, cluster, test.user, route, test.query)
					})
				})
				if test.afterConnTestFn != nil {
					test.afterConnTestFn(t)
				}
			})
		}
	})
}

// rdsAdminInfo contains common info needed to connect as an RDS admin user via
// password auth.
type rdsAdminInfo struct {
	address,
	username,
	password string
	port int
}

func connectAsRDSPostgresAdmin(t *testing.T, ctx context.Context, instanceID string) *pgx.Conn {
	t.Helper()
	info := getRDSAdminInfo(t, ctx, instanceID)
	pgCfg, err := pgx.ParseConfig(fmt.Sprintf("postgres://%s:%d/?sslmode=verify-full", info.address, info.port))
	require.NoError(t, err)
	pgCfg.User = info.username
	pgCfg.Password = info.password
	pgCfg.Database = "postgres"
	pgCfg.TLSConfig = &tls.Config{
		ServerName: info.address,
		RootCAs:    awsCertPool.Clone(),
	}

	conn, err := pgx.ConnectConfig(ctx, pgCfg)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = conn.Close(ctx)
	})
	return conn
}

// mySQLConn wraps a go-mysql conn to provide a client that's thread safe.
type mySQLConn struct {
	mu   sync.Mutex
	conn *mysqlclient.Conn
}

func (c *mySQLConn) Execute(command string, args ...interface{}) (*mysql.Result, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.conn.Execute(command, args...)
}

func connectAsRDSMySQLAdmin(t *testing.T, ctx context.Context, instanceID string) *mySQLConn {
	t.Helper()
	const dbName = "mysql"
	info := getRDSAdminInfo(t, ctx, instanceID)

	opt := func(conn *mysqlclient.Conn) {
		conn.SetTLSConfig(&tls.Config{
			ServerName: info.address,
			RootCAs:    awsCertPool.Clone(),
		})
	}
	endpoint := fmt.Sprintf("%s:%d", info.address, info.port)
	conn, err := mysqlclient.Connect(endpoint, info.username, info.password, dbName, opt)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = conn.Close()
	})
	return &mySQLConn{conn: conn}
}

func getRDSAdminInfo(t *testing.T, ctx context.Context, instanceID string) rdsAdminInfo {
	t.Helper()
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(mustGetEnv(t, awsRegionEnv)),
	)
	require.NoError(t, err)

	rdsClt := rds.NewFromConfig(cfg)
	result, err := rdsClt.DescribeDBInstances(ctx, &rds.DescribeDBInstancesInput{
		DBInstanceIdentifier: &instanceID,
	})
	require.NoError(t, err)
	require.Len(t, result.DBInstances, 1)
	dbInstance := result.DBInstances[0]
	require.NotNil(t, dbInstance.MasterUsername)
	require.NotNil(t, dbInstance.MasterUserSecret)
	require.NotNil(t, dbInstance.MasterUserSecret.SecretArn)
	require.NotEmpty(t, *dbInstance.MasterUsername)
	require.NotEmpty(t, *dbInstance.MasterUserSecret.SecretArn)
	return rdsAdminInfo{
		address:  *dbInstance.Endpoint.Address,
		port:     int(*dbInstance.Endpoint.Port),
		username: *dbInstance.MasterUsername,
		password: getRDSMasterUserPassword(t, ctx, *dbInstance.MasterUserSecret.SecretArn),
	}
}

func getRDSMasterUserPassword(t *testing.T, ctx context.Context, secretID string) string {
	t.Helper()
	secretVal := getSecretValue(t, ctx, secretID)
	type rdsMasterSecret struct {
		User string `json:"username"`
		Pass string `json:"password"`
	}
	var secret rdsMasterSecret
	if err := json.Unmarshal([]byte(*secretVal.SecretString), &secret); err != nil {
		// being paranoid. I don't want to leak the secret string in test error
		// logs.
		require.FailNow(t, "error unmarshaling secret string")
	}
	if len(secret.Pass) == 0 {
		require.FailNow(t, "empty master user secret string")
	}
	return secret.Pass
}

func getSecretValue(t *testing.T, ctx context.Context, secretID string) *secretsmanager.GetSecretValueOutput {
	t.Helper()
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(mustGetEnv(t, awsRegionEnv)),
	)
	require.NoError(t, err)

	secretsClt := secretsmanager.NewFromConfig(cfg)
	secretVal, err := secretsClt.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{
		SecretId: &secretID,
	})
	require.NoError(t, err)
	return secretVal
}

// provisionRDSPostgresAutoUsersAdmin provisions an admin user suitable for auto-user
// provisioning.
func provisionRDSPostgresAutoUsersAdmin(t *testing.T, ctx context.Context, conn *pgx.Conn, adminUser string) {
	t.Helper()
	// Create the admin user and grant rds_iam so Teleport can auth
	// with IAM as an existing user.
	// Also needed so the auto-user admin can auto-provision others.
	// If the admin already exists, ignore errors - there's only
	// one admin because the admin has to own all the functions
	// we provision and creating a different admin for each test
	// is not necessary.
	// Don't cleanup the db admin after, because test runs would interfere
	// with each other.
	_, _ = conn.Exec(ctx, fmt.Sprintf("CREATE USER %q WITH login createrole", adminUser))
	_, err := conn.Exec(ctx, fmt.Sprintf("GRANT rds_iam TO %q WITH ADMIN OPTION", adminUser))
	if err != nil {
		require.ErrorContains(t, err, "already a member")
	}
}

// provisionRDSMySQLAutoUsersAdmin provisions an admin user suitable for auto-user
// provisioning.
func provisionRDSMySQLAutoUsersAdmin(t *testing.T, ctx context.Context, conn *mySQLConn, adminUser string) {
	t.Helper()
	// provision the IAM user to test with.
	// ignore errors from user creation. If the user doesn't exist
	// later steps will catch it. The error we might get back when
	// another test runner already created the admin is
	// unpredictable: all we need to know is the user exists for
	// test setup.
	// Don't cleanup the db admin after, because test runs would interfere
	// with each other.
	_, _ = conn.Execute(fmt.Sprintf("CREATE USER IF NOT EXISTS %q IDENTIFIED WITH AWSAuthenticationPlugin AS 'RDS'", adminUser))

	// these statements are all idempotent - they should not return
	// an error even if run in parallel by many test runners.
	_, err := conn.Execute(fmt.Sprintf("GRANT SELECT ON mysql.role_edges TO %q", adminUser))
	require.NoError(t, err)
	_, err = conn.Execute(fmt.Sprintf("GRANT PROCESS, ROLE_ADMIN, CREATE USER ON *.* TO %q", adminUser))
	require.NoError(t, err)
	_, err = conn.Execute("CREATE DATABASE IF NOT EXISTS `teleport`")
	require.NoError(t, err)
	_, err = conn.Execute(fmt.Sprintf("GRANT ALTER ROUTINE, CREATE ROUTINE, EXECUTE ON `teleport`.* TO %q", adminUser))
	require.NoError(t, err)
}

// provisionMariaDBAdminUser provisions an admin user suitable for auto-user
// provisioning.
func provisionMariaDBAdminUser(t *testing.T, ctx context.Context, conn *mySQLConn, adminUser string) {
	t.Helper()
	// provision the IAM user to test with.
	// ignore errors from user creation. If the user doesn't exist
	// later steps will catch it. The error we might get back when
	// another test runner already created the admin is
	// unpredictable: all we need to know is the user exists for
	// test setup.
	_, _ = conn.Execute(fmt.Sprintf("CREATE USER IF NOT EXISTS %q IDENTIFIED WITH AWSAuthenticationPlugin AS 'RDS'", adminUser))

	// these statements are all idempotent - they should not return
	// an error even if run in parallel by many test runners.
	_, err := conn.Execute(fmt.Sprintf("GRANT PROCESS, CREATE USER ON *.* TO %q", adminUser))
	require.NoError(t, err)
	_, err = conn.Execute(fmt.Sprintf("GRANT SELECT ON mysql.roles_mapping to %q", adminUser))
	require.NoError(t, err)
	_, err = conn.Execute(fmt.Sprintf("GRANT UPDATE ON mysql.* TO %q", adminUser))
	require.NoError(t, err)
	_, err = conn.Execute(fmt.Sprintf("GRANT SELECT ON *.* TO %q", adminUser))
	require.NoError(t, err)
	_, err = conn.Execute("CREATE DATABASE IF NOT EXISTS `teleport`")
	require.NoError(t, err)
	_, err = conn.Execute(fmt.Sprintf("GRANT ALL ON `teleport`.* TO %q WITH GRANT OPTION", adminUser))
	require.NoError(t, err)
}

// randASCII is a helper func that returns a random string of ascii characters.
func randASCII(t *testing.T, length int) string {
	t.Helper()
	out, err := utils.CryptoRandomHex(length / 2)
	require.NoError(t, err)
	return out
}

const (
	autoUserWaitDur  = 20 * time.Second
	autoUserWaitStep = 2 * time.Second
)

func waitForPostgresAutoUserDeactivate(t *testing.T, ctx context.Context, conn *pgx.Conn, user string) {
	t.Helper()
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		rows, err := conn.Query(ctx, "SELECT 1 FROM pg_roles WHERE rolname=$1", user)
		if !assert.NoError(c, err) {
			return
		}
		if !assert.True(c, rows.Next(), "user %q should not have been dropped after disconnecting", user) {
			rows.Close()
			return
		}
		rows.Close()

		rows, err = conn.Query(ctx, "SELECT 1 FROM pg_roles WHERE rolname = $1 AND rolcanlogin = false", user)
		if !assert.NoError(c, err) {
			return
		}
		if !assert.True(c, rows.Next(), "user %q should not be able to login after deactivating", user) {
			rows.Close()
			return
		}
		rows.Close()

		rows, err = conn.Query(ctx, "SELECT 1 FROM pg_roles AS a WHERE pg_has_role($1, a.oid, 'member') AND a.rolname NOT IN ($1, 'teleport-auto-user')", user)
		if !assert.NoError(c, err) {
			return
		}
		if !assert.False(c, rows.Next(), "user %q should have lost all additional roles after deactivating", user) {
			rows.Close()
			return
		}
		rows.Close()
	}, autoUserWaitDur, autoUserWaitStep, "waiting for auto user %q to be deactivated", user)
}

func waitForPostgresAutoUserDrop(t *testing.T, ctx context.Context, conn *pgx.Conn, user string) {
	t.Helper()
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		rows, err := conn.Query(ctx, "SELECT 1 FROM pg_roles WHERE rolname=$1", user)
		if !assert.NoError(c, err) {
			return
		}
		assert.False(c, rows.Next(), "user %q should have been dropped automatically after disconnecting", user)
		rows.Close()
	}, autoUserWaitDur, autoUserWaitStep, "waiting for auto user %q to be dropped", user)
}

func waitForMySQLAutoUserDeactivate(t *testing.T, conn *mySQLConn, user string) {
	t.Helper()
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		result, err := conn.Execute("SELECT 1 FROM mysql.user AS u WHERE u.user = ?", user)
		if !assert.NoError(c, err) {
			return
		}
		if !assert.Equal(c, 1, result.RowNumber(), "user %q should not have been dropped after disconnecting", user) {
			result.Close()
			return
		}
		result.Close()

		result, err = conn.Execute("SELECT 1 FROM mysql.user AS u WHERE u.user = ? AND u.account_locked = 'Y'", user)
		if !assert.NoError(c, err) {
			return
		}
		if !assert.Equal(c, 1, result.RowNumber(), "user %q should not be able to login after deactivating", user) {
			result.Close()
			return
		}
		result.Close()

		result, err = conn.Execute("SELECT 1 FROM mysql.role_edges AS u WHERE u.to_user = ? AND u.from_user != 'teleport-auto-user'", user)
		if !assert.NoError(c, err) {
			return
		}
		if !assert.Equal(c, 0, result.RowNumber(), "user %q should have lost all additional roles after deactivating", user) {
			result.Close()
			return
		}
		result.Close()
	}, autoUserWaitDur, autoUserWaitStep, "waiting for auto user %q to be deactivated", user)
}

func waitForMySQLAutoUserDrop(t *testing.T, conn *mySQLConn, user string) {
	t.Helper()
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		result, err := conn.Execute("SELECT 1 FROM mysql.user AS u WHERE u.user = ?", user)
		if !assert.NoError(c, err) {
			return
		}
		assert.Equal(c, 0, result.RowNumber(), "user %q should have been dropped automatically after disconnecting", user)
		result.Close()
	}, autoUserWaitDur, autoUserWaitStep, "waiting for auto user %q to be dropped", user)
}

func waitForMariaDBAutoUserDeactivate(t *testing.T, conn *mySQLConn, user string) {
	t.Helper()
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		result, err := conn.Execute("SELECT 1 FROM mysql.user AS u WHERE u.user = ?", user)
		if !assert.NoError(c, err) {
			return
		}
		if !assert.Equal(c, 1, result.RowNumber(), "user %q should not have been dropped after disconnecting", user) {
			result.Close()
			return
		}
		result.Close()

		result, err = conn.Execute("SELECT 1 FROM mysql.global_priv AS u WHERE u.user = ? AND JSON_EXTRACT(u.priv, '$.account_locked') = true", user)
		if !assert.NoError(c, err) {
			return
		}
		if !assert.Equal(c, 1, result.RowNumber(), "user %q should not be able to login after deactivating", user) {
			result.Close()
			return
		}
		result.Close()

		result, err = conn.Execute("SELECT 1 FROM mysql.roles_mapping AS u WHERE u.user = ? AND u.role != 'teleport-auto-user' AND u.ADMIN_OPTION='N'", user)
		if !assert.NoError(c, err) {
			return
		}
		if !assert.Equal(c, 0, result.RowNumber(), "user %q should have lost all additional roles after deactivating", user) {
			result.Close()
			return
		}
		result.Close()
	}, autoUserWaitDur, autoUserWaitStep, "waiting for auto user %q to be deactivated", user)
}

func waitForMariaDBAutoUserDrop(t *testing.T, conn *mySQLConn, user string) {
	t.Helper()
	// run the same tests as mysql to check if the user was dropped.
	waitForMySQLAutoUserDrop(t, conn, user)
}
