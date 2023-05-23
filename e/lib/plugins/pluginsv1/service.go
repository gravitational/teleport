package pluginsv1

import (
	"context"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/gravitational/teleport/api/defaults"
	pluginspb "github.com/gravitational/teleport/api/gen/proto/go/teleport/plugins/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/plugins"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/services"
)

// ServiceConfig holds configuration options for the plugins gRPC service.
type ServiceConfig struct {
	Authorizer        authz.Authorizer
	PluginAuthorizers *plugins.AuthorizerSet
	BackendService    services.Plugins
	Log               *logrus.Entry
}

// CheckAndSetDefaults checks config for validity.
func (cfg *ServiceConfig) CheckAndSetDefaults() error {
	if cfg.Authorizer == nil {
		return trace.BadParameter("authorizer must be set")
	}
	if cfg.PluginAuthorizers == nil {
		return trace.BadParameter("pluginAuthorizers must be set")
	}
	if cfg.BackendService == nil {
		return trace.BadParameter("backendService must be set")
	}
	if cfg.Log == nil {
		cfg.Log = logrus.NewEntry(logrus.StandardLogger())
	}
	return nil
}

// Service implements pluginspb.PluginServiceServer.
type Service struct {
	pluginspb.UnimplementedPluginServiceServer

	authorizer        authz.Authorizer
	pluginAuthorizers *plugins.AuthorizerSet
	backendService    services.Plugins
	log               *logrus.Entry
}

var _ pluginspb.PluginServiceServer = (*Service)(nil)

// NewService creates a new plugins service from the given config.
func NewService(cfg ServiceConfig) (*Service, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, err
	}
	return &Service{
		authorizer:        cfg.Authorizer,
		pluginAuthorizers: cfg.PluginAuthorizers,
		backendService:    cfg.BackendService,
		log:               cfg.Log,
	}, nil
}

// CreatePlugin creates a new plugin instance.
func (s *Service) CreatePlugin(ctx context.Context, req *pluginspb.CreatePluginRequest) (*emptypb.Empty, error) {
	if err := s.authorizeVerbs(ctx, types.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}

	plugin := req.Plugin
	if plugin == nil {
		return nil, trace.BadParameter("Plugin must be set")
	}

	if err := s.updatePluginWithLiveCredentials(ctx, plugin, req.BootstrapCredentials); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.backendService.CreatePlugin(ctx, req.Plugin); err != nil {
		return nil, trace.Wrap(err)
	}
	return &emptypb.Empty{}, nil
}

// updatePluginWithLiveCredetials will update the plugin with live credentials if needed.
func (s *Service) updatePluginWithLiveCredentials(ctx context.Context, plugin types.Plugin, bootstrapCreds *types.PluginBootstrapCredentialsV1) error {
	if !plugins.NeedsOAuth(plugin) {
		return nil
	}

	if bootstrapCreds == nil {
		return trace.BadParameter("BootstrapCredentials must be set")
	}

	authCodeCreds := bootstrapCreds.GetOauth2AuthorizationCode()
	if authCodeCreds == nil {
		return trace.BadParameter("unknown type of bootstrap credentials received")
	}

	authorizer, err := s.pluginAuthorizers.Get(plugin.GetType())
	if err != nil {
		return trace.Wrap(err)
	}

	creds, err := authorizer.Exchange(ctx, authCodeCreds.AuthorizationCode, authCodeCreds.RedirectUri)
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(plugin.SetCredentials(&types.PluginCredentialsV1{
		Credentials: &types.PluginCredentialsV1_Oauth2AccessToken{
			Oauth2AccessToken: &types.PluginOAuth2AccessTokenCredentials{
				AccessToken:  creds.AccessToken,
				RefreshToken: creds.RefreshToken,
				Expires:      creds.ExpiresAt,
			},
		},
	}))
}

// GetPlugin returns a plugin instance by name.
func (s *Service) GetPlugin(ctx context.Context, req *pluginspb.GetPluginRequest) (*types.PluginV1, error) {
	readVerb := types.VerbReadNoSecrets

	if req.WithSecrets {
		readVerb = types.VerbRead
	}

	plugin, err := s.backendService.GetPlugin(ctx, req.Name, req.WithSecrets)
	if err != nil {
		// If the user has no RBAC to list the plugins,
		// avoid leaking the information on whether the resource exists,
		// and instead of possibly returning a "not found",
		// return an "access denied" error.
		// Log the original error instead.
		if authErr := s.authorizeVerbs(ctx, types.VerbList); authErr != nil {
			// Generate a fake auth error equivalent to a real one
			// using a dummy context which does not have user info, so will never have permissions
			fakeAuthError := s.authorizeVerbs(context.Background(), readVerb)
			s.log.Error(err)
			return nil, fakeAuthError
		}

		return nil, trace.Wrap(err)
	}

	if err := s.authorizeVerbsWithResource(ctx, plugin, readVerb); err != nil {
		return nil, trace.Wrap(err)
	}

	v1, ok := plugin.(*types.PluginV1)
	if !ok {
		return nil, trace.BadParameter("unsupported plugin type %T, expected %T", plugin, v1)
	}
	return v1, nil
}

