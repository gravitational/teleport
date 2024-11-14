package pluginsv1

import (
	"context"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/emptypb"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	pluginspb "github.com/gravitational/teleport/api/gen/proto/go/teleport/plugins/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/e/lib/jamf"
	"github.com/gravitational/teleport/e/lib/plugins"
	eteleport "github.com/gravitational/teleport/e/lib/teleport"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
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
		types.PluginTypeGitlab,
		types.PluginTypeEntraID,
		types.PluginTypeDatadog,
		types.PluginTypeAWSIdentityCenter,
		types.PluginTypeMSTeams,
	}
}

// ServiceConfig holds configuration options for the plugins gRPC service.
type ServiceConfig struct {
	Authorizer                     authz.Authorizer
	AuthServer                     *auth.Server
	PluginAuthorizers              *plugins.AuthorizerSet
	PluginService                  services.Plugins
	PluginStaticCredentialsService services.PluginStaticCredentials
	Logger                         *slog.Logger
}

// CheckAndSetDefaults checks config for validity.
func (cfg *ServiceConfig) CheckAndSetDefaults() error {
	if cfg.Authorizer == nil {
		return trace.BadParameter("authorizer must be set")
	}
	if cfg.AuthServer == nil {
		return trace.BadParameter("authServer must be set")
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
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	return nil
}

// Service implements pluginspb.PluginServiceServer.
type Service struct {
	pluginspb.UnimplementedPluginServiceServer

	authorizer                     authz.Authorizer
	authServer                     *auth.Server
	emitter                        apievents.Emitter
	pluginAuthorizers              *plugins.AuthorizerSet
	pluginService                  services.Plugins
	pluginStaticCredentialsService services.PluginStaticCredentials
	logger                         *slog.Logger
	httpClient                     *http.Client
}

// NewService creates a new plugins service from the given config.
func NewService(cfg ServiceConfig) (*Service, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, err
	}
	return &Service{
		authorizer:                     cfg.Authorizer,
		authServer:                     cfg.AuthServer,
		emitter:                        cfg.AuthServer,
		pluginAuthorizers:              cfg.PluginAuthorizers,
		pluginService:                  cfg.PluginService,
		pluginStaticCredentialsService: cfg.PluginStaticCredentialsService,
		logger:                         cfg.Logger,
		httpClient: &http.Client{
			Timeout: 1 * time.Minute,
		},
	}, nil
}

