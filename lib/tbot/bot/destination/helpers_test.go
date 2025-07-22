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

package destination

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/gravitational/teleport/lib/utils/testutils/golden"
)

type testYAMLCase[T any] struct {
	name string
	in   T
}

func testYAML[T any](t *testing.T, tests []testYAMLCase[T]) {
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := bytes.NewBuffer(nil)
			encoder := yaml.NewEncoder(b)
			encoder.SetIndent(2)
			require.NoError(t, encoder.Encode(&tt.in))

			if golden.ShouldSet() {
				golden.Set(t, b.Bytes())
			}
			require.Equal(
				t,
				string(golden.Get(t)),
				b.String(),
				"results of marshal did not match golden file, rerun tests with GOLDEN_UPDATE=1",
			)

			// Now test unmarshalling to see if we get the same object back
			decoder := yaml.NewDecoder(b)
			var unmarshalled T
			require.NoError(t, decoder.Decode(&unmarshalled))
			require.Equal(t, tt.in, unmarshalled, "unmarshalling did not result in same object as input")
		})
	}
}
