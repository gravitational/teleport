---
authors: camh (camh@goteleport.com)  
state: draft
---

# RFD 97 - License Expiry

## Required approvers

- Engineering: `@r0mant`
- Product:
- Security:

## What

Disable SSO enterprise features after license has expired, with a grace period.

## Why

To encourage users to renew their Teleport Enterprise license.

[RFD 84] specified a license expiry warning to encourage users to renew their
license. This RFD extends that to disable enterprise features if the license
still has not been renewed after a grace period.

This RFD proposes to disable SAML and OIDC SSO logins only as they are likely
the most common reason to use Teleport Enterprise. It does not seem likely that
other enterprise features are being used without also using the enterprise SSO
connectors, so it is unnecessary work to disable other features.

[RFD 84]: https://github.com/gravitational/teleport/blob/master/rfd/0084-license-expiration-warnings.md

## Success Criteria

The message shown upon license expiry will warn of upcoming disablement of
enterprise features.

After a 30-day grace period after licence expiry, the license expired banner
shown on the web UI and in response to `tsh login`, `tsh status` and `tctl` will
say that enterprise features have been disabled.

After a 30-day grace period after license expiry, login via SAML or OIDC SSO
will fail with a license expired error message.

## Scope

This RFD applies to self-hosted Teleport Enterprise. Teleport Cloud is currently
excluded due to missing license expiration.

## Detail

### UX (CLI and web)

The license checker implemented in [RFD 84] already emits alerts at 90 days
pre-expiry and at expiry. These alerts are shown when a user runs `tsh login`,
`tsh status` or any `tctl` command, and are displayed in a banner on the web UI.

The alert for license expiry reads:

    Your Teleport Enterprise Edition license has expired on one or more of your
    auth servers. Please reach out to licenses@goteleport.com to obtain a new
    license. Inaction may lead to unplanned outage or degraded performance and
    support.

The last sentence of that message will be changed to:

    Inaction will lead to enterprise features being disabled and limited
    support.

An additional alert will be emitted at 30 days post-expiry alerting users that
enterprise features have been disabled. The banner for this alert will not be
able to be snoozed or dismissed. The alert will read:

    Your Teleport Enterprise Edition license has expired on one or more of your
    auth servers. Enterprise features have been disabled. Please reach out to
    licenses@goteleport.com to obtain a new license.

This follows the existing behaviour as documented in [RFD 84] with the addition
of the 30 days post-expiry message, and the alteration of the expiry message to
warn of enterprise features being disabled (Note: it appears it is possible to
dismiss the expiry message. It appears this is still to be implemented).

### Implementation

The new alert will appear in the response to `ServerWithRoles.GetClusterAlerts`
as:

```
{
  "alerts": [
    {
      "kind": "cluster_alert",
      "version": "v1",
      "metadata": {
        "name": "123e4567-e89b-12d3-a456-426614174000",
        "labels": {
          "teleport.internal/alert-on-login": "yes",
          "teleport.internal/alert-permit-all": "yes",
          "teleport.internal/license-warning-expired": "yes",
        },
        "expires": "2022-08-31T17:26:05.728149Z"
      },
      "spec": {
        "severity": 5,
        "message": "Your Teleport Enterprise Edition license has expired on one or more of your auth servers. Enterprise features have been disabled. Please reach out to [licenses@goteleport.com](mailto:licenses@goteleport.com) to obtain a new license.",
        "created": "2022-08-30T17:26:05.728149Z"
      }
    }
  ]
}
```

The alerts for license checking are performed hourly and these run on a clock
internal to the license checker. This clock is not available to the SAML and
OIDC connectors. This makes it hard to ensure that the web and cli warnings are
synchronized with the enterprise features being disabled as it can take up to 60
minutes after the grace period expires for the license checker to update the
alerts.

To ensure that the SSO connectors disable features at the same time as the alert
is emitted, the license checker will be changed to ensure it emits an alert
exactly at license expiry as well as expiry of the grace period. This will allow
the SSO connectors to use just the license to check for expiry and not need to
know about the license checker clock.

The SAML and OIDC connectors have four entry points that will be amended to
check the license expiry:

* `UpsertConnector`
* `DeleteConnector`
* `CreateAuthRequest`
* `ValidateResponse`

These entry points will first check that that the license grace period has not
expired. If it has, it will return an `AccessDenied` error with the message
"Teleport Enterprise license expired", causing these operations with these
connectors to fail.

### Expiry timing

It is possible for user certificates to be issued prior to the expiry of the
grace period that would expire after the grace period. This would allow users to
continue to use their SSO-validated user credentials after the SSO connectors
have been disabled. Since a key feature of Teleport is short-term credentials,
this is acceptable as it will not be all that long until those user credentials
expire and need to be renewed, at which time the SSO connectors will be disabled
if the license is not renewed.
