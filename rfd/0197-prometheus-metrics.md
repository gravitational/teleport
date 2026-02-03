---
authors: Hugo Hervieux (hugo.hervieux@goteleport.com)
state: draft
---

# RFD 197 - Prometheus metrics guidelines

## Required Approvers

* Engineering: @codingllama && @rosstimothy && @zmb3

## What

This RFD covers the recommended way of adding metrics to Teleport and other services such as
tbot, the Teleport Kube Agent Updater, the Teleport Kubernetes Operator, or plugins
(slack, teams, pagerduty, event-handler, ...).

## Why

Every component currently relies on the global metrics registry and package-scoped metrics.
This poses several challenges:
- We pick up and serve arbitrary metrics declared by any of our dependencies. This can cause registration conflicts
  (errors, flakiness, panics), high cardinality, confusing metric names, and data loss.
- Several Teleport components cannot run in the same go process without causing metric conflicts. This is the case in
  integration tests (running both tbot and Teleport), or even in user-facing binaries (tctl uses `embeddedtbot`).
- The same Teleport component cannot be started multiple times without causing metric conflicts, or inaccurate metrics.
  For example, integration tests starting several Teleport instances will merge their metrics together.

## Conflicts

Metrics conflicts can happen in several cases:

1. The same metric is registered with different labels by several teleport processes. This is what happens with the
   grpc metrics currently as teleport and tbot register global metrics with the same name but different labels. This can
   be avoided in tests by using per-process registries.
2. The same metric is registered several times by different components. This can be avoided by namespacing and 
   adding metrics subsystem.
3. The same metric is registered both in the local and global registries. In this case, the process registry takes 
   precedence, the gathering succeeds but logs errors.
4. The same component is being started and stopped several times and re-register its metrics. This is the case for 
   hosted plugins. We work around by creating a dedicated registry, and registering/unregistering it as a collector. 
   See https://github.com/prometheus/client_golang/pull/1766 for an example.

## Guidelines

### Metrics Registry

#### Do

- Take the local in-process registry as an argument in your service constructor/main routine, like you would receive a
  logger, and register your metrics against it.
- Pass the registry as a `*metrics.Registry`

```golang
func NewService(log *slog.Logger, reg *metrics.Registry) (Service, error) {
    myMetric := prometheus.NewGauge(prometheus.GaugeOpts{
        Namespace: reg.Namespace(),
        Subsystem: reg.Subsystem(),
        Name: "my_metric",
        Help: "Measures the number of foos doing bars.",
    }),
    if err := reg.Register(myMetric); err != nil {
        return nil, trace.Wrap(err, "registering metric")
    }
    // ...
}
```

#### Don't

- <details>
  <summary>Register against the global prometheus registry</summary>
  
  ```golang
  func NewService(log *slog.Logger) (Service, error) {
      myMetric := prometheus.NewGauge(prometheus.GaugeOpts{
          Namespace: teleport.MetricNamespace,
          Subsystem: metricsSubsystem,
          Name: "my_metric",
          Help: "Measures the number of foos doing bars.",
      }),
      if err := prometheus.Register(myMetric); err != nil {
          return nil, trace.Wrap(err, "registering metric")
      }
      // ...
  }
  ```
  </details>
- <details>
  <summary>Pass the registry as a `*prometheus.Registry` or `prometheus.Registerer`</summary>

  ```golang
  func NewService(log *slog.Logger, reg *prometheus.Registry) (Service, error) {
      myMetric := prometheus.NewGauge(prometheus.GaugeOpts{
          Namespace: teleport.MetricNamespace,
          Subsystem: metricsSubsystem,
          Name: "my_metric",
          Help: "Measures the number of foos doing bars.",
      }),
      if err := reg.Register(myMetric); err != nil {
          return nil, trace.Wrap(err, "registering metric")
      }
      // ...
  }
  ```
  </details>

### Storing metrics

#### Do

- Store metrics in a private struct in your package

