## What

Defines ALPN SNI proxy configuration changes needed to support Teleport proxy running in single port mode.

## Why
The current Proxy configuration doesn't allow disabling a particular proxy listener in proxy configuration.
After introducing ALPN SNI listener allowing to multiplex all proxy service into one single port the legacy listener are no longer needed in order to simplify proxy configuration legacy per service listener can be removed.
## Details

### Proto ClusterNetworkingConfig Changes:

```protobuf
enum ProxyListenerMode {
   // Multiple is proxy listener mode indicating the proxy should use legacy per service listener mode.
   Multiple = 0;
   // multiplex is proxy listener mode indicating the proxy should use multiplex mode and all proxy services are multiplexed on a single proxy port.
   Multiplex = 1;
}
```

```protobuf
message ClusterNetworkingConfigSpecV2 {
   // ProxyListenerMode is proxy listener mode used by Teleport Proxies.
   ProxyListenerMode ProxyListenerMode = 3 [ (gogoproto.jsontag) = "proxy_listener_mode,omitempty" ];
}

```


### Proxy Ping Response Changes:

The Proxy ping response will be extended by the `ListenerMode` field used to propagate the current cluster `ProxyListenerMode` to proxy clients.
```go
// ProxyListenerMode is the proxy listener mode used by Teleport Proxies.
type ProxyListenerMode string

const (
    // ProxyListenerModeMultiple is proxy listener mode indicating the proxy per service listeners.
    ProxyListenerModeMultiple  ProxyListenerMode = "multiple"
    //ProxyListenerModeMultiplex is proxy listener mode indicating the proxy should use multiplex mode and all proxy service are multiplexed on single proxy port.
    ProxyListenerModeMultiplex                   = "multiplex"
)

type ProxySettings struct {
    // ...
    ListenerMode ProxyListenerMode `json:"listener_mode"`
}
```

### Teleport configuration changes:
#### Proxy `listener_mode` and Teleport config `V2`:

Teleport `v2` configuration will change default behavior when the listener's addresses are not provided.
The current implementation uses default settings if the listener's addresses are not provided. In order to disable
listeners the `v2` proxy configuration will be introduced. `v2` settings will change legacy behavior and by default, if listener addresses are not provided
proxy won't fall back to default allowing for disabling proxy services:

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
     listener_mode: multiple # multiple is default value for v1 config
      # missing listener means the listener is using default value
```

### Configuration Scenarios:
#### Scenario 1 - reverse tunnel connection reconfiguration.
1) All proxies in the cluster were configured in `multiple` proxy listener mode.
2) Database service establishes the connection to ReverseTunnel service on `tunnel_listen_addr` listener address.
3) User enables `ClusterNetworkingConfig.ProxyListenerMode` `Multiplex` listener mode.
4) Proxy clients based on multiplex ping response will start using a single master proxy port.
5) Database service reverse tunnel connection should be reconfigured and connection should be re-established to the master proxy port.


#### Scenario 2 - switching from `Multiplex` to `Multiple` proxy mode when proxies are configured with a single port listener.
1) All proxies in the cluster were configured in `multiplex` listener mode with legacy MySQL listener disabled and `ClusterNetworkingConfig.ProxyListenerMode` was set to `Multiplex` mode.
2) User wants switch `ClusterNetworkingConfig.ProxyListenerMode` proxy mode from `Multiplex` to `Multiple`
3) Proxy doesn't have a configuration for MySQL listener thus they won't be able to handle MySQL client connections thus setting ProxyListenerMode to `Multiple` should fail.

   *Question*: How to detect if `Multiple` mode can be changed? Auth service should be aware of all proxies' configuration to detect if per service listener were set and proxy mode can be listener mode can be degraded.

#### Scenario 3 - legacy proxy client:
1) Proxy was configured with `multiplex` mode with the only master port listener.
2) Legacy teleport client wants to connect to the proxy.
