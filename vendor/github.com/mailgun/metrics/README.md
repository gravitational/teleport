metrics
=======

Go library for emitting metrics to StatsD.

Originally: https://github.com/cactus/go-statsd-client

Usage example
-------------
```go
m = metrics.NewWithOptions("hostname:8125",
                           "servicename.hostname",
                           metrics.Options{UseBuffering: true})

m.Inc(m.Metric("cache", "hit"), int64(hit), 1)
m.Gauge(m.Metric("cache", "size"), int64(len(cache)), 1)
```
