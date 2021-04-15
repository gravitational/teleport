## Teleport Auth Go Client

This program demonstrates how to...

1. Authenticate the client using credential loaders.
2. Authorize API calls using an independent user and role.
3. Create a new client and make API calls to the Auth server.

### Demo

This demo can be used to quickly get the API client up and running.

##### Create resources

Create the `access-admin` user and role using the following commands:

```bash
$ tctl create -f access-admin.yaml
$ tctl users add access-admin --roles=access-admin
```

##### Generate Credentials

Login with `tsh` to generate Profile credentials.

```bash
# login and automatically generate keys
$ tsh login --user=access-admin
```

NOTE: You can pass the `InsecureAddressDiscovery` in `client.Config` field to skip verification of the TLS certificate of the proxy. Don't do this for production clients.

##### Run

```bash
$ go run main.go
```

### Reference

To see more information on the Go Client and how to use it, visit our API documentation:

- [Introduction](https:/goteleport.com/docs/reference/api/introduction)
- [Getting Started](https:/goteleport.com/docs/reference/api/getting-started)
- [Architecture](https:/goteleport.com/docs/reference/api/architecture)
- [pkg.go.dev](https://pkg.go.dev/github.com/gravitational/teleport/api/client)
  - [Client type](https://pkg.go.dev/github.com/gravitational/teleport/api/client#Client)
  - [Credentials type](https://pkg.go.dev/github.com/gravitational/teleport/api/client#Credentials)