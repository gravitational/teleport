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
	"context"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	usertasksv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/usertasks/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	usertaskstypes "github.com/gravitational/teleport/api/types/usertasks"
	libevents "github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
)

// mockUserTasks implements services.UserTasks for testing.
type mockUserTasks struct {
	services.UserTasks
	tasks []*usertasksv1.UserTask
}

func (m *mockUserTasks) ListUserTasks(_ context.Context, _ int64, _ string, _ *usertasksv1.ListUserTasksFilters) ([]*usertasksv1.UserTask, string, error) {
	return m.tasks, "", nil
}

// mockClient implements discoveryClient for testing.
type mockClient struct {
	events             []apievents.AuditEvent
	nodes              []*types.ServerV2
	userTasks          []*usertasksv1.UserTask
	acceptedEventTypes []string
}

func (m *mockClient) SearchEvents(_ context.Context, req libevents.SearchEventsRequest) ([]apievents.AuditEvent, string, error) {
	if m.acceptedEventTypes != nil {
		for _, want := range req.EventTypes {
			if !slices.Contains(m.acceptedEventTypes, want) {
				return nil, "", fmt.Errorf("unexpected event type %q (accepted: %v)", want, m.acceptedEventTypes)
			}
		}
	}
	return m.events, "", nil
}

func (m *mockClient) UserTasksClient() services.UserTasks {
	return &mockUserTasks{tasks: m.userTasks}
}

func (m *mockClient) GetResources(_ context.Context, _ *proto.ListResourcesRequest) (*proto.ListResourcesResponse, error) {
	var resources []*proto.PaginatedResource
	for _, node := range m.nodes {
		resources = append(resources, &proto.PaginatedResource{
			Resource: &proto.PaginatedResource_Node{Node: node},
		})
	}
	return &proto.ListResourcesResponse{
		Resources: resources,
	}, nil
}

// newTestCommand creates a Command for testing.
func newTestCommand(format string) *Command {
	return &Command{
		nodesLast:   time.Hour,
		nodesFormat: format,
	}
}

