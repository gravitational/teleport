/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

// Package mfatypes contains JSON-compatible MFA challenge types used by
// browser/web flows. It is a leaf package so that low-level wire-format
// packages (such as lib/srv/desktop/tdp/protocol/legacy) can depend on these
// types without pulling in lib/client.
package mfatypes

import (
	"encoding/json"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client/proto"
	wantypes "github.com/gravitational/teleport/lib/auth/webauthntypes"
)

// MFAAuthenticateChallenge is an MFA authentication challenge sent on user
// login / authentication ceremonies.
type MFAAuthenticateChallenge struct {
	// WebauthnChallenge contains a WebAuthn credential assertion used for
	// login/authentication ceremonies.
	WebauthnChallenge *wantypes.CredentialAssertion `json:"webauthn_challenge"`
	// TOTPChallenge specifies whether TOTP is supported for this user.
	TOTPChallenge bool `json:"totp_challenge"`
	// SSOChallenge is an SSO MFA challenge.
	SSOChallenge *SSOChallenge `json:"sso_challenge"`
	// BrowserMFAChallenge is a Browser MFA challenge.
	BrowserMFAChallenge *BrowserMFAChallenge `json:"browser_challenge"`
}

// SSOChallenge is a json compatible [proto.SSOChallenge].
type SSOChallenge struct {
	RequestID   string        `json:"requestId,omitempty"`
	RedirectURL string        `json:"redirectUrl,omitempty"`
	Device      *SSOMFADevice `json:"device"`
	// ChannelID is used by the front end to differentiate multiple ongoing SSO
	// MFA requests so they don't interfere with each other.
	ChannelID string `json:"channelId"`
}

// SSOMFADevice is a json compatible [proto.SSOMFADevice].
type SSOMFADevice struct {
	ConnectorID   string `json:"connectorId,omitempty"`
	ConnectorType string `json:"connectorType,omitempty"`
	DisplayName   string `json:"displayName,omitempty"`
}

func SSOChallengeFromProto(ssoChal *proto.SSOChallenge) *SSOChallenge {
	return &SSOChallenge{
		RequestID:   ssoChal.RequestId,
		RedirectURL: ssoChal.RedirectUrl,
		Device: &SSOMFADevice{
			ConnectorID:   ssoChal.Device.ConnectorId,
			ConnectorType: ssoChal.Device.ConnectorType,
			DisplayName:   ssoChal.Device.DisplayName,
		},
	}
}

// BrowserMFAChallenge is a json compatible [proto.BrowserMFAChallenge].
type BrowserMFAChallenge struct {
	RequestID string `json:"requestId,omitempty"`
}

// BrowserChallengeToProto converts an BrowserChallenge to proto format.
func BrowserChallengeToProto(browserChal *BrowserMFAChallenge) *proto.BrowserMFAChallenge {
	return &proto.BrowserMFAChallenge{
		RequestId: browserChal.RequestID,
	}
}

// BrowserChallengeFromProto converts a BrowserChallenge to json compatible format
func BrowserChallengeFromProto(browserChal *proto.BrowserMFAChallenge) *BrowserMFAChallenge {
	return &BrowserMFAChallenge{
		RequestID: browserChal.RequestId,
	}
}

// MFAChallengeResponse holds the response to a MFA challenge.
type MFAChallengeResponse struct {
	// TOTPCode is a code for a otp device.
	TOTPCode string `json:"totp_code,omitempty"`
	// WebauthnResponse is a response from a webauthn device.
	WebauthnResponse *wantypes.CredentialAssertionResponse `json:"webauthn_response,omitempty"`
	// SSOResponse is a response from an SSO MFA flow.
	SSOResponse *SSOResponse `json:"sso_response"`
	// TODO(Joerger): DELETE IN v20.0.0, WebauthnResponse used instead.
	WebauthnAssertionResponse *wantypes.CredentialAssertionResponse `json:"webauthnAssertionResponse"`
	// BrowserMFAResponse is a response the browser completing an MFA challenge
	// as part of the Browser MFA flow.
	BrowserMFAResponse *BrowserMFAResponse `json:"browser_response"`
}

// SSOResponse is a json compatible [proto.SSOResponse].
type SSOResponse struct {
	RequestID string `json:"requestId,omitempty"`
	Token     string `json:"token,omitempty"`
}

// BrowserMFAResponse is a json compatible [proto.BrowserMFAResponse].
type BrowserMFAResponse struct {
	RequestID        string                                `json:"requestId,omitempty"`
	WebauthnResponse *wantypes.CredentialAssertionResponse `json:"webauthnResponse,omitempty"`
}

// GetOptionalMFAResponseProtoReq converts response to a type proto.MFAAuthenticateResponse,
// if there were any responses set. Otherwise returns nil.
func (r *MFAChallengeResponse) GetOptionalMFAResponseProtoReq() (*proto.MFAAuthenticateResponse, error) {
	if r == nil {
		return nil, nil
	}

	var availableResponses int
	if r.TOTPCode != "" {
		availableResponses++
	}
	if r.WebauthnResponse != nil {
		availableResponses++
	}
	if r.SSOResponse != nil {
		availableResponses++
	}

	if availableResponses > 1 {
		return nil, trace.BadParameter("only one MFA response field can be set")
	}

	switch {
	case r.WebauthnResponse != nil:
		return &proto.MFAAuthenticateResponse{Response: &proto.MFAAuthenticateResponse_Webauthn{
			Webauthn: wantypes.CredentialAssertionResponseToProto(r.WebauthnResponse),
		}}, nil
	case r.SSOResponse != nil:
		return &proto.MFAAuthenticateResponse{Response: &proto.MFAAuthenticateResponse_SSO{
			SSO: &proto.SSOResponse{
				RequestId: r.SSOResponse.RequestID,
				Token:     r.SSOResponse.Token,
			},
		}}, nil
	case r.TOTPCode != "":
		return &proto.MFAAuthenticateResponse{Response: &proto.MFAAuthenticateResponse_TOTP{
			TOTP: &proto.TOTPResponse{Code: r.TOTPCode},
		}}, nil
	case r.WebauthnAssertionResponse != nil:
		return &proto.MFAAuthenticateResponse{Response: &proto.MFAAuthenticateResponse_Webauthn{
			Webauthn: wantypes.CredentialAssertionResponseToProto(r.WebauthnAssertionResponse),
		}}, nil
	}

	return nil, nil
}

// ParseMFAChallengeResponse parses [MFAChallengeResponse] from JSON and returns it as a [proto.MFAAuthenticateResponse].
func ParseMFAChallengeResponse(mfaResponseJSON []byte) (*proto.MFAAuthenticateResponse, error) {
	var resp MFAChallengeResponse
	if err := json.Unmarshal(mfaResponseJSON, &resp); err != nil {
		return nil, trace.Wrap(err)
	}

	protoResp, err := resp.GetOptionalMFAResponseProtoReq()
	return protoResp, trace.Wrap(err)
}
