/*
Copyright 2020 Gravitational, Inc.

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
	"time"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/utils"

	"github.com/gravitational/trace"
)

// SAMLConnector specifies configuration for SAML 2.0 identity providers
type SAMLConnector interface {
	// ResourceWithSecrets provides common methods for objects
	ResourceWithSecrets
	// GetDisplay returns display - friendly name for this provider.
	GetDisplay() string
	// SetDisplay sets friendly name for this provider.
	SetDisplay(string)
	// GetAttributesToRoles returns attributes to roles mapping
	GetAttributesToRoles() []AttributeMapping
	// SetAttributesToRoles sets attributes to roles mapping
	SetAttributesToRoles(mapping []AttributeMapping)
	// GetAttributes returns list of attributes expected by mappings
	GetAttributes() []string
	// GetTraitMappings converts gets all attribute mappings in the
	// generic trait mapping format.
	GetTraitMappings() TraitMappingSet
	// Check checks SAML connector for errors
	CheckAndSetDefaults() error
	// SetIssuer sets issuer
	SetIssuer(issuer string)
	// GetIssuer returns issuer
	GetIssuer() string
	// GetSigningKeyPair returns signing key pair
	GetSigningKeyPair() *AsymmetricKeyPair
	// GetSigningKeyPair sets signing key pair
	SetSigningKeyPair(k *AsymmetricKeyPair)
	// Equals returns true if the connectors are identical
	Equals(other SAMLConnector) bool
	// GetSSO returns SSO service
	GetSSO() string
	// SetSSO sets SSO service
	SetSSO(string)
	// GetEntityDescriptor returns XML entity descriptor of the service
	GetEntityDescriptor() string
	// SetEntityDescriptor sets entity descriptor of the service
	SetEntityDescriptor(v string)
	// GetEntityDescriptorURL returns the URL to obtain the entity descriptor.
	GetEntityDescriptorURL() string
	// SetEntityDescriptorURL sets the entity descriptor url.
	SetEntityDescriptorURL(string)
	// GetCert returns identity provider checking x509 certificate
	GetCert() string
	// SetCert sets identity provider checking certificate
	SetCert(string)
	// GetServiceProviderIssuer returns service provider issuer
	GetServiceProviderIssuer() string
	// SetServiceProviderIssuer sets service provider issuer
	SetServiceProviderIssuer(v string)
	// GetAudience returns audience
	GetAudience() string
	// SetAudience sets audience
	SetAudience(v string)
	// GetAssertionConsumerService returns assertion consumer service URL
	GetAssertionConsumerService() string
	// SetAssertionConsumerService sets assertion consumer service URL
	SetAssertionConsumerService(v string)
	// GetProvider returns the identity provider.
	GetProvider() string
	// SetProvider sets the identity provider.
	SetProvider(string)
	// GetEncryptionKeyPair returns the key pair for SAML assertions.
	GetEncryptionKeyPair() *AsymmetricKeyPair
	// SetEncryptionKeyPair sets the key pair for SAML assertions.
	SetEncryptionKeyPair(k *AsymmetricKeyPair)
}

// NewSAMLConnector returns a new SAMLConnector based off a name and SAMLConnectorSpecV2.
func NewSAMLConnector(name string, spec SAMLConnectorSpecV2) SAMLConnector {
	return &SAMLConnectorV2{
		Kind:    KindSAMLConnector,
		Version: V2,
		Metadata: Metadata{
			Name:      name,
			Namespace: defaults.Namespace,
		},
		Spec: spec,
	}
}

// SAMLConnectorV2 is version 1 resource spec for SAML connector
type SAMLConnectorV2 struct {
	// Kind is a resource kind
	Kind string `json:"kind"`
	// SubKind is a resource sub kind
	SubKind string `json:"sub_kind,omitempty"`
	// Version is version
	Version string `json:"version"`
	// Metadata is connector metadata
	Metadata Metadata `json:"metadata"`
	// Spec contains connector specification
	Spec SAMLConnectorSpecV2 `json:"spec"`
}

// GetVersion returns resource version
func (o *SAMLConnectorV2) GetVersion() string {
	return o.Version
}

// GetKind returns resource kind
func (o *SAMLConnectorV2) GetKind() string {
	return o.Kind
}

// GetSubKind returns resource sub kind
func (o *SAMLConnectorV2) GetSubKind() string {
	return o.SubKind
}

// SetSubKind sets resource subkind
func (o *SAMLConnectorV2) SetSubKind(sk string) {
	o.SubKind = sk
}

// GetResourceID returns resource ID
func (o *SAMLConnectorV2) GetResourceID() int64 {
	return o.Metadata.ID
}

// SetResourceID sets resource ID
func (o *SAMLConnectorV2) SetResourceID(id int64) {
	o.Metadata.ID = id
}

// WithoutSecrets returns an instance of resource without secrets.
func (o *SAMLConnectorV2) WithoutSecrets() Resource {
	k := o.GetSigningKeyPair()
	if k == nil {
		return o
	}
	k2 := *k
	k2.PrivateKey = ""
	o2 := *o
	o2.SetSigningKeyPair(&k2)
	return &o2
}

// GetServiceProviderIssuer returns service provider issuer
func (o *SAMLConnectorV2) GetServiceProviderIssuer() string {
	return o.Spec.ServiceProviderIssuer
}

// SetServiceProviderIssuer sets service provider issuer
func (o *SAMLConnectorV2) SetServiceProviderIssuer(v string) {
	o.Spec.ServiceProviderIssuer = v
}

// GetAudience returns audience
func (o *SAMLConnectorV2) GetAudience() string {
	return o.Spec.Audience
}

// SetAudience sets audience
func (o *SAMLConnectorV2) SetAudience(v string) {
	o.Spec.Audience = v
}

// GetCert returns identity provider checking x509 certificate
func (o *SAMLConnectorV2) GetCert() string {
	return o.Spec.Cert
}

// SetCert sets identity provider checking certificate
func (o *SAMLConnectorV2) SetCert(cert string) {
	o.Spec.Cert = cert
}

// GetSSO returns SSO service
func (o *SAMLConnectorV2) GetSSO() string {
	return o.Spec.SSO
}

// SetSSO sets SSO service
func (o *SAMLConnectorV2) SetSSO(sso string) {
	o.Spec.SSO = sso
}

// GetEntityDescriptor returns XML entity descriptor of the service
func (o *SAMLConnectorV2) GetEntityDescriptor() string {
	return o.Spec.EntityDescriptor
}

// SetEntityDescriptor sets entity descriptor of the service
func (o *SAMLConnectorV2) SetEntityDescriptor(v string) {
	o.Spec.EntityDescriptor = v
}

// GetEntityDescriptorURL returns the URL to obtain the entity descriptor.
func (o *SAMLConnectorV2) GetEntityDescriptorURL() string {
	return o.Spec.EntityDescriptorURL
}

// SetEntityDescriptorURL sets the entity descriptor url.
func (o *SAMLConnectorV2) SetEntityDescriptorURL(v string) {
	o.Spec.EntityDescriptorURL = v
}

// GetAssertionConsumerService returns assertion consumer service URL
func (o *SAMLConnectorV2) GetAssertionConsumerService() string {
	return o.Spec.AssertionConsumerService
}

// SetAssertionConsumerService sets assertion consumer service URL
func (o *SAMLConnectorV2) SetAssertionConsumerService(v string) {
	o.Spec.AssertionConsumerService = v
}

// Equals returns true if the connectors are identical
func (o *SAMLConnectorV2) Equals(other SAMLConnector) bool {
	if o.GetName() != other.GetName() {
		return false
	}
	if o.GetCert() != other.GetCert() {
		return false
	}
	if o.GetAudience() != other.GetAudience() {
		return false
	}
	if o.GetEntityDescriptor() != other.GetEntityDescriptor() {
		return false
	}
	if o.Expiry() != other.Expiry() {
		return false
	}
	if o.GetIssuer() != other.GetIssuer() {
		return false
	}
	if (o.GetSigningKeyPair() == nil && other.GetSigningKeyPair() != nil) || (o.GetSigningKeyPair() != nil && other.GetSigningKeyPair() == nil) {
		return false
	}
	if o.GetSigningKeyPair() != nil {
		a, b := o.GetSigningKeyPair(), other.GetSigningKeyPair()
		if a.Cert != b.Cert || a.PrivateKey != b.PrivateKey {
			return false
		}
	}
	mappings := o.GetAttributesToRoles()
	otherMappings := other.GetAttributesToRoles()
	if len(mappings) != len(otherMappings) {
		return false
	}
	for i := range mappings {
		a, b := mappings[i], otherMappings[i]
		if a.Name != b.Name || a.Value != b.Value || !utils.StringSlicesEqual(a.Roles, b.Roles) {
			return false
		}
	}
	return o.GetSSO() == other.GetSSO()
}

// SetDisplay sets friendly name for this provider.
func (o *SAMLConnectorV2) SetDisplay(display string) {
	o.Spec.Display = display
}

// GetMetadata returns object metadata
func (o *SAMLConnectorV2) GetMetadata() Metadata {
	return o.Metadata
}

// SetExpiry sets expiry time for the object
func (o *SAMLConnectorV2) SetExpiry(expires time.Time) {
	o.Metadata.SetExpiry(expires)
}

// Expiry returns object expiry setting
func (o *SAMLConnectorV2) Expiry() time.Time {
	return o.Metadata.Expiry()
}

// SetTTL sets Expires header using the provided clock.
// Use SetExpiry instead.
// DELETE IN 7.0.0
func (o *SAMLConnectorV2) SetTTL(clock Clock, ttl time.Duration) {
	o.Metadata.SetTTL(clock, ttl)
}

// GetName returns the name of the connector
func (o *SAMLConnectorV2) GetName() string {
	return o.Metadata.GetName()
}

// SetName sets client secret to some value
func (o *SAMLConnectorV2) SetName(name string) {
	o.Metadata.SetName(name)
}

// SetIssuer sets issuer
func (o *SAMLConnectorV2) SetIssuer(issuer string) {
	o.Spec.Issuer = issuer
}

// GetIssuer returns issuer
func (o *SAMLConnectorV2) GetIssuer() string {
	return o.Spec.Issuer
}

// GetDisplay returns the friendly name for this provider.
func (o *SAMLConnectorV2) GetDisplay() string {
	if o.Spec.Display != "" {
		return o.Spec.Display
	}
	return o.GetName()
}

// GetAttributesToRoles returns attributes to roles mapping
func (o *SAMLConnectorV2) GetAttributesToRoles() []AttributeMapping {
	return o.Spec.AttributesToRoles
}

// SetAttributesToRoles sets attributes to roles mapping
func (o *SAMLConnectorV2) SetAttributesToRoles(mapping []AttributeMapping) {
	o.Spec.AttributesToRoles = mapping
}

// SetProvider sets the identity provider.
func (o *SAMLConnectorV2) SetProvider(identityProvider string) {
	o.Spec.Provider = identityProvider
}

// GetProvider returns the identity provider.
func (o *SAMLConnectorV2) GetProvider() string {
	return o.Spec.Provider
}

// GetAttributes returns list of attributes expected by mappings
func (o *SAMLConnectorV2) GetAttributes() []string {
	var out []string
	for _, mapping := range o.Spec.AttributesToRoles {
		out = append(out, mapping.Name)
	}
	return utils.Deduplicate(out)
}

// GetTraitMappings returns the SAMLConnector's TraitMappingSet
func (o *SAMLConnectorV2) GetTraitMappings() TraitMappingSet {
	tms := make([]TraitMapping, 0, len(o.Spec.AttributesToRoles))
	for _, mapping := range o.Spec.AttributesToRoles {
		tms = append(tms, TraitMapping{
			Trait: mapping.Name,
			Value: mapping.Value,
			Roles: mapping.Roles,
		})
	}
	return TraitMappingSet(tms)
}

// GetSigningKeyPair returns signing key pair
func (o *SAMLConnectorV2) GetSigningKeyPair() *AsymmetricKeyPair {
	return o.Spec.SigningKeyPair
}

// SetSigningKeyPair sets signing key pair
func (o *SAMLConnectorV2) SetSigningKeyPair(k *AsymmetricKeyPair) {
	o.Spec.SigningKeyPair = k
}

// GetEncryptionKeyPair returns the key pair for SAML assertions.
func (o *SAMLConnectorV2) GetEncryptionKeyPair() *AsymmetricKeyPair {
	return o.Spec.EncryptionKeyPair
}

// SetEncryptionKeyPair sets the key pair for SAML assertions.
func (o *SAMLConnectorV2) SetEncryptionKeyPair(k *AsymmetricKeyPair) {
	o.Spec.EncryptionKeyPair = k
}

// CheckAndSetDefaults checks and sets default values
func (o *SAMLConnectorV2) CheckAndSetDefaults() error {
	if err := o.Metadata.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	if o.Metadata.Name == constants.Local {
		return trace.BadParameter("ID: invalid connector name, %v is a reserved name", constants.Local)
	}
	if o.Spec.AssertionConsumerService == "" {
		return trace.BadParameter("missing acs - assertion consumer service parameter, set service URL that will receive POST requests from SAML")
	}
	if o.Spec.ServiceProviderIssuer == "" {
		o.Spec.ServiceProviderIssuer = o.Spec.AssertionConsumerService
	}
	if o.Spec.Audience == "" {
		o.Spec.Audience = o.Spec.AssertionConsumerService
	}
	// Issuer and SSO can be automatically set later if EntityDescriptor is provided
	if o.Spec.EntityDescriptorURL == "" && o.Spec.EntityDescriptor == "" && (o.Spec.Issuer == "" || o.Spec.SSO == "") {
		return trace.BadParameter("no entity_descriptor set, either provide entity_descriptor or entity_descriptor_url in spec")
	}
	// make sure claim mappings have either roles or a role template
	for _, v := range o.Spec.AttributesToRoles {
		if len(v.Roles) == 0 {
			return trace.BadParameter("need roles field in attributes_to_roles")
		}
	}
	return nil
}

// SAMLConnectorSpecV2 specifies configuration for Open ID Connect compatible external
// identity provider, e.g. google in some organisation
type SAMLConnectorSpecV2 struct {
	// Issuer is identity provider issuer
	Issuer string `json:"issuer"`
	// SSO is URL of the identity provider SSO service
	SSO string `json:"sso"`
	// Cert is identity provider certificate PEM
	// IDP signs <Response> responses using this certificate
	Cert string `json:"cert"`
	// Display controls how this connector is displayed
	Display string `json:"display"`
	// AssertionConsumerService is a URL for assertion consumer service
	// on the service provider (Teleport's side)
	AssertionConsumerService string `json:"acs"`
	// Audience uniquely identifies our service provider
	Audience string `json:"audience"`
	// SertviceProviderIssuer is the issuer of the service provider (Teleport)
	ServiceProviderIssuer string `json:"service_provider_issuer"`
	// EntityDescriptor is XML with descriptor, can be used to supply configuration
	// parameters in one XML files vs supplying them in the individual elements
	EntityDescriptor string `json:"entity_descriptor"`
	// EntityDescriptor points to a URL that supplies a configuration XML.
	EntityDescriptorURL string `json:"entity_descriptor_url"`
	// AttriburesToRoles is a list of mappings of attribute statements to roles
	AttributesToRoles []AttributeMapping `json:"attributes_to_roles"`
	// SigningKeyPair is x509 key pair used to sign AuthnRequest
	SigningKeyPair *AsymmetricKeyPair `json:"signing_key_pair,omitempty"`
	// Provider is the external identity provider.
	Provider string `json:"provider,omitempty"`
	// EncryptionKeyPair is a key pair used for decryption of SAML assertions.
	EncryptionKeyPair *AsymmetricKeyPair `json:"assertion_key_pair,omitempty"`
}

// AttributeMapping is SAML Attribute statement mapping
// from SAML attribute statements to roles
type AttributeMapping struct {
	// Name is attribute statement name
	Name string `json:"name"`
	// Value is attribute statement value to match
	Value string `json:"value"`
	// Roles is a list of teleport roles to map to
	Roles []string `json:"roles,omitempty"`
}

// AsymmetricKeyPair is a combination of a public certificate and private key that
// can be used for encryption and signing.
type AsymmetricKeyPair struct {
	// PrivateKey is PEM encoded x509 private key
	PrivateKey string `json:"private_key"`
	// Cert is a PEM-encoded x509 certificate.
	Cert string `json:"cert"`
}
