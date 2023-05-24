package devicetrustv1_test

import devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"

type tpmBehavior struct {
	incorrectAttestAK             bool
	incorrectAttestNonce          bool
	incorrectAttestPCR            bool
	incorrectAttestEvent          bool
	incorrectCredActivateSolution bool

	modifyEnrollDeviceInit func(r *devicepb.EnrollDeviceInit)
}
