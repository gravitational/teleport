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

package metadata

import (
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

func TestFetchOSVersion(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		desc        string
		execCommand func(string, ...string) ([]byte, error)
		expected    string
	}{
		{
			desc: "combined product name and version if sw_vers exists",
			execCommand: func(name string, args ...string) ([]byte, error) {
				if name != "sw_vers" {
					return nil, trace.NotFound("command does not exist")
				}
				if len(args) != 1 {
					return nil, trace.Errorf("invalid command argument")
				}

				switch args[0] {
				case "-productName":
					return []byte("macOS"), nil
				case "-productVersion":
					return []byte("13.2.1"), nil
				default:
					return nil, trace.Errorf("invalid command argument")
				}
			},
			expected: "macOS 13.2.1",
		},
		{
			desc: "empty if sw_vers does not exist",
			execCommand: func(name string, args ...string) ([]byte, error) {
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
