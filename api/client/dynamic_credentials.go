package client

import (
	"crypto"
	"crypto/tls"
	"crypto/x509"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/identityfile"
	"github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/api/utils/sshutils"
	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"
	"golang.org/x/net/http2"
	"sync"
)

type WatchedIdentityFileCreds struct {
	mu      sync.RWMutex
	tlsCert *tls.Certificate
	pool    *x509.CertPool
	sshCert *ssh.Certificate
	sshKey  crypto.Signer

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

	// This sections is essentially id.SSHClientConfig()
	sshCert, err := sshutils.ParseCertificate(id.Certs.SSH)
	if err != nil {
		return trace.Wrap(err)
	}
	sshPrivateKey, err := keys.ParsePrivateKey(id.PrivateKey)
	if err != nil {
		return trace.Wrap(err)
	}
	_, err := sshutils.ProxyClientSSHConfig(
		sshCert, sshPrivateKey, id.CACerts.SSH...,
	)
	if err != nil {
		return trace.Wrap(err)
	}

	d.mu.Lock()
	d.pool = pool
	d.tlsCert = &cert
	d.sshCert = sshCert
	d.sshKey = sshPrivateKey
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

	// If the cluster name has been provided, we should use that for SNI.
	// This enables alpn routed connections on Teleport Cloud which relies on
	// a distinct SNI to route traffic to the correct Teleport Tenant.
	// TODO: Could we determine this from the cert 🤔 and not need to ask
	// the user explicitly for this.
	if d.clusterName != "" {
		cfg.ServerName = utils.EncodeClusterName(d.clusterName)
	} else {
		// Otherwise fall back to `teleport.cluster.local` which should work
		// in most general teleport installs.
		cfg.ServerName = constants.APIDomain
	}

	return cfg, nil
}

// SSHClientConfig returns SSH configuration.
func (d *WatchedIdentityFileCreds) SSHClientConfig() (*ssh.ClientConfig, error) {
	// Build a "dynamic" ssh config. Based roughly on
	// `sshutils.ProxyClientSSHConfig` with modifications to make it work with
	// dynamically changing credentials.
	cfg := &ssh.ClientConfig{
		Auth: []ssh.AuthMethod{
			ssh.PublicKeysCallback(func() (signers []ssh.Signer, err error) {
				d.mu.RLock()
				defer d.mu.Unlock()

				sshSigner, err := sshutils.SSHSigner(d.sshCert, d.sshKey)
				if err != nil {
					return nil, trace.Wrap(err)
				}
				return []ssh.Signer{sshSigner}, nil
			}),
		},
	}

	return cfg, nil
}
