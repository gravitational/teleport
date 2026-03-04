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
	"cmp"
	"context"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	libevents "github.com/gravitational/teleport/lib/events"
)

// discoveryClient abstracts the auth client methods used by discovery commands.
type discoveryClient interface {
	// SearchEvents searches audit events.
	SearchEvents(ctx context.Context, req libevents.SearchEventsRequest) ([]apievents.AuditEvent, string, error)
	// GetResources lists resources with pagination.
	GetResources(ctx context.Context, req *proto.ListResourcesRequest) (*proto.ListResourcesResponse, error)
}

// ssmResult captures the most recent SSM run result for an instance.
type ssmResult struct {
	ExitCode  int64     `json:"exit_code"`
	Output    string    `json:"output"`
	Time      time.Time `json:"time"`
	IsFailure bool      `json:"is_failure"`
}

// instanceInfo is one row in the inventory report. Instances with no failures
// have nil SSMResult.
type instanceInfo struct {
	InstanceID string     `json:"instance_id"`
	Region     string     `json:"region"`
	AccountID  string     `json:"account_id"`
	IsOnline   bool       `json:"is_online"`
	Expiry     time.Time  `json:"expiry,omitempty"`
	SSMResult  *ssmResult `json:"ssm_result,omitempty"`
}

// lastTime returns the most recent failure timestamp formatted as RFC3339.
// For instances with no failures, falls back to the node expiry time.
func (f instanceInfo) lastTime() string {
	var t time.Time
	if f.SSMResult != nil {
		t = f.SSMResult.Time
	}
	if t.IsZero() {
		t = f.Expiry
	}
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}

// ssmOutput returns the SSM stdout/stderr with newlines escaped for
// single-line display.
func (f instanceInfo) ssmOutput() string {
	if f.SSMResult == nil {
		return ""
	}
	out := strings.TrimSpace(f.SSMResult.Output)
	if out == "" {
		return ""
	}
	return fmt.Sprintf("%q", out)
}

// result returns a compact human-readable result string for the SSM run.
func (f instanceInfo) result() string {
	if f.SSMResult == nil {
		return ""
	}
	return fmt.Sprintf("exit=%d", f.SSMResult.ExitCode)
}

// buildInventory fetches audit events and nodes, then correlates them
// into a list of all discovered instances.
func buildInventory(ctx context.Context, clt discoveryClient, from, to time.Time) ([]instanceInfo, error) {
	slog.DebugContext(ctx, "Fetching SSM run events")
	ssmEvents, err := getSSMRunEvents(ctx, clt, from, to)
	if err != nil {
		return nil, trace.Wrap(err, "fetching SSM run events")
	}
	slog.DebugContext(ctx, "Fetched SSM run events", "count", len(ssmEvents))

	slog.DebugContext(ctx, "Fetching online nodes")
	nodes, err := client.GetAllResources[types.Server](ctx, clt, &proto.ListResourcesRequest{
		ResourceType: types.KindNode,
		Namespace:    apidefaults.Namespace,
	})
	if err != nil {
		return nil, trace.Wrap(err, "fetching nodes")
	}
	slog.DebugContext(ctx, "Fetched online nodes", "count", len(nodes))

	return correlate(ssmEvents, nodes), nil
}

// getSSMRunEvents pages through all SSM run audit events, collecting every
// page. Events are returned in descending order (most recent first).
func getSSMRunEvents(ctx context.Context, clt discoveryClient, from, to time.Time) ([]*apievents.SSMRun, error) {
	var out []*apievents.SSMRun

	const pageSize = 1000

	req := libevents.SearchEventsRequest{
		From:       from,
		To:         to,
		EventTypes: []string{libevents.SSMRunEvent},
		Order:      types.EventOrderDescending,
		Limit:      pageSize,
	}
	for {
		page, nextKey, err := clt.SearchEvents(ctx, req)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		for _, ev := range page {
			if run, ok := ev.(*apievents.SSMRun); ok {
				out = append(out, run)
			}
		}

		if nextKey == "" || len(page) == 0 {
			break
		}

		req.StartKey = nextKey
	}

	return out, nil
}

