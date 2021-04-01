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

package services

import (
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
	log "github.com/sirupsen/logrus"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	saml2 "github.com/russellhaering/gosaml2"
	"github.com/russellhaering/gosaml2/types"
	dsig "github.com/russellhaering/goxmldsig"
)

// ValidateSAMLConnector validates the SAMLConnector and sets default values
func ValidateSAMLConnector(sc SAMLConnector) error {
	if err := sc.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	if sc.GetEntityDescriptorURL() != "" {
		resp, err := http.Get(sc.GetEntityDescriptorURL())
		if err != nil {
			return trace.Wrap(err)
		}
		if resp.StatusCode != http.StatusOK {
			return trace.BadParameter("status code %v when fetching from %q", resp.StatusCode, sc.GetEntityDescriptorURL())
		}
		defer resp.Body.Close()
		body, err := utils.ReadAtMost(resp.Body, teleport.MaxHTTPResponseSize)
		if err != nil {
			return trace.Wrap(err)
		}
		sc.SetEntityDescriptor(string(body))
		log.Debugf("[SAML] Successfully fetched entity descriptor from %q", sc.GetEntityDescriptorURL())
	}

	if sc.GetEntityDescriptor() != "" {
		metadata := &types.EntityDescriptor{}
		if err := xml.Unmarshal([]byte(sc.GetEntityDescriptor()), metadata); err != nil {
			return trace.Wrap(err, "failed to parse entity_descriptor")
		}

		sc.SetIssuer(metadata.EntityID)
		if len(metadata.IDPSSODescriptor.SingleSignOnServices) > 0 {
			sc.SetSSO(metadata.IDPSSODescriptor.SingleSignOnServices[0].Location)
		}
	}

	if sc.GetIssuer() == "" {
		return trace.BadParameter("no issuer or entityID set, either set issuer as a parameter or via entity_descriptor spec")
	}
	if sc.GetSSO() == "" {
		return trace.BadParameter("no SSO set either explicitly or via entity_descriptor spec")
	}

	if sc.GetSigningKeyPair() == nil {
		keyPEM, certPEM, err := utils.GenerateSelfSignedSigningCert(pkix.Name{
			Organization: []string{"Teleport OSS"},
			CommonName:   "teleport.localhost.localdomain",
		}, nil, 10*365*24*time.Hour)
		if err != nil {
			return trace.Wrap(err)
		}
		sc.SetSigningKeyPair(&AsymmetricKeyPair{
			PrivateKey: string(keyPEM),
			Cert:       string(certPEM),
		})
	}

	log.Debugf("[SAML] SSO: %v", sc.GetSSO())
	log.Debugf("[SAML] Issuer: %v", sc.GetIssuer())
	log.Debugf("[SAML] ACS: %v", sc.GetAssertionConsumerService())

	return nil
}

// GetAttributeNames returns a list of claim names from the claim values
func GetAttributeNames(attributes map[string]types.Attribute) []string {
	var out []string
	for _, attr := range attributes {
		out = append(out, attr.Name)
	}
	return out
}

// SAMLAssertionsToTraits converts saml assertions to traits
func SAMLAssertionsToTraits(assertions saml2.AssertionInfo) map[string][]string {
	traits := make(map[string][]string, len(assertions.Values))
	for _, assr := range assertions.Values {
		vals := make([]string, 0, len(assr.Values))
		for _, value := range assr.Values {
			vals = append(vals, value.Value)
		}
		traits[assr.Name] = vals
	}
	return traits
}

