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

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
)

func TestFetchServices(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		desc          string
		helloServices []types.SystemRole
		expected      []string
	}{
		{
			desc: "node if types.RoleNode",
			helloServices: []types.SystemRole{
				types.RoleNode,
			},
			expected: []string{
				nodeService,
			},
		},
		{
			desc: "kube if types.RoleKube",
			helloServices: []types.SystemRole{
				types.RoleKube,
			},
			expected: []string{
				kubeService,
			},
		},
		{
			desc: "app if types.RoleApp",
			helloServices: []types.SystemRole{
				types.RoleApp,
			},
			expected: []string{
				appService,
			},
		},
		{
			desc: "db if types.RoleDatabase",
			helloServices: []types.SystemRole{
				types.RoleDatabase,
			},
			expected: []string{
				dbService,
			},
		},
		{
			desc: "windows_desktop if types.RoleWindowsDesktop",
			helloServices: []types.SystemRole{
				types.RoleWindowsDesktop,
			},
			expected: []string{
				windowsDesktopService,
			},
		},
		{
			desc:          "nil if none",
			helloServices: []types.SystemRole{},
			expected:      nil,
		},
		{
			desc: "nil if types.RoleProxy",
			helloServices: []types.SystemRole{
				types.RoleProxy,
			},
			expected: nil,
		},
		{
			desc: "db and app if types.RoleDatabase, types.RoleProxy and types.RoleApp",
			helloServices: []types.SystemRole{
				types.RoleDatabase,
				types.RoleProxy,
				types.RoleApp,
			},
			expected: []string{
				dbService,
				appService,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			c := &fetchConfig{
				hello: proto.UpstreamInventoryHello{
					Services: tc.helloServices,
				},
			}
			require.Equal(t, tc.expected, c.fetchServices())
		})
	}
}

func TestFetchHostArchitecture(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		desc        string
		execCommand func(string, ...string) ([]byte, error)
		expected    string
	}{
		{
			desc: "set correctly if expected format",
			execCommand: func(name string, args ...string) ([]byte, error) {
				if name != "arch" {
					return nil, trace.NotFound("command does not exist")
				}
				return []byte("x86_64"), nil
			},
			expected: "x86_64",
		},
		{
			desc: "full output if unexpected format",
			execCommand: func(name string, args ...string) ([]byte, error) {
				if name != "arch" {
					return nil, trace.NotFound("command does not exist")
				}
				return []byte("Architecture: x86_64"), nil
			},
			expected: sanitize("Architecture: x86_64"),
		},
		{
			desc: "empty if arch does not exist",
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
			require.Equal(t, tc.expected, c.fetchHostArchitecture())
		})
	}
}

func TestFetchContainerRuntime(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		desc     string
		readFile func(string) ([]byte, error)
		expected string
	}{
		{
			desc: "docker if /.dockerenv exists",
			readFile: func(name string) ([]byte, error) {
				if name != "/.dockerenv" {
					return nil, trace.NotFound("file does not exist")
				}
				return []byte{}, nil
			},
			expected: "docker",
		},
		{
			desc: "empty if /.dockerenv does not exist",
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
			require.Equal(t, tc.expected, c.fetchContainerRuntime())
		})
	}
}
