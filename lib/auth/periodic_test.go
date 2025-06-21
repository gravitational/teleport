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

func TestTotalInstances(t *testing.T) {
	instances := []*proto.UpstreamInventoryHello{
		{},
		{Version: "15.0.0"},
		{ServerID: "id"},
		{ExternalUpgrader: "kube"},
		{ExternalUpgraderVersion: "14.0.0"},
	}

	periodic := newInstanceMetricsPeriodic()
	for _, instance := range instances {
		periodic.VisitInstance(instance, new(proto.UpstreamInventoryAgentMetadata))
	}

	require.Equal(t, 5, periodic.TotalInstances())
}

func TestTotalEnrolledInUpgrades(t *testing.T) {
	tts := []struct {
		desc      string
		instances []*proto.UpstreamInventoryHello
		expected  int
	}{
		{
			desc: "mixed",
			instances: []*proto.UpstreamInventoryHello{
				{ExternalUpgrader: "kube", ExternalUpgraderVersion: "13.0.0"},
				{ExternalUpgrader: "kube", ExternalUpgraderVersion: "14.0.0"},
				{ExternalUpgrader: "unit", ExternalUpgraderVersion: "13.0.0"},
				{},
				{ExternalUpgrader: "unit", ExternalUpgraderVersion: "14.0.0"},
				{},
			},
			expected: 4,
		},
		{
			desc: "version omitted",
			instances: []*proto.UpstreamInventoryHello{
				{ExternalUpgrader: "kube"},
				{ExternalUpgrader: "unit"},
			},
			expected: 2,
		},
		{
			desc: "all-unenrolled",
			instances: []*proto.UpstreamInventoryHello{
				{},
				{},
			},
			expected: 0,
		},
		{
			desc:      "none",
			instances: []*proto.UpstreamInventoryHello{},
			expected:  0,
		},
	}
	for _, tt := range tts {
		t.Run(tt.desc, func(t *testing.T) {
			periodic := newInstanceMetricsPeriodic()
			for _, instance := range tt.instances {
				periodic.VisitInstance(instance, new(proto.UpstreamInventoryAgentMetadata))
			}
			require.Equal(t, tt.expected, periodic.TotalEnrolledInUpgrades(), "tt=%q", tt.desc)
		})
	}
}

func TestUpgraderCounts(t *testing.T) {
	tts := []struct {
		desc      string
		instances []*proto.UpstreamInventoryHello
		expected  map[upgrader]int
	}{
		{
			desc: "mixed",
			instances: []*proto.UpstreamInventoryHello{
				{ExternalUpgrader: "kube", ExternalUpgraderVersion: "13.0.0"},
				{ExternalUpgrader: "kube", ExternalUpgraderVersion: "14.0.0"},
				{ExternalUpgrader: "unit", ExternalUpgraderVersion: "13.0.0"},
				{},
				{ExternalUpgrader: "unit", ExternalUpgraderVersion: "14.0.0"},
				{},
			},
			expected: map[upgrader]int{
				{"kube", "13.0.0"}: 1,
				{"kube", "14.0.0"}: 1,
				{"unit", "13.0.0"}: 1,
				{"unit", "14.0.0"}: 1,
			},
		},
		{
			desc: "all-unenrolled",
			instances: []*proto.UpstreamInventoryHello{
				{},
				{},
			},
			expected: map[upgrader]int{},
		},
		{
			desc: "all-enrolled",
			instances: []*proto.UpstreamInventoryHello{
				{ExternalUpgrader: "kube", ExternalUpgraderVersion: "13.0.0"},
				{ExternalUpgrader: "kube", ExternalUpgraderVersion: "13.0.0"},
				{ExternalUpgrader: "unit", ExternalUpgraderVersion: "13.0.0"},
				{ExternalUpgrader: "unit", ExternalUpgraderVersion: "13.0.0"},
			},
			expected: map[upgrader]int{
				{"kube", "13.0.0"}: 2,
				{"unit", "13.0.0"}: 2,
			},
		},
		{
			desc: "nil version",
			instances: []*proto.UpstreamInventoryHello{
				{ExternalUpgrader: "kube"},
				{ExternalUpgrader: "unit"},
			},
			expected: map[upgrader]int{
				{"kube", ""}: 1,
				{"unit", ""}: 1,
			},
		},
		{
			desc:     "nothing",
			expected: map[upgrader]int{},
		},
	}

	for _, tt := range tts {
		t.Run(tt.desc, func(t *testing.T) {
			periodic := newInstanceMetricsPeriodic()

			for _, instance := range tt.instances {
				periodic.VisitInstance(instance, new(proto.UpstreamInventoryAgentMetadata))
			}
			require.Equal(t, tt.expected, periodic.UpgraderCounts(), "tt=%q", tt.desc)
		})
	}
}

