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
	discoveryconfigv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/discoveryconfig/v1"
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

func TestClusterSSMRuns(t *testing.T) {
	t.Parallel()

	t.Run("clusters by stdout/stderr", func(t *testing.T) {
		vmGroups := []ssmVMGroup{
			{
				InstanceID: "i-aaa",
				Runs: []ssmRunRecord{
					{InstanceID: "i-aaa", AccountID: "111", Region: "us-east-1", Status: "Failed", Code: "TDS00W", Stderr: "permission denied", parsedEventTime: mustTestParseTime(t, "2026-02-12T10:00:00Z")},
					{InstanceID: "i-aaa", AccountID: "111", Region: "us-east-1", Status: "Failed", Code: "TDS00W", Stderr: "permission denied", parsedEventTime: mustTestParseTime(t, "2026-02-12T11:00:00Z")},
				},
			},
			{
				InstanceID: "i-bbb",
				Runs: []ssmRunRecord{
					{InstanceID: "i-bbb", AccountID: "111", Region: "us-west-2", Status: "Failed", Code: "TDS00W", Stderr: "permission denied", parsedEventTime: mustTestParseTime(t, "2026-02-12T10:30:00Z")},
				},
			},
		}

		errors, successes := clusterSSMRuns(vmGroups, clusterDefaults())
		require.Empty(t, successes)
		require.Len(t, errors, 1, "identical errors should form one cluster")
		require.Len(t, errors[0].Instances, 2, "two distinct instances")

		// Instances should be sorted by instance ID.
		require.Equal(t, "i-aaa", errors[0].Instances[0].InstanceID)
		require.Equal(t, "i-bbb", errors[0].Instances[1].InstanceID)

		// i-aaa should have 2 sorted times.
		require.Len(t, errors[0].Instances[0].Times, 2)
		require.Equal(t, "us-east-1", errors[0].Instances[0].Region)
	})

	t.Run("falls back to status when stdout/stderr empty", func(t *testing.T) {
		vmGroups := []ssmVMGroup{
			{
				InstanceID: "i-ccc",
				Runs: []ssmRunRecord{
					{InstanceID: "i-ccc", AccountID: "222", Region: "ca-central-1", Status: "EC2 Instance is not registered in SSM.", Code: "TDS00W", ExitCode: "-1", parsedEventTime: mustTestParseTime(t, "2026-02-12T10:00:00Z")},
					{InstanceID: "i-ccc", AccountID: "222", Region: "ca-central-1", Status: "EC2 Instance is not registered in SSM.", Code: "TDS00W", ExitCode: "-1", parsedEventTime: mustTestParseTime(t, "2026-02-12T11:00:00Z")},
				},
			},
		}

		errors, successes := clusterSSMRuns(vmGroups, clusterDefaults())
		require.Empty(t, successes)
		require.Len(t, errors, 1, "status-only errors should be clustered")
		require.Len(t, errors[0].Instances, 1)
		require.Equal(t, "i-ccc", errors[0].Instances[0].InstanceID)
		require.Len(t, errors[0].Instances[0].Times, 2)
	})

	t.Run("no output and no status produces no clusters", func(t *testing.T) {
		vmGroups := []ssmVMGroup{
			{
				InstanceID: "i-ddd",
				Runs: []ssmRunRecord{
					{InstanceID: "i-ddd", Status: "", Code: "TDS00W", parsedEventTime: mustTestParseTime(t, "2026-02-12T10:00:00Z")},
				},
			},
		}
		errors, successes := clusterSSMRuns(vmGroups, clusterDefaults())
		require.Empty(t, errors)
		require.Empty(t, successes)
	})

	t.Run("successful runs clustered separately", func(t *testing.T) {
		vmGroups := []ssmVMGroup{
			{
				InstanceID: "i-eee",
				Runs: []ssmRunRecord{
					{InstanceID: "i-eee", Status: "Success", Code: "TDS00I", Stdout: "installed ok", parsedEventTime: mustTestParseTime(t, "2026-02-12T10:00:00Z")},
				},
			},
		}
		errors, successes := clusterSSMRuns(vmGroups, clusterDefaults())
		require.Empty(t, errors)
		require.Len(t, successes, 1, "successful runs should be clustered")
		require.Equal(t, "i-eee", successes[0].Instances[0].InstanceID)
	})

	t.Run("mixed runs split into separate buckets", func(t *testing.T) {
		vmGroups := []ssmVMGroup{
			{
				InstanceID: "i-fff",
				Runs: []ssmRunRecord{
					{InstanceID: "i-fff", AccountID: "333", Region: "eu-west-1", Status: "Failed", Code: "TDS00W", Stderr: "timeout", parsedEventTime: mustTestParseTime(t, "2026-02-12T10:00:00Z")},
					{InstanceID: "i-fff", AccountID: "333", Region: "eu-west-1", Status: "Success", Code: "TDS00I", Stdout: "installed ok", parsedEventTime: mustTestParseTime(t, "2026-02-12T11:00:00Z")},
				},
			},
		}
		errors, successes := clusterSSMRuns(vmGroups, clusterDefaults())
		require.Len(t, errors, 1, "one error cluster")
		require.Len(t, successes, 1, "one success cluster")
	})
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
	info, err := renderEC2Details(&out, task, 0, 25)
	require.NoError(t, err)
	require.Equal(t, 2, info.Total)

	got := out.String()
	require.Contains(t, got, "Affected EC2 instances:")
	require.Contains(t, got, "[1] INSTANCE        : i-003a23be4c3d13fa8")
	require.Contains(t, got, "[2] INSTANCE        : i-085159ed62c5364c2")
	require.Contains(t, got, "    NAME            : target-1")
	require.Contains(t, got, "    REGION          : eu-central-1")
	require.Contains(t, got, "    DISCOVERY CONFIG: cfg-main")
	require.Contains(t, got, "    DISCOVERY GROUP : main")
	require.Contains(t, got, "    SYNC TIME       :")
	require.Contains(t, got, "    INVOCATION URL  : https://example.test/invocation")
	require.Contains(t, got, "\n\n[2] INSTANCE        :")
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
	err := renderTaskDetailsText(&out, task, 0, 25, "tctl discovery tasks show e785789e --range=0,25")
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
	err := renderTaskDetailsText(&out, task, 0, 2, "tctl discovery tasks show e785789e --range=0,2")
	require.NoError(t, err)

	got := out.String()
	require.Contains(t, got, "Showing resources: 1-2 of 3.")
	require.Contains(t, got, "# Show next page of affected resources")
	require.Contains(t, got, "tctl discovery tasks show e785789e --range=2,3")
	require.NotContains(t, got, "--range=0,2 --range=2,3")
	require.NotContains(t, got, "More resources available")
	require.NotContains(t, got, "Next page:")
	require.Contains(t, got, "# Inspect this integration")
	require.Contains(t, got, "tctl discovery integration show integration-1")
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
	err := renderTaskDetailsText(&out, task, 999, 1024, "tctl discovery tasks show e785789e --range=999,1024")
	require.NoError(t, err)

	got := out.String()
	require.Contains(t, got, "Showing resources: 0-0 of 2.")
	require.Contains(t, got, "# Current resource page is out of range")
	require.Contains(t, got, "tctl discovery tasks show e785789e --range=0,2")
	require.NotContains(t, got, "--range=999,1024 --range=0,2")
}

