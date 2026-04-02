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
	"time"

	"github.com/gravitational/trace"
	"google.golang.org/grpc"

	"github.com/gravitational/teleport/api/client/proto"
	mfav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v1"
	"github.com/gravitational/teleport/api/utils/retryutils"
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
	// TargetCluster is the target cluster of session-bound MFA challenges.
	// This should be set when the target is a leaf cluster.
	TargetCluster string
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

// PerformSessionMFACeremony performs a session-bound MFA ceremony with the user.
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
			TargetCluster: c.TargetCluster,
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

	var mfaChalResp *proto.MFAAuthenticateResponse

	retry, err := retryutils.NewConstant(10 * time.Millisecond)
	if err != nil {
		return "", trace.Wrap(err)
	}

	retryCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	// Prompt the user to solve the session-bound MFA challenge.
	//
	// XXX: Retry on certain WebAuthn errors to allow users to recover from transient errors with their security
	// keys. This is a temporary workaround until we have a more robust solution for handling WebAuthn errors.
	err = retry.For(
		retryCtx,
		func() error {
			mfaChalResp, err = c.PromptConstructor().Run(ctx, protoChal)
			if err != nil {
				if strings.Contains(err.Error(), "failed to open security keys") ||
					strings.Contains(err.Error(), "failed to get assertion: rx error") {
					return trace.Wrap(err)
				}

				return retryutils.PermanentRetryError(trace.Wrap(err))
			}

			return nil
		})
	if err != nil {
		return "", trace.Wrap(err)
	}

	mfaResp, err := convertToAuthenticateResponse(createResp.GetMfaChallenge().Name, mfaChalResp)
	if err != nil {
		return "", trace.Wrap(err)
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

func convertToAuthenticateResponse(name string, mfaChalResp *proto.MFAAuthenticateResponse) (*mfav1.AuthenticateResponse, error) {
	if mfaChalResp == nil {
		return nil, trace.BadParameter("missing session-bound MFA response")
	}

	mfaResp := &mfav1.AuthenticateResponse{
		Name: name,
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
		return nil, trace.BadParameter("expected session-bound MFA response, got %T", mfaChalResp.GetResponse())
	}

	return mfaResp, nil
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
