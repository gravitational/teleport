---
authors: Tim Ross (tim.ross@goteleport.com)
state: implemented (v10.1.0)
---

# RFD 65 - Distributed Tracing

# Required Approvers
* Engineering: @zmb3 && (@fspmarshall || @espadolini)
* Product: (@klizhentas || @xinding33)

## What

Add distributed tracing to Teleport by leveraging [OpenTelemetry](https://github.com/open-telemetry/opentelemetry-go)
and its various libraries to create spans and export them in the
[OpenTelemetry Protocol (OTLP)](https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/protocol/otlp.md).

## Why

We don't currently have much insight into areas of latency within Teleport. Consider the following questions:

> _"Why is it taking so long to run `tsh ls`?"_
>
> _"Why does it take N seconds to connect via `tsh ssh`?"_

Today we can only guess what the cause is by trying to find logs that suggest where the latency is coming from. Not all
latency issues are straightforward to reproduce either. Some issues may be specific to the environment, or a
deployment strategy, which we may not be able to easily/reliably reproduce. At the time of this
writing there are a few open issues related to latency that have yet to be addressed:
[#11159](https://github.com/gravitational/teleport/issues/11159),
[#8149](https://github.com/gravitational/teleport/issues/8145),
[#7815](https://github.com/gravitational/teleport/issues/7815).


Distributed tracing provides full visibility into the life of a request across all the different service
boundaries it may pass. By adding distributed tracing to Teleport we would be able to see precisely what is occurring
and for how long across all the different Teleport services. With this level of observability we would gain the ability
to detect and identify areas of concern before a customer finds them. It would also allow us to capture and observe
spans from the customers' environment and reduce the burden we currently face to try and reproduce the latency issues.

## Details

### Goals

* Generate and export spans for Teleport service boundaries and core components
  (e.g. http client/server, grpc client/server, ssh client/server, cache, backend)
* Provide a solid foundation for how to use tracing within Teleport so that more areas of the
  code can easily be instrumented in the future

### Non-Goals

* Adding tracing to the entire codebase all at once
* Replace existing logging, metrics
* Change metrics or logging (e.g. to support trace-metric correlation)
* Adding a mechanism to serve and view traces

### Tracing Requests and Exporting Spans

#### Definitions

- **[Span](https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/trace/api.md#span)**:
  Represents a single operation within a trace. Spans can be nested to form a trace tree. Each trace contains a root span
  and one or more sub-spans for any sub-operations.
- **[Trace](https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/overview.md#traces)**:
  A DAG of Spans, where the edges between Spans are defined as parent/child relationship.
- **[Span Context](https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/trace/api.md#spancontext)**:
  Represents all the information that identifies Span in the Trace and MUST be propagated to child Spans and across
  process boundaries..

An example of a trace tree:
```
        [Span A]  ←←←(the root span)
            |
     +------+------+
     |             |
[Span B]      [Span C] ←←←(Span C is a `child` of Span A)
    |             |
[Span D]      +---+-------+
              |           |
          [Span E]    [Span F]
```
The same trace tree viewed with its relation to time:
```
––|–––––––|–––––––|–––––––|–––––––|–––––––|–––––––|–––––––|–> time

 [Span A···················································]
   [Span B··········································]
      [Span D······································]
    [Span C····················································]
         [Span E·······]        [Span F··]

```

#### Context Propagation

In order to properly correlate various **Spans** to the appropriate **Trace** the **SpanContext** must be propagated.
While this is especially true for spans that cross different service boundaries, it is also necessary for relating spans
within a service as well. OpenTelemetry achieves this by embedding the **SpanContext** into Go's `context.Context`. At
service boundaries a number of standard propagation mechanisms (W3C Trace Context, W3C Baggage, B3, Jaeger) can be used
to forward the **SpanContext**. For more details on OpenTelemetry Propagators see https://opentelemetry.io/docs/reference/specification/context/api-propagators/.

This means that any functions within Teleport that end up making a network call **must** take a `context.Context` as the
first parameter. While this has been idomatic Go regarding network calls for a while, it is not something that Teleport
currently abides by. Take the [`auth.ClientI`](https://github.com/gravitational/teleport/blob/632d851783ac5dbd7d1ba60f4fce8c95d81041e3/lib/auth/clt.go#L1925)
interface, which is used to abstract making calls to the `auth` service, as an example. If we look at just the first few
functions defined within the interface more than half don't take a `context.Context` as the first parameter.

```go
// ClientI is a client to Auth service
type ClientI interface {
	// NewKeepAliver returns a new instance of keep aliver
	NewKeepAliver(ctx context.Context) (types.KeepAliver, error)

	// RotateCertAuthority starts or restarts certificate authority rotation process.
	RotateCertAuthority(req RotateRequest) error

	// RotateExternalCertAuthority rotates external certificate authority,
	// this method is used to update only public keys and certificates of the
	// the certificate authorities of trusted clusters.
	RotateExternalCertAuthority(ca types.CertAuthority) error

	// ValidateTrustedCluster validates trusted cluster token with
	// main cluster, in case if validation is successful, main cluster
	// adds remote cluster
	ValidateTrustedCluster(context.Context, *ValidateTrustedClusterRequest) (*ValidateTrustedClusterResponse, error)

	// GetDomainName returns auth server cluster name
	GetDomainName() (string, error)

	// GetClusterCACert returns the PEM-encoded TLS certs for the local cluster.
	// If the cluster has multiple TLS certs, they will all be concatenated.
	GetClusterCACert() (*LocalCAResponse, error)
}
```

One of the biggest obstacles to getting **Spans** to correlate to the appropriate **Trace** will be plumbing the
`context.Context` throughout the entire Teleport codebase. Due to the number of files that will be impacted it is
probably best to address a few functions within an interface at a time and slowly move towards full propagation of
`context.Context`. While this is _necessary_, it is not a **blocker** to adding tracing capabilities to Teleport - it
only impacts the ability to correctly populate the trace tree.

#### Auto-instrumentation

While it is not possible to truly get automatic instrumentation in Go, we can get a decent amount of tracing for free
by leveraging the existing [OpenTelemetry Go](https://github.com/open-telemetry/opentelemetry-go-contrib) libraries.
We will specifically be using the `otelhttp` and `otelgrpc` libraries which automatically create spans that track inbound
and outbound requests for `net/http` and `grpc`. These were chosen because they will provide us with proper Span Context
propagation across the various Teleport services.

In the future we may want to use other automatic instrumentation libraries like `otelaws` or `otelsql` to get better
observability into certain parts of the code. Adding them should be on an as needed basis to help gain observability.

#### Manual Instrumentation

In order to instrument packages which don't already have a library available (e.g. golang.org/x/crypto/ssh) we will
have to manually create them. All Teleport application code will also have to be manually instrumented. In addition to
adding instrumentation to service boundaries, we will first trace the code paths along `tsh ssh` and `tsh ls` as these
are two areas which have had the most scrutiny about latency by customers. From there further instrumentation can be
added to other areas of the codebase, using the code already instrumented as a guide for best practices.

All spans created to instrument Teleport shall follow the guidelines laid out in the
[spec](https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/trace/api.md#span).
Likewise, all attributes shall follow the [guidelines](https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/common/common.md#attribute)
to provide consistency within Teleport and across other services.

#### Exporting Spans

All spans created by Teleport will be exported in the
[OpenTelemetry Protocol (OTLP)](https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/protocol/otlp.md)
format. OTLP is a vendor neutral binary format which is used to export traces and metrics to a variety of
different backends. Due to the fact that different Teleport customers will have different telemetry backends this
provides us a way to export traces in such a way that they can be easily consumed all customers. Most of the major
telemetry backends have already, or are working on adding support to consume OTLP spans. Customers also have the option
to use the [OpenTelemetry Collector (otel-collector)](https://github.com/open-telemetry/opentelemetry-collector) to
receive, process, and export the traces from Teleport to their desired backend. This is particularly useful for telemetry
backends which don't currently support OTLP. The otel-collector could be configured to retrieve spans from Teleport and
then forward them the backend in a format that the backend supports.


##### Tracing Configuration
```yaml
tracing:
  enabled: true
  # the url of the exporter to send spans to - can be http/https/grpc
  exporter_url: "http://localhost:14268/api/traces"
  # the number of spans to sample per million
  sampling_rate_per_million: 1000000
```

Teleport would consume the above configuration to create a [`TracerProvider`](https://pkg.go.dev/go.opentelemetry.io/otel/trace#TracerProvider)
and [`SpanExporter`](https://pkg.go.dev/go.opentelemetry.io/otel/sdk/trace#SpanExporter) which would only record 10% of
all spans and export them to the specified url.

Support will be added for exporting spans via `gRPC`, `http`, `https` and to the filesystem. Example configurations:

```yaml
  # export via gRPC
  exporter_url: "grpc://localhost:14268"
  
  # export via http
  exporter_url: "http://localhost:14268/api/traces"
  
  # export via https
  exporter_url: "https://localhost:14268/api/traces"
  
  # export to /var/lib/teleport/traces
  exporter_url: "file:///var/lib/teleport/traces"
```

#### Exporting Spans from tsh/tctl/tbot

In order to propagate spans that originated from `tctl`, `tsh`, or `tbot` we have two options:

1) Export the spans directly into the telemetry backend
2) Make the Auth Server a span forwarder

The first option is suboptimal for a variety of reasons. The telemetry backend may not be accessible from the machine
running `tsh`, it might only be accessible within the Data Center or Cloud VPC that Teleport is deployed in. It would
also require that end users know what the `exporter_url` is and how to set it correctly via a CLI flag. This is both a
poor user experience and adds operational friction to cluster administrators having to provide the correct configuration
to all users they want to capture traces for.

The forwarding option would make capturing traces something end users don't even need to know about. The Auth server
would expose a new RPC endpoint which `tsh`, `tctl`, `tbot` would export traces to right before the command exited.
The new endpoint would behave much the same as the OpenTelemetry Collector does in that it would simply forward the
received spans to the configured `exporter_url`.

 ```protobuf
service AuthService {
  // The rest of the AuthService is omitted for clarity
  
  // ExportSpans receives spans and forwards them to the configured exporter, if one is provided, otherwise it is a no-op.
  rpc ExportSpan(ExportSpanRequest) returns (ExportSpanResponse) {}
}

message ExportSpanRequest {
  // An array of OpenTelemetry [ResourceSpans](https://github.com/open-telemetry/opentelemetry-proto/blob/3c2915c01a9fb37abfc0415ec71247c4978386b0/opentelemetry/proto/trace/v1/trace.proto#L47).
  repeated opentelemetry.proto.trace.v1.ResourceSpans resource_spans = 1;
}

message ExportSpanResponse {}
```

By default, no spans will be captured and exported from the various tools mentioned above. In order to initiate collecting spans the
`--trace` flag must be explicitly provided. For example `tsh --trace ssh root@ubuntu` will capture and forward all spans
to the auth server for further propagation on to the telemetry backend. When the `--trace` flag is provided the sampling
rate will be set to `1`, meaning record all spans. Teleport will respect the sampling rate of remote spans, meaning that
even if the configured sampling rate of Teleport is `0`, when it receives a request which has a remote span already set
to be sampled, then all spans in response to said request will also be sampled and exported. This allows the full trace
to be captured in response to `--trace` even if Teleport wouldn't have otherwise sampled the spans.

We could further limit the number of spans that got exported from the Auth server by requiring the user to have a certain
role. Without having the role, the Auth server would log a warning and return instead of forwarding the spans on to the
telemetry backend.

An optional `--trace-exporter` flag could also be provided to allow users to direct their traces to a particular exporter.
If not provided, and `--trace` was set then traces would be forwarded to the Auth server. Allowing users to supply their
own exporter could be helpful for debugging purposes when users want to use a file exporter.

### Security

Turning Teleport into a span forwarder could be misused by malicious users to potentially inject fake spans, or try and
overwhelm the telemetry backend. Limiting who has the ability to exports spans to a particular role would reduce the
attack surface.

All forwarded spans will have the identity that exported that spans added as the attribute `teleport.forwarded.for`. This 
will allow an upstream collectors to potentially filter out spans.

As with logging, there is a potential to leak sensitive information in the span attributes and events.

### UX

Enabling tracing will be entirely opt-in by the user. When tracing is enabled it should be entirely transparent to all
users. There may be some performance issues if the configured sampling rate is too high. For example, sampling 100% of
the traces will cause every span originated to be recorded and exported. In a large and heavily used cluster that could
result in a significant amount of data being exported.

In order to export spans from `tctl`, `tsh` and `tbot` they will need to forward them to the Auth server prior
to the command exiting. The number of spans generated client side should be small enough that flushing all the spans
prior to exit should not be noticeable to the user.