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
	"sort"
	"strings"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/trace"
)

// Predicate struct defines a datalog fact with a variable number of atoms.
type Predicate struct {
	Atoms []uint32
}

// EDB (extensional database) types holds the already known facts.
type EDB map[string][]Predicate

// IDB (intensional database) type holds the interpreted facts from rules.
type IDB map[string][]Predicate

// NodeAccessRequest defines a request for access for a specific user, login, and node.
type NodeAccessRequest struct {
	Username  string
	Login     string
	Node      string
	Namespace string
}

// AccessResponse defines all interpreted facts from rules.
type AccessResponse struct {
	Accesses        IDB
	facts           EDB
	mappings        map[string]uint32
	reverseMappings map[uint32]string
}

const (
	loginTraitHash = 0
	keyJoin        = "_"

	hasRole             = "has_role"
	hasTrait            = "has_trait"
	roleAllowsLogin     = "role_allows_login"
	roleDeniesLogin     = "role_denies_login"
	roleAllowsNodeLabel = "role_allows_node_label"
	roleDeniesNodeLabel = "role_denies_node_label"
	nodeHasLabel        = "node_has_label"

	accesses     = "accesses"
	denyAccesses = "deny_accesses"
	denyLogins   = "deny_logins"

	denyNullString   = "No denied accesses found.\n"
	accessNullString = "No accesses found.\n"

	userIndex  = 0
	loginIndex = 1
	nodeIndex  = 2
	roleIndex  = 3
)

// QueryAccess returns a list of accesses to Teleport.
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
	resp.Accesses = cleanOutput(resp.Accesses, resp.reverseMappings)
	resp.Accesses = filterByLogin(resp.Accesses, resp.reverseMappings, c.Login)
	return &resp, nil
}

func (r *AccessResponse) addPredicate(key string, atoms ...interface{}) {
	var mappedAtoms []uint32
	for _, atom := range atoms {
		id, ok := atom.(int)
		if ok {
			mappedAtoms = append(mappedAtoms, uint32(id))
			continue
		}
		val, ok := atom.(string)
		if !ok {
			// Invalid type, we can just skip this predicate.
			return
		}
		loginNum := r.mappings[val]
		if val == teleport.TraitInternalLoginsVariable {
			loginNum = loginTraitHash
		}
		mappedAtoms = append(mappedAtoms, loginNum)
	}
	r.facts[key] = append(r.facts[key], Predicate{mappedAtoms})
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
		r.addPredicate(hasRole, user.GetName(), role)
	}
	for _, trait := range user.GetTraits()[teleport.TraitLogins] {
		r.addPredicate(hasTrait, user.GetName(), loginTraitHash, trait)
	}
}

func (r *AccessResponse) createRolePredicates(role types.Role) {
	r.createRoleMapping(role)
	for _, login := range role.GetLogins(types.Allow) {
		r.addPredicate(roleAllowsLogin, role.GetName(), login)
	}
	for _, login := range role.GetLogins(types.Deny) {
		r.addPredicate(roleDeniesLogin, role.GetName(), login)
	}
	for key, values := range role.GetNodeLabels(types.Allow) {
		for _, value := range values {
			r.addPredicate(roleAllowsNodeLabel, role.GetName(), key, value)
		}
	}
	for key, values := range role.GetNodeLabels(types.Deny) {
		for _, value := range values {
			r.addPredicate(roleDeniesNodeLabel, role.GetName(), key, value)
		}
	}
}

func (r *AccessResponse) createNodePredicates(node types.Server) {
	r.createNodeMapping(node)
	for key, value := range node.GetAllLabels() {
		r.addPredicate(nodeHasLabel, node.GetHostname(), key, value)
	}
	r.addPredicate(nodeHasLabel, node.GetHostname(), types.Wildcard, types.Wildcard)
}

func (r *AccessResponse) generateAtomStrings(key string) [][]string {
	var ret [][]string
	for _, pred := range r.Accesses[key] {
		var atoms []string
		for _, atom := range pred.Atoms {
			atoms = append(atoms, r.reverseMappings[atom])
		}
		ret = append(ret, atoms)
	}
	return ret
}

// BuildStringOutput creates the UI for displaying access responses.
func (r *AccessResponse) BuildStringOutput() string {
	accessTable := asciitable.MakeTable([]string{"User", "Login", "Node", "Allowing Roles"})
	allowingRoles := make(map[string][]string)
	accessStrings := r.generateAtomStrings(accesses)
	for _, atoms := range accessStrings {
		key := createDupMapKey(atoms[:roleIndex])
		allowingRoles[key] = append(
			allowingRoles[key],
			atoms[roleIndex],
		)
	}
	for _, key := range sortKeys(allowingRoles) {
		sort.Strings(allowingRoles[key])
		accessTable.AddRow(append(strings.Split(key, keyJoin), strings.Join(allowingRoles[key], ", ")))
	}

	denyTable := asciitable.MakeTable([]string{"User", "Logins", "Node", "Denying Role"})
	deniedLogins := make(map[string][]string)
	denyStrings := r.generateAtomStrings(denyAccesses)
	for _, atoms := range r.generateAtomStrings(denyLogins) {
		atomList := append(atoms[:nodeIndex], atoms[loginIndex:]...)
		atomList[nodeIndex] = types.Wildcard
		denyStrings = append(denyStrings, atomList)
	}
	for _, atoms := range denyStrings {
		key := createDupMapKey(remove(atoms, loginIndex))
		deniedLogins[key] = append(
			deniedLogins[key],
			atoms[loginIndex],
		)
	}
	for _, key := range sortKeys(deniedLogins) {
		atomList := strings.Split(key, keyJoin)
		atomList = append(atomList[:nodeIndex], atomList[loginIndex:]...)
		sort.Strings(deniedLogins[key])
		atomList[loginIndex] = strings.Join(deniedLogins[key], ", ")
		denyTable.AddRow(atomList)
	}

	denyOutputString := generateTableString(len(denyStrings) == 0 && len(r.Accesses[denyLogins]) == 0, denyTable, denyNullString)
	accessOutputString := generateTableString(len(accessStrings) == 0, accessTable, accessNullString)
	return accessOutputString + "\n" + denyOutputString
}

func cleanOutput(accesses IDB, reverse map[uint32]string) IDB {
	ret := make(IDB)
	for key, preds := range accesses {
		var cleaned []Predicate
		for _, pred := range preds {
			if _, exists := reverse[pred.Atoms[loginIndex]]; !exists {
				continue
			}
			cleaned = append(cleaned, pred)
		}
		ret[key] = cleaned
	}
	return ret
}

func filterByLogin(accesses IDB, reverse map[uint32]string, login string) IDB {
	if login == "" {
		return accesses
	}
	ret := make(IDB)
	for key, preds := range accesses {
		var filtered []Predicate
		for _, pred := range preds {
			if reverse[pred.Atoms[loginIndex]] != login {
				continue
			}
			filtered = append(filtered, pred)
		}
		ret[key] = filtered
	}
	return ret
}

func remove(atoms []string, idx int) []string {
	var ret []string
	for i, a := range atoms {
		if i == idx {
			continue
		}
		ret = append(ret, a)
	}
	return ret
}

func createDupMapKey(atoms []string) string {
	return strings.Join(atoms, keyJoin)
}

func generateTableString(condition bool, table asciitable.Table, nullString string) string {
	if condition {
		return nullString
	}
	return table.AsBuffer().String()
}

func sortKeys(mapping map[string][]string) []string {
	var keys []string
	for k := range mapping {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func hash(s string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(s))
	return h.Sum32()
}
