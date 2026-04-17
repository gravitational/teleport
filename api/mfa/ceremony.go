/*
Copyright 2024 Gravitational, Inc.

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
	"slices"

	"github.com/gravitational/trace"
	"google.golang.org/grpc"

	"github.com/gravitational/teleport/api/client/proto"
	mfav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v1"
)

// Ceremony is an MFA ceremony.
type Ceremony struct {
	// CreateAuthenticateChallenge creates an authentication challenge.
	CreateAuthenticateChallenge CreateAuthenticateChallengeFunc
	// PromptConstructor creates a prompt to prompt the user to solve an authentication challenge.
	PromptConstructor PromptConstructor
	// MFACeremonyConstructor is an optional MFA ceremony constructor. If provided,
	// the MFA ceremony will also attempt to retrieve an MFA challenge.
	MFACeremonyConstructor MFACeremonyConstructor
}

// CallbackCeremony is an SSO/Browser callback ceremony.
type CallbackCeremony interface {
	GetClientCallbackURL() string
	GetProxyAddress() string
	Run(ctx context.Context, chal *proto.MFAAuthenticateChallenge) (*proto.MFAAuthenticateResponse, error)
	Close()
}

// MFACeremonyConstructor constructs a new SSO or Browser MFA ceremony.
type MFACeremonyConstructor func(ctx context.Context) (CallbackCeremony, error)

// CreateAuthenticateChallengeFunc is a function that creates an authentication challenge.
type CreateAuthenticateChallengeFunc func(ctx context.Context, req *proto.CreateAuthenticateChallengeRequest) (*proto.MFAAuthenticateChallenge, error)

// CreateSessionChallengeFunc is a function that creates a session-bound MFA challenge.
type CreateSessionChallengeFunc func(ctx context.Context, req *mfav1.CreateSessionChallengeRequest, opts ...grpc.CallOption) (*mfav1.CreateSessionChallengeResponse, error)

// ValidateSessionChallengeFunc is a function that validates a session-bound MFA challenge.
type ValidateSessionChallengeFunc func(ctx context.Context, req *mfav1.ValidateSessionChallengeRequest, opts ...grpc.CallOption) (*mfav1.ValidateSessionChallengeResponse, error)

// Run the MFA ceremony.
//
// req may be nil if ceremony.CreateAuthenticateChallenge does not require it, e.g. in
// the moderated session mfa ceremony which uses a custom stream rpc to create challenges.
func (c *Ceremony) Run(ctx context.Context, req *proto.CreateAuthenticateChallengeRequest, promptOpts ...PromptOpt) (*proto.MFAAuthenticateResponse, error) {
	if c.CreateAuthenticateChallenge == nil {
		return nil, trace.BadParameter("mfa ceremony must have CreateAuthenticateChallenge set in order to begin")
	}

	// If available, prepare an MFA ceremony and set the client redirect URL in the challenge
	// request to request an SSO or Browser MFA challenge in addition to other challenges.
	if c.MFACeremonyConstructor != nil {
		mfaCeremony, err := c.MFACeremonyConstructor(ctx)
		if err != nil {
			// We may fail to start the MFA flow in cases where the Proxy is down or broken. Fall
			// back to skipping SSO/Browser MFA, especially since SSO/Browser MFA may not even be allowed on the server.
			slog.DebugContext(ctx, "Failed to attempt SSO/Browser MFA, continuing with other MFA methods", "error", err)
		} else {
			defer mfaCeremony.Close()

			// req may be nil in cases where the ceremony's CreateAuthenticateChallenge sources
			// its own req or uses a different e.g. login. We should still provide the sso client
			// redirect URL in case the custom CreateAuthenticateChallenge handles it.
			if req == nil {
				req = new(proto.CreateAuthenticateChallengeRequest)
			}

			req.SSOClientRedirectURL = mfaCeremony.GetClientCallbackURL()
			// Reuse the same callback server for Browser MFA because only one of
			// SSO MFA or Browser MFA can be used. Sending both redirect URLs
			// indicates to the server that both methods are available.
			req.BrowserMFATSHRedirectURL = mfaCeremony.GetClientCallbackURL()
			req.ProxyAddress = mfaCeremony.GetProxyAddress()
			promptOpts = append(promptOpts, withSSOMFACeremony(mfaCeremony))
		}
	}

	chal, err := c.CreateAuthenticateChallenge(ctx, req)
	if err != nil {
		// CreateAuthenticateChallenge returns a bad parameter error when the client
		// user is not a Teleport user - for example, the AdminRole. Treat this as an MFA
		// not supported error so the client knows when it can be ignored.
		if trace.IsBadParameter(err) {
			return nil, &ErrMFANotSupported
		}
		return nil, trace.Wrap(err)
	}

	// If an MFA required check was provided, and the client discovers MFA is not required,
	// skip the MFA prompt and return an empty response.
	if chal.MFARequired == proto.MFARequired_MFA_REQUIRED_NO {
		return nil, &ErrMFANotRequired
	}

	if c.PromptConstructor == nil {
		return nil, trace.Wrap(&ErrMFANotSupported, "mfa ceremony must have PromptConstructor set in order to succeed")
	}

	// Set challenge extensions in the prompt, if present, but set it first so the
	// caller can still override it.
	if req != nil && req.ChallengeExtensions != nil {
		promptOpts = slices.Insert(promptOpts, 0, WithPromptChallengeExtensions(req.ChallengeExtensions))
	}

	resp, err := c.PromptConstructor(promptOpts...).Run(ctx, chal)
	return resp, trace.Wrap(err)
}

// CeremonyFn is a function that will carry out an MFA ceremony.
type CeremonyFn func(ctx context.Context, in *proto.CreateAuthenticateChallengeRequest, promptOpts ...PromptOpt) (*proto.MFAAuthenticateResponse, error)

// PerformAdminActionMFACeremony retrieves an MFA challenge from the server for an admin
// action, prompts the user to answer the challenge, and returns the resulting MFA response.
func PerformAdminActionMFACeremony(ctx context.Context, mfaCeremony CeremonyFn, allowReuse bool) (*proto.MFAAuthenticateResponse, error) {
	allowReuseExt := mfav1.ChallengeAllowReuse_CHALLENGE_ALLOW_REUSE_NO
	if allowReuse {
		allowReuseExt = mfav1.ChallengeAllowReuse_CHALLENGE_ALLOW_REUSE_YES
	}

	challengeRequest := &proto.CreateAuthenticateChallengeRequest{
		MFARequiredCheck: &proto.IsMFARequiredRequest{
			Target: &proto.IsMFARequiredRequest_AdminAction{
				AdminAction: &proto.AdminAction{},
			},
		},
		ChallengeExtensions: &mfav1.ChallengeExtensions{
			Scope:      mfav1.ChallengeScope_CHALLENGE_SCOPE_ADMIN_ACTION,
			AllowReuse: allowReuseExt,
		},
	}

	// Remove MFA resp from context if set. This way, the mfa required
	// check will return true as long as MFA for admin actions is enabled,
	// even if the current context has a reusable MFA. v18 server will
	// return this requirement as expected.
	// TODO(Joerger): DELETE IN v19.0.0
	ceremonyCtx := ContextWithMFAResponse(ctx, nil)

	resp, err := mfaCeremony(ceremonyCtx, challengeRequest, WithPromptReasonAdminAction())
	return resp, trace.Wrap(err)
}

// SessionBoundCeremonyConfig contains configuration for a session-bound ceremony.
type SessionBoundCeremonyConfig struct {
	// CreateSessionChallenge creates a session-bound challenge.
	CreateSessionChallenge CreateSessionChallengeFunc
	// ValidateSessionChallenge validates a session-bound challenge.
	ValidateSessionChallenge ValidateSessionChallengeFunc
	// PromptConstructor creates a prompt to prompt the user to solve an authentication challenge.
	PromptConstructor PromptConstructor
	// CallbackCeremony is optional and if provided, will be used to complete an SSO or Browser MFA challenge that may
	// be offered by the server as part of the session-bound challenges.
	CallbackCeremony CallbackCeremony
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
		createSessionChallenge:   config.CreateSessionChallenge,
		validateSessionChallenge: config.ValidateSessionChallenge,
		promptConstructor:        config.PromptConstructor,
		callbackCeremony:         config.CallbackCeremony,
		targetCluster:            config.TargetCluster,
	}, nil
}

// SessionBoundCeremony represents a ceremony that is bound to a specific session.
type SessionBoundCeremony struct {
	createSessionChallenge   CreateSessionChallengeFunc
	validateSessionChallenge ValidateSessionChallengeFunc
	promptConstructor        PromptConstructor
	callbackCeremony         CallbackCeremony

	targetCluster string
}

// RunWithSessionBinding runs the ceremony with a session-bound challenge using the provided binding parameters and
// returns the name of the challenge that was satisfied.
func (c *SessionBoundCeremony) Run(ctx context.Context, payload *mfav1.SessionIdentifyingPayload, promptOpts ...PromptOpt) (string, error) {
	createReq := &mfav1.CreateSessionChallengeRequest{
		Payload:       payload,
		TargetCluster: c.targetCluster,
	}

	// If a callback ceremony is provided, set the client callback URL in the create challenge request to request an SSO
	// or Browser challenge in addition to other challenges. The callback ceremony will be used to complete the SSO or
	// Browser challenge if it is offered by the server.
	if c.callbackCeremony != nil {
		createReq.SsoClientRedirectUrl = c.callbackCeremony.GetClientCallbackURL()
		createReq.BrowserMfaTshRedirectUrl = c.callbackCeremony.GetClientCallbackURL()
		createReq.ProxyAddressForSso = c.callbackCeremony.GetProxyAddress()

		promptOpts = append(promptOpts, withSSOMFACeremony(c.callbackCeremony))
	}

	createResp, err := c.createSessionChallenge(ctx, createReq)
	if err != nil {
		return "", trace.Wrap(err)
	}

	protoAuthChal, err := convertToProtoAuthChal(createResp.MfaChallenge)
	if err != nil {
		return "", trace.Wrap(err)
	}

	protoAuthResp, err := c.promptConstructor(promptOpts...).Run(ctx, protoAuthChal)
	if err != nil {
		return "", trace.Wrap(err)
	}

	mfaAuthResp, err := convertToMFAAuthResp(protoAuthResp, createResp.MfaChallenge.GetName())
	if err != nil {
		return "", trace.Wrap(err)
	}

	if _, err := c.validateSessionChallenge(
		ctx,
		&mfav1.ValidateSessionChallengeRequest{
			MfaResponse: mfaAuthResp,
		},
	); err != nil {
		return "", trace.Wrap(err)
	}

	// If a callback ceremony was used, close it to clean up any resources associated with it.
	if c.callbackCeremony != nil {
		c.callbackCeremony.Close()
	}

	return createResp.MfaChallenge.GetName(), nil
}

func convertToProtoAuthChal(mfaAuthChal *mfav1.AuthenticateChallenge) (*proto.MFAAuthenticateChallenge, error) {
	if mfaAuthChal == nil {
		return nil, trace.BadParameter("AuthenticateChallenge must not be nil")
	}

	protoAuthChal := &proto.MFAAuthenticateChallenge{}

	if chal := mfaAuthChal.GetWebauthnChallenge(); chal != nil {
		protoAuthChal.WebauthnChallenge = chal
	}

	if chal := mfaAuthChal.GetSsoChallenge(); chal != nil {
		protoAuthChal.SSOChallenge = &proto.SSOChallenge{
			RequestId:   chal.GetRequestId(),
			RedirectUrl: chal.GetRedirectUrl(),
			Device:      chal.GetDevice(),
		}
	}

	if chal := mfaAuthChal.GetBrowserChallenge(); chal != nil {
		protoAuthChal.BrowserMFAChallenge = &proto.BrowserMFAChallenge{
			RequestId: chal.GetRequestId(),
		}
	}

	return protoAuthChal, nil
}

func convertToMFAAuthResp(protoResp *proto.MFAAuthenticateResponse, name string) (*mfav1.AuthenticateResponse, error) {
	switch {
	case protoResp == nil:
		return nil, trace.BadParameter("MFAAuthenticateResponse must not be nil")

	case name == "":
		return nil, trace.BadParameter("challenge name must not be empty")
	}

	mfaAuthResp := &mfav1.AuthenticateResponse{
		Name: name,
	}

	switch resp := protoResp.GetResponse().(type) {
	case *proto.MFAAuthenticateResponse_Webauthn:
		mfaAuthResp.Response = &mfav1.AuthenticateResponse_Webauthn{
			Webauthn: protoResp.GetWebauthn(),
		}

	case *proto.MFAAuthenticateResponse_SSO:
		mfaAuthResp.Response = &mfav1.AuthenticateResponse_Sso{
			Sso: &mfav1.SSOChallengeResponse{
				RequestId: resp.SSO.GetRequestId(),
				Token:     resp.SSO.GetToken(),
			},
		}

	case *proto.MFAAuthenticateResponse_Browser:
		mfaAuthResp.Response = &mfav1.AuthenticateResponse_Browser{
			Browser: &mfav1.BrowserMFAResponse{
				RequestId:        resp.Browser.GetRequestId(),
				WebauthnResponse: resp.Browser.GetWebauthnResponse(),
			},
		}

	default:
		return nil, trace.BadParameter("unknown or nil response with type %T (this is a bug)", protoResp.GetResponse())
	}

	return mfaAuthResp, nil
}
