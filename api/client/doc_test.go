package client_test

// this package adds godoc examples for several Client types and functions
// See https://pkg.go.dev/github.com/fluhus/godoc-tricks#Examples

import (
	"context"
	"crypto/tls"

	"github.com/gravitational/teleport/api/client"
)

var ctx context.Context

func ExampleCredentials_profile() {
	// Generate tsh profile with tsh.
	// $ tsh login --user=api-user

	// Load credentials from current profile in the default directory.
	client.LoadProfile("", "")

	// Load credentials from a the profile specified by the given directory and user.
	client.LoadProfile("profile-directory", "api-user")
}

func ExampleLoadProfile() {
	// Generate tsh profile with tsh.
	// $ tsh login --user=api-user

	// Load credentials from current profile in the default directory.
	client.LoadProfile("", "")

	// Load credentials from a the profile specified by the given directory and user.
	client.LoadProfile("profile-directory", "api-user")
}

func ExampleCredentials_identity() {
	// Generate identity file with tsh.
	// $ tsh login --user=api-user --out=identity-file-path
	//    OR
	// Generate identity file with tctl. (expiration customizable by --ttl)
	// $ tctl auth sign --user=api-user --out=identity-file-path

	// Load credentials from the specified identity file.
	client.LoadIdentityFile("identity-file-path")
}

func ExampleLoadIdentityFile() {
	// Generate identity file with tsh.
	// $ tsh login --user=api-user --out=identity-file-path
	//    OR
	// Generate identity file with tctl. (expiration customizable by --ttl)
	// $ tctl auth sign --user=api-user --out=identity-file-path

	// Load credentials from the specified identity file.
	client.LoadIdentityFile("identity-file-path")
}

func ExampleCredentials_keypair() {
	// Generate certificate key pair with tctl.
	// $ tctl auth sign --format=tls --user=api-user --out=path/to/certs
	client.LoadKeyPair("path/to/certs.crt", "path/to/certs.key", "path/to/certs.cas")
}

func ExampleLoadKeyPair() {
	// Generate certificate key pair with tctl.
	// $ tctl auth sign --format=tls --user=api-user --out=path/to/certs
	client.LoadKeyPair("path/to/certs.crt", "path/to/certs.key", "path/to/certs.cas")
}

var clt *client.Client
var err error

func ExampleNew() {
	clt, err = client.New(ctx, client.Config{
		// Multiple Addresses can be provided to attempt to connect to the auth server.
		// At least one address must be provided, except when using the ProfileCreds.
		Addrs: []string{
			// The auth server is only directly available locally
			"localhost:3025", // 3025 is the default auth port
			// public_address is the cluster's public address, and can be
			// used to connect to the auth address over ssh.
			"public_address:3080", // 3080 is the default web proxy port
			"public_address:3024", // 3024 is the default tunnel proxy port
		},
		// Multiple Credentials Can be provided to attempt to authenticate the client.
		// At least one Credentials must be provided. Some Credentials have additional
		// functionality, such as ssh connectivity and automatic address discovery.
		Credentials: []client.Credentials{
			client.LoadProfile("", ""),
			client.LoadIdentityFile("identity-path"),
			client.LoadKeyPair("cert.crt", "cert.key", "cert.cas"),
			// TLSCreds are primarily used internally and not recommended for most users.
			client.LoadTLS(&tls.Config{}),
		},
		// set to true if your web proxy doesn't have HTTP/TLS certificate
		// configured yet (never use this in production).
		InsecureAddressDiscovery: false,
	})
}
