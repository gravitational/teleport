spec ConnectionTerminates observes eAttemptStarted, eConnectionEstablished, eConnectionFailed {
  var pending: int;

  start state Idle {
    on eAttemptStarted do {
      pending = pending + 1;
      goto Pending;
    }
  }

  hot state Pending {
    on eAttemptStarted do {
      pending = pending + 1;
    }

    on eConnectionEstablished do Resolve;
    on eConnectionFailed do Resolve;
  }

  fun Resolve() {
    pending = pending - 1;
    if (pending == 0) {
      goto Idle;
    }
  }
}

type tDial = (replica: ApplicationService, health: ReplicaHealth);
type tAttemptTrace = (resolved: bool, snapshot: set[ApplicationService], dials: seq[tDial]);

// Records each attempt's snapshot and dials, observing each dial through the replica's reply, and judges them once the
// attempt resolves. Each dial must be the lowest-ranked replica of the snapshot not yet dialed, and only a tunnel
// failure moves on. A connection must follow a successful dial to the same replica. A non-retryable failure must follow
// a non-retryable dial, and any other failure must follow a tunnel failure from every replica in the snapshot.
spec AttemptIsCorrect observes eSessionRanking, eAttemptStarted, eReplicaSet, eDialResult, eConnectionEstablished,
    eConnectionFailed {
  var ranks: map[ApplicationService, int];
  var traces: map[int, tAttemptTrace];

  start state Checking {
    on eSessionRanking do (ranking: map[ApplicationService, int]) {
      ranks = ranking;
    }

    on eAttemptStarted do (attempt: int) {
      traces[attempt] = (resolved = false, snapshot = default(set[ApplicationService]), dials = default(seq[tDial]));
    }

    on eReplicaSet do (snapshot: (attempt: int, replicas: set[ApplicationService])) {
      AssertTracked(snapshot.attempt);
      assert !traces[snapshot.attempt].resolved,
        format("attempt {0} resolved replicas more than once", snapshot.attempt);
      traces[snapshot.attempt].resolved = true;
      traces[snapshot.attempt].snapshot = snapshot.replicas;
    }

    on eDialResult do (result: (attempt: int, replica: ApplicationService, health: ReplicaHealth)) {
      AssertTracked(result.attempt);
      assert traces[result.attempt].resolved, format("attempt {0} dialed before resolving replicas", result.attempt);
      traces[result.attempt].dials += (sizeof(traces[result.attempt].dials),
        (replica = result.replica, health = result.health));
    }

    on eConnectionEstablished do (result: (attempt: int, replica: ApplicationService)) {
      CheckConnectionFollowsDial(result.attempt, EndTrace(result.attempt), result.replica);
    }

    on eConnectionFailed do (result: (attempt: int, reason: FailureReason)) {
      CheckFailureJustified(result.attempt, EndTrace(result.attempt), result.reason);
    }
  }

  fun AssertTracked(attempt: int) {
    assert attempt in traces, format("attempt {0} acted before it started or after it resolved", attempt);
  }

  fun EndTrace(attempt: int): tAttemptTrace {
    var trace: tAttemptTrace;

    AssertTracked(attempt);
    trace = traces[attempt];
    traces -= (attempt);
    CheckDials(attempt, trace);
    return trace;
  }

  fun CheckDials(attempt: int, trace: tAttemptTrace) {
    var i: int;
    var dial: tDial;
    var candidate: ApplicationService;
    var untried: set[ApplicationService];

    untried = trace.snapshot;
    while (i < sizeof(trace.dials)) {
      dial = trace.dials[i];
      assert dial.replica in untried,
        format("attempt {0} dialed {1}, which is outside its snapshot or already dialed", attempt, dial.replica);
      foreach (candidate in untried) {
        assert ranks[dial.replica] <= ranks[candidate],
          format("attempt {0} dialed {1} before the lower-ranked {2}", attempt, dial.replica, candidate);
      }
      assert i == sizeof(trace.dials) - 1 || dial.health == TunnelDown,
        format("attempt {0} kept dialing after {1} answered {2}", attempt, dial.replica, dial.health);
      untried -= (dial.replica);
      i = i + 1;
    }
  }

  fun CheckConnectionFollowsDial(attempt: int, trace: tAttemptTrace, replica: ApplicationService) {
    var last: tDial;

    assert sizeof(trace.dials) > 0, format("attempt {0} established without dialing", attempt);
    last = trace.dials[sizeof(trace.dials) - 1];
    assert last.health == Healthy, format("attempt {0} established after {1} answered {2}", attempt, last.replica,
      last.health);
    assert last.replica == replica,
      format("attempt {0} established with {1} but its successful dial was {2}", attempt, replica, last.replica);
  }

  fun CheckFailureJustified(attempt: int, trace: tAttemptTrace, reason: FailureReason) {
    var dialCount: int;
    var lastHealth: ReplicaHealth;

    dialCount = sizeof(trace.dials);
    if (dialCount > 0) {
      lastHealth = trace.dials[dialCount - 1].health;
    }

    if (reason == NonRetryableError) {
      assert dialCount > 0 && lastHealth == NonRetryable,
        format("attempt {0} reported a non-retryable error it never saw", attempt);
      return;
    }
    assert dialCount == 0 || lastHealth == TunnelDown,
      format("attempt {0} hid a {1} dial behind reason {2}", attempt, lastHealth, reason);
    assert dialCount == sizeof(trace.snapshot),
      format("attempt {0} gave up after dialing {1} of the {2} replicas in its snapshot", attempt, dialCount,
        sizeof(trace.snapshot));
    assert (reason == NoCandidates) == (sizeof(trace.snapshot) == 0),
      format("attempt {0} reported {1} for a snapshot of size {2}", attempt, reason, sizeof(trace.snapshot));
  }
}
