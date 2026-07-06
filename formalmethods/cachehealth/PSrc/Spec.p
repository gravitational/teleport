event eCacheDown: machine;
event eCacheHealth: (who: machine, healthy: bool);
event eMetricChanged: bool;

spec MetricEventuallyConverges observes eCacheDown, eCacheHealth, eMetricChanged {
    // every cache currently up (any health)
    var up: set[machine];
    // the up caches reporting healthy
    var healthyUp: set[machine];
    var metric: bool;

    start cold state Init {
        entry {
            goto Converged;
        }
    }

    cold state Converged {
        on eCacheHealth do (r: (who: machine, healthy: bool)) {
            up += (r.who);
            if (r.healthy) {
                healthyUp += (r.who);
            } else {
                healthyUp -= (r.who);
            }
            check();
        }

        on eCacheDown do (who: machine) {
            up -= (who);
            healthyUp -= (who);
            check();
        }

        on eMetricChanged do (h: bool) {
            metric = h;
            check();
        }
    }

    hot state Diverged {
        on eCacheHealth do (r: (who: machine, healthy: bool)) {
            up += (r.who);
            if (r.healthy) {
                healthyUp += (r.who);
            } else {
                healthyUp -= (r.who);
            }
            check();
        }

        on eCacheDown do (who: machine) {
            up -= (who);
            healthyUp -= (who);
            check();
        }

        on eMetricChanged do (h: bool) {
            metric = h;
            check();
        }
    }

    fun check() {
        if (metric == expected()) {
            goto Converged;
        } else {
            goto Diverged;
        }
    }

    fun expected(): bool {
        return sizeof(up) == 0 || sizeof(healthyUp) > 0;
    }
}
