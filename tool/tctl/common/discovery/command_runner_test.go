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
	"encoding/json"
	"io"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	usertasksv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/usertasks/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/discoveryconfig"
	apievents "github.com/gravitational/teleport/api/types/events"
	usertasksapi "github.com/gravitational/teleport/api/types/usertasks"
	libevents "github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/usertasks"
	"github.com/gravitational/trace"
)

// ---------------------------------------------------------------------------
// Mock implementations
// ---------------------------------------------------------------------------

// mockDiscoveryClient implements discoveryClient for tests.
type mockDiscoveryClient struct {
	clusterName      types.ClusterName
	events           []apievents.AuditEvent
	integrations     []types.Integration
	integration      types.Integration // for GetIntegration
	userTasks        []*usertasksv1.UserTask
	discoveryConfigs []*discoveryconfig.DiscoveryConfig
	nodes            []types.Server
	searchErr        error
}

func (m *mockDiscoveryClient) SearchEvents(_ context.Context, req libevents.SearchEventsRequest) ([]apievents.AuditEvent, string, error) {
	if m.searchErr != nil {
		return nil, "", m.searchErr
	}
	var result []apievents.AuditEvent
	for _, ev := range m.events {
		if len(req.EventTypes) > 0 && !slices.Contains(req.EventTypes, ev.GetType()) {
			continue
		}
		if !req.From.IsZero() && ev.GetTime().Before(req.From) {
			continue
		}
		if !req.To.IsZero() && ev.GetTime().After(req.To) {
			continue
		}
		result = append(result, ev)
		if req.Limit > 0 && len(result) >= req.Limit {
			break
		}
	}
	return result, "", nil
}

func (m *mockDiscoveryClient) ListIntegrations(_ context.Context, _ int, _ string) ([]types.Integration, string, error) {
	return m.integrations, "", nil
}

func (m *mockDiscoveryClient) GetIntegration(_ context.Context, name string) (types.Integration, error) {
	if m.integration != nil {
		return m.integration, nil
	}
	for _, ig := range m.integrations {
		if ig.GetName() == name {
			return ig, nil
		}
	}
	return nil, trace.NotFound("integration %q not found", name)
}

func (m *mockDiscoveryClient) GetClusterName(_ context.Context) (types.ClusterName, error) {
	if m.clusterName == nil {
		return nil, trace.NotFound("cluster name not configured")
	}
	return m.clusterName, nil
}

func (m *mockDiscoveryClient) UserTasksClient() services.UserTasks {
	return &mockUserTasksClient{tasks: m.userTasks}
}

func (m *mockDiscoveryClient) DiscoveryConfigClient() services.DiscoveryConfigWithStatusUpdater {
	return &mockDiscoveryConfigClient{configs: m.discoveryConfigs}
}

func (m *mockDiscoveryClient) GetResources(_ context.Context, _ *proto.ListResourcesRequest) (*proto.ListResourcesResponse, error) {
	var resources []*proto.PaginatedResource
	for _, node := range m.nodes {
		sv2, ok := node.(*types.ServerV2)
		if !ok {
			continue
		}
		resources = append(resources, &proto.PaginatedResource{
			Resource: &proto.PaginatedResource_Node{Node: sv2},
		})
	}
	return &proto.ListResourcesResponse{
		Resources: resources,
	}, nil
}

// mockUserTasksClient implements services.UserTasks for tests.
type mockUserTasksClient struct {
	tasks []*usertasksv1.UserTask
}

