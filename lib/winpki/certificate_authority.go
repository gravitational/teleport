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

package winpki

import (
	"context"
	"crypto/tls"
	"encoding/pem"
	"log/slog"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/tlsutils"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/subca"
	"github.com/gravitational/teleport/lib/tlsca"
)

// NewCertificateStoreClient returns a new structure for modifying windows certificates in a Windows CA.
func NewCertificateStoreClient(cfg CertificateStoreConfig) *CertificateStoreClient {
	return &CertificateStoreClient{cfg: cfg}
}

// CertificateStoreClient implements access to a Windows Certificate Authority
type CertificateStoreClient struct {
	cfg CertificateStoreConfig
}

// CRLGenerator generates CRLs, which are required for certificate-based authentication on Windows.
// Teleport has its own locking concept that is used for revocation, so the CRLS generated here
// are always empty and exist only to satisfy the Windows requirements for CRL checking.
type CRLGenerator interface {
	// SubCAServiceGetter reads CA override resources.
	services.SubCAServiceGetter
	// GenerateCertAuthorityCRL returns an empty CRL for a CA.
	GenerateCertAuthorityCRL(ctx context.Context, caType types.CertAuthType) ([]byte, error)
	// GetCertAuthorities returns a list of cert authorities
	GetCertAuthorities(ctx context.Context, caType types.CertAuthType, loadKeys bool) ([]types.CertAuthority, error)
}

// CertificateStoreConfig is a config structure for a Windows Certificate Authority
type CertificateStoreConfig struct {
	// AccessPoint is the Auth API client (with caching).
	AccessPoint CRLGenerator
	// Domain is the Active Directory domain where Teleport publishes its
	// Certificate Revocation List (CRL).
	Domain string
	// Logger is the logging sink for the service
	Logger *slog.Logger
	// ClusterName is the name of this Teleport cluster
	ClusterName string
	// LC is the LDAPConfig
	LC *LDAPConfig
	// DialLDAPForTesting allows callers to provide an alternative implementation
	// of the DialLDAP function.
	// Used for testing.
	DialLDAPForTesting func(ctx context.Context, cfg *LDAPConfig, credentials *tls.Config) (LDAPClientForCRLUpdate, error)
}

// LDAPClientForCRLUpdate defines the subset of [LDAPClient] necessary for CRL
// updates.
//
// See [LDAPClient].
type LDAPClientForCRLUpdate interface {
	Close() error
	CreateContainer(ctx context.Context, dn string) error
	Create(dn string, class string, attrs map[string][]string) error
	Update(ctx context.Context, dn string, replaceAttrs map[string][]string) error
}

