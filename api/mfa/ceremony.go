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

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/client/webclient"
	"github.com/gravitational/teleport/api/constants"
	mfav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v1"
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

// CreateRegisterChallengeFunc is a function that creates an MFA device
// registration challenge.
type CreateRegisterChallengeFunc func(
	ctx context.Context, req *proto.CreateRegisterChallengeRequest,
) (*proto.MFARegisterChallenge, error)

// AddMFADeviceFunc is a function that adds an MFA device to Teleport backend.
type AddMFADeviceFunc func(ctx context.Context, req *proto.MFARegisterResponse, config RegisterConfig) error

// PingFunc is a function that
type PingFunc func(ctx context.Context) (*webclient.PingResponse, error)

// Run the MFA ceremony.
//
// req may be nil if ceremony.CreateAuthenticateChallenge does not require it, e.g. in
// the moderated session mfa ceremony which uses a custom stream rpc to create challenges.
//
// If the ceremony and was configured to support it, this method also offers
// the user to register their first MFA device.
func (c *Ceremony) Run(ctx context.Context, req *proto.CreateAuthenticateChallengeRequest, promptOpts ...PromptOpt) (*proto.MFAAuthenticateResponse, error) {
	resp, runErr := c.Authenticate(ctx, req, promptOpts...)
	regReason, ok := registrationReasonForError(req, runErr)
	if !ok {
		return resp, trace.Wrap(runErr)
	}

	cfg := RegistrationPromptConfig{
		Reason: regReason,
	}
	added, regErr := c.Register(ctx, cfg)
	if regErr != nil {
		return nil, trace.Wrap(regErr)
	}
	if !added {
		return nil, trace.Wrap(runErr)
	}

	return c.Authenticate(ctx, req, promptOpts...)
}

