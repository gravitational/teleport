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

func TestDestinationKubernetesSecret_CheckAndSetDefaults(t *testing.T) {
	tests := []testCheckAndSetDefaultsCase[*DestinationKubernetesSecret]{
		{
			name: "valid",
			in: func() *DestinationKubernetesSecret {
				return &DestinationKubernetesSecret{
					Name: "my-secret",
				}
			},
		},
		{
			name: "missing name",
			in: func() *DestinationKubernetesSecret {
				return &DestinationKubernetesSecret{
					Name: "",
				}
			},
			wantErr: "name must not be empty",
		},
	}
	testCheckAndSetDefaults(t, tests)
}

func TestDestinationKubernetesSecret_YAML(t *testing.T) {
	tests := []testYAMLCase[DestinationKubernetesSecret]{
		{
			name: "full",
			in: DestinationKubernetesSecret{
				Name: "my-secret",
			},
		},
	}
	testYAML(t, tests)
}
