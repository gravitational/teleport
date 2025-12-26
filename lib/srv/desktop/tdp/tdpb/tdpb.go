package tdpb

import (
	"bytes"
	"encoding/binary"
	"image/png"
	"io"
	"log/slog"

	"github.com/google/uuid"
	tdpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/desktop/v1"
	tdpbv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/desktop/v1"
	"github.com/gravitational/teleport/lib/srv/desktop/tdp"
	"github.com/gravitational/teleport/lib/utils/slices"
	"github.com/gravitational/trace"
	"google.golang.org/protobuf/proto"
)

const (
	// We can differentiate between TDP and TDPB messages on the wire
	// by inspecting the first byte received. A non-empty first byte
	// is presumed to be a TDP message, otherwise, TDPB.
	// Since the first byte of a TDPB message is the high 8 bits of its
	// length, we must take care not to allow TDPB messages that
	// meet or exceed length 2^24 (16MiB).
	// Once TDP is fully deprecated we can relax this constraint, although
	// it's unlikely we would ever want messages anywhere near this size.
	maxMessageLength = (1 << 24) - 1
	tdpbHeaderLength = 4 // sizeof(uint32)
)

func translateFso(fso *tdpb.FileSystemObject) tdp.FileSystemObject {
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

func translateFsoList(fso []*tdpb.FileSystemObject) []tdp.FileSystemObject {
	return slices.Map(fso, translateFso)
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
	return slices.Map(fso, translateFsoToModern)
}

func boolToButtonState(b bool) tdp.ButtonState {
	if b {
		return tdp.ButtonPressed
	}
	return tdp.ButtonNotPressed
}

// TranslateToLegacy converts a TDPB (Modern) message to one or more TDP (Legacy) messages.
func TranslateToLegacy(msg tdp.Message) ([]tdp.Message, error) {
	messages := make([]tdp.Message, 0, 1)
	switch m := msg.(type) {
	case *PNGFrame:
		messages = append(messages, tdp.PNG2Frame(m.Data))
	case *FastPathPDU:
		messages = append(messages, tdp.RDPFastPathPDU(m.Pdu))
	case *RDPResponsePDU:
		messages = append(messages, tdp.RDPResponsePDU(m.Response))
	case *SyncKeys:
		messages = append(messages, tdp.SyncKeys{
			ScrollLockState: boolToButtonState(m.ScrollLockPressed),
			NumLockState:    boolToButtonState(m.NumLockState),
			CapsLockState:   boolToButtonState(m.CapsLockState),
			KanaLockState:   boolToButtonState(m.KanaLockState),
		})
	case *MouseMove:
		messages = append(messages, tdp.MouseMove{X: m.X, Y: m.Y})
	case *MouseButton:
		button := tdp.MouseButtonType(m.Button - 1)
		state := tdp.ButtonNotPressed
		if m.Pressed {
			state = tdp.ButtonPressed
		}

		messages = append(messages, tdp.MouseButton{
			Button: button,
			State:  state,
		})
	case *KeyboardButton:
		state := tdp.ButtonNotPressed
		if m.Pressed {
			state = tdp.ButtonPressed
		}
		messages = append(messages, tdp.KeyboardButton{
			KeyCode: m.KeyCode,
			State:   state,
		})
	case *ClientScreenSpec:
		messages = append(messages, tdp.ClientScreenSpec{
			Width:  m.Width,
			Height: m.Height,
		})
	case *Alert:
		var severity tdp.Severity
		switch m.Severity {
		case tdpb.AlertSeverity_ALERT_SEVERITY_WARNING:
			severity = tdp.SeverityWarning
		case tdpb.AlertSeverity_ALERT_SEVERITY_ERROR:
			severity = tdp.SeverityError
		default:
			severity = tdp.SeverityInfo
		}
		messages = append(messages, tdp.Alert{
			Message:  m.Message,
			Severity: severity,
		})
	case *MouseWheel:
		messages = append(messages, tdp.MouseWheel{
			Axis:  tdp.MouseWheelAxis(m.Axis - 1),
			Delta: int16(m.Delta),
		})
	case *ClipboardData:
		messages = append(messages, tdp.ClipboardData(m.Data))
	case *SharedDirectoryAnnounce:
		messages = append(messages, tdp.SharedDirectoryAnnounce{
			DirectoryID: m.DirectoryId,
			Name:        m.Name,
		})
	case *SharedDirectoryAcknowledge:
		messages = append(messages, tdp.SharedDirectoryAcknowledge{
			DirectoryID: m.DirectoryId,
			ErrCode:     m.ErrorCode,
		})
	case *SharedDirectoryRequest:
		switch op := m.Operation.(type) {
		case *tdpb.SharedDirectoryRequest_Info_:
			messages = append(messages, tdp.SharedDirectoryInfoRequest{
				CompletionID: m.CompletionId,
				DirectoryID:  m.DirectoryId,
				Path:         op.Info.Path,
			})
		case *tdpb.SharedDirectoryRequest_Create_:
			messages = append(messages, tdp.SharedDirectoryCreateRequest{
				CompletionID: m.CompletionId,
				DirectoryID:  m.DirectoryId,
				FileType:     op.Create.FileType,
				Path:         op.Create.Path,
			})
		case *tdpb.SharedDirectoryRequest_Delete_:
			messages = append(messages, tdp.SharedDirectoryDeleteRequest{
				CompletionID: m.CompletionId,
				DirectoryID:  m.DirectoryId,
				Path:         op.Delete.Path,
			})
		case *tdpb.SharedDirectoryRequest_List_:
			messages = append(messages, tdp.SharedDirectoryListRequest{
				CompletionID: m.CompletionId,
				DirectoryID:  m.DirectoryId,
				Path:         op.List.Path,
			})
		case *tdpb.SharedDirectoryRequest_Read_:
			messages = append(messages, tdp.SharedDirectoryReadRequest{
				CompletionID: m.CompletionId,
				DirectoryID:  m.DirectoryId,
				Path:         op.Read.Path,
				Offset:       op.Read.Offset,
				Length:       op.Read.Length,
			})
		case *tdpb.SharedDirectoryRequest_Write_:
			messages = append(messages, tdp.SharedDirectoryWriteRequest{
				CompletionID:    m.CompletionId,
				DirectoryID:     m.DirectoryId,
				Path:            op.Write.Path,
				Offset:          op.Write.Offset,
				WriteDataLength: uint32(len(op.Write.Data)),
				WriteData:       op.Write.Data,
			})
		case *tdpb.SharedDirectoryRequest_Move_:
			messages = append(messages, tdp.SharedDirectoryMoveRequest{
				CompletionID: m.CompletionId,
				DirectoryID:  m.DirectoryId,
				NewPath:      op.Move.NewPath,
				OriginalPath: op.Move.OriginalPath,
			})
		case *tdpb.SharedDirectoryRequest_Truncate_:
			messages = append(messages, tdp.SharedDirectoryTruncateRequest{
				CompletionID: m.CompletionId,
				DirectoryID:  m.DirectoryId,
				Path:         op.Truncate.Path,
				EndOfFile:    op.Truncate.EndOfFile,
			})
		default:
			return nil, trace.Errorf("Unknown shared directory operation")
		}
	case *SharedDirectoryResponse:
		switch op := m.Operation.(type) {
		case *tdpb.SharedDirectoryResponse_Info_:
			messages = append(messages, tdp.SharedDirectoryInfoResponse{
				CompletionID: m.CompletionId,
				ErrCode:      m.ErrorCode,
				Fso:          translateFso(op.Info.Fso),
			})
		case *tdpb.SharedDirectoryResponse_Create_:
			messages = append(messages, tdp.SharedDirectoryCreateResponse{
				CompletionID: m.CompletionId,
				ErrCode:      m.ErrorCode,
				Fso:          translateFso(op.Create.Fso),
			})
		case *tdpb.SharedDirectoryResponse_Delete_:
			messages = append(messages, tdp.SharedDirectoryDeleteResponse{
				CompletionID: m.CompletionId,
				ErrCode:      m.ErrorCode,
			})
		case *tdpb.SharedDirectoryResponse_List_:
			messages = append(messages, tdp.SharedDirectoryListResponse{
				CompletionID: m.CompletionId,
				ErrCode:      m.ErrorCode,
				FsoList:      translateFsoList(op.List.FsoList),
			})
		case *tdpb.SharedDirectoryResponse_Read_:
			messages = append(messages, tdp.SharedDirectoryReadResponse{
				CompletionID:   m.CompletionId,
				ErrCode:        m.ErrorCode,
				ReadData:       op.Read.Data,
				ReadDataLength: uint32(len(op.Read.Data)),
			})
		case *tdpb.SharedDirectoryResponse_Write_:
			messages = append(messages, tdp.SharedDirectoryWriteResponse{
				CompletionID: m.CompletionId,
				ErrCode:      m.ErrorCode,
				BytesWritten: op.Write.BytesWritten,
			})
		case *tdpb.SharedDirectoryResponse_Move_:
			messages = append(messages, tdp.SharedDirectoryMoveResponse{
				CompletionID: m.CompletionId,
				ErrCode:      m.ErrorCode,
			})
		case *tdpb.SharedDirectoryResponse_Truncate_:
			messages = append(messages, tdp.SharedDirectoryTruncateResponse{
				CompletionID: m.CompletionId,
				ErrCode:      m.ErrorCode,
			})
		default:
			return nil, trace.Errorf("Unknown shared directory operation")
		}
	case *LatencyStats:
		messages = append(messages, tdp.LatencyStats{
			ClientLatency: m.ClientLatencyMs,
			ServerLatency: m.ServerLatencyMs,
		})
	case *Ping:
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
func TranslateToModern(msg tdp.Message) ([]tdp.Message, error) {
	messages := make([]tdp.Message, 0, 1)
	switch m := msg.(type) {
	case tdp.ClientScreenSpec:
		messages = append(messages, &ClientScreenSpec{
			Height: m.Height,
			Width:  m.Width,
		})
	case tdp.PNG2Frame:
		messages = append(messages, &PNGFrame{
			Coordinates: &tdpb.Rectangle{
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
		messages = append(messages, &PNGFrame{
			Coordinates: &tdpb.Rectangle{
				Top:    uint32(m.Img.Bounds().Min.Y),
				Left:   uint32(m.Img.Bounds().Min.X),
				Bottom: uint32(m.Img.Bounds().Max.Y),
				Right:  uint32(m.Img.Bounds().Max.X),
			},
			Data: buf.Bytes(),
		})
	case tdp.MouseMove:
		messages = append(messages, &MouseMove{
			X: m.X,
			Y: m.Y,
		})
	case tdp.MouseButton:
		messages = append(messages, &MouseButton{
			Pressed: m.State == tdp.ButtonPressed,
			Button:  tdpb.MouseButtonType(m.Button + 1),
		})
	case tdp.KeyboardButton:
		messages = append(messages, &KeyboardButton{
			KeyCode: m.KeyCode,
			Pressed: m.State == tdp.ButtonPressed,
		})
	case tdp.ClipboardData:
		messages = append(messages, &ClipboardData{
			Data: m,
		})
	case tdp.MouseWheel:
		messages = append(messages, &MouseWheel{
			Axis:  tdpb.MouseWheelAxis(m.Axis + 1),
			Delta: uint32(m.Delta),
		})
	case tdp.Error:
		messages = append(messages, &Alert{
			Message:  m.Message,
			Severity: tdpb.AlertSeverity_ALERT_SEVERITY_ERROR,
		})
	case tdp.Alert:
		var severity tdpb.AlertSeverity
		switch m.Severity {
		case tdp.SeverityError:
			severity = tdpb.AlertSeverity_ALERT_SEVERITY_ERROR
		case tdp.SeverityWarning:
			severity = tdpb.AlertSeverity_ALERT_SEVERITY_WARNING
		default:
			severity = tdpb.AlertSeverity_ALERT_SEVERITY_INFO
		}
		messages = append(messages, &Alert{
			Message:  m.Message,
			Severity: severity,
		})
	case tdp.RDPFastPathPDU:
		messages = append(messages, &FastPathPDU{
			Pdu: m,
		})
	case tdp.RDPResponsePDU:
		messages = append(messages, &RDPResponsePDU{
			Response: m,
		})
	case tdp.ConnectionActivated:
		// Legacy TDP servers send this message once at the start
		// of the connection.
		messages = append(messages, &ServerHello{
			ActivationSpec: &tdpb.ConnectionActivated{
				IoChannelId:   uint32(m.IOChannelID),
				UserChannelId: uint32(m.UserChannelID),
				ScreenWidth:   uint32(m.ScreenWidth),
				ScreenHeight:  uint32(m.ScreenHeight),
			},
			// Assume all legacy TDP servers support clipboard sharing
			ClipboardEnabled: true,
		})
	case tdp.SyncKeys:
		messages = append(messages, &SyncKeys{
			ScrollLockPressed: m.ScrollLockState == tdp.ButtonPressed,
			NumLockState:      m.NumLockState == tdp.ButtonPressed,
			CapsLockState:     m.CapsLockState == tdp.ButtonPressed,
			KanaLockState:     m.KanaLockState == tdp.ButtonPressed,
		})
	case tdp.LatencyStats:
		messages = append(messages, &LatencyStats{
			ClientLatencyMs: m.ClientLatency,
			ServerLatencyMs: m.ServerLatency,
		})
	case tdp.Ping:
		messages = append(messages, &Ping{
			Uuid: m.UUID[:],
		})
	case tdp.SharedDirectoryAnnounce:
		messages = append(messages, &SharedDirectoryAnnounce{
			DirectoryId: m.DirectoryID,
			Name:        m.Name,
		})
	case tdp.SharedDirectoryAcknowledge:
		messages = append(messages, &SharedDirectoryAcknowledge{
			DirectoryId: m.DirectoryID,
			ErrorCode:   m.ErrCode,
		})
	case tdp.SharedDirectoryInfoRequest:
		messages = append(messages, &SharedDirectoryRequest{
			DirectoryId:  m.DirectoryID,
			CompletionId: m.CompletionID,
			Operation: &tdpbv1.SharedDirectoryRequest_Info_{
				Info: &tdpbv1.SharedDirectoryRequest_Info{
					Path: m.Path,
				},
			},
		})
	case tdp.SharedDirectoryInfoResponse:
		messages = append(messages, &SharedDirectoryResponse{
			ErrorCode:    m.ErrCode,
			CompletionId: m.CompletionID,
			Operation: &tdpbv1.SharedDirectoryResponse_Info_{
				Info: &tdpbv1.SharedDirectoryResponse_Info{
					Fso: translateFsoToModern(m.Fso),
				},
			},
		})
	case tdp.SharedDirectoryCreateRequest:
		messages = append(messages, &SharedDirectoryRequest{
			DirectoryId:  m.DirectoryID,
			CompletionId: m.CompletionID,
			Operation: &tdpbv1.SharedDirectoryRequest_Create_{
				Create: &tdpbv1.SharedDirectoryRequest_Create{
					Path: m.Path,
				},
			},
		})
	case tdp.SharedDirectoryCreateResponse:
		messages = append(messages, &SharedDirectoryResponse{
			ErrorCode:    m.ErrCode,
			CompletionId: m.CompletionID,
			Operation: &tdpbv1.SharedDirectoryResponse_Create_{
				Create: &tdpbv1.SharedDirectoryResponse_Create{
					Fso: translateFsoToModern(m.Fso),
				},
			},
		})
	case tdp.SharedDirectoryDeleteRequest:
		messages = append(messages, &SharedDirectoryRequest{
			DirectoryId:  m.DirectoryID,
			CompletionId: m.CompletionID,
			Operation: &tdpbv1.SharedDirectoryRequest_Delete_{
				Delete: &tdpbv1.SharedDirectoryRequest_Delete{
					Path: m.Path,
				},
			},
		})
	case tdp.SharedDirectoryDeleteResponse:
		messages = append(messages, &SharedDirectoryResponse{
			ErrorCode:    m.ErrCode,
			CompletionId: m.CompletionID,
			Operation:    &tdpbv1.SharedDirectoryResponse_Delete_{},
		})
	case tdp.SharedDirectoryReadRequest:
		messages = append(messages, &SharedDirectoryRequest{
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
		messages = append(messages, &SharedDirectoryResponse{
			CompletionId: m.CompletionID,
			ErrorCode:    m.ErrCode,
			Operation: &tdpbv1.SharedDirectoryResponse_Read_{
				Read: &tdpbv1.SharedDirectoryResponse_Read{
					Data: m.ReadData,
				},
			},
		})
	case tdp.SharedDirectoryWriteRequest:
		messages = append(messages, &SharedDirectoryRequest{
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
		messages = append(messages, &SharedDirectoryResponse{
			CompletionId: m.CompletionID,
			ErrorCode:    m.ErrCode,
			Operation: &tdpbv1.SharedDirectoryResponse_Write_{
				Write: &tdpbv1.SharedDirectoryResponse_Write{
					BytesWritten: m.BytesWritten,
				},
			},
		})
	case tdp.SharedDirectoryMoveRequest:
		messages = append(messages, &SharedDirectoryRequest{
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
		messages = append(messages, &SharedDirectoryResponse{
			CompletionId: m.CompletionID,
			ErrorCode:    m.ErrCode,
			Operation:    &tdpbv1.SharedDirectoryResponse_Move_{},
		})
	case tdp.SharedDirectoryListRequest:
		messages = append(messages, &SharedDirectoryRequest{
			CompletionId: m.CompletionID,
			Operation: &tdpbv1.SharedDirectoryRequest_List_{
				List: &tdpbv1.SharedDirectoryRequest_List{
					Path: m.Path,
				},
			},
		})
	case tdp.SharedDirectoryListResponse:
		messages = append(messages, &SharedDirectoryResponse{
			CompletionId: m.CompletionID,
			ErrorCode:    m.ErrCode,
			Operation: &tdpbv1.SharedDirectoryResponse_List_{
				List: &tdpbv1.SharedDirectoryResponse_List{
					FsoList: translateFsoListToModern(m.FsoList),
				},
			},
		})
	case tdp.SharedDirectoryTruncateRequest:
		messages = append(messages, &SharedDirectoryRequest{
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
		messages = append(messages, &SharedDirectoryResponse{
			CompletionId: m.CompletionID,
			ErrorCode:    m.ErrCode,
		})
	default:
		return nil, trace.Errorf("Could not translate to TDPB. Encountered unexpected message type %T", m)
	}

	wrapped := []tdp.Message{}
	for _, msg := range messages {
		wrapped = append(wrapped, msg)
	}
	return wrapped, nil
}

// ClientHello message.
type ClientHello tdpb.ClientHello

// Encode encodes a ClientHello message.
func (c *ClientHello) Encode() ([]byte, error) {
	return marshalWithHeader(&tdpb.Envelope{
		Payload: &tdpbv1.Envelope_ClientHello{
			ClientHello: (*tdpbv1.ClientHello)(c),
		},
	})
}

// ServerHello message.
type ServerHello tdpb.ServerHello

// Encode encodes a ServerHello message.
func (S *ServerHello) Encode() ([]byte, error) {
	return marshalWithHeader(&tdpb.Envelope{
		Payload: &tdpbv1.Envelope_ServerHello{
			ServerHello: (*tdpbv1.ServerHello)(S),
		},
	})
}

// PNGFrame message.
type PNGFrame tdpb.PNGFrame

// Encode encodes a PNGFrame message.
func (p *PNGFrame) Encode() ([]byte, error) {
	return marshalWithHeader(&tdpb.Envelope{
		Payload: &tdpbv1.Envelope_PngFrame{
			PngFrame: (*tdpbv1.PNGFrame)(p),
		},
	})
}

// FastPathPDU message.
type FastPathPDU tdpb.FastPathPDU

// Encode encodes a FastPathPDU message.
func (f *FastPathPDU) Encode() ([]byte, error) {
	return marshalWithHeader(&tdpb.Envelope{
		Payload: &tdpbv1.Envelope_FastPathPdu{
			FastPathPdu: (*tdpbv1.FastPathPDU)(f),
		},
	})
}

// RDPResponsePDU message.
type RDPResponsePDU tdpb.RDPResponsePDU

// Encode encodes a RDPResponsePDU message.
func (f *RDPResponsePDU) Encode() ([]byte, error) {
	return marshalWithHeader(&tdpb.Envelope{
		Payload: &tdpbv1.Envelope_RdpResponsePdu{
			RdpResponsePdu: (*tdpbv1.RDPResponsePDU)(f),
		},
	})
}

// SyncKeys message.
type SyncKeys tdpb.SyncKeys

// Encode encodes a SyncKeys message.
func (s *SyncKeys) Encode() ([]byte, error) {
	return marshalWithHeader(&tdpb.Envelope{
		Payload: &tdpbv1.Envelope_SyncKeys{
			SyncKeys: (*tdpbv1.SyncKeys)(s),
		},
	})
}

// MouseMove message.
type MouseMove tdpb.MouseMove

// Encode encodes a MouseMove message.
func (m *MouseMove) Encode() ([]byte, error) {
	return marshalWithHeader(&tdpb.Envelope{
		Payload: &tdpbv1.Envelope_MouseMove{
			MouseMove: (*tdpbv1.MouseMove)(m),
		},
	})
}

// MouseButton message.
type MouseButton tdpb.MouseButton

// Encode encodes a MouseButton message.
func (m *MouseButton) Encode() ([]byte, error) {
	return marshalWithHeader(&tdpb.Envelope{
		Payload: &tdpbv1.Envelope_MouseButton{
			MouseButton: (*tdpbv1.MouseButton)(m),
		},
	})
}

// KeyboardButton message.
type KeyboardButton tdpb.KeyboardButton

// Encode encodes a KeyboardButton message.
func (k *KeyboardButton) Encode() ([]byte, error) {
	return marshalWithHeader(&tdpb.Envelope{
		Payload: &tdpbv1.Envelope_KeyboardButton{
			KeyboardButton: (*tdpbv1.KeyboardButton)(k),
		},
	})
}

// ClientScreenSpec message.
type ClientScreenSpec tdpb.ClientScreenSpec

// Encode encodes a ClientScreenSpec message.
func (c *ClientScreenSpec) Encode() ([]byte, error) {
	return marshalWithHeader(&tdpb.Envelope{
		Payload: &tdpbv1.Envelope_ClientScreenSpec{
			ClientScreenSpec: (*tdpbv1.ClientScreenSpec)(c),
		},
	})
}

// Alert message.
type Alert tdpb.Alert

// Encode encodes a Alert message.
func (a *Alert) Encode() ([]byte, error) {
	return marshalWithHeader(&tdpb.Envelope{
		Payload: &tdpbv1.Envelope_Alert{
			Alert: (*tdpbv1.Alert)(a),
		},
	})
}

// MouseWheel message.
type MouseWheel tdpb.MouseWheel

// Encode encodes a MouseWheel message.
func (m *MouseWheel) Encode() ([]byte, error) {
	return marshalWithHeader(&tdpb.Envelope{
		Payload: &tdpbv1.Envelope_MouseWheel{
			MouseWheel: (*tdpbv1.MouseWheel)(m),
		},
	})
}

// ClipboardData message.
type ClipboardData tdpb.ClipboardData

// Encode encodes a ClipboardData message.
func (c *ClipboardData) Encode() ([]byte, error) {
	return marshalWithHeader(&tdpb.Envelope{
		Payload: &tdpbv1.Envelope_ClipboardData{
			ClipboardData: (*tdpbv1.ClipboardData)(c),
		},
	})
}

// MFA message.
type MFA tdpb.MFA

// Encode encodes a MFA message.
func (m *MFA) Encode() ([]byte, error) {
	return marshalWithHeader(&tdpb.Envelope{
		Payload: &tdpbv1.Envelope_Mfa{
			Mfa: (*tdpbv1.MFA)(m),
		},
	})
}

// SharedDirectoryAnnounce message.
type SharedDirectoryAnnounce tdpb.SharedDirectoryAnnounce

// Encode encodes a SharedDirectoryAnnounce message.
func (s *SharedDirectoryAnnounce) Encode() ([]byte, error) {
	return marshalWithHeader(&tdpb.Envelope{
		Payload: &tdpbv1.Envelope_SharedDirectoryAnnounce{
			SharedDirectoryAnnounce: (*tdpbv1.SharedDirectoryAnnounce)(s),
		},
	})
}

// SharedDirectoryAcknowledge message.
type SharedDirectoryAcknowledge tdpb.SharedDirectoryAcknowledge

// Encode encodes a SharedDirectoryAcknowledge message.
func (s *SharedDirectoryAcknowledge) Encode() ([]byte, error) {
	return marshalWithHeader(&tdpb.Envelope{
		Payload: &tdpbv1.Envelope_SharedDirectoryAcknowledge{
			SharedDirectoryAcknowledge: (*tdpbv1.SharedDirectoryAcknowledge)(s),
		},
	})
}

// SharedDirectoryRequest message.
type SharedDirectoryRequest tdpb.SharedDirectoryRequest

// Encode encodes a SharedDirectoryRequest message.
func (s *SharedDirectoryRequest) Encode() ([]byte, error) {
	return marshalWithHeader(&tdpb.Envelope{
		Payload: &tdpbv1.Envelope_SharedDirectoryRequest{
			SharedDirectoryRequest: (*tdpbv1.SharedDirectoryRequest)(s),
		},
	})
}

// SharedDirectoryResponse message.
type SharedDirectoryResponse tdpb.SharedDirectoryResponse

// Encode encodes a SharedDirectoryResponse message.
func (s *SharedDirectoryResponse) Encode() ([]byte, error) {
	return marshalWithHeader(&tdpb.Envelope{
		Payload: &tdpbv1.Envelope_SharedDirectoryResponse{
			SharedDirectoryResponse: (*tdpbv1.SharedDirectoryResponse)(s),
		},
	})
}

// LatencyStats message.
type LatencyStats tdpb.LatencyStats

// Encode encodes a LatencyStats message.
func (l *LatencyStats) Encode() ([]byte, error) {
	return marshalWithHeader(&tdpb.Envelope{
		Payload: &tdpbv1.Envelope_LatencyStats{
			LatencyStats: (*tdpbv1.LatencyStats)(l),
		},
	})
}

// Ping message.
type Ping tdpb.Ping

// Encodes a ping message.
func (p *Ping) Encode() ([]byte, error) {
	return marshalWithHeader(&tdpb.Envelope{
		Payload: &tdpbv1.Envelope_Ping{
			Ping: (*tdpbv1.Ping)(p),
		},
	})
}

func marshalWithHeader(msg proto.Message) ([]byte, error) {
	data, err := proto.Marshal(msg)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if len(data) > maxMessageLength {
		return nil, trace.Errorf("TDPB message too large. %d bytes exceeds maximum: %d", len(data), maxMessageLength)
	}

	header := make([]byte, len(data)+tdpbHeaderLength)
	binary.BigEndian.PutUint32(header[:tdpbHeaderLength], uint32(len(data)))
	copy(header[tdpbHeaderLength:], data)

	return header, nil
}

// Decode reads a TDPB message from a reader.
// Returns ErrEmptyMessage if a valid TDPB Envelope was received, but no
// wrapped message was found.
func Decode(rdr io.Reader) (tdp.Message, error) {
	// Read header
	header := make([]byte, tdpbHeaderLength)
	_, err := io.ReadFull(rdr, header)
	if err != nil {
		return nil, trace.WrapWithMessage(err, "error reading next TDPB message header")
	}

	messageLength := binary.BigEndian.Uint32(header)

	if messageLength >= maxMessageLength {
		return nil, trace.Errorf("message of length '%d' exceeds maximum allowed length '%d'", messageLength, maxMessageLength)
	}

	message := make([]byte, messageLength)
	_, err = io.ReadFull(rdr, message)
	if err != nil {
		return nil, trace.WrapWithMessage(err, "error reading TDPB message body")
	}

	env := &tdpb.Envelope{}
	if err = proto.Unmarshal(message, env); err != nil {
		return nil, trace.WrapWithMessage(err, "error unmarshalling TDPB message envelope")
	}

	if msg := messageFromEnvelope(env); msg != nil {
		return msg, nil
	}

	// Allow the caller to distinguish unmarshal errors (likely considered fatal)
	// from an "empty" message, which could simply mean that we've received
	// a new (unsupported) message from a newer implementation.
	return nil, trace.Wrap(tdp.ErrEmptyMessage)
}

func messageFromEnvelope(e *tdpb.Envelope) tdp.Message {
	switch m := e.Payload.(type) {
	case *tdpb.Envelope_ClientHello:
		return (*ClientHello)(m.ClientHello)
	case *tdpb.Envelope_ServerHello:
		return (*ServerHello)(m.ServerHello)
	case *tdpb.Envelope_PngFrame:
		return (*PNGFrame)(m.PngFrame)
	case *tdpb.Envelope_FastPathPdu:
		return (*FastPathPDU)(m.FastPathPdu)
	case *tdpb.Envelope_RdpResponsePdu:
		return (*RDPResponsePDU)(m.RdpResponsePdu)
	case *tdpb.Envelope_SyncKeys:
		return (*SyncKeys)(m.SyncKeys)
	case *tdpb.Envelope_MouseMove:
		return (*MouseMove)(m.MouseMove)
	case *tdpb.Envelope_MouseButton:
		return (*MouseButton)(m.MouseButton)
	case *tdpb.Envelope_KeyboardButton:
		return (*KeyboardButton)(m.KeyboardButton)
	case *tdpb.Envelope_ClientScreenSpec:
		return (*ClientScreenSpec)(m.ClientScreenSpec)
	case *tdpb.Envelope_Alert:
		return (*Alert)(m.Alert)
	case *tdpb.Envelope_MouseWheel:
		return (*MouseWheel)(m.MouseWheel)
	case *tdpb.Envelope_ClipboardData:
		return (*ClipboardData)(m.ClipboardData)
	case *tdpb.Envelope_Mfa:
		return (*MFA)(m.Mfa)
	case *tdpb.Envelope_SharedDirectoryAnnounce:
		return (*SharedDirectoryAnnounce)(m.SharedDirectoryAnnounce)
	case *tdpb.Envelope_SharedDirectoryAcknowledge:
		return (*SharedDirectoryAcknowledge)(m.SharedDirectoryAcknowledge)
	case *tdpb.Envelope_SharedDirectoryRequest:
		return (*SharedDirectoryRequest)(m.SharedDirectoryRequest)
	case *tdpb.Envelope_SharedDirectoryResponse:
		return (*SharedDirectoryResponse)(m.SharedDirectoryResponse)
	case *tdpb.Envelope_LatencyStats:
		return (*LatencyStats)(m.LatencyStats)
	case *tdpb.Envelope_Ping:
		return (*Ping)(m.Ping)
	default:
		return nil
	}
}

// EncodeTo calls 'Encode' on the given message and writes it to 'w'.
func EncodeTo(w io.Writer, msg tdp.Message) error {
	data, err := msg.Encode()
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = w.Write(data)
	return trace.Wrap(err)
}
