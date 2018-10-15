# Changelog

## 2.7.6

This release of Teleport contains the following bug fix:

* Fix regression that marked ADFS claims as invalid. [#2293](https://github.com/gravitational/teleport/pull/2293)

## 2.7.5

This release of Teleport focuses on bugfixes.

* Fixed issue that causes auth server to fill up disk with `/tmp/multipart-` files [#2250](https://github.com/gravitational/teleport/issues/2250)

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
  migrating to role variables which are documented [here](https://gravitational.com/teleport/docs/ssh_rbac/#roles)

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

* Proper handling of `ENV_SUPATH` from login.defs [#1004](https://github.com/gravitational/teleport/pull/1004)
* Reverse tunnels would periodically lose connectivity. [#1156](https://github.com/gravitational/teleport/issues/1156)
* tsh now stores user identities in a format compatible with OpenSSH. [1171](https://github.com/gravitational/teleport/issues/1171).

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

## Features

* Native support for DynamoDB back-end for storing cluster state.
* It is now possible to turn off 2nd factor authentication.
* 2nd factor now uses TOTP. #522
* New and easy to use framework for implementing secret storage plug-ins.
* Audit log format has been finalized and documented.
* Experimental simple file-based secret storage back-end.
* SSH agent forwarding.

## Improvements

* Friendlier CLI error messages.
* `tsh login` is now compatible with SSH agents.

## Enterprise Features

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
* U2F was enabled by default in "demo mode" if teleport.yaml file was missing.

### Improvements

* U2F documentation has been improved

## 1.3

This release includes several major new features and it's recommended for production use.

### Features

* Support for hardware U2F keys for 2nd factor authentication.
* CLI client profiles: tsh can now remember its --proxy setting.
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
* Better compatibility with ssh command: -t flag can be used to force allocation of TTY

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
HTTPS port for Teleport proxy for tsh --proxy flag.

## 1.0.3

This release only includes one major bugfix #486 plus minor changes not exposed
to OSS Teleport users.

### Bugfixes

* Guessing `advertise_ip` chooses IPv6 address space. #486

## 1.0

The first official release of Teleport!
