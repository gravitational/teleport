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

package loginrule_test

import (
	"encoding/json"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/e/lib/loginrule"
)

func TestJSONPath(t *testing.T) {
	t.Parallel()

	errCheckIsBadParameter := func(tt require.TestingT, err error, _ ...any) {
		require.True(tt, trace.IsBadParameter(err), "expected bad parameter error, got %v", err)
	}
	type result struct {
		values   []string
		errCheck require.ErrorAssertionFunc
	}
	var tests = []struct {
		title string
		path  string
		json  string
		res   result
	}{
		{
			title: "jsonpath string",
			path:  "$.claim_name",
			json:  `{"claim_name": "normal_claim"}`,
			res:   result{values: []string{"normal_claim"}},
		},
		{
			title: "jsonpath strings",
			path:  "$.claim_name.*",
			json:  `{"claim_name": ["normal", "claims"]}`,
			res:   result{values: []string{"normal", "claims"}},
		},
		{
			title: "jsonpath object selector",
			path:  "$.claim_name.azure",
			json:  `{"claim_name":{"azure":"teleport_access","aws":"teleport_admin"}}`,
			res:   result{values: []string{"teleport_access"}},
		},
		{
			title: "jsonpath object wildcard flatten elements",
			path:  "$.claim_name.*",
			json:  `{"claim_name":{"azure":"teleport_access","aws":"teleport_admin"}}`,
			res:   result{values: []string{"teleport_admin", "teleport_access"}},
		},
		{
			title: "jsonpath array selector",
			path:  "$.claim_name[1].aws",
			json:  `{"claim_name":[{"azure":"teleport_access"},{"aws":"teleport_admin"}]}`,
			res:   result{values: []string{"teleport_admin"}},
		},
		{
			title: "jsonpath array wildcard flatten elements",
			path:  "$.claim_name.*.*",
			json:  `{"claim_name":[{"azure":"teleport_access"},{"aws":"teleport_admin"}]}`,
			res:   result{values: []string{"teleport_access", "teleport_admin"}},
		},
		{
			title: "jsonpath filter",
			path:  "$.claim_name[?(@.teleport == 'admin')].*",
			json:  `{"claim_name":{"azure":{"teleport":"access"},"aws":{"teleport":"admin"}}}`,
			res:   result{values: []string{"admin"}},
		},
		{
			title: "jsonpath regexp",
			path:  "$.claim_name[?(@ =~ 'teleport_.*')]",
			json:  `{"claim_name":{"azure":"other_access","aws":"teleport_admin"}}`,
			res:   result{values: []string{"teleport_admin"}},
		},
		{
			title: "jsonpath empty result",
			path:  "$.missing_claim",
			json:  `{"claim_name":[{"azure":"teleport_access"},{"aws":"teleport_admin"}]}`,
			res:   result{values: []string{}},
		},
		{
			title: "jsonpath parsing error",
			path:  "$.claim_name.=azure]",
			json:  `{"claim_name":{"azure":"teleport_access","aws":"teleport_admin"}}`,
			res:   result{errCheck: require.Error},
		},
		{
			title: "jsonpath interpolation to non string list error",
			path:  "$.claim_name",
			json:  `{"claim_name":{"azure":"teleport_access","aws":"teleport_admin"}}`,
			res:   result{errCheck: errCheckIsBadParameter},
		},
		{
			title: "jsonpath interpolation to list with non string error",
			path:  "$.claim_name",
			json:  `{"claim_name":["1",2,["3","4"]]}`,
			res:   result{errCheck: errCheckIsBadParameter},
		},
	}

	for _, tt := range tests {
		t.Run(tt.title, func(t *testing.T) {
			var input map[string]any
			err := json.Unmarshal([]byte(tt.json), &input)
			require.NoError(t, err)

			values, err := loginrule.JSONPath(input, tt.path)
			if tt.res.errCheck != nil {
				tt.res.errCheck(t, err)
				require.Empty(t, values)
				return
			}
			require.NoError(t, err)
			require.ElementsMatch(t, tt.res.values, values)
		})
	}
}