func (m *mockUserTasksClient) CreateUserTask(_ context.Context, _ *usertasksv1.UserTask) (*usertasksv1.UserTask, error) {
	return nil, trace.NotImplemented("not implemented")
}
func (m *mockUserTasksClient) UpsertUserTask(_ context.Context, _ *usertasksv1.UserTask) (*usertasksv1.UserTask, error) {
	return nil, trace.NotImplemented("not implemented")
}
func (m *mockUserTasksClient) GetUserTask(_ context.Context, _ string) (*usertasksv1.UserTask, error) {
	return nil, trace.NotImplemented("not implemented")
}
func (m *mockUserTasksClient) ListUserTasks(_ context.Context, _ int64, _ string, _ *usertasksv1.ListUserTasksFilters) ([]*usertasksv1.UserTask, string, error) {
	return m.tasks, "", nil
}
func (m *mockUserTasksClient) UpdateUserTask(_ context.Context, _ *usertasksv1.UserTask) (*usertasksv1.UserTask, error) {
	return nil, trace.NotImplemented("not implemented")
}
func (m *mockUserTasksClient) DeleteUserTask(_ context.Context, _ string) error {
	return trace.NotImplemented("not implemented")
}

// mockDiscoveryConfigClient implements services.DiscoveryConfigWithStatusUpdater for tests.
type mockDiscoveryConfigClient struct {
	configs []*discoveryconfig.DiscoveryConfig
}

func (m *mockDiscoveryConfigClient) ListDiscoveryConfigs(_ context.Context, _ int, _ string) ([]*discoveryconfig.DiscoveryConfig, string, error) {
	return m.configs, "", nil
}
func (m *mockDiscoveryConfigClient) GetDiscoveryConfig(_ context.Context, _ string) (*discoveryconfig.DiscoveryConfig, error) {
	return nil, trace.NotImplemented("not implemented")
}
func (m *mockDiscoveryConfigClient) CreateDiscoveryConfig(_ context.Context, _ *discoveryconfig.DiscoveryConfig) (*discoveryconfig.DiscoveryConfig, error) {
	return nil, trace.NotImplemented("not implemented")
}
func (m *mockDiscoveryConfigClient) UpdateDiscoveryConfig(_ context.Context, _ *discoveryconfig.DiscoveryConfig) (*discoveryconfig.DiscoveryConfig, error) {
	return nil, trace.NotImplemented("not implemented")
}
func (m *mockDiscoveryConfigClient) UpsertDiscoveryConfig(_ context.Context, _ *discoveryconfig.DiscoveryConfig) (*discoveryconfig.DiscoveryConfig, error) {
	return nil, trace.NotImplemented("not implemented")
}
func (m *mockDiscoveryConfigClient) DeleteDiscoveryConfig(_ context.Context, _ string) error {
	return trace.NotImplemented("not implemented")
}
func (m *mockDiscoveryConfigClient) DeleteAllDiscoveryConfigs(_ context.Context) error {
	return trace.NotImplemented("not implemented")
}
func (m *mockDiscoveryConfigClient) UpdateDiscoveryConfigStatus(_ context.Context, _ string, _ discoveryconfig.Status) (*discoveryconfig.DiscoveryConfig, error) {
	return nil, trace.NotImplemented("not implemented")
}

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

func newTestCommand(t *testing.T, buf *bytes.Buffer) *Command {
	t.Helper()
	return &Command{
		Stdout: buf,
		cache: &eventCache{
			Dir: t.TempDir(),
		},
	}
}

func makeSSMRunEvent(instanceID, accountID, region string, ts time.Time, status string, exitCode int64, output string) *apievents.SSMRun {
	code := libevents.SSMRunSuccessCode
	if exitCode != 0 || status == "Failed" {
		code = libevents.SSMRunFailCode
	}
	return &apievents.SSMRun{
		Metadata: apievents.Metadata{
			Type: libevents.SSMRunEvent,
			Time: ts,
			Code: code,
		},
		CommandID:      "cmd-test",
		InstanceID:     instanceID,
		AccountID:      accountID,
		Region:         region,
		ExitCode:       exitCode,
		Status:         status,
		StandardOutput: output,
		StandardError:  "",
	}
}

