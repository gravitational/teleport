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
	"maps"
	"slices"

	usertasksv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/usertasks/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	libevents "github.com/gravitational/teleport/lib/events"
)

const cloudAWS = "AWS"

// awsInfo holds AWS-specific instance metadata.
type awsInfo struct {
	InstanceID string `json:"instance_id"`
	AccountID  string `json:"account_id"`
}

func (a *awsInfo) cloudName() string      { return cloudAWS }
func (a *awsInfo) cloudAccountID() string { return a.AccountID }
func (a *awsInfo) instanceText() string   { return a.InstanceID }

// correlateSSMEvents builds an instance map from SSM run events.
// Events must be in descending order (most recent first); only the first
// event per instance ID is kept.
func correlateSSMEvents(events []*apievents.SSMRun) map[string]instanceInfo {
	instances := make(map[string]instanceInfo)
	for _, run := range events {
		if run.InstanceID == "" {
			continue
		}
		if _, ok := instances[run.InstanceID]; ok {
			continue // events are most-recent-first; keep only the latest
		}
		instances[run.InstanceID] = instanceInfo{
			Region: run.Region,
			AWS: &awsInfo{
				InstanceID: run.InstanceID,
				AccountID:  run.AccountID,
			},
			RunResult: &runResult{
				Time:      run.Time,
				ExitCode:  run.ExitCode,
				Output:    combineOutput(run.StandardOutput, run.StandardError),
				IsFailure: run.Code == libevents.SSMRunFailCode,
			},
		}
	}
	return instances
}

// correlateAWSNodes builds an instance map from online AWS-discovered Teleport
// nodes. Nodes without an AWS instance ID label are not AWS-discovered and are
// skipped. First node wins if two nodes share an instance ID.
func correlateAWSNodes(nodes []types.Server) map[string]instanceInfo {
	instances := make(map[string]instanceInfo)
	for _, node := range nodes {
		id := node.GetAWSInstanceID()
		if id == "" {
			continue
		}
		info := instanceInfo{
			IsOnline: true,
			AWS:      &awsInfo{InstanceID: id, AccountID: node.GetAWSAccountID()},
		}
		if aws := node.GetAWSInfo(); aws != nil {
			info.Region = aws.Region
		}
		if !node.Expiry().IsZero() {
			info.Expiry = node.Expiry()
		}
		instances[id] = info
	}
	return instances
}

func awsTaskInstanceKeys(task *usertasksv1.UserTask) []string {
	if instanceGroup := task.GetSpec().GetDiscoverEc2(); instanceGroup != nil {
		return slices.Collect(maps.Keys(instanceGroup.GetInstances()))
	}
	return nil
}
