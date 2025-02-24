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

package common

import (
	"bytes"
	"context"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/constants"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/observability/tracing"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/services"
	dbcommon "github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/tool/teleport/testenv"
)

func registerFakeEnterpriseDBEngines(t *testing.T) {
	newFakeEngine := func(dbcommon.EngineConfig) dbcommon.Engine {
		type fakeDBEngine struct {
			dbcommon.Engine
		}
		return fakeDBEngine{}
	}
	dbcommon.RegisterEngine(newFakeEngine, defaults.ProtocolOracle)
	t.Cleanup(func() {
		dbcommon.RegisterEngine(nil, defaults.ProtocolOracle)
	})
}

func TestTshDB(t *testing.T) {
	// Register missing Enterprise database engines. Otherwise db.CheckEngines
	// will fail. The fake engine registered are not functional. But other
	// Enterprise features like Access Request can still be tested.
	registerFakeEnterpriseDBEngines(t)
	modules.SetTestModules(t,
		&modules.TestModules{
			TestBuildType: modules.BuildEnterprise,
			TestFeatures: modules.Features{
				Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
					entitlements.DB: {Enabled: true},
				},
			},
		},
	)

	// this speeds up test suite setup substantially, which is where
	// tests spend the majority of their time, especially when leaf
	// clusters are setup.
	testenv.WithResyncInterval(t, 0)
	// Proxy uses self-signed certificates in tests.
	testenv.WithInsecureDevMode(t, true)
	t.Run("Login", testDatabaseLogin)
	t.Run("List", testListDatabase)
	t.Run("DatabaseSelection", testDatabaseSelection)
}

