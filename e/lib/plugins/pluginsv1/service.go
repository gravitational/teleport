package pluginsv1

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"google.golang.org/protobuf/types/known/emptypb"

	pluginspb "github.com/gravitational/teleport/api/gen/proto/go/teleport/plugins/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/jamf"
	"github.com/gravitational/teleport/e/lib/plugins"
	"github.com/gravitational/teleport/e/lib/teleport"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/services"
)

const (
	assistCredentialName = "openai-default"
)

// getStaticPlugins returns the list of integrations that use an API key
// (or some other static-secret based, non-OpenID-workflow method) to
// authenticate with their remote services. These are considered
// "always available" for installation as they do not require any
// system-level configuration in order to interact with their remote
// services, and can be created dynamically just from resources in the
// cluster's back-end data store.
func getStaticPlugins() []types.PluginType {
	return []types.PluginType{
		types.PluginTypeDiscord,
		types.PluginTypeOkta,
		types.PluginTypeOpsgenie,
		types.PluginTypePagerDuty,
		types.PluginTypeJamf,
		types.PluginTypeJira,
		types.PluginTypeMattermost,
		types.PluginTypeServiceNow,
	}
}

// ServiceConfig holds configuration options for the plugins gRPC service.
type ServiceConfig struct {
	Authorizer                     authz.Authorizer
	PluginAuthorizers              *plugins.AuthorizerSet
	PluginService                  services.Plugins
	PluginStaticCredentialsService services.PluginStaticCredentials
	Log                            *logrus.Entry
}

// CheckAndSetDefaults checks config for validity.
func (cfg *ServiceConfig) CheckAndSetDefaults() error {
	if cfg.Authorizer == nil {
		return trace.BadParameter("authorizer must be set")
	}
	if cfg.PluginAuthorizers == nil {
		return trace.BadParameter("pluginAuthorizers must be set")
	}
	if cfg.PluginService == nil {
		return trace.BadParameter("pluginService must be set")
	}
	if cfg.PluginStaticCredentialsService == nil {
		return trace.BadParameter("pluginStaticCredentialService must be set")
	}
	if cfg.Log == nil {
		cfg.Log = logrus.NewEntry(logrus.StandardLogger())
	}
	return nil
}

// Service implements pluginspb.PluginServiceServer.
type Service struct {
	pluginspb.UnimplementedPluginServiceServer

	authorizer                     authz.Authorizer
	pluginAuthorizers              *plugins.AuthorizerSet
	pluginService                  services.Plugins
	pluginStaticCredentialsService services.PluginStaticCredentials
	log                            *logrus.Entry
	httpClient                     *http.Client
}

