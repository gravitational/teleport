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

	"github.com/gravitational/teleport/api/types"
	"github.com/stretchr/testify/require"
)

type startTestCase struct {
	name         string
	host         types.Role
	sessionKind  types.SessionKind
	participants []SessionAccessContext
	expected     bool
}

func successStartTestCase(t *testing.T) startTestCase {
	hostRole, err := types.NewRole("host", types.RoleSpecV5{})
	require.NoError(t, err)
	participantRole, err := types.NewRole("participant", types.RoleSpecV5{})
	require.NoError(t, err)

	hostRole.SetSessionRequirePolicies([]*types.SessionRequirePolicy{{
		Filter:  "contains(user.roles, \"participant\")",
		Kinds:   []string{string(types.SSHSessionKind)},
		Count:   2,
		OnLeave: types.OnSessionLeaveTerminate,
		Modes:   []string{"peer"},
	}})

	participantRole.SetSessionJoinPolicies([]*types.SessionJoinPolicy{{
		Roles: []string{hostRole.GetName()},
		Kinds: []string{string(types.SSHSessionKind)},
		Modes: []string{string("*")},
	}})

	return startTestCase{
		name:        "success",
		host:        hostRole,
		sessionKind: types.SSHSessionKind,
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
		expected: true,
	}
}

func failCountStartTestCase(t *testing.T) startTestCase {
	hostRole, err := types.NewRole("host", types.RoleSpecV5{})
	require.NoError(t, err)
	participantRole, err := types.NewRole("participant", types.RoleSpecV5{})
	require.NoError(t, err)

	hostRole.SetSessionRequirePolicies([]*types.SessionRequirePolicy{{
		Filter: "contains(user.roles, \"participant\")",
		Kinds:  []string{string(types.SSHSessionKind)},
		Count:  3,
		Modes:  []string{"peer"},
	}})

	participantRole.SetSessionJoinPolicies([]*types.SessionJoinPolicy{{
		Roles: []string{hostRole.GetName()},
		Kinds: []string{string(types.SSHSessionKind)},
		Modes: []string{string("*")},
	}})

	return startTestCase{
		name:        "failCount",
		host:        hostRole,
		sessionKind: types.SSHSessionKind,
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
		expected: false,
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
		name:        "succeedDiscardPolicySet",
		host:        hostRole,
		sessionKind: types.SSHSessionKind,
		expected:    true,
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
		name:        "failFilter",
		host:        hostRole,
		sessionKind: types.SSHSessionKind,
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
		expected: false,
	}
}

func TestSessionAccessStart(t *testing.T) {
	testCases := []startTestCase{
		successStartTestCase(t),
		failCountStartTestCase(t),
		failFilterStartTestCase(t),
		succeedDiscardPolicySetStartTestCase(t),
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			policy := testCase.host.GetSessionPolicySet()
			evaluator := NewSessionAccessEvaluator([]*types.SessionTrackerPolicySet{&policy}, testCase.sessionKind)
			result, _, err := evaluator.FulfilledFor(testCase.participants)
			require.NoError(t, err)
			require.Equal(t, testCase.expected, result)
		})
	}
}

type joinTestCase struct {
	name        string
	host        types.Role
	sessionKind types.SessionKind
	participant SessionAccessContext
	expected    bool
}

func successJoinTestCase(t *testing.T) joinTestCase {
	hostRole, err := types.NewRole("host", types.RoleSpecV5{})
	require.NoError(t, err)
	participantRole, err := types.NewRole("participant", types.RoleSpecV5{})
	require.NoError(t, err)

	participantRole.SetSessionJoinPolicies([]*types.SessionJoinPolicy{{
		Roles: []string{hostRole.GetName()},
		Kinds: []string{string(types.SSHSessionKind)},
		Modes: []string{string("*")},
	}})

	return joinTestCase{
		name:        "success",
		host:        hostRole,
		sessionKind: types.SSHSessionKind,
		participant: SessionAccessContext{
			Username: "participant",
			Roles:    []types.Role{participantRole},
		},
		expected: true,
	}
}

func failRoleJoinTestCase(t *testing.T) joinTestCase {
	hostRole, err := types.NewRole("host", types.RoleSpecV5{})
	require.NoError(t, err)
	participantRole, err := types.NewRole("participant", types.RoleSpecV5{})
	require.NoError(t, err)

	return joinTestCase{
		name:        "failRole",
		host:        hostRole,
		sessionKind: types.SSHSessionKind,
		participant: SessionAccessContext{
			Username: "participant",
			Roles:    []types.Role{participantRole},
		},
		expected: false,
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
		name:        "failKind",
		host:        hostRole,
		sessionKind: types.SSHSessionKind,
		participant: SessionAccessContext{
			Username: "participant",
			Roles:    []types.Role{participantRole},
		},
		expected: false,
	}
}

func TestSessionAccessJoin(t *testing.T) {
	testCases := []joinTestCase{
		successJoinTestCase(t),
		failRoleJoinTestCase(t),
		failKindJoinTestCase(t),
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			policy := testCase.host.GetSessionPolicySet()
			evaluator := NewSessionAccessEvaluator([]*types.SessionTrackerPolicySet{&policy}, testCase.sessionKind)
			result, err := evaluator.CanJoin(testCase.participant)
			require.NoError(t, err)
			require.Equal(t, testCase.expected, len(result) > 0)
		})
	}
}