// testDatabaseLogin tests "tsh db login" command and verifies "tsh db
// env/config" after login.
func testDatabaseLogin(t *testing.T) {
	t.Parallel()

	autoUserRole, err := types.NewRole("autouser", types.RoleSpecV6{
		Allow: types.RoleConditions{
			DatabaseLabels: types.Labels{"auto-user": []string{"true"}},
			DatabaseNames:  []string{"*"},
			DatabaseUsers:  []string{"reader", "writer"},
		},
		Options: types.RoleOptions{
			CreateDatabaseUserMode: types.CreateDatabaseUserMode_DB_USER_MODE_KEEP,
		},
	})
	require.NoError(t, err)

	devAccessRole, err := types.NewRole("dev-access", types.RoleSpecV6{
		Allow: types.RoleConditions{
			DatabaseLabels: types.Labels{"env": []string{"dev"}},
			DatabaseNames:  []string{"default"},
			DatabaseUsers:  []string{"admin"},
		},
	})
	require.NoError(t, err)

	accessRequestorRole, err := types.NewRole("access-requestor", types.RoleSpecV6{
		Allow: types.RoleConditions{
			Request: &types.AccessRequestConditions{
				SearchAsRoles: []string{"access"},
			},
		},
	})
	require.NoError(t, err)

	alice, err := types.NewUser("alice@example.com")
	require.NoError(t, err)
	// to use default --db-user and --db-name selection, make a user with just
	// one of each allowed.
	alice.SetDatabaseUsers([]string{"admin"})
	alice.SetDatabaseNames([]string{"default"})
	alice.SetRoles([]string{"dev-access", "autouser", "access-requestor"})
	s := newTestSuite(t,
		withRootConfigFunc(func(cfg *servicecfg.Config) {
			cfg.Auth.BootstrapResources = append(
				cfg.Auth.BootstrapResources,
				autoUserRole,
				devAccessRole,
				accessRequestorRole,
				alice,
			)
			cfg.Auth.NetworkingConfig.SetProxyListenerMode(types.ProxyListenerMode_Multiplex)
			// separate MySQL port with TLS routing.
			// set the public address to be sure even on v2+, tsh clients will see the separate port.
			mySQLAddr := localListenerAddr()
			cfg.Proxy.MySQLAddr = utils.NetAddr{AddrNetwork: "tcp", Addr: mySQLAddr}
			cfg.Proxy.MySQLPublicAddrs = []utils.NetAddr{{AddrNetwork: "tcp", Addr: mySQLAddr}}
			cfg.Databases.Enabled = true
			cfg.Databases.Databases = []servicecfg.Database{
				{
					Name:     "postgres-rds-us-west-1-123456789012",
					Protocol: defaults.ProtocolPostgres,
					URI:      "localhost:5432",
					StaticLabels: map[string]string{
						types.DiscoveredNameLabel: "postgres",
						"region":                  "us-west-1",
						"env":                     "dev",
					},
					AWS: servicecfg.DatabaseAWS{
						AccountID: "123456789012",
						Region:    "us-west-1",
						RDS: servicecfg.DatabaseAWSRDS{
							InstanceID: "postgres",
						},
					},
				}, {
					Name:         "mysql",
					Protocol:     defaults.ProtocolMySQL,
					URI:          "localhost:3306",
					StaticLabels: map[string]string{"env": "dev"},
				}, {
					Name:         "mysql-prod",
					Protocol:     defaults.ProtocolMySQL,
					URI:          "localhost:3306",
					StaticLabels: map[string]string{"env": "prod"},
				}, {
					Name:     "mysql-autouser",
					Protocol: defaults.ProtocolMySQL,
					URI:      "localhost:3306",
					StaticLabels: map[string]string{
						"auto-user": "true",
					},
					AdminUser: servicecfg.DatabaseAdminUser{
						Name: "teleport-admin",
					},
				}, {
					Name:         "cassandra",
					Protocol:     defaults.ProtocolCassandra,
					URI:          "localhost:9042",
					StaticLabels: map[string]string{"env": "dev"},
				}, {
					Name:         "snowflake",
					Protocol:     defaults.ProtocolSnowflake,
					URI:          "localhost.snowflakecomputing.com",
					StaticLabels: map[string]string{"env": "dev"},
				}, {
					Name:         "mongo",
					Protocol:     defaults.ProtocolMongoDB,
					URI:          "localhost:27017",
					StaticLabels: map[string]string{"env": "dev"},
				}, {
					Name:         "mssql",
					Protocol:     defaults.ProtocolSQLServer,
					URI:          "sqlserver.example.com:1433",
					StaticLabels: map[string]string{"env": "dev"},
					AD: servicecfg.DatabaseAD{
						KeytabFile: "/etc/keytab",
						Domain:     "EXAMPLE.COM",
						SPN:        "MSSQLSvc/sqlserver.example.com:1433",
					},
				}, {
					Name:         "dynamodb",
					Protocol:     defaults.ProtocolDynamoDB,
					URI:          "", // uri can be blank for DynamoDB, it will be derived from the region and requests.
					StaticLabels: map[string]string{"env": "dev"},
					AWS: servicecfg.DatabaseAWS{
						AccountID:  "123456789012",
						ExternalID: "123123123",
						Region:     "us-west-1",
					},
				},
			}
		}),
	)
	s.user = alice

	// Log into Teleport cluster.
	tmpHomePath, _ := mustLoginLegacy(t, s)

	testCases := []struct {
		// the test name
		name string
		// databaseName should be the full database name.
		databaseName string
		// dbSelectors can be any of db name, --labels, --query predicate,
		// and defaults to be databaseName if not set.
		dbSelectors []string
		// extraLoginOptions is a list of extra options used for login like
		// `--db-roles`.
		extraLoginOptions     []string
		setAccessRequestState types.RequestState
		expectActiveRoute     tlsca.RouteToDatabase
		expectCertsLen        int
		expectKeysLen         int
		expectErrForConfigCmd bool
		expectErrForEnvCmd    bool
		expectLoginErrorIs    func(error) bool
	}{
		{
			name:         "mongo",
			databaseName: "mongo",
			expectActiveRoute: tlsca.RouteToDatabase{
				ServiceName: "mongo",
				Protocol:    "mongodb",
				Username:    "admin",
				Database:    "default", // Since database name is required.
			},
			expectCertsLen:     1,
			expectKeysLen:      1,
			expectErrForEnvCmd: true, // "tsh db env" not supported for Mongo.
		},
		{
			name:         "mssql",
			databaseName: "mssql",
			expectActiveRoute: tlsca.RouteToDatabase{
				ServiceName: "mssql",
				Protocol:    "sqlserver",
				Username:    "admin",
				Database:    "default",
			},
			expectCertsLen:        1,
			expectErrForConfigCmd: true, // "tsh db config" not supported for MSSQL.
			expectErrForEnvCmd:    true, // "tsh db env" not supported for MSSQL.
		},
		{
			name:         "mysql",
			databaseName: "mysql",
			expectActiveRoute: tlsca.RouteToDatabase{
				ServiceName: "mysql",
				Protocol:    "mysql",
				Username:    "admin",
			},
			expectCertsLen:        1,
			expectErrForConfigCmd: false, // "tsh db config" is supported for MySQL with TLS routing & separate MySQL port.
			expectErrForEnvCmd:    false, // "tsh db env" not supported for MySQL with TLS routing & separate MySQL port.
		},
		{
			name:         "cassandra",
			databaseName: "cassandra",
			expectActiveRoute: tlsca.RouteToDatabase{
				ServiceName: "cassandra",
				Protocol:    "cassandra",
				Username:    "admin",
			},
			expectCertsLen:        1,
			expectErrForConfigCmd: true, // "tsh db config" not supported for Cassandra.
			expectErrForEnvCmd:    true, // "tsh db env" not supported for Cassandra.
		},
		{
			name:         "snowflake",
			databaseName: "snowflake",
			expectActiveRoute: tlsca.RouteToDatabase{
				ServiceName: "snowflake",
				Protocol:    "snowflake",
				Username:    "admin",
			},
			expectCertsLen:        1,
			expectErrForConfigCmd: true, // "tsh db config" not supported for Snowflake.
			expectErrForEnvCmd:    true, // "tsh db env" not supported for Snowflake.
		},
		{
			name:         "dynamodb",
			databaseName: "dynamodb",
			expectActiveRoute: tlsca.RouteToDatabase{
				ServiceName: "dynamodb",
				Protocol:    "dynamodb",
				Username:    "admin",
			},
			expectCertsLen:        1,
			expectErrForConfigCmd: true, // "tsh db config" not supported for DynamoDB.
			expectErrForEnvCmd:    true, // "tsh db env" not supported for DynamoDB.
		},
		{
			name:         "by full name",
			databaseName: "postgres-rds-us-west-1-123456789012",
			expectActiveRoute: tlsca.RouteToDatabase{
				ServiceName: "postgres-rds-us-west-1-123456789012",
				Protocol:    "postgres",
				Username:    "admin",
				Database:    "default",
			},
			expectCertsLen: 1,
		},
		{
			name:         "by discovered name",
			databaseName: "postgres-rds-us-west-1-123456789012",
			dbSelectors:  []string{"postgres"},
			expectActiveRoute: tlsca.RouteToDatabase{
				ServiceName: "postgres-rds-us-west-1-123456789012",
				Protocol:    "postgres",
				Username:    "admin",
				Database:    "default",
			},
			expectCertsLen: 1,
		},
		{
			name:         "by labels",
			databaseName: "postgres-rds-us-west-1-123456789012",
			dbSelectors:  []string{"--labels", "region=us-west-1"},
			expectActiveRoute: tlsca.RouteToDatabase{
				ServiceName: "postgres-rds-us-west-1-123456789012",
				Protocol:    "postgres",
				Username:    "admin",
				Database:    "default",
			},
			expectCertsLen: 1,
		},
		{
			name:         "by query",
			databaseName: "postgres-rds-us-west-1-123456789012",
			dbSelectors:  []string{"--query", `labels.env=="dev" && labels.region == "us-west-1"`},
			expectActiveRoute: tlsca.RouteToDatabase{
				ServiceName: "postgres-rds-us-west-1-123456789012",
				Protocol:    "postgres",
				Username:    "admin",
				Database:    "default",
			},
			expectCertsLen: 1,
		},
		{
			name:              "select db role",
			databaseName:      "mysql-autouser",
			extraLoginOptions: []string{"--db-roles", "reader"},
			expectActiveRoute: tlsca.RouteToDatabase{
				ServiceName: "mysql-autouser",
				Protocol:    "mysql",
				Username:    "alice@example.com",
				Roles:       []string{"reader"},
			},
			expectCertsLen: 1,
		},
		{
			name:               "database not found",
			databaseName:       "db-not-found",
			expectLoginErrorIs: trace.IsNotFound,
		},
		{
			name:                  "access request approved",
			databaseName:          "mysql-prod",
			setAccessRequestState: types.RequestState_APPROVED,
			expectActiveRoute: tlsca.RouteToDatabase{
				ServiceName: "mysql-prod",
				Protocol:    "mysql",
				Username:    "admin",
			},
			expectCertsLen:        1,
			expectErrForConfigCmd: false,
			expectErrForEnvCmd:    false,
		},
		{
			name:                  "access request denied",
			databaseName:          "mysql-prod",
			setAccessRequestState: types.RequestState_DENIED,
			expectLoginErrorIs:    trace.IsAccessDenied,
		},
	}

	// Note: keystore currently races when multiple tsh clients work in the
	// same profile dir (e.g. StatusCurrent might fail reading if someone else
	// is writing a key at the same time).
	// Thus, in order to speed up this test, we clone the profile dir for each subtest
	// to enable parallel test runs.
	// Copying the profile dir is faster than sequential login for each database.
	for _, test := range testCases {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			tmpHomePath := mustCloneTempDir(t, tmpHomePath)
			selectors := test.dbSelectors
			if len(selectors) == 0 {
				selectors = []string{test.databaseName}
			}

			// override the mysql/postgres config file paths to avoid parallel
			// updates to the default location in the user home dir.
			mySqlCnfPath := filepath.Join(tmpHomePath, ".my.cnf")
			pgCnfPath := filepath.Join(tmpHomePath, ".pg_service.conf")
			// all subsequent tsh commands need these options.
			cliOpts := []CliOption{
				// set .tsh location to the temp dir for this test.
				setHomePath(tmpHomePath),
				setOverrideMySQLConfigPath(mySqlCnfPath),
				setOverridePostgresConfigPath(pgCnfPath),
			}
			args := append([]string{
				// default --db-user and --db-name are selected from roles.
				"db", "login", "--insecure",
			}, selectors...)
			args = append(args, test.extraLoginOptions...)

			// Access request setup.
			if test.setAccessRequestState != types.RequestState_NONE {
				args = append(args, "--request-reason", test.name)
				go updateAccessRequestForDB(t, s, test.name, test.databaseName, test.setAccessRequestState)
			}

			err := Run(context.Background(), args, cliOpts...)
			if test.expectLoginErrorIs != nil {
				require.Error(t, err)
				require.True(t, test.expectLoginErrorIs(err))
				return
			}
			require.NoError(t, err)

			// Fetch the active profile.
			clientStore := client.NewFSClientStore(tmpHomePath)
			profile, err := clientStore.ReadProfileStatus(s.root.Config.Proxy.WebAddr.String())
			require.NoError(t, err)
			require.Equal(t, s.user.GetName(), profile.Username)
			require.Len(t, profile.Databases, 1)
			require.Equal(t, test.expectActiveRoute, profile.Databases[0])

			// Verify certificates.
			// grab the certs using the actual database name to verify certs.
			certs, keys, err := decodePEM(profile.DatabaseCertPathForCluster("", test.databaseName))
			require.NoError(t, err)
			require.Len(t, certs, test.expectCertsLen)
			require.Len(t, keys, test.expectKeysLen)

			t.Run("print info", func(t *testing.T) {
				// organize these as parallel subtests in a group, so we can run
				// them in parallel together before the logout test runs below.
				t.Run("config", func(t *testing.T) {
					t.Parallel()
					args := append([]string{
						"db", "config",
					}, selectors...)
					err := Run(context.Background(), args, cliOpts...)

					if test.expectErrForConfigCmd {
						require.Error(t, err)
						require.NotContains(t, err.Error(), "matches multiple", "should not be ambiguity error")
					} else {
						require.NoError(t, err)
					}
				})
				t.Run("env", func(t *testing.T) {
					t.Parallel()
					args := append([]string{
						"db", "env",
					}, selectors...)
					err := Run(context.Background(), args, cliOpts...)

					if test.expectErrForEnvCmd {
						require.Error(t, err)
						require.NotContains(t, err.Error(), "matches multiple", "should not be ambiguity error")
					} else {
						require.NoError(t, err)
					}
				})
			})

			t.Run("logout", func(t *testing.T) {
				args := append([]string{
					"db", "logout",
				}, selectors...)
				err := Run(context.Background(), args, cliOpts...)
				require.NoError(t, err)
			})
		})
	}
}

