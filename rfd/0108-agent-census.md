---
authors: Vitor Enes (vitor@goteleport.com)
state: implemented (v11.3.8, v12.1.1)
---

# RFD 108 - Agent Census

## Required Approvals

* Engineering: @zmb3 && @jimbishopp
* Product: @xin || @klizhentas
* Security: @wadells

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
2. Teleport enabled services (`node`, `kube`, `app`, `db` and `windows_desktop`)
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

Instead, when the auth server handle is created at the agent ([here](https://github.com/gravitational/teleport/blob/6f9ad9553a5b5946f57cb35411c598754d3f926b/lib/inventory/inventory.go#L87-L97)), a new goroutine will be spawned in order to fetch the agent metadata in the background and send it every time a new stream with the auth server is established.

```protobuf
// UpstreamInventoryAgentMetadata is the message sent up the inventory control stream containing
// metadata about the instance.
message UpstreamInventoryAgentMetadata {
  // OS advertises the instance OS ("darwin" or "linux").
  string OS = 1;
  // OSVersion advertises the instance OS version (e.g. "ubuntu 22.04").
  string OSVersion = 2;
  // HostArchitecture advertises the instance host architecture (e.g. "x86_64" or "arm64").
  string HostArchitecture = 3;
  // GlibcVersion advertises the instance glibc version of linux instances (e.g. "2.35").
  string GlibcVersion = 4;
  // InstallMethods advertises the install methods used for the instance (e.g. "dockerfile").
  repeated string InstallMethods = 5;
  // ContainerRuntime advertises the container runtime for the instance, if any (e.g. "docker").
  string ContainerRuntime = 6;
  // ContainerOrchestrator advertises the container orchestrator for the instance, if any
  // (e.g. "kubernetes-v1.24.8-eks-ffeb93d").
  string ContainerOrchestrator = 7;
  // CloudEnvironment advertises the cloud environment for the instance, if any (e.g. "aws").
  string CloudEnvironment = 8;
}
```

When the auth server receives an `UpstreamInventoryAgentMetadata` message, it will take the information in the message and send it to PreHog.
For this, a new PreHog `AgentMetadataEvent` message will be added (note that only the `UpstreamInventoryHello.Hostname` won't be sent to PreHog as it can contain PII but also because it doesn't seem useful):

```protobuf
message AgentMetadataEvent {
  string version = 1;
  string host_id = 2;
  repeated string services = 3;
  string os = 4;
  string os_version = 5;
  string host_architecture = 6;
  string glibc_version = 7;
  repeated string install_methods = 8;
  string container_runtime = 9;
  string container_orchestrator = 10;
  string cloud_environment = 11;
}
```

##### PostHog data

Some of the fields above are `repeated`.
In PostHog, instead of storing these field values as arrays, we will create one event property for each element in the array (which will likely help visualizing this information in PostHog).

If, for example, `AgentMetadataEvent.services` contains both `node` and `kube`, in PostHog we'll have the following three properties:
- `tp.agent.services = [node, kube]`
- `tp.agent.service.node = true`
- `tp.agent.service.kube = true`

The same applies for `AgentMetadataEvent.install_methods`.

#### Data computation

Both the Teleport version and active Teleport services are already tracked in the ICS.
We detail below how the remaining data will be computed.

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

`UpstreamInventoryAgentMetadata.HostArchitecture` will be set to the value on the `GOARCH` environment variable.

In the future we may use `sysctl -n sysctl.proc_translated` in order to detect if a macOS agent is running under Rosetta.

##### 6. `glibc` version

If on `linux`, `UpstreamInventoryAgentMetadata.GLibCVersion` will be set to the output of `gnu_get_libc_version`.

```go
// #include <gnu/libc-version.h>
import "C"

func fetchGlibcVersion() string {
  return C.GoString(C.gnu_get_libc_version())
}
```

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
- `.deb`/`.rpm`/`.pkg` packages, APT or YUM repository, and [Teleport AMIs](https://github.com/gravitational/teleport/tree/6f9ad9553a5b5946f57cb35411c598754d3f926b/examples/aws/terraform/AMIS.md): It's unclear ATM how these can be tracked.
- built from source: While it's technically possible for customers to build Teleport from source, we won't try to track this installation method as it seems an unlikely use-case.
- `homebrew`: It's also possible to install Teleport on macOS using `homebrew`. The Teleport package in `homebrew` is not maintained by us, so we will also not track this installation method.

In summary, we'll have the following values in `UpstreamInventoryAgentMetadata.InstallMethods` for now:
- `dockerfile`
- `helm_kube_agent`
- `node_script`
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
For this, we'll call `client.ServerVersion()`:
- in EKS, the git version looks like `"v1.24.8-eks-ffeb93d"` (i.e. contains the substring `"-eks"`)
- in GPC ([docs](https://cloud.google.com/kubernetes-engine/docs/release-notes)), the git version looks like `"1.23.14-gke.1800"` (i.e. contains the substring `"-gke"`)
- in AKS, the git version looks like `"v1.25.2"`, so it's not possible to detect this environment using this method. (This is also a problem for Helm charts, as reported in [Azure/AKS#3375](https://github.com/Azure/AKS/issues/3375).)

In the end, `UpstreamInventoryAgentMetadata.ContainerOrchestrator` will be set to `kubernetes-$GIT_VERSION`.

Initially we considered setting `UpstreamInventoryAgentMetadata.ContainerOrchestrator` to `kubernetes-eks` if on EKS, `kubernetes-gcp` if on GCP and `kubernetes-unknown` otherwise.
However, this will require changing the agent code in order to track AKS (if at some point they decide to include the substring `"-aks"`) or some other container orchestrator that can also be detected using the git version.

##### 10. Cloud environment

The only way to determine this seems to be by hitting certain HTTP endpoints specific to each cloud environment:
- AWS ([docs](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/instancedata-data-retrieval.html)): http://169.254.169.254/latest/
- GCP ([docs](https://cloud.google.com/compute/docs/metadata/overview#parts-of-a-request)): http://metadata.google.internal/computeMetadata/v1/
- Azure ([docs](https://learn.microsoft.com/en-us/azure/virtual-machines/linux/instance-metadata-service?tabs=linux#access-azure-instance-metadata-service)): http://169.254.169.254/metadata/instance?api-version=2021-02-01

`UpstreamInventoryAgentMetadata.CloudEnvironment` will be set to:
- `aws` if on AWS
- `gcp` if on GCP
- `azure` if on Azure

### Security

Detecting the __9. Container orchestrator__ and __10. Cloud environment__ requires hitting certain HTTP endpoints.
This may be considered too intrusive, so we have to make a decision on whether we really want to track it and argue why it's okay to do so.

The host ID will be anonymized as it may not be just a UUID.

#### Data sanitization

Nothing special is done regarding sanitization.
This will be tackled more holistically in a follow-up project.

### UX

Data analysis and visualization are not a goal for this RFD, so no UX concerns for now.