func TestDiscoveryPaginateSlice(t *testing.T) {
	t.Parallel()

	items := []int{1, 2, 3, 4, 5}
	pageItems, info := paginateSlice(items, 2, 4)
	require.Equal(t, []int{3, 4}, pageItems)
	require.Equal(t, 5, info.Total)
	require.Equal(t, 2, info.Start)
	require.Equal(t, 4, info.End)
	require.Equal(t, 1, info.Remaining)
	require.True(t, info.HasNext)

	allItems, info2 := paginateSlice(items, 0, 5)
	require.Equal(t, items, allItems)
	require.Equal(t, 5, info2.Total)
	require.Equal(t, 0, info2.Remaining)
	require.False(t, info2.HasNext)

	clamped, info3 := paginateSlice(items, 3, 100)
	require.Equal(t, []int{4, 5}, clamped)
	require.Equal(t, 3, info3.Start)
	require.Equal(t, 5, info3.End)
	require.False(t, info3.HasNext)
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
	require.Contains(t, got, "[1] TASK             : e785789e-4fbc-5774-a3d3-4e34edc80dbd")
	require.Contains(t, got, "[2] TASK             : f1234567-1111-2222-3333-444444444444")
	require.Contains(t, got, "[3] TASK             : a1234567-1111-2222-3333-444444444444")
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
	require.Contains(t, got, "# List integrations")
	require.Contains(t, got, "tctl discovery integration ls")
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
		Integrations: []integrationListItem{
			{Name: "integration-active", Type: "AWS OIDC", Found: 2, Enrolled: 0, Failed: 1, AwaitingJoin: 1, OpenTasks: 1},
			{Name: "integration-idle", Type: "Azure OIDC", Found: 0, Enrolled: 0, Failed: 0, AwaitingJoin: 0, OpenTasks: 0},
		},
		SSMRunStats: &auditEventStats{
			Window:        "last 24h",
			Total:         15,
			Success:       12,
			Failed:        3,
			DistinctHosts: 5,
			FailingHosts:  2,
		},
		JoinStats: &auditEventStats{
			Window:          "last 24h",
			EffectiveWindow: "6h ago",
			SuggestedLimit:  84,
			Total:           20,
			Success:         18,
			Failed:          2,
			DistinctHosts:   8,
			FailingHosts:    1,
			LimitReached:    true,
		},
	}

	var out bytes.Buffer
	err := renderStatusText(&out, summary)
	require.NoError(t, err)

	got := out.String()
	require.Contains(t, got, "User Tasks")
	require.Contains(t, got, "User Tasks [2 total, 2 open, 0 resolved]")
	require.Contains(t, got, "┌")
	require.Contains(t, got, "Discovery Configs")
	require.Contains(t, got, "Discovery Configs [2 total, 1 group]")
	require.Contains(t, got, "Integrations [2 total, 1 active]")
	require.Contains(t, got, "integration-active")
	require.Contains(t, got, "integration-idle")
	require.Contains(t, got, "AWS OIDC")
	require.Contains(t, got, "Azure OIDC")
	require.Contains(t, got, "Awaiting Join")
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
	require.Contains(t, got, "discover-azure-vm")
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
	require.Contains(t, got, "SSM Runs (last 24h)")
	require.Contains(t, got, "TOTAL EVENTS       : 15")
	require.Contains(t, got, "SUCCESSFUL         : 12")
	require.Contains(t, got, "DISTINCT HOSTS     : 5")
	require.Contains(t, got, "HOSTS WITH FAILURES: 2")
	require.Contains(t, got, "Instance Joins (requested last 24h, oldest returned 6h ago; use --join-limit=84)")
	require.Contains(t, got, "TOTAL EVENTS       : 20")
	require.Contains(t, got, "# List discovery tasks")
	require.Contains(t, got, "tctl discovery tasks ls")
	require.Contains(t, got, "tctl discovery tasks ls --state=resolved")
	require.Contains(t, got, "# Investigate particular open task")
	require.Contains(t, got, "tctl discovery tasks show e785789e")
	require.Contains(t, got, "tctl discovery tasks show <task-id-prefix>")
	require.Contains(t, got, "# List integrations")
	require.Contains(t, got, "tctl discovery integration ls")
	require.Contains(t, got, "# Check SSM runs")
	require.Contains(t, got, "tctl discovery ssm-runs ls")
	require.Contains(t, got, "tctl discovery ssm-runs ls --last=1h")
	require.Contains(t, got, "# Use machine-readable output")
	require.Contains(t, got, "tctl discovery status --format=json")
	require.Contains(t, got, "\n\n  # Investigate particular open task")

	userTasksPos := strings.Index(got, "  User Tasks [")
	discoveryConfigsPos := strings.Index(got, "\n  Discovery Configs")
	integrationsPos := strings.Index(got, "\n  Integrations [")
	require.NotEqual(t, -1, userTasksPos)
	require.NotEqual(t, -1, discoveryConfigsPos)
	require.NotEqual(t, -1, integrationsPos)
	require.Less(t, userTasksPos, discoveryConfigsPos)
	require.Less(t, discoveryConfigsPos, integrationsPos)
}

