enum Replica {
  A,
  B,
  C
}

enum Membership {
  NoReplicas,
  OnlyA,
  OnlyB,
  AAndB,
  AllReplicas
}

enum StepKind {
  PublishStep,
  SetHealthStep,
  ConnectStep
}

enum Expectation {
  ExpectAnyOutcome,
  ExpectReplica,
  ExpectSameReplica
}

// Fields a step kind does not use are ignored, so build steps with the helpers below.
type tStep = (kind: StepKind, members: Membership, replica: Replica, health: ReplicaHealth, count: int,
  expect: Expectation);

fun Publish(members: Membership): tStep {
  return (kind = PublishStep, members = members, replica = A, health = Healthy, count = 0, expect = ExpectAnyOutcome);
}

fun SetHealth(replica: Replica, health: ReplicaHealth): tStep {
  return (kind = SetHealthStep, members = NoReplicas, replica = replica, health = health, count = 0,
    expect = ExpectAnyOutcome);
}

// Every connection in the step must succeed, all on the same replica.
fun Connect(count: int): tStep {
  return (kind = ConnectStep, members = NoReplicas, replica = A, health = Healthy, count = count,
    expect = ExpectSameReplica);
}

fun ConnectTo(count: int, replica: Replica): tStep {
  return (kind = ConnectStep, members = NoReplicas, replica = replica, health = Healthy, count = count,
    expect = ExpectReplica);
}

fun ConnectAnyOutcome(count: int): tStep {
  return (kind = ConnectStep, members = NoReplicas, replica = A, health = Healthy, count = count,
    expect = ExpectAnyOutcome);
}

// Ranks are a permutation, so no two replicas tie.
fun RandomRanking(replicas: map[Replica, ApplicationService]): map[ApplicationService, int] {
  var name: Replica;
  var rank: int;
  var ranks: map[ApplicationService, int];
  var unused: set[int];

  while (rank < sizeof(replicas)) {
    unused += (rank);
    rank = rank + 1;
  }
  foreach (name in keys(replicas)) {
    rank = choose(unused);
    unused -= (rank);
    ranks[replicas[name]] = rank;
  }
  return ranks;
}

fun MembersOf(members: Membership, replicas: map[Replica, ApplicationService]): set[ApplicationService] {
  var result: set[ApplicationService];

  if (members == OnlyA || members == AAndB || members == AllReplicas) {
    result += (replicas[A]);
  }
  if (members == OnlyB || members == AAndB || members == AllReplicas) {
    result += (replicas[B]);
  }
  if (members == AllReplicas) {
    result += (replicas[C]);
  }
  return result;
}

// Runs a scripted scenario one step at a time. Topology steps wait for their acknowledgement so each one is in effect
// before the next step begins. Connection steps may overlap each other but must all resolve before the next step.
machine ScenarioDriver {
  var replicas: map[Replica, ApplicationService];
  var watcher: Watcher;
  var proxy: Proxy;
  var steps: seq[tStep];
  var current: tStep;
  var next: int;
  var outstanding: int;
  var stepReplica: ApplicationService;
  var stepConnected: bool;

  start state Init {
    entry (input: seq[tStep]) {
      steps = input;
      replicas[A] = new ApplicationService(Healthy);
      replicas[B] = new ApplicationService(Healthy);
      replicas[C] = new ApplicationService(Healthy);
      watcher = new Watcher(default(set[ApplicationService]));
      proxy = new Proxy((watcher = watcher, ranks = RandomRanking(replicas)));
      goto RunNextStep;
    }
  }

  state RunNextStep {
    entry {
      var i: int;

      if (next == sizeof(steps)) {
        goto Done;
      }
      current = steps[next];
      next = next + 1;

      if (current.kind == PublishStep) {
        send watcher, eSetReplicas, (replicas = MembersOf(current.members, replicas), requester = this);
      } else if (current.kind == SetHealthStep) {
        send replicas[current.replica], eSetHealth, (health = current.health, requester = this);
      } else {
        outstanding = current.count;
        stepConnected = false;
        while (i < current.count) {
          send proxy, eConnectionRequested, this;
          i = i + 1;
        }
      }
      goto WaitingForStep;
    }
  }

  state WaitingForStep {
    on eReplicaSetUpdated goto RunNextStep;
    on eHealthSet goto RunNextStep;

    on eConnectionEstablished do (result: (attempt: int, replica: ApplicationService)) {
      if (current.expect == ExpectReplica) {
        assert result.replica == replicas[current.replica],
          format("step {0}: attempt {1} connected to {2} instead of {3}", next, result.attempt, result.replica,
            replicas[current.replica]);
      }
      if (current.expect == ExpectSameReplica && stepConnected) {
        assert result.replica == stepReplica,
          format("step {0}: attempt {1} connected to {2} but an earlier connection in the step used {3}", next,
            result.attempt, result.replica, stepReplica);
      }
      stepReplica = result.replica;
      stepConnected = true;
      ConnectionResolved();
    }

    on eConnectionFailed do (result: (attempt: int, reason: FailureReason)) {
      assert current.expect == ExpectAnyOutcome,
        format("step {0}: attempt {1} failed with {2} where a connection was expected", next, result.attempt,
          result.reason);
      ConnectionResolved();
    }
  }

  state Done {}

  fun ConnectionResolved() {
    outstanding = outstanding - 1;
    if (outstanding == 0) {
      goto RunNextStep;
    }
  }
}

