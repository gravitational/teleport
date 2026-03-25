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

package jira

import (
	"encoding/json"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/require"
)

func TestErrorResultUnmarshal(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name           string
		input          string
		expectedOutput ErrorResult
		assertErr      require.ErrorAssertionFunc
	}{
		{
			name:  "new error",
			input: `{"errorMessages":[], "errors": {"project": "project is required"}}`,
			expectedOutput: ErrorResult{
				Details: ErrorDetails{Errors: map[string]string{"project": "project is required"}},
			},
			assertErr: require.NoError,
		},
		{
			name:  "legacy error",
			input: `{"errorMessages":["foo"],"errors":["bar", "baz"]}`,
			expectedOutput: ErrorResult{
				ErrorMessages: []string{"foo"},
				Details:       ErrorDetails{LegacyErrors: []string{"bar", "baz"}},
			},
			assertErr: require.NoError,
		},
		{
			name:  "empty error",
			input: `{"errorMessages":["There was an error parsing JSON. Check that your request body is valid."]}`,
			expectedOutput: ErrorResult{
				ErrorMessages: []string{"There was an error parsing JSON. Check that your request body is valid."},
			},
			assertErr: require.NoError,
		},
		{
			name:           "malformed error",
			input:          `{"errorMessages":["Foo"],"errors":"This is a single string"}`,
			expectedOutput: ErrorResult{ErrorMessages: []string{"Foo"}},
			assertErr: func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "This is a single string")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt := tt
			t.Parallel()
			data := []byte(tt.input)
			var result ErrorResult
			tt.assertErr(t, json.Unmarshal(data, &result))
			diff := cmp.Diff(tt.expectedOutput, result, cmpopts.EquateEmpty())
			require.Empty(t, diff)
		})
	}
}
