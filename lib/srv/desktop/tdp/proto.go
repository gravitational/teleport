/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

// Package tdp implements the Teleport desktop protocol (TDP)
// encoder/decoder.
// See https://github.com/gravitational/teleport/blob/master/rfd/0037-desktop-access-protocol.md
package tdp

// TODO(zmb3): complete the implementation of all messages, even if we don't
// use them yet.

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"image"
	"image/png"
	"io"

	"github.com/gravitational/trace"

	authproto "github.com/gravitational/teleport/api/client/proto"
	wantypes "github.com/gravitational/teleport/lib/auth/webauthntypes"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/web/mfajson"
)

// MessageType identifies the type of the message.
type MessageType byte

// For descriptions of each message type see:
// https://github.com/gravitational/teleport/blob/master/rfd/0037-desktop-access-protocol.md#message-types
const (
	TypeClientScreenSpec                = MessageType(1)
	TypePNGFrame                        = MessageType(2)
	TypeMouseMove                       = MessageType(3)
	TypeMouseButton                     = MessageType(4)
	TypeKeyboardButton                  = MessageType(5)
	TypeClipboardData                   = MessageType(6)
	TypeClientUsername                  = MessageType(7)
	TypeMouseWheel                      = MessageType(8)
	TypeError                           = MessageType(9)
	TypeMFA                             = MessageType(10)
	TypeSharedDirectoryAnnounce         = MessageType(11)
	TypeSharedDirectoryAcknowledge      = MessageType(12)
	TypeSharedDirectoryInfoRequest      = MessageType(13)
	TypeSharedDirectoryInfoResponse     = MessageType(14)
	TypeSharedDirectoryCreateRequest    = MessageType(15)
	TypeSharedDirectoryCreateResponse   = MessageType(16)
	TypeSharedDirectoryDeleteRequest    = MessageType(17)
	TypeSharedDirectoryDeleteResponse   = MessageType(18)
	TypeSharedDirectoryReadRequest      = MessageType(19)
	TypeSharedDirectoryReadResponse     = MessageType(20)
	TypeSharedDirectoryWriteRequest     = MessageType(21)
	TypeSharedDirectoryWriteResponse    = MessageType(22)
	TypeSharedDirectoryMoveRequest      = MessageType(23)
	TypeSharedDirectoryMoveResponse     = MessageType(24)
	TypeSharedDirectoryListRequest      = MessageType(25)
	TypeSharedDirectoryListResponse     = MessageType(26)
	TypePNG2Frame                       = MessageType(27)
	TypeAlert                           = MessageType(28)
	TypeRDPFastPathPDU                  = MessageType(29)
	TypeRDPResponsePDU                  = MessageType(30)
	TypeRDPConnectionInitialized        = MessageType(31)
	TypeSyncKeys                        = MessageType(32)
	TypeSharedDirectoryTruncateRequest  = MessageType(33)
	TypeSharedDirectoryTruncateResponse = MessageType(34)
)

// Message is a Go representation of a desktop protocol message.
type Message interface {
	Encode() ([]byte, error)
}

// Decode decodes the wire representation of a message.
func Decode(buf []byte) (Message, error) {
	if len(buf) == 0 {
		return nil, trace.BadParameter("input desktop protocol message is empty")
	}
	return decode(bytes.NewReader(buf))
}

type byteReader interface {
	io.Reader
	io.ByteReader
}

func decode(in byteReader) (Message, error) {
	// Peek at the first byte to figure out message type.
	t, err := in.ReadByte()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return decodeMessage(t, in)
}

func decodeMessage(firstByte byte, in byteReader) (Message, error) {
	switch mt := MessageType(firstByte); mt {
	case TypeClientScreenSpec:
		return decodeClientScreenSpec(in)
	case TypePNGFrame:
		return decodePNGFrame(in)
	case TypePNG2Frame:
		return decodePNG2Frame(in)
	case TypeRDPFastPathPDU:
		return decodeRDPFastPathPDU(in)
	case TypeRDPResponsePDU:
		return decodeRDPResponsePDU(in)
	case TypeRDPConnectionInitialized:
		return decodeConnectionActivated(in)
	case TypeMouseMove:
		return decodeMouseMove(in)
	case TypeMouseButton:
		return decodeMouseButton(in)
	case TypeMouseWheel:
		return decodeMouseWheel(in)
	case TypeKeyboardButton:
		return decodeKeyboardButton(in)
	case TypeSyncKeys:
		return decodeSyncKeys(in)
	case TypeClientUsername:
		return decodeClientUsername(in)
	case TypeClipboardData:
		return decodeClipboardData(in, maxClipboardDataLength)
	case TypeError:
		return decodeError(in)
	case TypeAlert:
		return decodeAlert(in)
	case TypeMFA:
		return DecodeMFA(in)
	case TypeSharedDirectoryAnnounce:
		return decodeSharedDirectoryAnnounce(in)
	case TypeSharedDirectoryAcknowledge:
		return decodeSharedDirectoryAcknowledge(in)
	case TypeSharedDirectoryInfoRequest:
		return decodeSharedDirectoryInfoRequest(in)
	case TypeSharedDirectoryInfoResponse:
		return decodeSharedDirectoryInfoResponse(in)
	case TypeSharedDirectoryCreateRequest:
		return decodeSharedDirectoryCreateRequest(in)
	case TypeSharedDirectoryCreateResponse:
		return decodeSharedDirectoryCreateResponse(in)
	case TypeSharedDirectoryDeleteRequest:
		return decodeSharedDirectoryDeleteRequest(in)
	case TypeSharedDirectoryDeleteResponse:
		return decodeSharedDirectoryDeleteResponse(in)
	case TypeSharedDirectoryListRequest:
		return decodeSharedDirectoryListRequest(in)
	case TypeSharedDirectoryListResponse:
		return decodeSharedDirectoryListResponse(in)
	case TypeSharedDirectoryReadRequest:
		return decodeSharedDirectoryReadRequest(in)
	case TypeSharedDirectoryReadResponse:
		return decodeSharedDirectoryReadResponse(in, tdpMaxFileReadWriteLength)
	case TypeSharedDirectoryWriteRequest:
		return decodeSharedDirectoryWriteRequest(in, tdpMaxFileReadWriteLength)
	case TypeSharedDirectoryWriteResponse:
		return decodeSharedDirectoryWriteResponse(in)
	case TypeSharedDirectoryMoveRequest:
		return decodeSharedDirectoryMoveRequest(in)
	case TypeSharedDirectoryMoveResponse:
		return decodeSharedDirectoryMoveResponse(in)
	case TypeSharedDirectoryTruncateRequest:
		return decodeSharedDirectoryTruncateRequest(in)
	case TypeSharedDirectoryTruncateResponse:
		return decodeSharedDirectoryTruncateResponse(in)
	default:
		return nil, trace.BadParameter("unsupported desktop protocol message type %d", firstByte)
	}
}

// PNGFrame is the PNG frame message
// | message type (2) | left uint32 | top uint32 | right uint32 | bottom uint32 | data []byte |
type PNGFrame struct {
	Img image.Image

	enc *png.Encoder // optionally override the PNG encoder
}

func (f PNGFrame) Encode() ([]byte, error) {
	buf := new(bytes.Buffer)
	buf.WriteByte(byte(TypePNGFrame))
	writeUint32(buf, uint32(f.Img.Bounds().Min.X))
	writeUint32(buf, uint32(f.Img.Bounds().Min.Y))
	writeUint32(buf, uint32(f.Img.Bounds().Max.X))
	writeUint32(buf, uint32(f.Img.Bounds().Max.Y))

	encoder := f.enc
	if encoder == nil {
		encoder = &png.Encoder{}
	}
	if err := encoder.Encode(buf, f.Img); err != nil {
		return nil, trace.Wrap(err)
	}
	return buf.Bytes(), nil
}