// GetSAMLServiceProvider gets the SAMLConnector's service provider
func GetSAMLServiceProvider(sc SAMLConnector, clock clockwork.Clock) (*saml2.SAMLServiceProvider, error) {
	certStore := dsig.MemoryX509CertificateStore{
		Roots: []*x509.Certificate{},
	}

	if sc.GetEntityDescriptor() != "" {
		metadata := &types.EntityDescriptor{}
		if err := xml.Unmarshal([]byte(sc.GetEntityDescriptor()), metadata); err != nil {
			return nil, trace.Wrap(err, "failed to parse entity_descriptor")
		}

		for _, kd := range metadata.IDPSSODescriptor.KeyDescriptors {
			for _, samlCert := range kd.KeyInfo.X509Data.X509Certificates {
				certData, err := base64.StdEncoding.DecodeString(strings.TrimSpace(samlCert.Data))
				if err != nil {
					return nil, trace.Wrap(err, "failed to decode certificate defined in entity_descriptor")
				}
				cert, err := x509.ParseCertificate(certData)
				if err != nil {
					return nil, trace.Wrap(err, "failed to parse certificate defined in entity_descriptor")
				}
				certStore.Roots = append(certStore.Roots, cert)
			}
		}
	}

	if sc.GetCert() != "" {
		cert, err := tlsca.ParseCertificatePEM([]byte(sc.GetCert()))
		if err != nil {
			return nil, trace.Wrap(err, "failed to parse certificate defined in cert")
		}
		certStore.Roots = append(certStore.Roots, cert)
	}
	if len(certStore.Roots) == 0 {
		return nil, trace.BadParameter("no identity provider certificate provided, either set certificate as a parameter or via entity_descriptor")
	}

	signingKeyStore, err := utils.ParseSigningKeyStorePEM(sc.GetSigningKeyPair().PrivateKey, sc.GetSigningKeyPair().Cert)
	if err != nil {
		return nil, trace.Wrap(err, "failed to parse certificate defined in signing_key_pair")
	}

	// The encryption keystore here is defaulted to the value of the signing keystore
	// if no separate assertion decryption keys are provided. We do this here to initialize
	// the variable but if set to nil, gosaml2 will do this internally anyway.
	encryptionKeyStore := signingKeyStore
	encryptionKeyPair := sc.GetEncryptionKeyPair()
	if encryptionKeyPair != nil {
		encryptionKeyStore, err = utils.ParseSigningKeyStorePEM(encryptionKeyPair.PrivateKey, encryptionKeyPair.Cert)
		if err != nil {
			return nil, trace.Wrap(err, "failed to parse certificate defined in assertion_key_pair")
		}
	}

	sp := &saml2.SAMLServiceProvider{
		IdentityProviderSSOURL:         sc.GetSSO(),
		IdentityProviderIssuer:         sc.GetIssuer(),
		ServiceProviderIssuer:          sc.GetServiceProviderIssuer(),
		AssertionConsumerServiceURL:    sc.GetAssertionConsumerService(),
		SignAuthnRequests:              true,
		SignAuthnRequestsCanonicalizer: dsig.MakeC14N11Canonicalizer(),
		AudienceURI:                    sc.GetAudience(),
		IDPCertificateStore:            &certStore,
		SPKeyStore:                     encryptionKeyStore,
		SPSigningKeyStore:              signingKeyStore,
		Clock:                          dsig.NewFakeClock(clock),
		NameIdFormat:                   "urn:oasis:names:tc:SAML:1.1:nameid-format:unspecified",
	}

	// adfs specific settings
	if sc.GetProvider() == teleport.ADFS {
		log.WithFields(log.Fields{
			trace.Component: teleport.ComponentSAML,
		}).Debug("Setting ADFS values.")
		if sp.SignAuthnRequests {
			// adfs does not support C14N11, we have to use the C14N10 canonicalizer
			sp.SignAuthnRequestsCanonicalizer = dsig.MakeC14N10ExclusiveCanonicalizerWithPrefixList(dsig.DefaultPrefix)

			// at a minimum we require password protected transport
			sp.RequestedAuthnContext = &saml2.RequestedAuthnContext{
				Comparison: "minimum",
				Contexts:   []string{"urn:oasis:names:tc:SAML:2.0:ac:classes:PasswordProtectedTransport"},
			}
		}
	}

	return sp, nil
}

// SAMLConnectorV2SchemaTemplate is a template JSON Schema for SAMLConnector
const SAMLConnectorV2SchemaTemplate = `{
	"type": "object",
	"additionalProperties": false,
	"required": ["kind", "spec", "metadata", "version"],
	"properties": {
	  "kind": {"type": "string"},
	  "version": {"type": "string", "default": "v1"},
	  "metadata": %v,
	  "spec": %v
	}
  }`

