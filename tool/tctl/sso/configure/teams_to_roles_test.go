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

package configure

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func Test_teamsToRolesParser_Set(t *testing.T) {
	tests := []struct {
		name       string
		parser     teamsToRolesParser
		arg        string
		wantErr    bool
		wantParser teamsToRolesParser
	}{
		{
			name:   "one set of correct args",
			parser: teamsToRolesParser{mappings: new([]types.TeamRolesMapping)},
			arg:    "foo,bar,baz",
			wantParser: teamsToRolesParser{mappings: &[]types.TeamRolesMapping{
				{
					Organization: "foo",
					Team:         "bar",
					Roles:        []string{"baz"},
				},
			}},
			wantErr: false,
		},
		{
			name: "two sets of correct args",
			parser: teamsToRolesParser{mappings: &[]types.TeamRolesMapping{
				{
					Organization: "foo",
					Team:         "bar",
					Roles:        []string{"baz"},
				},
			}},
			arg: "aaa,bbb,ccc,ddd",
			wantParser: teamsToRolesParser{mappings: &[]types.TeamRolesMapping{
				{
					Organization: "foo",
					Team:         "bar",
					Roles:        []string{"baz"},
				},
				{
					Organization: "aaa",
					Team:         "bbb",
					Roles:        []string{"ccc", "ddd"},
				},
			}},
			wantErr: false,
		},
		{
			name:       "one set of incorrect args",
			parser:     teamsToRolesParser{mappings: new([]types.TeamRolesMapping)},
			arg:        "abracadabra",
			wantParser: teamsToRolesParser{mappings: new([]types.TeamRolesMapping)},
			wantErr:    true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.parser.Set(tt.arg)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.wantParser, tt.parser)
			}
		})
	}
}
