# Changelog

## 18.0.0 (xx/xx/xx)

### Breaking changes

#### TLS Cipher Suites

TLS cipher suites with known security issues can no longer be manually
configured in the Teleport YAML configuration file. If you do not explicitly
configure any of the listed TLS cipher suites, you are not affected by this
change.

Teleport 18 removes support for:
- `tls-rsa-with-aes-128-cbc-sha`
- `tls-rsa-with-aes-256-cbc-sha`
- `tls-rsa-with-aes-128-cbc-sha256`
- `tls-rsa-with-aes-128-gcm-sha256`
- `tls-rsa-with-aes-256-gcm-sha384`
- `tls-ecdhe-ecdsa-with-aes-128-cbc-sha256`
- `tls-ecdhe-rsa-with-aes-128-cbc-sha256`

#### Terraform provider role defaults

The Terraform provider previously defaulted unset booleans to `false`, starting
with v18 it will leave the fields empty and let Teleport pick the same default
value as if you were applying the manifest with the web UI, `tctl create`, or
the Kubernetes Operator.

This might change the default options of role where not every option was
explicitly set. For example:

```
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

The AWS endpoint URL mode (`--endpoint-url`) has been removed for `tsh proxy
aws` and `tsh aws`. Users using this mode should use the default HTTPS Proxy
mode from now on.

### Other changes

#### Configurable keyboard layouts for Windows desktop sessions

Teleport's Account Settings page now exposes an option to set your preferred
keyboard layout for Windows desktop sessions.

Note: in order for this setting to take affect, agent's running Teleport's
`windows_desktop_service` must be upgraded to v18.0.0 or later.

#### Windows desktop discovery enhancements

Teleport's LDAP-based discovery mechanism for Windows desktops now supports:

- a configurable discovery interval
- custom RDP ports
- the ability to run multiple separate discovery configurations, allowing you to
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

#### Legacy ALPN connection upgrade mode has been removed

Teleport v15.1 added WebSocket upgrade support for Teleport proxies behind
layer 7 load balancers and reverse proxies. The legacy ALPN upgrade mode using
`alpn` or `alpn-ping` as upgrade types was left as a fallback until v17.
Teleport v18 removes the legacy upgrade mode entirely including the use of
environment variable `TELEPORT_TLS_ROUTING_CONN_UPGRADE_MODE`.

## 16.0.0 (xx/xx/xx)

### Breaking changes

#### Opsgenie plugin annotations

Opsgenie plugin users, role annotations must now contain
`teleport.dev/notify-services` to receive notification on Opsgenie.
`teleport.dev/schedules` is now the label used to determine auto approval flow.
See [the Opsgenie plugin documentation](docs/pages/identity-governance/access-request-plugins/opsgenie.mdx)
for setup instructions.

#### Teleport Assist has been removed

Teleport Assist chat has been removed from Teleport 16. `auth_service.assist` and `proxy_service.assist`
options have been removed from the configuration. Teleport will not start if these options are present.

During the migration from v15 to v16, the options mentioned above should be removed from the configuration.

#### DynamoDB permission requirements have changed

Teleport clusters using the dynamodb backend must now have the `dynamodb:ConditionCheckItem`
permission. For a full list of all required permissions see the Teleport [Backend Reference](docs/pages/reference/backends.mdx#dynamodb).

#### Disabling multi-factor authentication_type

Support for disabling multi-factor authentication has been removed

#### Machine ID and OpenSSH client config changes

Users with custom `ssh_config` should modify their ProxyCommand to use the new,
more performant, `tbot ssh-proxy-command`. See the
[v16 upgrade guide](docs/pages/reference/machine-id/v16-upgrade-guide.mdx) for
more details.

#### Default keyboard shortcuts in Teleport Connect have been changed

On Windows and Linux, some of the default shortcuts conflicted with the default bash or nano shortcuts
(e.g. Ctrl + E, Ctrl + K).
On those platforms, the default shortcuts have been changed to a combination of Ctrl + Shift + *.
We also updated the shortcut to open a new terminal on macOS to Control + Shift + \`.
See [configuration](docs/pages/connect-your-client/teleport-connect.mdx#configuration)
for the current list of shortcuts.

## 15.0.0 (xx/xx/24)

### New features

#### FIPS now supported on ARM64

Teleport 15 now provides FIPS-compliant Linux builds on ARM64. Users will now
be able to run Teleport in FedRAMP/FIPS mode on ARM64.

#### Hardened AMIs now produced for ARM64

Teleport 15 now provides hardened AWS AMIs on ARM64.

#### Streaming session playback

Prior to Teleport 15, `tsh play` and the web UI would download the entire
session recording before starting playback. As a result, playback of large
recordings could be slow to start, and may fail to play at all in the browser.

In Teleport 15, session recordings are streamed from the Auth Service, allowing
playback to start before the entire session is downloaded and unpacked.

Additionally, `tsh play` now supports a `--speed` flag for adjusting the
playback speed.

#### Standalone Teleport Operator

Prior to Teleport 15, the Teleport Kubernetes Operator had to run as a sidecar
of the Teleport auth. It was not possible to use the operator in Teleport
Enterprise (Cloud) or against a Teleport cluster not deployed with the
`teleport-cluster` Helm chart.

In Teleport 15, the Teleport Operator can reconcile resources in any Teleport
cluster. Teleport Enterprise (Cloud) users can now use the operator to manage
their resources.

When deployed with the `teleport-cluster` chart, the operator now runs in a
separate pod. This ensures that Teleport's availability won't be impacted if
the operator becomes unready.

See [the Standalone Operator guide](docs/pages/admin-guides/infrastructure-as-code/teleport-operator/teleport-operator-standalone.mdx)
for installation instructions.

#### Teleport Operator now supports roles v6 and v7

Starting with Teleport 15, newly supported kinds will contain the resource version.
For example: `TeleportRoleV6` and `TeleportRoleV7` kinds will allow users to
create Teleport Roles v6 and v7.

Existing kinds will remain unchanged in Teleport 15, but will be renamed in
Teleport 16 for consistency.

To migrate an existing Custom Resource (CR) `TeleportRole` to
a `TeleportRoleV7`, you must:
- upgrade Teleport and the operator to v15
- annotate the exiting `TeleportRole` CR with `teleport.dev/keep: "true"`
- delete the `TeleportRole` CR (it won't delete the role in Teleport thanks to the annotation)
- create a new `TeleportRoleV7` CR with the same name

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

1. Remote Session Environment > RemoteFX for Windows Server 2008 R2 > Configure RemoteFX
1. Remote Session Environment > Enable RemoteFX encoding for RemoteFX clients designed for Windows Server 2008 R2 SP1
1. Remote Session Environment > Limit maximum color depth

Detailed instructions are available in the
[setup guide](docs/pages/enroll-resources/desktop-access/active-directory.mdx#enable-remotefx).
A reboot may be required for these changes to take effect.

#### `tsh ssh`

When running a command on multiple nodes with `tsh ssh`, each line of output
is now labeled with the hostname of the node it was written by. Users that
rely on parsing the output from multiple nodes should pass the `--log-dir` flag
to `tsh ssh`, which will create a directory where the separated output of each node
will be written.

#### `drop` host user creation mode

The `drop` host user creation mode has been removed in Teleport 15. It is replaced
by `insecure-drop`, which still creates temporary users but does not create a
home directory. Users who need home directory creation should either wrap `useradd`/`userdel`
or use PAM.

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

The legacy package repos will be shut off in mid 2025 after Teleport 14
has been out of support for many months.

#### Container images

Teleport 15 contains several breaking changes to improve the default security
and usability of Teleport-provided container images.

##### "Heavy" container images are discontinued

In order to increase default security in 15+, Teleport will no longer publish
[container images containing a shell and command line environment](https://github.com/gravitational/teleport/blob/branch/v14/build.assets/charts/Dockerfile)
to Elastic Container Registry's [gravitational/teleport](https://gallery.ecr.aws/gravitational/teleport)
image repo. Instead, all users should use the [distroless images](https://github.com/gravitational/teleport/blob/branch/v15/build.assets/charts/Dockerfile-distroless)
introduced in Teleport 12. These images can be found at:

* https://gallery.ecr.aws/gravitational/teleport-distroless
* https://gallery.ecr.aws/gravitational/teleport-ent-distroless

For users who need a shell in a Teleport container, a "debug" image is
available which contains BusyBox, including a shell and many CLI tools. Find
the debug images at:

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
only a single tag will be published with multi-platform support (e.g., `15.0.0`).
If you use Teleport Operator images with an architecture suffix, remove the
suffix and your client should automatically pull the platform-appropriate image.
Individual architectures may be pulled with `docker pull --platform <arch>`.

##### Quay.io registry

The quay.io container registry was deprecated and Teleport 12 is the last
version to publish images to quay.io. With Teleport 15's release, v12 is no
longer supported and no new container images will be published to quay.io.

For Teleport 8+, replacement container images can be found in [Teleport's public ECR registry](https://gallery.ecr.aws/gravitational).

Users who wish to continue to use unsupported container images prior to
Teleport 8 will need to download any quay.io images they depend on and mirror
them elsewhere before July 2024. Following brownouts in May and June, Teleport
will disable pulls from all Teleport quay.io repositories on Wednesday July 3,
2024.

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
last version to produce such legacy AMIs. With Teleport 15's release, only
the newer hardened Amazon Linux 2023 AMIs will be produced.

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

#### `teleport-cluster` Helm chart changes

Due to the new separate operator deployment, the operator is deployed by a subchart.
This causes the following breaking changes:
- `installCRDs` has been replaced by `operator.installCRDs`
- `teleportVersionOverride` does not set the operator version anymore, you must
  use `operator.teleportVersionOverride` to override the operator version.

Note: version overrides are dangerous and not recommended. Each chart version
is designed to run a specific Teleport and operator version. If you want to
deploy a specific Teleport version, use Helm's `--version X.Y.Z` instead.

The operator now joins using a Kubernetes ServiceAccount token. To validate the
token, the Teleport Auth Service must have access to the `TokenReview` API.
The chart configures this for you since v12, unless you disabled `rbac` creation.

##### Helm cluster chart FIPS mode changes

The teleport-cluster chart no longer uses versionOverride and extraArgs to set FIPS mode.

Instead, you should use the following values file configuration:

```
enterpriseImage: public.ecr.aws/gravitational/teleport-ent-fips-distroless
authentication:
  localAuth: false

