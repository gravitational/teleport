# Changelog

## 17.3.0

### Automatic Updates

17.3 introduces a new automatic update mechanism for system administrators to
control which Teleport version their agents are running. You can now configure
the agent update schedule and desired agent version via the `autoupdate_config`
and `autoupdate_version` resources.

Updates are performed by the new `teleport-update` binary. This new system is
package manager-agnostic and opt-in. Existing agents won't be automatically
enrolled, you can enroll existing 17.3+ agents by running `teleport-update
enable`.

`teleport-update` will become the new standard way of installing Teleport as it
always picks the appropriate Teleport edition (Community vs Enterprise), the
cluster's desired version, and the correct Teleport variant (e.g. FIPS-compliant
cryptography).

### Package layout changes

Starting with 17.3.0, the Teleport DEB and RPM packages, notably used by the
`apt`, `yum`, `dnf` and `zypper` package managers, will place the Teleport
binaries in `/opt/teleport` instead of `/usr/local/bin`.

The binaries will be symlinked to their previous location, no change should be
required in your scripts or systemd units.

This change allows us to do automatic updates without conflicting with the
package manager.

### Delegated joining for Oracle Cloud Infrastructure

Teleport agents running on Oracle Cloud Infrastructure (OCI) are now able to
join the Teleport cluster without a static join token.

### Stable UIDs for host-user creation

Teleport now provides the ability to create host users with stable UIDs across
the entire Teleport cluster.

### VNet for Windows

Teleport's VNet feature are now available for Windows, allowing users to access
TCP applications protected by Teleport as if they were on the same network.

### Improved GitHub Proxy enrollment flow

Teleport web UI now provides wizard-like guided enrollment flow for the new
GitHub Proxy integration.

### AWS Identity Center integration improvements

AWS Identity Center integration now supports using IAM authentication instead of
OIDC (useful for private clusters) and a hybrid setup that allows to use another
IdP as external identity source.

### Okta integration improvements

Teleport Okta integration now provides updated guided enrollment flow and will
allow updating integration settings (such as sync configuration or group
filters) without having to recreate the integration.

Note that the new enrollment flow uses OAuth authentication method instead of
API tokens. If the Okta integration is installed on v17.3 and the cluster is
downgraded the Okta plugin must be reinstalled to ensure proper functionality.

### Readiness endpoint changes

The Auth Service readiness now reflects the connectivity from the agent to the
backend storage, and the Proxy Service readiness also reflects its connectivity
to the Auth Service API. In case of Auth or backend storage failure, the agents
will now turn unready. This change ensures that control plane components can be
excluded from their relevant load-balancing pools. If you want to preserve the
old behaviour (the Auth Service agent or Proxy Service agent stays ready and
runs in degraded mode), you can tune the readiness setting to become unready
after a high number of failed probes.

### Other fixes and improvements

* Added `tctl edit` support for Identity Center plugin resources. [#52605](https://github.com/gravitational/teleport/pull/52605)
* Added Oracle join method to web UI provision token editor. [#52599](https://github.com/gravitational/teleport/pull/52599)
* Added warnings to VNet on macOS about other software that might conflict with VNet, based on inspecting network routes on the system. [#52552](https://github.com/gravitational/teleport/pull/52552)
* Added auto-importing of Oracle Cloud tags. [#52543](https://github.com/gravitational/teleport/pull/52543)
* Added support for X509 revocations to Workload Identity. [#52503](https://github.com/gravitational/teleport/pull/52503)
* Git proxy commands executed in terminals now support interactive login prompts when the `tsh` session expires. [#52475](https://github.com/gravitational/teleport/pull/52475)
* Connect is now installed per-machine instead of per-user on Windows. [#52453](https://github.com/gravitational/teleport/pull/52453)
* Added `teleport-update` for default build. [#52361](https://github.com/gravitational/teleport/pull/52361)
* Improved sync performance in Identity Center integration. [#6102](https://github.com/gravitational/teleport.e/pull/6102)
* Delete related Git servers when deleting GitHub integration in the web UI. [#6101](https://github.com/gravitational/teleport.e/pull/6101)

## 17.2.9 (02/25/25)

* Updated go-jose/v4 to v4.0.5 (addresses CVE-2025-27144). [#52467](https://github.com/gravitational/teleport/pull/52467)
* Updated /x/crypto and /x/oauth2 (addresses CVE-2025-22869 and CVE-2025-22868). [#52437](https://github.com/gravitational/teleport/pull/52437)
* Fixed missing audit event on GitHub proxy RBAC failure. [#52427](https://github.com/gravitational/teleport/pull/52427)
* Allow to provide `tbot` configurations via environment variables. Update `tbot-distroless` image to run `start` command by default. [#52351](https://github.com/gravitational/teleport/pull/52351)
* Logging out from a cluster no longer clears the client autoupdate binaries. [#52337](https://github.com/gravitational/teleport/pull/52337)
* Added `tctl` installer for Identity Center integration. [#52336](https://github.com/gravitational/teleport/pull/52336)
* Added JSON response support to the `/webapi/auth/export` public certificate API endpoint. [#52325](https://github.com/gravitational/teleport/pull/52325)
* Resolves an issue with `tbot` where the web proxy port would be used instead of the SSH proxy port when ports separate mode is in use. [#52291](https://github.com/gravitational/teleport/pull/52291)
* Fix Azure SQL Servers connect failures when the database agent runs on a VM scale set. [#52267](https://github.com/gravitational/teleport/pull/52267)
* Add filter drop-downs and pinning support for the "Enroll a New Resource" page in the web UI. [#52176](https://github.com/gravitational/teleport/pull/52176)
* Improve latency and reduce resource consumption of generating Kubernetes certificates via `tctl auth sign` and `tsh kube login`. [#52146](https://github.com/gravitational/teleport/pull/52146)

## 17.2.8 (02/19/25)

* Fixed broken `Download Metadata File` button from the SAML enrolling resource flow in the web UI. [#52276](https://github.com/gravitational/teleport/pull/52276)
* Fixed broken `Refresh` button in the Access Monitoring reports page in the web UI. [#52276](https://github.com/gravitational/teleport/pull/52276)
* Fixed broken `Download app.zip` menu item in the Integrations list dropdown menu for Microsoft Teams in the web UI. [#52276](https://github.com/gravitational/teleport/pull/52276)
* Fixed `Unexpected end of JSON input` error in an otherwise successful web API call. [#52276](https://github.com/gravitational/teleport/pull/52276)
* Teleport Connect now features a new menu for quick access request management. [#52217](https://github.com/gravitational/teleport/pull/52217)
* Remove the ability of tctl to load the default configuration file on Windows. [#52188](https://github.com/gravitational/teleport/pull/52188)
* Tbot: support overriding `credential_ttl` and `renewal_interval` on most outputs and services. [#52185](https://github.com/gravitational/teleport/pull/52185)
* Fix an issue that GitHub integration CA gets deleted during Auth restart for non-software key stores like KMS. For broken GitHub integrations, the `integration` resource must be deleted and recreated. [#52149](https://github.com/gravitational/teleport/pull/52149)
* Added support for non-FIPS AWS endpoints for IAM and STS on FIPS binaries (`TELEPORT_UNSTABLE_DISABLE_AWS_FIPS=yes`) [#52127](https://github.com/gravitational/teleport/pull/52127)
* Introduced the allow_reissue property to the tbot identity output for compatibility with tsh based reissuance. [#52116](https://github.com/gravitational/teleport/pull/52116)

## 17.2.7 (02/13/25)

### Security Fixes

* Fixed security issue with arbitrary file reads on SSH nodes. [#52136](https://github.com/gravitational/teleport/pull/52136)
* Verify that cluster name of TLS peer certs matches the cluster name of the CA that issued it to prevent Auth bypasses. [#52130](https://github.com/gravitational/teleport/pull/52130)
* Reject authentication attempts from remote identities in the git forwarder. [#52126](https://github.com/gravitational/teleport/pull/52126)

### Other fixes and improvements
* Added an escape hatch to allow non-FIPS AWS endpoints on FIPS binaries (`TELEPORT_UNSTABLE_DISABLE_AWS_FIPS=yes`). [#52069](https://github.com/gravitational/teleport/pull/52069)
* Fixed Postgres database access control privileges auto-provisioning to grant USAGE on schemas as needed for table privileges and fixed an issue that prevented user privileges from being revoked at the end of their session in some cases. [#52047](https://github.com/gravitational/teleport/pull/52047)
* Updated OpenSSL to 3.0.16. [#52037](https://github.com/gravitational/teleport/pull/52037)
* Added ability to disable path-style S3 access for third-party endpoints. [#52009](https://github.com/gravitational/teleport/pull/52009)
* Fixed displaying Access List form when request reason is required. [#51998](https://github.com/gravitational/teleport/pull/51998)
* Fixed a bug in the WebUI where file transfers would always prompt for MFA, even when not required. [#51962](https://github.com/gravitational/teleport/pull/51962)
* Reduced CPU consumption required to map roles between clusters and perform trait to role resolution. [#51935](https://github.com/gravitational/teleport/pull/51935)
* Client tools managed updates require a base URL for the open-source build type. [#51931](https://github.com/gravitational/teleport/pull/51931)
* Fixed an issue leaf AWS console app shows "not found" error when root cluster has an app of the same name. [#51928](https://github.com/gravitational/teleport/pull/51928)
* Added `securityContext` value to the `tbot` Helm chart. [#51907](https://github.com/gravitational/teleport/pull/51907)
* Fixed an issue where required apps wouldn't be authenticated when launching an application from outside the Teleport Web UI. [#51873](https://github.com/gravitational/teleport/pull/51873)
* Prevent Teleport proxy failing to initialize when listener address's host component is empty. [#51864](https://github.com/gravitational/teleport/pull/51864)
* Fixed connecting to Apps in a leaf cluster when Per-session MFA is enabled. [#51853](https://github.com/gravitational/teleport/pull/51853)
* Updated Go to 1.23.6. [#51835](https://github.com/gravitational/teleport/pull/51835)
* Fixed bug where role `max_duration` is not respected unless request `max_duration` is set. [#51821](https://github.com/gravitational/teleport/pull/51821)
* Improved `instance.join` event error messaging. [#51779](https://github.com/gravitational/teleport/pull/51779)
* Teleport agents always create the `debug.sock` UNIX socket. The configuration field `debug_service.enabled` now controls if the debug and metrics endpoints are available via the UNIX socket. [#51771](https://github.com/gravitational/teleport/pull/51771)
* Backport new Azure integration functionality to v17, which allows the Discovery Service to fetch Azure resources and send them to the Access Graph. [#51725](https://github.com/gravitational/teleport/pull/51725)
* Added support for caching Microsoft Remote Desktop Services licenses. [#51684](https://github.com/gravitational/teleport/pull/51684)
* Added Audit Log statistics to `tctl top`. [#51655](https://github.com/gravitational/teleport/pull/51655)
* Redesigned the profile switcher in Teleport Connect for a more intuitive experience. Clusters now have distinct colors for easier identification, and readability is improved by preventing truncation of long user and cluster names. [#51654](https://github.com/gravitational/teleport/pull/51654)
* Fixed a regression that caused the Kubernetes Service to reuse expired tokens when accessing EKS, GKE and AKS clusters using dynamic credentials. [#51652](https://github.com/gravitational/teleport/pull/51652)
* Fixes issue where the Postgres backend would drop App Access events. [#51643](https://github.com/gravitational/teleport/pull/51643)
* Fixed a rare crash that can happen with malformed SAML connector. [#51634](https://github.com/gravitational/teleport/pull/51634)
* Fixed occasional Web UI session renewal issues (reverts "Avoid tight renewals for sessions with short TTL"). [#51601](https://github.com/gravitational/teleport/pull/51601)
* Introduced `tsh workload-identity issue-x509` as the replacement to `tsh svid issue` and which is compatible with the new WorkloadIdentity resource. [#51597](https://github.com/gravitational/teleport/pull/51597)
* Machine ID's new kubernetes/v2 service supports access to multiple Kubernetes clusters by name or label without needing to issue new identities. [#51535](https://github.com/gravitational/teleport/pull/51535)
* Quoted the `KUBECONFIG` environment variable output by the `tsh proxy kube` command. [#51523](https://github.com/gravitational/teleport/pull/51523)
* Fixed a bug where performing an admin action in the WebUI would hang indefinitely instead of getting an actionable error if the user has no MFA devices registered. [#51513](https://github.com/gravitational/teleport/pull/51513)
* Added support for continuous profile collection with Pyroscope. [#51477](https://github.com/gravitational/teleport/pull/51477)
* Added support for customizing the base URL for downloading Teleport packages used in client tools managed updates. [#51476](https://github.com/gravitational/teleport/pull/51476)
* Improved handling of client session termination during Kubernetes Exec sessions. The disconnection reason is now accurately returned for cases such as certificate expiration, forced lock activation, or idle timeout. [#51454](https://github.com/gravitational/teleport/pull/51454)
* Fixed an issue that prevented IPs provided in the `X-Forwarded-For` header from being honored in some scenarios when `TrustXForwardedFor` is enabled. [#51416](https://github.com/gravitational/teleport/pull/51416)
* Added support for multiple active CAs in the `/auth/export` endpoint. [#51415](https://github.com/gravitational/teleport/pull/51415)
* Fixed integrations status page in WebUI. [#51404](https://github.com/gravitational/teleport/pull/51404)
* Fixed a bug in GKE auto-discovery where the process failed to discover any clusters if the identity lacked permissions for one or more detected GCP project IDs. [#51399](https://github.com/gravitational/teleport/pull/51399)
* Introduced the new `workload_identity` resource for configuring Teleport Workload Identity. [#51288](https://github.com/gravitational/teleport/pull/51288)

Enterprise:
* Fixed a regression in the Web UI that prevented Access List members to view the Access List's they are member of.
* Fixed an issue with recreating Teleport resources for Okta applications with multiple embed links.
* Fixed an issue in the Identity Center principal assignment service that incorrectly reported a successful permission assignment delete request as a failed one.
* Fixed an issue in the Identity Center group import service which incorrectly handled import error event.

## 17.2.1 (01/22/2025)

### Security Fixes

* Improve Azure join validation by verifying subscription ID. [#51328](https://github.com/gravitational/teleport/pull/51328)

### Other Improvements and Fixes

* Added support for multiple active CAs in `tctl auth export`. [#51375](https://github.com/gravitational/teleport/pull/51375)
* Teleport Connect now shows a resource name in the status bar. [#51374](https://github.com/gravitational/teleport/pull/51374)
* Role presets now include default values for `github_permissions` and the `git_server` resource kind. `github_permissions` now supports traits. [#51369](https://github.com/gravitational/teleport/pull/51369)
* Fix backwards compatibility error where users were unable to login with Teleport Connect if Connect version is below v17.2.0 with Teleport cluster version v17.2.0. [#51368](https://github.com/gravitational/teleport/pull/51368)
* Added `wildcard-workload-identity-issuer` preset role to improve Day 0 experience with configuring Teleport Workload Identity. [#51341](https://github.com/gravitational/teleport/pull/51341)
* Added more granular audit logging surrounding SSH port forwarding. [#51325](https://github.com/gravitational/teleport/pull/51325)
* FIxes a bug causing the `terraform-provider` preset role to not automatically allow newly supported resources. [#51320](https://github.com/gravitational/teleport/pull/51320)
* GitHub server resource now shows in Web UI. [#51303](https://github.com/gravitational/teleport/pull/51303)

## 17.2.0 (01/21/2025)

### Per-session MFA via IdP

Teleport users can now satisfy per-session MFA checks by authenticating with an
external identity provider as an alternative to using second factors registered
with Teleport.

### GitHub access

Teleport now natively supports GitHub access allowing users to transparently
interact with Github with RBAC and audit logging support.

### Oracle Toad client support

Oracle Database Access users can now use Toad GUI client.

### Trusted clusters support for Kubernetes operator

Kubernetes operator users can now create trusted clusters using Kubernetes
custom resources.

### Other improvements and fixes

* Fixed WebAuthn attestation for Windows Hello. [#51247](https://github.com/gravitational/teleport/pull/51247)
* Include invited and reason fields in SessionStartEvents. [#51175](https://github.com/gravitational/teleport/pull/51175)
* Updated Go to 1.23.5. [#51172](https://github.com/gravitational/teleport/pull/51172)
* Fixed client tools auto-updates executed by aliases (causes recursive alias error). [#51154](https://github.com/gravitational/teleport/pull/51154)
* Support proxying Git commands for github.com. [#51086](https://github.com/gravitational/teleport/pull/51086)
* Assuming an Access Request in Teleport Connect now propagates elevated permissions to already opened Kubernetes tabs. [#51055](https://github.com/gravitational/teleport/pull/51055)
* Fixed AWS SigV4 parse errors in app access when the application omits the optional spaces between the SigV4 components. [#51043](https://github.com/gravitational/teleport/pull/51043)
* Fixed a Database Service bug where `db_service.resources.aws.assume_role_arn` settings could affect non-AWS dynamic databases or incorrectly override `db_service.aws.assume_role_arn` settings. [#51039](https://github.com/gravitational/teleport/pull/51039)
* Adds support for defining labels in the web UI Discover flows for single resource enroll (server, AWS and web applications, Kubernetes, EKS, RDS). [#51038](https://github.com/gravitational/teleport/pull/51038)
* Added support for using multi-port TCP apps in Teleport Connect without VNet. [#51014](https://github.com/gravitational/teleport/pull/51014)
* Fix naming conflict of DynamoDB audit event auto scaling policy. [#50990](https://github.com/gravitational/teleport/pull/50990)
* Prevent routing issues for agentless nodes that are created with non-UUID `metadata.name` fields. [#50924](https://github.com/gravitational/teleport/pull/50924)
* Honor the cluster routing strategy when client initiated host resolution via proxy templates or label matching is ambiguous. [#50799](https://github.com/gravitational/teleport/pull/50799)
* Emit audit events on access request expiry. [#50775](https://github.com/gravitational/teleport/pull/50775)
* Add full SSO MFA support for the WebUI. [#50529](https://github.com/gravitational/teleport/pull/50529)

Enterprise:
* Oracle: accept database certificates configuration used by Teleport Connect.

## 17.1.6 (1/13/25)

* Fix panic in EKS Auto Discovery. [#50998](https://github.com/gravitational/teleport/pull/50998)
* Add trusted clusters support to Kubernetes operator. [#50995](https://github.com/gravitational/teleport/pull/50995)

## 17.1.5 (1/10/25)

* Fixes an issue causing Azure join method to fail due to throttling. [#50928](https://github.com/gravitational/teleport/pull/50928)
* Fix Teleport Connect Oracle support. Requires updated Teleport database agents (v17.1.5+). [#50922](https://github.com/gravitational/teleport/pull/50922)
* Prevent quoting errors in log messages. [#50821](https://github.com/gravitational/teleport/pull/50821)
* Fixed an issue that could cause teleport event handlers to become stuck in an error loop upon upgrading to v17 (fix requires upgrading auth server). [#50820](https://github.com/gravitational/teleport/pull/50820)
* Add `user_agent` field to `db.session.start` audit events. [#50806](https://github.com/gravitational/teleport/pull/50806)
* Fix an issue "tsh aws ssm start-session" fails when KMS encryption is enabled. [#50796](https://github.com/gravitational/teleport/pull/50796)
* Support wider range of Oracle clients and simplified configuration. [#50740](https://github.com/gravitational/teleport/pull/50740)
* Added support for multi-port TCP apps to `tsh proxy app`. [#50691](https://github.com/gravitational/teleport/pull/50691)

## 17.1.4 (1/6/25)

* Fixed a Postgres database-access auto-user provisioning syntax error that caused a misleading debug level error log in most cases, unless the database admin is not a superuser and the database was upgraded from Postgres v15 or lower to Postgres v16 or higher, in which case the role "teleport-auto-user" must be granted to the database admin with the ADMIN option manually. [#50782](https://github.com/gravitational/teleport/pull/50782)
* Fixes a bug where S3 bucket details fail to fetch due to incorrect bucket region. [#50763](https://github.com/gravitational/teleport/pull/50763)
* Present connection errors to the Web UI terminal during database sessions. [#50700](https://github.com/gravitational/teleport/pull/50700)

Enterprise:
* Fix missing cleanup actions if the Oracle db connection is closed in its initial phases.
* Significantly improve Oracle client compatibility. Add server support for connections without wallet. For client-side change see: [#49753](https://github.com/gravitational/teleport/pull/49753).

## 17.1.3 (1/2/25)

* Fixes a bug where v16 Teleport cannot connect to v17.1.0, v17.1.1 and v17.1.2 clusters. [#50658](https://github.com/gravitational/teleport/pull/50658)
* Prevent panicking during shutdown when SQS consumer is disabled. [#50648](https://github.com/gravitational/teleport/pull/50648)
* Add a --labels flag to the tctl tokens ls command. [#50624](https://github.com/gravitational/teleport/pull/50624)

## 17.1.2 (12/30/24)

* Fixed a bug in the WebUI that could cause an access denied error when accessing application. [#50611](https://github.com/gravitational/teleport/pull/50611)
* Improve session playback initial delay caused by an additional events query. [#50592](https://github.com/gravitational/teleport/pull/50592)
* Fix a bug in the `tbot` Helm chart causing invalid configuration when both default and custom outputs were used. [#50526](https://github.com/gravitational/teleport/pull/50526)
* Restore the ability to play session recordings in the web UI without specifying the session duration in the URL. [#50459](https://github.com/gravitational/teleport/pull/50459)
* Fix regression in `tbot` on Linux causing the Kubernetes credential helper to fail. [#50413](https://github.com/gravitational/teleport/pull/50413)

## 17.1.1 (12/20/24)

**Warning**: 17.1.1 fixes a regression in 17.1.0 that causes SSH server heartbeats
to disappear after a few minutes. Please skip 17.1.0 and upgrade straight to 17.1.1
or above. [#50490](https://github.com/gravitational/teleport/pull/50490)

### Access requests support for AWS Identity Center

AWS Identity Center integration now allows users to request short or long term access to permission sets via Access Requests.

### Database access for PostgreSQL via web UI

Database access users can now connect to PostgreSQL databases connected to Teleport right from the web UI and use psql-style interface to query the database.

### Hosted email plugin for Access Requests

Users now have the ability to setup Mailgun or generic SMTP server for Access Request notifications using Teleport web UI without needing to self-host the email plugin.

### Multi-port support for VNet

Users now supports multiple ports (or a range of ports) with a single TCP application, and Teleport VNet will make all of the application's ports accessible on the virtual network.

### Graphical Role Editor

Teleport's web UI includes a new role editor that allows users to create and modify roles without resorting to a raw YAML editor.

### Granular SSH port forwarding controls

Teleport now allows cluster administrators to enable local and remote port forwarding separately rather than grouping both types of port forwarding behind a single option.

### Other improvements and fixes

* Fixed an issue that could cause some antivirus tools to block Teleport's Device Trust feature on Windows machines. [#50453](https://github.com/gravitational/teleport/pull/50453)
* Updates the UI login redirection service to honor redirection to `enterprise/saml-idp/sso` path even if user is already authenticated with Teleport. [#50442](https://github.com/gravitational/teleport/pull/50442)
* Reduced cluster state storage load in clusters with a large amount of resources. [#50430](https://github.com/gravitational/teleport/pull/50430)
* Updated golang.org/x/net to v0.33.0 (addresses CVE-2024-45338). [#50397](https://github.com/gravitational/teleport/pull/50397)
* Fixed an issue causing panics in SAML app or OIDC integration deletion relating to AWS Identity Center integration. [#50360](https://github.com/gravitational/teleport/pull/50360)
* Fix missing roles in Access Lists causing users to be locked out of their account. [#50298](https://github.com/gravitational/teleport/pull/50298)
* Added support for connecting to PostgreSQL databases using WebUI. [#50287](https://github.com/gravitational/teleport/pull/50287)
* Improved the performance of Teleport agents serving a large number of resources in Kubernetes. [#50279](https://github.com/gravitational/teleport/pull/50279)
* Improve performance of Kubernetes App Auto Discover. [#50269](https://github.com/gravitational/teleport/pull/50269)
* Added more granular access controls for SSH port forwarding. Access to remote or local port forwarding can now be controlled individually using the new `ssh_port_forwarding` role option. [#50241](https://github.com/gravitational/teleport/pull/50241)
* Properly close ssh port forwarding connections to prevent requests hanging indefinitely. [#50238](https://github.com/gravitational/teleport/pull/50238)
* Teleport's RDP client now sets the load balancing cookie to improve compatibility with local traffic managers. [#50226](https://github.com/gravitational/teleport/pull/50226)
* Fixes an intermittent EKS authentication failure when dealing with EKS auto-discovery. [#50197](https://github.com/gravitational/teleport/pull/50197)
* Expose /.well-known/jwks-okta public endpoint for Okta API services type App. [#50177](https://github.com/gravitational/teleport/pull/50177)
* Switched to a new role editor UI. [#50030](https://github.com/gravitational/teleport/pull/50030)
* Added support for multiple ports to TCP applications. [#49711](https://github.com/gravitational/teleport/pull/49711)
* Allow multiple consecutive occurrences of `-` and `.` in SSH server hostnames.  [#50410](https://github.com/gravitational/teleport/pull/50410)
* Fixed bug causing users to see notifications for their own access requests in some cases. [#50076](https://github.com/gravitational/teleport/pull/50076)
* Improved the cluster initialization process's ability to recovery from errors. [#49966](https://github.com/gravitational/teleport/pull/49966)

Enterprise:
* Adds AWS Account name to Identity Center Roles and resources. Some manual cleanup may be required where users and Access Lists have been assigned the obsolete roles.

## 17.0.5 (12/11/24)

* Updated golang.org/x/crypto to v0.31.0 (CVE-2024-45337). [#50078](https://github.com/gravitational/teleport/pull/50078)
* Fixed `tsh ssh -Y` when jumping between multiple servers. [#50031](https://github.com/gravitational/teleport/pull/50031)
* Reduced Auth memory consumption when agents join using the azure join method. [#49998](https://github.com/gravitational/teleport/pull/49998)
* Our OSS OS packages (rpm, deb, etc) now have up-to-date metadata. [#49962](https://github.com/gravitational/teleport/pull/49962)
* `tsh` correctly respects the --no-allow-passwordless flag. [#49933](https://github.com/gravitational/teleport/pull/49933)
* The web session authorization dialog in Teleport Connect is now a dedicated tab, which properly shows a re-login dialog when the local session is expired. [#49931](https://github.com/gravitational/teleport/pull/49931)
* Added an interactive mode for `tctl auth rotate`. [#49896](https://github.com/gravitational/teleport/pull/49896)
* Fixed a panic when the auth server does not provide a license expiry. [#49876](https://github.com/gravitational/teleport/pull/49876)

Enterprise:
* Fixed a panic occurring during SCIM push operations when resource.metadata is empty. [#5654](https://github.com/gravitational/teleport.e/pull/5654)
* Improved "IP mismatch" audit entries for device trust web. [#5642](https://github.com/gravitational/teleport.e/pull/5642)
* Fixed assigning suggested reviewers in the edge case when the user already has access to the requested resources. [#5629](https://github.com/gravitational/teleport.e/pull/5629)

## 17.0.4 (12/5/2024)

* Fixed a bug introduced in 17.0.3 breaking in-cluster joining on some Kubernetes clusters. [#49841](https://github.com/gravitational/teleport/pull/49841)
* SSH or Kubernetes information included for audit log list for start session events. [#49832](https://github.com/gravitational/teleport/pull/49832)
* Avoid tight web session renewals for sessions with short TTL (between 3m and 30s). [#49768](https://github.com/gravitational/teleport/pull/49768)
* Updated Go to 1.23.4. [#49758](https://github.com/gravitational/teleport/pull/49758)
* Fixed re-rendering bug when filtering Unified Resources. [#49744](https://github.com/gravitational/teleport/pull/49744)

## 17.0.3 (12/3/2024)

* Restore ability to disable multi-factor authentication for local users. [#49692](https://github.com/gravitational/teleport/pull/49692)
* Bumping one of our dependencies to a more secure version to address CVE-2024-53259. [#49662](https://github.com/gravitational/teleport/pull/49662)
* Add ability to configure resource labels in `teleport-cluster`'s operator sub-chart. [#49647](https://github.com/gravitational/teleport/pull/49647)
* Fixed proxy peering listener not using the exact address specified in `peer_listen_addr`. [#49589](https://github.com/gravitational/teleport/pull/49589)
* Teleport Connect now shows whether it is being used on a trusted device or if enrollment is required for full access. [#49577](https://github.com/gravitational/teleport/pull/49577)
* Kubernetes in-cluster joining now also accepts tokens whose audience is the Teleport cluster name (before it only allowed the default Kubernetes audience). Kubernetes JWKS joining is unchanged and still requires tokens with the cluster name in the audience. [#49556](https://github.com/gravitational/teleport/pull/49556)
* Session recording playback in the web UI is now searchable. [#49506](https://github.com/gravitational/teleport/pull/49506)
* Fixed an incorrect warning indicating that tsh v17.0.2 was incompatible with cluster v17.0.1, despite full compatibility. [#49491](https://github.com/gravitational/teleport/pull/49491)
* Increase CockroachDB setup timeout from 5 to 30 seconds. This mitigates the Auth Service not being able to configure TTL on slow CockroachDB event backends. [#49469](https://github.com/gravitational/teleport/pull/49469)
* Fixed a potential panic in login rule and SAML IdP expression parser. [#49429](https://github.com/gravitational/teleport/pull/49429)
* Support for long-running kube exec/port-forward, respect client_idle_timeout config. [#49421](https://github.com/gravitational/teleport/pull/49421)
* Fixed a permissions error with Postgres database user auto-provisioning that occurs when the database admin is not a superuser and the database is upgraded to Postgres v16 or higher. [#49390](https://github.com/gravitational/teleport/pull/49390)

Enterprise:
* Jamf Service sync audit events are attributed to "Jamf Service".
* Users can now see a list of their enrolled devices on their Account page.
* Add support for Entra ID groups being members of other groups using Nested Access Lists.
* Added support for requiring reason for Access Requests (with a new role.spec.allow.request.reason.mode setting).

## 17.0.2 (11/25/2024)

* Fixed missing user participants in session recordings listing for non-interactive Kubernetes recordings. [#49343](https://github.com/gravitational/teleport/pull/49343)
* Support delegated joining for Bitbucket Pipelines in Machine ID. [#49335](https://github.com/gravitational/teleport/pull/49335)
* Fix a bug in the Teleport Operator chart that causes the operator to not be able to watch secrets during secret injection. [#49327](https://github.com/gravitational/teleport/pull/49327)
* You can now search text within SSH sessions in the Web UI and Teleport Connect. [#49269](https://github.com/gravitational/teleport/pull/49269)
* Teleport Connect now refreshes the resources view after dropping an access request. [#49264](https://github.com/gravitational/teleport/pull/49264)
* Fixed an issue where `teleport park` processes could be leaked causing runaway resource usage. [#49260](https://github.com/gravitational/teleport/pull/49260)
* Fixed VNet not being able to connect to the daemon. [#49199](https://github.com/gravitational/teleport/pull/49199)
* The `tsh puttyconfig` command now disables GSSAPI auth settings to avoid a "Not Responding" condition in PuTTY. [#49189](https://github.com/gravitational/teleport/pull/49189)
* Allow Azure VMs to join from a different subscription than their managed identity. [#49156](https://github.com/gravitational/teleport/pull/49156)
* Fix an issue loading the license file when Teleport is started without a configuration file. [#49150](https://github.com/gravitational/teleport/pull/49150)
* Added support for directly configuring JWKS for GitHub joining for circumstances where the GHES is not reachable by the Teleport Auth Service. [#49049](https://github.com/gravitational/teleport/pull/49049)
* Fixed a bug where Access Lists imported from Microsoft Entra ID fail to be created if their display names include special characters. [#5551](https://github.com/gravitational/teleport.e/pull/5551)

## 17.0.1 (11/15/2024)

Teleport 17 brings the following new features and improvements:

- Refreshed web UI
- Modern signature algorithms
- (Preview) AWS IAM Identity Center integration
- Hardware key support for Teleport Connect
- Nested access lists
- Access lists UI/UX improvements
- Signed and notarized macOS assets
- Datadog Incident Management plugin for access requests
- Hosted Microsoft Teams plugin for access requests
- Dynamic registration for Windows desktops
- Support for images in web SSH sessions
- `tbot` CLI updates

### Description

#### Refreshed Web UI

We have updated and improved designs and added a new navigation menu to Teleport
17’s web UI to enhance its usability and scalability.

#### Modern signature algorithms

Teleport 17 admins have the option to use elliptic curve cryptography for the
majority of user, host, and certificate authority key material.

This includes Ed25519 SSH keys and ECDSA TLS keys, replacing the RSA keys used
today.

New clusters will leverage modern signature algorithms by default. Existing
Teleport clusters will continue to use RSA2048 until a CA rotation is performed.

#### (Preview) AWS IAM Identity Center integration

Teleport 17 integrates with AWS IAM Identity Center to allow users to sync and
manage AWS IC group members via Access Lists.

#### Hardware key support for Teleport Connect

We have extended Teleport 17’s support for hardware-backed private keys to
Teleport Connect.

#### Nested access lists

Teleport 17 admins and access list owners can add access lists as members in
other access lists.

#### Access lists UI/UX improvements

Teleport 17 web UI has an updated access lists page that will include the new
table view, improved search and filtering capabilities.

#### Signed and notarized macOS assets

Starting from Teleport 17 macOS `teleport.pkg` installer includes signed and
notarized `tsh.app` and `tctl.app` so downloading a separate tsh.pkg to use
Touch ID is no longer necessary.

In addition, Teleport 17 event handler and Terraform provider for macOS are also
signed and notarized.

#### Datadog Incident Management plugin for access requests

Teleport 17 supports PagerDuty-like integration with Datadog's [on-call](https://docs.datadoghq.com/service_management/on-call/)
and [incident management](https://docs.datadoghq.com/service_management/incident_management/)
APIs for access request notifications.

#### Hosted Microsoft Teams plugin for access requests

Teleport 17 adds support for Microsoft Teams integration for access request
notifications using Teleport web UI without needing to self-host the plugin.

#### Dynamic registration for Windows desktops

Dynamic registration allows Teleport administrators to register new Windows
desktops without having to update the static configuration files read by
Teleport Windows Desktop Service instances.

#### Support for images in web SSH sessions

The SSH console in Teleport’s web UI includes support for rendering images via
both the SIXEL and iTerm Inline Image Protocol (IIP).

#### tbot CLI updates

The `tbot` client now supports starting most outputs and services directly from
the command line with no need for a configuration file using the new
`tbot start <mode>` family of commands. If desired, a given command can be
converted to a YAML configuration file with `tbot configure <mode>`.

Additionally, `tctl` now supports inspection and management of bot instances using
the `tctl bots instances` family of commands. This allows onboarding of new
instances for existing bots with `tctl bots instances add`, and inspection of
existing instances with `tctl bots instances list`.

### Breaking changes and deprecations

#### macOS assets

Starting with version 17, Teleport no longer provides a separate `tsh.pkg` macOS
package.

Instead, `teleport.pkg` and all macOS tarballs include signed and notarized
`tsh.app` and `tctl.app`.

#### Enforced stricter requirements for SSH hostnames

Hostnames are only allowed if they are less than 257 characters and consist of
only alphanumeric characters and the symbols `.` and `-`.

Any hostname that violates the new restrictions will be changed, the original
hostname will be moved to the `teleport.internal/invalid-hostname` label for
discoverability.

Any Teleport agents with an invalid hostname will be replaced with the host UUID.
Any Agentless OpenSSH Servers with an invalid hostname will be replaced with
the host of the address, if it is valid, or a randomly generated identifier.
Any hosts with invalid hostnames should be updated to comply with the new
requirements to avoid Teleport renaming them.

#### `TELEPORT_ALLOW_NO_SECOND_FACTOR` removed

As of Teleport 16, multi-factor authentication is required for local users. To
assist with upgrades, Teleport 16 included a temporary opt-out mechanism via the
`TELEPORT_ALLOW_NO_SECOND_FACTOR` environment variable. This opt-out mechanism
has been removed.

#### TOTP for per-session MFA

Teleport 17 is the last release where `tsh` will allow for using TOTP with
per-session MFA. Starting with Teleport 18, `tsh` will require a strong webauthn
credential for per-session MFA.

TOTP will continue to be accepted for the initial login.

## 16.4.12 (12//18/2024)

* Updated golang.org/x/net to v0.33.0 (addresses CVE-2024-45338). [#50398](https://github.com/gravitational/teleport/pull/50398)
* Improved the performance of Teleport agents serving a large number of resources in Kubernetes. [#50280](https://github.com/gravitational/teleport/pull/50280)
* Improve performance of Kubernetes App Auto Discover. [#50268](https://github.com/gravitational/teleport/pull/50268)
* Properly close ssh port forwarding connections to prevent requests hanging indefinitely. [#50239](https://github.com/gravitational/teleport/pull/50239)
* Teleport's RDP client now sets the load balancing cookie to improve compatibility with local traffic managers. [#50225](https://github.com/gravitational/teleport/pull/50225)
* Fixes an intermittent EKS authentication failure when dealing with EKS auto-discovery. [#50198](https://github.com/gravitational/teleport/pull/50198)
* Improved the cluster initialization process's ability to recovery from errors. [#49967](https://github.com/gravitational/teleport/pull/49967)

## 16.4.11 (12/11/2024)

* Updated golang.org/x/crypto to v0.31.0 (CVE-2024-45337). [#50079](https://github.com/gravitational/teleport/pull/50079)
* Fix tsh ssh -Y when jumping between multiple servers. [#50032](https://github.com/gravitational/teleport/pull/50032)
* Fixed an issue preventing default shell assignment for host users. [#50003](https://github.com/gravitational/teleport/pull/50003)
* Reduce Auth memory consumption when agents join using the azure join method. [#49999](https://github.com/gravitational/teleport/pull/49999)
* Our OSS OS packages (rpm, deb, etc) now have up-to-date metadata. [#49963](https://github.com/gravitational/teleport/pull/49963)
* Tsh correctly respects the --no-allow-passwordless flag. [#49934](https://github.com/gravitational/teleport/pull/49934)
* The web session authorization dialog in Teleport Connect is now a dedicated tab, which properly shows a re-login dialog when the local session is expired. [#49932](https://github.com/gravitational/teleport/pull/49932)
* Prevent a panic if the Auth Service does not provide a license expiry. [#49877](https://github.com/gravitational/teleport/pull/49877)

Enterprise:
* Improved "IP mismatch" audit entries for device trust web.
* Fixed assigning suggested reviewers in the edge case when the user already has access to the requested resources.
* Users can now see a list of their enrolled devices on their Account page.
* Jamf Service sync audit events are attributed to "Jamf Service".
* Added license updater service.
* Fixed a bug where Access Lists imported from Microsoft Entra ID fail to be created if their display names include special characters.

## 16.4.10 (12/5/2024)

* Fixed a bug introduced in v16.4.9 breaking in-cluster joining on some Kubernetes clusters. [#49842](https://github.com/gravitational/teleport/pull/49842)
* SSH or Kubernetes information included for audit log list for start session events. [#49833](https://github.com/gravitational/teleport/pull/49833)
* Avoid tight web session renewals for sessions with short TTL (between 3m and 30s). [#49769](https://github.com/gravitational/teleport/pull/49769)
* Updated Go to 1.22.10. [#49759](https://github.com/gravitational/teleport/pull/49759)
* Added support for hardware keys in Teleport Connect. [#49701](https://github.com/gravitational/teleport/pull/49701)
* Auto-updates for client tools (`tctl` and `tsh`) are controlled by cluster configuration. [#48645](https://github.com/gravitational/teleport/pull/48645)

## 16.4.9 (12/3/2024)

* Add ability to configure resource labels in `teleport-cluster`'s operator sub-chart. [#49648](https://github.com/gravitational/teleport/pull/49648)
* Fixed proxy peering listener not using the exact address specified in `peer_listen_addr`. [#49590](https://github.com/gravitational/teleport/pull/49590)
* Teleport Connect now shows whether it is being used on a trusted device or if enrollment is required for full access. [#49578](https://github.com/gravitational/teleport/pull/49578)
* Kubernetes in-cluster joining now also accepts tokens whose audience is the Teleport cluster name (before it only allowed the default Kubernetes audience). Kubernetes JWKS joining is unchanged and still requires tokens with the cluster name in the audience. [#49557](https://github.com/gravitational/teleport/pull/49557)
* Restore interactive PAM authentication functionality when use_pam_auth is applied. [#49519](https://github.com/gravitational/teleport/pull/49519)
* Session recording playback in the web UI is now searchable. [#49507](https://github.com/gravitational/teleport/pull/49507)
* Increase CockroachDB setup timeout from 5 to 30 seconds. This mitigates the Auth Service not being able to configure TTL on slow CockroachDB event backends. [#49470](https://github.com/gravitational/teleport/pull/49470)
* Fixed a potential panic in login rule and SAML IdP expression parser. [#49431](https://github.com/gravitational/teleport/pull/49431)
* Support for long-running kube exec/port-forward, respect client_idle_timeout config. [#49423](https://github.com/gravitational/teleport/pull/49423)
* Fixed a permissions error with Postgres database user auto-provisioning that occurs when the database admin is not a superuser and the database is upgraded to Postgres v16 or higher. [#49389](https://github.com/gravitational/teleport/pull/49389)
* Teleport Connect now refreshes the resources view after dropping an Access Request. [#49348](https://github.com/gravitational/teleport/pull/49348)
* Fixed missing user participants in session recordings listing for non-interactive Kubernetes recordings. [#49344](https://github.com/gravitational/teleport/pull/49344)
* Support delegated joining for Bitbucket Pipelines in Machine ID. [#49337](https://github.com/gravitational/teleport/pull/49337)
* Fix a bug in the Teleport Operator chart that causes the operator to not be able to watch secrets during secret injection. [#49326](https://github.com/gravitational/teleport/pull/49326)
* You can now search text within ssh sessions in the Web UI and Teleport Connect. [#49270](https://github.com/gravitational/teleport/pull/49270)
* Fixed an issue where `teleport park` processes could be leaked causing runaway resource usage. [#49261](https://github.com/gravitational/teleport/pull/49261)
* Update tsh scp to respect proxy templates when resolving the remote host. [#49227](https://github.com/gravitational/teleport/pull/49227)
* The `tsh puttyconfig` command now disables GSSAPI auth settings to avoid a "Not Responding" condition in PuTTY. [#49190](https://github.com/gravitational/teleport/pull/49190)
* Resolved an issue that caused false positive errors incorrectly indicating that the YubiKey was in use by another application, while only tsh was accessing it. [#47952](https://github.com/gravitational/teleport/pull/47952)

Enterprise:
* Jamf Service sync audit events are attributed to "Jamf Service".
* Fixed a bug where Access Lists imported from Microsoft Entra ID fail to be created if their display names include special characters.

## 16.4.8 (11/19/2024)

* Allow Azure VMs to join from a different subscription than their managed identity. [#49157](https://github.com/gravitational/teleport/pull/49157)
* Fix an issue loading the license file when Teleport is started without a configuration file. [#49149](https://github.com/gravitational/teleport/pull/49149)
* Fixed a bug in the `teleport-cluster` Helm chart that can cause token mount to fail when using ArgoCD. [#49069](https://github.com/gravitational/teleport/pull/49069)
* Fixed app access regression to apps on leaf clusters. [#49056](https://github.com/gravitational/teleport/pull/49056)
* Added support for directly configuring JWKS for GitHub joining for circumstances where the GHES is not reachable by the Teleport Auth Service. [#49052](https://github.com/gravitational/teleport/pull/49052)
* Fixed issue resulting in excess CPU usage and connection resets when `teleport-event-handler` is under moderate to high load. [#49036](https://github.com/gravitational/teleport/pull/49036)
* Fixed OpenSSH remote port forwarding not working for localhost. [#49020](https://github.com/gravitational/teleport/pull/49020)
* Fixed `tsh app login` prompting for user login when multiple AWS roles are present. [#48997](https://github.com/gravitational/teleport/pull/48997)
* Fixed incorrect cluster name when querying for Kubernetes namespaces on a leaf cluster for Connect UI. [#48990](https://github.com/gravitational/teleport/pull/48990)
* Allow to override Teleport license secret name when using `teleport-cluster` Helm chart. [#48979](https://github.com/gravitational/teleport/pull/48979)
* Added periodic health checks between proxies in proxy peering. [#48929](https://github.com/gravitational/teleport/pull/48929)
* Fixed users not being able to connect to SQL server instances with PKINIT integration when the cluster is configured with different CAs for database access. [#48924](https://github.com/gravitational/teleport/pull/48924)
* Fix a bug in the Teleport Operator chart that causes the operator to not be able to list secrets during secret injection. [#48901](https://github.com/gravitational/teleport/pull/48901)
* The access graph poll interval is now configurable with the `discovery_service.poll_interval` field, whereas before it was fixed to a 15 minute interval. [#48861](https://github.com/gravitational/teleport/pull/48861)
* The web terminal now supports SIXEL and IIP image protocols. [#48842](https://github.com/gravitational/teleport/pull/48842)
* Ensure that agentless server information is provided in all audit events. [#48833](https://github.com/gravitational/teleport/pull/48833)
* Fixed missing Access Request metadata in `app.session.start` audit events. [#48804](https://github.com/gravitational/teleport/pull/48804)
* Fixed `missing GetDatabaseFunc` error when `tsh` connects MongoDB databases in cluster with a separate MongoDB port. [#48129](https://github.com/gravitational/teleport/pull/48129)
* Ensure that Teleport can re-establish broken LDAP connections. [#48008](https://github.com/gravitational/teleport/pull/48008)
* Improved handling of scoped token when setting up Okta integration. [#5503](https://github.com/gravitational/teleport.e/pull/5503)
* Fixed Access Request deletion reconciliation race condition in Okta integration HA setup. [#5385](https://github.com/gravitational/teleport.e/pull/5385)
* Extend support for `group` claim setting in Entra ID integration. [#5493](https://github.com/gravitational/teleport.e/pull/5493)

## 16.4.7 (11/11/2024)

* Fixed bug in Kubernetes session recordings where both root and leaf cluster recorded the same Kubernetes session. Recordings of leaf resources are only available in leaf clusters. [#48738](https://github.com/gravitational/teleport/pull/48738)
* Machine ID can now be forced to use the explicitly configured proxy address using the `TBOT_USE_PROXY_ADDR` environment variable. This should better support split proxy address operation. [#48675](https://github.com/gravitational/teleport/pull/48675)
* Fixed undefined error in open source version when clicking on `Add Application` tile in the Enroll Resources page in the Web UI. [#48616](https://github.com/gravitational/teleport/pull/48616)
* Updated Go to 1.22.9. [#48581](https://github.com/gravitational/teleport/pull/48581)
* The teleport-cluster Helm chart now uses the configured `serviceAccount.name` from chart values for its pre-deploy configuration check Jobs. [#48579](https://github.com/gravitational/teleport/pull/48579)
* Fixed a bug that prevented the Teleport UI from properly displaying Plugin Audit log details. [#48462](https://github.com/gravitational/teleport/pull/48462)
* Fixed an issue preventing migration of unmanaged users to Teleport host users when including `teleport-keep` in a role's `host_groups`. [#48455](https://github.com/gravitational/teleport/pull/48455)
* Fixed showing the list of Access Requests in Teleport Connect when a leaf cluster is selected in the cluster selector. [#48441](https://github.com/gravitational/teleport/pull/48441)
* Added Connect support for selecting Kubernetes namespaces during Access Requests. [#48413](https://github.com/gravitational/teleport/pull/48413)
* Fixed a rare "internal error" on older U2F authenticators when using tsh. [#48402](https://github.com/gravitational/teleport/pull/48402)
* Fixed `tsh play` not skipping idle time when `--skip-idle-time` was provided. [#48397](https://github.com/gravitational/teleport/pull/48397)
* Added a warning to `tctl edit` about dynamic edits to statically configured resources. [#48392](https://github.com/gravitational/teleport/pull/48392)
* Define a new `role.allow.request` field called `kubernetes_resources` that allows admins to define what kinds of Kubernetes resources a requester can make. [#48387](https://github.com/gravitational/teleport/pull/48387)
* Fixed a Teleport Kubernetes Operator bug that happened for OIDCConnector resources with non-nil `max_age`. [#48376](https://github.com/gravitational/teleport/pull/48376)
* Updated host user creation to prevent local password expiration policies from affecting Teleport managed users. [#48163](https://github.com/gravitational/teleport/pull/48163)
* Added support for Entra ID directory synchronization for clusters without public internet access. [#48089](https://github.com/gravitational/teleport/pull/48089)
* Fixed "Missing Region" error for teleport bootstrap commands. [#47995](https://github.com/gravitational/teleport/pull/47995)
* Fixed a bug that prevented selecting security groups during the Aurora database enrollment wizard in the web UI. [#47975](https://github.com/gravitational/teleport/pull/47975)
* During the Set Up Access of the Enroll New Resource flows, Okta users will be asked to change the role instead of entering the principals and getting an error afterwards. [#47957](https://github.com/gravitational/teleport/pull/47957)
* Fixed `teleport_connected_resource` metric overshooting after keepalive errors. [#47949](https://github.com/gravitational/teleport/pull/47949)
* Fixed an issue preventing connections with users whose configured home directories were inaccessible. [#47916](https://github.com/gravitational/teleport/pull/47916)
* Added a `resolve` command to tsh that may be used as the target for a Match exec condition in an SSH config. [#47868](https://github.com/gravitational/teleport/pull/47868)
* Respect `HTTP_PROXY` environment variables for Access Request integrations. [#47738](https://github.com/gravitational/teleport/pull/47738)
* Updated tsh ssh to support the `--` delimiter similar to openssh. It is now possible to execute a command via `tsh ssh user@host -- echo test` or `tsh ssh -- host uptime`. [#47493](https://github.com/gravitational/teleport/pull/47493)

Enterprise:
* Jamf requests from Teleport set "teleport/$version" as the User-Agent.
* Add Web UI support for selecting Kubernetes namespaces during Access Requests.
* Import user roles and traits when using the EntraID directory sync.

## 16.4.6 (10/22/2024)

### Security Fixes

#### [High] Privilege persistence in Okta SCIM-only integration

When Okta SCIM-only integration is enabled, in certain cases Teleport could
calculate the effective set of permission based on SSO user's stale traits. This
could allow a user who was unassigned from an Okta group to log into a Teleport
cluster once with a role granted by the unassigned group being present in their
effective role set.

Note: This issue only affects Teleport clusters that have installed a SCIM-only
Okta integration as described in this guide. If you have an Okta integration
with user sync enabled or only using Okta SSO auth connector to log into your
Teleport cluster without SCIM integration configured, you're unaffected. To
verify your configuration:

- Use `tctl get plugins/okta --format=json | jq ".[].spec.Settings.okta.sync_settings.sync_users"`
  command to check if you have Okta integration with user sync enabled. If it
  outputs null or false, you may be affected and should upgrade.
- Check SCIM provisioning settings for the Okta application you created or
  updated while following the SCIM-only setup guide. If SCIM provisioning is
  enabled, you may be affected and should upgrade.

We strongly recommend customers who use Okta SCIM integration to upgrade their
auth servers to version 16.3.0 or later. Teleport services other than auth
(proxy, SSH, Kubernetes, desktop, application, database and discovery) are not
impacted and do not need to be updated.

### Other improvements and fixes

* Added a new teleport_roles_total metric that exposes the number of roles which exist in a cluster. [#47812](https://github.com/gravitational/teleport/pull/47812)
* Teleport's Windows Desktop Service now filters domain-joined Linux hosts out during LDAP discovery. [#47773](https://github.com/gravitational/teleport/pull/47773)
* The `join_token.create` audit event has been enriched with additional metadata. [#47765](https://github.com/gravitational/teleport/pull/47765)
* Propagate resources configured in teleport-kube-agent chart values to post-install and post-delete hooks. [#47743](https://github.com/gravitational/teleport/pull/47743)
* Add support for the Datadog Incident Management plugin helm chart. [#47727](https://github.com/gravitational/teleport/pull/47727)
* Automatic device enrollment may be locally disabled using the TELEPORT_DEVICE_AUTO_ENROLL_DISABLED=1 environment variable. [#47720](https://github.com/gravitational/teleport/pull/47720)
* Fixed the Machine ID and GitHub Actions wizard. [#47708](https://github.com/gravitational/teleport/pull/47708)
* Added migration to update the old import_all_objects database object import rule to the new preset. [#47707](https://github.com/gravitational/teleport/pull/47707)
* Alter ServiceAccounts in the teleport-cluster Helm chart to automatically disable mounting of service account tokens on newer Kubernetes distributions, helping satisfy security linters. [#47703](https://github.com/gravitational/teleport/pull/47703)
* Avoid tsh auto-enroll escalation in machines without a TPM. [#47695](https://github.com/gravitational/teleport/pull/47695)
* Fixed a bug that prevented users from canceling `tsh scan keys` executions. [#47658](https://github.com/gravitational/teleport/pull/47658)
* Postgres database session start events now include the Postgres backend PID for the session. [#47643](https://github.com/gravitational/teleport/pull/47643)
* Reworked the `teleport-event-handler` integration to significantly improve performance, especially when running with larger `--concurrency` values. [#47633](https://github.com/gravitational/teleport/pull/47633)
* Fixes a bug where Let's Encrypt certificate renewal failed in AMI and HA deployments due to insufficient disk space caused by syncing audit logs. [#47622](https://github.com/gravitational/teleport/pull/47622)
* Adds support for custom SQS consumer lock name and disabling a consumer. [#47614](https://github.com/gravitational/teleport/pull/47614)
* Fixed an issue that prevented RDS Aurora discovery configuration in the AWS OIDC enrollment wizard when any cluster existed without member instances. [#47605](https://github.com/gravitational/teleport/pull/47605)
* Extend the Datadog plugin to support automatic approvals. [#47602](https://github.com/gravitational/teleport/pull/47602)
* Allow using a custom database for Firestore backends. [#47583](https://github.com/gravitational/teleport/pull/47583)
* Include host name instead of host uuid in error messages when SSH connections are prevented due to an invalid login. [#47578](https://github.com/gravitational/teleport/pull/47578)
* Fix the example Terraform code to support the new larger Teleport Enterprise licenses and updates output of web address to use fqdn when ACM is disabled. [#47512](https://github.com/gravitational/teleport/pull/47512)
* Add new `tctl` subcommands to manage bot instances. [#47225](https://github.com/gravitational/teleport/pull/47225)

Enterprise:
* Device auto-enroll failures are now recorded in the audit log.
* Fixed possible panic when processing Okta assignments.

## 16.4.3 (10/16/2024)

* Extended Teleport Discovery Service to support resource discovery across all projects accessible by the service account. [#47568](https://github.com/gravitational/teleport/pull/47568)
* Fixed a bug that could allow users to list active sessions even when prohibited by RBAC. [#47564](https://github.com/gravitational/teleport/pull/47564)
* The `tctl tokens ls` command redacts secret join tokens by default. To include the token values, provide the new `--with-secrets flag`. [#47545](https://github.com/gravitational/teleport/pull/47545)
* Added missing field-level documentation to the terraform provider reference. [#47469](https://github.com/gravitational/teleport/pull/47469)
* Fixed a bug where `tsh logout` failed to parse flags passed with spaces. [#47460](https://github.com/gravitational/teleport/pull/47460)
* Fixed the resource-based labels handler crashing without restarting. [#47452](https://github.com/gravitational/teleport/pull/47452)
* Install teleport FIPS binary in FIPS environments during Server Auto Discover. [#47437](https://github.com/gravitational/teleport/pull/47437)
* Fix possibly missing rules when using large amount of Access Monitoring Rules. [#47430](https://github.com/gravitational/teleport/pull/47430)
* Added ability to list/get AccessMonitoringRule resources with `tctl`. [#47401](https://github.com/gravitational/teleport/pull/47401)
* Include JWK header in JWTs issued by Teleport Application Access. [#47393](https://github.com/gravitational/teleport/pull/47393)
* Teleport Workload ID now supports issuing JWT SVIDs via the Workload API. [#47389](https://github.com/gravitational/teleport/pull/47389)
* Added kubeconfig context name to the output table of `tsh proxy kube` command for enhanced clarity. [#47383](https://github.com/gravitational/teleport/pull/47383)
* Improve error messaging when connections to offline agents are attempted. [#47361](https://github.com/gravitational/teleport/pull/47361)
* Allow specifying the instance type of AWS HA Terraform bastion instance. [#47338](https://github.com/gravitational/teleport/pull/47338)
* Added a config option to Teleport Connect to control how it interacts with the local SSH agent (`sshAgent.addKeysToAgent`). [#47324](https://github.com/gravitational/teleport/pull/47324)
* Teleport Workload ID issued JWT SVIDs are now compatible with OIDC federation with a number of platforms. [#47317](https://github.com/gravitational/teleport/pull/47317)
* The "ha-autoscale-cluster" terraform module now support default AWS resource tags and ASG instance refresh on configuration or launch template changes. [#47299](https://github.com/gravitational/teleport/pull/47299)
* Fixed error in Workload ID in cases where the process ID cannot be resolved. [#47274](https://github.com/gravitational/teleport/pull/47274)
* Teleport Connect for Linux now requires glibc 2.31 or later. [#47262](https://github.com/gravitational/teleport/pull/47262)
* Fixed a bug where security group rules that refer to another security group by ID were not displayed in web UI enrollment wizards when viewing security group rules. [#47246](https://github.com/gravitational/teleport/pull/47246)
* Improve the msteams access plugin debug logging. [#47158](https://github.com/gravitational/teleport/pull/47158)
* Fix missing tsh MFA prompt in certain OTP+WebAuthn scenarios. [#47154](https://github.com/gravitational/teleport/pull/47154)
* Updates self-hosted db discover flow to generate 2190h TTL certs, not 12h. [#47125](https://github.com/gravitational/teleport/pull/47125)
* Fixes an issue preventing Access Requests from displaying user friendly resource names. [#47112](https://github.com/gravitational/teleport/pull/47112)
* Fixed a bug where only one IP CIDR block security group rule for a port range was displayed in the web UI RDS enrollment wizard when viewing a security group. [#47077](https://github.com/gravitational/teleport/pull/47077)
* The `tsh play` command now supports a text output format. [#47073](https://github.com/gravitational/teleport/pull/47073)
* Updated Go to 1.22.8. [#47050](https://github.com/gravitational/teleport/pull/47050)
* Fixed the "source path is empty" error when attempting to upload a file in Teleport Connect. [#47011](https://github.com/gravitational/teleport/pull/47011)
* Added static host users to Terraform provider. [#46974](https://github.com/gravitational/teleport/pull/46974)
* Enforce a global `device_trust.mode=required` on OSS processes paired with an Enterprise Auth. [#46947](https://github.com/gravitational/teleport/pull/46947)
* Added a new config option in Teleport Connect to control SSH agent forwarding (`ssh.forwardAgent`); starting in Teleport Connect v17, this option will be disabled by default. [#46895](https://github.com/gravitational/teleport/pull/46895)
* Correctly display available allowed logins of leaf AWS Console Apps on `tsh app login`. [#46806](https://github.com/gravitational/teleport/pull/46806)
* Allow all audit events to be trimmed if necessary. [#46499](https://github.com/gravitational/teleport/pull/46499)

Enterprise:
* Fixed possible panic when processing Okta assignments.
* Fixed bug where an unknown device aborts device web authentication.
* Add the Datadog Incident Management Plugin as a hosted plugin.
* Permit bootstrapping enterprise clusters with state from an open source cluster.

## 16.4.2 (09/25/2024)

* Fixed a panic when using the self-hosted PagerDuty plugin. [#46925](https://github.com/gravitational/teleport/pull/46925)
* A user joining a session will now see available controls for terminating & leaving the session. [#46901](https://github.com/gravitational/teleport/pull/46901)
* Fixed a regression in the SAML IdP service which prevented cache from initializing in a cluster that may have a service provider configured with unsupported `acs_url` and `relay_state` values. [#46845](https://github.com/gravitational/teleport/pull/46845)

Enterprise:
* Fixed a possible crash when using Teleport Policy's GitLab integration.

## 16.4.1 (09/25/2024)

### Secrets support for Kubernetes Operator

Kubernetes Operator is now able to lookup values from Kubernetes secrets for `GithubConnector.ClientSecret` and `OIDCConnector.ClientSecret`.

### Other improvements and fixes

* Fixed a regression that made it impossible to read the Teleport Audit Log after creating a plugin if the audit event is present. [#46831](https://github.com/gravitational/teleport/pull/46831)
* Added a new flag to static host users spec that allows teleport to automatically take ownership across matching hosts of any users with the same name as the static host user. [#46828](https://github.com/gravitational/teleport/pull/46828)
* Added support for Kubernetes SPDY over Websocket Protocols for PortForward. [#46815](https://github.com/gravitational/teleport/pull/46815)
* Fixed a regression where Teleport swallowed Kubernetes API errors when using kubectl exec with a Kubernetes cluster newer than v1.30.0. [#46811](https://github.com/gravitational/teleport/pull/46811)
* Added support for Access Request Datadog plugin. [#46740](https://github.com/gravitational/teleport/pull/46740)

## 16.4.0 (09/18/2024)

### Machine ID for HCP Terraform and Terraform Enterprise

Teleport now supports secure joining via Terraform Cloud, allowing Machine ID
workflows to run on Terraform Cloud without shared secrets.

### SPIFFE Federation for Workload Identity

Teleport Workload Identity now supports SPIFFE Federation, allowing trust
relationships to be established between a Teleport cluster's trust domain and
trust domains managed by other SPIFFE compatible platforms. Establishing a
relationship between the trust domains enables workloads belonging to one trust
domain to validate the identity of workloads in the other trust domain, and vice
versa.

### Multi-domain support for web applications

Teleport now supports web application access where one application depends on
another. For example, you may have a web application that depends on a backend
API service, both of which are separate apps protected by Teleport.

### Okta integration status dashboard

Cluster admins are now able to get a detailed overview of the Okta integration
status in the Teleport web UI.

### Other improvements and fixes

* Fixed the web favicon not displaying on specific builds. [#46736](https://github.com/gravitational/teleport/pull/46736)
* Fixed regression in private key parser to handle mismatched PEM headers. [#46727](https://github.com/gravitational/teleport/pull/46727)
* Removed TXT record validation from custom DNS zones in VNet; VNet now supports any custom DNS zone, as long as it's included in `vnet_config`. [#46722](https://github.com/gravitational/teleport/pull/46722)
* Fixed audit log not recognizing static host user events. [#46697](https://github.com/gravitational/teleport/pull/46697)
* Fixes a bug in Kubernetes access that causes the error `expected *metav1.PartialObjectMetadata object` when trying to list resources. [#46694](https://github.com/gravitational/teleport/pull/46694)
* Added a new `default_shell` configuration for the static host users resource that works exactly the same as the `create_host_user_default_shell` configuration added for roles. [#46688](https://github.com/gravitational/teleport/pull/46688)
* Machine ID now generates cluster-specific `ssh_config` and `known_hosts` files which will always direct SSH connections made using them via Teleport. [#46684](https://github.com/gravitational/teleport/pull/46684)
* Fixed a regression that prevented the `fish` shell from starting in Teleport Connect. [#46662](https://github.com/gravitational/teleport/pull/46662)
* Added a new `create_host_user_default_shell` configuration under role options that changes the default shell of auto provisioned host users. [#46648](https://github.com/gravitational/teleport/pull/46648)
* Fixed an issue that prevented host user creation when the username was also listed in `host_groups`. [#46635](https://github.com/gravitational/teleport/pull/46635)
* Fixed `tsh scp` showing a login prompt when attempting to transfer a folder without the recursive option. [#46603](https://github.com/gravitational/teleport/pull/46603)
* The Teleport Terraform provider now supports AccessMonitoringRule resources. [#46582](https://github.com/gravitational/teleport/pull/46582)
* The `teleport-plugin-slack` chart can now deploy `tbot` to obtain and renew the Slack plugin credentials automatically. This setup is easier and more secure than signing long-lived credentials. [#46581](https://github.com/gravitational/teleport/pull/46581)
* Always show the device trust green shield for authenticated devices. [#46565](https://github.com/gravitational/teleport/pull/46565)
* Add new `terraform_cloud` joining method to enable secretless authentication on HCP Terraform jobs for the Teleport Terraform provider. [#46049](https://github.com/gravitational/teleport/pull/46049)
* Emit audit logs when creating, updating or deleting Teleport Plugins. [#4939](https://github.com/gravitational/teleport.e/pull/4939)

## 16.3.0 (09/11/2024)

### Out-of-band user creation

Cluster administrators are now able to configure Teleport's `ssh_service` to
ensure that certain host users exist on the machine without the need to start
an SSH session. [#46498](https://github.com/gravitational/teleport/pull/46498)

### Other improvements and fixes

* Allow the cluster wide ssh dial timeout to be set via `auth_service.ssh_dial_timeout` in the Teleport config file. [#46507](https://github.com/gravitational/teleport/pull/46507)
* Fixed an issue preventing session joining while host user creation was in use. [#46501](https://github.com/gravitational/teleport/pull/46501)
* Added tbot Helm chart for deploying a Machine ID Bot into a Teleport cluster. [#46373](https://github.com/gravitational/teleport/pull/46373)

## 16.2.2 (09/10/24)

* Fixed an issue that prevented the Firestore backend from reading existing data. [#46433](https://github.com/gravitational/teleport/issues/46433)
* The `teleport-kube-agent` chart now correctly propagates configured annotations when deploying a StatefulSet. [#46421](https://github.com/gravitational/teleport/issues/46421)
* Fixed regression with Slack notification rules matching on plugin name instead of type. [#46391](https://github.com/gravitational/teleport/issues/46391)
* Update `tsh puttyconfig` to respect any defined proxy templates. [#46384](https://github.com/gravitational/teleport/issues/46384)
* Ensure that additional pod labels are carried over to post-upgrade and post-delete hook job pods when using the `teleport-kube-agent` Helm chart. [#46232](https://github.com/gravitational/teleport/issues/46232)
* Fix bug that renders WebUI unusable if a role is deleted while it is still being in use by the logged in user. [#45774](https://github.com/gravitational/teleport/issues/45774)

## 16.2.1 (09/05/24)

* Fixed debug service not being turned off by configuration; Connect My Computer in Teleport Connect should no longer fail with "bind: invalid argument". [#46293](https://github.com/gravitational/teleport/issues/46293)
* Fixed an issue that could result in duplicate session recordings being created. [#46265](https://github.com/gravitational/teleport/issues/46265)
* Connect now supports bulk selection of resources to create an Access Request in the unified resources view. [#46238](https://github.com/gravitational/teleport/issues/46238)
* Added support for the `teleport_installer` resource to the Teleport Terraform provider. [#46200](https://github.com/gravitational/teleport/issues/46200)
* Fixed an issue that would cause reissue of certificates to fail in some scenarios where a local auth service was present. [#46184](https://github.com/gravitational/teleport/issues/46184)
* Updated OpenSSL to 3.0.15. [#46180](https://github.com/gravitational/teleport/issues/46180)
* Extend Teleport ability to use non-default cluster domains in Kubernetes, avoiding the assumption of `cluster.local`. [#46150](https://github.com/gravitational/teleport/issues/46150)
* Fixed retention period handling in the CockroachDB audit log storage backend. [#46147](https://github.com/gravitational/teleport/issues/46147)
* Prevented Teleport Kubernetes access from resending resize events to the party that triggered the terminal resize, avoiding potential resize loops. [#46066](https://github.com/gravitational/teleport/issues/46066)
* Fixed an issue where attempts to play/export certain session recordings would fail with `gzip: invalid header`. [#46035](https://github.com/gravitational/teleport/issues/46035)
* Fixed a bug where Teleport services could not join the cluster using iam, azure, or tpm methods when the proxy service certificate did not contain IP SANs. [#46010](https://github.com/gravitational/teleport/issues/46010)
* Prevent connections from being randomly terminated by Teleport proxies when `proxy_protocol` is enabled and TLS is terminated before Teleport Proxy. [#45992](https://github.com/gravitational/teleport/issues/45992)
* Updated the icons for server, application, and desktop resources. [#45990](https://github.com/gravitational/teleport/issues/45990)
* Added `eks:UpdateAccessEntry` to IAM permissions generated by the teleport integration IAM setup command and to the documentation reference for auto-discovery IAM permissions. [#45983](https://github.com/gravitational/teleport/issues/45983)
* Added ServiceNow support to Access Request notification routing rules. [#45965](https://github.com/gravitational/teleport/issues/45965)
* Added PagerDuty support to Access Request notification routing rules. [#45913](https://github.com/gravitational/teleport/issues/45913)
* Fixed an issue where `host_sudoers` could be written to Teleport proxy server sudoer lists in Teleport v14 and v15. [#45958](https://github.com/gravitational/teleport/issues/45958)
* Prevent interactive sessions from hanging on exit. [#45952](https://github.com/gravitational/teleport/issues/45952)
* Fixed kernel version check of Enhanced Session Recording for distributions with backported BPF. [#45941](https://github.com/gravitational/teleport/issues/45941)
* Added a flag to skip a relogin attempt when using `tsh ssh` and `tsh proxy ssh`. [#45929](https://github.com/gravitational/teleport/issues/45929)
* The hostname where the process is running is returned when running `tctl get db_services`. [#45909](https://github.com/gravitational/teleport/issues/45909)
* Add buttons to clear all selected Roles/Reviewers in new Access Requests. [#45904](https://github.com/gravitational/teleport/issues/45904)
* Fixed an issue WebSocket upgrade fails with MiTM proxies that can remask payloads. [#45899](https://github.com/gravitational/teleport/issues/45899)
* When a database is created manually (without auto-discovery) the `teleport.dev/db-admin` and `teleport.dev/db-admin-default-database` labels are no longer ignored and can be used to configure database auto-user provisioning. [#45891](https://github.com/gravitational/teleport/issues/45891)
* Add support for non-RSA SSH signatures with imported CA keys. [#45890](https://github.com/gravitational/teleport/issues/45890)
* Update `tsh login` and `tsh status` output to truncate a list of roles. [#45581](https://github.com/gravitational/teleport/issues/45581)

## 16.2.0 (08/26/24)

### NLA Support for Windows desktops

Teleport now supports Network Level Authentication (NLA) when connecting to
Windows hosts that are part of an Active Directory domain. NLA support is
currently opt-in. It will be enabled by default in a future release.

To enable NLA, set the `TELEPORT_ENABLE_RDP_NLA` environment variable to `yes`
on your `windows_desktop_service` instances. It is not necessary to configure
the Windows hosts to require NLA - Teleport's client will perform NLA when
configured to do so, even if the server does not require it.

More information is available in the
[Active Directory docs](./docs/pages/enroll-resources/desktop-access/active-directory.mdx#network-level-authentication-nla)

### DocumentDB IAM authentication support

Teleport now supports authenticating to DocumentDB with IAM users and roles
[recently released](https://aws.amazon.com/about-aws/whats-new/2024/06/amazon-documentdb-iam-database-authentication/)
by AWS.

### Join Tokens in the Web UI

Teleport now allows users to manage join tokens in the web UI as an alternative
to the tctl tokens commands.

### Database Access Controls in Access Graph

Database Access users are now able to see database objects and their access
paths in Access Graph.

### Logrotate support

Teleport now integrates with logrotate by automatically reopening log files when
detecting that they were renamed.

### Other improvements and fixes

* Failure to share a local directory in a Windows desktop session is no longer considered a fatal error. [#45852](https://github.com/gravitational/teleport/pull/45852)
* Add `teleport.dev/project-id` label for auto-enrolled instances in GCP. [#45820](https://github.com/gravitational/teleport/pull/45820)
* Fix an issue that prevented the creation of AWS App Access for an Integration that used digits only (eg, AWS Account ID). [#45819](https://github.com/gravitational/teleport/pull/45819)
* Slack plugin now lists logins permitted by requested roles. [#45759](https://github.com/gravitational/teleport/pull/45759)
* For new EKS Cluster auto-enroll configurations, the temporary Access Entry is tagged with `teleport.dev/` namespaced tags. For existing set ups, please add the `eks:TagResource` action to the Integration IAM Role to get the same behavior. [#45725](https://github.com/gravitational/teleport/pull/45725)
* Added support for importing S3 Bucket Tags into Teleport Policy's Access Graph. For existing configurations, ensure that the `s3:GetBucketTagging` permission is manually included in the Teleport Access Graph integration role. [#45551](https://github.com/gravitational/teleport/pull/45551)
* Add a `tctl terraform env` command to simplify running the Teleport Terraform provider locally. [#44690](https://github.com/gravitational/teleport/pull/44690)
* Add native MachineID support to the Terraform provider. Environments with delegated joining methods such as GitHub Actions, GitLab CI, CircleCI, GCP, or AWS can run the Terraform provider without having to setup `tbot`. [#44690](https://github.com/gravitational/teleport/pull/44690)
* The Terraform Provider now sequentially tries every credential source and provide more actionable error messages if it cannot connect. [#44690](https://github.com/gravitational/teleport/pull/44690)
* When the Terraform provider finds expired credentials it will now fail fast with a clear error instead of hanging for 30 seconds and sending potentially misleading error about certificates being untrusted. [#44690](https://github.com/gravitational/teleport/pull/44690)
* Fix a bug that caused some enterprise clusters to incorrectly display a message that the cluster had a monthly allocation of 0 Access Requests. [#4923](https://github.com/gravitational/teleport.e/pull/4923)

## 16.1.8 (08/23/24)

### Security fix

#### [High] Stored XSS in SAML IdP

When registering a service provider with SAML IdP, Teleport did not sufficiently
validate the ACS endpoint. This could allow a Teleport administrator with
permissions to write saml_idp_service_provider resources to configure a
malicious service provider with an XSS payload and compromise session of users
who would access that service provider.

Note: This vulnerability is only applicable when Teleport itself is acting as
the identity provider. If you only use SAML to connect to an upstream identity
provider you are not impacted. You can use the tctl get
saml_idp_service_provider command to verify if you have any Service Provider
applications registered and Teleport acts as an IdP.

For self-hosted Teleport customers that use Teleport as SAML Identity Provider,
we recommend upgrading auth and proxy servers. Teleport agents (SSH, Kubernetes,
desktop, application, database and discovery) are not impacted and do not need
to be updated.

### Other fixes and improvements

* Fixed an issue where Teleport could modify group assignments for users not managed by Teleport. This will require a migration of host users created with create_host_user_mode: keep in order to maintain Teleport management. [#45791](https://github.com/gravitational/teleport/pull/45791)
* The terminal shell can now be changed in Teleport Connect by right-clicking on a terminal tab. This allows using WSL (`wsl.exe`) if it is installed. Also, the default shell on Windows has been changed to `pwsh.exe` (instead of `powershell.exe`). [#45734](https://github.com/gravitational/teleport/pull/45734)
* Improve web UI enroll RDS flow where VPC, subnets, and security groups are now selectable. [#45688](https://github.com/gravitational/teleport/pull/45688)
* Allow to limit duration of local tsh proxy certificates with a new MFAVerificationInterval option. [#45686](https://github.com/gravitational/teleport/pull/45686)
* Fixed host user creation for tsh scp. [#45680](https://github.com/gravitational/teleport/pull/45680)
* Fixed an issue AWS access fails when the username is longer than 64 characters. [#45658](https://github.com/gravitational/teleport/pull/45658)
* Permit setting a cluster wide SSH connection dial timeout. [#45650](https://github.com/gravitational/teleport/pull/45650)
* Improve performance of host resolution performed via tsh ssh when connecting via labels or proxy templates. [#45644](https://github.com/gravitational/teleport/pull/45644)
* Remove empty tcp app session recordings. [#45643](https://github.com/gravitational/teleport/pull/45643)
* Fixed bug causing FeatureHiding flag to not hide the "Access Management" section in the UI as intended. [#45608](https://github.com/gravitational/teleport/pull/45608)
* Fixed an issue where users created in `keep` mode could effectively become `insecure_drop` and get cleaned up as a result. [#45594](https://github.com/gravitational/teleport/pull/45594)
* Prevent RBAC bypass for new Postgres connections. [#45554](https://github.com/gravitational/teleport/pull/45554)
* tctl allows cluster administrators to create custom notifications targeting Teleport users. [#45503](https://github.com/gravitational/teleport/pull/45503)
* Fixed debug service not enabled by default when not using a configuration file. [#45480](https://github.com/gravitational/teleport/pull/45480)
* Introduce support for Envoy SDS into the Machine ID spiffe-workload-api service. [#45460](https://github.com/gravitational/teleport/pull/45460)
* Improve the output of `tsh sessions ls`. [#45452](https://github.com/gravitational/teleport/pull/45452)
* Fix access entry handling permission error when EKS auto-discovery was set up in the Discover UI. [#45442](https://github.com/gravitational/teleport/pull/45442)
* Fix showing error message when enrolling EKS clusters in the Discover UI. [#45415](https://github.com/gravitational/teleport/pull/45415)
* Fixed the "Create A Bot" flow for GitHub Actions and SSH. It now correctly grants the bot the role created during the flow, and the example YAML is now correctly formatted. [#45409](https://github.com/gravitational/teleport/pull/45409)
* Mark authenticators used for passwordless as a passkey, if not previously marked as such. [#45395](https://github.com/gravitational/teleport/pull/45395)
* Prevents a panic caused by AWS STS client not being initialized when assuming an AWS Role. [#45382](https://github.com/gravitational/teleport/pull/45382)
* Update teleport debug commands to handle data dir not set. [#45341](https://github.com/gravitational/teleport/pull/45341)
* Fix `tctl get all` not returning SAML or OIDC auth connectors. [#45319](https://github.com/gravitational/teleport/pull/45319)
* The Opsgenie plugin recipients can now be dynamically configured by creating Access Monitoring Rules resources with the required Opsgenie notify schedules. [#45307](https://github.com/gravitational/teleport/pull/45307)
* Improve discoverability of the source or rejected connections due to unsupported versions. [#45278](https://github.com/gravitational/teleport/pull/45278)
* Improved copy and paste behavior in the terminal in Teleport Connect. On Windows and Linux, Ctrl+Shift+C/V now copies and pastes text (these shortcuts can be changed with `keymap.terminalCopy`/`keymap.terminalPaste`).  A mouse right click (`terminal.rightClick`) can copy/paste text too (enabled by default on Windows). [#45265](https://github.com/gravitational/teleport/pull/45265)
* Fixed an issue that could cause auth servers to panic when their backend connectivity was interrupted. [#45225](https://github.com/gravitational/teleport/pull/45225)
* Adds SPIFFE compatible federation bundle endpoint to the Proxy API, allowing other workload identity platforms to federate with the Teleport cluster. [#44998](https://github.com/gravitational/teleport/pull/44998)
* Add 'Download CSV' button to Access Monitoring Query results. [#4899](https://github.com/gravitational/teleport.e/pull/4899)
* Fixed issue in Okta Sync that spuriously deletes Okta Applications due to connectivity errors. [#4885](https://github.com/gravitational/teleport.e/pull/4885)
* Fixed bug in Okta Sync that mistakenly removes Apps and Groups on connectivity failure. [#4883](https://github.com/gravitational/teleport.e/pull/4883)
* Fixed bug that caused some enterprise clusters to incorrectly display a message that the cluster had a monthly allocation of 0 Access Requests. [#4923](https://github.com/gravitational/teleport.e/pull/4923)

## 16.1.4 (08/07/24)

* Improved `tsh ssh` performance for concurrent execs. [#45162](https://github.com/gravitational/teleport/pull/45162)
* Fixed issue with loading cluster features when agents are upgraded prior to auth. [#45226](https://github.com/gravitational/teleport/pull/45226)
* Updated Go to `1.22.6`. [#45194](https://github.com/gravitational/teleport/pull/45194)

## 16.1.3 (08/06/24)

* Fixed an issue where `tsh aws` may display extra text in addition to the original command output. [#45168](https://github.com/gravitational/teleport/pull/45168)
* Fixed regression that denied access to launch some Apps. [#45149](https://github.com/gravitational/teleport/pull/45149)
* Bot resources now honor their `metadata.expires` field. [#45130](https://github.com/gravitational/teleport/pull/45130)
* Teleport Connect now sets `TERM_PROGRAM: Teleport_Connect` and `TERM_PROGRAM_VERSION: <app_version>` environment variables in the integrated terminal. [#45063](https://github.com/gravitational/teleport/pull/45063)
* Fixed a panic in the Microsoft Teams plugin when it receives an error. [#45011](https://github.com/gravitational/teleport/pull/45011)
* Added a background item for VNet in Teleport Connect; VNet now prompts for a password only during the first launch. [#44994](https://github.com/gravitational/teleport/pull/44994)
* Added warning on `tbot` startup when the requested certificate TTL exceeds the maximum allowed value. [#44989](https://github.com/gravitational/teleport/pull/44989)
* Fixed a race condition between session recording uploads and session recording upload cleanup. [#44978](https://github.com/gravitational/teleport/pull/44978)
* Prevented Kubernetes per-Resource RBAC from blocking access to namespaces when denying access to a single resource kind in every namespace. [#44974](https://github.com/gravitational/teleport/pull/44974)
* SSO login flows can now authorize web sessions with Device Trust. [#44906](https://github.com/gravitational/teleport/pull/44906)
* Added support for Kubernetes Workload Attestation into Teleport Workload Identity to allow the authentication of pods running within Kubernetes without secrets. [#44883](https://github.com/gravitational/teleport/pull/44883)

Enterprise:
* Fixed a redirection issue with the SAML IdP authentication middleware which prevented users from signing into the service provider when an SAML authentication request was made with an HTTP-POST binding protocol, and user's didn't already have an active session with Teleport. [#4806](https://github.com/gravitational/teleport.e/pull/4806)
* SAML applications can now be deleted from the Web UI. [#4778](https://github.com/gravitational/teleport.e/pull/4778)
* Fixed an issue introduced in v16.0.3 and v15.4.6 where `tbot` FIPS builds fail to start due to a missing boringcrypto dependency. [#4757](https://github.com/gravitational/teleport.e/pull/4757)

## 16.1.1 (07/31/24)

* Added option to allow client redirects from IPs in specified CIDR ranges in SSO client logins. [#44846](https://github.com/gravitational/teleport/pull/44846)
* Machine ID can now be configured to use Kubernetes Secret destinations from the command line using the `kubernetes-secret` schema. [#44801](https://github.com/gravitational/teleport/pull/44801)
* Prevent the Discovery Service from overwriting Teleport dynamic resources that have the same name as discovered resources. [#44785](https://github.com/gravitational/teleport/pull/44785)
* Reduced the probability that the event-handler deadlocks when encountering errors processing session recordings. [#44771](https://github.com/gravitational/teleport/pull/44771)
* Improved event-handler diagnostics by providing a way to capture profiles dynamically via `SIGUSR1`. [#44758](https://github.com/gravitational/teleport/pull/44758)
* Teleport Connect now uses ConPTY for better terminal resizing and accurate color rendering on Windows, with an option to disable it in the app config. [#44742](https://github.com/gravitational/teleport/pull/44742)
* Fixed event-handler Helm charts using the wrong command when starting the event-handler container. [#44697](https://github.com/gravitational/teleport/pull/44697)
* Improved stability of very large Teleport clusters during temporary backend disruption/degradation. [#44694](https://github.com/gravitational/teleport/pull/44694)
* Resolved compatibility issue with Paramiko and Machine ID's SSH multiplexer SSH agent. [#44673](https://github.com/gravitational/teleport/pull/44673)
* Teleport no longer creates invalid SAML Connectors when calling `tctl get saml/<connector-name> | tctl create -f` without the `--with-secrets` flag. [#44666](https://github.com/gravitational/teleport/pull/44666)
* Fixed a fatal error in `tbot` when unable to lookup the user from a given UID in containerized environments for checking ACL configuration. [#44645](https://github.com/gravitational/teleport/pull/44645)
* Fixed application access regression where an HTTP header wasn't set in forwarded requests. [#44628](https://github.com/gravitational/teleport/pull/44628)
* Added Server auto-discovery support for Rocky and AlmaLinux distros. [#44612](https://github.com/gravitational/teleport/pull/44612)
* Use the registered port of the target host when `tsh puttyconfig` is invoked without `--port`. [#44572](https://github.com/gravitational/teleport/pull/44572)
* Added more icons for guessing application icon by name or by label `teleport.icon` in the web UI. [#44566](https://github.com/gravitational/teleport/pull/44566)
* Remove deprecated S3 bucket option when creating or editing AWS OIDC integration in the web UI. [#44485](https://github.com/gravitational/teleport/pull/44485)
* Fixed terminal sessions with a database CLI client in Teleport Connect hanging indefinitely if the client cannot be found. [#44465](https://github.com/gravitational/teleport/pull/44465)
* Added `application-tunnel` service to Machine ID for establishing a long-lived tunnel to a HTTP or TCP application for Machine to Machine access. [#44443](https://github.com/gravitational/teleport/pull/44443)
* Fixed a regression that caused Teleport Connect to fail to start on Intel Macs. [#44435](https://github.com/gravitational/teleport/pull/44435)
* Improved auto-discovery resiliency by recreating Teleport configuration when the node fails to join the cluster. [#44432](https://github.com/gravitational/teleport/pull/44432)
* Fixed a low-probability panic in audit event upload logic. [#44425](https://github.com/gravitational/teleport/pull/44425)
* Fixed Teleport Connect binaries not being signed correctly. [#44419](https://github.com/gravitational/teleport/pull/44419)
* Prevented DoSing the cluster during a mass failed join event by agents. [#44414](https://github.com/gravitational/teleport/pull/44414)
* The availability filter is now a toggle to show (or hide) requestable resources. [#44413](https://github.com/gravitational/teleport/pull/44413)
* Moved PostgreSQL auto provisioning users procedures to `pg_temp` schema. [#44409](https://github.com/gravitational/teleport/pull/44409)
* Added audit events for AWS and Azure integration resource actions. [#44403](https://github.com/gravitational/teleport/pull/44403)
* Fixed automatic updates with previous versions of the `teleport.yaml` config. [#44379](https://github.com/gravitational/teleport/pull/44379)
* Added support for Rocky and AlmaLinux when enrolling a new server from the UI. [#44332](https://github.com/gravitational/teleport/pull/44332)
* Fixed PostgreSQL session playback not rendering queries line breaks correctly. [#44315](https://github.com/gravitational/teleport/pull/44315)
* Fixed Teleport access plugin tarballs containing a `build` directory, which was accidentally added upon v16.0.0 release. [#44300](https://github.com/gravitational/teleport/pull/44300)
* Prevented an infinite loop in DynamoDB event querying by advancing the cursor to the next day when the limit is reached at the end of a day with an empty iterator. This ensures the cursor does not reset to the beginning of the day. [#44275](https://github.com/gravitational/teleport/pull/44275)
* The clipboard sharing tooltip for desktop sessions now indicates why clipboard sharing is disabled. [#44237](https://github.com/gravitational/teleport/pull/44237)
* Prevented redirects to arbitrary URLs when launching an app. [#44188](https://github.com/gravitational/teleport/pull/44188)
* Added a `--skip-idle-time` flag to `tsh play`. [#44013](https://github.com/gravitational/teleport/pull/44013)
* Added audit events for discovery config actions. [#43793](https://github.com/gravitational/teleport/pull/43793)
* Enabled Access Monitoring Rules routing with Mattermost plugin. [#43601](https://github.com/gravitational/teleport/pull/43601)
* SAML application can now be deleted from the Web UI. [#4778](https://github.com/gravitational/teleport.e/pull/4778)
* Fixed an Access List permission bug where an Access List owner, who is also a member, was not able to add/remove Access List member. [#4744](https://github.com/gravitational/teleport.e/pull/4744)
* Fixed a bug in Web UI where clicking SAML GCP Workforce Identity Federation discover tile would throw an error, preventing from using the guided enrollment feature. [#4720](https://github.com/gravitational/teleport.e/pull/4720)
* Fixed an issue with incorrect yum/zypper updater packages being installed. [#4684](https://github.com/gravitational/teleport.e/pull/4684)

## 16.1.0 (07/15/24)

### New logo

We're excited to announce an update to the Teleport logo. This refresh aligns
with our evolving brand and will be reflected across the product, our marketing
site (goteleport.com), branded content, swag, and more.

The new logo will appear in the web UI starting with this release and on the
marketing website starting from July 17th, 2024.

### Database access session replay

Database access users will be able to watch PostgreSQL query replays in the web
UI or with tsh.

### Other improvements and fixes

* Fixed "staircase" text output for non-interactive Kube exec sessions in Web UI. [#44249](https://github.com/gravitational/teleport/pull/44249)
* Fixed a leak in the admin process spawned by starting VNet through `tsh vnet` or Teleport Connect. [#44225](https://github.com/gravitational/teleport/pull/44225)
* Fixed a `kube-agent-updater` bug affecting resolutions of private images. [#44191](https://github.com/gravitational/teleport/pull/44191)
* The `show_resources` option is no longer required for statically configured proxy ui settings. [#44181](https://github.com/gravitational/teleport/pull/44181)
* The `teleport-cluster` chart can now use existing ingresses instead of creating its own. [#44146](https://github.com/gravitational/teleport/pull/44146)
* Ensure that `tsh login` outputs accurate status information for the new session. [#44143](https://github.com/gravitational/teleport/pull/44143)
* Fixes "device trust mode _x_ requires Teleport Enterprise" errors on `tctl`. [#44133](https://github.com/gravitational/teleport/pull/44133)
* Added the `tbot install systemd` command for installing tbot as a service on Linux systems. [#44083](https://github.com/gravitational/teleport/pull/44083)
* Added ability to list Access List members in json format in `tctl`. [#44071](https://github.com/gravitational/teleport/pull/44071)
* Update grpc to `v1.64.1` (patches `GO-2024-2978`). [#44067](https://github.com/gravitational/teleport/pull/44067)
* Batch access review reminders into 1 message and provide link out to the web UI. [#44034](https://github.com/gravitational/teleport/pull/44034)
* Fixed denying access despite access being configured for Notification Routing Rules in the web UI. [#44029](https://github.com/gravitational/teleport/pull/44029)
* Honor proxy templates in tsh ssh. [#44026](https://github.com/gravitational/teleport/pull/44026)
* Fixed eBPF error occurring during startup on Linux RHEL 9. [#44023](https://github.com/gravitational/teleport/pull/44023)
* Fixed Redshift auto-user deactivation/deletion failure that occurs when a user is created or deleted and another user is deactivated concurrently. [#43968](https://github.com/gravitational/teleport/pull/43968)
* Lower latency of detecting Kubernetes cluster becoming online. [#43967](https://github.com/gravitational/teleport/pull/43967)
* Teleport AMIs now optionally source environment variables from `/etc/default/teleport` as regular Teleport package installations do. [#43962](https://github.com/gravitational/teleport/pull/43962)
* Make `tbot` compilable on Windows. [#43959](https://github.com/gravitational/teleport/pull/43959)
* Add a new event to the database session recording with query/command result information. [#43955](https://github.com/gravitational/teleport/pull/43955)
* Enabled setting event types to forward, skip events, skip session types in event-handler helm chart. [#43938](https://github.com/gravitational/teleport/pull/43938)
* `extraLabels` configured in `teleport-kube-agent` chart values are now correctly propagated to post-delete hooks. A new `extraLabels.job` object has been added for labels which should only apply to the post-delete job. [#43932](https://github.com/gravitational/teleport/pull/43932)
* Add support for Teams to Opsgenie plugin alert creation. [#43916](https://github.com/gravitational/teleport/pull/43916)
* Machine ID outputs now execute individually and concurrently, meaning that one failing output does not disrupt other outputs, and that performance when generating a large number of outputs is improved. [#43876](https://github.com/gravitational/teleport/pull/43876)
* SAML IdP service provider resource can now be updated from the Web UI. [#4651](https://github.com/gravitational/teleport.e/pull/4651)
* Fixed empty condition from unquoted string with YAML editor for Notification Routing Rules in the Web UI. [#4636](https://github.com/gravitational/teleport.e/pull/4636)
* Teleport Enterprise now supports the `TELEPORT_REPORTING_HTTP(S)_PROXY` environment variable to specify the URL of the HTTP(S) proxy used for connections to our usage reporting ingest service. [#4568](https://github.com/gravitational/teleport.e/pull/4568)
* Fixed inaccurately notifying user that Access List reviews are due in the web UI. [#4521](https://github.com/gravitational/teleport.e/pull/4521)

## 16.0.4 (07/03/24)

* Omit control plane services from the inventory list output for Cloud-Hosted instances. [#43779](https://github.com/gravitational/teleport/pull/43779)
* Updated Go toolchain to v1.22.5. [#43768](https://github.com/gravitational/teleport/pull/43768)
* Reduced CPU usage in auth servers experiencing very high concurrent request load. [#43755](https://github.com/gravitational/teleport/pull/43755)
* Machine ID defaults to disabling the use of the Kubernetes exec plugin when writing a Kubeconfig to a directory destination. This removes the need to manually configure `disable_exec_plugin`. [#43655](https://github.com/gravitational/teleport/pull/43655)
* Fixed startup crash of Teleport Connect on Ubuntu 24.04 by adding an AppArmor profile. [#43653](https://github.com/gravitational/teleport/pull/43653)
* Added support for dialling leaf clusters to the tbot SSH multiplexer. [#43634](https://github.com/gravitational/teleport/pull/43634)
* Extend Teleport ability to use non-default cluster domains in Kubernetes, avoiding the assumption of `cluster.local`. [#43631](https://github.com/gravitational/teleport/pull/43631)
* Wait for user MFA input when reissuing expired certificates for a kube proxy. [#43612](https://github.com/gravitational/teleport/pull/43612)
* Improved error diagnostics when using Machine ID's SSH multiplexer. [#43586](https://github.com/gravitational/teleport/pull/43586)

Enterprise:
* Teleport Enterprise now supports the `TELEPORT_REPORTING_HTTP(S)_PROXY` environment variable to specify the URL of the HTTP(S) proxy used for connections to our usage reporting ingest service.

## 16.0.3 (06/27/24)

This release of Teleport contains a fix for medium-level security issue impacting
Teleport Enterprise, as well as various other updates and improvements

### Security Fixes

* **[Medium]** Fixes issue where a SCIM client could potentially overwrite.
  Teleport system Roles using specially crafted groups. This issue impacts
  Teleport Enterprise deployments using the Okta integration with SCIM support
  enabled.

We strongly recommend all customers upgrade to the latest releases of Teleport.

### Other updates and improvements

* Update `go-retryablehttp` to v0.7.7 (fixes CVE-2024-6104). [#43474](https://github.com/gravitational/teleport/pull/43474)
* Fixed Discover setup access error when updating user. [#43560](https://github.com/gravitational/teleport/pull/43560)
* Added audit event field describing if the "MFA for admin actions" requirement changed. [#43541](https://github.com/gravitational/teleport/pull/43541)
* Fixed remote port forwarding validation error. [#43516](https://github.com/gravitational/teleport/pull/43516)
* Added support to trust system CAs for self-hosted databases. [#43493](https://github.com/gravitational/teleport/pull/43493)
* Added error display in the Web UI for SSH and Kubernetes sessions. [#43485](https://github.com/gravitational/teleport/pull/43485)
* Fixed accurate inventory reporting of the updater after it is removed. [#43454](https://github.com/gravitational/teleport/pull/43454)
* `tctl alerts ls` now displays remaining alert ttl. [#43436](https://github.com/gravitational/teleport/pull/43436)
* Fixed input search for Teleport Connect's Access Request listing. [#43429](https://github.com/gravitational/teleport/pull/43429)
* Added `Debug` setting for event-handler. [#43408](https://github.com/gravitational/teleport/pull/43408)
* Fixed Headless auth for sso users, including when local auth is disabled. [#43361](https://github.com/gravitational/teleport/pull/43361)
* Added configuration for custom CAs in the event-handler helm chart. [#43340](https://github.com/gravitational/teleport/pull/43340)
* Updated VNet panel in Teleport Connect to list custom DNS zones and DNS zones from leaf clusters. [#43312](https://github.com/gravitational/teleport/pull/43312)
* Fixed an issue with Database Access Controls preventing users from making additional database connections. [#43303](https://github.com/gravitational/teleport/pull/43303)
* Fixed bug that caused gRPC connections to be disconnected when their certificate expired even though DisconnectCertExpiry was false. [#43290](https://github.com/gravitational/teleport/pull/43290)
* Fixed Connect My Computer in Teleport Connect failing with "bind: invalid argument". [#43287](https://github.com/gravitational/teleport/pull/43287)
* Fix a bug where a Teleport instance running only Jamf or Discovery service would never have a healthy  `/readyz` endpoint. [#43283](https://github.com/gravitational/teleport/pull/43283)
* Added a missing `[Install]` section to the `teleport-acm` systemd unit file as used by Teleport AMIs. [#43257](https://github.com/gravitational/teleport/pull/43257)
* Patched timing variability in curve25519-dalek. [#43246](https://github.com/gravitational/teleport/pull/43246)
* Fixed setting request reason for automatic ssh Access Requests. [#43178](https://github.com/gravitational/teleport/pull/43178)
* Improved log rotation logic in Teleport Connect; now the non-numbered files always contain recent logs. [#43161](https://github.com/gravitational/teleport/pull/43161)
* Added `tctl desktop bootstrap` for bootstrapping AD environments to work with desktop access. [#43150](https://github.com/gravitational/teleport/pull/43150)

### Enterprise only changes and improvements

* The teleport updater will no longer default to using the global version channel, avoiding incompatible updates.
* Fixed sync error in Okta SCIM integration.

## 16.0.1 (06/17/24)

* `tctl` now ignores any configuration file if the auth_service section is disabled, and prefer loading credentials from a given identity file or tsh profile instead. [#43115](https://github.com/gravitational/teleport/pull/43115)
* Skip `jamf_service` validation when the service is not enabled. [#43095](https://github.com/gravitational/teleport/pull/43095)
* Fix v16.0.0 amd64 Teleport plugin images using arm64 binaries. [#43084](https://github.com/gravitational/teleport/pull/43084)
* Add ability to edit user traits from the Web UI. [#43067](https://github.com/gravitational/teleport/pull/43067)
* Enforce limits when reading events from Firestore for large time windows to prevent OOM events. [#42966](https://github.com/gravitational/teleport/pull/42966)
* Allow all authenticated users to read the cluster `vnet_config`. [#42957](https://github.com/gravitational/teleport/pull/42957)
* Improve search and predicate/label based dialing performance in large clusters under very high load. [#42943](https://github.com/gravitational/teleport/pull/42943)

## 16.0.0 (06/13/24)

Teleport 16 brings the following new features and improvements:

- Teleport VNet
- Device Trust for the Web UI
- Increased support for per-session MFA
- Web UI notification system
- Access requests from the resources view
- `tctl` for Windows
- Teleport plugins improvements

### Description

#### Teleport VNet

Teleport 16 introduces Teleport VNet, a new feature that provides a virtual IP
subnet and DNS server which automatically proxies TCP connections to Teleport
apps over mutually authenticated tunnels.

This allows scripts and software applications to connect to any
Teleport-protected application as if they were connected to a VPN, without the
need to manage local tunnels.

Teleport VNet is powered by the Teleport Connect client and is available for
macOS. Support for other operating systems will come in a future release.

#### Device Trust for the Web UI

Teleport Device Trust can now be enforced for browser-based workflows like
remote desktop and web application access. The Teleport Connect client must be
installed in order to satisfy device locality checks.

#### Increased support for per-session MFA

Teleport 16 now supports per-session MFA checks when accessing both web and TCP
applications via all supported clients (Web UI, `tsh`, and Teleport Connect).

Additionally, Teleport Connect now includes support for per-session MFA when
accessing database resources.

#### Web UI notification system

Teleport’s Web UI includes a new notifications system that notifies users of
items requiring attention (for example, Access Requests needing review).

#### Access requests from the resources view

The resources view in the web UI now shows both resources you currently have
access to and resources you can request access to. This allows users to request
access to resources without navigating to a separate page.

Cluster administrators who prefer the previous behavior of hiding requestable
resources from the main view can set `show_resources: accessible_only` in their
UI config:

For dynamic configuration, run `tctl edit ui_config`:

```yaml
kind: ui_config
version: v1
metadata:
  name: ui-config
spec:
  show_resources: accessible_only
```

Alternatively, self-hosted Teleport users can update the `ui` section of their
proxy configuration:

```yaml
proxy_service:
  enabled: yes
  ui:
    show_resources: accessible_only
```

#### `tctl` for Windows

Teleport 16 includes Windows builds of the `tctl` administrative tool, allowing
Windows users to administer their cluster without the need for a macOS or Linux
workstation.

Additionally, there are no longer enterprise-specific versions of `tctl`. All
Teleport clients (`tsh`, `tctl`, and Teleport Connect) are available in a single
distribution that works on both Enterprise and Community Edition clusters.

#### Teleport plugins improvements

Teleport 16 includes major improvements to the plugins. All plugins now have:

- amd64 and arm64 binaries available
- amd64 and arm64 multi-arch images
- Major and minor version rolling tags (ie
  `public.ecr.aws/gravitational/teleport-plugin-email:16`)
- Image signatures for all images
- Additional debug images with all of the above features

In addition, we now support plugins for each supported major version, starting
with v15. This means that if we fix a bug or security issue in a v16 plugin
version, we will also apply and release the change for the v15 plugin version.

#### Other

The Jamf plugin now authenticates with Jamf API credentials instead of username
and password.

### Breaking changes and deprecations

#### Community Edition license

Starting with this release, Teleport Community Edition restricts commercial
usage.

https://goteleport.com/blog/teleport-community-license/

#### License file validation on startup

Teleport 16 introduces license file validation on startup. This only applies to
customers running **Teleport Enterprise Self-Hosted**. No action is required for
customers running Teleport Enterprise (Cloud) or Teleport Community Edition.

If, after updating to Teleport 16, you receive an error message regarding an
outdated license file, follow our step-by-step [guide](docs/pages/admin-guides/deploy-a-cluster/license.mdx)
to update your license file.

#### Multi-factor authentication is now required for local users

Support for disabling multi-factor authentication has been removed. Teleport
will refuse to start until the `second_factor` setting is set to `on`, `webauthn`
or `otp`.

This change only affects _self-hosted_ Teleport users, as Teleport Enterprise (Cloud) has
always required multi-factor authentication.

**Important:** To avoid locking users out, we recommend the following steps:

1. Ensure that all cluster administrators have multi-factor devices registered
   in Teleport so that they will be able to reset any other users.
2. Announce to the user base that all users must register an MFA device.
   Consider creating a cluster alert with `tctl alerts create` to help spread
   the word.
3. While you are still on Teleport 15, set `second_factor: on`. This will help
   identify any users who have not registered MFA devices and allow you to
   revert to `second_factor: optional` if necessary.
4. Upgrade to Teleport 16.

Any users who do not register MFA devices prior to the Teleport 16 upgrade will
be unable to log in and must be reset by an administrator (`tctl users reset`).

#### Incompatible clients are rejected

In accordance with our [component compatibility](docs/pages/upgrading/overview.mdx#component-compatibility)
guidelines, Teleport 16 will start rejecting connections from clients and agents
running incompatible (ie too old) versions.

If Teleport detects connection attempts from outdated clients, it will show an
alert to cluster administrators in both the web UI and `tsh`.

To disable this behavior and run in an unsupported configuration that  allows
incompatible agents to connect to your cluster, start your Auth Service
instances with the `TELEPORT_UNSTABLE_ALLOW_OLD_CLIENTS=yes` environment
variable.

#### Opsgenie plugin annotations

Prior to Teleport 16, when using an Opsgenie plugin, the `teleport.dev/schedules`
role annotation was used to specify both schedules for Access Request
notifications as well as schedules to check for the request auto-approval.

Starting with Teleport 16, the annotations were split to provide behavior
consistent with other Access Request plugins: a role must now contain the
`teleport.dev/notify-services` to receive notifications on Opsgenie and the
`teleport.dev/schedules` to check for auto-approval.

Detailed setup instructions are available in the [documentation](https://github.com/gravitational/teleport/blob/branch/v16/docs/pages/access-controls/access-request-plugins/opsgenie.mdx).

#### Teleport Assist has been removed

Teleport Assist chat has been removed from Teleport 16. `auth_service.assist` and `proxy_service.assist`
options have been removed from the configuration. Teleport will not start if these options are present.

During the migration from v15 to v16, the options mentioned above should be removed from the configuration.

#### New required permissions for DynamoDB

Teleport clusters using the DynamoDB backend on AWS now require the
`dynamodb:ConditionCheckItem` permissions. For a full list of required
permissions, see the IAM policy [example](docs/pages/reference/backends.mdx#dynamodb).

#### Updated keyboard shortcuts in Teleport connect

On Windows and Linux, some of Teleport Connect’s keyboard shortcuts conflicted
with the default bash or nano shortcuts (Ctrl+E, Ctrl+K, etc). On those
platforms, the default shortcuts have been changed to a combination of
Ctrl+Shift+*.

On macOS, the default shortcut to open a new terminal has been changed to
Ctrl+Shift+`.

See the [configuration guide](https://github.com/gravitational/teleport/blob/branch/v16/docs/pages/connect-your-client/teleport-connect.mdx#configuration)
for a list of updated keyboard shortcuts.

#### Machine ID and OpenSSH client config changes

Users with custom `ssh_config` should modify their ProxyCommand to use the new,
more performant, `tbot ssh-proxy-command`. See the
[v16 upgrade guide](docs/pages/reference/machine-id/v16-upgrade-guide.mdx) for
more details.

#### Removal of Active Directory configuration flow

The Active Directory installation and configuration wizard has been removed.
Users who don’t already have Active Directory should leverage Teleport’s local
user support, and users with existing Active Directory environments should
follow the manual setup guide.

#### Teleport Assist is removed

All Teleport Assist functionality and OpenAI integration has been removed from
Teleport.

## 15.4.24 (12/11/2024)

* Updated golang.org/x/crypto to v0.31.0 (CVE-2024-45337). [#50080](https://github.com/gravitational/teleport/pull/50080)
* Fix tsh ssh -Y when jumping between multiple servers. [#50034](https://github.com/gravitational/teleport/pull/50034)
* Reduce Auth memory consumption when agents join using the azure join method. [#50000](https://github.com/gravitational/teleport/pull/50000)
* Tsh correctly respects the --no-allow-passwordless flag. [#49935](https://github.com/gravitational/teleport/pull/49935)
* Auto-updates for client tools (`tctl` and `tsh`) are controlled by cluster configuration. [#48648](https://github.com/gravitational/teleport/pull/48648)

## 15.4.23 (12/5/2024)

* Fixed a bug breaking in-cluster joining on some Kubernetes clusters. [#49843](https://github.com/gravitational/teleport/pull/49843)
* SSH or Kubernetes information is now included for audit log list for start session events. [#49834](https://github.com/gravitational/teleport/pull/49834)
* Avoid tight web session renewals for sessions with short TTL (between 3m and 30s). [#49770](https://github.com/gravitational/teleport/pull/49770)
* Updated Go to 1.22.10. [#49760](https://github.com/gravitational/teleport/pull/49760)
* Added ability to configure resource labels in `teleport-cluster`'s operator sub-chart. [#49649](https://github.com/gravitational/teleport/pull/49649)
* Fixed proxy peering listener not using the exact address specified in `peer_listen_addr`. [#49591](https://github.com/gravitational/teleport/pull/49591)
* Kubernetes in-cluster joining now also accepts tokens whose audience is the Teleport cluster name (before it only allowed the default Kubernetes audience). Kubernetes JWKS joining is unchanged and still requires tokens with the cluster name in the audience. [#49558](https://github.com/gravitational/teleport/pull/49558)
* Restore interactive PAM authentication functionality when `use_pam_auth` is applied. [#49520](https://github.com/gravitational/teleport/pull/49520)
* Increase CockroachDB setup timeout from 5 to 30 seconds. This mitigates the Auth Service not being able to configure TTL on slow CockroachDB event backends. [#49471](https://github.com/gravitational/teleport/pull/49471)
* Fixed a potential panic in login rule and SAML IdP expression parser. [#49432](https://github.com/gravitational/teleport/pull/49432)
* Support for long-running kube exec/port-forward, respect `client_idle_timeout` config. [#49430](https://github.com/gravitational/teleport/pull/49430)
* Fixed a permissions error with Postgres database user auto-provisioning that occurs when the database admin is not a superuser and the database is upgraded to Postgres v16 or higher. [#49391](https://github.com/gravitational/teleport/pull/49391)
* Fixed missing user participants in session recordings listing for non-interactive Kubernetes recordings. [#49345](https://github.com/gravitational/teleport/pull/49345)
* Fixed an issue where `teleport park` processes could be leaked causing runaway resource usage. [#49262](https://github.com/gravitational/teleport/pull/49262)
* The `tsh puttyconfig` command now disables GSSAPI auth settings to avoid a "Not Responding" condition in PuTTY. [#49191](https://github.com/gravitational/teleport/pull/49191)
* Allow Azure VMs to join from a different subscription than their managed identity. [#49158](https://github.com/gravitational/teleport/pull/49158)
* Fixed an issue loading the license file when Teleport is started without a configuration file. [#49148](https://github.com/gravitational/teleport/pull/49148)
* Fixed a bug in the `teleport-cluster` Helm chart that can cause token mount to fail when using ArgoCD. [#49070](https://github.com/gravitational/teleport/pull/49070)
* Fixed an issue resulting in excess cpu usage and connection resets when teleport-event-handler is under moderate to high load. [#49035](https://github.com/gravitational/teleport/pull/49035)
* Fixed OpenSSH remote port forwarding not working for localhost. [#49021](https://github.com/gravitational/teleport/pull/49021)
* Allow to override Teleport license secret name when using `teleport-cluster` Helm chart. [#48980](https://github.com/gravitational/teleport/pull/48980)
* Fixed users not being able to connect to SQL server instances with PKINIT integration when the cluster is configured with different CAs for database access. [#48925](https://github.com/gravitational/teleport/pull/48925)
* Ensure that agentless server information is provided in all audit events. [#48835](https://github.com/gravitational/teleport/pull/48835)
* Fixed an issue preventing migration of unmanaged users to Teleport host users when including `teleport-keep` in a role's `host_groups`. [#48456](https://github.com/gravitational/teleport/pull/48456)
* Resolved an issue that caused false positive errors incorrectly indicating that the YubiKey was in use by another application, while only tsh was accessing it. [#47953](https://github.com/gravitational/teleport/pull/47953)

Enterprise:
* Jamf Service sync audit events are attributed to "Jamf Service".

## 15.4.22 (11/12/24)

* Added a search input to the cluster dropdown in the Web UI when there's more than five clusters to show. [#48800](https://github.com/gravitational/teleport/pull/48800)
* Fixed bug in Kubernetes session recordings where both root and leaf cluster recorded the same Kubernetes session. Recordings of leaf resources are only available in leaf clusters. [#48739](https://github.com/gravitational/teleport/pull/48739)
* Machine ID can now be forced to use the explicitly configured proxy address using the `TBOT_USE_PROXY_ADDR` environment variable. This should better support split proxy address operation. [#48677](https://github.com/gravitational/teleport/pull/48677)
* Fixed undefined error in open source version when clicking on `Add Application` tile in the Enroll Resources page in the Web UI. [#48617](https://github.com/gravitational/teleport/pull/48617)
* Updated Go to 1.22.9. [#48582](https://github.com/gravitational/teleport/pull/48582)
* The teleport-cluster Helm chart now uses the configured `serviceAccount.name` from chart values for its pre-deploy configuration check Jobs. [#48578](https://github.com/gravitational/teleport/pull/48578)
* Fixed a bug that prevented the Teleport UI from properly displaying Plugin Audit log details. [#48463](https://github.com/gravitational/teleport/pull/48463)
* Fixed showing the list of Access Requests in Teleport Connect when a leaf cluster is selected in the cluster selector. [#48442](https://github.com/gravitational/teleport/pull/48442)
* Fixed a rare "internal error" on older U2F authenticators when using tsh. [#48403](https://github.com/gravitational/teleport/pull/48403)
* Fixed `tsh play` not skipping idle time when `--skip-idle-time` was provided. [#48398](https://github.com/gravitational/teleport/pull/48398)
* Added a warning to `tctl edit` about dynamic edits to statically configured resources. [#48393](https://github.com/gravitational/teleport/pull/48393)
* Fixed a Teleport Kubernetes Operator bug that happened for OIDCConnector resources with non-nil `max_age`. [#48377](https://github.com/gravitational/teleport/pull/48377)
* Updated host user creation to prevent local password expiration policies from affecting Teleport managed users. [#48162](https://github.com/gravitational/teleport/pull/48162)
* During the Set Up Access of the Enroll New Resource flows, Okta users will be asked to change the role instead of entering the principals and getting an error afterwards. [#47958](https://github.com/gravitational/teleport/pull/47958)
* Fixed `teleport_connected_resource` metric overshooting after keepalive errors. [#47950](https://github.com/gravitational/teleport/pull/47950)
* Fixed an issue preventing connections with users whose configured home directories were inaccessible. [#47917](https://github.com/gravitational/teleport/pull/47917)
* Added a `resolve` command to tsh that may be used as the target for a Match exec condition in an SSH config. [#47867](https://github.com/gravitational/teleport/pull/47867)
* Postgres database session start events now include the Postgres backend PID for the session. [#47644](https://github.com/gravitational/teleport/pull/47644)
* Updated `tsh ssh` to support the `--` delimiter similar to openssh. It is now possible to execute a command via `tsh ssh user@host -- echo test` or `tsh ssh -- host uptime`. [#47494](https://github.com/gravitational/teleport/pull/47494)

Enterprise:
* Jamf requests from Teleport set "teleport/$version" as the User-Agent.

## 15.4.21 (10/22/24)

### Security fixes

#### [High] Privilege persistence in Okta SCIM-only integration

When Okta SCIM-only integration is enabled, in certain cases Teleport could
calculate the effective set of permission based on SSO user's stale traits. This
could allow a user who was unassigned from an Okta group to log into a Teleport
cluster once with a role granted by the unassigned group being present in their
effective role set.

Note: This issue only affects Teleport clusters that have installed a SCIM-only
Okta integration as described in this guide. If you have an Okta integration
with user sync enabled or only using Okta SSO auth connector to log into your
Teleport cluster without SCIM integration configured, you're unaffected. To
verify your configuration:

- Use `tctl get plugins/okta --format=json | jq ".[].spec.Settings.okta.sync_settings.sync_users"`
  command to check if you have Okta integration with user sync enabled. If it
  outputs null or false, you may be affected and should upgrade.
- Check SCIM provisioning settings for the Okta application you created or
  updated while following the SCIM-only setup guide. If SCIM provisioning is
  enabled, you may be affected and should upgrade.

We strongly recommend customers who use Okta SCIM integration to upgrade their
auth servers to version 15.4.19 or later. Teleport services other than auth
(proxy, SSH, Kubernetes, desktop, application, database and discovery) are not
impacted and do not need to be updated.

### Other improvements and fixes

* Added a new teleport_roles_total metric that exposes the number of roles which exist in a cluster. [#47811](https://github.com/gravitational/teleport/pull/47811)
* The `join_token.create` audit event has been enriched with additional metadata. [#47766](https://github.com/gravitational/teleport/pull/47766)
* Automatic device enrollment may be locally disabled using the TELEPORT_DEVICE_AUTO_ENROLL_DISABLED=1 environment variable. [#47719](https://github.com/gravitational/teleport/pull/47719)
* Fixed the Machine ID and GitHub Actions wizard. [#47709](https://github.com/gravitational/teleport/pull/47709)
* Alter ServiceAccounts in the teleport-cluster Helm chart to automatically disable mounting of service account tokens on newer Kubernetes distributions, helping satisfy security linters. [#47702](https://github.com/gravitational/teleport/pull/47702)
* Avoid tsh auto-enroll escalation in machines without a TPM. [#47696](https://github.com/gravitational/teleport/pull/47696)
* Fixed a bug that prevented users from canceling `tsh scan keys` executions. [#47657](https://github.com/gravitational/teleport/pull/47657)
* Reworked the `teleport-event-handler` integration to significantly improve performance, especially when running with larger `--concurrency` values. [#47632](https://github.com/gravitational/teleport/pull/47632)
* Fixes a bug where Let's Encrypt certificate renewal failed in AMI and HA deployments due to insufficient disk space caused by syncing audit logs. [#47624](https://github.com/gravitational/teleport/pull/47624)
* Adds support for custom SQS consumer lock name and disabling a consumer. [#47613](https://github.com/gravitational/teleport/pull/47613)
* Allow using a custom database for Firestore backends. [#47584](https://github.com/gravitational/teleport/pull/47584)
* Include host name instead of host uuid in error messages when SSH connections are prevented due to an invalid login. [#47579](https://github.com/gravitational/teleport/pull/47579)
* Extended Teleport Discovery Service to support resource discovery across all projects accessible by the service account. [#47567](https://github.com/gravitational/teleport/pull/47567)
* Fixed a bug that could allow users to list active sessions even when prohibited by RBAC. [#47563](https://github.com/gravitational/teleport/pull/47563)
* The tctl tokens ls command redacts secret join tokens by default. To include the token values, provide the new --with-secrets flag. [#47546](https://github.com/gravitational/teleport/pull/47546)
* Fix the example Terraform code to support the new larger Teleport Enterprise licenses and updates output of web address to use fqdn when ACM is disabled. [#47511](https://github.com/gravitational/teleport/pull/47511)
* Added missing field-level documentation to the terraform provider reference. [#47470](https://github.com/gravitational/teleport/pull/47470)
* Fixed a bug where tsh logout failed to parse flags passed with spaces. [#47462](https://github.com/gravitational/teleport/pull/47462)
* Fixed the resource-based labels handler crashing without restarting. [#47453](https://github.com/gravitational/teleport/pull/47453)
* Fix possibly missing rules when using large amount of Access Monitoring Rules. [#47429](https://github.com/gravitational/teleport/pull/47429)

Enterprise:
* Device auto-enroll failures are now recorded in the audit log.
* Fixed possible panic when processing Okta assignments.

## 15.4.20 (10/10/24)

* Added ability to list/get access monitoring rules resources with `tctl`. [#47402](https://github.com/gravitational/teleport/pull/47402)
* Include JWK header in JWTs issued by Teleport Application Access. [#47394](https://github.com/gravitational/teleport/pull/47394)
* Added kubeconfig context name to the output table of `tsh proxy kube` command for enhanced clarity. [#47382](https://github.com/gravitational/teleport/pull/47382)
* Improve error messaging when connections to offline agents are attempted. [#47362](https://github.com/gravitational/teleport/pull/47362)
* Allow specifying the instance type of AWS HA Terraform bastion instance. [#47339](https://github.com/gravitational/teleport/pull/47339)
* Added a config option to Teleport Connect to control how it interacts with the local SSH agent (`sshAgent.addKeysToAgent`). [#47325](https://github.com/gravitational/teleport/pull/47325)
* Fixed error in Workload ID in cases where the process ID cannot be resolved. [#47275](https://github.com/gravitational/teleport/pull/47275)
* Teleport Connect for Linux now requires glibc 2.31 or later. [#47263](https://github.com/gravitational/teleport/pull/47263)
* Fix missing `tsh` MFA prompt in certain OTP+WebAuthn scenarios. [#47155](https://github.com/gravitational/teleport/pull/47155)
* Updates self-hosted db discover flow to generate 2190h TTL certs, not 12h. [#47127](https://github.com/gravitational/teleport/pull/47127)
* Fixes an issue preventing Access Requests from displaying user friendly resource names. [#47111](https://github.com/gravitational/teleport/pull/47111)
* Updated Go to `1.22.8`. [#47052](https://github.com/gravitational/teleport/pull/47052)
* Fixed the "source path is empty" error when attempting to upload a file in Teleport Connect. [#47013](https://github.com/gravitational/teleport/pull/47013)
* Enforce a global `device_trust.mode=required` on OSS processes paired with an Enterprise Auth. [#46946](https://github.com/gravitational/teleport/pull/46946)
* A user joining a session will now see available controls for terminating & leaving the session. [#46910](https://github.com/gravitational/teleport/pull/46910)
* Added a new config option in Teleport Connect to control SSH agent forwarding (`ssh.forwardAgent`); starting in Teleport Connect v17, this option will be disabled by default. [#46897](https://github.com/gravitational/teleport/pull/46897)
* Teleport no longer creates invalid SAML Connectors when calling `tctl get saml/<connector-name> | tctl create -f` without the `--with-secrets` flag. [#46864](https://github.com/gravitational/teleport/pull/46864)
* Fixed a regression in the SAML IdP service which prevented cache from initializing in a cluster that may have a service provider configured with unsupported `acs_url` and `relay_state` values. [#46846](https://github.com/gravitational/teleport/pull/46846)
* Machine ID now generates cluster-specific ssh_config and known_host files which will always direct SSH connections made using them via Teleport. [#46685](https://github.com/gravitational/teleport/pull/46685)
* Added new empty state to Devices list in web UI. [#5119](https://github.com/gravitational/teleport.e/pull/5119)
* Permit bootstrapping enterprise clusters with state from an open source cluster. [#5094](https://github.com/gravitational/teleport.e/pull/5094)
* Fixes a possible crash when using Teleport Policy's GitLab integration. [#5071](https://github.com/gravitational/teleport.e/pull/5071)
* Emit audit logs when creating, updating or deleting Teleport Plugins. [#5056](https://github.com/gravitational/teleport.e/pull/5056)

## 15.4.19 (09/17/24)

* Fixed a bug in Kubernetes access that causes the error `expected *metav1.PartialObjectMetadata object` when trying to list resources. [#46695](https://github.com/gravitational/teleport/pull/46695)
* Fixed an issue that prevented host user creation when the username was also listed in `host_groups`. [#46638](https://github.com/gravitational/teleport/pull/46638)
* Allow the cluster wide ssh dial timeout to be set via auth_service.ssh_dial_timeout in the Teleport config file. [#46508](https://github.com/gravitational/teleport/pull/46508)
* Allow all audit events to be trimmed if necessary. [#46504](https://github.com/gravitational/teleport/pull/46504)
* Fixed an issue preventing session joining while host user creation was in use. [#46502](https://github.com/gravitational/teleport/pull/46502)
* Fixed an issue that prevented the Firestore backend from reading existing data. [#46436](https://github.com/gravitational/teleport/pull/46436)
* The teleport-kube-agent chart now correctly propagates configured annotations when deploying a StatefulSet. [#46422](https://github.com/gravitational/teleport/pull/46422)
* Updated tsh puttyconfig to respect any defined proxy templates. [#46385](https://github.com/gravitational/teleport/pull/46385)
* Added tbot Helm chart for deploying a Machine ID Bot into a Teleport cluster. [#46374](https://github.com/gravitational/teleport/pull/46374)
* Ensure that additional pod labels are carried over to post-upgrade and post-delete hook job pods when using the teleport-kube-agent Helm chart. [#46231](https://github.com/gravitational/teleport/pull/46231)

## 15.4.18 (09/05/24)

* Fixed an issue that could result in duplicate session recordings being created. [#46264](https://github.com/gravitational/teleport/pull/46264)
* Added API resources for auto update (config and version). [#46257](https://github.com/gravitational/teleport/pull/46257)
* Added support for the teleport_installer resource to the Teleport Terraform provider. [#46202](https://github.com/gravitational/teleport/pull/46202)
* Fixed an issue that would cause reissue of certificates to fail in some scenarios where a local auth service was present. [#46183](https://github.com/gravitational/teleport/pull/46183)
* Updated OpenSSL to 3.0.15. [#46181](https://github.com/gravitational/teleport/pull/46181)
* Extended Teleport ability to use non-default cluster domains in Kubernetes, avoiding the assumption of `cluster.local`. [#46151](https://github.com/gravitational/teleport/pull/46151)
* Fixed retention period handling in the CockroachDB audit log storage backend. [#46148](https://github.com/gravitational/teleport/pull/46148)
* Prevented Teleport Kubernetes access from resending resize events to the party that triggered the terminal resize, avoiding potential resize loops. [#46067](https://github.com/gravitational/teleport/pull/46067)
* Fixed an issue where attempts to play/export certain session recordings would fail with `gzip: invalid header`. [#46034](https://github.com/gravitational/teleport/pull/46034)
* Fixed a bug where Teleport services could not join the cluster using IAM, Azure, or TPM methods when the proxy service certificate did not contain IP SANs. [#46009](https://github.com/gravitational/teleport/pull/46009)
* Updated the icons for server, application, and desktop resources. [#45991](https://github.com/gravitational/teleport/pull/45991)
* Failure to share a local directory in a Windows desktop session is no longer considered a fatal error. [#45853](https://github.com/gravitational/teleport/pull/45853)
* Fixed Okta role formatting in tsh login output. [#45582](https://github.com/gravitational/teleport/pull/45582)

## 15.4.17 (08/28/24)

* Prevent connections from being randomly terminated by Teleport proxies when `proxy_protocol` is enabled and TLS is terminated before Teleport Proxy. [#45993](https://github.com/gravitational/teleport/pull/45993)
* Fixed an issue where host_sudoers could be written to Teleport proxy server sudoer lists in Teleport v14 and v15. [#45961](https://github.com/gravitational/teleport/pull/45961)
* Prevent interactive sessions from hanging on exit. [#45953](https://github.com/gravitational/teleport/pull/45953)
* Fixed kernel version check of Enhanced Session Recording for distributions with backported BPF. [#45942](https://github.com/gravitational/teleport/pull/45942)
* Added a flag to skip a relogin attempt when using `tsh ssh` and `tsh proxy ssh`. [#45930](https://github.com/gravitational/teleport/pull/45930)
* Fixed an issue WebSocket upgrade fails with MiTM proxies that can remask payloads. [#45900](https://github.com/gravitational/teleport/pull/45900)
* When a database is created manually (without auto-discovery) the teleport.dev/db-admin and teleport.dev/db-admin-default-database labels are no longer ignored and can be used to configure database auto-user provisioning. [#45892](https://github.com/gravitational/teleport/pull/45892)
* Slack plugin now lists logins permitted by requested roles. [#45854](https://github.com/gravitational/teleport/pull/45854)
* Fixed an issue that prevented the creation of AWS App Access for an Integration that used digits only (eg, AWS Account ID). [#45818](https://github.com/gravitational/teleport/pull/45818)
* For new EKS Cluster auto-enroll configurations, the temporary Access Entry is tagged with `teleport.dev/` namespaced tags. For existing set ups, please add the `eks:TagResource` action to the Integration IAM Role to get the same behavior. [#45726](https://github.com/gravitational/teleport/pull/45726)
* Added support for importing S3 Bucket Tags into Teleport Policy's Access Graph. For existing configurations, ensure that the `s3:GetBucketTagging` permission is manually included in the Teleport Access Graph integration role. [#45550](https://github.com/gravitational/teleport/pull/45550)

## 15.4.16 (08/23/24)

### Security fix

#### [High] Stored XSS in SAML IdP

When registering a service provider with SAML IdP, Teleport did not sufficiently
validate the ACS endpoint. This could allow a Teleport administrator with
permissions to write saml_idp_service_provider resources to configure a
malicious service provider with an XSS payload and compromise session of users
who would access that service provider.

Note: This vulnerability is only applicable when Teleport itself is acting as
the identity provider. If you only use SAML to connect to an upstream identity
provider you are not impacted. You can use the tctl get
saml_idp_service_provider command to verify if you have any Service Provider
applications registered and Teleport acts as an IdP.

For self-hosted Teleport customers that use Teleport as SAML Identity Provider,
we recommend upgrading auth and proxy servers. Teleport agents (SSH, Kubernetes,
desktop, application, database and discovery) are not impacted and do not need
to be updated.

### Other fixes and improvements

* Fixed an issue where Teleport could modify group assignments for users not managed by Teleport. This will require a migration of host users created with create_host_user_mode: keep in order to maintain Teleport management. [#45792](https://github.com/gravitational/teleport/pull/45792)
* Fixed host user creation for tsh scp. [#45681](https://github.com/gravitational/teleport/pull/45681)
* Fixed AWS access failing when the username is longer than 64 characters. [#45656](https://github.com/gravitational/teleport/pull/45656)
* Permit setting a cluster wide SSH connection dial timeout. [#45651](https://github.com/gravitational/teleport/pull/45651)
* Improved performance of host resolution performed via tsh ssh when connecting via labels or proxy templates. [#45645](https://github.com/gravitational/teleport/pull/45645)
* Removed empty tcp app session recordings. [#45642](https://github.com/gravitational/teleport/pull/45642)
* Fixed Teleport plugins images using the wrong entrypoint. [#45618](https://github.com/gravitational/teleport/pull/45618)
* Added debug images for Teleport plugins. [#45618](https://github.com/gravitational/teleport/pull/45618)
* Fixed FeatureHiding flag not hiding the "Access Management" section in the UI. [#45613](https://github.com/gravitational/teleport/pull/45613)
* Fixed Host User Management deletes users that are not managed by Teleport. [#45595](https://github.com/gravitational/teleport/pull/45595)
* Fixed a security vulnerability with PostgreSQL integration where a maliciously crafted startup packet with an empty database name can bypass the intended access control. [#45555](https://github.com/gravitational/teleport/pull/45555)
* Fixed the debug service not being enabled by default when not using a configuration file. [#45479](https://github.com/gravitational/teleport/pull/45479)
* Introduced support for Envoy SDS into the Machine ID spiffe-workload-api service. [#45463](https://github.com/gravitational/teleport/pull/45463)
* Improved the output of `tsh sessions ls` to make it easier to understand what sessions are ongoing and what sessions are user can/should join as a moderator. [#45453](https://github.com/gravitational/teleport/pull/45453)
* Fixed access entry handling permission error when EKS auto-discovery was set up in the Discover UI. [#45443](https://github.com/gravitational/teleport/pull/45443)
* Fixed the web UI showing vague error messages when enrolling EKS clusters in the Discover UI. [#45416](https://github.com/gravitational/teleport/pull/45416)
* Fixed the "Create A Bot" flow for GitHub Actions and SSH not correctly granting the bot the role created during the flow. [#45410](https://github.com/gravitational/teleport/pull/45410)
* Fixed a panic caused by AWS STS client not being initialized when assuming an AWS Role. [#45381](https://github.com/gravitational/teleport/pull/45381)
* Fixed `teleport debug` commands incorrectly handling an unset data directory in the Teleport config. [#45342](https://github.com/gravitational/teleport/pull/45342)

Enterprise:
* Fixed Okta Sync spuriously deleting Okta Applications due to connectivity errors. [#4886](https://github.com/gravitational/teleport.e/pull/4886)
* Fixed Okta Sync mistakenly removing Apps and Groups on connectivity failure. [#4884](https://github.com/gravitational/teleport.e/pull/4884)
* Fixes the SAML IdP session preventing SAML IdP sessions from being consistently updated when users assumed a role or switched back from the role granted in the Access Request. [#4879](https://github.com/gravitational/teleport.e/pull/4879)
* Fixed a security issue where a user who can create `saml_idp_service_provider` resources can compromise the sessions of more powerful users and perform actions on behalf of others. [#4863](https://github.com/gravitational/teleport.e/pull/4863)
* Fixed the SAML IdP authentication middleware preventing users from signing into the service provider when an SAML authentication request was made with an HTTP-POST binding protocol and user's didn't already have an active session with Teleport. [#4852](https://github.com/gravitational/teleport.e/pull/4852)

## 15.4.12 (08/08/24)

* Improved copy and paste behavior in the terminal in Teleport Connect. On Windows and Linux, Ctrl+Shift+C/V now copies and pastes text (these shortcuts can be changed with `keymap.terminalCopy`/`keymap.terminalPaste`).  A mouse right click (`terminal.rightClick`) can copy/paste text too (enabled by default on Windows). [#45266](https://github.com/gravitational/teleport/pull/45266)
* Updated Go toolchain to `1.22.6`. [#45195](https://github.com/gravitational/teleport/pull/45195)
* Improved `tsh ssh` performance for concurrent execs. [#45163](https://github.com/gravitational/teleport/pull/45163)
* Fixed regression that denied access to launch some applications. [#45150](https://github.com/gravitational/teleport/pull/45150)
* Bot resources now honour their `metadata.expires` field. [#45133](https://github.com/gravitational/teleport/pull/45133)
* Teleport Connect now sets `TERM_PROGRAM: Teleport_Connect` and `TERM_PROGRAM_VERSION: <app_version>` environment variables in the integrated terminal. [#45064](https://github.com/gravitational/teleport/pull/45064)
* Fix a panic in the Microsoft teams plugin when it receives an error. [#45012](https://github.com/gravitational/teleport/pull/45012)
* Adds SPIFFE compatible federation bundle endpoint to the Proxy API, allowing other workload identity platforms to federate with the Teleport cluster. [#44999](https://github.com/gravitational/teleport/pull/44999)
* Added warning on `tbot` startup when the requested certificate TTL exceeds the maximum allowed value. [#44988](https://github.com/gravitational/teleport/pull/44988)
* Fixed race condition between session recording uploads and session recording upload cleanup. [#44979](https://github.com/gravitational/teleport/pull/44979)
* Prevent Kubernetes per-Resource RBAC from blocking access to namespaces when denying access to a single resource kind in every namespace. [#44975](https://github.com/gravitational/teleport/pull/44975)
* Fix `tbot` FIPS builds failing to start due to missing boringcrypto. [#44908](https://github.com/gravitational/teleport/pull/44908)
* Added support for Kubernetes Workload Attestation into Teleport Workload Identity to allow the authentication of pods running within Kubernetes without secrets. [#44884](https://github.com/gravitational/teleport/pull/44884)
* Machine ID can now be configured to use Kubernetes Secret destinations from the command line using the `kubernetes-secret` schema. [#44804](https://github.com/gravitational/teleport/pull/44804)
* Prevent discovery service from overwriting Teleport dynamic resources that have the same name as discovered resources. [#44786](https://github.com/gravitational/teleport/pull/44786)
* Teleport Connect now uses ConPTY for better terminal resizing and accurate color rendering on Windows, with an option to disable it in the app config. [#44743](https://github.com/gravitational/teleport/pull/44743)
* Fixed event-handler Helm charts using the wrong command when starting the event-handler container. [#44698](https://github.com/gravitational/teleport/pull/44698)
* Enabled Mattermost plugin for notification routing ruled. [#4773](https://github.com/gravitational/teleport.e/pull/4773)

## 15.4.11 (07/29/24)

* Fixed an issue that could cause auth servers to panic when their backend connectivity was interrupted. [#44787](https://github.com/gravitational/teleport/pull/44787)
* Reduced the probability that the event-handler deadlocks when encountering errors processing session recordings. [#44772](https://github.com/gravitational/teleport/pull/44772)
* Improved event-handler diagnostics by providing a way to capture profiles dynamically via `SIGUSR1`. [#44759](https://github.com/gravitational/teleport/pull/44759)
* Added support for Teams to Opsgenie plugin alert creation. [#44330](https://github.com/gravitational/teleport/pull/44330)

## 15.4.10 (07/28/24)

* Improved stability of very large teleport clusters during temporary backend disruption/degradation. [#44695](https://github.com/gravitational/teleport/pull/44695)
* Resolved compatibility issue with Paramiko and Machine ID's SSH multiplexer SSH agent. [#44672](https://github.com/gravitational/teleport/pull/44672)
* Fixed a fatal error in `tbot` when unable to lookup the user from a given UID in containerized environments for checking ACL configuration. [#44646](https://github.com/gravitational/teleport/pull/44646)
* Fixed application access regression where an HTTP header wasn't set in forwarded requests. [#44629](https://github.com/gravitational/teleport/pull/44629)
* Use the registered port of the target host when `tsh puttyconfig` is invoked without `--port`. [#44573](https://github.com/gravitational/teleport/pull/44573)
* Added more icons for guessing application icon by name or by label `teleport.icon` in the web UI. [#44568](https://github.com/gravitational/teleport/pull/44568)
* Removed deprecated S3 bucket option when creating or editing AWS OIDC integration in the web UI. [#44487](https://github.com/gravitational/teleport/pull/44487)
* Fixed terminal sessions with a database CLI client in Teleport Connect hanging indefinitely if the client cannot be found. [#44466](https://github.com/gravitational/teleport/pull/44466)
* Added application-tunnel service to Machine ID for establishing a long-lived tunnel to a HTTP or TCP application for Machine to Machine access. [#44446](https://github.com/gravitational/teleport/pull/44446)
* Fixed a low-probability panic in audit event upload logic. [#44424](https://github.com/gravitational/teleport/pull/44424)
* Fixed Teleport Connect binaries not being signed correctly. [#44420](https://github.com/gravitational/teleport/pull/44420)
* Prevented DoSing the cluster during a mass failed join event by agents. [#44415](https://github.com/gravitational/teleport/pull/44415)
* Added audit events for AWS and Azure integration resource actions. [#44404](https://github.com/gravitational/teleport/pull/44404)
* Fixed automatic updates with previous versions of the `teleport.yaml` config. [#44378](https://github.com/gravitational/teleport/pull/44378)
* Added support for Rocky and AlmaLinux when enrolling a new server from the UI. [#44331](https://github.com/gravitational/teleport/pull/44331)
* Fixed Teleport access plugin tarballs containing a `build` directory, which was accidentally added upon v15.4.5 release. [#44301](https://github.com/gravitational/teleport/pull/44301)
* Prevented an infinite loop in DynamoDB event querying by advancing the cursor to the next day when the limit is reached at the end of a day with an empty iterator. This ensures the cursor does not reset to the beginning of the day. [#44274](https://github.com/gravitational/teleport/pull/44274)
* The clipboard sharing tooltip for desktop sessions now indicates why clipboard sharing is disabled. [#44238](https://github.com/gravitational/teleport/pull/44238)
* Fixed a `kube-agent-updater` bug affecting resolutions of private images. [#44192](https://github.com/gravitational/teleport/pull/44192)
* Prevented redirects to arbitrary URLs when launching an app. [#44189](https://github.com/gravitational/teleport/pull/44189)
* Added audit event field describing if the "MFA for admin actions" requirement changed. [#44185](https://github.com/gravitational/teleport/pull/44185)
* The `teleport-cluster` chart can now use existing ingresses instead of creating its own. [#44147](https://github.com/gravitational/teleport/pull/44147)
* Ensured that `tsh login` outputs accurate status information for the new session. [#44144](https://github.com/gravitational/teleport/pull/44144)
* Fixed "device trust mode _x_ requires Teleport Enterprise" errors on `tctl`. [#44134](https://github.com/gravitational/teleport/pull/44134)
* Added a `--skip-idle-time` flag to `tsh play`. [#44095](https://github.com/gravitational/teleport/pull/44095)
* Added the `tbot install systemd` command for installing tbot as a service on Linux systems. [#44082](https://github.com/gravitational/teleport/pull/44082)
* Added ability to list Access List members in json format in `tctl` cli tool. [#44072](https://github.com/gravitational/teleport/pull/44072)
* Made `tbot` compilable on Windows. [#44070](https://github.com/gravitational/teleport/pull/44070)
* For slack integration, Access List reminders are batched into 1 message and provides link out to the web UI. [#44035](https://github.com/gravitational/teleport/pull/44035)
* Fixed denying access despite access being configured for Notification Routing Rules in the web UI. [#44028](https://github.com/gravitational/teleport/pull/44028)
* Fixed eBPF error occurring during startup on Linux RHEL 9. [#44024](https://github.com/gravitational/teleport/pull/44024)
* Lowered latency of detecting Kubernetes cluster becoming online. [#43971](https://github.com/gravitational/teleport/pull/43971)
* Enabled Access Monitoring Rules routing with Mattermost plugin. [#43600](https://github.com/gravitational/teleport/pull/43600)

Enterprise:
* Fixed an Access List permission bug where an Access List owner, who is also a member, was not able to add/rm Access List member.
* Fixed an issue with incorrect yum/zypper updater packages being installed.
* Fixed empty condition from unquoted string with yaml editor for Notification Routing Rules in the Web UI.

## 15.4.9 (07/11/24)

* Honor proxy templates in tsh ssh. [#44027](https://github.com/gravitational/teleport/pull/44027)
* Fixed Redshift auto-user deactivation/deletion failure that occurs when a user is created or deleted and another user is deactivated concurrently. [#43975](https://github.com/gravitational/teleport/pull/43975)
* Teleport AMIs now optionally source environment variables from `/etc/default/teleport` as regular Teleport package installations do. [#43961](https://github.com/gravitational/teleport/pull/43961)
* Enabled setting event types to forward, skip events, skip session types in event-handler helm chart. [#43939](https://github.com/gravitational/teleport/pull/43939)
* Correctly propagate `extraLabels` configured in teleport-kube-agent chart values to post-delete hooks. A new `extraLabels.job` object has been added for labels which should only apply to the post-delete job. [#43931](https://github.com/gravitational/teleport/pull/43931)
* Machine ID outputs now execute individually and concurrently, meaning that one failing output does not disrupt other outputs, and that performance when generating a large number of outputs is improved. [#43883](https://github.com/gravitational/teleport/pull/43883)
* Omit control plane services from the inventory list output for Cloud-Hosted instances. [#43778](https://github.com/gravitational/teleport/pull/43778)
* Fixed session recordings getting overwritten or not uploaded. [#42164](https://github.com/gravitational/teleport/pull/42164)

Enterprise:
* Fixed inaccurately notifying user that Access List reviews are due in the web UI.

## 15.4.7 (07/03/24)

* Added audit events for discovery config actions. [#43794](https://github.com/gravitational/teleport/pull/43794)
* Updated Go toolchain to v1.22.5. [#43769](https://github.com/gravitational/teleport/pull/43769)
* Reduced CPU usage in auth servers experiencing very high concurrent request load. [#43760](https://github.com/gravitational/teleport/pull/43760)
* Machine ID defaults to disabling the use of the Kubernetes exec plugin when writing a Kubeconfig to a directory destination. This removes the need to manually configure `disable_exec_plugin`. [#43656](https://github.com/gravitational/teleport/pull/43656)
* Fixed startup crash of Teleport Connect on Ubuntu 24.04 by adding an AppArmor profile. [#43652](https://github.com/gravitational/teleport/pull/43652)
* Added support for dialling leaf clusters to the tbot SSH multiplexer. [#43635](https://github.com/gravitational/teleport/pull/43635)
* Extend Teleport ability to use non-default cluster domains in Kubernetes, avoiding the assumption of `cluster.local`. [#43632](https://github.com/gravitational/teleport/pull/43632)
* Wait for user MFA input when reissuing expired certificates for a kube proxy. [#43613](https://github.com/gravitational/teleport/pull/43613)
* Improved error diagnostics when using Machine ID's SSH multiplexer. [#43587](https://github.com/gravitational/teleport/pull/43587)

Enterprise:
* Increased Access Monitoring refresh interval to 24h.
* Teleport Enterprise now supports the `TELEPORT_REPORTING_HTTP(S)_PROXY` environment variable to specify the URL of the HTTP(S) proxy used for connections to our usage reporting ingest service.

## 15.4.6 (06/27/24)

This release of Teleport contains a fix for medium-level security issue impacting
Teleport Enterprise, as well as various other updates and improvements

### Security Fixes

* **[Medium]** Fixes issue where a SCIM client could potentially overwrite.
  Teleport system Roles using specially crafted groups. This issue impacts
  Teleport Enterprise deployments using the Okta integration with SCIM support
  enabled.

We strongly recommend all customers upgrade to the latest releases of Teleport.

### Other updates and improvements

* Fixed Discover setup access error when updating user. [#43561](https://github.com/gravitational/teleport/pull/43561)
* Updated Go toolchain to 1.22. [#43550](https://github.com/gravitational/teleport/pull/43550)
* Fixed remote port forwarding validation error. [#43517](https://github.com/gravitational/teleport/pull/43517)
* Added support to trust system CAs for self-hosted databases. [#43500](https://github.com/gravitational/teleport/pull/43500)
* Added error display in the Web UI for SSH and Kubernetes sessions. [#43491](https://github.com/gravitational/teleport/pull/43491)
* Update `go-retryablehttp` to v0.7.7 (fixes CVE-2024-6104). [#43475](https://github.com/gravitational/teleport/pull/43475)
* Fixed accurate inventory reporting of the updater after it is removed.. [#43453](https://github.com/gravitational/teleport/pull/43453)
* `tctl alerts ls` now displays remaining alert ttl. [#43435](https://github.com/gravitational/teleport/pull/43435)
* Fixed input search for Teleport Connect's Access Request listing. [#43430](https://github.com/gravitational/teleport/pull/43430)
* Added `Debug` setting for event-handler. [#43409](https://github.com/gravitational/teleport/pull/43409)
* Fixed Headless auth for sso users, including when local auth is disabled. [#43362](https://github.com/gravitational/teleport/pull/43362)
* Added configuration for custom CAs in the event-handler helm chart. [#43341](https://github.com/gravitational/teleport/pull/43341)
* Fixed an issue with Database Access Controls preventing users from making additional database connections depending on their permissions. [#43302](https://github.com/gravitational/teleport/pull/43302)
* Fixed Connect My Computer in Teleport Connect failing with "bind: invalid argument". [#43288](https://github.com/gravitational/teleport/pull/43288)

### Enterprise only updates and improvements

* The teleport updater will no longer default to using the global version channel, avoiding incompatible updates. [#4476](https://github.com/gravitational/teleport.e/pull/4476)

## 15.4.5 (06/20/24)

* Added a missing `[Install]` section to the `teleport-acm` systemd unit file as used by Teleport AMIs. [#43256](https://github.com/gravitational/teleport/pull/43256)
* Patched timing variability in curve25519-dalek. [#43249](https://github.com/gravitational/teleport/pull/43249)
* Updated `tctl` to ignore a configuration file if the `auth_service` section is disabled, and prefer loading credentials from a given identity file or tsh profile instead. [#43203](https://github.com/gravitational/teleport/pull/43203)
* Fixed setting request reason for automatic ssh Access Requests. [#43180](https://github.com/gravitational/teleport/pull/43180)
* Updated `teleport` to skip `jamf_service` validation when the Jamf service is not enabled. [#43169](https://github.com/gravitational/teleport/pull/43169)
* Improved log rotation logic in Teleport Connect; now the non-numbered files always contain recent logs. [#43162](https://github.com/gravitational/teleport/pull/43162)
* Made `tsh` and Teleport Connect return early during login if ping to Proxy Service was not successful. [#43086](https://github.com/gravitational/teleport/pull/43086)
* Added ability to edit user traits from the Web UI. [#43068](https://github.com/gravitational/teleport/pull/43068)
* Enforce limits when reading events from Firestore to prevent OOM events. [#42967](https://github.com/gravitational/teleport/pull/42967)
* Fixed updating groups for Teleport-created host users. [#42884](https://github.com/gravitational/teleport/pull/42884)
* Added support for `crown_jewel` resource. [#42866](https://github.com/gravitational/teleport/pull/42866)
* Added ability to edit user traits from the Web UI. [#43068](https://github.com/gravitational/teleport/pull/43068)
* Fixed gRPC disconnection on certificate expiry even though DisconnectCertExpiry was false. [#43291](https://github.com/gravitational/teleport/pull/43291)
* Fixed issue where a Teleport instance running only Jamf or Discovery service would never have a healthy `/readyz` endpoint.  [#43284](https://github.com/gravitational/teleport/pull/43284)

### Enterprise-only changes

* Fixed sync error in Okta SCIM integration.

## 15.4.4 (06/13/24)

* Improve search and predicate/label based dialing performance in large clusters under very high load. [#42941](https://github.com/gravitational/teleport/pull/42941)
* Fix an issue Oracle access failed through trusted cluster. [#42928](https://github.com/gravitational/teleport/pull/42928)
* Fix errors caused by `dynamoevents` query `StartKey` not being within the [From, To] window. [#42915](https://github.com/gravitational/teleport/pull/42915)
* Fix Jira Issue creation when Summary exceeds the max allowed size. [#42862](https://github.com/gravitational/teleport/pull/42862)
* Fix editing reviewers from being ignored/overwritten when creating an Access Request from the web UI. [#4397](https://github.com/gravitational/teleport.e/pull/4397)

## 15.4.3 (06/12/24)

**Note:** This release includes a new binary, `fdpass-teleport`, that can be
optionally used by Machine ID to significantly reduce resource consumption in
use-cases that create large numbers of SSH connections (e.g. Ansible). Refer to
the [documentation](docs/pages/reference/machine-id/configuration.mdx#ssh-multiplexer)
for more details.

* Update `azidentity` to `v1.6.0` (patches `CVE-2024-35255`). [#42859](https://github.com/gravitational/teleport/pull/42859)
* Remote rate limits on endpoints used extensively to connect to the cluster. [#42835](https://github.com/gravitational/teleport/pull/42835)
* Machine ID SSH multiplexer now only writes artifacts if they have not changed, resolving a potential race condition with the OpenSSH client. [#42830](https://github.com/gravitational/teleport/pull/42830)
* Use more efficient API when querying SSH nodes to resolve Proxy Templates in `tbot`. [#42829](https://github.com/gravitational/teleport/pull/42829)
* Improve the performance of the Athena audit log and S3 session storage backends. [#42795](https://github.com/gravitational/teleport/pull/42795)
* Prevent a panic in the Proxy when accessing an offline application. [#42786](https://github.com/gravitational/teleport/pull/42786)
* Improve backoff of session recording uploads by teleport agents. [#42776](https://github.com/gravitational/teleport/pull/42776)
* Introduce the new Machine ID `ssh-multiplexer` service for significant improvements in SSH performance. [#42761](https://github.com/gravitational/teleport/pull/42761)
* Reduce backend writes incurred by tracking status of non-recorded sessions. [#42694](https://github.com/gravitational/teleport/pull/42694)
* Fix not being able to logout from the web UI when session invalidation errors. [#42648](https://github.com/gravitational/teleport/pull/42648)
* Fix Access List listing not updating when creating or deleting an Access List in the web UI. [#4383](https://github.com/gravitational/teleport.e/pull/4383)
* Fix crashes related to importing GCP labels. [#42871](https://github.com/gravitational/teleport/pull/42871)

## 15.4.2 (06/11/24)

* Fixed a desktop access resize bug which occurs when window was resized during MFA. [#42705](https://github.com/gravitational/teleport/pull/42705)
* Fixed listing available db users in Teleport Connect for databases from leaf clusters obtained through Access Requests. [#42679](https://github.com/gravitational/teleport/pull/42679)
* Fixed file upload/download for Teleport-created users in `insecure-drop` mode. [#42660](https://github.com/gravitational/teleport/pull/42660)
* Updated OpenSSL to 3.0.14. [#42642](https://github.com/gravitational/teleport/pull/42642)
* Fixed fetching resources with tons of metadata (such as labels or description) in Teleport Connect. [#42627](https://github.com/gravitational/teleport/pull/42627)
* Added support for Microsoft Entra ID directory synchronization (Teleport Enterprise only, preview). [#42555](https://github.com/gravitational/teleport/pull/42555)
* Added experimental support for storing audit events in cockroach. [#42549](https://github.com/gravitational/teleport/pull/42549)
* Teleport Connect binaries for Windows are now signed. [#42472](https://github.com/gravitational/teleport/pull/42472)
* Updated Go to 1.21.11. [#42404](https://github.com/gravitational/teleport/pull/42404)
* Added GCP Cloud SQL for PostgreSQL backend support. [#42399](https://github.com/gravitational/teleport/pull/42399)
* Added Prometheus metrics for the Postgres event backend. [#42384](https://github.com/gravitational/teleport/pull/42384)
* Fixed the event-handler Helm chart causing stuck rollouts when using a PVC. [#42363](https://github.com/gravitational/teleport/pull/42363)
* Fixed web UI notification dropdown menu height from growing too long from many notifications. [#42336](https://github.com/gravitational/teleport/pull/42336)
* Disabled session recordings for non-interactive sessions when enhanced recording is disabled. There is no loss of auditing or impact on data fidelity because these recordings only contained session.start, session.end, and session.leave events which were already captured in the audit log. This will cause all teleport components to consume less resources and reduce storage costs. [#42320](https://github.com/gravitational/teleport/pull/42320)
* Fixed an issue where removing an app could make teleport app agents incorrectly report as unhealthy for a short time. [#42270](https://github.com/gravitational/teleport/pull/42270)
* Fixed a panic in the DynamoDB audit log backend when the cursor fell outside of the [From,To] interval. [#42267](https://github.com/gravitational/teleport/pull/42267)
* The `teleport configure` command now supports a `--node-name` flag for overriding the node's hostname. [#42250](https://github.com/gravitational/teleport/pull/42250)
* Added support plugin resource in `tctl` tool. [#42224](https://github.com/gravitational/teleport/pull/42224)

## 15.4.0 (05/31/24)

### Access requests notification routing rules

Hosted Slack plugin users can now configure notification routing rules for
role-based Access Requests.

### Database access for Spanner

Database access users can now connect to GCP Spanner.

### Unix Workload Attestation

*Delayed from Teleport 15.3.0*

Teleport Workload ID now supports basic workload attestation on Unix systems,
allowing cluster administrators to restrict the issuance of SVIDs to specific
workloads based on UID/PID/GID.

### Other improvements and fixes

* Fixed an issue where mix-and-match of join tokens could interfere with some services appearing correctly in heartbeats. [#42189](https://github.com/gravitational/teleport/pull/42189)
* Added an alternate EC2 auto discover flow using AWS Systems Manager as a more scalable method than Endpoint Instance Connect in the "Enroll New Resource" view in the web UI. [#42205](https://github.com/gravitational/teleport/pull/42205)
* Fixed `kubectl exec` functionality when Teleport is running behind L7 load balancer. [#42192](https://github.com/gravitational/teleport/pull/42192)
* Fixed the plugins AMR cache to be updated when Access requests are removed from the subject of an existing rule. [#42186](https://github.com/gravitational/teleport/pull/42186)
* Improved temporary disk space usage for session recording processing. [#42174](https://github.com/gravitational/teleport/pull/42174)
* Fixed a regression where Kubernetes Exec audit events were not properly populated and lacked error details. [#42145](https://github.com/gravitational/teleport/pull/42145)
* Fixed Azure join method when using Resource Groups in the allow section. [#42141](https://github.com/gravitational/teleport/pull/42141)
* Added new `teleport debug set-log-level / profile` commands changing instance log level without a restart and collecting pprof profiles. [#42122](https://github.com/gravitational/teleport/pull/42122)
* Added ability to manage access monitoring rules via `tctl`. [#42092](https://github.com/gravitational/teleport/pull/42092)
* Added access monitoring rule routing for slack access plugin. [#42087](https://github.com/gravitational/teleport/pull/42087)
* Extended Discovery Service to self-bootstrap necessary permissions for Kubernetes Service to interact with the Kubernetes API on behalf of users. [#42075](https://github.com/gravitational/teleport/pull/42075)
* Fixed resource leak in session recording cleanup. [#42066](https://github.com/gravitational/teleport/pull/42066)
* Reduced memory and CPU usage after control plane restarts in clusters with a high number of roles. [#42062](https://github.com/gravitational/teleport/pull/42062)
* Added an option to send a `Ctrl+Alt+Del` sequence to remote desktops. [#41720](https://github.com/gravitational/teleport/pull/41720)
* Added support for GCP Spanner to Teleport Database Service. [#41349](https://github.com/gravitational/teleport/pull/41349)

## 15.3.7 (05/23/24)

* Fixed creating Access Requests for servers in Teleport Connect that were blocked due to a "no roles configured" error. [#41959](https://github.com/gravitational/teleport/pull/41959)
* Fixed regression issue with event-handler Linux artifacts not being available. [#4237](https://github.com/gravitational/teleport.e/pull/4237)
* Fixed failed startup on GCP if missing permissions. [#41985](https://github.com/gravitational/teleport/pull/41985)

## 15.3.6 (05/22/24)

This release contains fixes for several high-severity security issues, as well
as numerous other bug fixes and improvements.

### Security Fixes

* **[High]** Fixed unrestricted redirect in SSO Authentication. Teleport didn’t
  sufficiently validate the client redirect URL. This could allow an attacker to
  trick Teleport users into performing an SSO authentication and redirect to an
  attacker-controlled URL allowing them to steal the credentials. [#41834](https://github.com/gravitational/teleport/pull/41834).

* **[High]** Fixed CockroachDB authorization bypass. When connecting to
  CockroachDB using database access, Teleport did not properly consider the
  username case when running RBAC checks. As such, it was possible to establish
  a connection using an explicitly denied username when using a different case.
  [#41823](https://github.com/gravitational/teleport/pull/41823).
  
* **[High]** Fixed Long-lived connection persistence issue with expired
  certificates. Teleport did not terminate some long-running mTLS-authenticated
  connections past the expiry of client certificates for users with the
  `disconnect_expired_cert` option. This could allow such users to perform
  some API actions after their certificate has expired.  [#41827](https://github.com/gravitational/teleport/pull/41827).

* **[High]** Fixed PagerDuty integration privilege escalation. When creating a
  role Access Request, Teleport would include PagerDuty annotations from the
  entire user’s role set rather than a specific role being requested. For users
  who run multiple PagerDuty access plugins with auto-approval, this could
  result in a request for a different role being inadvertently auto-approved 
  than the one which corresponds to the user’s active on-call schedule. [#41837](https://github.com/gravitational/teleport/pull/41837).
  
* **[High]** Fixed SAML IdP session privilege escalation. When using Teleport as
  SAML IdP, authorization wasn’t properly enforced on the SAML IdP session
  creation. As such, authenticated users could use an internal API to escalate
  their own privileges by crafting a malicious program. [#41846](https://github.com/gravitational/teleport/pull/41846).  

We strongly recommend all customers upgrade to the latest releases of Teleport.

### Other fixes and improvements

* Fixed Access Request annotations when annotations contain globs, regular
  expressions, trait expansions, or `claims_to_roles` is used. [#41936](https://github.com/gravitational/teleport/pull/41936).
* Added AWS Management Console as a guided flow using AWS OIDC integration in
  the "Enroll New Resource" view in the web UI. [#41864](https://github.com/gravitational/teleport/pull/41864).
* Fixed spurious Windows Desktop sessions screen resize during an MFA ceremony. [#41856](https://github.com/gravitational/teleport/pull/41856).
* Fixed session upload completion with large number of simultaneous session
  uploads. [#41854](https://github.com/gravitational/teleport/pull/41854).
* Fixed MySQL databases version reporting on new connections. [#41819](https://github.com/gravitational/teleport/pull/41819).
* Added read-only permissions for cluster maintenance config. [#41790](https://github.com/gravitational/teleport/pull/41790).
* Stripped debug symbols from Windows builds, resulting in smaller `tsh` and
  `tctl` binaries. [#41787](https://github.com/gravitational/teleport/pull/41787)
* Fixed passkey deletion so that a user may now delete their last passkey if
  the have a password and another MFA configured. [#41771](https://github.com/gravitational/teleport/pull/41771).
* Changed the default permissions for the Workload Identity Unix socket to `0777`
  rather than the default as applied by the umask. This will allow the socket to
  be accessed by workloads running as users other than the user that owns the
  `tbot` process. [#41754](https://github.com/gravitational/teleport/pull/41754)
* Added ability for `teleport-event-handler` to skip certain events type when
  forwarding to an upstream server. [#41747](https://github.com/gravitational/teleport/pull/41747).
* Added automatic GCP label importing. [#41733](https://github.com/gravitational/teleport/pull/41733).
* Fixed missing variable and script options in Default Agentless Installer
  script. [#41723](https://github.com/gravitational/teleport/pull/41723).
* Removed invalid AWS Roles from Web UI picker. [#41707](https://github.com/gravitational/teleport/pull/41707).
* Added remote address to audit log events emitted when a Bot or Instance join
  completes, successfully or otherwise. [#41700](https://github.com/gravitational/teleport/pull/41700).
* Simplified how Bots are shown on the Users list page. [#41697](https://github.com/gravitational/teleport/pull/41697).
* Added improved-performance implementation of ProxyCommand for Machine ID and
  SSH. This will become the default in v16. You can adopt this new mode early by
  setting `TBOT_SSH_CONFIG_PROXY_COMMAND_MODE=new`. [#41694](https://github.com/gravitational/teleport/pull/41694).
* Improved EC2 Auto Discovery by adding the SSM script output and more explicit
  error messages. [#41664](https://github.com/gravitational/teleport/pull/41664).
* Added webauthn diagnostics commands to `tctl`. [#41643](https://github.com/gravitational/teleport/pull/41643).
* Upgraded application heartbeat service to support 1000+ dynamic applications. [#41626](https://github.com/gravitational/teleport/pull/41626)
* Fixed issue where Kubernetes watch requests are written out of order. [#41624](https://github.com/gravitational/teleport/pull/41624).
* Fixed a race condition triggered by a reload during Teleport startup. [#41592](https://github.com/gravitational/teleport/pull/41592).
* Updated discover wizard Install Script to support Ubuntu 24.04. [#41589](https://github.com/gravitational/teleport/pull/41589).
* Fixed `systemd` unit to always restart Teleport on failure unless explicitly stopped. [#41581](https://github.com/gravitational/teleport/pull/41581).
* Updated Teleport package installers to reload Teleport service config after
  upgrades. [#41547](https://github.com/gravitational/teleport/pull/41547).
* Fixed file truncation bug in Desktop Directory Sharing. [#41540](https://github.com/gravitational/teleport/pull/41540).
* Fixed WebUI SSH connection leak when browser tab closed during SSH connection
  establishment. [#41518](https://github.com/gravitational/teleport/pull/41518).
* Fixed AccessList reconciler comparison causing audit events noise. [#41517](https://github.com/gravitational/teleport/pull/41517).
* Added tooling to create SCIM integrations in tctl. [#41514](https://github.com/gravitational/teleport/pull/41514).
* Fixed Windows Desktop error preventing rendering of the remote session. [#41498](https://github.com/gravitational/teleport/pull/41498).
* Fixed issue in the PagerDuty, Opsgenie and ServiceNow access plugins that
  causing duplicate calls on Access Requests containing duplicate service names.
  Also increases the timeout so slow external API requests are less likely to
  fail. [#41488](https://github.com/gravitational/teleport/pull/41488).
* Added basic Unix workload attestation to the `tbot` SPIFFE workload API. You
  can now restrict the issuance of certain SVIDs to processes running with a
  certain UID, GID or PID. [#41450](https://github.com/gravitational/teleport/pull/41450).
* Added "login failed" audit events for invalid passwords on password+webauthn
  local authentication. [#41432](https://github.com/gravitational/teleport/pull/41432).
  Fixed Terraform provider issue causing the Provision Token options to default
  to `false` instead of empty. [#41429](https://github.com/gravitational/teleport/pull/41429).
* Added support to automatically download CA for MongoDB Atlas databases. [#41338](https://github.com/gravitational/teleport/pull/41338).
* Fixed broken "finish" web page for SSO Users on auto discover. [#41335](https://github.com/gravitational/teleport/pull/41335).
* Allow setting Kubernetes Cluster name when using non-default addresses. [#41331](https://github.com/gravitational/teleport/pull/41331).
* Added fallback on GetAccessList cache miss call. [#41326](https://github.com/gravitational/teleport/pull/41326).
* Fixed DiscoveryService panic when auto-enrolling EKS clusters. [#41320](https://github.com/gravitational/teleport/pull/41320).
* Added validation for application URL extracted from the web application launcher request route. [#41304](https://github.com/gravitational/teleport/pull/41304).
* Allow defining custom database names and users when selecting wildcard during test connection when enrolling a database through the web UI. [#41301](https://github.com/gravitational/teleport/pull/41301).
* Fixed broken link for alternative EC2 installation during EC2 discover flow. [#41292](https://github.com/gravitational/teleport/pull/41292)
* Updated Go to v1.21.10. [#41281](https://github.com/gravitational/teleport/pull/41281).
* Updated user management to explicitly deny password resets and local logins to
  SSO users. [#41270](https://github.com/gravitational/teleport/pull/41270).
* Fixed fetching suggested Access Lists with large IDs in Teleport Connect. [#41269](https://github.com/gravitational/teleport/pull/41269).
* Prevents cloud tenants from updating `cluster_networking_config` fields `keep_alive_count_max`,  `keep_alive_interval`, `tunnel_strategy`, or `proxy_listener_mode`. [#41247](https://github.com/gravitational/teleport/pull/41247).
* Added support for creating Okta integrations with `tctl` [#41888](https://github.com/gravitational/teleport/pull/41888).

## 15.3.1 (05/07/24)

* Fixed `screen_size` behavior for Windows Desktops, which was being overridden by the new resize feature. [#41241](https://github.com/gravitational/teleport/pull/41241)
* Ensure that the active sessions page shows up in the web UI for users with permissions to join sessions. [#41221](https://github.com/gravitational/teleport/pull/41221)
* Added indicators on the account settings page that tell which authentication methods are active. [#41169](https://github.com/gravitational/teleport/pull/41169)
* Fix a bug that was preventing tsh proxy kube certificate renewal from working when accessing a leaf kubernetes cluster via the root. [#41158](https://github.com/gravitational/teleport/pull/41158)
* Fixed `AccessDeniedException` for `dynamodb:ConditionCheckItem` operations when using Amazon DynamoDB for cluster state storage. [#41133](https://github.com/gravitational/teleport/pull/41133)
* Added lock target to lock deletion audit events. [#41112](https://github.com/gravitational/teleport/pull/41112)
* Fixed a permissions issue that prevented the teleport-cluster helm chart operator from registering agentless ssh servers. [#41108](https://github.com/gravitational/teleport/pull/41108)
* Improve the reliability of the upload completer. [#41103](https://github.com/gravitational/teleport/pull/41103)
* Allows the listener for the `tbot` `database-tunnel` service to be set to a unix socket. [#41008](https://github.com/gravitational/teleport/pull/41008)

## 15.3.0 (04/30/24)

### Improved Roles UI

The Roles page of the web UI is now backed by a paginated API, improving
load times even on clusters with large numbers of roles.

### Resizing for Windows desktop sessions

Windows desktop sessions now automatically resize as the size of the browser
window changes.

### Hardware key support for agentless nodes

Teleport now supports connecting to agentless OpenSSH nodes even when Teleport
is configured to require hardware key MFA checks.

### TPM joining

The new TPM join method enables secure joining for agents and Machine ID bots
that run on-premise. Based on the secure properties of the host's hardware
trusted platform module, this join method removes the need to create and
distribute secret tokens, significantly reducing the risk of exfiltration.

### Other improvements and fixes

* Fixed user SSO bypass by performing a local passwordless login. [#41067](https://github.com/gravitational/teleport/pull/41067)
* Enforce allow_passwordless server-side. [#41057](https://github.com/gravitational/teleport/pull/41057)
* Fixed a memory leak caused by incorrectly passing the offset when paginating all Access Lists' members when there are more than the default pagesize (200) Access Lists. [#41045](https://github.com/gravitational/teleport/pull/41045)
* Added resize capability to windows desktop sessions. [#41025](https://github.com/gravitational/teleport/pull/41025)
* Fixed a regression causing roles filtering to not work. [#40999](https://github.com/gravitational/teleport/pull/40999)
* Allow AWS integration to be used for global services without specifying a valid region. [#40991](https://github.com/gravitational/teleport/pull/40991)
* Made account id visible when selecting IAM Role for accessing the AWS Console. [#40987](https://github.com/gravitational/teleport/pull/40987)

## 15.2.5 (04/26/24)

* Extend proxy templates to allow the target host to be resolved via a predicate expression or fuzzy matching. [#40966](https://github.com/gravitational/teleport/pull/40966)
* Fix an issue where Access Requests would linger in UI and tctl after expiry. [#40964](https://github.com/gravitational/teleport/pull/40964)
* The `teleport-cluster` Helm chart can configure AccessMonitoring when running in `aws` mode. [#40957](https://github.com/gravitational/teleport/pull/40957)
* Make `podSecurityContext` configurable in the `teleport-cluster` Helm chart. [#40951](https://github.com/gravitational/teleport/pull/40951)
* Allow to mount extra volumes in the updater pod deployed by the `teleport-kube-agent`chart. [#40946](https://github.com/gravitational/teleport/pull/40946)
* Improve error message when performing an SSO login with a hardware key. [#40923](https://github.com/gravitational/teleport/pull/40923)
* Fix a bug in the `teleport-cluster` Helm chart that happened when `sessionRecording` was `off`. [#40919](https://github.com/gravitational/teleport/pull/40919)
* Fix audit event failures when using DynamoDB event storage. [#40913](https://github.com/gravitational/teleport/pull/40913)
* Allow setting additional Kubernetes labels on resources created by the `teleport-cluster` Helm chart. [#40909](https://github.com/gravitational/teleport/pull/40909)
* Fix Windows cursor getting stuck. [#40890](https://github.com/gravitational/teleport/pull/40890)
* Issue `cert.create` events during device authentication. [#40872](https://github.com/gravitational/teleport/pull/40872)
* Add the ability to control `ssh_config` generation in Machine ID's Identity Outputs. This allows the generation of the `ssh_config` to be disabled if unnecessary, improving performance and removing the dependency on the Proxy being online. [#40861](https://github.com/gravitational/teleport/pull/40861)
* Prevent deleting AWS OIDC integration used by External Audit Storage. [#40851](https://github.com/gravitational/teleport/pull/40851)
* Introduce the `tpm` join method, which allows for secure joining in on-prem environments without the need for a shared secret. [#40823](https://github.com/gravitational/teleport/pull/40823)
* Reduce parallelism when polling AWS resources to prevent API throttling when exporting them to Teleport Access Graph. [#40811](https://github.com/gravitational/teleport/pull/40811)
* Fix spurious deletion of Access List Membership metadata during SCIM push or sync. [#40544](https://github.com/gravitational/teleport/pull/40544)
* Properly enforce session moderation requirements when starting Kubernetes ephemeral containers. [#40906](https://github.com/gravitational/teleport/pull/40906)

## 15.2.4 (04/23/24)

* Fixed a deprecation warning being shown when `tbot` is used with OpenSSH. [#40837](https://github.com/gravitational/teleport/pull/40837)
* Added a new Audit log event that is emitted when an Agent or Bot request to join the cluster is denied. [#40814](https://github.com/gravitational/teleport/pull/40814)
* Fixed regenerating cloud account recovery codes. [#40786](https://github.com/gravitational/teleport/pull/40786)
* Changed UI for the sign-up and authentication reset flows. [#40773](https://github.com/gravitational/teleport/pull/40773)
* Added a new Prometheus metric to track requests initiated by Teleport against the control plane API. [#40754](https://github.com/gravitational/teleport/pull/40754)
* Fixed an issue that prevented uploading a zip file larger than 10MiB when updating an AWS Lambda function via tsh app access. [#40737](https://github.com/gravitational/teleport/pull/40737)
* Patched CVE-2024-32650. [#40735](https://github.com/gravitational/teleport/pull/40735)
* Fixed possible data race that could lead to concurrent map read and map write while proxying Kubernetes requests. [#40720](https://github.com/gravitational/teleport/pull/40720)
* Fixed Access Request promotion of windows_desktop resources. [#40712](https://github.com/gravitational/teleport/pull/40712)
* Fixed spurious ambiguous host errors in ssh routing. [#40706](https://github.com/gravitational/teleport/pull/40706)
* Patched CVE-2023-45288 and CVE-2024-32473. [#40695](https://github.com/gravitational/teleport/pull/40695)
* generic "not found" errors are returned whether a remote cluster can't be found or access is denied. [#40681](https://github.com/gravitational/teleport/pull/40681)
* Fixed a resource leak in the Teleport proxy server when using proxy peering. [#40672](https://github.com/gravitational/teleport/pull/40672)
* Added Azure CLI access support on AKS with Entra Workload ID. [#40660](https://github.com/gravitational/teleport/pull/40660)
* Allow other issue types when configuring JIRA plugin. [#40644](https://github.com/gravitational/teleport/pull/40644)
* Added `regexp.match` to Access Request `filter` and `where` expressions. [#40642](https://github.com/gravitational/teleport/pull/40642)
* Notify the requester in slack review request messages. [#40624](https://github.com/gravitational/teleport/pull/40624)
* Handle passwordless in MFA audit events. [#40617](https://github.com/gravitational/teleport/pull/40617)
* Added auto discover capability to EC2 enrollment in the web UI. [#40605](https://github.com/gravitational/teleport/pull/40605)
* Fixes RDP licensing. [#40595](https://github.com/gravitational/teleport/pull/40595)
* Added support for the ascii variants of smartcard calls. [#40566](https://github.com/gravitational/teleport/pull/40566)
* Added the ability to configure labels that should be set on the Kubernetes secret when using the `kubernetes_secret` destination in `tbot`. [#40550](https://github.com/gravitational/teleport/pull/40550)
* Updated cosign to address CVE-2024-29902 and CVE-2024-29903. [#40497](https://github.com/gravitational/teleport/pull/40497)
* The Web UI now supports large number of roles by paginating them. [#40463](https://github.com/gravitational/teleport/pull/40463)
* Improved the responsiveness of the session player during long periods of idle time. [#40442](https://github.com/gravitational/teleport/pull/40442)
* Fixed incorrect format for database_object_import_rule resources with non-empty expiry. [#40203](https://github.com/gravitational/teleport/pull/40203)
* Updated Opsgenie annotations so approve-schedules is used for both alert creation and auto approval if notify schedules is not set. [#40121](https://github.com/gravitational/teleport/pull/40121)

## 15.2.2 (04/11/24)

* Updated the cluster selector in the UI to now only be visible when more than one cluster is available. [#40478](https://github.com/gravitational/teleport/pull/40478)
* Fixed accidental passkey "downgrades" to MFA. [#40409](https://github.com/gravitational/teleport/pull/40409)
* Added `tsh proxy kube --exec` mode that spawns kube proxy in the background, which re-executes the user shell with the appropriate kubeconfig. [#40395](https://github.com/gravitational/teleport/pull/40395)
* Made Amazon S3 fields optional when creating or editing AWS OIDC integration on the web UI. [#40368](https://github.com/gravitational/teleport/pull/40368)
* Fixed a bug that prevented the available logins from being displayed for Windows desktops in leaf clusters that were being accessed via the root cluster web ui. [#40367](https://github.com/gravitational/teleport/pull/40367)
* Changed Teleport Connect to hide cluster name in the connection list if there is only a single cluster available. [#40356](https://github.com/gravitational/teleport/pull/40356)
* Fixed `invalid session TTL` error when creating Access Request with `tsh`. [#40335](https://github.com/gravitational/teleport/pull/40335)
* Added missing discovery AWS matchers fields "Integration" and "KubeAppDiscovery" to the file configuration. [#40320](https://github.com/gravitational/teleport/pull/40320)
* Added automatic role Access Requests. [#40285](https://github.com/gravitational/teleport/pull/40285)
* Redesigned the login UI. [#40272](https://github.com/gravitational/teleport/pull/40272)
* Added friendly role names for Okta sourced roles. These will be displayed in Access List and Access Request pages in the UI. [#40260](https://github.com/gravitational/teleport/pull/40260)
* Added Teleport Machine ID Workload Identity support for legacy systems which are not able to parse DNS SANs, and which are not SPIFFE aware. [#40180](https://github.com/gravitational/teleport/pull/40180)

## 15.2.1 (04/05/24)

* Teleport Connect now shows all recent connections instead of capping them at 10. [#40250](https://github.com/gravitational/teleport/pull/40250)
* Limit max read size for the tsh device trust DMI cache file on Linux. [#40234](https://github.com/gravitational/teleport/pull/40234)
* Fix an issue that prevents the teleport service from restarting. [#40229](https://github.com/gravitational/teleport/pull/40229)
* Add new resource filtering predicates to allow exact matches on a single item of a delimited list stored in a label value. For example, if given the following label containing a string separated list of values `foo=bar,baz,bang`, it is now possible to match on any resources with a label `foo` that contains the element `bar` via `contains(split(labels[foo], ","), bar)`. [#40183](https://github.com/gravitational/teleport/pull/40183)
* Updated Go to 1.21.9. [#40176](https://github.com/gravitational/teleport/pull/40176)
* Adds `disable_exec_plugin` option to the Machine ID Kubernetes Output to remove the dependency on `tbot` existing in the target environment. [#40162](https://github.com/gravitational/teleport/pull/40162)
* Adds the `database-tunnel` service to `tbot` which allows an authenticated database tunnel to be opened by `tbot`. This is an improvement over the original technique of using `tbot proxy db`. [#40151](https://github.com/gravitational/teleport/pull/40151)
* Allow diagnostic endpoints to be accessed behind a PROXY protocol enabled loadbalancer/proxy. [#40138](https://github.com/gravitational/teleport/pull/40138)
* Include system annotations in audit event entries for Access Requests. [#40123](https://github.com/gravitational/teleport/pull/40123)
* Fixed GitHub Auth Connector update event to show in Audit Log with name and description. [#40116](https://github.com/gravitational/teleport/pull/40116)
* Re-enabled the `show_desktop_wallpaper` flag. [#40088](https://github.com/gravitational/teleport/pull/40088)
* Reduce default Jamf inventory page size, allow custom values to be provided. [#3817](https://github.com/gravitational/teleport.e/pull/3817)

## 15.2.0 (03/29/24)

### Improved Access Requests UI

The Access Requests page of the web UI will be backed by a paginated API,
improving load times even on clusters with many Access Requests.

Additionally, the UI allows you to search for Access Requests, sort them based
on various attributes, and includes several new filtering options.

### Zero-downtime web asset rollout

Teleport 15.2 changes the way that web assets are served and cached, which will
allow multiple compatible versions of the Teleport Proxy to run behind the same
load balancer.

### Workload Identity MVP

With Teleport 15.2, Machine ID can bootstrap and issue identity to services
across multiple computing environments and organizational boundaries. Workload
Identity issues SPIFFE-compatible x509 certificates that can be used for mTLS
between services.

### Support for Kubernetes 1.29+

The Kubernetes project is deprecating the SPDY protocol for streaming commands
(kubectl exec, kubectl port-forward, etc) and replacing it with a new
websocket-based subprotocol. Teleport 15.2.0 will support the new protocol to
ensure compatibility with newer Kubernetes clusters.

### Automatic database Access Requests

Both tsh db connect and tsh proxy db will offer the option to submit an access
request if the user attempts to connect to a database that they don't already
have access to.

### GCP console access via Workforce Identity Federation

Teleport administrators will be able to setup access to GCP web console through
Workforce Identity Federation using Teleport as a SAML identity provider.

### IaC support for OpenSSH nodes

Users will be able to register OpenSSH nodes in the cluster using Terraform and
Kubernetes Operator.

### Access requests start time

Users submitting Access Requests via web UI will be able to request specific
access start time up to a week in advance.

### Terraform and Operator support for agentless SSH nodes

The Teleport Terraform provider and Kubernetes operator now support declaring
agentless OpenSSH and OpenSSH EC2 ICE servers. You can follow [this
guide](docs/pages/admin-guides/infrastructure-as-code/managing-resources/agentless-ssh-servers.mdx)
to register OpenSSH agents with infrastructure as code.

Setting up EC2 ICE automatic discovery with IaC will come in a future update.

### Operator and CRDs can be deployed separately

The `teleport-operator` and `teleport-cluster` charts now support deploying only
the CRD, the CRD and the operator, or only the operator.

From the `teleport-cluster` Helm chart:

```yaml
operator:
  enabled: true|false
  installCRDs: always|never|dynamic
```

From the `teleport-operator` Helm chart:

```yaml
enabled: true|false
installCRDs: always|never|dynamic
```

In dynamic mode (by default), the chart will install CRDs if the operator is
enabled, but will not remove the CRDs if you temporarily disable the operator.

### Operator now propagates labels

Kubernetes CR labels are now copied to the Teleport resource when applicable.
This allows you to configure RBAC for operator-created resources, and to filter
Teleport resources using CR labels.

### Terraform provider no longer forces resource re-creation on version change

Teleport v15 introduced two Terraform provider changes:
- setting the resource version is now mandatory
- a resource version change triggers the resource re-creation to ensure defaults
  were correctly set

The second change was too disruptive, especially for roles, as they cannot be
deleted if a user or an Access List references them. Teleport 15.2 lifts this
restriction and allows version change without forcing the resource deletion.

Another change to ensure resource defaults are correctly set during version
upgrades will happen in v16.

### Other improvements and fixes

* Fixed "Invalid URI" error in Teleport Connect when starting mongosh from database connection tab. [#40033](https://github.com/gravitational/teleport/pull/40033)
* Adds support for exporting the SPIFFE CA using `tls auth export --type tls-spiffe` and the `/webapi/auth/export` endpoint. [#40007](https://github.com/gravitational/teleport/pull/40007)
* Update Rust to 1.77.0, enable RDP font smoothing. [#39995](https://github.com/gravitational/teleport/pull/39995)
* The role, server and token Teleport operator CRs now display additional information when listed with `kubectl get`. [#39993](https://github.com/gravitational/teleport/pull/39993)
* Improve performance of filtering resources via predicate expressions. [#39972](https://github.com/gravitational/teleport/pull/39972)
* Fixes a bug that prevented CA import when a SPIFFE CA was present. [#39958](https://github.com/gravitational/teleport/pull/39958)
* Fix a verbosity issue that caused the `teleport-kube-agent-updater` to output debug logs by default. [#39953](https://github.com/gravitational/teleport/pull/39953)
* Reduce default Jamf inventory page size, allow custom values to be provided. [#39933](https://github.com/gravitational/teleport/pull/39933)
* AWS IAM Roles are now filterable in the web UI when launching a console app. [#39911](https://github.com/gravitational/teleport/pull/39911)
* The `teleport-cluster` Helm chart now supports using the Amazon Athena event backend. [#39907](https://github.com/gravitational/teleport/pull/39907)
* Correctly show the users allowed logins when accessing leaf resources via the root cluster web UI. [#39887](https://github.com/gravitational/teleport/pull/39887)
* Improve performance of resource filtering via labels and fuzzy search. [#39791](https://github.com/gravitational/teleport/pull/39791)
* Enforce optimistic locking for AuthPreferences, ClusterNetworkingConfig, SessionRecordingConfig. [#39785](https://github.com/gravitational/teleport/pull/39785)
* Fix potential issue with some resources expiry being set to 01/01/1970 instead of never. [#39773](https://github.com/gravitational/teleport/pull/39773)
* Update default Access Request TTLs to 1 week. [#39509](https://github.com/gravitational/teleport/pull/39509)
* Fixed an issue where creating or updating an Access List with Admin MFA would fail in the WebUI. [#3827](https://github.com/gravitational/teleport.e/pull/3827)


## 15.1.10 (03/27/24)

* Fixed possible phishing links which could result in code execution with install and join scripts. [#39837](https://github.com/gravitational/teleport/pull/39837)
* Fixed MFA checks not being prompted when joining a session. [#39814](https://github.com/gravitational/teleport/pull/39814)
* Added support for Kubernetes websocket streaming subprotocol v5 connections. [#39770](https://github.com/gravitational/teleport/pull/39770)
* Fixed a regression causing MFA prompts to not show up in Teleport Connect. [#39739](https://github.com/gravitational/teleport/pull/39739)
* Fixed broken SSO login landing page on certain versions of Google Chrome. [#39723](https://github.com/gravitational/teleport/pull/39723)
* Teleport Connect now shows specific error messages instead of generic "access denied". [#39720](https://github.com/gravitational/teleport/pull/39720)
* Added audit events for database auto user provisioning. [#39665](https://github.com/gravitational/teleport/pull/39665)
* Updated Electron to v29 in Teleport Connect. [#39657](https://github.com/gravitational/teleport/pull/39657)
* Added automatic Access Request support for `tsh db login`, `tsh db connect` and `tsh proxy db`. [#39617](https://github.com/gravitational/teleport/pull/39617)
* Fixed a bug in Teleport Enterprise (Cloud) causing the hosted ServiceNow plugin to crash when setting up the integration. [#39603](https://github.com/gravitational/teleport/pull/39603)
* Fixed a bug of the discovery script failing when `jq` was not installed. [#39599](https://github.com/gravitational/teleport/pull/39599)
* Ensured that audit events are emitted whenever the authentication preferences, cluster networking config, or session recording config are modified. [#39522](https://github.com/gravitational/teleport/pull/39522)
* Database object labels will now support templates. [#39496](https://github.com/gravitational/teleport/pull/39496)

## 15.1.9 (03/19/24)

* Improved performance when listing nodes with tsh or tctl. [#39567](https://github.com/gravitational/teleport/pull/39567)
* Require AWS S3 bucket fields when creating/editing AWS OIDC integration in the web UII. [#39510](https://github.com/gravitational/teleport/pull/39510)
* Added remote port forwarding to tsh. [#39441](https://github.com/gravitational/teleport/pull/39441)
* Added support for setting default relay state for SAML IdP initiated logins via the web interface and `tctl`. For supported preset service provider types, a default value will be applied if the field is not configured. [#39401](https://github.com/gravitational/teleport/pull/39401)

## 15.1.8 (03/18/24)

* Fixed an issue with AWS IAM permissions that may prevent AWS database access when discovery_service is enabled in the same Teleport config as the db_service, namely AWS RDS, Redshift, Elasticache, and MemoryDB. [#39488](https://github.com/gravitational/teleport/pull/39488)

## 15.1.7 (03/16/24)

* Added remote port forwarding for Teleport nodes. [#39440](https://github.com/gravitational/teleport/pull/39440)
* Added remote port forwarding for OpenSSH nodes. [#39438](https://github.com/gravitational/teleport/pull/39438)

## 15.1.5 (03/15/24)

* Improve error messaging when creating resources fails because they already exist or updating resources fails because they were removed. [#39395](https://github.com/gravitational/teleport/pull/39395)
* The audit entry for `access_request.search` will now truncate the list of roles in the audit UI if it exceeds 80 characters. [#39372](https://github.com/gravitational/teleport/pull/39372)
* Re-enable AWS IMDSv1 fallback due to some EKS clusters having their IMDSv2 hop limit set to `1`, leading to IMDSv2 requests failing. Users who wish to keep IMDSv1 fallback disabled can set the `AWS_EC2_METADATA_V1_DISABLED` environmental variable. [#39366](https://github.com/gravitational/teleport/pull/39366)
* Only allow necessary operations during moderated file transfers and limit in-flight file transfer requests to one per session. [#39351](https://github.com/gravitational/teleport/pull/39351)
* Make the Jira access plugin log Jira errors properly. [#39346](https://github.com/gravitational/teleport/pull/39346)
* Fixed allowing invalid Access Request start time date to be set. [#39322](https://github.com/gravitational/teleport/pull/39322)
* Teleport Enterprise now attempts to load the license file from the configured data directory if not otherwise specified. [#39314](https://github.com/gravitational/teleport/pull/39314)
* Improve the security for MFA for Admin Actions when used alongside Hardware Key support. [#39306](https://github.com/gravitational/teleport/pull/39306)
* The `saml_idp_service_provider` spec adds a new `preset` field that can be used to specify predefined SAML service provider profile. [#39277](https://github.com/gravitational/teleport/pull/39277)
* Fixed a bug that caused some MFA for Admin Action flows to fail instead of retrying: ex: `tctl bots add --token=<token>`. [#39269](https://github.com/gravitational/teleport/pull/39269)

## 15.1.4 (03/12/24)

* Raised concurrent connection limits between Teleport Enterprise (Cloud) regions and in clusters that use proxy peering. [#39233](https://github.com/gravitational/teleport/pull/39233)
* Improved cleanup of system resources during a shutdown of Teleport. [#39211](https://github.com/gravitational/teleport/pull/39211)
* Resolved sporadic errors caused by requests fail to comply with Kubernetes API spec by not specifying resource identifiers. [#39168](https://github.com/gravitational/teleport/pull/39168)
* Added a new password change wizard. [#39124](https://github.com/gravitational/teleport/pull/39124)
* Fixed the NumLock and Pause keys for desktop access sessions not working. [#39095](https://github.com/gravitational/teleport/pull/39095)

## 15.1.3 (03/08/24)

* Fix a bug when using automatic updates and the Discovery Service. The default install script now installs the correct teleport version by querying the version server. [#39099](https://github.com/gravitational/teleport/pull/39099)
* Fix a regression where `tsh kube credentials` fails to re-login when credentials expire. [#39075](https://github.com/gravitational/teleport/pull/39075)
* TBot now supports `--proxy-server` for explicitly configuring the Proxy address. We recommend switching to this if you currently specify the address of your Teleport proxy to `--auth-server`. [#39055](https://github.com/gravitational/teleport/pull/39055)
* Expand the EC2 joining process to include newly created AWS regions. [#39051](https://github.com/gravitational/teleport/pull/39051)
* Added GCP MySQL access IAM Authentication support. [#39040](https://github.com/gravitational/teleport/pull/39040)
* Fixed compatibility of the Teleport service file with older versions of systemd. [#39032](https://github.com/gravitational/teleport/pull/39032)
* Update WebUI database connection instructions. [#39027](https://github.com/gravitational/teleport/pull/39027)
* Teleport Proxy Service now runs a version server by default serving its own version. [#39017](https://github.com/gravitational/teleport/pull/39017)
* Significantly reduced latency of network calls in Teleport Connect. [#39012](https://github.com/gravitational/teleport/pull/39012)
* SPIFFE SVID generation introduced to tbot (experimental). [#39011](https://github.com/gravitational/teleport/pull/39011)
* Adds `tsh workload issue` command for issuing SVIDs using `tsh`. [#39115](https://github.com/gravitational/teleport/pull/39115)
* Fixed an issue in SAML IdP entity descriptor generator process, which would fail to generate entity descriptor if the configured Entity ID endpoint would return HTTP status code above `200` and below `400` . [#38987](https://github.com/gravitational/teleport/pull/38987)
* Updated Go to 1.21.8. [#38983](https://github.com/gravitational/teleport/pull/38983)
* Updated electron-builder dependency to address possible arbitrary code execution in the Windows installer of Teleport Connect  (CVE-2024-27303). [#38964](https://github.com/gravitational/teleport/pull/38964)
* Fixed an issue where it was possible to skip providing old password when setting a new one. [#38962](https://github.com/gravitational/teleport/pull/38962)
* Added database permission management support for Postgres. [#38945](https://github.com/gravitational/teleport/pull/38945)
* Improved reliability and performance of `tbot`. [#38928](https://github.com/gravitational/teleport/pull/38928)
* Filter terminated sessions from the `tsh sessions ls` output. [#38887](https://github.com/gravitational/teleport/pull/38887)
* Make it easier to identify Teleport browser tabs by placing the session information before the cluster name. [#38737](https://github.com/gravitational/teleport/pull/38737)
* The `teleport-ent-upgrader` package now gracefully restarts the Teleport binary if possible, to avoid cutting off ongoing connections. [#3578](https://github.com/gravitational/teleport.e/pull/3578)
* Trusted device authentication failures may now include a brief explanation message in the corresponding audit event. [#3572](https://github.com/gravitational/teleport.e/pull/3572)
* Okta Access Lists sync will now sync groups without members. [#3636](https://github.com/gravitational/teleport.e/pull/3636)

## 15.1.1 (03/01/24)

* Fixed panic when an older `tsh` or proxy changes an Access List. [#38861](https://github.com/gravitational/teleport/pull/38861)
* SSH connection resumption now works during graceful upgrades of the Teleport agent. [#38842](https://github.com/gravitational/teleport/pull/38842)
* Fixed an issue with over counting of reported Teleport updater metrics. [#38831](https://github.com/gravitational/teleport/pull/38831)
* Fixed `tsh` returning "private key policy not met" errors instead of automatically initiating re-login to satisfy the private key policy. [#38819](https://github.com/gravitational/teleport/pull/38819)
* Made graceful shutdown and graceful restart terminate active sessions after 30 hours. [#38803](https://github.com/gravitational/teleport/pull/38803)

## 15.1.0 (02/29/24)

### New Features

#### Standalone tbot Docker image
We now ship a new container image that contains tbot but omits other Teleport binaries, providing a light-weight option for Machine ID users.

#### Custom mouse pointers for remote desktop sessions
Teleport remote desktop sessions now automatically change the mouse cursor depending on context (when hovering over a link, resizing a window, or editing text, for example).

#### Synchronization of Okta groups and apps
Okta integration now support automatic synchronization of Okta groups and app assignments to Teleport as Access Lists giving users ability to request access to Okta apps without extra configuration.

#### EKS auto-discovery in Access Management UI
Users going through EKS enrollment flow in Access Management web UI now have an option to enable auto-discovery for EKS clusters.

### Other changes

* Fixed application access events being overwritten when using DynamoDB as event storage. [#38815](https://github.com/gravitational/teleport/pull/38815)
* Fixed a regression that had reintroduced long freezes for certain actions like "Run as different user". [#38805](https://github.com/gravitational/teleport/pull/38805)
* When teleport is configured to require MFA for admin actions, MFA is required to get certificate authority secrets. Ex: `tctl auth export --keys` or `tctl get cert_authority/host/root.example.com --with-secrets`. [#38777](https://github.com/gravitational/teleport/pull/38777)
* Added auto-enrolling capabilities to EKS discover flow in the web UI. [#38773](https://github.com/gravitational/teleport/pull/38773)
* Heavily optimized the Access List page in the UI, speeding things up considerably. [#38764](https://github.com/gravitational/teleport/pull/38764)
* Align DynamoDB BatchWriteItem max items limit. [#38763](https://github.com/gravitational/teleport/pull/38763)
* tbot-distroless image is now published. This contains just the tbot binary and therefore has a smaller image size. [#38718](https://github.com/gravitational/teleport/pull/38718)
* Fixed a regression with Teleport Connect not showing the re-login reason and connection errors when accessing databases, Kube clusters, and apps with an expired cert. [#38716](https://github.com/gravitational/teleport/pull/38716)
* Re-enabled the Windows key and prevents it from sticking or otherwise causing problems when cmd+tab-ing or alt+tab-ing away from the browser during desktop sessions. [#38699](https://github.com/gravitational/teleport/pull/38699)
* Resource limits are now correctly applied to the `wait-auth-update` initContainer in the `teleport-cluster` Helm chart. [#38692](https://github.com/gravitational/teleport/pull/38692)
* When teleport is configured to require MFA for admin actions, MFA is required to create, update, or delete trusted clusters. [#38690](https://github.com/gravitational/teleport/pull/38690)
* Fixed error in `tctl get users --with-secrets` when using SSO. [#38663](https://github.com/gravitational/teleport/pull/38663)
* When device trust is required and MFA is optional, users will need to add their first MFA device from a trusted device. [#38657](https://github.com/gravitational/teleport/pull/38657)
* Temporary files are no longer created during Discover UI EKS cluster enrollment. [#38649](https://github.com/gravitational/teleport/pull/38649)
* When teleport is configured to require MFA for admin actions, MFA is required to get or list tokens with `tctl`. Ex: `tctl tokens ls` or `tctl get tokens/foo`. [#38645](https://github.com/gravitational/teleport/pull/38645)
* Implemented dynamic mouse pointer updates to reflect context-specific actions, e.g. window resizing. [#38614](https://github.com/gravitational/teleport/pull/38614)
* MFA approval is no longer required in the beginning of EKS Discover flow. [#38580](https://github.com/gravitational/teleport/pull/38580)
* Fixed Postgres v16.x compatibility issue preventing multiple connections for auto-provisioned users. [#38543](https://github.com/gravitational/teleport/pull/38543)
* Fixed incorrect color of resource cards after changing the theme in Web UI and Connect. [#38537](https://github.com/gravitational/teleport/pull/38537)
* Updated the dialog for adding new authentication methods in the account settings screen. [#38535](https://github.com/gravitational/teleport/pull/38535)
* Displays review dates for Access Lists in dates, not remaining hours in tsh. [#38525](https://github.com/gravitational/teleport/pull/38525)
* Ensure that tsh continues to function if one of its profiles is invalid. [#38514](https://github.com/gravitational/teleport/pull/38514)
* Fixed logging output for `teleport configure ...` commands. [#38508](https://github.com/gravitational/teleport/pull/38508)
* Fixed tsh/WebAuthn.dll panic on Windows Server 2019. [#38490](https://github.com/gravitational/teleport/pull/38490)
* Fixes an issue that prevented the Web UI from properly displaying the hostname of servers in leaf clusters. [#38469](https://github.com/gravitational/teleport/pull/38469)
* Added `ssh_service.enhanced_recording.root_path` configuration option to change the cgroup slice path used by the agent. [#38394](https://github.com/gravitational/teleport/pull/38394)
* Fixed a bug that could cause expired SSH servers from appearing in the Web UI until the Proxy is restarted. [#38310](https://github.com/gravitational/teleport/pull/38310)
* Desktops can now be configured to use the same screen resolution for all sessions. [#38307](https://github.com/gravitational/teleport/pull/38307)
* The maximum duration for an Access Request is now 14 days, the okta-requester role has been added which takes advantage of this. [#38224](https://github.com/gravitational/teleport/pull/38224)
* Added TLS routing native WebSocket connection upgrade support. [#38108](https://github.com/gravitational/teleport/pull/38108)
* Fixed a bug allowing the operator to delete resource it does not own. [#37750](https://github.com/gravitational/teleport/pull/37750)

## 15.0.2 (02/15/24)

* Fixed a potential panic in the `tsh status` command. [#38305](https://github.com/gravitational/teleport/pull/38305)
* Fixed SSO user locking in the setup access step of the RDS auto discover flow in the web UI. [#38283](https://github.com/gravitational/teleport/pull/38283)
* Optionally permit the Auth Service to terminate client connections from unsupported versions. [#38182](https://github.com/gravitational/teleport/pull/38182)
* Fixed Assist obstructing the user dropdown menu when in docked mode. [#38156](https://github.com/gravitational/teleport/pull/38156)
* Improved the stability of Teleport during graceful upgrades. [#38145](https://github.com/gravitational/teleport/pull/38145)
* Added the ability to view and manage Machine ID bots from the UI. [#38122](https://github.com/gravitational/teleport/pull/38122)
* Fixed a bug that prevented desktop clipboard sharing from working when large amounts of text are placed on the clipboard. [#38120](https://github.com/gravitational/teleport/pull/38120)
* Added option to validate hardware key serial numbers with hardware key support. [#38068](https://github.com/gravitational/teleport/pull/38068)
* Removed access tokens from URL parameters, preventing them from being leaked to intermediary systems that may log them in plaintext. [#38032](https://github.com/gravitational/teleport/pull/38032)
* Forced agents to terminate Auth connections if joining fails. [#38005](https://github.com/gravitational/teleport/pull/38005)
* Added a tsh sessions ls command to list active sessions. [#37969](https://github.com/gravitational/teleport/pull/37969)
* Improved error handling when idle desktop connections are terminated. [#37955](https://github.com/gravitational/teleport/pull/37955)
* Updated Go to 1.21.7. [#37846](https://github.com/gravitational/teleport/pull/37846)
* Discover flow now starts two instances of DatabaseServices when setting up access to Amazon RDS. [#37805](https://github.com/gravitational/teleport/pull/37805)

## 15.0.1 (02/06/24)

* Correctly handle non-registered U2F keys. [#37720](https://github.com/gravitational/teleport/pull/37720)
* Fixed memory leak in tbot caused by never closing reverse tunnel address resolvers. [#37718](https://github.com/gravitational/teleport/pull/37718)
* Fixed conditional user modifications (used by certain Teleport subsystems such as Device Trust) on users that have previously been locked out due to repeated recovery attempts. [#37703](https://github.com/gravitational/teleport/pull/37703)
* Added okta integration SCIM support for web UI. [#37697](https://github.com/gravitational/teleport/pull/37697)
* Added SCIM support in Okta integration (cloud only). [#3341](https://github.com/gravitational/teleport.e/pull/3341)
* Fixed usage data submission becoming stuck sending too many reports at once (Teleport Enterprise only). [#37687](https://github.com/gravitational/teleport/pull/37687)
* Fixed cache init issue with Access List members/reviews. [#37673](https://github.com/gravitational/teleport/pull/37673)
* Fixed "failed to close stream" log messages. [#37662](https://github.com/gravitational/teleport/pull/37662)
* Skip tsh AppID pre-flight check whenever possible. [#37642](https://github.com/gravitational/teleport/pull/37642)

## 15.0.0 (01/31/24)

Teleport 15 brings the following new major features and improvements:

- Desktop access performance improvements
- Enhanced Device Trust support
- SSH connection resumption
- RDS auto-discovery in Access Management UI
- EKS Integration for Teleport
- MFA for Administrative Actions
- Improved SAML IdP configuration flow
- Improved provisioning for Okta
- Support for AWS KMS
- Teleport Connect improvements
- Session playback improvements
- Standalone Kubernetes Operator
- Roles v6 and v7 support for Kubernetes Operator
- Enhanced ARM64 builds

In addition, this release includes several changes that affect existing
functionality listed in the “Breaking changes” section below. Users are advised
to review them before upgrading.

### Description

#### Desktop access performance improvements

Teleport 15 leverages a new, more performant RDP engine, resulting in a smoother
desktop access experience.

#### Device Trust for Linux support

Teleport Device Trust now supports TPM joining on Linux devices.

Additionally, `tsh proxy app` can now solve device challenges, allowing users to
enforce the use of a trusted device to access applications.

#### SSH connection resumption

Teleport v15 introduces automatic SSH connection resumption if the network path
between the client and the Teleport node is interrupted due to connectivity
issues, and transparent connection migration if the control plane is gracefully
upgraded.

The feature is active by default when a v15 client (`tsh`, OpenSSH or PuTTY
configured by `tsh config`, or Teleport Connect) connects to a v15 Teleport
node.

#### RDS auto-discovery in Access Management UI

Users going through the Access Management UI flow to enroll RDS databases are
now able to set up auto-discovery.

#### EKS Integration for Teleport

Teleport now allows users to enroll EKS clusters via the Access Management UI.

#### Improved SAML IdP configuration flow

When adding a SAML application via Access Management UI, users are now able to
configure attribute mapping and have Teleport fetch service provider's entity
descriptor automatically.

#### Improved provisioning for Okta

Teleport 15 improves performance of receiving user/group updates from Okta by
leveraging System for Cross-domain Identity Management (SCIM).

Note: This feature will come out in a later 15.0 patch release.

#### Support for AWS KMS

Teleport 15 supports the use of AWS Key Management Service (KMS) to store and
handle the CA private key material used to sign all Teleport-issued
certificates. When enabled, private key material never leaves AWS KMS.

To migrate existing clusters to AWS KMS, you must perform a CA rotation.

#### MFA for administrative actions

When Teleport is configured to require webauthn (`second_factor: webauthn`),
administrative actions performed via `tctl` or the web UI will require an
additional MFA tap.

Examples of administrative actions include, but are not limited to:

- Resetting or recovering user accounts
- Inviting new users
- Updating cluster configuration resources
- Creating and approving Access Requests
- Generating new join tokens

Note: when MFA for administrative actions is enabled, user certificates produced
with `tctl auth sign` will no longer be suitable for automation due to the additional
MFA checks, unless run directly on a local Auth Service (legacy setup). We
recommend using Machine ID to issue certificates for automated workflows, which
uses role impersonation that is not subject to MFA checks.

#### Teleport Connect improvements

Teleport Connect will now prompt for an MFA tap prior to accessing Kubernetes
clusters when per-session MFA is enabled.

Additionally, Teleport Connect includes support for TCP and web applications,
and can also launch AWS and SAML apps in a web browser.

#### Session playback improvements

Prior to Teleport 15, `tsh play` and the web UI would download the entire
session recording before starting playback. As a result, playback of large
recordings could be slow to start, and may fail to play at all in the browser.

In Teleport 15, session recordings are streamed from the Auth Service, allowing
playback to start before the entire session is downloaded and unpacked.

Additionally, `tsh play` now supports a `--speed` flag for adjusting the
playback speed, and desktop session playback now supports seeking to arbitrary
positions in the recording.

#### Web UI improvements

Prior to Teleport 15, there was a dropdown in the sidebar between “Resources”
and “Management,” and in the Resources mode, there were tabs in the sidebar for
Access Requests and Active Sessions. In Teleport 15, all of the above have moved
to tabs in a top navbar, and the Resources view is fully responsive across
viewport widths. A side navbar still exists in the “Access Management” tab.

Prior to Teleport 15, Passkeys and MFA devices were shown in a single list on
the “Account Settings” screen, without a clear distinction between them. In
Teleport 15, these have been split into distinct lists so it is clearer which
type of authentication you are adding to your account.

#### Standalone Kubernetes Operator

Prior to Teleport 15, the Teleport Kubernetes Operator had to run as a sidecar
of the Teleport auth. It was not possible to use the operator in Teleport Enterprise (Cloud)
or against a Teleport cluster not deployed with the `teleport-cluster` Helm
chart.

In Teleport 15, the Teleport Operator can reconcile resources in any Teleport
cluster. Teleport Enterprise (Cloud) users can now use the operator to manage their
resources.

When deployed with the `teleport-cluster` chart, the operator now runs in a
separate pod. This ensures that Teleport's availability won't be impacted if the
operator becomes unready.

See [the Standalone Operator guide](docs/pages/admin-guides/infrastructure-as-code/teleport-operator/teleport-operator-standalone.mdx)
for installation instructions.

#### Roles v6 and v7 support for Kubernetes Operator

Starting with Teleport 15, newly supported kinds will contain the resource
version. For example: `TeleportRoleV6` and `TeleportRoleV7` kinds will allow
users to create Teleport Roles v6 and v7.

Existing kinds will remain unchanged in Teleport 15, but will be renamed in
Teleport 16 for consistency.

To migrate an existing Custom Resource (CR) `TeleportRole` to a
`TeleportRoleV7`, you must:
- upgrade Teleport and the operator to v15
- annotate the exiting `TeleportRole` CR with `teleport.dev/keep: "true"`
- delete the `TeleportRole` CR (it won't delete the role in Teleport thanks to
  the annotation)
- create a new `TeleportRoleV7` CR with the same name

#### Enhanced ARM64 builds

Teleport 15 now provides FIPS-compliant Linux builds on ARM64. Users will now be
able to run Teleport in FedRAMP/FIPS mode on ARM64.

Additionally, Teleport 15 includes hardened AWS AMIs for ARM64.

### Breaking changes and deprecations

#### RDP engine requires RemoteFX

Teleport 15 includes a new RDP engine that leverages the RemoteFX codec for
improved performance. Additional configuration may be required to enable
RemoteFX on your Windows hosts.

If you are using our authentication package for local users, the v15 installer
will automatically enable RemoteFX for you.

Alternatively, you can enable RemoteFX by updating the registry:

```powershell
Set-ItemProperty -Path 'HKLM:\Software\Policies\Microsoft\Windows NT\Terminal Services' -Name 'ColorDepth' -Type DWORD -Value 5
Set-ItemProperty -Path 'HKLM:\Software\Policies\Microsoft\Windows NT\Terminal Services' -Name 'fEnableVirtualizedGraphics' -Type DWORD -Value 1
```

If you are using Teleport with Windows hosts that are part of an Active
Directory environment, you should enable RemoteFX via group policy.

Under Computer Configuration > Administrative Templates > Windows Components >
Remote Desktop Services > Remote Desktop Session Host, enable:

1. Remote Session Environment > RemoteFX for Windows Server 2008 R2 > Configure
   RemoteFX
1. Remote Session Environment > Enable RemoteFX encoding for RemoteFX clients
   designed for Windows Server 2008 R2 SP1
1. Remote Session Environment > Limit maximum color depth

Detailed instructions are available in the
[setup guide](docs/pages/enroll-resources/desktop-access/active-directory.mdx#enable-remotefx).
A reboot may be required for these changes to take effect.

#### `tsh ssh`

When running a command on multiple nodes with `tsh ssh`, each line of output is
now labeled with the hostname of the node it was written by. Users that rely on
parsing the output from multiple nodes should pass the `--log-dir` flag to `tsh
ssh`, which will create a directory where the separated output of each node will
be written.

#### `drop` host user creation mode

The `drop` host user creation mode has been removed in Teleport 15. It is
replaced by `insecure-drop`, which still creates temporary users but does not
create a home directory. Users who need home directory creation should either
wrap `useradd`/`userdel` or use PAM.

#### Remove restricted sessions for SSH

The restricted session feature for SSH has been deprecated since Teleport 14 and
has been removed in Teleport 15. We recommend implementing network restrictions
outside of Teleport (iptables, security groups, etc).

#### Packages no longer published to legacy Debian and RPM repos

`deb.releases.teleport.dev` and `rpm.releases.teleport.dev` were deprecated in
Teleport 11. Beginning in Teleport 15, Debian and RPM packages will no longer be
published to these repos. Teleport 14 and prior packages will continue to be
published to these repos for the remainder of those releases' lifecycle.

All users are recommended to switch to `apt.releases.teleport.dev` and
`yum.releases.teleport.dev` repositories as described in installation
[instructions](docs/pages/installation.mdx).

The legacy package repos will be shut off in mid 2025 after Teleport 14 has been
out of support for many months.

#### Container images

Teleport 15 contains several breaking changes to improve the default security
and usability of Teleport-provided container images.

##### "Heavy" container images are discontinued

In order to increase default security in 15+, Teleport will no longer publish
[container images containing a shell and command line
environment](https://github.com/gravitational/teleport/blob/branch/v14/build.assets/charts/Dockerfile)
to Elastic Container Registry's
[gravitational/teleport](https://gallery.ecr.aws/gravitational/teleport) image
repo. Instead, all users should use the [distroless
images](https://github.com/gravitational/teleport/blob/branch/v15/build.assets/charts/Dockerfile-distroless)
introduced in Teleport 12. These images can be found at:

* https://gallery.ecr.aws/gravitational/teleport-distroless
* https://gallery.ecr.aws/gravitational/teleport-ent-distroless

For users who need a shell in a Teleport container, a "debug" image is available
which contains BusyBox, including a shell and many CLI tools. Find the debug
images at:

* https://gallery.ecr.aws/gravitational/teleport-distroless-debug
* https://gallery.ecr.aws/gravitational/teleport-ent-distroless-debug

Do not run debug container images in production environments.

Heavy container images will continue to be published for Teleport 13 and 14
throughout the remainder of these releases' lifecycle.

##### Helm cluster chart FIPS mode changes

The teleport-cluster chart no longer uses versionOverride and extraArgs to set FIPS mode. 

Instead, you should use the following values file configuration:
```
enterpriseImage: public.ecr.aws/gravitational/teleport-ent-fips-distroless
authentication:
  localAuth: false
```

##### Multi-architecture Teleport Operator images

Teleport Operator container images will no longer be published with architecture
suffixes in their tags (for example: `14.2.1-amd64` and `14.2.1-arm`). Instead,
only a single tag will be published with multi-platform support (e.g.,
`15.0.0`). If you use Teleport Operator images with an architecture suffix,
remove the suffix and your client should automatically pull the
platform-appropriate image. Individual architectures may be pulled with `docker
pull --platform <arch>`.

##### Quay.io registry

The quay.io container registry was deprecated and Teleport 12 is the last
version to publish images to quay.io. With Teleport 15's release, v12 is no
longer supported and no new container images will be published to quay.io.

For Teleport 8+, replacement container images can be found in [Teleport's public
ECR registry](https://gallery.ecr.aws/gravitational).

Users who wish to continue to use unsupported container images prior to Teleport
8 will need to download any quay.io images they depend on and mirror them
elsewhere before July 2024. Following brownouts in May and June, Teleport will
disable pulls from all Teleport quay.io repositories on Wednesday July 3, 2024.

#### Amazon AMIs

Teleport 15 contains several breaking changes to improve the default security
and usability of Teleport-provided Amazon AMIs.

##### Hardened AMIs

Teleport-provided Amazon Linux 2023 previously only supported x86_64/amd64.
Starting with Teleport 15, arm64-based AMIs will be produced. However, the
naming scheme for these AMIs has been changed to include the architecture.

- Previous naming scheme: `teleport-oss-14.0.0-$TIMESTAMP`
- New naming scheme: `teleport-oss-15.0.0-x86_64-$TIMESTAMP`

##### Legacy Amazon Linux 2 AMIs

Teleport-provided Amazon Linux 2 AMIs were deprecated, and Teleport 14 is the
last version to produce such legacy AMIs. With Teleport 15's release, only the
newer hardened Amazon Linux 2023 AMIs will be produced.

The legacy AMIs will continue to be published for Teleport 13 and 14 throughout
the remainder of these releases' lifecycle.

#### `windows_desktop_service` no longer writes to the NTAuth store

In Teleport 15, the process that periodically publishes Teleport's user CA to
the Windows NTAuth store has been removed. It is not necessary for Teleport to
perform this step since it must be done by an administrator at installation
time. As a result, Teleport's service account can use more restrictive
permissions.

#### Example AWS cluster deployments updated

The AWS terraform examples for Teleport clusters have been updated to use the
newer hardened Amazon Linux 2023 AMIs. Additionally, the default architecture
and instance type has been changed to ARM64/Graviton.

As a result of this modernization, the legacy monitoring stack configuration
used with the legacy AMIs has been removed.

##### `teleport-cluster` Helm chart changes

Due to the new separate operator deployment, the operator is deployed by a
subchart. This causes the following breaking changes:
- `installCRDs` has been replaced by `operator.installCRDs`
- `teleportVersionOverride` does not set the operator version anymore, you must
  use `operator.teleportVersionOverride` to override the operator version.

Note: version overrides are dangerous and not recommended. Each chart version is
designed to run a specific Teleport and operator version. If you want to deploy
a specific Teleport version, use Helm's `--version X.Y.Z` instead.

The operator now joins using a Kubernetes ServiceAccount token. To validate the
token, the Teleport Auth Service must have access to the `TokenReview` API. The
chart configures this for you since v12, unless you disabled `rbac` creation.

##### Helm cluster chart FIPS mode changes

The teleport-cluster chart no longer uses versionOverride and extraArgs to set FIPS mode. 

Instead, you should use the following values file configuration:

```
enterpriseImage: public.ecr.aws/gravitational/teleport-ent-fips-distroless
authentication:
  localAuth: false

```

#### Resource version is now mandatory and immutable in the Terraform provider

Starting with Teleport 15, each Terraform resource must have its version
specified. Before version 15, Terraform was picking the latest version available
on resource creation. This caused inconsistencies as new resources creates with
the same manifest as old resources were not exhibiting the same behavior.

Resource version is now immutable. Changing a resource version will cause
Terraform to delete and re-create the resource. This ensures the correct
defaults are set.

Existing resources will continue to work as Terraform already imported their
version. However, new resources will require an explicit version.

### Other changes

#### Increased password length

The minimum password length for local users has been increased from 6 to 12
characters.

#### Increased account lockout interval

The account lockout interval has been increased from 20 to 30 minutes.

## 14.3.21 (06/20/24)

* Fixed bug that caused gRPC connections to be disconnected when their certificate expired even though DisconnectCertExpiry was false. [#43292](https://github.com/gravitational/teleport/pull/43292)
* Fixed bug where a Teleport instance running only Jamf or Discovery service would never have a healthy  `/readyz` endpoint. [#43285](https://github.com/gravitational/teleport/pull/43285)
* Added a missing `[Install]` section to the `teleport-acm` systemd unit file as used by Teleport AMIs. [#43258](https://github.com/gravitational/teleport/pull/43258)
* Updated `teleport` to skip `jamf_service` validation when the Jamf is not enabled. [#43170](https://github.com/gravitational/teleport/pull/43170)
* Improved log rotation logic in Teleport Connect; now the non-numbered files always contain recent logs. [#43163](https://github.com/gravitational/teleport/pull/43163)
* Made tsh and Teleport Connect return early during login if ping to Proxy Service was not successful. [#43087](https://github.com/gravitational/teleport/pull/43087)
* Added ability to edit user traits from the Web UI. [#43070](https://github.com/gravitational/teleport/pull/43070)
* Enforce limits when reading events from Firestore to prevent OOM events. [#42968](https://github.com/gravitational/teleport/pull/42968)
* Fixed an issue Oracle access failed through trusted cluster. [#42929](https://github.com/gravitational/teleport/pull/42929)
* Fixes errors caused by `dynamoevents` query `StartKey` not being within the [From, To] window. [#42914](https://github.com/gravitational/teleport/pull/42914)
* Fixed updating groups for Teleport-created host users. [#42883](https://github.com/gravitational/teleport/pull/42883)
* Update azidentity to v1.6.0 (patches CVE-2024-35255). [#42860](https://github.com/gravitational/teleport/pull/42860)
* Remote rate limits on endpoints used extensively to connect to the cluster. [#42836](https://github.com/gravitational/teleport/pull/42836)
* Improved the performance of the Athena audit log and S3 session storage backends. [#42796](https://github.com/gravitational/teleport/pull/42796)
* Prevented a panic in the Proxy when accessing an offline application. [#42787](https://github.com/gravitational/teleport/pull/42787)
* Improve backoff of session recording uploads by teleport agents. [#42775](https://github.com/gravitational/teleport/pull/42775)
* Reduced backend writes incurred by tracking status of non-recorded sessions. [#42695](https://github.com/gravitational/teleport/pull/42695)
* Fixed listing available DB users in Teleport Connect for databases from leaf clusters obtained through Access Requests. [#42681](https://github.com/gravitational/teleport/pull/42681)
* Fixed not being able to logout from the web UI when session invalidation errors. [#42654](https://github.com/gravitational/teleport/pull/42654)
* Updated OpenSSL to 3.0.14. [#42643](https://github.com/gravitational/teleport/pull/42643)
* Teleport Connect binaries for Windows are now signed. [#42473](https://github.com/gravitational/teleport/pull/42473)
* Updated Go to 1.21.11. [#42416](https://github.com/gravitational/teleport/pull/42416)
* Fix web UI notification dropdown menu height from growing too long from many notifications. [#42338](https://github.com/gravitational/teleport/pull/42338)
* Disabled session recordings for non-interactive sessions when enhanced recording is disabled. [#42321](https://github.com/gravitational/teleport/pull/42321)
* Fixed issue where removing an app could make teleport app agents incorrectly report as unhealthy for a short time. [#42269](https://github.com/gravitational/teleport/pull/42269)
* Fixed a panic in the DynamoDB audit log backend when the cursor fell outside of the [From,To] interval. [#42266](https://github.com/gravitational/teleport/pull/42266)
* The `teleport configure` command now supports a `--node-name` flag for overriding the node's hostname. [#42249](https://github.com/gravitational/teleport/pull/42249)
* Fixed an issue where mix-and-match of join tokens could interfere with some services appearing correctly in heartbeats. [#42188](https://github.com/gravitational/teleport/pull/42188)
* Improved temporary disk space usage for session recording processing. [#42175](https://github.com/gravitational/teleport/pull/42175)
* Fixed a regression where Kubernetes Exec audit events were not properly populated and lacked error details. [#42146](https://github.com/gravitational/teleport/pull/42146)
* Fix Azure join method when using Resource Groups in the allow section. [#42140](https://github.com/gravitational/teleport/pull/42140)
* Fixed resource leak in session recording cleanup. [#42069](https://github.com/gravitational/teleport/pull/42069)
* Reduced memory and cpu usage after control plane restarts in clusters with a high number of roles. [#42064](https://github.com/gravitational/teleport/pull/42064)
* Fixed the field `allowed_https_hostnames` in the Teleport Operator resources: SAML, OIDC, and GitHub Connector. [#42056](https://github.com/gravitational/teleport/pull/42056)
* Enhanced error messaging for clients using `kubectl exec` v1.30+ to include warnings about a breaking change in Kubernetes. [#41989](https://github.com/gravitational/teleport/pull/41989)

### Enterprise-Only changes:
* Improved memory usage when reconciling Access Lists members to prevent Out of Memory events when reconciling a large number of Access Lists members.
* Prevented Access Monitoring reports from crashing when large datasets are returned.
* Ensured graceful restart of `teleport.service` after an upgrade.

## 14.3.20 (05/23/24)

This release contains fixes for several high-severity security issues, as well
as numerous other bug fixes and improvements.

### Security Fixes

#### **[High]** Unrestricted redirect in SSO Authentication

Teleport didn’t sufficiently validate the client redirect URL. This could allow
an attacker to trick Teleport users into performing an SSO authentication and
redirect to an attacker-controlled URL allowing them to steal the credentials.
[#41834](https://github.com/gravitational/teleport/pull/41834).

Warning: Teleport will now disallow non-localhost callback URLs for SSO logins
unless otherwise configured. Users of the `tsh login --callback` feature should
modify their auth connector configuration as follows:

```yaml
version: vX
kind: (saml|oidc|github)
metadata:
  name: ...
spec:
  ...
  client_redirect_settings:
    allowed_https_hostnames:
      - '*.app.github.dev'
      - '^\d+-[a-zA-Z0-9]+\.foo.internal$'
 ```

The `allowed_https_hostnames` field is an array containing allowed hostnames,
supporting glob matching and, if the string begins and ends with `^` and `$`
respectively, full regular expression syntax. Custom callback URLs are required
to be HTTPS on the standard port (443).

#### **[High]** CockroachDB authorization bypass

When connecting to CockroachDB using database access, Teleport did not properly
consider the username case when running RBAC checks. As such, it was possible to
establish a connection using an explicitly denied username when using a
different case. [#41823](https://github.com/gravitational/teleport/pull/41823).

#### **[High]** Long-lived connection persistence issue with expired certificates

Teleport did not terminate some long-running mTLS-authenticated connections past
the expiry of client certificates for users with the `disconnect_expired_cert`
option. This could allow such users to perform some API actions after their
certificate has expired.
[#41827](https://github.com/gravitational/teleport/pull/41827).

#### **[High]** PagerDuty integration privilege escalation

When creating a role Access Request, Teleport would include PagerDuty
annotations from the entire user’s role set rather than a specific role being
requested. For users who run multiple PagerDuty access plugins with
auto-approval, this could result in a request for a different role being
inadvertently auto-approved  than the one which corresponds to the user’s active
on-call schedule.
[#41837](https://github.com/gravitational/teleport/pull/41837).

#### **[High]** SAML IdP session privilege escalation

When using Teleport as SAML IdP, authorization wasn’t properly enforced on the
SAML IdP session creation. As such, authenticated users could use an internal
API to escalate their own privileges by crafting a malicious program.
[#41846](https://github.com/gravitational/teleport/pull/41846).

We strongly recommend all customers upgrade to the latest releases of Teleport.

### Other fixes and improvements

* Fixed session upload completion in situations where there's a large number of in-flight session uploads. [#41853](https://github.com/gravitational/teleport/pull/41853)
* Debug symbols are now stripped from Windows builds, resulting in smaller tsh and tctl binaries. [#41839](https://github.com/gravitational/teleport/pull/41839)
* Fixed an issue that the server version of the registered MySQL databases is not automatically updated upon new connections. [#41820](https://github.com/gravitational/teleport/pull/41820)
* Add read-only permissions for cluster maintenance config. [#41791](https://github.com/gravitational/teleport/pull/41791)
* Simplified how Bots are shown on the Users list page. [#41739](https://github.com/gravitational/teleport/pull/41739)
* Fix missing variable and script options in Default Agentless Installer script. [#41722](https://github.com/gravitational/teleport/pull/41722)
* Improved reliability of aggregated usage reporting with some cluster state storage backends (Teleport Enterprise only). [#41703](https://github.com/gravitational/teleport/pull/41703)
* Adds the remote address to audit log events emitted when a join for a Bot or Instance fails or succeeds. [#41699](https://github.com/gravitational/teleport/pull/41699)
* Allow the Application Service to heartbeat on behalf of more than 1000 dynamic applications. [#41627](https://github.com/gravitational/teleport/pull/41627)
* Ensure responses to Kubernetes watch requests are written sequentially. [#41625](https://github.com/gravitational/teleport/pull/41625)
* Install Script used in discover wizard now supports Ubuntu 24.04. [#41588](https://github.com/gravitational/teleport/pull/41588)
* Ensured that systemd always restarts Teleport on any failure unless explicitly stopped. [#41582](https://github.com/gravitational/teleport/pull/41582)
* Teleport service config is now reloaded on upgrades. [#41548](https://github.com/gravitational/teleport/pull/41548)
* Fix AccessList reconciler comparison causing audit events noise. [#41541](https://github.com/gravitational/teleport/pull/41541)
* Prevent SSH connections opened in the UI from leaking if the browser tab is closed while the SSH connection is being established. [#41519](https://github.com/gravitational/teleport/pull/41519)
* Emit login login failed audit events for invalid passwords on password+webauthn local authentication. [#41433](https://github.com/gravitational/teleport/pull/41433)
* Allow setting Kubernetes Cluster name when using non-default addresses. [#41355](https://github.com/gravitational/teleport/pull/41355)
* Added support to automatically download CA for MongoDB Atlas databases. [#41339](https://github.com/gravitational/teleport/pull/41339)
* Fix broken finish web page for SSO user's on auto discover. [#41336](https://github.com/gravitational/teleport/pull/41336)
* Add fallback on GetAccessList cache miss call. [#41327](https://github.com/gravitational/teleport/pull/41327)
* Validate application URL extracted from the web application launcher request route. [#41305](https://github.com/gravitational/teleport/pull/41305)
* Allow defining custom database names and users when selecting wildcard during test connection when enrolling a database through the web UI. [#41302](https://github.com/gravitational/teleport/pull/41302)
* Updated Go to v1.21.10. [#41282](https://github.com/gravitational/teleport/pull/41282)
* Forbid SSO users from local logins or password changes. [#41271](https://github.com/gravitational/teleport/pull/41271)
* Prevents Cloud tenants from updating `cluster_networking_config` fields `keep_alive_count_max`,  `keep_alive_interval`, `tunnel_strategy`, or `proxy_listener_mode`. [#41248](https://github.com/gravitational/teleport/pull/41248)

## 14.3.18 (05/07/24)

* Ensure that the active sessions page shows up in the web UI for users with permissions to join sessions. [#41222](https://github.com/gravitational/teleport/pull/41222)
* Fix a bug that was preventing tsh proxy kube certificate renewal from working when accessing a leaf kubernetes cluster via the root. [#41157](https://github.com/gravitational/teleport/pull/41157)
* Add lock target to lock deletion audit events. [#41111](https://github.com/gravitational/teleport/pull/41111)
* Improve the reliability of the upload completer. [#41104](https://github.com/gravitational/teleport/pull/41104)
* Allows the listener for the tbot database-tunnel service to be set to a unix socket. [#41042](https://github.com/gravitational/teleport/pull/41042)

## 14.3.17 (04/30/24)

* Fixed user SSO bypass by performing a local passwordless login. [#41071](https://github.com/gravitational/teleport/pull/41071)
* Enforce allow_passwordless server-side. [#41058](https://github.com/gravitational/teleport/pull/41058)
* Fixed a memory leak caused by incorrectly passing the offset when paginating all Access Lists' members when there are more than the default pagesize (200) Access Lists. [#41044](https://github.com/gravitational/teleport/pull/41044)
* Fixed a regression causing roles filtering to not work. [#41000](https://github.com/gravitational/teleport/pull/41000)
* Allow AWS integration to be used for global services without specifying a valid region. [#40990](https://github.com/gravitational/teleport/pull/40990)
* Fixed Access Requests lingering in the UI and tctl after expiry. [#40965](https://github.com/gravitational/teleport/pull/40965)
* Made `podSecurityContext` configurable in the `teleport-cluster` Helm chart. [#40950](https://github.com/gravitational/teleport/pull/40950)
* Allow mounting extra volumes in the updater pod deployed by the `teleport-kube-agent`chart. [#40949](https://github.com/gravitational/teleport/pull/40949)
* Improved error message when performing an SSO login with a hardware key. [#40924](https://github.com/gravitational/teleport/pull/40924)
* Fixed a bug in the `teleport-cluster` Helm chart that happened when `sessionRecording` was `off`. [#40920](https://github.com/gravitational/teleport/pull/40920)
* Allows setting additional Kubernetes labels on resources created by the `teleport-cluster` Helm chart. [#40916](https://github.com/gravitational/teleport/pull/40916)
* Fixed audit event failures when using DynamoDB event storage. [#40912](https://github.com/gravitational/teleport/pull/40912)
* Properly enforce session moderation requirements when starting Kubernetes ephemeral containers. [#40907](https://github.com/gravitational/teleport/pull/40907)
* Introduced the tpm join method, which allows for secure joining in on-prem environments without the need for a shared secret. [#40875](https://github.com/gravitational/teleport/pull/40875)
* Issue cert.create events during device authentication. [#40873](https://github.com/gravitational/teleport/pull/40873)
* Add the ability to control `ssh_config` generation in Machine ID's Identity Outputs. This allows the generation of the `ssh_config` to be disabled if unnecessary, improving performance and removing the dependency on the Proxy being online. [#40862](https://github.com/gravitational/teleport/pull/40862)
* Prevented deleting AWS OIDC integration used by External Audit Storage. [#40853](https://github.com/gravitational/teleport/pull/40853)
* Reduced parallelism when polling AWS resources to prevent API throttling when exporting them to Teleport Access Graph. [#40812](https://github.com/gravitational/teleport/pull/40812)
* Added hardware key support for agentless connections [#40929](https://github.com/gravitational/teleport/pull/40929)

## 14.3.16 (04/23/24)

* Fixed a deprecation warning being shown when `tbot` is used with OpenSSH. [#40838](https://github.com/gravitational/teleport/pull/40838)
* Added a new Audit log event that is emitted when an Agent or Bot request to join the cluster is denied. [#40815](https://github.com/gravitational/teleport/pull/40815)
* Added a new Prometheus metric to track requests initiated by Teleport against the control plane API. [#40755](https://github.com/gravitational/teleport/pull/40755)
* Fixed uploading zip files larger than 10MiB when updating an AWS Lambda function via tsh app access. [#40738](https://github.com/gravitational/teleport/pull/40738)
* Fixed possible data race that could lead to concurrent map read and map write while proxying Kubernetes requests. [#40721](https://github.com/gravitational/teleport/pull/40721)
* Fixed Access Request promotion of windows_desktop resources. [#40711](https://github.com/gravitational/teleport/pull/40711)
* Fixed spurious ambiguous host errors in ssh routing. [#40709](https://github.com/gravitational/teleport/pull/40709)
* Patched CVE-2023-45288 and CVE-2024-32473. [#40696](https://github.com/gravitational/teleport/pull/40696)
* Generic "not found" errors are returned whether a remote cluster can't be found or access is denied. [#40682](https://github.com/gravitational/teleport/pull/40682)
* Fixed a resource leak in the Teleport proxy server when using proxy peering. [#40675](https://github.com/gravitational/teleport/pull/40675)
* Allow other issue types when configuring JIRA plugin. [#40645](https://github.com/gravitational/teleport/pull/40645)
* Added the ability to configure labels that should be set on the Kubernetes secret when using the `kubernetes_secret` destination in `tbot`. [#40551](https://github.com/gravitational/teleport/pull/40551)
* Updated cosign to address CVE-2024-29902 and CVE-2024-29903. [#40498](https://github.com/gravitational/teleport/pull/40498)
* The Web UI now supports large number of roles by paginating them. [#40464](https://github.com/gravitational/teleport/pull/40464)

## 14.3.15 (04/12/24)

* Fixed accidental passkey "downgrades" to MFA. [#40410](https://github.com/gravitational/teleport/pull/40410)
* Added `tsh proxy kube --exec` mode that spawns kube proxy in the background, which re-executes the user shell with the appropriate kubeconfig. [#40394](https://github.com/gravitational/teleport/pull/40394)
* Made Amazon S3 fields optional when creating or editing AWS OIDC integration on the web UI. [#40372](https://github.com/gravitational/teleport/pull/40372)
* Changed Teleport Connect to hide cluster name in the connection list if there is only a single cluster available. [#40357](https://github.com/gravitational/teleport/pull/40357)
* Changed Teleport Connect to now show all recent connections instead of capping them at 10. [#40251](https://github.com/gravitational/teleport/pull/40251)
* Fixed an issue that prevents the Teleport service from restarting. [#40230](https://github.com/gravitational/teleport/pull/40230)
* Added a new resource filtering predicates to allow exact matches on a single item of a delimited list stored in a label value. For example, if given the following label containing a string separated list of values `foo=bar,baz,bang`, it is now possible to match on any resources with a label `foo` that contains the element `bar` via `contains(split(labels[foo], ","), bar)`. [#40184](https://github.com/gravitational/teleport/pull/40184)
* Updated Go to 1.21.9. [#40177](https://github.com/gravitational/teleport/pull/40177)
* Added `disable_exec_plugin` option to the Machine ID Kubernetes Output to remove the dependency on `tbot` existing in the target environment. [#40163](https://github.com/gravitational/teleport/pull/40163)
* Added the database-tunnel service to `tbot` which allows an authenticated database tunnel to be opened by `tbot`. This is an improvement over the original technique of using `tbot proxy db`. [#40160](https://github.com/gravitational/teleport/pull/40160)
* Enabled diagnostic endpoints access behind a PROXY protocol enabled loadbalancer/proxy. [#40139](https://github.com/gravitational/teleport/pull/40139)
* Added system annotations to audit event entries for Access Requests. [#40122](https://github.com/gravitational/teleport/pull/40122)
* Fixed "Invalid URI" error in Teleport Connect when starting MongoDB `mongosh` from the database connection tab. [#40105](https://github.com/gravitational/teleport/pull/40105)
* Improved the performance of filtering resources via predicate expressions. [#39975](https://github.com/gravitational/teleport/pull/39975)
* Fixed a verbosity issue that caused the `teleport-kube-agent-updater` to output debug logs by default. [#39954](https://github.com/gravitational/teleport/pull/39954)
* Reduced default Jamf inventory page size, and added support for custom values. [#39934](https://github.com/gravitational/teleport/pull/39934)
* Added support to the `teleport-cluster` Helm chart for using an Amazon Athena event backend. [#39908](https://github.com/gravitational/teleport/pull/39908)
* Improved the performance of resource filtering via labels and fuzzy search. [#39792](https://github.com/gravitational/teleport/pull/39792)

## 14.3.14 (03/27/24)

* Fixed possible phishing links which could result in code execution with install and join scripts. [#39838](https://github.com/gravitational/teleport/pull/39838)
* Fixed MFA checks not being prompted when joining a session. [#39815](https://github.com/gravitational/teleport/pull/39815)
* Fixed potential issue with some resources expiry being set to 01/01/1970 instead of never. [#39774](https://github.com/gravitational/teleport/pull/39774)
* Added support for Kubernetes websocket streaming subprotocol v5 connections. [#39771](https://github.com/gravitational/teleport/pull/39771)
* Fixed broken SSO login landing page on certain versions of Google Chrome. [#39722](https://github.com/gravitational/teleport/pull/39722)
* Updated Electron to v29 in Teleport Connect. [#39658](https://github.com/gravitational/teleport/pull/39658)
* Fixed a bug in Teleport Enterprise (Cloud) causing the hosted ServiceNow plugin to crash when setting up the integration. [#39604](https://github.com/gravitational/teleport/pull/39604)
* Fixed Teleport updater metrics for AWS OIDC deployments. [#39531](https://github.com/gravitational/teleport/pull/39531)
* Fixed allowing invalid Access Request start time date to be set. [#39324](https://github.com/gravitational/teleport/pull/39324)

## 14.3.13 (03/20/24)

* Fixed the discovery script failing when `jq` was not installed. [#39600](https://github.com/gravitational/teleport/pull/39600)
* Improve performance when listing nodes with tsh or tctl. [#39568](https://github.com/gravitational/teleport/pull/39568)
* Require AWS S3 bucket fields when creating/editing AWS OIDC integration in the web UI. [#39513](https://github.com/gravitational/teleport/pull/39513)
* Removed implicit AccessList membership and ownership modes. All AccessList owners and members must be explicitly specified. [#39388](https://github.com/gravitational/teleport/pull/39388)

## 14.3.11 (03/18/24)

* Fixed an issue with AWS IAM permissions that may prevent AWS database access when discovery_service is enabled in the same Teleport config as the db_service, namely AWS RDS, Redshift, Elasticache, and MemoryDB. [#39487](https://github.com/gravitational/teleport/pull/39487)

## 14.3.10 (03/16/24)

* Fixed issue with Teleport Auth Service panicking when Access Graph is enabled in Discovery Service. [#39456](https://github.com/gravitational/teleport/pull/39456)

## 14.3.8 (03/15/24)

* Improve error messaging when creating resources fails because they already exist or updating resources fails because they were removed. [#39396](https://github.com/gravitational/teleport/pull/39396)
* Support logging in with an identity file with `tsh login -i identity.pem`. This allows running `tsh app login` in CI environments where MachineID is impossible. [#39374](https://github.com/gravitational/teleport/pull/39374)
* Only allow necessary operations during moderated file transfers and limit in-flight file transfer requests to one per session. [#39352](https://github.com/gravitational/teleport/pull/39352)
* Make the Jira access plugin log Jira errors properly. [#39347](https://github.com/gravitational/teleport/pull/39347)
* Teleport Enterprise now attempts to load the license file from the configured data directory if not otherwise specified. [#39313](https://github.com/gravitational/teleport/pull/39313)
* Patched CVE-2024-27304 (Postgres driver). [#39259](https://github.com/gravitational/teleport/pull/39259)
* Raised concurrent connection limits between Teleport Enterprise (Cloud) regions and in clusters that use proxy peering. [#39232](https://github.com/gravitational/teleport/pull/39232)
* Improved cleanup of system resources during a shutdown of Teleport. [#39213](https://github.com/gravitational/teleport/pull/39213)
* Fixed an issue where it was possible to skip providing old password when setting a new one. [#39126](https://github.com/gravitational/teleport/pull/39126)

## 14.3.7 (03/11/24)

* Resolved sporadic errors caused by requests fail to comply with Kubernetes API spec by not specifying resource identifiers. [#39167](https://github.com/gravitational/teleport/pull/39167)
* Fixed a bug when using automatic updates and the Discovery Service. The default install script now installs the correct Teleport version by querying the version server. [#39100](https://github.com/gravitational/teleport/pull/39100)
* Teleport Proxy Service now runs a version server by default serving its own version. [#39096](https://github.com/gravitational/teleport/pull/39096)
* Fixed a regression where `tsh kube credentials` fails to re-login when credentials expire. [#39074](https://github.com/gravitational/teleport/pull/39074)
* TBot now supports `--proxy-server` for explicitly configuring the Proxy address. We recommend switching to this if you currently specify the address of your Teleport proxy to `--auth-server`. [#39056](https://github.com/gravitational/teleport/pull/39056)
* Expanded the EC2 joining process to include newly created AWS regions. [#39052](https://github.com/gravitational/teleport/pull/39052)
* Added GCP MySQL access IAM Authentication support. [#39041](https://github.com/gravitational/teleport/pull/39041)
* Fixed an issue in SAML IdP entity descriptor generator process, which would fail to generate entity descriptor if the configured Entity ID endpoint would return HTTP status code above `200` and below `400`. [#38988](https://github.com/gravitational/teleport/pull/38988)
* Updated Go to 1.21.8. [#38985](https://github.com/gravitational/teleport/pull/38985)
* Updated electron-builder dependency to address possible arbitrary code execution in the Windows installer of Teleport Connect (CVE-2024-27303). [#38966](https://github.com/gravitational/teleport/pull/38966)
* Improved reliability and performance of `tbot`. [#38929](https://github.com/gravitational/teleport/pull/38929)
* Filtered terminated sessions from the `tsh sessions ls` output. [#38886](https://github.com/gravitational/teleport/pull/38886)
* Prevented panic when AccessList's status field is not set. [#38862](https://github.com/gravitational/teleport/pull/38862)
* Fixed an issue with over counting of reported Teleport updater metrics. [#38832](https://github.com/gravitational/teleport/pull/38832)
* Fixed a bug that caused `tsh` to return "private key policy not met" errors instead of automatically initiating re-login to satisfy the private key policy. [#38818](https://github.com/gravitational/teleport/pull/38818)
* Fixed application access events being overwritten when using DynamoDB as event storage. [#38816](https://github.com/gravitational/teleport/pull/38816)
* Fixed issue where DynamoDB writes could fail when recording too many records. [#38762](https://github.com/gravitational/teleport/pull/38762)
* Added a tbot-only `tbot-distroless` container image, bringing an 80% size reduction over the Teleport `teleport` image. [#38719](https://github.com/gravitational/teleport/pull/38719)
* Fixed a Postgres v16.x compatibility issue preventing multiple connections for auto-provisioned users. [#38542](https://github.com/gravitational/teleport/pull/38542)
* Tsh will now show Access List review deadlines in dates rather than remaining hours.. [#38526](https://github.com/gravitational/teleport/pull/38526)
* Fixed an issue where tsh would not function if one of its profiles is invalid. [#38513](https://github.com/gravitational/teleport/pull/38513)
* Fixed an issue where `teleport configure` command logs would not use the configured logger. [#38509](https://github.com/gravitational/teleport/pull/38509)
* Removed `telnet` from legacy Ubuntu images due to CVE-2021-40491. Netcat `nc` can be used instead. [#38506](https://github.com/gravitational/teleport/pull/38506)
* Fixed a tsh WebAuthn.dll panic on Windows Server 2019. [#38489](https://github.com/gravitational/teleport/pull/38489)
* Added `ssh_service.enhanced_recording.root_path` configuration option to change the cgroup slice path used by the agent. [#38395](https://github.com/gravitational/teleport/pull/38395)
* Fixed a bug which allowed the operator to delete resources it does not own. [#37751](https://github.com/gravitational/teleport/pull/37751)

## 14.3.6 (02/16/24)

* Fixed a potential panic in the `tsh status` command. [#38304](https://github.com/gravitational/teleport/pull/38304)
* Fixed locking SSO user in the setup access step of the RDS auto discover flow in the web UI. [#38284](https://github.com/gravitational/teleport/pull/38284)
* Optionally permit the Auth Service to terminate client connections from unsupported versions. [#38186](https://github.com/gravitational/teleport/pull/38186)
* Removed access tokens from URL parameters, preventing them from being leaked to intermediary systems that may log them in plaintext. [#38070](https://github.com/gravitational/teleport/pull/38070)
* Added option to validate hardware key serial numbers with hardware key support. [#38069](https://github.com/gravitational/teleport/pull/38069)
* Forced agents to terminate Auth connections if joining fails. [#38004](https://github.com/gravitational/teleport/pull/38004)
* Added a tsh sessions ls command to list active sessions. [#37970](https://github.com/gravitational/teleport/pull/37970)
* Improved error handling when idle desktop connections are terminated. [#37956](https://github.com/gravitational/teleport/pull/37956)
* Updated Go to 1.21.7. [#37848](https://github.com/gravitational/teleport/pull/37848)
* Discover flow now starts two instances of DatabaseServices when setting up access to Amazon RDS. [#37804](https://github.com/gravitational/teleport/pull/37804)
* Fixed incorrect resizing of CLI apps in Teleport Connect on Windows. [#37799](https://github.com/gravitational/teleport/pull/37799)
* Fixed handling of non-registered U2F keys. [#37722](https://github.com/gravitational/teleport/pull/37722)
* Fixed memory leak in tbot caused by never closing reverse tunnel address resolvers. [#37719](https://github.com/gravitational/teleport/pull/37719)
* Fixed app redirection loop on browser's incognito mode and 3rd party cookie block. [#37692](https://github.com/gravitational/teleport/pull/37692)

## 14.3.4 (02/01/24)

* Skip `tsh` AppID pre-flight check whenever possible. [#37643](https://github.com/gravitational/teleport/pull/37643)
* Update OpenSSL to `3.0.13`. [#37552](https://github.com/gravitational/teleport/pull/37552)
* `tsh` FIDO2 backend re-written for improved responsiveness and reliability. [#37538](https://github.com/gravitational/teleport/pull/37538)
* Do not add alphabetically first Kube cluster's name to a user certificate on login. [#37501](https://github.com/gravitational/teleport/pull/37501)
* Allow to replicate proxy pods when using an ingress in the `teleport-cluster` Helm chart. [#37480](https://github.com/gravitational/teleport/pull/37480)
* Fix an issue `tsh` uses wrong default username for auto-user provisioning enabled databases in remote clusters [#37418](https://github.com/gravitational/teleport/pull/37418)
* Prevent backend throttling caused by a large number of app sessions. [#37391](https://github.com/gravitational/teleport/pull/37391)
* Emit audit events when SFTP or SCP commands are blocked. [#37385](https://github.com/gravitational/teleport/pull/37385)
* Fix goroutine leak on PostgreSQL access. [#37342](https://github.com/gravitational/teleport/pull/37342)
* Fixed incompatibility between leaf clusters and ProxyJump. [#37319](https://github.com/gravitational/teleport/pull/37319)
* Fixed a potential crash when setting up the Connect My Computer role in Teleport Connect. [#37314](https://github.com/gravitational/teleport/pull/37314)
* Fixed CA key generation when two auth servers share a single YubiHSM2. [#37296](https://github.com/gravitational/teleport/pull/37296)
* Add support for cancelling CockroachDB requests. [#37282](https://github.com/gravitational/teleport/pull/37282)
* Fix Terraform provider creating AccessLists with next audit date set to Epoch. [#37262](https://github.com/gravitational/teleport/pull/37262)
* Fix an issue selecting MySQL database is not reflected in the audit logs. [#37257](https://github.com/gravitational/teleport/pull/37257)
* The login screen will no longer be rendered for authenticated users. [#37230](https://github.com/gravitational/teleport/pull/37230)
* Fixed missing proxy address in GCP and Azure VM auto-discovery. [#37215](https://github.com/gravitational/teleport/pull/37215)
* Teleport namespace label prefixes are now sorted toward the end of the labels list in the web UI. [#37191](https://github.com/gravitational/teleport/pull/37191)
* Adds `tbot proxy kube` to support connecting to Kubernetes clusters using Machine ID when the Proxy is behind a L7 LB. [#37157](https://github.com/gravitational/teleport/pull/37157)
* Fix a bug that was breaking web UI if automatic upgrades are misconfigured. [#37130](https://github.com/gravitational/teleport/pull/37130)
* Fix an issue Amazon Redshift auto-provisioned user not deleted in drop mode. [#37036](https://github.com/gravitational/teleport/pull/37036)
* Fix an issue database auto-user provisioning fails to connect a second session on MariaDB older than 10.7. [#37028](https://github.com/gravitational/teleport/pull/37028)
* Improved styling of the login form in Connect and Web UI. [#37003](https://github.com/gravitational/teleport/pull/37003)
* Ensure that moderated sessions do not get stuck in the event of an unexpected drop in the moderator's connection. [#36917](https://github.com/gravitational/teleport/pull/36917)
* The web terminal now properly displays underscores on Linux. [#36890](https://github.com/gravitational/teleport/pull/36890)
* Fix `tsh` panic on Windows if `WebAuthn.dll` is missing. [#36868](https://github.com/gravitational/teleport/pull/36868)
* Increased timeout when waiting for response from Jira API and webhook to reconcile. [#36818](https://github.com/gravitational/teleport/pull/36818)
* Ensure `connect_to_node_attempts_total` is always incremented when dialing hosts. [#36739](https://github.com/gravitational/teleport/pull/36739)
* Fixed a potential crash in Teleport Connect after downgrading the app from v15+. [#36730](https://github.com/gravitational/teleport/pull/36730)
* Prevent a goroutine leak caused by app sessions not cleaning up resources properly. [#36668](https://github.com/gravitational/teleport/pull/36668)
* Added `tctl idp saml test-attribute-mapping` command to test SAML IdP attribute mapping. [#36662](https://github.com/gravitational/teleport/pull/36662)
* Fixed an issue where valid SAML entity descriptors could be rejected. [#36485](https://github.com/gravitational/teleport/pull/36485)
* Updated SAML IdP UI to display entity ID, SSO URL and X.509 certificate. [#3322](https://github.com/gravitational/teleport.e/pull/3322)
* Updated Access Request creation dialog to pre-select suggested reviewers. [#3325](https://github.com/gravitational/teleport.e/pull/3325)

## 14.3.3 (01/12/24)

* Fixed routing to nodes by their public addresses. [#36624](https://github.com/gravitational/teleport/pull/36624)
* Enhanced Kubernetes app discovery functionality to provide the ability to disable specific Service imports and configure the TLS Skip Verify option using an annotation. [#36611](https://github.com/gravitational/teleport/pull/36611)
* Added client remote IP address to some administrative audit events. [#36567](https://github.com/gravitational/teleport/pull/36567)

## 14.3.2 (01/11/24)

* Fixed routing to nodes by their public address. [#36591](https://github.com/gravitational/teleport/pull/36591)
* Verify MFA device locks during user authentication. [#36589](https://github.com/gravitational/teleport/pull/36589)
* Fixed `tctl get access_list` and support creating Access Lists without a next audit date. [#36572](https://github.com/gravitational/teleport/pull/36572)

## 14.3.1 (01/10/24)

* Added support to select database roles from `tsh`. [#36528](https://github.com/gravitational/teleport/pull/36528)
* Fixed goroutine leak per ssh session. [#36511](https://github.com/gravitational/teleport/pull/36511)
* Fixed user invites preventing listing tokens. [#36492](https://github.com/gravitational/teleport/pull/36492)
* Updated Go to v1.21.6. [#36478](https://github.com/gravitational/teleport/pull/36478)
* Fixed `refresh_identity = true` preventing Access Plugins connecting to Teleport using TLS routing with a L7 LB. [#36469](https://github.com/gravitational/teleport/pull/36469)
* Added --callback flag to tsh login. [#36468](https://github.com/gravitational/teleport/pull/36468)
* Added auto-enrolling capabilities to RDS discover flow in the web UI. [#36434](https://github.com/gravitational/teleport/pull/36434)
* Fixed an issue where bad cache state could cause spurious access denied errors during app access. [#36432](https://github.com/gravitational/teleport/pull/36432)
* Resources named `.` and `..` are no longer allowed. Please review the resources in your Teleport instance and rename any resources with these names before upgrading. [#36404](https://github.com/gravitational/teleport/pull/36404)
* Ensured that the login time is populated for app sessions. [#36373](https://github.com/gravitational/teleport/pull/36373)
* Fixed incorrect report of user's IP address in Kubernetes Audit Logs. [#36346](https://github.com/gravitational/teleport/pull/36346)
* Access lists and associated resources are now cached, which should significantly reduce the impact of Access List calculation. [#36331](https://github.com/gravitational/teleport/pull/36331)
* Added new certificate extensions and usage reporting flags to explicitly identify Machine ID bots and their cluster activity. [#36313](https://github.com/gravitational/teleport/pull/36313)
* Fixed potential panic after backend watcher failure. [#36301](https://github.com/gravitational/teleport/pull/36301)
* Prevent deleted users from using account reset links created prior to the user being deleted. [#36271](https://github.com/gravitational/teleport/pull/36271)
* Make Unified Resources page in Web UI responsive. [#36265](https://github.com/gravitational/teleport/pull/36265)
* Added "Database Roles" column to `tsh db ls -v`. [#36246](https://github.com/gravitational/teleport/pull/36246)
* Safeguard against the disruption of cluster access caused by incorrect Kubernetes APIService configurations. [#36227](https://github.com/gravitational/teleport/pull/36227)
* Support running a version server in the proxy for automatic agent upgrades. [#36220](https://github.com/gravitational/teleport/pull/36220)
* The user login state generator now uses the cache, which should reduce the number of calls to the backend. [#36196](https://github.com/gravitational/teleport/pull/36196)
* Added the `--insecure-no-resolve-image` flag to the `teleport-kube-agent-updater` to disable image tag resolution if it cannot pull the image. [#36097](https://github.com/gravitational/teleport/pull/36097)
* Added future assume time to Access Requests. [#35726](https://github.com/gravitational/teleport/pull/35726)

## 14.3.0 

This release of Teleport contains multiple security fixes, improvements and bug fixes.

### Security fixes

* Teleport Proxy now restricts SFTP for normal users as described under Advisory
  https://github.com/gravitational/teleport/security/advisories/GHSA-c9v7-wmwj-vf6x
  [#36139](https://github.com/gravitational/teleport/pull/36139)
* Fixed an issue that would allow for SSRF via Teleport's reverse tunnel
  subsystem. Documented under the advisory
  https://github.com/gravitational/teleport/security/advisories/GHSA-hw4x-mcx5-9q36
  [#36131](https://github.com/gravitational/teleport/pull/36131)
* On macOS, Teleport filters the environment to prevent code execution via
  `DYLD_` variables. Documented under
  https://github.com/gravitational/teleport/security/advisories/GHSA-vfxf-76hv-v4w4
  [#36135](https://github.com/gravitational/teleport/pull/36135)
* A fix was applied to Access Lists to prevent possible privilege escalation of
  list owners.  Documented under 
  https://github.com/gravitational/teleport/security/advisories/GHSA-76cc-p55w-63g3

### Other Fixes & Improvements

* Added the ability to promote an Access Request to an Access List in Teleport Connect
* Fixed an issue that would prevent websocket upgrades from completing. [#36088](https://github.com/gravitational/teleport/pull/36088)
* Enhanced the audit events related to Teleport's SAML IdP [#36087](https://github.com/gravitational/teleport/pull/36087)
* Added support for STS session tags in the database configuration for granular DynamoDB access. [#36064](https://github.com/gravitational/teleport/pull/36064)
* Added support for the IAM join method in ca-west-1. [#36049](https://github.com/gravitational/teleport/pull/36049)
* Improved the formatting of Access List notifications in tsh. [#36046](https://github.com/gravitational/teleport/pull/36046)
* Fixed downgrade logic of KubernetesResources to Role v6 [#36009](https://github.com/gravitational/teleport/pull/36009)
* Fixed potential panic during early phases of SSH service lifetime [#35923](https://github.com/gravitational/teleport/pull/35923)
* Added a `tsh latency` command to monitor ssh connection latency in realtime [#35916](https://github.com/gravitational/teleport/pull/35916)
* Support GitHub joining from Enterprise accounts with `include_enterprise_slug` enabled. [#35900](https://github.com/gravitational/teleport/pull/35900)
* Added vpc-id as a label to auto-discovered RDS databases [#35890](https://github.com/gravitational/teleport/pull/35890)
* Improved teleport agent performance when handling a large number of TCP forwarding requests. [#35887](https://github.com/gravitational/teleport/pull/35887)
* Bump golang.org/x/crypto to v0.17.0, which addresses the Terrapin vulnerability (CVE-2023-48795) [#35879](https://github.com/gravitational/teleport/pull/35879)
* Include the lock expiration time in `lock.create` audit events [#35874](https://github.com/gravitational/teleport/pull/35874)
* Add custom attribute mapping to the  `saml_idp_service_provider` spec. [#35873](https://github.com/gravitational/teleport/pull/35873)
* Fixed PIV not being available on Windows tsh binaries [#35866](https://github.com/gravitational/teleport/pull/35866)
* Restored direct dial SSH server compatibility with certain SSH tools such as `ssh-keyscan` (#35647) [#35859](https://github.com/gravitational/teleport/pull/35859)
* Prevent users from deleting their last passwordless device [#35855](https://github.com/gravitational/teleport/pull/35855)
* the `teleport-kube-agent` chart now supports passing extra arguments to the updater. [#35831](https://github.com/gravitational/teleport/pull/35831)
* New Access Lists with an unspecified NextAuditDate now pick a new date instead of being rejected [#35830](https://github.com/gravitational/teleport/pull/35830)
* Changed the minimal supported macOS version of Teleport Connect to 10.15 (Catalina) [#35819](https://github.com/gravitational/teleport/pull/35819)
* Add non-AD desktops to Enroll New Resource [#35797](https://github.com/gravitational/teleport/pull/35797)
* Fixed a bug in `teleport-kube-agent` chart when using both `appResources` and the `discovery` role. [#35783](https://github.com/gravitational/teleport/pull/35783)
* Fixed session upload audit events sometimes containing an incorrect URL for the session recording. [#35777](https://github.com/gravitational/teleport/pull/35777)
* Prevent tsh from re-authenticating if the MFA ceremony fails during `tsh ssh` [#35750](https://github.com/gravitational/teleport/pull/35750)
* Prevent attempts to join a nonexistent SSH session from hanging forever [#35743](https://github.com/gravitational/teleport/pull/35743)
* Improved Windows hosts registration with a new `static_hosts` configuration field [#35742](https://github.com/gravitational/teleport/pull/35742)
* Fixed the sorting of name and description columns for user groups when creating an Access Request [#35729](https://github.com/gravitational/teleport/pull/35729)

## 14.2.3 (12/14/23)

* Prevent Cloud tenants from being a leaf cluster. [#35687](https://github.com/gravitational/teleport/pull/35687)
* Added "Show All Labels" button in the unified resources list view. [#35666](https://github.com/gravitational/teleport/pull/35666)
* Added auto approval flow to servicenow plugin. [#35658](https://github.com/gravitational/teleport/pull/35658)
* Added guided SAML entity descriptor creation when entity descriptor XML is not yet available. [#35657](https://github.com/gravitational/teleport/pull/35657)
* Added a connection test when enrolling a new Connect My Computer resource in Web UI. [#35649](https://github.com/gravitational/teleport/pull/35649)
* Fixed regression of Kubernetes Server Address when Teleport runs in multiplex mode. [#35633](https://github.com/gravitational/teleport/pull/35633)
* When using the Slack plugin, users will now be notified directly of Access Requests and their approvals or denials. [#35577](https://github.com/gravitational/teleport/pull/35577)
* Fixed bug where configuration errors with an individual SSO connector impacted other connectors. [#35576](https://github.com/gravitational/teleport/pull/35576)
* Fixed client IP propagation from the Proxy to the Auth during IdP initiated SSO. [#35545](https://github.com/gravitational/teleport/pull/35545)

## 14.2.2 (12/07/23)

**Note**: `tsh` v14.2.2 has a known issue where `tsh kube login` uses an
incorrect port for clusters with multiplex mode enabled. If you use Kubernetes
access with multiplex mode, we recommend downgrading `tsh` to 14.2.1 until a fix
is available.

* Prevent panic when dialing a deleted Application Server. [#35525](https://github.com/gravitational/teleport/pull/35525)
* Fixed regression issue with arm32 binaries in 14.2.1 having higher glibc requirements. [#35539](https://github.com/gravitational/teleport/pull/35539)
* Fixed GCP VM auto-discovery not using instances' internal IP address. [#35521](https://github.com/gravitational/teleport/pull/35521)
* Calculate latency of Web SSH sessions and report it to users. [#35516](https://github.com/gravitational/teleport/pull/35516)
* Fix bot's unable to view or approve Access Requests issue. [#35512](https://github.com/gravitational/teleport/pull/35512)
* Fix querying of large audit events with Athena backend. [#35483](https://github.com/gravitational/teleport/pull/35483)
* Fix panic on potential nil value when requesting `/webapi/presetroles`. [#35463](https://github.com/gravitational/teleport/pull/35463)
* Add `insecure-drop` host user creation mode. [#35403](https://github.com/gravitational/teleport/pull/35403)
* IAM permissions for `rds:DescribeDBProxyTargets` are no longer required for RDS Proxy discovery. [#35389](https://github.com/gravitational/teleport/pull/35389)
* Update Go to `1.21.5`. [#35371](https://github.com/gravitational/teleport/pull/35371)
* Desktop connections default to RDP port 3389 if not otherwise specified. [#35343](https://github.com/gravitational/teleport/pull/35343)
* Add `cluster_auth_preferences` to the shortcuts for `cluster_auth_preference`. [#35329](https://github.com/gravitational/teleport/pull/35329)
* Make the `podSecurityPolicy` configurable in the `teleport-kube-agent` chart. [#35320](https://github.com/gravitational/teleport/pull/35320)
* Prevent EKS fetcher not having correct IAM permissions from stopping whole Discovery service start up. [#35319](https://github.com/gravitational/teleport/pull/35319)
* Add database automatic user provisioning support for self-hosted MongoDB. [#35317](https://github.com/gravitational/teleport/pull/35317)
* Improve the resilience of `tbot` to misconfiguration of auth connectors when generating a Kubernetes output. [#35309](https://github.com/gravitational/teleport/pull/35309)
* Fix crash when writing kubeconfig with `tctl auth sign --tar`. [#34874](https://github.com/gravitational/teleport/pull/34874)

## 14.2.1 (11/30/23)

* Fixed issue that could cause app and desktop session recording events to be written to the audit log. [#35183](https://github.com/gravitational/teleport/pull/35183)
* Fixed a possible panic when downgrading Teleport roles to older versions. [#35236](https://github.com/gravitational/teleport/pull/35236)
* Fixed a regression issue where tsh db connect to Redis 7 fails with an error on REDIS_REPLY_STATUS. [#35162](https://github.com/gravitational/teleport/pull/35162)
* Allow Teleport to complete abandoned uploads faster in HA deployments. [#35102](https://github.com/gravitational/teleport/pull/35102)
* Fixed error when installing a v13 node with the default installer from a v14 cluster. [#35058](https://github.com/gravitational/teleport/pull/35058)
* Fixed issue with the absence of membership expiry circumventing membership requirements check. [#35057](https://github.com/gravitational/teleport/pull/35057)
* Added read verb to suggested role spec when enrolling new resources. [#35053](https://github.com/gravitational/teleport/pull/35053)
* Added more new "Enroll Integration" tiles for Machine ID guides. [#35050](https://github.com/gravitational/teleport/pull/35050)
* Fixed default installer yum error on RHEL and Amazon Linux. [#35021](https://github.com/gravitational/teleport/pull/35021)
* External Audit Storage enables Cloud customers to store Audit Logs and Session Recordings in their own AWS account. [#35008](https://github.com/gravitational/teleport/pull/35008)
* Fixed IP propagation for nodes/bots joining the cluster and add LoginIP to bot certificates. [#34958](https://github.com/gravitational/teleport/pull/34958)
* Fixed an issue `tsh db connect <mongodb>` does not give reason on connection errors. [#34910](https://github.com/gravitational/teleport/pull/34910)
* Updated distroless images to use Debian 12. [#34878](https://github.com/gravitational/teleport/pull/34878)
* Added new email-based UI for inviting new local users on Teleport Enterprise (Cloud) clusters. [#34869](https://github.com/gravitational/teleport/pull/34869)
* Fix an issue "Allowed Users" in "tsh db ls" shows wrong user for databases with Automatic User Provisioning enabled. [#34850](https://github.com/gravitational/teleport/pull/34850)
* Fixed issue with application Access Requests and web UI large file downloads timing out after 30 seconds. [#34849](https://github.com/gravitational/teleport/pull/34849)
* Added default database support for PostgreSQL auto-user provisioning. [#34840](https://github.com/gravitational/teleport/pull/34840)
* Machine ID: handle kernel version check failing more gracefully. [#34828](https://github.com/gravitational/teleport/pull/34828)

## 14.2.0 (11/20/23)

### New Features
#### Advanced Okta Integration (Enterprise Edition only)
Teleport will be able to automatically create SSO connector and sync users when configuring Okta integration.

#### Connect my Computer support in Web UI
The Teleport web UI will provide a guided flow for joining your computer to the Teleport cluster using Teleport Connect.

#### Dynamic credential reloading for plugins
Teleport plugins will support dynamic credential reloading, allowing them to take advantage of short-lived (and frequently rotated) credentials generated by Machine ID.

### Fixes and Improvements
* Access list review reminders will now be sent via Slack [#34663](https://github.com/gravitational/teleport/pull/34663)
* Improve the error message when attempting to enroll a hardware key that cannot support passwordless [#34589](https://github.com/gravitational/teleport/pull/34589)
* Allow selecting multiple resource filters in the search bar in Connect [#34543](https://github.com/gravitational/teleport/pull/34543)
* Added a guided flow for joining your computer to the Teleport cluster using Teleport Connect; find it in the Web UI under Enroll New Resource -> Connect My Computer (available only for local users, with prerequisites) [#33688](https://github.com/gravitational/teleport/pull/33688)

## 14.1.5 (11/16/2023)

* Increased the maximum width of the console tabs in the web UI. [#34648](https://github.com/gravitational/teleport/pull/34648)
* Fixed accessing dedicated Proxy Kubernetes port when TLS routing is enabled. [#34645](https://github.com/gravitational/teleport/pull/34645)
* Fixed `tsh --piv-slot` custom PIV slot setting for Hardware Key Support. [#34592](https://github.com/gravitational/teleport/pull/34592)
* Disabled AWS IMDSv1 fallback and enforced use of FIPS endpoints in FIPS mode. [#34433](https://github.com/gravitational/teleport/pull/34433)
* Fixed incorrect permissions when opening X11 listener. [#34617](https://github.com/gravitational/teleport/pull/34617)
* Prevented `.tsh/environment` values from overriding prior set values. [#34626](https://github.com/gravitational/teleport/pull/34626)
* Changed Access Lists to respect user locking. [#34620](https://github.com/gravitational/teleport/pull/34620)
* Fixed Access Requests to respect explicit deny rules. [#34600](https://github.com/gravitational/teleport/pull/34600)
* Added Teleport Access Graph integration. [#34569](https://github.com/gravitational/teleport/pull/34569)
* Fixed cleanup of unused GCP KMS keys. [#34468](https://github.com/gravitational/teleport/pull/34468)
* Added list view option to the unified resources page. [#34466](https://github.com/gravitational/teleport/pull/34466)
* Fixed duplicate entries in resources view when updating nodename [#34236](https://github.com/gravitational/teleport/issues/34236) [#34453](https://github.com/gravitational/teleport/pull/34453)
* Allow configuring `cluster_networking_config` and `cluster_auth_preference` via `--bootstrap`. [#34445](https://github.com/gravitational/teleport/pull/34445)
* Fixed `tsh logout` with broken key directory. [#34435](https://github.com/gravitational/teleport/pull/34435)
* Added binary formatted parameters as base64 encoded strings to PostgreSQL Statement Bind audit log events. [#34432](https://github.com/gravitational/teleport/pull/34432)
* Reduced CPU & memory usage, and logging in the operator, by reusing connections to Teleport. [#34425](https://github.com/gravitational/teleport/pull/34425)
* Updated the code signing certificate for Windows artifacts. [#34377](https://github.com/gravitational/teleport/pull/34377)
* Added IAM Authentication support for Amazon MemoryDB Access. [#34348](https://github.com/gravitational/teleport/pull/34348)
* Split large desktop recordings into multiple files during export. [#34319](https://github.com/gravitational/teleport/pull/34319)
* Allow setting server labels from tctl. [#34137](https://github.com/gravitational/teleport/pull/34137)

## 14.1.3 (11/8/23)

### Security Fixes

#### [Medium] Arbitrary code execution with `LD_PRELOAD` and `SFTP`

Teleport implements SFTP using a subcommand. Prior to this release it was
possible to inject environment variables into the execution of this
subcommand, via shell init scripts or via the SSH environment request.

This is addressed by preventing `LD_PRELOAD` and other dangerous environment
variables from being forwarded during re-exec.

[#3274](https://github.com/gravitational/teleport/pull/34274)

#### [Medium] Outbound SSH from Proxy can lead to IP spoofing

If the Teleport auth or proxy services are configured to accept `PROXY`
protocol headers, a malicious actor can use this to spoof their IP address.

This is addressed by requiring that the first bytes of any SSH connection are
the SSH protocol prefix, denying a malicious actor the opportunity to send their
own proxy headers.

[#33729](https://github.com/gravitational/teleport/pull/33729)

### Other Fixes & Improvements

* Fixed issue where tbot would select the wrong address for Kubernetes access when in ports separate mode [#34283](https://github.com/gravitational/teleport/pull/34283)
* Added post-review state of Access Request in audit log description [#34213](https://github.com/gravitational/teleport/pull/34213)
* Updated Operator Reconciliation to skip Teleport Operator on status updates [#34194](https://github.com/gravitational/teleport/pull/34194)
* Updated Kube Agent Auto-Discovery to install the Teleport version provided by Automatic Upgrades [#34157](https://github.com/gravitational/teleport/pull/34157)
* Updated Server Auto-Discovery installer script to use `bash` instead of `sh` [#34144](https://github.com/gravitational/teleport/pull/34144)
* When a promotable Access Request targets a resource that belongs to an Access List, owners of that list will now automatically be added as reviewers.  [#34131](https://github.com/gravitational/teleport/pull/34131)
* Added Database Automatic User Provisioning support for Redshift [#34126](https://github.com/gravitational/teleport/pull/34126)
* Added `teleport_auth_type` config parameter to the AWS Terraform examples [#34124](https://github.com/gravitational/teleport/pull/34124)
* Fixed issue where an auto-provisioned PostgreSQL user may keep old roles indefinitely  [#34121](https://github.com/gravitational/teleport/pull/34121)
* Fixed incorrectly set file mode for Windows TPM files [#34113](https://github.com/gravitational/teleport/pull/34113)
* Added dynamic credential reloading for access plugins [#34079](https://github.com/gravitational/teleport/pull/34079)
* Fixed Azure Identity federated Application ID [#33960](https://github.com/gravitational/teleport/pull/33960)
* Fixed issue where Kubernetes Audit Events reported incorrect information in the exec audit [#33950](https://github.com/gravitational/teleport/pull/33950)
* Added support for formatting hostname as `host:port` to `tsh puttyconfig` [#33883](https://github.com/gravitational/teleport/pull/33883)
* Added support for `--set-context-name` to `tsh proxy kube`
* Fixed various Access List bookkeeping issues [#33834](https://github.com/gravitational/teleport/pull/33834)
* Fixed issue where `tsh aws ecs execute-command` would always fail [#33833](https://github.com/gravitational/teleport/pull/33833)
* Updated UI to automatically redirect to login page on missing session cookie [#33806](https://github.com/gravitational/teleport/pull/33806)
* Added Dynamic Discovery matching for Databases [#33693](https://github.com/gravitational/teleport/pull/33693)
* Fixed formatting errors on empty result sets in `tsh` [#33633](https://github.com/gravitational/teleport/pull/33633)
* Added Database Automatic User Provisioning support for MariaDB [#34256](https://github.com/gravitational/teleport/pull/34256)
* Fixed issue where MySQL auto-user deletion fails on usernames with quotes [#34304](https://github.com/gravitational/teleport/pull/34304)

## 14.1.1 (10/23/23)

* Fixed the top bar breaking layout when the window is narrow in Connect [#33821](https://github.com/gravitational/teleport/pull/33821)
* Limited Snowflake decompressed request to 10MB [#33764](https://github.com/gravitational/teleport/pull/33764)
* Added MySQL auto-user deletion [#33710](https://github.com/gravitational/teleport/pull/33710)
* Configured Connect to intercept deep link clicks [#33684](https://github.com/gravitational/teleport/pull/33684)
* Added URL and SAML connector name in entity descriptor URL errors [#33667](https://github.com/gravitational/teleport/pull/33667)
* Added the ability to run a specific tool to Assist. [#33640](https://github.com/gravitational/teleport/pull/33640)
* Added PostgreSQL auto-user deletion [#33570](https://github.com/gravitational/teleport/pull/33570)
* Added DiscoveryConfig CRUD operations [#33380](https://github.com/gravitational/teleport/pull/33380)

## 14.1.0 (10/18/23)

### New features

* Teleport Connect 14.1 introduces Connect My Computer which makes it possible to add your personal machine to a Teleport cluster in just a couple of clicks. Whether you're exploring capabilities of Teleport or want to make your computer available in your private cluster, Connect My Computer lets you do that without having to use the terminal to get the job done.
* Resource pinning allows you to pin your most frequently accessed resources to a separate page.
* Access Monitoring provides a view of risky accounts access and access anti-patterns in clusters using Athena as the audit log backend.
* Users can connect to EC2 instances via Amazon EC2 Instance Connect endpoints without needing to install Teleport Agents.
* Access list owners will be able to perform regular periodic reviews of the Access List members.

### Security fixes
* Updated golang.org/x/net dependency. [#33420](https://github.com/gravitational/teleport/pull/33420)
  * swift-nio-http2 vulnerable to HTTP/2 Stream Cancellation Attack: [CVE-2023-44487](https://github.com/advisories/GHSA-qppj-fm5r-hxr3)
* Updated `google.golang.org/grpc` to v1.57.1. [#33487](https://github.com/gravitational/teleport/pull/33487)
  * swift-nio-http2 vulnerable to HTTP/2 Stream Cancellation Attack: [CVE-2023-44487](https://github.com/advisories/GHSA-qppj-fm5r-hxr3)
* Updated OpenTelemetry dependency. [#33523](https://github.com/gravitational/teleport/pull/33523) [#33550](https://github.com/gravitational/teleport/pull/33550)
  * OpenTelemetry-Go Contrib vulnerable to denial of service in otelhttp due to unbound cardinality metrics: [CVE-2023-45142](https://github.com/advisories/GHSA-rcjv-mgp8-qvmr)
* Updated babel/core to 7.3.2. [#33441](https://github.com/gravitational/teleport/pull/33441)
  * Arbitrary code execution when compiling specifically crafted malicious code: [CVE-2023-45133](https://github.com/babel/babel/security/advisories/GHSA-67hx-6x53-jw92)

### Other fixes and improvements

* Web SSH sessions are terminated right away when a user closes the tab. [#33529](https://github.com/gravitational/teleport/pull/33529)
* Added the ability for bots to submit Access Request reviews. [#33509](https://github.com/gravitational/teleport/pull/33509)
* Added access review notifications when logging in via `tsh` or running `tsh status`. [#33468](https://github.com/gravitational/teleport/pull/33468)
* Added database automatic user provisioning support for MySQL. [#33379](https://github.com/gravitational/teleport/pull/33379)
* Added job to update the Teleport version for deployments in Amazon ECS used during RDS Enrollment. [#33313](https://github.com/gravitational/teleport/pull/33313)
* Fixed Teleport Assist SQL view names. [#33581](https://github.com/gravitational/teleport/pull/33581)
* Fixed hardware key support for sso web login. [#33548](https://github.com/gravitational/teleport/pull/33548)
* Fixed Access Lists to allow them to affect Access Request permissions. [#33350](https://github.com/gravitational/teleport/pull/33350)
* Prevented remote proxies from impersonating users from different clusters. [#33539](https://github.com/gravitational/teleport/pull/33539)
* Added link to Access Request in ServiceNow incidents. [#33593](https://github.com/gravitational/teleport/pull/33593)
* Added new "Identity Governance & Security" navigation section in web UI. [#33423](https://github.com/gravitational/teleport/pull/33423)
* Fixed `tsh` connection issue when Proxy is in separate mode and Web port is TLS-terminated by a load balancer. [#32531](https://github.com/gravitational/teleport/issues/32531) [#33406](https://github.com/gravitational/teleport/pull/33406)
* Fixed panic when trying to register resources from older Kubernetes clusters with `extensions/v1beta1` group/version. [#33402](https://github.com/gravitational/teleport/pull/33402)
* Fixed Access List audit log messages to properly include user names. [#33383](https://github.com/gravitational/teleport/pull/33383)
* Added notification icon to Web UI to show Access List review notifications. [#33381](https://github.com/gravitational/teleport/pull/33381)
* Fixed creation of `@teleport-access-approver` role to `v6` to support downgrades to Teleport 13. [#33354](https://github.com/gravitational/teleport/pull/33354)
* Added ability to specify PIV slot for hardware key support. [#33352](https://github.com/gravitational/teleport/pull/33352) [#33353](https://github.com/gravitational/teleport/pull/33353)
* Extended timeout when waiting for hardware key touch/PIN. [#33348](https://github.com/gravitational/teleport/pull/33348)
* Added support for Windows AD root domain for PKI operations. [#33275](https://github.com/gravitational/teleport/pull/33275)
* Added resources to Slack notification of Access Requests. [#33264](https://github.com/gravitational/teleport/pull/33264)
* Fixed provision tokens to make system roles case-insensitive. [#33260](https://github.com/gravitational/teleport/pull/33260)

## 14.0.3 (10/11/23)

### Security Fixes

#### [Critical] Privilege escalation through `RecursiveChown`

When using automatic Linux user creation, an attacker could exploit a race
condition in the user creation functionality to `chown` arbitrary files on the
system.

Users who aren't using automatic Linux host user creation aren’t affected by
this vulnerability.

[#33248](https://github.com/gravitational/teleport/pull/33248)

### Other Fixes

 * Fixed spurious timeouts in database access sessions [#32720](https://github.com/gravitational/teleport/pull/32720)
 * Azure VM auto-discovery can now find VMs with multiple managed identities [#32800](https://github.com/gravitational/teleport/pull/32800)
 * Fixed improperly set Kubernetes impersonation headers [#32848](https://github.com/gravitational/teleport/pull/32848)
 * `tsh puttyconfig` now uses `Validity` format for WinSCP compatibility [#32856](https://github.com/gravitational/teleport/pull/32856)
 * Teleport client now uses gRPC when connecting to the root cluster [#32662](https://github.com/gravitational/teleport/pull/32662)
 * Teleport client now uses gRPC when creating tracing client [#32663](https://github.com/gravitational/teleport/pull/32663)
 * Fixed panic on `tsh device enroll --current-device` [#32756](https://github.com/gravitational/teleport/pull/32756)
 * The Teleport `etcd` backend will now start if some nodes are unreachable [#32779](https://github.com/gravitational/teleport/pull/32779)
 * Fixed certificate verification issues when using `kubectl exec` [#32768](https://github.com/gravitational/teleport/pull/32768)
 * Added Discover flow for enrolling EC2 Instances with Endpoint Instance Connect [#32760](https://github.com/gravitational/teleport/pull/32760)
 * Added connection information to multiplexer logs [#32738](https://github.com/gravitational/teleport/pull/32738)
 * Fixed issue causing keys to be incorrectly removed in tsh and Teleport Connect on Windows [#32963](https://github.com/gravitational/teleport/pull/32963)
 * Improved Unified Resource Cache performance [#33027](https://github.com/gravitational/teleport/pull/33027)
 * Adds Audit Review recurrence presets [#32960](https://github.com/gravitational/teleport/pull/32960)
 * Fixed multiple discovery install attempts on Azure & GCP VMs [#32569](https://github.com/gravitational/teleport/pull/32569)
 * Fixed a corner case of privilege tokens where MFA devices disabled by cluster settings were still counted against the user [#32430](https://github.com/gravitational/teleport/pull/32430)
 * Fixed Access List caching & eventing issues  [#32649](https://github.com/gravitational/teleport/pull/32649)
 * Fixed user session tracking across trusted clusters [#32967](https://github.com/gravitational/teleport/pull/32967)
 * Added cost optimized pagination search for athena [#33007](https://github.com/gravitational/teleport/pull/33007)
 * Teleport now reports initial command to session moderators [#33112](https://github.com/gravitational/teleport/pull/33112)
 * OneOff install script now installs enterprise Teleport when generated by an enterprise cluster [#33148](https://github.com/gravitational/teleport/pull/33148)
 * Fixed issue when playing back a session recorded on a leaf cluster [#33102](https://github.com/gravitational/teleport/pull/33102)
 * Fixed self-signed certificate issue on macOS [#33156](https://github.com/gravitational/teleport/pull/33156)
 * Discovery EC2 instance listing now shows instance name [#33179](https://github.com/gravitational/teleport/pull/33179)
 * Fixed HTTP connection hijack issue when using `tsh proxy kube` [#33172](https://github.com/gravitational/teleport/pull/33172)
 * Improved error messaging in `tsh kube credentials`  when root cluster roles don't allow Kube access [#33210](https://github.com/gravitational/teleport/pull/33210)

## 14.0.1 (09/26/23)

* Fixed issue where Teleport Connect Kube terminal throws an internal server error [#32612](https://github.com/gravitational/teleport/pull/32612)
* Fixed `create_host_user_mode` issue with TeleportRole in the Teleport Operator CRDs [#32557](https://github.com/gravitational/teleport/pull/32557)
* Fixed issue that allowed for duplicate Access List owners [#32481](https://github.com/gravitational/teleport/pull/32481)
* Removed unnecessary permission requirement from PostgreSQL backend [#32474](https://github.com/gravitational/teleport/pull/32474)
* Added feature allowing for managing host sudoers without also creating users [#32400](https://github.com/gravitational/teleport/pull/32400)
* Fixed dynamic labels not being present on server access audit events [#32382](https://github.com/gravitational/teleport/pull/32382)
* Added PostHog events for discovered Kubernetes Apps [#32379](https://github.com/gravitational/teleport/pull/32379)
* Fixed issue where changing the cluster name leads to cluster being unaccessible [#32352](https://github.com/gravitational/teleport/pull/32352)
* Added additional logging for when the Teleport process file is not accessible due to a permission issue upon startup [#32348](https://github.com/gravitational/teleport/pull/32348)
* Fixed issue where the `teleport-kube-agent` Helm chart would created the same `ServiceAccount` multiple times [#32338](https://github.com/gravitational/teleport/pull/32338)
* Fixed GCP VM auto-discovery bugs [#32316](https://github.com/gravitational/teleport/pull/32316)
* Added Access List usage events [#32297](https://github.com/gravitational/teleport/pull/32297)
* Allowed for including only traits when doing a JWT rewrite for web application access [#32291](https://github.com/gravitational/teleport/pull/32291)
* Added `IneligibleStatus` fields for Access List members and owners [#32278](https://github.com/gravitational/teleport/pull/32278)
* Fixed issue where the Auth Service was listed twice in the inventory of connected resources [#32270](https://github.com/gravitational/teleport/pull/32270)
* Added three second shutdown delay on on `SIGINT`/`SIGTERM` [#32189](https://github.com/gravitational/teleport/pull/32189)
* Add initial ServiceNow plugin [#32131](https://github.com/gravitational/teleport/pull/32131)

## 14.0.0 (09/20/23)

Teleport 14 brings the following new major features and improvements:

- Access lists
- Unified resource view
- ClickHouse support for database access
- Advanced audit log
- Kubernetes apps auto-discovery
- Extended Kubernetes per-resource RBAC
- Oracle database access audit logging support
- Enhanced PuTTY support
- Support for TLS routing in Terraform deployment examples
- Discord and ServiceNow hosted plugins
- Limited passwordless access for local Windows users in Teleport Community
  Edition 
- Machine ID: Kubernetes Secret destination

In addition, this release includes several changes that affect existing
functionality listed in the “Breaking changes” section below. Users are advised
to review them before upgrading.

### New features

#### Advanced audit log

Teleport 14 includes support for a new audit log powered by Amazon S3 and Athena
that supports efficient searching, sorting, and filtering operations. Teleport
Enterprise (Cloud) customers will have their audit log automatically migrated to
this new backend.

See the documentation [here](docs/pages/reference/backends.mdx#athena).

#### Access lists

Teleport 14 introduces foundational support for Access Lists, an extension to
the short-lived Access Request system targeted towards longer-term access.
Administrators can add users to Access Lists granting them long-term permissions
within the cluster.

As the feature is being developed, future Teleport releases will add support for
periodic audit reviews and deeper integration of Access Lists with Okta.

You can find existing Access Lists documentation [here](docs/pages/admin-guides/access-controls/access-lists/guide.mdx).

#### Unified resources view

The web UI in Teleport 14 has been updated to show all resources in a single
unified view.

This is the first step in a series of changes designed to support a
customizable Teleport experience and make it easier to access the resources that
are most important to you.

#### Kubernetes apps auto-discovery

Teleport 14 updates its auto-discovery capabilities with support for web
applications in Kubernetes clusters. When connected to a Kubernetes cluster (or
deployed as a Helm chart), the Teleport Discovery Service will automatically find
and enroll web applications with your Teleport cluster.

See documentation [here](docs/pages/enroll-resources/auto-discovery/kubernetes-applications/kubernetes-applications.mdx).

#### Extended Kubernetes per-resource RBAC

Teleport 14 extends resource-based Access Requests to support more Kubernetes
resources than just pods, including custom resources, and verbs. Note that this
feature requires role version `v7`.

See Kubernetes resources documentation to see a full list of [supported
resources](docs/pages/enroll-resources/kubernetes-access/controls.mdx#kubernetes_resources).

#### ClickHouse support for database access

Teleport 14 adds database access support for ClickHouse HTTP and native (TCP)
protocols. When using HTTP protocol, the user's query activity is captured in
the Teleport audit log.

See how to connect ClickHouse to Teleport
[here](docs/pages/enroll-resources/database-access/enroll-self-hosted-databases/clickhouse-self-hosted.mdx).

#### Oracle database access audit logging support

In Teleport 14, database access for Oracle integration is updated with query
audit logging support.

See documentation on how to configure it in the [Oracle guide](docs/pages/enroll-resources/database-access/enroll-self-hosted-databases/oracle-self-hosted.mdx).

#### Limited passwordless access for local Windows users in Teleport Community Edition 

In Teleport 14, access to Windows desktops with local Windows users has been
extended to Community Edition. Teleport will permit users to register and
connect to up to 5 desktops with local users without an enterprise license.

For more information on using Teleport with local Windows users, see [docs](docs/pages/enroll-resources/desktop-access/getting-started.mdx).

#### Discord and ServiceNow hosted plugins

Teleport 14 includes support for hosted Discord and ServiceNow plugins. Teleport
Enterprise (Cloud) users can configure Discord and ServiceNow integrations to
receive Access Request notifications.

Discord plugin is available now, ServiceNow is coming in 14.0.1.

#### Enhanced PuTTY Support

tsh on Windows now supports the `tsh puttyconfig` command, which can
configure saved sessions inside the well-known PuTTY client to connect to
Teleport-protected servers.

For more information, see [docs](docs/pages/connect-your-client/putty-winscp.mdx).

#### Support for TLS routing in Terraform deployment examples

The ha-autoscale-cluster and starter-cluster Terraform deployment examples now
support a `USE_TLS_ROUTING` variable to enable TLS routing inside the deployed
Teleport cluster.

#### Machine ID: Kubernetes Secret destination

In Teleport 14, `tbot` can now be configured to write artifacts such as
credentials and configuration files directly to a Kubernetes secret rather than
a directory on the local file system.

For more information, see [docs](docs/pages/reference/machine-id/configuration.mdx).

### Breaking changes and deprecations

Please familiarize yourself with the following potentially disruptive changes in
Teleport 14 before upgrading.

#### SSH node open dial no longer supported

Teleport 14 no longer allows connecting to OpenSSH servers not registered with
the cluster. Follow the updated agentless OpenSSH integration [guide](docs/pages/enroll-resources/server-access/openssh/openssh-agentless.mdx)
to register your OpenSSH nodes in the cluster’s inventory.

You can set `TELEPORT_UNSTABLE_UNLISTED_AGENT_DIALING=yes` environment variable
on Teleport proxy to temporarily re-enable the open dial functionality. The
environment variable will be removed in Teleport 15.

#### Proxy protocol default change

Starting from version 14, Teleport will require users to explicitly enable or
disable PROXY protocol in their `proxy_service`/`auth_service` configuration
using `proxy_protocol: on|off` option.

Users who run their proxies behind L4 load balancers with PROXY protocol
enabled, should set `proxy_protocol: on`.  Users who don’t run Teleport behind
PROXY protocol enabled load balancers, should disable `proxy_protocol: off`
explicitly for security reasons.

By default, Teleport will accept the PROXY line but will prevent connections
with IP pinning enabled. IP pinning users will need to explicitly enable/disable
proxy protocol like explained above.

See more details in our [documentation](docs/pages/admin-guides/management/security/proxy-protocol.mdx).

#### Legacy deb/rpm package repositories are deprecated

Teleport 14 will be the last release published to the legacy package
repositories at `deb.releases.teleport.dev` and `rpm.releases.teleport.dev`.
Starting with Teleport 15, packages will only be published to the new
repositories at `apt.releases.teleport.dev` and `yum.releases.teleport.dev`.

All users are recommended to switch to `apt.releases.teleport.dev` and
`yum.releases.teleport.dev` repositories as described in installation
[instructions](docs/pages/installation.mdx).

#### `Cf-Access-Token` header no longer included with requests to Teleport-protected applications

Starting from Teleport 14, the `Cf-Access-Token` header containing the signed
JWT token will no longer be included by default with all requests to
Teleport-protected applications.
All requests will still include `Teleport-JWT-Assertion` containing the JWT
token.

See documentation for details on how to inject the JWT token into any header
using [header rewriting](docs/pages/enroll-resources/application-access/jwt/introduction.mdx#inject-jwt).

#### tsh db CLI commands changes

In Teleport 14 tsh db sub-commands will attempt to select a default value for
`--db-user` or `--db-name` flags if they are not provided by the user by
examining their allowed `db_users` and `db_names`.

The flags `--cert-file` and `--key-file` for tsh proxy db command were also
removed, in favor of the `--tunnel` flag that opens an authenticated local
database proxy.

#### MongoDB versions prior to 3.6 are no longer supported

Teleport 14 includes an update to the MongoDB driver.

Due to the MongoDB team dropping support for servers prior to version 3.6 (which
reached EOL on April 30, 2021), Teleport also will no longer be able to support
these old server versions.

#### Symlinks for `~/.tsh/environment` no longer supported

In order to strengthen the security in Teleport 14, file loading from home
directories where the path includes a symlink is no longer allowed.  The most
common use case for this is loading environment variables from the
`~/.tsh/environment` file.  This will still work normally as long as the path
includes no symlinks.

#### Deprecated audit event

Teleport 14 deprecates the `trusted_cluster_token.create` audit event, replacing
it with a new `join_token.create` event. The new event is emitted when any join
token is created, whether it be for trusted clusters or other Teleport services.

Teleport 14 will emit both events when a trusted cluster join token is created.
Starting in Teleport 15, the `trusted_cluster_token.create` event will no longer
be emitted.

### Other changes

#### DynamoDB billing mode defaults to on-demand

In Teleport 14, when creating new DynamoDB tables, Teleport will now create them
with the billing mode set to `pay_per_request` instead of being set to provisioned
mode.

The old behavior can be restored by setting the `billing_mode` option in the
storage configuration.

#### Default role version is v7

The default role version in Teleport 14 is `v7` which enables support for extended
Kubernetes per-resource RBAC, and changes the `kubernetes_resources` default to
wildcard for better getting started user experience.

You can review role versions in the [documentation](docs/pages/reference/access-controls/roles.mdx).

#### Stricter name validation for auto-discovered databases

In Teleport 14, database discovery via `db_service` config enforces the same name
validation as for databases created via tctl, static config, and
`discovery_service`.

As such, database names in AWS, GCP and Azure must start with a letter, contain
only letters, digits, and hyphens and end with a letter or digit (no trailing
hyphens).

#### Access Request API changes

Teleport 14 introduces a new and more secure API for submitting Access Requests.
As a result, tsh users may be prompted to upgrade their clients before
submitting an Access Request.

#### Desktop discovery name change

Desktops discovered via LDAP will have a short suffix appended to their name to
ensure uniqueness. Users will notice duplicate desktops (with and without the
suffix) for up to an hour after upgrading. Connectivity to desktops will not be
affected, and the old record will naturally expire after 1 hour.

#### Machine ID : New configuration schema

Teleport 14 introduces a new configuration schema (v2) for Machine ID’s agent
`tbot`.  The new schema is designed to be more explicit and more extensible:

```yaml
version: v2
onboarding:
 token: gcp-bot
 join_method: gcp
storage:
 type: memory
auth_server: example.teleport.sh:443
outputs:
 - type: identity
   destination:
     type: kubernetes_secret
     name: my-secret
​
 - type: kubernetes
   kubernetes_cluster: my-cluster
   destination:
     type: directory
     path: ./k8s
​
 - type: database
   service: my-postgres-service
   database: postgres
   username: postgres
   destination:
     type: directory
     path: ./db
​
 - type: application
   app_name: my-app
   destination:
     type: directory
     path: ./app
```

`tbot` will continue to support the v1 schema for several Teleport versions but it
is recommended that you migrate to v2 as soon as possible to benefit from new
Machine ID features.

For more details and guidance on how to upgrade to v2, see [docs](docs/pages/reference/machine-id/v14-upgrade-guide.mdx).

## 13.4.0 (09/20/23)

### Security Fixes

#### [Critical] Privilege escalation via host user creation

When using automatic Linux user creation, an attacker could exploit a race
condition in the user creation functionality to create arbitrary files on the
system as root writable by the created user.

This could allow the attacker to escalate their privileges to root.

Users who aren't using automatic Linux host user creation aren’t affected by
this vulnerability.

[#32210](https://github.com/gravitational/teleport/pull/32210)

#### [High] Insufficient auth token verification when signing self-hosted database certificates

When signing self-hosted database certificates, Teleport did not sufficiently
validate the authorization token type.

This could allow an attacker to sign valid database access certificates using a
guessed authorization token name.

Users who aren’t using self-hosted database access aren’t affected by this
vulnerability.

[#32215](https://github.com/gravitational/teleport/pull/32215)

#### [High] Privilege escalation via untrusted config file on Windows

When loading the global tsh configuration file tsh.yaml on Windows, Teleport
would look for the file in a potentially untrusted directory.

This could allow a malicious user to create harmful command aliases for all tsh
users on the system.

Users who aren’t using tsh on Windows aren’t affected by this vulnerability.

[#32223](https://github.com/gravitational/teleport/pull/32223)

#### [High] XSS in SAML IdP

When registering a service provider with SAML IdP, Teleport did not sufficiently
validate the ACS endpoint.

This could allow an attacker to execute arbitrary code at the client-side
leading to privilege escalation.

This issue only affects Teleport Enterprise Edition. Enterprise users who aren’t
using Teleport SAML IdP functionality aren’t affected by this vulnerability.

[#32220](https://github.com/gravitational/teleport/pull/32220)

### Other fixes and improvements

* Added `change_feed_conn_string` option to PostgreSQL backend. [#31938](https://github.com/gravitational/teleport/pull/31938)
* Added single-command AWS OIDC integration. [#31790](https://github.com/gravitational/teleport/pull/31790)
* Added `pprof` support to Kubernetes Operator to diagnose memory use. [#31707](https://github.com/gravitational/teleport/pull/31707)
* Added support for bot and agent joining from external Kubernetes Clusters. [#31703](https://github.com/gravitational/teleport/pull/31703)
* Extend EC2 joining to Discovery, MDM and Okta services. [#31894](https://github.com/gravitational/teleport/pull/31894)
* Support discovery for new AWS region il-central-1. [#31830](https://github.com/gravitational/teleport/pull/31830) [#31840](https://github.com/gravitational/teleport/pull/31840)
* Fails with an error if desktops are created with invalid names. [#31766](https://github.com/gravitational/teleport/pull/31766)
* Fixed directory sharing in desktop access for non-ascii directory names. [#31924](https://github.com/gravitational/teleport/pull/31924)
* Fixed a `MissingRegion` error that would sometimes occur when running the discovery bootstrap command [#31701](https://github.com/gravitational/teleport/pull/31701)
* Fixed incorrect autofill in Safari. [#31611](https://github.com/gravitational/teleport/pull/31611)
* Fixed terminal resizing bug in web terminal. [#31586](https://github.com/gravitational/teleport/pull/31586)
* Fixed Session & Identity search bar. [#31581](https://github.com/gravitational/teleport/pull/31581)
* Fixed desktop sessions' viewport size to the size of browser window at session start. [#31524](https://github.com/gravitational/teleport/pull/31524)
* Fixed database and k8s cluster resource names to avoid name collisions. [#30456](https://github.com/gravitational/teleport/pull/30456)
* `tctl sso configure github` now includes default GitHub endpoints [#31480](https://github.com/gravitational/teleport/pull/31480)
* `tsh [proxy | db | kube]` subcommands now support `--query` and `--labels` optional arguments. [#32087](https://github.com/gravitational/teleport/pull/32087)
* `tsh` and `tctl` can select an auto-discovered database or Kubernetes cluster by its original name instead of the more detailed name generated by the v14+ Teleport Discovery service. [#32087](https://github.com/gravitational/teleport/pull/32087)
* `tsh` text-formatted output in non-verbose mode will display auto-discovered resources with original resource names instead of the more detailed names generated by the v14+ Teleport Discovery service. [#32084](https://github.com/gravitational/teleport/pull/32084) [#32083](https://github.com/gravitational/teleport/pull/32083)
* Updated discovery installers to work with SUSE zypper package manager. [#31428](https://github.com/gravitational/teleport/pull/31428)
* Updated Go to v1.20.8 [#31506](https://github.com/gravitational/teleport/pull/31506)
* Updated OpenSSL to 3.0.11 [#32160](https://github.com/gravitational/teleport/pull/32160)

## 13.3.8 (09/05/23)

* Fix WebAuthn Windows registration breakage. [#31420](https://github.com/gravitational/teleport/pull/31420)
* Fix issue with App access on leaf cluster trimming query parameters on rewrite redirects. [#31379](https://github.com/gravitational/teleport/pull/31379)
* Fix issue with web UI integrations screen not wrapping tiles correctly. [#31365](https://github.com/gravitational/teleport/pull/31365)
* Fix issue with `tsh db connect` ignoring default user/database names. [#31250](https://github.com/gravitational/teleport/pull/31250)
* Fix issue with Azure auto-discovery not picking up updated credentials. [#31164](https://github.com/gravitational/teleport/pull/31164)
* Fix issue with failing to start shell on macOS in some scenarios. [#31152](https://github.com/gravitational/teleport/pull/31152)
* Desktop discovery: avoid mapping IPv6 addresses. [#31434](https://github.com/gravitational/teleport/pull/31434)
* MySQL: improve performance in read-heavy scenarios. [#31402](https://github.com/gravitational/teleport/pull/31402)
* Add known STS endpoint for il-central-1. [#31282](https://github.com/gravitational/teleport/pull/31282)
* Add support for configurable Okta service synchronization duration. [#31251](https://github.com/gravitational/teleport/pull/31251)
* Add an optional PodMonitor to the teleport-kube-agent chart. [#31247](https://github.com/gravitational/teleport/pull/31247)
* Update web UI to skip MOTD in UI if request was initiated from tsh headless auth. [#31205](https://github.com/gravitational/teleport/pull/31205)
* Update Okta service to slow down API calls to avoid throttling. [teleport.e#2134](https://github.com/gravitational/teleport.e/pull/2134)

## 13.3.7 (08/29/23)

* Fixed regression issue causing OIDC authentication to fail with some identity providers. [teleport.e#2076](https://github.com/gravitational/teleport.e/pull/2076)
* Updated headless modal to show both Reject and Cancel. [#31135](https://github.com/gravitational/teleport/pull/31135)
* Added support for proxy environment variables when dialing directly to the Kubernetes Cluster. [#31133](https://github.com/gravitational/teleport/pull/31133)
* Fixed the Oracle Database GUI Access flow on Windows Platform. [#31129](https://github.com/gravitational/teleport/pull/31129)
* Added dynamic identity file reloading support for API Client. [#31076](https://github.com/gravitational/teleport/pull/31076)
* Fixed leaking connection monitor instances. [#31042](https://github.com/gravitational/teleport/pull/31042)
* Added support for IAM joining over reverse tunnel port [#31000](https://github.com/gravitational/teleport/pull/31000)

## 13.3.6 (08/25/23)

* Fixed regression in 13.3.5 causing bot locking when using the Kubernetes Operator. [#30996](https://github.com/gravitational/teleport/pull/30996)
* Fixed connection to desktop access service when session MFA is required. [#30963](https://github.com/gravitational/teleport/pull/30963)
* Fixed a regression with desktop discovery that could cause desktops to expire in environments with large numbers of desktops. [#31032](https://github.com/gravitational/teleport/pull/31032)
* Added support for forcing reauthentication in OIDC connectors via `max_age` parameter. [teleport.e#2042](https://github.com/gravitational/teleport.e/pull/2042)
* Added Discord hosted plugin support for Teleport Enterprise (Cloud). [teleport.e#2035](https://github.com/gravitational/teleport.e/pull/2035)
* Helm: Use cert-manager secret or tls.existingSecretName for ingress when enabled. [#30984](https://github.com/gravitational/teleport/pull/30984)
* Added preset device trust roles. [#30908](https://github.com/gravitational/teleport/pull/30908)
* Machine ID: Added support for JSON log formatting. [#30763](https://github.com/gravitational/teleport/pull/30763)
* Reduced alert log spam. [#30904](https://github.com/gravitational/teleport/pull/30904)

## 13.3.5 (08/22/23)

* Fixed a bug in teleport-cluster Helm chart causing Teleport to crash when Amazon DynamoDB autoscaling is enabled. [#30841](https://github.com/gravitational/teleport/pull/30841)
* Added Teleport Assist to Web Terminal. [#30811](https://github.com/gravitational/teleport/pull/30811)
* Fixed S3 metric name for completed multipart uploads. [#30710](https://github.com/gravitational/teleport/pull/30710)
* Added the ability for `tsh` to register and enroll the `--current-device`. [#30702](https://github.com/gravitational/teleport/pull/30702)
* Fixed Review Requests to disallow reviews after request is resolved. [#30690](https://github.com/gravitational/teleport/pull/30690)
* Ensure that SSH session errors are reported to the terminal. [#30684](https://github.com/gravitational/teleport/pull/30684)
* Fixed an issue with `tsh aws ssm start-session`. [#30668](https://github.com/gravitational/teleport/pull/30668)
* Fixed an issue with the Access Request failing with `invalid maxDuration`. [teleport.e#2037](https://github.com/gravitational/teleport.e/pull/2037)

### Security fix

* Security improvements with possible `medium` severity DoS conditions through protocol level attacks. [#30854](https://github.com/gravitational/teleport/pull/30854)

## 13.3.4 (08/18/23)

* Allow host users to be created with specific UID/GIDs [#30178](https://github.com/gravitational/teleport/pull/30178)
* Fixed SSH agent forwarding under Cygwin [#30582](https://github.com/gravitational/teleport/pull/30582)
* Fixed resource name resolution issues in `tsh db` [#30563](https://github.com/gravitational/teleport/pull/30563)
* Retired obsolete AWS `aurora` engine identifier [#30548](https://github.com/gravitational/teleport/pull/30548)
* Fixed issues with `tsh proxy kube` [#30477](https://github.com/gravitational/teleport/pull/30477)
* Added `skipConfirm` option to Teleport Connect headless approval flow [#30475](https://github.com/gravitational/teleport/pull/30475)
* Added increased validation of Database URLs discovered by Discovery Service [#30462](https://github.com/gravitational/teleport/pull/30462)
* Fixed decoding of SAML certificates with whitespace padding [#30450](https://github.com/gravitational/teleport/pull/30450)
* Fixed OTP prompt on Windows [#30444](https://github.com/gravitational/teleport/pull/30444)
* Improved LDAP desktop discovery [#30383](https://github.com/gravitational/teleport/pull/30383)
* Fixed desktop connection issues [#30275](https://github.com/gravitational/teleport/pull/30275)
* Fixed "user is not managed" error when accessing ElastiCache and MemoryDB [#30353](https://github.com/gravitational/teleport/pull/30353)
* Fixed spurious resource deletion in Firestore backend during update [#30287](https://github.com/gravitational/teleport/pull/30287)
* Added JWT claim rewriting configuration [#30280](https://github.com/gravitational/teleport/pull/30280)
* Fixed issue with `tsh login --headless` [#30307](https://github.com/gravitational/teleport/pull/30307)
* EKS and AKS discovery are now considered Generally Available [#30209](https://github.com/gravitational/teleport/pull/30209)
* Fixed a panic when importing GKE clusters without labels [#30647](https://github.com/gravitational/teleport/pull/30647)
* Added support for auditing chunked SQL Server packets [#30243](https://github.com/gravitational/teleport/pull/30243)
* Plugins now exit when the connection breaks in Kubernetes [#30039](https://github.com/gravitational/teleport/pull/30039)

## 13.3.2 (08/08/23)

* Fixed regression issue with excessive backend reads for nodes. [#30198](https://github.com/gravitational/teleport/pull/30198)
* Save device keys on `%APPDATA%/Local` instead of `%APPDATA%/Roaming`. [#30177](https://github.com/gravitational/teleport/pull/30177)
* Improved "tsh kube login" message for proxy behind L7 load balancer. [#30174](https://github.com/gravitational/teleport/pull/30174)
* Added auto-approval flow for Opsgenie plugin. [#30161](https://github.com/gravitational/teleport/pull/30161)
* Extend `tsh kube login --set-context-name` to support templating functions. [#30157](https://github.com/gravitational/teleport/pull/30157)
* Allow setting storage class name for auth component. [#30145](https://github.com/gravitational/teleport/pull/30145)
* Added hosted Jira integration. [#30117](https://github.com/gravitational/teleport/pull/30117), [#30040](https://github.com/gravitational/teleport/pull/30040)
* Added AWS configurator support for OpenSearch. [#30085](https://github.com/gravitational/teleport/pull/30085)
* Tightened Discovery Service permissions. [#29994](https://github.com/gravitational/teleport/pull/29994)
* Fixed authorization rules to the Assistant and UserPreferences service [#29961](https://github.com/gravitational/teleport/pull/29961)

## 13.3.1 (08/04/23)

* Added new Prometheus metric for created Access Requests. [#29991](https://github.com/gravitational/teleport/pull/29991)
* Added support for the web UI for automatically deploying the Database Service with ECS Fargate containers when enrolling a new database. [#29978](https://github.com/gravitational/teleport/pull/29978)
* Added new Prometheus metrics to Kubernetes access. [#29970](https://github.com/gravitational/teleport/pull/29970)
* Added ability to delete proxy resources with `tctl`. [#29903](https://github.com/gravitational/teleport/pull/29903)
* Added headless approval UI to Teleport Connect. [#28975](https://github.com/gravitational/teleport/pull/28975)
* Removed requiring team/channel inputs for mattermost plugin. [#30009](https://github.com/gravitational/teleport/pull/30009)
* Fixed change feed with PostgreSQL backend on Azure [#29911](https://github.com/gravitational/teleport/issues/29911) [#29975](https://github.com/gravitational/teleport/pull/29975)
* Fixed `tctl` to obey `--verbose` when formatting text tables. [#29870](https://github.com/gravitational/teleport/pull/29870)
* Updated OpenSSL to [3.0.10](https://github.com/openssl/openssl/blob/openssl-3.0.10/CHANGES.md#changes-between-309-and-3010-1-aug-2023). [#29908](https://github.com/gravitational/teleport/pull/29908)
* Updated Go to 1.20.7. [#29904](https://github.com/gravitational/teleport/pull/29904)
* Reduced logging level in PostgreSQL backend for improved performance. [#29847](https://github.com/gravitational/teleport/pull/29847)

## 13.3.0 (08/01/23)

New backends

Teleport 13.3 includes a new Postgres backend that supports both
cluster state and the audit log. Additionally, Azure users can now
leverage Azure blob storage for session recordings.

* Added backwards compatibility for listing Apps of an older version leaf cluster [#29816](https://github.com/gravitational/teleport/pull/29816)
* Added classification code and emit event on execution [#29811](https://github.com/gravitational/teleport/pull/29811)
* Added max duration option to Access Request [#29754](https://github.com/gravitational/teleport/pull/29754)
* Refactored Teleport Assist token counting [#29753](https://github.com/gravitational/teleport/pull/29753)
* Added support for displaying onboarding questionnaire for existing users (#29378) [#29713](https://github.com/gravitational/teleport/pull/29713)
* Added flag to write tarred tctl auth sign output to stdout [#29666](https://github.com/gravitational/teleport/pull/29666)
* Added Azure support to Helm charts [#29734](https://github.com/gravitational/teleport/pull/29734)
* Fixed Kubernetes Legacy Proxy heartbeats [#29738](https://github.com/gravitational/teleport/pull/29738)
* Added Postgres backend and Azure session storage [#29705](https://github.com/gravitational/teleport/pull/29705)
* Fixed auth locking issue [#29706](https://github.com/gravitational/teleport/pull/29706)
* Fixed an issue where MachineID sometimes did not work behind L7 LB [#29700](https://github.com/gravitational/teleport/pull/29700)
* Fixed issue where incorrect session recording mode was using during session start and end events [#29689](https://github.com/gravitational/teleport/pull/29689)
* Fixed issue with custom OS checking in device trust authentication [#29629](https://github.com/gravitational/teleport/pull/29629)
* Added GCP VM auto-discovery (#28562) [#29612](https://github.com/gravitational/teleport/pull/29612)

## 13.2.5 (07/27/23)

* Removed alerts suggesting upgrade. [#29631](https://github.com/gravitational/teleport/pull/29631)
* Reduced memory use when migrating events to Athena. [#29604](https://github.com/gravitational/teleport/pull/29604)
* Updated etcd backend load distribution to be more even. [#29586](https://github.com/gravitational/teleport/pull/29586)
* Updated Kubernetes operator CRDs. [#29554](https://github.com/gravitational/teleport/pull/29554)
* Updated `tctl request create` to support `--resource` flag. [#29538](https://github.com/gravitational/teleport/pull/29538)
* Existing tokens can no longer be updated with "create token" access. [#29391](https://github.com/gravitational/teleport/pull/29391)
* Web UI now includes SAML Apps in the Applications list. [#29371](https://github.com/gravitational/teleport/pull/29371)
* DynamoDB backend tables are now created with PayPerRequest mode. [#29351](https://github.com/gravitational/teleport/pull/29351)
* Fixed enhanced recording of missing `session.command` events when PAM enabled. [#29030](https://github.com/gravitational/teleport/issues/29030) [#29578](https://github.com/gravitational/teleport/pull/29578)
* Fixed GCP joining for Machine ID. [#29563](https://github.com/gravitational/teleport/pull/29563)
* Fixed Opsgenie plugin to use v2 API paths. [#29553](https://github.com/gravitational/teleport/pull/29553)
* Fixed a panic in the S3 uploader. [#29470](https://github.com/gravitational/teleport/pull/29470)
* Fixed Database RBAC to take dynamic labels into account. [#29373](https://github.com/gravitational/teleport/pull/29373)
* Fixed memory leak in statistics reporter. [#29330](https://github.com/gravitational/teleport/pull/29330)
* Made `--type` flag required on `tctl auth crl` command. [#29591](https://github.com/gravitational/teleport/pull/29591)
* Added `--silent` flag to `teleport node configure` command. [#29587](https://github.com/gravitational/teleport/pull/29587)
* Added tsh flags `--labels` and `--query` for database resource selection. [#29163](https://github.com/gravitational/teleport/pull/29163)
* Added the `--opensearch-discovery` flag to specify AWS regions. [#28147](https://github.com/gravitational/teleport/pull/28147)
* Added support for Amazon Linux 2023 in installer script and UI [#29654](https://github.com/gravitational/teleport/pull/29654)

## 13.2.3 (07/20/23)

  * Fixed TLS routing bug [#29098](https://github.com/gravitational/teleport/issues/29098) [#29312](https://github.com/gravitational/teleport/pull/29312)
  * Provided warning when `tsh` ignores the `--user` flag due to SSO [#29221](https://github.com/gravitational/teleport/pull/29221)
  * Addressed vulnerability in Kubernetes access preview [#29274](https://github.com/gravitational/teleport/pull/29274)
  * Restored default API endpoint for PagerDuty plugin [#29295](https://github.com/gravitational/teleport/pull/29295)

## 13.2.2 (07/14/23)

* Assist
  * Reduced node polling interval to allow Assist detect new nodes faster. [#29153](https://github.com/gravitational/teleport/pull/29153)
  * Fixed issue with some Assist command executions not being captured in audit log. [#29137](https://github.com/gravitational/teleport/pull/29137)
  * Added various Assist UI tweaks and improvements. [#29067](https://github.com/gravitational/teleport/pull/29067), [#28911](https://github.com/gravitational/teleport/pull/28911)
* Audit Log
  * Suppressed unnecessary resource Access Request events. [#29063](https://github.com/gravitational/teleport/pull/29063)
* Cloud
  * Added ability to manage cluster networking config for Cloud tenants. [#28992](https://github.com/gravitational/teleport/pull/28992)
* CLI
  * Improved `tsh play` error handling. [#29077](https://github.com/gravitational/teleport/pull/29077)
  * Updated `tsh request search` to deduplicate resources. [#28889](https://github.com/gravitational/teleport/pull/28889)
* Database Access
  * Updated `teleport discovery bootstrap` to support setting up Database Service permissions. [#29002](https://github.com/gravitational/teleport/pull/29002)
* Helm Charts
  * Added ingress support to `teleport-cluster` chart. [#29084](https://github.com/gravitational/teleport/pull/29084)
* Stability & Reliability
  * Fixed issue with viewing audit log when using Firestore backend. [#29114](https://github.com/gravitational/teleport/pull/29114)
  * Cleaned up session uploader logging to suppress S3 permission errors. [#29078](https://github.com/gravitational/teleport/pull/29078)
  * Improved database and Kubernetes cluster name validation. [#29035](https://github.com/gravitational/teleport/pull/29035)
* Hosted Plugins
  * Added hosted PagerDuty plugin for Teleport Enterprise (Cloud) users. [#28986](https://github.com/gravitational/teleport/pull/28986)
* Internal
  * Updated Go to `1.20.6`. [#29073](https://github.com/gravitational/teleport/pull/29073)

## 13.2.1 (07/12/23)

* Kubernetes Operator
  * Fixed regression issue with Kube operator crashing on first startup. [#29013](https://github.com/gravitational/teleport/pull/29013)
* Installation
  * Fixed issue with the install script not working on non-systemd systems. [#28987](https://github.com/gravitational/teleport/pull/28987)
  * Fixed issue with RPM packages failing to install on FIPS-enabled RHEL 8 systems. [#28794](https://github.com/gravitational/teleport/pull/28794)
* CLI
  * Fixed issue with `tsh login` not displaying cluster alerts. [#28983](https://github.com/gravitational/teleport/pull/28983)
  * Added ability to provide GitHub API endpoint URL to `tctl sso configure github` command. [#28968](https://github.com/gravitational/teleport/pull/28968)
  * Updated `tctl alerts ack` to make `--reason` flag optional. [#28955](https://github.com/gravitational/teleport/pull/28955)
  * Updated `tctl alerts ls` to always show alert ID. [#28906](https://github.com/gravitational/teleport/pull/28906)
* Desktop Access
  * Improved LDAP connection errors handling. [#28974](https://github.com/gravitational/teleport/pull/28974)
* Access Controls
  * Fixed issue with locking servers via web UI. [#28963](https://github.com/gravitational/teleport/pull/28963)
* Azure
  * Fixed Azure joining for identities across resource groups. [#28961](https://github.com/gravitational/teleport/pull/28961)
* Teleport Assist
  * Updated Assist bot to produce more consistent responses. [#28959](https://github.com/gravitational/teleport/pull/28959)
* Database Access
  * Updated default SQL Server database client to `sqlcmd`. [#28944](https://github.com/gravitational/teleport/pull/28944)
* Web UI
  * Fixed issue with newlines not being displayed properly in message of the day. [#28937](https://github.com/gravitational/teleport/pull/28937)
  * Added Machine ID guides to the Enroll Integration page in the web UI. [#28888](https://github.com/gravitational/teleport/pull/28888)
* Server Access
  * Fixed issue with `SSH_*` environment variables not being respected in headless mode. [#28922](https://github.com/gravitational/teleport/pull/28922)
* Access Plugins
  * Added PagerDuty hosted plugin for Teleport Enterprise (Cloud). [#28883](https://github.com/gravitational/teleport/pull/28883)
* Audit
  * Added ID token attributes to GCP `bot.join` audit event. [#28882](https://github.com/gravitational/teleport/pull/28882)
* Automatic Upgrades
  * Updated `tctl inventory ls` command to show agent auto-upgrade status on Teleport Enterprise (Cloud). [#28847](https://github.com/gravitational/teleport/pull/28847)
* Kubernetes Access
  * Added support for specifying `assume_role_arn` for Kube cluster matchers in auto-discovery. [#28832](https://github.com/gravitational/teleport/pull/28832)
* Machine ID
  * Added GCP delegated joining support. [#28762](https://github.com/gravitational/teleport/pull/28762)
* GCP
  * Fixed issue with GCP joining not working with GKE workload identity. [#28759](https://github.com/gravitational/teleport/pull/28759)
* Stability & Reliability
  * Improved Firestore backend handling for cases when same collection is used for backend data and audit events. [#28737](https://github.com/gravitational/teleport/pull/28737)
* Okta
  * Updated Okta group Access Requests to automatically include list of the group's applications. [#28603](https://github.com/gravitational/teleport/pull/28603)

## 13.2.0 (07/05/23)

* Teleport Assist
  * Improved accuracy of node selection based on the user query. [#28116](https://github.com/gravitational/teleport/pull/28116)
  * Introduced reasoning feedback during the chat loop. [#27075](https://github.com/gravitational/teleport/pull/27075)
  * Added command execution progress feedback and settings UI. [#28480](https://github.com/gravitational/teleport/pull/28480)
* Server Access
  * Added option to preserve automatically created host users instead of deleting them. [#28432](https://github.com/gravitational/teleport/pull/28432)
  * Fixed issue with `tsh join` when per-session MFA is enabled. [#28456](https://github.com/gravitational/teleport/pull/28456)
* Database Access
  * Updated `tsh db connect` to prefer `mongosh` client. [#28668](https://github.com/gravitational/teleport/pull/28668)
  * Fixed issue with database agents not respecting graceful shutdown. [#28369](https://github.com/gravitational/teleport/pull/28369)
* Device Trust
  * Added Jamf integrations for inventory management. [teleport.e#1763](https://github.com/gravitational/teleport.e/pull/1763)
* Kubernetes Operator
  * Added Okta import rules support. [#28377](https://github.com/gravitational/teleport/pull/28377)
  * Fixed issue with recreating a bot that was previously partially removed. [#28543](https://github.com/gravitational/teleport/pull/28543)
* Teleport Connect
  * Added light theme. [#28277](https://github.com/gravitational/teleport/pull/28277)
* RBAC
  * Added support for RBAC label expressions. [#27641](https://github.com/gravitational/teleport/pull/27641)
* TLS Routing
  * Added IP pinning support for TLS routing behind ALB mode. [#28466](https://github.com/gravitational/teleport/pull/28466)
* Stability & Reliability
  * Fixed issue with invalid database resources preventing cache initialization. [#28638](https://github.com/gravitational/teleport/pull/28638)
* Web UI
  * Added light & dark themes to YAML editor. [#28517](https://github.com/gravitational/teleport/pull/28517)
  * Added light & dark themes to web terminal. [#28408](https://github.com/gravitational/teleport/pull/28408)

## 13.1.5 (06/27/23)

* Teleport Enterprise (Cloud)
  * Added Opsgenie hosted plugin. [#28098](https://github.com/gravitational/teleport/pull/28098)
  * Fixed issue with the install script sometimes failing to install Teleport during Cloud upgrades. [#28208](https://github.com/gravitational/teleport/pull/28208)
* Kubernetes Operator
  * Added support for label expressions to Kubernetes operator. [#28156](https://github.com/gravitational/teleport/pull/28156)
* Web UI
  * Ensured message of the day is displayed in the web UI.
    [#27922](https://github.com/gravitational/teleport/pull/27922)
  * Ensured that the Web UI does not make calls to Stripe for self-hosted
    customers.
    [teleport.e#1724](https://github.com/gravitational/teleport.e/pull/1724)
  * Remove Stripe from the CSP for self-hosted deployments
    [#28308](https://github.com/gravitational/teleport/pull/28308)
* Server Access
  * Ensured that keys are not added to the agent during headless login
    [#28236](https://github.com/gravitational/teleport/pull/28236)
* Kubernetes Access
  * Fixed a bug that could prevent some `kubernetes_resource` deny rules from
    being enforced
    [#28285](https://github.com/gravitational/teleport/pull/28285)
  * Ensure that `kubernetes_users` are properly recorded in the audit log when
    using `tsh kubectl --as`
    [#28323](https://github.com/gravitational/teleport/pull/28323)
* Application Access
  * Ensure that the URL's original query string is preserved even when
    reauthentication is necessary
    [#28218](https://github.com/gravitational/teleport/pull/28218)
* Database Access
  * Support a new `assume_role_arn` setting, allowing you to assume a particular
    AWS role when accessing a database
    [#28210](https://github.com/gravitational/teleport/pull/28210)
* Stability & Reliability
  * Fixed a bug causing the client idle timeout to be enforced prematurely
    [#28202](https://github.com/gravitational/teleport/pull/28202)
  * Improved routing of connections between agents and Auth Servers when proxy
    peering is enabled
    [#28316](https://github.com/gravitational/teleport/pull/28316)
  * Add a `max_session_ttl` option to Teleport's `cluster_auth_preference`
    [#28130](https://github.com/gravitational/teleport/pull/28130)

## 13.1.2 (06/21/23)

* Teleport Assist
  * Introduced new Assist web UI. [#27791](https://github.com/gravitational/teleport/pull/27791)
  * Improved OpenAI error handling. [#27935](https://github.com/gravitational/teleport/pull/27935)
* Access
  * Added `reviewer` and `requester` preset roles. [#28076](https://github.com/gravitational/teleport/pull/28076)
* Teleport Connect
  * Fixed issue with overlapping placeholder and keyboard shortcut in the search bar. [#28048](https://github.com/gravitational/teleport/pull/28048)
  * Updated resource filter ordering in the search bar. [#28034](https://github.com/gravitational/teleport/pull/28034)
* Helm Charts
  * Updated `teleport-cluster` chart to use local Auth Service address in the Auth Service pod to prevent extra connections. [#27980](https://github.com/gravitational/teleport/pull/27980)
  * Added support for `hostAlias` in `teleport-kube-agent` chart. [#27880](https://github.com/gravitational/teleport/pull/27880)
* Server Access
  * Fixed issue with `tsh` prompting for a password when joining invalid sessions. [#27974](https://github.com/gravitational/teleport/pull/27974)
  * Fixed issue with `SSH_SESSION_WEBPROXY_ADDR` not being set for some sessions. [#27865](https://github.com/gravitational/teleport/pull/27865)
* Device Trust
  * Updated `tsh` to prompt user for privilege elevation during TPM enrollment. [#27959](https://github.com/gravitational/teleport/pull/27959)
* Web UI
  * Added "Add SAML application" wizard to access management UI. [#27949](https://github.com/gravitational/teleport/pull/27949)
* Database Access
  * Added support for OpenSearch auto-discovery. [#27942](https://github.com/gravitational/teleport/pull/27942)
* IP Pinning
  * Fixed issue with SSO logins via web UI not working when IP pinning is enabled. [#27896](https://github.com/gravitational/teleport/pull/27896)
* Stability & Reliability
  * Improved shutdown stability. [#27887](https://github.com/gravitational/teleport/pull/27887)
* Desktop Access
  * Fixed issue with "Run as different user" window freezing. [#27874](https://github.com/gravitational/teleport/pull/27874)
* CLI
  * Added `--skip-confirm` flag to `tsh headless approve` command. [#27864](https://github.com/gravitational/teleport/pull/27864)
* Tooling
  * Updated Go to `1.20.5`. [#27860](https://github.com/gravitational/teleport/pull/27860)
* Metrics
  * Improved `backend_read_seconds` metric accuracy. [#27857](https://github.com/gravitational/teleport/pull/27857)
* TLS Routing
  * Fixed issue with ALPN handshake test not respecting `HTTPS_PROXY`. [#27810](https://github.com/gravitational/teleport/pull/27810)
* Okta
  * Updated Okta Access Requests to display app/group names instead of IDs. [#27803](https://github.com/gravitational/teleport/pull/27803)

## 13.1.1 (06/14/23)

* Access
  * Fixed "listing app servers for Okta access calculation" regression issue in `tsh login`. [#27839](https://github.com/gravitational/teleport/pull/27839)
* Performance & Scalability
  * Reduced reverse tunnels thundering herd effect on proxy restarts. [#27786](https://github.com/gravitational/teleport/pull/27786), [#27699](https://github.com/gravitational/teleport/pull/27699)
* Machine ID
  * Improved secure write support detection on some systems. [#27784](https://github.com/gravitational/teleport/pull/27784)
* Database Access
  * Added support for AWS IAM auth for MongoDB Atlas. [#27494](https://github.com/gravitational/teleport/pull/27494)
  * Hardened MongoDB protocol. [#27741](https://github.com/gravitational/teleport/pull/27741)
* Kubernetes Access
  * Fixed issue with `tsh kube login` requiring the use of local proxy in non TLS routing mode. [#27732](https://github.com/gravitational/teleport/pull/27732)
* Teleport Connect
  * Fixed issue with role assumption not working correctly. [#27723](https://github.com/gravitational/teleport/pull/27723)
* RBAC
  * Added support for RBAC label expressions. [#27641](https://github.com/gravitational/teleport/pull/27641)
  * Updated locking to support any service types. [#27442](https://github.com/gravitational/teleport/pull/27442)
* Helm Charts
  * Added conditional RBAC/ServiceAccount to `teleport-kube-agent` post-delete hook. [#27637](https://github.com/gravitational/teleport/pull/27637)
* Audit Log
  * Added login rules to Github login event. [#27607](https://github.com/gravitational/teleport/pull/27607)
* Web UI
  * Fixed issue with not being able to "login" with auth type set to SSO but no connectors set yet. [#27589](https://github.com/gravitational/teleport/pull/27589)
  * Fixed "non-positive parameter limit" error when adding RDS database in some cases. [#27415](https://github.com/gravitational/teleport/pull/27415)
* Server Access
  * Updated proxy templates to prioritize cluster value provided in the template. [#27581](https://github.com/gravitational/teleport/pull/27581)
  * Fixed issue with incorrect proxy port being used in SSH config in some cases. [#27545](https://github.com/gravitational/teleport/pull/27545)
  * Fixed issue with using `tsh` from a `tsh ssh` session. [#27507](https://github.com/gravitational/teleport/pull/27507)
  * Fixed issue with incorrect `SSH_SESSION_WEBPROXY_ADDR` in Web UI SSH sessions. [#27420](https://github.com/gravitational/teleport/pull/27420)
* Automatic Upgrades
  * Fixed the default Cloud upgrade server in `teleport-kube-agent` Helm chart. [#27572](https://github.com/gravitational/teleport/pull/27572)
* AMIs
  * Added support for hardened AMIs. [#27454](https://github.com/gravitational/teleport/pull/27454)
* Machine ID
  * Added Prometheus endpoint to `tbot`. [#27432](https://github.com/gravitational/teleport/pull/27432)
* Application Access
  * Added support for `--cluster` flag to `tsh app login`. [#27197](https://github.com/gravitational/teleport/pull/27197)

## 13.1.0 (06/05/23)

* SAML IdP
  * Fixed issue with SAML IdP advertising incorrect public address in some cases. [#27376](https://github.com/gravitational/teleport/pull/27376)
  * Fixed issue with SAML IdP authentication failing on subsequent attempts. [#27314](https://github.com/gravitational/teleport/pull/27314)
* Kubernetes Access
  * Fixed issue with simultaneous Kubernetes re-logins opening multiple browser tabs. [#27366](https://github.com/gravitational/teleport/pull/27366)
* Teleport Assist
  * Added Teleport Assist for Teleport Enterprise (Cloud) users on Team plan. [#27243](https://github.com/gravitational/teleport/pull/27243)
  * Fixed issue with intermittent connection reset and multiple UX tweaks. [#27356](https://github.com/gravitational/teleport/pull/27356)
* Stability
  * Fixed issue with stalled Auth Service initialization when the backend is unavailable. [#27298](https://github.com/gravitational/teleport/pull/27298)
* Web UI
  * Fixed issue with immediate web UI logout after successful login. [#27296](https://github.com/gravitational/teleport/pull/27296)
  * Fixed issue with resource names sometimes now showing up in Access Requests. [#27430](https://github.com/gravitational/teleport/pull/27430)
* CLI
  * Added support for creating Windows desktops via `tctl`. [#27250](https://github.com/gravitational/teleport/pull/27250)
  * Fixed issue with `tctl get all` not returning locks. [#27294](https://github.com/gravitational/teleport/pull/27294)
* Server Access
  * Fixed issue with Access Requests in headless mode. [#27241](https://github.com/gravitational/teleport/pull/27241)
  * Fixed issue with port forwarding configuration being cached in `tsh` profile. [#27208](https://github.com/gravitational/teleport/pull/27208)
* Database Access
  * Added support for automatic database user provisioning for PostgreSQL. [#26555](https://github.com/gravitational/teleport/pull/26555)
  * Updated Elasticache support to automatically include IAM connect permissions. [#27188](https://github.com/gravitational/teleport/pull/27188)
* Desktop Access
  * Fixed issue with automatic user creation failing in some cases. [teleport.e#1579](https://github.com/gravitational/teleport.e/pull/1579)

## 13.0.4 (05/31/23)

* Application Access
  * Updated `tsh proxy app` to not require explicit `tsh app login`. [#26820](https://github.com/gravitational/teleport/pull/26820)
* Auth
  * Fixed issue with headless authentication not working when leaf cluster is selected. [#26878](https://github.com/gravitational/teleport/pull/26878)
  * Fixed issue with GitHub Enterprise connector API endpoint URL path getting ignored. [#26863](https://github.com/gravitational/teleport/pull/26863)
* CLI
  * Added `tsh kubectl` support for tracer exporter. [#27130](https://github.com/gravitational/teleport/pull/27130)
  * Added support for bash and zsh autocompletion. [#26999](https://github.com/gravitational/teleport/pull/26999)
  * Added support for calling `tctl alert` commands remotely. [#26789](https://github.com/gravitational/teleport/pull/26789)
* Database Access
  * Added support for ElastiCache Redis IAM authentication. [#26990](https://github.com/gravitational/teleport/pull/26990)
* Desktop Access
  * Increased LDAP dial timeout from 5 to 15 seconds. [#27045](https://github.com/gravitational/teleport/pull/27045)
  * Improved LDAP error reporting. [#26984](https://github.com/gravitational/teleport/pull/26984)
* Helm Charts
  * Improved `clusterName` validation in `teleport-cluster` Helm chart. [#26973](https://github.com/gravitational/teleport/pull/26973)
* Kubernetes Operator
  * Fixed access denied issue when creating token resources. [#27001](https://github.com/gravitational/teleport/pull/27001)
* Okta
  * Updated Okta import rules regex to support glob matching. [#27126](https://github.com/gravitational/teleport/pull/27126)
  * Updated Okta import rules to support filtering user groups by description. [#27021](https://github.com/gravitational/teleport/pull/27021)
* Performance & Scalability
  * Improved `tsh login` latency by making sure cluster alerts are fetched once. [#27110](https://github.com/gravitational/teleport/pull/27110)
* Server Access
  * Added CA rotation support to EC2 OpenSSH discovery. [#26888](https://github.com/gravitational/teleport/pull/26888)
  * Extended Proxy Templates support to `tsh ssh`. [#26852](https://github.com/gravitational/teleport/pull/26852)
  * Fixed issue where system agent is not forwarded when using `--add-keys-to-agent=no`. [#26929](https://github.com/gravitational/teleport/pull/26929)
* Tooling
  * Upgraded OpenSSL to `3.0.9`. [#27123](https://github.com/gravitational/teleport/pull/27123)
* TLS Routing
  * Added multiple UX improvements for `tsh kube` commands in TLS Routing mode behind ALB. [#27155](https://github.com/gravitational/teleport/pull/27155)
* Web UI
  * Added back buttons to Access Management and Integrations flows where possible. [#26727](https://github.com/gravitational/teleport/pull/26727)
  * Fixed issue with pagination buttons sometimes being invisible on the Nodes page. [#26906](https://github.com/gravitational/teleport/pull/26906)

## 13.0.3 (05/24/23)

* Access Management
  * Added macOS arm64 support to the node install script. [#26504](https://github.com/gravitational/teleport/pull/26504), [#26698](https://github.com/gravitational/teleport/pull/26698)
  * Restored the "Add application" dialog. [#26457](https://github.com/gravitational/teleport/pull/26457)
  * Updated node install script to respect cluster version. [#26322](https://github.com/gravitational/teleport/pull/26322)
* Audit Log
  * Updated Okta events to include app and group names. [#26370](https://github.com/gravitational/teleport/pull/26370)
  * Updated user login event to add list of applied login rules. [#26474](https://github.com/gravitational/teleport/pull/26474)
* Desktop Access
  * Improved internal logging and user lookup efficiency. [#26413](https://github.com/gravitational/teleport/pull/26413)
* Okta Access
  * Updated Okta import rules to support regex group and app name matching. [#26799](https://github.com/gravitational/teleport/pull/26799)
* Server Access
  * Fixed issue with SSH sessions sometimes failing to start when enhanced session recording is enabled. [#26728](https://github.com/gravitational/teleport/pull/26728)
  * Fixed issue with port forwarding silently failing when using a label based target. [#26701](https://github.com/gravitational/teleport/pull/26701)
  * Added certificate rotation support to `teleport join openssh` command. [#26674](https://github.com/gravitational/teleport/pull/26674)
  * Added support for using hostnames in OpenSSH node resources in addition to IPs. [#26549](https://github.com/gravitational/teleport/pull/26549)
* Kubernetes Access
  * Extended `kubectl auth can-i` support to consider `kubernetes_resources` RBAC rules. [#26584](https://github.com/gravitational/teleport/pull/26584)
* Kubernetes Operator
  * Added `ProvisionToken` support. [#26618](https://github.com/gravitational/teleport/pull/26618)

## 13.0.2 (05/17/23)

* Auth
  * Improved error message on `tsh login` when trying to authenticate with unregistered device. [#26103](https://github.com/gravitational/teleport/pull/26103)
  * Populate `locked_time` user status value when local user is locked. [#26255](https://github.com/gravitational/teleport/pull/26255)
* Machine ID
  * Fixed goroutine leak in `tbot`. [#26125](https://github.com/gravitational/teleport/pull/26125)
  * Added pprof diagnostics endpoints to `tbot`. [#26117](https://github.com/gravitational/teleport/pull/26117)
* Audit Log
  * Added audit events for Okta integration. [#26000](https://github.com/gravitational/teleport/pull/26000)
  * Do not include empty Windows domains in audit log for desktop access. [#26078](https://github.com/gravitational/teleport/pull/26078)
* CLI
  * Added `--format` flag to `tctl alerts ls` command and include acknowledged alerts in verbose mode. [#26040](https://github.com/gravitational/teleport/pull/26040)
  * Added `tsh fido2 attobj` debug command that can parse attestation objects. [#25923](https://github.com/gravitational/teleport/pull/25923)
  * Fixed issue with `tsh` version not being reflected in the macOS application bundle info. [#26314](https://github.com/gravitational/teleport/pull/26314)
* IP Pinning
  * Fixed issue with `tctl` commands not working when IP pinning is enabled. [#25993](https://github.com/gravitational/teleport/pull/25993)
* Terraform
  * Fixed issue with ACL being disabled for new buckets in non-HA terraform setup. [#25854](https://github.com/gravitational/teleport/pull/25854)
* Performance & Scalability
  * Added ability to enable trace logging level. [#25833](https://github.com/gravitational/teleport/pull/25833)
* Helm Charts
  * Fixed issue with invite token being incorrectly overridden when it was manually created. [#26175](https://github.com/gravitational/teleport/pull/26175)
* HSM
  * Added support for YubiHSM2 SDK version 2023.01. [#25816](https://github.com/gravitational/teleport/pull/25816)
* Teleport Connect
  * Disabled "Open new terminal" action if there's no active workspace. [#26333](https://github.com/gravitational/teleport/pull/26333)
  * Fixed issue with cluster filter not being taken into account when listing offline clusters. [#26127](https://github.com/gravitational/teleport/pull/26127)
  * Added support for TLS Routing behind ALB support for SSH and Database access. [#25899](https://github.com/gravitational/teleport/pull/25899)
* Database Access
  * Improved initial connect auth check for Azure Cache access. [#26317](https://github.com/gravitational/teleport/pull/26317)
  * Fixed issue with connecting to default active Cassandra database. [#26378](https://github.com/gravitational/teleport/pull/26378)
* TLS Routing
  * Added support for `tsh kube join` in single-port mode behind ALB. [#26283](https://github.com/gravitational/teleport/pull/26283)
  * Added support for `tsh request search --kind=pod` in single-port mode behind ALB. [#26128](https://github.com/gravitational/teleport/pull/26128)
* Kubernetes Access
  * Fixed panic when using proxy peering. [#26174](https://github.com/gravitational/teleport/pull/26174)
* GCP
  * Added GCP IAM joining method. [#26165](https://github.com/gravitational/teleport/pull/26165)
* Desktop Access
  * Fixed issue with directory sharing not working. [#26090](https://github.com/gravitational/teleport/pull/26090)

## 13.0.1 (05/16/23)

* Helm Charts
  * Fixed issue with invite token being incorrectly overridden when it was manually created. [#26055](https://github.com/gravitational/teleport/pull/26055)

### Breaking Changes

Please familiarize yourself with the following potentially disruptive changes in
Teleport 13 before upgrading.

#### Teleport Kubernetes Agent helm chart

When upgrading to Teleport 13, users of the Teleport Kubernetes Agent Helm chart
that manually create their own Teleport token secret (`secretName=<secretName>` and no auth token provided)
will need to set the following values:

```yaml
# Manages the join token secret creation and its name.
joinTokenSecret:
  # create controls whether the Helm chart should create and manage the join token
  # secret.
  # If false, the chart assumes that the secret with the configured name already exists at the
  # installation namespace.
  create: false
  # Name of the Secret to store the teleport join token.
  name: <secretName>
```

The Helm chart parameter `secretName` was deprecated in Teleport 13 in favor of
`joinTokenSecret.name`. `joinTokenSecret.create` indicates whether the Helm
chart should create and manage the join token secret. If `create` is set to
`false`, the chart assumes that the secret with the configured name already
exists at the installation namespace.

## 13.0.0 (05/08/23)

Teleport 13 brings the following marquee features and improvements:

* (Preview) Automatic agent upgrades.
* (Preview) TLS routing through ALB for accessing servers, Kubernetes clusters, and applications.
* (Preview, Enterprise-only) Ability to import applications and groups from Okta.
* (Preview) Teleport support for AWS OpenSearch.
* (Preview) View and control access to OpenSSH nodes natively in Teleport.
* Cross-cluster search for Teleport Connect.
* Performance improvements for accessing Kubernetes clusters.
* Universal binaries (including Apple Silicon) for macOS.
* Simplified RDS onboarding flow in Access Management UI.
* Light theme for Web UI.

### (Preview) Automatic agent upgrades

In Teleport 13 users can configure their Teleport agents deployed via apt/yum
repositories or a Helm chart to be upgraded automatically.

### (Preview) TLS routing through ALB accessing servers, Kubernetes clusters, and applications

Teleport 13 adds single-port TLS routing mode support for servers, Kubernetes
clusters, and applications for clusters deployed behind application layer load
balancers such as AWS ALB.

### (Preview, Enterprise-only) Ability to import applications and groups from Okta

In Teleport 13  users can import apps and groups from Okta and use Teleport
Access Requests for requesting short-term access to them. This feature is only
available in the Teleport Enterprise edition.

### (Preview) Teleport support for AWS OpenSearch

Teleport users can now connect to AWS OpenSearch databases.

### (Preview) View and control access to OpenSSH nodes natively in Teleport

In Teleport 13 users will be able register OpenSSH nodes as a resource with the
cluster.

This will allow users to view the OpenSSH nodes in Web UI and using `tsh ls`
and use RBAC to control access to them.

See the updated [OpenSSH integration
guide](docs/pages/enroll-resources/server-access/openssh/openssh-agentless.mdx).

### Cross-cluster search for Teleport Connect

Teleport Connect now includes a new search experience, allowing you to search
for and connect to resources across all logged-in clusters.

### Performance improvements for accessing Kubernetes clusters

In Teleport 13 we improved the way the Teleport Proxy Service handles Kubernetes
credentials.

Users will experience better performance when interacting with Kubernetes
clusters using kubectl or via the API.

### Universal binaries (including Apple Silicon) for macOS

Teleport 13 binaries (including Teleport Connect) will have universal
architecture and run natively on both Intel and ARM macOS systems.

### Simplified RDS onboarding flow in Access Management UI

When connecting an RDS database using Teleport 13 Access Management UI, users
can connect their AWS account and select the RDS database to add instead of
entering details manually.

To try out the new flow, add an RDS database using the Resource Management UI
in your cluster’s Web UI dashboard.

### Light theme for Web UI

Teleport's web UI includes an optional light theme.

The light theme is enabled by default but can be changed back to the dark theme
via the top-right corner user settings menu.

### Windows desktop session recording export

Session recordings for Windows desktop sessions can now be exported to video
format for offline playback with the new tsh recordings export command.

### SFTP in Moderated Sessions

Teleport 13 adds the ability to transfer files in Moderated Sessions.
This feature requires that both the session originator and the moderator
have joined the session via the web UI.

### Breaking changes

Please familiarize yourself with the following potentially disruptive changes
in Teleport 13 before upgrading.

#### Default session join mode

Teleport 13 defaults to observer (read-only) mode when joining SSH and Kubernetes
sessions. Prior versions of Teleport would default to peer mode for SSH sessions
and moderator mode for Kubernetes sessions. To override the default join mode,
specify the --mode flag with tsh join.

#### CA rotation deprecation

Teleport 13 removes support for rotating all certificate authorities with
`tctl auth rotate --type=all`. The `type` flag is now required, which ensures
that only one CA is rotated at a time, increasing cluster stability during
rotations.

#### Join token API changes

The default 30-minute expiry no longer applies to tokens created via YAML
resource files. If you want to enforce an expiration, ensure this is set in the
`metadata.expires` field. Tokens created using `tctl nodes add` and `tctl tokens add`
will continue to have a default 30m expiry applied.

Additionally, users of Teleport’s API module will note that the `CreateToken`
and `UpsertToken` RPCs are now deprecated in favor of `CreateTokenV2` and
`UpsertTokenV2`. The new V2 variants no longer have a default expiry, so be sure
to set a TTL if you want your tokens to expire.

The original RPCs are still supported in Teleport 13 and will be removed
completely for Teleport 14.

#### Enhanced user validation

Teleport 13 will refuse to create or update users that reference non-existent
roles. In some circumstances, older versions of Teleport would permit you to
create users and assign them invalid roles. In Teleport 13 this is a hard error.

#### Quay.io registry

Quay.io registry was deprecated in Teleport 11 and starting with Teleport 13,
Teleport container images are no longer being published to it.

Users should use the [public ECR
registry](https://gallery.ecr.aws/gravitational).

#### Helm chart uses `distroless`-based container image by default

Starting with Teleport 13, the Helm charts `teleport-cluster` and `teleport-kube-agent`
are deploying distroless Teleport images by default. Those images are slimmer
and more secure but contain less tooling (e.g. neither `bash` nor
`apt` are available).

The Debian-based images are deprecated and will be removed in Teleport 14.
The chart image can be reverted back to the Debian-based images by setting:
```yaml
image: "public.ecr.aws/gravitational/teleport"
```

For debugging purposes, a "debug" image is available and contains BusyBox,
which includes a shell and most common POSIX executables:
`public.ecr.aws/gravitational/teleport-distroless-debug`.

## 12.3.0 (05/01/23)

This release of Teleport contains multiple improvements and bug fixes.

* Desktop Access
  * Added support for automatic Windows user creation. [#25348](https://github.com/gravitational/teleport/pull/25348)
* CLI
  * Fixed MFA permission denied error from `tsh` for non-SSH protocols. [#25430](https://github.com/gravitational/teleport/pull/25430)
* Terraform
  * Fixed `AccessControlListNotSupported` error in HA terraform. [#25335](https://github.com/gravitational/teleport/pull/25335)
* Device Trust
  * Updated Device Trust audit events to have descriptive types. [#25320](https://github.com/gravitational/teleport/pull/25320)

## 12.2.5 (04/28/23)

This release of Teleport contains multiple improvements and bug fixes.

* Auth
  * Fixed issue where Github SSO would fail if a user is a part of more than 30 teams. [#25098](https://github.com/gravitational/teleport/pull/25098)
  * Fixed issue with `tsh login` with "required" hardware key policy returning "policy not met" error. [#24956](https://github.com/gravitational/teleport/pull/24956)
  * Improved Device Trust logging and error reporting. [#24912](https://github.com/gravitational/teleport/pull/24912)
  * Detect and warn about RPID changes when using WebAuthn. [#25289](https://github.com/gravitational/teleport/pull/25289)
* Access Management
  * Fixed issue with running install script on macOS for enterprise clusters. [#25076](https://github.com/gravitational/teleport/pull/25076)
* Server Access
  * Fixed issue with headless `tsh ssh` not working when used in `rsync -rsh`. [#25242](https://github.com/gravitational/teleport/pull/25242)
  * Fixed issue with headless `tsh ssh` prompting users for MFA. [#25187](https://github.com/gravitational/teleport/pull/25187)
  * Fixed issue with `tsh ssh` failing to connect over public address with per-session MFA. [#25223](https://github.com/gravitational/teleport/pull/25223)
  * Fixed issue with `tsh scp` failing on some destination paths. [#24861](https://github.com/gravitational/teleport/pull/24861)
  * Require explicit username in headless `tsh ssh`. [#25112](https://github.com/gravitational/teleport/pull/25112)
  * Updated automatic user provisioning to sort sudoers lines by role name to ensure stable order. [#24792](https://github.com/gravitational/teleport/pull/24792)
  * Updated `tsh` commands to recognize `SSH_` environment variables. [#24470](https://github.com/gravitational/teleport/pull/24470)
* Database Access
  * Fixed issue with `tsh db env` and `tsh db config` not recognizing separate MySQL listener. [#24827](https://github.com/gravitational/teleport/pull/24827)
* Kubernetes Access
  * Added `--set-context` flag to `tsh kube login` to allow overriding default context name. [#25253](https://github.com/gravitational/teleport/pull/25253)
* IdP
  * Fixed issue with SAML IdP not being disabled properly. [#25309](https://github.com/gravitational/teleport/pull/25309)
* IP Pinning
  * Fixed interoperability issues with load balancers with proxy protocol v2 enabled. [#25302](https://github.com/gravitational/teleport/pull/25302)
* CLI
  * Fixed issue with cluster alerts sometimes not showing up after `tsh login`. [#25300](https://github.com/gravitational/teleport/pull/25300)
* AMIs
  * Fixed issue with startup script failing to acquire lock from AWS metadata. [#25296](https://github.com/gravitational/teleport/pull/25296)
* HSM
  * Fixed issue with inadvertent deletion of active HSM keys when using YubiHSM2 SDK version 2023.1. [#25208](https://github.com/gravitational/teleport/pull/25208)
* Performance & Scalability
  * Improved performance of MFA ceremony. [#24804](https://github.com/gravitational/teleport/pull/24804)

## 12.2.4 (04/18/23)

This release of Teleport contains multiple improvements and bug fixes.

* Auto-discovery
  * Added ability to specify discovery group for Discovery Services. [#24716](https://github.com/gravitational/teleport/pull/24716)
* CLI
  * Improved `tsh` performance on some Windows systems. [#24573](https://github.com/gravitational/teleport/pull/24573)
  * Improved `teleport configure` error/warning reporting. [#24676](https://github.com/gravitational/teleport/pull/24676)
  * Added `--raw` flag to `teleport version` command. [#24772](https://github.com/gravitational/teleport/pull/24772)
* Configuration
  * Prevent proxies from trying to join cluster over reverse tunnel. [#24668](https://github.com/gravitational/teleport/pull/24668)
* Server Access
  * Fixed issue with excessive audit logging when copying files over SFTP. [#24831](https://github.com/gravitational/teleport/pull/24831)
  * Fixed issue with `tsh scp` not recognizing wildcard patterns. [#24831](https://github.com/gravitational/teleport/pull/24831)
  * Fixed issue with `tsh scp` failing when max sessions is set to 1. [#24831](https://github.com/gravitational/teleport/pull/24831)
  * Improved error reporting from `tsh scp` when file copying is disabled. [#24831](https://github.com/gravitational/teleport/pull/24831)
* Kubernetes Access
  * Fixed issue with `tctl auth sign` not respecting `kube_public_addr`. [#24516](https://github.com/gravitational/teleport/pull/24516)
  * Fixed memory leak when using port forwarding. [#24763](https://github.com/gravitational/teleport/pull/24763)
  * Reduced log spam when using port forwarding. [#24658](https://github.com/gravitational/teleport/pull/24658)
* Database Access
  * Updated `teleport db configure` to support more AWS databases. [#24494](https://github.com/gravitational/teleport/pull/24494)
* Performance & Scalability
  * Reduced thundering herd effect in large clusters. [#24719](https://github.com/gravitational/teleport/pull/24719)
* Web UI
  * Fixed issue with downloading files from leaf clusters when per-session MFA is enabled. [#24768](https://github.com/gravitational/teleport/pull/24768)

## 12.2.3 (04/13/23)

This release of Teleport contains multiple bug fixes.

* CLI
  * Fixed potential panic in `tsh ssh`. [#24490](https://github.com/gravitational/teleport/pull/24490)
* Performance & Scalability
  * Improved `tsh ssh` latency. [#24371](https://github.com/gravitational/teleport/pull/24371)
* Kubernetes Access
  * Fixed issue with moderator joining session on a cluster they don't have access to. [#23993](https://github.com/gravitational/teleport/pull/23993)
* Security
  * Added IP pinning support to SSO users. [#24541](https://github.com/gravitational/teleport/pull/24541)

## 12.2.2 (04/12/23)

This release of Teleport contains multiple improvements and bug fixes.

* Server Access
  * Restored `MajorVersion` template variable for EC2 install scripts. [#24434](https://github.com/gravitational/teleport/pull/24434)
  * Added `--mlock` flag to headless `tsh` mode to allow memory locking. [#24410](https://github.com/gravitational/teleport/pull/24410)
  * Fixed issue with EC2 install script silently failing on errors. [#24034](https://github.com/gravitational/teleport/pull/24034)
* Database Access
  * Reduced log spam when AWS database engine name is not recognized. [#24413](https://github.com/gravitational/teleport/pull/24413)
* Machine ID
  * Improved post-renewal message by logging correct identity. [#24246](https://github.com/gravitational/teleport/pull/24246)
* Kubernetes Access
  * Fixed issue with incorrect status being returned on exec commands. [#24155](https://github.com/gravitational/teleport/pull/24155)
* Proxy Peering
  * Improved agent reconnect speed with proxy peering. [#24141](https://github.com/gravitational/teleport/pull/24141)
* Helm Charts
  * Fixed issue with `securityContext` and `nodeSelector` not being propagated to job hooks. [#24134](https://github.com/gravitational/teleport/pull/24134)
  * Fixed issue with TLS routing being disabled after v12 upgrade when `proxyListenerMode` is empty. [#24426](https://github.com/gravitational/teleport/pull/24426)

## 12.2.1 (04/04/23)

This release of Teleport contains several new features and improvements.

* Server Access
  * Added support for headless SSO to `tsh ls`, `tsh ssh` and `tsh scp`. [#23360](https://github.com/gravitational/teleport/pull/23360)
* Database Access
  * Added support for connecting to Oracle databases. [#23892](https://github.com/gravitational/teleport/pull/23892)
* Moderated Sessions
  * Fixed issue with joining moderated sessions via Web UI. [#24018](https://github.com/gravitational/teleport/pull/24018)
* Helm Charts
  * Added support for `imagePullSecrets` to `teleport-cluster` chart. [#24017](https://github.com/gravitational/teleport/pull/24017)
* Security
  * Added IP pinning support to Kubernetes and database access. [#23418](https://github.com/gravitational/teleport/pull/23418)
* Tooling
  * Upgraded Go to `1.20.3`. [#24062](https://github.com/gravitational/teleport/pull/24062)

## 12.1.5 (03/30/23)

This release of Teleport contains 2 security fixes as well as multiple improvements and bug fixes.

### [High] OS authorization bypass in SSH tunneling

When establishing an SSH port forwarding connection, Teleport did not
sufficiently validate the specified OS principal.

This could allow an attacker in possession of valid cluster credentials to
establish a TCP tunnel to a node using a non-existent Linux user.

The connection attempt would show up in the audit log as a "port" audit event
(code T3003I) and include Teleport username in the "user" field.

### [High] Teleport authorization bypass in Kubernetes cluster access

When authorizing a request to a Teleport-protected Kubernetes cluster, Teleport
did not adequately validate the target Kubernetes cluster.

This could allow an attacker in possession of valid Kubernetes agent credentials
or a join token to trick Teleport into forwarding requests to a different
Kubernetes cluster.

Every Kubernetes request would show up in the audit log as a "kube.request"
audit event (code T3009I) and include the Kubernetes cluster metadata.

### Other improvements and fixes

* AMIs
  * Added support for configuring TLS routing mode in AMIs. [#23678](https://github.com/gravitational/teleport/pull/23678)
* Application Access
  * Added support for application access behind ALB. [#23054](https://github.com/gravitational/teleport/pull/23054)
  * Fixed requests to Teleport-protected applications being redirected to leaf's public address in some cases. [#23220](https://github.com/gravitational/teleport/pull/23220)
  * Reduced log noise. [#23365](https://github.com/gravitational/teleport/pull/23365)
  * Added ability to specify command in AWS `tsh` proxy. [#23835](https://github.com/gravitational/teleport/pull/23835)
* Bootstrap
  * Added provision tokens support. [#23474](https://github.com/gravitational/teleport/pull/23474)
* CLI
  * Added `app_server` support to `tctl` resource commands. [#23136](https://github.com/gravitational/teleport/pull/23136)
  * Display year in `tctl` commands output. [#23371](https://github.com/gravitational/teleport/pull/23371)
  * Fixed issue with `tsh` reporting errors about missing webauthn.dll on Windows. [#23161](https://github.com/gravitational/teleport/pull/23161)
  * Updated `tsh status` to not display internal logins. [#23411](https://github.com/gravitational/teleport/pull/23411)
  * Added `--cluster` flag to `tsh kube sessions` command. [#23825](https://github.com/gravitational/teleport/pull/23825)
  * Fixed issue with invalid TLS mode when creating database resources. [#23808](https://github.com/gravitational/teleport/pull/23808)
* Database Access
  * Added support for canceling in-progress PostgreSQL requests in database access. [#23467](https://github.com/gravitational/teleport/pull/23467)
  * Fixed issue with query audit events always having `success: false` status. [#23274](https://github.com/gravitational/teleport/pull/23274)
* Desktop Access
  * Updated setup script to be idempotent. [#23176](https://github.com/gravitational/teleport/pull/23176)
* Helm Charts
  * Added ability to set resource limits and requests for pre-deployment jobs. [#23126](https://github.com/gravitational/teleport/pull/23126)
* Infrastructure
  * Introduced distroless Teleport container images. [#22814](https://github.com/gravitational/teleport/pull/22814)
* Kubernetes Access
  * Fixed issue with `tsh kube credentials` failing on remote clusters. [#23354](https://github.com/gravitational/teleport/pull/23354)
  * Fixed issue with `tsh kube credentials` loading incorrect profile. [#23716](https://github.com/gravitational/teleport/pull/23716)
* Machine ID
  * Added ability to specify memory backend using CLI parameters. [#23495](https://github.com/gravitational/teleport/pull/23495)
  * Added support for Azure delegated joining. [#23391](https://github.com/gravitational/teleport/pull/23391)
  * Added support for Gitlab delegated joining. [#23191](https://github.com/gravitational/teleport/pull/23191)
  * Added support for trusted clusters. [#23390](https://github.com/gravitational/teleport/pull/23390)
  * Added FIPS support. [#23850](https://github.com/gravitational/teleport/pull/23850)
* Proxy Peering
  * Fixed proxy peering issues when running behind a load balancer. [#23506](https://github.com/gravitational/teleport/pull/23506)
* Reverse Tunnels
  * Fixed issue when joining leaf cluster over tunnel port with enabled proxy protocol. [#23487](https://github.com/gravitational/teleport/pull/23487)
  * Fixed issue with joining agents over reverse tunnel port. [#23332](https://github.com/gravitational/teleport/pull/23332)
* Performance & scalability
  * Improved `tsh ls -R` performance in large clusters. [#23596](https://github.com/gravitational/teleport/pull/23596)
  * Improved performance when setting session environment variables. [#23834](https://github.com/gravitational/teleport/pull/23834)
* Server Access
  * Fixed issue with successful SFTP transfers returning non-zero code. [#23729](https://github.com/gravitational/teleport/pull/23729)
* SSO
  * Fixed issue with Github Enterprise SSO not working with custom URLs. [#23568](https://github.com/gravitational/teleport/pull/23568)
* Teleport Connect
  * Added support for config customization. [#23197](https://github.com/gravitational/teleport/pull/23197)
  * Fixed unresponsive terminal on Windows Server 2019. [#22996](https://github.com/gravitational/teleport/pull/22996)
* Tooling
  * Updated Electron to `22.3.2`. [#23048](https://github.com/gravitational/teleport/pull/23048)
  * Updated Go to `1.20.2`. [#22997](https://github.com/gravitational/teleport/pull/22997)
  * Updated Rust to `1.68.0`. [#23101](https://github.com/gravitational/teleport/pull/23101)
* Web UI
  * Added MFA support when copying files. [#23195](https://github.com/gravitational/teleport/pull/23195)
  * Fixed "ambiguous node" error when downloading files. [#23152](https://github.com/gravitational/teleport/pull/23152)
  * Fixed intermittent "client connection is closing" errors in web UI after logging in. [#23733](https://github.com/gravitational/teleport/pull/23733)

## 12.1.1 

This release of Teleport contains multiple improvements and bug fixes.

* Fixed issue with Access Management's connection tester not working with per-session MFA. [#22918](https://github.com/gravitational/teleport/pull/22918), [#22943](https://github.com/gravitational/teleport/pull/22943)
* Fixed Kubernetes access panic when using moderated sessions. [#22930](https://github.com/gravitational/teleport/pull/22930)
* Fixed `tsh db config` reporting incorrect port in TLS routing mode. [#22889](https://github.com/gravitational/teleport/pull/22889)
* Fixed issue with Teleport always performing OS group check even without auto user provisioning enabled. [#22805](https://github.com/gravitational/teleport/pull/22805)
* Fixed issue with desktop access crashing on systems that consume many file descriptors. [#22798](https://github.com/gravitational/teleport/pull/22798)
* Fixed issue with `teleport start --bootstrap` command failing on unexpected resource. [#22721](https://github.com/gravitational/teleport/pull/22721)
* Fixed issue with install script not refreshing repository metadata before installing new version. [#22585](https://github.com/gravitational/teleport/pull/22585)
* Added ability to export database CA in DER format via `tctl auth export`. [#22896](https://github.com/gravitational/teleport/pull/22896)
* Reduced log spam from proxy multiplexer. [#22802](https://github.com/gravitational/teleport/pull/22802)
* Updated EC2 auto-discovery install script to use enterprise binaries for enterprise clusters. [#22769](https://github.com/gravitational/teleport/pull/22769)
* Upgraded Go to `v1.19.7`. [#22725](https://github.com/gravitational/teleport/pull/22725)
* Improved idle connections handling. [#22908](https://github.com/gravitational/teleport/pull/22908), [#22893](https://github.com/gravitational/teleport/pull/22893)
* Improved Kubernetes service labels validation upon startup. [#22777](https://github.com/gravitational/teleport/pull/22777)
* Improved `tsh login` error reporting when proxy is not available. [#22763](https://github.com/gravitational/teleport/pull/22763)

## 12.1.0 

This release of Teleport contains multiple improvements and bug fixes.

* Added ability for Teleport to function as SAML IdP (Enterprise edition only).
* Downgraded Go to `v1.19.6` to resolve memory leak issues. [#22691](https://github.com/gravitational/teleport/pull/22691)
* Fixed issue with `tsh scp` overriding copied file permissions without `-p` flag. [#22609](https://github.com/gravitational/teleport/pull/22609)
* Improved performance of fetching remote clusters. [#22575](https://github.com/gravitational/teleport/pull/22575)

## 12.0.5 

This release of Teleport contains multiple improvements and bug fixes.

* Fixed issue with `tsh` not respecting HTTPS_PROXY in some cases. [#22492](https://github.com/gravitational/teleport/pull/22492)
* Fixed issue with config validation in Helm charts scratch mode. [#22423](https://github.com/gravitational/teleport/pull/22423)
* Added IAM joining support for Azure VMs. [#22204](https://github.com/gravitational/teleport/pull/22204)
* Added auto-discovery support for Azure VMs. [#22521](https://github.com/gravitational/teleport/pull/22521)
* Added support for `ap-southeast-4` AWS region for IAM joining. [#22486](https://github.com/gravitational/teleport/pull/22486)
* Added ability to specify web terminal scrollback length in proxy config. [#22422](https://github.com/gravitational/teleport/pull/22422)
* Added support for PuTTY's `winadj` channel requests. [#22420](https://github.com/gravitational/teleport/pull/22420)
* Added `--trace-profile` flag to `tsh` that allows generating runtime trace profiles. [#22406](https://github.com/gravitational/teleport/pull/22406)
* Added enhanced session recording support for arm64 architectures. [#22550](https://github.com/gravitational/teleport/pull/22550)
* Updated `tctl alert ack` to allow acknowledging alerts of any severity. [#22582](https://github.com/gravitational/teleport/pull/22582)
* Updated Windows desktop access to display only applicable logins. [#22333](https://github.com/gravitational/teleport/pull/22333)
* Improved Kubernetes access performance when using `kubectl`. [#22508](https://github.com/gravitational/teleport/pull/22508)
* Improved Teleport Connect performance when connecting to large clusters. [#22316](https://github.com/gravitational/teleport/pull/22316)
* Improved performance and scalability in large clusters. [#21495](https://github.com/gravitational/teleport/pull/21495)

## 12.0.4 

This release of Teleport contains multiple security fixes, improvements and bug fixes.

### Security fixes

* Fixed issue with malicious SQL Server packet being able to cause proxy crash. [#21638](https://github.com/gravitational/teleport/pull/21638)
* Fixed issue with session terminated after a short delay instead of being immediately paused when moderator leaves. [#21974](https://github.com/gravitational/teleport/pull/21974)

### Other improvements and bug fixes

* Fixed issue with orphaned child processes after session ends. [#22222](https://github.com/gravitational/teleport/pull/22222)
* Fixed issue with not being able to see any pods with an active Access Request. [#22196](https://github.com/gravitational/teleport/pull/22196)
* Fixed issue with remote cluster state not always being correctly updated. [#22088](https://github.com/gravitational/teleport/pull/22088)
* Fixed heartbeat errors from the Database Service. [#22087](https://github.com/gravitational/teleport/pull/22087)
* Fixed issue with applications temporarily disappearing during Application Service restart. [#21807](https://github.com/gravitational/teleport/pull/21807)
* Fixed issue with some Helm values being accidentally shared between Auth Service and Proxy Service configs. [#21768](https://github.com/gravitational/teleport/pull/21768)
* Fixed issues with desktop access flow in Access Management interface. [#21756](https://github.com/gravitational/teleport/pull/21756)
* Fixed "access denied" errors in Teleport Connect on Windows. [#21720](https://github.com/gravitational/teleport/pull/21720)
* Fixed issue with database GUI client connections requiring random taps when per-session MFA is enabled. [#21661](https://github.com/gravitational/teleport/pull/21661)
* Fixed issue with moderated sessions not working on leaf clusters. [#21612](https://github.com/gravitational/teleport/pull/21612)
* Fixed issue with missing `--request-id` flag in UI for Kubernetes login instructions. [#21445](https://github.com/gravitational/teleport/pull/21445)
* Fixed issue connecting to AWS resources when using full IAM role ARNs. [#21251](https://github.com/gravitational/teleport/pull/21251)
* Fixed issue with `local_auth: false` setting being ignored without explicitly setting `authentication_type`. [#22215](https://github.com/gravitational/teleport/pull/22215)
* Added `tctl` resource commands for Device Trust. [#22157](https://github.com/gravitational/teleport/pull/22157)
* Added support for assuming roles in `tsh proxy aws`. [#21990](https://github.com/gravitational/teleport/pull/21990)
* Added early feedback for successful security key taps in `tsh`. [#21780](https://github.com/gravitational/teleport/pull/21780)
* Added device lock support. [#21751](https://github.com/gravitational/teleport/pull/21751)
* Added support for security contexts in `teleport-kube-agent` Helm chart. [#21535](https://github.com/gravitational/teleport/pull/21535)
* Updated `tsh version` command to display client version only via `--client` flag. [#22167](https://github.com/gravitational/teleport/pull/22167)
* Updated install script to use enterprise packages for enterprise clusters. [#22109](https://github.com/gravitational/teleport/pull/22109)
* Updated install script to use deb/rpm repositories. [#22108](https://github.com/gravitational/teleport/pull/22108)
* Updated proxy init container in Helm charts to use security context. [#22064](https://github.com/gravitational/teleport/pull/22064)
* Updated `tsh` to include timestamps with debug logs. [#21996](https://github.com/gravitational/teleport/pull/21996)
* Updated AWS access to fetch credentials with TTL matching user's certificate TTL. [#21994](https://github.com/gravitational/teleport/pull/21994)
* Updated Go toolchain to `1.20.1`. [#21931](https://github.com/gravitational/teleport/pull/21931)
* Updated `tsh kube login --all` to not require cluster name. [#21765](https://github.com/gravitational/teleport/pull/21765)
* Updated `teleport db configure create` command to support more use-cases. [#21690](https://github.com/gravitational/teleport/pull/21690)
* Improved performance in large clusters with etcd backend. [#21905](https://github.com/gravitational/teleport/pull/21905), [#21496](https://github.com/gravitational/teleport/pull/21496)

## 12.0.2 

This release of Teleport contains a security fix as well as multiple improvements and bug fixes.

### OpenSSL update

* Updated OpenSSL to `1.1.1t`. [#21425](https://github.com/gravitational/teleport/pull/21425)

### Other fixes and improvements

* Fixed issue with Access Manager interface not accepting valid port numbers. [#21651](https://github.com/gravitational/teleport/pull/21651)
* Fixed issue with some requests  to Teleport-protected applications failing after proxy restart. [#21615](https://github.com/gravitational/teleport/pull/21615)
* Fixed issue with invalid role template namespaces leading to cluster lockouts. [#21573](https://github.com/gravitational/teleport/pull/21573)
* Fixed issue with Teleport Connect failing to recognize logged in user sometimes. [#21467](https://github.com/gravitational/teleport/pull/21467)
* Fixed issue with the back button not working in Web UI navigation. [#21236](https://github.com/gravitational/teleport/pull/21236)
* Fixed issue with Web UI SSH player having scroll bars. [#20868](https://github.com/gravitational/teleport/pull/20868)
* Added support for `tsh request search --kind=pod` command. [#21456](https://github.com/gravitational/teleport/pull/21456)
* Updated `tsh db configure create` to require flag for dynamic resources matching. [#21395](https://github.com/gravitational/teleport/pull/21395)
* Improved reconnect stability after Database Service restart. [#21635](https://github.com/gravitational/teleport/pull/21635)
* Improved reconnect stability after Kubernetes service restart.[#21617](https://github.com/gravitational/teleport/pull/21617)
* Improved `tsh ls -R` performance. [#21577](https://github.com/gravitational/teleport/pull/21577)
* Improved `tsh scp` error message when no remote path is specified. [#21373](https://github.com/gravitational/teleport/pull/21373)
* Improved error message when trying to rename resource. [#21179](https://github.com/gravitational/teleport/pull/21179)
* Reduced CPU usage when using enhanced session recording. [#21437](https://github.com/gravitational/teleport/pull/21437)

## 12.0.1 

Teleport 12 brings the following marquee features and improvements:

- Device Trust (Preview, Enterprise only)
- Passwordless Windows access for local users (Preview, Enterprise only)
- Per-pod RBAC for Kubernetes access (Preview)
- Azure and GCP CLI support for application access (Preview)
- Support for more databases in database access:
  - Amazon DynamoDB
  - Amazon Redshift Serverless
  - AWS RDS Proxy for PostgreSQL/MySQL
  - Azure SQLServer Auto Discovery
  - Azure Flexible Servers
- Refactored Helm charts (Preview)
- Dropped support for SHA1 in server access
- Signed/notarized macOS binaries

### Device Trust (Preview, Enterprise only)

Teleport 12 includes a preview of our upcoming Device Trust feature, which
allows administrators to require that Teleport access is performed from an
authenticated and trusted device.

This preview release requires macOS and a native client like tsh or Teleport
Connect. These clients leverage the Secure Enclave on macOS to solve device
challenges issued by the Teleport CA, proving their identity as a trusted
device.

Teleport features requiring the web UI (desktop access, application access) are
not currently supported.

### Passwordless Windows Access for Local Users (Preview, Enterprise only)

Teleport 12 brings passwordless certificate-based authentication to Windows
desktops in environments where Active Directory is not available. This feature
requires the installation of a Teleport package on each Windows desktop.

### Per-pod RBAC for Kubernetes access (Preview)

Teleport 12 extends RBAC to support controlling access to individual pods in
Kubernetes clusters. Pod RBAC integrates with existing Teleport RBAC features
such as role templating and Access Requests.

### Azure and GCP CLI support for application access (Preview)

In Teleport 12 administrators can interact with Azure and GCP APIs through the
Application Service using `tsh az` and `tsh gcloud` CLI commands, or using
standard `az` and `gcloud` tools through the local application proxy.

### Support for more databases in database access

Database access in Teleport 12 brings a number of new integrations to AWS-hosted
databases such as DynamoDB (now with audit log support), Redshift Serverless and
RDS Proxy for PostgreSQL/MySQL.

On Azure, database access adds SQLServer auto-discovery and support for Azure
Flexible Server for PostgreSQL/MySQL.

### Refactored Helm charts (Preview)

The “teleport-cluster” Helm chart underwent significant refactoring in Teleport
12 to provide better scalability and UX. Proxy and Auth are now separate
deployments and the new “scratch” chart mode makes it easier to provide a custom
Teleport config.

### Dropped support for SHA1 in Teleport-protected servers

Newer OpenSSH clients connecting to Teleport 12 clusters no longer need the
“PubAcceptedKeyTypes” workaround to include the deprecated “sha” algorithm.

### Signed/notarized macOS binaries

Users who download Teleport 12 Darwin binaries would no longer get an untrusted
software warning from macOS.

### tctl edit

tctl now supports an edit subcommand, allowing you to edit resources directly in
your preferred text editor.

### Breaking Changes

Please familiarize yourself with the following potentially disruptive changes in
Teleport 12 before upgrading.

#### Helm charts

The teleport-cluster Helm chart underwent significant changes in Teleport 12.

Additionally, PSPs are removed from the chart when installing on Kubernetes 1.23
and higher to account for the deprecation/removal of PSPs by Kubernetes.

#### tctl auth export

The tctl auth export command only exports the private key when passing the
--keys flag. Previously it would output the certificate and private key
together.

#### Desktop access

Windows Desktop sessions disable the wallpaper by default, improving
performance. To restore the previous behavior, add `show_desktop_wallpaper: true`
to your windows_desktop_service config.

## 11.3.2 

This release of Teleport contains multiple improvements and bug fixes.

* Fixed regression issue with accessing SSO apps behind application access. [#21049](https://github.com/gravitational/teleport/pull/21049)
* Fixed regression performance issue with `tsh scp`. [#20953](https://github.com/gravitational/teleport/pull/20953)
* Fixed issue with `tsh proxy aws --endpoint-url` not working. [#20880](https://github.com/gravitational/teleport/pull/20880)
* Fixed issue with MongoDB queries failing on large datasets. [#21113](https://github.com/gravitational/teleport/pull/21113)
* Fixed issue with direct node dial from web UI. [#20928](https://github.com/gravitational/teleport/pull/20928)
* Updated install scripts to download binaries from new CDN location. [#21057](https://github.com/gravitational/teleport/pull/21057)
* Updated `tsh` to detect unplugged devices when using hardware-backed keys. [#20949](https://github.com/gravitational/teleport/pull/20949)
* Updated Elasticsearch access to explicitly require `--db-user`. (#20695) [#20919](https://github.com/gravitational/teleport/pull/20919)
* Updated Rust to 1.67.0. [#20883](https://github.com/gravitational/teleport/pull/20883)

## 11.3.1 

This release of Teleport contains a security fix, as well as multiple improvements and bug fixes.

### Moderated Sessions

* Fixed issue with moderated sessions not being disconnected on Ctrl+C. [#20588](https://github.com/gravitational/teleport/pull/20588)

### Other fixes and improvements

* Fixed issue with node install script downloading OSS binaries in Enterprise edition. [#20816](https://github.com/gravitational/teleport/pull/20816)
* Fixed a regression when renewing Kubernetes dynamic credentials that prevented multiple renewals. [#20788](https://github.com/gravitational/teleport/pull/20788)
* Fixed issue with `tctl auth sign` not respecting Ctrl-C. [#20773](https://github.com/gravitational/teleport/pull/20773)
* Fixed occasional key attestation error in `tsh login`. [#20712](https://github.com/gravitational/teleport/pull/20712)
* Fixed issue with being able to create Access Requests with invalid cluster name. [#20674](https://github.com/gravitational/teleport/pull/20674)
* Fixed issue with EC2 auto-discovery install script for RHEL instances. [#20604](https://github.com/gravitational/teleport/pull/20604)
* Fixed issue connecting with Oracle MySQL client on Windows. [#20599](https://github.com/gravitational/teleport/pull/20599)
* Fixed issue with using `tctl auth sign --format kubernetes` against remote Auth Service instances. [#20571](https://github.com/gravitational/teleport/pull/20571)
* Fixed panic in Azure SQL Server access. [#20483](https://github.com/gravitational/teleport/pull/20483)
* Added support for Moderated Sessions in the Web UI. [#20796](https://github.com/gravitational/teleport/pull/20796)
* Added support for Login Rules for SSO users. [#20743](https://github.com/gravitational/teleport/pull/20743), [#20738](https://github.com/gravitational/teleport/pull/20738), [#20737](https://github.com/gravitational/teleport/pull/20737), [#20629](https://github.com/gravitational/teleport/pull/20629)
* Added ability to acknowledge alerts. [#20692](https://github.com/gravitational/teleport/pull/20692)
* Added `client_idle_timeout_message` support to Windows access. [#20617](https://github.com/gravitational/teleport/pull/20617)
* Added PodMonitor support in `teleport-cluster` Helm chart. [#20564](https://github.com/gravitational/teleport/pull/20564)
* Added support for passing raw config in `teleport-kube-agent` Helm chart. [#20449](https://github.com/gravitational/teleport/pull/20449)
* Added nodeSelector field to `teleport-cluster` Helm chart. [#20441](https://github.com/gravitational/teleport/pull/20441)
* Improved Kubernetes access stability for slow clients. [#20517](https://github.com/gravitational/teleport/pull/20517)
* Updated `teleport-cluster` Helm chart to reload proxy certificate daily. [#20503](https://github.com/gravitational/teleport/pull/20503)

## 11.2.3 

This release of Teleport contains multiple improvements and bug fixes.

* Fixed issue with `tsh login` defaulting to passwordless and ignoring the `--auth` and `--mfa-mode` flags. [#20474](https://github.com/gravitational/teleport/pull/20474)
* Fixed regression issue with AWS console access via `tsh aws`. [#20437](https://github.com/gravitational/teleport/pull/20437)
* Fixed issue connecting to MariaDB in non-TLS Routing mode. [#20409](https://github.com/gravitational/teleport/pull/20409)
* Fixed the `*:*` selector in EC2 auto-discovery. [#20390](https://github.com/gravitational/teleport/pull/20390)
* Improved handling of unknown events in the events search API. [#20329](https://github.com/gravitational/teleport/pull/20329)
* Added support for multiple transformations in role templates. [#20296](https://github.com/gravitational/teleport/pull/20296)
* Added the ability to update a Trusted Cluster's role mappings without recreating the cluster. [#20286](https://github.com/gravitational/teleport/pull/20286)
* Added `dnsConfig` support to the `teleport-kube-agent` Helm chart. [#20107](https://github.com/gravitational/teleport/pull/20107)

## 11.2.2 

This release of Teleport contains multiple improvements and bug fixes.

* Fixed issue connecting to leaf cluster nodes via web UI with per-session MFA. [#20238](https://github.com/gravitational/teleport/pull/20238)
* Fixed issue with `max_kubernetes_connections` leading to access denied errors. [#20174](https://github.com/gravitational/teleport/pull/20174)
* Fixed issue with `kube-agent` Helm chart leaving state behind after `helm uninstall`. [#20169](https://github.com/gravitational/teleport/pull/20169)
* Fixed X.509 issue after updating RDS database resource. [#20099](https://github.com/gravitational/teleport/pull/20099)
* Fixed issue with some `tsh` HTTP requests missing extra headers. [#20071](https://github.com/gravitational/teleport/pull/20071)
* Improved auto-discovery config validation. [#20288](https://github.com/gravitational/teleport/pull/20288)
* Improved graceful shutdown stability. [#20225](https://github.com/gravitational/teleport/pull/20225)
* Improved application access authentication flow. [#20165](https://github.com/gravitational/teleport/pull/20165)
* Reduced auth load by ensure proxy uses cache for periodic operations. [#20153](https://github.com/gravitational/teleport/pull/20153)
* Updated Rust to `1.66.1`. [#20201](https://github.com/gravitational/teleport/pull/20201)
* Updated macOS binaries to be signed and notarized. [#20305](https://github.com/gravitational/teleport/pull/20305)

## 11.2.1 

This release of Teleport contains multiple improvements and bug fixes.

* Added support for periodically reloading the proxy's TLS certificates [#20040](https://github.com/gravitational/teleport/pull/20040)
* Improved desktop certificate generation by using the proper field for querying a user's SID [#20022](https://github.com/gravitational/teleport/pull/20022)
* Updated the web UI to hide the trusted clusters screen for users who lack the appropriate role [#1494](https://github.com/gravitational/webapps/pull/1494/)
* Fixed an issue resulting in an "invalid bearer token" message [#20102](https://github.com/gravitational/teleport/pull/#20102)
* Fixed an issue preventing bots from using IAM joining [#20011](https://github.com/gravitational/teleport/pull/20011)
* Fixed an issue where Machine ID Certificates did not respect the provided TTL when using IAM joining [#20001](https://github.com/gravitational/teleport/pull/20001)
* Updated to Go 1.19.5 [#20084](https://github.com/gravitational/teleport/pull/20084)

## 11.2.0 

This release of Teleport contains multiple improvements and bug fixes.

### Machine ID GitHub Actions

In addition, we're happy to announce a set of GitHub Actions that you can use in
your workflows to assist with accessing Teleport Resources in your CI/CD pipelines.

Visit the individual repositories to find out more and see usage examples:

- https://github.com/teleport-actions/setup
- https://github.com/teleport-actions/auth
- https://github.com/teleport-actions/auth-k8s

For a more in-depth guide, see our
[documentation](./docs/pages/enroll-resources/machine-id/deployment/github-actions.mdx) for using
Teleport with GitHub Actions.

### Secure certificate mapping for desktop access

Later this year, Windows will begin requiring a stronger mapping from a certificate
to an Active Directory user. In anticipation of this change, Teleport 11.2.0 is compliant
with the new requirements.

*Warning:* This feature requires that Teleport's own service account also uses a strong
mapping. In order to support this requirement, you must now set a new Security Identifier
(`sid`) field in the LDAP configuration for your Windows Desktop Services. You can find
the SID for your service account by running the following PowerShell snippet
(replace `svc-teleport` with the name of the service account you are using):

```
Get-AdUser -Identity svc-teleport | Select SID
```

### Other improvements and bugfixes

* Added an improved database joining flow in the web UI [#1487](https://github.com/gravitational/webapps/pull/1487)
* Added support for secure certificate mapping for Windows desktop certificates [#19737](https://github.com/gravitational/teleport/pull/19737)
* Fixed an issue with desktop directory sharing where large files could be corrupted [#1472](https://github.com/gravitational/webapps/pull/1472)
* Fixed an issue where desktop access users may see a an error after ending a session [#1470](https://github.com/gravitational/webapps/pull/1470)
* Fixed an issue preventing database agents from joining due to improperly formatted YAML [#19958](https://github.com/gravitational/teleport/pull/19958)
* Updated the web UI to use session storage instead of local storage for Teleport's bearer token [#1470](https://github.com/gravitational/webapps/pull/1470)
* Added rate limiting to SAML/OIDC routes [#19950](https://github.com/gravitational/teleport/pull/19950)
* Fixed an issue connecting to leaf cluster desktops via reverse tunnel [#19945](https://github.com/gravitational/teleport/pull/19945)
* Fixed a backwards compatibility issue with database access in 11.1.4 [#19940](https://github.com/gravitational/teleport/pull/19940)
* Fixed an issue where Access Requests for Kubernetes clusters used improperly cached credentials [#19912](https://github.com/gravitational/teleport/pull/19912)
* Added support for CentOS 7 in ARM64 builds [#19895](https://github.com/gravitational/teleport/pull/19895)
* Added rate limiting to unauthenticated routes [#19869](https://github.com/gravitational/teleport/pull/19869)
* Add suggested reviewers and requestable roles to Teleport Connect Access Requests [#19846](https://github.com/gravitational/teleport/pull/19846)
* Fixed an issue listing all nodes with `tsh` [#19821](https://github.com/gravitational/teleport/pull/19821)
* Made `gcp.credentialSecretName` optional in the Teleport Cluster Helm chart [#19803](https://github.com/gravitational/teleport/pull/19803)
* Fixed an issue preventing audit events that exceed the maximum size limit from being logged [#19736](https://github.com/gravitational/teleport/pull/19736)
* Fixed an issue preventing some users from being able to play desktop recordings [#19709](https://github.com/gravitational/teleport/pull/19709)
* Added validation of AWS Account IDs when adding databases (#19638) [#19702](https://github.com/gravitational/teleport/pull/19702)
* Added a new audit event for DynamoDB requests via application access [#19667](https://github.com/gravitational/teleport/pull/19667)
* Added the ability to export `tsh` traces even when the Auth Service is not configured for tracing [#19583](https://github.com/gravitational/teleport/pull/19583)
* Added support for linking Teleport Connect's embedded `tsh` binary for use outside of Teleport Connect [#1488](https://github.com/gravitational/webapps/pull/1488)

## 11.1.4 

This release of Teleport contains multiple security fixes, improvements and bug fixes.

*Note:* This release of Teleport contains an issue that affects backwards compatibility
with database access agents. If you are a database access user we recommend skipping
straight to version 11.2.0.

### [Critical] RBAC bypass in SSH TCP tunneling

When establishing a direct-tcpip channel, Teleport did not sufficiently validate
RBAC.

This could allow an attacker in possession of valid cluster credentials to
establish a TCP tunnel to a node they didn’t have access to.

The connection attempt would show up in the audit log as a “port” audit event
(code T3003I) and include Teleport username in the “user” field.

### [High] Application access session hijack

When accepting application Access Requests, Teleport did not sufficiently
validate client credentials.

This could allow an attacker in possession of a valid active application session
ID to issue requests to this application impersonating the session owner for a
limited time window.

Presence of multiple “cert.create” audit events (code TC000I) with the same app
session ID in the “route_to_app.session_id” field may indicate the attempt to
impersonate an existing user’s application session.

### [Medium] SSH IP pinning bypass

When issuing a user certificate, Teleport did not check for the presence of IP
restrictions in the client’s credentials.

This could allow an attacker in possession of valid client credentials with IP
restrictions to reissue credentials without IP restrictions.

Presence of a “cert.create” audit event (code TC000I) without corresponding
“user.login” audit event (codes T1000I or T1101I) for users with IP restricted
roles may indicate an issuance of a certificate without IP restrictions.

### [Low] Web API session caching

After logging out via the web UI, a user’s session could remain cached in
Teleport’s proxy, allowing continued access to resources for a limited time
window.

### Other improvements and bugfixes

* Fixed issue with noisy-square distortions in desktop access. [#19545](https://github.com/gravitational/teleport/pull/19545)
* Fixed issue with LDAP search pagination in desktop access. [#19533](https://github.com/gravitational/teleport/pull/19533)
* Fixed issue with SSH sessions inheriting OOM score of the parent process. [#19521](https://github.com/gravitational/teleport/pull/19521)
* Fixed issue with ambiguous host resolution in web UI. [#19513](https://github.com/gravitational/teleport/pull/19513)
* Fixed issue with using desktop access with Windows 10. [#19504](https://github.com/gravitational/teleport/pull/19504)
* Fixed issue with `session.start` events being overwritten by `session.exec` events. [#19497](https://github.com/gravitational/teleport/pull/19497)
* Fixed issue with `tsh login --format kubernetes` not setting SNI info. [#19433](https://github.com/gravitational/teleport/pull/19433)
* Fixed issue with WebSockets not working via app access if the upstream web server is using HTTP/2. [#19423](https://github.com/gravitational/teleport/pull/19423)
* Fixed TLS routing in insecure mode. [#19410](https://github.com/gravitational/teleport/pull/19410)
* Fixed issue with connecting to ElastiCache 7.0.4 in database access. [#19400](https://github.com/gravitational/teleport/pull/19400)
* Fixed issue with SAML connector validation calling descriptor URL prior to authz checks. [#19317](https://github.com/gravitational/teleport/pull/19317)
* Fixed issue with database access complaining about "redis" engine not being registered. [#19251](https://github.com/gravitational/teleport/pull/19251)
* Fixed issue with `disconnect_expired_cert` and `require_session_mfa` settings conflicting with each other. [#19178](https://github.com/gravitational/teleport/pull/19178)
* Fixed startup failure when MongoDB URI is not resolvable. [#18984](https://github.com/gravitational/teleport/pull/18984)
* Added resource names for Access Requests in Teleport Connect. [#19549](https://github.com/gravitational/teleport/pull/19549)
* Added support for Github Enterprise join method. [#19518](https://github.com/gravitational/teleport/pull/19518)
* Added the ability to supply Access Request TTLs. [#19385](https://github.com/gravitational/teleport/pull/19385)
* Added new `instance.join` and `bot.join` audit events. [#19343](https://github.com/gravitational/teleport/pull/19343)
* Added support for port-forward over websocket protocol in Kubernetes access. [#19181](https://github.com/gravitational/teleport/pull/19181)
* Reduced latency of `tsh ls -R`. [#19482](https://github.com/gravitational/teleport/pull/19482)
* Updated desktop access config script to disable password prompt. [#19427](https://github.com/gravitational/teleport/pull/19427)
* Updated Go to 1.19.4. [#19127](https://github.com/gravitational/teleport/pull/19127)
* Improved performance when converting traits to roles. [#19170](https://github.com/gravitational/teleport/pull/19170)
* Improved handling of expired database certificates in Teleport Connect. [#19096](https://github.com/gravitational/teleport/pull/19096)

## 11.1.2 

This release of Teleport contains multiple improvements and bug fixes.

* Fixed issue with EC2 discovery failing to install Teleport on older Ubuntu instances. [#18965](https://github.com/gravitational/teleport/pull/18965)
* Fixed issue with log spam when cleaning up groups for automatically created Linux users. [#18990](https://github.com/gravitational/teleport/pull/18990)
* Fixed issue with `tctl windows_desktops ls` not producing results in JSON and YAML formats. [#19016](https://github.com/gravitational/teleport/pull/19016)
* Fixed issue with web SSH sessions in proxy recording mode. [#19021](https://github.com/gravitational/teleport/pull/19021)
* Improved handling of corrupted session recordings. [#19040](https://github.com/gravitational/teleport/pull/19040)

## 11.1.1 

This release of Teleport contains a security fix as well as multiple improvements and bug fixes.

### Insecure TOTP MFA seed removal

Fixed issue where an attacker with physical access to user's computer and raw
access to the filesystem could potentially recover the seed QR code.

[#18917](https://github.com/gravitational/teleport/pull/18917)

### Other improvements and fixes

* Fixed issue with Teleport Connect not working on macOS. [#18921](https://github.com/gravitational/teleport/pull/18921)
* Added support for Cloud HSM on Google Cloud. [#18835](https://github.com/gravitational/teleport/pull/18835)
* Added `server_hostname` to `session.*` audit events. [#18832](https://github.com/gravitational/teleport/pull/18832)
* Added ability to specify roles when making Access Requests in web UI. [#18868](https://github.com/gravitational/teleport/pull/18868)
* Improved error reporting from etcd backend. [#18822](https://github.com/gravitational/teleport/pull/18822)
* Improved failed session recording upload logs to include upload and session IDs. [#18872](https://github.com/gravitational/teleport/pull/18872)

## 11.1.0 

This release of Teleport contains multiple improvements and bug fixes.

* Added support for self-hosted Github Enterprise SSO connectors in Teleport Enterprise edition. [#18521](https://github.com/gravitational/teleport/pull/18521), [#18687](https://github.com/gravitational/teleport/pull/18687)
* Added audit events for DynamoDB via AWS CLI access. [#18035](https://github.com/gravitational/teleport/pull/18035)
* Added auth connectors support in Kubernetes Operator. [#18350](https://github.com/gravitational/teleport/pull/18350)
* Added audit events for desktop access directory sharing. [#18398](https://github.com/gravitational/teleport/pull/18398)
* Added trusted clusters support for desktop access. [#18666](https://github.com/gravitational/teleport/pull/18666)
* Added support for `user.spec` syntax in moderated session filters. [#18455](https://github.com/gravitational/teleport/pull/18455)
* Added support for GKE auto-discovery to Kubernetes access. [#18396](https://github.com/gravitational/teleport/pull/18396)
* Added FIPS support to desktop access. [#18743](https://github.com/gravitational/teleport/pull/18743)
* Added `teleport discovery bootstrap` command. [#18641](https://github.com/gravitational/teleport/pull/18641)
* Added `windows_desktops` as the correct resource for `tctl` commands. [#18816](https://github.com/gravitational/teleport/pull/18816)
* Updated `tsh db ls` JSON and YAML output to include allowed users. [#18543](https://github.com/gravitational/teleport/pull/18543)
* Updated `tctl auth sign --format kubernetes` to allow merging multiple clusters in the same kubeconfig. [#18525](https://github.com/gravitational/teleport/pull/18525)
* Improved web UI SSH performance. [#18797](https://github.com/gravitational/teleport/pull/18797), [#18839](https://github.com/gravitational/teleport/pull/18839)
* Improved `tsh play` output in JSON and YAML formats. [#18825](https://github.com/gravitational/teleport/pull/18825)
* Fixed issue with RDS auto-discovery failing to start in some cases. [#18590](https://github.com/gravitational/teleport/pull/18590)
* Fixed "cannot read properties of null" error when trying to add a new server using web UI. [webapps#1356](https://github.com/gravitational/webapps/pull/1356)
* Fixed issue with applications list pagination in web UI. [#18601](https://github.com/gravitational/teleport/pull/18601)
* Fixed issue with MongoDB commands sometimes failing through database access. [#18738](https://github.com/gravitational/teleport/pull/18738)
* Fixed issue with automatically imported cloud labels not being used in RBAC in App Access. [#18642](https://github.com/gravitational/teleport/pull/18642)
* Fixed issue with Kubernetes sessions lingering after all participants have disconnected. [#18684](https://github.com/gravitational/teleport/pull/18684)
* Fixed issue with Auth Service being down affecting ability to establish new non-moderated SSH sessions. [#18441](https://github.com/gravitational/teleport/pull/18441)
* Fixed issue with launching SSH sessions when SELinux is enabled. [#18810](https://github.com/gravitational/teleport/pull/18810)
* Fixed issue with not being able to create SAML connectors with templated role names. [#18766](https://github.com/gravitational/teleport/pull/18766)

## 11.0.3 

This release of Teleport contains multiple improvements and bug fixes.

* Fixed issue with validation of U2F devices. [#17876](https://github.com/gravitational/teleport/pull/17876)
* Fixed `tsh ssh -J` not being able to connect to leaf cluster nodes. [#18268](https://github.com/gravitational/teleport/pull/18268)
* Fixed issue with failed database connection when client requests GSS encryption. [#17811](https://github.com/gravitational/teleport/pull/17811)
* Fixed issue with setting Teleport version to v10 in Helm charts resulting in invalid config. [#18008](https://github.com/gravitational/teleport/pull/18008)
* Fixed issue with Teleport Kubernetes resource name conflicting with builtin resources. [#17717](https://github.com/gravitational/teleport/pull/17717)
* Fixed issue with invalid MS Teams plugin systemd service file. [#18028](https://github.com/gravitational/teleport/pull/18028)
* Fixed issue with failing to connect to OpenSSH 7.x servers. [#18248](https://github.com/gravitational/teleport/pull/18248)
* Fixed issue with extra trailing question mark in application Access Requests. [#17955](https://github.com/gravitational/teleport/pull/17955)
* Fixed issue with application access websocket requests sometimes failing in Chrome. [#18002](https://github.com/gravitational/teleport/pull/18002)
* Fixed issue with multiple `tbot`'s concurrently using the same output directory. [#17999](https://github.com/gravitational/teleport/pull/17999)
* Fixed issue with `tbot` failing to parse version on some kernels. [#18298](https://github.com/gravitational/teleport/pull/18298)
* Fixed panic when v9 node runs against v11 Auth Service. [#18383](https://github.com/gravitational/teleport/pull/18383)
* Fixed issue with Kubernetes proxy caching client credentials between sessions. [#18109](https://github.com/gravitational/teleport/pull/18109)
* Fixed issue with agents not being able to reconnect to proxies in some cases. [#18149](https://github.com/gravitational/teleport/pull/18149)
* Fixed issue with remote tunnel connections not being closed properly. [#18224](https://github.com/gravitational/teleport/pull/18224)
* Added CircleCI support to Machine ID. [#17996](https://github.com/gravitational/teleport/pull/17996)
* Added support for `arm` and `arm64` Docker images for Teleport and Operator. [#18222](https://github.com/gravitational/teleport/pull/18222)
* Added PostgreSQL and MySQL RDS Proxy support to database access. [#18045](https://github.com/gravitational/teleport/pull/18045)
* Improved database access denied error messages. [#17856](https://github.com/gravitational/teleport/pull/17856)
* Improved desktop access errors in case of locked sessions. [#17549](https://github.com/gravitational/teleport/pull/17549)
* Improved web UI handling of private key policy errors. [#17991](https://github.com/gravitational/teleport/pull/17991)
* Improved memory usage in clusters with large numbers of active sessions. [#18051](https://github.com/gravitational/teleport/pull/18051)
* Updated `tsh proxy ssh` to support `HTTPS_PROXY`. [#18295](https://github.com/gravitational/teleport/pull/18295)
* Updated Azure hosted databases to fetch the new CA. [#18172](https://github.com/gravitational/teleport/pull/18172)
* Updated `tsh kube login` to support providing default user, group and namespace. [#18185](https://github.com/gravitational/teleport/pull/18185)
* Updated web UI session listing to include active sessions of all types. [#18229](https://github.com/gravitational/teleport/pull/18229)
* Updated user locking to terminate in progress TCP application access connections. [#18187](https://github.com/gravitational/teleport/pull/18187)
* Updated `teleport configure` command to produce v2 config when Auth Service is provided. [#17914](https://github.com/gravitational/teleport/pull/17914)
* Updated all systemd service files to set max open files limit. [#17961](https://github.com/gravitational/teleport/pull/17961)

## 11.0.1 

This release of Teleport contains a security fix and multiple bug fixes.

### Block SFTP in Moderated Sessions

Teleport did not block SFTP protocol in Moderated Sessions.

[#17727](https://github.com/gravitational/teleport/pull/17727)

### Other fixes

* Fixed issue with agent forwarding not working for auto-created users. [#17586](https://github.com/gravitational/teleport/pull/17586)
* Fixed "traits missing" error in application access. [#17737](https://github.com/gravitational/teleport/pull/17737)
* Fixed connection leak issue in IAM joining. [#17737](https://github.com/gravitational/teleport/pull/17737)
* Fixed panic in "tsh db ls". [#17780](https://github.com/gravitational/teleport/pull/17780)
* Fixed issue with "tsh mfa add" not displaying OTP QR code image on Windows. [#17703](https://github.com/gravitational/teleport/pull/17703)
* Fixed issue with `tctl rm windows_desktop/<name>` removing all desktops. [#17732](https://github.com/gravitational/teleport/pull/17732)
* Fixed issue connecting to Redis 7.0 in cluster mode. [#17849](https://github.com/gravitational/teleport/pull/17849)
* Fixed "failed to open user account database" error after exiting SSH session. [#17825](https://github.com/gravitational/teleport/pull/17825)
* Improved `tctl` UX when using hardware-backed private keys. [#17681](https://github.com/gravitational/teleport/pull/17681)
* Improved `tsh mfa add` error reporting. [#17580](https://github.com/gravitational/teleport/pull/17580)

## 11.0.0 

Teleport 11 brings the following new major features and improvements:

- Hardware-backed private keys support for server access (Enterprise only).
- Replacement of obsolete SCP protocol with SFTP for server access.
- Removal of persistent storage requirement for Helm charts.
- Automatic discovery and enrollment of EKS/AKS clusters for Kubernetes access.
- Richer Azure integrations for server and database access.
- Cassandra and Scylla support for database access, including Amazon Keyspaces.
- GitHub Actions and Terraform support for Machine ID.
- Access Requests and file upload/download support for Teleport Connect.

### Hardware-backed private keys (Enterprise Only)

Teleport 11 clients (such as tsh or Connect) support storing their private key
material on Yubikey devices instead of filesystem which helps prevent
credentials exfiltration attacks.

See how to enable it in the
[documentation](docs/pages/admin-guides/access-controls/guides/hardware-key-support.mdx):

Hardware-backed private keys is an enterprise only feature, and is currently
supported for server access only.

### SFTP protocol

Teleport 11 adds server-side support for SFTP protocol which many IDEs such as
VSCode or JetBrains PyCharm, GoLand and others use for browsing, copying, and
editing files on remote systems.

The following guides explain how to use IDEs to connect to a remote machine via
Teleport:

- [VS Code](./docs/pages/enroll-resources/server-access/guides/vscode.mdx)
- [JetBrains](./docs/pages/enroll-resources/server-access/guides/jetbrains-sftp.mdx)

In addition, Teleport 11 clients will use SFTP protocol for file transfer under
the hood instead of the obsolete scp protocol. Server-side scp is still
supported so existing clients aren’t affected.

### Helm charts persistent storage

In Teleport 11 users no longer need to use persistent storage when deploying
Helm charts. When running on Kubernetes, Teleport services will now store their
identities in Kubernetes Secrets which removes the need for using persistent
storage or static join tokens.

For existing deployments, this change involves migration from Deployment to
StatefulSet which is performed automatically during Helm upgrade to Teleport 11.

### EKS/AKS discovery

Teleport 11 adds support for automatic discovery and enrollment of AWS Elastic
Kubernetes Service (EKS) and Azure Kubernetes Service (AKS) clusters.

### Azure integrations

Teleport 11 improves Azure support in multiple areas.

Teleport agents running on Azure VMs will now automatically import Azure tags to
label resources.

Teleport database access now supports auto-discovery for Azure-hosted PostgreSQL
and MySQL databases. See the [Azure
guide](docs/pages/enroll-resources/database-access/enroll-azure-databases/azure-postgres-mysql.mdx) for more
details.

In addition, Teleport database access will now use Azure AD managed identity
authentication for Azure-hosted SQL Server databases.

### Cassandra/ScyllaDB

Teleport 11 adds support for Cassandra and ScyllaDB databases in Database
Access. This includes support for Amazon Keyspaces.

### Machine ID

Teleport 11 adds support for secret-less joining of Machine ID agents in GitHub
Actions workflows. See the guide for more details: TODO

We have also released a GitHub Action for setting up the Teleport binaries
within a GitHub workflow environment. More details regarding this can be found
at the Teleport GitHub Actions repository:

https://github.com/gravitational/teleport-actions

In addition, the Teleport Terraform plugin now supports the creation of Machine
ID Bots and Bot Tokens.

### tsh MFA on Windows

tsh 11 adds support for MFA and passwordless logins via Windows Hello and
FIDO2 devices.

### Teleport Connect

Teleport Connect has added support for Access Requests and file upload/download.

### Breaking Changes

Please familiarize yourself with the following potentially disruptive changes in
Teleport 11 before upgrading.

#### Removed Github external SSO

Beginning in Teleport 11, GitHub SAML SSO will only be available in our
Enterprise Edition. GitHub SSO without SAML will continue to work with OSS
Teleport.

To keep using GitHub SSO with Teleport Community Edition, SAML SSO needs to be disabled
for your GitHub organization. Teleport Community Edition users can continue to use GitHub SSO
if using a Github Free or Team GitHub Plan.

#### Changed Terraform OIDC connector redirect_url type to array

In Teleport Plugins 11, `redirect_url` property in OIDC connectors created via
a Terraform module expects an array:

```
redirect_url = [ "http://example.com" ]
```

#### Deprecated Quay.io registry

Starting with Teleport 11, Quay.io as a container registry has been deprecated.
Customers should use the new AWS ECR registry to pull [Teleport Docker
images](./docs/pages/installation.mdx#docker).

Quay.io registry support will be removed in a future release.

#### Deprecated old deb/rpm repositories

In Teleport 11, old deb/rpm repositories (deb.releases.teleport.dev and
rpm.releases.teleport.dev) have been deprecated. Customers should use the new
repositories (apt.releases.teleport.dev and yum.releases.teleport.dev) to
[install Teleport](docs/pages/installation.mdx#linux).

Support for our old deb/rpm repositories will be removed in a future release.

#### Changed teleport-kube-agent Helm chart to StatefulSet

Teleport 11 agents will now store their identities in Kubernetes Secrets when
deployed via a Helm chart which eliminates the need for using persistent storage
or static join tokens. Due to this change, Teleport agents are now always
deployed as part of StatefulSet regardless of whether persistent storage is
enabled or not.

Existing agents that were deployed as Kubernetes Deployments (i.e. without
persistent storage) will be automatically converted to StatefulSets during
Teleport 11 Helm upgrade.

#### Removed PostgreSQL backend

The preview PostgreSQL backend was deleted due to performance and scalability
concerns.

#### Removed desktop access support for 32-bit ARM and 386 architectures

32-bit support for desktop access on ARM and 386 architectures has been removed
due to performance issues on these devices.

This also reduces the binary size for these builds, making them slightly more
convenient for smaller resource-constrained devices.

## 10.0.0 

Teleport 10 is a major release that brings the following new features.

Platform:

* Passwordless (Preview)
* Resource Access Requests (Preview)
* Proxy Peering (Preview)

Server access:

* IP-Based Restrictions (Preview)
* Automatic User Provisioning (Preview)

Database access:

* Audit Logging for Microsoft SQL Server database access
* Snowflake database access (Preview)
* ElastiCache/MemoryDB database access (Preview)

Teleport Connect:

* Teleport Connect for server and database access (Preview)

Machine ID:

* Machine ID database access support (Preview)

### Passwordless (Preview)

Teleport 10 introduces passwordless support to your clusters. To use passwordless
users may register a security key with resident credentials or use a built-in
authenticator, like Touch ID.

See the [documentation](docs/pages/admin-guides/access-controls/guides/passwordless.mdx).

### Resource Access Requests (Preview)

Teleport 10 expands just-in-time Access Requests to allow for requesting access
to specific resources. This lets you grant users the least privileged access
needed for their workflows.

Just-in-time Access Requests are only available in Teleport Enterprise Edition.

### Proxy Peering (Preview)

Proxy peering enables Teleport deployments to scale without an increase in load
from the number of agent connections. This is accomplished by allowing Proxy
Services to tunnel client connections to the desired agent through a neighboring
proxy and decoupling the number of agent connections from the number of Proxies.

Proxy peering can be enabled with the following configurations:

```yaml
auth_service:
  tunnel_strategy:
    type: proxy_peering
    agent_connection_count: 1
```

```yaml
proxy_service:
  peer_listen_addr: 0.0.0.0:3021
```

Network connectivity between proxy servers to the `peer_listen_addr` is required
for this feature to work.

Proxy peering is only available in Teleport Enterprise Edition.

### IP-Based Restrictions (Preview)

Teleport 10 introduces a new role option to pin the source IP in SSH
certificates. When enabled, the source IP that was used to request certificates
is embedded in the certificate, and SSH servers will reject connection attempts
from other IPs. This protects against attacks where valid credentials are
exfiltrated from disk and copied out into other environments.

IP-based restrictions are only available in Teleport Enterprise Edition.

### Automatic User Provisioning (Preview)

Teleport 10 can be configured to automatically create Linux host users upon
login without having to use Teleport's PAM integration. Users can be added to specific
Linux groups and assigned appropriate “sudoer” privileges.

To learn more about configuring automatic user provisioning read the
[documentation](docs/pages/enroll-resources/server-access/guides/host-user-creation.mdx).

### Audit Logging for Microsoft SQL Server database access

Teleport 9 introduced a preview of database access support for Microsoft SQL
Server which didn’t include audit logging of user queries. Teleport 10 captures
users' queries and prepared statements and sends them to the audit log, similarly
to other supported database protocols.

Teleport database access for SQL Server remains in Preview mode with more UX
improvements coming in future releases.

Refer to [the guide](docs/pages/enroll-resources/database-access/enroll-aws-databases/sql-server-ad.mdx) to set
up access to a SQL Server with Active Directory authentication.

### Snowflake database access (Preview)

Teleport 10 brings support for Snowflake to database access. Administrators can
set up access to Snowflake databases through Teleport for their users with
standard database access features like role-based access control and audit
logging, including query activity.

Connect your Snowflake database to Teleport following the
[documentation](docs/pages/enroll-resources/database-access/enroll-managed-databases/snowflake.mdx).

### Elasticache/MemoryDB database access (Preview)

Teleport 9 added Redis protocol support to database access. Teleport 10 improves
this integration by adding native support for AWS-hosted Elasticache and
MemoryDB, including auto-discovery and automatic credential management in some
deployment configurations.

Learn more about it in the [documentation](
docs/pages/enroll-resources/database-access/enroll-aws-databases/redis-aws.mdx).

### Teleport Connect for server and database access (Preview)

Teleport Connect is a graphical macOS application that simplifies access to your
Teleport resources. Teleport Connect 10 supports server access and database access.
Other protocols and Windows support are coming in a future release.

Get Teleport Connect installer from the macOS tab on the downloads page:
https://goteleport.com/download/.

### Machine ID database access support (Preview)

In Teleport 10 we’ve added database access support to Machine ID. Applications
can use Machine ID to access databases protected by Teleport.

You can find Machine ID guide for database access in the
[documentation](docs/pages/enroll-resources/machine-id/access-guides/databases.mdx).

### Breaking changes

Please familiarize yourself with the following potentially disruptive changes in
Teleport 10 before upgrading.

#### Auth Service version check

Teleport 10 agents will now refuse to start if they detect that the Auth Service
is more than one major version behind them. You can use the `--skip-version-check` flag to
bypass the version check.

Take a look at component compatibility guarantees in the
[documentation](docs/pages/upgrading/upgrading.mdx).

#### HTTP_PROXY for reverse tunnels

Reverse tunnel connections will now respect `HTTP_PROXY` environment variables.
This may result in reverse tunnel agents not being able to re-establish
connections if the HTTP proxy is set in their environment and does not allow
connections to the Teleport Proxy Service.

Refer to the
[documentation](docs/pages/reference/networking.mdx#http-connect-proxies)
for more details.

#### New APT repos

With Teleport 10 we’ve migrated to new APT repositories that now support
multiple release channels, Teleport versions and OS distributions. The new
repositories have been backfilled with Teleport versions starting from 6.2.31
and we recommend upgrading to them. The old repositories will be maintained for
the foreseeable future.

See the [installation
instructions](docs/pages/enroll-resources/server-access/getting-started.mdx#step-14-install-teleport-on-your-linux-host).

#### Removed “tctl access ls”

The `tctl access ls` command that returned information about user server access
within the cluster was removed. Please use a previous `tctl` version if you’d like
to keep using it.

#### Relaxed session join permissions

In previous versions of Teleport users needed full access to a Node/Kubernetes
pod in order to join a session. Teleport 10 relaxes this requirement. Joining
sessions remains deny-by-default but now only `join_sessions` statements are
checked for session join RBAC.

See the [Moderated Sessions
guide](docs/pages/admin-guides/access-controls/guides/joining-sessions.mdx) for more
details.

#### GitHub connectors

The GitHub authentication connector’s `teams_to_logins` field is deprecated in favor of the new
`teams_to_roles` field. The old field will be removed in a future release.

#### Teleport FIPS AWS endpoints

Teleport 10 will now automatically use FIPS endpoints for AWS S3 and DynamoDB
when started with the `--fips` flag. You can use the `use_fips_endpoint=false`
connection endpoint option to use regular endpoints for Teleport in FIPS mode,
for example:

```
s3://bucket/path?region=us-east-1&use_fips_endpoint=false
```

See the [S3/DynamoDB backend
documentation](docs/pages/reference/backends.mdx) for more information.

## 9.3.9 

This release of Teleport contains a security fix, as well as multiple improvements and bug fixes.

### Auth bypass in Moderated Sessions

When checking a user’s roles prior to starting a session, Teleport may have
incorrectly allowed a session to proceed without moderation depending on the
order roles are received from the backend.

### Other improvements and fixes

* Fixed issue with per-session MFA swallowing key presses. [#13822](https://github.com/gravitational/teleport/pull/13822)
* Fixed issue with `tsh db ls -R` now showing allowed users. [#13626](https://github.com/gravitational/teleport/pull/13626)
* Fixed vertical and horizontal scroll in desktop access. [#13905](https://github.com/gravitational/teleport/pull/13905)
* Fixed issue with invalid query filters forcing `tsh` relogin. [#13747](https://github.com/gravitational/teleport/pull/13747)
* Fixed issue with TLS routing and proxy jump. [#13928](https://github.com/gravitational/teleport/pull/13928)
* Fixed issue with MongoDB connections timing out in certain scenarios. [#13859](https://github.com/gravitational/teleport/pull/13859)
* Fixed issue with Machine ID certificate renewal with empty requested roles. [#13893](https://github.com/gravitational/teleport/pull/13893)
* Fixed issue with Windows desktops not being labeled with LDAP attribute labels. [#13681](https://github.com/gravitational/teleport/pull/13681)
* Fixed issue with desktop access streaming not being terminated properly. [#14024](https://github.com/gravitational/teleport/pull/14024)
* Added ability to use FIPS endpoints for S3 and DynamoDB using `use_fips_endpoint` connection option. [#13703](https://github.com/gravitational/teleport/pull/13703)
* Added ability to specify CA pin as a file path in the config. [#13089](https://github.com/gravitational/teleport/pull/13089)
* Improved reconnect reliability after root proxy restart. [#13967](https://github.com/gravitational/teleport/pull/13967)
* Improved error messages for failed auth client connections. [#13835](https://github.com/gravitational/teleport/pull/13835)

## 9.3.7 

This release of Teleport contains multiple improvements and bug fixes.

* Fixed issue with startup delay caused by AWS EC2 check. [#13167](https://github.com/gravitational/teleport/pull/13167)
* Added `tsh ls -R` that displays resources across all clusters and profiles. [#13313](https://github.com/gravitational/teleport/pull/13313)
* Fixed issue with `tsh` not correctly reporting "address in use" error during port forwarding. [#13679](https://github.com/gravitational/teleport/pull/13679)
* Fixed two potential panics. [#13590](https://github.com/gravitational/teleport/pull/13590), [#13655](https://github.com/gravitational/teleport/pull/13655)
* Fixed issue with enhanced session recording not working on recent Ubuntu versions. [#13650](https://github.com/gravitational/teleport/pull/13650)
* Fixed issue with CA rotation when Database Service does not contain any databases. [#13517](https://github.com/gravitational/teleport/pull/13517)
* Fixed issue with desktop access connection failing with "invalid channel name rdpsnd" error. [#13450](https://github.com/gravitational/teleport/issues/13450)
* Fixed issue with invalid Teleport config when enabling IMDSv2 in Terraform config. [#13537](https://github.com/gravitational/teleport/pull/13537)

## 9.3.6 

This release of Teleport contains multiple improvements and bug fixes.

* Added Unicode clipboard support to desktop access. [#13391](https://github.com/gravitational/teleport/pull/13391)
* Fixed backwards compatibility issue with fetch Access Requests from older servers. [#13490](https://github.com/gravitational/teleport/pull/13490)
* Fixed issue with requests to Teleport-protected apps periodically failing with 500 errors. [#13469](https://github.com/gravitational/teleport/pull/13469)
* Fixed issues with pagination when displaying applications. [#13451](https://github.com/gravitational/teleport/pull/13451)
* Fixed file descriptor leak in Machine ID. [#13386](https://github.com/gravitational/teleport/pull/13386)

## 9.3.5 

This release of Teleport contains multiple improvements and bug fixes.

* Fixed backwards compatibility issue with fetching Access Requests from older servers. [#13428](https://github.com/gravitational/teleport/pull/13428)
* Fixed issue with using Microsoft SQL Server Management Studio with database access. [#13337](https://github.com/gravitational/teleport/pull/13337)
* Added support for `tsh proxy ssh -J` to improve interoperability with OpenSSH clients. [#13311](https://github.com/gravitational/teleport/pull/13311)
* Added ability to provide security context in Helm charts. [#13286](https://github.com/gravitational/teleport/pull/13286)
* Added Application and database access support to reference AWS Terraform deployment. [#13383](https://github.com/gravitational/teleport/pull/13383)
* Improved reliability of dialing the Auth Service through the Proxy Service. [#13399](https://github.com/gravitational/teleport/pull/13399)
* Improved `kubectl exec` auditing by logging access denied attempts. [#12831](https://github.com/gravitational/teleport/pull/12831), [#13400](https://github.com/gravitational/teleport/pull/13400)

## 9.3.4 

This release of Teleport contains multiple security, bug fixes and improvements.

### Escalation attack in agent forwarding

When setting up agent forwarding on the node, Teleport did not handle unix socket creation in a secure manner.

This could have given a potential attacker an opportunity to get Teleport to change arbitrary file permissions to the attacker’s user.

### WebSockets CSRF

When handling websocket requests, Teleport did not verify that the provided Bearer token was generated for the correct user.

This could have allowed a malicious low privileged Teleport user to use a social engineering attack to gain higher privileged access on the same Teleport cluster.

### Denial of service in Access Requests

When accepting an Access Request, Teleport did not enforce the maximum request reason size.

This could allow a malicious actor to mount a DoS attack by creating an Access Request with a very large request reason.

### Auth bypass in moderated sessions

When initializing a moderated session, Teleport did not discard participant’s input prior to the moderator joining.

This could prevent a moderator from being able to interrupt a malicious command executed by a participant.

### Other fixes

* Fixed issue with stdin hijacking when per-session MFA is enabled. [#13212](https://github.com/gravitational/teleport/pull/13212)
* Added support for automatic tags import when running on AWS EC2. [#12593](https://github.com/gravitational/teleport/pull/12593)
* Added ability to use multiple redirect URLs in OIDC connectors. [#13046](https://github.com/gravitational/teleport/pull/13046)
* Fixed issue with ANSI escape sequences being broken when using `tsh` on Windows. [#13221](https://github.com/gravitational/teleport/pull/13221)
* Fixed issue with `tsh ssh` printing extra error upon exit if last command was unsuccessful. [#12903](https://github.com/gravitational/teleport/pull/12903)
* Added support for Proxy Protocol v2 in MySQL proxy. [#12993](https://github.com/gravitational/teleport/pull/12993)
* Upgraded to Go `v1.17.11`. [#13104](https://github.com/gravitational/teleport/pull/13104)
* Added Windows desktops labeling based on their LDAP attributes. [#13238](https://github.com/gravitational/teleport/pull/13238)
* Improved performance when listing resources for users with many roles. [#13263](https://github.com/gravitational/teleport/pull/13263)

## 9.3.2 

This release of Teleport contains two bug fixes.

* Fixed issue with Machine ID's `tsh` version check. [#13037](https://github.com/gravitational/teleport/pull/13037)
* Fixed AWS related log spam in database agent when not running on AWS. [#12984](https://github.com/gravitational/teleport/pull/12984)

## 9.3.0 

This release of Teleport contains multiple improvements and bug fixes.

* Fixed issue with `tctl` not taking `TELEPORT_HOME` environment variable into account. [#12738](https://github.com/gravitational/teleport/pull/12738)
* Fixed issue with Redis `AUTH` command not always authenticating the user in database access. [#12754](https://github.com/gravitational/teleport/pull/12754)
* Fixed issue with Teleport not starting with deprecated U2F configuration. [#12826](https://github.com/gravitational/teleport/pull/12826)
* Fixed issue with `tsh db ls` not showing allowed users for leaf clusters. [#12853](https://github.com/gravitational/teleport/pull/12853)
* Fixed issue with `teleport configure` failing when given non-existent data directory. [#12806](https://github.com/gravitational/teleport/pull/12806)
* Fixed issue with `tctl` not outputting debug logs. [#12920](https://github.com/gravitational/teleport/pull/12920)
* Fixed issue with Kubernetes access not working when using default CA pool. [#12874](https://github.com/gravitational/teleport/pull/12874)
* Fixed issue with Machine ID not working in TLS routing mode. [#12990](https://github.com/gravitational/teleport/pull/12990)
* Improved connection performance in large clusters. [#12832](https://github.com/gravitational/teleport/pull/12832)
* Improved memory usage in large clusters. [#12724](https://github.com/gravitational/teleport/pull/12724)

### Breaking Changes

Teleport 9.3.0 reduces the minimum GLIBC requirement to 2.18 and enforces more
secure cipher suites for desktop access.

As a result of these changes, desktop access users with desktops running Windows
Server 2012R2 will need to perform [additional
configuration](docs/pages/enroll-resources/desktop-access/getting-started.mdx) to force Windows
to use compatible cipher suites.

Windows desktops running Windows Server 2016 and newer will continue to operate
normally - no additional configuration is required.

## 9.2.4 

This release of Teleport contains multiple improvements and bug fixes.

* Fixed compatibility issue with agents connected to older Auth Service instances. [#12728](https://github.com/gravitational/teleport/pull/12728)
* Fixed issue with TLS routing endpoint advertising preference for `http/1.1` over `h2`. [#12749](https://github.com/gravitational/teleport/pull/12749)
* Implemented multiple proxy restart stability improvements. [#12632](https://github.com/gravitational/teleport/pull/12632), [#12488](https://github.com/gravitational/teleport/pull/12488), [#12689](https://github.com/gravitational/teleport/pull/12689)
* Improved compatibility with PuTTY. [#12662](https://github.com/gravitational/teleport/pull/12662)
* Added support for global tsh config file `/etc/tsh.yaml`. [#12626](https://github.com/gravitational/teleport/pull/12626)
* Added `tbot configure` command. [#12576](https://github.com/gravitational/teleport/pull/12576)
* Fixed issue with desktop access not working in Teleport Enterprise (Cloud). [#12781](https://github.com/gravitational/teleport/pull/12781)
* Improved Web UI performance in large clusters. [#12637](https://github.com/gravitational/teleport/pull/12637)
* Fixed issue with running MySQL stored procedures via database access. [#12734](https://github.com/gravitational/teleport/pull/12734)

## 9.2.3 

This release of Teleport contains multiple improvements and bug fixes.

* Fixed issue with `HTTP_PROXY` being inadvertently respected in reverse tunnel connections. [#12335](https://github.com/gravitational/teleport/pull/12335)
* Added `--format` flag to `tctl token add` command. [#12588](https://github.com/gravitational/teleport/pull/12588)
* Fixed backwards compatibility issues with session upload. [#12535](https://github.com/gravitational/teleport/pull/12535)
* Added support for persistency in custom mode in Helm charts. [#12218](https://github.com/gravitational/teleport/pull/12218)
* Fixed issue with PostgreSQL backend not respecting username from certificate. [#12553](https://github.com/gravitational/teleport/pull/12553)
* Fixed issues with `kubectl cp` and `kubectl exec` not working through Kubernetes access. [#12541](https://github.com/gravitational/teleport/pull/12541)
* Fixed issues with dynamic registration logic for cloud databases. [#12451](https://github.com/gravitational/teleport/pull/12451)
* Fixed issue with automatic Add Application script failing to join the cluster. [#12539](https://github.com/gravitational/teleport/pull/12539)
* Fixed issue with `tctl` crashing when PAM is enabled. [#12572](https://github.com/gravitational/teleport/pull/12572)
* Added support for setting priority class and extra labels in Helm charts. [#12568](https://github.com/gravitational/teleport/pull/12568)
* Fixed issue with App Access JWT tokens not including `iat` claim. [#12589](https://github.com/gravitational/teleport/pull/12589)
* Added ability to inject App Access JWT tokens in rewritten headers. [#12589](https://github.com/gravitational/teleport/pull/12589)
* Desktop access automatically adds a `teleport.dev/ou` label for desktops discovered via LDAP. [#12502](https://github.com/gravitational/teleport/pull/12502)
* Updated Machine ID to generates identity files compatible with `tctl` and `tsh`. [#12500](https://github.com/gravitational/teleport/pull/12500)
* Updated internal build infrastructure to Go 1.17.10. [#12607](https://github.com/gravitational/teleport/pull/12607)
* Improved proxy memory usage in clusters with large number of nodes. [#12573](https://github.com/gravitational/teleport/pull/12573)

## 9.2.1 

This release of Teleport contains an improvement and several bug fixes.

* Updated `tctl rm` command to support removing tokens. [#12439](https://github.com/gravitational/teleport/pull/12439)
* Fixed issue with Teleport failing to start when using DynamoDB backend in pay-per-request mode. [#12461](https://github.com/gravitational/teleport/pull/12461)
* Fixed issue with Kubernetes port forwarding not working. [#12468](https://github.com/gravitational/teleport/pull/12468)
* Fixed issue with IAM policy limit when using database auto-discovery on Kubernetes. [#12457](https://github.com/gravitational/teleport/pull/12457)

## 9.2.0 

This release of Teleport contains multiple improvements, security and bug fixes.

* Fixed issue with U2F facets not being properly validated. [#12208](https://github.com/gravitational/teleport/pull/12208)
* Hardened SQLite permissions. [#12360](https://github.com/gravitational/teleport/pull/12360)
* Fixed issue with OIDC callback not checking `email_verified` claim. [#12360](https://github.com/gravitational/teleport/pull/12360)
* Added `max_kubernetes_connections` role option for limiting simultaneous Kubernetes connections. [#12360](https://github.com/gravitational/teleport/pull/12360)
* Fixed issue with Teleport failing to start with pay-per-request DynamoDB mode. [#12360](https://github.com/gravitational/teleport/pull/12360)
* Reduced Machine ID verbosity in case of missing secure symlink kernel support. [#12423](https://github.com/gravitational/teleport/pull/12423)
* Fixed `tsh proxy db` tunnel mode not working for CockroachDB connections. [#12400](https://github.com/gravitational/teleport/pull/12400)
* Added support for database access certificates in Machine ID. [#12195](https://github.com/gravitational/teleport/pull/12195)
* Improved shutdown/restart stability in certain scenarios. [#12393](https://github.com/gravitational/teleport/pull/12393)
* Added support for clickable labels in web UI. [#12422](https://github.com/gravitational/teleport/pull/12422)

## 9.1.3 

This release of Teleport contains multiple improvements and bug fixes.

* Fixed issue with some MySQL clients not being able to connect to MySQL 8.0 servers. [#12340](https://github.com/gravitational/teleport/pull/12340)
* Fixed multiple conditions that could lead to SSH sessions freezing. [#12286](https://github.com/gravitational/teleport/pull/12286)
* Fixed issue with `tsh db ls` failing for leaf clusters. [#12320](https://github.com/gravitational/teleport/pull/12320)
* Fixed a scenario in which Teleport's internal cache could potentially become unhealthy. [#12251](https://github.com/gravitational/teleport/pull/12251), [#12002](https://github.com/gravitational/teleport/pull/12002)
* Improved performance when opening new application access sessions. [#12300](https://github.com/gravitational/teleport/pull/12300)
* Added flags to the `teleport configure` command. [#12267](https://github.com/gravitational/teleport/pull/12267)
* Improved CA rotation stability. [#12333](https://github.com/gravitational/teleport/pull/12333)
* Fixed issue with `mongosh` certificate verification when using TLS routing. [#12363](https://github.com/gravitational/teleport/pull/12363)

## 9.1.2 

This release of Teleport contains two bug fixes.

* Fixed issue with Teleport pods not becoming ready on Kubernetes. [#12243](https://github.com/gravitational/teleport/pull/12243)
* Fixed issue with Teleport processes crashing upon restart after failed host UUID generation. [#12222](https://github.com/gravitational/teleport/pull/12222)

## 9.1.1 

This release of Teleport contains multiple bug fixes and improvements.

* Fixed regression issue where reverse tunnel connections inadvertently started respecting `HTTP_PROXY`. [#12035](https://github.com/gravitational/teleport/pull/12035)
* Fixed potential deadlock in SSH server. [#12122](https://github.com/gravitational/teleport/pull/12122)
* Fixed issue with Kubernetes service not reporting its readiness. [#12152](https://github.com/gravitational/teleport/pull/12152)
* Fixed issue with JumpCloud identity provider. [#11936](https://github.com/gravitational/teleport/pull/11936)
* Fixed issue with deleting many records from Firestore backend. [#12177](https://github.com/gravitational/teleport/pull/12177)

## 9.1.0 

Teleport 9.1 is a minor release that brings several new features, security and bug fixes.

### Security

Teleport build infrastructure was updated to use Go v1.17.9 to fix CVE-2022-24675, CVE-2022-28327 and CVE-2022-27536.

### SQL backend (preview)

Teleport users can now use PostgreSQL or CockroachDB for storing Auth Service data.

See the [documentation](docs/pages/reference/backends.mdx) for more information.

### Server-side filtering and pagination

Searching and filtering resources is now handled on the server, improving the
efficiency of queries with `tsh`, `tctl`, or the web UI.

The web UI loads resources faster by leveraging server-side pagination.
Additionally, the web UI supports bookmarking searches by including the query in
the URL.

### Other improvements and fixes

* Fixed issue with stdin being ignored after refreshing expired credentials. [#11847](https://github.com/gravitational/teleport/pull/11847)
* Fixed issue with `tsh` requiring host login when using identity files for some commands. [#11793](https://github.com/gravitational/teleport/pull/11793)
* Added support for calling proxy over plain HTTP in insecure mode. [#11403](https://github.com/gravitational/teleport/pull/11403)
* Fixed multiple issues that could lead to sessions output freezing. [#11853](https://github.com/gravitational/teleport/pull/11853)
* Added optional gRPC client/server latency metrics. [#11773](https://github.com/gravitational/teleport/pull/11773)
* Fixed issue with connecting to self-hosted databases in TLS insecure mode. [#11758](https://github.com/gravitational/teleport/pull/11758)
* Improved error message when incorrect auth connector name is used. [#11884](https://github.com/gravitational/teleport/pull/11884)
* Implemented multiple moderated session stability improvements. [#11803](https://github.com/gravitational/teleport/pull/11803), [#11890](https://github.com/gravitational/teleport/pull/11890)
* Added authenticated tunnel mode to `tsh proxy db` command. [#11808](https://github.com/gravitational/teleport/pull/11808)
* Fixed issue with application sessions not being deleted upon web logout. [#11956](https://github.com/gravitational/teleport/pull/11956)
* Improved MySQL audit logging to include support for additional commands. [#11949](https://github.com/gravitational/teleport/pull/11949)
* Improved reliability of Teleport services restart. [#11795](https://github.com/gravitational/teleport/pull/11795)
* Fixed issue with Okta OIDC auth connector not working. [#11718](https://github.com/gravitational/teleport/pull/11718)
* Added support for `json` and `yaml` formatting to all `tsh` commands. [#12050](https://github.com/gravitational/teleport/pull/12050)
* Added support for setting `kubernetes_users`, `kubernetes_groups`, `db_names`, `db_users` and `aws_role_arns` traits when creating users. [#12133](https://github.com/gravitational/teleport/pull/12133)
* Fixed potential CA rotation panic. [#12004](https://github.com/gravitational/teleport/pull/12004)
* Updated `tsh db ls` to display allowed database usernames. [#11942](https://github.com/gravitational/teleport/pull/11942)
* Fixed goroutine leak in OIDC client. [#12078](https://github.com/gravitational/teleport/pull/12078)

## 9.0.4 

This release of Teleport contains multiple improvements and fixes.

* Fixed issue with `:` not being allowed in label keys. [#11563](https://github.com/gravitational/teleport/pull/11563)
* Fixed potential panic in Kubernetes access. [#11614](https://github.com/gravitational/teleport/pull/11614)
* Added `teleport_connect_to_node_attempts_total` Prometheus metric. [#11629](https://github.com/gravitational/teleport/pull/11629)
* Multiple CA rotation stability improvements. [#11658](https://github.com/gravitational/teleport/pull/11658)
* Fixed console player Ctrl-C and Ctrl-D functionality. [#11559](https://github.com/gravitational/teleport/pull/11559)
* Improved logging in case of node with existing state joining an new cluster. [#11751](https://github.com/gravitational/teleport/pull/11751)
* Added preview of PostgreSQL/CockroachDB backend. [#11667](https://github.com/gravitational/teleport/pull/11667)
* Fixed compatibility issues with CA loading between old and new tsh versions. [#11663](https://github.com/gravitational/teleport/pull/11663)
* Fixed loggers not respecting JSON configuration. [#11655](https://github.com/gravitational/teleport/pull/11655)
* Added support for Proxy Protocol v2. [#11722](https://github.com/gravitational/teleport/pull/11722)
* Fixed a number of tsh player stability issues. [#11491](https://github.com/gravitational/teleport/pull/11491)
* Improved network utilization caused by session uploader. [#11698](https://github.com/gravitational/teleport/pull/11698)
* Improved remote clusters inventory bookkeeping. [#11707](https://github.com/gravitational/teleport/pull/11707)

## 9.0.3 

This release of Teleport contains multiple fixes.

* Fixed issue with `tctl` ignoring `TELEPORT_HOME` environment variable. [#11561](https://github.com/gravitational/teleport/pull/11561)
* Fixed multiple moderated sessions stability issues. [#11494](https://github.com/gravitational/teleport/pull/11494)
* Fixed issue with `tsh version` exiting with error when tsh config file is not present. [#11571](https://github.com/gravitational/teleport/pull/11571)
* Fixed issue with `tsh` not respecting proxy hosts. [#11496](https://github.com/gravitational/teleport/pull/11496)
* Fixed issue with Kubernetes forwarder taking HTTP proxies into account. [#11462](https://github.com/gravitational/teleport/pull/11462)
* Fixed issue with stale DynamoDB Auth Service instances disrupting agent reconnect attempts. [#11598](https://github.com/gravitational/teleport/pull/11598)

## 9.0.2 

This release of Teleport contains multiple features, improvements and bug fixes.

* Added support for per-user `tsh` configuration preferences. [#10336](https://github.com/gravitational/teleport/pull/10336)
* Added support for role bootstrapping in OSS. [#11175](https://github.com/gravitational/teleport/pull/11175)
* Added `HTTP_PROXY` support to tsh. [#10209](https://github.com/gravitational/teleport/pull/10209)
* Improved error messages `tsh` and `tctl` show to include usage information on invalid command line invocation. [#11174](https://github.com/gravitational/teleport/pull/11174)
* Improved `tctl <resource> ls` output to make it consistent across all resources. [#9519](https://github.com/gravitational/teleport/pull/9519)
* Fixed multiple issues with CA rotation, graceful restart, and stability. [#10706](https://github.com/gravitational/teleport/pull/10706) [#11074](https://github.com/gravitational/teleport/pull/11074) [#11283](https://github.com/gravitational/teleport/pull/11283)
* Fixed issue where MOTD was not always shown. [#10735](https://github.com/gravitational/teleport/pull/10735)
* Fixed an issue where certificate extension not being included in `tctl auth sign`. [#10949](https://github.com/gravitational/teleport/pull/10949)
* Fixed a panic that could occur in the Web UI. [#11389](https://github.com/gravitational/teleport/pull/11389)

## 9.0.1 

This release of Teleport contains multiple improvements and bug fixes.

* Fixed issue with Ctrl-C freezing sessions. [#11188](https://github.com/gravitational/teleport/pull/11188)
* Improved handling of unknown audit events. [#11064](https://github.com/gravitational/teleport/pull/11064)
* Improved calculation of public addresses for dynamically registered apps. [#11139](https://github.com/gravitational/teleport/pull/11139)
* Fixed `tsh aws ecr` returning 500 errors. [#11108](https://github.com/gravitational/teleport/pull/11108)
* Fixed issue with deleting certain users. [#11131](https://github.com/gravitational/teleport/pull/11131)
* Fixed issue with Machine ID not detecting token in file config. [#11206](https://github.com/gravitational/teleport/pull/11206)

## 9.0.0 

Teleport 9.0 is a major release that brings:

- Teleport desktop access GA
- Teleport Machine ID Preview
- Various additions to Teleport database access
- Moderated Sessions for server and Kubernetes access

Desktop access adds support for clipboard sharing, session recording, and
per-session MFA.

Teleport Machine ID Preview extends identity-based access to machines. It's the
easiest way to issue, renew, and manage SSH and X.509 certificates for service
accounts, microservices, CI/CD automation and all other forms of
machine-to-machine access.

Database access brings self-hosted Redis support, RDS MariaDB (10.6 and higher)
support, auto-discovery for Redshift clusters, and auto-IAM configuration
improvements to GA. Additionally, this release also brings Microsoft SQL Server
with AD authentication to Preview.

Moderated Sessions enables the creation of sessions where a moderator has to
be present. This feature can be selectively enabled for specific sessions via
RBAC and can be used in conjunction with per-session MFA.

### Desktop access

#### Clipboard Support

Desktop access now supports copying and pasting text between your local
workstation and a remote Windows Desktop. This feature requires a Chromium-based
browser and can be disabled via RBAC.

#### Session Recording

Desktop sessions are now recorded and stored alongside SSH sessions, and can be
viewed in Teleport's web interface. Desktop session recordings are fully
compatible with the RBAC for sessions feature introduced in Teleport 8.1.

#### Per-session MFA

Per-session MFA settings now apply to desktop sessions. This allows cluster
administrators to require an additional MFA "tap" prior to opening a desktop
session. This feature requires a WebAuthn device.

### Machine ID (Preview)

Machine ID allows the creation of machine / bot / service account users who can
automatically issue, renew, and manage SSH and X.509 certificates to facilitate
machine-to-machine access.

Machine ID is a service that programmatically issues and renews short-lived
certificates to any service account (e.g., a CI/CD server) by retrieving
credentials from the Teleport Auth Service. This enables fine-grained role-based
access controls and audit.

Some of the things you can do with Machine ID:

- Machines can retrieve short-lived SSH certificates for CI/CD pipelines.
- Machines can retrieve short-lived X.509 certificates for use with databases or
  applications.
- Configure role-based access controls and locking for machines.
- Capture access events in the audit log.

[Machine ID getting started guide](docs/pages/enroll-resources/machine-id/getting-started.mdx)

### Database access

#### Redis

You can now use database access to connect to a self-hosted Redis instance or
Redis cluster and view Redis commands in the Teleport audit log. We will be
adding support for Amazon Elasticache in the coming weeks.

[Self-hosted Redis
guide](docs/pages/enroll-resources/database-access/enroll-self-hosted-databases/redis.mdx)

#### SQL Server (Preview)

Teleport 9 includes a preview release of Microsoft SQL Server with Active
Directory authentication support for database access. Audit logging of query
activity is not included in the preview release and will be implemented in a
later 9.x release.

[SQL Server guide](docs/pages/enroll-resources/database-access/enroll-aws-databases/sql-server-ad.mdx)

#### RDS MariaDB

Teleport 9 updates MariaDB support with auto-discovery and connection to AWS RDS
MariaDB databases using IAM authentication. The minimum MariaDB version that
supports IAM authentication is 10.6.

[Updated RDS guide](docs/pages/enroll-resources/database-access/enroll-aws-databases/rds.mdx)

#### Other Improvements

In addition, Teleport 9 expands auto-discovery to support Redshift databases and
2 new commands which simplify the database access getting started experience:
"teleport db configure create", which generates Database Service configuration,
and "teleport db configure bootstrap", which configures IAM permissions for the
Database Service when running on AWS.

CLI commands reference:
- [`teleport db configure
  create`](docs/pages/reference/agent-services/database-access-reference/cli.mdx)
- [`teleport db configure bootstrap`](docs/pages/reference/agent-services/database-access-reference/cli.mdx)

### Moderated Sessions

With Moderated Sessions, Teleport administrators can define policies that allow
users to invite other users to participate in SSH or Kubernetes sessions as
observers, moderators or peers.

[Moderated Sessions guide](docs/pages/admin-guides/access-controls/guides/joining-sessions.mdx)

### Breaking Changes

#### CentOS 6

CentOS 6 support was deprecated in Teleport 8 and has now been removed.

#### Desktop access

Desktop access now authenticates to LDAP using X.509 client certificates.
Support for the `password_file` configuration option has been removed.

## 8.0.0 

Teleport 8.0 is a major release of Teleport that contains new features, improvements, and bug fixes.

### New Features

#### Windows desktop access Preview

Teleport 8.0 includes a preview of the Windows desktop access feature, allowing
users passwordless login to Windows Desktops via any modern web browser.

Teleport users can connect to Active Directory enrolled Windows hosts running
Windows 10, Windows Server 2012 R2 and newer Windows versions.

To try this feature yourself, check out our
[Getting Started Guide](docs/pages/enroll-resources/desktop-access/getting-started.mdx).

Review the desktop access design in:

- [RFD #33](https://github.com/gravitational/teleport/blob/master/rfd/0033-desktop-access.md)
- [RFD #34](https://github.com/gravitational/teleport/blob/master/rfd/0034-desktop-access-windows.md)
- [RFD #35](https://github.com/gravitational/teleport/blob/master/rfd/0035-desktop-access-windows-authn.md)
- [RFD #37](https://github.com/gravitational/teleport/blob/master/rfd/0037-desktop-access-protocol.md)

#### TLS Routing

In TLS routing mode all client connections are wrapped in TLS and multiplexed on
a single Teleport proxy port.

TLS routing can be enabled by including the following Auth Service configuration:

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
to log into their AWS console using `tsh apps login` and use `tsh aws` commands
to interact with AWS APIs.

See more info in the
[documentation](docs/pages/enroll-resources/application-access/cloud-apis/aws-console.mdx).

#### Application and Database Dynamic Registration

With dynamic registration users are able to manage applications and databases
without needing to update static YAML configuration or restart application or
database agents.

See dynamic registration guides for
[apps](docs/pages/enroll-resources/application-access/guides/dynamic-registration.mdx)
and
[databases](docs/pages/enroll-resources/database-access/guides/dynamic-registration.mdx).

#### RDS Automatic Discovery

With RDS auto discovery Teleport database agents can automatically discover RDS
instances and Aurora clusters in an AWS account.

See updated
[RDS guide](docs/pages/enroll-resources/database-access/enroll-aws-databases/rds.mdx) for
more information.

#### WebAuthn

WebAuthn support enables Teleport users to use modern multi-factor options,
including Apple FaceID and TouchID.

In addition, the Teleport Web UI includes new multi-factor management tools,
enabling users to configure and update their multi-factor devices via their
web browser.

Lastly, our UI becomes more secure by requiring an additional multi-factor
confirmation for certain privileged actions (editing roles for multi-factor
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
* Added per-session MFA support to database access.
  [#8270](https://github.com/gravitational/teleport/pull/8270)
* Added support for profile specific `kubeconfig`.
  [#7840](https://github.com/gravitational/teleport/pull/7840)

### Fixes

* Fixed issues with web applications that utilized
  [EventSource](https://developer.mozilla.org/en-US/docs/Web/API/EventSource)
  with application access.
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

#### Database access Certificates

With the `GODEBUG=x509ignoreCN=0` flag removed in Go 1.17, database access users
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

## 7.0.0 

Teleport 7.0 is a major release of Teleport that contains new features, improvements, and bug fixes.

### New Features

#### MongoDB

Added support for [MongoDB](https://www.mongodb.com) to Teleport database access. [#6600](https://github.com/gravitational/teleport/issues/6600).

View the [database access with MongoDB](docs/pages/enroll-resources/database-access/enroll-self-hosted-databases/mongodb-self-hosted.mdx) for more details.

#### Cloud SQL MySQL

Added support for [GCP Cloud SQL MySQL](https://cloud.google.com/sql/docs/mysql) to Teleport database access. [#7302](https://github.com/gravitational/teleport/pull/7302)

View the Cloud SQL MySQL [guide](docs/pages/enroll-resources/database-access/enroll-google-cloud-databases/mysql-cloudsql.mdx) for more details.

#### AWS Console

Added support for [AWS Console](https://aws.amazon.com/console) to Teleport application access. [#7590](https://github.com/gravitational/teleport/pull/7590)

Teleport application access can now automatically sign users into the AWS Management Console using [Identity federation](https://aws.amazon.com/identity/federation). View AWS Management Console [guide](docs/pages/enroll-resources/application-access/cloud-apis/aws-console.mdx) for more details.

#### Restricted Sessions

Added the ability to block network traffic (IPv4 and IPv6) on a per-SSH session basis. Implemented using BPF tooling which required kernel 5.8 or above. [#7099](https://github.com/gravitational/teleport/pull/7099)

#### Enhanced Session Recording

Updated Enhanced Session Recording to no longer require the installation of external compilers like `bcc-tools`. Implemented using BPF tooling which required kernel 5.8 or above. [#6027](https://github.com/gravitational/teleport/pull/6027)

### Improvements

* Added the ability to terminate database access certificates when the certificate expires. [#5476](https://github.com/gravitational/teleport/issues/5476)
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

Enhanced Session Recording has been updated to use CO-RE BPF executables. This means that you no longer have to install `bcc-tools`, but comes with a higher minimum kernel version of 5.8 and above. [#6027](https://github.com/gravitational/teleport/pull/6027)

#### Kubernetes access

Kubernetes access will no longer automatically register a cluster named after the Teleport cluster if the proxy is running within a Kubernetes cluster. Users wishing to retain this functionality now have to explicitly set `kube_cluster_name`. [#6786](https://github.com/gravitational/teleport/pull/6786)

#### `tsh`

`tsh login` has been updated to no longer change the current Kubernetes context. While `tsh login` will write credentials to `kubeconfig` it will only update your context if `tsh login --kube-cluster` or `tsh kube login <kubeCluster>` is used. [#6045](https://github.com/gravitational/teleport/issues/6045)

## 6.2.0 

Teleport 6.2 contains new features, improvements, and bug fixes.

**Note:** the DynamoDB indexing change described below may cause rate-limiting
errors from AWS APIs and is slow on large deployments (1000+ existing audit
events). The next patch release, v6.2.1, will improve the migration performance.
If you run a large DynamoDB-based cluster, we advise you to wait for v6.2.1
before upgrading.

### New Features

#### Added Amazon Redshift Support

Added support for [Amazon Redshift](https://aws.amazon.com/redshift) to Teleport database access.[#6479](https://github.com/gravitational/teleport/pull/6479).

View the [database access with Redshift on AWS guide](docs/pages/enroll-resources/database-access/enroll-aws-databases/postgres-redshift.mdx) for more details.

### Improvements

* Added pass-through header support for Teleport application access. [#6601](https://github.com/gravitational/teleport/pull/6601)
* Added ability to propagate claim information from root to leaf clusters. [#6540](https://github.com/gravitational/teleport/pull/6540)
* Added Proxy Protocol for MySQL database access. [#6594](https://github.com/gravitational/teleport/pull/6594)
* Added prepared statement support for Postgres database access. [#6303](https://github.com/gravitational/teleport/pull/6303)
* Added `GetSessionEventsRequest` RPC endpoint for Audit Log pagination. [RFD 19](https://github.com/gravitational/teleport/blob/master/rfd/0019-event-iteration-api.md) [#6731](https://github.com/gravitational/teleport/pull/6731)
* Changed DynamoDB indexing strategy for events. [RFD 24](https://github.com/gravitational/teleport/blob/master/rfd/0024-dynamo-event-overflow.md) [#6583](https://github.com/gravitational/teleport/pull/6583)

### Fixes

* Fixed multiple per-session MFA issues. [#6542](https://github.com/gravitational/teleport/pull/6542) [#6567](https://github.com/gravitational/teleport/pull/6567) [#6625](https://github.com/gravitational/teleport/pull/6625) [#6779](https://github.com/gravitational/teleport/pull/6779) [#6948](https://github.com/gravitational/teleport/pull/6948)
* Fixed etcd JWT renewal issue. [#6905](https://github.com/gravitational/teleport/pull/6905)
* Fixed issue where `kubectl exec` sessions were not being recorded when the target pod was killed. [#6068](https://github.com/gravitational/teleport/pull/6068)
* Fixed an issue that prevented Teleport from starting on ARMv7 systems. [#6711](https://github.com/gravitational/teleport/pull/6711).
* Fixed issue that caused Access Requests to inconsistently allow elevated Kubernetes access. [#6492](https://github.com/gravitational/teleport/pull/6492)
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
performance perform this migration with only one Auth Service instance online. It may
take some time and progress will be periodically written to the Auth Service
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

* Added support for PROXY protocol to database access (MySQL). [#6517](https://github.com/gravitational/teleport/issues/6517)

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

For more details see [Per-Session
MFA](docs/pages/admin-guides/access-controls/guides/per-session-mfa.mdx) documentation or
[RFD
14](https://github.com/gravitational/teleport/blob/master/rfd/0014-session-2FA.md)
and [RFD
15](https://github.com/gravitational/teleport/blob/master/rfd/0015-2fa-management.md)
for technical details.

#### Dual Authorization Workflows

Added ability to request multiple users to review and approve Access Requests.

See [#5071](https://github.com/gravitational/teleport/pull/5071) for technical details.

### Improvements

* Added the ability to propagate SSO claims to PAM modules. [#6158](https://github.com/gravitational/teleport/pull/6158)
* Added support for cluster routing to reduce latency to leaf clusters. [RFD 21](https://github.com/gravitational/teleport/blob/master/rfd/0021-cluster-routing.md)
* Added support for Google Cloud SQL to database access. [#6090](https://github.com/gravitational/teleport/pull/6090)
* Added support CLI credential issuance for application access. [#5918](https://github.com/gravitational/teleport/pull/5918)
* Added support for Encrypted SAML Assertions. [#5598](https://github.com/gravitational/teleport/pull/5598)
* Added support for user impersonation. [#6073](https://github.com/gravitational/teleport/pull/6073)

### Fixes

* Fixed interoperability issues with `gpg-agent`. [RFD 18](http://github.com/gravitational/teleport/blob/master/rfd/0018-agent-loading.md)
* Fixed websocket support in application access. [#6028](https://github.com/gravitational/teleport/pull/6028)
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

We have implemented [database access](./docs/pages/enroll-resources/database-access/database-access.mdx),
open sourced role-based access control (RBAC), and added official API and a Go client library.

Users can review the [6.0 milestone](https://github.com/gravitational/teleport/milestone/33?closed=1) on Github for more details.

### New Features

#### Database access

Review the database access design in [RFD #11](https://github.com/gravitational/teleport/blob/master/rfd/0011-database-access.md).

With database access users can connect to PostgreSQL and MySQL databases using short-lived certificates, configure SSO authentication and role-based access controls for databases, and capture SQL query activity in the audit log.

##### Getting Started

Configure database access following the [Getting Started](./docs/pages/enroll-resources/database-access/getting-started.mdx/) guide.

##### Guides

* [AWS RDS/Aurora
  PostgreSQL](./docs/pages/enroll-resources/database-access/enroll-aws-databases/rds.mdx)
* [AWS RDS/Aurora MySQL](./docs/pages/enroll-resources/database-access/enroll-aws-databases/rds.mdx)
* [Self-hosted PostgreSQL](./docs/pages/enroll-resources/database-access/enroll-self-hosted-databases/postgres-self-hosted.mdx)
* [Self-hosted MySQL](./docs/pages/enroll-resources/database-access/enroll-self-hosted-databases/mysql-self-hosted.mdx)
* [GUI clients](docs/pages/connect-your-client/gui-clients.mdx)

##### Resources

To learn more about configuring role-based access control for database access, check out the [RBAC](./docs/pages/enroll-resources/database-access/database-access.mdx) section.

[Architecture](./docs/pages/enroll-resources/database-access/database-access.mdx) provides a more in-depth look at database access internals such as networking and security.

See [Reference](docs/pages/reference/agent-services/database-access-reference/database-access-reference.mdx) for an overview of database access related configuration and CLI commands.

Finally, check out [Frequently Asked Questions](docs/pages/enroll-resources/database-access/faq.mdx).

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
* Added Proxy Protocol support to Kubernetes access in [#5299](https://github.com/gravitational/teleport/pull/5299).
* Added ACME ([Let's Encrypt](https://letsencrypt.org/)) support to make getting and using TLS certificates easier. [#5177](https://github.com/gravitational/teleport/issues/5177).
* Added the ability to manage local users to the Web UI in [#2945](https://github.com/gravitational/teleport/issues/2945).
* Added the ability to preserve timestamps when using `tsh scp` in [#2889](https://github.com/gravitational/teleport/issues/2889).

### Fixes

* Fixed authentication failure when logging in via CLI with Access Workflows after removing `.tsh` directory in [#5323](https://github.com/gravitational/teleport/pull/5323).
* Fixed `tsh login` failure when `--proxy` differs from actual proxy public address in [#5380](https://github.com/gravitational/teleport/pull/5380).
* Fixed session playback issues in [#2945](https://github.com/gravitational/teleport/issues/2945).
* Fixed several UX issues in [#5559](https://github.com/gravitational/teleport/issues/5559), [#5568](https://github.com/gravitational/teleport/issues/5568), [#4965](https://github.com/gravitational/teleport/issues/4965), and [#5057](https://github.com/gravitational/teleport/pull/5057).

### Upgrade Notes

Please follow our [standard upgrade procedure](docs/pages/admin-guides/management/admin/admin.mdx) to upgrade your cluster.

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

Any Enterprise SSO users using Okta, Active Directory, OneLogin or custom SAML connectors should upgrade their Auth Service to version 5.0.2 and restart Teleport. If you are unable to upgrade immediately, we suggest disabling SAML connectors for all clusters until the updates can be applied.


## 5.0.1 

This release of Teleport contains multiple bug fixes.

* Always set expiry times on server resource in heartbeats [#5008](https://github.com/gravitational/teleport/pull/5008)
* Fixes streaming k8s responses (`kubectl logs -f`, `kubectl run -it`, etc) [#5009](https://github.com/gravitational/teleport/pull/5009)
* Multiple fixes for the k8s forwarder [#5038](https://github.com/gravitational/teleport/pull/5038)

## 5.0.0 

Teleport 5.0 is a major release with new features, functionality, and bug fixes. Users can review [5.0 closed issues](https://github.com/gravitational/teleport/milestone/39?closed=1) on Github for details of all items.

### New Features

Teleport 5.0 introduces two distinct features: Teleport application access and significant Kubernetes access improvements - multi-cluster support.

#### Teleport application access

Teleport can now be used to provide secure access to web applications. This new feature was built with the express intention of securing internal apps which might have once lived on a VPN or had an authorization and authentication mechanism with little to no audit trail. application access works with everything from dashboards to single page Javascript applications (SPA).

Application access uses mutually authenticated reverse tunnels to establish a secure connection with the Teleport unified Access Platform which can then becomes the single ingress point for all traffic to an internal application.

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
   # Teleport application access is enabled.
   enabled: yes
   # We've added a default sample app that will check
   # that Teleport application access is working
   # and output JWT tokens.
   # https://dumper.teleport.example.com:3080/
   debug_app: true
   apps:
   # application access can be used to proxy any HTTP endpoint.
   # Note: Name can't include any spaces and should be DNS-compatible A-Za-z0-9-._
   - name: "internal-dashboard"
     uri: "http://10.0.1.27:8000"
     # By default Teleport will make this application
     # available on a sub-domain of your Teleport proxy's hostname
     # internal-dashboard.teleport.example.com
     # - thus the importance of setting up wildcard DNS.
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
     # A proxy can support multiple applications. application access
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

You can learn more in [Introduction to Enrolling Applications](./docs/pages/enroll-resources/application-access/introduction.mdx).

#### Teleport Kubernetes access

Teleport 5.0 also introduces two highly requested features for Kubernetes.

* The ability to connect multiple Kubernetes Clusters to the Teleport Access Platform, greatly reducing operational complexity.
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

Learn more about [Teleport's RBAC Resources](docs/pages/admin-guides/access-controls/access-controls.mdx)

##### Cluster Labels

Teleport 5.0 also adds the ability to set labels on Trusted Clusters. The labels are set when creating a trusted cluster invite token. This lets teams use the same RBAC controls used on nodes to approve or deny access to clusters. This can be especially useful for MSPs that connect hundreds of customers' clusters - when combined with access workflows, cluster access can be delegated. Learn more by reviewing our [Truster Cluster Setup & RBAC Docs](docs/pages/admin-guides/management/admin/trustedclusters.mdx)

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

* We now provide local user management via `https://[cluster-url]/web/users`, providing the ability to edit, reset and delete local users.
* Teleport Node & App Install scripts. This is currently an Enterprise-only feature that provides customers with an 'auto-magic' installer script. Enterprise customers can enable this feature by modifying the 'token' resource. See note above.
* We've added a Waiting Room for customers using Access Workflows. [Docs](docs/pages/admin-guides/access-controls/access-request-plugins/access-request-plugins.mdx)

##### Signed RPM and Releases

Starting with Teleport 5.0, we now provide an RPM repo for stable releases of Teleport. We've also started signing our RPMs to provide assurance that you're always using an official build of Teleport.

See https://rpm.releases.teleport.dev/ for more details.

### Improvements

* Added `--format=json` playback option for `tsh play`. For example `tsh play --format=json ~/play/0c0b81ed-91a9-4a2a-8d7c-7495891a6ca0.tar | jq '.event` can be used to show all events within an a local archive. [#4578](https://github.com/gravitational/teleport/issues/4578)
* Added support for continuous backups and auto scaling for DynamoDB. [#4780](https://github.com/gravitational/teleport/issues/4780)
* Added a Linux ARM64/ARMv8 (64-bit) Release. [#3383](https://github.com/gravitational/teleport/issues/3383)
* Added `https_keypairs` field which replaces `https_key_file` and `https_cert_file`. This allows administrators to load multiple HTTPS certs for Teleport application access. Teleport 5.0 is backwards compatible with the old format, but we recommend updating your configuration to use `https_keypairs`.

Enterprise Only:

* `tctl` can load credentials from `~/.tsh` [#4678](https://github.com/gravitational/teleport/pull/4678)
* Teams can require a user submitted reason when using Access Workflows [#4573](https://github.com/gravitational/teleport/pull/4573#issuecomment-720777443)

### Fixes

* Updated `tctl` to always format resources as lists in JSON/YAML. [#4281](https://github.com/gravitational/teleport/pull/4281)
* Updated `tsh status` to now print Kubernetes status. [#4348](https://github.com/gravitational/teleport/pull/4348)
* Fixed intermittent issues with `loginuid.so`. [#3245](https://github.com/gravitational/teleport/issues/3245)
* Reduced `access denied to Proxy` log spam. [#2920](https://github.com/gravitational/teleport/issues/2920)
* Various AMI fixes: paths are now consistent with other Teleport packages and configuration files will not be overwritten on reboot.

### Documentation

We've added an [API Guide](docs/pages/admin-guides/api/api.mdx) to simply developing applications against Teleport.

### Upgrade Notes

Please follow our [standard upgrade procedure](docs/pages/upgrading/upgrading.mdx).

* Optional: Consider updating `https_key_file` & `https_cert_file` to our new `https_keypairs:` format.
* Optional: Consider migrating Kubernetes access from `proxy_service` to `kubernetes_service` after the upgrade.

## 4.4.6 

This release of teleport contains a security fix and a bug fix.

* Patch a SAML authentication bypass (see https://github.com/russellhaering/gosaml2/security/advisories/GHSA-xhqq-x44f-9fgg): [#5120](https://github.com/gravitational/teleport/pull/5120).

Any Enterprise SSO users using Okta, Active Directory, OneLogin or custom SAML connectors should upgrade their Auth Service to version 4.4.6 and restart Teleport. If you are unable to upgrade immediately, we suggest disabling SAML connectors for all clusters until the updates can be applied.

* Fix an issue where `tsh login` would fail with an `AccessDenied` error if
the user was perviously logged into a leaf cluster. [#5105](https://github.com/gravitational/teleport/pull/5105)

## 4.4.5 

This release of Teleport contains a bug fix.

* Fixed an issue where a slow or unresponsive Teleport Auth Service instance could hang client connections in async recording mode. [#4696](https://github.com/gravitational/teleport/pull/4696)

## 4.4.4 

This release of Teleport adds enhancements to the Access Workflows API.

* Support for creating limited roles that trigger Access Requests
on login, allowing users to be configured such that no nodes can
be accessed without externally granted roles.

* Teleport UI support for automatically generating Access Requests and
assuming new roles upon approval (Access Requests were previously
only available in `tsh`).

* New `claims_to_roles` mapping that can use claims from external
identity providers to determine which roles a user can request.

* Various minor API improvements to help make requests easier to
manage and audit, including support for human-readable
request/approve/deny reasons and structured annotations.

## 4.4.2 

This release of Teleport adds support for a new build architecture.

* Added automatic arm64 builds of Teleport to the download portal.

## 4.4.1 

This release of Teleport contains a bug fix.

* Fixed an issue where defining multiple logging configurations would cause Teleport to crash. [#4598](https://github.com/gravitational/teleport/issues/4598)

## 4.4.0 

This is a major Teleport release with a focus on new features, functionality, and bug fixes. It’s a substantial release and users can review [4.4 closed issues](https://github.com/gravitational/teleport/milestone/40?closed=1) on Github for details of all items.

### New Features

#### Concurrent Session Control

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
NOTE: These experimental modes require all Teleport Auth Service instances, Proxy Service instances, and nodes to be running Teleport 4.4.

```yaml
# This section configures the Auth Service:
auth_service:
    # Optional setting for configuring session recording. Possible values are:
    #     "node"  : sessions will be recorded on the node level (the default)
    #     "proxy" : recording on the proxy level, see "recording proxy mode" section.
    #     "off"   : session recording is turned off
    #
    #     EXPERIMENTAL *-sync modes: proxy and node send logs directly to S3 or other
    #     storage without storing the records on disk at all. This mode will kill a
    #     connection if network connectivity is lost.
    #     NOTE: These experimental modes require all Teleport Auth Service instances, 
    #     Proxy Service instances, and nodes to be running Teleport 4.4.
    #
    #     "node-sync" : sessions recording will be streamed from node -> auth -> storage
    #     "proxy-sync : sessions recording will be streamed from proxy -> auth -> storage
    #
    session_recording: "node-sync"
```

### Improvements

* Added session streaming. [#4045](https://github.com/gravitational/teleport/pull/4045)
* Added concurrent session control. [#4138](https://github.com/gravitational/teleport/pull/4138)
* Added ability to specify leaf cluster when generating `kubeconfig` via `tctl auth sign`. [#4446](https://github.com/gravitational/teleport/pull/4446)
* Added output options (like JSON) for `tsh ls`. [#4390](https://github.com/gravitational/teleport/pull/4390)
* Added node ID to heartbeat debug log [#4291](https://github.com/gravitational/teleport/pull/4291)
* Added the option to trigger `pam_authenticate` on login [#3966](https://github.com/gravitational/teleport/pull/3966)

### Fixes

* Fixed issue that caused some idle `kubectl exec` sessions to terminate. [#4377](https://github.com/gravitational/teleport/pull/4377)
* Fixed symlink issued when using `tsh` on Windows. [#4347](https://github.com/gravitational/teleport/pull/4347)
* Fixed `tctl top` so it runs without the debug flag and on dark terminals. [#4282](https://github.com/gravitational/teleport/pull/4282) [#4231](https://github.com/gravitational/teleport/pull/4231)
* Fixed issue that caused DynamoDB not to respect HTTP CONNECT proxies. [#4271](https://github.com/gravitational/teleport/pull/4271)
* Fixed `/readyz` endpoint to recover much quicker. [#4223](https://github.com/gravitational/teleport/pull/4223)

### Documentation

* Updated Google Workspace documentation to add clarification on supported account types. [#4394](https://github.com/gravitational/teleport/pull/4394)
* Updated IoT instructions on necessary ports. [#4398](https://github.com/gravitational/teleport/pull/4398)
* Updated Trusted Cluster documentation on how to remove trust from root and leaf clusters. [#4358](https://github.com/gravitational/teleport/pull/4358)
* Updated the PAM documentation with PAM authentication usage information. [#4352](https://github.com/gravitational/teleport/pull/4352)

### Upgrade Notes

Please follow our [standard upgrade
procedure](docs/pages/upgrading/upgrading.mdx).

## 4.3.9 

This release of Teleport contains a security fix.

* Patch a SAML authentication bypass (see https://github.com/russellhaering/gosaml2/security/advisories/GHSA-xhqq-x44f-9fgg): [#5122](https://github.com/gravitational/teleport/pull/5122).

Any Enterprise SSO users using Okta, Active Directory, OneLogin or custom SAML connectors should upgrade their Auth Service to version 4.3.9 and restart Teleport. If you are unable to upgrade immediately, we suggest disabling SAML connectors for all clusters until the updates can be applied.

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
Active Directory, OneLogin or custom SAML connectors should upgrade their Auth Service to version 4.3.7 and restart Teleport.

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

Teleport 4.3 includes a completely redesigned Web UI. The new Web UI expands the management functionality of a Teleport cluster and the user experience of using Teleport. Teleport's new terminal provides a jumping-off point to access nodes and nodes on other clusters via the web.

Teleport's Web UI now exposes Teleport’s Audit log, letting auditors and administrators view Teleport access events, SSH events, recording session, and enhanced session recording all in one view.

##### Teleport Plugins

Teleport 4.3 introduces four new plugins that work out of the box with [Approval Workflow](docs/pages/admin-guides/access-controls/access-request-plugins/access-request-plugins.mdx). These plugins allow you to automatically support role escalation with commonly used third party services. The built-in plugins are listed below.

*   [PagerDuty](docs/pages/admin-guides/access-controls/access-request-plugins/ssh-approval-pagerduty.mdx)
*   [Jira](docs/pages/admin-guides/access-controls/access-request-plugins/ssh-approval-jira.mdx)
*   [Slack](docs/pages/admin-guides/access-controls/access-request-plugins/ssh-approval-slack.mdx)
*   [Mattermost](docs/pages/admin-guides/access-controls/access-request-plugins/ssh-approval-mattermost.mdx)

### Improvements

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

### Fixes

*   Fixed removing existing user definitions in kubeconfig. [#3209](https://github.com/gravitational/teleport/issues/3749)
*   Fixed an issue where port forwarding could fail in certain circumstances. [#3749](https://github.com/gravitational/teleport/issues/3749)
*   Fixed temporary role grants issue when forwarding Kubernetes requests. [#3624](https://github.com/gravitational/teleport/pull/3624)
*   Fixed an issue that prevented copy/paste in the web termination. [#92](https://github.com/gravitational/webapps/issues/92)
*   Fixed an issue where the proxy did not test Kubernetes permissions at startup. [#3812](https://github.com/gravitational/teleport/pull/3812)
*   Fixed `tsh` and `gpg-agent` integration. [#3169](https://github.com/gravitational/teleport/issues/3169)
*   Fixed Vulnerabilities in Teleport Docker Image [https://quay.io/repository/gravitational/teleport?tab=tags](https://quay.io/repository/gravitational/teleport?tab=tags)

#### Upgrade Notes

Always follow the [recommended upgrade
procedure](docs/pages/upgrading/upgrading.mdx) to upgrade to this version.

##### New Signing Algorithm

If you’re upgrading an existing version of Teleport, you may want to consider rotating CA to SHA-256 or SHA-512 for RSA SSH certificate signatures. The previous default was SHA-1, which is now considered to be weak against brute-force attacks. SHA-1 certificate signatures are also [no longer accepted](https://www.openssh.com/releasenotes.html) by OpenSSH versions 8.2 and above. All new Teleport clusters will default to SHA-512 based signatures. To upgrade an existing cluster, set the following in your `teleport.yaml`:

```
teleport:
    ca_signature_algo: "rsa-sha2-512"
```

Rotate the cluster CA, following [these
docs](docs/pages/admin-guides/management/operations/ca-rotation.mdx).

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

The minimum set of Kubernetes permissions that need to be granted to Teleport
proxies has been updated. If you use the Kubernetes integration, please make
sure that the ClusterRole used by the proxy has [sufficient
permissions](./docs/pages/enroll-resources/kubernetes-access/controls.mdx).

##### Path prefix for etcd

The [etcd backend](docs/pages/reference/backends.mdx#etcd) now correctly uses
the “prefix” config value when storing data. Upgrading from 4.2 to 4.3 will
migrate the data as needed at startup. Make sure you follow our Teleport
[upgrade guidance](docs/pages/upgrading/upgrading.mdx).

**Note: If you use an etcd backend with a non-default prefix and need to downgrade from 4.3 to 4.2, you should [backup Teleport data and restore it](docs/pages/admin-guides/management/operations/backup-restore.mdx) into the downgraded cluster.**

## 4.2.12 

This release of Teleport contains a security fix.

* Mitigated [CVE-2020-15216](https://nvd.nist.gov/vuln/detail/CVE-2020-15216) by updating github.com/russellhaering/goxmldsig.

### Details
A vulnerability was discovered in the `github.com/russellhaering/goxmldsig` library which is used by Teleport to validate the
signatures of XML files used to configure SAML 2.0 connectors. With a carefully crafted XML file, an attacker can completely
bypass XML signature validation and pass off an altered file as a signed one.

### Actions
The `goxmldsig` library has been updated upstream and Teleport 4.2.12 includes the fix. Any Enterprise SSO users using Okta,
Active Directory, OneLogin or custom SAML connectors should upgrade their Auth Service to version 4.2.12 and restart Teleport.

If you are unable to upgrade immediately, we suggest deleting SAML connectors for all clusters until the updates can be applied.

## 4.2.11 

This release of Teleport contains multiple bug fixes.

* Fixed an issue that prevented upload of session archives to NFS volumes. [#3780](https://github.com/gravitational/teleport/pull/3780)
* Fixed an issue with port forwarding that prevented TCP connections from being closed correctly. [#3801](https://github.com/gravitational/teleport/pull/3801)
* Fixed an issue in `tsh` that would cause connections to the Auth Service to fail on large clusters. [#3872](https://github.com/gravitational/teleport/pull/3872)
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

* Fixed a regression in reissuing certificate that could cause nodes to not start. [#3449](https://github.com/gravitational/teleport/pull/3449)

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
* Improved variety of issues with Enhanced Session Recording including support for more operating systems and install from packages. [#3279](https://github.com/gravitational/teleport/pull/3279)

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
* Alpha: Workflows API lets admins escalate RBAC roles in response to user requests. [Read the docs](docs/pages/admin-guides/access-controls/access-requests/access-requests.mdx). [#3006](https://github.com/gravitational/teleport/issues/3006)
* Beta: Teleport provides HA Support using Firestore and Google Cloud Storage using Google Cloud Platform. [Read the docs](docs/pages/admin-guides/deploy-a-cluster/deployments/gcp.mdx). [#2821](https://github.com/gravitational/teleport/pull/2821)
* Remote tctl execution is now possible. [Read the docs](./docs/pages/reference/cli/tctl.mdx). [#1525](https://github.com/gravitational/teleport/issues/1525) [#2991](https://github.com/gravitational/teleport/issues/2991)

### Fixes

* Fixed issue in socks4 when rendering remote address [#3110](https://github.com/gravitational/teleport/issues/3110)

### Documentation

* Adopting root/leaf terminology for trusted clusters. [Trusted cluster documentation](docs/pages/admin-guides/management/admin/trustedclusters.mdx).
* Documented Teleport FedRAMP & FIPS Support. [FedRAMP & FIPS documentation](docs/pages/admin-guides/access-controls/compliance-frameworks/fedramp.mdx).

## 4.1.13 

This release of Teleport contains a bug fix.

* Fixed issue where the port forwarding option in a role was ignored. [#3208](https://github.com/gravitational/teleport/pull/3208)

## 4.1.11 

This release of Teleport contains a security fix.

* Mitigated [CVE-2020-15216](https://nvd.nist.gov/vuln/detail/CVE-2020-15216) by updating github.com/russellhaering/goxmldsig.

### Details
A vulnerability was discovered in the `github.com/russellhaering/goxmldsig` library which is used by Teleport to validate the
signatures of XML files used to configure SAML 2.0 connectors. With a carefully crafted XML file, an attacker can completely
bypass XML signature validation and pass off an altered file as a signed one.

### Actions
The `goxmldsig` library has been updated upstream and Teleport 4.1.11 includes the fix. Any Enterprise SSO users using Okta,
Active Directory, OneLogin or custom SAML connectors should upgrade their Auth Service to version 4.1.11 and restart Teleport.

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

## 4.0.12 

This release of Teleport contains a bug fix.

* Fixed an issue that caused Teleport not to start with certain OIDC claims. [#3053](https://github.com/gravitational/teleport/issues/3053)

## 4.0.11 

This release of Teleport adds support for an older version of Linux.

* Added RHEL/CentOS 6.x builds to the build pipeline. [#3175](https://github.com/gravitational/teleport/pull/3175)

## 4.0.10 

This release of Teleport contains a bug fix.

* Fixed a goroutine leak that occurred whenever a leaf cluster disconnected from the root cluster. [#3037](https://github.com/gravitational/teleport/pull/3037)

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

* Teleport now support 10,000 remote connections to a single Teleport cluster. [Using our recommend hardware setup.](docs/pages/admin-guides/management/operations/scaling.mdx)
* Added ability to delete node using `tctl rm`. [#2685](https://github.com/gravitational/teleport/pull/2685)
* Output of `tsh ls` is now sorted by node name. [#2534](https://github.com/gravitational/teleport/pull/2534)

### Bug Fixes

* Switched to `xdg-open` to open a browser window on Linux. [#2536](https://github.com/gravitational/teleport/pull/2536)
* Increased SSO callback timeout to 180 seconds. [#2533](https://github.com/gravitational/teleport/pull/2533)
* Set permissions on TTY similar to OpenSSH. [#2508](https://github.com/gravitational/teleport/pull/2508)

The lists of improvements and bug fixes above mention only the significant changes, please take a look at the complete list on Github for more.

### Upgrading

Teleport 4.0 is backwards compatible with Teleport 3.2 and later. [Follow the recommended upgrade procedure to upgrade to this version.](docs/pages/upgrading/upgrading.mdx)

Note that due to substantial changes between Teleport 3.2 and 4.0, we recommend creating a backup of the backend datastore (DynamoDB, etcd, or dir) before upgrading a cluster to Teleport 4.0 to allow downgrades.

#### Notes on compatibility

Teleport has always validated host certificates when a client connects to a server, however prior to Teleport 4.0, Teleport did not validate the host the user requests a connection to is in the list of principals on the certificate. To avoid issues during the upgrade, make sure the hosts you connect to have the appropriate address set in `public_addr` in `teleport.yaml` before upgrading.

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

## 3.2.0 

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

Teleport 3.1.3 contains two security fixes.

#### Bugfixes

* Updated xterm.js to mitigate a [RCE in xterm.js](https://github.com/xtermjs/xterm.js/releases/tag/3.10.1).
* Mitigate potential timing attacks during bearer token authentication. [#2482](https://github.com/gravitational/teleport/pull/2482)
* Fixed `x509: certificate signed by unknown authority` error when connecting to DynamoDB within Gravitational publish Docker image. [#2473](https://github.com/gravitational/teleport/pull/2473)

## 3.1.2 

Teleport 3.1.2 contains a security fix. We strongly encourage anyone running Teleport 3.1.1 to upgrade.

#### Bugfixes

* Due to the flaw in internal RBAC verification logic, a compromised node, trusted cluster or authenticated non-privileged user can craft special request to Teleport's internal Auth Service API to gain access to the private key material of the cluster's internal certificate authorities and elevate their privileges to gain full administrative access to the Teleport cluster. This vulnerability only affects authenticated clients, there is no known way to exploit this vulnerability outside the cluster for unauthenticated clients.

## 3.1.1 

Teleport 3.1.1 contains a security fix. We strongly encourage anyone running Teleport 3.1.0 to upgrade.

* Upgraded Go to 1.11.4 to mitigate CVE-2018-16875: [CPU denial of service in chain validation](https://golang.org/issue/29233) Go. For customers using the RHEL5.x compatible release of Teleport, we've backported this fix to Go 1.9.7, before releasing RHEL 5.x compatible binaries.

## 3.1.0 

This is a major Teleport release with a focus on backwards compatibility, stability, and bug fixes. Some of the improvements:

* Added support for regular expressions in RBAC label keys and values. [#2161](https://github.com/gravitational/teleport/issues/2161)
* Added support for configurable server side keepalives. [#2334](https://github.com/gravitational/teleport/issues/2334)
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

Teleport 3.0.4 contains two security fixes.

#### Bugfixes

* Updated xterm.js to mitigate a [RCE in xterm.js](https://github.com/xtermjs/xterm.js/releases/tag/3.10.1).
* Mitigate potential timing attacks during bearer token authentication. [#2482](https://github.com/gravitational/teleport/pull/2482)

## 3.0.3 

Teleport 3.0.3 contains a security fix. We strongly encourage anyone running Teleport 3.0.2 to upgrade.

#### Bugfixes

* Due to the flaw in internal RBAC verification logic, a compromised node, trusted cluster or authenticated non-privileged user can craft special request to Teleport's internal Auth Service API to gain access to the private key material of the cluster's internal certificate authorities and elevate their privileges to gain full administrative access to the Teleport cluster. This vulnerability only affects authenticated clients, there is no known way to exploit this vulnerability outside the cluster for unauthenticated clients.

## 3.0.2 

Teleport 3.0.2 contains a security fix. We strongly encourage anyone running Teleport 3.0.1 to upgrade.

* Upgraded Go to 1.11.4 to mitigate CVE-2018-16875: [CPU denial of service in chain validation](https://golang.org/issue/29233) Go. For customers using the RHEL5.x compatible release of Teleport, we've backported this fix to Go 1.9.7, before releasing RHEL 5.x compatible binaries.

## 3.0.1 

This release of Teleport contains the following bug fix:

* Fix regression that marked ADFS claims as invalid. [#2293](https://github.com/gravitational/teleport/pull/2293)

## 3.0.0 

This is a major Teleport release which introduces support for Kubernetes
clusters. In addition to this new feature this release includes several
usability and performance improvements listed below.

#### Kubernetes Support

* `tsh login` can retrieve and install certificates for both Kubernetes and SSH
  at the same time.
* Full audit log support for `kubectl` commands, including recording of the sessions
  if `kubectl exec` command was interactive.
* Unified (AKA "single pane of glass") RBAC for both SSH and Kubernetes permissions.

### Improvements

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

Follow the [recommended upgrade
procedure](docs/pages/upgrading/upgrading.mdx) to upgrade to this
version.

**WARNING:** if you are using Teleport with the etcd back-end, make sure your
`etcd` version is 3.3 or newer prior to upgrading to Teleport 3.0.

## 2.7.9 

Teleport 2.7.9 contains a security fix.

#### Bug fixes

* Upgraded Go to 1.11.5 to mitigate [CVE-2019-6486](https://groups.google.com/forum/#!topic/golang-announce/mVeX35iXuSw): CPU denial of service in P-521 and P-384 elliptic curve implementation.

## 2.7.8 

Teleport 2.7.8 contains two security fixes.

#### Bugfixes

* Updated xterm.js to mitigate a [RCE in xterm.js](https://github.com/xtermjs/xterm.js/releases/tag/3.10.1).
* Mitigate potential timing attacks during bearer token authentication. [#2482](https://github.com/gravitational/teleport/pull/2482)

## 2.7.7 

Teleport 2.7.7 contains two security fixes. We strongly encourage anyone running Teleport 2.7.6 to upgrade.

#### Bugfixes

* Due to the flaw in internal RBAC verification logic, a compromised node, trusted cluster or authenticated non-privileged user can craft special request to Teleport's internal Auth Service API to gain access to the private key material of the cluster's internal certificate authorities and elevate their privileges to gain full administrative access to the Teleport cluster. This vulnerability only affects authenticated clients, there is no known way to exploit this vulnerability outside the cluster for unauthenticated clients.
* Upgraded Go to 1.11.4 to mitigate CVE-2018-16875: CPU denial of service in chain validation Go.

## 2.7.6 

This release of Teleport contains the following bug fix:

* Fix regression that marked ADFS claims as invalid. [#2293](https://github.com/gravitational/teleport/pull/2293)

## 2.7.5 

This release of Teleport contains the following bug fix:

* Teleport Auth Service instances do not delete temporary files named `/tmp/multipart-` [#2250](https://github.com/gravitational/teleport/issues/2250)

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

As always, this release contains several bug fixes. The full list can be seen [here](https://github.com/gravitational/teleport/milestone/25?closed=1). Here are some notable ones:

* It is now possible to issue certificates with a long TTL via admin's `auth sign` tool. Previously they were limited to 30 hours for undocumented reason. [1745](https://github.com/gravitational/teleport/issues/1745)
* Dynamic label values were shown as empty strings. [2056](https://github.com/gravitational/teleport/issues/2056)

#### Upgrading

Follow the [recommended upgrade
procedure](docs/pages/upgrading/upgrading.mdx) to upgrade to this
version.

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
  migrating to role variables which are documented [here](docs/pages/admin-guides/access-controls/guides/role-templates.mdx)

* Resource names (like roles, connectors, trusted clusters) can no longer
  contain unicode or other special characters. Update the names of all user
  created resources to only include characters, hyphens, and dots.

* `advertise_ip` has been deprecated and replaced with `public_addr` setting. See [#1803](https://github.com/gravitational/teleport/issues/1803)
  The existing configuration files will still work, but we advise Teleport
  administrators to update it to reflect the new format.

* Teleport no longer uses `boltdb` back-end for storing cluster state _by
  default_.  The new default is called `dir` and it uses JSON files
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

Follow the [recommended upgrade
procedure](docs/pages/upgrading/upgrading.mdx) to upgrade to this
version.

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
  zero-downtime upgrades.  [See
  documentation](docs/pages/upgrading/upgrading.mdx).

* Dynamic join tokens for new nodes can now be explicitly set via `tctl node add --token`.
  This allows Teleport admins to use an external mechanism for generating
  cluster invitation tokens.
  [#1615](https://github.com/gravitational/teleport/pull/1615)

* Teleport now correctly manages certificates for accessing proxies behind a
  load balancer with the same domain name. The new configuration parameter
  `public_addr` must be used for this.
  [#1174](https://github.com/gravitational/teleport/issues/1174).

### Improvements

* Switching to a new TLS-based Auth Service API improves performance of large clusters.
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
* Update Signup URL. [#1643](https://github.com/gravitational/teleport/issues/1643)
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

### Improvements

* Updated documentation to accurately reflect 2.3 changes
* Web UI can use introspection so users can skip explicitly specifying SSH port [#1410](https://github.com/gravitational/teleport/issues/1410)

#### Bug fixes

* Fixed issue of MFA users getting prematurely locked out [#1347](https://github.com/gravitational/teleport/issues/1347)
* UI (regression) when invite link is expired, nothing is shown to the user  [#1400](https://github.com/gravitational/teleport/issues/1400)
* OIDC regression with some providers [#1371](https://github.com/gravitational/teleport/issues/1371)
* Legacy configuration for trusted clusters regression: [#1381](https://github.com/gravitational/teleport/issues/1381)
* Dynamic tokens for adding nodes: "access denied" [#1348](https://github.com/gravitational/teleport/issues/1348)

## 2.3.1 

#### Bug fixes

* Added CSRF protection to login endpoint. [#1356](https://github.com/gravitational/teleport/issues/1356)
* Proxy subsystem handling is more robust. [#1336](https://github.com/gravitational/teleport/issues/1336)

## 2.3.0 

This release focus was to increase Teleport user experience in the following areas:

* Easier configuration via `tctl` resource commands.
* Improved documentation, with expanded 'examples' directory.
* Improved CLI interface.
* Web UI improvements.

### Improvements

* Web UI: users can connect to OpenSSH servers using the Web UI.
* Web UI now supports arbitrary SSH logins, in addition to role-defined ones, for better compatibility with OpenSSH.
* CLI: trusted clusters can now be managed on the fly without having to edit Teleport configuration. [#1137](https://github.com/gravitational/teleport/issues/1137)
* CLI: `tsh login` supports exporting a user identity into a file to be used later with OpenSSH.
* `tsh agent` command has been deprecated: users are expected to use native SSH Agents on their platforms.

#### Teleport Enterprise

* More granular RBAC rules [#1092](https://github.com/gravitational/teleport/issues/1092)
* Role definitions now support templates. [#1120](https://github.com/gravitational/teleport/issues/1120)
* Authentication: Teleport now supports multiple OIDC/SAML endpoints.
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
* Improvements to Auth Service resiliency and availability. [#1071](https://github.com/gravitational/teleport/issues/1071)
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

* Roles created in the Web UI now have `node` resource. [#949](https://github.com/gravitational/teleport/pull/949)

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

## 2.0.0 

This is a major new release of Teleport.

### Features

* Native support for DynamoDB back-end for storing cluster state.
* It is now possible to turn off 2nd factor authentication.
* 2nd factor now uses TOTP. #522
* New framework for implementing secret storage plug-ins.
* Audit log format has been finalized and documented.
* Experimental file-based secret storage back-end.
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

## 1.3.0 

This release includes several major new features and it's recommended for production use.

### Features

* Support for hardware U2F keys for 2nd factor authentication.
* CLI client profiles: `tsh` can now remember its `--proxy` setting.
* tctl auth sign command to allow administrators to generate user session keys
* Web UI is now served directly from the executable. There is no more need for web
  assets in `/usr/local/share/teleport`

### Bugfixes

* Multiple Auth Service instances in config doesn't work if the last on is not reachable. #593
* `tsh scp -r` does not handle directory upload properly #606

## 1.2.0 

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
to Teleport Community Edition users.

### Bugfixes

* Guessing `advertise_ip` chooses IPv6 address space. #486

## 1.0.0 

The first official release of Teleport!
