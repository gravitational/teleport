/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package rdpstate

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/draw"
	"io"
	"math"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/proto"

	tdpbv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/desktop/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/srv/desktop/rdp/decoder"
)

const (
	// Legacy TDP message types relevant for recording replay.
	legacyTypeConnectionActivated = 31
	legacyTypeRDPFastPathPDU      = 29
	legacyTypeMouseMove           = 3

	// tdpbHeaderLength is the size of the TDPB message length prefix.
	tdpbHeaderLength = 4

	// maxMessageLength is the maximum allowed TDPB message body size (16 MiB - 1).
	maxMessageLength = (1 << 24) - 1

	// Linux desktop sessions don't run MCS, so they record ServerHello with
	// zero channel IDs. ironrdp-session's FastPath processor rejects zero
	// values and silently drops every surface update, so we substitute the
	// conventional RDP defaults when initializing the decoder.
	defaultIoChannelID   = 1003
	defaultUserChannelID = 1007
)

// RDPState reconstructs the screen state of a desktop session by processing a sequence of DesktopRecording events.
// It supports two recording formats:
//   - Legacy TDP (evt.Message): ConnectionActivated + RDPFastPathPDU messages, which are translated to modern TDPB
//     types before processing.
//   - Modern TDPB (evt.TDPBMessage): ServerHello + FastPathPDU messages.
//
// The RDP decoder is initialized on the first ServerHello (or its legacy equivalent, ConnectionActivated).
// Subsequent FastPathPDU messages are fed to the decoder, which maintains the screen framebuffer and cursor state.
type RDPState struct {
	decoder *decoder.Decoder

	mouseX, mouseY uint16
	hasMouse       bool
}

// New creates a new RDPState.
func New() *RDPState {
	return &RDPState{}
}

// HandleMessage processes a single DesktopRecording event.
// Events must be fed in recording order. Only one of evt.Message (legacy TDP) or evt.TDPBMessage (modern TDPB) should
// be set; if both are empty the call is a no-op.
func (s *RDPState) HandleMessage(evt *events.DesktopRecording) error {
	if len(evt.TDPBMessage) > 0 {
		return trace.Wrap(s.processTDPBMessage(evt.TDPBMessage), "processing TDPB message")
	}

	if len(evt.Message) > 0 {
		return trace.Wrap(s.processTDPMessage(evt.Message), "processing legacy TDP message")
	}

	return nil
}

// CursorState returns the current cursor visibility and position. If the decoder has not been initialized yet, it
// returns a default hidden cursor at (0, 0).
func (s *RDPState) CursorState() decoder.CursorState {
	if s.decoder == nil {
		return decoder.CursorState{}
	}

	return s.decoder.CursorState()
}

// Image returns the current screen image as an RGBA bitmap. If the decoder has not been initialized yet, it returns nil.
func (s *RDPState) Image() *image.RGBA {
	if s.decoder == nil {
		return nil
	}

	return s.decoder.Image()
}

// Release frees any resources associated with the RDPState, including the decoder.
func (s *RDPState) Release() {
	if s.decoder != nil {
		s.decoder.Release()
		s.decoder = nil
	}
}

// ImageWithCursor returns the current screen image with the cursor composited at its current position, along with the
// cursor state.
// When MouseMove TDPB messages have set a position, that overrides the Rust decoder's internal cursor position for compositing.
func (s *RDPState) ImageWithCursor() (*image.RGBA, decoder.CursorState) {
	if s.decoder == nil {
		return nil, decoder.CursorState{}
	}

	img := s.decoder.Image()
	cs := s.CursorState()
	if img == nil || !cs.Visible {
		return img, cs
	}

	bmp := s.decoder.CursorBitmap()
	if bmp == nil {
		return img, cs
	}

	cursorX, cursorY := cs.X, cs.Y
	if s.hasMouse {
		cursorX, cursorY = s.mouseX, s.mouseY
	}

	drawX := int(cursorX) - bmp.HotspotX
	drawY := int(cursorY) - bmp.HotspotY
	cb := bmp.Image.Bounds()

	dstRect := image.Rect(drawX, drawY, drawX+cb.Dx(), drawY+cb.Dy())
	draw.Draw(img, dstRect, bmp.Image, image.Point{}, draw.Over)

	return img, cs
}

// UpdatedRegions returns the individual screen regions updated since the last call to ResetUpdatedRegions.
func (s *RDPState) UpdatedRegions() []image.Rectangle {
	if s.decoder == nil {
		return nil
	}
	return s.decoder.UpdatedRegions()
}

// ResetUpdatedRegions clears the accumulated update regions.
func (s *RDPState) ResetUpdatedRegions() {
	if s.decoder != nil {
		s.decoder.ResetUpdatedRegions()
	}
}

type connectionActivated struct {
	IOChannelID, UserChannelID, ScreenWidth, ScreenHeight uint16
}

