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
	"slices"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/defaults"
	devicetrustpublicv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/public/v1"
	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/e/lib/devicetrust/devicetrustv1"
	dterrors "github.com/gravitational/teleport/e/lib/devicetrust/errors"
	"github.com/gravitational/teleport/e/lib/devicetrust/storage"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/authz"
	dtoss "github.com/gravitational/teleport/lib/devicetrust"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/tlsca"
)

// enrollPairingPollInterval is how often [Service.awaitApproval] re-reads the
// pairing it waits on.
const enrollPairingPollInterval = time.Second

// enrollPairingApprovalTimeout bounds the whole approval wait, so that a caller
// that never cancels cannot block a handler forever. Nothing can legitimately
// approve a pairing past its TTL. The extra margin absorbs the last poll
// interval, a slow backend read and clock skew between the Auth Service that
// stamped the expiry and the one serving the wait. A poll that still sees the
// pairing this late is looking at storage that missed the expiry.
const enrollPairingApprovalTimeout = local.EnrollPairingExpireDuration + 30*time.Second

var errPairingClaimed = &trace.AccessDeniedError{Message: "enroll pairing claimed by another device"}

// errInvalidPairingToken is returned for a token that resolves to no pairing,
// without relaying the underlying NotFound, so that storage state doesn't leak
// to the unauthenticated caller.
//
// It is an AccessDeniedError rather than a NotFoundError because the pairing
// token is the caller's bearer credential, not a resource identifier.
var errInvalidPairingToken = &trace.AccessDeniedError{Message: "invalid enroll pairing token"}

// errPairingLookupUnavailable stands in for a pairing lookup that failed for a
// reason other than a missing pairing, so that backend state doesn't reach the
// unauthenticated caller. The real error survives in a Debug log.
//
// It is a ConnectionProblemError, and thus retryable, so that a transient
// storage failure doesn't read as a terminal rejection of a token that is
// still good.
var errPairingLookupUnavailable = &trace.ConnectionProblemError{Message: "enroll pairing lookup failed"}

// errPairingDeniedOrExpired is returned for a pairing that is gone from storage
// while a device waits on it, which is either a denial or a TTL expiry.
var errPairingDeniedOrExpired = &trace.AccessDeniedError{Message: "enroll pairing was denied or has expired"}

// errPairingConsumed goes to a waiter that lost the delete consuming the
// pairing. Several handlers may wait on the same approval, but only the winner
// issues a token, so this is a normal concurrency outcome rather than a fault.
// CompareFailedError gets translated to FailedPrecondition, so that the caller
// knows it cannot just repeat the request
var errPairingConsumed = &trace.CompareFailedError{Message: "enroll pairing was already consumed"}

// errEnrollVerificationFailed erases the error behind a failed enrollment token
// creation, so that neither the device inventory state nor the collected data
// drift details reach the unauthenticated caller.
var errEnrollVerificationFailed = &trace.BadParameterError{Message: "device enrollment verifications failed"}

// errEnrollTokenIssuanceFailed erases backend failures observed while consuming
// the pairing for token issuance. It is a ConnectionProblemError because a
// retry is always the right next step: it reattaches to the pairing if the
// failed delete left it in place, and settles on a terminal error otherwise.
var errEnrollTokenIssuanceFailed = &trace.ConnectionProblemError{Message: "enroll token issuance failed"}

// errUserAuthzUnavailable stands in for a user authorization that failed for a
// reason other than a missing user or a denial, so that backend state doesn't
// reach the unauthenticated caller. The real error survives in a Debug log.
//
// It is a ConnectionProblemError, and thus retryable, so that a transient
// storage failure doesn't read as a terminal rejection of a token that is still
// good.
var errUserAuthzUnavailable = &trace.ConnectionProblemError{Message: "internal error while authorizing user"}

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
// device identified by the pairing token, then blocks until the owning user
// approves the request in the Web UI and returns a device enrollment token.
//
// A denial and a TTL expiration are indistinguishable to the caller: both
// delete the pairing and surface as AccessDenied.
//
// The call is retryable from the same device, so that a dropped connection
// doesn't strand the pairing without a waiter. See the "Interruptibility"
// section of RFD 32e.
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

	pairing, err := s.requestEnrollment(ctx, token, cd)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	pairing, err = s.awaitApproval(ctx, pairing)
	if err != nil {
		// No audit event here. The request was audited at claim time. A denial is
		// audited by DenyEnrollPairing, and a TTL expiry gets no event.
		return nil, trace.Wrap(err)
	}

	enrollToken, err := s.issueEnrollToken(ctx, pairing, cd)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return devicetrustpublicv1pb.CreatePairedDeviceEnrollTokenResponse_builder{
		DeviceEnrollToken: enrollToken,
	}.Build(), nil
}

