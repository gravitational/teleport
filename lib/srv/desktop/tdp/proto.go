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
	TypePNG2Frame                     = MessageType(27)
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

func readRaw(in byteReader) ([]byte, error) {
	// Peek at the first byte to figure out message type.
	t, err := in.ReadByte()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch mt := MessageType(t); mt {
	case TypePNG2Frame:
		return readRawPNG2Frame(t, in)
	default:
		message, err := decodeMessage(t, in)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return message.Encode()
	}
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
		return nil, trace.BadParameter("unsupported desktop protocol message type %d", firstByte)
	}
}

// PNGFrame is the PNG frame message
// https://github.com/gravitational/teleport/blob/master/rfd/0037-desktop-access-protocol.md#2---png-frame
type PNGFrame struct {
	Img image.Image

	enc *png.Encoder // optionally override the PNG encoder
}

func NewPNG(img image.Image, enc *png.Encoder) PNG2Frame {
	return PNG2Frame{
		Img: img,
		enc: enc,
	}
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

// PNG2Frame is a newer versin of PNGFrame that includes the
// length of the PNG data.
type PNG2Frame PNGFrame

func decodePNG2Frame(in byteReader) (PNG2Frame, error) {
	var length uint32
	var header struct {
		Left, Top     uint32
		Right, Bottom uint32
	}
	if err := binary.Read(in, binary.BigEndian, &length); err != nil {
		return PNG2Frame{}, trace.Wrap(err)
	}
	if err := binary.Read(in, binary.BigEndian, &header); err != nil {
		return PNG2Frame{}, trace.Wrap(err)
	}
	img, err := png.Decode(io.LimitReader(in, int64(length)))
	if err != nil {
		return PNG2Frame{}, trace.Wrap(err)
	}
	// PNG encoding does not preserve offset image bounds.
	// Opportunistically restore them based on the header.
	switch img := img.(type) {
	case *image.RGBA:
		img.Rect = image.Rect(int(header.Left), int(header.Top), int(header.Right), int(header.Bottom))
	case *image.NRGBA:
		img.Rect = image.Rect(int(header.Left), int(header.Top), int(header.Right), int(header.Bottom))
	}
	return PNG2Frame{Img: img}, nil
}

func readRawPNG2Frame(firstByte byte, in byteReader) ([]byte, error) {
	if MessageType(firstByte) != TypePNG2Frame {
		return nil, trace.Wrap(trace.BadParameter("incorrect first byte passed to readRawPNG2Frame, expected %v but got %v", TypePNG2Frame, firstByte))
	}

	var pngLength uint32
	if err := binary.Read(in, binary.BigEndian, &pngLength); err != nil {
		return nil, trace.Wrap(err)
	}

	// prevent allocation of giant buffers.
	// this also avoids panic for due to overflow.
	if pngLength > maxPNGFrameDataLength {
		return nil, trace.BadParameter("pngLength too big: %v", pngLength)
	}

	b := make([]byte, 1+4+pngLength+16)
	b[0] = firstByte

	binary.BigEndian.PutUint32(b[1:5], pngLength)

	if _, err := io.ReadFull(in, b[5:]); err != nil {
		return nil, trace.Wrap(err)
	}

	return b, nil
}

func (f PNG2Frame) Encode() ([]byte, error) {
	buf := new(bytes.Buffer)
	buf.WriteByte(byte(TypePNG2Frame))

	// write 0 as a placeholder length for now
	writeUint32(buf, uint32(0))

	writeUint32(buf, uint32(f.Img.Bounds().Min.X))
	writeUint32(buf, uint32(f.Img.Bounds().Min.Y))
	writeUint32(buf, uint32(f.Img.Bounds().Max.X))
	writeUint32(buf, uint32(f.Img.Bounds().Max.Y))

	encoder := f.enc
	if encoder == nil {
		encoder = &png.Encoder{}
	}
	before := buf.Len()
	if err := encoder.Encode(buf, f.Img); err != nil {
		return nil, trace.Wrap(err)
	}
	after := buf.Len()

	result := buf.Bytes()

	// fill in the length
	pngLen := after - before
	binary.BigEndian.PutUint32(result[1:5], uint32(pngLen))

	return result, nil
}

// MouseMove is the mouse movement message.
// https://github.com/gravitational/teleport/blob/master/rfd/0037-desktop-access-protocol.md#3---mouse-move
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
// https://github.com/gravitational/teleport/blob/master/rfd/0037-desktop-access-protocol.md#4---mouse-button
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
// https://github.com/gravitational/teleport/blob/master/rfd/0037-desktop-access-protocol.md#4---keyboard-input
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
// https://github.com/gravitational/teleport/blob/master/rfd/0037-desktop-access-protocol.md#1---client-screen-spec
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

func decodeClientUsername(in io.Reader) (ClientUsername, error) {
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

func decodeError(in io.Reader) (Error, error) {
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

func decodeMouseWheel(in io.Reader) (MouseWheel, error) {
	var w MouseWheel
	err := binary.Read(in, binary.BigEndian, &w)
	return w, trace.Wrap(err)
}

const maxClipboardDataLength = 1024 * 1024

// ClipboardData represents shared clipboard data.
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
func DecodeMFAChallenge(in byteReader) (*MFA, error) {
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
	writeUint32(buf, s.CompletionID)
	writeUint32(buf, s.ErrCode)
	writeUint32(buf, s.ReadDataLength)
	buf.Write(s.ReadData)
	return buf.Bytes(), nil
}

func decodeSharedDirectoryReadResponse(in io.Reader) (SharedDirectoryReadResponse, error) {
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

func decodeSharedDirectoryWriteRequest(in byteReader) (SharedDirectoryWriteRequest, error) {
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
	writeUint32(buf, s.CompletionID)
	writeUint32(buf, s.ErrCode)
	return buf.Bytes(), nil
}

func decodeSharedDirectoryMoveResponse(in io.Reader) (SharedDirectoryMoveResponse, error) {
	var res SharedDirectoryMoveResponse
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
		return "", trace.BadParameter("TDP string length exceeds allowable limit")
	}

	s := make([]byte, int(length))
	if _, err := io.ReadFull(r, s); err != nil {
		return "", trace.Wrap(err)
	}
	return string(s), nil
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

// maxPNGFrameDataLength is maximum data length for PNG2Frame
const maxPNGFrameDataLength = 10 * 1024 * 1024 // 10MB

// These correspond to TdpErrCode enum in the rust RDP client.
const (
	ErrCodeNil           uint32 = 0
	ErrCodeFailed        uint32 = 1
	ErrCodeDoesNotExist  uint32 = 2
	ErrCodeAlreadyExists uint32 = 3
)
