# Changelog

## 17.7.16 (02/02/26)

* Improved robustness of the Slack hosted plugin to reduce the likeliness of failed token refresh when experiencing external disruption. [#63347](https://github.com/gravitational/teleport/pull/63347)
* Ensure application session rejections for untrusted devices are consistently audited as AppSessionStart failures after MFA. [#63260](https://github.com/gravitational/teleport/pull/63260)
* Fixed a `CredentialContainer` error when attempting to log in to the Web UI with a hardware key using Firefox >=147.0.2. [#63246](https://github.com/gravitational/teleport/pull/63246)
* Updated OpenSSL to 3.0.19. [#63203](https://github.com/gravitational/teleport/pull/63203)

Enterprise:
* Mitigated a race in the Slack token refresh logic.
* Fixe the issue where Okta integration may not remove previously synced apps after plugin restart.
* Added support for multi-arch lock file population for the terraform provider.


## 17.7.15 (01/26/26)

* Updated indirect dependency go-chi/chi/v5 (addresses GO-2026-4316). [#63093](https://github.com/gravitational/teleport/pull/63093)
* The `tbot systemd install` command now supports a `--pid-file` flag for setting the path to the PID file. [#63038](https://github.com/gravitational/teleport/pull/63038)
* Fixed GCS session recording backend not respecting rate limits. [#62987](https://github.com/gravitational/teleport/pull/62987)
* Made the teleport-cluster Helm chart job resources configurable again via the `jobResources` value. [#62924](https://github.com/gravitational/teleport/pull/62924)
* Reverted a disruptive change from v17.7.11: `teleport-cluster` Helm chart uses `resources` for Jobs again. If set `jobResources` takes precedence. This will change in v18, only `jobResources` will be used. [#62924](https://github.com/gravitational/teleport/pull/62924)


## 17.7.14 (01/06/26)

* Updated Go to 1.24.12. [#62886](https://github.com/gravitational/teleport/pull/62886)
* Fixed launching AWS Identity Center from Teleport Connect. [#62870](https://github.com/gravitational/teleport/pull/62870)
* Fixed renewed X509-SVIDs not being proactively sent to Envoy instances. [#62829](https://github.com/gravitational/teleport/pull/62829)
* Updated rustcrypto/rsa dependency to fix potential panic (CVE-2026-21895). [#62769](https://github.com/gravitational/teleport/pull/62769)
* Fixed an issue that would prevent tsh from working when the 1password SSH agent is running. [#62737](https://github.com/gravitational/teleport/pull/62737)

Enterprise:
* Fixed an issue in the Entra ID integration where a user account with an unsupported username character `/` could prevent other valid users and groups to be synced to Teleport. Such user accounts are now filtered.

## 17.7.13 (01/08/26)

* Fixed an issue where logging in to the Web UI with Device Trust would lose query params of the redirect URL. [#62678](https://github.com/gravitational/teleport/pull/62678)
* Fixed an issue where Teleport Connect could generate a flurry of notifications about not being able to connect to a resource. [#62672](https://github.com/gravitational/teleport/pull/62672)
* Fixed issuance of wildcard DNS SANs with Workload Identity. [#62669](https://github.com/gravitational/teleport/pull/62669)
* Added IAM joining support from new AWS regions in asia. [#62628](https://github.com/gravitational/teleport/pull/62628)
* Added cleanup of access entries for EKS auto-discovered clusters when they no longer match the filtering criteria and are removed. [#62599](https://github.com/gravitational/teleport/pull/62599)
* Fixed some auto update audit events showing up as unknown in the web UI. [#62548](https://github.com/gravitational/teleport/pull/62548)
* The join tokens UI now indicates which tokens are managed by the Teleport Cloud platform. [#62543](https://github.com/gravitational/teleport/pull/62543)
* Audit log and session uploader now respect region field of external_audit_storage resource when present. [#62519](https://github.com/gravitational/teleport/pull/62519)
* Fixed an issue that prevented searching for users by role in the web UI. [#62475](https://github.com/gravitational/teleport/pull/62475)
* Acknowledging a cluster alert no longer requires the create permission. [#62469](https://github.com/gravitational/teleport/pull/62469)
* Fixed tilde expansion for moderated SFTP. [#62454](https://github.com/gravitational/teleport/pull/62454)
* Fixed a potential SSRF vulnerability in the Azure join method implementation. [#62420](https://github.com/gravitational/teleport/pull/62420)
* Updated github.com/quic-go/quic-go to 0.57.0 to mitigate CVE-2025-64702. [#62294](https://github.com/gravitational/teleport/pull/62294)
* Fixed issue where AltGr key combinations did not work correctly in remote desktop sessions. [#62197](https://github.com/gravitational/teleport/pull/62197)
* Fixed a memory leak in access list reminder notifications affecting clusters with more than 1000 pending Access List reviews. [#62664](https://github.com/gravitational/teleport/pull/62664)


## 17.7.12 (12/15/25)

* Fixed an Auth Service bug causing the event handler to miss up to 1 event every 5 minutes when storing audit events in S3. [#62149](https://github.com/gravitational/teleport/pull/62149)
* Fixed bug where event handler dies on malformed session events. [#62142](https://github.com/gravitational/teleport/pull/62142)
* Changed "tsh --mfa-mode=cross-platform" to favor security keys on current Windows versions. [#62136](https://github.com/gravitational/teleport/pull/62136)
* Improved detail of error messages for `identity` service in `tbot`. [#62121](https://github.com/gravitational/teleport/pull/62121)
* Fixed bug where event handler `types` filter is ignored for Teleport clients using Athena storage backend. [#62083](https://github.com/gravitational/teleport/pull/62083)
* Fixed intermittent issues with VNet on Windows when other NRPT rules from GPOs are present under `HKLM:\SOFTWARE\Policies\Microsoft\Windows NT\DNSClient\DnsPolicyConfig`. [#62053](https://github.com/gravitational/teleport/pull/62053)

Enterprise:
* Fix a potential race where Okta assignments may never be cleaned up if the Okta integration is down while the assignment expires.
* Updated AWS Identity Center integration sign-in start URL format to support AWS GovCloud accounts.
* Added support for AWS Account name and ID labels (`teleport.dev/account-id`, `teleport.dev/account-name`) on AWS Identity Center resources (`aws_ic_account_assignment` and `aws_ic_account`). These labels improve compatibility with Access Monitoring Rules, allowing users to more easily target and audit AWS IC accounts.
* Prevented Trivy from reporting false positives when scanning the Teleport binaries.
* Added support for Jamf's /v2/computers-inventory API (addresses Jamf's deprecation of /v1/computers-inventory).
* Updated the AWS Identity Center resource synchronizer to handle AWS Account name changes more gracefully.

## 17.7.11 (12/08/25)

* Reduced memory consumption of the Application service. [#62013](https://github.com/gravitational/teleport/pull/62013)
* Prevented stuck `teleport-cluster` Helm chart rollouts in small Kubernetes clusters. Removed resource requests from configuration check hooks. [#62004](https://github.com/gravitational/teleport/pull/62004)
* Updated Go to 1.24.11. [#61954](https://github.com/gravitational/teleport/pull/61954)
* Updates `tsh workload-identity issue-x509` to automatically create the specified folder if it does not exist. [#61951](https://github.com/gravitational/teleport/pull/61951)
* Fixed a bug where JWT-SVID timestamp claims would be represented using scientific notation. [#61922](https://github.com/gravitational/teleport/pull/61922)
* Fixed a bug causing high memory consumption in the Teleport Auth Service when clients were listing large resources. [#61848](https://github.com/gravitational/teleport/pull/61848)
* Prevent data races when terminating interactive Kubernetes sessions. [#61822](https://github.com/gravitational/teleport/pull/61822)
* Fix `tsh db connect` failing to connect to databases using separate ports configuration (non-TLS routing mode). [#61811](https://github.com/gravitational/teleport/pull/61811)
* Fixed bug where Kubernetes App Discovery `poll_interval` is not set correctly. [#61792](https://github.com/gravitational/teleport/pull/61792)
* Fixed relative path evaluation for SFTP in proxy recording mode. [#61759](https://github.com/gravitational/teleport/pull/61759)
* Fixed `tsh kube ls` showing deleted clusters. [#61743](https://github.com/gravitational/teleport/pull/61743)
* Fixed workload identity templating to support certain numeric values that previously gave a "expression did not evaluate to a string" error. [#61739](https://github.com/gravitational/teleport/pull/61739)
* Fixed AWS Console access when using AWS IAM Roles Anywhere or AWS OIDC integrations, when IP Pinning is enabled. [#61655](https://github.com/gravitational/teleport/pull/61655)
* Added ability to update existing Azure OIDC integration with `tctl`. [#61593](https://github.com/gravitational/teleport/pull/61593)
* Prevented Trivy from reporting false positives when scanning the Teleport binaries. [#61540](https://github.com/gravitational/teleport/pull/61540)
* Updated tsh debug output to include tsh client version when --debug flag is set. [#61526](https://github.com/gravitational/teleport/pull/61526)
* Fixed web upload/download failure behind load balancers when web listen address is unspecified. [#61394](https://github.com/gravitational/teleport/pull/61394)
* Fixed corrupted private keys breaking tsh. [#61387](https://github.com/gravitational/teleport/pull/61387)
* Fix an issue connections to MongoDB Atlas clusters fail if clusters use certs signed by Google Trust Services (GTS). [#61325](https://github.com/gravitational/teleport/pull/61325)
* GOAWAY errors received from Kubernetes API Servers configured with a non-zero --goaway-chance are now forward to clients to be retried. [#61255](https://github.com/gravitational/teleport/pull/61255)
* Added a Workload Identities page to the web UI to list workload identities. [#59478](https://github.com/gravitational/teleport/pull/59478)

## 17.7.10 (11/13/25)

* Improved reverse tunnel dialing recovery from default route changes by 1min on average. [#61318](https://github.com/gravitational/teleport/pull/61318)
* Fixed an issue with the Identity Center resource cache that could cause the account resources to be deleted from the cache. [#61313](https://github.com/gravitational/teleport/pull/61313)
* Fixed an issue Postgres database cannot be accessed via Teleport Connect when per-session MFA is enabled and the role does not have wildcard `db_names`. [#61300](https://github.com/gravitational/teleport/pull/61300)
* Improved conflict detection of application public address and Teleport cluster addresses. [#61292](https://github.com/gravitational/teleport/pull/61292)
* Fixed rare error in the `authorized_keys` secret scanner when running the Teleport agent on MacOS. [#61267](https://github.com/gravitational/teleport/pull/61267)
* Updated Go to v1.24.10. [#61210](https://github.com/gravitational/teleport/pull/61210)
* Instrumented tbot to better support teleport-update. [#61190](https://github.com/gravitational/teleport/pull/61190)
* Improved error message of `tsh` when there is a certificate DNS SAN mismatch when connecting to Auth via Proxy. [#61187](https://github.com/gravitational/teleport/pull/61187)
* Improved error handling during desktop sessions that encounter unknown/invalid smartcard commands. This prevents abrupt desktop session termination with a "PDU error" message when using certain applications. [#61179](https://github.com/gravitational/teleport/pull/61179)
* Updated github.com/containerd/containerd dependency to fix https://github.com/advisories/GHSA-pwhc-rpq9-4c8w. [#61145](https://github.com/gravitational/teleport/pull/61145)
* Updated quic-go dependency to fix CVE-2025-59530. [#61111](https://github.com/gravitational/teleport/pull/61111)
* Fixed a bug causing `tsh` to stop waiting for access request approval and incorrectly report that the request had been deleted. [#61110](https://github.com/gravitational/teleport/pull/61110)
* Fixed an issue where resources in Teleport Connect were not always refreshed correctly after re-logging in as a different user. [#61100](https://github.com/gravitational/teleport/pull/61100)
* Fixed an issue which could lead to session recordings saved on disk being truncated. [#60965](https://github.com/gravitational/teleport/pull/60965)

## 17.7.9 (11/05/25)

* Fixed configuration files such as `.kube/config` referring to non-existent `tsh` binaries. [#60872](https://github.com/gravitational/teleport/pull/60872)
* Fixed an issue in the web UI where a bot with zero tokens would show a validation error. [#60759](https://github.com/gravitational/teleport/pull/60759)
* The browser window for SSO MFA is slightly taller in order to accommodate larger elements like QR codes. [#60702](https://github.com/gravitational/teleport/pull/60702)
* Fixed MongoDB topology monitoring connection leak in the Teleport Database Service. [#60693](https://github.com/gravitational/teleport/pull/60693)
* Okta-managed apps are now pinned correctly in the web UI. [#60677](https://github.com/gravitational/teleport/pull/60677)
* Slack access plugin no longer crashes in the event access list is unsupported. [#60674](https://github.com/gravitational/teleport/pull/60674)
* Fixed tsh scp failing on files that grow during transfer. [#60608](https://github.com/gravitational/teleport/pull/60608)
* Allowed moderated session peers to perform file transfers. [#60605](https://github.com/gravitational/teleport/pull/60605)
* Fixed a startup error `EADDRINUSE: address already in use` in Teleport Connect on macOS and Linux that could occur with long system usernames. [#60577](https://github.com/gravitational/teleport/pull/60577)
* MWI: `tbot`'s auto-generated service names are now simpler and easier to use in the `/readyz` endpoint. [#60459](https://github.com/gravitational/teleport/pull/60459)
* Client tools managed updates stores OS and ARCH in the configuration. This ensures compatibility when `TELEPORT_HOME` directory is shared with a virtual instance running a different OS or architecture. [#60413](https://github.com/gravitational/teleport/pull/60413)
* Updated LDAP dial timeout from 15 seconds to 30 seconds. [#60392](https://github.com/gravitational/teleport/pull/60392)
* Fixed a bug that prevented using database role names longer than 30 chars for MySQL auto user provisioning. Now role names as long as 32 chars, which is the MySQL limit, can be used. [#60378](https://github.com/gravitational/teleport/pull/60378)
* Fixed a bug in Proxy Recording Mode that causes SSH sessions in the WebUI to fail. [#60368](https://github.com/gravitational/teleport/pull/60368)
* Added `extraEnv` and `extraArgs` to the teleport-operator helm chart. [#60356](https://github.com/gravitational/teleport/pull/60356)
* Fixed malformed audit events breaking the audit log. [#60335](https://github.com/gravitational/teleport/pull/60335)
* Added editing bot description to the web UI. [#60213](https://github.com/gravitational/teleport/pull/60213)

## 17.7.8 (10/15/25)

* Updated error messages displayed by `tsh ssh` when access to hosts is denied and when attempting to connect to a host that is offline or not enrolled in the cluster. [#60226](https://github.com/gravitational/teleport/pull/60226)
* Fixed an issue in Teleport Connect where Ctrl+D would sometimes not close a terminal tab. [#60222](https://github.com/gravitational/teleport/pull/60222)
* Added support for PodSecurityContext to `tbot` helm chart. [#60207](https://github.com/gravitational/teleport/pull/60207)
* MWI: Add `teleport_bot_instances` metric. [#60205](https://github.com/gravitational/teleport/pull/60205)
* The `tbot` Workload API now logs errors encountered when handling requests. [#60192](https://github.com/gravitational/teleport/pull/60192)
* Added explicit timeout to tbot when the Trust Bundle Cache is establishing an event watch. [#60187](https://github.com/gravitational/teleport/pull/60187)
* Fixed a bug where OpenSSH EICE node connections would fail. [#60125](https://github.com/gravitational/teleport/pull/60125)
* Updated Go to 1.24.9. [#60114](https://github.com/gravitational/teleport/pull/60114)
* Fixed SFTP audit events breaking the audit log. [#60070](https://github.com/gravitational/teleport/pull/60070)
* Fixed excessive memory usage on Teleport Proxy Service instances when using the the Teleport Web UI PostgreSQL REPL. [#60001](https://github.com/gravitational/teleport/pull/60001)
* Fixed `tsh scp` getting stuck in symlink loops. [#59995](https://github.com/gravitational/teleport/pull/59995)
* Fixed handling of local `tsh scp` targets that contain a colon. [#59982](https://github.com/gravitational/teleport/pull/59982)
* Fixed issue where temporarily unreachable app servers were permanently removed from session cache, causing persistent connection failures: `no application servers remaining to connect`. [#59955](https://github.com/gravitational/teleport/pull/59955)
* Fixed the issue with automatic access requests for `tsh ssh` when `spec.allow.request.max_duration` is set on the requester role. [#59925](https://github.com/gravitational/teleport/pull/59925)
* Fixes a bug with the check for a running Teleport process in the install-node.sh script. [#59888](https://github.com/gravitational/teleport/pull/59888)
* MWI: The `kubernetes/v2` output now supports customizing context names with a template. [#59740](https://github.com/gravitational/teleport/pull/59740)
* Updated mongo-driver to v1.17.4 to include fixes for possible connection leaks that could affect Teleport Database Service instances. [#59733](https://github.com/gravitational/teleport/pull/59733)
* The event-handler plugin will now skip over Windows desktop session recording events by default. [#59682](https://github.com/gravitational/teleport/pull/59682)
* MWI: The `kubernetes/argo-cd` output now supports customizing cluster names with a template. [#59576](https://github.com/gravitational/teleport/pull/59576)

## 17.7.7 (09/29/25)

* Fixed auto-approvals in the Datadog Incident Management integration by updating the on-call API client. [#59669](https://github.com/gravitational/teleport/pull/59669)
* Fixed auto-approvals in the Datadog Incident Management integration to ignore case sensitivity in user emails. [#59669](https://github.com/gravitational/teleport/pull/59669)
* Fixed `tsh play` not returning an error when playing a session fails. [#59626](https://github.com/gravitational/teleport/pull/59626)
* Fixed an issue in Teleport Connect where clicking 'Restart' to apply an update could close the window without actually restarting the app. [#59593](https://github.com/gravitational/teleport/pull/59593)
* Introduced `application-proxy` service to `tbot` for HTTP proxying to applications protected by Teleport. [#59588](https://github.com/gravitational/teleport/pull/59588)
* Fixed persistence of `metadata.description` field for the Bot resource. [#59571](https://github.com/gravitational/teleport/pull/59571)
* Fixed a crash in Teleport's Windows Desktop Service introduced in 17.7.3. Compaction of certain shared directory read/write audit events could result in a stack overflow error. [#59514](https://github.com/gravitational/teleport/pull/59514)
* Enabled Oracle Cloud joining in Machine ID's `tbot` client. [#59041](https://github.com/gravitational/teleport/pull/59041)
* Fixed an issue that prevented connecting to agents over peered tunnels when proxy peering was enabled. [#59557](https://github.com/gravitational/teleport/pull/59557)

## 17.7.6 (09/23/25)

* Made the check for a running Teleport process in the install-node.sh script more robust. [#59495](https://github.com/gravitational/teleport/pull/59495)
* Fixed `tctl edit` producing an error when trying to modify a Bot resource. [#59481](https://github.com/gravitational/teleport/pull/59481)
* Improved app access error messages in case of network error. [#59467](https://github.com/gravitational/teleport/pull/59467)
* Fixed database IAM configurator potentially getting stuck and never recovering (#59290). [#59418](https://github.com/gravitational/teleport/pull/59418)
* Fixed `tsh config` binary path after managed updates. [#59385](https://github.com/gravitational/teleport/pull/59385)

## 17.7.5 (09/18/25)

* Fix issue preventing auto enrollment of EKS clusters when using the Web UI. [#59273](https://github.com/gravitational/teleport/pull/59273)
* Terraform provider: Allow creating access lists without setting spec.grants. [#59238](https://github.com/gravitational/teleport/pull/59238)
* Fixes a panic that occurs when creating a Bound Keypair join token with the `spec.onboarding` field unset. [#59179](https://github.com/gravitational/teleport/pull/59179)
* Added desktop name for Windows Directory and Clipboard audit events. [#59154](https://github.com/gravitational/teleport/pull/59154)
* Added the ability to update the AWS Identity Center SCIM token in tctl. [#59115](https://github.com/gravitational/teleport/pull/59115)
* Fixed client tools managed updates sequential update. [#59089](https://github.com/gravitational/teleport/pull/59089)
* Fixed headless login so that it supports both WebAuthn and SSO for MFA. [#59077](https://github.com/gravitational/teleport/pull/59077)
* When selecting a login for an SSH server, Teleport Connect now shows only logins allowed by RBAC for that specific server rather than showing all logins which the user has access to. [#59068](https://github.com/gravitational/teleport/pull/59068)
* Added services to correctly choose Access Request roles in remote clusters. [#59063](https://github.com/gravitational/teleport/pull/59063)
* Install script allows specifying a group for agent installation with managed updates V2 enabled. [#59060](https://github.com/gravitational/teleport/pull/59060)
* Fixed a bug preventing users to create access lists with empty grants through Terraform. [#59031](https://github.com/gravitational/teleport/pull/59031)
* Fixed a DynamoDB bug potentially causing event queries to return a different range of events. In the worst case scenario, this bug would block the event-handler. [#59030](https://github.com/gravitational/teleport/pull/59030)
* Teleport Connect now runs in the background by default on macOS and Windows. On Linux, this behavior can be enabled in the app configuration. [#58924](https://github.com/gravitational/teleport/pull/58924)
* Added fdpass-teleport binary to install script for Teleport tar downloads. [#58920](https://github.com/gravitational/teleport/pull/58920)
* Support multiple resource editing in `tctl edit` when editing collections. [#58901](https://github.com/gravitational/teleport/pull/58901)
* Fixed an issue that would cause trusted cluster resource updates to fail silently. [#58887](https://github.com/gravitational/teleport/pull/58887)
* Added ability for user to select whether IC integration creates roles for all possible Account Assignments. [#58862](https://github.com/gravitational/teleport/pull/58862)
* Allow controlling the description of auto-discovered Kubernetes apps with an annotation. [#58816](https://github.com/gravitational/teleport/pull/58816)
* Added new bound_keypair join method for Machine and Workload ID to better support bots in on-prem and other environments without a platform-specific join method. [#58334](https://github.com/gravitational/teleport/pull/58334)

Enterprise:
* Fixed an issue in the Entra ID integration where a user account with an unsupported username value could prevent other valid users and groups to be synced to Teleport. Such user accounts are now filtered.

## 17.7.4 (09/08/25)

* Updated Go to 1.24.7. [#58836](https://github.com/gravitational/teleport/pull/58836)
* Added support for `tbot` configuration of a default namespace for kubeconfig files generated by the kubernetes/v2 service. [#58791](https://github.com/gravitational/teleport/pull/58791)
* Prevented an application from being registered if its public address matches a Teleport cluster address. [#58767](https://github.com/gravitational/teleport/pull/58767)
* Removed AccessList review notification check from `tsh login` / `status` flow. [#58666](https://github.com/gravitational/teleport/pull/58666)
* Added Lock, unlock and delete operations to the Bot Details page, as well as viewing lock status. [#58647](https://github.com/gravitational/teleport/pull/58647)
* Fixed panic in `tbot`'s `ssh-multiplexer` service. [#58596](https://github.com/gravitational/teleport/pull/58596)
* MWI: Added support to `tbot` for managing Argo CD clusters via the `kubernetes/argo-cd` output service. [#58567](https://github.com/gravitational/teleport/pull/58567)
* Added support for configure SCIM Plugin with OIDC or Github Teleport Connectors. [#58555](https://github.com/gravitational/teleport/pull/58555)
* Appended headers to configuration files generated by `teleport-update`. [#56578](https://github.com/gravitational/teleport/pull/56578)

Enterprise:
* Updated AWS Identity Center plugin to honor Role and Access Request locks.
* Updated AWS Identity Center plugin to not provision users when Teleport is not acting as a SAML IdP for AWS.

## 17.7.3 (09/02/25)

* Aa namespace can now be specified for the `tbot` Kubernetes Secret destination. [#58553](https://github.com/gravitational/teleport/pull/58553)
* Fixed nested access list hierarchy propagation in case of `tctl` using UpsertAccessList API call. [#58550](https://github.com/gravitational/teleport/pull/58550)
* Added support for setting `"*"` in role `kubernetes_users`. [#58478](https://github.com/gravitational/teleport/pull/58478)
* Reduced audit log clutter by compacting contiguous shared directory read/write events into a single audit log event. [#58445](https://github.com/gravitational/teleport/pull/58445)
* Fixed an issue where VNet could not start because of "VNet is already running" error. [#58389](https://github.com/gravitational/teleport/pull/58389)
* Fixed incorrect scp exit status between OpenSSH clients and servers. [#58328](https://github.com/gravitational/teleport/pull/58328)
* Fixed sftp readdir failing due to broken symlinks. [#58321](https://github.com/gravitational/teleport/pull/58321)
* The following Helm charts now support obtaining the plugin credentials using `tbot`: `teleport-plugin-discord`, `teleport-plugin-email`, `teleport-plugin-jira`, `teleport-plugin-mattermost`, `teleport-plugin-msteams`, `teleport-plugin-pagerduty`, `teleport-plugin-event-handler`. [#58300](https://github.com/gravitational/teleport/pull/58300)
* Enabled separate request_object_mode setting for MFA flow in OIDC connectors. [#58280](https://github.com/gravitational/teleport/pull/58280)
* Teleport Connect now supports managed updates. [#58261](https://github.com/gravitational/teleport/pull/58261)
* Teleport Connect now brings focus back from the browser to itself after a successful SSO login. [#58261](https://github.com/gravitational/teleport/pull/58261)
* Fixed failure to close user accounting session. [#58164](https://github.com/gravitational/teleport/pull/58164)
* Fixed an uncaught exception in Teleport Connect on Windows when closing the app while the `TELEPORT_TOOLS_VERSION` environment variable is set. [#58132](https://github.com/gravitational/teleport/pull/58132)
* Fixed a Teleport Connect crash that occurred when assuming an access request while an application or database connection was active. [#58110](https://github.com/gravitational/teleport/pull/58110)
* Added paginated API ListDatabases, deprecate GetDatabases. [#58104](https://github.com/gravitational/teleport/pull/58104)
* Fixed modifier keys getting stuck during remote desktop sessions. [#58102](https://github.com/gravitational/teleport/pull/58102)
* Enable Azure joining with VMSS. [#58093](https://github.com/gravitational/teleport/pull/58093)
* Windows desktop LDAP discovery now auto-populates the resource's description field. [#58081](https://github.com/gravitational/teleport/pull/58081)
* TBot now emits a log message stating the current version on startup. [#58057](https://github.com/gravitational/teleport/pull/58057)
* Added experimental bound keypair joining method, disabled by default behind a flag. [#57961](https://github.com/gravitational/teleport/pull/57961)
* Updated Go to 1.24.6. [#57860](https://github.com/gravitational/teleport/pull/57860)
* Added new `oidc` joining mode for Kubernetes delegated joining to support providers that can be configured to provide public OIDC endpoints, like EKS, AKS, and GKE. [#57800](https://github.com/gravitational/teleport/pull/57800)
* Newly enrolled Kubernetes agents in will now use Managed Updates by default. [#57783](https://github.com/gravitational/teleport/pull/57783)

Enterprise:
* For OIDC SSO, the IdP app/client configured for MFA checks is no longer expected to return claims that map to Teleport roles. Valid claim to role mappings are only required for login flows.
* Fixed SSO MFA method for applications when Teleport is the SAML identity provider and Per-Session MFA is enabled.
* Fix: Handle disabling okta-requester role assignment.

## 17.7.2 (08/18/25)

* Fixed an issue that could cause some hosts not to register dynamic Windows desktops. [#58062](https://github.com/gravitational/teleport/pull/58062)
* Improve error message when a User without any MFA devices enrolled attempts to access a resource that requires MFA. [#58044](https://github.com/gravitational/teleport/pull/58044)
* Add TELEPORT_UNSTABLE_GRPC_RECV_SIZE env var which can be set to overwrite client side max grpc message size. [#58028](https://github.com/gravitational/teleport/pull/58028)
* Add support for JWT-Secured Authorization Requests to OIDC Connector. [#58013](https://github.com/gravitational/teleport/pull/58013)
* Fixed an issue that could cause revocation checks to fail in Windows environments. [#57879](https://github.com/gravitational/teleport/pull/57879)
* Fixed the case where the auto-updated client tools did not use the intended version. [#57871](https://github.com/gravitational/teleport/pull/57871)
* Fix database PKINIT issues caused missing CDP information in the certificate. [#57851](https://github.com/gravitational/teleport/pull/57851)
* Device Trust: added `required-for-humans` mode to allow bots to run on unenrolled devices, while enforcing checks for human users. [#57845](https://github.com/gravitational/teleport/pull/57845)
* Updated Go to 1.23.12. [#57765](https://github.com/gravitational/teleport/pull/57765)
* Added the `--auth` flag to the `tctl plugins install scim` CLI command to support Bearer token and OAuth authentication methods. [#57758](https://github.com/gravitational/teleport/pull/57758)
* Fix Alt+Click not being registered in remote desktop sessions. [#57756](https://github.com/gravitational/teleport/pull/57756)
* Kubernetes Access: `kubectl port-forward` now exits cleanly when backend pods are removed. [#57742](https://github.com/gravitational/teleport/pull/57742)
* Kubernetes Access: Fixed a bug when forwarding multiple ports to a single pod. [#57737](https://github.com/gravitational/teleport/pull/57737)
* Fixed unlink-package during upgrade/downgrade. [#57721](https://github.com/gravitational/teleport/pull/57721)
* Teleport `event-handler` now accepts HTTP Status Code 204 from the recipient. This adds support for sending events to Grafana Alloy and newer Fluentd versions. [#57681](https://github.com/gravitational/teleport/pull/57681)
* Enrich the windows.desktop.session.start audit event with additional certificate metadata. [#57678](https://github.com/gravitational/teleport/pull/57678)
* Added `--force` option to `tctl workload-identity x509-issuer-overrides sign-csrs` to allow displaying the output of partial failures, intended for use in clusters that make use of HSMs. [#57661](https://github.com/gravitational/teleport/pull/57661)
* Tctl top can now display raw prometheus metrics. [#57634](https://github.com/gravitational/teleport/pull/57634)
* Fixed access denied error messages not being displayed in the Teleport web UI PostgreSQL client. [#57569](https://github.com/gravitational/teleport/pull/57569)
* Use the bot details page to view and edit bot configuration, and see active instances with their upgrade status. [#57543](https://github.com/gravitational/teleport/pull/57543)
* Fix a bug in the default discovery script that can happen discovering instances whose PATH doesn't contain `/usr/local/bin`. [#57531](https://github.com/gravitational/teleport/pull/57531)
* Fix a race condition in the Terraform Provider potentially causing "does not exist" errors the following resources: `auth_preference`, `autoupdate_config`, `autoupdate_version`, `cluster_maintenance_config`, `cluster_network_config`, and `session_recording_config`. [#57528](https://github.com/gravitational/teleport/pull/57528)
* Fix a Terraform provider bug causing resource creation to be retried more times than the MaxRetries setting. [#57528](https://github.com/gravitational/teleport/pull/57528)
* Make it easier to identify Windows desktop certificate issuance on the audit log page. [#57520](https://github.com/gravitational/teleport/pull/57520)
* Fix a bug in the TF provider happening when `autoupdate_version` or `autoupdate_config` have non-empty metadata. [#57517](https://github.com/gravitational/teleport/pull/57517)
* Fix a bug on Windows where a forwarded SSH agent would become dysfunctional after a single connection using the agent. [#57512](https://github.com/gravitational/teleport/pull/57512)
* Machine and Workload ID: Add experimental implementation of new `bound_keypair` join method for improved bot joining in on-prem environments. [#55037](https://github.com/gravitational/teleport/pull/55037)

## 17.7.1 (08/01/25)

* Fixed usage print for global `--help` flag. [#57452](https://github.com/gravitational/teleport/pull/57452)
* Tctl top respects local teleport config file. [#57353](https://github.com/gravitational/teleport/pull/57353)
* Fixed an issue backfilling CRLs during startup for long-standing clusters. [#57322](https://github.com/gravitational/teleport/pull/57322)
* Disable NLA in FIPS mode. [#57308](https://github.com/gravitational/teleport/pull/57308)

Enterprise:
* Slightly optimized access token refresh logic for Jamf integration when using API credentials.

## 17.7.0 (07/28/25)

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

### Other fixes and improvements

* Allow YubiKeys running 5.7.4+ firmware to be usable as PIV hardware keys. [#57217](https://github.com/gravitational/teleport/pull/57217)
* Tctl will now warn the user when importing a SPIFFE issuer override chain that contains the root CA. [#57168](https://github.com/gravitational/teleport/pull/57168)
* Fixed fallback for web login when second factor is set to `on` but only OTP is configured. [#57159](https://github.com/gravitational/teleport/pull/57159)
* Fix a bug causing `tctl`/`tsh` to fail on read-only file systems. [#57148](https://github.com/gravitational/teleport/pull/57148)
* The `teleport-distroless` container image now disables client tools updates by default (when using tsh/tctl, you will always use the version from the image). You can enable them back by unsetting the `TELEPORT_TOOLS_VERSION` environment variable. [#57148](https://github.com/gravitational/teleport/pull/57148)
* Fixed a crash in Teleport Connect that could occur when copying large clipboard content during desktop sessions. [#57131](https://github.com/gravitational/teleport/pull/57131)
* Audit log events for SPIFFE SVID issuances now include the name/label selector used by the client. [#57128](https://github.com/gravitational/teleport/pull/57128)
* Fixed client tools managed updates downgrade to older version. [#57111](https://github.com/gravitational/teleport/pull/57111)
* Removed unnecessary macOS entitlements from Teleport Connect subprocesses. [#57067](https://github.com/gravitational/teleport/pull/57067)
* Machine and Workload ID: The `tbot` client will now discard expired identities if needed during renewal to allow automatic recovery without restarting the process. [#57062](https://github.com/gravitational/teleport/pull/57062)
* Define access-plugin preset role. [#57057](https://github.com/gravitational/teleport/pull/57057)
* Resolved an issue where RemoteCluster objects stored in the cache had incorrect revisions, causing Update calls to fail. [#56974](https://github.com/gravitational/teleport/pull/56974)
* Update Application APIs to use pagination to avoid exceeding message size limitations. [#56949](https://github.com/gravitational/teleport/pull/56949)
* Fix certificate revocation failures in Active Directory environments when Teleport is using HSM-backed key material. [#56928](https://github.com/gravitational/teleport/pull/56928)

Enterprise:

* Fix SCIM user provisioning when a user already exists and is managed by the same connector as the SCIM integration.
* Fix SCIM integration front-end enroll flow.

## 17.6.0 (07/22/25)

## VNet for SSH

Teleport VNet now has native support for SSH, enabling any SSH client to connect to Teleport SSH servers with zero configuration. Advanced Teleport features like per-session MFA now have first-class support for a seamless user experience. [#55313](https://github.com/gravitational/teleport/pull/55313)

### Other fixes and improvements

* `tctl` top now supports the local unix sock debug endpoint. [#57027](https://github.com/gravitational/teleport/pull/57027)
* Added support to `tsh` App Access commands for Azure CLI (`az`) version `2.73.0` and newer. [#56951](https://github.com/gravitational/teleport/pull/56951)
* Fixed a bug in the Teleport install scripts when running on MacOS. The install scripts now error instead of trying to install non existing MacOS FIPS binaries. [#56942](https://github.com/gravitational/teleport/pull/56942)
* Fixed using relative path `TELEPORT_HOME` env with client tools managed update. [#56934](https://github.com/gravitational/teleport/pull/56934)
* Client tools managed updates support multi-cluster environments and track each version in the configuration file. [#56934](https://github.com/gravitational/teleport/pull/56934)

## 17.5.6 (07/17/25)

* Fix backward compatibility issue introduced in the 17.5.5 / 18.0.1 release related to Access List type, causing the `unknown access_list type "dynamic"` validation error. [#56888](https://github.com/gravitational/teleport/pull/56888)
* Added support for glob-style matching to Spacelift join rules. [#56878](https://github.com/gravitational/teleport/pull/56878)
* Improve PKINIT compatibility by always including CDP information in the certificate. [#56876](https://github.com/gravitational/teleport/pull/56876)

## 17.5.5 (07/15/25)

* Fixed backward compatibility for Access List 'membershipRequires is missing' for older terraform providers. [#56743](https://github.com/gravitational/teleport/pull/56743)
* Fixed VNet DNS configuration on Windows hosts joined to Active Directory domains. [#56739](https://github.com/gravitational/teleport/pull/56739)
* Updated default client timeout and upload rate for Pyroscope. [#56731](https://github.com/gravitational/teleport/pull/56731)
* Bot instances are now sortable by latest heartbeat time in the web UI. [#56685](https://github.com/gravitational/teleport/pull/56685)
* Updated Go to 1.23.11. [#56680](https://github.com/gravitational/teleport/pull/56680)
* Fixed `tbot` SPIFFE Workload API failing to renew SPIFFE SVIDs. [#56663](https://github.com/gravitational/teleport/pull/56663)
* Fixed some icons displaying as white/black blocks. [#56620](https://github.com/gravitational/teleport/pull/56620)
* Terraform Provider: add support for skipping proxy certificate verification in development environments. [#56530](https://github.com/gravitational/teleport/pull/56530)
* Made VNet DNS available over IPv4. [#56476](https://github.com/gravitational/teleport/pull/56476)
* Fixed `heartbeat_connections_received_total` undercounting Database and Kubernetes heartbeats by 1. [#54726](https://github.com/gravitational/teleport/pull/54726)

Enterprise:
* Added enrolment for a generic SCIM Integration.
* Fixed a email integration enrollment documentation link.

## 17.5.4 (07/02/25)

* Fixes broken `tbot` joining in the Terraform provider. [#56343](https://github.com/gravitational/teleport/pull/56343)
* Machine and Workload Identity: tbot's `/readyz` endpoint is now representative of the bot's health. [#56306](https://github.com/gravitational/teleport/pull/56306)
* Machine and Workload Identity: service names used in tbot's logs and `/readyz` endpoint can now be overridden. [#56306](https://github.com/gravitational/teleport/pull/56306)
* Resolved an issue where directory sharing could become unavailable after sharing a directory, disconnecting the desktop session, and reconnecting again. [#56275](https://github.com/gravitational/teleport/pull/56275)

## 17.5.3 (06/30/25)

### Security fixes

This release also includes fixes for the following security issues:

#### [Critical] Remote authentication bypass

* Removed special handling for `*ssh.Certificate` authorities in the `IsHostAuthority` and `IsUserAuthority` callbacks used by `x/crypto/ssh.CertChecker`. [#56252](https://github.com/gravitational/teleport/pull/56252)

Resolved an issue that allowed remote SSH authentication bypass on servers with Teleport SSH agents, OpenSSH-integrated deployments and Teleport Git proxy deployments.  [CVE-2025-49825](https://github.com/gravitational/teleport/security/advisories/GHSA-8cqv-pj7f-pwpc). Refer to the [RCA](https://trust.goteleport.com/resources?s=32t147ja8aawd6px7irxat&name=cve-2025-49825-rca) for the full details.

### Other fixes and improvements

* Fixed duplicated entries in `tctl inventory list` when using DynamoDB as cluster state storage. [#56182](https://github.com/gravitational/teleport/pull/56182)
* Fixed an issue that prevented deletion of an integration resource if AWS Identity Center plugin was installed in the Teleport cluster. [#56173](https://github.com/gravitational/teleport/pull/56173)
* Updated WindowsDesktop and WindowsDesktopService APIs to use pagination to avoid exceeding message size limitations. [#56155](https://github.com/gravitational/teleport/pull/56155)
* Fixed users not being redirected back to the login page when their session expires. [#56152](https://github.com/gravitational/teleport/pull/56152)
* Fixed error on setting up Teleport Discovery Service step of the EC2 SSM web UI flow when admin action is enabled (webauthn). [#56145](https://github.com/gravitational/teleport/pull/56145)
* Fixed Hardware Key Support for YubiKey firmware versions 5.7.x. [#56107](https://github.com/gravitational/teleport/pull/56107)
* Added SSO MFA support for desktop access. [#56058](https://github.com/gravitational/teleport/pull/56058)
* Fixed an issue that could prevent Windows desktop sessions from terminating when the idle timeout was exceeded. [#56048](https://github.com/gravitational/teleport/pull/56048)
* Added the `teleport-update status --is-up-to-date` flag to change the return code based on the update status. [#55950](https://github.com/gravitational/teleport/pull/55950)
* Added fork after authentication to `tsh ssh`. [#55894](https://github.com/gravitational/teleport/pull/55894)
* Fixed error when creating or updating join tokens in the web UI when admin action is enabled (second_factor set to webauthn). [#55832](https://github.com/gravitational/teleport/pull/55832)
* Machine and Workload Identity: `tbot` no longer supports providing a proxy server address via `--auth-server` or `auth_server`, use `--proxy-server` or `proxy_server` instead. [#55820](https://github.com/gravitational/teleport/pull/55820)
* Machine and Workload Identity: `tbot` will keep retrying if the auth server is unavailable on startup, instead of exiting immediately. [#55820](https://github.com/gravitational/teleport/pull/55820)
* Fixed a memory leak in Kubernetes Access caused by resources not being cleaned up when clients terminate watch streams. [#55767](https://github.com/gravitational/teleport/pull/55767)
* Added support for `tsh db exec` which executes commands across multiple target databases. When per-session MFA is required, only one MFA prompt is needed within a 5-minute window. [#55736](https://github.com/gravitational/teleport/pull/55736)
* Fixed an issue where the output from `tctl sso configure github` could not be used with `tctl create -f` in OSS Teleport. [#55727](https://github.com/gravitational/teleport/pull/55727)
* Fixed a bug that could cause Kubernetes exec requests to fail when the Kubernetes cluster had the WebSocket-based exec protocol disabled. [#55722](https://github.com/gravitational/teleport/pull/55722)
* Fixed an issue that prevented changes to default shell from propagating for host users and static host users. [#55650](https://github.com/gravitational/teleport/pull/55650)
* Updated Go to 1.23.10. [#55602](https://github.com/gravitational/teleport/pull/55602)
* User experience: Forbid creating Access Requests to user_group resources when Okta bidirectional sync is disabled. [#55586](https://github.com/gravitational/teleport/pull/55586)
* Teleport Connect: Add support for custom reason prompts. [#55584](https://github.com/gravitational/teleport/pull/55584)
* Fixed database connect options dialog displaying wrong database username options. [#55559](https://github.com/gravitational/teleport/pull/55559)
* Fixed updating the default PIN and PUK for hardware key support in Teleport Connect. [#55508](https://github.com/gravitational/teleport/pull/55508)
* The `tbot` client now ensures the `O_CLOEXEC` flag is used when opening files on Linux hosts. [#55503](https://github.com/gravitational/teleport/pull/55503)
* Fixed a bug that caused clipboard and directory sharing to remain unavailable when the initial desktop connection failed. [#55454](https://github.com/gravitational/teleport/pull/55454)
* The Windows installer of Teleport Connect now adds the folder with tsh to the system path rather than the user path. [#55449](https://github.com/gravitational/teleport/pull/55449)
* Added support for AWS KMS multi-region keys with key replication. [#55212](https://github.com/gravitational/teleport/pull/55212)
* Database protocols using Kerberos (SQL Server, Oracle) can now be configured to fetch user SID for Full Enforcement mapping. [#54870](https://github.com/gravitational/teleport/pull/54870)

Enterprise:
* Added support for Oracle SCAN (Single Client Access Name). [#6751](https://github.com/gravitational/teleport.e/pull/6751)
* Okta: Fixed disabling user sync in the existing plugin while bidirectional sync is enabled (the default). [#6669](https://github.com/gravitational/teleport.e/pull/6669)
* Okta: Fixed syncing back RBAC changes to Okta for legacy App and Group only sync configuration where Access List sync is disabled. [#6634](https://github.com/gravitational/teleport.e/pull/6634)
* Added support for viewing and exploring "active" bot instances via the web UI. [#6612](https://github.com/gravitational/teleport.e/pull/6612)

## 17.5.1 (06/04/25)

Rerelease of 17.5.0 due to some build issues.

### Azure Console via SAML IdP
Teleport SAML IdP now supports Azure web console as a service provider.

### Desktop Access in Teleport Connect
Teleport Connect now allows users to connect to Windows desktops directly from the Teleport Connect application without needing to use a browser.

### Desktop Access latency detector
Teleport's web UI now shows latency measurements during remote desktop sessions which indicate both the latency between the user and the Teleport proxy as well as the latency between the Teleport proxy and the target host.

### Machine & Workload Identity - Sigstore attestation
Machine & Workload Identity now supports attesting Sigstore signatures of workloads running on Docker, Podman and Kubernetes. This allows the issuance of credentials to be restricted to workloads with container images produced by legitimate CI/CD systems.

### Azure DevOps joining
Teleport now supports secretless authentication for Bots running within Azure DevOps pipelines.

### Security fixes

This release also includes fixes for the following security issues.
These issues are present in previous v17 releases.
Impacted users are recommended to upgrade their auth and proxy servers to the latest version.

#### [High] Unauthorized deletion in AWS IAM Identity Center integration

* Fixed an issue that allowed unauthenticated access to delete resources created by Identity Center integration. [#55400](https://github.com/gravitational/teleport/pull/55400)

This vulnerability affects all AWS IAM Identity Center integration users. You can check whether you have AWS Identity Center integration installed either in the Teleport web UI under Zero Trust Access / Integrations or by running “tctl get plugins/aws-identity-center” CLI command.

#### [High] Short to long term access escalation in Okta integration

* Enterprise fix: Verify required Okta OAuth scopes during plugin creation/update.

In Okta integration configurations with enabled access lists sync, a user with an approved  just-in-time access request to an Okta application could be unintentionally promoted to an access list granting access to the same application. This would result in the access to the Okta app/group persisting after the access request expiration.

This vulnerability affects Okta integration users who have access lists sync enabled. You can check whether you have an Okta integration installed with access lists sync enabled either in the Teleport web UI under Zero Trust Access / Integrations page or by running “tctl get plugins/okta” CLI command and looking at the “spec.settings.okta.sync_settings.sync_access_lists” flag.

#### [High] Credential theft via GitHub SSO authentication flow

* Fix improper redirect URL validation for SSO login which could be taken advantage of in a phishing attack. [#55399](https://github.com/gravitational/teleport/pull/55399)

This vulnerability affects GitHub SSO users. You can check whether you’re using GitHub SSO either on the Zero Trust Access / Auth Connectors page in Teleport web UI or by running “tctl get connectors” CLI command against your cluster.

### Other fixes and improvements

* Allow the `ssh_service.listen_addr` to forcibly be enabled when operating in reverse tunnel mode to provide an optional direct access path to hosts. [#54215](https://github.com/gravitational/teleport/pull/54215)
* View details for a bot instance. [#55347](https://github.com/gravitational/teleport/pull/55347)
* Prevent unknown resource kinds from rendering errors in the web UI. [#55208](https://github.com/gravitational/teleport/pull/55208)
* View and explore "active" bot instances. [#55201](https://github.com/gravitational/teleport/pull/55201)
* UI: Access Request reason prompts configured in Role.spec.options.request_prompt are now displayed in the reason text box, if such a role is assigned to the user. [#55173](https://github.com/gravitational/teleport/pull/55173)
* Okta: Fixed RBAC sync and Access Requests when only App and Group sync is enabled (no Access Lists sync). [#55169](https://github.com/gravitational/teleport/pull/55169)
* Fixed `tctl` rendering of timestamps in BotInstance resource YAML. [#55163](https://github.com/gravitational/teleport/pull/55163)
* Fix the impact of malicious `--db-user` values on PKINIT flow. [#55142](https://github.com/gravitational/teleport/pull/55142)
* Fix an issue with Hardware Key Support on Windows where a command would fail if the PIN prompt was not answered within 5 seconds. [#55110](https://github.com/gravitational/teleport/pull/55110)
* Fix an issue "Allowed Users" from "tsh db ls" may include irrelevant entities. [#55068](https://github.com/gravitational/teleport/pull/55068)
* Updated Web UI, tsh and Connect SSO login to support SAML `http-post` binding authentication method. The feature can be enabled from the SSO connector configuration by adding a new field as `preferred_request_binding: http-post`. [#55065](https://github.com/gravitational/teleport/pull/55065)
* Fix an issue database discovery fails when there are more than 5 OpenSearch domains. [#55058](https://github.com/gravitational/teleport/pull/55058)
* Fixed an issue with Device Trust web authentication redirection that lost the original encoding of SAML authentication data during service provider initiated SAML login. [#55048](https://github.com/gravitational/teleport/pull/55048)
* Fix configured X509 CA override chain not being used by AWS Roles Anywhere exchange. [#54947](https://github.com/gravitational/teleport/pull/54947)
* Disabled the "another session is active" prompt when per-session MFA is enabled, since MFA already enforces user confirmation when starting a desktop session. [#54928](https://github.com/gravitational/teleport/pull/54928)
* Added support for desktop access in Teleport Connect. [#54926](https://github.com/gravitational/teleport/pull/54926)
* Added workload_identity_x509_issuer_override kind to editor preset role. [#54913](https://github.com/gravitational/teleport/pull/54913)
* Hardware Key Agent validates known keys by checking active or expired login session. [#54907](https://github.com/gravitational/teleport/pull/54907)
* Expose the Teleport service cache health via prometheus metrics. [#54902](https://github.com/gravitational/teleport/pull/54902)
* Updated Go to 1.23.9. [#54896](https://github.com/gravitational/teleport/pull/54896)
* Okta: Fix creating Access Requests for Okta-originated resources in the legacy okta_service setup. [#54876](https://github.com/gravitational/teleport/pull/54876)
* Introduced the azure_devops join method to support Bot joining from the Azure Devops CI/CD platform. [#54875](https://github.com/gravitational/teleport/pull/54875)
* Add support for exclude filter for AWS IC account and groups filters. [#54835](https://github.com/gravitational/teleport/pull/54835)
* Terraform: Fixed Access List resource import. [#54802](https://github.com/gravitational/teleport/pull/54802)
* Fixed Proxy cache initialization errors in clusters with large amounts of open web sessions. [#54781](https://github.com/gravitational/teleport/pull/54781)
* Prevent restrictive validation of cluster auth preferences from causing non-auth instances to become healthy. [#54761](https://github.com/gravitational/teleport/pull/54761)
* Improved performance of joining & improved audit log entries for failed joins. [#54747](https://github.com/gravitational/teleport/pull/54747)
* Resolved an issue that could cause Teleport Connect to crash after downgrading from a newer version. [#54740](https://github.com/gravitational/teleport/pull/54740)
* Reverted the default behavior of the `teleport-cluster` Helm chart to use `authentication.secondFactor` rather than `authentication.secondFactors` to avoid incompatibility during upgrades. [#54735](https://github.com/gravitational/teleport/pull/54735)
* Workload ID: Added binary_path and binary_hash to the Unix workload attestor's attributes. [#54716](https://github.com/gravitational/teleport/pull/54716)
* Includes the attributes used in templating and rule evaluation within the audit log event for a workload identity credential issuance. [#54714](https://github.com/gravitational/teleport/pull/54714)
* Fix an issue with PIV PIN caching where a PIN that is incorrect would be cached. [#54697](https://github.com/gravitational/teleport/pull/54697)
* Fix a bug causing a malformed user to break Teleport web UI's "Users" page. [#54681](https://github.com/gravitational/teleport/pull/54681)
* Machine ID: Allow `--no-oneshot` and similar flags to override config file values. [#54651](https://github.com/gravitational/teleport/pull/54651)
* Fixed major version check for stateless environment. [#54639](https://github.com/gravitational/teleport/pull/54639)
* Teleport-update: full support for FIPS agent installations. [#54609](https://github.com/gravitational/teleport/pull/54609)
* Added support for SSO MFA as a headless MFA method. [#54599](https://github.com/gravitational/teleport/pull/54599)
* Fixed an issue preventing connections due to missing client IPs when using class E address space with GKE or CloudFlare pseudo IPv4 forward headers. [#54597](https://github.com/gravitational/teleport/pull/54597)
* Create and edit GitHub join tokens from the Join Tokens page. [#54477](https://github.com/gravitational/teleport/pull/54477)

Enterprise:
* Added ability to re-run group import in Identity Center integration.

## 17.4.8 (05/06/25)

* Fixed a possible moderator/observer terminal freeze when joining a Kubernetes moderated session. [#54523](https://github.com/gravitational/teleport/pull/54523)
* Removed background color for resources that required access request in the web UI Resources view. [#54465](https://github.com/gravitational/teleport/pull/54465)
* Show human readable title for access list audit logs. [#54459](https://github.com/gravitational/teleport/pull/54459)
* Fixed race conditions in `tsh ssh` multi-node output. [#54456](https://github.com/gravitational/teleport/pull/54456)
* Fixed an issue causing Join Token expiries to be overwritten when editing a token. [#54450](https://github.com/gravitational/teleport/pull/54450)
* Workload Identity: Fixed bugs for the Kubernetes workload attestor's container resolution. [#54442](https://github.com/gravitational/teleport/pull/54442)
* Fixed a bug in the EC2 installer script causing `Illegal option -o pipefail` errors on several distros when Managed Updates v2 are enabled. [#54429](https://github.com/gravitational/teleport/pull/54429)
* Include access request's max duration in MsTeams plugin messages. [#54388](https://github.com/gravitational/teleport/pull/54388)

## 17.4.7 (04/29/25)

* AWS Roles Anywhere output now includes the expiration time as milliseconds since unix epoch. [#54386](https://github.com/gravitational/teleport/pull/54386)
* Increased the email access plugin timeout for sending e-mails from 5 to 15 seconds. [#54381](https://github.com/gravitational/teleport/pull/54381)
* Fixed a potential panic during Auth Server startup when the backend returns an error. [#54327](https://github.com/gravitational/teleport/pull/54327)
* Added a Hardware Key Agent to Teleport Connect along with other significant UX improvements for Hardware Key support. With the agent enabled, Teleport Connect will handle prompts on behalf of other Teleport Clients (`tsh`, `tctl`), with an additional option to cache the PIN between client calls (New cluster option:`cap.hardware_key.pin_cache_ttl`). [#54297](https://github.com/gravitational/teleport/pull/54297)
* More customizability options for the AWS Roles Anywhere MWI service. [#54260](https://github.com/gravitational/teleport/pull/54260)

Enterprise:
* Okta integration: Fixed fetching Okta apps and groups preview when enrolling Access List sync. [#6411](https://github.com/gravitational/teleport.e/pull/6411)
* Fixed the Oracle audit puller breaking connection in some configurations due to expected service name mismatch. [#6399](https://github.com/gravitational/teleport.e/pull/6399)
* Web UI now correctly displays inherited Access List ownership and membership. [#6395](https://github.com/gravitational/teleport.e/pull/6395)

## 17.4.6 (04/22/25)

* User Kind is now correctly reported for Bots in the `app.session.start` audit log event. [#54241](https://github.com/gravitational/teleport/pull/54241)
* Fix a goroutine leak on TLS routing handler errors when Proxy is behind TLS-terminated load balancers. [#54224](https://github.com/gravitational/teleport/pull/54224)
* Fix issue that prevent Kubernetes agents from connecting to GKE control plane using the new DNS-based access mechanism. [#54216](https://github.com/gravitational/teleport/pull/54216)
* Tbot can now be configured to use a non-standard environment variable when sourcing the ID Token for GitLab joining. [#54187](https://github.com/gravitational/teleport/pull/54187)
* Teleport-update: stabilize binary paths in generated tbot config. [#54178](https://github.com/gravitational/teleport/pull/54178)
* Fix a bug where the `terraform-provider` preset role to lacked permissions to list Windows Desktops on clusters that got updated from v16 to v17. [#54170](https://github.com/gravitational/teleport/pull/54170)
* Fixed OIDC SSO MFA with multiple redirect URLs. [#54167](https://github.com/gravitational/teleport/pull/54167)
* Fix a bug causing the Terraform provider to fail to update `dynamic_windows_desktop` resources. [#54162](https://github.com/gravitational/teleport/pull/54162)
* Reduce log spam in discovery service error messaging. [#54149](https://github.com/gravitational/teleport/pull/54149)
* The web UI now shows role descriptions in the roles table. [#54137](https://github.com/gravitational/teleport/pull/54137)
* Leaf cluster joining attempts that conflict with an existing cluster registered with the root now generate an error instead of failing silently. [#54134](https://github.com/gravitational/teleport/pull/54134)
* Reduce backend load in clusters with large numbers of Windows desktops. [#53719](https://github.com/gravitational/teleport/pull/53719)

Enterprise:
* Fix SCIM user update bug cause by missing revision.

## 17.4.5 (04/17/25)

* The Teleport Terraform Provider now supports setting the Managed Updates v2 resources `autoupdate_config` and `autoupdate_version`. [#54109](https://github.com/gravitational/teleport/pull/54109)
* Fix a bug in managed updates v1 causing updaters v2 and AWS integrations to never update if weekdays were set in the `cluster_maintenance_config` resource. [#54088](https://github.com/gravitational/teleport/pull/54088)
* Teleport-update: ensure teleport-upgrade is always disabled when teleport-update is used. [#54087](https://github.com/gravitational/teleport/pull/54087)
* Added an option for users to select database roles when connecting to PostgreSQL databases using WebUI. [#54068](https://github.com/gravitational/teleport/pull/54068)
* Allow the use of expressions in the Where condition on Role RBAC rules for the Bot resource. [#54065](https://github.com/gravitational/teleport/pull/54065)
* Machine and Workload Identity: Increase the maximum allowed bot certificate TTL to 7 days, up from 24 hours. Larger values than the default 12 hours must be explicitly requested using the new `--max-session-ttl` flag in `tctl bots add`. [#54063](https://github.com/gravitational/teleport/pull/54063)
* Teleport-update: Improve defaulting for update groups. [#54050](https://github.com/gravitational/teleport/pull/54050)
* Fixed VNet on MacOS with hardware keys. [#54037](https://github.com/gravitational/teleport/pull/54037)
* Added SAML IdP service provider preset for Microsoft Entra External ID. [#54021](https://github.com/gravitational/teleport/pull/54021)
* Fixed TLS errors when switching between VNet apps on Windows. [#54010](https://github.com/gravitational/teleport/pull/54010)

Enterprise:
* Added support to Machine & Workload Identity SPIFFE CA for issuing X509-SVIDs using an external PKI hierarchy.

## 17.4.4 (04/14/25)

* Fixed formatting of Ed25519 SSH keys for PuTTY users. [#53972](https://github.com/gravitational/teleport/pull/53972)
* Support Oracle join method in Workload Identity templating and rule evaluation. [#53945](https://github.com/gravitational/teleport/pull/53945)
* Workload ID: the Kubernetes, Podman, and Docker attestors now capture the container image digest. [#53939](https://github.com/gravitational/teleport/pull/53939)
* Fixed web UI and tsh issues when a SAML metadata URL takes an unusually long time to respond. [#53933](https://github.com/gravitational/teleport/pull/53933)
* Updated Go to 1.23.8. [#53918](https://github.com/gravitational/teleport/pull/53918)
* Added support for specifying a WorkloadIdentity-specific maximum TTL. [#53902](https://github.com/gravitational/teleport/pull/53902)
* Fixed Azure VM auto discovery when not filtering by resource group. [#53899](https://github.com/gravitational/teleport/pull/53899)
* Added new `proxy_protocol_allow_downgrade` field to the `proxy_service` configuration in support of environments where single stack IPv6 sources are connecting to single stack IPv4 destinations. This feature is not compatible with IP pinning. [#53885](https://github.com/gravitational/teleport/pull/53885)
* Support for managing the WorkloadIdentity resource in the Teleport Kubernetes Operator. [#53862](https://github.com/gravitational/teleport/pull/53862)
* Added detailed audit events for SFTP sessions on agentless nodes. [#53836](https://github.com/gravitational/teleport/pull/53836)
* Teleport-update: Add `last_update` metadata and update tracking UUID. [#53828](https://github.com/gravitational/teleport/pull/53828)
* Restrict agent update days to Mon-Thu on Cloud. [#53765](https://github.com/gravitational/teleport/pull/53765)

Enterprise:
* Fixed an issue in the Identity Center group provisioning where group and group membership provisioning was skipped if the provisioning service failed to get user account of Access List member.

## 17.4.3 (04/07/25)

* Fixed throttling in the DynamoDB backend event stream for tables with a high amount of stream shards. [#53804](https://github.com/gravitational/teleport/pull/53804)
* Support for managing the Bot resource in the Teleport Kubernetes Operator. [#53708](https://github.com/gravitational/teleport/pull/53708)
* Kubernetes app discovery now supports an additional annotation for apps that are served on a sub-path of an HTTP service. [#53094](https://github.com/gravitational/teleport/pull/53094)

Enterprise:
* Fix Okta Integration Update Flow when the Okta integration credentials are updated from SSWS API tokens to OAuth-based credentials.
* "Bidirectional Sync" option added to the Okta Integration, allowing for a "read-only" integration where changes are only synced from Okta to Teleport.
* Fix SCIM sync for Okta plugins with OAuth credentials.

## 17.4.2 (04/01/25)

* Reduced resource consumption and improve latency of `tsh ssh`. [#53645](https://github.com/gravitational/teleport/pull/53645)
* Fixed an issue where expired app session won't redirect to login page when Teleport is using DynamoDB backend. [#53591](https://github.com/gravitational/teleport/pull/53591)
* Workload ID: Support for adding custom claims to JWT-SVIDs. [#53585](https://github.com/gravitational/teleport/pull/53585)

## 17.4.1 (03/28/25)

* Fix a bug causing the discovery service to fail to configure teleport on discovered nodes when managed updates v2 are enabled. [#53543](https://github.com/gravitational/teleport/pull/53543)
* Machine ID: `tbot` is supported for Windows and included in Windows client downloads. [#53550](https://github.com/gravitational/teleport/pull/53550)

## 17.4.0 (03/27/25)

### Database access for Oracle RDS
Teleport database access now supports connecting to Oracle RDS with Kerberos
authentication.

### AWS integration status dashboard
Teleport web UI now provides a detailed status dashboard for AWS integration as
well as the new "user tasks" view that highlights integration issues
requiring user attention along with suggested remediation steps.

### Windows desktop improvements
Teleport now supports registering the same host twice - once as a domain-joined
machine, and one as a standalone machine. This allows Teleport users to
connect as Active Directory users and local users to the same host.

### Other fixes and improvements

* Enable support for joining Kubernetes sessions in the web UI. [#53450](https://github.com/gravitational/teleport/pull/53450)
* Fixed an issue `tsh proxy db` does not honour `--db-roles` when renewing certificates. [#53445](https://github.com/gravitational/teleport/pull/53445)
* Fixed an issue that could cause backend instability when running very large numbers of app/db/kube resources through a single agent. [#53419](https://github.com/gravitational/teleport/pull/53419)
* Added `static_jwks` field to the GitLab join method configuration to support cases where Teleport Auth Service cannot reach the GitLab instance. [#53413](https://github.com/gravitational/teleport/pull/53413)
* Introduced `workload-identity-aws-ra` service for generating AWS credentials using Roles Anywhere directly from tbot. [#53408](https://github.com/gravitational/teleport/pull/53408)
* Helm chart now supports specifying a second factor list, this simplifies setting up SSO MFA with the `teleport-cluster` chart. [#53319](https://github.com/gravitational/teleport/pull/53319)
* Improved resource consumption when retrieving resources via the Web UI or tsh ls. [#53302](https://github.com/gravitational/teleport/pull/53302)
* Added support for topologySpreadConstraints to the `teleport-cluster` Helm chart. [#53287](https://github.com/gravitational/teleport/pull/53287)
* Fixed rare high CPU usage bug in reverse tunnel agents. [#53281](https://github.com/gravitational/teleport/pull/53281)
* Fixed an issue PostgreSQL via WebUI fails when IP pinning is enabled. PostgreSQL via WebUI no longer requires Proxy to dial its own public address. [#53250](https://github.com/gravitational/teleport/pull/53250)
* Added overview information to "Enroll New Resource" guides in the web UI. [#53218](https://github.com/gravitational/teleport/pull/53218)
* Added support for `SendEnv` OpenSSH option in `tsh`. [#53216](https://github.com/gravitational/teleport/pull/53216)
* Added support for using DynamoDB Streams FIPS endpoints. [#53201](https://github.com/gravitational/teleport/pull/53201)
* Allow AD and non-AD logins to single Windows desktop. [#53199](https://github.com/gravitational/teleport/pull/53199)
* Workload ID: support for attesting Systemd services. [#53108](https://github.com/gravitational/teleport/pull/53108)

Enterprise:
* Fixed Slack plugin failing to enroll with "need auth" error in the web UI.

## 17.3.4 (03/19/25)

* Improved clarity of error logs and address UX edge cases in teleport-update, part 2. [#53197](https://github.com/gravitational/teleport/pull/53197)
* Fixed the `teleport-update` systemd service in CentOS 7 and distros with older systemd versions. [#53196](https://github.com/gravitational/teleport/pull/53196)
* Fixed panic when trimming audit log entries. [#53195](https://github.com/gravitational/teleport/pull/53195)
* Fixed an issue causing the teleport process to crash on group database errors when host user creation was enabled. [#53082](https://github.com/gravitational/teleport/pull/53082)
* Workload ID: support for attesting Docker workloads. [#53069](https://github.com/gravitational/teleport/pull/53069)
* Added a `--join-method` flag to the `teleport configure` command. [#53061](https://github.com/gravitational/teleport/pull/53061)
* Improved clarity of error logs and address UX edge cases in `teleport-update`. [#53048](https://github.com/gravitational/teleport/pull/53048)
* The event handler can now generate certificates for DNS names that are not resolvable. [#53026](https://github.com/gravitational/teleport/pull/53026)
* Machine ID: Added warning when generated certificates will not last as long as expected. [#53019](https://github.com/gravitational/teleport/pull/53019)
* Improve support for `teleport-update` on CentOS 7 and distros with older systemd versions. [#53017](https://github.com/gravitational/teleport/pull/53017)
* You can now use `==` and `!=` operators with integer operands in Teleport predicate language. [#52991](https://github.com/gravitational/teleport/pull/52991)
* Workload ID: support for attesting Podman workloads. [#52978](https://github.com/gravitational/teleport/pull/52978)
* Web UI now properly shows per-session MFA errors in desktop sessions. [#52916](https://github.com/gravitational/teleport/pull/52916)
* Allow specifying the maximum number of PKCS#11 HSM connections. [#52870](https://github.com/gravitational/teleport/pull/52870)
* Resolved an issue where desktop session recordings could have incorrect proportions. [#52866](https://github.com/gravitational/teleport/pull/52866)
* The audit log web UI now renders Teleport Autoupdate Config and Version events properly. [#52838](https://github.com/gravitational/teleport/pull/52838)
* Fixed terraform provider data sources. [#52816](https://github.com/gravitational/teleport/pull/52816)

Enterprise:
* Fixed Slack plugin failing to enroll with "need auth" error in the web UI.
* Added checks to opsgenie and servicenow plugin to cause enrollment to fail if the provided config is invalid.

## 17.3.3 (03/06/25)

* Updated golang.org/x/net (addresses CVE-2025-22870). [#52846](https://github.com/gravitational/teleport/pull/52846)
* Fix the issue with multiple Okta app links that is causing a high level of Okta API usage. [#52841](https://github.com/gravitational/teleport/pull/52841)

## 17.3.2 (03/04/25)

* Updated Go to 1.23.7. [#52772](https://github.com/gravitational/teleport/pull/52772)
* Fixed VNet on Windows when the cluster uses the `legacy` signature algorithm suite. [#52767](https://github.com/gravitational/teleport/pull/52767)
* Fixed Connect installer on Windows systems using languages other than English. [#52765](https://github.com/gravitational/teleport/pull/52765)
* Allow `teleport-update` to be used in shells that set a restrictive umask. [#52755](https://github.com/gravitational/teleport/pull/52755)
* Updated `tctl create` to automatically fill the metadata and name on the `autoupdate_config` and `autoupdate_version` resources. [#52751](https://github.com/gravitational/teleport/pull/52751)
* Added version compatibility warnings to Teleport Connect when logging in to a cluster. [#52709](https://github.com/gravitational/teleport/pull/52709)
* Support setting the public address for discovered apps based on Kubernetes annotations. [#52700](https://github.com/gravitational/teleport/pull/52700)
* Fixed `cannot execute: required file not found` error with the `teleport-spacelift-runner` image. [#52560](https://github.com/gravitational/teleport/pull/52560)
* Machine ID: Added new Prometheus metrics to track success and failure of renewal loops. [#52496](https://github.com/gravitational/teleport/pull/52496)

## 17.3.1 (03/03/25)

* Fixes two issues in the 17.3.0 RPM causing package upgrades to fail and leading to teleport binaries not being symlinked in /usr/local/bin. [#52704](https://github.com/gravitational/teleport/pull/52704)
* On RPM-based distros, 17.3.0 can lead to a failed installation without a working Teleport service. The 17.3.0 RPM was pulled from our CDN. 17.3.1 should be used instead. If you updated to 17.3.0, you should update to 17.3.1. [#52704](https://github.com/gravitational/teleport/pull/52704)
* Escape user provided labels when creating the shell script that enrolls servers, applications and databases into Teleport. [#52698](https://github.com/gravitational/teleport/pull/52698)
* Disable legacy `alpn` upgrade fallback during TLS routing connection upgrades. Now only WebSocket upgrade headers are sent by default. `TELEPORT_TLS_ROUTING_CONN_UPGRADE_MODE=legacy` can still be used to force legacy upgrades but it will be deprecated in v18. [#52620](https://github.com/gravitational/teleport/pull/52620)
* Workload ID: Support for Teleport Predicate Language in Workload Identity templates and rules. [#52564](https://github.com/gravitational/teleport/pull/52564)

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

The Auth Service readiness now reflects the connectivity from the instance to
the backend storage, and the Proxy Service readiness reflects the connectivity
to the Auth Service API. In case of Auth or backend storage failure, the
instances will now turn unready. This change ensures that control plane
components can be excluded from their relevant load-balancing pools. If you want
to preserve the old behaviour (the Auth Service or Proxy Service instance stays
ready and runs in degraded mode) in the `teleport-cluster` Helm chart, you can
now tune the readiness setting to have the pods become unready after a high
number of failed probes.

### Other fixes and improvements

* Added `tctl edit` support for Identity Center plugin resources. [#52605](https://github.com/gravitational/teleport/pull/52605)
* Added Oracle join method to web UI provision token editor. [#52599](https://github.com/gravitational/teleport/pull/52599)
* Added warnings to VNet on macOS about other software that might conflict with VNet, based on inspecting network routes on the system. [#52552](https://github.com/gravitational/teleport/pull/52552)
* Added auto-importing of Oracle Cloud tags. [#52543](https://github.com/gravitational/teleport/pull/52543)
* Added support for X509 revocations to Workload Identity. [#52503](https://github.com/gravitational/teleport/pull/52503)
* Git proxy commands executed in terminals now support interactive login prompts when the `tsh` session expires. [#52475](https://github.com/gravitational/teleport/pull/52475)
* Connect is now installed per-machine instead of per-user on Windows. [#52453](https://github.com/gravitational/teleport/pull/52453)
* Added `teleport-update` for default build. [#52361](https://github.com/gravitational/teleport/pull/52361)

Enterprise:

* Improved sync performance in Identity Center integration.
* Delete related Git servers when deleting GitHub integration in the web UI.

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
* Added a preview of changes to access to resources in the role editor. This feature requires Teleport Identity Security.

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
