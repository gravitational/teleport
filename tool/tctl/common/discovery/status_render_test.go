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

	discoveryconfigv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/discoveryconfig/v1"
)

func TestRenderSummaryText(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)

	t.Run("empty input shows no-configurations message", func(t *testing.T) {
		var buf bytes.Buffer
		require.NoError(t, discoverySummary(nil).renderText(&buf, now))
		require.Equal(t, "No discovery_config resources are configured.\n", buf.String())
	})

	t.Run("renders no-service warning", func(t *testing.T) {
		lastRun := now.Add(-5 * time.Minute)
		var buf bytes.Buffer
		require.NoError(t, discoverySummary{
			{
				Name:           "empty-config",
				DiscoveryGroup: "prod",
				State:          discoveryconfigv1.DiscoveryConfigState_DISCOVERY_CONFIG_STATE_ERROR.String(),
				ErrorMessage:   "AccessDenied: missing ec2:DescribeInstances permission",
				LastSyncTime:   &lastRun,
			},
		}.renderText(&buf, now))

		out := buf.String()
		requireNoTrailingWhitespace(t, out)
		require.Equal(t, `Discovery config empty-config:
  Discovery group: prod
  Status: error
  Last run: 5 minutes ago
  Error: AccessDenied: missing ec2:DescribeInstances permission
  No Discovery Services running for prod. See https://goteleport.com/docs/reference/deployment/config/#discovery-service.
`, out)
	})

	t.Run("renders service hierarchy", func(t *testing.T) {
		out := renderMultiServerSummary(t, now)
		requireNoTrailingWhitespace(t, out)
		require.Equal(t, `Discovery config multi-config:
  Discovery group: prod
  Status: healthy
  Last run: 2 minutes ago

  Service (server-a):
    Poll interval: 5 minutes
    Last update: 1 minute ago
    ambient credentials:
      AWS EC2:
        Previous sync: 4 minutes ago (took 12s)
        Result: 10 found, 8 enrolled, 2 failed
      AWS RDS:
        Previous sync: 3 minutes ago (took 30s)
        Result: 5 found, 4 enrolled, 1 failed
    aws-prod:
      AWS EC2:
        Previous sync: 2 minutes ago (took 30s)
        Result: 8 found, 7 enrolled, 1 failed
      AWS RDS:
        Previous sync: 1 minute ago (took 30s)
        Result: 6 found, 6 enrolled, 0 failed
  Service (server-b):
    Poll interval: 10 minutes
    Last update: 3 minutes ago
    ambient credentials:
      AWS RDS:
        Previous sync: 4 minutes ago (took 30s)
        Result: 3 found, 2 enrolled, 1 failed
      Azure VM:
        Previous sync: 3 minutes ago (took 30s)
        Result: 7 found, 6 enrolled, 1 failed
    azure-prod:
      Azure VM:
        Previous sync: 2 minutes ago (took 30s)
        Result: 4 found, 4 enrolled, 0 failed
`, out)
		require.Contains(t, out, "\n  Last run: 2 minutes ago\n\n  Service (server-a):")
		require.Contains(t, out, "\n        Result: 10 found, 8 enrolled, 2 failed")
		require.NotContains(t, out, "AWS EKS:")
		requireResultIndentation(t, out)
	})
}

func renderMultiServerSummary(t *testing.T, now time.Time) string {
	t.Helper()

	roundsToDisplayDuration := 11*time.Second + 832865310*time.Nanosecond
	var buf bytes.Buffer
	require.NoError(t, discoverySummary{
		{
			Name:           "multi-config",
			DiscoveryGroup: "prod",
			State:          discoveryconfigv1.DiscoveryConfigState_DISCOVERY_CONFIG_STATE_RUNNING.String(),
			LastSyncTime:   ptrTo(now.Add(-2 * time.Minute)),
			Servers: []serverSummary{
				{
					ServerID:     "server-a",
					PollInterval: "5m0s",
					LastUpdate:   ptrTo(now.Add(-time.Minute)),
					Integrations: []integrationSummary{
						{
							Resources: []resourceResult{
								testResourceResult(resourceKindAWSEC2, 10, 8, 2, now.Add(-4*time.Minute-roundsToDisplayDuration), now.Add(-4*time.Minute)),
								testResourceResult(resourceKindAWSRDS, 5, 4, 1, now.Add(-3*time.Minute-30*time.Second), now.Add(-3*time.Minute)),
							},
						},
						{
							Integration: "aws-prod",
							Resources: []resourceResult{
								testResourceResult(resourceKindAWSEC2, 8, 7, 1, now.Add(-2*time.Minute-30*time.Second), now.Add(-2*time.Minute)),
								testResourceResult(resourceKindAWSRDS, 6, 6, 0, now.Add(-time.Minute-30*time.Second), now.Add(-time.Minute)),
							},
						},
					},
				},
				{
					ServerID:     "server-b",
					PollInterval: "10m0s",
					LastUpdate:   ptrTo(now.Add(-3 * time.Minute)),
					Integrations: []integrationSummary{
						{
							Resources: []resourceResult{
								testResourceResult(resourceKindAWSRDS, 3, 2, 1, now.Add(-4*time.Minute-30*time.Second), now.Add(-4*time.Minute)),
								testResourceResult(resourceKindAzureVM, 7, 6, 1, now.Add(-3*time.Minute-30*time.Second), now.Add(-3*time.Minute)),
							},
						},
						{
							Integration: "azure-prod",
							Resources: []resourceResult{
								testResourceResult(resourceKindAzureVM, 4, 4, 0, now.Add(-2*time.Minute-30*time.Second), now.Add(-2*time.Minute)),
							},
						},
					},
				},
			},
		},
	}.renderText(&buf, now))
	return buf.String()
}

func testResourceResult(kind string, found, enrolled, failed uint64, syncStart, syncEnd time.Time) resourceResult {
	return resourceResult{
		Kind:      kind,
		Found:     found,
		Enrolled:  enrolled,
		Failed:    failed,
		SyncStart: &syncStart,
		SyncEnd:   &syncEnd,
	}
}

func requireResultIndentation(t *testing.T, out string) {
	t.Helper()

	var sawService bool
	for _, line := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		if strings.HasPrefix(line, "  Service (") {
			sawService = true
		}
		if strings.Contains(line, "Result:") {
			require.True(t, sawService, "result line rendered before service: %q", line)
			require.True(t, strings.HasPrefix(line, "        Result:"), "result line has wrong indentation: %q", line)
		}
	}
}

func requireNoTrailingWhitespace(t *testing.T, out string) {
	t.Helper()
	for _, line := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		require.False(t, strings.HasSuffix(line, " "), "trailing whitespace: %q", line)
	}
}