func decodePNGFrame(in byteReader) (PNGFrame, error) {
	var header struct {
		Left, Top     uint32
		Right, Bottom uint32
	}
	if err := binary.Read(in, binary.BigEndian, &header); err != nil {
		return PNGFrame{}, trace.Wrap(err)
	}
	img, err := png.Decode(in)
	if err != nil {
		return PNGFrame{}, trace.Wrap(err)
	}
	// PNG encoding does not preserve offset image bounds.
	// Opportunistically restore them based on the header.
	switch img := img.(type) {
	case *image.RGBA:
		img.Rect = image.Rect(int(header.Left), int(header.Top), int(header.Right), int(header.Bottom))
	case *image.NRGBA:
		img.Rect = image.Rect(int(header.Left), int(header.Top), int(header.Right), int(header.Bottom))
	}
	return PNGFrame{Img: img}, nil
}

// PNG2Frame is a newer version of PNGFrame that includes the
// length of the PNG data. It's represented as a fully encoded
// byte slice to optimize for speed and simplicity of encoding/decoding.
// | message type (27) | png_length uint32 | left uint32 | top uint32 | right uint32 | bottom uint32 | data []byte |
type PNG2Frame []byte

func decodePNG2Frame(in byteReader) (PNG2Frame, error) {
	// Read PNG length so we can allocate buffer that will fit PNG2Frame message
	var pngLength uint32
	if err := binary.Read(in, binary.BigEndian, &pngLength); err != nil {
		return PNG2Frame{}, trace.Wrap(err)
	}

	// prevent allocation of giant buffers.
	// this also avoids panic for pngLength ~ 4294967295 due to overflow.
	if pngLength > maxPNGFrameDataLength {
		return PNG2Frame{}, trace.BadParameter("pngLength too big: %v", pngLength)
	}

	// Allocate buffer that will fit PNG2Frame message
	// https://github.com/gravitational/teleport/blob/master/rfd/0037-desktop-access-protocol.md#27---png-frame-2
	// message type (1) + png length (4) + left, right, top, bottom (4 x 4) + data => 21 + data
	png2frame := make([]byte, 21+pngLength)

	// Write message type and png length into the buffer
	png2frame[0] = byte(TypePNG2Frame)
	binary.BigEndian.PutUint32(png2frame[1:5], pngLength)

	// Write left, top, right, bottom, and the png itself into the buffer
	if _, err := io.ReadFull(in, png2frame[5:]); err != nil {
		return PNG2Frame{}, trace.Wrap(err)
	}

	return png2frame, nil
}

func (f PNG2Frame) Encode() ([]byte, error) {
	// Encode gets called on the reusable buffer at
	// lib/srv/desktop/rdp/rdclient.Client.png2FrameBuffer,
	// which was causing us recording problems due to the async
	// nature of SessionWriter. Copying into a new buffer here is
	// a temporary hack that fixes that.
	//
	// TODO(isaiah, zmb3): remove this once a buffer pool is added.
	b := make([]byte, len(f))
	copy(b, f)
	return b, nil
}

func (f PNG2Frame) Left() uint32   { return binary.BigEndian.Uint32(f[5:9]) }
func (f PNG2Frame) Top() uint32    { return binary.BigEndian.Uint32(f[9:13]) }
func (f PNG2Frame) Right() uint32  { return binary.BigEndian.Uint32(f[13:17]) }
func (f PNG2Frame) Bottom() uint32 { return binary.BigEndian.Uint32(f[17:21]) }
func (f PNG2Frame) Data() []byte   { return f[21:] }

// RDPFastPathPDU is an RDP Fast-Path PDU message. It carries a raw
// RDP Server Fast-Path Update PDU (https://tinyurl.com/3t2t6er8) which
// is used to transport image data to the frontend.
//
// | message type (29) | data_length uint32 | data []byte |
//
// Whenever you see this type itself, you can assume that it's just
// the | data []byte | part of the message. Calling Encode() on this
// type will return the full encoded message, including the
// | message type (29) | data_length uint32 | parts.
type RDPFastPathPDU []byte

func decodeRDPFastPathPDU(in byteReader) (RDPFastPathPDU, error) {
	// Read data length so we can allocate buffer that will fit RDPFastPathPDU message
	var dataLength uint32
	if err := binary.Read(in, binary.BigEndian, &dataLength); err != nil {
		return RDPFastPathPDU(nil), trace.Wrap(err)
	}

	// Allocate buffer that will fit the data
	// TODO(isaiah): improve performance by changing
	// this api to allow buffer re-use.
	data := make([]byte, dataLength)

	// Write the data into the buffer
	if _, err := io.ReadFull(in, data); err != nil {
		return RDPFastPathPDU(nil), trace.Wrap(err)
	}

	return RDPFastPathPDU(data), nil
}

func (f RDPFastPathPDU) Encode() ([]byte, error) {
	// TODO(isaiah, zmb3): remove this once a buffer pool is added.
	b := make([]byte, 1+4+len(f))                      // byte + uint32 + len(f)
	b[0] = byte(TypeRDPFastPathPDU)                    // message type (29)
	binary.BigEndian.PutUint32(b[1:5], uint32(len(f))) // data_length uint32
	copy(b[5:], f)                                     // data []byte
	return b, nil
}

// RDPResponsePDU is an RDP Response PDU message. It carries a raw
// encoded RDP response PDU created by the ironrdp client on the
// frontend and sends it directly to the RDP server.
//
// | message type (30) | data_length uint32 | data []byte |
//
// Whenever you see this type itself, you can assume that it's just
// the | data []byte | section of the message. Calling Encode() on
// this type will return the full encoded message, including the
// | message type (30) | data_length uint32 | parts.
type RDPResponsePDU []byte

func decodeRDPResponsePDU(in byteReader) (RDPResponsePDU, error) {
	var resFrameLength uint32
	if err := binary.Read(in, binary.BigEndian, &resFrameLength); err != nil {
		return RDPResponsePDU{}, trace.Wrap(err)
	}

	resFrame := make([]byte, resFrameLength)
	if _, err := io.ReadFull(in, resFrame); err != nil {
		return RDPResponsePDU{}, trace.Wrap(err)
	}

	return resFrame, nil
}

func (r RDPResponsePDU) Encode() ([]byte, error) {
	buf := new(bytes.Buffer)
	buf.WriteByte(byte(TypeRDPResponsePDU))
	writeUint32(buf, uint32(len(r)))
	buf.Write(r)
	return buf.Bytes(), nil
}

// ConnectionActivated is sent to the browser when an RDP session is fully activated.
// This includes after the RDP connection is first initialized, or after executing a
// Deactivation-Reactivation Sequence.
//
// It contains data that the browser needs in order to correctly handle the session.
//
// See "3. Channel Connection" at https://learn.microsoft.com/en-us/openspecs/windows_protocols/ms-rdpbcgr/023f1e69-cfe8-4ee6-9ee0-7e759fb4e4ee
// Also see 1.3.1.3 Deactivation-Reactivation Sequence: https://learn.microsoft.com/en-us/openspecs/windows_protocols/ms-rdpbcgr/dfc234ce-481a-4674-9a5d-2a7bafb14432
//
// | message type (31) | io_channel_id uint16 | user_channel_id uint16 | screen_width uint16 | screen_height uint16 |
type ConnectionActivated struct {
	IOChannelID   uint16
	UserChannelID uint16
	ScreenWidth   uint16
	ScreenHeight  uint16
}

func (c ConnectionActivated) Encode() ([]byte, error) {
	buf := new(bytes.Buffer)
	buf.WriteByte(byte(TypeRDPConnectionInitialized))
	writeUint16(buf, c.IOChannelID)
	writeUint16(buf, c.UserChannelID)
	writeUint16(buf, c.ScreenWidth)
	writeUint16(buf, c.ScreenHeight)
	return buf.Bytes(), nil
}