func updateAccessRequestForDB(t *testing.T, s *suite, wantRequestReason, wantDBName string, updateState types.RequestState) {
	var accessRequestID string
	require.Eventually(t, func() bool {
		filter := types.AccessRequestFilter{State: types.RequestState_PENDING}
		accessRequests, err := s.root.GetAuthServer().GetAccessRequests(context.Background(), filter)
		if err != nil {
			return false
		}

		for _, accessRequest := range accessRequests {
			if accessRequest.GetRequestReason() != wantRequestReason {
				continue
			}
			for _, resourceID := range accessRequest.GetRequestedResourceIDs() {
				if resourceID.Kind == types.KindDatabase &&
					resourceID.Name == wantDBName {
					accessRequestID = accessRequest.GetName()
					return true
				}
			}
		}
		return false
	}, 10*time.Second, 500*time.Millisecond, "waiting for access request")

	err := s.root.GetAuthServer().SetAccessRequestState(context.Background(), types.AccessRequestUpdate{
		RequestID: accessRequestID,
		State:     updateState,
	})
	require.NoError(t, err)
}

func TestLocalProxyRequirement(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	tmpHomePath := t.TempDir()
	connector := mockConnector(t)
	alice, err := types.NewUser("alice@example.com")
	require.NoError(t, err)
	alice.SetRoles([]string{"access"})

	authProcess, proxyProcess := makeTestServers(t, withBootstrap(connector, alice),
		withAuthConfig(func(cfg *servicecfg.AuthConfig) {
			cfg.NetworkingConfig.SetProxyListenerMode(types.ProxyListenerMode_Multiplex)
		}))

	authServer := authProcess.GetAuthServer()
	require.NotNil(t, authServer)

	proxyAddr, err := proxyProcess.ProxyWebAddr()
	require.NoError(t, err)

	// Log into Teleport cluster.
	err = Run(context.Background(), []string{
		"login", "--insecure", "--debug", "--proxy", proxyAddr.String(),
	}, setHomePath(tmpHomePath), setMockSSOLogin(authServer, alice, connector.GetName()))
	require.NoError(t, err)

	defaultAuthPref, err := authServer.GetAuthPreference(ctx)
	require.NoError(t, err)
	tests := map[string]struct {
		clusterAuthPref  types.AuthPreference
		route            *tlsca.RouteToDatabase
		setupTC          func(*client.TeleportClient)
		tunnelFlag       bool
		wantLocalProxy   bool
		wantTunnel       bool
		wantTunnelReason string
	}{
		"tunnel not required": {
			clusterAuthPref: defaultAuthPref,
			wantLocalProxy:  true,
			wantTunnel:      false,
		},
		"tunnel required for MFA DB session": {
			clusterAuthPref: &types.AuthPreferenceV2{
				Spec: types.AuthPreferenceSpecV2{
					Type:         constants.Local,
					SecondFactor: constants.SecondFactorOptional,
					Webauthn: &types.Webauthn{
						RPID: "127.0.0.1",
					},
					RequireMFAType: types.RequireMFAType_SESSION,
				},
			},
			wantLocalProxy:   true,
			wantTunnel:       true,
			wantTunnelReason: "MFA is required",
		},
		"local proxy not required for separate port": {
			clusterAuthPref: defaultAuthPref,
			setupTC: func(tc *client.TeleportClient) {
				tc.TLSRoutingEnabled = false
				tc.TLSRoutingConnUpgradeRequired = true
				tc.PostgresProxyAddr = "separate.postgres.hostport:8888"
			},
			wantLocalProxy: false,
			wantTunnel:     false,
		},
		"local proxy required if behind lb": {
			clusterAuthPref: defaultAuthPref,
			setupTC: func(tc *client.TeleportClient) {
				tc.TLSRoutingEnabled = true
				tc.TLSRoutingConnUpgradeRequired = true
			},
			wantLocalProxy: true,
			wantTunnel:     false,
		},
		"tunnel required by tunnel flag": {
			clusterAuthPref:  defaultAuthPref,
			tunnelFlag:       true,
			wantLocalProxy:   true,
			wantTunnel:       true,
			wantTunnelReason: dbConnectRequireReasonTunnelFlag,
		},
		"tunnel required for separate port by tunnel flag": {
			clusterAuthPref: defaultAuthPref,
			setupTC: func(tc *client.TeleportClient) {
				tc.TLSRoutingEnabled = false
				tc.TLSRoutingConnUpgradeRequired = false
				tc.PostgresProxyAddr = "separate.postgres.hostport:8888"
			},
			tunnelFlag:       true,
			wantLocalProxy:   true,
			wantTunnel:       true,
			wantTunnelReason: dbConnectRequireReasonTunnelFlag,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			_, err = authServer.UpsertAuthPreference(ctx, tt.clusterAuthPref)
			require.NoError(t, err)
			t.Cleanup(func() {
				_, err = authServer.UpsertAuthPreference(ctx, defaultAuthPref)
				require.NoError(t, err)
			})
			cf := &CLIConf{
				Context:         ctx,
				TracingProvider: tracing.NoopProvider(),
				HomePath:        tmpHomePath,
				tracer:          tracing.NoopTracer(teleport.ComponentTSH),
			}
			tc, err := makeClient(cf)
			require.NoError(t, err)
			if tt.setupTC != nil {
				tt.setupTC(tc)
			}
			route := tlsca.RouteToDatabase{
				ServiceName: "foo-db",
				Protocol:    "postgres",
				Username:    "alice",
				Database:    "postgres",
			}
			requires := getDBConnectLocalProxyRequirement(ctx, tc, route, tt.tunnelFlag)
			require.Equal(t, tt.wantLocalProxy, requires.localProxy)
			require.Equal(t, tt.wantTunnel, requires.tunnel)
			if requires.tunnel {
				require.Len(t, requires.tunnelReasons, 1)
				require.Contains(t, requires.tunnelReasons[0], tt.wantTunnelReason)
			}
		})
	}
}

func testListDatabase(t *testing.T) {
	t.Parallel()
	discoveredName := "root-postgres"
	fullName := "root-postgres-rds-us-west-1-123456789012"
	s := newTestSuite(t,
		withRootConfigFunc(func(cfg *servicecfg.Config) {
			cfg.Auth.StorageConfig.Params["poll_stream_period"] = 50 * time.Millisecond
			cfg.Auth.NetworkingConfig.SetProxyListenerMode(types.ProxyListenerMode_Multiplex)
			cfg.Databases.Enabled = true
			cfg.Databases.Databases = []servicecfg.Database{{
				Name:     fullName,
				Protocol: defaults.ProtocolPostgres,
				URI:      "localhost:5432",
				StaticLabels: map[string]string{
					types.DiscoveredNameLabel: discoveredName,
				},
				AWS: servicecfg.DatabaseAWS{
					AccountID: "123456789012",
					Region:    "us-west-1",
					RDS: servicecfg.DatabaseAWSRDS{
						InstanceID: "root-postgres",
					},
				},
			}}
		}),
		withLeafCluster(),
		withLeafConfigFunc(func(cfg *servicecfg.Config) {
			cfg.Auth.StorageConfig.Params["poll_stream_period"] = 50 * time.Millisecond
			cfg.Databases.Enabled = true
			cfg.Databases.Databases = []servicecfg.Database{{
				Name:     "leaf-postgres",
				Protocol: defaults.ProtocolPostgres,
				URI:      "localhost:5432",
			}}
		}),
	)

	tshHome, _ := mustLoginLegacy(t, s)

	captureStdout := new(bytes.Buffer)
	err := Run(context.Background(), []string{
		"db",
		"ls",
		"--insecure",
		"--debug",
	}, setCopyStdout(captureStdout), setHomePath(tshHome))

	require.NoError(t, err)
	lines := strings.Split(captureStdout.String(), "\n")
	require.Greater(t, len(lines), 2,
		"there should be two lines of header followed by data rows")
	require.True(t,
		strings.HasPrefix(lines[2], discoveredName),
		"non-verbose listing should print the discovered db name")
	require.False(t,
		strings.HasPrefix(lines[2], fullName),
		"non-verbose listing should not print full db name")

	captureStdout.Reset()
	err = Run(context.Background(), []string{
		"db",
		"ls",
		"--verbose",
		"--insecure",
		"--debug",
	}, setCopyStdout(captureStdout), setHomePath(tshHome))
	require.NoError(t, err)
	lines = strings.Split(captureStdout.String(), "\n")
	require.Greater(t, len(lines), 2,
		"there should be two lines of header followed by data rows")
	require.True(t,
		strings.HasPrefix(lines[2], fullName),
		"verbose listing should print full db name")

	captureStdout.Reset()
	err = Run(context.Background(), []string{
		"db",
		"ls",
		"--cluster",
		"leaf1",
		"--insecure",
		"--debug",
	}, setCopyStdout(captureStdout), setHomePath(tshHome))

	require.NoError(t, err)
	require.Contains(t, captureStdout.String(), "leaf-postgres")
}

