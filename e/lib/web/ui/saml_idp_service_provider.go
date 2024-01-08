package ui

import "github.com/gravitational/teleport/api/types"

// CreateSAMLIdPServiceProviderRequest is the request from the UI to create a `saml_idp_service_provider`.
type CreateSAMLIdPServiceProviderRequest struct {
	// Name is the friendly name of the service provider.
	Name string `json:"name,omitempty"`
	// EntityDescriptor is the XML content of the Entity Descriptor of the service provider.
	EntityDescriptor string `json:"entityDescriptor,omitempty"`
	// EntityID is the metadata endpoint of the service provider.
	EntityID string `json:"entityID,omitempty"`
	// ACSURL, also known as SSO URL, is the endpoint where users will be redirected
	// after a SAML authentication.
	ACSURL string `json:"acsURL,omitempty"`
	// AttributeMapping maps custom user attributes for SAML response.
	AttributeMapping []*types.SAMLAttributeMapping `json:"attributeMapping,omitempty"`
}
