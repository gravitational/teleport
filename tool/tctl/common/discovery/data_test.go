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
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	usertasksv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/usertasks/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/discoveryconfig"
	usertasksapi "github.com/gravitational/teleport/api/types/usertasks"
	libevents "github.com/gravitational/teleport/lib/events"
)

func TestShortUUIDPrefix(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		input  string
		want   string
		wantOK bool
	}{
		{name: "valid UUID", input: "a1b2c3d4-e5f6-7890-abcd-ef1234567890", want: "a1b2c3d4", wantOK: true},
		{name: "uppercase UUID", input: "A1B2C3D4-E5F6-7890-ABCD-EF1234567890", want: "A1B2C3D4", wantOK: true},
		{name: "not a UUID", input: "hello-world", want: "", wantOK: false},
		{name: "wrong part lengths", input: "abc-defg-hijk-lmno-pqrstuvwxyz", want: "", wantOK: false},
		{name: "non-hex chars", input: "g1b2c3d4-e5f6-7890-abcd-ef1234567890", want: "", wantOK: false},
		{name: "empty string", input: "", want: "", wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := shortUUIDPrefix(tt.input)
			require.Equal(t, tt.wantOK, ok)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestFindTaskByNamePrefix(t *testing.T) {
	t.Parallel()

	makeTask := func(name string) *usertasksv1.UserTask {
		return &usertasksv1.UserTask{
			Metadata: &headerv1.Metadata{Name: name},
		}
	}

	tasks := []*usertasksv1.UserTask{
		makeTask("abc-123"),
		makeTask("abc-456"),
		makeTask("def-789"),
	}

	t.Run("exact match", func(t *testing.T) {
		result, err := findTaskByNamePrefix(tasks, "abc-123")
		require.NoError(t, err)
		require.Equal(t, "abc-123", result.GetMetadata().GetName())
	})

	t.Run("unique prefix", func(t *testing.T) {
		result, err := findTaskByNamePrefix(tasks, "def")
		require.NoError(t, err)
		require.Equal(t, "def-789", result.GetMetadata().GetName())
	})

	t.Run("ambiguous prefix", func(t *testing.T) {
		_, err := findTaskByNamePrefix(tasks, "abc")
		require.Error(t, err)
		require.Contains(t, err.Error(), "ambiguous")
	})

	t.Run("no match", func(t *testing.T) {
		_, err := findTaskByNamePrefix(tasks, "zzz")
		require.Error(t, err)
		require.Contains(t, err.Error(), "not found")
	})

	t.Run("empty input", func(t *testing.T) {
		_, err := findTaskByNamePrefix(tasks, "")
		require.Error(t, err)
	})

	t.Run("strips ellipsis suffix", func(t *testing.T) {
		result, err := findTaskByNamePrefix(tasks, "def...")
		require.NoError(t, err)
		require.Equal(t, "def-789", result.GetMetadata().GetName())
	})
}

func TestParseAuditEventTime(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		input  string
		wantOK bool
		wantTS string // expected UTC RFC3339
	}{
		{name: "RFC3339", input: "2026-02-12T10:30:00Z", wantOK: true, wantTS: "2026-02-12T10:30:00Z"},
		{name: "RFC3339Nano", input: "2026-02-12T10:30:00.123456789Z", wantOK: true, wantTS: "2026-02-12T10:30:00.123456789Z"},
		{name: "space separated", input: "2026-02-12 10:30:00", wantOK: true, wantTS: "2026-02-12T10:30:00Z"},
		{name: "space separated millis", input: "2026-02-12 10:30:00.123", wantOK: true, wantTS: "2026-02-12T10:30:00.123Z"},
		{name: "empty", input: "", wantOK: false},
		{name: "garbage", input: "not a time", wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed, ok := parseAuditEventTime(tt.input)
			require.Equal(t, tt.wantOK, ok)
			if ok {
				require.Equal(t, tt.wantTS, parsed.Format("2006-01-02T15:04:05.999999999Z"))
			}
		})
	}
}