func decodeConnectionActivated(in byteReader) (ConnectionActivated, error) {
	var ids ConnectionActivated
	err := binary.Read(in, binary.BigEndian, &ids)
	return ids, trace.Wrap(err)
}

// | message type (32) | scroll_lock_state byte | num_lock_state byte | caps_lock_state byte | kana_lock_state byte |
type SyncKeys struct {
	ScrollLockState ButtonState
	NumLockState    ButtonState
	CapsLockState   ButtonState
	KanaLockState   ButtonState
}

func (k SyncKeys) Encode() ([]byte, error) {
	buf := new(bytes.Buffer)
	buf.WriteByte(byte(TypeSyncKeys))
	buf.WriteByte(byte(k.ScrollLockState))
	buf.WriteByte(byte(k.NumLockState))
	buf.WriteByte(byte(k.CapsLockState))
	buf.WriteByte(byte(k.KanaLockState))
	return buf.Bytes(), nil
}

func decodeSyncKeys(in byteReader) (SyncKeys, error) {
	var k SyncKeys
	err := binary.Read(in, binary.BigEndian, &k)
	return k, trace.Wrap(err)
}

// MouseMove is the mouse movement message.
// | message type (3) | x uint32 | y uint32 |
type MouseMove struct {
	X, Y uint32
}

func (m MouseMove) Encode() ([]byte, error) {
	buf := new(bytes.Buffer)
	buf.WriteByte(byte(TypeMouseMove))
	writeUint32(buf, m.X)
	writeUint32(buf, m.Y)
	return buf.Bytes(), nil
}

func decodeMouseMove(in byteReader) (MouseMove, error) {
	var m MouseMove
	err := binary.Read(in, binary.BigEndian, &m)
	return m, trace.Wrap(err)
}

// MouseButtonType identifies a specific button on the mouse.
type MouseButtonType byte

const (
	LeftMouseButton   = MouseButtonType(0)
	MiddleMouseButton = MouseButtonType(1)
	RightMouseButton  = MouseButtonType(2)
)

// ButtonState is the press state of a keyboard or mouse button.
type ButtonState byte

const (
	ButtonNotPressed = ButtonState(0)
	ButtonPressed    = ButtonState(1)
)

// MouseButton is the mouse button press message.
// | message type (4) | button byte | state byte |
type MouseButton struct {
	Button MouseButtonType
	State  ButtonState
}

func (m MouseButton) Encode() ([]byte, error) {
	return []byte{byte(TypeMouseButton), byte(m.Button), byte(m.State)}, nil
}

func decodeMouseButton(in byteReader) (MouseButton, error) {
	var m MouseButton
	err := binary.Read(in, binary.BigEndian, &m)
	return m, trace.Wrap(err)
}

// KeyboardButton is the keyboard button press message.
// | message type (5) | key_code uint32 | state byte |
type KeyboardButton struct {
	KeyCode uint32
	State   ButtonState
}

func (k KeyboardButton) Encode() ([]byte, error) {
	buf := new(bytes.Buffer)
	buf.WriteByte(byte(TypeKeyboardButton))
	writeUint32(buf, k.KeyCode)
	buf.WriteByte(byte(k.State))
	return buf.Bytes(), nil
}

func decodeKeyboardButton(in byteReader) (KeyboardButton, error) {
	var k KeyboardButton
	err := binary.Read(in, binary.BigEndian, &k)
	return k, trace.Wrap(err)
}

// ClientScreenSpec is the client screen specification.
// | message type (1) | width uint32 | height uint32 |
type ClientScreenSpec struct {
	Width  uint32
	Height uint32
}

func (s ClientScreenSpec) Encode() ([]byte, error) {
	buf := new(bytes.Buffer)
	buf.WriteByte(byte(TypeClientScreenSpec))
	writeUint32(buf, s.Width)
	writeUint32(buf, s.Height)
	return buf.Bytes(), nil
}

func decodeClientScreenSpec(in io.Reader) (ClientScreenSpec, error) {
	var s ClientScreenSpec
	err := binary.Read(in, binary.BigEndian, &s)
	return s, trace.Wrap(err)
}

// ClientUsername is the client username.
// | message type (7) | username_length uint32 | username []byte |
type ClientUsername struct {
	Username string
}

// windowsMaxUsernameLength is the maximum username length, as defined by Windows
// https://docs.microsoft.com/en-us/windows-hardware/customize/desktop/unattend/microsoft-windows-shell-setup-autologon-username
const windowsMaxUsernameLength = 256

func (r ClientUsername) Encode() ([]byte, error) {
	buf := new(bytes.Buffer)
	buf.WriteByte(byte(TypeClientUsername))
	if err := encodeString(buf, r.Username); err != nil {
		return nil, trace.Wrap(err)
	}
	return buf.Bytes(), nil
}

func decodeClientUsername(in io.Reader) (ClientUsername, error) {
	username, err := decodeString(in, windowsMaxUsernameLength)
	if err != nil {
		if errors.Is(err, stringMaxLenErr) {
			// Change the error message here so it's considered a fatal error
			return ClientUsername{}, trace.LimitExceeded("ClientUsername exceeded maximum length")
		}
		return ClientUsername{}, trace.Wrap(err)
	}
	return ClientUsername{Username: username}, nil
}

// Error is used to send a fatal error message to the browser.
//
// In Teleport 12 and up, Error is deprecated and Notification
// should be preferred. Nevertheless, IT SHOULD NOT BE DELETED
// in order for older session recordings to continue to work.
//
// | message type (9) | message_length uint32 | message []byte |
type Error struct {
	Message string
}

func (m Error) Encode() ([]byte, error) {
	buf := new(bytes.Buffer)
	buf.WriteByte(byte(TypeError))
	if err := encodeString(buf, m.Message); err != nil {
		return nil, trace.Wrap(err)
	}
	return buf.Bytes(), nil
}

func decodeError(in io.Reader) (Error, error) {
	message, err := decodeString(in, tdpMaxAlertMessageLength)
	if err != nil {
		return Error{}, trace.Wrap(err)
	}
	return Error{Message: message}, nil
}

type Severity byte

const (
	SeverityInfo    Severity = 0
	SeverityWarning Severity = 1
	SeverityError   Severity = 2
)

// Alert is an informational message sent from Teleport
// to the Web UI. It can be used for fatal errors or non-fatal
// warnings.
//
// | message type (28) | message_length uint32 | message []byte | severity byte |
type Alert struct {
	Message  string
	Severity Severity
}

func (m Alert) Encode() ([]byte, error) {
	buf := new(bytes.Buffer)
	buf.WriteByte(byte(TypeAlert))
	if err := encodeString(buf, m.Message); err != nil {
		return nil, trace.Wrap(err)
	}
	buf.WriteByte(byte(m.Severity))
	return buf.Bytes(), nil
}

func decodeAlert(in byteReader) (Alert, error) {
	message, err := decodeString(in, tdpMaxAlertMessageLength)
	if err != nil {
		return Alert{}, trace.Wrap(err)
	}
	severity, err := in.ReadByte()
	if err != nil {
		return Alert{}, trace.Wrap(err)
	}
	return Alert{Message: message, Severity: Severity(severity)}, nil
}

// MouseWheelAxis identifies a scroll axis on the mouse wheel.
type MouseWheelAxis byte

const (
	VerticalWheelAxis   = MouseWheelAxis(0)
	HorizontalWheelAxis = MouseWheelAxis(1)
)

// MouseWheel is the mouse wheel scroll message.
// | message type (8) | axis byte | delta int16 |
type MouseWheel struct {
	Axis  MouseWheelAxis
	Delta int16
}

