/*
Copyright 2026 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package mfa

import (
	"context"
	"log/slog"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client/proto"
	mfav2 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v2"
	"github.com/gravitational/teleport/api/types"
	webauthnpb "github.com/gravitational/teleport/api/types/webauthn"
)

// SessionBoundCeremonyConfig contains configuration for a session-bound ceremony.
type SessionBoundCeremonyConfig struct {
	// CreateSessionChallenge creates a session-bound challenge.
	CreateSessionChallenge CreateSessionChallengeFunc
	// ValidateSessionChallenge validates a session-bound challenge.
	ValidateSessionChallenge ValidateSessionChallengeFunc
	// PromptConstructor creates a prompt to prompt the user to solve an authentication challenge.
	PromptConstructor PromptConstructor
	// CallbackCeremonyConstructor is optional and, if provided, constructs a callback
	// ceremony for the current run to complete SSO or Browser MFA challenges offered
	// by the server as part of the session-bound challenge.
	CallbackCeremonyConstructor MFACeremonyConstructor
	// TargetCluster is the name of the cluster to target for the session-bound challenge.
	TargetCluster string
}

// NewSessionBoundCeremony creates a new session-bound ceremony with the provided configuration.
func NewSessionBoundCeremony(config SessionBoundCeremonyConfig) (*SessionBoundCeremony, error) {
	switch {
	case config.CreateSessionChallenge == nil:
		return nil, trace.BadParameter("config.CreateSessionChallenge must not be nil")

	case config.ValidateSessionChallenge == nil:
		return nil, trace.BadParameter("config.ValidateSessionChallenge must not be nil")

	case config.PromptConstructor == nil:
		return nil, trace.BadParameter("config.PromptConstructor must not be nil")

	case config.TargetCluster == "":
		return nil, trace.BadParameter("config.TargetCluster must not be empty")
	}

	return &SessionBoundCeremony{
		createSessionChallenge:      config.CreateSessionChallenge,
		validateSessionChallenge:    config.ValidateSessionChallenge,
		promptConstructor:           config.PromptConstructor,
		callbackCeremonyConstructor: config.CallbackCeremonyConstructor,
		targetCluster:               config.TargetCluster,
	}, nil
}

// SessionBoundCeremony represents a ceremony that is bound to a specific session.
type SessionBoundCeremony struct {
	createSessionChallenge      CreateSessionChallengeFunc
	validateSessionChallenge    ValidateSessionChallengeFunc
	promptConstructor           PromptConstructor
	callbackCeremonyConstructor MFACeremonyConstructor

	targetCluster string
}

// Run runs the ceremony with a session-bound challenge using the provided binding parameters and returns the name of
// the challenge that was satisfied.
func (c *SessionBoundCeremony) Run(ctx context.Context, payload *mfav2.SessionIdentifyingPayload, promptOpts ...PromptOpt) (string, error) {
	createReq := mfav2.CreateSessionChallengeRequest_builder{
		Payload:       payload,
		TargetCluster: c.targetCluster,
	}.Build()

	// If a callback ceremony is provided, set the client callback URL in the create challenge request to request an SSO
	// or Browser challenge in addition to other challenges. The callback ceremony will be used to complete the SSO or
	// Browser challenge if it is offered by the server. If the callback ceremony fails to start, continue with the
	// session-bound challenge without SSO or Browser MFA.
	if c.callbackCeremonyConstructor != nil {
		callbackCeremony, err := c.callbackCeremonyConstructor(ctx)
		if err != nil {
			slog.DebugContext(
				ctx,
				"Failed starting callback ceremony for SSO/Browser MFA, continuing with other MFA methods",
				"error", err,
			)
		} else {
			defer callbackCeremony.Close()

			createReq.SetSsoClientRedirectUrl(callbackCeremony.GetClientCallbackURL())
			createReq.SetBrowserMfaTshRedirectUrl(callbackCeremony.GetClientCallbackURL())
			createReq.SetProxyAddressForSso(callbackCeremony.GetProxyAddress())

			promptOpts = append(promptOpts, withSSOMFACeremony(callbackCeremony))
		}
	}

	createResp, err := c.createSessionChallenge(ctx, createReq)
	if err != nil {
		return "", trace.Wrap(err)
	}

	protoAuthChal, err := convertMFAAuthenticateChallengeToClient(createResp.GetMfaChallenge())
	if err != nil {
		return "", trace.Wrap(err)
	}

	protoAuthResp, err := c.promptConstructor(promptOpts...).Run(ctx, protoAuthChal)
	if err != nil {
		return "", trace.Wrap(err)
	}

	mfaAuthResp, err := convertClientAuthenticateResponseToMFA(protoAuthResp, createResp.GetMfaChallenge().GetName())
	if err != nil {
		return "", trace.Wrap(err)
	}

	if _, err := c.validateSessionChallenge(
		ctx,
		mfav2.ValidateSessionChallengeRequest_builder{
			MfaResponse: mfaAuthResp,
		}.Build(),
	); err != nil {
		return "", trace.Wrap(err)
	}

	return createResp.GetMfaChallenge().GetName(), nil
}

func convertMFAAuthenticateChallengeToClient(mfaAuthChal *mfav2.AuthenticateChallenge) (*proto.MFAAuthenticateChallenge, error) {
	if mfaAuthChal == nil {
		return nil, trace.BadParameter("AuthenticateChallenge must not be nil")
	}

	protoAuthChal := &proto.MFAAuthenticateChallenge{}

	if chal := mfaAuthChal.GetWebauthnChallenge(); chal != nil {
		protoAuthChal.WebauthnChallenge = webauthnpb.CredentialAssertionV2ToV1(chal)
	}

	if chal := mfaAuthChal.GetSsoChallenge(); chal != nil {
		protoAuthChal.SSOChallenge = &proto.SSOChallenge{
			RequestId:   chal.GetRequestId(),
			RedirectUrl: chal.GetRedirectUrl(),
			Device:      ssoMFADeviceV2ToV1(chal.GetDevice()),
		}
	}

	if chal := mfaAuthChal.GetBrowserChallenge(); chal != nil {
		protoAuthChal.BrowserMFAChallenge = &proto.BrowserMFAChallenge{
			RequestId: chal.GetRequestId(),
		}
	}

	return protoAuthChal, nil
}

func convertClientAuthenticateResponseToMFA(protoResp *proto.MFAAuthenticateResponse, name string) (*mfav2.AuthenticateResponse, error) {
	switch {
	case protoResp == nil:
		return nil, trace.BadParameter("MFAAuthenticateResponse must not be nil")

	case name == "":
		return nil, trace.BadParameter("challenge name must not be empty")
	}

	mfaAuthResp := mfav2.AuthenticateResponse_builder{
		Name: name,
	}.Build()

	switch resp := protoResp.GetResponse().(type) {
	case *proto.MFAAuthenticateResponse_Webauthn:
		mfaAuthResp.SetWebauthn(webauthnpb.CredentialAssertionResponseV1ToV2(protoResp.GetWebauthn()))

	case *proto.MFAAuthenticateResponse_SSO:
		mfaAuthResp.SetSso(
			mfav2.SSOChallengeResponse_builder{
				RequestId: resp.SSO.GetRequestId(),
				Token:     resp.SSO.GetToken(),
			}.Build(),
		)

	case *proto.MFAAuthenticateResponse_Browser:
		mfaAuthResp.SetBrowser(
			mfav2.BrowserMFAResponse_builder{
				RequestId:        resp.Browser.GetRequestId(),
				WebauthnResponse: webauthnpb.CredentialAssertionResponseV1ToV2(resp.Browser.GetWebauthnResponse()),
			}.Build(),
		)

	default:
		return nil, trace.BadParameter(
			"unsupported MFA response from client (type %T); update your client to the latest supported version for this cluster and try again",
			protoResp.GetResponse(),
		)
	}

	return mfaAuthResp, nil
}

func ssoMFADeviceV2ToV1(v2 *mfav2.SSOMFADevice) *types.SSOMFADevice {
	if v2 == nil {
		return nil
	}

	return &types.SSOMFADevice{
		ConnectorId:   v2.GetConnectorId(),
		ConnectorType: v2.GetConnectorType(),
		DisplayName:   v2.GetDisplayName(),
	}
}
