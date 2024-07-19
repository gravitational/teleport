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
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/mfa"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authclient"
	wancli "github.com/gravitational/teleport/lib/auth/webauthncli"
	wantypes "github.com/gravitational/teleport/lib/auth/webauthntypes"
	libmfa "github.com/gravitational/teleport/lib/client/mfa"
	"github.com/gravitational/teleport/lib/client/sso"
)

// WebauthnLoginFunc matches the signature of [wancli.Login].
type WebauthnLoginFunc func(ctx context.Context, origin string, assertion *wantypes.CredentialAssertion, prompt wancli.LoginPrompt, opts *wancli.LoginOpts) (*proto.MFAAuthenticateResponse, string, error)

// NewMFAPrompt creates a new MFA prompt from client settings.
func (tc *TeleportClient) NewMFAPrompt(opts ...mfa.PromptOpt) mfa.Prompt {
	cfg := tc.newPromptConfig(opts...)

	var prompt mfa.Prompt = libmfa.NewCLIPrompt(cfg, tc.Stderr)
	if tc.MFAPromptConstructor != nil {
		prompt = tc.MFAPromptConstructor(cfg)
	}

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

	cfg.SSOLoginFunc = func(ctx context.Context, connectorID, connectorType string) (*proto.MFAAuthenticateResponse, error) {
		rdConfig, err := tc.SSORedirectorConfig(ctx, connectorID, connectorID, connectorType)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		rdConfig.InitiateSSOLoginFn = func(clientRedirectURL string) (redirectURL string, err error) {
			err = tc.WithRootClusterClient(ctx, func(clt authclient.ClientI) error {
				switch connectorType {
				case constants.OIDC:
					resp, err := clt.CreateOIDCAuthRequest(ctx, types.OIDCAuthRequest{
						Username:              tc.Username,
						ConnectorID:           connectorID,
						Type:                  connectorType,
						CheckUser:             true,
						ClientRedirectURL:     clientRedirectURL,
						CreatePrivilegedToken: true,
					})
					if err != nil {
						return trace.Wrap(err)
					}
					redirectURL = resp.RedirectURL
				case constants.SAML:
					resp, err := clt.CreateSAMLAuthRequest(ctx, types.SAMLAuthRequest{
						Username:              tc.Username,
						ConnectorID:           connectorID,
						Type:                  connectorType,
						ClientRedirectURL:     clientRedirectURL,
						CreatePrivilegedToken: true,
					})
					if err != nil {
						return trace.Wrap(err)
					}
					redirectURL = resp.RedirectURL
				case constants.Github:
					resp, err := clt.CreateGithubAuthRequest(ctx, types.GithubAuthRequest{
						Username:              tc.Username,
						ConnectorID:           connectorID,
						Type:                  connectorType,
						ClientRedirectURL:     clientRedirectURL,
						CreatePrivilegedToken: true,
					})
					if err != nil {
						return trace.Wrap(err)
					}
					redirectURL = resp.RedirectURL
				}
				return nil
			})
			return redirectURL, trace.Wrap(err)
		}

		rd, err := sso.NewRedirector(ctx, rdConfig)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		resp, err := rd.SSOLoginCeremony(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		if resp.Token == "" {
			panic("no token")
		}

		return &proto.MFAAuthenticateResponse{
			Response: &proto.MFAAuthenticateResponse_TokenID{
				TokenID: resp.Token,
			},
		}, nil
	}

	return cfg
}
