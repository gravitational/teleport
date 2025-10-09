# 0226 - Availability, Failure and Self-Healing

## What
Explore improvements to how Teleport responds to failure scenarios with a focus
on availability and self-healing. This involves ensuring that agents can detect
issues and recover without manual intervention.

## Why

We’ve identified several failure scenarios where agents can become temporarily
unreachable or unavailable. This risks disrupting customer access. While Teleport
has mechanisms to recover in many cases, there are still gaps where recovery is
slow or requires manual intervention.

Improving self-healing behavior in these scenarios will increase availability and
provide a better customer experience when failures inevitably occur.

## Details

### Current State of Failure Recovery

When a Teleport control plane component (Auth or Proxy) becomes unhealthy, the
agents connected to that component may become unavailable. Recovery in this
situation depends entirely on the specific service instance recovering. There is
no automated mechanism for agents to detect the failure and reconnect elsewhere.
While this may be acceptable in simple deployments, for environments that require
high availability it can result in service interruptions that should be handled
automatically.

Both Auth and Proxy services are considered unhealthy when they are unable to
heartbeat to the backend. When either service is unhealthy it should be taken
out of rotation from service load balancers.

Agents connected to an unhealthy auth service cannot send heartbeats and will
eventually expire from the backend. This makes them undiscoverable and unreachable
by the rest of the cluster.

Agents connected to an unhealthy Proxy service may or may not be available. Proxy
health only reflects an inability to write to the backend, not whether the Proxy
can continue serving agent connectivity. Unhealthy Proxies may still be connected
to and reachable by other Proxy instances.

### Desired State of Failure Recovery

Teleport should automatically detect when control plane components become unhealthy
and recover without manual intervention. Agents should be able to seamlessly
reconnect to healthy services to minimize downtime and prevent situations where
they remain stranded due to single service instance failures.

For Proxy services, ideally we would distinguish between heartbeat failures and
true agent connectivity failures to avoid unnecessary agent reconnects. However
reliably determining whether a Proxy can still service agent connectivity is complex.
As a pragmatic approach, we can base recovery decisions solely on the existing
heartbeat health mechanism.

### Auth Reconnects

Agents create a gRPC client connection to Auth. The transport of this connection
could be a direct TCP connection to Auth or a tunneled connection through the
Proxy. The solution should be indifferent to the transport.

We can communicate the health of the Auth instance an agent is connected to using
gRPC health checking[^1]. This is a standardized service API that gRPC servers can implement. Clients that enable health checking can then poll or stream the health
of the service.

This solves half the problem. Agents can enable health checking and can be aware
of when they are connected to an unhealthy Auth service. However the client will
not automatically reconnect in this scenario. Within gRPC client connections are
maintained by load balancing policies[^2].

Evaluating the default `pick_first` policy. It will only create a new connection
when the existing TCP connection is disrupted. When health checking is enabled
the client begins failing requests immediately rather than sending them to the
server but never attempts to reconnect.

Evaluating the `round_robin` policy, a connection is created to each address that
was returned by the resolver. This policy is able to send requests to healthy
connections based on health checks. However this assumes that each address represents
a unique server instance. This is not usually the case. Teleport is typically fronted by one or more layers of load balancers. This means there could be
a single address or connecting to multiple addresses could result in connections
to the same server instance.

In summary, neither the `pick_first` or the `round_robin` load balancing policies
will work for our use case. Other built-in policies are available but they also
do not meet our use case[^3].

The solution here is to implement our own load balancing policy. The go-grpc library
conveniently allows you to register custom policies[^2] by implementing the
balancer interface[^4]. This policy will enable gRPC health checking by default
and use the streaming API to ensure health checks can scale to a large number of
clients.

To avoid introducing too much new behavior we will base our implementation off of
the existing `pick_first` policy[^5]. The primary difference will be when the existing
connection enters the `TRANSIENT_FAILURE` state[^6] new connection attempts will be
made until either the existing connection returns to `READY` or the new connection
reaches the `READY` state.

If the new connection reaches the `READY` state it will replace the old connection
and the old connection will be shutdown.

Clients will discover the new load balancing policy via the `/webapi/ping` endpoint.

It will be set in the JSON response as follows:

```json
{
    "grpc_client_lb_policy": {
        "loadBalancingConfig": [{"teleport_pick_healthy": {}}],
        "healthCheckConfig": {
            "serviceName": ""
        }
    }
}
```

This can be configured on a Proxy server by setting the environment variable
`TELEPORT_UNSTABLE_GRPC_CLIENT_LB_POLICY`. When not specified clients will continue
using the default `pick_first` policy. This will allow us to opt-in to this new
behavior.

### Proxy Reconnects

The agent connections we are concerned with for Proxy reconnects are the
reversetunnel connections. These provide user connectivity to the agent. 
They are long-lived connections that only disconnect when the Proxy 
service itself shuts down or the underlying TCP connection is disrupted.

The reconnect procedure must be graceful as to not disrupt any existing user
connectivity to the agent and it should not decrease agent availability in the process.

We will use the reversetunnel server's Proxy discovery requests to communicate Proxy health.

Today Proxies will eventually be seen as unhealthy after a Proxy expires from the
backend and the agent tracker's default Proxy expiry is exceeded.

