---
authors: Noah Stride (noah.stride@goteleport.com)
state: draft
---

# RFD 0145 - API Client support for Credential Reload

## Required Approvals

* Engineering: ??
* Product: ??
* Security: @reedloden || @jent

## What

Support reloading credentials without the recreation of the Teleport API client.

This was initially specified in "RFD 0010 - API and Client Libraries":

> Credential loaders should detect underlying file changes and automatically reload credentials and update the grpc transport.

However, this did not make it into the implementation.

## Why

Teleport encourages the use of short-lived certificates, but consuming these
with our API client is difficult. 

In the current state, we rely on consumers to implement their own reload 
mechanism which detects the change to the file and then creates a new client, 
they then must propagate the new client through their application. This is a 
relatively challenging feat for less experienced Go engineers and raises the 
barrier of entry to those who wish to create custom automations.

This not only affects the custom automations, but also our Teleport Access
Plugins.

This issue has raised in prominence with the growth of Machine ID but as
Machine ID produces short-lived certificates it is incompatible with usage
of the API client and Access plugins in their current state.

## Details

It may be helpful to reduce the success criteria of this build to focus on
short-lived identity files. This simplifies the implementation as an identity
file contains all relevant material within a single file. This is in contrast
to a separate key and certificate file where it is possible to reload it in an
inconsistent state where the key does not match the certificate.

One of the challenges with implementing this is the complexity of transport
to the Teleport API - we not only need to reload the TLS credentials which are
presented during the TLS handshake, but also the SSH credentials which are
used as part of our custom diallers that establish connectivity over tunnels.

We should also aim to support the rotation of Certificate Authorities. Machine
ID will output rotated CA credentials and having the API client automatically
load these will improve stability of tools that use the API over CA rotations.

### Option A: Credential reloading

In this option, we build support for reloading files directly into each of the 
`client.Credential` credential loader implementations. The client itself remains
unaware that any credential reloading as occurred.

For a developer writing a custom automation, it would look like so:

```go
func example() {
	identityFileWatcher, err := client.WatchIdentityFile("my-identity-file")
	defer identityFileWatcher.Close()
	
    clt, err := client.New(ctx, client.Config{
        Addrs: []string{
        "tele.example.com:443",
        },
        Credentials: []client.Credentials{
            identityFileWatcher,
        },
    })
	
	// Or, rather than relying on change detection within the
	// IdentityFileWatcher, a Reload() could be manually invoked.
	err = identityFileWatcher.Reload()
}
```

The challenge of this is that the `tls.Config` and `ssh.Config` passed into the
`api.Client` cannot be mutated. This is not just down to concerns of concurrent
read and writes, but also because these config types are deep copied before use
in the gRPC client.

Therefore, this implementation relies on the callback fields offered on these
config types (e.g `tls.Config.VerifyConnection`). This can be challenging as 
these callbacks do not always map one-to-one to the static field equivalent.

Benefits:

- The scope of the changes in this build is limited to the credential loader
  implementations themselves - this reduces the risk of introducing a bug in the
  Teleport API client and makes this build simpler.
- By creating a new `client.Credential` instead of modifying an existing one, we
  can further reduce the risk of regressions.
- Implementation is relatively simple compared to option B.

Drawbacks:

- Changes in credentials (e.g roles associated with the identity) will not be 
  reflected in requests made to the server until the connection between the Auth
  Server and client is broken. This could be anything from minutes to days 
  depending on the stability of the connection.

### Option B: Client reloading

In this option, no changes are made to the existing credential loaders. Instead,
a change is made to the Teleport Client to support reloading credentials on a 
fixed interval or when manually requested.

For a developer writing a custom automation, it would look like so:

```go
func example() {
    clt, err := client.New(ctx, client.Config{
        Addrs: []string{
        "tele.example.com:443",
        },
        Credentials: []client.Credentials{
            client.LoadIdentityFile("my-identity-file"),
        },
		ReloadInterval: time.Minute * 5,
    })
	
	// Or, rather than relying on ReloadInterval, ReloadCredentials() could be
	// invoked in a file watcher.
	err = clt.ReloadCredentials(ctx)
}
```

### Decision
