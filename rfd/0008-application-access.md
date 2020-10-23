---
authors: Russell Jones (rjones@gravitational.com)
state: draft
---

# RFD 8 - Teleport Application Access

## What

This document contains technical implementation details of Teleport Application Access.

## Why

## Use Cases

The initial implementation of Teleport Application Access is targeted at users that would like to expose internal applications and dashboards on the public internet.

## Details

### Identity Headers

As described in a previous section, Teleport uses TLS mutual authentication to pass identity information between internal components. However, identity information is passed to proxied applications in the form of a signed JWT in a request header named `teleport-jwt-assertion`.

This identity information can be used to show the identity of the user currently logged in as well as change the state of the internal application. For example, because Teleport roles are forwarded to proxied applications within the JWT header, an control panel application could show an regular or admin view based on the Teleport identity of the user.

#### Issuance

All Teleport clusters have a User and Host CA used to issue user and host SSH and TLS certificates. Teleport Application Access introduces a JWT signer to each cluster to issue JWTs. The JWT signer uses 2048-bit RSA keys similar to the existing CAs.

#### Verification

An unauthenticated endpoint will be added at https://proxy.example.com:3080/.well-known/jwks.json endpoint which returns the public keys that can be used to verify the signed JWT. Multiple keys are supported because JWT signers can be rotated similar to CAs.

Many sources exist that explain the JWT signature scheme and how to verify a JWT. Introduction to JSON Web Tokens is a good starting point for general JWT information.

However, we strongly recommend you use a well written and supported library in the language of your choice to validate the JWT and you not try to write parsing and validation code yourself. We have provided an example within Teleport on how to validate the JWT token written in Go.

#### Claims

The JWT embeds within it claims about the identity of the subject and issuer of the token.

The following public claims are included:

* `aud`: Audience of JWT. This is the URI of the proxied application to which the request is being forwarded.
* `exp`: Expiration time of the JWT. This value is always in sync with the expiration of the TLS certificate.
* `iss`: Issuer of the JWT. This value is the name of the Teleport cluster issuing the token.
* `nbf`: Not before time of the JWT. This is the time at which the JWT becomes valid.
* `sub`: Subject of the JWT. This is the Teleport identity of the user to whom the JWT was issued.

The following private claims are included.

* `username`: Similar to sub. This is the Teleport identity of the user to whom the JWT was issued.
* `roles`: List of Teleport roles assigned to the user.

#### Rotation

The JWT signing keys are rotated along with User and Host CAs when using the `tctl auth rotate [...]` command. If you specifically only want to rotate your JWT signer, use the `--type=jwt` flag.

#### Example

The following header will be sent to an internal application:

```
teleport-jwt-assertion: eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.eyJhdWQiOlsiaHR0cDovL2xvY2FsaG9zdDo4MDgwL2FwcCJdLCJleHAiOjE2MDM1Mjk5NzEsImlzcyI6ImV4YW1wbGUuY29tIiwibmJmIjoxNjAzNDg2Nzg2LCJyb2xlcyI6WyJhZG1pbiJdLCJzdWIiOiJyam9uZXMiLCJ1c2VybmFtZSI6InJqb25lcyJ9.SnyYMyjjcxEUsPnf-WxWy33yVWsHR3hQPCml-fizX1HJY7jojkKroPbrXBCO-WEJ8RqCzv0j6u1pz_PllNPhhPrCE8Q32WAB2OaazVM2FHxsiEVyMInCUVEAsrieYo0BQidXTj85yGgEPV45VdbnqWdJSzVr1UmUF6kDdMwhS3Zyr-SRAZw9ix_jBK6nxDmlD0TgJh9eAvRhbjvxU12I6A4VqZVPTrefoWsdZTHrYvg2oqztHtNSycqsbqfIBnNmg__opWKgouW_t-Xv58aA8scW_5DavVitPhQBbsPH0QRKfu-xMNDtmfa6eBAKe7E9uO2uDcmDA26dHKIA2n90Gw
```

Decoding to the below JSON.

```
{
  "aud": [
    "http://localhost:8080/app"
  ],
  "exp": 1603530049,
  "iss": "example.com",
  "nbf": 1603486842,
  "roles": [
    "admin"
  ],
  "sub": "foo",
  "username": "foo"
}
```

### Logout

Each application the user launches maintains its own session. Sessions automatically TTL out after the time specified on the role and certificate.

To explicitly logout a session, an authenticated session can issue a `GET /teleport-logout` or a `DELETE /teleport-logout` request.

Internal applications and implementers are encouraged to support `DELETE /teleport-logout` in the form of a logout button within the internal application.

The `GET /teleport-logout` endpoint is for internal applications that can not be modified. For example, you may go to `https://acme.example.com/teleport-logout` to log out of the ACME application.

## Configuration

### Server

Teleport server configuration has been updated to add file, process, and role configuration.

#### File Configuration

The `auth_service` section has been updated to support `app` type of static token.

```yaml
auth_service:
  enabled: yes

  tokens:
    # Defines a static token that can be used for application services
    # joining a cluster.
    - app:F6CA0D839B9B691FF62FDC31FAF5F7E5

    # Defines a static token that can be used for application and SSH
    # services joining a cluster.
    - node,app:4FF2791E11C596BF33A73A8A5BD50415
```

The `proxy_service` section has been updated to support loading multiple certificate and key pairs. This allows loading per-application TLS certificates as well as loading of wildcard TLS certificates. Also note that because the application service uses reverse tunnels, having a valid `tunnel_public_addr` is required.

