package entraid

import (
	"context"
	"log/slog"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/gravitational/trace"
	saml2 "github.com/russellhaering/gosaml2"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/services"
)

// componentName is the component name used by the default logger.
const componentName = "saml-entra-token-resolver"

// TokenResolverConfig holds the dependencies for the TokenResolver.
type TokenResolverConfig struct {
	// Plugins provides the service for getting Plugins.
	Plugins services.PluginGetter
	// Integrations provides the service for getting Integrations.
	Integrations services.IntegrationsGetter
	// TokenGenerator returns a client assertion token for Azure OIDC integration auth.
	// If omitted, the resolver will return an error if the integration path is used.
	TokenGenerator func(ctx context.Context) (string, error)
	// NewSystemCredential constructs the Azure credentials used for SYSTEM_CREDENTIALS auth source.
	// If omitted, checkAndSetDefaults creates a default (azidentity.NewDefaultAzureCredential).
	NewSystemCredential func(options *azidentity.DefaultAzureCredentialOptions) (azcore.TokenCredential, error)
	// Logger provides the Logger to use.
	// If omitted, checkAndSetDefaults creates a default Logger.
	Logger *slog.Logger
}

// checkAndSetDefaults validates the fields on TokenResolverConfig.
// For any missing required fields, it returns an error.
// For any missing optional fields, it sets a default value.
func (c *TokenResolverConfig) checkAndSetDefaults() error {
	switch {
	case c.Plugins == nil:
		return trace.BadParameter("plugins is required")
	case c.Integrations == nil:
		return trace.BadParameter("integrations is required")
	}

	if c.NewSystemCredential == nil {
		c.NewSystemCredential = func(options *azidentity.DefaultAzureCredentialOptions) (azcore.TokenCredential, error) {
			return azidentity.NewDefaultAzureCredential(options)
		}
	}

	if c.Logger == nil {
		c.Logger = slog.With(teleport.ComponentKey, teleport.Component(teleport.ComponentAuth, componentName))
	}

	return nil
}

// TokenResolver is used to resolve a token credential to authenticate against MS Graph API.
type TokenResolver struct {
	TokenResolverConfig
}

// NewTokenResolver creates a TokenResolver with the provided config.
func NewTokenResolver(cfg TokenResolverConfig) (*TokenResolver, error) {
	if err := cfg.checkAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &TokenResolver{cfg}, nil
}

// ResolveToken returns a token credential for the given connector.
// The order of resolution is:
//
// 1. Plugin (system credentials or integration credentials, depending on configured plugin credentials source)
// 2. Connector (credentials specified on the connector, currently only OAuth)
func (r *TokenResolver) ResolveToken(
	ctx context.Context,
	connector types.SAMLConnector,
	assertionInfo *saml2.AssertionInfo,
) (azcore.TokenCredential, error) {
	plugin, err := r.getPluginForConnector(ctx, connector.GetName())
	if err == nil {
		return r.tokenFromPlugin(ctx, plugin)
	}
	// If no matching plugin, fallback to connector credentials.
	if trace.IsNotFound(err) {
		return r.tokenFromConnectorCredentials(connector.GetCredentials(), assertionInfo)
	}
	return nil, trace.Wrap(err)
}

