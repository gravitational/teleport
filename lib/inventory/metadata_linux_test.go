//go:build linux
// +build linux

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

func TestFetchOSVersionInfo(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		desc     string
		readFile func(string) ([]byte, error)
		expected string
	}{
		{
			desc: "/etc/os-release content if /etc/os-release exists",
			readFile: func(name string) ([]byte, error) {
				if name != "/etc/os-release" {
					return nil, trace.NotFound("file does not exist")
				}
				return []byte("file content"), nil
			},
			expected: "file content",
		},
		{
			desc: "empty if /etc/os-release does not exist",
			readFile: func(name string) ([]byte, error) {
				return nil, trace.NotFound("file does not exist")
			},
			expected: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			c := &fetchConfig{
				readFile: tc.readFile,
			}
			require.Equal(t, tc.expected, c.fetchOSVersionInfo())
		})
	}
}

func TestFetchGlibcVersion(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		desc        string
		execCommand func(string, ...string) ([]byte, error)
		expected    string
	}{
		{
			desc: "ldd output if ldd exists",
			execCommand: func(name string, args ...string) ([]byte, error) {
				if name != "ldd" {
					return nil, trace.NotFound("command does not exist")
				}
				if len(args) != 1 {
					return nil, trace.Errorf("invalid command argument")
				}

				switch args[0] {
				case "--version":
					return []byte("command output"), nil
				default:
					return nil, trace.Errorf("invalid command argument")
				}
			},
			expected: "command output",
		},
		{
			desc: "empty if ldd does not exist",
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
			require.Equal(t, tc.expected, c.fetchGlibcVersionInfo())
		})
	}
}
