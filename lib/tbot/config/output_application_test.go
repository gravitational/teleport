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

func TestApplicationOutput_YAML(t *testing.T) {
	dest := &DestinationMemory{}
	tests := []testYAMLCase[ApplicationOutput]{
		{
			name: "full",
			in: ApplicationOutput{
				Destination: dest,
				Roles:       []string{"access"},
				AppName:     "my-app",
			},
		},
		{
			name: "minimal",
			in: ApplicationOutput{
				Destination: dest,
				AppName:     "my-app",
			},
		},
	}
	testYAML(t, tests)
}

func TestApplicationOutput_CheckAndSetDefaults(t *testing.T) {
	tests := []testCheckAndSetDefaultsCase[*ApplicationOutput]{
		{
			name: "valid",
			in: func() *ApplicationOutput {
				return &ApplicationOutput{
					Destination: memoryDestForTest(),
					Roles:       []string{"access"},
					AppName:     "app",
				}
			},
		},
		{
			name: "missing destination",
			in: func() *ApplicationOutput {
				return &ApplicationOutput{
					Destination: nil,
					AppName:     "app",
				}
			},
			wantErr: "no destination configured for output",
		},
		{
			name: "missing app_name",
			in: func() *ApplicationOutput {
				return &ApplicationOutput{
					Destination: memoryDestForTest(),
				}
			},
			wantErr: "app_name must not be empty",
		},
	}
	testCheckAndSetDefaults(t, tests)
}
