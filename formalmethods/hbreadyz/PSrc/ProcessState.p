machine ProcessState {
    /*
     * age is the modeled now - recoveryTime.
     */
    var age: int;

    start state stateStarting {
        entry {
            /* Empty processState.states also resolves to stateStarting. The
             * agent startup event registers the modeled component and leaves
             * the resolved state unchanged. */
            age = 0;
        }

        on eComponentStarting do (req: (comp: tComponent, from: machine)) {
            Reply(req.from, StateStarting);
        }

        on eHeartbeatReport do (req: (comp: tComponent, ok: bool, from: machine)) {
            if (req.ok) {
                Reply(req.from, StateOK);
                goto stateOK;
            } else {
                Reply(req.from, StateDegraded);
                goto stateDegraded;
            }
        }

        on eTick do (from: machine) {
            AdvanceClock();
            Reply(from, StateStarting);
        }
    }

    state stateOK {
        on eComponentStarting do (req: (comp: tComponent, from: machine)) {
            Reply(req.from, StateOK);
        }

        on eHeartbeatReport do (req: (comp: tComponent, ok: bool, from: machine)) {
            if (req.ok) {
                Reply(req.from, StateOK);
            } else {
                Reply(req.from, StateDegraded);
                goto stateDegraded;
            }
        }

        on eTick do (from: machine) {
            AdvanceClock();
            Reply(from, StateOK);
        }
    }

    state stateDegraded {
        on eComponentStarting do (req: (comp: tComponent, from: machine)) {
            Reply(req.from, StateDegraded);
        }

        on eHeartbeatReport do (req: (comp: tComponent, ok: bool, from: machine)) {
            if (req.ok) {
                /* stateDegraded --OK--> stateRecovering, recoveryTime = now */
                age = 0;
                Reply(req.from, StateRecovering);
                goto stateRecovering;
            } else {
                /* Degraded events always keep/force stateDegraded and do not
                 * touch recoveryTime. */
                Reply(req.from, StateDegraded);
            }
        }

        on eTick do (from: machine) {
            AdvanceClock();
            Reply(from, StateDegraded);
        }
    }

    state stateRecovering {
        on eComponentStarting do (req: (comp: tComponent, from: machine)) {
            Reply(req.from, StateRecovering);
        }

        on eHeartbeatReport do (req: (comp: tComponent, ok: bool, from: machine)) {
            if (!req.ok) {
                Reply(req.from, StateDegraded);
                goto stateDegraded;
            } else if (age > RecoveryThreshold()) {
                Reply(req.from, StateOK);
                goto stateOK;
            } else {
                /*
                 * The OK report arrived too early and is silently dropped.
                 * The component stays in stateRecovering.
                 */
                Reply(req.from, StateRecovering);
            }
        }

        on eTick do (from: machine) {
            AdvanceClock();
            Reply(from, StateRecovering);
        }
    }

    fun AdvanceClock() {
        if (age <= RecoveryThreshold()) {
            age = age + 1;
        }
    }

    fun Reply(target: machine, resolved: tComponentState) {
        announce eAnnState, resolved;
        send target, eStateAck, resolved;
    }
}
