/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package auth

import (
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
)

func TestInstanceMetricsPeriodic(t *testing.T) {
	tts := []struct {
		desc           string
		instances      []proto.UpstreamInventoryHello
		expectedCounts map[string]map[string]int
		upgraders      []string
		expectEnrolled int
	}{
		{
			desc: "mixed",
			instances: []proto.UpstreamInventoryHello{
				{ExternalUpgrader: "kube", ExternalUpgraderVersion: "13.0.0"},
				{ExternalUpgrader: "kube", ExternalUpgraderVersion: "14.0.0"},
				{ExternalUpgrader: "unit", ExternalUpgraderVersion: "13.0.0"},
				{},
				{ExternalUpgrader: "unit", ExternalUpgraderVersion: "14.0.0"},
				{},
			},
			upgraders: []string{
				"kube",
				"kube",
				"unit",
				"",
				"unit",
				"",
			},
			expectedCounts: map[string]map[string]int{
				"kube": {
					"13.0.0": 1,
					"14.0.0": 1,
				},
				"unit": {
					"13.0.0": 1,
					"14.0.0": 1,
				},
			},
			expectEnrolled: 4,
		},
		{
			desc: "all-unenrolled",
			instances: []proto.UpstreamInventoryHello{
				{},
				{},
			},
			upgraders: []string{
				"",
				"",
			},
			expectedCounts: map[string]map[string]int{},
		},
		{
			desc: "all-enrolled",
			instances: []proto.UpstreamInventoryHello{
				{ExternalUpgrader: "kube", ExternalUpgraderVersion: "13.0.0"},
				{ExternalUpgrader: "kube", ExternalUpgraderVersion: "13.0.0"},
				{ExternalUpgrader: "unit", ExternalUpgraderVersion: "13.0.0"},
				{ExternalUpgrader: "unit", ExternalUpgraderVersion: "13.0.0"},
			},
			upgraders: []string{
				"kube",
				"kube",
				"unit",
				"unit",
			},
			expectedCounts: map[string]map[string]int{
				"kube": {
					"13.0.0": 2,
				},
				"unit": {
					"13.0.0": 2,
				},
			},
			expectEnrolled: 4,
		},
		{
			desc: "nil version",
			instances: []proto.UpstreamInventoryHello{
				{ExternalUpgrader: "kube"},
				{ExternalUpgrader: "unit"},
			},
			upgraders: []string{
				"kube",
				"unit",
			},
			expectedCounts: map[string]map[string]int{
				"kube": {
					"": 1,
				},
				"unit": {
					"": 1,
				},
			},
			expectEnrolled: 2,
		},
		{
			desc:           "nothing",
			expectedCounts: map[string]map[string]int{},
		},
	}

	for _, tt := range tts {
		t.Run(tt.desc, func(t *testing.T) {
			periodic := newInstanceMetricsPeriodic()

			for _, instance := range tt.instances {
				periodic.VisitInstance(instance)
			}

			require.Equal(t, tt.expectedCounts, periodic.upgraderCounts, "tt=%q", tt.desc)

			require.Equal(t, tt.expectEnrolled, periodic.TotalEnrolledInUpgrades(), "tt=%q", tt.desc)

			require.Len(t, tt.upgraders, periodic.TotalInstances(), "tt=%q", tt.desc)
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
