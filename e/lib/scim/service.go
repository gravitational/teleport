package scim

import (
	"context"

	"github.com/elimity-com/scim/schema"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/gravitational/teleport"
	scimpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/scim/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/modules"
)

const (
	ComponentName     = "scim"
	ContentTypeHeader = "Content-Type"
	ContentType       = "application/scim+json"
	resourceTypeUser  = "User"
	resourceTypeGroup = "Group"
)

// Service implements the GRPC SCIM service back end. It acts as the central
// contact point for all SCIM requests, before farming them out to resource-
// and IdP-specific handlers.
type Service struct {
	scimpb.UnimplementedSCIMServiceServer

	authorizer    authz.Authorizer
	users         UsersService
	plugins       PluginsService
	creds         CredentialsService
	locks         LocksService
	accessLists   AccessListsService
	shimFactories map[types.PluginType]shimFactory
	resourceTypes map[string]resourceTypeHandler
	log           logrus.FieldLogger
	clock         clockwork.Clock
}

// defaultShimFactoryMap is the default set of known compatibility shims
// for the various supported Identity providers
var defaultShimFactoryMap = map[types.PluginType]shimFactory{
	types.PluginTypeOkta: newOktaShim,
}

// Config is the externally-supplied configuration data for the SCIM service.
type Config struct {
	Authorizer         authz.Authorizer
	Log                logrus.FieldLogger
	UsersService       UsersService
	RolesService       RolesService
	PluginsService     PluginsService
	CredentialsService CredentialsService
	AccessListsService AccessListsService
	LocksService       LocksService
	ShimFactories      map[types.PluginType]shimFactory
	Clock              clockwork.Clock
}

func (cfg *Config) CheckAndSetDefaults() error {
	if cfg.Authorizer == nil {
		return trace.BadParameter("missing authorizer")
	}

	if cfg.UsersService == nil {
		return trace.BadParameter("missing user service")
	}

	if cfg.RolesService == nil {
		return trace.BadParameter("missing roles service")
	}

	if cfg.PluginsService == nil {
		return trace.BadParameter("missing plugin service")
	}

	if cfg.LocksService == nil {
		return trace.BadParameter("missing locks service")
	}

	if cfg.AccessListsService == nil {
		return trace.BadParameter("missing access lists service")
	}

	if cfg.ShimFactories == nil {
		cfg.ShimFactories = defaultShimFactoryMap
	}

	if cfg.CredentialsService == nil {
		return trace.BadParameter("missing plugin credentials service")
	}

	if cfg.Log == nil {
		cfg.Log = logrus.StandardLogger()
	}

	if cfg.Clock == nil {
		cfg.Clock = clockwork.NewRealClock()
	}

	return nil
}

// NewService creates and configures a new SCIM service
func NewService(cfg *Config) (*Service, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	log := cfg.Log.WithField(teleport.ComponentKey, ComponentName)
	usersLog := log.WithField("ResourceType", "Users")
	groupsLog := log.WithField("ResourceType", "Groups")

	return &Service{
		authorizer:    cfg.Authorizer,
		users:         cfg.UsersService,
		plugins:       cfg.PluginsService,
		locks:         cfg.LocksService,
		accessLists:   cfg.AccessListsService,
		shimFactories: cfg.ShimFactories,
		creds:         cfg.CredentialsService,
		log:           log,
		clock:         cfg.Clock,

		resourceTypes: map[string]resourceTypeHandler{
			"Users": {
				name:     "User",
				endpoint: "/Users",
				schema:   schema.CoreUserSchema(),
				handler:  &userHandler{users: cfg.UsersService, log: usersLog},
				log:      usersLog,
			},
			"Groups": {
				name:     "Group",
				endpoint: "/Groups",
				schema:   schema.CoreGroupSchema(),
				log:      groupsLog,
				handler: &groupHandler{
					accessLists: cfg.AccessListsService,
					roles:       cfg.RolesService,
					users:       cfg.UsersService,
					clock:       cfg.Clock,
					log:         groupsLog,
				},
			},
		},
	}, nil
}

// compile-time assertion that the service meets the Protobuf-defined GRPC
// service definition
var _ scimpb.SCIMServiceServer = (*Service)(nil)

// makeProviderShim creates a new compatibility shim based on the type of the
// supplied plugin. The resulting compatibility shims are expected to live
// *at most* as long as the current request.
func (s *Service) makeProviderShim(ctx context.Context, pluginID string) (providerShim, error) {
	plugin, err := s.plugins.GetPlugin(ctx, pluginID, true)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	shimFactory, ok := s.shimFactories[plugin.GetType()]
	if !ok {
		return nil, trace.NotFound("unsupported plugin type %q", plugin.GetType())
	}

	shim, err := shimFactory(ctx, plugin, s)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return shim, nil
}

