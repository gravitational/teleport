/*
Copyright 2015 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.


Keystore implements functions for saving and loading from hard disc
temporary teleport certificates
*/

package client

import (
	"encoding/json"
	"io/ioutil"
	"math/rand"
	"net"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gravitational/teleport/lib/backend/boltbk"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/sshutils"

	log "github.com/Sirupsen/logrus"
	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

type LocalKeyAgent struct {
	agent.Agent
	// KeyDir is the directory path where local keys are stored
	KeyDir string
}

// AddHostSignersToCache takes a list of CAs whom we trust. This list is added to a database
// of "seen" CAs.
//
// Every time we connect to a new host, we'll request its certificaate to be signed by one
// of these trusted CAs.
//
// Why do we trust these CAs? Because we received them from a trusted Teleport Proxy.
// Why do we trust the proxy? Because we've connected to it via HTTPS + username + Password + HOTP.
func (a *LocalKeyAgent) AddHostSignersToCache(hostSigners []services.CertAuthority) error {
	bk, err := boltbk.New(filepath.Join(a.KeyDir, HostSignersFilename))
	if err != nil {
		return trace.Wrap(nil)
	}
	defer bk.Close()
	ca := local.NewCAService(bk)

	for _, hostSigner := range hostSigners {
		err := ca.UpsertCertAuthority(hostSigner, 0)
		if err != nil {
			return trace.Wrap(nil)
		}
	}
	return nil
}

// CheckHostSignature checks if the given host key was signed by one of the trusted
// certificaate authorities (CAs)
func (a *LocalKeyAgent) CheckHostSignature(hostId string, remote net.Addr, key ssh.PublicKey) error {
	cert, ok := key.(*ssh.Certificate)
	if !ok {
		return trace.Errorf("expected certificate")
	}

	bk, err := boltbk.New(filepath.Join(a.KeyDir, HostSignersFilename))
	if err != nil {
		return trace.Wrap(nil)
	}
	defer bk.Close()
	ca := local.NewCAService(bk)

	cas, err := ca.GetCertAuthorities(services.HostCA, false)
	if err != nil {
		return trace.Wrap(err)
	}

	for i := range cas {
		checkers, err := cas[i].Checkers()
		if err != nil {
			return trace.Wrap(err)
		}
		for _, checker := range checkers {
			if sshutils.KeysEqual(cert.SignatureKey, checker) {
				return nil
			}
		}
	}
	return trace.Errorf("no matching authority found")
}

// getKeyes returns a list of local keys agents can use to authenticate
func (a *LocalKeyAgent) GetKeys() ([]agent.AddedKey, error) {
	existingKeys, err := a.loadAllKeys()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	addedKeys := make([]agent.AddedKey, len(existingKeys))
	for i, key := range existingKeys {
		pcert, _, _, _, err := ssh.ParseAuthorizedKey(key.Cert)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		pk, err := ssh.ParseRawPrivateKey(key.Priv)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		addedKey := agent.AddedKey{
			PrivateKey:       pk,
			Certificate:      pcert.(*ssh.Certificate),
			Comment:          "",
			LifetimeSecs:     0,
			ConfirmBeforeUse: false,
		}
		addedKeys[i] = addedKey
	}
	return addedKeys, nil
}

// GetLocalAgent loads all the saved teleport certificates and
// creates ssh agent with them
func GetLocalAgent(keyDir string) (a *LocalKeyAgent, err error) {
	if keyDir == "" {
		keyDir, err = initDefaultKeysDir()
		if err != nil {
			return nil, err
		}
	} else {
		if !isDir(keyDir) {
			return nil, trace.Errorf("Cannot store keys. %s is not a directory", keyDir)
		}
	}
	a = &LocalKeyAgent{
		Agent:  agent.NewKeyring(),
		KeyDir: keyDir,
	}
	keys, err := a.GetKeys()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for _, key := range keys {
		if err := a.Add(key); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return a, nil
}

// initDefaultKeysDir initializes `~/.tsh` directory
func initDefaultKeysDir() (dirPath string, err error) {
	// construct `~/.tsh` path name:
	u, err := user.Current()
	if err != nil {
		dirPath = os.TempDir()
	} else {
		dirPath = u.HomeDir
	}
	dirPath = filepath.Join(dirPath, ".tsh")

	// need to create it?
	_, err = os.Stat(dirPath)
	if os.IsNotExist(err) {
		err = os.MkdirAll(dirPath, os.ModeDir|0777)
		if err != nil {
			return "", trace.Wrap(err)
		}
	} else {
		if err != nil {
			return "", trace.Wrap(err)
		}
	}
	return dirPath, nil
}

// Key describes a key on disk
type Key struct {
	Priv     []byte    `json:"Priv,omitempty"`
	Pub      []byte    `json:"Pub,omitempty"`
	Cert     []byte    `json:"Cert,omitempty"`
	Deadline time.Time `json:"Deadline,omitempty"`
}

func (a *LocalKeyAgent) saveKey(key *Key) error {
	filename := filepath.Join(a.KeyDir, KeyFilePrefix+strconv.FormatInt(rand.Int63n(100), 16)+KeyFileSuffix)
	bytes, err := json.Marshal(key)
	if err != nil {
		return trace.Wrap(err)
	}
	return ioutil.WriteFile(filename, bytes, 0666)
}

func loadKey(filename string) (Key, error) {
	bytes, err := ioutil.ReadFile(filename)
	if err != nil {
		return Key{}, trace.Wrap(err)
	}

	var key Key

	err = json.Unmarshal(bytes, &key)
	if err != nil {
		return Key{}, trace.Wrap(err)
	}

	return key, nil

}

func (a *LocalKeyAgent) loadAllKeys() ([]Key, error) {
	keys := make([]Key, 0)
	files, err := ioutil.ReadDir(a.KeyDir)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for _, file := range files {
		fn := file.Name()
		if !file.IsDir() && strings.HasPrefix(fn, KeyFilePrefix) && strings.HasSuffix(fn, KeyFileSuffix) {
			fp := filepath.Join(a.KeyDir, file.Name())
			key, err := loadKey(fp)
			if err != nil {
				log.Errorf(err.Error())
				continue
			}

			if time.Now().Before(key.Deadline) {
				keys = append(keys, key)
			} else {
				// remove old keys
				err = os.Remove(fp)
				if err != nil {
					log.Errorf(err.Error())
				}
			}
		}
	}
	return keys, nil
}

var (
	KeyFilePrefix       = "teleport_"
	KeyFileSuffix       = ".tkey"
	HostSignersFilename = "hostsigners.db"
)

func isDir(dirPath string) bool {
	fi, err := os.Stat(dirPath)
	if err == nil {
		return fi.IsDir()
	}
	return false
}
