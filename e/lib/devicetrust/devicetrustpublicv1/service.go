// Package devicetrustpublicv1 implements the auth-side handler for
// teleport.devicetrust.public.v1.DeviceTrustService.
//
// The public Device Trust service is the unauthenticated counterpart to
// teleport.devicetrust.v1.DeviceTrustService. It exists so that mobile devices
// can carry out enrollment and device auth without first having to obtain a
// user cert through a full login procedure.
//
// See RFD 32e for more details.
package devicetrustpublicv1

import (
	"context"
	"errors"
	"log/slog"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/defaults"
	devicetrustpublicv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/public/v1"
	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/e/lib/devicetrust/storage"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tlsca"
)

// errAwaitingApproval is an error returned by CreatePairedDeviceEnrollToken
// when the enroll pairing is not yet approved. It's a temporary solution used
// only until we implement actual watcher that makes the RPC block until the
// enroll pairing is approved.
var errAwaitingApproval = &trace.CompareFailedError{Message: "enroll pairing is awaiting approval"}
var errPairingClaimed = &trace.AccessDeniedError{Message: "enroll pairing claimed by another device"}

// Service implements the
// teleport.devicetrust.public.v1.DeviceTrustService RPC service.
type Service struct {
	devicetrustpublicv1pb.UnimplementedDeviceTrustServiceServer

	logger        *slog.Logger
	enrollPairing services.EnrollPairing
	storage       *storage.S
	authorizer    authz.Authorizer
	cachedUsers   UsersService
	emitter       apievents.Emitter
	modules       modules.Modules
}

// UsersService represents the IdentityService methods used by [Service].
type UsersService interface {
	GetUser(ctx context.Context, user string, withSecrets bool) (types.User, error)
}

// ServiceParams holds creation parameters for [Service].
type ServiceParams struct {
	Logger        *slog.Logger
	EnrollPairing services.EnrollPairing
	Storage       *storage.S
	Authorizer    authz.Authorizer
	CachedUsers   UsersService
	Emitter       apievents.Emitter
	Modules       modules.Modules
}

// New creates a new public Device Trust [Service].
func New(params ServiceParams) (*Service, error) {
	switch {
	case params.Storage == nil:
		return nil, trace.BadParameter("parameter Storage required")
	case params.EnrollPairing == nil:
		return nil, trace.BadParameter("parameter EnrollPairing required")
	case params.Authorizer == nil:
		return nil, trace.BadParameter("parameter Authorizer required")
	case params.CachedUsers == nil:
		return nil, trace.BadParameter("parameter CachedUsers required")
	case params.Emitter == nil:
		return nil, trace.BadParameter("parameter Emitter required")
	case params.Modules == nil:
		return nil, trace.BadParameter("parameter Modules required")
	}

	logger := params.Logger
	if logger == nil {
		logger = slog.Default()
	}

	return &Service{
		logger:        logger.With(teleport.ComponentKey, "devicetrust.public.service"),
		storage:       params.Storage,
		authorizer:    params.Authorizer,
		cachedUsers:   params.CachedUsers,
		emitter:       params.Emitter,
		enrollPairing: params.EnrollPairing,
		modules:       params.Modules,
	}, nil
}

// CreatePairedDeviceEnrollToken claims an enroll pairing on behalf of the
// device identified by the pairing token, transitioning it to "awaiting
// approval" state.
//
// The work on enroll pairing approval is deferred, so a successful claim
// currently returns CompareFailed – the client can keep polling the RPC until
// the paring is approved.
func (s *Service) CreatePairedDeviceEnrollToken(
	ctx context.Context,
	req *devicetrustpublicv1pb.CreatePairedDeviceEnrollTokenRequest,
) (_ *devicetrustpublicv1pb.CreatePairedDeviceEnrollTokenResponse, err error) {
	if err := s.authorizeProxy(ctx); err != nil {
		return nil, trace.Wrap(err)
	}

	token := req.GetEnrollPairingToken()
	if token == "" {
		return nil, trace.BadParameter("enroll_pairing_token required")
	}
	cd := req.GetDeviceData()
	if err := storage.ValidateCollectedData(cd); err != nil {
		return nil, trace.Wrap(err)
	}
	deviceMd := deviceMetadataFromCollectedData(cd)

	var user string
	// TODO(ravicious): Get rid of this var and the errAwaitingApproval handling
	// below once this RPC no longer returns errAwaitingApproval on success.
	var requestedApprovalWithinThisCall bool
	defer func() {
		// errAwaitingApproval stands in for a successful claim, so it must not
		// be audited as a failure. The requestedApprovalWithinThisCall check is
		// there so that a device that merely polls its own pending request gets no
		// event at all, otherwise we'd emit one on each retry.
		auditErr := err
		if errors.Is(err, errAwaitingApproval) {
			if !requestedApprovalWithinThisCall {
				return
			}
			auditErr = nil
		}
		s.emitRequestEvent(ctx, user, deviceMd, auditErr)
	}()

	// The pairing token is what identifies the user, so a failed lookup leaves us
	// with no user to attribute the failure to.
	pairing, err := s.enrollPairing.GetEnrollPairingByToken(ctx, token)
	if err != nil {
		return nil, trace.Wrap(err, "read enroll pairing token")
	}
	user = pairing.GetMetadata().GetName()

	// Reconstruct the pairing user's identity in the context and run the
	// centralized authz, so create_enroll_token is evaluated through the same
	// entry point the authenticated service uses.
	if err := s.authorizeUser(ctx, user); err != nil {
		return nil, trace.Wrap(err)
	}

	device := devicepb.EnrollPairingDevice_builder{
		OsType:       cd.GetOsType(),
		SerialNumber: cd.GetSerialNumber(),
		OsVersion:    cd.GetOsVersion(),
	}.Build()

	// Two attempts: the first can lose the CAS to a concurrent claim, and the
	// second dispatches on the state the winner left behind. The state machine
	// only moves forward, so there is nothing for a third attempt to see.
	for range 2 {
		// Only meaningful once a device has moved past AWAITING_DEVICE.
		claimed := pairing.GetStatus().GetDevice()
		sameDevice := claimed.GetOsType() == device.GetOsType() &&
			claimed.GetSerialNumber() == device.GetSerialNumber()

		switch state := pairing.GetStatus().GetState(); state {
		case devicepb.EnrollPairingState_ENROLL_PAIRING_STATE_AWAITING_DEVICE:
			_, err := s.enrollPairing.RequestEnrollPairingApproval(ctx, pairing, device)
			if err == nil {
				requestedApprovalWithinThisCall = true
				return nil, trace.Wrap(errAwaitingApproval)
			}
			if !trace.IsCompareFailed(err) {
				return nil, trace.Wrap(err)
			}

			// A concurrent request claimed the pairing between our read and the
			// CAS. Re-read so the next attempt dispatches on the winner's state.
			fresh, err := s.enrollPairing.GetEnrollPairingByToken(ctx, token)
			if err != nil {
				return nil, trace.Wrap(err, "re-read enroll pairing token")
			}
			pairing = fresh
			continue

		case devicepb.EnrollPairingState_ENROLL_PAIRING_STATE_AWAITING_APPROVAL:
			// Either the claiming device polling its own in-progress request or
			// a different one attempting a hijack.
			if !sameDevice {
				return nil, errPairingClaimed
			}
			return nil, trace.Wrap(errAwaitingApproval)

		case devicepb.EnrollPairingState_ENROLL_PAIRING_STATE_APPROVED:
			if !sameDevice {
				return nil, errPairingClaimed
			}
			// TODO(ravicious): Consume the pairing through a conditional delete and
			// return the enrollment token, as described in the Interruptibility
			// section of RFD 32e.
			return nil, trace.NotImplemented("retrieving an enrollment token for an approved enroll pairing")

		default:
			return nil, trace.Errorf("enroll pairing in unexpected state %v", state)
		}
	}

	return nil, trace.Errorf("enroll pairing claim did not settle, this is a bug")
}