// CreatePlugin creates a new plugin instance.
func (s *Service) CreatePlugin(ctx context.Context, req *pluginspb.CreatePluginRequest) (*emptypb.Empty, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindPlugin, types.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}

	if req.Plugin == nil {
		return nil, trace.BadParameter("missing plugin")
	}
	_, err = s.pluginService.GetPlugin(ctx, req.GetPlugin().GetName(), false /* withSecrets */)
	switch {
	case err == nil:
		return nil, trace.AlreadyExists("plugin %q already exists", req.Plugin.GetName())
	case !trace.IsNotFound(err):
		return nil, trace.Wrap(err)
	default:
		// If the plugin doesn't exist, we'll continue.
	}

	plugin := req.Plugin
	if plugin == nil {
		return nil, trace.BadParameter("Plugin must be set")
	}

	if err := validateEntraTenantID(plugin); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.defaultCleanup(ctx, req.GetPlugin().GetType()); err != nil {
		// needsCleanup will return below if defaultCleanup failed at deleting resource.
		s.logger.WarnContext(ctx, `failed to cleanup resources created by the previous installation of this plugin.
This may cause issues with the current installation`,
			"error",
			err,
		)
	}

	// If the plugin needs cleanup, we won't allow the plugin to be created.
	needsCleanup, _, err := s.needsCleanup(ctx, plugin.GetType())
	// We'll ignore the not found error for now and let the rest of this function produce a more specific error.
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}
	if len(needsCleanup) > 0 {
		return nil, trace.BadParameter("plugin needs to be cleaned up first, please run the plugin cleanup command")
	}

	if err := s.updatePluginWithLiveCredentials(ctx, plugin, req.BootstrapCredentials); err != nil {
		return nil, trace.Wrap(err)
	}

	staticCreds := req.StaticCredentialsList
	staticCredLabels := req.CredentialLabels
	if req.StaticCredentials != nil {
		// For backwards compatibility, if the single StaticCredential value is
		// set then we override the supplied credential list with that single
		// credential
		staticCreds = []*types.PluginStaticCredentialsV1{req.StaticCredentials}

		// Similarly, we need to generate the identifying label set from the
		// single credential, rather than use the supplied label set.
		staticCredLabels = req.StaticCredentials.GetStaticLabels()
		if staticCredLabels == nil {
			staticCredLabels = map[string]string{}
		}

		for k := range staticCredLabels {
			if strings.HasPrefix(k, types.TeleportInternalLabelPrefix) {
				delete(staticCredLabels, k)
			}
		}
	}

	if err := s.updatePluginAndCreateStaticCredentials(ctx, plugin, staticCredLabels, staticCreds); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.pluginService.CreatePlugin(ctx, req.Plugin); err != nil {
		return nil, trace.Wrap(err)
	}

	resource := req.Plugin.WithoutSecrets()
	out, ok := resource.(*types.PluginV1)
	if !ok {
		return nil, trace.BadParameter("unsupported plugin type %T, expected %T", req.Plugin, out)
	}

	if err := s.emitter.EmitAuditEvent(ctx, &apievents.PluginCreate{
		Metadata: apievents.Metadata{
			Type: events.PluginCreateEvent,
			Code: events.PluginCreateCode,
		},
		UserMetadata: authCtx.GetUserMetadata(),
		ResourceMetadata: apievents.ResourceMetadata{
			Name: plugin.GetName(),
		},
		PluginMetadata: apievents.PluginMetadata{
			PluginType:     string(plugin.GetType()),
			HasCredentials: staticCreds != nil,
			PluginData:     s.pluginToProtobufStruct(ctx, out),
		},
		ConnectionMetadata: authz.ConnectionMetadata(ctx),
	}); err != nil {
		s.logger.WarnContext(ctx, "Failed to emit plugin create event", "error", err)
	}

	if s.logger.Enabled(ctx, slog.LevelInfo) {
		spec := utils.CloneProtoMsg(&(plugin.Spec))
		s.logger.InfoContext(ctx, "Plugin created",
			slog.Group("plugin",
				"type", string(plugin.GetType()),
				"name", plugin.GetName(),
				"spec", spec,
			),
		)
	}

	return &emptypb.Empty{}, nil
}

func (s *Service) pluginToProtobufStruct(ctx context.Context, plugin types.Plugin) *apievents.Struct {
	out, err := services.MarshalPlugin(plugin)
	if err != nil {
		s.logger.WarnContext(ctx, "Failed to marshal plugin", "error", err)
		return nil
	}

	var str apievents.Struct
	if err := str.UnmarshalJSON(out); err != nil {
		s.logger.WarnContext(ctx, "Failed to unmarshal plugin", "error", err)
		return nil
	}
	return &str
}

