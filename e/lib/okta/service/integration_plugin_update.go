package oktaservice

import (
	"context"
	"encoding/xml"
	"fmt"
	"maps"
	"regexp"

	"github.com/gravitational/trace"
	"github.com/okta/okta-sdk-golang/v2/okta"
	samltypes "github.com/russellhaering/gosaml2/types"

	oktapb "github.com/gravitational/teleport/api/gen/proto/go/teleport/okta/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/okta/api"
	"github.com/gravitational/teleport/e/lib/okta/common"
	"github.com/gravitational/teleport/e/lib/okta/common/sso"
	"github.com/gravitational/teleport/e/lib/teleport"
)

func validatePlugin(plugin types.Plugin) (*types.PluginV1, error) {
	pluginV1, ok := plugin.(*types.PluginV1)
	if !ok {
		return nil, trace.BadParameter("plugin is not of type PluginV1")
	}
	if staticCredsRef := plugin.GetCredentials().GetStaticCredentialsRef(); staticCredsRef == nil {
		return nil, trace.NotFound("no static credentials found")
	}
	return pluginV1, nil
}

func (s *Service) updateOktaSpec(ctx context.Context, req *oktapb.UpdateIntegrationRequest, plugin *types.PluginV1) error {
	pluginSpec := plugin.Spec.GetOkta()
	pluginSpec.SyncSettings.SyncUsers = req.GetEnableUserSync()
	pluginSpec.SyncSettings.DisableSyncAppGroups = !req.GetEnableAppGroupSync()
	pluginSpec.SyncSettings.SyncAccessLists = req.GetEnableAccessListSync()
	pluginSpec.SyncSettings.GroupFilters = req.GetAccessListSettings().GetGroupFilters()
	pluginSpec.SyncSettings.AppFilters = req.GetAccessListSettings().GetAppFilters()
	pluginSpec.SyncSettings.DefaultOwners = req.GetAccessListSettings().GetDefaultOwner()
	if pluginSpec.SyncSettings.AppId == "" {
		s.tryToUpdateOktaAppID(ctx, req, pluginSpec, plugin)
	}
	plugin.Spec.Settings = &types.PluginSpecV1_Okta{Okta: pluginSpec}
	return nil
}

func (s *Service) tryToUpdateOktaAppID(ctx context.Context, req *oktapb.UpdateIntegrationRequest, pluginSpec *types.PluginOktaSettings, plugin types.Plugin) {
	params := &createOktaClientParams{
		credsFromReq:            req.GetApiCredentials(),
		oktaOrganization:        pluginSpec.OrgUrl,
		pluginCredentialsLabels: plugin.GetCredentials().GetStaticCredentialsRef().Labels,
		connectorID:             pluginSpec.SsoConnectorId,
	}
	appID, err := s.tryToFetchOktaAppID(ctx, params)
	if err != nil {
		s.logger.DebugContext(ctx, "Failed to fetch Okta App ID", "error", err)
		return
	}
	pluginSpec.SyncSettings.AppId = appID
}

