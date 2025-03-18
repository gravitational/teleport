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

package client

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"net"
	"os"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/srv/alpnproxy"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

// CertChecker is a local proxy middleware that ensures certs are valid
// on start up and on each new connection.
type CertChecker struct {
	// certIssuer checks and issues certs.
	certIssuer CertIssuer
	// clock specifies the time provider. Will be used to override the time anchor
	// for TLS certificate verification. Defaults to real clock if unspecified
	clock clockwork.Clock

	cert   tls.Certificate
	certMu sync.Mutex
}

var _ alpnproxy.LocalProxyMiddleware = (*CertChecker)(nil)

// NewCertChecker creates a new CertChecker with the given CertIssuer.
func NewCertChecker(certIssuer CertIssuer, clock clockwork.Clock) *CertChecker {
	if clock == nil {
		clock = clockwork.NewRealClock()
	}

	return &CertChecker{
		certIssuer: certIssuer,
		clock:      clock,
	}
}

// CertCheckerOption is a variadic options func to set options for CertChecker functions
type CertCheckerOption func(*certCheckerOptions)

type certCheckerOptions struct {
	ttl time.Duration
}

func applyOptions(opts ...CertCheckerOption) certCheckerOptions {
	o := certCheckerOptions{}
	for _, opt := range opts {
		opt(&o)
	}
	return o
}

// WithTTL sets the TTL option.
func WithTTL(ttl time.Duration) CertCheckerOption {
	return func(options *certCheckerOptions) {
		options.ttl = ttl
	}
}

// NewDBCertChecker creates a new CertChecker for the given database.
func NewDBCertChecker(tc *TeleportClient, dbRoute tlsca.RouteToDatabase, clock clockwork.Clock, opts ...CertCheckerOption) *CertChecker {
	opt := applyOptions(opts...)
	return NewCertChecker(&DBCertIssuer{
		Client:     tc,
		RouteToApp: dbRoute,
		TTL:        opt.ttl,
	}, clock)
}

// NewAppCertChecker creates a new CertChecker for the given app.
func NewAppCertChecker(tc *TeleportClient, appRoute proto.RouteToApp, clock clockwork.Clock, opts ...CertCheckerOption) *CertChecker {
	opt := applyOptions(opts...)
	return NewCertChecker(&AppCertIssuer{
		Client:     tc,
		RouteToApp: appRoute,
		TTL:        opt.ttl,
	}, clock)
}

