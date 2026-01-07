package legacy

import (
	"log/slog"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/mfa"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/srv/desktop/tdp"
	"github.com/gravitational/trace"

	wantypes "github.com/gravitational/teleport/lib/auth/webauthntypes"
)

// NewTDPMFAPrompt constructs an mfa.PromptFunc that handles the MFA ceremony
// with a TDP client.
func NewTDPMFAPrompt(rw tdp.MessageReadWriter, withheld *[]tdp.Message, log *slog.Logger) func(string) mfa.PromptFunc {
	return func(channelID string) mfa.PromptFunc {
		convert := func(chal *proto.MFAAuthenticateChallenge) (tdp.Message, error) {
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

		isResponse := func(msg tdp.Message) (*proto.MFAAuthenticateResponse, error) {
			switch t := msg.(type) {
			case *MFA:
				return t.MFAAuthenticateResponse, nil
			default:
				return nil, tdp.ErrUnexpectedTDPMessageType
			}
		}

		return tdp.NewMfaPrompt(rw, isResponse, convert, withheld, log)
	}
}
