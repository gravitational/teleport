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
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/go-ldap/ldap/v3"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/modules"
)

const (
	// ldapDialTimeout is the timeout for dialing the LDAP server
	// when making an initial connection
	ldapDialTimeout = 5 * time.Second

	// ldapRequestTimeout is the timeout for making LDAP requests.
	// It is larger than the dial timeout because LDAP queries in large
	// Active Directory environments may take longer to complete.
	ldapRequestTimeout = 20 * time.Second
)

// NewCertificateStoreClient returns a new structure for modifying windows certificates in a windows CA
func NewCertificateStoreClient(cfg CertificateStoreConfig) *CertificateStoreClient {
	return &CertificateStoreClient{
		Cfg: cfg,
	}
}

// CertificateStoreClient implements access to a Windows Certificate Authority
type CertificateStoreClient struct {
	Cfg CertificateStoreConfig
	mu  sync.Mutex

	ldapInitialized bool
	ldapCertRenew   *time.Timer
}

// CertificateStoreConfig is a config structure for a Windows Certificate Authority
type CertificateStoreConfig struct {
	// AuthClient is the Auth API client (without caching).
	AuthClient AuthInterface
	// LDAPConfig is the ldap configuration
	LDAPConfig
	// Log is the logging sink for the service
	Log logrus.FieldLogger
	// ClusterName is the name of this cluster
	ClusterName string
	// LC is the LDAPClient
	LC *LDAPClient
	// LDAPCertTTL is the time to live for a teleport generated certificate used with LDAP
	LDAPCertTTL time.Duration
	// RetryInterval is the retry interval for connecting with LDAP
	RetryInterval time.Duration
	// CAType is the teleport CA to use
	CAType types.CertAuthType
}

// Update publishes the certificate to the current cluster's certificate authority
func (c *CertificateStoreClient) Update(ctx context.Context) error {
	// Publish the CA cert for current cluster CA. For trusted clusters, their
	// respective windows_desktop_services will publish their CAs so we don't
	// have to do it here.
	//
	// TODO(zmb3): support multiple CA certs per cluster (such as with HSMs).
	ca, err := c.Cfg.AuthClient.GetCertAuthority(ctx, types.CertAuthID{
		Type:       c.Cfg.CAType,
		DomainName: c.Cfg.ClusterName,
	}, false)
	if err != nil {
		return trace.Wrap(err, "fetching Teleport CA: %v", err)
	}
	// LDAP stores certs and CRLs in binary DER format, so remove the outer PEM
	// wrapper.
	caPEM := ca.GetTrustedTLSKeyPairs()[0].Cert
	caBlock, _ := pem.Decode(caPEM)
	if caBlock == nil {
		return trace.BadParameter("failed to decode CA PEM block")
	}
	caDER := caBlock.Bytes

	crlDER, err := c.Cfg.AuthClient.GenerateCertAuthorityCRL(ctx, c.Cfg.CAType)
	if err != nil {
		return trace.Wrap(err, "generating CRL: %v", err)
	}

	// To make the CA trusted, we need 3 things:
	// 1. put the CA cert into the Trusted Certification Authorities in the
	//    Group Policy (done manually for now, see public docs)
	// 2. put the CA cert into NTAuth store in LDAP
	// 3. put the CRL of the CA into a dedicated LDAP entry
	//
	// Below we do #2 and #3.
	if err := c.updateCAInNTAuthStore(ctx, caDER); err != nil {
		return trace.Wrap(err, "updating NTAuth store over LDAP: %v", err)
	}
	if err := c.updateCRL(ctx, crlDER); err != nil {
		return trace.Wrap(err, "updating CRL over LDAP: %v", err)
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
	ntAuthDN := "CN=NTAuthCertificates,CN=Public Key Services,CN=Services,CN=Configuration," + c.Cfg.LDAPConfig.DomainDN()
	entries, err := c.Cfg.LC.Read(ntAuthDN, "certificationAuthority", []string{"cACertificate"})
	if err != nil {
		return trace.Wrap(err, "fetching existing CAs: %v", err)
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
			c.Cfg.Log.Info("Teleport CA already present in NTAuthStore in LDAP")
			return nil
		}
	}

	c.Cfg.Log.Debugf("None of the %d existing NTAuthCertificates matched Teleport's", len(existingCAs))

	// CA is not in the store, append it.
	updatedCAs := make([]string, 0, len(existingCAs)+1)
	for _, existingCADER := range existingCAs {
		updatedCAs = append(updatedCAs, string(existingCADER))
	}
	updatedCAs = append(updatedCAs, string(caDER))

	if err := c.Cfg.LC.Update(ntAuthDN, map[string][]string{
		"cACertificate": updatedCAs,
	}); err != nil {
		return trace.Wrap(err, "updating CA entry: %v", err)
	}
	c.Cfg.Log.Info("Added Teleport CA to NTAuthStore via LDAP")
	return nil
}

