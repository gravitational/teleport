// Copyright 2025 Gravitational, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package gcp

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsAlloyDBConnectionURI(t *testing.T) {
	require.True(t, IsAlloyDBConnectionURI("alloydb://dummy"))
	require.False(t, IsAlloyDBConnectionURI("http://dummy"))
	require.False(t, IsAlloyDBConnectionURI("just/some/stuff"))
}

func TestParseAlloyDBConnectionURI(t *testing.T) {
	tests := []struct {
		name    string
		uri     string
		want    *AlloyDBFullInstanceName
		wantErr string
	}{
		{
			name: "valid address",
			uri:  "alloydb://projects/my-project-123456/locations/europe-west1/clusters/my-cluster/instances/my-instance",
			want: &AlloyDBFullInstanceName{
				ProjectID:  "my-project-123456",
				Location:   "europe-west1",
				ClusterID:  "my-cluster",
				InstanceID: "my-instance",
			},
		},
		{
			name:    "empty string is rejected",
			uri:     "",
			wantErr: `connection URI cannot be empty`,
		},
		{
			name:    "missing 'projects'",
			uri:     "alloydb://PROJECTS/my-project-123456/locations/europe-west1/clusters/my-cluster/instances/my-instance",
			wantErr: `invalid connection URI "alloydb://PROJECTS/my-project-123456/locations/europe-west1/clusters/my-cluster/instances/my-instance": expected 'projects', got "PROJECTS"`,
		},
		{
			name:    "missing 'locations'",
			uri:     "alloydb://projects/my-project-123456/LOCATIONS/europe-west1/clusters/my-cluster/instances/my-instance",
			wantErr: `invalid connection URI "alloydb://projects/my-project-123456/LOCATIONS/europe-west1/clusters/my-cluster/instances/my-instance": expected 'locations', got "LOCATIONS"`,
		},
		{
			name:    "missing 'clusters'",
			uri:     "alloydb://projects/my-project-123456/locations/europe-west1/CLUSTERS/my-cluster/instances/my-instance",
			wantErr: `invalid connection URI "alloydb://projects/my-project-123456/locations/europe-west1/CLUSTERS/my-cluster/instances/my-instance": expected 'clusters', got "CLUSTERS"`,
		},
		{
			name:    "missing 'instances'",
			uri:     "alloydb://projects/my-project-123456/locations/europe-west1/clusters/my-cluster/INSTANCES/my-instance",
			wantErr: `invalid connection URI "alloydb://projects/my-project-123456/locations/europe-west1/clusters/my-cluster/INSTANCES/my-instance": expected 'instances', got "INSTANCES"`,
		},
		{
			name:    "empty project",
			uri:     "alloydb://projects//locations/europe-west1/clusters/my-cluster/instances/my-instance",
			wantErr: `invalid connection URI "alloydb://projects//locations/europe-west1/clusters/my-cluster/instances/my-instance": project cannot be empty`,
		},
		{
			name:    "empty location",
			uri:     "alloydb://projects/my-project-123456/locations//clusters/my-cluster/instances/my-instance",
			wantErr: `invalid connection URI "alloydb://projects/my-project-123456/locations//clusters/my-cluster/instances/my-instance": location cannot be empty`,
		},
		{
			name:    "empty cluster",
			uri:     "alloydb://projects/my-project-123456/locations/europe-west1/clusters//instances/my-instance",
			wantErr: `invalid connection URI "alloydb://projects/my-project-123456/locations/europe-west1/clusters//instances/my-instance": cluster cannot be empty`,
		},
		{
			name:    "empty instance",
			uri:     "alloydb://projects/my-project-123456/locations/europe-west1/clusters/my-cluster/instances/",
			wantErr: `invalid connection URI "alloydb://projects/my-project-123456/locations/europe-west1/clusters/my-cluster/instances/": instance cannot be empty`,
		},
		{
			name:    "missing scheme",
			uri:     "projects/my-project-123456/locations/europe-west1/clusters/my-cluster/instances/my-instance",
			wantErr: `invalid connection URI "projects/my-project-123456/locations/europe-west1/clusters/my-cluster/instances/my-instance": should start with alloydb://`,
		},
		{
			name:    "invalid address",
			uri:     "alloydb://invalid",
			wantErr: `invalid connection URI "alloydb://invalid": wrong number of parts`,
		},
		{
			name:    "query params are not accepted",
			uri:     "alloydb://projects/my-project-123456/locations/europe-west1/clusters/my-cluster/instances/my-instance?foo=bar",
			wantErr: `invalid connection URI "alloydb://projects/my-project-123456/locations/europe-west1/clusters/my-cluster/instances/my-instance?foo=bar": query parameters are not accepted`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed, err := ParseAlloyDBConnectionURI(tt.uri)
			if tt.wantErr == "" {
				require.NoError(t, err)
				require.Equal(t, tt.want, parsed)
			} else {
				require.ErrorContains(t, err, tt.wantErr)
			}
		})
	}
}

func TestAlloyDBResourceNames(t *testing.T) {
	info := AlloyDBFullInstanceName{
		ProjectID:  "my-project-123456",
		Location:   "europe-west1",
		ClusterID:  "my-cluster",
		InstanceID: "my-instance",
	}
	require.Equal(t, "projects/my-project-123456/locations/europe-west1/clusters/my-cluster", info.ParentClusterName())
	require.Equal(t, "projects/my-project-123456/locations/europe-west1/clusters/my-cluster/instances/my-instance", info.InstanceName())
}

func TestValidateAlloyDBEndpointType(t *testing.T) {
	tests := []struct {
		name    string
		str     string
		wantErr string
	}{
		{name: "empty string", str: "", wantErr: ""},
		{name: "private", str: "private", wantErr: ""},
		{name: "public", str: "public", wantErr: ""},
		{name: "psc", str: "psc", wantErr: ""},
		{name: "caps", str: "PUBLIC", wantErr: `invalid alloy db endpoint type: PUBLIC, expected one of [public private psc]`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := ValidateAlloyDBEndpointType(test.str)
			if test.wantErr != "" {
				require.ErrorContains(t, err, test.wantErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