// UpdatePlugin updates the specified plugin instance.
func (s *Service) UpdatePlugin(ctx context.Context, req *pluginspb.UpdatePluginRequest) (*types.PluginV1, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.CheckAccessToKind(types.KindPlugin, types.VerbRead, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}

	oldPlugin, err := s.pluginService.GetPlugin(ctx, req.Plugin.GetName(), true /* withSecrets */)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Don't allow to update the plugin state.
	if err := req.Plugin.SetStatus(oldPlugin.GetStatus()); err != nil {
		return nil, trace.Wrap(err)
	}

	inPlugin, ok := req.GetPlugin().Clone().(*types.PluginV1)
	if !ok {
		return nil, trace.BadParameter("unsupported plugin type %T", req.Plugin)
	}

	if inPlugin.Credentials == nil {
		// If the credentials are not set, we'll copy the existing credentials.
		if err := inPlugin.SetCredentials(oldPlugin.GetCredentials()); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	if err := validateEntraTenantID(inPlugin); err != nil {
		return nil, trace.Wrap(err)
	}

	updatedPlugin, err := s.pluginService.UpdatePlugin(ctx, inPlugin)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if s.logger.Enabled(ctx, slog.LevelInfo) {
		spec := utils.CloneProtoMsg(&(req.Plugin.Spec))
		s.logger.InfoContext(ctx, "Plugin updated",
			slog.Group("plugin",
				"type", string(req.Plugin.GetType()),
				"name", req.Plugin.GetName(),
				"spec", spec,
			),
			slog.Bool("using_existing_credentials", inPlugin.Credentials == nil),
		)
	}

	resource := updatedPlugin.WithoutSecrets()
	out, ok := resource.(*types.PluginV1)
	if !ok {
		return nil, trace.BadParameter("unsupported plugin type %T, expected %T", updatedPlugin, out)
	}

	if err := s.emitter.EmitAuditEvent(ctx, &apievents.PluginUpdate{
		Metadata: apievents.Metadata{
			Type: events.PluginUpdateEvent,
			Code: events.PluginUpdateCode,
		},
		UserMetadata: authCtx.GetUserMetadata(),
		ResourceMetadata: apievents.ResourceMetadata{
			Name: out.GetName(),
		},
		PluginMetadata: apievents.PluginMetadata{
			PluginType:        string(out.GetType()),
			HasCredentials:    inPlugin.Credentials != nil,
			ReusesCredentials: inPlugin.Credentials == nil,
			PluginData:        s.pluginToProtobufStruct(ctx, out),
		},
		ConnectionMetadata: authz.ConnectionMetadata(ctx),
	}); err != nil {
		s.logger.WarnContext(ctx, "Failed to emit plugin update event", "error", err)
	}

	return out, nil
}

// updatePluginWithLiveCredentials will update the plugin with live credentials if needed.
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

// updatePluginAndCreateStaticCredentials will update the plugin with static credentials and create them if needed.
func (s *Service) updatePluginAndCreateStaticCredentials(ctx context.Context, plugin *types.PluginV1, credLabels map[string]string, staticCreds []*types.PluginStaticCredentialsV1) error {
	if len(staticCreds) == 0 {
		return nil
	}

	// Add in a random UUID to the static credentials label set and attach it to
	// both the static credentials and the static credentials reference to ensure
	// that the plugin only reads the static credentials specified here.
	pluginUUID := uuid.NewString()
	if credLabels == nil {
		credLabels = map[string]string{}
	}
	credLabels[eteleport.PluginLabel] = pluginUUID

	// Update the plugin to contain the a CredentialsRef that will select all
	// credentials tagged with `credLabels`
	err := plugin.SetCredentials(&types.PluginCredentialsV1{
		Credentials: &types.PluginCredentialsV1_StaticCredentialsRef{
			StaticCredentialsRef: &types.PluginStaticCredentialsRef{
				Labels: credLabels,
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
		// Pass any existing credentials to NewClient.
		var user, pass, clientID, clientSecret string
		if len(staticCreds) > 0 {
			sc := staticCreds[0]
			user, pass = sc.GetBasicAuth()
			clientID, clientSecret = sc.GetOAuthClientSecret()
		}

		// Creating a client automatically verifies the credentials.
		if _, err := jamf.NewClient(ctx, jamf.ClientOpts{
			Logger:       s.logger,
			HTTPClient:   s.httpClient,
			APIURL:       plugin.Spec.GetJamf().JamfSpec.ApiEndpoint,
			Username:     user,
			Password:     pass,
			ClientID:     clientID,
			ClientSecret: clientSecret,
		}); err != nil {
			s.logger.WarnContext(ctx, "failed to verify Jamf endpoint and credentials", "error", err)
			return trace.Errorf("failed to verify Jamf endpoint and credentials")
		}
	}

	for _, cred := range staticCreds {
		// Fetch existing credential label set, creating if nil...
		instanceLabels := cred.GetStaticLabels()
		if instanceLabels == nil {
			instanceLabels = make(map[string]string)
		}

		// Merge this credential's existing labels with the supplied credential-
		// identifying label set
		for k, v := range credLabels {
			instanceLabels[k] = v
		}
		cred.SetStaticLabels(instanceLabels)

		// And finally, write the cred to the back-end
		err := s.pluginStaticCredentialsService.CreatePluginStaticCredentials(ctx, cred)
		if err != nil {
			return trace.Wrap(err, "creating static credential")
		}
	}

	return nil
}

// GetPlugin returns a plugin instance by name.
func (s *Service) GetPlugin(ctx context.Context, req *pluginspb.GetPluginRequest) (*types.PluginV1, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

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

		if authErr := authCtx.CheckAccessToKind(types.KindPlugin, types.VerbList); authErr != nil {
			// Generate a fake auth error equivalent to a real one
			// using a dummy context which does not have user info, so will never have permissions
			fakeAuthError := authCtx.CheckAccessToKind(types.KindPlugin, readVerb)
			s.logger.ErrorContext(ctx, "user does not have access to retrieve plugin", "error", err)
			return nil, fakeAuthError
		}

		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToResource(plugin, readVerb); err != nil {
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
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindPlugin, types.VerbRead); err != nil {
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
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindPlugin, types.VerbDelete); err != nil {
		return nil, trace.Wrap(err)
	}

	// Get the plugin so that we can find any static credentials references and delete them.
	plugin, err := s.pluginService.GetPlugin(ctx, req.Name, true)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := s.checkResourceCleanupPermissions(ctx, plugin.GetType()); err != nil {
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

	resource := plugin.WithoutSecrets()
	out, ok := resource.(*types.PluginV1)
	if !ok {
		return nil, trace.BadParameter("unsupported plugin type %T, expected %T", plugin, out)
	}
	if err := s.emitter.EmitAuditEvent(ctx, &apievents.PluginDelete{
		Metadata: apievents.Metadata{
			Type: events.PluginDeleteEvent,
			Code: events.PluginDeleteCode,
		},
		UserMetadata: authCtx.GetUserMetadata(),
		ResourceMetadata: apievents.ResourceMetadata{
			Name: req.Name,
		},
		PluginMetadata: apievents.PluginMetadata{
			PluginType:     string(plugin.GetType()),
			HasCredentials: staticCredsRef != nil,
			PluginData:     s.pluginToProtobufStruct(ctx, out),
		},
		ConnectionMetadata: authz.ConnectionMetadata(ctx),
	}); err != nil {
		s.logger.WarnContext(ctx, "Failed to emit plugin delete event", "error", err)
	}
	s.logger.InfoContext(ctx, "Plugin deleted", "name", req.Name)

	// Plugin such as Okta does not currently cleanup resource on Delete. So we just pick
	// PluginTypeAWSIdentityCenter plugin.
	if req.Name == types.PluginTypeAWSIdentityCenter {
		if err := s.cleanupAWSIdentityCenter(ctx, out); err != nil {
			s.logger.WarnContext(ctx, "failed to cleanup resources created by the Identity Center plugin.", "error", err)
			return nil, trace.Wrap(err)
		}
	}

	return &emptypb.Empty{}, nil
}

// SetPluginCredentials sets the credentials for the given plugin.
func (s *Service) SetPluginCredentials(ctx context.Context, req *pluginspb.SetPluginCredentialsRequest) (*emptypb.Empty, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindPlugin, types.VerbRead, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := s.pluginService.SetPluginCredentials(ctx, req.Name, req.Credentials); err != nil {
		return nil, trace.Wrap(err)
	}
	return &emptypb.Empty{}, nil
}

// SetPluginStatus sets the status for the given plugin.
func (s *Service) SetPluginStatus(ctx context.Context, req *pluginspb.SetPluginStatusRequest) (*emptypb.Empty, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindPlugin, types.VerbUpdate); err != nil {
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
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindPlugin, types.VerbCreate); err != nil {
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
	}

	s.logger.WarnContext(ctx, "Plugin static credential retrieval denied", "labels", req.Labels, "user", authCtx.Identity.GetIdentity().Username)

	// This has some other role, so deny access.
	return nil, trace.AccessDenied("access denied")
}

// NeedsCleanup will indicate whether artifacts from a previous instance of the plugin needs to be cleaned up before a
// new one can be created.
func (s *Service) NeedsCleanup(ctx context.Context, req *pluginspb.NeedsCleanupRequest) (*pluginspb.NeedsCleanupResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// We'll allow this if the user has access to create plugins.
	if err := authCtx.CheckAccessToKind(types.KindPlugin, types.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}

	needsCleanup, active, err := s.needsCleanup(ctx, types.PluginType(req.GetType()))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &pluginspb.NeedsCleanupResponse{
		NeedsCleanup:       len(needsCleanup) > 0,
		ResourcesToCleanup: needsCleanup,
		PluginActive:       active,
	}, nil
}

// Cleanup will clean up the artifactes from a previous instance of the given plugin.
func (s *Service) Cleanup(ctx context.Context, req *pluginspb.CleanupRequest) (*emptypb.Empty, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// We'll allow this if the user has access to create plugins.
	if err := authCtx.CheckAccessToKind(types.KindPlugin, types.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.cleanup(ctx, types.PluginType(req.Type), nil /* plugin */); err != nil {
		return nil, trace.Wrap(err, "cleanup of plugin %s failed", req.Type)
	}

	return &emptypb.Empty{}, nil
}

// needsCleanup will return the resources that need to be cleaned up. It will also return true if the plugin
// is currently active.
func (s *Service) needsCleanup(ctx context.Context, pluginType types.PluginType) ([]*types.ResourceID, bool, error) {
	switch pluginType {
	case types.PluginTypeOkta:
		return s.oktaNeedsCleanup(ctx)
	case types.PluginTypeAWSIdentityCenter:
		return s.identityCenterNeedsCleanup(ctx)
	}

	// If this plugin is valid and we don't need to clean it up, just return.
	if slices.Contains(types.AllPluginTypes, pluginType) {
		active, err := s.isPluginOfTypeActive(ctx, pluginType)
		if err != nil {
			return nil, false, trace.Wrap(err)
		}
		return nil, active, nil
	}

	return nil, false, trace.NotFound("Teleport doesn't support plugin type %s", pluginType)
}

// cleanup will cleanup the resources necessary to start the plugin
func (s *Service) cleanup(ctx context.Context, pluginType types.PluginType, plugin *types.PluginV1) error {
	switch pluginType {
	case types.PluginTypeOkta:
		return s.cleanupOkta(ctx)
	case types.PluginTypeAWSIdentityCenter:
		return s.cleanupAWSIdentityCenter(ctx, plugin)
	}

	// If this plugin is valid and we don't need to clean it up, just return.
	if slices.Contains(types.AllPluginTypes, pluginType) {
		return nil
	}

	return trace.NotFound("Teleport doesn't support plugin type %s", pluginType)
}

// isPluginOfTypeActive will return true if a plugin of the given type is active.
func (s *Service) isPluginOfTypeActive(ctx context.Context, pluginType types.PluginType) (bool, error) {
	pageToken := ""
	for {
		var plugins []types.Plugin
		var err error
		plugins, pageToken, err = s.pluginService.ListPlugins(ctx, apidefaults.DefaultChunkSize, pageToken, false /* withSecrets */)
		if err != nil {
			return false, trace.Wrap(err)
		}

		for _, plugin := range plugins {
			if plugin.GetType() == pluginType {
				return true, nil
			}
		}

		if pageToken == "" {
			break
		}
	}

	return false, nil
}

// defaultCleanup runs cleanup without explicit user confirmation.
func (s *Service) defaultCleanup(ctx context.Context, pluginType types.PluginType) error {
	if pluginType == types.PluginTypeAWSIdentityCenter {
		if err := s.checkResourceCleanupPermissions(ctx, pluginType); err != nil {
			return trace.Wrap(err)
		}
		return trace.Wrap(s.cleanupAWSIdentityCenter(ctx, nil /* plugin */))
	}

	return nil
}

// checkResourceCleanupPermissions checks delete access to plugin created resources that need cleanup.
func (s *Service) checkResourceCleanupPermissions(ctx context.Context, pluginType types.PluginType) error {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	if pluginType == types.PluginTypeAWSIdentityCenter {
		return trace.Wrap(checkIdentityCenterResourceDeleteAccess(authCtx))
	}
	return nil
}

func isEntraIDPlugin(plugin *types.PluginV1) bool {
	return plugin.Spec.GetEntraId() != nil
}

func validateEntraTenantID(plugin *types.PluginV1) error {
	if !isEntraIDPlugin(plugin) {
		return nil
	}

	if plugin.Spec.GetEntraId().SyncSettings == nil || plugin.Spec.GetEntraId().SyncSettings.TenantId != "" {
		return nil
	}
	return trace.BadParameter("field Spec.EntraId.SyncSettings.TenantId must be present")
}
