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
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"

	"github.com/gravitational/teleport/api/constants"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/api/utils/keys/hardwarekey"
	"github.com/gravitational/teleport/api/utils/sshutils"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/sshca"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

// KeyRingIndex identifies a KeyRing in the store.
type KeyRingIndex struct {
	// ProxyHost is the root proxy hostname that a key is associated with.
	ProxyHost string
	// Username is the username that a key is associated with.
	Username string
	// ClusterName is the cluster name that a key is associated with.
	ClusterName string
}

// Check verifies the KeyRingIndex is fully specified.
func (idx KeyRingIndex) Check() error {
	missingField := "keyring index field %s is not set"
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

// Match compares this KeyRingIndex to the given matchKeyRing index.
// It will be considered a match if all non-zero elements of
// the matchKeyRing are matched by this KeyRingIndex.
func (idx KeyRingIndex) Match(matchKeyRing KeyRingIndex) bool {
	return (matchKeyRing.ProxyHost == "" || matchKeyRing.ProxyHost == idx.ProxyHost) &&
		(matchKeyRing.ClusterName == "" || matchKeyRing.ClusterName == idx.ClusterName) &&
		(matchKeyRing.Username == "" || matchKeyRing.Username == idx.Username)
}

func (idx KeyRingIndex) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("proxy", idx.ProxyHost),
		slog.String("cluster", idx.ClusterName),
		slog.String("username", idx.Username),
	)
}

func (idx KeyRingIndex) contextualKeyInfo() hardwarekey.ContextualKeyInfo {
	return hardwarekey.ContextualKeyInfo{
		ProxyHost:   idx.ProxyHost,
		Username:    idx.Username,
		ClusterName: idx.ClusterName,
	}
}

// TLSCredential holds a signed TLS certificate and matching private key.
type TLSCredential struct {
	// PrivateKey is the private key of the credential.
	PrivateKey *keys.PrivateKey
	// Cert is a PEM-encoded signed X509 certificate.
	Cert []byte
}

// TLSCertificate returns a valid [tls.Certificate] ready to be used in a TLS
// handshake.
func (c *TLSCredential) TLSCertificate() (tls.Certificate, error) {
	cert, err := c.PrivateKey.TLSCertificate(c.Cert)
	return cert, trace.Wrap(err)
}

