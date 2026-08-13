package devicetrustv1

import (
	"context"
	"crypto/subtle"

	"github.com/gravitational/trace"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/events"
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

// ApproveEnrollPairing moves the calling user's EnrollPairing to APPROVED,
// provided it matches req.PairingToken, unblocking CreatePairedDeviceEnrollToken
// on the public Device Trust service.
func (s *Service) ApproveEnrollPairing(ctx context.Context, req *devicepb.ApproveEnrollPairingRequest) (_ *devicepb.ApproveEnrollPairingResponse, err error) {
	authCtx, err := s.authorizeAccess(ctx, types.KindMobileDevice, types.VerbCreateEnrollToken)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Approval releases an enrollment token for a device that is registered but
	// not yet enrolled, so it is gated like the other mutating RPCs of this
	// service. Unlike those it doesn't accept a reused MFA response, since
	// approving is a one-off decision rather than something done in bulk.
	if err := authCtx.AuthorizeAdminAction(); err != nil {
		return nil, trace.Wrap(err)
	}

	var device *devicepb.EnrollPairingDevice
	var alreadyApproved bool
	defer func() {
		if !alreadyApproved {
			s.emitEnrollPairingApproveEvent(ctx, device, err)
		}
	}()

	pairing, err := s.enrollPairingMatchingToken(ctx, authCtx.User.GetName(), req.GetPairingToken())
	// Assigned before the error check so a token mismatch, which returns the
	// fetched pairing alongside the error, still audits the pending device.
	device = pairing.GetStatus().GetDevice()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// An already approved pairing means this is a retry: an earlier call moved
	// the pairing to APPROVED but its response was lost, e.g. to a dropped
	// connection, so the user ran the MFA ceremony and approved again. The
	// approval already happened and was audited, so report success and emit
	// nothing.
	if pairing.GetStatus().GetState() == devicepb.EnrollPairingState_ENROLL_PAIRING_STATE_APPROVED {
		alreadyApproved = true
		return &devicepb.ApproveEnrollPairingResponse{}, nil
	}

	if _, err := s.enrollPairing.ApproveEnrollPairing(ctx, pairing); err != nil {
		return nil, trace.Wrap(err)
	}
	return &devicepb.ApproveEnrollPairingResponse{}, nil
}

// DenyEnrollPairing deletes the EnrollPairing for the calling user, provided it
// matches req.PairingToken.
func (s *Service) DenyEnrollPairing(ctx context.Context, req *devicepb.DenyEnrollPairingRequest) (*devicepb.DenyEnrollPairingResponse, error) {
	authCtx, err := s.authorizeAccess(ctx, types.KindMobileDevice, types.VerbCreateEnrollToken)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Unlike ApproveEnrollPairing, DenyEnrollPairing takes no MFA ceremony.
	// Denial only withholds enrollment, so a session thief gains nothing from it
	// that they could not already achieve (e.g. DoS by hitting
	// CreatePairedDeviceEnrollToken with bogus data and the token), and rejecting
	// a suspicious request should not cost more than ignoring it.

	pairing, err := s.enrollPairingMatchingToken(ctx, authCtx.User.GetName(), req.GetPairingToken())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.enrollPairing.DeleteEnrollPairing(ctx, pairing); err != nil {
		return nil, trace.Wrap(err)
	}

	// Only a completed denial is audited. Unlike ApproveEnrollPairing, deny
	// failures deliberately emit no event. RFD 32e defines no deny failure code
	// and a deny that fails changes nothing, while a failed approval attempt is
	// the security relevant action worth tracing.
	s.emitAuditEvent(ctx, &apievents.DeviceEvent2{
		Metadata: apievents.Metadata{
			Type: events.DeviceEnrollPairingDenyEvent,
			Code: events.DeviceEnrollPairingDenyCode,
		},
		Status: apievents.Status{
			Success: false,
		},
		Device:       getEnrollPairingDeviceMetadata(pairing.GetStatus().GetDevice()),
		UserMetadata: getUserMetadata(ctx),
	})
	return &devicepb.DenyEnrollPairingResponse{}, nil
}

// enrollPairingMatchingToken returns the user's current pairing provided its
// token matches tokenFromReq. On a token mismatch it returns the fetched
// pairing alongside the error, so callers can audit the device whose
// enrollment was pending.
func (s *Service) enrollPairingMatchingToken(ctx context.Context, user, tokenFromReq string) (*devicepb.EnrollPairing, error) {
	if tokenFromReq == "" {
		return nil, trace.BadParameter("pairing token required")
	}

	pairing, err := s.enrollPairing.GetCurrentEnrollPairing(ctx, user)
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("user %q has no enroll pairing", user)
		}
		return nil, trace.Wrap(err)
	}
	tokenFromPairing := pairing.GetStatus().GetToken()

	if subtle.ConstantTimeCompare([]byte(tokenFromPairing), []byte(tokenFromReq)) != 1 {
		// This func must return NotFound on mismatch or user not having a pairing
		// because the Web UI depends on 404 from ApproveEnrollPairing and DenyEnrollPairing.
		return pairing, trace.NotFound("pairing token does not match the user's current enroll pairing; use the token from GetCurrentEnrollPairing")
	}
	return pairing, nil
}

// emitEnrollPairingApproveEvent emits the approve audit event, success or
// failure. device is nil when the pairing could not be read.
func (s *Service) emitEnrollPairingApproveEvent(ctx context.Context, device *devicepb.EnrollPairingDevice, err error) {
	evt := &apievents.DeviceEvent2{
		Metadata: apievents.Metadata{
			Type: events.DeviceEnrollPairingApproveEvent,
			Code: events.DeviceEnrollPairingApproveCode,
		},
		Status: apievents.Status{
			Success: true,
		},
		Device:       getEnrollPairingDeviceMetadata(device),
		UserMetadata: getUserMetadata(ctx),
	}
	if err != nil {
		evt.Metadata.Code = events.DeviceEnrollPairingApproveFailureCode
		evt.Status.Success = false
		evt.Status.Error = err.Error()
	}
	s.emitAuditEvent(ctx, evt)
}

func getEnrollPairingDeviceMetadata(device *devicepb.EnrollPairingDevice) *apievents.DeviceMetadata {
	if device == nil {
		return nil
	}
	return &apievents.DeviceMetadata{
		OsType:   apievents.OSType(device.GetOsType()),
		AssetTag: device.GetSerialNumber(),
	}
}
