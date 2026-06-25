// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package tdpb

import (
	"log/slog"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client/proto"
	mfav2 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v2"
	"github.com/gravitational/teleport/api/mfa"
	webauthnpb "github.com/gravitational/teleport/api/types/webauthn"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/srv/desktop/tdp"
)

// NewTDPBMFAPrompt constructs an mfa.PromptFunc that handles the MFA ceremony
// with a TDPB client.
func NewTDPBMFAPrompt(rw tdp.MessageReadWriter, withheld *[]tdp.Message, log *slog.Logger) func(string) mfa.PromptFunc {
	return func(channelID string) mfa.PromptFunc {
		//nolint:staticcheck // TODO(rhammonds-teleport): Delete when Desktop has migrated to mfav2.
		convert := func(challenge *proto.MFAAuthenticateChallenge) (tdp.Message, error) {
			if challenge == nil {
				return nil, trace.Errorf("empty MFA challenge")
			}

			mfaMsg := &MFA{
				ChannelId: channelID,
				Challenge: &mfav2.AuthenticateChallenge{},
			}

			if challenge.WebauthnChallenge != nil {
				mfaMsg.Challenge.SetWebauthnChallenge(webauthnpb.CredentialAssertionV1ToV2(challenge.GetWebauthnChallenge()))
			}

			if challenge.SSOChallenge != nil {
				mfaMsg.Challenge.SetSsoChallenge(mfav2.SSOChallenge_builder{
					RequestId:   challenge.SSOChallenge.RequestId,
					RedirectUrl: challenge.SSOChallenge.RedirectUrl,
					Device: mfav2.SSOMFADevice_builder{
						ConnectorId:   challenge.SSOChallenge.Device.ConnectorId,
						ConnectorType: challenge.SSOChallenge.Device.ConnectorType,
						DisplayName:   challenge.SSOChallenge.Device.DisplayName,
					}.Build(),
				}.Build())
			}

			if challenge.WebauthnChallenge == nil && challenge.SSOChallenge == nil && challenge.TOTP == nil {
				return nil, trace.Wrap(authclient.ErrNoMFADevices)
			}

			return mfaMsg, nil
		}

		asResponse := func(msg tdp.Message) (*proto.MFAAuthenticateResponse, error) {
			mfaMsg, ok := msg.(*MFA)
			if !ok {
				return nil, tdp.ErrUnexpectedTDPMessageType
			}

			if mfaMsg.AuthenticationResponse == nil {
				return nil, trace.Errorf("MFA response is empty")
			}

			switch mfaMsg.AuthenticationResponse.WhichResponse() {
			case mfav2.AuthenticateResponse_Sso_case:
				sso := mfaMsg.AuthenticationResponse.GetSso()
				return &proto.MFAAuthenticateResponse{
					Response: &proto.MFAAuthenticateResponse_SSO{
						SSO: &proto.SSOResponse{
							RequestId: sso.GetRequestId(),
							Token:     sso.GetToken(),
						},
					},
				}, nil
			case mfav2.AuthenticateResponse_Webauthn_case:
				webauthn := mfaMsg.AuthenticationResponse.GetWebauthn()
				return &proto.MFAAuthenticateResponse{
					Response: &proto.MFAAuthenticateResponse_Webauthn{
						Webauthn: webauthnpb.CredentialAssertionResponseV2ToV1(webauthn),
					},
				}, nil
			default:
				return nil, trace.Errorf("Unexpected MFA response type %T", mfaMsg.AuthenticationResponse)
			}
		}

		return tdp.NewMFAPrompt(rw, asResponse, convert, withheld, log)
	}
}
