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

package moderation

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

type startTestCase struct {
	name         string
	host         []types.Role
	sessionKinds []types.SessionKind
	participants []SessionAccessContext
	owner        string
	expected     []bool
	terminate    types.OnSessionLeaveAction
}

func successStartTestCase(t *testing.T) startTestCase {
	hostRole, err := types.NewRole("host", types.RoleSpecV6{})
	require.NoError(t, err)
	participantRole, err := types.NewRole("participant", types.RoleSpecV6{})
	require.NoError(t, err)

	hostRole.SetSessionRequirePolicies([]*types.SessionRequirePolicy{{
		Filter:  "contains(user.roles, \"participant\")",
		Kinds:   []string{string(types.SSHSessionKind), string(types.KubernetesSessionKind)},
		Count:   2,
		OnLeave: string(types.OnSessionLeaveTerminate),
		Modes:   []string{"peer"},
	}})

	participantRole.SetSessionJoinPolicies([]*types.SessionJoinPolicy{{
		Roles: []string{hostRole.GetName()},
		Kinds: []string{string(types.SSHSessionKind), string(types.KubernetesSessionKind)},
		Modes: []string{"*"},
	}})

	return startTestCase{
		name:         "success",
		host:         []types.Role{hostRole},
		sessionKinds: []types.SessionKind{types.SSHSessionKind, types.KubernetesSessionKind},
		participants: []SessionAccessContext{
			{
				Username: "participant",
				Roles:    []types.Role{participantRole},
				Mode:     "peer",
			},
			{
				Username: "participant2",
				Roles:    []types.Role{participantRole},
				Mode:     "peer",
			},
		},
		expected:  []bool{true, true},
		terminate: types.OnSessionLeaveTerminate,
	}
}

func successStartTestCasePause(t *testing.T) startTestCase {
	hostRole, err := types.NewRole("host", types.RoleSpecV6{})
	require.NoError(t, err)
	participantRole, err := types.NewRole("participant", types.RoleSpecV6{})
	require.NoError(t, err)

	hostRole.SetSessionRequirePolicies([]*types.SessionRequirePolicy{{
		Filter:  "contains(user.roles, \"participant\")",
		Kinds:   []string{string(types.SSHSessionKind), string(types.KubernetesSessionKind)},
		Count:   2,
		OnLeave: string(types.OnSessionLeavePause),
		Modes:   []string{"peer"},
	}})

	participantRole.SetSessionJoinPolicies([]*types.SessionJoinPolicy{{
		Roles: []string{hostRole.GetName()},
		Kinds: []string{string(types.SSHSessionKind), string(types.KubernetesSessionKind)},
		Modes: []string{"*"},
	}})

	return startTestCase{
		name:         "successStartTestCasePause",
		host:         []types.Role{hostRole},
		sessionKinds: []types.SessionKind{types.SSHSessionKind, types.KubernetesSessionKind},
		participants: []SessionAccessContext{
			{
				Username: "participant",
				Roles:    []types.Role{participantRole},
				Mode:     "peer",
			},
			{
				Username: "participant2",
				Roles:    []types.Role{participantRole},
				Mode:     "peer",
			},
		},
		expected:  []bool{true, true},
		terminate: types.OnSessionLeavePause,
	}
}

