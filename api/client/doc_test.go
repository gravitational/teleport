package client_test

// this package adds godoc examples for several Client types and functions
// See https://pkg.go.dev/github.com/fluhus/godoc-tricks#Examples

import (
	"context"
	"log"

	"github.com/gravitational/teleport/api/client"
)

func ExampleNew() {
	ctx := context.Background()
	clt, err := client.New(ctx, client.Config{
		// Multiple Addresses can be provided to attempt to
		// connect to the auth server. At least one address
		// must be provided, except when using the ProfileCreds.
		Addrs: []string{
			// The auth server is only directly available locally
			"localhost:3025", // 3025 is the default auth port
			// public_address is the cluster's public address, and can be
			// used to connect to the auth server over ssh.
			"public_address:3080", // 3080 is the default web proxy port
			"public_address:3024", // 3024 is the default tunnel proxy port
		},
		// Multiple Credentials can be provided to attempt to authenticate
		// the client. At least one Credentials object must be provided.
		Credentials: []client.Credentials{
			client.LoadProfile("", ""),
			client.LoadIdentityFile("identity-path"),
			client.LoadKeyPair("cert.crt", "cert.key", "cert.cas"),
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
