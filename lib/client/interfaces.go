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
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"runtime"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/native"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/sshutils"
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
	// TLSCert is a PEM encoded client TLS x509 certificate.
	// It's used to authenticate to the Teleport APIs.
	TLSCert []byte `json:"TLSCert,omitempty"`
	// KubeTLSCerts are TLS certificates (PEM-encoded) for individual
	// kubernetes clusters. Map key is a kubernetes cluster name.
	KubeTLSCerts map[string][]byte `json:"KubeCerts,omitempty"`
	// DBTLSCerts are PEM-encoded TLS certificates for database access.
	// Map key is the database service name.
	DBTLSCerts map[string][]byte `json:"DBCerts,omitempty"`

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
		Priv:         priv,
		Pub:          pub,
		KubeTLSCerts: make(map[string][]byte),
		DBTLSCerts:   make(map[string][]byte),
	}, nil
}

// TLSCAs returns all TLS CA certificates from this key
func (k *Key) TLSCAs() (result [][]byte) {
	for _, ca := range k.TrustedCA {
		result = append(result, ca.TLSCertificates...)
	}
	return result
}

// SSHCAs returns all SSH CA certificates from this key
func (k *Key) SSHCAs() (result [][]byte) {
	for _, ca := range k.TrustedCA {
		result = append(result, ca.HostCertificates...)
	}
	return result
}

// KubeClientTLSConfig returns client TLS configuration used
// to authenticate against kubernetes servers.
func (k *Key) KubeClientTLSConfig(cipherSuites []uint16, kubeClusterName string) (*tls.Config, error) {
	tlsCert, ok := k.KubeTLSCerts[kubeClusterName]
	if !ok {
		return nil, trace.NotFound("TLS certificate for kubernetes cluster %q not found", kubeClusterName)
	}
	return k.clientTLSConfig(cipherSuites, tlsCert)
}

// TeleportClientTLSConfig returns client TLS configuration used
// to authenticate against API servers.
func (k *Key) TeleportClientTLSConfig(cipherSuites []uint16) (*tls.Config, error) {
	return k.clientTLSConfig(cipherSuites, k.TLSCert)
}