func makeJoinEvent(hostID, nodeName, method string, ts time.Time, success bool) *apievents.InstanceJoin {
	code := libevents.InstanceJoinCode
	if !success {
		code = libevents.InstanceJoinFailureCode
	}
	return &apievents.InstanceJoin{
		Metadata: apievents.Metadata{
			Type: libevents.InstanceJoinEvent,
			Time: ts,
			Code: code,
		},
		Status:   apievents.Status{Success: success},
		HostID:   hostID,
		NodeName: nodeName,
		Method:   method,
	}
}

// requireContainsAll asserts that haystack contains every non-empty line of
// needles. Use a multi-line raw string to check several substrings at once:
//
//	requireContainsAll(t, out, `
//	  cluster-a
//	  cluster-b
//	  Affected EKS clusters
//	`)
func requireContainsAll(t *testing.T, haystack, needles string) {
	t.Helper()
	for line := range strings.SplitSeq(needles, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		require.Contains(t, haystack, line, "output should contain %q", line)
	}
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestRunList(t *testing.T) {
	now := time.Now().UTC()
	tests := []struct {
		name   string
		setup  func(*Command)
		run    func(*Command, context.Context, discoveryClient) error
		client *mockDiscoveryClient
	}{
		{
			name: "SSMRuns",
			setup: func(c *Command) {
				c.ssmRunsLast = "1h"
				c.ssmRunsFormat = teleport.Text
				c.ssmRunsRange = "0,10"
			},
			run: (*Command).runSSMRunsList,
			client: &mockDiscoveryClient{
				events: []apievents.AuditEvent{
					makeSSMRunEvent("i-001", "123456789", "us-east-1", now.Add(-10*time.Minute), "Failed", 1, "error: timeout"),
					makeSSMRunEvent("i-002", "123456789", "us-east-1", now.Add(-5*time.Minute), "Success", 0, "ok"),
				},
			},
		},
		{
			name: "Joins",
			setup: func(c *Command) {
				c.joinsLast = "1h"
				c.joinsFormat = teleport.Text
				c.joinsRange = "0,10"
			},
			run: (*Command).runJoinsList,
			client: &mockDiscoveryClient{
				events: []apievents.AuditEvent{
					makeJoinEvent("host-001", "node-1", "ec2", now.Add(-10*time.Minute), true),
					makeJoinEvent("host-002", "node-2", "iam", now.Add(-5*time.Minute), false),
				},
			},
		},
		{
			name: "Inventory",
			setup: func(c *Command) {
				c.inventoryLast = "1h"
				c.inventoryFormat = teleport.Text
				c.inventoryRange = "0,10"
			},
			run: (*Command).runInventoryList,
			client: &mockDiscoveryClient{
				events: []apievents.AuditEvent{
					makeSSMRunEvent("i-001", "123456789", "us-east-1", now.Add(-10*time.Minute), "Failed", 1, "error"),
					makeJoinEvent("host-001", "node-1", "ec2", now.Add(-5*time.Minute), true),
				},
				nodes: []types.Server{},
			},
		},
		{
			name: "Joins/empty",
			setup: func(c *Command) {
				c.joinsLast = "1h"
				c.joinsFormat = teleport.Text
				c.joinsRange = "0,10"
			},
			run:    (*Command).runJoinsList,
			client: &mockDiscoveryClient{events: []apievents.AuditEvent{}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			c := newTestCommand(t, &buf)
			tt.setup(c)
			require.NoError(t, tt.run(c, context.Background(), tt.client))
			require.NotEmpty(t, buf.String())
		})
	}
}

func TestRunShow(t *testing.T) {
	now := time.Now().UTC()
	tests := []struct {
		name   string
		setup  func(*Command)
		run    func(*Command, context.Context, discoveryClient) error
		client *mockDiscoveryClient
		expect string
	}{
		{
			name: "SSMRuns",
			setup: func(c *Command) {
				c.ssmRunsLast = "1h"
				c.ssmRunsFormat = teleport.Text
				c.ssmRunsShowInstanceID = "i-001"
			},
			run: (*Command).runSSMRunsShow,
			client: &mockDiscoveryClient{
				events: []apievents.AuditEvent{
					makeSSMRunEvent("i-001", "123456789", "us-east-1", now.Add(-10*time.Minute), "Failed", 1, "error: timeout"),
					makeSSMRunEvent("i-001", "123456789", "us-east-1", now.Add(-5*time.Minute), "Success", 0, "ok"),
					makeSSMRunEvent("i-002", "123456789", "us-east-1", now.Add(-3*time.Minute), "Success", 0, "ok"),
				},
			},
			expect: "i-001",
		},
		{
			name: "Joins",
			setup: func(c *Command) {
				c.joinsLast = "1h"
				c.joinsFormat = teleport.Text
				c.joinsShowHostID = "host-001"
			},
			run: (*Command).runJoinsShow,
			client: &mockDiscoveryClient{
				events: []apievents.AuditEvent{
					makeJoinEvent("host-001", "node-1", "ec2", now.Add(-10*time.Minute), true),
					makeJoinEvent("host-001", "node-1", "ec2", now.Add(-5*time.Minute), false),
					makeJoinEvent("host-002", "node-2", "iam", now.Add(-3*time.Minute), true),
				},
			},
			expect: "host-001",
		},
		{
			name: "Inventory",
			setup: func(c *Command) {
				c.inventoryLast = "1h"
				c.inventoryFormat = teleport.Text
				c.inventoryShowHostID = "i-001"
			},
			run: (*Command).runInventoryShow,
			client: &mockDiscoveryClient{
				events: []apievents.AuditEvent{
					makeSSMRunEvent("i-001", "123456789", "us-east-1", now.Add(-10*time.Minute), "Failed", 1, "error"),
					makeSSMRunEvent("i-001", "123456789", "us-east-1", now.Add(-5*time.Minute), "Success", 0, "ok"),
				},
				nodes: []types.Server{},
			},
			expect: "i-001",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			c := newTestCommand(t, &buf)
			tt.setup(c)
			require.NoError(t, tt.run(c, context.Background(), tt.client))
			requireContainsAll(t, buf.String(), tt.expect)
		})
	}
}

