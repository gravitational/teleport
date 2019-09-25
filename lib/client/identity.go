/*
Copyright 2016 Gravitational, Inc.

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

package client

import (
	"io"
	"io/ioutil"
	"os"

	"github.com/gravitational/teleport/lib/auth/native"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/sshutils"

	"github.com/gravitational/trace"
)

// NewKey generates a new unsigned key. Such key must be signed by a
// Teleport CA (auth server) before it becomes useful.
func NewKey() (key *Key, err error) {
	priv, pub, err := native.GenerateKeyPair("")
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &Key{
		Priv: priv,
		Pub:  pub,
	}, nil
}

// IdentityFileFormat describes possible file formats how a user identity can be sotred
type IdentityFileFormat string

const (
	// IdentityFormatFile is when a key + cert are stored concatenated into a single file
	IdentityFormatFile IdentityFileFormat = "file"

	// IdentityFormatOpenSSH is OpenSSH-compatible format, when a key and a cert are stored in
	// two different files (in the same directory)
	IdentityFormatOpenSSH IdentityFileFormat = "openssh"

	// DefaultIdentityFormat is what Teleport uses by default
	DefaultIdentityFormat = IdentityFormatFile
)

// MakeIdentityFile takes a username + their credentials and saves them to disk
// in a specified format
func MakeIdentityFile(filePath string, key *Key, format IdentityFileFormat, certAuthorities []services.CertAuthority) (err error) {
	const (
		// the files and the dir will be created with these permissions:
		fileMode = 0600
		dirMode  = 0700
	)

	if filePath == "" {
		return trace.BadParameter("identity location is not specified")
	}

	var output io.Writer = os.Stdout
	switch format {
	// dump user identity into a single file:
	case IdentityFormatFile:
		f, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, fileMode)
		if err != nil {
			return trace.Wrap(err)
		}
		output = f
		defer f.Close()

		// write key:
		if _, err = output.Write(key.Priv); err != nil {
			return trace.Wrap(err)
		}
		// append ssh cert:
		if _, err = output.Write(key.Cert); err != nil {
			return trace.Wrap(err)
		}
		// append tls cert:
		if _, err = output.Write(key.TLSCert); err != nil {
			return trace.Wrap(err)
		}
		// append trusted host certificate authorities
		for _, ca := range certAuthorities {
			for _, publicKey := range ca.GetCheckingKeys() {
				// append ssh ca certificates
				data, err := sshutils.MarshalAuthorizedHostsFormat(ca.GetClusterName(), publicKey, nil)
				if err != nil {
					return trace.Wrap(err)
				}
				if _, err = output.Write([]byte(data)); err != nil {
					return trace.Wrap(err)
				}
				if _, err = output.Write([]byte("\n")); err != nil {
					return trace.Wrap(err)
				}
				// append tls ca certificates
				for _, keyPair := range ca.GetTLSKeyPairs() {
					if _, err = output.Write(keyPair.Cert); err != nil {
						return trace.Wrap(err)
					}
				}
			}
		}

	// dump user identity into separate files:
	case IdentityFormatOpenSSH:
		keyPath := filePath
		certPath := keyPath + "-cert.pub"

		err = ioutil.WriteFile(certPath, key.Cert, fileMode)
		if err != nil {
			return trace.Wrap(err)
		}

		err = ioutil.WriteFile(keyPath, key.Priv, fileMode)
		if err != nil {
			return trace.Wrap(err)
		}
	default:
		return trace.BadParameter("unsupported identity format: %q, use either %q or %q",
			format, IdentityFormatFile, IdentityFormatOpenSSH)
	}
	return nil
}
