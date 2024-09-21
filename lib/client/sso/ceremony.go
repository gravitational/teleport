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

type MFACeremony struct {
	*Ceremony
}

// GetClientCallbackURL returns the SSO client callback URL.
func (c *MFACeremony) GetClientCallbackURL() string {
	return c.clientCallbackURL
}

// HandleRedirect handles redirect.
func (c *MFACeremony) HandleRedirect(ctx context.Context, redirectURL string) error {
	return c.Ceremony.HandleRedirect(ctx, redirectURL)
}

// GetCallbackMFAResponse returns an SSO MFA auth response once the callback is complete.
func (c *MFACeremony) GetCallbackMFAToken(ctx context.Context) (string, error) {
	loginResp, err := c.GetCallbackResponse(ctx)
	if err != nil {
		return "", trace.Wrap(err)
	}

	if loginResp.MFAToken == "" {
		return "", trace.BadParameter("login response for SSO MFA flow missing MFA token")
	}

	return loginResp.MFAToken, nil
}
