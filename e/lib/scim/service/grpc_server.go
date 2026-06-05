package service

import (
	"context"
	"fmt"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/emptypb"

	pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/scim/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/scim/service/common"
	"github.com/gravitational/teleport/e/lib/scim/service/middleware"
	"github.com/gravitational/teleport/e/lib/scim/service/provider"
)

// Service contact point for all SCIM requests, before farming them out to resource-
// and IdP-specific handlers.
type Service struct {
	pb.UnimplementedSCIMServiceServer
	common.Config
	CreatePluginHandler func(plugin types.Plugin, config common.Config, resourceType string) (common.ResourceHandler, error)

	locker common.Locker
}

// NewService creates and configures a new SCIM service
func NewService(cfg *common.Config) (*Service, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	distributeLock, err := common.NewDistributedLocker(cfg.AccessPoint, common.WithClock(cfg.Clock))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &Service{
		Config:              *cfg,
		CreatePluginHandler: provider.CreatePluginHandler,
		locker: common.NewLockerChain(
			common.NewSingleProcessLocker(),
			distributeLock,
		),
	}, nil
}

func (s *Service) createMiddlewaresChain(plugin types.Plugin) (*middleware.Chain, error) {
	loggingMiddlewareConfig := middleware.LoggingMiddlewareConfig{
		Log: s.Logger,
	}
	loggingMiddleware, err := middleware.NewLoggingMiddleware(loggingMiddlewareConfig)
	if err != nil {
		return nil, trace.Wrap(err, "creating logging middleware")
	}

	lockMiddleware, err := middleware.NewLockMiddleware(middleware.LockMiddlewareConfig{
		Locker:     s.locker,
		Log:        s.Logger,
		PluginName: plugin.GetName(),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return middleware.NewMiddlewareChain(
		loggingMiddleware,
		lockMiddleware,
	), nil
}

func (s *Service) createHandlerWithMiddleware(plugin types.Plugin, resourceType string) (common.ResourceHandler, error) {
	handler, err := s.CreatePluginHandler(plugin, s.Config, resourceType)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	chain, err := s.createMiddlewaresChain(plugin)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return chain.Wrap(handler), nil
}

// ListSCIMResources handles a request to list all the appropriate resources
// of a given type.
func (s *Service) ListSCIMResources(ctx context.Context, req *pb.ListSCIMResourcesRequest) (*pb.ResourceList, error) {
	plugin, err := s.authorize(ctx, req.GetTarget())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	handler, err := s.createHandlerWithMiddleware(plugin, req.GetTarget().GetResourceType())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if req.GetPage() == nil {
		req.SetPage(pb.Page_builder{StartIndex: 1, Count: 100}.Build())
	}
	resp, err := handler.ListResources(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for _, r := range resp.GetResources() {
		if err := ensureMetadata(r, req.GetTarget().GetResourceType()); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return resp, nil
}

// GetSCIMResource handles a request to fetch a single, specific instance of a
// resource
func (s *Service) GetSCIMResource(ctx context.Context, req *pb.GetSCIMResourceRequest) (*pb.Resource, error) {
	plugin, err := s.authorize(ctx, req.GetTarget())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	handler, err := s.createHandlerWithMiddleware(plugin, req.GetTarget().GetResourceType())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	resp, err := handler.GetResource(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := ensureMetadata(resp, req.GetTarget().GetResourceType()); err != nil {
		return nil, trace.Wrap(err)
	}
	return resp, nil
}

// CreateSCIMResource handles a request to create a new SCIM-provisioned
// resource
func (s *Service) CreateSCIMResource(ctx context.Context, req *pb.CreateSCIMResourceRequest) (*pb.Resource, error) {
	plugin, err := s.authorize(ctx, req.GetTarget())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	handler, err := s.createHandlerWithMiddleware(plugin, req.GetTarget().GetResourceType())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	resp, err := handler.CreateResource(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := ensureMetadata(resp, req.GetTarget().GetResourceType()); err != nil {
		return nil, trace.Wrap(err)
	}
	return resp, nil
}

// UpdateSCIMResource handles a request to update a single, specific instance of
// a resource. Depending on how the upstream IdP implements the SCIM protocol,
// an "update" may can include deleting the user.
func (s *Service) UpdateSCIMResource(ctx context.Context, req *pb.UpdateSCIMResourceRequest) (*pb.Resource, error) {
	plugin, err := s.authorize(ctx, req.GetTarget())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	handler, err := s.createHandlerWithMiddleware(plugin, req.GetTarget().GetResourceType())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	resp, err := handler.UpdateResource(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := ensureMetadata(resp, req.GetTarget().GetResourceType()); err != nil {
		return nil, trace.Wrap(err)
	}
	return resp, nil
}

// DeleteSCIMResource handles a request to delete a single, specific instance
// of a resource.
func (s *Service) DeleteSCIMResource(ctx context.Context, req *pb.DeleteSCIMResourceRequest) (*emptypb.Empty, error) {
	plugin, err := s.authorize(ctx, req.GetTarget())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	handler, err := s.createHandlerWithMiddleware(plugin, req.GetTarget().GetResourceType())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := handler.DeleteResource(ctx, req); err != nil {
		return nil, trace.Wrap(err)
	}
	return &emptypb.Empty{}, nil
}

// PatchSCIMResource handles a request to patch a single, specific instance of
// a resource using SCIM PATCH operations as per RFC 7644 Section 3.5.2.
func (s *Service) PatchSCIMResource(ctx context.Context, req *pb.PatchSCIMResourceRequest) (*pb.Resource, error) {
	plugin, err := s.authorize(ctx, req.GetTarget())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	handler, err := s.createHandlerWithMiddleware(plugin, req.GetTarget().GetResourceType())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	resp, err := handler.PatchResource(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := ensureMetadata(resp, req.GetTarget().GetResourceType()); err != nil {
		return nil, trace.Wrap(err)
	}
	return resp, nil
}

func ensureMetadata(resource *pb.Resource, rt string) error {
	if len(resource.GetSchemas()) == 0 {
		var schemaID string
		switch rt {
		case "Users":
			schemaID = common.SchemaUserCore
		case "Groups":
			schemaID = common.SchemaGroupCore
		default:
			return trace.BadParameter("unsupported resource type %q", rt)
		}
		resource.SetSchemas(append(resource.GetSchemas(), schemaID))
	}

	if resource.GetMeta().GetLocation() == "" {
		resource.GetMeta().SetLocation(fmt.Sprintf("/%s/%s", rt, resource.GetId()))
	}
	return nil
}
