# Gravitational Teleport

Gravitational Teleport is a modern SSH server for remotely accessing clusters
of Linux servers via SSH or HTTPS. It is intended to be used instead of `sshd`
for organizations who need:

* SSH audit with session recording/replay.
* Easily manage SSH trust between teams, organizations and data centers.
* SSH into behind-firewall clusters without any open ports.
* Role-based access control (RBAC) for SSH protocol.

In addition to its hallmark features, Teleport is interesting for smaller teams
because it facilitates easy adoption of the best SSH practices like:

- No need to distribute keys: Teleport uses certificate-based access with automatic certificate expiration time.
- 2nd factor authentication (2FA) for SSH.
- Collaboratively troubleshoot issues through session sharing.
- Single sign-on (SSO) for SSH and your organization identities via OpenID Connect or SAML with endpoints like Okta or Active Directory.
- Cluster introspection: every SSH node and its status can be queried via CLI and Web UI.

Teleport is built on top of the high-quality [Golang SSH](https://godoc.org/golang.org/x/crypto/ssh) 
implementation and it is _fully compatible with OpenSSH_ and can be used with
`sshd` servers and `ssh` clients.

|Project Links| Description
|---|----
| [Teleport Website](http://gravitational.com/teleport)  | The official website of the project |
| [Documentation](http://gravitational.com/teleport/docs/quickstart/)  | Admin guide, user manual and more |
| [Demo Video](https://www.youtube.com/watch?v=7eVAC2U8OtM) | 3-minute video overview of the UI. |
| [Teleconsole](http://www.teleconsole.com) | The free service to "invite" SSH clients behind NAT, built on top of Teleport |
| [Blog](http://blog.gravitational.com) | Our blog where we publish Teleport news |

## Installing and Running

Download the [latest binary release](https://github.com/gravitational/teleport/releases), 
unpack the .tar.gz and run `sudo ./install`. This will copy Teleport binaries into 
`/usr/local/bin`.

Then you can run Teleport as a single-node cluster:

```bash
$ sudo teleport start 
```

In a production environment Teleport must run as root. But to play, just do `chown $USER /var/lib/teleport` 
and run it under `$USER`, in this case you will not be able to login as someone else though.

## Building Teleport

Teleport source code consists of the actual Teleport daemon binary written in Golang, and also
it has a web UI (located in /web directory) written in Javascript. The WebUI is not changed often
and we keep it checked into Git under `/dist`, so you only need to build Golang:

Make sure you have Golang `v1.8.3` or newer, then run:

```bash
# get the source & build:
$ mkdir -p $GOPATH/src/github.com/gravitational
$ cd $GOPATH/src/github.com/gravitational
$ git clone https://github.com/gravitational/teleport.git
$ cd teleport
$ make full

# create the default data directory before starting:
$ sudo mkdir -p /var/lib/teleport
$ sudo chown $USER /var/lib/teleport
```

If the build succeds the binaries will be placed in 
`$GOPATH/src/github.com/gravitational/teleport/build`

NOTE: The Go compiler is somewhat sensitive to amount of memory: you will need
at least 1GB of virtual memory to compile Teleport. 512MB instance without swap
will not work.

### Rebuilding Web UI

To enable speedy iterations on the Web UI, teleport can load the web UI assets 
from the source directory. To enable this behavior, set the environment variable 
`DEBUG=1` and rebuild with the default target:

```bash
$ make

# Run Teleport as a single-node cluster in development mode: 
$ DEBUG=1 ./build/teleport start -d
```

Keep the server running in this mode, and make your UI changes in `/dist` directory.
Refer to [web/README.md](web/README.md) for instructions on how to update the Web UI.

## Why did We Build Teleport?

Mature tech companies with significant infrastructure footprints tend to implement most
of these patterns internally. Teleport allows smaller companies without 
significant in-house SSH expertise to easily adopt them, as well. Teleport comes with an 
accessible Web UI and a very permissive [Apache 2.0](https://github.com/gravitational/teleport/blob/master/LICENSE)
license to facilitate adoption and use.

Being a complete standalone tool, Teleport can be used as a software library
enabling trust management in complex multi-cluster, multi-region scenarios
across many teams within multiple organizations.

## More Information

* [Quick Start Guide](http://gravitational.com/teleport/docs/quickstart)
* [Teleport Architecture](http://gravitational.com/teleport/docs/architecture)
* [Admin Manual](http://gravitational.com/teleport/docs/admin-guide)
* [User Manual](http://gravitational.com/teleport/docs/user-manual)
* [FAQ](http://gravitational.com/teleport/docs/faq)

## Contributing

The best way to contribute is to create issues or pull requests right here on Github. 
You can also reach the Gravitational team through their [website](https://gravitational.com/)

## Is Teleport Secure and Production Ready?

Teleport has completed several security audits from the nationally recognized
technology security companies. Some of them have been [made public](https://gravitational.com/blog/teleport-release-2-2/). 
We are comfortable with the use of Teleport from a security perspective.

However, Teleport is still a relatively young product so you may experience
usability issues.  We are actively supporting Teleport and addressing any
issues that are submitted to this repo. Ask questions, send pull requests,
report issues and don't be shy! :)

The latest stable Teleport build can be found in [Releases](https://github.com/gravitational/teleport/releases)

## Known Issues

* Teleport does not officially support IPv6 yet.

## Who Built Teleport?

Teleport was created by [Gravitational Inc](https://gravitational.com). We have
built Teleport by borrowing from our previous experiences at Rackspace. It has 
been extracted from [Telekube](https://gravitational.com/product), our
Kubernetes distribution optimized for deploying and remotely controlling complex 
applications into multiple environments _at the same time_:

* Multiple cloud regions
* Colocation 
* Private enterprise clouds located behind firewalls

