## Teleport Auth Go Client

### Introduction

This program demonstrates how to...

1. Create a user and role, which the API client will act as.
2. Authenticate against the API server using the four different methods.
3. Create a new client and make API calls.

### API Authentication

Auth API clients must perform two-way authentication using x509 certificates:

1. They have to validate the auth server x509 certificate to make sure the
   API endpoint can be trusted.
2. They must offer their x509 certificate, which has been previously issued
   by the auth sever.

There are a few simple ways to generate the certificates needed to authenticate the client, each of which is detailed in this demo.

### Authorization

The Server will authorize requests for the user associated with the certificates used to authenticate the client. 

Therefore, to use the API client, you need to create a user and any roles it may need for your use case. The client will act as that user and have access to everything defined by the user's role(s).

Note: role based access control is an enterprise feature. All users in the OSS versions of Teleport have the role `admin`.

### Run the Demo

First, create the `access-admin` user and role using the commands below.

```bash
$ tctl create -f access-admin.yaml
$ tctl users add access-admin --roles=access-admin
```

Second, Replace the address `proxy.example.com:3025` with the address of your auth server.

Third, choose one of the authentication methods, and make the corresponding changes to `main.go`.

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

   Generate valid TLS certificates by whatever means desired. In `main.go`, provide valid credentials to create a `*tls.Config`, and replace `client.ProfileCreds` with `client.TLSCreds`.

   ```go
   tls := &tls.Config{...}
   creds, err := client.TLSCreds(tls)
   ```

Lastly, run the demo:

```bash
$ go run .
```