func TestRunNodes(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		desc     string
		client   discoveryClient
		wantText string
		wantJSON string
	}{
		{
			desc: "SSM failures and successes",
			client: &mockClient{
				events: []apievents.AuditEvent{
					makeSSMRun("i-fail111", "111", "us-east-1", "Failed", 1, "install failed", now),
					makeSSMRun("i-success", "222", "us-west-2", "Success", 0, "", now),
				},
			},
			wantText: `Cloud Account Region    Instance  Time          Status        Details           
----- ------- --------- --------- ------------- ------------- ----------------- 
AWS   111     us-east-1 i-fail111 2026-01-15... Failed (ex... Script output:... 
AWS   222     us-west-2 i-success 2026-01-15... Installed ...                   
`,
			wantJSON: `[
    {
        "region": "us-east-1",
        "is_online": false,
        "run_result": {
            "api_error": "",
            "exit_code": 1,
            "output": "install failed",
            "time": "2026-01-15T12:00:00Z",
            "is_failure": true
        },
        "aws": {
            "instance_id": "i-fail111",
            "account_id": "111"
        }
    },
    {
        "region": "us-west-2",
        "is_online": false,
        "run_result": {
            "api_error": "",
            "exit_code": 0,
            "output": "",
            "time": "2026-01-15T12:00:00Z",
            "is_failure": false
        },
        "aws": {
            "instance_id": "i-success",
            "account_id": "222"
        }
    }
]
`,
		},
		{
			desc:   "empty result",
			client: &mockClient{},
			wantText: `No instances found.
`,
			wantJSON: `[]
`,
		},
		{
			desc: "online instance with SSM run failure",
			client: &mockClient{
				events: []apievents.AuditEvent{
					makeSSMRun("i-online1", "111", "us-east-1", "Failed", 1, "err", now),
				},
				nodes: []*types.ServerV2{makeNode("node-1", "i-online1", "111", "us-east-1", time.Time{})},
			},
			wantText: `Cloud Account Region    Instance  Time          Status        Details           
----- ------- --------- --------- ------------- ------------- ----------------- 
AWS   111     us-east-1 i-online1 2026-01-15... Online, ex... Script output:... 
`,
			wantJSON: `[
    {
        "region": "us-east-1",
        "is_online": true,
        "run_result": {
            "api_error": "",
            "exit_code": 1,
            "output": "err",
            "time": "2026-01-15T12:00:00Z",
            "is_failure": true
        },
        "aws": {
            "instance_id": "i-online1",
            "account_id": "111"
        }
    }
]
`,
		},
		{
			desc: "Azure run failure with online Azure node",
			client: &mockClient{
				events: []apievents.AuditEvent{
					makeAzureRun("vm-online", "sub-1", "rg-1", "eastus", azureExecStateFailed, 1, "err", "", now),
				},
				nodes: []*types.ServerV2{makeAzureNode("node-1", "vm-online", "sub-1", "rg-1", "eastus", time.Time{})},
			},
			wantText: `Cloud Account Region Instance      Time          Status        Details          
----- ------- ------ ------------- ------------- ------------- ---------------- 
Azure sub-1   eastus rg-1/vm-on... 2026-01-15... Online, ex... Script output... 
`,
			wantJSON: `[
    {
        "region": "eastus",
        "is_online": true,
        "run_result": {
            "api_error": "",
            "exit_code": 1,
            "output": "err",
            "time": "2026-01-15T12:00:00Z",
            "is_failure": true
        },
        "azure": {
            "vm_id": "vm-online",
            "subscription_id": "sub-1",
            "resource_group": "rg-1"
        }
    }
]
`,
		},
		{
			desc: "AWS instance with user task",
			client: &mockClient{
				events: []apievents.AuditEvent{
					makeSSMRun("i-bad", "111", "us-east-1", "Failed", 1, "install failed", now),
				},
				userTasks: []*usertasksv1.UserTask{
					makeEC2Task(t, usertaskstypes.AutoDiscoverEC2IssueSSMScriptFailure, "i-bad"),
				},
			},
			wantText: `Cloud Account Region    Instance Time          Status        Details            
----- ------- --------- -------- ------------- ------------- ------------------ 
AWS   111     us-east-1 i-bad    2026-01-15... Failed (ex... SSM Script fail... 
`,
			wantJSON: `[
    {
        "region": "us-east-1",
        "is_online": false,
        "run_result": {
            "api_error": "",
            "exit_code": 1,
            "output": "install failed",
            "time": "2026-01-15T12:00:00Z",
            "is_failure": true
        },
        "user_task_id": "07cccc8f-bb13-5f93-99d8-0ba51ca1da92",
        "user_task_issue": "ec2-ssm-script-failure",
        "aws": {
            "instance_id": "i-bad",
            "account_id": "111"
        }
    }
]
`,
		},
		{
			desc: "Azure VM with user task",
			client: &mockClient{
				events: []apievents.AuditEvent{
					makeAzureRun("vm-bad", "sub-1", "rg-1", "eastus", azureExecStateFailed, 1, "", "forbidden", now),
				},
				userTasks: []*usertasksv1.UserTask{
					makeAzureVMTask(t, usertaskstypes.AutoDiscoverAzureVMIssueEnrollmentError, "vm-bad"),
				},
			},
			wantText: `Cloud Account Region Instance      Time          Status        Details          
----- ------- ------ ------------- ------------- ------------- ---------------- 
Azure sub-1   eastus rg-1/vm-ba... 2026-01-15... Failed (AP... Enrollment fa... 
`,
			wantJSON: `[
    {
        "region": "eastus",
        "is_online": false,
        "run_result": {
            "api_error": "forbidden",
            "exit_code": 1,
            "output": "",
            "time": "2026-01-15T12:00:00Z",
            "is_failure": true
        },
        "user_task_id": "a8febb79-9ffd-519b-8be7-98e2fd80be86",
        "user_task_issue": "azure-vm-enrollment-error",
        "azure": {
            "vm_id": "vm-bad",
            "subscription_id": "sub-1",
            "resource_group": "rg-1"
        }
    }
]
`,
		},
		{
			desc: "Azure API failure",
			client: &mockClient{
				events: []apievents.AuditEvent{
					makeAzureRun("vm-offline", "sub-1", "rg-1", "eastus", azureExecStateFailed, 1, "", "forbidden", now),
				},
				nodes: []*types.ServerV2{makeAzureNode("node-1", "vm-online", "sub-1", "rg-1", "eastus", time.Time{})},
			},
			wantText: `Cloud Account Region Instance      Time          Status        Details          
----- ------- ------ ------------- ------------- ------------- ---------------- 
Azure sub-1   eastus rg-1/vm-of... 2026-01-15... Failed (AP... API error: "f... 
Azure sub-1   eastus rg-1/vm-on...               Online                         
`,
			wantJSON: `[
    {
        "region": "eastus",
        "is_online": false,
        "run_result": {
            "api_error": "forbidden",
            "exit_code": 1,
            "output": "",
            "time": "2026-01-15T12:00:00Z",
            "is_failure": true
        },
        "azure": {
            "vm_id": "vm-offline",
            "subscription_id": "sub-1",
            "resource_group": "rg-1"
        }
    },
    {
        "region": "eastus",
        "is_online": true,
        "azure": {
            "vm_id": "vm-online",
            "subscription_id": "sub-1",
            "resource_group": "rg-1"
        }
    }
]
`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			t.Run(teleport.Text, func(t *testing.T) {
				var buf bytes.Buffer
				c := newTestCommand(teleport.Text)
				require.NoError(t, c.runNodes(t.Context(), tt.client, &buf, time.Now().Add(-time.Hour), time.Now()))
				require.Equal(t, tt.wantText, buf.String())
			})
			t.Run(teleport.JSON, func(t *testing.T) {
				var buf bytes.Buffer
				c := newTestCommand(teleport.JSON)
				require.NoError(t, c.runNodes(t.Context(), tt.client, &buf, time.Now().Add(-time.Hour), time.Now()))
				require.Equal(t, tt.wantJSON, buf.String())
			})
		})
	}
}

