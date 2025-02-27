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

package common

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsValidLabelKey(t *testing.T) {
	tests := []struct {
		name     string
		label    string
		expected bool
	}{
		{
			name:     "alphanumeric key",
			label:    "labelLABEL1234",
			expected: true,
		},
		{
			name:     "dashes and underscores",
			label:    "label-LABEL12__34",
			expected: true,
		},
		{
			name:     "periods and colons",
			label:    "label.:-LABEL12__34",
			expected: true,
		},
		{
			name:     "forward slashes and stars",
			label:    "label/.:-LABEL12__34*",
			expected: true,
		},
		{
			name:     "carot",
			label:    "label^/.:-LABEL12__34*",
			expected: false,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, test.expected, IsValidLabelKey(test.label))
		})
	}
}

func BenchmarkIsValidLabelKey(b *testing.B) {
	var labelsForBenchmark = []string{
		"labelLABEL1234",
		"label-LABEL12__34",
		"label.:-LABEL12__34",
		"label/.:-LABEL12__34*",
		"label^/.:-LABEL12__34*",
	}

	for i := 0; i < b.N; i++ {
		for _, s := range labelsForBenchmark {
			IsValidLabelKey(s)
		}
	}
}