func pauseCanBeOverwritten(t *testing.T) startTestCase {
	hostRole, err := types.NewRole("host", types.RoleSpecV6{})
	require.NoError(t, err)
	participantRole, err := types.NewRole("participant", types.RoleSpecV6{})
	require.NoError(t, err)

	hostRole.SetSessionRequirePolicies([]*types.SessionRequirePolicy{
		{
			Filter:  "contains(user.roles, \"participant\")",
			Kinds:   []string{string(types.SSHSessionKind), string(types.KubernetesSessionKind)},
			Count:   2,
			OnLeave: string(types.OnSessionLeavePause),
			Modes:   []string{"peer"},
		},
		{
			Filter:  "contains(user.roles, \"participant\")",
			Kinds:   []string{string(types.SSHSessionKind), string(types.KubernetesSessionKind)},
			Count:   2,
			OnLeave: string(types.OnSessionLeaveTerminate),
			Modes:   []string{"peer"},
		},
	})

	participantRole.SetSessionJoinPolicies([]*types.SessionJoinPolicy{{
		Roles: []string{hostRole.GetName()},
		Kinds: []string{string(types.SSHSessionKind), string(types.KubernetesSessionKind)},
		Modes: []string{"*"},
	}})

	return startTestCase{
		name:         "pauseCanBeOverwritten",
		host:         []types.Role{hostRole},
		sessionKinds: []types.SessionKind{types.SSHSessionKind, types.KubernetesSessionKind},
		participants: []SessionAccessContext{
			{
				Username: "participant",
				Roles:    []types.Role{participantRole},
				Mode:     "peer",
			},
			{
				Username: "participant2",
				Roles:    []types.Role{participantRole},
				Mode:     "peer",
			},
		},
		expected:  []bool{true, true},
		terminate: types.OnSessionLeaveTerminate,
	}
}

func successStartTestCaseSpec(t *testing.T) startTestCase {
	hostRole, err := types.NewRole("host", types.RoleSpecV6{})
	require.NoError(t, err)
	participantRole, err := types.NewRole("participant", types.RoleSpecV6{})
	require.NoError(t, err)

	hostRole.SetSessionRequirePolicies([]*types.SessionRequirePolicy{{
		Filter:  "contains(user.spec.roles, \"participant\")",
		Kinds:   []string{string(types.SSHSessionKind), string(types.KubernetesSessionKind)},
		Count:   2,
		OnLeave: string(types.OnSessionLeaveTerminate),
		Modes:   []string{"peer"},
	}})

	participantRole.SetSessionJoinPolicies([]*types.SessionJoinPolicy{{
		Roles: []string{hostRole.GetName()},
		Kinds: []string{string(types.SSHSessionKind), string(types.KubernetesSessionKind)},
		Modes: []string{"*"},
	}})

	return startTestCase{
		name:         "success with spec",
		host:         []types.Role{hostRole},
		sessionKinds: []types.SessionKind{types.SSHSessionKind, types.KubernetesSessionKind},
		participants: []SessionAccessContext{
			{
				Username: "participant",
				Roles:    []types.Role{participantRole},
				Mode:     "peer",
			},
			{
				Username: "participant2",
				Roles:    []types.Role{participantRole},
				Mode:     "peer",
			},
		},
		expected:  []bool{true, true},
		terminate: types.OnSessionLeaveTerminate,
	}
}

func failCountStartTestCase(t *testing.T) startTestCase {
	hostRole, err := types.NewRole("host", types.RoleSpecV6{})
	require.NoError(t, err)
	participantRole, err := types.NewRole("participant", types.RoleSpecV6{})
	require.NoError(t, err)

	hostRole.SetSessionRequirePolicies([]*types.SessionRequirePolicy{{
		Filter: "contains(user.roles, \"participant\")",
		Kinds:  []string{string(types.SSHSessionKind), string(types.KubernetesSessionKind)},
		Count:  3,
		Modes:  []string{"peer"},
	}})

	participantRole.SetSessionJoinPolicies([]*types.SessionJoinPolicy{{
		Roles: []string{hostRole.GetName()},
		Kinds: []string{string(types.SSHSessionKind), string(types.KubernetesSessionKind)},
		Modes: []string{"*"},
	}})

	return startTestCase{
		name:         "failCount",
		host:         []types.Role{hostRole},
		sessionKinds: []types.SessionKind{types.SSHSessionKind, types.KubernetesSessionKind},
		participants: []SessionAccessContext{
			{
				Username: "participant",
				Roles:    []types.Role{participantRole},
				Mode:     "peer",
			},
			{
				Username: "participant2",
				Roles:    []types.Role{participantRole},
				Mode:     "peer",
			},
		},
		expected:  []bool{false, false},
		terminate: types.OnSessionLeaveTerminate,
	}
}