// getPluginForConnector returns the plugin associated with the given connector.
// If more than one plugin match then an error is returned.
func (r *TokenResolver) getPluginForConnector(ctx context.Context, connectorName string) (*types.PluginV1, error) {
	// TODO(nixpig): Consider approaches for determining plugin that's not O(n) lookup. As it stands, we expect this to be operating
	// on cache and for customers to have relatively few plugins configured.
	var match *types.PluginV1
	for plugin, err := range clientutils.Resources(ctx, func(ctx context.Context, limit int, startKey string) ([]types.Plugin, string, error) {
		return r.Plugins.ListPlugins(ctx, limit, startKey, false)
	}) {
		if err != nil {
			return nil, trace.Wrap(err)
		}

		pluginV1, ok := plugin.(*types.PluginV1)
		if !ok {
			continue
		}

		if pluginV1.GetType() != types.PluginTypeEntraID {
			continue
		}

		entraSettings := pluginV1.Spec.GetEntraId()
		if entraSettings == nil || entraSettings.SyncSettings == nil {
			r.Logger.WarnContext(ctx, "Entra plugin missing required settings", "plugin_name", pluginV1.GetName())
			continue
		}

		if entraSettings.SyncSettings.SsoConnectorId != connectorName {
			continue
		}

		if match != nil {
			return nil, trace.BadParameter("multiple Entra plugins match connector")
		}

		match = pluginV1
	}

	if match == nil {
		return nil, trace.NotFound("no Entra plugin matching connector found")
	}

	return match, nil
}

// tokenFromPlugin returns a token credential for the given plugin.
// If the plugin uses system credentials, then system credentials are used.
// If the plugin uses an integration, then the integration credentials are used.
func (r *TokenResolver) tokenFromPlugin(ctx context.Context, plugin *types.PluginV1) (azcore.TokenCredential, error) {
	entraSettings := plugin.Spec.GetEntraId()
	if entraSettings == nil || entraSettings.SyncSettings == nil {
		return nil, trace.BadParameter("invalid Entra settings on plugin")
	}

	switch entraSettings.SyncSettings.CredentialsSource {
	case types.EntraIDCredentialsSource_ENTRAID_CREDENTIALS_SOURCE_SYSTEM_CREDENTIALS:
		return r.NewSystemCredential(nil)
	case types.EntraIDCredentialsSource_ENTRAID_CREDENTIALS_SOURCE_OIDC,
		types.EntraIDCredentialsSource_ENTRAID_CREDENTIALS_SOURCE_UNKNOWN:
		integration, err := r.Integrations.GetIntegration(ctx, plugin.GetName())
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return r.tokenFromIntegration(integration)
	default:
		return nil, trace.BadParameter("unknown credentials source: %v", entraSettings.SyncSettings.CredentialsSource)
	}
}

// tokenFromIntegration returns a token credential for the given integration.
func (r *TokenResolver) tokenFromIntegration(integration types.Integration) (azcore.TokenCredential, error) {
	if r.TokenGenerator == nil {
		return nil, trace.BadParameter("tokenGenerator is required for Azure OIDC integration credentials")
	}

	spec := integration.GetAzureOIDCIntegrationSpec()
	if spec == nil {
		return nil, trace.BadParameter("integration %s isn't an Azure OIDC integration", integration.GetName())
	}

	return azidentity.NewClientAssertionCredential(spec.TenantID, spec.ClientID, r.TokenGenerator, nil)
}

// tokenFromOAuthCredentials returns a token credential for the given OAuth credentials.
func (r *TokenResolver) tokenFromConnectorCredentials(creds *types.SAMLConnectorCredentials, assertionInfo *saml2.AssertionInfo) (azcore.TokenCredential, error) {
	if creds == nil {
		return nil, trace.BadParameter("no credentials set for connector")
	}

	if assertionInfo == nil {
		return nil, trace.BadParameter("no assertion info")
	}

	// NOTE: The tenant ID is extracted after the SAML assertion has been validated in the login flow
	// so we know it's from trusted source.
	tenantIDAttr, ok := assertionInfo.Values[entraIDAttrTenantID]
	if !ok || len(tenantIDAttr.Values) == 0 {
		return nil, trace.BadParameter("no tenant ID present on the assertion")
	}

	tenantID := tenantIDAttr.Values[0].Value

	switch {
	case creds.Oauth != nil:
		return azidentity.NewClientSecretCredential(
			tenantID,
			creds.Oauth.ClientId,
			creds.Oauth.ClientSecret,
			&azidentity.ClientSecretCredentialOptions{},
		)
	default:
		return nil, trace.BadParameter("unknown connector credentials type")
	}
}
