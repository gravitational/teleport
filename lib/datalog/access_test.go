// +build roletester

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

package datalog

import (
	"testing"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/services"
	"github.com/stretchr/testify/require"
	"gopkg.in/check.v1"
)

type AccessTestSuite struct {
	testUser types.User
	testRole types.Role
	testNode types.Server
}

var _ = check.Suite(&AccessTestSuite{})

func TestRootAccess(t *testing.T) { check.TestingT(t) }

func (s *AccessTestSuite) SetUpSuite(c *check.C) {
	testUser, err := types.NewUser("tester")
	c.Assert(err, check.IsNil)
	s.testUser = testUser
	s.testUser.SetRoles([]string{"testRole", "testRole1", "testRole2"})
	s.testUser.SetTraits(map[string][]string{
		"logins":     {"login1", "login2", "login3"},
		"otherTrait": {"trait1", "trait2"},
	})

	s.testRole = services.NewAdminRole()
	s.testRole.SetName("testRole")
	s.testRole.SetLogins(services.Allow, []string{"{{internal.logins}}", "root"})
	s.testRole.SetNodeLabels(services.Allow, map[string]apiutils.Strings{"env": []string{"example"}})

	testNode, err := types.NewServerWithLabels(
		"testNode",
		types.KindNode,
		types.ServerSpecV2{},
		map[string]string{"name": "testNode", "env": "example", "type": "test"},
	)
	c.Assert(err, check.IsNil)
	s.testNode = testNode
}

// TestMappings checks if all required string values are mapped to integer hashes
func (s *AccessTestSuite) TestMappings(c *check.C) {
	resp := AccessResponse{make(IDB), make(EDB), make(map[string]uint32), make(map[uint32]string)}
	resp.createUserMapping(s.testUser)
	resp.createRoleMapping(s.testRole)
	resp.createNodeMapping(s.testNode)

	require.Contains(c, resp.mappings, s.testUser.GetName())
	require.Equal(c, resp.reverseMappings[resp.mappings[s.testUser.GetName()]], s.testUser.GetName())
	for _, role := range s.testUser.GetRoles() {
		require.Contains(c, resp.mappings, role)
		require.Equal(c, resp.reverseMappings[resp.mappings[role]], role)
	}
	for _, login := range s.testUser.GetTraits()[teleport.TraitLogins] {
		require.Contains(c, resp.mappings, login)
		require.Equal(c, resp.reverseMappings[resp.mappings[login]], login)
	}
	require.Contains(c, resp.mappings, s.testRole.GetName())
	require.NotContains(c, resp.mappings, teleport.TraitInternalLoginsVariable)
	for _, login := range append(s.testRole.GetLogins(types.Allow), s.testRole.GetLogins(types.Deny)...) {
		if login == teleport.TraitInternalLoginsVariable {
			continue
		}
		require.Contains(c, resp.mappings, login)
		require.Equal(c, resp.reverseMappings[resp.mappings[login]], login)
	}
	for key, values := range s.testRole.GetNodeLabels(types.Allow) {
		require.Contains(c, resp.mappings, key)
		require.Equal(c, resp.reverseMappings[resp.mappings[key]], key)
		for _, value := range values {
			require.Contains(c, resp.mappings, value)
			require.Equal(c, resp.reverseMappings[resp.mappings[value]], value)
		}
	}
	for key, values := range s.testRole.GetNodeLabels(types.Deny) {
		require.Contains(c, resp.mappings, key)
		require.Equal(c, resp.reverseMappings[resp.mappings[key]], key)
		for _, value := range values {
			require.Contains(c, resp.mappings, value)
			require.Equal(c, resp.reverseMappings[resp.mappings[value]], value)
		}
	}
	require.Contains(c, resp.mappings, s.testNode.GetName())
	for key, value := range s.testNode.GetAllLabels() {
		require.Contains(c, resp.mappings, key)
		require.Contains(c, resp.mappings, value)
		require.Equal(c, resp.reverseMappings[resp.mappings[key]], key)
		require.Equal(c, resp.reverseMappings[resp.mappings[value]], value)
	}
}

// TestPredicates checks if all required predicates are created correctly with the right hashes
func (s *AccessTestSuite) TestPredicates(c *check.C) {
	// Test user predicates
	resp := AccessResponse{make(IDB), make(EDB), make(map[string]uint32), make(map[uint32]string)}
	resp.createUserPredicates(s.testUser, "")
	resp.createRolePredicates(s.testRole)
	resp.createNodePredicates(s.testNode)
	roleCountMap := make(map[string]bool)
	for _, pred := range resp.facts[hasRole] {
		require.Equal(c, resp.reverseMappings[pred.Atoms[0]], s.testUser.GetName())
		require.Contains(c, s.testUser.GetRoles(), resp.reverseMappings[pred.Atoms[1]])
		roleCountMap[resp.reverseMappings[pred.Atoms[1]]] = true
	}
	require.Equal(c, len(s.testUser.GetRoles()), len(roleCountMap))

	traitCountMap := make(map[string]bool)
	for _, pred := range resp.facts[hasTrait] {
		if pred.Atoms[1] != loginTraitHash {
			continue
		}
		require.Equal(c, resp.reverseMappings[pred.Atoms[0]], s.testUser.GetName())
		require.Contains(c, s.testUser.GetTraits()[teleport.TraitLogins], resp.reverseMappings[pred.Atoms[2]])
		traitCountMap[resp.reverseMappings[pred.Atoms[2]]] = true
	}
	require.Equal(c, len(s.testUser.GetTraits()[teleport.TraitLogins]), len(traitCountMap))

	// Test role logins
	loginCountMap := make(map[string]bool)
	allLogins := append(s.testRole.GetLogins(types.Allow), s.testRole.GetLogins(types.Deny)...)
	for _, pred := range append(resp.facts[roleAllowsLogin], resp.facts[roleDeniesLogin]...) {
		require.Equal(c, resp.reverseMappings[pred.Atoms[0]], s.testRole.GetName())
		require.Contains(c, allLogins, resp.reverseMappings[pred.Atoms[1]])
		loginCountMap[resp.reverseMappings[pred.Atoms[1]]] = true
	}
	require.Equal(c, 1, len(loginCountMap))

	// Test role labels
	allLabels := make(map[string][]string)
	for key, values := range s.testRole.GetNodeLabels(types.Allow) {
		for _, value := range values {
			allLabels[key] = append(allLabels[key], value)
		}
	}
	for _, pred := range resp.facts[roleAllowsNodeLabel] {
		require.Contains(c, allLabels[resp.reverseMappings[pred.Atoms[1]]], resp.reverseMappings[pred.Atoms[2]])
	}
	for _, pred := range resp.facts[roleDeniesNodeLabel] {
		require.Contains(c, allLabels[resp.reverseMappings[pred.Atoms[1]]], resp.reverseMappings[pred.Atoms[2]])
	}

	// Test node labels
	for _, pred := range resp.facts[nodeHasLabel] {
		require.Equal(c, s.testNode.GetAllLabels()[resp.reverseMappings[pred.Atoms[1]]], resp.reverseMappings[pred.Atoms[2]])
	}
}
