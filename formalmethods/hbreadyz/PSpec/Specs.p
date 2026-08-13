/* LIVENESS: An idle agent is an agent with no resources to serve.
 * The resolved process state must eventually become StateOK. */
spec EventuallyReadyWhenIdle observes eAnnResources, eAnnState {
    var lastState: tComponentState;
    var lastTotal: int;

    start cold state HasNoResourcesAndHealthy {
        entry {
            lastState = StateOK;
            lastTotal = 0;
        }

        on eAnnState do (resolved: tComponentState) {
            lastState = resolved;
            if (lastState != StateOK) {
                goto NoResourcesMustBecomeOK;
            }
        }

        on eAnnResources do (r: (total: int, failing: int)) {
            lastTotal = r.total;
            if (lastTotal > 0) {
                goto HasResources;
            }
        }
    }

    cold state HasResources {
        on eAnnState do (resolved: tComponentState) {
            lastState = resolved;
            if (lastTotal == 0 && lastState == StateOK) {
                goto HasNoResourcesAndHealthy;
            } else if (lastTotal == 0) {
                goto NoResourcesMustBecomeOK;
            }
        }

        on eAnnResources do (r: (total: int, failing: int)) {
            lastTotal = r.total;
            if (lastTotal == 0 && lastState == StateOK) {
                goto HasNoResourcesAndHealthy;
            } else if (lastTotal == 0) {
                goto NoResourcesMustBecomeOK;
            }
        }
    }

    /* HOT: nothing left to serve, but resolved state is not StateOK. */
    hot state NoResourcesMustBecomeOK {
        on eAnnState do (resolved: tComponentState) {
            lastState = resolved;
            if (resolved == StateOK) {
                goto HasNoResourcesAndHealthy;
            }
        }
        ignore eAnnResources;
    }
}


/* SAFETY: the process must not resolve to StateOK when the latest heartbeat
 * report observed that some resource was failing. */
spec NoFalseReady observes eAnnReport, eAnnState {
    var lastReportSawFailure: bool;

    start state Watching {
        entry { lastReportSawFailure = false; }

        on eAnnReport do (r: (ok: bool, anyFailing: bool)) {
            lastReportSawFailure = r.anyFailing;
        }

        on eAnnState do (resolved: tComponentState) {
            if (resolved == StateOK) {
                assert !lastReportSawFailure,
                    "resolved StateOK while the most recent heartbeat report saw a failing resource";
            }
        }
    }
}
