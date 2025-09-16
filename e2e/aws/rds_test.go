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
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	mysqlclient "github.com/go-mysql-org/go-mysql/client"
	"github.com/go-mysql-org/go-mysql/mysql"
	"github.com/gravitational/trace"
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
	// Give everything some time to finish. Realistically it takes ~10-20
	// seconds, but let's be generous to maybe avoid flakey failures.
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	t.Cleanup(cancel)

	// use random names so we can test auto provisioning these users with these
	// roles via Teleport, without tests colliding with eachother across
	// parallel test runs.
	autoUserFineGrain := "auto_fine_grain_" + randASCII(t)
	autoUserKeep := "auto_keep_" + randASCII(t)
	autoUserDrop := "auto_drop_" + randASCII(t)
	autoUserFineGrain2 := "auto_fine_grain2_" + randASCII(t)
	autoUserKeep2 := "auto_keep2_" + randASCII(t)
	autoUserDrop2 := "auto_drop2_" + randASCII(t)
	autoRole1 := "auto_granted_role1_" + randASCII(t)
	autoRole2 := "auto_granted_role2_" + randASCII(t)

	testSchema := "test_" + randASCII(t)

	accessRole := mustGetEnv(t, rdsAccessRoleARNEnv)
	discoveryRole := mustGetEnv(t, rdsDiscoveryRoleARNEnv)
	dbAutoUserFineGrainRole := makeAutoUserDBPermissions(
		types.DatabasePermission{
			Permissions: []string{"SELECT"},
			Match: types.Labels{
				"object_kind": {"table"},
				"schema":      {"public", testSchema, "information_schema"},
			},
		},
		types.DatabasePermission{
			Permissions: []string{"SELECT"},
			Match: types.Labels{
				"object_kind": {"table"},
				"schema":      {"pg_catalog"},
				"name":        {"pg_range", "pg_proc"},
			},
		},
	)
	opts := []testOptionsFunc{
		withUserRole(t, autoUserFineGrain, "db-auto-user-fine-grain", dbAutoUserFineGrainRole),
		withUserRole(t, autoUserKeep, "db-auto-user-keeper", makeAutoUserKeepRoleSpec(autoRole1, autoRole2)),
		withUserRole(t, autoUserDrop, "db-auto-user-dropper", makeAutoUserDropRoleSpec(autoRole1, autoRole2)),
		withUserRole(t, autoUserFineGrain2, "db-auto-user-fine-grain", dbAutoUserFineGrainRole),
		withUserRole(t, autoUserKeep2, "db-auto-user-keeper", makeAutoUserKeepRoleSpec(autoRole1, autoRole2)),
		withUserRole(t, autoUserDrop2, "db-auto-user-dropper", makeAutoUserDropRoleSpec(autoRole1, autoRole2)),
	}
	cluster := makeDBTestCluster(t, accessRole, discoveryRole, types.AWSMatcherRDS, opts...)

	t.Run("postgres", func(t *testing.T) {
		t.Parallel()

		// wait for the database to be discovered
		pgDBName := mustGetEnv(t, rdsPostgresInstanceNameEnv)
		waitForDatabases(t, cluster.Process, pgDBName)
		db, err := cluster.Process.GetAuthServer().GetDatabase(ctx, pgDBName)
		require.NoError(t, err)
		// make sure we have set the db admin from labels
		_ = mustGetDBAdmin(t, db)

		// provision new databases with new db admin to have distinct admin names in concurrent test runs.
		// db1 admin *will not* be a Postgres superuser
		db1 := cloneDBWithNewAdmin(t, db, &types.DatabaseAdminUser{
			Name: "admin_" + randASCII(t),
		})
		require.NoError(t, cluster.Process.GetAuthServer().CreateDatabase(ctx, db1))
		// db2 admin *will* be a Postgres superuser
		db2 := cloneDBWithNewAdmin(t, db, &types.DatabaseAdminUser{
			Name: "su_admin_" + randASCII(t),
		})
		require.NoError(t, cluster.Process.GetAuthServer().CreateDatabase(ctx, db2))
		waitForDatabases(t, cluster.Process, db1.GetName(), db2.GetName())
		db1, err = cluster.Process.GetAuthServer().GetDatabase(ctx, db1.GetName())
		require.NoError(t, err)
		db2, err = cluster.Process.GetAuthServer().GetDatabase(ctx, db2.GetName())
		require.NoError(t, err)

		conn := connectAsRDSPostgresAdmin(t, ctx, db.GetAWS().RDS.InstanceID)

		// these users will be auto-created by Teleport, so make sure we clean
		// them up after the test.
		cleanupDB(t, ctx, conn, fmt.Sprintf("DROP ROLE IF EXISTS %q", autoUserKeep))
		cleanupDB(t, ctx, conn, fmt.Sprintf("DROP ROLE IF EXISTS %q", autoUserDrop))
		cleanupDB(t, ctx, conn, fmt.Sprintf("DROP ROLE IF EXISTS %q", autoUserFineGrain))
		cleanupDB(t, ctx, conn, fmt.Sprintf("DROP ROLE IF EXISTS %q", autoUserKeep2))
		cleanupDB(t, ctx, conn, fmt.Sprintf("DROP ROLE IF EXISTS %q", autoUserDrop2))
		cleanupDB(t, ctx, conn, fmt.Sprintf("DROP ROLE IF EXISTS %q", autoUserFineGrain2))

		// create the roles that Teleport will auto assign.
		for _, r := range [...]string{autoRole1, autoRole2} {
			createPGTestRole(t, ctx, conn, r)
		}

		// create a new schema with tables that can only be accessed if the
		// auto roles are granted by Teleport automatically.
		createPGTestSchema(t, ctx, conn, testSchema)
		testTable := "ctf" + randASCII(t) // capture the flag :)
		createPGTestTable(t, ctx, conn, testSchema, testTable)
		createPGTestTable(t, ctx, conn, "public", testTable)

		// provision db1 admin that is not a postgres superuser
		createPGTestUser(t, ctx, conn, db1.GetAdminUser().Name)
		pgMustExec(t, ctx, conn, fmt.Sprintf("ALTER USER %q WITH CREATEROLE", db1.GetAdminUser().Name))
		pgMustExec(t, ctx, conn, fmt.Sprintf("GRANT rds_iam TO %q WITH ADMIN OPTION", db1.GetAdminUser().Name))
		pgMustExec(t, ctx, conn, fmt.Sprintf("GRANT USAGE ON SCHEMA public, information_schema, %q TO %q WITH GRANT OPTION", testSchema, db1.GetAdminUser().Name))
		cleanupDB(t, ctx, conn, fmt.Sprintf("REVOKE USAGE ON SCHEMA public, information_schema, %q FROM %q", testSchema, db1.GetAdminUser().Name))
		pgMustExec(t, ctx, conn, fmt.Sprintf("GRANT ALL ON ALL TABLES IN SCHEMA public, information_schema, %q TO %q WITH GRANT OPTION", testSchema, db1.GetAdminUser().Name))
		cleanupDB(t, ctx, conn, fmt.Sprintf("REVOKE ALL ON ALL TABLES IN SCHEMA public, information_schema, %q FROM %q", testSchema, db1.GetAdminUser().Name))

		// provision db2 admin that IS a postgres super user
		createPGTestUser(t, ctx, conn, db2.GetAdminUser().Name)
		// granting rds_superuser is as close as we can get to a superuser in RDS Postgres
		pgMustExec(t, ctx, conn, fmt.Sprintf("GRANT rds_iam, rds_superuser TO %q", db2.GetAdminUser().Name))

		// auto role 1 only allows usage of the test schema.
		// auto role 2 only allows select of the test table in the test schema.
		// a user needs to have both roles to select from the test table.
		pgMustExec(t, ctx, conn, fmt.Sprintf("GRANT USAGE ON SCHEMA %q TO %q", testSchema, autoRole1))
		pgMustExec(t, ctx, conn, fmt.Sprintf("GRANT SELECT ON %q.%q TO %q", testSchema, testTable, autoRole2))

		autoRolesQuery := fmt.Sprintf("select 1 from %q.%q", testSchema, testTable)
		var pgxConnMu sync.Mutex
		for _, test := range []struct {
			name              string
			db                types.Database
			autoUserKeep      string
			autoUserDrop      string
			autoUserFineGrain string
		}{
			{
				name:              "non superuser db admin",
				db:                db1,
				autoUserKeep:      autoUserKeep,
				autoUserDrop:      autoUserDrop,
				autoUserFineGrain: autoUserFineGrain,
			},
			{
				name:              "superuser db admin",
				db:                db2,
				autoUserKeep:      autoUserKeep2,
				autoUserDrop:      autoUserDrop2,
				autoUserFineGrain: autoUserFineGrain2,
			},
		} {
			autoUserKeep := test.autoUserKeep
			autoUserDrop := test.autoUserDrop
			autoUserFineGrain := test.autoUserFineGrain
			db := test.db
			t.Run(test.name, func(t *testing.T) {
				t.Parallel()
				for name, test := range map[string]struct {
					user            string
					dbUser          string
					query           string
					afterConnTestFn func(t *testing.T)
				}{
					"existing user": {
						user:   hostUser,
						dbUser: db.GetAdminUser().Name, // admin user already has RDS IAM auth
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
					"db permissions": {
						user:   autoUserFineGrain,
						dbUser: autoUserFineGrain,
						query: fmt.Sprintf(`
							SELECT
								1
							FROM
								pg_catalog.pg_range,
								pg_catalog.pg_proc,
								information_schema.sql_parts,
								public.%q,
								%q.%q
							`, testTable, testSchema, testTable),
						afterConnTestFn: func(t *testing.T) {
							pgxConnMu.Lock()
							defer pgxConnMu.Unlock()
							waitForPostgresAutoUserPermissionsRemoved(t, ctx, conn, autoUserFineGrain)
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
		provisionRDSMySQLAutoUsersAdmin(t, conn, adminUser.Name)

		// create a couple test tables to test role assignment with.
		testTable1 := "teleport.test_" + randASCII(t)
		_, err = conn.Execute(fmt.Sprintf("CREATE TABLE %s (x int)", testTable1))
		require.NoError(t, err)
		testTable2 := "teleport.test_" + randASCII(t)
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
			for _, stmt := range []string{
				fmt.Sprintf("DROP TABLE %s", testTable1),
				fmt.Sprintf("DROP TABLE %s", testTable2),
				fmt.Sprintf("DROP ROLE IF EXISTS %q", autoRole1),
				fmt.Sprintf("DROP ROLE IF EXISTS %q", autoRole2),
				fmt.Sprintf("DROP USER IF EXISTS %q", autoUserKeep),
				fmt.Sprintf("DROP USER IF EXISTS %q", autoUserDrop),
			} {
				_, err := conn.Execute(stmt)
				assert.NoError(t, err, "test cleanup failed, stmt=%q", stmt)
			}
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
		provisionMariaDBAdminUser(t, conn, adminUser.Name)

		// create a couple test tables to test role assignment with.
		testTable1 := "teleport.test_" + randASCII(t)
		_, err = conn.Execute(fmt.Sprintf("CREATE TABLE %s (x int)", testTable1))
		require.NoError(t, err)
		t.Cleanup(func() {
			_, _ = conn.Execute(fmt.Sprintf("DROP TABLE %s", testTable1))
		})
		testTable2 := "teleport.test_" + randASCII(t)
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
			// best effort cleanup all the users created for the tests,
			// including the auto drop user in case Teleport fails to do so.
			for _, stmt := range []string{
				fmt.Sprintf("DROP ROLE IF EXISTS %q", "tp-role-"+autoUserKeep),
				fmt.Sprintf("DROP ROLE IF EXISTS %q", "tp-role-"+autoUserDrop),
				fmt.Sprintf("DROP USER IF EXISTS %q", autoUserKeep),
				fmt.Sprintf("DROP USER IF EXISTS %q", autoUserDrop),
				fmt.Sprintf("DELETE FROM teleport.user_attributes WHERE USER=%q", autoUserKeep),
				fmt.Sprintf("DELETE FROM teleport.user_attributes WHERE USER=%q", autoUserDrop),
			} {
				_, err := conn.Execute(stmt)
				assert.NoError(t, err, "test cleanup failed, stmt=%q", stmt)
			}
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

func connectAsRDSPostgresAdmin(t *testing.T, ctx context.Context, instanceID string) *pgConn {
	t.Helper()
	info := getRDSAdminInfo(t, ctx, instanceID)
	const dbName = "postgres"
	return connectPostgres(t, ctx, info, dbName)
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

func getRDSAdminInfo(t *testing.T, ctx context.Context, instanceID string) dbUserLogin {
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
	return dbUserLogin{
		username: *dbInstance.MasterUsername,
		password: getMasterUserPassword(t, ctx, *dbInstance.MasterUserSecret.SecretArn),
		address:  *dbInstance.Endpoint.Address,
		port:     int(*dbInstance.Endpoint.Port),
	}
}

// provisionRDSMySQLAutoUsersAdmin provisions an admin user suitable for auto-user
// provisioning.
func provisionRDSMySQLAutoUsersAdmin(t *testing.T, conn *mySQLConn, adminUser string) {
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
func provisionMariaDBAdminUser(t *testing.T, conn *mySQLConn, adminUser string) {
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
func randASCII(t *testing.T) string {
	t.Helper()
	const charLen = 8
	out, err := utils.CryptoRandomHex(charLen / 2)
	require.NoError(t, err)
	return out
}

const (
	// autoUserWaitDur controls how long a test will wait for auto user
	// deactivation or drop.
	// The duration is generous - better to be slow sometimes than flakey.
	autoUserWaitDur  = time.Minute
	autoUserWaitStep = 10 * time.Second
)

func waitForPostgresAutoUserPermissionsRemoved(t *testing.T, ctx context.Context, conn *pgConn, user string) {
	t.Helper()
	waitForSuccess(t, func() error {
		rows, _ := conn.Query(ctx, `
SELECT DISTINCT
	pg_namespace.nspname AS table_schema,
	obj.relname AS table_name,
	acl.privilege_type AS privilege_type
FROM
	pg_class as obj
INNER JOIN
	pg_namespace ON obj.relnamespace = pg_namespace.oid
INNER JOIN LATERAL
	aclexplode(COALESCE(obj.relacl, acldefault('r'::"char", obj.relowner))) AS acl ON true
INNER JOIN
	pg_roles AS grantee ON acl.grantee = grantee.oid
WHERE
	(obj.relkind = ANY (ARRAY['r', 'v', 'f', 'p']))
	AND (acl.privilege_type = ANY (ARRAY['DELETE'::text, 'INSERT'::text, 'REFERENCES'::text, 'SELECT'::text, 'TRUNCATE'::text, 'TRIGGER'::text, 'UPDATE'::text]))
	AND grantee.rolname = $1
`, user)
		var privs []string
		for rows.Next() {
			for _, v := range rows.RawValues() {
				privs = append(privs, string(v))
			}
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return trace.Wrap(err)
		}
		if len(privs) > 0 {
			return trace.Errorf("user %q db permissions %s should have been revoked after disconnecting", user, privs)
		}
		return nil
	}, autoUserWaitDur, autoUserWaitStep, "waiting for auto user %q permissions to be removed", user)
}

func waitForPostgresAutoUserDeactivate(t *testing.T, ctx context.Context, conn *pgConn, user string) {
	t.Helper()
	waitForSuccess(t, func() error {
		// `Query` documents that it is always safe to attempt to read from the
		// returned rows even if an error is returned.
		// It also documents that the same error will be in rows.Err() and
		// rows.Err() will also contain any error from executing the query after
		// closing rows. Hence, we do not check the error until after reading
		// and closing rows.
		rows, _ := conn.Query(ctx, "SELECT 1 FROM pg_roles WHERE rolname=$1", user)
		gotRow := rows.Next()
		rows.Close()
		if err := rows.Err(); err != nil {
			return trace.Wrap(err)
		}
		if !gotRow {
			return trace.Errorf("user %q should not have been dropped after disconnecting", user)
		}

		rows, _ = conn.Query(ctx, "SELECT 1 FROM pg_roles WHERE rolname = $1 AND rolcanlogin = false", user)
		gotRow = rows.Next()
		rows.Close()
		if err := rows.Err(); err != nil {
			return trace.Wrap(err)
		}
		if !gotRow {
			return trace.Errorf("user %q should not be able to login after deactivating", user)
		}

		rows, _ = conn.Query(ctx, "SELECT 1 FROM pg_roles AS a WHERE pg_has_role($1, a.oid, 'member') AND a.rolname NOT IN ($1, 'teleport-auto-user')", user)
		gotRow = rows.Next()
		rows.Close()
		if err := rows.Err(); err != nil {
			return trace.Wrap(err)
		}
		if gotRow {
			return trace.Errorf("user %q should have lost all additional roles after deactivating", user)
		}
		return nil
	}, autoUserWaitDur, autoUserWaitStep, "waiting for auto user %q to be deactivated", user)
}

func waitForPostgresAutoUserDrop(t *testing.T, ctx context.Context, conn *pgConn, user string) {
	t.Helper()
	waitForSuccess(t, func() error {
		// `Query` documents that it is always safe to attempt to read from the
		// returned rows even if an error is returned.
		// It also documents that the same error will be in rows.Err() and
		// rows.Err() will also contain any error from executing the query after
		// closing rows. Hence, we do not check the error until after reading
		// and closing rows.
		rows, _ := conn.Query(ctx, "SELECT 1 FROM pg_roles WHERE rolname=$1", user)
		gotRow := rows.Next()
		rows.Close()
		if err := rows.Err(); err != nil {
			return trace.Wrap(err)
		}
		if gotRow {
			return trace.Errorf("user %q should have been dropped automatically after disconnecting", user)
		}
		return nil
	}, autoUserWaitDur, autoUserWaitStep, "waiting for auto user %q to be dropped", user)
}

func waitForMySQLAutoUserDeactivate(t *testing.T, conn *mySQLConn, user string) {
	t.Helper()
	waitForSuccess(t, func() error {
		result, err := conn.Execute("SELECT 1 FROM mysql.user AS u WHERE u.user = ?", user)
		if err != nil {
			return trace.Wrap(err)
		}
		if result.RowNumber() != 1 {
			result.Close()
			return trace.Errorf("user %q should not have been dropped after disconnecting", user)
		}
		result.Close()

		result, err = conn.Execute("SELECT 1 FROM mysql.user AS u WHERE u.user = ? AND u.account_locked = 'Y'", user)
		if err != nil {
			return trace.Wrap(err)
		}
		if result.RowNumber() != 1 {
			result.Close()
			return trace.Errorf("user %q should not be able to login after deactivating", user)
		}
		result.Close()

		result, err = conn.Execute("SELECT 1 FROM mysql.role_edges AS u WHERE u.to_user = ? AND u.from_user != 'teleport-auto-user'", user)
		if err != nil {
			return trace.Wrap(err)
		}
		if result.RowNumber() != 0 {
			result.Close()
			return trace.Errorf("user %q should have lost all additional roles after deactivating", user)
		}
		result.Close()
		return nil
	}, autoUserWaitDur, autoUserWaitStep, "waiting for auto user %q to be deactivated", user)
}

func waitForMySQLAutoUserDrop(t *testing.T, conn *mySQLConn, user string) {
	t.Helper()
	waitForSuccess(t, func() error {
		result, err := conn.Execute("SELECT 1 FROM mysql.user AS u WHERE u.user = ?", user)
		if err != nil {
			return trace.Wrap(err)
		}
		defer result.Close()
		if result.RowNumber() != 0 {
			return trace.Errorf("user %q should have been dropped automatically after disconnecting", user)
		}
		return nil
	}, autoUserWaitDur, autoUserWaitStep, "waiting for auto user %q to be dropped", user)
}

func waitForMariaDBAutoUserDeactivate(t *testing.T, conn *mySQLConn, user string) {
	t.Helper()
	waitForSuccess(t, func() error {
		result, err := conn.Execute("SELECT 1 FROM mysql.user AS u WHERE u.user = ?", user)
		if err != nil {
			return trace.Wrap(err)
		}
		if result.RowNumber() != 1 {
			result.Close()
			return trace.Errorf("user %q should not have been dropped after disconnecting", user)
		}
		result.Close()

		result, err = conn.Execute("SELECT 1 FROM mysql.global_priv AS u WHERE u.user = ? AND JSON_EXTRACT(u.priv, '$.account_locked') = true", user)
		if err != nil {
			return trace.Wrap(err)
		}
		if result.RowNumber() != 1 {
			result.Close()
			return trace.Errorf("user %q should not be able to login after deactivating", user)
		}
		result.Close()

		result, err = conn.Execute("SELECT 1 FROM mysql.roles_mapping AS u WHERE u.user = ? AND u.role != 'teleport-auto-user' AND u.ADMIN_OPTION='N'", user)
		if err != nil {
			return trace.Wrap(err)
		}
		if result.RowNumber() != 0 {
			result.Close()
			return trace.Errorf("user %q should have lost all additional roles after deactivating", user)
		}
		result.Close()
		return nil
	}, autoUserWaitDur, autoUserWaitStep, "waiting for auto user %q to be deactivated", user)
}

func waitForMariaDBAutoUserDrop(t *testing.T, conn *mySQLConn, user string) {
	t.Helper()
	// run the same tests as mysql to check if the user was dropped.
	waitForMySQLAutoUserDrop(t, conn, user)
}

func pgMustExec(t *testing.T, ctx context.Context, conn *pgConn, statement string) {
	t.Helper()
	_, err := conn.Exec(ctx, statement)
	require.NoError(t, err)
}

func createPGTestTable(t *testing.T, ctx context.Context, conn *pgConn, schemaName, tableName string) {
	t.Helper()
	pgMustExec(t, ctx, conn, fmt.Sprintf("CREATE TABLE %q.%q ()", schemaName, tableName))
	cleanupDB(t, ctx, conn, fmt.Sprintf("DROP TABLE IF EXISTS %q.%q", schemaName, tableName))
}

func createPGTestSchema(t *testing.T, ctx context.Context, conn *pgConn, schemaName string) {
	t.Helper()
	pgMustExec(t, ctx, conn, fmt.Sprintf("CREATE SCHEMA %q", schemaName))
	cleanupDB(t, ctx, conn, fmt.Sprintf("DROP SCHEMA IF EXISTS %q CASCADE", schemaName))
}

func createPGTestUser(t *testing.T, ctx context.Context, conn *pgConn, userName string) {
	t.Helper()
	pgMustExec(t, ctx, conn, fmt.Sprintf("CREATE USER %q", userName))
	cleanupDB(t, ctx, conn, fmt.Sprintf("DROP USER IF EXISTS %q", userName))
}

func createPGTestRole(t *testing.T, ctx context.Context, conn *pgConn, roleName string) {
	t.Helper()
	pgMustExec(t, ctx, conn, fmt.Sprintf("CREATE ROLE %q", roleName))
	cleanupDB(t, ctx, conn, fmt.Sprintf("DROP ROLE IF EXISTS %q", roleName))
}

func cleanupDB(t *testing.T, ctx context.Context, conn *pgConn, statement string) {
	t.Helper()
	t.Cleanup(func() {
		_, err := conn.Exec(ctx, statement)
		assert.NoError(t, err, "failed to cleanup test resource with %s", statement)
	})
}

func cloneDBWithNewAdmin(t *testing.T, db types.Database, admin *types.DatabaseAdminUser) types.Database {
	t.Helper()
	clone := db.Copy()
	clone.SetName("db-" + randASCII(t))
	clone.SetOrigin(types.OriginDynamic)
	clone.Spec.AdminUser = admin
	// sanity check
	dbAdmin := mustGetDBAdmin(t, clone)
	require.Equal(t, clone.Spec.AdminUser.Name, dbAdmin.Name)
	require.Equal(t, clone.Spec.AdminUser.DefaultDatabase, dbAdmin.DefaultDatabase)
	return clone
}
