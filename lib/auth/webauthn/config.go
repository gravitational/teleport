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
	"github.com/duo-labs/webauthn/protocol"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/defaults"

	wan "github.com/duo-labs/webauthn/webauthn"
)

const (
	defaultDisplayName = "Teleport"
	defaultIcon        = ""
)

func newWebAuthn(cfg *types.Webauthn, rpID, origin string) (*wan.WebAuthn, error) {
	attestation := protocol.PreferNoAttestation
	if len(cfg.AttestationAllowedCAs) > 0 || len(cfg.AttestationDeniedCAs) > 0 {
		attestation = protocol.PreferDirectAttestation
	}
	rrk := false
	return wan.New(&wan.Config{
		RPID:                  rpID,
		RPOrigin:              origin,
		RPDisplayName:         defaultDisplayName,
		RPIcon:                defaultIcon,
		AttestationPreference: attestation,
		AuthenticatorSelection: protocol.AuthenticatorSelection{
			RequireResidentKey: &rrk,
			// Do not ask for PIN (or other verifications), users already go through
			// password authn.
			UserVerification: protocol.VerificationDiscouraged,
		},
		Timeout: int(defaults.WebauthnChallengeTimeout.Milliseconds()),
	})
}
