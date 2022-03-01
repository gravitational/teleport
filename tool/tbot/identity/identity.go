/*
Copyright 2021-2022 Gravitational, Inc.

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

package identity

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"strings"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	apiutils "github.com/gravitational/teleport/api/utils"
	apisshutils "github.com/gravitational/teleport/api/utils/sshutils"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/tool/tbot/destination"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
)

const (
	// TLSCertKey is the name under which TLS certificates exist in a destination.
	TLSCertKey = "tlscert"

	// TLSCertKey is the name under which SSH certificates exist in a destination.
	SSHCertKey = "sshcert"

	// SSHCACertsKey is the name under which SSH CA certificates exist in a destination.
	SSHCACertsKey = "sshcacerts"

	// TLSCACertsKey is the name under which SSH CA certificates exist in a destination.
	TLSCACertsKey = "tlscacerts"

	// PrivateKeyKey is the name under which the private key exists in a destination.
	// The same private key is used for SSH and TLS certificates.
	PrivateKeyKey = "key"

	// PublicKeyKey is the ssh public key, required for successful SSH connections.
	PublicKeyKey = "key.pub"

	// TokenHashKey is the storage where a hash of the onboarding token will be stored.
	TokenHashKey = "tokenhash"

	// WriteTestKey is the key for a file used to check that the destination is
	// writable.
	WriteTestKey = ".write-test"
)

var log = logrus.WithFields(logrus.Fields{
	trace.Component: teleport.ComponentTBot,
})

// Identity is collection of certificates and signers that represent server
// identity. This is derived from Teleport's usual auth.Identity with small
// modifications to work with user rather than host certificates.
type Identity struct {
	// PrivateKeyBytes is a PEM encoded private key
	PrivateKeyBytes []byte
	// PublicKeyBytes contains bytes of the original SSH public key
	PublicKeyBytes []byte
	// CertBytes is a PEM encoded SSH host cert
	CertBytes []byte
	// TLSCertBytes is a PEM encoded TLS x509 client certificate
	TLSCertBytes []byte
	// TLSCACertBytes is a list of PEM encoded TLS x509 certificate of certificate authority
	// associated with auth server services
	TLSCACertsBytes [][]byte
	// SSHCACertBytes is a list of SSH CAs encoded in the authorized_keys format.
	SSHCACertBytes [][]byte
	// KeySigner is an SSH host certificate signer
	KeySigner ssh.Signer
	// SSHCert is a parsed SSH certificate
	SSHCert *ssh.Certificate
	// X509Cert is an X509 client certificate
	X509Cert *x509.Certificate
	// ClusterName is a name of host's cluster
	ClusterName string
	// TokenHashBytes is the hash of the original join token
	TokenHashBytes []byte
}

// LoadIdentityParams contains parameters beyond proto.Certs needed to load a
// stored identity.
type LoadIdentityParams struct {
	PrivateKeyBytes []byte
	PublicKeyBytes  []byte
	TokenHashBytes  []byte
}

// Params returns the LoadIdentityParams for this Identity, which are the
// local-only parameters to be carried over to a renewed identity.
func (i *Identity) Params() *LoadIdentityParams {
	return &LoadIdentityParams{
		PrivateKeyBytes: i.PrivateKeyBytes,
		PublicKeyBytes:  i.PublicKeyBytes,
		TokenHashBytes:  i.TokenHashBytes,
	}
}

// String returns user-friendly representation of the identity.
func (i *Identity) String() string {
	var out []string
	if i.X509Cert != nil {
		out = append(out, fmt.Sprintf("cert(%v issued by %v:%v)", i.X509Cert.Subject.CommonName, i.X509Cert.Issuer.CommonName, i.X509Cert.Issuer.SerialNumber))
	}
	for j := range i.TLSCACertsBytes {
		cert, err := tlsca.ParseCertificatePEM(i.TLSCACertsBytes[j])
		if err != nil {
			out = append(out, err.Error())
		} else {
			out = append(out, fmt.Sprintf("trust root(%v:%v)", cert.Subject.CommonName, cert.Subject.SerialNumber))
		}
	}
	return fmt.Sprintf("Identity(%v)", strings.Join(out, ","))
}

// CertInfo returns diagnostic information about certificate
func CertInfo(cert *x509.Certificate) string {
	return fmt.Sprintf("cert(%v issued by %v:%v)", cert.Subject.CommonName, cert.Issuer.CommonName, cert.Issuer.SerialNumber)
}

// TLSCertInfo returns diagnostic information about certificate
func TLSCertInfo(cert *tls.Certificate) string {
	x509cert, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		return err.Error()
	}
	return CertInfo(x509cert)
}

// CertAuthorityInfo returns debugging information about certificate authority
func CertAuthorityInfo(ca types.CertAuthority) string {
	var out []string
	for _, keyPair := range ca.GetTrustedTLSKeyPairs() {
		cert, err := tlsca.ParseCertificatePEM(keyPair.Cert)
		if err != nil {
			out = append(out, err.Error())
		} else {
			out = append(out, fmt.Sprintf("trust root(%v:%v)", cert.Subject.CommonName, cert.Subject.SerialNumber))
		}
	}
	return fmt.Sprintf("cert authority(state: %v, phase: %v, roots: %v)", ca.GetRotation().State, ca.GetRotation().Phase, strings.Join(out, ", "))
}

// HasTSLConfig returns true if this identity has TLS certificate and private key
func (i *Identity) HasTLSConfig() bool {
	return len(i.TLSCACertsBytes) != 0 && len(i.TLSCertBytes) != 0
}

// HasPrincipals returns whether identity has principals
func (i *Identity) HasPrincipals(additionalPrincipals []string) bool {
	set := utils.StringsSet(i.SSHCert.ValidPrincipals)
	for _, principal := range additionalPrincipals {
		if _, ok := set[principal]; !ok {
			return false
		}
	}
	return true
}

// HasDNSNames returns true if TLS certificate has required DNS names
func (i *Identity) HasDNSNames(dnsNames []string) bool {
	if i.X509Cert == nil {
		return false
	}
	set := utils.StringsSet(i.X509Cert.DNSNames)
	for _, dnsName := range dnsNames {
		if _, ok := set[dnsName]; !ok {
			return false
		}
	}
	return true
}

// TLSConfig returns TLS config for mutual TLS authentication
// can return NotFound error if there are no TLS credentials setup for identity
func (i *Identity) TLSConfig(cipherSuites []uint16) (*tls.Config, error) {
	tlsConfig := utils.TLSConfig(cipherSuites)
	if !i.HasTLSConfig() {
		return nil, trace.NotFound("no TLS credentials setup for this identity")
	}
	tlsCert, err := tls.X509KeyPair(i.TLSCertBytes, i.PrivateKeyBytes)
	if err != nil {
		return nil, trace.BadParameter("failed to parse private key: %v", err)
	}
	certPool := x509.NewCertPool()
	for j := range i.TLSCACertsBytes {
		parsedCert, err := tlsca.ParseCertificatePEM(i.TLSCACertsBytes[j])
		if err != nil {
			return nil, trace.Wrap(err, "failed to parse CA certificate")
		}
		certPool.AddCert(parsedCert)
	}
	tlsConfig.Certificates = []tls.Certificate{tlsCert}
	tlsConfig.RootCAs = certPool
	tlsConfig.ClientCAs = certPool
	tlsConfig.ServerName = apiutils.EncodeClusterName(i.ClusterName)
	return tlsConfig, nil
}

func (i *Identity) getSSHCheckers() ([]ssh.PublicKey, error) {
	checkers, err := apisshutils.ParseAuthorizedKeys(i.SSHCACertBytes)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return checkers, nil
}

// SSHClientConfig returns a ssh.ClientConfig used by the bot to connect to
// the reverse tunnel server.
func (i *Identity) SSHClientConfig() (*ssh.ClientConfig, error) {
	callback, err := apisshutils.NewHostKeyCallback(
		apisshutils.HostKeyCallbackConfig{
			GetHostCheckers: i.getSSHCheckers,
		})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if len(i.SSHCert.ValidPrincipals) < 1 {
		return nil, trace.BadParameter("user cert has no valid principals")
	}
	return &ssh.ClientConfig{
		User:            i.SSHCert.ValidPrincipals[0],
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(i.KeySigner)},
		HostKeyCallback: callback,
		Timeout:         apidefaults.DefaultDialTimeout,
	}, nil
}

// ReadIdentityFromStore reads stored identity credentials
func ReadIdentityFromStore(params *LoadIdentityParams, certs *proto.Certs, kinds ...ArtifactKind) (*Identity, error) {
	var identity Identity
	if ContainsKind(KindSSH, kinds) {
		if len(certs.SSH) == 0 {
			return nil, trace.BadParameter("identity requires SSH certificates but they are unset")
		}

		err := ReadSSHIdentityFromKeyPair(&identity, params.PrivateKeyBytes, params.PrivateKeyBytes, certs.SSH)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		if len(certs.SSHCACerts) != 0 {
			identity.SSHCACertBytes = certs.SSHCACerts
		}
	}

	if ContainsKind(KindTLS, kinds) {
		if len(certs.TLSCACerts) == 0 || len(certs.TLS) == 0 {
			return nil, trace.BadParameter("identity requires TLS certificates but they are empty")
		}

		// Parse the key pair to verify that identity parses properly for future use.
		err := ReadTLSIdentityFromKeyPair(&identity, params.PrivateKeyBytes, certs.TLS, certs.TLSCACerts)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	identity.PublicKeyBytes = params.PublicKeyBytes
	identity.PrivateKeyBytes = params.PrivateKeyBytes
	identity.TokenHashBytes = params.TokenHashBytes

	return &identity, nil
}

// ReadTLSIdentityFromKeyPair reads TLS identity from key pair
func ReadTLSIdentityFromKeyPair(identity *Identity, keyBytes, certBytes []byte, caCertsBytes [][]byte) error {
	if len(keyBytes) == 0 {
		return trace.BadParameter("missing private key")
	}

	if len(certBytes) == 0 {
		return trace.BadParameter("missing certificate")
	}

	cert, err := tlsca.ParseCertificatePEM(certBytes)
	if err != nil {
		return trace.Wrap(err, "failed to parse TLS certificate")
	}

	if len(cert.Issuer.Organization) == 0 {
		return trace.BadParameter("missing CA organization")
	}

	clusterName := cert.Issuer.Organization[0]
	if clusterName == "" {
		return trace.BadParameter("misssing cluster name")
	}

	identity.ClusterName = clusterName
	identity.PrivateKeyBytes = keyBytes
	identity.TLSCertBytes = certBytes
	identity.TLSCACertsBytes = caCertsBytes
	identity.X509Cert = cert

	// The passed in ciphersuites don't appear to matter here since the returned
	// *tls.Config is never actually used?
	_, err = identity.TLSConfig(utils.DefaultCipherSuites())
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// ReadSSHIdentityFromKeyPair reads identity from initialized keypair
func ReadSSHIdentityFromKeyPair(identity *Identity, keyBytes, publicKeyBytes, certBytes []byte) error {
	if len(keyBytes) == 0 {
		return trace.BadParameter("PrivateKey: missing private key")
	}

	if len(publicKeyBytes) == 0 {
		return trace.BadParameter("PublicKey: missing public key")
	}

	if len(certBytes) == 0 {
		return trace.BadParameter("Cert: missing parameter")
	}

	cert, err := apisshutils.ParseCertificate(certBytes)
	if err != nil {
		return trace.BadParameter("failed to parse server certificate: %v", err)
	}

	signer, err := ssh.ParsePrivateKey(keyBytes)
	if err != nil {
		return trace.BadParameter("failed to parse private key: %v", err)
	}
	// this signer authenticates using certificate signed by the cert authority
	// not only by the public key
	certSigner, err := ssh.NewCertSigner(cert, signer)
	if err != nil {
		return trace.BadParameter("unsupported private key: %v", err)
	}

	// check principals on certificate
	if len(cert.ValidPrincipals) < 1 {
		return trace.BadParameter("valid principals: at least one valid principal is required")
	}
	for _, validPrincipal := range cert.ValidPrincipals {
		if validPrincipal == "" {
			return trace.BadParameter("valid principal can not be empty: %q", cert.ValidPrincipals)
		}
	}

	clusterName := cert.Permissions.Extensions[teleport.CertExtensionTeleportRouteToCluster]
	if clusterName == "" {
		return trace.BadParameter("missing cert extension %v", utils.CertExtensionAuthority)
	}

	identity.ClusterName = clusterName
	identity.PrivateKeyBytes = keyBytes
	identity.PublicKeyBytes = publicKeyBytes
	identity.CertBytes = certBytes
	identity.KeySigner = certSigner
	identity.SSHCert = cert

	return nil
}

// VerifyWrite attempts to write to the .write-test artifact inside the given
// destination. It should be called before attempting a renewal to help ensure
// we won't then fail to save the identity.
func VerifyWrite(dest destination.Destination) error {
	return trace.Wrap(dest.Write(WriteTestKey, []byte{}))
}

// SaveIdentity saves a bot identity to a destination.
func SaveIdentity(id *Identity, d destination.Destination, kinds ...ArtifactKind) error {
	for _, artifact := range GetArtifacts() {
		// Only store artifacts matching one of the set kinds.
		if !artifact.Matches(kinds...) {
			continue
		}

		data := artifact.ToBytes(id)

		log.Debugf("Writing %s", artifact.Key)
		if err := d.Write(artifact.Key, data); err != nil {
			return trace.WrapWithMessage(err, "could not write to %v", artifact.Key)
		}
	}

	return nil
}

// LoadIdentity loads a bot identity from a destination.
func LoadIdentity(d destination.Destination, kinds ...ArtifactKind) (*Identity, error) {
	var certs proto.Certs
	var params LoadIdentityParams

	for _, artifact := range GetArtifacts() {
		// Only attempt to load artifacts matching one of the specified kinds
		if !artifact.Matches(kinds...) {
			continue
		}

		data, err := d.Read(artifact.Key)
		if err != nil {
			return nil, trace.WrapWithMessage(err, "could not read artifact %q from destination %s", artifact.Key, d)
		}

		artifact.FromBytes(&certs, &params, data)
	}

	log.Debugf("Loaded %d SSH CA certs and %d TLS CA certs", len(certs.SSHCACerts), len(certs.TLSCACerts))

	return ReadIdentityFromStore(&params, &certs, kinds...)
}
