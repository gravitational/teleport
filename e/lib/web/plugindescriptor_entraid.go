package web

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/gravitational/trace"

	pluginsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/plugins/v1"
	"github.com/gravitational/teleport/api/types"
	entraapiutils "github.com/gravitational/teleport/api/utils/entraid"
	"github.com/gravitational/teleport/e/lib/web/ui"
	"github.com/gravitational/teleport/lib/integrations/azureoidc"
	"github.com/gravitational/teleport/lib/plugins/filter"
	"github.com/gravitational/teleport/lib/utils/oidc"
	"github.com/gravitational/teleport/lib/web"
)

var errNoTAGCache = errors.New("TAG cache was not submitted")

// entraIDPluginDescriptor is an empty type used to implement an Entra ID specific
// version of the pluginDescriptor interface
type entraIDPluginDescriptor struct {
	// The Entra ID plugin installation step fetches live
	// Microsoft Entra ID SAML entity descriptor. Providing an entity descriptor
	// beforehand causes the SAML connector validator to skip the fetcher,
	// which is useful for tests.
	testEntityDescriptor string
}

// HandleInstallRequest implements pluginDescriptor.
func (e entraIDPluginDescriptor) HandleInstallRequest(ctx context.Context, sessCtx *web.SessionContext, w http.ResponseWriter, r *http.Request, p *Plugin) (*ui.Plugin, error) {
	inputs, err := parseEntraIDPluginInputs(r.Form, true /*read all inputs*/)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	client, err := sessCtx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	proxyPublicAddr, err := oidc.IssuerFromPublicAddress(p.h.PublicProxyAddr(), "")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	proxyPublicAddr = strings.TrimRight(proxyPublicAddr, "/")

	// TAG cache file is submitted only when Access Graph integration was requested by the user.
	tagCache, err := readTAGCache(r)
	if err != nil && !errors.Is(err, errNoTAGCache) {
		return nil, trace.Wrap(err)
	}
	var tagSyncSettings *types.PluginEntraIDAccessGraphSettings
	if tagCache != nil {
		tagSyncSettings = &types.PluginEntraIDAccessGraphSettings{
			AppSsoSettingsCache: tagCache.AppSsoSettingsCache,
		}
	}

	edURL := entraapiutils.FederationMetadataURL(inputs.tenantID, inputs.clientID)
	ed := ""
	if e.testEntityDescriptor != "" {
		// skip live entity descriptor validator in tests.
		edURL = ""
		ed = e.testEntityDescriptor
	}
	saml, err := types.NewSAMLConnector(inputs.authConnectorName, types.SAMLConnectorSpecV2{
		AssertionConsumerService: proxyPublicAddr + "/v1/webapi/saml/acs/" + inputs.authConnectorName,
		AllowIDPInitiated:        true,
		// AttributesToRoles is required, but Entra ID does not have a default group (like Okta's "Everyone"),
		// so we add a dummy claim that will always be fulfilled and map them to the "requester" role.
		AttributesToRoles: []types.AttributeMapping{
			{
				Name:  "http://schemas.microsoft.com/ws/2008/06/identity/claims/groups",
				Value: "*",
				Roles: []string{"requester"},
			},
		},
		Display:             "Entra ID",
		EntityDescriptorURL: edURL,
		EntityDescriptor:    ed,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if _, err = client.CreateSAMLConnector(ctx, saml); err != nil {
		return nil, trace.Wrap(err)
	}

	filters, err := filter.NewFromInputs(inputs.groupFilters)
	if err != nil {
		return nil, trace.Wrap(err, "invalid group filter")
	}

	integrationSpec, err := types.NewIntegrationAzureOIDC(
		types.Metadata{Name: inputs.name},
		&types.AzureOIDCIntegrationSpecV1{
			TenantID: inputs.tenantID,
			ClientID: inputs.clientID,
		},
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if _, err = client.CreateIntegration(ctx, integrationSpec); err != nil {
		return nil, trace.Wrap(err)
	}

	owners, err := inputs.getDefaultOwners()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	req := pluginsv1.CreatePluginRequest_builder{
		Plugin: &types.PluginV1{
			Metadata: types.Metadata{
				Name: inputs.name,
				Labels: map[string]string{
					"teleport.dev/hosted-plugin": "true",
				},
			},
			Spec: types.PluginSpecV1{
				Settings: &types.PluginSpecV1_EntraId{
					EntraId: &types.PluginEntraIDSettings{
						SyncSettings: &types.PluginEntraIDSyncSettings{
							DefaultOwners:          owners,
							SsoConnectorId:         inputs.authConnectorName,
							TenantId:               inputs.tenantID,
							CredentialsSource:      types.EntraIDCredentialsSource_ENTRAID_CREDENTIALS_SOURCE_OIDC,
							EntraAppId:             inputs.clientID,
							GroupFilters:           filters,
							AccessListOwnersSource: inputs.accessListOwnersSource,
						},
						AccessGraphSettings: tagSyncSettings,
					},
				},
			},
		},
	}.Build()

	_, err = client.PluginsClient().CreatePlugin(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	plugin, err := client.PluginsClient().GetPlugin(ctx, pluginsv1.GetPluginRequest_builder{
		Name: inputs.name,
	}.Build())

	if err != nil {
		return nil, trace.Wrap(err)
	}

	uiPlugin, err := ui.NewPlugin(plugin)
	return uiPlugin, trace.Wrap(err)
}

// HandleValidateConfigRequest implements pluginDescriptor.
func (e entraIDPluginDescriptor) HandleValidateConfigRequest(ctx context.Context, sessCtx *web.SessionContext, form url.Values, p *Plugin) error {
	inputs, err := parseEntraIDPluginInputs(form, false /*skip Entra ID specific config which is not available at this stage*/)
	if err != nil {
		return trace.Wrap(err)
	}

	client, err := sessCtx.GetClient()
	if err != nil {
		return trace.Wrap(err)
	}

	// Check for duplicate object names

	_, err = client.GetIntegration(ctx, inputs.name)
	if err == nil {
		return trace.BadParameter("integration named %q already exists", inputs.name)
	} else if !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}

	_, err = client.PluginsClient().GetPlugin(ctx, pluginsv1.GetPluginRequest_builder{
		Name: inputs.name,
	}.Build())
	if err == nil {
		return trace.BadParameter("integration named %q already exists", inputs.name)
	} else if !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}

	const withSecrets = false
	_, err = client.GetSAMLConnector(ctx, inputs.authConnectorName, withSecrets)
	if err == nil {
		return trace.BadParameter("auth connector named %q already exists", inputs.authConnectorName)
	} else if !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}

	if _, err = filter.NewFromInputs(inputs.groupFilters); err != nil {
		return trace.Wrap(err, "invalid group filter")
	}

	return nil
}

// HandleUpdateRequest updates the Entra ID plugin.
func (entraIDPluginDescriptor) HandleUpdateRequest(ctx context.Context, sessCtx *web.SessionContext, req *ui.PluginUpdateRequest) (*ui.Plugin, error) {
	if req.EntraID == nil {
		return nil, trace.BadParameter("missing Entra plugin update params")
	}
	if req.EntraID.Name == "" {
		return nil, trace.BadParameter("plugin name is required")
	}
	if len(req.EntraID.DefaultOwners) == 0 {
		return nil, trace.BadParameter("default owners cannot be empty")
	}
	filters, err := filter.NewFromInputs(req.EntraID.GroupFilters)
	if err != nil {
		return nil, trace.Wrap(err, "invalid group filter")
	}
	ownersSource, err := parseOwnersSource(req.EntraID.AccessListOwnersSource)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	client, err := sessCtx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	existingPlugin, err := client.PluginsClient().GetPlugin(ctx,
		pluginsv1.GetPluginRequest_builder{
			Name: req.EntraID.Name,
		}.Build(),
	)
	if err != nil {
		return nil, trace.Wrap(err, "failed getting existing plugin")
	}

	newPlugin, ok := existingPlugin.Clone().(*types.PluginV1)
	if !ok {
		return nil, trace.BadParameter("expected *PluginV1 while cloning existing plugin for update, got %T", newPlugin)
	}
	settings := newPlugin.Spec.GetEntraId()
	settings.SyncSettings.DefaultOwners = req.EntraID.DefaultOwners
	settings.SyncSettings.GroupFilters = filters
	settings.SyncSettings.AccessListOwnersSource = ownersSource
	if req.EntraID.SyncIntervals != nil {
		settings.SyncSettings.SyncIntervals = req.EntraID.SyncIntervals
	}
	newPlugin.Spec.Settings = &types.PluginSpecV1_EntraId{
		EntraId: settings,
	}

	resp, err := client.PluginsClient().UpdatePlugin(ctx,
		pluginsv1.UpdatePluginRequest_builder{
			Plugin: newPlugin,
		}.Build())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return ui.NewPlugin(resp)
}

// TranslateCallbackCookie implements pluginDescriptor.
func (entraIDPluginDescriptor) TranslateCallbackCookie(*types.PluginSpecV1, *pluginOnboardingCookie) error {
	// This always returns not implemented, since Entra ID plugin onboarding does not use the OAuth2 web flow.
	return trace.NotImplemented("TranslateCallbackCookie is not implemented for Entra ID")
}

// HandleOAuthStart implements PluginDescriptor for entraIDPluginDescriptor, always
// returning "Not Implemented".
func (entraIDPluginDescriptor) HandleOAuthStart(ctx context.Context, sessCtx *web.SessionContext, w http.ResponseWriter, r *http.Request, p *Plugin) (*ui.OAuthPluginStartResponse, error) {
	return nil, trace.NotImplemented("HandleOAuthStart")
}

type entraIDPluginInputs struct {
	name                   string
	authConnectorName      string
	defaultOwners          string
	tenantID               string
	clientID               string
	groupFilters           filter.Inputs
	accessListOwnersSource types.EntraIDAccessListOwnersSource
}

// parseEntraIDPluginInputs parses Entra ID plugin inputs.
func parseEntraIDPluginInputs(form url.Values, includeEntraConfig bool) (entraIDPluginInputs, error) {
	var inputs entraIDPluginInputs

	inputs.name = form.Get("name")
	if inputs.name == "" {
		return inputs, trace.BadParameter("plugin name must be specified")
	}

	inputs.authConnectorName = form.Get("authConnectorName")
	if inputs.authConnectorName == "" {
		return inputs, trace.BadParameter("auth connector name must be specified")
	}

	inputs.defaultOwners = form.Get("defaultOwners")
	_, err := inputs.getDefaultOwners()
	if err != nil {
		return inputs, trace.Wrap(err, "parsing default owners")
	}

	groupFilters := form.Get("groupFilters")
	if groupFilters != "" {
		err = json.Unmarshal([]byte(groupFilters), &inputs.groupFilters)
		if err != nil {
			return inputs, trace.Wrap(err, "parsing group filters")
		}
	}

	if includeEntraConfig {
		inputs.tenantID = form.Get("tenantId")
		if inputs.tenantID == "" {
			return inputs, trace.BadParameter("tenant ID must be specified")
		}
		inputs.clientID = form.Get("clientId")

		if inputs.clientID == "" {
			return inputs, trace.BadParameter("client ID must be specified")
		}
	}

	ownersSource, err := parseOwnersSource(form.Get("accessListOwnersSource"))
	if err != nil {
		return inputs, trace.Wrap(err)
	}
	inputs.accessListOwnersSource = ownersSource

	return inputs, nil
}

// getDefaultOwners returns a list of user names of users that will be made owners
// of access lists imported via the Okta plugin.
func (e *entraIDPluginInputs) getDefaultOwners() ([]string, error) {
	var defaultOwners []string
	defaultOwnersString := e.defaultOwners
	if defaultOwnersString != "" {
		defaultOwners = []string{}
		if err := json.Unmarshal([]byte(defaultOwnersString), &defaultOwners); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	if len(defaultOwners) == 0 {
		return nil, trace.BadParameter("default owners must be specified")
	}

	return defaultOwners, nil
}

func readTAGCache(r *http.Request) (*azureoidc.TAGInfoCache, error) {
	if r.MultipartForm == nil {
		return nil, trace.Wrap(errNoTAGCache)
	}
	files := r.MultipartForm.File["accessGraphCache"]
	if len(files) != 1 {
		return nil, trace.Wrap(errNoTAGCache)
	}

	file, err := files[0].Open()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer file.Close()

	var result azureoidc.TAGInfoCache
	if err := json.NewDecoder(file).Decode(&result); err != nil {
		return nil, trace.Wrap(err)
	}

	return &result, nil
}

func parseOwnersSource(in string) (types.EntraIDAccessListOwnersSource, error) {
	enumVal, ok := types.EntraIDAccessListOwnersSource_value[in]
	if !ok || enumVal == 0 {
		return types.EntraIDAccessListOwnersSource_ENTRAID_ACCESS_LIST_OWNERS_SOURCE_UNSPECIFIED,
			trace.BadParameter("unexpected Access List owners source %q", in)
	}

	return types.EntraIDAccessListOwnersSource(enumVal), nil
}
