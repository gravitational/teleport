// Copyright 2022 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package breaker implements a circuit breaker.
//
// Circuit breaker watches the error from executed functions and according to the configured TripFn will
// allow requests for a period of time before slowly allowing a few.
//
// Circuit breakers start in StateStandby first, observing errors and watching Metrics.
//
// Once the Circuit breaker TripFn returns true, it enters the StateTripped, where it blocks all traffic and returns
// ErrStateTripped for the configured Config.TrippedPeriod.
//
// After the Config.TrippedPeriod passes, Circuit breaker enters StateRecovering, during that state it will
// either transition to StateStandby after the Config.RecoveryLimit is met or StateTripped if the Config.Recover TripFn
// is satisfied.
//
// It is possible to define actions on transitions between states:
//
// * Config.OnTripped is called on transition (StateStandby -> StateTripped)
// * Config.OnStandBy is called on transition (StateRecovering -> StateStandby)
package breaker
