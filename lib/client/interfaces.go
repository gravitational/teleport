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
	"bytes"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rsa"
	"crypto/subtle"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"strings"
	"time"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"

	"github.com/gravitational/teleport/api/constants"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/api/utils/sshutils"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/auth/native"
	"github.com/gravitational/teleport/lib/sshca"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

// KeyIndex helps to identify a key in the store.
type KeyIndex struct {
	// ProxyHost is the root proxy hostname that a key is associated with.
	ProxyHost string
	// Username is the username that a key is associated with.
	Username string
	// ClusterName is the cluster name that a key is associated with.
	ClusterName string
}

// Check verifies the KeyIndex is fully specified.
func (idx KeyIndex) Check() error {
	missingField := "key index field %s is not set"
	if idx.ProxyHost == "" {
		return trace.BadParameter(missingField, "ProxyHost")
	}
	if idx.Username == "" {
		return trace.BadParameter(missingField, "Username")
	}
	if idx.ClusterName == "" {
		return trace.BadParameter(missingField, "ClusterName")
	}
	return nil
}

// Match compares this key index to the given matchKey index.
// It will be considered a match if all non-zero elements of
// the matchKey are matched by this key index.
func (idx KeyIndex) Match(matchKey KeyIndex) bool {
	return (matchKey.ProxyHost == "" || matchKey.ProxyHost == idx.ProxyHost) &&
		(matchKey.ClusterName == "" || matchKey.ClusterName == idx.ClusterName) &&
		(matchKey.Username == "" || matchKey.Username == idx.Username)
}

// Key describes a complete (signed) client key
type Key struct {
	KeyIndex

	// PrivateKey is a private key used for cryptographical operations.
	*keys.PrivateKey

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
	// AppTLSCerts are TLS certificates for application access.
	// Map key is the application name.
	AppTLSCerts map[string][]byte `json:"AppCerts,omitempty"`
	// WindowsDesktopCerts are TLS certificates for Windows Desktop access.
	// Map key is the desktop server name.
	WindowsDesktopCerts map[string][]byte `json:"WindowsDesktopCerts,omitempty"`
	// TrustedCerts is a list of trusted certificate authorities
	TrustedCerts []authclient.TrustedCerts
}

// Copy returns a shallow copy of k, or nil if k is nil.
func (k *Key) Copy() *Key {
	if k == nil {
		return nil
	}
	copy := *k
	return &copy
}

// GenerateRSAKey generates a new unsigned key.
func GenerateRSAKey() (*Key, error) {
	priv, err := native.GeneratePrivateKey()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return NewKey(priv), nil
}

// NewKey creates a new Key for the given private key.
func NewKey(priv *keys.PrivateKey) *Key {
	return &Key{
		PrivateKey:          priv,
		KubeTLSCerts:        make(map[string][]byte),
		DBTLSCerts:          make(map[string][]byte),
		AppTLSCerts:         make(map[string][]byte),
		WindowsDesktopCerts: make(map[string][]byte),
	}
}

// RootClusterCAs returns root cluster CAs.
func (k *Key) RootClusterCAs() ([][]byte, error) {
	rootClusterName, err := k.RootClusterName()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var out [][]byte
	for _, cas := range k.TrustedCerts {
		for _, v := range cas.TLSCertificates {
			cert, err := tlsca.ParseCertificatePEM(v)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			if cert.Subject.CommonName == rootClusterName {
				out = append(out, v)
			}
		}
	}
	if len(out) > 0 {
		return out, nil
	}
	return nil, trace.NotFound("failed to find TLS CA for %q root cluster", rootClusterName)
}

// TLSCAs returns all TLS CA certificates from this key
func (k *Key) TLSCAs() (result [][]byte) {
	for _, ca := range k.TrustedCerts {
		result = append(result, ca.TLSCertificates...)
	}
	return result
}