func TestExtractEC2InstanceID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "full ARN", input: "arn:aws:sts::123456:assumed-role/role-name/i-030a87f439b67b43a", want: "i-030a87f439b67b43a"},
		{name: "no instance prefix", input: "arn:aws:sts::123456:assumed-role/role-name/not-an-instance", want: ""},
		{name: "no slash", input: "no-slash-here", want: ""},
		{name: "trailing slash", input: "arn:aws:sts::123456/", want: ""},
		{name: "empty", input: "", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, extractEC2InstanceID(tt.input))
		})
	}
}

func TestSanitizeTokenName(t *testing.T) {
	t.Parallel()

	require.Equal(t, "********", sanitizeTokenName("my-secret-token", "token"))
	require.Equal(t, "********", sanitizeTokenName("my-secret-token", "Token"))
	require.Equal(t, "my-token", sanitizeTokenName("my-token", "ec2"))
	require.Equal(t, "my-token", sanitizeTokenName("my-token", "iam"))
	require.Equal(t, "", sanitizeTokenName("", "token"))
}

func TestIsSSMRunFailure(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		record ssmRunRecord
		want   bool
	}{
		{name: "TDS00W code", record: ssmRunRecord{Code: "TDS00W"}, want: true},
		{name: "TDS00W case insensitive", record: ssmRunRecord{Code: "tds00w"}, want: true},
		{name: "TDS00I success", record: ssmRunRecord{Code: "TDS00I", Status: "Success"}, want: false},
		{name: "failed status", record: ssmRunRecord{Code: "TDS00I", Status: "Failed"}, want: true},
		{name: "empty status non-warning", record: ssmRunRecord{Code: "TDS00I"}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, isSSMRunFailure(tt.record))
		})
	}
}

func TestIsJoinFailure(t *testing.T) {
	t.Parallel()

	require.True(t, isJoinFailure(joinRecord{Code: libevents.InstanceJoinFailureCode}))
	require.True(t, isJoinFailure(joinRecord{Code: "TJ002I", Success: false}))
	require.False(t, isJoinFailure(joinRecord{Code: "TJ001I", Success: true}))
}

func TestJoinGroupKey(t *testing.T) {
	t.Parallel()

	require.Equal(t, "host-1", joinGroupKey(joinRecord{HostID: "host-1"}))
	require.Equal(t, "host-2", joinGroupKey(joinRecord{HostID: "  host-2  "}))
	require.Equal(t, "unknown", joinGroupKey(joinRecord{HostID: ""}))
	require.Equal(t, "unknown", joinGroupKey(joinRecord{HostID: "   "}))
}

func TestIsUnknownHost(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		hostID string
		want   bool
	}{
		{name: "unknown literal", hostID: "unknown", want: true},
		{name: "real host ID", hostID: "i-0123456789abcdef0", want: false},
		{name: "empty string", hostID: "", want: false},
		{name: "Unknown capitalized", hostID: "Unknown", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, isUnknownHost(tt.hostID))
		})
	}
}

func TestIsHexString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{name: "valid hex lowercase", input: "abcdef0123456789", want: true},
		{name: "valid hex uppercase", input: "ABCDEF0123456789", want: true},
		{name: "valid hex mixed", input: "aAbBcC123", want: true},
		{name: "invalid chars", input: "xyz123", want: false},
		{name: "empty string", input: "", want: true},
		{name: "spaces", input: "ab cd", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, isHexString(tt.input))
		})
	}
}

func TestTaskNamePrefix(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "UUID gets shortened", input: "a1b2c3d4-e5f6-7890-abcd-ef1234567890", want: "a1b2c3d4"},
		{name: "non-UUID passthrough", input: "my-task-name", want: "my-task-name"},
		{name: "strips ellipsis", input: "my-task...", want: "my-task"},
		{name: "strips unicode ellipsis", input: "my-task\u2026", want: "my-task"},
		{name: "empty string", input: "", want: ""},
		{name: "whitespace only", input: "   ", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, taskNamePrefix(tt.input))
		})
	}
}

