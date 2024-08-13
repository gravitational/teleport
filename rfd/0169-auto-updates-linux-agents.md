---
authors: Stephen Levine (stephen.levine@goteleport.com)
state: draft
---

# RFD 0169 - Automatic Updates for Agents

## Required Approvers

* Engineering: @russjones
* Product: @klizhentas || @xinding33 
* Security: Doyensec

## What

This RFD proposes a new mechanism for scheduled, automatic updates of Teleport agents.

Users of Teleport will be able to use the tctl CLI to specify desired versions and update schedules.

All agent installations are in-scope for this proposal, including agents installed on Linux servers and Kubernetes.

The following anti-goals are out-of-scope for this proposal, but will be addressed in future RFDs:
- Signing of agent artifacts (e.g., via TUF)
- Teleport Cloud APIs for updating agents
- Improvements to the local functionality of the Kubernetes agent for better compatibility with FluxCD and ArgoCD
- Support for progressive rollouts to different groups of ephemeral or auto-scaling agents (see: Version Promotion)

This RFD proposes a specific implementation of several sections in https://github.com/gravitational/teleport/pull/39217.

Additionally, this RFD parallels the auto-update functionality for client tools proposed in https://github.com/gravitational/teleport/pull/39805.

## Why

The existing mechanism for automatic agent updates does not provide a hands-off experience for all Teleport users.

1. The use of system package management leads to interactions with `apt upgrade`, `yum upgrade`, etc. that can result in unintentional upgrades.
2. The use of system package management requires logic that varies significantly by target distribution.
3. The installation mechanism requires 4-5 commands, includes manually installing multiple packages, and varies depending on your version and edition of Teleport.
4. The use of bash to implement the updater makes changes difficult and prone to error.
5. The existing auto-updater has limited automated testing.
6. The use of GPG keys in system package managers has key management implications that we would prefer to solve with TUF in the future.
7. The desired agent version cannot be set via Teleport's operator-targeted CLI (tctl).
8. The rollout plan for new agent versions is not fully-configurable using tctl.
9. Agent installation logic is spread between the auto-updater script, install script, auto-discovery script, and documentation.
10. Teleport contains logic that is specific to Teleport Cloud upgrade workflows.
11. The existing auto-updater is not self-updating.
12. It is difficult and undocumented to automate agent upgrades with custom automation (e.g., with JamF). 
13. There is no phased rollout mechanism for updates.
14. There is no way to automatically detect and halt failed updates.

We must provide a seamless, hands-off experience for auto-updates of Teleport Agents that is easy to maintain and safer for production use.

## Details - Teleport API

Teleport proxies will be updated to serve the desired agent version and edition from `/v1/webapi/find`.
The version and edition served from that endpoint will be configured using new `autoupdate_version` resource.

Whether the Teleport updater querying the endpoint is instructed to upgrade (via the `agent_autoupdate` field) is dependent on:
- The `host=[uuid]` parameter sent to `/v1/webapi/find`
- The schedule defined in the new `cluster_autoupdate_config` resource
- The status of past agent upgrades for the given version

To ensure that the updater is always able to retrieve the desired version, instructions to the updater are delivered via unauthenticated requests to `/v1/webapi/find`.
Teleport auth servers use their access to heartbeat data to drive the rollout, while Teleport proxies modulate the `/v1/webapi/find` response given the host UUID.

Rollouts are specified as interdependent groups of hosts, selected by upgrade group identifier specified in the agent's `teleport.yaml` file.
```
teleport:
  upgrade_group: staging
```

At the start of a group rollout, the Teleport auth server captures the desired group of hosts to update in the backend.
An fixed number of hosts (`max_in_flight % x total`) are instructed to upgrade at the same time via `/v1/webapi/find`.
Additional hosts are instructed to update as earlier updates complete, timeout, or fail, never exceeding `max_in_flight`.
The group rollout is halted if timeouts or failures exceed their specified thresholds.
Rollouts may be retried with `tctl autoupdate run`.

### Scalability

