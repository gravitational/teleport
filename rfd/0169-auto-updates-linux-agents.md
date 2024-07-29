---
authors: Stephen Levine (stephen.levine@goteleport.com)
state: draft
---

# RFD 0169 - Automatic Updates for Agents

## Required Approvers

* Engineering: @russjones && @bernardjkim
* Product: @klizhentas || @xinding33 
* Security: Doyensec

## What

This RFD proposes a new mechanism for Teleport agents to automatically update to a version scheduled by an operator via tctl.

All agent installations are in-scope for this proposal, including agents installed on Linux servers and Kubernetes.

The following anti-goals are out-of-scope for this proposal, but will be addressed in future RFDs:
- Signing of agent artifacts via TUF
- Teleport Cloud APIs for updating agents
- Improvements to the local functionality of the Kubernetes agent for better compatibility with FluxCD and ArgoCD.

This RFD proposes a specific implementation of several sections in https://github.com/gravitational/teleport/pull/39217.

Additionally, this RFD parallels the auto-update functionality for client tools proposed in https://github.com/gravitational/teleport/pull/39805.

## Why

The existing mechanism for automatic agent updates does not provide a hands-off experience for all Teleport users.

1. The use of system package management leads to interactions with `apt upgrade`, `yum upgrade`, etc. that can result in unintentional upgrades.
2. The use of system package management requires complex logic for each target distribution.
3. The installation mechanism requires 4-5 commands, includes manually installing multiple packages, and varies depending on your version and edition of Teleport.
4. The use of bash to implement the updater makes changes difficult and prone to error.
5. The existing auto-updater has limited automated testing.
6. The use of GPG keys in system package managers has key management implications that we would prefer to solve with TUF in the future.
7. The desired agent version cannot be set via Teleport's operator-targeted CLI (tctl).
8. The rollout plan for the new agent version is not fully-configurable using tctl.
9. Agent installation logic is spread between the auto-updater script, install script, auto-discovery script, and documentation.
10. Teleport contains logic that is specific to Teleport Cloud upgrade workflows.
11. The existing auto-updater is not self-updating.
12. It is difficult and undocumented to automate agent upgrades with custom automation (e.g., with JamF). 

We must provide a seamless, hands-off experience for auto-updates that is easy to maintain.

## Details - Teleport API

Teleport will be updated to serve the desired agent version and edition from `/v1/webapi/find`.
The version and edition served from that endpoint will be configured using the `cluster_maintenance_config` and `autoupdate_version` resources.
Whether the updater querying the endpoint is instructed to upgrade (via `agent_auto_update`) is dependent on the `host=[uuid]` parameter sent to `/v1/webapi/find`.

To ensure that the updater is always able to retrieve the desired version, instructions to the updater are delivered via unauthenticated requests to the `/v1/webapi/find`.
Teleport proxies use their access to heartbeat data to drive the rollout and modulate the `/v1/webapi/find` response given the host UUID.

Rollouts are specified as interdependent groups of hosts, selected by SSH resource or instance label query.
A host is eligible to upgrade if the seleciton query returns true.
Instance labels are a new feature introduced by this RFD that may be used when SSH service is not running or it is undesirable to reuse SSH labels:

```
teleport:
  labels:
    environment: staging
  commands:
    # this command will add a label 'arch=x86_64' to an instance
    - name: arch
      command: ['/bin/uname', '-p']
      period: 1h0m0s
```

Only static and command-based and labels may be used.

At the start of a group rollout, the Teleport proxy marks a desired group of hosts to update in the backend.
An arbitrary but UUID-deterministic fixed number of hosts (`max_in_flight % x total`) are instructed to upgrade at the same time via `/v1/webapi/find`.
Additional hosts are instructed to update as earlier updates complete, timeout, or fail, never exceeding `max_in_flight`.
The group rollout is halted if timeouts or failures exceed their specified thresholds.
Group rollouts may be retried with `tctl autoupdate run`.

### Scalability

Instance heartbeats will now be cached at both the auth server and the proxy.