func TestFormatDatabaseLoginCommand(t *testing.T) {
	t.Parallel()

	t.Run("default", func(t *testing.T) {
		require.Equal(t, "tsh db login", formatDatabaseLoginCommand(""))
	})

	t.Run("with cluster flag", func(t *testing.T) {
		require.Equal(t, "tsh db login --cluster=leaf", formatDatabaseLoginCommand("leaf"))
	})
}

func TestFormatDatabaseListCommand(t *testing.T) {
	t.Parallel()

	t.Run("default", func(t *testing.T) {
		require.Equal(t, "tsh db ls", formatDatabaseListCommand(""))
	})

	t.Run("with cluster flag", func(t *testing.T) {
		require.Equal(t, "tsh db ls --cluster=leaf", formatDatabaseListCommand("leaf"))
	})
}

func TestFormatConfigCommand(t *testing.T) {
	t.Parallel()

	db := tlsca.RouteToDatabase{
		ServiceName: "example-db",
	}

	t.Run("default", func(t *testing.T) {
		require.Equal(t, "tsh db config --format=cmd example-db", formatDatabaseConfigCommand("", db))
	})

	t.Run("with cluster flag", func(t *testing.T) {
		require.Equal(t, "tsh db config --cluster=leaf --format=cmd example-db", formatDatabaseConfigCommand("leaf", db))
	})
}

func TestDBInfoHasChanged(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name               string
		databaseUserName   string
		databaseName       string
		databaseRoles      []string
		db                 tlsca.RouteToDatabase
		wantUserHasChanged bool
	}{
		{
			name:             "empty cli database user flag",
			databaseUserName: "",
			db: tlsca.RouteToDatabase{
				Username: "alice",
				Protocol: defaults.ProtocolMongoDB,
			},
			wantUserHasChanged: false,
		},
		{
			name:             "different user",
			databaseUserName: "alice",
			db: tlsca.RouteToDatabase{
				Username: "bob",
				Protocol: defaults.ProtocolMongoDB,
			},
			wantUserHasChanged: true,
		},
		{
			name:             "different user mysql protocol",
			databaseUserName: "alice",
			db: tlsca.RouteToDatabase{
				Username: "bob",
				Protocol: defaults.ProtocolMySQL,
			},
			wantUserHasChanged: true,
		},
		{
			name:             "same user",
			databaseUserName: "bob",
			db: tlsca.RouteToDatabase{
				Username: "bob",
				Protocol: defaults.ProtocolMongoDB,
			},
			wantUserHasChanged: false,
		},
		{
			name:             "empty cli database user and database name flags",
			databaseUserName: "",
			databaseName:     "",
			db: tlsca.RouteToDatabase{
				Username: "alice",
				Protocol: defaults.ProtocolMongoDB,
			},
			wantUserHasChanged: false,
		},
		{
			name:             "different database name",
			databaseUserName: "",
			databaseName:     "db1",
			db: tlsca.RouteToDatabase{
				Username: "alice",
				Database: "db2",
				Protocol: defaults.ProtocolMongoDB,
			},
			wantUserHasChanged: true,
		},
		{
			name:             "same database name",
			databaseUserName: "",
			databaseName:     "db1",
			db: tlsca.RouteToDatabase{
				Username: "alice",
				Database: "db1",
				Protocol: defaults.ProtocolMongoDB,
			},
			wantUserHasChanged: false,
		},
		{
			name:             "same database roles",
			databaseUserName: "",
			databaseName:     "",
			databaseRoles:    []string{"reader", "writer"},
			db: tlsca.RouteToDatabase{
				Username: "alice",
				Database: "db1",
				Protocol: defaults.ProtocolMongoDB,
				Roles:    []string{"reader", "writer"},
			},
			wantUserHasChanged: false,
		},
		{
			name:             "added database roles",
			databaseUserName: "",
			databaseName:     "",
			databaseRoles:    []string{"reader", "writer"},
			db: tlsca.RouteToDatabase{
				Username: "alice",
				Database: "db1",
				Protocol: defaults.ProtocolMongoDB,
			},
			wantUserHasChanged: true,
		},
		{
			name:             "changed database roles",
			databaseUserName: "",
			databaseName:     "",
			databaseRoles:    []string{"reader", "writer"},
			db: tlsca.RouteToDatabase{
				Username: "alice",
				Database: "db1",
				Protocol: defaults.ProtocolMongoDB,
				Roles:    []string{"reader"},
			},
			wantUserHasChanged: true,
		},
	}

	ca, err := tlsca.FromKeys([]byte(fixtures.TLSCACertPEM), []byte(fixtures.TLSCAKeyPEM))
	require.NoError(t, err)
	privateKey, err := cryptosuites.GenerateKeyWithAlgorithm(cryptosuites.RSA2048)
	require.NoError(t, err)

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			identity := tlsca.Identity{
				Username:        "user",
				RouteToDatabase: tc.db,
				Groups:          []string{"none"},
			}
			subj, err := identity.Subject()
			require.NoError(t, err)
			certBytes, err := ca.GenerateCertificate(tlsca.CertificateRequest{
				PublicKey: privateKey.Public(),
				Subject:   subj,
				NotAfter:  time.Now().Add(time.Hour),
			})
			require.NoError(t, err)

			certPath := filepath.Join(t.TempDir(), "mongo_db_cert.pem")
			require.NoError(t, os.WriteFile(certPath, certBytes, 0o600))

			cliConf := &CLIConf{
				DatabaseUser:  tc.databaseUserName,
				DatabaseName:  tc.databaseName,
				DatabaseRoles: strings.Join(tc.databaseRoles, ","),
			}
			got, err := dbInfoHasChanged(cliConf, certPath)
			require.NoError(t, err)
			require.Equal(t, tc.wantUserHasChanged, got)
		})
	}
}

func waitForDatabases(t *testing.T, auth *service.TeleportProcess, dbs []servicecfg.Database) {
	timeout := 10 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	for {
		select {
		case <-time.After(250 * time.Millisecond):
			all, err := auth.GetAuthServer().GetDatabaseServers(ctx, apidefaults.Namespace)
			require.NoError(t, err)

			// Count how many input "dbs" are registered.
			var registered int
			for _, db := range dbs {
				for _, a := range all {
					if a.GetName() == db.Name {
						registered++
						break
					}
				}
			}

			if registered == len(dbs) {
				return
			}
		case <-ctx.Done():
			t.Fatalf("databases not registered after %v", timeout)
		}
	}
}

// decodePEM sorts out specified PEM file into certificates and private keys.
func decodePEM(pemPath string) (certs []pem.Block, privs []pem.Block, err error) {
	bytes, err := os.ReadFile(pemPath)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	var block *pem.Block
	for {
		block, bytes = pem.Decode(bytes)
		if block == nil {
			break
		}
		switch block.Type {
		case "CERTIFICATE":
			certs = append(certs, *block)
		case keys.PKCS1PrivateKeyType:
			privs = append(privs, *block)
		case keys.PKCS8PrivateKeyType:
			privs = append(privs, *block)
		}
	}
	return certs, privs, nil
}

