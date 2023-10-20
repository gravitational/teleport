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

package auth

import (
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func TestInstanceMetricsPeriodic(t *testing.T) {
	tts := []struct {
		desc           string
		upgraders      []string
		expectCounts   map[string]int
		expectEnrolled int
	}{
		{
			desc: "mixed",
			upgraders: []string{
				"kube",
				"unit",
				"",
				"unit",
				"",
			},
			expectCounts: map[string]int{
				"kube": 1,
				"unit": 2,
			},
			expectEnrolled: 3,
		},
		{
			desc: "all-unenrolled",
			upgraders: []string{
				"",
				"",
			},
		},
		{
			desc: "all-enrolled",
			upgraders: []string{
				"kube",
				"kube",
				"unit",
				"unit",
			},
			expectCounts: map[string]int{
				"kube": 2,
				"unit": 2,
			},
			expectEnrolled: 4,
		},
		{
			desc: "nothing",
		},
	}

	for _, tt := range tts {
		t.Run(tt.desc, func(t *testing.T) {
			periodic := newInstanceMetricsPeriodic()

			for _, upgrader := range tt.upgraders {
				instance, err := types.NewInstance(uuid.New().String(), types.InstanceSpecV1{
					ExternalUpgrader: upgrader,
				})
				require.NoError(t, err)

				periodic.VisitInstance(instance)
			}

			for upgrader, count := range tt.expectCounts {
				require.Equal(t, count, periodic.InstancesWithUpgrader(upgrader), "upgrader=%q, tt=%q", upgrader, tt.desc)
			}

			require.Equal(t, tt.expectEnrolled, periodic.TotalEnrolledInUpgrades(), "tt=%q", tt.desc)

			require.Equal(t, len(tt.upgraders), periodic.TotalInstances(), "tt=%q", tt.desc)
		})
	}
}

func TestUpgradeEnrollPeriodic(t *testing.T) {
	tts := []struct {
		desc          string
		enrolled      map[string]int
		unenrolled    map[string]int
		promptVersion string
	}{
		{
			desc: "mixed-case",
			enrolled: map[string]int{
				"v2.3.4": 5,
				"v1.2.3": 2,
			},
			unenrolled: map[string]int{
				"v2.3.4": 1,
				"v1.2.3": 2,
				"v1.0.0": 1,
			},
			promptVersion: "v2.3.4",
		},
		{
			desc: "trivial-case",
			enrolled: map[string]int{
				"v1.2.3": 1,
			},
			unenrolled: map[string]int{
				"v1.2.2": 1,
			},
			promptVersion: "v1.2.3",
		},
		{
			desc: "insufficient-lag",
			enrolled: map[string]int{
				"v2.3.4": 2,
				"v1.2.3": 3,
			},
			unenrolled: map[string]int{
				"v1.2.3": 3,
				"v1.2.2": 1,
			},
		},
		{
			desc: "sparse-with-prompt",
			enrolled: map[string]int{
				"v1.2.5": 1,
				"v1.2.4": 1,
				"v1.2.3": 1,
				"v1.2.2": 1,
			},
			unenrolled: map[string]int{
				"v1.2.4": 1,
				"v1.2.3": 1,
				"v1.2.2": 1,
				"v1.2.1": 1,
			},
			promptVersion: "v1.2.4",
		},
		{
			desc: "sparse-no-prompt",
			enrolled: map[string]int{
				"v1.2.4": 1,
				"v1.2.3": 1,
				"v1.2.2": 1,
				"v1.2.1": 1,
			},
			unenrolled: map[string]int{
				"v1.2.4": 1,
				"v1.2.3": 1,
				"v1.2.2": 1,
				"v1.2.1": 1,
			},
		},
		{
			desc: "no-enrolled",
			unenrolled: map[string]int{
				"v1.2.3": 1,
			},
		},
		{
			desc: "all-enrolled",
			enrolled: map[string]int{
				"v2.3.4": 1,
			},
		},
		{
			desc: "no-instances",
		},
	}

	for _, tt := range tts {
		t.Run(tt.desc, func(t *testing.T) {
			periodic := newUpgradeEnrollPeriodic()

			for ver, count := range tt.enrolled {
				for i := 0; i < count; i++ {
					instance, err := types.NewInstance(uuid.New().String(), types.InstanceSpecV1{
						Version:          ver,
						ExternalUpgrader: "some-upgrader",
					})
					require.NoError(t, err)

					periodic.VisitInstance(instance)
				}
			}

			for ver, count := range tt.unenrolled {
				for i := 0; i < count; i++ {
					instance, err := types.NewInstance(uuid.New().String(), types.InstanceSpecV1{
						Version:          ver,
						ExternalUpgrader: "",
					})
					require.NoError(t, err)

					periodic.VisitInstance(instance)
				}
			}

			msg, ok := periodic.GenerateEnrollPrompt()

			if tt.promptVersion == "" {
				require.False(t, ok, "expected no prompt gen, but got %q, tt=%q", msg, tt.desc)
				return
			}

			require.True(t, ok, "expected prompt containing version %s, but prompt was not generated. tt=%q", tt.promptVersion, tt.desc)

			pattern := fmt.Sprintf("--older-than=%s", tt.promptVersion)
			require.Contains(t, msg, pattern, "tt=%q", tt.desc)
		})
	}
}
