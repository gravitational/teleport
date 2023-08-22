/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package client

import (
	"crypto"
	"crypto/tls"
	"crypto/x509"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/identityfile"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/api/utils/sshutils"
	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"
	"golang.org/x/net/http2"
	"net"
	"sync"
)

// DynamicIdentityFileCreds allows a changing identity file to be used as the
// source of authentication for Client. It does not automatically watch the
// identity file or reload on an interval, this is left as an exercise for the
// consumer.
type DynamicIdentityFileCreds struct {
	// mu protects the fields that may change if the underlying identity file
	// is reloaded.
	mu            sync.RWMutex
	tlsCert       *tls.Certificate
	tlsRootCAs    *x509.CertPool
	sshCert       *ssh.Certificate
	sshKey        crypto.Signer
	sshKnownHosts []ssh.PublicKey
	sshUser       string

	// Path is the path to the identity file to load and reload.
	Path string
}

// NewDynamicIdentityFileCreds returns a DynamicIdentityFileCreds which has
// been initially loaded and is ready for use.
func NewDynamicIdentityFileCreds(path string) (*DynamicIdentityFileCreds, error) {
	d := &DynamicIdentityFileCreds{
		Path: path,
	}
	if err := d.Reload(); err != nil {
		return nil, trace.Wrap(err)
	}
	return d, nil
}

// Reload causes the identity file to be re-read from the disk. It will return
// an error if loading the credentials fails.
func (d *DynamicIdentityFileCreds) Reload() error {
	id, err := identityfile.ReadFile(d.Path)
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
	knownHosts, err := sshutils.ParseKnownHosts(id.CACerts.SSH)
	if err != nil {
		return trace.Wrap(err)
	}
	// The KeyId is not always a valid principal, so we use the first valid
	// principal instead.
	sshUser := d.sshCert.KeyId
	if len(d.sshCert.ValidPrincipals) > 0 {
		sshUser = d.sshCert.ValidPrincipals[0]
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	d.tlsRootCAs = pool
	d.tlsCert = &cert
	d.sshCert = sshCert
	d.sshKey = sshPrivateKey
	d.sshKnownHosts = knownHosts
	d.sshUser = sshUser
	return nil
}

// Dialer returns a dialer for the client to use. This is not used, but is
// needed to implement the Credentials interface.
func (d *DynamicIdentityFileCreds) Dialer(
	_ Config,
) (ContextDialer, error) {
	// Returning a dialer isn't necessary for this credential.
	return nil, trace.NotImplemented("no dialer")
}

// TLSConfig returns TLS configuration. Implementing the Credentials interface.
func (d *DynamicIdentityFileCreds) TLSConfig() (*tls.Config, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	// Build a "dynamic" tls.Config which can support a changing cert and root
	// CA pool.
	cfg := &tls.Config{
		NextProtos: []string{http2.NextProtoTLS},

		// GetClientCertificate is used instead of the static Certificates
		// field.
		Certificates: nil,
		GetClientCertificate: func(
			_ *tls.CertificateRequestInfo,
		) (*tls.Certificate, error) {
			// GetClientCertificate callback is used to allow us to dynamically
			// change the certificate when reloaded.
			d.mu.RLock()
			defer d.mu.RUnlock()
			return d.tlsCert, nil
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
			d.mu.RLock()
			defer d.mu.RUnlock()
			opts := x509.VerifyOptions{
				DNSName:       state.ServerName,
				Intermediates: x509.NewCertPool(),
				Roots:         d.tlsRootCAs,
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
		// Set the default name that's included in all Teleport issued
		// certificates.
		ServerName: constants.APIDomain,
	}

	return cfg, nil
}

// SSHClientConfig returns SSH configuration, implementing the Credentials
// interface.
func (d *DynamicIdentityFileCreds) SSHClientConfig() (*ssh.ClientConfig, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	// Build a "dynamic" ssh config. Based roughly on
	// `sshutils.ProxyClientSSHConfig` with modifications to make it work with
	// dynamically changing credentials and CAs.
	cfg := &ssh.ClientConfig{
		Auth: []ssh.AuthMethod{
			ssh.PublicKeysCallback(func() (signers []ssh.Signer, err error) {
				d.mu.RLock()
				defer d.mu.RUnlock()
				sshSigner, err := sshutils.SSHSigner(d.sshCert, d.sshKey)
				if err != nil {
					return nil, trace.Wrap(err)
				}
				return []ssh.Signer{sshSigner}, nil
			}),
		},
		HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			d.mu.RLock()
			defer d.mu.RUnlock()
			hostKeyCallback, err := sshutils.HostKeyCallback(
				d.sshKnownHosts,
				false,
			)
			if err != nil {
				return trace.Wrap(err)
			}
			return hostKeyCallback(hostname, remote, key)
		},
		Timeout: defaults.DefaultIOTimeout,
		User:    d.sshUser,
	}
	return cfg, nil
}