func TestFriendlyTaskType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "EC2", input: usertasksapi.TaskTypeDiscoverEC2, want: "AWS EC2"},
		{name: "EKS", input: usertasksapi.TaskTypeDiscoverEKS, want: "AWS EKS"},
		{name: "RDS", input: usertasksapi.TaskTypeDiscoverRDS, want: "AWS RDS"},
		{name: "unknown type", input: "some-other-type", want: "some-other-type"},
		{name: "empty", input: "", want: "Unknown"},
		{name: "whitespace", input: "   ", want: "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, friendlyTaskType(tt.input))
		})
	}
}

func TestFriendlyIntegrationType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "AWS OIDC", input: types.IntegrationSubKindAWSOIDC, want: "AWS OIDC"},
		{name: "Azure OIDC", input: types.IntegrationSubKindAzureOIDC, want: "Azure OIDC"},
		{name: "GitHub", input: types.IntegrationSubKindGitHub, want: "GitHub"},
		{name: "AWS Roles Anywhere", input: types.IntegrationSubKindAWSRolesAnywhere, want: "AWS Roles Anywhere"},
		{name: "unknown subkind", input: "custom-kind", want: "custom-kind"},
		{name: "empty", input: "", want: "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, friendlyIntegrationType(tt.input))
		})
	}
}

func TestAwaitingJoin(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		stats resourcesAggregate
		want  uint64
	}{
		{name: "found exceeds joined+failed", stats: resourcesAggregate{Found: 10, Enrolled: 3, Failed: 2}, want: 5},
		{name: "all enrolled", stats: resourcesAggregate{Found: 5, Enrolled: 5, Failed: 0}, want: 0},
		{name: "joined+failed exceeds found", stats: resourcesAggregate{Found: 3, Enrolled: 3, Failed: 2}, want: 0},
		{name: "all zeros", stats: resourcesAggregate{}, want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, awaitingJoin(tt.stats))
		})
	}
}

func TestDeriveInventoryStateTransitions(t *testing.T) {
	t.Parallel()

	failedSSMRun := ssmRunRecord{Code: "TDS00W"}
	successSSMRun := ssmRunRecord{Code: "TDS00I", Status: "Success"}
	failedJoin := joinRecord{Code: libevents.InstanceJoinFailureCode}
	successJoin := joinRecord{Code: "TJ001I", Success: true}

	tests := []struct {
		name              string
		isOnline          bool
		hasSuccessfulJoin bool
		joinRecs          []joinRecord
		ssmRuns           []ssmRunRecord
		want              inventoryHostState
	}{
		{name: "online", isOnline: true, want: inventoryStateOnline},
		{name: "offline with successful join", hasSuccessfulJoin: true, joinRecs: []joinRecord{successJoin}, want: inventoryStateOffline},
		{name: "join failed", joinRecs: []joinRecord{failedJoin}, want: inventoryStateJoinFailed},
		{name: "SSM failed", ssmRuns: []ssmRunRecord{failedSSMRun}, want: inventoryStateSSMFailed},
		{name: "SSM attempted", ssmRuns: []ssmRunRecord{successSSMRun}, want: inventoryStateSSMAttempted},
		{name: "joined only - no data", want: inventoryStateJoinedOnly},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := deriveInventoryState(tt.isOnline, tt.hasSuccessfulJoin, tt.joinRecs, tt.ssmRuns)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestMaxTime(t *testing.T) {
	t.Parallel()

	t1 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	t3 := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name  string
		times []time.Time
		want  time.Time
	}{
		{name: "no times", times: nil, want: time.Time{}},
		{name: "single time", times: []time.Time{t1}, want: t1},
		{name: "picks latest", times: []time.Time{t1, t2, t3}, want: t2},
		{name: "with zero time", times: []time.Time{{}, t1}, want: t1},
		{name: "all zero", times: []time.Time{{}, {}}, want: time.Time{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, maxTime(tt.times...))
		})
	}
}

