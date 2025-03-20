/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
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

type SAMLConnectorGetter interface {
	GetSAMLConnector(ctx context.Context, id string, withSecrets bool) (types.SAMLConnector, error)
}

const ErrMsgHowToFixMissingPrivateKey = "You must either specify the signing key pair (obtain the existing one with `tctl get saml --with-secrets`) or let Teleport generate a new one (remove signing_key_pair in the resource you're trying to create)."

// ValidateSAMLConnector validates the SAMLConnector and sets default values.
// If a remote to fetch roles is specified, roles will be validated to exist.
func ValidateSAMLConnector(sc types.SAMLConnector, rg RoleGetter) error {
	if err := CheckAndSetDefaults(sc); err != nil {
		return trace.Wrap(err)
	}

	getEntityDescriptorFromURL := func(url string) (string, error) {
		resp, err := http.Get(url)
		if err != nil {
			return "", trace.WrapWithMessage(err, "unable to fetch entity descriptor from %v for SAML connector %v", url, sc.GetName())
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return "", trace.BadParameter("status code %v when fetching from %v for SAML connector %v", resp.StatusCode, url, sc.GetName())
		}
		body, err := utils.ReadAtMost(resp.Body, teleport.MaxHTTPResponseSize)
		if err != nil {
			return "", trace.Wrap(err)
		}
		return string(body), nil
	}

	getEntityDescriptorMetadata := func(ed string) (*samltypes.EntityDescriptor, error) {
		metadata := &samltypes.EntityDescriptor{}
		if err := xml.Unmarshal([]byte(ed), metadata); err != nil {
			return nil, trace.Wrap(err, "failed to parse entity_descriptor")
		}
		return metadata, nil
	}

	// Validate standard settings.
	if url := sc.GetEntityDescriptorURL(); url != "" {
		entityDescriptor, err := getEntityDescriptorFromURL(url)
		if err != nil {
			return trace.Wrap(err)
		}

		sc.SetEntityDescriptor(entityDescriptor)
		log.Debugf("[SAML] Successfully fetched entity descriptor from %v for connector %v", url, sc.GetName())
	}

	if ed := sc.GetEntityDescriptor(); ed != "" {
		md, err := getEntityDescriptorMetadata(ed)
		if err != nil {
			return trace.Wrap(err)
		}

		sc.SetIssuer(md.EntityID)
		if md.IDPSSODescriptor != nil && len(md.IDPSSODescriptor.SingleSignOnServices) > 0 {
			metadataSsoUrl := md.IDPSSODescriptor.SingleSignOnServices[0].Location
			if sc.GetSSO() != "" && sc.GetSSO() != metadataSsoUrl {
				log.WithFields(log.Fields{
					"connector_name":       sc.GetName(),
					"connector_sso_url":    sc.GetSSO(),
					"idp_metadata_sso_url": metadataSsoUrl,
				}).Warn(
					"Connector has set SSO URL, but it does not match the one found in IDP metadata. Overwriting with the IDP metadata SSO URL.",
				)
			}
			sc.SetSSO(metadataSsoUrl)
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

	// Validate MFA settings.
	if mfa := sc.GetMFASettings(); mfa != nil {
		if mfa.EntityDescriptorUrl != "" {
			if mfa.EntityDescriptorUrl == sc.GetEntityDescriptorURL() {
				// we got the entity descriptor above, skip the redundant round trip.
				mfa.EntityDescriptor = sc.GetEntityDescriptor()
			} else {
				entityDescriptor, err := getEntityDescriptorFromURL(mfa.EntityDescriptorUrl)
				if err != nil {
					return trace.Wrap(err)
				}
				mfa.EntityDescriptor = entityDescriptor
			}
		}
		if mfa.EntityDescriptor != "" {
			md, err := getEntityDescriptorMetadata(mfa.EntityDescriptor)
			if err != nil {
				return trace.Wrap(err)
			}

			mfa.Issuer = md.EntityID
			if md.IDPSSODescriptor != nil && len(md.IDPSSODescriptor.SingleSignOnServices) > 0 {
				mfa.Sso = md.IDPSSODescriptor.SingleSignOnServices[0].Location
			}
		}
		sc.SetMFASettings(mfa)
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

	if metadata.IDPSSODescriptor == nil {
		return nil, nil
	}

	var roots []*x509.Certificate

	for _, kd := range metadata.IDPSSODescriptor.KeyDescriptors {
		for _, samlCert := range kd.KeyInfo.X509Data.X509Certificates {
			// The certificate is base64 encoded and can be split into multiple lines.
			// Each line can be padded with spaces/tabs, so we need to remove them first
			// before decoding otherwise we'll get an error.
			// We need to run this through strings.Fields to remove spaces/tabs
			// from each line and then join them back with newlines.
			// The last step isn't strictly necessary, but it makes payload more readable.
			certData, err := base64.StdEncoding.DecodeString(strings.Join(strings.Fields(samlCert.Data), "\n"))
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
		ForceAuthn:                     sc.GetForceAuthn(),
	}

	// Provider specific settings for ADFS and JumpCloud. Specifically these
	// providers do not support C14N11, which means a C14N10 canonicalizer has to
	// be used.
	switch sc.GetProvider() {
	case teleport.ADFS, teleport.JumpCloud:
		log.WithFields(log.Fields{
			teleport.ComponentKey: teleport.ComponentSAML,
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

		if cfg.Revision != "" {
			c.SetRevision(cfg.Revision)
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
		return utils.FastMarshal(maybeResetProtoRevision(cfg.PreserveRevision, samlConnector))
	default:
		return nil, trace.BadParameter("unrecognized SAML connector version %T", samlConnector)
	}
}

// FillSAMLSigningKeyFromExisting looks up the existing SAML connector and populates the signing key if it's missing.
// This must be called only if the SAML Connector signing key pair has been initialized (ValidateSAMLConnector) and
// the private key is still empty.
func FillSAMLSigningKeyFromExisting(ctx context.Context, connector types.SAMLConnector, sg SAMLConnectorGetter) error {
	existing, err := sg.GetSAMLConnector(ctx, connector.GetName(), true /* with secrets */)
	switch {
	case trace.IsNotFound(err):
		return trace.BadParameter("failed to create SAML connector, the SAML connector has no signing key set. " + ErrMsgHowToFixMissingPrivateKey)
	case err != nil:
		return trace.BadParameter("failed to update SAML connector, the SAML connector has no signing key set and looking up the existing connector failed with the error: %s. %s", err.Error(), ErrMsgHowToFixMissingPrivateKey)
	}

	existingSkp := existing.GetSigningKeyPair()
	if existingSkp == nil || existingSkp.Cert != connector.GetSigningKeyPair().Cert {
		return trace.BadParameter("failed to update the SAML connector, the SAML connector has no signing key and its signing certificate does not match the existing one. " + ErrMsgHowToFixMissingPrivateKey)
	}
	connector.SetSigningKeyPair(existingSkp)
	return nil
}
