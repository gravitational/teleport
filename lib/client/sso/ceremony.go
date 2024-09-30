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

package sso

import (
	"context"

	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/trace"
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

	return c.GetCallbackResponse(ctx)
}

// NewCLICeremony creates a new CLI SSO ceremony from the given redirector.
func NewCLICeremony(rd *Redirector) *Ceremony {
	return &Ceremony{
		clientCallbackURL:   rd.ClientCallbackURL,
		HandleRedirect:      rd.OpenRedirect,
		GetCallbackResponse: rd.WaitForResponse,
	}
}
