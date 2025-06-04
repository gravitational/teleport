/*
Copyright 2025 Gravitational, Inc.

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

package types

import (
	"fmt"
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGroupByTargetHealth(t *testing.T) {
	t.Parallel()
	statuses := []TargetHealthStatus{
		TargetHealthStatusHealthy,
		TargetHealthStatusUnknown,
		TargetHealthStatusUnhealthy,
		"", // older agents dont set status
	}

	var servers []DatabaseServer
	for _, status := range statuses {
		for range 10 {
			name := fmt.Sprintf("db-%d", len(servers))
			server, err := NewDatabaseServerV3(Metadata{
				Name: name,
			}, DatabaseServerSpecV3{
				HostID:   "_",
				Hostname: "_",
				Database: &DatabaseV3{
					Metadata: Metadata{
						Name: name,
					},
					Spec: DatabaseSpecV3{
						Protocol: "_",
						URI:      "_",
						AWS: AWS{
							Redshift: Redshift{
								ClusterID: "_",
							},
						},
					},
				},
			})
			require.NoError(t, err)
			server.SetTargetHealth(TargetHealth{Status: string(status)})
			servers = append(servers, server)
		}
	}
	rand.Shuffle(len(servers), func(i, j int) {
		servers[i], servers[j] = servers[j], servers[i]
	})
	groups := GroupByTargetHealthStatus(servers)
	for _, server := range groups.Healthy {
		require.Equal(t, TargetHealthStatusHealthy,
			server.GetTargetHealthStatus(),
			"server %s is in the wrong group", server.GetName(),
		)
	}
	for _, server := range groups.Unhealthy {
		require.Equal(t, TargetHealthStatusUnhealthy,
			server.GetTargetHealthStatus(),
			"server %s is in the wrong group", server.GetName(),
		)
	}
	for _, server := range groups.Unknown {
		require.Contains(t, []TargetHealthStatus{TargetHealthStatusUnknown, ""},
			server.GetTargetHealthStatus(),
			"server %s is in the wrong group", server.GetName(),
		)
	}
}

func TestTargetHealthStatusCanonical(t *testing.T) {
	tests := []struct {
		name     string
		input    TargetHealthStatus
		expected TargetHealthStatus
	}{
		{"healthy remains healthy", TargetHealthStatusHealthy, TargetHealthStatusHealthy},
		{"unhealthy remains unhealthy", TargetHealthStatusUnhealthy, TargetHealthStatusUnhealthy},
		{"unknown becomes unknown", TargetHealthStatusUnknown, TargetHealthStatusUnknown},
		{"mixed becomes unknown", TargetHealthStatusMixed, TargetHealthStatusUnknown},
		{"empty string becomes unknown", TargetHealthStatus(""), TargetHealthStatusUnknown},
		{"random string becomes unknown", TargetHealthStatus("invalid"), TargetHealthStatusUnknown},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual := test.input.Canonical()
			require.Equal(t, test.expected, actual)
		})
	}
}

func TestTargetHealthStatusesAggregate(t *testing.T) {
	tests := []struct {
		name     string
		input    []TargetHealthStatus
		expected TargetHealthStatus
	}{
		{"empty list returns unknown", []TargetHealthStatus{}, TargetHealthStatusUnknown},
		{"one healthy", []TargetHealthStatus{TargetHealthStatusHealthy}, TargetHealthStatusHealthy},
		{"one unhealthy", []TargetHealthStatus{TargetHealthStatusUnhealthy}, TargetHealthStatusUnhealthy},
		{"one unknown", []TargetHealthStatus{TargetHealthStatusUnknown}, TargetHealthStatusUnknown},
		{"one mixed", []TargetHealthStatus{TargetHealthStatusMixed}, TargetHealthStatusUnknown},
		{"all healthy", []TargetHealthStatus{TargetHealthStatusHealthy, TargetHealthStatusHealthy}, TargetHealthStatusHealthy},
		{"all unhealthy", []TargetHealthStatus{TargetHealthStatusUnhealthy, TargetHealthStatusUnhealthy}, TargetHealthStatusUnhealthy},
		{"all unknown", []TargetHealthStatus{TargetHealthStatusUnknown, TargetHealthStatusUnknown}, TargetHealthStatusUnknown},
		{"all empty", []TargetHealthStatus{"", ""}, TargetHealthStatusUnknown},
		{"empty and unknown", []TargetHealthStatus{"", TargetHealthStatusUnknown}, TargetHealthStatusUnknown},
		{"healthy and unhealthy", []TargetHealthStatus{TargetHealthStatusHealthy, TargetHealthStatusUnhealthy}, TargetHealthStatusMixed},
		{"unhealthy and unknown", []TargetHealthStatus{TargetHealthStatusUnhealthy, TargetHealthStatusUnknown}, TargetHealthStatusMixed},
		{"healthy and unhealthy and unknown", []TargetHealthStatus{TargetHealthStatusHealthy, TargetHealthStatusUnhealthy, TargetHealthStatusUnknown}, TargetHealthStatusMixed},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual := AggregateHealthStatus(slices.Values(test.input))
			require.Equal(t, test.expected, actual)
		})
	}
}
