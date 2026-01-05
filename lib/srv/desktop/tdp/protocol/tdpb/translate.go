package tdpb

import (
	"bytes"
	"image/png"

	"github.com/google/uuid"
	tdpbv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/desktop/v1"
	tdpRoot "github.com/gravitational/teleport/lib/srv/desktop/tdp"
	tdp "github.com/gravitational/teleport/lib/srv/desktop/tdp/protocol/legacy"
	"github.com/gravitational/teleport/lib/utils/slices"
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

func translateFsoToModern(fso tdp.FileSystemObject) *tdpbv1.FileSystemObject {
	return &tdpbv1.FileSystemObject{
		LastModified: fso.LastModified,
		Size:         fso.Size,
		FileType:     fso.FileType,
		IsEmpty:      fso.IsEmpty == 1,
		Path:         fso.Path,
	}
}

func toButtonState(b bool) tdp.ButtonState {
	if b {
		return tdp.ButtonPressed
	}
	return tdp.ButtonNotPressed
}

// TranslateToLegacy converts a TDPB (Modern) message to one or more TDP (Legacy) messages.
func TranslateToLegacy(msg tdpRoot.Message) ([]tdpRoot.Message, error) {
	switch m := msg.(type) {
	case *PNGFrame:
		return []tdpRoot.Message{tdp.PNG2Frame(m.Data)}, nil
	case *FastPathPDU:
		return []tdpRoot.Message{tdp.RDPFastPathPDU(m.Pdu)}, nil
	case *RDPResponsePDU:
		return []tdpRoot.Message{tdp.RDPResponsePDU(m.Response)}, nil
	case *SyncKeys:
		return []tdpRoot.Message{tdp.SyncKeys{
			ScrollLockState: toButtonState(m.ScrollLockPressed),
			NumLockState:    toButtonState(m.NumLockState),
			CapsLockState:   toButtonState(m.CapsLockState),
			KanaLockState:   toButtonState(m.KanaLockState),
		}}, nil
	case *MouseMove:
		return []tdpRoot.Message{tdp.MouseMove{X: m.X, Y: m.Y}}, nil
	case *MouseButton:
		button := tdp.MouseButtonType(m.Button - 1)
		state := tdp.ButtonNotPressed
		if m.Pressed {
			state = tdp.ButtonPressed
		}

		return []tdpRoot.Message{tdp.MouseButton{
			Button: button,
			State:  state,
		}}, nil
	case *KeyboardButton:
		state := tdp.ButtonNotPressed
		if m.Pressed {
			state = tdp.ButtonPressed
		}
		return []tdpRoot.Message{tdp.KeyboardButton{
			KeyCode: m.KeyCode,
			State:   state,
		}}, nil
	case *ClientScreenSpec:
		return []tdpRoot.Message{tdp.ClientScreenSpec{
			Width:  m.Width,
			Height: m.Height,
		}}, nil
	case *Alert:
		var severity tdp.Severity
		switch m.Severity {
		case tdpbv1.AlertSeverity_ALERT_SEVERITY_WARNING:
			severity = tdp.SeverityWarning
		case tdpbv1.AlertSeverity_ALERT_SEVERITY_ERROR:
			severity = tdp.SeverityError
		default:
			severity = tdp.SeverityInfo
		}
		return []tdpRoot.Message{tdp.Alert{
			Message:  m.Message,
			Severity: severity,
		}}, nil
	case *MouseWheel:
		return []tdpRoot.Message{tdp.MouseWheel{
			Axis:  tdp.MouseWheelAxis(m.Axis - 1),
			Delta: int16(m.Delta),
		}}, nil
	case *ClipboardData:
		return []tdpRoot.Message{tdp.ClipboardData(m.Data)}, nil
	case *SharedDirectoryAnnounce:
		return []tdpRoot.Message{tdp.SharedDirectoryAnnounce{
			DirectoryID: m.DirectoryId,
			Name:        m.Name,
		}}, nil
	case *SharedDirectoryAcknowledge:
		return []tdpRoot.Message{tdp.SharedDirectoryAcknowledge{
			DirectoryID: m.DirectoryId,
			ErrCode:     m.ErrorCode,
		}}, nil
	case *SharedDirectoryRequest:
		switch op := m.Operation.(type) {
		case *tdpbv1.SharedDirectoryRequest_Info_:
			return []tdpRoot.Message{tdp.SharedDirectoryInfoRequest{
				CompletionID: m.CompletionId,
				DirectoryID:  m.DirectoryId,
				Path:         op.Info.Path,
			}}, nil
		case *tdpbv1.SharedDirectoryRequest_Create_:
			return []tdpRoot.Message{tdp.SharedDirectoryCreateRequest{
				CompletionID: m.CompletionId,
				DirectoryID:  m.DirectoryId,
				FileType:     op.Create.FileType,
				Path:         op.Create.Path,
			}}, nil
		case *tdpbv1.SharedDirectoryRequest_Delete_:
			return []tdpRoot.Message{tdp.SharedDirectoryDeleteRequest{
				CompletionID: m.CompletionId,
				DirectoryID:  m.DirectoryId,
				Path:         op.Delete.Path,
			}}, nil
		case *tdpbv1.SharedDirectoryRequest_List_:
			return []tdpRoot.Message{tdp.SharedDirectoryListRequest{
				CompletionID: m.CompletionId,
				DirectoryID:  m.DirectoryId,
				Path:         op.List.Path,
			}}, nil
		case *tdpbv1.SharedDirectoryRequest_Read_:
			return []tdpRoot.Message{tdp.SharedDirectoryReadRequest{
				CompletionID: m.CompletionId,
				DirectoryID:  m.DirectoryId,
				Path:         op.Read.Path,
				Offset:       op.Read.Offset,
				Length:       op.Read.Length,
			}}, nil
		case *tdpbv1.SharedDirectoryRequest_Write_:
			return []tdpRoot.Message{tdp.SharedDirectoryWriteRequest{
				CompletionID:    m.CompletionId,
				DirectoryID:     m.DirectoryId,
				Path:            op.Write.Path,
				Offset:          op.Write.Offset,
				WriteDataLength: uint32(len(op.Write.Data)),
				WriteData:       op.Write.Data,
			}}, nil
		case *tdpbv1.SharedDirectoryRequest_Move_:
			return []tdpRoot.Message{tdp.SharedDirectoryMoveRequest{
				CompletionID: m.CompletionId,
				DirectoryID:  m.DirectoryId,
				NewPath:      op.Move.NewPath,
				OriginalPath: op.Move.OriginalPath,
			}}, nil
		case *tdpbv1.SharedDirectoryRequest_Truncate_:
			return []tdpRoot.Message{tdp.SharedDirectoryTruncateRequest{
				CompletionID: m.CompletionId,
				DirectoryID:  m.DirectoryId,
				Path:         op.Truncate.Path,
				EndOfFile:    op.Truncate.EndOfFile,
			}}, nil
		default:
			return nil, trace.Errorf("Unknown shared directory operation")
		}
	case *SharedDirectoryResponse:
		switch op := m.Operation.(type) {
		case *tdpbv1.SharedDirectoryResponse_Info_:
			return []tdpRoot.Message{tdp.SharedDirectoryInfoResponse{
				CompletionID: m.CompletionId,
				ErrCode:      m.ErrorCode,
				Fso:          translateFso(op.Info.Fso),
			}}, nil
		case *tdpbv1.SharedDirectoryResponse_Create_:
			return []tdpRoot.Message{tdp.SharedDirectoryCreateResponse{
				CompletionID: m.CompletionId,
				ErrCode:      m.ErrorCode,
				Fso:          translateFso(op.Create.Fso),
			}}, nil
		case *tdpbv1.SharedDirectoryResponse_Delete_:
			return []tdpRoot.Message{tdp.SharedDirectoryDeleteResponse{
				CompletionID: m.CompletionId,
				ErrCode:      m.ErrorCode,
			}}, nil
		case *tdpbv1.SharedDirectoryResponse_List_:
			return []tdpRoot.Message{tdp.SharedDirectoryListResponse{
				CompletionID: m.CompletionId,
				ErrCode:      m.ErrorCode,
				FsoList:      slices.Map(op.List.FsoList, translateFso),
			}}, nil
		case *tdpbv1.SharedDirectoryResponse_Read_:
			return []tdpRoot.Message{tdp.SharedDirectoryReadResponse{
				CompletionID:   m.CompletionId,
				ErrCode:        m.ErrorCode,
				ReadData:       op.Read.Data,
				ReadDataLength: uint32(len(op.Read.Data)),
			}}, nil
		case *tdpbv1.SharedDirectoryResponse_Write_:
			return []tdpRoot.Message{tdp.SharedDirectoryWriteResponse{
				CompletionID: m.CompletionId,
				ErrCode:      m.ErrorCode,
				BytesWritten: op.Write.BytesWritten,
			}}, nil
		case *tdpbv1.SharedDirectoryResponse_Move_:
			return []tdpRoot.Message{tdp.SharedDirectoryMoveResponse{
				CompletionID: m.CompletionId,
				ErrCode:      m.ErrorCode,
			}}, nil
		case *tdpbv1.SharedDirectoryResponse_Truncate_:
			return []tdpRoot.Message{tdp.SharedDirectoryTruncateResponse{
				CompletionID: m.CompletionId,
				ErrCode:      m.ErrorCode,
			}}, nil
		default:
			return nil, trace.Errorf("Unknown shared directory operation")
		}
	case *LatencyStats:
		return []tdpRoot.Message{tdp.LatencyStats{
			ClientLatency: m.ClientLatencyMs,
			ServerLatency: m.ServerLatencyMs,
		}}, nil
	case *Ping:
		id, err := uuid.FromBytes(m.Uuid)
		if err != nil {
			return nil, trace.Errorf("Cannot parse uuid bytes from ping", "error", err)
		}
		return []tdpRoot.Message{tdp.Ping{UUID: id}}, nil
	default:
		return nil, trace.Errorf("Could not translate to TDP. Encountered unexpected message type %T", m)
	}
}

// TranslateToModern converts a TDP (Legacy) message to one or more TDPB (Modern) messages.
func TranslateToModern(msg tdpRoot.Message) ([]tdpRoot.Message, error) {
	switch m := msg.(type) {
	case tdp.ClientScreenSpec:
		return []tdpRoot.Message{&ClientScreenSpec{
			Height: m.Height,
			Width:  m.Width,
		}}, nil
	case tdp.PNG2Frame:
		return []tdpRoot.Message{&PNGFrame{
			Coordinates: &tdpbv1.Rectangle{
				Top:    m.Top(),
				Left:   m.Left(),
				Bottom: m.Bottom(),
				Right:  m.Right(),
			},
			Data: m.Data(),
		}}, nil
	case tdp.PNGFrame:
		buf := new(bytes.Buffer)
		if err := png.Encode(buf, m.Img); err != nil {
			return nil, trace.Errorf("Erroring converting TDP PNGFrame to TDPB - dropping message!: %w", err)
		}
		return []tdpRoot.Message{&PNGFrame{
			Coordinates: &tdpbv1.Rectangle{
				Top:    uint32(m.Img.Bounds().Min.Y),
				Left:   uint32(m.Img.Bounds().Min.X),
				Bottom: uint32(m.Img.Bounds().Max.Y),
				Right:  uint32(m.Img.Bounds().Max.X),
			},
			Data: buf.Bytes(),
		}}, nil
	case tdp.MouseMove:
		return []tdpRoot.Message{&MouseMove{
			X: m.X,
			Y: m.Y,
		}}, nil
	case tdp.MouseButton:
		return []tdpRoot.Message{&MouseButton{
			Pressed: m.State == tdp.ButtonPressed,
			Button:  tdpbv1.MouseButtonType(m.Button + 1),
		}}, nil
	case tdp.KeyboardButton:
		return []tdpRoot.Message{&KeyboardButton{
			KeyCode: m.KeyCode,
			Pressed: m.State == tdp.ButtonPressed,
		}}, nil
	case tdp.ClipboardData:
		return []tdpRoot.Message{&ClipboardData{
			Data: m,
		}}, nil
	case tdp.MouseWheel:
		return []tdpRoot.Message{&MouseWheel{
			Axis:  tdpbv1.MouseWheelAxis(m.Axis + 1),
			Delta: uint32(m.Delta),
		}}, nil
	case tdp.Error:
		return []tdpRoot.Message{&Alert{
			Message:  m.Message,
			Severity: tdpbv1.AlertSeverity_ALERT_SEVERITY_ERROR,
		}}, nil
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
		return []tdpRoot.Message{&Alert{
			Message:  m.Message,
			Severity: severity,
		}}, nil
	case tdp.RDPFastPathPDU:
		return []tdpRoot.Message{&FastPathPDU{
			Pdu: m,
		}}, nil
	case tdp.RDPResponsePDU:
		return []tdpRoot.Message{&RDPResponsePDU{
			Response: m,
		}}, nil
	case tdp.ConnectionActivated:
		// Legacy TDP servers send this message once at the start
		// of the connection.
		return []tdpRoot.Message{&ServerHello{
			ActivationSpec: &tdpbv1.ConnectionActivated{
				IoChannelId:   uint32(m.IOChannelID),
				UserChannelId: uint32(m.UserChannelID),
				ScreenWidth:   uint32(m.ScreenWidth),
				ScreenHeight:  uint32(m.ScreenHeight),
			},
			// Assume all legacy TDP servers support clipboard sharing
			ClipboardEnabled: true,
		}}, nil
	case tdp.SyncKeys:
		return []tdpRoot.Message{&SyncKeys{
			ScrollLockPressed: m.ScrollLockState == tdp.ButtonPressed,
			NumLockState:      m.NumLockState == tdp.ButtonPressed,
			CapsLockState:     m.CapsLockState == tdp.ButtonPressed,
			KanaLockState:     m.KanaLockState == tdp.ButtonPressed,
		}}, nil
	case tdp.LatencyStats:
		return []tdpRoot.Message{&LatencyStats{
			ClientLatencyMs: m.ClientLatency,
			ServerLatencyMs: m.ServerLatency,
		}}, nil
	case tdp.Ping:
		return []tdpRoot.Message{&Ping{
			Uuid: m.UUID[:],
		}}, nil
	case tdp.SharedDirectoryAnnounce:
		return []tdpRoot.Message{&SharedDirectoryAnnounce{
			DirectoryId: m.DirectoryID,
			Name:        m.Name,
		}}, nil
	case tdp.SharedDirectoryAcknowledge:
		return []tdpRoot.Message{&SharedDirectoryAcknowledge{
			DirectoryId: m.DirectoryID,
			ErrorCode:   m.ErrCode,
		}}, nil
	case tdp.SharedDirectoryInfoRequest:
		return []tdpRoot.Message{&SharedDirectoryRequest{
			DirectoryId:  m.DirectoryID,
			CompletionId: m.CompletionID,
			Operation: &tdpbv1.SharedDirectoryRequest_Info_{
				Info: &tdpbv1.SharedDirectoryRequest_Info{
					Path: m.Path,
				},
			},
		}}, nil
	case tdp.SharedDirectoryInfoResponse:
		return []tdpRoot.Message{&SharedDirectoryResponse{
			ErrorCode:    m.ErrCode,
			CompletionId: m.CompletionID,
			Operation: &tdpbv1.SharedDirectoryResponse_Info_{
				Info: &tdpbv1.SharedDirectoryResponse_Info{
					Fso: translateFsoToModern(m.Fso),
				},
			},
		}}, nil
	case tdp.SharedDirectoryCreateRequest:
		return []tdpRoot.Message{&SharedDirectoryRequest{
			DirectoryId:  m.DirectoryID,
			CompletionId: m.CompletionID,
			Operation: &tdpbv1.SharedDirectoryRequest_Create_{
				Create: &tdpbv1.SharedDirectoryRequest_Create{
					Path: m.Path,
				},
			},
		}}, nil
	case tdp.SharedDirectoryCreateResponse:
		return []tdpRoot.Message{&SharedDirectoryResponse{
			ErrorCode:    m.ErrCode,
			CompletionId: m.CompletionID,
			Operation: &tdpbv1.SharedDirectoryResponse_Create_{
				Create: &tdpbv1.SharedDirectoryResponse_Create{
					Fso: translateFsoToModern(m.Fso),
				},
			},
		}}, nil
	case tdp.SharedDirectoryDeleteRequest:
		return []tdpRoot.Message{&SharedDirectoryRequest{
			DirectoryId:  m.DirectoryID,
			CompletionId: m.CompletionID,
			Operation: &tdpbv1.SharedDirectoryRequest_Delete_{
				Delete: &tdpbv1.SharedDirectoryRequest_Delete{
					Path: m.Path,
				},
			},
		}}, nil
	case tdp.SharedDirectoryDeleteResponse:
		return []tdpRoot.Message{&SharedDirectoryResponse{
			ErrorCode:    m.ErrCode,
			CompletionId: m.CompletionID,
			Operation:    &tdpbv1.SharedDirectoryResponse_Delete_{},
		}}, nil
	case tdp.SharedDirectoryReadRequest:
		return []tdpRoot.Message{&SharedDirectoryRequest{
			CompletionId: m.CompletionID,
			DirectoryId:  m.DirectoryID,
			Operation: &tdpbv1.SharedDirectoryRequest_Read_{
				Read: &tdpbv1.SharedDirectoryRequest_Read{
					Path:   m.Path,
					Offset: m.Offset,
					Length: m.Length,
				},
			},
		}}, nil
	case tdp.SharedDirectoryReadResponse:
		return []tdpRoot.Message{&SharedDirectoryResponse{
			CompletionId: m.CompletionID,
			ErrorCode:    m.ErrCode,
			Operation: &tdpbv1.SharedDirectoryResponse_Read_{
				Read: &tdpbv1.SharedDirectoryResponse_Read{
					Data: m.ReadData,
				},
			},
		}}, nil
	case tdp.SharedDirectoryWriteRequest:
		return []tdpRoot.Message{&SharedDirectoryRequest{
			CompletionId: m.CompletionID,
			DirectoryId:  m.DirectoryID,
			Operation: &tdpbv1.SharedDirectoryRequest_Write_{
				Write: &tdpbv1.SharedDirectoryRequest_Write{
					Path:   m.Path,
					Offset: m.Offset,
					Data:   m.WriteData,
				},
			},
		}}, nil
	case tdp.SharedDirectoryWriteResponse:
		return []tdpRoot.Message{&SharedDirectoryResponse{
			CompletionId: m.CompletionID,
			ErrorCode:    m.ErrCode,
			Operation: &tdpbv1.SharedDirectoryResponse_Write_{
				Write: &tdpbv1.SharedDirectoryResponse_Write{
					BytesWritten: m.BytesWritten,
				},
			},
		}}, nil
	case tdp.SharedDirectoryMoveRequest:
		return []tdpRoot.Message{&SharedDirectoryRequest{
			CompletionId: m.CompletionID,
			DirectoryId:  m.DirectoryID,
			Operation: &tdpbv1.SharedDirectoryRequest_Move_{
				Move: &tdpbv1.SharedDirectoryRequest_Move{
					OriginalPath: m.OriginalPath,
					NewPath:      m.NewPath,
				},
			},
		}}, nil
	case tdp.SharedDirectoryMoveResponse:
		return []tdpRoot.Message{&SharedDirectoryResponse{
			CompletionId: m.CompletionID,
			ErrorCode:    m.ErrCode,
			Operation:    &tdpbv1.SharedDirectoryResponse_Move_{},
		}}, nil
	case tdp.SharedDirectoryListRequest:
		return []tdpRoot.Message{&SharedDirectoryRequest{
			CompletionId: m.CompletionID,
			Operation: &tdpbv1.SharedDirectoryRequest_List_{
				List: &tdpbv1.SharedDirectoryRequest_List{
					Path: m.Path,
				},
			},
		}}, nil
	case tdp.SharedDirectoryListResponse:
		return []tdpRoot.Message{&SharedDirectoryResponse{
			CompletionId: m.CompletionID,
			ErrorCode:    m.ErrCode,
			Operation: &tdpbv1.SharedDirectoryResponse_List_{
				List: &tdpbv1.SharedDirectoryResponse_List{
					FsoList: slices.Map(m.FsoList, translateFsoToModern),
				},
			},
		}}, nil
	case tdp.SharedDirectoryTruncateRequest:
		return []tdpRoot.Message{&SharedDirectoryRequest{
			DirectoryId:  m.DirectoryID,
			CompletionId: m.CompletionID,
			Operation: &tdpbv1.SharedDirectoryRequest_Truncate_{
				Truncate: &tdpbv1.SharedDirectoryRequest_Truncate{
					Path:      m.Path,
					EndOfFile: m.EndOfFile,
				},
			},
		}}, nil
	case tdp.SharedDirectoryTruncateResponse:
		return []tdpRoot.Message{&SharedDirectoryResponse{
			CompletionId: m.CompletionID,
			ErrorCode:    m.ErrCode,
		}}, nil
	default:
		return nil, trace.Errorf("Could not translate to  Encountered unexpected message type %T", m)
	}
}