func (w MouseWheel) Encode() ([]byte, error) {
	buf := new(bytes.Buffer)
	buf.WriteByte(byte(TypeMouseWheel))
	if err := binary.Write(buf, binary.BigEndian, w); err != nil {
		return nil, trace.Wrap(err)
	}
	return buf.Bytes(), nil
}

func decodeMouseWheel(in io.Reader) (MouseWheel, error) {
	var w MouseWheel
	err := binary.Read(in, binary.BigEndian, &w)
	return w, trace.Wrap(err)
}

// ClipboardData represents shared clipboard data.
// | message type (6) | length uint32 | data []byte |
type ClipboardData []byte

func (c ClipboardData) Encode() ([]byte, error) {
	buf := new(bytes.Buffer)
	buf.WriteByte(byte(TypeClipboardData))
	writeUint32(buf, uint32(len(c)))
	buf.Write(c)
	return buf.Bytes(), nil
}

func decodeClipboardData(in io.Reader, maxLen uint32) (ClipboardData, error) {
	var length uint32
	if err := binary.Read(in, binary.BigEndian, &length); err != nil {
		return nil, trace.Wrap(err)
	}

	if length > maxLen {
		// If clipboard data exceeds maxLen,
		// discard the rest of the message
		_, _ = io.CopyN(io.Discard, in, int64(length))
		return nil, clipDataMaxLenErr
	}

	b := make([]byte, int(length))
	if _, err := io.ReadFull(in, b); err != nil {
		return nil, trace.Wrap(err)
	}

	return ClipboardData(b), nil
}

const maxMFADataLength = 1024 * 1024

// MFA represents a MFA challenge or response.
// | message type (10) | mfa_type byte | length uint32 | JSON []byte |
type MFA struct {
	// Type should be defaults.WebsocketWebauthnChallenge
	Type byte
	// MFAAuthenticateChallenge is the challenge we send to the client.
	// Used for messages from Teleport to the user's browser.
	*client.MFAAuthenticateChallenge
	// MFAAuthenticateResponse is the response to the MFA challenge,
	// sent from the browser to Teleport.
	*authproto.MFAAuthenticateResponse
}

func (m MFA) Encode() ([]byte, error) {
	buf := new(bytes.Buffer)
	buf.WriteByte(byte(TypeMFA))
	buf.WriteByte(m.Type)
	var buff []byte
	var err error

	if m.MFAAuthenticateChallenge != nil {
		buff, err = json.Marshal(m.MFAAuthenticateChallenge)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	} else if m.MFAAuthenticateResponse != nil {
		switch t := m.MFAAuthenticateResponse.Response.(type) {
		case *authproto.MFAAuthenticateResponse_Webauthn:
			buff, err = json.Marshal(wantypes.CredentialAssertionResponseFromProto(m.MFAAuthenticateResponse.GetWebauthn()))
			if err != nil {
				return nil, trace.Wrap(err)
			}
		default:
			return nil, trace.BadParameter("unsupported type %T", t)
		}
	} else {
		return nil, trace.BadParameter("got nil MFAAuthenticateChallenge and MFAAuthenticateResponse fields")
	}

	if len(buff) > maxMFADataLength {
		return nil, trace.BadParameter("mfa challenge data exceeds maximum length")
	}
	writeUint32(buf, uint32(len(buff)))
	buf.Write(buff)
	return buf.Bytes(), nil
}

func DecodeMFA(in byteReader) (*MFA, error) {
	mt, err := in.ReadByte()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	s := string(mt)
	switch s {
	case defaults.WebsocketMFAChallenge:
	default:
		return nil, trace.BadParameter(
			"got mfa type %v, expected %v (MFAChallenge)", mt, defaults.WebsocketMFAChallenge)
	}

	var length uint32
	if err := binary.Read(in, binary.BigEndian, &length); err != nil {
		return nil, trace.Wrap(err)
	}

	if length > maxMFADataLength {
		_, _ = io.CopyN(io.Discard, in, int64(length))
		return nil, mfaDataMaxLenErr
	} else if length == 0 {
		return nil, trace.BadParameter("mfa data missing")
	}

	b := make([]byte, int(length))
	if _, err := io.ReadFull(in, b); err != nil {
		return nil, trace.Wrap(err)
	}

	mfaResp, err := mfajson.Decode(b, s)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &MFA{
		Type:                    mt,
		MFAAuthenticateResponse: mfaResp,
	}, nil
}

