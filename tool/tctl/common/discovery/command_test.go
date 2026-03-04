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
	"slices"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	libevents "github.com/gravitational/teleport/lib/events"
)

// mockClient implements discoveryClient for testing.
// SearchEvents returns one event per call, paginating via StartKey.
type mockClient struct {
	events []apievents.AuditEvent
	nodes  []types.Server
}

func (m *mockClient) SearchEvents(_ context.Context, req libevents.SearchEventsRequest) ([]apievents.AuditEvent, string, error) {
	// Return one event per call to exercise pagination.
	start := slices.IndexFunc(m.events, func(e apievents.AuditEvent) bool { return e.GetID() == req.StartKey })
	if req.StartKey == "" {
		start = 0
	}
	if start < 0 || start >= len(m.events) {
		return nil, "", nil
	}
	page := m.events[start : start+1]
	if start+1 < len(m.events) {
		return page, m.events[start+1].GetID(), nil
	}
	return page, "", nil
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

// newTestCommand creates a Command wired to a bytes.Buffer for capturing output.
func newTestCommand(format string) (*Command, *bytes.Buffer) {
	var buf bytes.Buffer
	c := &Command{
		clock:           clockwork.NewRealClock(),
		stdout:          &buf,
		inventoryLast:   "1h",
		inventoryFormat: format,
	}
	return c, &buf
}

func TestRunInventory(t *testing.T) {
	now := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		desc     string
		client   discoveryClient
		wantText string
		wantJSON string
	}{
		{
			desc: "failures and successes",
			client: &mockClient{
				events: []apievents.AuditEvent{
					makeSSMRun("i-fail111", "111", "us-east-1", "Failed", 1, "install failed", now),
					makeSSMRun("i-success", "222", "us-west-2", "Success", 0, "", now),
				},
			},
			wantText: `Instance ID Region    Account Online Time                 Result SSM Output       
----------- --------- ------- ------ -------------------- ------ ---------------- 
i-fail111   us-east-1 111     no     2026-01-15T12:00:00Z exit=1 "install failed" 
i-success   us-west-2 222     no     2026-01-15T12:00:00Z exit=0                  
`,
			wantJSON: `[
  {
    "instance_id": "i-fail111",
    "region": "us-east-1",
    "account_id": "111",
    "is_online": false,
    "expiry": "0001-01-01T00:00:00Z",
    "ssm_result": {
      "exit_code": 1,
      "output": "install failed",
      "time": "2026-01-15T12:00:00Z",
      "is_failure": true
    }
  },
  {
    "instance_id": "i-success",
    "region": "us-west-2",
    "account_id": "222",
    "is_online": false,
    "expiry": "0001-01-01T00:00:00Z",
    "ssm_result": {
      "exit_code": 0,
      "output": "",
      "time": "2026-01-15T12:00:00Z",
      "is_failure": false
    }
  }
]
`,
		},
		{
			desc:     "empty result",
			client:   &mockClient{},
			wantText: `No instances found.
`,
			wantJSON: `[]
`,
		},
		{
			desc: "online instance with SSM failure",
			client: &mockClient{
				events: []apievents.AuditEvent{
					makeSSMRun("i-online1", "111", "us-east-1", "Failed", 1, "err", now),
				},
				nodes: []types.Server{makeNode("node-1", "i-online1", "111", "us-east-1", time.Time{})},
			},
			wantText: `Instance ID Region    Account Online Time                 Result SSM Output 
----------- --------- ------- ------ -------------------- ------ ---------- 
i-online1   us-east-1 111     yes    2026-01-15T12:00:00Z exit=1 "err"      
`,
			wantJSON: `[
  {
    "instance_id": "i-online1",
    "region": "us-east-1",
    "account_id": "111",
    "is_online": true,
    "expiry": "0001-01-01T00:00:00Z",
    "ssm_result": {
      "exit_code": 1,
      "output": "err",
      "time": "2026-01-15T12:00:00Z",
      "is_failure": true
    }
  }
]
`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			t.Run(teleport.Text, func(t *testing.T) {
				c, buf := newTestCommand(teleport.Text)
				require.NoError(t, c.runInventory(t.Context(), tt.client))
				require.Equal(t, tt.wantText, buf.String())
			})
			t.Run(teleport.JSON, func(t *testing.T) {
				c, buf := newTestCommand(teleport.JSON)
				require.NoError(t, c.runInventory(t.Context(), tt.client))
				require.Equal(t, tt.wantJSON, buf.String())
			})
		})
	}
}
