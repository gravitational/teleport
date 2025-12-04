package tdp

import (
	"bytes"
	"io"
	"net"
	"testing"

	tdpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/desktop/v1"
	tdpbv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/desktop/v1"
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
}

func TestMessageOperations(t *testing.T) {
	png := &tdpbv1.PNGFrame{
		Coordinates: &tdpbv1.Rectangle{
			Top:    1,
			Bottom: 2,
			Left:   3,
			Right:  4,
		},
		Data: []byte{0xDE, 0xCA, 0xFB, 0xAD},
	}
	rawMessage := NewTDPBMessage(png)
	data, err := rawMessage.Encode()
	require.NoError(t, err)

	decodedMessage, err := DecodeTDPB(bytes.NewReader(data))
	require.NoError(t, err)

	// Test both "raw" and "decoded" messages.
	// "raw" messages are plain tdpb protobuf messages wrapped in a 'TdpbMessage' struct
	// "decoded" messages are simply a byte buffer containing TDPB messages headers and
	//  unmarshalled protobuf data, also wrapped in a 'TdpbMessage' struct.
	//  raw messages are lazily encoded (when their 'encode' method is called), and
	//  decodes messages are lazily unmarshalled when inspected with 'ToTDPBProto', or 'AsTDPB'.
	for _, test := range []struct {
		Name    string
		Message Message
	}{
		{Name: "raw-message-happy-path", Message: rawMessage},
		{Name: "decoded-message-path", Message: &decodedMessage},
	} {
		t.Run(test.Name, func(t *testing.T) {
			protoMsg, err := ToTDPBProto(test.Message)
			require.NoError(t, err)

			object, ok := protoMsg.(*tdpbv1.PNGFrame)
			require.True(t, ok)
			require.True(t, proto.Equal(object, png))

			// Alternatively, use AsTDPB
			object = &tdpbv1.PNGFrame{}
			require.NoError(t, AsTDPB(test.Message, object))
			require.True(t, proto.Equal(object, png))
		})
	}

	t.Run("decode-corrupted-message", func(t *testing.T) {
		validMessage, err := rawMessage.Encode()
		require.NoError(t, err)

		corruptedMessage := bytes.NewBuffer(validMessage[:8])
		_, _ = corruptedMessage.WriteString("this is not a valid protobuf message!")

		// DecodeTDPB doesn't fail because it lazily decodes the underlying proto!
		badMsg, err := DecodeTDPB(corruptedMessage)
		require.NoError(t, err)

		// Inspecting the proto will result in an error
		_, err = ToTDPBProto(&badMsg)
		require.Error(t, err)

	})
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
			writeError = out.EncodeTo(writer)
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
	_ = reader.Close()

	// Should not have received a write error
	require.NoError(t, writeError)
}
