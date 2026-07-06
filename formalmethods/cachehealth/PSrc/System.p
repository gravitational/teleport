event eReport: (who: machine, healthy: bool);
event eStop;
event eDeregister: machine;
event eSetHealth: bool;

machine MetricBug {
    var healthy: bool;

    start state Reporting {
        entry {
            healthy = false;
        }

        on eReport do (r: (who: machine, healthy: bool)) {
            healthy = r.healthy;
            announce eMetricChanged, healthy;
        }

        ignore eDeregister;
    }
}

machine Metric {
    var health: map[machine, bool];

    start state Init {
        entry {
            goto Reporting;
        }
    }

    state Reporting {
        on eReport do (r: (who: machine, healthy: bool)) {
            health[r.who] = r.healthy;
            announce eMetricChanged, anyHealthy();
        }

        on eDeregister do (who: machine) {
            health -= who;
            announce eMetricChanged, anyHealthy();
        }
    }

    fun anyHealthy(): bool {
        var k: machine;

        // If no cache is up, report healthy. This is a valid state and
        // alternative to deleting this metric.
        if (sizeof(health) == 0) {
            return true;
        }

        // If any cache is is healthy, report healthy status.
        foreach (k in keys(health)) {
            if (health[k]) {
                return true;
            }
        }

        // If nothing is healthy, then report unhealthy.
        return false;
    }
}

machine Cache {
    var metric: machine;

    start state Init {
        entry (m: machine) {
            metric = m;
            goto Healthy;
        }
    }

    state Healthy {
        entry {
            send metric, eReport, (who = this, healthy = true);
            announce eCacheHealth, (who = this, healthy = true);
        }

        on eSetHealth do (healthy: bool) {
            if (!healthy) {
                goto Unhealthy;
            }
        }

        on eStop do {
            goto Stopped;
        }
    }

    state Unhealthy {
        entry {
            send metric, eReport, (who = this, healthy = false);
            announce eCacheHealth, (who = this, healthy = false);
        }

        on eSetHealth do (healthy: bool) {
            if (healthy) {
                goto Healthy;
            }
        }

        on eStop do {
            goto Stopped;
        }
    }

    state Stopped {
        entry {
            send metric, eDeregister, this;
            announce eCacheDown, this;
        }

        ignore eStop;
        ignore eSetHealth;
    }
}
