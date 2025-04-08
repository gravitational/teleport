/*
Copyright 2020-2021 Gravitational, Inc.

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
	"slices"
	"strings"
	"time"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/utils"
)

// SAMLConnector specifies configuration for SAML 2.0 identity providers
type SAMLConnector interface {
	// ResourceWithSecrets provides common methods for objects
	ResourceWithSecrets
	ResourceWithOrigin

	// SetMetadata sets the connector metadata
	SetMetadata(Metadata)
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
	// SetIssuer sets issuer
	SetIssuer(issuer string)
	// GetIssuer returns issuer
	GetIssuer() string
	// GetSigningKeyPair returns signing key pair
	GetSigningKeyPair() *AsymmetricKeyPair
	// GetSigningKeyPair sets signing key pair
	SetSigningKeyPair(k *AsymmetricKeyPair)
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
	// GetAllowIDPInitiated returns whether the identity provider can initiate a login or not.
	GetAllowIDPInitiated() bool
	// SetAllowIDPInitiated sets whether the identity provider can initiate a login or not.
	SetAllowIDPInitiated(bool)
	// GetClientRedirectSettings returns the client redirect settings.
	GetClientRedirectSettings() *SSOClientRedirectSettings
	// GetSingleLogoutURL returns the SAML SLO (single logout) URL for the identity provider.
	GetSingleLogoutURL() string
	// SetSingleLogoutURL sets the SAML SLO (single logout) URL for the identity provider.
	SetSingleLogoutURL(string)
}

// NewSAMLConnector returns a new SAMLConnector based off a name and SAMLConnectorSpecV2.
func NewSAMLConnector(name string, spec SAMLConnectorSpecV2) (SAMLConnector, error) {
	o := &SAMLConnectorV2{
		Metadata: Metadata{
			Name: name,
		},
		Spec: spec,
	}
	if err := o.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return o, nil
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

// GetRevision returns the revision
func (o *SAMLConnectorV2) GetRevision() string {
	return o.Metadata.GetRevision()
}

// SetRevision sets the revision
func (o *SAMLConnectorV2) SetRevision(rev string) {
	o.Metadata.SetRevision(rev)
}

// WithoutSecrets returns an instance of resource without secrets.
func (o *SAMLConnectorV2) WithoutSecrets() Resource {
	k1 := o.GetSigningKeyPair()
	k2 := o.GetEncryptionKeyPair()
	o2 := *o
	if k1 != nil {
		q1 := *k1
		q1.PrivateKey = ""
		o2.SetSigningKeyPair(&q1)
	}
	if k2 != nil {
		q2 := *k2
		q2.PrivateKey = ""
		o2.SetEncryptionKeyPair(&q2)
	}
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

// SetDisplay sets friendly name for this provider.
func (o *SAMLConnectorV2) SetDisplay(display string) {
	o.Spec.Display = display
}

// GetMetadata returns object metadata
func (o *SAMLConnectorV2) GetMetadata() Metadata {
	return o.Metadata
}

// SetMetadata sets object metadata
func (o *SAMLConnectorV2) SetMetadata(m Metadata) {
	o.Metadata = m
}

// Origin returns the origin value of the resource.
func (o *SAMLConnectorV2) Origin() string {
	return o.Metadata.Origin()
}

// SetOrigin sets the origin value of the resource.
func (o *SAMLConnectorV2) SetOrigin(origin string) {
	o.Metadata.SetOrigin(origin)
}

// SetExpiry sets expiry time for the object
func (o *SAMLConnectorV2) SetExpiry(expires time.Time) {
	o.Metadata.SetExpiry(expires)
}

// Expiry returns object expiry setting
func (o *SAMLConnectorV2) Expiry() time.Time {
	return o.Metadata.Expiry()
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

// GetAllowIDPInitiated returns whether the identity provider can initiate a login or not.
func (o *SAMLConnectorV2) GetAllowIDPInitiated() bool {
	return o.Spec.AllowIDPInitiated
}

// SetAllowIDPInitiated sets whether the identity provider can initiate a login or not.
func (o *SAMLConnectorV2) SetAllowIDPInitiated(allow bool) {
	o.Spec.AllowIDPInitiated = allow
}

// GetClientRedirectSettings returns the client redirect settings.
func (o *SAMLConnectorV2) GetClientRedirectSettings() *SSOClientRedirectSettings {
	if o == nil {
		return nil
	}
	return o.Spec.ClientRedirectSettings
}

// GetSingleLogoutURL returns the SAML SLO (single logout) URL for the identity provider.
func (o *SAMLConnectorV2) GetSingleLogoutURL() string {
	return o.Spec.SingleLogoutURL
}

// SetSingleLogoutURL sets the SAML SLO (single logout) URL for the identity provider.
func (o *SAMLConnectorV2) SetSingleLogoutURL(url string) {
	o.Spec.SingleLogoutURL = url
}

// setStaticFields sets static resource header and metadata fields.
func (o *SAMLConnectorV2) setStaticFields() {
	o.Kind = KindSAMLConnector
	o.Version = V2
}

// CheckAndSetDefaults checks and sets default values
func (o *SAMLConnectorV2) CheckAndSetDefaults() error {
	o.setStaticFields()
	if err := o.Metadata.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	if name := o.Metadata.Name; slices.Contains(constants.SystemConnectors, name) {
		return trace.BadParameter("ID: invalid connector name, %v is a reserved name", name)
	}
	if o.Spec.AssertionConsumerService == "" {
		return trace.BadParameter("missing acs - assertion consumer service parameter, set service URL that will receive POST requests from SAML")
	}
	if o.Spec.AllowIDPInitiated && !strings.HasSuffix(o.Spec.AssertionConsumerService, "/"+o.Metadata.Name) {
		return trace.BadParameter("acs - assertion consumer service parameter must end with /%v when allow_idp_initiated is set to true, eg https://cluster.domain/webapi/v1/saml/acs/%v. Ensure this URI matches the one configured at the identity provider.", o.Metadata.Name, o.Metadata.Name)
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

// Check returns nil if all parameters are great, err otherwise
func (i *SAMLAuthRequest) Check() error {
	if i.ConnectorID == "" {
		return trace.BadParameter("ConnectorID: missing value")
	}
	if len(i.PublicKey) != 0 {
		_, _, _, _, err := ssh.ParseAuthorizedKey(i.PublicKey)
		if err != nil {
			return trace.BadParameter("PublicKey: bad key: %v", err)
		}
		if (i.CertTTL > defaults.MaxCertDuration) || (i.CertTTL < defaults.MinCertDuration) {
			return trace.BadParameter("CertTTL: wrong certificate TTL")
		}
	}

	// we could collapse these two checks into one, but the error message would become ambiguous.
	if i.SSOTestFlow && i.ConnectorSpec == nil {
		return trace.BadParameter("ConnectorSpec cannot be nil when SSOTestFlow is true")
	}

	if !i.SSOTestFlow && i.ConnectorSpec != nil {
		return trace.BadParameter("ConnectorSpec must be nil when SSOTestFlow is false")
	}

	return nil
}
