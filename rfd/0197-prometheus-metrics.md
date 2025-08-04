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

## Guidelines

### Metrics Registry

#### Do

- Take the local in-process registry as an argument in your service constructor/main routine, like you would receive a
  logger, and register your metrics against it.
- Pass the registry as a `promtetheus.Registerer`

```golang
func NewService(log *slog.Logger, reg prometheus.Registerer) (Service, error) {
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

#### Don't

- Register against the global prometheus registry
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

- Pass the registry as a `*prometheus.Registry`

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

### Storing metrics

#### Do

- Store metrics in a private struct in your package

```golang
type metrics struct {
    currentFoo prometheus.Gauge
    barCounter *prometheus.CounterVec
}

func newMetrics(reg prometheus.Registerer) (*metrics, error) {
    m := metrics{
        currentFoo: prometheus.NewGauge(prometheus.GaugeOpts{
            Namespace: teleport.MetricNamespace,
            Subsystem: metricsSubsystem,
            Name: "foo_current",
            Help: "Measures the number of foos.",
        }),
        barCounter: // ...
    }
    
    errs := trace.NewAggregate(
        reg.Register(m.currentFoo),
        reg.Register(m.barCounter),
    )
    
    if errs != nil {
        return trace.Wrap(err, "registering metrics")
    }
    
    return &m
}

```

#### Don't

- Store metrics in a package-scoped variable

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

### Naming metrics

#### Do

- Use `teleport.MetricsNamespace` as the namespace.
- Use a subsystem name unique to your component.
- Follow [the prometheus metrics naming guidelines](https://prometheus.io/docs/practices/naming/),
  especially always specify the unit and use a suffix to clarify the metric type (`_total`, `_info`).

```golang
type metrics struct {
    currentFoo prometheus.Gauge
    barCounter *prometheus.CounterVec
}

const metricsSubsystem = "my_service"

func newMetrics(reg prometheus.Registerer) (*metrics, error) {
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
```

#### Don't

- Manually namespace metrics
- Create non-namespaced metrics

```golang
type metrics struct {
    currentFoo prometheus.Gauge
    barCounter *prometheus.CounterVec
}

func newMetrics(reg prometheus.Registerer) (*metrics, error) {
    m := metrics{
        currentFoo: prometheus.NewGauge(prometheus.GaugeOpts{
            Name: "teleport_foo_timestamp_seconds",
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

### Metric registration

#### Do

- Use `reg.Register()` to register the metric.
- Aggregate errors and fail early if you can't register metrics.

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

## Enforcing the guidelines

Some guidelines can be enforced by setting up linters:
- [promlinter](https://golangci-lint.run/usage/linters/#promlinter) to ensure that metric naming and labeling follows 
  the Prometheus guidelines.
- [forbidigo](https://golangci-lint.run/usage/linters/#forbidigo) to reject usages of `prometheus.DefaultRegisterer`,
  `prometheus.(Must)Register`. `reg.MustRegister` conflicts with `backend.MustRegister`, we might not be able to detect
  it (`fobidigo.analyze-types` might not be sufficient)

Existing non-compliant metrics and edge case usages will be allowed via `//nolint` comments.