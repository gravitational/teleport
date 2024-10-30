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

package services

import "github.com/gravitational/teleport/lib/auth/mfatypes"

// SSOMFASessionData SSO MFA Session data.
type SSOMFASessionData struct {
	// RequestID is the ID of the corresponding SSO Auth request, which is used to
	// identity this session.
	RequestID string `json:"request_id,omitempty"`
	// Username is the Teleport username.
	Username string `json:"username,omitempty"`
	// Token is an active token used to verify the owner of this SSO MFA session data.
	Token string `json:"token,omitempty"`
	// ConnectorID is id of the corresponding Auth connector.
	ConnectorID string `json:"connector_id,omitempty"`
	// ConnectorType is SSO type of the corresponding Auth connector (SAML, OIDC).
	ConnectorType string `json:"connector_type,omitempty"`
	// ChallengeExtensions are Teleport extensions that apply to this SSO MFA session.
	ChallengeExtensions *mfatypes.ChallengeExtensions `json:"challenge_extensions"`
}
