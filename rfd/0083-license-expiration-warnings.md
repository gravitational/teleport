---
authors: edwarddowling (edward.dowling@goteleport.com), r0mant (roman@goteleport.com)
---


# RFD 83 - License warnings

## Required approvers

- Engineering: `@r0mant`
- Product: `@klizentas && @xinding33`
- Security: `@reedloden`

## What

This RFD proposes a way to display license warnings to self hosted users both before and after license expiration.

## Why

To encourage the renewal of licenses after expiration and let users know before their license expires.

## Success criteria

Teleport displays license warnings to users in both CLI and web UI starting from a certain time prior to the license expiration, as well as post-expiration.
Teleport administrators can optionally configure the license expiration warning interval but not disable it completely on a per role basis.

## Scope

Self-hosted Teleport will be the target of this RFD. Teleport Cloud is out of scope because the Cloud license does not have proper expiration yet. When Cloud implements proper license expiration, this feature will kick in automatically.
License warning should be displayed in both Web UI and CLI (tsh/tctl).

### CLI UX

License warnings should be displayed in CLI (tsh/tctl) 
TSH - on “tsh login”, in “tsh status”
When the user uses either the “tsh login” or ‘tsh status” commands the appropriate license warning will be displayed. (see examples below)
TCTL - “tctl status”
When the user uses the “tctl status” command the appropriate license warning will be displayed.
After expiration all tsh and tctl will display the license expiry warning.

### Example outputs

The license warning portion of the outputs should be coloured red and orange respectively for the expired and unexpired warnings.

```bash
$ build/tsh status
> Profile URL:        https://localhost:9876
  Logged in as:       someUser
  Cluster:            someCluster
  Roles:              access, editor
  Logins:             someUser
  Valid until:        2022-08-12 21:57:19 +0100 IST [valid for 4h44m0s]
  Extensions:         permit-agent-forwarding, permit-port-forwarding, permit-pty
  Your Teleport Enterprise Edition license will expire in 10 days. Please reach out
  to   your Teleport Account Manager (or licenses@goteleport.com) to obtain a new
  license.
```

```
$ build/tsh login --proxy=localhost:9876 --user someUser
> Profile URL:        https://localhost:9876
  Logged in as:       someUser
  Cluster:            localhost
  Roles:              access, editor
  Logins:             someUser
  Valid until:        2022-08-16 03:38:07 +0100 IST [valid for 12h0m0s]
  Extensions:         permit-agent-forwarding, permit-port-forwarding, permit-pty

  Your Teleport Enterprise Edition license has expired. Please reach out to your
  Teleport Account Manager (or licenses@goteleport.com) to obtain a new license.
```

```
$ build/tctl status -c /home/edward/teleport/teleport.yaml
Cluster  localhost                                                               
Version  10.1.2                                                                  
host CA  never updated                                                           
user CA  never updated                                                           
db CA    never updated                                                           
jwt CA   never updated                                                           
CA pin   sha256:3d72102f020146d09ff400810f59f70b8163ebfe6ec1ecfb0b3b2a0c151592

Your Teleport Enterprise Edition license will expire in 10 days. Please reach out
to   your Teleport Account Manager (or licenses@goteleport.com) to obtain a new
license.
```


CLI does not support snoozing the warnings as you can in the webUI. Which provides the functionality to disable the warnings temporarily.

### Web UI UX

Web UI will display license expiration banner on top of the page with the following rules:
Show warning (yellow) to users N days prior to expiration, 1 day snooze, based on license_warning_days role option with same warning text as the CLI
Show error (red) to all users after expiration, no dismiss or snoozing available.
Snoozing allows users to disable the warning in the web UI from being displayed for N days.

### Configuration UX

The time interval in which to show a warning before expiry can be configured on a per role basis.
Interval can be configured by modifying a role option ‘license_warning_days’.
Default will be 30 days, admins can configure if needed.
Default preset roles will use the default. For example the editor/auditor/access roles.
When multiple roles for the same user have different values for this interval the shortest one will be used.
This value cannot be configured to be below the minimum value of 10 days.


```yaml
kind: role
version: v5
metadata:
  name: editor
spec:
  options:
    license_warning_days: 90  # ← defaults to 30, min 10
```

Default values for default roles:
Editor - 90 days
Auditor - 30 days
Access - 30 days

If you have multiple roles, the shortest time period takes precedence.

## Implementation details

Existing web api endpoint ‘/ping’ already returns a license warning. Due to this endpoint being unauthenticated and the proposed functionality requires the ability to differentiate based on identity/role this cannot be used as is.
Instead this RFD proposes a new authenticated endpoint ‘/license’ be exposed to provide the required functionality. This would consist of
An auth api method under the hood.
A webapi endpoint that calls it.
Its request body would be empty and the the response would be of the form

```
message LicenseWarning { 
    WarningMessage String
}
```

## Security considerations
Admins cannot fully disable warnings, because there will be a minimum threshold of 30 days.
If your license expired, you can’t disable or snooze the warning.
Users could avoid seeing this warning by passing the output of the CLI through a program that strips the warning from it.

