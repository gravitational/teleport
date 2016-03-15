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
	"io/ioutil"
	"strings"

	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
)

// LocalRegister is used in standalone mode to register roles without
// connecting to remote clients and provisioning tokens
func LocalRegister(dataDir string, id IdentityID, authServer *AuthServer) error {
	keys, err := authServer.GenerateServerKeys(id.HostUUID, id.Role)
	if err != nil {
		return trace.Wrap(err)
	}
	return writeKeys(dataDir, id, keys.Key, keys.Cert)
}

// Register is used by auth service clients (other services, like proxy or SSH) when a new node
// joins the cluster
func Register(dataDir, token string, id IdentityID, servers []utils.NetAddr) error {
	tok, err := readToken(token)
	if err != nil {
		return trace.Wrap(err)
	}
	method, err := NewTokenAuth(id.HostUUID, tok)
	if err != nil {
		return trace.Wrap(err)
	}

	client, err := NewTunClient(
		servers,
		id.HostUUID,
		method)
	if err != nil {
		return trace.Wrap(err)
	}
	defer client.Close()

	keys, err := client.RegisterUsingToken(tok, id.HostUUID, id.Role)
	if err != nil {
		return trace.Wrap(err)
	}
	return writeKeys(dataDir, id, keys.Key, keys.Cert)
}

func RegisterNewAuth(domainName, token string, servers []utils.NetAddr) error {
	tok, err := readToken(token)
	if err != nil {
		return trace.Wrap(err)
	}
	method, err := NewTokenAuth(domainName, tok)
	if err != nil {
		return trace.Wrap(err)
	}

	client, err := NewTunClient(
		servers,
		domainName,
		method)
	if err != nil {
		return trace.Wrap(err)
	}
	defer client.Close()

	return client.RegisterNewAuthServer(tok)
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
