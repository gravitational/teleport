package oktaservice

import (
	"context"
	"maps"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/bcrypt"

	oktapb "github.com/gravitational/teleport/api/gen/proto/go/teleport/okta/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/okta/common"
	"github.com/gravitational/teleport/e/lib/okta/common/sso"
)

func (s *Service) updatePluginOktaSpec(ctx context.Context, req *oktapb.UpdateIntegrationRequest, plugin *types.PluginV1) error {
	oktaSpec := plugin.Spec.GetOkta()

	// Make sure we don't change the users' sync source for users in legacy plugins.
	if oktaSpec.SyncSettings.SyncUsers && oktaSpec.SyncSettings.AppId == "" {
		if oktaSpec.SyncSettings.GetUserSyncSource().IsUnknown() {
			oktaSpec.SyncSettings.SetUserSyncSource(types.OktaUserSyncSourceOrg)
		}
	}

	if oktaSpec.SyncSettings.AppId == "" || oktaSpec.SyncSettings.AppName == "" {
		if err := s.updateOktaAppInfo(ctx, req, plugin); err != nil && req.GetEnableUserSync() {
			// On init, if UserSync is enabled but AppId is not set, the integration will fail to start,
			// so we shouldn't allow continuing here.
			return trace.BadParameter("Could not fetch info about your Okta SAML application. Verify your API Services application in Okta has all necessary scopes granted and can access your SAML application as part of the defined resource set.")
		}
	}

	if req.GetEnableUserSync() {
		if oktaSpec.SyncSettings.GetUserSyncSource().IsUnknown() {
			oktaSpec.SyncSettings.SetUserSyncSource(types.OktaUserSyncSourceSamlApp)
		}
	}

	oktaSpec.SyncSettings.SyncUsers = req.GetEnableUserSync()
	oktaSpec.SyncSettings.DisableAssignDefaultRoles = req.GetDisableAssignDefaultRoles()
	oktaSpec.SyncSettings.DisableSyncAppGroups = !req.GetEnableAppGroupSync()
	oktaSpec.SyncSettings.DisableBidirectionalSync = !req.GetEnableBidirectionalSync()
	oktaSpec.SyncSettings.SyncAccessLists = req.GetEnableAccessListSync()
	oktaSpec.SyncSettings.GroupFilters = req.GetAccessListSettings().GetGroupFilters()
	oktaSpec.SyncSettings.AppFilters = req.GetAccessListSettings().GetAppFilters()
	oktaSpec.SyncSettings.DefaultOwners = req.GetAccessListSettings().GetDefaultOwner()
	oktaSpec.SyncSettings.EnableSystemLogExport = req.GetEnableSystemLogExport()
	oktaSpec.SyncSettings.TimeBetweenImports = durationToString(req.GetTimeBetweenImports())
	plugin.Spec.Settings = &types.PluginSpecV1_Okta{Okta: oktaSpec}

	return nil
}

func (s *Service) updateOktaAppInfo(ctx context.Context, req *oktapb.UpdateIntegrationRequest, plugin *types.PluginV1) error {
	connectorInfo, err := s.getSAMLConnectorInfo(ctx, req, plugin)
	if err != nil {
		return trace.Wrap(err)
	}

	plugin.Spec.GetOkta().SyncSettings.AppId = connectorInfo.OktaAppID
	plugin.Spec.GetOkta().SyncSettings.AppName = connectorInfo.OktaAppName
	return nil
}

func (s *Service) getSAMLConnectorInfo(ctx context.Context, req *oktapb.UpdateIntegrationRequest, plugin *types.PluginV1) (*sso.SAMLConnectorInfo, error) {
	oktaClient, err := s.createOktaClient(ctx, newUpdateIntegrationRequestWithOrgURL(req, plugin), plugin)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	connectorID := plugin.Spec.GetOkta().GetSyncSettings().SsoConnectorId
	connector, err := s.authService.GetSAMLConnector(ctx, connectorID, false)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	connectorInfo, err := sso.FetchOktaSAMLConnectorInfo(ctx, oktaClient, connector)
	if err != nil {
		return nil, trace.Wrap(err, "fetching Okta SAML app info")
	}
	return connectorInfo, trace.Wrap(err)
}

