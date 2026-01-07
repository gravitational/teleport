package tdpb

import (
	"errors"
	"log/slog"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client/proto"
	mfav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v1"
	"github.com/gravitational/teleport/api/mfa"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/srv/desktop/tdp"
)

// NewTDPBMFAPrompt constructs an mfa.PromptFunc that handles the MFA ceremony
// with a TDPB client.
func NewTDPBMFAPrompt(rw tdp.MessageReadWriter, withheld *[]tdp.Message, log *slog.Logger) func(string) mfa.PromptFunc {
	return func(channelID string) mfa.PromptFunc {
		convert := func(challenge *proto.MFAAuthenticateChallenge) (tdp.Message, error) {
			if challenge == nil {
				return nil, errors.New("empty MFA challenge")
			}

			mfaMsg := &MFA{
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

			return mfaMsg, nil
		}

		isResponse := func(msg tdp.Message) (*proto.MFAAuthenticateResponse, error) {
			mfaMsg, ok := msg.(*MFA)
			if !ok {
				return nil, tdp.ErrUnexpectedTDPMessageType
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

		return tdp.NewMfaPrompt(rw, isResponse, convert, withheld, log)
	}
}
