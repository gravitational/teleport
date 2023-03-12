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

package controller

import (
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
)

const (
	nonSemverTag = "my-custom-tag"
)

func newDeploymentWithImage(image string) *appsv1.Deployment {
	return &appsv1.Deployment{
		Spec: appsv1.DeploymentSpec{
			Template: v1.PodTemplateSpec{
				Spec: v1.PodSpec{
					Containers: []v1.Container{{
						Name:  teleportContainerName,
						Image: image,
					}},
				},
			},
		},
	}
}

func Test_getDeploymentVersion(t *testing.T) {
	tests := []struct {
		name       string
		deployment *appsv1.Deployment
		expected   string
		assertErr  require.ErrorAssertionFunc
	}{
		{
			name:       "OK regular deployment, semver tag no digest",
			deployment: newDeploymentWithImage(defaultTestRegistry + "/" + defaultTestPath + ":" + versionMid),
			expected:   "v" + versionMid,
			assertErr:  require.NoError,
		},
		{
			name:       "OK regular deployment, semver tag with digest",
			deployment: newDeploymentWithImage(defaultTestRegistry + "/" + defaultTestPath + ":" + versionMid + "@" + defaultImageDigest.String()),
			expected:   "v" + versionMid,
			assertErr:  require.NoError,
		},
		{
			name:       "KO regular deployment, non-semver tag no digest",
			deployment: newDeploymentWithImage(defaultTestRegistry + "/" + defaultTestPath + ":" + nonSemverTag),
			expected:   "",
			assertErr:  errorIsType(&trace.BadParameterError{}),
		},
		{
			name:       "KO regular deployment, non-semver tag with digest",
			deployment: newDeploymentWithImage(defaultTestRegistry + "/" + defaultTestPath + ":" + nonSemverTag + "@" + defaultImageDigest.String()),
			expected:   "",
			assertErr:  errorIsType(&trace.BadParameterError{}),
		},
		{
			name:       "KO regular deployment, no tag, only digest",
			deployment: newDeploymentWithImage(defaultTestRegistry + "/" + defaultTestPath + "@" + defaultImageDigest.String()),
			expected:   "",
			assertErr:  errorIsType(&trace.BadParameterError{}),
		},
		{
			name:       "KO regular deployment, no tag, no digest",
			deployment: newDeploymentWithImage(defaultTestRegistry + "/" + defaultTestPath),
			expected:   "",
			assertErr:  errorIsType(&trace.BadParameterError{}),
		},
		{
			name: "OK regular deployment multi-container",
			deployment: &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Template: v1.PodTemplateSpec{
						Spec: v1.PodSpec{
							Containers: []v1.Container{
								{
									Image: "foo",
									Name:  "bar",
								},
								{
									Image: defaultTestRegistry + "/" + defaultTestPath + ":" + versionMid,
									Name:  teleportContainerName,
								},
							},
						},
					},
				},
			},
			expected:  "v" + versionMid,
			assertErr: require.NoError,
		},
		{
			name: "KO no teleport container",
			deployment: &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Template: v1.PodTemplateSpec{
						Spec: v1.PodSpec{
							Containers: []v1.Container{
								{
									Image: "foo",
									Name:  "bar",
								},
							},
						},
					},
				},
			},
			expected:  "",
			assertErr: require.Error,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getDeploymentVersion(tt.deployment)
			tt.assertErr(t, err)
			require.Equal(t, tt.expected, got)
		})
	}
}
