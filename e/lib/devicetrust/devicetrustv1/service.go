package devicetrustv1

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/gravitational/oxy/ratelimit"
	"github.com/gravitational/trace"
	"github.com/gravitational/trace/trail"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/defaults"
	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	resourceusagepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/resourceusage/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/e/lib/devicetrust/storage"
	prehogv1alpha "github.com/gravitational/teleport/gen/proto/go/prehog/v1alpha"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/authz"
	libdefaults "github.com/gravitational/teleport/lib/defaults"
	config "github.com/gravitational/teleport/lib/devicetrust/config"
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
		Help:      "AuthenticateDevice RPC histogram labeled by grpc_code",
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
		syncOperationsTotal,
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
	AugmentContextUserCertificates(ctx context.Context, authCtx *authz.Context, opts *auth.AugmentUserCertificateOpts) (*proto.Certs, error)

	// GetAuthPreference gets the cluster's auth preferences.
	// This method is not guarded by user permissions.
	GetAuthPreference(ctx context.Context) (types.AuthPreference, error)

	// AnonymizeAndSubmit submits usage events to Prehog.
	AnonymizeAndSubmit(event ...usagereporter.Anonymizable)
}

// UsersService represents the [local.IdentityService] methods used by
// [Service].
type UsersService interface {
	GetUser(ctx context.Context, user string, withSecrets bool) (types.User, error)
}

// RateLimiter is a subset of [limiter.RateLimiter].
type RateLimiter interface {
	RegisterRequest(token string, customRate *ratelimit.RateSet) error
}

// Service implements the teleport.devicetrust.v1.DeviceTrustService RPC
// service.
type Service struct {
	devicepb.UnimplementedDeviceTrustServiceServer

	logger *log.Entry

	authServer  AuthServer
	authorizer  authz.Authorizer
	cachedUsers UsersService
	emitter     apievents.Emitter
	limiter     RateLimiter
	storage     *storage.S
}