func (k *Key) clientTLSConfig(cipherSuites []uint16, tlsCertRaw []byte) (*tls.Config, error) {
	tlsConfig := utils.TLSConfig(cipherSuites)

	pool := x509.NewCertPool()
	for _, ca := range k.TrustedCA {
		for _, certPEM := range ca.TLSCertificates {
			if !pool.AppendCertsFromPEM(certPEM) {
				return nil, trace.BadParameter("failed to parse certificate received from the proxy")
			}
		}
	}
	tlsConfig.RootCAs = pool
	tlsCert, err := tls.X509KeyPair(tlsCertRaw, k.Priv)
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

// ClientSSHConfig returns an ssh.ClientConfig with SSH credentials from this
// Key and HostKeyCallback matching SSH CAs in the Key.
func (k *Key) ClientSSHConfig() (*ssh.ClientConfig, error) {
	username, err := k.CertUsername()
	if err != nil {
		return nil, trace.Wrap(err, "failed to extract username from SSH certificate")
	}
	authMethod, err := k.AsAuthMethod()
	if err != nil {
		return nil, trace.Wrap(err, "failed to convert identity file to auth method")
	}
	hostKeyCallback, err := k.HostKeyCallback()
	if err != nil {
		return nil, trace.Wrap(err, "failed to convert identity file to HostKeyCallback")
	}
	return &ssh.ClientConfig{
		User:            username,
		Auth:            []ssh.AuthMethod{authMethod},
		HostKeyCallback: hostKeyCallback,
		Timeout:         defaults.DefaultDialTimeout,
	}, nil
}

// CertUsername returns the name of the Teleport user encoded in the SSH certificate.
func (k *Key) CertUsername() (string, error) {
	cert, err := k.SSHCert()
	if err != nil {
		return "", trace.Wrap(err)
	}
	return cert.KeyId, nil
}

// CertPrincipals returns the principals listed on the SSH certificate.
func (k *Key) CertPrincipals() ([]string, error) {
	cert, err := k.SSHCert()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return cert.ValidPrincipals, nil
}

func (k *Key) CertRoles() ([]string, error) {
	cert, err := k.SSHCert()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// Extract roles from certificate. Note, if the certificate is in old format,
	// this will be empty.
	var roles []string
	rawRoles, ok := cert.Extensions[teleport.CertExtensionTeleportRoles]
	if ok {
		roles, err = services.UnmarshalCertRoles(rawRoles)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return roles, nil
}

// AsAgentKeys converts client.Key struct to a []*agent.AddedKey. All elements
// of the []*agent.AddedKey slice need to be loaded into the agent!
func (k *Key) AsAgentKeys() ([]agent.AddedKey, error) {
	// unmarshal certificate bytes into a ssh.PublicKey
	cert, err := k.SSHCert()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// unmarshal private key bytes into a *rsa.PrivateKey
	privateKey, err := ssh.ParseRawPrivateKey(k.Priv)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// put a teleport identifier along with the teleport user into the comment field
	comment := fmt.Sprintf("teleport:%v", cert.KeyId)

	// On Windows, return the certificate with the private key embedded.
	if runtime.GOOS == teleport.WindowsOS {
		return []agent.AddedKey{
			{
				PrivateKey:       privateKey,
				Certificate:      cert,
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
	return []agent.AddedKey{
		{
			PrivateKey:       privateKey,
			Certificate:      cert,
			Comment:          comment,
			LifetimeSecs:     0,
			ConfirmBeforeUse: false,
		},
		{
			PrivateKey:       privateKey,
			Certificate:      nil,
			Comment:          comment,
			LifetimeSecs:     0,
			ConfirmBeforeUse: false,
		},
	}, nil
}

// TeleportTLSCertificate returns the parsed x509 certificate for
// authentication against Teleport APIs.
func (k *Key) TeleportTLSCertificate() (*x509.Certificate, error) {
	return tlsca.ParseCertificatePEM(k.TLSCert)
}

// KubeTLSCertificate returns the parsed x509 certificate for
// authentication against a named kubernetes cluster.
func (k *Key) KubeTLSCertificate(kubeClusterName string) (*x509.Certificate, error) {
	tlsCert, ok := k.KubeTLSCerts[kubeClusterName]
	if !ok {
		return nil, trace.NotFound("TLS certificate for kubernetes cluster %q not found", kubeClusterName)
	}
	return tlsca.ParseCertificatePEM(tlsCert)
}

// DBTLSCertificates returns all parsed x509 database access certificates.
func (k *Key) DBTLSCertificates() (certs []x509.Certificate, err error) {
	for _, bytes := range k.DBTLSCerts {
		cert, err := tlsca.ParseCertificatePEM(bytes)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		certs = append(certs, *cert)
	}
	return certs, nil
}

// TeleportTLSCertValidBefore returns the time of the TLS cert expiration
func (k *Key) TeleportTLSCertValidBefore() (t time.Time, err error) {
	cert, err := tlsca.ParseCertificatePEM(k.TLSCert)
	if err != nil {
		return t, trace.Wrap(err)
	}
	return cert.NotAfter, nil
}

// CertValidBefore returns the time of the cert expiration
func (k *Key) CertValidBefore() (t time.Time, err error) {
	cert, err := k.SSHCert()
	if err != nil {
		return t, trace.Wrap(err)
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

// HostKeyCallback returns an ssh.HostKeyCallback that validates host
// keys/certs against SSH CAs in the Key.
//
// If not CAs are present in the Key, the returned ssh.HostKeyCallback is nil.
// This causes golang.org/x/crypto/ssh to prompt the user to verify host key
// fingerprint (same as OpenSSH does for an unknown host).
func (k *Key) HostKeyCallback() (ssh.HostKeyCallback, error) {
	var trustedKeys []ssh.PublicKey
	for _, caCert := range k.SSHCAs() {
		_, _, publicKey, _, _, err := ssh.ParseKnownHosts(caCert)
		if err != nil {
			return nil, trace.BadParameter("failed parsing CA cert: %v; raw CA cert line: %q", err, caCert)
		}
		trustedKeys = append(trustedKeys, publicKey)
	}
	// No CAs are provided, return a nil callback which will prompt the user
	// for trust.
	if len(trustedKeys) == 0 {
		return nil, nil
	}

	return func(host string, a net.Addr, hostKey ssh.PublicKey) error {
		clusterCert, ok := hostKey.(*ssh.Certificate)
		if ok {
			hostKey = clusterCert.SignatureKey
		}
		for _, trustedKey := range trustedKeys {
			if sshutils.KeysEqual(trustedKey, hostKey) {
				return nil
			}
		}
		return trace.AccessDenied("host %v is untrusted or Teleport CA has been rotated; try getting new credentials by logging in again ('tsh login') or re-exporting the identity file ('tctl auth sign' or 'tsh login -o'), depending on how you got them initially", host)
	}, nil
}

// ProxyClientSSHConfig returns an ssh.ClientConfig with SSH credentials from this
// Key and HostKeyCallback matching SSH CAs in the Key.
//
// The config is set up to authenticate to proxy with the first
// available principal and trust local SSH CAs without asking
// for public keys.
//
func ProxyClientSSHConfig(k *Key, keyStore LocalKeyStore) (*ssh.ClientConfig, error) {
	sshConfig, err := k.ClientSSHConfig()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	principals, err := k.CertPrincipals()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	sshConfig.User = principals[0]
	sshConfig.HostKeyCallback = NewKeyStoreCertChecker(keyStore)
	return sshConfig, nil
}
