/*
Copyright 2020 Gravitational, Inc.

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

package common

//#cgo LDFLAGS: -Lroletester/target/release -lrole_tester
//#include <stdio.h>
//extern char * process_access(const char *str);
//extern void test();
import "C"
import (
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"log"
	"strings"

	"github.com/gravitational/kingpin"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/trace"
)

// Predicate struct defines a datalog fact with a variable number of atoms
type Predicate struct {
	Atoms []uint32
}

// EDB type represents the known facts
type EDB map[string][]Predicate

// IDB type represents the interpreted facts
type IDB map[string][]Predicate

const (
	loginTraitKey      = "logins"
	loginTraitHash     = 0
	loginTraitTemplate = "{{internal.logins}}"
	wildCard           = "*"

	hasRole             = "has_role"
	hasTrait            = "has_trait"
	hasLoginTrait       = "has_login_trait"
	roleAllowsLogin     = "role_allows_login"
	roleDeniesLogin     = "role_denies_login"
	roleAllowsNodeLabel = "role_allows_node_label"
	roleDeniesNodeLabel = "role_denies_node_label"
	nodeHasLabel        = "node_has_label"
)

var mappings = make(map[string]uint32)
var reverseMappings = make(map[uint32]string)

// AccessCommand implements "tctl access" group of commands.
type AccessCommand struct {
	config *service.Config

	// format is the output format (text, json, or yaml)
	user      string
	login     string
	node      string
	namespace string

	// appsList implements the "tctl apps ls" subcommand.
	accessList *kingpin.CmdClause
}

// Initialize allows AppsCommand to plug itself into the CLI parser
func (c *AccessCommand) Initialize(app *kingpin.Application, config *service.Config) {
	c.config = config

	accesses := app.Command("access", "Get access information within the cluster.")
	c.accessList = accesses.Command("ls", "List all accesses within the cluster.")
	c.accessList.Flag("user", "Teleport user").Default("").StringVar(&c.user)
	c.accessList.Flag("login", "Teleport login").Default("").StringVar(&c.login)
	c.accessList.Flag("node", "Teleport node").Default("").StringVar(&c.node)
	c.accessList.Flag("namespace", "Teleport namespace").Default("default").StringVar(&c.namespace)
}

// TryRun attempts to run subcommands like "apps ls".
func (c *AccessCommand) TryRun(cmd string, client auth.ClientI) (match bool, err error) {
	switch cmd {
	case c.accessList.FullCommand():
		err = c.ListAccesses(client)
	default:
		return false, nil
	}
	return true, trace.Wrap(err)
}

// ListAccesses prints the list of accesses to Teleport
func (c *AccessCommand) ListAccesses(client auth.ClientI) error {
	facts := make(EDB)
	ctx := context.TODO()
	addToMap(wildCard)
	if c.user != "" {
		u, err := client.GetUser(c.user, false)
		if err != nil {
			return err
		}

		createUserMapping(u)
		facts = createUserPredicates(facts, u)
	} else {
		us, err := client.GetUsers(false)
		if err != nil {
			return err
		}
		us = filterUsersByLogin(us, c.login)
		for _, u := range us {
			createUserMapping(u)
			facts = createUserPredicates(facts, u)
		}
	}

	rs, err := client.GetRoles(ctx)
	if err != nil {
		return err
	}
	for _, r := range rs {
		createRoleMapping(r)
		facts = createRolePredicates(facts, r)
	}

	ns, err := client.GetNodes(ctx, c.namespace)
	if err != nil {
		return err
	}
	for _, n := range ns {
		if len(c.node) == 0 || n.GetHostname() == c.node {
			createNodeMappping(n)
			facts = createNodePredicates(facts, n)
		}
	}

	b, err := json.Marshal(facts)
	if err != nil {
		log.Fatalln(err)
	}
	idb := make(IDB)
	json.Unmarshal([]byte(C.GoString(C.process_access(C.CString(string(b))))), &idb)
	fmt.Println(buildOutput(idb))

	return nil
}

func hash(s string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(s))
	return h.Sum32()
}

func filterUsersByLogin(users []types.User, login string) []types.User {
	if len(login) == 0 {
		return users
	}
	ret := []types.User{}
	for _, u := range users {
		for _, t := range u.GetTraits()[loginTraitKey] {
			if login != t {
				continue
			}
			ret = append(ret, u)
			break
		}
	}
	return ret
}

func addToMap(value string) {
	if _, exists := mappings[value]; exists {
		return
	}
	h := hash(value)
	for _, exists := reverseMappings[h]; exists; {
		h = hash(fmt.Sprint(h))
	}
	reverseMappings[h] = value
	mappings[value] = h
}

func createUserMapping(user types.User) {
	addToMap(user.GetName())
	for _, login := range user.GetTraits()[loginTraitKey] {
		addToMap(login)
	}
	for _, role := range user.GetRoles() {
		addToMap(role)
	}
}

func createRoleMapping(role types.Role) {
	addToMap(role.GetName())
	for _, login := range append(role.GetLogins(types.Allow), role.GetLogins(types.Deny)...) {
		if login == loginTraitTemplate {
			continue
		}
		addToMap(login)
	}
	for key, values := range role.GetNodeLabels(types.Allow) {
		addToMap(key)
		for _, value := range values {
			addToMap(value)
		}
	}
	for key, values := range role.GetNodeLabels(types.Deny) {
		addToMap(key)
		for _, value := range values {
			addToMap(value)
		}
	}
}

func createNodeMappping(node types.Server) {
	addToMap(node.GetHostname())
	for key, value := range node.GetAllLabels() {
		addToMap(key)
		addToMap(value)
	}
}

func createUserPredicates(facts EDB, user types.User) EDB {
	for _, role := range user.GetRoles() {
		facts[hasRole] = append(facts[hasRole], Predicate{
			[]uint32{mappings[user.GetName()], mappings[role]},
		})
	}
	traitExists := false
	for _, trait := range user.GetTraits()[loginTraitKey] {
		facts[hasTrait] = append(facts[hasTrait], Predicate{
			[]uint32{mappings[user.GetName()], loginTraitHash, mappings[trait]},
		})
		traitExists = true
	}
	if traitExists {
		facts[hasLoginTrait] = append(facts[hasLoginTrait], Predicate{
			[]uint32{mappings[user.GetName()]},
		})
	}
	return facts
}

func createRolePredicates(facts EDB, role types.Role) EDB {
	for _, login := range role.GetLogins(types.Allow) {
		if _, exists := mappings[login]; !exists {
			continue
		}
		facts[roleAllowsLogin] = append(facts[roleAllowsLogin], Predicate{
			[]uint32{mappings[role.GetName()], mappings[login]},
		})
	}
	for _, login := range role.GetLogins(types.Deny) {
		if _, exists := mappings[login]; !exists {
			continue
		}
		facts[roleDeniesLogin] = append(facts[roleDeniesLogin], Predicate{
			[]uint32{mappings[role.GetName()], mappings[login]},
		})
	}
	for key, values := range role.GetNodeLabels(types.Allow) {
		for _, value := range values {
			facts[roleAllowsNodeLabel] = append(facts[roleAllowsNodeLabel], Predicate{
				[]uint32{mappings[role.GetName()], mappings[key], mappings[value]},
			})
		}
	}
	for key, values := range role.GetNodeLabels(types.Deny) {
		for _, value := range values {
			facts[roleDeniesNodeLabel] = append(facts[roleDeniesNodeLabel], Predicate{
				[]uint32{mappings[role.GetName()], mappings[key], mappings[value]},
			})
		}
	}
	return facts
}

func createNodePredicates(facts EDB, node types.Server) EDB {
	for key, value := range node.GetAllLabels() {
		facts[nodeHasLabel] = append(facts[nodeHasLabel], Predicate{
			[]uint32{mappings[node.GetHostname()], mappings[key], mappings[value]},
		})
	}
	facts[nodeHasLabel] = append(facts[nodeHasLabel], Predicate{
		[]uint32{mappings[node.GetHostname()], mappings[wildCard], mappings[wildCard]},
	})
	return facts
}

func buildOutput(idb IDB) string {
	var sb strings.Builder
	sb.WriteString("Accesses:\n")
	for _, pred := range idb["accesses"] {
		sb.WriteString(reverseMappings[pred.Atoms[0]] + " ")
		sb.WriteString(reverseMappings[pred.Atoms[1]] + " ")
		sb.WriteString(reverseMappings[pred.Atoms[2]] + "\n")
	}
	sb.WriteString("\nAllowing roles:\n")
	for _, pred := range idb["allow_roles"] {
		sb.WriteString(reverseMappings[pred.Atoms[0]] + " ")
		sb.WriteString(reverseMappings[pred.Atoms[1]] + " ")
		sb.WriteString(reverseMappings[pred.Atoms[2]] + "\n")
	}
	sb.WriteString("\nDenying roles:\n")
	for _, pred := range idb["deny_roles"] {
		sb.WriteString(reverseMappings[pred.Atoms[0]] + " ")
		sb.WriteString(reverseMappings[pred.Atoms[1]] + " ")
		sb.WriteString(reverseMappings[pred.Atoms[2]] + "\n")
	}
	return sb.String()
}