// OnNewConnection is a callback triggered when a new downstream connection is
// accepted by the local proxy.
func (c *CertChecker) OnNewConnection(ctx context.Context, lp *alpnproxy.LocalProxy) error {
	cert, err := c.GetOrIssueCert(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	lp.SetCert(cert)
	return nil
}

// OnStart is a callback triggered when the local proxy starts.
func (c *CertChecker) OnStart(ctx context.Context, lp *alpnproxy.LocalProxy) error {
	cert, err := c.GetOrIssueCert(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	lp.SetCert(cert)
	return nil
}

// SetCert sets the CertChecker's certificate.
func (c *CertChecker) SetCert(cert tls.Certificate) {
	c.certMu.Lock()
	defer c.certMu.Unlock()
	c.cert = cert
}

// GetOrIssueCert gets the CertChecker's certificate, or issues a new
// certificate if the it is invalid (e.g. expired) or missing.
func (c *CertChecker) GetOrIssueCert(ctx context.Context) (tls.Certificate, error) {
	c.certMu.Lock()
	defer c.certMu.Unlock()

	if err := c.checkCert(); err == nil {
		return c.cert, nil
	}

	cert, err := c.certIssuer.IssueCert(ctx)
	if err != nil {
		return tls.Certificate{}, trace.Wrap(err)
	}

	// reduce per-handshake processing by setting the parsed leaf.
	if err := utils.InitCertLeaf(&cert); err != nil {
		return tls.Certificate{}, trace.Wrap(err)
	}

	certTTL := cert.Leaf.NotAfter.Sub(c.clock.Now()).Round(time.Minute)
	log.Debugf("Certificate renewed: valid until %s [valid for %v]", cert.Leaf.NotAfter.Format(time.RFC3339), certTTL)

	c.cert = cert
	return c.cert, nil
}

func (c *CertChecker) checkCert() error {
	leaf, err := utils.TLSCertLeaf(c.cert)
	if err != nil {
		return trace.Wrap(err)
	}

	// Check for cert expiration.
	if err := utils.VerifyCertificateExpiry(leaf, c.clock); err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(c.certIssuer.CheckCert(leaf))
}

// CertIssuer checks and issues certs.
type CertIssuer interface {
	// CheckCert checks that an existing certificate is valid.
	CheckCert(cert *x509.Certificate) error
	// IssueCert issues a tls certificate.
	IssueCert(ctx context.Context) (tls.Certificate, error)
}

// DBCertIssuer checks and issues db certs.
type DBCertIssuer struct {
	// Client is a TeleportClient used to issue certificates when necessary.
	Client *TeleportClient
	// RouteToApp contains database routing information.
	RouteToApp tlsca.RouteToDatabase
	// TTL defines the maximum time-to-live for user certificates.
	// This variable sets the upper limit on the duration for which a certificate
	// remains valid. It's bounded by the `max_session_ttl` or `mfa_verification_interval`
	// if MFA is required.
	TTL time.Duration
}

func (c *DBCertIssuer) CheckCert(cert *x509.Certificate) error {
	return alpnproxy.CheckDBCertSubject(cert, c.RouteToApp)
}

func (c *DBCertIssuer) IssueCert(ctx context.Context) (tls.Certificate, error) {
	var accessRequests []string
	if profile, err := c.Client.ProfileStatus(); err != nil {
		log.WithError(err).Warn("unable to load profile, requesting database certs without access requests")
	} else {
		accessRequests = profile.ActiveRequests
	}

	var key *Key
	if err := RetryWithRelogin(ctx, c.Client, func() error {
		dbCertParams := ReissueParams{
			RouteToCluster: c.Client.SiteName,
			RouteToDatabase: proto.RouteToDatabase{
				ServiceName: c.RouteToApp.ServiceName,
				Protocol:    c.RouteToApp.Protocol,
				Username:    c.RouteToApp.Username,
				Database:    c.RouteToApp.Database,
			},
			AccessRequests: accessRequests,
			RequesterName:  proto.UserCertsRequest_TSH_DB_LOCAL_PROXY_TUNNEL,
			TTL:            c.TTL,
		}

		clusterClient, err := c.Client.ConnectToCluster(ctx)
		if err != nil {
			return trace.Wrap(err)
		}

		newKey, mfaRequired, err := clusterClient.IssueUserCertsWithMFA(ctx, dbCertParams)
		if err != nil {
			return trace.Wrap(err)
		}

		// If MFA was not required, we do not require certs be stored solely in memory.
		// Save it to disk to avoid additional roundtrips for future requests.
		if mfaRequired == proto.MFARequired_MFA_REQUIRED_NO {
			if err := c.Client.LocalAgent().AddDatabaseKey(newKey); err != nil {
				return trace.Wrap(err)
			}
		}

		key = newKey
		return trace.Wrap(err)
	}); err != nil {
		return tls.Certificate{}, trace.Wrap(err)
	}

	dbCert, err := key.DBTLSCert(c.RouteToApp.ServiceName)
	if err != nil {
		return tls.Certificate{}, trace.Wrap(err)
	}
	return dbCert, nil
}

// AppCertIssuer checks and issues app certs.
type AppCertIssuer struct {
	// Client is a TeleportClient used to issue certificates when necessary.
	Client *TeleportClient
	// RouteToApp contains app routing information.
	RouteToApp proto.RouteToApp
	// TTL defines the maximum time-to-live for user certificates.
	// This variable sets the upper limit on the duration for which a certificate
	// remains valid. It's bounded by the `max_session_ttl` or `mfa_verification_interval`
	// if MFA is required.
	TTL time.Duration
}

func (c *AppCertIssuer) CheckCert(cert *x509.Certificate) error {
	// appCertIssuer does not perform any additional certificate checks.
	return nil
}

func (c *AppCertIssuer) IssueCert(ctx context.Context) (tls.Certificate, error) {
	var accessRequests []string
	if profile, err := c.Client.ProfileStatus(); err != nil {
		log.WithError(err).Warn("unable to load profile, requesting app certs without access requests")
	} else {
		accessRequests = profile.ActiveRequests
	}

	var key *Key
	if err := RetryWithRelogin(ctx, c.Client, func() error {
		appCertParams := ReissueParams{
			RouteToCluster: c.Client.SiteName,
			RouteToApp:     c.RouteToApp,
			AccessRequests: accessRequests,
			RequesterName:  proto.UserCertsRequest_TSH_APP_LOCAL_PROXY,
			TTL:            c.TTL,
		}

		clusterClient, err := c.Client.ConnectToCluster(ctx)
		if err != nil {
			return trace.Wrap(err)
		}

		// TODO (Joerger): DELETE IN v17.0.0
		rootClient, err := clusterClient.ConnectToRootCluster(ctx)
		if err != nil {
			return trace.Wrap(err)
		}
		appCertParams.RouteToApp.SessionID, err = authclient.TryCreateAppSessionForClientCertV15(ctx, rootClient, c.Client.Username, appCertParams.RouteToApp)
		if err != nil {
			return trace.Wrap(err)
		}

		newKey, mfaRequired, err := clusterClient.IssueUserCertsWithMFA(ctx, appCertParams)
		if err != nil {
			return trace.Wrap(err)
		}

		// If MFA was not required, we do not require certs be stored solely in memory.
		// Save it to disk to avoid additional roundtrips for future requests.
		if mfaRequired == proto.MFARequired_MFA_REQUIRED_NO {
			if err := c.Client.LocalAgent().AddAppKey(newKey); err != nil {
				return trace.Wrap(err)
			}
		}

		key = newKey
		return trace.Wrap(err)
	}); err != nil {
		return tls.Certificate{}, trace.Wrap(err)
	}

	appCert, err := key.AppTLSCert(c.RouteToApp.Name)
	if err != nil {
		return tls.Certificate{}, trace.Wrap(err)
	}
	return appCert, nil
}

// LocalCertGenerator is a TLS Certificate generator used to inject
// valid TLS certificates based on SNI during local HTTPS handshakes.
type LocalCertGenerator struct {
	certChecker *CertChecker
	caPath      string

	mu sync.Mutex
	// ca is the certificate authority for signing certificates.
	ca tls.Certificate
	// certsByHost is a cache of certs for hosts generated with the local CA.
	// The key is the host's servername SNI.
	certsByHost map[string]*tls.Certificate
}

// NewLocalCertGenerator creates a new LocalCertGenerator and listens to the
// configured listen address.
func NewLocalCertGenerator(ctx context.Context, certChecker *CertChecker, caPath string) (*LocalCertGenerator, error) {
	r := &LocalCertGenerator{
		certChecker: certChecker,
		caPath:      caPath,
	}

	if err := r.ensureValidCA(ctx); err != nil {
		return nil, trace.Wrap(err)
	}

	return r, nil
}

// GetCertificate generates and returns TLS certificate for incoming
// connection. Implements tls.Config.GetCertificate.
func (r *LocalCertGenerator) GetCertificate(clientHello *tls.ClientHelloInfo) (*tls.Certificate, error) {
	if err := r.ensureValidCA(clientHello.Context()); err != nil {
		return nil, trace.Wrap(err)
	}

	cert, err := r.generateCert(clientHello.ServerName)
	if err != nil {
		return nil, trace.WrapWithMessage(err, "failed to generate certificate for %q: %v", clientHello.ServerName, err)
	}

	return cert, nil
}

// generateCert generates a new certificate for the specified host.
func (r *LocalCertGenerator) generateCert(host string) (*tls.Certificate, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if cert, found := r.certsByHost[host]; found {
		return cert, nil
	}

	certKey, err := rsa.GenerateKey(rand.Reader, constants.RSAKeySize)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	certAuthority, err := tlsca.FromTLSCertificate(r.ca)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	subject := certAuthority.Cert.Subject
	subject.CommonName = host

	certPem, err := certAuthority.GenerateCertificate(tlsca.CertificateRequest{
		PublicKey: &certKey.PublicKey,
		Subject:   subject,
		NotAfter:  certAuthority.Cert.NotAfter,
		DNSNames:  []string{host},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	keyPEM, err := keys.MarshalPrivateKey(certKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	cert, err := tls.X509KeyPair(certPem, keyPEM)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := utils.InitCertLeaf(&cert); err != nil {
		return nil, trace.Wrap(err)
	}

	r.certsByHost[host] = &cert
	return &cert, nil
}

// ensureValidCA checks if the CA is valid. If it is no longer valid, generate a new
// CA and clear the host cert cache.
func (r *LocalCertGenerator) ensureValidCA(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Check if the CA is invalid (expired)
	if err := r.checkCA(); err == nil {
		return nil
	}

	// Generate a new CA from a valid remote cert.
	remoteTLSCert, err := r.certChecker.GetOrIssueCert(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	caTLSCert, err := generateSelfSignedCAFromCert(remoteTLSCert, r.caPath)
	if err != nil {
		return trace.Wrap(err)
	}

	if err := utils.InitCertLeaf(&caTLSCert); err != nil {
		return trace.Wrap(err)
	}

	certTTL := time.Until(caTLSCert.Leaf.NotAfter).Round(time.Minute)
	log.Debugf("Local CA renewed: valid until %s [valid for %v]", caTLSCert.Leaf.NotAfter.Format(time.RFC3339), certTTL)

	// Clear cert cache and use CA for hostnames in the CA.
	r.certsByHost = make(map[string]*tls.Certificate)
	for _, host := range caTLSCert.Leaf.DNSNames {
		r.certsByHost[host] = &caTLSCert
	}

	// Requests to IPs have no server names. Default to CA.
	r.certsByHost[""] = &caTLSCert

	r.ca = caTLSCert
	return nil
}

func (r *LocalCertGenerator) checkCA() error {
	caCert, err := utils.TLSCertLeaf(r.ca)
	if err != nil {
		return trace.Wrap(err)
	}

	err = utils.VerifyCertificateExpiry(caCert, nil /*real clock*/)
	return trace.Wrap(err)
}

// generateSelfSignedCA generates a new self-signed CA for localhost
// and saves/overwrites the local CA file in the given path.
func generateSelfSignedCAFromCert(cert tls.Certificate, caPath string) (tls.Certificate, error) {
	certExpiry, err := getTLSCertExpireTime(cert)
	if err != nil {
		return tls.Certificate{}, trace.Wrap(err)
	}

	signer, ok := cert.PrivateKey.(crypto.Signer)
	if !ok {
		return tls.Certificate{}, trace.BadParameter("private key type %T does not implement crypto.Signer", cert.PrivateKey)
	}

	certPem, err := tlsca.GenerateSelfSignedCAWithConfig(tlsca.GenerateCAConfig{
		Entity: pkix.Name{
			CommonName:   "localhost",
			Organization: []string{"Teleport"},
		},
		Signer:      signer,
		DNSNames:    []string{"localhost"},
		IPAddresses: []net.IP{net.ParseIP(defaults.Localhost)},
		TTL:         time.Until(certExpiry),
	})
	if err != nil {
		return tls.Certificate{}, trace.Wrap(err)
	}

	if _, err := utils.EnsureLocalPath(caPath, "", ""); err != nil {
		return tls.Certificate{}, trace.Wrap(err)
	}

	if err = os.WriteFile(caPath, certPem, 0o600); err != nil {
		return tls.Certificate{}, trace.ConvertSystemError(err)
	}

	keyPem, err := keys.MarshalPrivateKey(signer)
	if err != nil {
		return tls.Certificate{}, trace.Wrap(err)
	}

	caCert, err := tls.X509KeyPair(certPem, keyPem)
	return caCert, trace.Wrap(err)
}

// getTLSCertExpireTime returns the certificate NotAfter time.
func getTLSCertExpireTime(cert tls.Certificate) (time.Time, error) {
	x509cert, err := utils.TLSCertLeaf(cert)
	if err != nil {
		return time.Time{}, trace.Wrap(err)
	}
	return x509cert.NotAfter, nil
}
