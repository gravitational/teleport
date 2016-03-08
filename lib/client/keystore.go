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
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	"github.com/gravitational/teleport/lib/backend/boltbk"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/sshutils"

	log "github.com/Sirupsen/logrus"
	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

func AddHostSignersToCache(hostSigners []services.CertAuthority) error {
	bk, err := boltbk.New(filepath.Join(getKeysDir(), HostSignersFilename))
	if err != nil {
		return trace.Wrap(nil)
	}
	defer bk.Close()
	ca := services.NewCAService(bk)

	for _, hostSigner := range hostSigners {
		err := ca.UpsertCertAuthority(hostSigner, 0)
		if err != nil {
			return trace.Wrap(nil)
		}
	}
	return nil
}

func CheckHostSignerFromCache(hostId string, remote net.Addr, key ssh.PublicKey) error {
	cert, ok := key.(*ssh.Certificate)
	if !ok {
		return trace.Errorf("expected certificate")
	}

	bk, err := boltbk.New(filepath.Join(getKeysDir(), HostSignersFilename))
	if err != nil {
		return trace.Wrap(nil)
	}
	defer bk.Close()
	ca := services.NewCAService(bk)

	cas, err := ca.GetCertAuthorities(services.HostCA)
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

// GetLoadAgent loads all the saved teleport certificates and
// creates ssh agent with them
func GetLocalAgent() (agent.Agent, error) {
	err := initKeysDir()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ag := agent.NewKeyring()
	existingKeys, err := loadAllKeys()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for _, key := range existingKeys {
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
		if err := ag.Add(addedKey); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return ag, nil
}

func initKeysDir() error {
	_, err := os.Stat(getKeysDir())
	if os.IsNotExist(err) {
		err = os.MkdirAll(getKeysDir(), os.ModeDir|0777)
		if err != nil {
			return trace.Wrap(err)
		}
	} else {
		if err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

type Key struct {
	Priv     []byte
	Cert     []byte
	Deadline time.Time
}

func saveKey(key Key, filename string) error {
	err := initKeysDir()
	if err != nil {
		return trace.Wrap(err)
	}
	bytes, err := json.Marshal(key)
	if err != nil {
		return trace.Wrap(err)
	}

	err = ioutil.WriteFile(filename, bytes, 0666)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
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

func loadAllKeys() ([]Key, error) {
	keys := make([]Key, 0)
	files, err := ioutil.ReadDir(getKeysDir())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for _, file := range files {
		if !file.IsDir() && strings.HasPrefix(file.Name(), KeyFilePrefix) &&
			strings.HasSuffix(file.Name(), KeyFileSuffix) {
			key, err := loadKey(filepath.Join(getKeysDir(), file.Name()))
			if err != nil {
				log.Errorf(err.Error())
				continue
			}

			if time.Now().Before(key.Deadline) {
				keys = append(keys, key)
			} else {
				// remove old keys
				err = os.Remove(filepath.Join(getKeysDir(), file.Name()))
				if err != nil {
					log.Errorf(err.Error())
				}
			}
		}
	}
	return keys, nil
}

// getKeysDir() returns the directory where a client can store the temporary keys
func getKeysDir() string {
	var baseDir string
	u, err := user.Current()
	if err != nil {
		baseDir = os.TempDir()
	} else {
		baseDir = u.HomeDir
	}
	return filepath.Join(baseDir, ".tsh")
}

var (
	KeyFilePrefix       = "teleport_"
	KeyFileSuffix       = ".tkey"
	HostSignersFilename = "HostSigners.db"
)
