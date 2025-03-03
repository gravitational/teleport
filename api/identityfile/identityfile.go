/*
Copyright 2021 Gravitational, Inc.

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

// Package identityfile implements parsing and serialization of Teleport identity files.
package identityfile

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/api/utils/keypaths"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/api/utils/sshutils"
)

const (
	// FilePermissions defines file permissions for identity files.
	//
	// Specifically, for postgres, this must be 0600 or 0640 (choosing 0600 as it's more restrictive)
	// https://www.postgresql.org/docs/current/libpq-ssl.html
	// On Unix systems, the permissions on the private key file must disallow any access to world or group;
	//  achieve this by a command such as chmod 0600 ~/.postgresql/postgresql.key.
	// Alternatively, the file can be owned by root and have group read access (that is, 0640 permissions).
	//
	// Other services should accept 0600 as well, if not, we must change the Write function (in `lib/client/identityfile/identity.go`)
	FilePermissions = 0600
)

// IdentityFile represents the basic components of an identity file.
type IdentityFile struct {
	// PrivateKey is PEM encoded private key data.
	PrivateKey []byte
	// Certs contains PEM encoded certificates.
	Certs Certs
	// CACerts contains PEM encoded CA certificates.
	CACerts CACerts
}

// Certs contains PEM encoded certificates.
type Certs struct {
	// SSH is a cert used for SSH.
	SSH []byte
	// TLS is a cert used for TLS.
	TLS []byte
}

// CACerts contains PEM encoded CA certificates.
type CACerts struct {
	// SSH are CA certs used for SSH in known_hosts format.
	SSH [][]byte
	// TLS are CA certs used for TLS.
	TLS [][]byte
}

// TLSConfig returns the identity file's associated TLSConfig.
func (i *IdentityFile) TLSConfig() (*tls.Config, error) {
	cert, err := keys.X509KeyPair(i.Certs.TLS, i.PrivateKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	pool := x509.NewCertPool()
	for _, caCerts := range i.CACerts.TLS {
		if !pool.AppendCertsFromPEM(caCerts) {
			return nil, trace.BadParameter("invalid CA cert PEM")
		}
	}

	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      pool,
	}, nil
}

// SSHClientConfig returns the identity file's associated SSHClientConfig.
func (i *IdentityFile) SSHClientConfig() (*ssh.ClientConfig, error) {
	sshCert, err := sshutils.ParseCertificate(i.Certs.SSH)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	priv, err := keys.ParsePrivateKey(i.PrivateKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ssh, err := sshutils.ProxyClientSSHConfig(sshCert, priv, i.CACerts.SSH...)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return ssh, nil
}

// Expiry returns the credential expiry.
func (i *IdentityFile) Expiry() (time.Time, bool) {
	if i.Certs.TLS == nil {
		return time.Time{}, false
	}
	cert, _, err := keys.X509Certificate(i.Certs.TLS)
	if err != nil {
		return time.Time{}, false
	}
	return cert.NotAfter, true
}

// Write writes the given identityFile to the specified path.
func Write(idFile *IdentityFile, path string) error {
	buf := new(bytes.Buffer)
	if err := encodeIdentityFile(buf, idFile); err != nil {
		return trace.Wrap(err)
	}
	if err := os.WriteFile(path, buf.Bytes(), FilePermissions); err != nil {
		return trace.ConvertSystemError(err)
	}
	return nil
}

// Encode encodes the given identityFile to bytes.
func Encode(idFile *IdentityFile) ([]byte, error) {
	buf := new(bytes.Buffer)
	if err := encodeIdentityFile(buf, idFile); err != nil {
		return nil, trace.Wrap(err)
	}

	return buf.Bytes(), nil
}

// Read reads an identity file from generic io.Reader interface.
func Read(r io.Reader) (*IdentityFile, error) {
	ident, err := decodeIdentityFile(r)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if len(ident.Certs.SSH) == 0 {
		return nil, trace.BadParameter("could not find SSH cert in the identity file")
	}

	return ident, nil
}

// ReadFile reads an identity file from a given path.
func ReadFile(path string) (*IdentityFile, error) {
	r, err := os.Open(path)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer r.Close()

	ident, err := decodeIdentityFile(r)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Did not find the SSH certificate in the file? look in a
	// separate file with -cert.pub suffix.
	if len(ident.Certs.SSH) == 0 {
		certFn := keypaths.IdentitySSHCertPath(path)
		if ident.Certs.SSH, err = os.ReadFile(certFn); err != nil {
			return nil, trace.Wrap(err, "could not find SSH cert in the identity file or %v", certFn)
		}
	}

	return ident, nil
}

// FromString reads an identity file from a string.
func FromString(content string) (*IdentityFile, error) {
	ident, err := decodeIdentityFile(strings.NewReader(content))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if len(ident.Certs.SSH) == 0 {
		return nil, trace.BadParameter("could not find SSH cert in the identity file")
	}

	return ident, nil
}

// encodeIdentityFile combines the components of an identity file in its file format.
func encodeIdentityFile(w io.Writer, idFile *IdentityFile) error {
	// write key:
	if err := writeWithNewline(w, idFile.PrivateKey); err != nil {
		return trace.Wrap(err)
	}
	// append ssh cert:
	if err := writeWithNewline(w, idFile.Certs.SSH); err != nil {
		return trace.Wrap(err)
	}
	// append tls cert:
	if err := writeWithNewline(w, idFile.Certs.TLS); err != nil {
		return trace.Wrap(err)
	}
	// append ssh ca certificates
	for _, caCert := range idFile.CACerts.SSH {
		if err := writeWithNewline(w, caCert); err != nil {
			return trace.Wrap(err)
		}
	}
	// append tls ca certificates
	for _, caCert := range idFile.CACerts.TLS {
		if err := writeWithNewline(w, caCert); err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}

func writeWithNewline(w io.Writer, data []byte) error {
	if _, err := w.Write(data); err != nil {
		return trace.Wrap(err)
	}
	if bytes.HasSuffix(data, []byte{'\n'}) {
		return nil
	}
	_, err := fmt.Fprintln(w)
	return trace.Wrap(err)
}

// decodeIdentityFile attempts to break up the contents of an identity file into its
// respective components.
func decodeIdentityFile(idFile io.Reader) (*IdentityFile, error) {
	scanner := bufio.NewScanner(idFile)
	var ident IdentityFile
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
	hasPrefix := func(p string) bool {
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
		case isSSHCert(line):
			ident.Certs.SSH = append(cloneln(), '\n')
		case hasPrefix("@cert-authority"):
			ident.CACerts.SSH = append(ident.CACerts.SSH, append(cloneln(), '\n'))
		case hasPrefix("-----BEGIN"):
			// Current line marks the beginning of a PEM block.  Consume all
			// lines until a corresponding END is found.
			var pemBlock []byte
			for {
				pemBlock = append(pemBlock, line...)
				pemBlock = append(pemBlock, '\n')
				if hasPrefix("-----END") {
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

// Check if the given data has an ssh cert type prefix as it's first part.
func isSSHCert(data []byte) bool {
	sshCertType := bytes.Split(data, []byte(" "))[0]
	return sshutils.IsSSHCertType(string(sshCertType))
}
