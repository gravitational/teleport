// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package service

import (
	"os"
	"testing"

	"github.com/grafana/pyroscope-go"
	"github.com/stretchr/testify/assert"
)

func TestGetPyroscopeProfileTypesFromEnv(t *testing.T) {
	tests := []struct {
		name    string
		envVars map[string]string
		want    []pyroscope.ProfileType
	}{
		{
			name:    "No profiles enabled",
			envVars: map[string]string{},
			want:    []pyroscope.ProfileType{},
		},
		{
			name: "Memory profiles enabled",
			envVars: map[string]string{
				"TELEPORT_PYROSCOPE_PROFILE_MEMORY_ENABLED": "true",
			},
			want: []pyroscope.ProfileType{
				pyroscope.ProfileAllocObjects,
				pyroscope.ProfileAllocSpace,
				pyroscope.ProfileInuseObjects,
				pyroscope.ProfileInuseSpace,
			},
		},
		{
			name: "CPU profiles enabled",
			envVars: map[string]string{
				"TELEPORT_PYROSCOPE_PROFILE_CPU_ENABLED": "true",
			},
			want: []pyroscope.ProfileType{
				pyroscope.ProfileCPU,
			},
		},
		{
			name: "Goroutine profiles enabled",
			envVars: map[string]string{
				"TELEPORT_PYROSCOPE_PROFILE_GOROUTINES_ENABLED": "true",
			},
			want: []pyroscope.ProfileType{
				pyroscope.ProfileGoroutines,
			},
		},
		{
			name: "Multiple profiles enabled",
			envVars: map[string]string{
				"TELEPORT_PYROSCOPE_PROFILE_MEMORY_ENABLED":     "true",
				"TELEPORT_PYROSCOPE_PROFILE_CPU_ENABLED":        "true",
				"TELEPORT_PYROSCOPE_PROFILE_GOROUTINES_ENABLED": "true",
			},
			want: []pyroscope.ProfileType{
				pyroscope.ProfileAllocObjects,
				pyroscope.ProfileAllocSpace,
				pyroscope.ProfileInuseObjects,
				pyroscope.ProfileInuseSpace,
				pyroscope.ProfileCPU,
				pyroscope.ProfileGoroutines,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			for k, v := range tc.envVars {
				os.Setenv(k, v)
			}
			defer func() {
				for k := range tc.envVars {
					os.Unsetenv(k)
				}
			}()

			got := getPyroscopeProfileTypesFromEnv()
			assert.ElementsMatch(t, tc.want, got)
		})
	}
}

func TestAddKubeTagsFromEnv(t *testing.T) {
	tests := []struct {
		name        string
		envVars     map[string]string
		initialTags map[string]string
		wantTags    map[string]string
	}{
		{
			name:        "No Kubernetes env variables set",
			envVars:     map[string]string{},
			initialTags: map[string]string{},
			wantTags:    map[string]string{},
		},
		{
			name: "Kubernetes env variables set",
			envVars: map[string]string{
				"TELEPORT_PYROSCOPE_KUBE_COMPONENT": "auth",
				"TELEPORT_PYROSCOPE_KUBE_NAMESPACE": "test-namespace",
			},
			initialTags: map[string]string{},
			wantTags: map[string]string{
				"component": "auth",
				"namespace": "test-namespace",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			for k, v := range tc.envVars {
				os.Setenv(k, v)
			}
			defer func() {
				for k := range tc.envVars {
					os.Unsetenv(k)
				}
			}()

			gotTags := addKubeTagsFromEnv(tc.initialTags)
			assert.Equal(t, tc.wantTags, gotTags)
		})
	}
}
