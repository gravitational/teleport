/*
 * componentStateEnum, lib/service/state.go.
 */
enum tComponentState {
    StateOK,
    StateRecovering,
    StateDegraded,
    StateStarting
}


enum tComponent {
    CompAppServer
}

/*
 * 2 * defaults.HeartbeatCheckPeriod, expressed in heartbeat periods.
 */
fun RecoveryThreshold(): int { return 2; }


event eHeartbeatReport: (comp: tComponent, ok: bool, from: machine);
event eComponentStarting: (comp: tComponent, from: machine);
event eTick: machine;
event eStateAck: tComponentState;
event eAgentStep: machine;
event eAgentStepAck;


type tAgentConfig = (
    processState: machine,
    from: machine
);


/* The agent's resource set changed, or was just observed. `failing` counts
 * resources whose heartbeats are currently unable to succeed. */
event eAnnResources: (total: int, failing: int);

/* A heartbeat reported. `anyFailing` is whether ANY of the agent's resources
 * was failing at the moment of the report. */
event eAnnReport: (ok: bool, anyFailing: bool);

/* The resolved process state after every processState update. */
event eAnnState: tComponentState;