// requestEnrollment resolves the pairing behind token, authorizes its user and
// claims it for the device described by cd, emitting the audit event for every
// outcome except a retry for an already-auditted claim.
//
// The event is emitted at claim time rather than when the RPC finishes so that
// it carries the request's own timestamp and doesn't ride a context that the
// approval wait may see canceled.
func (s *Service) requestEnrollment(ctx context.Context, token string, cd *devicepb.DeviceCollectedData) (_ *devicepb.EnrollPairing, err error) {
	deviceMd := deviceMetadataFromCollectedData(cd)
	var user string
	// A retry reattaches to a device enrollment request (EnrollPairing) that was
	// already audited when it was first made, so only a fresh claim is recorded.
	var isRetry bool
	defer func() {
		if isRetry {
			return
		}
		s.emitRequestEvent(ctx, user, deviceMd, err)
	}()

	// The pairing token is what identifies the user, so a failed lookup leaves us
	// with no user to attribute the failure to.
	pairing, err := s.enrollPairing.GetEnrollPairingByToken(ctx, token)
	if err != nil {
		s.logger.DebugContext(ctx,
			"CreatePairedDeviceEnrollToken: enroll pairing lookup failed",
			"error", err,
		)
		return nil, redactPairingLookupError(ctx, err, errInvalidPairingToken)
	}
	user = pairing.GetMetadata().GetName()

	// Reconstruct the pairing user's identity in the context and run the
	// centralized authz, so create_enroll_token is evaluated through the same
	// entry point the authenticated service uses.
	if err := s.authorizeUser(ctx, user, types.KindMobileDevice, types.VerbCreateEnrollToken); err != nil {
		s.logger.DebugContext(ctx,
			"CreatePairedDeviceEnrollToken: user authorization failed",
			"error", err,
		)
		return nil, redactUserAuthzError(ctx, err)
	}

	device := devicepb.EnrollPairingDevice_builder{
		OsType:       cd.GetOsType(),
		SerialNumber: cd.GetSerialNumber(),
		OsVersion:    cd.GetOsVersion(),
	}.Build()

	pairing, claimed, err := s.claimPairing(ctx, pairing, device)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	isRetry = !claimed
	return pairing, nil
}

// claimPairing transitions pairing to AWAITING_APPROVAL on behalf of device,
// or, for a pairing that has already been claimed, checks that the claim
// belongs to the same device.
//
// claimed reports whether this call is what moved the pairing out of
// AWAITING_DEVICE, as opposed to reattaching to an existing claim.
func (s *Service) claimPairing(ctx context.Context, pairing *devicepb.EnrollPairing, device *devicepb.EnrollPairingDevice) (_ *devicepb.EnrollPairing, claimed bool, err error) {
	// Two attempts: the first can lose the CAS to a concurrent claim, and the
	// second dispatches on the state the winner left behind. The state machine
	// only moves forward, so there is nothing for a third attempt to see.
	for range 2 {
		// Only meaningful once a device has moved past AWAITING_DEVICE.
		claimedDevice := pairing.GetStatus().GetDevice()
		sameDevice := claimedDevice.GetOsType() == device.GetOsType() &&
			claimedDevice.GetSerialNumber() == device.GetSerialNumber()

		switch state := pairing.GetStatus().GetState(); state {
		case devicepb.EnrollPairingState_ENROLL_PAIRING_STATE_AWAITING_DEVICE:
			updated, err := s.enrollPairing.RequestEnrollPairingApproval(ctx, pairing, device)
			if err == nil {
				return updated, true, nil
			}
			if !trace.IsCompareFailed(err) {
				s.logger.DebugContext(ctx,
					"CreatePairedDeviceEnrollToken: enroll pairing claim failed",
					"error", err,
				)
				return nil, false, redactPairingLookupError(ctx, err, errInvalidPairingToken)
			}

			// err is CompareFailed. A concurrent request claimed the pairing between
			// our read and the CAS. Re-read so the next attempt dispatches on the
			// winner's state.
			fresh, err := s.enrollPairing.GetEnrollPairingByToken(ctx, pairing.GetStatus().GetToken())
			if err != nil {
				s.logger.DebugContext(ctx,
					"CreatePairedDeviceEnrollToken: enroll pairing re-read failed",
					"error", err,
				)
				return nil, false, redactPairingLookupError(ctx, err, errInvalidPairingToken)
			}
			pairing = fresh
			continue

		case devicepb.EnrollPairingState_ENROLL_PAIRING_STATE_AWAITING_APPROVAL,
			devicepb.EnrollPairingState_ENROLL_PAIRING_STATE_APPROVED:
			// Either the claiming device reattaching to its own in-progress
			// enrollment request or a different one attempting a hijack.
			// An APPROVED pairing needs no branch of its own: it's what the polling
			// loop exits on, so the wait returns right away.
			if !sameDevice {
				return nil, false, errPairingClaimed
			}
			return pairing, false, nil

		default:
			return nil, false, trace.Errorf("enroll pairing in unexpected state %v", state)
		}
	}

	return nil, false, trace.Errorf("enroll pairing claim did not settle, this is a bug")
}

