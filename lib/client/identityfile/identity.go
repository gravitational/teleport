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
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"

	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/kube/kubeconfig"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/utils/prompt"

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

// KnownFormats is a list of all above formats.
var KnownFormats = []Format{FormatFile, FormatOpenSSH, FormatTLS, FormatKubernetes, FormatDatabase}

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
		buf := new(bytes.Buffer)
		// write key:
		if err := writeWithNewline(buf, cfg.Key.Priv); err != nil {
			return nil, trace.Wrap(err)
		}
		// append ssh cert:
		if err := writeWithNewline(buf, cfg.Key.Cert); err != nil {
			return nil, trace.Wrap(err)
		}
		// append tls cert:
		if err := writeWithNewline(buf, cfg.Key.TLSCert); err != nil {
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
				if err := writeWithNewline(buf, []byte(data)); err != nil {
					return nil, trace.Wrap(err)
				}
			}
			// append tls ca certificates
			for _, cert := range ca.TLSCertificates {
				if err := writeWithNewline(buf, cert); err != nil {
					return nil, trace.Wrap(err)
				}
			}
		}

		filesWritten = append(filesWritten, cfg.OutputPath)
		if err := checkOverwrite(cfg.OverwriteDestination, filesWritten...); err != nil {
			return nil, trace.Wrap(err)
		}
		if err := writeFile(cfg.OutputPath, buf.Bytes()); err != nil {
			return nil, trace.Wrap(err)
		}

	// dump user identity into separate files:
	case FormatOpenSSH:
		keyPath := cfg.OutputPath
		certPath := keyPath + "-cert.pub"
		filesWritten = append(filesWritten, keyPath, certPath)
		if err := checkOverwrite(cfg.OverwriteDestination, filesWritten...); err != nil {
			return nil, trace.Wrap(err)
		}

		err = writeFile(certPath, cfg.Key.Cert)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		err = writeFile(keyPath, cfg.Key.Priv)
		if err != nil {
			return nil, trace.Wrap(err)
		}

	case FormatTLS, FormatDatabase:
		keyPath := cfg.OutputPath + ".key"
		certPath := cfg.OutputPath + ".crt"
		casPath := cfg.OutputPath + ".cas"
		filesWritten = append(filesWritten, keyPath, certPath, casPath)
		if err := checkOverwrite(cfg.OverwriteDestination, filesWritten...); err != nil {
			return nil, trace.Wrap(err)
		}

		err = writeFile(certPath, cfg.Key.TLSCert)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		err = writeFile(keyPath, cfg.Key.Priv)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		var caCerts []byte
		for _, ca := range cfg.Key.TrustedCA {
			for _, cert := range ca.TLSCertificates {
				caCerts = append(caCerts, cert...)
			}
		}
		err = writeFile(casPath, caCerts)
		if err != nil {
			return nil, trace.Wrap(err)
		}

	case FormatKubernetes:
		filesWritten = append(filesWritten, cfg.OutputPath)
		if err := checkOverwrite(cfg.OverwriteDestination, filesWritten...); err != nil {
			return nil, trace.Wrap(err)
		}
		// Clean up the existing file, if it exists.
		//
		// kubeconfig.Update would try to parse it and merge in new
		// credentials, which is not what we want.
		if err := os.Remove(cfg.OutputPath); err != nil && !os.IsNotExist(err) {
			return nil, trace.Wrap(err)
		}

		if err := kubeconfig.Update(cfg.OutputPath, kubeconfig.Values{
			TeleportClusterName: cfg.Key.ClusterName,
			ClusterAddr:         cfg.KubeProxyAddr,
			Credentials:         cfg.Key,
		}); err != nil {
			return nil, trace.Wrap(err)
		}

	default:
		return nil, trace.BadParameter("unsupported identity format: %q, use one of %q", cfg.Format, KnownFormats)
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

func writeFile(path string, data []byte) error {
	return trace.Wrap(ioutil.WriteFile(path, data, writeFileMode))
}

func checkOverwrite(force bool, paths ...string) error {
	var existingFiles []string
	// Check if the destination file exists.
	for _, path := range paths {
		_, err := os.Stat(path)
		if os.IsNotExist(err) {
			// File doesn't exist, proceed.
			continue
		}
		if err != nil {
			// Something else went wrong, fail.
			return trace.ConvertSystemError(err)
		}
		existingFiles = append(existingFiles, path)
	}
	if len(existingFiles) == 0 || force {
		// Files don't exist or we're asked not to prompt, proceed.
		return nil
	}

	// Some files exist, prompt user whether to overwrite.
	overwrite, err := prompt.Confirmation(os.Stderr, os.Stdin, fmt.Sprintf("Destination file(s) %s exist. Overwrite?", strings.Join(existingFiles, ", ")))
	if err != nil {
		return trace.Wrap(err)
	}
	if !overwrite {
		return trace.Errorf("not overwriting destination files %s", strings.Join(existingFiles, ", "))
	}
	return nil
}
