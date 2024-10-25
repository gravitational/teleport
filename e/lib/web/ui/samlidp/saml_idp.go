package samlidp

import (
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
)

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
	// Preset is used to define service provider profile that will have a custom behavior
	// processed by Teleport.
	Preset string `json:"preset,omitempty"`
	// Labels are SAML service provider resource metadata labels.
	Labels map[string]string `json:"labels,omitempty"`
}

// TransformToProtoType transforms SAMLIdPServiceProvider to
// proto SAMLIdPServiceProviderV1.
func TransformToProtoType(req CreateSAMLIdPServiceProviderRequest) (*types.SAMLIdPServiceProviderV1, error) {
	sp := &types.SAMLIdPServiceProviderV1{
		ResourceHeader: types.ResourceHeader{
			Metadata: types.Metadata{
				Name:   req.Name,
				Labels: req.Labels,
			},
		},
		Spec: types.SAMLIdPServiceProviderSpecV1{
			EntityDescriptor: req.EntityDescriptor,
			EntityID:         req.EntityID,
			ACSURL:           req.ACSURL,
			AttributeMapping: req.AttributeMapping,
			Preset:           req.Preset,
		},
	}
	if err := sp.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return sp, nil
}
