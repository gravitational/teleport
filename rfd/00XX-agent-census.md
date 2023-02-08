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
A brief description of this task was described in [Cloud's RFD 53](https://github.com/gravitational/cloud/tree/54559795b45b2e8515ea7e159d26cadfbb52482f/rfd/0053-prehog.md).

#### Goals

- Track more information about each Teleport agent.

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
6. `glibc` version
7. [Install method](https://goteleport.com/docs/installation/) (Dockerfile, Helm, `install-node.sh` and `*-ad*.ps1` scripts)
8. Container orchestrator (e.g. Kubernetes)
9. Cloud environment (e.g. AWS, GCP, Azure)

#### Data collection

Currently, when an agent first starts, the [inventory control system](https://github.com/gravitational/teleport/tree/6f9ad9553a5b5946f57cb35411c598754d3f926b/lib/inventory) sends an [`UpstreamInventoryHello`](https://github.com/gravitational/teleport/blob/6f9ad9553a5b5946f57cb35411c598754d3f926b/api/proto/teleport/legacy/client/proto/authservice.proto#L1936-L1953) message to the auth server.
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

We will extend this message to contain the remaining agent data that we want to track:

```protobuf
message UpstreamInventoryHello {
  // (...)
  string OS = 5;
  string OSVersion = 6;
  string HostArchitecture = 7;
  string GLibCVersion = 8;
  string InstallMethod = 9;
  string ContainerOrchestrator = 10;
  string CloudEnvironment = 11;
}
```

When the auth server receives an `UpstreamInventoryHello` message, it will take the information in the message and send it to PreHog.
For this, a new PreHog `AgentMetadataEvent` message will be added (note that only the `UpstreamInventoryHello.Hostname` won't be sent to PreHog):

```protobuf
message AgentMetadataEvent {
  string version = 1;
  string server_id = 2;
  repeated TeleportAccessProtocol protocols = 3;
  string os = 4;
  string os_version = 5;
  string host_architecture = 6;
  string glibc_version = 7;
  string install_method = 8;
  string container_orchestrator = 9;
  string cloud_environment = 10;
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

Both the Teleport version and active Teleport services (which can be used to determine the Teleport access protocols enabled) are already tracked in the inventory control system.
We detail below how the remaining data will be computed.

##### 3. OS

The OS will be the value on the `GOOS` environment variable.
This will give us either `linux` or `darwin`.

##### 4. OS version

- `linux`: `$(lsb_release -is) $(lsb_release -rs)` (e.g. "Ubuntu 22.04")
- `darwin`: `$(sw_vers -productName) $(sw_vers -productVersion)` (e.g. "macOS 13.2")

The actual implementation could follow what `gopsutil` is doing ([linux](https://github.com/shirou/gopsutil/blob/v3.23.1/host/host_linux.go#L128-L314) / [darwin](https://github.com/shirou/gopsutil/blob/v3.23.1/host/host_darwin.go#L94-L120)).

##### 5. Host architecture

- `linux` and `darwin`: `arch` (e.g. "x86_64" or "arm64")

The architecture can be different from the `GOARCH` environment variable (e.g. if running the amd64 build of Teleport on an ARM Mac).
For this reason, we'll use the [`arch`](https://man7.org/linux/man-pages/man1/arch.1.html) command-line utility.

We could also use `uname` but the flag to retrieve the architecture is not portable (`-i` on `linux` and `-p` on `darwin`).
However, `gopsutil` is using [`golang.org/x/sys/unix.Uname`](https://pkg.go.dev/golang.org/x/sys/unix#Uname) for both OS ([here](https://github.com/shirou/gopsutil/blob/v3.23.1/host/host_posix.go)), so we may consider to do the same.

##### 6. `glibc` version

- `linux`: `ldd --version | head -n1 | awk '{ print $NF }'` (e.g. "2.35")

##### 7. Install method

Different installation methods will be tracked with a new `TELEPORT_INSTALL_METHOD` environment variable:
- [Dockerfile](https://github.com/gravitational/teleport/blob/6f9ad9553a5b5946f57cb35411c598754d3f926b/build.assets/charts/Dockerfile): `ENV TELEPORT_INSTALL_METHOD=Dockerfile` will be added to the Dockerfile.
- [`teleport-kube-agent`](https://goteleport.com/docs/reference/helm-reference/teleport-kube-agent) Helm chart: `TELEPORT_INSTALL_METHOD` will be set to `"teleport-kube-agent"` in the [deployment spec](https://github.com/gravitational/teleport/blob/6f9ad9553a5b5946f57cb35411c598754d3f926b/examples/chart/teleport-kube-agent/templates/deployment.yaml#L129).
- [`*-ad*.ps1`](https://github.com/gravitational/teleport/tree/6f9ad9553a5b5946f57cb35411c598754d3f926b/lib/web/scripts/desktop): `setx TELEPORT_INSTALL_METHOD=ps1` will be added to one of these scripts as they are the recommended way to configure windows desktops.
- [`install-node.sh`](https://github.com/gravitational/teleport/blob/6f9ad9553a5b5946f57cb35411c598754d3f926b/lib/web/scripts/node-join/install.sh): `export TELEPORT_INSTALL_METHOD="install-node.sh"` will be added to this script. It is the recommended way to install SSH nodes, apps and many databases. Even though `export` doesn't persist across restarts, we can have the agent persist such value (and maybe all of the values sent in `UpstreamInventoryHello`) when it first starts.

The following installation methods won't be tracked for now:
- tarball, `.deb`/`.rpm`/`.pkg` packages, APT or YUM repository: For tarball, we can add `export TELEPORT_INSTALL_METHOD="tarball"` to the [`install`](https://github.com/gravitational/teleport/blob/6f9ad9553a5b5946f57cb35411c598754d3f926b/build.assets/install) script. (However, if the customer does not use the `install` script and instead moves the binaries manually, we won't be able to track this installation method.) We'll try to these methods if, once we start tracking the above installation methods, we notice that we're not yet covering most installation methods. (It's also unclear ATM if tracking these methods could conflict with the tracking of `install-node.sh`.)
- _built from source_: While it's technically possible for customers to build Teleport from source, we won't try to track this installation method as it seems an unlikely use-case.
- `homebrew`: It's also possible to install Teleport on macOS using `homebrew`. The Teleport package in `homebrew` is not maintained by us, so we will also not track this installation method.

##### 8. Container orchestrator

To determine if the agent is running on a Kubernetes pod, we can check if the `KUBERNETES_SERVICE_HOST` environment variable is set or if the `/var/run/secrets/kubernetes.io/serviceaccount` directory exists ([docs](https://kubernetes.io/docs/tasks/run-application/access-api-from-pod/#directly-accessing-the-rest-api)).

##### 9. Cloud environment

The only way to determine this seems to be by hitting certain HTTP endpoints specific to each cloud environment:
- AWS ([docs](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/instancedata-data-retrieval.html)): http://169.254.169.254/latest/
- GCP ([docs](https://cloud.google.com/compute/docs/metadata/overview#parts-of-a-request)): http://metadata.google.internal/computeMetadata/v1/
- Azure ([docs](https://learn.microsoft.com/en-us/azure/virtual-machines/linux/instance-metadata-service?tabs=linux#access-azure-instance-metadata-service)): http://169.254.169.254/metadata/instance?api-version=2021-02-01

This may be considered too intrusive, so we have to make a decision on whether we really want to track it and argue why it's okay to do so.

#### Development plan

The above work will be divided in the following tasks:

1. Add new message type `AgentMetadataEvent` to PreHog.
2. Extend `UpstreamInventoryHello` message with new fields. These will be set to an empty string initially. (This is okay since being empty means that the field could not be determined, which may happen anyways.)
3. Extend auth server to convert `UpstreamInventoryHello` messages to `AgentMetadataEvent` messages and push them to PreHog.
4. Gradually instrument & add new code that fills each new `UpstreamInventoryHello` message field.

We could decide to do step 4 together with step 2 if we don't want to risk adding fields to `UpstreamInventoryHello` that possibly won't be used in the end (if for some reason we figure out they can't/shouldn't be tracked).
However, step 4 requires several changes (Go code & files used by the multiple installation methods) which we may want to review separately.
A similar reasoning also applies to step 1 since each field in `AgentMetadataEvent` should only exist if it also exists in `UpstreamInventoryHello`.

### Security

Besides hitting certain HTTP endpoints to track which cloud environment the agent is running on, there doesn't seem to be any further concern.

### UX

Data analysis and visualization is not a goal of this RFD, so no UX concerns for now.

### Open questions

- Should we [Teleport AMIs](https://github.com/gravitational/teleport/tree/6f9ad9553a5b5946f57cb35411c598754d3f926b/examples/aws/terraform/AMIS.md) an installation method?
- Which [\*-ad*.ps1](https://github.com/gravitational/teleport/tree/6f9ad9553a5b5946f57cb35411c598754d3f926b/lib/web/scripts/desktop) script should be changed? Are these scripts going away with non-AD desktop access?
  - Does `setx` take effect right away or requires some sort of restart?
- Is the [`install`](https://github.com/gravitational/teleport/blob/6f9ad9553a5b5946f57cb35411c598754d3f926b/build.assets/install) script used for anything else?
- Which container runtimes are we interested in tracking?
- Do we want to track cloud environments? If so, which?
