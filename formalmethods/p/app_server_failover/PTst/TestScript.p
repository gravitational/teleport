test tcSameSessionFailoverAndRecovery [main=SameSessionFailoverScenario]:
  assert ConnectionTerminates, AttemptIsCorrect in
  (union AppServerFailover, { SameSessionFailoverScenario, ScenarioDriver });

test tcConcurrentSameSessionFailover [main=ConcurrentSameSessionFailoverScenario]:
  assert ConnectionTerminates, AttemptIsCorrect in
  (union AppServerFailover, { ConcurrentSameSessionFailoverScenario, ScenarioDriver });

test tcRemovedReplicaExcludedFromFreshSnapshot [main=RemovedReplicaScenario]:
  assert ConnectionTerminates, AttemptIsCorrect in
  (union AppServerFailover, { RemovedReplicaScenario, ScenarioDriver });

test tcRecoveredReplicaAcceptsNewConnection [main=RecoveredReplicaScenario]:
  assert ConnectionTerminates, AttemptIsCorrect in
  (union AppServerFailover, { RecoveredReplicaScenario, ScenarioDriver });

test tcNonRetryableDialDoesNotFailOver [main=NonRetryableDialScenario]:
  assert ConnectionTerminates, AttemptIsCorrect in
  (union AppServerFailover, { NonRetryableDialScenario, ScenarioDriver });

test tcEmptySnapshotFailsWithoutDial [main=EmptySnapshotScenario]:
  assert ConnectionTerminates, AttemptIsCorrect in
  (union AppServerFailover, { EmptySnapshotScenario, ScenarioDriver });

test tcConcurrentFallbackAgrees [main=ConcurrentFallbackScenario]:
  assert ConnectionTerminates, AttemptIsCorrect in
  (union AppServerFailover, { ConcurrentFallbackScenario, ScenarioDriver });

test tcNondeterministicTopologyTerminatesSafely [main=NondeterministicEnvironmentTestDriver]:
  assert ConnectionTerminates, AttemptIsCorrect in
  (union AppServerFailover, { NondeterministicEnvironmentTestDriver });
