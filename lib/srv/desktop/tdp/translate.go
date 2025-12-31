package tdp

import (
	"bytes"
	"image/png"
	"log/slog"

	"github.com/google/uuid"
	tdpbv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/desktop/v1"
	"github.com/gravitational/teleport/lib/srv/desktop/tdp/protocol"
	tdp "github.com/gravitational/teleport/lib/srv/desktop/tdp/protocol/legacy"
	"github.com/gravitational/teleport/lib/srv/desktop/tdp/protocol/tdpb"
	sliceutils "github.com/gravitational/teleport/lib/utils/slices"
	"github.com/gravitational/trace"
)

func translateFso(fso *tdpbv1.FileSystemObject) tdp.FileSystemObject {
	isEmpty := uint8(0)
	if !fso.IsEmpty {
		isEmpty = 1
	}
	return tdp.FileSystemObject{
		LastModified: fso.LastModified,
		Size:         fso.Size,
		FileType:     fso.FileType,
		IsEmpty:      isEmpty,
		Path:         fso.Path,
	}
}

func translateFsoList(fso []*tdpbv1.FileSystemObject) []tdp.FileSystemObject {
	return sliceutils.Map(fso, translateFso)
}

func translateFsoToModern(fso tdp.FileSystemObject) *tdpbv1.FileSystemObject {
	return &tdpbv1.FileSystemObject{
		LastModified: fso.LastModified,
		Size:         fso.Size,
		FileType:     fso.FileType,
		IsEmpty:      fso.IsEmpty == 1,
		Path:         fso.Path,
	}
}

func translateFsoListToModern(fso []tdp.FileSystemObject) []*tdpbv1.FileSystemObject {
	return sliceutils.Map(fso, translateFsoToModern)
}

func boolToButtonState(b bool) tdp.ButtonState {
	if b {
		return tdp.ButtonPressed
	}
	return tdp.ButtonNotPressed
}

