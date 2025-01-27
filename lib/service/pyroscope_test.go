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
				"TELEPORT_PYROSCOPE_KUBE_NAME":      "test-name",
				"TELEPORT_PYROSCOPE_KUBE_INSTANCE":  "test-instance",
				"TELEPORT_PYROSCOPE_KUBE_COMPONENT": "test-component",
				"TELEPORT_PYROSCOPE_KUBE_NAMESPACE": "test-namespace",
				"TELEPORT_PYROSCOPE_KUBE_REGION":    "us-east-1",
			},
			initialTags: map[string]string{},
			wantTags: map[string]string{
				"name":      "test-name",
				"instance":  "test-instance",
				"component": "test-component",
				"namespace": "test-namespace",
				"region":    "us-east-1",
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