```golang
type fooMetrics struct {
    currentFoo prometheus.Gauge
    barCounter *prometheus.CounterVec
}

type fooService struct {
    metrics *fooMetrics
}

func newMetrics(reg *metrics.Registry) (metrics, error) {
    m := metrics{
        currentFoo: prometheus.NewGauge(prometheus.GaugeOpts{
            Namespace: teleport.MetricNamespace,
            Subsystem: metricsSubsystem,
            Name: "foo_current",
            Help: "Measures the number of foos.",
        }),
        barCounter: // ...
    }
    
    return &m
}

```

#### Don't

- <details>
  <summary>Store metrics in a package-scoped variable</summary>

  ```golang
  var (
      currentFoo = prometheus.NewGauge(prometheus.GaugeOpts{
              Namespace: teleport.MetricNamespace,
              Subsystem: metricsSubsystem,
              Name: "foo_current",
              Help: "Measures the number of foos.",
          })
      // ...
  )
  ```
</details>

### Naming metrics

#### Do

- Honour the namespace and subsystem from the `metrics.Registry`
- Wrap the `metrics.Registry` to add component-level information to the metrics subsystem. 
- Follow [the prometheus metrics naming guidelines](https://prometheus.io/docs/practices/naming/),
  especially always specify the unit and use a suffix to clarify the metric type (`_total`, `_info`).

```golang
type metrics struct {
    currentFoo prometheus.Gauge
    barCounter *prometheus.CounterVec
}

func newMetrics(reg *metrics.Registry) (*metrics, error) {
    m := metrics{
        currentFoo: prometheus.NewGauge(prometheus.GaugeOpts{
            Namespace: teleport.MetricNamespace,
            Subsystem: metricsSubsystem,
            Name: "foo_timestamp_seconds",
            Help: "Represents the foo time, in seconds.",
        }),
        barCounter: prometheus.NewCounter(prometheus.GaugeOpts{
            Namespace: teleport.MetricNamespace,
            Subsystem: metricsSubsystem,
            Name: "bar_total",
            Help: "Number of times bar happened.",
        }),
    }
    // ...
}

func newService(reg *metrics.Registry) {
	go runComponentA(reg.Wrap("component_a"))
}

func newComponentA(reg *metrics.Registry) {
    m := newMetrics(reg)
	err := m.register(reg)
}
```

#### Don't

- Manually namespace metrics
  <details>
  <summary>Manually namespace metrics or create non-namespaced metrics</summary>
  
  ```golang
  type metrics struct {
      currentFoo prometheus.Gauge
      barCounter *prometheus.CounterVec
  }
  
  func newMetrics(reg prometheus.Registerer) (*metrics, error) {
      m := metrics{
          currentFoo: prometheus.NewGauge(prometheus.GaugeOpts{
  			Namespace: "teleport"
              Name: "foo_timestamp_seconds",
              Help: "Represents the foo time, in seconds.",
          }),
          barCounter: prometheus.NewCounter(prometheus.GaugeOpts{
              Name: "bar_total",
              Help: "Number of times bar happened.",
          }),
      }
      // ...
  }
  ```
  </details>

### Metric registration

#### Do

- Use `reg.Register()` to register the metric.
- Aggregate errors and fail early if you can't register metrics?

#### Don't

- Use the package helpers `prometheus.Register()` or `prometheus.MustRegister`.
- Use `reg.MustRegister` as it panics in case of conflict and doesn't clearly indicate which metric descriptor conflicted.

### Labeling

#### Do

- Ensure no secret information is present in the labels.
- Avoid high cardinality labels

#### Don't

- Put user input directly in the labels
- Create large (1k+) metric combinations
- Create metrics with an ever growing number of labels

## Enforcing the guidelines

Some guidelines can be enforced by setting up linters:
- [promlinter](https://golangci-lint.run/usage/linters/#promlinter) to ensure that metric naming and labeling follows 
  the Prometheus guidelines. Note: we might not be able to use the strict mode as namespaces and subsystems are
  passed from the caller.
- [forbidigo](https://golangci-lint.run/usage/linters/#forbidigo) to reject usages of `prometheus.DefaultRegisterer`,
  `prometheus.(Must)Register`. `reg.MustRegister` conflicts with `backend.MustRegister`, we might not be able to detect
  it (`forbidigo.analyze-types` might not be sufficient)

Existing non-compliant metrics and edge case usages will be allowed via `//nolint` comments.
