/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package tdpb

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"io"
	"net"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"

	tdpbv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/desktop/v1"
	"github.com/gravitational/teleport/lib/srv/desktop/tdp"
	"github.com/gravitational/teleport/lib/srv/desktop/tdp/protocol/legacy"
)

func TestEncodeDecode(t *testing.T) {
	png := &PNGFrame{
		Coordinates: &tdpbv1.Rectangle{
			Top:    1,
			Bottom: 2,
			Left:   3,
			Right:  4,
		},
		Data: []byte{0xDE, 0xCA, 0xFB, 0xAD},
	}
	data, err := png.Encode()
	require.NoError(t, err)

	decodedMessage, err := DecodeStrict(bytes.NewReader(data))
	require.NoError(t, err)

	// Test both "raw" and "decoded" messages.
	// "raw" messages are plain tdpb protobuf messages wrapped in a 'TdpbMessage' struct
	// "decoded" messages are simply a byte buffer containing TDPB messages headers and
	//  unmarshalled protobuf data, also wrapped in a 'TdpbMessage' struct.
	//  raw messages are lazily encoded (when their 'encode' method is called), and
	//  decodes messages are lazily unmarshalled when inspected with 'ToTDPBProto', or 'AsTDPB'.
	for _, test := range []struct {
		Name    string
		Message tdp.Message
	}{
		{Name: "raw-message-happy-path", Message: png},
		{Name: "decoded-message-happy-path", Message: decodedMessage},
	} {
		t.Run(test.Name, func(t *testing.T) {
			object, ok := test.Message.(*PNGFrame)
			require.True(t, ok)
			require.Empty(t, cmp.Diff((*tdpbv1.PNGFrame)(object), (*tdpbv1.PNGFrame)(object), protocmp.Transform()))
		})
	}

	t.Run("decode-corrupted-message", func(t *testing.T) {
		mouseMove := &MouseMove{X: 1, Y: 2}
		// message is a perfectly valid, encoded TDPB message
		message, err := mouseMove.Encode()
		require.NoError(t, err)

		// Corrupt the payload, but leave the valid header intact.
		offset := 4
		for idx := range message[offset:] {
			message[idx+offset] = 0xFF
		}

		// Decode fails because we can't unmarshal the envelope
		badMsg, err := DecodeStrict(bytes.NewReader(message))
		require.Error(t, err)
		require.Nil(t, badMsg)
	})
}

func TestHandleUnknownMessageTypes(t *testing.T) {
	// Craft an empty envelope
	data, err := proto.Marshal(&tdpbv1.Envelope{})
	require.NoError(t, err)

	innerData, err := proto.Marshal(&tdpbv1.Rectangle{Top: 10, Bottom: 100, Left: 1234, Right: 4321})
	require.NoError(t, err)

	// Append an unknown field number, 1000
	tag := protowire.AppendTag(nil, 1000, protowire.BytesType)
	// Append some phony message data (but make sure it conforms to the wire format)
	payload := protowire.AppendBytes(nil, innerData)

	badMsg := append(data, tag...)
	badMsg = append(badMsg, payload...)

	badMsgBuffer := make([]byte, len(badMsg)+4)
	binary.BigEndian.PutUint32(badMsgBuffer, uint32(len(badMsg)))
	copy(badMsgBuffer[4:], badMsg)

	t.Run("strict-decode-returns-error", func(t *testing.T) {
		// Write the bad message first, then a good message
		buf := bytes.NewBuffer(badMsgBuffer)

		// Should return a sentinel error indicating that
		// an empty message was received.
		decoded, err := DecodeStrict(buf)
		require.ErrorIs(t, err, ErrUnknownMessage)
		require.Nil(t, decoded)
	})

	t.Run("permissive-decode-drops-bad-message", func(t *testing.T) {
		buf := bytes.NewBuffer(nil)

		// Write the bad message
		_, err := buf.Write(badMsgBuffer)
		require.NoError(t, err)

		// Then write a good message
		msg := &MouseButton{Pressed: true, Button: 2}
		data, err := msg.Encode()
		require.NoError(t, err)
		_, err = buf.Write(data)
		require.NoError(t, err)

		// Validate that good message is received without error
		// (bad message dropped)
		newMsg, err := DecodePermissive(buf)
		require.NoError(t, err)
		_, ok := newMsg.(*MouseButton)
		require.True(t, ok)
	})

	t.Run("tdp-discard", func(t *testing.T) {
		data, err := legacy.Alert{Message: "alert!", Severity: legacy.SeverityError}.Encode()
		require.NoError(t, err)

		_, err = DecodeWithTDPDiscard(data)
		require.ErrorIs(t, err, ErrIsTDP)
	})
}

func TestSendRecv(t *testing.T) {
	// Define a few messages to encode and decode
	alertMsg := &Alert{
		Message:  "Warning!",
		Severity: tdpbv1.AlertSeverity_ALERT_SEVERITY_WARNING,
	}
	fastPathMsg := &FastPathPDU{
		Pdu: []byte{0xDE, 0xCA, 0xFB, 0xAD},
	}
	helloMsg := &ClientHello{
		ScreenSpec: &tdpbv1.ClientScreenSpec{
			Width:  1920,
			Height: 1080,
		},
		KeyboardLayout: 1,
	}

	messages := []tdp.Message{
		alertMsg,
		fastPathMsg,
		helloMsg,
	}

	writer, reader := net.Pipe()
	// Write/Encode messages through one side of the pipe
	var writeError error
	go func() {
		defer writer.Close()
		for _, msg := range messages {
			data, err := msg.Encode()
			if err != nil {
				writeError = err
				return
			}

			_, writeError = writer.Write(data)
			if writeError != nil {
				return
			}
		}
	}()

	// Read/Decode message from the other side of the pipe
	idx := 0
	rdr := bufio.NewReader(reader)
	for {
		msg, err := DecodeStrict(rdr)
		if err != nil {
			require.ErrorIs(t, err, io.EOF)
			break
		}

		switch m := msg.(type) {
		case *Alert:
			require.Empty(t, cmp.Diff((*tdpbv1.Alert)(alertMsg), (*tdpbv1.Alert)(m), protocmp.Transform()))
		case *FastPathPDU:
			require.Empty(t, cmp.Diff((*tdpbv1.FastPathPDU)(fastPathMsg), (*tdpbv1.FastPathPDU)(m), protocmp.Transform()))
		case *ClientHello:
			require.Empty(t, cmp.Diff((*tdpbv1.ClientHello)(helloMsg), (*tdpbv1.ClientHello)(m), protocmp.Transform()))
		}
		idx++
	}
	_ = reader.Close()

	// Should not have received a write error
	require.NoError(t, writeError)
}
