# Introduction 

## What is Teleport?

Gravitational Teleport is a modern SSH server for remotely accessing clusters
of Linux servers via SSH or HTTPS. It is intended to be used instead of `sshd`
for organizations who need:

* SSH audit with session recording/replay.
* Easily manage SSH trust between teams, organizations and data centers.
* SSH into behind-firewall clusters without any open ports.
* Role-based access control (RBAC) for SSH protocol.

## Teleport Editions

Welcome to the Teleport documentation! Teleport is available through the free,
open source edition ("Teleport Community Edition") or a commercial edition
("Teleport Enterprise Edition"). 

Below is a brief summary of the documentation available here. 

## Teleport Community 

The Community Edition is [on Github](https://github.com/gravitational/teleport) 
if you want to dive into the
code. These documentation is also available in [the Github
repository](https://github.com/gravitational/teleport/tree/master/docs) so feel
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
majority of documentation between the Community Edition and the commercial,
Enterprise Edition overlaps we have separated out the documentation that is
specific to Teleport Enterprise. 

- [Teleport Enterprise Introduction](enterprise) - Overview of the additional capabilities of Teleport Enterprise.
- [RBAC for SSH](ssh_rbac) - Details on how Teleport Enterprise provides Role-based Access Controls (RBAC) for SSH.
- [SSO for SSH](ssh_sso) - Overview on how Teleport Enterprise works with external identity providers for single sign-on (SSO).

## Guides

- [Okta Integration](ssh_okta) - How to integrate Teleport Enterprise with Okta.
- [ADFS Integration](ssh_adfs) - How to integrate Teleport Enterprise with Active Directory.
- [One Login Integration](ssh_one_login) - How to integrate Teleport Enterprise with One Login.
- [OIDC Integration](oidc) - How to integrate Teleport Enterprise with identity providers using OpenID Connect (OIDC) / OAuth2.

Teleport is made by [Gravitational](https://gravitational.com/). We hope you
enjoy using Teleport. If you have comments or questions, feel free to reach out
to the Gravitational Team:
[info@gravitational.com](mailto:info@gravitational.com).
