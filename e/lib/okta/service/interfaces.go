package oktaservice

import (
	oktav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/okta/v1"
	"github.com/gravitational/teleport/api/types"
)

// requestWithCredentials represents a okta/v1 GRPC request for which Okta client can be created.
// For *oktav1.UpdateIntegrationRequest use [newUpdateIntegrationRequestWithOrgURL].
type requestWithCredentials interface {
	GetApiCredentials() *oktav1.OktaAPICredentials
	GetOktaOrganizationUrl() string
}

// updateIntegrationRequestWithOrgURL implements [requestWithCredentials] for
// *oktav1.UpdateIntegrationRequest.
type updateIntegrationRequestWithOrgURL struct {
	*oktav1.UpdateIntegrationRequest
	orgURL string
}

// newUpdateIntegrationRequestWithOrgURL creates updateIntegrationRequestWithCredentials.
func newUpdateIntegrationRequestWithOrgURL(
	req *oktav1.UpdateIntegrationRequest,
	plugin *types.PluginV1,
) *updateIntegrationRequestWithOrgURL {
	return &updateIntegrationRequestWithOrgURL{
		UpdateIntegrationRequest: req,
		orgURL:                   plugin.Spec.GetOkta().OrgUrl,
	}
}

// GetOktaOrganizationUrl implements the missing part of [requestWithCredentials] for
// *oktav1.UpdateIntegrationRequest.
func (r *updateIntegrationRequestWithOrgURL) GetOktaOrganizationUrl() string {
	return r.orgURL
}
