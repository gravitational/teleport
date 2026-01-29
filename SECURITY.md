# Security Policy

## Supported Versions

The list of supported versions can be found
[here](https://goteleport.com/docs/faq/#which-version-of-teleport-is-supported).

## Reporting a Vulnerability

To report a security vulnerability to us, use the embedded form on our
[security page](https://goteleport.com/security).

## Private Releases

Teleport undergoes regular security reviews by internal teams and third-party firms.
These reviews occasionally uncover high- and critical-severity issues that need prompt patching.

Immediately committing a fix to our public open source repository can expose implementation
details that allow attackers to develop exploits before users have time to install patches.
To reduce that risk, we may publish a private security release.

Release artifacts (binaries, Helm charts, Docker images, Linux packages, etc.) are
published to their normal download locations, while the source code and technical
details remain private until after customers have had time to upgrade.

### What to expect

- The private release appears in our [changelog](https://goteleport.com/docs/changelog/)
  like any other release, but with a brief note indicating that it is a private security
  release (no detailed changelog).
- Release assets are published to their standard locations for both Teleport Enterprise
  and Teleport Community Edition users.
- The release is built from a private repository. The source code for the release is
  not immediately available in the public repository.

### Rollout and disclosure timeline

- Release published: private release assets become available in download channels
- Cloud rollout: we upgrade Teleport Cloud. The rollout schedule depends
  on severity, but most tenants receive the update within 7 days.
- Enterprise Notification: after Teleport Cloud has been upgraded, Teleport Enterprise
  customers are notified with details about the vulnerability and guidance for remediation.
- Public Disclosure (one week later): we merge the fixes to the public repository and
  publish a new public release containing the security patches and any accumulated non-security changes.

### FAQ

#### I am a Teleport Enterprise customer. Why haven't I received details about the private release I see on the changelog?

Details of the release are communicated to Teleport Enterprise customers after
the Teleport Cloud platform has been upgraded. This can take up to two weeks
from the time the release is made available.

#### I am a Teleport Enterprise customer. How do I know where these security notifications will be sent?

Teleport Enterprise customers can configure up to 3 email addresses to receive
security notifications. Instructions on how to update security contacts are
available in our docs for both
[self-hosted](https://goteleport.com/docs/faq/#how-do-i-update-my-security-and-business-contacts)
and
[cloud](https://goteleport.com/docs/cloud-faq/#how-do-i-update-my-security-and-business-contacts)
customers.

#### I am a Teleport Community Edition user. Are private security releases available for me too?

Yes. We publish release assets for both Teleport Enterprise and Teleport Community Edition builds.
