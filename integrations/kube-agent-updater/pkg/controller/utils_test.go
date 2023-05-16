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
	v1 "k8s.io/api/core/v1"
)

const (
	nonSemverTag = "my-custom-tag"
)

func Test_getContainerImageFromPod(t *testing.T) {
	image := validRandomResourceName("img")
	secondImage := validRandomResourceName("otherimage")

	type args struct {
		spec      v1.PodSpec
		container string
	}
	tests := []struct {
		name      string
		args      args
		want      string
		assertErr require.ErrorAssertionFunc
	}{
		{
			name: "single container OK",
			args: args{
				spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Name:  teleportContainerName,
							Image: image,
						},
					},
				},
				container: teleportContainerName,
			},
			want:      image,
			assertErr: require.NoError,
		},
		{
			name: "multiple containers OK",
			args: args{
				spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Name:  "foo",
							Image: secondImage,
						},
						{
							Name:  teleportContainerName,
							Image: image,
						},
					},
				},
				container: teleportContainerName,
			},
			want:      image,
			assertErr: require.NoError,
		},
		{
			name: "single container KO",
			args: args{
				spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Name:  "bar",
							Image: image,
						},
					},
				},
				container: teleportContainerName,
			},
			want:      "",
			assertErr: require.Error,
		},
		{
			name: "multiple containers KO",
			args: args{
				spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Name:  "foo",
							Image: secondImage,
						},
						{
							Name:  "bar",
							Image: image,
						},
					},
				},
				container: teleportContainerName,
			},
			want:      "",
			assertErr: require.Error,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getContainerImageFromPodSpec(tt.args.spec, tt.args.container)
			tt.assertErr(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func Test_setContainerImageFromPod(t *testing.T) {
	image := validRandomResourceName("img")
	type args struct {
		spec      *v1.PodSpec
		container string
		image     string
	}
	tests := []struct {
		name      string
		args      args
		assertErr require.ErrorAssertionFunc
		expected  *v1.PodSpec
	}{
		{
			name: "single container OK",
			args: args{
				spec: &v1.PodSpec{
					Containers: []v1.Container{
						{
							Name:  teleportContainerName,
							Image: "changeme",
						},
					}},
				container: teleportContainerName,
				image:     image,
			},
			assertErr: require.NoError,
			expected: &v1.PodSpec{
				Containers: []v1.Container{
					{
						Name:  teleportContainerName,
						Image: image,
					},
				},
			},
		},
		{
			name: "multi container OK",
			args: args{
				spec: &v1.PodSpec{
					Containers: []v1.Container{
						{
							Name:  "foo",
							Image: "dontchangeme",
						},
						{
							Name:  teleportContainerName,
							Image: "changeme",
						},
					}},
				container: teleportContainerName,
				image:     image,
			},
			assertErr: require.NoError,
			expected: &v1.PodSpec{
				Containers: []v1.Container{
					{
						Name:  "foo",
						Image: "dontchangeme",
					},
					{
						Name:  teleportContainerName,
						Image: image,
					},
				},
			},
		},
		{
			name: "single container KO",
			args: args{
				spec: &v1.PodSpec{
					Containers: []v1.Container{
						{
							Name:  "nomatch",
							Image: "dontchangeme",
						},
					}},
				container: teleportContainerName,
				image:     image,
			},
			assertErr: require.Error,
			expected: &v1.PodSpec{
				Containers: []v1.Container{
					{
						Name:  "nomatch",
						Image: "dontchangeme",
					},
				},
			},
		},
		{
			name: "multi container KO",
			args: args{
				spec: &v1.PodSpec{
					Containers: []v1.Container{
						{
							Name:  "nomatch",
							Image: "dontchangeme",
						},
						{
							Name:  "nomatchbis",
							Image: "dontchangeme",
						},
					}},
				container: teleportContainerName,
				image:     image,
			},
			assertErr: require.Error,
			expected: &v1.PodSpec{
				Containers: []v1.Container{
					{
						Name:  "nomatch",
						Image: "dontchangeme",
					},
					{
						Name:  "nomatchbis",
						Image: "dontchangeme",
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := setContainerImageFromPodSpec(tt.args.spec, tt.args.container, tt.args.image)
			tt.assertErr(t, err)
			require.Equal(t, tt.expected, tt.args.spec)
		})
	}
}
func newPodSpecWithImage(image string) v1.PodSpec {
	return v1.PodSpec{
		Containers: []v1.Container{{
			Name:  teleportContainerName,
			Image: image,
		}},
	}
}

func Test_getWorkloadVersion(t *testing.T) {
	tests := []struct {
		name      string
		podSpec   v1.PodSpec
		expected  string
		assertErr require.ErrorAssertionFunc
	}{
		{
			name:      "OK regular podSpec, semver tag no digest",
			podSpec:   newPodSpecWithImage(defaultTestRegistry + "/" + defaultTestPath + ":" + versionMid),
			expected:  "v" + versionMid,
			assertErr: require.NoError,
		},
		{
			name:      "OK regular podSpec, semver tag with digest",
			podSpec:   newPodSpecWithImage(defaultTestRegistry + "/" + defaultTestPath + ":" + versionMid + "@" + defaultImageDigest.String()),
			expected:  "v" + versionMid,
			assertErr: require.NoError,
		},
		{
			name:      "KO regular podSpec, non-semver tag no digest",
			podSpec:   newPodSpecWithImage(defaultTestRegistry + "/" + defaultTestPath + ":" + nonSemverTag),
			expected:  "",
			assertErr: errorIsType(&trace.BadParameterError{}),
		},
		{
			name:      "KO regular podSpec, non-semver tag with digest",
			podSpec:   newPodSpecWithImage(defaultTestRegistry + "/" + defaultTestPath + ":" + nonSemverTag + "@" + defaultImageDigest.String()),
			expected:  "",
			assertErr: errorIsType(&trace.BadParameterError{}),
		},
		{
			name:      "KO regular podSpec, no tag, only digest",
			podSpec:   newPodSpecWithImage(defaultTestRegistry + "/" + defaultTestPath + "@" + defaultImageDigest.String()),
			expected:  "",
			assertErr: errorIsType(&trace.BadParameterError{}),
		},
		{
			name:      "KO regular podSpec, no tag, no digest",
			podSpec:   newPodSpecWithImage(defaultTestRegistry + "/" + defaultTestPath),
			expected:  "",
			assertErr: errorIsType(&trace.BadParameterError{}),
		},
		{
			name: "OK regular podSpec multi-container",
			podSpec: v1.PodSpec{
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
			expected:  "v" + versionMid,
			assertErr: require.NoError,
		},
		{
			name: "KO no teleport container",
			podSpec: v1.PodSpec{
				Containers: []v1.Container{
					{
						Image: "foo",
						Name:  "bar",
					},
				},
			},
			expected:  "",
			assertErr: require.Error,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getWorkloadVersion(tt.podSpec)
			tt.assertErr(t, err)
			require.Equal(t, tt.expected, got)
		})
	}
}
