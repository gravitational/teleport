package scim

import (
	"context"
	"log/slog"

	"github.com/elimity-com/scim/schema"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/gravitational/teleport"
	scimpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/scim/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/entitlements"
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
	roles         RolesService
	plugins       PluginsService
	creds         CredentialsService
	locks         LocksService
	accessLists   AccessListsService
	shimFactories map[types.PluginType]shimFactory
	resourceTypes map[string]resourceTypeHandler
	logger        *slog.Logger
	clock         clockwork.Clock
	identity      IdentityService
}

// defaultShimFactoryMap is the default set of known compatibility shims
// for the various supported Identity providers
var defaultShimFactoryMap = map[types.PluginType]shimFactory{
	types.PluginTypeOkta: newOktaShim,
}

// Config is the externally-supplied configuration data for the SCIM service.
type Config struct {
	Authorizer         authz.Authorizer
	Logger             *slog.Logger
	UsersService       UsersService
	RolesService       RolesService
	PluginsService     PluginsService
	CredentialsService CredentialsService
	AccessListsService AccessListsService
	LocksService       LocksService
	ShimFactories      map[types.PluginType]shimFactory
	Clock              clockwork.Clock
	IdentityService    IdentityService
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

	if cfg.IdentityService == nil {
		return trace.BadParameter("missing identity service")
	}

	if cfg.ShimFactories == nil {
		cfg.ShimFactories = defaultShimFactoryMap
	}

	if cfg.CredentialsService == nil {
		return trace.BadParameter("missing plugin credentials service")
	}

	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
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

	logger := cfg.Logger.With(teleport.ComponentKey, ComponentName)
	usersLogger := logger.With("resource_type", "users")
	groupsLogger := logger.With("resource_type", "groups")

	return &Service{
		authorizer:    cfg.Authorizer,
		users:         cfg.UsersService,
		roles:         cfg.RolesService,
		plugins:       cfg.PluginsService,
		locks:         cfg.LocksService,
		identity:      cfg.IdentityService,
		accessLists:   cfg.AccessListsService,
		shimFactories: cfg.ShimFactories,
		creds:         cfg.CredentialsService,
		logger:        logger,
		clock:         cfg.Clock,

		resourceTypes: map[string]resourceTypeHandler{
			"Users": {
				name:     "User",
				endpoint: "/Users",
				schema:   schema.CoreUserSchema(),
				handler: &userHandler{
					users:  cfg.UsersService,
					logger: usersLogger,
				},
				logger: usersLogger,
			},
			"Groups": {
				name:     "Group",
				endpoint: "/Groups",
				schema:   schema.CoreGroupSchema(),
				logger:   groupsLogger,
				handler: &groupHandler{
					accessLists: cfg.AccessListsService,
					roles:       cfg.RolesService,
					users:       cfg.UsersService,
					clock:       cfg.Clock,
					logger:      groupsLogger,
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
func (s *Service) makeProviderShim(ctx context.Context, pluginID string) (providerShim, types.Plugin, error) {
	plugin, err := s.plugins.GetPlugin(ctx, pluginID, true)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	shimFactory, ok := s.shimFactories[plugin.GetType()]
	if !ok {
		return nil, nil, trace.NotFound("unsupported plugin type %q", plugin.GetType())
	}

	shim, err := shimFactory(ctx, plugin, s)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	return shim, plugin, nil
}

// authorizeGRPCRequest checks that the service has the correct license GRPC call that is forwarding a SCIM
// request to the server comes from a Teleport proxy.
func (s *Service) authorizeGRPCRequest(ctx context.Context) error {
	if !modules.GetModules().Features().GetEntitlement(entitlements.OktaSCIM).Enabled {
		return trace.NotImplemented("SCIM support requires Identity license")
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
	shim, err := s.authAndValidateRequest(ctx, req.GetTarget())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	resourceTypeHandler, ok := s.resourceTypes[req.Target.ResourceType]
	if !ok {
		return nil, trace.NotFound(req.Target.ResourceType)
	}

	return resourceTypeHandler.listResources(ctx, shim, req.Filter, req.Page)
}

func (s *Service) authAndValidateRequest(ctx context.Context, target *scimpb.RequestTarget) (providerShim, error) {
	if err := s.authorizeGRPCRequest(ctx); err != nil {
		return nil, trace.Wrap(err)
	}

	shim, plugin, err := s.makeProviderShim(ctx, target.GetPluginId())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := shim.authorizeRequest(ctx, target.GetAuthorization()); err != nil {
		return nil, trace.Wrap(err)
	}
	supported, err := isResourceSupported(plugin, target.GetResourceType())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if !supported {
		return nil, trace.BadParameter("Resource type %q is not supported by the plugin", target.GetResourceType())
	}

	return shim, nil
}

func isResourceSupported(plugin types.Plugin, resourceType string) (bool, error) {
	switch resourceType {
	case resourceTypeUser:
		return true, nil
	case resourceTypeGroup:
		pluginV1, ok := plugin.(*types.PluginV1)
		if !ok {
			return false, trace.BadParameter("Unsupported plugin type %T", plugin)
		}
		return pluginV1.Spec.GetOkta().SyncSettings.SyncAccessLists, nil
	default:
		return true, nil
	}
}

// GetSCIMResource handles a request to fetch a single, specific instance of a
// resource
func (s *Service) GetSCIMResource(ctx context.Context, req *scimpb.GetSCIMResourceRequest) (*scimpb.Resource, error) {
	shim, err := s.authAndValidateRequest(ctx, req.GetTarget())
	if err != nil {
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
	shim, err := s.authAndValidateRequest(ctx, req.GetTarget())
	if err != nil {
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
	shim, err := s.authAndValidateRequest(ctx, req.GetTarget())
	if err != nil {
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
	shim, err := s.authAndValidateRequest(ctx, req.GetTarget())
	if err != nil {
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
