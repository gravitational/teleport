package devicetrustv1

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"strconv"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"github.com/gravitational/trace/trail"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/gravitational/teleport"
	clientpb "github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/defaults"
	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	resourceusagepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/resourceusage/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/e/lib/devicetrust/storage"
	"github.com/gravitational/teleport/entitlements"
	prehogv1alpha "github.com/gravitational/teleport/gen/proto/go/prehog/v1alpha"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/authz"
	libdefaults "github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/devicetrust/assertserver"
	dtconfig "github.com/gravitational/teleport/lib/devicetrust/config"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/limiter"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/observability/metrics"
	"github.com/gravitational/teleport/lib/services"
	usagereporter "github.com/gravitational/teleport/lib/usagereporter/teleport"
)

// DataDriftDetectedMessage is the error message used to redact data drift
// errors.
const DataDriftDetectedMessage = "collected data drift detected"

// deviceTrustSubsystem is the metric subsystem for Device Trust.
const deviceTrustSubsystem = "devicetrust"

var (
	createEnrollTokenHist = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: teleport.MetricNamespace,
		Subsystem: deviceTrustSubsystem,
		Name:      "create_device_enroll_token_seconds",
		Help:      "CreateDeviceEnrollToken RPC histogram labeled by grpc_code",
		Buckets:   prometheus.DefBuckets,
	}, []string{"grpc_code"})

	enrollHist = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: teleport.MetricNamespace,
		Subsystem: deviceTrustSubsystem,
		Name:      "enroll_device_seconds",
		Help:      "EnrollDevice RPC histogram labeled by grpc_code",
		Buckets:   prometheus.DefBuckets,
	}, []string{"grpc_code"})

	authnHist = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: teleport.MetricNamespace,
		Subsystem: deviceTrustSubsystem,
		Name:      "authenticate_device_seconds",
		Help:      "AuthenticateDevice RPC histogram labeled by grpc_code and web_authentication",
		Buckets:   prometheus.DefBuckets,
	}, []string{"grpc_code", "web_authentication"})

	createDeviceWebTokenHist = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: teleport.MetricNamespace,
		Subsystem: deviceTrustSubsystem,
		Name:      "create_device_web_token_seconds",
		Help:      "CreateDeviceWebToken method histogram labeled by grpc_code",
		Buckets:   prometheus.DefBuckets,
	}, []string{"grpc_code"}) // Technically not an RPC, but grpc_code is a good way to record outcome.

	confirmDeviceWebAuthenticationHist = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: teleport.MetricNamespace,
		Subsystem: deviceTrustSubsystem,
		Name:      "confirm_device_web_authentication_seconds",
		Help:      "ConfirmDeviceWebAuthentication RPC histogram labeled by grpc_code",
		Buckets:   prometheus.DefBuckets,
	}, []string{"grpc_code"})

	syncOperationsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: teleport.MetricNamespace,
		Subsystem: deviceTrustSubsystem,
		Name:      "sync_inventory_device_operations_total",
		Help:      "SyncInventory device operations counter, labeled by operation",
	}, []string{"operation"})

	allMetrics = []prometheus.Collector{
		createEnrollTokenHist,
		enrollHist,
		authnHist,
		createDeviceWebTokenHist,
		confirmDeviceWebAuthenticationHist,
		syncOperationsTotal,
	}
)

var (
	errDeviceTrustDisabled = &trace.BadParameterError{
		Message: "device trust disabled by cluster settings",
	}
	errInvalidDeviceConfirmationToken = &trace.AccessDeniedError{
		Message: "invalid device confirmation token",
	}
)

// AuthServer represents the [auth.Server] methods used by [Service].
type AuthServer interface {
	// AugmentContextUserCertificates augments the context certificate and the supplied
	// certificates with device extensions.
	// All certificates must be valid, issued by the Teleport CA, match each
	// other, and conform to whatever checks the underlying implementation sees
	// fit to perform.
	// See [auth.Server.AugmentContextUserCertificates]
	AugmentContextUserCertificates(ctx context.Context, authCtx *authz.Context, opts *auth.AugmentUserCertificateOpts) (*clientpb.Certs, error)

	// AugmentWebSessionCertificates is a variant of
	// [AugmentContextUserCertificates] that works directly on the WebSession
	// certificates.
	AugmentWebSessionCertificates(ctx context.Context, opts *auth.AugmentWebSessionCertificatesOpts) error

	// GetAuthPreference gets the cluster's auth preferences.
	// This method is not guarded by user permissions.
	GetAuthPreference(ctx context.Context) (types.AuthPreference, error)

	// AnonymizeAndSubmit submits usage events to Prehog.
	AnonymizeAndSubmit(event ...usagereporter.Anonymizable)
}

// AccessService represents the [local.AccessService] methods used by [Service].
type AccessService interface {
	GetRole(ctx context.Context, name string) (types.Role, error)
}

// UsersService represents the [local.IdentityService] methods used by
// [Service].
type UsersService interface {
	GetUser(ctx context.Context, user string, withSecrets bool) (types.User, error)
}

// RateLimiter is a subset of [limiter.RateLimiter].
type RateLimiter interface {
	RegisterRequestWithCustomRate(token string, customRate *limiter.RateSet) error
}

// Service implements the teleport.devicetrust.v1.DeviceTrustService RPC
// service.
type Service struct {
	devicepb.UnimplementedDeviceTrustServiceServer

	logger *slog.Logger

	authServer  AuthServer
	authorizer  authz.Authorizer
	cachedRoles AccessService
	cachedUsers UsersService
	emitter     apievents.Emitter
	limiter     RateLimiter
	storage     *storage.S
}

// ServiceParams holds creation parameters for Service.
type ServiceParams struct {
	Logger              *slog.Logger
	AuthServer          AuthServer
	Authorizer          authz.Authorizer
	CachedAccessService AccessService
	CachedUsersService  UsersService
	Emitter             apievents.Emitter
	Storage             *storage.S

	// Limiter is the rate limiter for loosely-authorized requests, like
	// auto-enrollment token creation or device authentication.
	// Requests are typically rate-limited by user.
	// If `nil` a default limiter is used.
	Limiter RateLimiter
}