func succeedDiscardPolicySetStartTestCase(t *testing.T) startTestCase {
	hostRole, err := types.NewRole("host", types.RoleSpecV6{})
	require.NoError(t, err)

	hostRole.SetSessionRequirePolicies([]*types.SessionRequirePolicy{{
		Filter: "contains(user.roles, \"host\")",
		Kinds:  []string{string(types.KubernetesSessionKind)},
		Count:  2,
		Modes:  []string{"peer"},
	}})

	return startTestCase{
		name:         "succeedDiscardPolicySet",
		host:         []types.Role{hostRole},
		sessionKinds: []types.SessionKind{types.SSHSessionKind},
		expected:     []bool{true},
	}
}

func failFilterStartTestCase(t *testing.T) startTestCase {
	hostRole, err := types.NewRole("host", types.RoleSpecV6{})
	require.NoError(t, err)
	participantRole, err := types.NewRole("participant", types.RoleSpecV6{})
	require.NoError(t, err)

	hostRole.SetSessionRequirePolicies([]*types.SessionRequirePolicy{{
		Filter: "contains(user.roles, \"host\")",
		Kinds:  []string{string(types.SSHSessionKind)},
		Count:  2,
		Modes:  []string{"peer"},
	}})

	participantRole.SetSessionJoinPolicies([]*types.SessionJoinPolicy{{
		Roles: []string{hostRole.GetName()},
		Kinds: []string{string(types.SSHSessionKind)},
		Modes: []string{"*"},
	}})

	return startTestCase{
		name:         "failFilter",
		host:         []types.Role{hostRole},
		sessionKinds: []types.SessionKind{types.SSHSessionKind},
		participants: []SessionAccessContext{
			{
				Username: "participant",
				Roles:    []types.Role{participantRole},
				Mode:     "peer",
			},
			{
				Username: "participant2",
				Roles:    []types.Role{participantRole},
				Mode:     "peer",
			},
		},
		expected:  []bool{false},
		terminate: types.OnSessionLeaveTerminate,
	}
}

func TestSessionAccessStart(t *testing.T) {
	t.Parallel()

	testCases := []startTestCase{
		successStartTestCase(t),
		successStartTestCasePause(t),
		successStartTestCaseSpec(t),
		failCountStartTestCase(t),
		failFilterStartTestCase(t),
		succeedDiscardPolicySetStartTestCase(t),
		pauseCanBeOverwritten(t),
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			var policies []*types.SessionTrackerPolicySet
			for _, role := range testCase.host {
				policySet := role.GetSessionPolicySet()
				policies = append(policies, &policySet)
			}

			for i, kind := range testCase.sessionKinds {
				evaluator := NewSessionAccessEvaluator(policies, kind, testCase.owner)
				result, policyOptions, err := evaluator.FulfilledFor(testCase.participants)
				require.NoError(t, err)
				require.Equal(t, testCase.expected[i], result)
				require.Equal(t, testCase.terminate, policyOptions.OnLeaveAction)
			}
		})
	}
}

type joinTestCase struct {
	name         string
	host         types.Role
	sessionKinds []types.SessionKind
	participant  SessionAccessContext
	owner        string
	expected     []bool
}

func successJoinTestCase(t *testing.T) joinTestCase {
	hostRole, err := types.NewRole("host", types.RoleSpecV6{})
	require.NoError(t, err)
	participantRole, err := types.NewRole("participant", types.RoleSpecV6{})
	require.NoError(t, err)

	participantRole.SetSessionJoinPolicies([]*types.SessionJoinPolicy{{
		Roles: []string{hostRole.GetName()},
		Kinds: []string{string(types.SSHSessionKind), string(types.KubernetesSessionKind)},
		Modes: []string{types.Wildcard},
	}})

	return joinTestCase{
		name:         "success",
		host:         hostRole,
		sessionKinds: []types.SessionKind{types.SSHSessionKind, types.KubernetesSessionKind},
		participant: SessionAccessContext{
			Username: "participant",
			Roles:    []types.Role{participantRole},
		},
		expected: []bool{true, true},
	}
}

