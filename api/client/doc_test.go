// Copyright 2021 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package client_test

// this package adds godoc examples for several Client types and functions
// See https://pkg.go.dev/github.com/fluhus/godoc-tricks#Examples

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/types"
)

// Below is an example of creating a new Teleport Auth client with Profile credentials,
// and using that client to create, get, and delete a Role resource object.
//
// Make sure to look at the Getting Started guide before attempting to run this example.
func ExampleClient_roleCRUD() {
	ctx := context.Background()

	// Create a new client in your go file.
	clt, err := client.New(ctx, client.Config{
		Credentials: []client.Credentials{
			client.LoadProfile("", ""),
		},
		// set to true if your Teleport web proxy doesn't have HTTP/TLS certificate
		// configured yet (never use this in production).
		InsecureAddressDiscovery: false,
	})
	if err != nil {
		log.Fatalf("failed to create client: %v", err)
	}
	defer clt.Close()

	// Resource Spec structs reflect their Resource's yaml definition.
	roleSpec := types.RoleSpecV5{
		Options: types.RoleOptions{
			MaxSessionTTL: types.Duration(time.Hour),
		},
		Allow: types.RoleConditions{
			Logins: []string{"role1"},
			Rules: []types.Rule{
				types.NewRule(types.KindAccessRequest, []string{types.VerbList, types.VerbRead}),
			},
		},
		Deny: types.RoleConditions{
			NodeLabels: types.Labels{"*": []string{"*"}},
		},
	}

	// There are helper functions for creating Teleport resources.
	role, err := types.NewRole("role1", roleSpec)
	if err != nil {
		log.Fatalf("failed to get role: %v", err)
	}

	// Getters and setters can be used to alter specs.
	role.SetLogins(types.Allow, []string{"root"})

	// Upsert overwrites the resource if it exists. Use this to create/update resources.
	// Equivalent to `tctl create -f role1.yaml`.
	err = clt.UpsertRole(ctx, role)
	if err != nil {
		log.Fatalf("failed to create role: %v", err)
	}

	// Equivalent to `tctl get role/role1`.
	role, err = clt.GetRole(ctx, "role1")
	if err != nil {
		log.Fatalf("failed to get role: %v", err)
	}

	// Equivalent to `tctl rm role/role1`.
	err = clt.DeleteRole(ctx, "role1")
	if err != nil {
		log.Fatalf("failed to delete role: %v", err)
	}
}
func ExampleNew() {
	ctx := context.Background()
	clt, err := client.New(ctx, client.Config{
		// Multiple Addresses can be provided to attempt to
		// connect to the auth server. At least one address
		// must be provided, except when using the ProfileCreds.
		Addrs: []string{
			// The Auth server address can be provided to connect locally.
			"auth.example.com:3025",
			// The tunnel proxy address can be provided
			// to connect to the Auth server over SSH.
			"proxy.example.com:3024",
			// The web proxy address can be provided to automatically
			// find the tunnel proxy address and connect using it.
			"proxy.example.com:3080",
		},
		// Multiple Credentials can be provided to attempt to authenticate
		// the client. At least one Credentials object must be provided.
		Credentials: []client.Credentials{
			client.LoadProfile("", ""),
			client.LoadIdentityFile("identity-path"),
			client.LoadKeyPair("cert.crt", "cert.key", "cert.cas"),
			client.LoadIdentityFileFromString(os.Getenv("TELEPORT_IDENTITY")),
		},
		// set to true if your web proxy doesn't have HTTP/TLS certificate
		// configured yet (never use this in production).
		InsecureAddressDiscovery: false,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer clt.Close()

	clt.Ping(ctx)
}

// Generate tsh profile with tsh.
//  $ tsh login --user=api-user
// Load credentials from the default directory and current profile, or specify the directory and profile.
func ExampleCredentials_loadProfile() {
	client.LoadProfile("", "")
	client.LoadProfile("profile-directory", "api-user")
}

// Load credentials from the default directory and current profile, or specify the directory and profile.
func ExampleLoadProfile() {
	client.LoadProfile("", "")
	client.LoadProfile("profile-directory", "api-user")
}

// Generate identity file with tsh or tctl.
//  $ tsh login --user=api-user --out=identity-file-path
//  $ tctl auth sign --user=api-user --out=identity-file-path
// Load credentials from the specified identity file.
func ExampleCredentials_loadIdentity() {
	client.LoadIdentityFile("identity-file-path")
}

// Load credentials from the specified identity file.
func ExampleLoadIdentityFile() {
	client.LoadIdentityFile("identity-file-path")
}

// Generate identity file with tsh or tctl.
//  $ tsh login --user=api-user --out=identity-file-path
//  $ tctl auth sign --user=api-user --out=identity-file-path
//  $ export TELEPORT_IDENTITY=$(cat identity-file-path)
// Load credentials from the envrironment variable.
func ExampleCredentials_loadIdentityString() {
	client.LoadIdentityFileFromString(os.Getenv("TELEPORT_IDENTITY"))
}

// Load credentials from the specified environment variable.
func ExampleLoadIdentityFileFromString() {
	client.LoadIdentityFileFromString(os.Getenv("TELEPORT_IDENTITY"))
}

// Generate certificate key pair with tctl.
//  $ tctl auth sign --format=tls --user=api-user --out=path/to/certs
// Load credentials from the specified certificate files.
func ExampleCredentials_loadKeyPair() {
	client.LoadKeyPair(
		"path/to/certs.crt",
		"path/to/certs.key",
		"path/to/certs.cas",
	)
}

// Load credentials from the specified certificate files.
func ExampleLoadKeyPair() {
	client.LoadKeyPair(
		"path/to/certs.crt",
		"path/to/certs.key",
		"path/to/certs.cas",
	)
}
