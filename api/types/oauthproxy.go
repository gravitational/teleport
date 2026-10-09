/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package types

import (
	"time"

	"github.com/gravitational/trace"
)

// OAuthProxyAuthRequest is an in-flight request for an OAuth Proxy authorization.
// It's single use and any persisted request is deleted on completion.
type OAuthProxyAuthRequest struct {
	StateToken   string    `json:"state_token"`
	Integration  string    `json:"integration"`
	CodeVerifier string    `json:"code_verifier"`
	User         string    `json:"user"`
	Expires      time.Time `json:"expires"`
}

// CheckAndSetDefaults validates the configuration for an OAuthProxyAuthRequest and sets default
// values for any missing optional fields.
func (o *OAuthProxyAuthRequest) CheckAndSetDefaults() error {
	switch {
	case o.StateToken == "":
		return trace.BadParameter("state_token is required")
	case o.Integration == "":
		return trace.BadParameter("integration is required")
	case o.CodeVerifier == "":
		return trace.BadParameter("code_verifier is required")
	case o.User == "":
		return trace.BadParameter("user is required")
	case o.Expires.IsZero():
		return trace.BadParameter("expires is required")
	}

	return nil
}
