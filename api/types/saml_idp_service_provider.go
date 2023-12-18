/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package types

import (
	"encoding/xml"
	"fmt"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/utils"
)

const (
	unspecifiedNameFormat = "urn:oasis:names:tc:SAML:2.0:attrname-format:unspecified"
	uriNameFormat         = "urn:oasis:names:tc:SAML:2.0:attrname-format:uri"
	basicNameFormat       = "urn:oasis:names:tc:SAML:2.0:attrname-format:basic"
)

var (
	// ErrMissingEntityDescriptorAndEntityID is returned when both entity descriptor and entity ID is empty.
	ErrEmptyEntityDescriptorAndEntityID = &trace.BadParameterError{Message: "either entity_descriptor or entity_id must be provided"}
	// ErrMissingEntityDescriptorAndACSURL is returned when both entity descriptor and ACS URL is empty.
	ErrEmptyEntityDescriptorAndACSURL = &trace.BadParameterError{Message: "either entity_descriptor or acs_url must be provided"}
	// ErrDuplicateAttributeName is returned when attribute mapping declares two or more
	// attributes with the same name.
	ErrDuplicateAttributeName = &trace.BadParameterError{Message: "duplicate attribute name not allowed"}
)

// SAMLIdPServiceProvider specifies configuration for service providers for Teleport's built in SAML IdP.
//
// Note: The EntityID is the entity ID for the entity descriptor. This ID is checked that it
// matches the entity ID in the entity descriptor at upsert time to avoid having to parse the
// XML blob in the entity descriptor every time we need to use this resource.
type SAMLIdPServiceProvider interface {
	ResourceWithLabels
	// GetEntityDescriptor returns the entity descriptor of the service provider.
	GetEntityDescriptor() string
	// SetEntityDescriptor sets the entity descriptor of the service provider.
	SetEntityDescriptor(string)
	// GetEntityID returns the entity ID.
	GetEntityID() string
	// SetEntityID sets the entity ID.
	SetEntityID(string)
	// GetACSURL returns the ACS URL.
	GetACSURL() string
	// SetACSURL sets the ACS URL.
	SetACSURL(string)
	// GetAttributeMapping returns Attribute Mapping.
	GetAttributeMapping() []*SAMLAttributeMapping
	// SetAttributeMapping sets Attribute Mapping.
	SetAttributeMapping([]*SAMLAttributeMapping)
	// Copy returns a copy of this saml idp service provider object.
	Copy() SAMLIdPServiceProvider
	// CloneResource returns a copy of the SAMLIdPServiceProvider as a ResourceWithLabels
	// This is helpful when interfacing with multiple types at the same time in unified resources
	CloneResource() ResourceWithLabels
}

// NewSAMLIdPServiceProvider returns a new SAMLIdPServiceProvider based off a metadata object and SAMLIdPServiceProviderSpecV1.
func NewSAMLIdPServiceProvider(metadata Metadata, spec SAMLIdPServiceProviderSpecV1) (SAMLIdPServiceProvider, error) {
	s := &SAMLIdPServiceProviderV1{
		ResourceHeader: ResourceHeader{
			Metadata: metadata,
		},
		Spec: spec,
	}

	if err := s.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return s, nil
}

// GetEntityDescriptor returns the entity descriptor.
func (s *SAMLIdPServiceProviderV1) GetEntityDescriptor() string {
	return s.Spec.EntityDescriptor
}

// SetEntityDescriptor sets the entity descriptor.
func (s *SAMLIdPServiceProviderV1) SetEntityDescriptor(entityDescriptor string) {
	s.Spec.EntityDescriptor = entityDescriptor
}

// GetEntityID returns the entity ID.
func (s *SAMLIdPServiceProviderV1) GetEntityID() string {
	return s.Spec.EntityID
}

// SetEntityID sets the entity ID.
func (s *SAMLIdPServiceProviderV1) SetEntityID(entityID string) {
	s.Spec.EntityID = entityID
}

// GetACSURL returns the ACS URL.
func (s *SAMLIdPServiceProviderV1) GetACSURL() string {
	return s.Spec.ACSURL
}

// SetACSURL sets the ACS URL.
func (s *SAMLIdPServiceProviderV1) SetACSURL(acsURL string) {
	s.Spec.ACSURL = acsURL
}

