package oktaservice

import (
	"context"
	"maps"

	"github.com/gravitational/trace"

	oktapb "github.com/gravitational/teleport/api/gen/proto/go/teleport/okta/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/okta/common"
	"github.com/gravitational/teleport/e/lib/okta/common/sso"
)

func (s *Service) updatePluginOktaSpec(ctx context.Context, req *oktapb.UpdateIntegrationRequest, plugin *types.PluginV1) error {
	pluginSpec := plugin.Spec.GetOkta()
	pluginSpec.SyncSettings.SyncUsers = req.GetEnableUserSync()
	pluginSpec.SyncSettings.DisableSyncAppGroups = !req.GetEnableAppGroupSync()
	pluginSpec.SyncSettings.SyncAccessLists = req.GetEnableAccessListSync()
	pluginSpec.SyncSettings.GroupFilters = req.GetAccessListSettings().GetGroupFilters()
	pluginSpec.SyncSettings.AppFilters = req.GetAccessListSettings().GetAppFilters()
	pluginSpec.SyncSettings.DefaultOwners = req.GetAccessListSettings().GetDefaultOwner()
	if pluginSpec.SyncSettings.AppId == "" {
		s.tryUpdateOktaAppID(ctx, req, pluginSpec, plugin)
	}
	plugin.Spec.Settings = &types.PluginSpecV1_Okta{Okta: pluginSpec}
	return nil
}

func (s *Service) tryUpdateOktaAppID(ctx context.Context, req *oktapb.UpdateIntegrationRequest, pluginSpec *types.PluginOktaSettings, plugin types.Plugin) {
	params := &createOktaClientParams{
		credsFromReq:            req.GetApiCredentials(),
		oktaOrganization:        pluginSpec.OrgUrl,
		pluginCredentialsLabels: plugin.GetCredentials().GetStaticCredentialsRef().Labels,
	}

	appId, err := s.fetchOktaAppIdFromConnector(ctx, params, pluginSpec.SyncSettings.SsoConnectorId)
	if err != nil {
		s.logger.WarnContext(ctx, "Failed to fetch Okta App ID", "error", err)
		return
	}

	pluginSpec.SyncSettings.AppId = appId
}

func (s *Service) fetchOktaAppIdFromConnector(ctx context.Context, createOktaClientParams *createOktaClientParams, connectorId string) (appId string, err error) {
	oktaClient, err := s.createOktaClient(ctx, createOktaClientParams)
	if err != nil {
		return "", trace.Wrap(err)
	}

	connector, err := s.authService.GetSAMLConnector(ctx, connectorId, false)
	if err != nil {
		return "", trace.Wrap(err)
	}

	appId, err = sso.FetchOktaAppIdFromConnector(ctx, oktaClient, connector)
	if err != nil {
		s.logger.WarnContext(ctx, "Failed to fetch Okta App ID", "error", err)
		return
	}
	return appId, trace.Wrap(err)
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
	v, ok := resLabels.GetLabel(common.CredPurposeLabel)
	return ok && v == common.CredPurposeSCIMToken
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
		item.Spec.Credentials = &types.PluginStaticCredentialsSpecV1_APIToken{
			APIToken: scimToken,
		}
		if _, err := s.credsBackend.UpdatePluginStaticCredentials(ctx, item); err != nil {
			return trace.Wrap(err, "failed to update plugin static credentials")
		}
		s.logger.DebugContext(ctx, "Updated Okta plugin SCIM credentials.")
	case trace.IsNotFound(err):
		newSCIMCreds := buildSCIMCredentials(scimToken)
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

func hasOauthPurpose(resLabels types.ResourceWithLabels) bool {
	v, ok := resLabels.GetLabel(common.CredPurposeLabel)
	return ok && v == common.CredPurposeOktaOauth || v == ""
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

func (s *Service) upsertOauthClientID(ctx context.Context, clientID string, creds []types.PluginStaticCredentials, labels map[string]string) error {
	item, err := selectCredsByLabelsFilter(creds, hasOauthPurpose)
	switch {
	case err == nil:
		if _, err := s.credsBackend.UpdatePluginStaticCredentials(ctx, item); err != nil {
			return trace.Wrap(err, "failed to update plugin static credentials")
		}
		s.logger.DebugContext(ctx, "Updated Okta plugin OAuth credentials.")
	case trace.IsNotFound(err):
		scimCreds := buildOAuthCredentials(clientID)
		appendLabelsToResource(scimCreds, labels)
		if err := s.credsBackend.CreatePluginStaticCredentials(ctx, scimCreds); err != nil {
			return trace.Wrap(err, "failed to create plugin static credentials")
		}
		s.logger.DebugContext(ctx, "Created Okta plugin OAuth credentials.")
	case err != nil:
		return trace.Wrap(err)
	}
	return nil
}

func hasOktaSSWAuthPurpose(resLabels types.ResourceWithLabels) bool {
	v, ok := resLabels.GetLabel(common.CredPurposeLabel)
	return ok && v == common.CredPurposeOktaAuth
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
