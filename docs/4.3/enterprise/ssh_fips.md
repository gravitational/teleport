# FedRAMP / FIPS
With Teleport 4.0 we have built the foundation to meet FedRAMP requirements for
the purposes of accessing infrastructure. This includes support for [FIPS 140-2](https://en.wikipedia.org/wiki/FIPS_140-2), also known as the Federal Information Processing Standard, which is the US government approved standard for cryptographic modules. This document outlines a high
level overview of how Teleport FIPS mode works and how it can help your company to become FedRAMP certified.


### Obtain FedRAMP certification with Teleport
Teleport includes new FedRAMP and FIPS 140-2 features to support companies that sell into
government agencies.

| Control  | Teleport Features |
|----------|---------------------|
| [AC-03 Access Enforcement](https://nvd.nist.gov/800-53/Rev4/control/AC-3)   | Teleport Enterprise supports robust [Role-based Access Controls (RBAC)](./ssh_rbac.md) to: <br>• Control which SSH nodes a user can or cannot access. <br>• Control cluster level configuration (session recording, configuration, etc.) <br>• Control which UNIX logins a user is allowed to use when logging into a server. |
| [AC-17 Remote Access](https://nvd.nist.gov/800-53/Rev4/control/AC-17)   | Teleport administrators create users with configurable roles that can be used to allow or deny access to system resources. |
| [AC-20 Use of External Information Systems](https://nvd.nist.gov/800-53/Rev4/control/AC-20)  | Teleport supports connecting multiple independent clusters using a feature called [Trusted Clusters](../trustedclusters/). When allowing access from one cluster to another, roles are mapped according to a pre-defined relationship of the scope of access.|
| [AU-03 Audit and Accountability](https://nvd.nist.gov/800-53/Rev4/control/AU-3) – Content of Audit Records and [AU-12 Audit Generation](https://nvd.nist.gov/800-53/Rev4/control/AU-12) | Teleport contains an [Audit Log](../architecture/teleport_auth/#audit-log) that records cluster-wide events such as: <br>• Failed login attempts.<br>• Commands that were executed (SSH “exec” commands).<br> • Ports that were forwarded. <br>• File transfers that were initiated.|
| [AU-10 Non-Repudiation](https://nvd.nist.gov/800-53/Rev4/control/AU-10)  | Teleport audit logging supports both events as well as audit of an entire SSH session. For non-repudiation purposes a full session can be replayed back and viewed.  |
| [CM-08 Information System Component Inventory](https://nvd.nist.gov/800-53/Rev4/control/CM-8)  | Teleport maintains a live list of all nodes within a cluster. This node list can be queried by users (who see a subset they have access to) and administrators any time.|
| [IA-03 Device Identification and Authentication](https://nvd.nist.gov/800-53/Rev4/control/IA-3)  | Teleport requires valid x509 or SSH certificates issued by a Teleport Certificate Authority (CA) to establish a network connection for device-to-device network connection between Teleport components. |
| [SC-12 Cryptographic Key Establish and Management](https://nvd.nist.gov/800-53/Rev4/control/SC-12)  | Teleport initializes cryptographic keys that act as a Certificate Authority (CA) to further issue x509 and SSH certificates. SSH and x509 user certificates that are issued are signed by the CA and are (by default) short-lived. SSH host certificates are also signed by the CA and rotated automatically (a manual force rotation can also be performed).<br>Teleport Enterprise builds against a FIPS 140-2 compliant library (BoringCrypto) is available. <br>In addition, when Teleport Enterprise is in FedRAMP/FIPS 140-2 mode, Teleport will only start and use FIPS 140-2 compliant cryptography. |
| [AC-2 Account Management](https://nvd.nist.gov/800-53/Rev4/control/AC-2) | Audit events are emitted in the auth server when a user is created, updated, deleted, locked or unlocked.  |
| [AC-2 (12) Account Management](https://nvd.nist.gov/800-53/Rev4/control/AC-2) | At the close of a connection the total data transmitted and received is emitted to the Audit Log. |


Enterprise customers can download the custom FIPS package from the [Gravitational Dashboard](https://dashboard.gravitational.com/web/).  Look for `Linux 64-bit (FedRAMP/FIPS)`. RPM and DEB packages are also available.

# Setup
Customers can follow our [Enterprise Quickstart](quickstart-enterprise.md) for basic
instructions on how to setup Teleport Enterprise. You'll need to start with the Teleport
Enterprise FIPS Binary.

After downloading the binary tarball, run:

```bsh
$ tar -xzf teleport-ent-v{{ teleport.version }}-linux-amd64-fips-bin.tar.gz
$ cd teleport-ent
$ sudo ./install
# This will copy Teleport Enterprise to /usr/local/bin.
```
## Configuration

### Teleport Auth Server
Now, save the following configuration file as `/etc/teleport.yaml` on the auth server.

```yaml
teleport:
  auth_token: zw6C82kq7VEUSJeSDzuldWsxakql6jrTYmphxRQOlrATTGbLQoaIwEBo48o9
  # Pre-defined tokens for adding new nodes to a cluster. Each token specifies
  # the role a new node will be allowed to assume. The more secure way to
  # add nodes is to use `ttl node add --ttl` command to generate auto-expiring
  # tokens.
  #
  # We recommend to use tools like `pwgen` to generate sufficiently random
  # tokens of 32+ byte length.
  # you can also use auth server's IP, i.e. "10.1.1.10:3025"
  auth_servers: [ "10.1.1.10:3025" ]

auth_service:
  # enable the auth service:
  enabled: true

  tokens:
  # this static token is used for other nodes to join this Teleport cluster
  - proxy,node:zw6C82kq7VEUSJeSDzuldWsxakql6jrTYmphxRQOlrATTGbLQoaIwEBo48o9
  # this token is used to establish trust with other Teleport clusters
  - trusted_cluster:TaZff3DLbpsMZmIMhvEr7kulOgegjg7yyQNTS0q6UFWfsJ9N6rxVBjg6t7nw

  # To Support FIPS local_auth needs to be turned off and a SSO connector is
  # required to log into Teleport.
  authentication:
    # local_auth needs to be set to false in FIPS mode.
    local_auth: false
    type: saml

  # If using Proxy Mode, Teleport requires host key checks.
  # This setting needs is required to start in Teleport in FIPS mode
  proxy_checks_host_keys: true

  # SSH is also enabled on this node:
ssh_service:
  enabled: false
```

### Teleport Node

Save the following configuration file as `/etc/teleport.yaml` on the node server.
```yaml
teleport:
  auth_token: zw6C82kq7VEUSJeSDzuldWsxakql6jrTYmphxRQOlrATTGbLQoaIwEBo48o9

  auth_servers: [ "10.1.1.10:3025" ]

  # enable ssh service and disable auth and proxy:
ssh_service:
  enabled: true

auth_service:
  enabled: false
proxy_service:
  enabled: false
```

### Systemd Unit File

Next, download the systemd service unit file from the [examples directory](https://github.com/gravitational/teleport/tree/master/examples/systemd/fips)
on Github and save it as `/etc/systemd/system/teleport.service` on both servers.

```bsh
# run this on both servers:
$ sudo systemctl daemon-reload
$ sudo systemctl enable teleport
```

### Starting Teleport in FIPS mode.

When using `teleport start --fips`, Teleport will start in FIPS mode.

Teleport will configure the TLS and SSH servers with FIPS compliant
cryptographic algorithms.  In FIPS mode, if non-compliant algorithms are
chosen, Teleport will fail to start.  In addition, Teleport checks if the
binary was compiled against an approved cryptographic module (BoringCrypto) and
fails to start if it was not.

* For OSS and Enterprise binaries not compiled with BoringCrypto, this flag will report that this version of Teleport is not compiled with the appropriate cryptographic module.

* Running commands like `ps aux` can be useful to note that Teleport is running in FedRAMP enforcing mode.

* If no ciphersuites are provided, Teleport will set the default ciphersuites to be FIPS 140-2 compliant.

* If ciphersuites, key exchange and MAC algorithms are provided in the Teleport configuration, Teleport will validate that they are FIPS 140-2 compliant..

* Teleport will always enable at-rest encryption for both DynamoDB and S3.

* If recording proxy mode is selected, validation of host certificates should always happen.


### FedRAMP Audit Log

At the close of a connection (close of a *srv.ServerContext) the total data transmitted and received
is emitted to the Audit Log.

## What else does the Teleport FIPS binary enforce?
* Supporting configurable TLS versions. This is to ensure that only TLS 1.2 is supported in FedRAMP mode.
* Removes all uses of non-compliant algorithms like NaCl and replace with compliant algorithms like AES-GCM.
* Teleport is compiled  with [BoringCrypto](https://csrc.nist.gov/projects/cryptographic-module-validation-program/Certificate/2964)
* User, host and CA certificates (and host keys for recording proxy mode) should only use 2048-bit RSA private keys.
