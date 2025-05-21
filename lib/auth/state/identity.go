// Teleport
// Copyright (C) 2024 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package state

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"slices"
	"strings"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/api/client/proto"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/api/utils/keys"
	apisshutils "github.com/gravitational/teleport/api/utils/sshutils"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

// IdentityID is a combination of role, host UUID, and node name.
type IdentityID struct {
	Role     types.SystemRole
	HostUUID string
	NodeName string
}

// HostID is host ID part of the host UUID that consists cluster name
func (id *IdentityID) HostID() string {
	return strings.SplitN(id.HostUUID, ".", 2)[0]
}

// Equals returns true if two identities are equal
func (id *IdentityID) Equals(other IdentityID) bool {
	return id.Role == other.Role && id.HostUUID == other.HostUUID
}

// String returns debug friendly representation of this identity
func (id *IdentityID) String() string {
	return fmt.Sprintf("Identity(hostuuid=%v, role=%v)", id.HostUUID, id.Role)
}

// Identity is collection of certificates and signers that represent server identity
type Identity struct {
	// ID specifies server unique ID, name and role
	ID IdentityID
	// KeyBytes is a PEM encoded private key
	KeyBytes []byte
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
	// SystemRoles is a list of additional system roles.
	SystemRoles []string
}

// HasSystemRole checks if this identity encompasses the supplied system role.
func (i *Identity) HasSystemRole(role types.SystemRole) bool {
	// check identity's primary system role
	if i.ID.Role == role {
		return true
	}

	return slices.Contains(i.SystemRoles, string(role))
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
	return fmt.Sprintf("Identity(%v, %v)", i.ID.Role, strings.Join(out, ","))
}

// HasTLSConfig returns true if this identity has TLS certificate and private
// key.
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

// HasDNSNames returns true if TLS certificate has required DNS names or IP
// addresses.
func (i *Identity) HasDNSNames(requested []string) bool {
	if i.XCert == nil {
		return false
	}
	for _, dnsName := range requested {
		if slices.Contains(i.XCert.DNSNames, dnsName) {
			continue
		}
		// this matches the check done by the auth as part of
		// (*tlsca.CertAuthority).GenerateCertificate (there's only a list of
		// "dns names" but ip addresses are rendered as IP SANs rather than DNS
		// SANs)
		if ip := net.ParseIP(dnsName); ip != nil && slices.ContainsFunc(i.XCert.IPAddresses, ip.Equal) {
			continue
		}
		return false
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

	tlsCert, err := keys.X509KeyPair(i.TLSCertBytes, i.KeyBytes)
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

// SSHClientConfig returns a ssh.ClientConfig used by nodes to connect to
// the reverse tunnel server.
func (i *Identity) SSHClientConfig(fips bool) (*ssh.ClientConfig, error) {
	callback, err := apisshutils.NewHostKeyCallback(
		apisshutils.HostKeyCallbackConfig{
			GetHostCheckers: i.getSSHCheckers,
			FIPS:            fips,
		})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &ssh.ClientConfig{
		User:            i.ID.HostUUID,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(i.KeySigner)},
		HostKeyCallback: callback,
		Timeout:         apidefaults.DefaultIOTimeout,
	}, nil
}

// ReadIdentityFromKeyPair reads SSH and TLS identity from key pair.
func ReadIdentityFromKeyPair(privateKey []byte, certs *proto.Certs) (*Identity, error) {
	identity, err := ReadSSHIdentityFromKeyPair(privateKey, certs.SSH)
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
		identity.SystemRoles = i.SystemRoles
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

	id, err := tlsca.FromSubject(cert.Subject, cert.NotAfter)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if len(cert.Issuer.Organization) == 0 {
		return nil, trace.BadParameter("missing CA organization")
	}

	clusterName := cert.Issuer.Organization[0]
	if clusterName == "" {
		return nil, trace.BadParameter("missing cluster name")
	}
	identity := &Identity{
		ID:              IdentityID{HostUUID: id.Username, Role: types.SystemRole(id.Groups[0])},
		ClusterName:     clusterName,
		KeyBytes:        keyBytes,
		TLSCertBytes:    certBytes,
		TLSCACertsBytes: caCertsBytes,
		XCert:           cert,
		SystemRoles:     id.SystemRoles,
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
func ReadSSHIdentityFromKeyPair(keyBytes, certBytes []byte) (*Identity, error) {
	if len(keyBytes) == 0 {
		return nil, trace.BadParameter("PrivateKey: missing private key")
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
	if len(cert.Permissions.Extensions) == 0 {
		return nil, trace.BadParameter("extensions: missing needed extensions for host roles")
	}
	roleString := cert.Permissions.Extensions[utils.CertExtensionRole]
	if roleString == "" {
		return nil, trace.BadParameter("missing cert extension %v", utils.CertExtensionRole)
	}
	roles, err := types.ParseTeleportRoles(roleString)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	foundRoles := len(roles)
	if foundRoles != 1 {
		return nil, trace.Errorf("expected one role per certificate. found %d: '%s'",
			foundRoles, roles.String())
	}
	role := roles[0]
	clusterName := cert.Permissions.Extensions[utils.CertExtensionAuthority]
	if clusterName == "" {
		return nil, trace.BadParameter("missing cert extension %v", utils.CertExtensionAuthority)
	}

	return &Identity{
		ID:          IdentityID{HostUUID: cert.ValidPrincipals[0], Role: role},
		ClusterName: clusterName,
		KeyBytes:    keyBytes,
		CertBytes:   certBytes,
		KeySigner:   certSigner,
		Cert:        cert,
	}, nil
}
