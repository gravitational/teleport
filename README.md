# Teleport

Teleport is a unified access plane for infrastructure:

* [Clusters of Linux servers](https://goteleport.com/teleport/docs/quickstart/) via SSH or SSH-over-HTTPS in a browser
* [Kubernetes clusters](https://goteleport.com/teleport/docs/kubernetes-access/)
* [Web Applications](https://goteleport.com/teleport/docs/application-access/)
* [Databases - (Postgres Preview)](https://goteleport.com/teleport/docs/preview/teleport-database-access/)

## Why Use Teleport

Here are some of the most popular use-cases for Teleport:

* Centralized SSH and Kubernetes Certificate Authority.
* Capture sessions and manage certificates for existing [OpenSSH fleet](https://goteleport.com/teleport/docs/openssh-teleport).
* Secure access to internal web applications and services with [application access](https://goteleport.com/teleport/docs/application-access).
* Use short lived certificates instead of static keys for SSH, Kubernetes, Databases and Web Apps.
* Structured events and session recording/replay for `ssh` and `kubectl`.
* Setup a unified access plane for an entire organization.
* Enforce 2nd factor auth with U2F and TOTP.
* Connect to computing resources located behind firewalls or without static IPs.
* Collaboratively troubleshoot issues through [session sharing](https://goteleport.com/teleport/docs/user-manual#sharing-sessions).
* Discover online servers and Docker containers within a cluster with [dynamic node labels](https://goteleport.com/teleport/docs/admin-guide#labeling-nodes-and-applications).

In addition to its hallmark features, Teleport is interesting for smaller teams
because it facilitates easy adoption of the best infrastructure security
practices like:

- No need to distribute keys: Teleport uses certificate-based access with automatic certificate expiration time.
- 2nd factor authentication (2FA) for Apps, SSH and Kubernetes.
- Collaboratively troubleshoot issues through session sharing.
- Single sign-on (SSO) for Applications, SSH/Kubernetes and your organization identities via
  Github Auth, OpenID Connect or SAML with endpoints like Okta or Active Directory.

Teleport is built on top of [Golang SSH](https://godoc.org/golang.org/x/crypto/ssh)
implementation and it can be used with `sshd` servers and `ssh` clients.

|Project Links| Description
|---|----
| [Teleport Website](https://goteleport.com/teleport) | The official website of the project |
| [Documentation](https://goteleport.com/teleport/docs/quickstart/) | Admin guide, user manual and more |
| [Demo Video](https://www.youtube.com/watch?v=DUlTAlEJr5w) | 5-minute video overview of the UI. |
| [Teleconsole](https://www.teleconsole.com) | The free service to "invite" SSH clients behind NAT, built on top of Teleport |
| [Blog](https://goteleport.com/blog/) | Our blog where we publish Teleport news |
| [Community Forum](https://community.goteleport.com) | Teleport Community Forum|

[![Teleport Demo](/docs/4.3/img/readme/teleport-4.3-video-thumb.png)](https://www.youtube.com/watch?v=0HlyGk8dihM)

## Installing and Running

Download the [latest binary release](https://goteleport.com/teleport/download),
unpack the .tar.gz and run `sudo ./install`. This will copy Teleport binaries into
`/usr/local/bin`.

Then you can run Teleport as a single-node cluster:

```bash
$ sudo teleport start
```

In a production environment Teleport must run as root. But to play, just do `chown $USER /var/lib/teleport`
and run it under `$USER`, in this case you will not be able to login as someone else though.

## Docker

### Deploy Teleport
If you wish to deploy Teleport inside a Docker container:
```
# This command will pull the Teleport container image for version 5.0
# Replace 5.0 with the version you need:
$ docker pull quay.io/gravitational/teleport:5.0
```
View latest tags on [Quay.io | gravitational/teleport](https://quay.io/repository/gravitational/teleport?tab=tags)

### For Local Testing and Development
Follow instructions at [docker/README](docker/README.md)

## Building Teleport

Teleport source code consists of the actual Teleport daemon binary written in Golang, and also
of a web UI (a git submodule located in /webassets directory) written in Javascript.

Make sure you have Golang `v1.15` or newer, then run:

```bash
# get the source & build:
$ git clone https://github.com/gravitational/teleport.git
$ cd teleport
$ make full

# create the default data directory before starting:
$ sudo mkdir -p -m0700 /var/lib/teleport
$ sudo chown $USER /var/lib/teleport
```

If the build succeeds the binaries will be placed in
`$GOPATH/src/github.com/gravitational/teleport/build`

NOTE: The Go compiler is somewhat sensitive to amount of memory: you will need
at least 1GB of virtual memory to compile Teleport. 512MB instance without swap
will not work.

NOTE: This will build the latest version of Teleport, regardless of whether it is stable. If you want to build the latest stable release, `git checkout` to that tag (e.g. `git checkout v5.0.0`) before running `make full`.

### Rebuilding Web UI

Teleport Web UI is located in the [Gravitational Webapps](https://github.com/gravitational/webapps) monorepo.
You can clone that repository and rebuild teleport UI package with:

```bash
$ git clone git@github.com:gravitational/webapps.git
$ cd webapps
$ make build-teleport
```

Then you can replace Teleport web UI files with the one found in the generated `/dist` folder.

To enable speedy iterations on the Web UI, you can run a
[local web-dev server](https://github.com/gravitational/webapps/tree/master/packages/teleport).

You can also tell teleport to load the web UI assets from the source directory.
To enable this behavior, set the environment variable `DEBUG=1` and rebuild with the default target:

```bash
# Run Teleport as a single-node cluster in development mode:
$ DEBUG=1 ./build/teleport start -d
```

Keep the server running in this mode, and make your UI changes in `/dist` directory.
Refer to [the webapps README](https://github.com/gravitational/webapps/blob/master/README.md) for instructions on how to update the Web UI.

### Managing dependencies

Dependencies are managed using [Go
modules](https://blog.golang.org/using-go-modules). Here are instructions for
some common tasks:

#### Add a new dependency

Latest version:

```bash
go get github.com/new/dependency
# Update the source to actually use this dependency, then run:
make update-vendor
```

Specific version:

```bash
go get github.com/new/dependency@version
# Update the source to actually use this dependency, then run:
make update-vendor
```

#### Set dependency to a specific version

```bash
go get github.com/new/dependency@version
make update-vendor
```

#### Update dependency to the latest version

```bash
go get -u github.com/new/dependency
make update-vendor
```

#### Update all dependencies

```bash
go get -u all
make update-vendor
```

#### Debugging dependencies

Why is a specific package imported: `go mod why $pkgname`.

Why is a specific module imported: `go mod why -m $modname`.

Why is a specific version of a module imported: `go mod graph | grep $modname`.

## Why did We Build Teleport?

The Teleport creators used to work together at Rackspace. We noticed that most
cloud computing users struggle with setting up and configuring infrastructure
security because popular tools, while flexible, are complex to understand and
expensive to maintain. Additionally, most organizations use multiple
infrastructure form factors such as several cloud providers, multiple cloud
accounts, servers in colocation, and even smart devices. Some of those devices
run on untrusted networks, behind third party firewalls. This only magnifies
complexity and increases operational overhead.

We had a choice, either to start a security consulting business or build a
solution that’s dead-easy to use and understand, something that creates an
illusion of all of your servers being in the same room as you as if they were
magically _teleported_. And Teleport was born!

## More Information

* [Quick Start Guide](https://goteleport.com/teleport/docs/quickstart)
* [Teleport Architecture](https://goteleport.com/teleport/docs/architecture)
* [Admin Manual](https://goteleport.com/teleport/docs/admin-guide)
* [User Manual](https://goteleport.com/teleport/docs/user-manual)
* [FAQ](https://goteleport.com/teleport/docs/faq)

## Support and Contributing

We offer a few different options for support. First of all, we try to provide clear and comprehensive documentation. The docs are also in Github, so feel free to create a PR or file an issue if you think improvements can be made. If you still have questions after reviewing our docs, you can also:

* Join the [Teleport Community](https://community.goteleport.com/c/teleport) to ask questions. Our engineers are available there to help you.
* If you want to contribute to Teleport or file a bug report/issue, you can do so by creating an issue here in Github.
* If you are interested in Teleport Enterprise or more responsive support during a POC, we can also create a dedicated Slack channel for you during your POC. You can [reach out to us through our website](https://goteleport.com/teleport/) to arrange for a POC.

## Is Teleport Secure and Production Ready?

Teleport has completed several security audits from the nationally recognized
technology security companies. [Some](https://goteleport.com/blog/teleport-release-2-2/) of
[them](https://goteleport.com/blog/teleport-security-audit/) have been made public.
We are comfortable with the use of Teleport from a security perspective.

You can see the list of companies who use Teleport in production on the Teleport
[product page](https://goteleport.com/case-study/).

However, Teleport is still a relatively young product so you may experience
usability issues.  We are actively supporting Teleport and addressing any
issues that are submitted to this repo. Ask questions, send pull requests,
report issues and don't be shy! :)

The latest stable Teleport build can be found in [Releases](https://goteleport.com/teleport/download)

## Who Built Teleport?

Teleport was created by [Gravitational Inc](https://goteleport.com). We have
built Teleport by borrowing from our previous experiences at Rackspace. It has
been extracted from [Gravity](https://goteleport.com/gravity), our
Kubernetes distribution optimized for deploying and remotely controlling complex
applications into multiple environments _at the same time_:

* Multiple cloud regions
* Colocation
* Private enterprise clouds located behind firewalls
