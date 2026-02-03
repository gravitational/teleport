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
	"strings"

	"github.com/gravitational/trace"
	"google.golang.org/grpc"

	"github.com/gravitational/teleport/api/client/proto"
	mfav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v1"
)

// Ceremony is an MFA ceremony.
type Ceremony struct {
	// CreateAuthenticateChallenge creates an authentication challenge.
	CreateAuthenticateChallenge CreateAuthenticateChallengeFunc
	// CreateSessionChallenge creates a session-bound MFA challenge.
	CreateSessionChallenge CreateSessionChallengeFunc
	// ValidateSessionChallenge validates a session-bound MFA challenge.
	ValidateSessionChallenge ValidateSessionChallengeFunc
	// PromptConstructor creates a prompt to prompt the user to solve an authentication challenge.
	PromptConstructor PromptConstructor
	// SSOMFACeremonyConstructor is an optional SSO MFA ceremony constructor. If provided,
	// the MFA ceremony will also attempt to retrieve an SSO MFA challenge.
	SSOMFACeremonyConstructor SSOMFACeremonyConstructor
}

// SSOMFACeremony is an SSO MFA ceremony.
type SSOMFACeremony interface {
	GetClientCallbackURL() string
	GetProxyAddress() string
	Run(ctx context.Context, chal *proto.MFAAuthenticateChallenge) (*proto.MFAAuthenticateResponse, error)
	Close()
}

// SSOMFACeremonyConstructor constructs a new SSO MFA ceremony.
type SSOMFACeremonyConstructor func(ctx context.Context) (SSOMFACeremony, error)

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

	// If available, prepare an SSO MFA ceremony and set the client redirect URL in the challenge
	// request to request an SSO challenge in addition to other challenges.
	if c.SSOMFACeremonyConstructor != nil {
		ssoMFACeremony, err := c.SSOMFACeremonyConstructor(ctx)
		if err != nil {
			// We may fail to start the SSO MFA flow in cases where the Proxy is down or broken. Fall
			// back to skipping SSO MFA, especially since SSO MFA may not even be allowed on the server.
			slog.DebugContext(ctx, "Failed to attempt SSO MFA, continuing with other MFA methods", "error", err)
		} else {
			defer ssoMFACeremony.Close()

			// req may be nil in cases where the ceremony's CreateAuthenticateChallenge sources
			// its own req or uses a different e.g. login. We should still provide the sso client
			// redirect URL in case the custom CreateAuthenticateChallenge handles it.
			if req == nil {
				req = new(proto.CreateAuthenticateChallengeRequest)
			}

			req.SSOClientRedirectURL = ssoMFACeremony.GetClientCallbackURL()
			req.ProxyAddress = ssoMFACeremony.GetProxyAddress()
			promptOpts = append(promptOpts, withSSOMFACeremony(ssoMFACeremony))
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

// PerformSessionMFACeremony performs a session-bound MFA ceremony with the user.
// TODO(cthach): Add trusted cluster support.
// TODO(cthach): Add SSO MFA support.
func (c *Ceremony) PerformSessionMFACeremony(ctx context.Context, sessionID []byte) (string, error) {
	if c.PromptConstructor == nil {
		return "", trace.Wrap(&ErrMFANotSupported, "ceremony must have PromptConstructor set in order to proceed")
	}

	createResp, err := c.CreateSessionChallenge(
		ctx,
		&mfav1.CreateSessionChallengeRequest{
			Payload: &mfav1.SessionIdentifyingPayload{
				Payload: &mfav1.SessionIdentifyingPayload_SshSessionId{
					SshSessionId: sessionID,
				},
			},
			// TargetCluster: "TODO",
			// SsoClientRedirectUrl: "TODO",
			// ProxyAddressForSso:   "TODO",
		},
	)
	if err != nil {
		return "", trace.Wrap(err)
	}

	protoChal := &proto.MFAAuthenticateChallenge{
		WebauthnChallenge: createResp.GetMfaChallenge().GetWebauthnChallenge(),
		SSOChallenge:      nil, // TODO(cthach): Add SSO challenge support.
	}

	// Prompt the user to solve the session-bound MFA challenge.
	var (
		mfaChalResp *proto.MFAAuthenticateResponse
		attempts    int
	)
	const maxAttempts = 5

	for {
		mfaChalResp, err = c.PromptConstructor().Run(ctx, protoChal)
		if err != nil {
			// XXX: Retry on certain WebAuthn errors to allow users to recover from transient errors with their security
			// keys.This is a temporary workaround until we have a more robust solution for handling WebAuthn errors.
			if strings.Contains(err.Error(), "failed to open security keys") || strings.Contains(err.Error(), "failed to get assertion: rx error") && attempts < maxAttempts-1 {
				attempts++
				continue
			}

			return "", trace.Wrap(err)
		}

		break
	}

	// Convert from the legacy proto.MFAAuthenticateResponse to the mfav1.AuthenticateResponse.
	// TODO(cthach): Move conversion logic into a helper.
	mfaResp := &mfav1.AuthenticateResponse{
		Name: createResp.GetMfaChallenge().Name,
	}

	switch mfaChalResp.GetResponse().(type) {
	case *proto.MFAAuthenticateResponse_Webauthn:
		mfaResp.Response = &mfav1.AuthenticateResponse_Webauthn{
			Webauthn: mfaChalResp.GetWebauthn(),
		}

	case *proto.MFAAuthenticateResponse_SSO:
		mfaResp.Response = &mfav1.AuthenticateResponse_Sso{
			Sso: (*mfav1.SSOChallengeResponse)(mfaChalResp.GetSSO()),
		}

	default:
		return "", trace.BadParameter("expected session-bound MFA response, got %T", mfaChalResp.GetResponse())
	}

	if _, err := c.ValidateSessionChallenge(
		ctx,
		&mfav1.ValidateSessionChallengeRequest{
			MfaResponse: mfaResp,
		},
	); err != nil {
		return "", trace.Wrap(err)
	}

	return createResp.GetMfaChallenge().Name, nil
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
