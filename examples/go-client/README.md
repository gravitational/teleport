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

Second, replace the address `localhost:3025` with the address of your auth server. Note that the client only supports local connections, but support will be added for connecting through the proxy soon.

Third, choose one of the authentication methods and make the corresponding changes to `main.go`.

1. login and generate identity file:

   ```bash
   $ tsh login --user=access-admin --out=identity_file_path
   ```

   In `main.go` place the following code before `client.New`:

   ```go
   creds, err = client.LoadIdentityFile("identity_file_path")
	if err != nil {
		log.Fatalf("Failed to load credentials: %v", err)
	}
	creds1.TLS.Certificates[0] = tls.Certificate{}
   ```

2. generate certificates from auth server without login:

   ```bash
   $ mkdir -p certs
   $ tctl auth sign --format=tls --ttl=87600h --user=access-admin --out=certs/access-admin
   ```

   In `main.go` place the following code before `client.New`:

   ```go
   creds, err = client.LoadKeyPair("certs/access-admin.crt", "certs/access-admin.key", "certs/access-admin.cas")
	if err != nil {
		log.Fatalf("Failed to load credentials: %v", err)
	}
   ```

3. generate and gather TLS config manually.

   Generate valid TLS certificates by whatever means desired. 
   
   In `main.go` place the following code before `client.New`, replacing the comment with your logic to load the certificates into the `*tls.Config`:


   ```go
   var tlsConfig *tls.Config
   // load certificates into the tlsConfig here
   creds, err = client.LoadConfig(tlsConfig)
	if err != nil {
		log.Fatalf("Failed to load credentials: %v", err)
	}
   ```

   Note that this is not the recommended strategy since the TLS config can be generated automatically with the methods above. However, some users may find this useful if they already have a TLS config defined or they have an irregular Teleport setup.

4. generate multiple credentials and pass them all to the client:

   It might be useful to provide multiple credentials to the client, in case they are expected to expire quickly, or just for redundancy. Just pass a list of credentials to `client.New`, and whichever credentials successfully authenticate the client first will be used.

   Update the `Credentials` in main.go.

   ```go
   Credentials: client.CredentialsList{
		creds1,
      creds2,
      creds3,
      ...
	},
   ```

Lastly, run the demo:

```bash
$ go run .
```