func TestRunTasksList(t *testing.T) {
	var buf bytes.Buffer
	c := newTestCommand(t, &buf)
	c.tasksListFormat = teleport.Text
	c.tasksListState = "open"

	client := &mockDiscoveryClient{
		userTasks: []*usertasksv1.UserTask{
			{
				Metadata: &headerv1.Metadata{Name: "task-001"},
				Spec: &usertasksv1.UserTaskSpec{
					TaskType:    usertasksapi.TaskTypeDiscoverEC2,
					IssueType:   "ec2-ssm-script-failure",
					Integration: "my-integration",
					State:       usertasksapi.TaskStateOpen,
					DiscoverEc2: &usertasksv1.DiscoverEC2{
						Instances: map[string]*usertasksv1.DiscoverEC2Instance{
							"i-001": {InstanceId: "i-001"},
						},
					},
				},
			},
		},
	}

	require.NoError(t, c.runTasksList(context.Background(), client))
	require.NotEmpty(t, buf.String())
}

func TestRunTaskShow(t *testing.T) {
	tests := []struct {
		name   string
		task   *usertasksv1.UserTask
		expect string
	}{
		{
			name: "EC2",
			task: &usertasksv1.UserTask{
				Metadata: &headerv1.Metadata{Name: "task-001"},
				Spec: &usertasksv1.UserTaskSpec{
					TaskType:    usertasksapi.TaskTypeDiscoverEC2,
					IssueType:   "ec2-ssm-script-failure",
					Integration: "my-integration",
					State:       usertasksapi.TaskStateOpen,
					DiscoverEc2: &usertasksv1.DiscoverEC2{
						Instances: map[string]*usertasksv1.DiscoverEC2Instance{
							"i-001": {InstanceId: "i-001"},
						},
					},
				},
			},
			expect: "task-001",
		},
		{
			name: "EKS",
			task: &usertasksv1.UserTask{
				Metadata: &headerv1.Metadata{Name: "eks-task-001"},
				Spec: &usertasksv1.UserTaskSpec{
					TaskType:    usertasksapi.TaskTypeDiscoverEKS,
					IssueType:   usertasksapi.AutoDiscoverEKSIssueAgentNotConnecting,
					Integration: "my-integration",
					State:       usertasksapi.TaskStateOpen,
					DiscoverEks: &usertasksv1.DiscoverEKS{
						Region:    "us-west-2",
						AccountId: "123456789012",
						Clusters: map[string]*usertasksv1.DiscoverEKSCluster{
							"my-cluster": {
								Name:            "my-cluster",
								DiscoveryConfig: "dc-1",
								DiscoveryGroup:  "dg-1",
							},
						},
					},
				},
			},
			expect: `
				eks-task-001
				my-cluster
				EKS
			`,
		},
		{
			name: "RDS",
			task: &usertasksv1.UserTask{
				Metadata: &headerv1.Metadata{Name: "rds-task-001"},
				Spec: &usertasksv1.UserTaskSpec{
					TaskType:    usertasksapi.TaskTypeDiscoverRDS,
					IssueType:   usertasksapi.AutoDiscoverRDSIssueIAMAuthenticationDisabled,
					Integration: "my-integration",
					State:       usertasksapi.TaskStateOpen,
					DiscoverRds: &usertasksv1.DiscoverRDS{
						Region:    "eu-west-1",
						AccountId: "987654321098",
						Databases: map[string]*usertasksv1.DiscoverRDSDatabase{
							"my-db": {
								Name:            "my-db",
								Engine:          "aurora-postgresql",
								IsCluster:       true,
								DiscoveryConfig: "dc-2",
								DiscoveryGroup:  "dg-2",
							},
						},
					},
				},
			},
			expect: `
				rds-task-001
				my-db
				aurora-postgresql
				RDS
			`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			c := newTestCommand(t, &buf)
			c.tasksShowFormat = teleport.Text
			c.tasksShowName = tt.task.GetMetadata().GetName()
			c.tasksShowRange = "0,10"

			client := &mockDiscoveryClient{userTasks: []*usertasksv1.UserTask{tt.task}}
			require.NoError(t, c.runTaskShow(context.Background(), client))
			requireContainsAll(t, buf.String(), tt.expect)
		})
	}
}

