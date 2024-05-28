/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package mfa

import (
	"context"
	"errors"
	"strings"
	"sync"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/mfa"
	wancli "github.com/gravitational/teleport/lib/auth/webauthncli"
	wantypes "github.com/gravitational/teleport/lib/auth/webauthntypes"
)

// PromptConfig contains common mfa prompt config options.
type PromptConfig struct {
	mfa.PromptConfig
	// ProxyAddress is the address of the authenticating proxy. required.
	ProxyAddress string
	// WebauthnLoginFunc performs client-side Webauthn login.
	WebauthnLoginFunc func(ctx context.Context, origin string, assertion *wantypes.CredentialAssertion, prompt wancli.LoginPrompt, opts *wancli.LoginOpts) (*proto.MFAAuthenticateResponse, string, error)
	// AllowStdinHijack allows stdin hijack during MFA prompts.
	// Stdin hijack provides a better login UX, but it can be difficult to reason
	// about and is often a source of bugs.
	// Do not set this options unless you deeply understand what you are doing.
	// If false then only the strongest auth method is prompted.
	AllowStdinHijack bool
	// AuthenticatorAttachment specifies the desired authenticator attachment.
	AuthenticatorAttachment wancli.AuthenticatorAttachment
	// PreferOTP favors OTP challenges, if applicable.
	// Takes precedence over AuthenticatorAttachment settings.
	PreferOTP bool
	// WebauthnSupported indicates whether Webauthn is supported.
	WebauthnSupported bool
}

// NewPromptConfig returns a prompt config that will induce default behavior.
func NewPromptConfig(proxyAddr string, opts ...mfa.PromptOpt) *PromptConfig {
	cfg := &PromptConfig{
		ProxyAddress:      proxyAddr,
		WebauthnLoginFunc: wancli.Login,
		WebauthnSupported: wancli.HasPlatformSupport(),
	}

	for _, opt := range opts {
		opt(&cfg.PromptConfig)
	}

	return cfg
}

// RunOpts are mfa prompt run options.
type RunOpts struct {
	PromptTOTP     bool
	PromptWebauthn bool
}

// GetRunOptions gets mfa prompt run options by cross referencing the mfa challenge with prompt configuration.
func (c PromptConfig) GetRunOptions(ctx context.Context, chal *proto.MFAAuthenticateChallenge) (RunOpts, error) {
	promptTOTP := chal.TOTP != nil
	promptWebauthn := chal.WebauthnChallenge != nil

	// Does the current platform support hardware MFA? Adjust accordingly.
	switch {
	case !promptTOTP && !c.WebauthnSupported:
		return RunOpts{}, trace.BadParameter("hardware device MFA not supported by your platform, please register an OTP device")
	case !c.WebauthnSupported:
		// Do not prompt for hardware devices, it won't work.
		promptWebauthn = false
	}

	// Tweak enabled/disabled methods according to opts.
	switch {
	case promptTOTP && c.PreferOTP:
		promptWebauthn = false
	case promptWebauthn && c.AuthenticatorAttachment != wancli.AttachmentAuto:
		// Prefer Webauthn if an specific attachment was requested.
		promptTOTP = false
	case promptWebauthn && !c.AllowStdinHijack:
		// Use strongest auth if hijack is not allowed.
		promptTOTP = false
	}

	return RunOpts{promptTOTP, promptWebauthn}, nil
}

func (c PromptConfig) GetWebauthnOrigin() string {
	if !strings.HasPrefix(c.ProxyAddress, "https://") {
		return "https://" + c.ProxyAddress
	}
	return c.ProxyAddress
}

// MFAGoroutineResponse is an MFA goroutine response.
type MFAGoroutineResponse struct {
	Resp *proto.MFAAuthenticateResponse
	Err  error
}

// HandleMFAPromptGoroutines spawns MFA prompt goroutines and returns the first successful response,
// terminating error, or an aggregated error if they all fail.
func HandleMFAPromptGoroutines(ctx context.Context, startGoroutines func(context.Context, *sync.WaitGroup, chan<- MFAGoroutineResponse)) (*proto.MFAAuthenticateResponse, error) {
	respC := make(chan MFAGoroutineResponse, 2)
	var wg sync.WaitGroup

	ctx, cancel := context.WithCancel(ctx)
	defer func() {
		cancel()
		// wait for all goroutines to complete to ensure there are no leaks.
		wg.Wait()
	}()

	startGoroutines(ctx, &wg, respC)

	// Wait for spawned goroutines above to complete, then close respC.
	go func() {
		wg.Wait()
		close(respC)
	}()

	// Wait for a successful response, or terminating error, from the spawned goroutines.
	// The goroutine above will ensure the response channel is closed once all goroutines are done.
	var errs []error
	for resp := range respC {
		switch err := resp.Err; {
		case errors.Is(err, wancli.ErrUsingNonRegisteredDevice):
			// Surface error immediately.
			return nil, trace.Wrap(resp.Err)
		case err != nil:
			errs = append(errs, err)
			// Continue to give the other authn goroutine a chance to succeed.
			// If both have failed, this will exit the loop.
			continue
		}

		// Return successful response.
		return resp.Resp, nil
	}

	if len(errs) == 0 {
		return &proto.MFAAuthenticateResponse{}, nil
	}

	return nil, trace.Wrap(trace.NewAggregate(errs...), "failed to authenticate using available MFA devices")
}
