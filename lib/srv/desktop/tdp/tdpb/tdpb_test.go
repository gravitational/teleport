package tdpb

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"io"
	"net"
	"testing"

	"github.com/google/go-cmp/cmp"
	apiProto "github.com/gravitational/teleport/api/client/proto"
	tdpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/desktop/v1"
	tdpbv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/desktop/v1"
	mfav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v1"
	webauthnpb "github.com/gravitational/teleport/api/types/webauthn"
	"github.com/gravitational/teleport/lib/srv/desktop/tdp"
	"google.golang.org/protobuf/testing/protocmp"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
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

	decodedMessage, err := Decode(bytes.NewReader(data))
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
		badMsg, err := Decode(bytes.NewReader(message))
		require.Error(t, err)
		require.Nil(t, badMsg)
	})
}

func TestHandleUnknownMessageTypes(t *testing.T) {
	// Craft an empty envelope
	msg := &tdpb.Envelope{}
	data, err := proto.Marshal(msg)
	require.NoError(t, err)
	msgBuffer := make([]byte, len(data)+4)
	binary.BigEndian.PutUint32(msgBuffer, uint32(len(data)))
	copy(msgBuffer[4:], data)

	// Write the bad message first, then a good message
	buf := bytes.NewBuffer(msgBuffer)

	// Should return a sentinel error indicating that
	// an empty message was received.
	decoded, err := Decode(buf)
	require.ErrorIs(t, err, ErrEmptyMessage)
	require.Nil(t, decoded)
}

func TestSendRecv(t *testing.T) {
	// Define a few messages to encode and decode
	alertMsg := &Alert{
		Message:  "Warning!",
		Severity: tdpb.AlertSeverity_ALERT_SEVERITY_WARNING,
	}
	fastPathMsg := &FastPathPDU{
		Pdu: []byte{0xDE, 0xCA, 0xFB, 0xAD},
	}
	helloMsg := &ClientHello{
		ScreenSpec: &tdpb.ClientScreenSpec{
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
		msg, err := Decode(rdr)
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

func TestTDPBMFAFlow(t *testing.T) {
	client, server := net.Pipe()
	clientConn := tdp.NewConn(client, tdp.WithDecoder(Decode))
	serverConn := tdp.NewConn(server, tdp.WithDecoder(Decode))
	defer clientConn.Close()
	defer serverConn.Close()

	witheld := []tdp.Message{}
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
	mfaMessage := expectTDPBMessage[*MFA](t, clientConn)
	// Validate the received MFA challenge matches wahat was sent
	assert.Equal(t, requestMsg.WebauthnChallenge, mfaMessage.Challenge.WebauthnChallenge)
	assert.Equal(t, "channel_id", mfaMessage.ChannelId)

	// Send a random, non-MFA TDPB message
	require.NoError(t, clientConn.WriteMessage(&Alert{Message: "random message!"}))

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
		&MFA{
			AuthenticationResponse: response,
		},
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

func expectTDPBMessage[T any](t *testing.T, c *tdp.Conn) T {
	var zero T
	msg, err := c.ReadMessage()
	require.NoError(t, err)

	require.IsType(t, msg, zero)
	return msg.(T)
}
