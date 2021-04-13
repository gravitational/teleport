### Introduction

The Teleport API Client provides secure access to the gRPC Auth API used by Teleport's CLIs ([tsh](https://goteleport.com/docs/cli-docs#tsh) and [tctl](https://goteleport.com/docs/cli-docs#tctl).

A fair amount of Teleport knowledge is expected before getting started. Make sure to familiarize yourself with the [admin manual](https://goteleport.com/docs/admin-guide/) and other [Teleport documentation](https://goteleport.com/docs/).

[Skip to Getting Started](#getting-started)

#### Authentication

The client uses TLS certificates signed by a Teleport Auth server to authenticate its connection. 
These TLS certificates must be associated with a Teleport user, which allows the client to
impersonate that user.

#### Authorization

Once authenticated, the client will be authorized to call client methods that the underlying user has
permissions for.

For example, the client user must have permission to perform `read` on `role` in order to call:
```go
role, err := clt.GetRole(ctx, "role1")
```

See our [roles documentation](https://goteleport.com/docs/access-controls/reference/#roles) for more details
on role based access control.

#### Credentials
Credentials hold TLS certificates and provide additional features and benefits depending on which Credential loader you choose.

Since there are several Credential loaders to choose from, here's a quick breakdown:

 - Profile Credentials are the easiest to get started with. All you have to do is login
   on your device with `tsh login`. Your Teleport proxy address and credentials will
   automatically be located and used. However, the other options don't necessarily require
   a login step and have the ability to authenticate long lived clients.

 - IdentityFile Credentials are the most well rounded in terms of usability, functionality,
   and customizability. Identity files can be generated through `tsh login` or `tctl auth sign`,
   making them ideal for both long lived proxy and auth server connections.

 - Key Pair Credentials have a much simpler implementation than the first two Credentials listed,
   and may feel more familiar. These are good for authenticating client's hosted on the auth server.

 - TLS Credentials leave everything up to the client user. This isn't recommended for most
   users and is mostly used internally, but some users may find that this fits their use case best.

Here are some more specific details to differentiate them by:

| Type | Profile Credentials | Identity Credentials | Key Pair Credentials | TLS Credentials |
| - | - | - | - | - | 
| Ease of use | easy | easy | med | hard | 
| Supports long lived certificates | not easily (configurable on server side) | yes | yes | yes | 
| Supports proxy connections | yes | yes (6.1+) | no | no | 
| Automatic Proxy Address discovery | yes | no | no | no | 
| CLI used | tsh | tctl/tsh | tctl | - | 
| Available in | 6.1+ | 6.0+ | 6.0+ | 6.0+ | 

See the [Credentials type](https://pkg.go.dev/github.com/gravitational/teleport/api/client#Credentials) on pkg.go.dev for more information and examples for each.

#### Additional resources

The Teleport client is mainly documented through [pkg.go.dev](https://pkg.go.dev/github.com/gravitational/teleport/api/client). 

For supplementary information about using Teleport, visit the main Teleport [docs](https://goteleport.com/docs/).

### Getting started

Before you begin:

- Install [Go](https://golang.org/doc/install) 1.15+ and Setup Go Dev Environment
- Set up Teleport with the [Getting Started Guide](https://goteleport.com/docs/getting-started/)

Once you have the required rerequisites, simply set up a new [go module](https://golang.org/doc/tutorial/create-module) and import the `client` package: 

```bash
$ go mod init client-demo
$ go get github.com/gravitational.com/api/client
```

#### Create a User and Role

It is recommended to make a new user and role for each client. This makes it easier to track client actions, as well as carefully control client permissions.

For the example below, the client must have `create`, `read`, and `delete` permissions for the `role` resource. Use the following set of commands to create a new user and role directly on your teleport Auth server.

```bash
# Copy and Paste the below and run on the Teleport Auth server.
$ cat > api-role.yaml <<EOF
kind: role
metadata:
  name: api-role
spec:
  allow:
    rules:
      - resources: ['role']
        verbs: ['create', 'read', 'delete']
  deny:
    node_labels:
      '*': '*'
version: v3
EOF
$ tctl create -f api-role.yaml
# Add user and login via web proxy
$ tctl users add api-user --roles=api-role
```

#### Client Credentials

The simplest [Credentials](#credentials) to get started with are Profile Credentials. Simply log in with `tsh`, and the [Profile Credential loader](https://pkg.go.dev/github.com/gravitational/teleport/api/client#LoadProfile) will automatically retrieve SSH and TLS certificates and a public proxy address from the current profile.

```bash
# generate tsh profile
$ tsh login --user=api-user
```

#### Create a Client

```go
package main

import (
    "context"
    "log"

    "github.com/gravitational/teleport/api/client"
)

func main() {
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
}
```

#### Using the Client

The client created above can be used to make a variety of API calls. Below is an
example of creating, getting, and deleting a Role resource object. 

```go
package main

import (
	"context"
	"log"
	"time"

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/types"
)

func main() {
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
	roleSpec := types.RoleSpecV3{
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

	// There are helper functions for dealing with Teleport resources.
	role, err := types.NewRole("role1", roleSpec)
	if err != nil {
		log.Fatalf("failed to get role: %v", err)
	}

	// Getters and setters can be used to alter specs.
	role.SetLogins(types.Allow, []string{""})

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

```