// authorizeGRPCRequest checks that the service has the correct license GRPC call that is forwarding a SCIM
// request to the server comes from a Teleport proxy.
func (s *Service) authorizeGRPCRequest(ctx context.Context) error {
	if !modules.GetModules().Features().IGSEnabled() {
		return trace.NotImplemented("SCIM support requires IGS license")
	}

	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil || !auth.HasBuiltinRole(*authCtx, string(types.RoleProxy)) {
		return trace.AccessDenied("Only Teleport Proxy may issue requests to the SCIM service")
	}

	return nil
}

// ListSCIMResources handles a request to list all of the appropriate resources
// of a given type.
func (s *Service) ListSCIMResources(ctx context.Context, req *scimpb.ListSCIMResourcesRequest) (*scimpb.ResourceList, error) {
	if err := s.authorizeGRPCRequest(ctx); err != nil {
		return nil, trace.Wrap(err)
	}

	shim, err := s.makeProviderShim(ctx, req.Target.PluginId)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := shim.authorizeRequest(ctx, req.Target.Authorization); err != nil {
		return nil, trace.Wrap(err)
	}

	resourceTypeHandler, ok := s.resourceTypes[req.Target.ResourceType]
	if !ok {
		return nil, trace.NotFound(req.Target.ResourceType)
	}

	return resourceTypeHandler.listResources(ctx, shim, req.Filter, req.Page)
}

// GetSCIMResource handles a request to fetch a single, specific instance of a
// resource
func (s *Service) GetSCIMResource(ctx context.Context, req *scimpb.GetSCIMResourceRequest) (*scimpb.Resource, error) {
	if err := s.authorizeGRPCRequest(ctx); err != nil {
		return nil, trace.Wrap(err)
	}

	shim, err := s.makeProviderShim(ctx, req.Target.PluginId)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := shim.authorizeRequest(ctx, req.Target.Authorization); err != nil {
		return nil, trace.Wrap(err)
	}

	resourceTypeHandler, ok := s.resourceTypes[req.Target.ResourceType]
	if !ok {
		return nil, trace.NotFound(req.Target.ResourceType)
	}

	return resourceTypeHandler.getResource(ctx, shim, req.Target)
}

// CreateSCIMResource handles a request to create a new SCIM-provisioned
// resource
func (s *Service) CreateSCIMResource(ctx context.Context, req *scimpb.CreateSCIMResourceRequest) (*scimpb.Resource, error) {
	if err := s.authorizeGRPCRequest(ctx); err != nil {
		return nil, trace.Wrap(err)
	}

	shim, err := s.makeProviderShim(ctx, req.Target.PluginId)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := shim.authorizeRequest(ctx, req.Target.Authorization); err != nil {
		return nil, trace.Wrap(err)
	}

	resourceTypeHandler, ok := s.resourceTypes[req.Target.ResourceType]
	if !ok {
		return nil, trace.NotFound(req.Target.ResourceType)
	}

	return resourceTypeHandler.createResource(ctx, shim, req.Resource)
}

// UpdateSCIMResource handles a request to update a single, specific instance of
// a resource. Depending on how the upstream IdP implements the SCIM protocol,
// an "update" may can include deleting the user.
func (s *Service) UpdateSCIMResource(ctx context.Context, req *scimpb.UpdateSCIMResourceRequest) (*scimpb.Resource, error) {
	if err := s.authorizeGRPCRequest(ctx); err != nil {
		return nil, trace.Wrap(err)
	}

	shim, err := s.makeProviderShim(ctx, req.Target.PluginId)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := shim.authorizeRequest(ctx, req.Target.Authorization); err != nil {
		return nil, trace.Wrap(err)
	}

	resourceTypeHandler, ok := s.resourceTypes[req.Target.ResourceType]
	if !ok {
		return nil, trace.NotFound(req.Target.ResourceType)
	}

	if req.Resource.Id == "" {
		req.Resource.Id = req.Target.ResourceId
	}

	return resourceTypeHandler.updateResource(ctx, shim, req.Resource)
}

func (s *Service) DeleteSCIMResource(ctx context.Context, req *scimpb.DeleteSCIMResourceRequest) (*emptypb.Empty, error) {
	if err := s.authorizeGRPCRequest(ctx); err != nil {
		return nil, trace.Wrap(err)
	}

	shim, err := s.makeProviderShim(ctx, req.Target.PluginId)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := shim.authorizeRequest(ctx, req.Target.Authorization); err != nil {
		return nil, trace.Wrap(err)
	}

	resourceTypeHandler, ok := s.resourceTypes[req.Target.ResourceType]
	if !ok {
		return nil, trace.NotFound(req.Target.ResourceType)
	}

	if err := resourceTypeHandler.deleteResource(ctx, shim, req.Target.ResourceId); err != nil {
		return nil, trace.Wrap(err)
	}

	return &emptypb.Empty{}, nil
}
