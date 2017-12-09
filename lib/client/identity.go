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
	"github.com/gravitational/trace"
)

// NewKey generates a new unsigned key. Such key must be signed by a
// Teleport CA (auth server) before it becomes useful.
func NewKey() (key *Key, err error) {
	key = &Key{}
	keygen := native.New()
	defer keygen.Close()
	key.Priv, key.Pub, err = keygen.GenerateKeyPair("")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return key, nil
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

// MakeIdentityFile takes a username + his credentials and saves them to disk
// in a specified format
func MakeIdentityFile(filePath string, key *Key, format IdentityFileFormat) (err error) {
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
		f, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY, fileMode)
		if err != nil {
			return trace.Wrap(err)
		}
		output = f
		defer f.Close()

		// write key:
		if _, err = output.Write(key.Priv); err != nil {
			return trace.Wrap(err)
		}
		// append cert:
		if _, err = output.Write(key.Cert); err != nil {
			return trace.Wrap(err)
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
	}
	return nil
}
