## Teleport Auth Go Client

### Introduction

This program demonstrates how to...

1. Create a user and a role which the API client will act as.
2. Authenticate against the API server using any of the four different methods.
3. Create a new client and make API calls.

### API Authentication

Auth API clients must perform two-way authentication using x509 certificates:

1. They have to validate the auth server x509 certificate to make sure the
   API endpoint can be trusted.
2. They must offer their x509 certificate, which has been previously issued
   by the auth sever.

There are a few ways to generate the certificates needed to authenticate the client, each of which is detailed in this demo.

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

Second, replace the address `proxy.example.com:3025` with the address of your auth server.

Third, choose one of the authentication methods and make the corresponding changes to `main.go`.

1. login and generate tsh profile (default):

   ```bash 
   $ tsh login --user=access-admin
   ```

2. login and generate identity file:

   ```bash
   $ tsh login --user=access-admin --out=identity_file_path
   ```

   In `main.go` replace `client.ProfileCreds`, with `client.IdentityFileCreds`.

   ```go
   creds, err := client.IdentityFileCreds("identity_file_path")
   ```

3. generate certificates from auth server without login:

   ```bash
   $ mkdir -p certs
   $ tctl auth sign --format=tls --ttl=87600h --user=access-admin --out=certs/access-admin
   ```

   In `main.go` replace `client.ProfileCreds`, with `client.CertsPathCreds`.

   ```go
   creds, err := client.CertsPathCreds("certs/access-admin")
   ```

4. generate and gather TLS config manually.

   Note that this is not the recommended strategy since the TLS config can be generated automatically with the methods above. However, some users may find this useful if they already have a TLS config defined or they have an irregular Teleport setup.
   
   Generate valid TLS certificates by whatever means desired. In `main.go`, use the ge certificates to build a new `*tls.Config` value and use it in `client.TLSCreds` to create credentials.

   ```go
   creds, err := client.TLSCreds(tls.Config)
   ```

Lastly, run the demo:

```bash
$ go run .
```