func TestRunIntegration(t *testing.T) {
	ig, err := types.NewIntegrationAWSOIDC(
		types.Metadata{Name: "my-aws-integration"},
		&types.AWSOIDCIntegrationSpecV1{
			RoleARN: "arn:aws:iam::123456789012:role/test",
		},
	)
	require.NoError(t, err)

	client := &mockDiscoveryClient{
		integrations:     []types.Integration{ig},
		userTasks:        []*usertasksv1.UserTask{},
		discoveryConfigs: []*discoveryconfig.DiscoveryConfig{},
	}

	tests := []struct {
		name  string
		setup func(*Command)
		run   func(*Command, context.Context, discoveryClient) error
	}{
		{
			name:  "list",
			setup: func(c *Command) { c.integrationListFormat = teleport.Text },
			run:   (*Command).runIntegrationList,
		},
		{
			name: "show",
			setup: func(c *Command) {
				c.integrationShowFormat = teleport.Text
				c.integrationShowName = "my-aws-integration"
			},
			run: (*Command).runIntegrationShow,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			c := newTestCommand(t, &buf)
			tt.setup(c)
			require.NoError(t, tt.run(c, context.Background(), client))
			require.Contains(t, buf.String(), "my-aws-integration")
		})
	}
}

func TestRunCacheOps(t *testing.T) {
	tests := []struct {
		name   string
		setup  func(*Command)
		run    func(*Command, context.Context, discoveryClient) error
		client discoveryClient
		expect string
	}{
		{
			name:   "status",
			run:    (*Command).runCacheStatus,
			expect: "No cached files",
		},
		{
			name:   "prune",
			run:    (*Command).runCachePrune,
			expect: "Removed 0 cache file(s)",
		},
		{
			name:   "load",
			setup:  func(c *Command) { c.cacheLoadLast = "1h" },
			run:    (*Command).runCacheLoad,
			client: &mockDiscoveryClient{events: []apievents.AuditEvent{}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			c := newTestCommand(t, &buf)
			if tt.setup != nil {
				tt.setup(c)
			}
			require.NoError(t, tt.run(c, context.Background(), tt.client))
			if tt.expect != "" {
				require.Contains(t, buf.String(), tt.expect)
			} else {
				require.NotEmpty(t, buf.String())
			}
		})
	}
}

