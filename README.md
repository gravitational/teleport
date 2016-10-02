# Gravitational Teleport

|Project Links|
|---|
| [Teleport Website](http://gravitational.com/teleport)  |
| [Documentation](http://gravitational.com/teleport/docs/quickstart/)  |
| [Demo Video](https://www.youtube.com/watch?v=7eVAC2U8OtM) |

## Introduction

Gravitational Teleport is a modern SSH server for remotely accessing clusters of Linux servers via SSH or HTTPS. It is intended to be used instead of `sshd`. Teleport enables teams to easily adopt the best SSH practices like:

- No need to distribute keys: Teleport uses certificate-based access with automatic expiration time.
- Enforcement of 2nd factor authentication.
- Cluster introspection: every Teleport node becomes a part of a cluster and is visible on the Web UI.
- Record and replay SSH sessions for knowledge sharing and auditing purposes.
- Collaboratively troubleshoot issues through session sharing.
- Connect to clusters located behind firewalls without direct Internet access via SSH bastions.
- Ability to integrate SSH credentials with your organization identities via OAuth (Google Apps, Github).

Teleport is built on top of the high-quality [Golang SSH](https://godoc.org/golang.org/x/crypto/ssh) implementation and it is fully compatible with OpenSSH.

## Installing and Running

Download the [latest binary release](https://github.com/gravitational/teleport/releases), 
unpack the .tar.gz and run `sudo make install`. This will copy Teleport binaries into 
`/usr/local/bin`.

Then you can run Teleport as a single-node cluster:

```
teleport start 
```

## Building Teleport

You need to have Golang `v1.7` or newer, and you must have the `zip` binary on
your path.

Run 

1. `go get github.com/gravitational/teleport`
2. `cd $GOPATH/src/github.com/gravitational/teleport`
3. `make`

If the build was successful the binaries are here: `$GOPATH/src/github.com/gravitational/teleport/build`

You'll have to create `/var/lib/teleport` directory and then you can start 
Teleport as a single-node cluster in development mode: `build/teleport start -d`

If you want to release your own Teleport version, edit this [Makefile](Makefile), update 
`VERSION` and `SUFFIX` constants, then run `make setver` to update [version.go](version.go)

If you want to cut another binary release tarball, run `make release`.

NOTE: The Go compiler is somewhat sensitive to amount of memory: you will need at least 1GB of 
virtual memory to compile Teleport. 512MB instance without swap will not work.

## Why did We Build Teleport?

Mature tech companies with significant infrastructure footprints tend to implement most
of these patterns internally. Teleport allows smaller companies without 
significant in-house SSH expertise to easily adopt them, as well. Teleport comes with an 
accessible Web UI and a very permissive [Apache 2.0](https://github.com/gravitational/teleport/blob/master/LICENSE)
license to facilitate adoption and use.

Being a complete standalone tool, Teleport can be used as a software library enabling 
trust management in complex multi-cluster, multi-region scenarios across many teams 
within multiple organizations.

## More Information

* [Quick Start Guide](docs/quickstart.md)
* [Teleport Architecture](docs/architecture.md)
* [Admin Manual](docs/admin-guide.md)
* [User Manual](docs/user-manual.md)
* [FAQ](docs/faq.md)

## Contributing

The best way to contribute is to create issues or pull requests right here on Github. You can also reach the Gravitational team through their [website](https://gravitational.com/)


## Status

Teleport has completed a security audit from a nationally recognized technology security company. 
So we are comfortable with the use of Teleport from a security perspective.

However, Teleport is still a relatively young product so you may experience usability issues. 
We are actively supporting Teleport and addressing any issues that are submitted to this repo. Ask questions,
send pull requests, report issues and don't be shy! :)

The latest stable Teleport build can be found in [Releases](https://github.com/gravitational/teleport/releases)

## Known Issues

* Teleport does not officially support IPv6 yet.

## Who Built Teleport?

Teleport was created by [Gravitational Inc](https://gravitational.com). We have built Teleport 
by borrowing from our previous experiences at Rackspace. It has been extracted from [Gravity](https://gravitational.com/product), our system for helping our clients to deploy 
and remotely manage their SaaS applications on many cloud regions or even on-premise.