func TestRoundUpSignificant(t *testing.T) {
	t.Parallel()
	tests := []struct {
		n, digits, want int
	}{
		{1234567, 4, 1235000},
		{1234000, 4, 1234000},
		{84, 4, 84},
		{999, 4, 999},
		{9999, 4, 9999},
		{10000, 4, 10000},
		{10001, 4, 10010},
		{123, 2, 130},
		{99, 1, 100},
		{50, 1, 50},
		{0, 4, 0},
		{5, 4, 5},
	}
	for _, tt := range tests {
		got := roundUpSignificant(tt.n, tt.digits)
		require.Equal(t, tt.want, got, "roundUpSignificant(%d, %d)", tt.n, tt.digits)
	}
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

func TestDiscoveryWithRangeFlag(t *testing.T) {
	t.Parallel()

	require.Equal(t, "tctl discovery tasks show e785789e --range=2,4", withRangeFlag("tctl discovery tasks show e785789e", 2, 4))
	require.Equal(t, "tctl discovery tasks show e785789e --range=25,50", withRangeFlag("tctl discovery tasks show e785789e --range=0,25", 25, 50))
	require.Equal(t, "tctl discovery ssm-runs ls --range=0,25", withRangeFlag("tctl discovery ssm-runs ls", 0, 25))
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
		Window:       "last 1h",
		TotalRuns:    3,
		SuccessRuns:  1,
		FailedRuns:   2,
		TotalVMs:     2,
		FailingVMs:   1,
		VMPage:       pageInfo{Start: 0, End: 25},
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
	err := renderSSMRunsText(&out, output, "", false, "tctl discovery ssm-runs ls --last=1h")
	require.NoError(t, err)

	got := out.String()
	require.Contains(t, got, "[1] INSTANCE   : i-06b58359e8c2aad58")
	require.Contains(t, got, "MOST RECENT:")
	require.Contains(t, got, "RESULT")
	require.Contains(t, got, "RUNS")
	require.Contains(t, got, "FAILED")
	require.Contains(t, got, "ago")
	// History section is hidden in ls view without --show-all-runs when no output is present.
	require.NotContains(t, got, "Run history:")
	require.NotContains(t, got, "Status counts:")
	require.NotContains(t, got, "Most recent details:")
	require.NotContains(t, got, "Most recent stderr:")
	require.Contains(t, got, "# Adjust SSM time window")
	require.Contains(t, got, "tctl discovery ssm-runs ls --last=1h")
	require.Contains(t, got, "# View all runs for a specific failing instance")
	require.Contains(t, got, "tctl discovery ssm-runs show i-06b58359e8c2aad58 --show-all-runs")
	require.Contains(t, got, "# Inspect the discovery tasks themselves")
	require.Contains(t, got, "tctl discovery tasks ls --task-type=discover-ec2 --state=open")
	require.Contains(t, got, "# List integrations")
	require.Contains(t, got, "tctl discovery integration ls")
	require.Contains(t, got, "# Use machine-readable output")
	require.Contains(t, got, "tctl discovery ssm-runs ls --last=1h --format=json")
	require.Contains(t, got, "\n\n  # View all runs for a specific failing instance")
	require.Contains(t, got, "\n\n  # Inspect the discovery tasks themselves")
}

func TestDiscoveryRenderSSMRunsTextSingleVMHistoryNumbering(t *testing.T) {
	t.Parallel()

	output := ssmRunsOutput{
		Window:       "last 10h",
		TotalRuns:    2,
		SuccessRuns:  0,
		FailedRuns:   2,
		TotalVMs:     1,
		FailingVMs:   1,
		VMPage:       pageInfo{Start: 0, End: 1, Total: 1},
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
	err := renderSSMRunsText(&out, output, "i-003a23be4c3d13fa8", true, "tctl discovery ssm-runs show i-003a23be4c3d13fa8 --last=10h")
	require.NoError(t, err)

	got := out.String()
	require.Contains(t, got, "VM:")
	require.Contains(t, got, "INSTANCE   : i-003a23be4c3d13fa8")
	require.Contains(t, got, "Run history:")
	require.Contains(t, got, "[1] TIMESTAMP")
	require.Contains(t, got, "[2] TIMESTAMP")
	require.Contains(t, got, "TIMESTAMP: 2026-02-13 17:43:33")
	require.Contains(t, got, "TIMESTAMP: 2026-02-13 16:41:36")
	require.Contains(t, got, "  RESULT")
	require.Contains(t, got, "  COMMAND")
	require.Contains(t, got, "  EXIT")
	require.Contains(t, got, "[1] INSTANCE")
	require.Contains(t, got, "[1] VM:")
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
	t.Run("no indent", func(t *testing.T) {
		var out bytes.Buffer
		err := renderSSMRunHistoryRows(&out, textStyle{}, rows, now)
		require.NoError(t, err)

		got := out.String()
		require.Contains(t, got, "[1] TIMESTAMP")
		require.Contains(t, got, "[2] TIMESTAMP")
		require.Contains(t, got, "RESULT")
		require.Contains(t, got, "COMMAND")
		require.Contains(t, got, "EXIT")
	})

	t.Run("with indent", func(t *testing.T) {
		var out bytes.Buffer
		indented := textStyle{indent: "     "}
		err := renderSSMRunHistoryRows(&out, indented, rows, now)
		require.NoError(t, err)

		got := out.String()
		require.Contains(t, got, "     [1] TIMESTAMP")
		require.Contains(t, got, "     [2] TIMESTAMP")
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

func TestDiscoveryParseRange(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		input     string
		wantStart int
		wantEnd   int
		wantErr   string
	}{
		{name: "valid", input: "0,25", wantStart: 0, wantEnd: 25},
		{name: "mid range", input: "10,20", wantStart: 10, wantEnd: 20},
		{name: "same start and end", input: "5,5", wantStart: 5, wantEnd: 5},
		{name: "with spaces", input: " 0 , 25 ", wantStart: 0, wantEnd: 25},
		{name: "missing comma", input: "025", wantErr: "invalid range"},
		{name: "negative start", input: "-1,25", wantErr: "non-negative"},
		{name: "end less than start", input: "25,10", wantErr: "must be >= start"},
		{name: "non-numeric", input: "a,b", wantErr: "invalid range start"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			start, end, err := parseRange(tc.input)
			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.wantStart, start)
			require.Equal(t, tc.wantEnd, end)
		})
	}
}

func TestDiscoveryResolveTimeRange(t *testing.T) {
	t.Parallel()

	t.Run("default to 1h", func(t *testing.T) {
		from, to, err := resolveTimeRangeFromFlags("", "", "")
		require.NoError(t, err)
		require.InDelta(t, time.Hour.Seconds(), to.Sub(from).Seconds(), 1)
	})

	t.Run("last flag", func(t *testing.T) {
		from, to, err := resolveTimeRangeFromFlags("30m", "", "")
		require.NoError(t, err)
		require.InDelta(t, (30 * time.Minute).Seconds(), to.Sub(from).Seconds(), 1)
	})

	t.Run("from-utc and to-utc", func(t *testing.T) {
		from, to, err := resolveTimeRangeFromFlags("", "2026-02-15T08:00", "2026-02-15T20:00")
		require.NoError(t, err)
		require.Equal(t, "2026-02-15T08:00", from.Format("2006-01-02T15:04"))
		require.Equal(t, "2026-02-15T20:00", to.Format("2006-01-02T15:04"))
	})

	t.Run("mutual exclusivity", func(t *testing.T) {
		_, _, err := resolveTimeRangeFromFlags("1h", "2026-02-10", "")
		require.ErrorContains(t, err, "cannot be combined")
	})

	t.Run("invalid last", func(t *testing.T) {
		_, _, err := resolveTimeRangeFromFlags("bad", "", "")
		require.ErrorContains(t, err, "invalid --last value")
	})

	t.Run("invalid from-utc", func(t *testing.T) {
		_, _, err := resolveTimeRangeFromFlags("", "not-a-date", "")
		require.ErrorContains(t, err, "invalid --from-utc value")
	})
}

func mustTestParseTime(t *testing.T, raw string) time.Time {
	t.Helper()
	parsed, ok := parseAuditEventTime(raw)
	require.True(t, ok)
	return parsed
}

func TestDiscoveryIntegrationUsesLsAndShowSubcommands(t *testing.T) {
	t.Parallel()

	cfg := servicecfg.MakeDefaultConfig()
	cfg.CircuitBreakerConfig = breaker.NoopBreakerConfig()

	var cmd Command
	app := utils.InitCLIParser("tctl", "tctl test")
	cmd.Initialize(app, &tctlcfg.GlobalCLIFlags{}, cfg)

	selected, err := app.Parse([]string{"discovery", "integration", "ls"})
	require.NoError(t, err)
	require.Equal(t, "discovery integration ls", selected)

	selected, err = app.Parse([]string{"discovery", "integration", "show", "my-integration"})
	require.NoError(t, err)
	require.Equal(t, "discovery integration show", selected)

	_, err = app.Parse([]string{"discovery", "integration"})
	require.Error(t, err)
}

func TestDiscoveryFriendlyIntegrationType(t *testing.T) {
	t.Parallel()

	require.Equal(t, "AWS OIDC", friendlyIntegrationType(types.IntegrationSubKindAWSOIDC))
	require.Equal(t, "Azure OIDC", friendlyIntegrationType(types.IntegrationSubKindAzureOIDC))
	require.Equal(t, "GitHub", friendlyIntegrationType(types.IntegrationSubKindGitHub))
	require.Equal(t, "AWS Roles Anywhere", friendlyIntegrationType(types.IntegrationSubKindAWSRolesAnywhere))
	require.Equal(t, "custom-kind", friendlyIntegrationType("custom-kind"))
	require.Equal(t, "Unknown", friendlyIntegrationType(""))
}

func TestDiscoveryIntegrationCredentialDetails(t *testing.T) {
	t.Parallel()

	t.Run("aws-oidc", func(t *testing.T) {
		ig, err := types.NewIntegrationAWSOIDC(types.Metadata{Name: "aws-test"}, &types.AWSOIDCIntegrationSpecV1{
			RoleARN: "arn:aws:iam::123456789012:role/MyRole",
		})
		require.NoError(t, err)
		creds := integrationCredentialDetails(ig)
		require.Equal(t, "arn:aws:iam::123456789012:role/MyRole", creds["Role ARN"])
	})

	t.Run("azure-oidc", func(t *testing.T) {
		ig, err := types.NewIntegrationAzureOIDC(types.Metadata{Name: "azure-test"}, &types.AzureOIDCIntegrationSpecV1{
			TenantID: "tenant-123",
			ClientID: "client-456",
		})
		require.NoError(t, err)
		creds := integrationCredentialDetails(ig)
		require.Equal(t, "tenant-123", creds["Tenant ID"])
		require.Equal(t, "client-456", creds["Client ID"])
	})

	t.Run("github", func(t *testing.T) {
		ig, err := types.NewIntegrationGitHub(types.Metadata{Name: "gh-test"}, &types.GitHubIntegrationSpecV1{
			Organization: "my-org",
		})
		require.NoError(t, err)
		creds := integrationCredentialDetails(ig)
		require.Equal(t, "my-org", creds["Organization"])
	})
}

func TestDiscoveryBuildIntegrationStatsMap(t *testing.T) {
	t.Parallel()

	dcs := []*discoveryconfig.DiscoveryConfig{
		{
			Status: discoveryconfig.Status{
				IntegrationDiscoveredResources: map[string]*discoveryconfigv1.IntegrationDiscoveredSummary{
					"integration-a": {
						AwsEc2: &discoveryconfigv1.ResourcesDiscoveredSummary{Found: 10, Enrolled: 5, Failed: 2},
						AwsRds: &discoveryconfigv1.ResourcesDiscoveredSummary{Found: 3, Enrolled: 1, Failed: 0},
					},
				},
			},
		},
		{
			Status: discoveryconfig.Status{
				IntegrationDiscoveredResources: map[string]*discoveryconfigv1.IntegrationDiscoveredSummary{
					"integration-a": {
						AwsEc2: &discoveryconfigv1.ResourcesDiscoveredSummary{Found: 5, Enrolled: 3, Failed: 1},
					},
					"integration-b": {
						AwsEks: &discoveryconfigv1.ResourcesDiscoveredSummary{Found: 2, Enrolled: 2, Failed: 0},
					},
				},
			},
		},
	}

	statsMap := buildIntegrationStatsMap(dcs)
	require.Len(t, statsMap, 2)

	a := statsMap["integration-a"]
	require.EqualValues(t, 18, a.Found)
	require.EqualValues(t, 9, a.Enrolled)
	require.EqualValues(t, 3, a.Failed)

	b := statsMap["integration-b"]
	require.EqualValues(t, 2, b.Found)
	require.EqualValues(t, 2, b.Enrolled)
	require.EqualValues(t, 0, b.Failed)
}

func TestDiscoveryToIntegrationListItems(t *testing.T) {
	t.Parallel()

	igA, err := types.NewIntegrationAWSOIDC(types.Metadata{Name: "alpha"}, &types.AWSOIDCIntegrationSpecV1{RoleARN: "arn"})
	require.NoError(t, err)
	igB, err := types.NewIntegrationAzureOIDC(types.Metadata{Name: "bravo"}, &types.AzureOIDCIntegrationSpecV1{TenantID: "t", ClientID: "c"})
	require.NoError(t, err)
	igC, err := types.NewIntegrationGitHub(types.Metadata{Name: "charlie"}, &types.GitHubIntegrationSpecV1{Organization: "org"})
	require.NoError(t, err)

	statsMap := map[string]resourcesAggregate{
		"alpha":   {Found: 10, Enrolled: 5, Failed: 3},
		"bravo":   {Found: 20, Enrolled: 10, Failed: 0},
		"charlie": {Found: 5, Enrolled: 2, Failed: 3},
	}
	taskCountMap := map[string]int{
		"alpha":   2,
		"charlie": 1,
	}

	items := toIntegrationListItems([]types.Integration{igA, igB, igC}, statsMap, taskCountMap)
	require.Len(t, items, 3)
	// Sorted by Failed desc, Found desc, Name asc.
	// alpha: Failed=3, Found=10; charlie: Failed=3, Found=5; bravo: Failed=0, Found=20
	require.Equal(t, "alpha", items[0].Name)
	require.Equal(t, "charlie", items[1].Name)
	require.Equal(t, "bravo", items[2].Name)
	require.EqualValues(t, 2, items[0].AwaitingJoin) // 10 - 5 - 3
	require.Equal(t, 2, items[0].OpenTasks)
}

func TestDiscoveryRenderIntegrationListText(t *testing.T) {
	t.Parallel()

	items := []integrationListItem{
		{Name: "my-aws-integration", Type: "AWS OIDC", Found: 10, Enrolled: 5, Failed: 2, AwaitingJoin: 3, OpenTasks: 1},
		{Name: "my-azure-integration", Type: "Azure OIDC", Found: 5, Enrolled: 5, Failed: 0, AwaitingJoin: 0, OpenTasks: 0},
	}

	var out bytes.Buffer
	err := renderIntegrationListText(&out, items)
	require.NoError(t, err)

	got := out.String()
	require.Contains(t, got, "Integrations [2]")
	require.Contains(t, got, "my-aws-integration")
	require.Contains(t, got, "my-azure-integration")
	require.Contains(t, got, "AWS OIDC")
	require.Contains(t, got, "Azure OIDC")
	require.Contains(t, got, "Found")
	require.Contains(t, got, "Enrolled")
	require.Contains(t, got, "Failed")
	require.Contains(t, got, "Awaiting Join")
	require.Contains(t, got, "Open Tasks")
	require.Contains(t, got, "# Inspect an integration")
	require.Contains(t, got, "tctl discovery integration show my-aws-integration")
	require.Contains(t, got, "# List discovery tasks")
	require.Contains(t, got, "tctl discovery tasks ls")
	require.Contains(t, got, "# Check discovery status")
	require.Contains(t, got, "tctl discovery status")
	require.Contains(t, got, "# Use machine-readable output")
	require.Contains(t, got, "tctl discovery integration ls --format=json")
}

func TestDiscoveryRenderIntegrationShowText(t *testing.T) {
	t.Parallel()

	detail := integrationDetail{
		Name: "my-aws-integration",
		Type: "AWS OIDC",
		Credentials: map[string]string{
			"Role ARN": "arn:aws:iam::123456789012:role/MyRole",
		},
		ResourceTypeStats: []resourceTypeStatsRow{
			{ResourceType: "EC2", Found: 10, Enrolled: 5, Failed: 2},
			{ResourceType: "RDS", Found: 3, Enrolled: 1, Failed: 0},
		},
		DiscoveryConfigs: []configStatus{
			{Name: "dc-main", State: "RUNNING", Matchers: "aws=2", LastSync: mustTestParseTime(t, "2026-02-13T11:58:00Z")},
		},
		OpenTasks: []taskListItem{
			{Name: "e785789e-4fbc-5774-a3d3-4e34edc80dbd", IssueType: "ec2-ssm-script-failure", Affected: 3},
		},
	}

	var out bytes.Buffer
	err := renderIntegrationShowText(&out, detail)
	require.NoError(t, err)

	got := out.String()
	require.Contains(t, got, "NAME    : my-aws-integration")
	require.Contains(t, got, "TYPE    : AWS OIDC")
	require.Contains(t, got, "ROLE ARN: arn:aws:iam::123456789012:role/MyRole")
	require.Contains(t, got, "Resource Stats by Type")
	require.Contains(t, got, "EC2")
	require.Contains(t, got, "RDS")
	require.Contains(t, got, "Discovery Configs")
	require.Contains(t, got, "dc-main")
	require.Contains(t, got, "Open Tasks")
	require.Contains(t, got, "e785789e-4fbc-5774-a3d3-4e34edc80dbd")
	require.Contains(t, got, "ec2-ssm-script-failure")
	require.Contains(t, got, "# Inspect a task")
	require.Contains(t, got, "tctl discovery tasks show e785789e")
	require.Contains(t, got, "# List tasks for this integration")
	require.Contains(t, got, "tctl discovery tasks ls --integration=my-aws-integration")
	require.Contains(t, got, "tctl discovery tasks ls --integration=my-aws-integration --state=resolved")
	require.Contains(t, got, "# Check SSM runs")
	require.Contains(t, got, "# Return to integration list")
	require.Contains(t, got, "tctl discovery integration ls")
	require.Contains(t, got, "# Check discovery status")
	require.Contains(t, got, "tctl discovery status")
	require.Contains(t, got, "# Use machine-readable output")
	require.Contains(t, got, "tctl discovery integration show my-aws-integration --format=json")
}

func TestDiscoveryParseInstanceJoinEvents(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	eventList := []apievents.AuditEvent{
		&apievents.InstanceJoin{
			Metadata: apievents.Metadata{Time: now.Add(-10 * time.Minute), Code: libevents.InstanceJoinCode},
			Status:   apievents.Status{Success: true},
			HostID:   "host-1",
			NodeName: "node-alpha",
			Role:     "Node",
			Method:   "ec2",
			ConnectionMetadata: apievents.ConnectionMetadata{RemoteAddr: "10.0.0.1:1234"},
		},
		&apievents.InstanceJoin{
			Metadata: apievents.Metadata{Time: now.Add(-5 * time.Minute), Code: libevents.InstanceJoinFailureCode},
			Status:   apievents.Status{Success: false, Error: "token expired"},
			HostID:   "host-2",
			NodeName: "node-beta",
			Role:     "Node",
			Method:   "token",
			ConnectionMetadata: apievents.ConnectionMetadata{RemoteAddr: "10.0.0.2:1234"},
		},
		&apievents.InstanceJoin{
			Metadata: apievents.Metadata{Time: now.Add(-2 * time.Minute), Code: libevents.InstanceJoinCode},
			Status:   apievents.Status{Success: true},
			HostID:   "host-1",
			NodeName: "node-alpha",
			Role:     "Node",
			Method:   "ec2",
			ConnectionMetadata: apievents.ConnectionMetadata{RemoteAddr: "10.0.0.1:1234"},
		},
	}

	t.Run("parse all", func(t *testing.T) {
		records := parseInstanceJoinEvents(eventList, joinEventFilters{})
		require.Len(t, records, 3)
		// Sorted desc by time.
		require.Equal(t, "host-1", records[0].HostID)
		require.Equal(t, "host-2", records[1].HostID)
		require.Equal(t, "host-1", records[2].HostID)
	})

	t.Run("filter by host", func(t *testing.T) {
		records := parseInstanceJoinEvents(eventList, joinEventFilters{HostID: "host-2"})
		require.Len(t, records, 1)
		require.Equal(t, "host-2", records[0].HostID)
		require.False(t, records[0].Success)
	})

	t.Run("filter unknown matches empty host ID", func(t *testing.T) {
		eventsWithEmpty := append(eventList, &apievents.InstanceJoin{
			Metadata: apievents.Metadata{Time: now.Add(-1 * time.Minute), Code: libevents.InstanceJoinFailureCode},
			Status:   apievents.Status{Success: false, Error: "no token"},
			HostID:   "",
			NodeName: "node-gamma",
			Role:     "Node",
			Method:   "token",
		})
		// Filter using the group key "unknown".
		records := parseInstanceJoinEvents(eventsWithEmpty, joinEventFilters{HostID: "unknown"})
		require.Len(t, records, 1)
		require.Equal(t, "", records[0].HostID)
		require.Equal(t, "node-gamma", records[0].NodeName)
	})

	t.Run("filter unknown without IP matches empty host and addr", func(t *testing.T) {
		eventsWithNoAddr := append(eventList, &apievents.InstanceJoin{
			Metadata: apievents.Metadata{Time: now.Add(-1 * time.Minute), Code: libevents.InstanceJoinFailureCode},
			Status:   apievents.Status{Success: false, Error: "no token"},
			HostID:   "",
			NodeName: "node-delta",
			Role:     "Node",
			Method:   "token",
		})
		records := parseInstanceJoinEvents(eventsWithNoAddr, joinEventFilters{HostID: "unknown"})
		require.Len(t, records, 1)
		require.Equal(t, "", records[0].HostID)
		require.Equal(t, "node-delta", records[0].NodeName)
	})
}

func TestDiscoveryGroupJoinsByHost(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	records := []joinRecord{
		{HostID: "host-1", NodeName: "node-alpha", Code: libevents.InstanceJoinCode, Success: true, parsedEventTime: now.Add(-2 * time.Minute)},
		{HostID: "host-2", NodeName: "node-beta", Code: libevents.InstanceJoinFailureCode, Success: false, Error: "token expired", parsedEventTime: now.Add(-5 * time.Minute)},
		{HostID: "host-1", NodeName: "node-alpha", Code: libevents.InstanceJoinCode, Success: true, parsedEventTime: now.Add(-10 * time.Minute)},
	}

	groups := groupJoinsByHost(records)
	require.Len(t, groups, 2)

	// host-1 is more recent (2m ago vs 5m ago).
	require.Equal(t, "host-1", groups[0].HostID)
	require.Equal(t, "node-alpha", groups[0].NodeName)
	require.Equal(t, 2, groups[0].TotalJoins)
	require.Equal(t, 0, groups[0].FailedJoins)
	require.False(t, groups[0].MostRecentFailed)

	require.Equal(t, "host-2", groups[1].HostID)
	require.Equal(t, 1, groups[1].TotalJoins)
	require.Equal(t, 1, groups[1].FailedJoins)
	require.True(t, groups[1].MostRecentFailed)

	// Unknown hosts (empty HostID) all group under "unknown".
	recordsWithUnknown := []joinRecord{
		{HostID: "", Code: libevents.InstanceJoinFailureCode, Success: false, parsedEventTime: now.Add(-1 * time.Minute)},
		{HostID: "", Code: libevents.InstanceJoinFailureCode, Success: false, parsedEventTime: now.Add(-2 * time.Minute)},
		{HostID: "", Code: libevents.InstanceJoinFailureCode, Success: false, parsedEventTime: now.Add(-3 * time.Minute)},
	}
	unknownGroups := groupJoinsByHost(recordsWithUnknown)
	require.Len(t, unknownGroups, 1)
	require.Equal(t, "unknown", unknownGroups[0].HostID)
	require.Equal(t, 3, unknownGroups[0].TotalJoins)
}

func TestDiscoverySelectFailingJoinGroups(t *testing.T) {
	t.Parallel()

	groups := []joinGroup{
		{HostID: "host-1", MostRecentFailed: false},
		{HostID: "host-2", MostRecentFailed: true},
		{HostID: "host-3", MostRecentFailed: true},
	}

	failing := selectFailingJoinGroups(groups, 0)
	require.Len(t, failing, 2)
	require.Equal(t, "host-2", failing[0].HostID)
	require.Equal(t, "host-3", failing[1].HostID)

	limited := selectFailingJoinGroups(groups, 1)
	require.Len(t, limited, 1)
}

func TestDiscoveryJoinHistoryRows(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	group := joinGroup{
		HostID: "host-1",
		Joins: []joinRecord{
			{HostID: "host-1", Code: libevents.InstanceJoinCode, Success: true, Method: "ec2", Role: "Node", parsedEventTime: now.Add(-2 * time.Minute)},
			{HostID: "host-1", Code: libevents.InstanceJoinCode, Success: true, Method: "ec2", Role: "Node", parsedEventTime: now.Add(-10 * time.Minute)},
		},
	}

	rowsDefault := buildJoinHistoryRows(group, false)
	require.Len(t, rowsDefault, 1)
	require.Equal(t, "success", rowsDefault[0].Result)
	require.Equal(t, "ec2", rowsDefault[0].Method)

	rowsAll := buildJoinHistoryRows(group, true)
	require.Len(t, rowsAll, 2)
}

func TestDiscoveryRenderJoinsText(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	output := joinsOutput{
		Window:         "last 1h",
		TotalJoins:     5,
		SuccessJoins:   3,
		FailedJoins:    2,
		TotalHosts:     3,
		FailingHosts:   1,
		HostPage:       pageInfo{Start: 0, End: 1, Total: 1},
		Hosts: []joinGroup{
			{
				HostID:           "host-fail",
				NodeName:         "node-fail",
				MostRecent:       joinRecord{Code: libevents.InstanceJoinFailureCode, Success: false, Error: "token expired", Method: "token", Role: "Node", parsedEventTime: now.Add(-5 * time.Minute)},
				MostRecentFailed: true,
				TotalJoins:       2,
				FailedJoins:      2,
				SuccessJoins:     0,
				Joins: []joinRecord{
					{Code: libevents.InstanceJoinFailureCode, Success: false, Error: "token expired", Method: "token", Role: "Node", parsedEventTime: now.Add(-5 * time.Minute)},
				},
			},
		},
	}

	var out bytes.Buffer
	err := renderJoinsText(&out, output, "", false, "tctl discovery joins ls")
	require.NoError(t, err)

	got := out.String()
	require.Contains(t, got, "last 1h")
	require.Contains(t, got, "5 total (2 failed, 3 success)")
	require.Contains(t, got, "Hosts (sorted by most recent join)")
	require.Contains(t, got, "host-fail")
	require.Contains(t, got, "NODE NAME")
	require.Contains(t, got, "node-fail")
	require.Contains(t, got, "token expired")
	require.Contains(t, got, "TOKEN")
	// History section is hidden in ls view without --show-all-joins.
	require.NotContains(t, got, "Join history:")
	require.Contains(t, got, "# Adjust joins time window")
	require.Contains(t, got, "# Hide hosts with unknown/empty host ID")
	require.Contains(t, got, "--hide-unknown")
	require.Contains(t, got, "# Check SSM runs")
	require.Contains(t, got, "# List integrations")
	require.Contains(t, got, "# Use machine-readable output")
	require.NotContains(t, got, "limit:")
	require.NotContains(t, got, "Fetch more events")

	// With limit reached.
	output.FetchLimit = 200
	output.LimitReached = true
	var outLimited bytes.Buffer
	err = renderJoinsText(&outLimited, output, "", false, "tctl discovery joins ls")
	require.NoError(t, err)
	gotLimited := outLimited.String()
	require.Contains(t, gotLimited, "[limit: 200, use --limit to increase]")
	require.Contains(t, gotLimited, "# Fetch more events to cover full search window")
	require.Contains(t, gotLimited, "--limit=1000")
}

func TestDiscoveryRenderJoinsTextSingleHost(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	output := joinsOutput{
		Window:         "last 1h",
		TotalJoins:     2,
		SuccessJoins:   1,
		FailedJoins:    1,
		TotalHosts:     1,
		FailingHosts:   1,
		HostPage:       pageInfo{Start: 0, End: 1, Total: 1},
		Hosts: []joinGroup{
			{
				HostID:           "host-1",
				NodeName:         "node-alpha",
				MostRecent:       joinRecord{Code: libevents.InstanceJoinCode, Success: true, Method: "ec2", Role: "Node", parsedEventTime: now.Add(-2 * time.Minute)},
				MostRecentFailed: false,
				TotalJoins:       2,
				FailedJoins:      1,
				SuccessJoins:     1,
				Joins: []joinRecord{
					{Code: libevents.InstanceJoinCode, Success: true, Method: "ec2", Role: "Node", parsedEventTime: now.Add(-2 * time.Minute)},
					{Code: libevents.InstanceJoinFailureCode, Success: false, Error: "token expired", Method: "ec2", Role: "Node", parsedEventTime: now.Add(-10 * time.Minute)},
				},
			},
		},
	}

	var out bytes.Buffer
	err := renderJoinsText(&out, output, "host-1", false, "tctl discovery joins show host-1")
	require.NoError(t, err)

	got := out.String()
	require.Contains(t, got, "Filtered host")
	require.Contains(t, got, "host-1")
	require.Contains(t, got, "Host:")
	require.Contains(t, got, "# Return to joins overview")
	require.Contains(t, got, "# Check SSM runs")
	require.Contains(t, got, "tctl discovery joins show host-1 --format=json")
}

func TestDeriveInventoryState(t *testing.T) {
	t.Parallel()

	successJoin := joinRecord{Success: true, Code: "TJ002I"}
	failedJoin := joinRecord{Success: false, Code: "TJ002E"}
	successSSM := ssmRunRecord{Status: "Success", Code: "TDSSMR0I"}
	failedSSM := ssmRunRecord{Status: "Failed", Code: "TDSSMR0E"}

	tests := []struct {
		name              string
		isOnline          bool
		hasSuccessfulJoin bool
		joinRecs          []joinRecord
		ssmRuns           []ssmRunRecord
		expected          inventoryHostState
	}{
		{name: "online node", isOnline: true, expected: inventoryStateOnline},
		{name: "online with events", isOnline: true, joinRecs: []joinRecord{successJoin}, hasSuccessfulJoin: true, expected: inventoryStateOnline},
		{name: "offline after successful join", hasSuccessfulJoin: true, joinRecs: []joinRecord{successJoin}, expected: inventoryStateOffline},
		{name: "join failed", joinRecs: []joinRecord{failedJoin}, expected: inventoryStateJoinFailed},
		{name: "ssm failed no joins", ssmRuns: []ssmRunRecord{failedSSM}, expected: inventoryStateSSMFailed},
		{name: "ssm success no joins", ssmRuns: []ssmRunRecord{successSSM}, expected: inventoryStateSSMAttempted},
		{name: "joined only no events", isOnline: true, expected: inventoryStateOnline},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := deriveInventoryState(tt.isOnline, tt.hasSuccessfulJoin, tt.joinRecs, tt.ssmRuns)
			require.Equal(t, tt.expected, got)
		})
	}
}

