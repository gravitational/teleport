package oktaservice

import (
	oktav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/okta/v1"
)

// requestWithCredentials represents a okta/v1 GRPC request for which Okta client can be created.
type requestWithCredentials interface {
	GetApiCredentials() *oktav1.OktaAPICredentials
	GetOktaOrganizationUrl() string
}