// Update publishes an empty CRLs (Certificate Revocation Lists) to LDAP.
// Both CA and CA override resources are queried for CRLs to publish.
func (c *CertificateStoreClient) Update(ctx context.Context, tc *tls.Config) error {
	caType := types.WindowsCA

	// TODO(zmb3): check for the presence of Teleport's CA in the NTAuth store

	// To make the CA trusted, we need 3 things:
	// 1. put the CA cert into the Trusted Certification Authorities in Group Policy
	// 2. put the CA cert into NTAuth store in LDAP
	// 3. put the CRL of the CA into a dedicated LDAP entry
	//
	// #1 and #2 are done manually as part of the set-up process (see public docs).
	// Below we do #3.

	certAuthorities, err := c.cfg.AccessPoint.GetCertAuthorities(ctx, caType, false)
	if err != nil {
		return trace.Wrap(err)
	}
	// The cache doesn't error on an unknown CA, it returns empty instead.
	// TODO(codingllama): DELETE IN 20. WindowsCA is guaranteed to exist by then.
	if len(certAuthorities) == 0 {
		c.cfg.Logger.WarnContext(ctx, "Found no CAs with type WindowsCA. Falling back to UserCA.")
		caType = types.UserCA
		certAuthorities, err = c.cfg.AccessPoint.GetCertAuthorities(ctx, caType, false)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	for _, ca := range certAuthorities {
		for _, keySet := range [][]*types.TLSKeyPair{
			ca.GetActiveKeys().TLS,
			ca.GetAdditionalTrustedKeys().TLS,
		} {
			for _, keyPair := range keySet {
				if len(keyPair.CRL) == 0 {
					continue
				}

				cert, err := tlsca.ParseCertificatePEM(keyPair.Cert)
				if err != nil {
					return trace.Wrap(err)
				}
				c.cfg.Logger.DebugContext(ctx, "Processing CA key pair",
					"issuer", cert.Issuer,
					"subject", cert.Subject,
				)

				if err := c.updateCRL(ctx, cert.Subject.CommonName, cert.SubjectKeyId, keyPair.CRL, ca.GetType(), tc); err != nil {
					return trace.Wrap(err)
				}
			}
		}
	}

	if err := c.updateCAOverrideCRLs(ctx, tc); err != nil {
		return trace.Wrap(err, "update CA override CRLs")
	}

	return nil
}

func (c *CertificateStoreClient) updateCAOverrideCRLs(ctx context.Context, tc *tls.Config) error {
	const caType = types.WindowsCA
	clusterName := c.cfg.ClusterName

	caOverride, err := c.cfg.AccessPoint.GetCertAuthorityOverride(ctx, types.CertAuthorityOverrideID{
		ClusterName: clusterName,
		CAType:      string(caType),
	})
	if trace.IsNotFound(err) {
		return nil // OK, overrides are optional.
	}
	if err != nil {
		return trace.Wrap(err, "read CA overrides")
	}

	crlMap := caOverride.GetStatus().GetPublicKeyHashToCrl()

	var errs []error
	for _, co := range caOverride.GetSpec().GetCertificateOverrides() {
		// Find a candidate override.
		if co.GetCertificate() == "" {
			continue
		}

		// Parse certificate.
		cert, err := tlsutils.ParseCertificatePEM([]byte(co.GetCertificate()))
		if err != nil {
			c.cfg.Logger.WarnContext(ctx, "Failed to parse CA override certificate, skipping",
				"ca_type", caType,
				"cluster_name", clusterName,
				"public_key_hash", co.GetPublicKey(), // Note, may be empty.
			)
			continue
		}

		// Produce PKH.
		pkh := subca.HashCertificatePublicKey(cert)

		logger := c.cfg.Logger.With(
			"ca_type", caType,
			"cluster_name", clusterName,
			"public_key_hash", pkh,
		)

		// Find and parse CRL.
		crlPB, ok := crlMap[pkh]
		if !ok || crlPB == nil {
			logger.WarnContext(ctx, "CA override lacks CRL for certificate, skipping",
				"ca_type", caType,
				"cluster_name", clusterName,
				"public_key_hash", pkh,
			)
			continue
		}
		block, _ := pem.Decode([]byte(crlPB.Pem))
		if block == nil {
			logger.WarnContext(ctx, "Failed to decode CA override CRL PEM")
			continue
		}
		crlDER := block.Bytes
		if len(crlDER) == 0 {
			logger.WarnContext(ctx, "CA override has empty CRL DER")
			continue
		}
		logger.DebugContext(ctx, "Updating CA override CRL")

		// Update CRL. Record errors from this step.
		if err := c.updateCRL(ctx, cert.Subject.CommonName, cert.SubjectKeyId, crlDER, caType, tc); err != nil {
			errs = append(errs, err)
		}
	}

	return trace.NewAggregate(errs...)
}

func (c *CertificateStoreClient) updateCRL(ctx context.Context, issuerCN string, issuerSKID []byte, crlDER []byte, caType types.CertAuthType, tc *tls.Config) error {
	// Publish the CRL for current cluster CA. For trusted clusters, their
	// respective windows_desktop_services will publish CRLs of their CAs so we
	// don't have to do it here.
	//
	// CRLs live under the CDP (CRL Distribution Point) LDAP container. There's
	// another nested container with the CA name, I think, and then multiple
	// separate CRL objects in that container.
	//
	// We name our parent container based on the CA type (for example, for User
	// CA, it is called "Teleport"), and the CRL object is named after the
	// Teleport cluster name. So, for instance, CRL for cluster "prod" and User
	// CA will be placed at:
	// ... > CDP > Teleport > prod
	containerDN, err := crlContainerDN(c.cfg.Domain, caType)
	if err != nil {
		return trace.Wrap(err)
	}
	crlDN, err := CRLDN(issuerCN, issuerSKID, c.cfg.Domain, caType)
	if err != nil {
		return trace.Wrap(err)
	}

	var ldapClient LDAPClientForCRLUpdate
	if c.cfg.DialLDAPForTesting != nil {
		ldapClient, err = c.cfg.DialLDAPForTesting(ctx, c.cfg.LC, tc)
	} else {
		ldapClient, err = DialLDAP(ctx, c.cfg.LC, tc)
	}
	if err != nil {
		return trace.Wrap(err, "dialing LDAP server")
	}
	defer ldapClient.Close()

	// Create the parent container.
	if err := ldapClient.CreateContainer(ctx, containerDN); err != nil {
		return trace.Wrap(err, "creating CRL container")
	}

	logger := c.cfg.Logger.With(
		"ca_type", caType,
		"dn", crlDN,
	)

	// Create the CRL object itself.
	if err := ldapClient.Create(
		crlDN,
		"cRLDistributionPoint",
		map[string][]string{"certificateRevocationList": {string(crlDER)}},
	); err != nil {
		if !trace.IsAlreadyExists(err) {
			return trace.Wrap(err)
		}
		// CRL already exists, update it.
		if err := ldapClient.Update(
			ctx,
			crlDN,
			map[string][]string{"certificateRevocationList": {string(crlDER)}},
		); err != nil {
			return trace.Wrap(err)
		}
		logger.InfoContext(ctx, "Updated CRL for Windows logins via LDAP")
	} else {
		logger.InfoContext(ctx, "Added CRL for Windows logins via LDAP")
	}
	return nil
}
