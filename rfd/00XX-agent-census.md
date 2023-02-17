---
authors: Vitor Enes (vitor@goteleport.com)
state: draft
---

# RFD XX - Agent Census

## Required Approvals

* Engineering: @rjones && @jimbishopp
* Product: @xin || @klizhentas

## What

This RFD details how we'll track more information about agents (aka Agent Census).
A brief description of this task can be found in [Cloud's RFD 53](https://github.com/gravitational/cloud/tree/54559795b45b2e8515ea7e159d26cadfbb52482f/rfd/0053-prehog.md#agent-census).

#### Goals

- Track more information about each Teleport agent (such as OS, OS version, architecture, installation methods, container runtime and others)

#### Non-goals

- Detail how this information will be analyzed / visualized.

## Why

We want to understand how agents are installed and where they are running so that we can prioritize the work around [cloud agent upgrades](https://github.com/gravitational/teleport/pull/20622).

## Details

### Terminology

* Service: A Teleport service manages access to resources such as SSH nodes, kubernetes clusters, internal web applications, databases, and windows desktops.
* Agent: A `teleport` process that runs one or more Teleport services (depending on the configuration).
* PreHog: A microservice used to capture user events across several Teleport tools.

### Implementation Details

This section is divided in the following subsections:
- [Data tracked](#data-tracked): which data about each agent will be tracked
- [Data collection](#data-collection): how such data will flow from the agents to PreHog
- [Data computation](#data-computation): how to compute such data

#### Data tracked

We want to start tracking the following data in PreHog:

1. Teleport version
2. Teleport access protocols (`ssh`, `kube`, `app`, `db` and `windows_desktop`)
3. OS (`linux` or `darwin`, as these are the only two OS currently supported)
4. OS version (e.g. Linux distribution)
5. Host architecture (e.g. `amd64`)
6. `glibc` version (Linux only)
7. [Installation methods](https://goteleport.com/docs/installation/) (Dockerfile, Helm, `install-node.sh`)
8. Container runtime (e.g. Docker)
9. Container orchestrator (e.g. Kubernetes)
10. Cloud environment (e.g. AWS, GCP, Azure)

#### Data collection

Currently, when an agent first starts, the [inventory control system](https://github.com/gravitational/teleport/tree/6f9ad9553a5b5946f57cb35411c598754d3f926b/lib/inventory) (ICS) sends an [`UpstreamInventoryHello`](https://github.com/gravitational/teleport/blob/6f9ad9553a5b5946f57cb35411c598754d3f926b/api/proto/teleport/legacy/client/proto/authservice.proto#L1936-L1953) message to the auth server.
This message has the following fields:

```protobuf
message UpstreamInventoryHello {
  string Version = 1;
  string ServerID = 2;
  repeated string Services = 3 [(gogoproto.casttype) = "github.com/gravitational/teleport/api/types.SystemRole"];
  string Hostname = 4;
}
```

The `Version` field contains the Teleport version, while the `Services` field contains the subset of the system roles that are currently active at the agent.

While initially we considered extending this message to contain all the agent metadata we want track, we decided to instead add a new message type `UpstreamInventoryAgentMetadata` (see the message definition below).
Some of the agent metadata may be slow to compute (due to HTTP requests), and thus blocking the sending of the `UpstreamInventoryHello` until such metadata is computed could potentially increase the agent start-up/connection time.

Instead, when the auth server handle is created at the agent ([here](https://github.com/gravitational/teleport/blob/6f9ad9553a5b5946f57cb35411c598754d3f926b/lib/inventory/inventory.go#L87-L97)), a new goroutine will be spawned in order to fetch the agent metadata in the background.

Then, once the agent has sent the `UpstreamInventoryHello` to the auth server and received the `DownstreamInventoryHello` reply ([here](https://github.com/gravitational/teleport/blob/6f9ad9553a5b5946f57cb35411c598754d3f926b/lib/inventory/inventory.go#L167-L191)), it will request the metadata from the new goroutine and send it to the auth server once it is available.
This step is non-blocking, and thus it should not impact the existing ICS mechanism.

An initial sketch of this flow can be found in [9dc0d07](https://github.com/gravitational/teleport/commit/9dc0d076e9e613f90481a9c5f564a3de5bb4d170).
Note that the agent metadata will be sent to auth server at most once (per boot): if the first attempt does not succeed, the proposed flow won't try to send it again.
However, this attempt only occurs after a successful hello exchange and thus it will likely succeed.

```protobuf
message UpstreamInventoryAgentMetadata {
  // Version advertises the teleport version of the instance.
  string Version = 1;
  // ServerID advertises the server ID of the instance.
  string ServerID = 2;
  repeated string Services = 3 [(gogoproto.casttype) = "github.com/gravitational/teleport/api/types.SystemRole"];
  string OS = 4;
  string OSVersion = 5;
  string HostArchitecture = 6;
  string GLibCVersion = 7;
  repeated string InstallMethods = 8;
  string ContainerRuntime = 9;
  string ContainerOrchestrator = 10;
  string CloudEnvironment = 11;
}
```

When the auth server receives an `UpstreamInventoryAgentMetadata` message, it will take the information in the message and send it to PreHog.
For this, a new PreHog `AgentMetadataEvent` message will be added (note that only the `UpstreamInventoryHello.Hostname` won't be sent to PreHog as it can contain PII but also because it doesn't seem useful):

```protobuf
message AgentMetadataEvent {
  string version = 1;
  string server_id = 2;
  repeated TeleportAccessProtocol protocols = 3;
  string os = 4;
  string os_version = 5;
  string host_architecture = 6;
  string glibc_version = 7;
  repeated string install_methods = 8;
  string container_runtime = 9;
  string container_orchestrator = 10;
  string cloud_environment = 11;
}

enum TeleportAccessProtocol {
  TELEPORT_ACCESS_PROTOCOL_UNSPECIFIED = 0;
  TELEPORT_ACCESS_PROTOCOL_SSH = 1;
  TELEPORT_ACCESS_PROTOCOL_KUBE = 2;
  TELEPORT_ACCESS_PROTOCOL_APP = 3;
  TELEPORT_ACCESS_PROTOCOL_DB = 4;
  TELEPORT_ACCESS_PROTOCOL_WINDOWS_DESKTOP = 5;
}
```

#### Data computation

Both the Teleport version and active Teleport services (which can be used to determine the Teleport access protocols enabled) are already tracked in the ICS.
We detail below how the remaining data will be computed.

Note that some of the suggestions below require running command-line utilities (such as `sw_vers` and `ldd`) or inspecting some files (such as `/etc/os-release`) and then parsing the output.
This parsing will be done in Go and if the output does not have the expected format, the whole output will be included in the `UpstreamInventoryAgentMetadata` field.
For example, if `sw_vers` does not look like what follows, `UpstreamInventoryAgentMetadata.OSVersion` will be set to the full output of `sw_vers` (instead of just `"macOs 13.2"`, as detailed below).

```bash
ProductName:            macOS
ProductVersion:         13.2.1
BuildVersion:           22D68
```

Including the full output allows us to improve the parsing code as this project evolves.
This is especially true since we do not have access to agent logs.

An alternative would be to send the full command output to the auth server and do the parsing there (instead of at the agent).
In this case, we could log the unexpected output and only send "clean" data to PostHog.

##### 3. OS

`UpstreamInventoryAgentMetadata.OS` will be set to the value on the `GOOS` environment variable.
This will give us either `darwin` or `linux` as they are the only two supported OS for now.

##### 4. OS version

On `darwin`, `UpstreamInventoryAgentMetadata.OSVersion` will be set to the outcome of (something equivalent to) `$(sw_vers -productName) $(sw_vers -productVersion)` (e.g. `"macOS 13.2"`).
This is what `gopsutil` is doing ([here](https://github.com/shirou/gopsutil/blob/v3.23.1/host/host_darwin.go#L94-L120)).

On `linux`, we'll inspect `/etc/os-release` and combine the values associated with `"NAME="` and `"VERSION_ID="` (e.g. "Ubuntu 22.04").
If this file does not exist (unlikely, as it seems widely supported), we can fallback to `/etc/lsb-release` and combine the values associated with `"DISTRIB_ID="` and `"DISTRIB_RELEASE="` (which is what `gopsutil` is doing ([here](https://github.com/shirou/gopsutil/blob/v3.23.1/host/host_linux.go#L128-L314))).
Following this approach is more reliable than using `/usr/bin/lsb_release` directly as it is not always available (e.g. `docker run -ti ubuntu:22.04 lsb_release` fails).

##### 5. Host architecture

On `linux` and `darwin`, `UpstreamInventoryAgentMetadata.HostArchitecture` will be set to the outcome of `arch` (e.g. "x86_64" or "arm64").

The architecture can be different from the `GOARCH` environment variable (e.g. if running the amd64 build of Teleport on an ARM Mac).
For this reason, we'll use the [`arch`](https://man7.org/linux/man-pages/man1/arch.1.html) command-line utility.

We could also use `uname` but the flag to retrieve the architecture is not portable (`-i` on `linux` and `-p` on `darwin`).
However, `gopsutil` is using [`golang.org/x/sys/unix.Uname`](https://pkg.go.dev/golang.org/x/sys/unix#Uname) for both OS ([here](https://github.com/shirou/gopsutil/blob/v3.23.1/host/host_posix.go)), so we may consider to do the same.

##### 6. `glibc` version

If on `linux`, `UpstreamInventoryAgentMetadata.GLibCVersion` will be set to the outcome of (something equivalent to) `ldd --version | head -n1 | awk '{ print $NF }'` (e.g. "2.35").

##### 7. Installation methods

Different installation methods will be tracked by setting new `TELEPORT_INSTALL_METHOD_$NAME` environment variables to `true` (where `$NAME` is the installation method).
We have one environment variable for each installation method as some of the installation methods below may occur at the same time (e.g. `Dockerfile` and `teleport-kube-agent`, or `install-node.sh` and `APT` and `systemctl`).

- [Dockerfile](https://github.com/gravitational/teleport/blob/6f9ad9553a5b5946f57cb35411c598754d3f926b/build.assets/charts/Dockerfile): `ENV TELEPORT_INSTALL_METHOD_DOCKERFILE=true` will be added to the Dockerfile.
- [`teleport-kube-agent`](https://goteleport.com/docs/reference/helm-reference/teleport-kube-agent) Helm chart: `TELEPORT_INSTALL_METHOD_HELM_KUBE_AGENT` will be set to `true` in the [deployment spec](https://github.com/gravitational/teleport/blob/6f9ad9553a5b5946f57cb35411c598754d3f926b/examples/chart/teleport-kube-agent/templates/deployment.yaml#L129).
- [`install-node.sh`](https://github.com/gravitational/teleport/blob/6f9ad9553a5b5946f57cb35411c598754d3f926b/lib/web/scripts/node-join/install.sh): `export TELEPORT_INSTALL_METHOD_NODE_SCRIPT="true"` will be added to this script. It is the recommended way to install SSH nodes, apps and many databases. Even though `export` doesn't persist across restarts, we can have the agent persist such value (and maybe all of the values sent in `UpstreamInventoryAgentMetadata`) when it first starts.
- `systemctl`: Tracking whether the agent is running using `systemctl` does not require a new environment variable. For this, we'll simply check if `systemctl status teleport.service` succeeds and, if so, if it contains the string `"active (running)"`.

The installation methods that follow won't be tracked for now.
Later on, we may try to track these if, once we start tracking the above installation methods, we notice that we're not yet covering most methods.
- tarball: We can add `export TELEPORT_INSTALL_METHOD_TARBALL="true"` to the [`install`](https://github.com/gravitational/teleport/blob/6f9ad9553a5b5946f57cb35411c598754d3f926b/build.assets/install) script. (However, if the customer does not use the `install` script and instead moves the binaries manually, we won't be able to track this installation method.)
- `.deb`/`.rpm`/`.pkg` packages, APT or YUM repository: It's unclear ATM how these can be tracked.
- built from source: While it's technically possible for customers to build Teleport from source, we won't try to track this installation method as it seems an unlikely use-case.
- `homebrew`: It's also possible to install Teleport on macOS using `homebrew`. The Teleport package in `homebrew` is not maintained by us, so we will also not track this installation method.

In summary, we'll have the following values in `UpstreamInventoryAgentMetadata.InstallMethods` for now:
- `dockerfile`
- `helm-kube-agent`
- `node-script`
- `systemctl`

##### 8. Container runtime

To determine if the agent is running on Docker, we'll check if the file `/.dockerenv` exists.
(Docker itself [does this](https://github.com/moby/libnetwork/blob/1f3b98be6833a93f254aa0f765ff55d407dfdd69/drivers/bridge/setup_bridgenetfiltering.go#L161)).
If so, `UpstreamInventoryAgentMetadata.ContainerRuntime` will be set to `docker`.

If we're interested in tracking other container runtimes, we could follow the approach by `gopsutil` ([here](https://github.com/shirou/gopsutil/blob/v3.23.1/internal/common/common_linux.go#L130-L278)).

##### 9. Container orchestrator

To determine if the agent is running on a Kubernetes pod, we can try to initialize a Kubernetes `client` similar to how [Validator.getClient()](https://github.com/gravitational/teleport/blob/master/lib/kubernetestoken/token_validator.go#L50-L68) does it.
If this succeeds, the agent is running on Kubernetes.

Afterwards, we'll try to detect in which cloud provider the pod is running on.
For this, we'll call `client.ServerVersion()` and check if the returned `gitVersion` contains a certain substring specific for each provider:

- in EKS, the git version looks like `"v1.24.8-eks-ffeb93d"` (we'll search for substring `"-eks"`)
- in GPC ([docs](https://cloud.google.com/kubernetes-engine/docs/release-notes)), the git version looks like `"1.23.14-gke.1800"` (we'll search for substring `"-gke"`)

In AKS, the git version looks like `"v1.25.2"`, so it's not possible to detect this environment using this method. (This is also a problem for Helm charts, as reported in [Azure/AKS#3375](https://github.com/Azure/AKS/issues/3375).)

In the end, `UpstreamInventoryAgentMetadata.ContainerOrchestrator` will be set to:
- `kubernetes-eks` if on EKS
- `kubernetes-gke` if on GKE
- `kubernetes-unknown` otherwise (AKS, other cloud provider, or no cloud provider)

##### 10. Cloud environment

The only way to determine this seems to be by hitting certain HTTP endpoints specific to each cloud environment:
- AWS ([docs](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/instancedata-data-retrieval.html)): http://169.254.169.254/latest/
- GCP ([docs](https://cloud.google.com/compute/docs/metadata/overview#parts-of-a-request)): http://metadata.google.internal/computeMetadata/v1/
- Azure ([docs](https://learn.microsoft.com/en-us/azure/virtual-machines/linux/instance-metadata-service?tabs=linux#access-azure-instance-metadata-service)): http://169.254.169.254/metadata/instance?api-version=2021-02-01

`UpstreamInventoryAgentMetadata.CloudEnvironment` will be set to:
- `aws` if on AWS
- `gcp` if on GCP
- `azure` if on Azure

#### Development plan

The above work will be divided in the following tasks:

1. Add new message type `AgentMetadataEvent` to PreHog.
2. Add new message type `UpstreamInventoryAgentMetadata`. These will be empty initially, except for the fields common with `UpstreamInventoryHello`. This is okay since being empty means that the field could not be determined, which may happen anyways.
3. Extend auth server to convert `UpstreamInventoryAgentMetadata` messages to `AgentMetadataEvent` messages and push them to PreHog.
4. Gradually instrument & add new code that fills each new `UpstreamInventoryAgentMetadata` message field.

We could decide to do step 4 together with step 2 if we don't want to risk adding fields to `UpstreamInventoryAgentMetadata` that possibly won't be used in the end (if for some reason we figure out they can/should not be tracked).
However, step 4 requires several changes (Go code & files used by the multiple installation methods) which we may want to review separately.
A similar reasoning also applies to step 1 since each field in `AgentMetadataEvent` should only exist if it also exists in `UpstreamInventoryAgentMetadata`.

### Security

Detecting the __9. Container orchestrator__ and __10. Cloud environment__ requires hitting certain HTTP endpoints.
This may be considered too intrusive, so we have to make a decision on whether we really want to track it and argue why it's okay to do so.

### UX

Data analysis and visualization are not a goal for this RFD, so no UX concerns for now.

### Open questions

- Which alternative should we use for __5. Host architecture__?
- Should we consider [Teleport AMIs](https://github.com/gravitational/teleport/tree/6f9ad9553a5b5946f57cb35411c598754d3f926b/examples/aws/terraform/AMIS.md) an installation method?
- Which container runtimes are we interested in tracking?
- What happens if the agent is upgraded before the auth server, and the auth server does not know about `UpstreamInventoryAgentMetadata`?
