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
package auth

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"strings"

	"github.com/gravitational/teleport/lib/backend/encryptedbk/encryptor"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/trace"
	"github.com/gravitational/teleport/Godeps/_workspace/src/golang.org/x/crypto/ssh"
)

func Register(fqdn, dataDir, token string, servers []utils.NetAddr) error {
	tok, err := readToken(token)
	if err != nil {
		return err
	}
	method, err := NewTokenAuth(fqdn, tok)
	if err != nil {
		return err
	}
	config := &ssh.ClientConfig{
		User: fqdn,
		Auth: method,
	}
	client, err := ssh.Dial(servers[0].Network, servers[0].Addr, config)
	if err != nil {
		return err
	}
	defer client.Close()

	ch, _, err := client.OpenChannel(ReqProvision, nil)
	if err != nil {
		return err
	}
	defer ch.Close()

	buf := &bytes.Buffer{}
	if _, err = io.Copy(buf, ch.Stderr()); err != nil {
		return fmt.Errorf("failed to read key pair from channel: %v", err)
	}
	var keys *PackedKeys
	if err := json.NewDecoder(buf).Decode(&keys); err != nil {
		return err
	}
	return writeKeys(fqdn, dataDir, keys.Key, keys.Cert)
}

func RegisterNewAuth(fqdn, token string, publicSealKey encryptor.Key,
	servers []utils.NetAddr) (masterKey encryptor.Key, e error) {
	tok, err := readToken(token)
	if err != nil {
		return encryptor.Key{}, err
	}
	method, err := NewTokenAuth(fqdn, tok)
	if err != nil {
		return encryptor.Key{}, err
	}
	config := &ssh.ClientConfig{
		User: fqdn,
		Auth: method,
	}

	// initializing ssh channel
	client, err := ssh.Dial(servers[0].Network, servers[0].Addr, config)
	if err != nil {
		return encryptor.Key{}, err
	}
	defer client.Close()

	ch, _, err := client.OpenChannel(ReqNewAuth, nil)
	if err != nil {
		return encryptor.Key{}, err
	}
	defer ch.Close()

	// writing server own public seal key to the channel
	data, err := json.Marshal(publicSealKey.Public())
	if err != nil {
		return encryptor.Key{}, trace.Errorf("gen marshal error: %v", err)
	}

	if _, err := io.Copy(ch.Stderr(), bytes.NewReader(data)); err != nil {
		return encryptor.Key{}, trace.Errorf("key transfer error: %v", err)
	}

	if err := ch.CloseWrite(); err != nil {
		return encryptor.Key{}, trace.Errorf("Can't close write: &v", err)
	}

	// reading master public seal key from the channel
	buf := &bytes.Buffer{}
	if _, err = io.Copy(buf, ch.Stderr()); err != nil {
		return encryptor.Key{}, fmt.Errorf("failed to read key from channel: %v", err)
	}

	if err := json.NewDecoder(buf).Decode(&masterKey); err != nil {
		return encryptor.Key{}, err
	}

	return masterKey, nil
}

func readToken(token string) (string, error) {
	if !strings.HasPrefix(token, "/") {
		return token, nil
	}
	// treat it as a file
	out, err := ioutil.ReadFile(token)
	if err != nil {
		return "", nil
	}
	return string(out), nil
}

type PackedKeys struct {
	Key  []byte `json:"key"`
	Cert []byte `json:"cert"`
}
