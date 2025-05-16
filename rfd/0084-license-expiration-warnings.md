---
authors: edwarddowling (edward.dowling@goteleport.com), r0mant (roman@goteleport.com)
state: implemented
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

## Scope

Self-hosted Teleport will be the target of this RFD. Teleport Cloud is out of scope because the Cloud license does not have proper expiration yet. When Cloud implements proper license expiration, this feature will kick in automatically.
License warning should be displayed in both Web UI and CLI (tsh/tctl).

### CLI UX

License warnings should be displayed in CLI (tsh/tctl)

TSH - on “tsh login” and "tsh status"
When the user uses the “tsh login” command the appropriate license warning will be displayed. (see examples below)
This will be achieved by showing alerts that have the 'on-login' label.
When the user uses "tsh status" expired license warnings will be displayed along with any other 'high severity' alerts.

TCTL - on any "tctl" command when expired.
When the user uses any "tctl" command the appropriate license warning will be displayed if expired.
Similar to 'tsh status' this will be all alerts that have 'high severity' which the expired license alerts will be included in.

### Example outputs

The license warning portion of the outputs should be coloured red and yellow respectively for the expired and unexpired warnings.
This can be determined by the severity of the cluster alert, 5-9 for yellow and 10+ for red.

```
$ build/tsh status
Profile URL:        https://localhost:9876
Logged in as:       someUser
Cluster:            someCluster
Roles:              access, editor
Logins:             someUser
Valid until:        2022-08-12 21:57:19 +0100 IST [valid for 4h44m0s]
Extensions:         permit-agent-forwarding, permit-port-forwarding, permit-pty

Your Teleport Enterprise Edition license has expired on instance InstanceName. Please reach out to [licenses@goteleport.com](mailto:licenses@goteleport.com) to obtain a new license. Inaction may lead to unplanned outage or degraded performance and support.
```

```
$ build/tsh login --proxy=localhost:9876 --user someUser
Profile URL:        https://localhost:9876
Logged in as:       someUser
Cluster:            localhost
Roles:              access, editor
Logins:             someUser
Valid until:        2022-08-16 03:38:07 +0100 IST [valid for 12h0m0s]
Extensions:         permit-agent-forwarding, permit-port-forwarding, permit-pty

Your Teleport Enterprise Edition license will expire in 10 days on instance InstanceName. Please reach out to [licenses@goteleport.com](mailto:licenses@goteleport.com) to obtain a new license. Inaction may lead to unplanned outage or degraded performance and support.
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

Your Teleport Enterprise Edition license has expired on instance InstanceName. Please reach out to [licenses@goteleport.com](mailto:licenses@goteleport.com) to obtain a new license. Inaction may lead to unplanned outage or degraded performance and support.
```


CLI does not support snoozing the warnings as you can in the webUI. Which provides the functionality to disable the warnings temporarily.

### Web UI UX

Web UI will display license expiration banner on top of the page with the following rules:
Show warning (yellow) to users N days prior to expiration, 1 day snooze
Show error (red) to all users after expiration, no dismiss or snoozing available.
Snoozing allows users to disable the warning in the web UI from being displayed for N days.

## Implementation details

License warnings can piggyback on the cluster alert endpoint `ServerWithRoles.GetClusterAlerts` with the responses being

```
{
  "alerts": [
    {
      "kind": "cluster_alert",
      "version": "v1",
      "metadata": {
        "name": "123e4567-e89b-12d3-a456-426614174000.",
        "labels": {
          "teleport.internal/alert-on-login": "yes",
          "teleport.internal/alert-permit-all": "yes"
        },
        "expires": "2022-08-31T17:26:05.728149Z"
      },
      "spec": {
        "severity": 5,
        "message": "Your Teleport Enterprise Edition license will expire in 10 days on instance InstanceName. Please reach out to [licenses@goteleport.com](mailto:licenses@goteleport.com) to obtain a new license. Inaction may lead to unplanned outage or degraded performance and support.",
        "created": "2022-08-30T17:26:05.728149Z"
      }
    }
  ]
}
```

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
        "message": "Your Teleport Enterprise Edition license has expired on instance InstanceName. Please reach out to [licenses@goteleport.com](mailto:licenses@goteleport.com) to obtain a new license. Inaction may lead to unplanned outage or degraded performance and support.",
        "created": "2022-08-30T17:26:05.728149Z"
      }
    }
  ]
}
```


On startup and every 1 hour afterwards the auth server will check the license and generate a license warning if applicable, or clear license warning alerts if they are no longer needed. License warning alerts are only generated and cleared in Teleport enterprise and not Teleport cloud or OSS Teleport.

The warnings are to be be created up to 90 days prior to the license expiring.
All requests to grab cluster alerts will be made with a timeout of 500ms.

The cluster alert spec may also need to be modified to add a bool for whether an alert is allowed to be dismissed for the web ui alerts.

```
message ClusterAlertSpec {
  // Severity represents how problematic/urgent the alert is.
  AlertSeverity Severity = 1 [(gogoproto.jsontag) = "severity"];
  // Message is the user-facing message associated with the alert.
  string Message = 2 [(gogoproto.jsontag) = "message"];
  // Created is the time at which the alert was generated.
  google.protobuf.Timestamp Created = 3 [
    (gogoproto.jsontag) = "created,omitempty",
    (gogoproto.stdtime) = true,
    (gogoproto.nullable) = false
  ];
  // Dismissible is set if the alert is unable to be dismissed in the webui.
  bool Dismissible = 5 [(gogoproto.jsontag) = "dismissible"];
}
```

## Security considerations
If your license expired, you can’t disable or snooze the warning.
Users could avoid seeing this warning by passing the output of the CLI through a program that strips the warning from it.
