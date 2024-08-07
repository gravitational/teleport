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
	"context"
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

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/constants"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/api/utils/sshutils"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/auth/native"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

// KeyIndex identifies a KeyRing in the store.
// TODO(nklaassen): rename to KeyRingIndex because it identifies an entire
// KeyRing in a KeyStore.
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

// TLSCredential holds a signed TLS certificate and matching private key.
type TLSCredential struct {
	PrivateKey *keys.PrivateKey
	Cert       []byte
}

// TLSCertificate returns a valid [tls.Certificate] ready to be used in a TLS
// handshake.
func (c *TLSCredential) TLSCertificate() (tls.Certificate, error) {
	cert, err := c.PrivateKey.TLSCertificate(c.Cert)
	return cert, trace.Wrap(err)
}

// KeyRing describes a set of client keys and certificates for a specific cluster.
type KeyRing struct {
	KeyIndex

	// PrivateKey used to represent the single cryptographic key associated with all
	// certificates in the KeyRing. This is in the process of being deprecated
	// and replaced with unique keys for each certificate, as part of the
	// implementation of RFD 136.
	PrivateKey *keys.PrivateKey

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
	// AppTLSCredetials are TLS credentials for application access.
	// Map key is the application name.
	AppTLSCredentials map[string]TLSCredential
	// WindowsDesktopCerts are TLS certificates for Windows Desktop access.
	// Map key is the desktop server name.
	WindowsDesktopCerts map[string][]byte `json:"WindowsDesktopCerts,omitempty"`
	// TrustedCerts is a list of trusted certificate authorities
	TrustedCerts []authclient.TrustedCerts
}

// Copy returns a shallow copy of k, or nil if k is nil.
func (k *KeyRing) Copy() *KeyRing {
	if k == nil {
		return nil
	}
	copy := *k
	return &copy
}

