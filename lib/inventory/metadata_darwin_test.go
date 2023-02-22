//go:build darwin
// +build darwin

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

package inventory

import (
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

func TestFetchOSVersion(t *testing.T) {
	t.Parallel()

	expectedFormat := `
ProductName:            macOS
ProductVersion:         13.2.1
BuildVersion:           22D68
`

	unexpectedFormat := `
Productname:            macOS
ProductVersion:         13.2.1
BuildVersion:           22D68
`

	testCases := []struct {
		desc        string
		execCommand func(string) ([]byte, error)
		expected    string
	}{
		{
			desc: "set correctly if expected format",
			execCommand: func(name string) ([]byte, error) {
				if name != "sw_vers" {
					return nil, trace.NotFound("command does not exist")
				}
				return []byte(expectedFormat), nil
			},
			expected: "macOS 13.2.1",
		},
		{
			desc: "full output if unexpected format",
			execCommand: func(name string) ([]byte, error) {
				if name != "sw_vers" {
					return nil, trace.NotFound("command does not exist")
				}
				return []byte(unexpectedFormat), nil
			},
			expected: sanitize(unexpectedFormat),
		},
		{
			desc: "empty if sw_vers does not exist",
			execCommand: func(name string) ([]byte, error) {
				return nil, trace.NotFound("command does not exist")
			},
			expected: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			c := &fetchConfig{
				execCommand: tc.execCommand,
			}
			require.Equal(t, tc.expected, c.fetchOSVersion())
		})
	}
}
