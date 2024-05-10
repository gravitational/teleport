/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
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

			for i := 0; i < tokens; i++ {
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