func successGlobJoinTestCase(t *testing.T) joinTestCase {
	hostRole, err := types.NewRole("host", types.RoleSpecV6{})
	require.NoError(t, err)
	participantRole, err := types.NewRole("participant", types.RoleSpecV6{})
	require.NoError(t, err)

	participantRole.SetSessionJoinPolicies([]*types.SessionJoinPolicy{{
		Roles: []string{types.Wildcard},
		Kinds: []string{string(types.SSHSessionKind), string(types.KubernetesSessionKind)},
		Modes: []string{types.Wildcard},
	}})

	return joinTestCase{
		name:         "successGlobJoin",
		host:         hostRole,
		sessionKinds: []types.SessionKind{types.SSHSessionKind, types.KubernetesSessionKind},
		participant: SessionAccessContext{
			Username: "participant",
			Roles:    []types.Role{participantRole},
		},
		expected: []bool{true, true},
	}
}

func successSameUserJoinTestCase(t *testing.T) joinTestCase {
	hostRole, err := types.NewRole("host", types.RoleSpecV6{})
	require.NoError(t, err)
	participantRole, err := types.NewRole("participant", types.RoleSpecV6{})
	require.NoError(t, err)

	return joinTestCase{
		name:         "successSameUser",
		host:         hostRole,
		sessionKinds: []types.SessionKind{types.SSHSessionKind},
		participant: SessionAccessContext{
			Username: "john",
			Roles:    []types.Role{participantRole},
		},
		owner:    "john",
		expected: []bool{true},
	}
}

func failRoleJoinTestCase(t *testing.T) joinTestCase {
	hostRole, err := types.NewRole("host", types.RoleSpecV6{})
	require.NoError(t, err)
	participantRole, err := types.NewRole("participant", types.RoleSpecV6{})
	require.NoError(t, err)

	return joinTestCase{
		name:         "failRole",
		host:         hostRole,
		sessionKinds: []types.SessionKind{types.SSHSessionKind, types.KubernetesSessionKind},
		participant: SessionAccessContext{
			Username: "participant",
			Roles:    []types.Role{participantRole},
		},
		expected: []bool{false, false},
	}
}

func failKindJoinTestCase(t *testing.T) joinTestCase {
	hostRole, err := types.NewRole("host", types.RoleSpecV6{})
	require.NoError(t, err)
	participantRole, err := types.NewRole("participant", types.RoleSpecV6{})
	require.NoError(t, err)

	participantRole.SetSessionJoinPolicies([]*types.SessionJoinPolicy{{
		Roles: []string{hostRole.GetName()},
		Kinds: []string{string(types.KubernetesSessionKind)},
		Modes: []string{types.Wildcard},
	}})

	return joinTestCase{
		name:         "failKind",
		host:         hostRole,
		sessionKinds: []types.SessionKind{types.SSHSessionKind},
		participant: SessionAccessContext{
			Username: "participant",
			Roles:    []types.Role{participantRole},
		},
		expected: []bool{false},
	}
}

