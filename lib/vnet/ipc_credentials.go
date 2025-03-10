// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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

package vnet

import (
	"crypto"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"os"
	"path/filepath"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/tlsca"
)

type ipcCredentials struct {
	// server holds credentials for the server-side of the connection (the VNet
	// client application).
	server credentials
	// client holds credentials for the client-side of the connection (the VNet
	// admin process).
	client credentials
}

type credentials struct {
	trustedCAPEM []byte        // X.509 CA certificate
	certPEM      []byte        // X.509 certificate for the VNet process
	signer       crypto.Signer // Private key associated with cert
}

func newIPCCredentials() (*ipcCredentials, error) {
	const (
		// We don't know which clusters will be connected to at this point so
		// there's no way to fetch the cluster signature_algorithm_suite or unify
		// suites across multiple root clusters, so just statically use ECDSA
		// P-256 for these keys.
		keyAlgo = cryptosuites.ECDSAP256
		// These certs need to be valid for the full VNet process lifetime,
		// which could be longer than any individual Teleport session. Going
		// with 30 days for now, which should be more than long enough.
		certTTL                = 30 * 24 * time.Hour
		certOrganizationalUnit = "TeleportVNet"
	)

	serverCASigner, err := cryptosuites.GenerateKeyWithAlgorithm(keyAlgo)
	if err != nil {
		return nil, trace.Wrap(err, "generating server CA key")
	}
	clientCASigner, err := cryptosuites.GenerateKeyWithAlgorithm(keyAlgo)
	if err != nil {
		return nil, trace.Wrap(err, "generating client CA key")
	}
	serverSigner, err := cryptosuites.GenerateKeyWithAlgorithm(keyAlgo)
	if err != nil {
		return nil, trace.Wrap(err, "generating server key")
	}
	clientSigner, err := cryptosuites.GenerateKeyWithAlgorithm(keyAlgo)
	if err != nil {
		return nil, trace.Wrap(err, "generating client key")
	}

	serverCAPEM, err := tlsca.GenerateSelfSignedCAWithSigner(
		serverCASigner,
		pkix.Name{
			OrganizationalUnit: []string{certOrganizationalUnit},
			CommonName:         "Server CA",
		},
		nil, // dnsNames
		certTTL,
	)
	if err != nil {
		return nil, trace.Wrap(err, "generating self-signed server CA")
	}
	serverCA, err := tlsca.FromCertAndSigner(serverCAPEM, serverCASigner)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	clientCAPEM, err := tlsca.GenerateSelfSignedCAWithSigner(
		clientCASigner,
		pkix.Name{
			OrganizationalUnit: []string{certOrganizationalUnit},
			CommonName:         "Client CA",
		},
		nil, // dnsNames
		certTTL,
	)
	if err != nil {
		return nil, trace.Wrap(err, "generating self-signed client CA")
	}
	clientCA, err := tlsca.FromCertAndSigner(clientCAPEM, clientCASigner)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	now := time.Now()
	serverCertPEM, err := serverCA.GenerateCertificate(tlsca.CertificateRequest{
		PublicKey: serverSigner.Public(),
		Subject: pkix.Name{
			OrganizationalUnit: []string{certOrganizationalUnit},
			CommonName:         "localhost",
		},
		DNSNames: []string{"localhost", "127.0.0.1", "::1"},
		NotAfter: now.Add(certTTL),
	})
	if err != nil {
		return nil, trace.Wrap(err, "generating server TLS certificate")
	}
	clientCertPEM, err := clientCA.GenerateCertificate(tlsca.CertificateRequest{
		PublicKey: clientSigner.Public(),
		Subject: pkix.Name{
			OrganizationalUnit: []string{certOrganizationalUnit},
			CommonName:         "client",
		},
		NotAfter: now.Add(certTTL),
	})
	if err != nil {
		return nil, trace.Wrap(err, "generating client TLS certificate")
	}

	return &ipcCredentials{
		server: credentials{
			trustedCAPEM: clientCAPEM,
			certPEM:      serverCertPEM,
			signer:       serverSigner,
		},
		client: credentials{
			trustedCAPEM: serverCAPEM,
			certPEM:      clientCertPEM,
			signer:       clientSigner,
		},
	}, nil
}

