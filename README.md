# Gravitational Teleport

## Introduction

Gravitational Teleport is a tool for remotely accessing isolated clusters of 
Linux servers via SSH or HTTPS. Unlike traditional key-based access, Teleport 
enables teams to easily adopt the following practices:

- Avoid key distribution and [trust on first use](https://en.wikipedia.org/wiki/Trust_on_first_use) issues by using auto-expiring keys signed by a cluster certificate authority (CA).
- Enforce 2nd factor authentication.
- Connect to clusters located behind firewalls without direct Internet access via SSH bastions.
- Record and replay SSH sessions for audit purposes.
- Collaboratively troubleshoot issues via built-in session sharing.
- Discover online servers and running Docker containers within a cluster with dynamic node labels.

Take a look at [Quick Start](docs/quickstart.md) page to get a taste of using Teleport, or read the 
[Architecture Document](docs/architecture.md) to get a full understanding of how Teleport works.

## Why?

Mature tech companies with significant infrastructure footprints tend to implement most
of these patterns internally. Gravitational Teleport allows smaller companies without 
significant in-house SSH expertise to easily adopt them as well. Teleport comes with a 
beautiful Web UI and a very permissive [Apache 2.0](https://github.com/gravitational/teleport/blob/master/LICENSE)
license.

Teleport is built on top of the high-quality [Golang SSH](https://godoc.org/golang.org/x/crypto/ssh) 
implementation and it is fully [compatible with OpenSSH](docs/admin-guide.md#using-teleport-with-openssh): you can 
use `ssh` clients and `sshd` servers with Teleport clusters.

## More Information

* [Quick Start Guide](docs/quickstart.md)
* [Teleport Architecture](docs/architecture.md)
* [Admin Manual](docs/admin-guide.md)
* [User Manual](docs/user-manual.md)
* [FAQ](docs/faq.md)

## Contributing

The best way to contribute is to create issues right here on Github. You can also reach the Gravitational team through their [website](http://gravitational.com/)

### Building

Teleport is written in Go. If you have Golang 1.5 and newer, simply clone this repository
and run `make`.

## Status

**Teleport is not ready to be used in production yet**. Teleport is undergoing a comprehensive 
independent security audit.

## Who Built Teleport?

Teleport was created by [Gravitational Inc](https://gravitational.com). We have built Teleport 
by borrowing from our previous experiences at Rackspace. It has been extracted from [Gravity](http://gravitational.com/vendors.html), our system for helping our clients to deploy 
and remotely manage their SaaS applications into many cloud regions or even on-premise.

Being a wonderful standalone tool, Teleport can be used as a software library enabling 
trust management in a complex multi-cluster, multi-region scenarios across many teams 
within multiple organizations.