// Tests to make sure that the regexp matching for roles only matches a full string
// match and not just any substring match.
// In this test case, we are making sure that having access to sessions hosted
// by someone with the role `test` doesn't also grant you access to sessions
// hosted by someone with the role `prod-test`.
func failJoinRoleNameInSubstringTestCase(t *testing.T) joinTestCase {
	hostRole, err := types.NewRole("prod-test", types.RoleSpecV6{})
	require.NoError(t, err)
	participantRole, err := types.NewRole("participant", types.RoleSpecV6{})
	require.NoError(t, err)

	participantRole.SetSessionJoinPolicies([]*types.SessionJoinPolicy{{
		Roles: []string{"test"},
		Kinds: []string{string(types.SSHSessionKind), string(types.KubernetesSessionKind)},
		Modes: []string{types.Wildcard},
	}})

	return joinTestCase{
		name:         "failRoleInSubstring",
		host:         hostRole,
		sessionKinds: []types.SessionKind{types.SSHSessionKind, types.KubernetesSessionKind},
		participant: SessionAccessContext{
			Username: "participant",
			Roles:    []types.Role{participantRole},
		},
		expected: []bool{false, false},
	}
}

func versionDefaultJoinTestCase(t *testing.T) joinTestCase {
	hostRole, err := types.NewRole("host", types.RoleSpecV6{})
	require.NoError(t, err)

	// create a v3 role to check that access controls
	// prior to Moderated Sessions are honored
	participantRole := &types.RoleV6{
		Version: types.V3,
		Metadata: types.Metadata{
			Name: "participant",
		},
		Spec: types.RoleSpecV6{},
	}
	require.NoError(t, participantRole.CheckAndSetDefaults())

	return joinTestCase{
		name:         "failVersion",
		host:         hostRole,
		sessionKinds: []types.SessionKind{types.SSHSessionKind, types.KubernetesSessionKind},
		participant: SessionAccessContext{
			Username: "participant",
			Roles:    []types.Role{participantRole},
		},
		expected: []bool{true, false},
	}
}

func TestSessionAccessJoin(t *testing.T) {
	t.Parallel()

	testCases := []joinTestCase{
		successJoinTestCase(t),
		successGlobJoinTestCase(t),
		successSameUserJoinTestCase(t),
		failRoleJoinTestCase(t),
		failKindJoinTestCase(t),
		failJoinRoleNameInSubstringTestCase(t),
		versionDefaultJoinTestCase(t),
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			for i, kind := range testCase.sessionKinds {
				t.Run(string(kind), func(t *testing.T) {
					policy := testCase.host.GetSessionPolicySet()
					evaluator := NewSessionAccessEvaluator([]*types.SessionTrackerPolicySet{&policy}, kind, testCase.owner)
					result := evaluator.CanJoin(testCase.participant)
					require.Equal(t, testCase.expected[i], len(result) > 0)
				})
			}
		})
	}
}

