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

package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/pem"
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gravitational/teleport/api/constants"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

// TestDatabaseLogin verifies "tsh db login" command.
func TestDatabaseLogin(t *testing.T) {
	tmpHomePath := t.TempDir()

	connector := mockConnector(t)

	alice, err := types.NewUser("alice@example.com")
	require.NoError(t, err)
	alice.SetRoles([]string{"access"})

	authProcess, proxyProcess := makeTestServers(t, withBootstrap(connector, alice))
	makeTestDatabaseServer(t, authProcess, proxyProcess, service.Database{
		Name:     "postgres",
		Protocol: defaults.ProtocolPostgres,
		URI:      "localhost:5432",
	}, service.Database{
		Name:     "mongo",
		Protocol: defaults.ProtocolMongoDB,
		URI:      "localhost:27017",
	})

	authServer := authProcess.GetAuthServer()
	require.NotNil(t, authServer)

	proxyAddr, err := proxyProcess.ProxyWebAddr()
	require.NoError(t, err)

	// Log into Teleport cluster.
	err = Run([]string{
		"login", "--insecure", "--debug", "--auth", connector.GetName(), "--proxy", proxyAddr.String(),
	}, setHomePath(tmpHomePath), cliOption(func(cf *CLIConf) error {
		cf.mockSSOLogin = mockSSOLogin(t, authServer, alice)
		return nil
	}))
	require.NoError(t, err)

	// Fetch the active profile.
	profile, err := client.StatusFor(tmpHomePath, proxyAddr.Host(), alice.GetName())
	require.NoError(t, err)

	// Log into test Postgres database.
	err = Run([]string{
		"db", "login", "--debug", "postgres",
	}, setHomePath(tmpHomePath))
	require.NoError(t, err)

	// Verify Postgres identity file contains certificate.
	certs, keys, err := decodePEM(profile.DatabaseCertPathForCluster("", "postgres"))
	require.NoError(t, err)
	require.Len(t, certs, 1)
	require.Len(t, keys, 0)

	// Log into test Mongo database.
	err = Run([]string{
		"db", "login", "--debug", "--db-user", "admin", "mongo",
	}, setHomePath(tmpHomePath))
	require.NoError(t, err)

	// Verify Mongo identity file contains both certificate and key.
	certs, keys, err = decodePEM(profile.DatabaseCertPathForCluster("", "mongo"))
	require.NoError(t, err)
	require.Len(t, certs, 1)
	require.Len(t, keys, 1)
}

func TestFormatDatabaseListCommand(t *testing.T) {
	t.Run("default", func(t *testing.T) {
		require.Equal(t, "tsh db ls", formatDatabaseListCommand(""))
	})

	t.Run("with cluster flag", func(t *testing.T) {
		require.Equal(t, "tsh db ls --cluster=leaf", formatDatabaseListCommand("leaf"))
	})
}

func TestFormatConfigCommand(t *testing.T) {
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
			require.NoError(t, ioutil.WriteFile(certPath, certBytes, 0600))

			cliConf := &CLIConf{DatabaseUser: tc.databaseUserName, DatabaseName: tc.databaseName}
			got, err := dbInfoHasChanged(cliConf, certPath)
			require.NoError(t, err)
			require.Equal(t, tc.wantUserHasChanged, got)
		})
	}
}

func makeTestDatabaseServer(t *testing.T, auth *service.TeleportProcess, proxy *service.TeleportProcess, dbs ...service.Database) (db *service.TeleportProcess) {
	// Proxy uses self-signed certificates in tests.
	lib.SetInsecureDevMode(true)

	cfg := service.MakeDefaultConfig()
	cfg.Hostname = "localhost"
	cfg.DataDir = t.TempDir()

	proxyAddr, err := proxy.ProxyWebAddr()
	require.NoError(t, err)

	cfg.AuthServers = []utils.NetAddr{*proxyAddr}
	cfg.Token = proxy.Config.Token
	cfg.SSH.Enabled = false
	cfg.Auth.Enabled = false
	cfg.Databases.Enabled = true
	cfg.Databases.Databases = dbs
	cfg.Log = utils.NewLoggerForTests()

	db, err = service.NewTeleport(cfg)
	require.NoError(t, err)
	require.NoError(t, db.Start())

	t.Cleanup(func() {
		db.Close()
	})

	// Wait for database agent to start.
	eventCh := make(chan service.Event, 1)
	db.WaitForEvent(db.ExitContext(), service.DatabasesReady, eventCh)
	select {
	case <-eventCh:
	case <-time.After(10 * time.Second):
		t.Fatal("database server didn't start after 10s")
	}

	// Wait for all databases to register to avoid races.
	for _, database := range dbs {
		waitForDatabase(t, auth, database)
	}

	return db
}

