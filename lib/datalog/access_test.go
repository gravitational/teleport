//go:build roletester
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
	"context"
	"testing"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"gopkg.in/check.v1"
)

type mockClient struct {
	auth.ClientI

	users []types.User
	roles []types.Role
	nodes []types.Server
}

const (
	denyNullString   = "No denied access found.\n"
	accessNullString = "No access found.\n"
)

func (c mockClient) GetUser(name string, withSecrets bool) (types.User, error) {
	for _, user := range c.users {
		if user.GetName() == name {
			return user, nil
		}
	}
	return nil, trace.AccessDenied("No user")
}

func (c mockClient) GetUsers(withSecrets bool) ([]types.User, error) {
	return c.users, nil
}

func (c mockClient) GetRoles(ctx context.Context) ([]types.Role, error) {
	return c.roles, nil
}

func (c mockClient) GetNodes(ctx context.Context, namespace string, opts ...services.MarshalOption) ([]types.Server, error) {
	return c.nodes, nil
}

type AccessTestSuite struct {
	testUser      types.User
	testRole      types.Role
	testNode      types.Server
	client        mockClient
	emptyAccesses mockClient
	emptyDenies   mockClient
	empty         mockClient
}

var _ = check.Suite(&AccessTestSuite{})

func TestRootAccess(t *testing.T) { check.TestingT(t) }

func createUser(name string, roles []string, traits map[string][]string) (types.User, error) {
	user, err := types.NewUser(name)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	user.SetRoles(roles)
	user.SetTraits(traits)
	return user, nil
}

