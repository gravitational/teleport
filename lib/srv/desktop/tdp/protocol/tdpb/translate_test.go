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

package tdpb

import (
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/srv/desktop/tdp"
	"github.com/gravitational/teleport/lib/srv/desktop/tdp/protocol/legacy"
)

func TestTranslation(t *testing.T) {
	// Tests "core" messages which can be translated in both directions
	// (TDPB -> TDP as well as TDP -> TDPB).
	clipboard := []byte("some data")
	pdu := []byte("some pdu")
	response := []byte("some response")
	cases := []tdp.Message{
		legacy.ClientScreenSpec{
			Width: 1920, Height: 1080,
		},
		legacy.MouseWheel{
			Axis:  legacy.HorizontalWheelAxis,
			Delta: -100,
		},
		legacy.MouseWheel{
			Axis:  legacy.VerticalWheelAxis,
			Delta: 123,
		},
		legacy.MouseButton{
			Button: legacy.LeftMouseButton,
			State:  legacy.ButtonPressed,
		},
		legacy.MouseButton{
			Button: legacy.RightMouseButton,
			State:  legacy.ButtonPressed,
		},
		legacy.MouseButton{
			Button: legacy.MiddleMouseButton,
			State:  legacy.ButtonPressed,
		},
		legacy.SyncKeys{
			ScrollLockState: legacy.ButtonPressed,
			NumLockState:    legacy.ButtonPressed,
			CapsLockState:   legacy.ButtonPressed,
			KanaLockState:   legacy.ButtonPressed,
		},
		legacy.SyncKeys{
			ScrollLockState: legacy.ButtonPressed,
			NumLockState:    legacy.ButtonPressed,
			CapsLockState:   legacy.ButtonPressed,
			KanaLockState:   legacy.ButtonPressed,
		},
		legacy.KeyboardButton{
			KeyCode: 10,
			State:   legacy.ButtonPressed,
		},
		legacy.KeyboardButton{
			KeyCode: 23,
			State:   legacy.ButtonPressed,
		},
		legacy.Alert{Severity: legacy.SeverityError, Message: "error"},
		legacy.Alert{Severity: legacy.SeverityWarning, Message: "warn"},
		legacy.Alert{Severity: legacy.SeverityInfo, Message: "info"},
		legacy.ClipboardData(clipboard),
		legacy.RDPFastPathPDU(pdu),
		legacy.RDPResponsePDU(response),
		legacy.SharedDirectoryCreateRequest{
			CompletionID: 1,
			DirectoryID:  2,
			FileType:     3,
			Path:         "create req",
		},
		legacy.SharedDirectoryInfoRequest{
			CompletionID: 1,
			DirectoryID:  2,
			Path:         "info req",
		},
		legacy.SharedDirectoryDeleteRequest{
			CompletionID: 1,
			DirectoryID:  2,
			Path:         "delete req",
		},
		legacy.SharedDirectoryTruncateRequest{
			CompletionID: 1,
			DirectoryID:  2,
			Path:         "truncate req",
			EndOfFile:    3,
		},
		legacy.SharedDirectoryListRequest{
			CompletionID: 1,
			DirectoryID:  2,
			Path:         "list req",
		},
		legacy.SharedDirectoryWriteRequest{
			CompletionID:    1,
			DirectoryID:     2,
			WriteDataLength: 3,
			Offset:          4,
			WriteData:       []byte("aaa"),
		},
		legacy.SharedDirectoryReadRequest{
			CompletionID: 1,
			DirectoryID:  2,
			Path:         "read req",
			Offset:       3,
			Length:       4,
		},
		legacy.SharedDirectoryInfoResponse{
			CompletionID: 1,
			ErrCode:      2,
			Fso: legacy.FileSystemObject{
				LastModified: 3,
				Size:         4,
				IsEmpty:      1,
				Path:         "create response",
			},
		},
		legacy.SharedDirectoryCreateResponse{
			CompletionID: 1,
			ErrCode:      2,
			Fso: legacy.FileSystemObject{
				LastModified: 3,
				Size:         4,
				IsEmpty:      1,
				Path:         "create response",
			},
		},
		legacy.SharedDirectoryDeleteResponse{
			CompletionID: 1,
			ErrCode:      2,
		},
		legacy.SharedDirectoryListResponse{
			CompletionID: 1,
			ErrCode:      2,
			FsoList: []legacy.FileSystemObject{
				{
					LastModified: 3,
					Size:         2,
					IsEmpty:      1,
					Path:         "fso 1",
				},
				{
					LastModified: 4,
					Size:         5,
					IsEmpty:      0,
					Path:         "fso 2",
				},
			},
		},
		legacy.SharedDirectoryReadResponse{
			CompletionID:   1,
			ErrCode:        2,
			ReadDataLength: 3,
			ReadData:       []byte("aaa"),
		},
		legacy.SharedDirectoryWriteResponse{
			CompletionID: 1,
			ErrCode:      2,
			BytesWritten: 3,
		},
		legacy.SharedDirectoryMoveResponse{
			CompletionID: 1,
			ErrCode:      2,
		},
		legacy.SharedDirectoryTruncateResponse{
			CompletionID: 1,
			ErrCode:      2,
		},
		legacy.SharedDirectoryAcknowledge{
			DirectoryID: 1,
			ErrCode:     2,
		},
		legacy.SharedDirectoryAnnounce{
			DirectoryID: 1,
			Name:        "directory",
		},
		legacy.Ping{
			UUID: uuid.New(),
		},
		legacy.LatencyStats{
			ClientLatency: 1,
			ServerLatency: 2,
		},
	}

	// Translate each message to TDPB and back. Then check
	// the resulting message for equality with the original.
	for _, msg := range cases {
		t.Run(fmt.Sprintf("translate %T", msg), func(t *testing.T) {
			modern, err := TranslateToModern(msg)
			require.NoError(t, err)
			require.Len(t, modern, 1)

			legacyMsgs, err := TranslateToLegacy(modern[0])
			require.NoError(t, err)
			require.Len(t, legacyMsgs, 1)
			require.Equal(t, msg, legacyMsgs[0])
		})
	}
}
