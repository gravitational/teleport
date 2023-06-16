package ui

// CreateSAMLIdPServiceProviderRequest is the request from the UI to create a `saml_idp_service_provider`.
type CreateSAMLIdPServiceProviderRequest struct {
	// Name is the friendly name of the service provider.
	Name string `json:"name,omitempty"`
	// EntityDescriptor is the XML content of the Entity Descriptor of the service provider.
	EntityDescriptor string `json:"entityDescriptor,omitempty"`
}
