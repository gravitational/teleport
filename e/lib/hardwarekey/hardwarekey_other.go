//go:build !piv && !pivtest

package hardwarekey

import (
	"errors"

	"github.com/gravitational/trace"

	attestation "github.com/gravitational/teleport/api/gen/proto/go/attestation/v1"
	"github.com/gravitational/teleport/api/utils/keys"
)

var errPIVUnavailable = errors.New("PIV is unavailable in current build")

// attestYubikey verifies that the given slot certificate chains to the attestation certificate,
// which chains to a Yubico CA.
func attestYubikey(stm *attestation.YubiKeyAttestationStatement) (*keys.AttestationData, error) {
	return nil, trace.Wrap(errPIVUnavailable)
}
