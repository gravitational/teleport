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
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/breaker"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	usertasksv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/usertasks/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/discoveryconfig"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/auth/authclient"
	libevents "github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/utils"
	tctlcfg "github.com/gravitational/teleport/tool/tctl/common/config"

	usertasksapi "github.com/gravitational/teleport/api/types/usertasks"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestDiscoveryNormalizeTaskState(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    string
		wantErr string
	}{
		{name: "empty defaults to open", input: "", want: usertasksapi.TaskStateOpen},
		{name: "open lower", input: "open", want: usertasksapi.TaskStateOpen},
		{name: "open upper", input: "OPEN", want: usertasksapi.TaskStateOpen},
		{name: "resolved", input: "resolved", want: usertasksapi.TaskStateResolved},
		{name: "all", input: "all", want: ""},
		{name: "invalid", input: "broken", wantErr: "invalid state"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := normalizeTaskState(tc.input)
			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestDiscoveryParseAndAnalyzeSSMRuns(t *testing.T) {
	t.Parallel()

	eventList := []apievents.AuditEvent{
		&apievents.SSMRun{
			Metadata:      apievents.Metadata{Time: mustTestParseTime(t, "2026-02-12T10:00:00Z"), Code: libevents.SSMRunSuccessCode},
			InstanceID:    "i-1",
			Status:        "Success",
			ExitCode:      0,
			AccountID:     "123",
			Region:        "us-east-1",
			CommandID:     "cmd-1",
			InvocationURL: "https://example/cmd-1",
		},
		&apievents.SSMRun{
			Metadata:      apievents.Metadata{Time: mustTestParseTime(t, "2026-02-12T10:05:00Z"), Code: libevents.SSMRunFailCode},
			InstanceID:    "i-2",
			Status:        "Failed",
			ExitCode:      1,
			AccountID:     "123",
			Region:        "us-east-1",
			CommandID:     "cmd-2",
			InvocationURL: "https://example/cmd-2",
			StandardError: "script error",
		},
		&apievents.SSMRun{
			Metadata:      apievents.Metadata{Time: mustTestParseTime(t, "2026-02-12T10:10:00Z"), Code: libevents.SSMRunFailCode},
			InstanceID:    "i-2",
			Status:        "TimedOut",
			ExitCode:      -1,
			AccountID:     "123",
			Region:        "us-east-1",
			CommandID:     "cmd-3",
			InvocationURL: "https://example/cmd-3",
			StandardError: "timeout",
		},
	}

	parsed := parseSSMRunEvents(eventList, ssmRunEventFilters{})
	require.Len(t, parsed, 3)

	analysis := analyzeSSMRuns(parsed)
	require.Equal(t, 3, analysis.Total)
	require.Equal(t, 1, analysis.Success)
	require.Equal(t, 2, analysis.Failed)
	require.Equal(t, 2, analysis.ByInstance["i-2"])
}

func TestDiscoveryParseSSMRunEvents(t *testing.T) {
	t.Parallel()

	eventList := []apievents.AuditEvent{
		&apievents.SSMRun{
			Metadata:      apievents.Metadata{Time: mustTestParseTime(t, "2026-02-12T10:00:00Z"), Code: libevents.SSMRunSuccessCode},
			InstanceID:    "i-1",
			Status:        "Success",
			ExitCode:      0,
			AccountID:     "123",
			Region:        "us-east-1",
			CommandID:     "cmd-1",
			InvocationURL: "https://example/cmd-1",
		},
		&apievents.SSMRun{
			Metadata:      apievents.Metadata{Time: mustTestParseTime(t, "2026-02-12T10:05:00Z"), Code: libevents.SSMRunFailCode},
			InstanceID:    "i-2",
			Status:        "Failed",
			ExitCode:      1,
			AccountID:     "123",
			Region:        "us-east-1",
			CommandID:     "cmd-2",
			InvocationURL: "https://example/cmd-2",
			StandardError: "script error",
		},
		&apievents.SSMRun{
			Metadata:      apievents.Metadata{Time: mustTestParseTime(t, "2026-02-12T10:10:00Z"), Code: libevents.SSMRunSuccessCode},
			InstanceID:    "i-2",
			Status:        "TimedOut",
			ExitCode:      -1,
			AccountID:     "123",
			Region:        "us-east-1",
			CommandID:     "cmd-3",
			InvocationURL: "https://example/cmd-3",
			StandardError: "timeout",
		},
	}

	all := parseSSMRunEvents(eventList, ssmRunEventFilters{})
	require.Len(t, all, 3)
	require.Equal(t, "cmd-3", all[0].CommandID)
	require.Equal(t, "cmd-2", all[1].CommandID)
	require.Equal(t, "cmd-1", all[2].CommandID)
	require.Equal(t, "-1", all[0].ExitCode)
	require.Equal(t, "timeout", all[0].Stderr)

	filtered := parseSSMRunEvents(eventList, ssmRunEventFilters{
		FailedOnly: true,
		InstanceID: "i-2",
	})
	require.Len(t, filtered, 2)
	require.Equal(t, "cmd-3", filtered[0].CommandID)
	require.Equal(t, "i-2", filtered[0].InstanceID)
	require.Equal(t, "cmd-2", filtered[1].CommandID)
}

func TestDiscoverySelectFailingVMGroups(t *testing.T) {
	t.Parallel()

	records := []ssmRunRecord{
		{InstanceID: "i-1", Status: "Success", Code: "TDS00I", CommandID: "cmd-101", parsedEventTime: mustTestParseTime(t, "2026-02-12T10:10:00Z")},
		{InstanceID: "i-1", Status: "Failed", Code: "TDS00W", CommandID: "cmd-100", parsedEventTime: mustTestParseTime(t, "2026-02-12T10:00:00Z")},
		{InstanceID: "i-2", Status: "Failed", Code: "TDS00W", CommandID: "cmd-201", parsedEventTime: mustTestParseTime(t, "2026-02-12T10:20:00Z")},
		{InstanceID: "i-2", Status: "Success", Code: "TDS00I", CommandID: "cmd-200", parsedEventTime: mustTestParseTime(t, "2026-02-12T10:05:00Z")},
		{InstanceID: "i-3", Status: "TimedOut", Code: "TDS00W", CommandID: "cmd-301", parsedEventTime: mustTestParseTime(t, "2026-02-12T10:30:00Z")},
	}

	vmGroups := groupSSMRunsByVM(records)
	require.Len(t, vmGroups, 3)

	failing := selectFailingVMGroups(vmGroups, 25)
	require.Len(t, failing, 2)
	require.Equal(t, "i-3", failing[0].InstanceID)
	require.Equal(t, "i-2", failing[1].InstanceID)

	limited := selectFailingVMGroups(vmGroups, 1)
	require.Len(t, limited, 1)
	require.Equal(t, "i-3", limited[0].InstanceID)
}

func TestDiscoveryVMHistoryRowsDefaultAndAll(t *testing.T) {
	t.Parallel()

	vmGroup := ssmVMGroup{
		InstanceID: "i-55",
		Runs: []ssmRunRecord{
			{InstanceID: "i-55", Status: "Failed", Code: "TDS00W", CommandID: "cmd-new", parsedEventTime: mustTestParseTime(t, "2026-02-12T12:10:00Z")},
			{InstanceID: "i-55", Status: "Success", Code: "TDS00I", CommandID: "cmd-old", parsedEventTime: mustTestParseTime(t, "2026-02-12T12:00:00Z")},
		},
	}

	defaultRows := buildVMHistoryRows(vmGroup, false)
	require.Len(t, defaultRows, 1)
	require.Equal(t, "cmd-new", defaultRows[0].CommandID)
	require.Equal(t, "2026-02-12 12:10:00", defaultRows[0].Timestamp)

	allRows := buildVMHistoryRows(vmGroup, true)
	require.Len(t, allRows, 2)
	require.Equal(t, "cmd-new", allRows[0].CommandID)
	require.Equal(t, "cmd-old", allRows[1].CommandID)
	require.Equal(t, "2026-02-12 12:10:00", allRows[0].Timestamp)
	require.Equal(t, "2026-02-12 12:00:00", allRows[1].Timestamp)
}

func TestDiscoveryRenderEC2DetailsCompact(t *testing.T) {
	t.Parallel()

	task := &usertasksv1.UserTask{
		Spec: &usertasksv1.UserTaskSpec{
			TaskType: usertasksapi.TaskTypeDiscoverEC2,
			DiscoverEc2: &usertasksv1.DiscoverEC2{
				Region: "eu-central-1",
				Instances: map[string]*usertasksv1.DiscoverEC2Instance{
					"i-003a23be4c3d13fa8": {
						InstanceId:      "i-003a23be4c3d13fa8",
						Name:            "target-1",
						DiscoveryConfig: "cfg-main",
						DiscoveryGroup:  "main",
						SyncTime:        timestamppb.New(mustTestParseTime(t, "2026-02-13T15:46:24Z")),
					},
					"i-085159ed62c5364c2": {
						InstanceId:      "i-085159ed62c5364c2",
						Name:            "target-2",
						DiscoveryConfig: "cfg-main",
						DiscoveryGroup:  "main",
						SyncTime:        timestamppb.New(mustTestParseTime(t, "2026-02-13T15:46:24Z")),
						InvocationUrl:   "https://example.test/invocation",
					},
				},
			},
		},
	}

	var out bytes.Buffer
	info, err := renderEC2Details(&out, task, 1, 25)
	require.NoError(t, err)
	require.Equal(t, 2, info.Total)

	got := out.String()
	require.Contains(t, got, "Affected EC2 instances:")
	require.Contains(t, got, "[1] INSTANCE: i-003a23be4c3d13fa8")
	require.Contains(t, got, "[2] INSTANCE: i-085159ed62c5364c2")
	require.Contains(t, got, "    NAME            : target-1")
	require.Contains(t, got, "    REGION          : eu-central-1")
	require.Contains(t, got, "    DISCOVERY CONFIG: cfg-main")
	require.Contains(t, got, "    DISCOVERY GROUP : main")
	require.Contains(t, got, "    SYNC TIME       :")
	require.Contains(t, got, "    INVOCATION URL  : https://example.test/invocation")
	require.Contains(t, got, "\n\n[2] INSTANCE:")
	require.NotContains(t, got, "┌")
	require.NotContains(t, got, "│")
}

func TestDiscoveryRenderAzureVMDetailsCompact(t *testing.T) {
	t.Parallel()

	task := &usertasksv1.UserTask{
		Spec: &usertasksv1.UserTaskSpec{
			TaskType: usertasksapi.TaskTypeDiscoverAzureVM,
			DiscoverAzureVm: &usertasksv1.DiscoverAzureVM{
				Instances: map[string]*usertasksv1.DiscoverAzureVMInstance{
					"4d944eab-ad1d-49ca-8cc3-8a62f920b5b1": {
						VmId:            "4d944eab-ad1d-49ca-8cc3-8a62f920b5b1",
						Name:            "tener-dev-9cab71ed-vm-0",
						ResourceId:      "/subscriptions/060a97ea-3a57-4218-9be5-dba3f19ff2b5/resourceGroups/tener-dev-9cab71ed-workload-rg/providers/Microsoft.Compute/virtualMachines/tener-dev-9cab71ed-vm-0",
						DiscoveryConfig: "tener-dev-9cab71ed-azure_teleport",
						DiscoveryGroup:  "main",
						SyncTime:        timestamppb.New(mustTestParseTime(t, "2026-02-13T17:02:21Z")),
					},
				},
			},
		},
	}

	var out bytes.Buffer
	info, err := renderAzureVMDetails(&out, task, 1, 25)
	require.NoError(t, err)
	require.Equal(t, 1, info.Total)

	got := out.String()
	require.Contains(t, got, "Affected Azure VMs:")
	require.Contains(t, got, "[1] VM ID: 4d944eab-ad1d-49ca-8cc3-8a62f920b5b1")
	require.Contains(t, got, "    NAME            : tener-dev-9cab71ed-vm-0")
	require.Contains(t, got, "    RESOURCE ID     : /subscriptions/060a97ea-3a57-4218-9be5-dba3f19ff2b5/resourceGroups/tener-dev-9cab71ed-workload-rg/providers/Microsoft.Compute/virtualMachines/tener-dev-9cab71ed-vm-0")
	require.Contains(t, got, "    DISCOVERY CONFIG: tener-dev-9cab71ed-azure_teleport")
	require.Contains(t, got, "    DISCOVERY GROUP : main")
	require.Contains(t, got, "    SYNC TIME       : 2026-02-13T17:02:21Z")
	require.NotContains(t, got, "┌")
	require.NotContains(t, got, "│")
}

func TestDiscoveryRenderTaskDetailsShowingResourcesLine(t *testing.T) {
	t.Parallel()

	task := &usertasksv1.UserTask{
		Metadata: &headerv1.Metadata{
			Name:    "e785789e-4fbc-5774-a3d3-4e34edc80dbd",
			Expires: timestamppb.New(time.Now().UTC().Add(90 * time.Minute)),
		},
		Spec: &usertasksv1.UserTaskSpec{
			State:       usertasksapi.TaskStateOpen,
			TaskType:    usertasksapi.TaskTypeDiscoverEC2,
			IssueType:   usertasksapi.AutoDiscoverEC2IssueSSMInstanceNotRegistered,
			Integration: "",
			DiscoverEc2: &usertasksv1.DiscoverEC2{
				Region: "eu-central-1",
			},
		},
	}

	var out bytes.Buffer
	err := renderTaskDetailsText(&out, task, 1, 25, "tctl discovery tasks show e785789e --page-size=25")
	require.NoError(t, err)

	got := out.String()
	require.Contains(t, got, "Showing resources: 0-0.")
	require.Contains(t, got, "none (ambient credentials)")
	require.Contains(t, got, "in ")
	require.Contains(t, got, "How to fix:")
	require.Contains(t, got, "SSM Fleet Manager: https://console.aws.amazon.com/systems-manager/fleet-manager/managed-nodes")
	require.Contains(t, got, "SSM AGENT IS NOT RUNNING:")
	require.NotContains(t, got, "[SSM Fleet Manager](")
	require.NotContains(t, got, "**SSM Agent is not running**")
	require.NotContains(t, got, "`AmazonSSMManagedInstanceCore`")
	require.Greater(t, strings.Index(got, "How to fix:"), strings.Index(got, "Affected EC2 instances:"))
	require.NotContains(t, got, "--page=")
	require.NotContains(t, got, "More resources available")
	require.NotContains(t, got, "Next page:")
	require.Contains(t, got, "# Use machine-readable output")
	require.Contains(t, got, "tctl discovery tasks show e785789e --format=json")
	require.Contains(t, got, "tctl discovery tasks show e785789e --format=yaml")
}

func TestDiscoveryRenderTaskDetailsNextPageInNextSection(t *testing.T) {
	t.Parallel()

	task := &usertasksv1.UserTask{
		Metadata: &headerv1.Metadata{Name: "e785789e-4fbc-5774-a3d3-4e34edc80dbd"},
		Spec: &usertasksv1.UserTaskSpec{
			State:       usertasksapi.TaskStateOpen,
			TaskType:    usertasksapi.TaskTypeDiscoverEC2,
			IssueType:   usertasksapi.AutoDiscoverEC2IssueSSMInstanceNotRegistered,
			Integration: "integration-1",
			DiscoverEc2: &usertasksv1.DiscoverEC2{
				Region: "eu-central-1",
				Instances: map[string]*usertasksv1.DiscoverEC2Instance{
					"i-1": {InstanceId: "i-1"},
					"i-2": {InstanceId: "i-2"},
					"i-3": {InstanceId: "i-3"},
				},
			},
		},
	}

	var out bytes.Buffer
	err := renderTaskDetailsText(&out, task, 1, 2, "tctl discovery tasks show e785789e --page=1 --page-size=2")
	require.NoError(t, err)

	got := out.String()
	require.Contains(t, got, "Showing resources: 1-2 of 3.")
	require.Contains(t, got, "# Show next page of affected resources")
	require.Contains(t, got, "tctl discovery tasks show e785789e --page-size=2 --page=2")
	require.NotContains(t, got, "--page=1 --page=2")
	require.NotContains(t, got, "More resources available")
	require.NotContains(t, got, "Next page:")
}

func TestDiscoveryRenderTaskDetailsOutOfRangePage(t *testing.T) {
	t.Parallel()

	task := &usertasksv1.UserTask{
		Metadata: &headerv1.Metadata{Name: "e785789e-4fbc-5774-a3d3-4e34edc80dbd"},
		Spec: &usertasksv1.UserTaskSpec{
			State:       usertasksapi.TaskStateOpen,
			TaskType:    usertasksapi.TaskTypeDiscoverEC2,
			IssueType:   usertasksapi.AutoDiscoverEC2IssueSSMInstanceNotRegistered,
			Integration: "integration-1",
			DiscoverEc2: &usertasksv1.DiscoverEC2{
				Region: "eu-central-1",
				Instances: map[string]*usertasksv1.DiscoverEC2Instance{
					"i-1": {InstanceId: "i-1"},
					"i-2": {InstanceId: "i-2"},
				},
			},
		},
	}

	var out bytes.Buffer
	err := renderTaskDetailsText(&out, task, 999, 25, "tctl discovery tasks show e785789e --page=999 --page-size=25")
	require.NoError(t, err)

	got := out.String()
	require.Contains(t, got, "Showing resources: 0-0 of 2.")
	require.Contains(t, got, "# Current resource page is out of range")
	require.Contains(t, got, "tctl discovery tasks show e785789e --page-size=25 --page=1")
	require.NotContains(t, got, "--page=999 --page=1")
}

func TestDiscoveryPaginateSlice(t *testing.T) {
	t.Parallel()

	items := []int{1, 2, 3, 4, 5}
	pageItems, info := paginateSlice(items, 2, 2)
	require.Equal(t, []int{3, 4}, pageItems)
	require.Equal(t, 2, info.Page)
	require.Equal(t, 2, info.PageSize)
	require.Equal(t, 5, info.Total)
	require.Equal(t, 2, info.Start)
	require.Equal(t, 4, info.End)
	require.Equal(t, 1, info.Remaining)
	require.True(t, info.HasNext)
	require.Equal(t, 3, info.NextPage)
}

func TestDiscoveryRenderTasksListNoPagination(t *testing.T) {
	t.Parallel()

	items := []taskListItem{
		{Name: "e785789e-4fbc-5774-a3d3-4e34edc80dbd", State: "OPEN", TaskType: "discover-ec2", IssueType: "ec2-ssm-script-failure", Affected: 1, Integration: "i1"},
		{Name: "f1234567-1111-2222-3333-444444444444", State: "OPEN", TaskType: "discover-ec2", IssueType: "ec2-ssm-script-failure", Affected: 1, Integration: "i1"},
		{Name: "a1234567-1111-2222-3333-444444444444", State: "OPEN", TaskType: "discover-ec2", IssueType: "ec2-ssm-script-failure", Affected: 1, Integration: "i1"},
	}

	var out bytes.Buffer
	err := renderTasksListText(&out, items, taskListHintsInput{State: usertasksapi.TaskStateOpen})
	require.NoError(t, err)

	got := out.String()
	require.Contains(t, got, "User Tasks [3 matching filters]")
	require.Contains(t, got, "[1] TASK: e785789e-4fbc-5774-a3d3-4e34edc80dbd")
	require.Contains(t, got, "[2] TASK: f1234567-1111-2222-3333-444444444444")
	require.Contains(t, got, "[3] TASK: a1234567-1111-2222-3333-444444444444")
	require.NotContains(t, got, "e785789e-...")
	require.Contains(t, got, "    TYPE             : AWS EC2")
	require.Contains(t, got, "    ISSUE TYPE       : ec2-ssm-script-failure")
	require.Contains(t, got, "    AFFECTED         : 1")
	require.Contains(t, got, "    INTEGRATION      : i1")
	require.Contains(t, got, "    LAST STATE CHANGE: never")
	require.NotContains(t, got, "Summary Item")
	require.NotContains(t, got, "User tasks matching filters")
	require.NotContains(t, got, "┌")
	require.NotContains(t, got, "│")
	require.NotContains(t, got, "More tasks available")
	require.NotContains(t, got, "Next page:")
	require.Contains(t, got, "# Adjust task list filters")
	require.Contains(t, got, "tctl discovery tasks ls --task-type=discover-ec2")
	require.Contains(t, got, "tctl discovery tasks ls --issue-type=ec2-ssm-script-failure")
	require.Contains(t, got, "tctl discovery tasks ls --integration=i1")
	require.Contains(t, got, "# Inspect one task in detail")
	require.Contains(t, got, "tctl discovery tasks show e785789e")
	require.Contains(t, got, "tctl discovery tasks show <task-id-prefix>")
	require.Contains(t, got, "# Use machine-readable output")
	require.Contains(t, got, "tctl discovery tasks ls --format=json")
	require.Contains(t, got, "tctl discovery tasks ls --format=yaml")
	require.Contains(t, got, "\n\n  # Inspect one task in detail")
}

func TestDiscoveryAliasCommand(t *testing.T) {
	t.Parallel()

	cfg := servicecfg.MakeDefaultConfig()
	cfg.CircuitBreakerConfig = breaker.NoopBreakerConfig()

	var cmd Command
	app := utils.InitCLIParser("tctl", "tctl test")
	cmd.Initialize(app, &tctlcfg.GlobalCLIFlags{}, cfg)

	selected, err := app.Parse([]string{"discover", "status"})
	require.NoError(t, err)
	require.Equal(t, "discovery status", selected)

	sentinelErr := errors.New("sentinel")
	_, err = cmd.TryRun(t.Context(), selected, func(context.Context) (*authclient.Client, func(context.Context), error) {
		return nil, func(context.Context) {}, sentinelErr
	})
	require.ErrorIs(t, err, sentinelErr)
}

func TestDiscoveryStatusDoesNotAcceptPagingFlags(t *testing.T) {
	t.Parallel()

	cfg := servicecfg.MakeDefaultConfig()
	cfg.CircuitBreakerConfig = breaker.NoopBreakerConfig()

	var cmd Command
	app := utils.InitCLIParser("tctl", "tctl test")
	cmd.Initialize(app, &tctlcfg.GlobalCLIFlags{}, cfg)

	_, err := app.Parse([]string{"discovery", "status", "--page=2"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown long flag '--page'")

	_, err = app.Parse([]string{"discovery", "status", "--page-size=10"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown long flag '--page-size'")
}

func TestDiscoveryTasksListDoesNotAcceptPagingFlags(t *testing.T) {
	t.Parallel()

	cfg := servicecfg.MakeDefaultConfig()
	cfg.CircuitBreakerConfig = breaker.NoopBreakerConfig()

	var cmd Command
	app := utils.InitCLIParser("tctl", "tctl test")
	cmd.Initialize(app, &tctlcfg.GlobalCLIFlags{}, cfg)

	_, err := app.Parse([]string{"discovery", "tasks", "ls", "--page=2"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown long flag '--page'")

	_, err = app.Parse([]string{"discovery", "tasks", "ls", "--page-size=10"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown long flag '--page-size'")
}

func TestDiscoverySSMRunsUsesListAndShowSubcommands(t *testing.T) {
	t.Parallel()

	cfg := servicecfg.MakeDefaultConfig()
	cfg.CircuitBreakerConfig = breaker.NoopBreakerConfig()

	var cmd Command
	app := utils.InitCLIParser("tctl", "tctl test")
	cmd.Initialize(app, &tctlcfg.GlobalCLIFlags{}, cfg)

	selected, err := app.Parse([]string{"discovery", "ssm-runs", "ls"})
	require.NoError(t, err)
	require.Equal(t, "discovery ssm-runs ls", selected)

	selected, err = app.Parse([]string{"discovery", "ssm-runs", "show", "i-123"})
	require.NoError(t, err)
	require.Equal(t, "discovery ssm-runs show", selected)

	_, err = app.Parse([]string{"discovery", "ssm-runs"})
	require.Error(t, err)
}

func TestDiscoverySSMRunsDoesNotAcceptStatusFlag(t *testing.T) {
	t.Parallel()

	cfg := servicecfg.MakeDefaultConfig()
	cfg.CircuitBreakerConfig = breaker.NoopBreakerConfig()

	var cmd Command
	app := utils.InitCLIParser("tctl", "tctl test")
	cmd.Initialize(app, &tctlcfg.GlobalCLIFlags{}, cfg)

	_, err := app.Parse([]string{"discovery", "ssm-runs", "ls", "--status=failed"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown long flag '--status'")

	_, err = app.Parse([]string{"discovery", "ssm-runs", "show", "i-123", "--status=failed"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown long flag '--status'")
}

func TestDiscoveryCountRowsSortByCountDesc(t *testing.T) {
	t.Parallel()

	rows := countRows(map[string]int{
		"low":   1,
		"high":  7,
		"equal": 7,
	})

	require.Len(t, rows, 3)
	require.Equal(t, "equal", rows[0].Key)
	require.Equal(t, 7, rows[0].Count)
	require.Equal(t, "high", rows[1].Key)
	require.Equal(t, 7, rows[1].Count)
	require.Equal(t, "low", rows[2].Key)
	require.Equal(t, 1, rows[2].Count)
}

func TestDiscoveryRenderStatusTextIncludesConfigsAndIdleIntegrations(t *testing.T) {
	t.Parallel()

	summary := statusSummary{
		GeneratedAt:          mustTestParseTime(t, "2026-02-13T12:00:00Z"),
		FilteredState:        "OPEN",
		DiscoveryConfigCount: 2,
		DiscoveryGroupCount:  1,
		DiscoveryConfigs: []configStatus{
			{
				Name:       "dc-alpha",
				Group:      "group-1",
				State:      "RUNNING",
				Matchers:   "aws=1",
				Discovered: 3,
				LastSync:   mustTestParseTime(t, "2026-02-13T11:58:00Z"),
			},
			{
				Name:       "dc-beta",
				Group:      "group-1",
				State:      "DISCOVERY_CONFIG_STATE_SYNCING",
				Matchers:   "azure=1",
				Discovered: 0,
				LastSync:   mustTestParseTime(t, "2026-02-13T11:59:00Z"),
			},
		},
		TotalTasks:        2,
		OpenTasks:         2,
		ResolvedTasks:     0,
		FilteredTaskCount: 2,
		UserTasks: []taskListItem{
			{
				Name:            "e785789e-4fbc-5774-a3d3-4e34edc80dbd",
				State:           "OPEN",
				TaskType:        "discover-azure-vm",
				IssueType:       "azure-vm-not-running",
				Integration:     "integration-active",
				Affected:        1,
				LastStateChange: mustTestParseTime(t, "2026-02-13T11:50:00Z"),
			},
		},
		TasksByType: map[string]int{
			"discover-azure-vm": 2,
		},
		TasksByIssue: map[string]int{
			"azure-vm-enrollment-error": 1,
			"azure-vm-not-running":      1,
		},
		IntegrationResourceStats: map[string]resourcesAggregate{
			"":                   {Found: 2, Enrolled: 0, Failed: 2},
			"integration-active": {Found: 2, Enrolled: 0, Failed: 1},
			"integration-idle":   {Found: 0, Enrolled: 0, Failed: 0},
		},
	}

	var out bytes.Buffer
	err := renderStatusText(&out, summary)
	require.NoError(t, err)

	got := out.String()
	require.Contains(t, got, "User Tasks")
	require.Contains(t, got, "User Tasks [2 total, 2 open, 0 resolved]")
	require.Contains(t, got, "Showing 2 tasks matching state=OPEN")
	require.Contains(t, got, "┌")
	require.Contains(t, got, "Discovery Configs")
	require.Contains(t, got, "Discovery Configs [2 total, 1 group]")
	require.Contains(t, got, "Integration Resource Status [3 total]")
	require.Contains(t, got, "Awaiting Join")
	require.Contains(t, got, "none (ambient credentials)")
	require.NotContains(t, got, "Generated")
	require.NotContains(t, got, "  Summary")
	require.NotContains(t, got, "User tasks in all states")
	require.NotContains(t, got, "User tasks matching this view")
	require.Contains(t, got, "dc-alpha")
	require.Contains(t, got, "dc-beta")
	require.Contains(t, got, "Syncing")
	require.Contains(t, got, "1m ago")
	require.Contains(t, got, "10m ago")
	require.Contains(t, got, "e785789e-4fbc-5774-a3d3-4e34edc80dbd")
	require.NotContains(t, got, "e785789e-...")
	require.Contains(t, got, "Azure VM")
	require.NotContains(t, got, "discover-azure-vm")
	require.Contains(t, got, "azure=1")
	require.NotContains(t, got, "aws=0")
	require.Contains(t, got, "integration-idle")
	require.NotContains(t, got, "Hidden idle integrations")
	require.NotContains(t, got, "Page settings")
	require.NotContains(t, got, "More user-task rows")
	require.NotContains(t, got, "More discovery-config rows")
	require.NotContains(t, got, "More integration rows")
	require.NotContains(t, got, "Next page:")
	require.NotContains(t, got, "Task Breakdown: by issue\n----------------")
	require.Contains(t, got, "# List discovery tasks")
	require.Contains(t, got, "tctl discovery tasks ls")
	require.Contains(t, got, "tctl discovery tasks ls --state=resolved")
	require.Contains(t, got, "# Investigate particular open task")
	require.Contains(t, got, "tctl discovery tasks show e785789e")
	require.Contains(t, got, "tctl discovery tasks show <task-id-prefix>")
	require.Contains(t, got, "# Check SSM runs")
	require.Contains(t, got, "tctl discovery ssm-runs ls")
	require.Contains(t, got, "tctl discovery ssm-runs ls --since=1h")
	require.Contains(t, got, "tctl discovery ssm-runs ls --since=1h --failed")
	require.Contains(t, got, "# Use machine-readable output")
	require.Contains(t, got, "tctl discovery status --format=json")
	require.Contains(t, got, "\n\n  # Investigate particular open task")

	userTasksPos := strings.Index(got, "  User Tasks [")
	discoveryConfigsPos := strings.Index(got, "\n  Discovery Configs")
	integrationsPos := strings.Index(got, "\n  Integration Resource Status")
	require.NotEqual(t, -1, userTasksPos)
	require.NotEqual(t, -1, discoveryConfigsPos)
	require.NotEqual(t, -1, integrationsPos)
	require.Less(t, userTasksPos, discoveryConfigsPos)
	require.Less(t, discoveryConfigsPos, integrationsPos)
}

func TestDiscoveryHumanizeEnumValue(t *testing.T) {
	t.Parallel()

	require.Equal(t, "Syncing", humanizeEnumValue("DISCOVERY_CONFIG_STATE_SYNCING"))
	require.Equal(t, "Running", humanizeEnumValue("RUNNING"))
	require.Equal(t, "Unknown", humanizeEnumValue(""))
}

func TestDiscoveryFormatRelativeTime(t *testing.T) {
	t.Parallel()

	now := mustTestParseTime(t, "2026-02-13T12:00:00Z")
	require.Equal(t, "1m ago", formatRelativeTime(mustTestParseTime(t, "2026-02-13T11:59:00Z"), now))
	require.Equal(t, "2h ago", formatRelativeTime(mustTestParseTime(t, "2026-02-13T10:00:00Z"), now))
	require.Equal(t, "never", formatRelativeTime(time.Time{}, now))
	require.Equal(t, "1m from now", formatRelativeTime(mustTestParseTime(t, "2026-02-13T12:01:00Z"), now))
}

func TestDiscoveryFormatHistoryTimestamp(t *testing.T) {
	t.Parallel()

	now := mustTestParseTime(t, "2026-02-13T12:00:00Z")
	require.Equal(t, "2026-02-13 11:59:00 (1m ago)", formatHistoryTimestamp("2026-02-13 11:59:00", now))
	require.Equal(t, "2026-02-13 09:46:28 (2h 13m ago)", formatHistoryTimestamp("2026-02-13 09:46:28", now))
	require.Equal(t, "raw-value", formatHistoryTimestamp("raw-value", now))
	require.Equal(t, "never", formatHistoryTimestamp("", now))
}

func TestDiscoveryConfigMatchersSummary(t *testing.T) {
	t.Parallel()

	dc := &discoveryconfig.DiscoveryConfig{
		Spec: discoveryconfig.Spec{
			Azure: []types.AzureMatcher{{Types: []string{"vm"}}},
		},
	}
	require.Equal(t, "azure=1", configMatchersSummary(dc))

	empty := &discoveryconfig.DiscoveryConfig{}
	require.Equal(t, "none", configMatchersSummary(empty))
}

func TestDiscoveryAwaitingJoin(t *testing.T) {
	t.Parallel()

	require.EqualValues(t, 1, awaitingJoin(resourcesAggregate{
		Found:    2,
		Enrolled: 0,
		Failed:   1,
	}))
	require.EqualValues(t, 0, awaitingJoin(resourcesAggregate{
		Found:    2,
		Enrolled: 1,
		Failed:   1,
	}))
	require.EqualValues(t, 0, awaitingJoin(resourcesAggregate{
		Found:    1,
		Enrolled: 3,
		Failed:   0,
	}))
}

func TestDiscoveryFormatCountLabel(t *testing.T) {
	t.Parallel()

	require.Equal(t, "1 group", formatCountLabel(1, "group", "groups"))
	require.Equal(t, "2 groups", formatCountLabel(2, "group", "groups"))
}

func TestDiscoveryResourceDisplayRange(t *testing.T) {
	t.Parallel()

	require.Equal(t, 0, func() int { s, _ := resourceDisplayRange(pageInfo{}); return s }())
	require.Equal(t, 0, func() int { _, e := resourceDisplayRange(pageInfo{}); return e }())

	s, e := resourceDisplayRange(pageInfo{Total: 2, Start: 0, End: 2})
	require.Equal(t, 1, s)
	require.Equal(t, 2, e)

	s, e = resourceDisplayRange(pageInfo{Total: 2, Start: 2, End: 2})
	require.Equal(t, 0, s)
	require.Equal(t, 0, e)
}

func TestDiscoveryWithPageFlag(t *testing.T) {
	t.Parallel()

	require.Equal(t, "tctl discovery tasks show e785789e --page-size=2 --page=2", withPageFlag("tctl discovery tasks show e785789e --page-size=2", 2))
	require.Equal(t, "tctl discovery tasks show e785789e --page-size=2 --page=3", withPageFlag("tctl discovery tasks show e785789e --page=1 --page-size=2", 3))
	require.Equal(t, "tctl discovery ssm-runs ls --failed --page=4", withPageFlag("tctl discovery ssm-runs ls --failed", 4))
}

func TestDiscoveryWriteOutputByFormat(t *testing.T) {
	t.Parallel()

	type payload struct {
		Name string `json:"name" yaml:"name"`
	}
	data := payload{Name: "demo"}

	t.Run("text", func(t *testing.T) {
		var out bytes.Buffer
		textCalls := 0

		err := writeOutputByFormat(&out, "text", data, func(w io.Writer) error {
			textCalls++
			_, err := io.WriteString(w, "text-output")
			return err
		})
		require.NoError(t, err)
		require.Equal(t, 1, textCalls)
		require.Equal(t, "text-output", out.String())
	})

	t.Run("json", func(t *testing.T) {
		var out bytes.Buffer
		textCalls := 0

		err := writeOutputByFormat(&out, "json", data, func(io.Writer) error {
			textCalls++
			return nil
		})
		require.NoError(t, err)
		require.Equal(t, 0, textCalls)
		require.Contains(t, out.String(), "\"name\": \"demo\"")
	})

	t.Run("yaml", func(t *testing.T) {
		var out bytes.Buffer
		textCalls := 0

		err := writeOutputByFormat(&out, "yaml", data, func(io.Writer) error {
			textCalls++
			return nil
		})
		require.NoError(t, err)
		require.Equal(t, 0, textCalls)
		require.Contains(t, out.String(), "name: demo")
	})

	t.Run("text renderer required", func(t *testing.T) {
		var out bytes.Buffer
		err := writeOutputByFormat(&out, "text", data, nil)
		require.Error(t, err)
		require.ErrorContains(t, err, "text output renderer is required")
	})

	t.Run("unknown format", func(t *testing.T) {
		var out bytes.Buffer
		err := writeOutputByFormat(&out, "toml", data, func(io.Writer) error { return nil })
		require.Error(t, err)
		require.ErrorContains(t, err, "unknown format")
	})
}

func TestDiscoveryDisplayIntegrationName(t *testing.T) {
	t.Parallel()

	require.Equal(t, "none (ambient credentials)", displayIntegrationName(""))
	require.Equal(t, "none (ambient credentials)", displayIntegrationName("   "))
	require.Equal(t, "integration-a", displayIntegrationName("integration-a"))
}

func TestDiscoveryRenderSSMRunsTextNextGuidance(t *testing.T) {
	t.Parallel()

	output := ssmRunsOutput{
		Window:       "1h",
		TotalRuns:    3,
		SuccessRuns:  1,
		FailedRuns:   2,
		TotalVMs:     2,
		FailingVMs:   1,
		DisplayedVMs: 1,
		VMPage:       pageInfo{Page: 1, PageSize: 25},
		VMs: []ssmVMGroup{
			{
				InstanceID: "i-06b58359e8c2aad58",
				MostRecent: ssmRunRecord{
					InstanceID:      "i-06b58359e8c2aad58",
					Status:          "Failed",
					CommandID:       "cmd-1",
					InvocationURL:   "https://example/cmd-1",
					parsedEventTime: mustTestParseTime(t, "2026-02-13T11:59:00Z"),
				},
				MostRecentFailed: true,
				TotalRuns:        2,
				FailedRuns:       2,
				Runs: []ssmRunRecord{
					{
						InstanceID:      "i-06b58359e8c2aad58",
						Status:          "Failed",
						CommandID:       "cmd-1",
						InvocationURL:   "https://example/cmd-1",
						parsedEventTime: mustTestParseTime(t, "2026-02-13T11:59:00Z"),
					},
				},
			},
		},
	}

	var out bytes.Buffer
	err := renderSSMRunsText(&out, output, "", false, "tctl discovery ssm-runs ls --since=1h")
	require.NoError(t, err)

	got := out.String()
	require.Contains(t, got, "[1] INSTANCE: i-06b58359e8c2aad58")
	require.Contains(t, got, "MOST RECENT:")
	require.Contains(t, got, "RESULT")
	require.Contains(t, got, "RUNS")
	require.Contains(t, got, "FAILED")
	require.Contains(t, got, "[1] VM: i-06b58359e8c2aad58")
	require.Contains(t, got, "RUN:")
	require.Contains(t, got, "TIMESTAMP:")
	require.Contains(t, got, "COMMAND")
	require.Contains(t, got, "EXIT")
	require.Contains(t, got, "ago")
	require.NotContains(t, got, "Status counts:")
	require.NotContains(t, got, "Most recent details:")
	require.NotContains(t, got, "Most recent stderr:")
	require.Contains(t, got, "# Start with SSM overview")
	require.Contains(t, got, "tctl discovery ssm-runs ls")
	require.Contains(t, got, "tctl discovery ssm-runs ls --since=1h")
	require.Contains(t, got, "tctl discovery ssm-runs ls --since=1h --failed")
	require.Contains(t, got, "# View all runs for a specific failing instance")
	require.Contains(t, got, "tctl discovery ssm-runs show i-06b58359e8c2aad58 --show-all-runs")
	require.Contains(t, got, "# Inspect the discovery tasks themselves")
	require.Contains(t, got, "tctl discovery tasks ls --task-type=discover-ec2 --state=open")
	require.Contains(t, got, "# Use machine-readable output")
	require.Contains(t, got, "tctl discovery ssm-runs ls --since=1h --format=json")
	require.Contains(t, got, "\n\n  # View all runs for a specific failing instance")
	require.Contains(t, got, "\n\n  # Inspect the discovery tasks themselves")
}

func TestDiscoveryRenderSSMRunsTextSingleVMHistoryNumbering(t *testing.T) {
	t.Parallel()

	output := ssmRunsOutput{
		Window:       "10h",
		TotalRuns:    2,
		SuccessRuns:  0,
		FailedRuns:   2,
		TotalVMs:     1,
		FailingVMs:   1,
		DisplayedVMs: 1,
		VMPage:       pageInfo{Page: 1, PageSize: 1},
		VMs: []ssmVMGroup{
			{
				InstanceID: "i-003a23be4c3d13fa8",
				MostRecent: ssmRunRecord{
					InstanceID:      "i-003a23be4c3d13fa8",
					Status:          "Failed",
					CommandID:       "cmd-2",
					parsedEventTime: mustTestParseTime(t, "2026-02-13T17:43:33Z"),
				},
				MostRecentFailed: true,
				TotalRuns:        2,
				FailedRuns:       2,
				Runs: []ssmRunRecord{
					{
						InstanceID:      "i-003a23be4c3d13fa8",
						Status:          "Failed",
						CommandID:       "cmd-2",
						ExitCode:        "-1",
						parsedEventTime: mustTestParseTime(t, "2026-02-13T17:43:33Z"),
					},
					{
						InstanceID:      "i-003a23be4c3d13fa8",
						Status:          "Failed",
						CommandID:       "cmd-1",
						ExitCode:        "-1",
						parsedEventTime: mustTestParseTime(t, "2026-02-13T16:41:36Z"),
					},
				},
			},
		},
	}

	var out bytes.Buffer
	err := renderSSMRunsText(&out, output, "i-003a23be4c3d13fa8", true, "tctl discovery ssm-runs show i-003a23be4c3d13fa8 --since=10h")
	require.NoError(t, err)

	got := out.String()
	require.Contains(t, got, "VM:")
	require.Contains(t, got, "INSTANCE: i-003a23be4c3d13fa8")
	require.Contains(t, got, "Run history:")
	require.Contains(t, got, "[1] TIMESTAMP: 2026-02-13 17:43:33")
	require.Contains(t, got, "[2] TIMESTAMP: 2026-02-13 16:41:36")
	require.Contains(t, got, "    RESULT")
	require.Contains(t, got, "    COMMAND")
	require.Contains(t, got, "    EXIT")
	require.NotContains(t, got, "[1] INSTANCE:")
	require.NotContains(t, got, "[1] VM:")
	require.NotContains(t, got, "[1] RUN:")
	require.NotContains(t, got, "RUN 1:")
}

func TestDiscoveryRenderSSMRunHistoryRows(t *testing.T) {
	t.Parallel()

	now := mustTestParseTime(t, "2026-02-13T18:00:00Z")
	rows := []ssmRunHistoryRow{
		{
			Timestamp: "2026-02-13 17:43:33",
			Result:    "Failed",
			CommandID: "cmd-2",
			ExitCode:  "-1",
		},
		{
			Timestamp: "2026-02-13 16:41:36",
			Result:    "Failed",
			CommandID: "cmd-1",
			ExitCode:  "-1",
		},
	}
	style := textStyle{enabled: false}

	t.Run("single instance format", func(t *testing.T) {
		var out bytes.Buffer
		err := renderSSMRunHistoryRows(&out, style, rows, now, true)
		require.NoError(t, err)

		got := out.String()
		require.Contains(t, got, "[1] TIMESTAMP: 2026-02-13 17:43:33")
		require.Contains(t, got, "[2] TIMESTAMP: 2026-02-13 16:41:36")
		require.Contains(t, got, "RESULT")
		require.Contains(t, got, "COMMAND")
		require.Contains(t, got, "EXIT")
		require.NotContains(t, got, "RUN 1:")
	})

	t.Run("multi instance format", func(t *testing.T) {
		var out bytes.Buffer
		err := renderSSMRunHistoryRows(&out, style, rows, now, false)
		require.NoError(t, err)

		got := out.String()
		require.Contains(t, got, "  RUN 1:")
		require.Contains(t, got, "  RUN 2:")
		require.Contains(t, got, "TIMESTAMP")
		require.NotContains(t, got, "[1] TIMESTAMP")
	})
}

func TestDiscoveryFormatRelativeOrTimestamp(t *testing.T) {
	t.Parallel()

	now := mustTestParseTime(t, "2026-02-13T12:00:00Z")
	require.Equal(t, "1m ago", formatRelativeOrTimestamp(mustTestParseTime(t, "2026-02-13T11:59:00Z"), "", now))
	require.Equal(t, "timestamp: 2026-02-13T11:59:00Z", formatRelativeOrTimestamp(time.Time{}, "2026-02-13T11:59:00Z", now))
	require.Equal(t, "never", formatRelativeOrTimestamp(time.Time{}, "", now))
}

func TestDiscoveryTextStyle(t *testing.T) {
	t.Parallel()

	plain := textStyle{enabled: false}
	require.Equal(t, "Summary", plain.section("Summary"))
	require.Equal(t, "Failed", plain.statusValue("Failed"))

	colored := textStyle{enabled: true}
	section := colored.section("Summary")
	require.Contains(t, section, "\x1b[")
	require.True(t, strings.HasSuffix(section, "\x1b[0m"))
	require.Contains(t, colored.statusValue("Success"), "[32m")
	require.Contains(t, colored.statusValue("Failed"), "[31m")
	require.Contains(t, colored.statusValue("TimedOut"), "[33m")
	require.Contains(t, colored.statusValue("Syncing"), "[33m")
	require.Contains(t, colored.discoveredCount(0), "[33m")
	require.Contains(t, colored.discoveredCount(2), "[32m")
}

func TestDiscoveryColorEnabledForceColor(t *testing.T) {
	var out bytes.Buffer

	t.Setenv("FORCE_COLOR", "1")
	t.Setenv("NO_COLOR", "1")
	t.Setenv("TERM", "dumb")
	require.True(t, colorEnabled(&out))

	t.Setenv("FORCE_COLOR", "0")
	require.False(t, colorEnabled(&out))
}

func TestDiscoveryTaskNamePrefix(t *testing.T) {
	t.Parallel()

	require.Equal(t, "e785789e", taskNamePrefix("e785789e-4fbc-5774-a3d3-4e34edc80dbd"))
	require.Equal(t, "f1234567", taskNamePrefix("f1234567-1111-2222-3333-444444444444"))
	require.Equal(t, "discover-ec2-task-42", taskNamePrefix("discover-ec2-task-42"))
	require.Equal(t, "discover-ec2-task-42", taskNamePrefix(" discover-ec2-task-42... "))
}

func TestDiscoveryFriendlyTaskType(t *testing.T) {
	t.Parallel()

	require.Equal(t, "AWS EC2", friendlyTaskType(usertasksapi.TaskTypeDiscoverEC2))
	require.Equal(t, "AWS EKS", friendlyTaskType(usertasksapi.TaskTypeDiscoverEKS))
	require.Equal(t, "AWS RDS", friendlyTaskType(usertasksapi.TaskTypeDiscoverRDS))
	require.Equal(t, "Azure VM", friendlyTaskType(usertasksapi.TaskTypeDiscoverAzureVM))
	require.Equal(t, "unknown-type", friendlyTaskType("unknown-type"))
}

func TestDiscoveryFindTaskByNamePrefix(t *testing.T) {
	t.Parallel()

	tasks := []*usertasksv1.UserTask{
		{Metadata: &headerv1.Metadata{Name: "e785789e-4fbc-5774-a3d3-4e34edc80dbd"}},
		{Metadata: &headerv1.Metadata{Name: "f1234567-1111-2222-3333-444444444444"}},
	}

	task, err := findTaskByNamePrefix(tasks, "e785789e-")
	require.NoError(t, err)
	require.Equal(t, "e785789e-4fbc-5774-a3d3-4e34edc80dbd", task.GetMetadata().GetName())

	task, err = findTaskByNamePrefix(tasks, "e785789e-...")
	require.NoError(t, err)
	require.Equal(t, "e785789e-4fbc-5774-a3d3-4e34edc80dbd", task.GetMetadata().GetName())

	_, err = findTaskByNamePrefix(tasks, "deadbeef-")
	require.Error(t, err)
	require.ErrorContains(t, err, "not found")
}

func TestDiscoveryFindTaskByNamePrefixAmbiguous(t *testing.T) {
	t.Parallel()

	tasks := []*usertasksv1.UserTask{
		{Metadata: &headerv1.Metadata{Name: "e785789e-4fbc-5774-a3d3-4e34edc80dbd"}},
		{Metadata: &headerv1.Metadata{Name: "e785789e-7777-8888-9999-aaaaaaaaaaaa"}},
	}

	_, err := findTaskByNamePrefix(tasks, "e785789e-")
	require.Error(t, err)
	require.Contains(t, err.Error(), "ambiguous")
}

func mustTestParseTime(t *testing.T, raw string) time.Time {
	t.Helper()
	parsed, ok := parseAuditEventTime(raw)
	require.True(t, ok)
	return parsed
}
