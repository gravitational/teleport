/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package seek

import (
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"
)

// seekState represents the state of a seeker.
type seekState uint

const (
	// stateSeeking indicates that we want to connect to this proxy
	stateSeeking seekState = iota
	// stateClaimed indicates that an agent has successfully claimed
	// responsibility for this proxy
	stateClaimed
	// stateBackoff indicates that this proxy was claimed but that the
	// agent responsible for it lost the connection prematurely
	stateBackoff
)

func (s seekState) String() string {
	switch s {
	case stateSeeking:
		return "seeking"
	case stateClaimed:
		return "claimed"
	case stateBackoff:
		return "backoff"
	default:
		return fmt.Sprintf("unknown(%d)", s)
	}
}

// seekEvent represents an asynchronous event which may change
// the state of a seeker.
type seekEvent uint

const (
	// eventRefresh indicates that a proxy has been indirectly observed (e.g. via gossip).
	eventRefresh seekEvent = iota
	// eventAcquire indicates that an agent has connected to this proxy and would like to
	// take responsibility for it.
	eventAcquire
	// eventRelease indicates that the agent responsible for this proxy has lost its
	// connection to it.
	eventRelease
)

func (s seekEvent) String() string {
	switch s {
	case eventRefresh:
		return "refresh"
	case eventAcquire:
		return "acquire"
	case eventRelease:
		return "release"
	default:
		return fmt.Sprintf("unknown(%d)", s)
	}
}

// seeker manages the state associated with a proxy.
type seeker struct {
	state   seekState
	at      time.Time
	backOff uint64
}

// transit attempts a state transition.  If transit returns true, then
// a state-transition did occur.
func (s *seeker) transit(t time.Time, e seekEvent, c *Config) (ok bool) {
	switch s.state {
	case stateSeeking:
		// stateSeeking can either transition to stateClaimed, or
		// be "refreshed" in order to prevent expiration.
		switch e {
		case eventRefresh:
			if t.After(s.at) {
				s.at = t
				return true
			} else {
				return false
			}
		case eventAcquire:
			s.state = stateClaimed
			s.at = t
			return true
		case eventRelease:
			return false
		default:
			log.Errorf("Invalid event: %q", e)
			return false
		}
	case stateClaimed:
		// stateClaimed can either transition into stateSeeking or
		// stateBackoff depending on whether the claim failed
		// immediately, or after some period of normal operation.
		switch e {
		case eventRefresh, eventAcquire:
			return false
		case eventRelease:
			// If the release event comes within the backoff threshold
			// then we are potentially dealing with an unhealthy proxy.
			// The backoff state serves as both backpressure and an
			// an escape hatch, preventing infinite retry loops.
			if s.shouldBackoff(t, c.BackoffThreshold) {
				s.state = stateBackoff
				s.backOff++
			} else {
				s.state = stateSeeking
				s.backOff = 0
			}
			s.at = t
			return true
		default:
			log.Errorf("Invalid event: %q", e)
			return false
		}
	case stateBackoff:
		// stateBackoff effectively "becomes" stateSeeking
		// once the backoff period has been observed, so we
		// either reject all transitions if still within the
		// backoff period, or accept both stateSeeking and
		// stateClaimed.
		switch e {
		case eventRefresh:
			if !s.backoffPassed(t, c.BackoffInterval) {
				return false
			}
			s.state = stateSeeking
			s.at = t
			return true
		case eventAcquire:
			if !s.backoffPassed(t, c.BackoffInterval) {
				return false
			}
			s.state = stateClaimed
			s.at = t
			return true
		case eventRelease:
			return false
		default:
			log.Errorf("Invalid event: %q", e)
			return false
		}
	default:
		log.Errorf("Invalid state: %q", s.state)
		return false
	}
}

func (s *seeker) backoffPassed(t time.Time, interval time.Duration) bool {
	end := s.at.Add(interval * time.Duration(s.backOff))
	return t.After(end)
}

func (s *seeker) shouldBackoff(t time.Time, threshold time.Duration) bool {
	cutoff := s.at.Add(threshold)
	return cutoff.After(t)
}

// expiry calculates the time at which this entry will expire,
// if it should be expired at all.
func (s *seeker) expiry(ttl time.Duration) (exp time.Time, ok bool) {
	switch s.state {
	case stateSeeking, stateBackoff:
		// calculate normal expiry
		exp = s.at.Add(ttl)
		ok = true
	case stateClaimed:
		// wait for release
		ok = false
	default:
		// invalid, expire entry immediately
		ok = true
	}
	return
}

// tick calculates the current state, possibly affecting
// a time-based transition.
func (s *seeker) tick(t time.Time, c *Config) seekState {
	if s.state == stateBackoff && s.backoffPassed(t, c.BackoffInterval) {
		s.state = stateSeeking
	}
	return s.state
}

// Status is a summary of the status of a collection
// of proxy seek states.
type Status struct {
	Seeking int
	Claimed int
	Backoff int
}

// ShouldSeek checks if we should be seeking connections.
func (s *Status) ShouldSeek() bool {
	// if we are seeking specific proxies, or we don't currently
	// have any proxies, then we should seek proxies.
	if s.Seeking > 0 || s.Claimed < 1 {
		return true
	}
	return false
}

// TargetCount is the minumum number of agents that should be active.
func (s *Status) TargetCount() int {
	total := s.Seeking + s.Claimed
	if total < 1 {
		return 1
	}
	return total
}

// Sum returns the sum of all known proxies.
func (s *Status) Sum() int {
	if s == nil {
		return 0
	}
	return s.Seeking + s.Claimed + s.Backoff
}
