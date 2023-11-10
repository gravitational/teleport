package identity

import (
	"crypto/tls"
	"crypto/x509"
	"github.com/gravitational/teleport/api/client"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/api/utils/sshutils"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"
	"golang.org/x/net/http2"
	"sync"
)

// Assert that this UnstableClientCredentialOutput can be used as client
// credential.
var _ client.Credentials = new(Facade)

// Facade manages storing a rotating identity, and presenting it as
// something compatible with a client.Credentials
type Facade struct {
	mu       sync.RWMutex
	readyCh  chan struct{}
	identity *Identity

	fips               bool
	cipherSuites       []uint16
	insecureSkipVerify bool
}

func NewFacade(fips bool, cipherSuites []uint16, insecureSkipVerify bool) *Facade {
	return &Facade{
		readyCh:      make(chan struct{}),
		fips:         fips,
		cipherSuites: cipherSuites,
		// TODO: Implement insecureSkipVerify
		insecureSkipVerify: insecureSkipVerify,
	}
}

func (f *Facade) Set(newIdentity *Identity) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.identity = newIdentity
}

func (f *Facade) Ready() <-chan struct{} {
	return f.readyCh
}

func (f *Facade) Dialer(_ client.Config) (client.ContextDialer, error) {
	// Returning a dialer isn't necessary for this credential.
	return nil, trace.NotImplemented("no dialer")
}

func (f *Facade) TLSConfig() (*tls.Config, error) {
	// Build a "dynamic" tls.Config which can support a changing cert and root
	// CA pool.
	cfg := &tls.Config{
		// Set the default NextProto of "h2". Based on the value in
		// configureTLS()
		NextProtos: []string{http2.NextProtoTLS},

		// TODO: Make this dynamic ? Forced ?
		CipherSuites: f.cipherSuites,

		// GetClientCertificate is used instead of the static Certificates
		// field.
		Certificates: nil,
		GetClientCertificate: func(
			_ *tls.CertificateRequestInfo,
		) (*tls.Certificate, error) {
			// GetClientCertificate callback is used to allow us to dynamically
			// change the certificate when reloaded.
			f.mu.RLock()
			defer f.mu.RUnlock()
			tlsCert, err := keys.X509KeyPair(f.identity.TLSCertBytes, f.identity.PrivateKeyBytes)
			if err != nil {
				return nil, trace.BadParameter("failed to parse private key: %v", err)
			}
			return &tlsCert, nil
		},

		// VerifyConnection is used instead of the static RootCAs field.
		RootCAs: nil,
		// InsecureSkipVerify is forced true to ensure that only our
		// VerifyConnection callback is used to verify the server's presented
		// certificate.
		InsecureSkipVerify: true,
		VerifyConnection: func(state tls.ConnectionState) error {
			// This VerifyConnection callback is based on the standard library
			// implementation of verifyServerCertificate in the `tls` package.
			// We provide our own implementation so we can dynamically handle
			// a changing CA Roots pool.
			f.mu.RLock()
			defer f.mu.RUnlock()
			certPool := x509.NewCertPool()
			for j := range f.identity.TLSCACertsBytes {
				parsedCert, err := tlsca.ParseCertificatePEM(f.identity.TLSCACertsBytes[j])
				if err != nil {
					return trace.Wrap(err, "failed to parse CA certificate")
				}
				certPool.AddCert(parsedCert)
			}
			opts := x509.VerifyOptions{
				DNSName:       state.ServerName,
				Intermediates: x509.NewCertPool(),
				Roots:         certPool,
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
		// Set ServerName for SNI & Certificate Validation to the sentinel
		// teleport.cluster.local which is included on all Teleport Auth Server
		// certificates. Based on the value in configureTLS()
		// FIX THIS !!
		ServerName: apiutils.EncodeClusterName(f.identity.ClusterName),
	}

	return cfg, nil
}

func (f *Facade) SSHClientConfig() (*ssh.ClientConfig, error) {
	hostKeyCallback, err := sshutils.NewHostKeyCallback(sshutils.HostKeyCallbackConfig{
		GetHostCheckers: func() ([]ssh.PublicKey, error) {
			f.mu.RLock()
			defer f.mu.RUnlock()
			checkers, err := sshutils.ParseAuthorizedKeys(f.identity.SSHCACertBytes)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			return checkers, nil
		},
	})
	if err != nil {
		return nil, err
	}

	// Build a "dynamic" ssh config. Based roughly on
	// `sshutils.ProxyClientSSHConfig` with modifications to make it work with
	// dynamically changing credentials and CAs.
	cfg := &ssh.ClientConfig{
		Auth: []ssh.AuthMethod{
			ssh.PublicKeysCallback(func() (signers []ssh.Signer, err error) {
				f.mu.RLock()
				defer f.mu.RUnlock()
				return []ssh.Signer{f.identity.KeySigner}, nil
			}),
		},
		HostKeyCallback: hostKeyCallback,
		Timeout:         apidefaults.DefaultIOTimeout,
		// We use this because we can't always guarantee that a user will have
		// a principal other than this (they may not have access to SSH nodes)
		// and the actual user here doesn't matter for auth server API
		// authentication. All that matters is that the principal specified here
		// is stable across all certificates issued to the user, since this
		// value cannot be changed in a following rotation -
		// SSHSessionJoinPrincipal is included on all user ssh certs.
		//
		// This is a bit of a hack - the ideal solution is a refactor of the
		// API client in order to support the SSH config being generated at
		// time of use, rather than a single SSH config being made dynamic.
		// ~ noah
		User: "-teleport-internal-join",
	}
	if f.fips {
		cfg.Config = ssh.Config{
			KeyExchanges: defaults.FIPSKEXAlgorithms,
			MACs:         defaults.FIPSMACAlgorithms,
			Ciphers:      defaults.FIPSCiphers,
		}
	}
	return cfg, nil
}
