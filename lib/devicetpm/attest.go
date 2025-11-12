package devicetpm

import "github.com/google/go-attestation/attest"

// OpenTPM calls [attest.OpenTPM] using TPM version 2.0.
func OpenTPM(config *attest.OpenConfig) (*attest.TPM, error) {
	if config == nil {
		config = &attest.OpenConfig{}
	}
	config.TPMVersion = attest.TPMVersion20
	return attest.OpenTPM(config)
}

// ActivationParameters prepares ap for TPM version 2.0.
func ActivationParameters(ap *attest.ActivationParameters) *attest.ActivationParameters {
	if ap != nil {
		ap.TPMVersion = attest.TPMVersion20
	}
	return ap
}

// ParseAKPublic calls [attest.ParseAKPublic] using TPM version 2.0.
func ParseAKPublic(public []byte) (*attest.AKPublic, error) {
	return attest.ParseAKPublic(attest.TPMVersion20, public)
}
