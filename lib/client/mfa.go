/*
Copyright 2021 Gravitational, Inc.

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

package client

import (
	"context"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/mfa"
	wancli "github.com/gravitational/teleport/lib/auth/webauthncli"
	wantypes "github.com/gravitational/teleport/lib/auth/webauthntypes"
	libmfa "github.com/gravitational/teleport/lib/client/mfa"
)

// WebauthnLoginFunc matches the signature of [wancli.Login].
type WebauthnLoginFunc func(ctx context.Context, origin string, assertion *wantypes.CredentialAssertion, prompt wancli.LoginPrompt, opts *wancli.LoginOpts) (*proto.MFAAuthenticateResponse, string, error)

// NewMFAPrompt creates a new MFA prompt from client settings.
func (tc *TeleportClient) NewMFAPrompt(opts ...mfa.PromptOpt) mfa.Prompt {
	cfg := tc.newPromptConfig(opts...)
	var prompt mfa.Prompt = libmfa.NewCLIPrompt(cfg, tc.Stderr)
	return prompt
}

// PromptMFA runs a standard MFA prompt from client settings.
func (tc *TeleportClient) PromptMFA(ctx context.Context, chal *proto.MFAAuthenticateChallenge) (*proto.MFAAuthenticateResponse, error) {
	return tc.NewMFAPrompt().Run(ctx, chal)
}

func (tc *TeleportClient) newPromptConfig(opts ...mfa.PromptOpt) *libmfa.PromptConfig {
	cfg := libmfa.NewPromptConfig(tc.WebProxyAddr, opts...)
	cfg.AuthenticatorAttachment = tc.AuthenticatorAttachment
	cfg.PreferOTP = tc.PreferOTP
	cfg.AllowStdinHijack = tc.AllowStdinHijack

	if tc.WebauthnLogin != nil {
		cfg.WebauthnLoginFunc = tc.WebauthnLogin
		cfg.WebauthnSupported = true
	}
	return cfg
}
