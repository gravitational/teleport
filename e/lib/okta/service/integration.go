package oktaservice

import (
	"context"
	"log/slog"

	"github.com/google/uuid"
	"github.com/gravitational/trace"

	oktapb "github.com/gravitational/teleport/api/gen/proto/go/teleport/okta/v1"
	pluginspb "github.com/gravitational/teleport/api/gen/proto/go/teleport/plugins/v1"
	"github.com/gravitational/teleport/api/types"
	oktacommon "github.com/gravitational/teleport/e/lib/okta/common"
	"github.com/gravitational/teleport/e/lib/okta/common/sso"
	oktaplugin "github.com/gravitational/teleport/e/lib/okta/plugin"
	"github.com/gravitational/teleport/e/lib/plugins"
	eteleport "github.com/gravitational/teleport/e/lib/teleport"
)

// DefaultRolesAssignmentDisabledError is returned when default roles assignment is disabled during
// the attempt to create SAML connector. In such case no groups to role mappings can be created and
// at least one mapping is required to create the SAML connector.
var DefaultRolesAssignmentDisabledError = trace.BadParameter("Default roles assignment can be disabled only if the Okta SSO connector is already configured in Teleport. Either allow the default roles assignment or create the SAML connector upfront.")

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
	// Since it can't be determined here if the intention is to create a read-only or
	// bidirectional integration installed with those credentials, check read-only scopes here
	// which are required in any case.
	oauthScopes := oktacommon.GetReadOnlyOAuthScopes()
	if err := s.validateClientCredentials(ctx, req, nil, oauthScopes); err != nil {
		return nil, trace.Wrap(err)
	}
	return &oktapb.ValidateClientCredentialsResponse{}, nil
}

// validateClientCredentials validates Okta credentials by trying to list Okta users. It also
// verifies the client created with credentials is authorized to the provided OAuth scopes. Both
// request and plugin can be nil - they are used to create the Okta client as defined in
// [Service.createOktaClient].
func (s *Service) validateClientCredentials(ctx context.Context, req requestWithCredentials, plugin *types.PluginV1, oauthScopes []string) error {
	oktaClient, err := s.createOktaClient(ctx, req, plugin, withOAuthScopes(oauthScopes))
	if err != nil {
		return trace.BadParameter("okta credential verification failed: %v", err)
	}
	if err := oktacommon.CheckClientOAuthScopes(ctx, oktaClient, oauthScopes...); err != nil {
		return trace.BadParameter("Okta OAuth scopes verification failed: %v", err)
	}
	if _, err := oktaClient.ListUsers(ctx); err != nil {
		return trace.BadParameter("Okta credential verification failed: %v", err)
	}
	return nil
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

// CreateIntegration creates a new Okta integration. Depending on the request, it may create a new
// SAML connector or reuse an existing one. SAML connector can be created only when
// DisableAssignDefaultRoles is set to false. Otherwise [DefaultRolesAssignmentDisabledError] is
// returned.
func (s *Service) CreateIntegration(ctx context.Context, req *oktapb.CreateIntegrationRequest) (*oktapb.CreateIntegrationResponse, error) {
	if err := s.authorize(ctx, types.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
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
	samlConnectorName := getSAMLConnectorName(req)
	samlConnector, err := s.getSAMLConnector(ctx, samlConnectorName)
	if trace.IsNotFound(err) {
		if req.GetDisableAssignDefaultRoles() {
			return nil, DefaultRolesAssignmentDisabledError
		}
		// Make sure it's nil for validation.
		samlConnector = nil
	} else if err != nil {
		return nil, trace.Wrap(err, "fetching SAML connector %q", samlConnectorName)
	}

	if err := validateCreateIntegrationRequest(req, samlConnector); err != nil {
		return nil, trace.Wrap(err, "create integration failed due to invalid request")
	}
	if req.GetApiCredentials() != nil {
		if err := s.validateClientCredentials(ctx, req, nil, oktacommon.GetOAuthScopesForIntegrationRequest(req)); err != nil {
			return nil, trace.Wrap(err, "validating request credentials")
		}
	}

	connectorInfo, err := s.ensureSAMLConnector(ctx, req, samlConnector)
	if err != nil {
		return nil, trace.Wrap(err, "failed to get or create SAML connector")
	}

	creds, err := newOktaPluginCredentials(req)
	if err != nil {
		return nil, trace.Wrap(err, "failed to get Okta plugin credentials")
	}

	oktaPlugin, err := newOktaPlugin(req, connectorInfo, creds)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := validatePlugin(oktaPlugin); err != nil {
		return nil, trace.Wrap(err, "failed to gather all the necessary information for the plugin creation")
	}

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
			OktaAppId:             connectorInfo.OktaAppID,
			OktaAppName:           connectorInfo.OktaAppName,
			OktaAppLabels:         connectorInfo.OktaAppLabel,
			TeleportConnectorName: connectorInfo.Connector.GetName(),
		},
	}, nil
}

