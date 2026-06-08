# Changelog

## 18.9.0-rc.2 (06/08/26)

### Device Bound Session Credentials for App Access

Application access session cookies are now compatible with Google's Device Bound
Session Credentials, adding a layer of protection against session hijacking and
cookie theft.

### High-DPI support for Windows desktop sessions

Remote desktop sessions now support high-DPI mode, improving the clarity and
quality of the display rendering on supported displays.

### Sub-CA

Teleport now supports operating as a sub-CA of an external root for the Windows
Desktop and Database Client CAs. Subsequent releases will extend support for
other CAs.

### Other fixes and improvements

* Outdated agents joining via the legacy Auth HTTP endpoint now receive an explicit "client too old" error instead of a confusing 404. [#67532](https://github.com/gravitational/teleport/pull/67532)
* Rename --from/--to to --from-utc/--to-utc on `recordings search` to match the `recordings ls` flag naming convention. [#67502](https://github.com/gravitational/teleport/pull/67502)
* Improved performance and reduced resource usage of the auth service for clusters with large numbers of registered applications with per-session MFA enabled. [#67471](https://github.com/gravitational/teleport/pull/67471)
* Prevented ssh users from being able to cancel other users' remote port forwards. [#67442](https://github.com/gravitational/teleport/pull/67442)
* Improved application server resolution times for large number of applications. [#62585](https://github.com/gravitational/teleport/pull/62585)

## 18.8.3 (06/03/26)

* Fixed minor formatting bug on `tsh request show` output. [#67447](https://github.com/gravitational/teleport/pull/67447)
* The embedded session helper functionality introduced in v18.8.0 to improve memory usage and latency of SSH sessions is now disabled by default due to incompatibility with some endpoint protection services. It can be enabled by setting the `TELEPORT_UNSTABLE_DISABLE_EMBEDDED_REEXEC` envvar to `no`. [#67430](https://github.com/gravitational/teleport/pull/67430)
* Updated Go to 1.25.11. [#67421](https://github.com/gravitational/teleport/pull/67421)
* Improved notification messaging for Slack and Discord access plugins. [#67415](https://github.com/gravitational/teleport/pull/67415)
* Added support for auto discovering VMs deployed in uniform Azure VM Scale Sets to terraform modules used in Auto Discovery. [#67323](https://github.com/gravitational/teleport/pull/67323)
* Added secret lookup support for `TeleportOIDCConnector.spec.google_service_account` to the Teleport Kubernetes Operator. [#67309](https://github.com/gravitational/teleport/pull/67309)
* Improved the latency of SSH agent forwarding used by multiple clients at once. [#67305](https://github.com/gravitational/teleport/pull/67305)
* Tightened signature handling in Device Trust challenge/response validation. [#67302](https://github.com/gravitational/teleport/pull/67302)
* Added `web_terminal_clipboard_mode` role option to restrict copying text from a web terminal SSH session. [#67276](https://github.com/gravitational/teleport/pull/67276)
* Improved performance and reduced resource usage of the auth service for clusters with large numbers of registered Kubernetes clusters with per-session MFA enabled. [#67203](https://github.com/gravitational/teleport/pull/67203)
* Fixed an issue where generated installer scripts could incorrectly escape special characters in some values. [#67191](https://github.com/gravitational/teleport/pull/67191)
* Fixed a bug in Teleport Connect where the last terminal input could be logged to `renderer.log` if the terminal closed on its own — for example, when a `tsh ssh` session is dropped by the remote side (idle timeout, network disconnection) after the user pasted content but before they pressed Enter. [#67172](https://github.com/gravitational/teleport/pull/67172)
* Fixed a Enhanced Session Recording bug in proxy recording mode that caused Teleport Nodes to stop emitting BPF events. [#67155](https://github.com/gravitational/teleport/pull/67155)
* Fixed the `teleport-kube-agent` updater not honouring the `podSecurityContext` value. [#67097](https://github.com/gravitational/teleport/pull/67097)
* Fixed device trust for remote users connecting to a trusted cluster. [#67031](https://github.com/gravitational/teleport/pull/67031)
* Improved performance and reduced resource usage of the auth service for clusters with large numbers of registered databases with per-session MFA enabled. [#67029](https://github.com/gravitational/teleport/pull/67029)
* NOCL: [v18] Bump github.com/containerd/containerd from 1.7.30 to 1.7.32 [#67007](https://github.com/gravitational/teleport/pull/67007)
* Reduced peak memory usage of SSH target resolution in Auth service instances. [#67005](https://github.com/gravitational/teleport/pull/67005)
* Introduced `tsh workload-identity issue-jwt` command for human issuance of JWT-SVIDs. [#66995](https://github.com/gravitational/teleport/pull/66995)
* Improved the reliability of clipboard sharing for remote desktop sessions in both Teleport Connect and browsers running Chrome 144+. [#66979](https://github.com/gravitational/teleport/pull/66979)
* Fixed a TLS certificate error that prevented users from connecting to Amazon Keyspaces databases through Teleport. [#66974](https://github.com/gravitational/teleport/pull/66974)
* Tightened default permission when creating AWS configuration files. [#66941](https://github.com/gravitational/teleport/pull/66941)
* Stopped traversing symlinks and allowing relative paths in moderated file transfers. [#66796](https://github.com/gravitational/teleport/pull/66796)
* Added `identity/key-agent` service to enable `tbot` to generate un-exfiltratable credentials. [#66701](https://github.com/gravitational/teleport/pull/66701)
* Reduced unnecessary S3 uploads for Athena audit log deployments that publish directly to SQS by applying the correct SQS message size limit when the client has `sqs:GetQueueAttributes` permission, instead of always using the 256 KB SNS limit. [#66532](https://github.com/gravitational/teleport/pull/66532)
* Combined passkeys and MFA devices into one list on the account settings page. [#66435](https://github.com/gravitational/teleport/pull/66435)
* Added support for allowing or denying AWS IAM join attempts using the account's Organizational Units in their current Organization. [#66276](https://github.com/gravitational/teleport/pull/66276)
* Fixed a fatal connection error that occurs in Windows Desktop sessions when attempting to create a file larger than 4GiB within a shared directory. [#65478](https://github.com/gravitational/teleport/pull/65478)

Enterprise:
* Fixed regresion where users added to an Okta group via SCIM were silently dropped when the Okta integration was configured in read-only mode with SCIM enabled.
* SCIM-synced access lists will now have a badge displayed next to them in the web UI.
* Fixed a bug that could cause panics in Teleport's SAML IdP during failure scenarios.

## 18.8.2 (05/21/26)

* Fixed `tsh aws`, `tsh gcp`, `tsh azure`, and `tsh proxy app` failing with certificate errors. [#66962](https://github.com/gravitational/teleport/pull/66962)
* Fixed a regression introduced in v18.7.6 affecting connectivity to resources via approved just-in-time resource access requests when the cluster is running agents older than v18.7.6. [#66933](https://github.com/gravitational/teleport/pull/66933)
* Teleport Connect now remembers recently used clusters after logout. [#66781](https://github.com/gravitational/teleport/pull/66781)
* Fixed an issue where Windows desktop LDAP discovery could conflict with dynamic registration causing desktops to be removed from the cluster. [#66743](https://github.com/gravitational/teleport/pull/66743)
* Windows desktop controls in Teleport Connect now reside in the status bar in order to allocate more screen real estate to the RDP session. [#66726](https://github.com/gravitational/teleport/pull/66726)

Enterprise:
* SCIM-synced access lists will now have a badge displayed next to them in the web UI.
* Fixed access monitoring graph data handling in the Web UI when the amount of results exceeds the display maximum - now hides earlier instead of later data.
* Restricted user traits preserved during a SAML logon to those created by the Okta or SCIM integrations.
* Improved reliability of Okta assignments processing.

## 18.8.1 (05/14/26)

**Warning:** This release contains a regression that affects connectivity to
resources via an approved just-in-time resource access request when the cluster
is running agents older than v18.8.0.

If you use resource access requests and unable to ensure all agents are upgraded
to v18.8.1 in tandem with auth and proxy, we recommend skipping this release and
upgrading to v18.8.2 once it's available instead.

* Improved the performance of certain predicate expressions used to select SSH servers. [#66769](https://github.com/gravitational/teleport/pull/66769)
* Fixes an issue preventing joins using the azure join method in regions where the trust chain has been updated with an additional intermediate. [#66764](https://github.com/gravitational/teleport/pull/66764)
* Fix Teleport Connect's VNet failing to start on Linux when an older `tsh` is present at `/usr/local/bin/tsh`. [#66757](https://github.com/gravitational/teleport/pull/66757)
* The MFA prompt now includes the name of a leaf cluster if the resource belongs to one. [#66741](https://github.com/gravitational/teleport/pull/66741)
* When attempting to access a web app protected by Device Trust from an untrusted device, browsers now see a simple HTML page instead of a plain text response. [#66717](https://github.com/gravitational/teleport/pull/66717)
* Improved the error message on login in tsh and Teleport Connect when `/webapi/ping` returns a non-200 response. [#66712](https://github.com/gravitational/teleport/pull/66712)
* The kubernetes join method now supports allow rules targeting specific service account names and namespaces and supports wildcards when the new fields are used. [#66700](https://github.com/gravitational/teleport/pull/66700)
* Raise the app access upstream response-header cap from 5 minutes to 1 hour so long-running HTTP requests complete. [#66687](https://github.com/gravitational/teleport/pull/66687)
* Fixed an issue preventing host sudoers entries from being written on newer Linux distributions (i.e. Ubuntu 25.10) using sudo-rs. [#66433](https://github.com/gravitational/teleport/pull/66433)

Enterprise:
* Internal performance optimizations to the SCIM PATCH flow when multiple parallel PATCH requests target the same SCIM groups.
* Fixed an issue with sessions failing to be summarized when using non-alternate buffer TUI applications.
* Commands in the session summary timeline now show detected MITRE attack IDs and suspicious flags.
* Fixed Web UI to no longer show audit review prompts or 0001-01-01 dates for static Access Lists.

## 18.8.0 (05/07/26)

**Warning:** This release contains a regression that affects connectivity to
resources via an approved just-in-time resource access request when the cluster
is running agents older than v18.8.0.

If you use resource access requests and unable to ensure all agents are upgraded
to v18.8.0 in tandem with auth and proxy, we recommend skipping this release and
upgrading to v18.8.2 once it's available instead.

### Performance improvements in the SSH service

Thanks to internal improvements ([#66220](https://github.com/gravitational/teleport/pull/66220)), the Teleport SSH service memory usage and latency when opening shells/running commands is significantly lower than previous versions.

The reduction in the latency compared to the previous version of Teleport, as measured on a `m7i.xlarge` EC2 instance, amounts to roughly 100 ms when opening shells or launching commands and about 150 ms when using SFTP, with an additional 40 ms improvement when establishing the very first port forward for a given SSH connection.

The improvement in memory usage trades off an additional 7MiB of baseline memory usage for a significant reduction in the per-session memory usage of about 23 MiB for each shell or command execution, with another 20 MiB of memory savings for each SSH connection using port forwarding, and about 45 MiB for SFTP sessions.

### VNet for Linux

Teleport VNet support extends to Linux workstations.

### Improvements to access list creation UX

Teleport provides guided in-product UX for creating common types of access lists centered around granting users permissions to resources and permissions to request access to resources.

### tsh MFA via browser

tsh delegates MFA checks (both on login and for per-session MFA) to the browser, enabling the use of browser based passkeys or password managers with tsh.

### Multi-domain support for Windows desktop access

Teleport supports RDP connections to Windows hosts where the Windows users belong to different Active Directory domains than the target hosts.

### Bound keypair joining for agents

Teleport's bound keypair join method extends to support arbitrary Teleport agents in addition to bots.

### Session summaries search

Identity Security provides users with CLI tooling for searching session summaries allowing users to find sessions based on natural language queries.

### Terraform support for AWS EKS discovery

Users will be able to set up AWS EKS discovery at the AWS account level using the Terraform module.

### Terraform support for access list workflows

Short and long term access list creation flows in the web UI now include Terraform support allowing users to define access with infrastructure-as-code.

### Teleport Connect installation and updates

Teleport Connect for Windows now supports both per-machine and per-user installations. (Note: VNet is not available in per-user mode.)

Per-machine installations can now receive automatic updates without prompting for administrator privileges. Those privileges are only required during the initial installation.

Starting with this release, Teleport Connect only supports automatic upgrades. Downgrades must now be performed manually. This change applies to all platforms.

#### Access requests privilege escalation UX for AWS

Teleport users are now able to see specific IAM roles available to them when requesting
elevated access to AWS CLI/console. Future releases will extend support for specific
principal selection to access requests for other resource types as well.

### Other fixes and improvements

* Added support for AWS RDS discovery in the `teleport/discovery/aws` Terraform module. [#66627](https://github.com/gravitational/teleport/pull/66627)
* Fixed identifier-first login form overflowing on mobile viewports. [#66620](https://github.com/gravitational/teleport/pull/66620)
* Improved the performance of VNet on macOS by eliminating unnecessary reconnects. [#66562](https://github.com/gravitational/teleport/pull/66562)
* Fixed `metadata.revision` not being excluded from the `teleport_vnet_config` Terraform schema. Users with existing state may need to run `terraform refresh` if `terraform show` fails with "unsupported attribute revision". [#66617](https://github.com/gravitational/teleport/pull/66617)
* Fixed resource-based access requests failing when node/ssh agents have not yet been updated to a version supporting Resource Constraints. [#66585](https://github.com/gravitational/teleport/pull/66585)
* Updated Go to 1.25.10. [#66569](https://github.com/gravitational/teleport/pull/66569)
* Fixed an issue with Azure discovery where blocked installation attempts prevent discovery from making progress. Install attempts will now time out after 5 minutes, but this can be adjusted by setting an environment variable on the Teleport Discovery Service, e.g., `TELEPORT_UNSTABLE_AZURE_RUN_COMMAND_TIMEOUT=3m45s`. [#66558](https://github.com/gravitational/teleport/pull/66558)
* Increased verbosity of Teleport Discovery Service logs for VM discovery. [#66553](https://github.com/gravitational/teleport/pull/66553)
* Improved Teleport Connect startup reliability on Windows. [#66509](https://github.com/gravitational/teleport/pull/66509)
* Hardened event handler so it recovers in case of malformed session ID or corrupted data directory. [#66473](https://github.com/gravitational/teleport/pull/66473)
* Added Azure Discovery With Terraform integration guided flow in the web UI. [#66493](https://github.com/gravitational/teleport/pull/66493)
* Fixed app access dropping URL fragments through the auth redirect flow. [#66460](https://github.com/gravitational/teleport/pull/66460)
* Added user traits filtering in the web UI. [#66457](https://github.com/gravitational/teleport/pull/66457)
* Fixed an issue that could cause LDAP discovery to fail when a single desktop service discovers large numbers of hosts. [#66397](https://github.com/gravitational/teleport/pull/66397)
* Added Azure VM support for `tctl discovery nodes` command for troubleshooting auto-discovery enrollment issues on Azure. [#66395](https://github.com/gravitational/teleport/pull/66395)
* Fixed a rare input swallowing bug when resuming a moderated Node session. [#66370](https://github.com/gravitational/teleport/pull/66370)
* Role with unknown fields is now rejected at create/edit time instead of being silently dropped. Applies to `tctl` and the web UI YAML editor. [#66360](https://github.com/gravitational/teleport/pull/66360)
* Fix issue where generic error messages were being shown instead of specific ones for failed SSO logins. [#66348](https://github.com/gravitational/teleport/pull/66348)
* Fixed MCP clients' timeout and broken connections when the MCP server tries to resume the previous session. [#66343](https://github.com/gravitational/teleport/pull/66343)
* Add `tsh beams` commands for the Beams public beta. [#66316](https://github.com/gravitational/teleport/pull/66316)
* Fixed possible unavailability of Proxy service instances as a result of some API errors. [#66312](https://github.com/gravitational/teleport/pull/66312)
* Fixed an issue where WebAssembly not being available would crash the web UI. [#66216](https://github.com/gravitational/teleport/pull/66216)
* Added audit events for Azure VM auto-discovery installations, with install script output and exit status. [#66067](https://github.com/gravitational/teleport/pull/66067)
* Fixed an issue where EC2 auto-discovery could install Teleport on an instance but silently drop the failure when the agent could not join the cluster. A new `ec2-join-failure` user task is now raised with the actual join error message surfaced from the agent's readyz socket. [#66023](https://github.com/gravitational/teleport/pull/66023)
* Added support for `WorkloadIdentity` when using the `--apply-on-startup` and `--bootstrap` flags. [#65581](https://github.com/gravitational/teleport/pull/65581)
* Fixed a bug where tbot's `/readyz` endpoint would report "unhealthy" even after identity renewal succeeds on-retry. [#65258](https://github.com/gravitational/teleport/pull/65258)
* Added support for both per-machine and per-user installations in Teleport Connect on Windows (Note: VNet is unavailable in per-user mode). [#65173](https://github.com/gravitational/teleport/pull/65173)
* Enabled silent automatic updates for Teleport Connect per-machine installations on Windows; elevated privileges are now only required during the initial setup. [#65173](https://github.com/gravitational/teleport/pull/65173)
* Deprecated the `TELEPORT_CDN_BASE_URL` and `TELEPORT_TOOLS_VERSION` environment variables for configuring Teleport Connect Windows updates. These must now be managed via system policy registry keys under `HKEY_LOCAL_MACHINE` or `HKEY_CURRENT_USER\SOFTWARE\Policies\Teleport\TeleportConnect`. The environment variables are still read for compatibility, but per-machine updates may require UAC prompts until configuration is migrated to registry policy keys. [#65173](https://github.com/gravitational/teleport/pull/65173)
* Automatic updates in Teleport Connect no longer allow app version downgrades (applies to all platforms). [#65173](https://github.com/gravitational/teleport/pull/65173)
* Added support for reverse tunnel agent stale connection timeout detection and recovery. [#62531](https://github.com/gravitational/teleport/pull/62531)

Enterprise:
* Reject AWS Identity Center System Credentials on Teleport Cloud.
* Validate AWS Identity Center install credentials with AWS API calls.
* Added support for Terraform configuration generation in the Access List creation wizard in the web UI, allowing users to deploy their Access List via Terraform.
* Fix a potential deadlock in the CockroachDB backend.
* Handle mapping of groups for Entra ID SAML logins when user is member of 150+ groups.
* Enterprise licenses with a devices limit for device trust can now enroll unlimited devices.

## 18.7.6 (04/29/26)

**Warning:** This release includes a regression that affects the use of JIT Access Requests when agents are still running versions older than v18.7.6
The following workarounds are available:
- Upgrade proxy and auth servers to 18.8.0 — this release includes a regression fix that allows JIT Access Requests to work correctly even when some agents are still running older versions.
- Upgrade all agents to 18.7.6

### Security fixes

This release includes various security-related improvements and bug fixes.
We recommend that users on versions prior to v18.7.4 upgrade their Auth and Database Services to this latest release.
For Teleport Cloud customers, your control plane has already been upgraded to a patched release.

#### [High] Authorization bypass in encrypted session recordings

Teleport did not ensure sufficient authorization in some of the encrypted session recordings APIs.
This could allow an attacker to upload recordings to the cluster.
For self-hosted users that do not use encrypted session recordings, the following debug log messages
on auth server would indicate vulnerable APIs being called:
- “creating encrypted session upload”
- “uploading encrypted session part”
- “completing encrypted session upload”

This issue specifically affects Teleport v18. We recommend that all users upgrade their
Auth Services to this release to ensure continued security and stability.

#### [High] Cross-node session recording access

When checking system service access to session recordings and audit logs, Teleport did not
perform sufficient authorization. This could allow a compromised Teleport SSH node service to
access audit events and session recordings from other nodes in the cluster.
We recommend that all users upgrade their Auth Services to this release to ensure continued security and stability.

#### [Medium] SSRF via AWS database access endpoint

Teleport did not sufficiently validate the connection endpoint for AWS database access
(DynamoDB, OpenSearch, Keyspaces). This could allow a malicious actor with access to Teleport
configuration to steal database access credentials by crafting a connection endpoint pointing to
their domain.
All users that use Teleport to access AWS-hosted databases (DynamoDB, OpenSearch, Keyspaces)
are advised to upgrade their Auth and Database Services to this release to ensure continued security
and stability.

### Other fixes and improvements

* Fixed an issue that prevents GCP Server discovery to try to enroll all the VMs that are found when one of them returns an error. [#66240](https://github.com/gravitational/teleport/pull/66240)
* Added scoped roles support to the Terraform provider. [#66225](https://github.com/gravitational/teleport/pull/66225)
* Added scoped role assignment support to the Terraform provider. [#66225](https://github.com/gravitational/teleport/pull/66225)
* Fixed an issue where `tctl edit plugin/jamf` could break other plugins when providing non-zero duration value. [#66191](https://github.com/gravitational/teleport/pull/66191)
* Introduces `skip_initial_connection` option to the `teleportmwi` provider to allow lazy initialization of the provider. [#66139](https://github.com/gravitational/teleport/pull/66139)
* Initialize keystore sign and decrypt metrics at startup and register missing decrypt metric collectors. [#66110](https://github.com/gravitational/teleport/pull/66110)
* Added current and previous resources discovered summary per service to Discovery Config Status. [#66097](https://github.com/gravitational/teleport/pull/66097)
* Fixed a bug where generated JWT tokens were leaked into audit event. [#66095](https://github.com/gravitational/teleport/pull/66095)
* Updated internal database dependencies to resolve multiple security vulnerabilities (CVE-2026-4427, CVE-2026-32286, and others). [#66083](https://github.com/gravitational/teleport/pull/66083)
* Fixed a possible panic during TTY session processing/playback/summarization from crashing Teleport. [#66080](https://github.com/gravitational/teleport/pull/66080)
* Fixed an issue where the endpoint used by `tsh scan keys` could leak resources on a server error; this affected only clusters with Access Graph enabled. [#66076](https://github.com/gravitational/teleport/pull/66076)
* Added `teleport_app_active_sessions` Prometheus gauge with `app` label for app access agent autoscaling. [#66050](https://github.com/gravitational/teleport/pull/66050)
* Fixed joining for agents and proxies connecting directly to an Auth service when they specify a CA pin and any lock in the cluster is in force. [#66044](https://github.com/gravitational/teleport/pull/66044)
* Added scoped role to the k8s operator. [#66034](https://github.com/gravitational/teleport/pull/66034)
* Added scoped role assignments to the k8s operator. [#66034](https://github.com/gravitational/teleport/pull/66034)
* Fixed Access List-granted roles being absent from the web session created after a local user password reset or invite acceptance, requiring a logout/login cycle to restore access. [#66011](https://github.com/gravitational/teleport/pull/66011)
* Added support for Azure join tokens based on Azure tenant ID. [#65989](https://github.com/gravitational/teleport/pull/65989)
* Fixed a "No such process" error that could happen on the very first launch of VNet on macOS. [#65967](https://github.com/gravitational/teleport/pull/65967)
* Improved readability of the search results in Teleport Connect. [#65928](https://github.com/gravitational/teleport/pull/65928)
* Fixed a Teleport Connect issue on Windows where startup could fail when `HTTPS_PROXY` is set. [#65924](https://github.com/gravitational/teleport/pull/65924)
* Added `user.metadata.name` variable to RBAC role templates and expressions. [#65923](https://github.com/gravitational/teleport/pull/65923)
* Fix VNet SSH per-session MFA checks to use the requested SSH login instead of the profile default login. [#65909](https://github.com/gravitational/teleport/pull/65909)
* Initialize backend read and requests metrics to zero at startup. [#65898](https://github.com/gravitational/teleport/pull/65898)
* Fixed Teleport not taking over an existing unmanaged host user when configured to. [#65838](https://github.com/gravitational/teleport/pull/65838)
* Fixes race condition in dynamoDB backend which can lead to missed events, resulting in a inconsistent cache state. [#65821](https://github.com/gravitational/teleport/pull/65821)
* Added `ui_config` resource support to the Terraform provider. [#65800](https://github.com/gravitational/teleport/pull/65800)
* Set default name for `UIConfig` resource as `ui-config`. [#65800](https://github.com/gravitational/teleport/pull/65800)
* Fixed an issue in Teleport Connect on macOS where selecting "Open Teleport Connect" from the menu bar would not reliably open the app. [#65774](https://github.com/gravitational/teleport/pull/65774)
* The github join method now supports the enterprise/enterprise_id claims. [#65700](https://github.com/gravitational/teleport/pull/65700)
* Teleport Connect now displays user roles in an expandable list. [#65654](https://github.com/gravitational/teleport/pull/65654)
* Standard Teleport agents can now join using the `bound_keypair` join method. [#65625](https://github.com/gravitational/teleport/pull/65625)
* Add x11 forwarding, SSH File Copying, Agent Forwarding, SSH Port Forwarding, Create Host User, Max Sessions, and host sudoers to scoped ssh role options. [#65601](https://github.com/gravitational/teleport/pull/65601)
* Added `tctl discovery nodes` command for troubleshooting AWS EC2 auto-discovery enrollment issues. [#65598](https://github.com/gravitational/teleport/pull/65598)
* Update Go to v1.25.9. [#65586](https://github.com/gravitational/teleport/pull/65586)
* Fix access graph AWS discovery to not deadlock when Identity Activity Center is disabled. [#65574](https://github.com/gravitational/teleport/pull/65574)
* Clear certs from local ssh agent when switching between unscoped user to scoped user. [#65568](https://github.com/gravitational/teleport/pull/65568)
* Added `lock` resource support to the Kubernetes operator. [#65543](https://github.com/gravitational/teleport/pull/65543)
* Added support for `*` and `$` globbing to the GitHub Actions token rules. [#65539](https://github.com/gravitational/teleport/pull/65539)
* The `tbot keypair create` command will now create the specified directory if necessary. [#65528](https://github.com/gravitational/teleport/pull/65528)
* Fixed an issue in Teleport Connect where the "Reopen" button in the "Reopen previous session" modal would not automatically receive focus. [#65513](https://github.com/gravitational/teleport/pull/65513)
* Fixed a bug where Teleport Connect displayed an error about an expired certificate instead of showing the login modal. [#65512](https://github.com/gravitational/teleport/pull/65512)
* Added visible `teleport.dev/` labels for Azure and GCP auto-discovered VMs, making subscription ID, VM ID, region, resource group, VM name, and zone available in the web UI, CLI output, and RBAC rules. [#65462](https://github.com/gravitational/teleport/pull/65462)
* Fixed panic in tctl get scoped_token when non-token join method scoped tokens were present. [#65461](https://github.com/gravitational/teleport/pull/65461)
* Fix "tctl edit" bugs when editing multiple resources, or resources with sub_kinds (for example, CAs). [#65341](https://github.com/gravitational/teleport/pull/65341)
* Removed expired Baltimore CyberTrust Root CA used for Azure databases. [#65329](https://github.com/gravitational/teleport/pull/65329)
* Reimplemented how Teleport Connect handles deep links for Device Trust auth and launching VNet from the Web UI. [#65316](https://github.com/gravitational/teleport/pull/65316)
* Extended access monitoring predicate language with `contains(set, item)` expression. [#65294](https://github.com/gravitational/teleport/pull/65294)
* Fixed an issue where viewing a session recording that did not exist/was not uploaded yet would show an empty player instead of an error message. [#65269](https://github.com/gravitational/teleport/pull/65269)
* Auth connector names are now limited to 768 characters. [#65242](https://github.com/gravitational/teleport/pull/65242)
* Fix a goroutine leak in the Teleport Connect MFA prompt when both SSO MFA and Webauthn are available second factors. [#65229](https://github.com/gravitational/teleport/pull/65229)
* Add SAML IdP Service Provider support to Terraform provider. [#65199](https://github.com/gravitational/teleport/pull/65199)
* Fixed minor bug in Web UI and Connect where static and dynamic labels with the same key are duplicated. [#65198](https://github.com/gravitational/teleport/pull/65198)
* Fix URL components for SAML auth connector ACS URL in tests, error message. [#65197](https://github.com/gravitational/teleport/pull/65197)
* Add `lock` support to the Terraform Provider. [#65134](https://github.com/gravitational/teleport/pull/65134)
* Fix a bug in the `teleport-cluster` chart causing some `auth.*` values to not be used when rendering hooks or config manifests. [#65131](https://github.com/gravitational/teleport/pull/65131)
* Fix Cross-access-list member injection in raw gRPC UpsertAccessListWithMembers call due to missing member.Spec.AccessList field validation. [#65123](https://github.com/gravitational/teleport/pull/65123)
* Fix an issue that allowed bypassing Resource Access Requests' AllowedResourceIDs when creating app sessions. [#65116](https://github.com/gravitational/teleport/pull/65116)
* Fix an issue that allowed IP Pinning protections to be bypassed via direct dial to a Teleport Node. [#65094](https://github.com/gravitational/teleport/pull/65094)
* Fix an issue that allowed IP Pinning protections to be bypassed via the WebUI. Also fix an issue with sporadic WebUI connection errors when the Proxy sees an unexpected client IP even though IP Pinning is not enforced. [#65090](https://github.com/gravitational/teleport/pull/65090)
* Fix an issue where `tctl get tokens` would prompt for admin MFA three times rather than once. [#65084](https://github.com/gravitational/teleport/pull/65084)
* Display an alert when SAML connector signing certificates are within 90 days of expiry. [#65067](https://github.com/gravitational/teleport/pull/65067)
* Add scopes status to webapi ping endpoint to determine whether the proxy and auth server has scopes enabled or not. [#65062](https://github.com/gravitational/teleport/pull/65062)
* Azure VM discovery configuration now supports specifying a wildcard ("*") subscription to discover all VMs in all subscriptions where the Discovery service has Microsoft.Resources/subscriptions/read permission. [#65045](https://github.com/gravitational/teleport/pull/65045)
* Added support for multiple trusted CA certificates to Windows Desktop Service's LDAP configuration. [#65040](https://github.com/gravitational/teleport/pull/65040)
* Fix mouse move when using fixed screen size Windows desktop. [#65025](https://github.com/gravitational/teleport/pull/65025)
* Fixed intermittent issues with VNet on Windows with NRPT rules being wiped after Group Policy refresh. [#65017](https://github.com/gravitational/teleport/pull/65017)
* Fixed AWS IAM Roles Anywhere Web/Console access for Gov Cloud and China partitioned AWS accounts. [#65016](https://github.com/gravitational/teleport/pull/65016)
* Device Trust is now accessible under Zero Trust Access in the web UI. [#65005](https://github.com/gravitational/teleport/pull/65005)
* Fixed an issue with desktop directory sharing in Teleport Connect that caused file modification times not to be displayed. [#64921](https://github.com/gravitational/teleport/pull/64921)
* Added support for configuring agentless OpenSSH servers using `tbot`. [#64899](https://github.com/gravitational/teleport/pull/64899)
* Fixed an issue preventing Teleport Connect from launching when the OS username contains non-ASCII characters. [#64885](https://github.com/gravitational/teleport/pull/64885)
* Fixed access request comments and review reasons not preserving newlines when rendered. [#64866](https://github.com/gravitational/teleport/pull/64866)
* Add documentation for the `tsh aws-profile` command. [#64777](https://github.com/gravitational/teleport/pull/64777)
* API rate limiting for authenticated per-session MFA requests now follows the regular API rate limits, making the limit unlikely to be hit during parallel SSH operations. [#64775](https://github.com/gravitational/teleport/pull/64775)
* Print a message indicating that `tctl recordings download <session_id>` completed successfully. [#64721](https://github.com/gravitational/teleport/pull/64721)
* Adds the ability to not render hooks in the `teleport-kube-agent` chart. [#64706](https://github.com/gravitational/teleport/pull/64706)
* Fixes version number for charts `teleport-kube-agent-updater` and `teleport-spiffe-daemon-set`. [#64686](https://github.com/gravitational/teleport/pull/64686)
* Vnet_config can now be managed via the Teleport Terraform Provider. [#64682](https://github.com/gravitational/teleport/pull/64682)
* Added scoped tokens to the k8s operator. [#64639](https://github.com/gravitational/teleport/pull/64639)
* Fixed a bug affecting nodes on v18.3.0+ rejoining with new system roles to clusters with Auth services on v18.2.10-. [#64638](https://github.com/gravitational/teleport/pull/64638)
* Updated github.com/docker/cli to v29.2.0+incompatible (addresses CVE-2025-15558). [#64606](https://github.com/gravitational/teleport/pull/64606)
* Fix map generation for teleport resources to k8s. [#64597](https://github.com/gravitational/teleport/pull/64597)
* Add scope aware profiles after tsh login. [#64592](https://github.com/gravitational/teleport/pull/64592)
* Added a new tsh aws-profile command that detects your AWS Identity Center integration (if configured) and writes corresponding AWS profiles into your local AWS config file for later use. [#64590](https://github.com/gravitational/teleport/pull/64590)
* Add `tctl acl` commands for managing access list reviews. [#64587](https://github.com/gravitational/teleport/pull/64587)
* Validate Kubernetes impersonation user and group header values to prevent CRLF or null-byte header injection via crafted role definitions. [#64586](https://github.com/gravitational/teleport/pull/64586)
* Fixed Azure and GCP server auto-discovery installation when the target VM had a system-wide HTTP proxy configured. [#64552](https://github.com/gravitational/teleport/pull/64552)
* Teleport Connect now displays the Message of the Day (MOTD) before login. [#64549](https://github.com/gravitational/teleport/pull/64549)
* Fixed bug that causes Windows desktop connection errors on EC2 joined nodes. [#64545](https://github.com/gravitational/teleport/pull/64545)
* Fixed `tsh login --request-id` to display up to date profile information including the assumed access request and roles. [#64536](https://github.com/gravitational/teleport/pull/64536)
* Update Go to v1.25.8. [#64434](https://github.com/gravitational/teleport/pull/64434)
* Added terraform provider support for teleport_scoped_token. [#64392](https://github.com/gravitational/teleport/pull/64392)
* Add SAML IdP Service Provider support to k8s operator. [#64380](https://github.com/gravitational/teleport/pull/64380)
* Fixed a bug causing spurious failures to upload encrypted session recordings when 4MB or larger. [#64361](https://github.com/gravitational/teleport/pull/64361)
* Fix agent forwarding in proxy recording mode. [#64349](https://github.com/gravitational/teleport/pull/64349)
* Fixed failures to record extra large session events in synchronous recording modes. [#64343](https://github.com/gravitational/teleport/pull/64343)
* Fixed a rare race condition causing initial node heartbeats to be missing an address. [#64332](https://github.com/gravitational/teleport/pull/64332)
* Fixed a VNet regression where web app access could fail when access was assumed in the Web UI only while VNet was already running. [#64317](https://github.com/gravitational/teleport/pull/64317)
* Add support for Resource-Scoped Constraints to Access Requests, allowing users to narrow the scope of their requested access by selecting individual principals on resources while creating an Access Request via the Teleport Web UI. Initial support includes Role ARNs for AWS Console app resources. [#63207](https://github.com/gravitational/teleport/pull/63207)
* Azure VM discovery will wait for installation and collect potential errors. The results will update discovery config statistics. VM installation errors will be captured as user tasks. [#62731](https://github.com/gravitational/teleport/pull/62731)

Enterprise:
* SAML IdP: Custom attribute mappings now overwrite default attributes with the same Name instead of duplicating them. Previously, mapping a default attribute like eduPersonAffiliation.
* Fixes the issue where users with ongoing Access Requests to Okta resources would synced back as permanent Access List members for the requested resources during the periodic Okta import.
* Fixed AWSIC Group membership updates so they are no longer abandoned when AWS is presented with an obsolete User ID.
* Fix Okta assignment reconciliation failing for applications with large user lists where the API response time exceeded the 30s HTTP client timeout by increase the Okta http connection Timeout to 5 min.
* Fixed an issue where Teleport could return a stale SCIM user group attribute when a user no longer belonged to any SCIM groups. Previously, in this case, the SCIM /Users endpoint could return outdated data from an earlier state when the user still had at least one group assigned.
* Add support for workload_cluster resource for Teleport Cloud users to provision new Teleport Cloud clusters.
* Improve SCIM performance by using cache for /Users and /Groups listing calls.
* Fixed SCIM group and user listing pagination to return the correct totalResults count across all pages. Previously, totalResults could undercount matching resources when paginating beyond the first page,.
* Fixed an issue where characters such as `.` were not permitted in an instance profile ARN when setting up session summaries inference.
* Access list owners are now automatically suggested as reviewers when the list grants members the ability to request access to resources and owners the ability to review those requests.
* Added handling for AWS Identity Center users deleted outside of Teleport's control.
* Device Trust is now accessible under Zero Trust Access in the web UI.
* Fixed group member provisioning bug in Identity Center plugin caused by an unsupported PATCH operation.
* Fix an issue where Okta pending assignments could overlap with Access List synchronization, causing users to be unintentionally re-added to access lists in case of strict Okta API rate limits.
* Fixed AWS Account rename handling in Identity Center plugin when an old account name is restored.
* Fix the issue where clicking **Provision User** after enrolling Okta SCIM integration could result in "missing Okta user ID" error.
* Added a wizard for creating access lists that includes defining which resources members can access.

## 18.7.5 (04/21/26)

* Prevented a possible panic during TTY session processing/playback/summarization from crashing Teleport.
* Fixed an upload failure for encrypted recordings near the 4MB limit caused by metadata exceeding the maximum message size.
* Fixed joining for agents and Proxy Server instances connecting directly to an Auth Service when they specify a CA pin and any lock in the cluster is in force.
* Fixed a bug where generated JWT tokens were leaked into audit events.
* Fixed a bug that caused Azure Server Discovery to stop prematurely if an error was encountered on a single VM.
* Fixed an issue with Directory Sharing in Teleport Connect that caused file modification times not to be displayed.

## 18.7.4 (04/03/26)

This is a private security release. Changelog will be publicly announced in a later version.

## 18.7.3 (03/10/26)

This is a private security release. Changelog will be publicly announced in a later version.

## 18.7.2 (03/05/26)

* Added `TeleportAccessMonitoringRuleV1` support to the Teleport Kubernetes operator. [#64368](https://github.com/gravitational/teleport/pull/64368)
* Added update scoped token support to tctl and update upsert scoped token rpc to not require status. [#64345](https://github.com/gravitational/teleport/pull/64345)
* Improved performance and reduced resource usage of the database proxy for clusters with large numbers of registered databases. [#64311](https://github.com/gravitational/teleport/pull/64311)
* Added more helpful messages to `ssm.run` events when there's a failure in discovering EC2 instances. [#64273](https://github.com/gravitational/teleport/pull/64273)
* Fixed a bug that could cause desktop connection errors during proxy upgrades for some cluster configurations. [#64258](https://github.com/gravitational/teleport/pull/64258)
* Fixed an issue where the UI would display a white screen and no error when an error occurred. [#64246](https://github.com/gravitational/teleport/pull/64246)
* Improve the layout of the web UI's message of the day. [#64213](https://github.com/gravitational/teleport/pull/64213)
* Fixed an issue where VNet on Windows could fail to start after an update with the error: `The specified service does not exist as an installed service.`. [#64206](https://github.com/gravitational/teleport/pull/64206)
* Fixed a bug where audit events could be created forever for an expired access request. [#64180](https://github.com/gravitational/teleport/pull/64180)
* Add scoped tokens to tctl resource commands. [#64040](https://github.com/gravitational/teleport/pull/64040)
* Fixed correct reporting of server discovery enrollment failures when the Proxy is not accessible from the target server. [#64007](https://github.com/gravitational/teleport/pull/64007)
* Fixed an issue that caused Discovery Service to stop working for Discovery Configs, also affecting AWS OIDC resource enrollments created from the UI. [#63970](https://github.com/gravitational/teleport/pull/63970)
* Added support for session summarizer resources to the Kubernetes operator. [#63884](https://github.com/gravitational/teleport/pull/63884)

Enterprise:
* Fixed an error log and a memory leak when manually deleting an okta_assignment resource.
* Fixed a potential panic in Auth service when getting a non-existing plugin without list permissions.
* Prevented membership modifications for Access Lists synchronized from Entra ID.

## 18.7.1 (02/24/26)

* Fixed web app access in leaf clusters when VNet is enabled. [#63993](https://github.com/gravitational/teleport/pull/63993)
* Fixed an issue where desktop session recordings would show a white screen instead of the recording player, and fixed an issue where if a session's metadata failed to load and the session had a summary it didn't display the summary. [#63982](https://github.com/gravitational/teleport/pull/63982)
* Fixed db session page refresh redirecting to empty page. [#63938](https://github.com/gravitational/teleport/pull/63938)
* Improved the performance of `tsh` and `tctl` when the profile directory is on a remote filesystem (NFS, SMB, etc.). [#63937](https://github.com/gravitational/teleport/pull/63937)
* Added platform information to ssm.run events when auto discovering EC2 instances. [#63925](https://github.com/gravitational/teleport/pull/63925)
* Added server side secret obfuscating for GetScopedTokens rpc and added UpsertScopedToken rpc. [#63902](https://github.com/gravitational/teleport/pull/63902)

Enterprise:
* Clarified MS Teams enrollment configuration values.

## 18.7.0 (02/13/26)

#### Session timeline view for Identity Security

Session player for Identity Security users received an enhanced timeline view with
per-command session breakdown.

#### Organization-level auto-discovery for AWS EC2 instances

AWS auto-discovery supports EC2 instance enrollment from all or a subset of accounts
of an AWS organization without having to configure per-account discovery.

Organization-level discovery for other resources within AWS (RDS, EKS) as well as other
for cloud providers will follow in future releases.

#### Terraform-native flow for configuration of AWS EC2 auto-discovery

Teleport provides in-product UX for configuring EC2 auto-discovery in a single AWS
account using terraform module.

#### Static labels for auto-discovered Windows desktops

Teleport can now be configured to apply a set of static labels to Windows
desktops that it discovers via LDAP. This is an alternative to setting labels
based on the value of LDAP attributes.

#### Entra ID integration status page

Teleport users are now able to see status of the configured Entra ID integration in the
web UI.

#### Inventory UI

Teleport's web UI now includes a new page showing the complete inventory of all instances
and bots connected to the cluster.

#### Managed Updates UI

Teleport's web UI now includes new functionality for working with managed updates.
The UI offers the ability to view and manage the updater configuration as well
as monitor the progress of update rollouts.

#### Split Windows CA

Teleport now introduces a new Windows CA responsible for issuing user certificates for
Windows Desktop access. Currently the User CA issues those certificates, as they are end-user certs.
Splitting the CAs improves Teleport's security posture by introducing a more specialized CA
and allows both CAs to be rotated independently.

#### Other fixes and improvements

* Fixed `tsh kubectl` failing when kubectl flags appear before positional arguments (e.g., `tsh kubectl -n default get pod`). [#63807](https://github.com/gravitational/teleport/pull/63807)
* The tsh status command can now be executed in client-only mode with --client. This skips all server-side operations. [#63786](https://github.com/gravitational/teleport/pull/63786)
* Improved tracing support via `tsh --trace kubectl`. [#63762](https://github.com/gravitational/teleport/pull/63762)
* Added `tctl recordings download` command to download session recordings to local files without requiring direct access to the storage backend. [#63726](https://github.com/gravitational/teleport/pull/63726)
* MWI: Add new `tbot start no-op` helper that starts no services. [#63666](https://github.com/gravitational/teleport/pull/63666)
* Improved performance and user experience of `teleport backend clone`. [#63635](https://github.com/gravitational/teleport/pull/63635)
* Fixed out of sequent audit logs rendering in ui for same timestamp logs. [#63613](https://github.com/gravitational/teleport/pull/63613)
* Added the Windows CA, used to issue Windows Desktop Access user certificates. The Windows CA is initially created as a copy of the User CA, so existing trust relationships are maintained. You may rotate either CA in order to create distinct key material (make sure to consult the Certificate Authority Rotation guide before performing a CA rotation). The Windows CA is a top-level CA entity, so it is reflected in all commands that operate on CAs. Updating both command-line tools and Windows Desktop agents is recommended. [#63547](https://github.com/gravitational/teleport/pull/63547)
* Added support for summarizer resources to the Teleport Terraform provider. [#63534](https://github.com/gravitational/teleport/pull/63534)
* Add Managed Updates dashboard to the WebUI. [#63310](https://github.com/gravitational/teleport/pull/63310)
* Fixed a bug that could cause Windows desktops discovered via LDAP to be removed in error. [#62471](https://github.com/gravitational/teleport/pull/62471)
* Fixed an issue that could cause failed Active Directory user lookups to cache the error rather than retry. [#62471](https://github.com/gravitational/teleport/pull/62471)
* Ensure that discovered Windows desktops don't expire when a large discovery interval is configured. [#62471](https://github.com/gravitational/teleport/pull/62471)
* Each Windows desktop `discovery_config` can now include a set of static labels to apply to discovered hosts. [#62452](https://github.com/gravitational/teleport/pull/62452)
* Added support for discovering EC2 instances in all the accounts under an AWS Organization. [#62302](https://github.com/gravitational/teleport/pull/62302)
* Added support for EC2 instances to join based on their AWS Organization. [#62302](https://github.com/gravitational/teleport/pull/62302)

Enterprise:
* Updated Entra ID plugin UI to support Access List owners source configuration.
* Fixes a panic that occurred when External Audit Storage was available but not enabled in Teleport Cloud while Access Monitoring was enabled.
* Added plugin status page for Teleport Entra ID integration.

## 18.6.8 (02/10/26)

* Added `--exec-cmd` and `--exec-arg` flags to `tsh proxy kube` to allow launching custom commands like k9s directly without requiring environment variable workarounds. [#63066](https://github.com/gravitational/teleport/pull/63066)

Enterprise:
* Fixes a panic that occurred when External Audit Storage was available but not enabled in Teleport Cloud while Access Monitoring was enabled.

## 18.6.7 (02/09/26)

* Revised help messages for event handler CLI commands. [#63620](https://github.com/gravitational/teleport/pull/63620)
* Fixed `tsh ssh user@foo=bar uptime` from running serially if users did not have `role:read` permissions. [#63612](https://github.com/gravitational/teleport/pull/63612)
* The minimum version of macOS required to run Teleport or associated client tools is now macOS 12 (Monterey). [#63587](https://github.com/gravitational/teleport/pull/63587)
* The minimal macOS version required by Teleport Connect is now macOS 12. [#63569](https://github.com/gravitational/teleport/pull/63569)
* Fixed bug where event handler would throw an error on Athena backend when handling large events. [#63550](https://github.com/gravitational/teleport/pull/63550)
* Updated Go to 1.25.7. [#63539](https://github.com/gravitational/teleport/pull/63539)
* Fixed an issue where a role requiring a trusted device could incorrectly block access to all applications. [#63527](https://github.com/gravitational/teleport/pull/63527)
* Fixed bug where event handler would get stuck on DynamoDB backend when handling large events. [#63526](https://github.com/gravitational/teleport/pull/63526)
* Updated tsh/Linux to correctly capture the OS login user for device trust. [#63452](https://github.com/gravitational/teleport/pull/63452)
* Fixed a server error when rejecting a headless authentication request in the Web UI. [#63431](https://github.com/gravitational/teleport/pull/63431)
* Added opt-in support to use `cert-manager` certificates for `teleport-plugin-event-handler` helm chart. [#63420](https://github.com/gravitational/teleport/pull/63420)
* Modified `tbot` helm chart with default `token` value to simplify deployment. [#63360](https://github.com/gravitational/teleport/pull/63360)
* Improved GitHub + Kubernetes guide experience. [#63185](https://github.com/gravitational/teleport/pull/63185)
* Fixed `teleport join openssh` on recent versions of Ubuntu. [#63040](https://github.com/gravitational/teleport/pull/63040)

Enterprise:
* Extend Access Monitoring feature to Teleport Cloud customers using External Audit Storage.
* Added recording and validation for the fixed OS login user values from tsh.
* Mitigated a race in the Slack token refresh logic.

## 18.6.6 (02/02/26)

* Fixed tsh/Linux sending a too-large username for device trust. [#63387](https://github.com/gravitational/teleport/pull/63387)
* Fixed an issue where MCP JSON-RPC messages with mixed-case field names could be parsed inconsistently and re-serialized to lower cases. Teleport now enforces canonical lowercase JSON-RPC fields. [#63364](https://github.com/gravitational/teleport/pull/63364)
* Improved robustness of the Slack hosted plugin to reduce the likeliness of failed token refresh when experiencing external disruption. [#63344](https://github.com/gravitational/teleport/pull/63344)
* Fixed a bug affecting access list review queries for lists where the name is a prefix of another list name. [#63337](https://github.com/gravitational/teleport/pull/63337)
* Updated the OCI SDK to support new regions. [#63265](https://github.com/gravitational/teleport/pull/63265)
* Ensure application session rejections for untrusted devices are consistently audited as AppSessionStart failures after MFA. [#63149](https://github.com/gravitational/teleport/pull/63149)
* Added Helm chart support to the `teleport-event-handler configure` command. [#63147](https://github.com/gravitational/teleport/pull/63147)
* Added `tctl` support for removing `okta_assignment` internal resource should it be needed. [#62698](https://github.com/gravitational/teleport/pull/62698)

Enterprise:
* Prevented manual membership changes to SCIM-type access lists while enabling support for their reviews.
* Fixed the issue where Okta integration may not remove previously synced apps after plugin restart.

## 18.6.5 (01/29/26)

* Fixed a `CredentialContainer` error when attempting to log in to the Web UI with a hardware key using Firefox >=147.0.2. [#63245](https://github.com/gravitational/teleport/pull/63245)
* Added support for deleting cluster alerts via `tctl alerts rm <alertID>` command. [#63211](https://github.com/gravitational/teleport/pull/63211)
* Updated OpenSSL to 3.0.19. [#63202](https://github.com/gravitational/teleport/pull/63202)
* Added support for injecting Teleport-issued ID tokens into outgoing MCP requests, enabling integrations with MCP servers such as the AWS Bedrock AgentCore MCP Gateway that can validate tokens via OIDC discovery URL. [#63156](https://github.com/gravitational/teleport/pull/63156)
* Export "additional_trusted_keys" when exporting TLS CAs, which includes new certificates generated in the "init" rotation phase. Reflected in "tctl auth export" and the "/webapi/auth/export" endpoint. [#63134](https://github.com/gravitational/teleport/pull/63134)
* Updated indirect dependency go-chi/chi/v5 (addresses GO-2026-4316). [#63092](https://github.com/gravitational/teleport/pull/63092)
* The `tbot systemd install` command now supports a `--pid-file` flag for setting the path to the PID file. [#63039](https://github.com/gravitational/teleport/pull/63039)
* Allow kubeconfig and context to be explicitly configured for `tbot` `kubernetes_secret` destination. [#63037](https://github.com/gravitational/teleport/pull/63037)
* Implemented "tctl get cert_authority/catype", in addition to the already existing "tctl get cert_authority" and "tctl get cert_authority/catype/domain". [#63027](https://github.com/gravitational/teleport/pull/63027)
* Added a Terraform module to configure Teleport and AWS for EC2 discovery in an AWS account. [#63004](https://github.com/gravitational/teleport/pull/63004)
* Added opt-in support to bootstrap the `teleport-plugin-event-handler` helm chart with MWI to auto-join Teleport clusters when Operator is enabled. [#63001](https://github.com/gravitational/teleport/pull/63001)
* Added permissions to the `editor` role allowing users to view autoupdate agent reports. [#62973](https://github.com/gravitational/teleport/pull/62973)
* Improved performance of large search queries for DynamoDB event backend. [#62890](https://github.com/gravitational/teleport/pull/62890)
* Introduced tbot-spiffe-daemon-set helm chart for deploying a Daemon Set of tbot agents which serve SPIFFE SVIDs to Kubernetes pods via the SPIFFE Workload API. [#62583](https://github.com/gravitational/teleport/pull/62583)

Enterprise:
* Fixed an issue with the legacy Azure OIDC IdP SSO `issuer=sts.windows.net` where Teleport was unable to map Teleport roles from the groups claim available in the `id_token`.
* Added updated resources to SCIM audit events that create or change SCIM resources.
* Support multi-arch lock file population for the terraform provider.
* Added audit events to SCIM PATCH operations.
* Updated Entra ID plugin to support importing Entra ID group owners as Access List owners.
* Replaced enterprise downloads list view in Web UI with links to Public Downloads page.

## 18.6.4 (01/20/26)

* Fixed GCS session recording backend not respecting rate limits. [#62986](https://github.com/gravitational/teleport/pull/62986)
* Fixed a bug where members of a former owner Access List retain the owner permissions grants of the former owned Access List. It also fixes the issue with not being able to delete a former owner Access List. Please note: this could only happen if the owner Access List ownership was removed via the web UI. [#62979](https://github.com/gravitational/teleport/pull/62979)
* Tctl commands executed from Teleport Connect now target the current root cluster with the `TELEPORT_AUTH_SERVER` env var, similar to how it works for tsh; this behavior can be turned off in the config file. [#62923](https://github.com/gravitational/teleport/pull/62923)
* Made the `teleport-cluster` Helm chart job resources configurable again via the `jobResources` value. [#62922](https://github.com/gravitational/teleport/pull/62922)
* Updated Go to 1.24.12. [#62885](https://github.com/gravitational/teleport/pull/62885)
* Fixed launching AWS Identity Center from Teleport Connect. [#62840](https://github.com/gravitational/teleport/pull/62840)
* Removed erroneous `pair-wise` subject type from Teleport's OpenID configuration. [#62835](https://github.com/gravitational/teleport/pull/62835)
* Fixed renewed X509-SVIDs not being proactively sent to Envoy instances. [#62830](https://github.com/gravitational/teleport/pull/62830)
* Fix an issue `MCP Session Listen` events may spam audit log with app service error `malformed line in SSE stream: ""`. [#62811](https://github.com/gravitational/teleport/pull/62811)
* Added automatic client certificate reloading option for postgres backends. [#62747](https://github.com/gravitational/teleport/pull/62747)
* Fixed an issue that would prevent tsh from working when the 1password SSH agent is running. [#62736](https://github.com/gravitational/teleport/pull/62736)
* Add `tbot wait` API and helper to let scripts wait for bots to become ready. [#62719](https://github.com/gravitational/teleport/pull/62719)
* MWI: Add support for templating secret annotations in the tbot's `kubernetes/argo-cd` service. [#62709](https://github.com/gravitational/teleport/pull/62709)
* Add `quicksight.aws.amazon.com` as valid URL for AWS Console access. [#62700](https://github.com/gravitational/teleport/pull/62700)
* Fixed potential delay in updating User Task status for Discovery resources. [#62699](https://github.com/gravitational/teleport/pull/62699)
* Fixed an issue where logging in to the Web UI with Device Trust would lose query params of the redirect URL. [#62677](https://github.com/gravitational/teleport/pull/62677)
* Fixed an issue where Teleport Connect could generate a flurry of notifications about not being able to connect to a resource. [#62671](https://github.com/gravitational/teleport/pull/62671)
* Fixed issuance of wildcard DNS SANs with Workload Identity. [#62667](https://github.com/gravitational/teleport/pull/62667)
* Fixed a memory leak in access list reminder notifications affecting clusters with more than 1000 pending Access List reviews. [#62663](https://github.com/gravitational/teleport/pull/62663)
* Added support for health checks to monitor cert authority availability and affect Teleport Auth readiness. [#62637](https://github.com/gravitational/teleport/pull/62637)
* Added IAM joining support from new AWS regions in asia. [#62627](https://github.com/gravitational/teleport/pull/62627)
* Added VNet config Create/Update/Delete audit events. [#62618](https://github.com/gravitational/teleport/pull/62618)
* Added cleanup of access entries for EKS auto-discovered clusters when they no longer match the filtering criteria and are removed. [#62598](https://github.com/gravitational/teleport/pull/62598)
* Added `teleport debug metrics` command. [#62586](https://github.com/gravitational/teleport/pull/62586)
* Fixed missing initialization of Azure IMDS clients, which could cause operational failures in some Teleport configurations deployed to Azure, in particular when accessing Azure SQL Server. [#62579](https://github.com/gravitational/teleport/pull/62579)
* Fixed some auto update audit events showing up as unknown in the web UI. [#62547](https://github.com/gravitational/teleport/pull/62547)
* The join tokens UI now indicates which tokens are managed by the Teleport Cloud platform. [#62544](https://github.com/gravitational/teleport/pull/62544)
* The tctl tokens add command now includes the CA pins in JSON and YAML output. [#62536](https://github.com/gravitational/teleport/pull/62536)
* Added `teleport debug readyz` command. [#62532](https://github.com/gravitational/teleport/pull/62532)
* Audit log and session uploader now respect region field of external_audit_storage resource when present. [#62520](https://github.com/gravitational/teleport/pull/62520)
* Added default routes to the web UI left nav top-level category buttons. [#62502](https://github.com/gravitational/teleport/pull/62502)
* Fixed an issue that prevented searching for users by role in the web UI. [#62474](https://github.com/gravitational/teleport/pull/62474)
* Fixed tilde expansion for moderated SFTP. [#62453](https://github.com/gravitational/teleport/pull/62453)
* Added support for standard TLS secret key names for helm charts: `teleport-plugin-event-handler`, `teleport-cluster`, `teleport-operator`, `teleport-kube-agent`. [#62451](https://github.com/gravitational/teleport/pull/62451)
* Added a plan modifier to recompute kubernetes_resources defaults during role version upgrades, fixing Terraform role upgrade issues. [#62417](https://github.com/gravitational/teleport/pull/62417)
* Fix an issue in the Teleport SSH Service where interactive PAM Auth modules always fail when trying to run exec sessions with tty allocated. e.g. `tsh ssh --tty <node> ls`. [#62064](https://github.com/gravitational/teleport/pull/62064)

Enterprise:
* Fixed an issue in the Entra ID integration where a user account with an unsupported username character `/` could prevent other valid users and groups to be synced to Teleport. Such user accounts are now filtered.
* Cockroachdb: add automatic client certificate reloading option.
* Enabled UI editing of Access List descriptions.
* Added protections against replay attacks when IdP-initiated SAML is enabled.
* Added Access Automations Terraform dialog.

## 18.6.3 (01/07/26)

This is a follow up to the private security release. Changelog will be publicly announced in a later version.

In addition to the previous release it includes the following bug fixes:

* Fixed a memory leak in access list reminder notifications affecting clusters with more than 1000 pending Access List reviews. [#62663](https://github.com/gravitational/teleport/pull/62663)

## 18.6.2 (12/26/25)

This is a private security release. Changelog will be publicly announced in a later version.

## 18.6.1 (12/24/25)

* Fixed an issue preventing text editors in the Web UI from allowing edits. [#62488](https://github.com/gravitational/teleport/pull/62488)
* Acking a cluster alert no longer requires the create permission. [#62468](https://github.com/gravitational/teleport/pull/62468)
* Fixed service health reason formatting for bot instances in the Web UI. [#62328](https://github.com/gravitational/teleport/pull/62328)
* Fixed an issue causing a ref type of "any" to be added when editing GitHub or Gitlab join tokens in the Web UI. [#62487](https://github.com/gravitational/teleport/pull/62487)

## 18.6.0 (12/22/25)

### Identifier-first login enhancements
Teleport now automatically passes the username to the identifier provider when performing Identifier-first login with OIDC or SAML IdPs.

### GitHub Actions Kubernetes Wizard
Teleport now ships with a new guided flow for setting up GitHub Actions workflows that connects to Teleport-protected Kubernetes clusters without secrets.

### Other changes and improvements

* Fixed unspecified proxy address breaking moderated SFTP when mixing IPv4 and IPv6. [#62296](https://github.com/gravitational/teleport/pull/62296)
* Added full configuration file for `teleport-plugin-event-handler` helm chart. [#62280](https://github.com/gravitational/teleport/pull/62280)
* Added full environment variable configuration for event handler CLI. [#62280](https://github.com/gravitational/teleport/pull/62280)
* Added support for extraArgs/extraEnv/extraLabels patterns for `teleport-plugin-event-handler` helm chart. [#62266](https://github.com/gravitational/teleport/pull/62266)
* Fixed issue where AltGr key combinations did not work correctly in remote desktop sessions. [#62198](https://github.com/gravitational/teleport/pull/62198)
* Added `annotations` support for `teleport-plugin-event-handler` helm chart. [#62188](https://github.com/gravitational/teleport/pull/62188)
* Added a new global configuration section auth_connection_config allowing users to configure the backoff behavior for Proxy and Agent instances connecting to the Auth Service. [#62139](https://github.com/gravitational/teleport/pull/62139)
* Fixed a potential SSRF vulnerability in the Azure join method implementation. [#62406](https://github.com/gravitational/teleport/pull/62406)
* Support for v8 roles has been added to the Terraform provider. [#62380](https://github.com/gravitational/teleport/pull/62380)
* Added support for selecting Kube agents as Managed Updates v2 canaries. Important: the default update group is corrected to "default" from "stable/cloud". [#62211](https://github.com/gravitational/teleport/pull/62211)

## 18.5.1 (12/12/25)

* Fixed Teleport instances running the Auth Service sometimes not becoming ready during initialization. [#62194](https://github.com/gravitational/teleport/pull/62194)
* Fixed an Auth Service bug causing the event handler to miss up to 1 event every 5 minutes when storing audit events in S3. [#62150](https://github.com/gravitational/teleport/pull/62150)
* Fixed bug where event handler dies on malformed session events. [#62141](https://github.com/gravitational/teleport/pull/62141)
* Updated event handler to ingest missing session recordings at twice the `concurrency` instead of only 10 sessions at a time. [#62141](https://github.com/gravitational/teleport/pull/62141)
* Changed "tsh --mfa-mode=cross-platform" to favor security keys on current Windows versions. [#62134](https://github.com/gravitational/teleport/pull/62134)
* Fixed "the client connection is closing" error happening under certain conditions in Teleport Connect when connecting to resources with per-session MFA enabled. [#62127](https://github.com/gravitational/teleport/pull/62127)
* Improved detail of error messages for `identity` service in `tbot`. [#62120](https://github.com/gravitational/teleport/pull/62120)
* Teleport Connect now supports expanding `~/` home-directory paths in the configuration file. [#62104](https://github.com/gravitational/teleport/pull/62104)
* Added support for --format flag for `tsh request search`. [#62099](https://github.com/gravitational/teleport/pull/62099)
* Fixed bug where event handler `types` filter is ignored for Teleport clients using Athena storage backend. [#62082](https://github.com/gravitational/teleport/pull/62082)
* Fixed intermittent issues with VNet on Windows when other NRPT rules from GPOs are present under `HKLM:\SOFTWARE\Policies\Microsoft\Windows NT\DNSClient\DnsPolicyConfig`. [#62052](https://github.com/gravitational/teleport/pull/62052)
* Added Terraform provider support for teleport_integration resources. [#62040](https://github.com/gravitational/teleport/pull/62040)
* DiscoveryConfig resources can now be managed via the Teleport Terraform Provider. [#62034](https://github.com/gravitational/teleport/pull/62034)
* Reduced memory consumption of the Application service. [#62014](https://github.com/gravitational/teleport/pull/62014)
* Added support for listing application session recordings in `tsh recording ls` and the Web UI. [#62010](https://github.com/gravitational/teleport/pull/62010)
* Fixed a Web UI issue where the copy button for the session ID did not work for non-interactive session recordings. [#62010](https://github.com/gravitational/teleport/pull/62010)
* Prevented stuck `teleport-cluster` Helm chart rollouts in small Kubernetes clusters. Removed resource requests from configuration check hooks. [#62003](https://github.com/gravitational/teleport/pull/62003)
* Fixed static keypair creation in `tbot keypair create` when the `--static-key-path` flag is used. [#61947](https://github.com/gravitational/teleport/pull/61947)
* Re-enabled MySQL database health checks. MySQL health checks will now authenticate to the database as a user, rather than TCP dialing and closing the connection, to prevent MySQL from automatically blocking the Teleport database service instance host. The health check user name default is "teleport-healthchecker". [#61942](https://github.com/gravitational/teleport/pull/61942)
* Added support for templating `secret_labels`, and the `{{.Labels}}` template variable, to tbot's `kubernetes/argo-cd` output. [#61876](https://github.com/gravitational/teleport/pull/61876)

Enterprise:
* Updated AWS Identity Center integration sign-in start URL format to support AWS GovCloud accounts.
* Fix a potential race where Okta assignments may never be cleaned up if the Okta integration is down while the assignment expires.
* Created a dedicated Access Automations feature page within the Web UI.
* Entra ID directory reconciler now overwrites user accounts created by the referenced SAML Auth Connector.

## 18.5.0 (12/04/25)

### Kubernetes support for Relay Service
The relay service now facilitates Kubernetes connections.

### Shared state between tsh and Teleport Connect
Teleport Connect and tsh now share the same local state. Logins from one app will automatically be reflected in the other.

### SCIM PATCH support in SailPoint integration
Teleport SCIM server now natively supports PATCH operations to improve reliability of bulk SCIM operations in integrations like SailPoint.

### Other changes and improvements

* Updated Go to 1.24.11. [#61953](https://github.com/gravitational/teleport/pull/61953)
* Added support for discovering EC2 instances in all regions, without enumerating them. Requires access to `account.ListRegions` in the IAM Role assumed by the Discovery Service. [#61924](https://github.com/gravitational/teleport/pull/61924)
* Fixed a bug where JWT-SVID timestamp claims would be represented using scientific notation. [#61921](https://github.com/gravitational/teleport/pull/61921)
* Fixed "SSH cert not found" errors in Teleport Connect. [#61846](https://github.com/gravitational/teleport/pull/61846)
* Added support for authenticating Azure resource discovery using Azure OIDC integrations. [#61830](https://github.com/gravitational/teleport/pull/61830)
* Fixed a bug in Proxy recording mode where Teleport Node sessions would result in duplicate audit events with a different session ID. [#61246](https://github.com/gravitational/teleport/pull/61246)
* Tuned teleport-cluster, teleport-kube-agent, and teleport-relay Helm charts to reduce the probability of Teleport exceeding its memory limits and being OOM-Killed. GOMEMLIMIT defaults to 90% of the configured memory limits.

Enterprise:
* Added support for AWS Account name and ID labels (`teleport.dev/account-id`, `teleport.dev/account-name`) on AWS Identity Center resources (`aws_ic_account_assignment` and `aws_ic_account`). These labels improve compatibility with Access Monitoring Rules, allowing users to more easily target and audit AWS IC accounts.
* Updated the Access Automation Rules dialog to display rules in a paginated view.

## 18.4.2 (12/01/25)

* Fixed a bug causing high memory consumption in the Teleport Auth Service when clients were listing large resources. [#61849](https://github.com/gravitational/teleport/pull/61849)
* Prevent data races when terminating interactive Kubernetes sessions. [#61818](https://github.com/gravitational/teleport/pull/61818)
* Fixed `tsh db connect` failing to connect to databases using separate ports configuration (non-TLS routing mode). [#61812](https://github.com/gravitational/teleport/pull/61812)
* Fixed a bug where Kubernetes App Discovery `poll_interval` is not set correctly. [#61791](https://github.com/gravitational/teleport/pull/61791)
* Fixed an issue that caused a failed upload of an encrypted session recording to block other recordings from uploading. [#61774](https://github.com/gravitational/teleport/pull/61774)
* Fixed relative path evaluation for SFTP in proxy recording mode. [#61760](https://github.com/gravitational/teleport/pull/61760)
* Fixed `tsh kube ls` showing deleted clusters. [#61742](https://github.com/gravitational/teleport/pull/61742)
* Fixed workload identity templating to support certain numeric values that previously gave a "expression did not evaluate to a string" error. [#61738](https://github.com/gravitational/teleport/pull/61738)
* Added User Details view to Web UI. [#61737](https://github.com/gravitational/teleport/pull/61737)
* Added --roles flag for tsh request search, allowing users to list all requestable roles. This flag is mutually exclusive with --kind. [#61699](https://github.com/gravitational/teleport/pull/61699)
* Fixed EC2 SSM Document set up script used in Enroll New Resource. [#61673](https://github.com/gravitational/teleport/pull/61673)
* Fixed AWS Console access when using AWS IAM Roles Anywhere or AWS OIDC integrations, when IP Pinning is enabled. [#61654](https://github.com/gravitational/teleport/pull/61654)
* Fixed "invalid name syntax" connection error for PostgreSQL auto-provisioned users with email usernames. [#61631](https://github.com/gravitational/teleport/pull/61631)
* Auth readiness tuned to wait for cache initialization. [#61620](https://github.com/gravitational/teleport/pull/61620)
* Added ability to update existing Azure OIDC integration with `tctl`. [#61592](https://github.com/gravitational/teleport/pull/61592)

Enterprise:
* Added Entra directory sync metrics.
* Improved the initial EntraID user and group synchronization time, reducing the time required for the first full sync.
* Prevented Trivy from reporting false positives when scanning the Teleport binaries.

## 18.4.1 (11/20/25)

* Fixed a bug that prevented searching audit log events in the web UI when using Athena audit storage. [#61603](https://github.com/gravitational/teleport/pull/61603)
* Prevented Trivy from reporting false positives when scanning the Teleport binaries. [#61539](https://github.com/gravitational/teleport/pull/61539)
* Added support for `tsh logout --proxy` (or `TELEPORT_PROXY` set) to work without `--user` flag when one identity exists. [#61404](https://github.com/gravitational/teleport/pull/61404)
* Fixed web upload/download failure behind load balancers when web listen address is unspecified. [#61393](https://github.com/gravitational/teleport/pull/61393)
* Fixed corrupted private keys breaking tsh. [#61388](https://github.com/gravitational/teleport/pull/61388)
* Resource names are now properly validated for AWS Roles Anywhere integration `Generate Command`. [#61385](https://github.com/gravitational/teleport/pull/61385)
* Added caches to reduce Active Directory user SID lookups and TLS certificate requests. [#61317](https://github.com/gravitational/teleport/pull/61317)
* GOAWAY errors received from Kubernetes API Servers configured with a non-zero --goaway-chance are now forward to clients to be retried. [#61256](https://github.com/gravitational/teleport/pull/61256)
* Added support for creating and managing scoped tokens using `tctl scoped tokens add/ls/rm`. SSH nodes can now join a cluster within a particular scope by joining with a scoped token. [#60758](https://github.com/gravitational/teleport/pull/60758)

Enterprise:
* Removed sync of the model identifier from Intune to avoid mismatches between the identifier reported by Intune vs Teleport clients.
* Added support for Jamf's /v2/computers-inventory API (addresses Jamf's deprecation of /v1/computers-inventory).
* Updated the AWS Identity Center resource synchronizer to handle AWS Account name changes more gracefully.
* Added audit events in response to SCIM provisioning requests.

## 18.4.0 (11/13/25)

### Streamable-HTTP and SSE support for MCP Zero-Trust Access
MCP Zero-Trust Access users are now able to secure and audit connections to MCP servers that use HTTP-based transport protocols in addition to stdio.

### Improved Bot Instances Dashboard
The Bot Instances dashboard now provides a more intuitive interface for managing a fleet of Machine & Workload Identity bot instances. This includes improved filtering, sorting and searching capabilities, and a high-level overview of the versions of all bot instances in the cluster.

### Updated Oracle Joining Support
Oracle compute instances are no longer required to have additional IAM permissions granted to them in order to join. Oracle join tokens now also allow restricting which instances may leverage a token to join.

### Other changes and improvements

* Fixed an issue connections to MongoDB Atlas clusters fail if clusters use certs signed by Google Trust Services (GTS). [#61324](https://github.com/gravitational/teleport/pull/61324)
* Improved reverse tunnel dialing recovery from default route changes by 1min on average. [#61319](https://github.com/gravitational/teleport/pull/61319)
* Fixed an issue Postgres database cannot be accessed via Teleport Connect when per-session MFA is enabled and the role does not have wildcard `db_names`. [#61299](https://github.com/gravitational/teleport/pull/61299)
* Improved conflict detection of application public address and Teleport cluster addresses. [#61290](https://github.com/gravitational/teleport/pull/61290)
* Fixed AWS Roles Anywhere cli access when using per-session MFA. [#61273](https://github.com/gravitational/teleport/pull/61273)
* Fixed rare error in the `authorized_keys` secret scanner when running the Teleport agent on MacOS. [#61268](https://github.com/gravitational/teleport/pull/61268)
* Updated Go to v1.24.10. [#61212](https://github.com/gravitational/teleport/pull/61212)
* Terraform: `teleport_bot` resource now supports import, and follows the standard resource structure. [#61201](https://github.com/gravitational/teleport/pull/61201)
* Added support for tbot to teleport-update. [#61198](https://github.com/gravitational/teleport/pull/61198)
* Instrumented tbot to better support teleport-update. [#61189](https://github.com/gravitational/teleport/pull/61189)
* Improved error message of `tsh` when there is a certificate DNS SAN mismatch when connecting to Auth via Proxy. [#61186](https://github.com/gravitational/teleport/pull/61186)
* Improved error handling during desktop sessions that encounter unknown/invalid smartcard commands. This prevents abrupt desktop session termination with a "PDU error" message when using certain applications. [#61180](https://github.com/gravitational/teleport/pull/61180)
* Fixed an issue causing Access Automation Rules to evaluate incorrectly when users are granted traits via Access Lists. [#61169](https://github.com/gravitational/teleport/pull/61169)
* Added support for tsh copying files between two hosts, i.e. `tsh scp alice@foo:/path/1.txt bob@bar:/path/2.txt`. [#61165](https://github.com/gravitational/teleport/pull/61165)
* Added support for custom reason prompts for Access Requests, per requested role/resource (`role.spec.allow.request.reason.prompt`). [#61127](https://github.com/gravitational/teleport/pull/61127)
* Fixed the webUI timeout time to respect the cluster's WebIdleTimeout configuration. [#61103](https://github.com/gravitational/teleport/pull/61103)
* Added an option to restrict Oracle join tokens to specific instance IDs. [#61078](https://github.com/gravitational/teleport/pull/61078)
* Stabilized tsh paths when run from agent installation. [#60873](https://github.com/gravitational/teleport/pull/60873)
* Added advanced search and sorting to the bot instances list in the web UI. [#60761](https://github.com/gravitational/teleport/pull/60761)
* Added filter and sort flags to `tctl bots instances ls`. [#60761](https://github.com/gravitational/teleport/pull/60761)
* Added service health to the output `tctl bots instances ls` and `tctl bot instance show` commands. [#60761](https://github.com/gravitational/teleport/pull/60761)
* Added a dashboard to visualize bot instances by their version compatibility. [#60761](https://github.com/gravitational/teleport/pull/60761)
* Added bot instance service health to web UI. [#60761](https://github.com/gravitational/teleport/pull/60761)
* Added new `env0` join method to support joining within Env0 workflows. [#60710](https://github.com/gravitational/teleport/pull/60710)
* Added a new OCI join method that does not require IAM policies. [#60293](https://github.com/gravitational/teleport/pull/60293)

## 18.3.2 (11/07/25)

* Updated github.com/containerd/containerd dependency to fix https://github.com/advisories/GHSA-pwhc-rpq9-4c8w. [#61143](https://github.com/gravitational/teleport/pull/61143)
* Fixed regression when connecting to non-AD desktops. [#61117](https://github.com/gravitational/teleport/pull/61117)
* Fixed a bug causing `tsh` to stop waiting for access request approval and incorrectly report that the request had been deleted. [#61109](https://github.com/gravitational/teleport/pull/61109)
* Fixed an issue where resources in Teleport Connect were not always refreshed correctly after re-logging in as a different user. [#61099](https://github.com/gravitational/teleport/pull/61099)

Enterprise:
* Added support for Amazon Bedrock to session recording summarizer (unavailable in Teleport Cloud). [#7463](https://github.com/gravitational/teleport.e/pull/7463)

## 18.3.1 (11/04/25)

**Warning:** This release includes a regression that prevents connection to non-AD desktops.
The following workaround is available:
- Upgrade Windows Desktop Service to 18.3.2

* Fixed an issue MCP session end event is not being sent sometimes. [#61009](https://github.com/gravitational/teleport/pull/61009)
* Teleport's Windows Desktop service can now discover the KDC server address via DNS. [#60988](https://github.com/gravitational/teleport/pull/60988)
* Fixed Kubernetes metrics API unmarshaling errors causing kubectl top commands to fail in certain scenarios. [#60971](https://github.com/gravitational/teleport/pull/60971)
* Fixed an issue which could lead to session recordings saved on disk being truncated. [#60964](https://github.com/gravitational/teleport/pull/60964)
* Fixed a bug causing unencrypted session recordings to be deleted 24 hours after being created while using `node` and `proxy` recording modes. [#60948](https://github.com/gravitational/teleport/pull/60948)
* Enabled summarization and metadata generation for encrypted session recordings, storing metadata and summaries in encrypted form. [#60945](https://github.com/gravitational/teleport/pull/60945)
* Fixed a bug where encrypted sessions recordings could not be uploaded to S3. [#60895](https://github.com/gravitational/teleport/pull/60895)
* Added "tsh mcp config/connect" support for custom headers for streamable-HTTP MCP servers. [#60843](https://github.com/gravitational/teleport/pull/60843)
* Fixed the session recording player that was unable to play SSH sessions captured prior to v18.1.6. [#60832](https://github.com/gravitational/teleport/pull/60832)
* Fixed an issue in the web UI where a bot with zero tokens would show a validation error. [#60760](https://github.com/gravitational/teleport/pull/60760)
* Added the ability to set OIDC Integration credentials in the tctl AWS Identity Center plugin installer. [#60712](https://github.com/gravitational/teleport/pull/60712)
* Kubernetes OIDC responses are now cached to improve performance and reliability when joining bots and nodes. [#60711](https://github.com/gravitational/teleport/pull/60711)
* Fixed MongoDB topology monitoring connection leak in the Teleport Database Service. [#60692](https://github.com/gravitational/teleport/pull/60692)
* Added support for topologySpreadConstraints to the teleport-kube-agent Helm chart. [#58012](https://github.com/gravitational/teleport/pull/58012)
* The teleport-kube-agent Helm chart now tries to spread pods across hosts and zones. [#58012](https://github.com/gravitational/teleport/pull/58012)

## 18.3.0 (10/28/25)

### Web UI Workload ID

Teleport's Web UI now lists all workload identity resources registered in the cluster.

### Relay Service

Teleport now includes a new relay service that acts as a lightweight proxy service. This new service can receive connections from both SSH clients and agents.

The relay service can be used to avoid routing SSH connections through the broader Teleport control plane, providing the ability to optimize network flows in large or complex deployments.

### Multi-cluster Discovery

Multiple Teleport clusters can now discover the same EC2 instances simultaneously through auto-discovery, with each cluster operating independently without interference.

### Kubernetes Health Checks

Teleport now continuously monitors the health of your registered Kubernetes clusters and displays their status directly in the web UI. When connecting to Kubernetes clusters, Teleport automatically routes you to healthy services, ensuring reliable access to your infrastructure.

### ElastiCache Serverless

Teleport Database Access now supports connecting to ElastiCache Serverless databases.

### Other fixes and improvements

* The browser window for SSO MFA is slightly taller in order to accommodate larger elements like QR codes. [#60703](https://github.com/gravitational/teleport/pull/60703)
* Slack access plugin no longer crashes in the event access list is unsupported. [#60671](https://github.com/gravitational/teleport/pull/60671)
* Okta-managed apps are now pinned correctly in the web UI. [#60667](https://github.com/gravitational/teleport/pull/60667)
* Create and edit GitLab join tokens from the Web UI. [#60649](https://github.com/gravitational/teleport/pull/60649)
* Teleport Connect now displays the profile name (instead of the cluster name) in the UI when referring to the profile; this affects only clusters where the cluster name was specifically set to something else than the proxy hostname during setup. [#60615](https://github.com/gravitational/teleport/pull/60615)
* Fixed tsh scp failing on files that grow during transfer. [#60607](https://github.com/gravitational/teleport/pull/60607)
* Allowed moderated session peers to perform file transfers. [#60604](https://github.com/gravitational/teleport/pull/60604)
* Added support for regular expression conditions for AccessMonitoringRule. [#60598](https://github.com/gravitational/teleport/pull/60598)
* Added support for SSE and streamable-HTTP MCP servers. [#60519](https://github.com/gravitational/teleport/pull/60519)
* Added health checks for enrolled Kubernetes clusters. [#60492](https://github.com/gravitational/teleport/pull/60492)
* MWI: `tbot`'s auto-generated service names are now simpler and easier to use in the `/readyz` endpoint. [#60458](https://github.com/gravitational/teleport/pull/60458)
* Client tools managed updates stores OS and ARCH in the configuration. This ensures compatibility when `TELEPORT_HOME` directory is shared with a virtual instance running a different OS or architecture. [#60414](https://github.com/gravitational/teleport/pull/60414)
* Added a Workload Identities page to the web UI to list workload identities. [#59479](https://github.com/gravitational/teleport/pull/59479)

Enterprise:
* Enabled Access Automation Rule schedule configuration within the WebUI.
* Updated Entra ID plugin installation UI to support group filter configuration.

## 18.2.10 (10/23/25)

* Fixed a bug where listing members of an access list results in listing members of access lists which have names prefixed with the original access list name. This may lead to RBAC escalations. [#60587](https://github.com/gravitational/teleport/pull/60587)
* Fixed a startup error `EADDRINUSE: address already in use` in Teleport Connect on macOS and Linux that could occur with long system usernames. [#60576](https://github.com/gravitational/teleport/pull/60576)
* Fixed an issue where the eligibility reconsideration flow could continuously reset the Owner’s eligibility status when the Access List contains a dangling reference to a non-existent user. [#60575](https://github.com/gravitational/teleport/pull/60575)
* Fixed Username AccessList name collision. [#60563](https://github.com/gravitational/teleport/pull/60563)
* Playback speed can be changed in the new SSH/k8s recording player. [#60451](https://github.com/gravitational/teleport/pull/60451)
* Adapts EC2 Server auto discovery to send the correct parameters when using the `AWS-RunShellScript` pre-defined SSM Document. [#60434](https://github.com/gravitational/teleport/pull/60434)
* Updated tsh debug output to include tsh client version when --debug flag is set. [#60407](https://github.com/gravitational/teleport/pull/60407)
* Updated LDAP dial timeout from 15 seconds to 30 seconds. [#60388](https://github.com/gravitational/teleport/pull/60388)
* Fixed a bug that prevented using database role names longer than 30 chars for MySQL auto user provisioning. Now role names as long as 32 chars, which is the MySQL limit, can be used. [#60377](https://github.com/gravitational/teleport/pull/60377)
* Fixed a bug in Proxy Recording Mode that causes SSH sessions in the WebUI to fail. [#60369](https://github.com/gravitational/teleport/pull/60369)
* Added `extraEnv` and `extraArgs` to the teleport-operator helm chart. [#60357](https://github.com/gravitational/teleport/pull/60357)
* Fixed issue with inherited roles interfering with auto role provisioning cleanup in Postgres. [#60345](https://github.com/gravitational/teleport/pull/60345)
* Fixed malformed audit events breaking the audit log. [#60334](https://github.com/gravitational/teleport/pull/60334)
* Enabled use of schedules within automatic review and notification access_monitoring_rules. [#60327](https://github.com/gravitational/teleport/pull/60327)
* Fixed an issue that caused Kubernetes debug containers to fail with a “container not valid” error when launched by a user requiring moderated sessions. [#60302](https://github.com/gravitational/teleport/pull/60302)
* Added `tbot start ssh-multiplexer` helper to start the SSH multiplexer service without a config file. [#60287](https://github.com/gravitational/teleport/pull/60287)
* Fixed "The server-side graphics subsystem is in an error state" during connection initialization to Windows Desktop. [#60285](https://github.com/gravitational/teleport/pull/60285)
* Fixed a bug where SSH host certificates are missing the `<hostname>.<clustername>` principal, breaking SSH access via third-party clients. [#60276](https://github.com/gravitational/teleport/pull/60276)
* Reduces the memory usage when processing a session recording by ~80%. [#60275](https://github.com/gravitational/teleport/pull/60275)
* Fixed AWS CLI access when using the AWS Roles Anywhere integration. [#60227](https://github.com/gravitational/teleport/pull/60227)
* Fixed an issue in Teleport Connect where Ctrl+D would sometimes not close a terminal tab. [#60221](https://github.com/gravitational/teleport/pull/60221)
* Updated error messages displayed by tsh ssh when access to hosts is denied and when attempting to connect to a host that is offline or not enrolled in the cluster. [#60215](https://github.com/gravitational/teleport/pull/60215)
* Added editing bot description to the web UI. [#60212](https://github.com/gravitational/teleport/pull/60212)
* Added support for PodSecurityContext to `tbot` helm chart. [#60206](https://github.com/gravitational/teleport/pull/60206)
* MWI: Add `teleport_bot_instances` metric. [#60196](https://github.com/gravitational/teleport/pull/60196)
* The `tbot` Workload API now logs errors encountered when handling requests. [#60193](https://github.com/gravitational/teleport/pull/60193)
* Added explicit timeout to `tbot` when the Trust Bundle Cache is establishing an event watch. [#60182](https://github.com/gravitational/teleport/pull/60182)
* Fixed a bug where OpenSSH EICE node connections would fail. [#60124](https://github.com/gravitational/teleport/pull/60124)
* Updated Go to 1.24.9. [#60108](https://github.com/gravitational/teleport/pull/60108)
* Fixed SFTP audit events breaking the audit log. [#60069](https://github.com/gravitational/teleport/pull/60069)
* Fixed Access List owners permission inheritance when the nesting depth is one. (Members of an Access List configured as an Owner of another Access List). [#60056](https://github.com/gravitational/teleport/pull/60056)
* Added support for loading bound keypair joining parameters from the environment. [#60031](https://github.com/gravitational/teleport/pull/60031)
* Deleting an AWS OIDC integration will remove associated Teleport Discovery Configs and App servers that reference the integration. [#60018](https://github.com/gravitational/teleport/pull/60018)
* Fixed selinux warning in teleport-update output and error during remove. [#59997](https://github.com/gravitational/teleport/pull/59997)
* Fixed tsh scp getting stuck in symlink loops. [#59994](https://github.com/gravitational/teleport/pull/59994)
* Fixed handling of local tsh scp targets that contain a colon. [#59981](https://github.com/gravitational/teleport/pull/59981)
* Fixed EC2 auto discovery report of failed installations. [#59972](https://github.com/gravitational/teleport/pull/59972)
* Fixed issue where temporarily unreachable app servers were permanently removed from session cache, causing persistent connection failures: `no application servers remaining to connect`. [#59956](https://github.com/gravitational/teleport/pull/59956)
* Fixed the issue with automatic access requests for `tsh ssh` when `spec.allow.request.max_duration` is set on the requester role. [#59924](https://github.com/gravitational/teleport/pull/59924)
* Fixes a bug with the check for a running Teleport process in the install-node.sh script. [#59887](https://github.com/gravitational/teleport/pull/59887)
* Fixed handling SFTP file transfers when the SSH agent is enforced by SELinux. [#59874](https://github.com/gravitational/teleport/pull/59874)
* Periods of inactivity in SSH session playback can now be skipped. [#59701](https://github.com/gravitational/teleport/pull/59701)

Enterprise:
* Oracle database local proxies started with `tsh proxy db` will now accept connections to any database name.

## 18.2.9 (10/23/25)

This is a follow up to the private security release. Changelog will be publicly announced on 10/24/25.

In addition to the previous release it includes the following bug fixes:

* Fixed crash of EC2 auto discovery when AWS credentials provided in to the Discovery Service are not valid. [#60046](https://github.com/gravitational/teleport/pull/60046)

## 18.2.8 (10/20/25)

This is a follow up to the private security release. Changelog will be publicly announced on 10/24/25.

In addition to the previous release it includes the following bug fixes:

* Fixed issue with access list ineligibility status reconciler blocking member updates.
* Fixed issue with SSH host certificates missing the `<hostname>.<clustername>` principal, breaking SSH access via third-party clients.

## 18.2.7 (10/09/25)

This is a follow up to the private security release. Changelog will be publicly announced on 10/24/25.

In addition to the previous release it includes the following bug fixes:

* Fixed issue with automatic access requests for `tsh ssh` when `spec.allow.request.max_duration` is set on the requester role.

## 18.2.6 (10/06/25)

This is a follow up to the private security release. Changelog will be publicly announced on 10/24/25.

## 18.2.5 (10/02/25)

This is a private security release. Changelog will be publicly announced on 10/24/25.

## 18.2.4 (10/01/25)

* Fixed an issue where the new SSH/Kubernetes recording player would indefinitely show a loading spinner when seeking into a long period of inactivity. [#59816](https://github.com/gravitational/teleport/pull/59816)
* MWI: Added support for customizing context names with a template in `kubernetes/v2` output. [#59739](https://github.com/gravitational/teleport/pull/59739)
* Updated mongo-driver to v1.17.4 to include fixes for possible connection leaks that could affect Teleport Database Service instances. [#59732](https://github.com/gravitational/teleport/pull/59732)
* Fixed excessive memory usage on Teleport Proxy Service instances when using the the Teleport Web UI MySQL REPL. [#59719](https://github.com/gravitational/teleport/pull/59719)
* Added support for multiple agents in EC2, GCP and Azure Server auto discovery, allowing server access from different Teleport clusters. [#59688](https://github.com/gravitational/teleport/pull/59688)
* Changed the event-handler plugin to skip over Windows desktop session recording events by default. [#59681](https://github.com/gravitational/teleport/pull/59681)
* Fixed an issue that would cause trusted cluster resource updates to fail silently. [#58886](https://github.com/gravitational/teleport/pull/58886)

## 18.2.3 (09/29/25)

* Fixed auto-approvals in the Datadog Incident Management integration by updating the on-call API client. [#59668](https://github.com/gravitational/teleport/pull/59668)
* Fixed auto-approvals in the Datadog Incident Management integration to ignore case sensitivity in user emails. [#59668](https://github.com/gravitational/teleport/pull/59668)
* Database recordings now show the session summary if it is available. [#59634](https://github.com/gravitational/teleport/pull/59634)
* Added automatic `@<project-id>.iam` suffix to GCP Postgres usernames (Teleport Connect). [#59629](https://github.com/gravitational/teleport/pull/59629)
* Fixed `tsh play` not returning an error when playing a session fails. [#59625](https://github.com/gravitational/teleport/pull/59625)
* Fixed an issue in Teleport Connect where clicking 'Restart' to apply an update could close the window without actually restarting the app. [#59592](https://github.com/gravitational/teleport/pull/59592)
* Added automatic `@<project-id>.iam` suffix to GCP Postgres usernames (tsh, web UI). [#59590](https://github.com/gravitational/teleport/pull/59590)
* Introduced `application-proxy` service to `tbot` for HTTP proxying to applications protected by Teleport. [#59587](https://github.com/gravitational/teleport/pull/59587)
* MWI: Added support for customizing cluster names with a template to the `kubernetes/argo-cd` output. [#59575](https://github.com/gravitational/teleport/pull/59575)
* Fixed persistence of `metadata.description` field for the Bot resource. [#59570](https://github.com/gravitational/teleport/pull/59570)
* Fixed a crash in Teleport's Windows Desktop Service introduced in 18.2.0. Compaction of certain shared directory read/write audit events could result in a stack overflow error. [#59515](https://github.com/gravitational/teleport/pull/59515)
* Added `tctl tokens configure-kube` helper command to easily trust Kubernetes clusters and allow secure repeatable joining. [#59497](https://github.com/gravitational/teleport/pull/59497)
* Made the check for a running Teleport process in the install-node.sh script more robust. [#59496](https://github.com/gravitational/teleport/pull/59496)
* Fixed `tctl edit` producing an error when trying to modify a Bot resource. [#59480](https://github.com/gravitational/teleport/pull/59480)
* Added support for generating VSCode and Claude Code MCP servers configurations to the `tsh mcp config` and `tsh mcp db config` commands. [#59473](https://github.com/gravitational/teleport/pull/59473)
* Fixed a bug where session IDs were tied to the client connection, resulting in issues when combined with multiplexed connection features (OpenSSH ControlPath/ControlMaster/ControlPersist). [#59472](https://github.com/gravitational/teleport/pull/59472)
* Improved app access error messages in case of network error. [#59468](https://github.com/gravitational/teleport/pull/59468)
* Fixed database IAM configurator potentially getting stuck and never recovering (#59290). [#59417](https://github.com/gravitational/teleport/pull/59417)
* Added tbot copy-binaries command to simplify using tbot as a Kubernetes sidecar. [#59404](https://github.com/gravitational/teleport/pull/59404)
* Fixed `tsh config` binary path after managed updates. [#59384](https://github.com/gravitational/teleport/pull/59384)
* Updated Entra ID integration to support group filters. [#59378](https://github.com/gravitational/teleport/pull/59378)
* Fixed regression allowing SAML apps to be included when filtering resources by 'Applications' in the Web UI. [#59327](https://github.com/gravitational/teleport/pull/59327)
* Allow controlling the description of auto-discovered Kubernetes apps with an annotation. [#58817](https://github.com/gravitational/teleport/pull/58817)
* Fixed an issue that prevented connecting to agents over peered tunnels when proxy peering was enabled. [#59556](https://github.com/gravitational/teleport/pull/59556)

## 18.2.2 (09/19/25)

* Fixed a regression in Teleport Connect for Windows that caused the executable to be unsigned. [#59302](https://github.com/gravitational/teleport/pull/59302)
* Fixed an issue that prevented uploading encrypted recordings using the S3 session recording backend. [#59281](https://github.com/gravitational/teleport/pull/59281)
* Fix issue preventing auto enrollment of EKS clusters when using the Web UI. [#59272](https://github.com/gravitational/teleport/pull/59272)
* Terraform provider: Allow creating access lists without setting spec.grants. [#59217](https://github.com/gravitational/teleport/pull/59217)
* Fixes a panic that occurs when creating a Bound Keypair join token with the `spec.onboarding` field unset. [#59178](https://github.com/gravitational/teleport/pull/59178)
* Added desktop name for Windows Directory and Clipboard audit events. [#59146](https://github.com/gravitational/teleport/pull/59146)
* Added the ability to update the AWS Identity Center SCIM token in `tctl`. [#59114](https://github.com/gravitational/teleport/pull/59114)
* Added services to correctly choose Access Request roles in remote clusters. [#59062](https://github.com/gravitational/teleport/pull/59062)
* Install script allows specifying a group for agent installation with managed updates V2 enabled. [#59059](https://github.com/gravitational/teleport/pull/59059)
* Added support for ElastiCache Serverless for Redis OSS and Valkey database access. [#58891](https://github.com/gravitational/teleport/pull/58891)

Enterprise:
* Fixed an issue in the Entra ID integration where a user account with an unsupported username value could prevent other valid users and groups to be synced to Teleport. Such user accounts are now filtered.

## 18.2.1 (09/12/25)

* Fixed client tools managed updates sequential update. [#59086](https://github.com/gravitational/teleport/pull/59086)
* Fixed headless login so that it supports both WebAuthn and SSO for MFA. [#59078](https://github.com/gravitational/teleport/pull/59078)
* When selecting a login for an SSH server, Teleport Connect now shows only logins allowed by RBAC for that specific server rather than showing all logins which the user has access to. [#59067](https://github.com/gravitational/teleport/pull/59067)
* Terraform Provider is now supported on Windows machines. [#59055](https://github.com/gravitational/teleport/pull/59055)
* Enabled Oracle Cloud joining in Machine ID's `tbot` client. [#59040](https://github.com/gravitational/teleport/pull/59040)
* Fixed a bug preventing users to create access lists with empty grants through Terraform. [#59032](https://github.com/gravitational/teleport/pull/59032)
* Fixed a DynamoDB bug potentially causing event queries to return a different range of events. In the worst case scenario, this bug would block the event-handler. [#59029](https://github.com/gravitational/teleport/pull/59029)
* Fixed an issue where SSH file copying attempts would be spuriously denied in proxy recording mode. [#59027](https://github.com/gravitational/teleport/pull/59027)
* Updated Enroll Integration page design. [#58985](https://github.com/gravitational/teleport/pull/58985)
* Teleport Connect now runs in the background by default on macOS and Windows. On Linux, this behavior can be enabled in the app configuration. [#58923](https://github.com/gravitational/teleport/pull/58923)
* Added fdpass-teleport binary to install script for Teleport tar downloads. [#58919](https://github.com/gravitational/teleport/pull/58919)
* Support multiple resource editing in `tctl edit` when editing collections. [#58902](https://github.com/gravitational/teleport/pull/58902)
* Added support for browser window resizing to the Teleport Web UI database client terminal. [#58900](https://github.com/gravitational/teleport/pull/58900)
* Fixed a bug that prevented root users from viewing session recordings when they were participants. [#58897](https://github.com/gravitational/teleport/pull/58897)
* Added ability for user to select whether IC integration creates roles for all possible Account Assignments. [#58861](https://github.com/gravitational/teleport/pull/58861)
* Updated Go to 1.24.7. [#58835](https://github.com/gravitational/teleport/pull/58835)
* Populate `user_roles` and `user_traits` fields for SSH audit events. [#58804](https://github.com/gravitational/teleport/pull/58804)
* Added support for wtmpdb as a user accounting backend to wtmp. [#58777](https://github.com/gravitational/teleport/pull/58777)
* Prevents an application from being registered if its public address matches a Teleport cluster address. [#58766](https://github.com/gravitational/teleport/pull/58766)
* Added a preset role `mcp-user` that has access to all MCP servers and their tools. [#58613](https://github.com/gravitational/teleport/pull/58613)

Enterprise:
* Fixed an issue where sometimes the session summary was marked as a success, even though the summary was empty (this was particularly visible using GPT 5).
* Updated Enroll Integration page design.

## 18.2.0 (09/04/25)

### Encrypted session recordings

Teleport now provides the ability to integrate with Hardware Security Modules (HSMs) in order to encrypt session recordings prior to uploading them to storage.

### AI session summaries

Teleport Identity Security users are now able to view AI-generated summaries for SSH, Kubernetes and database sessions.

### Updated session recordings page

Session recordings page in Teleport web UI are now updated with a new design that will include session thumbnails and ability to view session summaries for Identity Security users.

### Teleport Connect Managed Updates

Teleport Connect is now able to detect when application updates are available and automatically apply them on the next restart.

### Teleport Device Trust Intune Support

Teleport now includes a new hosted plugin for Microsoft's Intune suite, allowing trusted devices to be synchronized from the Intune inventory.

### Terraform support for Access List members

Users are now able to provision Access Lists and their members (including other nested Access Lists) with terraform.

### Long-term access requests UX

Teleport access requests creation dialog in web UI now better differentiate between short and long-term access requests.

### Database web terminal for MySQL

Teleport web UI now provides terminal interface for MySQL database access.

### Database access for AlloyDB

Teleport now supports database access for GCP AlloyDB databases.

### Other changes and improvements

* Improved observability by adding health check metrics for healthy, unhealthy, and unknown states. Database health checks can now be monitored with these metrics. [#58708](https://github.com/gravitational/teleport/pull/58708)
* Removed AccessList review notification check from tsh login/status flow. [#58662](https://github.com/gravitational/teleport/pull/58662)
* Lock, unlock and delete from the Bot Details page, as well as viewing lock status. [#58653](https://github.com/gravitational/teleport/pull/58653)
* Fixed internal access list membership caching issue that caused high CPU usage when the total number of members exceeded 200. [#58614](https://github.com/gravitational/teleport/pull/58614)
* Fixed internal cache issue that could cause crashes in AWS IC, Database, and App access flows. [#58611](https://github.com/gravitational/teleport/pull/58611)
* Fixed panic in `tbot`'s `ssh-multiplexer` service. [#58595](https://github.com/gravitational/teleport/pull/58595)
* Teleport now honours Entra ID OIDC groups overage claim. The OIDC connector spec in Teleport must be updated to request OIDC `profile` scope and the enterprise application in Entra ID must be granted with `User.ReadBasic.All` Graph API permission for this feature to work. By default, Teleport will query the Microsoft Graph API `graph.microsoft.com` endpoint and filter user's group membership of "security groups" group type. This behaviour can be updated by configuring `entra_id_groups_provider` configuration field, which is available in the OIDC connector configuration spec. [#58593](https://github.com/gravitational/teleport/pull/58593)
* Enhanced session recordings RBAC to enforce recording access based on rules that reference creator’s roles, traits, and resource properties. [#58563](https://github.com/gravitational/teleport/pull/58563)
* Added support for configure SCIM Plugin with OIDC or Github Teleport Connectors. [#58554](https://github.com/gravitational/teleport/pull/58554)
* Added `user_agent` field to MySQL database session start audit events. [#58523](https://github.com/gravitational/teleport/pull/58523)
* `tbot` now supports the configuration of a default namespace for kubeconfig files generated by the `kubernetes/v2` service. [#58494](https://github.com/gravitational/teleport/pull/58494)
* Reduced audit log clutter by compacting contiguous shared directory read/write events into a single audit log event. [#58446](https://github.com/gravitational/teleport/pull/58446)
* Session metadata now appears next to SSH sessions in the UI. [#58405](https://github.com/gravitational/teleport/pull/58405)
* Refreshed the list session recordings UI with thumbnails, more filtering options and a card/list view. [#58390](https://github.com/gravitational/teleport/pull/58390)
* Added thumbnail and metadata generation for session recordings. [#58360](https://github.com/gravitational/teleport/pull/58360)
* Teleport Connect now supports managed updates. [#58260](https://github.com/gravitational/teleport/pull/58260)
* Teleport Connect now brings focus back from the browser to itself after a successful SSO login. [#58260](https://github.com/gravitational/teleport/pull/58260)
* Added support for GCP AlloyDB. [#58202](https://github.com/gravitational/teleport/pull/58202)
* Added support for encrypting session recordings at rest across all recording modes. Encryption can be enabled statically by setting `auth_server.session_recording_config.enabled: yes` in the Teleport file configuration, or dynamically by editing the `session_recording_config` resource and setting `spec.encryption.enabled: yes`. [#57959](https://github.com/gravitational/teleport/pull/57959)
* Added SSH SELinux module management to teleport-update. [#57660](https://github.com/gravitational/teleport/pull/57660)
* Added Terraform support for Access List members. [#57058](https://github.com/gravitational/teleport/pull/57058)

## 18.1.8 (08/29/25)

* Fixed an issue introduced in v18.1.5 that caused desktop connection attempts to stall on the loading screen. [#58500](https://github.com/gravitational/teleport/pull/58500)
* Support setting `"*"` in role `kubernetes_users`. [#58477](https://github.com/gravitational/teleport/pull/58477)
* The following Helm charts now support obtaining the plugin credentials using tbot: `teleport-plugin-discord`, `teleport-plugin-email`, `teleport-plugin-jira`, `teleport-plugin-mattermost`, `teleport-plugin-msteams`, `teleport-plugin-pagerduty`, `teleport-plugin-event-handler`. [#58301](https://github.com/gravitational/teleport/pull/58301)

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

