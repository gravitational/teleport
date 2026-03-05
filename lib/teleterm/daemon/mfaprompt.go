/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package daemon

import (
	"context"
	"io"

	"github.com/gravitational/trace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/gravitational/teleport/api/client/proto"
	mfav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v1"
	"github.com/gravitational/teleport/api/mfa"
	"github.com/gravitational/teleport/api/trail"
	api "github.com/gravitational/teleport/gen/proto/go/teleport/lib/teleterm/v1"
	wancli "github.com/gravitational/teleport/lib/auth/webauthncli"
	wantypes "github.com/gravitational/teleport/lib/auth/webauthntypes"
	libmfa "github.com/gravitational/teleport/lib/client/mfa"
	"github.com/gravitational/teleport/lib/teleterm/api/uri"
)

// mfaPrompt is a tshd implementation of mfa.Prompt that uses the
// tshdEventsClient to propagate mfa prompts to the Electron App.
type mfaPrompt struct {
	cfg          libmfa.PromptConfig
	resourceURI  uri.ResourceURI
	promptAppMFA func(ctx context.Context, in *api.PromptMFARequest) (*api.PromptMFAResponse, error)
}

// NewMFAPromptConstructor returns a new MFA prompt constructor
// for this service and the given resource URI.
func (s *Service) NewMFAPromptConstructor(resourceURI uri.ResourceURI) func(cfg *libmfa.PromptConfig) mfa.Prompt {
	return func(cfg *libmfa.PromptConfig) mfa.Prompt {
		return s.NewMFAPrompt(resourceURI, cfg)
	}
}

// NewMFAPrompt returns a new MFA prompt for this service and the given resource URI.
func (s *Service) NewMFAPrompt(resourceURI uri.ResourceURI, cfg *libmfa.PromptConfig) *mfaPrompt {
	return &mfaPrompt{
		cfg:          *cfg,
		resourceURI:  resourceURI,
		promptAppMFA: s.promptAppMFA,
	}
}

func (s *Service) promptAppMFA(ctx context.Context, in *api.PromptMFARequest) (*api.PromptMFAResponse, error) {
	s.mfaMu.Lock()
	defer s.mfaMu.Unlock()

	return s.cfg.TshdEventsClient.client.PromptMFA(ctx, in)
}

// Run prompts the user to complete an MFA authentication challenge.
func (p *mfaPrompt) Run(ctx context.Context, chal *proto.MFAAuthenticateChallenge) (*proto.MFAAuthenticateResponse, error) {
	promptOTP := chal.TOTP != nil
	promptWebauthn := chal.WebauthnChallenge != nil && p.cfg.WebauthnSupported
	promptSSO := chal.SSOChallenge != nil && p.cfg.CallbackCeremony != nil
	promptBrowserMfa := chal.BrowserMFAChallenge != nil && p.cfg.CallbackCeremony != nil

	// No prompt to run, no-op.
	if !promptOTP && !promptWebauthn && !promptSSO && !promptBrowserMfa {
		return &proto.MFAAuthenticateResponse{}, nil
	}

	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)

	appPrompt := func(ctx context.Context, chal *proto.MFAAuthenticateChallenge) (*proto.MFAAuthenticateResponse, error) {
		resp, err := p.promptApp(ctx, chal)

		// If the user closes the modal in the Electron app, we need to be able to cancel the other
		// goroutine as well so that we stop waiting for the hardware key tap.
		if err != nil && status.Code(err) == codes.Aborted {
			cancel(err)
		}

		return resp, trace.Wrap(err)
	}

	return libmfa.HandleConcurrentMFAPrompts(ctx, chal, appPrompt, p.maybePromptWebauthn, p.maybePromptBrowserOrSSO)
}

