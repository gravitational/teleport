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
	"google.golang.org/grpc"

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
		CreateSessionChallenge:      tc.CreateSessionChallenge,
		ValidateSessionChallenge:    tc.ValidateSessionChallenge,
		PromptConstructor:           tc.NewMFAPrompt,
		SSOMFACeremonyConstructor:   tc.NewSSOMFACeremony,
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

// CreateSessionChallenge creates a session-bound MFA challenge by connecting to the MFA service on the root cluster.
func (tc *TeleportClient) CreateSessionChallenge(ctx context.Context, req *mfav1.CreateSessionChallengeRequest, opts ...grpc.CallOption) (*mfav1.CreateSessionChallengeResponse, error) {
	clusterClient, err := tc.ConnectToCluster(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	rootClient, err := clusterClient.ConnectToRootCluster(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return rootClient.MFAServiceClient().CreateSessionChallenge(ctx, req)
}

// ValidateSessionChallenge validates a session-bound MFA challenge by connecting to the MFA service on the root cluster.
func (tc *TeleportClient) ValidateSessionChallenge(ctx context.Context, req *mfav1.ValidateSessionChallengeRequest, opts ...grpc.CallOption) (*mfav1.ValidateSessionChallengeResponse, error) {
	clusterClient, err := tc.ConnectToCluster(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	rootClient, err := clusterClient.ConnectToRootCluster(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return rootClient.MFAServiceClient().ValidateSessionChallenge(ctx, req, opts...)
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

// NewSSOMFACeremony creates a new SSO MFA ceremony.
func (tc *TeleportClient) NewSSOMFACeremony(ctx context.Context) (mfa.SSOMFACeremony, error) {
	rdConfig, err := tc.ssoRedirectorConfig(ctx, "" /*connectorDisplayName*/)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	rd, err := sso.NewRedirector(rdConfig)
	if err != nil {
		return nil, trace.Wrap(err, "failed to create a redirector for SSO MFA")
	}

	if tc.SSOMFACeremonyConstructor != nil {
		return tc.SSOMFACeremonyConstructor(rd), nil
	}

	return sso.NewCLIMFACeremony(rd), nil
}