func TestSelectFailingVMGroups(t *testing.T) {
	t.Parallel()

	groups := []ssmVMGroup{
		{InstanceID: "i-1", MostRecentFailed: true},
		{InstanceID: "i-2", MostRecentFailed: false},
		{InstanceID: "i-3", MostRecentFailed: true},
		{InstanceID: "i-4", MostRecentFailed: true},
	}

	t.Run("no limit", func(t *testing.T) {
		result := selectFailingVMGroups(groups, 0)
		require.Len(t, result, 3)
	})

	t.Run("with limit", func(t *testing.T) {
		result := selectFailingVMGroups(groups, 2)
		require.Len(t, result, 2)
		require.Equal(t, "i-1", result[0].InstanceID)
		require.Equal(t, "i-3", result[1].InstanceID)
	})

	t.Run("empty input", func(t *testing.T) {
		result := selectFailingVMGroups(nil, 0)
		require.Empty(t, result)
	})
}

func TestFilterOutUnknownJoinGroups(t *testing.T) {
	t.Parallel()

	groups := []joinGroup{
		{HostID: "host-1"},
		{HostID: "unknown"},
		{HostID: "host-2"},
		{HostID: "unknown"},
	}

	t.Run("filters unknown", func(t *testing.T) {
		result := filterOutUnknownJoinGroups(groups)
		require.Len(t, result, 2)
		require.Equal(t, "host-1", result[0].HostID)
		require.Equal(t, "host-2", result[1].HostID)
	})

	t.Run("all unknown", func(t *testing.T) {
		result := filterOutUnknownJoinGroups([]joinGroup{{HostID: "unknown"}})
		require.Empty(t, result)
	})

	t.Run("none unknown", func(t *testing.T) {
		result := filterOutUnknownJoinGroups([]joinGroup{{HostID: "h1"}, {HostID: "h2"}})
		require.Len(t, result, 2)
	})

	t.Run("empty input", func(t *testing.T) {
		result := filterOutUnknownJoinGroups(nil)
		require.Empty(t, result)
	})
}

func TestNormalizeTaskState(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "empty defaults to open", input: "", want: usertasksapi.TaskStateOpen},
		{name: "open lower", input: "open", want: usertasksapi.TaskStateOpen},
		{name: "open upper", input: "OPEN", want: usertasksapi.TaskStateOpen},
		{name: "resolved", input: "RESOLVED", want: usertasksapi.TaskStateResolved},
		{name: "all", input: "ALL", want: ""},
		{name: "invalid", input: "bogus", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeTaskState(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestMatchesStateFilter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		state  inventoryHostState
		filter string
		want   bool
	}{
		{name: "online matches Online", state: inventoryStateOnline, filter: "online", want: true},
		{name: "online matches JoinedOnly", state: inventoryStateJoinedOnly, filter: "online", want: true},
		{name: "online rejects Offline", state: inventoryStateOffline, filter: "online", want: false},
		{name: "offline matches Offline", state: inventoryStateOffline, filter: "offline", want: true},
		{name: "offline rejects Online", state: inventoryStateOnline, filter: "offline", want: false},
		{name: "failed matches JoinFailed", state: inventoryStateJoinFailed, filter: "failed", want: true},
		{name: "failed matches SSMFailed", state: inventoryStateSSMFailed, filter: "failed", want: true},
		{name: "failed rejects Online", state: inventoryStateOnline, filter: "failed", want: false},
		{name: "attempted matches SSMAttempted", state: inventoryStateSSMAttempted, filter: "attempted", want: true},
		{name: "attempted rejects Online", state: inventoryStateOnline, filter: "attempted", want: false},
		{name: "case-insensitive fallback", state: inventoryStateOnline, filter: "Online", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, matchesStateFilter(tt.state, tt.filter))
		})
	}
}

