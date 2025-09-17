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
the existing `pick_first` policy[^5]. The difference is when the existing connection enters the `TRANSIENT_FAILURE` state[^6] an additional connection will be created.

If the new connection reaches the `READY` state it will replace the old connection
and the old connection will be shutdown.

If the new connection fails to connect or hits an unhealthy service it will be
shutdown and another new connection will be created.

If the old connection recovers before a new connection is able to reach the `READY`
state, we will stop creating new connections and begin using the old connection again.

### Proxy Reconnects

The agent connections we are concerned with for Proxy reconnects are the
reversetunnel connections. These provide user connectivity to the agent. 
They are long-lived connections that only disconnect when the Proxy 
service itself shuts down or the underlying TCP connection is disrupted.

The reconnect procedure must be graceful as to not disrupt any existing user
connectivity to the agent and it should not decrease agent availability in the process.

To avoid disrupting existing user connectivity, a reversetunnel should
not be closed until all active user traffic has been drained from the
connection.

To avoid decreasing availability we should not stop accepting new sessions on a reversetunnel until a new connection is established. 

To implement this reconnect procedure we can leverage the `reconnect` signal[^7] that
already exists and is sent by Proxy servers during termination.

Proxy services will begin sending reconnect signals to agents after the proxy
instance is unhealthy for a configurable duration. The signal will only be sent
in response to an agent heartbeat[^8] to avoid sending the signal to all connected
agents at the same time.

Currently, agents stop accepting new transport requests while draining[^9]. This
could impact agent availability if we begin to drain and an agent is unable to
establish a new connection to the cluster. To resolve this issue, after receiving a
reconnect request, agents will first establish a new connection to a healthy proxy 
instance. Only then will the agent begin to drain the connection and stop accepting
new transport requests.

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

#### Periodic Tunnel Reconnects
With the improved agent draining behavior described in [Proxy Reconnects](#proxy-reconnects)
it should be safe to introduce periodic reversetunnel reconnects. This would
allow for better load distribution and the most up-to-date low latency routing paths
for these connections. There is some risk that long lived sessions could keep draining
tunnels open for a very long period of time. Given enough of these types of sessions
the total number of open tunnels would slowly climb and potentially cause issues.

#### Alternative Approaches to Auth Reconnects
Alternative approaches such as closing the agent's connection server side or
using a HTTP2 GOAWAY to signal an agent to reconnect were quickly rejected.
Closing the connection server side allows for no graceful draining behavior to
be used. Sending an HTTP2 GOAWAY is not exposed by the standard go http/grpc
libraries.

#### Alternative Approaches to Proxy Reconnects
Having a proxy stop advertising proxy addresses or stop advertising itself was brought up as a possible way to trigger the agent to begin attempting to reconnect without agent side changes. This did not end up working as expected. When no proxies
are advertised the agent does not update its tracked proxy cache. When a proxy stops
advertising itself, this does not trigger the agent to disconnect or give up its
lease on the proxy. There is also the issue that other proxies may still be sending
discovery requests for the agent to receive.

[^1]: https://grpc.io/docs/guides/health-checking/
[^2]: https://grpc.io/docs/guides/custom-load-balancing/
[^3]: https://pkg.go.dev/google.golang.org/grpc/balancer#section-directories
[^4]: https://pkg.go.dev/google.golang.org/grpc/balancer#Balancer
[^5]: https://github.com/grpc/grpc-go/blob/v1.75.1/balancer/pickfirst/pickfirstleaf/pickfirstleaf.go
[^6]: https://grpc.github.io/grpc/core/md_doc_connectivity-semantics-and-api.html
[^7]: https://github.com/gravitational/teleport/blob/4e19f750520d0ccf2c49ed109dc3c94383ec4765/lib/reversetunnel/agent.go#L640
[^8]: https://github.com/gravitational/teleport/blob/4e19f750520d0ccf2c49ed109dc3c94383ec4765/lib/reversetunnel/local_cluster.go#L809
[^9]: https://github.com/gravitational/teleport/blob/4e19f750520d0ccf2c49ed109dc3c94383ec4765/lib/reversetunnel/agent.go#L545-L550