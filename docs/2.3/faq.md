# FAQ

### Can I use Teleport in production today?

Teleport has completed a security audit from a nationally recognized technology security company. 
So we are comfortable with the use of Teleport from a security perspective. However, Teleport 
is still a relatively young product so you may experience usability issues. We are actively 
supporting Teleport and addressing any issues that are submitted to the [github repo](https://github.com/gravitational/teleport).

### Can I connect to nodes behind a firewall?

Yes, Teleport supports reverse SSH tunnels out of the box. To configure behind-firewall clusters
refer to [Trusted Clusters](admin-guide.md#trusted-clusters) section of the Admin Manual.

### Does Web UI support copy and paste?

Yes. You can copy&paste using the mouse. For working with a keyboard, Teleport employs `tmux`-like
"prefix" mode. To enter prefix mode, press `Ctrl+A`.

While in prefix mode, you can press `Ctrl+V` to paste, or enter text selection mode by pressing `[`.
When in text selection mode, move around using `hjkl`, select text by toggling `space` and copy
it via `Ctrl+C`.

### Can I use OpenSSH with a Teleport cluster?

Yes. Take a look at [Using OpenSSH client](user-manual.md##using-teleport-with-openssh) section in the User Manual
and [Using OpenSSH servers](admin-guide.md) in the Admin Manual.

### What TCP ports does Teleport use?

[Ports](admin-guide.md#ports) section of the Admin Manual covers it.

### Does Teleport support authentication via OAuth, SAML or Active Directory?

Gravitational offers this feature for the [commercial versions of Teleport](enterprise.md#rbac).

### Do you offer a commercial version of Teleport?

Yes, in addition to the [numerous advanced features](enterprise.md), the commercial Teleport license 
also gives you the following:

* Commercial support.
* Premium SLA with guaranteed response times.
* Implementation Services: our team can help you integrate Teleport with your
  existing systems and processes.

Reach out to `sales@gravitational.com` if you have questions about commercial edition of Teleport.