// ListPlugins returns a paginated view of plugin instances.
func (s *Service) ListPlugins(ctx context.Context, req *pluginspb.ListPluginsRequest) (*pluginspb.ListPluginsResponse, error) {
	if err := s.authorizeVerbs(ctx, types.VerbList, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}

	const withSecrets = false
	results, nextKey, err := s.backendService.ListPlugins(ctx, int(req.PageSize), req.StartKey, withSecrets)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	resultsV1 := make([]*types.PluginV1, 0, len(results))
	for _, plugin := range results {
		v1, ok := plugin.(*types.PluginV1)
		if !ok {
			return nil, trace.BadParameter("unsupported plugin type %T, expected %T", plugin, v1)
		}
		resultsV1 = append(resultsV1, v1)
	}
	return &pluginspb.ListPluginsResponse{
		Plugins: resultsV1,
		NextKey: nextKey,
	}, nil
}

// DeletePlugin removes the specified plugin instance.
func (s *Service) DeletePlugin(ctx context.Context, req *pluginspb.DeletePluginRequest) (*emptypb.Empty, error) {
	if err := s.authorizeVerbs(ctx, types.VerbDelete); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.backendService.DeletePlugin(ctx, req.Name); err != nil {
		return nil, trace.Wrap(err)
	}
	return &emptypb.Empty{}, nil
}

// SetPluginCredentials sets the credentials for the given plugin.
func (s *Service) SetPluginCredentials(ctx context.Context, req *pluginspb.SetPluginCredentialsRequest) (*emptypb.Empty, error) {
	if err := s.authorizeVerbs(ctx, types.VerbRead, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := s.backendService.SetPluginCredentials(ctx, req.Name, req.Credentials); err != nil {
		return nil, trace.Wrap(err)
	}
	return &emptypb.Empty{}, nil
}

// SetPluginStatus sets the status for the given plugin.
func (s *Service) SetPluginStatus(ctx context.Context, req *pluginspb.SetPluginStatusRequest) (*emptypb.Empty, error) {
	if err := s.authorizeVerbs(ctx, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := s.backendService.SetPluginStatus(ctx, req.Name, req.Status); err != nil {
		return nil, trace.Wrap(err)
	}
	return &emptypb.Empty{}, nil
}

// GetAvailablePluginTypes returns the types of plugins
// that the auth server supports onboarding.
func (s *Service) GetAvailablePluginTypes(ctx context.Context, req *pluginspb.GetAvailablePluginTypesRequest) (*pluginspb.GetAvailablePluginTypesResponse, error) {
	if err := s.authorizeVerbs(ctx, types.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}

	resp := &pluginspb.GetAvailablePluginTypesResponse{
		PluginTypes: make([]*pluginspb.PluginType, 0, len(s.pluginAuthorizers.Authorizers)),
	}

	for typ, a := range s.pluginAuthorizers.Authorizers {
		resp.PluginTypes = append(resp.PluginTypes, &pluginspb.PluginType{
			Type:          string(typ),
			OauthClientId: a.ClientID,
		})
	}

	return resp, nil
}

func (s *Service) authorizeVerbs(ctx context.Context, verbs ...string) error {
	return s.authorizeVerbsWithResource(ctx, nil, verbs...)
}

func (s *Service) authorizeVerbsWithResource(ctx context.Context, resource types.Resource, verbs ...string) error {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	ruleCtx := &services.Context{
		User:     authCtx.User,
		Resource: resource,
	}
	errs := make([]error, len(verbs))
	for i, verb := range verbs {
		errs[i] = authCtx.Checker.CheckAccessToRule(ruleCtx, defaults.Namespace, types.KindPlugin, verb, false /* silent */)
	}
	// Convert generic aggregate error to AccessDenied (auth_with_roles also does this).
	if err := trace.NewAggregate(errs...); err != nil {
		return trace.AccessDenied(err.Error())
	}
	return nil
}
