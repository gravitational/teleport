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

// #cgo linux,386 LDFLAGS: -L${SRCDIR}/../../target/i686-unknown-linux-gnu/release
// #cgo linux,amd64 LDFLAGS: -L${SRCDIR}/../../target/x86_64-unknown-linux-gnu/release
// #cgo linux,arm LDFLAGS: -L${SRCDIR}/../../target/arm-unknown-linux-gnueabihf/release
// #cgo linux,arm64 LDFLAGS: -L${SRCDIR}/../../target/aarch64-unknown-linux-gnu/release
// #cgo darwin,amd64 LDFLAGS: -L${SRCDIR}/../../target/x86_64-apple-darwin/release
// #cgo darwin,arm64 LDFLAGS: -L${SRCDIR}/../../target/aarch64-apple-darwin/release
// #cgo LDFLAGS: -lrole_tester -ldl -lm
// #include <stdio.h>
// #include <stdlib.h>
// typedef struct output output_t;
// extern output_t *process_access(unsigned char *input, size_t input_len);
// extern unsigned char *output_access(output_t *);
// extern size_t output_length(output_t *);
// extern int output_error(output_t *);
// extern void drop_output_struct(output_t *output);
import "C"
import (
	"context"
	"fmt"
	"hash/fnv"
	"sort"
	"strings"
	"unsafe"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth"

	"github.com/gogo/protobuf/proto"
	"github.com/gravitational/trace"
)

// NodeAccessRequest defines a request for access for a specific user, login, and node.
type NodeAccessRequest struct {
	Username  string
	Login     string
	Node      string
	Namespace string
}

// NodeAccessResponse defines all interpreted facts from the input facts.
type NodeAccessResponse struct {
	// Generated output from Rust.
	Accesses Facts
	// Generated input for Rust.
	facts Facts
	// Mappings to convert from hash values to readable strings.
	// Note: Improves performance compared to using strings. Avoids complications within the Rust code if done this way.
	// Could be switched to strings in the future if needed.
	mappings        map[string]uint32
	reverseMappings map[uint32]string
}

const (
	// Constant for Rust datalog parsing on login traits.
	loginTraitHash = 0
	// Used for final table key generation.
	keyJoin = "_"
	// Indices used for final table generation.
	//nolint:deadcode,varcheck
	userIndex  = 0
	loginIndex = 1
	nodeIndex  = 2
	roleIndex  = 3
)

