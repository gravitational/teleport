/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package auth

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/gravitational/trace"
	"github.com/vulcand/predicate"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
)

// SessionAccessEvaluator takes a set of policies
// and uses rules to evaluate them to determine when a session may start
// and if a user can join a session.
//
// The current implementation is very simple and uses a brute-force algorithm.
// More efficient implementations that run in non O(n^2)-ish time are possible but require complex code
// that is harder to debug in the case of misconfigured policies or other error and are harder to intuitively follow.
// In the real world, the number of roles and session are small enough that this doesn't have a meaningful impact.
type SessionAccessEvaluator struct {
	kind        types.SessionKind
	policySets  []*types.SessionTrackerPolicySet
	isModerated bool
	owner       string
}

type PolicyFulfillmentStatus struct {
	Name     string              `json:"string"`
	Count    int32               `json:"count"`
	Satifies []types.Participant `json:"satisfies"`
}

// NewSessionAccessEvaluator creates a new session access evaluator for a given session kind
// and a set of roles attached to the host user.
func NewSessionAccessEvaluator(policySets []*types.SessionTrackerPolicySet, kind types.SessionKind, owner string) SessionAccessEvaluator {
	e := SessionAccessEvaluator{
		kind:       kind,
		policySets: policySets,
		owner:      owner,
	}

	for _, policySet := range policySets {
		if len(e.extractApplicablePolicies(policySet)) != 0 {
			e.isModerated = true
			break
		}
	}

	return e
}

func getAllowPolicies(participant SessionAccessContext) []*types.SessionJoinPolicy {
	var policies []*types.SessionJoinPolicy

	for _, role := range participant.Roles {
		policies = append(policies, role.GetSessionJoinPolicies()...)
	}

	return policies
}

func ContainsSessionKind(s []string, e types.SessionKind) bool {
	for _, a := range s {
		if types.SessionKind(a) == e {
			return true
		}
	}

	return false
}

// SessionAccessContext is the context that must be provided per participant in the session.
type SessionAccessContext struct {
	Username string
	Roles    []types.Role
	Mode     types.SessionParticipantMode
}

// GetIdentifier is used by the `predicate` library to evaluate variable expressions when
// evaluating policy filters. It deals with evaluating strings like `participant.name` to the appropriate value.
func (ctx *SessionAccessContext) GetIdentifier(fields []string) (interface{}, error) {
	if fields[0] == "user" {
		if len(fields) == 2 || len(fields) == 3 {
			checkedFieldIdx := 1
			// Unify the format. Moderated session originally skipped the spec field (user.roles was used instead of
			// user.spec.roles) which was not aligned with how our roles filtering works.
			// Here we try support both cases. We don't want to modify the original fields slice,
			// as that would change the reported error message (see return below).
			if len(fields) == 3 && fields[1] == "spec" {
				checkedFieldIdx = 2
			}
			switch fields[checkedFieldIdx] {
			case "name":
				return ctx.Username, nil
			case "roles":
				var roles []string
				for _, role := range ctx.Roles {
					roles = append(roles, role.GetName())
				}

				return roles, nil
			}
		}
	}

	return nil, trace.NotFound("%v is not defined", strings.Join(fields, "."))
}

func (ctx *SessionAccessContext) GetResource() (types.Resource, error) {
	return nil, trace.BadParameter("resource unsupported")
}

// IsModerated returns true if the session needs moderation.
func (e *SessionAccessEvaluator) IsModerated() bool {
	return e.isModerated
}

