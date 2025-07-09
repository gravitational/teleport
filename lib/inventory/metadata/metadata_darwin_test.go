//go:build darwin
// +build darwin

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
