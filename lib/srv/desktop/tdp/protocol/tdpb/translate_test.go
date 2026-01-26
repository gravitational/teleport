package tdpb

import (
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/gravitational/teleport/lib/srv/desktop/tdp"
	"github.com/gravitational/teleport/lib/srv/desktop/tdp/protocol/legacy"
	"github.com/stretchr/testify/require"
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
			State:  legacy.ButtonNotPressed,
		},
		legacy.MouseButton{
			Button: legacy.RightMouseButton,
			State:  legacy.ButtonNotPressed,
		},
		legacy.MouseButton{
			Button: legacy.MiddleMouseButton,
			State:  legacy.ButtonPressed,
		},
		legacy.SyncKeys{
			ScrollLockState: legacy.ButtonNotPressed,
			NumLockState:    legacy.ButtonNotPressed,
			CapsLockState:   legacy.ButtonNotPressed,
			KanaLockState:   legacy.ButtonNotPressed,
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
			State:   legacy.ButtonNotPressed,
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

	// Let's play a game of telephone. Translate each message
	// to TDPB and back. We should get the exact same input message.
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
