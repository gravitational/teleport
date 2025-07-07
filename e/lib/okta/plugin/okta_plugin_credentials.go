package oktaplugin

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	oktacommon "github.com/gravitational/teleport/e/lib/okta/common"
)

// GetStaticCredentials retrieves static credentials for the Okta plugin.
func GetStaticCredentials(ctx context.Context, creds PluginStaticCredentialsService, ref *types.PluginStaticCredentialsRef) ([]types.PluginStaticCredentials, error) {
	if ref == nil {
		return nil, trace.NotFound("plugin has no static credentials ref")
	}
	if len(ref.Labels) == 0 {
		return nil, trace.NotFound("plugin credentials ref has no labels set")
	}
	staticCreds, err := creds.GetPluginStaticCredentialsByLabels(ctx, ref.Labels)
	if err != nil {
		return nil, trace.Wrap(err, "getting plugin static credentials for labels %v", ref.Labels)
	}
	return staticCreds, nil
}

// SelectedOktaCredentials is the result of [SelectOktaCredentials]. If both client ID and API
// token are not set, it means Okta credentials were not found in the plugin static credentials.
type SelectedOktaCredentials struct {
	// OauthClientId are preferred credentials and should be used over API token.
	OauthClientId string
	// ApiToken is present for legacy plugins which were not updated yet.
	ApiToken string
	// ApiTokenForSCIMOnly denotes that if the selected credentials are API token credentials,
	// they should be used only for SCIM integration.
	ApiTokenForSCIMOnly bool
}

// SelectOktaCredentials selects the preferred credentials for the Okta plugin.
func SelectOktaCredentials(staticCreds []types.PluginStaticCredentials) (SelectedOktaCredentials, error) {
	var selected SelectedOktaCredentials
	// Try OAuth Client ID.
	if clientId, ok, err := selectOktaOauthClientId(staticCreds); err != nil {
		return selected, trace.Wrap(err)
	} else if ok {
		selected.OauthClientId = clientId
		return selected, nil
	}
	// Try API token for SCIM-only integration.
	if apiToken, ok, err := selectOktaApiTokenForSCIM(staticCreds); err != nil {
		return selected, trace.Wrap(err)
	} else if ok {
		selected.ApiToken = apiToken
		selected.ApiTokenForSCIMOnly = true
		return selected, nil
	}
	// Try API token.
	if apiToken, ok, err := selectOktaApiToken(staticCreds); err != nil {
		return selected, trace.Wrap(err)
	} else if ok {
		selected.ApiToken = apiToken
		return selected, nil
	}
	// Try legacy API token.
	if apiToken, ok, err := selectOktaApiTokenLegacy(staticCreds); err != nil {
		return selected, trace.Wrap(err)
	} else if ok {
		selected.ApiToken = apiToken
		return selected, nil
	}
	return selected, nil
}

// SelectSCIMTokenHash searches the backend for SCIM bearer token configured for the plugin.
func SelectSCIMTokenHash(staticCreds []types.PluginStaticCredentials) (string, bool, error) {
	scimToken, ok, err := selectScimToken(staticCreds)
	return scimToken, ok, trace.Wrap(err)
}

func selectCredsByPurposeLabel(staticCreds []types.PluginStaticCredentials, purpose string) (types.PluginStaticCredentials, bool) {
	for _, cred := range staticCreds {
		if v, ok := cred.GetLabel(types.OktaCredPurposeLabel); ok && v == purpose {
			return cred, true
		}
	}
	return nil, false
}

func selectCredsWithoutPurposeLabel(staticCreds []types.PluginStaticCredentials) (types.PluginStaticCredentials, bool) {
	for _, cred := range staticCreds {
		if _, ok := cred.GetLabel(types.OktaCredPurposeLabel); !ok {
			return cred, true
		}
	}
	return nil, false
}

func selectOktaOauthClientId(staticCreds []types.PluginStaticCredentials) (string, bool, error) {
	selectedCreds, ok := selectCredsByPurposeLabel(staticCreds, oktacommon.CredPurposeOktaOauth)
	if !ok {
		return "", false, nil
	}
	clientId, _ /* clientSecret */ := selectedCreds.GetOAuthClientSecret()
	if clientId == "" {
		return "", false, trace.BadParameter("OAuth plugin static credentials found, but client ID is missing")
	}
	return clientId, true, nil
}

func selectOktaApiTokenForSCIM(staticCreds []types.PluginStaticCredentials) (string, bool, error) {
	// CredPurposeOktaAPITokenWithSCIMOnlyIntegration is set only when Okta app sync is
	// disabled. For backward compatibility, when Teleport is downgraded to a version that
	// doesn't support stopping app group sync via the feature flag (AppGroupSyncDisabled), we
	// will rely on the behavior of preventing starting the Okta Plugin due to the missing
	// credential.
	selectedCreds, ok := selectCredsByPurposeLabel(staticCreds, types.CredPurposeOKTAAPITokenWithSCIMOnlyIntegration)
	if !ok {
		return "", false, nil
	}
	apiToken := selectedCreds.GetAPIToken()
	if apiToken == "" {
		return "", false, trace.BadParameter("Okta SCIM-only API token plugin static credentials found, but API token is missing")
	}
	return apiToken, true, nil
}

func selectOktaApiToken(staticCreds []types.PluginStaticCredentials) (string, bool, error) {
	selectedCreds, ok := selectCredsByPurposeLabel(staticCreds, types.OktaCredPurposeAuth)
	if !ok {
		return "", false, nil
	}
	apiToken := selectedCreds.GetAPIToken()
	if apiToken == "" {
		return "", false, trace.BadParameter("API token plugin static credentials found, but API token is missing")
	}
	return apiToken, true, nil
}

func selectOktaApiTokenLegacy(staticCreds []types.PluginStaticCredentials) (string, bool, error) {
	// Older Okta API credentials are not labeled with a purpose, so a cred is considered
	// eligible if it has no purpose label.
	selectedCreds, ok := selectCredsWithoutPurposeLabel(staticCreds)
	if !ok {
		return "", false, nil
	}
	apiToken := selectedCreds.GetAPIToken()
	if apiToken == "" {
		return "", false, trace.BadParameter("legacy API token plugin static credentials found, but API token is missing")
	}
	return apiToken, true, nil
}

func selectScimToken(staticCreds []types.PluginStaticCredentials) (string, bool, error) {
	selectedCreds, ok := selectCredsByPurposeLabel(staticCreds, types.OktaCredPurposeSCIMToken)
	if !ok {
		return "", false, nil
	}
	scimToken := selectedCreds.GetAPIToken()
	if scimToken == "" {
		return "", false, trace.BadParameter("SCIM plugin static credentials found, but token is missing")
	}
	return scimToken, true, nil
}