func newOktaPlugin(req *oktapb.CreateIntegrationRequest, connectorInfo *sso.SAMLConnectorInfo, creds []*types.PluginStaticCredentialsV1) (*types.PluginV1, error) {
	credsInfo := buildCredentialsInfo(creds)

	oktaSettings := &types.PluginOktaSettings{
		CredentialsInfo: credsInfo,
		OrgUrl:          connectorInfo.OktaOrg,
		SyncSettings: &types.PluginOktaSyncSettings{
			SyncUsers:                 req.GetEnableUserSync(),
			UserSyncSource:            string(types.OktaUserSyncSourceSamlApp),
			DisableAssignDefaultRoles: req.GetDisableAssignDefaultRoles(),
			SyncAccessLists:           req.GetEnableAccessListSync(),
			DisableSyncAppGroups:      !req.GetEnableAppGroupSync(),
			DisableBidirectionalSync:  !req.GetEnableBidirectionalSync(),
			SsoConnectorId:            connectorInfo.Connector.GetName(),
			AppId:                     connectorInfo.OktaAppID,
			AppName:                   connectorInfo.OktaAppName,

			GroupFilters:          req.GetAccessListSettings().GetGroupFilters(),
			AppFilters:            req.GetAccessListSettings().GetAppFilters(),
			DefaultOwners:         req.GetAccessListSettings().GetDefaultOwner(),
			EnableSystemLogExport: req.GetEnableSystemLogExport(),

			TimeBetweenImports: durationToString(req.GetTimeBetweenImports()),
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

	if len(creds) == 0 {
		// In case if there is not SSWS Oauth and SCIM token credentials (Plugin with SSO flow only)
		// we will generate the label ref credentials to fulfill the validation.
		if err := plugin.SetCredentials(&types.PluginCredentialsV1{
			Credentials: &types.PluginCredentialsV1_StaticCredentialsRef{
				StaticCredentialsRef: &types.PluginStaticCredentialsRef{
					Labels: map[string]string{eteleport.PluginLabel: uuid.NewString()},
				},
			},
		}); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return plugin, nil
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
	plugin, err := oktaplugin.Get(ctx, s.pluginBackend, true /* withSecrets */)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := validateUpdateIntegrationRequest(req, plugin); err != nil {
		return nil, trace.Wrap(err, "request validation")
	}

	if req.GetEnableUserSync() {
		if err := s.validateClientCredentials(ctx, newUpdateIntegrationRequestWithOrgURL(req, plugin), plugin, oktacommon.GetOAuthScopesForIntegrationRequest(req)); err != nil {
			return nil, trace.Wrap(err, "validating plugin credentials")
		}
	}

	if err := s.updatePluginOktaSpec(ctx, req, plugin); err != nil {
		return nil, trace.Wrap(err, "updating plugin Okta settings")
	}
	if err := s.updatePluginCredentials(ctx, req, plugin); err != nil {
		return nil, trace.Wrap(err, "updating plugin credentials")
	}

	if err := validatePlugin(plugin); err != nil {
		return nil, trace.Wrap(err, "failed to gather all the necessary information for the plugin update")
	}

	updatedPlugin, err := s.pluginBackend.UpdatePlugin(ctx, plugin)
	if err != nil {
		return nil, trace.Wrap(err, "updating plugin in backend")
	}
	updatedPluginV1, ok := updatedPlugin.(*types.PluginV1)
	if !ok {
		return nil, trace.BadParameter("plugin is not of type PluginV1")
	}
	return &oktapb.UpdateIntegrationResponse{
		Plugin: updatedPluginV1,
	}, nil
}
