/*
Copyright 2015-2017 Gravitational, Inc.

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
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"runtime"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/native"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

// Key describes a complete (signed) client key
type Key struct {
	// Priv is a PEM encoded private key
	Priv []byte `json:"Priv,omitempty"`
	// Pub is a public key
	Pub []byte `json:"Pub,omitempty"`
	// Cert is an SSH client certificate
	Cert []byte `json:"Cert,omitempty"`
	// TLSCert is a PEM encoded client TLS x509 certificate
	TLSCert []byte `json:"TLSCert,omitempty"`

	// ProxyHost (optionally) contains the hostname of the proxy server
	// which issued this key
	ProxyHost string

	// TrustedCA is a list of trusted certificate authorities
	TrustedCA []auth.TrustedCerts

	// ClusterName is a cluster name this key is associated with
	ClusterName string
}

// NewKey generates a new unsigned key. Such key must be signed by a
// Teleport CA (auth server) before it becomes useful.
func NewKey() (key *Key, err error) {
	priv, pub, err := native.GenerateKeyPair("")
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &Key{
		Priv: priv,
		Pub:  pub,
	}, nil
}

// TLSCAs returns all TLS CA certificates from this key
func (k *Key) TLSCAs() (result [][]byte) {
	for _, ca := range k.TrustedCA {
		result = append(result, ca.TLSCertificates...)
	}
	return result
}

// TLSConfig returns client TLS configuration used
// to authenticate against API servers
func (k *Key) ClientTLSConfig() (*tls.Config, error) {
	// Because Teleport clients can't be configured (yet), they take the default
	// list of cipher suites from Go.
	tlsConfig := utils.TLSConfig(nil)

	pool := x509.NewCertPool()
	for _, ca := range k.TrustedCA {
		for _, certPEM := range ca.TLSCertificates {
			if !pool.AppendCertsFromPEM(certPEM) {
				return nil, trace.BadParameter("failed to parse certificate received from the proxy")
			}
		}
	}
	tlsConfig.RootCAs = pool
	tlsCert, err := tls.X509KeyPair(k.TLSCert, k.Priv)
	if err != nil {
		return nil, trace.Wrap(err, "failed to parse TLS cert and key")
	}
	tlsConfig.Certificates = append(tlsConfig.Certificates, tlsCert)
	// Use Issuer CN from the certificate to populate the correct SNI in
	// requests.
	leaf, err := x509.ParseCertificate(tlsCert.Certificate[0])
	if err != nil {
		return nil, trace.Wrap(err, "failed to parse TLS cert")
	}
	tlsConfig.ServerName = auth.EncodeClusterName(leaf.Issuer.CommonName)
	return tlsConfig, nil
}

// CertUsername returns the name of the Teleport user encoded in the SSH certificate.
func (k *Key) CertUsername() (string, error) {
	pubKey, _, _, _, err := ssh.ParseAuthorizedKey(k.Cert)
	if err != nil {
		return "", trace.Wrap(err)
	}
	cert, ok := pubKey.(*ssh.Certificate)
	if !ok {
		return "", trace.BadParameter("expected SSH certificate, got public key")
	}
	return cert.KeyId, nil
}

// CertPrincipals returns the principals listed on the SSH certificate.
func (k *Key) CertPrincipals() ([]string, error) {
	publicKey, _, _, _, err := ssh.ParseAuthorizedKey(k.Cert)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	cert, ok := publicKey.(*ssh.Certificate)
	if !ok {
		return nil, trace.BadParameter("no certificate found")
	}
	return cert.ValidPrincipals, nil
}

// AsAgentKeys converts client.Key struct to a []*agent.AddedKey. All elements
// of the []*agent.AddedKey slice need to be loaded into the agent!
func (k *Key) AsAgentKeys() ([]*agent.AddedKey, error) {
	// unmarshal certificate bytes into a ssh.PublicKey
	publicKey, _, _, _, err := ssh.ParseAuthorizedKey(k.Cert)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// unmarshal private key bytes into a *rsa.PrivateKey
	privateKey, err := ssh.ParseRawPrivateKey(k.Priv)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// put a teleport identifier along with the teleport user into the comment field
	comment := fmt.Sprintf("teleport:%v", publicKey.(*ssh.Certificate).KeyId)

	// On Windows, return the certificate with the private key embedded.
	if runtime.GOOS == teleport.WindowsOS {
		return []*agent.AddedKey{
			&agent.AddedKey{
				PrivateKey:       privateKey,
				Certificate:      publicKey.(*ssh.Certificate),
				Comment:          comment,
				LifetimeSecs:     0,
				ConfirmBeforeUse: false,
			},
		}, nil
	}

	// On Unix, return the certificate (with embedded private key) as well as
	// a private key.
	//
	// This is done because OpenSSH clients older than OpenSSH 7.3/7.3p1
	// (2016-08-01) have a bug in how they use certificates that have been loaded
	// in an agent. Specifically when you add a certificate to an agent, you can't
	// just embed the private key within the certificate, you have to add the
	// certificate and private key to the agent separately. Teleport works around
	// this behavior to ensure OpenSSH interoperability.
	//
	// For more details see the following: https://bugzilla.mindrot.org/show_bug.cgi?id=2550
	// WARNING: callers expect the returned slice to be __exactly as it is__
	return []*agent.AddedKey{
		&agent.AddedKey{
			PrivateKey:       privateKey,
			Certificate:      publicKey.(*ssh.Certificate),
			Comment:          comment,
			LifetimeSecs:     0,
			ConfirmBeforeUse: false,
		},
		&agent.AddedKey{
			PrivateKey:       privateKey,
			Certificate:      nil,
			Comment:          comment,
			LifetimeSecs:     0,
			ConfirmBeforeUse: false,
		},
	}, nil
}

// EqualsTo returns true if this key is the same as the other.
// Primarily used in tests
func (k *Key) EqualsTo(other *Key) bool {
	if k == other {
		return true
	}
	return bytes.Equal(k.Cert, other.Cert) &&
		bytes.Equal(k.Priv, other.Priv) &&
		bytes.Equal(k.Pub, other.Pub) &&
		bytes.Equal(k.TLSCert, other.TLSCert)
}

// TLSCertificate returns x509 certificate
func (k *Key) TLSCertificate() (*x509.Certificate, error) {
	return tlsca.ParseCertificatePEM(k.TLSCert)
}

// TLSCertValidBefore returns the time of the TLS cert expiration
func (k *Key) TLSCertValidBefore() (t time.Time, err error) {
	cert, err := tlsca.ParseCertificatePEM(k.TLSCert)
	if err != nil {
		return t, trace.Wrap(err)
	}
	return cert.NotAfter, nil
}

// CertValidBefore returns the time of the cert expiration
func (k *Key) CertValidBefore() (t time.Time, err error) {
	pcert, _, _, _, err := ssh.ParseAuthorizedKey(k.Cert)
	if err != nil {
		return t, trace.Wrap(err)
	}
	cert, ok := pcert.(*ssh.Certificate)
	if !ok {
		return t, trace.Errorf("not supported certificate type")
	}
	return time.Unix(int64(cert.ValidBefore), 0), nil
}

// AsAuthMethod returns an "auth method" interface, a common abstraction
// used by Golang SSH library. This is how you actually use a Key to feed
// it into the SSH lib.
func (k *Key) AsAuthMethod() (ssh.AuthMethod, error) {
	keys, err := k.AsAgentKeys()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	signer, err := ssh.NewSignerFromKey(keys[0].PrivateKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if signer, err = ssh.NewCertSigner(keys[0].Certificate, signer); err != nil {
		return nil, trace.Wrap(err)
	}
	return NewAuthMethodForCert(signer), nil
}

// SSHCert returns parsed SSH certificate
func (k *Key) SSHCert() (*ssh.Certificate, error) {
	key, _, _, _, err := ssh.ParseAuthorizedKey(k.Cert)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cert, ok := key.(*ssh.Certificate)
	if !ok {
		return nil, trace.BadParameter("found key, not certificate")
	}
	return cert, nil
}

// CheckCert makes sure the SSH certificate is valid.
func (k *Key) CheckCert() error {
	cert, err := k.SSHCert()
	if err != nil {
		return trace.Wrap(err)
	}

	// A valid principal is always passed in because the principals are not being
	// checked here, but rather the validity period, signature, and algorithms.
	certChecker := utils.CertChecker{
		FIPS: isFIPS(),
	}
	err = certChecker.CheckCert(cert.ValidPrincipals[0], cert)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}
