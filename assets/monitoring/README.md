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

