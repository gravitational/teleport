# Overview

## Introduction

Gravitational Teleport ("Teleport") is a tool for remotely accessing isolated clusters of
Linux servers via SSH or HTTPS. Unlike traditional key-based access, Teleport
enables teams to easily adopt the following practices:

- Avoid key distribution and [trust on first use](https://en.wikipedia.org/wiki/Trust_on_first_use) issues by using auto-expiring keys signed by a cluster certificate authority (CA).
- Enforce 2nd factor authentication.
- Connect to clusters located behind firewalls without direct Internet access via SSH bastions.
- Record and replay SSH sessions for knowledge sharing and auditing purposes.
- Collaboratively troubleshoot issues through session sharing.
- Discover online servers and Docker containers within a cluster with dynamic node labels.

Teleport is built on top of the high-quality [Golang SSH](https://godoc.org/golang.org/x/crypto/ssh)
implementation and it is fully compatible with OpenSSH.

## Why Build Teleport?

Mature tech companies with significant infrastructure footprints tend to implement most
of these patterns internally. Teleport allows smaller companies without
significant in-house SSH expertise to easily adopt them, as well. Teleport comes with an
accessible Web UI and a very permissive [Apache 2.0](https://github.com/gravitational/teleport/blob/master/LICENSE)
license to facilitate adoption and use.

Being a complete standalone tool, Teleport can be used as a software library enabling
trust management in complex multi-cluster, multi-region scenarios across many teams
within multiple organizations.

## Who Built Teleport?

Teleport was created by [Gravitational Inc](https://gravitational.com). We have built Teleport
by borrowing from our previous experiences at Rackspace. It has been extracted from [Telekube](https://gravitational.com/telekube/), our system for helping our clients to deploy
and remotely manage their SaaS applications on many cloud regions or even on-premise.

## Resources
To get started with Teleport we recommend starting with the [Architecture Document](architecture.md). Then if you want to jump right in and play with Teleport, you can read the [Quick Start](quickstart.md). For a deeper understanding of how everything works and recommended production setup, please review the [Admin Manual](admin-guide.md) to setup Teleport and the [User Manual](user-manual.md) for daily usage. There is also an [FAQ](faq.md) where we'll be collecting common questions. Finally, you can always type `tsh`, `tctl` or `teleport` in terminal after Teleport has been installed to review those reference guides.

The best way to ask questions or file issues regarding Teleport is by creating a Github issue or pull request. Otherwise, you can reach us through the contact form or chat on our [website](https://gravitational.com/).