func TestBuildInventoryHosts(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()

	// Mock node (online)
	onlineNode, err := types.NewServer("i-online", types.KindNode, types.ServerSpecV2{
		Hostname: "ip-10-0-1-1",
	})
	require.NoError(t, err)
	onlineNode.SetExpiry(now.Add(10 * time.Minute))

	// SSM records
	ssmRecords := []ssmRunRecord{
		{InstanceID: "i-ssm-only", Status: "Success", Code: "TDSSMR0I", parsedEventTime: now.Add(-5 * time.Minute)},
		{InstanceID: "i-online", Status: "Success", Code: "TDSSMR0I", parsedEventTime: now.Add(-10 * time.Minute)},
	}

	// Join records
	joinRecords := []joinRecord{
		{HostID: "i-online", Success: true, Code: "TJ002I", Method: "ec2", NodeName: "ip-10-0-1-1", parsedEventTime: now.Add(-3 * time.Minute)},
		{HostID: "i-join-failed", Success: false, Code: "TJ002E", Method: "iam", parsedEventTime: now.Add(-7 * time.Minute)},
	}

	hosts := buildInventoryHosts([]types.Server{onlineNode}, ssmRecords, joinRecords)
	require.Len(t, hosts, 3)

	// Find by host ID
	byID := make(map[string]inventoryHost)
	for _, h := range hosts {
		byID[h.HostID] = h
	}

	// Online node with SSM + join
	online := byID["i-online"]
	require.Equal(t, inventoryStateOnline, online.State)
	require.Equal(t, "ip-10-0-1-1", online.NodeName)
	require.Equal(t, "ec2", online.Method)
	require.True(t, online.IsOnline)
	require.Equal(t, 1, online.SSMRuns)
	require.Equal(t, 1, online.Joins)

	// SSM only (no join, no node)
	ssmOnly := byID["i-ssm-only"]
	require.Equal(t, inventoryStateSSMAttempted, ssmOnly.State)
	require.False(t, ssmOnly.IsOnline)
	require.Equal(t, 1, ssmOnly.SSMRuns)
	require.Equal(t, 0, ssmOnly.Joins)

	// Join failed (no node, no SSM)
	joinFailed := byID["i-join-failed"]
	require.Equal(t, inventoryStateJoinFailed, joinFailed.State)
	require.Equal(t, "iam", joinFailed.Method)
	require.Equal(t, 0, joinFailed.SSMRuns)
	require.Equal(t, 1, joinFailed.Joins)
}

