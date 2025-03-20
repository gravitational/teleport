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

package windows

import (
	"context"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authclient"
)

// NewCertificateStoreClient returns a new structure for modifying windows certificates in a Windows CA.
func NewCertificateStoreClient(cfg CertificateStoreConfig) *CertificateStoreClient {
	return &CertificateStoreClient{cfg: cfg}
}

// CertificateStoreClient implements access to a Windows Certificate Authority
type CertificateStoreClient struct {
	cfg CertificateStoreConfig
}

// CertificateStoreConfig is a config structure for a Windows Certificate Authority
type CertificateStoreConfig struct {
	// AccessPoint is the Auth API client (with caching).
	AccessPoint authclient.WindowsDesktopAccessPoint
	// LDAPConfig is the ldap configuration
	LDAPConfig
	// Log is the logging sink for the service
	Log logrus.FieldLogger
	// ClusterName is the name of this cluster
	ClusterName string
	// LC is the LDAPClient
	LC *LDAPClient
}

// Update publishes an empty certificate revocation list to LDAP.
func (c *CertificateStoreClient) Update(ctx context.Context) error {
	caType := types.UserCA

	crlDER, err := c.cfg.AccessPoint.GenerateCertAuthorityCRL(ctx, caType)
	if err != nil {
		return trace.Wrap(err, "generating CRL")
	}

	// TODO(zmb3): check for the presence of Teleport's CA in the NTAuth store

	// To make the CA trusted, we need 3 things:
	// 1. put the CA cert into the Trusted Certification Authorities in Group Policy
	// 2. put the CA cert into NTAuth store in LDAP
	// 3. put the CRL of the CA into a dedicated LDAP entry
	//
	// #1 and #2 are done manually as part of the set up process (see public docs).
	// Below we do #3.
	if err := c.updateCRL(ctx, crlDER, caType); err != nil {
		return trace.Wrap(err, "updating CRL over LDAP")
	}
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
