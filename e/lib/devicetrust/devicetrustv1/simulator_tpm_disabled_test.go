//go:build !tpmsimulator

package devicetrustv1_test

import (
	"context"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
)

// package to use when `tpmsimulator` build tag is not provided.
// implementation to use when provided is in tpm_simulator_enabled_test.go

// inform tests they need to skip tpm based cases
var tpmSkip = "test skipped as tpmsimulator tag not set"

type tpmSimulator struct{}

// newTPMSimulator here needs to match the signature of the one in
// tpm_simulator_enable_test.go
func newTPMSimulator(_ tpmBehavior) *tpmSimulator {
	return &tpmSimulator{}
}

func (e *tpmSimulator) setup() (func(), error) {
	panic("unimplemented")
}

func (e *tpmSimulator) enrollRequest(
	dev *devicepb.Device,
	enrollToken string,
) *devicepb.EnrollDeviceRequest {
	panic("unimplemented")
}

func (e *tpmSimulator) handleEnrollStream(
	resp *devicepb.EnrollDeviceResponse,
	stream devicepb.DeviceTrustService_EnrollDeviceClient,
	testBehavior bool,
) (*devicepb.Device, error) {
	panic("unimplemented")
}

func (e *tpmSimulator) wantCredential() *devicepb.DeviceCredential {
	panic("unimplemented")
}

func (e *tpmSimulator) authenticate(
	ctx context.Context,
	dev *devicepb.Device,
	stream devicepb.DeviceTrustService_AuthenticateDeviceClient,
	initTemplate *devicepb.AuthenticateDeviceInit,
) (*devicepb.AuthenticateDeviceResponse, error) {
	panic("unimplemented")
}