func TestBuildNodes_CloudFilter(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		desc       string
		cfg        cloudProviderConfig
		accepted   []string
		wantClouds []string
	}{
		{
			desc:       "aws only",
			cfg:        cloudProviderConfig{aws: true},
			accepted:   []string{libevents.SSMRunEvent},
			wantClouds: []string{cloudAWS},
		},
		{
			desc:       "azure only",
			cfg:        cloudProviderConfig{azure: true},
			accepted:   []string{libevents.AzureRunEvent},
			wantClouds: []string{cloudAzure},
		},
		{
			desc:       "both clouds",
			cfg:        cloudProviderConfig{aws: true, azure: true},
			accepted:   []string{libevents.SSMRunEvent, libevents.AzureRunEvent},
			wantClouds: []string{cloudAWS, cloudAzure},
		},
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			client := &mockClient{
				events: []apievents.AuditEvent{
					makeSSMRun("i-1", "111", "us-east-1", "Success", 0, "", now),
					makeAzureRun("vm-1", "sub-1", "rg-1", "eastus", azureExecStateSucceeded, 0, "", "", now),
				},
				acceptedEventTypes: tt.accepted,
			}
			instances, err := buildNodes(t.Context(), client, now.Add(-time.Hour), now, tt.cfg)
			require.NoError(t, err)

			var clouds []string
			for _, inst := range instances {
				clouds = append(clouds, inst.cloud().cloudName())
			}
			require.Equal(t, tt.wantClouds, clouds)
		})
	}
}
