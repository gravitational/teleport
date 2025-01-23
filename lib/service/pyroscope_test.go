package service

import (
	"os"
	"reflect"
	"testing"
)

func TestAddKubeTagsFromEnv(t *testing.T) {
	tests := []struct {
		name         string
		existingTags map[string]string
		envVars      map[string]string
		expectedTags map[string]string
	}{
		{
			name: "NoEnvironmentVariablesSet",
			existingTags: map[string]string{
				"host":    "hostname",
				"version": "17.0.0",
				"git_ref": "17.0.0-dev.1-123-abcde",
			},
			envVars: map[string]string{},
			expectedTags: map[string]string{
				"host":    "hostname",
				"version": "17.0.0",
				"git_ref": "17.0.0-dev.1-123-abcde",
			},
		},
		{
			name: "AllEnvironmentVariablesSet",
			existingTags: map[string]string{
				"host":    "hostname",
				"version": "17.0.0",
				"git_ref": "17.0.0-dev.1-123-abcde",
			},
			envVars: map[string]string{
				"TELEPORT_PYROSCOPE_KUBE_NAME":              "teleport",
				"TELEPORT_PYROSCOPE_KUBE_INSTANCE":          "teleport-auth",
				"TELEPORT_PYROSCOPE_KUBE_COMPONENT":         "tenant-instance",
				"TELEPORT_PYROSCOPE_KUBE_REGION":            "us-east-1",
				"TELEPORT_PYROSCOPE_KUBE_POD_TEMPLATE_HASH": "abcdef123456",
			},
			expectedTags: map[string]string{
				"host":              "hostname",
				"version":           "17.0.0",
				"git_ref":           "17.0.0-dev.1-123-abcde",
				"name":              "teleport",
				"instance":          "teleport-auth",
				"component":         "tenant-instance",
				"region":            "us-east-1",
				"pod_template_hash": "abcdef123456",
			},
		},
		{
			name: "SomeEnvironmentVariablesSet",
			existingTags: map[string]string{
				"host":    "hostname",
				"version": "17.0.0",
				"git_ref": "17.0.0-dev.1-123-abcde",
			},
			envVars: map[string]string{
				"TELEPORT_PYROSCOPE_KUBE_NAME":   "teleport",
				"TELEPORT_PYROSCOPE_KUBE_REGION": "us-east-1",
			},
			expectedTags: map[string]string{
				"host":    "hostname",
				"version": "17.0.0",
				"git_ref": "17.0.0-dev.1-123-abcde",
				"name":    "teleport",
				"region":  "us-east-1",
			},
		},
		{
			name:         "EmptyExistingTags",
			existingTags: map[string]string{},
			envVars: map[string]string{
				"TELEPORT_PYROSCOPE_KUBE_NAME": "teleport",
			},
			expectedTags: map[string]string{
				"name": "teleport",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variables for this test case
			for envVar, value := range tt.envVars {
				os.Setenv(envVar, value)
			}

			// Call the function
			resultTags := addKubeTagsFromEnv(tt.existingTags)

			// Check if the result matches the expected tags
			if !reflect.DeepEqual(resultTags, tt.expectedTags) {
				t.Errorf("addKubeTagsFromEnv() = %v, want %v", resultTags, tt.expectedTags)
			}
		})
	}
}