func TestRunStatus(t *testing.T) {
	var buf bytes.Buffer
	c := newTestCommand(t, &buf)
	c.statusLast = "1h"
	c.statusFormat = teleport.Text
	c.statusSSMLimit = 100
	c.statusJoinLimit = 100

	now := time.Now().UTC()
	client := &mockDiscoveryClient{
		events: []apievents.AuditEvent{
			makeSSMRunEvent("i-001", "123456789", "us-east-1", now.Add(-10*time.Minute), "Failed", 1, "error"),
			makeJoinEvent("host-001", "node-1", "ec2", now.Add(-5*time.Minute), true),
		},
		integrations:     []types.Integration{},
		userTasks:        []*usertasksv1.UserTask{},
		discoveryConfigs: []*discoveryconfig.DiscoveryConfig{},
	}

	require.NoError(t, c.runStatus(context.Background(), client))
	require.NotEmpty(t, buf.String())
}

func TestGroupByAccount(t *testing.T) {
	now := time.Now().UTC()
	tests := []struct {
		name      string
		setup     func(*Command)
		run       func(*Command, context.Context, discoveryClient) error
		client    *mockDiscoveryClient
		expectKey string
	}{
		{
			name: "SSMRuns",
			setup: func(c *Command) {
				c.ssmRunsLast = "1h"
				c.ssmRunsFormat = teleport.JSON
				c.ssmRunsRange = "0,10"
				c.groupByAccount = true
			},
			run: (*Command).runSSMRunsList,
			client: &mockDiscoveryClient{
				events: []apievents.AuditEvent{
					makeSSMRunEvent("i-001", "111111111", "us-east-1", now.Add(-10*time.Minute), "Failed", 1, "error"),
					makeSSMRunEvent("i-002", "222222222", "us-west-2", now.Add(-5*time.Minute), "Failed", 1, "error"),
				},
			},
			expectKey: "vms_by_account",
		},
		{
			name: "Joins",
			setup: func(c *Command) {
				c.joinsLast = "1h"
				c.joinsFormat = teleport.JSON
				c.joinsRange = "0,10"
				c.groupByAccount = true
			},
			run: (*Command).runJoinsList,
			client: &mockDiscoveryClient{
				events: []apievents.AuditEvent{
					makeJoinEvent("host-001", "node-1", "ec2", now.Add(-10*time.Minute), true),
					makeJoinEvent("host-002", "node-2", "iam", now.Add(-5*time.Minute), false),
				},
			},
			expectKey: "hosts_by_account",
		},
		{
			name: "Inventory",
			setup: func(c *Command) {
				c.inventoryLast = "1h"
				c.inventoryFormat = teleport.JSON
				c.inventoryRange = "0,10"
				c.groupByAccount = true
			},
			run: (*Command).runInventoryList,
			client: &mockDiscoveryClient{
				events: []apievents.AuditEvent{
					makeSSMRunEvent("i-001", "111111111", "us-east-1", now.Add(-10*time.Minute), "Failed", 1, "error"),
				},
				nodes: []types.Server{},
			},
			expectKey: "hosts_by_account",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			c := newTestCommand(t, &buf)
			tt.setup(c)
			require.NoError(t, tt.run(c, context.Background(), tt.client))

			var parsed map[string]any
			require.NoError(t, json.Unmarshal(buf.Bytes(), &parsed))
			require.Contains(t, parsed, tt.expectKey)
		})
	}
}

