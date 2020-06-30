# Changelog

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

* New build command for client-only (tsh) .pkg builds. [#3159](https://github.com/gravitational/teleport/pull/3159)
* Added support for etcd password auth. [#3234](https://github.com/gravitational/teleport/pull/3234)
* Added third-party s3 support. [#3234](https://github.com/gravitational/teleport/pull/3234)
* Fixed an issue where access-request event system fails when cache is enabled. [#3223](https://github.com/gravitational/teleport/pull/3223)
* Fixed cgroup resolution so enhanced session recording works on Debian based distributions. [#3215](https://github.com/gravitational/teleport/pull/3215)

## 4.2.0

This is a minor Teleport release with a focus on new features and bug fixes.

### Improvements

* Alpha: Enhanced Session Recording lets you know what's really happening during a Teleport Session. [Read the docs](https://gravitational.com/teleport/docs/ver/4.2/features/enhanced_session_recording/). [#2948](https://github.com/gravitational/teleport/issues/2948)
* Alpha: Workflows API lets admins escalate RBAC roles in response to user requests. [Read the docs](https://gravitational.com/teleport/docs/ver/4.2/enterprise/#approval-workflows). [#3006](https://github.com/gravitational/teleport/issues/3006)
* Beta: Teleport provides HA Support using Firestore and Google Cloud Storage using Google Cloud Platform. [Read the docs](https://gravitational.com/teleport/docs/ver/4.2/gcp_guide/). [#2821](https://github.com/gravitational/teleport/pull/2821)
* Remote tctl execution is now possible. [Read the docs](https://gravitational.com/teleport/docs/ver/4.2/cli-docs/#tctl). [#1525](https://github.com/gravitational/teleport/issues/1525) [#2991](https://github.com/gravitational/teleport/issues/2991)

### Fixes

* Fixed issue in socks4 when rendering remote address [#3110](https://github.com/gravitational/teleport/issues/3110)

### Documentation

* Adopting root/leaf terminology for trusted clusters. [Trusted cluster documentation](https://gravitational.com/teleport/docs/ver/4.2/trustedclusters/).
* Documented Teleport FedRAMP & FIPS Support. [FedRAMP & FIPS documentation](https://gravitational.com/teleport/docs/ver/4.2/enterprise/ssh_fips/).

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
* Allow tsh to go background and without executing remote command. [#2297](https://github.com/gravitational/teleport/issues/2297)
* Provide a high level tool to backup and restore the cluster state. [#2480](https://github.com/gravitational/teleport/issues/2480)
* Investigate nodes using stale list when connecting to proxies (discovery protocol). [#2832](https://github.com/gravitational/teleport/issues/2832)

### Fixes

* Proxy can hang due to invalid OIDC connector. [#2690](https://github.com/gravitational/teleport/issues/2690)
* Proper `-D` flag parsing. [#2663](https://github.com/gravitational/teleport/issues/2663)
* tsh status does not show correct cluster name. [#2671](https://github.com/gravitational/teleport/issues/2671)
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

* Fixed issue where new versions of tsh could not connect to older clusters. [#2969](https://github.com/gravitational/teleport/pull/2969)
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
* Fixed issue preventing tsh 4.0 from connection to 3.2 clusters. [#2784](https://github.com/gravitational/teleport/pull/2784)

## 4.0.0

This is a major Teleport release which introduces support for Teleport Internet of Things (IoT). In addition to this new feature this release includes usability, performance, and bug fixes listed below.

## New Features

### Teleport for IoT

With Teleport 4.0, nodes gain the ability to use reverse tunnels to dial back to a Teleport cluster to bypass firewall restrictions. This allows connections even to nodes that a cluster does not have direct network access to. Customers that have been using Trusted Clusters to achieve this can now utilize a unified interface to access all nodes within their infrastructure.

### FedRamp Compliance

With this release of Teleport, we have built out the foundation to help Teleport Enterprise customers build and meet the requirements in a FedRAMP System Security Plan (SSP). This includes a FIPS 140-2 friendly build of Teleport Enterprise as well as a variety of improvements to aid in complying with security controls even in FedRAMP High environments.

## Improvements

* Teleport now support 10,000 remote connections to a single Teleport cluster. [Using our recommend hardware setup.](https://gravitational.com/teleport/faq/#whats-teleport-scalability-and-hardware-recommendations)
* Added ability to delete node using `tctl rm`. [#2685](https://github.com/gravitational/teleport/pull/2685)
* Output of `tsh ls` is now sorted by node name. [#2534](https://github.com/gravitational/teleport/pull/2534)

## Bug Fixes

* Switched to `xdg-open` to open a browser window on Linux. [#2536](https://github.com/gravitational/teleport/pull/2536)
* Increased SSO callback timeout to 180 seconds. [#2533](https://github.com/gravitational/teleport/pull/2533)
* Set permissions on TTY similar to OpenSSH. [#2508](https://github.com/gravitational/teleport/pull/2508)

The lists of improvements and bug fixes above mention only the significant changes, please take a look at the complete list on Github for more.

## Upgrading

Teleport 4.0 is backwards compatible with Teleport 3.2 and later. [Follow the recommended upgrade procedure to upgrade to this version.](https://gravitational.com/teleport/docs/admin-guide/#upgrading-teleport)

Note that due to substantial changes between Teleport 3.2 and 4.0, we recommend creating a backup of the backend datastore (DynamoDB, etcd, or dir) before upgrading a cluster to Teleport 4.0 to allow downgrades.

### Notes on compatibility

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

* Fixed issue where new versions of tsh could not connect to older clusters. [#2969](https://github.com/gravitational/teleport/pull/2969)
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

* Added `--bind-addr` to force tsh to bind to a specific port during SSO login. [#2620](https://github.com/gravitational/teleport/issues/2620)

## 3.2

This version brings support for Amazon's managed Kubernetes offering (EKS).

Starting with this release, Teleport proxy uses [the impersonation API](https://kubernetes.io/docs/reference/access-authn-authz/authentication/#user-impersonation) instead of the [CSR API](https://kubernetes.io/docs/tasks/tls/managing-tls-in-a-cluster/#requesting-a-certificate).

## 3.1.14

This release of Teleport contains a bug fix.

* Fixed issue where Web UI could not connect to older nodes within a cluster. [#2993](https://github.com/gravitational/teleport/pull/2993)

## 3.1.13

This release of Teleport contains two bug fixes.

* Fixed issue where new versions of tsh could not connect to older clusters. [#2969](https://github.com/gravitational/teleport/pull/2969)
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