#### Window Capture

Instance heartbeats will be cached by auth servers using a dedicated cache.
This cache is updated using rate-limited backend reads that occur in the background, to avoid mass-reads of instance heartbeats.
The rate is modulated by the total number of instance heartbeats, to avoid putting too much load on the backend on large clusters.
The cache is considered healthy when all instance heartbeats present on the backend have been read within a time period that is also modulated by the total number of heartbeats.

At the start of the upgrade window, auth servers attempt to write an update rollout plan to the backend under a single key.
This plan is protected by optimistic locking, and contains the following data:

Data key: `/autoupdate/[name of group](/[page-id])` (e.g., `/autoupdate/staging/8745823`)

Data value JSON:
- `start_time`: timestamp of current window start time
- `version`: version for which this rollout is valid
- `schedule`: type of schedule that triggered the rollout
- `hosts`: list of host UUIDs in randomized order
- `next_page`: additional UUIDs, if list is greater than 100,000 UUIDs

Expiration time of each key is 2 weeks.

At a fixed interval, auth servers will check the plan to determine if a new plan is needed by comparing `start_time` to the current time and the desired window.
If a new plan is needed, auth servers will query their cache of instance heartbeats and attempt to write the new plan.
The first auth server to write the plan wins; others will be rejected by the optimistic lock.
Auth servers will only write the plan if their instance heartbeat cache is healthy.

If the resource size is greater than 100 KiB, auth servers will divide the resource into pages no greater than 100 KiB each.
This is necessary to support backends with a value size limit.

Each page will duplicate all values besides `hosts`, which will be different for each page.
All pages besides the first page will be suffixed with a randomly generated number.
Pages will be written in reverse order, in a linked-link, before the final atomic non-suffixed write of the first page.
If the non-suffixed write fails, the auth server is responsible for cleaning up the unusable pages.
If cleanup fails, the unusable pages will expire after 2 weeks.

```
Winning auth:
  WRITE: /autoupdate/staging/4324234 | next_page: null
  WRITE: /autoupdate/staging/8745823 | next_page: 4324234
  WRITE: /autoupdate/staging | next_page: 8745823

Losing auth:
  WRITE: /autoupdate/staging/2342343 | next_page: null
  WRITE: /autoupdate/staging/7678686 | next_page: 2342343
  WRITE CONFLICT: /autoupdate/staging | next_page: 7678686
  DELETE: /autoupdate/staging/7678686
  DELETE: /autoupdate/staging/2342343
```

#### Rollout

The rollout logic is progressed by instance heartbeat backend writes, as changes can only occur on these events.

The following data related to the rollout are stored in each instance heartbeat:
- `agent_upgrade_start_time`: timestamp of individual agent's upgrade time
- `agent_upgrade_version`: current agent version

Expiration time of the heartbeat is extended to 24 hours when `agent_upgrade_start_time` is written.

Additionally, an in-memory data structure is maintained based on the cache, and kept up-to-date by a background process.
This data structure contains the number of unfinished (pending and ongoing) upgrades preceding each instance heartbeat in the rollout plan.
Instance heartbeats are considered completed when either `agent_upgrade_version` matches the plan version, or `agent_upgrade_start_time` is past the expiration time.
```golang
unfinished := make(map[Rollout][UUID]int)
```

On each instance heartbeat write, the auth server looks at the data structure to determine if the associated agent should begin upgrading.
This determination is made by comparing the stored number of unfinished upgrades to `max_in_flight % x len(hosts)`.
If the stored number is fewer, `agent_upgrade_start_time` is updated to the current time when the heartbeat is written.

The auth server writes the following keys to `/autoupdate/[name of group]/status` (e.g., `/autoupdate/staging/status`):
- `last_active_host_index`: index of the last host allowed to upgrade
- `failed_host_count`: failed host count
- `timeout_host_count`: timed-out host count

Writes are rate-limited such that the progress is only updated every 10 seconds.
If the auth server's cached progress is greater than its calculated progress, the auth server declines to update the progress.

