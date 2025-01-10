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

package servicecfg

import (
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
)

// JamfCredentials is the credentials for the Jamf MDM service.
type JamfCredentials struct {
	// Username is the Jamf API username.
	// Username and password are used to acquire short-lived Jamf Pro API tokens.
	// See https://developer.jamf.com/jamf-pro/docs/jamf-pro-api-overview.
	// Prefer using client_id and client_secret.
	// Either username+password or client_id+client_secret are required.
	Username string
	// Password is the Jamf API password.
	// Username and password are used to acquire short-lived Jamf Pro API tokens.
	// See https://developer.jamf.com/jamf-pro/docs/jamf-pro-api-overview.
	// Prefer using client_id and client_secret.
	// Either username+password or client_id+client_secret are required.
	Password string
	// ClientID is the Jamf API client ID.
	// See https://developer.jamf.com/jamf-pro/docs/client-credentials.
	// Either username+password or client_id+client_secret are required.
	ClientID string
	// ClientSecret is the Jamf API client secret.
	// See https://developer.jamf.com/jamf-pro/docs/client-credentials.
	// Either username+password or client_id+client_secret are required
	ClientSecret string
}

// ValidateJamfCredentials validates the Jamf credentials.
func ValidateJamfCredentials(j *JamfCredentials) error {
	hasUserPass := j.Username != "" && j.Password != ""
	hasAPICreds := j.ClientID != "" && j.ClientSecret != ""
	switch {
	case !hasUserPass && !hasAPICreds:
		return trace.BadParameter("either username+password or clientID+clientSecret must be provided")
	}
	return nil
}

// JamfConfig is the configuration for the Jamf MDM service.
type JamfConfig struct {
	// Spec is the configuration spec.
	Spec *types.JamfSpecV1
	// Credentials are the Jamf API credentials.
	Credentials *JamfCredentials
	// ExitOnSync controls whether the service performs a single sync operation
	// before exiting.
	ExitOnSync bool
}

func (j *JamfConfig) Enabled() bool {
	return j != nil && j.Spec != nil && j.Spec.Enabled
}
