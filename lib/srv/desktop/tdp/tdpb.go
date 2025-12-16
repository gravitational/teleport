package tdp

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log/slog"

	"github.com/google/uuid"
	tdpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/desktop/v1"
	tdpbv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/desktop/v1"
	"github.com/gravitational/teleport/lib/utils/slices"
	"github.com/gravitational/trace"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
)

const (
	maxMessageLength = 10 * 1024 * 1024 /* 10MiB */
	tdpbHeaderLength = 8
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

// TDPBMessageType determines the TDPB message type of the message.
func TDPBMessageType(msg Message) tdpbv1.MessageType {
	if m, ok := msg.(TdpbMessage); ok {
		return m.messageType
	}
	return tdpbv1.MessageType_MESSAGE_TYPE_UNSPECIFIED
}

// AsTDPB is a convenience for working with TDP messages.
// Uses reflection to populate the .
func AsTDPB(msg Message, p proto.Message) error {
	if m, ok := msg.(TdpbMessage); ok {
		return m.As(p)
	}
	return trace.Errorf("expected a tdpb message type, but got %T: %w", msg, ErrInvalidMessage)
}

// ToTDPBProto attempts to extract the underlying proto.Message
// representation of a TDPB message.
func ToTDPBProto(msg Message) (proto.Message, error) {
	if m, ok := msg.(TdpbMessage); ok {
		source, err := m.Proto()
		if err != nil {
			return nil, trace.Wrap(fmt.Errorf("%w: %v", ErrInvalidMessage, err))
		}
		return source, nil
	}
	return nil, ErrInvalidMessage
}

// DecodeTDPB decodes a TDPB message
func DecodeTDPB(rdr io.Reader) (TdpbMessage, error) {
	return decodeFrom(rdr)
}

func decodeFrom(rdr io.Reader) (TdpbMessage, error) {
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

// Reads file descriptor for TDPB protobufs to build a mapping
// of message types to their corresponding protoreflect.MessageType instances.
// Think of these as blueprints used to dynamically decode protobuf messages.
// Ideally we do this once at during initialization.
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
	msgBuffer := bytes.NewBuffer(make([]byte, 0, 1024))

	// Read until we find a valid message
	for {
		// Start by searching for the header
		_, err := io.CopyN(msgBuffer, in, tdpbHeaderLength)
		if err != nil {
			return 0, nil, err
		}

		// Determine the message type and length
		mType := int32(binary.BigEndian.Uint32(msgBuffer.Bytes()[:4]))
		mLength := binary.BigEndian.Uint32(msgBuffer.Bytes()[4:])

		// Make sure this is a known message type. Simply discard it if the message
		// type does not match. We want to tolerate unknown message types in the case
		// that we add new messages later.
		_, ok := tdpb.MessageType_name[int32(mType)]
		if !ok || tdpb.MessageType(mType) == tdpb.MessageType_MESSAGE_TYPE_UNSPECIFIED {
			// Invalid/unknown message. Discard it
			slog.Info("discarding unknown TDPB message", "message-type", int32(mType), "length", mLength)
			io.CopyN(io.Discard, in, int64(mLength))
			msgBuffer.Reset()
			continue
		}

		_, err = io.CopyN(msgBuffer, in, int64(mLength))
		return tdpb.MessageType(mType), msgBuffer.Bytes(), trace.Wrap(err)
	}
}

// Scenarios for creating a tdpbMessage
// 1. Construct a "raw" message that the caller wishes to encode, ie. msg := tdpb.ServerHello{}
//    - We'll probably just want to marshal the message and write it to a stream
// 2. Receive message from the wire with intent to inspect and *maybe* unmarshal it.
//    - We *may* want to inspect the type and then *may* decide to inspect the message. Inspection
//      requires unmarshalling the proto. Alternatively, we may wish to simply pass the message
//      along without inspection, in which case we should not need to unmarshal/re-marshal the message.
// 3. Receive message frome the wire with intent to handle. We *will* be unmarshalling it.

// TdpbMessage represents a partially decoded TDPB message
// It allows for lazy decoding of protobufs only when inspection is needed.
type TdpbMessage struct {
	messageType tdpb.MessageType
	// The full message including the TDPB wire header.
	// Populated when read from the wire
	data []byte
	// The underlying proto message. Populated either when explicitly wrapped
	// using 'NewTDPBMessage' or when inspecting a 'TdpbMessage' that was decoded
	// from a stream (using TdpbMessage.Proto, or TdpbMessage.As)
	msg proto.Message
}

func NewTDPBMessage(msg proto.Message) TdpbMessage {
	return TdpbMessage{
		msg: msg,
	}
}

func (i TdpbMessage) Encode() ([]byte, error) {
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
func (i TdpbMessage) EncodeTo(w io.Writer) error {
	data, err := i.Encode()
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = w.Write(data)
	return err
}

// Get the unmarshalled protobuf message.
func (i TdpbMessage) Proto() (proto.Message, error) {
	switch {
	case i.msg != nil:
		return i.msg, nil
	case len(i.data) > 0:
		var err error
		msg, err := globalDecoder.decode(i.messageType, i.data[tdpbHeaderLength:])
		return msg, err
	default:
		return nil, errors.New("empty message")
	}
}

func (i TdpbMessage) As(p proto.Message) error {
	switch {
	case i.msg != nil:
		if i.msg.ProtoReflect().Type() != p.ProtoReflect().Type() {
			return ErrUnexpectedMessageType
		}
		proto.Merge(p, i.msg)
		return nil
	case len(i.data) > 0:
		if getMessageType(p) != i.messageType {
			return ErrUnexpectedMessageType
		}
		return proto.Unmarshal(i.data[tdpbHeaderLength:], p)
	default:
		return errors.New("empty message")
	}
}

// getMessageType uses protoreflection to determine the TDPB message type of
// an arbitrary proto.Message
func getMessageType(msg proto.Message) tdpb.MessageType {
	if msg == nil {
		return tdpbv1.MessageType_MESSAGE_TYPE_UNSPECIFIED
	}
	// Grab the TDPOptions extension on the message
	options := msg.ProtoReflect().Descriptor().Options().(*descriptorpb.MessageOptions)
	return proto.GetExtension(options, tdpb.E_TdpTypeOption).(tdpb.MessageType)
}

// encodeTDPB marshals the given protobuf message
// and prepends a varint message length for framing purposes.
// Returns an intermediary type that can be treated as an io.Reader or Message
func encodeTDPB(msg proto.Message) ([]byte, error) {
	mType := getMessageType(msg)
	if mType == tdpb.MessageType_MESSAGE_TYPE_UNSPECIFIED {
		return nil, trace.BadParameter("Protocol buffer message is not a valid TDPB message")
	}

	messageData, err := proto.Marshal(msg)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if len(messageData) > maxMessageLength {
		return nil, trace.LimitExceeded("Teleport Desktop Protocol message exceeds maximum allowed length")
	}

	// Messages follow the format:
	// message_type varint | length varint | message_data []byte
	// where 'length' is the length of the 'message_data'
	out := make([]byte, len(messageData)+tdpbHeaderLength)
	binary.BigEndian.PutUint32(out[:4], uint32(mType))
	binary.BigEndian.PutUint32(out[4:], uint32(len(messageData)))

	if count := copy(out[tdpbHeaderLength:], messageData); count != len(messageData) {
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

func translateFsoList(fso []*tdpb.FileSystemObject) []FileSystemObject {
	return slices.Map(fso, translateFso)
}

func translateFsoToModern(fso FileSystemObject) *tdpbv1.FileSystemObject {
	return &tdpbv1.FileSystemObject{
		LastModified: fso.LastModified,
		Size:         fso.Size,
		FileType:     fso.FileType,
		IsEmpty:      fso.IsEmpty == 1,
		Path:         fso.Path,
	}
}

func translateFsoListToModern(fso []FileSystemObject) []*tdpbv1.FileSystemObject {
	return slices.Map(fso, translateFsoToModern)
}

func boolToButtonState(b bool) ButtonState {
	if b {
		return ButtonPressed
	}
	return ButtonNotPressed
}

// Converts a TDPB (Modern) message to one or more TDP (Legacy) messages
func TranslateToLegacy(msg Message) ([]Message, error) {
	tdpbMsg, err := ToTDPBProto(msg)
	if err != nil {
		return nil, trace.WrapWithMessage(err, "Error unmarshalling TDPB message for translation to TDP")
	}
	messages := make([]Message, 0, 1)
	switch m := tdpbMsg.(type) {
	case *tdpb.PNGFrame:
		messages = append(messages, PNG2Frame(m.Data))
	case *tdpb.FastPathPDU:
		messages = append(messages, RDPFastPathPDU(m.Pdu))
	case *tdpb.RDPResponsePDU:
		messages = append(messages, RDPResponsePDU(m.Response))
	case *tdpb.ConnectionActivated:
		messages = append(messages, ConnectionActivated{
			IOChannelID:   uint16(m.IoChannelId),
			UserChannelID: uint16(m.UserChannelId),
			ScreenWidth:   uint16(m.ScreenWidth),
			ScreenHeight:  uint16(m.ScreenHeight),
		})
	case *tdpb.SyncKeys:
		messages = append(messages, SyncKeys{
			ScrollLockState: boolToButtonState(m.ScrollLockPressed),
			NumLockState:    boolToButtonState(m.NumLockState),
			CapsLockState:   boolToButtonState(m.CapsLockState),
			KanaLockState:   boolToButtonState(m.KanaLockState),
		})
	case *tdpb.MouseMove:
		messages = append(messages, MouseMove{X: m.X, Y: m.Y})
	case *tdpb.MouseButton:
		button := MouseButtonType(m.Button - 1)
		state := ButtonNotPressed
		if m.Pressed {
			state = ButtonPressed
		}

		messages = append(messages, MouseButton{
			Button: button,
			State:  state,
		})
	case *tdpb.KeyboardButton:
		state := ButtonNotPressed
		if m.Pressed {
			state = ButtonPressed
		}
		messages = append(messages, KeyboardButton{
			KeyCode: m.KeyCode,
			State:   state,
		})
	case *tdpb.ClientScreenSpec:
		messages = append(messages, ClientScreenSpec{
			Width:  m.Width,
			Height: m.Height,
		})
	case *tdpb.Alert:
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
	case *tdpb.MouseWheel:
		messages = append(messages, MouseWheel{
			Axis:  MouseWheelAxis(m.Axis - 1),
			Delta: int16(m.Delta),
		})
	case *tdpb.ClipboardData:
		messages = append(messages, ClipboardData(m.Data))
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
	case *tdpb.SharedDirectoryRequest:
		switch m.OperationCode {
		case tdpb.DirectoryOperation_DIRECTORY_OPERATION_INFO:
			messages = append(messages, SharedDirectoryInfoRequest{
				CompletionID: m.CompletionId,
				DirectoryID:  m.DirectoryId,
				Path:         m.Path,
			})
		case tdpb.DirectoryOperation_DIRECTORY_OPERATION_CREATE:
			messages = append(messages, SharedDirectoryCreateRequest{
				CompletionID: m.CompletionId,
				DirectoryID:  m.DirectoryId,
				FileType:     m.FileType,
				Path:         m.Path,
			})
		case tdpb.DirectoryOperation_DIRECTORY_OPERATION_DELETE:
			messages = append(messages, SharedDirectoryDeleteRequest{
				CompletionID: m.CompletionId,
				DirectoryID:  m.DirectoryId,
				Path:         m.Path,
			})
		case tdpb.DirectoryOperation_DIRECTORY_OPERATION_LIST:
			messages = append(messages, SharedDirectoryListRequest{
				CompletionID: m.CompletionId,
				DirectoryID:  m.DirectoryId,
				Path:         m.Path,
			})
		case tdpb.DirectoryOperation_DIRECTORY_OPERATION_READ:
			messages = append(messages, SharedDirectoryReadRequest{
				CompletionID: m.CompletionId,
				DirectoryID:  m.DirectoryId,
				Path:         m.Path,
				Offset:       m.Offset,
				Length:       m.Length,
			})
		case tdpb.DirectoryOperation_DIRECTORY_OPERATION_WRITE:
			messages = append(messages, SharedDirectoryWriteRequest{
				CompletionID:    m.CompletionId,
				DirectoryID:     m.DirectoryId,
				Path:            m.Path,
				Offset:          m.Offset,
				WriteDataLength: uint32(len(m.Data)),
				WriteData:       m.Data,
			})
		case tdpb.DirectoryOperation_DIRECTORY_OPERATION_MOVE:
			messages = append(messages, SharedDirectoryMoveRequest{
				CompletionID: m.CompletionId,
				DirectoryID:  m.DirectoryId,
				NewPath:      m.NewPath,
				OriginalPath: m.Path,
			})
		case tdpb.DirectoryOperation_DIRECTORY_OPERATION_TRUNCATE:
			messages = append(messages, SharedDirectoryTruncateRequest{
				CompletionID: m.CompletionId,
				DirectoryID:  m.DirectoryId,
				Path:         m.NewPath,
				EndOfFile:    m.EndOfFile,
			})
		default:
			return nil, trace.Errorf("Unknown shared directory operation code: %d", m.OperationCode)
		}
	case *tdpb.SharedDirectoryResponse:
		switch m.OperationCode {
		case tdpb.DirectoryOperation_DIRECTORY_OPERATION_INFO:
			var fso FileSystemObject
			if len(m.FsoList) > 0 {
				fso = translateFso(m.FsoList[0])
			}
			messages = append(messages, SharedDirectoryInfoResponse{
				CompletionID: m.CompletionId,
				ErrCode:      m.ErrorCode,
				Fso:          fso,
			})
		case tdpb.DirectoryOperation_DIRECTORY_OPERATION_CREATE:
			var fso FileSystemObject
			if len(m.FsoList) > 0 {
				fso = translateFso(m.FsoList[0])
			}
			messages = append(messages, SharedDirectoryCreateResponse{
				CompletionID: m.CompletionId,
				ErrCode:      m.ErrorCode,
				Fso:          fso,
			})
		case tdpb.DirectoryOperation_DIRECTORY_OPERATION_DELETE:
			messages = append(messages, SharedDirectoryDeleteResponse{
				CompletionID: m.CompletionId,
				ErrCode:      m.ErrorCode,
			})
		case tdpb.DirectoryOperation_DIRECTORY_OPERATION_LIST:
			messages = append(messages, SharedDirectoryListResponse{
				CompletionID: m.CompletionId,
				ErrCode:      m.ErrorCode,
				FsoList:      translateFsoList(m.FsoList),
			})
		case tdpb.DirectoryOperation_DIRECTORY_OPERATION_READ:
			messages = append(messages, SharedDirectoryReadResponse{
				CompletionID:   m.CompletionId,
				ErrCode:        m.ErrorCode,
				ReadData:       m.Data,
				ReadDataLength: uint32(len(m.Data)),
			})
		case tdpb.DirectoryOperation_DIRECTORY_OPERATION_WRITE:
			messages = append(messages, SharedDirectoryWriteResponse{
				CompletionID: m.CompletionId,
				ErrCode:      m.ErrorCode,
				BytesWritten: m.BytesWritten,
			})
		case tdpb.DirectoryOperation_DIRECTORY_OPERATION_MOVE:
			messages = append(messages, SharedDirectoryMoveResponse{
				CompletionID: m.CompletionId,
				ErrCode:      m.ErrorCode,
			})
		case tdpb.DirectoryOperation_DIRECTORY_OPERATION_TRUNCATE:
			messages = append(messages, SharedDirectoryTruncateResponse{
				CompletionID: m.CompletionId,
				ErrCode:      m.ErrorCode,
			})
		default:
			return nil, trace.Errorf("Unknown shared directory operation code: %d", m.OperationCode)
		}
	case *tdpb.LatencyStats:
		messages = append(messages, LatencyStats{
			ClientLatency: m.ClientLatency,
			ServerLatency: m.ServerLatency,
		})
	case *tdpb.Ping:
		id, err := uuid.FromBytes(m.Uuid)
		if err != nil {
			slog.Warn("Cannot parse uuid bytes from ping", "error", err)
		} else {
			messages = append(messages, Ping{UUID: id})
		}
	default:
		return nil, trace.Errorf("Could not translate to TDP. Encountered unexpected message type %T", m)
	}

	return messages, nil
}

// Converts a TDP (Legacy) message to one or more TDPB (Modern) messages
func TranslateToModern(msg Message) ([]Message, error) {
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
			return nil, trace.Errorf("Erroring converting TDP PNGFrame to TDPB - dropping message!: %w", err)
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
	case MouseWheel:
		messages = append(messages, &tdpb.MouseWheel{
			Axis:  tdpb.MouseWheelAxis(m.Axis + 1),
			Delta: uint32(m.Delta),
		})
	case Error:
		messages = append(messages, &tdpb.Alert{
			Message:  m.Message,
			Severity: tdpb.AlertSeverity_ALERT_SEVERITY_ERROR,
		})
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
		// Legacy TDP servers send this message once at the start
		// of the connection.
		messages = append(messages, &tdpbv1.ServerHello{
			ActivationSpec: &tdpb.ConnectionActivated{
				IoChannelId:   uint32(m.IOChannelID),
				UserChannelId: uint32(m.UserChannelID),
				ScreenWidth:   uint32(m.ScreenWidth),
				ScreenHeight:  uint32(m.ScreenHeight),
			},
			// Assume all legacy TDP servers support clipboard sharing
			ClipboardEnabled: true,
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
	case SharedDirectoryAnnounce:
		messages = append(messages, &tdpb.SharedDirectoryAnnounce{
			DirectoryId: m.DirectoryID,
			Name:        m.Name,
		})
	case SharedDirectoryAcknowledge:
		messages = append(messages, &tdpb.SharedDirectoryAcknowledge{
			DirectoryId: m.DirectoryID,
			ErrorCode:   m.ErrCode,
		})
	case SharedDirectoryInfoRequest:
		messages = append(messages, &tdpb.SharedDirectoryRequest{
			OperationCode: tdpbv1.DirectoryOperation_DIRECTORY_OPERATION_INFO,
			DirectoryId:   m.DirectoryID,
			CompletionId:  m.CompletionID,
			Path:          m.Path,
		})
	case SharedDirectoryInfoResponse:
		messages = append(messages, &tdpb.SharedDirectoryResponse{
			OperationCode: tdpbv1.DirectoryOperation_DIRECTORY_OPERATION_INFO,
			ErrorCode:     m.ErrCode,
			CompletionId:  m.CompletionID,
			FsoList:       []*tdpb.FileSystemObject{translateFsoToModern(m.Fso)},
		})
	case SharedDirectoryCreateRequest:
		messages = append(messages, &tdpb.SharedDirectoryRequest{
			OperationCode: tdpbv1.DirectoryOperation_DIRECTORY_OPERATION_CREATE,
			DirectoryId:   m.DirectoryID,
			CompletionId:  m.CompletionID,
			Path:          m.Path,
		})
	case SharedDirectoryCreateResponse:
		messages = append(messages, &tdpb.SharedDirectoryResponse{
			OperationCode: tdpbv1.DirectoryOperation_DIRECTORY_OPERATION_CREATE,
			ErrorCode:     m.ErrCode,
			CompletionId:  m.CompletionID,
			FsoList:       []*tdpb.FileSystemObject{translateFsoToModern(m.Fso)},
		})
	case SharedDirectoryDeleteRequest:
		messages = append(messages, &tdpb.SharedDirectoryRequest{
			OperationCode: tdpbv1.DirectoryOperation_DIRECTORY_OPERATION_DELETE,
			DirectoryId:   m.DirectoryID,
			CompletionId:  m.CompletionID,
			Path:          m.Path,
		})
	case SharedDirectoryDeleteResponse:
		messages = append(messages, &tdpb.SharedDirectoryResponse{
			OperationCode: tdpbv1.DirectoryOperation_DIRECTORY_OPERATION_DELETE,
			ErrorCode:     m.ErrCode,
			CompletionId:  m.CompletionID,
		})
	case SharedDirectoryReadRequest:
		messages = append(messages, &tdpb.SharedDirectoryRequest{
			OperationCode: tdpbv1.DirectoryOperation_DIRECTORY_OPERATION_READ,
			CompletionId:  m.CompletionID,
			DirectoryId:   m.DirectoryID,
			Path:          m.Path,
			Offset:        m.Offset,
			Length:        m.Length,
		})
	case SharedDirectoryReadResponse:
		messages = append(messages, &tdpb.SharedDirectoryResponse{
			OperationCode: tdpbv1.DirectoryOperation_DIRECTORY_OPERATION_READ,
			CompletionId:  m.CompletionID,
			ErrorCode:     m.ErrCode,
			Data:          m.ReadData,
		})
	case SharedDirectoryWriteRequest:
		messages = append(messages, &tdpb.SharedDirectoryRequest{
			OperationCode: tdpbv1.DirectoryOperation_DIRECTORY_OPERATION_WRITE,
			CompletionId:  m.CompletionID,
			DirectoryId:   m.DirectoryID,
			Offset:        m.Offset,
			Path:          m.Path,
			Data:          m.WriteData,
		})
	case SharedDirectoryWriteResponse:
		messages = append(messages, &tdpb.SharedDirectoryResponse{
			OperationCode: tdpbv1.DirectoryOperation_DIRECTORY_OPERATION_WRITE,
			CompletionId:  m.CompletionID,
			ErrorCode:     m.ErrCode,
			BytesWritten:  m.BytesWritten,
		})
	case SharedDirectoryMoveRequest:
		messages = append(messages, &tdpb.SharedDirectoryRequest{
			OperationCode: tdpbv1.DirectoryOperation_DIRECTORY_OPERATION_MOVE,
			CompletionId:  m.CompletionID,
			DirectoryId:   m.DirectoryID,
			Path:          m.OriginalPath,
			NewPath:       m.NewPath,
		})
	case SharedDirectoryMoveResponse:
		messages = append(messages, &tdpb.SharedDirectoryResponse{
			OperationCode: tdpbv1.DirectoryOperation_DIRECTORY_OPERATION_MOVE,
			CompletionId:  m.CompletionID,
			ErrorCode:     m.ErrCode,
		})
	case SharedDirectoryListRequest:
		messages = append(messages, &tdpb.SharedDirectoryRequest{
			OperationCode: tdpbv1.DirectoryOperation_DIRECTORY_OPERATION_LIST,
			CompletionId:  m.CompletionID,
			Path:          m.Path,
		})
	case SharedDirectoryListResponse:
		messages = append(messages, &tdpb.SharedDirectoryResponse{
			OperationCode: tdpbv1.DirectoryOperation_DIRECTORY_OPERATION_LIST,
			CompletionId:  m.CompletionID,
			ErrorCode:     m.ErrCode,
			FsoList:       translateFsoListToModern(m.FsoList),
		})
	case SharedDirectoryTruncateRequest:
		messages = append(messages, &tdpb.SharedDirectoryRequest{
			OperationCode: tdpbv1.DirectoryOperation_DIRECTORY_OPERATION_TRUNCATE,
			DirectoryId:   m.DirectoryID,
			CompletionId:  m.CompletionID,
			Path:          m.Path,
			EndOfFile:     m.EndOfFile,
		})
	case SharedDirectoryTruncateResponse:
		messages = append(messages, &tdpb.SharedDirectoryResponse{
			OperationCode: tdpbv1.DirectoryOperation_DIRECTORY_OPERATION_TRUNCATE,
			CompletionId:  m.CompletionID,
			ErrorCode:     m.ErrCode,
		})
	default:
		return nil, trace.Errorf("Could not translate to TDPB. Encountered unexpected message type %T", m)
	}

	wrapped := []Message{}
	for _, msg := range messages {
		wrapped = append(wrapped, NewTDPBMessage(msg))
	}
	return wrapped, nil
}