// promptApp handles the client modal, cancellation, and TOTP.
func (p *mfaPrompt) promptApp(ctx context.Context, chal *proto.MFAAuthenticateChallenge) (*proto.MFAAuthenticateResponse, error) {
	promptOTP := chal.TOTP != nil
	promptWebauthn := chal.WebauthnChallenge != nil && p.cfg.WebauthnSupported
	promptSSO := chal.SSOChallenge != nil && p.cfg.CallbackCeremony != nil
	promptBrowserMfa := chal.BrowserMFAChallenge != nil && p.cfg.CallbackCeremony != nil
	scope := p.cfg.Extensions.GetScope()

	var ssoChallenge *api.SSOChallenge
	if promptSSO {
		ssoChallenge = &api.SSOChallenge{
			ConnectorId:   chal.SSOChallenge.Device.ConnectorId,
			ConnectorType: chal.SSOChallenge.Device.ConnectorType,
			DisplayName:   chal.SSOChallenge.Device.DisplayName,
			RedirectUrl:   chal.SSOChallenge.RedirectUrl,
		}
	}

	var browserMfaChallenge *mfav1.BrowserMFAChallenge
	if promptBrowserMfa {
		browserMfaChallenge = &mfav1.BrowserMFAChallenge{
			RequestId: chal.BrowserMFAChallenge.RequestId,
		}
	}

	resp, err := p.promptAppMFA(ctx, &api.PromptMFARequest{
		ClusterUri:    p.resourceURI.GetClusterURI().String(),
		Reason:        p.cfg.PromptReason,
		Totp:          promptOTP,
		Webauthn:      promptWebauthn,
		Sso:           ssoChallenge,
		Browser:       browserMfaChallenge,
		PerSessionMfa: scope == mfav1.ChallengeScope_CHALLENGE_SCOPE_USER_SESSION,
	})
	if err != nil {
		return nil, trail.FromGRPC(err)
	}
	return &proto.MFAAuthenticateResponse{
		Response: &proto.MFAAuthenticateResponse_TOTP{
			TOTP: &proto.TOTPResponse{Code: resp.TotpCode},
		},
	}, nil
}

// Prompt Webauthn if it's a supported method.
func (p *mfaPrompt) maybePromptWebauthn(ctx context.Context, chal *proto.MFAAuthenticateChallenge) (*proto.MFAAuthenticateResponse, error) {
	if chal.WebauthnChallenge == nil || !p.cfg.WebauthnSupported {
		return nil, nil
	}

	prompt := wancli.NewDefaultPrompt(ctx, io.Discard)
	opts := &wancli.LoginOpts{AuthenticatorAttachment: p.cfg.AuthenticatorAttachment}
	resp, _, err := p.cfg.WebauthnLoginFunc(ctx, p.cfg.GetWebauthnOrigin(), wantypes.CredentialAssertionFromProto(chal.WebauthnChallenge), prompt, opts)
	if err != nil {
		return nil, trace.Wrap(err, "Webauthn authentication failed")
	}
	return resp, nil
}

// Prompt Browser/SSO if it's a supported method.
func (p *mfaPrompt) maybePromptBrowserOrSSO(ctx context.Context, chal *proto.MFAAuthenticateChallenge) (*proto.MFAAuthenticateResponse, error) {
	if (chal.SSOChallenge == nil && chal.BrowserMFAChallenge == nil) || p.cfg.CallbackCeremony == nil {
		return nil, nil
	}

	resp, err := p.cfg.CallbackCeremony.Run(ctx, chal)
	return resp, trace.Wrap(err)
}

// AskRegister prompts user for device details and registers a new MFA device.
func (f *mfaPrompt) AskRegister(ctx context.Context, config mfa.RegistrationPromptConfig) (*mfa.RegistrationResult, error) {
	return nil, trace.NotImplemented("not supported")
}

// NotifyRegistrationSuccess notifies the user that the device registration was
// successful.
func (f *mfaPrompt) NotifyRegistrationSuccess(_ context.Context, _ mfa.RegistrationPromptConfig) error {
	return trace.NotImplemented("not supported")
}
