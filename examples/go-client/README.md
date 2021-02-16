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

The client supports multiple `CredentialProviders`, which can gather credentials for client authentication from their defined source.

Multiple providers can be specified in order to try multiple authentication methods. This is helpful for redundancy, especially in cases where some authentication methods are expected to fail/expire sometimes.

This demo sets up a few `CredentialProviders` which can be easily tried out in the demo below.

### Authorization

The Server will authorize requests for the user associated with the certificates used to authenticate the client. 

Therefore, to use the API client, you need to create a user and any roles it may need for your use case. The client will act on behalf of that user and have access as defined by the user's role(s).

Note: role based access control is an enterprise feature. All users in the OSS versions of Teleport are limited to a single `admin` role.

### Run the Demo

First, create the `access-admin` user and role using the commands below.

```bash
$ tctl create -f access-admin.yaml
$ tctl users add access-admin --roles=access-admin
```

Second, replace the address `localhost:3025` with the address of your auth server. Note that the client only supports local connections, but support will be added for connecting through the proxy soon.

Third, choose at least one `CredentialProvider` to authenticate the client, and follow the steps below corresponding to the method. Multiple `CredentialsProvider` can ge specified and the first method to successfully load `Credentials` will be used to authenticate the client.

1. Identity File Provider:

   ```bash
   # login and generate identity file
   $ tsh login --user=access-admin --out=identity_file_path
   ```

2. Key Pair Provider:

   ```bash
   $ mkdir -p certs
   # Generate certificates from auth server without login
   $ tctl auth sign --format=tls --ttl=87600h --user=access-admin --out=certs/access-admin
   ```

3. TLS Provider (manual):

   Generate valid TLS certificates by whatever means desired. Replace the following comment in `main.go` with custom logic to load the certificates into the `*tls.Config`:

   ```go
	var tlsConfig *tls.Config
	// Create valid tlsConfig here to use TLS Provider
   ```

   Note that this is not the recommended strategy since the TLS config can be generated automatically with the methods above. However, some users may find this useful if they already have a TLS config defined or they have an irregular Teleport setup.

Lastly, run the demo:

```bash
$ go run .
```