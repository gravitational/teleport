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
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"strings"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/utils/keys"
	apisshutils "github.com/gravitational/teleport/api/utils/sshutils"
	"github.com/gravitational/teleport/lib/tbot/bot"
	"github.com/gravitational/teleport/lib/tlsca"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

const (
	// TLSCertKey is the name under which TLS certificates exist in a destination.
	TLSCertKey = "tlscert"

	// SSHCertKey is the name under which SSH certificates exist in a destination.
	SSHCertKey = "key-cert.pub"

	// SSHCACertsKey is the name under which SSH CA certificates exist in a destination.
	SSHCACertsKey = "sshcacerts"

	// TLSCACertsKey is the name under which SSH CA certificates exist in a destination.
	TLSCACertsKey = "tlscacerts"

	// PrivateKeyKey is the name under which the private key exists in a destination.
	// The same private key is used for SSH and TLS certificates.
	PrivateKeyKey = "key"

	// PublicKeyKey is the ssh public key, required for successful SSH connections.
	PublicKeyKey = "key.pub"

	// TokenHashKey is the key where a hash of the onboarding token will be stored.
	TokenHashKey = "tokenhash"

	// WriteTestKey is the key for a file used to check that the destination is
	// writable.
	WriteTestKey = ".write-test"
)

var log = logutils.NewPackageLogger(teleport.ComponentKey, teleport.ComponentTBot)

// Identity is collection of raw key and certificate data as well as the
// parsed equivalents that make up a Teleport identity.
type Identity struct {
	// PrivateKeyBytes is a PEM encoded private key.
	PrivateKeyBytes []byte
	// PublicKeyBytes is the public key corresponding to [PrivateKeyBytes] in
	// SSH authorized_keys format.
	PublicKeyBytes []byte
	// CertBytes is a PEM encoded SSH user cert
	CertBytes []byte
	// TLSCertBytes is a PEM encoded TLS x509 client certificate
	TLSCertBytes []byte
	// TLSCACertBytes is a list of PEM encoded TLS x509 certificate of certificate authority
	// associated with auth server services
	TLSCACertsBytes [][]byte
	// SSHCACertBytes is a list of SSH CAs encoded in the authorized_keys format.
	SSHCACertBytes [][]byte
	// TokenHashBytes is the hash of the original join token
	TokenHashBytes []byte

	// Below fields are "computed" by ReadIdentityFromStore - this essentially
	// validates the raw data and saves these being continually recomputed.
	// CertSigner is an ssh.Signer for the certificate and private key.
	CertSigner ssh.Signer
	// PrivateKey is a crypto.Signer for the private key.
	PrivateKey *keys.PrivateKey
	// SSHCert is a parsed SSH certificate
	SSHCert *ssh.Certificate
	// SSHHostCheckers holds the parsed SSH CAs
	SSHHostCheckers []ssh.PublicKey
	// X509Cert is the parsed X509 client certificate
	X509Cert *x509.Certificate
	// TLSCAPool is the parsed TLS CAs
	TLSCAPool *x509.CertPool
	// TLSCert is the parsed TLS client certificate
	TLSCert *tls.Certificate
	// ClusterName is a name of host's cluster determined from the
	// x509 certificate.
	ClusterName string
	// TLSIdentity is the parsed TLS identity based on the X509 certificate.
	TLSIdentity *tlsca.Identity
}

