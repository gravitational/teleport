event eInject;

machine FaultInjector {
    var cache: machine;
    var failures: int;

    start state Init {
        entry (c: machine) {
            cache = c;
            failures = 4;
            send this, eInject;
            goto Injecting;
        }
    }

    state Injecting {
        on eInject do {
            if (failures == 0) {
                goto Done;
            }
            send cache, eSetHealth, $;
            failures = failures - 1;
            send this, eInject;
        }
    }

    state Done {}
}

machine Driver {
    // m is the single metric for the cache being healthy or unhealthy.
    var m: machine;

    // a and b are the two instances of a cache that is reporting to m
    // if it is healthy or unhealthy.
    var a: machine;
    var b: machine;

    start state Init {
        entry {
            // Create a single metric and two caches that point to that metric.
            m = new Metric();
            a = new Cache(m);
            b = new Cache(m);

            new FaultInjector(a);
            new FaultInjector(b);

            send a, eStop;
            send b, eStop;
        }
    }
}
