/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package tdpb

import (
	"bytes"
	"image/png"
	"math"

	"github.com/google/uuid"
	"github.com/gravitational/trace"

	tdpbv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/desktop/v1"
	"github.com/gravitational/teleport/lib/srv/desktop/tdp"
	"github.com/gravitational/teleport/lib/srv/desktop/tdp/protocol/legacy"
	"github.com/gravitational/teleport/lib/utils/slices"
)

func translateFSO(fso *tdpbv1.FileSystemObject) legacy.FileSystemObject {
	isEmpty := uint8(0)
	if fso.IsEmpty {
		isEmpty = 1
	}
	return legacy.FileSystemObject{
		LastModified: fso.LastModified,
		Size:         fso.Size,
		FileType:     fso.FileType,
		IsEmpty:      isEmpty,
		Path:         fso.Path,
	}
}

func translateFSOToModern(fso legacy.FileSystemObject) *tdpbv1.FileSystemObject {
	return &tdpbv1.FileSystemObject{
		LastModified: fso.LastModified,
		Size:         fso.Size,
		FileType:     fso.FileType,
		IsEmpty:      fso.IsEmpty == 1,
		Path:         fso.Path,
	}
}

func toButtonState(b bool) legacy.ButtonState {
	if b {
		return legacy.ButtonPressed
	}
	return legacy.ButtonNotPressed
}

func clampInt32ToInt16(val int32) int16 {
	switch {
	case val < math.MinInt16:
		return math.MinInt16
	case val > math.MaxInt16:
		return math.MaxInt16
	default:
		return int16(val)
	}
}

