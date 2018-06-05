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
	"bufio"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/sshutils"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

const (
	defaultKeyDir      = ProfileDir
	fileExtTLSCert     = "-x509.pem"
	fileExtCert        = "-cert.pub"
	fileExtPub         = ".pub"
	sessionKeyDir      = "keys"
	fileNameKnownHosts = "known_hosts"
	fileNameTLSCerts   = "certs.pem"

	// profileDirPerms is the default permissions applied to the profile
	// directory (usually ~/.tsh)
	profileDirPerms os.FileMode = 0700

	// keyFilePerms is the default permissions applied to key files (.cert, .key, pub)
	// under ~/.tsh
	keyFilePerms os.FileMode = 0600
)

// LocalKeyStore interface allows for different storage backends for tsh to
// load/save its keys.
//
// The _only_ filesystem-based implementation of LocalKeyStore is declared
// below (FSLocalKeyStore)
type LocalKeyStore interface {
	// AddKey adds the given session key for the proxy and username to the
	// storage backend.
	AddKey(proxy string, username string, key *Key) error

	// GetKey returns the session key for the given username and proxy.
	GetKey(proxy string, username string) (*Key, error)

	// DeleteKey removes a specific session key from a proxy.
	DeleteKey(proxyHost string, username string) error

	// DeleteKeys removes all session keys from disk.
	DeleteKeys() error

	// AddKnownHostKeys adds the public key to the list of known hosts for
	// a hostname.
	AddKnownHostKeys(hostname string, keys []ssh.PublicKey) error

	// GetKnownHostKeys returns all public keys for a hostname.
	GetKnownHostKeys(hostname string) ([]ssh.PublicKey, error)

	// SaveCerts saves trusted TLS certificates of certificate authorities.
	SaveCerts(proxy string, cas []auth.TrustedCerts) error

	// GetCerts gets trusted TLS certificates of certificate authorities.
	GetCerts(proxy string) (*x509.CertPool, error)

	// GetCertsPEM gets trusted TLS certificates of certificate authorities.
	GetCertsPEM(proxy string) ([]byte, error)
}

// FSLocalKeyStore implements LocalKeyStore interface using the filesystem.
// Here's the file layout for the FS store:
//
// ~/.tsh/
// ├── known_hosts             --> trusted certificate authorities (their keys) in a format similar to known_hosts
// └── keys
//    ├── one.example.com
//    │   ├── certs.pem
//    │   ├── foo              --> RSA Private Key
//    │   ├── foo-cert.pub     --> SSH certificate for proxies and nodes
//    │   ├── foo.pub          --> Public Key
//    │   └── foo-x509.pem     --> TLS client certificate for Auth Server
//    └── two.example.com
//        ├── certs.pem
//        ├── bar
//        ├── bar-cert.pub
//        ├── bar.pub
//        └── bar-x509.pem
type FSLocalKeyStore struct {
	// log holds the structured logger.
	log *logrus.Entry

	// KeyDir is the directory where all keys are stored.
	KeyDir string
}

// NewFSLocalKeyStore creates a new filesystem-based local keystore object
// and initializes it.
//
// If dirPath is empty, sets it to ~/.tsh.
func NewFSLocalKeyStore(dirPath string) (s *FSLocalKeyStore, err error) {
	dirPath, err = initKeysDir(dirPath)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &FSLocalKeyStore{
		log: logrus.WithFields(logrus.Fields{
			trace.Component: teleport.ComponentKeyStore,
		}),
		KeyDir: dirPath,
	}, nil
}

