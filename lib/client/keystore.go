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
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/utils"

	log "github.com/Sirupsen/logrus"
	"github.com/gravitational/trace"

	"golang.org/x/crypto/ssh"
)

const (
	defaultKeyDir      = ProfileDir
	fileExtCert        = "-cert.pub"
	fileExtPub         = ".pub"
	sessionKeyDir      = "keys"
	fileNameKnownHosts = "known_hosts"

	// profileDirPerms is the default permissions applied to the profile
	// directory (usually ~/.tsh)
	profileDirPerms os.FileMode = 0700

	// keyFilePerms is the default permissions applied to key files (.cert, .key, pub)
	// under ~/.tsh
	keyFilePerms os.FileMode = 0600
)

// LocalKeyStore interface allows for different storage back-ends for TSH to
// load/save its keys
//
// The _only_ filesystem-based implementation of LocalKeyStore is declared
// below (FSLocalKeyStore)
type LocalKeyStore interface {
	// client key management
	GetKeys(username string) ([]Key, error)
	AddKey(host string, username string, key *Key) error
	GetKey(host string, username string) (*Key, error)
	DeleteKey(host string, username string) error

	// interface to known_hosts file:
	AddKnownHostKeys(hostname string, keys []ssh.PublicKey) error
	GetKnownHostKeys(hostname string) ([]ssh.PublicKey, error)
}

// FSLocalKeyStore implements LocalKeyStore interface using the filesystem
// Here's the file layout for the FS store:
// ~/.tsh/
// ├── known_hosts   --> trusted certificate authorities (their keys) in a format similar to known_hosts
// └── sessions      --> server-signed session keys
//     └── host-a
//     |   ├── cert
//     |   ├── key
//     |   └── pub
//     └── host-b
//         ├── cert
//         ├── key
//         └── pub
type FSLocalKeyStore struct {
	LocalKeyStore

	// KeyDir is the directory where all keys are stored
	KeyDir string
}

// NewFSLocalKeyStore creates a new filesystem-based local keystore object
// and initializes it.
//
// if dirPath is empty, sets it to ~/.tsh
func NewFSLocalKeyStore(dirPath string) (s *FSLocalKeyStore, err error) {
	dirPath, err = initKeysDir(dirPath)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &FSLocalKeyStore{
		KeyDir: dirPath,
	}, nil
}

// GetKeys returns all user session keys stored in the store
func (fs *FSLocalKeyStore) GetKeys(username string) (keys []Key, err error) {
	dirPath := filepath.Join(fs.KeyDir, sessionKeyDir)
	if !utils.IsDir(dirPath) {
		return make([]Key, 0), nil
	}
	dirEntries, err := ioutil.ReadDir(dirPath)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for _, fi := range dirEntries {
		if !fi.IsDir() {
			continue
		}
		k, err := fs.GetKey(fi.Name(), username)
		if err != nil {
			// if a key is reported as 'not found' it's probably because it expired
			if !trace.IsNotFound(err) {
				return nil, trace.Wrap(err)
			}
			continue
		}
		keys = append(keys, *k)
	}
	return keys, nil
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
			log.Error(err)
		}
		return err
	}
	if err = writeBytes(username+fileExtCert, key.Cert); err != nil {
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

// GetKey returns a key for a given host. If the key is not found,
// returns trace.NotFound error.
func (fs *FSLocalKeyStore) GetKey(host, username string) (*Key, error) {
	dirPath, err := fs.dirFor(host)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	certFile := filepath.Join(dirPath, username+fileExtCert)
	cert, err := ioutil.ReadFile(certFile)
	if err != nil {
		log.Error(err)
		return nil, trace.Wrap(err)
	}
	pub, err := ioutil.ReadFile(filepath.Join(dirPath, username+fileExtPub))
	if err != nil {
		log.Error(err)
		return nil, trace.Wrap(err)
	}
	priv, err := ioutil.ReadFile(filepath.Join(dirPath, username))
	if err != nil {
		log.Error(err)
		return nil, trace.Wrap(err)
	}

	key := &Key{Pub: pub, Priv: priv, Cert: cert, ProxyHost: host}

	// expired certificate? this key won't be accepted anymore, lets delete it:
	certExpiration, err := key.CertValidBefore()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	log.Debugf("[KEYSTORE] Returning certificate %q valid until %q", certFile, certExpiration)
	if certExpiration.Before(time.Now()) {
		log.Infof("[KEYSTORE] TTL expired (%v) for session key %v", certExpiration, dirPath)
		os.RemoveAll(dirPath)
		return nil, trace.NotFound("session keys for %s are not found", host)
	}
	return key, nil
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
		log.Debugf("adding known host %s with key: %v", hostname, sshutils.Fingerprint(hostKeys[i]))
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
// for a given host are stored
func (fs *FSLocalKeyStore) dirFor(hostname string) (string, error) {
	dirPath := filepath.Join(fs.KeyDir, sessionKeyDir, hostname)
	if err := os.MkdirAll(dirPath, profileDirPerms); err != nil {
		log.Error(err)
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
