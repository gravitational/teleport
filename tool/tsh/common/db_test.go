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
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/breaker"
	"github.com/gravitational/teleport/api/constants"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/observability/tracing"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

// TestDatabaseLogin tests "tsh db login" command and verifies "tsh db
// env/config" after login.
func TestDatabaseLogin(t *testing.T) {
	tmpHomePath := t.TempDir()

	connector := mockConnector(t)

	alice, err := types.NewUser("alice@example.com")
	require.NoError(t, err)
	alice.SetDatabaseUsers([]string{"admin"})
	alice.SetDatabaseNames([]string{"default"})
	alice.SetRoles([]string{"access"})

	authProcess, proxyProcess := makeTestServers(t, withBootstrap(connector, alice),
		withAuthConfig(func(cfg *servicecfg.AuthConfig) {
			cfg.NetworkingConfig.SetProxyListenerMode(types.ProxyListenerMode_Multiplex)
		}),
		withConfig(func(cfg *servicecfg.Config) {
			// separate MySQL port with TLS routing.
			cfg.Proxy.MySQLAddr = utils.NetAddr{AddrNetwork: "tcp", Addr: net.JoinHostPort("127.0.0.1", ports.Pop())}
		}))
	makeTestDatabaseServer(t, authProcess, proxyProcess,
		servicecfg.Database{
			Name:     "postgres",
			Protocol: defaults.ProtocolPostgres,
			URI:      "localhost:5432",
		}, servicecfg.Database{
			Name:     "mysql",
			Protocol: defaults.ProtocolMySQL,
			URI:      "localhost:3306",
		}, servicecfg.Database{
			Name:     "cassandra",
			Protocol: defaults.ProtocolCassandra,
			URI:      "localhost:9042",
		}, servicecfg.Database{
			Name:     "snowflake",
			Protocol: defaults.ProtocolSnowflake,
			URI:      "localhost.snowflakecomputing.com",
		}, servicecfg.Database{
			Name:     "mongo",
			Protocol: defaults.ProtocolMongoDB,
			URI:      "localhost:27017",
		}, servicecfg.Database{
			Name:     "mssql",
			Protocol: defaults.ProtocolSQLServer,
			URI:      "localhost:1433",
		}, servicecfg.Database{
			Name:     "dynamodb",
			Protocol: defaults.ProtocolDynamoDB,
			URI:      "", // uri can be blank for DynamoDB, it will be derived from the region and requests.
			AWS: servicecfg.DatabaseAWS{
				AccountID:  "123456789012",
				ExternalID: "123123123",
				Region:     "us-west-1",
			},
		})

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

	testCases := []struct {
		databaseName          string
		expectCertsLen        int
		expectKeysLen         int
		expectErrForConfigCmd bool
		expectErrForEnvCmd    bool
	}{
		{
			databaseName:   "postgres",
			expectCertsLen: 1,
		},
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
	}

	// Note: keystore currently races when multiple tsh clients work in the
	// same profile dir (e.g. StatusCurrent might fail reading if someone else
	// is writing a key at the same time).
	// Thus, in order to speed up this test, we clone the profile dir for each subtest
	// to enable parallel test runs.
	// Copying the profile dir is faster than sequential login for each database.
	for _, test := range testCases {
		test := test
		t.Run(fmt.Sprintf("%v/%v", "tsh db login", test.databaseName), func(t *testing.T) {
			t.Parallel()
			tmpHomePath := mustCloneTempDir(t, tmpHomePath)
			err := Run(context.Background(), []string{
				// default --db-user and --db-name are selected from roles.
				"db", "login", test.databaseName,
			}, setHomePath(tmpHomePath))
			require.NoError(t, err)

			// Fetch the active profile.
			clientStore := client.NewFSClientStore(tmpHomePath)
			profile, err := clientStore.ReadProfileStatus(proxyAddr.Host())
			require.NoError(t, err)
			require.Equal(t, alice.GetName(), profile.Username)

			// Verify certificates.
			certs, keys, err := decodePEM(profile.DatabaseCertPathForCluster("", test.databaseName))
			require.NoError(t, err)
			require.Equal(t, test.expectCertsLen, len(certs)) // don't use require.Len, because it spams PEM bytes on fail.
			require.Equal(t, test.expectKeysLen, len(keys))   // don't use require.Len, because it spams PEM bytes on fail.

			t.Run(fmt.Sprintf("%v/%v", "tsh db config", test.databaseName), func(t *testing.T) {
				t.Parallel()
				err := Run(context.Background(), []string{
					"db", "config", test.databaseName,
				}, setHomePath(tmpHomePath))

				if test.expectErrForConfigCmd {
					require.Error(t, err)
				} else {
					require.NoError(t, err)
				}
			})

			t.Run(fmt.Sprintf("%v/%v", "tsh db env", test.databaseName), func(t *testing.T) {
				t.Parallel()
				err := Run(context.Background(), []string{
					"db", "env", test.databaseName,
				}, setHomePath(tmpHomePath))

				if test.expectErrForEnvCmd {
					require.Error(t, err)
				} else {
					require.NoError(t, err)
				}
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

func TestListDatabase(t *testing.T) {
	lib.SetInsecureDevMode(true)
	defer lib.SetInsecureDevMode(false)

	s := newTestSuite(t,
		withRootConfigFunc(func(cfg *servicecfg.Config) {
			cfg.Auth.NetworkingConfig.SetProxyListenerMode(types.ProxyListenerMode_Multiplex)
			cfg.Databases.Enabled = true
			cfg.Databases.Databases = []servicecfg.Database{{
				Name:     "root-postgres",
				Protocol: defaults.ProtocolPostgres,
				URI:      "localhost:5432",
			}}
		}),
		withLeafCluster(),
		withLeafConfigFunc(func(cfg *servicecfg.Config) {
			cfg.Databases.Enabled = true
			cfg.Databases.Databases = []servicecfg.Database{{
				Name:     "leaf-postgres",
				Protocol: defaults.ProtocolPostgres,
				URI:      "localhost:5432",
			}}
		}),
	)

	mustLoginSetEnv(t, s)

	captureStdout := new(bytes.Buffer)
	err := Run(context.Background(), []string{
		"db",
		"ls",
		"--insecure",
		"--debug",
	}, setCopyStdout(captureStdout))

	require.NoError(t, err)
	require.Contains(t, captureStdout.String(), "root-postgres")

	captureStdout.Reset()
	err = Run(context.Background(), []string{
		"db",
		"ls",
		"--cluster",
		"leaf1",
		"--insecure",
		"--debug",
	}, setCopyStdout(captureStdout))

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

func makeTestDatabaseServer(t *testing.T, auth *service.TeleportProcess, proxy *service.TeleportProcess, dbs ...servicecfg.Database) (db *service.TeleportProcess) {
	// Proxy uses self-signed certificates in tests.
	lib.SetInsecureDevMode(true)

	cfg := servicecfg.MakeDefaultConfig()
	cfg.Hostname = "localhost"
	cfg.DataDir = t.TempDir()
	cfg.CircuitBreakerConfig = breaker.NoopBreakerConfig()

	proxyAddr, err := proxy.ProxyWebAddr()
	require.NoError(t, err)

	cfg.SetAuthServerAddress(*proxyAddr)

	token, err := proxy.Config.Token()
	require.NoError(t, err)

	cfg.SetToken(token)
	cfg.SSH.Enabled = false
	cfg.Auth.Enabled = false
	cfg.Proxy.Enabled = false
	cfg.Databases.Enabled = true
	cfg.Databases.Databases = dbs
	cfg.Log = utils.NewLoggerForTests()

	db = runTeleport(t, cfg)

	// Wait for all databases to register to avoid races.
	waitForDatabases(t, auth, dbs)
	return db
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
