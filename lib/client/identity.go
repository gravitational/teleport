package client

import (
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/gravitational/teleport/lib/auth/native"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
)

// MakeNewKey generates a new unsigned key. Such key must be signed by a
// Teleport CA (auth server) before it becomes useful.
func MakeNewKey() (key *Key, err error) {
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

	// IdentityFormatDir is OpenSSH-compatible format, when a key and a cert are stored in
	// two different files (in the same directory)
	IdentityFormatDir IdentityFileFormat = "dir"

	// DefaultIdentityFormat is what Teleport uses by default
	DefaultIdentityFormat = IdentityFormatFile
)

// MakeIdentityFile takes a username + his credentials and saves them to disk
// in a specified format
func MakeIdentityFile(username, fp string, key *Key, format IdentityFileFormat) (err error) {
	const (
		// the files and the dir will be created with these permissions:
		fileMode = 0600
		dirMode  = 0770
	)
	var output io.Writer = os.Stdout
	switch format {
	// dump user identity into a single file:
	case IdentityFormatFile:
		if fp != "" {
			f, err := os.OpenFile(fp, os.O_CREATE|os.O_WRONLY, fileMode)
			if err != nil {
				return trace.Wrap(err)
			}
			output = f
			defer f.Close()
		}
		// write key:
		if _, err = output.Write(key.Priv); err != nil {
			return trace.Wrap(err)
		}
		// append cert:
		if _, err = output.Write(key.Cert); err != nil {
			return trace.Wrap(err)
		}
	// dump user identity into separate files:
	case IdentityFormatDir:
		certPath := username + "-cert.pub"
		keyPath := username

		// --out flag
		if fp != "" {
			if !utils.IsDir(fp) {
				if err = os.MkdirAll(fp, dirMode); err != nil {
					return trace.Wrap(err)
				}
			}
			certPath = filepath.Join(fp, certPath)
			keyPath = filepath.Join(fp, keyPath)
		}

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
