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

The Server will authorize requests based on the user associated with the certs used to authenticate the Client. 

Therefore, to use the API client, you need to create a user and any roles it may need for your use case. The client will act as that user and have access to everything defined by the user's role(s).

Note: role based access control is an enterprise feature. All users will be treated 
as the `admin` role in non-enterprise instances of Teleport. 

### Run the Demo

First, create the `access-admin` user and role using the commands below.

```bash
$ tctl create -f access-admin.yaml
$ tctl users add access-admin --roles=access-admin
```

Second, Replace the address `proxy.example.com:3025` with the address of your auth server.

Third, choose one of the authentication methods.

1. tsh profile (default):

   ```bash 
   $ tsh login --user=access-admin
   ```

2. identity file:

   In `main.go` replace `client.ProfileCreds()` (line 33), with `client.IdentityFileCreds("identity_file_path")`.

   ```bash
   $ tsh login --user=access-admin --out=identity_file_path
   ```

3. generate certificates from auth server without login:

   In `main.go` replace `client.ProfileCreds()` (line 33), with `client.CertsPathCreds("certs/access-admin")`.

   ```bash
   $ mkdir -p certs
   $ tctl auth sign --format=tls --ttl=87600h --user=access-admin --out=certs/access-admin
   ```

4. Generate the `tls.Config` yourself, and provide it directly to `NewClient` using `client.TLSCreds(tls.Config)`.

Lastly, run the demo:

```bash
$ go run .
```