## What

Defines ALPN SNI proxy configuration changes needed to support Teleport proxy running in single port mode.

## Why

The current Proxy configuration doesn't allow disabling a particular proxy listener in proxy configuration. After
introducing ALPN SNI listener allowing to multiplex all proxy service into one single port the legacy listener is no
longer needed in order to simplify proxy configuration legacy per service listener can be removed.

## Details

### Proto ClusterNetworkingConfig Changes:

```protobuf
enum ProxyListenerMode {
  // Separate is proxy listener mode indicating the proxy should use legacy per service listener mode.
  Separate = 0;
  // multiplex is proxy listener mode indicating the proxy should use multiplex mode and all proxy services are multiplexed on a single proxy port.
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
cluster `ProxyListenerMode` to proxy clients.

```go
// ProxyListenerMode is the proxy listener mode used by Teleport Proxies.
type ProxyListenerMode string

const (
    // ProxyListenerModeSeparate is proxy listener mode indicating the proxy per service listeners.
    ProxyListenerModeSeparate  ProxyListenerMode = "separate"
    //ProxyListenerModeMultiplex is proxy listener mode indicating the proxy should use multiplex mode and all proxy service are multiplexed on single proxy port.
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

- Teleport cluster network config (ClusterNetworkingConfigSpecV2) ProxyListenerMode is set to `single` mode.
- Teleport Proxy is configured in v2 single mode.
```yaml
   version: v2
   teleport:
      proxy:
        web_proxy_addr: 0.0.0.0:443
        tunnel_listen_addr: 0.0.0.0:3024 
        listener_mode: single 
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
The `ClusterNetworkingConfigSpecV2` `ProxyListenerMode` is changed from `single` to `multiple` mode.

Result:
DB Agent still is connected to the old reverse tunnel port where Cluster ProxyListenerMode was set to `single`  mode.

#### Solutions:
- Proxy restart.
- DB Agent restart.
- Dynamic reverse tunnel connection reconfiguration.


### Scenario 2 - switching from `Multiplex` to `Separate proxy mode when proxies are configured with a single port listener.

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

The `ClusterNetworkingConfigSpecV2` `ProxyListenerMode` is changed from `multiple` to `single` mode.

#### Result:

Changing cluster mode ProxyListenerMode to `single` mode when Teleport Proxies uses `v2` config
without `mysql_listen_addr` address configuration will make MySQL proxy service no longer available. `tsh db connect` command will
try to obtain and connect to MySQL single port listener but `mysql_listen_addr` address is missing from proxy configuration thus MySQL CLI is unable to
reach proxy service.


### Scenario 3 - legacy proxy client:
#### Precondition:
- Teleport cluster network config (ClusterNetworkingConfigSpecV2) ProxyListenerMode is set to `single` mode.
- Teleport Proxy is configured in v2 single mode.
```yaml
   version: v2
   teleport:
      proxy:
        web_proxy_addr: 0.0.0.0:443
        listener_mode: single 
```

#### Action:
Legacy `tsh` client wants to connect to the proxy configured.

#### Result:
`tsh` client is unable to connect to the Teleport Proxy running in `single` v2 mode.