// updatePluginCredentials updates plugin credentials if the update requests provides any.
// Otherwise it does nothing, i.e. it does not remove existing credentials from the plugin if there
// aren't any in the request.
func (s *Service) updatePluginCredentials(ctx context.Context, req *oktapb.UpdateIntegrationRequest, pluginV1 *types.PluginV1) error {
	staticCredsRef := pluginV1.GetCredentials().GetStaticCredentialsRef()
	if staticCredsRef == nil {
		return trace.NotFound("no static credentials found in plugin")
	}
	staticCreds, err := s.credsBackend.GetPluginStaticCredentialsByLabels(ctx, staticCredsRef.Labels)
	if err != nil {
		return trace.Wrap(err)
	}

	// TODO(kopiczko) clean up credentials, e.g. remove SSWS token when OAuth client ID is upserted.

	if pluginV1.Spec.GetOkta().CredentialsInfo == nil {
		pluginV1.Spec.GetOkta().CredentialsInfo = &types.PluginOktaCredentialsInfo{}
	}
	if req.GetScimToken() != "" {
		if err := s.upsertSCIMCreds(ctx, req.GetScimToken(), staticCreds, staticCredsRef.Labels); err != nil {
			return trace.Wrap(err)
		}
		pluginV1.Spec.GetOkta().CredentialsInfo.HasScimToken = true
	}
	if req.GetApiCredentials().GetOauthId() != "" {
		if err := s.upsertOauthClientID(ctx, req.GetApiCredentials().GetOauthId(), staticCreds, staticCredsRef.Labels); err != nil {
			return trace.Wrap(err)
		}
		pluginV1.Spec.GetOkta().CredentialsInfo.HasSsmToken = false
		pluginV1.Spec.GetOkta().CredentialsInfo.HasOauthCredentials = true
	}
	if req.GetApiCredentials().GetSswsBearerToken() != "" {
		if err := s.upsertSSWSToken(ctx, req.GetApiCredentials().GetSswsBearerToken(), staticCreds, staticCredsRef.Labels); err != nil {
			return trace.Wrap(err)
		}
		pluginV1.Spec.GetOkta().CredentialsInfo.HasSsmToken = true
	}
	return nil
}

func (s *Service) buildBasicConnectorInfo(connector types.SAMLConnector) (*sso.SAMLConnectorInfo, error) {
	if connector == nil {
		return nil, trace.BadParameter("connector is nil")
	}
	connV2, ok := connector.(*types.SAMLConnectorV2)
	if !ok {
		return nil, trace.BadParameter("connector is not of type SAMLConnectorV2")
	}
	oktaOrg, err := sso.ExtractOktaOrganizationFromURL(connV2.Spec.SSO)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &sso.SAMLConnectorInfo{
		Connector: connector,
		OktaOrg:   oktaOrg,
	}, nil
}

func hasSCIMPurpose(resLabels types.ResourceWithLabels) bool {
	v, ok := resLabels.GetLabel(types.OktaCredPurposeLabel)
	return ok && v == types.OktaCredPurposeSCIMToken
}

func appendLabelsToResource(r types.ResourceWithLabels, labels map[string]string) {
	cpy := maps.Clone(labels)
	maps.Copy(cpy, r.GetAllLabels())
	r.SetStaticLabels(cpy)
}

func (s *Service) upsertSCIMCreds(ctx context.Context, scimToken string, creds []types.PluginStaticCredentials, labels map[string]string) error {
	item, err := selectCredsByLabelsFilter(creds, hasSCIMPurpose)
	switch {
	case err == nil:
		scimTokenHash, err := bcrypt.GenerateFromPassword([]byte(scimToken), bcrypt.DefaultCost)
		if err != nil {
			return trace.Wrap(err)
		}
		item.Spec.Credentials = &types.PluginStaticCredentialsSpecV1_APIToken{
			APIToken: string(scimTokenHash),
		}
		if _, err := s.credsBackend.UpdatePluginStaticCredentials(ctx, item); err != nil {
			return trace.Wrap(err, "failed to update plugin static credentials")
		}
		s.logger.DebugContext(ctx, "Updated Okta plugin SCIM credentials.")
	case trace.IsNotFound(err):
		scimTokenHash, err := bcrypt.GenerateFromPassword([]byte(scimToken), bcrypt.DefaultCost)
		if err != nil {
			return trace.Wrap(err)
		}
		newSCIMCreds := buildSCIMCredentials(string(scimTokenHash))
		appendLabelsToResource(newSCIMCreds, labels)
		if err := s.credsBackend.CreatePluginStaticCredentials(ctx, newSCIMCreds); err != nil {
			return trace.Wrap(err, "failed to create plugin static credentials")
		}
		s.logger.DebugContext(ctx, "Created Okta plugin SCIM credentials.")
	case err != nil:
		return trace.Wrap(err)
	}
	return nil
}