func waitForDatabase(t *testing.T, auth *service.TeleportProcess, db service.Database) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for {
		select {
		case <-time.After(500 * time.Millisecond):
			all, err := auth.GetAuthServer().GetDatabaseServers(ctx, apidefaults.Namespace)
			require.NoError(t, err)
			for _, a := range all {
				if a.GetName() == db.Name {
					return
				}
			}
		case <-ctx.Done():
			t.Fatal("database not registered after 10s")
		}
	}
}

// decodePEM sorts out specified PEM file into certificates and private keys.
func decodePEM(pemPath string) (certs []pem.Block, keys []pem.Block, err error) {
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
		case "RSA PRIVATE KEY":
			keys = append(keys, *block)
		}
	}
	return certs, keys, nil
}

// fakeExec implements execer interface for mocking purposes.
type fakeExec struct {
	// execOutput maps binary name and output that should be returned on RunCommand().
	// Map is also being used to check if a binary exist. Command line args are not supported.
	execOutput map[string][]byte
}

func (f fakeExec) RunCommand(cmd string, _ ...string) ([]byte, error) {
	out, found := f.execOutput[cmd]
	if !found {
		return nil, errors.New("binary not found")
	}

	return out, nil
}

func (f fakeExec) LookPath(path string) (string, error) {
	if _, found := f.execOutput[path]; found {
		return "", nil
	}
	return "", trace.NotFound("not found")
}

