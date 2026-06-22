// Teleport
// Copyright (C) 2026 Gravitational, Inc.
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

package discovery

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRenderSummaryText(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)

	t.Run("empty input shows no-configurations message", func(t *testing.T) {
		var buf bytes.Buffer
		require.NoError(t, renderSummaryText(&buf, nil, now))
		require.Equal(t, "No AWS or Azure discovery_config resources are configured.\nStatic discovery_service matchers from teleport.yaml do not report discovery config status.\n", buf.String())
	})

	t.Run("renders summaries", func(t *testing.T) {
		var buf bytes.Buffer
		lastRun := now.Add(-2 * time.Minute)
		require.NoError(t, renderSummaryText(&buf, []configSummary{
			{
				Name:           "aws-config",
				DiscoveryGroup: "prod",
				Status: configStatusSummary{
					label:   "healthy",
					lastRun: &lastRun,
				},
				Resources: []resourceSummary{{
					Cloud:        cloudAWS,
					ResourceType: "EC2",
					Integration:  "int1",
					Scopes: []resourceScope{
						{
							Regions:   []string{"eu-west-1", "eu-west-2"},
							MatchTags: []string{"env=prod"},
						},
					},
					LastSync: &lastRun,
					Result: summaryResult{
						Kind:     resultCounts,
						Found:    10,
						Enrolled: 8,
						Failed:   2,
					},
				}},
			},
			{
				Name:           "azure-config",
				DiscoveryGroup: "prod",
				Status: configStatusSummary{
					label: summaryStatusNotReporting,
				},
				Resources: []resourceSummary{{
					Cloud:        cloudAzure,
					ResourceType: "VM",
					Scopes: []resourceScope{
						{
							Regions:        []string{"eastus"},
							Subscriptions:  []string{"sub-1"},
							ResourceGroups: []string{"rg-1"},
							MatchTags:      []string{"all"},
						},
					},
					Result: summaryResult{Kind: resultNotReporting},
				}},
			},
		}, now))

		out := buf.String()
		require.Contains(t, out, "Discovery config status")
		require.Contains(t, out, "AWS EC2 discovery")
		require.Contains(t, out, "Config:")
		require.Contains(t, out, "aws-config")
		require.Contains(t, out, "Discovery group:")
		require.Contains(t, out, "prod")
		require.Contains(t, out, "Source:")
		require.Contains(t, out, "integration int1")
		require.Contains(t, out, "Regions:")
		require.Contains(t, out, "eu-west-1, eu-west-2")
		require.Contains(t, out, "Match tags:")
		require.Contains(t, out, "env=prod")
		require.Contains(t, out, "Status:")
		require.Contains(t, out, "healthy")
		require.Contains(t, out, "Last run:")
		require.Contains(t, out, "2 minutes ago")
		require.Contains(t, out, "Last resource sync:")
		require.Contains(t, out, "Result:")
		require.Contains(t, out, "10 found, 8 enrolled, 2 failed")
		require.Contains(t, out, "Discovery config status")
		require.Contains(t, out, "Azure VM discovery")
		require.Contains(t, out, "azure-config")
		require.Contains(t, out, "ambient credentials")
		require.Contains(t, out, "Subscriptions:")
		require.Contains(t, out, "sub-1")
		require.Contains(t, out, "Resource groups:")
		require.Contains(t, out, "rg-1")
		require.Contains(t, out, "no status reported by a Discovery Service")
		require.NotContains(t, out, "DiscoveryGroup")
		require.NotContains(t, out, "DISCOVERY_CONFIG_STATE_UNSPECIFIED")
	})

	t.Run("renders distinct matcher scopes", func(t *testing.T) {
		var buf bytes.Buffer
		require.NoError(t, renderSummaryText(&buf, []configSummary{
			{
				Name:           "aws-config",
				DiscoveryGroup: "prod",
				Status: configStatusSummary{
					label: "healthy",
				},
				Resources: []resourceSummary{{
					Cloud:        cloudAWS,
					ResourceType: "EC2",
					Integration:  "int1",
					Scopes: []resourceScope{
						{
							Regions:   []string{"eu-west-1"},
							MatchTags: []string{"env=prod"},
						},
						{
							Regions:   []string{"eu-west-2"},
							MatchTags: []string{"env=staging"},
						},
					},
					Result: summaryResult{
						Kind:     resultCounts,
						Found:    10,
						Enrolled: 8,
						Failed:   2,
					},
				}},
			},
		}, now))

		out := buf.String()
		require.Contains(t, out, "Matcher scopes:")
		require.Contains(t, out, "- Regions: eu-west-1; Match tags: env=prod")
		require.Contains(t, out, "- Regions: eu-west-2; Match tags: env=staging")
		require.NotContains(t, out, "eu-west-1, eu-west-2")
		require.NotContains(t, out, "env=prod, env=staging")
	})

	t.Run("renders config error separately from resource result", func(t *testing.T) {
		var buf bytes.Buffer
		lastRun := now.Add(-5 * time.Minute)
		require.NoError(t, renderSummaryText(&buf, []configSummary{
			{
				Name:           "aws-config",
				DiscoveryGroup: "prod",
				Status: configStatusSummary{
					label:        "error",
					lastRun:      &lastRun,
					errorMessage: "AccessDenied: missing ec2:DescribeInstances permission",
				},
				Resources: []resourceSummary{{
					Cloud:        cloudAWS,
					ResourceType: "EC2",
					Scopes: []resourceScope{
						{
							Regions:   []string{"eu-west-1"},
							MatchTags: []string{"env=prod"},
						},
					},
					Result: summaryResult{Kind: resultNoResourceStatus},
				}},
			},
		}, now))

		out := buf.String()
		require.Contains(t, out, "Discovery config status")
		require.Contains(t, out, "Config:")
		require.Contains(t, out, "aws-config")
		require.Contains(t, out, "AWS EC2 discovery")
		require.Contains(t, out, "Status:")
		require.Contains(t, out, "error")
		require.Contains(t, out, "5 minutes ago")
		require.Contains(t, out, "AccessDenied: missing ec2:DescribeInstances permission")
		require.Equal(t, 1, strings.Count(out, "AccessDenied: missing ec2:DescribeInstances permission"))
		require.Contains(t, out, "no resource status reported for this discovery target")
	})
}
