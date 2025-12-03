package tdp

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"log/slog"
	"reflect"

	"github.com/google/uuid"
	tdpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/desktop/v1"
	"github.com/gravitational/trace"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
)

const (
	MAX_MESSAGE_LENGTH        = 1024 * 1024 /* 1MiB */
	DEFAULT_MESSAGE_RECV_SIZE = 4096
	TDPB_HEADER_LENGTH        = 8
)

var (
	ErrInvalidMessage        = errors.New("unknown or invalid TDPB message")
	ErrUnexpectedMessageType = errors.New("unexpected message type")
)

var globalDecoder *messageDecoder

type messageDecoder struct {
	key map[tdpb.MessageType]protoreflect.MessageType
}

func init() {
	var err error
	globalDecoder, err = newMessageDecoder()
	if err != nil {
		panic(err)
	}
}

// AsTDPB is a convenience for working with TDP messages.
// It is not particularly performant.
func AsTDPB(msg Message, p proto.Message) error {
	source, err := ToTDPBProto(msg)
	if err != nil {
		return trace.Wrap(errors.Join(ErrInvalidMessage, err))
	}

	if p.ProtoReflect().Type() == source.ProtoReflect().Type() {
		reflect.ValueOf(p).Elem().Set(reflect.ValueOf(source).Elem())
		return nil
	}

	// No errors parsing the message, but it doesn't match
	// what the caller is expecting
	return ErrUnexpectedMessageType
}

// ToTDPBProto attempts to extract the underlying proto.Message
// representation of a TDPB message.
func ToTDPBProto(msg Message) (proto.Message, error) {
	var m *TdpbMessage
	var ok bool
	if m, ok = msg.(*TdpbMessage); !ok {
		return nil, ErrInvalidMessage
	}

	source, err := m.Proto()
	if err != nil {
		return nil, trace.Wrap(errors.Join(ErrInvalidMessage, err))
	}
	return source, nil
}

// DecodeTDPB decodes a TDPB message
func DecodeTDPB(rdr io.Reader) (TdpbMessage, error) {
	return globalDecoder.Decode(rdr)
}

func (m *messageDecoder) Decode(rdr io.Reader) (TdpbMessage, error) {
	mType, mBytes, err := readTDPBMessage(rdr)
	if err != nil {
		return TdpbMessage{}, err
	}

	return TdpbMessage{messageType: mType, data: mBytes}, err
}

func (m *messageDecoder) decode(mType tdpb.MessageType, data []byte) (proto.Message, error) {
	if protoType, ok := m.key[mType]; ok {
		msg := protoType.New().Interface()
		return msg, proto.Unmarshal(data, msg)
	}
	return nil, trace.Errorf("unknown TDPB message type")
}

func newMessageDecoder() (*messageDecoder, error) {
	// Maintain a mapping of our message type enum to
	// the protoreflect.MessageType corresponding to that message type
	key := map[tdpb.MessageType]protoreflect.MessageType{}

	descriptors := tdpb.File_teleport_desktop_v1_tdpb_proto.Messages()
	for i := 0; i < descriptors.Len(); i++ {
		desc := descriptors.Get(i)
		mtype, err := protoregistry.GlobalTypes.FindMessageByName(desc.FullName())
		if err != nil {
			return &messageDecoder{}, err
		}

		options := desc.Options().(*descriptorpb.MessageOptions)
		typeOption := proto.GetExtension(options, tdpb.E_TdpTypeOption).(tdpb.MessageType)
		if typeOption == tdpb.MessageType_MESSAGE_TYPE_UNSPECIFIED {
			// Not all messages are intended for transmission, and so they
			// don't have a message type option. Just ignore them.
			continue
		}
		key[typeOption] = mtype
	}

	return &messageDecoder{
		key: key,
	}, nil
}

func readTDPBMessage(in io.Reader) (tdpb.MessageType, []byte, error) {
	//
	msgBuffer := bytes.NewBuffer(make([]byte, 0, 1024))

	_, err := io.CopyN(msgBuffer, in, TDPB_HEADER_LENGTH)
	if err != nil {
		return 0, nil, err
	}

	mType := binary.BigEndian.Uint32(msgBuffer.Bytes()[:4])
	mLength := binary.BigEndian.Uint32(msgBuffer.Bytes()[4:])

	_, err = io.CopyN(msgBuffer, in, int64(mLength))
	return tdpb.MessageType(mType), msgBuffer.Bytes(), err
}