```yaml
proxy_service:
  # Tunnel public address defines the address the application service will
  # attempt to connect to to establish the reverse tunnel. 
  tunnel_public_addr: "proxy.example.com:3024"

  # List of certificates for the proxy to load.
  https_keypairs:
    - key_file: /var/lib/teleport/certs/proxy.example.com+3-key.pem
      cert_file: /var/lib/teleport/certs/proxy.example.com+3.pem
    - key_file: /var/lib/teleport/certs/app.example.com+3-key.pem
      cert_file: /var/lib/teleport/certs/app.example.com+3.pem
```

An `app_service` section has been added to configure the application proxy service.

```yaml
# An application service establishes a reverse tunnel to the proxy which
# is used both to heartbeat the presence of the application as well as
# used to establish connections through.
app_service:
   # Enabled controls if the application service is enabled.
   enabled: true

   # A list of applications that will be proxied.
   apps:
     # Name of the application.
   - name: jenkins
     # URI is the address the application being proxied can be reached at.
     uri: http://localhost:8080
     # Public address is used to override the default address the application
     # is avaiable at.
     public_addr: jenkins.acme.teleport.dev
     # Insecure skip verify is used to disable server TLS certificate
     # verification. Useful for internal applications using a self signed
     # certificate.
     insecure_skip_verify: true
     # Rewriting rules that get applied to every request.
     rewrite:
        # Rewrite the "Location" header on redirect "30{1-8}"   
        # responses replacing the host with the public address of
        # this application.   
        redirect:
           - "localhost"
           - "jenkins.internal.dev"
     # Static labels to assign to this application.
     labels:
        key: value
     # Dynamic labels to assign to this application.
     commands:
     - name: "arch"
       command: ["/bin/uname", "-p"]
       period: "1h0m0s"
```

#### Command Line Flags

The `teleport` process has been updated to support three additional flags that can be used to configure an application service in addition to the existing `--roles` and `--labels` flag (which now apply to the application, not the service).

```
$ teleport start \
   # Define the role of service, in this case "app_service".
   --roles="app" \
   # Define the token the service will use to join the cluster.
   --token=D83721493A6BE34862FAD8C9FFBDADA7 \
   # Define any static or dynamic labels to apply to the application.
   --labels="foo=bar,baz=qux" \
   # Define the name of the application.
   --app-name="jenkins" \
   # Define the URI that the application is running at.
   --app-uri="http://localhost:8080" \
   # Define the public address, used to overide the automatically generated
   # address of appName.proxyPublicAddr.
   --app-public-addr="jenkins.acme.teleport.dev"
```

#### Roles

An additional field `app_labels` has been added to both the `allow` and `deny` section of a role. `app_labels` behaves similarly to `node_labels`.

```yaml
kind: "role"
version: "v3"
spec:
  options:
    forward_agent: true
    port_forwarding: false
  allow:
    logins: ["rjones"]
    # Application labels define labels that an application must match for this
    # role to be allowed access to it.
    app_labels:
      "*": "staging"
```

### Clients

Teleport client configuration has been updated to add subcommands to both `tctl` and `tsh`.

#### `tctl`

`tctl` has been updated to support creating ephemeral application join tokens. Specifying the `--type=app` flag will create a ephemeral join token and fill out the name and address of the application with dummy values. If you specify the `--app-name` and `--app-uri` these fields will be filled out with the passed in values.

```
$ ./tctl.sh tokens add --type=app
The invite token: 98071b6d461cfa59a410e58c2ae68ea6
This token will expire in 60 minutes

Fill out and run this command on a node to make the application available:

> teleport start \
   --roles=app \
   --token=98071b6d461cfa59a410e58c2ae68ea6 \
   --ca-pin=sha256:14b84254ac3a30cf31906326e0f7800d7fe54b4e23cef8e84d04af0bb3d71962 \
   --auth-server=proxy.example.com:3080 \
   --app-name=example-app                    \ # Change "example-app" to the name of your application.
   --app-uri=http://localhost:8080             # Change "http://localhost:8080" to the address of your application.

Your application will be available at example-app.proxy.example.com:3080.

Please note:

  - This invitation token will expire in 60 minutes.
  - proxy.example.com:3080 must be reachable from the new application service.
  - Update DNS to point example-app.proxy.example.com:3080 to the Teleport proxy.
  - Add a TLS certificate for example-app.proxy.example.com:3080 to the Teleport proxy under "https_keypairs".
```

`tctl` has also been updated to add the `apps` subcommand which can be used to show a list of all applications registered with the cluster.

```
$ tctl apps ls
Application Host                                 Public Address           URI                    Labels
----------- ------------------------------------ ------------------------ ---------------------- ------
dumper      1f54c61d-0edb-4dd7-89a3-e291a031d903 dumper.proxy.example.com http://127.0.0.1:8080
jenkins     a52723a4-e852-4467-b756-18c2978367b3 jenkins.proxy.example.com http://127.0.0.1:8081
```

#### `tsh`

`tsh` has been updated to add the `apps` subcommand which is used to display a list of all running applications. Only applications the user has access to are shown.

```
$ tsh apps ls 
Application Public Address           Labels
----------- ------------------------ ------
dumper      dumper.proxy.example.com
```

The `-v` flag has also been added to show the list of applications in long form.

```
$ tsh apps ls -v
Application Host                                 Public Address           URI                    Labels
----------- ------------------------------------ ------------------------ ---------------------- ------
dumper      1f54c61d-0edb-4dd7-89a3-e291a031d903 dumper.proxy.example.com http://127.0.0.1:8080
```
