package oktaservice

import (
	"github.com/gravitational/trace"

	oktapb "github.com/gravitational/teleport/api/gen/proto/go/teleport/okta/v1"
)

type requestOktaCredsGetter interface {
	// GetOktaOrganizationUrl returns the Okta organization URL.
	GetOktaOrganizationUrl() string
	// GetApiCredentials returns the Okta API credentials.
	GetApiCredentials() *oktapb.OktaAPICredentials
}

func validateCredential(in requestOktaCredsGetter) error {
	if in.GetOktaOrganizationUrl() == "" {
		return trace.BadParameter("missing Okta organization URL")
	}

	if in.GetApiCredentials().GetSswsBearerToken() == "" && in.GetApiCredentials().GetOauthId() == "" {
		return trace.BadParameter("missing Okta API credentials")
	}
	return nil
}
