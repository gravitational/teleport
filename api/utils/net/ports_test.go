// Copyright 2024 Gravitational, Inc.
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

package net

import (
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

func TestValidatePortRange(t *testing.T) {
	tests := []struct {
		name    string
		port    int
		endPort int
		check   require.ErrorAssertionFunc
	}{
		{
			name:    "valid single port",
			port:    1337,
			endPort: 0,
			check:   require.NoError,
		},
		{
			name:    "valid port range",
			port:    1337,
			endPort: 3456,
			check:   require.NoError,
		},
		{
			name:    "port smaller than 1",
			port:    0,
			endPort: 0,
			check:   badParameterError,
		},
		{
			name:    "port bigger than max port",
			port:    98765,
			endPort: 0,
			check:   badParameterError,
		},
		{
			name:    "end port smaller than 2",
			port:    5,
			endPort: 1,
			check:   badParameterErrorAndContains("end port must be between 6 and 65535"),
		},
		{
			name:    "end port bigger than max port",
			port:    5,
			endPort: 98765,
			check:   badParameterErrorAndContains("end port must be between 6 and 65535"),
		},
		{
			name:    "end port smaller than port",
			port:    10,
			endPort: 5,
			check:   badParameterErrorAndContains("end port must be between 11 and 65535"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.check(t, ValidatePortRange(tt.port, tt.endPort))
		})
	}
}

func TestIsPortInRange(t *testing.T) {
	tests := []struct {
		name       string
		port       int
		endPort    int
		targetPort int
		check      require.BoolAssertionFunc
	}{
		{
			name:       "within single port range",
			port:       1337,
			endPort:    0,
			targetPort: 1337,
			check:      require.True,
		},
		{
			name:       "within port range",
			port:       1337,
			endPort:    3456,
			targetPort: 2550,
			check:      require.True,
		},
		{
			name:       "equal to range start",
			port:       1337,
			endPort:    3456,
			targetPort: 1337,
			check:      require.True,
		},
		{
			name:       "equal to range end",
			port:       1337,
			endPort:    3456,
			targetPort: 3456,
			check:      require.True,
		},
		{
			name:       "outside of single port range",
			port:       1337,
			endPort:    0,
			targetPort: 7331,
			check:      require.False,
		},
		{
			name:       "equal to end of single port range",
			port:       1337,
			endPort:    0,
			targetPort: 0,
			check:      require.False,
		},
		{
			name:       "smaller than range start",
			port:       1337,
			endPort:    3456,
			targetPort: 42,
			check:      require.False,
		},
		{
			name:       "bigger than range end",
			port:       1337,
			endPort:    3456,
			targetPort: 7331,
			check:      require.False,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.check(t, IsPortInRange(tt.port, tt.endPort, tt.targetPort), "compared %d against %d-%d", tt.targetPort, tt.port, tt.endPort)
		})
	}
}

func badParameterError(t require.TestingT, err error, msgAndArgs ...interface{}) {
	require.True(t, trace.IsBadParameter(err), "expected bad parameter error, got %+v", err)
}

func badParameterErrorAndContains(msg string) require.ErrorAssertionFunc {
	return func(t require.TestingT, err error, msgAndArgs ...interface{}) {
		require.True(t, trace.IsBadParameter(err), "expected bad parameter error, got %+v", err)
		require.ErrorContains(t, err, msg, msgAndArgs...)
	}
}