func (k *Key) KubeClientTLSConfig(cipherSuites []uint16, kubeClusterName string) (*tls.Config, error) {
	rootCluster, err := k.RootClusterName()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tlsCert, ok := k.KubeTLSCerts[kubeClusterName]
	if !ok {
		return nil, trace.NotFound("TLS certificate for kubernetes cluster %q not found", kubeClusterName)
	}

	tlsConfig, err := k.clientTLSConfig(cipherSuites, tlsCert, []string{rootCluster})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tlsConfig.ServerName = fmt.Sprintf("%s%s", constants.KubeTeleportProxyALPNPrefix, constants.APIDomain)
	return tlsConfig, nil
}

// HostKeyCallback returns an ssh.HostKeyCallback that validates host
// keys/certs against SSH CAs in the Key.
//
// If not CAs are present in the Key, the returned ssh.HostKeyCallback is nil.
// This causes golang.org/x/crypto/ssh to prompt the user to verify host key
// fingerprint (same as OpenSSH does for an unknown host).
func (k *Key) HostKeyCallback(hostnames ...string) (ssh.HostKeyCallback, error) {
	trustedHostKeys, err := k.authorizedHostKeys(hostnames...)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return sshutils.HostKeyCallback(trustedHostKeys, true)
}

// authorizedHostKeys returns all authorized host keys from this key. If any host
// names are provided, only matching host keys will be returned.
func (k *Key) authorizedHostKeys(hostnames ...string) ([]ssh.PublicKey, error) {
	var hostKeys []ssh.PublicKey
	for _, ca := range k.TrustedCerts {
		// Mirror the hosts we would find in a known_hosts entry.
		hosts := []string{k.ProxyHost, ca.ClusterName, "*." + ca.ClusterName}

		if len(hostnames) == 0 || sshutils.HostNameMatch(hostnames, hosts) {
			for _, authorizedKey := range ca.AuthorizedKeys {
				sshPub, _, _, _, err := ssh.ParseAuthorizedKey(authorizedKey)
				if err != nil {
					return nil, trace.Wrap(err)
				}
				hostKeys = append(hostKeys, sshPub)
			}
		}
	}
	return hostKeys, nil
}

// TeleportClientTLSConfig returns client TLS configuration used
// to authenticate against API servers.
func (k *Key) TeleportClientTLSConfig(cipherSuites []uint16, clusters []string) (*tls.Config, error) {
	if len(k.TLSCert) == 0 {
		return nil, trace.NotFound("TLS certificate not found")
	}
	return k.clientTLSConfig(cipherSuites, k.TLSCert, clusters)
}

func (k *Key) clientTLSConfig(cipherSuites []uint16, tlsCertRaw []byte, clusters []string) (*tls.Config, error) {
	tlsCert, err := k.TLSCertificate(tlsCertRaw)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	pool, err := k.clientCertPool(clusters...)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	tlsConfig := utils.TLSConfig(cipherSuites)
	tlsConfig.RootCAs = pool
	tlsConfig.Certificates = append(tlsConfig.Certificates, tlsCert)
	// Use Issuer CN from the certificate to populate the correct SNI in
	// requests.
	leaf, err := x509.ParseCertificate(tlsCert.Certificate[0])
	if err != nil {
		return nil, trace.Wrap(err, "failed to parse TLS cert")
	}
	tlsConfig.ServerName = apiutils.EncodeClusterName(leaf.Issuer.CommonName)
	return tlsConfig, nil
}

// ClientCertPool returns x509.CertPool containing trusted CA.
func (k *Key) clientCertPool(clusters ...string) (*x509.CertPool, error) {
	pool := x509.NewCertPool()
	for _, caPEM := range k.TLSCAs() {
		cert, err := tlsca.ParseCertificatePEM(caPEM)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		for _, k := range clusters {
			if cert.Subject.CommonName == k {
				if !pool.AppendCertsFromPEM(caPEM) {
					return nil, trace.BadParameter("failed to parse TLS CA certificate")
				}
			}
		}
	}
	return pool, nil
}