The predetermined ordering of hosts avoids cache synchronization issues between auth servers.
Two concurrent heartbeat writes may temporarily result in fewer upgrading instances than desired, but this will eventually be resolved by cache propagation.

Each group rollout is represented by an `agent_rollout_plan` Teleport resource that includes the progress and host count, but not the list of UUIDs.
Proxies use the start time in the resource to determine when to stream the list of UUIDs via a dedicated RPC.
Proxies watch the status section of `agent_rollout_plan` for updates to progress.

Proxies read all started rollouts and maintain an in-memory map of host UUID to upgrading status:
```golang
upgrading := make(map[UUID]bool)
```
Proxies watch for changes to the plan and update the map accordingly.

When the updater queries the proxy via `/v1/webapi/find?host=[host_uuid]`, the proxies query the map to determine the value of `agent_auto_upgrade: true`.

Upgrading all agents generates the following additional backend write load:
- One write per page of the rollout plan per upgrade group.
- One write per auth server every 10 seconds, during rollouts.

### REST Endpoints

`/v1/webapi/find?host=[host_uuid]`
```json
{
  "server_edition": "enterprise",
  "agent_version": "15.1.1",
  "agent_autoupdate": true,
  "agent_update_jitter_seconds": 10
}
```
Notes:
- Agents will only upgrade if `agent_autoupdate` is `true`, but new installations will use `agent_version` regardless of the value in `agent_autoupdate`.
- The edition served is the cluster edition (enterprise, enterprise-fips, or oss), and cannot be configured.
- The host UUID is read from `/var/lib/teleport/host_uuid` by the updater.

### Teleport Resources

#### Scheduling

```yaml
kind: cluster_autoupdate_config
spec:
  # agent_autoupdate allows turning agent updates on or off at the
  # cluster level. Only turn agent automatic updates off if self-managed
  # agent updates are in place.
  agent_autoupdate: true|false

  # agent_group_schedules contains both "regular" and "critical" schedules.
  # The schedule used is determined by the agent_version_schedule associated
  # with the version in autoupdate_version.
  # Groups are not configurable with the "immediate" schedule.
  agent_group_schedules:
    # schedule is "regular" or "critical"
    regular:
      # name of the group
    - name: staging-group
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
      # timeout_seconds specifies the amount of time, after the specified jitter, after which
      # an agent upgrade will be considered timed out if the version does not change.
      #  default: 60
      timeout_seconds: 30-900
      # failure_seconds specifies the amount of time after which an agent upgrade will be considered
      # failed if the agent heartbeat stops before the upgrade is complete.
      #  default: 0
      failure_seconds: 0-900
      # max_in_flight specifies the maximum number of agents that may be upgraded at the same time.
      #  default: 100%
      max_in_flight: 0-100%
      # max_timeout_before_halt specifies the percentage of clients that may time out before this group
      # and all dependent groups are halted.
      #  default: 10%
      max_timeout_before_halt: 0-100%
      # max_failed_before_halt specifies the percentage of clients that may fail before this group
      # and all dependent groups are halted.
      #  default: 0
      max_failed_before_halt: 0-100%
      # requires specifies groups that must pass with the current version before this group is allowed
      # to run using that version.
      requires: ["test-group"]
  # ...
```

Dependency cycles are rejected.
Dependency chains longer than a week will be rejected.
Otherwise, updates could take up to 7 weeks to propagate.

The updater will receive `agent_autoupdate: true` from the time is it designated for upgrade until the version changes in `autoupdate_version`.
After 24 hours, the upgrade is halted in-place, and the group is considered failed if unfinished.

Changing the version or schedule completely resets progress.
Releasing new client versions multiple times a week has the potential to starve dependent groups from updates.

Note the MVP version of this resource will not support host UUIDs, groups, or backpressure, and will use the following simplified UX with `agent_default_schedules` field.
This field will remain indefinitely to cover connected agents that are not matched to a group.

