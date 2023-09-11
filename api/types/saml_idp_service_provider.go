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

// String returns the SAML IdP service provider string representation.
func (s *SAMLIdPServiceProviderV1) String() string {
	return fmt.Sprintf("SAMLIdPServiceProviderV1(Name=%v)",
		s.GetName())
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
		return trace.BadParameter("missing entity descriptor")
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
