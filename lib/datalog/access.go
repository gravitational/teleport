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

// #cgo LDFLAGS: -Lroletester/target/release -lrole_tester
// #include <stdio.h>
// extern char * process_access(const char *str);
import "C"
import (
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"strings"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/trace"
)

// Predicate struct defines a datalog fact with a variable number of atoms
type Predicate struct {
	Atoms []uint32
}

// EDB (extensional database) types holds the already known facts
type EDB map[string][]Predicate

// IDB (intensional database) type holds the interpreted facts from rules
type IDB map[string][]Predicate

// Access defines the query interface
type Access interface {
	QueryAccess(auth.ClientI) (*AccessResponse, error)
}

// NodeAccessRequest defines a request for access for a specific user, login, and node
type NodeAccessRequest struct {
	Username  string
	Login     string
	Node      string
	Namespace string
}

// AccessResponse defines all interpreted facts from rules
type AccessResponse struct {
	Accesses        IDB
	facts           EDB
	mappings        map[string]uint32
	reverseMappings map[uint32]string
}

const (
	loginTraitHash = 0
	keyJoin        = "_"

	// Input
	hasRole             = "has_role"
	hasTrait            = "has_trait"
	roleAllowsLogin     = "role_allows_login"
	roleDeniesLogin     = "role_denies_login"
	roleAllowsNodeLabel = "role_allows_node_label"
	roleDeniesNodeLabel = "role_denies_node_label"
	nodeHasLabel        = "node_has_label"

	// Output
	accesses   = "accesses"
	allowRoles = "allow_roles"
	denyRoles  = "deny_roles"

	// No results
	denyNullString   = "No denied accesses.\n"
	accessNullString = "No accesses.\n"
)

