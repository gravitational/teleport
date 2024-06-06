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

package e2e

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/redshift"
	"github.com/jackc/pgx/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/tlsca"
)

func testRedshiftServerless(t *testing.T) {
	t.Skip("skipped until we fix the spacelift stack")
	t.Parallel()
	accessRole := mustGetEnv(t, rssAccessRoleARNEnv)
	discoveryRole := mustGetEnv(t, rssDiscoveryRoleARNEnv)
	cluster := makeDBTestCluster(t, accessRole, discoveryRole, types.AWSMatcherRedshiftServerless)

	// wait for the database to be discovered
	rssDBName := mustGetEnv(t, rssNameEnv)
	rssEndpointName := mustGetEnv(t, rssEndpointNameEnv)
	waitForDatabases(t, cluster.Process, rssDBName, rssEndpointName)

	t.Run("connect as iam role", func(t *testing.T) {
		// test connections
		rssRoute := tlsca.RouteToDatabase{
			ServiceName: rssDBName,
			Protocol:    defaults.ProtocolPostgres,
			Username:    mustGetEnv(t, rssDBUserEnv),
			Database:    "postgres",
		}
		t.Run("via proxy", func(t *testing.T) {
			t.Parallel()
			postgresConnTest(t, cluster, hostUser, rssRoute, "select 1")
		})
		t.Run("via local proxy", func(t *testing.T) {
			t.Parallel()
			postgresLocalProxyConnTest(t, cluster, hostUser, rssRoute, "select 1")
		})
	})
}

func testRedshiftCluster(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	t.Cleanup(cancel)

	autoUserKeep := "auto_keep_" + randASCII(t, 6)
	autoUserDrop := "auto_drop_" + randASCII(t, 6)
	autoRole1 := "auto_role1_" + randASCII(t, 6)
	autoRole2 := "auto_role2_" + randASCII(t, 6)
	opts := []testOptionsFunc{
		withUserRole(t, autoUserKeep, "db-auto-user-keeper", makeAutoUserKeepRoleSpec(autoRole1, autoRole2)),
		withUserRole(t, autoUserDrop, "db-auto-user-dropper", makeAutoUserDropRoleSpec(autoRole1, autoRole2)),
	}

	accessRole := mustGetEnv(t, redshiftAccessRoleARNEnv)
	discoveryRole := mustGetEnv(t, redshiftDiscoveryRoleARNEnv)
	cluster := makeDBTestCluster(t, accessRole, discoveryRole, types.AWSMatcherRedshift, opts...)

	// wait for the database to be discovered
	redshiftDBName := mustGetEnv(t, redshiftNameEnv)
	waitForDatabases(t, cluster.Process, redshiftDBName)
	db, err := cluster.Process.GetAuthServer().GetDatabase(ctx, redshiftDBName)
	require.NoError(t, err)
	adminUser := mustGetDBAdmin(t, db)

	conn := connectAsRedshiftClusterAdmin(t, ctx, db.GetAWS().Redshift.ClusterID)
	provisionRedshiftAutoUsersAdmin(t, ctx, conn, adminUser.Name)

	// create a new schema with tables that can only be accessed if the
	// auto roles are granted by Teleport automatically.
	testSchema := "test_" + randASCII(t, 4)
	_, err = conn.Exec(ctx, fmt.Sprintf("CREATE SCHEMA %q", testSchema))
	require.NoError(t, err)
	t.Cleanup(func() {
		// users/roles can only be dropped after we drop the schema+table.
		// So, rather than juggling the order of drops, just attempt to drop
		// everything as part of test cleanup, regardless of what the test
		// actually created successfully.
		for _, stmt := range []string{
			fmt.Sprintf("DROP SCHEMA %q CASCADE", testSchema),
			fmt.Sprintf("DROP ROLE %q", autoRole1),
			fmt.Sprintf("DROP ROLE %q", autoRole2),
			fmt.Sprintf("DROP USER IF EXISTS %q", autoUserKeep),
			fmt.Sprintf("DROP USER IF EXISTS %q", autoUserDrop),
		} {
			_, err := conn.Exec(ctx, stmt)
			assert.NoError(t, err, "test cleanup failed, stmt=%q", stmt)
		}
	})
	testTable := "ctf" // capture the flag :)
	_, err = conn.Exec(ctx, fmt.Sprintf("CREATE TABLE %q.%q (id int)", testSchema, testTable))
	require.NoError(t, err)

	// create the roles that Teleport will auto assign.
	// role 1 only allows usage of the test schema.
	// role 2 only allows select of the test table in the test schema.
	// a user needs to have both roles to select from the test table.
	_, err = conn.Exec(ctx, fmt.Sprintf("CREATE ROLE %q", autoRole1))
	require.NoError(t, err)
	_, err = conn.Exec(ctx, fmt.Sprintf("CREATE ROLE %q", autoRole2))
	require.NoError(t, err)
	_, err = conn.Exec(ctx, fmt.Sprintf("GRANT USAGE ON SCHEMA %q TO ROLE %q", testSchema, autoRole1))
	require.NoError(t, err)
	_, err = conn.Exec(ctx, fmt.Sprintf("GRANT SELECT ON %q.%q TO ROLE %q", testSchema, testTable, autoRole2))
	require.NoError(t, err)
	autoRolesQuery := fmt.Sprintf("select 1 from %q.%q", testSchema, testTable)

	var pgxConnMu sync.Mutex
	for name, test := range map[string]struct {
		user            string
		dbUser          string
		query           string
		afterConnTestFn func(t *testing.T)
	}{
		"iam role": {
			user: hostUser,
			// role/<name> is the syntax we require for redshift IAM role auth
			dbUser: "role/" + mustGetEnv(t, redshiftIAMDBUserEnv),
			query:  "select 1",
		},
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
				pgxConnMu.Lock()
				defer pgxConnMu.Unlock()
				waitForRedshiftAutoUserDeactivate(t, ctx, conn, autoUserKeep)
			},
		},
		"auto user drop": {
			user:   autoUserDrop,
			dbUser: autoUserDrop,
			query:  autoRolesQuery,
			afterConnTestFn: func(t *testing.T) {
				pgxConnMu.Lock()
				defer pgxConnMu.Unlock()
				waitForRedshiftAutoUserDrop(t, ctx, conn, autoUserDrop)
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
					Database:    "dev",
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
}

func connectAsRedshiftClusterAdmin(t *testing.T, ctx context.Context, clusterID string) *pgx.Conn {
	t.Helper()
	info := getRedshiftAdminInfo(t, ctx, clusterID)
	const dbName = "dev"
	return connectPostgres(t, ctx, info, dbName)
}

func getRedshiftAdminInfo(t *testing.T, ctx context.Context, clusterID string) dbUserLogin {
	t.Helper()
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(mustGetEnv(t, awsRegionEnv)),
	)
	require.NoError(t, err)
	clt := redshift.NewFromConfig(cfg)
	result, err := clt.DescribeClusters(ctx, &redshift.DescribeClustersInput{
		ClusterIdentifier: &clusterID,
	})
	require.NoError(t, err)
	require.Len(t, result.Clusters, 1)
	dbInstance := result.Clusters[0]
	require.NotNil(t, dbInstance.MasterUsername)
	require.NotNil(t, dbInstance.MasterPasswordSecretArn)
	require.NotEmpty(t, *dbInstance.MasterUsername)
	require.NotEmpty(t, *dbInstance.MasterPasswordSecretArn)
	return dbUserLogin{
		username: *dbInstance.MasterUsername,
		password: getMasterUserPassword(t, ctx, *dbInstance.MasterPasswordSecretArn),
		address:  *dbInstance.Endpoint.Address,
		port:     int(*dbInstance.Endpoint.Port),
	}
}