func TestSSMRunsJSONFormat(t *testing.T) {
	var buf bytes.Buffer
	c := newTestCommand(t, &buf)
	c.ssmRunsLast = "1h"
	c.ssmRunsFormat = teleport.JSON
	c.ssmRunsRange = "0,10"

	now := time.Now().UTC()
	client := &mockDiscoveryClient{
		events: []apievents.AuditEvent{
			makeSSMRunEvent("i-001", "123456789", "us-east-1", now.Add(-10*time.Minute), "Failed", 1, "error: timeout"),
			makeSSMRunEvent("i-002", "123456789", "us-east-1", now.Add(-5*time.Minute), "Success", 0, "ok"),
		},
	}

	require.NoError(t, c.runSSMRunsList(context.Background(), client))

	var parsed map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &parsed), "output should be valid JSON")
}

func TestSSMRunsWithGrouping(t *testing.T) {
	var buf bytes.Buffer
	c := newTestCommand(t, &buf)
	c.ssmRunsLast = "1h"
	c.ssmRunsFormat = teleport.Text
	c.ssmRunsRange = "0,10"
	c.ssmRunsGroup = true
	c.ssmRunsSimilarity = 0.4

	now := time.Now().UTC()
	client := &mockDiscoveryClient{
		events: []apievents.AuditEvent{
			makeSSMRunEvent("i-001", "123456789", "us-east-1", now.Add(-10*time.Minute), "Failed", 1, "error: connection timeout to host"),
			makeSSMRunEvent("i-002", "123456789", "us-east-1", now.Add(-5*time.Minute), "Failed", 1, "error: connection timeout to host"),
			makeSSMRunEvent("i-003", "123456789", "us-east-1", now.Add(-3*time.Minute), "Success", 0, "ok"),
		},
	}

	require.NoError(t, c.runSSMRunsList(context.Background(), client))
	require.NotEmpty(t, buf.String())
}

func TestRunJoinsRaw(t *testing.T) {
	now := time.Now().UTC()
	events := []apievents.AuditEvent{
		makeJoinEvent("host-001", "node-1", "ec2", now.Add(-5*time.Minute), true),
		makeJoinEvent("host-002", "node-2", "iam", now.Add(-3*time.Minute), false),
	}
	tests := []struct {
		name       string
		hostFilter string
		wantCount  int
	}{
		{name: "all", hostFilter: "", wantCount: 2},
		{name: "filtered", hostFilter: "host-001", wantCount: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			c := newTestCommand(t, &buf)
			c.joinsLast = "1h"
			client := &mockDiscoveryClient{events: events}

			require.NoError(t, c.runJoinsRaw(context.Background(), client, tt.hostFilter))

			var parsed []json.RawMessage
			require.NoError(t, json.Unmarshal(buf.Bytes(), &parsed))
			require.Len(t, parsed, tt.wantCount)
		})
	}
}

func TestInitCache(t *testing.T) {
	cn, err := types.NewClusterName(types.ClusterNameSpecV2{ClusterName: "test-cluster", ClusterID: "id-1"})
	require.NoError(t, err)

	tests := []struct {
		name       string
		client     *mockDiscoveryClient
		wantNewDir string // empty means cache should not be replaced
	}{
		{name: "success", client: &mockDiscoveryClient{clusterName: cn}, wantNewDir: "test-cluster"},
		{name: "error/no_cluster_name", client: &mockDiscoveryClient{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			c := newTestCommand(t, &buf)
			origCache := c.cache
			if tt.wantNewDir != "" {
				c.cache = nil
			}
			c.initCache(context.Background(), tt.client)
			if tt.wantNewDir != "" {
				require.NotNil(t, c.cache)
				require.Contains(t, c.cache.Dir, tt.wantNewDir)
			} else {
				require.Equal(t, origCache, c.cache)
			}
		})
	}
}