func selectCredsByLabelsFilter(creds []types.PluginStaticCredentials, fn func(types.ResourceWithLabels) bool) (*types.PluginStaticCredentialsV1, error) {
	for _, cred := range creds {
		if !fn(cred) {
			continue
		}
		item, ok := cred.(*types.PluginStaticCredentialsV1)
		if !ok {
			return nil, trace.BadParameter("unexpected credential type %T", cred)
		}
		return item, nil
	}
	return nil, trace.NotFound("no credentials found")
}

func isSyncCredential(resLabels types.ResourceWithLabels) bool {
	v, ok := resLabels.GetLabel(types.OktaCredPurposeLabel)
	return ok && v == common.CredPurposeOktaOauth || v == types.OktaCredPurposeAuth || v == ""
}

func (s *Service) upsertOauthClientID(ctx context.Context, clientID string, creds []types.PluginStaticCredentials, labels map[string]string) error {
	item, err := selectCredsByLabelsFilter(creds, isSyncCredential)
	switch {
	case err == nil:
		oauthCred := buildOAuthCredentials(clientID)
		appendLabelsToResource(oauthCred, labels)
		oauthCred.Metadata.Revision = item.GetMetadata().Revision
		if _, err := s.credsBackend.UpdatePluginStaticCredentials(ctx, oauthCred); err != nil {
			return trace.Wrap(err, "failed to update plugin static credentials")
		}
		s.logger.DebugContext(ctx, "Updated Okta plugin OAuth credentials.")
	case trace.IsNotFound(err):
		oauthCred := buildOAuthCredentials(clientID)
		appendLabelsToResource(oauthCred, labels)
		if err := s.credsBackend.CreatePluginStaticCredentials(ctx, oauthCred); err != nil {
			return trace.Wrap(err, "failed to create plugin static credentials")
		}
		s.logger.DebugContext(ctx, "Created Okta plugin OAuth credentials.")
	case err != nil:
		return trace.Wrap(err)
	}
	return nil
}

func hasOktaSSWAuthPurpose(resLabels types.ResourceWithLabels) bool {
	v, ok := resLabels.GetLabel(types.OktaCredPurposeLabel)
	return ok && v == types.OktaCredPurposeAuth
}

func (s *Service) upsertSSWSToken(ctx context.Context, sswsToken string, creds []types.PluginStaticCredentials, labels map[string]string) error {
	item, err := selectCredsByLabelsFilter(creds, hasOktaSSWAuthPurpose)
	switch {
	case err == nil:
		item.Spec.Credentials = &types.PluginStaticCredentialsSpecV1_APIToken{
			APIToken: sswsToken,
		}
		if _, err := s.credsBackend.UpdatePluginStaticCredentials(ctx, item); err != nil {
			return trace.Wrap(err, "failed to update plugin static credentials")
		}
		s.logger.DebugContext(ctx, "Updated okta plugin SSWS credentials.")
	case trace.IsNotFound(err):
		sswsCreds := buildAPITokenCredentials(sswsToken)
		appendLabelsToResource(sswsCreds, labels)
		if err := s.credsBackend.CreatePluginStaticCredentials(ctx, sswsCreds); err != nil {
			return trace.Wrap(err, "failed to create plugin static credentials")
		}
		s.logger.DebugContext(ctx, "Updated okta plugin SSWS credentials.")
	case err != nil:
		return trace.Wrap(err)
	}
	return nil
}