// provisionRedshiftAutoUsersAdmin provisions an admin user suitable for auto-user
// provisioning.
func provisionRedshiftAutoUsersAdmin(t *testing.T, ctx context.Context, conn *pgx.Conn, adminUser string) {
	t.Helper()
	// Don't cleanup the db admin after, because test runs would interfere
	// with each other.
	_, err := conn.Exec(ctx, fmt.Sprintf("CREATE USER %q WITH PASSWORD DISABLE", adminUser))
	if err != nil {
		require.ErrorContains(t, err, "already exists")
	}
	_, err = conn.Exec(ctx, fmt.Sprintf(`GRANT ROLE "sys:superuser" TO %q`, adminUser))
	if err != nil {
		require.ErrorContains(t, err, "already a member")
	}
}

func waitForRedshiftAutoUserDeactivate(t *testing.T, ctx context.Context, conn *pgx.Conn, user string) {
	t.Helper()
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		// `Query` documents that it is always safe to attempt to read from the
		// returned rows even if an error is returned.
		// It also documents that the same error will be in rows.Err() and
		// rows.Err() will also contain any error from executing the query after
		// closing rows. Hence, we do not check the error until after reading
		// and closing rows.
		rows, _ := conn.Query(ctx, "SELECT 1 FROM pg_user_info as a WHERE a.usename = $1", user)
		gotRow := rows.Next()
		rows.Close()
		if !assert.NoError(c, rows.Err()) {
			return
		}
		if !assert.True(c, gotRow, "user %q should not have been dropped after disconnecting", user) {
			return
		}

		rows, _ = conn.Query(ctx, "SELECT 1 FROM pg_user_info WHERE usename = $1 AND useconnlimit != 0", user)
		gotRow = rows.Next()
		rows.Close()
		if !assert.NoError(c, rows.Err()) {
			return
		}
		if !assert.False(c, gotRow, "user %q should not be able to login after deactivating", user) {
			return
		}

		rows, _ = conn.Query(ctx, "SELECT 1 FROM svv_user_grants as a WHERE a.user_name = $1 AND a.role_name != 'teleport-auto-user'", user)
		gotRow = rows.Next()
		rows.Close()
		if !assert.NoError(c, rows.Err()) {
			return
		}
		assert.False(c, gotRow, "user %q should have lost all additional roles after deactivating", user)
	}, autoUserWaitDur, autoUserWaitStep, "waiting for auto user %q to be deactivated", user)
}

func waitForRedshiftAutoUserDrop(t *testing.T, ctx context.Context, conn *pgx.Conn, user string) {
	t.Helper()
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		// `Query` documents that it is always safe to attempt to read from the
		// returned rows even if an error is returned.
		// It also documents that the same error will be in rows.Err() and
		// rows.Err() will also contain any error from executing the query after
		// closing rows. Hence, we do not check the error until after reading
		// and closing rows.
		rows, _ := conn.Query(ctx, "SELECT 1 FROM pg_user_info WHERE usename=$1", user)
		gotRow := rows.Next()
		rows.Close()
		if !assert.NoError(c, rows.Err()) {
			return
		}
		assert.False(c, gotRow, "user %q should have been dropped automatically after disconnecting", user)
	}, autoUserWaitDur, autoUserWaitStep, "waiting for auto user %q to be dropped", user)
}
