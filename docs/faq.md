# FAQ

### Can I use Gravitational Teleport in production today?

Teleport has completed a security audit from a nationally recongized technology security company. 
So we are comfortable with the use of Teleport from a security perspective. However, Teleport 
is still a relatively young product so you may experience usability issues. We are actively 
supporting Teleport and addressing any issues that are submitted to the [github repo](https://github.com/gravitational/teleport).

### Can I connect to nodes behind a firewall via SSH with Teleport?

Yes, Teleport supports reverse SSH tunnels out of the box. To configure behind-firewall clusters
refer to [Trusted Clusters](admin-guide.md#trusted-clusters) section of the Admin Manual.

### Does Web UI support copy and paste?

Yes. You can copy&paste using the mouse. For working with a keyboard, Teleport employs `tmux`-like
"prefix" mode. To enter prefix mode, press `Ctrl+A`.

While in prefix mode, you can press `Ctrl+V` to paste, or enter text selection mode by pressing `[`.
When in text selection mode, move around using `hjkl`, select text by toggling `space` and copy
it via `Ctrl+C`.

### Can I use OpenSSH client to connect to servers in a Teleport cluster?

Yes. Take a look at [Using OpenSSH client](user-manual.md#integration-with-openssh) section in the User Manual
and [Using OpenSSH servers](admin-guide.md) in the Admin Manual.

### What TCP ports does Teleport use?

[Ports](admin-guide.md#ports) section of the Admin Manual covers it.

### Does Teleport support 3rd party Authentication?

Teleport supports Google Apps out of the box, see [OpenID/OAuth2](admin-guide/#openid-oauth2) for how to configure it.
Other OpenID providers can easily be added - Teleport code is open and your contributions are welcome! :)

### Does Teleport support Authentication via LDAP or Active Directory?

Gravitational offers this feature as part of the commercial version for Teleport.

### Do you offer a commercial version of Teleport?

Yes, we offer a commercial version which includes:

* Features like multi-cluster capabilities, integration with enterprise identity management (LDAP and others) and custom feature development.
* The option of fully managed Teleport clusters running on your infrastructure.
* Shipping of audit logs to 3rd party log management systems.
* Premium SLA with guaranteed response times.

Reach out to `sales@gravitational.com` if you have questions about commerial edition of Teleport.
