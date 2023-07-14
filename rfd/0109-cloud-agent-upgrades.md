---
authors: Forrest Marshall (forrest@goteleport.com)
state: draft
---

# RFD 0109 - Cloud Agent Upgrades


## Required Approvers

* Engineering: @klizhentas && (@russjones || @zmb3 || @rosstimothy || @espadolini)

* Infra: @fheinecke

* Kube: @hugoShaka


## What

A simple mechanism for automatically keeping cloud agents on a common "stable"
install channel.


## Why

We would like to increase the cadence at which cloud clusters can be upgraded, and
eliminate the significant overhead incurred by requiring users to manually upgrade agents.

The existing upgrade system proposal is very complex, and many of its features
are only relevant for niche on-prem scenarios. The majority of cloud deployments can be served
by a far simpler system.

This proposal describes a simple framework for keeping cloud agents up to date, designed with the
intent to rely as little as possible on teleport itself for the upgrades.

Because we are striving for as simple a system as possible, we will make a few scope-limiting
assumptions/concessions:

- All enrolled cloud agents track a single global cloud-stable channel, rather than having a
per-cluster target agent version.

- Only a limited subset of common deployment scenarios will be supported.

- The only supported form of "rollback" will be to downgrade the global cloud-stable target itself.

- We won't be trying to come up with a solution that works for everyone. Some cloud clusters will have
this behavior enabled, and some won't.


## Details

From the start, we will support the following scenarios:

- Teleport agents deployed as `k8s` pods using a helm chart the explicitly supports this upgrade system.

- Teleport agents installed via a dedicated `apt` or `yum` package that sets up the installer.

Agents deployed in one of the above manners will automatically be "enrolled" in the cloud upgrade system as part of the
installation process. Enrollment will not be coordinated at the cluster-level (i.e. agents will not check with the
cluster to determine if upgrades should occur). It will be assumed that the latest available cloud-stable package is
always the "correct" package to be running. This is an important divergence from how teleport versioning is typically
handled, and has the effect of allowing unhealthy teleport instances to attempt upgrading as a means of "fixing" themselves.

The only form of cluster-level coordination around agent upgrades will be for timing. Since a restart is
necessary for an upgrade to take effect, healthy agents will periodically load and store an agent maintenance
window/schedule, and seek to update only during a valid window. Unhealthy agents will be permitted to be
upgraded outside of the maintenance window in an attempt to become healthy.

Teleport cloud clusters that track cloud-stable will have their control planes frequently upgraded s.t.
their control planes are always compatible with the current version targeted by the `cloud-stable` channel.

The current `cloud-stable` target version will be served via an `s3` bucket backed by CloudFront. Ex:

```
$ VERSION="$(curl --proto '=https' --tlsv1.2 -sSf https://updates.releases.teleport.dev/v1/cloud/stable/version)"
```

Formally, this will be the only definition of what the current "cloud-stable" target is. An agent "downgrade" will
consist of decrementing the version string served here, and an "upgrade" will consist of incrementing the version.

Rollout of a new version to cloud-stable will generally follow the following steps:

1. A new target version is selected (but *not* published to the cloud-stable channel endpoint).

2. Each cloud control plane is upgraded to the target version during its next appropriate maintenance window.

3. The target version of the cloud-stable endpoint is updated to point to the new target client version.

4. A grace period is observed, during which the control plane is not updated s.t. it breaks compatibility with
agents that have yet to be upgraded. I.e. if cloud-stable targets `v1` at `T1`, then the control plane must maintain
compatibility with `v1` until at least `T1 + grace_period`.

Note that in practice, wether or not the ordering/timing of the above steps matters depends entirely on what changes
were made between versions. Most minor/patch releases don't actually require this kind of procedure, tho its best to
assume that the procedure is required unless we are rolling out a critical bug fix that was specifically designed to
be self-contained.

Control plane downgrades are permissible, so long as the downgrade does not break compatibility with recently
served cloud-stable agent versions. Likewise, downgrade of the cloud-stable client version is also permissible so long as
said downgrade does not break compatibility with the current control-plane version.

If we get ourselves into a situation where an immediate downgrade is required for either the control plane or agent version
*and* said downgrade will cause a break in compatibility, some amount of service interruption becomes unavoidable. In
this scenario, we would likely be required to force agents to downgrade ASAP. See discussion below of the
`cloud-stable/critical` endpoint.


### Version Discovery Endpoint