// AddKey adds a new key to the session store. If a key for the host is already
// stored, overwrites it.
func (fs *FSLocalKeyStore) AddKey(host, username string, key *Key) error {
	dirPath, err := fs.dirFor(host)
	if err != nil {
		return trace.Wrap(err)
	}
	writeBytes := func(fname string, data []byte) error {
		fp := filepath.Join(dirPath, fname)
		err := ioutil.WriteFile(fp, data, keyFilePerms)
		if err != nil {
			fs.log.Error(err)
		}
		return err
	}
	if err = writeBytes(username+fileExtCert, key.Cert); err != nil {
		return trace.Wrap(err)
	}
	if err = writeBytes(username+fileExtTLSCert, key.TLSCert); err != nil {
		return trace.Wrap(err)
	}
	if err = writeBytes(username+fileExtPub, key.Pub); err != nil {
		return trace.Wrap(err)
	}
	if err = writeBytes(username, key.Priv); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// DeleteKey deletes a key from the local store
func (fs *FSLocalKeyStore) DeleteKey(host string, username string) error {
	dirPath, err := fs.dirFor(host)
	if err != nil {
		return trace.Wrap(err)
	}
	files := []string{
		filepath.Join(dirPath, username+fileExtCert),
		filepath.Join(dirPath, username+fileExtTLSCert),
		filepath.Join(dirPath, username+fileExtPub),
		filepath.Join(dirPath, username),
	}
	for _, fn := range files {
		if err = os.Remove(fn); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// DeleteKeys removes all session keys from disk.
func (fs *FSLocalKeyStore) DeleteKeys() error {
	dirPath := filepath.Join(fs.KeyDir, sessionKeyDir)

	err := os.RemoveAll(dirPath)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// GetKey returns a key for a given host. If the key is not found,
// returns trace.NotFound error.
func (fs *FSLocalKeyStore) GetKey(proxyHost string, username string) (*Key, error) {
	dirPath, err := fs.dirFor(proxyHost)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	_, err = ioutil.ReadDir(dirPath)
	if err != nil {
		return nil, trace.NotFound("no session keys for %v in %v", username, proxyHost)
	}

	certFile := filepath.Join(dirPath, username+fileExtCert)
	cert, err := ioutil.ReadFile(certFile)
	if err != nil {
		fs.log.Error(err)
		return nil, trace.Wrap(err)
	}
	tlsCertFile := filepath.Join(dirPath, username+fileExtTLSCert)
	tlsCert, err := ioutil.ReadFile(tlsCertFile)
	if err != nil {
		fs.log.Error(err)
		return nil, trace.Wrap(err)
	}
	pub, err := ioutil.ReadFile(filepath.Join(dirPath, username+fileExtPub))
	if err != nil {
		fs.log.Error(err)
		return nil, trace.Wrap(err)
	}
	priv, err := ioutil.ReadFile(filepath.Join(dirPath, username))
	if err != nil {
		fs.log.Error(err)
		return nil, trace.Wrap(err)
	}

	key := &Key{Pub: pub, Priv: priv, Cert: cert, ProxyHost: proxyHost, TLSCert: tlsCert}

	certExpiration, err := key.CertValidBefore()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tlsCertExpiration, err := key.TLSCertValidBefore()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// TODO(russjones): Note, we may be returning expired certificates here, that
	// is okay. If the certificates is expired, it's the responsibility of the
	// TeleportClient to perform cleanup of the certificates and the profile.
	fs.log.Debugf("Returning SSH certificate %q valid until %q, TLS certificate %q valid until %q",
		certFile, certExpiration, tlsCertFile, tlsCertExpiration)

	return key, nil
}

// SaveCerts saves trusted TLS certificates of certificate authorities
func (fs *FSLocalKeyStore) SaveCerts(proxy string, cas []auth.TrustedCerts) error {
	dir, err := fs.dirFor(proxy)
	if err != nil {
		return trace.Wrap(err)
	}
	fp, err := os.OpenFile(filepath.Join(dir, fileNameTLSCerts), os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0640)
	if err != nil {
		return trace.Wrap(err)
	}
	defer fp.Sync()
	defer fp.Close()
	for _, ca := range cas {
		for _, cert := range ca.TLSCertificates {
			_, err := fp.Write(cert)
			if err != nil {
				return trace.ConvertSystemError(err)
			}
			_, err = fp.WriteString("\n")
			if err != nil {
				return trace.ConvertSystemError(err)
			}
		}
	}
	return nil
}

// GetCertsPEM returns trusted TLS certificates of certificate authorities PEM block
func (fs *FSLocalKeyStore) GetCertsPEM(proxy string) ([]byte, error) {
	dir, err := fs.dirFor(proxy)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return ioutil.ReadFile(filepath.Join(dir, fileNameTLSCerts))
}

// GetCerts returns trusted TLS certificates of certificate authorities
func (fs *FSLocalKeyStore) GetCerts(proxy string) (*x509.CertPool, error) {
	dir, err := fs.dirFor(proxy)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	bytes, err := ioutil.ReadFile(filepath.Join(dir, fileNameTLSCerts))
	if err != nil {
		return nil, trace.ConvertSystemError(err)
	}
	pool := x509.NewCertPool()
	for len(bytes) > 0 {
		var block *pem.Block
		block, bytes = pem.Decode(bytes)
		if block == nil {
			break
		}
		if block.Type != "CERTIFICATE" || len(block.Headers) != 0 {
			fs.log.Debugf("Skipping PEM block type=%v headers=%v.", block.Type, block.Headers)
			continue
		}

		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, trace.BadParameter("failed to parse certificate: %v", err)
		}
		fs.log.Debugf("Adding trusted cluster certificate authority %q to trusted pool.", cert.Issuer)
		pool.AddCert(cert)
	}
	return pool, nil
}

// AddKnownHostKeys adds a new entry to 'known_hosts' file
func (fs *FSLocalKeyStore) AddKnownHostKeys(hostname string, hostKeys []ssh.PublicKey) error {
	fp, err := os.OpenFile(filepath.Join(fs.KeyDir, fileNameKnownHosts), os.O_CREATE|os.O_RDWR, 0640)
	if err != nil {
		return trace.Wrap(err)
	}
	defer fp.Sync()
	defer fp.Close()
	// read all existing entries into a map (this removes any pre-existing dupes)
	entries := make(map[string]int)
	output := make([]string, 0)
	scanner := bufio.NewScanner(fp)
	for scanner.Scan() {
		line := scanner.Text()
		if _, exists := entries[line]; !exists {
			output = append(output, line)
			entries[line] = 1
		}
	}
	// add every host key to the list of entries
	for i := range hostKeys {
		fs.log.Debugf("Adding known host %s with key: %v", hostname, sshutils.Fingerprint(hostKeys[i]))
		bytes := ssh.MarshalAuthorizedKey(hostKeys[i])
		line := strings.TrimSpace(fmt.Sprintf("%s %s", hostname, bytes))
		if _, exists := entries[line]; !exists {
			output = append(output, line)
		}
	}
	// re-create the file:
	_, err = fp.Seek(0, 0)
	if err != nil {
		return trace.Wrap(err)
	}
	if err = fp.Truncate(0); err != nil {
		return trace.Wrap(err)
	}
	for _, line := range output {
		fmt.Fprintf(fp, "%s\n", line)
	}
	return nil
}

// GetKnownHostKeys returns all known public keys from 'known_hosts'
func (fs *FSLocalKeyStore) GetKnownHostKeys(hostname string) ([]ssh.PublicKey, error) {
	bytes, err := ioutil.ReadFile(filepath.Join(fs.KeyDir, fileNameKnownHosts))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, trace.Wrap(err)
	}
	var (
		pubKey    ssh.PublicKey
		retval    []ssh.PublicKey = make([]ssh.PublicKey, 0)
		hosts     []string
		hostMatch bool
	)
	for err == nil {
		_, hosts, pubKey, _, bytes, err = ssh.ParseKnownHosts(bytes)
		if err == nil {
			hostMatch = (hostname == "")
			if !hostMatch {
				for i := range hosts {
					if hosts[i] == hostname {
						hostMatch = true
						break
					}
				}
			}
			if hostMatch {
				retval = append(retval, pubKey)
			}
		}
	}
	if err != io.EOF {
		return nil, trace.Wrap(err)
	}
	return retval, nil
}

// dirFor is a helper function. It returns a directory where session keys
// for a given host are stored. fs.KeyDir is typically "~/.tsh", sessionKeyDir
// is typically "keys", and proxyHost is typically something like
// "proxy.example.com".
func (fs *FSLocalKeyStore) dirFor(proxyHost string) (string, error) {
	dirPath := filepath.Join(fs.KeyDir, sessionKeyDir, proxyHost)
	if err := os.MkdirAll(dirPath, profileDirPerms); err != nil {
		fs.log.Error(err)
		return "", trace.Wrap(err)
	}
	return dirPath, nil
}

// initKeysDir initializes the keystore root directory. Usually it is ~/.tsh
func initKeysDir(dirPath string) (string, error) {
	var err error
	// not specified? use `~/.tsh`
	if dirPath == "" {
		u, err := user.Current()
		if err != nil {
			dirPath = os.TempDir()
		} else {
			dirPath = u.HomeDir
		}
		dirPath = filepath.Join(dirPath, defaultKeyDir)
	}
	// create if doesn't exist:
	_, err = os.Stat(dirPath)
	if err != nil {
		if os.IsNotExist(err) {
			err = os.MkdirAll(dirPath, os.ModeDir|profileDirPerms)
			if err != nil {
				return "", trace.Wrap(err)
			}
		} else {
			return "", trace.Wrap(err)
		}
	}

	return dirPath, nil
}