// authorizeProxy checks that the request is coming from a Proxy Service.
//
// The endpoints in this service are unauthenticated from the end user's
// perspective: the mobile app holds no user certs. Requests reach the Auth
// Service via the Proxy Service, so the in-context identity is the Proxy
// builtin role.
func (s *Service) authorizeProxy(ctx context.Context) error {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	if !authz.HasBuiltinRole(*authCtx, string(types.RoleProxy)) {
		return trace.AccessDenied("this request can only be executed by a proxy")
	}

	if !s.modules.Features().GetEntitlement(entitlements.DeviceTrust).Enabled {
		return trace.AccessDenied("this Teleport cluster is not licensed for device trust, please contact the cluster administrator")
	}

	return nil
}

// authorizeUser reconstructs the pairing user's identity in the context and
// runs the centralized authz to check the create_enroll_token permission.
func (s *Service) authorizeUser(ctx context.Context, user string) error {
	u, err := s.cachedUsers.GetUser(ctx, user, false)
	if err != nil {
		if trace.IsNotFound(err) {
			// The pairing outlived its user record, likely an SSO user whose
			// ephemeral record expired within the pairing TTL.
			return trace.NotFound("user not found")
		}
		return trace.Wrap(err, "get user")
	}

	reconstructedCtx := authz.ContextWithUser(ctx, authz.LocalUser{
		Username: user,
		Identity: tlsca.Identity{
			Username: user,
			Groups:   u.GetRoles(),
			Traits:   u.GetTraits(),
		},
	})

	authCtx, err := s.authorizer.Authorize(reconstructedCtx)
	if err != nil {
		return trace.Wrap(err, "authorize from reconstructed context")
	}

	return trace.Wrap(authCtx.Checker.CheckAccessToRule(
		&services.Context{User: authCtx.User},
		defaults.Namespace, types.KindMobileDevice, types.VerbCreateEnrollToken,
	), "check rule access")
}

func deviceMetadataFromCollectedData(cd *devicepb.DeviceCollectedData) *apievents.DeviceMetadata {
	return &apievents.DeviceMetadata{
		OsType:   apievents.OSType(cd.GetOsType()),
		AssetTag: cd.GetSerialNumber(),
	}
}

// emitRequestEvent emits the device enroll pairing request audit event,
// success or failure. user is empty when the pairing lookup failed before a
// user could be resolved.
func (s *Service) emitRequestEvent(ctx context.Context, user string, device *apievents.DeviceMetadata, err error) {
	evt := &apievents.DeviceEvent2{
		Metadata: apievents.Metadata{
			Type: events.DeviceEnrollPairingRequestEvent,
			Code: events.DeviceEnrollPairingRequestCode,
		},
		Device: device,
		Status: apievents.Status{
			Success: true,
		},
	}
	if user != "" {
		evt.UserMetadata = apievents.UserMetadata{User: user}
	}
	if err != nil {
		evt.Metadata.Code = events.DeviceEnrollPairingRequestFailureCode
		evt.Status.Success = false
		evt.Status.Error = err.Error()
	}
	if emitErr := s.emitter.EmitAuditEvent(ctx, evt); emitErr != nil {
		s.logger.WarnContext(ctx, "Failed to emit audit event",
			"error", emitErr,
			"type", evt.GetType(),
			"code", evt.GetCode(),
		)
	}
}