// redactPairingLookupError maps a pairing lookup or claim failure onto an error
// fit for the unauthenticated caller: notFound for a pairing that isn't there,
// whose meaning differs per call site, and errPairingLookupUnavailable for anything
// else. Callers log the real error at Debug.
func redactPairingLookupError(ctx context.Context, err error, notFound error) error {
	switch {
	case trace.IsNotFound(err):
		return trace.Wrap(notFound)
	case ctx.Err() != nil:
		// Cancellation is the caller's own doing, so it carries no storage state
		// and is worth telling apart from a server-side failure.
		return trace.Wrap(ctx.Err())
	default:
		// err swallowed on purpose.
		return trace.Wrap(errPairingLookupUnavailable)
	}
}

// redactUserAuthzError maps a user authorization failure onto an error fit for
// the unauthenticated caller: the deliberate outcomes, a missing user and a
// denial, pass through, and everything else is a server-side failure redacted
// as errUserAuthzUnavailable. Callers log the real error.
func redactUserAuthzError(ctx context.Context, err error) error {
	switch {
	case trace.IsNotFound(err), trace.IsAccessDenied(err):
		return trace.Wrap(err)
	case ctx.Err() != nil:
		// Cancellation is the caller's own doing, so it carries no storage state
		// and is worth telling apart from a server-side failure.
		return trace.Wrap(ctx.Err())
	default:
		return trace.Wrap(errUserAuthzUnavailable)
	}
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

// authorizeUser reconstructs the identity of user, the one the caller's token
// is bound to, and runs the centralized authz on it before checking access to
// verb on kind.
func (s *Service) authorizeUser(ctx context.Context, user, kind, verb string) error {
	authCtx, err := s.authorizeUserIdentity(ctx, user)
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(authCtx.Checker.CheckAccessToRule(
		&services.Context{User: authCtx.User},
		defaults.Namespace, kind, verb,
	), "check rule access")
}

// authorizeUserIdentity reconstructs the identity of user, the one the caller's
// token is bound to, and runs the centralized authz on it. Rule checks are left
// to the caller as some RPCs, like AuthenticateDevice, don't need to run them.
func (s *Service) authorizeUserIdentity(ctx context.Context, user string) (*authz.Context, error) {
	u, err := s.cachedUsers.GetUser(ctx, user, false)
	if err != nil {
		if trace.IsNotFound(err) {
			// The pairing outlived its user record, likely an SSO user whose
			// ephemeral record expired within the pairing TTL.
			return nil, trace.NotFound("user not found")
		}
		return nil, trace.Wrap(err, "get user")
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
		return nil, trace.Wrap(err, "authorize from reconstructed context")
	}
	return authCtx, nil
}

func deviceMetadataFromCollectedData(cd *devicepb.DeviceCollectedData) *apievents.DeviceMetadata {
	return &apievents.DeviceMetadata{
		OsType:   apievents.OSType(cd.GetOsType()),
		AssetTag: cd.GetSerialNumber(),
	}
}

func deviceMetadataFromDevice(d *devicepb.Device) *apievents.DeviceMetadata {
	return &apievents.DeviceMetadata{
		OsType:       apievents.OSType(d.GetOsType()),
		AssetTag:     d.GetAssetTag(),
		DeviceId:     d.GetId(),
		CredentialId: d.GetCredential().GetId(),
		DeviceOrigin: apievents.DeviceOrigin(d.GetSource().GetOrigin()),
	}
}

// deviceMetadata describes the device for an audit event: the inventory record
// when the handler has resolved one, the caller's collected data otherwise.
func deviceMetadata(dev *devicepb.Device, cd *devicepb.DeviceCollectedData) *apievents.DeviceMetadata {
	if dev != nil {
		return deviceMetadataFromDevice(dev)
	}
	return deviceMetadataFromCollectedData(cd)
}

// awaitApproval blocks until the owning user approves the pairing in the Web
// UI. A pairing that is already approved on entry returns right away, covering
// the case where the user approved while no CreatePairedDeviceEnrollToken
// handler was active. The wait is bounded by [enrollPairingApprovalTimeout]
// even for a ctx that never ends.
func (s *Service) awaitApproval(ctx context.Context, pairing *devicepb.EnrollPairing) (*devicepb.EnrollPairing, error) {
	timeout := time.After(enrollPairingApprovalTimeout)

	token := pairing.GetStatus().GetToken()
	for pairing.GetStatus().GetState() != devicepb.EnrollPairingState_ENROLL_PAIRING_STATE_APPROVED {
		select {
		case <-ctx.Done():
			return nil, trace.Wrap(ctx.Err())
		case <-timeout:
			// The pairing is past its TTL, so the outcome is settled for the caller.
			// This can typically only ever occur if the storage is faulty.
			s.logger.DebugContext(ctx,
				"CreatePairedDeviceEnrollToken: approval wait timed out past the pairing TTL",
			)
			return nil, errPairingDeniedOrExpired
		case <-time.After(enrollPairingPollInterval):
		}
		var err error
		pairing, err = s.enrollPairing.GetEnrollPairingByToken(ctx, token)
		if err != nil {
			s.logger.DebugContext(ctx,
				"CreatePairedDeviceEnrollToken: enroll pairing poll failed",
				"error", err,
			)
			return nil, redactPairingLookupError(ctx, err, errPairingDeniedOrExpired)
		}
	}
	return pairing, nil
}

// issueEnrollToken consumes the approved pairing and creates the enrollment
// token. Deleting the pairing first makes it single-use: when several handlers
// wait on the same approval, only the one that wins the conditional delete
// issues a token.
func (s *Service) issueEnrollToken(ctx context.Context, pairing *devicepb.EnrollPairing,
	cd *devicepb.DeviceCollectedData) (*devicepb.DeviceEnrollToken, error) {
	user := pairing.GetMetadata().GetName()

	if err := s.enrollPairing.DeleteEnrollPairing(ctx, pairing); err != nil {
		// The errors below do not need to be audited through something like
		// auditStatusError. CompareFailed is returned in a very specific scenario
		// and its errPairingConsumed is included in the audit event. Other
		// errors are likely transient and do not need to be in the audit.
		if trace.IsCompareFailed(err) {
			return nil, errPairingConsumed
		}
		s.logger.DebugContext(ctx,
			"CreatePairedDeviceEnrollToken: enroll pairing delete failed",
			"error", err,
			"user", user,
			"asset_tag", cd.GetSerialNumber(),
		)
		return nil, errEnrollTokenIssuanceFailed
	}

	// The device owner rides with the token, as the EnrollDevice ceremony has no
	// authenticated caller to derive it from.
	dev, err := s.storage.CreateDeviceEnrollTokenUsingData(ctx, cd, user)
	deviceMd := deviceMetadataFromCollectedData(cd)
	if dev != nil {
		deviceMd.DeviceId = dev.GetId()
	}
	s.emitEnrollTokenCreateEvent(ctx, user, deviceMd, err)
	if err != nil {
		// err swallowed on purpose.
		return nil, errEnrollVerificationFailed
	}
	return dev.GetEnrollToken(), nil
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
	s.emitAuditEvent(ctx, evt)
}

// emitEnrollTokenCreateEvent emits the device enroll token creation audit
// event, success or failure, so that an auditor filtering on the event sees
// mobile-issued tokens alongside the ones the private service issues.
//
// The event has no failure code of its own, so a failed issuance carries the
// same code with the success flag unset, as in the private service.
func (s *Service) emitEnrollTokenCreateEvent(ctx context.Context, user string, device *apievents.DeviceMetadata, err error) {
	evt := &apievents.DeviceEvent2{
		Metadata: apievents.Metadata{
			Type: events.DeviceEnrollTokenCreateEvent,
			Code: events.DeviceEnrollTokenCreateCode,
		},
		Device: device,
		Status: apievents.Status{
			Success: true,
		},
		UserMetadata: apievents.UserMetadata{User: user},
	}
	if err != nil {
		// The device gets a redacted error, the audit trail keeps the real one.
		evt.Status.Success = false
		evt.Status.UserMessage = err.Error()
	}
	s.emitAuditEvent(ctx, evt)
}

// emitAuditEvent emits evt, reporting a failure to emit through the log rather
// than through the RPC.
func (s *Service) emitAuditEvent(ctx context.Context, evt apievents.AuditEvent) {
	if err := s.emitter.EmitAuditEvent(ctx, evt); err != nil {
		s.logger.WarnContext(ctx, "Failed to emit audit event",
			"error", err,
			"type", evt.GetType(),
			"code", evt.GetCode(),
		)
	}
}

// errEnrollTokenLookupUnavailable stands in for a token resolution that failed
// for a reason other than a missing device or token, so that backend state
// doesn't reach the unauthenticated caller. The real error survives in the
// audit event and a Debug log.
//
// It is a ConnectionProblemError, and thus retryable, so that a transient
// storage failure doesn't read as a terminal rejection of a token that is still
// good.
var errEnrollTokenLookupUnavailable = &trace.ConnectionProblemError{Message: "enrollment token lookup failed"}

// errEnrollDeviceUnavailable stands in for an enrollment ceremony error that is
// not written for the caller and happened while the token was still stored, so
// that backend and inventory state doesn't reach the unauthenticated caller.
// The real error survives in the audit event and a Debug log.
//
// It is a ConnectionProblemError, and thus retryable, so that a transient
// backend failure doesn't read as a terminal rejection of a token that is still
// good. See [errEnrollDeviceFailed] for failures past the point where the token
// is spent.
var errEnrollDeviceUnavailable = &trace.ConnectionProblemError{Message: "device enrollment failed"}

// errEnrollDeviceFailed stands in for an enrollment ceremony error that is not
// written for the caller and happened after the token was spent.
//
// A retry with the same token will fail, so unlike [errEnrollDeviceUnavailable]
// this error must not read as retryable: the caller has to start a new pairing
// flow. CompareFailedError gets translated to FailedPrecondition, so that the
// caller knows it cannot just repeat the request.
var errEnrollDeviceFailed = &trace.CompareFailedError{Message: "device enrollment failed, request a new enrollment token"}

// errEnrollDeviceTimeout ends a stream that outlived [dtoss.PublicEnrollDeviceTimeout].
// It is a LimitExceededError to signal that the caller ran into a limit imposed
// by the service.
var errEnrollDeviceTimeout = &trace.LimitExceededError{Message: "device enrollment timed out"}

// mobileOSTypes gates the public Device Trust RPCs to mobile devices. Desktop
// devices enroll and authenticate through the private Device Trust service.
var mobileOSTypes = []devicepb.OSType{
	devicepb.OSType_OS_TYPE_IOS,
	devicepb.OSType_OS_TYPE_IPADOS,
}

// EnrollDevice implements the device enrollment ceremony over the public Device
// Trust service.
//
// The caller is identified by the enrollment token sent in the init message:
// the handler resolves the token's user, reruns authorization on their behalf
// and only then lets the ceremony from the private service spend the token.
func (s *Service) EnrollDevice(stream devicetrustpublicv1pb.DeviceTrustService_EnrollDeviceServer) error {
	return runWithStreamTimeout(stream, dtoss.PublicEnrollDeviceTimeout, errEnrollDeviceTimeout, s.enrollDevice)
}

func (s *Service) enrollDevice(stream devicetrustpublicv1pb.DeviceTrustService_EnrollDeviceServer) error {
	ctx := stream.Context()
	if err := s.authorizeProxy(ctx); err != nil {
		return trace.Wrap(err)
	}

	req, err := stream.Recv()
	if err != nil {
		return trace.Wrap(err)
	}
	init := req.GetInit()
	if err := validateEnrollDeviceInit(init); err != nil {
		return trace.Wrap(err)
	}

	dev, user, err := s.resolveEnrollToken(ctx, init)
	if err != nil {
		s.logger.DebugContext(ctx,
			"EnrollDevice: enroll token resolution failed",
			"error", err,
		)
		s.emitEnrollEvent(ctx, user, deviceMetadata(dev, init.GetDeviceData()), err)
		return trace.Wrap(redactEnrollTokenError(ctx, err))
	}
	// mobile_device.create_enroll_token gates the ceremony as well as the token:
	// the private service's device.enroll rule and its auto-enroll exemption
	// have no mobile counterpart, and the user established their right to
	// enroll this device when the token was minted for them. Rechecking the
	// same permission here catches a lock or a revoked role between the two
	// calls, before the token is spent.
	if err := s.authorizeUser(ctx, user, types.KindMobileDevice, types.VerbCreateEnrollToken); err != nil {
		s.logger.DebugContext(ctx,
			"EnrollDevice: token user authorization failed",
			"error", err,
		)
		s.emitEnrollEvent(ctx, user, deviceMetadata(dev, init.GetDeviceData()), err)
		return trace.Wrap(redactUserAuthzError(ctx, err))
	}

	dev, tokenSpent, err := devicetrustv1.RunEnrollDeviceCeremony(
		&enrollDeviceStreamAdapter{DeviceTrustService_EnrollDeviceServer: stream, init: init},
		devicetrustv1.EnrollDeviceCeremonyParams{
			Logger:  s.logger,
			Storage: s.storage,
			AuditCallback: func(d *devicepb.Device, err error) {
				s.emitEnrollEvent(ctx, user, deviceMetadata(d, init.GetDeviceData()), err)
			},
			AllowedOSTypes: mobileOSTypes,
			User:           user,
		})
	if err != nil {
		s.logCeremonyError(ctx, "EnrollDevice: enrollment ceremony failed", dev, err)
	}

	return trace.Wrap(redactEnrollError(ctx, err, tokenSpent))
}

// logCeremonyError logs a failed ceremony, at Debug under msg. Collected data
// drift is logged at Warn with the device it was detected on, as the private
// service does, so that a mobile device failing drift checks is visible in the
// Auth Service log at the default level like a desktop one. The audit event
// carries the same error.
func (s *Service) logCeremonyError(ctx context.Context, msg string, dev *devicepb.Device, err error) {
	if errors.Is(err, &storage.CollectedDataDriftError{}) {
		s.logger.WarnContext(ctx,
			"Collected data drift detected",
			"error", err,
			"device_id", dev.GetId(),
			"asset_tag", dev.GetAssetTag(),
		)
		return
	}
	//nolint:sloglint // message cannot be constant
	s.logger.DebugContext(ctx, msg, "error", err)
}

// validateEnrollDeviceInit checks just enough of the init message for the
// handler to resolve the token user. The ceremony re-runs the equivalent checks
// on the adapted message.
//
// The OS type gate runs here, before any storage access, so that a token
// stolen from a desktop device cannot be probed for validity through this
// RPC: a non-mobile init fails the same way whether or not the token is good.
func validateEnrollDeviceInit(init *devicetrustpublicv1pb.EnrollDeviceInit) error {
	switch {
	case init == nil:
		return trace.BadParameter("bad payload, expected EnrollDeviceInit")
	case init.GetToken() == "":
		return trace.BadParameter("enrollment token required")
	case !init.HasDeviceData():
		return trace.BadParameter("device data required")
	case init.GetDeviceData().GetOsType() == devicepb.OSType_OS_TYPE_UNSPECIFIED:
		return trace.BadParameter("device OS type required")
	case !slices.Contains(mobileOSTypes, init.GetDeviceData().GetOsType()):
		return trace.BadParameter("unsupported OS type: %v",
			dtoss.FriendlyOSType(init.GetDeviceData().GetOsType()))
	case init.GetDeviceData().GetSerialNumber() == "":
		return trace.BadParameter("device serial number required")
	}
	return nil
}

// resolveEnrollToken resolves the device and the user the enrollment token from
// init is bound to, without spending the token, so that a rejected attempt
// doesn't burn it.
//
// The enroll ceremony spends the token as soon as it runs, so the user must be
// resolved and authorized before it: a rejection at these stages, such as a
// locked user or revoked permissions, leaves the still-valid token stored for a
// later attempt instead of forcing a new pairing flow to mint another
// enrollment token.
//
// Tokens without a bound user, such as admin-issued tokens, are rejected: they
// have no user to authorize on the public path.
//
// The device is returned whenever the lookup resolved one, so that the audit
// event for a rejected attempt can name it.
// Errors are returned unredacted so that the caller can audit them. They must
// pass through [redactEnrollTokenError] before leaving the RPC.
func (s *Service) resolveEnrollToken(ctx context.Context, init *devicetrustpublicv1pb.EnrollDeviceInit) (*devicepb.Device, string, error) {
	dev, data, err := s.storage.GetDeviceEnrollTokenDataUsingData(ctx, init.GetDeviceData(), init.GetToken())
	if err != nil {
		return dev, "", trace.Wrap(err)
	}
	if data.User == "" {
		return dev, "", trace.AccessDenied("enrollment token has no user")
	}
	// Currently, tokens with non-empty User are always CreatedByAutoEnroll, but
	// let's make this explicit here.
	if !data.CreatedByAutoEnroll {
		return dev, "", trace.AccessDenied("enrollment token is not an auto-enroll token")
	}
	return dev, data.User, nil
}

func redactEnrollTokenError(ctx context.Context, err error) error {
	switch {
	case trace.IsNotFound(err), trace.IsBadParameter(err), trace.IsAccessDenied(err):
		return trace.Wrap(dterrors.ErrInvalidDeviceEnrollToken)
	case ctx.Err() != nil:
		return trace.Wrap(ctx.Err())
	default:
		// err swallowed on purpose.
		return trace.Wrap(errEnrollTokenLookupUnavailable)
	}
}

// redactEnrollError maps a ceremony error onto an error fit for the
// unauthenticated caller. Errors written for the caller pass through, anything
// else is erased. tokenSpent picks the erased error: retryable while the token
// is still stored, terminal once the ceremony has spent it.
//
// By the time redactEnrollError is called, the user is already authorized, so
// revealing whether a token is spent should not pose a security risk and allows
// for a better UX on the client side.
func redactEnrollError(ctx context.Context, err error, tokenSpent bool) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, dterrors.ErrInvalidDeviceEnrollToken):
		return trace.Wrap(err)
	case errors.Is(err, &storage.CollectedDataDriftError{}):
		// Drift is reported as the private service reports it: a constant
		// message for the caller, the drifted field only in the audit trail and
		// the Warn log.
		// err swallowed on purpose.
		return trace.AccessDenied("%s", devicetrustv1.DataDriftDetectedMessage)
	case trace.IsBadParameter(err):
		// The ceremony's BadParameter errors describe the caller's own payload.
		return trace.Wrap(err)
	case ctx.Err() != nil:
		// Cancellation is the caller's own doing.
		return trace.Wrap(ctx.Err())
	case tokenSpent:
		// err swallowed on purpose.
		return trace.Wrap(errEnrollDeviceFailed)
	default:
		// err swallowed on purpose.
		return trace.Wrap(errEnrollDeviceUnavailable)
	}
}

