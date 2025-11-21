package tdp

import (
	"bytes"
	"io"
	"net"
	"testing"

	tdpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/desktop/v1"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

func TestMessageType(t *testing.T) {
	var msg proto.Message
	// Should marshal successfully
	msg = &tdpb.ClientHello{}
	require.Equal(t, tdpb.MessageType_MESSAGE_TYPE_CLIENT_HELLO, getMessageType(msg))

	// Not wire compatible (does not have a 'tdp_type_option')
	msg = &tdpb.Rectangle{}
	require.Equal(t, tdpb.MessageType_MESSAGE_TYPE_UNSPECIFIED, getMessageType(msg))

	// nil should not panic
	msg = nil
	require.Equal(t, tdpb.MessageType_MESSAGE_TYPE_UNSPECIFIED, getMessageType(msg))
}

func TestInvalidEncode(t *testing.T) {
	// Rectangle is not allowed to be encoded
	// as it's not a full fledged "Message Type"
	msg := &tdpb.Rectangle{}
	// This works because it's convenient for this function
	// to be infallible. It should instead fail when we try
	// to encode it.
	tdpbMsg := NewTDPBMessage(msg)

	// Encode fails
	_, err := tdpbMsg.Encode()
	require.Error(t, err)

	// Similarly, WriteTo fails
	_, err = tdpbMsg.WriteTo(&bytes.Buffer{})
	require.Error(t, err)
}

func TestSendRecv(t *testing.T) {
	// Define a few messages to encode and decode
	messages := []proto.Message{
		&tdpb.Alert{
			Message:  "Warning!",
			Severity: tdpb.AlertSeverity_ALERT_SEVERITY_WARNING,
		},
		&tdpb.FastPathPDU{
			Pdu: []byte{0xDE, 0xCA, 0xFB, 0xAD},
		},
		&tdpb.ClientHello{
			ScreenSpec: &tdpb.ClientScreenSpec{
				Width:  1920,
				Height: 1080,
			},
			KeyboardLayout: 1,
		},
	}

	writer, reader := net.Pipe()
	// Write/Encode messages through one side of the pipe
	var writeError error
	go func() {
		defer writer.Close()
		for _, msg := range messages {
			out := NewTDPBMessage(msg)
			_, writeError = out.WriteTo(writer)
			if writeError != nil {
				return
			}
		}
	}()

	// Read/Decode message from the other side of the pipe
	idx := 0
	for {
		msg, err := DecodeTDPB(reader)
		if err != nil {
			require.ErrorIs(t, err, io.EOF)
			break
		}

		protoMsg, err := msg.Proto()
		require.NoError(t, err)
		require.True(t, proto.Equal(messages[idx], protoMsg))
		idx++
	}

	// Should not have received a write error
	require.NoError(t, writeError)
}
