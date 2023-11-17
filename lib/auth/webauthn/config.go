/*
Copyright 2021 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
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
	defaultIcon        = ""
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

	return wan.New(&wan.Config{
		RPID:                  p.rpID,
		RPOrigins:             []string{p.origin},
		RPDisplayName:         defaultDisplayName,
		RPIcon:                defaultIcon,
		AttestationPreference: attestation,
		AuthenticatorSelection: protocol.AuthenticatorSelection{
			RequireResidentKey: &p.requireResidentKey,
			ResidentKey:        residentKeyRequirement,
			UserVerification:   userVerification,
		},
		Timeout: int(defaults.WebauthnChallengeTimeout.Milliseconds()),
	})
}