The upgrade model will rely on two points of global coordination. The ability to query the current target cloud-stable
version, and the ability to check if the current target represents a critical update (i.e. should it be installed even
on agents that are outside their maintenance window).

The discovery endpoint will serve static values of the form `{apiversion}/{channel}/{value}`, with two options being initially
supported:

- `v1/cloud-stable/version`: Semver string (e.g. `1.2.3`) representing the current target version.

- `v1/cloud-stable/critical`: The string `yes` indicates that we are in a critical update state. Any other value
is treated as non-critical.


Example usage:

```bash
VERSION="$(curl --proto '=https' --tlsv1.2 -sSf https://updates.releases.teleport.dev/v1/cloud/stable/version)"
CRITICAL="$(curl --proto '=https' --tlsv1.2 -sSf https://updates.releases.teleport.dev/v1/cloud/stable/critical)"

if [[ "$CRITICAL" == "yes" || "$UPGRADE_WINDOW" == "yes" ]]
then
    apt install teleport-ent="$VERSION"
fi
```

If the `cloud-stable/version` endpoint is unavailable, upgrades should not be attempted. If the `cloud-stable/critical` endpoint
is unavailable, updates should be assumed to be non-critical.

Due to the potentially high load on these endpoints, we will be leveraging s3+CloudFront to ensure that the values are highly
available.


### Core Upgrade Model

In the interest of simplicity/stability, the actual upgrader will be decoupled from the teleport agent. In kubernetes the
upgrader is a controller. In VM/bare metal deployments, the upgrader is a systemd timer that invokes a `teleport-upgrade`
script.  In both cases, the upgrader is responsible for installing the current target matching the cloud-stable version,
and restarting teleport if appropriate.

When the teleport agent is unhealthy or `cloud-stable/critical` is `yes`, the upgrader should attempt upgrades on
a time-based loop s.t. between 1 and 2 upgrade attempts are made each hour.

When the teleport agent is healthy, it should periodically publish a "schedule" indicating when it is appropriate to
attempt an upgrade (schedule variants are discussed later). Updaters should obey the emitted schedule to ensure that
any necessary downtime occurs during a maintenance window.

Abstractly, the updater loop looks like this:

```
┌─► Loop ◄──────────────────────────┐
│     │                             │
│     │                             │
│     ▼                             │
│   Critical Update?                │
│     │   │                         │
│     │   │                         │
│    no   └──────yes────────┐       │
│     │                     │       │
│     │                     │       │
│     ▼                     ▼       │
│   Agent Healthy?     Attempt Upgrade
│     │  │                      ▲   ▲
│     │  │                      │   │
│    yes └────────no────────────┘   │
│     │                             │
no    │                            yes
│     ▼                             │
└───Within Maintenance Window? ─────┘
```

Note that this is a bit vague on timing/frequency. Different updaters use different scheduling/looping mechanics (discussed
in more detail below).

Different variants will require different upgrade window specifications. In order to minimize client-side logic, the auth server
will present agents with the raw "schedule spec" for the agent's given deployment context. For k8s this will be a json blob
understood by the controller and exported to a kubernetes config object. For VM/bare metal this will be a systemd
`OnCalendar` string (e.g. `Tue,Wed *-*-* 00:00:00 UTC`) to be written out to a systemd `override.conf`.

```proto3
message GetAgentMaintenanceScheduleRequest {
    string TeleportVersion = 1;
    AgentUpgraderKind Kind = 2;
}

enum AgentUpgraderKind {
    KUBE_CONTROLLER = 1;
    SYSTEMD_TIMER = 2;
}

message GetAgentMaintenanceScheduleResponse {
    string Spec = 1;
}

service AuthServer {
    // ...

    rpc GetAgentMaintenanceSchedule(GetAgentMaintenanceScheduleRequest) returns (GetAgentMaintenanceScheduleResponse);
}

// note: opting for pull-based over push-based since agents will want to discover the window immediately on
// startup under some conditions, but do *not* need to rediscover the window upon auth connectivity interruption.
```

Both the systemd and k8s variants of the upgrade model need to assume that frequent agent crashes indicate an unhealthy
state. Rather than adding additional health check mechanisms, agents that _know_ that they are unhealthy should perform
a periodic exit to inform their updater. The primary metric of health will be auth server connectivity. If an agent
cannot establish a healthy connection to any auth agent for greater than 5 minutes, it will exit.

Abstractly, the agent-side loop looks like this:

