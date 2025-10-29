# Changelog

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