// B registers after the session first connects, as in #652, so a session that kept its first snapshot could not fail
// over to it. Once A recovers, the rank order the specs check sends the session back to whichever of A and B ranks
// first.
machine SameSessionFailoverScenario {
  start state Init {
    entry {
      var steps: seq[tStep];

      steps += (sizeof(steps), Publish(OnlyA));
      steps += (sizeof(steps), ConnectTo(1, A));
      steps += (sizeof(steps), Publish(AAndB));
      steps += (sizeof(steps), SetHealth(A, TunnelDown));
      steps += (sizeof(steps), ConnectTo(1, B));
      steps += (sizeof(steps), SetHealth(A, Healthy));
      steps += (sizeof(steps), Connect(1));
      new ScenarioDriver(steps);
    }
  }
}

// Each step's two connections overlap, and only one replica can serve each step, so both must reach it.
machine ConcurrentSameSessionFailoverScenario {
  start state Init {
    entry {
      var steps: seq[tStep];

      steps += (sizeof(steps), Publish(OnlyA));
      steps += (sizeof(steps), ConnectTo(2, A));
      steps += (sizeof(steps), Publish(AAndB));
      steps += (sizeof(steps), SetHealth(A, TunnelDown));
      steps += (sizeof(steps), ConnectTo(2, B));
      new ScenarioDriver(steps);
    }
  }
}

// The session connects before and after A is removed, so a session that kept its first snapshot would still reach A.
machine RemovedReplicaScenario {
  start state Init {
    entry {
      var steps: seq[tStep];

      steps += (sizeof(steps), Publish(AAndB));
      steps += (sizeof(steps), Connect(1));
      steps += (sizeof(steps), Publish(OnlyB));
      steps += (sizeof(steps), ConnectTo(1, B));
      new ScenarioDriver(steps);
    }
  }
}

// The session connects while A is down, so a session that drops a replica after one failure cannot reach A again.
machine RecoveredReplicaScenario {
  start state Init {
    entry {
      var steps: seq[tStep];

      steps += (sizeof(steps), Publish(OnlyA));
      steps += (sizeof(steps), ConnectTo(1, A));
      steps += (sizeof(steps), SetHealth(A, TunnelDown));
      steps += (sizeof(steps), ConnectAnyOutcome(1));
      steps += (sizeof(steps), SetHealth(A, Healthy));
      steps += (sizeof(steps), ConnectTo(1, A));
      new ScenarioDriver(steps);
    }
  }
}

// Depending on the ranking, the attempt connects to B or fails on A without trying B, and the specs judge which.
machine NonRetryableDialScenario {
  start state Init {
    entry {
      var steps: seq[tStep];

      steps += (sizeof(steps), Publish(AAndB));
      steps += (sizeof(steps), SetHealth(A, NonRetryable));
      steps += (sizeof(steps), ConnectAnyOutcome(1));
      new ScenarioDriver(steps);
    }
  }
}

machine EmptySnapshotScenario {
  start state Init {
    entry {
      var steps: seq[tStep];

      steps += (sizeof(steps), Publish(NoReplicas));
      steps += (sizeof(steps), ConnectAnyOutcome(1));
      new ScenarioDriver(steps);
    }
  }
}

// After A fails, overlapping connections see the same candidates and failures, so they must reach the same fallback.
machine ConcurrentFallbackScenario {
  start state Init {
    entry {
      var steps: seq[tStep];

      steps += (sizeof(steps), Publish(OnlyA));
      steps += (sizeof(steps), ConnectTo(2, A));
      steps += (sizeof(steps), Publish(AllReplicas));
      steps += (sizeof(steps), SetHealth(A, TunnelDown));
      steps += (sizeof(steps), Connect(2));
      steps += (sizeof(steps), Connect(2));
      new ScenarioDriver(steps);
    }
  }
}

machine NondeterministicEnvironmentTestDriver {
  var replicas: map[Replica, ApplicationService];
  var watcher: Watcher;

  start state Init {
    entry {
      var healths: set[ReplicaHealth];
      var memberships: set[Membership];
      var proxy: Proxy;
      var started: int;
      var healthTargets: set[Replica];
      var topologySteps: int;

      replicas[A] = new ApplicationService(Healthy);
      replicas[B] = new ApplicationService(Healthy);
      watcher = new Watcher(MembersOf(OnlyA, replicas));
      proxy = new Proxy((watcher = watcher, ranks = RandomRanking(replicas)));

      memberships += (NoReplicas);
      memberships += (OnlyA);
      memberships += (OnlyB);
      memberships += (AAndB);
      healths += (Healthy);
      healths += (TunnelDown);
      healths += (NonRetryable);
      healthTargets += (A);
      healthTargets += (B);

      // Connection starts interleave nondeterministically with topology changes and nothing waits for
      // acknowledgements, so changes land before, between or during the dials of overlapping attempts.
      while (started < 4 || topologySteps < 3) {
        if (started < 4 && (topologySteps >= 3 || $)) {
          send proxy, eConnectionRequested, this;
          started = started + 1;
        } else if ($) {
          send watcher, eSetReplicas, (replicas = MembersOf(choose(memberships), replicas), requester = this);
          topologySteps = topologySteps + 1;
        } else {
          send replicas[choose(healthTargets)], eSetHealth, (health = choose(healths), requester = this);
          topologySteps = topologySteps + 1;
        }
      }
      goto Done;
    }
  }

  state Done {
    ignore eReplicaSetUpdated, eHealthSet, eConnectionEstablished, eConnectionFailed;
  }
}
