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

	"github.com/gravitational/teleport/api/types/samlsp"
	"github.com/gravitational/teleport/api/utils"
)

// The following name formats are defined in the SAML 2.0 Core OS Standard -
// https://docs.oasis-open.org/security/saml/v2.0/saml-core-2.0-os.pdf
const (
	// SAMLURINameFormat is an attribute name format that follows the convention for URI references [RFC 2396].
	SAMLURINameFormat = "urn:oasis:names:tc:SAML:2.0:attrname-format:uri"
	// SAMLBasicNameFormat is an attribute name format that specifies a simple string value.
	SAMLBasicNameFormat = "urn:oasis:names:tc:SAML:2.0:attrname-format:basic"
	// SAMLUnspecifiedNameFormat is an attribute name format for names that does not fall into Basic or URI category.
	SAMLUnspecifiedNameFormat = "urn:oasis:names:tc:SAML:2.0:attrname-format:unspecified"

	// SAMLStringType is a string value type.
	SAMLStringType = "xs:string"
)

// SAML Name ID formats.
// https://docs.oasis-open.org/security/saml/v2.0/saml-core-2.0-os.pdf.
const (
	// SAMLUnspecifiedNameIDFormat is a Name ID format of unknown type and it is upto the
	// service provider to interpret the format of the value. [Saml Core v2, 8.3.1]
	SAMLUnspecifiedNameIDFormat = "urn:oasis:names:tc:SAML:1.1:nameid-format:unspecified"
	// SAMLEmailAddressNameIDFormat is a Name ID format of email address type as specified
	// in IETF RFC 2822 [RFC 2822] Section 3.4.1. [Saml Core v2, 8.3.2]
	SAMLEmailAddressNameIDFormat = "urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress"
	// SAMLX509SubjectNameNameIDFormat is a Name ID format of the X.509 certificate
	// subject name which is used in XML Signature Recommendation (XMLSig). [Saml Core v2, 8.3.3].
	SAMLX509SubjectNameNameIDFormat = "urn:oasis:names:tc:SAML:1.1:nameid-format:X509SubjectName"
	// SAMLWindowsDomainQualifiedNameNameIDFormat is a Name ID format of Windows Domain Qualified
	// Name whose syntax "DomainName\UserName". [Saml Core v2, 8.3.4].
	SAMLWindowsDomainQualifiedNameNameIDFormat = "urn:oasis:names:tc:SAML:1.1:nameid-format:WindowsDomainQualifiedName"
	// SAMLKerberosPrincipalNameNameNameIDFormat is a Name ID format of Kerberos Principal Name
	// whose syntax is "name[/instance]@REALM". IETF RFC 1510 [RFC 1510]. [Saml Core v2, 8.3.5].
	SAMLKerberosPrincipalNameNameNameIDFormat = "urn:oasis:names:tc:SAML:2.0:nameid-format:kerberos"
	// SAMLEntityNameIDFormat is a Name ID format for SAML IdP Entity ID value. [Saml Core v2, 8.3.6].
	SAMLEntityNameIDFormat = "urn:oasis:names:tc:SAML:2.0:nameid-format:entity"
	// SAMLPersistentNameIDFormat is a Name ID format whose value is to be treated as a persistent
	// user identitifer by the service provider. [Saml Core v2, 8.3.7]
	SAMLPersistentNameIDFormat = "urn:oasis:names:tc:SAML:2.0:nameid-format:persistent"
	// SAMLTransientNameIDFormat is a Name ID format whose value is to be treated as a temporary value by the
	// service provider. [Saml Core v2, 8.3.8]
	SAMLTransientNameIDFormat = "urn:oasis:names:tc:SAML:2.0:nameid-format:transient"
)

const (
	// SAMLAuthnContextPublicKeyX509ClassRef is a Public Key X.509 reference authentication standard.
	// Defined in SAML 2.0 Authentication Context Standard -
	// https://docs.oasis-open.org/security/saml/v2.0/saml-authn-context-2.0-os.pdf
	SAMLAuthnContextPublicKeyX509ClassRef = "urn:oasis:names:tc:SAML:2.0:ac:classes:X509"

	// SAMLBearerMethod is a subject confirmation method, which tells the service provider
	// that the user in the context of authentication (the bearer of SAML assertion) lay claim to the SAML
	// assertion value. Defined in the SAML 2.0 Technical Overview -
	// http://docs.oasis-open.org/security/saml/Post2.0/sstc-saml-tech-overview-2.0-cd-02.pdf
	SAMLBearerMethod = "urn:oasis:names:tc:SAML:2.0:cm:bearer"

	// SAMLSubjectIDName is a general purpose subject identifier as defined in SAML Subject Indentifier Attribuets -
	// http://docs.oasis-open.org/security/saml-subject-id-attr/v1.0/csprd03/saml-subject-id-attr-v1.0-csprd03.pdf
	SAMLSubjectIDName = "urn:oasis:names:tc:SAML:attribute:subject-id"
)

