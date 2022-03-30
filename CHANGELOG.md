# Changelog

## 8.3.6

This release of Teleport contains multiple fixes.

* Fixed issue with message of the day not being displayed in some cases.
  [#11371](https://github.com/gravitational/teleport/pull/11371)
* Fixed issue with automatic node join script returning 404 in web UI.
  [#11572](https://github.com/gravitational/teleport/pull/11572)
* Fixed issue with tsh proxy jump not connecting to leaf proxy.
  [#11497](https://github.com/gravitational/teleport/pull/11497)

## 8.3.5

This release of Teleport contains multiple features, improvements and fixes.

* Added `HTTP_PROXY` support to tsh. [#10209](https://github.com/gravitational/teleport/pull/10209)
* Added support for per-user `tsh` configuration preferences. [#10336](https://github.com/gravitational/teleport/pull/10336)
* Added automatic node joining wizard to OSS. [#10288](https://github.com/gravitational/teleport/pull/10288)
* Improved Desktop Access performance by fixing memory leaks and bitmap optimizations. [#10915](https://github.com/gravitational/teleport/pull/10915)
* Improved Desktop Access to support proxying to different desktops. [#10101](https://github.com/gravitational/teleport/pull/10101)
* Improved Application Access HA behavior when accessing applications within a leaf cluster. [#10734](https://github.com/gravitational/teleport/pull/10734)
* Improved Database Access log spam and automatic discovery. [#11020](https://github.com/gravitational/teleport/pull/11020) [#10699](https://github.com/gravitational/teleport/pull/10699)
* Improved error messages when host is missing in `tctl auth sign`. [#10588](https://github.com/gravitational/teleport/pull/10588)
* Improved X11 forwarding support on macOS. [#10719](https://github.com/gravitational/teleport/pull/10719)
* Fixed multiple issues with CA rotation, graceful restart, and stability. [#10706](https://github.com/gravitational/teleport/pull/10706) [#11074](https://github.com/gravitational/teleport/pull/11074) [#11283](https://github.com/gravitational/teleport/pull/11283)
* Fixed an issue where users could create system roles. [#8924](https://github.com/gravitational/teleport/pull/8924)
* Fixed an issue where an invalid event could lead to the Audit Log being inaccessible to view. [#10665](https://github.com/gravitational/teleport/pull/10665)
* Fixed an issue with lease contention and concurrent session control. [#10666](https://github.com/gravitational/teleport/pull/10666)
* Fixed an issue where Teleport could panic during a session recording. [#10792](https://github.com/gravitational/teleport/pull/10792)
* Fixed an issue where `tctl auth sign` was creating a `kubeconfig` file incompatible with Teleport Cloud. [#10844](https://github.com/gravitational/teleport/pull/10844)
* Fixed an issue where Teleport would not regenerate server identity for Kubernetes Access. [#10904](https://github.com/gravitational/teleport/pull/10904)
* Fixed an issue where `tsh` would not deduplicate Access Request IDs. [#9453](https://github.com/gravitational/teleport/pull/9453)
* Fixed an issue where `tsh` would not respect `TELEPORT_HOME` [#11087](https://github.com/gravitational/teleport/pull/11087)
* Fixed an issue where `tsh aws ecr` could return `Internal Server`. [#10475](https://github.com/gravitational/teleport/pull/10475)
* Fixed an memory leak in the Teleport watcher system. [#10871](https://github.com/gravitational/teleport/pull/10871)
* Fixed an issue where certain resources could not be deleted. [#11124](https://github.com/gravitational/teleport/pull/11124)

## 8.3.4

This release of Teleport contains multiple improvements and fixes.

* Fixed utmp accounting on some systems. [#10617](https://github.com/gravitational/teleport/pull/10617)
* Fixed an issue with DynamoDB pagination when result set exceeds 1MB. [#10847](https://github.com/gravitational/teleport/pull/10847)
* Improved join instructions printed by `tctl` when using Teleport Cloud. [#10749](https://github.com/gravitational/teleport/pull/10749)
* Improved HA behavior of database agents in leaf clusters. [#10770](https://github.com/gravitational/teleport/pull/10770)
* Fixed an issue with .deb packages not being published. [#10806](https://github.com/gravitational/teleport/pull/10806)
* Fixed an issue with session uploader leaving empty directories behind in some cases. [#10793](https://github.com/gravitational/teleport/pull/10793)

## 8.3.3

This release of Teleport contains a security fix and multiple improvements and fixes.

### Trusted Clusters security fix

An attacker in possession of a valid Trusted Cluster join token could inject a
malicious CA into a Teleport cluster that would allow them to bypass root
cluster authorization and potentially connect to any node within the root
cluster.

For customers using Trusted Clusters, we recommend upgrading to one of the
patched releases listed below then revoking and rotating all Trusted Cluster
tokens. As a best practice, make sure that Trusted Cluster tokens have short
time-to-live and ideally are removed after being used once.

### Other fixes

* Fixed dynamic labeling for Kubernetes agents. [#10464](https://github.com/gravitational/teleport/pull/10464)
* Added `teleport_audit_emit_event` and `teleport_connected_resources` Prometheus metrics. [#10462](https://github.com/gravitational/teleport/pull/10462), [#10461](https://github.com/gravitational/teleport/pull/10461)
* Fixed an issue with serving multiple concurrent X11 forwarding sessions. [#10473](https://github.com/gravitational/teleport/pull/10473)
* Fixed a misnaming in the X11 forwarding configuration file options. [#10758](https://github.com/gravitational/teleport/pull/10758)
* Fixed an issue with MongoDB connections not being properly closed. [#10730](https://github.com/gravitational/teleport/pull/10730)
* Clear terminal at the end of the session in FIPS mode. [#10533](https://github.com/gravitational/teleport/pull/10533)

## 8.3.1

This release of Teleport contains an improvement and fix.

* Added additional Prometheus metrics for cache and event monitoring. [#9826](https://github.com/gravitational/teleport/pull/9826)
* Fixed an issue with user home directory checking. [#10321](https://github.com/gravitational/teleport/pull/10321)

## 8.3.0

This release of Teleport contains new features, improvements, and fixes.

* Added IAM support for [Joining Nodes and Proxies in AWS](https://goteleport.com/docs/setup/guides/joining-nodes-aws/). [#8690](https://github.com/gravitational/teleport/pull/8690) [#10085](https://github.com/gravitational/teleport/pull/10085) [#10087](https://github.com/gravitational/teleport/pull/10087)
* Added GitHub team information to claims for [GitHub SSO](https://goteleport.com/docs/setup/admin/github-sso/). [#9604](https://github.com/gravitational/teleport/pull/9604)
* Added the `cert.create` event. [#9822](https://github.com/gravitational/teleport/pull/9822)
* Updated `tsh ls` output to truncate labels. [#9589](https://github.com/gravitational/teleport/pull/9589)
* Updated smart card PIN generation to generate a random PIN for each desktop session, preventing the smart card from being used after the initial login. [#9919](https://github.com/gravitational/teleport/pull/9919)
* Fixed an issue that could cause the audit logger to crash. [#10254](https://github.com/gravitational/teleport/pull/10254)
* Fixed an issue with `tctl --insecure` and TLS routing. [#10297](https://github.com/gravitational/teleport/pull/10297)
* Fixed an issue where reverse tunnels would not properly reconnect. [#10368](https://github.com/gravitational/teleport/pull/10368)

## 8.2.0

This release of Teleport contains a new feature.

* Added support for X11 forwarding to Server Access. [#9897](https://github.com/gravitational/teleport/pull/9897)

## 8.1.5

This release of Teleport contains a fix.

* Fixed an issue impacting clusters that upgraded to 8.1.3 that broke the Web UI and audit log search functionality. [#10193](https://github.com/gravitational/teleport/pull/10193)

## 8.1.4

This release of Teleport contains a few improvements and fixes.

* Rolled back `session.connect` event. [#10156](https://github.com/gravitational/teleport/pull/10156)
* Add new `teleport_build_info` Prometheus metric. [#10135](https://github.com/gravitational/teleport/pull/10135)
* Improvements to dynamically resolving tunnel address in reverse tunnel agents. [#10139](https://github.com/gravitational/teleport/pull/10139)

## 8.1.3

This release of Teleport contains multiple features, improvements, bug fixes, and a security fix.

### Kubernetes Access security fix

* Fixed issue where labels of the target Kubernetes Service were ignored when calculating `kubernetes_users` and `kubernetes_groups`. [#9955](https://github.com/gravitational/teleport/pull/9955)

We recommend all Kubernetes Access users to upgrade their Proxies and Kubernetes Services.

### Other improvements and fixes

* Added support for locking Access Requests. [#9478](https://github.com/gravitational/teleport/pull/9478)
* Added support for jitter and backoff to prevent thundering herd situations. [#9133](https://github.com/gravitational/teleport/pull/9133)
* Added support for nested groups with Google SSO. [#9697](https://github.com/gravitational/teleport/pull/9697)
* Added support for pulling multiple domain groups from Google Workspace. [#9697](https://github.com/gravitational/teleport/pull/9697)
* Added event `session.connect` which is emitted when connecting to a non-Teleport server. [#9370](https://github.com/gravitational/teleport/pull/9370)
* Added Access Request information to audit events. [#9758](https://github.com/gravitational/teleport/pull/9758)
* Added client certificate authentication support for GCP Cloud SQL [#9991](https://github.com/gravitational/teleport/pull/9991)
* Added support for canned AWS S3 ACLs. [#9042](https://github.com/gravitational/teleport/pull/9042)
* Improved ACME support to automatically renew certificates affected by the Let's Encrypt TLS-ALPN-01 issues. [#9984](https://github.com/gravitational/teleport/pull/9984)
* Improved Desktop Access performance. [#9817](https://github.com/gravitational/teleport/pull/9817)
* Improved network utilization by replacing cluster periodics with watchers. [#9609](https://github.com/gravitational/teleport/pull/9609)
* Fixed reverse tunneling for Windows Desktop Connections. [#9740](https://github.com/gravitational/teleport/pull/9740)
* Fixed issue where database auto-discovery could fail with databases created by CloudFormation. [#9742](https://github.com/gravitational/teleport/pull/9742)
* Fixed issue with Application Access in High Availability (HA) configurations. [#9288](https://github.com/gravitational/teleport/pull/9288)
* Fixed issue where Database Access could fail to connect to RDS instance in `ca-central-1`. [#9890](https://github.com/gravitational/teleport/pull/9890)
* Fixed issue with auto-discovery and RDS or Aurora permissions. [#9426](https://github.com/gravitational/teleport/pull/9426)
* Fixed issue with Desktop Access token type name inconsistencies. [#9756](https://github.com/gravitational/teleport/pull/9756)
* Fixed issue where prefixing an application name with "kube" would make the proxy route it as a Kubernetes cluster. [#9777](https://github.com/gravitational/teleport/pull/9777)
* Fixed issue where `tsh db ls` could show incorrect information. [#9386](https://github.com/gravitational/teleport/pull/9386)
* Fixed issue where Database Access would not register Aurora reader instances. [#9668](https://github.com/gravitational/teleport/pull/9668)
* Fixed issue with AWS credential brokering with federated accounts. [#9792](https://github.com/gravitational/teleport/pull/9792)
* Fixed regression in Kubernetes Access performance introduced in Teleport 8.1.1. [#10011](https://github.com/gravitational/teleport/pull/10011)
* Fixed an issue where OIDC UserInfo were not respected. [#9951](https://github.com/gravitational/teleport/pull/9951)

## 8.1.1

This release of Teleport contains a feature, improvement, and fixes.

* Added the `access_request.delete` event to track deleted Access Requests. [#9552](https://github.com/gravitational/teleport/pull/9552) 
* Improved Kubernetes Access performance by forcing the use of http2. [#9294](https://github.com/gravitational/teleport/pull/9294)
* Fixed an issue where `tsh kube login` would not respect `TELEPORT_HOME`. [#9760](https://github.com/gravitational/teleport/pull/9760)
* Fixed an issue where EC2 node could fail if two nodes shared a nodename. [#9722](https://github.com/gravitational/teleport/pull/9722)
* Fixed an issue where login would fail if the users home directory does not exist. [#9413](https://github.com/gravitational/teleport/pull/9413)

## 8.1.0

This release of Teleport contains features and fixes.

* Added RBAC for sessions. It is now possible to further limit access to [shared sessions](https://goteleport.com/docs/server-access/guides/tsh/#sharing-sessions) and [session recordings](https://goteleport.com/docs/architecture/nodes/#session-recording). See the [RBAC for sessions](https://goteleport.com/docs/access-controls/reference/#rbac-for-sessions) documentation for more details.
* Added ability to specify level of TLS verification for database connections. [#9197](https://github.com/gravitational/teleport/pull/9197)
* Added `--cluster` and `--diag_addr` to `tsh db` and `teleport` respectively. [#9220](https://github.com/gravitational/teleport/pull/9220)
* Fixed an issue with user specification with `tsh db connect` and MongoDB. [#9196](https://github.com/gravitational/teleport/pull/9196)
* Fixed an issue when connecting to an auth server over a tunnel when running in `proxy_listener_mode`. [#9498](https://github.com/gravitational/teleport/pull/9498)
* Fixed an issue with Access Requests where the request reason was not being escaped when using `tctl`. [#9381](https://github.com/gravitational/teleport/pull/9381)
* Fixed an issue where Teleport would incorrectly log `json: unsupported type: utils.Jitter`. [#9417](https://github.com/gravitational/teleport/pull/9417)
* Fixed an issue with incorrect session ID being emitted in `session.leave` events. [#9651](https://github.com/gravitational/teleport/pull/9651)
* Update `tctl lock` to allow locking a Windows Desktop. [#9543](https://github.com/gravitational/teleport/pull/9543)
* Removed the libatomic dependency: Teleport 8.1.0 will run on systems without libatomic, but note that Desktop Access will not be enabled in 32-bit ARM builds. [#9667](https://github.com/gravitational/teleport/pull/9667)

## 8.0.7

This release of Teleport contains multiple features and bug fixes.

* Added support for a configurable event TTL in DynamoDB. [#8840](https://github.com/gravitational/teleport/pull/8840)
* Added support for `tsh play -f json <ID>` [#9319](https://github.com/gravitational/teleport/pull/9319)
* Added Helm chart enhancements. [#8105](https://github.com/gravitational/teleport/pull/8105) [#8774](https://github.com/gravitational/teleport/pull/8774) [#9130](https://github.com/gravitational/teleport/pull/9130) [#9263](https://github.com/gravitational/teleport/pull/9263) [#9349](https://github.com/gravitational/teleport/pull/9349) [#9503](https://github.com/gravitational/teleport/pull/9503)
* Fixed an issue with TLS Routing that would cause Teleport to not respect `NO_PROXY`. [#9287](https://github.com/gravitational/teleport/pull/9287)
* Fixed an issue with Database Access where running `show tables` MySQL would result in an error. [#9411](https://github.com/gravitational/teleport/pull/9411)
* Fixed an issue with Server Access where a null route would cause high latency when connecting to hosts. [#9254](https://github.com/gravitational/teleport/pull/9254)
* Fixed an issue with Database Access that would cause the Web UI to fail to list databases. [#9096](https://github.com/gravitational/teleport/pull/9096)
* Fixed a goroutine leak in Application Access. [#9332](https://github.com/gravitational/teleport/pull/9332)
* Fixed potentially short reads from the system random number generator. [#9186](https://github.com/gravitational/teleport/pull/9186)
* Fixed RPM repository compatibility issues for CentOS 7 users. [#9464](https://github.com/gravitational/teleport/pull/9464)
* Fixed issue with Kubernetes Access and CA rotation. [#9418](https://github.com/gravitational/teleport/pull/9418)

## 8.0.6

This release of Teleport contains a feature and bug fixes.

* Added ability to run Postgres and MongoDB proxy on separate listener. [#8323](https://github.com/gravitational/teleport/pull/8323)
* Fixed an issue that could cause search engine crawlers to break signup and login pages.
* Fixed issue that would cause `tsh login` to hang indefinitely. [#9193](https://github.com/gravitational/teleport/pull/9193)

## 8.0.5

This release of Teleport contains a bug fix.

* Fixed issue with desktop access smart card authentication.

## 8.0.4

This release of Teleport contains multiple security fixes.

### Security fixes

As part of a routine security audit of Teleport, several security
vulnerabilities and miscellaneous issues were discovered. Below are the issues
found, their impact, and the components of Teleport they affect.

#### Insufficient authorization check in self-hosted MySQL database access

Teleport MySQL proxy engine did not handle internal MySQL protocol command that
allows to reauthenticate the active connection.

This could allow an attacker with a valid client certificate for a particular
database user to reauthenticate as a different MySQL user created using
`require x509` clause.

#### Insufficient authorization check in MongoDB database access

Teleport MongoDB proxy engine did not implement processing for all possible
MongoDB wire protocol messages.

This could allow an attacker with a valid client certificate to connect to the
database in a way that would prevent Teleport from enforcing authorization
check on the database names.

#### Authorization bypass in application access

When proxying a websocket connection, Teleport did not check for a successful
connection upgrade response from the target application.

In scenarios where Teleport proxy is located behind a load balancer, this could
result in the load balancer reusing the cached authenticated connection for
future unauthenticated requests.

#### Actions

For Database Access users we recommend upgrading database agents that handle
connections to self-hosted MySQL servers and MongoDB clusters.

For Application Access users we recommend upgrading application agents.

Upgrades should follow the normal Teleport upgrade procedure:
https://goteleport.com/teleport/docs/admin-guide/#upgrading-teleport.

## 8.0.1

This release of Teleport contains multiple improvements, bug fixes and a security fix.

* Mitigated [CVE-2021-43565](https://groups.google.com/g/golang-announce/c/2AR1sKiM-Qs) by updating golang.org/x/crypto. [#9203](https://github.com/gravitational/teleport/pull/9203)
* Desktop Access: Windows Desktop discovery can now be customized by specifying a base DN and LDAP filters to search. [#9201](https://github.com/gravitational/teleport/pull/9201)
* Added Azure PostgreSQL and MySQL managed identity authentication support to database access. [#9185](https://github.com/gravitational/teleport/pull/9185)
* Added support for RBAC "where" condition for active sessions. [#9076](https://github.com/gravitational/teleport/pull/9076)
* Added hint to `tsh` that MFA is not supported on Windows. [#9198](https://github.com/gravitational/teleport/pull/9198)
* Fixed an issue with long redirect URLs causing `tsh login` to fail. [#8980](https://github.com/gravitational/teleport/pull/8980)
* Fixed Okta SAML authentication issues when email address contains `+` sign. [#8396](https://github.com/gravitational/teleport/pull/8396)
* Added application metadata to application access audit events. [#9056](https://github.com/gravitational/teleport/pull/9056)
* Fixed an issue with malformed MySQL client handshake messages crashing proxy. [#9162](https://github.com/gravitational/teleport/pull/9162)
* Added support for `--cert-file`, `--key-file` and `--public-addr` to `teleport configure` command. [#9049](https://github.com/gravitational/teleport/pull/9049)
* Made sure reverse tunnel agents reconnect to the proxy after tunnel address change. [#9043](https://github.com/gravitational/teleport/pull/9043)
* Made Teleport startup more resilient to the presence of invalid roles in the backend. [#9105](https://github.com/gravitational/teleport/pull/9105)

## 8.0.0

Teleport 8.0 is a major release of Teleport that contains new features, improvements, and bug fixes.

### New Features

#### Windows Desktop Access Preview

Teleport 8.0 includes a preview of the Windows Desktop Access feature, allowing
users passwordless login to Windows Desktops via any modern web browser.

Teleport users can connect to Active Directory enrolled Windows hosts running
Windows 10, Windows Server 2012 R2 and newer Windows versions.

To try this feature yourself, check out our
[Getting Started Guide](https://goteleport.com/docs/ver/8.0/desktop-access/getting-started/).

Review the Desktop Access design in:

- [RFD #33](https://github.com/gravitational/teleport/blob/master/rfd/0033-desktop-access.md)
- [RFD #34](https://github.com/gravitational/teleport/blob/master/rfd/0034-desktop-access-windows.md)
- [RFD #35](https://github.com/gravitational/teleport/blob/master/rfd/0035-desktop-access-windows-authn.md)
- [RFD #37](https://github.com/gravitational/teleport/blob/master/rfd/0037-desktop-access-protocol.md)

#### TLS Routing

In TLS routing mode all client connections are wrapped in TLS and multiplexed on
a single Teleport proxy port.

TLS routing can be enabled by including the following auth service configuration:

```yaml
auth_service:
  proxy_listener_mode: multiplex
  ...
```

and setting proxy configuration version to `v2` to prevent legacy listeners from
being created:

```yaml
version: v2
proxy_service:
  ...
```

#### AWS CLI

Teleport application access extends AWS console support to CLI . Users are able
to log into their AWS console using `tsh app login` and use `tsh aws` commands
to interact with AWS APIs.

See more info in the [documentation](https://goteleport.com/docs/ver/8.0/application-access/guides/aws-console/#step-8-using-aws-cli).

#### Application and Database Dynamic Registration

With dynamic registration users are able to manage applications and databases
without needing to update static YAML configuration or restart application or
database agents.

See dynamic registration guides for
[apps](https://goteleport.com/docs/ver/8.0/application-access/guides/dynamic-registration/)
and
[databases](https://goteleport.com/docs/ver/8.0/database-access/guides/dynamic-registration/).

#### RDS Automatic Discovery

With RDS auto discovery Teleport database agents can automatically discover RDS
instances and Aurora clusters in an AWS account.

See updated
[RDS guide](https://goteleport.com/docs/ver/8.0/database-access/guides/rds/) for
more information.

#### WebAuthn

WebAuthn support enables Teleport users to use modern second factor options,
including Apple FaceID and TouchID.

In addition, the Teleport Web UI includes new second factor management tools,
enabling users to configure and update their second factor devices via their
web browser.

Lastly, our UI becomes more secure by requiring an additional second factor
confirmation for certain privileged actions (editing roles for second factor
confirmation, for example).

### Improvements

* Added support for [CockroachDB](https://www.cockroachlabs.com) to Database
  Access. [#8505](https://github.com/gravitational/teleport/pull/8505)
* Reduced network utilization on large clusters during login.
  [#8471](https://github.com/gravitational/teleport/pull/8471)
* Added metrics and added the ability for `tctl top` to show network utilization
  for resource propagation.
  [#8338](https://github.com/gravitational/teleport/pull/8338)
  [#8603](https://github.com/gravitational/teleport/pull/8603)
  [#8491](https://github.com/gravitational/teleport/pull/8491)
* Added support for account recovery and cancellation.
  [#6769](https://github.com/gravitational/teleport/pull/6769)
* Added per-session MFA support to Database Access.
  [#8270](https://github.com/gravitational/teleport/pull/8270)
* Added support for profile specific `kubeconfig`.
  [#7840](https://github.com/gravitational/teleport/pull/7840)

### Fixes

* Fixed issues with web applications that utilized
  [EventSource](https://developer.mozilla.org/en-US/docs/Web/API/EventSource)
  with Application Access.
  [#8359](https://github.com/gravitational/teleport/pull/8359)
* Fixed issue were interactive sessions would always return exit code 0.
  [#8081](https://github.com/gravitational/teleport/pull/8081)
* Fixed issue where JWT signer was omitted from bootstrap logic.
  [#8119](https://github.com/gravitational/teleport/pull/8119)

### Breaking Changes

#### CentOS 6

CentOS 6 support will be deprecated in Teleport 8 and removed in Teleport 9.

Teleport 8 will continue to receive security patches for about 9 months after
which it will be EOL. Users are encouraged to upgrade to CentOS 7 in that time
frame.

#### Updated  dependencies

New run time dependencies have been added to Teleport 8 due to the inclusion of
Rust in the build chain. Teleport 8 requires `libgcc_s.so` and `libm.so` be
installed on systems running Teleport.

Users of [distroless](https://github.com/GoogleContainerTools/distroless)
container images are encouraged to use the
[gcr.io/distroless/cc-debian11](https://github.com/GoogleContainerTools/distroless/blob/main/examples/rust/Dockerfile)
image to run Teleport.

```
FROM gcr.io/distroless/cc-debian11
```

Alpine users are recommended to install the `libgcc` package in addition to any
glibc compatibility layer they have already been using.

```
apk --update --no-cache add libgcc
```

#### Database Access Certificates

With the `GODEBUG=x509ignoreCN=0` flag removed in Go 1.17, Database Access users
will no longer be able to connect to databases that include their hostname in
the `CommonName` field of the presented certificate. Users are recommended to
update their database certificates to include hostname in the
`Subject Alternative Name` extension instead.

Subscribe to Github issue
[#7636](https://github.com/gravitational/teleport/issues/7636) which will add
ability to control level of TLS verification as a workaround.

#### Role Changes

New clusters will no longer have the default `admin` role, it has been replaced
with 3 smaller scoped roles: `access`, `auditor`, and `editor`.

## 7.3.3

This release of teleport contains performance improvements, fixes, and a feature.

* Improved cache and label-based operations performance. [#8670](https://github.com/gravitational/teleport/pull/8670)

* Added support for custom `routing_strategy` configuration. [#8567](https://github.com/gravitational/teleport/pull/8567)

* Fixed an issue with "Simplified EC2 Join" for some regions. [#8704](https://github.com/gravitational/teleport/pull/8704)

* Fixed a regression in web terminal. [#8797](https://github.com/gravitational/teleport/pull/8797)

## 7.3.2

This release of Teleport contains a feature and a fix.

* Fixed issue that could cause `kubectl exec` to fail. [#8601](https://github.com/gravitational/teleport/pull/8601)
* Added email notification plugin, see [Teleport Plugins 7.3.1](https://github.com/gravitational/teleport-plugins/releases/tag/v7.3.0) for more details.

## 7.3.0

This release of Teleport contains a feature and a fix.

* Added ability for nodes to join cluster without needing to share secret tokens on AWS. See [Node Joining in AWS](https://goteleport.com/docs/setup/guides/joining-nodes-aws/) guide for more details. [#8250](https://github.com/gravitational/teleport/pull/8250) [#7292](https://github.com/gravitational/teleport/pull/7292)
* Fixed an issue that could cause intermittent connectivity issues for Kubernetes Access. [#8362](https://github.com/gravitational/teleport/pull/8362)

## 7.2.1

This release of Teleport contains bug fixes, features, and improvements.

* Added network and resource utilization information to `tctl top`. [#8338](https://github.com/gravitational/teleport/pull/8338)
* Fixed issue that prevented OIDC integration with Ping. [#8308](https://github.com/gravitational/teleport/pull/8308)
* Added ability for agents to connect over HTTP with `--insecure` flag. [#7835](https://github.com/gravitational/teleport/pull/7835)
* Updated CLI SSO login flow to use Javascript redirect instead of a 302 redirect to support users with high number of claims.  [#8293](https://github.com/gravitational/teleport/pull/8293)

## 7.2.0

This release of Teleport contains bug fixes and features.

* Added support for Hardware Security Modules (HSMs). [#7981](https://github.com/gravitational/teleport/pull/7981)
* Added `tsh ssh` support for Windows. [#8306](https://github.com/gravitational/teleport/pull/8306) [#8221](https://github.com/gravitational/teleport/pull/8221) [#8295](https://github.com/gravitational/teleport/pull/8295)
* Fixed regressions in graceful restart behavior of Teleport. [#8083](https://github.com/gravitational/teleport/pull/8083)
* Fixed an issue with forwarding requests to EventSource apps via application access. [#8385](https://github.com/gravitational/teleport/pull/8385)

## 7.1.3

This release of Teleport contains bug fixes, improvements, and multiple features.

* Fixed performance and stability issues for DynamoDB clusters. [#8279](https://github.com/gravitational/teleport/pull/8279)
* Fixed issue that could cause Teleport to panic when disconnecting expired certificates. [#8288](https://github.com/gravitational/teleport/pull/8288)
* Fixed issue that could cause Teleport to fail to start if unable to connect to Kubernetes cluster. [#7523](https://github.com/gravitational/teleport/pull/7523)
* Fixed issue that prevented the Web UI from loading in Safari. [#7929](https://github.com/gravitational/teleport/pull/7929)
* Improved performance for Google Firestore users. [#8181](https://github.com/gravitational/teleport/pull/8181) [#8241](https://github.com/gravitational/teleport/pull/8241)
* Added support for profile specific `kubeconfig` file. [#7840](https://github.com/gravitational/teleport/pull/7840)
* Added support to Terraform Plugin to load loading identity from environment variables instead of disk. [#8061](https://github.com/gravitational/teleport/pull/8061) [teleport-plugins#299](https://github.com/gravitational/teleport-plugins/pull/299)

## 7.1.2

This release of Teleport contains multiple bug fixes.

* Fixed an issue with `teleport configure` generating empty hostname for web proxy address. [#8245](https://github.com/gravitational/teleport/pull/8245)
* Fixed an issue with interactive sessions always exiting with code 0. [#8252](https://github.com/gravitational/teleport/pull/8252)
* Fixed an issue with AWS console access silently filtering out IAM roles with paths. [#8225](https://github.com/gravitational/teleport/pull/8225)
* Fixed an issue with `fsGroup` not being set in teleport-kube-agent chart when using persistent storage. [#8085](https://github.com/gravitational/teleport/pull/8085)
* Fixed an issue with Kubernetes service not respecting `public_addr` setting. [#8258](https://github.com/gravitational/teleport/pull/8258)

## 7.1.1

This release of Teleport contains multiple bug fixes and security fixes.

* Fixed an issue with starting Teleport with `--bootstrap` flag. [#8128](https://github.com/gravitational/teleport/pull/8128)
* Added support for non-blocking access requests via `--request-nowait` flag. [#7979](https://github.com/gravitational/teleport/pull/7979)
* Added support for a profile specific kubeconfig file. [#8048](https://github.com/gravitational/teleport/pull/8048)

### Security fixes

As part of a routine security audit of Teleport, several security vulnerabilities
and miscellaneous issues were discovered. Below are the issues found, their
impact, and the components of Teleport they affect.

#### Server Access

An attacker with privileged network position could forge SSH host certificates
that Teleport would incorrectly validate in specific code paths. The specific
paths of concern are:

* Using `tsh` with an identity file (commonly used for service accounts). This
  could lead to potentially leaking of sensitive commands the service account
  runs or in the case of proxy recording mode, the attacker could also gain
  control of the SSH agent being used.

* Teleport agents could incorrectly connect to an attacker controlled cluster.
  Note, this would not give the attacker access or control of resources (like
  SSH, Kubernetes, Applications, or Database servers) because Teleport agents
  will still reject all connections without a valid x509 or SSH user
  certificate.

#### Database Access

When connecting to a Postgres database, an attacker could craft a database name
or a username in a way that would have allowed them control over the resulting
connection string.

An attacker could have probed connections to other reachable database servers
and alter connection parameters such as disable TLS or connect to a database
authenticated by a password.

#### All

During an internal security exercise our engineers have discovered a
vulnerability in Teleport build infrastructure that could have been potentially
used to alter build artifacts. We have found no evidence of any exploitation. In
an effort to be open and transparent with our customers, we encourage all
customers to upgrade to the latest patch release.

#### Actions

For all users, we recommend upgrading all components of their Teleport cluster.
If upgrading all components is not possible, we recommend upgrading `tsh` and
Teleport agents (including trusted cluster proxies) that use reverse tunnels.

Upgrades should follow the normal Teleport upgrade procedure:
https://goteleport.com/teleport/docs/admin-guide/#upgrading-teleport.

## 7.1.0

This release of Teleport contains a feature and bug fix.

* Added support for user and session locking. [RFD#9](https://github.com/gravitational/teleport/blob/master/rfd/0009-locking.md)
* Fixed DynamoDB performance issues. [#7992](https://github.com/gravitational/teleport/pull/7992)
* Fixed issue in build pipeline that was generating empty CentOS 6 archives. [#8033](https://github.com/gravitational/teleport/pull/8033)

## 7.0.3

This release of Teleport contains a bug fix.

* Fixed an issue that could prevent DynamoDB users from logging into Teleport.

## 7.0.2

This release of Teleport contains multiple bug fixes.

* Fixed issue that prevented preset editor role from creating SSO connectors in Web UI. [#7667](https://github.com/gravitational/teleport/issues/7667)
* Fixed issue where OSS Web UI was enabled in Enterprise Docker images.

## 7.0.0

Teleport 7.0 is a major release of Teleport that contains new features, improvements, and bug fixes.

### New Features

#### MongoDB

Added support for [MongoDB](https://www.mongodb.com) to Teleport Database Access. [#6600](https://github.com/gravitational/teleport/issues/6600).

View the [Database Access with MongoDB](https://goteleport.com/docs/ver/7.0/database-access/guides/mongodb-self-hosted/) for more details.

#### Cloud SQL MySQL

Added support for [GCP Cloud SQL MySQL](https://cloud.google.com/sql/docs/mysql) to Teleport Database Access. [#7302](https://github.com/gravitational/teleport/pull/7302)

View the Cloud SQL MySQL [guide](https://goteleport.com/docs/ver/7.0/database-access/guides/mysql-cloudsql/) for more details.

#### AWS Console

Added support for [AWS Console](https://aws.amazon.com/console) to Teleport Application Access. [#7590](https://github.com/gravitational/teleport/pull/7590)

Teleport Application Access can now automatically sign users into the AWS Management Console using [Identity federation](https://aws.amazon.com/identity/federation). View AWS Management Console [guide](https://goteleport.com/docs/ver/7.0/application-access/guides/aws-console/) for more details.

#### Restricted Sessions

Added the ability to block network traffic (IPv4 and IPv6) on a per-SSH session basis. Implemented using BPF tooling which required kernel 5.8 or above. [#7099](https://github.com/gravitational/teleport/pull/7099)

#### Enhanced Session Recording

Updated Enhanced Session Recording to no longer require the installation of external compilers like `bcc-tools`. Implemented using BPF tooling which required kernel 5.8 or above. [#6027](https://github.com/gravitational/teleport/pull/6027)

### Improvements

* Added the ability to terminate Database Access certificates when the certificate expires. [#5476](https://github.com/gravitational/teleport/issues/5476)
* Added additional FedRAMP compliance controls, such as custom disconnect and MOTD messages. [#6091](https://github.com/gravitational/teleport/issues/6091) [#7396](https://github.com/gravitational/teleport/pull/7396)
* Added the ability to export Audit Log and session recordings using the Teleport API. [#6731](https://github.com/gravitational/teleport/pull/6731) [#7360](https://github.com/gravitational/teleport/pull/7360)
* Added the ability to partially configure a cluster. [#5857](https://github.com/gravitational/teleport/issues/5857) [RFD #28](https://github.com/gravitational/teleport/blob/master/rfd/0028-cluster-config-resources.md)
* Added the ability to disable port forwarding on a per-host basis. [#6989](https://github.com/gravitational/teleport/pull/6989)
* Added ability to configure `tsh` home directory. [#7035](https://github.com/gravitational/teleport/pull/7035/files)
* Added ability to generate OpenSSH client configuration snippets using `tsh config`. [#7437](https://github.com/gravitational/teleport/pull/7437)
* Added default-port detection to `tsh` [#6374](https://github.com/gravitational/teleport/pull/6374)
* Improved performance of the Web UI for users with many roles. [#7588](https://github.com/gravitational/teleport/pull/7588)

### Fixes

* Fixed a memory leak that could affect etcd users. [#7631](https://github.com/gravitational/teleport/pull/7631)
* Fixed an issue where `tsh login` could fail if the user had multiple public addresses defined on the proxy. [#7368](https://github.com/gravitational/teleport/pull/7368)

### Breaking Changes

#### Enhanced Session Recording

Enhanced Session Recording has been updated to use CO-RE BPF executables. This makes deployment much simpler, you no longer have to install `bcc-tools`, but comes with a higher minimum kernel version of 5.8 and above. [#6027](https://github.com/gravitational/teleport/pull/6027)

#### Kubernetes Access

Kubernetes Access will no longer automatically register a cluster named after the Teleport cluster if the proxy is running within a Kubernetes cluster. Users wishing to retain this functionality now have to explicitly set `kube_cluster_name`. [#6786](https://github.com/gravitational/teleport/pull/6786)

#### `tsh`

`tsh login` has been updated to no longer change the current Kubernetes context. While `tsh login` will write credentials to `kubeconfig` it will only update your context if `tsh login --kube-cluster` or `tsh kube login <kubeCluster>` is used. [#6045](https://github.com/gravitational/teleport/issues/6045)

## 6.2.8

This release of Teleport contains an improvement and new feature.

* Improved Web UI performance for DynamoDB users. [#7587](https://github.com/gravitational/teleport/pull/7587)
* Added API to export session recordings. [#7360](https://github.com/gravitational/teleport/pull/7360)

## 6.2.7

This release of Teleport contains multiple fixes.

* Fixed issue that could cause `GetNodes` to fail on large clusters. [#7415](https://github.com/gravitational/teleport/pull/7415)
* Fixed issue that could cause long commands to hang. [#7449](https://github.com/gravitational/teleport/pull/7449)

## 6.2.6

This release of Teleport contains new features and improvements.

* Added ability to disable port forwarding on a per-node basis. [#6989](https://github.com/gravitational/teleport/pull/6989)
* Updated `api` client to support additional endpoints. [#7220](https://github.com/gravitational/teleport/pull/7220)
* Improved performance of DynamoDB events filtering. [#7231](https://github.com/gravitational/teleport/pull/7231)

## 6.2.5

This release of Teleport contains new features, bug fixes, and multiple improvements.

* Added support for `regexp.replace` in role templates. [#7152](https://github.com/gravitational/teleport/pull/7152)
* Added `RoleV4` with stricter default allow labels. `RoleV4` is backward-compatible with `RoleV3` and is completely opt-in. [#7132](https://github.com/gravitational/teleport/pull/7132) [#7118](https://github.com/gravitational/teleport/pull/7118)
* Updated OIDC connector to gracefully degrade UserInfo endpoint. [#7333](https://github.com/gravitational/teleport/pull/7333)
* Improved Access Requests events to allow easier correlation between access requests and `session.start` events. [#6863](https://github.com/gravitational/teleport/pull/6863)
* Fixed an issue that could cause upgrade from Teleport 5.x to 6.x to fail. [#7310](https://github.com/gravitational/teleport/pull/7310)
* Fixed multiple issues with events subsystem. [#7303](https://github.com/gravitational/teleport/pull/7303) [#7266](https://github.com/gravitational/teleport/pull/7266)

## 6.2.3

This release of Teleport contains multiple improvements.

* Improvements to speed up DynamoDB events migration. We now encourage all DynamoDB users to upgrade to Teleport 6.2. [#7073](https://github.com/gravitational/teleport/pull/7073) [#7097](https://github.com/gravitational/teleport/pull/7097)

## 6.2.1

This release of Teleport contains an improvement and several bug fixes.

### Improvements

* Improve performance of DynamoDB events migration introduced in `v6.2.0`.
  [#7083](https://github.com/gravitational/teleport/pull/7083)

### Fixes

* Fixed an issue with connecting to etcd in insecure mode.
  [#7049](https://github.com/gravitational/teleport/pull/7049)
* Fixed an issue with running Teleport on systems without utmp/wtmp support such
  as alpine. [#7059](https://github.com/gravitational/teleport/pull/7059)
* Fixed an issue with signing database certificates using `tctl auth sign` via
  proxy. See [#7071](https://github.com/gravitational/teleport/discussions/7071)
  for details. [#7038](https://github.com/gravitational/teleport/pull/7038)

## 6.2.0

Teleport 6.2 contains new features, improvements, and bug fixes.

**Note:** the DynamoDB migration described [below](#dynamodb-indexing-change)
may cause rate-limiting errors from AWS APIs and is slow on large deployments
(1000+ existing audit events). The next patch release, v6.2.1, will improve the
migration performance. If you run a large DynamoDB-based cluster, we advise you
to wait for v6.2.1 before upgrading.

### New Features

#### Added Amazon Redshift Support

Added support for [Amazon Redshift](https://aws.amazon.com/redshift) to Teleport Database Access.[#6479](https://github.com/gravitational/teleport/pull/6479).

View the [Database Access with Redshift on AWS Guide](https://goteleport.com/docs/ver/6.2/database-access/guides/postgres-redshift/) for more details.

### Improvements

* Added pass-through header support for Teleport Application Access. [#6601](https://github.com/gravitational/teleport/pull/6601)
* Added ability to propagate claim information from root to leaf clusters. [#6540](https://github.com/gravitational/teleport/pull/6540)
* Added Proxy Protocol for MySQL Database Access. [#6594](https://github.com/gravitational/teleport/pull/6594)
* Added prepared statement support for Postgres Database Access. [#6303](https://github.com/gravitational/teleport/pull/6303)
* Added `GetSessionEventsRequest` RPC endpoint for Audit Log pagination. [RFD 19](https://github.com/gravitational/teleport/blob/master/rfd/0019-event-iteration-api.md) [#6731](https://github.com/gravitational/teleport/pull/6731)
* Changed DynamoDB indexing strategy for events. [RFD 24](https://github.com/gravitational/teleport/blob/master/rfd/0024-dynamo-event-overflow.md) [#6583](https://github.com/gravitational/teleport/pull/6583)

### Fixes

* Fixed multiple per-session MFA issues. [#6542](https://github.com/gravitational/teleport/pull/6542) [#6567](https://github.com/gravitational/teleport/pull/6567) [#6625](https://github.com/gravitational/teleport/pull/6625) [#6779](https://github.com/gravitational/teleport/pull/6779) [#6948](https://github.com/gravitational/teleport/pull/6948)
* Fixed etcd JWT renewal issue. [#6905](https://github.com/gravitational/teleport/pull/6905)
* Fixed issue where `kubectl exec` sessions were not being recorded when the target pod was killed. [#6068](https://github.com/gravitational/teleport/pull/6068)
* Fixed an issue that prevented Teleport from starting on ARMv7 systems. [#6711](https://github.com/gravitational/teleport/pull/6711).
* Fixed issue that caused Access Requests to inconsistently allow elevated Kuberentes access. [#6492](https://github.com/gravitational/teleport/pull/6492)
* Fixed an issue that could cause `session.end` events not to be emitted. [#6756](https://github.com/gravitational/teleport/pull/6756)
* Fixed an issue with PAM variable interpolation. [#6558](https://github.com/gravitational/teleport/pull/6558)

### Breaking Changes

#### Agent Forwarding

Teleport 6.2 brings a potentially backward incompatible change with `tsh` agent forwarding.

Prior to Teleport 6.2, `tsh ssh -A` would create an in-memory SSH agent from your `~/.tsh` directory and forward that agent to the target host.

Starting in Teleport 6.2 `tsh ssh -A` by default now forwards your system SSH agent (available at `$SSH_AUTH_SOCK`). Users wishing to retain the prior behavior can use `tsh ssh -o "ForwardAgent local"`.

For more details see [RFD 22](https://github.com/gravitational/teleport/blob/master/rfd/0022-ssh-agent-forwarding.md) and implementation in [#6525](https://github.com/gravitational/teleport/pull/6525).

#### DynamoDB Indexing Change

DynamoDB users should note that the events backend indexing strategy has
changed and a data migration will be triggered after upgrade. For optimal
performance perform this migration with only one auth server online. It may
take some time and progress will be periodically written to the auth server
log. During this migration, only events that have been migrated will appear in
the Web UI. After completion, all events will be available.

For more details see [RFD 24](https://github.com/gravitational/teleport/blob/master/rfd/0024-dynamo-event-overflow.md) and implementation in [#6583](https://github.com/gravitational/teleport/pull/6583).

## 6.1.5

This release of Teleport contains multiple bug fixes.

* Added additional Prometheus Metrics. [#6511](https://github.com/gravitational/teleport/pull/6511)
* Updated the TLS handshake timeout to 5 seconds to avoid timeout issues on large clusters. [#6692](https://github.com/gravitational/teleport/pull/6692)
* Fixed issue that caused non-interactive SSH output to show up in logs. [#6683](https://github.com/gravitational/teleport/pull/6683)
* Fixed two issues that could cause Teleport to panic upon startup. [#6431](https://github.com/gravitational/teleport/pull/6431) [#5712](https://github.com/gravitational/teleport/pull/5712)

## 6.1.3

This release of Teleport contains a bug fix.

* Added support for PROXY protocol to Database Access (MySQL). [#6517](https://github.com/gravitational/teleport/issues/6517)

## 6.1.2

This release of Teleport contains a new feature.

* Added log formatting and support to enable timestamps for logs. [#5898](https://github.com/gravitational/teleport/pull/5898)

## 6.1.1

This release of Teleport contains a bug fix.

* Fixed an issue where DEB builds were not published to the [Teleport DEB repository](https://deb.releases.teleport.dev/).

## 6.1.0

Teleport 6.1 contains multiple new features, improvements, and bug fixes.

### New Features

#### U2F for Kubernetes and SSH sessions

Added support for U2F authentication on every SSH and Kubernetes "connection" (a single `tsh ssh` or `kubectl` call). This is an advanced security feature that protects users against compromises of their on-disk Teleport certificates. Per-session MFA can be enforced cluster-wide or only for some specific roles.

For more details see [Per-Session MFA](https://goteleport.com/docs/access-controls/guides/per-session-mfa) documentation or [RFD 14](https://github.com/gravitational/teleport/blob/master/rfd/0014-session-2FA.md) and [RFD 15](https://github.com/gravitational/teleport/blob/master/rfd/0015-2fa-management.md) for technical details.

#### Dual Authorization Workflows

Added ability to request multiple users to review and approve access requests.

See [#5071](https://github.com/gravitational/teleport/pull/5071) for technical details.

### Improvements

* Added the ability to propagate SSO claims to PAM modules. [#6158](https://github.com/gravitational/teleport/pull/6158)
* Added support for cluster routing to reduce latency to leaf clusters. [RFD 21](https://github.com/gravitational/teleport/blob/master/rfd/0021-cluster-routing.md)
* Added support for Google Cloud SQL to Database Access. [#6090](https://github.com/gravitational/teleport/pull/6090)
* Added support CLI credential issuance for Application Access. [#5918](https://github.com/gravitational/teleport/pull/5918)
* Added support for Encrypted SAML Assertions. [#5598](https://github.com/gravitational/teleport/pull/5598)
* Added support for user impersonation. [#6073](https://github.com/gravitational/teleport/pull/6073)

### Fixes

* Fixed interoperability issues with `gpg-agent`. [RFD 18](http://github.com/gravitational/teleport/blob/master/rfd/0018-agent-loading.md)
* Fixed websocket support in Application Access. [#6028](https://github.com/gravitational/teleport/pull/6028)
* Fixed file argument issues with `tsh play`. [#1580](https://github.com/gravitational/teleport/issues/1580)
* Fixed `utmp` regressions that caused issues in LXC containers. [#6256](https://github.com/gravitational/teleport/pull/6256)

## 6.0.3

This release of Teleport contains a bug fix.

* Fixed a issue that caused high network on deployments with many leaf Trusted Clusters. [#6263](https://github.com/gravitational/teleport/pull/6263)

## 6.0.2

This release of Teleport contains bug fixes and adds new default roles.

* Fixed an issue with proxy web endpoint resetting connection when run with `--insecure-no-tls` flag. [#5923](https://github.com/gravitational/teleport/pull/5923)
* Introduced role presets: `auditor`, `editor` and `access`. [#5968](https://github.com/gravitational/teleport/pull/5968)
* Added ability to inline `google_service_account` field into Google Workspace OIDC connector. [#5563](http://github.com/gravitational/teleport/pull/5563)

## 6.0.1

This release of Teleport contains multiple bug fixes.

* Fixed issue that caused ACME default configuration to fail with `TLS-ALPN-01` challenge. [#5839](https://github.com/gravitational/teleport/pull/5839)
* Fixed regression in ADFS integration. [#5880](https://github.com/gravitational/teleport/pull/5880)

## 6.0.0

Teleport 6.0 is a major release with new features, functionality, and bug fixes.

We have implemented [Database Access](https://goteleport.com/teleport/docs/database-access/),
open sourced role-based access control (RBAC), and added official API and a Go client library.

Users can review the [6.0 milestone](https://github.com/gravitational/teleport/milestone/33?closed=1) on Github for more details.

### New Features

#### Database Access

Review the Database Access design in [RFD #11](https://github.com/gravitational/teleport/blob/master/rfd/0011-database-access.md).

With Database Access users can connect to PostgreSQL and MySQL databases using short-lived certificates, configure SSO authentication and role-based access controls for databases, and capture SQL query activity in the audit log.

##### Getting Started

Configure Database Access following the [Getting Started](https://goteleport.com/teleport/docs/database-access/getting-started/) guide.

##### Guides

* [AWS RDS/Aurora PostgreSQL](https://goteleport.com/teleport/docs/database-access/guides/postgres-aws/)
* [AWS RDS/Aurora MySQL](https://goteleport.com/teleport/docs/database-access/guides/mysql-aws/)
* [Self-hosted PostgreSQL](https://goteleport.com/teleport/docs/database-access/guides/postgres-self-hosted/)
* [Self-hosted MySQL](https://goteleport.com/teleport/docs/database-access/guides/mysql-self-hosted/)
* [GUI clients](https://goteleport.com/teleport/docs/database-access/guides/gui-clients/)

##### Resources

To learn more about configuring role-based access control for Database Access, check out [RBAC](https://goteleport.com/teleport/docs/database-access/rbac/) section.

[Architecture](https://goteleport.com/teleport/docs/database-access/architecture/) provides a more in-depth look at Database Access internals such as networking and security.

See [Reference](https://goteleport.com/teleport/docs/database-access/reference/) for an overview of Database Access related configuration and CLI commands.

Finally, check out [Frequently Asked Questions](./database-access/faq.mdx).

#### OSS RBAC

Open source RBAC support was introduced in [RFD #7](https://github.com/gravitational/teleport/blob/master/rfd/0007-rbac-oss.md).

RBAC support gives OSS administrators more granular access controls to servers and other resources with a cluster (like session recording access). An example of an RBAC policy could be: "admins can do anything, developers must never touch production servers and interns can only SSH into staging servers as guests"

In addition, some Access Workflow Plugins will now become available to open source users.

* Access Workflows Golang SDK and API
* Slack
* Gitlab
* Mattermost
* JIRA Plugin
* PagerDuty Plugin

#### Client libraries and API

API and Client Libraries support was introduced in [RFD #10](https://github.com/gravitational/teleport/blob/master/rfd/0010-api.md).

The new API and client library reduces the dependencies needed to use the Teleport API as well as making it easier to use. An example of using the new API is below.

```go
// Create a client connected to the Auth server with an exported identity file.
clt, err := client.NewClient(client.Config{
  Addrs: []string{"auth.example.com:3025"},
  Credentials: []client.Credentials{
    client.LoadIdentityFile("identity.pem"),
  },
})
if err != nil {
  log.Fatalf("Failed to create client: %v.", err)
}
defer clt.Close()

// Create a Access Request.
accessRequest, err := types.NewAccessRequest(uuid.New(), "access-admin", "admin")
if err != nil {
  log.Fatalf("Failed to build access request: %v.", err)
}
if err = clt.CreateAccessRequest(ctx, accessRequest); err != nil {
  log.Fatalf("Failed to create access request: %v.", err)
}
```

### Improvements

* Added `utmp`/`wtmp` support for SSH in [#5491](https://github.com/gravitational/teleport/pull/5491).
* Added the ability to set a Kubernetes specific public address in [#5611](https://github.com/gravitational/teleport/pull/5611).
* Added Proxy Protocol support to Kubernetes Access in [#5299](https://github.com/gravitational/teleport/pull/5299).
* Added ACME ([Let's Encrypt](https://letsencrypt.org/)) support to make getting and using TLS certificates easier. [#5177](https://github.com/gravitational/teleport/issues/5177).
* Added the ability to manage local users to the Web UI in [#2945](https://github.com/gravitational/teleport/issues/2945).
* Added the ability to preserve timestamps when using `tsh scp` in [#2889](https://github.com/gravitational/teleport/issues/2889).

### Fixes

* Fixed authentication failure when logging in via CLI with Access Workflows after removing `.tsh` directory in [#5323](https://github.com/gravitational/teleport/pull/5323).
* Fixed `tsh login` failure when `--proxy` differs from actual proxy public address in [#5380](https://github.com/gravitational/teleport/pull/5380).
* Fixed session playback issues in [#2945](https://github.com/gravitational/teleport/issues/2945).
* Fixed several UX issues in [#5559](https://github.com/gravitational/teleport/issues/5559), [#5568](https://github.com/gravitational/teleport/issues/5568), [#4965](https://github.com/gravitational/teleport/issues/4965), and [#5057](https://github.com/gravitational/teleport/pull/5057).

### Upgrade Notes

Please follow our [standard upgrade procedure](https://goteleport.com/teleport/docs/admin-guide/#upgrading-teleport) to upgrade your cluster.

Note, for clusters using GitHub SSO and Trusted Clusters, when upgrading SSO users will lose connectivity to leaf clusters. Local users will not be affected.

To restore connectivity to leaf clusters for SSO users, leaf admins should update the `trusted_cluster` role mapping resource like below.

```yaml
kind: trusted_cluster
version: v2
metadata:
   name: "zztop-oss"
spec:
   enabled: true
   token: "bar"
   web_proxy_addr: 172.10.1.1:3080
   tunnel_addr: 172.10.1.1:3024
   role_map:
   - remote: "admin"
     local: ['admin']
   - remote: "^(github-.*)$"
     local: ['admin']
```

## 5.1.0

This release of Teleport adds a new feature.

* Support for creating and assuming Access Workflow requests from within the Web UI (first step toward full Workflow UI support: [#4937](https://github.com/gravitational/teleport/issues/4937)).

## 5.0.2

This release of Teleport contains a security fix.

* Patch a SAML authentication bypass (see https://github.com/russellhaering/gosaml2/security/advisories/GHSA-xhqq-x44f-9fgg): [#5119](https://github.com/gravitational/teleport/pull/5119).

Any Enterprise SSO users using Okta, Active Directory, OneLogin or custom SAML connectors should upgrade their auth servers to version 5.0.2 and restart Teleport. If you are unable to upgrade immediately, we suggest disabling SAML connectors for all clusters until the updates can be applied.


## 5.0.1

This release of Teleport contains multiple bug fixes.

* Always set expiry times on server resource in heartbeats [#5008](https://github.com/gravitational/teleport/pull/5008)
* Fixes streaming k8s responses (`kubectl logs -f`, `kubectl run -it`, etc) [#5009](https://github.com/gravitational/teleport/pull/5009)
* Multiple fixes for the k8s forwarder [#5038](https://github.com/gravitational/teleport/pull/5038)

## 5.0.0

Teleport 5.0 is a major release with new features, functionality, and bug fixes. Users can review [5.0 closed issues](https://github.com/gravitational/teleport/milestone/39?closed=1) on Github for details of all items.

#### New Features

Teleport 5.0 introduces two distinct features: Teleport Application Access and significant Kubernetes Access improvements - multi-cluster support.

##### Teleport Application Access

Teleport can now be used to provide secure access to web applications. This new feature was built with the express intention of securing internal apps which might have once lived on a VPN or had a simple authorization and authentication mechanism with little to no audit trail. Application Access works with everything from dashboards to single page Javascript applications (SPA).

Application Access uses mutually authenticated reverse tunnels to establish a secure connection with the Teleport unified Access Plane which can then becomes the single ingress point for all traffic to an internal application.

Adding an application follows the same UX as adding SSH servers or Kubernetes clusters, starting with creating a static or dynamic invite token.

```bash
$ tctl tokens add --type=app
```

Then simply start Teleport with a few new flags.

```sh
$ teleport start --roles=app --token=xyz --auth-server=proxy.example.com:3080 \
    --app-name="example-app" \
    --app-uri="http://localhost:8080"
```

This command will start an app server that proxies the application "example-app" running at `http://localhost:8080` at the public address `https://example-app.example.com`.

Applications can also be configured using the new `app_service` section in `teleport.yaml`.

```yaml
app_service:
   # Teleport Application Access is enabled.
   enabled: yes
   # We've added a default sample app that will check
   # that Teleport Application Access is working
   # and output JWT tokens.
   # https://dumper.teleport.example.com:3080/
   debug_app: true
   apps:
   # Application Access can be used to proxy any HTTP endpoint.
   # Note: Name can't include any spaces and should be DNS-compatible A-Za-z0-9-._
   - name: "internal-dashboard"
     uri: "http://10.0.1.27:8000"
     # By default Teleport will make this application
     # available on a sub-domain of your Teleport proxy's hostname
     # internal-dashboard.teleport.example.com
     # - thus the importance of setting up wilcard DNS.
     # If you want, it's possible to set up a custom public url.
     # DNS records should point to the proxy server.
     # internal-dashboard.teleport.example.com
     # Example Public URL for the internal-dashboard app.
     # public_addr: "internal-dashboard.acme.com"
     # Optional labels
     # Labels can be combined with RBAC rules to provide access.
     labels:
       customer: "acme"
       env: "production"
     # Optional dynamic labels
     commands:
     - name: "os"
       command: ["/usr/bin/uname"]
       period: "5s"
     # A proxy can support multiple applications. Application Access
     # can also be deployed with a Teleport node.
     - name: "arris"
       uri: "http://localhost:3001"
       public_addr: "arris.example.com"
```

Application access requires two additional changes. DNS must be updated to point the application domain to the proxy and the proxy must be loaded with a TLS certificate for the domain. Wildcard DNS and TLS certificates can be used to simplify deployment.

```yaml
# When adding the app_service certificates are required to provide a TLS
# connection. The certificates are managed by the proxy_service
proxy_service:
  # We've extended support for https certs. Teleport can now load multiple
  # TLS certificates. In the below example we've obtained a wildcard cert
  # that'll be used for proxying the applications.
  # The correct certificate is selected based on the hostname in the HTTPS
  # request using SNI.
  https_keypairs:
  - key_file: /etc/letsencrypt/live/teleport.example.com/privkey.pem
    cert_file: /etc/letsencrypt/live/teleport.example.com/fullchain.pem
  - key_file: /etc/letsencrypt/live/*.teleport.example.com/privkey.pem
    cert_file: /etc/letsencrypt/live/*.teleport.example.com/fullchain.pem
```

You can learn more at [https://goteleport.com/teleport/docs/application-access/](https://goteleport.com/teleport/docs/application-access/)

##### Teleport Kubernetes Access

Teleport 5.0 also introduces two highly requested features for Kubernetes.

* The ability to connect multiple Kubernetes Clusters to the Teleport Access Plane, greatly reducing operational complexity.
* Complete Kubernetes audit log capture [#4526](https://github.com/gravitational/teleport/pull/4526), going beyond the existing `kubectl exec` capture.

For a full overview please review the [Kubernetes RFD](https://github.com/gravitational/teleport/blob/master/rfd/0005-kubernetes-service.md).

To support these changes, we've introduced a new service. This moves Teleport Kubernetes configuration from the `proxy_service` into its own dedicated `kubernetes_service` section.

When adding the new Kubernetes service, a new type of join token is required.

```bash
tctl tokens add --type=kube
```

Example configuration for the new `kubernetes_service`:

```yaml
# ...
kubernetes_service:
   enabled: yes
   listen_addr: 0.0.0.0:3027
   kubeconfig_file: /secrets/kubeconfig
```

Note: a Kubernetes port still needs to be configured in the `proxy_service` via `kube_listen_addr`.

#### New "tsh kube" commands

`tsh kube` commands are used to query registered clusters and switch `kubeconfig` context:

```sh
$ tsh login --proxy=proxy.example.com --user=awly

# list all registered clusters
$ tsh kube ls
Cluster Name       Status
-------------      ------
a.k8s.example.com  online
b.k8s.example.com  online
c.k8s.example.com  online

# on login, kubeconfig is pointed at the first cluster (alphabetically)
$ kubectl config current-context
proxy.example.com-a.k8s.example.com

# but all clusters are populated as contexts
$ kubectl config get-contexts
CURRENT   NAME                     CLUSTER             AUTHINFO
*         proxy.example.com-a.k8s.example.com   proxy.example.com   proxy.example.com-a.k8s.example.com
          proxy.example.com-b.k8s.example.com   proxy.example.com   proxy.example.com-b.k8s.example.com
          proxy.example.com-c.k8s.example.com   proxy.example.com   proxy.example.com-c.k8s.example.com

# switch between different clusters:
$ tsh kube login c.k8s.example.com

# the traditional way is also supported:
$ kubectl config use-context proxy.example.com-c.k8s.example.com

# check current cluster
$ kubectl config current-context
proxy.example.com-c.k8s.example.com
```

Other Kubernetes changes:

* Support k8s clusters behind firewall/NAT using a single Teleport cluster [#3667](https://github.com/gravitational/teleport/issues/3667)
* Support multiple k8s clusters with a single Teleport proxy instance [#3952](https://github.com/gravitational/teleport/issues/3952)

##### Additional User and Token Resource

We've added two new RBAC resources; these provide the ability to limit token creation and to list and modify Teleport users:

```yaml
- resources: [user]
  verbs: [list,create,read,update,delete]
- resources: [token]
  verbs: [list,create,read,update,delete]
```

Learn more about [Teleport's RBAC Resources](https://goteleport.com/teleport/docs/enterprise/ssh-rbac/)

##### Cluster Labels

Teleport 5.0 also adds the ability to set labels on Trusted Clusters. The labels are set when creating a trusted cluster invite token. This lets teams use the same RBAC controls used on nodes to approve or deny access to clusters. This can be especially useful for MSPs that connect hundreds of customers' clusters - when combined with Access Workflows, cluster access can easily be delegated. Learn more by reviewing our [Truster Cluster Setup & RBAC Docs](https://goteleport.com/teleport/docs/trustedclusters/#dynamic-join-tokens)

Creating a trusted cluster join token for a production environment:

```bash
$ tctl tokens add --type=trusted_cluster --labels=env=prod
```

```yaml
kind: role
#...
  deny:
    # cluster labels control what clusters user can connect to. The wildcard ('*')
    # means any cluster. By default, deny rules are empty to preserve backwards
    # compatibility
    cluster_labels:
      'env': 'prod'
```

##### Teleport UI Updates

Teleport 5.0 also iterates on the UI Refresh from 4.3. We've moved the cluster list into our sidebar and have added an Application launcher. For customers moving from 4.4 to 5.0, you'll notice that we have moved session recordings back to their own dedicated section.

Other updates:

* We now provide local user management via `https://[cluster-url]/web/users`, providing the ability to easily edit, reset and delete local users.
* Teleport Node & App Install scripts. This is currently an Enterprise-only feature that provides customers with an easy 'auto-magic' installer script. Enterprise customers can enable this feature by modifying the 'token' resource. See note above.
* We've added a Waiting Room for customers using Access Workflows. [Docs](https://goteleport.com/teleport/docs/enterprise/workflow/#adding-a-reason-to-access-workflows)

##### Signed RPM and Releases

Starting with Teleport 5.0, we now provide an RPM repo for stable releases of Teleport. We've also started signing our RPMs to provide assurance that you're always using an official build of Teleport.

See https://rpm.releases.teleport.dev/ for more details.

#### Improvements

* Added `--format=json` playback option for `tsh play`. For example `tsh play --format=json ~/play/0c0b81ed-91a9-4a2a-8d7c-7495891a6ca0.tar | jq '.event` can be used to show all events within an a local archive. [#4578](https://github.com/gravitational/teleport/issues/4578)
* Added support for continuous backups and auto scaling for DynamoDB. [#4780](https://github.com/gravitational/teleport/issues/4780)
* Added a Linux ARM64/ARMv8 (64-bit) Release. [#3383](https://github.com/gravitational/teleport/issues/3383)
* Added `https_keypairs` field which replaces `https_key_file` and `https_cert_file`. This allows administrators to load multiple HTTPS certs for Teleport Application Access. Teleport 5.0 is backwards compatible with the old format, but we recommend updating your configuration to use `https_keypairs`.

Enterprise Only:

* `tctl` can load credentials from `~/.tsh` [#4678](https://github.com/gravitational/teleport/pull/4678)
* Teams can require a user submitted reason when using Access Workflows [#4573](https://github.com/gravitational/teleport/pull/4573#issuecomment-720777443)

#### Fixes

* Updated `tctl` to always format resources as lists in JSON/YAML. [#4281](https://github.com/gravitational/teleport/pull/4281)
* Updated `tsh status` to now print Kubernetes status. [#4348](https://github.com/gravitational/teleport/pull/4348)
* Fixed intermittent issues with `loginuid.so`. [#3245](https://github.com/gravitational/teleport/issues/3245)
* Reduced `access denied to Proxy` log spam. [#2920](https://github.com/gravitational/teleport/issues/2920)
* Various AMI fixes: paths are now consistent with other Teleport packages and configuration files will not be overwritten on reboot.

#### Documentation

We've added an [API Reference](https://goteleport.com/teleport/docs/api-reference/) to simply developing applications against Teleport.

#### Upgrade Notes

Please follow our [standard upgrade procedure](https://goteleport.com/teleport/docs/admin-guide/#upgrading-teleport).

* Optional: Consider updating `https_key_file` & `https_cert_file` to our new `https_keypairs:` format.
* Optional: Consider migrating Kubernetes Access from `proxy_service` to `kubernetes_service` after the upgrade.

### 4.4.6

This release of teleport contains a security fix and a bug fix.

* Patch a SAML authentication bypass (see https://github.com/russellhaering/gosaml2/security/advisories/GHSA-xhqq-x44f-9fgg): [#5120](https://github.com/gravitational/teleport/pull/5120).

Any Enterprise SSO users using Okta, Active Directory, OneLogin or custom SAML connectors should upgrade their auth servers to version 4.4.6 and restart Teleport. If you are unable to upgrade immediately, we suggest disabling SAML connectors for all clusters until the updates can be applied.

* Fix an issue where `tsh login` would fail with an `AccessDenied` error if
the user was perviously logged into a leaf cluster. [#5105](https://github.com/gravitational/teleport/pull/5105)

### 4.4.5

This release of Teleport contains a bug fix.

* Fixed an issue where a slow or unresponsive Teleport auth service could hang client connections in async recording mode. [#4696](https://github.com/gravitational/teleport/pull/4696)

### 4.4.4

This release of Teleport adds enhancements to the Access Workflows API.

* Support for creating limited roles that trigger access requests
on login, allowing users to be configured such that no nodes can
be accessed without externally granted roles.

* Teleport UI support for automatically generating access requests and
assuming new roles upon approval (access requests were previously
only available in `tsh`).

* New `claims_to_roles` mapping that can use claims from external
identity providers to determine which roles a user can request.

* Various minor API improvements to help make requests easier to
manage and audit, including support for human-readable
request/approve/deny reasons and structured annotations.

### 4.4.2

This release of Teleport adds support for a new build architecture.

* Added automatic arm64 builds of Teleport to the download portal.

### 4.4.1

This release of Teleport contains a bug fix.

* Fixed an issue where defining multiple logging configurations would cause Teleport to crash. [#4598](https://github.com/gravitational/teleport/issues/4598)

### 4.4.0

This is a major Teleport release with a focus on new features, functionality, and bug fixes. It’s a substantial release and users can review [4.4 closed issues](https://github.com/gravitational/teleport/milestone/40?closed=1) on Github for details of all items.

#### New Features

##### Concurrent Session Control

This addition to Teleport helps customers obtain AC-10 control. We now provide two new optional configuration values: `max_connections` and `max_sessions`.

###### `max_connections`

This value is the total number of concurrent sessions within a cluster to nodes running Teleport. This value is applied at a per user level. If you set `max_connections` to `1`, a `tsh` user would only be able to `tsh ssh` into one node at a time.

###### `max_sessions` per connection

This value limits the total number of session channels which can be established across a single SSH connection (typically used for interactive terminals or remote exec operations). This is for cases where nodes have Teleport set up, but a user is using OpenSSH to connect to them. It is essentially equivalent to the `MaxSessions` configuration value accepted by `sshd`.

```yaml
spec:
  options:
    # Optional: Required to be set for AC-10 Compliance
    max_connections: 2
    # Optional: To match OpenSSH behavior set to 10
    max_sessions: 10
```

###### `session_control_timeout`

A new `session_control_timeout` configuration value has been added to the `auth_service` configuration block of the Teleport config file. It's unlikely that you'll need to modify this.

```yaml
auth_service:
  session_control_timeout: 2m # default
# ...
```

#### Session Streaming Improvements

Teleport 4.4 includes a complete refactoring of our event system. This resolved a few customer bug reports such as [#3800: Events overwritten in DynamoDB](https://github.com/gravitational/teleport/issues/3800) and [#3182: Teleport consuming all disk space with multipart uploads](https://github.com/gravitational/teleport/issues/3182).

Along with foundational improvements, 4.4 includes two new experimental `session_recording` options: `node-sync` and `proxy-sync`.
NOTE: These experimental modes require all Teleport auth servers, proxy servers and nodes to be running Teleport 4.4.

```yaml
# This section configures the 'auth service':
auth_service:
    # Optional setting for configuring session recording. Possible values are:
    #     "node"  : sessions will be recorded on the node level (the default)
    #     "proxy" : recording on the proxy level, see "recording proxy mode" section.
    #     "off"   : session recording is turned off
    #
    #     EXPERIMENTAL *-sync modes: proxy and node send logs directly to S3 or other
    #     storage without storing the records on disk at all. This mode will kill a
    #     connection if network connectivity is lost.
    #     NOTE: These experimental modes require all Teleport auth servers, proxy servers and
    #     nodes to be running Teleport 4.4.
    #
    #     "node-sync" : sessions recording will be streamed from node -> auth -> storage
    #     "proxy-sync : sessions recording will be streamed from proxy -> auth -> storage
    #
    session_recording: "node-sync"
```

#### Improvements

* Added session streaming. [#4045](https://github.com/gravitational/teleport/pull/4045)
* Added concurrent session control. [#4138](https://github.com/gravitational/teleport/pull/4138)
* Added ability to specify leaf cluster when generating `kubeconfig` via `tctl auth sign`. [#4446](https://github.com/gravitational/teleport/pull/4446)
* Added output options (like JSON) for `tsh ls`. [#4390](https://github.com/gravitational/teleport/pull/4390)
* Added node ID to heartbeat debug log [#4291](https://github.com/gravitational/teleport/pull/4291)
* Added the option to trigger `pam_authenticate` on login [#3966](https://github.com/gravitational/teleport/pull/3966)

#### Fixes

* Fixed issue that caused some idle `kubectl exec` sessions to terminate. [#4377](https://github.com/gravitational/teleport/pull/4377)
* Fixed symlink issued when using `tsh` on Windows. [#4347](https://github.com/gravitational/teleport/pull/4347)
* Fixed `tctl top` so it runs without the debug flag and on dark terminals. [#4282](https://github.com/gravitational/teleport/pull/4282) [#4231](https://github.com/gravitational/teleport/pull/4231)
* Fixed issue that caused DynamoDB not to respect HTTP CONNECT proxies. [#4271](https://github.com/gravitational/teleport/pull/4271)
* Fixed `/readyz` endpoint to recover much quicker. [#4223](https://github.com/gravitational/teleport/pull/4223)

#### Documentation

* Updated Google Workspace documentation to add clarification on supported account types. [#4394](https://github.com/gravitational/teleport/pull/4394)
* Updated IoT instructions on necessary ports. [#4398](https://github.com/gravitational/teleport/pull/4398)
* Updated Trusted Cluster documentation on how to remove trust from root and leaf clusters. [#4358](https://github.com/gravitational/teleport/pull/4358)
* Updated the PAM documentation with PAM authentication usage information. [#4352](https://github.com/gravitational/teleport/pull/4352)

#### Upgrade Notes

Please follow our [standard upgrade procedure](https://gravitational.com/teleport/docs/admin-guide/#upgrading-teleport).

## 4.3.9

This release of Teleport contains a security fix.

* Patch a SAML authentication bypass (see https://github.com/russellhaering/gosaml2/security/advisories/GHSA-xhqq-x44f-9fgg): [#5122](https://github.com/gravitational/teleport/pull/5122).

Any Enterprise SSO users using Okta, Active Directory, OneLogin or custom SAML connectors should upgrade their auth servers to version 4.3.9 and restart Teleport. If you are unable to upgrade immediately, we suggest disabling SAML connectors for all clusters until the updates can be applied.

## 4.3.8

This release of Teleport adds support for a new build architecture.

* Added automatic arm64 builds of Teleport to the download portal.

## 4.3.7

This release of Teleport contains a security fix and a bug fix.

* Mitigated [CVE-2020-15216](https://nvd.nist.gov/vuln/detail/CVE-2020-15216) by updating github.com/russellhaering/goxmldsig.

### Details
A vulnerability was discovered in the `github.com/russellhaering/goxmldsig` library which is used by Teleport to validate the
signatures of XML files used to configure SAML 2.0 connectors. With a carefully crafted XML file, an attacker can completely
bypass XML signature validation and pass off an altered file as a signed one.

### Actions
The `goxmldsig` library has been updated upstream and Teleport 4.3.7 includes the fix. Any Enterprise SSO users using Okta,
Active Directory, OneLogin or custom SAML connectors should upgrade their auth servers to version 4.3.7 and restart Teleport.

If you are unable to upgrade immediately, we suggest deleting SAML connectors for all clusters until the updates can be applied.

* Fixed an issue where DynamoDB connections made by Teleport would not respect the `HTTP_PROXY` or `HTTPS_PROXY` environment variables. [#4271](https://github.com/gravitational/teleport/pull/4271)

## 4.3.6

This release of Teleport contains multiple bug fixes.

* Fixed an issue with prefix migration that could lead to loss of cluster state. [#4299](https://github.com/gravitational/teleport/pull/4299) [#4345](https://github.com/gravitational/teleport/pull/4345)
* Fixed an issue that caused excessively slow loading of the UI on large clusters. [#4326](https://github.com/gravitational/teleport/pull/4326)
* Updated `/readyz` endpoint to recover faster after node goes into degraded state. [#4223](https://github.com/gravitational/teleport/pull/4223)
* Added node UUID to debug logs to allow correlation between TCP connections and nodes. [#4291](https://github.com/gravitational/teleport/pull/4291)

## 4.3.5

This release of Teleport contains a bug fix.

* Fixed issue that caused Teleport Docker images to be built incorrectly. [#4201](https://github.com/gravitational/teleport/pull/4201)

## 4.3.4

This release of Teleport contains multiple bug fixes.

* Fixed issue that caused intermittent login failures when using PAM modules like `pam_loginuid.so` and `pam_selinux.so`. [#4133](https://github.com/gravitational/teleport/pull/4133)
* Fixed issue that required users to manually verify a certificate when exporting an identity file. [#4003](https://github.com/gravitational/teleport/pull/4003)
* Fixed issue that prevented local user creation using Firestore. [#4160](https://github.com/gravitational/teleport/pull/4160)
* Fixed issue that could cause `tsh` to panic when using a PEM file. [#4189](https://github.com/gravitational/teleport/pull/4189)

## 4.3.2

This release of Teleport contains multiple bug fixes.

* Reverted base OS in container images to Ubuntu. [#4054](https://github.com/gravitational/teleport/issues/4054)
* Fixed an issue that prevented changing the path for the Audit Log. [#3771](https://github.com/gravitational/teleport/issues/3771)
* Fixed an issue that allowed servers with invalid labels to be added to the cluster. [#4034](https://github.com/gravitational/teleport/issues/4034)
* Fixed an issue that caused Cloud Firestore to panic on startup. [#4041](https://github.com/gravitational/teleport/pull/4041)
* Fixed an error that would cause Teleport to fail to load with the error "list of proxies empty". [#4005](https://github.com/gravitational/teleport/issues/4005)
* Fixed an issue that would prevent playback of Kubernetes session [#4055](https://github.com/gravitational/teleport/issues/4055)
* Fixed regressions in the UI. [#4013](https://github.com/gravitational/teleport/issues/4013) [#4012](https://github.com/gravitational/teleport/issues/4012)  [#4035](https://github.com/gravitational/teleport/issues/4035)  [#4051](https://github.com/gravitational/teleport/issues/4051)  [#4044](https://github.com/gravitational/teleport/issues/4044)

## 4.3.0

This is a major Teleport release with a focus on new features, functionality, and bug fixes. It’s a substantial release and users can review [4.3 closed issues](https://github.com/gravitational/teleport/milestone/37?closed=1) on Github for details of all items.

#### New Features

##### Web UI

Teleport 4.3 includes a completely redesigned Web UI. The new Web UI expands the management functionality of a Teleport cluster and the user experience of using Teleport to make it easier and simpler to use. Teleport's new terminal provides a quick jumping-off point to access nodes and nodes on other clusters via the web.

Teleport's Web UI now exposes Teleport’s Audit log, letting auditors and administrators view Teleport access events, SSH events, recording session, and enhanced session recording all in one view.

##### Teleport Plugins

Teleport 4.3 introduces four new plugins that work out of the box with [Approval Workflow](https://gravitational.com/teleport/docs/enterprise/workflow/?utm_source=github&utm_medium=changelog&utm_campaign=4_3). These plugins allow you to automatically support role escalation with commonly used third party services. The built-in plugins are listed below.

*   [PagerDuty](https://gravitational.com/teleport/docs/enterprise/workflow/ssh_approval_pagerduty/?utm_source=github&utm_medium=changelog&utm_campaign=4_3)
*   [Jira Server](https://gravitational.com/teleport/docs/enterprise/workflow/ssh_approval_jira_server/?utm_source=github&utm_medium=changelog&utm_campaign=4_3) and [Jira Cloud](https://gravitational.com/teleport/docs/enterprise/workflow/ssh_approval_jira_cloud/?utm_source=github&utm_medium=changelog&utm_campaign=4_3)
*   [Slack](https://gravitational.com/teleport/docs/enterprise/workflow/ssh_approval_slack/?utm_source=github&utm_medium=changelog&utm_campaign=4_3)
*   [Mattermost](https://gravitational.com/teleport/docs/enterprise/workflow/ssh_approval_mattermost/?utm_source=github&utm_medium=changelog&utm_campaign=4_3)

#### Improvements

*   Added the ability for local users to reset their own passwords. [#2387](https://github.com/gravitational/teleport/pull/3287)
*   Added user impersonation (`kube_users)` support to Kubernetes Proxy. [#3369](https://github.com/gravitational/teleport/issues/3369)
*   Added support for third party S3-compatible storage for sessions. [#3057](https://github.com/gravitational/teleport/pull/3057)
*   Added support for GCP backend data stores. [#3766](https://github.com/gravitational/teleport/pull/3766)  [#3014](https://github.com/gravitational/teleport/pull/3014)
*   Added support for X11 forwarding to OpenSSH servers. [#3401](https://github.com/gravitational/teleport/issues/3401)
*   Added support for auth plugins in proxy `kubeconfig`. [#3655](https://github.com/gravitational/teleport/pull/3655)
*   Added support for OpenSSH-like escape sequence. [#3752](https://github.com/gravitational/teleport/pull/3752)
*   Added `--browser` flag to `tsh`. [#3737](https://github.com/gravitational/teleport/pull/3737)
*   Updated `teleport configure` output to be more useful out of the box. [#3429](https://github.com/gravitational/teleport/pull/3429)
*   Updated ability to only show SSO on the login page. [#2789](https://github.com/gravitational/teleport/issues/2789)
*   Updated help and support section in Web UI. [#3531](https://github.com/gravitational/teleport/issues/3531)
*   Updated default SSH signing algorithm to SHA-512 for new clusters. [#3777](https://github.com/gravitational/teleport/pull/3777)
*   Standardized audit event fields.

#### Fixes

*   Fixed removing existing user definitions in kubeconfig. [#3209](https://github.com/gravitational/teleport/issues/3749)
*   Fixed an issue where port forwarding could fail in certain circumstances. [#3749](https://github.com/gravitational/teleport/issues/3749)
*   Fixed temporary role grants issue when forwarding Kubernetes requests. [#3624](https://github.com/gravitational/teleport/pull/3624)
*   Fixed an issue that prevented copy/paste in the web termination. [#92](https://github.com/gravitational/webapps/issues/92)
*   Fixed an issue where the proxy did not test Kubernetes permissions at startup. [#3812](https://github.com/gravitational/teleport/pull/3812)
*   Fixed `tsh` and `gpg-agent` integration. [#3169](https://github.com/gravitational/teleport/issues/3169)
*   Fixed Vulnerabilities in Teleport Docker Image [https://quay.io/repository/gravitational/teleport?tab=tags](https://quay.io/repository/gravitational/teleport?tab=tags)

#### Documentation

*   [Moved SSO under Enterprise Section](https://gravitational.com/teleport/docs/enterprise/sso/ssh_sso/)
*   [Documented Teleport Plugins](https://gravitational.com/teleport/docs/enterprise/workflow/)
*   [Documented Kubernetes Role Mapping](https://gravitational.com/teleport/docs/kubernetes_ssh/#kubernetes-groups-and-users)

#### Upgrade Notes

Always follow the [recommended upgrade procedure](https://gravitational.com/teleport/docs/admin-guide/#upgrading-teleport?utm_source=github&utm_medium=changelog&utm_campaign=4_3) to upgrade to this version.

##### New Signing Algorithm

If you’re upgrading an existing version of Teleport, you may want to consider rotating CA to SHA-256 or SHA-512 for RSA SSH certificate signatures. The previous default was SHA-1, which is now considered to be weak against brute-force attacks. SHA-1 certificate signatures are also [no longer accepted](https://www.openssh.com/releasenotes.html) by OpenSSH versions 8.2 and above. All new Teleport clusters will default to SHA-512 based signatures. To upgrade an existing cluster, set the following in your `teleport.yaml`:

```
teleport:
    ca_signature_algo: "rsa-sha2-512"
```

Rotate the cluster CA, following [these docs](https://gravitational.com/teleport/docs/admin-guide/#certificate-rotation?utm_source=github&utm_medium=changelog&utm_campaign=4_3).

##### Web UI

Due to the number of changes included in the redesigned Web UI, some URLs and functionality have shifted. Refer to the following ticket for more details. [#3580](https://github.com/gravitational/teleport/issues/3580)

##### RBAC for Audit Log and Recorded Sessions

Teleport 4.3 has made the audit log accessible via the Web UI. Enterprise customers
can limit access by changing the options on the new `event` resource.

```yaml
# list and read audit log, including audit events and recorded sessions
- resources: [event]
  verbs: [list, read]
```

##### Kubernetes Permissions

The minimum set of Kubernetes permissions that need to be granted to Teleport proxies has been updated. If you use the Kubernetes integration, please make sure that the ClusterRole used by the proxy has [sufficient permissions](https://gravitational.com/teleport/docs/kubernetes_ssh#impersonation).

##### Path prefix for etcd

The [etcd backend](https://gravitational.com/teleport/docs/admin-guide/#using-etcd) now correctly uses the “prefix” config value when storing data. Upgrading from 4.2 to 4.3 will migrate the data as needed at startup. Make sure you follow our Teleport [upgrade guidance](https://gravitational.com/teleport/docs/admin-guide/#upgrading-teleport).

**Note: If you use an etcd backend with a non-default prefix and need to downgrade from 4.3 to 4.2, you should [backup Teleport data and restore it](https://gravitational.com/teleport/docs/admin-guide/#backing-up-teleport) into the downgraded cluster.**

## 4.2.12

This release of Teleport contains a security fix.

* Mitigated [CVE-2020-15216](https://nvd.nist.gov/vuln/detail/CVE-2020-15216) by updating github.com/russellhaering/goxmldsig.

### Details
A vulnerability was discovered in the `github.com/russellhaering/goxmldsig` library which is used by Teleport to validate the
signatures of XML files used to configure SAML 2.0 connectors. With a carefully crafted XML file, an attacker can completely
bypass XML signature validation and pass off an altered file as a signed one.

### Actions
The `goxmldsig` library has been updated upstream and Teleport 4.2.12 includes the fix. Any Enterprise SSO users using Okta,
Active Directory, OneLogin or custom SAML connectors should upgrade their auth servers to version 4.2.12 and restart Teleport.

If you are unable to upgrade immediately, we suggest deleting SAML connectors for all clusters until the updates can be applied.

## 4.2.11

This release of Teleport contains multiple bug fixes.

* Fixed an issue that prevented upload of session archives to NFS volumes. [#3780](https://github.com/gravitational/teleport/pull/3780)
* Fixed an issue with port forwarding that prevented TCP connections from being closed correctly. [#3801](https://github.com/gravitational/teleport/pull/3801)
* Fixed an issue in `tsh` that would cause connections to the Auth Server to fail on large clusters. [#3872](https://github.com/gravitational/teleport/pull/3872)
* Fixed an issue that prevented the use of Write-Only roles with S3 and GCS. [#3810](https://github.com/gravitational/teleport/pull/3810)

## 4.2.10

This release of Teleport contains multiple bug fixes.

* Fixed an issue that caused Teleport environment variables not to be available in PAM modules. [#3725](https://github.com/gravitational/teleport/pull/3725)
* Fixed an issue with `tsh login <clusterName>` not working correctly with Kubernetes clusters. [#3693](https://github.com/gravitational/teleport/issues/3693)

## 4.2.9

This release of Teleport contains multiple bug fixes.

* Fixed an issue where double `tsh login` would be required to login to a leaf cluster. [#3639](https://github.com/gravitational/teleport/pull/3639)
* Fixed an issue that was preventing connection reuse. [#3613](https://github.com/gravitational/teleport/pull/3613)
* Fixed an issue that could cause `tsh ls` to return stale results. [#3536](https://github.com/gravitational/teleport/pull/3536)

## 4.2.8

This release of Teleport contains multiple bug fixes.

* Fixed issue where `^C` would not terminate `tsh`. [#3456](https://github.com/gravitational/teleport/pull/3456)
* Fixed an issue where enhanced session recording could cause Teleport to panic. [#3506](https://github.com/gravitational/teleport/pull/3506)

## 4.2.7

As part of a routine security audit of Teleport, a security vulnerability was discovered that affects all recent releases of Teleport. We strongly suggest upgrading to the latest patched release to mitigate this vulnerability.

### Details

Due to a flaw in how the Teleport Web UI handled host certificate validation, host certificate validation was disabled for clusters where connections were terminated at the node. This means that an attacker could impersonate a Teleport node without detection when connecting through the Web UI.

Clusters where sessions were terminated at the proxy (recording proxy mode) are not affected.

Command line programs like `tsh` (or `ssh`) are not affected by this vulnerability.

### Actions

To mitigate this issue, upgrade and restart all Teleport proxy processes.

## 4.2.6

This release of Teleport contains a bug fix.

* Fixed a regression in certificate reissuance that could cause nodes to not start. [#3449](https://github.com/gravitational/teleport/pull/3449)

## 4.2.5

This release of Teleport contains multiple bug fixes.

* Added support for custom OIDC prompts. [#3409](https://github.com/gravitational/teleport/pull/3409)
* Added support for `kubernetes_users` in roles. [#3409](https://github.com/gravitational/teleport/pull/3404)
* Added support for extended variable interpolation. [#3409](https://github.com/gravitational/teleport/pull/3404)
* Added SameSite attribute to CSRF cookie. [#3441](https://github.com/gravitational/teleport/pull/3441)

## 4.2.4

This release of Teleport contains bug fixes.

* Fixed issue where Teleport could connect to the wrong node and added support to connect via UUID. [#2396](https://github.com/gravitational/teleport/issues/2396)
* Fixed issue where `tsh login` would fail to output identity when using the `--out` parameter. [#3339](https://github.com/gravitational/teleport/issues/3339)

## 4.2.3

This release of Teleport contains bug and security fixes.

* Mitigated [CVE-2020-9283](https://groups.google.com/forum/#!msg/golang-announce/3L45YRc91SY/ywEPcKLnGQAJ) by updating golang.org/x/crypto.
* Fixed PAM integration to support user creation upon login. [#3317](https://github.com/gravitational/teleport/pull/3317) [#3346](//github.com/gravitational/teleport/pull/3346)
* Improved Teleport performance on large IoT clusters. [#3227](https://github.com/gravitational/teleport/issues/3227)
* Added support for PluginData to Teleport plugins. [#3286](https://github.com/gravitational/teleport/issues/3286) [#3298](https://github.com/gravitational/teleport/issues/3298)

## 4.2.2

This release of Teleport contains bug fixes and improvements.

* Fixed a regression in role mapping between trusted clusters. [#3252](https://github.com/gravitational/teleport/issues/3252)
* Improved variety of issues with Enhanced Session Recording including support for more opearting systems and install from packages. [#3279](https://github.com/gravitational/teleport/pull/3279)

## 4.2.1

This release of Teleport contains bug fixes and minor usability improvements.

* New build command for client-only (`tsh`) .pkg builds. [#3159](https://github.com/gravitational/teleport/pull/3159)
* Added support for etcd password auth. [#3234](https://github.com/gravitational/teleport/pull/3234)
* Added third-party s3 support. [#3234](https://github.com/gravitational/teleport/pull/3234)
* Fixed an issue where access-request event system fails when cache is enabled. [#3223](https://github.com/gravitational/teleport/pull/3223)
* Fixed cgroup resolution so enhanced session recording works on Debian based distributions. [#3215](https://github.com/gravitational/teleport/pull/3215)

## 4.2.0

This is a minor Teleport release with a focus on new features and bug fixes.

### Improvements

* Alpha: Enhanced Session Recording lets you know what's really happening during a Teleport Session. [#2948](https://github.com/gravitational/teleport/issues/2948)
* Alpha: Workflows API lets admins escalate RBAC roles in response to user requests. [Read the docs](./enterprise/workflow). [#3006](https://github.com/gravitational/teleport/issues/3006)
* Beta: Teleport provides HA Support using Firestore and Google Cloud Storage using Google Cloud Platform. [Read the docs](./setup/deployments/gcp.mdx). [#2821](https://github.com/gravitational/teleport/pull/2821)
* Remote tctl execution is now possible. [Read the docs](./setup/reference/cli.mdx#tctl). [#1525](https://github.com/gravitational/teleport/issues/1525) [#2991](https://github.com/gravitational/teleport/issues/2991)

### Fixes

* Fixed issue in socks4 when rendering remote address [#3110](https://github.com/gravitational/teleport/issues/3110)

### Documentation

* Adopting root/leaf terminology for trusted clusters. [Trusted cluster documentation](./setup/admin/trustedclusters.mdx).
* Documented Teleport FedRAMP & FIPS Support. [FedRAMP & FIPS documentation](./enterprise/fedramp.mdx).

## 4.1.11

This release of Teleport contains a security fix.

* Mitigated [CVE-2020-15216](https://nvd.nist.gov/vuln/detail/CVE-2020-15216) by updating github.com/russellhaering/goxmldsig.

### Details
A vulnerability was discovered in the `github.com/russellhaering/goxmldsig` library which is used by Teleport to validate the
signatures of XML files used to configure SAML 2.0 connectors. With a carefully crafted XML file, an attacker can completely
bypass XML signature validation and pass off an altered file as a signed one.

### Actions
The `goxmldsig` library has been updated upstream and Teleport 4.1.11 includes the fix. Any Enterprise SSO users using Okta,
Active Directory, OneLogin or custom SAML connectors should upgrade their auth servers to version 4.1.11 and restart Teleport.

If you are unable to upgrade immediately, we suggest deleting SAML connectors for all clusters until the updates can be applied.

## 4.1.10

As part of a routine security audit of Teleport, a security vulnerability was discovered that affects all recent releases of Teleport. We strongly suggest upgrading to the latest patched release to mitigate this vulnerability.

### Details

Due to a flaw in how the Teleport Web UI handled host certificate validation, host certificate validation was disabled for clusters where connections were terminated at the node. This means that an attacker could impersonate a Teleport node without detection when connecting through the Web UI.

Clusters where sessions were terminated at the proxy (recording proxy mode) are not affected.

Command line programs like `tsh` (or `ssh`) are not affected by this vulnerability.

### Actions

To mitigate this issue, upgrade and restart all Teleport proxy processes.

## 4.1.9

This release of Teleport contains a security fix.

* Mitigated [CVE-2020-9283](https://groups.google.com/forum/#!msg/golang-announce/3L45YRc91SY/ywEPcKLnGQAJ) by updating golang.org/x/crypto.

## 4.1.8

This release of Teleport contains a bug fix.

* Fixed a regression in role mapping between trusted clusters. [#3252](https://github.com/gravitational/teleport/issues/3252)

## 4.1.7

This release of Teleport contains a bug fix.

* Fixed issue where the port forwarding option in a role was ignored. [#3208](https://github.com/gravitational/teleport/pull/3208)

## 4.1.6

This release of Teleport contains a bug fix.

* Fixed an issue that caused Teleport not to start with certain OIDC claims. [#3053](https://github.com/gravitational/teleport/issues/3053)

## 4.1.5

This release of Teleport adds support for an older version of Linux.

* Added RHEL/CentOS 6.x builds to the build pipeline. [#3175](https://github.com/gravitational/teleport/pull/3175)

## 4.1.4

This release of Teleport contains a bug fix.

* Fixed GSuite integration by adding support for service accounts. [#3122](https://github.com/gravitational/teleport/pull/3122)

## 4.1.3

This release of Teleport contains multiple bug fixes.

* Removed `TLS_RSA_WITH_AES_128_GCM_SHA{256,384}` from default ciphersuites due to compatibility issues with HTTP2.
* Fixed issues with `local_auth` for FIPS builds. [#3100](https://github.com/gravitational/teleport/pull/3100)
* Upgraded Go runtime to 1.13.2 to mitigate [CVE-2019-16276](https://github.com/golang/go/issues/34540) and [CVE-2019-17596](https://github.com/golang/go/issues/34960).

## 4.1.2

This release of Teleport contains improvements to the build code.

* Added support for building Docker images using the FIPS-compliant version of Teleport. The first of these images is quay.io/gravitational/teleport-ent:4.1.2-fips
* In future, these images will be automatically built for use by Teleport Enterprise customers.

## 4.1.1

This release of Teleport contains a bug fix.

* Fixed an issue with multi-cluster EKS when the Teleport proxy runs outside EKS. [#3070](https://github.com/gravitational/teleport/pull/3070)

## 4.1.0

This is a major Teleport release with a focus on stability and bug fixes.

### Improvements

* Support for IPv6. [#2124](https://github.com/gravitational/teleport/issues/2124)
* Kubernetes support does not require SNI. [#2766](https://github.com/gravitational/teleport/issues/2766)
* Support use of a path for `auth_token` in `teleport.yaml`. [#2515](https://github.com/gravitational/teleport/issues/2515)
* Implement ProxyJump compatibility. [#2543](https://github.com/gravitational/teleport/issues/2543)
* Audit logs should show roles. [#2823](https://github.com/gravitational/teleport/issues/2823)
* Allow `tsh` to go background and without executing remote command. [#2297](https://github.com/gravitational/teleport/issues/2297)
* Provide a high level tool to backup and restore the cluster state. [#2480](https://github.com/gravitational/teleport/issues/2480)
* Investigate nodes using stale list when connecting to proxies (discovery protocol). [#2832](https://github.com/gravitational/teleport/issues/2832)

### Fixes

* Proxy can hang due to invalid OIDC connector. [#2690](https://github.com/gravitational/teleport/issues/2690)
* Proper `-D` flag parsing. [#2663](https://github.com/gravitational/teleport/issues/2663)
* `tsh` status does not show correct cluster name. [#2671](https://github.com/gravitational/teleport/issues/2671)
* Teleport truncates MOTD with PAM. [#2477](https://github.com/gravitational/teleport/issues/2477)
* Miscellaneous fixes around error handling and reporting.

## 4.0.16

As part of a routine security audit of Teleport, a security vulnerability was discovered that affects all recent releases of Teleport. We strongly suggest upgrading to the latest patched release to mitigate this vulnerability.

### Details

Due to a flaw in how the Teleport Web UI handled host certificate validation, host certificate validation was disabled for clusters where connections were terminated at the node. This means that an attacker could impersonate a Teleport node without detection when connecting through the Web UI.

Clusters where sessions were terminated at the proxy (recording proxy mode) are not affected.

Command line programs like `tsh` (or `ssh`) are not affected by this vulnerability.

### Actions

To mitigate this issue, upgrade and restart all Teleport proxy processes.

## 4.0.15

This release of Teleport contains a security fix.

* Mitigated [CVE-2020-9283](https://groups.google.com/forum/#!msg/golang-announce/3L45YRc91SY/ywEPcKLnGQAJ) by updating golang.org/x/crypto.

## 4.0.14

This release of Teleport contains a bug fix.

* Fixed a regression in role mapping between trusted clusters. [#3252](https://github.com/gravitational/teleport/issues/3252)

## 4.1.13

This release of Teleport contains a bug fix.

* Fixed issue where the port forwarding option in a role was ignored. [#3208](https://github.com/gravitational/teleport/pull/3208)

## 4.0.12

This release of Teleport contains a bug fix.

* Fixed an issue that caused Teleport not to start with certain OIDC claims. [#3053](https://github.com/gravitational/teleport/issues/3053)

## 4.0.11

This release of Teleport adds support for an older version of Linux.

* Added RHEL/CentOS 6.x builds to the build pipeline. [#3175](https://github.com/gravitational/teleport/pull/3175)

## 4.0.10

This release of Teleport contains a bug fix.

* Fixed a goroutine leak that occured whenever a leaf cluster disconnected from the root cluster. [#3037](https://github.com/gravitational/teleport/pull/3037)

## 4.0.9

This release of Teleport contains a bug fix.

* Fixed issue where Web UI could not connect to older nodes within a cluster. [#2993](https://github.com/gravitational/teleport/pull/2993)

## 4.0.8

This release of Teleport contains two bug fixes.

### Description

* Fixed issue where new versions of `tsh` could not connect to older clusters. [#2969](https://github.com/gravitational/teleport/pull/2969)
* Fixed trait encoding to be more robust. [#2970](https://github.com/gravitational/teleport/pull/2970)

## 4.0.6

This release of Teleport contains a bug fix.

* Fixed issue introduced in 4.0.5 that broke session recording when using the recording proxy. [#2957](https://github.com/gravitational/teleport/pull/2957)

## 4.0.4

This release of Teleport contains a bug fix.

* Fixed a memory leak in the cache module. [#2892](https://github.com/gravitational/teleport/pull/2892)

## 4.0.3

* Reduced keep-alive interval to improve interoperability with popular load balancers. [#2845](https://github.com/gravitational/teleport/issues/2845)
* Fixed issue where non-RSA certificates were rejected when not in FIPS mode. [#2805](https://github.com/gravitational/teleport/pull/2879)

## 4.0.2

This release of Teleport contains multiple bug fixes.

* Fixed an issue that caused active sessions not to be shown. [#2801](https://github.com/gravitational/teleport/issues/2801)
* Fixed further issues with host certificate principal generation. [#2812](https://github.com/gravitational/teleport/pull/2812)
* Fixed issue where fetching CA would sometimes return not found. [#2805](https://github.com/gravitational/teleport/pull/2805)

## 4.0.1

This release of Teleport contains multiple bug fixes.

* Fixed issue that caused processes to be spawned with an incorrect GID. [#2791](https://github.com/gravitational/teleport/pull/2791)
* Fixed host certificate principal generation to only include hosts or IP addresses. [#2790](https://github.com/gravitational/teleport/pull/2790)
* Fixed issue preventing `tsh` 4.0 from connection to 3.2 clusters. [#2784](https://github.com/gravitational/teleport/pull/2784)

## 4.0.0

This is a major Teleport release which introduces support for Teleport Internet of Things (IoT). In addition to this new feature this release includes usability, performance, and bug fixes listed below.

### New Features

#### Teleport for IoT

With Teleport 4.0, nodes gain the ability to use reverse tunnels to dial back to a Teleport cluster to bypass firewall restrictions. This allows connections even to nodes that a cluster does not have direct network access to. Customers that have been using Trusted Clusters to achieve this can now utilize a unified interface to access all nodes within their infrastructure.

#### FedRamp Compliance

With this release of Teleport, we have built out the foundation to help Teleport Enterprise customers build and meet the requirements in a FedRAMP System Security Plan (SSP). This includes a FIPS 140-2 friendly build of Teleport Enterprise as well as a variety of improvements to aid in complying with security controls even in FedRAMP High environments.

### Improvements

* Teleport now support 10,000 remote connections to a single Teleport cluster. [Using our recommend hardware setup.](./setup/operations/scaling.mdx#hardware-recommendations)
* Added ability to delete node using `tctl rm`. [#2685](https://github.com/gravitational/teleport/pull/2685)
* Output of `tsh ls` is now sorted by node name. [#2534](https://github.com/gravitational/teleport/pull/2534)

### Bug Fixes

* Switched to `xdg-open` to open a browser window on Linux. [#2536](https://github.com/gravitational/teleport/pull/2536)
* Increased SSO callback timeout to 180 seconds. [#2533](https://github.com/gravitational/teleport/pull/2533)
* Set permissions on TTY similar to OpenSSH. [#2508](https://github.com/gravitational/teleport/pull/2508)

The lists of improvements and bug fixes above mention only the significant changes, please take a look at the complete list on Github for more.

### Upgrading

Teleport 4.0 is backwards compatible with Teleport 3.2 and later. [Follow the recommended upgrade procedure to upgrade to this version.](https://gravitational.com/teleport/docs/admin-guide/#upgrading-teleport)

Note that due to substantial changes between Teleport 3.2 and 4.0, we recommend creating a backup of the backend datastore (DynamoDB, etcd, or dir) before upgrading a cluster to Teleport 4.0 to allow downgrades.

#### Notes on compatibility

Teleport has always validated host certificates when a client connects to a server, however prior to Teleport 4.0, Teleport did not validate the host the user requests a connection to is in the list of principals on the certificate. To ensure a seamless upgrade, make sure the hosts you connect to have the appropriate address set in `public_addr` in `teleport.yaml` before upgrading.

## 3.2.15

This release of Teleport contains a bug fix.

* Fixed a regression in role mapping between trusted clusters. [#3252](https://github.com/gravitational/teleport/issues/3252)

## 3.2.14

This release of Teleport contains a bug fix and a feature.

* Restore `CreateWebSession` method used by some integrations. [#3076](https://github.com/gravitational/teleport/pull/3076)
* Add Docker registry and Helm repository support to `tsh login`. [#3045](https://github.com/gravitational/teleport/pull/3045)

## 3.2.13

This release of Teleport contains a bug fix.

### Description

* Fixed issue with TLS certificate not included in identity exported by `tctl auth sign`. [#3001](https://github.com/gravitational/teleport/pull/3001)

## 3.2.12

This release of Teleport contains a bug fix.

* Fixed issue where Web UI could not connect to older nodes within a cluster. [#2993](https://github.com/gravitational/teleport/pull/2993)

## 3.2.11

This release of Teleport contains two bug fixes.

* Fixed issue where new versions of `tsh` could not connect to older clusters. [#2969](https://github.com/gravitational/teleport/pull/2969)
* Fixed trait encoding to be more robust. [#2970](https://github.com/gravitational/teleport/pull/2970)

## 3.2.9

This release of Teleport contains a bug fix.

* Fixed issue introduced in 3.2.8 that broke session recording when using the recording proxy. [#2957](https://github.com/gravitational/teleport/pull/2957)

## 3.2.4

This release of Teleport contains multiple bug fixes.

* Read cluster name from `TELEPORT_SITE` environment variable in `tsh`. [#2675](https://github.com/gravitational/teleport/pull/2675)
* Multiple improvements around logging in and saving `tsh` profiles. [#2657](https://github.com/gravitational/teleport/pull/2657)

## 3.2.2

This release of Teleport contains a bug fix.

#### Changes

* Fixed issue with `--bind-addr` implementation. [#2650](https://github.com/gravitational/teleport/pull/2650)

## 3.2.1

This release of Teleport contains a new feature.

#### Changes

* Added `--bind-addr` to force `tsh` to bind to a specific port during SSO login. [#2620](https://github.com/gravitational/teleport/issues/2620)

## 3.2

This version brings support for Amazon's managed Kubernetes offering (EKS).

Starting with this release, Teleport proxy uses [the impersonation API](https://kubernetes.io/docs/reference/access-authn-authz/authentication/#user-impersonation) instead of the [CSR API](https://kubernetes.io/docs/tasks/tls/managing-tls-in-a-cluster/#requesting-a-certificate).

## 3.1.14

This release of Teleport contains a bug fix.

* Fixed issue where Web UI could not connect to older nodes within a cluster. [#2993](https://github.com/gravitational/teleport/pull/2993)

## 3.1.13

This release of Teleport contains two bug fixes.

* Fixed issue where new versions of `tsh` could not connect to older clusters. [#2969](https://github.com/gravitational/teleport/pull/2969)
* Fixed trait encoding to be more robust. [#2970](https://github.com/gravitational/teleport/pull/2970)

## 3.1.11

This release of Teleport contains a bug fix.

* Fixed issue introduced in 3.1.10 that broke session recording when using the recording proxy. [#2957](https://github.com/gravitational/teleport/pull/2957)

## 3.1.8

This release of Teleport contains a bug fix.

#### Changes

* Fixed issue where SSO users TTL was set incorrectly. [#2564](https://github.com/gravitational/teleport/pull/2564)

## 3.1.7

This release of Teleport contains a bug fix.

#### Changes

* Fixed issue where `tctl users ls` output contained duplicates. [#2569](https://github.com/gravitational/teleport/issues/2569) [#2107](https://github.com/gravitational/teleport/issues/2107)

## 3.1.6

This release of Teleport contains bug fixes, security fixes, and user experience improvements.

#### Changes

* Use `xdg-open` instead of `sensible-browser` to open links on Linux. [#2454](https://github.com/gravitational/teleport/issues/2454)
* Increased SSO callback timeout to 180 seconds. [#2483](https://github.com/gravitational/teleport/issues/2483)
* Improved Teleport error messages when it fails to start. [#2525](https://github.com/gravitational/teleport/issues/2525)
* Sort `tsh ls` output by node name. [#2511](https://github.com/gravitational/teleport/issues/2511)
* Support different regions for S3 (sessions) and DynamoDB (audit log). [#2007](https://github.com/gravitational/teleport/issues/2007)
* Fixed syslog output even when Teleport is in debug mode. [#2550](https://github.com/gravitational/teleport/issues/2550)
* Fixed audit log naming conventions. [#2388](https://github.com/gravitational/teleport/issues/2388)
* Fixed issue where `~/.tsh/profile` was deleted upon logout. [#2546](https://github.com/gravitational/teleport/issues/2546)
* Fixed output of `tctl get` to be compatible with `tctl create`. [#2479](https://github.com/gravitational/teleport/issues/2479)
* Fixed issue where multiple file upload with `scp` did not work correctly. [#2094](https://github.com/gravitational/teleport/issues/2094)
* Correctly set permissions TTY. [#2540](https://github.com/gravitational/teleport/issues/2540)
* Mitigated scp issues when connected to malicious server [#2539](https://github.com/gravitational/teleport/issues/2539)

## 3.1.5

Teleport 3.1.5 contains a bug fix and security fix.

#### Bug fixes

* Fixed issue where certificate authorities were not fetched during every login. [#2526](https://github.com/gravitational/teleport/pull/2526)
* Upgraded Go to 1.11.5 to mitigate [CVE-2019-6486](https://groups.google.com/forum/#!topic/golang-announce/mVeX35iXuSw): CPU denial of service in P-521 and P-384 elliptic curve implementation.

## 3.1.4

Teleport 3.1.4 contains one new feature and two bug fixes.

#### New Feature

* Added support for GSuite as a SSO provider. [#2455](https://github.com/gravitational/teleport/issues/2455)

#### Bug fixes

* Fixed issue where Kubernetes groups were not being passed to remote clusters. [#2484](https://github.com/gravitational/teleport/pull/2484)
* Fixed issue where the client was pulling incorrect CA for trusted clusters. [#2487](https://github.com/gravitational/teleport/pull/2487)

## 3.1.3

Teleport 3.1.3 contains two security fixs.

#### Bugfixes

* Updated xterm.js to mitigate a [RCE in xterm.js](https://github.com/xtermjs/xterm.js/releases/tag/3.10.1).
* Mitigate potential timing attacks during bearer token authentication. [#2482](https://github.com/gravitational/teleport/pull/2482)
* Fixed `x509: certificate signed by unknown authority` error when connecting to DynamoDB within Gravitational publish Docker image. [#2473](https://github.com/gravitational/teleport/pull/2473)

## 3.1.2

Teleport 3.1.2 contains a security fix. We strongly encourage anyone running Teleport 3.1.1 to upgrade.

#### Bugfixes

* Due to the flaw in internal RBAC verification logic, a compromised node, trusted cluster or authenticated non-privileged user can craft special request to Teleport's internal auth server API to gain access to the private key material of the cluster's internal certificate authorities and elevate their privileges to gain full administrative access to the Teleport cluster. This vulnerability only affects authenticated clients, there is no known way to exploit this vulnerability outside the cluster for unauthenticated clients.

## 3.1.1

Teleport 3.1.1 contains a security fix. We strongly encourage anyone running Teleport 3.1.0 to upgrade.

* Upgraded Go to 1.11.4 to mitigate CVE-2018-16875: [CPU denial of service in chain validation](https://golang.org/issue/29233) Go. For customers using the RHEL5.x compatible release of Teleport, we've backported this fix to Go 1.9.7, before releasing RHEL 5.x compatible binaries.

## 3.1

This is a major Teleport release with a focus on backwards compatibility, stability, and bug fixes. Some of the improvements:

* Added support for regular expressions in RBAC label keys and values. [#2161](https://github.com/gravitational/teleport/issues/2161)
* Added support for configurable server side keep-alives. [#2334](https://github.com/gravitational/teleport/issues/2334)
* Added support for some `-o` to improve OpenSSH interoperability. [#2330](https://github.com/gravitational/teleport/issues/2330)
* Added i386 binaries as well as binaries built with older version of Go to support legacy systems. [#2277](https://github.com/gravitational/teleport/issues/2277)
* Added SOCKS5 support to `tsh`. [#1693](https://github.com/gravitational/teleport/issues/1693)
* Improved UX and security for nodes joining a cluster. [#2294](https://github.com/gravitational/teleport/issues/2294)
* Improved Kubernetes UX. [#2291](https://github.com/gravitational/teleport/issues/2291) [#2258](https://github.com/gravitational/teleport/issues/2258) [#2304](https://github.com/gravitational/teleport/issues/2304)
* Fixed bug that did not allow copy and paste of texts over 128 in the Web UI. [#2313](https://github.com/gravitational/teleport/issues/2313)
* Fixes issues with `scp` when using the Web UI. [#2300](https://github.com/gravitational/teleport/issues/2300)

## 3.0.5

Teleport 3.0.5 contains a security fix.

#### Bug fixes

* Upgraded Go to 1.11.5 to mitigate [CVE-2019-6486](https://groups.google.com/forum/#!topic/golang-announce/mVeX35iXuSw): CPU denial of service in P-521 and P-384 elliptic curve implementation.

## 3.0.4

Teleport 3.0.4 contains two security fixs.

#### Bugfixes

* Updated xterm.js to mitigate a [RCE in xterm.js](https://github.com/xtermjs/xterm.js/releases/tag/3.10.1).
* Mitigate potential timing attacks during bearer token authentication. [#2482](https://github.com/gravitational/teleport/pull/2482)

## 3.0.3

Teleport 3.0.3 contains a security fix. We strongly encourage anyone running Teleport 3.0.2 to upgrade.

#### Bugfixes

* Due to the flaw in internal RBAC verification logic, a compromised node, trusted cluster or authenticated non-privileged user can craft special request to Teleport's internal auth server API to gain access to the private key material of the cluster's internal certificate authorities and elevate their privileges to gain full administrative access to the Teleport cluster. This vulnerability only affects authenticated clients, there is no known way to exploit this vulnerability outside the cluster for unauthenticated clients.

## 3.0.2

Teleport 3.0.2 contains a security fix. We strongly encourage anyone running Teleport 3.0.1 to upgrade.

* Upgraded Go to 1.11.4 to mitigate CVE-2018-16875: [CPU denial of service in chain validation](https://golang.org/issue/29233) Go. For customers using the RHEL5.x compatible release of Teleport, we've backported this fix to Go 1.9.7, before releasing RHEL 5.x compatible binaries.

## 3.0.1

This release of Teleport contains the following bug fix:

* Fix regression that marked ADFS claims as invalid. [#2293](https://github.com/gravitational/teleport/pull/2293)

## 3.0

This is a major Teleport release which introduces support for Kubernetes
clusters. In addition to this new feature this release includes several
usability and performance improvements listed below.

#### Kubernetes Support

* `tsh login` can retreive and install certificates for both Kubernetes and SSH
  at the same time.
* Full audit log support for `kubectl` commands, including recording of the sessions
  if `kubectl exec` command was interactive.
* Unified (AKA "single pane of glass") RBAC for both SSH and Kubernetes permissions.

For more information about Kubernetes support, take a look at
the [Kubernetes and SSH Integration Guide](https://gravitational.com/teleport/docs/kubernetes_ssh/)

#### Improvements

* Teleport administrators can now fine-tune the enabled ciphersuites [#1999](https://github.com/gravitational/teleport/issues/1999)
* Improved user experience linking trusted clusters together [#1971](https://github.com/gravitational/teleport/issues/1971)
* All Teleport components (proxy, auth and nodes) now support `public_addr`
  setting which allows them to be hosted behind NAT/Load Balancers. [#1793](https://github.com/gravitational/teleport/issues/1793)
* We have documented the previously undocumented monitoring endpoints [#2103](https://github.com/gravitational/teleport/issues/2103)
* The `etcd` back-end has been updated to implement 3.3+ protocol. See the upgrading notes below.
* Listing nodes via `tsh ls` or the web UI no longer shows nodes that the currently logged in user has no access to. [#1954](https://github.com/gravitational/teleport/issues/1954)
* It is now possible to build `tsh` client on Windows. Note: only `tsh login` command is implemented. [#1996](https://github.com/gravitational/teleport/pull/1996).
* `-i` flag to `tsh login` is now guarantees to be non-interactive. [#2221](https://github.com/gravitational/teleport/issues/2221)

#### Bugfixes

* Removed the bogus error message "access denied to perform action create on user" [#2132](https://github.com/gravitational/teleport/issues/2132)
* `scp` implementation in "recording proxy" mode did not work correctly. [#2176](https://github.com/gravitational/teleport/issues/2176)
* Removed the limit of 8 trusted clusters with SSO. [#2192](https://github.com/gravitational/teleport/issues/2192)
* `tsh ls` now works correctly when executed on a remote/trusted cluster [#2204](https://github.com/gravitational/teleport/milestone/24?closed=1)

The lists of improvements and bug fixes above mention only the significant
changes, please take a look at [the complete list](https://github.com/gravitational/teleport/milestone/24?closed=1)
on Github for more.

#### Upgrading to 3.0

Follow the [recommended upgrade procedure](https://gravitational.com/teleport/docs/admin-guide/#upgrading-teleport)
to upgrade to this version.

**WARNING:** if you are using Teleport with the etcd back-end, make sure your
`etcd` version is 3.3 or newer prior to upgrading to Teleport 3.0.

## 2.7.9

Teleport 2.7.9 contains a security fix.

#### Bug fixes

* Upgraded Go to 1.11.5 to mitigate [CVE-2019-6486](https://groups.google.com/forum/#!topic/golang-announce/mVeX35iXuSw): CPU denial of service in P-521 and P-384 elliptic curve implementation.

## 2.7.8

Teleport 2.7.8 contains two security fixs.

#### Bugfixes

* Updated xterm.js to mitigate a [RCE in xterm.js](https://github.com/xtermjs/xterm.js/releases/tag/3.10.1).
* Mitigate potential timing attacks during bearer token authentication. [#2482](https://github.com/gravitational/teleport/pull/2482)

## 2.7.7

Teleport 2.7.7 contains two security fixes. We strongly encourage anyone running Teleport 2.7.6 to upgrade.

#### Bugfixes

* Due to the flaw in internal RBAC verification logic, a compromised node, trusted cluster or authenticated non-privileged user can craft special request to Teleport's internal auth server API to gain access to the private key material of the cluster's internal certificate authorities and elevate their privileges to gain full administrative access to the Teleport cluster. This vulnerability only affects authenticated clients, there is no known way to exploit this vulnerability outside the cluster for unauthenticated clients.
* Upgraded Go to 1.11.4 to mitigate CVE-2018-16875: CPU denial of service in chain validation Go.

## 2.7.6

This release of Teleport contains the following bug fix:

* Fix regression that marked ADFS claims as invalid. [#2293](https://github.com/gravitational/teleport/pull/2293)

## 2.7.5

This release of Teleport contains the following bug fix:

* Teleport auth servers do not delete temporary files named `/tmp/multipart-` [#2250](https://github.com/gravitational/teleport/issues/2250)

## 2.7.4

This release of Teleport focuses on bugfixes.

#### Bug Fixes

* Fixed issues with `client_idle_timeout`. [#2166](https://github.com/gravitational/teleport/issues/2166)
* Added support for scalar and list values for `node_labels` in roles. [#2136](https://github.com/gravitational/teleport/issues/2136)
* Improved font support on Ubuntu.

## 2.7.3

This release of Teleport focuses on bugfixes.

#### Bug Fixes

* Fixed issue that cause `failed executing request: user agent missing` missing error when upgrading from 2.6.

## 2.7.2

This release of Teleport focuses on bugfixes.

#### Bug Fixes

* Fixed issue in Teleport 2.7.2 where rollback to Go 1.9.7 was not complete for `linux-amd64` binaries.

## 2.7.1

This release of Teleport focuses on bugfixes.

#### Bug Fixes

* Rollback to Go 1.9.7 for users with custom CA running into `x509: certificate signed by unknown authority`.

## 2.7.0

The primary goal of 2.7.0 release was to address the community feedback and improve the performance and flexibility when running Teleport clusters with large number of nodes.

#### New Features

* The Web UI now includes `scp` (secure copy) functionality. This allows Windows users and other users of the Web UI to upload/download files into SSH nodes using a web browser.
* Fine-grained control over forceful session termination has been added [#1935](https://github.com/gravitational/teleport/issues/1935). It is now possible to:
  * Forcefully disconnect idle clients (no client activity) after a specified timeout.
  * Forcefully disconnect clients when their certificates expire in the middle of an active SSH session.

#### Performance Improvements

* Performance of SSH login commands have been improved on large clusters (thousands of nodes). [#2061](https://github.com/gravitational/teleport/issues/2061)
* DynamoDB storage back-end performance has been improved. [#2021](https://github.com/gravitational/teleport/issues/2021)
* Performance of session recording via a proxy has been improved [#1966](https://github.com/gravitational/teleport/issues/1966)
* Connections between trusted clusters are managed better [#2023](https://github.com/gravitational/teleport/issues/2023)

#### Bug Fixes

As awlays, this release contains several bug fixes. The full list can be seen [here](https://github.com/gravitational/teleport/milestone/25?closed=1). Here are some notable ones:

* It is now possible to issue certificates with a long TTL via admin's `auth sign` tool. Previously they were limited to 30 hours for undocumented reason. [1745](https://github.com/gravitational/teleport/issues/1745)
* Dynamic label values were shown as empty strings. [2056](https://github.com/gravitational/teleport/issues/2056)

#### Upgrading

Follow the [recommended upgrade procedure](https://gravitational.com/teleport/docs/admin-guide/#upgrading-teleport) to upgrade to this version.

## 2.6.9

This release of Teleport focuses on bugfixes.

#### Bug Fixes

* Fixed issue in Teleport 2.6.8 where rollback to Go 1.9.7 was not complete for `linux-amd64` binaries.

## 2.6.8

This release of Teleport focuses on bugfixes.

#### Bug Fixes

* Rollback to Go 1.9.7 for users with custom CA running into `x509: certificate signed by unknown authority`.

## 2.6.7

This release of Teleport focuses on bugfixes.

#### Bug Fixes

* Resolved dynamic label regression. [#2056](https://github.com/gravitational/teleport/issues/2056)

## 2.6.5

This release of Teleport focuses on bugfixes.

#### Bug Fixes

* Remote clusters no longer try to re-connect to proxies that have been permanently removed. [#2023](https://github.com/gravitational/teleport/issues/2023)
* Speed up login on systems with many users. [#2021](https://github.com/gravitational/teleport/issues/2021)
* Improve overall performance of the etcd backend. [#2030](https://github.com/gravitational/teleport/issues/2030)
* Role login validation now applies after variables have been substituted. [#2022](https://github.com/gravitational/teleport/issues/2022)

## 2.6.3

This release of Teleport focuses on bugfixes.

#### Bug Fixes

* Remote clusters no longer try to re-connect to proxies that have been permanently removed. [#2023](https://github.com/gravitational/teleport/issues/2023)
* Speed up login on systems with many users. [#2021](https://github.com/gravitational/teleport/issues/2021)
* Improve overall performance of the etcd backend. [#2030](https://github.com/gravitational/teleport/issues/2030)
* Role login validation now applies after variables have been substituted. [#2022](https://github.com/gravitational/teleport/issues/2022)

## 2.6.2

This release of Teleport focuses on bugfixes.

#### Bug Fixes

* Reduced go routine usage by the forwarding proxy. [#1966](https://github.com/gravitational/teleport/issues/1966)
* Teleport no longer sends full version in the SSH handshake. [#970](https://github.com/gravitational/teleport/issues/970)
* Force flag works correctly for Trusted Clusters. [#1871](https://github.com/gravitational/teleport/issues/1871)
* Allow manual creation of Certificate Authorities. [#2001](https://github.com/gravitational/teleport/pull/2001)
* Include Teleport username in port forwarding events. [#2004](https://github.com/gravitational/teleport/pull/2004)
* Allow `tctl auth sign` to create user certificate with arbitrary TTL values. [#1745](https://github.com/gravitational/teleport/issues/1745)
* Upgrade to Go 1.10.3. [#2008](https://github.com/gravitational/teleport/pull/2008)

## 2.6.1

This release of Teleport focuses on bugfixes.

#### Bug Fixes

* Use ciphers, KEX, and MAC algorithms from Teleport configuration in reverse tunnel server. [#1984](https://github.com/gravitational/teleport/pull/1984)
* Update path sanitizer it allow `@`. [#1985](https://github.com/gravitational/teleport/pull/1985)

## 2.6.0

This release of Teleport brings new features, significant performance and
usability improvements as well usual bugfixes.

During this release cycle, the Teleport source code has been audited for
security vulnerabilities by Cure53 and this release (2.6.0) contains patches
for the discovered problems.

#### New Features

* Support for DynamoDB for storing the audit log events. [#1755](https://github.com/gravitational/teleport/issues/1755)
* Support for Amazon S3 for storing the recorded SSH sessions. [#1755](https://github.com/gravitational/teleport/issues/1755)
* Support for rotating certificate authorities (CA rotation). [#1899] (https://github.com/gravitational/teleport/pull/1899)
* Integration with Linux PAM (pluggable authentication modules) subsystem. [#742](https://github.com/gravitational/teleport/issues/742) and [#1766](https://github.com/gravitational/teleport/issues/1766)
* The new CLI command `tsh status` shows users which Teleport clusters they are authenticated with. [#1628](https://github.com/gravitational/teleport/issues/1628)

Additionally, Teleport 2.6.0 has been submitted to the AWS marketplace. Soon
AWS users will be able to create properly configured, secure and highly
available Teleport clusters with ease.

#### Configuration Changes

* Role templates (depreciated in Teleport 2.3) were fully removed. We recommend
  migrating to role variables which are documented [here](./access-controls/guides/role-templates.mdx)

* Resource names (like roles, connectors, trusted clusters) can no longer
  contain unicode or other special characters. Update the names of all user
  created resources to only include characters, hyphens, and dots.

* `advertise_ip` has been deprecated and replaced with `public_addr` setting. See [#1803](https://github.com/gravitational/teleport/issues/1803)
  The existing configuration files will still work, but we advise Teleport
  administrators to update it to reflect the new format.

* Teleport no longer uses `boltdb` back-end for storing cluster state _by
  default_.  The new default is called `dir` and it uses simple JSON files
  stored in `/var/lib/teleport/backend`. This change applies to brand new
  Teleport installations, the existing clusters will continue to use `boltdb`.

* The default set of enabled cryptographic primitives has been
  updated to reflect the latest state of SSH and TLS security. [#1856](https://github.com/gravitational/teleport/issues/1856).

#### Bug Fixes

The list of most visible bug fixes in this release:

* `tsh` now properly handles Ctrl+C [#1882](https://github.com/gravitational/teleport/issues/1882)
* High CPU utilization on ARM platforms during daemon start-up. [#1886](https://github.com/gravitational/teleport/issues/1886)
* Terminal window size can get out of sync on AWS. [#1874](https://github.com/gravitational/teleport/issues/1874)
* Some CLI commands print errors twice. [#1889](https://github.com/gravitational/teleport/issues/1889)
* SSH session playback can be interrupted for long sessions. [#1774](https://github.com/gravitational/teleport/issues/1774)
* Processing `HUP` UNIX signal is unreliable when `teleport` daemon runs under `systemd`. [#1844](https://github.com/gravitational/teleport/issues/1844)

You can see the full list of 2.6.0 changes [here](https://github.com/gravitational/teleport/milestone/22?closed=1).

#### Upgrading

Follow the [recommended upgrade procedure](https://gravitational.com/teleport/docs/admin-guide/#upgrading-teleport)
to upgrade to this version.

## 2.5.7

This release of Teleport focuses on bugfixes.

#### Bug Fixes

* Allow creation of users from `tctl create`. [#1949](https://github.com/gravitational/teleport/pull/1949)

## 2.5.6

This release of Teleport focuses on bugfixes.

#### Bug Fixes

* Improvements to Teleport HUP signal handling for more reliable reload.  [#1844](https://github.com/gravitational/teleport/issues/1844)
* Restore output format of `tctl nodes add --format=json`. [#1846](https://github.com/gravitational/teleport/issues/1846)

## 2.5.5

This release of Teleport focuses on bugfixes.

#### Bug Fixes

* Allow creation of multiple sessions per connection (fixes Ansible issues with the recording proxy). [#1811](https://github.com/gravitational/teleport/issues/1811)

## 2.5.4

This release of Teleport focuses on bugfixes.

#### Bug Fixes

* Only reset SIGINT handler if it has not been set to ignore. [#1814](https://github.com/gravitational/teleport/pull/1814)
* Improvement of user-visible errors. [#1798](https://github.com/gravitational/teleport/issues/1798) [#1779](https://github.com/gravitational/teleport/issues/1779)

## 2.5.3

This release of Teleport focuses on bugfixes.

#### Bug Fixes

* Fix logging, collect status of forked processes. [#1785](https://github.com/gravitational/teleport/issues/1785) [#1776](https://github.com/gravitational/teleport/issues/1776)
* Turn off proxy support when no-tls is used. [#1800](https://github.com/gravitational/teleport/issues/1800)
* Correct the signup URL. [#1777](https://github.com/gravitational/teleport/issues/1777)
* Fix GitHub team pagination issues. [#1734](https://github.com/gravitational/teleport/issues/1734)
* Increase global dial timeout to 30 seconds. [#1760](https://github.com/gravitational/teleport/issues/1760)
* Reuse existing singing key. [#1713](https://github.com/gravitational/teleport/issues/1713)
* Don't panic on channel failures. [#1808](https://github.com/gravitational/teleport/pull/1808)

## 2.5.2

This release of Teleport includes bug fixes and regression fixes.

#### Bug Fixes

* Run session migration in the background. [#1784](https://github.com/gravitational/teleport/pull/1784)
* Include node name in regenerated host certificates. [#1786](https://github.com/gravitational/teleport/issues/1786)

## 2.5.1

This release of Teleport fixes a regression in Teleport binaries.

#### Bug Fixes

* Binaries for macOS have been rebuilt to resolve "certificate signed by a unknown authority" issue.

## 2.5.0

This is a major release of Teleport. Its goal is to make cloud-native
deployments easier. Numerous AWS users have contributed feedback to this
release, which includes:

#### New Features

* Auth servers in highly available (HA) configuration can share the same `/var/lib/teleport`
  data directory when it's hosted on NFS (or AWS EFS).
  [#1351](https://github.com/gravitational/teleport/issues/1351)

* There is now an AWS reference deployment in `examples/aws` directory.  It
  uses Terraform and demonstrates how to deploy large Teleport clusters on AWS
  using best practices like auto-scaling groups, security groups, secrets
  management, load balancers, etc.

* The Teleport daemon now implements built-in connection draining which allows
  zero-downtime upgrades.
  [See documentation](https://gravitational.com/teleport/docs/admin-guide/#graceful-restarts).

* Dynamic join tokens for new nodes can now be explicitly set via `tctl node add --token`.
  This allows Teleport admins to use an external mechanism for generating
  cluster invitation tokens.
  [#1615](https://github.com/gravitational/teleport/pull/1615)

* Teleport now correctly manages certificates for accessing proxies behind a
  load balancer with the same domain name. The new configuration parameter
  `public_addr` must be used for this.
  [#1174](https://github.com/gravitational/teleport/issues/1174).

#### Improvements

* Switching to a new TLS-based auth server API improves performance of large clusters.
  [#1528](https://github.com/gravitational/teleport/issues/1528)

* Session recordings are now compressed by default using gzip. This reduces storage
  requirements by up to 80% in our real-world tests.
  [#1579](https://github.com/gravitational/teleport/issues/1528)

* More user-friendly authentication errors in Teleport audit log helps Teleport admins
  troubleshoot configuration errors when integrating with SAML/OIDC providers.
  [#1554](https://github.com/gravitational/teleport/issues/1554),
  [#1553](https://github.com/gravitational/teleport/issues/1553),
  [#1599](https://github.com/gravitational/teleport/issues/1599)

* `tsh` client will now report if a server's API is no longer compatible.

#### Bug Fixes

* `tsh logout` will now correctly log out from all active Teleport sessions. This is useful
  for users who're connected to multiple Teleport clusters at the same time.
  [#1541](https://github.com/gravitational/teleport/issues/1541)

* When parsing YAML, Teleport now supports `--` list item separator to create
  multiple resources with a single `tctl create` command.
  [#1663](https://github.com/gravitational/teleport/issues/1663)

* Fixed a panic in the Web UI backend [#1558](https://github.com/gravitational/teleport/issues/1558)

#### Behavior Changes

Certain components of Teleport behave differently in version 2.5. It is important to
note that these changes are not breaking Teleport functionality. They improve
Teleport behavior on large clusters deployed on highly dynamic cloud
environments such as AWS. This includes:

* Session list in the Web UI is now limited to 1,000 sessions.
* The audit log and recorded session storage has been moved from `/var/lib/teleport/log`
  to `/var/lib/teleport/log/<auth-server-id>`. This is related to [#1351](https://github.com/gravitational/teleport/issues/1351)
  described above.
* When connecting a trusted cluster users can no longer pick an arbitrary name for them.
  Their own (local) names will be used, i.e. the `cluster_name` setting now defines how
  the cluster is seen from the outside.
  [#1543](https://github.com/gravitational/teleport/issues/1543)

## 2.4.7

This release of Teleport contains a bugfix.

#### Bug Fixes

* Only reset SIGINT handler if it has not been set to ignore. [#1814](https://github.com/gravitational/teleport/pull/1814)

## 2.4.6

This release of Teleport focuses on bugfixes.

#### Bug Fixes

* Increase global dial timeout to 30 seconds. [#1760](https://github.com/gravitational/teleport/issues/1760)
* Don't panic on channel failures. [#1808](https://github.com/gravitational/teleport/pull/1808)

## 2.4.5

This release of Teleport fixes a regression in Teleport binaries.

#### Bug Fixes

* Binaries for macOS have been rebuilt to resolve "certificate signed by a unknown authority" issue.

## 2.4.4

This release of Teleport focuses on bugfixes.

#### Bug Fixes

* Resolved `tsh logout` regression. [#1541](https://github.com/gravitational/teleport/issues/1541)
* Binaries for supported platforms all built with Go 1.9.2.

## 2.4.3

This release of Teleport focuses on bugfixes.

#### Bug Fixes

* Resolved "access denied" regression in Trusted Clusters. [#1733](https://github.com/gravitational/teleport/issues/1733)
* Key written with wrong username to `~/.tsh`. [#1749](https://github.com/gravitational/teleport/issues/1749)
* Resolved Trusted Clusters toggling regression. [#1751](https://github.com/gravitational/teleport/issues/1751)

## 2.4.2

This release of Teleport focuses on bugfixes.

#### Bug Fixes

* Wait for copy to complete before propagating exit-status. [#1646](https://github.com/gravitational/teleport/issues/1646)
* Don't discard initial bytes in HTTP CONNECT tunnel. [#1659](https://github.com/gravitational/teleport/issues/1659)
* Pass caching key generator to services and use cache in recording proxy. [#1639](https://github.com/gravitational/teleport/issues/1639)
* Only display "Change Password" in UI for local users. [#1669](https://github.com/gravitational/teleport/issues/1669)
* Update Singup URL. [#1643](https://github.com/gravitational/teleport/issues/1643)
* Improved Teleport version reporting. [#1538](https://github.com/gravitational/teleport/issues/1538)
* Fixed regressions in terminal size handling and Trusted Clusters introduced in 2.4.1. [#1674](https://github.com/gravitational/teleport/issues/1674) [#1692](https://github.com/gravitational/teleport/issues/1692)

## 2.4.1

This release is focused on fixing a few regressions in Teleport as well as
adding a new feature.

#### New Features

* Exposed the `--compat` flag to Web UI users. [#1542](https://github.com/gravitational/teleport/issues/1542)

#### Bug Fixes

* Wrap lines correctly on initial login. [#1087](https://github.com/gravitational/teleport/issues/1087)
* Accept port numbers larger than `32767`: [#1576](https://github.com/gravitational/teleport/issues/1576)
* Don't show the `Join` button when using the recording proxy. [#1421](https://github.com/gravitational/teleport/issues/1421)
* Don't double record sessions when using the recording proxy and Teleport nodes. [#1582](https://github.com/gravitational/teleport/issues/1582)
* Fixed regressions in `tsh login` and `tsh logout`. [#1611](https://github.com/gravitational/teleport/issues/1611) [#1541](https://github.com/gravitational/teleport/issues/1541)

## 2.4.0

This release adds two major new features and a few improvements and bugfixes.

#### New Features

* New Commercial Teleport Editions: "Pro" and "Business" allow users to
  purchase a Teleport subscription without signing contracts.
* Teleport now supports SSH session recording even for nodes running OpenSSH [#1327](https://github.com/gravitational/teleport/issues/1327)
  This feature is called "recording proxy mode".
* Users of open source edition of Teleport can now authenticate against Github [#1445](https://github.com/gravitational/teleport/issues/1445)
* The Web UI now supports persistent URLs to Teleport nodes which can be
  integrated into 3rd party web apps. [#1511](https://github.com/gravitational/teleport/issues/1511)
* Session recording can now be turned off [#1430](https://github.com/gravitational/teleport/pull/1430)

#### Deprecated Features

* Teleport client `tsh` no longer supports being an SSH agent. We recommend
  using build-in SSH agents for MacOS and Linux, like `ssh-agent` from
  `openssh-client` package.

#### Bug Fixes

There have been numerous small usability and performance improvements, but some
notable fixed bugs are listed below:

* Resource (file descriptor) leak [#1433](https://github.com/gravitational/teleport/issues/1433)
* Correct handling of the terminal type [#1402](https://github.com/gravitational/teleport/issues/1402)
* Crash on startup [#1395](https://github.com/gravitational/teleport/issues/1395)

## 2.3.5

This release is focused on fixing a few regressions in configuration and UI/UX.

#### Improvements

* Updated documentation to accurately reflect 2.3 changes
* Web UI can use introspection so users can skip explicitly specifying SSH port [#1410](https://github.com/gravitational/teleport/issues/1410)

#### Bug fixes

* Fixed issue of 2FA users getting prematurely locked out [#1347](https://github.com/gravitational/teleport/issues/1347)
* UI (regression) when invite link is expired, nothing is shown to the user  [#1400](https://github.com/gravitational/teleport/issues/1400)
* OIDC regression with some providers [#1371](https://github.com/gravitational/teleport/issues/1371)
* Legacy configuration for trusted clusters regression: [#1381](https://github.com/gravitational/teleport/issues/1381)
* Dynamic tokens for adding nodes: "access denied" [#1348](https://github.com/gravitational/teleport/issues/1348)

## 2.3.1

#### Bug fixes

* Added CSRF protection to login endpoint. [#1356](https://github.com/gravitational/teleport/issues/1356)
* Proxy subsystem handling is more robust. [#1336](https://github.com/gravitational/teleport/issues/1336)

## 2.3

This release focus was to increase Teleport user experience in the following areas:

* Easier configuration via `tctl` resource commands.
* Improved documentation, with expanded 'examples' directory.
* Improved CLI interface.
* Web UI improvements.

#### Improvements

* Web UI: users can connect to OpenSSH servers using the Web UI.
* Web UI now supports arbitrarty SSH logins, in addition to role-defined ones, for better compatibility with OpenSSH.
* CLI: trusted clusters can now be managed on the fly without having to edit Teleport configuration. [#1137](https://github.com/gravitational/teleport/issues/1137)
* CLI: `tsh login` supports exporting a user identity into a file to be used later with OpenSSH.
* `tsh agent` command has been deprecated: users are expected to use native SSH Agents on their platforms.

#### Teleport Enterprise

* More granular RBAC rules [#1092](https://github.com/gravitational/teleport/issues/1092)
* Role definitions now support templates. [#1120](https://github.com/gravitational/teleport/issues/1120)
* Authentication: Teleport now supports multilpe OIDC/SAML endpoints.
* Configuration: local authentication is always enabled as a fallback if a SAML/OIDC endpoints go offline.
* Configuration: SAML/OIDC endpoints can be created on the fly using `tctl` and without having to edit configuration file or restart Teleport.
* Web UI: it is now easier to turn a trusted cluster on/off [#1199](https://github.com/gravitational/teleport/issues/1199).

#### Bug Fixes

* Proper handling of `ENV_SUPATH` from `login.defs` [#1004](https://github.com/gravitational/teleport/pull/1004)
* Reverse tunnels would periodically lose connectivity. [#1156](https://github.com/gravitational/teleport/issues/1156)
* `tsh` now stores user identities in a format compatible with OpenSSH. [1171](https://github.com/gravitational/teleport/issues/1171).

## 2.2.7

#### Bug fixes

* Updated YAML parsing library. [#1226](https://github.com/gravitational/teleport/pull/1226)

## 2.2.6

#### Bug fixes

* Fixed issue with SSH dial potentially hanging indefinitely. [#1153](https://github.com/gravitational/teleport/issues/1153)

## 2.2.5

#### Bug fixes

* Fixed issue where node did not have correct permissions. [#1151](https://github.com/gravitational/teleport/issues/1151)

## 2.2.4

#### Bug fixes

* Fixed issue with remote tunnel timeouts. [#1140](https://github.com/gravitational/teleport/issues/1140).

## 2.2.3

### Bug fixes

* Fixed issue with Trusted Clusters where a clusters could lose its signing keys. [#1050](https://github.com/gravitational/teleport/issues/1050).
* Fixed SAML signing certificate export in Enterprise. [#1109](https://github.com/gravitational/teleport/issues/1109).

## 2.2.2

### Bug fixes

* Fixed an issue where in certain situations `tctl ls` would not work. [#1102](https://github.com/gravitational/teleport/issues/1102).

## 2.2.1

### Improvements

* Added `--compat=oldssh` to both `tsh` and `tctl` that can be used to request certificates in the legacy format (no roles in extensions). [#1083](https://github.com/gravitational/teleport/issues/1083)

### Bugfixes

* Fixed multiple regressions when using SAML with dynamic roles. [#1080](https://github.com/gravitational/teleport/issues/1080)

## 2.2.0

### Features

* HTTP CONNECT tunneling for Trusted Clusters. [#860](https://github.com/gravitational/teleport/issues/860)
* Long lived certificates and identity export which can be used for automation. [#1033](https://github.com/gravitational/teleport/issues/1033)
* New terminal for Web UI. [#933](https://github.com/gravitational/teleport/issues/933)
* Read user environment files. [#1014](https://github.com/gravitational/teleport/issues/1014)
* Improvements to Auth Server resiliency and availability. [#1071](https://github.com/gravitational/teleport/issues/1071)
* Server side configuration of support ciphers, key exchange (KEX) algorithms, and MAC algorithms. [#1062](https://github.com/gravitational/teleport/issues/1062)
* Renaming `tsh` to `ssh` or making a symlink `tsh -> ssh` removes the need to type `tsh ssh`, making it compatible with familiar `ssh user@host`. [#929](https://github.com/gravitational/teleport/issues/929)

### Enterprise Features

* SAML 2.0. [#1070](https://github.com/gravitational/teleport/issues/1070)
* Role mapping for Trusted Clusters. [#983](https://github.com/gravitational/teleport/issues/983)
* ACR parsing for OIDC identity providers. [#901](https://github.com/gravitational/teleport/issues/901)

### Improvements

* Improvements to OpenSSH interoperability.
  * Certificate export format changes to match OpenSSH. [#1068](https://github.com/gravitational/teleport/issues/1068)
  * CA export format changes to match OpenSSH. [#918](https://github.com/gravitational/teleport/issues/918)
  * Improvements to `scp` implementation to fix incompatibility issues. [#1048](https://github.com/gravitational/teleport/issues/1048)
  * OpenSSH keep alive messages are now processed correctly. [#963](https://github.com/gravitational/teleport/issues/963)
* `tsh` profile is now always read. [#1047](https://github.com/gravitational/teleport/issues/1047)
* Correct signal handling when Teleport is launched using sysvinit. [#981](https://github.com/gravitational/teleport/issues/981)
* Role templates now automatically fill out default values when omitted. [#912](https://github.com/gravitational/teleport/issues/912)

## 2.0.6

### Bugfixes

* Fixed regression in TLP-01-009.

## 2.0.5

Teleport 2.0.5 contains a variety of security fixes. We strongly encourage anyone running Teleport 2.0.0 and above to upgrade to 2.0.5.

The most pressing issues (a phishing attack which can potentially be used to extract plaintext credentials and an attack where an already authenticated user can escalate privileges) can be resolved by upgrading the web proxy. However, however all nodes need to be upgraded to mitigate all vulnerabilities.

### Bugfixes

* Patch for TLP-01-001 and TLP-01-003: Check redirect.
* Patch for TLP-01-004: Always check is namespace is valid.
* Patch for TLP-01-005: Check user principal when joining session.
* Patch for TLP-01-006 and TLP-01-007: Validate Session ID.
* Patch for TLP-01-008: Use a fake hash for password authentication if user does not exist.
* Patch for TLP-01-009: Command injection in scp.

## 2.0.4

### Bugfixes

* Roles created the the Web UI now have `node` resource. [#949](https://github.com/gravitational/teleport/pull/949)

## 2.0.3

### Bugfixes

* Execute commands using user's shell.  [#943](https://github.com/gravitational/teleport/pull/943)
* Allow users to read their own roles. [#941](https://github.com/gravitational/teleport/pull/941)
* Fix User CA import. [#919](https://github.com/gravitational/teleport/pull/919)
* Role template defaults. [#916](https://github.com/gravitational/teleport/pull/916)
* Skip UserInfo if not provided. [#915](https://github.com/gravitational/teleport/pull/915)

## 2.0.2

### Bugfixes

* Agent socket had wrong permissions. [#936](https://github.com/gravitational/teleport/pull/936)

## 2.0.1

### Features

* Introduced Dynamic Roles. [#897](https://github.com/gravitational/teleport/pull/897)

### Improvements

* Improved OpenSSH interoperability. [#902](https://github.com/gravitational/teleport/pull/902), [#911](https://github.com/gravitational/teleport/pull/911)
* Enhanced OIDC Functionality. [#882](https://github.com/gravitational/teleport/pull/882)

### Bugfixes

* Fixed Regressions. [#874](https://github.com/gravitational/teleport/pull/874), [#876](https://github.com/gravitational/teleport/pull/876), [#883](https://github.com/gravitational/teleport/pull/883), [#892](https://github.com/gravitational/teleport/pull/892), and [#906](https://github.com/gravitational/teleport/pull/906)

## 2.0

This is a major new release of Teleport.

### Features

* Native support for DynamoDB back-end for storing cluster state.
* It is now possible to turn off 2nd factor authentication.
* 2nd factor now uses TOTP. #522
* New and easy to use framework for implementing secret storage plug-ins.
* Audit log format has been finalized and documented.
* Experimental simple file-based secret storage back-end.
* SSH agent forwarding.

### Improvements

* Friendlier CLI error messages.
* `tsh login` is now compatible with SSH agents.

### Enterprise Features

* Role-based access control (RBAC)
* Dynamic configuration: ability to manage roles and trusted clusters at runtime.

Full list of Github issues:
https://github.com/gravitational/teleport/milestone/8

## 1.3.2

v1.3.2 is a maintenance release which fixes a Web UI issue when in some cases
static web assets like custom fonts would not load properly.

### Bugfixes

* Issue #687 - broken web assets on some browsers.

## 1.3.1

v1.3.1 is a maintenance release which fixes a few issues found in 1.3

### Bugfixes

* Teleport session recorder can skip characters.
* U2F was enabled by default in "demo mode" if `teleport.yaml` file was missing.

### Improvements

* U2F documentation has been improved

## 1.3

This release includes several major new features and it's recommended for production use.

### Features

* Support for hardware U2F keys for 2nd factor authentication.
* CLI client profiles: `tsh` can now remember its `--proxy` setting.
* tctl auth sign command to allow administrators to generate user session keys
* Web UI is now served directly from the executable. There is no more need for web
  assets in `/usr/local/share/teleport`

### Bugfixes

* Multiple auth servers in config doesn't work if the last on is not reachable. #593
* `tsh scp -r` does not handle directory upload properly #606

## 1.2

This is a maintenance release and it's a drop-in replacement for previous versions.

### Changes

* Usability bugfixes as can be seen here
* Updated documentation
* Added examples directory with sample configuration and systemd unit file.

## 1.1.0

This is a maintenance release meant to be a drop-in upgrade of previous versions.

### Changes

* User experience improvements: nicer error messages
* Better compatibility with ssh command: `-t` flag can be used to force allocation of TTY

## 1.0.5

This release was recommended for production with one reservation: time-limited
certificates did not work correctly in this release due to #529

* Improvements in performance and usability of the Web UI
* Smaller binary sizes thanks to Golang v1.7

### Bugfixes

* Wrong url to register new users. #497
* Logged in users inherit Teleport supplemental groups bug security. #507
* Joining a session running on a trusted cluster does not work. #504

## 1.0.4

This release only includes the addition of the ability to specify non-standard
HTTPS port for Teleport proxy for `tsh --proxy` flag.

## 1.0.3

This release only includes one major bugfix #486 plus minor changes not exposed
to OSS Teleport users.

### Bugfixes

* Guessing `advertise_ip` chooses IPv6 address space. #486

## 1.0

The first official release of Teleport!