func TestSelectFailingJoinGroups(t *testing.T) {
	t.Parallel()

	groups := []joinGroup{
		{HostID: "h1", MostRecentFailed: true},
		{HostID: "h2", MostRecentFailed: false},
		{HostID: "h3", MostRecentFailed: true},
		{HostID: "h4", MostRecentFailed: true},
	}

	t.Run("no limit", func(t *testing.T) {
		result := selectFailingJoinGroups(groups, 0)
		require.Len(t, result, 3)
	})

	t.Run("with limit", func(t *testing.T) {
		result := selectFailingJoinGroups(groups, 1)
		require.Len(t, result, 1)
		require.Equal(t, "h1", result[0].HostID)
	})

	t.Run("empty input", func(t *testing.T) {
		result := selectFailingJoinGroups(nil, 0)
		require.Empty(t, result)
	})
}

func TestCompareTimeDesc(t *testing.T) {
	t.Parallel()

	t1 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name string
		a    time.Time
		b    time.Time
		want int
	}{
		{name: "both zero", a: time.Time{}, b: time.Time{}, want: 0},
		{name: "a zero b non-zero", a: time.Time{}, b: t1, want: 1},
		{name: "a non-zero b zero", a: t1, b: time.Time{}, want: -1},
		{name: "a newer than b", a: t2, b: t1, want: -1},
		{name: "a older than b", a: t1, b: t2, want: 1},
		{name: "equal times", a: t1, b: t1, want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, compareTimeDesc(tt.a, tt.b))
		})
	}
}

func TestConfigMatchersSummary(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		dc   *discoveryconfig.DiscoveryConfig
		want string
	}{
		{
			name: "no matchers",
			dc:   &discoveryconfig.DiscoveryConfig{},
			want: "none",
		},
		{
			name: "aws only",
			dc: &discoveryconfig.DiscoveryConfig{
				Spec: discoveryconfig.Spec{
					AWS: make([]types.AWSMatcher, 3),
				},
			},
			want: "aws=3",
		},
		{
			name: "multiple matcher types",
			dc: &discoveryconfig.DiscoveryConfig{
				Spec: discoveryconfig.Spec{
					AWS:   make([]types.AWSMatcher, 2),
					Azure: make([]types.AzureMatcher, 1),
					Kube:  make([]types.KubernetesMatcher, 4),
				},
			},
			want: "aws=2 azure=1 kube=4",
		},
		{
			name: "with access graph",
			dc: &discoveryconfig.DiscoveryConfig{
				Spec: discoveryconfig.Spec{
					AccessGraph: &types.AccessGraphSync{
						AWS: make([]*types.AccessGraphAWSSync, 2),
					},
				},
			},
			want: "ag=2",
		},
		{
			name: "nil access graph",
			dc: &discoveryconfig.DiscoveryConfig{
				Spec: discoveryconfig.Spec{
					GCP:         make([]types.GCPMatcher, 1),
					AccessGraph: nil,
				},
			},
			want: "gcp=1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, configMatchersSummary(tt.dc))
		})
	}
}

func TestCountDiscoveryGroups(t *testing.T) {
	t.Parallel()

	makeDC := func(group string) *discoveryconfig.DiscoveryConfig {
		dc := &discoveryconfig.DiscoveryConfig{}
		dc.Spec.DiscoveryGroup = group
		return dc
	}

	tests := []struct {
		name string
		dcs  []*discoveryconfig.DiscoveryConfig
		want int
	}{
		{name: "nil input", dcs: nil, want: 0},
		{name: "empty input", dcs: []*discoveryconfig.DiscoveryConfig{}, want: 0},
		{name: "single group", dcs: []*discoveryconfig.DiscoveryConfig{makeDC("g1"), makeDC("g1")}, want: 1},
		{name: "distinct groups", dcs: []*discoveryconfig.DiscoveryConfig{makeDC("g1"), makeDC("g2"), makeDC("g3")}, want: 3},
		{name: "mixed duplicates", dcs: []*discoveryconfig.DiscoveryConfig{makeDC("a"), makeDC("b"), makeDC("a"), makeDC("c"), makeDC("b")}, want: 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, countDiscoveryGroups(tt.dcs))
		})
	}
}