// ProxyClientSSHConfig returns an ssh.ClientConfig with SSH credentials from this
// Key and HostKeyCallback matching SSH CAs in the Key.
//
// The config is set up to authenticate to proxy with the first available principal
// and ( if keyStore != nil ) trust local SSH CAs without asking for public keys.
func (k *Key) ProxyClientSSHConfig(hostname string) (*ssh.ClientConfig, error) {
	sshCert, err := k.SSHCert()
	if err != nil {
		return nil, trace.Wrap(err, "failed to extract username from SSH certificate")
	}

	sshConfig, err := sshutils.ProxyClientSSHConfig(sshCert, k)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	sshConfig.HostKeyCallback, err = k.HostKeyCallback(hostname)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return sshConfig, nil
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

	ident, err := sshca.DecodeIdentity(cert)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return ident.Roles, nil
}

const (
	agentKeyCommentPrefix    = "teleport"
	agentKeyCommentSeparator = ":"
)

// teleportAgentKeyComment returns a teleport agent key comment
// like "teleport:<proxyHost>:<userName>:<clusterName>".
func teleportAgentKeyComment(k KeyIndex) string {
	return strings.Join([]string{
		agentKeyCommentPrefix,
		k.ProxyHost,
		k.ClusterName,
		k.Username,
	}, agentKeyCommentSeparator)
}

// parseTeleportAgentKeyComment parses an agent key comment into
// its associated KeyIndex.
func parseTeleportAgentKeyComment(comment string) (KeyIndex, bool) {
	parts := strings.Split(comment, agentKeyCommentSeparator)
	if len(parts) != 4 || parts[0] != agentKeyCommentPrefix {
		return KeyIndex{}, false
	}

	return KeyIndex{
		ProxyHost:   parts[1],
		ClusterName: parts[2],
		Username:    parts[3],
	}, true
}

// isTeleportAgentKey returns whether the given agent key was added
// by Teleport by checking the key's comment.
func isTeleportAgentKey(key *agent.Key) bool {
	return strings.HasPrefix(key.Comment, agentKeyCommentPrefix+agentKeyCommentSeparator)
}

// AsAgentKey converts client.Key struct to an agent.AddedKey. Any agent.AddedKey
// can be added to a local agent (keyring), nut non-standard keys cannot be added
// to an SSH system agent through the ssh agent protocol. Check canAddToSystemAgent
// before adding this key to an SSH system agent.
func (k *Key) AsAgentKey() (agent.AddedKey, error) {
	sshCert, err := k.SSHCert()
	if err != nil {
		return agent.AddedKey{}, trace.Wrap(err)
	}

	return agent.AddedKey{
		PrivateKey:       k.Signer,
		Certificate:      sshCert,
		Comment:          teleportAgentKeyComment(k.KeyIndex),
		LifetimeSecs:     0,
		ConfirmBeforeUse: false,
	}, nil
}

// canAddToSystemAgent returns whether this agent key can be added to an SSH system agent.
// Non-standard private keys will return false.
func canAddToSystemAgent(agentKey agent.AddedKey) bool {
	switch agentKey.PrivateKey.(type) {
	case *rsa.PrivateKey, *ecdsa.PrivateKey, ed25519.PrivateKey:
		return true
	default:
		return false
	}
}

// TeleportTLSCertificate returns the parsed x509 certificate for
// authentication against Teleport APIs.
func (k *Key) TeleportTLSCertificate() (*x509.Certificate, error) {
	if len(k.TLSCert) == 0 {
		return nil, trace.NotFound("TLS certificate not found")
	}
	return tlsca.ParseCertificatePEM(k.TLSCert)
}

// KubeX509Cert returns the parsed x509 certificate for authentication against
// a named kubernetes cluster.
func (k *Key) KubeX509Cert(kubeClusterName string) (*x509.Certificate, error) {
	tlsCert, ok := k.KubeTLSCerts[kubeClusterName]
	if !ok {
		return nil, trace.NotFound("TLS certificate for kubernetes cluster %q not found", kubeClusterName)
	}
	return tlsca.ParseCertificatePEM(tlsCert)
}

