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

package client

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client/proto"
	mfav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v1"
	"github.com/gravitational/teleport/api/mfa"
	libmfa "github.com/gravitational/teleport/lib/client/mfa"
	"github.com/gravitational/teleport/lib/client/sso"
)

// NewMFACeremony returns a new MFA ceremony configured for this client.
func (tc *TeleportClient) NewMFACeremony() *mfa.Ceremony {
	return &mfa.Ceremony{
		CreateAuthenticateChallenge: tc.createAuthenticateChallenge,
		PromptConstructor:           tc.NewMFAPrompt,
		MFACeremonyConstructor:      tc.NewRedirectorMFACeremony,
	}
}

// createAuthenticateChallenge creates and returns MFA challenges for a users registered MFA devices.
func (tc *TeleportClient) createAuthenticateChallenge(ctx context.Context, req *proto.CreateAuthenticateChallengeRequest) (*proto.MFAAuthenticateChallenge, error) {
	clusterClient, err := tc.ConnectToCluster(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	rootClient, err := clusterClient.ConnectToRootCluster(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return rootClient.CreateAuthenticateChallenge(ctx, req)
}

func (tc *TeleportClient) NewSessionBoundCeremony(ctx context.Context) (*mfa.SessionBoundCeremony, error) {
	clusterClient, err := tc.ConnectToCluster(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	rootClient, err := clusterClient.ConnectToRootCluster(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	mfaClient := rootClient.MFAServiceClient()
	if mfaClient == nil {
		return nil, trace.BadParameter("MFA service client is not initialized (this is a bug)")
	}

	config := mfa.SessionBoundCeremonyConfig{
		CreateSessionChallenge:      mfaClient.CreateSessionChallenge,
		ValidateSessionChallenge:    mfaClient.ValidateSessionChallenge,
		PromptConstructor:           tc.NewMFAPrompt,
		CallbackCeremonyConstructor: tc.NewRedirectorMFACeremony,
		TargetCluster:               tc.SiteName,
	}

	ceremony, err := mfa.NewSessionBoundCeremony(config)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return ceremony, nil
}

func (tc *TeleportClient) PerformSessionMFACeremony(ctx context.Context, sessionID []byte) (challengeName string, err error) {
	ceremony, err := tc.NewSessionBoundCeremony(ctx)
	if err != nil {
		return "", trace.Wrap(err)
	}

	name, err := ceremony.Run(
		ctx,
		&mfav1.SessionIdentifyingPayload{
			Payload: &mfav1.SessionIdentifyingPayload_SshSessionId{
				SshSessionId: sessionID,
			},
		},
	)
	if err != nil {
		return "", trace.Wrap(err)
	}

	return name, nil
}

// WebauthnLoginFunc is a function that performs WebAuthn login.
// Mimics the signature of [webauthncli.Login].
type WebauthnLoginFunc = libmfa.WebauthnLoginFunc

// NewMFAPrompt creates a new MFA prompt from client settings.
func (tc *TeleportClient) NewMFAPrompt(opts ...mfa.PromptOpt) mfa.Prompt {
	cfg := tc.newPromptConfig(opts...)

	var prompt mfa.Prompt = libmfa.NewCLIPrompt(&libmfa.CLIPromptConfig{
		PromptConfig:     *cfg,
		Writer:           tc.Stderr,
		PreferOTP:        tc.PreferOTP,
		PreferSSO:        tc.PreferSSO,
		PreferBrowser:    tc.PreferBrowser,
		AllowStdinHijack: tc.AllowStdinHijack,
		StdinFunc:        tc.StdinFunc,
	})

	if tc.MFAPromptConstructor != nil {
		prompt = tc.MFAPromptConstructor(cfg)
	}

	return prompt
}

func (tc *TeleportClient) newPromptConfig(opts ...mfa.PromptOpt) *libmfa.PromptConfig {
	cfg := libmfa.NewPromptConfig(tc.WebProxyAddr, opts...)
	cfg.AuthenticatorAttachment = tc.AuthenticatorAttachment
	if tc.WebauthnLogin != nil {
		cfg.WebauthnLoginFunc = tc.WebauthnLogin
		cfg.WebauthnSupported = true
	}

	return cfg
}

// NewRedirectorMFACeremony creates a new redirector for SSO or Browser MFA ceremony.
func (tc *TeleportClient) NewRedirectorMFACeremony(ctx context.Context) (mfa.CallbackCeremony, error) {
	rdConfig, err := tc.ssoRedirectorConfig(ctx, "" /*connectorDisplayName*/)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	rd, err := sso.NewRedirector(rdConfig)
	if err != nil {
		return nil, trace.Wrap(err, "failed to create a redirector for SSO MFA")
	}

	if tc.MFACeremonyConstructor != nil {
		return tc.MFACeremonyConstructor(rd), nil
	}

	return sso.NewCLIMFACeremony(rd), nil
}
