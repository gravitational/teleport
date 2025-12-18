package tdp

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"io"
	"math"
	"net"
	"testing"

	apiProto "github.com/gravitational/teleport/api/client/proto"
	tdpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/desktop/v1"
	tdpbv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/desktop/v1"
	mfav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v1"
	webauthnpb "github.com/gravitational/teleport/api/types/webauthn"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/stretchr/testify/assert"
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
	// This succeeds because it's convenient for this function
	// to be infallible. It should instead fail when we try
	// to encode it.
	tdpbMsg := NewTDPBMessage(msg)

	// Encode fails
	_, err := tdpbMsg.Encode()
	require.Error(t, err)

	// Try encoding a non-TDPB protobuf
	metadata := &headerv1.Metadata{}
	tdpbMsg = NewTDPBMessage(metadata)
	// Fails because we cannot determine the message type.
	// Non-TDPB messages do not have the type option
	// used to identy TDPB message protos.
	_, err = tdpbMsg.Encode()
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
		{Name: "decoded-message-happy-path", Message: decodedMessage},
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
		mouseMove := &tdpbv1.MouseMove{X: 1, Y: 2}
		raw := NewTDPBMessage(mouseMove)
		// message is a perfectly valid, encoded TDPB message
		message, err := raw.Encode()
		require.NoError(t, err)

		// Corrupt the payload, but leave the valid header intact.
		offset := 8
		for idx := range message[8:] {
			message[idx+offset] = 0xFF
		}

		// DecodeTDPB doesn't fail because it lazily decodes the underlying proto!
		badMsg, err := DecodeTDPB(bytes.NewReader(message))
		require.NoError(t, err)

		// Inspecting the proto will result in an error
		_, err = ToTDPBProto(badMsg)
		require.Error(t, err)

		// Other methods of inspection also fail
		newMouseMove := &tdpbv1.MouseMove{}
		require.Error(t, AsTDPB(badMsg, newMouseMove))
	})
}

func TestHandleUnknownMessageTypes(t *testing.T) {
	// The decoder should quietly ignore unknown message types.
	// This gives us flexibility to add new TDPB messages later on
	// without fear of breaking old implementations.
	goodMessage, err := NewTDPBMessage(&tdpbv1.MouseMove{X: 1, Y: 2}).Encode()
	require.NoError(t, err)

	// Make a copy of the valid message, but change the MessageType
	// in the header to an invalid message type.
	badMessage := make([]byte, len(goodMessage))
	copy(badMessage, goodMessage)
	binary.BigEndian.PutUint32(badMessage[:4], math.MaxUint32)

	// Write the bad message first, then a good message
	buf := bytes.NewBuffer(badMessage)
	_, _ = buf.Write(goodMessage) /* infallible write */

	msg, err := DecodeTDPB(buf)
	// Decode should succeed and we should get the
	// valid MouseMove message. The unknown message type
	// is quietly ignore and its message body discarded.
	require.NoError(t, err)
	move := &tdpbv1.MouseMove{}
	require.NoError(t, AsTDPB(msg, move))
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
	rdr := bufio.NewReader(reader)
	for {
		msg, err := DecodeTDPB(rdr)
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

func TestTDPBMFAFlow(t *testing.T) {
	client, server := net.Pipe()
	clientConn := NewConn(client, WithTDPBDecoder())
	serverConn := NewConn(server, WithTDPBDecoder())
	defer clientConn.Close()
	defer serverConn.Close()

	witheld := []Message{}
	promptFn := NewTDPBMFAPrompt(serverConn, &witheld)("channel_id")
	requestMsg := &apiProto.MFAAuthenticateChallenge{
		WebauthnChallenge: &webauthnpb.CredentialAssertion{
			PublicKey: &webauthnpb.PublicKeyCredentialRequestOptions{
				Challenge: []byte("Some challenge"),
				TimeoutMs: 1000,
				RpId:      "teleport",
				AllowCredentials: []*webauthnpb.CredentialDescriptor{
					{Type: "some device", Id: []byte("1234")},
				},
			},
		},
	}

	type result struct {
		response *apiProto.MFAAuthenticateResponse
		err      error
	}

	done := make(chan result)
	go func() {
		response, err := promptFn(t.Context(), requestMsg)
		done <- result{response, err}
	}()

	// Simulate the client
	mfaMessage := expectTDPBMessage[*tdpbv1.MFA](t, clientConn)
	// Validate the received MFA challenge matches wahat was sent
	assert.Equal(t, requestMsg.WebauthnChallenge, mfaMessage.Challenge.WebauthnChallenge)
	assert.Equal(t, "channel_id", mfaMessage.ChannelId)

	// Send a random, non-MFA TDPB message
	require.NoError(t, clientConn.WriteMessage(NewTDPBMessage(&tdpbv1.Alert{Message: "random message!"})))

	response := &mfav1.AuthenticateResponse{
		Response: &mfav1.AuthenticateResponse_Webauthn{
			Webauthn: &webauthnpb.CredentialAssertionResponse{
				Type:  "sometype",
				RawId: []byte("rawid"),
				Response: &webauthnpb.AuthenticatorAssertionResponse{
					ClientDataJson: []byte(`{"data": "value"}`),
					Signature:      []byte("john hancock"),
				},
			},
		},
	}
	// Send response
	err := clientConn.WriteMessage(
		NewTDPBMessage(&tdpbv1.MFA{
			AuthenticationResponse: response,
		}),
	)
	require.NoError(t, err)
	// Wait for MFA flow to complete and return the response
	res := <-done
	require.NoError(t, res.err)
	// Response should match what was sent
	assert.Equal(t, response.GetWebauthn(), res.response.GetWebauthn())
	// Should still have that alert message in our withheld message slice
	assert.Len(t, witheld, 1)
}

func expectTDPBMessage[T any](t *testing.T, c *Conn) T {
	var zero T
	msg, err := c.ReadMessage()
	require.NoError(t, err)

	proto, err := ToTDPBProto(msg)
	require.NoError(t, err)

	require.IsType(t, proto, zero)
	return proto.(T)
}
