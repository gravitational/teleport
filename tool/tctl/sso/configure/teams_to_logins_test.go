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

func Test_teamsToLoginsParser_Set(t *testing.T) {
	tests := []struct {
		name       string
		parser     teamsToLoginsParser
		arg        string
		wantErr    bool
		wantParser teamsToLoginsParser
	}{
		{
			name:   "one set of correct args",
			parser: teamsToLoginsParser{mappings: new([]types.TeamMapping)},
			arg:    "foo,bar,baz",
			wantParser: teamsToLoginsParser{mappings: &[]types.TeamMapping{
				{
					Organization: "foo",
					Team:         "bar",
					Logins:       []string{"baz"},
				},
			}},
			wantErr: false,
		},
		{
			name: "two sets of correct args",
			parser: teamsToLoginsParser{mappings: &[]types.TeamMapping{
				{
					Organization: "foo",
					Team:         "bar",
					Logins:       []string{"baz"},
				},
			}},
			arg: "aaa,bbb,ccc,ddd",
			wantParser: teamsToLoginsParser{mappings: &[]types.TeamMapping{
				{
					Organization: "foo",
					Team:         "bar",
					Logins:       []string{"baz"},
				},
				{
					Organization: "aaa",
					Team:         "bbb",
					Logins:       []string{"ccc", "ddd"},
				},
			}},
			wantErr: false,
		},
		{
			name:       "one set of incorrect args",
			parser:     teamsToLoginsParser{mappings: new([]types.TeamMapping)},
			arg:        "abracadabra",
			wantParser: teamsToLoginsParser{mappings: new([]types.TeamMapping)},
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