func TestRenderInventoryTextList(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	output := inventoryOutput{
		Window:         "last 24h",
		TotalHosts:     3,
		OnlineHosts:    1,
		OfflineHosts:   1,
		FailedHosts:    1,
		HostPage:       pageInfo{Start: 0, End: 3, Total: 3},
		Hosts: []inventoryHost{
			{
				DisplayID: "i-online", HostID: "i-online", NodeName: "ip-10-0-1-1", State: inventoryStateOnline,
				Method: "ec2", IsOnline: true,
				SSMRuns: 2, SSMSuccess: 2, SSMFailed: 0,
				Joins: 1, JoinSuccess: 1, JoinFailed: 0,
			},
			{
				DisplayID: "i-offline", HostID: "i-offline", NodeName: "ip-10-0-1-2", State: inventoryStateOffline,
				Method: "ec2",
				Joins: 1, JoinSuccess: 1, JoinFailed: 0,
			},
			{
				DisplayID: "i-failed", HostID: "i-failed", State: inventoryStateJoinFailed,
				Method: "iam",
				Joins: 2, JoinSuccess: 0, JoinFailed: 2,
			},
		},
	}

	err := renderInventoryText(&buf, output, "", false, "tctl discovery inventory ls")
	require.NoError(t, err)

	text := buf.String()
	require.Contains(t, text, "i-online")
	require.Contains(t, text, "Online")
	require.Contains(t, text, "i-offline")
	require.Contains(t, text, "Offline")
	require.Contains(t, text, "i-failed")
	require.Contains(t, text, "Join Failed")
	require.Contains(t, text, "Hosts (sorted by most recent activity):")
	require.Contains(t, text, "[1] HOST        : i-online")
	require.Contains(t, text, "2 (2 success, 0 failed)") // SSM runs
	require.Contains(t, text, "2 (0 success, 2 failed)") // joins for failed host
}

