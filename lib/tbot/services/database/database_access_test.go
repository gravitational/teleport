/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package database

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func TestChooseOneDatabase(t *testing.T) {
	t.Parallel()
	fooDB1 := newMockDiscoveredDB(t, "foo-rds-us-west-1-123456789012", "foo")
	fooDB2 := newMockDiscoveredDB(t, "foo-rds-us-west-2-123456789012", "foo")
	barDB := newMockDiscoveredDB(t, "bar-rds-us-west-1-123456789012", "bar")
	tests := []struct {
		desc      string
		databases []types.Database
		dbSvc     string
		wantDB    types.Database
		wantErr   string
	}{
		{
			desc:      "by exact name match",
			databases: []types.Database{fooDB1, fooDB2, barDB},
			dbSvc:     "bar-rds-us-west-1-123456789012",
			wantDB:    barDB,
		},
		{
			desc:      "by unambiguous discovered name match",
			databases: []types.Database{fooDB1, fooDB2, barDB},
			dbSvc:     "bar",
			wantDB:    barDB,
		},
		{
			desc:      "ambiguous discovered name matches is an error",
			databases: []types.Database{fooDB1, fooDB2, barDB},
			dbSvc:     "foo",
			wantErr:   `"foo" matches multiple auto-discovered databases`,
		},
		{
			desc:      "no match is an error",
			databases: []types.Database{fooDB1, fooDB2, barDB},
			dbSvc:     "xxx",
			wantErr:   `database "xxx" not found`,
		},
	}
	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			gotDB, err := chooseOneDatabase(test.databases, test.dbSvc)
			if test.wantErr != "" {
				require.ErrorContains(t, err, test.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, test.wantDB, gotDB)
		})
	}
}
