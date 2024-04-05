---
authors: Stephen Levine (stephen.levine@goteleport.com)
state: draft
---

# RFD 0169 - Automatic Updates for Linux Agents

## Required Approvers

* Engineering: @russjones && @bernardjkim
* Product: @klizhentas || @xinding33 
* Security: @reedloden

## What

This RFD proposes a new mechanism for Teleport agents installed on Linux servers to automatically update to a version set by an operator via tctl.

The following anti-goals are out-of-scope for this proposal, but will be addressed in future RFDs:
- Analogous adjustments for Teleport agents installed on Kubernetes
- Phased rollouts of new agent versions for agents connected to an existing cluster
- Signing of agent artifacts via TUF
- Teleport Cloud APIs for updating agents

This RFD proposes a specific implementation of several sections in https://github.com/gravitational/teleport/pull/39217.

Additionally, this RFD parallels the auto-update functionality for client tools proposed in https://github.com/gravitational/teleport/pull/39805.

## Why

The existing mechanism for automatic agent updates does not provide a hands-off experience for all Teleport users.

1. The use of system package management leads to interactions with `apt upgrade`, `yum upgrade`, etc. that can result in unintentional upgrades or confusing command output.
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

## Details

We will ship a new auto-updater package written in Go that does not interface with the system package manager.
It will be versioned separately from Teleport, and manage the installation of the correct Teleport agent version manually. 
It will read the unauthenticated `/v1/webapi/ping` endpoint from the Teleport proxy, parse new fields on that endpoint, and install the specified agent version according to the specified upgrade plan.
It will download the correct version of Teleport as a tarball, unpack it in `/var/lib/teleport`, and ensure it is symlinked from `/usr/local/bin`.

### Installation

```shell
$ apt-get install teleport-ent-updater
$ teleport-update enable --proxy example.teleport.sh

# if not enabled already, configure teleport and:
$ systemctl enable teleport
```

### API

#### Endpoints

`/v1/webapi/ping`
```json
{
  "agent_version": "15.1.1",
  "agent_auto_update": true,
  "agent_update_after": "2024-04-23T18:00:00.000Z",
  "agent_update_jitter_seconds": 10,
}
```
Notes:
- Critical updates are achieved by serving `agent_update_after` with the current time.
- The Teleport proxy translates upgrade hours (below) into a specific time after which all agents should be upgraded.
- If an agent misses an upgrade window, it will always update immediately.

#### Teleport Resources

```yaml
kind: cluster_maintenance_config
spec:
  # agent_auto_update allows turning agent updates on or off at the
  # cluster level. Only turn agent automatic updates off if self-managed
  # agent updates are in place.
  agent_auto_update: on|off
  # agent_update_hour sets the hour in UTC at which clients should update their agents.
  # The value -1 will set the upgrade time to the current time, resulting in immediate upgrades.
  agent_update_hour: -1-23
  # agent_update_jitter_seconds sets a duration in which the upgrade will occur after the hour.
  # The agent upgrader will pick a random time within this duration in which to upgrade.
  agent_update_jitter_seconds: 0-MAXINT64
  
  [...]
```
```
$ tctl autoupdate update --set-agent-auto-update=off
Automatic updates configuration has been updated.
$ tctl autoupdate update --set-agent-update-hour=3
Automatic updates configuration has been updated.
$ tctl autoupdate update --set-agent-update-jitter-seconds=600
Automatic updates configuration has been updated.
```

```yaml
kind: autoupdate_version
spec:
  # agent_version is the version of the agent the cluster will advertise.
  # Can be auto (match the version of the proxy) or an exact semver formatted
  # version.
  agent_version: auto|X.Y.Z

  [...]
```
```
$ tctl autoupdate update --set-agent-version=15.1.1
Automatic updates configuration has been updated.
```

Notes:
- These two resources are separate so that Cloud customers can be restricted from updating `autoupdate_version`, while maintaining control over the rollout.

### Filesystem

