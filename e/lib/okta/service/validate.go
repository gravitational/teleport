package oktaservice

import (
	"github.com/gravitational/trace"
)

func validateCredential(in oktaAuthConfigGetter) error {
	if in.GetOktaOrganizationUrl() == "" {
		return trace.BadParameter("missing Okta organization URL")
	}
	if in.GetApiCredentials().GetSswsBearerToken() == "" {
		return trace.BadParameter("missing Okta API credentials")
	}
	return nil
}