// TranslateToLegacy converts a TDPB (Modern) message to one or more TDP (Legacy) messages.
func TranslateToLegacy(msg protocol.Message) ([]protocol.Message, error) {
	messages := make([]protocol.Message, 0, 1)
	switch m := msg.(type) {
	case *tdpb.PNGFrame:
		messages = append(messages, tdp.PNG2Frame(m.Data))
	case *tdpb.FastPathPDU:
		messages = append(messages, tdp.RDPFastPathPDU(m.Pdu))
	case *tdpb.RDPResponsePDU:
		messages = append(messages, tdp.RDPResponsePDU(m.Response))
	case *tdpb.SyncKeys:
		messages = append(messages, tdp.SyncKeys{
			ScrollLockState: boolToButtonState(m.ScrollLockPressed),
			NumLockState:    boolToButtonState(m.NumLockState),
			CapsLockState:   boolToButtonState(m.CapsLockState),
			KanaLockState:   boolToButtonState(m.KanaLockState),
		})
	case *tdpb.MouseMove:
		messages = append(messages, tdp.MouseMove{X: m.X, Y: m.Y})
	case *tdpb.MouseButton:
		button := tdp.MouseButtonType(m.Button - 1)
		state := tdp.ButtonNotPressed
		if m.Pressed {
			state = tdp.ButtonPressed
		}

		messages = append(messages, tdp.MouseButton{
			Button: button,
			State:  state,
		})
	case *tdpb.KeyboardButton:
		state := tdp.ButtonNotPressed
		if m.Pressed {
			state = tdp.ButtonPressed
		}
		messages = append(messages, tdp.KeyboardButton{
			KeyCode: m.KeyCode,
			State:   state,
		})
	case *tdpb.ClientScreenSpec:
		messages = append(messages, tdp.ClientScreenSpec{
			Width:  m.Width,
			Height: m.Height,
		})
	case *tdpb.Alert:
		var severity tdp.Severity
		switch m.Severity {
		case tdpbv1.AlertSeverity_ALERT_SEVERITY_WARNING:
			severity = tdp.SeverityWarning
		case tdpbv1.AlertSeverity_ALERT_SEVERITY_ERROR:
			severity = tdp.SeverityError
		default:
			severity = tdp.SeverityInfo
		}
		messages = append(messages, tdp.Alert{
			Message:  m.Message,
			Severity: severity,
		})
	case *tdpb.MouseWheel:
		messages = append(messages, tdp.MouseWheel{
			Axis:  tdp.MouseWheelAxis(m.Axis - 1),
			Delta: int16(m.Delta),
		})
	case *tdpb.ClipboardData:
		messages = append(messages, tdp.ClipboardData(m.Data))
	case *tdpb.SharedDirectoryAnnounce:
		messages = append(messages, tdp.SharedDirectoryAnnounce{
			DirectoryID: m.DirectoryId,
			Name:        m.Name,
		})
	case *tdpb.SharedDirectoryAcknowledge:
		messages = append(messages, tdp.SharedDirectoryAcknowledge{
			DirectoryID: m.DirectoryId,
			ErrCode:     m.ErrorCode,
		})
	case *tdpb.SharedDirectoryRequest:
		switch op := m.Operation.(type) {
		case *tdpbv1.SharedDirectoryRequest_Info_:
			messages = append(messages, tdp.SharedDirectoryInfoRequest{
				CompletionID: m.CompletionId,
				DirectoryID:  m.DirectoryId,
				Path:         op.Info.Path,
			})
		case *tdpbv1.SharedDirectoryRequest_Create_:
			messages = append(messages, tdp.SharedDirectoryCreateRequest{
				CompletionID: m.CompletionId,
				DirectoryID:  m.DirectoryId,
				FileType:     op.Create.FileType,
				Path:         op.Create.Path,
			})
		case *tdpbv1.SharedDirectoryRequest_Delete_:
			messages = append(messages, tdp.SharedDirectoryDeleteRequest{
				CompletionID: m.CompletionId,
				DirectoryID:  m.DirectoryId,
				Path:         op.Delete.Path,
			})
		case *tdpbv1.SharedDirectoryRequest_List_:
			messages = append(messages, tdp.SharedDirectoryListRequest{
				CompletionID: m.CompletionId,
				DirectoryID:  m.DirectoryId,
				Path:         op.List.Path,
			})
		case *tdpbv1.SharedDirectoryRequest_Read_:
			messages = append(messages, tdp.SharedDirectoryReadRequest{
				CompletionID: m.CompletionId,
				DirectoryID:  m.DirectoryId,
				Path:         op.Read.Path,
				Offset:       op.Read.Offset,
				Length:       op.Read.Length,
			})
		case *tdpbv1.SharedDirectoryRequest_Write_:
			messages = append(messages, tdp.SharedDirectoryWriteRequest{
				CompletionID:    m.CompletionId,
				DirectoryID:     m.DirectoryId,
				Path:            op.Write.Path,
				Offset:          op.Write.Offset,
				WriteDataLength: uint32(len(op.Write.Data)),
				WriteData:       op.Write.Data,
			})
		case *tdpbv1.SharedDirectoryRequest_Move_:
			messages = append(messages, tdp.SharedDirectoryMoveRequest{
				CompletionID: m.CompletionId,
				DirectoryID:  m.DirectoryId,
				NewPath:      op.Move.NewPath,
				OriginalPath: op.Move.OriginalPath,
			})
		case *tdpbv1.SharedDirectoryRequest_Truncate_:
			messages = append(messages, tdp.SharedDirectoryTruncateRequest{
				CompletionID: m.CompletionId,
				DirectoryID:  m.DirectoryId,
				Path:         op.Truncate.Path,
				EndOfFile:    op.Truncate.EndOfFile,
			})
		default:
			return nil, trace.Errorf("Unknown shared directory operation")
		}
	case *tdpb.SharedDirectoryResponse:
		switch op := m.Operation.(type) {
		case *tdpbv1.SharedDirectoryResponse_Info_:
			messages = append(messages, tdp.SharedDirectoryInfoResponse{
				CompletionID: m.CompletionId,
				ErrCode:      m.ErrorCode,
				Fso:          translateFso(op.Info.Fso),
			})
		case *tdpbv1.SharedDirectoryResponse_Create_:
			messages = append(messages, tdp.SharedDirectoryCreateResponse{
				CompletionID: m.CompletionId,
				ErrCode:      m.ErrorCode,
				Fso:          translateFso(op.Create.Fso),
			})
		case *tdpbv1.SharedDirectoryResponse_Delete_:
			messages = append(messages, tdp.SharedDirectoryDeleteResponse{
				CompletionID: m.CompletionId,
				ErrCode:      m.ErrorCode,
			})
		case *tdpbv1.SharedDirectoryResponse_List_:
			messages = append(messages, tdp.SharedDirectoryListResponse{
				CompletionID: m.CompletionId,
				ErrCode:      m.ErrorCode,
				FsoList:      translateFsoList(op.List.FsoList),
			})
		case *tdpbv1.SharedDirectoryResponse_Read_:
			messages = append(messages, tdp.SharedDirectoryReadResponse{
				CompletionID:   m.CompletionId,
				ErrCode:        m.ErrorCode,
				ReadData:       op.Read.Data,
				ReadDataLength: uint32(len(op.Read.Data)),
			})
		case *tdpbv1.SharedDirectoryResponse_Write_:
			messages = append(messages, tdp.SharedDirectoryWriteResponse{
				CompletionID: m.CompletionId,
				ErrCode:      m.ErrorCode,
				BytesWritten: op.Write.BytesWritten,
			})
		case *tdpbv1.SharedDirectoryResponse_Move_:
			messages = append(messages, tdp.SharedDirectoryMoveResponse{
				CompletionID: m.CompletionId,
				ErrCode:      m.ErrorCode,
			})
		case *tdpbv1.SharedDirectoryResponse_Truncate_:
			messages = append(messages, tdp.SharedDirectoryTruncateResponse{
				CompletionID: m.CompletionId,
				ErrCode:      m.ErrorCode,
			})
		default:
			return nil, trace.Errorf("Unknown shared directory operation")
		}
	case *tdpb.LatencyStats:
		messages = append(messages, tdp.LatencyStats{
			ClientLatency: m.ClientLatencyMs,
			ServerLatency: m.ServerLatencyMs,
		})
	case *tdpb.Ping:
		id, err := uuid.FromBytes(m.Uuid)
		if err != nil {
			slog.Warn("Cannot parse uuid bytes from ping", "error", err)
		} else {
			messages = append(messages, tdp.Ping{UUID: id})
		}
	default:
		return nil, trace.Errorf("Could not translate to TDP. Encountered unexpected message type %T", m)
	}

	return messages, nil
}

