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
	wancli "github.com/gravitational/teleport/lib/auth/webauthncli"
	wantypes "github.com/gravitational/teleport/lib/auth/webauthntypes"
	"github.com/gravitational/teleport/lib/client/mfa"
)

// PromptMFAFunc matches the signature of [mfa.Prompt.Run].
type PromptMFAFunc func(ctx context.Context, chal *proto.MFAAuthenticateChallenge) (*proto.MFAAuthenticateResponse, error)

// WebauthnLoginFunc matches the signature of [wancli.Login].
type WebauthnLoginFunc func(ctx context.Context, origin string, assertion *wantypes.CredentialAssertion, prompt wancli.LoginPrompt, opts *wancli.LoginOpts) (*proto.MFAAuthenticateResponse, string, error)

// NewMFAPrompt creates a new MFA prompt from client settings.
func (tc *TeleportClient) NewMFAPrompt(opts ...mfa.PromptOpt) PromptMFAFunc {
	if tc.PromptMFAFunc != nil {
		return tc.PromptMFAFunc
	}

	prompt := mfa.NewPrompt(tc.WebProxyAddr)
	prompt.AuthenticatorAttachment = tc.AuthenticatorAttachment
	prompt.PreferOTP = tc.PreferOTP
	prompt.AllowStdinHijack = tc.AllowStdinHijack

	if tc.WebauthnLogin != nil {
		prompt.WebauthnLogin = tc.WebauthnLogin
		prompt.WebauthnSupported = true
	}

	for _, opt := range opts {
		opt(prompt)
	}

	return prompt.Run
}

// PromptMFA prompts for MFA for the given challenge using the clients standard settings.
// Use [NewMFAPrompt] to create a prompt with customizable settings.
func (tc *TeleportClient) PromptMFA(ctx context.Context, chal *proto.MFAAuthenticateChallenge) (*proto.MFAAuthenticateResponse, error) {
	return tc.NewMFAPrompt()(ctx, chal)
}