// Scenarios for creating a tdpbMessage
// 1. Have a "raw" message that the caller wishes to wrap, ie. msg := tdpb.ServerHello{}
//    - We'll probably just want to efficiently write to a stream
// 2. Receive message from the wire with intent to inspect and *maybe* unmarshal
//    - We *may* want to inspect the type, optionally unmarshal the message, and/or pass it on
//      to another stream, ideally without having to unmarshal/re-marshal
// 3. Receive message frome the wire with intent to handle. We *will* be unmarshalling it
//    - Ideally, we could marshal *into* an existing buffer

// TdpbMessage represents a partially decoded TDPB message
// It allows for lazy decoding of protobufs only when inspection is needed.
type TdpbMessage struct {
	messageType tdpb.MessageType
	// The full message including the TDPB wire header
	data []byte
	// The underlying proto message
	msg proto.Message
}

func NewTDPBMessage(msg proto.Message) *TdpbMessage {
	return &TdpbMessage{
		msg: msg,
	}
}

func (i *TdpbMessage) Encode() ([]byte, error) {
	switch {
	case i.msg != nil:
		return encodeTDPB(i.msg)
	case len(i.data) > 0:
		return i.data, nil
	default:
		return nil, errors.New("empty message")
	}
}

// EncodeTo is a convenience function that calls 'Encode' on
// the message and writes the resulting data to the writer.
func (i *TdpbMessage) EncodeTo(w io.Writer) error {
	data, err := i.Encode()
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = w.Write(data)
	return err
}

// Get the unmarshalled protobuf message.
func (i *TdpbMessage) Proto() (proto.Message, error) {
	switch {
	case i.msg != nil:
		return i.msg, nil
	case len(i.data) > 0:
		var err error
		i.msg, err = globalDecoder.decode(i.messageType, i.data[TDPB_HEADER_LENGTH:])
		return i.msg, err
	default:
		return nil, errors.New("empty message")
	}
}

func (i *TdpbMessage) As(p proto.Message) error {
	return AsTDPB(i, p)
}

// getMessageType uses protoreflection to determine the TDPB message type of
// an arbitrary proto.Message
func getMessageType(msg proto.Message) tdpb.MessageType {
	// Grab the TDPOptions extension on the message
	options := msg.ProtoReflect().Descriptor().Options().(*descriptorpb.MessageOptions)
	return proto.GetExtension(options, tdpb.E_TdpTypeOption).(tdpb.MessageType)
}

// encodeTDPB marshals the given protobuf message
// and prepends a varint message length for framing purposes.
// Returns an intermediary type that can be treated as an io.Reader or tdp.Message
func encodeTDPB(msg proto.Message) ([]byte, error) {
	mType := getMessageType(msg)
	if mType == tdpb.MessageType_MESSAGE_TYPE_UNSPECIFIED {
		return nil, trace.BadParameter("Protocol buffer message is not a valid TDPB message")
	}

	messageData, err := proto.Marshal(msg)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if len(messageData) > MAX_MESSAGE_LENGTH {
		return nil, trace.LimitExceeded("Teleport Desktop Protocol message exceeds maximum allowed length")
	}

	// Messages follow the format:
	// message_type varint | length varint | message_data []byte
	// where 'length' is the length of the 'message_data'
	out := make([]byte, len(messageData)+TDPB_HEADER_LENGTH)
	binary.BigEndian.PutUint32(out[:4], uint32(mType))
	binary.BigEndian.PutUint32(out[4:], uint32(len(messageData)))

	if count := copy(out[TDPB_HEADER_LENGTH:], messageData); count != len(messageData) {
		return nil, trace.Errorf("failed to copy message data to TDPB message buffer")
	}

	return out, nil
}

func translateFso(fso *tdpb.FileSystemObject) FileSystemObject {
	isEmpty := uint8(0)
	if !fso.IsEmpty {
		isEmpty = 1
	}
	return FileSystemObject{
		LastModified: fso.LastModified,
		Size:         fso.Size,
		FileType:     fso.FileType,
		IsEmpty:      isEmpty,
		Path:         fso.Path,
	}
}

func boolToButtonState(b bool) ButtonState {
	if b {
		return ButtonPressed
	}
	return ButtonNotPressed
}