```yaml
kind: cluster_autoupdate_config
spec:
  # agent_autoupdate allows turning agent updates on or off at the
  # cluster level. Only turn agent automatic updates off if self-managed
  # agent updates are in place.
  agent_autoupdate: true|false

  # agent_default_schedules contains "regular," "critical," and "immediate" schedules.
  # These schedules apply to agents not scheduled by agent_group_schedules.
  # The schedule used is determined by the agent_version_schedule associated
  # with the agent_version in the autoupdate_version resource.
  agent_default_schedules:
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

To allow `agent_default_schedules` and `agent_group_schedules` to co-exist, a reserved `agent_rollout_plan` named `default` will be employed.

```shell
# configuration
$ tctl autoupdate update--set-agent-auto-update=off
Automatic updates configuration has been updated.
$ tctl autoupdate update --schedule regular --group staging-group --set-start-hour=3
Automatic updates configuration has been updated.
$ tctl autoupdate update --schedule regular --group staging-group --set-jitter-seconds=60
Automatic updates configuration has been updated.
$ tctl autoupdate update --schedule regular --default --set-jitter-seconds=60
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
- `autoupdate_version` is separate from `cluster_autoupdate_config` so that Cloud customers can be restricted from updating `autoupdate_version`, while maintaining control over the rollout.

#### Rollout

```yaml
kind: agent_rollout_plan
spec:
  # start time of the rollout
  start_time: 0001-01-01T00:00:00Z
  # target version of the rollout
  version: X.Y.Z
  # schedule that triggered the rollout
  schedule: regular
  # hosts updated by the rollout
  host_count: 127
status:
  # current host index in rollout progress
  last_active_host_index: 23
  # failed hosts
  failed_host_count: 3
  # timed-out hosts
  timeout_host_count: 1
```

Notes:
- This resource is stored in a paginated format with separate keys for each page and progress

### Version Promotion

Maintaining the version of different groups of agents is out-of-scope for this RFD.
This means that groups which employ auto-scaling or ephemeral resources will slowly converge to the latest Teleport version.

**This could lead to a production outage, as the latest Teleport version may not receive any validation before it is advertised to newly provisioned resources in production.**

To solve this in the future, we can add an additional `--group` flag to `teleport-update`:
```shell
$ teleport-update enable --proxy example.teleport.sh --group staging-group
```

This group name could be provided as a parameter to `/v1/webapi/find`, so that newly added resources may install at the group's designated version.

This will require tracking the desired version of groups in the backend, which will add additional complexity to the rollout logic.

## Details - Linux Agents

We will ship a new auto-updater package for Linux servers written in Go that does not interface with the system package manager.
It will be distributed as a separate package from Teleport, and manage the installation of the correct Teleport agent version manually.
It will read the unauthenticated `/v1/webapi/find` endpoint from the Teleport proxy, parse new fields on that endpoint, and install the specified agent version according to the specified upgrade plan.
It will download the correct version of Teleport as a tarball, unpack it in `/var/lib/teleport`, and ensure it is symlinked from `/usr/local/bin`.

Source code for the updater will live in the main Teleport repository, with the updater binary built from `tools/teleport-update`.

### Installation

```shell
$ apt-get install teleport-ent-updater
$ teleport-update enable --proxy example.teleport.sh

# if not enabled already, configure teleport and:
$ systemctl enable teleport
```

For air-gapped Teleport installs, the agent may be configured with a custom tarball path template:
```shell
$ teleport-update enable --proxy example.teleport.sh --template 'https://example.com/teleport-{{ .Edition }}-{{ .Version }}-{{ .Arch }}.tgz'
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
4. Ensure there is enough free disk space to upgrade Teleport via `df .` and `content-length` header from `HEAD` request.
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
3. Check that `agent_autoupdates` is true, quit otherwise.
4. If the current version of Teleport is the latest, quit.
5. Wait `random(0, agent_update_jitter_seconds)` seconds.
6. Ensure there is enough free disk space to upgrade Teleport via `df .` and `content-length` header from `HEAD` request.
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

To ensure that `teleport` package removal does not lead to a hard restart of Teleport, the updater will ensure that the package is removed without triggering needrestart or similar services.

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

Eventually, the `cluster_maintenance_config` resource will be deprecated.

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

## Protobuf API Changes

Note: all updates use revisions to prevent data loss in case of concurrent access.

### clusterconfig/v1

```protobuf
syntax = "proto3";