func TestFormatDatabaseConnectArgs(t *testing.T) {
	tests := []struct {
		name      string
		cluster   string
		route     tlsca.RouteToDatabase
		wantFlags []string
	}{
		{
			name:      "match user and db name, cluster set",
			cluster:   "foo",
			route:     tlsca.RouteToDatabase{Protocol: defaults.ProtocolMongoDB, ServiceName: "svc"},
			wantFlags: []string{"--cluster=foo", "--db-user=<user>", "--db-name=<name>", "svc"},
		},
		{
			name:      "match user and db name",
			cluster:   "",
			route:     tlsca.RouteToDatabase{Protocol: defaults.ProtocolMongoDB, ServiceName: "svc"},
			wantFlags: []string{"--db-user=<user>", "--db-name=<name>", "svc"},
		},
		{
			name:      "match user and db name, username given",
			cluster:   "",
			route:     tlsca.RouteToDatabase{Protocol: defaults.ProtocolMongoDB, Username: "bob", ServiceName: "svc"},
			wantFlags: []string{"--db-name=<name>", "svc"},
		},
		{
			name:      "match user and db name, db name given",
			cluster:   "",
			route:     tlsca.RouteToDatabase{Protocol: defaults.ProtocolMongoDB, Database: "sales", ServiceName: "svc"},
			wantFlags: []string{"--db-user=<user>", "svc"},
		},
		{
			name:      "match user and db name, both given",
			cluster:   "",
			route:     tlsca.RouteToDatabase{Protocol: defaults.ProtocolMongoDB, Database: "sales", Username: "bob", ServiceName: "svc"},
			wantFlags: []string{"svc"},
		},
		{
			name:      "match user name",
			cluster:   "",
			route:     tlsca.RouteToDatabase{Protocol: defaults.ProtocolMySQL, ServiceName: "svc"},
			wantFlags: []string{"--db-user=<user>", "svc"},
		},
		{
			name:      "match user name, given",
			cluster:   "",
			route:     tlsca.RouteToDatabase{Protocol: defaults.ProtocolMySQL, Username: "bob", ServiceName: "svc"},
			wantFlags: []string{"svc"},
		},
		{
			name:      "match user name, dynamodb",
			cluster:   "",
			route:     tlsca.RouteToDatabase{Protocol: defaults.ProtocolDynamoDB, ServiceName: "svc"},
			wantFlags: []string{"--db-user=<user>", "svc"},
		},
		{
			name:      "match user and db name, oracle protocol",
			cluster:   "",
			route:     tlsca.RouteToDatabase{Protocol: defaults.ProtocolOracle, ServiceName: "svc"},
			wantFlags: []string{"--db-user=<user>", "--db-name=<name>", "svc"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := formatDatabaseConnectArgs(tt.cluster, tt.route)
			require.Equal(t, tt.wantFlags, out)
		})
	}
}

// TestGetDefaultDBNameAndUser tests getting a default --db-name and --db-user
// from a user's roles.
func TestGetDefaultDBNameAndUser(t *testing.T) {
	t.Parallel()
	genericDB, err := types.NewDatabaseV3(types.Metadata{
		Name:   "test-db",
		Labels: map[string]string{"foo": "bar"},
	}, types.DatabaseSpecV3{
		Protocol: "protocol",
		URI:      "uri",
	})
	require.NoError(t, err)
	dbRedis, err := types.NewDatabaseV3(types.Metadata{
		Name: "aws-elasticache",
	}, types.DatabaseSpecV3{
		Protocol: "redis",
		URI:      "clustercfg.my-redis-cluster.xxxxxx.cac1.cache.amazonaws.com:6379",
	})
	require.NoError(t, err)

	tests := map[string]struct {
		db               types.Database
		allowedDBUsers   []string
		allowedDBNames   []string
		expectDBUser     string
		expectDBName     string
		expectErr        string
		expectDBUserHint string
		expectDBNameHint string
	}{
		"one allowed": {
			db:             genericDB,
			allowedDBUsers: []string{"alice"},
			allowedDBNames: []string{"dev"},
			expectDBUser:   "alice",
			expectDBName:   "dev",
		},
		"wildcard allowed but one explicit": {
			db:             genericDB,
			allowedDBUsers: []string{"*", "alice"},
			allowedDBNames: []string{"*", "dev"},
			expectDBUser:   "alice",
			expectDBName:   "dev",
		},
		"select default user from wildcard for Redis": {
			db:             dbRedis,
			allowedDBUsers: []string{"*"},
			allowedDBNames: []string{"*", "dev"},
			expectDBUser:   "default",
			expectDBName:   "dev",
		},
		"none allowed": {
			db:        genericDB,
			expectErr: "not allowed access to any",
		},
		"denied matches allowed": {
			db:             genericDB,
			allowedDBUsers: []string{"rootDBUser"},
			allowedDBNames: []string{"rootDBName"},
			expectErr:      "not allowed access to any",
		},
		"only wildcard allowed due to deny rules": {
			db:               genericDB,
			allowedDBUsers:   []string{"*", "rootDBUser"},
			allowedDBNames:   []string{"*", "rootDBName"},
			expectErr:        "please provide",
			expectDBUserHint: "[*] except [rootDBUser]",
			expectDBNameHint: "[*] except [rootDBName]",
		},
		"has multiple db users": {
			db:               genericDB,
			allowedDBUsers:   []string{"alice", "bob"},
			allowedDBNames:   []string{"dev", "prod"},
			expectErr:        "please provide",
			expectDBUserHint: "[alice bob]",
			expectDBNameHint: "[dev prod]",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			role := &types.RoleV6{
				Metadata: types.Metadata{Name: "test-role", Namespace: apidefaults.Namespace},
				Spec: types.RoleSpecV6{
					Allow: types.RoleConditions{
						Namespaces:     []string{apidefaults.Namespace},
						DatabaseLabels: types.Labels{"*": []string{"*"}},
						DatabaseUsers:  test.allowedDBUsers,
						DatabaseNames:  test.allowedDBNames,
					},
					Deny: types.RoleConditions{
						Namespaces:    []string{apidefaults.Namespace},
						DatabaseUsers: []string{"rootDBUser"},
						DatabaseNames: []string{"rootDBName"},
					},
				},
			}
			accessChecker := services.NewAccessCheckerWithRoleSet(&services.AccessInfo{}, "clustername", services.NewRoleSet(role))
			dbUser, err := getDefaultDBUser(test.db, accessChecker)
			if test.expectErr != "" {
				require.ErrorContains(t, err, test.expectErr)
				if test.expectDBUserHint != "" {
					require.ErrorContains(t, err, fmt.Sprintf("allowed database users for test-db: %v", test.expectDBUserHint))
				}
				return
			}
			require.NoError(t, err)
			require.Equal(t, test.expectDBUser, dbUser)
			dbName, err := getDefaultDBName(test.db, accessChecker)
			if test.expectErr != "" {
				require.ErrorContains(t, err, test.expectErr)
				if test.expectDBNameHint != "" {
					require.ErrorContains(t, err, fmt.Sprintf("allowed database names for test-db: %v", test.expectDBNameHint))
				}
				return
			}
			require.NoError(t, err)
			require.Equal(t, test.expectDBName, dbName)
		})
	}
}

func TestResourceSelectors(t *testing.T) {
	t.Parallel()
	t.Run("formatting", testResourceSelectorsFormatting)
	t.Run("IsEmpty", testResourceSelectorsIsEmpty)
}

func testResourceSelectorsIsEmpty(t *testing.T) {
	t.Parallel()
	tests := []struct {
		desc      string
		selectors resourceSelectors
		wantEmpty bool
	}{
		{
			desc:      "no fields set",
			selectors: resourceSelectors{},
			wantEmpty: true,
		},
		{
			desc:      "kind field set",
			selectors: resourceSelectors{kind: "x"},
			wantEmpty: true,
		},
		{
			desc:      "name field set",
			selectors: resourceSelectors{name: "x"},
		},
		{
			desc:      "labels field set",
			selectors: resourceSelectors{labels: "x"},
		},
		{
			desc:      "query field set",
			selectors: resourceSelectors{query: "x"},
		},
	}
	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			require.Equal(t, test.wantEmpty, test.selectors.IsEmpty())
		})
	}
}

