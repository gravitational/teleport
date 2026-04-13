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

func (a *awsInfo) cloudName() string       { return cloudAWS }
func (a *awsInfo) cloudInstanceID() string { return a.InstanceID }
func (a *awsInfo) cloudAccountID() string  { return a.AccountID }

// correlateSSMEvents processes SSM run events into the instances map.
// Events must be in descending order (most recent first); only the first
// event per instance ID is kept.
func correlateSSMEvents(instances map[string]*instanceInfo, events []*apievents.SSMRun) {
	for _, run := range events {
		if run.InstanceID == "" {
			continue
		}
		if _, ok := instances[run.InstanceID]; ok {
			continue // events are most-recent-first; keep only the latest
		}
		instances[run.InstanceID] = &instanceInfo{
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
}

// correlateNodes merges online Teleport nodes into the instances map. For nodes
// that already have an entry (from audit events), it sets IsOnline and fills in
// any missing region/account fields. For nodes with no prior entry, it creates
// a new one with only online status and cloud metadata.
func correlateNodes(instances map[string]*instanceInfo, nodes []types.Server) {
	for _, node := range nodes {
		// TODO(Tener): handle Azure and GCP nodes once CloudMetadata supports them.

		id := node.GetAWSInstanceID()
		if id == "" {
			continue
		}
		info, ok := instances[id]
		if !ok {
			info = &instanceInfo{}
			instances[id] = info
		}
		info.IsOnline = true

		if info.AWS == nil {
			info.AWS = &awsInfo{InstanceID: id}
		}
		if aws := node.GetAWSInfo(); aws != nil {
			if info.Region == "" {
				info.Region = aws.Region
			}
		}
		if info.AWS.AccountID == "" {
			info.AWS.AccountID = node.GetAWSAccountID()
		}

		if !node.Expiry().IsZero() {
			info.Expiry = node.Expiry()
		}
	}
}
