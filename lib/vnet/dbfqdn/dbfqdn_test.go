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

package dbfqdn_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/defaults"
	dbvnet "github.com/gravitational/teleport/lib/srv/db/vnet"
	"github.com/gravitational/teleport/lib/vnet/dbfqdn"
)

func TestParse(t *testing.T) {
	myPostgres := dbvnet.DNSName("my-postgres")
	myDB := dbvnet.DNSName("my-db")

	tests := []struct {
		name            string
		fqdn            string
		zone            string
		wantVNetDNSName string
		wantErr         error
	}{
		{
			name:            "vnet_dns_name only",
			fqdn:            myPostgres + ".db.proxy.example.com.",
			zone:            "proxy.example.com",
			wantVNetDNSName: myPostgres,
		},
		{
			name:            "different db hash",
			fqdn:            myDB + ".db.proxy.example.com.",
			zone:            "proxy.example.com",
			wantVNetDNSName: myDB,
		},
		{
			name:            "zone already fully qualified",
			fqdn:            myPostgres + ".db.proxy.example.com.",
			zone:            "proxy.example.com.",
			wantVNetDNSName: myPostgres,
		},
		{
			name:    "no db infix",
			fqdn:    "app.proxy.example.com.",
			zone:    "proxy.example.com",
			wantErr: dbfqdn.ErrNoMatch,
		},
		{
			name:    "wrong zone",
			fqdn:    myPostgres + ".db.other.example.com.",
			zone:    "proxy.example.com",
			wantErr: dbfqdn.ErrNoMatch,
		},
		{
			name:    "empty prefix",
			fqdn:    ".db.proxy.example.com.",
			zone:    "proxy.example.com",
			wantErr: dbfqdn.ErrNoMatch,
		},
		{
			name:    "only db infix and zone",
			fqdn:    "db.proxy.example.com.",
			zone:    "proxy.example.com",
			wantErr: dbfqdn.ErrNoMatch,
		},
		{
			name:    "literal db name (not a hash) is rejected",
			fqdn:    "my-postgres.db.proxy.example.com.",
			zone:    "proxy.example.com",
			wantErr: dbfqdn.ErrNoMatch,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := dbfqdn.Parse(tt.fqdn, tt.zone)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.wantVNetDNSName, got)
		})
	}
}

func TestIsSupportedProtocol(t *testing.T) {
	supported := []string{
		defaults.ProtocolPostgres,
		defaults.ProtocolCockroachDB,
		defaults.ProtocolMySQL,
		defaults.ProtocolSQLServer,
	}
	for _, p := range supported {
		t.Run(p, func(t *testing.T) {
			require.True(t, dbfqdn.IsSupportedProtocol(p))
		})
	}

	unsupported := []string{
		defaults.ProtocolMongoDB,
		defaults.ProtocolClickHouse,
	}
	for _, p := range unsupported {
		t.Run(p+"_unsupported", func(t *testing.T) {
			require.False(t, dbfqdn.IsSupportedProtocol(p))
		})
	}
}
