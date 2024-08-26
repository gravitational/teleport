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

package webauthn

import (
	wan "github.com/go-webauthn/webauthn/webauthn"

	"github.com/gravitational/teleport/api/types"
)

// webUser implements a WebAuthn protocol user.
// It is used to provide user information to WebAuthn APIs, but has no direct
// counterpart in storage nor in other packages.
type webUser struct {
	credentials []wan.Credential
	name        string
	webID       []byte
}

type webUserOpts struct {
	name             string
	webID            []byte
	devices          []*types.MFADevice
	credentialIDOnly bool
	currentFlags     *credentialFlags
}

func newWebUser(opts webUserOpts) *webUser {
	var credentials []wan.Credential
	for _, dev := range opts.devices {
		c, ok := deviceToCredential(dev, opts.credentialIDOnly, opts.currentFlags)
		if ok {
			credentials = append(credentials, c)
		}
	}
	return &webUser{
		credentials: credentials,
		name:        opts.name,
		webID:       opts.webID,
	}
}

func (w *webUser) WebAuthnID() []byte {
	return w.webID
}

func (w *webUser) WebAuthnName() string {
	return w.name
}

func (w *webUser) WebAuthnDisplayName() string {
	return w.name
}

func (w *webUser) WebAuthnIcon() string {
	return ""
}

func (w *webUser) WebAuthnCredentials() []wan.Credential {
	return w.credentials
}
