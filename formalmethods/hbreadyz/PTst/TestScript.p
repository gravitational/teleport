machine AgentProcessStateDriver {
    var ps: machine;
    var agent: machine;

    start state Init {
        entry {
            ps = new ProcessState();
            agent = new Agent((processState = ps, from = this));
            AwaitAgent();
            goto Run;
        }
    }

    state Run {
        entry {
            if (choose()) {
                send agent, eAgentStep, this;
                AwaitAgent();
            } else {
                Tick();

                /*
                 * The checker explores arbitrary finite prefixes by letting the
                 * harness stop nondeterministically after time has advanced.
                 */
                if (choose()) {
                    goto Done;
                }
            }
            goto Run;
        }
    }

    state Done { }

    fun Tick() {
        // announce eAnnTick;
        send ps, eTick, this;
        AwaitState();
    }

    fun AwaitAgent() {
        receive {
            case eAgentStepAck: { }
        }
    }

    fun AwaitState() {
        receive {
            case eStateAck: (resolved: tComponentState) { }
        }
    }
}



test tcAgentProcessStateDriver [main = AgentProcessStateDriver]:
    assert EventuallyReadyWhenIdle, NoFalseReady in
        (union { AgentProcessStateDriver, Agent }, { ProcessState });