// correlate builds the instance inventory from SSM events and online nodes.
// All instances seen in any event or node list are included.
func correlate(ssmEvents []*apievents.SSMRun, nodes []types.Server) []instanceInfo {
	type instanceData struct {
		region    string
		accountID string
		expiry    time.Time
		ssm       *ssmResult
	}
	instances := make(map[string]*instanceData)

	getOrCreate := func(id, region, account string) *instanceData {
		d, ok := instances[id]
		if !ok {
			d = &instanceData{region: region, accountID: account}
			instances[id] = d
		}
		if d.region == "" && region != "" {
			d.region = region
		}
		if d.accountID == "" && account != "" {
			d.accountID = account
		}
		return d
	}

	// Process all SSM events; keep the most recent per instance (events are desc-ordered).
	for _, run := range ssmEvents {
		if run.InstanceID == "" {
			continue
		}
		d := getOrCreate(run.InstanceID, run.Region, run.AccountID)
		if d.ssm == nil {
			d.ssm = &ssmResult{
				ExitCode:  run.ExitCode,
				Output:    combineOutput(run.StandardOutput, run.StandardError),
				Time:      run.Time.UTC(),
				IsFailure: isSSMFailure(run),
			}
		}
	}

	// Include online nodes; enrich with region/account from cloud metadata.
	onlineInstances := make(map[string]bool)
	for _, node := range nodes {
		id := node.GetAWSInstanceID()
		if id == "" {
			continue
		}
		onlineInstances[id] = true
		var region string
		if aws := node.GetAWSInfo(); aws != nil {
			region = aws.Region
		}
		d := getOrCreate(id, region, node.GetAWSAccountID())
		if !node.Expiry().IsZero() {
			d.expiry = node.Expiry()
		}
	}

	// Build result.
	result := make([]instanceInfo, 0, len(instances))
	for id, d := range instances {
		result = append(result, instanceInfo{
			InstanceID: id,
			Region:     d.region,
			AccountID:  d.accountID,
			IsOnline:   onlineInstances[id],
			Expiry:     d.expiry,
			SSMResult:  d.ssm,
		})
	}

	// Sort by instance ID for stable output.
	slices.SortFunc(result, func(a, b instanceInfo) int {
		return cmp.Compare(a.InstanceID, b.InstanceID)
	})
	return result
}

// isSSMFailure returns true if the SSM run event represents a failure.
// A run is a failure when:
//   - the event code is "TDS00W" (SSMRunFailCode), OR
//   - the status field is non-empty and not "Success".
func isSSMFailure(run *apievents.SSMRun) bool {
	if strings.EqualFold(strings.TrimSpace(run.Code), libevents.SSMRunFailCode) {
		return true
	}
	status := strings.TrimSpace(run.Status)
	if status == "" {
		return false
	}
	return !strings.EqualFold(status, "Success")
}

// combineOutput joins stdout and stderr with a newline separator.
func combineOutput(stdout, stderr string) string {
	stdout = strings.TrimSpace(stdout)
	stderr = strings.TrimSpace(stderr)
	if stdout != "" && stderr != "" {
		return stdout + "\n" + stderr
	}
	if stdout != "" {
		return stdout
	}
	return stderr
}

// resolveTimeRange parses a --last duration string into a (from, to) pair.
func resolveTimeRange(clock clockwork.Clock, last string) (from, to time.Time, err error) {
	now := clock.Now().UTC()
	d, err := time.ParseDuration(strings.TrimSpace(last))
	if err != nil {
		return time.Time{}, time.Time{}, trace.BadParameter("invalid --last value %q", last)
	}
	return now.Add(-d), now, nil
}