const (
	// SAMLUIDFriendlyName is a user friendly name with a userid format as defiend in OID-info db -
	// http://www.oid-info.com/cgi-bin/display?oid=urn%3Aoid%3A0.9.2342.19200300.100.1.1&a=display
	SAMLUIDFriendlyName = "uid"
	// SAMLUIDName is a URN value of UIDFriendlyName.
	SAMLUIDName = "urn:oid:0.9.2342.19200300.100.1.1"
	// SAMLEduPersonAffiliationFriendlyName is used to reference groups associated with a user as
	// defiend in OID-info db - http://www.oid-info.com/cgi-bin/display?oid=urn%3Aoid%3A1.3.6.1.4.1.5923.1.1.1.1&a=display
	SAMLEduPersonAffiliationFriendlyName = "eduPersonAffiliation"
	// SAMLEduPersonAffiliationName is a URN value of EduPersonAffiliationFriendlyName.
	SAMLEduPersonAffiliationName = "urn:oid:1.3.6.1.4.1.5923.1.1.1.1"
)

var (
	// ErrMissingEntityDescriptorAndEntityID is returned when both entity descriptor and entity ID is empty.
	ErrEmptyEntityDescriptorAndEntityID = &trace.BadParameterError{Message: "either entity_descriptor or entity_id must be provided"}
	// ErrMissingEntityDescriptorAndACSURL is returned when both entity descriptor and ACS URL is empty.
	ErrEmptyEntityDescriptorAndACSURL = &trace.BadParameterError{Message: "either entity_descriptor or acs_url must be provided"}
	// ErrDuplicateAttributeName is returned when attribute mapping declares two or more
	// attributes with the same name.
	ErrDuplicateAttributeName = &trace.BadParameterError{Message: "duplicate attribute name not allowed"}
	// ErrUnsupportedPresetName is returned when preset name is not supported.
	ErrUnsupportedPresetName = &trace.BadParameterError{Message: "unsupported preset name"}
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
	// GetPreset returns the Preset.
	GetPreset() string
	// GetRelayState returns Relay State.
	GetRelayState() string
	// SetRelayState sets Relay State.
	SetRelayState(string)
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

// GetPreset returns the Preset.
func (s *SAMLIdPServiceProviderV1) GetPreset() string {
	return s.Spec.Preset
}

// GetRelayState returns Relay State.
func (s *SAMLIdPServiceProviderV1) GetRelayState() string {
	return s.Spec.RelayState
}

// SetRelayState sets Relay State.
func (s *SAMLIdPServiceProviderV1) SetRelayState(relayState string) {
	s.Spec.RelayState = relayState
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

	if ok := s.checkAndSetPresetDefaults(s.Spec.Preset); !ok {
		return trace.Wrap(ErrUnsupportedPresetName)
	}

	return nil
}

// validatePreset validates SAMLIdPServiceProviderV1 preset field.
// preset can be either empty or one of the supported type.
func (s *SAMLIdPServiceProviderV1) checkAndSetPresetDefaults(preset string) bool {
	switch preset {
	case "":
		return true
	case samlsp.GCPWorkforce:
		if s.GetRelayState() == "" {
			s.SetRelayState(samlsp.DefaultRelayStateGCPWorkforce)
		}
		return true
	default:
		return false
	}
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
	if am.Name == "" {
		return trace.BadParameter("attribute name is required")
	}
	if am.Value == "" {
		return trace.BadParameter("attribute value is required")
	}
	// verify name format is one of the supported
	// formats - unspecifiedNameFormat, basicNameFormat or uriNameFormat
	// and assign it with the URN value of that format.
	switch am.NameFormat {
	case "", "unspecified", SAMLUnspecifiedNameFormat:
		am.NameFormat = SAMLUnspecifiedNameFormat
	case "basic", SAMLBasicNameFormat:
		am.NameFormat = SAMLBasicNameFormat
	case "uri", SAMLURINameFormat:
		am.NameFormat = SAMLURINameFormat
	default:
		return trace.BadParameter("invalid name format: %s", am.NameFormat)
	}
	return nil
}
