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
	"encoding/binary"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	tdpbv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/desktop/v1"
	"github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/srv/desktop/rdpstate/rdpstatetest"
)

// These tests cover message routing, error handling, and edge cases that don't require a real RDP decoder.
// Tests that process actual FastPath PDUs and verify decoded image content are in rdpstate_rdp_test.go (build-tagged).

func TestHandleMessage_EmptyEvent(t *testing.T) {
	s := New()
	require.NoError(t, s.HandleMessage(&events.DesktopRecording{}))
	require.Nil(t, s.decoder)
}

func TestHandleMessage_InvalidData(t *testing.T) {
	for _, tt := range []struct {
		name string
		evt  *events.DesktopRecording
	}{
		{"invalid TDPB", rdpstatetest.TDPBEvent([]byte{0xFF, 0xFF, 0xFF})},
		{"truncated legacy", rdpstatetest.LegacyEvent([]byte{29})}, // RDPFastPathPDU type with no payload
	} {
		t.Run(tt.name, func(t *testing.T) {
			require.Error(t, New().HandleMessage(tt.evt))
		})
	}
}

func TestServerHello_NoOp(t *testing.T) {
	for _, tt := range []struct {
		name string
		evt  *events.DesktopRecording
	}{
		{"zero width", encodeTDPBServerHello(t, 0, 600)},
		{"zero height", encodeTDPBServerHello(t, 800, 0)},
		{"legacy zero width", legacyConnectionActivated(t, 0, 600)},
		{"legacy zero height", legacyConnectionActivated(t, 800, 0)},
	} {
		t.Run(tt.name, func(t *testing.T) {
			s := New()

			require.NoError(t, s.HandleMessage(tt.evt))
			require.Nil(t, s.decoder)
		})
	}
}

func TestFastPathPDU_BeforeServerHello(t *testing.T) {
	s := New()
	err := s.HandleMessage(encodeTDPBFastPathPDU(t, []byte{0xDE, 0xAD}))

	require.True(t, trace.IsBadParameter(err), "expected BadParameter, got: %v", err)
}

func TestFastPathPDU_EmptyPDU(t *testing.T) {
	s := New()

	require.NoError(t, s.HandleMessage(encodeTDPBFastPathPDU(t, nil)))
	require.Nil(t, s.decoder)
}

func TestUnknownMessage_Ignored(t *testing.T) {
	s := New()

	// Encode a SyncKeys message (not handled by RDPState) as a TDPB envelope.
	body, err := proto.Marshal(&tdpbv1.Envelope{
		Payload: &tdpbv1.Envelope_SyncKeys{SyncKeys: &tdpbv1.SyncKeys{}},
	})
	require.NoError(t, err)
	data := make([]byte, 4+len(body))
	binary.BigEndian.PutUint32(data[:4], uint32(len(body)))
	copy(data[4:], body)

	require.NoError(t, s.HandleMessage(rdpstatetest.TDPBEvent(data)))
	require.Nil(t, s.decoder)
}

func encodeTDPBServerHello(t *testing.T, width, height uint32) *events.DesktopRecording {
	t.Helper()

	evt, err := rdpstatetest.EncodeTDPBServerHello(width, height)
	require.NoError(t, err)

	return evt
}

func encodeTDPBFastPathPDU(t *testing.T, pdu []byte) *events.DesktopRecording {
	t.Helper()

	evt, err := rdpstatetest.EncodeTDPBFastPathPDU(pdu)
	require.NoError(t, err)

	return evt
}

func legacyConnectionActivated(t *testing.T, width, height uint16) *events.DesktopRecording {
	t.Helper()

	evt, err := rdpstatetest.LegacyConnectionActivated(width, height)
	require.NoError(t, err)

	return evt
}
