# Changelog

## 18.1.7 (08/27/25)

**Warning:** This release includes a regression that prevents connecting to Windows desktops via the Web UI.
The following workarounds are available:
- Downgrade proxy servers to 18.1.4
- Use Teleport Connect instead of the web UI to access desktops
- Set your preferred keyboard layout (under account settings) to something other than _system_.

* Fixed an issue where VNet could not start because of "VNet is already running" error. [#58388](https://github.com/gravitational/teleport/pull/58388)
* Fix MCP icon displaying as white/black blocks. [#58347](https://github.com/gravitational/teleport/pull/58347)
* Fix crash when running 'teleport backend clone' on non-Linux platforms. [#58332](https://github.com/gravitational/teleport/pull/58332)
* Disabled MySQL database health checks to avoid MySQL blocking the Teleport Database Service for too many connection errors. MySQL health checks can be re-enabled by setting max_connect_errors on MySQL to its maximum value and setting the environment variable TELEPORT_ENABLE_MYSQL_DB_HEALTH_CHECKS=1 on the Teleport Database Service instance. [#58331](https://github.com/gravitational/teleport/pull/58331)
* Fixed incorrect scp exit status between OpenSSH clients and servers. [#58327](https://github.com/gravitational/teleport/pull/58327)
* Fixed sftp readdir failing due to broken symlinks. [#58320](https://github.com/gravitational/teleport/pull/58320)
* Added "MCP Servers" filter in resources view for Web UI and Teleport Connect. [#58309](https://github.com/gravitational/teleport/pull/58309)
* Enable separate request_object_mode setting for MFA flow in OIDC connectors. [#58281](https://github.com/gravitational/teleport/pull/58281)
* Allow a namespace to be specified for the `tbot` Kubernetes Secret destination. [#58203](https://github.com/gravitational/teleport/pull/58203)
* MWI: `tbot` now supports managing Argo CD clusters via the `kubernetes/argo-cd` output service. [#58200](https://github.com/gravitational/teleport/pull/58200)
* Fixed failure to close user accounting session. [#58163](https://github.com/gravitational/teleport/pull/58163)
* Add paginated API ListDatabases, deprecate GetDatabases. [#58105](https://github.com/gravitational/teleport/pull/58105)
* Prevent modifier keys from getting stuck during remote desktop sessions. [#58103](https://github.com/gravitational/teleport/pull/58103)
* Fixed AWS app access signature verification for AWS requests that use an unsigned payload. [#58085](https://github.com/gravitational/teleport/pull/58085)
* Windows desktop LDAP discovery now auto-populates the resource's description field. [#58082](https://github.com/gravitational/teleport/pull/58082)

Enterprise:
* For OIDC SSO, the IdP app/client configured for MFA checks is no longer expected to return claims that map to Teleport roles. Valid claim to role mappings are only required for login flows.
* Fix SSO MFA method for applications when Teleport is the SAML identity provider and Per-Session MFA is enabled.
* Added an optional session recording summarizer that uses OpenAI or a compatible API.

## 18.1.6 (08/20/25)

**Warning:** This release includes a regression that prevents connecting to Windows desktops via the Web UI.
The following workarounds are available:
- Downgrade proxy servers to 18.1.4
- Use Teleport Connect instead of the web UI to access desktops
- Set your preferred keyboard layout (under account settings) to something other than _system_.

* Fixed an uncaught exception in Teleport Connect on Windows when closing the app while the `TELEPORT_TOOLS_VERSION` environment variable is set. [#58131](https://github.com/gravitational/teleport/pull/58131)
* Fixed a Teleport Connect crash that occurred when assuming an access request while an application or database connection was active. [#58109](https://github.com/gravitational/teleport/pull/58109)
* Enable Azure joining with VMSS. [#58094](https://github.com/gravitational/teleport/pull/58094)
* Add support for JWT-Secured Authorization Requests to OIDC Connector. [#58063](https://github.com/gravitational/teleport/pull/58063)
* Fixed an issue that could cause some hosts not to register dynamic Windows desktops. [#58061](https://github.com/gravitational/teleport/pull/58061)
* TBot now emits a log message stating the current version on startup. [#58056](https://github.com/gravitational/teleport/pull/58056)
* Improve error message when a User without any MFA devices enrolled attempts to access a resource that requires MFA. [#58042](https://github.com/gravitational/teleport/pull/58042)
* Web assets are now pre-compressed with Brotli. [#58039](https://github.com/gravitational/teleport/pull/58039)
* Add TELEPORT_UNSTABLE_GRPC_RECV_SIZE env var which can be set to overwrite client side max grpc message size. [#58029](https://github.com/gravitational/teleport/pull/58029)

## 18.1.5 (08/18/25)

**Warning:** This release includes a regression that prevents connecting to Windows desktops via the Web UI.
The following workarounds are available:
- Downgrade proxy servers to 18.1.4
- Use Teleport Connect instead of the web UI to access desktops
- Set your preferred keyboard layout (under account settings) to something other than _system_.

* Fix AWS CLI access using AWS OIDC integration. [#57977](https://github.com/gravitational/teleport/pull/57977)
* Fixed an issue that could cause revocation checks to fail in Windows environments. [#57880](https://github.com/gravitational/teleport/pull/57880)
* Fixed the case where the auto-updated client tools did not use the intended version. [#57870](https://github.com/gravitational/teleport/pull/57870)
* Bound Keypair Joining: Fix lock generation on sequence desync. [#57863](https://github.com/gravitational/teleport/pull/57863)
* Fix database PKINIT issues caused missing CDP information in the certificate. [#57850](https://github.com/gravitational/teleport/pull/57850)
* Fixed connection issues to Windows Desktop Services (v17 or earlier) in Teleport Connect. [#57842](https://github.com/gravitational/teleport/pull/57842)
* The teleport-kube-agent Helm chart now supports kubernetes joining. `teleportClusterName` must be set to enable the feature. [#57824](https://github.com/gravitational/teleport/pull/57824)
* Fixed the web UI's access request submission panel getting stuck when scrolling down the page. [#57797](https://github.com/gravitational/teleport/pull/57797)
* Enroll new Kubernetes agents in Managed Updates. [#57784](https://github.com/gravitational/teleport/pull/57784)
* Teleport now supports displaying more than 2k tokens. [#57772](https://github.com/gravitational/teleport/pull/57772)
* Updated Go to 1.24.6. [#57764](https://github.com/gravitational/teleport/pull/57764)
* Database MCP server now supports CockroachDB databases. [#57762](https://github.com/gravitational/teleport/pull/57762)
* Added support for CockroachDB Web Access and interactive CockroachDB session playback. [#57762](https://github.com/gravitational/teleport/pull/57762)
* Added the `--auth` flag to the `tctl plugins install scim` CLI command to support Bearer token and OAuth authentication methods. [#57759](https://github.com/gravitational/teleport/pull/57759)
* Fix Alt+Click not being registered in remote desktop sessions. [#57757](https://github.com/gravitational/teleport/pull/57757)
* Kubernetes Access: `kubectl port-forward` now exits cleanly when backend pods are removed. [#57738](https://github.com/gravitational/teleport/pull/57738)
* Kubernetes Access: Fixed a bug when forwarding multiple ports to a single pod. [#57736](https://github.com/gravitational/teleport/pull/57736)
* Fixed unlink-package during upgrade/downgrade. [#57720](https://github.com/gravitational/teleport/pull/57720)
* Add new oidc joining mode for Kubernetes delegated joining to support providers that can be configured to provide public OIDC endpoints, like EKS, AKS, and GKE. [#57683](https://github.com/gravitational/teleport/pull/57683)
* Teleport `event-handler` now accepts HTTP Status Code 204 from the recipient. This adds support for sending events to Grafana Alloy and newer Fluentd versions. [#57680](https://github.com/gravitational/teleport/pull/57680)
* Enrich the windows.desktop.session.start audit event with additional certificate metadata. [#57676](https://github.com/gravitational/teleport/pull/57676)
* Allow the use of ResourceGroupsTaggingApi for KMS Key deletion. [#57671](https://github.com/gravitational/teleport/pull/57671)
* Added `--force` option to `tctl workload-identity x509-issuer-overrides sign-csrs` to allow displaying the output of partial failures, intended for use in clusters that make use of HSMs. [#57662](https://github.com/gravitational/teleport/pull/57662)
* Tctl top can now display raw prometheus metrics. [#57632](https://github.com/gravitational/teleport/pull/57632)
* Enable resource label conditions for notification routing rules. [#57616](https://github.com/gravitational/teleport/pull/57616)
* Use the bot details page to view and edit bot configuration, and see active instances with their upgrade status. [#57542](https://github.com/gravitational/teleport/pull/57542)
* Device Trust: added `required-for-humans` mode to allow bots to run on unenrolled devices, while enforcing checks for human users. [#57222](https://github.com/gravitational/teleport/pull/57222)
* Add `TeleportDatabaseV3` support to the Teleport Kubernetes Operator. [#56948](https://github.com/gravitational/teleport/pull/56948)
* Add `TeleportAppV3` support to the Teleport Kubernetes Operator. [#56948](https://github.com/gravitational/teleport/pull/56948)
* Fix TELEPORT_SESSION and SSH_SESSION_ID environmental variables not matching in an SSH session. [#55272](https://github.com/gravitational/teleport/pull/55272)

Enterprise:
* Allow OIDC authentication to complete if email verification is not provided when the OIDC connecter is set to enforce verified email addresses.

## 18.1.4 (08/06/25)

* Fixed access denied error messages not being displayed in the Teleport web UI PostgreSQL client. [#57568](https://github.com/gravitational/teleport/pull/57568)
* Fixed a bug in the default discovery script that can happen discovering instances whose PATH doesn't contain `/usr/local/bin`. [#57530](https://github.com/gravitational/teleport/pull/57530)

## 18.1.3 (08/05/25)

* Fixed a panic that may occur when fetching non-existent resources from the cache. [#57583](https://github.com/gravitational/teleport/pull/57583)
* Added support for consuming arbitrary JSON OIDC claims using the JSONPath query language. [#57570](https://github.com/gravitational/teleport/pull/57570)
* Made it easier to identify Windows desktop certificate issuance on the audit log page. [#57521](https://github.com/gravitational/teleport/pull/57521)
* Fixed a race condition in the Terraform Provider potentially causing "does not exist" errors the following resources: `auth_preference`, `autoupdate_config`, `autoupdate_version`, `cluster_maintenance_config`, `cluster_network_config`, and `session_recording_config`. [#57518](https://github.com/gravitational/teleport/pull/57518)
* Fixed a Terraform provider bug causing resource creation to be retried more times than the `MaxRetries` setting. [#57518](https://github.com/gravitational/teleport/pull/57518)
* Fixed a Terraform provider bug happening when `autoupdate_version` or `autoupdate_config` have non-empty metadata. [#57516](https://github.com/gravitational/teleport/pull/57516)

## 18.1.2 (08/05/25)

* Fix a bug on Windows where a forwarded SSH agent would become dysfunctional after a single connection using the agent. [#57511](https://github.com/gravitational/teleport/pull/57511)
* Fixed usage print for global `--help` flag. [#57451](https://github.com/gravitational/teleport/pull/57451)
* Added Cursor and VSCode install buttons in MCP connect dialog in Web UI. [#57362](https://github.com/gravitational/teleport/pull/57362)
* Added "Allowed Tools" to "tsh mcp ls" and show a warning if no tools allowed. [#57360](https://github.com/gravitational/teleport/pull/57360)
* Tctl top respects local teleport config file. [#57354](https://github.com/gravitational/teleport/pull/57354)
* Fixed an issue backfilling CRLs during startup for long-standing clusters. [#57321](https://github.com/gravitational/teleport/pull/57321)
* Disable NLA in FIPS mode. [#57307](https://github.com/gravitational/teleport/pull/57307)
* Added a configurable delay between receiving a termination signal and shutting down. [#57211](https://github.com/gravitational/teleport/pull/57211)

Enterprise:
* Slightly optimized access token refresh logic for Jamf integration when using API credentials.

## 18.1.1 (07/29/25)

* Fix CRL publication for Active Directory Windows desktop access. [#57264](https://github.com/gravitational/teleport/pull/57264)
* Allow YubiKeys running 5.7.4+ firmware to be usable as PIV hardware keys. [#57216](https://github.com/gravitational/teleport/pull/57216)
* Append headers to configuration files generated by teleport-update. [#56577](https://github.com/gravitational/teleport/pull/56577)

Enterprise:
* Fixed application crash that could occur when using GitHub personal access tokens that don't have an expiration date

## 18.1.0 (07/25/25)

### MCP server access

Teleport now provides the ability to connect to stdio-based MCP servers with
connection proxying and audit logging support.

### MCP for database access

Teleport now allows MCP clients such as Claude Desktop to execute queries in
Teleport-protected databases.

### VNet for SSH

Teleport VNet adds native support for SSH, enabling any SSH client to connect to
Teleport SSH servers with zero configuration. Advanced Teleport features like
per-session MFA have first-class support for a seamless user experience.

### Identifier-first login

Teleport adds support for identifier-first login flows. When enabled, the
initial login screen contains only a username prompt. Users are presented with
the SSO connectors that apply to them after submitting their username.

### Bound keypair joining for Machine ID

The new bound keypair join method for Machine ID is a more secure and
user-friendly alternative to token joining in both on-prem environments and
cloud providers without a delegated join method. It allows for automatic
self-recovery in case of expired client certificates and gives administrators
new options to manage and automate bot joining.

### Sailpoint SCIM integration

Teleport now supports Sailpoint as a SCIM provider allowing administrators to
synchronize Sailpoint entitlement groups with Teleport access lists.

### LDAP server discovery for desktop access

Teleport's `windows_desktop_service` can now locate the LDAP server via DNS as
an alternative to providing the address in the configuration file.

### Managed Updates canary support

Managed Updates v2 now support performing canary updates. When canary updates
are enabled for a group, Teleport will update a few agents first and confirm
they come back healthy before updating the rest of the group.

You can unable canary updates by setting `canary_count` in your
`autoupdate_config`:

```yaml
kind: autoupdate_config
spec:
  agents:
    mode: enabled
    schedules:
      regular:
      - name: dev
        days:
        - Mon
        - Tue
        - Wed
        - Thu
        start_hour: 20
        canary_count: 5
    strategy: halt-on-error
```

Each group can have a maximum of 5 canaries, canaries are picked randomly among
the connected agents.

Canary update support is currently only support by Linux agents, Kubernetes
support will be part of a future release.

### Improved access requests UX

Teleport's web UI makes a better distinction between just-in-time and long-term
access request UX.

### Other changes and improvements

* Fixed a bug causing `tctl`/`tsh` to fail on read-only file systems. [#57147](https://github.com/gravitational/teleport/pull/57147)
* The `teleport-distroless` container image now disables client tools updates by default (when using tsh/tctl, you will always use the version from the image). You can enable them back by unsetting the `TELEPORT_TOOLS_VERSION` environment variable. [#57147](https://github.com/gravitational/teleport/pull/57147)
* Fixed a crash in Teleport Connect that could occur when copying large clipboard content during desktop sessions. [#57130](https://github.com/gravitational/teleport/pull/57130)
* Audit log events for SPIFFE SVID issuances now include the name/label selector used by the client. [#57129](https://github.com/gravitational/teleport/pull/57129)
* Fixed an issue with `tsh aws` failing for STS and other AWS services. [#57122](https://github.com/gravitational/teleport/pull/57122)
* Fixed client tools managed updates downgrade to older version. [#57073](https://github.com/gravitational/teleport/pull/57073)
* Removed unnecessary macOS entitlements from Teleport Connect subprocesses. [#57066](https://github.com/gravitational/teleport/pull/57066)
* Machine and Workload ID: The `tbot` client will now discard expired identities if needed during renewal to allow automatic recovery without restarting the process. [#57060](https://github.com/gravitational/teleport/pull/57060)
* Defined `access-plugin` preset role. [#57056](https://github.com/gravitational/teleport/pull/57056)
* The `tctl top` command now supports the local unix sock debug endpoint. [#57025](https://github.com/gravitational/teleport/pull/57025)
* Added `--listen` flag to `tsh proxy db` for setting local listener address. [#57005](https://github.com/gravitational/teleport/pull/57005)
* Added multi-account support to teleport discovery bootstrap. [#56998](https://github.com/gravitational/teleport/pull/56998)
* Added `TeleportRoleV8` support to the Teleport Kubernetes Operator. [#56946](https://github.com/gravitational/teleport/pull/56946)
* Fixed a bug in the Teleport install scripts when running on macOS. The install scripts now error instead of trying to install non existing macOS FIPS binaries. [#56941](https://github.com/gravitational/teleport/pull/56941)
* Fixed using relative path `TELEPORT_HOME` environment variable with client tools managed update. [#56933](https://github.com/gravitational/teleport/pull/56933)
* Client tools managed updates support multi-cluster environments and track each version in the configuration file. [#56933](https://github.com/gravitational/teleport/pull/56933)
* Fixed certificate revocation failures in Active Directory environments when Teleport is using HSM-backed key material. [#56924](https://github.com/gravitational/teleport/pull/56924)
* Fixed database connect options dialog displaying wrong database username options. [#55560](https://github.com/gravitational/teleport/pull/55560)

Enterprise:
* Fixed SCIM user provisioning when a user already exists and is managed by the same connector as the SCIM integration.
* Added enrolment for a generic SCIM Integration.

## 18.0.2 (07/17/25)

* Fix backward compatibility issue introduced in the 17.5.5 / 18.0.1 release related to Access List type, causing the `unknown access_list type "dynamic"` validation error. [#56892](https://github.com/gravitational/teleport/pull/56892)
* Added support for glob-style matching to Spacelift join rules. [#56877](https://github.com/gravitational/teleport/pull/56877)
* Improve PKINIT compatibility by always including CDP information in the certificate. [#56875](https://github.com/gravitational/teleport/pull/56875)
* Update Application APIs to use pagination to avoid exceeding message size limitations. [#56727](https://github.com/gravitational/teleport/pull/56727)
* MWI: tbot's `/readyz` endpoint is now representative of the bot's health. [#56719](https://github.com/gravitational/teleport/pull/56719)

## 18.0.1 (07/15/25)

* Fixed backward compatibility for Access List 'membershipRequires is missing' for older terraform providers. [#56742](https://github.com/gravitational/teleport/pull/56742)
* Fixed VNet DNS configuration on Windows hosts joined to Active Directory domains. [#56738](https://github.com/gravitational/teleport/pull/56738)
* Updated default client timeout and upload rate for Pyroscope. [#56730](https://github.com/gravitational/teleport/pull/56730)
* Bot instances are now sortable by latest heartbeat time in the web UI. [#56696](https://github.com/gravitational/teleport/pull/56696)
* Enabled automatic reviews of resource requests. [#56690](https://github.com/gravitational/teleport/pull/56690)
* Updated Go to 1.24.5. [#56679](https://github.com/gravitational/teleport/pull/56679)
* Fixed `tbot` SPIFFE Workload API failing to renew SPIFFE SVIDs. [#56662](https://github.com/gravitational/teleport/pull/56662)
* Fixed some icons displaying as white/black blocks. [#56619](https://github.com/gravitational/teleport/pull/56619)
* Fixed Teleport Cache ListUsers pagination. [#56613](https://github.com/gravitational/teleport/pull/56613)
* Fixed duplicated `db_client` CA in `tctl status` and `tctl get cas` output. [#56563](https://github.com/gravitational/teleport/pull/56563)
* Added cross-account support for EC2 discovery. [#56535](https://github.com/gravitational/teleport/pull/56535)
* Terraform Provider: added support for skipping proxy certificate verification in development environments. [#56527](https://github.com/gravitational/teleport/pull/56527)
* Added support for CRD in access requests. [#56496](https://github.com/gravitational/teleport/pull/56496)
* Added `tctl autoupdate agents report` command. [#56495](https://github.com/gravitational/teleport/pull/56495)
* Made VNet DNS available over IPv4. [#56477](https://github.com/gravitational/teleport/pull/56477)
* Fixed missing Teleport Kube Operator permission in v18.0.0 causing the operator to fail. [#56466](https://github.com/gravitational/teleport/pull/56466)
* Trait role templating is now supported in the `workload_identity_labels` field. [#56296](https://github.com/gravitational/teleport/pull/56296)
* MWI: `tbot` no longer supports providing a proxy server address via `--auth-server` or `auth_server`, use `--proxy-server` or `proxy_server` instead. [#55818](https://github.com/gravitational/teleport/pull/55818)
* UX: Forbid creating Access Requests to user_group resources when Okta bidirectional sync is disabled. [#55585](https://github.com/gravitational/teleport/pull/55585)
* Teleport Connect: Added support for custom reason prompts. [#55557](https://github.com/gravitational/teleport/pull/55557)

Enterprise:
* Renamed Access Monitoring Rules to Access Automation Rules within the WebUI.
* Prevent the lack of an `email_verified` OIDC claim from failing authentication when the OIDC connecter is set to enforce verified email addresses.
* Fixed a email integration enrollment documentation link.
* Fixed a regression in SAML IdP that caused service provider initiated login to fail if the request was made with `http-redirect` binding encoding and the user had an active session in Teleport.

## 18.0.0 (07/03/25)

Teleport 18 brings the following new features and improvements:

* Identity Activity Center
* Automatic access request reviews
* Multi-session MFA for database access
* RBAC and device trust for SAML applications
* Database health checks
* Kubernetes CRD

### Description

#### Identity Activity Center

Teleport Identity Security, Identity Activity Center helps teams expose and
eliminate hidden identity risk in your infrastructure.  By correlating user
activity from multiple sources, it accelerates incident response to
identity-based attacks. The first iteration will support integrations with AWS
(CloudTrail), GitHub (Audit Log API), Okta (Audit API) and Teleport (Audit Log).

#### Automatic access request reviews

Teleport 18 includes built-in support for automatic access request reviews,
eliminating the need to run separate access request plugins. Automatic reviews
are enabled by setting up Access Monitoring Rules which define the conditions
that must be satisfied in order for a request to be automatically approved or
denied.

For more information, refer to the [docs](docs/pages/identity-governance/access-requests/automatic-reviews.mdx).

#### Multi-session MFA for database access

Per-session MFA has been extended to support multi-session reuse, allowing a
single MFA challenge to authorize multiple database connections using the new
`tsh db exec` command. This command executes a query across multiple selected
databases, making it user-friendly for ad-hoc user and script-friendly for
automation.

For more details, see the *database access examples* in the [per-session MFA
guide](docs/pages/zero-trust-access/authentication/per-session-mfa.mdx).

#### RBAC and device trust for SAML applications

Access to SAML IdP service provider resources can now be controlled with
resource labels. The resource labels are matched against `app_labels` defined in
user roles. Additionally, SAML IdP sessions now enforce device trust.

#### Database health checks

In Teleport 18, the database service performs regular health checks for
registered databases. Health status and any networking issues are reported in
the Teleport web UI and reflected in `db_server` resources.

In highly-available deployments with multiple database services, Teleport
prioritizes healthy services when routing user connections. For more
information, see the [database health checks
guide](docs/pages/enroll-resources/database-access/guides/health-checks.mdx).

#### Kubernetes CRD

In Teleport 18, the `kubernetes_resources` control of [role version
8](https://goteleport.com/docs/reference/resources/#role-versions) has been
updated to support Kubernetes Custom Resource Definitions and the behavior of
the `kind` and `namespace` fields has been updated to allow finer control. When
the  `kind`: `namespace` is set, it  will now only refer to the Kubernetes
namespace itself and not all resources within the namespace. The `kind` field
now expects the plural version of the resource name (i.e. `pods` instead of
`pod`) and a new field `api_group` has been added  which must match the apiGroup
that the Kubernetes resource belongs to.

##### Examples

A role which allows access to the CronTab CRD.

```yaml
kind: role
metadata:
  name: kube-access-v8
spec:
  allow:
    kubernetes_groups:
    - '{{internal.kubernetes_groups}}'
    kubernetes_labels:
      '*': '*'
    kubernetes_resources:
    - api_group: stable.example.com
      kind: crontabs
      name: '*'
      namespace: '*'
      verbs:
      - '*'
  deny: {}
version: v8
```

Converting a v7 Role to a v8 Role. Note the addition of the now required
`api_group` field and the change from **deployment** to **deployments** and from
**persistentvolume** to **persistentvolumes** for the `kind` field.

```yaml
kind: role
metadata:
  name: kube-access-v7
spec:
  allow:
    kubernetes_groups:
    - '{{internal.kubernetes_groups}}'
    kubernetes_labels:
      '*': '*'
    kubernetes_resources:
    - kind: deployment
      name: '*'
      namespace: default
      verbs:
      - '*'
    - kind: persistentvolume
      name: '*'
      verbs:
      - '*'
  deny: {}
version: v7
```

```yaml
kind: role
metadata:
  name: kube-access-v8
spec:
  allow:
    kubernetes_groups:
    - '{{internal.kubernetes_groups}}'
    kubernetes_labels:
      '*': '*'
    kubernetes_resources:
    - api_group: apps
      kind: deployments
      name: '*'
      namespace: default
      verbs:
      - '*'
    - api_group: ''
      kind: persistentvolumes
      name: '*'
      verbs:
      - '*'
  deny: {}
version: v8
```

Granting access to all items within a namespace. Note that in v8 there are two
entries, the first is for the namespace itself and the second is for all entries
within the namespace.

```yaml
kind: role
metadata:
  name: kube-access-v7-ns
spec:
  allow:
    kubernetes_groups:
    - '{{internal.kubernetes_groups}}'
    kubernetes_labels:
      '*': '*'
    kubernetes_resources:
    - kind: namespace
      name: default
      verbs:
      - '*'
  deny: {}
version: v7
```

```yaml
kind: role
metadata:
  name: kube-access-v8-ns
spec:
  allow:
    kubernetes_groups:
    - '{{internal.kubernetes_groups}}'
    kubernetes_labels:
      '*': '*'
    kubernetes_resources:
    - api_group: ''
      kind: namespaces
      name: default
      verbs:
      - '*'
    - api_group: '*'
      kind: '*'
      name: '*'
      namespace: default
      verbs:
      - '*'
  deny: {}
version: v8
```

For more information, refer to the [docs](docs/pages/enroll-resources/kubernetes-access/controls.mdx#kubernetes_resources).

### Breaking changes and deprecations

#### TLS cipher suites

TLS cipher suites with known security issues can no longer be manually
configured in the Teleport YAML configuration file. If you do not explicitly
configure any of the listed TLS cipher suites, you are not affected by this
change.

Teleport 18 removes support for:
* `tls-rsa-with-aes-128-cbc-sha`
* `tls-rsa-with-aes-256-cbc-sha`
* `tls-rsa-with-aes-128-cbc-sha256`
* `tls-rsa-with-aes-128-gcm-sha256`
* `tls-rsa-with-aes-256-gcm-sha384`
* `tls-ecdhe-ecdsa-with-aes-128-cbc-sha256`
* `tls-ecdhe-rsa-with-aes-128-cbc-sha256`

#### Terraform provider role defaults

The Terraform provider previously defaulted unset booleans to `false`, starting
with v18 it will leave the fields empty and let Teleport pick the same default
value as if you were applying the manifest with the web UI, `tctl create`, or
the Kubernetes Operator.

This might change the default options of role where not every option was
explicitly set. For example:

```hcl
resource "teleport_role" "one-option-set" {
  version = "v7"
  metadata = {
    name        = "one-option-set"
  }

  spec = {
    options = {
      max_session_ttl = "7m"
      # other boolean options were wrongly set to false by default
    }
  }
}
```

This change does not affect you if you were not setting role options,
or setting every role option in your Terraform code.

After updating the Terraform provider to v18, `terraform plan` will display the
role option differences, please review it and check that the default changes are
acceptable. If they are not, you must set the options to `false`.

Here's a plan example for the code above:

```
# teleport_role.one-option-set will be updated in-place
~ resource "teleport_role" "one-option-set" {
      id       = "one-option-set"
    ~ spec     = {
        ~ options = {
            - cert_format               = "standard" -> null
            - create_host_user          = false -> null
            ~ desktop_clipboard         = false -> true
            ~ desktop_directory_sharing = false -> true
            - port_forwarding           = false -> null
            ~ ssh_file_copy             = false -> true
              # (4 unchanged attributes hidden)
          }
      }
      # (3 unchanged attributes hidden)
  }
```

#### AWS endpoint URL mode removed

The `tsh aws` and `tsh proxy aws` commands no longer support being used as
custom service endpoints.  Instead, users should use them as `HTTPS_PROXY` proxy
servers.

For example, the following command will no longer work: `aws s3 ls
--endpoint-url https://localhost:LOCAL_PROXY_PORT`.  To achieve a similar result
with Teleport 18, run `HTTPS_PROXY=http://localhost:LOCAL_PROXY_PORT aws s3 ls`.

#### TOTP for per-session MFA

Starting with Teleport 18, `tsh` will no longer allow for using TOTP as a second
factor for per-session MFA. TOTP continues to be accepted as a second factor for
the initial login.

#### Linux kernel 3.2 required

On Linux, Teleport now requires Linux kernel version 3.2 or later.

### Other changes

#### PKCE support for OpenID Connect

Teleport 18 includes support for Proof Key for Code Exchange (PKCE) in OpenID
Connect flows. PKCE is a security enhancement that ensures that attackers who
can intercept the authorization code will not be able to exchange it for an
access token.

To enable PKCE, set `pkce_mode: enabled` in your OIDC connector. Future versions
of Teleport may enable PKCE by default.

#### Cache improvements

Teleport 18 ships with an improved cache implementation that stores resources
directly instead of storing their JSON-encoded representation. In addition to
performance gains, this new storage mechanism will also improve compatibility
between older agents and newer versions of resources.

#### Windows desktop discovery enhancements

Teleport's LDAP-based discovery mechanism for Windows desktops now supports:

* a configurable discovery interval
* custom RDP ports
* the ability to run multiple separate discovery configurations, allowing you to
  configure finely-grained discovery policies without running multiple agents

To update your configuration, move the `discovery` section to `discovery_configs`:

```diff
windows_desktop_service:
  enabled: yes
+  discovery_interval: 10m # optional, defaults to 5 minutes
-  discovery:
-    base_dn: '*'
-    label_attributes: [ department ]
+  discovery_configs:
+    - base_dn: '*'
+      label_attributes: [ department ]
+      rdp_port: 9989 # optional, defaults to 3389
```

#### Customizable keyboard layouts for remote desktop sessions

The web UI's account settings page now includes an option for
setting your desired keyboard layout for remote desktop sessions.

This keyboard layout will be respected by agents running Teleport 18
or later.

#### Faster user lookups on domain-joined Windows workstations

Teleport 18 is built with Go 1.24, which includes an optimized user lookup
implementation. As a result, the
[workarounds](https://goteleport.com/docs/faq/#tsh-is-very-slow-on-windows-what-to-do)
for avoiding slow lookups in tsh and Teleport Connect are no longer necessary.

#### Agent Managed Updates v2 enhancements

Managed Updates v2 can now track which version agents are running and use this
information to progress the rollout. Only Linux agents are supported, agent
reports for `teleport-kube-agent` will come in a future update. Reports are
generated every minute and only count agents connected and stable for at least
a minute.

You can now observe the agent managed update progress by using
`tctl autoupdate agents status` and `tctl autoupdate agents report`.

If the strategy is `halt-on-error`, the group will be marked as done and the
rollout will continue only after at least 90% of the agents are updated.

You can now manually trigger a group, mark it as done, or rollback an update
with `tctl`:

```shell
autoupdate agents start-update [group1, group2, ...]
autoupdate agents mark-done [group1, group2, ...]
autoupdate agents rollback [group1, group2, ...]
```

#### Legacy ALPN connection upgrade mode has been removed

Teleport v15.1 added WebSocket upgrade support for Teleport proxies behind layer
7 load balancers and reverse proxies. The legacy ALPN upgrade mode using `alpn`
or `alpn-ping` as upgrade types was left as a fallback until v17.

Teleport v18 removes the legacy upgrade mode entirely including the use of the
`TELEPORT_TLS_ROUTING_CONN_UPGRADE_MODE` environment variable.
