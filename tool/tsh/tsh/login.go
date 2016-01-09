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
*/
package tsh

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gravitational/teleport/lib/auth/native"
	"github.com/gravitational/teleport/lib/web"

	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/log"
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/trace"
	"github.com/gravitational/teleport/Godeps/_workspace/src/golang.org/x/crypto/ssh"
	"github.com/gravitational/teleport/Godeps/_workspace/src/golang.org/x/crypto/ssh/agent"
	"github.com/gravitational/teleport/Godeps/_workspace/src/golang.org/x/crypto/ssh/terminal"
)

func getLocalAgent() (agent.Agent, error) {
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
	_, err := os.Stat(KeysDir)
	if os.IsNotExist(err) {
		err = os.MkdirAll(KeysDir, os.ModeDir|0777)
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

func login(ag agent.Agent, webProxyAddr string, user string,
	ttl time.Duration) error {
	fmt.Printf("Enter your password for user %v:\n", user)
	password, err := readPassword()
	if err != nil {
		fmt.Println(err)
		return trace.Wrap(err)
	}

	fmt.Printf("Enter your HOTP token:\n")
	hotpToken, err := readPassword()
	if err != nil {
		fmt.Println(err)
		return trace.Wrap(err)
	}

	fmt.Printf("Logging in...\n")

	priv, pub, err := native.New().GenerateKeyPair("")
	if err != nil {
		return trace.Wrap(err)
	}

	cert, err := web.SSHAgentLogin(webProxyAddr, user, password, hotpToken,
		pub, ttl)
	if err != nil {
		return trace.Wrap(err)
	}

	pcert, _, _, _, err := ssh.ParseAuthorizedKey(cert)
	if err != nil {
		return trace.Wrap(err)
	}

	pk, err := ssh.ParseRawPrivateKey(priv)
	if err != nil {
		return trace.Wrap(err)
	}
	addedKey := agent.AddedKey{
		PrivateKey:       pk,
		Certificate:      pcert.(*ssh.Certificate),
		Comment:          "",
		LifetimeSecs:     0,
		ConfirmBeforeUse: false,
	}
	if err := ag.Add(addedKey); err != nil {
		return trace.Wrap(err)
	}

	key := Key{
		Priv:     priv,
		Cert:     cert,
		Deadline: time.Now().Add(ttl),
	}

	keyID := time.Now().Sub(time.Time{}).Nanoseconds()
	keyPath := filepath.Join(KeysDir,
		KeyFilePrefix+strconv.FormatInt(keyID, 16)+KeyFileSuffix)

	err = saveKey(key, keyPath)
	if err != nil {
		return trace.Wrap(err)
	}

	fmt.Println("Logged in successfully")
	return nil
}

type Key struct {
	Priv     []byte
	Cert     []byte
	Deadline time.Time
}

func saveKey(key Key, filename string) error {
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
	files, err := ioutil.ReadDir(KeysDir)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for _, file := range files {
		if !file.IsDir() && strings.HasPrefix(file.Name(), KeyFilePrefix) &&
			strings.HasSuffix(file.Name(), KeyFileSuffix) {
			key, err := loadKey(filepath.Join(KeysDir, file.Name()))
			if err != nil {
				log.Errorf(err.Error())
				continue
			}

			if time.Now().Before(key.Deadline) {
				keys = append(keys, key)
			} else {
				// remove old keys
				err = os.Remove(filepath.Join(KeysDir, file.Name()))
				if err != nil {
					log.Errorf(err.Error())
				}
			}
		}
	}
	return keys, nil
}

func readPassword() (string, error) {
	password, err := terminal.ReadPassword(0)
	if err != nil {
		return "", trace.Wrap(err)
	}
	return string(password), nil
}

const (
	KeysDir       = "/tmp/teleport"
	KeyFilePrefix = "teleport_"
	KeyFileSuffix = ".tkey"
)