// GetAttributeMapping returns the Attribute Mapping.
func (s *SAMLIdPServiceProviderV1) GetAttributeMapping() []*SAMLAttributeMapping {
	return s.Spec.AttributeMapping
}

// SetAttributeMapping sets Attribute Mapping.
func (s *SAMLIdPServiceProviderV1) SetAttributeMapping(attrMaps []*SAMLAttributeMapping) {
	s.Spec.AttributeMapping = attrMaps
}

// String returns the SAML IdP service provider string representation.
func (s *SAMLIdPServiceProviderV1) String() string {
	return fmt.Sprintf("SAMLIdPServiceProviderV1(Name=%v)",
		s.GetName())
}

func (s *SAMLIdPServiceProviderV1) Copy() SAMLIdPServiceProvider {
	return utils.CloneProtoMsg(s)
}

func (s *SAMLIdPServiceProviderV1) CloneResource() ResourceWithLabels {
	return s.Copy()
}

// MatchSearch goes through select field values and tries to
// match against the list of search values.
func (s *SAMLIdPServiceProviderV1) MatchSearch(values []string) bool {
	fieldVals := append(utils.MapToStrings(s.GetAllLabels()), s.GetEntityID(), s.GetName(), staticSAMLIdPServiceProviderDescription)
	return MatchSearch(fieldVals, values, nil)
}

// setStaticFields sets static resource header and metadata fields.
func (s *SAMLIdPServiceProviderV1) setStaticFields() {
	s.Kind = KindSAMLIdPServiceProvider
	s.Version = V1
}

// CheckAndSetDefaults checks and sets default values
func (s *SAMLIdPServiceProviderV1) CheckAndSetDefaults() error {
	s.setStaticFields()
	if err := s.Metadata.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	if s.Spec.EntityDescriptor == "" {
		if s.Spec.EntityID == "" {
			return trace.Wrap(ErrEmptyEntityDescriptorAndEntityID)
		}

		if s.Spec.ACSURL == "" {
			return trace.Wrap(ErrEmptyEntityDescriptorAndACSURL)
		}

	}

	if s.Spec.EntityID == "" {
		// Extract just the entityID attribute from the descriptor
		ed := &struct {
			EntityID string `xml:"entityID,attr"`
		}{}
		err := xml.Unmarshal([]byte(s.Spec.EntityDescriptor), ed)
		if err != nil {
			return trace.Wrap(err)
		}

		s.Spec.EntityID = ed.EntityID
	}

	attrNames := make(map[string]struct{})
	for _, attributeMap := range s.GetAttributeMapping() {
		if err := attributeMap.CheckAndSetDefaults(); err != nil {
			return trace.Wrap(err)
		}
		// check for duplicate attribute names
		if _, ok := attrNames[attributeMap.Name]; ok {
			return trace.Wrap(ErrDuplicateAttributeName)
		}
		attrNames[attributeMap.Name] = struct{}{}
	}

	return nil
}

// SAMLIdPServiceProviders is a list of SAML IdP service provider resources.
type SAMLIdPServiceProviders []SAMLIdPServiceProvider

// AsResources returns these service providers as resources with labels.
func (s SAMLIdPServiceProviders) AsResources() ResourcesWithLabels {
	resources := make([]ResourceWithLabels, 0, len(s))
	for _, sp := range s {
		resources = append(resources, sp)
	}
	return resources
}

// Len returns the slice length.
func (s SAMLIdPServiceProviders) Len() int { return len(s) }

// Less compares service providers by name.
func (s SAMLIdPServiceProviders) Less(i, j int) bool { return s[i].GetName() < s[j].GetName() }

// Swap swaps two service providers.
func (s SAMLIdPServiceProviders) Swap(i, j int) { s[i], s[j] = s[j], s[i] }

// CheckAndSetDefaults check and sets SAMLAttributeMapping default values
func (am *SAMLAttributeMapping) CheckAndSetDefaults() error {
	// verify name format is one of the supported
	// formats - unspecifiedNameFormat, basicNameFormat or uriNameFormat
	// and assign it with the URN value of that format.
	switch am.NameFormat {
	case "", "unspecified", unspecifiedNameFormat:
		am.NameFormat = unspecifiedNameFormat
	case "basic", basicNameFormat:
		am.NameFormat = basicNameFormat
	case "uri", uriNameFormat:
		am.NameFormat = uriNameFormat
	default:
		return trace.BadParameter("invalid name format: %s", am.NameFormat)
	}
	return nil
}
