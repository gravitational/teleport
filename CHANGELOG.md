# Changelog

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
