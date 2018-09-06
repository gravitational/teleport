/*
Copyright 2015 Gravitational, Inc.

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

package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
	"github.com/pborman/uuid"
	. "gopkg.in/check.v1"
)

func TestRoleParsing(t *testing.T) { TestingT(t) }

type RoleSuite struct {
}

var _ = Suite(&RoleSuite{})
var _ = fmt.Printf

func (s *RoleSuite) SetUpSuite(c *C) {
	utils.InitLoggerForTests()
}

func (s *RoleSuite) TestRoleExtension(c *C) {
	type Spec struct {
		RoleSpecV2
		A string `json:"a"`
	}
	type ExtendedRole struct {
		Spec Spec `json:"spec"`
	}
	in := `{"kind": "role", "metadata": {"name": "name1"}, "spec": {"a": "b"}}`
	var role ExtendedRole
	err := utils.UnmarshalWithSchema(GetRoleSchema(V2, `"a": {"type": "string"}`), &role, []byte(in))
	c.Assert(err, IsNil)
	c.Assert(role.Spec.A, Equals, "b")

	// this is a bad type
	in = `{"kind": "role", "metadata": {"name": "name1"}, "spec": {"a": 12}}`
	err = utils.UnmarshalWithSchema(GetRoleSchema(V2, `"a": {"type": "string"}`), &role, []byte(in))
	c.Assert(err, NotNil)
}

func (s *RoleSuite) TestRoleParse(c *C) {
	testCases := []struct {
		name         string
		in           string
		role         RoleV3
		error        error
		matchMessage string
	}{
		{
			name:  "no input, should not parse",
			in:    ``,
			role:  RoleV3{},
			error: trace.BadParameter("empty input"),
		},
		{
			name:  "validation error, no name",
			in:    `{}`,
			role:  RoleV3{},
			error: trace.BadParameter("failed to validate: name: name is required"),
		},
		{
			name:  "validation error, no name",
			in:    `{"kind": "role"}`,
			role:  RoleV3{},
			error: trace.BadParameter("failed to validate: name: name is required"),
		},
		{
			name: "validation error, missing resources",
			in: `{
		      "kind": "role",
		      "version": "v3",
		      "metadata": {"name": "name1"},
		      "spec": {
                 "allow": {
                   "node_labels": {"a": "b"},
                   "namespaces": ["default"],
                   "rules": [
                     {
                       "verbs": ["read", "list"]
                     }
                   ]
                 }
		      }
		    }`,
			error:        trace.BadParameter(""),
			matchMessage: ".*missing resources.*",
		},
		{
			name: "validation error, missing verbs",
			in: `{
		      "kind": "role",
		      "version": "v3",
		      "metadata": {"name": "name1"},
		      "spec": {
                 "allow": {
                   "node_labels": {"a": "b"},
                   "namespaces": ["default"],
                   "rules": [
                     {
                       "resources": ["role"]
                     }
                   ]
                 }
		      }
		    }`,
			error:        trace.BadParameter(""),
			matchMessage: ".*missing verbs.*",
		},
		{
			name: "validation error, unsupported function in where",
			in: `{
		      "kind": "role",
		      "version": "v3",
		      "metadata": {"name": "name1"},
		      "spec": {
                 "allow": {
                   "node_labels": {"a": "b"},
                   "namespaces": ["default"],
                   "rules": [
                     {
                       "resources": ["role"],
                       "verbs": ["read", "list"],
                       "where": "containz(user.spec.traits[\"groups\"], \"prod\")"
                     }
                   ]
                 }
		      }
		    }`,
			error:        trace.BadParameter(""),
			matchMessage: ".*unsupported function: containz.*",
		},
		{
			name: "validation error, unsupported function in actions",
			in: `{
		      "kind": "role",
		      "version": "v3",
		      "metadata": {"name": "name1"},
		      "spec": {
                 "allow": {
                   "node_labels": {"a": "b"},
                   "namespaces": ["default"],
                   "rules": [
                     {
                       "resources": ["role"],
                       "verbs": ["read", "list"],
                       "where": "contains(user.spec.traits[\"groups\"], \"prod\")",
                       "actions": [
                          "zzz(\"info\", \"log entry\")"
                       ]
                     }
                   ]
                 }
		      }
		    }`,
			error:        trace.BadParameter(""),
			matchMessage: ".*unsupported function: zzz.*",
		},
		{
			name: "role with no spec still gets defaults",
			in:   `{"kind": "role", "version": "v3", "metadata": {"name": "defrole"}, "spec": {}}`,
			role: RoleV3{
				Kind:    KindRole,
				Version: V3,
				Metadata: Metadata{
					Name:      "defrole",
					Namespace: defaults.Namespace,
				},
				Spec: RoleSpecV3{
					Options: RoleOptions{
						CertificateFormat: teleport.CertificateFormatStandard,
						MaxSessionTTL:     NewDuration(defaults.MaxCertDuration),
						PortForwarding:    NewBoolOption(true),
					},
					Allow: RoleConditions{
						NodeLabels: Labels{Wildcard: []string{Wildcard}},
						Namespaces: []string{defaults.Namespace},
					},
					Deny: RoleConditions{
						Namespaces: []string{defaults.Namespace},
					},
				},
			},
			error: nil,
		},
		{
			name: "full valid role",
			in: `{
		      "kind": "role",
		      "version": "v3",
		      "metadata": {"name": "name1"},
		      "spec": {
                 "options": {
                   "cert_format": "standard",
                   "max_session_ttl": "20h",
                   "port_forwarding": true,
                   "client_idle_timeout": "17m",
                   "disconnect_expired_cert": "yes"
                 },
                 "allow": {
                   "node_labels": {"a": "b"},
                   "namespaces": ["default"],
                   "rules": [
                     {
                       "resources": ["role"],
                       "verbs": ["read", "list"],
                       "where": "contains(user.spec.traits[\"groups\"], \"prod\")",
                       "actions": [
                          "log(\"info\", \"log entry\")"
                       ]
                     }
                   ]
                 },
                 "deny": {
                   "logins": ["c"]
                 }
		      }
		    }`,
			role: RoleV3{
				Kind:    KindRole,
				Version: V3,
				Metadata: Metadata{
					Name:      "name1",
					Namespace: defaults.Namespace,
				},
				Spec: RoleSpecV3{
					Options: RoleOptions{
						CertificateFormat:     teleport.CertificateFormatStandard,
						MaxSessionTTL:         NewDuration(20 * time.Hour),
						PortForwarding:        NewBoolOption(true),
						ClientIdleTimeout:     NewDuration(17 * time.Minute),
						DisconnectExpiredCert: NewBool(true),
					},
					Allow: RoleConditions{
						NodeLabels: Labels{"a": []string{"b"}},
						Namespaces: []string{"default"},
						Rules: []Rule{
							Rule{
								Resources: []string{KindRole},
								Verbs:     []string{VerbRead, VerbList},
								Where:     "contains(user.spec.traits[\"groups\"], \"prod\")",
								Actions: []string{
									"log(\"info\", \"log entry\")",
								},
							},
						},
					},
					Deny: RoleConditions{
						Namespaces: []string{defaults.Namespace},
						Logins:     []string{"c"},
					},
				},
			},
			error: nil,
		},
		{
			name: "alternative options form",
			in: `{
		      "kind": "role",
		      "version": "v3",
		      "metadata": {"name": "name1"},
		      "spec": {
                 "options": {
                   "cert_format": "standard",
                   "max_session_ttl": "20h",
                   "port_forwarding": "yes",
                   "forward_agent": "yes",
                   "client_idle_timeout": "never",
                   "disconnect_expired_cert": "no"
                 },
                 "allow": {
                   "node_labels": {"a": "b"},
                   "namespaces": ["default"],
                   "rules": [
                     {
                       "resources": ["role"],
                       "verbs": ["read", "list"],
                       "where": "contains(user.spec.traits[\"groups\"], \"prod\")",
                       "actions": [
                          "log(\"info\", \"log entry\")"
                       ]
                     }
                   ]
                 },
                 "deny": {
                   "logins": ["c"]
                 }
		      }
		    }`,
			role: RoleV3{
				Kind:    KindRole,
				Version: V3,
				Metadata: Metadata{
					Name:      "name1",
					Namespace: defaults.Namespace,
				},
				Spec: RoleSpecV3{
					Options: RoleOptions{
						CertificateFormat:     teleport.CertificateFormatStandard,
						ForwardAgent:          NewBool(true),
						MaxSessionTTL:         NewDuration(20 * time.Hour),
						PortForwarding:        NewBoolOption(true),
						ClientIdleTimeout:     NewDuration(0),
						DisconnectExpiredCert: NewBool(false),
					},
					Allow: RoleConditions{
						NodeLabels: Labels{"a": []string{"b"}},
						Namespaces: []string{"default"},
						Rules: []Rule{
							Rule{
								Resources: []string{KindRole},
								Verbs:     []string{VerbRead, VerbList},
								Where:     "contains(user.spec.traits[\"groups\"], \"prod\")",
								Actions: []string{
									"log(\"info\", \"log entry\")",
								},
							},
						},
					},
					Deny: RoleConditions{
						Namespaces: []string{defaults.Namespace},
						Logins:     []string{"c"},
					},
				},
			},
			error: nil,
		},
		{
			name: "non-scalar and scalar values of labels",
			in: `{
		      "kind": "role",
		      "version": "v3",
		      "metadata": {"name": "name1"},
		      "spec": {
                 "options": {
                   "cert_format": "standard",
                   "max_session_ttl": "20h",
                   "port_forwarding": "yes",
                   "forward_agent": "yes",
                   "client_idle_timeout": "never",
                   "disconnect_expired_cert": "no"
                 },
                 "allow": {
                   "node_labels": {"a": "b", "key": ["val"], "key2": ["val2", "val3"]}
                 },
                 "deny": {
                   "logins": ["c"]
                 }
		      }
		    }`,
			role: RoleV3{
				Kind:    KindRole,
				Version: V3,
				Metadata: Metadata{
					Name:      "name1",
					Namespace: defaults.Namespace,
				},
				Spec: RoleSpecV3{
					Options: RoleOptions{
						CertificateFormat:     teleport.CertificateFormatStandard,
						ForwardAgent:          NewBool(true),
						MaxSessionTTL:         NewDuration(20 * time.Hour),
						PortForwarding:        NewBoolOption(true),
						ClientIdleTimeout:     NewDuration(0),
						DisconnectExpiredCert: NewBool(false),
					},
					Allow: RoleConditions{
						NodeLabels: Labels{
							"a":    []string{"b"},
							"key":  []string{"val"},
							"key2": []string{"val2", "val3"},
						},
						Namespaces: []string{"default"},
					},
					Deny: RoleConditions{
						Namespaces: []string{defaults.Namespace},
						Logins:     []string{"c"},
					},
				},
			},
			error: nil,
		},
	}
	for i, tc := range testCases {
		comment := Commentf("test case %v %q", i, tc.name)

		role, err := UnmarshalRole([]byte(tc.in))
		if tc.error != nil {
			c.Assert(err, NotNil, comment)
			if tc.matchMessage != "" {
				c.Assert(err.Error(), Matches, tc.matchMessage)
			}
		} else {
			c.Assert(err, IsNil, comment)
			fixtures.DeepCompare(c, *role, tc.role)

			out, err := json.Marshal(role)
			c.Assert(err, IsNil, comment)

			role2, err := UnmarshalRole(out)
			c.Assert(err, IsNil, comment)
			fixtures.DeepCompare(c, *role2, tc.role)
		}
	}
}

// TestLabelCompatibility makes sure that labels
// are serialized in format understood by older servers with
// scalar labels
func (s *RoleSuite) TestLabelCompatibility(c *C) {
	labels := Labels{
		"key": []string{"val"},
	}
	data, err := json.Marshal(labels)
	c.Assert(err, IsNil)

	var out map[string]string
	err = json.Unmarshal(data, &out)
	c.Assert(err, IsNil)
	c.Assert(out, DeepEquals, map[string]string{"key": "val"})
}

func (s *RoleSuite) TestCheckAccess(c *C) {
	type check struct {
		server    Server
		hasAccess bool
		login     string
	}
	serverA := &ServerV2{
		Metadata: Metadata{
			Name: "a",
		},
	}
	serverB := &ServerV2{
		Metadata: Metadata{
			Name:      "b",
			Namespace: defaults.Namespace,
			Labels:    map[string]string{"role": "worker", "status": "follower"},
		},
	}
	namespaceC := "namespace-c"
	serverC := &ServerV2{
		Metadata: Metadata{
			Name:      "c",
			Namespace: namespaceC,
			Labels:    map[string]string{"role": "db", "status": "follower"},
		},
	}
	serverC2 := &ServerV2{
		Metadata: Metadata{
			Name:      "c2",
			Namespace: namespaceC,
			Labels:    map[string]string{"role": "db01", "status": "follower01"},
		},
	}
	testCases := []struct {
		name   string
		roles  []RoleV3
		checks []check
	}{
		{
			name:  "empty role set has access to nothing",
			roles: []RoleV3{},
			checks: []check{
				{server: serverA, login: "root", hasAccess: false},
				{server: serverB, login: "root", hasAccess: false},
				{server: serverC, login: "root", hasAccess: false},
			},
		},
		{
			name: "role is limited to default namespace",
			roles: []RoleV3{
				RoleV3{
					Metadata: Metadata{
						Name:      "name1",
						Namespace: defaults.Namespace,
					},
					Spec: RoleSpecV3{
						Options: RoleOptions{
							MaxSessionTTL: Duration{20 * time.Hour},
						},
						Allow: RoleConditions{
							Namespaces: []string{defaults.Namespace},
							Logins:     []string{"admin"},
							NodeLabels: Labels{Wildcard: []string{Wildcard}},
						},
					},
				},
			},
			checks: []check{
				{server: serverA, login: "root", hasAccess: false},
				{server: serverA, login: "admin", hasAccess: true},
				{server: serverB, login: "root", hasAccess: false},
				{server: serverB, login: "admin", hasAccess: true},
				{server: serverC, login: "root", hasAccess: false},
				{server: serverC, login: "admin", hasAccess: false},
			},
		},
		{
			name: "role is limited to labels in default namespace",
			roles: []RoleV3{
				RoleV3{
					Metadata: Metadata{
						Name:      "name1",
						Namespace: defaults.Namespace,
					},
					Spec: RoleSpecV3{
						Options: RoleOptions{
							MaxSessionTTL: Duration{20 * time.Hour},
						},
						Allow: RoleConditions{
							Logins:     []string{"admin"},
							NodeLabels: Labels{"role": []string{"worker"}},
							Namespaces: []string{defaults.Namespace},
						},
					},
				},
			},
			checks: []check{
				{server: serverA, login: "root", hasAccess: false},
				{server: serverA, login: "admin", hasAccess: false},
				{server: serverB, login: "root", hasAccess: false},
				{server: serverB, login: "admin", hasAccess: true},
				{server: serverC, login: "root", hasAccess: false},
				{server: serverC, login: "admin", hasAccess: false},
			},
		},
		{
			name: "role matches any label out of multiple labels",
			roles: []RoleV3{
				RoleV3{
					Metadata: Metadata{
						Name:      "name1",
						Namespace: defaults.Namespace,
					},
					Spec: RoleSpecV3{
						Options: RoleOptions{
							MaxSessionTTL: Duration{20 * time.Hour},
						},
						Allow: RoleConditions{
							Logins:     []string{"admin"},
							NodeLabels: Labels{"role": []string{"worker2", "worker"}},
							Namespaces: []string{defaults.Namespace},
						},
					},
				},
			},
			checks: []check{
				{server: serverA, login: "root", hasAccess: false},
				{server: serverA, login: "admin", hasAccess: false},
				{server: serverB, login: "root", hasAccess: false},
				{server: serverB, login: "admin", hasAccess: true},
				{server: serverC, login: "root", hasAccess: false},
				{server: serverC, login: "admin", hasAccess: false},
			},
		},
		{
			name: "node_labels with emtpy list value matches nothing",
			roles: []RoleV3{
				RoleV3{
					Metadata: Metadata{
						Name:      "name1",
						Namespace: defaults.Namespace,
					},
					Spec: RoleSpecV3{
						Options: RoleOptions{
							MaxSessionTTL: Duration{20 * time.Hour},
						},
						Allow: RoleConditions{
							Logins:     []string{"admin"},
							NodeLabels: Labels{"role": []string{}},
							Namespaces: []string{defaults.Namespace},
						},
					},
				},
			},
			checks: []check{
				{server: serverA, login: "root", hasAccess: false},
				{server: serverA, login: "admin", hasAccess: false},
				{server: serverB, login: "root", hasAccess: false},
				{server: serverB, login: "admin", hasAccess: false},
				{server: serverC, login: "root", hasAccess: false},
				{server: serverC, login: "admin", hasAccess: false},
			},
		},
		{
			name: "one role is more permissive than another",
			roles: []RoleV3{
				RoleV3{
					Metadata: Metadata{
						Name:      "name1",
						Namespace: defaults.Namespace,
					},
					Spec: RoleSpecV3{
						Options: RoleOptions{
							MaxSessionTTL: Duration{20 * time.Hour},
						},
						Allow: RoleConditions{
							Logins:     []string{"admin"},
							NodeLabels: Labels{"role": []string{"worker"}},
							Namespaces: []string{defaults.Namespace},
						},
					},
				},
				RoleV3{
					Metadata: Metadata{
						Name:      "name1",
						Namespace: defaults.Namespace,
					},
					Spec: RoleSpecV3{
						Options: RoleOptions{
							MaxSessionTTL: Duration{20 * time.Hour},
						},
						Allow: RoleConditions{
							Logins:     []string{"root", "admin"},
							NodeLabels: Labels{Wildcard: []string{Wildcard}},
							Namespaces: []string{Wildcard},
						},
					},
				},
			},
			checks: []check{
				{server: serverA, login: "root", hasAccess: true},
				{server: serverA, login: "admin", hasAccess: true},
				{server: serverB, login: "root", hasAccess: true},
				{server: serverB, login: "admin", hasAccess: true},
				{server: serverC, login: "root", hasAccess: true},
				{server: serverC, login: "admin", hasAccess: true},
			},
		},
		{
			name: "one role needs to access servers sharing the partially same label value",
			roles: []RoleV3{
				RoleV3{
					Metadata: Metadata{
						Name:      "name1",
						Namespace: namespaceC,
					},
					Spec: RoleSpecV3{
						Options: RoleOptions{
							MaxSessionTTL: Duration{20 * time.Hour},
						},
						Allow: RoleConditions{
							Logins:     []string{"admin"},
							NodeLabels: Labels{"role": []string{"^db(.*)$"}, "status": []string{"follow*"}},
							Namespaces: []string{namespaceC},
						},
					},
				},
			},
			checks: []check{
				{server: serverA, login: "root", hasAccess: false},
				{server: serverA, login: "admin", hasAccess: false},
				{server: serverB, login: "root", hasAccess: false},
				{server: serverB, login: "admin", hasAccess: false},
				{server: serverC, login: "root", hasAccess: false},
				{server: serverC, login: "admin", hasAccess: true},
				{server: serverC2, login: "root", hasAccess: false},
				{server: serverC2, login: "admin", hasAccess: true},
			},
		},
	}
	for i, tc := range testCases {

		var set RoleSet
		for i := range tc.roles {
			set = append(set, &tc.roles[i])
		}
		for j, check := range tc.checks {
			comment := Commentf("test case %v '%v', check %v", i, tc.name, j)
			result := set.CheckAccessToServer(check.login, check.server)
			if check.hasAccess {
				c.Assert(result, IsNil, comment)
			} else {
				c.Assert(trace.IsAccessDenied(result), Equals, true, comment)
			}

		}
	}
}

// testContext overrides context and captures log writes in action
type testContext struct {
	Context
	// Buffer captures log writes
	buffer *bytes.Buffer
}

// Write is implemented explicitly to avoid collision
// of String methods when embedding
func (t *testContext) Write(data []byte) (int, error) {
	return t.buffer.Write(data)
}

func (s *RoleSuite) TestCheckRuleAccess(c *C) {
	type check struct {
		hasAccess   bool
		verb        string
		namespace   string
		rule        string
		context     testContext
		matchBuffer string
	}
	testCases := []struct {
		name   string
		roles  []RoleV3
		checks []check
	}{
		{
			name:  "0 - empty role set has access to nothing",
			roles: []RoleV3{},
			checks: []check{
				{rule: KindUser, verb: ActionWrite, namespace: defaults.Namespace, hasAccess: false},
			},
		},
		{
			name: "1 - user can read session but can't list in default namespace",
			roles: []RoleV3{
				RoleV3{
					Metadata: Metadata{
						Name:      "name1",
						Namespace: defaults.Namespace,
					},
					Spec: RoleSpecV3{
						Allow: RoleConditions{
							Namespaces: []string{defaults.Namespace},
							Rules: []Rule{
								NewRule(KindSSHSession, []string{VerbRead}),
							},
						},
					},
				},
			},
			checks: []check{
				{rule: KindSSHSession, verb: VerbRead, namespace: defaults.Namespace, hasAccess: true},
				{rule: KindSSHSession, verb: VerbList, namespace: defaults.Namespace, hasAccess: false},
			},
		},
		{
			name: "2 - user can read sessions in system namespace and create stuff in default namespace",
			roles: []RoleV3{
				RoleV3{
					Metadata: Metadata{
						Name:      "name1",
						Namespace: defaults.Namespace,
					},
					Spec: RoleSpecV3{
						Allow: RoleConditions{
							Namespaces: []string{"system"},
							Rules: []Rule{
								NewRule(KindSSHSession, []string{VerbRead}),
							},
						},
					},
				},
				RoleV3{
					Metadata: Metadata{
						Name:      "name2",
						Namespace: defaults.Namespace,
					},
					Spec: RoleSpecV3{
						Allow: RoleConditions{
							Namespaces: []string{defaults.Namespace},
							Rules: []Rule{
								NewRule(KindSSHSession, []string{VerbCreate, VerbRead}),
							},
						},
					},
				},
			},
			checks: []check{
				{rule: KindSSHSession, verb: VerbRead, namespace: defaults.Namespace, hasAccess: true},
				{rule: KindSSHSession, verb: VerbCreate, namespace: defaults.Namespace, hasAccess: true},
				{rule: KindSSHSession, verb: VerbCreate, namespace: "system", hasAccess: false},
				{rule: KindRole, verb: VerbRead, namespace: defaults.Namespace, hasAccess: false},
			},
		},
		{
			name: "3 - deny rules override allow rules",
			roles: []RoleV3{
				RoleV3{
					Metadata: Metadata{
						Name:      "name1",
						Namespace: defaults.Namespace,
					},
					Spec: RoleSpecV3{
						Deny: RoleConditions{
							Namespaces: []string{defaults.Namespace},
							Rules: []Rule{
								NewRule(KindSSHSession, []string{VerbCreate}),
							},
						},
						Allow: RoleConditions{
							Namespaces: []string{defaults.Namespace},
							Rules: []Rule{
								NewRule(KindSSHSession, []string{VerbCreate}),
							},
						},
					},
				},
			},
			checks: []check{
				{rule: KindSSHSession, verb: VerbCreate, namespace: defaults.Namespace, hasAccess: false},
			},
		},
		{
			name: "4 - user can read sessions if trait matches",
			roles: []RoleV3{
				RoleV3{
					Metadata: Metadata{
						Name:      "name1",
						Namespace: defaults.Namespace,
					},
					Spec: RoleSpecV3{
						Allow: RoleConditions{
							Namespaces: []string{defaults.Namespace},
							Rules: []Rule{
								Rule{
									Resources: []string{KindSession},
									Verbs:     []string{VerbRead},
									Where:     `contains(user.spec.traits["group"], "prod")`,
									Actions: []string{
										`log("info", "4 - tc match for user %v", user.metadata.name)`,
									},
								},
							},
						},
					},
				},
			},
			checks: []check{
				{rule: KindSession, verb: VerbRead, namespace: defaults.Namespace, hasAccess: false},
				{rule: KindSession, verb: VerbList, namespace: defaults.Namespace, hasAccess: false},
				{
					context: testContext{
						buffer: &bytes.Buffer{},
						Context: Context{
							User: &UserV2{
								Metadata: Metadata{
									Name: "bob",
								},
								Spec: UserSpecV2{
									Traits: map[string][]string{
										"group": []string{"dev", "prod"},
									},
								},
							},
						},
					},
					rule:      KindSession,
					verb:      VerbRead,
					namespace: defaults.Namespace,
					hasAccess: true,
				},
				{
					context: testContext{
						buffer: &bytes.Buffer{},
						Context: Context{
							User: &UserV2{
								Spec: UserSpecV2{
									Traits: map[string][]string{
										"group": []string{"dev"},
									},
								},
							},
						},
					},
					rule:      KindSession,
					verb:      VerbRead,
					namespace: defaults.Namespace,
					hasAccess: false,
				},
			},
		},
		{
			name: "5 - user can read role if role has label",
			roles: []RoleV3{
				RoleV3{
					Metadata: Metadata{
						Name:      "name1",
						Namespace: defaults.Namespace,
					},
					Spec: RoleSpecV3{
						Allow: RoleConditions{
							Namespaces: []string{defaults.Namespace},
							Rules: []Rule{
								Rule{
									Resources: []string{KindRole},
									Verbs:     []string{VerbRead},
									Where:     `equals(resource.metadata.labels["team"], "dev")`,
									Actions: []string{
										`log("error", "4 - tc match")`,
									},
								},
							},
						},
					},
				},
			},
			checks: []check{
				{rule: KindRole, verb: VerbRead, namespace: defaults.Namespace, hasAccess: false},
				{rule: KindRole, verb: VerbList, namespace: defaults.Namespace, hasAccess: false},
				{
					context: testContext{
						buffer: &bytes.Buffer{},
						Context: Context{
							Resource: &RoleV2{
								Metadata: Metadata{
									Labels: map[string]string{"team": "dev"},
								},
							},
						},
					},
					rule:      KindRole,
					verb:      VerbRead,
					namespace: defaults.Namespace,
					hasAccess: true,
				},
			},
		},
		{
			name: "More specific rule wins",
			roles: []RoleV3{
				RoleV3{
					Metadata: Metadata{
						Name:      "name1",
						Namespace: defaults.Namespace,
					},
					Spec: RoleSpecV3{
						Allow: RoleConditions{
							Namespaces: []string{defaults.Namespace},
							Rules: []Rule{
								Rule{
									Resources: []string{Wildcard},
									Verbs:     []string{Wildcard},
								},
								Rule{
									Resources: []string{KindRole},
									Verbs:     []string{VerbRead},
									Where:     `equals(resource.metadata.labels["team"], "dev")`,
									Actions: []string{
										`log("info", "matched more specific rule")`,
									},
								},
							},
						},
					},
				},
			},
			checks: []check{
				{
					context: testContext{
						buffer: &bytes.Buffer{},
						Context: Context{
							Resource: &RoleV2{
								Metadata: Metadata{
									Labels: map[string]string{"team": "dev"},
								},
							},
						},
					},
					rule:        KindRole,
					verb:        VerbRead,
					namespace:   defaults.Namespace,
					hasAccess:   true,
					matchBuffer: ".*more specific rule.*",
				},
			},
		},
	}
	for i, tc := range testCases {
		var set RoleSet
		for i := range tc.roles {
			set = append(set, &tc.roles[i])
		}
		for j, check := range tc.checks {
			comment := Commentf("test case %v '%v', check %v", i, tc.name, j)
			result := set.CheckAccessToRule(&check.context, check.namespace, check.rule, check.verb, false)
			if check.hasAccess {
				c.Assert(result, IsNil, comment)
			} else {
				c.Assert(trace.IsAccessDenied(result), Equals, true, comment)
			}
			if check.matchBuffer != "" {
				c.Assert(check.context.buffer.String(), Matches, check.matchBuffer, comment)
			}
		}
	}
}

func (s *RoleSuite) TestCheckRuleSorting(c *C) {
	testCases := []struct {
		name  string
		rules []Rule
		set   RuleSet
	}{
		{
			name: "single rule set sorts OK",
			rules: []Rule{
				{
					Resources: []string{KindUser},
					Verbs:     []string{VerbCreate},
				},
			},
			set: RuleSet{
				KindUser: []Rule{
					{
						Resources: []string{KindUser},
						Verbs:     []string{VerbCreate},
					},
				},
			},
		},
		{
			name: "rule with where section is more specific",
			rules: []Rule{
				{
					Resources: []string{KindUser},
					Verbs:     []string{VerbCreate},
				},
				{
					Resources: []string{KindUser},
					Verbs:     []string{VerbCreate},
					Where:     "contains(user.spec.traits[\"groups\"], \"prod\")",
				},
			},
			set: RuleSet{
				KindUser: []Rule{
					{
						Resources: []string{KindUser},
						Verbs:     []string{VerbCreate},
						Where:     "contains(user.spec.traits[\"groups\"], \"prod\")",
					},
					{
						Resources: []string{KindUser},
						Verbs:     []string{VerbCreate},
					},
				},
			},
		},
		{
			name: "rule with action is more specific",
			rules: []Rule{
				{
					Resources: []string{KindUser},
					Verbs:     []string{VerbCreate},

					Where: "contains(user.spec.traits[\"groups\"], \"prod\")",
				},
				{
					Resources: []string{KindUser},
					Verbs:     []string{VerbCreate},
					Where:     "contains(user.spec.traits[\"groups\"], \"prod\")",
					Actions: []string{
						"log(\"info\", \"log entry\")",
					},
				},
			},
			set: RuleSet{
				KindUser: []Rule{
					{
						Resources: []string{KindUser},
						Verbs:     []string{VerbCreate},
						Where:     "contains(user.spec.traits[\"groups\"], \"prod\")",
						Actions: []string{
							"log(\"info\", \"log entry\")",
						},
					},
					{
						Resources: []string{KindUser},
						Verbs:     []string{VerbCreate},
						Where:     "contains(user.spec.traits[\"groups\"], \"prod\")",
					},
				},
			},
		},
	}
	for i, tc := range testCases {
		comment := Commentf("test case %v '%v'", i, tc.name)
		out := MakeRuleSet(tc.rules)
		c.Assert(tc.set, DeepEquals, out, comment)
	}
}

func (s *RoleSuite) TestApplyTraits(c *C) {
	type rule struct {
		inLogins      []string
		outLogins     []string
		inLabels      Labels
		outLabels     Labels
		inKubeGroups  []string
		outKubeGroups []string
	}
	var tests = []struct {
		comment  string
		inTraits map[string][]string
		allow    rule
		deny     rule
	}{

		{
			comment: "logins substitute in allow rule",
			inTraits: map[string][]string{
				"foo": []string{"bar"},
			},
			allow: rule{
				inLogins:  []string{`{{external.foo}}`, "root"},
				outLogins: []string{"bar", "root"},
			},
		},
		{
			comment: "logins substitute in deny rule",
			inTraits: map[string][]string{
				"foo": []string{"bar"},
			},
			deny: rule{
				inLogins:  []string{`{{external.foo}}`},
				outLogins: []string{"bar"},
			},
		},
		{
			comment: "kube group substitute in allow rule",
			inTraits: map[string][]string{
				"foo": []string{"bar"},
			},
			allow: rule{
				inKubeGroups:  []string{`{{external.foo}}`, "root"},
				outKubeGroups: []string{"bar", "root"},
			},
		},
		{
			comment: "kube group substitute in deny rule",
			inTraits: map[string][]string{
				"foo": []string{"bar"},
			},
			deny: rule{
				inKubeGroups:  []string{`{{external.foo}}`, "root"},
				outKubeGroups: []string{"bar", "root"},
			},
		},
		{
			comment: "no variable in logins",
			inTraits: map[string][]string{
				"foo": []string{"bar"},
			},
			allow: rule{
				inLogins:  []string{"root"},
				outLogins: []string{"root"},
			},
		},

		{
			comment: "invalid variable in logins gets passed along",
			inTraits: map[string][]string{
				"foo": []string{"bar"},
			},
			allow: rule{
				inLogins:  []string{`external.foo}}`},
				outLogins: []string{`external.foo}}`},
			},
		},
		{
			comment: "variable in logins, none in traits",
			inTraits: map[string][]string{
				"foo": []string{"bar"},
			},
			allow: rule{
				inLogins:  []string{`{{internal.bar}}`, "root"},
				outLogins: []string{"root"},
			},
		},
		{
			comment: "multiple variables in traits",
			inTraits: map[string][]string{
				"logins": []string{"bar", "baz"},
			},
			allow: rule{
				inLogins:  []string{`{{internal.logins}}`, "root"},
				outLogins: []string{"bar", "baz", "root"},
			},
		},
		{
			comment: "deduplicate",
			inTraits: map[string][]string{
				"foo": []string{"bar"},
			},
			allow: rule{
				inLogins:  []string{`{{external.foo}}`, "bar"},
				outLogins: []string{"bar"},
			},
		},
		{
			comment: "invalid unix login",
			inTraits: map[string][]string{
				"foo": []string{"-foo"},
			},
			allow: rule{
				inLogins:  []string{`{{external.foo}}`, "bar"},
				outLogins: []string{"bar"},
			},
		},
		{
			comment: "label substitute in allow and deny rule",
			inTraits: map[string][]string{
				"foo":   []string{"bar"},
				"hello": []string{"there"},
			},
			allow: rule{
				inLabels:  Labels{`{{external.foo}}`: []string{"{{external.hello}}"}},
				outLabels: Labels{`bar`: []string{"there"}},
			},
			deny: rule{
				inLabels:  Labels{`{{external.hello}}`: []string{"{{external.foo}}"}},
				outLabels: Labels{`there`: []string{"bar"}},
			},
		},

		{
			comment: "missing node variables are set to empty during substitution",
			inTraits: map[string][]string{
				"foo": []string{"bar"},
			},
			allow: rule{
				inLabels: Labels{
					`{{external.foo}}`:     []string{"value"},
					`{{external.missing}}`: []string{"missing"},
					`missing`:              []string{"{{external.missing}}", "othervalue"},
				},
				outLabels: Labels{
					`bar`:     []string{"value"},
					"missing": []string{"", "othervalue"},
					"":        []string{"missing"},
				},
			},
		},

		{
			comment: "the first variable value is picked for label keys",
			inTraits: map[string][]string{
				"foo": []string{"bar", "baz"},
			},
			allow: rule{
				inLabels:  Labels{`{{external.foo}}`: []string{"value"}},
				outLabels: Labels{`bar`: []string{"value"}},
			},
		},

		{
			comment: "all values are expanded for label values",
			inTraits: map[string][]string{
				"foo": []string{"bar", "baz"},
			},
			allow: rule{
				inLabels:  Labels{`key`: []string{`{{external.foo}}`}},
				outLabels: Labels{`key`: []string{"bar", "baz"}},
			},
		},
	}

	for i, tt := range tests {
		comment := Commentf("Test %v %v", i, tt.comment)

		role := &RoleV3{
			Kind:    KindRole,
			Version: V3,
			Metadata: Metadata{
				Name:      "name1",
				Namespace: defaults.Namespace,
			},
			Spec: RoleSpecV3{
				Allow: RoleConditions{
					Logins:     tt.allow.inLogins,
					NodeLabels: tt.allow.inLabels,
					KubeGroups: tt.allow.inKubeGroups,
				},
				Deny: RoleConditions{
					Logins:     tt.deny.inLogins,
					NodeLabels: tt.deny.inLabels,
					KubeGroups: tt.deny.inKubeGroups,
				},
			},
		}

		outRole := role.ApplyTraits(tt.inTraits)
		c.Assert(outRole.GetLogins(Allow), DeepEquals, tt.allow.outLogins, comment)
		c.Assert(outRole.GetNodeLabels(Allow), DeepEquals, tt.allow.outLabels, comment)
		c.Assert(outRole.GetKubeGroups(Allow), DeepEquals, tt.allow.outKubeGroups, comment)

		c.Assert(outRole.GetLogins(Deny), DeepEquals, tt.deny.outLogins, comment)
		c.Assert(outRole.GetNodeLabels(Deny), DeepEquals, tt.deny.outLabels, comment)
		c.Assert(outRole.GetKubeGroups(Deny), DeepEquals, tt.deny.outKubeGroups, comment)
	}
}

func (s *RoleSuite) TestCheckAndSetDefaults(c *C) {
	var tests = []struct {
		inLogins []string
		outError bool
	}{
		// 0 - invalid syntax
		{
			[]string{"{{foo"},
			true,
		},
		// 1 - invalid syntax
		{
			[]string{"bar}}"},
			true,
		},
		// 2 - valid syntax
		{
			[]string{"{{foo.bar}}"},
			false,
		},
		// 3 - valid syntax
		{
			[]string{`{{external["http://schemas.microsoft.com/ws/2008/06/identity/claims/windowsaccountname"]}}`},
			false,
		},
	}

	for i, tt := range tests {
		comment := Commentf("Test %v", i)

		role := &RoleV3{
			Kind:    KindRole,
			Version: V3,
			Metadata: Metadata{
				Name:      "name1",
				Namespace: defaults.Namespace,
			},
			Spec: RoleSpecV3{
				Allow: RoleConditions{
					Logins: tt.inLogins,
				},
			},
		}
		if tt.outError {
			c.Assert(role.CheckAndSetDefaults(), NotNil, comment)
		} else {
			c.Assert(role.CheckAndSetDefaults(), IsNil, comment)
		}
	}

}

// BenchmarkCheckAccessToServer tests how long it takes to run
// CheckAccessToServer across 4,000 nodes for 5 roles each with 5 logins each.
//
// To run benchmark:
//
//    go test -bench=.
//
// To run benchmark and obtain CPU and memory profiling:
//
//    go test -bench=. -cpuprofile=cpu.prof -memprofile=mem.prof
//
// To use the command line tool to read the profile:
//
//   go tool pprof cpu.prof
//   go tool pprof cpu.prof
//
// To generate a graph:
//
//   go tool pprof --pdf cpu.prof > cpu.pdf
//   go tool pprof --pdf mem.prof > mem.pdf
//
func BenchmarkCheckAccessToServer(b *testing.B) {
	servers := make([]*ServerV2, 0, 4000)

	// Create 4,000 servers with random IDs.
	for i := 0; i < 4000; i++ {
		hostname := uuid.NewUUID().String()
		servers = append(servers, &ServerV2{
			Kind:    KindNode,
			Version: V2,
			Metadata: Metadata{
				Name:      hostname,
				Namespace: defaults.Namespace,
			},
			Spec: ServerSpecV2{
				Addr:     "127.0.0.1:3022",
				Hostname: hostname,
			},
		})
	}

	// Create RoleSet with five roles: one admin role and four generic roles
	// that have five logins each and only have access to the foo:bar label.
	var set RoleSet
	set = append(set, NewAdminRole())
	for i := 0; i < 4; i++ {
		set = append(set, &RoleV3{
			Kind:    KindRole,
			Version: V3,
			Metadata: Metadata{
				Name:      strconv.Itoa(i),
				Namespace: defaults.Namespace,
			},
			Spec: RoleSpecV3{
				Allow: RoleConditions{
					Logins:     []string{"admin", "one", "two", "three", "four"},
					NodeLabels: Labels{"a": []string{"b"}},
				},
			},
		})
	}

	// Initialization is complete, start the benchmark timer.
	b.ResetTimer()

	// Build a map of all allowed logins.
	allowLogins := map[string]bool{}
	for _, role := range set {
		for _, login := range role.GetLogins(Allow) {
			allowLogins[login] = true
		}
	}

	// Check access to all 4,000 nodes.
	for n := 0; n < b.N; n++ {
		for i := 0; i < 4000; i++ {
			for login, _ := range allowLogins {
				set.CheckAccessToServer(login, servers[i])
			}
		}
	}
}
