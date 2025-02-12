package oktaservice

import (
	"net/url"

	"github.com/gravitational/trace"

	oktapb "github.com/gravitational/teleport/api/gen/proto/go/teleport/okta/v1"
	"github.com/gravitational/teleport/api/types"
)

// TODO(kopiczko) will be merged with https://github.com/gravitational/teleport.e/pull/5756/

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
	u, err := url.Parse(urlStr)
	if err != nil {
		return "", trace.BadParameter("invalid URL: %v", err)
	}
	if u.Scheme == "" {
		u.Scheme = "https"
	}
	if u.Scheme != "https" {
		return "", trace.BadParameter("required https scheme, but got %q", u.Scheme)
	}
	return u.String(), nil
}