func testResourceSelectorsFormatting(t *testing.T) {
	t.Parallel()
	tests := []struct {
		testName  string
		selectors resourceSelectors
		want      string
	}{
		{
			testName: "no selectors",
			selectors: resourceSelectors{
				kind: "database",
			},
			want: "database",
		},
		{
			testName: "by name",
			selectors: resourceSelectors{
				kind: "database",
				name: "foo",
			},
			want: `database "foo"`,
		},
		{
			testName: "by labels",
			selectors: resourceSelectors{
				kind:   "database",
				labels: "env=dev,region=us-west-1",
			},
			want: `database with labels "env=dev,region=us-west-1"`,
		},
		{
			testName: "by predicate",
			selectors: resourceSelectors{
				kind:  "database",
				query: `labels["env"]=="dev" && labels.region == "us-west-1"`,
			},
			want: `database with query (labels["env"]=="dev" && labels.region == "us-west-1")`,
		},
		{
			testName: "by name and labels and predicate",
			selectors: resourceSelectors{
				kind:   "app",
				name:   "foo",
				labels: "env=dev,region=us-west-1",
				query:  `labels["env"]=="dev" && labels.region == "us-west-1"`,
			},
			want: `app "foo" with labels "env=dev,region=us-west-1" with query (labels["env"]=="dev" && labels.region == "us-west-1")`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.testName, func(t *testing.T) {
			require.Equal(t, tt.want, fmt.Sprintf("%v", tt.selectors))
		})
	}
}

// makeDBConfigAndRoute is a helper func that makes a db config and
// corresponding cert encoded route to that db - protocol etc not important.
func makeDBConfigAndRoute(name string, staticLabels map[string]string) (servicecfg.Database, tlsca.RouteToDatabase) {
	db := servicecfg.Database{
		Name:         name,
		Protocol:     defaults.ProtocolPostgres,
		URI:          "localhost:5432",
		StaticLabels: staticLabels,
	}
	route := tlsca.RouteToDatabase{
		ServiceName: name,
		Protocol:    defaults.ProtocolPostgres,
		Username:    "alice",
		Database:    "postgres",
	}
	return db, route
}

func TestChooseOneDatabase(t *testing.T) {
	t.Parallel()
	db0, err := types.NewDatabaseV3(types.Metadata{
		Name:   "my-db",
		Labels: map[string]string{"foo": "bar"},
	}, types.DatabaseSpecV3{
		Protocol: "protocol",
		URI:      "uri",
	})
	require.NoError(t, err)
	db1, err := types.NewDatabaseV3(types.Metadata{
		Name:   "my-db-1",
		Labels: map[string]string{"foo": "bar"},
	}, types.DatabaseSpecV3{
		Protocol: "protocol",
		URI:      "uri",
	})
	require.NoError(t, err)
	db2, err := types.NewDatabaseV3(types.Metadata{
		Name:   "my-db-2",
		Labels: map[string]string{"foo": "bar"},
	}, types.DatabaseSpecV3{
		Protocol: "protocol",
		URI:      "uri",
	})
	require.NoError(t, err)
	db3, err := types.NewDatabaseV3(types.Metadata{
		Name:   "my-db-with-some-suffix",
		Labels: map[string]string{"foo": "bar", types.DiscoveredNameLabel: "my-db"},
	}, types.DatabaseSpecV3{
		Protocol: "protocol",
		URI:      "uri",
	})
	require.NoError(t, err)
	db4, err := types.NewDatabaseV3(types.Metadata{
		Name:   "my-db-with-some-other-suffix",
		Labels: map[string]string{"foo": "bar", types.DiscoveredNameLabel: "my-db"},
	}, types.DatabaseSpecV3{
		Protocol: "protocol",
		URI:      "uri",
	})
	require.NoError(t, err)
	tests := []struct {
		desc            string
		databases       types.Databases
		wantDB          types.Database
		wantErrContains string
	}{
		{
			desc:      "only one database to choose from",
			databases: types.Databases{db1},
			wantDB:    db1,
		},
		{
			desc:      "multiple databases to choose from with unambiguous name match",
			databases: types.Databases{db0, db1, db2},
			wantDB:    db0,
		},
		{
			desc:      "multiple databases to choose from with unambiguous discovered name match",
			databases: types.Databases{db1, db2, db3},
			wantDB:    db3,
		},
		{
			desc:            "zero databases to choose from is an error",
			wantErrContains: `database "my-db" with labels "foo=bar" with query (hasPrefix(name, "my-db")) not found, use 'tsh db ls --cluster=local-site'`,
		},
		{
			desc:            "ambiguous databases to choose from is an error",
			databases:       types.Databases{db1, db2},
			wantErrContains: `database "my-db" with labels "foo=bar" with query (hasPrefix(name, "my-db")) matches multiple databases`,
		},
		{
			desc:            "ambiguous discovered name databases is an error",
			databases:       types.Databases{db3, db4},
			wantErrContains: `database "my-db" with labels "foo=bar" with query (hasPrefix(name, "my-db")) matches multiple databases`,
		},
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			cf := &CLIConf{
				Context:             ctx,
				TracingProvider:     tracing.NoopProvider(),
				tracer:              tracing.NoopTracer(teleport.ComponentTSH),
				DatabaseService:     "my-db",
				Labels:              "foo=bar",
				PredicateExpression: `hasPrefix(name, "my-db")`,
				SiteName:            "local-site",
			}
			db, err := chooseOneDatabase(cf, test.databases)
			if test.wantErrContains != "" {
				require.ErrorContains(t, err, test.wantErrContains)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, db, "should have chosen a database")
			require.Empty(t, cmp.Diff(test.wantDB, db))
		})
	}
}

func TestMaybePickActiveDatabase(t *testing.T) {
	t.Parallel()
	x := tlsca.RouteToDatabase{ServiceName: "x"}
	y := tlsca.RouteToDatabase{ServiceName: "y"}
	z := tlsca.RouteToDatabase{ServiceName: "z"}
	tests := []struct {
		desc                   string
		svcName, labels, query string
		routes                 []tlsca.RouteToDatabase
		wantRoute              *tlsca.RouteToDatabase
		wantErr                string
	}{
		{
			desc:    "does nothing if labels given",
			routes:  []tlsca.RouteToDatabase{x},
			svcName: "x",
			labels:  "env=dev",
		},
		{
			desc:    "does nothing if query given",
			svcName: "x",
			routes:  []tlsca.RouteToDatabase{x},
			query:   `name == "x"`,
		},
		{
			desc:      "picks an active route by name",
			svcName:   "y",
			routes:    []tlsca.RouteToDatabase{x, y, z},
			wantRoute: &y,
		},
		{
			desc:    "does nothing if only unmatched name is given",
			svcName: "y",
			routes:  []tlsca.RouteToDatabase{x, z},
		},
		{
			desc:      "picks the only active route without selectors",
			routes:    []tlsca.RouteToDatabase{x},
			wantRoute: &x,
		},
		{
			desc:    "no routes and no selectors is an error",
			routes:  []tlsca.RouteToDatabase{},
			wantErr: "please login",
		},
		{
			desc:    "many routes and no selectors is an error",
			routes:  []tlsca.RouteToDatabase{x, y, z},
			wantErr: "multiple databases",
		},
	}
	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			cf := &CLIConf{
				DatabaseService:     test.svcName,
				Labels:              test.labels,
				PredicateExpression: test.query,
			}
			route, err := maybePickActiveDatabase(cf, test.routes)
			if test.wantErr != "" {
				require.ErrorContains(t, err, test.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, test.wantRoute, route)
		})
	}
}