All rollout logic is trigger by instance heartbeat backend writes, as changes can only occur on these events.
The following data related to the rollout are stored in each instance heartbeat:
- `agent_upgrade_start_time`: timestamp of individual agent's upgrade time
- `agent_upgrade_group_schedule`: schedule type of group (e.g., critical)
- `agent_upgrade_group_name`: name of group (e.g., staging)
- `agent_upgrade_group_start_time`: timestamp of current window start time
- `agent_upgrade_group_end_time`: timestamp of current window start time

At the start of the window, all queried instance heartbeats are marked with updated values for the `agent_upgrade_group_*` fields.
Instance heartbeats are included in the current window if all three fields match the window defined in `cluster_maintenance_config`.

On each instance heartbeat write, the auth server looks at instance heartbeats in cache and determines if additional agents should be upgrading.
If they should, additional instance heartbeats are marked as upgrading by setting `agent_upgrade_start_time` to the current time.
When `agent_upgrade_start_time` is in the group's window, the proxy serves `agent_auto_upgrade: true` when queried via `/v1/webapi/find`.

To avoid synchronization issues between auth servers, the rollout order is deterministically sorted by UUID.
Two concurrent writes to different auth servers may temporarily result in fewer upgrading instances than desired, but this should be resolved on the next write.

Upgrading all agents generates the following write load:
- One write of `agent_upgrade_group_*` fields per agent
- One write of `agent_upgrade_start_time` field per agent

All reads are from cache.

### Endpoints

`/v1/webapi/find?host=[host_uuid]`
```json
{
  "server_edition": "enterprise",
  "agent_version": "15.1.1",
  "agent_auto_update": true,
  "agent_update_jitter_seconds": 10
}
```
Notes:
- The Teleport proxy uses `cluster_maintenance_config` and `autoupdate_config` (below) to determine the time when the served `agent_auto_update` is `true` for the provided host UUID.
- Agents will only upgrade if `agent_auto_update` is `true`, but new installations will use `agent_version` regardless of the value in `agent_auto_update`.
- The edition served is the cluster edition (enterprise, enterprise-fips, or oss), and cannot be configured.
- The host UUID is ready from `/var/lib/teleport` by the updater.

### Teleport Resources

```yaml
kind: cluster_maintenance_config
spec:
  # agent_auto_update allows turning agent updates on or off at the
  # cluster level. Only turn agent automatic updates off if self-managed
  # agent updates are in place.
  agent_auto_update: true|false

  # agent_auto_update_groups contains both "regular" and "critical" schedules.
  # The schedule used is determined by the agent_version_schedule associated
  # with the version in autoupdate_version.
  # Groups are not configurable with the "immediate" schedule.
  agent_auto_update_groups:
    # schedule is "regular" or "critical"
    regular:
    - name: staging-group
      # agents defines which agents are included in the group.
      agents:
        # node_labels_expression selects agents by SSH resource query.
        #  default: all connected agents
        node_labels_expression: 'labels["environment"]=="staging"'
        # instance_labels_expression selects agents by instance query.
        #  default: all connected agents
        instance_labels_expression: 'labels["environment"]=="staging"'
      # days specifies the days of the week when the group may be upgraded.
      #  default: ["*"] (all days)
      days: [“Sun”, “Mon”, ... | "*"]
      # start_hour specifies the hour when the group may start upgrading.
      #  default: 0
      start_hour: 0-23
      # jitter_seconds specifies a maximum jitter duration after the start hour.
      # The agent upgrader client will pick a random time within this duration to wait to upgrade.
      #  default: 0
      jitter_seconds: 0-60
      # max_in_flight specifies the maximum number of agents that may be upgraded at the same time.
      #  default: 100%
      max_in_flight: 0-100%
      # timeout_seconds specifies the amount of time, after the specified jitter, after which
      # an agent upgrade will be considered timed out if the version does not change.
      #  default: 60
      timeout_seconds: 30-900
      # failure_seconds specifies the amount of time after which an agent upgrade will be considered
      # failed if the agent heartbeat stops before the upgrade is complete.
      #  default: 0
      failure_seconds: 0-900
      # max_failed_before_halt specifies the percentage of clients that may fail before this group
      # and all dependent groups are halted.
      #  default: 0
      max_failed_before_halt: 0-100%
      # max_timeout_before_halt specifies the percentage of clients that may time out before this group
      # and all dependent groups are halted.
      #  default: 10%
      max_timeout_before_halt: 0-100%
      # requires specifies groups that must pass with the current version before this group is allowed
      # to run using that version.
      requires: ["test-group"]
  # ...
```

