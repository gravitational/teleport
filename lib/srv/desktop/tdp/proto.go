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
	"image"
	"image/png"
	"io"

	"github.com/gravitational/trace"
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
	default:
		return nil, trace.BadParameter("unsupported desktop protocol message type %d", t)
	}
}

// PNGFrame is the PNG frame message
// https://github.com/gravitational/teleport/blob/master/rfd/0037-desktop-access-protocol.md#2---png-frame
type PNGFrame struct {
	Img image.Image
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
	// Note: this uses the default png.Encoder parameters.
	// You can tweak compression level and reduce memory allocations by using a
	// custom png.Encoder, if this happens to be a bottleneck.
	if err := png.Encode(buf, f.Img); err != nil {
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
	username, err := decodeString(in)
	if err != nil {
		return ClientUsername{}, trace.Wrap(err)
	}
	return ClientUsername{Username: username}, nil
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

func decodeString(r io.Reader) (string, error) {
	var length uint32
	if err := binary.Read(r, binary.BigEndian, &length); err != nil {
		return "", trace.Wrap(err)
	}
	s := make([]byte, int(length))
	if _, err := io.ReadFull(r, s); err != nil {
		return "", trace.Wrap(err)
	}
	return string(s), nil
}
