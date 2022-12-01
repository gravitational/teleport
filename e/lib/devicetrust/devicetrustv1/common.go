package devicetrustv1

import (
	"context"

	"github.com/gravitational/trace"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/e/lib/devicetrust/storage"
	dtoss "github.com/gravitational/teleport/lib/devicetrust"
)

func findDeviceBySerial(ctx context.Context, s *storage.S, osType devicepb.OSType, serialNumber string) (*devicepb.Device, error) {
	devs, err := s.GetDevicesByAssetTag(ctx, serialNumber)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for _, dev := range devs {
		if dev.OsType == osType {
			return dev, nil
		}
	}
	return nil, trace.NotFound("device %q/%v not registered", serialNumber, dtoss.FriendlyOSType(osType))
}