Note that cycles and dependency chains longer than a week will be rejected.
Otherwise, updates could take up to 7 weeks to propagate.

Changing the version or schedule completely resets progress.
Releasing new client versions multiple times a week has the potential to starve dependent groups from updates.

Note the MVP version of this resource will not support host UUIDs, groups, or backpressure, and will use the following simplified UX with `agent_auto_update` field.
This field will remain indefinitely, to cover agents that do not present a known host UUID, as well as connected agents that are not matched to a group.

```yaml
kind: cluster_maintenance_config
spec:
  # ...

  # agent_auto_update contains "regular," "critical," and "immediate" schedules.
  # The schedule used is determined by the agent_version_schedule associated
  # with the version in autoupdate_version.
  agent_auto_update:
    # The immediate schedule results in all agents updating simultaneously.
    # Only client-side jitter is configurable.
    immediate:
      # jitter_seconds specifies a maximum jitter duration after the start hour.
      # The agent upgrader client will pick a random time within this duration to wait to upgrade.
      #  default: 0
      jitter_seconds: 0-60
    regular: # or "critical"
      # days specifies the days of the week when the group may be upgraded.
      #  default: ["*"] (all days)
      days: [“Sun”, “Mon”, ... | "*"]
      # start_hour specifies the hour when the group may start upgrading.
      #  default: 0
      start_hour: 0-23
      # jitter_seconds specifies a maximum jitter duration after the start hour.
      # The agent upgrader client will pick a random time within this duration to wait to upgrade.
      #  default: 0
      jitter_seconds: 0-60
  # ...
```


```shell
# configuration
$ tctl autoupdate update--set-agent-auto-update=off
Automatic updates configuration has been updated.
$ tctl autoupdate update --schedule regular --group staging-group --set-start-hour=3
Automatic updates configuration has been updated.
$ tctl autoupdate update --schedule regular --group staging-group --set-jitter-seconds=600
Automatic updates configuration has been updated.
$ tctl autoupdate reset
Automatic updates configuration has been reset to defaults.

# status
$ tctl autoupdate status
Status: disabled
Version: v1.2.4
Schedule: regular

Groups:
staging-group: succeeded at 2024-01-03 23:43:22 UTC
prod-group: scheduled for 2024-01-03 23:43:22 UTC (depends on prod-group)
other-group: failed at 2024-01-05 22:53:22 UTC

$ tctl autoupdate status --group staging-group
Status: succeeded
Date: 2024-01-03 23:43:22 UTC
Requires: (none)

Upgraded: 230 (95%)
Unchanged: 10 (2%)
Failed: 15 (3%)
Timed-out: 0

# re-running failed group
$ tctl autoupdate run --group staging-group
Executing auto-update for group 'staging-group' immediately.
```

```yaml
kind: autoupdate_version
spec:
  # agent_version is the version of the agent the cluster will advertise.
  agent_version: X.Y.Z
  # agent_version_schedule specifies the rollout schedule associated with the version.
  # Currently, only critical, regular, and immediate schedules are permitted.
  agent_version_schedule: regular|critical|immediate

  # ...
```

```shell
$ tctl autoupdate update --set-agent-version=15.1.1
Automatic updates configuration has been updated.
$ tctl autoupdate update --set-agent-version=15.1.2 --critical
Automatic updates configuration has been updated.
```

Notes:
- These two resources are separate so that Cloud customers can be restricted from updating `autoupdate_version`, while maintaining control over the rollout.

## Details - Linux Agents

