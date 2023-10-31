// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package daemon

import (
	"context"
	"errors"
	"io"
	"sync"

	"github.com/gravitational/trace"
	"github.com/gravitational/trace/trail"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/mfa"
	api "github.com/gravitational/teleport/gen/proto/go/teleport/lib/teleterm/v1"
	wancli "github.com/gravitational/teleport/lib/auth/webauthncli"
	wantypes "github.com/gravitational/teleport/lib/auth/webauthntypes"
	libmfa "github.com/gravitational/teleport/lib/client/mfa"
)

// mfaPrompt is a tshd implementation of mfa.Prompt that uses the
// tshdEventsClient to propagate mfa prompts to the Electron App.
type mfaPrompt struct {
	cfg              libmfa.PromptConfig
	clusterURI       string
	tshdEventsClient api.TshdEventsServiceClient
	logger           *logrus.Logger
}

// NewMFAPromptConstructor returns a new MFA prompt constructor
// for this service and the given cluster.
func (s *Service) NewMFAPromptConstructor(clusterURI string) func(cfg *libmfa.PromptConfig) mfa.Prompt {
	return func(cfg *libmfa.PromptConfig) mfa.Prompt {
		return s.NewMFAPrompt(clusterURI, cfg)
	}
}

// NewMFAPrompt returns a new MFA prompt for this service and the given cluster.
func (s *Service) NewMFAPrompt(clusterURI string, cfg *libmfa.PromptConfig) *mfaPrompt {
	return &mfaPrompt{
		cfg:              *cfg,
		clusterURI:       clusterURI,
		tshdEventsClient: s.tshdEventsClient,
		logger:           s.cfg.Log.Logger,
	}
}

// Run prompts the user to complete an MFA authentication challenge.
func (p *mfaPrompt) Run(ctx context.Context, chal *proto.MFAAuthenticateChallenge) (*proto.MFAAuthenticateResponse, error) {
	runOpts, err := p.cfg.GetRunOptions(ctx, chal)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	type response struct {
		kind string
		resp *proto.MFAAuthenticateResponse
		err  error
	}
	respC := make(chan response, 2)
	var wg sync.WaitGroup

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Fire Electron notification goroutine.
	wg.Add(1)
	go func() {
		defer wg.Done()

		resp, err := p.promptApp(ctx, chal, runOpts)
		respC <- response{kind: "APP", resp: resp, err: err}
	}()

	// Fire Webauthn goroutine.
	if runOpts.PromptWebauthn {
		wg.Add(1)
		go func() {
			defer wg.Done()

			resp, err := p.promptWebauthn(ctx, chal)
			respC <- response{kind: "WEBAUTHN", resp: resp, err: err}
		}()
	}

	// Wait for the 1-2 authn goroutines above to complete, then close respC.
	go func() {
		wg.Wait()
		close(respC)
	}()

	// Wait for a successful response, or terminating error, from the 1-2 authn goroutines.
	// The goroutine above will ensure the response channel is closed once all goroutines are done.
	for resp := range respC {
		switch err := resp.err; {
		case errors.Is(err, wancli.ErrUsingNonRegisteredDevice):
			// Surface error immediately.
			return nil, trace.Wrap(resp.err)
		case err != nil:
			p.logger.WithError(err).Debugf("%s authentication failed", resp.kind)
			// Continue to give the other authn goroutine a chance to succeed.
			// If both have failed, this will exit the loop.
			continue
		}

		// Return successful response.
		return resp.resp, nil
	}

	// If no successful response is returned, this means the authn goroutines were unsuccessful.
	// This usually occurs when the prompt times out or no devices are available to prompt.
	// Return a user readable error message.
	return nil, trace.BadParameter("failed to authenticate using available MFA devices, rerun the command with '-d' to see error details for each device")
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

func (p *mfaPrompt) promptApp(ctx context.Context, chal *proto.MFAAuthenticateChallenge, runOpts libmfa.RunOpts) (*proto.MFAAuthenticateResponse, error) {
	resp, err := p.tshdEventsClient.PromptMFA(ctx, &api.PromptMFARequest{
		RootClusterUri: p.clusterURI,
		Reason:         p.cfg.PromptReason,
		Totp:           runOpts.PromptTOTP,
		Webauthn:       runOpts.PromptWebauthn,
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
