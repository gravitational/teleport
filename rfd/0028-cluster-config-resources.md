---
authors: Andrej Tokarčík (andrej@goteleport.com)
state: implemented
---

# RFD 28 - Cluster configuration related resources

## What

Reorganization of resources related to cluster configuration, primarily as it
regards the splitting of the `ClusterConfig` resource into smaller logical
resources.

## Why

To provide coherent user experience when working with dynamically-configurable singleton resources,
see [RFD 16](https://github.com/gravitational/teleport/blob/master/rfd/0016-dynamic-configuration.md).

## Details

### Splitting `ClusterConfig`

The current `ClusterConfig` resource has the following fields:

```proto
message ClusterConfig {
    // ClusterID is the unique cluster ID that is set once during the first auth
    // server startup.
    string ClusterID;

    // SessionRecording controls where (or if) the session is recorded.
    string SessionRecording;

    // ProxyChecksHostKeys is used to control if the proxy will check host keys
    // when in recording mode.
    string ProxyChecksHostKeys;

    // Audit is a section with audit config
    AuditConfig Audit;

    // ClientIdleTimeout sets global cluster default setting for client idle
    // timeouts
    int64 ClientIdleTimeout;

    // DisconnectExpiredCert provides disconnect expired certificate setting -
    // if true, connections with expired client certificates will get disconnected
    bool DisconnectExpiredCert;

    // KeepAliveInterval is the interval the server sends keep-alive messsages
    // to the client at.
    int64 KeepAliveInterval;

    // KeepAliveCountMax is the number of keep-alive messages that can be missed
    // before
    // the server disconnects the connection to the client.
    int64 KeepAliveCountMax;

    // LocalAuth is true if local authentication is enabled.
    bool LocalAuth;

    // SessionControlTimeout is the session control lease expiry and defines
    // the upper limit of how long a node may be out of contact with the auth
    // server before it begins terminating controlled sessions.
    int64 SessionControlTimeout;
}
```

It is proposed to distribute them in the following fashion:

```proto
// ClusterID field is moved into ClusterName resource.
message ClusterName {
    ...
    string ClusterID;
}

message SessionRecordingConfig {
    // ClusterConfig.SessionRecording is renamed to SessionRecordingConfig.Mode.
    string Mode;
    bool ProxyChecksHostKeys;
}

message ClusterNetworkingConfig {
    int64 ClientIdleTimeout;
    int64 KeepAliveInterval;
    int64 KeepAliveCountMax;
    int64 SessionControlTimeout;
}

// The already existing AuditConfig is turned into a standalone resource.
message AuditConfig { ... }

// DisconnectExpiredCert & LocalAuth fields are moved into ClusterAuthPreference.
message ClusterAuthPreference {
    ...
    bool AllowLocalAuth;
    bool DisconnectExpiredCert;
}
```

No configuration value should be stored in more than one location in the backend.
Consequently, after the proposed transition has fully taken place,
there will be no `ClusterConfig` resource to be stored in the backend.

#### Backward compatibility

Updating the `ClusterConfig` resource using the `SetClusterConfig` endpoint -- exposed as part of the Auth server HTTP/JSON API, but not really used in the wild -- will not be supported anymore.

To fulfill the obligation of backward compatibility with respect to older Teleport components, reading of `ClusterConfig` will remain supported:

1. `GetClusterConfig` is to populate the legacy `ClusterConfig` structure with data obtained from the other configuration resources.
2. To ensure proper cache propagation, updates to the other configuration resources that contain fields previously belonging to `ClusterConfig` will trigger a `ClusterConfig` event in addition to the event of their own kind.
3. `ClusterConfig` events will be populated with data obtained from the other configuration resources.

### (Teleport Cloud only) Restricting to a subset of values of a field

Certain field values should not be available for configuring by Cloud users.
The `Modules` interface shall be extended to provide a resource validation step,
allowing to inject additional checks when the Cloud license flag is set.

### Additional dynamically configurable resources

In addition to the resources derived from `ClusterConfig`, the resources
`ClusterAuthPreference` and `PAMConfig` should also be adapted to facilitate
the dynamic configuration workflow.

### Working with a whole cluster configuration

`KindClusterConfig` should be retained but reinterpreted as a helper meta-kind
similar to `KindConnectors`. It would allow aggregating all the cluster config
related resources into a single `tctl get` output:

```
$ tctl get cluster_config
kind: session_recording_config
[...]
---
kind: cluster_networking_config
[...]
---
kind: audit_config
[...]
---
kind: cluster_auth_preference
[...]
---
kind: pam_config
[...]
```

This combined output can be subsequently edited and used to replace the old
configuration by passing it to the `tctl create` command which is able to
consume multiple resource definitions (provided the user has the privileges
needed to update all the resource kinds).

Note that if a field of a configuration resource is omitted from the YAML, the
field's value will be reset to its default.  The `tctl` workflow supports only
replacing (or overwriting) of a stored resource with another full resource,
not a partial update of the stored resource.

### Examples

#### Setting up node-sync session recording

In this example, `session_recording_config` is assumed to be already
dynamically pre-configured: in particular, while the `mode` field would be by
default set to `node`, here it is already set to `off`.

```
$ tctl get session_recording_config | tee reccfg.yaml
kind: session_recording_config
metadata:
  id: 1618929344290245400
  name: session-recording-config
  labels:
    teleport.dev/origin: dynamic
spec:
  mode: "off"
  proxy_checks_host_keys: yes
version: v2

$ sed -i 's/mode: "off"/mode: "node-sync"/' reccfg.yaml
$ tctl create -f reccfg.yaml
session recording config has been updated
```

#### Configuring PAM custom environment variables

```
$ tctl get pam_config
kind: pam_config
metadata:
  id: 1618929344290245400
  name: pam-config
  labels:
    teleport.dev/origin: defaults
spec:
  enabled: false
version: v3

$ tctl create <<EOF
kind: pam_config
spec:
  enabled: true
  environment:
    FOO: "bar"
    EMAIL: "{{ external.email }}"
version: v3
EOF

PAM config has been updated
```

#### Setting auth preference to Auth0 OIDC

```
$ tctl get oidc/auth0
kind: oidc
metadata:
  name: auth0
spec:
  [...]

$ tctl create <<EOF
kind: cluster_auth_preference
spec:
  type: oidc
  connector_name: auth0
  second_factor: "off"
version: v2
EOF

cluster auth preference has been updated
```

#### Setting up U2F second factor

```
$ cat >u2fcap.yaml
kind: cluster_auth_preference
version: v2
spec:
  type: local
  second_factor: u2f
  u2f:
    app_id: https://proxy.example.com:443
    facets:
    - https://proxy.example.com:443
    - https://proxy.example.com
    - proxy.example.com:443
    - proxy.example.com

$ tctl create u2fcap.yaml
cluster auth preference has been updated
```

##### With U2F CA attestation

```
$ cat >>u2fcap.yaml
    # under spec.u2f
    device_attestation_cas:
    - "/path/to/u2f_attestation_ca.pem"

$ tctl create -f u2fcap.yaml
cluster auth preference has been updated
```

##### With cluster-wide per-session U2F checks

```
$ cat >>u2fcap.yml
  # under spec
  require_session_mfa: yes

$ tctl create -f u2fcap.yaml
cluster auth preference has been updated
```