func (c *CertificateStoreClient) updateCRL(ctx context.Context, crlDER []byte) error {
	// Publish the CRL for current cluster CA. For trusted clusters, their
	// respective windows_desktop_services will publish CRLs of their CAs so we
	// don't have to do it here.
	//
	// CRLs live under the CDP (CRL Distribution Point) LDAP container. There's
	// another nested container with the CA name, I think, and then multiple
	// separate CRL objects in that container.
	//
	// We name our parent container "Teleport" and the CRL object is named
	// after the Teleport cluster name. For example, CRL for cluster "prod"
	// will be placed at:
	// ... > CDP > Teleport > prod
	containerDN := crlContainerDN(c.Cfg.LDAPConfig)
	crlDN := crlDN(c.Cfg.ClusterName, c.Cfg.LDAPConfig)

	// Create the parent container.
	if err := c.Cfg.LC.CreateContainer(containerDN); err != nil {
		return trace.Wrap(err, "creating CRL container: %v", err)
	}

	// Create the CRL object itself.
	if err := c.Cfg.LC.Create(
		crlDN,
		"cRLDistributionPoint",
		map[string][]string{"certificateRevocationList": {string(crlDER)}},
	); err != nil {
		if !trace.IsAlreadyExists(err) {
			return trace.Wrap(err)
		}
		// CRL already exists, update it.
		if err := c.Cfg.LC.Update(
			crlDN,
			map[string][]string{"certificateRevocationList": {string(crlDER)}},
		); err != nil {
			return trace.Wrap(err)
		}
		c.Cfg.Log.Info("Updated CRL for Windows logins via LDAP")
	} else {
		c.Cfg.Log.Info("Added CRL for Windows logins via LDAP")
	}
	return nil
}

type credentialsFunc func(ctx context.Context, username, domain string, ttl time.Duration, clusterName string, ldapConfig LDAPConfig, authClient AuthInterface) (certDER, keyDER []byte, err error)

