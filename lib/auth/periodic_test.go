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
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/client/proto"
)

func TestTotalInstances(t *testing.T) {
	instances := []proto.UpstreamInventoryHello{
		{},
		{Version: "15.0.0"},
		{ServerID: "id"},
		{ExternalUpgrader: "kube"},
		{ExternalUpgraderVersion: "14.0.0"},
	}

	periodic := newInstanceMetricsPeriodic()
	for _, instance := range instances {
		periodic.VisitInstance(instance, proto.UpstreamInventoryAgentMetadata{})
	}

	require.Equal(t, 5, periodic.TotalInstances())
}

func TestTotalEnrolledInUpgrades(t *testing.T) {
	tts := []struct {
		desc      string
		instances []proto.UpstreamInventoryHello
		expected  int
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
			expected: 4,
		},
		{
			desc: "version omitted",
			instances: []proto.UpstreamInventoryHello{
				{ExternalUpgrader: "kube"},
				{ExternalUpgrader: "unit"},
			},
			expected: 2,
		},
		{
			desc: "all-unenrolled",
			instances: []proto.UpstreamInventoryHello{
				{},
				{},
			},
			expected: 0,
		},
		{
			desc:      "none",
			instances: []proto.UpstreamInventoryHello{},
			expected:  0,
		},
	}
	for _, tt := range tts {
		t.Run(tt.desc, func(t *testing.T) {
			periodic := newInstanceMetricsPeriodic()
			for _, instance := range tt.instances {
				periodic.VisitInstance(instance, proto.UpstreamInventoryAgentMetadata{})
			}
			require.Equal(t, tt.expected, periodic.TotalEnrolledInUpgrades(), "tt=%q", tt.desc)
		})
	}
}

func TestUpgraderCounts(t *testing.T) {
	tts := []struct {
		desc      string
		instances []proto.UpstreamInventoryHello
		expected  map[upgrader]int
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
			expected: map[upgrader]int{
				{"kube", "13.0.0"}: 1,
				{"kube", "14.0.0"}: 1,
				{"unit", "13.0.0"}: 1,
				{"unit", "14.0.0"}: 1,
			},
		},
		{
			desc: "all-unenrolled",
			instances: []proto.UpstreamInventoryHello{
				{},
				{},
			},
			expected: map[upgrader]int{},
		},
		{
			desc: "all-enrolled",
			instances: []proto.UpstreamInventoryHello{
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
			instances: []proto.UpstreamInventoryHello{
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
				periodic.VisitInstance(instance, proto.UpstreamInventoryAgentMetadata{})
			}
			require.Equal(t, tt.expected, periodic.UpgraderCounts(), "tt=%q", tt.desc)
		})
	}
}

func TestInstallMethodCounts(t *testing.T) {
	tts := []struct {
		desc     string
		metadata []proto.UpstreamInventoryAgentMetadata
		expected map[string]int
	}{
		{
			desc:     "none",
			metadata: []proto.UpstreamInventoryAgentMetadata{},
			expected: map[string]int{},
		},
		{
			desc: "unknown install method",
			metadata: []proto.UpstreamInventoryAgentMetadata{
				{},
			},
			expected: map[string]int{
				"unknown": 1,
			},
		},
		{
			desc: "various install methods",
			metadata: []proto.UpstreamInventoryAgentMetadata{
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
			metadata: []proto.UpstreamInventoryAgentMetadata{
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
				periodic.VisitInstance(proto.UpstreamInventoryHello{}, metadata)
			}
			require.Equal(t, tt.expected, periodic.InstallMethodCounts(), "tt=%q", tt.desc)
		})
	}
}

func TestRegisteredAgentsCounts(t *testing.T) {
	tts := []struct {
		desc     string
		instance []proto.UpstreamInventoryHello
		expected map[registeredAgent]int
	}{
		{
			desc:     "none",
			instance: []proto.UpstreamInventoryHello{},
			expected: map[registeredAgent]int{},
		},
		{
			desc: "automatic updates disabled",
			instance: []proto.UpstreamInventoryHello{
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
			instance: []proto.UpstreamInventoryHello{
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
				periodic.VisitInstance(instance, proto.UpstreamInventoryAgentMetadata{})
			}
			require.Equal(t, tt.expected, periodic.RegisteredAgentsCount(), "tt=%q", tt.desc)
		})
	}
}

func TestUpgradeEnrollPeriodic(t *testing.T) {
	tts := []struct {
		desc           string
		enrolled       map[string]int
		unenrolled     map[string]int
		generatePrompt bool
	}{
		{
			desc: "enrolled in auto updates and up to date",
			enrolled: map[string]int{
				"v15.4.4": 5,
			},
			generatePrompt: false,
		},
		{
			desc: "unenrolled in auto updates and up to date",
			unenrolled: map[string]int{
				"v15.4.4": 5,
			},
			generatePrompt: false,
		},
		{
			desc: "enrolled in auto updates and out dated",
			enrolled: map[string]int{
				"v14.3.20": 5,
			},
			generatePrompt: false,
		},
		{
			desc: "unenrolled in auto updates and out dated",
			unenrolled: map[string]int{
				"v14.3.20": 5,
			},
			generatePrompt: true,
		},
		{
			desc: "mixed with all up to date",
			enrolled: map[string]int{
				"v15.4.4": 5,
			},
			unenrolled: map[string]int{
				"v15.4.4": 5,
			},
			generatePrompt: false,
		},
		{
			desc: "mixed with all out dated",
			enrolled: map[string]int{
				"v14.3.20": 5,
			},
			unenrolled: map[string]int{
				"v14.3.20": 5,
			},
			generatePrompt: true,
		},
		{
			desc: "mixed with all enrolled in auto updates",
			enrolled: map[string]int{
				"v15.4.4":  5,
				"v14.3.20": 5,
			},
			generatePrompt: false,
		},
		{
			desc:           "no-instances",
			generatePrompt: false,
		},
	}

	for _, tt := range tts {
		t.Run(tt.desc, func(t *testing.T) {
			periodic := &upgradeEnrollPeriodic{
				authVersion: "v15.4.7",
				enrolled:    make(map[string]int),
				unenrolled:  make(map[string]int),
			}

			for ver, count := range tt.enrolled {
				for i := 0; i < count; i++ {
					instance := proto.UpstreamInventoryHello{
						Version:          ver,
						ExternalUpgrader: "test-upgrader",
					}
					periodic.VisitInstance(instance)
				}
			}

			for ver, count := range tt.unenrolled {
				for i := 0; i < count; i++ {
					instance := proto.UpstreamInventoryHello{
						Version: ver,
					}

					periodic.VisitInstance(instance)
				}
			}

			_, ok := periodic.GenerateEnrollPrompt()
			require.Equal(t, tt.generatePrompt, ok)
		})
	}
}
