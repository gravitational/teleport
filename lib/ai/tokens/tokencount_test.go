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

package tokens

import (
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	testCompletionStart = "This is the beginning of the response."
	testCompletionEnd   = "And this is the end."
)

// This test checks that Add() properly appends content in the completion
// response.
func TestAsynchronousTokenCounter_TokenCount(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name            string
		completionStart string
		completionEnd   string
		expectedTokens  int
	}{
		{
			name:           "empty count",
			expectedTokens: 3,
		},
		{
			name:            "only completion start",
			completionStart: testCompletionStart,
			expectedTokens:  12,
		},
		{
			name:           "only completion add",
			completionEnd:  testCompletionEnd,
			expectedTokens: 8,
		},
		{
			name:            "completion start and end",
			completionStart: testCompletionStart,
			completionEnd:   testCompletionEnd,
			expectedTokens:  17,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// Test setup
			tc, err := NewAsynchronousTokenCounter(tt.completionStart)
			require.NoError(t, err)
			tokens := countTokens(tt.completionEnd)

			for range tokens {
				require.NoError(t, tc.Add())
			}

			// Doing the real test: asserting the count is right
			count := tc.TokenCount()
			require.Equal(t, tt.expectedTokens, count)
		})
	}
}

func TestAsynchronousTokenCounter_Finished(t *testing.T) {
	tc, err := NewAsynchronousTokenCounter(testCompletionStart)
	require.NoError(t, err)

	// We can Add() if the counter has not been read yet
	require.NoError(t, tc.Add())

	// We read from the counter
	count := tc.TokenCount()
	require.Equal(t, 13, count)

	// Adding new tokens should be impossible
	require.Error(t, tc.Add())
}