We will ship a new auto-updater package for Linux servers written in Go that does not interface with the system package manager.
It will be distributed as a separate package from Teleport, and manage the installation of the correct Teleport agent version manually.
It will read the unauthenticated `/v1/webapi/find` endpoint from the Teleport proxy, parse new fields on that endpoint, and install the specified agent version according to the specified upgrade plan.
It will download the correct version of Teleport as a tarball, unpack it in `/var/lib/teleport`, and ensure it is symlinked from `/usr/local/bin`.

Source code for the updater will live in `integrations/updater`.

### Installation

```shell
$ apt-get install teleport-ent-updater
$ teleport-update enable --proxy example.teleport.sh

# if not enabled already, configure teleport and:
$ systemctl enable teleport
```

### Filesystem

```
$ tree /var/lib/teleport
/var/lib/teleport
└── versions
   ├── 15.0.0
   │  ├── bin
   │  │  ├── tsh
   │  │  ├── tbot
   │  │  ├── ... # other binaries
   │  │  ├── teleport-updater
   │  │  └── teleport
   │  ├── etc
   │  │  └── systemd
   │  │     └── teleport.service
   │  └── backup
   │     ├── sqlite.db
   │     └── backup.yaml
   ├── 15.1.1
   │  ├── bin
   │  │  ├── tsh
   │  │  ├── tbot
   │  │  ├── ... # other binaries
   │  │  ├── teleport-updater
   │  │  └── teleport
   │  └── etc
   │     └── systemd
   │        └── teleport.service
   └── updates.yaml
$ ls -l /usr/local/bin/tsh
/usr/local/bin/tsh -> /var/lib/teleport/versions/15.0.0/bin/tsh
$ ls -l /usr/local/bin/tbot
/usr/local/bin/tbot -> /var/lib/teleport/versions/15.0.0/bin/tbot
$ ls -l /usr/local/bin/teleport
/usr/local/bin/teleport -> /var/lib/teleport/versions/15.0.0/bin/teleport
$ ls -l /usr/local/bin/teleport-updater
/usr/local/bin/teleport-updater -> /var/lib/teleport/versions/15.0.0/bin/teleport-updater
$ ls -l /usr/local/lib/systemd/system/teleport.service
/usr/local/lib/systemd/system/teleport.service -> /var/lib/teleport/versions/15.0.0/etc/systemd/teleport.service
```

updates.yaml:
```
version: v1
kind: agent_versions
spec:
  # proxy specifies the Teleport proxy address to retrieve the agent version and update configuration from.
  proxy: mytenant.teleport.sh
  # enabled specifies whether auto-updates are enabled, i.e., whether teleport-updater update is allowed to update the agent.
  enabled: true
  # active_version specifies the active (symlinked) deployment of the telepport agent.
  active_version: 15.1.1
```

backup.yaml:
```
version: v1
kind: db_backup
spec:
  # proxy address from the backup
  proxy: mytenant.teleport.sh
  # version from the backup
  version: 15.1.0
  # time the backup was created
  creation_time: 2020-12-09T16:09:53+00:00
```

### Runtime

The agent-updater will run as a periodically executing systemd service which runs every 10 minutes.
The systemd service will run:
```shell
$ teleport-updater update
```

After it is installed, the `update` subcommand will no-op when executed until configured with the `teleport-updater` command:
```shell
$ teleport-updater enable --proxy mytenant.teleport.sh
```

If the proxy address is not provided with `--proxy`, the current proxy address from `teleport.yaml` is used.

The `enable` subcommand will change the behavior of `teleport-update update` to update teleport and restart the existing agent, if running.
It will also run update teleport immediately, to ensure that subsequent executions succeed.

Both `update` and `enable` will maintain a shared lock file preventing any re-entrant executions.

