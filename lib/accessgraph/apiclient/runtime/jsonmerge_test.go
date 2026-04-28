/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package runtime

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestJSONMerge mirrors github.com/oapi-codegen/runtime's per-shape unit
// tests for JSONMerge, which is the wrapper this package replaces.
func TestJSONMerge(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		data     string
		patch    string
		expected string
	}{
		{
			name:     "merges properties defined in both objects",
			data:     `{"foo":1}`,
			patch:    `{"foo":null}`,
			expected: `{"foo":null}`,
		},
		{
			name:     "sets property defined only in patch",
			data:     `{}`,
			patch:    `{"source":"merge-me"}`,
			expected: `{"source":"merge-me"}`,
		},
		{
			name:     "merges nested objects recursively",
			data:     `{"channel":{"status":"valid"}}`,
			patch:    `{"channel":{"id":1}}`,
			expected: `{"channel":{"id":1,"status":"valid"}}`,
		},
		{
			name:     "handles empty objects",
			data:     `{}`,
			patch:    `{}`,
			expected: `{}`,
		},
		{
			name:     "handles nil data",
			data:     "",
			patch:    `{"foo":"bar"}`,
			expected: `{"foo":"bar"}`,
		},
		{
			name:     "handles nil patch",
			data:     `{"foo":"bar"}`,
			patch:    "",
			expected: `{"foo":"bar"}`,
		},
		{
			// Top-level array data is left alone when both sides are arrays;
			// the patch only takes effect when it is an object whose keys
			// are stringified indices.
			name:     "does not merge top-level arrays",
			data:     `[{"foo":1}]`,
			patch:    `[{"foo":null}]`,
			expected: `[{"foo":1}]`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var data, patch json.RawMessage
			if tc.data != "" {
				data = json.RawMessage(tc.data)
			}
			if tc.patch != "" {
				patch = json.RawMessage(tc.patch)
			}

			got, err := JSONMerge(data, patch)
			require.NoError(t, err)
			require.JSONEq(t, tc.expected, string(got))
		})
	}
}

// TestJSONMerge_ApapschCompatibility runs the upstream
// github.com/apapsch/go-jsonmerge/v2 test fixtures verbatim against our
// implementation. oapi-codegen calls that library with
// CopyNonexistent=true, which is the only mode our wrapper supports, so
// we expect to match TestMergeBytesNonexistent and TestLongNumbers
// exactly.
func TestJSONMerge_ApapschCompatibility(t *testing.T) {
	t.Parallel()

	t.Run("nonexistent fixture", func(t *testing.T) {
		t.Parallel()

		input := `
{
  "number": 1,
  "string": "value",
  "object": {
    "number": 1,
    "string": "value",
    "nested_object": {
      "number": 2
    },
    "array": [1, 2, 3],
    "partial_array": [1, 2, 3]
  }
}`
		patch := `
{
  "number": 2,
  "string": "value1",
  "nonexitent": "woot",
  "object": {
    "number": 3,
    "string": "value2",
    "nested_object": {
      "number": 4
    },
    "array": [3, 2, 1],
    "partial_array": {
      "1": 4
    }
  }
}`
		expected := `
{
  "number": 2,
  "string": "value1",
  "nonexitent": "woot",
  "object": {
    "number": 3,
    "string": "value2",
    "nested_object": {
      "number": 4
    },
    "array": [3, 2, 1],
    "partial_array": [1, 4, 3]
  }
}`

		got, err := JSONMerge(json.RawMessage(input), json.RawMessage(patch))
		require.NoError(t, err)
		require.JSONEq(t, expected, string(got))
	})

	t.Run("long numbers preserved", func(t *testing.T) {
		t.Parallel()

		got, err := JSONMerge(
			json.RawMessage(`{"Id":12423434,"Value":12423434}`),
			json.RawMessage(`{"Value":12423439}`),
		)
		require.NoError(t, err)
		require.JSONEq(t, `{"Id":12423434,"Value":12423439}`, string(got))
	})
}
