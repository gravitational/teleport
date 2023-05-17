package testenv

import (
	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/trace"
)

// FakeLinuxDevice only implements GetOSType Linux OS type so we can be sure
// this fails in a user friendly manner.
type FakeLinuxDevice struct {
}

func NewFakeLinuxDevice() *FakeLinuxDevice {
	return &FakeLinuxDevice{}
}

func (d *FakeLinuxDevice) GetOSType() devicepb.OSType {
	return devicepb.OSType_OS_TYPE_LINUX
}

func (d *FakeLinuxDevice) CollectDeviceData() (*devicepb.DeviceCollectedData, error) {
	return nil, trace.NotImplemented("linux device fake unimplemented")
}

func (d *FakeLinuxDevice) EnrollDeviceInit() (*devicepb.EnrollDeviceInit, error) {
	return nil, trace.NotImplemented("linux device fake unimplemented")
}

func (d *FakeLinuxDevice) SignChallenge(_ []byte) (sig []byte, err error) {
	return nil, trace.NotImplemented("linux device fake unimplemented")
}

func (d *FakeLinuxDevice) SolveTPMEnrollChallenge(_ *devicepb.TPMEnrollChallenge) (*devicepb.TPMEnrollChallengeResponse, error) {
	return nil, trace.NotImplemented("linux device fake unimplemented")
}
