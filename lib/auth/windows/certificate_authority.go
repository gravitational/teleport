// Copyright 2022 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package windows

import (
	"bytes"
	"context"
	"encoding/pem"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
)

// NewCertificateStoreClient returns a new structure for modifying windows certificates in a windows CA
func NewCertificateStoreClient(cfg CertificateStoreConfig) *CertificateStoreClient {
	return &CertificateStoreClient{
		cfg: cfg,
	}
}

// CertificateStoreClient implements access to a Windows Certificate Authority
type CertificateStoreClient struct {
	cfg CertificateStoreConfig
}

// CertificateStoreConfig is a config structure for a Windows Certificate Authority
type CertificateStoreConfig struct {
	// AccessPoint is the Auth API client (with caching).
	AccessPoint auth.WindowsDesktopAccessPoint
	// LDAPConfig is the ldap configuration
	LDAPConfig
	// Log is the logging sink for the service
	Log logrus.FieldLogger
	// ClusterName is the name of this cluster
	ClusterName string
	// LC is the LDAPClient
	LC *LDAPClient
}

// Update publishes the certificate to the current cluster's certificate authority
func (c *CertificateStoreClient) Update(ctx context.Context) error {
	// Publish the CA cert for current cluster CA. For trusted clusters, their
	// respective windows_desktop_services will publish their CAs so we don't
	// have to do it here.
	//
	// TODO(zmb3): support multiple CA certs per cluster (such as with HSMs).
	caType := types.UserCA
	ca, err := c.cfg.AccessPoint.GetCertAuthority(ctx, types.CertAuthID{
		Type:       caType,
		DomainName: c.cfg.ClusterName,
	}, false)
	if err != nil {
		return trace.Wrap(err, "fetching Teleport CA")
	}

	keypairs := ca.GetTrustedTLSKeyPairs()
	c.cfg.Log.Debugf("Teleport CA has %d trusted keypairs", len(keypairs))

	// LDAP stores certs and CRLs in binary DER format, so remove the outer PEM
	// wrapper.
	caPEM := keypairs[0].Cert
	caBlock, _ := pem.Decode(caPEM)
	if caBlock == nil {
		return trace.BadParameter("failed to decode CA PEM block")
	}
	caDER := caBlock.Bytes

	crlDER, err := c.cfg.AccessPoint.GenerateCertAuthorityCRL(ctx, types.UserCA)
	if err != nil {
		return trace.Wrap(err, "generating CRL")
	}

	// To make the CA trusted, we need 3 things:
	// 1. put the CA cert into the Trusted Certification Authorities in the
	//    Group Policy (done manually for now, see public docs)
	// 2. put the CA cert into NTAuth store in LDAP
	// 3. put the CRL of the CA into a dedicated LDAP entry
	//
	// Below we do #2 and #3.
	if err := c.updateCAInNTAuthStore(ctx, caDER); err != nil {
		return trace.Wrap(err, "updating NTAuth store over LDAP")
	}
	if err := c.updateCRL(ctx, crlDER, caType); err != nil {
		return trace.Wrap(err, "updating CRL over LDAP")
	}
	return nil
}

// updateCAInNTAuthStore records the Teleport user CA in the Windows store which records
// CAs that are eligible to issue smart card login certificates and perform client
// private key archival.
//
// This function is equivalent to running:
//
//	certutil –dspublish –f <PathToCertFile.cer> NTAuthCA
//
// You can confirm the cert is present by running:
//
//	certutil -viewstore "ldap:///CN=NTAuthCertificates,CN=Public Key Services,CN=Services,CN=Configuration,DC=example,DC=com>?caCertificate"
//
// Once the CA is published to LDAP, it should eventually sync and be present in the
// machine's enterprise NTAuth store. You can check that with:
//
//	certutil -viewstore -enterprise NTAuth
//
// You can expedite the synchronization by running:
//
//	certutil -pulse
func (c *CertificateStoreClient) updateCAInNTAuthStore(ctx context.Context, caDER []byte) error {
	// Check if our CA is already in the store. The LDAP entry for NTAuth store
	// is constant and it should always exist.
	ntAuthDN := "CN=NTAuthCertificates,CN=Public Key Services,CN=Services,CN=Configuration," + c.cfg.LDAPConfig.DomainDN()
	entries, err := c.cfg.LC.Read(ntAuthDN, "certificationAuthority", []string{"cACertificate"})
	if err != nil {
		return trace.Wrap(err, "fetching existing CAs")
	}
	if len(entries) != 1 {
		return trace.BadParameter("expected exactly 1 NTAuthCertificates CA store at %q, but found %d", ntAuthDN, len(entries))
	}
	// TODO(zmb3): during CA rotation, find the old CA in NTAuthStore and remove it.
	// Right now we just append the active CA and let the old ones hang around.
	existingCAs := entries[0].GetRawAttributeValues("cACertificate")
	for _, existingCADER := range existingCAs {
		// CA already present.
		if bytes.Equal(existingCADER, caDER) {
			c.cfg.Log.Info("Teleport CA already present in NTAuthStore in LDAP")
			return nil
		}
	}

	c.cfg.Log.Debugf("None of the %d existing NTAuthCertificates matched Teleport's", len(existingCAs))

	// CA is not in the store, append it.
	updatedCAs := make([]string, 0, len(existingCAs)+1)
	for _, existingCADER := range existingCAs {
		updatedCAs = append(updatedCAs, string(existingCADER))
	}
	updatedCAs = append(updatedCAs, string(caDER))

	if err := c.cfg.LC.Update(ntAuthDN, map[string][]string{
		"cACertificate": updatedCAs,
	}); err != nil {
		return trace.Wrap(err, "updating CA entry")
	}
	c.cfg.Log.Info("Added Teleport CA to NTAuthStore via LDAP")
	return nil
}

func (c *CertificateStoreClient) updateCRL(ctx context.Context, crlDER []byte, caType types.CertAuthType) error {
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
	containerDN := crlContainerDN(c.cfg.LDAPConfig, caType)
	crlDN := crlDN(c.cfg.ClusterName, c.cfg.LDAPConfig, caType)

	// Create the parent container.
	if err := c.cfg.LC.CreateContainer(containerDN); err != nil {
		return trace.Wrap(err, "creating CRL container")
	}

	// Create the CRL object itself.
	if err := c.cfg.LC.Create(
		crlDN,
		"cRLDistributionPoint",
		map[string][]string{"certificateRevocationList": {string(crlDER)}},
	); err != nil {
		if !trace.IsAlreadyExists(err) {
			return trace.Wrap(err)
		}
		// CRL already exists, update it.
		if err := c.cfg.LC.Update(
			crlDN,
			map[string][]string{"certificateRevocationList": {string(crlDER)}},
		); err != nil {
			return trace.Wrap(err)
		}
		c.cfg.Log.Info("Updated CRL for Windows logins via LDAP")
	} else {
		c.cfg.Log.Info("Added CRL for Windows logins via LDAP")
	}
	return nil
}
