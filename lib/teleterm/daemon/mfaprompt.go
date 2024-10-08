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
	"sync"

	"github.com/gravitational/trace"
	"github.com/gravitational/trace/trail"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/mfa"
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
	if err := s.importantModalSemaphore.Acquire(ctx); err != nil {
		return nil, trace.Wrap(err)
	}
	defer s.importantModalSemaphore.Release()

	return s.tshdEventsClient.PromptMFA(ctx, in)
}

// Run prompts the user to complete an MFA authentication challenge.
func (p *mfaPrompt) Run(ctx context.Context, chal *proto.MFAAuthenticateChallenge) (*proto.MFAAuthenticateResponse, error) {
	runOpts, err := p.cfg.GetRunOptions(ctx, chal)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// No prompt to run, no-op.
	if !runOpts.PromptTOTP && !runOpts.PromptWebauthn {
		return &proto.MFAAuthenticateResponse{}, nil
	}

	// Depending on the run opts, we may spawn a TOTP goroutine, webauth goroutine, or both.
	spawnGoroutines := func(ctx context.Context, wg *sync.WaitGroup, respC chan<- libmfa.MFAGoroutineResponse) {
		ctx, cancel := context.WithCancelCause(ctx)

		// Fire App goroutine (TOTP).
		wg.Add(1)
		go func() {
			defer wg.Done()

			resp, err := p.promptMFA(ctx, chal, runOpts)
			respC <- libmfa.MFAGoroutineResponse{Resp: resp, Err: err}

			// If the user closes the modal in the Electron app, we need to be able to cancel the other
			// goroutine as well so that we stop waiting for the hardware key tap.
			if err != nil && status.Code(err) == codes.Aborted {
				cancel(err)
			}
		}()

		// Fire Webauthn goroutine.
		if runOpts.PromptWebauthn {
			wg.Add(1)
			go func() {
				defer wg.Done()

				resp, err := p.promptWebauthn(ctx, chal)
				respC <- libmfa.MFAGoroutineResponse{Resp: resp, Err: trace.Wrap(err, "Webauthn authentication failed")}
			}()
		}
	}

	return libmfa.HandleMFAPromptGoroutines(ctx, spawnGoroutines)
}

func (p *mfaPrompt) promptWebauthn(ctx context.Context, chal *proto.MFAAuthenticateChallenge) (*proto.MFAAuthenticateResponse, error) {
	prompt := wancli.NewDefaultPrompt(ctx, io.Discard)
	opts := &wancli.LoginOpts{AuthenticatorAttachment: p.cfg.AuthenticatorAttachment}
	resp, _, err := p.cfg.WebauthnLoginFunc(ctx, p.cfg.GetWebauthnOrigin(), wantypes.CredentialAssertionFromProto(chal.WebauthnChallenge), prompt, opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return resp, nil
}

func (p *mfaPrompt) promptMFA(ctx context.Context, chal *proto.MFAAuthenticateChallenge, runOpts libmfa.RunOpts) (*proto.MFAAuthenticateResponse, error) {
	resp, err := p.promptAppMFA(ctx, &api.PromptMFARequest{
		ClusterUri: p.resourceURI.GetClusterURI().String(),
		Reason:     p.cfg.PromptReason,
		Totp:       runOpts.PromptTOTP,
		Webauthn:   runOpts.PromptWebauthn,
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