func TestEksActionURL(t *testing.T) {
	tests := []struct {
		name    string
		cluster *usertasks.DiscoverEKSClusterWithURLs
		want    string
	}{
		{
			name: "OpenTeleportAgentURL takes priority",
			cluster: &usertasks.DiscoverEKSClusterWithURLs{
				OpenTeleportAgentURL:    "https://agent-url",
				ManageAccessURL:         "https://access-url",
				ManageEndpointAccessURL: "https://endpoint-url",
				ManageClusterURL:        "https://cluster-url",
			},
			want: "https://agent-url",
		},
		{
			name: "ManageAccessURL is second priority",
			cluster: &usertasks.DiscoverEKSClusterWithURLs{
				ManageAccessURL:         "https://access-url",
				ManageEndpointAccessURL: "https://endpoint-url",
				ManageClusterURL:        "https://cluster-url",
			},
			want: "https://access-url",
		},
		{
			name: "ManageEndpointAccessURL is third priority",
			cluster: &usertasks.DiscoverEKSClusterWithURLs{
				ManageEndpointAccessURL: "https://endpoint-url",
				ManageClusterURL:        "https://cluster-url",
			},
			want: "https://endpoint-url",
		},
		{
			name:    "ManageClusterURL is last",
			cluster: &usertasks.DiscoverEKSClusterWithURLs{ManageClusterURL: "https://cluster-url"},
			want:    "https://cluster-url",
		},
		{
			name:    "empty returns empty",
			cluster: &usertasks.DiscoverEKSClusterWithURLs{},
			want:    "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, eksActionURL(tt.cluster))
		})
	}
}

func TestRenderDetails(t *testing.T) {
	tests := []struct {
		name      string
		task      *usertasksv1.UserTask
		render    func(io.Writer, *usertasksv1.UserTask, int, int) (pageInfo, error)
		wantTotal int
		expect    string
	}{
		{
			name: "EKS",
			task: &usertasksv1.UserTask{
				Spec: &usertasksv1.UserTaskSpec{
					TaskType:  usertasksapi.TaskTypeDiscoverEKS,
					IssueType: usertasksapi.AutoDiscoverEKSIssueAgentNotConnecting,
					DiscoverEks: &usertasksv1.DiscoverEKS{
						Region: "us-east-1",
						Clusters: map[string]*usertasksv1.DiscoverEKSCluster{
							"cluster-a": {Name: "cluster-a", DiscoveryConfig: "dc-1", DiscoveryGroup: "dg-1"},
							"cluster-b": {Name: "cluster-b", DiscoveryConfig: "dc-2", DiscoveryGroup: "dg-2"},
						},
					},
				},
			},
			render:    renderEKSDetails,
			wantTotal: 2,
			expect: `
				cluster-a
				cluster-b
				Affected EKS clusters
			`,
		},
		{
			name: "RDS",
			task: &usertasksv1.UserTask{
				Spec: &usertasksv1.UserTaskSpec{
					TaskType:  usertasksapi.TaskTypeDiscoverRDS,
					IssueType: usertasksapi.AutoDiscoverRDSIssueIAMAuthenticationDisabled,
					DiscoverRds: &usertasksv1.DiscoverRDS{
						Region: "eu-west-1",
						Databases: map[string]*usertasksv1.DiscoverRDSDatabase{
							"db-prod": {Name: "db-prod", Engine: "aurora-postgresql", IsCluster: true, DiscoveryConfig: "dc-1", DiscoveryGroup: "dg-1"},
						},
					},
				},
			},
			render:    renderRDSDetails,
			wantTotal: 1,
			expect: `
				db-prod
				aurora-postgresql
				Affected RDS databases
			`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			info, err := tt.render(&buf, tt.task, 0, 10)
			require.NoError(t, err)
			require.Equal(t, tt.wantTotal, info.Total)
			requireContainsAll(t, buf.String(), tt.expect)
		})
	}
}