func createRole(name string, allowLogins []string, denyLogins []string, allowLabels types.Labels, denyLabels types.Labels) (types.Role, error) {
	role, err := types.NewRoleV3(name, types.RoleSpecV5{
		Allow: types.RoleConditions{
			Logins:     allowLogins,
			NodeLabels: allowLabels,
		},
		Deny: types.RoleConditions{
			Logins:     denyLogins,
			NodeLabels: denyLabels,
		},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return role, nil
}

func createNode(name string, kind string, hostname string, labels map[string]string) (types.Server, error) {
	node, err := types.NewServerWithLabels(name, kind, types.ServerSpecV2{
		Hostname: hostname,
	}, labels)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return node, nil
}

func tableToString(resp *NodeAccessResponse) string {
	accessTable, denyTable, accessLen, denyLen := resp.ToTable()
	var denyOutputString string
	if denyLen == 0 {
		denyOutputString = denyNullString
	} else {
		denyOutputString = denyTable.AsBuffer().String()
	}

	var accessOutputString string
	if accessLen == 0 {
		accessOutputString = accessNullString
	} else {
		accessOutputString = accessTable.AsBuffer().String()
	}
	return accessOutputString + "\n" + denyOutputString
}

func (s *AccessTestSuite) SetUpSuite(c *check.C) {
	bob, err := createUser("bob", []string{"admin", "dev"}, map[string][]string{"logins": {"bob", "ubuntu"}})
	c.Assert(err, check.IsNil)
	joe, err := createUser("joe", []string{"dev", "lister"}, map[string][]string{"logins": {"joe"}})
	c.Assert(err, check.IsNil)
	rui, err := createUser("rui", []string{"intern"}, map[string][]string{"logins": {"rui"}})
	c.Assert(err, check.IsNil)
	julia, err := createUser("julia", []string{"auditor"}, map[string][]string{"logins": {"julia"}})
	c.Assert(err, check.IsNil)

	// Allow case
	admin, err := createRole(
		"admin",
		[]string{"root", "admin", teleport.TraitInternalLoginsVariable},
		[]string{},
		types.Labels{types.Wildcard: []string{types.Wildcard}},
		types.Labels{},
	)
	c.Assert(err, check.IsNil)
	// Denied login case
	dev, err := createRole(
		"dev",
		[]string{"dev", teleport.TraitInternalLoginsVariable},
		[]string{"admin"},
		types.Labels{"env": []string{"prod", "test"}},
		types.Labels{},
	)
	c.Assert(err, check.IsNil)
	// Denied node case
	lister, err := createRole(
		"lister",
		[]string{"lister", teleport.TraitInternalLoginsVariable},
		[]string{},
		types.Labels{types.Wildcard: []string{types.Wildcard}},
		types.Labels{"env": []string{"prod", "test"}},
	)
	c.Assert(err, check.IsNil)
	// Denied login and denied node case
	intern, err := createRole(
		"intern",
		[]string{"intern", teleport.TraitInternalLoginsVariable},
		[]string{"rui"},
		types.Labels{},
		types.Labels{"env": []string{"prod"}},
	)
	c.Assert(err, check.IsNil)
	// Denied login traits
	auditor, err := createRole(
		"auditor",
		[]string{"auditor", teleport.TraitInternalLoginsVariable},
		[]string{teleport.TraitInternalLoginsVariable},
		types.Labels{types.Wildcard: []string{types.Wildcard}},
		types.Labels{"env": []string{"prod"}},
	)
	c.Assert(err, check.IsNil)

	prod, err := createNode("prod", "node", "prod.example.com", map[string]string{"env": "prod"})
	c.Assert(err, check.IsNil)
	test, err := createNode("test", "node", "test.example.com", map[string]string{"env": "test"})
	c.Assert(err, check.IsNil)
	secret, err := createNode("secret", "node", "secret.example.com", map[string]string{"env": "secret"})
	c.Assert(err, check.IsNil)

	users := []types.User{bob, joe, rui, julia}
	roles := []types.Role{admin, dev, lister, intern, auditor}
	nodes := []types.Server{prod, test, secret}
	s.client = mockClient{
		users: users,
		roles: roles,
		nodes: nodes,
	}
	s.emptyAccesses = mockClient{
		users: []types.User{rui},
		roles: roles,
		nodes: nodes,
	}
	s.emptyDenies = mockClient{
		users: []types.User{bob},
		roles: []types.Role{admin},
		nodes: nodes,
	}
	s.empty = mockClient{}

	testUser, err := types.NewUser("tester")
	c.Assert(err, check.IsNil)
	s.testUser = testUser
	s.testUser.SetRoles([]string{"testRole", "testRole1", "testRole2"})
	s.testUser.SetTraits(map[string][]string{
		"logins":     {"login1", "login2", "login3"},
		"otherTrait": {"trait1", "trait2"},
	})

	s.testRole = services.NewImplicitRole()
	s.testRole.SetName("testRole")
	s.testRole.SetLogins(types.Allow, []string{"{{internal.logins}}", "root"})
	s.testRole.SetNodeLabels(types.Allow, map[string]apiutils.Strings{"env": []string{"example"}})

	testNode, err := types.NewServerWithLabels(
		"testNode",
		types.KindNode,
		types.ServerSpecV2{},
		map[string]string{"name": "testNode", "env": "example", "type": "test"},
	)
	c.Assert(err, check.IsNil)
	s.testNode = testNode
}

// TestAccessDeduction checks if all the deduced access facts are correct.
func (s *AccessTestSuite) TestAccessDeduction(c *check.C) {
	access := NodeAccessRequest{}
	ctx := context.TODO()
	resp, err := QueryNodeAccess(ctx, s.client, access)
	c.Assert(err, check.IsNil)
	accessTable := asciitable.MakeTable([]string{"User", "Login", "Node", "Allowing Roles"})
	denyTable := asciitable.MakeTable([]string{"User", "Logins", "Node", "Denying Role"})
	accessTestOutput := [][]string{
		{"bob", "bob", "prod.example.com", "admin, dev"},
		{"bob", "bob", "secret.example.com", "admin"},
		{"bob", "bob", "test.example.com", "admin, dev"},
		{"bob", "dev", "prod.example.com", "dev"},
		{"bob", "dev", "test.example.com", "dev"},
		{"bob", "root", "prod.example.com", "admin"},
		{"bob", "root", "secret.example.com", "admin"},
		{"bob", "root", "test.example.com", "admin"},
		{"bob", "ubuntu", "prod.example.com", "admin, dev"},
		{"bob", "ubuntu", "secret.example.com", "admin"},
		{"bob", "ubuntu", "test.example.com", "admin, dev"},
		{"joe", "joe", "secret.example.com", "lister"},
		{"joe", "lister", "secret.example.com", "lister"},
		{"julia", "auditor", "secret.example.com", "auditor"},
		{"julia", "auditor", "test.example.com", "auditor"},
	}
	denyTestOutput := [][]string{
		{"bob", "admin", types.Wildcard, "dev"},
		{"joe", "admin", types.Wildcard, "dev"},
		{"joe", "dev, joe, lister", "prod.example.com", "lister"},
		{"joe", "dev, joe, lister", "test.example.com", "lister"},
		{"julia", "julia", types.Wildcard, "auditor"},
		{"julia", "auditor, julia", "prod.example.com", "auditor"},
		{"rui", "rui", types.Wildcard, "intern"},
		{"rui", "rui", "prod.example.com", "intern"},
	}
	for _, row := range accessTestOutput {
		accessTable.AddRow(row)
	}
	for _, row := range denyTestOutput {
		denyTable.AddRow(row)
	}
	c.Assert(accessTable.AsBuffer().String()+"\n"+denyTable.AsBuffer().String(), check.Equals, tableToString(resp))
}

// TestNoAccesses tests the output is correct when there are no access facts.
func (s *AccessTestSuite) TestNoAccesses(c *check.C) {
	access := NodeAccessRequest{}
	ctx := context.TODO()
	resp, err := QueryNodeAccess(ctx, s.emptyAccesses, access)
	c.Assert(err, check.IsNil)
	denyTable := asciitable.MakeTable([]string{"User", "Logins", "Node", "Denying Role"})
	denyTestOutput := [][]string{
		{"rui", "rui", types.Wildcard, "intern"},
		{"rui", "rui", "prod.example.com", "intern"},
	}
	for _, row := range denyTestOutput {
		denyTable.AddRow(row)
	}
	c.Assert(accessNullString+"\n"+denyTable.AsBuffer().String(), check.Equals, tableToString(resp))
}

// TestNoDeniedAccesses tests the output is correct when there are no denied access facts.
func (s *AccessTestSuite) TestNoDeniedAccesses(c *check.C) {
	access := NodeAccessRequest{}
	ctx := context.TODO()
	resp, err := QueryNodeAccess(ctx, s.emptyDenies, access)
	c.Assert(err, check.IsNil)
	accessTable := asciitable.MakeTable([]string{"User", "Login", "Node", "Allowing Roles"})
	accessTestOutput := [][]string{
		{"bob", "admin", "prod.example.com", "admin"},
		{"bob", "admin", "secret.example.com", "admin"},
		{"bob", "admin", "test.example.com", "admin"},
		{"bob", "bob", "prod.example.com", "admin"},
		{"bob", "bob", "secret.example.com", "admin"},
		{"bob", "bob", "test.example.com", "admin"},
		{"bob", "root", "prod.example.com", "admin"},
		{"bob", "root", "secret.example.com", "admin"},
		{"bob", "root", "test.example.com", "admin"},
		{"bob", "ubuntu", "prod.example.com", "admin"},
		{"bob", "ubuntu", "secret.example.com", "admin"},
		{"bob", "ubuntu", "test.example.com", "admin"},
	}
	for _, row := range accessTestOutput {
		accessTable.AddRow(row)
	}
	c.Assert(accessTable.AsBuffer().String()+"\n"+denyNullString, check.Equals, tableToString(resp))
}

// TestEmptyResults tests the output is correct when there are no facts.
func (s *AccessTestSuite) TestEmptyResults(c *check.C) {
	// No results
	ctx := context.TODO()
	access := NodeAccessRequest{}
	resp, err := QueryNodeAccess(ctx, s.empty, access)
	c.Assert(err, check.IsNil)
	c.Assert(accessNullString+"\n"+denyNullString, check.Equals, tableToString(resp))
}

// TestFiltering checks if all the deduced access facts are correct.
func (s *AccessTestSuite) TestFiltering(c *check.C) {
	ctx := context.TODO()
	access := NodeAccessRequest{Username: "julia", Login: "auditor", Node: "secret.example.com"}
	resp, err := QueryNodeAccess(ctx, s.client, access)
	c.Assert(err, check.IsNil)
	accessTable := asciitable.MakeTable([]string{"User", "Login", "Node", "Allowing Roles"})
	denyTable := asciitable.MakeTable([]string{"User", "Logins", "Node", "Denying Role"})
	accessTestOutput := [][]string{
		{"julia", "auditor", "secret.example.com", "auditor"},
	}
	for _, row := range accessTestOutput {
		accessTable.AddRow(row)
	}
	c.Assert(accessTable.AsBuffer().String()+"\n"+denyNullString, check.Equals, tableToString(resp))

	access = NodeAccessRequest{Username: "julia", Login: "julia", Node: "secret.example.com"}
	resp, err = QueryNodeAccess(ctx, s.client, access)
	c.Assert(err, check.IsNil)
	denyTestOutput := [][]string{
		{"julia", "julia", types.Wildcard, "auditor"},
	}
	for _, row := range denyTestOutput {
		denyTable.AddRow(row)
	}
	c.Assert(accessNullString+"\n"+denyTable.AsBuffer().String(), check.Equals, tableToString(resp))

	access = NodeAccessRequest{Login: "joe"}
	resp, err = QueryNodeAccess(ctx, s.client, access)
	c.Assert(err, check.IsNil)
	accessTable = asciitable.MakeTable([]string{"User", "Login", "Node", "Allowing Roles"})
	denyTable = asciitable.MakeTable([]string{"User", "Logins", "Node", "Denying Role"})
	accessTestOutput = [][]string{
		{"joe", "joe", "secret.example.com", "lister"},
	}
	denyTestOutput = [][]string{
		{"joe", "joe", "prod.example.com", "lister"},
		{"joe", "joe", "test.example.com", "lister"},
	}
	for _, row := range accessTestOutput {
		accessTable.AddRow(row)
	}
	for _, row := range denyTestOutput {
		denyTable.AddRow(row)
	}
	c.Assert(accessTable.AsBuffer().String()+"\n"+denyTable.AsBuffer().String(), check.Equals, tableToString(resp))

	access = NodeAccessRequest{Node: "test.example.com"}
	resp, err = QueryNodeAccess(ctx, s.client, access)
	c.Assert(err, check.IsNil)
	accessTable = asciitable.MakeTable([]string{"User", "Login", "Node", "Allowing Roles"})
	denyTable = asciitable.MakeTable([]string{"User", "Logins", "Node", "Denying Role"})
	accessTestOutput = [][]string{
		{"bob", "bob", "test.example.com", "admin, dev"},
		{"bob", "dev", "test.example.com", "dev"},
		{"bob", "root", "test.example.com", "admin"},
		{"bob", "ubuntu", "test.example.com", "admin, dev"},
		{"julia", "auditor", "test.example.com", "auditor"},
	}
	denyTestOutput = [][]string{
		{"bob", "admin", types.Wildcard, "dev"},
		{"joe", "admin", types.Wildcard, "dev"},
		{"joe", "dev, joe, lister", "test.example.com", "lister"},
		{"julia", "julia", types.Wildcard, "auditor"},
		{"rui", "rui", types.Wildcard, "intern"},
	}
	for _, row := range accessTestOutput {
		accessTable.AddRow(row)
	}
	for _, row := range denyTestOutput {
		denyTable.AddRow(row)
	}
	c.Assert(accessTable.AsBuffer().String()+"\n"+denyTable.AsBuffer().String(), check.Equals, tableToString(resp))

	access = NodeAccessRequest{Username: "joe"}
	resp, err = QueryNodeAccess(ctx, s.client, access)
	c.Assert(err, check.IsNil)
	accessTable = asciitable.MakeTable([]string{"User", "Login", "Node", "Allowing Roles"})
	denyTable = asciitable.MakeTable([]string{"User", "Logins", "Node", "Denying Role"})
	accessTestOutput = [][]string{
		{"joe", "joe", "secret.example.com", "lister"},
		{"joe", "lister", "secret.example.com", "lister"},
	}
	denyTestOutput = [][]string{
		{"joe", "admin", types.Wildcard, "dev"},
		{"joe", "dev, joe, lister", "prod.example.com", "lister"},
		{"joe", "dev, joe, lister", "test.example.com", "lister"},
	}
	for _, row := range accessTestOutput {
		accessTable.AddRow(row)
	}
	for _, row := range denyTestOutput {
		denyTable.AddRow(row)
	}
	c.Assert(accessTable.AsBuffer().String()+"\n"+denyTable.AsBuffer().String(), check.Equals, tableToString(resp))
}

// TestMappings checks if all required string values are mapped to integer hashes.
func (s *AccessTestSuite) TestMappings(c *check.C) {
	resp := NodeAccessResponse{Facts{}, Facts{}, make(map[string]uint32), make(map[uint32]string)}
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

// TestPredicates checks if all required predicates are created correctly with the right hashes.
func (s *AccessTestSuite) TestPredicates(c *check.C) {
	// Test user predicates
	resp := NodeAccessResponse{Facts{}, Facts{}, make(map[string]uint32), make(map[uint32]string)}
	resp.createUserPredicates(s.testUser, "")
	resp.createRolePredicates(s.testRole)
	resp.createNodePredicates(s.testNode)
	roleCountMap := make(map[string]bool)
	factsMap := generatePredicateMap(resp.facts)
	for _, pred := range factsMap[Facts_HasRole.String()] {
		require.Equal(c, resp.reverseMappings[pred.Atoms[0]], s.testUser.GetName())
		require.Contains(c, s.testUser.GetRoles(), resp.reverseMappings[pred.Atoms[1]])
		roleCountMap[resp.reverseMappings[pred.Atoms[1]]] = true
	}
	require.Equal(c, len(s.testUser.GetRoles()), len(roleCountMap))

	traitCountMap := make(map[string]bool)
	for _, pred := range factsMap[Facts_HasTrait.String()] {
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
	allLogins = append(allLogins, "")
	for _, pred := range append(factsMap[Facts_RoleAllowsLogin.String()], factsMap[Facts_RoleDeniesLogin.String()]...) {
		require.Equal(c, resp.reverseMappings[pred.Atoms[0]], s.testRole.GetName())
		require.Contains(c, allLogins, resp.reverseMappings[pred.Atoms[1]])
		loginCountMap[resp.reverseMappings[pred.Atoms[1]]] = true
	}
	require.Equal(c, 2, len(loginCountMap))

	// Test role labels
	allLabels := make(map[string][]string)
	for key, values := range s.testRole.GetNodeLabels(types.Allow) {
		for _, value := range values {
			allLabels[key] = append(allLabels[key], value)
		}
	}
	for _, pred := range factsMap[Facts_RoleAllowsNodeLabel.String()] {
		require.Contains(c, allLabels[resp.reverseMappings[pred.Atoms[1]]], resp.reverseMappings[pred.Atoms[2]])
	}
	for _, pred := range factsMap[Facts_RoleDeniesNodeLabel.String()] {
		require.Contains(c, allLabels[resp.reverseMappings[pred.Atoms[1]]], resp.reverseMappings[pred.Atoms[2]])
	}

	// Test node labels
	for _, pred := range factsMap[Facts_NodeHasLabel.String()] {
		require.Equal(c, s.testNode.GetAllLabels()[resp.reverseMappings[pred.Atoms[1]]], resp.reverseMappings[pred.Atoms[2]])
	}
}