// Converts a TDPB (Modern) message to one or more TDP (Legacy) messages
func TranslateToLegacy(msg proto.Message) []Message {
	slog.Warn("translating TDPB to TDP")
	messages := make([]Message, 0, 1)
	switch m := msg.(type) {
	case *tdpb.PNGFrame:
		messages = append(messages, PNG2Frame(m.Data))
	case *tdpb.FastPathPDU:
		messages = append(messages, RDPFastPathPDU(m.Pdu))
	case *tdpb.RDPResponsePDU: //MessageType_MESSAGE_RDP_RESPONSE_PDU:
		messages = append(messages, RDPResponsePDU(m.Response))
	case *tdpb.ConnectionActivated: //MessageType_MESSAGE_CONNECTION_ACTIVATED:
		messages = append(messages, ConnectionActivated{
			IOChannelID:   uint16(m.IoChannelId),
			UserChannelID: uint16(m.UserChannelId),
			ScreenWidth:   uint16(m.ScreenWidth),
			ScreenHeight:  uint16(m.ScreenHeight),
		})
	case *tdpb.SyncKeys: //MessageType_MESSAGE_SYNC_KEYS
		messages = append(messages, SyncKeys{
			ScrollLockState: boolToButtonState(m.ScrollLockPressed),
			NumLockState:    boolToButtonState(m.NumLockState),
			CapsLockState:   boolToButtonState(m.CapsLockState),
			KanaLockState:   boolToButtonState(m.KanaLockState),
		})
	case *tdpb.MouseMove: //MessageType_MESSAGE_MOUSE_MOVE
		messages = append(messages, MouseMove{X: m.X, Y: m.Y})
	case *tdpb.MouseButton: //MessageType_MESSAGE_MOUSE_BUTTON:
		button := MouseButtonType(m.Button - 1)
		state := ButtonNotPressed
		if m.Pressed {
			state = ButtonPressed
		}

		messages = append(messages, MouseButton{
			Button: button,
			State:  state,
		})
	case *tdpb.KeyboardButton: //MessageType_MESSAGE_KEYBOARD_BUTTON:
		state := ButtonNotPressed
		if m.Pressed {
			state = ButtonPressed
		}
		messages = append(messages, KeyboardButton{
			KeyCode: m.KeyCode,
			State:   state,
		})
	case *tdpb.ClientScreenSpec: //MessageType_MESSAGE_CLIENT_SCREEN_SPEC:
		messages = append(messages, ClientScreenSpec{
			Width:  m.Width,
			Height: m.Height,
		})
	case *tdpb.Alert: //MessageType_MESSAGE_ALERT:
		var severity Severity
		switch m.Severity {
		case tdpb.AlertSeverity_ALERT_SEVERITY_WARNING:
			severity = SeverityWarning
		case tdpb.AlertSeverity_ALERT_SEVERITY_ERROR:
			severity = SeverityError
		default:
			severity = SeverityInfo
		}
		messages = append(messages, Alert{
			Message:  m.Message,
			Severity: severity,
		})
	case *tdpb.MouseWheel: //MessageType_MESSAGE_MOUSE_WHEEL:
		messages = append(messages, MouseWheel{
			// TODO: Fix this hack
			Axis: MouseWheelAxis(m.Axis - 1),
			// TODO: validate size
			Delta: int16(m.Delta),
		})
	case *tdpb.ClipboardData: //MessageType_MESSAGE_CLIPBOARD_DATA:
		messages = append(messages, ClipboardData(m.Data))
	case *tdpb.MFA: //MessageType_MESSAGE_MFA:
		var mfaType byte
		switch m.Type {
		case tdpb.MFAType_MFA_TYPE_U2F:
			mfaType = 'u'
		case tdpb.MFAType_MFA_TYPE_WEBAUTHN:
			mfaType = 'n'
		}
		messages = append(messages, MFA{
			Type: mfaType,
			//MFAAuthenticateChallenge: m.Challenge,
			MFAAuthenticateResponse: m.AuthenticationResponse,
		})
	case *tdpb.SharedDirectoryAnnounce:
		messages = append(messages, SharedDirectoryAnnounce{
			DirectoryID: m.DirectoryId,
			Name:        m.Name,
		})
	case *tdpb.SharedDirectoryAcknowledge:
		messages = append(messages, SharedDirectoryAcknowledge{
			DirectoryID: m.DirectoryId,
			ErrCode:     m.ErrorCode,
		})
	//case *tdpb.SharedDirectoryInfoRequest:
	//	messages = append(messages, SharedDirectoryInfoRequest{
	//		DirectoryID:  m.DirectoryId,
	//		CompletionID: m.CompletionId,
	//		Path:         m.Path,
	//	})
	//case *tdpb.SharedDirectoryInfoResponse:
	//	messages = append(messages, SharedDirectoryInfoResponse{
	//		CompletionID: m.CompletionId,
	//		ErrCode:      m.ErrorCode,
	//		Fso:          translateFso(m.Fso),
	//	})
	//case *tdpb.SharedDirectoryCreateRequest:
	//	messages = append(messages, SharedDirectoryCreateRequest{
	//		CompletionID: m.CompletionId,
	//		DirectoryID:  m.DirectoryId,
	//		FileType:     m.FileType,
	//		Path:         m.Path,
	//	})
	//case *tdpb.SharedDirectoryCreateResponse: //MessageType_MESSAGE_SHARED_DIRECTORY_CREATE_RESPONSE:
	//	messages = append(messages, SharedDirectoryCreateResponse{
	//		CompletionID: m.CompletionId,
	//		Fso:          translateFso(m.Fso),
	//		ErrCode:      m.ErrorCode,
	//	})
	//case *tdpb.SharedDirectoryDeleteRequest: //MessageType_MESSAGE_SHARED_DIRECTORY_DELETE_REQUEST:
	//	messages = append(messages, SharedDirectoryDeleteRequest{
	//		DirectoryID:  m.DirectoryId,
	//		CompletionID: m.CompletionId,
	//		Path:         m.Path,
	//	})
	//case *tdpb.SharedDirectoryDeleteResponse: //MessageType_MESSAGE_SHARED_DIRECTORY_DELETE_RESPONSE:
	//	messages = append(messages, SharedDirectoryDeleteResponse{
	//		CompletionID: m.CompletionId,
	//		ErrCode:      m.ErrorCode,
	//	})
	//case *tdpb.SharedDirectoryListRequest: //MessageType_MESSAGE_SHARED_DIRECTORY_LIST_REQUEST:
	//	messages = append(messages, SharedDirectoryListRequest{
	//		CompletionID: m.CompletionId,
	//		DirectoryID:  m.DirectoryId,
	//		Path:         m.Path,
	//	})
	//case *tdpb.SharedDirectoryListResponse: //MessageType_MESSAGE_SHARED_DIRECTORY_LIST_RESPONSE:
	//	messages = append(messages, SharedDirectoryListResponse{
	//		CompletionID: m.CompletionId,
	//		ErrCode:      m.ErrorCode,
	//		FsoList: func() (out []FileSystemObject) {
	//			for _, item := range m.FsoList {
	//				out = append(out, translateFso(item))
	//			}
	//			return
	//		}(),
	//	})
	//case *tdpb.SharedDirectoryReadRequest: //MessageType_MESSAGE_SHARED_DIRECTORY_READ_REQUEST:
	//	messages = append(messages, SharedDirectoryReadRequest{
	//		CompletionID: m.CompletionId,
	//		DirectoryID:  m.DirectoryId,
	//		Path:         m.Path,
	//		Offset:       m.Offset,
	//		Length:       m.Length,
	//	})
	//case *tdpb.SharedDirectoryReadResponse: //MessageType_MESSAGE_SHARED_DIRECTORY_READ_RESPONSE:
	//	messages = append(messages, SharedDirectoryReadResponse{
	//		CompletionID:   m.CompletionId,
	//		ErrCode:        m.ErrorCode,
	//		ReadDataLength: m.ReadDataLength,
	//		ReadData:       m.ReadData,
	//	})
	//case *tdpb.SharedDirectoryWriteRequest: //MessageType_MESSAGE_SHARED_DIRECTORY_WRITE_REQUEST:
	//	messages = append(messages, SharedDirectoryWriteRequest{
	//		CompletionID:    m.CompletionId,
	//		DirectoryID:     m.DirectoryId,
	//		Offset:          m.Offset,
	//		Path:            m.Path,
	//		WriteDataLength: m.WriteDataLength,
	//		WriteData:       m.WriteData,
	//	})
	//case *tdpb.SharedDirectoryWriteResponse: //MessageType_MESSAGE_SHARED_DIRECTORY_WRITE_RESPONSE:
	//	messages = append(messages, SharedDirectoryWriteResponse{
	//		CompletionID: m.CompletionId,
	//		ErrCode:      m.ErrorCode,
	//		BytesWritten: m.BytesWritten,
	//	})
	//case *tdpb.SharedDirectoryMoveRequest: //MessageType_MESSAGE_SHARED_DIRECTORY_MOVE_REQUEST:
	//	messages = append(messages, SharedDirectoryMoveRequest{
	//		CompletionID: m.CompletionId,
	//		DirectoryID:  m.DirectoryId,
	//		OriginalPath: m.OriginalPath,
	//		NewPath:      m.NewPath,
	//	})
	//case *tdpb.SharedDirectoryMoveResponse: //MessageType_MESSAGE_SHARED_DIRECTORY_MOVE_RESPONSE:
	//	messages = append(messages, SharedDirectoryMoveResponse{
	//		CompletionID: m.CompletionId,
	//		ErrCode:      m.ErrorCode,
	//	})
	//case *tdpb.SharedDirectoryTruncateRequest: //MessageType_MESSAGE_SHARED_DIRECTORY_TRUNCATE_REQUEST:
	//	messages = append(messages, SharedDirectoryTruncateRequest{
	//		CompletionID: m.CompletionId,
	//		DirectoryID:  m.DirectoryId,
	//		Path:         m.Path,
	//		EndOfFile:    m.EndOfFile,
	//	})
	//case *tdpb.SharedDirectoryTruncateResponse: //MessageType_MESSAGE_SHARED_DIRECTORY_TRUNCATE_RESPONSE:
	//	messages = append(messages, SharedDirectoryTruncateResponse{
	//		CompletionID: m.CompletionId,
	//		ErrCode:      m.ErrorCode,
	//	})
	case *tdpb.LatencyStats: //MessageType_MESSAGE_LATENCY_STATS:
		messages = append(messages, LatencyStats{
			ClientLatency: m.ClientLatency,
			ServerLatency: m.ServerLatency,
		})
	case *tdpb.Ping: //MessageType_MESSAGE_PING:
		id, err := uuid.FromBytes(m.Uuid)
		if err != nil {
			slog.Warn("Cannot parse uuid bytes from ping", "error", err)
		} else {
			messages = append(messages, Ping{UUID: id})
		}
	//case *tdpb.ClientKeyboardLayout: //MessageType_MESSAGE_CLIENT_KEYBOARD_LAYOUT:
	//	messages = append(messages, ClientKeyboardLayout{
	//		KeyboardLayout: m.KeyboardLayout,
	//	})
	default:
		slog.Warn("Encountered unknown TDPB message!")
	}

	return messages
}

