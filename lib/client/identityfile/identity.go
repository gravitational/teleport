/*
Copyright 2020 Gravitational, Inc.

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

// Package identityfile handles formatting and parsing of identity files.
package identityfile

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"

	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/kube/kubeconfig"
	"github.com/gravitational/teleport/lib/sshutils"

	"github.com/gravitational/trace"
)

// Format describes possible file formats how a user identity can be stored.
type Format string

const (
	// FormatFile is when a key + cert are stored concatenated into a single file
	FormatFile Format = "file"

	// FormatOpenSSH is OpenSSH-compatible format, when a key and a cert are stored in
	// two different files (in the same directory)
	FormatOpenSSH Format = "openssh"

	// FormatTLS is a standard TLS format used by common TLS clients (e.g. GRPC) where
	// certificate and key are stored in separate files.
	FormatTLS Format = "tls"

	// FormatKubernetes is a standard Kubernetes format, with all credentials
	// stored in a "kubeconfig" file.
	FormatKubernetes Format = "kubernetes"

	// FormatDatabase produces CA and key pair suitable for configuring a
	// database instance for mutual TLS.
	FormatDatabase Format = "db"

	// DefaultFormat is what Teleport uses by default
	DefaultFormat = FormatFile
)

const (
	// The files created by Write will have these permissions.
	writeFileMode = 0600
)

// WriteConfig holds the necessary information to write an identity file.
type WriteConfig struct {
	// OutputPath is the output path for the identity file. Note that some
	// formats (like FormatOpenSSH and FormatTLS) write multiple output files
	// and use OutputPath as a prefix.
	OutputPath string
	// Key contains the credentials to write to the identity file.
	Key *client.Key
	// Format is the output format for the identity file.
	Format Format
	// KubeProxyAddr is the public address of the proxy with its kubernetes
	// port. KubeProxyAddr is only used when Format is FormatKubernetes.
	KubeProxyAddr string
	// OverwriteDestination forces all existing destination files to be
	// overwritten. When false, user will be prompted for confirmation of
	// overwite first.
	OverwriteDestination bool
}

// Write writes user credentials to disk in a specified format.
// It returns the names of the files successfully written.
func Write(cfg WriteConfig) (filesWritten []string, err error) {
	if cfg.OutputPath == "" {
		return nil, trace.BadParameter("identity output path is not specified")
	}

	switch cfg.Format {
	// dump user identity into a single file:
	case FormatFile:
		if err := checkOverwrite(cfg.OutputPath, cfg.OverwriteDestination); err != nil {
			return nil, trace.Wrap(err)
		}
		filesWritten = append(filesWritten, cfg.OutputPath)
		f, err := os.OpenFile(cfg.OutputPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, writeFileMode)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		defer f.Close()

		// write key:
		if err := writeWithNewline(f, cfg.Key.Priv); err != nil {
			return nil, trace.Wrap(err)
		}
		// append ssh cert:
		if err := writeWithNewline(f, cfg.Key.Cert); err != nil {
			return nil, trace.Wrap(err)
		}
		// append tls cert:
		if err := writeWithNewline(f, cfg.Key.TLSCert); err != nil {
			return nil, trace.Wrap(err)
		}
		// append trusted host certificate authorities
		for _, ca := range cfg.Key.TrustedCA {
			// append ssh ca certificates
			for _, publicKey := range ca.HostCertificates {
				data, err := sshutils.MarshalAuthorizedHostsFormat(ca.ClusterName, publicKey, nil)
				if err != nil {
					return nil, trace.Wrap(err)
				}
				if err := writeWithNewline(f, []byte(data)); err != nil {
					return nil, trace.Wrap(err)
				}
			}
			// append tls ca certificates
			for _, cert := range ca.TLSCertificates {
				if err := writeWithNewline(f, cert); err != nil {
					return nil, trace.Wrap(err)
				}
			}
		}

	// dump user identity into separate files:
	case FormatOpenSSH:
		keyPath := cfg.OutputPath
		certPath := keyPath + "-cert.pub"
		filesWritten = append(filesWritten, keyPath, certPath)

		err = writeFile(certPath, cfg.Key.Cert, cfg.OverwriteDestination)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		err = writeFile(keyPath, cfg.Key.Priv, cfg.OverwriteDestination)
		if err != nil {
			return nil, trace.Wrap(err)
		}

	case FormatTLS, FormatDatabase:
		keyPath := cfg.OutputPath + ".key"
		certPath := cfg.OutputPath + ".crt"
		casPath := cfg.OutputPath + ".cas"
		filesWritten = append(filesWritten, keyPath, certPath, casPath)

		err = writeFile(certPath, cfg.Key.TLSCert, cfg.OverwriteDestination)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		err = writeFile(keyPath, cfg.Key.Priv, cfg.OverwriteDestination)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		var caCerts []byte
		for _, ca := range cfg.Key.TrustedCA {
			for _, cert := range ca.TLSCertificates {
				caCerts = append(caCerts, cert...)
			}
		}
		err = writeFile(casPath, caCerts, cfg.OverwriteDestination)
		if err != nil {
			return nil, trace.Wrap(err)
		}

	case FormatKubernetes:
		if err := checkOverwrite(cfg.OutputPath, cfg.OverwriteDestination); err != nil {
			return nil, trace.Wrap(err)
		}
		// Clean up the existing file, if it exists.
		//
		// kubeconfig.Update would try to parse it and merge in new
		// credentials, which is now what we want.
		if err := os.Remove(cfg.OutputPath); err != nil && !os.IsNotExist(err) {
			return nil, trace.Wrap(err)
		}

		filesWritten = append(filesWritten, cfg.OutputPath)
		if err := kubeconfig.Update(cfg.OutputPath, kubeconfig.Values{
			TeleportClusterName: cfg.Key.ClusterName,
			ClusterAddr:         cfg.KubeProxyAddr,
			Credentials:         cfg.Key,
		}); err != nil {
			return nil, trace.Wrap(err)
		}

	default:
		return nil, trace.BadParameter("unsupported identity format: %q, use one of %q, %q, %q, %q or %q",
			cfg.Format, FormatFile, FormatOpenSSH, FormatTLS, FormatKubernetes, FormatDatabase)
	}
	return filesWritten, nil
}

func writeWithNewline(w io.Writer, data []byte) error {
	if _, err := w.Write(data); err != nil {
		return trace.Wrap(err)
	}
	if !bytes.HasSuffix(data, []byte{'\n'}) {
		if _, err := fmt.Fprintln(w); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

func writeFile(path string, data []byte, forceOverwrite bool) error {
	if err := checkOverwrite(path, forceOverwrite); err != nil {
		return trace.Wrap(err)
	}
	if err := ioutil.WriteFile(path, data, writeFileMode); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func checkOverwrite(path string, force bool) error {
	// Check if destination file exists.
	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		// File doesn't exist, proceed.
		return nil
	}
	if err != nil {
		// Something else went wrong, fail.
		return trace.Wrap(err)
	}
	if force {
		// File exists but we're asked not to prompt, proceed.
		return nil
	}

	// File exists, prompt user whether to overwrite.
	fmt.Fprintf(os.Stderr, "Destination file %q exists. Overwrite it? [y/N]: ", path)
	scan := bufio.NewScanner(os.Stdin)
	if !scan.Scan() {
		return trace.WrapWithMessage(scan.Err(), "failed reading prompt response")
	}
	if strings.ToLower(strings.TrimSpace(scan.Text())) == "y" {
		return nil
	}
	return trace.Errorf("NOT overwriting destination file %q", path)
}

// IdentityFile represents the basic components of an identity file.
type IdentityFile struct {
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

// Decode attempts to break up the contents of an identity file
// into its respective components.
func Decode(r io.Reader) (*IdentityFile, error) {
	scanner := bufio.NewScanner(r)
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