func (c *CertificateStoreClient) tlsConfigForLDAP(ctx context.Context) (*tls.Config, error) {
	// trim NETBIOS name from username
	user := c.Cfg.Username
	if i := strings.LastIndex(c.Cfg.Username, `\`); i != -1 {
		user = user[i+1:]
	}

	var cf credentialsFunc

	switch c.Cfg.CAType {
	case types.UserCA:
		cf = GenerateCredentials
	case types.DatabaseCA:
		cf = GenerateDatabaseCredentials
	default:
		return nil, trace.BadParameter("CA type: %s is unsupported for LDAP config", c.Cfg.CAType)
	}

	certDER, keyDER, err := cf(ctx, user, c.Cfg.Domain, c.Cfg.LDAPCertTTL, c.Cfg.ClusterName, c.Cfg.LDAPConfig, c.Cfg.AuthClient)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		return nil, trace.Wrap(err, "parsing cert DER")
	}

	key, err := x509.ParsePKCS1PrivateKey(keyDER)
	if err != nil {
		return nil, trace.Wrap(err, "parsing key DER")
	}

	tc := &tls.Config{
		Certificates: []tls.Certificate{
			{
				Certificate: [][]byte{cert.Raw},
				PrivateKey:  key,
			},
		},
		InsecureSkipVerify: c.Cfg.InsecureSkipVerify,
		ServerName:         c.Cfg.ServerName,
	}

	if c.Cfg.CA != nil {
		pool := x509.NewCertPool()
		pool.AddCert(c.Cfg.CA)
		tc.RootCAs = pool
	}

	return tc, nil
}

// InitializeLDAP requests a TLS certificate from the auth server to be used for
// authenticating with the LDAP server. If the certificate is obtained, and
// authentication with the LDAP server succeeds, it schedules a renewal to take
// place before the certificate expires. If we are unable to obtain a certificate
// and authenticate with the LDAP server, then the operation will be automatically
// retried.
func (c *CertificateStoreClient) InitializeLDAP(ctx context.Context) error {
	tc, err := c.tlsConfigForLDAP(ctx)
	if trace.IsAccessDenied(err) && modules.GetModules().BuildType() == modules.BuildEnterprise {
		c.Cfg.Log.Warn("Could not generate certificate for LDAPS. Ensure that the auth server is licensed.")
	}
	if err != nil {
		c.mu.Lock()
		c.ldapInitialized = false
		// in the case where we're not licensed for desktop access, we retry less frequently,
		// since this is likely not an intermittent error that will resolve itself quickly
		c.scheduleNextLDAPCertRenewalLocked(ctx, c.Cfg.RetryInterval*3)
		c.mu.Unlock()
		return trace.Wrap(err)
	}

	conn, err := ldap.DialURL("ldaps://"+c.Cfg.Addr,
		ldap.DialWithTLSDialer(tc, &net.Dialer{Timeout: ldapDialTimeout}))
	if err != nil {
		c.mu.Lock()
		c.ldapInitialized = false
		c.scheduleNextLDAPCertRenewalLocked(ctx, c.Cfg.RetryInterval)
		c.mu.Unlock()
		return trace.Wrap(err, "dial")
	}

	conn.SetTimeout(ldapRequestTimeout)
	c.Cfg.LC.SetClient(conn)

	// Note: admin still needs to import our CA into the Group Policy following
	// https://docs.vmware.com/en/VMware-Horizon-7/7.13/horizon-installation/GUID-7966AE16-D98F-430E-A916-391E8EAAFE18.html
	//
	// We can find the group policy object via LDAP, but it only contains an
	// SMB file path with the actual policy. See
	// https://en.wikipedia.org/wiki/Group_Policy
	//
	// In theory, we could update the policy file(s) over SMB following
	// https://docs.microsoft.com/en-us/previous-versions/windows/desktop/policy/registry-policy-file-format,
	// but I'm leaving this for later.
	//
	if err := c.Update(ctx); err != nil {
		return trace.Wrap(err)
	}

	c.mu.Lock()
	c.ldapInitialized = true
	c.scheduleNextLDAPCertRenewalLocked(ctx, c.Cfg.LDAPCertTTL/3)
	c.mu.Unlock()

	return nil
}

// scheduleNextLDAPCertRenewalLocked schedules a renewal of our LDAP credentials
// after some amount of time has elapsed. If an existing renewal is already
// scheduled, it is canceled and this new one takes its place.
//
// The lock on c.mu MUST be held.
func (c *CertificateStoreClient) scheduleNextLDAPCertRenewalLocked(ctx context.Context, after time.Duration) {
	if c.ldapCertRenew != nil {
		c.ldapCertRenew.Reset(after)
	} else {
		c.ldapCertRenew = time.AfterFunc(after, func() {
			if err := c.InitializeLDAP(ctx); err != nil {
				c.Cfg.Log.WithError(err).Error("couldn't renew certificate for LDAP auth")
			}
		})
	}
}

// Close closes the underlying LDAP Client
func (c *CertificateStoreClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.ldapCertRenew != nil {
		c.ldapCertRenew.Stop()
	}
	c.Cfg.LC.Close()
	return nil
}

// LDAPReady reports whether the ldap client is initialized
func (c *CertificateStoreClient) LDAPReady() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.ldapInitialized
}
