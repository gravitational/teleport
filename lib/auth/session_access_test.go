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
}

func successStartTestCase(t *testing.T) startTestCase {
	hostRole, err := types.NewRole("host", types.RoleSpecV5{})
	require.NoError(t, err)
	participantRole, err := types.NewRole("participant", types.RoleSpecV5{})
	require.NoError(t, err)

	hostRole.SetSessionRequirePolicies([]*types.SessionRequirePolicy{{
		Filter:  "contains(user.roles, \"participant\")",
		Kinds:   []string{string(types.SSHSessionKind), string(types.KubernetesSessionKind)},
		Count:   2,
		OnLeave: types.OnSessionLeaveTerminate,
		Modes:   []string{"peer"},
	}})

	participantRole.SetSessionJoinPolicies([]*types.SessionJoinPolicy{{
		Roles: []string{hostRole.GetName()},
		Kinds: []string{string(types.SSHSessionKind), string(types.KubernetesSessionKind)},
		Modes: []string{string("*")},
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
		expected: []bool{true, true},
	}
}

func successStartTestCaseSpec(t *testing.T) startTestCase {
	hostRole, err := types.NewRole("host", types.RoleSpecV5{})
	require.NoError(t, err)
	participantRole, err := types.NewRole("participant", types.RoleSpecV5{})
	require.NoError(t, err)

	hostRole.SetSessionRequirePolicies([]*types.SessionRequirePolicy{{
		Filter:  "contains(user.spec.roles, \"participant\")",
		Kinds:   []string{string(types.SSHSessionKind), string(types.KubernetesSessionKind)},
		Count:   2,
		OnLeave: types.OnSessionLeaveTerminate,
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
		expected: []bool{true, true},
	}
}

func failCountStartTestCase(t *testing.T) startTestCase {
	hostRole, err := types.NewRole("host", types.RoleSpecV5{})
	require.NoError(t, err)
	participantRole, err := types.NewRole("participant", types.RoleSpecV5{})
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
		Modes: []string{string("*")},
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
		expected: []bool{false, false},
	}
}

func succeedDiscardPolicySetStartTestCase(t *testing.T) startTestCase {
	hostRole, err := types.NewRole("host", types.RoleSpecV5{})
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
	hostRole, err := types.NewRole("host", types.RoleSpecV5{})
	require.NoError(t, err)
	participantRole, err := types.NewRole("participant", types.RoleSpecV5{})
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
		Modes: []string{string("*")},
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
		expected: []bool{false},
	}
}

func TestSessionAccessStart(t *testing.T) {
	testCases := []startTestCase{
		successStartTestCase(t),
		successStartTestCaseSpec(t),
		failCountStartTestCase(t),
		failFilterStartTestCase(t),
		succeedDiscardPolicySetStartTestCase(t),
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			var policies []*types.SessionTrackerPolicySet
			for _, role := range testCase.host {
				policySet := role.GetSessionPolicySet()
				policies = append(policies, &policySet)
			}

			for i, kind := range testCase.sessionKinds {
				evaluator := NewSessionAccessEvaluator(policies, kind, testCase.owner)
				result, _, err := evaluator.FulfilledFor(testCase.participants)
				require.NoError(t, err)
				require.Equal(t, testCase.expected[i], result)
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
	hostRole, err := types.NewRole("host", types.RoleSpecV5{})
	require.NoError(t, err)
	participantRole, err := types.NewRole("participant", types.RoleSpecV5{})
	require.NoError(t, err)

	participantRole.SetSessionJoinPolicies([]*types.SessionJoinPolicy{{
		Roles: []string{hostRole.GetName()},
		Kinds: []string{string(types.SSHSessionKind), string(types.KubernetesSessionKind)},
		Modes: []string{string("*")},
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
	hostRole, err := types.NewRole("host", types.RoleSpecV5{})
	require.NoError(t, err)
	participantRole, err := types.NewRole("participant", types.RoleSpecV5{})
	require.NoError(t, err)

	participantRole.SetSessionJoinPolicies([]*types.SessionJoinPolicy{{
		Roles: []string{"*"},
		Kinds: []string{string(types.SSHSessionKind), string(types.KubernetesSessionKind)},
		Modes: []string{string("*")},
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

func successSameUserJoinTestCase(t *testing.T) joinTestCase {
	hostRole, err := types.NewRole("host", types.RoleSpecV5{})
	require.NoError(t, err)
	participantRole, err := types.NewRole("participant", types.RoleSpecV5{})
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
	hostRole, err := types.NewRole("host", types.RoleSpecV5{})
	require.NoError(t, err)
	participantRole, err := types.NewRole("participant", types.RoleSpecV5{})
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
	hostRole, err := types.NewRole("host", types.RoleSpecV5{})
	require.NoError(t, err)
	participantRole, err := types.NewRole("participant", types.RoleSpecV5{})
	require.NoError(t, err)

	participantRole.SetSessionJoinPolicies([]*types.SessionJoinPolicy{{
		Roles: []string{hostRole.GetName()},
		Kinds: []string{string(types.KubernetesSessionKind)},
		Modes: []string{string("*")},
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

func versionDefaultJoinTestCase(t *testing.T) joinTestCase {
	hostRole, err := types.NewRole("host", types.RoleSpecV5{})
	require.NoError(t, err)
	participantRole, err := types.NewRoleV3("participant", types.RoleSpecV5{})
	require.NoError(t, err)

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
	testCases := []joinTestCase{
		successJoinTestCase(t),
		successGlobJoinTestCase(t),
		successSameUserJoinTestCase(t),
		failRoleJoinTestCase(t),
		failKindJoinTestCase(t),
		versionDefaultJoinTestCase(t),
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			for i, kind := range testCase.sessionKinds {
				policy := testCase.host.GetSessionPolicySet()
				evaluator := NewSessionAccessEvaluator([]*types.SessionTrackerPolicySet{&policy}, kind, testCase.owner)
				result := evaluator.CanJoin(testCase.participant)
				require.Equal(t, testCase.expected[i], len(result) > 0)
			}
		})
	}
}