// SAMLConnectorSpecV2Schema is a JSON Schema for SAML Connector
var SAMLConnectorSpecV2Schema = fmt.Sprintf(`{
	"type": "object",
	"additionalProperties": false,
	"required": ["acs"],
	"properties": {
	  "issuer": {"type": "string"},
	  "sso": {"type": "string"},
	  "cert": {"type": "string"},
	  "provider": {"type": "string"},
	  "display": {"type": "string"},
	  "acs": {"type": "string"},
	  "audience": {"type": "string"},
	  "service_provider_issuer": {"type": "string"},
	  "entity_descriptor": {"type": "string"},
	  "entity_descriptor_url": {"type": "string"},
	  "attributes_to_roles": {
		"type": "array",
		"items": %v
	  },
	  "signing_key_pair": %v,
	  "assertion_key_pair": %v
	}
  }`, AttributeMappingSchema, SigningKeyPairSchema, SigningKeyPairSchema)

// AttributeMappingSchema is JSON schema for claim mapping
var AttributeMappingSchema = `{
	"type": "object",
	"additionalProperties": false,
	"required": ["name", "value" ],
	"properties": {
	  "name": {"type": "string"},
	  "value": {"type": "string"},
	  "roles": {
		"type": "array",
		"items": {
		  "type": "string"
		}
	  }
	}
  }`

// SigningKeyPairSchema is the JSON schema for signing key pair.
var SigningKeyPairSchema = `{
	"type": "object",
	"additionalProperties": false,
	"properties": {
	  "private_key": {"type": "string"},
	  "cert": {"type": "string"}
	}
  }`

// GetSAMLConnectorSchema returns schema for SAMLConnector
func GetSAMLConnectorSchema() string {
	return fmt.Sprintf(SAMLConnectorV2SchemaTemplate, MetadataSchema, SAMLConnectorSpecV2Schema)
}

// UnmarshalSAMLConnector unmarshals the SAMLConnector resource from JSON.
func UnmarshalSAMLConnector(bytes []byte, opts ...MarshalOption) (SAMLConnector, error) {
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var h ResourceHeader
	err = utils.FastUnmarshal(bytes, &h)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	switch h.Version {
	case V2:
		var c SAMLConnectorV2
		if cfg.SkipValidation {
			if err := utils.FastUnmarshal(bytes, &c); err != nil {
				return nil, trace.BadParameter(err.Error())
			}
		} else {
			if err := utils.UnmarshalWithSchema(GetSAMLConnectorSchema(), &c, bytes); err != nil {
				return nil, trace.BadParameter(err.Error())
			}
		}

		if err := ValidateSAMLConnector(&c); err != nil {
			return nil, trace.Wrap(err)
		}

		if cfg.ID != 0 {
			c.SetResourceID(cfg.ID)
		}
		if !cfg.Expires.IsZero() {
			c.SetExpiry(cfg.Expires)
		}

		return &c, nil
	}

	return nil, trace.BadParameter("SAML connector resource version %v is not supported", h.Version)
}

// MarshalSAMLConnector marshals the SAMLConnector resource to JSON.
func MarshalSAMLConnector(samlConnector SAMLConnector, opts ...MarshalOption) ([]byte, error) {
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch samlConnector := samlConnector.(type) {
	case *SAMLConnectorV2:
		if version := samlConnector.GetVersion(); version != V2 {
			return nil, trace.BadParameter("mismatched SAML connector version %v and type %T", version, samlConnector)
		}
		if !cfg.PreserveResourceID {
			// avoid modifying the original object
			// to prevent unexpected data races
			copy := *samlConnector
			copy.SetResourceID(0)
			samlConnector = &copy
		}
		return utils.FastMarshal(samlConnector)
	default:
		return nil, trace.BadParameter("unrecognized SAML connector version %T", samlConnector)
	}
}