// New creates a new DeviceTrustService implementer.
func New(params ServiceParams) (*Service, error) {
	// Register service metrics. Expected to always work.
	if err := metrics.RegisterPrometheusCollectors(allMetrics...); err != nil {
		return nil, trace.Wrap(err)
	}

	switch {
	case params.AuthServer == nil:
		return nil, trace.BadParameter("parameter AuthServer required")
	case params.Authorizer == nil:
		return nil, trace.BadParameter("parameter Authorizer required")
	case params.CachedAccessService == nil:
		return nil, trace.BadParameter("parameter CachedAccessService required")
	case params.CachedUsersService == nil:
		return nil, trace.BadParameter("parameter CachedUsersService required")
	case params.Emitter == nil:
		return nil, trace.BadParameter("parameter Emitter required")
	case params.Storage == nil:
		return nil, trace.BadParameter("parameter Storage required")
	}

	baseLogger := params.Logger
	if baseLogger == nil {
		baseLogger = slog.Default()
	}

	rateLimiter := params.Limiter
	if rateLimiter == nil {
		var err error
		rateLimiter, err = limiter.NewLimiter(limiter.Config{
			MaxConnections: libdefaults.LimiterMaxConnections,
			Rates: []limiter.Rate{
				{
					Period:  libdefaults.LimiterPeriod,
					Average: libdefaults.LimiterAverage,
					Burst:   libdefaults.LimiterBurst,
				},
			},
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	return &Service{
		logger:      baseLogger.With(teleport.ComponentKey, "devicetrust.service"),
		authServer:  params.AuthServer,
		authorizer:  params.Authorizer,
		cachedRoles: params.CachedAccessService,
		cachedUsers: params.CachedUsersService,
		emitter:     params.Emitter,
		limiter:     rateLimiter,
		storage:     params.Storage,
	}, nil
}

func (s *Service) CreateDevice(ctx context.Context, req *devicepb.CreateDeviceRequest) (*devicepb.Device, error) {
	var verbs []string
	if req.CreateEnrollToken {
		verbs = []string{types.VerbCreate, types.VerbCreateEnrollToken}
	} else {
		verbs = []string{types.VerbCreate}
	}
	authCtx, err := s.authorizeAccess(ctx, types.KindDevice, verbs...)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.AuthorizeAdminAction(); err != nil {
		return nil, trace.Wrap(err)
	}

	dev, err := s.storage.CreateDevice(ctx, req.Device, req.CreateAsResource)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	s.emitAuditEvent(ctx, &apievents.DeviceEvent2{
		Metadata: apievents.Metadata{
			Type: events.DeviceCreateEvent,
			Code: events.DeviceCreateCode,
		},
		Status: apievents.Status{
			Success: true,
		},
		Device:       getDeviceMetadata(dev),
		UserMetadata: getUserMetadata(ctx),
	})

	if req.CreateEnrollToken {
		token, err := s.storage.CreateDeviceEnrollToken(ctx, dev.Id, getExpireTime(req.EnrollTokenExpireTime))
		s.emitAuditEvent(ctx, &apievents.DeviceEvent2{
			Metadata: apievents.Metadata{
				Type: events.DeviceEnrollTokenCreateEvent,
				Code: events.DeviceEnrollTokenCreateCode,
			},
			Status: apievents.Status{
				Success: err == nil,
			},
			Device:       getDeviceMetadata(dev),
			UserMetadata: getUserMetadata(ctx),
		})
		if err != nil {
			s.logger.WarnContext(ctx,
				"Failed to create device enrollment token, returning device without it",
				"error", err,
			)
		} else {
			dev.EnrollToken = token
		}
	}

	return dev, nil
}

func (s *Service) UpdateDevice(ctx context.Context, req *devicepb.UpdateDeviceRequest) (*devicepb.Device, error) {
	authCtx, err := s.authorizeAccess(ctx, types.KindDevice, types.VerbUpdate)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.AuthorizeAdminAction(); err != nil {
		return nil, trace.Wrap(err)
	}

	switch {
	case req.Device == nil:
		return nil, trace.BadParameter("device required")
	case req.Device.Id == "":
		return nil, trace.BadParameter("device ID required")
	case req.UpdateMask == nil:
		return nil, trace.BadParameter("update mask required")
	}
	dev := req.Device
	paths := req.UpdateMask.Paths

	// Validate update mask before hitting storage.
	if err := applyDeviceUpdateMask(paths, dev, dev); err != nil {
		return nil, trace.Wrap(err)
	}

	updated, err := s.storage.UpdateDevice(ctx, dev.Id, func(stored *devicepb.Device) *devicepb.Device {
		// err is safe to swallow if the validation above passed.
		_ = applyDeviceUpdateMask(paths, stored, dev)
		return stored
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	s.emitAuditEvent(ctx, &apievents.DeviceEvent2{
		Metadata: apievents.Metadata{
			Type: events.DeviceUpdateEvent,
			Code: events.DeviceUpdateCode,
		},
		Status: apievents.Status{
			Success: true,
		},
		Device:       getDeviceMetadata(updated),
		UserMetadata: getUserMetadata(ctx),
	})

	return updated, nil
}

func applyDeviceUpdateMask(paths []string, dst, src *devicepb.Device) error {
	if len(paths) == 0 {
		return trace.BadParameter("at least one update mask path is required")
	}

	for _, path := range paths {
		switch path {
		case "enroll_status":
			dst.EnrollStatus = src.EnrollStatus
		case "profile":
			dst.Profile = src.Profile
		case "source":
			dst.Source = src.Source
		default:
			return trace.BadParameter("unsupported update mask path: %q", path)
		}
	}

	return nil
}

func (s *Service) UpsertDevice(ctx context.Context, req *devicepb.UpsertDeviceRequest) (*devicepb.Device, error) {
	authCtx, err := s.authorizeAccess(ctx, types.KindDevice, types.VerbCreate, types.VerbUpdate)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.AuthorizeAdminAction(); err != nil {
		return nil, trace.Wrap(err)
	}

	if req.Device == nil {
		return nil, trace.BadParameter("device required")
	}
	dev := req.Device

	emitEvent := func(eventType, eventCode string, dev *devicepb.Device) {
		s.emitAuditEvent(ctx, &apievents.DeviceEvent2{
			Metadata: apievents.Metadata{
				Type: eventType,
				Code: eventCode,
			},
			Status: apievents.Status{
				Success: true,
			},
			Device:       getDeviceMetadata(dev),
			UserMetadata: getUserMetadata(ctx),
		})
	}

	// Attempt an update first, if it makes sense.
	if dev.Id != "" {
		updated, err := s.storage.UpdateDevice(ctx, dev.Id, func(stored *devicepb.Device) *devicepb.Device {
			// Be nice and fill in ApiVersion if it's empty.
			if dev.ApiVersion == "" {
				dev.ApiVersion = stored.ApiVersion
			}

			// Play nice with the Terraform provider and ignore changes on fields it
			// loses precision (like Timestamps) or doesn't manage (Credential).
			dev.CreateTime = stored.CreateTime
			dev.UpdateTime = stored.UpdateTime
			dev.Credential = stored.Credential
			dev.Owner = stored.Owner

			// Use the request device for all else.
			return dev
		})
		switch {
		case err == nil:
			emitEvent(events.DeviceUpdateEvent, events.DeviceUpdateCode, updated)
			return updated, nil
		case !trace.IsNotFound(err):
			return nil, trace.Wrap(err)
		}
		// NotFound errors fall into the create flow.
	}

	created, err := s.storage.CreateDevice(ctx, dev, req.CreateAsResource)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	emitEvent(events.DeviceCreateEvent, events.DeviceCreateCode, created)

	return created, nil
}

func (s *Service) DeleteDevice(ctx context.Context, req *devicepb.DeleteDeviceRequest) (*emptypb.Empty, error) {
	authCtx, err := s.authorizeAccess(ctx, types.KindDevice, types.VerbDelete)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.AuthorizeAdminAction(); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.storage.DeleteDevice(ctx, req.DeviceId); err != nil {
		return nil, trace.Wrap(err)
	}
	s.emitAuditEvent(ctx, &apievents.DeviceEvent2{
		Metadata: apievents.Metadata{
			Type: events.DeviceDeleteEvent,
			Code: events.DeviceDeleteCode,
		},
		Status: apievents.Status{
			Success: true,
		},
		Device: &apievents.DeviceMetadata{
			// Without extra queries, the device ID is all we got here.
			DeviceId: req.DeviceId,
		},
		UserMetadata: getUserMetadata(ctx),
	})

	return &emptypb.Empty{}, nil
}

func (s *Service) FindDevices(ctx context.Context, req *devicepb.FindDevicesRequest) (*devicepb.FindDevicesResponse, error) {
	if _, err := s.authorizeAccess(ctx, types.KindDevice, types.VerbList, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	if req.IdOrTag == "" {
		return nil, trace.BadParameter("id_or_tag required")
	}

	innerCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Fire read by ID concurrently. This should speed things up a bit without
	// being a huge cost.
	type deviceRead struct {
		dev *devicepb.Device
		err error
	}
	readC := make(chan deviceRead, 1)
	go func() {
		dev, err := s.storage.GetDeviceByID(innerCtx, req.IdOrTag)
		readC <- deviceRead{
			dev: dev,
			err: trace.Wrap(err),
		}
	}()

	// Read devices by asset tag
	devs, err := s.storage.GetDevicesByAssetTag(innerCtx, req.IdOrTag)
	if err != nil {
		// Be nice and wait for our goroutines to complete.
		cancel()
		<-readC

		return nil, trace.Wrap(err, "reading devices by asset tag")
	}

	// Sync with read by ID.
	r := <-readC
	if r.err != nil && !trace.IsNotFound(r.err) {
		return nil, trace.Wrap(err, "reading device by ID")
	}
	// Prepend ID match to the results, it's the stronger match.
	if r.dev != nil {
		devs = append([]*devicepb.Device{r.dev}, devs...)
	}

	return &devicepb.FindDevicesResponse{
		Devices: devs,
	}, nil
}

func (s *Service) GetDevice(ctx context.Context, req *devicepb.GetDeviceRequest) (*devicepb.Device, error) {
	if _, err := s.authorizeAccess(ctx, types.KindDevice, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}

	dev, err := s.storage.GetDeviceByID(ctx, req.DeviceId)
	return dev, trace.Wrap(err)
}

func (s *Service) ListDevices(ctx context.Context, req *devicepb.ListDevicesRequest) (*devicepb.ListDevicesResponse, error) {
	if _, err := s.authorizeAccess(ctx, types.KindDevice, types.VerbList, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}

	// Default to "list" view if not specified.
	view := req.View
	if view == devicepb.DeviceView_DEVICE_VIEW_UNSPECIFIED {
		view = devicepb.DeviceView_DEVICE_VIEW_LIST
	}

	devs, nextPageToken, err := s.storage.ListDevices(ctx, int(req.PageSize), req.PageToken, view)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &devicepb.ListDevicesResponse{
		Devices:       devs,
		NextPageToken: nextPageToken,
	}, nil
}

func (s *Service) ListDevicesByUser(ctx context.Context, req *devicepb.ListDevicesByUserRequest) (*devicepb.ListDevicesByUserResponse, error) {
	authCtx, err := s.authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	devs, nextToken, err := s.storage.ListDevicesByUser(ctx, int(req.PageSize), req.PageToken, authCtx.User.GetName())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &devicepb.ListDevicesByUserResponse{
		Devices:       devs,
		NextPageToken: nextToken,
	}, nil
}

func (s *Service) BulkCreateDevices(ctx context.Context, req *devicepb.BulkCreateDevicesRequest) (*devicepb.BulkCreateDevicesResponse, error) {
	authCtx, err := s.authorizeAccess(ctx, types.KindDevice, types.VerbCreate)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.AuthorizeAdminAction(); err != nil {
		return nil, trace.Wrap(err)
	}

	if len(req.Devices) == 0 {
		return nil, trace.BadParameter("devices required")
	}

	devs := s.storage.BulkCreateDevices(ctx, req.Devices, req.CreateAsResource)

	// Emit audit events.
	for _, created := range devs {
		if created.GetId() == "" {
			continue
		}
		s.emitAuditEvent(ctx, &apievents.DeviceEvent2{
			Metadata: apievents.Metadata{
				Type: events.DeviceCreateEvent,
				Code: events.DeviceCreateCode,
			},
			Status: apievents.Status{
				Success: true,
			},
			Device: &apievents.DeviceMetadata{
				DeviceId: created.Id,
			},
			UserMetadata: getUserMetadata(ctx),
		})
	}

	return &devicepb.BulkCreateDevicesResponse{
		Devices: devs,
	}, nil
}

func (s *Service) CreateDeviceEnrollToken(ctx context.Context, req *devicepb.CreateDeviceEnrollTokenRequest) (_ *devicepb.DeviceEnrollToken, err error) {
	start := time.Now()
	defer func() {
		createEnrollTokenHist.
			WithLabelValues(toGRPCCode(err)).
			Observe(time.Since(start).Seconds())
	}()

	authorizeOutcome, err :=
		s.authorizeWithAutoEnrollExemption(ctx, types.KindDevice, types.VerbCreateEnrollToken)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	authCtx := authorizeOutcome.authCtx
	autoEnrollEnabled := authorizeOutcome.autoEnrollEnabled
	allowedByAutoEnroll := authorizeOutcome.allowedByAutoEnroll

	// Auto-enroll if:
	// - User failed verb check (aka allowedByAutoEnroll)
	// - User succeeded verb check, but only supplied auto-enroll information.
	//   (Otherwise, favor legacy behavior.)
	autoEnroll := allowedByAutoEnroll || (req.DeviceId == "" && req.DeviceData != nil && autoEnrollEnabled)

	var devMetadata *apievents.DeviceMetadata
	var token *devicepb.DeviceEnrollToken
	if autoEnroll {
		// Rate limit auto-enroll/data-based token creation.
		if err := s.rateLimitByUser(authCtx.User.GetName()); err != nil {
			return nil, trace.Wrap(err)
		}

		dev, err := s.storage.CreateDeviceEnrollTokenUsingData(ctx, req.DeviceData)
		devMetadata = getDeviceMetadata(dev)
		if err != nil {
			// Record auto-enroll failures to audit, it can be hard to diagnose
			// otherwise.
			s.emitAuditEvent(ctx, &apievents.DeviceEvent2{
				Metadata: apievents.Metadata{
					Type: events.DeviceEnrollTokenCreateEvent,
					Code: events.DeviceEnrollTokenCreateCode,
				},
				Device: devMetadata,
				Status: apievents.Status{
					Success:     false,
					UserMessage: err.Error(),
				},
				UserMetadata: getUserMetadata(ctx),
			})

			err = s.redactAutoEnrollError(err, dev, authCtx.User.GetName(), allowedByAutoEnroll)
			return nil, trace.Wrap(err)
		}

		token = dev.GetEnrollToken()
	} else {
		if err := authCtx.AuthorizeAdminAction(); err != nil {
			return nil, trace.Wrap(err)
		}

		var err error
		token, err = s.storage.CreateDeviceEnrollToken(ctx, req.DeviceId, getExpireTime(req.ExpireTime))
		if err != nil {
			return nil, trace.Wrap(err)
		}

		devMetadata = &apievents.DeviceMetadata{
			DeviceId: req.DeviceId,
		}
	}
	s.emitAuditEvent(ctx, &apievents.DeviceEvent2{
		Metadata: apievents.Metadata{
			Type: events.DeviceEnrollTokenCreateEvent,
			Code: events.DeviceEnrollTokenCreateCode,
		},
		Status: apievents.Status{
			Success: true,
		},
		Device: devMetadata,
		// Don't log the user TrustedDevice here, they didn't pass a device
		// challenge yet.
		UserMetadata: getUserMetadata(ctx),
	})

	return token, nil
}

type authorizeWithAutoEnrollOutcome struct {
	authCtx             *authz.Context
	autoEnrollEnabled   bool
	allowedByAutoEnroll bool  // Implies `autoEnrollEnabled && checkErr != nil`.
	checkErr            error // CheckAccessToRule error for supplied verbs.
}

func (s *Service) authorizeWithAutoEnrollExemption(ctx context.Context, kind, verb string) (*authorizeWithAutoEnrollOutcome, error) {
	// Authorize user.
	authCtx, err := s.authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Fetch auto-enroll setting.
	authPref, err := s.authServer.GetAuthPreference(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	autoEnrollEnabled := authPref.GetDeviceTrust() != nil && authPref.GetDeviceTrust().AutoEnroll

	// Verify access to the required verbs, allowing for an exemption if
	// auto-enroll is enabled.
	allowedByAutoEnroll := false
	checkErr := authCtx.Checker.CheckAccessToRule(
		&services.Context{User: authCtx.User},
		defaults.Namespace, kind, verb,
	)
	if checkErr != nil && autoEnrollEnabled {
		allowedByAutoEnroll = true
	} else if checkErr != nil {
		return nil, trace.Wrap(checkErr)
	}

	return &authorizeWithAutoEnrollOutcome{
		authCtx:             authCtx,
		autoEnrollEnabled:   autoEnrollEnabled,
		allowedByAutoEnroll: allowedByAutoEnroll,
		checkErr:            checkErr,
	}, nil
}

func (s *Service) redactAutoEnrollError(actualErr error, dev *devicepb.Device, user string, allowedByAutoEnroll bool) error {
	if actualErr == nil {
		return nil
	}

	if allowedByAutoEnroll {
		s.logger.WarnContext(context.Background(),
			"Attempt to issue device enrollment token via auto-enroll denied",
			"error", actualErr,
			"user", user,
			"device_id", dev.GetId(),
			"asset_tag", dev.GetAssetTag(),
		)

		// Reply with a redacted error so we don't relay information about what
		// might be wrong with the collected data.
		return trace.BadParameter("auto-enroll verifications failed")
	}

	// Transform drift errors into BadParameter, but otherwise no need to redact.
	// The user already has permissions to create tokens without data.
	if errors.Is(actualErr, &storage.CollectedDataDriftError{}) {
		actualErr = trace.BadParameter(actualErr.Error())
	}
	return trace.Wrap(actualErr)
}

func (s *Service) EnrollDevice(stream devicepb.DeviceTrustService_EnrollDeviceServer) (err error) {
	start := time.Now()
	ctx := stream.Context()
	defer func() {
		if err != nil {
			if errors.Is(err, storage.ErrEnrolledDeviceLimit) {
				s.emitDeviceLimitEvent(prehogv1alpha.LicenseLimit_LICENSE_LIMIT_DEVICE_TRUST_TEAM_USAGE)
			}
			s.logger.DebugContext(ctx,
				"EnrollDevice stream exited with error",
				"error", err,
				"code", status.Code(trail.ToGRPC(err)),
			)
		}

		enrollHist.
			WithLabelValues(toGRPCCode(err)).
			Observe(time.Since(start).Seconds())
	}()

	var dev *devicepb.Device
	defer func() { err = s.redactDataDriftErr(dev, err) }()

	// Both device/enroll or auto-enroll allow access to enrolling devices
	// (provided the user has a token, of course).
	authorizeOutcome, err :=
		s.authorizeWithAutoEnrollExemption(ctx, types.KindDevice, types.VerbEnroll)
	if err != nil {
		return trace.Wrap(err)
	}
	authCtx := authorizeOutcome.authCtx
	allowedByAutoEnroll := authorizeOutcome.allowedByAutoEnroll
	checkErr := authorizeOutcome.checkErr

	if err := s.storage.VerifyEnrolledDevicesLimit(ctx); err != nil {
		return trace.Wrap(err)
	}
	user := authCtx.User.GetName()

	authPref, err := s.authServer.GetAuthPreference(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	var ekCertAllowedCAs []string
	if authPref.GetDeviceTrust() != nil {
		ekCertAllowedCAs = authPref.GetDeviceTrust().EKCertAllowedCAs
	}

	// Attempt to enroll the device.
	c := &enrollCeremony{
		logger:           s.logger,
		storage:          s.storage,
		ekCertAllowedCAs: ekCertAllowedCAs,
		auditCallback: func(dev *devicepb.Device, err error) {
			success := err == nil
			devMetadata := getDeviceMetadata(dev)
			userMetadata := getUserMetadata(ctx)

			// Manually assign the device in use, if successful.
			// At this stage the device is not in the user certificate.
			if success {
				userMetadata.TrustedDevice = devMetadata
			}

			s.emitAuditEvent(ctx, &apievents.DeviceEvent2{
				Metadata: apievents.Metadata{
					Type: events.DeviceEnrollEvent,
					Code: events.DeviceEnrollCode,
				},
				Status: apievents.Status{
					Success: success,
				},
				Device:       devMetadata,
				UserMetadata: userMetadata,
			})
		},
	}
	dev, err = c.EnrollDevice(stream, user, allowedByAutoEnroll)
	// If denied because the user tried to spend a non-auto enroll token, then
	// return the original CheckAccessToRule error.
	// Sadly the token is already spent at this stage, which is not the behavior
	// for an ordinary permission failure.
	if errors.Is(err, errDeniedByNonAutoToken) {
		s.logger.DebugContext(ctx,
			"Denied by non auto-enroll error swallowed, user is missing device/enroll permissions",
			"error", err,
		)
		// err swallowed on purpose.
		return trace.Wrap(checkErr)
	}
	return trace.Wrap(err)
}

var authnDisabledLogOnce sync.Once

func (s *Service) AuthenticateDevice(stream devicepb.DeviceTrustService_AuthenticateDeviceServer) (err error) {
	start := time.Now()
	ctx := stream.Context()
	var isWebAuthentication bool // Set in the audit step.
	defer func() {
		if err != nil {
			s.logger.DebugContext(ctx,
				"AuthenticateDevice stream exited with error",
				"error", err,
				"code", status.Code(trail.ToGRPC(err)),
			)
		}

		authnHist.
			WithLabelValues(
				toGRPCCode(err),
				strconv.FormatBool(isWebAuthentication),
			).
			Observe(time.Since(start).Seconds())
	}()

	var dev *devicepb.Device
	defer func() { err = s.redactDataDriftErr(dev, err) }()

	// Authenticate the user, but do not perform any additional authorization
	// checks. Any user may authenticate devices.
	authCtx, err := s.authorize(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	// Is device authentication allowed?
	authPref, err := s.authServer.GetAuthPreference(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := s.isDeviceAuthnAllowed(authPref.GetDeviceTrust()); err != nil {
		authnDisabledLogOnce.Do(func() {
			s.logger.WarnContext(ctx, "Device authentication attempted, but device trust is disabled by cluster settings")
		})
		return trace.Wrap(err)
	}

	// Rate limit device authn.
	user := authCtx.User.GetName()
	if err := s.rateLimitByUser(user); err != nil {
		return trace.Wrap(err)
	}

	c := &authnCeremony{
		logger:      s.logger,
		storage:     s.storage,
		cachedUsers: s.cachedUsers,
		augmentCertsFunc: func(ctx context.Context, opts *auth.AugmentUserCertificateOpts) (*clientpb.Certs, error) {
			certs, err := s.authServer.AugmentContextUserCertificates(ctx, authCtx, opts)
			return certs, trace.Wrap(err)
		},
		auditCallback: func(dev *devicepb.Device, auditData *deviceAuthnAuditData, err error) {
			success := err == nil
			devMetadata := getDeviceMetadata(dev)
			userMetadata := getUserMetadata(ctx)

			// Assign web authentication fields.
			if devMetadata != nil && auditData != nil {
				isWebAuthentication = auditData.HasDeviceWebToken // written to authnHist
				devMetadata.WebAuthentication = isWebAuthentication
				devMetadata.WebAuthenticationId = auditData.WebAuthenticationID
			}

			// Manually assign the device in use, if successful.
			// At this stage the device is not in the user certificate.
			if success {
				userMetadata.TrustedDevice = devMetadata
			}

			s.emitAuditEvent(ctx, &apievents.DeviceEvent2{
				Metadata: apievents.Metadata{
					Type: events.DeviceAuthenticateEvent,
					Code: events.DeviceAuthenticateCode,
				},
				Status: apievents.Status{
					Success:     success,
					UserMessage: getUserMessage(err),
				},
				Device:       devMetadata,
				UserMetadata: userMetadata,
			})
		},
	}
	dev, err = c.AuthenticateDevice(ctx, stream, user)
	return trace.Wrap(err)
}

func (s *Service) isDeviceAuthnAllowed(dt *types.DeviceTrust) error {
	if dtconfig.GetEffectiveMode(dt) == constants.DeviceTrustModeOff {
		return trace.Wrap(errDeviceTrustDisabled)
	}
	return nil
}

func (s *Service) ConfirmDeviceWebAuthentication(ctx context.Context, req *devicepb.ConfirmDeviceWebAuthenticationRequest) (_ *devicepb.ConfirmDeviceWebAuthenticationResponse, err error) {
	start := time.Now()
	defer func() {
		confirmDeviceWebAuthenticationHist.
			WithLabelValues(toGRPCCode(err)).
			Observe(time.Since(start).Seconds())
	}()

	switch {
	case req.ConfirmationToken == nil:
		return nil, trace.BadParameter("confirmation token required")
	case req.CurrentWebSessionId == "":
		return nil, trace.BadParameter("current web session ID required")
	}

	// Only the Proxy may call this RPC.
	authCtx, err := s.authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if !authz.HasBuiltinRole(*authCtx, string(types.RoleProxy)) {
		return nil, trace.AccessDenied("access denied")
	}

	tokenData, dev, err := s.confirmDeviceWebAuthentication(ctx, req)
	// err handled after audit.

	var deviceID, user string
	if tokenData != nil {
		deviceID = tokenData.AuthenticatedDeviceID
		user = tokenData.User
	}

	s.emitAuditEvent(ctx, &apievents.DeviceEvent2{
		Metadata: apievents.Metadata{
			Type: events.DeviceAuthenticateConfirmEvent,
			Code: events.DeviceAuthenticateConfirmCode,
		},
		Device: &apievents.DeviceMetadata{
			DeviceId:            deviceID,
			WebAuthenticationId: req.ConfirmationToken.Id,
		},
		Status: apievents.Status{
			Success:     err == nil,
			UserMessage: getUserMessage(err),
		},
		UserMetadata: apievents.UserMetadata{
			User:          user,
			TrustedDevice: getDeviceMetadata(dev),
		},
	})

	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &devicepb.ConfirmDeviceWebAuthenticationResponse{}, nil
}

func (s *Service) confirmDeviceWebAuthentication(
	ctx context.Context,
	req *devicepb.ConfirmDeviceWebAuthenticationRequest,
) (*storage.DeviceConfirmationTokenData, *devicepb.Device, error) {
	tokenData, err := s.storage.SpendDeviceConfirmationToken(ctx, req.ConfirmationToken)
	if err != nil {
		s.logger.DebugContext(ctx,
			"Failed to spend device confirmation token",
			"error", err,
		)
		// err swallowed on purpose.
		return tokenData, nil, auditStatusError{
			Err:         trace.Wrap(errInvalidDeviceConfirmationToken),
			UserMessage: "invalid device confirmation token",
		}
	}
	// Always return tokenData for audit purposes.

	if req.CurrentWebSessionId != tokenData.WebSessionID {
		return tokenData, nil, auditStatusError{
			Err:         trace.Wrap(errInvalidDeviceConfirmationToken),
			UserMessage: "token move check failed",
		}
	}

	dev, err := s.storage.GetDeviceByID(ctx, tokenData.AuthenticatedDeviceID)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	// Always return dev for audit purposes.

	if err := s.authServer.AugmentWebSessionCertificates(ctx, &auth.AugmentWebSessionCertificatesOpts{
		WebSessionID: tokenData.WebSessionID,
		User:         tokenData.User,
		DeviceExtensions: &auth.DeviceExtensions{
			DeviceID:     dev.Id,
			AssetTag:     dev.AssetTag,
			CredentialID: dev.Credential.Id,
		},
	}); err != nil {
		s.logger.DebugContext(ctx,
			"Failed to augment WebSession certificates",
			"error", err,
		)
		return tokenData, dev, auditStatusError{
			Err:         trace.Wrap(err),
			UserMessage: "failed to issue device web certificates",
		}
	}

	return tokenData, dev, nil
}

func (s *Service) SyncInventory(stream devicepb.DeviceTrustService_SyncInventoryServer) error {
	verbs := []string{
		types.VerbCreate,
		types.VerbUpdate,
		types.VerbList,   // listing of missing devices
		types.VerbDelete, // removal of missing devices
	}
	ctx := stream.Context()
	authCtx, err := s.authorizeAccess(ctx, types.KindDevice, verbs...)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := authCtx.AuthorizeAdminAction(); err != nil {
		return trace.Wrap(err)
	}

	if f := modules.GetModules().Features(); !f.GetEntitlement(entitlements.MobileDeviceManagement).Enabled {
		// TODO(sshah): update event type once Intune integration is supported.
		s.emitDeviceLimitEvent(prehogv1alpha.LicenseLimit_LICENSE_LIMIT_DEVICE_TRUST_TEAM_JAMF)
		return trace.AccessDenied(
			"this Teleport cluster is not licensed for MDM integrations, please contact the cluster administrator")
	}

	userMeta := getUserMetadata(ctx)
	auditCB := func(eventType, eventCode string, dev *devicepb.Device, err error) {
		if err != nil {
			return // Don't issue failures for create/update/delete.
		}
		s.emitAuditEvent(ctx, &apievents.DeviceEvent2{
			Metadata: apievents.Metadata{
				Type: eventType,
				Code: eventCode,
			},
			Status: apievents.Status{
				Success: true,
			},
			Device:       getDeviceMetadata(dev),
			UserMetadata: userMeta,
		})
	}

	incCounter := func(op string, err error) {
		if err != nil {
			syncOperationsTotal.WithLabelValues("errors").Inc()
			return
		}
		syncOperationsTotal.WithLabelValues(op).Inc()
	}

	syncer := &inventorySyncer{
		storage: s.storage,
		createCallback: func(dev *devicepb.Device, err error) {
			incCounter("create", err)
			auditCB(events.DeviceCreateEvent, events.DeviceCreateCode, dev, err)
		},
		updateCallback: func(dev *devicepb.Device, err error) {
			incCounter("update", err)
			auditCB(events.DeviceUpdateEvent, events.DeviceUpdateCode, dev, err)
		},
		noopCallback: func(_ *devicepb.Device, err error) {
			incCounter("noop", err)
		},
		deleteCallback: func(dev *devicepb.Device, err error) {
			incCounter("delete", err)
			auditCB(events.DeviceDeleteEvent, events.DeviceDeleteCode, dev, err)
		},
	}
	return trace.Wrap(syncer.SyncInventory(stream))
}

// GetResourceDevicesUsage returns the trusted device limit and usage for
// usage-based accounts.
//
// Unlike other service methods, this is not an RPC.
//
// Returns a default instance for non-usage-based accounts.
func (s *Service) GetResourceDevicesUsage(ctx context.Context, f *modules.Features) (*resourceusagepb.DevicesUsage, error) {
	switch {
	case f == nil:
		return nil, trace.BadParameter("features cannot be nil")
	case !f.IsUsageBasedBilling:
		return &resourceusagepb.DevicesUsage{}, nil
	}

	usage, err := s.storage.GetDevicesUsage(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &resourceusagepb.DevicesUsage{
		DevicesUsageLimit: f.GetEntitlement(entitlements.DeviceTrust).Limit,
		DevicesInUse:      int32(usage.NumEnrolled),
	}, nil
}

func (s *Service) GetDevicesUsage(_ context.Context, _ *devicepb.GetDevicesUsageRequest) (*devicepb.DevicesUsage, error) {
	return nil, trace.BadParameter("deprecated, use ResourceUsageService.GetUsage instead")
}

// CreateDeviceWebToken creates a device web token for a recently logged in Web
// user.
//
// Returns `nil, nil` if the user has no suitable trusted device (ie, token
// creation was not attempted).
//
// Returns a token if creation is successful or an error in other cases.
//
// CreateDeviceWebToken is not an RPC. Instead, it is called directly by the
// Auth Server's web login logic.
func (s *Service) CreateDeviceWebToken(ctx context.Context, token *devicepb.DeviceWebToken) (_ *devicepb.DeviceWebToken, err error) {
	start := time.Now()
	defer func() {
		createDeviceWebTokenHist.
			WithLabelValues(toGRPCCode(err)).
			Observe(time.Since(start).Seconds())
	}()

	switch {
	case token == nil:
		return nil, trace.BadParameter("device web token required")
	case token.BrowserUserAgent == "":
		return nil, trace.BadParameter("browser user agent required")
	case token.User == "":
		return nil, trace.BadParameter("user required")
	}
	// Fields we don't use directly in this method are validated by the storage
	// write.

	// Is device authentication allowed? Don't continue otherwise.
	authPref, err := s.authServer.GetAuthPreference(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := s.isDeviceAuthnAllowed(authPref.GetDeviceTrust()); err != nil {
		// Device authn not allowed, simply return a nil token.
		// err swallowed on purpose.
		return nil, nil
	}

	// Parse user agent, determine OS.
	expectedOS := loggedGetOSFromUserAgent(s.logger, token.BrowserUserAgent)
	if expectedOS == devicepb.OSType_OS_TYPE_UNSPECIFIED {
		return nil, trace.BadParameter("cannot parse OS from user agent")
	}

	// Fetch user devices.
	userDevices, err := s.getUserTrustedDevices(ctx, token.User)
	switch {
	case err != nil:
		return nil, trace.Wrap(err, "reading user trusted devices")
	case len(userDevices) == 0:
		s.logger.DebugContext(ctx,
			"User has no trusted devices, skipping DeviceWebToken creation",
			"user", token.User,
		)
		return nil, nil
	}

	// Determine Expected Device IDs.
	var deviceIDs []string
	var auditDev *devicepb.Device
	for _, dev := range userDevices {
		if dev.OsType != expectedOS {
			continue
		}

		// Device allowed to authenticate.
		deviceIDs = append(deviceIDs, dev.Id)

		// Pick one of the devices as the audit target.
		// This is correct for users with a single suitable device, but just a guess
		// in other cases.
		if auditDev == nil {
			auditDev = dev
		}
	}
	if len(deviceIDs) == 0 {
		s.logger.DebugContext(ctx,
			"User has no suitable trusted device for Web authentication",
			"os", expectedOS,
			"user", token.User,
		)
		return nil, nil // User has no suitable trusted devices.
	}

	// Avoid modifying input.
	createToken := proto.Clone(token).(*devicepb.DeviceWebToken)
	createToken.ExpectedDeviceIds = deviceIDs

	created, err := s.storage.CreateDeviceWebToken(ctx, createToken)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	devMetadata := getDeviceMetadata(auditDev)
	devMetadata.WebAuthenticationId = created.Id
	s.emitAuditEvent(ctx, &apievents.DeviceEvent2{
		Metadata: apievents.Metadata{
			Type: events.DeviceWebTokenCreateEvent,
			Code: events.DeviceWebTokenCreateCode,
		},
		Status: apievents.Status{
			Success: true,
		},
		Device: devMetadata,
		// Do not use getUserMetadata, the context user is the Auth process.
		UserMetadata: apievents.UserMetadata{
			User: token.User,
		},
	})

	return created, nil
}

func (s *Service) getUserTrustedDevices(ctx context.Context, user string) ([]*devicepb.Device, error) {
	deviceIDs, err := s.getCombinedUserTrustedDeviceIDs(ctx, user)
	if err != nil {
		return nil, trace.Wrap(err, "reading user trusted devices")
	}

	devs, err := s.getDevicesByID(ctx, deviceIDs)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Filter out devices based on ownership.
	for i := 0; i < len(devs); i++ {
		dev := devs[i]

		if dev.Owner == user {
			continue
		}

		devs = slices.Delete(devs, i, i+1)
		i--
	}
	return devs, nil
}

// getCombinedUserTrustedDeviceIDs returns the trusted device IDs for the user,
// combining the values from s.storage.GetTrustedDeviceIDs() and
// user.GetUserTrustedDeviceIDs().
//
// The presence of an ID in the list doesn't necessarily mean the user still
// owns the device, as we could be looking at outdated (or manually edited)
// data.
func (s *Service) getCombinedUserTrustedDeviceIDs(ctx context.Context, user string) ([]string, error) {
	const numGoroutines = 2
	deviceIDsC := make(chan []string, numGoroutines)
	g, gCtx := errgroup.WithContext(ctx)

	// devicesByUser.
	g.Go(func() error {
		deviceIDs, err := s.storage.GetUserTrustedDeviceIDs(gCtx, user)
		if err != nil {
			return trace.Wrap(err)
		}

		deviceIDsC <- deviceIDs
		return nil
	})

	// User.TrustedDeviceIDs.
	g.Go(func() error {
		u, err := s.cachedUsers.GetUser(gCtx, user, false /* withSecrets */)
		if err != nil {
			return trace.Wrap(err)
		}

		deviceIDsC <- u.GetTrustedDeviceIDs()
		return nil
	})

	// Fail on errors so it fails early. Technically we could take either
	// response and keep going, but that might create a more confusing failure (or
	// soft-failure) later on.
	if err := g.Wait(); err != nil {
		return nil, trace.Wrap(err)
	}
	close(deviceIDsC)

	// Receive and dedup.
	seenIDs := make(map[string]struct{})
	var deviceIDs []string
	for ids := range deviceIDsC {
		for _, id := range ids {
			if _, ok := seenIDs[id]; ok {
				continue
			}
			seenIDs[id] = struct{}{}
			deviceIDs = append(deviceIDs, id)
		}
	}
	return deviceIDs, nil
}

// CreateAssertCeremony creates a new [assert.Ceremony] backed by this
// [Service].
func (s *Service) CreateAssertCeremony() (assertserver.Ceremony, error) {
	return &assertCeremony{
		logger: s.logger,
		impl: &authnCeremony{
			logger:            s.logger,
			storage:           s.storage,
			skipOwnerBackfill: true, // Don't backfill, caller may not be the owner.
			augmentCertsFunc:  nil,  // Don't issue certificates.
			auditCallback: func(*devicepb.Device, *deviceAuthnAuditData, error) {
				// Audit is responsibility of the caller.
			},
		},
	}, nil
}

// getDevicesByID reads devices from storage concurrently.
//
// Returns the devices read and the aggregated errors.
func (s *Service) getDevicesByID(ctx context.Context, ids []string) ([]*devicepb.Device, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	var g errgroup.Group
	const maxGoroutines = 4 // Arbitrary. Should be just fine for most users.
	g.SetLimit(maxGoroutines)

	// mu guards the variables below it.
	var mu sync.Mutex
	devs := make([]*devicepb.Device, 0, len(ids))
	errs := make([]error, 0, len(ids))

	for _, id := range ids {
		id := id
		g.Go(func() error {
			dev, err := s.storage.GetDeviceByID(ctx, id)
			mu.Lock()
			switch {
			case trace.IsNotFound(err):
				// This could be for a few reasons:
				// * Stale/manually edited User.TrustedDeviceIDs.
				// * Stale /devices/by_user index.
				// This has been observed in practice so it has to be handled
				// gracefully.
				s.logger.DebugContext(ctx, "Queried unknown device ID", "device_id", id)
			case err != nil:
				errs = append(errs, err)
			default:
				devs = append(devs, dev)
			}
			mu.Unlock()
			return nil // Do not fail the errgroup.
		})
	}

	_ = g.Wait() // Safe to swallow, our funcs don't error.

	return devs, trace.NewAggregate(errs...)
}

func (s *Service) redactDataDriftErr(dev *devicepb.Device, err error) error {
	if !errors.Is(err, &storage.CollectedDataDriftError{}) {
		return err
	}

	s.logger.WarnContext(context.Background(),
		"Collected data drift detected",
		"error", err,
		"device_id", dev.GetId(),
		"asset_tag", dev.GetAssetTag(),
	)
	return trace.AccessDenied(DataDriftDetectedMessage)
}

// authorizeAccess authorizes the ctx user, verifies the Device Trust feature
// settings and verifies rule/verb access.
func (s *Service) authorizeAccess(ctx context.Context, rule string, verbs ...string) (*authz.Context, error) {
	authCtx, err := s.authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ruleCtx := &services.Context{
		User: authCtx.User,
	}
	for _, verb := range verbs {
		if err := authCtx.Checker.CheckAccessToRule(ruleCtx, defaults.Namespace, rule, verb); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return authCtx, nil
}

// authorize authorizes the ctx user and verifies the Device Trust feature
// settings.
func (s *Service) authorize(ctx context.Context) (*authz.Context, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if !modules.GetModules().Features().GetEntitlement(entitlements.DeviceTrust).Enabled {
		return nil, trace.AccessDenied("this Teleport cluster is not licensed for device trust, please contact the cluster administrator")
	}

	return authCtx, nil
}

func (s *Service) emitAuditEvent(ctx context.Context, e apievents.AuditEvent) {
	if err := s.emitter.EmitAuditEvent(ctx, e); err != nil {
		um := getUserMetadata(ctx)
		s.logger.WarnContext(ctx,
			"Failed to emit audit event",
			"error", err,
			"type", e.GetType(),
			"code", e.GetCode(),
			"user", um.User,
			"impersonator", um.Impersonator,
		)
	}
}

func (s *Service) emitDeviceLimitEvent(l prehogv1alpha.LicenseLimit) {
	s.authServer.AnonymizeAndSubmit(&usagereporter.LicenseLimitEvent{
		LicenseLimit: l,
	})
}

func (s *Service) rateLimitByUser(user string) error {
	return s.limiter.RegisterRequestWithCustomRate(user, nil /* customRate */)
}

func toGRPCCode(err error) string {
	return status.Code(trail.ToGRPC(err)).String()
}

func getDeviceMetadata(dev *devicepb.Device) *apievents.DeviceMetadata {
	if dev == nil {
		return nil
	}
	return &apievents.DeviceMetadata{
		DeviceId:     dev.Id,
		OsType:       apievents.OSType(dev.OsType),
		AssetTag:     dev.AssetTag,
		CredentialId: dev.Credential.GetId(),
		DeviceOrigin: apievents.DeviceOrigin(dev.Source.GetOrigin()),
	}
}

func getExpireTime(tpb *timestamppb.Timestamp) time.Time {
	if !tpb.IsValid() {
		return time.Time{}
	}
	return tpb.AsTime()
}

func getUserMetadata(ctx context.Context) apievents.UserMetadata {
	return authz.ClientUserMetadata(ctx)
}
