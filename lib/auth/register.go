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

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
)

// LocalRegister is used to generate host keys when a node or proxy is running within the same process
// as the auth server. This method does not need to use provisioning tokens.
func LocalRegister(dataDir string, id IdentityID, authServer *AuthServer) error {
	keys, err := authServer.GenerateServerKeys(id.HostUUID, id.NodeName, teleport.Roles{id.Role})
	if err != nil {
		return trace.Wrap(err)
	}

	return writeKeys(dataDir, id, keys.Key, keys.Cert)
}

// Register is used to generate host keys when a node or proxy are running on different hosts
// than the auth server. This method requires provisioning tokens to prove a valid auth server
// was used to issue the joining request.
func Register(dataDir, token string, id IdentityID, servers []utils.NetAddr) error {
	tok, err := readToken(token)
	if err != nil {
		return trace.Wrap(err)
	}

	// connect to the auth server using a provisioning token. the auth server will
	// only allow you to connect if it's a valid provisioning token it has generated
	method, err := NewTokenAuth(id.HostUUID, tok)
	if err != nil {
		return trace.Wrap(err)
	}
	client, err := NewTunClient(
		"auth.client.register",
		servers,
		id.HostUUID,
		method)
	if err != nil {
		return trace.Wrap(err)
	}
	defer client.Close()

	// create the host certificate and keys
	keys, err := client.RegisterUsingToken(tok, id.HostUUID, id.NodeName, id.Role)
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
		"auth.server.register",
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
