// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package db_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/defaults"
	dbvnet "github.com/gravitational/teleport/lib/srv/db/vnet"
	"github.com/gravitational/teleport/lib/vnet/db"
)

func TestParse(t *testing.T) {
	myPostgresHash := dbvnet.DNSName("my-postgres")
	myDBHash := dbvnet.DNSName("my-db")

	tests := []struct {
		name           string
		fqdn           string
		zone           string
		wantIdentifier string
		wantOK         bool
	}{
		{
			name:           "vnet_dns_name (hash) prefix",
			fqdn:           myPostgresHash + ".db.proxy.example.com.",
			zone:           "proxy.example.com",
			wantIdentifier: myPostgresHash,
			wantOK:         true,
		},
		{
			name:           "different db hash",
			fqdn:           myDBHash + ".db.proxy.example.com.",
			zone:           "proxy.example.com",
			wantIdentifier: myDBHash,
			wantOK:         true,
		},
		{
			name:           "literal database name is also accepted",
			fqdn:           "my-postgres.db.proxy.example.com.",
			zone:           "proxy.example.com",
			wantIdentifier: "my-postgres",
			wantOK:         true,
		},
		{
			name:           "zone already fully qualified",
			fqdn:           myPostgresHash + ".db.proxy.example.com.",
			zone:           "proxy.example.com.",
			wantIdentifier: myPostgresHash,
			wantOK:         true,
		},
		{
			name:   "no db infix",
			fqdn:   "app.proxy.example.com.",
			zone:   "proxy.example.com",
			wantOK: false,
		},
		{
			name:   "wrong zone",
			fqdn:   myPostgresHash + ".db.other.example.com.",
			zone:   "proxy.example.com",
			wantOK: false,
		},
		{
			name:   "empty prefix",
			fqdn:   ".db.proxy.example.com.",
			zone:   "proxy.example.com",
			wantOK: false,
		},
		{
			name:   "only db infix and zone",
			fqdn:   "db.proxy.example.com.",
			zone:   "proxy.example.com",
			wantOK: false,
		},
		{
			name:   "extra dotted prefix is rejected (no embedded user prefix)",
			fqdn:   "reader." + myPostgresHash + ".db.proxy.example.com.",
			zone:   "proxy.example.com",
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := db.Parse(tt.fqdn, tt.zone)
			require.Equal(t, tt.wantOK, ok)
			if !tt.wantOK {
				return
			}
			require.Equal(t, tt.wantIdentifier, got)
		})
	}
}

func TestIsUserOptional(t *testing.T) {
	supported := []string{
		defaults.ProtocolPostgres,
		defaults.ProtocolCockroachDB,
		defaults.ProtocolMySQL,
		defaults.ProtocolSQLServer,
	}
	for _, p := range supported {
		t.Run(p, func(t *testing.T) {
			require.True(t, db.IsUserOptional(p))
		})
	}

	unsupported := []string{
		defaults.ProtocolMongoDB,
		defaults.ProtocolClickHouse,
	}
	for _, p := range unsupported {
		t.Run(p+"_user_required", func(t *testing.T) {
			require.False(t, db.IsUserOptional(p))
		})
	}
}