```
┌─► Loop ◄──────────────────────────┐   ┌──► Exit
│     │                             │   │
│     │                             │   │
│     ▼                             │   │
│   Connected to Control Plane?    no  yes
│     │   │                         │   │
│     │   │                         │   │
│    yes  └──────no──────────► Persistent Issue?
│     │                                 ▲
│     │                                 │
│     ▼                                 │
│   Schedule Recently Updated?         err
│     │   │                             │
│     │   │                             │
│    yes  └──────no──────────► Refresh Schedule
│     │                                 │
│     │                                 │
└─────┴───────────────────────────ok────┘
```


### Kubernetes Model

Kubernetes based agents will be managed by a separate cloud-stable upgrade controller. The upgrade controller will
be responsible for all teleport agents installed by the helm chart, and will perform rolling
updates, applying the latest immutable image tag to all agents.

Agents running in kubernetes mode will periodically export their schedules to a kubernetes config object to be
consumed by the controller.

The k8s controller's specific loop will end up looking like this:

![Upgrade Controller](https://user-images.githubusercontent.com/16366487/219150279-9255d4b3-74fb-48f7-b966-04cb79520e66.png)

The kubernetes controller will leverage cosign based image signing for an added layer of security.

Our kube agent helm chart will be updated to support enabling all necessary cloud-stable behaviors
(added controller and any necessary command/env changes) via a single flag. Ex:

```
$ helm install teleport-kube-agent [...] --set cloudStableUpgrades=yes
```


### Apt/Yum Model

For `apt` and `yum` based installs, we operate under the assumption that teleport is managed by `systemd`. A new
`cloud` "channel" will be set up, and a new `teleport-ent-cloud-updater` package will be made available on that channel,
which will depend on the standard teleport package and install additional systemd units.

In order to minimize the risk of users accidentally manually upgrading to an agent version *too new* for the cloud
control plane, the `cloud` channel will lag a bit behind master.

Note that the behavior of the *repository* "channels" differs from the behavior of the `cloud-stable` TLS endpoint
"channel".  The repo channels contain all package versions that *have* or *might* become a target of the `cloud-stable`
channel endpoint. The `cloud-stable` channel endpoint serves the specific target version that agents *aught* to be
running at the current time.

When a user installs `teleport-ent-cloud-updater`, the main `teleport` package will be brought in as a dependency. The
`teleport-ent-cloud-updater` package itself will install a `teleport-upgrade` script, and a pair of additional systemd
timer units: `teleport-upgrade.timer` and `teleport-upgrade-check.timer`. It will also install a systemd override file
for the main teleport service file adding an `ExecStopPost` command clearing the overrides currently applied to
`teleport-upgrade.timer` (more on this later).

The `teleport-upgrade` script will be the most trivial of the updater components. It will attempt to install
the current target version of the `teleport` package as specified by the `cloud-stable/version` endpoint, restarting
the main teleport service if appropriate.

The `teleport-upgrade.timer` unit will invoke `teleport-upgrade` whenever it "fires". This timer unit will default
to firing approximately every 30min. When healthy, teleport agents will export a systemd override file, blanking
the default monotonic fire rate and setting an `OnCalendar` directive matching the current agent maintenance schedule,
thereby limiting the upgrade timer to firing only during maintenance windows.

The `teleport-upgrade-check.timer` unit will periodically check `cloud-stable/critical`. If it detects a critical update
*and* that the currently installed teleport version differs from the cloud-stable target, it will revert the primary
timer back to monotonic/high-frequency mode. Note that we *could* invoke the upgrade script directly instead and the
system would be simpler overall, however this would lead to duplicate sources of truth for what a "firing" upgrade
was, and potentially lead to confusing interactions between conflicting check rates. After some back and forth,
I'm opting to commit to the concept of a single upgrade timer whose modes are switched by external actors, but
which ultimately retains sole control over the upgrade process.


### Automatic/Guided Install Changes

Teleport provides two kinds of automatic/guided install mechanism for adding the teleport agent software to existing
machines. There are "discovery installers" which are scripts used by ec2 auto-discovery (and possibly by other forms of
server auto-discovery in the future) to install teleport agents automatically, and there is the `node-join` script template
used by the web UI to serve custom installers during the interactive agent-adding process.

To ensure a seamless experience, we will need both of these install scripts to be capable of being parameterized or overridden s.t.
they target the appropriate cloud-stable package when the cluster is configured for continuous agent upgrades. Based on the current
thinking of using a `teleport-ent-cloud-updater` package, this would probably mean going with modifying the install syntax to
look something like this:

```
$ apt install teleport-ent-cloud-updater="$(curl ...)"
```

Alternatively, we could have auth servers cache the current cloud-stable version and inject it into discovery/join scripts statically,
but that would likely be more trouble than its worth.

Repository channel registration will also need to target the `stable/cloud` repo channel, rather than the current pattern of targeting
the major version channel corresponding to the current control plane version.

The current implementation of the `node-join` script template relies upon direct downloads from `get.gravitational.com`. This will need
to be changed to follow the same pattern of registering a release channel and installing via package manager, as is currently employed
by ec2 discovery.


### Documentation Changes

After initial trials, we would like tracking cloud-stable to become the default for new cloud clusters. Because of this, we will
need to make some updates to existing documentation to guide new cloud users to invoke the necessary install variants.

Mostly, this will consist of adding appropriate `ScopedBlock`s to docs that display cloud variants of install
commands. A number of the pages under `deploy-a-cluster/helm-deployments/` will need this treatment. Most cloud guides prompt
users to use a join script, so the majority of those will be able to be left untouched, as clusters will automatically serve
the correct script variant.

We should also have a page we can link to explaining the basics of the `cloud-stable` upgrade behavior, and how to opt
in/out when creating the cluster. By default, we should assume that `cloud-stable` is the default path, and guides should
generally reflect that.


### Summary of Changes

- Agents will expose new flags to enable the systemd and k8s variants of the agent upgrade behavior.

- Package repositories will expose a `teleport-ent-cloud-updater` package that depends on a corresponding
teleport version and installs the appropriate update script and systemd units.

- A new `stable/cloud` family of repository channels will be added. The `teleport-ent-cloud-updater` will only
be made available via these channels in order to prevent users from accidentally enabling cloud-stable behavior
while configured to track an incompatible repo channel.

- A new update controller will be added to our helm charts that can be enabled with an optional flag.

- Teleport auth servers will be modified to be able to serve the maintenance window to agents.

- Teleport auth servers will accept a flag or env var that switches them into "cloud-stable" mode, causing
them to serve modified discovery scripts s.t. they target the cloud-stable channel.

- Teleport discovery and join scripts will be reworked to support targeting the appropriate cloud-stable channels and
versions.

- Cloud clusters will have the option to track cloud-stable rather than a specific major version. When this option is
enabled, their auth servers will be started with the appropriate flags to modify discovery behavior, and the cloud team
will include them in periodic cloud-stable control plane updates.

- A new `cloud-stable` TLS endpoint will be set up that serves the current target agent `version`, and a `critical` flag
that causes updaters to expedite upgrades when set.

- New github actions will be added to enable promoting artifacts to the `stable/cloud` repo channels, and updating the
`cloud-stable/version` and `cloud-stable/critical` endpoints.

- Documentation will be updated to include cloud-stable variants of install commands where appropriate, and to provide
cloud users with a basic explanation of how automated cloud upgrades work.


### Security Considerations

For the most part, this proposal relies on the existing security model of our package distribution system. So long
as package managers/repos remains secure, this system aught to be secure by extension, and as we improve the security
of our distribution systems (e.g. adding image signing to OCI repos), we should ensure that this system benefits from
those improvements. The only truly novel components
are the upgrade script and timer unit. Unauthorized modification would be bad, but that is just a matter of setting
correct file permissions. I think the main risk induced by these additions would be in making the upgrade script too
dynamic. Upgrade scripts should not attempt multiple different download options (e.g. an upgrade script
installed by `yum` shouldn't fallback to invoking `apt` if something in the host environment changes, or if an unexpected
error occurs). Upgrade scripts should also not accept any variables/inputs from auth or from teleport. The origin of the
upgrade script should statically determine its behavior.

It may be useful to add checks to limit downgrades, though this system does require the ability to perform limited
downgrades in the event of bugs. A simple control that seems reasonable might be to disallow major version downgrades.
More granular rules than that may not be suitable given the cadence of minor/patch releases.



### Future Considerations

- Initial work will hard-code the `cloud-stable` channel target for simplicity's sake, but the updaters described here could
be modified to use custom channels/endpoints without too much extra effort.

- Establishing a `cloud-staging` channel alongside the `cloud-stable` channel would probably be a good idea, as it would
let us better test upcoming rollouts, and give customers an opportunity to preview upcoming changes in a standardized way.
This would be most useful if we could improve client compatibility with older control planes, which is something that we
aught to be doing anyway.

- Customers have mentioned that it would be helpful if agents had the ability to warn you if they were
schedule to restart soon when you ssh into them. This seems like a reasonable and useful feature.
