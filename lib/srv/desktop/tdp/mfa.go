package tdp

import (
	"context"
	"errors"

	"github.com/gravitational/teleport/api/client/proto"
	tdpbv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/desktop/v1"
	"github.com/gravitational/teleport/api/mfa"
	"github.com/gravitational/teleport/lib/auth/authclient"
	wantypes "github.com/gravitational/teleport/lib/auth/webauthntypes"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/trace"
)

type MessageReadWriter interface {
	ReadMessage() (Message, error)
	WriteMessage(Message) error
}

// convertChallenge converts an MFA challenge to a Message. Returns
// a non-nil error if the conversion fails
type convertChallenge func(*proto.MFAAuthenticateChallenge) (Message, error)

// isMFAResponse returns:
//   - ErrUnexpectedMessageType if a valid messages was received but was not an MFA message.
//   - Any other non-nil error if there was an error intepreeting the message.
//   - nil if a valid, non-nil MFA messages was found.
type isMFAResponse func(Message) (*proto.MFAAuthenticateResponse, error)

// newMfaPrompt constructs a function that reads encodes and sends an MFA challenge to the client
// and waits for the corresponding MFA response message. It caches any non-MFA messages received so
// that they may be forwarded to the server later on.
func newMfaPrompt(rw MessageReadWriter, isResponse isMFAResponse, toMessage convertChallenge, withheld *[]Message) func(ctx context.Context, chal *proto.MFAAuthenticateChallenge) (*proto.MFAAuthenticateResponse, error) {
	return func(ctx context.Context, chal *proto.MFAAuthenticateChallenge) (*proto.MFAAuthenticateResponse, error) {
		challengeMsg, err := toMessage(chal)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		if err = rw.WriteMessage(challengeMsg); err != nil {
			return nil, trace.Wrap(err)
		}

		for {
			msg, err := rw.ReadMessage()
			if err != nil {
				return nil, trace.Wrap(err)
			}

			resp, err := isResponse(msg)
			if err != nil {
				if errors.Is(err, ErrUnexpectedMessageType) {
					// Withhold this non-MFA message and try reading again
					*withheld = append(*withheld, msg)
					continue
				} else {
					// Unexpected error occurred while inspecting the message
					return nil, trace.Wrap(err)
				}
			}
			// Found our MFA response!
			return resp, nil
		}
	}
}

// Handle TDP MFA ceremony
func NewTDPMFAPrompt(rw MessageReadWriter, withheld *[]Message) func(string) mfa.PromptFunc {
	return func(channelID string) mfa.PromptFunc {
		convert := func(chal *proto.MFAAuthenticateChallenge) (Message, error) {
			// Convert from proto to JSON types.
			var challenge client.MFAAuthenticateChallenge
			if chal.WebauthnChallenge != nil {
				challenge.WebauthnChallenge = wantypes.CredentialAssertionFromProto(chal.WebauthnChallenge)
			}

			if chal.SSOChallenge != nil {
				challenge.SSOChallenge = client.SSOChallengeFromProto(chal.SSOChallenge)
				challenge.SSOChallenge.ChannelID = channelID
			}

			if chal.WebauthnChallenge == nil && chal.SSOChallenge == nil {
				return nil, trace.Wrap(authclient.ErrNoMFADevices)
			}

			tdpMsg := &MFA{
				Type:                     defaults.WebsocketMFAChallenge[0],
				MFAAuthenticateChallenge: &challenge,
			}
			return tdpMsg, nil
		}

		isResponse := func(msg Message) (*proto.MFAAuthenticateResponse, error) {
			switch t := msg.(type) {
			case MFA:
				return t.MFAAuthenticateResponse, nil
			default:
				return nil, ErrUnexpectedMessageType
			}
		}

		return newMfaPrompt(rw, isResponse, convert, withheld)
	}
}

// Handle TDPB MFA ceremony
func NewTDPBMFAPrompt(rw MessageReadWriter, withheld *[]Message) func(string) mfa.PromptFunc {
	return func(channelID string) mfa.PromptFunc {
		convert := func(challenge *proto.MFAAuthenticateChallenge) (Message, error) {

			return NewTDPBMessage(&tdpbv1.MFA{
				Challenge: challenge,
			}), nil
		}

		isResponse := func(msg Message) (*proto.MFAAuthenticateResponse, error) {
			mfaMsg := &tdpbv1.MFA{}
			// Is this an MFA message?
			if err := AsTDPB(msg, mfaMsg); err != nil {
				if mfaMsg.AuthenticationResponse == nil {
					return nil, trace.Errorf("MFA response is empty")
				}
				return mfaMsg.AuthenticationResponse, nil
			}

			*withheld = append(*withheld, msg)
			return nil, ErrUnexpectedMessageType
		}

		return newMfaPrompt(rw, isResponse, convert, withheld)
	}
}
