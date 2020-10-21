---
title: Teleport API Reference
description: The detailed guide to Teleport API
---

# Teleport API Reference

Teleport is currently working on documenting our API.

!!! warning

        We are currently working on this project. If you have an API suggestion, [please complete our survey](https://docs.google.com/forms/d/1HPQu5Asg3lR0cu5crnLDhlvovGpFVIIbDMRvqclPhQg/edit).

# gRPC APIs

In most cases, you can interact with teleport using our cli tools, [tsh](cli-docs.md#tsh) and [tctl](cli-docs.md#tctl). However, there are some scenarios where you may need to interact with teleport programmatically. For this purpose, the same gRPC auth API that `tctl` and `tsh` use is available for direct usage.

## Go Examples

!!! Note
    The Go examples depend on some code that won't be released until `Teleport version 5.0`. Until then, it would be best to start experimenting with the API with the latest [master branch of Teleport](https://github.com/gravitational/teleport).

Below are some code examples that can be used with Teleport to perform a few key tasks.

Before you begin:

- Install [Go](https://golang.org/doc/install) 1.13+ and Setup Go Dev Environment
- Have access to a Teleport Auth server ([quickstart](quickstart.md))

The easiest way to get started with the Teleport API is to clone the [Go Client Example](https://github.com/gravitational/teleport/tree/master/examples/go-client) in our github repo. Follow the README there to quickly authenticate the API with your Teleport Auth Server and test the API.

Or if you prefer, follow the authentication, client, and packages sections below to add the necessary files to a new directory called `/api-examples`. At the end, you should have this file structure:

```
api-examples
+-- api-admin.yaml
+-- certs
|   +-- api-admin.cas
|   +-- api-admin.crt
|   +-- api-admin.key
+-- client.go
+-- go.mod
+-- go.sum
+-- main.go
```

## Authentication

In order to interact with the API, you will need to provision appropriate
TLS certificates. In order to provision certificates, you will need to create a
user with appropriate permissions. You should only give the api user permissions for what it actually needs.

To quickly get started with the api, you can use this api-admin user, but in real usage make sure to have stringent permissions in place.

```yaml
# Copy and Paste the below and run on the Teleport Auth server.
$ cat > api-admin.yaml <<EOF
{!examples/go-client/api-admin.yaml!}
EOF

$ tctl create -f api-admin.yaml
$ mkdir -p certs
$ tctl auth sign --format=tls --user=api-admin --out=certs/api-admin
```

This should result in three PEM encoded files being generated in the `/certs` directory: `api-admin.crt`, `api-admin.key`, and `api-admin.cas` (certificate, private key, and CA certs respectively).

Move the `/certs` folder into your `/api-examples` folder.

!!! Note
    By default, `tctl auth sign` produces certificates with a relatively short lifetime.
    For production deployments, the `--ttl` flag can be used to ensure a more practical
    certificate lifetime. See our [Kubernetes Section](kubernetes-ssh.md#using-teleport-kubernetes-with-automation) for more information on automating the signing process for short lived certificates.

## Go Client

Add `client.go` into `/api-examples`.

**client.go**

```go
{!examples/go-client/client.go!}
```

## Go Packages

Copy the Teleport module's go.mod below into `/api-examples` and then run `go mod tidy` to slim it down to only what's needed for these api examples.

```
{!go.mod!}
```

## Main file

Add this main file to your `/api-examples` folder. Now you can simply plug in the examples below and then run `go run .` to see them in action.

**main.go**

```go
package main

import (
	"fmt"
	"log"
)

func main() {
	log.Printf("Starting teleport client...")
	client, err := connectClient()
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
}
```

## Roles

[Roles](enterprise/ssh-rbac.md#roles) are used to define what resources and actions a user is allowed/denied access to. It's best to make your role's permissions as stringent as possible.

**Retrieve role**

```go
role, err := client.GetRole("auditor")
if err != nil {
  return err
}
```

**Create a new role**

```go
// create a new auditor role which has very limited permissions
role, err := services.NewRole("auditor", services.RoleSpecV3{
  Options: services.RoleOptions{
    MaxSessionTTL: services.Duration(time.Hour),
  },
  Allow: services.RoleConditions{
    Logins: []string{"auditor"},
    Rules: []services.Rule{
      // - resources: ['session']
      //   verbs: ['list', 'read']
      services.NewRule(services.KindSession, services.RO()),
    },
  },
  Deny: services.RoleConditions{
    // node_labels: '*': '*'
    NodeLabels: services.Labels{"*": []string{"*"}},
  },
})
if err != nil {
  return err
}

if err = client.UpsertRole(ctx, role); err != nil {
  return err
}
```

**Update role**

```go
// update the auditor role's ttl to one day
role.SetOptions(services.RoleOptions{
	MaxSessionTTL: services.Duration(time.Hour * 24),
})
if err := client.UpsertRole(ctx, role); err != nil {
  return err
}
```

**Delete role**

```go
if err := client.DeleteRole(ctx, "auditor"); err != nil {
  return err
}
```

## Tokens

[Tokens](admin-guide.md#adding-nodes-to-the-cluster) are used to add nodes to clusters, and add leaf clusters to [trusted clusters](admin-guide.md#trusted-clusters).

**Retrieve token**

```go
token, err := client.GetToken(tokenString)
if err != nil {
  return err
}
```

**Create token**

```go
tokenString, err := client.GenerateToken(ctx, auth.GenerateTokenRequest{
  // You can set token explicitly, otherwise it will be generated
  // Token: CryptoRandomHex()
  // https://sosedoff.com/2014/12/15/generate-random-hex-string-in-go.html
  Roles: teleport.Roles{teleport.RoleProxy},
  TTL:   time.Hour,
})
if err != nil {
  return err
}
```

**Update token**

```go
// updates the token to be a proxy token
token.SetRoles(teleport.Roles{teleport.RoleProxy})
if err := client.UpsertToken(token); err != nil {
  return err
}
```

**Delete token**

```go
if err := client.DeleteToken(tokenString); err != nil {
  return err
}
```

## Cluster Labels

The root cluster in a [trusted cluster](admin-guide.md#adding-nodes-to-the-cluster) can add/update cluster labels on leaf clusters.

**Create a leaf cluster token with Labels**

Leaf clusters will inherit the labels of the [join token](trustedclusters.md#join-tokens) used to add them to the trusted cluster. This is the preferred method of managing cluster tokens.

Follow the trusted cluster join token [docs](trustedclusters.md#join-tokens) to use this token to create a leaf cluster using `tctl`.

```go
tokenString, err := client.GenerateToken(ctx, auth.GenerateTokenRequest{
  // You can provide 'Token' for a static token name
  Roles: teleport.Roles{teleport.RoleTrustedCluster},
  TTL:   time.Hour,
  Labels: map[string]string{
    "env": "staging",
  },
})
```

**Update leaf cluster's labels**

You can also update a leaf cluster's labels from the root cluster using the auth api if necessary.

```go
rc, err := client.GetRemoteCluster("leafClusterName")
if err != nil {
  return err
}

md := rc.GetMetadata()
md.Labels = map[string]string{"env": "prod"}
rc.SetMetadata(md)

if err = client.UpdateRemoteCluster(ctx, rc); err != nil {
  return err
}

```

## Access Workflow

[Access Workflow](enterprise/workflow/index.md) can be used to dynamically approve and deny a user access to local or remote resources in a trusted cluster.

**Retrieve access requests**

```go
filter := services.AccessRequestFilter{State: services.RequestState_PENDING}
ars, err := client.GetAccessRequests(ctx, filter)
if err != nil {
  return err
}
```

**Create access request**

```go
// create a new access request for api-admin to use the admin role in the cluster
ar, err := services.NewAccessRequest("api-admin", "admin")
if err != nil {
  return err
}

if err = client.CreateAccessRequest(ctx, accessReq); err != nil {
  return err
}
```

**Approve access request**

```go
aruApprove := services.AccessRequestUpdate{
  RequestID: accessReqID,
  State:     services.RequestState_APPROVED,
}
if err := client.SetAccessRequestState(ctx, aruApprove); err != nil {
  return err
}
```

**Deny access request**

```go
aruDeny := services.AccessRequestUpdate{
  RequestID: accessReqID,
  State:     services.RequestState_DENIED,
}
if err := client.SetAccessRequestState(ctx, aruDeny); err != nil {
  return err
}
```

**Delete access request**

```go
if err = client.DeleteAccessRequest(ctx, accessReqID); err != nil {
	return err
}
```

## Certificate Authority

It might be useful to retrieve your [Certificate Authority](architecture/authentication.md#ssh-certificates) through the API if it is rotating frequently.

```go
ca, err := client.GetCertAuthority(services.CertAuthID{
  DomainName: clusterName,
  Type:       services.HostCA,
}, false)
if err != nil {
  return err
}
```