package pluginsv1

import (
	"context"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/gravitational/teleport/api/defaults"
	pluginspb "github.com/gravitational/teleport/api/gen/proto/go/teleport/plugins/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/plugins"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/services"
)

// ServiceConfig holds configuration options for the plugins gRPC service.
type ServiceConfig struct {
	Authorizer     auth.Authorizer
	Exchangers     *plugins.ExchangerSet
	BackendService services.Plugins
}

// CheckAndSetDefaults checks config for validity.
func (cfg *ServiceConfig) CheckAndSetDefaults() error {
	if cfg.Authorizer == nil {
		return trace.BadParameter("authorizer must be set")
	}
	if cfg.Exchangers == nil {
		return trace.BadParameter("exchangers must be set")
	}
	if cfg.BackendService == nil {
		return trace.BadParameter("backendService must be set")
	}
	return nil
}

// Service implements pluginspb.PluginServiceServer.
type Service struct {
	pluginspb.UnimplementedPluginServiceServer

	authorizer     auth.Authorizer
	exchangers     *plugins.ExchangerSet
	backendService services.Plugins
}

var _ pluginspb.PluginServiceServer = (*Service)(nil)

// NewService creates a new plugins service from the given config.
func NewService(cfg ServiceConfig) (*Service, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, err
	}
	return &Service{
		authorizer:     cfg.Authorizer,
		exchangers:     cfg.Exchangers,
		backendService: cfg.BackendService,
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
	bootstrapCreds := req.BootstrapCredentials
	if bootstrapCreds == nil {
		return nil, trace.BadParameter("BootstrapCredentials must be set")
	}

	authCodeCreds := bootstrapCreds.GetOauth2AuthorizationCode()
	if authCodeCreds == nil {
		return nil, trace.BadParameter("unknown type of bootstrap credentials received")
	}

	exchanger, err := s.exchangers.GetExchanger(plugin)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	creds, err := exchanger.Exchange(ctx, authCodeCreds.AuthorizationCode, authCodeCreds.RedirectUri)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = plugin.SetCredentials(&types.PluginCredentialsV1{
		Credentials: &types.PluginCredentialsV1_Oauth2AccessToken{
			Oauth2AccessToken: &types.PluginOAuth2AccessTokenCredentials{
				AccessToken:  creds.AccessToken,
				RefreshToken: creds.RefreshToken,
				Expires:      creds.ExpiresAt,
			},
		},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.backendService.CreatePlugin(ctx, req.Plugin); err != nil {
		return nil, trace.Wrap(err)
	}
	return &emptypb.Empty{}, nil
}

// GetPlugin returns a plugin instance by name.
func (s *Service) GetPlugin(ctx context.Context, req *pluginspb.GetPluginRequest) (*types.PluginV1, error) {
	if err := s.authorizeVerbs(ctx, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}

	const withSecrets = false
	plugin, err := s.backendService.GetPlugin(ctx, req.Name, withSecrets)
	if err != nil {
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

func (s *Service) authorizeVerbs(ctx context.Context, verbs ...string) error {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	ruleCtx := &services.Context{
		User: authCtx.User,
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