// TranslateToModern converts a TDP (Legacy) message to one or more TDPB (Modern) messages.
func TranslateToModern(msg protocol.Message) ([]protocol.Message, error) {
	messages := make([]protocol.Message, 0, 1)
	switch m := msg.(type) {
	case tdp.ClientScreenSpec:
		messages = append(messages, &tdpb.ClientScreenSpec{
			Height: m.Height,
			Width:  m.Width,
		})
	case tdp.PNG2Frame:
		messages = append(messages, &tdpb.PNGFrame{
			Coordinates: &tdpbv1.Rectangle{
				Top:    m.Top(),
				Left:   m.Left(),
				Bottom: m.Bottom(),
				Right:  m.Right(),
			},
			Data: m.Data(),
		})
	case tdp.PNGFrame:
		buf := new(bytes.Buffer)
		if err := png.Encode(buf, m.Img); err != nil {
			return nil, trace.Errorf("Erroring converting TDP PNGFrame to TDPB - dropping message!: %w", err)
		}
		messages = append(messages, &tdpb.PNGFrame{
			Coordinates: &tdpbv1.Rectangle{
				Top:    uint32(m.Img.Bounds().Min.Y),
				Left:   uint32(m.Img.Bounds().Min.X),
				Bottom: uint32(m.Img.Bounds().Max.Y),
				Right:  uint32(m.Img.Bounds().Max.X),
			},
			Data: buf.Bytes(),
		})
	case tdp.MouseMove:
		messages = append(messages, &tdpb.MouseMove{
			X: m.X,
			Y: m.Y,
		})
	case tdp.MouseButton:
		messages = append(messages, &tdpb.MouseButton{
			Pressed: m.State == tdp.ButtonPressed,
			Button:  tdpbv1.MouseButtonType(m.Button + 1),
		})
	case tdp.KeyboardButton:
		messages = append(messages, &tdpb.KeyboardButton{
			KeyCode: m.KeyCode,
			Pressed: m.State == tdp.ButtonPressed,
		})
	case tdp.ClipboardData:
		messages = append(messages, &tdpb.ClipboardData{
			Data: m,
		})
	case tdp.MouseWheel:
		messages = append(messages, &tdpb.MouseWheel{
			Axis:  tdpbv1.MouseWheelAxis(m.Axis + 1),
			Delta: uint32(m.Delta),
		})
	case tdp.Error:
		messages = append(messages, &tdpb.Alert{
			Message:  m.Message,
			Severity: tdpbv1.AlertSeverity_ALERT_SEVERITY_ERROR,
		})
	case tdp.Alert:
		var severity tdpbv1.AlertSeverity
		switch m.Severity {
		case tdp.SeverityError:
			severity = tdpbv1.AlertSeverity_ALERT_SEVERITY_ERROR
		case tdp.SeverityWarning:
			severity = tdpbv1.AlertSeverity_ALERT_SEVERITY_WARNING
		default:
			severity = tdpbv1.AlertSeverity_ALERT_SEVERITY_INFO
		}
		messages = append(messages, &tdpb.Alert{
			Message:  m.Message,
			Severity: severity,
		})
	case tdp.RDPFastPathPDU:
		messages = append(messages, &tdpb.FastPathPDU{
			Pdu: m,
		})
	case tdp.RDPResponsePDU:
		messages = append(messages, &tdpb.RDPResponsePDU{
			Response: m,
		})
	case tdp.ConnectionActivated:
		// Legacy TDP servers send this message once at the start
		// of the connection.
		messages = append(messages, &tdpb.ServerHello{
			ActivationSpec: &tdpbv1.ConnectionActivated{
				IoChannelId:   uint32(m.IOChannelID),
				UserChannelId: uint32(m.UserChannelID),
				ScreenWidth:   uint32(m.ScreenWidth),
				ScreenHeight:  uint32(m.ScreenHeight),
			},
			// Assume all legacy TDP servers support clipboard sharing
			ClipboardEnabled: true,
		})
	case tdp.SyncKeys:
		messages = append(messages, &tdpb.SyncKeys{
			ScrollLockPressed: m.ScrollLockState == tdp.ButtonPressed,
			NumLockState:      m.NumLockState == tdp.ButtonPressed,
			CapsLockState:     m.CapsLockState == tdp.ButtonPressed,
			KanaLockState:     m.KanaLockState == tdp.ButtonPressed,
		})
	case tdp.LatencyStats:
		messages = append(messages, &tdpb.LatencyStats{
			ClientLatencyMs: m.ClientLatency,
			ServerLatencyMs: m.ServerLatency,
		})
	case tdp.Ping:
		messages = append(messages, &tdpb.Ping{
			Uuid: m.UUID[:],
		})
	case tdp.SharedDirectoryAnnounce:
		messages = append(messages, &tdpb.SharedDirectoryAnnounce{
			DirectoryId: m.DirectoryID,
			Name:        m.Name,
		})
	case tdp.SharedDirectoryAcknowledge:
		messages = append(messages, &tdpb.SharedDirectoryAcknowledge{
			DirectoryId: m.DirectoryID,
			ErrorCode:   m.ErrCode,
		})
	case tdp.SharedDirectoryInfoRequest:
		messages = append(messages, &tdpb.SharedDirectoryRequest{
			DirectoryId:  m.DirectoryID,
			CompletionId: m.CompletionID,
			Operation: &tdpbv1.SharedDirectoryRequest_Info_{
				Info: &tdpbv1.SharedDirectoryRequest_Info{
					Path: m.Path,
				},
			},
		})
	case tdp.SharedDirectoryInfoResponse:
		messages = append(messages, &tdpb.SharedDirectoryResponse{
			ErrorCode:    m.ErrCode,
			CompletionId: m.CompletionID,
			Operation: &tdpbv1.SharedDirectoryResponse_Info_{
				Info: &tdpbv1.SharedDirectoryResponse_Info{
					Fso: translateFsoToModern(m.Fso),
				},
			},
		})
	case tdp.SharedDirectoryCreateRequest:
		messages = append(messages, &tdpb.SharedDirectoryRequest{
			DirectoryId:  m.DirectoryID,
			CompletionId: m.CompletionID,
			Operation: &tdpbv1.SharedDirectoryRequest_Create_{
				Create: &tdpbv1.SharedDirectoryRequest_Create{
					Path: m.Path,
				},
			},
		})
	case tdp.SharedDirectoryCreateResponse:
		messages = append(messages, &tdpb.SharedDirectoryResponse{
			ErrorCode:    m.ErrCode,
			CompletionId: m.CompletionID,
			Operation: &tdpbv1.SharedDirectoryResponse_Create_{
				Create: &tdpbv1.SharedDirectoryResponse_Create{
					Fso: translateFsoToModern(m.Fso),
				},
			},
		})
	case tdp.SharedDirectoryDeleteRequest:
		messages = append(messages, &tdpb.SharedDirectoryRequest{
			DirectoryId:  m.DirectoryID,
			CompletionId: m.CompletionID,
			Operation: &tdpbv1.SharedDirectoryRequest_Delete_{
				Delete: &tdpbv1.SharedDirectoryRequest_Delete{
					Path: m.Path,
				},
			},
		})
	case tdp.SharedDirectoryDeleteResponse:
		messages = append(messages, &tdpb.SharedDirectoryResponse{
			ErrorCode:    m.ErrCode,
			CompletionId: m.CompletionID,
			Operation:    &tdpbv1.SharedDirectoryResponse_Delete_{},
		})
	case tdp.SharedDirectoryReadRequest:
		messages = append(messages, &tdpb.SharedDirectoryRequest{
			CompletionId: m.CompletionID,
			DirectoryId:  m.DirectoryID,
			Operation: &tdpbv1.SharedDirectoryRequest_Read_{
				Read: &tdpbv1.SharedDirectoryRequest_Read{
					Path:   m.Path,
					Offset: m.Offset,
					Length: m.Length,
				},
			},
		})
	case tdp.SharedDirectoryReadResponse:
		messages = append(messages, &tdpb.SharedDirectoryResponse{
			CompletionId: m.CompletionID,
			ErrorCode:    m.ErrCode,
			Operation: &tdpbv1.SharedDirectoryResponse_Read_{
				Read: &tdpbv1.SharedDirectoryResponse_Read{
					Data: m.ReadData,
				},
			},
		})
	case tdp.SharedDirectoryWriteRequest:
		messages = append(messages, &tdpb.SharedDirectoryRequest{
			CompletionId: m.CompletionID,
			DirectoryId:  m.DirectoryID,
			Operation: &tdpbv1.SharedDirectoryRequest_Write_{
				Write: &tdpbv1.SharedDirectoryRequest_Write{
					Path:   m.Path,
					Offset: m.Offset,
					Data:   m.WriteData,
				},
			},
		})
	case tdp.SharedDirectoryWriteResponse:
		messages = append(messages, &tdpb.SharedDirectoryResponse{
			CompletionId: m.CompletionID,
			ErrorCode:    m.ErrCode,
			Operation: &tdpbv1.SharedDirectoryResponse_Write_{
				Write: &tdpbv1.SharedDirectoryResponse_Write{
					BytesWritten: m.BytesWritten,
				},
			},
		})
	case tdp.SharedDirectoryMoveRequest:
		messages = append(messages, &tdpb.SharedDirectoryRequest{
			CompletionId: m.CompletionID,
			DirectoryId:  m.DirectoryID,
			Operation: &tdpbv1.SharedDirectoryRequest_Move_{
				Move: &tdpbv1.SharedDirectoryRequest_Move{
					OriginalPath: m.OriginalPath,
					NewPath:      m.NewPath,
				},
			},
		})
	case tdp.SharedDirectoryMoveResponse:
		messages = append(messages, &tdpb.SharedDirectoryResponse{
			CompletionId: m.CompletionID,
			ErrorCode:    m.ErrCode,
			Operation:    &tdpbv1.SharedDirectoryResponse_Move_{},
		})
	case tdp.SharedDirectoryListRequest:
		messages = append(messages, &tdpb.SharedDirectoryRequest{
			CompletionId: m.CompletionID,
			Operation: &tdpbv1.SharedDirectoryRequest_List_{
				List: &tdpbv1.SharedDirectoryRequest_List{
					Path: m.Path,
				},
			},
		})
	case tdp.SharedDirectoryListResponse:
		messages = append(messages, &tdpb.SharedDirectoryResponse{
			CompletionId: m.CompletionID,
			ErrorCode:    m.ErrCode,
			Operation: &tdpbv1.SharedDirectoryResponse_List_{
				List: &tdpbv1.SharedDirectoryResponse_List{
					FsoList: translateFsoListToModern(m.FsoList),
				},
			},
		})
	case tdp.SharedDirectoryTruncateRequest:
		messages = append(messages, &tdpb.SharedDirectoryRequest{
			DirectoryId:  m.DirectoryID,
			CompletionId: m.CompletionID,
			Operation: &tdpbv1.SharedDirectoryRequest_Truncate_{
				Truncate: &tdpbv1.SharedDirectoryRequest_Truncate{
					Path:      m.Path,
					EndOfFile: m.EndOfFile,
				},
			},
		})
	case tdp.SharedDirectoryTruncateResponse:
		messages = append(messages, &tdpb.SharedDirectoryResponse{
			CompletionId: m.CompletionID,
			ErrorCode:    m.ErrCode,
		})
	default:
		return nil, trace.Errorf("Could not translate to TDPB. Encountered unexpected message type %T", m)
	}

	wrapped := []protocol.Message{}
	for _, msg := range messages {
		wrapped = append(wrapped, msg)
	}
	return wrapped, nil
}
