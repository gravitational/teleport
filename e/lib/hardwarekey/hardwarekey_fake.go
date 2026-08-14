//go:build pivtest

package hardwarekey

import (
	"github.com/gravitational/trace"

	attestation "github.com/gravitational/teleport/api/gen/proto/go/attestation/v1"
	"github.com/gravitational/teleport/api/utils/keys"
)

// fakeAttestationData is used by tests to set fake attestation data.
var fakeAttestationData *keys.AttestationData

func attestYubikey(stm *attestation.YubiKeyAttestationStatement) (*keys.AttestationData, error) {
	if fakeAttestationData == nil {
		return nil, trace.BadParameter("fakeAttestationData not set for testing")
	}
	return fakeAttestationData, nil
}
