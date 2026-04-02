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

package vnet

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseDatabaseFQDN(t *testing.T) {
	tests := []struct {
		name       string
		fqdn       string
		zone       string
		wantUser   string
		wantDBName string
		wantErr    error
	}{
		{
			name:       "user and db name",
			fqdn:       "reader.my-postgres.db.proxy.example.com.",
			zone:       "proxy.example.com",
			wantUser:   "reader",
			wantDBName: "my-postgres",
		},
		{
			name:       "db name only (no user)",
			fqdn:       "my-postgres.db.proxy.example.com.",
			zone:       "proxy.example.com",
			wantUser:   "",
			wantDBName: "my-postgres",
		},
		{
			name:       "dotted username",
			fqdn:       "some.dotted.user.my-db.db.proxy.example.com.",
			zone:       "proxy.example.com",
			wantUser:   "some.dotted.user",
			wantDBName: "my-db",
		},
		{
			name:       "zone already fully qualified",
			fqdn:       "reader.my-postgres.db.proxy.example.com.",
			zone:       "proxy.example.com.",
			wantUser:   "reader",
			wantDBName: "my-postgres",
		},
		{
			name:    "no db infix",
			fqdn:    "app.proxy.example.com.",
			zone:    "proxy.example.com",
			wantErr: errNoMatch,
		},
		{
			name:    "wrong zone",
			fqdn:    "reader.my-postgres.db.other.example.com.",
			zone:    "proxy.example.com",
			wantErr: errNoMatch,
		},
		{
			name:    "empty prefix",
			fqdn:    ".db.proxy.example.com.",
			zone:    "proxy.example.com",
			wantErr: errNoMatch,
		},
		{
			name:    "only db infix and zone",
			fqdn:    "db.proxy.example.com.",
			zone:    "proxy.example.com",
			wantErr: errNoMatch,
		},
		{
			name:    "trailing dot user only (empty db name)",
			fqdn:    "reader..db.proxy.example.com.",
			zone:    "proxy.example.com",
			wantErr: errNoMatch,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotUser, gotDBName, err := parseDatabaseFQDN(tt.fqdn, tt.zone)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.wantUser, gotUser)
			require.Equal(t, tt.wantDBName, gotDBName)
		})
	}
}