// ServiceParams holds creation parameters for Service.
type ServiceParams struct {
	Logger             log.FieldLogger
	AuthServer         AuthServer
	Authorizer         authz.Authorizer
	CachedUsersService UsersService
	Emitter            apievents.Emitter
	Storage            *storage.S

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
	case params.CachedUsersService == nil:
		return nil, trace.BadParameter("parameter CachedUsersService required")
	case params.Emitter == nil:
		return nil, trace.BadParameter("parameter Emitter required")
	case params.Storage == nil:
		return nil, trace.BadParameter("parameter Storage required")
	}

	baseLogger := params.Logger
	if baseLogger == nil {
		baseLogger = log.StandardLogger()
	}

	rateLimiter := params.Limiter
	if rateLimiter == nil {
		var err error
		rateLimiter, err = limiter.NewRateLimiter(limiter.Config{
			Rates: []limiter.Rate{
				{
					Period:  libdefaults.LimiterPeriod,
					Average: libdefaults.LimiterAverage,
					Burst:   libdefaults.LimiterBurst,
				},
			},
			MaxConnections:   libdefaults.LimiterMaxConnections,
			MaxNumberOfUsers: libdefaults.LimiterMaxConcurrentUsers,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	return &Service{
		logger:      baseLogger.WithField(trace.Component, "devicetrust.service"),
		authServer:  params.AuthServer,
		authorizer:  params.Authorizer,
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
	if err := authz.AuthorizeAdminAction(ctx, authCtx); err != nil {
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
			s.logger.
				WithError(err).
				Warn("Failed to create device enrollment token, returning device without it")
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
	if err := authz.AuthorizeAdminAction(ctx, authCtx); err != nil {
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
	if err := authz.AuthorizeAdminAction(ctx, authCtx); err != nil {
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
	if err := authz.AuthorizeAdminAction(ctx, authCtx); err != nil {
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

func (s *Service) BulkCreateDevices(ctx context.Context, req *devicepb.BulkCreateDevicesRequest) (*devicepb.BulkCreateDevicesResponse, error) {
	authCtx, err := s.authorizeAccess(ctx, types.KindDevice, types.VerbCreate)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authz.AuthorizeAdminAction(ctx, authCtx); err != nil {
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

func (s *Service) CreateDeviceEnrollToken(ctx context.Context, req *devicepb.CreateDeviceEnrollTokenRequest) (token *devicepb.DeviceEnrollToken, err error) {
	start := time.Now()
	defer func() {
		createEnrollTokenHist.
			WithLabelValues(status.Code(err).String()).
			Observe(time.Since(start).Seconds())
	}()

	authCtx, err := s.authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	authPref, err := s.authServer.GetAuthPreference(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	autoEnrollEnabled := authPref.GetDeviceTrust() != nil && authPref.GetDeviceTrust().AutoEnroll

	// Verify access to the necessary verbs.
	// It's possible to issue an enroll token without the verb if auto-enrollment
	// is enabled.
	checkErr := authCtx.Checker.CheckAccessToRule(
		&services.Context{User: authCtx.User},
		defaults.Namespace, types.KindDevice, types.VerbCreateEnrollToken,
		false, /* silent */
	)
	if checkErr != nil && !autoEnrollEnabled {
		return nil, trace.Wrap(checkErr)
	}

	// Auto-enroll if:
	// - User failed verb check
	// - User succeeded verb check, but only supplied auto-enroll information.
	//   (Otherwise, favor legacy behavior.)
	var devMetadata *apievents.DeviceMetadata
	if checkErr != nil || (req.DeviceId == "" && req.DeviceData != nil && autoEnrollEnabled) {
		// Rate limit auto-enroll/data-based token creation.
		if err := s.rateLimitByUser(authCtx.User.GetName()); err != nil {
			return nil, trace.Wrap(err)
		}

		var dev *devicepb.Device
		dev, err = s.storage.CreateDeviceEnrollTokenUsingData(ctx, req.DeviceData)
		// err verified below
		token = dev.GetEnrollToken() // This is safe even if `dev` is nil, proto getters don't panic.
		err = s.redactTokenErr(dev, authCtx.User.GetName(), checkErr, err)

		// Audit information.
		devMetadata = getDeviceMetadata(dev)
	} else {
		if err := authz.AuthorizeAdminAction(ctx, authCtx); err != nil {
			return nil, trace.Wrap(err)
		}

		token, err = s.storage.CreateDeviceEnrollToken(ctx, req.DeviceId, getExpireTime(req.ExpireTime))
		// err verified below

		// Audit information.
		devMetadata = &apievents.DeviceMetadata{
			DeviceId: req.DeviceId,
		}
	}
	if err != nil {
		return nil, trace.Wrap(err)
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

func (s *Service) redactTokenErr(dev *devicepb.Device, user string, checkErr, actualErr error) error {
	if actualErr == nil {
		return nil
	}

	if checkErr != nil {
		// Reply with checkErr instead of err, so we don't relay information about
		// what might be wrong with the collected data.
		s.logger.
			WithError(actualErr).
			WithFields(log.Fields{
				"User":     user,
				"DeviceID": dev.GetId(),
				"AssetTag": dev.GetAssetTag(),
			}).
			Warn("Attempt to issue device enrollment token via auto-enroll denied")
		return trace.Wrap(checkErr)
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
	defer func() {
		if err != nil {
			if errors.Is(err, storage.ErrEnrolledDeviceLimit) {
				s.emitDeviceLimitEvent(context.Background(), prehogv1alpha.LicenseLimit_LICENSE_LIMIT_DEVICE_TRUST_TEAM_USAGE)
			}
			s.logger.
				WithFields(log.Fields{
					"code":  status.Code(trail.ToGRPC(err)),
					"error": err.Error(),
				}).
				Debug("EnrollDevice stream exited with error")
		}

		enrollHist.
			WithLabelValues(status.Code(err).String()).
			Observe(time.Since(start).Seconds())
	}()

	var dev *devicepb.Device
	defer func() { err = s.redactDataDriftErr(dev, err) }()

	ctx := stream.Context()
	authCtx, err := s.authorizeAccess(ctx, types.KindDevice, types.VerbEnroll)
	if err != nil {
		return trace.Wrap(err)
	}

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
	dev, err = c.EnrollDevice(stream, user)
	return trace.Wrap(err)
}

var authnDisabledLogOnce sync.Once

func (s *Service) AuthenticateDevice(stream devicepb.DeviceTrustService_AuthenticateDeviceServer) (err error) {
	start := time.Now()
	defer func() {
		if err != nil {
			s.logger.
				WithFields(log.Fields{
					"code":  status.Code(trail.ToGRPC(err)),
					"error": err.Error(),
				}).
				Debug("AuthenticateDevice stream exited with error")
		}

		authnHist.
			WithLabelValues(status.Code(err).String()).
			Observe(time.Since(start).Seconds())
	}()

	var dev *devicepb.Device
	defer func() { err = s.redactDataDriftErr(dev, err) }()

	// Authenticate the user, but do not perform any additional authorization
	// checks. Any user may authenticate devices.
	ctx := stream.Context()
	authCtx, err := s.authorize(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	authPref, err := s.authServer.GetAuthPreference(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	// Is device authn allowed by the cluster mode?
	authnAllowed := config.GetEffectiveMode(authPref.GetDeviceTrust()) != constants.DeviceTrustModeOff

	// If not, is device authn required by the user's roles?
	if !authnAllowed {
		roles := authCtx.Checker.Roles()
		for _, role := range roles {
			deviceMode := role.GetOptions().DeviceTrustMode
			if deviceMode != "" && deviceMode != constants.DeviceTrustModeOff {
				authnAllowed = true
				break
			}
		}
	}
	if !authnAllowed {
		authnDisabledLogOnce.Do(func() {
			s.logger.Warn("Device authentication attempted, but device trust is disabled by cluster settings")
		})
		return trace.BadParameter("device trust disabled by cluster settings")
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
		augmentCertsFunc: func(ctx context.Context, opts *auth.AugmentUserCertificateOpts) (*proto.Certs, error) {
			certs, err := s.authServer.AugmentContextUserCertificates(ctx, authCtx, opts)
			return certs, trace.Wrap(err)
		},
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
					Type: events.DeviceAuthenticateEvent,
					Code: events.DeviceAuthenticateCode,
				},
				Status: apievents.Status{
					Success: success,
				},
				Device:       devMetadata,
				UserMetadata: userMetadata,
			})
		},
	}
	dev, err = c.AuthenticateDevice(stream, user)
	return trace.Wrap(err)
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
	if err := authz.AuthorizeAdminAction(ctx, authCtx); err != nil {
		return trace.Wrap(err)
	}

	// MDMs are disallowed for Teleport Team.
	if f := modules.GetModules().Features(); f.IsTeam() {
		// TODO(sshah): update event type once Intune integration is supported.
		s.emitDeviceLimitEvent(ctx, prehogv1alpha.LicenseLimit_LICENSE_LIMIT_DEVICE_TRUST_TEAM_JAMF)
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
		logger:  s.logger,
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
		DevicesUsageLimit: int32(f.DeviceTrust.DevicesUsageLimit),
		DevicesInUse:      int32(usage.NumEnrolled),
	}, nil
}

func (s *Service) GetDevicesUsage(ctx context.Context, req *devicepb.GetDevicesUsageRequest) (*devicepb.DevicesUsage, error) {
	return nil, trace.BadParameter("deprecated, use ResourceUsageService.GetUsage instead")
}

func (s *Service) redactDataDriftErr(dev *devicepb.Device, err error) error {
	if !errors.Is(err, &storage.CollectedDataDriftError{}) {
		return err
	}

	var fields log.Fields
	if dev != nil {
		fields = log.Fields{
			"device_id": dev.Id,
			"asset_tag": dev.AssetTag,
		}
	}
	s.logger.
		WithError(err).
		WithFields(fields).
		Warn("Collected data drift detected")
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
		if err := authCtx.Checker.CheckAccessToRule(ruleCtx, defaults.Namespace, rule, verb, false /* silent */); err != nil {
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

	if !modules.GetModules().Features().DeviceTrust.Enabled {
		return nil, trace.AccessDenied("this Teleport cluster is not licensed for device trust, please contact the cluster administrator")
	}

	return authCtx, nil
}

func (s *Service) emitAuditEvent(ctx context.Context, e apievents.AuditEvent) {
	if err := s.emitter.EmitAuditEvent(ctx, e); err != nil {
		um := getUserMetadata(ctx)
		s.logger.
			WithError(err).
			WithFields(log.Fields{
				"type":         e.GetType(),
				"code":         e.GetCode(),
				"user":         um.User,
				"impersonator": um.Impersonator,
			}).
			Warn("Failed to emit audit event")
	}
}

func (s *Service) emitDeviceLimitEvent(ctx context.Context, l prehogv1alpha.LicenseLimit) {
	s.authServer.AnonymizeAndSubmit(&usagereporter.LicenseLimitEvent{
		LicenseLimit: l,
	})
}

func (s *Service) rateLimitByUser(user string) error {
	return s.limiter.RegisterRequest(user, nil /* customRate */)
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