// TranslateToLegacy converts a TDPB (Modern) message to one or more TDP (Legacy) messages.
func TranslateToLegacy(msg tdp.Message) ([]tdp.Message, error) {
	switch m := msg.(type) {
	case *PNGFrame:
		return []tdp.Message{legacy.PNG2Frame(m.Data)}, nil
	case *FastPathPDU:
		return []tdp.Message{legacy.RDPFastPathPDU(m.Pdu)}, nil
	case *RDPResponsePDU:
		return []tdp.Message{legacy.RDPResponsePDU(m.Response)}, nil
	case *SyncKeys:
		return []tdp.Message{legacy.SyncKeys{
			ScrollLockState: toButtonState(m.ScrollLockPressed),
			NumLockState:    toButtonState(m.NumLockState),
			CapsLockState:   toButtonState(m.CapsLockState),
			KanaLockState:   toButtonState(m.KanaLockState),
		}}, nil
	case *MouseMove:
		return []tdp.Message{legacy.MouseMove{X: m.X, Y: m.Y}}, nil
	case *MouseButton:
		button := legacy.MouseButtonType(m.Button - 1)
		state := legacy.ButtonNotPressed
		if m.Pressed {
			state = legacy.ButtonPressed
		}

		return []tdp.Message{legacy.MouseButton{
			Button: button,
			State:  state,
		}}, nil
	case *KeyboardButton:
		state := legacy.ButtonNotPressed
		if m.Pressed {
			state = legacy.ButtonPressed
		}
		return []tdp.Message{legacy.KeyboardButton{
			KeyCode: m.KeyCode,
			State:   state,
		}}, nil
	case *ClientScreenSpec:
		return []tdp.Message{legacy.ClientScreenSpec{
			Width:  m.Width,
			Height: m.Height,
		}}, nil
	case *Alert:
		var severity legacy.Severity
		switch m.Severity {
		case tdpbv1.AlertSeverity_ALERT_SEVERITY_WARNING:
			severity = legacy.SeverityWarning
		case tdpbv1.AlertSeverity_ALERT_SEVERITY_ERROR:
			severity = legacy.SeverityError
		default:
			severity = legacy.SeverityInfo
		}
		return []tdp.Message{legacy.Alert{
			Message:  m.Message,
			Severity: severity,
		}}, nil
	case *MouseWheel:
		return []tdp.Message{legacy.MouseWheel{
			Axis:  legacy.MouseWheelAxis(m.Axis - 1),
			Delta: clampInt32ToInt16(m.Delta),
		}}, nil
	case *ClipboardData:
		return []tdp.Message{legacy.ClipboardData(m.Data)}, nil
	case *SharedDirectoryAnnounce:
		return []tdp.Message{legacy.SharedDirectoryAnnounce{
			DirectoryID: m.DirectoryId,
			Name:        m.Name,
		}}, nil
	case *SharedDirectoryAcknowledge:
		return []tdp.Message{legacy.SharedDirectoryAcknowledge{
			DirectoryID: m.DirectoryId,
			ErrCode:     m.ErrorCode,
		}}, nil
	case *SharedDirectoryRequest:
		switch op := m.Operation.(type) {
		case *tdpbv1.SharedDirectoryRequest_Info_:
			return []tdp.Message{legacy.SharedDirectoryInfoRequest{
				CompletionID: m.CompletionId,
				DirectoryID:  m.DirectoryId,
				Path:         op.Info.Path,
			}}, nil
		case *tdpbv1.SharedDirectoryRequest_Create_:
			return []tdp.Message{legacy.SharedDirectoryCreateRequest{
				CompletionID: m.CompletionId,
				DirectoryID:  m.DirectoryId,
				FileType:     op.Create.FileType,
				Path:         op.Create.Path,
			}}, nil
		case *tdpbv1.SharedDirectoryRequest_Delete_:
			return []tdp.Message{legacy.SharedDirectoryDeleteRequest{
				CompletionID: m.CompletionId,
				DirectoryID:  m.DirectoryId,
				Path:         op.Delete.Path,
			}}, nil
		case *tdpbv1.SharedDirectoryRequest_List_:
			return []tdp.Message{legacy.SharedDirectoryListRequest{
				CompletionID: m.CompletionId,
				DirectoryID:  m.DirectoryId,
				Path:         op.List.Path,
			}}, nil
		case *tdpbv1.SharedDirectoryRequest_Read_:
			return []tdp.Message{legacy.SharedDirectoryReadRequest{
				CompletionID: m.CompletionId,
				DirectoryID:  m.DirectoryId,
				Path:         op.Read.Path,
				Offset:       op.Read.Offset,
				Length:       op.Read.Length,
			}}, nil
		case *tdpbv1.SharedDirectoryRequest_Write_:
			return []tdp.Message{legacy.SharedDirectoryWriteRequest{
				CompletionID:    m.CompletionId,
				DirectoryID:     m.DirectoryId,
				Path:            op.Write.Path,
				Offset:          op.Write.Offset,
				WriteDataLength: uint32(len(op.Write.Data)),
				WriteData:       op.Write.Data,
			}}, nil
		case *tdpbv1.SharedDirectoryRequest_Move_:
			return []tdp.Message{legacy.SharedDirectoryMoveRequest{
				CompletionID: m.CompletionId,
				DirectoryID:  m.DirectoryId,
				NewPath:      op.Move.NewPath,
				OriginalPath: op.Move.OriginalPath,
			}}, nil
		case *tdpbv1.SharedDirectoryRequest_Truncate_:
			return []tdp.Message{legacy.SharedDirectoryTruncateRequest{
				CompletionID: m.CompletionId,
				DirectoryID:  m.DirectoryId,
				Path:         op.Truncate.Path,
				EndOfFile:    op.Truncate.EndOfFile,
			}}, nil
		default:
			return nil, trace.BadParameter("Unknown shared directory operation")
		}
	case *SharedDirectoryResponse:
		switch op := m.Operation.(type) {
		case *tdpbv1.SharedDirectoryResponse_Info_:
			return []tdp.Message{legacy.SharedDirectoryInfoResponse{
				CompletionID: m.CompletionId,
				ErrCode:      m.ErrorCode,
				Fso:          translateFSO(op.Info.Fso),
			}}, nil
		case *tdpbv1.SharedDirectoryResponse_Create_:
			return []tdp.Message{legacy.SharedDirectoryCreateResponse{
				CompletionID: m.CompletionId,
				ErrCode:      m.ErrorCode,
				Fso:          translateFSO(op.Create.Fso),
			}}, nil
		case *tdpbv1.SharedDirectoryResponse_Delete_:
			return []tdp.Message{legacy.SharedDirectoryDeleteResponse{
				CompletionID: m.CompletionId,
				ErrCode:      m.ErrorCode,
			}}, nil
		case *tdpbv1.SharedDirectoryResponse_List_:
			return []tdp.Message{legacy.SharedDirectoryListResponse{
				CompletionID: m.CompletionId,
				ErrCode:      m.ErrorCode,
				FsoList:      slices.Map(op.List.FsoList, translateFSO),
			}}, nil
		case *tdpbv1.SharedDirectoryResponse_Read_:
			return []tdp.Message{legacy.SharedDirectoryReadResponse{
				CompletionID:   m.CompletionId,
				ErrCode:        m.ErrorCode,
				ReadData:       op.Read.Data,
				ReadDataLength: uint32(len(op.Read.Data)),
			}}, nil
		case *tdpbv1.SharedDirectoryResponse_Write_:
			return []tdp.Message{legacy.SharedDirectoryWriteResponse{
				CompletionID: m.CompletionId,
				ErrCode:      m.ErrorCode,
				BytesWritten: op.Write.BytesWritten,
			}}, nil
		case *tdpbv1.SharedDirectoryResponse_Move_:
			return []tdp.Message{legacy.SharedDirectoryMoveResponse{
				CompletionID: m.CompletionId,
				ErrCode:      m.ErrorCode,
			}}, nil
		case *tdpbv1.SharedDirectoryResponse_Truncate_:
			return []tdp.Message{legacy.SharedDirectoryTruncateResponse{
				CompletionID: m.CompletionId,
				ErrCode:      m.ErrorCode,
			}}, nil
		default:
			return nil, trace.Errorf("Unknown shared directory operation")
		}
	case *LatencyStats:
		return []tdp.Message{legacy.LatencyStats{
			ClientLatency: m.ClientLatencyMs,
			ServerLatency: m.ServerLatencyMs,
		}}, nil
	case *Ping:
		id, err := uuid.FromBytes(m.Uuid)
		if err != nil {
			return nil, trace.WrapWithMessage(err, "Cannot parse uuid bytes from ping")
		}
		return []tdp.Message{legacy.Ping{UUID: id}}, nil
	case *ServerHello:
		return []tdp.Message{legacy.ConnectionActivated{
			IOChannelID:   uint16(m.ActivationSpec.IoChannelId),
			UserChannelID: uint16(m.ActivationSpec.UserChannelId),
			ScreenWidth:   uint16(m.ActivationSpec.ScreenWidth),
			ScreenHeight:  uint16(m.ActivationSpec.ScreenHeight),
		}}, nil
	default:
		return nil, trace.Errorf("Could not translate to TDP. Encountered unexpected message type %T", m)
	}
}