// DecodeMFAChallenge is a helper function used in test purpose to decode MFA challenge payload because in
// real flow this logic is invoked by a fronted client.
func DecodeMFAChallenge(in byteReader) (*MFA, error) {
	mt, err := in.ReadByte()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	s := string(mt)
	switch s {
	case defaults.WebsocketMFAChallenge:
	default:
		return nil, trace.BadParameter(
			"got mfa type %v, expected %v (MFAChallenge)", mt, defaults.WebsocketMFAChallenge)
	}

	var length uint32
	if err := binary.Read(in, binary.BigEndian, &length); err != nil {
		return nil, trace.Wrap(err)
	}

	if length > maxMFADataLength {
		return nil, trace.BadParameter("mfa challenge data exceeds maximum length")
	} else if length == 0 {
		return nil, trace.BadParameter("mfa challenge data missing")
	}

	b := make([]byte, int(length))
	if _, err := io.ReadFull(in, b); err != nil {
		return nil, trace.Wrap(err)
	}

	var req client.MFAAuthenticateChallenge
	if err := json.Unmarshal(b, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	return &MFA{
		Type:                     mt,
		MFAAuthenticateChallenge: &req,
	}, nil
}

// SharedDirectoryAnnounce announces a new directory to be shared.
// | message type (11) | directory_id uint32 | name_length uint32 | name []byte |
type SharedDirectoryAnnounce struct {
	DirectoryID uint32
	Name        string
}

func (s SharedDirectoryAnnounce) Encode() ([]byte, error) {
	buf := new(bytes.Buffer)
	buf.WriteByte(byte(TypeSharedDirectoryAnnounce))
	// TODO(isaiah): The discard here allows fuzz tests to succeed, it should eventually be done away with.
	writeUint32(buf, 0) // discard
	writeUint32(buf, s.DirectoryID)
	if err := encodeString(buf, s.Name); err != nil {
		return nil, trace.Wrap(err)
	}
	return buf.Bytes(), nil
}

func decodeSharedDirectoryAnnounce(in io.Reader) (SharedDirectoryAnnounce, error) {
	// TODO(isaiah): The discard here is a copy-paste error, but we need to keep it
	// for now in order that the proxy stay compatible with previous versions of the wds.
	var discard uint32
	err := binary.Read(in, binary.BigEndian, &discard)
	if err != nil {
		return SharedDirectoryAnnounce{}, trace.Wrap(err)
	}

	var directoryID uint32
	err = binary.Read(in, binary.BigEndian, &directoryID)
	if err != nil {
		return SharedDirectoryAnnounce{}, trace.Wrap(err)
	}
	name, err := decodeString(in, windowsMaxUsernameLength)
	if err != nil {
		return SharedDirectoryAnnounce{}, trace.Wrap(err)
	}

	return SharedDirectoryAnnounce{
		DirectoryID: directoryID,
		Name:        name,
	}, nil
}

// SharedDirectoryAcknowledge acknowledges a SharedDirectoryAnnounce was received.
// | message type (12) | err_code uint32 | directory_id uint32 |
type SharedDirectoryAcknowledge struct {
	ErrCode     uint32
	DirectoryID uint32
}

func decodeSharedDirectoryAcknowledge(in io.Reader) (SharedDirectoryAcknowledge, error) {
	var s SharedDirectoryAcknowledge
	err := binary.Read(in, binary.BigEndian, &s)
	return s, trace.Wrap(err)
}

func (s SharedDirectoryAcknowledge) Encode() ([]byte, error) {
	buf := new(bytes.Buffer)
	buf.WriteByte(byte(TypeSharedDirectoryAcknowledge))
	writeUint32(buf, s.ErrCode)
	writeUint32(buf, s.DirectoryID)
	return buf.Bytes(), nil
}

// SharedDirectoryInfoRequest requests information about a file or directory.
// | message type (13) | completion_id uint32 | directory_id uint32 | path_length uint32 | path []byte |
type SharedDirectoryInfoRequest struct {
	CompletionID uint32
	DirectoryID  uint32
	Path         string
}

func (s SharedDirectoryInfoRequest) Encode() ([]byte, error) {
	buf := new(bytes.Buffer)
	buf.WriteByte(byte(TypeSharedDirectoryInfoRequest))
	writeUint32(buf, s.CompletionID)
	writeUint32(buf, s.DirectoryID)
	if err := encodeString(buf, s.Path); err != nil {
		return nil, trace.Wrap(err)
	}
	return buf.Bytes(), nil
}

func decodeSharedDirectoryInfoRequest(in io.Reader) (SharedDirectoryInfoRequest, error) {
	var completionID, directoryID uint32
	err := binary.Read(in, binary.BigEndian, &completionID)
	if err != nil {
		return SharedDirectoryInfoRequest{}, trace.Wrap(err)
	}
	err = binary.Read(in, binary.BigEndian, &directoryID)
	if err != nil {
		return SharedDirectoryInfoRequest{}, trace.Wrap(err)
	}
	path, err := decodeString(in, tdpMaxPathLength)
	if err != nil {
		return SharedDirectoryInfoRequest{}, trace.Wrap(err)
	}

	return SharedDirectoryInfoRequest{
		CompletionID: completionID,
		DirectoryID:  directoryID,
		Path:         path,
	}, nil
}

// SharedDirectoryInfoResponse returns information about a file or directory.
// | message type (14) | completion_id uint32 | err_code uint32 | file_system_object fso |
type SharedDirectoryInfoResponse struct {
	CompletionID uint32
	ErrCode      uint32
	Fso          FileSystemObject
}

func (s SharedDirectoryInfoResponse) Encode() ([]byte, error) {
	buf := new(bytes.Buffer)
	buf.WriteByte(byte(TypeSharedDirectoryInfoResponse))
	writeUint32(buf, s.CompletionID)
	writeUint32(buf, s.ErrCode)
	fso, err := s.Fso.Encode()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	buf.Write(fso)
	return buf.Bytes(), nil
}

func decodeSharedDirectoryInfoResponse(in byteReader) (SharedDirectoryInfoResponse, error) {
	var completionID, errCode uint32
	err := binary.Read(in, binary.BigEndian, &completionID)
	if err != nil {
		return SharedDirectoryInfoResponse{}, trace.Wrap(err)
	}
	err = binary.Read(in, binary.BigEndian, &errCode)
	if err != nil {
		return SharedDirectoryInfoResponse{}, trace.Wrap(err)
	}
	fso, err := decodeFileSystemObject(in)
	if err != nil {
		return SharedDirectoryInfoResponse{}, trace.Wrap(err)
	}

	return SharedDirectoryInfoResponse{
		CompletionID: completionID,
		ErrCode:      errCode,
		Fso:          fso,
	}, nil
}

// FileSystemObject represents a file or directory.
// | last_modified uint64 | size uint64 | file_type uint32 | is_empty bool | path_length uint32 | path byte[] |
type FileSystemObject struct {
	LastModified uint64
	Size         uint64
	FileType     uint32
	IsEmpty      uint8
	Path         string
}

func (f FileSystemObject) Encode() ([]byte, error) {
	buf := new(bytes.Buffer)
	writeUint64(buf, f.LastModified)
	writeUint64(buf, f.Size)
	writeUint32(buf, f.FileType)
	buf.WriteByte(f.IsEmpty)
	if err := encodeString(buf, f.Path); err != nil {
		return nil, trace.Wrap(err)
	}
	return buf.Bytes(), nil
}

func decodeFileSystemObject(in byteReader) (FileSystemObject, error) {
	var lastModified, size uint64
	var fileType uint32
	var isEmpty uint8
	err := binary.Read(in, binary.BigEndian, &lastModified)
	if err != nil {
		return FileSystemObject{}, trace.Wrap(err)
	}
	err = binary.Read(in, binary.BigEndian, &size)
	if err != nil {
		return FileSystemObject{}, trace.Wrap(err)
	}
	err = binary.Read(in, binary.BigEndian, &fileType)
	if err != nil {
		return FileSystemObject{}, trace.Wrap(err)
	}
	isEmpty, err = in.ReadByte()
	if err != nil {
		return FileSystemObject{}, trace.Wrap(err)
	}
	path, err := decodeString(in, tdpMaxPathLength)
	if err != nil {
		return FileSystemObject{}, trace.Wrap(err)
	}

	return FileSystemObject{
		LastModified: lastModified,
		Size:         size,
		FileType:     fileType,
		IsEmpty:      isEmpty,
		Path:         path,
	}, nil
}

// SharedDirectoryCreateRequest is sent by the TDP server to the client to request the creation of a new file or directory.
// | message type (15) | completion_id uint32 | directory_id uint32 | file_type uint32 | path_length uint32 | path []byte |
type SharedDirectoryCreateRequest struct {
	CompletionID uint32
	DirectoryID  uint32
	FileType     uint32
	Path         string
}

func (s SharedDirectoryCreateRequest) Encode() ([]byte, error) {
	buf := new(bytes.Buffer)
	buf.WriteByte(byte(TypeSharedDirectoryCreateRequest))
	writeUint32(buf, s.CompletionID)
	writeUint32(buf, s.DirectoryID)
	writeUint32(buf, s.FileType)
	if err := encodeString(buf, s.Path); err != nil {
		return nil, trace.Wrap(err)
	}

	return buf.Bytes(), nil
}

func decodeSharedDirectoryCreateRequest(in io.Reader) (SharedDirectoryCreateRequest, error) {
	var completionID, directoryID, fileType uint32
	err := binary.Read(in, binary.BigEndian, &completionID)
	if err != nil {
		return SharedDirectoryCreateRequest{}, trace.Wrap(err)
	}
	err = binary.Read(in, binary.BigEndian, &directoryID)
	if err != nil {
		return SharedDirectoryCreateRequest{}, trace.Wrap(err)
	}
	err = binary.Read(in, binary.BigEndian, &fileType)
	if err != nil {
		return SharedDirectoryCreateRequest{}, trace.Wrap(err)
	}
	path, err := decodeString(in, tdpMaxPathLength)
	if err != nil {
		return SharedDirectoryCreateRequest{}, trace.Wrap(err)
	}

	return SharedDirectoryCreateRequest{
		CompletionID: completionID,
		DirectoryID:  directoryID,
		FileType:     fileType,
		Path:         path,
	}, nil
}

// SharedDirectoryCreateResponseis sent by the TDP client to the server with information from an executed SharedDirectoryCreateRequest.
// | message type (16) | completion_id uint32 | err_code uint32 | file_system_object fso |
type SharedDirectoryCreateResponse struct {
	CompletionID uint32
	ErrCode      uint32
	Fso          FileSystemObject
}

func (s SharedDirectoryCreateResponse) Encode() ([]byte, error) {
	buf := new(bytes.Buffer)
	buf.WriteByte(byte(TypeSharedDirectoryCreateResponse))
	writeUint32(buf, s.CompletionID)
	writeUint32(buf, s.ErrCode)
	fsoEnc, err := s.Fso.Encode()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	buf.Write(fsoEnc)

	return buf.Bytes(), nil
}

func decodeSharedDirectoryCreateResponse(in byteReader) (SharedDirectoryCreateResponse, error) {
	var completionID, errCode uint32
	err := binary.Read(in, binary.BigEndian, &completionID)
	if err != nil {
		return SharedDirectoryCreateResponse{}, trace.Wrap(err)
	}
	err = binary.Read(in, binary.BigEndian, &errCode)
	if err != nil {
		return SharedDirectoryCreateResponse{}, trace.Wrap(err)
	}
	fso, err := decodeFileSystemObject(in)
	if err != nil {
		return SharedDirectoryCreateResponse{}, trace.Wrap(err)
	}

	return SharedDirectoryCreateResponse{
		CompletionID: completionID,
		ErrCode:      errCode,
		Fso:          fso,
	}, err
}

// SharedDirectoryDeleteRequest is sent by the TDP server to the client to request the deletion of a file or directory.
// | message type (17) | completion_id uint32 | directory_id uint32 | path_length uint32 | path []byte |
type SharedDirectoryDeleteRequest struct {
	CompletionID uint32
	DirectoryID  uint32
	Path         string
}

func (s SharedDirectoryDeleteRequest) Encode() ([]byte, error) {
	buf := new(bytes.Buffer)
	buf.WriteByte(byte(TypeSharedDirectoryDeleteRequest))
	writeUint32(buf, s.CompletionID)
	writeUint32(buf, s.DirectoryID)
	if err := encodeString(buf, s.Path); err != nil {
		return nil, trace.Wrap(err)
	}

	return buf.Bytes(), nil
}

func decodeSharedDirectoryDeleteRequest(in io.Reader) (SharedDirectoryDeleteRequest, error) {
	var completionID, directoryID uint32
	err := binary.Read(in, binary.BigEndian, &completionID)
	if err != nil {
		return SharedDirectoryDeleteRequest{}, trace.Wrap(err)
	}
	err = binary.Read(in, binary.BigEndian, &directoryID)
	if err != nil {
		return SharedDirectoryDeleteRequest{}, trace.Wrap(err)
	}
	path, err := decodeString(in, tdpMaxPathLength)
	if err != nil {
		return SharedDirectoryDeleteRequest{}, trace.Wrap(err)
	}

	return SharedDirectoryDeleteRequest{
		CompletionID: completionID,
		DirectoryID:  directoryID,
		Path:         path,
	}, nil
}

// SharedDirectoryDeleteResponse is sent by the TDP client to the server with information from an executed SharedDirectoryDeleteRequest.
// | message type (18) | completion_id uint32 | err_code uint32 |
type SharedDirectoryDeleteResponse struct {
	CompletionID uint32
	ErrCode      uint32
}

func (s SharedDirectoryDeleteResponse) Encode() ([]byte, error) {
	buf := new(bytes.Buffer)
	buf.WriteByte(byte(TypeSharedDirectoryDeleteResponse))
	writeUint32(buf, s.CompletionID)
	writeUint32(buf, s.ErrCode)
	return buf.Bytes(), nil
}

func decodeSharedDirectoryDeleteResponse(in io.Reader) (SharedDirectoryDeleteResponse, error) {
	var res SharedDirectoryDeleteResponse
	err := binary.Read(in, binary.BigEndian, &res)
	return res, err
}

// SharedDirectoryListRequest is sent by the TDP server to the client to request a directory listing.
// | message type (25) | completion_id uint32 | directory_id uint32 | path_length uint32 | path []byte |
type SharedDirectoryListRequest struct {
	CompletionID uint32
	DirectoryID  uint32
	Path         string
}

func (s SharedDirectoryListRequest) Encode() ([]byte, error) {
	buf := new(bytes.Buffer)
	buf.WriteByte(byte(TypeSharedDirectoryListRequest))
	writeUint32(buf, s.CompletionID)
	writeUint32(buf, s.DirectoryID)
	if err := encodeString(buf, s.Path); err != nil {
		return nil, trace.Wrap(err)
	}

	return buf.Bytes(), nil
}

func decodeSharedDirectoryListRequest(in io.Reader) (SharedDirectoryListRequest, error) {
	var completionID, directoryID uint32
	err := binary.Read(in, binary.BigEndian, &completionID)
	if err != nil {
		return SharedDirectoryListRequest{}, trace.Wrap(err)
	}
	err = binary.Read(in, binary.BigEndian, &directoryID)
	if err != nil {
		return SharedDirectoryListRequest{}, trace.Wrap(err)
	}
	path, err := decodeString(in, tdpMaxPathLength)
	if err != nil {
		return SharedDirectoryListRequest{}, trace.Wrap(err)
	}

	return SharedDirectoryListRequest{
		CompletionID: completionID,
		DirectoryID:  directoryID,
		Path:         path,
	}, nil
}

// SharedDirectoryListResponse is sent by the TDP client to the server with the information from an executed SharedDirectoryListRequest.
// | message type (26) | completion_id uint32 | err_code uint32 | fso_list_length uint32 | fso_list fso[] |
type SharedDirectoryListResponse struct {
	CompletionID uint32
	ErrCode      uint32
	FsoList      []FileSystemObject
}

func (s SharedDirectoryListResponse) Encode() ([]byte, error) {
	buf := new(bytes.Buffer)
	buf.WriteByte(byte(TypeSharedDirectoryListResponse))
	writeUint32(buf, s.CompletionID)
	writeUint32(buf, s.ErrCode)
	writeUint32(buf, uint32(len(s.FsoList)))

	for _, fso := range s.FsoList {
		fsoEnc, err := fso.Encode()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		buf.Write(fsoEnc)
	}

	return buf.Bytes(), nil
}

func decodeSharedDirectoryListResponse(in byteReader) (SharedDirectoryListResponse, error) {
	var completionID, errCode, fsoListLength uint32
	err := binary.Read(in, binary.BigEndian, &completionID)
	if err != nil {
		return SharedDirectoryListResponse{}, trace.Wrap(err)
	}
	err = binary.Read(in, binary.BigEndian, &errCode)
	if err != nil {
		return SharedDirectoryListResponse{}, trace.Wrap(err)
	}
	err = binary.Read(in, binary.BigEndian, &fsoListLength)
	if err != nil {
		return SharedDirectoryListResponse{}, trace.Wrap(err)
	}

	var fsoList []FileSystemObject
	for i := uint32(0); i < fsoListLength; i++ {
		fso, err := decodeFileSystemObject(in)
		if err != nil {
			return SharedDirectoryListResponse{}, trace.Wrap(err)
		}
		fsoList = append(fsoList, fso)
	}

	return SharedDirectoryListResponse{
		CompletionID: completionID,
		ErrCode:      errCode,
		FsoList:      fsoList,
	}, nil
}

// SharedDirectoryReadRequest is a message sent by the TDP server to the client to request
// bytes to be read from the file at the path and starting at byte offset.
// | message type (19) | completion_id uint32 | directory_id uint32 | path_length uint32 | path []byte | offset uint64 | length uint32 |
type SharedDirectoryReadRequest struct {
	CompletionID uint32
	DirectoryID  uint32
	Path         string
	Offset       uint64
	Length       uint32
}

func (s SharedDirectoryReadRequest) Encode() ([]byte, error) {
	buf := new(bytes.Buffer)
	buf.WriteByte(byte(TypeSharedDirectoryReadRequest))
	writeUint32(buf, s.CompletionID)
	writeUint32(buf, s.DirectoryID)
	if err := encodeString(buf, s.Path); err != nil {
		return nil, trace.Wrap(err)
	}
	writeUint64(buf, s.Offset)
	writeUint32(buf, s.Length)
	return buf.Bytes(), nil
}

func decodeSharedDirectoryReadRequest(in io.Reader) (SharedDirectoryReadRequest, error) {
	var completionID, directoryID, length uint32
	var offset uint64

	err := binary.Read(in, binary.BigEndian, &completionID)
	if err != nil {
		return SharedDirectoryReadRequest{}, trace.Wrap(err)
	}

	err = binary.Read(in, binary.BigEndian, &directoryID)
	if err != nil {
		return SharedDirectoryReadRequest{}, trace.Wrap(err)
	}

	path, err := decodeString(in, tdpMaxPathLength)
	if err != nil {
		return SharedDirectoryReadRequest{}, trace.Wrap(err)
	}

	err = binary.Read(in, binary.BigEndian, &offset)
	if err != nil {
		return SharedDirectoryReadRequest{}, trace.Wrap(err)
	}

	err = binary.Read(in, binary.BigEndian, &length)
	if err != nil {
		return SharedDirectoryReadRequest{}, trace.Wrap(err)
	}

	return SharedDirectoryReadRequest{
		CompletionID: completionID,
		DirectoryID:  directoryID,
		Path:         path,
		Offset:       offset,
		Length:       length,
	}, nil
}

// SharedDirectoryReadResponse is a message sent by the TDP client to the server
// in response to the SharedDirectoryReadRequest.
// | message type (20) | completion_id uint32 | err_code uint32 | read_data_length uint32 | read_data []byte |
type SharedDirectoryReadResponse struct {
	CompletionID   uint32
	ErrCode        uint32
	ReadDataLength uint32
	ReadData       []byte
}

func (s SharedDirectoryReadResponse) Encode() ([]byte, error) {
	buf := new(bytes.Buffer)
	buf.WriteByte(byte(TypeSharedDirectoryReadResponse))
	writeUint32(buf, s.CompletionID)
	writeUint32(buf, s.ErrCode)
	writeUint32(buf, s.ReadDataLength)
	buf.Write(s.ReadData)
	return buf.Bytes(), nil
}

func decodeSharedDirectoryReadResponse(in io.Reader, maxLen uint32) (SharedDirectoryReadResponse, error) {
	var completionID, errorCode, readDataLength uint32

	err := binary.Read(in, binary.BigEndian, &completionID)
	if err != nil {
		return SharedDirectoryReadResponse{}, trace.Wrap(err)
	}

	err = binary.Read(in, binary.BigEndian, &errorCode)
	if err != nil {
		return SharedDirectoryReadResponse{}, trace.Wrap(err)
	}

	err = binary.Read(in, binary.BigEndian, &readDataLength)
	if err != nil {
		return SharedDirectoryReadResponse{}, trace.Wrap(err)
	}

	if readDataLength > maxLen {
		_, _ = io.CopyN(io.Discard, in, int64(readDataLength))
		return SharedDirectoryReadResponse{}, fileReadWriteMaxLenErr
	}

	readData := make([]byte, int(readDataLength))
	if _, err := io.ReadFull(in, readData); err != nil {
		return SharedDirectoryReadResponse{}, trace.Wrap(err)
	}

	return SharedDirectoryReadResponse{
		CompletionID:   completionID,
		ErrCode:        errorCode,
		ReadDataLength: readDataLength,
		ReadData:       readData,
	}, nil
}

// SharedDirectoryWriteRequest is a message sent by the TDP server to the client to request
// bytes to be written the file at the path and starting at byte offset.
// | message type (21) | completion_id uint32 | directory_id uint32 | path_length uint32 | path []byte | offset uint64 | write_data_length uint32 | write_data []byte |
type SharedDirectoryWriteRequest struct {
	CompletionID    uint32
	DirectoryID     uint32
	Offset          uint64
	Path            string
	WriteDataLength uint32
	WriteData       []byte
}

func (s SharedDirectoryWriteRequest) Encode() ([]byte, error) {
	buf := new(bytes.Buffer)

	buf.WriteByte(byte(TypeSharedDirectoryWriteRequest))
	writeUint32(buf, s.CompletionID)
	writeUint32(buf, s.DirectoryID)
	writeUint64(buf, s.Offset)
	if err := encodeString(buf, s.Path); err != nil {
		return nil, trace.Wrap(err)
	}
	writeUint32(buf, s.WriteDataLength)
	buf.Write(s.WriteData)
	return buf.Bytes(), nil
}

func decodeSharedDirectoryWriteRequest(in byteReader, maxLen uint32) (SharedDirectoryWriteRequest, error) {
	var completionID, directoryID, writeDataLength uint32
	var offset uint64

	err := binary.Read(in, binary.BigEndian, &completionID)
	if err != nil {
		return SharedDirectoryWriteRequest{}, trace.Wrap(err)
	}

	err = binary.Read(in, binary.BigEndian, &directoryID)
	if err != nil {
		return SharedDirectoryWriteRequest{}, trace.Wrap(err)
	}

	err = binary.Read(in, binary.BigEndian, &offset)
	if err != nil {
		return SharedDirectoryWriteRequest{}, trace.Wrap(err)
	}

	path, err := decodeString(in, tdpMaxPathLength)
	if err != nil {
		return SharedDirectoryWriteRequest{}, trace.Wrap(err)
	}

	err = binary.Read(in, binary.BigEndian, &writeDataLength)
	if err != nil {
		return SharedDirectoryWriteRequest{}, trace.Wrap(err)
	}

	if writeDataLength > maxLen {
		_, _ = io.CopyN(io.Discard, in, int64(writeDataLength))
		return SharedDirectoryWriteRequest{}, fileReadWriteMaxLenErr
	}

	writeData := make([]byte, int(writeDataLength))
	if _, err := io.ReadFull(in, writeData); err != nil {
		return SharedDirectoryWriteRequest{}, trace.Wrap(err)
	}

	return SharedDirectoryWriteRequest{
		CompletionID:    completionID,
		DirectoryID:     directoryID,
		Path:            path,
		Offset:          offset,
		WriteDataLength: writeDataLength,
		WriteData:       writeData,
	}, nil
}

// SharedDirectoryWriteResponse is a message sent by the TDP client to the server
// in response to the SharedDirectoryWriteRequest.
// | message type (22) | completion_id uint32 | err_code uint32 | bytes_written uint32 |
type SharedDirectoryWriteResponse struct {
	CompletionID uint32
	ErrCode      uint32
	BytesWritten uint32
}

func (s SharedDirectoryWriteResponse) Encode() ([]byte, error) {
	buf := new(bytes.Buffer)
	buf.WriteByte(byte(TypeSharedDirectoryWriteResponse))
	writeUint32(buf, s.CompletionID)
	writeUint32(buf, s.ErrCode)
	writeUint32(buf, s.BytesWritten)
	return buf.Bytes(), nil
}

func decodeSharedDirectoryWriteResponse(in io.Reader) (SharedDirectoryWriteResponse, error) {
	var res SharedDirectoryWriteResponse
	err := binary.Read(in, binary.BigEndian, &res)
	return res, err
}

// SharedDirectoryMoveRequest is sent from the TDP server to the client
// to request a file at original_path be moved to new_path.
// | message type (23) | completion_id uint32 | directory_id uint32 | original_path_length uint32 | original_path []byte | new_path_length uint32 | new_path []byte |
type SharedDirectoryMoveRequest struct {
	CompletionID uint32
	DirectoryID  uint32
	OriginalPath string
	NewPath      string
}

func (s SharedDirectoryMoveRequest) Encode() ([]byte, error) {
	buf := new(bytes.Buffer)
	buf.WriteByte(byte(TypeSharedDirectoryMoveRequest))
	writeUint32(buf, s.CompletionID)
	writeUint32(buf, s.DirectoryID)
	if err := encodeString(buf, s.OriginalPath); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := encodeString(buf, s.NewPath); err != nil {
		return nil, trace.Wrap(err)
	}
	return buf.Bytes(), nil
}

func decodeSharedDirectoryMoveRequest(in io.Reader) (SharedDirectoryMoveRequest, error) {
	var completionID, directoryID uint32
	err := binary.Read(in, binary.BigEndian, &completionID)
	if err != nil {
		return SharedDirectoryMoveRequest{}, trace.Wrap(err)
	}
	err = binary.Read(in, binary.BigEndian, &directoryID)
	if err != nil {
		return SharedDirectoryMoveRequest{}, trace.Wrap(err)
	}
	originalPath, err := decodeString(in, tdpMaxPathLength)
	if err != nil {
		return SharedDirectoryMoveRequest{}, trace.Wrap(err)
	}
	newPath, err := decodeString(in, tdpMaxPathLength)
	if err != nil {
		return SharedDirectoryMoveRequest{}, trace.Wrap(err)
	}
	return SharedDirectoryMoveRequest{
		CompletionID: completionID,
		DirectoryID:  directoryID,
		OriginalPath: originalPath,
		NewPath:      newPath,
	}, nil
}

// SharedDirectoryMoveResponse is sent from the TDP client to the server
// to acknowledge a SharedDirectoryMoveRequest was executed.
// | message type (24) | completion_id uint32 | err_code uint32 |
type SharedDirectoryMoveResponse struct {
	CompletionID uint32
	ErrCode      uint32
}

func (s SharedDirectoryMoveResponse) Encode() ([]byte, error) {
	buf := new(bytes.Buffer)
	buf.WriteByte(byte(TypeSharedDirectoryMoveResponse))
	writeUint32(buf, s.CompletionID)
	writeUint32(buf, s.ErrCode)
	return buf.Bytes(), nil
}

func decodeSharedDirectoryMoveResponse(in io.Reader) (SharedDirectoryMoveResponse, error) {
	var res SharedDirectoryMoveResponse
	err := binary.Read(in, binary.BigEndian, &res)
	return res, err
}

// | message type (33) | completion_id uint32 | directory_id uint32 | path_length uint32 | path []byte | end_of_file uint32 |
type SharedDirectoryTruncateRequest struct {
	CompletionID uint32
	DirectoryID  uint32
	Path         string
	EndOfFile    uint32
}

func (s SharedDirectoryTruncateRequest) Encode() ([]byte, error) {
	buf := new(bytes.Buffer)
	buf.WriteByte(byte(TypeSharedDirectoryTruncateRequest))
	writeUint32(buf, s.CompletionID)
	writeUint32(buf, s.DirectoryID)
	if err := encodeString(buf, s.Path); err != nil {
		return nil, trace.Wrap(err)
	}
	writeUint32(buf, s.EndOfFile)
	return buf.Bytes(), nil
}

func decodeSharedDirectoryTruncateRequest(in io.Reader) (SharedDirectoryTruncateRequest, error) {
	var completionID, directoryID, endOfFile uint32
	err := binary.Read(in, binary.BigEndian, &completionID)
	if err != nil {
		return SharedDirectoryTruncateRequest{}, trace.Wrap(err)
	}
	err = binary.Read(in, binary.BigEndian, &directoryID)
	if err != nil {
		return SharedDirectoryTruncateRequest{}, trace.Wrap(err)
	}
	path, err := decodeString(in, tdpMaxPathLength)
	if err != nil {
		return SharedDirectoryTruncateRequest{}, trace.Wrap(err)
	}
	err = binary.Read(in, binary.BigEndian, &endOfFile)
	if err != nil {
		return SharedDirectoryTruncateRequest{}, trace.Wrap(err)
	}
	return SharedDirectoryTruncateRequest{
		CompletionID: completionID,
		DirectoryID:  directoryID,
		Path:         path,
		EndOfFile:    endOfFile,
	}, nil
}

// SharedDirectoryTruncateResponse is sent from the TDP client to the server
// to acknowledge a SharedDirectoryTruncateRequest was executed.
// | message type (34) | completion_id uint32 | err_code uint32 |
type SharedDirectoryTruncateResponse struct {
	CompletionID uint32
	ErrCode      uint32
}

func (s SharedDirectoryTruncateResponse) Encode() ([]byte, error) {
	buf := new(bytes.Buffer)
	buf.WriteByte(byte(TypeSharedDirectoryTruncateResponse))
	writeUint32(buf, s.CompletionID)
	writeUint32(buf, s.ErrCode)
	return buf.Bytes(), nil
}

func decodeSharedDirectoryTruncateResponse(in io.Reader) (SharedDirectoryTruncateResponse, error) {
	var res SharedDirectoryTruncateResponse
	err := binary.Read(in, binary.BigEndian, &res)
	return res, err
}

// encodeString encodes strings for TDP. Strings are encoded as UTF-8 with
// a 32-bit length prefix (in bytes):
// https://github.com/gravitational/teleport/blob/master/rfd/0037-desktop-access-protocol.md#field-types
func encodeString(w io.Writer, s string) error {
	if err := binary.Write(w, binary.BigEndian, uint32(len(s))); err != nil {
		return trace.Wrap(err)
	}
	if _, err := io.WriteString(w, s); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func decodeString(r io.Reader, maxLen uint32) (string, error) {
	var length uint32
	if err := binary.Read(r, binary.BigEndian, &length); err != nil {
		return "", trace.Wrap(err)
	}

	if length > maxLen {
		_, _ = io.CopyN(io.Discard, r, int64(length))
		return "", stringMaxLenErr
	}

	s := make([]byte, int(length))
	if _, err := io.ReadFull(r, s); err != nil {
		return "", trace.Wrap(err)
	}
	return string(s), nil
}

// writeUint16 writes v to b in big endian order
func writeUint16(b *bytes.Buffer, v uint16) {
	b.WriteByte(byte(v >> 8))
	b.WriteByte(byte(v))
}

// writeUint32 writes v to b in big endian order
func writeUint32(b *bytes.Buffer, v uint32) {
	b.WriteByte(byte(v >> 24))
	b.WriteByte(byte(v >> 16))
	b.WriteByte(byte(v >> 8))
	b.WriteByte(byte(v))
}

// writeUint64 writes v to b in big endian order
func writeUint64(b *bytes.Buffer, v uint64) {
	b.WriteByte(byte(v >> 56))
	b.WriteByte(byte(v >> 48))
	b.WriteByte(byte(v >> 40))
	b.WriteByte(byte(v >> 32))
	b.WriteByte(byte(v >> 24))
	b.WriteByte(byte(v >> 16))
	b.WriteByte(byte(v >> 8))
	b.WriteByte(byte(v))
}

const (
	// tdpMaxAlertMessageLength is somewhat arbitrary, as it is only sent *to*
	// the browser (Teleport never receives this message, so won't be decoding it)
	tdpMaxAlertMessageLength = 10240

	// tdpMaxPathLength is somewhat arbitrary because we weren't able to determine
	// a precise value to set it to: https://github.com/gravitational/teleport/issues/14950#issuecomment-1341632465
	// The limit is kept as an additional defense-in-depth measure.
	tdpMaxPathLength = 10240

	maxClipboardDataLength    = 1024 * 1024 // 1MB
	tdpMaxFileReadWriteLength = 1024 * 1024 // 1MB
)

// maxPNGFrameDataLength is maximum data length for PNG2Frame
const maxPNGFrameDataLength = 10 * 1024 * 1024 // 10MB

// These correspond to TdpErrCode enum in the rust RDP client.
const (
	ErrCodeNil           uint32 = 0
	ErrCodeFailed        uint32 = 1
	ErrCodeDoesNotExist  uint32 = 2
	ErrCodeAlreadyExists uint32 = 3
)

var (
	clipDataMaxLenErr      = trace.LimitExceeded("clipboard sync failed: clipboard data exceeded maximum length")
	stringMaxLenErr        = trace.LimitExceeded("TDP string length exceeds allowable limit")
	fileReadWriteMaxLenErr = trace.LimitExceeded("TDP file read or write message exceeds maximum size limit")
	mfaDataMaxLenErr       = trace.LimitExceeded("MFA challenge data exceeds maximum length")
)
