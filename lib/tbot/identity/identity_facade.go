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

package identity

import (
	"crypto/tls"
	"crypto/x509"
	"sync"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"
	"golang.org/x/net/http2"

	"github.com/gravitational/teleport/api/client"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/api/utils/sshutils"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils"
)

// Assert that this Facade can be used as client credential.
var _ client.Credentials = new(Facade)

// Facade manages storing a rotating identity, and presenting it as
// something compatible with a client.Credentials
type Facade struct {
	mu       sync.RWMutex
	identity *Identity

	// These don't need locking as they are configuration values that are
	// only set on construction
	fips     bool
	insecure bool
	// initialIdentity is used in some special circumstances where the value
	// must remain stable.
	initialIdentity *Identity
}

func NewFacade(
	fips bool,
	insecure bool,
	initialIdentity *Identity,
) *Facade {
	f := &Facade{
		fips:            fips,
		identity:        initialIdentity,
		insecure:        insecure,
		initialIdentity: initialIdentity,
	}

	return f
}

func (f *Facade) Get() *Identity {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.identity
}

func (f *Facade) Set(newIdentity *Identity) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.identity = newIdentity
}

func (f *Facade) Dialer(_ client.Config) (client.ContextDialer, error) {
	// Returning a dialer isn't necessary for this credential.
	return nil, trace.NotImplemented("no dialer")
}

func (f *Facade) TLSConfig() (*tls.Config, error) {
	cipherSuites := utils.DefaultCipherSuites()
	if f.fips {
		cipherSuites = defaults.FIPSCipherSuites
	}

	// Build a "dynamic" tls.Config which can support a changing cert and root
	// CA pool.
	cfg := &tls.Config{
		// Set the default NextProto of "h2". Based on the value in
		// configureTLS()
		NextProtos:   []string{http2.NextProtoTLS},
		CipherSuites: cipherSuites,

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
			return f.identity.TLSCert, nil
		},

		// VerifyConnection is actually used instead of the static RootCAs
		// field - however, we also populate the RootCAs field to work around
		// a lot of Teleport code which relies on this field. This means we
		// may not handle CA rotations when using certain connection
		RootCAs: f.initialIdentity.TLSCAPool,
		// InsecureSkipVerify is forced true to ensure that only our
		// VerifyConnection callback is used to verify the server's presented
		// certificate.
		InsecureSkipVerify: true,
		VerifyConnection: func(state tls.ConnectionState) error {
			// This VerifyConnection callback is based on the standard library
			// implementation of verifyServerCertificate in the `tls` package.
			// We provide our own implementation so we can dynamically handle
			// a changing CA Roots pool.
			if f.insecure {
				return nil
			}
			caPool := func() *x509.CertPool {
				f.mu.RLock()
				defer f.mu.RUnlock()
				return f.identity.TLSCAPool
			}()
			opts := x509.VerifyOptions{
				DNSName: state.ServerName,
				Roots:   caPool,
			}
			if len(state.PeerCertificates) > 1 {
				opts.Intermediates = x509.NewCertPool()
				for _, cert := range state.PeerCertificates[1:] {
					// Whilst we don't currently use intermediate certs at
					// Teleport, including this here means that we are
					// future-proofed in case we do.
					opts.Intermediates.AddCert(cert)
				}
			}
			_, err := state.PeerCertificates[0].Verify(opts)
			return err
		},
		ServerName: apiutils.EncodeClusterName(f.initialIdentity.ClusterName),
	}

	return cfg, nil
}

func (f *Facade) SSHClientConfig() (*ssh.ClientConfig, error) {
	hostKeyCallback, err := sshutils.NewHostKeyCallback(sshutils.HostKeyCallbackConfig{
		GetHostCheckers: func() ([]ssh.PublicKey, error) {
			f.mu.RLock()
			defer f.mu.RUnlock()
			return f.identity.SSHHostCheckers, nil
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
