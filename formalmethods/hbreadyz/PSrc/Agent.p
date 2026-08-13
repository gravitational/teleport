enum tAgentAction {
    ActionAddResource,
    ActionDeleteHealthyResource,
    ActionDeleteFailingResource,
    ActionResourceStartsFailing,
    ActionResourceRecovers
}

machine Agent {
    var ps: machine;     /* process state */
    var total: int;      /* resources this agent currently matches */
    var failing: int;    /* how many of those cannot heartbeat successfully */

    start state Init {
        entry (config: tAgentConfig) {
            ps = config.processState;
            total = 0;
            failing = 0;
            announce eAnnResources, (total = total, failing = failing);
            SendStarting();
            send config.from, eAgentStepAck;
            goto Run;
        }
    }

    state Run {
        on eAgentStep do (from: machine) {
            /*
             * P does not have first-class function values, so choose a named
             * agent action and dispatch to the corresponding action function.
             * Disabled choices become no-ops. Clock ticks are outside the agent
             * now, so time is not hidden behind disabled resource branches.
             */
            RunAction(ChooseAction());
            send from, eAgentStepAck;
        }
    }

    fun ChooseAction(): tAgentAction {
        var pick: int;
        pick = choose(5);

        if (pick == 0) {
            return ActionAddResource;
        } else if (pick == 1) {
            return ActionDeleteHealthyResource;
        } else if (pick == 2) {
            return ActionDeleteFailingResource;
        } else if (pick == 3) {
            return ActionResourceStartsFailing;
        } else {
            return ActionResourceRecovers;
        }
    }

    fun RunAction(action: tAgentAction) {
        if (action == ActionAddResource) {
            AddResource();
        } else if (action == ActionDeleteHealthyResource) {
            DeleteHealthyResource();
        } else if (action == ActionDeleteFailingResource) {
            DeleteFailingResource();
        } else if (action == ActionResourceStartsFailing) {
            ResourceStartsFailing();
        } else if (action == ActionResourceRecovers) {
            ResourceRecovers();
        }

        if (failing > 0) {
            announce eAnnReport, (ok = false, anyFailing = true);
            Report(false);
        } else if (total > 0 ){ // The check here is the root cause, no heartbeats for 0 resources
            announce eAnnReport, (ok = true, anyFailing = failing > 0);
            Report(true);
        }
    }

    fun AddResource() {
        total = total + 1;
        announce eAnnResources, (total = total, failing = failing);
    }

    fun DeleteHealthyResource() {
        if (total - failing > 0) {
            /* A healthy resource was deleted. Its heartbeat goroutine stops;
             * nothing is broadcast on the way out. */
            total = total - 1;
            announce eAnnResources, (total = total, failing = failing);
        }
    }

    fun DeleteFailingResource() {
        if (failing > 0) {
            /* A failing resource was deleted. */
            total = total - 1;
            failing = failing - 1;
            announce eAnnResources, (total = total, failing = failing);
        }
    }

    fun ResourceStartsFailing() {
        if (total - failing > 0) {
            /* A resource's backend went bad. Not reported yet. */
            failing = failing + 1;
            announce eAnnResources, (total = total, failing = failing);
        }
    }

    fun ResourceRecovers() {
        if (failing > 0) {
            /* A resource's backend came good. Not reported yet. */
            failing = failing - 1;
            announce eAnnResources, (total = total, failing = failing);
        }
    }

    /*------------------------------------------------------------------------
     * The supervisor lock serialises every processState mutation, so the agent
     * waits for each update to land before choosing its next action. This
     * keeps the state space small without losing behaviour the real code can
     * exhibit.
     *-----------------------------------------------------------------------*/
    fun SendStarting() {
        send ps, eComponentStarting, (comp = CompAppServer, from = this);
        AwaitAck();
    }

    fun Report(ok: bool) {
        send ps, eHeartbeatReport, (comp = CompAppServer, ok = ok, from = this);
        AwaitAck();
    }

    fun AwaitAck() {
        receive {
            case eStateAck: (resolved: tComponentState) { }
        }
    }
}
