package storage

import (
	"context"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
)

// GetDeviceCollecteDataForTests gets collected data for the specified deviceID,
// regardless of the device itself existing.
// Used to assert collected data deletion in tests.
func (s *S) GetDeviceCollecteDataForTests(ctx context.Context, deviceID string) ([]*devicepb.DeviceCollectedData, error) {
	return s.getDeviceCollectedData(ctx, deviceID)
}
