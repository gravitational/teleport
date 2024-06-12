## Metrics

Teleport exports Prometheus Metrics endpoints.

**Start Teleport With Prometheus Endpoint**

To start teleport with prometheus endpoint enabled:

```bash
teleport start --diag-addr=127.0.0.1:3000
# http://127.0.0.1:3000/metrics
```

To start monitoring stack, simply `docker-compose up`

Then go to `http://localhost:3001`

```
Username: admin
Password: admin
```

### Grafana Dashboard

Modify and export Grafana dashboard, then convert to the format that Grafana can auto import using this tool

```bash
python convert.py health-raw.json health-dashboard.json
```

## Low level monitoring

Teleport can be started with Go's standard profiler [pprof](https://golang.org/pkg/net/http/pprof/).

```bash
$ teleport start -d --diag-addr=127.0.0.1:3000
# http://127.0.0.1:3000/debug/pprof/
```

When teleport is started in debug mode (with teleport start -d flag) Go’s CPU,
memory and go routines dumps could be collected on the host.

Assuming debugging endpoint address is set to `127.0.0.1:3000`, the following key profiles
can be collected:

CPU profile (it will observe the system for 30 seconds and collect metrics) about CPU usage
per function:

`curl -o cpu.profile http://127.0.0.1:3000/debug/pprof/profile`

Note: This curl command will hang for 30 seconds collecting the CPU profile

Goroutine profile shows how many concurrent Golang “lightweight threads” are used
in the system:

`curl -o goroutine.profile http://127.0.0.1:3000/debug/pprof/goroutine`

Heap profile shows allocated objects in the system:

`curl -o heap.profile http://127.0.0.1:3000/debug/pprof/heap`

To view the resulting profiles, use go tool pprof:

```
go tool pprof cpu.profile
go tool pprof heap.profile
go tool pprof goroutine.profile
# Use --web flag to create a SVG diagram in the browser, for high-level view of whats going on.
go tool pprof --web heap.profile
```

### Performance Testing

By default tsh bench does not create interactive sessions, but is using exec.

**Loging in**

You have to login before calling `tsh bench` using `tsh login`, otherwise
requests will fail.

**Non interactive mode**

E.g. this creates requests at a rate 10 requests per second
and uses a pool of 100 execution threads (goroutines in go) for 30 seconds

```bash
tsh bench --threads=100 --duration=300s --rate=10 localhost ls -l
```

**NOTE:** Algorithm does not apply backpressure if requests delay on purpose
(watch [this](https://www.infoq.com/presentations/latency-pitfalls) for more details about why).
In practice this means that you could pick a seemingly low rate value per second,
however it could trigger system outage because you will locate the system breaking
point and the amount of connections will blow up. Also times are measured from the point where
request was originated, and not dispacthed to the thread, so latency report is closer to
what real users will observe.


**Interactive mode**

This creates real interactive session, allocating PTY, calling `ls -l` and then `exit`:

```bash
tsh bench --interactive --threads=100 --duration=300s --rate=10 localhost ls -l
```

The performance difference is huge between interactive and non interactive modes.