// GenerateKey returns a new private key with the appropriate algorithm for
// [purpose]. If this KeyRing uses a PIV/hardware key, it will always return
// that hardware key.
func (k *KeyRing) GenerateKey(ctx context.Context, tc *TeleportClient, purpose cryptosuites.KeyPurpose) (*keys.PrivateKey, error) {
	if k.PrivateKey.IsHardware() {
		// We always use the same hardware key.
		return k.PrivateKey, nil
	}

	// Ping is cached if called more than once on [tc].
	pingResp, err := tc.Ping(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	key, err := cryptosuites.GenerateKeyWithSuite(ctx, pingResp.Auth.SignatureAlgorithmSuite, purpose)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	privateKeyPEM, err := keys.MarshalPrivateKey(key)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	priv, err := keys.NewPrivateKey(key, privateKeyPEM)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return priv, nil
}

// GenerateRSAKey generates a new unsigned key.
func GenerateRSAKey() (*KeyRing, error) {
	priv, err := native.GeneratePrivateKey()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return NewKeyRing(priv), nil
}

// NewKeyRing creates a new KeyRing for the given private key.
func NewKeyRing(priv *keys.PrivateKey) *KeyRing {
	return &KeyRing{
		PrivateKey:          priv,
		KubeTLSCerts:        make(map[string][]byte),
		DBTLSCerts:          make(map[string][]byte),
		AppTLSCredentials:   make(map[string]TLSCredential),
		WindowsDesktopCerts: make(map[string][]byte),
	}
}

// RootClusterCAs returns root cluster CAs.
func (k *KeyRing) RootClusterCAs() ([][]byte, error) {
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
func (k *KeyRing) TLSCAs() (result [][]byte) {
	for _, ca := range k.TrustedCerts {
		result = append(result, ca.TLSCertificates...)
	}
	return result
}

func (k *KeyRing) KubeClientTLSConfig(cipherSuites []uint16, kubeClusterName string) (*tls.Config, error) {
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
func (k *KeyRing) HostKeyCallback(hostnames ...string) (ssh.HostKeyCallback, error) {
	trustedHostKeys, err := k.authorizedHostKeys(hostnames...)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return sshutils.HostKeyCallback(trustedHostKeys, true)
}

// authorizedHostKeys returns all authorized host keys from this key. If any host
// names are provided, only matching host keys will be returned.
func (k *KeyRing) authorizedHostKeys(hostnames ...string) ([]ssh.PublicKey, error) {
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
func (k *KeyRing) TeleportClientTLSConfig(cipherSuites []uint16, clusters []string) (*tls.Config, error) {
	if len(k.TLSCert) == 0 {
		return nil, trace.NotFound("TLS certificate not found")
	}
	return k.clientTLSConfig(cipherSuites, k.TLSCert, clusters)
}

func (k *KeyRing) clientTLSConfig(cipherSuites []uint16, tlsCertRaw []byte, clusters []string) (*tls.Config, error) {
	tlsCert, err := k.PrivateKey.TLSCertificate(tlsCertRaw)
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
func (k *KeyRing) clientCertPool(clusters ...string) (*x509.CertPool, error) {
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
func (k *KeyRing) ProxyClientSSHConfig(hostname string) (*ssh.ClientConfig, error) {
	sshCert, err := k.SSHCert()
	if err != nil {
		return nil, trace.Wrap(err, "failed to extract username from SSH certificate")
	}

	sshConfig, err := sshutils.ProxyClientSSHConfig(sshCert, k.PrivateKey.Signer)
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
func (k *KeyRing) CertUsername() (string, error) {
	cert, err := k.SSHCert()
	if err != nil {
		return "", trace.Wrap(err)
	}
	return cert.KeyId, nil
}

// CertPrincipals returns the principals listed on the SSH certificate.
func (k *KeyRing) CertPrincipals() ([]string, error) {
	cert, err := k.SSHCert()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return cert.ValidPrincipals, nil
}

func (k *KeyRing) CertRoles() ([]string, error) {
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
func (k *KeyRing) AsAgentKey() (agent.AddedKey, error) {
	sshCert, err := k.SSHCert()
	if err != nil {
		return agent.AddedKey{}, trace.Wrap(err)
	}

	return agent.AddedKey{
		PrivateKey:       k.PrivateKey.Signer,
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
func (k *KeyRing) TeleportTLSCertificate() (*x509.Certificate, error) {
	if len(k.TLSCert) == 0 {
		return nil, trace.NotFound("TLS certificate not found")
	}
	return tlsca.ParseCertificatePEM(k.TLSCert)
}

// KubeX509Cert returns the parsed x509 certificate for authentication against
// a named kubernetes cluster.
func (k *KeyRing) KubeX509Cert(kubeClusterName string) (*x509.Certificate, error) {
	tlsCert, ok := k.KubeTLSCerts[kubeClusterName]
	if !ok {
		return nil, trace.NotFound("TLS certificate for kubernetes cluster %q not found", kubeClusterName)
	}
	return tlsca.ParseCertificatePEM(tlsCert)
}

// KubeTLSCert returns the tls.Certificate for authentication against a named
// kubernetes cluster.
func (k *KeyRing) KubeTLSCert(kubeClusterName string) (tls.Certificate, error) {
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

// DBTLSCert returns the tls.Certificate for authentication against a named database.
func (k *KeyRing) DBTLSCert(dbName string) (tls.Certificate, error) {
	certPem, ok := k.DBTLSCerts[dbName]
	if !ok {
		return tls.Certificate{}, trace.NotFound("TLS certificate for database %q not found", dbName)
	}
	tlsCert, err := k.PrivateKey.TLSCertificate(certPem)
	if err != nil {
		return tls.Certificate{}, trace.Wrap(err)
	}
	return tlsCert, nil
}

// DBTLSCertificates returns all parsed x509 database access certificates.
func (k *KeyRing) DBTLSCertificates() (certs []x509.Certificate, err error) {
	for _, bytes := range k.DBTLSCerts {
		cert, err := tlsca.ParseCertificatePEM(bytes)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		certs = append(certs, *cert)
	}
	return certs, nil
}

// AppTLSCert returns the tls.Certificate for authentication against a named app.
func (k *KeyRing) AppTLSCert(appName string) (tls.Certificate, error) {
	cred, ok := k.AppTLSCredentials[appName]
	if !ok {
		return tls.Certificate{}, trace.NotFound("TLS certificate for application %q not found", appName)
	}
	return cred.TLSCertificate()
}

// AppTLSCertificates returns all parsed x509 app access certificates.
func (k *KeyRing) AppTLSCertificates() (certs []x509.Certificate, err error) {
	for _, cred := range k.AppTLSCredentials {
		cert, err := tlsca.ParseCertificatePEM(cred.Cert)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		certs = append(certs, *cert)
	}
	return certs, nil
}

// TeleportTLSCertValidBefore returns the time of the TLS cert expiration
func (k *KeyRing) TeleportTLSCertValidBefore() (t time.Time, err error) {
	cert, err := k.TeleportTLSCertificate()
	if err != nil {
		return t, trace.Wrap(err)
	}
	return cert.NotAfter, nil
}

// CertValidBefore returns the time of the cert expiration
func (k *KeyRing) CertValidBefore() (t time.Time, err error) {
	cert, err := k.SSHCert()
	if err != nil {
		return t, trace.Wrap(err)
	}
	return time.Unix(int64(cert.ValidBefore), 0), nil
}

// AsAuthMethod returns an "auth method" interface, a common abstraction
// used by Golang SSH library. This is how you actually use a Key to feed
// it into the SSH lib.
func (k *KeyRing) AsAuthMethod() (ssh.AuthMethod, error) {
	cert, err := k.SSHCert()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return sshutils.AsAuthMethod(cert, k.PrivateKey)
}

// SSHSigner returns an ssh.Signer using the SSH certificate in this key.
func (k *KeyRing) SSHSigner() (ssh.Signer, error) {
	cert, err := k.SSHCert()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return sshutils.SSHSigner(cert, k.PrivateKey)
}

// SSHCert returns parsed SSH certificate
func (k *KeyRing) SSHCert() (*ssh.Certificate, error) {
	if k.Cert == nil {
		return nil, trace.NotFound("SSH cert not found")
	}
	return sshutils.ParseCertificate(k.Cert)
}

// ActiveRequests gets the active requests associated with this key.
func (k *KeyRing) ActiveRequests() (services.RequestIDs, error) {
	var activeRequests services.RequestIDs
	sshCert, err := k.SSHCert()
	if err != nil {
		return activeRequests, trace.Wrap(err)
	}
	rawRequests, ok := sshCert.Extensions[teleport.CertExtensionTeleportActiveRequests]
	if ok {
		if err := activeRequests.Unmarshal([]byte(rawRequests)); err != nil {
			return activeRequests, trace.Wrap(err)
		}
	}
	return activeRequests, nil
}

// CheckCert makes sure the key's SSH certificate is valid.
func (k *KeyRing) CheckCert() error {
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
func (k *KeyRing) checkCert(sshCert *ssh.Certificate) error {
	// Check that the certificate was for the current public key. If not, the
	// public/private key pair may have been rotated.
	if !sshutils.KeysEqual(sshCert.Key, k.PrivateKey.SSHPublicKey()) {
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
func (k *KeyRing) RootClusterName() (string, error) {
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
func (k *KeyRing) EqualPrivateKey(other *KeyRing) bool {
	// Compare both private and public key PEM, since hardware keys
	// may not be uniquely identifiable by their private key PEM alone.
	// For example, for PIV keys, the private key PEM only uniquely
	// identifies a PIV slot, so we can use the public key to verify
	// that the private key on the slot hasn't changed.
	return subtle.ConstantTimeCompare(k.PrivateKey.PrivateKeyPEM(), other.PrivateKey.PrivateKeyPEM()) == 1 &&
		bytes.Equal(k.PrivateKey.MarshalSSHPublicKey(), other.PrivateKey.MarshalSSHPublicKey())
}
