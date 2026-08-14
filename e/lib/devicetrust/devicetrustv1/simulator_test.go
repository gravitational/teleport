package devicetrustv1_test

import (
	"context"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
)

// simulator pretends to be a client device, and can have its behavior tweaked
// to trigger unhappy paths.
type simulator interface {
	// setup performs any initial setup on the simulator before it can be used.
	// This must be called first. It always returns a "closer" function which
	// must be called when the subtest using the simulator finishes.
	setup() (closer func(), err error)
	enrollRequest(dev *devicepb.Device, enrollToken string) *devicepb.EnrollDeviceRequest
	// handleEnrollStream runs the client side element of the device enrollment
	// flow. testBehavior must be set to true for the simulators behavior
	// to be respected. This allows us to disable any custom behavior when
	// using this as a helper in other tests (e.g enrolling for device authn
	// testing).
	handleEnrollStream(
		resp *devicepb.EnrollDeviceResponse,
		stream devicepb.DeviceTrustService_EnrollDeviceClient,
		testBehavior bool,
	) (*devicepb.Device, error)
	wantCredential() *devicepb.DeviceCredential
	// authenticate runs the client side element of the device authentication
	// flow.
	//
	// initTemplate is used to supply both UserCertificates and DeviceWebToken,
	// other fields are overwritten.
	authenticate(
		ctx context.Context,
		dev *devicepb.Device,
		stream devicepb.DeviceTrustService_AuthenticateDeviceClient,
		initTemplate *devicepb.AuthenticateDeviceInit,
	) (*devicepb.AuthenticateDeviceResponse, error)
}
