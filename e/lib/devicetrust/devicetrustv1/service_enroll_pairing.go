package devicetrustv1

import (
	"context"

	"github.com/gravitational/trace"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/api/types"
)

// CreateEnrollPairing creates an EnrollPairing for the calling user.
// Returns AlreadyExists if a pairing already exists.
func (s *Service) CreateEnrollPairing(ctx context.Context, _ *devicepb.CreateEnrollPairingRequest) (*devicepb.CreateEnrollPairingResponse, error) {
	authCtx, err := s.authorizeAccess(ctx, types.KindMobileDevice, types.VerbCreateEnrollToken)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	pairing, err := s.enrollPairing.CreateEnrollPairing(ctx, authCtx.User.GetName())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return devicepb.CreateEnrollPairingResponse_builder{
		EnrollPairing: pairing,
	}.Build(), nil
}

// GetCurrentEnrollPairing returns the EnrollPairing for the calling user.
// Returns NotFound if none exists.
func (s *Service) GetCurrentEnrollPairing(ctx context.Context, _ *devicepb.GetCurrentEnrollPairingRequest) (*devicepb.GetCurrentEnrollPairingResponse, error) {
	// Check for mobile_device.create_enroll_token on Get too. If the user was
	// able to call CreateEnrollPairing but the permission has since been revoked,
	// we don't want them to be able to continue the enrollment process.
	authCtx, err := s.authorizeAccess(ctx, types.KindMobileDevice, types.VerbCreateEnrollToken)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	pairing, err := s.enrollPairing.GetCurrentEnrollPairing(ctx, authCtx.User.GetName())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return devicepb.GetCurrentEnrollPairingResponse_builder{
		EnrollPairing: pairing,
	}.Build(), nil
}
