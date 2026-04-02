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
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	usertasksv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/usertasks/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	libevents "github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/service/servicecfg"
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
	events    []apievents.AuditEvent
	nodes     []types.Server
	userTasks []*usertasksv1.UserTask
}

func (m *mockClient) SearchEvents(_ context.Context, req libevents.SearchEventsRequest) ([]apievents.AuditEvent, string, error) {
	return m.events, "", nil
}

func (m *mockClient) UserTasksClient() services.UserTasks {
	return &mockUserTasks{tasks: m.userTasks}
}

func (m *mockClient) GetResources(_ context.Context, _ *proto.ListResourcesRequest) (*proto.ListResourcesResponse, error) {
	resources := make([]*proto.PaginatedResource, len(m.nodes))
	for i, n := range m.nodes {
		sv2, _ := n.(*types.ServerV2)
		resources[i] = &proto.PaginatedResource{
			Resource: &proto.PaginatedResource_Node{Node: sv2},
		}
	}
	return &proto.ListResourcesResponse{
		Resources: resources,
	}, nil
}

// newTestCommand creates a Command for testing.
func newTestCommand(format string) *Command {
	return &Command{
		config:      &servicecfg.Config{Clock: clockwork.NewRealClock()},
		nodesLast:   "1h",
		nodesFormat: format,
	}
}

func TestRunNodes(t *testing.T) {
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
			wantText: `Cloud Account Region    Instance ID Time          Status        Details       
----- ------- --------- ----------- ------------- ------------- ------------- 
AWS   111     us-east-1 i-fail111   2026-01-15... Failed (ex... Script out... 
AWS   222     us-west-2 i-success   2026-01-15... Installed ...               
`,
			wantJSON: `[
    {
        "region": "us-east-1",
        "is_online": false,
        "expiry": "0001-01-01T00:00:00Z",
        "run_result": {
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
        "expiry": "0001-01-01T00:00:00Z",
        "run_result": {
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
				nodes: []types.Server{makeNode("node-1", "i-online1", "111", "us-east-1", time.Time{})},
			},
			wantText: `Cloud Account Region    Instance ID Time          Status        Details       
----- ------- --------- ----------- ------------- ------------- ------------- 
AWS   111     us-east-1 i-online1   2026-01-15... Online, ex... Script out... 
`,
			wantJSON: `[
    {
        "region": "us-east-1",
        "is_online": true,
        "expiry": "0001-01-01T00:00:00Z",
        "run_result": {
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
