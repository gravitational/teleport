package web

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/gravitational/trace"

	pluginspb "github.com/gravitational/teleport/api/gen/proto/go/teleport/plugins/v1"
	pluginsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/plugins/v1"
	"github.com/gravitational/teleport/api/types"
	entraapiutils "github.com/gravitational/teleport/api/utils/entraid"
	"github.com/gravitational/teleport/e/lib/web/ui"
	"github.com/gravitational/teleport/lib/integrations/azureoidc"
	"github.com/gravitational/teleport/lib/utils/oidc"
	"github.com/gravitational/teleport/lib/web"
)

var errNoTAGCache = errors.New("TAG cache was not submitted")

// entraIDPluginDescriptor is an empty type used to implement an Entra ID specific
// version of the pluginDescriptor interface
type entraIDPluginDescriptor struct{}

// HandleInstallRequest implements pluginDescriptor.
func (entraIDPluginDescriptor) HandleInstallRequest(ctx context.Context, sessCtx *web.SessionContext, w http.ResponseWriter, r *http.Request, p *Plugin) (*ui.Plugin, error) {
	inputs := entraIDPluginInputsFromForm(r.Form)
	if err := inputs.validate(); err != nil {
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
		EntityDescriptorURL: entraapiutils.FederationMetadataURL(inputs.tenantID, inputs.clientID),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if _, err = client.CreateSAMLConnector(ctx, saml); err != nil {
		return nil, trace.Wrap(err)
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
	req := &pluginspb.CreatePluginRequest{
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
							DefaultOwners:     owners,
							SsoConnectorId:    inputs.authConnectorName,
							TenantId:          inputs.tenantID,
							CredentialsSource: types.EntraIDCredentialsSource_ENTRAID_CREDENTIALS_SOURCE_OIDC,
						},
						AccessGraphSettings: tagSyncSettings,
					},
				},
			},
		},
	}

	_, err = client.PluginsClient().CreatePlugin(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	plugin, err := client.PluginsClient().GetPlugin(ctx, &pluginsv1.GetPluginRequest{
		Name: inputs.name,
	})

	if err != nil {
		return nil, trace.Wrap(err)
	}

	uiPlugin, err := ui.NewPlugin(plugin)
	return uiPlugin, trace.Wrap(err)
}

// HandleValidateConfigRequest implements pluginDescriptor.
func (e entraIDPluginDescriptor) HandleValidateConfigRequest(ctx context.Context, sessCtx *web.SessionContext, form url.Values, p *Plugin) error {
	inputs := entraIDPluginInputsFromForm(form)
	if err := inputs.validateEarly(); err != nil {
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

	_, err = client.PluginsClient().GetPlugin(ctx, &pluginsv1.GetPluginRequest{
		Name: inputs.name,
	})
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

	return nil
}

// TranslateCallbackCookie implements pluginDescriptor.
func (entraIDPluginDescriptor) TranslateCallbackCookie(*types.PluginSpecV1, *pluginOnboardingCookie) error {
	// This always returns not implemented, since Entra ID plugin onboarding does not use the OAuth2 web flow.
	return trace.NotImplemented("TranslateCallbackCookie is not implemented for Entra ID")
}

type entraIDPluginInputs struct {
	name              string
	authConnectorName string
	defaultOwners     string
	tenantID          string
	clientID          string
}

func entraIDPluginInputsFromForm(form url.Values) entraIDPluginInputs {
	return entraIDPluginInputs{
		name:              form.Get("name"),
		authConnectorName: form.Get("authConnectorName"),
		defaultOwners:     form.Get("defaultOwners"),
		tenantID:          form.Get("tenantId"),
		clientID:          form.Get("clientId"),
	}
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

// validateEarly validates inputs that the user enters before running the onboarding script.
func (e *entraIDPluginInputs) validateEarly() error {
	if e.name == "" {
		return trace.BadParameter("integration name must be specified")
	}

	if e.authConnectorName == "" {
		return trace.BadParameter("auth connector name must be specified")
	}

	_, err := e.getDefaultOwners()
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func (e *entraIDPluginInputs) validate() error {
	if err := e.validateEarly(); err != nil {
		return trace.Wrap(err)
	}

	if e.tenantID == "" {
		return trace.BadParameter("tenant ID must be specified")
	}
	if e.clientID == "" {
		return trace.BadParameter("client ID must be specified")
	}

	return nil
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
