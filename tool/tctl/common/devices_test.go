// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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

package common

import (
	"bytes"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/gravitational/teleport"
	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/api/types"
)

func TestDeviceSourceToString(t *testing.T) {
	tests := []struct {
		name   string
		source *devicepb.DeviceSource
		want   string
	}{
		{
			name:   "default name for origin",
			source: devicepb.DeviceSource_builder{Origin: devicepb.DeviceOrigin_DEVICE_ORIGIN_INTUNE, Name: "intune"}.Build(),
			want:   "Intune",
		},
		{
			name:   "custom name",
			source: devicepb.DeviceSource_builder{Origin: devicepb.DeviceOrigin_DEVICE_ORIGIN_JAMF, Name: "cool jamf"}.Build(),
			want:   "cool jamf",
		},
		{
			name:   "no source",
			source: nil,
			want:   "",
		},
		{
			name:   "unsupported origin",
			source: devicepb.DeviceSource_builder{Origin: 1337, Name: "even cooler jamf"}.Build(),
			// Show the name instead of something like "unknown" as name is required and likely more
			// informative than displaying "unknown".
			want: "even cooler jamf",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, deviceSourceToString(tt.source))
		})
	}
}

// TestWriteEnrollToken covers the structured output of `tctl devices enroll`.
// Device enrollment itself is an Enterprise feature, so we exercise the output
// rendering directly with a synthetic token.
func TestWriteEnrollToken(t *testing.T) {
	t.Parallel()
	token := devicepb.DeviceEnrollToken_builder{
		Token:      "sometoken",
		ExpireTime: timestamppb.New(time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)),
	}.Build()

	t.Run("text", func(t *testing.T) {
		var buf bytes.Buffer
		require.NoError(t, writeEnrollToken(teleport.Text, "dev-123", "display-name", "asset-1", token, &buf))
		require.Contains(t, buf.String(), "sometoken")
		require.Contains(t, buf.String(), "tsh device enroll")
	})

	t.Run("json", func(t *testing.T) {
		var buf bytes.Buffer
		require.NoError(t, writeEnrollToken(teleport.JSON, "dev-123", "display-name", "asset-1", token, &buf))
		got := mustDecodeJSON[deviceEnrollTokenOutput](t, &buf)
		require.Equal(t, "sometoken", got.Token)
		require.Equal(t, "asset-1", got.AssetTag)
		require.Equal(t, "dev-123", got.DeviceID)
		require.NotNil(t, got.Expires)
		require.NotContains(t, buf.String(), "tsh device enroll")
	})

	t.Run("yaml", func(t *testing.T) {
		var buf bytes.Buffer
		require.NoError(t, writeEnrollToken(teleport.YAML, "dev-123", "display-name", "asset-1", token, &buf))
		got := mustDecodeJSON[deviceEnrollTokenOutput](t, bytes.NewReader(mustTranscodeYAMLToJSON(t, &buf)))
		require.Equal(t, "sometoken", got.Token)
		require.Equal(t, "asset-1", got.AssetTag)
		require.Equal(t, "dev-123", got.DeviceID)
		require.NotContains(t, buf.String(), "tsh device enroll")
	})

	t.Run("invalid format", func(t *testing.T) {
		err := writeEnrollToken("bogus", "dev-123", "display-name", "asset-1", token, &bytes.Buffer{})
		require.True(t, trace.IsBadParameter(err), "expected BadParameter, got %v", err)
	})
}

// TestWriteCreatedLock covers the structured output shared by `tctl lock` and
// `tctl devices lock`.
func TestWriteCreatedLock(t *testing.T) {
	t.Parallel()
	lock, err := types.NewLock("test-lock", types.LockSpecV2{
		Target:  types.LockTarget{User: "bad@actor"},
		Message: "Come see me",
	})
	require.NoError(t, err)

	t.Run("text", func(t *testing.T) {
		var buf bytes.Buffer
		require.NoError(t, writeCreatedLock(teleport.Text, lock, &buf))
		require.Equal(t, "Created a lock with name \"test-lock\".\n", buf.String())
	})

	t.Run("json", func(t *testing.T) {
		var buf bytes.Buffer
		require.NoError(t, writeCreatedLock(teleport.JSON, lock, &buf))
		got := mustDecodeJSON[*types.LockV2](t, &buf)
		require.Equal(t, "test-lock", got.GetName())
		require.Equal(t, "bad@actor", got.Spec.Target.User)
	})

	t.Run("yaml", func(t *testing.T) {
		var buf bytes.Buffer
		require.NoError(t, writeCreatedLock(teleport.YAML, lock, &buf))
		got := mustDecodeJSON[*types.LockV2](t, bytes.NewReader(mustTranscodeYAMLToJSON(t, &buf)))
		require.Equal(t, "test-lock", got.GetName())
		require.Equal(t, "bad@actor", got.Spec.Target.User)
	})

	t.Run("invalid format", func(t *testing.T) {
		err := writeCreatedLock("bogus", lock, &bytes.Buffer{})
		require.True(t, trace.IsBadParameter(err), "expected BadParameter, got %v", err)
	})
}