func TestRenderInventoryTextShow(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	var buf bytes.Buffer
	output := inventoryOutput{
		Window:         "last 24h",
		TotalHosts:     1,
		OnlineHosts:    1,
		HostPage:       pageInfo{Start: 0, End: 1, Total: 1},
		Hosts: []inventoryHost{
			{
				DisplayID: "i-online", HostID: "i-online", NodeName: "ip-10-0-1-1", State: inventoryStateOnline,
				Method: "ec2", IsOnline: true,
				LastSSMRun: now.Add(-5 * time.Minute), LastJoin: now.Add(-3 * time.Minute),
				LastSeen: now.Add(-30 * time.Second),
				SSMRuns: 2, SSMSuccess: 1, SSMFailed: 1,
				Joins: 1, JoinSuccess: 1, JoinFailed: 0,
				JoinRecords: []joinRecord{
					{HostID: "i-online", Success: true, Code: "TJ002I", Method: "ec2", Role: "Node", parsedEventTime: now.Add(-3 * time.Minute)},
				},
				SSMRecords: []ssmRunRecord{
					{InstanceID: "i-online", Status: "Success", Code: "TDSSMR0I", ExitCode: "0", parsedEventTime: now.Add(-5 * time.Minute)},
					{InstanceID: "i-online", Status: "Failed", Code: "TDSSMR0E", ExitCode: "1", Stderr: "timeout", parsedEventTime: now.Add(-10 * time.Minute)},
				},
			},
		},
	}

	err := renderInventoryText(&buf, output, "i-online", true, "tctl discovery inventory show i-online")
	require.NoError(t, err)

	text := buf.String()
	require.Contains(t, text, "HOST        : i-online")
	require.Contains(t, text, "Timeline:")
	require.Contains(t, text, "JOIN")
	require.Contains(t, text, "SSM RUN")
	require.Contains(t, text, "Success")
	require.Contains(t, text, "Failed")
	require.Contains(t, text, "timeout")
}

