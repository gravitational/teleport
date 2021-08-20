package webauthn

import (
	"github.com/duo-labs/webauthn/protocol"

	wan "github.com/duo-labs/webauthn/webauthn"
)

const (
	defaultDisplayName = "Teleport"
	defaultIcon        = ""
)

// Config represents the Webauthn configuration.
// TODO(codingllama): Replace with types.Webauthn once it's merged.
type Config struct {
	RPID                                        string
	AttestationAllowedCAs, AttestationDeniedCAs []string
}

func newWebAuthn(cfg *Config, rpID, origin string) (*wan.WebAuthn, error) {
	var attestation protocol.ConveyancePreference
	if len(cfg.AttestationAllowedCAs) > 0 && len(cfg.AttestationDeniedCAs) > 0 {
		attestation = protocol.PreferDirectAttestation
	}
	return wan.New(&wan.Config{
		RPID:                  rpID,
		RPOrigin:              origin,
		RPDisplayName:         defaultDisplayName,
		RPIcon:                defaultIcon,
		AttestationPreference: attestation,
	})
}
