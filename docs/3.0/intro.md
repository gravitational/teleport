# Introduction

## What is Teleport?

Gravitational Teleport is a gateway for managing access to clusters of Linux servers via SSH or the Kubernetes API. It is intended to be used instead of traditional OpenSSH for organizations that need to: 

* Secure their infrastructure and comply with security best-practices and regulatory requirements.
* Have complete visibility into activity happening across their infrastructure.
* Reduce the operational overhead of privileged access management across both traditional and cloud-native infrastructure.

Teleport gives teams that manage infrastructure access:

* The ability to manage trust between teams, organizations and data centers.
* Certificate based authentication instead of static keys.
* SSH access into behind-firewall environments without any open ports.
* Role-based access control (RBAC) for SSH.
* A single tool ("pane of glass") to manage RBAC for both SSH and Kubernetes.
* SSH audit log with session recording/replay.
* Kubernetes audit log, including the recording of interactive commands executed via `kubectl`.
* The same workflows and ease of that they get with familiar `ssh` / `kubectl` commands.

Teleport is available through the free, open source edition ("Teleport Community Edition") 
or a commercial edition ("Teleport Enterprise Edition"). 

Below is a brief summary of the documentation available here. 

## Teleport Community 

The Community Edition is [on Github](https://github.com/gravitational/teleport) 
if you want to dive into the
code. This documentation is also available in [the Github
repository](https://github.com/gravitational/teleport/tree/master/docs), so feel
free to create an issue or pull request if you have comments. 

- [Quickstart Guide](quickstart/) - A quick tutorial to show off the basic
  capabilities of Teleport. A good place to start if you want to jump right in.
- [Teleport Architecture](architecture/) - This section covers the underlying
  design principles of Teleport and a detailed description of Teleport
  architecture. A good place to learn about Teleport's design and how it works.
- [User Manual](user-manual/) - This manual expands on the Quickstart and
  provides end users with all they need to know about how to use Teleport.
- [Admin Manual](admin-guide/) - This manual covers installation and
  configuration of Teleport and the ongoing management of Teleport.
- [FAQ](faq/) - Common questions about Teleport.

## Teleport Enterprise

Teleport Enterprise is built around the open-source core in Teleport Community,
with the added benefits of role-based access control (RBAC) and easy
integration with identity managers for single sign-on (SSO). Because the
majority of documentation between the Community and Enterprise Editions overlap, 
we have separated out the documentation that is specific to Teleport Enterprise. 

- [Teleport Enterprise Introduction](enterprise) - Overview of the additional capabilities of Teleport Enterprise.
- [Teleport Enterprise Quick Start](quickstart-enterprise) - A quick tutorial to show off the basic capabilities of Teleport Enterprise. 
A good place to start if you want to jump right in.
- [RBAC for SSH](ssh_rbac) - Details on how Teleport Enterprise provides Role-based Access Controls (RBAC) for SSH.
- [SSO for SSH](ssh_sso) - Overview on how Teleport Enterprise works with external identity providers for single sign-on (SSO).

## Guides

We also have several guides that go through the most typical configurations and integrations.

- [Okta Integration](ssh_okta) - How to integrate Teleport Enterprise with Okta.
- [ADFS Integration](ssh_adfs) - How to integrate Teleport Enterprise with Active Directory.
- [One Login Integration](ssh_one_login) - How to integrate Teleport Enterprise with One Login.
- [OIDC Integration](oidc) - How to integrate Teleport Enterprise with identity providers using OIDC/OAuth2.
- [Kubernetes Integration](kubernetes_ssh) - How to configure Teleport to serve as a unified gateway for Kubernetes clusters and clusters of regular SSH nodes.

Teleport is made by [Gravitational](https://gravitational.com/). We hope you
enjoy using Teleport. If you have comments or questions, feel free to reach out
to the Gravitational Team:
[info@gravitational.com](mailto:info@gravitational.com).