// QueryAccess returns a list of accesses to Teleport
func (c *NodeAccessRequest) QueryAccess(client auth.ClientI) (*AccessResponse, error) {
	resp := AccessResponse{make(IDB), make(EDB), make(map[string]uint32), make(map[uint32]string)}
	ctx := context.TODO()
	resp.addToMap(types.Wildcard)

	if c.Username != "" {
		u, err := client.GetUser(c.Username, false)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		resp.createUserPredicates(u, c.Login)
	} else {
		us, err := client.GetUsers(false)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		for _, u := range us {
			resp.createUserPredicates(u, c.Login)
		}
	}

	rs, err := client.GetRoles(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for _, r := range rs {
		resp.createRolePredicates(r)
	}

	ns, err := client.GetNodes(ctx, c.Namespace)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for _, n := range ns {
		if len(c.Node) == 0 || n.GetHostname() == c.Node {
			resp.createNodePredicates(n)
		}
	}

	b, err := json.Marshal(resp.facts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = json.Unmarshal([]byte(C.GoString(C.process_access(C.CString(string(b))))), &resp.Accesses)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	for k := range resp.Accesses {
		resp.Accesses[k] = filterByLogin(resp.Accesses[k], resp.reverseMappings, c.Login)
	}
	return &resp, nil
}

func hash(s string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(s))
	return h.Sum32()
}

func (r *AccessResponse) addToMap(value string) {
	if _, exists := r.mappings[value]; exists {
		return
	}
	h := hash(value)
	for _, exists := r.reverseMappings[h]; exists; {
		h = hash(fmt.Sprint(h))
	}
	r.reverseMappings[h] = value
	r.mappings[value] = h
}

func (r *AccessResponse) createUserMapping(user types.User) {
	r.addToMap(user.GetName())
	for _, login := range user.GetTraits()[teleport.TraitLogins] {
		r.addToMap(login)
	}
	for _, role := range user.GetRoles() {
		r.addToMap(role)
	}
}

func (r *AccessResponse) createRoleMapping(role types.Role) {
	r.addToMap(role.GetName())
	for _, login := range append(role.GetLogins(types.Allow), role.GetLogins(types.Deny)...) {
		if login == teleport.TraitInternalLoginsVariable {
			continue
		}
		r.addToMap(login)
	}
	for key, values := range role.GetNodeLabels(types.Allow) {
		r.addToMap(key)
		for _, value := range values {
			r.addToMap(value)
		}
	}
	for key, values := range role.GetNodeLabels(types.Deny) {
		r.addToMap(key)
		for _, value := range values {
			r.addToMap(value)
		}
	}
}

func (r *AccessResponse) createNodeMapping(node types.Server) {
	r.addToMap(node.GetHostname())
	for key, value := range node.GetAllLabels() {
		r.addToMap(key)
		r.addToMap(value)
	}
}

func (r *AccessResponse) createUserPredicates(user types.User, login string) {
	r.createUserMapping(user)
	for _, role := range user.GetRoles() {
		r.facts[hasRole] = append(r.facts[hasRole], Predicate{
			[]uint32{r.mappings[user.GetName()], r.mappings[role]},
		})
	}
	for _, trait := range user.GetTraits()[teleport.TraitLogins] {
		r.facts[hasTrait] = append(r.facts[hasTrait], Predicate{
			[]uint32{r.mappings[user.GetName()], loginTraitHash, r.mappings[trait]},
		})
	}
}

func (r *AccessResponse) createRolePredicates(role types.Role) {
	r.createRoleMapping(role)
	for _, login := range role.GetLogins(types.Allow) {
		if _, exists := r.mappings[login]; !exists {
			continue
		}
		r.facts[roleAllowsLogin] = append(r.facts[roleAllowsLogin], Predicate{
			[]uint32{r.mappings[role.GetName()], r.mappings[login]},
		})
	}
	for _, login := range role.GetLogins(types.Deny) {
		if _, exists := r.mappings[login]; !exists {
			continue
		}
		r.facts[roleDeniesLogin] = append(r.facts[roleDeniesLogin], Predicate{
			[]uint32{r.mappings[role.GetName()], r.mappings[login]},
		})
	}
	for key, values := range role.GetNodeLabels(types.Allow) {
		for _, value := range values {
			r.facts[roleAllowsNodeLabel] = append(r.facts[roleAllowsNodeLabel], Predicate{
				[]uint32{r.mappings[role.GetName()], r.mappings[key], r.mappings[value]},
			})
		}
	}
	for key, values := range role.GetNodeLabels(types.Deny) {
		for _, value := range values {
			r.facts[roleDeniesNodeLabel] = append(r.facts[roleDeniesNodeLabel], Predicate{
				[]uint32{r.mappings[role.GetName()], r.mappings[key], r.mappings[value]},
			})
		}
	}
}

func (r *AccessResponse) createNodePredicates(node types.Server) {
	r.createNodeMapping(node)
	for key, value := range node.GetAllLabels() {
		r.facts[nodeHasLabel] = append(r.facts[nodeHasLabel], Predicate{
			[]uint32{r.mappings[node.GetHostname()], r.mappings[key], r.mappings[value]},
		})
	}
	r.facts[nodeHasLabel] = append(r.facts[nodeHasLabel], Predicate{
		[]uint32{r.mappings[node.GetHostname()], r.mappings[types.Wildcard], r.mappings[types.Wildcard]},
	})
}

func filterByLogin(accesses []Predicate, mappings map[uint32]string, login string) []Predicate {
	if login == "" {
		return accesses
	}
	ret := make([]Predicate, 0)
	for _, pred := range accesses {
		if mappings[pred.Atoms[1]] != login {
			continue
		}
		ret = append(ret, pred)
	}
	return ret
}

func createUserLoginKey(atoms []uint32, reverseMappings map[uint32]string) string {
	return reverseMappings[atoms[0]] + keyJoin + reverseMappings[atoms[1]]
}

func generateTableString(condition bool, table asciitable.Table, nullString string) string {
	if condition {
		return table.AsBuffer().String()
	}
	return nullString
}

func generateAtomStrings(atoms []uint32, reverseMappings map[uint32]string) []string {
	var ret []string
	for _, atom := range atoms {
		ret = append(ret, reverseMappings[atom])
	}
	return ret
}

// BuildStringOutput creates the UI for displaying access responses
func (r *AccessResponse) BuildStringOutput() string {
	accessTable := asciitable.MakeTable([]string{"User", "Login", "Node", "Allowing Roles"})
	allowingRoles := make(map[string][]string)
	for _, pred := range r.Accesses[allowRoles] {
		allowingRoles[createUserLoginKey(pred.Atoms, r.reverseMappings)] = append(
			allowingRoles[createUserLoginKey(pred.Atoms, r.reverseMappings)],
			r.reverseMappings[pred.Atoms[3]],
		)
	}
	for _, pred := range r.Accesses[accesses] {
		allowRoles := strings.Join(allowingRoles[createUserLoginKey(pred.Atoms, r.reverseMappings)], ", ")
		accessTable.AddRow(append(generateAtomStrings(pred.Atoms, r.reverseMappings), allowRoles))
	}

	denyTable := asciitable.MakeTable([]string{"User", "Login", "Node", "Denying Role"})
	for _, pred := range r.Accesses[denyRoles] {
		denyTable.AddRow(generateAtomStrings(pred.Atoms, r.reverseMappings))
	}
	denyOutputString := generateTableString(len(r.Accesses[denyRoles]) > 0, denyTable, denyNullString)
	accessOutputString := generateTableString(len(r.Accesses[accesses]) > 0, accessTable, accessNullString)
	return accessOutputString + "\n" + denyOutputString
}