func (c *Ceremony) Authenticate(ctx context.Context, req *proto.CreateAuthenticateChallengeRequest, promptOpts ...PromptOpt) (*proto.MFAAuthenticateResponse, error) {
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

	hasOTP := chal.TOTP != nil
	hasWebauthn := chal.WebauthnChallenge != nil
	hasSSO := chal.SSOChallenge != nil
	hasBrowserMfa := chal.BrowserMFAChallenge != nil
	isPerSessionMFA := isPerSessionMFARequest(req)
	switch {
	case !hasOTP && !hasWebauthn && !hasSSO && !hasBrowserMfa:
		return nil, &ErrNoMFADevices
	case isPerSessionMFA && hasOTP && !hasWebauthn && !hasSSO && !hasBrowserMfa:
		return nil, &ErrNoEligibleMFADevices
	}

	resp, err := c.PromptConstructor(promptOpts...).Run(ctx, chal)
	return resp, trace.Wrap(err)
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

type RegistrationReason string

const (
	// RegistrationReasonExplicit indicates that the user explicitly asked to
	// register an MFA device.
	RegistrationReasonExplicit RegistrationReason = "explicit"
	// RegistrationReasonSessionMFANoDevices indicates that the registration is
	// attempted because a per-session MFA is required, but there are no MFA
	// devices registered.
	RegistrationReasonSessionMFANoDevices RegistrationReason = "session_mfa_no_devices"
	// RegistrationReasonSessionMFANoEligibleDevices indicates that the
	// registration is attempted because a per-session MFA is required, but none
	// of the registered MFA devices are eligible.
	RegistrationReasonSessionMFANoEligibleDevices RegistrationReason = "session_mfa_no_eligible_devices"
)

// Register interacts with user to register an MFA device on the client side
// and adds the device to Teleport backend. Returns true if the device was
// added, and false if it was not (for example, the user refused to register
// it).
func (c *Ceremony) Register(ctx context.Context, cfg RegistrationPromptConfig) (bool, error) {
	if c.CreateRegisterChallenge == nil {
		return false, trace.BadParameter("mfa ceremony must have CreateRegisterChallenge set in order to begin")
	}

	if c.AddMFADevice == nil {
		return false, trace.BadParameter("mfa ceremony must have AddMFADevice set in order to begin")
	}

	regPrompt := c.PromptConstructor(WithPromptDeviceType(DeviceDescriptorNew))

	if cfg.DeviceType == "" && len(cfg.DeviceTypeOptions) == 0 {
		// If we are prompting the user for the device type, then take a glimpse at
		// server-side settings and adjust the options accordingly.
		// This is undesirable to do during flag setup, but we can do it here.
		if c.Ping == nil {
			return false, trace.BadParameter("mfa ceremony must have Ping set in order to prompt for a device type")
		}
		pingResp, err := c.Ping(ctx)
		if err != nil {
			return false, trace.Wrap(err)
		}

		switch pingResp.Auth.SecondFactor {
		case constants.SecondFactorOTP:
			cfg.DeviceTypeOptions = totpDeviceTypes
		case constants.SecondFactorWebauthn:
			cfg.DeviceTypeOptions = webDeviceTypes
		default:
			cfg.DeviceTypeOptions = DefaultDeviceTypes
		}
	}

	// If the device is being created because of a session MFA, exclude TOTP, as
	// it's not eligible.
	switch cfg.Reason {
	case RegistrationReasonSessionMFANoDevices,
		RegistrationReasonSessionMFANoEligibleDevices:
		cfg.DeviceTypeOptions = slices.DeleteFunc(cfg.DeviceTypeOptions, func(d MFADeviceType) bool {
			return d == MFADeviceTypeTOTP
		})
	}

	// Attempt the actual interactive registration.
	regCfg, err := regPrompt.AskRegister(ctx, cfg)
	if err != nil {
		if errors.Is(err, &ErrDeniedRegister) {
			// No device has been registered.
			return false, nil
		}
		return false, trace.Wrap(err)
	}

	mfaResp, err := c.Authenticate(ctx, &proto.CreateAuthenticateChallengeRequest{
		ChallengeExtensions: &mfav1.ChallengeExtensions{
			Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_MANAGE_DEVICES,
		},
	}, WithPromptDeviceType(DeviceDescriptorRegistered))
	if errors.Is(err, &ErrNoMFADevices) {
		// First-device registration is allowed without an existing MFA response.
		mfaResp = &proto.MFAAuthenticateResponse{}
	} else if err != nil {
		return false, trace.Wrap(err)
	}

	devTypePB := registrationDeviceTypeProto(regCfg.DeviceType)
	if devTypePB == proto.DeviceType_DEVICE_TYPE_UNSPECIFIED {
		return false, trace.BadParameter("unexpected device type: %q", regCfg.DeviceType)
	}

	regChal, err := c.CreateRegisterChallenge(ctx, &proto.CreateRegisterChallengeRequest{
		ExistingMFAResponse: mfaResp,
		DeviceType:          devTypePB,
		DeviceUsage:         regCfg.DeviceUsage,
	})
	if err != nil {
		return false, trace.Wrap(err)
	}

	result, err := regPrompt.RunRegister(ctx, regCfg, regChal)
	if err != nil {
		return false, trace.Wrap(err)
	}
	if result == nil {
		return false, nil
	}

	// Add the registered device to the backend.
	if err = c.AddMFADevice(ctx, result.Response, regCfg); err != nil {
		result.Callbacks.Rollback()
		return false, trace.Wrap(err)
	}
	if err := result.Callbacks.Confirm(); err != nil {
		return false, trace.Wrap(err)
	}

	regPrompt.NotifyRegistrationSuccess(ctx, regCfg)
	return true, nil
}

func registrationDeviceTypeProto(deviceType MFADeviceType) proto.DeviceType {
	return map[MFADeviceType]proto.DeviceType{
		MFADeviceTypeTOTP:     proto.DeviceType_DEVICE_TYPE_TOTP,
		MFADeviceTypeWebauthn: proto.DeviceType_DEVICE_TYPE_WEBAUTHN,
		MFADeviceTypeTouchID:  proto.DeviceType_DEVICE_TYPE_WEBAUTHN,
	}[deviceType]
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
	// even if the current context has a reusable  v18 server will
	// return this requirement as expected.
	// TODO(Joerger): DELETE IN v19.0.0
	ceremonyCtx := ContextWithMFAResponse(ctx, nil)

	resp, err := mfaCeremony(ceremonyCtx, challengeRequest, WithPromptReasonAdminAction())
	return resp, trace.Wrap(err)
}

const (
	// cliMFATypeOTP is the CLI display name for OTP.
	cliMFATypeOTP = "OTP"
	// cliMFATypeWebauthn is the CLI display name for Webauthn.
	cliMFATypeWebauthn = "WEBAUTHN"
	// cliMFATypeSSO is the CLI display name for SSO.
	cliMFATypeSSO = "SSO"
	// cliMFATypeBrowserMFA is the CLI display name for Browser
	cliMFATypeBrowserMFA = "BROWSER"
)
