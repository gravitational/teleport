/*
Copyright 2021 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
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
	"image"
	"image/png"
	"io"

	"github.com/gravitational/trace"

	authproto "github.com/gravitational/teleport/api/client/proto"
	wanlib "github.com/gravitational/teleport/lib/auth/webauthn"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/web/mfajson"
)

// MessageType identifies the type of the message.
type MessageType byte

// For descriptions of each message type see:
// https://github.com/gravitational/teleport/blob/master/rfd/0037-desktop-access-protocol.md#message-types
const (
	TypeClientScreenSpec              = MessageType(1)
	TypePNGFrame                      = MessageType(2)
	TypeMouseMove                     = MessageType(3)
	TypeMouseButton                   = MessageType(4)
	TypeKeyboardButton                = MessageType(5)
	TypeClipboardData                 = MessageType(6)
	TypeClientUsername                = MessageType(7)
	TypeMouseWheel                    = MessageType(8)
	TypeError                         = MessageType(9)
	TypeMFA                           = MessageType(10)
	TypeSharedDirectoryAnnounce       = MessageType(11)
	TypeSharedDirectoryAcknowledge    = MessageType(12)
	TypeSharedDirectoryInfoRequest    = MessageType(13)
	TypeSharedDirectoryInfoResponse   = MessageType(14)
	TypeSharedDirectoryCreateRequest  = MessageType(15)
	TypeSharedDirectoryCreateResponse = MessageType(16)
	TypeSharedDirectoryDeleteRequest  = MessageType(17)
	TypeSharedDirectoryDeleteResponse = MessageType(18)
	TypeSharedDirectoryReadRequest    = MessageType(19)
	TypeSharedDirectoryReadResponse   = MessageType(20)
	TypeSharedDirectoryWriteRequest   = MessageType(21)
	TypeSharedDirectoryWriteResponse  = MessageType(22)
	TypeSharedDirectoryMoveRequest    = MessageType(23)
	TypeSharedDirectoryMoveResponse   = MessageType(24)
	TypeSharedDirectoryListRequest    = MessageType(25)
	TypeSharedDirectoryListResponse   = MessageType(26)
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

// peekReader is an io.Reader which lets us peek at the first byte
// (MessageType) for decoding.
type peekReader interface {
	io.Reader
	io.ByteScanner
}

func decode(in peekReader) (Message, error) {
	// Peek at the first byte to figure out message type.
	t, err := in.ReadByte()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := in.UnreadByte(); err != nil {
		return nil, trace.Wrap(err)
	}
	switch MessageType(t) {
	case TypeClientScreenSpec:
		return decodeClientScreenSpec(in)
	case TypePNGFrame:
		return decodePNGFrame(in)
	case TypeMouseMove:
		return decodeMouseMove(in)
	case TypeMouseButton:
		return decodeMouseButton(in)
	case TypeMouseWheel:
		return decodeMouseWheel(in)
	case TypeKeyboardButton:
		return decodeKeyboardButton(in)
	case TypeClientUsername:
		return decodeClientUsername(in)
	case TypeClipboardData:
		return decodeClipboardData(in, maxClipboardDataLength)
	case TypeError:
		return decodeError(in)
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
		return decodeSharedDirectoryReadResponse(in)
	case TypeSharedDirectoryWriteRequest:
		return decodeSharedDirectoryWriteRequest(in)
	case TypeSharedDirectoryWriteResponse:
		return decodeSharedDirectoryWriteResponse(in)
	case TypeSharedDirectoryMoveRequest:
		return decodeSharedDirectoryMoveRequest(in)
	case TypeSharedDirectoryMoveResponse:
		return decodeSharedDirectoryMoveResponse(in)
	default:
		return nil, trace.BadParameter("unsupported desktop protocol message type %d", t)
	}
}

// PNGFrame is the PNG frame message
// https://github.com/gravitational/teleport/blob/master/rfd/0037-desktop-access-protocol.md#2---png-frame
type PNGFrame struct {
	Img image.Image

	enc *png.Encoder // optionally override the PNG encoder
}

func NewPNG(img image.Image, enc *png.Encoder) PNGFrame {
	return PNGFrame{
		Img: img,
		enc: enc,
	}
}

func (f PNGFrame) Encode() ([]byte, error) {
	type header struct {
		Type          byte
		Left, Top     uint32
		Right, Bottom uint32
	}

	buf := new(bytes.Buffer)
	if err := binary.Write(buf, binary.BigEndian, header{
		Type:   byte(TypePNGFrame),
		Left:   uint32(f.Img.Bounds().Min.X),
		Top:    uint32(f.Img.Bounds().Min.Y),
		Right:  uint32(f.Img.Bounds().Max.X),
		Bottom: uint32(f.Img.Bounds().Max.Y),
	}); err != nil {
		return nil, trace.Wrap(err)
	}
	encoder := f.enc
	if encoder == nil {
		encoder = &png.Encoder{}
	}
	if err := encoder.Encode(buf, f.Img); err != nil {
		return nil, trace.Wrap(err)
	}
	return buf.Bytes(), nil
}

func decodePNGFrame(in peekReader) (PNGFrame, error) {
	t, err := in.ReadByte()
	if err != nil {
		return PNGFrame{}, trace.Wrap(err)
	}
	if t != byte(TypePNGFrame) {
		return PNGFrame{}, trace.BadParameter("got message type %v, expected TypePNGFrame(%v)", t, TypePNGFrame)
	}
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

// MouseMove is the mouse movement message.
// https://github.com/gravitational/teleport/blob/master/rfd/0037-desktop-access-protocol.md#3---mouse-move
type MouseMove struct {
	X, Y uint32
}

func (m MouseMove) Encode() ([]byte, error) {
	buf := new(bytes.Buffer)
	buf.WriteByte(byte(TypeMouseMove))
	if err := binary.Write(buf, binary.BigEndian, m); err != nil {
		return nil, trace.Wrap(err)
	}
	return buf.Bytes(), nil
}

func decodeMouseMove(in peekReader) (MouseMove, error) {
	t, err := in.ReadByte()
	if err != nil {
		return MouseMove{}, trace.Wrap(err)
	}
	if t != byte(TypeMouseMove) {
		return MouseMove{}, trace.BadParameter("got message type %v, expected TypeMouseMove(%v)", t, TypeMouseMove)
	}
	var m MouseMove
	err = binary.Read(in, binary.BigEndian, &m)
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
// https://github.com/gravitational/teleport/blob/master/rfd/0037-desktop-access-protocol.md#4---mouse-button
type MouseButton struct {
	Button MouseButtonType
	State  ButtonState
}

func (m MouseButton) Encode() ([]byte, error) {
	return []byte{byte(TypeMouseButton), byte(m.Button), byte(m.State)}, nil
}

func decodeMouseButton(in peekReader) (MouseButton, error) {
	t, err := in.ReadByte()
	if err != nil {
		return MouseButton{}, trace.Wrap(err)
	}
	if t != byte(TypeMouseButton) {
		return MouseButton{}, trace.BadParameter("got message type %v, expected TypeMouseButton(%v)", t, TypeMouseButton)
	}
	var m MouseButton
	err = binary.Read(in, binary.BigEndian, &m)
	return m, trace.Wrap(err)
}

// KeyboardButton is the keyboard button press message.
// https://github.com/gravitational/teleport/blob/master/rfd/0037-desktop-access-protocol.md#4---keyboard-input
type KeyboardButton struct {
	KeyCode uint32
	State   ButtonState
}

func (k KeyboardButton) Encode() ([]byte, error) {
	buf := new(bytes.Buffer)
	buf.WriteByte(byte(TypeKeyboardButton))
	if err := binary.Write(buf, binary.BigEndian, k); err != nil {
		return nil, trace.Wrap(err)
	}
	return buf.Bytes(), nil
}

func decodeKeyboardButton(in peekReader) (KeyboardButton, error) {
	t, err := in.ReadByte()
	if err != nil {
		return KeyboardButton{}, trace.Wrap(err)
	}
	if t != byte(TypeKeyboardButton) {
		return KeyboardButton{}, trace.BadParameter("got message type %v, expected TypeKeyboardButton(%v)", t, TypeKeyboardButton)
	}
	var k KeyboardButton
	err = binary.Read(in, binary.BigEndian, &k)
	return k, trace.Wrap(err)
}

// ClientScreenSpec is the client screen specification.
// https://github.com/gravitational/teleport/blob/master/rfd/0037-desktop-access-protocol.md#1---client-screen-spec
type ClientScreenSpec struct {
	Width  uint32
	Height uint32
}

func (s ClientScreenSpec) Encode() ([]byte, error) {
	buf := new(bytes.Buffer)
	buf.WriteByte(byte(TypeClientScreenSpec))
	if err := binary.Write(buf, binary.BigEndian, s); err != nil {
		return nil, trace.Wrap(err)
	}
	return buf.Bytes(), nil
}

func decodeClientScreenSpec(in peekReader) (ClientScreenSpec, error) {
	t, err := in.ReadByte()
	if err != nil {
		return ClientScreenSpec{}, trace.Wrap(err)
	}
	if t != byte(TypeClientScreenSpec) {
		return ClientScreenSpec{}, trace.BadParameter("got message type %v, expected TypeClientScreenSpec(%v)", t, TypeClientScreenSpec)
	}
	var s ClientScreenSpec
	err = binary.Read(in, binary.BigEndian, &s)
	return s, trace.Wrap(err)
}

// ClientUsername is the client username.
// https://github.com/gravitational/teleport/blob/master/rfd/0037-desktop-access-protocol.md#7---client-username
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

func decodeClientUsername(in peekReader) (ClientUsername, error) {
	t, err := in.ReadByte()
	if err != nil {
		return ClientUsername{}, trace.Wrap(err)
	}
	if t != byte(TypeClientUsername) {
		return ClientUsername{}, trace.BadParameter("got message type %v, expected TypeClientUsername(%v)", t, TypeClientUsername)
	}
	username, err := decodeString(in, windowsMaxUsernameLength)
	if err != nil {
		return ClientUsername{}, trace.Wrap(err)
	}
	return ClientUsername{Username: username}, nil
}

type Error struct {
	Message string
}

// tdpMaxErrorMessageLength is somewhat arbitrary, as it is only sent *to*
// the browser (Teleport never receives this message, so won't be decoding it)
const tdpMaxErrorMessageLength = 10240

func (m Error) Encode() ([]byte, error) {
	buf := new(bytes.Buffer)
	buf.WriteByte(byte(TypeError))
	if err := encodeString(buf, m.Message); err != nil {
		return nil, trace.Wrap(err)
	}
	return buf.Bytes(), nil
}

func decodeError(in peekReader) (Error, error) {
	t, err := in.ReadByte()
	if err != nil {
		return Error{}, trace.Wrap(err)
	}
	if t != byte(TypeError) {
		return Error{}, trace.BadParameter("got message type %v, expected TypeError(%v)", t, TypeError)
	}
	message, err := decodeString(in, tdpMaxErrorMessageLength)
	if err != nil {
		return Error{}, trace.Wrap(err)
	}
	return Error{Message: message}, nil
}

// MouseWheelAxis identifies a scroll axis on the mouse wheel.
type MouseWheelAxis byte

const (
	VerticalWheelAxis   = MouseWheelAxis(0)
	HorizontalWheelAxis = MouseWheelAxis(1)
)

// MouseWheel is the mouse wheel scroll message.
// https://github.com/gravitational/teleport/blob/master/rfd/0037-desktop-access-protocol.md#8---mouse-wheel
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

func decodeMouseWheel(in peekReader) (MouseWheel, error) {
	t, err := in.ReadByte()
	if err != nil {
		return MouseWheel{}, trace.Wrap(err)
	}
	if t != byte(TypeMouseWheel) {
		return MouseWheel{}, trace.BadParameter("got message type %v, expected TypeMouseWheel(%v)", t, TypeMouseWheel)
	}
	var w MouseWheel
	err = binary.Read(in, binary.BigEndian, &w)
	return w, trace.Wrap(err)
}

const maxClipboardDataLength = 1024 * 1024

// ClipboardData represents shared clipboard data.
type ClipboardData []byte

func (c ClipboardData) Encode() ([]byte, error) {
	buf := new(bytes.Buffer)
	buf.WriteByte(byte(TypeClipboardData))
	binary.Write(buf, binary.BigEndian, uint32(len(c)))
	buf.Write(c)
	return buf.Bytes(), nil
}

func decodeClipboardData(in peekReader, maxLen uint32) (ClipboardData, error) {
	t, err := in.ReadByte()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if t != byte(TypeClipboardData) {
		return nil, trace.BadParameter("got message type %v, expected TypeClipboardData(%v)", t, TypeClipboardData)
	}

	var length uint32
	if err := binary.Read(in, binary.BigEndian, &length); err != nil {
		return nil, trace.Wrap(err)
	}

	if length > maxLen {
		return nil, trace.BadParameter("clipboard data exceeds maximum length")
	}

	b := make([]byte, int(length))
	if _, err := io.ReadFull(in, b); err != nil {
		return nil, trace.Wrap(err)
	}

	return ClipboardData(b), nil
}

const maxMFADataLength = 1024 * 1024

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
			buff, err = json.Marshal(wanlib.CredentialAssertionResponseFromProto(m.MFAAuthenticateResponse.GetWebauthn()))
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
	binary.Write(buf, binary.BigEndian, uint32(len(buff)))
	buf.Write(buff)
	return buf.Bytes(), nil
}

func DecodeMFA(in peekReader) (*MFA, error) {
	t, err := in.ReadByte()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if t != byte(TypeMFA) {
		return nil, trace.BadParameter("got message type %v, expected TypeMFA(%v)", t, TypeMFA)
	}

	mt, err := in.ReadByte()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	s := string(mt)
	switch s {
	case defaults.WebsocketWebauthnChallenge:
	default:
		return nil, trace.BadParameter(
			"got mfa type %v, expected %v (WebAuthn)", mt, defaults.WebsocketWebauthnChallenge)
	}

	var length uint32
	if err := binary.Read(in, binary.BigEndian, &length); err != nil {
		return nil, trace.Wrap(err)
	}

	if length > maxMFADataLength {
		return nil, trace.BadParameter("mfa challenge data exceeds maximum length")
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
func DecodeMFAChallenge(in peekReader) (*MFA, error) {
	t, err := in.ReadByte()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if t != byte(TypeMFA) {
		return nil, trace.BadParameter("got message type %v, expected TypeMFAJson(%v)", t, TypeMFA)
	}

	mt, err := in.ReadByte()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	s := string(mt)
	switch s {
	case defaults.WebsocketWebauthnChallenge:
	default:
		return nil, trace.BadParameter(
			"got mfa type %v, expected %v (WebAuthn)", mt, defaults.WebsocketWebauthnChallenge)
	}

	var length uint32
	if err := binary.Read(in, binary.BigEndian, &length); err != nil {
		return nil, trace.Wrap(err)
	}

	if length > maxMFADataLength {
		return nil, trace.BadParameter("mfa challenge data exceeds maximum length")
	}

	b := make([]byte, int(length))
	if _, err := io.ReadFull(in, b); err != nil {
		return nil, trace.Wrap(err)
	}

	var req *client.MFAAuthenticateChallenge
	if err := json.Unmarshal(b, &req); err != nil {
		return nil, trace.Wrap(err)
	}
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &MFA{
		Type:                     mt,
		MFAAuthenticateChallenge: req,
	}, nil
}

type SharedDirectoryAnnounce struct {
	DirectoryID uint32
	Name        string
}

func (s SharedDirectoryAnnounce) Encode() ([]byte, error) {
	buf := new(bytes.Buffer)
	buf.WriteByte(byte(TypeSharedDirectoryAnnounce))
	binary.Write(buf, binary.BigEndian, s.DirectoryID)
	if err := encodeString(buf, s.Name); err != nil {
		return nil, trace.Wrap(err)
	}
	return buf.Bytes(), nil
}

func decodeSharedDirectoryAnnounce(in peekReader) (SharedDirectoryAnnounce, error) {
	t, err := in.ReadByte()
	if err != nil {
		return SharedDirectoryAnnounce{}, trace.Wrap(err)
	}
	if t != byte(TypeSharedDirectoryAnnounce) {
		return SharedDirectoryAnnounce{}, trace.BadParameter("got message type %v, expected SharedDirectoryAnnounce(%v)", t, TypeSharedDirectoryAnnounce)
	}
	var completionID, directoryID uint32
	err = binary.Read(in, binary.BigEndian, &completionID)
	if err != nil {
		return SharedDirectoryAnnounce{}, trace.Wrap(err)
	}
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

type SharedDirectoryAcknowledge struct {
	ErrCode     uint32
	DirectoryID uint32
}

func decodeSharedDirectoryAcknowledge(in peekReader) (SharedDirectoryAcknowledge, error) {
	t, err := in.ReadByte()
	if err != nil {
		return SharedDirectoryAcknowledge{}, trace.Wrap(err)
	}
	if t != byte(TypeSharedDirectoryAcknowledge) {
		return SharedDirectoryAcknowledge{}, trace.BadParameter("got message type %v, expected SharedDirectoryAcknowledge(%v)", t, TypeSharedDirectoryAcknowledge)
	}

	var s SharedDirectoryAcknowledge
	err = binary.Read(in, binary.BigEndian, &s)
	return s, trace.Wrap(err)
}

func (s SharedDirectoryAcknowledge) Encode() ([]byte, error) {
	buf := new(bytes.Buffer)
	buf.WriteByte(byte(TypeSharedDirectoryAcknowledge))
	binary.Write(buf, binary.BigEndian, s.ErrCode)
	binary.Write(buf, binary.BigEndian, s.DirectoryID)
	return buf.Bytes(), nil
}

type SharedDirectoryInfoRequest struct {
	CompletionID uint32
	DirectoryID  uint32
	Path         string
}

func (s SharedDirectoryInfoRequest) Encode() ([]byte, error) {
	buf := new(bytes.Buffer)
	buf.WriteByte(byte(TypeSharedDirectoryInfoRequest))
	binary.Write(buf, binary.BigEndian, s.CompletionID)
	binary.Write(buf, binary.BigEndian, s.DirectoryID)
	if err := encodeString(buf, s.Path); err != nil {
		return nil, trace.Wrap(err)
	}
	return buf.Bytes(), nil
}

func decodeSharedDirectoryInfoRequest(in peekReader) (SharedDirectoryInfoRequest, error) {
	t, err := in.ReadByte()
	if err != nil {
		return SharedDirectoryInfoRequest{}, trace.Wrap(err)
	}
	if t != byte(TypeSharedDirectoryInfoRequest) {
		return SharedDirectoryInfoRequest{}, trace.BadParameter("got message type %v, expected SharedDirectoryInfoRequest(%v)", t, TypeSharedDirectoryInfoRequest)
	}
	var completionID, directoryID uint32
	err = binary.Read(in, binary.BigEndian, &completionID)
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

type SharedDirectoryInfoResponse struct {
	CompletionID uint32
	ErrCode      uint32
	Fso          FileSystemObject
}

func (s SharedDirectoryInfoResponse) Encode() ([]byte, error) {
	buf := new(bytes.Buffer)
	buf.WriteByte(byte(TypeSharedDirectoryInfoResponse))
	binary.Write(buf, binary.BigEndian, s.CompletionID)
	binary.Write(buf, binary.BigEndian, s.ErrCode)
	fso, err := s.Fso.Encode()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	binary.Write(buf, binary.BigEndian, fso)
	return buf.Bytes(), nil
}

func decodeSharedDirectoryInfoResponse(in peekReader) (SharedDirectoryInfoResponse, error) {
	t, err := in.ReadByte()
	if err != nil {
		return SharedDirectoryInfoResponse{}, trace.Wrap(err)
	}
	if t != byte(TypeSharedDirectoryInfoResponse) {
		return SharedDirectoryInfoResponse{}, trace.BadParameter("got message type %v, expected SharedDirectoryInfoResponse(%v)", t, TypeSharedDirectoryInfoResponse)
	}
	var completionID, errCode uint32
	err = binary.Read(in, binary.BigEndian, &completionID)
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

const tdpMaxPathLength = tdpMaxErrorMessageLength

type FileSystemObject struct {
	LastModified uint64
	Size         uint64
	FileType     uint32
	IsEmpty      uint8
	Path         string
}

func (f FileSystemObject) Encode() ([]byte, error) {
	buf := new(bytes.Buffer)
	binary.Write(buf, binary.BigEndian, f.LastModified)
	binary.Write(buf, binary.BigEndian, f.Size)
	binary.Write(buf, binary.BigEndian, f.FileType)
	buf.WriteByte(f.IsEmpty)
	if err := encodeString(buf, f.Path); err != nil {
		return nil, trace.Wrap(err)
	}
	return buf.Bytes(), nil
}

func decodeFileSystemObject(in peekReader) (FileSystemObject, error) {
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

type SharedDirectoryCreateRequest struct {
	CompletionID uint32
	DirectoryID  uint32
	FileType     uint32
	Path         string
}

func (s SharedDirectoryCreateRequest) Encode() ([]byte, error) {
	buf := new(bytes.Buffer)
	buf.WriteByte(byte(TypeSharedDirectoryCreateRequest))
	binary.Write(buf, binary.BigEndian, s.CompletionID)
	binary.Write(buf, binary.BigEndian, s.DirectoryID)
	binary.Write(buf, binary.BigEndian, s.FileType)
	if err := encodeString(buf, s.Path); err != nil {
		return nil, trace.Wrap(err)
	}

	return buf.Bytes(), nil
}

func decodeSharedDirectoryCreateRequest(in peekReader) (SharedDirectoryCreateRequest, error) {
	t, err := in.ReadByte()
	if err != nil {
		return SharedDirectoryCreateRequest{}, trace.Wrap(err)
	}
	if t != byte(TypeSharedDirectoryCreateRequest) {
		return SharedDirectoryCreateRequest{}, trace.BadParameter("got message type %v, expected SharedDirectoryCreateRequest(%v)", t, TypeSharedDirectoryCreateRequest)
	}
	var completionID, directoryID, fileType uint32
	err = binary.Read(in, binary.BigEndian, &completionID)
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

type SharedDirectoryCreateResponse struct {
	CompletionID uint32
	ErrCode      uint32
	Fso          FileSystemObject
}

func (s SharedDirectoryCreateResponse) Encode() ([]byte, error) {
	buf := new(bytes.Buffer)
	buf.WriteByte(byte(TypeSharedDirectoryCreateResponse))
	binary.Write(buf, binary.BigEndian, s.CompletionID)
	binary.Write(buf, binary.BigEndian, s.ErrCode)
	fsoEnc, err := s.Fso.Encode()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	buf.Write(fsoEnc)

	return buf.Bytes(), nil
}

func decodeSharedDirectoryCreateResponse(in peekReader) (SharedDirectoryCreateResponse, error) {
	t, err := in.ReadByte()
	if err != nil {
		return SharedDirectoryCreateResponse{}, trace.Wrap(err)
	}
	if t != byte(TypeSharedDirectoryCreateResponse) {
		return SharedDirectoryCreateResponse{}, trace.BadParameter("got message type %v, expected SharedDirectoryCreateResponse(%v)", t, TypeSharedDirectoryCreateResponse)
	}
	var completionID, errCode uint32
	err = binary.Read(in, binary.BigEndian, &completionID)
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

type SharedDirectoryDeleteRequest struct {
	CompletionID uint32
	DirectoryID  uint32
	Path         string
}

func (s SharedDirectoryDeleteRequest) Encode() ([]byte, error) {
	buf := new(bytes.Buffer)
	buf.WriteByte(byte(TypeSharedDirectoryDeleteRequest))
	binary.Write(buf, binary.BigEndian, s.CompletionID)
	binary.Write(buf, binary.BigEndian, s.DirectoryID)
	if err := encodeString(buf, s.Path); err != nil {
		return nil, trace.Wrap(err)
	}

	return buf.Bytes(), nil
}

func decodeSharedDirectoryDeleteRequest(in peekReader) (SharedDirectoryDeleteRequest, error) {
	t, err := in.ReadByte()
	if err != nil {
		return SharedDirectoryDeleteRequest{}, trace.Wrap(err)
	}
	if t != byte(TypeSharedDirectoryDeleteRequest) {
		return SharedDirectoryDeleteRequest{}, trace.BadParameter("got message type %v, expected SharedDirectoryDeleteRequest(%v)", t, TypeSharedDirectoryDeleteRequest)
	}
	var completionID, directoryID uint32
	err = binary.Read(in, binary.BigEndian, &completionID)
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

type SharedDirectoryDeleteResponse struct {
	CompletionID uint32
	ErrCode      uint32
}

func (s SharedDirectoryDeleteResponse) Encode() ([]byte, error) {
	buf := new(bytes.Buffer)
	buf.WriteByte(byte(TypeSharedDirectoryDeleteResponse))
	binary.Write(buf, binary.BigEndian, s)
	return buf.Bytes(), nil
}

func decodeSharedDirectoryDeleteResponse(in peekReader) (SharedDirectoryDeleteResponse, error) {
	t, err := in.ReadByte()
	if err != nil {
		return SharedDirectoryDeleteResponse{}, trace.Wrap(err)
	}
	if t != byte(TypeSharedDirectoryDeleteResponse) {
		return SharedDirectoryDeleteResponse{}, trace.BadParameter("got message type %v, expected SharedDirectoryDeleteResponse(%v)", t, TypeSharedDirectoryDeleteResponse)
	}

	var res SharedDirectoryDeleteResponse
	err = binary.Read(in, binary.BigEndian, &res)
	return res, err
}

type SharedDirectoryListRequest struct {
	CompletionID uint32
	DirectoryID  uint32
	Path         string
}

func (s SharedDirectoryListRequest) Encode() ([]byte, error) {
	buf := new(bytes.Buffer)
	buf.WriteByte(byte(TypeSharedDirectoryListRequest))
	binary.Write(buf, binary.BigEndian, s.CompletionID)
	binary.Write(buf, binary.BigEndian, s.DirectoryID)
	if err := encodeString(buf, s.Path); err != nil {
		return nil, trace.Wrap(err)
	}

	return buf.Bytes(), nil
}

func decodeSharedDirectoryListRequest(in peekReader) (SharedDirectoryListRequest, error) {
	t, err := in.ReadByte()
	if err != nil {
		return SharedDirectoryListRequest{}, trace.Wrap(err)
	}
	if t != byte(TypeSharedDirectoryListRequest) {
		return SharedDirectoryListRequest{}, trace.BadParameter("got message type %v, expected SharedDirectoryListRequest(%v)", t, TypeSharedDirectoryListRequest)
	}
	var completionID, directoryID uint32
	err = binary.Read(in, binary.BigEndian, &completionID)
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

// | message type (26) | completion_id uint32 | err_code uint32 | fso_list_length uint32 | fso_list fso[] |
type SharedDirectoryListResponse struct {
	CompletionID uint32
	ErrCode      uint32
	FsoList      []FileSystemObject
}

func (s SharedDirectoryListResponse) Encode() ([]byte, error) {
	buf := new(bytes.Buffer)
	buf.WriteByte(byte(TypeSharedDirectoryListResponse))
	binary.Write(buf, binary.BigEndian, s.CompletionID)
	binary.Write(buf, binary.BigEndian, s.ErrCode)
	binary.Write(buf, binary.BigEndian, uint32(len(s.FsoList)))
	for _, fso := range s.FsoList {
		fsoEnc, err := fso.Encode()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		buf.Write(fsoEnc)
	}

	return buf.Bytes(), nil
}

func decodeSharedDirectoryListResponse(in peekReader) (SharedDirectoryListResponse, error) {
	t, err := in.ReadByte()
	if err != nil {
		return SharedDirectoryListResponse{}, trace.Wrap(err)
	}
	if t != byte(TypeSharedDirectoryListResponse) {
		return SharedDirectoryListResponse{}, trace.BadParameter("got message type %v, expected SharedDirectoryListResponse(%v)", t, TypeSharedDirectoryListResponse)
	}
	var completionID, errCode, fsoListLength uint32
	err = binary.Read(in, binary.BigEndian, &completionID)
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

// SharedDirectoryReadRequest is a message sent by the server to the client to request
// bytes to be read from the file at the path and starting at byte offset.
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
	binary.Write(buf, binary.BigEndian, s.CompletionID)
	binary.Write(buf, binary.BigEndian, s.DirectoryID)
	if err := encodeString(buf, s.Path); err != nil {
		return nil, trace.Wrap(err)
	}
	binary.Write(buf, binary.BigEndian, s.Offset)
	binary.Write(buf, binary.BigEndian, s.Length)

	return buf.Bytes(), nil
}

func decodeSharedDirectoryReadRequest(in peekReader) (SharedDirectoryReadRequest, error) {
	t, err := in.ReadByte()
	if err != nil {
		return SharedDirectoryReadRequest{}, trace.Wrap(err)
	}
	if t != byte(TypeSharedDirectoryReadRequest) {
		return SharedDirectoryReadRequest{}, trace.BadParameter("got message type %v, expected TypeSharedDirectoryReadRequest(%v)", t, TypeSharedDirectoryReadRequest)
	}

	var completionID, directoryID, length uint32
	var offset uint64

	err = binary.Read(in, binary.BigEndian, &completionID)
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

// SharedDirectoryReadResponse is a message sent by the client to the server
// in response to the SharedDirectoryReadRequest.
type SharedDirectoryReadResponse struct {
	CompletionID   uint32
	ErrCode        uint32
	ReadDataLength uint32
	ReadData       []byte
}

func (s SharedDirectoryReadResponse) Encode() ([]byte, error) {
	buf := new(bytes.Buffer)
	buf.WriteByte(byte(TypeSharedDirectoryReadResponse))
	binary.Write(buf, binary.BigEndian, s.CompletionID)
	binary.Write(buf, binary.BigEndian, s.ErrCode)
	binary.Write(buf, binary.BigEndian, s.ReadDataLength)
	if _, err := buf.Write(s.ReadData); err != nil {
		return nil, trace.Wrap(err)
	}
	return buf.Bytes(), nil
}

func decodeSharedDirectoryReadResponse(in peekReader) (SharedDirectoryReadResponse, error) {
	t, err := in.ReadByte()
	if err != nil {
		return SharedDirectoryReadResponse{}, trace.Wrap(err)
	}
	if t != byte(TypeSharedDirectoryReadResponse) {
		return SharedDirectoryReadResponse{}, trace.BadParameter("got message type %v, expected TypeSharedDirectoryReadResponse(%v)", t, TypeSharedDirectoryReadResponse)
	}

	var completionID, errorCode, readDataLength uint32

	err = binary.Read(in, binary.BigEndian, &completionID)
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

// SharedDirectoryWriteRequest is a message sent by the server to the client to request
// bytes to be written the file at the path and starting at byte offset.
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
	binary.Write(buf, binary.BigEndian, s.CompletionID)
	binary.Write(buf, binary.BigEndian, s.DirectoryID)
	binary.Write(buf, binary.BigEndian, s.Offset)
	if err := encodeString(buf, s.Path); err != nil {
		return nil, trace.Wrap(err)
	}
	binary.Write(buf, binary.BigEndian, s.WriteDataLength)
	if _, err := buf.Write(s.WriteData); err != nil {
		return nil, trace.Wrap(err)
	}
	return buf.Bytes(), nil

}

func decodeSharedDirectoryWriteRequest(in peekReader) (SharedDirectoryWriteRequest, error) {
	t, err := in.ReadByte()
	if err != nil {
		return SharedDirectoryWriteRequest{}, trace.Wrap(err)
	}
	if t != byte(TypeSharedDirectoryWriteRequest) {
		return SharedDirectoryWriteRequest{}, trace.BadParameter("got message type %v, expected TypeSharedDirectoryWriteRequest(%v)", t, TypeSharedDirectoryWriteRequest)
	}

	var completionID, directoryID, writeDataLength uint32
	var offset uint64

	err = binary.Read(in, binary.BigEndian, &completionID)
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

// SharedDirectoryWriteResponse is a message sent by the client to the server
// in response to the SharedDirectoryWriteRequest.
type SharedDirectoryWriteResponse struct {
	CompletionID uint32
	ErrCode      uint32
	BytesWritten uint32
}

func (s SharedDirectoryWriteResponse) Encode() ([]byte, error) {
	buf := new(bytes.Buffer)
	buf.WriteByte(byte(TypeSharedDirectoryWriteResponse))
	binary.Write(buf, binary.BigEndian, s)
	return buf.Bytes(), nil
}

func decodeSharedDirectoryWriteResponse(in peekReader) (SharedDirectoryWriteResponse, error) {
	t, err := in.ReadByte()
	if err != nil {
		return SharedDirectoryWriteResponse{}, trace.Wrap(err)
	}
	if t != byte(TypeSharedDirectoryWriteResponse) {
		return SharedDirectoryWriteResponse{}, trace.BadParameter("got message type %v, expected SharedDirectoryWriteResponse(%v)", t, TypeSharedDirectoryWriteResponse)
	}

	var res SharedDirectoryWriteResponse
	err = binary.Read(in, binary.BigEndian, &res)
	return res, err
}

// SharedDirectoryMoveRequest is sent from the TDP server to the client
// to request a file at original_path be moved to new_path.
type SharedDirectoryMoveRequest struct {
	CompletionID uint32
	DirectoryID  uint32
	OriginalPath string
	NewPath      string
}

func (s SharedDirectoryMoveRequest) Encode() ([]byte, error) {
	buf := new(bytes.Buffer)
	buf.WriteByte(byte(TypeSharedDirectoryMoveRequest))
	binary.Write(buf, binary.BigEndian, s.CompletionID)
	binary.Write(buf, binary.BigEndian, s.DirectoryID)
	if err := encodeString(buf, s.OriginalPath); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := encodeString(buf, s.NewPath); err != nil {
		return nil, trace.Wrap(err)
	}
	return buf.Bytes(), nil
}

func decodeSharedDirectoryMoveRequest(in peekReader) (SharedDirectoryMoveRequest, error) {
	t, err := in.ReadByte()
	if err != nil {
		return SharedDirectoryMoveRequest{}, trace.Wrap(err)
	}
	if t != byte(TypeSharedDirectoryMoveRequest) {
		return SharedDirectoryMoveRequest{}, trace.BadParameter("got message type %v, expected TypeClientUsername(%v)", t, TypeClientUsername)
	}
	var completionID, directoryID uint32
	err = binary.Read(in, binary.BigEndian, &completionID)
	if err != nil {
		return SharedDirectoryMoveRequest{}, trace.Wrap(err)
	}
	err = binary.Read(in, binary.BigEndian, &directoryID)
	if err != nil {
		return SharedDirectoryMoveRequest{}, trace.Wrap(err)
	}
	originalPath, err := decodeString(in, windowsMaxUsernameLength)
	if err != nil {
		return SharedDirectoryMoveRequest{}, trace.Wrap(err)
	}
	newPath, err := decodeString(in, windowsMaxUsernameLength)
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

type SharedDirectoryMoveResponse struct {
	CompletionID uint32
	ErrCode      uint32
}

func (s SharedDirectoryMoveResponse) Encode() ([]byte, error) {
	buf := new(bytes.Buffer)
	buf.WriteByte(byte(TypeSharedDirectoryMoveResponse))
	binary.Write(buf, binary.BigEndian, s)
	return buf.Bytes(), nil
}

func decodeSharedDirectoryMoveResponse(in peekReader) (SharedDirectoryMoveResponse, error) {
	t, err := in.ReadByte()
	if err != nil {
		return SharedDirectoryMoveResponse{}, trace.Wrap(err)
	}
	if t != byte(TypeSharedDirectoryMoveResponse) {
		return SharedDirectoryMoveResponse{}, trace.BadParameter("got message type %v, expected SharedDirectoryMoveResponse(%v)", t, TypeSharedDirectoryMoveResponse)
	}

	var res SharedDirectoryMoveResponse
	err = binary.Read(in, binary.BigEndian, &res)
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
		return "", trace.BadParameter("TDP string length exceeds allowable limit")
	}

	s := make([]byte, int(length))
	if _, err := io.ReadFull(r, s); err != nil {
		return "", trace.Wrap(err)
	}
	return string(s), nil
}