The `enable` subcommand will:
1. Query the `/v1/webapi/find` endpoint.
2. If the current updater-managed version of Teleport is the latest, and teleport package is not installed, jump to (16).
3. If the current updater-managed version of Teleport is the latest, but the teleport package is installed, jump to (13).
4. Ensure there is enough free disk space to upgrade Teleport via `df .`.
5. Download the desired Teleport tarball specified by `agent_version` and `server_edition`.
6. Download and verify the checksum (tarball URL suffixed with `.sha256`).
7. Extract the tarball to `/var/lib/teleport/versions/VERSION` and write the SHA to `/var/lib/teleport/versions/VERSION/sha256`.
8. Replace any existing binaries or symlinks with symlinks to the current version.
9. Backup `/var/lib/teleport/proc/sqlite.db` into `/var/lib/teleport/versions/OLD-VERSION/backup/sqlite.db` and create `backup.yaml`.
10. Restart the agent if the systemd service is already enabled.
11. Set `active_version` in `updates.yaml` if successful or not enabled.
12. Replace the symlinks/binaries and `/var/lib/teleport/proc/sqlite.db` and quit (exit 1) if unsuccessful.
13. Remove and purge any `teleport` package if installed.
14. Verify the symlinks to the active version still exists.
15. Remove all stored versions of the agent except the current version and last working version.
16. Configure `updates.yaml` with the current proxy address and set `enabled` to true.

The `disable` subcommand will:
1. Configure `updates.yaml` to set `enabled` to false.

When `update` subcommand is otherwise executed, it will:
1. Check `updates.yaml`, and quit (exit 0) if `enabled` is false, or quit (exit 1) if `enabled` is true and no proxy address is set.
2. Query the `/v1/webapi/find` endpoint.
3. Check that `agent_auto_updates` is true, quit otherwise.
4. If the current version of Teleport is the latest, quit.
5. Wait `random(0, agent_update_jitter_seconds)` seconds.
6. Ensure there is enough free disk space to upgrade Teleport via `df .`.
7. Download the desired Teleport tarball specified by `agent_version` and `server_edition`.
8. Download and verify the checksum (tarball URL suffixed with `.sha256`).
9. Extract the tarball to `/var/lib/teleport/versions/VERSION` and write the SHA to `/var/lib/teleport/versions/VERSION/sha256`.
10. Update symlinks to point at the new version.
11. Backup `/var/lib/teleport/proc/sqlite.db` into `/var/lib/teleport/versions/OLD-VERSION/backup/sqlite.db` and create `backup.yaml`.
12. Restart the agent if the systemd service is already enabled.
13. Set `active_version` in `updates.yaml` if successful or not enabled.
14. Replace the old symlinks/binaries and `/var/lib/teleport/proc/sqlite.db` and quit (exit 1) if unsuccessful.
15. Remove all stored versions of the agent except the current version and last working version.

To enable auto-updates of the updater itself, all commands will first check for an `active_version`, and reexec using the `teleport-updater` at that version if present and different.
The `/usr/local/bin/teleport-updater` symlink will take precedence to avoid reexec in most scenarios.

To ensure that SELinux permissions do not prevent the `teleport-updater` binary from installing/removing Teleport versions, the updater package will configure SELinux contexts to allow changes to all required paths.

To ensure that `teleport` package removal does not interfere with `teleport-updater`, package removal will run `apt purge` (or `yum` equivalent) while ensuring that `/etc/teleport.yaml` and `/var/lib/teleport` are not purged.
Failure to do this could result in `/etc/teleport.yaml` being removed when an operator runs `apt purge` at a later date.

