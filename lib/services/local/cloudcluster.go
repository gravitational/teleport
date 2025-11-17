package local

import (
	"context"

	cloudclusterv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/cloudcluster/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local/generic"
	"github.com/gravitational/trace"
)

const (
	cloudClusterPrefix = "cloud_cluster"
)

type CloudClusterService struct {
	service *generic.ServiceWrapper[*cloudclusterv1.CloudCluster]
}

// NewCloudClusterService creates a new CloudClusterService.
func NewCloudClusterService(b backend.Backend) (*CloudClusterService, error) {
	service, err := generic.NewServiceWrapper(
		generic.ServiceConfig[*cloudclusterv1.CloudCluster]{
			Backend:       b,
			ResourceKind:  types.KindCloudCluster,
			BackendPrefix: backend.NewKey(cloudClusterPrefix),
			MarshalFunc:   services.MarshalCloudCluster,
			UnmarshalFunc: services.UnmarshalCloudCluster,
		})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &CloudClusterService{service: service}, nil
}

func (s *CloudClusterService) ListCloudClusters(ctx context.Context, pagesize int, lastKey string) ([]*cloudclusterv1.CloudCluster, string, error) {
	r, nextToken, err := s.service.ListResources(ctx, pagesize, lastKey)
	return r, nextToken, trace.Wrap(err)
}

func (s *CloudClusterService) GetCloudCluster(ctx context.Context, name string) (*cloudclusterv1.CloudCluster, error) {
	r, err := s.service.GetResource(ctx, name)
	return r, trace.Wrap(err)
}

func (s *CloudClusterService) CreateCloudCluster(ctx context.Context, cloudCluster *cloudclusterv1.CloudCluster) (*cloudclusterv1.CloudCluster, error) {
	r, err := s.service.CreateResource(ctx, cloudCluster)
	return r, trace.Wrap(err)
}

func (s *CloudClusterService) UpdateCloudCluster(ctx context.Context, cloudCluster *cloudclusterv1.CloudCluster) (*cloudclusterv1.CloudCluster, error) {
	r, err := s.service.ConditionalUpdateResource(ctx, cloudCluster)
	return r, trace.Wrap(err)
}

func (s *CloudClusterService) UpsertCloudCluster(ctx context.Context, cloudCluster *cloudclusterv1.CloudCluster) (*cloudclusterv1.CloudCluster, error) {
	r, err := s.service.UpsertResource(ctx, cloudCluster)
	return r, trace.Wrap(err)
}

func (s *CloudClusterService) DeleteCloudCluster(ctx context.Context, name string) error {
	err := s.service.DeleteResource(ctx, name)
	return trace.Wrap(err)
}

func (s *CloudClusterService) DeleteAllCloudClusters(ctx context.Context) error {
	err := s.service.DeleteAllResources(ctx)
	return trace.Wrap(err)
}
