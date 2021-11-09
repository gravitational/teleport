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

package utils

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

type SessionAccessRequirePolicy struct {
	name   string
	filter string
	kinds  []SessionKind
	count  int
}

type SessionAccessAllowPolicy struct {
	name            string
	initiator_roles []string
	kinds           []SessionKind
	modes           []SessionParticipantMode
}

type SessionAccessEvaluator struct {
	requires []SessionAccessRequirePolicy
}

func getPoliciesFor(participant *types.Participant) []SessionAccessAllowPolicy {
	return []SessionAccessAllowPolicy{}
}

func matchesPolicy(require *SessionAccessRequirePolicy, allow SessionAccessAllowPolicy) bool {
	return true
}

func (e *SessionAccessEvaluator) FulfilledFor(participants []*types.Participant) bool {
	for _, requirePolicy := range e.requires {
		left := requirePolicy.count

		for _, participant := range participants {
			allowPolicies := getPoliciesFor(participant)
			for _, allowPolicy := range allowPolicies {
				if matchesPolicy(&requirePolicy, allowPolicy) {
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