// QueryNodeAccess returns a list of accesses to Teleport.
func QueryNodeAccess(ctx context.Context, client auth.ClientI, req NodeAccessRequest) (*NodeAccessResponse, error) {
	resp := NodeAccessResponse{Facts{}, Facts{}, make(map[string]uint32), make(map[uint32]string)}
	resp.addToMap(types.Wildcard)

	// Since we can filter on specific usernames, this conditional will only retrieve and build the facts for the required user(s).
	if req.Username != "" {
		u, err := client.GetUser(req.Username, false)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		resp.createUserPredicates(u, req.Login)
	} else {
		us, err := client.GetUsers(false)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		for _, u := range us {
			resp.createUserPredicates(u, req.Login)
		}
	}

	rs, err := client.GetRoles(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for _, r := range rs {
		resp.createRolePredicates(r)
	}

	ns, err := client.GetNodes(ctx, req.Namespace)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for _, n := range ns {
		if len(req.Node) == 0 || n.GetHostname() == req.Node {
			resp.createNodePredicates(n)
		}
	}

	b, err := resp.facts.Marshal()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Create the byte buffer pointers for input and output
	ptr := (*C.uchar)(C.CBytes(b))
	defer C.free(unsafe.Pointer(ptr))

	output := C.process_access(ptr, C.size_t(len(b)))
	defer C.drop_output_struct(output)

	res := C.output_access(output)
	statusCode := C.output_error(output)
	outputLength := C.output_length(output)

	// If statusCode != 0, then there was an error. We return the error string.
	if int(statusCode) != 0 {
		return nil, trace.BadParameter(C.GoStringN((*C.char)(unsafe.Pointer(res)), C.int(outputLength)))
	}

	err = proto.Unmarshal(C.GoBytes(unsafe.Pointer(res), C.int(outputLength)), &resp.Accesses)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	resp.Accesses = cleanOutput(resp.Accesses, resp.reverseMappings)
	resp.Accesses = filterByLogin(resp.Accesses, resp.reverseMappings, req.Login)
	return &resp, nil
}

func (r *NodeAccessResponse) addPredicate(key Facts_PredicateType, atoms []uint32) {
	r.facts.Predicates = append(r.facts.Predicates, &Facts_Predicate{Atoms: atoms, Name: key})
}

func (r *NodeAccessResponse) addToMap(value string) {
	// TraitInternalLoginsVariable represents a login string that isn't the literal login that is used.
	// Rather, this string is used to identify the specific trait that the user has, which is "logins".
	// We assign the loginTraitHash constant so that the datalog interpreter knows which trait this is.
	// The datalog interpreter is set up this way so we can reuse the HasTrait fact for other traits in the future.
	if value == teleport.TraitInternalLoginsVariable {
		return
	}
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

func (r *NodeAccessResponse) createUserMapping(user types.User) {
	r.addToMap(user.GetName())
	for _, login := range user.GetTraits()[teleport.TraitLogins] {
		r.addToMap(login)
	}
	for _, role := range user.GetRoles() {
		r.addToMap(role)
	}
}

func (r *NodeAccessResponse) createRoleMapping(role types.Role) {
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

func (r *NodeAccessResponse) createNodeMapping(node types.Server) {
	r.addToMap(node.GetHostname())
	for key, value := range node.GetAllLabels() {
		r.addToMap(key)
		r.addToMap(value)
	}
}

func (r *NodeAccessResponse) createUserPredicates(user types.User, login string) {
	r.createUserMapping(user)
	for _, role := range user.GetRoles() {
		r.addPredicate(Facts_HasRole, []uint32{r.mappings[user.GetName()], r.mappings[role]})
	}
	for _, trait := range user.GetTraits()[teleport.TraitLogins] {
		r.addPredicate(Facts_HasTrait, []uint32{r.mappings[user.GetName()], loginTraitHash, r.mappings[trait]})
	}
}

func (r *NodeAccessResponse) createRolePredicates(role types.Role) {
	r.createRoleMapping(role)
	for _, login := range role.GetLogins(types.Allow) {
		r.addPredicate(Facts_RoleAllowsLogin, []uint32{r.mappings[role.GetName()], r.mappings[login]})
	}
	for _, login := range role.GetLogins(types.Deny) {
		r.addPredicate(Facts_RoleDeniesLogin, []uint32{r.mappings[role.GetName()], r.mappings[login]})
	}
	for key, values := range role.GetNodeLabels(types.Allow) {
		for _, value := range values {
			r.addPredicate(Facts_RoleAllowsNodeLabel, []uint32{r.mappings[role.GetName()], r.mappings[key], r.mappings[value]})
		}
	}
	for key, values := range role.GetNodeLabels(types.Deny) {
		for _, value := range values {
			r.addPredicate(Facts_RoleDeniesNodeLabel, []uint32{r.mappings[role.GetName()], r.mappings[key], r.mappings[value]})
		}
	}
}

func (r *NodeAccessResponse) createNodePredicates(node types.Server) {
	r.createNodeMapping(node)
	for key, value := range node.GetAllLabels() {
		r.addPredicate(Facts_NodeHasLabel, []uint32{r.mappings[node.GetHostname()], r.mappings[key], r.mappings[value]})
	}
	// This needs to be added to account for any roles that allow to all nodes with the wildcard label.
	// Serves as a bandaid fix before regex implementation.
	r.addPredicate(Facts_NodeHasLabel, []uint32{r.mappings[node.GetHostname()], r.mappings[types.Wildcard], r.mappings[types.Wildcard]})
}

// ToTable builds the data structure used to output accesses
func (r *NodeAccessResponse) ToTable() (asciitable.Table, asciitable.Table, int, int) {
	accessMap := generatePredicateMap(r.Accesses)
	accessTable := asciitable.MakeTable([]string{"User", "Login", "Node", "Allowing Roles"})
	allowingRoles := make(map[string][]string)
	accessStrings := generateAtomStrings(accessMap, Facts_HasAccess.String(), r.reverseMappings)
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
	denyStrings := generateAtomStrings(accessMap, Facts_DenyAccess.String(), r.reverseMappings)
	for _, atoms := range generateAtomStrings(accessMap, Facts_DenyLogins.String(), r.reverseMappings) {
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
	return accessTable, denyTable, len(accessStrings), len(denyStrings) + len(accessMap[Facts_DenyLogins.String()])
}

func cleanOutput(accesses Facts, reverse map[uint32]string) Facts {
	ret := Facts{}
	for _, pred := range accesses.Predicates {
		if _, exists := reverse[pred.Atoms[loginIndex]]; !exists {
			continue
		}
		ret.Predicates = append(ret.Predicates, pred)
	}
	return ret
}

func filterByLogin(accesses Facts, reverse map[uint32]string, login string) Facts {
	if login == "" {
		return accesses
	}
	ret := Facts{}
	for _, pred := range accesses.Predicates {
		if reverse[pred.Atoms[loginIndex]] != login {
			continue
		}
		ret.Predicates = append(ret.Predicates, pred)
	}
	return ret
}

func generatePredicateMap(accesses Facts) map[string][]*Facts_Predicate {
	ret := make(map[string][]*Facts_Predicate)
	for _, pred := range accesses.Predicates {
		ret[pred.Name.String()] = append(ret[pred.Name.String()], pred)
	}
	return ret
}

func generateAtomStrings(accesses map[string][]*Facts_Predicate, key string, reverse map[uint32]string) [][]string {
	var ret [][]string
	for _, pred := range accesses[key] {
		var atoms []string
		for _, atom := range pred.Atoms {
			atoms = append(atoms, reverse[atom])
		}
		ret = append(ret, atoms)
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
