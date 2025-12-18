package tdp

import (
	"context"
	"errors"
	"log/slog"

	"github.com/gravitational/teleport/api/client/proto"
	tdpbv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/desktop/v1"
	mfav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v1"
	"github.com/gravitational/teleport/api/mfa"
	"github.com/gravitational/teleport/lib/auth/authclient"
	wantypes "github.com/gravitational/teleport/lib/auth/webauthntypes"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/trace"
)

// convertChallenge converts an MFA challenge to a Message. Returns
// a non-nil error if the conversion fails
type convertChallenge func(*proto.MFAAuthenticateChallenge) (Message, error)

// isMFAResponse returns:
//   - ErrUnexpectedMessageType if a valid messages was received but was not an MFA message.
//   - Any other non-nil error if there was an error intepreeting the message.
//   - nil if a valid, non-nil MFA messages was found.
type isMFAResponse func(Message) (*proto.MFAAuthenticateResponse, error)

// newMfaPrompt constructs a function that reads, encodes, and sends an MFA challenge to the client,
// then waits for the corresponding MFA response message. It caches any non-MFA messages received so
// that they may be forwarded to the server later on.
func newMfaPrompt(rw MessageReadWriter, isResponse isMFAResponse, toMessage convertChallenge, withheld *[]Message) mfa.PromptFunc {
	return func(ctx context.Context, chal *proto.MFAAuthenticateChallenge) (*proto.MFAAuthenticateResponse, error) {
		challengeMsg, err := toMessage(chal)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		slog.DebugContext(ctx, "Writing MFA challenge to client")
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
					slog.DebugContext(ctx, "Received non-MFA message", "message", msg)
					*withheld = append(*withheld, msg)
					continue
				} else {
					slog.DebugContext(ctx, "Error receiving MFA response", "error", err)
					// Unexpected error occurred while inspecting the message
					return nil, trace.Wrap(err)
				}
			}
			// Found our MFA response!
			slog.DebugContext(ctx, "Received MFA response")
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

			if chal.WebauthnChallenge == nil && chal.SSOChallenge == nil && chal.TOTP == nil {
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
			case *MFA:
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
			if challenge == nil {
				return nil, errors.New("empty MFA challenge")
			}

			mfaMsg := &tdpbv1.MFA{
				ChannelId: channelID,
			}

			if challenge.WebauthnChallenge != nil {
				mfaMsg.Challenge = &mfav1.AuthenticateChallenge{
					WebauthnChallenge: challenge.WebauthnChallenge,
				}
			}

			if challenge.SSOChallenge != nil {
				mfaMsg.Challenge = &mfav1.AuthenticateChallenge{
					SsoChallenge: &mfav1.SSOChallenge{
						RequestId:   challenge.SSOChallenge.RequestId,
						RedirectUrl: challenge.SSOChallenge.RedirectUrl,
						Device:      challenge.SSOChallenge.Device,
					},
				}
			}

			if challenge.WebauthnChallenge == nil && challenge.SSOChallenge == nil && challenge.TOTP == nil {
				return nil, trace.Wrap(authclient.ErrNoMFADevices)
			}

			return NewTDPBMessage(mfaMsg), nil
		}

		isResponse := func(msg Message) (*proto.MFAAuthenticateResponse, error) {
			mfaMsg := &tdpbv1.MFA{}
			// Is this an MFA message?
			if err := AsTDPB(msg, mfaMsg); err != nil {
				// AsTDPB returns the well-known ErrUnexpectedMessageType error
				// if we got a valid TDPB message, but it wasn't an MFA message specifically.
				return nil, trace.Wrap(err)
			}

			if mfaMsg.AuthenticationResponse == nil {
				return nil, trace.Errorf("MFA response is empty")
			}

			switch response := mfaMsg.AuthenticationResponse.Response.(type) {
			case *mfav1.AuthenticateResponse_Sso:
				return &proto.MFAAuthenticateResponse{
					Response: &proto.MFAAuthenticateResponse_SSO{
						SSO: &proto.SSOResponse{
							RequestId: response.Sso.RequestId,
							Token:     response.Sso.Token,
						},
					},
				}, nil
			case *mfav1.AuthenticateResponse_Webauthn:
				return &proto.MFAAuthenticateResponse{
					Response: &proto.MFAAuthenticateResponse_Webauthn{
						Webauthn: response.Webauthn,
					},
				}, nil
			default:
				return nil, trace.Errorf("Unexpected MFA response type %T", mfaMsg.AuthenticationResponse)
			}
		}

		return newMfaPrompt(rw, isResponse, convert, withheld)
	}
}
