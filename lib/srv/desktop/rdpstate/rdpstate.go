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
	"image"
	"sync"

	"github.com/gravitational/trace"

	tdpbv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/desktop/v1"
	"github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/srv/desktop/rdp/decoder"
	"github.com/gravitational/teleport/lib/srv/desktop/tdp"
	"github.com/gravitational/teleport/lib/srv/desktop/tdp/protocol/legacy"
	"github.com/gravitational/teleport/lib/srv/desktop/tdp/protocol/tdpb"
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
	mu      sync.Mutex
	decoder *decoder.Decoder
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
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.decoder == nil {
		return decoder.CursorState{}
	}

	return s.decoder.CursorState()
}

// Image returns the current screen image as an RGBA bitmap. If the decoder has not been initialized yet, it returns nil.
func (s *RDPState) Image() *image.RGBA {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.decoder == nil {
		return nil
	}

	return s.decoder.Image()
}

// Release frees any resources associated with the RDPState, including the decoder.
func (s *RDPState) Release() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.decoder != nil {
		s.decoder.Release()
		s.decoder = nil
	}
}

func (s *RDPState) processTDPMessage(data []byte) error {
	msg, err := legacy.Decode(bytes.NewReader(data))
	if err != nil {
		return trace.Wrap(err, "decoding legacy TDP message")
	}

	msgs, err := tdpb.TranslateToModern(msg)
	if err != nil {
		return trace.Wrap(err, "translating legacy TDP message")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, m := range msgs {
		if err := s.handleMessage(m); err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}

func (s *RDPState) processTDPBMessage(data []byte) error {
	msg, err := tdpb.DecodeStrict(bytes.NewReader(data))
	if err != nil {
		return trace.Wrap(err, "decoding TDPB message")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	return s.handleMessage(msg)
}

func (s *RDPState) handleMessage(msg tdp.Message) error {
	switch m := msg.(type) {
	case *tdpb.ServerHello:
		return s.handleServerHello((*tdpbv1.ServerHello)(m))

	case *tdpb.FastPathPDU:
		return s.handleFastPathPDU((*tdpbv1.FastPathPDU)(m))
	}

	return nil
}

func (s *RDPState) handleServerHello(msg *tdpbv1.ServerHello) error {
	spec := msg.GetActivationSpec()
	if spec == nil {
		return nil
	}

	w := uint16(spec.GetScreenWidth())
	h := uint16(spec.GetScreenHeight())

	if w == 0 || h == 0 {
		return nil
	}

	if s.decoder == nil {
		d, err := decoder.New(w, h)
		if err != nil {
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
