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

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/mfa"
	wancli "github.com/gravitational/teleport/lib/auth/webauthncli"
	wantypes "github.com/gravitational/teleport/lib/auth/webauthntypes"
)

// WebauthnLoginFunc is a function that performs WebAuthn login.
// Mimics the signature of [wancli.Login].
type WebauthnLoginFunc func(
	ctx context.Context,
	origin string,
	assertion *wantypes.CredentialAssertion,
	prompt wancli.LoginPrompt,
	opts *wancli.LoginOpts,
) (*proto.MFAAuthenticateResponse, string, error)

// PromptConfig contains common mfa prompt config options shared by
// different implementations of [mfa.Prompt].
type PromptConfig struct {
	mfa.PromptConfig
	// ProxyAddress is the address of the authenticating proxy. required.
	ProxyAddress string
	// WebauthnLoginFunc performs client-side Webauthn login.
	WebauthnLoginFunc WebauthnLoginFunc
	// AuthenticatorAttachment specifies the desired authenticator attachment.
	AuthenticatorAttachment wancli.AuthenticatorAttachment
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

func (c PromptConfig) GetWebauthnOrigin() string {
	if !strings.HasPrefix(c.ProxyAddress, "https://") {
		return "https://" + c.ProxyAddress
	}
	return c.ProxyAddress
}

// HandleConcurrentMFAPrompts handles concurrently prompting for MFA with all
// of the given promptFuncs and returning the the first successful response,
// terminating error, or an aggregated error if they all fail.
func HandleConcurrentMFAPrompts(ctx context.Context, chal *proto.MFAAuthenticateChallenge, promptFuncs ...mfa.PromptFunc) (*proto.MFAAuthenticateResponse, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	type promptResponse struct {
		resp *proto.MFAAuthenticateResponse
		err  error
	}

	respC := make(chan promptResponse, len(promptFuncs))
	for _, prompt := range promptFuncs {
		go func() {
			resp, err := prompt(ctx, chal)
			respC <- promptResponse{
				resp: resp,
				err:  err,
			}
		}()
	}

	// Wait for a successful response, or terminating error, from the spawned goroutines.
	var errs []error
	for range len(promptFuncs) {
		resp := <-respC

		switch err := resp.err; {
		case errors.Is(err, wancli.ErrUsingNonRegisteredDevice):
			// Surface error immediately.
			return nil, trace.Wrap(resp.err)
		case err != nil:
			log.
				WithError(err).
				Debug("MFA goroutine failed, continuing so other goroutines have a chance to succeed")
			errs = append(errs, err)
			// Continue to give the other authn goroutine a chance to succeed.
			// If both have failed, this will exit the loop.
			continue
		}

		if resp.resp == nil {
			continue
		}

		// Return successful response.
		return resp.resp, nil
	}

	// If the prompts result in no response or error, it's a no-op.
	if len(errs) == 0 {
		return &proto.MFAAuthenticateResponse{}, nil
	}

	return nil, trace.Wrap(trace.NewAggregate(errs...), "failed to authenticate using available MFA devices")
}