func TestCliCommandBuilderGetConnectCommand(t *testing.T) {
	conf := &CLIConf{
		HomePath: t.TempDir(),
		Proxy:    "proxy",
		UserHost: "localhost",
		SiteName: "db.example.com",
	}

	tc, err := makeClient(conf, true)
	require.NoError(t, err)

	profile := &client.ProfileStatus{
		Name:     "example.com",
		Username: "bob",
		Dir:      "/tmp",
	}

	tests := []struct {
		name       string
		dbProtocol string
		execer     *fakeExec
		cmd        []string
		wantErr    bool
	}{
		{
			name:       "postgres",
			dbProtocol: defaults.ProtocolPostgres,
			cmd: []string{"psql",
				"postgres://myUser@localhost:12345/mydb?sslrootcert=/tmp/keys/example.com/cas/root.pem&" +
					"sslcert=/tmp/keys/example.com/bob-db/db.example.com/mysql-x509.pem&" +
					"sslkey=/tmp/keys/example.com/bob&sslmode=verify-full"},
			wantErr: false,
		},
		{
			name:       "cockroach",
			dbProtocol: defaults.ProtocolCockroachDB,
			execer: &fakeExec{
				execOutput: map[string][]byte{
					"cockroach": []byte(""),
				},
			},
			cmd: []string{"cockroach", "sql", "--url",
				"postgres://myUser@localhost:12345/mydb?sslrootcert=/tmp/keys/example.com/cas/root.pem&" +
					"sslcert=/tmp/keys/example.com/bob-db/db.example.com/mysql-x509.pem&" +
					"sslkey=/tmp/keys/example.com/bob&sslmode=verify-full"},
			wantErr: false,
		},
		{
			name:       "cockroach psql fallback",
			dbProtocol: defaults.ProtocolCockroachDB,
			execer:     &fakeExec{},
			cmd: []string{"psql",
				"postgres://myUser@localhost:12345/mydb?sslrootcert=/tmp/keys/example.com/cas/root.pem&" +
					"sslcert=/tmp/keys/example.com/bob-db/db.example.com/mysql-x509.pem&" +
					"sslkey=/tmp/keys/example.com/bob&sslmode=verify-full"},
			wantErr: false,
		},
		{
			name:       "mariadb",
			dbProtocol: defaults.ProtocolMySQL,
			execer: &fakeExec{
				execOutput: map[string][]byte{
					"mariadb": []byte(""),
				},
			},
			cmd: []string{"mariadb",
				"--user", "myUser",
				"--database", "mydb",
				"--port", "12345",
				"--host", "localhost",
				"--protocol", "TCP",
				"--ssl-key", "/tmp/keys/example.com/bob",
				"--ssl-ca", "/tmp/keys/example.com/cas/root.pem",
				"--ssl-cert", "/tmp/keys/example.com/bob-db/db.example.com/mysql-x509.pem",
				"--ssl-verify-server-cert"},
			wantErr: false,
		},
		{
			name:       "mysql by mariadb",
			dbProtocol: defaults.ProtocolMySQL,
			execer: &fakeExec{
				execOutput: map[string][]byte{
					"mysql": []byte("mysql  Ver 15.1 Distrib 10.3.32-MariaDB, for debian-linux-gnu (x86_64) using readline 5.2"),
				},
			},
			cmd: []string{"mysql",
				"--user", "myUser",
				"--database", "mydb",
				"--port", "12345",
				"--host", "localhost",
				"--protocol", "TCP",
				"--ssl-key", "/tmp/keys/example.com/bob",
				"--ssl-ca", "/tmp/keys/example.com/cas/root.pem",
				"--ssl-cert", "/tmp/keys/example.com/bob-db/db.example.com/mysql-x509.pem",
				"--ssl-verify-server-cert"},
			wantErr: false,
		},
		{
			name:       "mysql by oracle",
			dbProtocol: defaults.ProtocolMySQL,
			execer: &fakeExec{
				execOutput: map[string][]byte{
					"mysql": []byte("Ver 8.0.27-0ubuntu0.20.04.1 for Linux on x86_64 ((Ubuntu))"),
				},
			},
			cmd: []string{"mysql",
				"--defaults-group-suffix=_db.example.com-mysql",
				"--user", "myUser",
				"--database", "mydb",
				"--port", "12345",
				"--host", "localhost",
				"--protocol", "TCP"},
			wantErr: false,
		},
		{
			name:       "no mysql nor mariadb",
			dbProtocol: defaults.ProtocolMySQL,
			execer: &fakeExec{
				execOutput: map[string][]byte{},
			},
			cmd:     []string{},
			wantErr: true,
		},
		{
			name:       "mongodb (legacy)",
			dbProtocol: defaults.ProtocolMongoDB,
			execer: &fakeExec{
				execOutput: map[string][]byte{},
			},
			cmd: []string{"mongo",
				"--host", "localhost",
				"--port", "12345",
				"--ssl",
				"--sslPEMKeyFile", "/tmp/keys/example.com/bob-db/db.example.com/mysql-x509.pem",
				"mydb"},
			wantErr: false,
		},
		{
			name:       "mongosh",
			dbProtocol: defaults.ProtocolMongoDB,
			execer: &fakeExec{
				execOutput: map[string][]byte{
					"mongosh": []byte("1.1.6"),
				},
			},
			cmd: []string{"mongosh",
				"--host", "localhost",
				"--port", "12345",
				"--tls",
				"--tlsCertificateKeyFile", "/tmp/keys/example.com/bob-db/db.example.com/mysql-x509.pem",
				"mydb"},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			database := &tlsca.RouteToDatabase{
				Protocol:    tt.dbProtocol,
				Database:    "mydb",
				Username:    "myUser",
				ServiceName: "mysql",
			}

			c := newCmdBuilder(tc, profile, database, "root", WithLocalProxy("localhost", 12345, ""))
			c.exe = tt.execer
			got, err := c.getConnectCommand()
			if tt.wantErr {
				if err == nil {
					t.Errorf("getConnectCommand() should return an error, but it didn't")
				}
				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.cmd, got.Args)
		})
	}
}
