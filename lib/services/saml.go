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
	"context"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/xml"
	"net/http"
	"strings"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	saml2 "github.com/russellhaering/gosaml2"
	samltypes "github.com/russellhaering/gosaml2/types"
	dsig "github.com/russellhaering/goxmldsig"
	log "github.com/sirupsen/logrus"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

// ValidateSAMLConnector validates the SAMLConnector and sets default values.
// If a remote to fetch roles is specified, roles will be validated to exist.
func ValidateSAMLConnector(sc types.SAMLConnector, rg RoleGetter) error {
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
		metadata := &samltypes.EntityDescriptor{}
		if err := xml.Unmarshal([]byte(sc.GetEntityDescriptor()), metadata); err != nil {
			return trace.Wrap(err, "failed to parse entity_descriptor")
		}

		sc.SetIssuer(metadata.EntityID)
		if metadata.IDPSSODescriptor != nil && len(metadata.IDPSSODescriptor.SingleSignOnServices) > 0 {
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
		sc.SetSigningKeyPair(&types.AsymmetricKeyPair{
			PrivateKey: string(keyPEM),
			Cert:       string(certPEM),
		})
	}

	if len(sc.GetAttributesToRoles()) == 0 {
		return trace.BadParameter("attributes_to_roles is empty, authorization with connector would never assign any roles")
	}

	if rg != nil {
		for _, mapping := range sc.GetAttributesToRoles() {
			for _, role := range mapping.Roles {
				if utils.ContainsExpansion(role) {
					// Role is a template so we cannot check for existence of that literal name.
					continue
				}
				_, err := rg.GetRole(context.Background(), role)
				switch {
				case trace.IsNotFound(err):
					return trace.BadParameter("role %q specified in attributes_to_roles not found", role)
				case err != nil:
					return trace.Wrap(err)
				}
			}
		}
	}

	log.Debugf("[SAML] SSO: %v", sc.GetSSO())
	log.Debugf("[SAML] Issuer: %v", sc.GetIssuer())
	log.Debugf("[SAML] ACS: %v", sc.GetAssertionConsumerService())

	return nil
}

// GetAttributeNames returns a list of claim names from the claim values
func GetAttributeNames(attributes map[string]samltypes.Attribute) []string {
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

// CheckSAMLEntityDescriptor checks if the entity descriptor XML is valid and has at least one valid certificate.
func CheckSAMLEntityDescriptor(entityDescriptor string) ([]*x509.Certificate, error) {
	if entityDescriptor == "" {
		return nil, nil
	}

	metadata := &samltypes.EntityDescriptor{}
	if err := xml.Unmarshal([]byte(entityDescriptor), metadata); err != nil {
		return nil, trace.Wrap(err, "failed to parse entity_descriptor")
	}

	var roots []*x509.Certificate

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
			roots = append(roots, cert)
		}
	}

	return roots, nil
}

// GetSAMLServiceProvider gets the SAMLConnector's service provider
func GetSAMLServiceProvider(sc types.SAMLConnector, clock clockwork.Clock) (*saml2.SAMLServiceProvider, error) {
	roots, errEd := CheckSAMLEntityDescriptor(sc.GetEntityDescriptor())
	if errEd != nil {
		return nil, trace.Wrap(errEd)
	}

	certStore := dsig.MemoryX509CertificateStore{Roots: roots}

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

	signingKeyPair := sc.GetSigningKeyPair()
	encryptionKeyPair := sc.GetEncryptionKeyPair()
	var keyStore *utils.KeyStore
	var signingKeyStore *utils.KeyStore
	var err error

	// Due to some weird design choices with how gosaml2 keys are configured we have to do some trickery
	// in order to default properly when SAML assertion encryption is turned off.
	// Below are the different possible cases.
	if encryptionKeyPair == nil {
		// Case 1: Only the signing key pair is set. This means that SAML encryption is not expected
		// and we therefore configure the main key that gets used for all operations as the signing key.
		// This is done because gosaml2 mandates an encryption key even if not used.
		log.Info("No assertion_key_pair was detected. Falling back to signing key for all SAML operations.")
		keyStore, err = utils.ParseKeyStorePEM(signingKeyPair.PrivateKey, signingKeyPair.Cert)
		signingKeyStore = keyStore
		if err != nil {
			return nil, trace.Wrap(err, "failed to parse certificate or private key defined in signing_key_pair")
		}
	} else {
		// Case 2: An encryption keypair is configured. This means that encrypted SAML responses are expected.
		// Since gosaml2 always uses the main key for encryption, we set it to assertion_key_pair.
		// To handle signing correctly, we now instead set the optional signing key in gosaml2 to signing_key_pair.
		log.Info("Detected assertion_key_pair and configured it to decrypt SAML responses.")
		keyStore, err = utils.ParseKeyStorePEM(encryptionKeyPair.PrivateKey, encryptionKeyPair.Cert)
		if err != nil {
			return nil, trace.Wrap(err, "failed to parse certificate or private key defined in assertion_key_pair")
		}
		signingKeyStore, err = utils.ParseKeyStorePEM(signingKeyPair.PrivateKey, signingKeyPair.Cert)
		if err != nil {
			return nil, trace.Wrap(err, "failed to parse certificate or private key defined in signing_key_pair")
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
		SPSigningKeyStore:              signingKeyStore,
		SPKeyStore:                     keyStore,
		Clock:                          dsig.NewFakeClock(clock),
		NameIdFormat:                   "urn:oasis:names:tc:SAML:1.1:nameid-format:unspecified",
	}

	// Provider specific settings for ADFS and JumpCloud. Specifically these
	// providers do not support C14N11, which means a C14N10 canonicalizer has to
	// be used.
	switch sc.GetProvider() {
	case teleport.ADFS, teleport.JumpCloud:
		log.WithFields(log.Fields{
			trace.Component: teleport.ComponentSAML,
		}).Debug("Setting ADFS/JumpCloud values.")
		if sp.SignAuthnRequests {
			sp.SignAuthnRequestsCanonicalizer = dsig.MakeC14N10ExclusiveCanonicalizerWithPrefixList(dsig.DefaultPrefix)

			// At a minimum we require password protected transport.
			sp.RequestedAuthnContext = &saml2.RequestedAuthnContext{
				Comparison: "minimum",
				Contexts:   []string{"urn:oasis:names:tc:SAML:2.0:ac:classes:PasswordProtectedTransport"},
			}
		}
	}

	return sp, nil
}

// UnmarshalSAMLConnector unmarshals the SAMLConnector resource from JSON.
func UnmarshalSAMLConnector(bytes []byte, opts ...MarshalOption) (types.SAMLConnector, error) {
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var h types.ResourceHeader
	err = utils.FastUnmarshal(bytes, &h)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	switch h.Version {
	case types.V2:
		var c types.SAMLConnectorV2
		if err := utils.FastUnmarshal(bytes, &c); err != nil {
			return nil, trace.BadParameter(err.Error())
		}

		if err := ValidateSAMLConnector(&c, nil); err != nil {
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
func MarshalSAMLConnector(samlConnector types.SAMLConnector, opts ...MarshalOption) ([]byte, error) {
	if err := ValidateSAMLConnector(samlConnector, nil); err != nil {
		return nil, trace.Wrap(err)
	}

	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch samlConnector := samlConnector.(type) {
	case *types.SAMLConnectorV2:
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