package teleport.clusterconfig.v1;

option go_package = "github.com/gravitational/teleport/api/gen/proto/go/teleport/clusterconfig/v1;clusterconfigv1";

// ClusterConfigService provides methods to manage cluster configuration resources.
service ClusterConfigService {
  // ...

  // GetClusterAutoupdateConfig updates the cluster autoupdate config.
  rpc GetClusterAutoupdateConfig(GetClusterAutoupdateConfigRequest) returns (ClusterAutoupdateConfig);
  // CreateClusterAutoupdateConfig creates the cluster autoupdate config.
  rpc CreateClusterAutoupdateConfig(CreateClusterAutoupdateConfigRequest) returns (ClusterAutoupdateConfig);
  // UpdateClusterAutoupdateConfig updates the cluster autoupdate config.
  rpc UpdateClusterAutoupdateConfig(UpdateClusterAutoupdateConfigRequest) returns (ClusterAutoupdateConfig);
  // UpsertClusterAutoupdateConfig overwrites the cluster autoupdate config.
  rpc UpsertClusterAutoupdateConfig(UpsertClusterAutoupdateConfigRequest) returns (ClusterAutoupdateConfig);
  // ResetClusterAutoupdateConfig restores the cluster autoupdate config to default values.
  rpc ResetClusterAutoupdateConfig(ResetClusterAutoupdateConfigRequest) returns (ClusterAutoupdateConfig);
}

// GetClusterAutoupdateConfigRequest requests the contents of the ClusterAutoupdateConfig.
message GetClusterAutoupdateConfigRequest {}

// CreateClusterAutoupdateConfigRequest requests creation of the the ClusterAutoupdateConfig.
message CreateClusterAutoupdateConfigRequest {
  ClusterAutoupdateConfig cluster_autoupdate_config = 1;
}

// UpdateClusterAutoupdateConfigRequest requests an update of the the ClusterAutoupdateConfig.
message UpdateClusterAutoupdateConfigRequest {
  ClusterAutoupdateConfig cluster_autoupdate_config = 1;
}

// UpsertClusterAutoupdateConfigRequest requests an upsert of the the ClusterAutoupdateConfig.
message UpsertClusterAutoupdateConfigRequest {
  ClusterAutoupdateConfig cluster_autoupdate_config = 1;
}

// ResetClusterAutoupdateConfigRequest requests a reset of the the ClusterAutoupdateConfig to default values.
message ResetClusterAutoupdateConfigRequest {}

// ClusterAutoupdateConfig holds dynamic configuration settings for cluster maintenance activities.
message ClusterAutoupdateConfig {
  // kind is the kind of the resource.
  string kind = 1;
  // sub_kind is the sub kind of the resource.
  string sub_kind = 2;
  // version is the version of the resource.
  string version = 3;
  // metadata is the metadata of the resource.
  teleport.header.v1.Metadata metadata = 4;
  // spec is the spec of the resource.
  ClusterAutoupdateConfigSpec spec = 7;
}

// ClusterAutoupdateConfigSpec is the spec for the cluster autoupdate config.
message ClusterAutoupdateConfigSpec {
  // agent_autoupdate specifies whether agent autoupdates are enabled.
  bool agent_autoupdate = 1;
  // agent_default_schedules specifies schedules for upgrades of agents.
  //   not scheduled by agent_group_schedules.
  AgentAutoupdateDefaultSchedules agent_default_schedules = 2;
  // agent_group_schedules specifies schedules for upgrades of grouped agents.
  AgentAutoupdateGroupSchedules agent_group_schedules = 3;
}

