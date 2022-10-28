package devicetrustv1

import (
	"context"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/api/defaults"
	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/devicetrust/storage"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/services"
)

// TODO(codingllama): Pull from api/types.KindDevice
const kindDevice = "device"

// Service implements the teleport.devicetrust.v1.DeviceTrustService RPC
// service.
type Service struct {
	devicepb.UnimplementedDeviceTrustServiceServer

	logger *log.Entry

	authorizer auth.Authorizer
	storage    *storage.S
}

// ServiceParams holds creation parameters for Service.
type ServiceParams struct {
	Authorizer auth.Authorizer
	Storage    *storage.S
}

// New creates a new DeviceTrustService implementer.
func New(params ServiceParams) (*Service, error) {
	switch {
	case params.Authorizer == nil:
		return nil, trace.BadParameter("authorizer required")
	case params.Storage == nil:
		return nil, trace.BadParameter("storage required")
	}

	return &Service{
		logger:     log.WithField(trace.Component, "devicetrust.service"),
		authorizer: params.Authorizer,
		storage:    params.Storage,
	}, nil
}

func (s *Service) CreateDevice(ctx context.Context, req *devicepb.CreateDeviceRequest) (*devicepb.Device, error) {
	if err := s.authorizeVerb(ctx, kindDevice, types.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}

	dev, err := s.storage.CreateDevice(ctx, req.Device)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// TODO(codingllama): Audit.

	if req.CreateEnrollToken {
		token, err := s.storage.CreateDeviceEnrollToken(ctx, dev.Id)
		if err != nil {
			s.logger.
				WithError(err).
				Warn("Failed to create device enrollment token, returning device without it")
		}
		dev.EnrollToken = token
		// TODO(codingllama): Audit.
	}

	return dev, nil
}

func (s *Service) GetDevice(ctx context.Context, req *devicepb.GetDeviceRequest) (*devicepb.Device, error) {
	if err := s.authorizeVerb(ctx, kindDevice, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}

	dev, err := s.storage.GetDeviceByID(ctx, req.DeviceId)
	return dev, trace.Wrap(err)
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
