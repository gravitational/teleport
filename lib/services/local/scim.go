package local

import (
	"context"

	"github.com/gravitational/teleport"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/gravitational/teleport/api/gen/proto/go/teleport/scim/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local/generic"
)

const (
	scimResourcePrefix = "scim"
)

// SCIMService manages Okta resources in the Backend.
type SCIMService struct {
	log     logrus.FieldLogger
	service *generic.ServiceWrapper[*scimv1.ResourceItem]
}

// NewSCIMService creates a new OktaService.
func NewSCIMService(backend backend.Backend) (*SCIMService, error) {
	scimService, err := generic.NewServiceWrapper[*scimv1.ResourceItem](
		backend,
		types.KindSCIMResource,
		scimResourcePrefix,
		marshalSCIMResourceItem,
		unmarshalSCIMResourceItem,
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &SCIMService{
		log:     logrus.WithFields(logrus.Fields{teleport.ComponentKey: "scim:local-service"}),
		service: scimService,
	}, nil
}

func marshalSCIMResourceItem(object *scimv1.ResourceItem, opts ...services.MarshalOption) ([]byte, error) {
	cfg, err := services.CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if !cfg.PreserveResourceID {
		object = proto.Clone(object).(*scimv1.ResourceItem)
		object.Metadata.Revision = ""
	}
	data, err := protojson.Marshal(object)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return data, nil
}

func unmarshalSCIMResourceItem(data []byte, opts ...services.MarshalOption) (*scimv1.ResourceItem, error) {
	if len(data) == 0 {
		return nil, trace.BadParameter("missing data")
	}
	cfg, err := services.CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var obj scimv1.ResourceItem
	if err := protojson.Unmarshal(data, &obj); err != nil {
		return nil, trace.BadParameter(err.Error())
	}
	if cfg.Revision != "" {
		obj.Metadata.Revision = cfg.Revision
	}
	if !cfg.Expires.IsZero() {
		obj.Metadata.Expires = timestamppb.New(cfg.Expires)
	}
	return &obj, nil
}

func (s *SCIMService) ListSCIMUserResources(ctx context.Context, pageSize int64, lastKey string) ([]*scimv1.ResourceItem, string, error) {
	out, nextToken, err := s.service.WithPrefix("resource", "user").ListResources(ctx, int(pageSize), lastKey)
	return out, nextToken, trace.Wrap(err)
}

func (s *SCIMService) GetSCIMUserResource(ctx context.Context, name string) (*scimv1.ResourceItem, error) {
	out, err := s.service.WithPrefix("resource", "user").GetResource(ctx, name)
	return out, trace.Wrap(err)
}

func (s *SCIMService) CreateSCIMUserResource(ctx context.Context, item *scimv1.ResourceItem) (*scimv1.ResourceItem, error) {
	out, err := s.service.WithPrefix("resource", "user").CreateResource(ctx, item)
	return out, trace.Wrap(err)
}

func (s *SCIMService) UpdateSCIMUserResource(ctx context.Context, item *scimv1.ResourceItem) (*scimv1.ResourceItem, error) {
	out, err := s.service.WithPrefix("resource", "user").ConditionalUpdateResource(ctx, item)
	return out, trace.Wrap(err)
}

func (s *SCIMService) UpsertSCIMUserResource(ctx context.Context, item *scimv1.ResourceItem) (*scimv1.ResourceItem, error) {
	r, err := s.service.WithPrefix("resource", "user").UpsertResource(ctx, item)
	return r, trace.Wrap(err)
}

func (s *SCIMService) DeleteSCIMUserResource(ctx context.Context, name string) error {
	err := s.service.WithPrefix("resource", "user").DeleteResource(ctx, name)
	return trace.Wrap(err)
}

func (s *SCIMService) DeleteAllSCIMUserResources(ctx context.Context) error {
	err := s.service.WithPrefix("resource", "user").DeleteAllResources(ctx)
	return trace.Wrap(err)
}
