# Introduction

## What is Teleport?

Gravitational Teleport is a gateway for managing access to clusters of Linux
servers via SSH or the Kubernetes API. It is intended to be used instead of
traditional OpenSSH for organizations that need to:

* Secure their infrastructure and comply with security best-practices and
  regulatory requirements.
* Have complete visibility into activity happening across their infrastructure.
* Reduce the operational overhead of privileged access management across both
  traditional and cloud-native infrastructure.

<iframe width="712" height="400" src="https://www.youtube.com/embed/GvAD5aNcdBA?rel=0&modestbranding=1&widget_referrer=gravitational.com/teleport/docs" frameborder="0" allow="accelerometer; autoplay; encrypted-media; modestbranding; gyroscope; picture-in-picture" allowfullscreen></iframe>

Teleport aims to be a cloud-native SSH solution, i.e. it makes it natural to think of
environments, not servers. Below is a list of the most popular Teleport features:

* Single SSH/Kubernetes access gateway for an entire organization.
* SSH certificate based authentication instead of static keys.
* Avoid key distribution and [trust on first use](https://en.wikipedia.org/wiki/Trust_on_first_use) issues by using auto-expiring keys signed by a cluster certificate authority (CA).
* Enforce 2nd factor authentication.
* Connect to clusters located behind firewalls without direct Internet access via SSH bastions.
* Collaboratively troubleshoot issues through session sharing.
* Discover online servers and Docker containers within a cluster with dynamic node labels.
* The ability to manage trust between teams, organizations and data centers.
* SSH/Kubernetes access into behind-firewall environments without any open ports.
* Role-based access control (RBAC) for SSH.
* A single tool ("pane of glass") to manage RBAC for both SSH and Kubernetes.
* Audit log with session recording/replay.
* Kubernetes audit log, including the recording of interactive commands executed via `kubectl`.
* The same workflows and ease of use that devs get with familiar `ssh` / `kubectl` commands.
* Ability to run in "agentless" mode, i.e. most Teleport features are
  available on clusters with pre-existing SSH daemons, usually `sshd`.

Teleport is available through the free, open source edition ("Teleport Community Edition")
or a commercial edition ("Teleport Enterprise Edition").

![teleport diagram](https://gravitational.com/images/diagram-teleport.png)

## Operating System Support

Teleport is officially supported on the platforms listed below. It is worth noting
that the open source community has been successful in building and running Teleport on
UNIX variants other than Linux [2].

Operating System      |  Teleport Client   | Teleport Server
----------------------|--------------------|-----------------
Linux v2.6+           |  yes               | yes
MacOS v10.12+         |  yes               | yes
Windows [1]           |  yes [1]           | no

[1] _Teleport server does not run on Windows yet, but `tsh` (the Teleport client)
  can be used on Windows to execute `tsh login` to retrieve a user's SSH
  certificate and use it with `ssh`, the OpenSSH client, running on a Windows
  client machine._

[2] _Teleport is written in Go and it is theoretically possible to build it on
    any OS supported by the [Golang toolchain](https://github.com/golang/go/wiki/MinimumRequirements)_.

## Teleport Community

The Community Edition is [on Github](https://github.com/gravitational/teleport)
if you want to dive into the
code. This documentation is also available in [the Github
repository](https://github.com/gravitational/teleport/tree/master/docs), so feel
free to create an issue or pull request if you have comments.

- [Quickstart Guide](quickstart.md) - A quick tutorial to show off the basic
  capabilities of Teleport. A good place to start if you want to jump right in.
- [Teleport Architecture](architecture/teleport_architecture_overview.md) - This section covers the underlying
  design principles of Teleport and provides a detailed description of Teleport's
  architecture. A good place to learn about Teleport's design and how it works.
- [User Manual](user-manual.md) - This manual expands on the Quickstart and
  provides end users with all they need to know about how to use Teleport.
- [Admin Manual](admin-guide.md) - This manual covers installation and
  configuration of Teleport and the ongoing management of Teleport.
- [FAQ](faq.md) - Common questions about Teleport.

## Teleport Enterprise

Teleport Enterprise is built around the open-source core in Teleport Community,
with the added benefits of role-based access control (RBAC) and easy
integration with identity managers for single sign-on (SSO). Because the
majority of documentation between the Community and Enterprise Editions overlap,
we have separated out the documentation that is specific to Teleport Enterprise.

- [Teleport Enterprise Introduction](enterprise) - Overview of the additional capabilities of Teleport Enterprise.
- [Teleport Enterprise Quick Start](enterprise/quickstart-enterprise.md) - A quick tutorial to show off the basic capabilities of Teleport Enterprise.
A good place to start if you want to jump right in.
- [RBAC for SSH](enterprise/ssh_rbac.md) - Details on how Teleport Enterprise provides Role-based Access Controls (RBAC) for SSH.
- [SSO for SSH](enterprise/ssh_sso.md) - Overview on how Teleport Enterprise works with external identity providers for single sign-on (SSO).

## Guides

We also have several guides that go through the most typical configurations and integrations.

- [Okta Integration](enterprise/sso/ssh_okta.md) - How to integrate Teleport Enterprise with Okta.
- [ADFS Integration](enterprise/sso/ssh_adfs.md) - How to integrate Teleport Enterprise with Active Directory.
- [One Login Integration](enterprise/sso/ssh_one_login.md) - How to integrate Teleport Enterprise with One Login.
- [OIDC Integration](enterprise/sso/oidc.md) - How to integrate Teleport Enterprise with identity providers using OIDC/OAuth2.
- [Kubernetes Integration](kubernetes_ssh.md) - How to configure Teleport to serve as a unified gateway for Kubernetes clusters and clusters of regular SSH nodes.

## Why Build Teleport?

Mature tech companies with significant infrastructure footprints tend to implement most
of these patterns internally. Teleport allows smaller companies without significant in-house
SSH expertise to easily adopt them, as well. Teleport comes with an accessible Web UI
and a very permissive [Apache 2.0](https://github.com/gravitational/teleport/blob/master/LICENSE)
license to facilitate adoption and use.

Being a complete standalone tool, Teleport can be used as a software library enabling
trust management in complex multi-cluster, multi-region scenarios across many teams
within multiple organizations.

## Who Built Teleport?

Teleport was created by [Gravitational Inc](https://gravitational.com). We have built
Teleport by borrowing from our previous experiences at Rackspace. It has been extracted
from [Gravity](https://gravitational.com/gravity/), our system for helping our clients
to deploy and remotely manage their SaaS applications on many cloud regions or even on-premise.

## Support and Contributing

We offer a few different options for support. First, we try to provide clear and comprehensive documentation. Documentation is also available in [the Github repository](https://github.com/gravitational/teleport/tree/master/docs), so feel free to create a PR or file an issue if you think improvements can be made.

If you still have questions after reviewing our docs, you can also:

* Join the [Teleport Community](https://community.gravitational.com/c/teleport) to ask questions. Our engineers are available there to help you.
* If you want to contribute to Teleport or file a bug report/issue, you can do so by creating an issue in [Github](https://github.com/gravitational/teleport/).
* If you are interested in [Teleport Enterprise](https://gravitational.com/teleport/docs/enterprise/) or more responsive support during a POC, we can also create a dedicated Slack channel for you during your POC. You can [reach out to us through our website](https://gravitational.com/teleport/) or email us at [sales@gravitational.com](mailto:sales@gravitational.com) to arrange for a POC.

Teleport is made by [Gravitational](https://gravitational.com/), and we hope you
enjoy using it. If you have comments or questions, feel free to reach out
to the Gravitational Team:
[info@gravitational.com](mailto:info@gravitational.com).
