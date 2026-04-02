/**
 * Teleport
 * Copyright (C) 2026 Gravitational, Inc.
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

// Package rdpstatetest provides shared test helpers for constructing
// DesktopRecording events and raw FastPath PDUs used in RDP-related tests.
package rdpstatetest

import (
	"testing"

	"github.com/stretchr/testify/require"

	tdpbv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/desktop/v1"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/srv/desktop/tdp/protocol/legacy"
	"github.com/gravitational/teleport/lib/srv/desktop/tdp/protocol/tdpb"
)

// TDPBEvent wraps raw TDPB-encoded bytes in a DesktopRecording event.
func TDPBEvent(data []byte) *apievents.DesktopRecording {
	return &apievents.DesktopRecording{TDPBMessage: data}
}

// LegacyEvent wraps raw legacy-TDP-encoded bytes in a DesktopRecording event.
func LegacyEvent(data []byte) *apievents.DesktopRecording {
	return &apievents.DesktopRecording{Message: data}
}

// EncodeTDPBServerHello creates a DesktopRecording event containing a TDPB ServerHello message with the given screen
// dimensions.
func EncodeTDPBServerHello(t *testing.T, width, height uint32) *apievents.DesktopRecording {
	t.Helper()

	data, err := (&tdpb.ServerHello{
		ActivationSpec: &tdpbv1.ConnectionActivated{
			ScreenWidth:  width,
			ScreenHeight: height,
		},
	}).Encode()
	require.NoError(t, err)

	return TDPBEvent(data)
}

// EncodeTDPBFastPathPDU creates a DesktopRecording event containing a TDPB FastPathPDU message wrapping the given raw
// PDU bytes.
func EncodeTDPBFastPathPDU(t *testing.T, pdu []byte) *apievents.DesktopRecording {
	t.Helper()

	data, err := (&tdpb.FastPathPDU{Pdu: pdu}).Encode()
	require.NoError(t, err)

	return TDPBEvent(data)
}

// LegacyConnectionActivated creates a DesktopRecording event containing a legacy ConnectionActivated message with the
// given screen dimensions.
func LegacyConnectionActivated(t *testing.T, width, height uint16) *apievents.DesktopRecording {
	t.Helper()

	data, err := legacy.ConnectionActivated{ScreenWidth: width, ScreenHeight: height}.Encode()
	require.NoError(t, err)

	return LegacyEvent(data)
}