func (s *RDPState) processTDPMessage(data []byte) error {
	if len(data) == 0 {
		return trace.BadParameter("empty legacy TDP message")
	}

	msgType := data[0]
	r := bytes.NewReader(data[1:])

	switch msgType {
	case legacyTypeConnectionActivated:
		var ca connectionActivated
		if err := binary.Read(r, binary.BigEndian, &ca); err != nil {
			return trace.Wrap(err, "decoding legacy ConnectionActivated")
		}

		return s.handleServerHello(&tdpbv1.ServerHello{
			ActivationSpec: &tdpbv1.ConnectionActivated{
				IoChannelId:   uint32(ca.IOChannelID),
				UserChannelId: uint32(ca.UserChannelID),
				ScreenWidth:   uint32(ca.ScreenWidth),
				ScreenHeight:  uint32(ca.ScreenHeight),
			},
		})

	case legacyTypeRDPFastPathPDU:
		var dataLen uint32
		if err := binary.Read(r, binary.BigEndian, &dataLen); err != nil {
			return trace.Wrap(err, "reading legacy RDPFastPathPDU length")
		}

		if dataLen >= maxMessageLength {
			return trace.BadParameter("legacy RDPFastPathPDU length %d exceeds maximum %d", dataLen, maxMessageLength)
		}

		pdu := make([]byte, dataLen)
		if _, err := io.ReadFull(r, pdu); err != nil {
			return trace.Wrap(err, "reading legacy RDPFastPathPDU data")
		}

		return s.handleFastPathPDU(&tdpbv1.FastPathPDU{Pdu: pdu})

	case legacyTypeMouseMove:
		var mm struct{ X, Y uint32 }
		if err := binary.Read(r, binary.BigEndian, &mm); err != nil {
			return trace.Wrap(err, "reading legacy MouseMove")
		}

		return s.handleMouseMove(&tdpbv1.MouseMove{X: mm.X, Y: mm.Y})
	}

	return nil
}

func (s *RDPState) processTDPBMessage(data []byte) error {
	r := bytes.NewReader(data)

	header := make([]byte, tdpbHeaderLength)
	if _, err := io.ReadFull(r, header); err != nil {
		return trace.Wrap(err, "reading TDPB header")
	}

	msgLen := binary.BigEndian.Uint32(header)
	if msgLen >= maxMessageLength {
		return trace.BadParameter("TDPB message length %d exceeds maximum %d", msgLen, maxMessageLength)
	}

	msg := make([]byte, msgLen)
	if _, err := io.ReadFull(r, msg); err != nil {
		return trace.Wrap(err, "reading TDPB body")
	}

	env := &tdpbv1.Envelope{}
	if err := proto.Unmarshal(msg, env); err != nil {
		return trace.Wrap(err, "unmarshalling TDPB envelope")
	}

	switch m := env.Payload.(type) {
	case *tdpbv1.Envelope_ServerHello:
		return s.handleServerHello(m.ServerHello)
	case *tdpbv1.Envelope_FastPathPdu:
		return s.handleFastPathPDU(m.FastPathPdu)
	case *tdpbv1.Envelope_MouseMove:
		return s.handleMouseMove(m.MouseMove)
	}

	return nil
}

func (s *RDPState) handleServerHello(msg *tdpbv1.ServerHello) error {
	spec := msg.GetActivationSpec()
	if spec == nil {
		return nil
	}

	sw, sh := spec.GetScreenWidth(), spec.GetScreenHeight()
	if sw > types.MaxRDPScreenWidth || sh > types.MaxRDPScreenHeight {
		return trace.BadParameter("screen dimensions (%d x %d) exceed maximum (%d x %d)",
			sw, sh, types.MaxRDPScreenWidth, types.MaxRDPScreenHeight)
	}

	w := uint16(sw)
	h := uint16(sh)

	if w == 0 || h == 0 {
		return nil
	}

	if s.decoder == nil {
		ioChan, userChan := spec.GetIoChannelId(), spec.GetUserChannelId()
		if ioChan > math.MaxUint16 || userChan > math.MaxUint16 {
			return trace.BadParameter("channel IDs out of range: io=%d, user=%d", ioChan, userChan)
		}
		if ioChan == 0 && userChan == 0 {
			ioChan = defaultIoChannelID
			userChan = defaultUserChannelID
		}
		ioChannelID := uint16(ioChan)
		userChannelID := uint16(userChan)

		//nolint:staticcheck // err is always non-nil in nop build but nil in RDP build
		d, err := decoder.New(
			w, h,
			decoder.WithIOChannelID(ioChannelID),
			decoder.WithUserChannelID(userChannelID),
		)
		if err != nil { //nolint:staticcheck // err is always non-nil in nop build but nil in RDP build
			return trace.Wrap(err, "creating RDP decoder")
		}

		s.decoder = d
	} else {
		s.decoder.Resize(w, h)
	}

	return nil
}

func (s *RDPState) handleFastPathPDU(msg *tdpbv1.FastPathPDU) error {
	pdu := msg.GetPdu()
	if len(pdu) == 0 {
		return nil
	}

	if s.decoder == nil {
		return trace.BadParameter("received FastPathPDU before ServerHello initialized decoder")
	}

	s.decoder.Process(pdu)

	return nil
}

func (s *RDPState) handleMouseMove(msg *tdpbv1.MouseMove) error {
	if msg.X > math.MaxUint16 || msg.Y > math.MaxUint16 {
		return trace.BadParameter("mouse coordinates out of range: (%d, %d)", msg.X, msg.Y)
	}

	s.mouseX = uint16(msg.X)
	s.mouseY = uint16(msg.Y)
	s.hasMouse = true

	return nil
}
