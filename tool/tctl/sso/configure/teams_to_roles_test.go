// Copyright 2022 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
				require.Equal(t, tt.parser, tt.wantParser)
			}
		})
	}
}