// KubeTLSCert returns the tls.Certificate for authentication against a named
// kubernetes cluster.
func (k *Key) KubeTLSCert(kubeClusterName string) (tls.Certificate, error) {
	certPem, ok := k.KubeTLSCerts[kubeClusterName]
	if !ok {
		return tls.Certificate{}, trace.NotFound("TLS certificate for kubernetes cluster %q not found", kubeClusterName)
	}
	tlsCert, err := k.PrivateKey.TLSCertificate(certPem)
	if err != nil {
		return tls.Certificate{}, trace.Wrap(err)
	}
	return tlsCert, nil
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

// AppTLSCertificates returns all parsed x509 app access certificates.
func (k *Key) AppTLSCertificates() (certs []x509.Certificate, err error) {
	for _, bytes := range k.AppTLSCerts {
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
	cert, err := k.TeleportTLSCertificate()
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
	cert, err := k.SSHCert()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return sshutils.AsAuthMethod(cert, k)
}

// SSHSigner returns an ssh.Signer using the SSH certificate in this key.
func (k *Key) SSHSigner() (ssh.Signer, error) {
	cert, err := k.SSHCert()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return sshutils.SSHSigner(cert, k)
}

// SSHCert returns parsed SSH certificate
func (k *Key) SSHCert() (*ssh.Certificate, error) {
	if k.Cert == nil {
		return nil, trace.NotFound("SSH cert not found")
	}
	return sshutils.ParseCertificate(k.Cert)
}

// ActiveRequests gets the active requests associated with this key.
func (k *Key) ActiveRequests() ([]string, error) {
	sshCert, err := k.SSHCert()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ident, err := sshca.DecodeIdentity(sshCert)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return ident.ActiveRequests, nil
}

// CheckCert makes sure the key's SSH certificate is valid.
func (k *Key) CheckCert() error {
	cert, err := k.SSHCert()
	if err != nil {
		return trace.Wrap(err)
	}

	if err := k.checkCert(cert); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// checkCert makes sure the given SSH certificate is valid.
func (k *Key) checkCert(sshCert *ssh.Certificate) error {
	// Check that the certificate was for the current public key. If not, the
	// public/private key pair may have been rotated.
	if !sshutils.KeysEqual(sshCert.Key, k.SSHPublicKey()) {
		return trace.CompareFailed("public key in profile does not match the public key in SSH certificate")
	}

	// A valid principal is always passed in because the principals are not being
	// checked here, but rather the validity period, signature, and algorithms.
	certChecker := sshutils.CertChecker{
		FIPS: isFIPS(),
	}
	if len(sshCert.ValidPrincipals) == 0 {
		return trace.BadParameter("cert is not valid for any principles")
	}
	if err := certChecker.CheckCert(sshCert.ValidPrincipals[0], sshCert); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// RootClusterName extracts the root cluster name from the issuer
// of the Teleport TLS certificate.
func (k *Key) RootClusterName() (string, error) {
	cert, err := k.TeleportTLSCertificate()
	if err != nil {
		return "", trace.Wrap(err)
	}
	clusterName := cert.Issuer.CommonName
	if clusterName == "" {
		return "", trace.NotFound("failed to extract root cluster name from Teleport TLS cert")
	}
	return clusterName, nil
}

// EqualPrivateKey returns whether this key and the given key have the same PrivateKey.
func (k *Key) EqualPrivateKey(other *Key) bool {
	// Compare both private and public key PEM, since hardware keys
	// may not be uniquely identifiable by their private key PEM alone.
	// For example, for PIV keys, the private key PEM only uniquely
	// identifies a PIV slot, so we can use the public key to verify
	// that the private key on the slot hasn't changed.
	return subtle.ConstantTimeCompare(k.PrivateKeyPEM(), other.PrivateKeyPEM()) == 1 &&
		bytes.Equal(k.MarshalSSHPublicKey(), other.MarshalSSHPublicKey())
}