func TestFindActiveDatabase(t *testing.T) {
	t.Parallel()
	x := tlsca.RouteToDatabase{ServiceName: "x", Protocol: "postgres", Username: "alice", Database: "postgres"}
	y := tlsca.RouteToDatabase{ServiceName: "y", Protocol: "postgres", Username: "alice", Database: "postgres"}
	z := tlsca.RouteToDatabase{ServiceName: "z", Protocol: "postgres", Username: "alice", Database: "postgres"}
	tests := []struct {
		desc      string
		name      string
		routes    []tlsca.RouteToDatabase
		wantOK    bool
		wantRoute tlsca.RouteToDatabase
	}{
		{
			desc: "zero routes",
			name: "x",
		},
		{
			desc: "no name with zero routes",
		},
		{
			desc:   "no name with one route",
			routes: []tlsca.RouteToDatabase{x},
		},
		{
			desc:   "no name with many routes",
			routes: []tlsca.RouteToDatabase{x, y},
		},
		{
			desc:      "name in routes",
			name:      "x",
			routes:    []tlsca.RouteToDatabase{x, y},
			wantOK:    true,
			wantRoute: x,
		},
		{
			desc:   "name not in routes",
			name:   "x",
			routes: []tlsca.RouteToDatabase{y, z},
		},
	}
	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			route, ok := findActiveDatabase(test.name, test.routes)
			require.Equal(t, test.wantOK, ok)
			require.Equal(t, test.wantRoute, route)
		})
	}
}

// testDatabaseSelection tests database selection by name, prefix name, labels,
// query, etc.
func testDatabaseSelection(t *testing.T) {
	t.Parallel()
	// setup some databases and "active" routes to test filtering

	// databases that all have a name starting with with "foo"
	fooDB1, fooRoute1 := makeDBConfigAndRoute("foo", map[string]string{"env": "dev", "svc": "fooer"})
	fooRDSDB, fooRDSRoute := makeDBConfigAndRoute("foo-rds-us-west-1-123456789012", map[string]string{"env": "prod", "region": "us-west-1", types.DiscoveredNameLabel: "foo-rds"})
	fooRDSCustomDB, fooRDSCustomRoute := makeDBConfigAndRoute("foo-rds-custom-us-west-1-123456789012", map[string]string{"env": "prod", "region": "us-west-1", types.DiscoveredNameLabel: "foo-rds-custom"})
	// a route that isn't registered anymore, like when a user has logged into
	// a db that isn't registered in the cluster anymore.
	_, staleRoute := makeDBConfigAndRoute("stale", map[string]string{"env": "dev", "svc": "fooer"})

	// databases that all have a name starting with with "bar"
	barRDSDB1, barRDSRoute1 := makeDBConfigAndRoute("bar-rds-us-west-1-123456789012", map[string]string{"env": "prod", "region": "us-west-1", types.DiscoveredNameLabel: "bar-rds"})
	barRDSDB2, barRDSRoute2 := makeDBConfigAndRoute("bar-rds-us-west-2-123456789012", map[string]string{"env": "prod", "region": "us-west-2", types.DiscoveredNameLabel: "bar-rds"})

	activeRoutes := []tlsca.RouteToDatabase{
		fooRoute1, fooRDSRoute, fooRDSCustomRoute, staleRoute,
		barRDSRoute1, barRDSRoute2,
	}

	alice, err := types.NewUser("alice@example.com")
	require.NoError(t, err)
	alice.SetDatabaseUsers([]string{"alice", "bob"})
	alice.SetDatabaseNames([]string{"postgres", "other"})
	alice.SetRoles([]string{"access"})
	s := newTestSuite(t,
		withRootConfigFunc(func(cfg *servicecfg.Config) {
			cfg.Auth.BootstrapResources = append(cfg.Auth.BootstrapResources, alice)
			cfg.Auth.NetworkingConfig.SetProxyListenerMode(types.ProxyListenerMode_Multiplex)
			cfg.Databases.Enabled = true
			cfg.Databases.Databases = []servicecfg.Database{
				fooDB1, fooRDSDB, fooRDSCustomDB,
				barRDSDB1, barRDSDB2,
			}
		}),
	)
	s.user = alice

	// Log into Teleport cluster.
	tmpHomePath, _ := mustLoginLegacy(t, s)

	t.Run("GetDatabasesForLogout", func(t *testing.T) {
		t.Parallel()
		tests := []struct {
			name,
			svcName,
			labels,
			query string
			wantRoutes []tlsca.RouteToDatabase
			wantErr    string
		}{
			{
				name:       "by exact name",
				svcName:    fooRDSRoute.ServiceName,
				wantRoutes: []tlsca.RouteToDatabase{fooRDSRoute},
			},
			{
				name:       "by exact discovered name",
				svcName:    "foo-rds",
				wantRoutes: []tlsca.RouteToDatabase{fooRDSRoute},
			},
			{
				name:       "by labels",
				labels:     "region=us-west-2",
				wantRoutes: []tlsca.RouteToDatabase{barRDSRoute2},
			},
			{
				name:       "by query",
				query:      `labels.region == "us-west-2"`,
				wantRoutes: []tlsca.RouteToDatabase{barRDSRoute2},
			},
			{
				name:       "by exact name of unregistered database",
				svcName:    staleRoute.ServiceName,
				wantRoutes: []tlsca.RouteToDatabase{staleRoute},
			},
			{
				name:    "by exact discovered name that is ambiguous",
				svcName: "bar-rds",
				wantErr: "matches multiple",
			},
			{
				name:       "by exact discovered name with labels",
				svcName:    "bar-rds",
				labels:     "region=us-west-1",
				wantRoutes: []tlsca.RouteToDatabase{barRDSRoute1},
			},
			{
				name:       "by exact discovered name with query",
				svcName:    "bar-rds",
				query:      `labels.region == "us-west-1"`,
				wantRoutes: []tlsca.RouteToDatabase{barRDSRoute1},
			},
			{
				name:       "all",
				wantRoutes: activeRoutes,
			},
		}
		ctx, cancel := context.WithCancel(context.Background())
		t.Cleanup(cancel)
		for _, tt := range tests {
			tt := tt
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				cf := &CLIConf{
					Context:             ctx,
					HomePath:            tmpHomePath,
					DatabaseService:     tt.svcName,
					Labels:              tt.labels,
					PredicateExpression: tt.query,
				}
				tc, err := makeClient(cf)
				require.NoError(t, err)
				gotRoutes, err := getDatabasesForLogout(cf, tc, activeRoutes)
				if tt.wantErr != "" {
					require.ErrorContains(t, err, tt.wantErr)
					return
				}
				require.NoError(t, err)
				require.Empty(t, cmp.Diff(tt.wantRoutes, gotRoutes))
			})
		}
	})

	t.Run("GetDatabaseInfo", func(t *testing.T) {
		t.Parallel()
		tests := []struct {
			desc                   string
			svcName, labels, query string
			dbUser, dbName         string
			activeRoutes           []tlsca.RouteToDatabase
			wantRoute              tlsca.RouteToDatabase
			wantActive             bool
			wantErr                string
		}{
			{
				desc:      "by exact name",
				svcName:   "foo",
				dbUser:    "alice",
				dbName:    "postgres",
				wantRoute: fooRoute1,
			},
			{
				desc:         "by exact name of active db",
				svcName:      "foo",
				activeRoutes: []tlsca.RouteToDatabase{fooRoute1},
				wantRoute:    fooRoute1,
				wantActive:   true,
			},
			{
				desc:         "by exact name of active db overriding user and schema",
				svcName:      "foo",
				dbUser:       "bob",
				dbName:       "other",
				activeRoutes: []tlsca.RouteToDatabase{fooRoute1},
				wantRoute:    tlsca.RouteToDatabase{ServiceName: "foo", Protocol: "postgres", Username: "bob", Database: "other"},
				wantActive:   true,
			},
			{
				desc:         "by exact name that is a prefix of an active db",
				svcName:      "foo",
				dbUser:       "alice",
				dbName:       "postgres",
				activeRoutes: []tlsca.RouteToDatabase{fooRDSRoute},
				wantRoute:    fooRoute1,
			},
			{
				desc:      "by exact discovered name",
				svcName:   "foo-rds",
				dbUser:    "alice",
				dbName:    "postgres",
				wantRoute: fooRDSRoute,
			},
			{
				desc:      "by labels",
				labels:    "env=dev,svc=fooer",
				dbUser:    "alice",
				dbName:    "postgres",
				wantRoute: fooRoute1,
			},
			{
				desc:         "by labels and active route",
				labels:       "env=dev,svc=fooer",
				activeRoutes: []tlsca.RouteToDatabase{fooRoute1},
				wantRoute:    fooRoute1,
				wantActive:   true,
			},
			{
				desc:      "by query",
				query:     `name=="foo" && labels.env=="dev" && labels.svc=="fooer"`,
				dbUser:    "alice",
				dbName:    "postgres",
				wantRoute: fooRoute1,
			},
			{
				desc:         "by query and active route",
				query:        `name == "foo" && labels.env=="dev" && labels.svc=="fooer"`,
				activeRoutes: []tlsca.RouteToDatabase{fooRoute1},
				wantRoute:    fooRoute1,
				wantActive:   true,
			},
			{
				desc:    "by ambiguous exact discovered name",
				svcName: "bar-rds",
				wantErr: "matches multiple databases",
			},
			{
				desc:      "resolves ambiguous exact discovered name by label",
				svcName:   "bar-rds",
				labels:    "region=us-west-1",
				dbUser:    "alice",
				dbName:    "postgres",
				wantRoute: barRDSRoute1,
			},
			{
				desc:      "resolves ambiguous exact discovered name by query",
				svcName:   "bar-rds",
				query:     `labels.region=="us-west-2"`,
				dbUser:    "alice",
				dbName:    "postgres",
				wantRoute: barRDSRoute2,
			},
			{
				desc:    "by name of db that does not exist",
				svcName: "foo-rds-",
				wantErr: `"foo-rds-" not found, use 'tsh db ls' to see registered databases`,
			},
			{
				desc:         "by name of db that does not exist and is not active",
				svcName:      "foo-rds-",
				activeRoutes: []tlsca.RouteToDatabase{fooRDSRoute},
				wantErr:      `"foo-rds-" not found, use 'tsh db ls' to see registered databases`,
			},
			{
				desc:    "by ambiguous labels",
				labels:  "region=us-west-1",
				wantErr: "matches multiple databases",
			},
			{
				desc:    "by ambiguous query",
				query:   `labels.region == "us-west-1"`,
				wantErr: "matches multiple databases",
			},
			{
				desc:         "by exact name of unregistered database",
				svcName:      staleRoute.ServiceName,
				activeRoutes: []tlsca.RouteToDatabase{staleRoute},
				wantErr:      `you are logged into a database that no longer exists in the cluster`,
			},
			// cases without selectors should try choose to from active databases
			{
				desc:         "no selectors with one active registered db",
				activeRoutes: []tlsca.RouteToDatabase{fooRDSRoute},
				wantRoute:    fooRDSRoute,
				wantActive:   true,
			},
			{
				desc:         "no selectors with zero active registered db",
				activeRoutes: []tlsca.RouteToDatabase{staleRoute},
				wantErr:      `you are logged into a database that no longer exists in the cluster`,
			},
			{
				desc:         "no selectors with multiple active registered db",
				activeRoutes: []tlsca.RouteToDatabase{fooRoute1, fooRDSRoute},
				wantErr:      "multiple databases are available",
			},
		}
		ctx, cancel := context.WithCancel(context.Background())
		t.Cleanup(cancel)
		for _, test := range tests {
			test := test
			t.Run(test.desc, func(t *testing.T) {
				t.Parallel()
				cf := &CLIConf{
					Context:             ctx,
					HomePath:            tmpHomePath,
					DatabaseService:     test.svcName,
					Labels:              test.labels,
					PredicateExpression: test.query,
					DatabaseUser:        test.dbUser,
					DatabaseName:        test.dbName,
				}
				tc, err := makeClient(cf)
				require.NoError(t, err)
				info, err := getDatabaseInfo(cf, tc, test.activeRoutes)
				if test.wantErr != "" {
					require.ErrorContains(t, err, test.wantErr)
					return
				}
				require.NoError(t, err)
				require.Equal(t, test.wantRoute, info.RouteToDatabase)
				db, err := info.GetDatabase(cf.Context, tc)
				require.NoError(t, err)
				require.Equal(t, info.ServiceName, db.GetName())
				require.Equal(t, info.Protocol, db.GetProtocol())
				require.Equal(t, db, info.database, "database should have been fetched and cached")
				require.Equal(t, test.wantActive, info.isActive)
			})
		}
	})

	t.Run("PickActiveDatabase", func(t *testing.T) {
		t.Parallel()
		tests := []struct {
			desc         string
			activeRoutes []tlsca.RouteToDatabase
			dbName       string
			wantRoute    tlsca.RouteToDatabase
			wantErr      string
		}{
			{
				desc:         "pick active db without selector",
				activeRoutes: []tlsca.RouteToDatabase{barRDSRoute1},
				wantRoute:    barRDSRoute1,
			},
			{
				desc:         "pick active db with discovered name selector",
				activeRoutes: []tlsca.RouteToDatabase{fooRDSRoute, barRDSRoute1},
				dbName:       "foo-rds",
				wantRoute:    fooRDSRoute,
			},
			{
				desc:         "pick active db with exact name selector",
				activeRoutes: []tlsca.RouteToDatabase{fooRDSRoute, barRDSRoute1},
				dbName:       fooRDSRoute.ServiceName,
				wantRoute:    fooRDSRoute,
			},
			{
				desc:         "pick inactive db with selector",
				dbName:       "foo-rds",
				activeRoutes: []tlsca.RouteToDatabase{barRDSRoute1},
				wantErr:      `not logged into database "foo-rds"`,
			},
			{
				desc:         "no active db",
				activeRoutes: []tlsca.RouteToDatabase{},
				wantErr:      "please login using 'tsh db login' first",
			},
			{
				desc:         "multiple active db without selector",
				activeRoutes: []tlsca.RouteToDatabase{fooRDSRoute, barRDSRoute1},
				wantErr:      "multiple databases are available",
			},
		}
		ctx, cancel := context.WithCancel(context.Background())
		t.Cleanup(cancel)
		for _, test := range tests {
			test := test
			t.Run(test.desc, func(t *testing.T) {
				t.Parallel()
				cf := &CLIConf{
					Context:         ctx,
					HomePath:        tmpHomePath,
					DatabaseService: test.dbName,
				}
				tc, err := makeClient(cf)
				require.NoError(t, err)
				route, err := pickActiveDatabase(cf, tc, test.activeRoutes)
				if test.wantErr != "" {
					require.ErrorContains(t, err, test.wantErr)
					return
				}
				require.NoError(t, err)
				require.NotNil(t, route)
				require.Equal(t, test.wantRoute, *route)
			})
		}
	})
}

