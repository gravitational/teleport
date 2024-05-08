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
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509/pkix"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	sqladmin "google.golang.org/api/sqladmin/v1beta4"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud/mocks"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/gravitational/teleport/lib/srv/db/mongodb"
	"github.com/gravitational/teleport/lib/srv/db/mysql"
	"github.com/gravitational/teleport/lib/srv/db/postgres"
	"github.com/gravitational/teleport/lib/tlsca"
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

	redshiftServerless, err := types.NewDatabaseV3(types.Metadata{
		Name: "redshift",
	}, types.DatabaseSpecV3{
		Protocol: defaults.ProtocolPostgres,
		URI:      "localhost:5432",
		AWS: types.AWS{
			Region:    "us-east-1",
			AccountID: "123456789012",
			RedshiftServerless: types.RedshiftServerless{
				WorkgroupName: "workgroup",
			},
		},
	})
	require.NoError(t, err)

	memoryDB, err := types.NewDatabaseV3(types.Metadata{
		Name: "memorydb",
	}, types.DatabaseSpecV3{
		Protocol: defaults.ProtocolRedis,
		URI:      "localhost:5432",
		AWS:      types.AWS{Region: "us-east-1", MemoryDB: types.MemoryDB{ClusterName: "cluster"}},
	})
	require.NoError(t, err)

	mongodbAtlas, err := types.NewDatabaseV3(types.Metadata{
		Name: "mongodb-atlas",
	}, types.DatabaseSpecV3{
		Protocol: defaults.ProtocolMongoDB,
		URI:      "mongodb+srv://test.xxxx.mongodb.net",
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
		selfHosted, rds, rdsWithCert, redshift, redshiftServerless, cloudSQL, azureMySQL, memoryDB, mongodbAtlas,
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
			desc:     "should download Redshift Serverless CA when it's not set",
			database: redshiftServerless.GetName(),
			cert:     fixtures.TLSCACertPEM,
		},
		{
			desc:     "should download MemoryDB CA when it's not set",
			database: memoryDB.GetName(),
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
			cert:     fixtures.TLSCACertPEM + "\n" + fixtures.TLSCACertPEM, // Two CA files.
		},
		{
			desc:     "should download MongoDB Atlas CA when it's not set",
			database: mongodbAtlas.GetName(),
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
	require.Equal(t, int64(1), databaseServer.cfg.CADownloader.(*fakeDownloader).count)

	// Reset it and initialize again, it should already be downloaded.
	rds.SetStatusCA("")
	require.NoError(t, databaseServer.initCACert(ctx, rds))
	require.Equal(t, int64(2), databaseServer.cfg.CADownloader.(*fakeDownloader).count)
}

// TestUpdateCACerts given a cloud-hosted database, update the cached CA files
// when the CA version changes.
func TestUpdateCACerts(t *testing.T) {
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

	initialCA := generateDatabaseCA(t)
	caDownloader := &fakeDownloader{
		cert:    initialCA,
		version: []byte("initial"),
	}
	databaseServer := testCtx.setupDatabaseServer(ctx, t, agentParams{
		Databases:    []types.Database{rds},
		NoStart:      true,
		CADownloader: caDownloader,
	})

	// Initialize the CA certs as normal.
	require.NoError(t, databaseServer.initCACert(ctx, rds))
	require.Equal(t, string(initialCA), rds.GetStatusCA())

	// Change CA version and content in the downloader.
	updatedCA := generateDatabaseCA(t)
	caDownloader.cert = updatedCA
	caDownloader.version = []byte("second-version")

	// Trigger the update.
	newCAContents, err := databaseServer.getCACerts(ctx, rds)
	require.NoError(t, err)
	require.Equal(t, updatedCA, newCAContents)

	// Fetch CA certificate from the cached files.
	cached, err := databaseServer.getCACerts(ctx, rds)
	require.NoError(t, err)
	require.Equal(t, updatedCA, cached)
}

// TestCARenewer given a list of started databases, renew their CA every 24 hour
// ensuring only the CA that have changed contents are updated.
func TestCARenewer(t *testing.T) {
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

	initialCA := generateDatabaseCA(t)
	caDownloader := &fakeDownloader{
		cert:    initialCA,
		version: []byte("initial"),
	}
	databaseServer := testCtx.setupDatabaseServer(ctx, t, agentParams{
		Databases:    []types.Database{rds},
		NoStart:      true,
		CADownloader: caDownloader,
	})

	// Initialize the CA certs as normal.
	require.NoError(t, databaseServer.initCACert(ctx, rds))
	require.Equal(t, string(initialCA), rds.GetStatusCA())

	// Start the database CA renewer.
	renewerCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	caRenewerChan := make(chan struct{})
	go func() {
		databaseServer.startCARenewer(renewerCtx)
		caRenewerChan <- struct{}{}
	}()

	// Change CA version and content in the downloader.
	updatedCA := generateDatabaseCA(t)
	caDownloader.cert = updatedCA
	caDownloader.version = []byte("second-version")

	// Trigger the CA renews by advancing in time.
	testCtx.clock.Advance(caRenewInterval)

	// Check if the database status CA is updated with new contents.
	require.Eventually(t, func() bool {
		return atomic.LoadInt64(&caDownloader.count) == 2
	}, 5*time.Second, time.Second, "failed to wait the CA download")

	// Advance another time to trigger another renew.
	testCtx.clock.Advance(caRenewInterval)

	// Wait until renewer is gone to check database CA contents. This avoids,
	// test race condition.
	cancel()
	require.Eventually(t, func() bool {
		select {
		case <-caRenewerChan:
			return true
		default:
			return false
		}
	}, 5*time.Second, time.Second, "failed waiting CA renewer to stop")

	// Assert download counts and database CA contents.
	require.Equal(t, string(updatedCA), rds.GetStatusCA())
	require.Equal(t, int64(2), atomic.LoadInt64(&caDownloader.count))
}

// TestInitAzureCAs given Azure hosted databases, init their CAs ensuring the
// download and get version calls have the correct CA hint.
func TestInitAzureCAs(t *testing.T) {
	ctx := context.Background()
	testCtx := setupTestContext(ctx, t)

	azureDB, err := types.NewDatabaseV3(types.Metadata{
		Name: "azure",
	}, types.DatabaseSpecV3{
		Protocol: defaults.ProtocolPostgres,
		URI:      "localhost:5432",
		Azure: types.Azure{
			Name: "azure",
		},
	})
	require.NoError(t, err)

	supportedHints := []string{filepath.Base(azureCAURLBaltimore), filepath.Base(azureCAURLDigiCert)}
	initialCA := generateDatabaseCA(t)
	caDownloader := &fakeDownloader{
		cert:    initialCA,
		version: []byte("v1"),
		assertHintFunc: func(hint string) {
			require.Contains(
				t,
				supportedHints,
				hint,
				"CA download hint must be one of: %s. But got %q", strings.Join(supportedHints, ","),
				hint,
			)
		},
	}
	databaseServer := testCtx.setupDatabaseServer(ctx, t, agentParams{
		Databases:    []types.Database{azureDB},
		NoStart:      true,
		CADownloader: caDownloader,
	})

	require.NoError(t, databaseServer.initCACert(ctx, azureDB))
	// It must have the contents of two CAs. Since we're returning the same
	// content for both, it should have 2 instances of "initialCA".
	require.Equal(t, string(bytes.Join([][]byte{initialCA, initialCA}, []byte("\n"))), azureDB.GetStatusCA())
}

func generateDatabaseCA(t *testing.T) []byte {
	t.Helper()
	_, caCert, err := tlsca.GenerateSelfSignedCA(pkix.Name{
		CommonName: "localhost",
	}, []string{"localhost"}, defaults.CATTL)
	require.NoError(t, err)
	return caCert
}

type fakeDownloader struct {
	// cert is the cert to return as downloaded one.
	cert []byte
	// count keeps track of how many times the downloader has been invoked.
	count int64
	// version is the CA version returned when GetVersion is called.
	version []byte
	// assertHintFunc is a function used to assert the contents of "hint"
	// argument.
	assertHintFunc func(string)
}

func (d *fakeDownloader) Download(_ context.Context, _ types.Database, hint string) ([]byte, []byte, error) {
	if d.assertHintFunc != nil {
		d.assertHintFunc(hint)
	}

	atomic.AddInt64(&d.count, 1)
	return d.cert, d.version, nil
}

func (d *fakeDownloader) GetVersion(_ context.Context, _ types.Database, hint string) ([]byte, error) {
	if d.assertHintFunc != nil {
		d.assertHintFunc(hint)
	}

	if d.version == nil {
		return nil, trace.NotImplemented("GetVersion not implemented")
	}

	return d.version, nil
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
		for conn := range testCtx.fakeRemoteSite.ProxyConn() {
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
		for conn := range testCtx.fakeRemoteSite.ProxyConn() {
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
	t.Cleanup(func() {
		require.NoError(t, mongoServer.Close())
	})

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
	t.Cleanup(func() {
		require.NoError(t, server1.Close())
	})

	go func() {
		for conn := range testCtx.fakeRemoteSite.ProxyConn() {
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
						t.Cleanup(func() {
							err = mongoConn.Disconnect(ctx)
							require.NoError(t, err)
						})
						if tt.errMsg == "" {
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
		"us-west-1":      "https://truststore.pki.rds.amazonaws.com/us-west-1/us-west-1-bundle.pem",
		"ca-central-1":   "https://truststore.pki.rds.amazonaws.com/ca-central-1/ca-central-1-bundle.pem",
		"us-gov-east-1":  "https://truststore.pki.us-gov-west-1.rds.amazonaws.com/us-gov-east-1/us-gov-east-1-bundle.pem",
		"us-gov-west-1":  "https://truststore.pki.us-gov-west-1.rds.amazonaws.com/us-gov-west-1/us-gov-west-1-bundle.pem",
		"cn-northwest-1": "https://rds-truststore.s3.cn-north-1.amazonaws.com.cn/cn-northwest-1/cn-northwest-1-bundle.pem",
		"cn-north-1":     "https://rds-truststore.s3.cn-north-1.amazonaws.com.cn/cn-north-1/cn-north-1-bundle.pem",
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
		"us-west-1":      "https://s3.amazonaws.com/redshift-downloads/amazon-trust-ca-bundle.crt",
		"ca-central-1":   "https://s3.amazonaws.com/redshift-downloads/amazon-trust-ca-bundle.crt",
		"cn-north-1":     "https://s3.cn-north-1.amazonaws.com.cn/redshift-downloads-cn/amazon-trust-ca-bundle.crt",
		"cn-northwest-1": "https://s3.cn-north-1.amazonaws.com.cn/redshift-downloads-cn/amazon-trust-ca-bundle.crt",
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

func TestCADownloaderGetVersion(t *testing.T) {
	ctx := context.Background()

	// Given databases that have CA certs downloaded from a public CDN, the
	// downloader should return the CA version or a not implemented error.
	t.Run("from URL", func(t *testing.T) {
		t.Parallel()

		rds, err := types.NewDatabaseV3(types.Metadata{
			Name: "rds",
		}, types.DatabaseSpecV3{
			Protocol: defaults.ProtocolPostgres,
			URI:      "localhost:5432",
			AWS:      types.AWS{Region: "us-east-1"},
		})
		require.NoError(t, err)

		for _, tc := range []struct {
			desc              string
			database          types.Database
			hint              string
			version           []byte
			supportEtag       bool
			expectError       require.ErrorAssertionFunc
			expectedHTTPCalls int32
		}{
			{
				desc:              "support to ETag",
				database:          rds,
				version:           []byte("rds-test-cert"),
				supportEtag:       true,
				expectError:       require.NoError,
				expectedHTTPCalls: 1,
			},
			{
				desc:        "without support to ETag returns error",
				database:    rds,
				supportEtag: false,
				expectError: func(t require.TestingT, err error, _ ...interface{}) {
					require.Error(t, err)
					require.True(t, trace.IsNotImplemented(err), "expected trace.NotImplementedError but received %T", err)
				},
				expectedHTTPCalls: 1,
			},
		} {
			t.Run(tc.desc, func(t *testing.T) {
				var httpCalls int32

				// Start the CA CDN server.
				ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if tc.supportEtag {
						w.Header().Set("ETag", string(tc.version))
					}
					w.WriteHeader(http.StatusOK)
					atomic.AddInt32(&httpCalls, 1)
				}))
				defer ts.Close()

				downloader := &realDownloader{
					httpClient: &http.Client{
						Transport: &http.Transport{
							TLSClientConfig: &tls.Config{
								InsecureSkipVerify: true,
							},
							// Replace DialContext to always hit the test server.
							DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
								return (&net.Dialer{}).DialContext(ctx, network, strings.TrimPrefix(ts.URL, "https://"))
							},
						},
					},
				}

				version, err := downloader.GetVersion(ctx, tc.database, "")
				tc.expectError(t, err)
				require.Equal(t, tc.version, version)
				require.Equal(t, tc.expectedHTTPCalls, atomic.LoadInt32(&httpCalls))
			})
		}
	})

	// Given a CloudSQL database, the downloader should return an error because
	// it is not supported.
	t.Run("from CloudSQL", func(t *testing.T) {
		cloudSQL, err := types.NewDatabaseV3(types.Metadata{
			Name: "cloud-sql",
		}, types.DatabaseSpecV3{
			Protocol: defaults.ProtocolPostgres,
			URI:      "localhost:5432",
			GCP:      types.GCPCloudSQL{InstanceID: "instance-id", ProjectID: "project-id"},
		})
		require.NoError(t, err)

		downloader := &realDownloader{}
		_, err = downloader.GetVersion(ctx, cloudSQL, "")
		require.Error(t, err)
		require.True(t, trace.IsNotImplemented(err), "expected trace.NotImplementedError but received %T", err)
	})
}
func TestCADownloaderDownload(t *testing.T) {
	ctx := context.Background()

	// Given databases that have CA certs downloaded from a public CDN, the
	// downloader should return the CA contents and version.
	t.Run("from URL", func(t *testing.T) {
		t.Parallel()

		rds, err := types.NewDatabaseV3(types.Metadata{
			Name: "rds",
		}, types.DatabaseSpecV3{
			Protocol: defaults.ProtocolPostgres,
			URI:      "localhost:5432",
			AWS:      types.AWS{Region: "us-east-1"},
		})
		require.NoError(t, err)

		// Calculate the resource version using sha256 of the CA cert.
		certContents := []byte("rds-test-cert")
		sum := sha256.Sum256(certContents)

		for _, tc := range []struct {
			desc              string
			database          types.Database
			cert              []byte
			hint              string
			version           []byte
			supportEtag       bool
			statusCode        int
			expectError       require.ErrorAssertionFunc
			expectedHTTPCalls int32
		}{
			{
				desc:              "version from ETag",
				database:          rds,
				cert:              []byte("rds-test-cert"),
				version:           []byte("rds-test-version"),
				supportEtag:       true,
				statusCode:        http.StatusOK,
				expectError:       require.NoError,
				expectedHTTPCalls: 1,
			},
			{
				desc:              "version from contents",
				database:          rds,
				cert:              certContents,
				version:           sum[:],
				supportEtag:       false,
				statusCode:        http.StatusOK,
				expectError:       require.NoError,
				expectedHTTPCalls: 1,
			},
			{
				desc:              "download failure",
				database:          rds,
				supportEtag:       false,
				statusCode:        http.StatusInternalServerError,
				expectError:       require.Error,
				expectedHTTPCalls: 1,
			},
		} {
			t.Run(tc.desc, func(t *testing.T) {
				var httpCalls int32

				// Start the CA CDN server.
				ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if tc.supportEtag {
						w.Header().Set("ETag", string(tc.version))
					}
					w.WriteHeader(tc.statusCode)
					w.Write(tc.cert)
					atomic.AddInt32(&httpCalls, 1)
				}))
				defer ts.Close()

				downloader := &realDownloader{
					httpClient: &http.Client{
						Transport: &http.Transport{
							TLSClientConfig: &tls.Config{
								InsecureSkipVerify: true,
							},
							// Replace DialContext to always hit the test server.
							DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
								return (&net.Dialer{}).DialContext(ctx, network, strings.TrimPrefix(ts.URL, "https://"))
							},
						},
					},
				}

				contents, version, err := downloader.Download(ctx, tc.database, "")
				tc.expectError(t, err)
				require.Equal(t, tc.cert, contents)
				require.Equal(t, tc.version, version)
				require.Equal(t, tc.expectedHTTPCalls, atomic.LoadInt32(&httpCalls))
			})
		}
	})

	// Given a CloudSQL database, the downloader should return the database
	// certificate.
	t.Run("from CloudSQL", func(t *testing.T) {
		cloudSQL, err := types.NewDatabaseV3(types.Metadata{
			Name: "cloud-sql",
		}, types.DatabaseSpecV3{
			Protocol: defaults.ProtocolPostgres,
			URI:      "localhost:5432",
			GCP:      types.GCPCloudSQL{InstanceID: "instance-id", ProjectID: "project-id"},
		})
		require.NoError(t, err)

		certContents := []byte("cloud-sql-test-cert")
		fingerPrint := []byte("cert-fingerprint")
		downloader := &realDownloader{
			sqlAdminClient: &mocks.GCPSQLAdminClientMock{
				DatabaseInstance: &sqladmin.DatabaseInstance{
					ServerCaCert: &sqladmin.SslCert{
						Cert:            string(certContents),
						Sha1Fingerprint: string(fingerPrint),
					},
				},
			},
		}
		contents, version, err := downloader.Download(ctx, cloudSQL, "")
		require.NoError(t, err)
		require.Equal(t, certContents, contents)
		require.Equal(t, fingerPrint, version)
	})
}