func TestInstallMethodCounts(t *testing.T) {
	tts := []struct {
		desc     string
		metadata []*proto.UpstreamInventoryAgentMetadata
		expected map[string]int
	}{
		{
			desc:     "none",
			metadata: []*proto.UpstreamInventoryAgentMetadata{},
			expected: map[string]int{},
		},
		{
			desc: "unknown install method",
			metadata: []*proto.UpstreamInventoryAgentMetadata{
				{},
			},
			expected: map[string]int{
				"unknown": 1,
			},
		},
		{
			desc: "various install methods",
			metadata: []*proto.UpstreamInventoryAgentMetadata{
				{InstallMethods: []string{"systemctl"}},
				{InstallMethods: []string{"systemctl"}},
				{InstallMethods: []string{"helm_kube_agent"}},
				{InstallMethods: []string{"dockerfile"}},
			},
			expected: map[string]int{
				"systemctl":       2,
				"helm_kube_agent": 1,
				"dockerfile":      1,
			},
		},
		{
			desc: "multiple install methods",
			metadata: []*proto.UpstreamInventoryAgentMetadata{
				{InstallMethods: []string{"dockerfile", "helm_kube_agent"}},
				{InstallMethods: []string{"helm_kube_agent", "dockerfile"}},
			},
			expected: map[string]int{
				"dockerfile,helm_kube_agent": 2,
			},
		},
	}
	for _, tt := range tts {
		t.Run(tt.desc, func(t *testing.T) {
			periodic := newInstanceMetricsPeriodic()

			for _, metadata := range tt.metadata {
				periodic.VisitInstance(new(proto.UpstreamInventoryHello), metadata)
			}
			require.Equal(t, tt.expected, periodic.InstallMethodCounts(), "tt=%q", tt.desc)
		})
	}
}

func TestRegisteredAgentsCounts(t *testing.T) {
	tts := []struct {
		desc     string
		instance []*proto.UpstreamInventoryHello
		expected map[registeredAgent]int
	}{
		{
			desc:     "none",
			instance: []*proto.UpstreamInventoryHello{},
			expected: map[registeredAgent]int{},
		},
		{
			desc: "automatic updates disabled",
			instance: []*proto.UpstreamInventoryHello{
				{Version: "13.0.0"},
				{Version: "14.0.0"},
				{Version: "15.0.0"},
			},
			expected: map[registeredAgent]int{
				{"13.0.0", "false"}: 1,
				{"14.0.0", "false"}: 1,
				{"15.0.0", "false"}: 1,
			},
		},
		{
			desc: "automatic updates enabled",
			instance: []*proto.UpstreamInventoryHello{
				{Version: "13.0.0", ExternalUpgrader: "unit"},
				{Version: "13.0.0", ExternalUpgrader: "kube"},
				{Version: "13.0.0"},
				{Version: "14.0.0", ExternalUpgrader: "unit"},
				{Version: "14.0.0", ExternalUpgrader: "kube"},
				{Version: "14.0.0"},
				{Version: "15.0.0", ExternalUpgrader: "unit"},
				{Version: "15.0.0", ExternalUpgrader: "kube"},
				{Version: "15.0.0"},
			},
			expected: map[registeredAgent]int{
				{"13.0.0", "true"}:  2,
				{"13.0.0", "false"}: 1,
				{"14.0.0", "true"}:  2,
				{"14.0.0", "false"}: 1,
				{"15.0.0", "true"}:  2,
				{"15.0.0", "false"}: 1,
			},
		},
	}
	for _, tt := range tts {
		t.Run(tt.desc, func(t *testing.T) {
			periodic := newInstanceMetricsPeriodic()

			for _, instance := range tt.instance {
				periodic.VisitInstance(instance, new(proto.UpstreamInventoryAgentMetadata))
			}
			require.Equal(t, tt.expected, periodic.RegisteredAgentsCount(), "tt=%q", tt.desc)
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
				for range count {
					instance, err := types.NewInstance(uuid.New().String(), types.InstanceSpecV1{
						Version:          ver,
						ExternalUpgrader: "some-upgrader",
					})
					require.NoError(t, err)

					periodic.VisitInstance(instance)
				}
			}

			for ver, count := range tt.unenrolled {
				for range count {
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