func Test_shouldRetryGetDatabaseUsingSearchAsRoles(t *testing.T) {
	tests := []struct {
		name        string
		cf          *CLIConf
		tc          *client.TeleportClient
		inputError  error
		checkOutput require.BoolAssertionFunc
	}{
		{
			name: "tsh db connect",
			cf: &CLIConf{
				command: "db connect",
			},
			tc:          &client.TeleportClient{},
			inputError:  trace.NotFound("not found"),
			checkOutput: require.True,
		},
		{
			name: "tsh db login",
			cf: &CLIConf{
				command: "db connect",
			},
			tc:          &client.TeleportClient{},
			inputError:  trace.NotFound("not found"),
			checkOutput: require.True,
		},
		{
			name: "tsh proxy db",
			cf: &CLIConf{
				command: "db connect",
			},
			tc:          &client.TeleportClient{},
			inputError:  trace.NotFound("not found"),
			checkOutput: require.True,
		},
		{
			name: "not NotFound error",
			cf: &CLIConf{
				command: "db connect",
			},
			tc:          &client.TeleportClient{},
			inputError:  trace.ConnectionProblem(fmt.Errorf("timed out"), "timed out"),
			checkOutput: require.False,
		},
		{
			name: "not supported command",
			cf: &CLIConf{
				command: "db env",
			},
			tc:          &client.TeleportClient{},
			inputError:  trace.NotFound("not found"),
			checkOutput: require.False,
		},
		{
			name: "access request disabled",
			cf: &CLIConf{
				command:              "db connect",
				disableAccessRequest: true,
			},
			tc:          &client.TeleportClient{},
			inputError:  trace.NotFound("not found"),
			checkOutput: require.False,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			test.checkOutput(t, shouldRetryGetDatabaseUsingSearchAsRoles(test.cf, test.tc, test.inputError))
		})
	}
}
