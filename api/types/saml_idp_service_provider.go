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
	io "io"
	"strings"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/utils"
)

// SAMLIdPServiceProvider specifies configuration for service providers for Teleport's built in SAML IdP.
type SAMLIdPServiceProvider interface {
	ResourceWithLabels
	// GetEntityDescriptor returns the entity descriptor of the service provider.
	GetEntityDescriptor() string
	// SetEntityDescriptor sets the entity descriptor of the service provider.
	SetEntityDescriptor(string)
}

// NewSAMLIdPServiceProvider returns a new SAMLIdPServiceProvider based off a name and SAMLIdPServiceProviderSpecV1.
func NewSAMLIdPServiceProvider(name string, spec SAMLIdPServiceProviderSpecV1) (SAMLIdPServiceProvider, error) {
	s := &SAMLIdPServiceProviderV1{
		Metadata: Metadata{
			Name: name,
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

// GetVersion returns resource version
func (s *SAMLIdPServiceProviderV1) GetVersion() string {
	return s.Version
}

// GetKind returns resource kind
func (s *SAMLIdPServiceProviderV1) GetKind() string {
	return s.Kind
}

// GetSubKind returns resource sub kind
func (s *SAMLIdPServiceProviderV1) GetSubKind() string {
	return s.SubKind
}

// SetSubKind sets resource subkind
func (s *SAMLIdPServiceProviderV1) SetSubKind(sk string) {
	s.SubKind = sk
}

// GetResourceID returns resource ID
func (s *SAMLIdPServiceProviderV1) GetResourceID() int64 {
	return s.Metadata.ID
}

// SetResourceID sets resource ID
func (s *SAMLIdPServiceProviderV1) SetResourceID(id int64) {
	s.Metadata.ID = id
}

// GetMetadata returns object metadata
func (s *SAMLIdPServiceProviderV1) GetMetadata() Metadata {
	return s.Metadata
}

// Origin returns the origin value of the resource.
func (s *SAMLIdPServiceProviderV1) Origin() string {
	return s.Metadata.Origin()
}

// SetOrigin sets the origin value of the resource.
func (s *SAMLIdPServiceProviderV1) SetOrigin(origin string) {
	s.Metadata.SetOrigin(origin)
}

// SetExpiry sets expiry time for the object
func (s *SAMLIdPServiceProviderV1) SetExpiry(expires time.Time) {
	s.Metadata.SetExpiry(expires)
}

// Expiry returns object expiry setting
func (s *SAMLIdPServiceProviderV1) Expiry() time.Time {
	return s.Metadata.Expiry()
}

// GetName returns the name of the service provider
func (s *SAMLIdPServiceProviderV1) GetName() string {
	return s.Metadata.GetName()
}

// SetName sets the name
func (s *SAMLIdPServiceProviderV1) SetName(name string) {
	s.Metadata.SetName(name)
}

// String returns the SAML IdP service provider string representation.
func (s *SAMLIdPServiceProviderV1) String() string {
	return fmt.Sprintf("SAMLIdPServiceProviderV1(Name=%v)",
		s.GetName())
}

// GetStaticLabels returns the service provider static labels.
func (s *SAMLIdPServiceProviderV1) GetStaticLabels() map[string]string {
	return s.Metadata.Labels
}

// SetStaticLabels sets the SAML IdP service provider static labels.
func (s *SAMLIdPServiceProviderV1) SetStaticLabels(sl map[string]string) {
	s.Metadata.Labels = sl
}

// GetAllLabels returns all labels from the service provider.
func (s *SAMLIdPServiceProviderV1) GetAllLabels() map[string]string {
	return s.Metadata.Labels
}

// MatchSearch goes through select field values and tries to
// match against the list of search values.
func (s *SAMLIdPServiceProviderV1) MatchSearch(values []string) bool {
	fieldVals := append(utils.MapToStrings(s.GetAllLabels()), s.GetName())
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

	if strings.TrimSpace(s.Spec.EntityDescriptor) == "" {
		return trace.BadParameter("missing entity descriptor")
	}

	// Validate the entity descriptor is valid XML. Ideally this would
	// validate the XML against the SAML schema
	// (https://docs.oasis-open.org/security/saml/v2.0/saml-schema-metadata-2.0.xsd),
	// but there doesn't appear to be a good XSD library for go.
	decoder := xml.NewDecoder(strings.NewReader(s.Spec.EntityDescriptor))
	readAnyXML := false
	for {
		err := decoder.Decode(new(interface{}))
		if err != nil {
			if err == io.EOF {
				if !readAnyXML {
					return trace.BadParameter("XML appears to be invalid")
				}
				break
			}
			return trace.Wrap(err)
		}
		readAnyXML = true
	}
	return nil
}

// SAMLIdPServiceProviders is a list of SAML IdP service provider resources.
type SAMLIdPServiceProviders []SAMLIdPServiceProvider

// AsResources returns these service providers as resources with labels.
func (s SAMLIdPServiceProviders) AsResources() (resources ResourcesWithLabels) {
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
