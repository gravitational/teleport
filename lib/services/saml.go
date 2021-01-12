package services

import (
	"crypto/x509"
	"encoding/base64"
	"encoding/xml"
	"strings"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	saml2 "github.com/russellhaering/gosaml2"
	"github.com/russellhaering/gosaml2/types"
	dsig "github.com/russellhaering/goxmldsig"
)

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
		err := xml.Unmarshal([]byte(sc.GetEntityDescriptor()), metadata)
		if err != nil {
			return nil, trace.Wrap(err, "failed to parse entity_descriptor")
		}

		for _, kd := range metadata.IDPSSODescriptor.KeyDescriptors {
			for _, samlCert := range kd.KeyInfo.X509Data.X509Certificates {
				certData, err := base64.StdEncoding.DecodeString(strings.TrimSpace(samlCert.Data))
				if err != nil {
					return nil, trace.Wrap(err)
				}
				cert, err := x509.ParseCertificate(certData)
				if err != nil {
					return nil, trace.Wrap(err, "failed to parse certificate in metadata")
				}
				certStore.Roots = append(certStore.Roots, cert)
			}
		}
	}

	if sc.GetCert() != "" {
		cert, err := tlsca.ParseCertificatePEM([]byte(sc.GetCert()))
		if err != nil {
			return nil, trace.Wrap(err)
		}
		certStore.Roots = append(certStore.Roots, cert)
	}

	if len(certStore.Roots) == 0 {
		return nil, trace.BadParameter("no identity provider certificate provided, either set certificate as a parameter or via entity_descriptor")
	}

	keyStore, err := utils.ParseSigningKeyStorePEM(sc.GetSigningKeyPair().PrivateKey, sc.GetSigningKeyPair().Cert)
	if err != nil {
		return nil, trace.Wrap(err)
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
		SPKeyStore:                     keyStore,
		Clock:                          dsig.NewFakeClock(clock),
		NameIdFormat:                   "urn:oasis:names:tc:SAML:1.1:nameid-format:unspecified",
	}

	// adfs specific settings
	if sc.GetAudience() == teleport.ADFS {
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
