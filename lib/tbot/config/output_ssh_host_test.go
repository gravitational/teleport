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

package config

import "testing"

func TestSSHHostOutput_YAML(t *testing.T) {
	dest := &DestinationMemory{}
	tests := []testYAMLCase[SSHHostOutput]{
		{
			name: "full",
			in: SSHHostOutput{
				Destination: dest,
				Roles:       []string{"access"},
				Principals:  []string{"host.example.com"},
			},
		},
		{
			name: "minimal",
			in: SSHHostOutput{
				Destination: dest,
				Principals:  []string{"host.example.com"},
			},
		},
	}
	testYAML(t, tests)
}

func TestSSHHostOutput_CheckAndSetDefaults(t *testing.T) {
	tests := []testCheckAndSetDefaultsCase[*SSHHostOutput]{
		{
			name: "valid",
			in: func() *SSHHostOutput {
				return &SSHHostOutput{
					Destination: memoryDestForTest(),
					Roles:       []string{"access"},
					Principals:  []string{"host.example.com"},
				}
			},
		},
		{
			name: "missing destination",
			in: func() *SSHHostOutput {
				return &SSHHostOutput{
					Destination: nil,
					Principals:  []string{"host.example.com"},
				}
			},
			wantErr: "no destination configured for output",
		},
		{
			name: "missing principals",
			in: func() *SSHHostOutput {
				return &SSHHostOutput{
					Destination: memoryDestForTest(),
				}
			},
			wantErr: "at least one principal must be specified",
		},
	}
	testCheckAndSetDefaults(t, tests)
}