// LoadIdentityParams contains parameters beyond proto.Certs needed to load a
// stored identity.
type LoadIdentityParams struct {
	PrivateKeyBytes []byte
	PublicKeyBytes  []byte
	TokenHashBytes  []byte
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

// ReadIdentityFromStore reads stored identity credentials
func ReadIdentityFromStore(params *LoadIdentityParams, certs *proto.Certs) (*Identity, error) {
	// Note: in practice we should always expect certificates to have all
	// fields set even though destinations do not contain sufficient data to
	// load a stored identity. This works in practice because we never read
	// destination identities from disk and only read them from the result of
	// `generateUserCerts`, which is always fully-formed.
	switch {
	case len(certs.SSH) == 0:
		return nil, trace.BadParameter("identity requires SSH certificates but they are unset")
	case len(params.PrivateKeyBytes) == 0:
		return nil, trace.BadParameter("missing private key")
	case len(certs.TLSCACerts) == 0 || len(certs.TLS) == 0:
		return nil, trace.BadParameter("identity requires TLS certificates but they are empty")
	}

	privateKey, err := keys.ParsePrivateKey(params.PrivateKeyBytes)
	if err != nil {
		return nil, trace.Wrap(err, "parsing private key")
	}

	sshHostCheckers, keySigner, sshCert, err := parseSSHIdentity(
		params.PrivateKeyBytes, certs.SSH, certs.SSHCACerts,
	)
	if err != nil {
		return nil, trace.Wrap(err, "parsing ssh identity")
	}

	clusterName, tlsIdent, x509Cert, tlsCert, tlsCAPool, err := ParseTLSIdentity(
		params.PrivateKeyBytes, certs.TLS, certs.TLSCACerts,
	)
	if err != nil {
		return nil, trace.Wrap(err, "parsing tls identity")
	}

	return &Identity{
		PublicKeyBytes:  params.PublicKeyBytes,
		PrivateKeyBytes: params.PrivateKeyBytes,
		CertBytes:       certs.SSH,
		SSHCACertBytes:  certs.SSHCACerts,
		TLSCertBytes:    certs.TLS,
		TLSCACertsBytes: certs.TLSCACerts,
		TokenHashBytes:  params.TokenHashBytes,

		// These fields are "computed"
		ClusterName:     clusterName,
		CertSigner:      keySigner,
		PrivateKey:      privateKey,
		SSHCert:         sshCert,
		SSHHostCheckers: sshHostCheckers,
		X509Cert:        x509Cert,
		TLSCert:         tlsCert,
		TLSCAPool:       tlsCAPool,
		TLSIdentity:     tlsIdent,
	}, nil
}

// ParseTLSIdentity reads TLS identity from key pair
func ParseTLSIdentity(
	keyBytes []byte, certBytes []byte, caCertsBytes [][]byte,
) (
	clusterName string,
	tlsIdentity *tlsca.Identity,
	x509Cert *x509.Certificate,
	tlsCert *tls.Certificate,
	certPool *x509.CertPool,
	err error,
) {
	x509Cert, err = tlsca.ParseCertificatePEM(certBytes)
	if err != nil {
		return "", nil, nil, nil, nil, trace.Wrap(err, "parsing certificate")
	}

	if len(x509Cert.Issuer.Organization) == 0 {
		return "", nil, nil, nil, nil, trace.BadParameter("certificate missing CA organization")
	}
	clusterName = x509Cert.Issuer.Organization[0]
	if clusterName == "" {
		return "", nil, nil, nil, nil, trace.BadParameter("certificate missing cluster name")
	}

	certPool = x509.NewCertPool()
	for j := range caCertsBytes {
		parsedCert, err := tlsca.ParseCertificatePEM(caCertsBytes[j])
		if err != nil {
			return "", nil, nil, nil, nil, trace.Wrap(err, "parsing CA certificate")
		}
		certPool.AddCert(parsedCert)
	}

	cert, err := keys.X509KeyPair(certBytes, keyBytes)
	if err != nil {
		return "", nil, nil, nil, nil, trace.Wrap(err, "parse private key")
	}

	tlsIdent, err := tlsca.FromSubject(x509Cert.Subject, x509Cert.NotAfter)
	if err != nil {
		return "", nil, nil, nil, nil, trace.Wrap(err, "parse tls identity")
	}

	return clusterName, tlsIdent, x509Cert, &cert, certPool, nil
}

// parseSSHIdentity reads identity from initialized keypair
func parseSSHIdentity(
	keyBytes, certBytes []byte, caBytes [][]byte,
) (hostCheckers []ssh.PublicKey, certSigner ssh.Signer, cert *ssh.Certificate, err error) {
	cert, err = apisshutils.ParseCertificate(certBytes)
	if err != nil {
		return nil, nil, nil, trace.Wrap(err, "parsing certificate")
	}

	signer, err := ssh.ParsePrivateKey(keyBytes)
	if err != nil {
		return nil, nil, nil, trace.Wrap(err, "parsing key")
	}
	// this signer authenticates using certificate signed by the cert authority
	// not only by the public key
	certSigner, err = ssh.NewCertSigner(cert, signer)
	if err != nil {
		return nil, nil, nil, trace.Wrap(err, "creating signer from certificate and key")
	}

	// check principals on certificate
	if len(cert.ValidPrincipals) < 1 {
		return nil, nil, nil, trace.BadParameter("valid principals: at least one valid principal is required")
	}
	for _, validPrincipal := range cert.ValidPrincipals {
		if validPrincipal == "" {
			return nil, nil, nil, trace.BadParameter("valid principal can not be empty: %q", cert.ValidPrincipals)
		}
	}

	hostCheckers, err = apisshutils.ParseAuthorizedKeys(caBytes)
	if err != nil {
		return nil, nil, nil, trace.Wrap(err, "parsing ca bytes")
	}

	return hostCheckers, certSigner, cert, nil
}

// VerifyWrite attempts to write to the .write-test artifact inside the given
// destination. It should be called before attempting a renewal to help ensure
// we won't then fail to save the identity.
func VerifyWrite(ctx context.Context, dest bot.Destination) error {
	return trace.Wrap(dest.Write(ctx, WriteTestKey, []byte{}))
}

// ListKeys returns a list of artifact keys that will be written given a list
// of artifacts.
func ListKeys(kinds ...ArtifactKind) []string {
	keys := []string{}
	for _, artifact := range GetArtifacts() {
		if !artifact.Matches(kinds...) {
			continue
		}

		keys = append(keys, artifact.Key)
	}

	return keys
}

// SaveIdentity saves a bot identity to a destination.
func SaveIdentity(ctx context.Context, id *Identity, d bot.Destination, kinds ...ArtifactKind) error {
	for _, artifact := range GetArtifacts() {
		// Only store artifacts matching one of the set kinds.
		if !artifact.Matches(kinds...) {
			continue
		}

		data := artifact.ToBytes(id)

		log.DebugContext(ctx, "Writing artifact", "key", artifact.Key)
		if err := d.Write(ctx, artifact.Key, data); err != nil {
			return trace.Wrap(err, "could not write to %v", artifact.Key)
		}
	}

	return nil
}

// LoadIdentity loads a bot identity from a destination.
func LoadIdentity(ctx context.Context, d bot.Destination, kinds ...ArtifactKind) (*Identity, error) {
	var certs proto.Certs
	var params LoadIdentityParams

	for _, artifact := range GetArtifacts() {
		// Only attempt to load artifacts matching one of the specified kinds
		if !artifact.Matches(kinds...) {
			continue
		}

		data, err := d.Read(ctx, artifact.Key)
		if err != nil {
			return nil, trace.Wrap(err, "could not read artifact %q from destination %s", artifact.Key, d)
		}

		// Attempt to load from an old key if there was no data in the current
		// key. This will be in the case as d.Read for the file destination will
		// not throw an error if the file does not exist.
		// This allows migrations of key names.
		if artifact.OldKey != "" && len(data) == 0 {
			log.DebugContext(
				ctx,
				"Unable to load from current key, trying to migrate from old key",
				"key", artifact.Key,
				"old_key", artifact.OldKey,
			)
			data, err = d.Read(ctx, artifact.OldKey)
			if err != nil {
				return nil, trace.Wrap(
					err,
					"could not read artifact %q from destination %q",
					artifact.OldKey,
					d,
				)
			}
		}

		// We generally expect artifacts to exist beforehand regardless of
		// whether or not they've actually been written to (due to `tbot init`
		// or our lightweight init during `tbot start`). If required artifacts
		// are empty, this identity can't be loaded.
		if !artifact.Optional && len(data) == 0 {
			return nil, trace.NotFound("artifact %q is unexpectedly empty in destination %s", artifact.Key, d)
		}

		artifact.FromBytes(&certs, &params, data)
	}

	log.DebugContext(
		ctx,
		"Loaded SSH CA certs and TLS CA certs",
		"ssh_ca_len", len(certs.SSHCACerts),
		"tls_ca_len", len(certs.TLSCACerts),
	)

	return ReadIdentityFromStore(&params, &certs)
}
