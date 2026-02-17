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

	"github.com/stretchr/testify/require"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	usertasksv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/usertasks/v1"
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
