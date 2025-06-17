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

package scripts

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils"
)

func TestMarshalLabelsYAML(t *testing.T) {
	for _, tt := range []struct {
		name           string
		labels         types.Labels
		numExtraIndent int
		expected       []string
	}{
		{
			name:     "empty",
			labels:   types.Labels{},
			expected: []string{"{}"},
		},
		{
			name: "wildcard to wildcard",
			labels: types.Labels{
				types.Wildcard: utils.Strings{types.Wildcard},
			},
			expected: []string{`'*': '*'`},
		},
		{
			name: "multiple labels",
			labels: types.Labels{
				"dev":     utils.Strings{types.Wildcard},
				"product": utils.Strings{"scripts"},
			},
			expected: []string{`dev: '*'`, `product: scripts`},
		},
		{
			name: "multiple label values",
			labels: types.Labels{
				"dev":     utils.Strings{types.Wildcard},
				"env":     utils.Strings{"dev1", "dev2"},
				"product": utils.Strings{"scripts"},
			},
			expected:       []string{"dev: '*'", "env:\n      - dev1\n      - dev2", "product: scripts"},
			numExtraIndent: 2,
		},
	} {
		got, err := marshalLabelsYAML(tt.labels, tt.numExtraIndent)
		require.NoError(t, err)

		require.YAMLEq(t, strings.Join(tt.expected, "\n"), strings.Join(got, "\n"))
	}
}
