## Teleport Auth Go Client

### Introduction

This program demonstrates how to...

1. Authenticate against the Auth API using the three different methods.
2. Make API calls to issue CRUD requests for cluster join tokens, roles, and labels.
3. Receive, allow, and deny access requests.

### API Authentication

Auth API clients must perform two-way authentication using x509 certificates:

1. They have to validate the auth server x509 certificate to make sure the
   API endpoint can be trusted.
2. They must offer their x509 certificate, which has been previously issued
   by the auth sever.

There are a few ways to generate the certificates needed to authenticate the client. This
demo uses the default profile, which is generated with `tsh login`.

### API Authorization

The API will authorize requests based on the certs provided. Since these credentials
are created by `tsh login`, the API will authorize requests based on that login.

Note: role based access control is an enterprise feature. All users will be treated 
as the `admin` role in non-enterprise instances of Teleport. 

### Run the Demo

To use the API client, you need to create a role, a user with that role, and login as that user. That client will act as that user and have access to everything defined by the role.

You can test that flow here with the small example in `main.go`, where an `access-admin`, which only has priviledges for creating and updating access-requests, requests to use the `admin` role.

First, create the user and role described using the commands below.

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

   In `main.go` replace `client.ProfileCreds()` (line 33), with `client.IdentityCreds("full_id_file_path")`.

   ```bash
   $ tsh login --user=access-admin --out=[full_id_file_path]
   ```

3. generate certificates from auth server without login:

   In `main.go` replace `client.ProfileCreds()` (line 33), with `client.PathCreds("certs/access-admin")`.

   ```bash
   $ mkdir -p certs
   $ tctl auth sign --format=tls --ttl=87600h --user=access-admin --out=certs/access-admin
   ```

4. Generate the `tls.Config` yourself, and provide it directly to `NewClient` using `client.TLSCreds(tls.Config)`.

Lastly, run the demo:

```bash
$ go run .
```