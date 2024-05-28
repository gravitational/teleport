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

package types

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestServerInfoSetLabels(t *testing.T) {
	t.Parallel()
	labels := map[string]string{
		"a":         "1",
		"dynamic/b": "2",
		"aws/c":     "3",
	}

	tests := []struct {
		name           string
		serverInfoName string
		expectedLabels map[string]string
	}{
		{
			name:           "fix manual labels",
			serverInfoName: "si-test",
			expectedLabels: map[string]string{
				"dynamic/a": "1",
				"dynamic/b": "2",
				"dynamic/c": "3",
			},
		},
		{
			name:           "fix aws labels",
			serverInfoName: "aws-test",
			expectedLabels: map[string]string{
				"aws/a": "1",
				"aws/b": "2",
				"aws/c": "3",
			},
		},
		{
			name:           "leave other labels alone",
			serverInfoName: "test",
			expectedLabels: labels,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			si, err := NewServerInfo(Metadata{
				Name: tc.serverInfoName,
			}, ServerInfoSpecV1{
				NewLabels: labels,
			})
			require.NoError(t, err)
			require.Equal(t, tc.expectedLabels, si.GetNewLabels())
		})
	}
}

func TesServerInfoForServer(t *testing.T) {
	tests := []struct {
		name   string
		server Server
		want   string
	}{
		{
			name: "node with account and instance id labels uses the aws-<account-id>-<instance-id> format",
			server: &ServerV2{
				Metadata: Metadata{
					Labels: map[string]string{
						AWSAccountIDLabel:  "123456789012",
						AWSInstanceIDLabel: "i-123",
					},
				},
			},
			want: "aws-123456789012-i-123",
		},
		{
			name: "node with aws metadata uses the aws-<account-id>-<instance-id> format",
			server: &ServerV2{
				Metadata: Metadata{Labels: map[string]string{}},
				Spec: ServerSpecV2{
					CloudMetadata: &CloudMetadata{
						AWS: &AWSInfo{
							AccountID:  "123456789012",
							InstanceID: "i-123",
						},
					},
				},
			},
			want: "aws-123456789012-i-123",
		},
		{
			name: "other nodes have their server info name following the si-<namen> format",
			server: &ServerV2{
				Metadata: Metadata{
					Name:   "abcd",
					Labels: map[string]string{},
				},
			},
			want: "si-abcd",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ServerInfoForServer(tt.server)
			require.NoError(t, err)
			if got.GetName() != tt.want {
				t.Errorf("ServerInfoForServer() = %v, want %v", got, tt.want)
			}
		})
	}
}