func TestFilterInventoryHosts(t *testing.T) {
	t.Parallel()

	hosts := []inventoryHost{
		{HostID: "a", State: inventoryStateOnline, Method: "ec2"},
		{HostID: "b", State: inventoryStateJoinFailed, Method: "iam"},
		{HostID: "c", State: inventoryStateSSMFailed, Method: "ec2"},
		{HostID: "d", State: inventoryStateOffline, Method: "azure"},
	}

	// Filter by state
	online := filterInventoryHosts(hosts, "online", "")
	require.Len(t, online, 1)
	require.Equal(t, "a", online[0].HostID)

	failed := filterInventoryHosts(hosts, "failed", "")
	require.Len(t, failed, 2)

	// Filter by method
	ec2 := filterInventoryHosts(hosts, "", "ec2")
	require.Len(t, ec2, 2)

	// Combined filter
	failedEC2 := filterInventoryHosts(hosts, "failed", "ec2")
	require.Len(t, failedEC2, 1)
	require.Equal(t, "c", failedEC2[0].HostID)

	// No filter
	all := filterInventoryHosts(hosts, "", "")
	require.Len(t, all, 4)
}

func TestDiscoveryJoinsUsesLsAndShowSubcommands(t *testing.T) {
	t.Parallel()

	cfg := servicecfg.MakeDefaultConfig()
	cfg.CircuitBreakerConfig = breaker.NoopBreakerConfig()

	var cmd Command
	app := utils.InitCLIParser("tctl", "tctl test")
	cmd.Initialize(app, &tctlcfg.GlobalCLIFlags{}, cfg)

	selected, err := app.Parse([]string{"discovery", "joins", "ls"})
	require.NoError(t, err)
	require.Equal(t, cmd.joinsListCmd.FullCommand(), selected)

	selected, err = app.Parse([]string{"discovery", "joins", "show", "host-123"})
	require.NoError(t, err)
	require.Equal(t, cmd.joinsShowCmd.FullCommand(), selected)
	require.Equal(t, "host-123", cmd.joinsShowHostID)

	// Alias: "join" instead of "joins".
	selected, err = app.Parse([]string{"discovery", "join", "ls"})
	require.NoError(t, err)
	require.Equal(t, cmd.joinsListCmd.FullCommand(), selected)
}
