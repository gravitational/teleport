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
	"github.com/go-webauthn/webauthn/protocol"
	wan "github.com/go-webauthn/webauthn/webauthn"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/defaults"
)

const (
	defaultDisplayName = "Teleport"
)

// webAuthnParams groups the parameters necessary for the creation of
// wan.WebAuthn instances.
type webAuthnParams struct {
	cfg                     *types.Webauthn
	rpID                    string
	origin                  string
	requireResidentKey      bool
	requireUserVerification bool
}

func newWebAuthn(p webAuthnParams) (*wan.WebAuthn, error) {
	attestation := protocol.PreferNoAttestation
	if len(p.cfg.AttestationAllowedCAs) > 0 || len(p.cfg.AttestationDeniedCAs) > 0 {
		attestation = protocol.PreferDirectAttestation
	}

	residentKeyRequirement := protocol.ResidentKeyRequirementDiscouraged
	if p.requireResidentKey {
		residentKeyRequirement = protocol.ResidentKeyRequirementRequired
	}

	// Default to "discouraged", otherwise some browsers may do needless PIN
	// prompts.
	userVerification := protocol.VerificationDiscouraged
	if p.requireUserVerification {
		userVerification = protocol.VerificationRequired
	}

	timeoutConfig := wan.TimeoutConfig{
		Enforce:    true,
		Timeout:    defaults.WebauthnChallengeTimeout,
		TimeoutUVD: defaults.WebauthnChallengeTimeout,
	}

	return wan.New(&wan.Config{
		RPID:                  p.rpID,
		RPOrigins:             []string{p.origin},
		RPDisplayName:         defaultDisplayName,
		AttestationPreference: attestation,
		AuthenticatorSelection: protocol.AuthenticatorSelection{
			RequireResidentKey: &p.requireResidentKey,
			ResidentKey:        residentKeyRequirement,
			UserVerification:   userVerification,
		},
		Timeouts: wan.TimeoutsConfig{
			Login:        timeoutConfig,
			Registration: timeoutConfig,
		},
	})
}
