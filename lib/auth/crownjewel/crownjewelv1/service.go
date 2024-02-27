package crownjewelv1

import (
	"context"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/gravitational/teleport"
	crownjewelv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/crownjewel/v1"
	"github.com/gravitational/teleport/api/types"
	conv "github.com/gravitational/teleport/api/types/crownjewel/convert/v1"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/services"
)

// ServiceConfig holds configuration options for the DiscoveryConfig gRPC service.
type ServiceConfig struct {
	// Logger is the logger to use.
	Logger logrus.FieldLogger

	// Authorizer is the authorizer to use.
	Authorizer authz.Authorizer

	// Backend is the backend for storing DiscoveryConfigs.
	Backend services.CrownJewels

	// Clock is the clock.
	Clock clockwork.Clock
}

// CheckAndSetDefaults checks the ServiceConfig fields and returns an error if
// a required param is not provided.
// Authorizer, Cache and Backend are required params
func (s *ServiceConfig) CheckAndSetDefaults() error {
	if s.Authorizer == nil {
		return trace.BadParameter("authorizer is required")
	}
	if s.Backend == nil {
		return trace.BadParameter("backend is required")
	}

	if s.Logger == nil {
		s.Logger = logrus.New().WithField(teleport.ComponentKey, "discoveryconfig_crud_service")
	}

	if s.Clock == nil {
		s.Clock = clockwork.NewRealClock()
	}

	return nil
}

// Service implements the teleport.DiscoveryConfig.v1.DiscoveryConfigService RPC service.
type Service struct {
	crownjewelv1.UnimplementedCrownJewelServiceServer

	log        logrus.FieldLogger
	authorizer authz.Authorizer
	backend    services.CrownJewels
	clock      clockwork.Clock
}

// NewService returns a new DiscoveryConfigs gRPC service.
func NewService(cfg ServiceConfig) (*Service, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &Service{
		log:        cfg.Logger,
		authorizer: cfg.Authorizer,
		backend:    cfg.Backend,
		clock:      cfg.Clock,
	}, nil
}

// CreateCrownJewel ...
func (s *Service) CreateCrownJewel(ctx context.Context, crownJewel *crownjewelv1.CreateCrownJewelRequest) (*crownjewelv1.CreateCrownJewelResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindCrownJewel, types.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}

	rsp, err := s.backend.CreateCrownJewel(ctx, conv.FromProto(crownJewel.CrownJewels))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &crownjewelv1.CreateCrownJewelResponse{
		CrownJewels: conv.ToProto(rsp),
	}, nil
}

func (s *Service) GetCrownJewels(ctx context.Context, req *crownjewelv1.GetCrownJewelsRequest) (*crownjewelv1.GetCrownJewelsResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindCrownJewel, types.VerbRead, types.VerbList); err != nil {
		return nil, trace.Wrap(err)
	}

	rsp, err := s.backend.GetCrownJewels(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var elems []*crownjewelv1.CrownJewel
	for _, elem := range rsp {
		elems = append(elems, conv.ToProto(elem))
	}

	return &crownjewelv1.GetCrownJewelsResponse{
		CrownJewels: elems,
	}, nil
}

func (s *Service) UpdateCrownJewel(ctx context.Context, req *crownjewelv1.UpdateCrownJewelRequest) (*crownjewelv1.UpdateCrownJewelResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindCrownJewel, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}

	rsp, err := s.backend.UpdateCrownJewel(ctx, conv.FromProto(req.CrownJewels))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &crownjewelv1.UpdateCrownJewelResponse{
		CrownJewels: conv.ToProto(rsp),
	}, nil
}

func (s *Service) DeleteCrownJewel(ctx context.Context, req *crownjewelv1.DeleteCrownJewelRequest) (*emptypb.Empty, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindCrownJewel, types.VerbDelete); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.backend.DeleteCrownJewel(ctx, req.GetName()); err != nil {
		return nil, trace.Wrap(err)
	}

	return &emptypb.Empty{}, nil
}

func (s *Service) DeleteAllCrownJewels(ctx context.Context, _ *crownjewelv1.DeleteAllCrownJewelsRequest) (*emptypb.Empty, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindCrownJewel, types.VerbDelete); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.backend.DeleteAllCrownJewels(ctx); err != nil {
		return nil, trace.Wrap(err)
	}

	return &emptypb.Empty{}, nil
}
