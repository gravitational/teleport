package internal

import (
	"context"

	"github.com/gravitational/trace"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/e/lib/devicetrust/storage"
	"github.com/gravitational/teleport/lib/authz"
	dtoss "github.com/gravitational/teleport/lib/devicetrust"
	"github.com/gravitational/teleport/lib/utils"
)

func FindDeviceBySerial(ctx context.Context, s *storage.S, osType devicepb.OSType, serialNumber string) (*devicepb.Device, error) {
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

func ProtectReadOnlyDeviceDataFields(dcd *devicepb.DeviceCollectedData) error {
	// Whilst other system managed fields are simply overwritten or ignored,
	// this field is especially sensitive as if injected by a hostile client,
	// it could be used to bypass checks that consider historical TPM PCR state.
	// Because of this, we take more affirmative action and reject the request.
	if dcd.TpmPlatformAttestation != nil {
		return trace.BadParameter("tpm_platform_attestation is a read only field and cannot be submitted in device collected data")
	}
	return nil
}

func getSourceIPFromContext(ctx context.Context) (string, error) {
	sourceAddr, err := authz.ClientSrcAddrFromContext(ctx)
	if err != nil {
		return "", trace.Wrap(err, "read source address from context")
	}

	sourceIP, _, err := utils.SplitHostPort(sourceAddr.String())
	if err != nil {
		return "", trace.Wrap(err, "split host/port from source address")
	}

	return sourceIP, nil
}
