package oktaservice

import (
	"context"
	"log/slog"
	"net/url"

	"github.com/gravitational/trace"

	oktapb "github.com/gravitational/teleport/api/gen/proto/go/teleport/okta/v1"
	pluginspb "github.com/gravitational/teleport/api/gen/proto/go/teleport/plugins/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/okta/api"
	"github.com/gravitational/teleport/e/lib/okta/common/sso"
	"github.com/gravitational/teleport/e/lib/plugins"
	eteleport "github.com/gravitational/teleport/e/lib/teleport"
)

func (s *Service) authorize(ctx context.Context, verb string) error {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := authCtx.CheckAccessToKind(types.KindPlugin, verb); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// ValidateClientCredentials validates the client credentials by making a request to the Okta API.
func (s *Service) ValidateClientCredentials(ctx context.Context, req *oktapb.ValidateClientCredentialsRequest) (*oktapb.ValidateClientCredentialsResponse, error) {
	if err := s.authorize(ctx, types.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}
	params := &createOktaClientParams{
		credsFromReq:     req.GetApiCredentials(),
		oktaOrganization: req.GetOktaOrganizationUrl(),
		scopes:           []string{api.ScopeUserRead},
	}
	oktaClient, err := s.createOktaClient(ctx, params)
	if err != nil {
		return nil, trace.BadParameter("okta credential verification failed: %v", err)
	}
	if _, err = oktaClient.ListUsers(ctx); err != nil {
		return nil, trace.BadParameter("okta credential verification failed: %v", err)
	}
	return &oktapb.ValidateClientCredentialsResponse{}, nil
}

func (s *Service) GetGroups(ctx context.Context, req *oktapb.GetGroupsRequest) (*oktapb.GetGroupsResponse, error) {
	if err := s.authorize(ctx, types.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}
	groups, err := s.fetchAllOktaGroups(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	filtered, err := filterOktaResources(req.GetFilters(), groups)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &oktapb.GetGroupsResponse{
		Groups: toGroups(filtered),
	}, nil
}

func (s *Service) GetApps(ctx context.Context, req *oktapb.GetAppsRequest) (*oktapb.GetAppsResponse, error) {
	if err := s.authorize(ctx, types.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}
	apps, err := s.fetchAllOktaApps(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	filtered, err := filterOktaResources(req.GetFilters(), apps)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &oktapb.GetAppsResponse{
		Apps: toApps(filtered),
	}, nil
}

func validateCreateIntegrationRequest(req *oktapb.CreateIntegrationRequest) error {
	// Plugin can be setup only based on the SSO metadata URL or Okta organization URL.
	if req.GetSsoMetadataUrl() == "" {
		// If SSO metadata URL is not provided, Okta organization URL is required because
		// it can be extracted from the SSO metadata URL.
		if req.GetOktaOrganizationUrl() == "" {
			return trace.BadParameter("missing Okta organization URL")
		}
		u, err := url.Parse(req.GetOktaOrganizationUrl())
		if err != nil {
			return trace.BadParameter("invalid Okta organization URL: %v", err)
		}
		if u.Scheme == "" {
			u.Scheme = "https"
			req.OktaOrganizationUrl = u.String()
		}
	}

	if req.GetApiCredentials() == nil {
		// Credentials are required for access list sync, user sync, and group sync.
		// Otherwise, the plugin will not be able to fetch and sync required data.
		if req.GetEnableUserSync() {
			return trace.BadParameter("Okta API credentials are required for access list sync")
		}
		if req.GetEnableUserSync() {
			return trace.BadParameter("Okta API credentials are required for user sync")
		}
		if req.GetEnableAppGroupSync() {
			return trace.BadParameter("Okta API credentials are required for group sync")
		}
	}
	return nil
}

// CreateIntegration creates a new Okta integration.
// Depending on the request, it may create a new SAML connector or reuse an existing one.
func (s *Service) CreateIntegration(ctx context.Context, req *oktapb.CreateIntegrationRequest) (*oktapb.CreateIntegrationResponse, error) {
	if err := s.authorize(ctx, types.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := validateCreateIntegrationRequest(req); err != nil {
		return nil, trace.Wrap(err, "create integration failed due to invalid request")
	}

	resp, err := s.createIntegration(ctx, req)
	if err != nil {
		s.logger.WarnContext(ctx, "Failed to create Okta integration.", "error", err)
		return nil, trace.Wrap(err)
	}
	log := slog.With(
		"sync_settings", resp.GetPlugin().Spec.GetOkta().SyncSettings,
		"connector", resp.GetConnectorInfo().GetTeleportConnectorName(),
		"okta_app_id", resp.GetConnectorInfo().GetOktaAppId(),
		"okta_app_name", resp.GetConnectorInfo().GetOktaAppId(),
	)
	log.InfoContext(ctx, "Okta integration successfully created.")
	return resp, nil
}

func (s *Service) createIntegration(ctx context.Context, req *oktapb.CreateIntegrationRequest) (*oktapb.CreateIntegrationResponse, error) {
	info, err := s.getOrCreateSAMLConnector(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err, "failed to get or create SAML connector")
	}

	creds, err := getOktaPluginCredentials(req)
	if err != nil {
		return nil, trace.Wrap(err, "failed to get Okta plugin credentials")
	}

	oktaPlugin := createOktaPlugin(req, info)

	createPluginRequest := &pluginspb.CreatePluginRequest{
		Plugin:                oktaPlugin,
		StaticCredentialsList: creds,
		CredentialLabels: map[string]string{
			eteleport.OktaOrgURLLabel: req.GetOktaOrganizationUrl(),
		},
	}

	if _, err = s.pluginService.CreatePlugin(ctx, createPluginRequest); err != nil {
		return nil, trace.Wrap(err, "failed to create Okta plugin")
	}

	return &oktapb.CreateIntegrationResponse{
		Plugin: oktaPlugin,
		ConnectorInfo: &oktapb.ConnectorInfo{
			OktaAppId:             info.OktaAppID,
			OktaAppName:           info.OktaAppName,
			OktaAppLabels:         info.OktaAppLabel,
			TeleportConnectorName: info.Connector.GetName(),
		},
	}, nil
}

func createOktaPlugin(req *oktapb.CreateIntegrationRequest, info *sso.SAMLConnectorInfo) *types.PluginV1 {
	oktaSettings := &types.PluginOktaSettings{
		OrgUrl: info.OktaOrg,
		SyncSettings: &types.PluginOktaSyncSettings{
			SyncUsers:            req.GetEnableUserSync(),
			SyncAccessLists:      req.GetEnableAccessListSync(),
			DisableSyncAppGroups: !req.GetEnableAppGroupSync(),
			SsoConnectorId:       info.Connector.GetName(),
			AppId:                info.OktaAppID,

			GroupFilters:  req.GetAccessListSettings().GetGroupFilters(),
			AppFilters:    req.GetAccessListSettings().GetAppFilters(),
			DefaultOwners: req.GetAccessListSettings().GetDefaultOwner(),
		},
	}

	plugin := &types.PluginV1{
		SubKind: types.PluginSubkindAccess,
		Metadata: types.Metadata{
			Labels: map[string]string{
				plugins.HostedPluginLabel: "true",
			},
			Name: types.PluginTypeOkta,
		},
		Spec: types.PluginSpecV1{
			Settings: &types.PluginSpecV1_Okta{
				Okta: oktaSettings,
			},
		},
	}
	return plugin
}

func (s *Service) UpdateIntegration(ctx context.Context, req *oktapb.UpdateIntegrationRequest) (*oktapb.UpdateIntegrationResponse, error) {
	if err := s.authorize(ctx, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}
	resp, err := s.updateIntegration(ctx, req)
	if err != nil {
		s.logger.WarnContext(ctx, "Failed to update Okta integration.", "error", err)
		return nil, trace.Wrap(err)
	}
	log := s.logger.With("sync_settings", resp.GetPlugin().Spec.GetOkta().SyncSettings)
	log.InfoContext(ctx, "Okta integration successfully updated.")
	return resp, nil
}

func (s *Service) updateIntegration(ctx context.Context, req *oktapb.UpdateIntegrationRequest) (*oktapb.UpdateIntegrationResponse, error) {
	plugin, err := s.pluginBackend.GetPlugin(ctx, types.PluginTypeOkta, true)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	pluginV1, err := validatePlugin(plugin)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.updateOktaSpec(ctx, req, pluginV1); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := s.maybeUpdatePluginCredentials(ctx, req, plugin.GetCredentials().GetStaticCredentialsRef(), pluginV1); err != nil {
		return nil, trace.Wrap(err)
	}
	updatePlugin, err := s.pluginBackend.UpdatePlugin(ctx, pluginV1)
	if err != nil {
		s.logger.WarnContext(ctx, "Failed to updated backend plugin item")
		return nil, trace.Wrap(err)
	}
	updatedPluginV1, ok := updatePlugin.(*types.PluginV1)
	if !ok {
		return nil, trace.BadParameter("plugin is not of type PluginV1")
	}
	return &oktapb.UpdateIntegrationResponse{
		Plugin: updatedPluginV1,
	}, nil
}

func (s *Service) createOktaClientForPluginInstall(ctx context.Context, req *oktapb.CreateIntegrationRequest, connector types.SAMLConnector) (api.Client, error) {
	if req.GetApiCredentials() == nil {
		return nil, trace.BadParameter("missing Okta API credentials")
	}
	if req.GetOktaOrganizationUrl() == "" && connector != nil {
		oktaOrg, err := sso.ExtractOktaOrganizationFromURL(connector.GetSSO())
		if err != nil {
			return nil, trace.BadParameter("missing Okta organization URL")
		}
		if oktaOrg == "" {
			return nil, trace.BadParameter("missing Okta organization URL")
		}
		req.OktaOrganizationUrl = oktaOrg
	}
	scopes := []string{
		api.ScopeAppsRead,
		api.ScopeOrgsRead,
		api.ScopeGroupsRead,
	}
	if connector == nil {
		// If the reuses connector is not set, the flow needs to create SAML Okta app in Okta organization.
		// For that the okta.apps.manage scope is required.
		scopes = append(scopes, api.ScopeAppsManage)
	}

	oktaClient, err := s.createOktaClient(ctx, &createOktaClientParams{
		credsFromReq:     req.GetApiCredentials(),
		oktaOrganization: req.GetOktaOrganizationUrl(),
		scopes:           scopes,
	})
	if err != nil {
		return nil, trace.Wrap(err, "failed to create Okta client")
	}
	return oktaClient, nil
}
