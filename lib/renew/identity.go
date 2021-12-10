package renew

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"strings"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	apiutils "github.com/gravitational/teleport/api/utils"
	apisshutils "github.com/gravitational/teleport/api/utils/sshutils"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"
)

// Identity is collection of certificates and signers that represent server identity
type Identity struct {
	// KeyBytes is a PEM encoded private key
	KeyBytes []byte
	// SSHPublicKeyBytes contains bytes of the original SSH public key
	SSHPublicKeyBytes []byte
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
	// Cert is a parsed SSH certificate
	Cert *ssh.Certificate
	// XCert is X509 client certificate
	XCert *x509.Certificate
	// ClusterName is a name of host's cluster
	ClusterName string
}

// String returns user-friendly representation of the identity.
func (i *Identity) String() string {
	var out []string
	if i.XCert != nil {
		out = append(out, fmt.Sprintf("cert(%v issued by %v:%v)", i.XCert.Subject.CommonName, i.XCert.Issuer.CommonName, i.XCert.Issuer.SerialNumber))
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
	set := utils.StringsSet(i.Cert.ValidPrincipals)
	for _, principal := range additionalPrincipals {
		if _, ok := set[principal]; !ok {
			return false
		}
	}
	return true
}

// HasDNSNames returns true if TLS certificate has required DNS names
func (i *Identity) HasDNSNames(dnsNames []string) bool {
	if i.XCert == nil {
		return false
	}
	set := utils.StringsSet(i.XCert.DNSNames)
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
	tlsCert, err := tls.X509KeyPair(i.TLSCertBytes, i.KeyBytes)
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

// ReadIdentityFromKeyPair reads SSH and TLS identity from key pair.
func ReadIdentityFromKeyPair(privateKey []byte, publicKey []byte, certs *proto.Certs) (*Identity, error) {
	identity, err := ReadSSHIdentityFromKeyPair(privateKey, publicKey, certs.SSH)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if len(certs.SSHCACerts) != 0 {
		identity.SSHCACertBytes = certs.SSHCACerts
	}

	if len(certs.TLSCACerts) != 0 {
		// Parse the key pair to verify that identity parses properly for future use.
		i, err := ReadTLSIdentityFromKeyPair(privateKey, certs.TLS, certs.TLSCACerts)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		identity.XCert = i.XCert
		identity.TLSCertBytes = certs.TLS
		identity.TLSCACertsBytes = certs.TLSCACerts
	}

	return identity, nil
}

// ReadTLSIdentityFromKeyPair reads TLS identity from key pair
func ReadTLSIdentityFromKeyPair(keyBytes, certBytes []byte, caCertsBytes [][]byte) (*Identity, error) {
	if len(keyBytes) == 0 {
		return nil, trace.BadParameter("missing private key")
	}

	if len(certBytes) == 0 {
		return nil, trace.BadParameter("missing certificate")
	}

	cert, err := tlsca.ParseCertificatePEM(certBytes)
	if err != nil {
		return nil, trace.Wrap(err, "failed to parse TLS certificate")
	}

	if len(cert.Issuer.Organization) == 0 {
		return nil, trace.BadParameter("missing CA organization")
	}

	clusterName := cert.Issuer.Organization[0]
	if clusterName == "" {
		return nil, trace.BadParameter("misssing cluster name")
	}
	identity := &Identity{
		ClusterName:     clusterName,
		KeyBytes:        keyBytes,
		TLSCertBytes:    certBytes,
		TLSCACertsBytes: caCertsBytes,
		XCert:           cert,
	}
	// The passed in ciphersuites don't appear to matter here since the returned
	// *tls.Config is never actually used?
	_, err = identity.TLSConfig(utils.DefaultCipherSuites())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return identity, nil
}

// ReadSSHIdentityFromKeyPair reads identity from initialized keypair
func ReadSSHIdentityFromKeyPair(keyBytes, publicKeyBytes, certBytes []byte) (*Identity, error) {
	if len(keyBytes) == 0 {
		return nil, trace.BadParameter("PrivateKey: missing private key")
	}

	if len(publicKeyBytes) == 0 {
		return nil, trace.BadParameter("PublicKey: missing public key")
	}

	if len(certBytes) == 0 {
		return nil, trace.BadParameter("Cert: missing parameter")
	}

	cert, err := apisshutils.ParseCertificate(certBytes)
	if err != nil {
		return nil, trace.BadParameter("failed to parse server certificate: %v", err)
	}

	signer, err := ssh.ParsePrivateKey(keyBytes)
	if err != nil {
		return nil, trace.BadParameter("failed to parse private key: %v", err)
	}
	// this signer authenticates using certificate signed by the cert authority
	// not only by the public key
	certSigner, err := ssh.NewCertSigner(cert, signer)
	if err != nil {
		return nil, trace.BadParameter("unsupported private key: %v", err)
	}

	// check principals on certificate
	if len(cert.ValidPrincipals) < 1 {
		return nil, trace.BadParameter("valid principals: at least one valid principal is required")
	}
	for _, validPrincipal := range cert.ValidPrincipals {
		if validPrincipal == "" {
			return nil, trace.BadParameter("valid principal can not be empty: %q", cert.ValidPrincipals)
		}
	}

	// check permissions on certificate
	// TODO: do we care to even verify this at all?
	// if len(cert.Permissions.Extensions) == 0 {
	// 	return nil, trace.BadParameter("extensions: misssing needed extensions for host roles")
	// }

	// roleString := cert.Permissions.Extensions[utils.CertExtensionRole]
	// if roleString == "" {
	// 	return nil, trace.BadParameter("misssing cert extension %v", utils.CertExtensionRole)
	// }
	// roles, err := types.ParseTeleportRoles(roleString)
	// if err != nil {
	// 	return nil, trace.Wrap(err)
	// }
	// foundRoles := len(roles)
	// if foundRoles != 1 {
	// 	return nil, trace.Errorf("expected one role per certificate. found %d: '%s'",
	// 		foundRoles, roles.String())
	// }

	// TODO: host certs use CertExtensionAuthority, client certs use CertExtensionTeleportRouteToCluster
	// (or at least, teleport-route-to-cluster _appears_ to be a sane cluster name, but I have no idea
	// how it is actually set.)
	//clusterName := cert.Permissions.Extensions[utils.CertExtensionAuthority]
	clusterName := cert.Permissions.Extensions[teleport.CertExtensionTeleportRouteToCluster]
	if clusterName == "" {
		return nil, trace.BadParameter("missing cert extension %v", utils.CertExtensionAuthority)
	}

	return &Identity{
		ClusterName:       clusterName,
		KeyBytes:          keyBytes,
		SSHPublicKeyBytes: publicKeyBytes,
		CertBytes:         certBytes,
		KeySigner:         certSigner,
		Cert:              cert,
	}, nil
}