This can take ~13 minutes based on the Proxy ServerAnnounceTTL = 10 minutes and
the tracker.DefaultProxyExpirey = 3 minutes. To put this into perspective this is
roughly equal to the quarterly downtime budget when targeting `99.99` availability.

To speed up this process we will support configuring a lower `ServerAnnounceTTL`.

The environment variable `TELEPORT_UNSTABLE_SERVER_ANNOUNCE_TTL` will be used to
configure the `ServerAnnounceTTL. We can use this to configure more frequent Proxy and Auth
heartbeats to reduce the time to detect unhealthy instances.

This will decrease the time it takes to detect an unhealthy Proxy or Auth server.

We will also add an `ExpiryDuration` field to Proxy discovery requests. Agents can
than use this field in favor of the `DefaultProxyExpiry` when its present.

```
type Proxy struct {
	Version  string `json:"version"`
	Metadata struct {
		Name string `json:"name"`
	} `json:"metadata"`

	ProxyGroupID         string         `json:"gid,omitempty"`
	ProxyGroupGeneration uint64         `json:"ggen,omitempty"`
	ExpiryDuration       *time.Duration `json:"exp,omitempty"`
}
```

The `ExpiryDuration` for each server will be set based on the `ServerAnnounceTTL`
configured on the Proxy sending the discovery request.

Lowering the `ServerAnnounceTTL` for Proxy servers has the negative effect of
increasing the traffic created by Proxy discovery requests.

We can reduce the overall traffic created by Proxy discovery requests by changing
the behavior.

The new behavior will only send Proxy discovery requests when there is a Proxy
heartbeat and will only send the set of proxies that have heartbeated since the 
last Proxy discovery request was sent.

The first discovery request will always contain the full list of unexpired Proxy
servers.

Proxy heartbeats will be detected by looking for changes in the `server.Expiry()`
value when events are received by the `ProxyWatcher *services.GenericWatcher[types.Server, readonly.Server]`.

This behavior is backwards compatible with existing agent Proxy tracking.

If one of the Agents reversetunnels is connected to an expired Proxy. The agent will
begin creating new connections attempting to reach a non-expired Proxy.

Today the agent never closes reversetunnel connections. To solve this Agents will
evaluate whether they can disconnect from Proxies when they have more than the
desired connection count. Connections will only be closed if the Proxy is expired
or there are excess connections to the desired Proxy set.

### Additonal Thoughts and Considerations

#### Periodic Auth Reconnects
The custom load balancer policy described in [Auth Reconnects](#auth-reconnects)
could be extended to support periodic auth reconnects. This would help to balance
load across auth server instances and redistribute load after recovering from 
a failure.

There is one long lived stream for cache updates created by the agent
that we do not want to disrupt frequently as it could cause drastic increases
in network traffic. However given gRPC's graceful shutdown process this may not
be an issue. Assuming the streams are not a major contributor to the typical auth
load we see it may be beneficial to periodically move new requests to new connections
while leaving old connections open for long lived streams to continue.

As an alternative approach we could also configure a `MaxConnectionAge` on the server
side. This would force clients to disconnect after the specific age but may be less
graceful than the option described above.

#### Periodic Tunnel Reconnects
Reconnecting reversetunnels during a failure can lead to imbalanced and suboptimal
routing that should be addressed when the Teleport cluster recovers. To address
this we need a way to trigger periodic reconnects. This can be achieved by having
the proxy send reconnects at a specific interval + jitter.

The default behavior will remain the same where periodic reconnects are never sent.
We will add an environment variable `TELEPORT_UNSTABLE_REVERSETUNNEL_RECONNECT_INTERVAL`
that when set on a Proxy will enable the behavior.

Agents should reconnect to a non-expired proxy server before they close a tunnel
connection that received a reconnect signal.

#### Alternative Approaches to Auth Reconnects
Alternative approaches such as closing the agent's connection server side or
using a HTTP2 GOAWAY to signal an agent to reconnect were quickly rejected.
Closing the connection server side allows for no graceful draining behavior to
be used. Sending an HTTP2 GOAWAY is not exposed by the standard go http/grpc
libraries.

[^1]: https://grpc.io/docs/guides/health-checking/
[^2]: https://grpc.io/docs/guides/custom-load-balancing/
[^3]: https://pkg.go.dev/google.golang.org/grpc/balancer#section-directories
[^4]: https://pkg.go.dev/google.golang.org/grpc/balancer#Balancer
[^5]: https://github.com/grpc/grpc-go/blob/v1.75.1/balancer/pickfirst/pickfirstleaf/pickfirstleaf.go
[^6]: https://grpc.github.io/grpc/core/md_doc_connectivity-semantics-and-api.html
[^7]: https://github.com/gravitational/teleport/blob/4e19f750520d0ccf2c49ed109dc3c94383ec4765/lib/reversetunnel/agent.go#L640
[^8]: https://github.com/gravitational/teleport/blob/4e19f750520d0ccf2c49ed109dc3c94383ec4765/lib/reversetunnel/local_cluster.go#L809
[^9]: https://github.com/gravitational/teleport/blob/4e19f750520d0ccf2c49ed109dc3c94383ec4765/lib/reversetunnel/agent.go#L545-L550