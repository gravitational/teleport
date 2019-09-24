/*
Copyright 2019 Gravitational, Inc.

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

package common

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"io"
	"io/ioutil"
	"net"
	"os"

	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/sshutils"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
)

// LoadIdentity loads the private key + certificate from a file
// Returns:
//	 - client key: user's private key+cert
//   - host auth callback: function to validate the host (may be null)
//   - error, if something happens when reading the identity file
//
// If the "host auth callback" is not returned, user will be prompted to
// trust the proxy server.
func LoadIdentity(idFn string) (*client.Key, ssh.HostKeyCallback, error) {
	logrus.Infof("Reading identity file: %v", idFn)

	f, err := os.Open(idFn)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	defer f.Close()
	ident, err := decodeIdentity(f)
	if err != nil {
		return nil, nil, trace.Wrap(err, "failed to parse identity file")
	}
	// did not find the certificate in the file? look in a separate file with
	// -cert.pub prefix
	if len(ident.Certs.SSH) == 0 {
		certFn := idFn + "-cert.pub"
		logrus.Infof("Certificate not found in %s. Looking in %s.", idFn, certFn)
		ident.Certs.SSH, err = ioutil.ReadFile(certFn)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}
	}
	// validate both by parsing them:
	privKey, err := ssh.ParseRawPrivateKey(ident.PrivateKey)
	if err != nil {
		return nil, nil, trace.BadParameter("invalid identity: %s. %v", idFn, err)
	}
	signer, err := ssh.NewSignerFromKey(privKey)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	// validate TLS Cert (if present):
	if len(ident.Certs.TLS) > 0 {
		_, err := tls.X509KeyPair(ident.Certs.TLS, ident.PrivateKey)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}
	}
	// Validate TLS CA certs (if present).
	var trustedCA []auth.TrustedCerts
	if len(ident.CACerts.TLS) > 0 {
		var trustedCerts auth.TrustedCerts
		pool := x509.NewCertPool()
		for i, certPEM := range ident.CACerts.TLS {
			if !pool.AppendCertsFromPEM(certPEM) {
				return nil, nil, trace.BadParameter("identity file contains invalid TLS CA cert (#%v)", i+1)
			}
			trustedCerts.TLSCertificates = append(trustedCerts.TLSCertificates, certPEM)
		}
		trustedCA = []auth.TrustedCerts{trustedCerts}
	}
	var hostAuthFunc ssh.HostKeyCallback = nil
	// validate CA (cluster) cert
	if len(ident.CACerts.SSH) > 0 {
		var trustedKeys []ssh.PublicKey
		for _, caCert := range ident.CACerts.SSH {
			_, _, publicKey, _, _, err := ssh.ParseKnownHosts(caCert)
			if err != nil {
				return nil, nil, trace.BadParameter("CA cert parsing error: %v. cert line :%v",
					err.Error(), string(caCert))
			}
			trustedKeys = append(trustedKeys, publicKey)
		}

		// found CA cert in the indentity file? construct the host key checking function
		// and return it:
		hostAuthFunc = func(host string, a net.Addr, hostKey ssh.PublicKey) error {
			clusterCert, ok := hostKey.(*ssh.Certificate)
			if ok {
				hostKey = clusterCert.SignatureKey
			}
			for _, trustedKey := range trustedKeys {
				if sshutils.KeysEqual(trustedKey, hostKey) {
					return nil
				}
			}
			err = trace.AccessDenied("host %v is untrusted", host)
			logrus.Error(err)
			return err
		}
	}
	return &client.Key{
		Priv:      ident.PrivateKey,
		Pub:       signer.PublicKey().Marshal(),
		Cert:      ident.Certs.SSH,
		TLSCert:   ident.Certs.TLS,
		TrustedCA: trustedCA,
	}, hostAuthFunc, nil
}

// rawIdentity encodes the basic components of an identity file.
type rawIdentity struct {
	PrivateKey []byte
	Certs      struct {
		SSH []byte
		TLS []byte
	}
	CACerts struct {
		SSH [][]byte
		TLS [][]byte
	}
}

// decodeIdentity attempts to break up the contents of an identity file
// into its respective components.
func decodeIdentity(r io.Reader) (*rawIdentity, error) {
	scanner := bufio.NewScanner(r)
	var ident rawIdentity
	// Subslice of scanner's buffer pointing to current line
	// with leading and trailing whitespace trimmed.
	var line []byte
	// Attempt to scan to the next line.
	scanln := func() bool {
		if !scanner.Scan() {
			line = nil
			return false
		}
		line = bytes.TrimSpace(scanner.Bytes())
		return true
	}
	// Check if the current line starts with prefix `p`.
	peekln := func(p string) bool {
		return bytes.HasPrefix(line, []byte(p))
	}
	// Get an "owned" copy of the current line.
	cloneln := func() []byte {
		ln := make([]byte, len(line))
		copy(ln, line)
		return ln
	}
	// Scan through all lines of identity file.  Lines with a known prefix
	// are copied out of the scanner's buffer.  All others are ignored.
	for scanln() {
		switch {
		case peekln("ssh"):
			ident.Certs.SSH = cloneln()
		case peekln("@cert-authority"):
			ident.CACerts.SSH = append(ident.CACerts.SSH, cloneln())
		case peekln("-----BEGIN"):
			// Current line marks the beginning of a PEM block.  Consume all
			// lines until a corresponding END is found.
			var pemBlock []byte
			for {
				pemBlock = append(pemBlock, line...)
				pemBlock = append(pemBlock, '\n')
				if peekln("-----END") {
					break
				}
				if !scanln() {
					// If scanner has terminated in the middle of a PEM block, either
					// the reader encountered an error, or the PEM block is a fragment.
					if err := scanner.Err(); err != nil {
						return nil, trace.Wrap(err)
					}
					return nil, trace.BadParameter("invalid PEM block (fragment)")
				}
			}
			// Decide where to place the pem block based on
			// which pem blocks have already been found.
			switch {
			case ident.PrivateKey == nil:
				ident.PrivateKey = pemBlock
			case ident.Certs.TLS == nil:
				ident.Certs.TLS = pemBlock
			default:
				ident.CACerts.TLS = append(ident.CACerts.TLS, pemBlock)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &ident, nil
}