// emitEnrollEvent emits the device enrollment audit event, success or failure.
// user is empty when the failure precedes user resolution. The audit trail
// keeps the real error even when the RPC returns a redacted one.
func (s *Service) emitEnrollEvent(ctx context.Context, user string, device *apievents.DeviceMetadata, err error) {
	evt := &apievents.DeviceEvent2{
		Metadata: apievents.Metadata{
			Type: events.DeviceEnrollEvent,
			Code: events.DeviceEnrollCode,
		},
		Device: device,
		Status: apievents.Status{
			Success: true,
		},
	}
	if user != "" {
		// TODO(ravicious): Put full user metadata into audit events.
		// https://github.com/gravitational/teleport.e/issues/9351
		evt.UserMetadata = apievents.UserMetadata{User: user}
	}
	if err != nil {
		evt.Status.Success = false
		evt.Status.Error = err.Error()
		// No error reachable through the public service carries a custom
		// UserMessage today. Reading it anyway keeps the audit event in step with
		// the private service if a ceremony error gains one.
		evt.Status.UserMessage = devicetrustv1.UserMessage(err)
	} else {
		// The enrolled device is the trusted device in use. It's not in a user cert
		// at this stage, so it's assigned manually, as in the private service.
		evt.UserMetadata.TrustedDevice = device
	}
	s.emitAuditEvent(ctx, evt)
}
