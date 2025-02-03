// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package common

import (
	"testing"

	"github.com/alecthomas/kingpin/v2"
	"github.com/stretchr/testify/require"
)

func TestLabelCLI(t *testing.T) {
	testCases := []struct {
		name           string
		labelArgs      []string
		expectedError  require.ErrorAssertionFunc
		expectedLabels Labels
	}{
		{
			name:          "empty",
			expectedError: require.NoError,
		},
		{
			name:           "simple",
			labelArgs:      []string{"key=value"},
			expectedError:  require.NoError,
			expectedLabels: Labels{"key": "value"},
		},
		{
			name:          "malformed",
			labelArgs:     []string{"key="},
			expectedError: require.Error,
		},
		{
			name:           "multiple inline",
			labelArgs:      []string{"a=alpha,b=beta"},
			expectedError:  require.NoError,
			expectedLabels: Labels{"a": "alpha", "b": "beta"},
		},
		{
			name:           "multiple args",
			labelArgs:      []string{"a=alpha,b=beta", "g=gamma,d=delta"},
			expectedError:  require.NoError,
			expectedLabels: Labels{"a": "alpha", "b": "beta", "g": "gamma", "d": "delta"},
		},
		{
			name:           "last definition wins",
			labelArgs:      []string{"a=alpha,b=beta", "a=apple,d=delta"},
			expectedError:  require.NoError,
			expectedLabels: Labels{"a": "apple", "b": "beta", "d": "delta"},
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			var actualLabels Labels
			app := kingpin.New(t.Name(), "test label parsing")
			app.Flag("label", "???").
				SetValue(&actualLabels)

			var args []string
			for _, arg := range test.labelArgs {
				args = append(args, "--label", arg)
			}

			_, err := app.Parse(args)
			test.expectedError(t, err)
			require.Equal(t, test.expectedLabels, actualLabels)
		})
	}
}
