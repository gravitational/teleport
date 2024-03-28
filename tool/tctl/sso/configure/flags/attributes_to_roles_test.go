// Teleport
// Copyright (C) 2024 Gravitational, Inc.
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

package flags

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func Test_attributesToRolesParser_Set(t *testing.T) {
	tests := []struct {
		name       string
		parser     attributesToRolesParser
		arg        string
		wantErr    bool
		wantParser attributesToRolesParser
	}{
		{
			name:   "one set of correct args",
			parser: attributesToRolesParser{mappings: &[]types.AttributeMapping{}},
			arg:    "foo,bar,baz",
			wantParser: attributesToRolesParser{mappings: &[]types.AttributeMapping{
				{
					Name:  "foo",
					Value: "bar",
					Roles: []string{"baz"},
				}}},
			wantErr: false,
		},
		{
			name: "two sets of correct args",
			parser: attributesToRolesParser{mappings: &[]types.AttributeMapping{
				{
					Name:  "foo",
					Value: "bar",
					Roles: []string{"baz"},
				}}},
			arg: "aaa,bbb,ccc,ddd",
			wantParser: attributesToRolesParser{mappings: &[]types.AttributeMapping{
				{
					Name:  "foo",
					Value: "bar",
					Roles: []string{"baz"},
				},
				{
					Name:  "aaa",
					Value: "bbb",
					Roles: []string{"ccc", "ddd"},
				}}},
			wantErr: false,
		},
		{
			name:       "one set of incorrect args",
			parser:     attributesToRolesParser{mappings: &[]types.AttributeMapping{}},
			arg:        "abracadabra",
			wantParser: attributesToRolesParser{mappings: &[]types.AttributeMapping{}},
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