```
$ tree /var/lib/teleport
/var/lib/teleport
└── versions
   ├── 15.0.0
   │  ├── bin
   │  │  ├── ...
   │  │  ├── teleport-updater
   │  │  └── teleport
   │  └── etc
   │     ├── ...
   │     └── systemd
   │        └── teleport.service
   ├── 15.1.1
   │  ├── bin
   │  │  ├── ...
   │  │  ├── teleport-updater
   │  │  └── teleport
   │  └── etc
   │     ├── ...
   │     └── systemd
   │        └── teleport.service
   └── updates.yaml
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

On servers without Teleport installed already, the `enable` subcommand will change the behavior of `teleport-update update` to update teleport and restart the existing agent, if running.
It will also run update teleport immediately, to ensure that subsequent executions succeed.

The `enable` subcommand will:
1. Configure `updates.yaml` with the current proxy address and set `enabled` to true.
2. Query the `/v1/webapi/ping` endpoint.
3. If the current updater-managed version of Teleport is the latest, and teleport package is not installed, quit.
4. If the current updater-managed version of Teleport is the latest, but the teleport package is installed, jump to (12).
5. Download the desired Teleport tarball specified by `agent_version`.
6. Verify the checksum.
7. Extract the tarball to `/var/lib/teleport/versions/VERSION`.
8. Replace any existing binaries or symlinks with symlinks to the current version.
9. Restart the agent if the systemd service is already enabled.
10. Set `active_version` in `updates.yaml` if successful or not enabled.
11. Replace the old symlinks or binaries and quit (exit 1) if unsuccessful.
12. Remove any `teleport` package if installed.
13. Verify the symlinks to the active version still exists.
14. Remove all stored versions of the agent except the current version and last working version.

The `disable` subcommand will:
1. Configure `updates.yaml` to set `enabled` to false.

When `update` subcommand is otherwise executed, it will:
1. Check `updates.yaml`, and quit (exit 0) if `enabled` is false, or quit (exit 1) if `enabled` is true and no proxy address is set.
2. Query the `/v1/webapi/ping` endpoint.
3. Check if the current time is after the time advertised in `agent_update_after`, and that `agent_auto_updates` is true.
4. If the current version of Teleport is the latest, quit.
5. Wait `random(0, agent_update_jitter_seconds)` seconds.
6. Download the desired Teleport tarball specified by `agent_version`.
7. Verify the checksum.
8. Extract the tarball to `/var/lib/teleport/versions/VERSION`.
9. Update symlinks to point at the new version.
10. Restart the agent if the systemd service is already enabled.
11. Set `active_version` in `updates.yaml` if successful or not enabled.
12. Replace the old symlink or binary and quit (exit 1) if unsuccessful.
13. Remove all stored versions of the agent except the current version and last working version.

To enable auto-updates of the updater itself, all commands will first check for an `active_version`, and reexec using the `teleport-updater` at that version if present and different.
The `/usr/local/bin/teleport-upgrader` symlink will take precedence to avoid reexec in most scenarios.

### Manual Workflow

For use cases that fall outside of the functionality provided by `teleport-updater`, we provide an alternative manual workflow using the `/v1/webapi/ping` endpoint.
This workflow supports customers that cannot use the auto-update mechanism provided by `teleport-updater` because they use their own automation for updates (e.g., JamF or ansible).

Cluster administrators that want to self-manage agent updates will be
able to get and watch for changes to agent versions which can then be
used to trigger other integrations to update the installed version of agents.

```shell
$ tctl autoupdate watch
{"agent_version": "1.0.0", ... }
{"agent_version": "1.0.1, ... }
{"agent_version": "2.0.0", ... }
[...]
```

```shell
$ tctl autoupdate get
{"agent_version": "2.0.0", ... }
```

### Scripts

All scripts will install the latest updater and run `teleport-updater enable` with the proxy address.

Eventually, additional logic from the scripts could be added to `teleport-updater`, such that `teleport-updater` can configure teleport.

This is out-of-scope for this proposal.

## Security

The initial version of automatic updates will rely on TLS to establish
connection authenticity to the Teleport download server. The authenticity of
assets served from the download server is out of scope for this RFD. Cluster
administrators concerned with the authenticity of assets served from the
download server can use self-managed updates with system package managers which
are signed.

The Upgrade Framework (TUF) will be used to implement secure updates in the future.

## Execution Plan

1. Implement new auto-updater in Go.
2. Prep documentation changes.
3. Release new updater via teleport-ent-updater package.
4. Release documentation changes.