// NewService creates a new plugins service from the given config.
func NewService(cfg ServiceConfig) (*Service, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, err
	}
	return &Service{
		authorizer:                     cfg.Authorizer,
		pluginAuthorizers:              cfg.PluginAuthorizers,
		pluginService:                  cfg.PluginService,
		pluginStaticCredentialsService: cfg.PluginStaticCredentialsService,
		log:                            cfg.Log,
		httpClient: &http.Client{
			Timeout: 1 * time.Minute,
		},
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

	if err := s.updatePluginAndCreateStaticCredentials(ctx, plugin, req.StaticCredentials); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.pluginService.CreatePlugin(ctx, req.Plugin); err != nil {
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

// updatePluginAndCreateStaticCredetials will update the plugin with static credentials and create them if needed.
func (s *Service) updatePluginAndCreateStaticCredentials(ctx context.Context, plugin *types.PluginV1, staticCreds *types.PluginStaticCredentialsV1) error {
	if staticCreds == nil {
		return nil
	}

	// Add in a random UUID to the static credentials and attach it to both the static credentials
	// and the static credentials reference to ensure that the plugin only reads the static credentials
	// specified here.
	pluginUUID := uuid.NewString()
	labels := staticCreds.GetStaticLabels()

	// Create if nil, we add keys to it below.
	if labels == nil {
		labels = make(map[string]string)
	}

	// Make sure that we remove any teleport internal labels.
	for k := range labels {
		if strings.HasPrefix(k, types.TeleportInternalLabelPrefix) {
			delete(labels, k)
		}
	}
	labels[teleport.PluginLabel] = pluginUUID
	staticCreds.SetStaticLabels(labels)

	err := plugin.SetCredentials(&types.PluginCredentialsV1{
		Credentials: &types.PluginCredentialsV1_StaticCredentialsRef{
			StaticCredentialsRef: &types.PluginStaticCredentialsRef{
				Labels: labels,
			},
		},
	})
	if err != nil {
		return trace.Wrap(err)
	}

	// Before adding any static credentials, make sure the plugin is valid.
	if err := plugin.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	// Verify Jamf API endpoint and credentials.
	if plugin.GetType() == types.PluginTypeJamf {
		user, pass := staticCreds.GetBasicAuth()

		// Creating a client automatically verifies the credentials.
		if _, err := jamf.NewClient(ctx, jamf.ClientOpts{
			Logger:     s.log,
			HTTPClient: s.httpClient,
			APIURL:     plugin.Spec.GetJamf().JamfSpec.ApiEndpoint,
			Username:   user,
			Password:   pass,
		}); err != nil {
			return trace.Wrap(err, "verifying Jamf endpoint and credentials")
		}
	}
	return trace.Wrap(s.pluginStaticCredentialsService.CreatePluginStaticCredentials(ctx, staticCreds))
}

// GetPlugin returns a plugin instance by name.
func (s *Service) GetPlugin(ctx context.Context, req *pluginspb.GetPluginRequest) (*types.PluginV1, error) {
	readVerb := types.VerbReadNoSecrets

	if req.WithSecrets {
		readVerb = types.VerbRead
	}

	plugin, err := s.pluginService.GetPlugin(ctx, req.Name, req.WithSecrets)
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
	results, nextKey, err := s.pluginService.ListPlugins(ctx, int(req.PageSize), req.StartKey, withSecrets)
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

// DeletePlugin removes the specified plugin instance and any associated static credentials.
func (s *Service) DeletePlugin(ctx context.Context, req *pluginspb.DeletePluginRequest) (*emptypb.Empty, error) {
	if err := s.authorizeVerbs(ctx, types.VerbDelete); err != nil {
		return nil, trace.Wrap(err)
	}

	// Get the plugin so that we can find any static credentials references and delete them.
	plugin, err := s.pluginService.GetPlugin(ctx, req.Name, true)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	staticCredsRef := plugin.GetCredentials().GetStaticCredentialsRef()
	if staticCredsRef != nil {
		allStaticCreds, err := s.pluginStaticCredentialsService.GetPluginStaticCredentialsByLabels(ctx, staticCredsRef.Labels)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		for _, cred := range allStaticCreds {
			if err := s.pluginStaticCredentialsService.DeletePluginStaticCredentials(ctx, cred.GetName()); err != nil {
				return nil, trace.Wrap(err)
			}
		}
	}

	if err := s.pluginService.DeletePlugin(ctx, req.Name); err != nil {
		return nil, trace.Wrap(err)
	}
	return &emptypb.Empty{}, nil
}

// SetPluginCredentials sets the credentials for the given plugin.
func (s *Service) SetPluginCredentials(ctx context.Context, req *pluginspb.SetPluginCredentialsRequest) (*emptypb.Empty, error) {
	if err := s.authorizeVerbs(ctx, types.VerbRead, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := s.pluginService.SetPluginCredentials(ctx, req.Name, req.Credentials); err != nil {
		return nil, trace.Wrap(err)
	}
	return &emptypb.Empty{}, nil
}

// SetPluginStatus sets the status for the given plugin.
func (s *Service) SetPluginStatus(ctx context.Context, req *pluginspb.SetPluginStatusRequest) (*emptypb.Empty, error) {
	if err := s.authorizeVerbs(ctx, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := s.pluginService.SetPluginStatus(ctx, req.Name, req.Status); err != nil {
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

	staticPlugins := getStaticPlugins()

	resp := &pluginspb.GetAvailablePluginTypesResponse{
		PluginTypes: make([]*pluginspb.PluginType, 0,
			len(s.pluginAuthorizers.Authorizers)+len(staticPlugins)),
	}

	for _, typ := range staticPlugins {
		resp.PluginTypes = append(resp.PluginTypes, &pluginspb.PluginType{Type: string(typ)})
	}

	for typ, a := range s.pluginAuthorizers.Authorizers {
		resp.PluginTypes = append(resp.PluginTypes, &pluginspb.PluginType{
			Type:          string(typ),
			OauthClientId: a.ClientID,
		})
	}

	return resp, nil
}

// SearchPluginStaticCredentials returns static credentials that are searched for. Only accessible by RoleAdmin and,
// in the case of Teleport Assist, RoleProxy.
func (s *Service) SearchPluginStaticCredentials(ctx context.Context, req *pluginspb.SearchPluginStaticCredentialsRequest) (*pluginspb.SearchPluginStaticCredentialsResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch {
	case authz.HasBuiltinRole(*authCtx, string(types.RoleAdmin)):
		// RoleAdmin is allowed to retrieve plugin static credentials.
		credentials, err := s.pluginStaticCredentialsService.GetPluginStaticCredentialsByLabels(ctx, req.Labels)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		credentialsV1 := make([]*types.PluginStaticCredentialsV1, len(credentials))
		for i, credential := range credentials {
			credentialV1, ok := credential.(*types.PluginStaticCredentialsV1)
			if !ok {
				return nil, trace.BadParameter("expected *types.PluginStaticCredentialsV1, got %T", credential)
			}

			credentialsV1[i] = credentialV1
		}

		return &pluginspb.SearchPluginStaticCredentialsResponse{
			Credentials: credentialsV1,
		}, nil
	case authz.HasBuiltinRole(*authCtx, string(types.RoleProxy)):
		// RoleProxy is allowed to retrieve the Teleport assist static credential and nothing else. We'll ignore the
		// request here.
		credential, err := s.pluginStaticCredentialsService.GetPluginStaticCredentials(ctx, assistCredentialName)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		// If the labels don't match the openai-default credential, we'll return access denied.
		if !types.MatchLabels(credential, req.Labels) {
			s.log.Warnf("Proxy supplied labels (%v) for static credentials other than the ones for Teleport Assist", req.Labels)
			return nil, trace.AccessDenied("access denied")
		}

		credentialV1, ok := credential.(*types.PluginStaticCredentialsV1)
		if !ok {
			return nil, trace.BadParameter("expected *types.PluginStaticCredentialsV1, got %T", credential)
		}

		return &pluginspb.SearchPluginStaticCredentialsResponse{
			Credentials: []*types.PluginStaticCredentialsV1{credentialV1},
		}, nil
	}

	s.log.Warnf("Plugin static credential retrieval for labels %v denied for user %q", req.Labels, authCtx.Identity.GetIdentity().Username)

	// This has some other role, so deny access.
	return nil, trace.AccessDenied("access denied")
}

func (s *Service) authorizeVerbs(ctx context.Context, verbs ...string) error {
	_, err := authz.AuthorizeWithVerbs(ctx, s.log, s.authorizer, false /* quiet */, types.KindPlugin, verbs...)
	return trace.Wrap(err)
}

func (s *Service) authorizeVerbsWithResource(ctx context.Context, resource types.Resource, verbs ...string) error {
	_, err := authz.AuthorizeResourceWithVerbs(ctx, s.log, s.authorizer, false /* quiet */, resource, verbs...)
	return trace.Wrap(err)
}