// Converts a TDP (Legacy) message to one or more TDPB (Modern) messages
func TranslateToModern(msg Message) []Message {
	slog.Warn("translating TDP to TDPB")
	messages := make([]proto.Message, 0, 1)
	switch m := msg.(type) {
	case ClientScreenSpec:
		messages = append(messages, &tdpb.ClientScreenSpec{
			Height: m.Height,
			Width:  m.Width,
		})
	case PNG2Frame:
		messages = append(messages, &tdpb.PNGFrame{
			Coordinates: &tdpb.Rectangle{
				Top:    m.Top(),
				Left:   m.Left(),
				Bottom: m.Bottom(),
				Right:  m.Right(),
			},
			Data: m.Data(),
		})
	case PNGFrame:
		buf := new(bytes.Buffer)
		if err := m.enc.Encode(buf, m.Img); err != nil {
			slog.Warn("Erroring converting TDP PNGFrame to TDPB - dropping message!")
			return nil
		}
		messages = append(messages, &tdpb.PNGFrame{
			Coordinates: &tdpb.Rectangle{
				Top:    uint32(m.Img.Bounds().Min.Y),
				Left:   uint32(m.Img.Bounds().Min.X),
				Bottom: uint32(m.Img.Bounds().Max.Y),
				Right:  uint32(m.Img.Bounds().Max.X),
			},
			Data: buf.Bytes(),
		})
	case MouseMove:
		messages = append(messages, &tdpb.MouseMove{
			X: m.X,
			Y: m.Y,
		})
	case MouseButton:
		messages = append(messages, &tdpb.MouseButton{
			Pressed: m.State == ButtonPressed,
			Button:  tdpb.MouseButtonType(m.Button + 1),
		})
	case KeyboardButton:
		messages = append(messages, &tdpb.KeyboardButton{
			KeyCode: m.KeyCode,
			Pressed: m.State == ButtonPressed,
		})
	case ClipboardData:
		messages = append(messages, &tdpb.ClipboardData{
			Data: m,
		})
	//case ClientUsername:
	//	messages = append(messages, &tdpb.ClientUsername{
	//		Username: m.Username,
	//	})
	case MouseWheel:
		messages = append(messages, &tdpb.MouseWheel{
			Axis:  tdpb.MouseWheelAxis(m.Axis - 1),
			Delta: uint32(m.Delta),
		})
	case Error:
		messages = append(messages, &tdpb.Alert{
			Message:  m.Message,
			Severity: tdpb.AlertSeverity_ALERT_SEVERITY_ERROR,
		})
	case MFA:
		// Goodness gracious, the MFA message...
		messages = append(messages)
	case Alert:
		var severity tdpb.AlertSeverity
		switch m.Severity {
		case SeverityError:
			severity = tdpb.AlertSeverity_ALERT_SEVERITY_ERROR
		case SeverityWarning:
			severity = tdpb.AlertSeverity_ALERT_SEVERITY_WARNING
		default:
			severity = tdpb.AlertSeverity_ALERT_SEVERITY_INFO
		}
		messages = append(messages, &tdpb.Alert{
			Message:  m.Message,
			Severity: severity,
		})
	case RDPFastPathPDU:
		messages = append(messages, &tdpb.FastPathPDU{
			Pdu: m,
		})
	case RDPResponsePDU:
		messages = append(messages, &tdpb.RDPResponsePDU{
			Response: m,
		})
	case ConnectionActivated:
		messages = append(messages, &tdpb.ConnectionActivated{
			IoChannelId:   uint32(m.IOChannelID),
			UserChannelId: uint32(m.UserChannelID),
			ScreenWidth:   uint32(m.ScreenWidth),
			ScreenHeight:  uint32(m.ScreenHeight),
		})
	case SyncKeys:
		messages = append(messages, &tdpb.SyncKeys{
			ScrollLockPressed: m.ScrollLockState == ButtonPressed,
			NumLockState:      m.NumLockState == ButtonPressed,
			CapsLockState:     m.CapsLockState == ButtonPressed,
			KanaLockState:     m.KanaLockState == ButtonPressed,
		})
	case LatencyStats:
		messages = append(messages, &tdpb.LatencyStats{
			ClientLatency: m.ClientLatency,
			ServerLatency: m.ServerLatency,
		})
	case Ping:
		messages = append(messages, &tdpb.Ping{
			Uuid: m.UUID[:],
		})
		//case ClientKeyboardLayout:
		//	messages = append(messages, &tdpb.ClientKeyboardLayout{
		//		KeyboardLayout: m.KeyboardLayout,
		//	})
	}

	// TODO: Translate shared directory messages
	//case TypeSharedDirectoryAnnounce:
	//case TypeSharedDirectoryAcknowledge:
	//case TypeSharedDirectoryInfoRequest:
	//case TypeSharedDirectoryInfoResponse:
	//case TypeSharedDirectoryCreateRequest:
	//case TypeSharedDirectoryCreateResponse:
	//case TypeSharedDirectoryDeleteRequest:
	//case TypeSharedDirectoryDeleteResponse:
	//case TypeSharedDirectoryReadRequest:
	//case TypeSharedDirectoryReadResponse:
	//case TypeSharedDirectoryWriteRequest:
	//case TypeSharedDirectoryWriteResponse:
	//case TypeSharedDirectoryMoveRequest:
	//case TypeSharedDirectoryMoveResponse:
	//case TypeSharedDirectoryListRequest:
	//case TypeSharedDirectoryListResponse:
	//case TypeSharedDirectoryTruncateRequest:
	//case TypeSharedDirectoryTruncateResponse:
	out := []Message{}
	for _, msg := range messages {
		out = append(out, NewTDPBMessage(msg))
	}
	return out
}
