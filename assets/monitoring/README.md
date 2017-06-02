## Metrics

Teleport exports Prometheus Metrics endpoints.

**Start Teleport With Prometheus Endpoint**

To start teleport with prometheus endpoint enabled:

```
teleport start --diag-addr=127.0.0.1:3434
```

To start monitoring stack, simply `docker-compose up`

Then go to `http://localhost:3000`

```
Username: admin
Password: admin
```

### Grafana Dashboard

Modify and export grafana dashboard, then convert to the format that Grafana can auto import using this tool

```bash
python convert.py health-raw.json health-dashboard.json
```

## Low level monitoring

Teleport adds `gops` as a low level debugging solution:

```bash
teleport start --gops --gops-addr=127.0.0.1:4321
```

Then to use gops:

```bash
go get github.com/google/gops
gops stack $(pidof teleport
```

#### Diffing goroutine dumps

We have a tool to give you idea of the difference between two teleport stack dumps,
so we can see what's the overhead and difference to detect leaks:

```bash
gops stack $(pidof teleport) | python gops.py collect > /tmp/a
# take a second diff
gops stack $(pidof teleport) | python gops.py collect > /tmp/b
# compare two diffs
python gops.py diff /tmp/a /tmp/b
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


**Debugging the debugger**

Sometimes it is useful to see how many gorotuines `tsh bench` produces itself,
you can launch it with `gops` endpoint. (Used by https://github.com/google/gops) tool

```bash
tsh --gops --gops-addr=127.0.0.1:4322 bench --threads=100 --duration=300s --rate=10 localhost ls -l
# then use gops tool to inspect
gops stack <pid>
```
