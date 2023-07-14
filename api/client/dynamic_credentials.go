package client

import (
	"crypto/tls"
	"crypto/x509"
	"github.com/gravitational/teleport/api/identityfile"
	"github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"
	"golang.org/x/net/http2"
	"sync"
)

type WatchedIdentityFileCreds struct {
	mu      sync.RWMutex
	tlsCert *tls.Certificate
	pool    *x509.CertPool

	path        string
	clusterName string
}

func LoadAndWatchIdentityFile(path string, clusterName string) (*WatchedIdentityFileCreds, error) {
	d := &WatchedIdentityFileCreds{
		path:        path,
		clusterName: clusterName,
	}

	err := d.Reload()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return d, nil
}

func (d *WatchedIdentityFileCreds) Reload() error {
	id, err := identityfile.ReadFile(d.path)
	if err != nil {
		return trace.Wrap(err)
	}

	// This section is essentially id.TLSConfig()
	cert, err := keys.X509KeyPair(id.Certs.TLS, id.PrivateKey)
	if err != nil {
		return trace.Wrap(err)
	}
	pool := x509.NewCertPool()
	for _, caCerts := range id.CACerts.TLS {
		if !pool.AppendCertsFromPEM(caCerts) {
			return trace.BadParameter("invalid CA cert PEM")
		}
	}

	d.mu.Lock()
	d.pool = pool
	d.tlsCert = &cert
	d.mu.Unlock()
	return nil
}

// Dialer is used to dial a connection to an Auth server.
func (d *WatchedIdentityFileCreds) Dialer(
	_ Config,
) (ContextDialer, error) {
	// Returning a dialer isn't necessary for this credential.
	return nil, trace.NotImplemented("no dialer")
}

// TLSConfig returns TLS configuration.
func (d *WatchedIdentityFileCreds) TLSConfig() (*tls.Config, error) {
	cfg := &tls.Config{
		// GetClientCertificate is used instead of the static Certificates
		// field.
		Certificates: nil,
		// Encoded cluster name required to ensure requests are routed to the
		// correct cloud tenants.
		// TODO: Make this optional - yet encouraged.
		ServerName: utils.EncodeClusterName(d.clusterName),
		GetClientCertificate: func(
			info *tls.CertificateRequestInfo,
		) (*tls.Certificate, error) {
			// GetClientCertificate callback is used to allow us to dynamically
			// change the certificate when reloaded.
			d.mu.RLock()
			defer d.mu.RUnlock()
			return d.tlsCert, nil
		},
		// InsecureSkipVerify is forced true to ensure that only our
		// VerifyConnection callback is used to verify the server's presented
		// certificate.
		InsecureSkipVerify: true,
		VerifyConnection: func(state tls.ConnectionState) error {
			// This VerifyConnection callback is based on the standard library
			// implementation of verifyServerCertificate in the tls package.
			// We provide our own implementation so we can dynamically handle
			// a changing CA Roots pool.
			d.mu.RLock()
			defer d.mu.RUnlock()

			opts := x509.VerifyOptions{
				DNSName:       state.ServerName,
				Intermediates: x509.NewCertPool(),
				Roots:         d.pool,
			}
			for _, cert := range state.PeerCertificates[1:] {
				// Whilst we don't currently use intermediate certs at
				// Teleport, including this here means that we are
				// future-proofed in case we do.
				opts.Intermediates.AddCert(cert)
			}
			_, err := state.PeerCertificates[0].Verify(opts)
			return err
		},
		NextProtos: []string{http2.NextProtoTLS},
	}

	return cfg, nil
}

// SSHClientConfig returns SSH configuration.
func (d *WatchedIdentityFileCreds) SSHClientConfig() (*ssh.ClientConfig, error) {
	// For now, SSH Client Config is disabled until I can wrap my head around
	// the changes needed to make an SSH config dynamic.
	// This means the auth server must be available directly or using
	// the ALPN/SNI.
	return nil, trace.NotImplemented("no ssh config")
}
