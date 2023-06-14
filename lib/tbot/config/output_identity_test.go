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

import (
	"testing"
)

func TestIdentityOutput_YAML(t *testing.T) {
	dest := &DestinationMemory{}
	tests := []testYAMLCase[IdentityOutput]{
		{
			name: "full",
			in: IdentityOutput{
				Destination: dest,
				Roles:       []string{"access"},
				Cluster:     "leaf.example.com",
			},
		},
		{
			name: "minimal",
			in: IdentityOutput{
				Destination: dest,
			},
		},
	}
	testYAML(t, tests)
}

func TestIdentityOutput_CheckAndSetDefaults(t *testing.T) {
	tests := []testCheckAndSetDefaultsCase[*IdentityOutput]{
		{
			name: "valid",
			in: func() *IdentityOutput {
				return &IdentityOutput{
					Destination: memoryDestForTest(),
					Roles:       []string{"access"},
				}
			},
		},
		{
			name: "missing destination",
			in: func() *IdentityOutput {
				return &IdentityOutput{
					Destination: nil,
				}
			},
			wantErr: "no destination configured for output",
		},
	}
	testCheckAndSetDefaults(t, tests)
}