func TestSessionAccessEvaluatorCommandApproval(t *testing.T) {
	enabled := &types.CommandApproval{
		Enabled:  true,
		Approver: types.CommandApproverHuman,
	}
	disabled := &types.CommandApproval{
		Enabled:  false,
		Approver: types.CommandApproverHuman,
	}

	tests := []struct {
		name       string
		policySets []*types.SessionTrackerPolicySet
		kind       types.SessionKind
		wantNil    bool
	}{
		{
			name:       "no policy sets",
			policySets: nil,
			kind:       types.SSHSessionKind,
			wantNil:    true,
		},
		{
			name: "no command approval",
			policySets: []*types.SessionTrackerPolicySet{{
				Name: "p1",
				RequireSessionJoin: []*types.SessionRequirePolicy{{
					Name:  "r1",
					Kinds: []string{string(types.SSHSessionKind)},
				}},
			}},
			kind:    types.SSHSessionKind,
			wantNil: true,
		},
		{
			name: "disabled command approval",
			policySets: []*types.SessionTrackerPolicySet{{
				Name: "p1",
				RequireSessionJoin: []*types.SessionRequirePolicy{{
					Name:            "r1",
					Kinds:           []string{string(types.SSHSessionKind)},
					CommandApproval: disabled,
				}},
			}},
			kind:    types.SSHSessionKind,
			wantNil: true,
		},
		{
			name: "enabled command approval for SSH",
			policySets: []*types.SessionTrackerPolicySet{{
				Name: "p1",
				RequireSessionJoin: []*types.SessionRequirePolicy{{
					Name:            "r1",
					Kinds:           []string{string(types.SSHSessionKind)},
					CommandApproval: enabled,
				}},
			}},
			kind:    types.SSHSessionKind,
			wantNil: false,
		},
		{
			name: "enabled command approval with empty kinds defaults to SSH",
			policySets: []*types.SessionTrackerPolicySet{{
				Name: "p1",
				RequireSessionJoin: []*types.SessionRequirePolicy{{
					Name:            "r1",
					CommandApproval: enabled,
				}},
			}},
			kind:    types.SSHSessionKind,
			wantNil: false,
		},
		{
			name: "empty kinds does not apply to non-SSH kind",
			policySets: []*types.SessionTrackerPolicySet{{
				Name: "p1",
				RequireSessionJoin: []*types.SessionRequirePolicy{{
					Name:            "r1",
					CommandApproval: enabled,
				}},
			}},
			kind:    types.KubernetesSessionKind,
			wantNil: true,
		},
		{
			name: "returns first enabled across policy sets",
			policySets: []*types.SessionTrackerPolicySet{
				{
					Name: "p1",
					RequireSessionJoin: []*types.SessionRequirePolicy{{
						Name:            "r1",
						Kinds:           []string{string(types.SSHSessionKind)},
						CommandApproval: disabled,
					}},
				},
				{
					Name: "p2",
					RequireSessionJoin: []*types.SessionRequirePolicy{{
						Name:            "r2",
						Kinds:           []string{string(types.SSHSessionKind)},
						CommandApproval: enabled,
					}},
				},
			},
			kind:    types.SSHSessionKind,
			wantNil: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := NewSessionAccessEvaluator(tt.policySets, tt.kind, "owner")
			got := e.CommandApproval()
			if tt.wantNil {
				require.Nil(t, got)
				return
			}
			require.NotNil(t, got)
			require.True(t, got.Enabled)
		})
	}
}

// TestFulfilledForAICommandApproval verifies that a require policy configured
// for autonomous AI command approval lets a session start without any human
// moderator, while a human-approver (or ordinary moderation) policy still
// requires a real matching moderator.
func TestFulfilledForAICommandApproval(t *testing.T) {
	aiPolicy := &types.CommandApproval{
		Enabled:  true,
		Approver: types.CommandApproverAI,
		AI:       &types.CommandApprovalAI{Policy: "deny dangerous commands", Model: "claude"},
	}
	humanPolicy := &types.CommandApproval{
		Enabled:  true,
		Approver: types.CommandApproverHuman,
	}

	newEvaluator := func(ca *types.CommandApproval) SessionAccessEvaluator {
		return NewSessionAccessEvaluator([]*types.SessionTrackerPolicySet{{
			Name: "p1",
			RequireSessionJoin: []*types.SessionRequirePolicy{{
				Name:            "approve",
				Filter:          "contains(user.roles, 'moderator')",
				Kinds:           []string{string(types.SSHSessionKind)},
				Modes:           []string{string(types.SessionModeratorMode)},
				Count:           1,
				CommandApproval: ca,
			}},
		}}, types.SSHSessionKind, "owner")
	}

	t.Run("AI approver session starts without a human", func(t *testing.T) {
		e := newEvaluator(aiPolicy)
		// No participants at all: the AI moderator satisfies the require policy.
		ok, _, err := e.FulfilledFor(nil)
		require.NoError(t, err)
		require.True(t, ok, "AI-approver policy must be fulfilled without a human moderator")
	})

	t.Run("AI approver session starts with only a peer", func(t *testing.T) {
		e := newEvaluator(aiPolicy)
		ok, _, err := e.FulfilledFor([]SessionAccessContext{{
			Username: "peer",
			Mode:     types.SessionPeerMode,
		}})
		require.NoError(t, err)
		require.True(t, ok, "AI-approver policy must be fulfilled by peers only")
	})

	t.Run("human approver session is NOT fulfilled without a moderator", func(t *testing.T) {
		e := newEvaluator(humanPolicy)
		ok, _, err := e.FulfilledFor(nil)
		require.NoError(t, err)
		require.False(t, ok, "human-approver policy must still require a real moderator")
	})
}

