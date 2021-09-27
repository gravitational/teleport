---
authors: Marek Smoliński (marek@goteleport.com)
state: draft
---

## What

Defines ALPN SNI proxy configuration changes allowing to start Teleport proxy in with only one open port (multiplex mode).

## Why

The current Proxy configuration doesn't allow disabling a particular proxy listener in proxy configuration - default ports values are used if not provided. After
introducing ALPN SNI listener allowing to multiplex all proxy service into one single proxy port the legacy listeners are no
longer needed.

## Details

### Proto ClusterNetworkingConfig Changes:

```protobuf
enum ProxyListenerMode {
  // Separate is proxy listener mode indicating the proxy should use legacy per service listener mode.
  Separate = 0;
  // multiplex is proxy listener mode indicating the proxy should use multiplex mode
  // and all proxy services are multiplexed on a single proxy port.
  Multiplex = 1;
}
```

```protobuf
message ClusterNetworkingConfigSpecV2 {
  // ProxyListenerMode is proxy listener mode used by Teleport Proxies.
  ProxyListenerMode ProxyListenerMode = 3 [(gogoproto.jsontag) = "proxy_listener_mode,omitempty"];
}

```

### Proxy Ping Response Changes:

The Proxy ping response will be extended by the `ListenerMode` field used to propagate the current
cluster `ProxyListenerMode` value.

```go
// ProxyListenerMode is the proxy listener mode used by Teleport Proxies.
type ProxyListenerMode string

const (
    // ProxyListenerModeSeparate is proxy listener mode indicating the proxy per service listeners.
    ProxyListenerModeSeparate  ProxyListenerMode = "separate"
    // ProxyListenerModeMultiplex is proxy listener mode indicating the proxy should use multiplex mode
	// and all proxy service are multiplexed on single proxy port.
    ProxyListenerModeMultiplex ProxyListenerMode = "multiplex"
)

type ProxySettings struct {
    // ...
    ListenerMode ProxyListenerMode `json:"listener_mode"`
}
```

### Teleport configuration changes:

#### Proxy `listener_mode` and Teleport config `V2`:

Teleport `v2` configuration will change default behavior when the listener's addresses are not provided. The current
implementation uses default settings if the listener's addresses are not provided. In order to disable listeners,
the `v2` proxy configuration will be introduced. `v2` settings will change legacy behavior and by default, if listener
addresses are not provided proxy won't fall back to default allowing for disabling proxy services:

```yaml
version: v2
teleport:
  proxy:
    listener_mode: multiplex # multiplex is default value for v2 config
    # missing listeners means the listener is not activated
```

```yaml
version: v1 # default v1
teleport:
  proxy:
    listener_mode: separate # separate is default value for v1 config
    # missing listener means the listener is using default value
```

### Configuration Scenarios:

#### Scenario 1 - reverse tunnel connection reconfiguration.
#### Precondition:

- Teleport cluster network config (ClusterNetworkingConfigSpecV2) ProxyListenerMode is set to `separate` mode.
- Teleport Proxy is configured in v2 separate mode.
```yaml
   version: v2
   teleport:
      proxy:
        web_proxy_addr: 0.0.0.0:443
        tunnel_listen_addr: 0.0.0.0:3024
        listener_mode: separate
```

```

                                                         Proxy
                                                      Multiplex Mode
                                                ┌───────────────────────┐
                                                │3080 Web Port          │
                                                │                       │
                                       ┌───────►│3024 Tunnel port       │
                                       │        │                       │
                                       │        │                       │
                                       │        └───────────────────────┘
  ┌───────────────────┐                │
  │                   │  Reverse Tunnel│Connection
  │  DB Agent         ├────────────────┘
  │                   │
  └───────────────────┘
```
#### Action:
The `ClusterNetworkingConfigSpecV2` `ProxyListenerMode` is changed from `separate` to `multiplex` mode.

Result:
DB Agent is still connected to the old reverse tunnel port.

#### Solutions:
- DB Agent restart.
- Adding support for dynamic reverse tunnel connection reconfiguration.


### Scenario 2 - switching from `multiplex` to `separate` proxy mode when Teleport proxy is configured only with multiplex port.

#### Precondition:

- Teleport cluster network config (ClusterNetworkingConfigSpecV2) ProxyListenerMode is set to `multiplex` mode.

- Teleport Proxy is configured in v2 multiplex mode.
   ```yaml
   version: v2
   teleport:
      proxy:
        web_proxy_addr: 0.0.0.0:443
        listener_mode: multiplex
   ```

- Client uses MySQL CLI to connect to DB instance through Proxy configured with multiplex mode.

```
                                          Proxy
                                       Multiplex Mode
                                   ┌─────────────────────┐
                                   │                     │
    ┌────────────┐                 │443  Multiplex       │
    │  mysql cli ├────────────────►│       Port          │
    └────────────┘                 │                     │
                                   │                     │
                                   └─────────────────────┘
```

#### Action:

The `ClusterNetworkingConfigSpecV2` `ProxyListenerMode` is changed from `multiple` to `separate` mode.

#### Result:

Changing cluster mode ProxyListenerMode to `single` mode when Teleport Proxies uses `v2` config
without `mysql_listen_addr` address configuration will make MySQL proxy service no longer available. `tsh db connect` command will
try to obtain and connect to MySQL single port listener but `mysql_listen_addr` address is missing from proxy configuration thus MySQL CLI is unable to
reach proxy service.


### Scenario 3 - legacy tsh client without ALPN SNI support:
#### Precondition:
- Teleport cluster network config (ClusterNetworkingConfigSpecV2) ProxyListenerMode is set to `multiplex` mode.
- Teleport Proxy is configured in v2 multiplex.
```yaml
   version: v2
   teleport:
      proxy:
        web_proxy_addr: 0.0.0.0:443
        listener_mode: multiplex
```

#### Action:
Legacy `tsh` client without support for ALPN dialer wants to connect to the proxy configured.

#### Result:
`tsh` client is unable to connect to the Teleport Proxy running in `single` v2 mode.


