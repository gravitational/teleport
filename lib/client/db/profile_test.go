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
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/client/db/profile"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/observability/tracing"
	"github.com/gravitational/teleport/lib/tlsca"
)

// TestAddProfile verifies that connection profile is populated with correct
// proxy address.
func TestAddProfile(t *testing.T) {
	tests := []struct {
		desc                string
		webProxyAddrIn      string
		postgresProxyAddrIn string
		mysqlProxyAddrIn    string
		protocolIn          string
		profileHostOut      string
		profilePortOut      int
	}{
		{
			desc:           "postgres - web proxy host port",
			webProxyAddrIn: "web.example.com:443",
			protocolIn:     defaults.ProtocolPostgres,
			profileHostOut: "web.example.com",
			profilePortOut: 443,
		},
		{
			desc:                "postgres - custom host",
			webProxyAddrIn:      "web.example.com:443",
			postgresProxyAddrIn: "postgres.example.com",
			protocolIn:          defaults.ProtocolPostgres,
			profileHostOut:      "postgres.example.com",
			profilePortOut:      443,
		},
		{
			desc:                "postgres - custom host port",
			webProxyAddrIn:      "web.example.com:443",
			postgresProxyAddrIn: "postgres.example.com:5432",
			protocolIn:          defaults.ProtocolPostgres,
			profileHostOut:      "postgres.example.com",
			profilePortOut:      5432,
		},
		{
			desc:           "mysql - web proxy host, default port",
			webProxyAddrIn: "web.example.com:443",
			protocolIn:     defaults.ProtocolMySQL,
			profileHostOut: "web.example.com",
			profilePortOut: defaults.MySQLListenPort,
		},
		{
			desc:             "mysql - custom host",
			webProxyAddrIn:   "web.example.com:443",
			mysqlProxyAddrIn: "mysql.example.com",
			protocolIn:       defaults.ProtocolMySQL,
			profileHostOut:   "mysql.example.com",
			profilePortOut:   defaults.MySQLListenPort,
		},
		{
			desc:             "mysql - custom host port",
			webProxyAddrIn:   "web.example.com:443",
			mysqlProxyAddrIn: "mysql.example.com:3336",
			protocolIn:       defaults.ProtocolMySQL,
			profileHostOut:   "mysql.example.com",
			profilePortOut:   3336,
		},
	}
	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			tc := &client.TeleportClient{
				Config: client.Config{
					SiteName:          "example.com",
					WebProxyAddr:      test.webProxyAddrIn,
					PostgresProxyAddr: test.postgresProxyAddrIn,
					MySQLProxyAddr:    test.mysqlProxyAddrIn,
					Tracer:            tracing.NoopProvider().Tracer("test"),
				},
			}
			db := tlsca.RouteToDatabase{
				ServiceName: "example",
				Protocol:    test.protocolIn,
			}
			ps := client.ProfileStatus{
				Dir:  t.TempDir(),
				Name: "alice",
			}
			actual, err := add(tc, db, ps, &testProfileFile{profiles: make(map[string]profile.ConnectProfile)}, "root-cluster")
			require.NoError(t, err)
			require.EqualValues(t, &profile.ConnectProfile{
				Name:       profileName(tc.SiteName, db.ServiceName),
				Host:       test.profileHostOut,
				Port:       test.profilePortOut,
				CACertPath: ps.CACertPathForCluster("root-cluster"),
				CertPath:   ps.DatabaseCertPathForCluster(tc.SiteName, db.ServiceName),
				KeyPath:    ps.DatabaseKeyPathForCluster(tc.SiteName, db.ServiceName),
			}, actual)
		})
	}
}

// testProfileFile is the test implementation of connection profile file.
type testProfileFile struct {
	profiles map[string]profile.ConnectProfile
}

// Upsert saves the provided connection profile.
func (p *testProfileFile) Upsert(profile profile.ConnectProfile) error {
	p.profiles[profile.Name] = profile
	return nil
}

// Env returns the specified connection profile as environment variables.
func (p *testProfileFile) Env(name string) (map[string]string, error) {
	return nil, trace.NotImplemented("not implemented")
}

// Delete removes the specified connection profile.
func (p *testProfileFile) Delete(name string) error {
	delete(p.profiles, name)
	return nil
}
