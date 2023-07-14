/*
Copyright 2015-2017 Gravitational, Inc.

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

package common

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
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
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/observability/tracing"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/tool/teleport/testenv"
)

func TestTshDB(t *testing.T) {
	// this speeds up test suite setup substantially, which is where
	// tests spend the majority of their time, especially when leaf
	// clusters are setup.
	testenv.WithResyncInterval(t, 0)
	// Proxy uses self-signed certificates in tests.
	testenv.WithInsecureDevMode(t, true)
	t.Run("Login", testDatabaseLogin)
	t.Run("List", testListDatabase)
	t.Run("FilterActiveDatabases", testFilterActiveDatabases)
}

// testDatabaseLogin tests "tsh db login" command and verifies "tsh db
// env/config" after login.
func testDatabaseLogin(t *testing.T) {
	t.Parallel()
	alice, err := types.NewUser("alice@example.com")
	require.NoError(t, err)
	// to use default --db-user and --db-name selection, make a user with just
	// one of each allowed.
	alice.SetDatabaseUsers([]string{"admin"})
	alice.SetDatabaseNames([]string{"default"})
	alice.SetRoles([]string{"access"})
	s := newTestSuite(t,
		withRootConfigFunc(func(cfg *servicecfg.Config) {
			cfg.Auth.BootstrapResources = append(cfg.Auth.BootstrapResources, alice)
			cfg.Auth.NetworkingConfig.SetProxyListenerMode(types.ProxyListenerMode_Multiplex)
			// separate MySQL port with TLS routing.
			// set the public address to be sure even on v2+, tsh clients will see the separate port.
			mySQLAddr := localListenerAddr()
			cfg.Proxy.MySQLAddr = utils.NetAddr{AddrNetwork: "tcp", Addr: mySQLAddr}
			cfg.Proxy.MySQLPublicAddrs = []utils.NetAddr{{AddrNetwork: "tcp", Addr: mySQLAddr}}
			cfg.Databases.Enabled = true
			cfg.Databases.Databases = []servicecfg.Database{
				{
					Name:     "postgres-local",
					Protocol: defaults.ProtocolPostgres,
					URI:      "localhost:5432",
					StaticLabels: map[string]string{
						"env": "local",
					},
				}, {
					Name:     "postgres-rds-us-west-1-123456789012",
					Protocol: defaults.ProtocolPostgres,
					URI:      "localhost:5432",
					StaticLabels: map[string]string{
						types.DiscoveredNameLabel: "postgres",
						"region":                  "us-west-1",
						"env":                     "prod",
					},
					AWS: servicecfg.DatabaseAWS{
						AccountID: "123456789012",
						Region:    "us-west-1",
						RDS: servicecfg.DatabaseAWSRDS{
							InstanceID: "postgres",
						},
					},
				}, {
					Name:     "postgres-rds-us-west-2-123456789012",
					Protocol: defaults.ProtocolPostgres,
					URI:      "localhost:5432",
					StaticLabels: map[string]string{
						types.DiscoveredNameLabel: "postgres",
						"region":                  "us-west-2",
						"env":                     "prod",
					},
					AWS: servicecfg.DatabaseAWS{
						AccountID: "123456789012",
						Region:    "us-west-2",
						RDS: servicecfg.DatabaseAWSRDS{
							InstanceID: "postgres",
						},
					},
				}, {
					Name:     "mysql",
					Protocol: defaults.ProtocolMySQL,
					URI:      "localhost:3306",
				}, {
					Name:     "cassandra",
					Protocol: defaults.ProtocolCassandra,
					URI:      "localhost:9042",
				}, {
					Name:     "snowflake",
					Protocol: defaults.ProtocolSnowflake,
					URI:      "localhost.snowflakecomputing.com",
				}, {
					Name:     "mongo",
					Protocol: defaults.ProtocolMongoDB,
					URI:      "localhost:27017",
				}, {
					Name:     "mssql",
					Protocol: defaults.ProtocolSQLServer,
					URI:      "localhost:1433",
				}, {
					Name:     "dynamodb",
					Protocol: defaults.ProtocolDynamoDB,
					URI:      "", // uri can be blank for DynamoDB, it will be derived from the region and requests.
					AWS: servicecfg.DatabaseAWS{
						AccountID:  "123456789012",
						ExternalID: "123123123",
						Region:     "us-west-1",
					},
				}}
		}),
	)
	s.user = alice

	// Log into Teleport cluster.
	tmpHomePath, _ := mustLogin(t, s)

	testCases := []struct {
		// databaseName should be the full database name.
		databaseName string
		// dbSelectors can be any of db name, --labels, --query predicate,
		// and defaults to be databaseName if not set.
		dbSelectors           []string
		expectCertsLen        int
		expectKeysLen         int
		expectErrForConfigCmd bool
		expectErrForEnvCmd    bool
	}{
		{
			databaseName:       "mongo",
			expectCertsLen:     1,
			expectKeysLen:      1,
			expectErrForEnvCmd: true, // "tsh db env" not supported for Mongo.
		},
		{
			databaseName:          "mssql",
			expectCertsLen:        1,
			expectErrForConfigCmd: true, // "tsh db config" not supported for MSSQL.
			expectErrForEnvCmd:    true, // "tsh db env" not supported for MSSQL.
		},
		{
			databaseName:          "mysql",
			expectCertsLen:        1,
			expectErrForConfigCmd: false, // "tsh db config" is supported for MySQL with TLS routing & separate MySQL port.
			expectErrForEnvCmd:    false, // "tsh db env" not supported for MySQL with TLS routing & separate MySQL port.
		},
		{
			databaseName:          "cassandra",
			expectCertsLen:        1,
			expectErrForConfigCmd: true, // "tsh db config" not supported for Cassandra.
			expectErrForEnvCmd:    true, // "tsh db env" not supported for Cassandra.
		},
		{
			databaseName:          "snowflake",
			expectCertsLen:        1,
			expectErrForConfigCmd: true, // "tsh db config" not supported for Snowflake.
			expectErrForEnvCmd:    true, // "tsh db env" not supported for Snowflake.
		},
		{
			databaseName:          "dynamodb",
			expectCertsLen:        1,
			expectErrForConfigCmd: true, // "tsh db config" not supported for DynamoDB.
			expectErrForEnvCmd:    true, // "tsh db env" not supported for DynamoDB.
		},
		{
			databaseName: "postgres-local",
			// select by labels alone.
			dbSelectors:    []string{"--labels", "env=local"},
			expectCertsLen: 1,
		},
		{
			databaseName: "postgres-rds-us-west-1-123456789012",
			// select by query alone.
			dbSelectors:    []string{"--query", `labels.env=="prod" && labels.region == "us-west-1"`},
			expectCertsLen: 1,
		},
		{
			databaseName: "postgres-rds-us-west-2-123456789012",
			// select by uniquely identifying prefix.
			dbSelectors:    []string{"postgres-rds-us-west-2"},
			expectCertsLen: 1,
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
		t.Run(test.databaseName, func(t *testing.T) {
			t.Parallel()
			tmpHomePath := mustCloneTempDir(t, tmpHomePath)
			selectors := test.dbSelectors
			if len(selectors) == 0 {
				selectors = []string{test.databaseName}
			}
			args := append([]string{
				// default --db-user and --db-name are selected from roles.
				"db", "login",
			}, selectors...)
			err := Run(context.Background(), args, setHomePath(tmpHomePath))
			require.NoError(t, err)

			// Fetch the active profile.
			clientStore := client.NewFSClientStore(tmpHomePath)
			profile, err := clientStore.ReadProfileStatus(s.root.Config.Proxy.WebAddr.String())
			require.NoError(t, err)
			require.Equal(t, s.user.GetName(), profile.Username)

			// Verify certificates.
			// grab the certs using the actual database name to verify certs.
			certs, keys, err := decodePEM(profile.DatabaseCertPathForCluster("", test.databaseName))
			require.NoError(t, err)
			require.Equal(t, test.expectCertsLen, len(certs)) // don't use require.Len, because it spams PEM bytes on fail.
			require.Equal(t, test.expectKeysLen, len(keys))   // don't use require.Len, because it spams PEM bytes on fail.

			t.Run("print info", func(t *testing.T) {
				// organize these as parallel subtests in a group, so we can run
				// them in parallel together before the logout test runs below.
				t.Run("config", func(t *testing.T) {
					t.Parallel()
					args := append([]string{
						"db", "config",
					}, selectors...)
					err := Run(context.Background(), args, setHomePath(tmpHomePath))

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
					err := Run(context.Background(), args, setHomePath(tmpHomePath))

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
				err := Run(context.Background(), args, setHomePath(tmpHomePath))
				require.NoError(t, err)
			})
		})
	}
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
		"login", "--insecure", "--debug", "--auth", connector.GetName(), "--proxy", proxyAddr.String(),
	}, setHomePath(tmpHomePath), CliOption(func(cf *CLIConf) error {
		cf.MockSSOLogin = mockSSOLogin(t, authServer, alice)
		return nil
	}))
	require.NoError(t, err)

	defaultAuthPref, err := authServer.GetAuthPreference(ctx)
	require.NoError(t, err)
	tests := map[string]struct {
		clusterAuthPref types.AuthPreference
		route           *tlsca.RouteToDatabase
		setupTC         func(*client.TeleportClient)
		wantLocalProxy  bool
		wantTunnel      bool
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
			wantLocalProxy: true,
			wantTunnel:     true,
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
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			require.NoError(t, authServer.SetAuthPreference(ctx, tt.clusterAuthPref))
			t.Cleanup(func() {
				require.NoError(t, authServer.SetAuthPreference(ctx, defaultAuthPref))
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
			requires := getDBConnectLocalProxyRequirement(ctx, tc, route)
			require.Equal(t, tt.wantLocalProxy, requires.localProxy)
			require.Equal(t, tt.wantTunnel, requires.tunnel)
			if requires.tunnel {
				require.Len(t, requires.tunnelReasons, 1)
				require.Contains(t, requires.tunnelReasons[0], "MFA is required")
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

	tshHome, _ := mustLogin(t, s)

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
	}

	ca, err := tlsca.FromKeys([]byte(fixtures.TLSCACertPEM), []byte(fixtures.TLSCAKeyPEM))
	require.NoError(t, err)
	privateKey, err := rsa.GenerateKey(rand.Reader, constants.RSAKeySize)
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

			cliConf := &CLIConf{DatabaseUser: tc.databaseUserName, DatabaseName: tc.databaseName}
			got, err := dbInfoHasChanged(cliConf, certPath)
			require.NoError(t, err)
			require.Equal(t, tc.wantUserHasChanged, got)
		})
	}
}

func waitForDatabases(t *testing.T, auth *service.TeleportProcess, dbs []servicecfg.Database) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for {
		select {
		case <-time.After(500 * time.Millisecond):
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
			t.Fatal("databases not registered after 10s")
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

func testFilterActiveDatabases(t *testing.T) {
	t.Parallel()
	// setup some databases and "active" routes to test filtering
	db1, route1 := makeDBConfigAndRoute("foobar", map[string]string{"env": "dev", "svc": "fooer"})
	db1AWS, route1AWS := makeDBConfigAndRoute("foobar-us-west-1-123456789012", map[string]string{"env": "prod", "region": "us-west-1"})
	db1Azure, route1Azure := makeDBConfigAndRoute("foobar-westus-11111", map[string]string{"env": "prod", "region": "westus"})
	db2, route2 := makeDBConfigAndRoute("bazqux", map[string]string{"env": "dev", "svc": "bazzer"})
	db2AWS, route2AWS := makeDBConfigAndRoute("bazqux-us-west-1-123456789012", map[string]string{"env": "prod", "region": "us-west-1"})
	db3, route3 := makeDBConfigAndRoute("some-unique-name", map[string]string{"env": "dev"})
	routes := []tlsca.RouteToDatabase{route1, route1AWS, route1Azure, route2, route2AWS, route3}
	s := newTestSuite(t,
		withRootConfigFunc(func(cfg *servicecfg.Config) {
			cfg.Auth.NetworkingConfig.SetProxyListenerMode(types.ProxyListenerMode_Multiplex)
			cfg.Databases.Enabled = true
			cfg.Databases.Databases = []servicecfg.Database{
				db1, db1AWS, db1Azure, db2, db2AWS, db3,
			}
		}),
	)

	// Log into Teleport cluster.
	tmpHomePath, _ := mustLogin(t, s)

	tests := []struct {
		name,
		dbName,
		labels,
		query string
		wantAPICall bool
		wantRoutes  []tlsca.RouteToDatabase
	}{
		{
			name:        "by exact name",
			dbName:      route1.ServiceName,
			wantAPICall: false,
			wantRoutes:  []tlsca.RouteToDatabase{route1},
		},
		{
			name:        "by name prefix",
			dbName:      "foo",
			wantAPICall: false,
			wantRoutes:  []tlsca.RouteToDatabase{route1, route1AWS, route1Azure},
		},
		{
			name:        "by labels",
			labels:      "env=dev",
			wantAPICall: true,
			wantRoutes:  []tlsca.RouteToDatabase{route1, route2, route3},
		},
		{
			name:        "by query",
			query:       `labels.env == "dev"`,
			wantAPICall: true,
			wantRoutes:  []tlsca.RouteToDatabase{route1, route2, route3},
		},
		{
			name:        "by name prefix and labels",
			dbName:      "foo",
			labels:      "env=prod",
			wantAPICall: true,
			wantRoutes:  []tlsca.RouteToDatabase{route1AWS, route1Azure},
		},
		{
			name:        "by name prefix and query",
			dbName:      "foo",
			query:       `labels.region == "us-west-1"`,
			wantAPICall: true,
			wantRoutes:  []tlsca.RouteToDatabase{route1AWS},
		},
		{
			name:        "by labels and query",
			labels:      "env=dev",
			query:       `hasPrefix(name, "some-uniq")`,
			wantAPICall: true,
			wantRoutes:  []tlsca.RouteToDatabase{route3},
		},
		{
			name:        "by name prefix and labels and query",
			dbName:      "foo",
			labels:      "env=prod",
			query:       `labels.region == "westus"`,
			wantAPICall: true,
			wantRoutes:  []tlsca.RouteToDatabase{route1Azure},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithCancel(context.Background())
			t.Cleanup(cancel)
			cf := &CLIConf{
				Context:             ctx,
				HomePath:            tmpHomePath,
				DatabaseService:     tt.dbName,
				Labels:              tt.labels,
				PredicateExpression: tt.query,
			}
			tc, err := makeClient(cf)
			require.NoError(t, err)
			routes, dbs, err := filterActiveDatabases(ctx, tc, routes)
			require.NoError(t, err)
			require.Empty(t, cmp.Diff(tt.wantRoutes, routes))
			if tt.wantAPICall {
				require.Equal(t, len(routes), len(dbs),
					"returned routes should have corresponding types.Databases")
				return
			}
			require.Zero(t, len(dbs), "unexpected API call to ListDatabases")
		})
	}
}

func TestResourceSelectorsFormatting(t *testing.T) {
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
	route := tlsca.RouteToDatabase{ServiceName: name}
	return db, route
}
