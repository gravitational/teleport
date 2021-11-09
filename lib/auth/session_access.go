/*
Copyright 2021 Gravitational, Inc.

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

package auth

import (
	"github.com/gravitational/teleport/api/types"
)

const (
	SSHSessionKind        = "ssh"
	KubernetesSessionKind = "kubernetes"
	SessionObserverMode   = "observer"
	SessionModeratorMode  = "moderator"
)

type SessionKind string
type SessionParticipantMode string

type User struct {
	user  types.User
	roles []types.Role
}

type SessionAccessEvaluator struct {
	kind     SessionKind
	requires []*types.SessionRequirePolicy
}

func NewSessionAccessEvaluator(initiator User, kind SessionKind) SessionAccessEvaluator {
	requires := getRequirePolicies(initiator)

	return SessionAccessEvaluator{
		kind,
		requires,
	}
}

// TODO(joel): implement this
func getRequirePolicies(participant User) []*types.SessionRequirePolicy {
	var policies []*types.SessionRequirePolicy
	return policies
}

// TODO(joel): implement this
func getAllowPolicies(participant User) []*types.SessionJoinPolicy {
	var policies []*types.SessionJoinPolicy
	return policies
}

// TODO(joel): implement this
func matchesPolicy(require *types.SessionRequirePolicy, allow *types.SessionJoinPolicy) bool {
	return true
}

func (e *SessionAccessEvaluator) FulfilledFor(participants []User) bool {
	for _, requirePolicy := range e.requires {
		left := requirePolicy.Count

		for _, participant := range participants {
			allowPolicies := getAllowPolicies(participant)
			for _, allowPolicy := range allowPolicies {
				if matchesPolicy(requirePolicy, allowPolicy) {
					left--
					break
				}
			}
		}

		if left <= 0 {
			return true
		}
	}

	return false
}