// AgentAutoupdateDefaultSchedules specifies the default update schedules for non-grouped agent.
message AgentAutoupdateDefaultSchedules {
  // regular schedule for non-critical versions.
  AgentAutoupdateSchedule regular = 1;
  // critical schedule for urgently needed versions.
  AgentAutoupdateSchedule critical = 2;
  // immediate schedule for versions that must be deployed with no delay.
  AgentAutoupdateImmediateSchedule immediate = 3;
}

// AgentAutoupdateSchedule specifies a default schedule for non-grouped agents.
message AgentAutoupdateSchedule {
  // days to run update
  repeated Day days = 2;
  // start_hour to initiate update
  int32 start_hour = 3;
  // jitter_seconds to introduce before update as rand([0, jitter_seconds]).
  int32 jitter_seconds = 4;
}

// AgentAutoupdateSchedule specifies a default schedule for non-grouped agents on the immediate scehdule.
message AgentAutoupdateImmediateSchedule {
  // jitter to introduce before update as rand([0, jitter_seconds]).
  int32 jitter_seconds = 4;
}

// AgentAutoupdateGroupSchedules specifies update scheduled for grouped agents.
message AgentAutoupdateGroupSchedules {
  // regular schedules for non-critical versions.
  repeated AgentAutoupdateGroup regular = 1;
  // critical schedules for urgently needed versions.
  repeated AgentAutoupdateGroup critical = 2;
}

// AgentAutoupdateGroup specifies the update schedule for a group of agents.
message AgentAutoupdateGroup {
  // name of the group
  string name = 1;
  // days to run update
  repeated Day days = 2;
  // start_hour to initiate update
  int32 start_hour = 3;
  // jitter_seconds to introduce before update as rand([0, jitter_seconds]).
  int32 jitter_seconds = 4;
  // timeout_seconds before an agent is considered time-out (no version change)
  int32 timeout_seconds = 5;
  // failure_seconds before an agent is considered failed (loses connection)
  int32 failure_seconds = 6;
  // max_in_flight specifies agents that can be upgraded at the same time, by percent.
  string max_in_flight = 7;
  // max_timeout_before_halt specifies agents that can timeout before the rollout is halted, by percent.
  string max_timeout_before_halt = 8;
  // max_failed_before_halt specifies agents that can fail before the rollout is halted, by percent.
  string max_failed_before_halt = 9;
  // requires specifies rollout groups that must succeed for the current version/schedule before this rollout can run.
  repeated string requires = 10;
}

// Day of the week
enum Day {
  ALL = 0;
  SUNDAY = 1;
  MONDAY = 2;
  TUESDAY = 3;
  WEDNESDAY = 4;
  THURSDAY = 5;
  FRIDAY = 6;
  SATURDAY = 7;
}
```

### autoupdate/v1

```protobuf
syntax = "proto3";

package teleport.autoupdate.v1;

option go_package = "github.com/gravitational/teleport/api/gen/proto/go/teleport/autoupdate/v1;autoupdatev1";

// AutoupdateService serves agent and client automatic version updates.
service AutoupdateService {
  // GetAutoupdateVersion returns the autoupdate version.
  rpc GetAutoupdateVersion(GetAutoupdateVersionRequest) returns (AutoupdateVersion);
  // CreateAutoupdateVersion creates the autoupdate version.
  rpc CreateAutoupdateVersion(CreateAutoupdateVersionRequest) returns (AutoupdateVersion);
  // UpdateAutoupdateVersion updates the autoupdate version.
  rpc UpdateAutoupdateVersion(UpdateAutoupdateVersionRequest) returns (AutoupdateVersion);
  // UpsertAutoupdateVersion overwrites the autoupdate version.
  rpc UpsertAutoupdateVersion(UpsertAutoupdateVersionRequest) returns (AutoupdateVersion);

  // GetAgentRolloutPlan returns the agent rollout plan and current progress.
  rpc GetAgentRolloutPlan(GetAgentRolloutPlanRequest) returns (AgentRolloutPlan);
  // GetAutoupdateVersion streams the agent rollout plan's list of all hosts.
  rpc GetAgentRolloutPlanHosts(GetAgentRolloutPlanHostsRequest) returns (stream AgentRolloutPlanHost);
}

