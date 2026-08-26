/*
Copyright 2022 Gravitational, Inc.

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

package types

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type testCase struct {
	name          string
	stringValue   string
	expectedValue Duration
}

// TestDurationUnmarshal tests unmarshaling of various duration formats.
func TestDurationUnmarshal(t *testing.T) {
	testCases := []testCase{
		{
			name:          "simple",
			stringValue:   `"100h"`,
			expectedValue: Duration(time.Hour * 100),
		},
		{
			name:          "combined large",
			stringValue:   `"1y6mo"`,
			expectedValue: Duration(time.Hour*24*365 + time.Hour*24*30*6),
		},
		{
			name:          "large + small",
			stringValue:   `"24d30μs"`,
			expectedValue: Duration(time.Hour*24*24 + time.Microsecond*30),
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			var duration Duration
			err := duration.UnmarshalJSON([]byte(testCase.stringValue))
			require.NoError(t, err)
			require.Equal(t, testCase.expectedValue, duration)
		})
	}
}
