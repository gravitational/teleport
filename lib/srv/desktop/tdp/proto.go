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
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/u2f"
	wanlib "github.com/gravitational/teleport/lib/auth/webauthn"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/web/mfajson"
)

// MessageType identifies the type of the message.
type MessageType byte

// For descriptions of each message type see:
// https://github.com/gravitational/teleport/blob/master/rfd/0037-desktop-access-protocol.md#message-types
const (
	TypeClientScreenSpec = MessageType(1)
	TypePNGFrame         = MessageType(2)
	TypeMouseMove        = MessageType(3)
	TypeMouseButton      = MessageType(4)
	TypeKeyboardButton   = MessageType(5)
	TypeClipboardData    = MessageType(6)
	TypeClientUsername   = MessageType(7)
	TypeMouseWheel       = MessageType(8)
	TypeError            = MessageType(9)
	TypeMFA              = MessageType(10)
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
	// Type should be one of defaults.WebsocketU2FChallenge or defaults.WebsocketWebauthnChallenge
	Type byte
	// MFAAuthenticateChallenge is the challenge we send to the client.
	// Used for messages from Teleport to the user's browser.
	*auth.MFAAuthenticateChallenge
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
		case *authproto.MFAAuthenticateResponse_U2F:
			msg := m.MFAAuthenticateResponse.GetU2F()
			resp := u2f.AuthenticateChallengeResponse{
				KeyHandle:     msg.KeyHandle,
				SignatureData: msg.Signature,
				ClientData:    msg.ClientData,
			}
			buff, err = json.Marshal(resp)
			if err != nil {
				return nil, trace.Wrap(err)
			}
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
	case defaults.WebsocketWebauthnChallenge, defaults.WebsocketU2FChallenge:
	default:
		return nil, trace.BadParameter("got mfa type %v, expected %v (WebAuthn) or %v (U2F)",
			mt, defaults.WebsocketWebauthnChallenge, defaults.WebsocketU2FChallenge)
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
	case defaults.WebsocketWebauthnChallenge, defaults.WebsocketU2FChallenge:
	default:
		return nil, trace.BadParameter("got mfa type %v, expected %v (WebAuthn) or %v (U2F)",
			mt, defaults.WebsocketWebauthnChallenge, defaults.WebsocketU2FChallenge)
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

	var req *auth.MFAAuthenticateChallenge
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