// KeyRing describes a set of client keys and certificates for a specific cluster.
type KeyRing struct {
	KeyRingIndex

	// SSHPrivateKey is a private key used for SSH authentication.
	SSHPrivateKey *keys.PrivateKey
	// Cert is an SSH client certificate.
	Cert []byte

	// TLSPrivateKey is a private key used for TLS authentication.
	TLSPrivateKey *keys.PrivateKey
	// TLSCert is a PEM encoded client TLS x509 certificate.
	// It's used to authenticate to the Teleport APIs.
	TLSCert []byte

	// KubeTLSCredentials are TLS credentials for individual kubernetes clusters.
	// Map key is a kubernetes cluster name.
	KubeTLSCredentials map[string]TLSCredential
	// DBTLSCredentials are TLS credentials for database access.
	// Map key is the database service name.
	DBTLSCredentials map[string]TLSCredential
	// AppTLSCredentials are TLS credentials for application access.
	// Map key is the application name.
	AppTLSCredentials map[string]TLSCredential
	// WindowsDesktopTLSCredentials are TLS credentials for desktop access.
	// Map key is the desktop name.
	WindowsDesktopTLSCredentials map[string]TLSCredential
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

// generateSubjectTLSKey returns a new private key with the appropriate algorithm for
// [purpose], meant to be used as the subject key for a new user cert request.
// If [k.PrivateKey] is a PIV/hardware key or an RSA key, it will be re-used.
func (k *KeyRing) generateSubjectTLSKey(ctx context.Context, tc *TeleportClient, purpose cryptosuites.KeyPurpose) (*keys.PrivateKey, error) {
	if k.TLSPrivateKey.IsHardware() {
		// We always re-use the root TLS key if it is a hardware key.
		return k.TLSPrivateKey, nil
	}
	if _, isRSA := k.TLSPrivateKey.Public().(*rsa.PublicKey); isRSA {
		// We always re-use the root TLS key if it is RSA (it would be expensive
		// to always generate new RSA keys). If [k.PrivateKey] is RSA we must be
		// using the `legacy` signature algorithm suitei and the subject keys
		// should be RSA as well.
		return k.TLSPrivateKey, nil
	}

	key, err := cryptosuites.GenerateKey(ctx, tc.GetCurrentSignatureAlgorithmSuite, purpose)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	priv, err := keys.NewPrivateKey(key)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return priv, nil
}

// NewKeyRing creates a new KeyRing for the given private keys.
func NewKeyRing(sshPriv, tlsPriv *keys.PrivateKey) *KeyRing {
	return &KeyRing{
		SSHPrivateKey:                sshPriv,
		TLSPrivateKey:                tlsPriv,
		KubeTLSCredentials:           make(map[string]TLSCredential),
		DBTLSCredentials:             make(map[string]TLSCredential),
		AppTLSCredentials:            make(map[string]TLSCredential),
		WindowsDesktopTLSCredentials: make(map[string]TLSCredential),
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

// TLSCAs returns all TLS CA certificates from this KeyRing.
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
	cred, ok := k.KubeTLSCredentials[kubeClusterName]
	if !ok {
		return nil, trace.NotFound("TLS certificate for kubernetes cluster %q not found", kubeClusterName)
	}

	tlsConfig, err := k.clientTLSConfig(cipherSuites, cred, []string{rootCluster})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tlsConfig.ServerName = fmt.Sprintf("%s%s", constants.KubeTeleportProxyALPNPrefix, constants.APIDomain)
	return tlsConfig, nil
}

// HostKeyCallback returns an ssh.HostKeyCallback that validates host
// keys/certs against SSH CAs in the KeyRing.
//
// If not CAs are present in the KeyRing, the returned ssh.HostKeyCallback is nil.
// This causes golang.org/x/crypto/ssh to prompt the user to verify host key
// fingerprint (same as OpenSSH does for an unknown host).
func (k *KeyRing) HostKeyCallback(hostnames ...string) (ssh.HostKeyCallback, error) {
	trustedHostKeys, err := k.authorizedHostKeys(hostnames...)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return sshutils.HostKeyCallback(trustedHostKeys, true)
}

// authorizedHostKeys returns all authorized host keys from this KeyRing. If any host
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
	return k.clientTLSConfig(cipherSuites, TLSCredential{
		PrivateKey: k.TLSPrivateKey,
		Cert:       k.TLSCert,
	}, clusters)
}

func (k *KeyRing) clientTLSConfig(cipherSuites []uint16, cred TLSCredential, clusters []string) (*tls.Config, error) {
	tlsCert, err := cred.TLSCertificate()
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
	certPoolPEM, err := k.clientCertPoolPEM(clusters...)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	pool := x509.NewCertPool()
	if len(certPoolPEM) == 0 {
		// It's valid to have no matching CAs and therefore an empty cert pool.
		return pool, nil
	}
	if !pool.AppendCertsFromPEM(certPoolPEM) {
		return nil, trace.BadParameter("failed to parse TLS CA certificate")
	}
	return pool, nil
}

func (k *KeyRing) clientCertPoolPEM(clusters ...string) ([]byte, error) {
	var certPoolPEM bytes.Buffer
	for _, caPEM := range k.TLSCAs() {
		cert, err := tlsca.ParseCertificatePEM(caPEM)
		if err != nil {
			return nil, trace.Wrap(err, "parsing TLS CA certificate")
		}
		if !slices.Contains(clusters, cert.Subject.CommonName) {
			continue
		}
		certPoolPEM.Write(caPEM)
		// PEM files should end with a trailing newline, just double check
		// before potentially concatenating multiple together.
		if caPEM[len(caPEM)-1] != '\n' {
			certPoolPEM.WriteByte('\n')
		}
	}
	return certPoolPEM.Bytes(), nil
}

// ProxyClientSSHConfig returns an ssh.ClientConfig with SSH credentials from this
// KeyRing and HostKeyCallback matching SSH CAs in the KeyRing.
//
// The config is set up to authenticate to proxy with the first available principal
// and ( if keyStore != nil ) trust local SSH CAs without asking for public keys.
func (k *KeyRing) ProxyClientSSHConfig(hostname string) (*ssh.ClientConfig, error) {
	sshCert, err := k.SSHCert()
	if err != nil {
		return nil, trace.Wrap(err, "failed to extract username from SSH certificate")
	}

	sshConfig, err := sshutils.ProxyClientSSHConfig(sshCert, k.SSHPrivateKey.Signer)
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
func teleportAgentKeyComment(k KeyRingIndex) string {
	return strings.Join([]string{
		agentKeyCommentPrefix,
		k.ProxyHost,
		k.ClusterName,
		k.Username,
	}, agentKeyCommentSeparator)
}

// parseTeleportAgentKeyComment parses an agent key comment into
// its associated KeyRingIndex.
func parseTeleportAgentKeyComment(comment string) (KeyRingIndex, bool) {
	parts := strings.Split(comment, agentKeyCommentSeparator)
	if len(parts) != 4 || parts[0] != agentKeyCommentPrefix {
		return KeyRingIndex{}, false
	}

	return KeyRingIndex{
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

// AsAgentKey converts client.KeyRing struct to an agent.AddedKey. Any agent.AddedKey
// can be added to a local agent (keyring), but non-standard keys cannot be added
// to an SSH system agent through the ssh agent protocol. Check canAddToSystemAgent
// before adding this key to an SSH system agent.
func (k *KeyRing) AsAgentKey() (agent.AddedKey, error) {
	sshCert, err := k.SSHCert()
	if err != nil {
		return agent.AddedKey{}, trace.Wrap(err)
	}

	return agent.AddedKey{
		PrivateKey:       k.SSHPrivateKey.Signer,
		Certificate:      sshCert,
		Comment:          teleportAgentKeyComment(k.KeyRingIndex),
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
	cred, ok := k.KubeTLSCredentials[kubeClusterName]
	if !ok {
		return nil, trace.NotFound("TLS credential for kubernetes cluster %q not found", kubeClusterName)
	}
	return tlsca.ParseCertificatePEM(cred.Cert)
}

// KubeTLSCert returns the tls.Certificate for authentication against a named
// kubernetes cluster.
func (k *KeyRing) KubeTLSCert(kubeClusterName string) (tls.Certificate, error) {
	cred, ok := k.KubeTLSCredentials[kubeClusterName]
	if !ok {
		return tls.Certificate{}, trace.NotFound("TLS certificate for kubernetes cluster %q not found", kubeClusterName)
	}
	tlsCert, err := cred.TLSCertificate()
	if err != nil {
		return tls.Certificate{}, trace.Wrap(err)
	}
	return tlsCert, nil
}

// DBTLSCert returns the tls.Certificate for authentication against a named database.
func (k *KeyRing) DBTLSCert(dbName string) (tls.Certificate, error) {
	cred, ok := k.DBTLSCredentials[dbName]
	if !ok {
		return tls.Certificate{}, trace.NotFound("TLS certificate for database %q not found", dbName)
	}
	return cred.TLSCertificate()
}

// DBTLSCertificates returns all parsed x509 database access certificates.
func (k *KeyRing) DBTLSCertificates() (certs []x509.Certificate, err error) {
	for _, cred := range k.DBTLSCredentials {
		cert, err := tlsca.ParseCertificatePEM(cred.Cert)
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

// WindowsDesktopTLSCert returns the tls.Certificate for authentication against a named desktop.
func (k *KeyRing) WindowsDesktopTLSCert(desktopName string) (tls.Certificate, error) {
	cred, ok := k.WindowsDesktopTLSCredentials[desktopName]
	if !ok {
		return tls.Certificate{}, trace.NotFound("TLS certificate for Windows desktop %q not found", desktopName)
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
	return sshutils.AsAuthMethod(cert, k.SSHPrivateKey)
}

// SSHSigner returns an ssh.Signer using the SSH certificate in this key.
func (k *KeyRing) SSHSigner() (ssh.Signer, error) {
	cert, err := k.SSHCert()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return sshutils.SSHSigner(cert, k.SSHPrivateKey)
}

// SSHCert returns parsed SSH certificate
func (k *KeyRing) SSHCert() (*ssh.Certificate, error) {
	if k.Cert == nil {
		return nil, trace.NotFound("SSH cert not found")
	}
	return sshutils.ParseCertificate(k.Cert)
}

// ActiveRequests gets the active requests associated with this key.
func (k *KeyRing) ActiveRequests() ([]string, error) {
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
	if !sshutils.KeysEqual(sshCert.Key, k.SSHPrivateKey.SSHPublicKey()) {
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
	return bytes.Equal(k.SSHPrivateKey.MarshalSSHPublicKey(), other.SSHPrivateKey.MarshalSSHPublicKey()) &&
		bytes.Equal(k.TLSPrivateKey.MarshalSSHPublicKey(), other.TLSPrivateKey.MarshalSSHPublicKey()) &&
		subtle.ConstantTimeCompare(k.SSHPrivateKey.PrivateKeyPEM(), other.SSHPrivateKey.PrivateKeyPEM()) == 1 &&
		subtle.ConstantTimeCompare(k.TLSPrivateKey.PrivateKeyPEM(), other.TLSPrivateKey.PrivateKeyPEM()) == 1
}