func (e *SessionAccessEvaluator) matchesPredicate(ctx *SessionAccessContext, require *types.SessionRequirePolicy, allow *types.SessionJoinPolicy) (bool, error) {
	if !e.matchesKind(allow.Kinds) {
		return false, nil
	}

	parser, err := services.NewWhereParser(ctx)
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

func (e *SessionAccessEvaluator) matchesJoin(allow *types.SessionJoinPolicy) bool {
	if !e.matchesKind(allow.Kinds) {
		return false
	}

	for _, allowRole := range allow.Roles {
		// GlobToRegexp makes sure this is always a valid regexp.
		expr := regexp.MustCompile("^" + utils.GlobToRegexp(allowRole) + "$")

		for _, policySet := range e.policySets {
			if expr.MatchString(policySet.Name) {
				return true
			}
		}
	}

	return false
}

func (e *SessionAccessEvaluator) matchesKind(allow []string) bool {
	if ContainsSessionKind(allow, e.kind) || ContainsSessionKind(allow, "*") {
		return true
	}

	return false
}

// RoleSupportsModeratedSessions checks if the role version is higher or equal to
// V5 - V5 is the version where ModeratedSession support was introduced.
func RoleSupportsModeratedSessions(roles []types.Role) bool {
	for _, role := range roles {
		switch role.GetVersion() {
		case types.V5, types.V6, types.V7:
			return true
		}
	}
	return false
}

// CanJoin returns the modes a user has access to join a session with.
// If the list is empty, the user doesn't have access to join the session at all.
func (e *SessionAccessEvaluator) CanJoin(user SessionAccessContext) []types.SessionParticipantMode {
	// If we don't support session access controls, return the default mode set that was supported prior to Moderated Sessions.
	if !RoleSupportsModeratedSessions(user.Roles) {
		return preAccessControlsModes(e.kind)
	}

	// Session owners can always join their own sessions.
	if user.Username == e.owner {
		return []types.SessionParticipantMode{types.SessionPeerMode, types.SessionModeratorMode, types.SessionObserverMode}
	}

	var modes []types.SessionParticipantMode

	// Loop over every allow policy attached the participant and check its applicability.
	// This code serves to merge the permissions of all applicable join policies.
	for _, allowPolicy := range getAllowPolicies(user) {
		// If the policy is applicable and allows joining the session, add the allowed modes to the list of modes.
		if e.matchesJoin(allowPolicy) {
			for _, modeString := range allowPolicy.Modes {
				mode := types.SessionParticipantMode(modeString)
				if !slices.Contains(modes, mode) {
					modes = append(modes, mode)
				}
			}
		}
	}

	return modes
}

// PolicyOptions is a set of settings for the session determined by the matched required policy.
type PolicyOptions struct {
	OnLeaveAction types.OnSessionLeaveAction
}

// PrettyRequirementsList generates a pretty-printed string of precise requirements for session start suitable for user display.
func (e *SessionAccessEvaluator) PrettyRequirementsList() string {
	s := new(strings.Builder)
	s.WriteString("require all:")

	for _, policySet := range e.policySets {
		policies := e.extractApplicablePolicies(policySet)
		if len(policies) == 0 {
			continue
		}

		fmt.Fprintf(s, "\r\n   one of (%v):", policySet.Name)
		for _, require := range policies {
			fmt.Fprintf(s, "\r\n    - %vx %v with mode %v", require.Count, require.Filter, strings.Join(require.Modes, " or "))
		}
	}

	return s.String()
}

// extractApplicablePolicies extracts all policies that match the session kind.
func (e *SessionAccessEvaluator) extractApplicablePolicies(set *types.SessionTrackerPolicySet) []*types.SessionRequirePolicy {
	var policies []*types.SessionRequirePolicy

	for _, require := range set.RequireSessionJoin {
		if e.matchesKind(require.Kinds) {
			policies = append(policies, require)
		}
	}

	return policies
}

func (e *SessionAccessEvaluator) GetFulfilledStatusFor(participants []SessionAccessContext) []PolicyFulfillmentStatus {
	var policyFullmentStatuses []PolicyFulfillmentStatus

	for _, policySet := range e.policySets {
		policies := e.extractApplicablePolicies(policySet)
		if len(policies) == 0 {
			continue
		}

		for _, requirePolicy := range policies {
			status := PolicyFulfillmentStatus{
				Count: requirePolicy.Count,
				Name:  requirePolicy.Name,
			}

			var requireModes []types.SessionParticipantMode
			for _, mode := range requirePolicy.Modes {
				requireModes = append(requireModes, types.SessionParticipantMode(mode))
			}

			for _, participant := range participants {
				if !slices.Contains(requireModes, participant.Mode) {
					continue
				}

				allowPolicies := getAllowPolicies(participant)
				for _, allowPolicy := range allowPolicies {
					matchesPredicate, err := e.matchesPredicate(&participant, requirePolicy, allowPolicy)
					if err != nil {
						// return false, options, trace.Wrap(err)
					}

					if matchesPredicate && e.matchesJoin(allowPolicy) {
						status.Satifies = append(status.Satifies, types.Participant{User: participant.Username, Mode: string(participant.Mode)})
					}
				}
			}
			policyFullmentStatuses = append(policyFullmentStatuses, status)

		}

	}

	return policyFullmentStatuses
}

// FulfilledFor checks if a given session may run with a list of participants.
func (e *SessionAccessEvaluator) FulfilledFor(participants []SessionAccessContext) (bool, PolicyOptions, error) {
	var options PolicyOptions

	// Check every policy set to check if it's fulfilled.
	// We need every policy set to match to allow the session.
policySetLoop:
	for _, policySet := range e.policySets {
		policies := e.extractApplicablePolicies(policySet)
		if len(policies) == 0 {
			continue
		}

		if options.OnLeaveAction != types.OnSessionLeaveTerminate {
			terminateOnLeave := types.OnSessionLeavePause
			for _, p := range policies {
				if p.OnLeave != string(types.OnSessionLeavePause) {
					terminateOnLeave = types.OnSessionLeaveTerminate
					break
				}
			}
			options = PolicyOptions{OnLeaveAction: terminateOnLeave}
		}

		// Check every require policy to see if it's fulfilled.
		// Only one needs to be checked to pass the policyset.
		for _, requirePolicy := range policies {
			// Count of how many additional participant matches we need to fulfill the policy.
			left := requirePolicy.Count

			var requireModes []types.SessionParticipantMode
			for _, mode := range requirePolicy.Modes {
				requireModes = append(requireModes, types.SessionParticipantMode(mode))
			}

			// Check every participant against the policy.
			for _, participant := range participants {
				if !slices.Contains(requireModes, participant.Mode) {
					continue
				}

				// Check the allow polices attached to the participant against the session.
				allowPolicies := getAllowPolicies(participant)
				for _, allowPolicy := range allowPolicies {
					// Evaluate the filter in the require policy against the participant and allow policy.
					matchesPredicate, err := e.matchesPredicate(&participant, requirePolicy, allowPolicy)
					if err != nil {
						return false, options, trace.Wrap(err)
					}

					// If the filter matches the participant and the allow policy matches the session
					// we conclude that the participant matches against the require policy.
					if matchesPredicate && e.matchesJoin(allowPolicy) {
						left--
						break
					}
				}

				// If we've matched enough participants against the require policy, we can allow the session.
				if left <= 0 {
					// We matched at least one require policy within the set. Continue ahead.
					continue policySetLoop
				}
			}
		}

		// We failed to match against any require policy and thus the set.
		// Thus, we can't allow the session.
		return false, options, nil
	}

	// All policy sets matched, we can allow the session.
	return true, options, nil
}

func preAccessControlsModes(kind types.SessionKind) []types.SessionParticipantMode {
	switch kind {
	case types.SSHSessionKind:
		return []types.SessionParticipantMode{types.SessionPeerMode}
	default:
		return nil
	}
}
