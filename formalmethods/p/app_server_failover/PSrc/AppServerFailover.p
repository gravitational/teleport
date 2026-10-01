// TunnelDown is any dial error isReverseTunnelDownError in lib/web/app/transport.go accepts, so the attempt moves on to
// the next candidate. NonRetryable is any other error, which ends the attempt.
enum ReplicaHealth {
  Healthy,
  TunnelDown,
  NonRetryable
}

enum FailureReason {
  NoCandidates,
  CandidatesExhausted,
  NonRetryableError
}

event eConnectionRequested: machine;
// Announced to monitors only, so specs can track attempts and check dial order against the session ranking.
event eAttemptStarted: int;
event eSessionRanking: map[ApplicationService, int];
event eConnectionEstablished: (attempt: int, replica: ApplicationService);
event eConnectionFailed: (attempt: int, reason: FailureReason);

event eGetReplicas: (attempt: int, requester: machine);
event eReplicaSet: (attempt: int, replicas: set[ApplicationService]);
event eSetReplicas: (replicas: set[ApplicationService], requester: machine);
event eReplicaSetUpdated;

event eDial: (attempt: int, requester: machine);
event eDialResult: (attempt: int, replica: ApplicationService, health: ReplicaHealth);
event eSetHealth: (health: ReplicaHealth, requester: machine);
event eHealthSet;

// Every dial gets exactly one reply. In production a tunnel that stops answering but stays open hangs the dial, since
// tunnel dials have no deadline. That hang is out of scope.
machine ApplicationService {
  var health: ReplicaHealth;

  start state Running {
    entry (initialHealth: ReplicaHealth) {
      health = initialHealth;
    }

    on eDial do (request: (attempt: int, requester: machine)) {
      send request.requester, eDialResult, (attempt = request.attempt, replica = this, health = health);
    }

    on eSetHealth do (request: (health: ReplicaHealth, requester: machine)) {
      health = request.health;
      send request.requester, eHealthSet;
    }
  }
}

// A snapshot is the set of app service hosts serving the route. Left out are routes that match only by public address,
// which list a host once per app, and failed lookups, since the Watcher always answers.
machine Watcher {
  var replicas: set[ApplicationService];

  start state Running {
    entry (initialReplicas: set[ApplicationService]) {
      replicas = initialReplicas;
    }

    on eGetReplicas do (request: (attempt: int, requester: machine)) {
      send request.requester, eReplicaSet, (attempt = request.attempt, replicas = replicas);
    }

    on eSetReplicas do (request: (replicas: set[ApplicationService], requester: machine)) {
      replicas = request.replicas;
      send request.requester, eReplicaSetUpdated;
    }
  }
}

// Each connection runs in its own ConnectionAttempt with a fresh snapshot, so concurrent connections interleave the way
// dials sharing one cached app session do. The ranking stands in for rendezvous hashing of (session ID, host ID), and
// drivers choose it nondeterministically so the checker covers every order a session could hash to.
machine Proxy {
  var watcher: Watcher;
  var ranks: map[ApplicationService, int];
  var nextAttempt: int;

  start state Ready {
    entry (input: (watcher: Watcher, ranks: map[ApplicationService, int])) {
      watcher = input.watcher;
      ranks = input.ranks;
      announce eSessionRanking, ranks;
    }

    on eConnectionRequested do (requester: machine) {
      nextAttempt = nextAttempt + 1;
      announce eAttemptStarted, nextAttempt;
      new ConnectionAttempt((attempt = nextAttempt, requester = requester, watcher = watcher, ranks = ranks));
    }
  }
}

machine ConnectionAttempt {
  var attempt: int;
  var requester: machine;
  var ranks: map[ApplicationService, int];
  var remaining: set[ApplicationService];

  start state WaitingForReplicas {
    entry (input: (attempt: int, requester: machine, watcher: Watcher, ranks: map[ApplicationService, int])) {
      attempt = input.attempt;
      requester = input.requester;
      ranks = input.ranks;
      send input.watcher, eGetReplicas, (attempt = attempt, requester = this);
    }

    on eReplicaSet do (snapshot: (attempt: int, replicas: set[ApplicationService])) {
      remaining = snapshot.replicas;
      if (sizeof(remaining) == 0) {
        send requester, eConnectionFailed, (attempt = attempt, reason = NoCandidates);
        goto Done;
      }
      goto DialNext;
    }
  }

  state DialNext {
    entry {
      var replica: ApplicationService;

      if (sizeof(remaining) == 0) {
        send requester, eConnectionFailed, (attempt = attempt, reason = CandidatesExhausted);
        goto Done;
      }

      replica = LowestRanked(remaining);
      remaining -= (replica);
      send replica, eDial, (attempt = attempt, requester = this);
      goto WaitingForDial;
    }
  }

  state WaitingForDial {
    on eDialResult do (result: (attempt: int, replica: ApplicationService, health: ReplicaHealth)) {
      if (result.health == TunnelDown) {
        goto DialNext;
      }
      if (result.health == NonRetryable) {
        send requester, eConnectionFailed, (attempt = attempt, reason = NonRetryableError);
        goto Done;
      }
      send requester, eConnectionEstablished, (attempt = attempt, replica = result.replica);
      goto Done;
    }
  }

  // No handlers on purpose. A stray event after the attempt resolves is a modeling bug the checker should report.
  state Done {}

  fun LowestRanked(candidates: set[ApplicationService]): ApplicationService {
    var candidate: ApplicationService;
    var lowest: ApplicationService;
    var found: bool;

    foreach (candidate in candidates) {
      if (!found || ranks[candidate] < ranks[lowest]) {
        lowest = candidate;
        found = true;
      }
    }
    return lowest;
  }
}

module AppServerFailover = {
  Proxy,
  ConnectionAttempt,
  Watcher,
  ApplicationService
};
