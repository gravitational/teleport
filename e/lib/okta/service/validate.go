package oktaservice

import (
	"net/url"
	"strings"

	"github.com/gravitational/trace"

	oktapb "github.com/gravitational/teleport/api/gen/proto/go/teleport/okta/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/okta/common/sso"
)

// validateCreateIntegrationRequest checks update integration request invariants for the plugin to
// be updated.
func validateCreateIntegrationRequest(req *oktapb.CreateIntegrationRequest, samlConnector types.SAMLConnector) error {
	var err error

	if req.GetOktaOrganizationUrl() != "" {
		req.OktaOrganizationUrl, err = validateAndSanitizeUrl(req.GetOktaOrganizationUrl())
		if err != nil {
			return trace.Wrap(err, "invalid Okta org URL")
		}
	}
	// Verify with or extract Okta org URL from SSO metadata URL.
	if req.GetSsoMetadataUrl() != "" {
		req.SsoMetadataUrl, err = validateAndSanitizeUrl(req.GetSsoMetadataUrl())
		if err != nil {
			return trace.Wrap(err, "invalid connector SSO metadata URL")
		}
		if req.GetOktaOrganizationUrl() == "" {
			req.OktaOrganizationUrl, err = sso.ExtractOktaOrganizationFromURL(req.GetSsoMetadataUrl())
			if err != nil {
				return trace.Wrap(err, "extracting Okta org URL from SSO metadata URL")
			}
		}
	}
	if req.GetOktaOrganizationUrl() != "" && req.GetSsoMetadataUrl() != "" {
		if !strings.HasPrefix(req.GetSsoMetadataUrl(), req.GetOktaOrganizationUrl()) {
			return trace.BadParameter("SSO metadata URL and Okta org URL have different hostnames")
		}
	}
	// Verify with or extract Okta org URL from SAML connector.
	if samlConnector != nil {
		ssoUrl, err := validateAndSanitizeUrl(samlConnector.GetSSO())
		if err != nil {
			return trace.Wrap(err, "connector %q has invalid SSO URL", samlConnector.GetName())
		}
		if req.GetOktaOrganizationUrl() == "" {
			req.OktaOrganizationUrl, err = sso.ExtractOktaOrganizationFromURL(ssoUrl)
			if err != nil {
				return trace.Wrap(err, "invalid SSO URL in connector %q", samlConnector.GetName())
			}
		}
		if !strings.HasPrefix(ssoUrl, req.GetOktaOrganizationUrl()) {
			return trace.BadParameter("SAML connector %q SSO URL and Okta org URL have different hostnames", samlConnector.GetName())
		}
	}
	// At this point Okta org URL must be known.
	if req.GetOktaOrganizationUrl() == "" {
		return trace.BadParameter("Okta org URL or SSO metadata URL missing")
	}

	if req.GetApiCredentials() == nil {
		// Credentials are required for access list sync, user sync, and group sync.
		// Otherwise, the plugin will not be able to fetch and sync required data.
		if req.GetEnableUserSync() {
			return trace.BadParameter("Okta API credentials are required for user sync")
		}
		if req.GetEnableAccessListSync() {
			return trace.BadParameter("Okta API credentials are required for access list sync")
		}
		if req.GetEnableAppGroupSync() {
			return trace.BadParameter("Okta API credentials are required for group sync")
		}
	}
	return nil
}

// validateUpdateIntegrationRequest checks update integration request invariants for the plugin to
// be updated.
func validateUpdateIntegrationRequest(req *oktapb.UpdateIntegrationRequest, plugin *types.PluginV1) error {
	oktaSettings := plugin.Spec.GetOkta()
	credentialsInfo := oktaSettings.GetCredentialsInfo()
	syncSettings := oktaSettings.GetSyncSettings()

	requestSyncEnabled := req.GetEnableAccessListSync() || req.GetEnableUserSync() || req.GetEnableAppGroupSync()

	pluginHasCredentials := false
	if credentialsInfo != nil {
		pluginHasCredentials = credentialsInfo.HasSsmToken || credentialsInfo.HasOauthCredentials
	}
	// This is the situation for the legacy plugins where sync is enabled but CredentialsInfo
	// is not always set. It can be assumed that the plugin has credentials if sync
	// (specifically user sync) is enabled.
	// TODO(kopiczko) implement plugin migration and remove it
	if syncSettings != nil && syncSettings.SyncUsers {
		pluginHasCredentials = true
	}

	if requestSyncEnabled && req.GetApiCredentials() == nil && !pluginHasCredentials {
		return trace.BadParameter("update integration request enables sync but does not provide API credentials, and the plugin has no Okta credentials configured")
	}

	return nil
}

func validateAndSanitizeUrl(urlStr string) (string, error) {
	if !strings.Contains(urlStr, "://") {
		urlStr = "https://" + urlStr
	}
	u, err := url.Parse(urlStr)
	if err != nil {
		return "", trace.BadParameter("invalid URL: %v", err)
	}
	if u.Hostname() == "" {
		return "", trace.BadParameter("hostname missing")
	}
	if u.Scheme != "https" {
		return "", trace.BadParameter("required https scheme, but got %q", u.Scheme)
	}
	return u.String(), nil
}