// TranslateToModern converts a TDP (Legacy) message to one or more TDPB (Modern) messages.
func TranslateToModern(msg tdp.Message) ([]tdp.Message, error) {
	switch m := msg.(type) {
	case legacy.ClientScreenSpec:
		return []tdp.Message{&ClientScreenSpec{
			Height: m.Height,
			Width:  m.Width,
		}}, nil
	case legacy.PNG2Frame:
		return []tdp.Message{&PNGFrame{
			Coordinates: &tdpbv1.Rectangle{
				Top:    m.Top(),
				Left:   m.Left(),
				Bottom: m.Bottom(),
				Right:  m.Right(),
			},
			Data: m.Data(),
		}}, nil
	case legacy.PNGFrame:
		buf := new(bytes.Buffer)
		if err := png.Encode(buf, m.Img); err != nil {
			return nil, trace.Errorf("Erroring converting TDP PNGFrame to TDPB - dropping message!: %w", err)
		}
		return []tdp.Message{&PNGFrame{
			Coordinates: &tdpbv1.Rectangle{
				Top:    uint32(m.Img.Bounds().Min.Y),
				Left:   uint32(m.Img.Bounds().Min.X),
				Bottom: uint32(m.Img.Bounds().Max.Y),
				Right:  uint32(m.Img.Bounds().Max.X),
			},
			Data: buf.Bytes(),
		}}, nil
	case legacy.MouseMove:
		return []tdp.Message{&MouseMove{
			X: m.X,
			Y: m.Y,
		}}, nil
	case legacy.MouseButton:
		return []tdp.Message{&MouseButton{
			Pressed: m.State == legacy.ButtonPressed,
			Button:  tdpbv1.MouseButtonType(m.Button + 1),
		}}, nil
	case legacy.KeyboardButton:
		return []tdp.Message{&KeyboardButton{
			KeyCode: m.KeyCode,
			Pressed: m.State == legacy.ButtonPressed,
		}}, nil
	case legacy.ClipboardData:
		return []tdp.Message{&ClipboardData{
			Data: m,
		}}, nil
	case legacy.MouseWheel:
		return []tdp.Message{&MouseWheel{
			Axis:  tdpbv1.MouseWheelAxis(m.Axis + 1),
			Delta: int32(m.Delta),
		}}, nil
	case legacy.Error:
		return []tdp.Message{&Alert{
			Message:  m.Message,
			Severity: tdpbv1.AlertSeverity_ALERT_SEVERITY_ERROR,
		}}, nil
	case legacy.Alert:
		var severity tdpbv1.AlertSeverity
		switch m.Severity {
		case legacy.SeverityError:
			severity = tdpbv1.AlertSeverity_ALERT_SEVERITY_ERROR
		case legacy.SeverityWarning:
			severity = tdpbv1.AlertSeverity_ALERT_SEVERITY_WARNING
		default:
			severity = tdpbv1.AlertSeverity_ALERT_SEVERITY_INFO
		}
		return []tdp.Message{&Alert{
			Message:  m.Message,
			Severity: severity,
		}}, nil
	case legacy.RDPFastPathPDU:
		return []tdp.Message{&FastPathPDU{
			Pdu: m,
		}}, nil
	case legacy.RDPResponsePDU:
		return []tdp.Message{&RDPResponsePDU{
			Response: m,
		}}, nil
	case legacy.ConnectionActivated:
		// Legacy TDP servers send this message once at the start
		// of the connection.
		return []tdp.Message{&ServerHello{
			ActivationSpec: &tdpbv1.ConnectionActivated{
				IoChannelId:   uint32(m.IOChannelID),
				UserChannelId: uint32(m.UserChannelID),
				ScreenWidth:   uint32(m.ScreenWidth),
				ScreenHeight:  uint32(m.ScreenHeight),
			},
			// Assume all legacy TDP servers support clipboard sharing
			ClipboardEnabled: true,
		}}, nil
	case legacy.SyncKeys:
		return []tdp.Message{&SyncKeys{
			ScrollLockPressed: m.ScrollLockState == legacy.ButtonPressed,
			NumLockState:      m.NumLockState == legacy.ButtonPressed,
			CapsLockState:     m.CapsLockState == legacy.ButtonPressed,
			KanaLockState:     m.KanaLockState == legacy.ButtonPressed,
		}}, nil
	case legacy.LatencyStats:
		return []tdp.Message{&LatencyStats{
			ClientLatencyMs: m.ClientLatency,
			ServerLatencyMs: m.ServerLatency,
		}}, nil
	case legacy.Ping:
		return []tdp.Message{&Ping{
			Uuid: m.UUID[:],
		}}, nil
	case legacy.SharedDirectoryAnnounce:
		return []tdp.Message{&SharedDirectoryAnnounce{
			DirectoryId: m.DirectoryID,
			Name:        m.Name,
		}}, nil
	case legacy.SharedDirectoryAcknowledge:
		return []tdp.Message{&SharedDirectoryAcknowledge{
			DirectoryId: m.DirectoryID,
			ErrorCode:   m.ErrCode,
		}}, nil
	case legacy.SharedDirectoryInfoRequest:
		return []tdp.Message{&SharedDirectoryRequest{
			DirectoryId:  m.DirectoryID,
			CompletionId: m.CompletionID,
			Operation: &tdpbv1.SharedDirectoryRequest_Info_{
				Info: &tdpbv1.SharedDirectoryRequest_Info{
					Path: m.Path,
				},
			},
		}}, nil
	case legacy.SharedDirectoryInfoResponse:
		return []tdp.Message{&SharedDirectoryResponse{
			ErrorCode:    m.ErrCode,
			CompletionId: m.CompletionID,
			Operation: &tdpbv1.SharedDirectoryResponse_Info_{
				Info: &tdpbv1.SharedDirectoryResponse_Info{
					Fso: translateFSOToModern(m.Fso),
				},
			},
		}}, nil
	case legacy.SharedDirectoryCreateRequest:
		return []tdp.Message{&SharedDirectoryRequest{
			DirectoryId:  m.DirectoryID,
			CompletionId: m.CompletionID,
			Operation: &tdpbv1.SharedDirectoryRequest_Create_{
				Create: &tdpbv1.SharedDirectoryRequest_Create{
					Path:     m.Path,
					FileType: m.FileType,
				},
			},
		}}, nil
	case legacy.SharedDirectoryCreateResponse:
		return []tdp.Message{&SharedDirectoryResponse{
			ErrorCode:    m.ErrCode,
			CompletionId: m.CompletionID,
			Operation: &tdpbv1.SharedDirectoryResponse_Create_{
				Create: &tdpbv1.SharedDirectoryResponse_Create{
					Fso: translateFSOToModern(m.Fso),
				},
			},
		}}, nil
	case legacy.SharedDirectoryDeleteRequest:
		return []tdp.Message{&SharedDirectoryRequest{
			DirectoryId:  m.DirectoryID,
			CompletionId: m.CompletionID,
			Operation: &tdpbv1.SharedDirectoryRequest_Delete_{
				Delete: &tdpbv1.SharedDirectoryRequest_Delete{
					Path: m.Path,
				},
			},
		}}, nil
	case legacy.SharedDirectoryDeleteResponse:
		return []tdp.Message{&SharedDirectoryResponse{
			ErrorCode:    m.ErrCode,
			CompletionId: m.CompletionID,
			Operation:    &tdpbv1.SharedDirectoryResponse_Delete_{},
		}}, nil
	case legacy.SharedDirectoryReadRequest:
		return []tdp.Message{&SharedDirectoryRequest{
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
	case legacy.SharedDirectoryReadResponse:
		return []tdp.Message{&SharedDirectoryResponse{
			CompletionId: m.CompletionID,
			ErrorCode:    m.ErrCode,
			Operation: &tdpbv1.SharedDirectoryResponse_Read_{
				Read: &tdpbv1.SharedDirectoryResponse_Read{
					Data: m.ReadData,
				},
			},
		}}, nil
	case legacy.SharedDirectoryWriteRequest:
		return []tdp.Message{&SharedDirectoryRequest{
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
	case legacy.SharedDirectoryWriteResponse:
		return []tdp.Message{&SharedDirectoryResponse{
			CompletionId: m.CompletionID,
			ErrorCode:    m.ErrCode,
			Operation: &tdpbv1.SharedDirectoryResponse_Write_{
				Write: &tdpbv1.SharedDirectoryResponse_Write{
					BytesWritten: m.BytesWritten,
				},
			},
		}}, nil
	case legacy.SharedDirectoryMoveRequest:
		return []tdp.Message{&SharedDirectoryRequest{
			CompletionId: m.CompletionID,
			DirectoryId:  m.DirectoryID,
			Operation: &tdpbv1.SharedDirectoryRequest_Move_{
				Move: &tdpbv1.SharedDirectoryRequest_Move{
					OriginalPath: m.OriginalPath,
					NewPath:      m.NewPath,
				},
			},
		}}, nil
	case legacy.SharedDirectoryMoveResponse:
		return []tdp.Message{&SharedDirectoryResponse{
			CompletionId: m.CompletionID,
			ErrorCode:    m.ErrCode,
			Operation:    &tdpbv1.SharedDirectoryResponse_Move_{},
		}}, nil
	case legacy.SharedDirectoryListRequest:
		return []tdp.Message{&SharedDirectoryRequest{
			CompletionId: m.CompletionID,
			Operation: &tdpbv1.SharedDirectoryRequest_List_{
				List: &tdpbv1.SharedDirectoryRequest_List{
					Path: m.Path,
				},
			},
		}}, nil
	case legacy.SharedDirectoryListResponse:
		return []tdp.Message{&SharedDirectoryResponse{
			CompletionId: m.CompletionID,
			ErrorCode:    m.ErrCode,
			Operation: &tdpbv1.SharedDirectoryResponse_List_{
				List: &tdpbv1.SharedDirectoryResponse_List{
					FsoList: slices.Map(m.FsoList, translateFSOToModern),
				},
			},
		}}, nil
	case legacy.SharedDirectoryTruncateRequest:
		return []tdp.Message{&SharedDirectoryRequest{
			DirectoryId:  m.DirectoryID,
			CompletionId: m.CompletionID,
			Operation: &tdpbv1.SharedDirectoryRequest_Truncate_{
				Truncate: &tdpbv1.SharedDirectoryRequest_Truncate{
					Path:      m.Path,
					EndOfFile: m.EndOfFile,
				},
			},
		}}, nil
	case legacy.SharedDirectoryTruncateResponse:
		return []tdp.Message{&SharedDirectoryResponse{
			CompletionId: m.CompletionID,
			ErrorCode:    m.ErrCode,
		}}, nil
	default:
		return nil, trace.Errorf("Could not translate to TDPB. Encountered unexpected message type %T", m)
	}
}
