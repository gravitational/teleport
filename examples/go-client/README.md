## Teleport Auth Go Client

### Introduction

This program demonstrates how to...

1. Create a user and a role which the API client will act as.
2. Authenticate against the API server using any of the three different methods.
3. Create a new client and make API calls.

### API Authentication

Auth API clients must perform two-way authentication using x509 certificates:

1. They have to validate the auth server x509 certificate to make sure the
   API endpoint can be trusted.
2. They must offer their x509 certificate, which has been previously issued
   by the auth sever.

The client supports multiple credential loading methods to authenticate the client. Multiple methods can be used simultaneously in order to increase redundancy or to handle special cases where some authentication methods are expected to fail/expire sometimes.

This demo sets up a few credentials which can be easily tried out in the demo below.

### Authorization

The server will authorize requests for the user associated with the certificates used to authenticate the client. 

Therefore, to use the API client, you need to create a user and any roles it may need for your use case. The client will act on behalf of that user and have access as defined by the user's role(s).

Note: role based access control is an enterprise feature. All users in the OSS versions of Teleport are limited to a single `admin` role.

### Run the Demo

First, create the `access-admin` user and role using the commands below.

```bash
$ tctl create -f access-admin.yaml
$ tctl users add access-admin --roles=access-admin
```

Second, replace the address `localhost:3025` with the address of your auth server. Note that the client only supports local connections, but support will be added for connecting through the proxy soon.

Third, create credentials to authenticate the client, and follow the steps below corresponding to the credentials chosen. Multiple credentials can be specified and the first credentials to successfully load will be used.

1. Identity File Credentials:

   ```bash
   # login and generate identity file
   $ tsh login --user=access-admin --out=certs/access-admin-identity
   ```

2. Key Pair Credentials:

   ```bash
   $ mkdir -p certs
   # Generate certificates from auth server without login
   $ tctl auth sign --format=tls --ttl=87600h --user=access-admin --out=certs/access-admin
   ```

3. TLS Credentials (manual):

   Generate valid TLS certificates by whatever means desired. Replace the following comment in `main.go` with custom logic to load the certificates into the `*tls.Config`:

   ```go
	var tlsConfig *tls.Config
	// Create valid tlsConfig here to use TLS Credentials
   ```

   Note that this is not the recommended strategy since the TLS config can be generated automatically with the methods above. However, some users may find this useful if they already have a TLS config defined or they have a custom Teleport setup.

Lastly, run the demo:

```bash
$ go run .
```