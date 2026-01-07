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
	"fmt"
	"net/url"
	"path"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/mfa"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/client/browser"
	libmfa "github.com/gravitational/teleport/lib/client/mfa"
	"github.com/gravitational/teleport/lib/client/sso"
	"github.com/gravitational/teleport/lib/services"
)

// NewMFACeremony returns a new MFA ceremony configured for this client.
func (tc *TeleportClient) NewMFACeremony() *mfa.Ceremony {
	return &mfa.Ceremony{
		CreateAuthenticateChallenge: tc.createAuthenticateChallenge,
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
		PreferBrowserMFA: tc.PreferBrowserMFA,
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

// browserLoginWithType performs browser-based authentication by opening the user's
// browser to complete the authentication flow. This is the shared implementation
// used by browserLogin (for initial login) and browserMFA (for per-session MFA).
//
// The authType parameter specifies the type of headless authentication to perform.
// The message parameter customizes the prompt displayed to the user before opening
// the browser.
func (tc *TeleportClient) browserLoginWithType(ctx context.Context, keyRing *KeyRing, authType types.HeadlessAuthenticationType, message string) (*authclient.SSHLoginResponse, error) {
	browserAuthenticationID := services.NewHeadlessAuthenticationID(keyRing.SSHPrivateKey.MarshalSSHPublicKey())

	u := &url.URL{
		Scheme: "https",
		Host:   tc.WebProxyAddr,
		Path:   path.Join("web", "headless", browserAuthenticationID),
	}
	if tc.Username != "" {
		u.RawQuery = url.Values{"user": []string{tc.Username}}.Encode()
	}
	webUILink := u.String()

	_ = browser.OpenURLInBrowser(tc.Browser, webUILink)
	fmt.Fprintf(tc.Stderr, "%s:\n\n%s\n", message, webUILink)

	tlsPub, err := keyRing.TLSPrivateKey.MarshalTLSPublicKey()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	response, err := SSHAgentHeadlessLogin(ctx, SSHLoginHeadless{
		SSHLogin: SSHLogin{
			ProxyAddr:         tc.WebProxyAddr,
			SSHPubKey:         keyRing.SSHPrivateKey.MarshalSSHPublicKey(),
			TLSPubKey:         tlsPub,
			TTL:               tc.KeyTTL,
			Insecure:          tc.InsecureSkipVerify,
			Compatibility:     tc.CertificateFormat,
			KubernetesCluster: tc.KubernetesCluster,
		},
		User:                     tc.Username,
		RemoteAuthenticationType: authType,
		HeadlessAuthenticationID: browserAuthenticationID,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return response, nil
}

// browserLogin performs browser-based MFA authentication for getting user certs
func (tc *TeleportClient) browserLogin(ctx context.Context, keyRing *KeyRing) (*authclient.SSHLoginResponse, error) {
	return tc.browserLoginWithType(ctx, keyRing, types.HeadlessAuthenticationType_HEADLESS_AUTHENTICATION_TYPE_BROWSER, "Complete login in your web browser")
}

// browserMFA performs browser-based MFA authentication for per-session MFA.
// This is an alternative to the challenge-response MFA ceremony and is used when
// the server supports the Browser MFA.
// Unlike the ceremony flow, this method returns certificates directly rather than
// an MFAAuthenticateResponse, as authentication happens out-of-band via the browser.
func (tc *TeleportClient) browserMFA(ctx context.Context, keyRing *KeyRing) (*PerformSessionMFACeremonyResult, error) {
	response, err := tc.browserLoginWithType(ctx, keyRing, types.HeadlessAuthenticationType_HEADLESS_AUTHENTICATION_TYPE_SESSION, "Complete MFA authentication in your web browser")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	certs := &proto.Certs{
		SSH: response.Cert,
		TLS: response.TLSCert,
	}

	keyRing.Cert = certs.SSH
	keyRing.TLSCert = certs.TLS

	return &PerformSessionMFACeremonyResult{
		KeyRing:  keyRing,
		NewCerts: certs,
	}, nil
}