To ensure that backups are consistent, the updater will use the [SQLite backup API](https://www.sqlite.org/backup.html) to perform the backup.

#### Failure Conditions

If the new version of Teleport fails to start, the installation of Teleport is reverted as described above.

If `teleport-updater` itself fails with an error, and an older version of `teleport-updater` is available, the upgrade will retry with the older version.

Known failure conditions caused by intentional configuration (e.g., upgrades disabled) will not trigger retry logic.

#### Status

To retrieve known information about agent upgrades, the `status` subcommand will return the following:
```json
{
  "agent_version_installed": "15.1.1",
  "agent_version_desired": "15.1.2",
  "agent_version_previous": "15.1.0",
  "agent_edition_installed": "enterprise",
  "agent_edition_desired": "enterprise",
  "agent_edition_previous": "enterprise",
  "agent_update_time_last": "2020-12-10T16:00:00+00:00",
  "agent_update_time_jitter": 600,
  "agent_updates_enabled": true
}
```

### Downgrades

Downgrades may be necessary in cases where we have rolled out a bug or security vulnerability with critical impact.
To initiate a downgrade, `agent_version` is set to an older version than it was previously set to.

Downgrades are challenging, because `sqlite.db` used by newer version of Teleport may not be valid for older versions of Teleport.

When Teleport is downgraded to a previous version that has a backup of `sqlite.db` present in `/var/lib/teleport/versions/OLD-VERSION/backup/`:
1. `/var/lib/teleport/versions/OLD-VERSION/backup/backup.yaml` is validated to determine if the backup is usable (proxy and version must match, age must be less than cert lifetime, etc.)
2. If the backup is valid, Teleport is fully stopped, the backup is restored along with symlinks, and the downgraded version of Teleport is started.
3. If the backup is invalid, we refuse to downgrade.

Downgrades are applied with `teleport-updater update`, just like upgrades.
The above steps modulate the standard workflow in the section above.
If the downgraded version is already present, the uncompressed version is used to ensure fast recovery of the exact state before the failed upgrade.
To ensure that the target version is was not corrupted by incomplete extraction, the downgrade checks for the existance of `/var/lib/teleport/versions/TARGET-VERSION/sha256` before downgrading.
To ensure that the DB backup was not corrupted by incomplete copying, the downgrade checks for the existance of `/var/lib/teleport/versions/TARGET-VERSION/backup/backup.yaml` before restoring.

Teleport must be fully-stopped to safely replace `sqlite.db`.
When restarting the agent during an upgrade, `SIGHUP` is used.
When restarting the agent during a downgrade, `systemd stop/start` are used before/after the downgrade.

Teleport CA certificate rotations will break rollbacks.
In the future, this could be addressed with additional validation of the agent's client certificate issuer fingerprints.
For now, rolling forward will allow recovery from a broken rollback.

Given that rollbacks may fail, we must maintain the following invariants:
1. Broken rollbacks can always be reverted by reversing the rollback exactly.
2. Broken versions can always be reverted by rolling back and then skipping the broken version.

When rolling forward, the backup of the newer version's `sqlite.db` is only restored if that exact version is the roll-forward version.
Otherwise, the older, rollback version of `sqlite.db` is preserved (i.e., the newer version's backup is not used).
This ensures that a version upgrade which broke the database can be recovered with a rollback and a new patch.
It also ensures that a broken rollback is always recoverable by reversing the rollback.

Example: Given v1, v2, v3 versions of Teleport, where v2 is broken:
1. v1 -> v2 -> v1 -> v3 => DB from v1 is migrated directly to v3, avoiding v2 breakage.
2. v1 -> v2 -> v1 -> v2 -> v3 => DB from v2 is recovered, in case v1 database no longer has a valid certificate.

### Manual Workflow

For use cases that fall outside of the functionality provided by `teleport-updater`, we provide an alternative manual workflow using the `/v1/webapi/find` endpoint.
This workflow supports customers that cannot use the auto-update mechanism provided by `teleport-updater` because they use their own automation for updates (e.g., JamF or Ansible).

Cluster administrators that want to self-manage agent updates may manually query the `/v1/webapi/find` endpoint using the host UUID, and implement auto-updates with their own automation.

### Installers

The following install scripts will be updated to install the latest updater and run `teleport-updater enable` with the proxy address:
- [/api/types/installers/agentless-installer.sh.tmpl](https://github.com/gravitational/teleport/blob/d0a68fd82412b48cb54f664ae8500f625fb91e48/api/types/installers/agentless-installer.sh.tmpl)
- [/api/types/installers/installer.sh.tmpl](https://github.com/gravitational/teleport/blob/d0a68fd82412b48cb54f664ae8500f625fb91e48/api/types/installers/installer.sh.tmpl)
- [/lib/web/scripts/oneoff/oneoff.sh](https://github.com/gravitational/teleport/blob/d0a68fd82412b48cb54f664ae8500f625fb91e48/lib/web/scripts/oneoff/oneoff.sh)
- [/lib/web/scripts/node-join/install.sh](https://github.com/gravitational/teleport/blob/d0a68fd82412b48cb54f664ae8500f625fb91e48/lib/web/scripts/node-join/install.sh)
- [/assets/aws/files/install-hardened.sh](https://github.com/gravitational/teleport/blob/d0a68fd82412b48cb54f664ae8500f625fb91e48/assets/aws/files/install-hardened.sh)

Eventually, additional logic from the scripts could be added to `teleport-updater`, such that `teleport-updater` can configure teleport.

Moving additional logic into the upgrader is out-of-scope for this proposal.

To create pre-baked VM or container images that reduce the complexity of the cluster joining operation, two workflows are permitted:
- Install the `teleport-updater` package and defer `teleport-updater enable`, Teleport configuration, and `systemctl enable teleport` to cloud-init scripts.
  This allows both the proxy address and token to be injected at VM initialization. The VM image may be used with any Teleport cluster.
  Installers scripts will continue to function, as the package install operation will no-op.
- Install the `teleport-updater` package and run `teleport-updater enable` before the image is baked, but defer final Teleport configuration and `systemctl enable teleport` to cloud-init scripts.
  This allows the proxy address to be pre-set in the image. `teleport.yaml` can be partially configured during image creation. At minimum, the token must be injected via cloud-init scripts.
  Installers scripts would be skipped in favor of the `teleport configure` command.

It is possible for a VM or container image to be created with a baked-in join token.
We should recommend against this workflow for security reasons, since a long-lived token improperly stored in an image could be leaked.

Alternatively, users may prefer to skip pre-baked agent configuration, and run one of the script-based installers to join VMs to the cluster after the VM is started.

Documentation should be created covering the above workflows.

### Documentation

The following documentation will need to be updated to cover the new upgrader workflow:
- https://goteleport.com/docs/choose-an-edition/teleport-cloud/downloads
- https://goteleport.com/docs/installation
- https://goteleport.com/docs/upgrading/self-hosted-linux
- https://goteleport.com/docs/upgrading/self-hosted-automatic-agent-updates

Additionally, the Cloud dashboard tenants downloads tab will need to be updated to reference the new instructions.

## Details - Kubernetes Agents

The Kubernetes agent updater will be updated for compatibility with the new scheduling system.

This means that it will stop reading upgrade windows using the authenticated connection to the proxy, and instead upgrade when indicated by the `/v1/webapi/find` endpoint.

Rollbacks for the Kubernetes updater, as well as packaging changes to improve UX and compatibility, will be covered in a future RFD.

## Migration

The existing update scheduling system will remain in-place until the old auto-updater is fully deprecated.

## Security

The initial version of automatic updates will rely on TLS to establish
connection authenticity to the Teleport download server. The authenticity of
assets served from the download server is out of scope for this RFD. Cluster
administrators concerned with the authenticity of assets served from the
download server can use self-managed updates with system package managers which
are signed.

The Upgrade Framework (TUF) will be used to implement secure updates in the future.

Anyone who possesses a host UUID can determine when that host is scheduled to upgrade by repeatedly querying the public `/v1/webapi/find` endpoint.
It is not possible to discover the current version of that host, only the designated upgrade window.

## Logging

All installation steps will be logged locally, such that they are viewable with `journalctl`.
Care will be taken to ensure that updater logs are sharable with Teleport Support for debugging and auditing purposes.

When TUF is added, that events related to supply chain security may be sent to the Teleport cluster via the Teleport Agent.

## Execution Plan

1. Implement Teleport APIs for new scheduling system (without groups and backpressure)
2. Implement new auto-updater in Go.
3. Implement changes to Kubernetes auto-updater.
4. Test extensively on all supported Linux distributions.
5. Prep documentation changes.
6. Release new updater via teleport-ent-updater package.
7. Release documentation changes.
8. Communicate to select Cloud customers that they must update their updater, starting with lower ARR customers.
9. Communicate to all Cloud customers that they must update their updater.
10. Deprecate old auto-updater endpoints.
11. Add groups and backpressure features.