func (c *credentials) serverTLSConfig() (*tls.Config, error) {
	caPool := x509.NewCertPool()
	if !caPool.AppendCertsFromPEM(c.trustedCAPEM) {
		return nil, trace.Errorf("parsing trusted CA certificate")
	}

	tlsCert, err := keys.TLSCertificateForSigner(c.signer, c.certPEM)
	if err != nil {
		return nil, trace.Wrap(err, "parsing VNet server certificate")
	}

	return &tls.Config{
		Certificates: []tls.Certificate{tlsCert},
		ClientCAs:    caPool,
		ClientAuth:   tls.RequireAndVerifyClientCert,
		MinVersion:   tls.VersionTLS13,
	}, nil
}

func (c *credentials) clientTLSConfig() (*tls.Config, error) {
	caPool := x509.NewCertPool()
	if !caPool.AppendCertsFromPEM(c.trustedCAPEM) {
		return nil, trace.Errorf("parsing trusted CA certificate")
	}

	tlsCert, err := keys.TLSCertificateForSigner(c.signer, c.certPEM)
	if err != nil {
		return nil, trace.Wrap(err, "parsing VNet client certificate")
	}

	return &tls.Config{
		Certificates: []tls.Certificate{tlsCert},
		RootCAs:      caPool,
		MinVersion:   tls.VersionTLS13,
	}, nil
}

const (
	caFileName   = "ca.pem"
	certFileName = "cert.pem"
	keyFileName  = "key.pem"
)

// write writes the credentials to the filesystem directory.
func (c *credentials) write(dir string) (err error) {
	// Attempt to clean up if returning an error for any reason.
	defer func() {
		if err == nil {
			return
		}
		deleteErr := trace.Wrap(c.remove(dir), "cleaning up after failing to write credentials")
		err = trace.NewAggregate(err, deleteErr)
	}()
	keyPEM, err := keys.MarshalPrivateKey(c.signer)
	if err != nil {
		return trace.Wrap(err)
	}
	for fileName, data := range map[string][]byte{
		caFileName:   c.trustedCAPEM,
		certFileName: c.certPEM,
		keyFileName:  keyPEM,
	} {
		filePath := filepath.Join(dir, fileName)
		if err := os.WriteFile(filePath, data, 0600); err != nil {
			return trace.Wrap(err, "writing service credential file %s", filePath)
		}
	}
	return nil
}

// remove removes the files from the filesystem directory.
// Note: can't just call os.RemoveAll in case the current user does not have
// permissions to list files.
func (c *credentials) remove(dir string) error {
	var errors []error
	for _, fileName := range []string{
		caFileName, certFileName, keyFileName,
	} {
		filePath := filepath.Join(dir, fileName)
		if err := os.Remove(filePath); err != nil {
			errors = append(errors, trace.Wrap(err, "deleting service credential file %s", filePath))
		}
	}
	return trace.NewAggregate(errors...)
}

func readCredentials(dir string) (*credentials, error) {
	caBytes, err := os.ReadFile(filepath.Join(dir, caFileName))
	if err != nil {
		return nil, trace.Wrap(err, "reading service credential file %s", caFileName)
	}
	certBytes, err := os.ReadFile(filepath.Join(dir, certFileName))
	if err != nil {
		return nil, trace.Wrap(err, "reading service credential file %s", certFileName)
	}
	keyBytes, err := os.ReadFile(filepath.Join(dir, keyFileName))
	if err != nil {
		return nil, trace.Wrap(err, "reading service credential file %s", keyFileName)
	}
	signer, err := keys.ParsePrivateKey(keyBytes)
	if err != nil {
		return nil, trace.Wrap(err, "parsing service private key")
	}
	return &credentials{
		trustedCAPEM: caBytes,
		certPEM:      certBytes,
		signer:       signer,
	}, nil
}
