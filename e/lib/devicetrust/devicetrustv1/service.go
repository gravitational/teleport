package devicetrustv1

import (
	"context"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/gravitational/teleport/api/defaults"
	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/e/lib/devicetrust/storage"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
)

// Service implements the teleport.devicetrust.v1.DeviceTrustService RPC
// service.
type Service struct {
	devicepb.UnimplementedDeviceTrustServiceServer

	logger *log.Entry

	authorizer auth.Authorizer
	emitter    apievents.Emitter
	storage    *storage.S
}

// ServiceParams holds creation parameters for Service.
type ServiceParams struct {
	Authorizer auth.Authorizer
	Emitter    apievents.Emitter
	Storage    *storage.S
}

// New creates a new DeviceTrustService implementer.
func New(params ServiceParams) (*Service, error) {
	switch {
	case params.Authorizer == nil:
		return nil, trace.BadParameter("authorizer required")
	case params.Emitter == nil:
		return nil, trace.BadParameter("emitter required")
	case params.Storage == nil:
		return nil, trace.BadParameter("storage required")
	}

	return &Service{
		logger:     log.WithField(trace.Component, "devicetrust.service"),
		authorizer: params.Authorizer,
		emitter:    params.Emitter,
		storage:    params.Storage,
	}, nil
}

func (s *Service) CreateDevice(ctx context.Context, req *devicepb.CreateDeviceRequest) (*devicepb.Device, error) {
	if err := s.authorizeVerb(ctx, types.KindDevice, types.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}

	dev, err := s.storage.CreateDevice(ctx, req.Device)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	s.emitAuditEvent(ctx, &apievents.DeviceEvent{
		Metadata: apievents.Metadata{
			Type: events.DeviceEvent,
			Code: events.DeviceCreateCode,
		},
		Status: &apievents.Status{
			Success: true,
		},
		Device: getDeviceMetadata(dev),
		User:   getUserMetadata(ctx),
	})

	if req.CreateEnrollToken {
		token, err := s.storage.CreateDeviceEnrollToken(ctx, dev.Id)
		s.emitAuditEvent(ctx, &apievents.DeviceEvent{
			Metadata: apievents.Metadata{
				Type: events.DeviceEvent,
				Code: events.DeviceEnrollTokenCreateCode,
			},
			Status: &apievents.Status{
				Success: err == nil,
			},
			Device: getDeviceMetadata(dev),
			User:   getUserMetadata(ctx),
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
func (s *Service) DeleteDevice(ctx context.Context, req *devicepb.DeleteDeviceRequest) (*emptypb.Empty, error) {
	if err := s.authorizeVerb(ctx, types.KindDevice, types.VerbDelete); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.storage.DeleteDevice(ctx, req.DeviceId); err != nil {
		return nil, trace.Wrap(err)
	}
	s.emitAuditEvent(ctx, &apievents.DeviceEvent{
		Metadata: apievents.Metadata{
			Type: events.DeviceEvent,
			Code: events.DeviceDeleteCode,
		},
		Status: &apievents.Status{
			Success: true,
		},
		Device: &apievents.DeviceMetadata{
			// Without extra queries, the device ID is all we got here.
			DeviceId: req.DeviceId,
		},
		User: getUserMetadata(ctx),
	})

	return &emptypb.Empty{}, nil
}

func (s *Service) FindDevices(ctx context.Context, req *devicepb.FindDevicesRequest) (*devicepb.FindDevicesResponse, error) {
	if err := s.authorizeVerb(ctx, types.KindDevice, types.VerbList); err != nil {
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
	if err := s.authorizeVerb(ctx, types.KindDevice, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}

	dev, err := s.storage.GetDeviceByID(ctx, req.DeviceId)
	return dev, trace.Wrap(err)
}

func (s *Service) ListDevices(ctx context.Context, req *devicepb.ListDevicesRequest) (*devicepb.ListDevicesResponse, error) {
	if err := s.authorizeVerb(ctx, types.KindDevice, types.VerbList); err != nil {
		return nil, trace.Wrap(err)
	}

	// TODO(codingllama): Implement list views correctly.
	devs, nextPageToken, err := s.storage.ListDevices(ctx, int(req.PageSize), req.PageToken)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &devicepb.ListDevicesResponse{
		Devices:       devs,
		NextPageToken: nextPageToken,
	}, nil
}

func (s *Service) authorizeVerb(ctx context.Context, rule, verb string) error {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	ruleCtx := &services.Context{
		User: authCtx.User,
	}
	err = authCtx.Checker.CheckAccessToRule(ruleCtx, defaults.Namespace, rule, verb, false /* silent */)
	return trace.Wrap(err)
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

func getDeviceMetadata(dev *devicepb.Device) *apievents.DeviceMetadata {
	if dev == nil {
		return nil
	}
	return &apievents.DeviceMetadata{
		DeviceId:     dev.Id,
		OsType:       apievents.OSType(dev.OsType),
		AssetTag:     dev.AssetTag,
		CredentialId: dev.Credential.GetId(),
	}
}

func getUserMetadata(ctx context.Context) *apievents.UserMetadata {
	m := auth.ClientUserMetadata(ctx)
	return &m
}
