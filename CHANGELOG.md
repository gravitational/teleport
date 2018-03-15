# Changelog

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
* Resolved toggling regression in Trusted Clusters. [#1751](https://github.com/gravitational/teleport/issues/1751)
* Key written with wrong username to `~/.tsh`. [#1749](https://github.com/gravitational/teleport/issues/1749)

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

## 2.3.7

Teleport 2.3.7 fixes a security vulnerability that allowed an attacker with
direct network access to the Auth Server to write a client for the Auth Server
that could by-pass second factor authentication.

We strongly encourage anyone running Teleport 2.3 to upgrade their Auth Server
to 2.3.7 to mitigate this issue.

#### Bug Fixes

* Don't allow second factor by-pass. [#1550](https://github.com/gravitational/teleport/pull/1550)

## 2.3.6

This release contains a minor bugfix.

#### Bug fixes

* When returning a full trusted cluster schema, the extension schema was ignored [#1476](https://github.com/gravitational/teleport/pull/1476)

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
