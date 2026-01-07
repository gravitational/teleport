package tdpb

import (
	"log/slog"
	"net"
	"testing"

	"github.com/gravitational/teleport/api/client/proto"
	mfav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v1"
	webauthnpb "github.com/gravitational/teleport/api/types/webauthn"
	"github.com/gravitational/teleport/lib/srv/desktop/tdp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTDPBMFAFlow(t *testing.T) {
	client, server := net.Pipe()
	clientConn := tdp.NewConn(client, tdp.DecoderAdapter(DecodeStrict))
	serverConn := tdp.NewConn(server, tdp.DecoderAdapter(DecodeStrict))
	defer clientConn.Close()
	defer serverConn.Close()

	withheld := []tdp.Message{}
	promptFn := NewTDPBMFAPrompt(serverConn, &withheld, slog.Default())("channel_id")
	requestMsg := &proto.MFAAuthenticateChallenge{
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
		response *proto.MFAAuthenticateResponse
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
	assert.Len(t, withheld, 1)
}

func expectTDPBMessage[T any](t *testing.T, c *tdp.Conn) T {
	var zero T
	msg, err := c.ReadMessage()
	require.NoError(t, err)

	require.IsType(t, msg, zero)
	return msg.(T)
}
