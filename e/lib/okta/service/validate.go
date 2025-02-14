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
func validateCreateIntegrationRequest(req *oktapb.CreateIntegrationRequest) error {
	var err error
	if req.GetOktaOrganizationUrl() != "" {
		req.OktaOrganizationUrl, err = validateAndSanitizeUrl(req.GetOktaOrganizationUrl())
		if err != nil {
			return trace.Wrap(err)
		}
	}
	if req.GetSsoMetadataUrl() != "" {
		req.SsoMetadataUrl, err = validateAndSanitizeUrl(req.GetSsoMetadataUrl())
		if err != nil {
			return trace.Wrap(err)
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

	syncEnabled := req.EnableAccessListSync || req.EnableUserSync || req.EnableAppGroupSync
	pluginHasCredentials := oktaSettings.CredentialsInfo.HasSsmToken || oktaSettings.CredentialsInfo.HasOauthCredentials

	if syncEnabled && req.ApiCredentials == nil && !pluginHasCredentials {
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