// GetAutoupdateVersionRequest requests the autoupdate_version singleton resource.
message GetAutoupdateVersionRequest {}

// GetAutoupdateVersionRequest requests creation of the autoupdate_version singleton resource.
message CreateAutoupdateVersionRequest {
  // autoupdate_version resource contents
  AutoupdateVersion autoupdate_version = 1;
}

// GetAutoupdateVersionRequest requests an update of the autoupdate_version singleton resource.
message UpdateAutoupdateVersionRequest {
  // autoupdate_version resource contents
  AutoupdateVersion autoupdate_version = 1;
}

// GetAutoupdateVersionRequest requests an upsert of the autoupdate_version singleton resource.
message UpsertAutoupdateVersionRequest {
  // autoupdate_version resource contents
  AutoupdateVersion autoupdate_version = 1;
}

// AutoupdateVersion holds dynamic configuration settings for autoupdate versions.
message AutoupdateVersion {
  // kind is the kind of the resource.
  string kind = 1;
  // sub_kind is the sub kind of the resource.
  string sub_kind = 2;
  // version is the version of the resource.
  string version = 3;
  // metadata is the metadata of the resource.
  teleport.header.v1.Metadata metadata = 4;
  // spec is the spec of the resource.
  AutoupdateVersionSpec spec = 6;
}

// AutoupdateVersionSpec is the spec for the autoupdate version.
message AutoupdateVersionSpec {
  // agent_version is the desired agent version for new rollouts.
  string agent_version = 1;
  // agent_version schedule is the schedule to use for rolling out the agent_version.
  Schedule agent_version_schedule = 2;
}

// Schedule type for the rollout
enum Schedule {
  // REGULAR update schedule
  REGULAR = 0;
  // CRITICAL update schedule for critical bugs and vulnerabilities
  CRITICAL = 1;
  // IMMEDIATE update schedule for updating all agents immediately
  IMMEDIATE = 2;
}

// GetAgentRolloutPlanRequest requests an agent_rollout_plan.
message GetAgentRolloutPlanRequest {
  // name of the agent_rollout_plan
  string name = 1;
}

// GetAgentRolloutPlanHostsRequest requests the ordered host UUIDs for an agent_rollout_plan.
message GetAgentRolloutPlanHostsRequest {
  // name of the agent_rollout_plan
  string name = 1;
}

// AgentRolloutPlan defines a version update rollout consisting a fixed group of agents.
message AgentRolloutPlan {
  // kind is the kind of the resource.
  string kind = 1;
  // sub_kind is the sub kind of the resource.
  string sub_kind = 2;
  // version is the version of the resource.
  string version = 3;
  // metadata is the metadata of the resource.
  teleport.header.v1.Metadata metadata = 4;
  // spec is the spec of the resource.
  AgentRolloutPlanSpec spec = 5;
  // status is the status of the resource.
  AgentRolloutPlanStatus status = 6;
}

// AutoupdateVersionSpec is the spec for the autoupdate version.
message AgentRolloutPlanSpec {
  // start_time of the rollout
  google.protobuf.Timestamp start_time = 1;
  // version targetted by the rollout
  string version = 2;
  // schedule that triggered the rollout
  string schedule = 3;
  // host_count of hosts to update
  int64 host_count = 4;
}

// AutoupdateVersionSpec is the spec for the autoupdate version.
message AgentRolloutPlanStatus {
  // last_active_host_index specifies the index of the last host that may be updated.
  int64 last_active_host_index = 1;
  // failed_host_count specifies the number of failed hosts.
  int64 failed_host_count = 2;
  // timeout_host_count specifies the number of timed-out hosts.
  int64 timeout_host_count = 3;
}

// AgentRolloutPlanHost identifies an agent by host ID
message AgentRolloutPlanHost {
  // host_id of a host included in the rollout
  string host_id = 1;
}
```

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
