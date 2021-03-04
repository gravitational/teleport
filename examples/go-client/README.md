## Teleport Auth Go Client

### Introduction

This program demonstrates how to...

1. Create a user and a role which the API client will act on behalf of.
2. Authenticate the client using the supported methods.
3. Create a new client and make API calls.

### Authentication

Auth API clients must perform two-way authentication using x509 certificates to:

1. Validate the auth server x509 certificate to make sure the API endpoint can be trusted.
2. Offer their x509 certificate, which has been previously issued by the auth sever.

The client supports multiple credential loading methods to authenticate the client.

1. Identity File Credentials:

   This method creates short lived credentials (12 hours by default) and mirrors the login flow of `tsh`. This method can be used to act on behalf of users in a dynamic system.

   For example, one could write an external CLI which has more advanced combinations of actions compared to `tsh` and `tctl`. Teleport users could use this CLI by logging in with `tsh login --out=identity-file-path`.

   Additionally, this is the only method that currently supports connecting through SSH, which is necessary for connecting to proxy or web proxy addresses.

   ```bash
   # login and generate identity file
   $ tsh login --user=access-admin --out=certs/access-admin-identity
   ```

2. Key Pair Credentials:

   This method can be used to create long lived credentials, which can be defined by the flag `--ttl`. This method is useful for long lived API clients which have static permissions.

   For example, a custom access workflow bot which automatically manages access requests could use long term credentials since its role is static.

   ```bash
   $ mkdir -p certs
   # Generate long term certificates from auth server without login
   $ tctl auth sign --format=tls --ttl=8760h --user=access-admin --out=certs/access-admin
   ```

3. Manual TLS Credentials:

   A `tls.Config` can also be provided directly to the client. This method gives advanced API users the freedom to manage credentials themselves, but is not as straightforward and easy to use out of the box.

   This may be useful in special situations, such as if multiple Teleport API client's are running and want to share a single `tls.Config`.

   Note that this method is not recommended for the average use case, but some users may find it useful to have this additional level of control over the client's TLS configuration.

This demo sets up a few credentials which can be easily tried out in the demo below.

### Authorization

The server will authorize requests for the user associated with the certificates used to authenticate the client. 

Therefore, to use the API client, you need to create a user and any roles it may need for your use case. The client will act on behalf of that user and have access as defined by the user's role(s).

### Run the Demo

1. Create the `access-admin` user and role using the commands below.

```bash
$ tctl create -f access-admin.yaml
$ tctl users add access-admin --roles=access-admin
```

2. Replace the address `localhost:3025` in `main.go` with the local address of the auth server to connect to. If the client is using identity file credentials, it can also connect through SSH to a proxy or web proxy address. 

3. Create credentials to authenticate the client. And follow the steps below corresponding to the credentials chosen. Multiple credentials can be specified and the first credentials to successfully load will be used. The client will only return an error if all methods fail.

   - Identity File Credentials:

      ```bash
      # login and generate identity file
      $ tsh login --user=access-admin --out=certs/access-admin-identity
      ```

   - Key Pair Credentials:

      ```bash
      $ mkdir -p certs
      # Generate long term certificates from auth server without login
      $ tctl auth sign --format=tls --ttl=8760h --user=access-admin --out=certs/access-admin
      ```

   - TLS Credentials (manual):

      Generate valid TLS certificates by whatever means desired. Use those certificates to create a `*tls.Config` and provide it into the `CredentialsList` with `client.LoadTLS(*tls.Config)`.

4. run the demo:

   ```bash
   $ go run .
   ```

To see more information on the Go Client and how to use it, visit our [API Documentation](https://goteleport.com/teleport/docs/api-reference/).