```

#### Resource version is now mandatory and immutable in the Terraform provider

Starting with Teleport 15, each Terraform resource must have its version specified.
Before version 15, Terraform was picking the latest version available on resource creation.
This caused inconsistencies as new resources created with the same manifest as
old resources were not exhibiting the same behavior.

Resource version is now immutable. Changing a resource version will cause
Terraform to delete and re-create the resource. This ensures the correct
defaults are set.

Existing resources will continue to work as Terraform already imported their
version. However, new resources will require an explicit version.

### Other changes

#### Increased password length

The minimum password length has been increased to 12 characters.

#### Increased account lockout interval

The account lockout interval has been increased to 30 minutes.

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

You can find existing Access Lists documentation [here](docs/pages/identity-governance/access-lists/guide.mdx).

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

For more details and guidance on how to upgrade to v2, see [docs](https://github.com/gravitational/teleport/blob/branch/v14/docs/pages/reference/machine-id/v14-upgrade-guide.mdx).

## 13.0.1 (05/xx/23)

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
  * Added ability to specify discovery group for discovery services. [#24716](https://github.com/gravitational/teleport/pull/24716)
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

desktop access now authenticates to LDAP using X.509 client certificates.
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

## 6.2

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

#### New Features

Teleport 5.0 introduces two distinct features: Teleport application access and significant Kubernetes access improvements - multi-cluster support.

##### Teleport application access

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

##### Teleport Kubernetes access

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
* We've added a Waiting Room for customers using Access Workflows. [Docs](docs/pages/identity-governance/access-request-plugins/access-request-plugins.mdx)

##### Signed RPM and Releases

Starting with Teleport 5.0, we now provide an RPM repo for stable releases of Teleport. We've also started signing our RPMs to provide assurance that you're always using an official build of Teleport.

See https://rpm.releases.teleport.dev/ for more details.

#### Improvements

* Added `--format=json` playback option for `tsh play`. For example `tsh play --format=json ~/play/0c0b81ed-91a9-4a2a-8d7c-7495891a6ca0.tar | jq '.event` can be used to show all events within an a local archive. [#4578](https://github.com/gravitational/teleport/issues/4578)
* Added support for continuous backups and auto scaling for DynamoDB. [#4780](https://github.com/gravitational/teleport/issues/4780)
* Added a Linux ARM64/ARMv8 (64-bit) Release. [#3383](https://github.com/gravitational/teleport/issues/3383)
* Added `https_keypairs` field which replaces `https_key_file` and `https_cert_file`. This allows administrators to load multiple HTTPS certs for Teleport application access. Teleport 5.0 is backwards compatible with the old format, but we recommend updating your configuration to use `https_keypairs`.

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

We've added an [API Guide](docs/pages/admin-guides/api/api.mdx) to simply developing applications against Teleport.

#### Upgrade Notes

Please follow our [standard upgrade procedure](docs/pages/upgrading/upgrading.mdx).

* Optional: Consider updating `https_key_file` & `https_cert_file` to our new `https_keypairs:` format.
* Optional: Consider migrating Kubernetes access from `proxy_service` to `kubernetes_service` after the upgrade.

### 4.4.6

This release of teleport contains a security fix and a bug fix.

* Patch a SAML authentication bypass (see https://github.com/russellhaering/gosaml2/security/advisories/GHSA-xhqq-x44f-9fgg): [#5120](https://github.com/gravitational/teleport/pull/5120).

Any Enterprise SSO users using Okta, Active Directory, OneLogin or custom SAML connectors should upgrade their Auth Service to version 4.4.6 and restart Teleport. If you are unable to upgrade immediately, we suggest disabling SAML connectors for all clusters until the updates can be applied.

* Fix an issue where `tsh login` would fail with an `AccessDenied` error if
the user was perviously logged into a leaf cluster. [#5105](https://github.com/gravitational/teleport/pull/5105)

### 4.4.5

This release of Teleport contains a bug fix.

* Fixed an issue where a slow or unresponsive Teleport Auth Service instance could hang client connections in async recording mode. [#4696](https://github.com/gravitational/teleport/pull/4696)

### 4.4.4

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

Teleport 4.3 introduces four new plugins that work out of the box with [Approval Workflow](docs/pages/identity-governance/access-request-plugins/access-request-plugins.mdx). These plugins allow you to automatically support role escalation with commonly used third party services. The built-in plugins are listed below.

*   [PagerDuty](docs/pages/identity-governance/access-request-plugins/ssh-approval-pagerduty.mdx)
*   [Jira](docs/pages/identity-governance/access-request-plugins/ssh-approval-jira.mdx)
*   [Slack](docs/pages/identity-governance/access-request-plugins/ssh-approval-slack.mdx)
*   [Mattermost](docs/pages/identity-governance/access-request-plugins/ssh-approval-mattermost.mdx)

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
* Alpha: Workflows API lets admins escalate RBAC roles in response to user requests. [Read the docs](docs/pages/identity-governance/access-requests/access-requests.mdx). [#3006](https://github.com/gravitational/teleport/issues/3006)
* Beta: Teleport provides HA Support using Firestore and Google Cloud Storage using Google Cloud Platform. [Read the docs](docs/pages/admin-guides/deploy-a-cluster/deployments/gcp.mdx). [#2821](https://github.com/gravitational/teleport/pull/2821)
* Remote tctl execution is now possible. [Read the docs](./docs/pages/reference/cli/tctl.mdx). [#1525](https://github.com/gravitational/teleport/issues/1525) [#2991](https://github.com/gravitational/teleport/issues/2991)

### Fixes

* Fixed issue in socks4 when rendering remote address [#3110](https://github.com/gravitational/teleport/issues/3110)

### Documentation

* Adopting root/leaf terminology for trusted clusters. [Trusted cluster documentation](docs/pages/admin-guides/management/admin/trustedclusters.mdx).
* Documented Teleport FedRAMP & FIPS Support. [FedRAMP & FIPS documentation](docs/pages/admin-guides/access-controls/compliance-frameworks/fedramp.mdx).

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

## 3.1

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

## 3.0

This is a major Teleport release which introduces support for Kubernetes
clusters. In addition to this new feature this release includes several
usability and performance improvements listed below.

#### Kubernetes Support

* `tsh login` can retrieve and install certificates for both Kubernetes and SSH
  at the same time.
* Full audit log support for `kubectl` commands, including recording of the sessions
  if `kubectl exec` command was interactive.
* Unified (AKA "single pane of glass") RBAC for both SSH and Kubernetes permissions.

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

#### Improvements

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

#### Improvements

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

## 2.3

This release focus was to increase Teleport user experience in the following areas:

* Easier configuration via `tctl` resource commands.
* Improved documentation, with expanded 'examples' directory.
* Improved CLI interface.
* Web UI improvements.

#### Improvements

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

## 2.0

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

## 1.3

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
to Teleport Community Edition users.

### Bugfixes

* Guessing `advertise_ip` chooses IPv6 address space. #486

## 1.0

The first official release of Teleport!
