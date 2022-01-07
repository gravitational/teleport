/*
Copyright 2021 Gravitational, Inc.

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
	"net"
	"testing"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/gravitational/teleport/lib/srv/db/mongodb"
	"github.com/gravitational/teleport/lib/srv/db/mysql"
	"github.com/gravitational/teleport/lib/srv/db/postgres"
	"github.com/stretchr/testify/require"
)

// TestInitCACert verifies automatic download of root certs for cloud databases.
func TestInitCACert(t *testing.T) {
	ctx := context.Background()
	testCtx := setupTestContext(ctx, t)

	selfHosted, err := types.NewDatabaseV3(types.Metadata{
		Name: "self-hosted",
	}, types.DatabaseSpecV3{
		Protocol: defaults.ProtocolPostgres,
		URI:      "localhost:5432",
	})
	require.NoError(t, err)

	rds, err := types.NewDatabaseV3(types.Metadata{
		Name: "rds",
	}, types.DatabaseSpecV3{
		Protocol: defaults.ProtocolPostgres,
		URI:      "localhost:5432",
		AWS:      types.AWS{Region: "us-east-1"},
	})
	require.NoError(t, err)

	rdsWithCert, err := types.NewDatabaseV3(types.Metadata{
		Name: "rds-with-cert",
	}, types.DatabaseSpecV3{
		Protocol: defaults.ProtocolPostgres,
		URI:      "localhost:5432",
		AWS:      types.AWS{Region: "us-east-1"},
		CACert:   "rds-test-cert",
	})
	require.NoError(t, err)

	redshift, err := types.NewDatabaseV3(types.Metadata{
		Name: "redshift",
	}, types.DatabaseSpecV3{
		Protocol: defaults.ProtocolPostgres,
		URI:      "localhost:5432",
		AWS:      types.AWS{Region: "us-east-1", Redshift: types.Redshift{ClusterID: "cluster-id"}},
	})
	require.NoError(t, err)

	cloudSQL, err := types.NewDatabaseV3(types.Metadata{
		Name: "cloud-sql",
	}, types.DatabaseSpecV3{
		Protocol: defaults.ProtocolPostgres,
		URI:      "localhost:5432",
		GCP:      types.GCPCloudSQL{ProjectID: "project-id", InstanceID: "instance-id"},
	})
	require.NoError(t, err)

	azureMySQL, err := types.NewDatabaseV3(types.Metadata{
		Name: "azure-mysql",
	}, types.DatabaseSpecV3{
		Protocol: defaults.ProtocolMySQL,
		URI:      "localhost:3306",
		Azure:    types.Azure{Name: "azure-mysql"},
	})
	require.NoError(t, err)

	allDatabases := []types.Database{
		selfHosted, rds, rdsWithCert, redshift, cloudSQL, azureMySQL,
	}

	tests := []struct {
		desc     string
		database string
		cert     string
	}{
		{
			desc:     "shouldn't download self-hosted CA",
			database: selfHosted.GetName(),
			cert:     selfHosted.GetCA(),
		},
		{
			desc:     "should download RDS CA when it's not set",
			database: rds.GetName(),
			cert:     fixtures.TLSCACertPEM,
		},
		{
			desc:     "shouldn't download RDS CA when it's set",
			database: rdsWithCert.GetName(),
			cert:     rdsWithCert.GetCA(),
		},
		{
			desc:     "should download Redshift CA when it's not set",
			database: redshift.GetName(),
			cert:     fixtures.TLSCACertPEM,
		},
		{
			desc:     "should download Cloud SQL CA when it's not set",
			database: cloudSQL.GetName(),
			cert:     fixtures.TLSCACertPEM,
		},
		{
			desc:     "should download Azure CA when it's not set",
			database: azureMySQL.GetName(),
			cert:     fixtures.TLSCACertPEM,
		},
	}

	databaseServer := testCtx.setupDatabaseServer(ctx, t, agentParams{
		Databases: allDatabases,
	})

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			var database types.Database
			for _, db := range databaseServer.getProxiedDatabases() {
				if db.GetName() == test.database {
					database = db
				}
			}
			require.NotNil(t, database)
			require.Equal(t, test.cert, database.GetCA())
		})
	}
}

// TestInitCACertCaching verifies root certificate is not re-downloaded if
// it was already downloaded before.
func TestInitCACertCaching(t *testing.T) {
	ctx := context.Background()
	testCtx := setupTestContext(ctx, t)

	rds, err := types.NewDatabaseV3(types.Metadata{
		Name: "rds",
	}, types.DatabaseSpecV3{
		Protocol: defaults.ProtocolPostgres,
		URI:      "localhost:5432",
		AWS:      types.AWS{Region: "us-east-1"},
	})
	require.NoError(t, err)

	databaseServer := testCtx.setupDatabaseServer(ctx, t, agentParams{
		Databases: []types.Database{rds},
		NoStart:   true,
	})

	// Initialize RDS cert for the first time.
	require.NoError(t, databaseServer.initCACert(ctx, rds))
	require.Equal(t, 1, databaseServer.cfg.CADownloader.(*fakeDownloader).count)

	// Reset it and initialize again, it should already be downloaded.
	rds.SetStatusCA("")
	require.NoError(t, databaseServer.initCACert(ctx, rds))
	require.Equal(t, 1, databaseServer.cfg.CADownloader.(*fakeDownloader).count)
}

type fakeDownloader struct {
	// cert is the cert to return as downloaded one.
	cert []byte
	// count keeps track of how many times the downloader has been invoked.
	count int
}

func (d *fakeDownloader) Download(context.Context, types.Database) ([]byte, error) {
	d.count++
	return d.cert, nil
}

type setupTLSTestCfg struct {
	commonName    string
	tlsMode       types.DatabaseTLSMode
	serverName    string
	caCert        string
	injectValidCA bool
}

func setupPostgres(ctx context.Context, t *testing.T, cfg *setupTLSTestCfg) *testContext {
	testCtx := setupTestContext(ctx, t)
	go testCtx.startProxy()

	testCtx.createUserAndRole(ctx, t, "bob", "admin", []string{types.Wildcard}, []string{types.Wildcard})

	if cfg.injectValidCA {
		cfg.caCert = string(testCtx.databaseCA.GetActiveKeys().TLS[0].Cert)
	}

	postgresServer, err := postgres.NewTestServer(common.TestServerConfig{
		Name:       "postgres",
		AuthClient: testCtx.authClient,
		CN:         cfg.commonName,
	})
	require.NoError(t, err)
	go postgresServer.Serve()
	t.Cleanup(func() { postgresServer.Close() })

	// Offline database server will be tried first and trigger connection error.
	postgresDB, err := types.NewDatabaseV3(types.Metadata{
		Name: "postgres",
	}, types.DatabaseSpecV3{
		Protocol: defaults.ProtocolPostgres,
		URI:      net.JoinHostPort("localhost", postgresServer.Port()),
		TLS: types.DatabaseTLS{
			Mode:       cfg.tlsMode,
			ServerName: cfg.serverName,
			CACert:     cfg.caCert,
		},
	})
	require.NoError(t, err)

	server1 := testCtx.setupDatabaseServer(ctx, t, agentParams{
		Databases: types.Databases{postgresDB},
	})

	go func() {
		for conn := range testCtx.proxyConn {
			go server1.HandleConnection(conn)
		}
	}()

	return testCtx
}

func setupMySQL(ctx context.Context, t *testing.T, cfg *setupTLSTestCfg) *testContext {
	testCtx := setupTestContext(ctx, t)
	go testCtx.startProxy()

	testCtx.createUserAndRole(ctx, t, "bob", "admin", []string{types.Wildcard}, []string{types.Wildcard})

	if cfg.injectValidCA {
		cfg.caCert = string(testCtx.databaseCA.GetActiveKeys().TLS[0].Cert)
	}

	mysqlServer, err := mysql.NewTestServer(common.TestServerConfig{
		Name:       "postgres",
		AuthClient: testCtx.authClient,
		CN:         cfg.commonName,
	})
	require.NoError(t, err)
	go mysqlServer.Serve()
	t.Cleanup(func() { mysqlServer.Close() })

	mysqlDB, err := types.NewDatabaseV3(types.Metadata{
		Name: "mysql",
	}, types.DatabaseSpecV3{
		Protocol: defaults.ProtocolMySQL,
		URI:      net.JoinHostPort("localhost", mysqlServer.Port()),
		TLS: types.DatabaseTLS{
			Mode:       cfg.tlsMode,
			ServerName: cfg.serverName,
			CACert:     cfg.caCert,
		},
	})
	require.NoError(t, err)

	server1 := testCtx.setupDatabaseServer(ctx, t, agentParams{
		Databases: types.Databases{mysqlDB},
	})

	go func() {
		for conn := range testCtx.proxyConn {
			go server1.HandleConnection(conn)
		}
	}()

	return testCtx
}

func setupMongo(ctx context.Context, t *testing.T, cfg *setupTLSTestCfg) *testContext {
	testCtx := setupTestContext(ctx, t)
	go testCtx.startProxy()

	testCtx.createUserAndRole(ctx, t, "bob", "admin", []string{types.Wildcard}, []string{types.Wildcard})

	if cfg.injectValidCA {
		cfg.caCert = string(testCtx.databaseCA.GetActiveKeys().TLS[0].Cert)
	}

	mongoServer, err := mongodb.NewTestServer(common.TestServerConfig{
		Name:       "mongo",
		AuthClient: testCtx.authClient,
		CN:         cfg.commonName,
	})
	require.NoError(t, err)
	go mongoServer.Serve()
	t.Cleanup(func() { mongoServer.Close() })

	mongoDB, err := types.NewDatabaseV3(types.Metadata{
		Name: "mongo",
	}, types.DatabaseSpecV3{
		Protocol: defaults.ProtocolMongoDB,
		URI:      net.JoinHostPort("localhost", mongoServer.Port()),
		TLS: types.DatabaseTLS{
			Mode:       cfg.tlsMode,
			ServerName: cfg.serverName,
			CACert:     cfg.caCert,
		},
	})
	require.NoError(t, err)

	server1 := testCtx.setupDatabaseServer(ctx, t, agentParams{
		Databases: types.Databases{mongoDB},
	})

	go func() {
		for conn := range testCtx.proxyConn {
			go server1.HandleConnection(conn)
		}
	}()

	return testCtx
}

func TestTLSConfiguration(t *testing.T) {
	tests := []struct {
		// name is the test name
		name string
		// commonName overrides the DNS name used by the DB.
		commonName string
		// serverName overrides the certificate DNS name.
		serverName string
		caCert     string
		// injectValidCA if true injects a valid CA certificate before connecting to the DB.
		injectValidCA bool
		tlsMode       types.DatabaseTLSMode
		// errMsg is an expected error message returned during connection.
		errMsg string
	}{
		{
			name: "use default config",
		},
		{
			name:       "incorrect server name",
			serverName: "abc.example.test",
			errMsg:     "certificate is valid for localhost, not abc.example.test",
		},
		{
			name:       "insecure ignores incorrect CN",
			tlsMode:    types.DatabaseTLSMode_INSECURE,
			commonName: "bad.example.test",
		},
		{
			name:       "verify CA ignores incorrect CN",
			tlsMode:    types.DatabaseTLSMode_VERIFY_CA,
			commonName: "bad.example.test",
		},
		{
			name:       "verify full fails if CN is incorrect",
			tlsMode:    types.DatabaseTLSMode_VERIFY_FULL,
			commonName: "bad.example.test",
			errMsg:     "certificate is valid for bad.example.test, not localhost",
		},
		{
			name:       "custom domain name with matching server name",
			commonName: "customDomain.example.test",
			serverName: "customDomain.example.test",
		},
		{
			name:   "invalid CA certificate",
			caCert: "invalidCert",
			errMsg: "invalid server CA certificate",
		},
		{
			name:          "valid provided CA",
			injectValidCA: true,
		},
		{
			name:          "server name can be overridden on provided CA",
			injectValidCA: true,
			commonName:    "testName.example.test",
			serverName:    "testName.example.test",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Run the same scenario for all supported databases.
			for _, dbType := range []string{
				defaults.ProtocolPostgres,
				defaults.ProtocolMySQL,
				defaults.ProtocolMongoDB,
			} {
				dbType := dbType
				t.Run(dbType, func(t *testing.T) {
					ctx := context.Background()
					cfg := &setupTLSTestCfg{
						commonName: tt.commonName,
						serverName: tt.serverName,
						tlsMode:    tt.tlsMode,
						caCert:     tt.caCert,
					}

					switch dbType {
					case defaults.ProtocolPostgres:
						testCtx := setupPostgres(ctx, t, cfg)
						t.Cleanup(func() {
							err := testCtx.Close()
							require.NoError(t, err)
						})

						psql, err := testCtx.postgresClient(ctx, "bob", "postgres", "postgres", "postgres")
						if tt.errMsg == "" {
							require.NoError(t, err)

							err = psql.Close(ctx)
							require.NoError(t, err)
						} else {
							require.Error(t, err)
							// skip error message validation here. Postgres driver by default tried to connect to
							// a database on localhost using IPv4 and IPv6. Docker doesn't enable IPv6 support
							// by default and that fails the check (on our CI and every default Docker installation )
							// as error is different "connection refused" that expected x509 related.
						}
					case defaults.ProtocolMySQL:
						testCtx := setupMySQL(ctx, t, cfg)
						t.Cleanup(func() {
							err := testCtx.Close()
							require.NoError(t, err)
						})

						mysqlConn, err := testCtx.mysqlClient("bob", "mysql", "admin")
						if tt.errMsg == "" {
							require.NoError(t, err)

							err = mysqlConn.Close()
							require.NoError(t, err)
						} else {
							require.Error(t, err)
							require.Contains(t, err.Error(), tt.errMsg)
						}
					case defaults.ProtocolMongoDB:
						testCtx := setupMongo(ctx, t, cfg)
						t.Cleanup(func() {
							err := testCtx.Close()
							require.NoError(t, err)
						})

						mongoConn, err := testCtx.mongoClient(ctx, "bob", "mongo", "admin")
						if tt.errMsg == "" {
							require.NoError(t, err)

							err = mongoConn.Disconnect(ctx)
							require.NoError(t, err)
						} else {
							require.Error(t, err)
							// Do not verify Mongo error message. On authentication error Mongo re-tries and
							// returns timeout instead of x509 related error.
						}
					default:
						t.Fatalf("unrecognized database: %s", dbType)
					}
				})
			}
		})
	}
}

func TestRDSCAURLForDatabase(t *testing.T) {
	tests := map[string]string{
		"us-west-1":     "https://truststore.pki.rds.amazonaws.com/us-west-1/us-west-1-bundle.pem",
		"ca-central-1":  "https://truststore.pki.rds.amazonaws.com/ca-central-1/ca-central-1-bundle.pem",
		"us-gov-east-1": "https://truststore.pki.us-gov-west-1.rds.amazonaws.com/us-gov-east-1/us-gov-east-1-bundle.pem",
		"us-gov-west-1": "https://truststore.pki.us-gov-west-1.rds.amazonaws.com/us-gov-west-1/us-gov-west-1-bundle.pem",
	}
	for region, expectURL := range tests {
		t.Run(region, func(t *testing.T) {
			database, err := types.NewDatabaseV3(types.Metadata{
				Name: "db",
			}, types.DatabaseSpecV3{
				Protocol: defaults.ProtocolPostgres,
				URI:      "localhost:5432",
				AWS:      types.AWS{Region: region},
			})
			require.NoError(t, err)
			require.Equal(t, expectURL, rdsCAURLForDatabase(database))
		})
	}
}

func TestRedshiftCAURLForDatabase(t *testing.T) {
	tests := map[string]string{
		"us-west-1":    "https://s3.amazonaws.com/redshift-downloads/amazon-trust-ca-bundle.crt",
		"ca-central-1": "https://s3.amazonaws.com/redshift-downloads/amazon-trust-ca-bundle.crt",
		"cn-north-1":   "https://s3.cn-north-1.amazonaws.com.cn/redshift-downloads-cn/amazon-trust-ca-bundle.crt",
	}
	for region, expectURL := range tests {
		t.Run(region, func(t *testing.T) {
			database, err := types.NewDatabaseV3(types.Metadata{
				Name: "db",
			}, types.DatabaseSpecV3{
				Protocol: defaults.ProtocolPostgres,
				URI:      "localhost:5432",
				AWS:      types.AWS{Region: region},
			})
			require.NoError(t, err)
			require.Equal(t, expectURL, redshiftCAURLForDatabase(database))
		})
	}
}
