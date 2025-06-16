package service

import (
	"context"
	"fmt"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/emptypb"

	pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/scim/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/scim/service/common"
	"github.com/gravitational/teleport/e/lib/scim/service/provider"
)

// Service contact point for all SCIM requests, before farming them out to resource-
// and IdP-specific handlers.
type Service struct {
	pb.UnimplementedSCIMServiceServer
	common.Config
	CreateHandlerForPlugin func(plugin types.Plugin, config common.Config, resourceType string) (common.ResourceHandler, error)
}

// NewService creates and configures a new SCIM service
func NewService(cfg *common.Config) (*Service, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &Service{
		Config:                 *cfg,
		CreateHandlerForPlugin: provider.CreateHandlerForPlugin,
	}, nil
}

// ListSCIMResources handles a request to list all the appropriate resources
// of a given type.
func (s *Service) ListSCIMResources(ctx context.Context, req *pb.ListSCIMResourcesRequest) (*pb.ResourceList, error) {
	plugin, err := s.authorize(ctx, req.GetTarget())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	handler, err := s.CreateHandlerForPlugin(plugin, s.Config, req.GetTarget().GetResourceType())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if req.GetPage() == nil {
		req.Page = &pb.Page{StartIndex: 1, Count: 100}
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
	handler, err := s.CreateHandlerForPlugin(plugin, s.Config, req.GetTarget().GetResourceType())
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
	handler, err := s.CreateHandlerForPlugin(plugin, s.Config, req.GetTarget().GetResourceType())
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
	handler, err := s.CreateHandlerForPlugin(plugin, s.Config, req.GetTarget().GetResourceType())
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
	handler, err := s.CreateHandlerForPlugin(plugin, s.Config, req.GetTarget().GetResourceType())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := handler.DeleteResource(ctx, req); err != nil {
		return nil, trace.Wrap(err)
	}
	return &emptypb.Empty{}, nil
}

func ensureMetadata(resource *pb.Resource, rt string) error {
	if len(resource.Schemas) == 0 {
		var schemaID string
		switch rt {
		case "Users":
			schemaID = common.SchemaUserCore
		case "Groups":
			schemaID = common.SchemaGroupCore
		default:
			return trace.BadParameter("unsupported resource type %q", rt)
		}
		resource.Schemas = append(resource.Schemas, schemaID)
	}

	if resource.GetMeta().GetLocation() == "" {
		resource.Meta.Location = fmt.Sprintf("/%s/%s", rt, resource.GetId())
	}
	return nil
}