func (s *Service) maybeUpdatePluginCredentials(ctx context.Context, req *oktapb.UpdateIntegrationRequest, staticCredsRef *types.PluginStaticCredentialsRef, pluginV1 *types.PluginV1) error {
	staticCreds, err := s.credsBackend.GetPluginStaticCredentialsByLabels(ctx, staticCredsRef.Labels)
	if err != nil {
		return trace.Wrap(err)
	}

	if pluginV1.Spec.GetOkta().CredentialsInfo == nil {
		pluginV1.Spec.GetOkta().CredentialsInfo = &types.PluginOktaCredentialsInfo{}
	}
	if req.GetScimToken() != "" {
		if err := s.upsertSCIMCreds(ctx, req.GetScimToken(), staticCreds, staticCredsRef.Labels); err != nil {
			return trace.Wrap(err)
		}
	}
	if req.GetApiCredentials().GetOauthId() != "" {
		if err := s.upsertOauthClientID(ctx, req.GetApiCredentials().GetOauthId(), staticCreds, staticCredsRef.Labels); err != nil {
			return trace.Wrap(err)
		}
	}
	if req.GetApiCredentials().GetSswsBearerToken() != "" {
		if err := s.upsertSSWSToken(ctx, req.GetApiCredentials().GetSswsBearerToken(), staticCreds, staticCredsRef.Labels); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

func (s *Service) tryToFetchOktaAppID(ctx context.Context, params *createOktaClientParams) (string, error) {
	oktaClient, err := s.createOktaClient(ctx, params)
	if err != nil {
		return "", trace.Wrap(err)
	}
	samlConnector, err := s.authService.GetSAMLConnector(ctx, params.connectorID, false)
	if err != nil {
		return "", trace.Wrap(err)
	}
	connInfo, err := s.maybeFetchMetadataFromApp(ctx, oktaClient, samlConnector)
	if err != nil {
		return "", trace.Wrap(err)
	}
	return connInfo.OktaAppID, nil
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

func (s *Service) maybeFetchMetadataFromApp(ctx context.Context, oktaClient api.Client, connector types.SAMLConnector) (*sso.SAMLConnectorInfo, error) {
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
	if connAppID := connector.GetMetadata().Labels[teleport.OktaAppIDLabel]; connAppID != "" {
		samlApp, err := fetchMetadataBasedOnAppID(ctx, oktaClient, connAppID)
		if err != nil {
			return &sso.SAMLConnectorInfo{
				OktaAppID: connAppID,
				Connector: connector,
				OktaOrg:   oktaOrg,
			}, nil
		}
		return &sso.SAMLConnectorInfo{
			OktaOrg:      oktaOrg,
			Connector:    connector,
			OktaAppID:    connAppID,
			OktaAppName:  samlApp.Name,
			OktaAppLabel: samlApp.Label,
		}, nil
	}

	appName, err := extractOktaAppNameFromConnector(connector)
	if err != nil {
		return &sso.SAMLConnectorInfo{
			Connector: connector,
			OktaOrg:   oktaOrg,
		}, nil
	}
	app, err := s.fetchOktaAppByName(ctx, oktaClient, appName)
	if err != nil {
		// The fetchOktaAppByName Okta API call can fail due to various reasons
		// for instance okta API token does not have enough permissions to get okta SAML app details
		// Okta App is not found.
		s.logger.With("app_name", appName, "error", err).InfoContext(ctx, "Failed to fetch Okta app by name.")

		return &sso.SAMLConnectorInfo{
			OktaAppName: appName,
			Connector:   connector,
			OktaOrg:     oktaOrg,
		}, nil
	}
	return &sso.SAMLConnectorInfo{
		OktaAppName:  appName,
		OktaAppID:    app.Id,
		OktaAppLabel: app.Label,
		Connector:    connector,
		OktaOrg:      oktaOrg,
	}, nil
}

func fetchMetadataBasedOnAppID(ctx context.Context, oktaClient api.Client, appID string) (*okta.SamlApplication, error) {
	app, err := oktaClient.GetApplication(ctx, api.OktaAppID(appID), &okta.SamlApplication{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	samlApp, ok := app.(*okta.SamlApplication)
	if !ok {
		return nil, trace.BadParameter("invalid Okta App type: %T", app)
	}
	return samlApp, nil
}

func extractOktaAppNameFromConnector(connector types.SAMLConnector) (string, error) {
	var entityDesc samltypes.EntityDescriptor
	if err := xml.Unmarshal([]byte(connector.GetEntityDescriptor()), &entityDesc); err != nil {
		return "", trace.Wrap(err)
	}
	if len(entityDesc.IDPSSODescriptor.SingleSignOnServices) == 0 {
		return "", trace.NotFound("no SingleSignOnService found in SAML connector")
	}

	re := regexp.MustCompile(`(https://[^/]+)/(app/([^/]+))`)
	matches := re.FindStringSubmatch(entityDesc.IDPSSODescriptor.SingleSignOnServices[0].Location)
	if len(matches) < 4 {
		return "", trace.NotFound("failed to parse app name from SingleSignOnService location")
	}
	return matches[3], nil
}

func (s *Service) fetchOktaAppByName(ctx context.Context, oktaClient api.Client, oktaSAMLAppName string) (*okta.Application, error) {
	var selectedApp *okta.Application
	err := oktaClient.IterateApps(ctx, func(a okta.App) error {
		var ok bool
		app, ok := a.(*okta.Application)
		if !ok {
			s.logger.DebugContext(ctx, "Unable to process Okta application of unknown type", "type", fmt.Sprintf("%T", a))
			return nil
		}
		if app.Name == oktaSAMLAppName {
			selectedApp = app
		}
		return nil
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if selectedApp == nil {
		return nil, trace.NotFound("Okta app %q not found", oktaSAMLAppName)
	}
	return selectedApp, nil
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
		sswsCreds := buildAPITokenCredential(sswsToken)
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
