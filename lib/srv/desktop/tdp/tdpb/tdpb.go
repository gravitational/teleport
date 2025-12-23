package tdpb

import (
	"bytes"
	"encoding/binary"
	"errors"
	"image/png"
	"io"
	"log/slog"

	"github.com/google/uuid"
	clientProto "github.com/gravitational/teleport/api/client/proto"
	tdpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/desktop/v1"
	tdpbv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/desktop/v1"
	mfav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v1"
	"github.com/gravitational/teleport/api/mfa"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/srv/desktop/tdp"
	"github.com/gravitational/teleport/lib/utils/slices"
	"github.com/gravitational/trace"
	"google.golang.org/protobuf/proto"
)

const (

	// We differentiate between TDP and TDPB messages on the wire
	// by inspecting the first byte received. A non-empty first byte
	// is presumed to be a TDP message, otherwise, TDPB.
	// Since the first byte of a TDPB message is the high 8 bits of its
	// length, we must take care not to allow TDPB messages that
	// meet or exceed length 2^24 (16MiB).
	maxMessageLength = (1 << 24) - 1
	tdpbHeaderLength = 4
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

// Converts a TDPB (Modern) message to one or more TDP (Legacy) messages
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

// Converts a TDP (Legacy) message to one or more TDPB (Modern) messages
func TranslateToModern(msg tdp.Message) ([]tdp.Message, error) {
	slog.Warn("translating TDP to TDPB")
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

type ClientHello tdpb.ClientHello

func (c *ClientHello) Encode() ([]byte, error) {
	return marshalWithHeader(&tdpb.Envelope{
		Payload: &tdpbv1.Envelope_ClientHello{
			ClientHello: (*tdpbv1.ClientHello)(c),
		},
	})
}

type ServerHello tdpb.ServerHello

func (c *ServerHello) Encode() ([]byte, error) {
	return marshalWithHeader(&tdpb.Envelope{
		Payload: &tdpbv1.Envelope_ServerHello{
			ServerHello: (*tdpbv1.ServerHello)(c),
		},
	})
}

type PNGFrame tdpb.PNGFrame

func (c *PNGFrame) Encode() ([]byte, error) {
	return marshalWithHeader(&tdpb.Envelope{
		Payload: &tdpbv1.Envelope_PngFrame{
			PngFrame: (*tdpbv1.PNGFrame)(c),
		},
	})
}

type FastPathPDU tdpb.FastPathPDU

func (c *FastPathPDU) Encode() ([]byte, error) {
	return marshalWithHeader(&tdpb.Envelope{
		Payload: &tdpbv1.Envelope_FastPathPdu{
			FastPathPdu: (*tdpbv1.FastPathPDU)(c),
		},
	})
}

type RDPResponsePDU tdpb.RDPResponsePDU

func (c *RDPResponsePDU) Encode() ([]byte, error) {
	return marshalWithHeader(&tdpb.Envelope{
		Payload: &tdpbv1.Envelope_RdpResponsePdu{
			RdpResponsePdu: (*tdpbv1.RDPResponsePDU)(c),
		},
	})
}

type SyncKeys tdpb.SyncKeys

func (c *SyncKeys) Encode() ([]byte, error) {
	return marshalWithHeader(&tdpb.Envelope{
		Payload: &tdpbv1.Envelope_SyncKeys{
			SyncKeys: (*tdpbv1.SyncKeys)(c),
		},
	})
}

type MouseMove tdpb.MouseMove

func (c *MouseMove) Encode() ([]byte, error) {
	return marshalWithHeader(&tdpb.Envelope{
		Payload: &tdpbv1.Envelope_MouseMove{
			MouseMove: (*tdpbv1.MouseMove)(c),
		},
	})
}

type MouseButton tdpb.MouseButton

func (c *MouseButton) Encode() ([]byte, error) {
	return marshalWithHeader(&tdpb.Envelope{
		Payload: &tdpbv1.Envelope_MouseButton{
			MouseButton: (*tdpbv1.MouseButton)(c),
		},
	})
}

type KeyboardButton tdpb.KeyboardButton

func (c *KeyboardButton) Encode() ([]byte, error) {
	return marshalWithHeader(&tdpb.Envelope{
		Payload: &tdpbv1.Envelope_KeyboardButton{
			KeyboardButton: (*tdpbv1.KeyboardButton)(c),
		},
	})
}

type ClientScreenSpec tdpb.ClientScreenSpec

func (c *ClientScreenSpec) Encode() ([]byte, error) {
	return marshalWithHeader(&tdpb.Envelope{
		Payload: &tdpbv1.Envelope_ClientScreenSpec{
			ClientScreenSpec: (*tdpbv1.ClientScreenSpec)(c),
		},
	})
}

type Alert tdpb.Alert

func (c *Alert) Encode() ([]byte, error) {
	return marshalWithHeader(&tdpb.Envelope{
		Payload: &tdpbv1.Envelope_Alert{
			Alert: (*tdpbv1.Alert)(c),
		},
	})
}

type MouseWheel tdpb.MouseWheel

func (c *MouseWheel) Encode() ([]byte, error) {
	return marshalWithHeader(&tdpb.Envelope{
		Payload: &tdpbv1.Envelope_MouseWheel{
			MouseWheel: (*tdpbv1.MouseWheel)(c),
		},
	})
}

type ClipboardData tdpb.ClipboardData

func (c *ClipboardData) Encode() ([]byte, error) {
	return marshalWithHeader(&tdpb.Envelope{
		Payload: &tdpbv1.Envelope_ClipboardData{
			ClipboardData: (*tdpbv1.ClipboardData)(c),
		},
	})
}

type MFA tdpb.MFA

func (c *MFA) Encode() ([]byte, error) {
	return marshalWithHeader(&tdpb.Envelope{
		Payload: &tdpbv1.Envelope_Mfa{
			Mfa: (*tdpbv1.MFA)(c),
		},
	})
}

type SharedDirectoryAnnounce tdpb.SharedDirectoryAnnounce

func (c *SharedDirectoryAnnounce) Encode() ([]byte, error) {
	return marshalWithHeader(&tdpb.Envelope{
		Payload: &tdpbv1.Envelope_SharedDirectoryAnnounce{
			SharedDirectoryAnnounce: (*tdpbv1.SharedDirectoryAnnounce)(c),
		},
	})
}

type SharedDirectoryAcknowledge tdpb.SharedDirectoryAcknowledge

func (c *SharedDirectoryAcknowledge) Encode() ([]byte, error) {
	return marshalWithHeader(&tdpb.Envelope{
		Payload: &tdpbv1.Envelope_SharedDirectoryAcknowledge{
			SharedDirectoryAcknowledge: (*tdpbv1.SharedDirectoryAcknowledge)(c),
		},
	})
}

type SharedDirectoryRequest tdpb.SharedDirectoryRequest

func (c *SharedDirectoryRequest) Encode() ([]byte, error) {
	return marshalWithHeader(&tdpb.Envelope{
		Payload: &tdpbv1.Envelope_SharedDirectoryRequest{
			SharedDirectoryRequest: (*tdpbv1.SharedDirectoryRequest)(c),
		},
	})
}

type SharedDirectoryResponse tdpb.SharedDirectoryResponse

func (c *SharedDirectoryResponse) Encode() ([]byte, error) {
	return marshalWithHeader(&tdpb.Envelope{
		Payload: &tdpbv1.Envelope_SharedDirectoryResponse{
			SharedDirectoryResponse: (*tdpbv1.SharedDirectoryResponse)(c),
		},
	})
}

type LatencyStats tdpb.LatencyStats

func (c *LatencyStats) Encode() ([]byte, error) {
	return marshalWithHeader(&tdpb.Envelope{
		Payload: &tdpbv1.Envelope_LatencyStats{
			LatencyStats: (*tdpbv1.LatencyStats)(c),
		},
	})
}

type Ping tdpb.Ping

func (c *Ping) Encode() ([]byte, error) {
	return marshalWithHeader(&tdpb.Envelope{
		Payload: &tdpbv1.Envelope_Ping{
			Ping: (*tdpbv1.Ping)(c),
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

	header := make([]byte, len(data)+4)
	binary.BigEndian.PutUint32(header[:4], uint32(len(data)))
	copy(header[4:], data)

	return header, nil
}

var ErrEmptyMessage = errors.New("decoded empty TDPB envelope")

func Decode(rdr io.Reader) (tdp.Message, error) {
	// Read header
	header := [4]byte{}
	_, err := io.ReadFull(rdr, header[:])
	if err != nil {
		return nil, trace.Wrap(err)
	}

	messageLength := binary.BigEndian.Uint32(header[:])

	if messageLength >= maxMessageLength {
		return nil, trace.Errorf("message of length '%d' exceeds maximum allowed length '%d'", messageLength, maxMessageLength)
	}

	message := make([]byte, messageLength)
	_, err = io.ReadFull(rdr, message)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	env := &tdpb.Envelope{}
	if err = proto.Unmarshal(message, env); err != nil {
		return nil, trace.Wrap(err)
	}

	if msg := messageFromEnvelope(env); msg != nil {
		return msg, nil
	}

	return nil, trace.Wrap(ErrEmptyMessage)
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

// Handle TDPB MFA ceremony
func NewTDPBMFAPrompt(rw tdp.MessageReadWriter, withheld *[]tdp.Message) func(string) mfa.PromptFunc {
	return func(channelID string) mfa.PromptFunc {
		convert := func(challenge *clientProto.MFAAuthenticateChallenge) (tdp.Message, error) {
			if challenge == nil {
				return nil, errors.New("empty MFA challenge")
			}

			mfaMsg := &MFA{
				ChannelId: channelID,
			}

			if challenge.WebauthnChallenge != nil {
				mfaMsg.Challenge = &mfav1.AuthenticateChallenge{
					WebauthnChallenge: challenge.WebauthnChallenge,
				}
			}

			if challenge.SSOChallenge != nil {
				mfaMsg.Challenge = &mfav1.AuthenticateChallenge{
					SsoChallenge: &mfav1.SSOChallenge{
						RequestId:   challenge.SSOChallenge.RequestId,
						RedirectUrl: challenge.SSOChallenge.RedirectUrl,
						Device:      challenge.SSOChallenge.Device,
					},
				}
			}

			if challenge.WebauthnChallenge == nil && challenge.SSOChallenge == nil && challenge.TOTP == nil {
				return nil, trace.Wrap(authclient.ErrNoMFADevices)
			}

			return mfaMsg, nil
		}

		isResponse := func(msg tdp.Message) (*clientProto.MFAAuthenticateResponse, error) {
			mfaMsg, ok := msg.(*MFA)
			if !ok {
				return nil, tdp.ErrUnexpectedMessageType
			}

			if mfaMsg.AuthenticationResponse == nil {
				return nil, trace.Errorf("MFA response is empty")
			}

			switch response := mfaMsg.AuthenticationResponse.Response.(type) {
			case *mfav1.AuthenticateResponse_Sso:
				return &clientProto.MFAAuthenticateResponse{
					Response: &clientProto.MFAAuthenticateResponse_SSO{
						SSO: &clientProto.SSOResponse{
							RequestId: response.Sso.RequestId,
							Token:     response.Sso.Token,
						},
					},
				}, nil
			case *mfav1.AuthenticateResponse_Webauthn:
				return &clientProto.MFAAuthenticateResponse{
					Response: &clientProto.MFAAuthenticateResponse_Webauthn{
						Webauthn: response.Webauthn,
					},
				}, nil
			default:
				return nil, trace.Errorf("Unexpected MFA response type %T", mfaMsg.AuthenticationResponse)
			}
		}

		return tdp.NewMfaPrompt(rw, isResponse, convert, withheld)
	}
}

func EncodeTo(w io.Writer, msg tdp.Message) error {
	data, err := msg.Encode()
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = w.Write(data)
	return trace.Wrap(err)
}
