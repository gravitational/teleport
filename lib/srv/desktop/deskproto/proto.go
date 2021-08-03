// Package deskproto implements the desktop protocol encoder/decoder.
// See https://github.com/gravitational/teleport/blob/master/rfd/0037-desktop-access-protocol.md
//
// TODO(awly): complete the implementation of all messages, even if we don't
// use them yet.
package deskproto

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/png"

	"github.com/gravitational/trace"
)

// MessageType identifies the type of the message.
type MessageType byte

const (
	TypePNGFrame       = MessageType(2)
	TypeMouseMove      = MessageType(3)
	TypeMouseButton    = MessageType(4)
	TypeKeyboardButton = MessageType(5)
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
	switch buf[0] {
	case byte(TypePNGFrame):
		return decodePNGFrame(buf)
	case byte(TypeMouseMove):
		return decodeMouseMove(buf)
	case byte(TypeMouseButton):
		return decodeMouseButton(buf)
	case byte(TypeKeyboardButton):
		return decodeKeyboardButton(buf)
	default:
		return nil, trace.BadParameter("unsupported desktop protocol message type %d", buf[0])
	}
}

// PNGFrame is the PNG frame message.
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
	if err := png.Encode(buf, f.Img); err != nil {
		return nil, trace.Wrap(err)
	}
	return buf.Bytes(), nil
}

func decodePNGFrame(buf []byte) (PNGFrame, error) {
	var header struct {
		Left, Top     uint32
		Right, Bottom uint32
	}
	r := bytes.NewReader(buf[1:])
	if err := binary.Read(r, binary.BigEndian, &header); err != nil {
		return PNGFrame{}, trace.Wrap(err)
	}
	img, err := png.Decode(r)
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

func decodeMouseMove(buf []byte) (MouseMove, error) {
	var m MouseMove
	err := binary.Read(bytes.NewReader(buf[1:]), binary.BigEndian, &m)
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

func decodeMouseButton(buf []byte) (MouseButton, error) {
	var m MouseButton
	err := binary.Read(bytes.NewReader(buf[1:]), binary.BigEndian, &m)
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

func decodeKeyboardButton(buf []byte) (KeyboardButton, error) {
	var k KeyboardButton
	err := binary.Read(bytes.NewReader(buf[1:]), binary.BigEndian, &k)
	return k, trace.Wrap(err)
}
