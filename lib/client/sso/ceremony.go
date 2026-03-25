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

package sso

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/mfa"
	"github.com/gravitational/teleport/lib/auth/authclient"
)

// Ceremony is a customizable SSO login ceremony.
type Ceremony struct {
	clientCallbackURL   string
	Init                CeremonyInit
	HandleRedirect      func(ctx context.Context, redirectURL string) error
	GetCallbackResponse func(ctx context.Context) (*authclient.SSHLoginResponse, error)
}

// CeremonyInit initializes an SSO login ceremony.
type CeremonyInit func(ctx context.Context, clientCallbackURL string) (redirectURL string, err error)

// Run the SSO ceremony.
func (c *Ceremony) Run(ctx context.Context) (*authclient.SSHLoginResponse, error) {
	redirectURL, err := c.Init(ctx, c.clientCallbackURL)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := c.HandleRedirect(ctx, redirectURL); err != nil {
		return nil, trace.Wrap(err)
	}

	resp, err := c.GetCallbackResponse(ctx)
	return resp, trace.Wrap(err)
}

// NewCLICeremony creates a new CLI SSO ceremony from the given redirector.
func NewCLICeremony(rd *Redirector, init CeremonyInit) *Ceremony {
	return &Ceremony{
		clientCallbackURL:   rd.ClientCallbackURL,
		Init:                init,
		HandleRedirect:      rd.OpenRedirect,
		GetCallbackResponse: rd.WaitForResponse,
	}
}

// SAMLCeremony handles SAML login ceremony.
type SAMLCeremony struct {
	clientCallbackURL   string
	Init                SAMLCeremonyInit
	HandleRequest       func(ctx context.Context, redirectURL, postformData string) error
	GetCallbackResponse func(ctx context.Context) (*authclient.SSHLoginResponse, error)
}

// SAMLCeremonyInit initializes an SAML based SSO login ceremony.
type SAMLCeremonyInit func(ctx context.Context, clientCallbackURL string) (redirectURL, postformData string, err error)

// Run the SAML SSO ceremony.
func (c *SAMLCeremony) Run(ctx context.Context) (*authclient.SSHLoginResponse, error) {
	redirectURL, postformData, err := c.Init(ctx, c.clientCallbackURL)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := c.HandleRequest(ctx, redirectURL, postformData); err != nil {
		return nil, trace.Wrap(err)
	}

	resp, err := c.GetCallbackResponse(ctx)
	return resp, trace.Wrap(err)
}

// NewCLISAMLCeremony creates a new CLI SAML SSO ceremony from the given redirector.
func NewCLISAMLCeremony(rd *Redirector, init SAMLCeremonyInit) *SAMLCeremony {
	return &SAMLCeremony{
		clientCallbackURL:   rd.ClientCallbackURL,
		Init:                init,
		HandleRequest:       rd.OpenLoginURL,
		GetCallbackResponse: rd.WaitForResponse,
	}
}

// Ceremony is a customizable SSO MFA ceremony.
type MFACeremony struct {
	close               func()
	ClientCallbackURL   string
	ProxyAddress        string
	HandleRedirect      func(ctx context.Context, redirectURL string) error
	GetCallbackMFAToken func(ctx context.Context) (string, error)
}

// GetClientCallbackURL returns the client callback URL.
func (m *MFACeremony) GetClientCallbackURL() string {
	return m.ClientCallbackURL
}

// GetProxyAddress returns the proxy address the client is connected to.
func (m *MFACeremony) GetProxyAddress() string {
	return m.ProxyAddress
}

// Run the SSO MFA ceremony.
func (m *MFACeremony) Run(ctx context.Context, chal *proto.MFAAuthenticateChallenge) (*proto.MFAAuthenticateResponse, error) {
	if err := m.HandleRedirect(ctx, chal.SSOChallenge.RedirectUrl); err != nil {
		return nil, trace.Wrap(err)
	}

	mfaToken, err := m.GetCallbackMFAToken(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &proto.MFAAuthenticateResponse{
		Response: &proto.MFAAuthenticateResponse_SSO{
			SSO: &proto.SSOResponse{
				RequestId: chal.SSOChallenge.RequestId,
				Token:     mfaToken,
			},
		},
	}, nil
}

// Close closes resources associated with the SSO MFA ceremony.
func (m *MFACeremony) Close() {
	if m.close != nil {
		m.close()
	}
}

// NewCLIMFACeremony creates a new CLI SSO ceremony from the given redirector.
// The returned MFACeremony takes ownership of the Redirector.
func NewCLIMFACeremony(rd *Redirector) *MFACeremony {
	return &MFACeremony{
		close:             rd.Close,
		ClientCallbackURL: rd.ClientCallbackURL,
		ProxyAddress:      rd.ProxyAddr,
		HandleRedirect:    rd.OpenRedirect,
		GetCallbackMFAToken: func(ctx context.Context) (string, error) {
			loginResp, err := rd.WaitForResponse(ctx)
			if err != nil {
				return "", trace.Wrap(err)
			}

			if loginResp.MFAToken == "" {
				return "", trace.BadParameter("login response for SSO MFA flow missing MFA token")
			}

			return loginResp.MFAToken, nil
		},
	}
}

// NewConnectMFACeremony creates a new Teleport Connect SSO ceremony from the given redirector.
func NewConnectMFACeremony(rd *Redirector) mfa.SSOMFACeremony {
	return &MFACeremony{
		close:             rd.Close,
		ClientCallbackURL: rd.ClientCallbackURL,
		ProxyAddress:      rd.ProxyAddr,
		HandleRedirect: func(ctx context.Context, redirectURL string) error {
			// Connect handles redirect on the Electron side.
			return nil
		},
		GetCallbackMFAToken: func(ctx context.Context) (string, error) {
			loginResp, err := rd.WaitForResponse(ctx)
			if err != nil {
				return "", trace.Wrap(err)
			}

			if loginResp.MFAToken == "" {
				return "", trace.BadParameter("login response for SSO MFA flow missing MFA token")
			}

			return loginResp.MFAToken, nil
		},
	}
}
