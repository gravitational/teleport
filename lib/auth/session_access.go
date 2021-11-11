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
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"
	"github.com/vulcand/predicate"
)

const (
	SSHSessionKind        SessionKind            = "ssh"
	KubernetesSessionKind SessionKind            = "kubernetes"
	SessionObserverMode   SessionParticipantMode = "observer"
	SessionModeratorMode  SessionParticipantMode = "moderator"
)

type SessionKind string
type SessionParticipantMode string

type SessionAccessEvaluator struct {
	kind     SessionKind
	requires []*types.SessionRequirePolicy
}

func NewSessionAccessEvaluator(initiator []types.Role, kind SessionKind) SessionAccessEvaluator {
	requires := getRequirePolicies(initiator)

	return SessionAccessEvaluator{
		kind,
		requires,
	}
}

func getRequirePolicies(participant []types.Role) []*types.SessionRequirePolicy {
	var policies []*types.SessionRequirePolicy

	for _, role := range participant {
		policiesFromRole := role.GetSessionRequirePolicies(types.Allow)
		if len(policiesFromRole) == 0 {
			return nil
		}

		policies = append(policies, policiesFromRole...)
	}

	return policies
}

func getAllowPolicies(participant []types.Role) []*types.SessionJoinPolicy {
	var policies []*types.SessionJoinPolicy

	for _, role := range participant {
		policies = append(policies, role.GetSessionJoinPolicies(types.Allow)...)
	}

	return policies
}

func contains(s []string, e SessionKind) bool {
	for _, a := range s {
		if SessionKind(a) == e {
			return true
		}
	}

	return false
}

// TODO(joel): set up parser context
func (e *SessionAccessEvaluator) matchesPolicy(require *types.SessionRequirePolicy, allow *types.SessionJoinPolicy) (bool, error) {
	if !contains(require.Kinds, e.kind) || !contains(allow.Kinds, e.kind) {
		return false, nil
	}

	parser, err := services.NewWhereParser(nil)
	if err != nil {
		return false, trace.Wrap(err)
	}

	ifn, err := parser.Parse(require.Filter)
	if err != nil {
		return false, trace.Wrap(err)
	}

	fn, ok := ifn.(predicate.BoolPredicate)
	if !ok {
		return false, trace.BadParameter("unsupported type: %T", ifn)
	}

	return fn(), nil
}

func (e *SessionAccessEvaluator) FulfilledFor(participants [][]types.Role) (bool, error) {
	if len(e.requires) == 0 {
		return true, nil
	}

	for _, requirePolicy := range e.requires {
		left := requirePolicy.Count

		for _, participant := range participants {
			allowPolicies := getAllowPolicies(participant)
			for _, allowPolicy := range allowPolicies {
				matches, err := e.matchesPolicy(requirePolicy, allowPolicy)
				if err != nil {
					return false, trace.Wrap(err)
				}

				if matches {
					left--
					break
				}
			}

			if left <= 0 {
				return true, nil
			}
		}
	}

	return false, nil
}