// TestFulfilledForAIDoesNotBypassSeparateHumanRequirement is a security
// regression lock: when one policy set carries an AI-approver command_approval
// require policy (auto-satisfied without any human) AND a *separate* policy set
// carries an ordinary require policy that needs a real human moderator, the AI
// auto-satisfy must NOT bypass the human requirement coming from the other set.
// FulfilledFor must return FALSE until a matching human moderator participant is
// present, and TRUE once it is.
func TestFulfilledForAIDoesNotBypassSeparateHumanRequirement(t *testing.T) {
	moderatorRole, err := types.NewRole("moderator", types.RoleSpecV6{})
	require.NoError(t, err)
	// The moderator participant must carry a join policy that matches the
	// session so it counts toward the require policy (FulfilledFor matches both
	// the require filter and the participant's allow/join policy).
	moderatorRole.SetSessionJoinPolicies([]*types.SessionJoinPolicy{{
		Roles: []string{types.Wildcard},
		Kinds: []string{string(types.SSHSessionKind)},
		Modes: []string{types.Wildcard},
	}})

	aiSet := &types.SessionTrackerPolicySet{
		Name: "ai-set",
		RequireSessionJoin: []*types.SessionRequirePolicy{{
			Name:   "ai-approve",
			Filter: `contains(user.roles, "moderator")`,
			Kinds:  []string{string(types.SSHSessionKind)},
			Modes:  []string{string(types.SessionModeratorMode)},
			Count:  1,
			CommandApproval: &types.CommandApproval{
				Enabled:  true,
				Approver: types.CommandApproverAI,
				AI:       &types.CommandApprovalAI{Policy: "deny dangerous commands", Model: "claude"},
			},
		}},
	}
	// Ordinary moderation require policy (no command approval): genuinely needs
	// a human moderator participant.
	humanSet := &types.SessionTrackerPolicySet{
		Name: "human-set",
		RequireSessionJoin: []*types.SessionRequirePolicy{{
			Name:   "human-moderator",
			Filter: `contains(user.roles, "moderator")`,
			Kinds:  []string{string(types.SSHSessionKind)},
			Modes:  []string{string(types.SessionModeratorMode)},
			Count:  1,
		}},
	}

	e := NewSessionAccessEvaluator(
		[]*types.SessionTrackerPolicySet{aiSet, humanSet},
		types.SSHSessionKind,
		"owner",
	)

	t.Run("AI set must NOT bypass the human requirement from the other set", func(t *testing.T) {
		// No moderator present: the AI set auto-satisfies, but the human set
		// must remain unfulfilled, so the overall result is FALSE.
		ok, _, err := e.FulfilledFor(nil)
		require.NoError(t, err)
		require.False(t, ok, "AI auto-satisfy must not bypass a human requirement from a separate policy set")

		// A non-moderator peer is not enough either.
		ok, _, err = e.FulfilledFor([]SessionAccessContext{{
			Username: "peer",
			Mode:     types.SessionPeerMode,
		}})
		require.NoError(t, err)
		require.False(t, ok, "a non-moderator participant must not satisfy the human requirement")
	})

	t.Run("session starts once a real human moderator joins", func(t *testing.T) {
		ok, _, err := e.FulfilledFor([]SessionAccessContext{{
			Username: "human-mod",
			Roles:    []types.Role{moderatorRole},
			Mode:     types.SessionModeratorMode,
		}})
		require.NoError(t, err)
		require.True(t, ok, "AI set auto-satisfies and human set is satisfied by the moderator participant")
	})
}
