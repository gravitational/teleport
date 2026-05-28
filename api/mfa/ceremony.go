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
	"errors"
	"log/slog"
	"slices"

	"github.com/gravitational/trace"
	"google.golang.org/grpc"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/client/webclient"
	mfav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v1"
	mfav2 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v2"
)

// Ceremony is an MFA ceremony.
type Ceremony struct {
	// CreateAuthenticateChallenge creates an authentication challenge.
	CreateAuthenticateChallenge CreateAuthenticateChallengeFunc
	// CreateRegisterChallenge creates a device registration challenge. If set to
	// nil, the ceremony is unable to register MFA devices.
	CreateRegisterChallenge CreateRegisterChallengeFunc
	// PromptConstructor creates a prompt to prompt the user to solve an authentication challenge.
	PromptConstructor PromptConstructor
	// MFACeremonyConstructor is an optional MFA ceremony constructor. If provided,
	// the MFA ceremony will also attempt to retrieve an MFA challenge.
	MFACeremonyConstructor MFACeremonyConstructor
	// AddMFADevice adds a device to Teleport backend after it has been
	// registered on the client side. If set to nil, the ceremony is unable to
	// register MFA devices.
	AddMFADevice AddMFADeviceFunc
	// Ping fetches a [webclient.PingResponse] from the server.
	Ping PingFunc
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
type CreateSessionChallengeFunc func(ctx context.Context, req *mfav2.CreateSessionChallengeRequest, opts ...grpc.CallOption) (*mfav2.CreateSessionChallengeResponse, error)

// ValidateSessionChallengeFunc is a function that validates a session-bound MFA challenge.
type ValidateSessionChallengeFunc func(ctx context.Context, req *mfav2.ValidateSessionChallengeRequest, opts ...grpc.CallOption) (*mfav2.ValidateSessionChallengeResponse, error)

// CreateRegisterChallengeFunc is a function that creates an MFA device
// registration challenge.
type CreateRegisterChallengeFunc func(
	ctx context.Context, req *proto.CreateRegisterChallengeRequest,
) (*proto.MFARegisterChallenge, error)

// AddMFADeviceFunc is a function that adds an MFA device to Teleport backend.
type AddMFADeviceFunc func(
	ctx context.Context, req *proto.MFARegisterResponse, config RegistrationCeremonyConfig,
) error

// PingFunc is a function that fetches a PingResponse from the server. Its
// purpose is to discover available MFA methods.
type PingFunc func(ctx context.Context) (*webclient.PingResponse, error)

// Run runs the MFA ceremony. If the user has no eligible MFA devices and the
// ceremony is requested for per-session MFA, this function will attempt to
// register a new device and then retry the ceremony.
//
// req may be nil if ceremony.CreateAuthenticateChallenge does not require it, e.g. in
// the moderated session mfa ceremony which uses a custom stream rpc to create challenges.
func (c *Ceremony) Run(
	ctx context.Context, req *proto.CreateAuthenticateChallengeRequest, promptOpts ...PromptOpt,
) (*proto.MFAAuthenticateResponse, error) {
	res, authErr := c.Authenticate(ctx, req, promptOpts...)
	// The success case is actually trivial here, so deal with it first.
	if authErr == nil {
		return res, nil
	}

	// The user has no device registered that would allow per-session MFA, so
	// prompt them to register and then retry the ceremony.
	reason, ok := registrationReasonForError(req, authErr)
	if !ok {
		return nil, trace.Wrap(authErr)
	}
	added, err := c.Register(ctx, RegistrationCeremonyConfig{
		Reason: reason,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if !added {
		return nil, trace.Wrap(authErr)
	}

	res, authErr = c.Authenticate(ctx, req, promptOpts...)
	return res, trace.Wrap(authErr)
}

func registrationReasonForError(req *proto.CreateAuthenticateChallengeRequest, err error) (RegistrationReason, bool) {
	if !isPerSessionMFARequest(req) {
		return "", false
	}

	switch {
	case errors.Is(err, &ErrNoMFADevices):
		return RegistrationReasonSessionMFANoDevices, true
	case errors.Is(err, &ErrNoEligibleMFADevices):
		return RegistrationReasonSessionMFANoEligibleDevices, true
	default:
		return "", false
	}
}

func isPerSessionMFARequest(req *proto.CreateAuthenticateChallengeRequest) bool {
	return req != nil && req.ChallengeExtensions.GetScope() == mfav1.ChallengeScope_CHALLENGE_SCOPE_USER_SESSION
}

// Authenticate performs a single MFA ceremony.
//
// req may be nil if ceremony.CreateAuthenticateChallenge does not require it, e.g. in
// the moderated session mfa ceremony which uses a custom stream rpc to create challenges.
func (c *Ceremony) Authenticate(
	ctx context.Context, req *proto.CreateAuthenticateChallengeRequest, promptOpts ...PromptOpt,
) (*proto.MFAAuthenticateResponse, error) {
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

	switch chal.MFARequired {
	case proto.MFARequired_MFA_REQUIRED_NO:
		// If an MFA required check was provided, and the client discovers MFA is not
		// required, skip the MFA prompt and return an error.
		return nil, &ErrMFANotRequired
	case proto.MFARequired_MFA_REQUIRED_YES:
		// If the user has no eligible device while attempting a per-session MFA,
		// return an error. The caller might retry after registering a new device.
		if chal.WebauthnChallenge == nil &&
			chal.SSOChallenge == nil &&
			chal.BrowserMFAChallenge == nil &&
			isPerSessionMFARequest(req) {
			if chal.TOTP == nil {
				// No TOTP device = no devices at all
				return nil, trace.Wrap(&ErrNoMFADevices)
			}
			// There are TOTP devices, but these are not eligible
			return nil, trace.Wrap(&ErrNoEligibleMFADevices)
		}
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

type RegistrationReason string

const (
	// RegistrationReasonExplicit indicates that the user explicitly asked to
	// register an MFA device.
	RegistrationReasonExplicit RegistrationReason = "explicit"
	// RegistrationReasonSessionMFANoDevices indicates that the registration is
	// attempted for per-session MFA, but there are no MFA devices registered.
	RegistrationReasonSessionMFANoDevices RegistrationReason = "session_mfa_no_devices"
	// RegistrationReasonSessionMFANoEligibleDevices indicates that the
	// registration is attempted for per-session MFA, but none of the registered
	// MFA devices are eligible.
	RegistrationReasonSessionMFANoEligibleDevices RegistrationReason = "session_mfa_no_eligible_devices"
)

// RegistrationCeremonyConfig provides configuration for the
// [Ceremony.Register] function.
type RegistrationCeremonyConfig struct {
	// Reason indicates the reason why we register an MFA device.
	Reason RegistrationReason
	// DeviceName is the name of the device to be added. If empty, the user will
	// be prompted to enter it.
	DeviceName string
	// DeviceType is the type of the device to be added. If empty, the user will
	// be prompted to enter it.
	DeviceType MFADeviceType
	// DeviceUsage is the intended usage for the MFA device to be added. If set
	// to [proto.DeviceUsage_DEVICE_USAGE_UNSPECIFIED], the user may be prompted
	// whether to register the device as passwordless.
	DeviceUsage proto.DeviceUsage
}

// Register interacts with user to register an MFA device on the client side
// and adds the device to Teleport backend. Returns true if the device was
// added, and false if it was not (for example, the user refused to register
// it).
func (c *Ceremony) Register(ctx context.Context, config RegistrationCeremonyConfig) (bool, error) {
	if c.CreateRegisterChallenge == nil {
		return false, trace.BadParameter("mfa ceremony must have CreateRegisterChallenge set in order to begin")
	}

	if c.AddMFADevice == nil {
		return false, trace.BadParameter("mfa ceremony must have AddMFADevice set in order to begin")
	}

	regPrompt := c.PromptConstructor()

	promptConfig := RegistrationPromptConfig{
		RegistrationCeremonyConfig: config,
	}
	if config.DeviceType == "" && c.Ping != nil {
		// If we are prompting the user for the device type, then take a glimpse at
		// server-side settings and adjust the options accordingly.
		// This is undesirable to do during flag setup, but we can do it here.
		pingResp, err := c.Ping(ctx)
		if err != nil {
			return false, trace.Wrap(err)
		}
		promptConfig.AuthSecondFactor = pingResp.Auth.SecondFactor
	}

	// Query for missing data to register the device.
	updatedPromptConfig, err := regPrompt.AskRegister(ctx, promptConfig)
	if err != nil {
		return false, trace.Wrap(err)
	}
	if updatedPromptConfig == nil {
		// No device has been registered.
		return false, nil
	} else {
		promptConfig = *updatedPromptConfig
		config = updatedPromptConfig.RegistrationCeremonyConfig
	}

	mfaResp, err := c.Run(ctx, &proto.CreateAuthenticateChallengeRequest{
		ChallengeExtensions: &mfav1.ChallengeExtensions{
			Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_MANAGE_DEVICES,
		},
	})
	if err != nil {
		return false, trace.Wrap(err)
	}

	devTypePB := map[MFADeviceType]proto.DeviceType{
		MFADeviceTypeTOTP:     proto.DeviceType_DEVICE_TYPE_TOTP,
		MFADeviceTypeWebauthn: proto.DeviceType_DEVICE_TYPE_WEBAUTHN,
		MFADeviceTypeTouchID:  proto.DeviceType_DEVICE_TYPE_WEBAUTHN,
	}[config.DeviceType]
	// Sanity check.
	if devTypePB == proto.DeviceType_DEVICE_TYPE_UNSPECIFIED {
		return false, trace.BadParameter("unexpected device type: %q", config.DeviceType)
	}

	regChal, err := c.CreateRegisterChallenge(ctx, &proto.CreateRegisterChallengeRequest{
		ExistingMFAResponse: mfaResp,
		DeviceType:          devTypePB,
		DeviceUsage:         config.DeviceUsage,
	})
	if err != nil {
		return false, trace.Wrap(err)
	}

	result, err := regPrompt.RunRegister(ctx, promptConfig, regChal)
	if err != nil {
		return false, trace.Wrap(err)
	}

	// Add the registered device to the backend.
	if err = c.AddMFADevice(ctx, result.Response, promptConfig.RegistrationCeremonyConfig); err != nil {
		result.Callbacks.Rollback()
		return false, trace.Wrap(err)
	}
	if err := result.Callbacks.Confirm(); err != nil {
		return false, trace.Wrap(err)
	}

	// Failure to notify doesn't justify making it an error, so just log it and
	// move on.
	if err = regPrompt.NotifyRegistrationSuccess(ctx, promptConfig); err != nil {
		slog.ErrorContext(ctx, "Unable to notify about registration success", "error", err)
	}
	return true, nil
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
