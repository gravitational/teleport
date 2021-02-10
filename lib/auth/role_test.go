/*
Copyright 2015-2021 Gravitational, Inc.

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
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/pborman/uuid"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/trace"
)

// TestConnAndSessLimits verifies that role sets correctly calculate
// a user's MaxConnections and MaxSessions values from multiple
// roles with different individual values.  These are tested together since
// both values use the same resolution rules.
func TestConnAndSessLimits(t *testing.T) {
	tts := []struct {
		desc string
		vals []int64
		want int64
	}{
		{
			desc: "smallest nonzero value is selected from mixed values",
			vals: []int64{8, 6, 7, 5, 3, 0, 9},
			want: 3,
		},
		{
			desc: "smallest value selected from all nonzero values",
			vals: []int64{5, 6, 7, 8},
			want: 5,
		},
		{
			desc: "all zero values results in a zero value",
			vals: []int64{0, 0, 0, 0, 0, 0, 0},
			want: 0,
		},
	}
	for ti, tt := range tts {
		cmt := fmt.Sprintf("test case %d: %s", ti, tt.desc)
		var set RoleSet
		for i, val := range tt.vals {
			role := &RoleV3{
				Kind:    KindRole,
				Version: V3,
				Metadata: Metadata{
					Name:      fmt.Sprintf("role-%d", i),
					Namespace: defaults.Namespace,
				},
				Spec: RoleSpecV3{
					Options: RoleOptions{
						MaxConnections: val,
						MaxSessions:    val,
					},
				},
			}
			require.NoError(t, role.CheckAndSetDefaults(), cmt)
			set = append(set, role)
		}
		require.Equal(t, tt.want, set.MaxConnections(), cmt)
		require.Equal(t, tt.want, set.MaxSessions(), cmt)
	}
}

func TestValidateRole(t *testing.T) {
	var tests = []struct {
		name         string
		spec         RoleSpecV3
		err          error
		matchMessage string
	}{
		{
			name: "valid syntax",
			spec: RoleSpecV3{
				Allow: RoleConditions{
					Logins: []string{`{{external["http://schemas.microsoft.com/ws/2008/06/identity/claims/windowsaccountname"]}}`},
				},
			},
		},
		{
			name: "invalid role condition login syntax",
			spec: RoleSpecV3{
				Allow: RoleConditions{
					Logins: []string{"{{foo"},
				},
			},
			err:          trace.BadParameter(""),
			matchMessage: "invalid login found",
		},
		{
			name: "unsupported function in actions",
			spec: RoleSpecV3{
				Allow: RoleConditions{
					Logins: []string{`{{external["http://schemas.microsoft.com/ws/2008/06/identity/claims/windowsaccountname"]}}`},
					Rules: []Rule{
						{
							Resources: []string{"role"},
							Verbs:     []string{"read", "list"},
							Where:     "containz(user.spec.traits[\"groups\"], \"prod\")",
						},
					},
				},
			},
			err:          trace.BadParameter(""),
			matchMessage: "unsupported function: containz",
		},
		{
			name: "unsupported function in where",
			spec: RoleSpecV3{
				Allow: RoleConditions{
					Logins: []string{`{{external["http://schemas.microsoft.com/ws/2008/06/identity/claims/windowsaccountname"]}}`},
					Rules: []Rule{
						{
							Resources: []string{"role"},
							Verbs:     []string{"read", "list"},
							Where:     "contains(user.spec.traits[\"groups\"], \"prod\")",
							Actions:   []string{"zzz(\"info\", \"log entry\")"},
						},
					},
				},
			},
			err:          trace.BadParameter(""),
			matchMessage: "unsupported function: zzz",
		},
	}

	for _, tc := range tests {
		err := ValidateRole(&types.RoleV3{
			Metadata: Metadata{
				Name:      "name1",
				Namespace: defaults.Namespace,
			},
			Spec: tc.spec,
		})
		if tc.err != nil {
			require.Error(t, err, tc.name)
			if tc.matchMessage != "" {
				require.Contains(t, err.Error(), tc.matchMessage)
			}
		} else {
			require.NoError(t, err, tc.name)
		}
	}
}

// TestLabelCompatibility makes sure that labels
// are serialized in format understood by older servers with
// scalar labels
func TestLabelCompatibility(t *testing.T) {
	labels := Labels{
		"key": []string{"val"},
	}
	data, err := json.Marshal(labels)
	require.NoError(t, err)

	var out map[string]string
	err = json.Unmarshal(data, &out)
	require.NoError(t, err)
	require.Equal(t, map[string]string{"key": "val"}, out)
}

func TestCheckAccessToServer(t *testing.T) {
	type check struct {
		server    Server
		hasAccess bool
		login     string
	}
	serverNoLabels := &ServerV2{
		Metadata: Metadata{
			Name: "a",
		},
	}
	serverWorker := &ServerV2{
		Metadata: Metadata{
			Name:      "b",
			Namespace: defaults.Namespace,
			Labels:    map[string]string{"role": "worker", "status": "follower"},
		},
	}
	namespaceC := "namespace-c"
	serverDB := &ServerV2{
		Metadata: Metadata{
			Name:      "c",
			Namespace: namespaceC,
			Labels:    map[string]string{"role": "db", "status": "follower"},
		},
	}
	serverDBWithSuffix := &ServerV2{
		Metadata: Metadata{
			Name:      "c2",
			Namespace: namespaceC,
			Labels:    map[string]string{"role": "db01", "status": "follower01"},
		},
	}
	newRole := func(mut func(*RoleV3)) RoleV3 {
		r := RoleV3{
			Metadata: Metadata{
				Name:      "name",
				Namespace: defaults.Namespace,
			},
			Spec: RoleSpecV3{
				Options: RoleOptions{
					MaxSessionTTL: Duration(20 * time.Hour),
				},
				Allow: RoleConditions{
					NodeLabels: Labels{Wildcard: []string{Wildcard}},
					Namespaces: []string{Wildcard},
				},
			},
		}
		mut(&r)
		return r
	}
	testCases := []struct {
		name      string
		roles     []RoleV3
		checks    []check
		mfaParams AccessMFAParams
	}{
		{
			name:  "empty role set has access to nothing",
			roles: []RoleV3{},
			checks: []check{
				{server: serverNoLabels, login: "root", hasAccess: false},
				{server: serverWorker, login: "root", hasAccess: false},
				{server: serverDB, login: "root", hasAccess: false},
			},
		},
		{
			name: "role is limited to default namespace",
			roles: []RoleV3{
				newRole(func(r *RoleV3) {
					r.Spec.Allow.Logins = []string{"admin"}
					r.Spec.Allow.Namespaces = []string{defaults.Namespace}
				}),
			},
			checks: []check{
				{server: serverNoLabels, login: "root", hasAccess: false},
				{server: serverNoLabels, login: "admin", hasAccess: true},
				{server: serverWorker, login: "root", hasAccess: false},
				{server: serverWorker, login: "admin", hasAccess: true},
				{server: serverDB, login: "root", hasAccess: false},
				{server: serverDB, login: "admin", hasAccess: false},
			},
		},
		{
			name: "role is limited to labels in default namespace",
			roles: []RoleV3{
				newRole(func(r *RoleV3) {
					r.Spec.Allow.Logins = []string{"admin"}
					r.Spec.Allow.NodeLabels = Labels{"role": []string{"worker"}}
				}),
			},
			checks: []check{
				{server: serverNoLabels, login: "root", hasAccess: false},
				{server: serverNoLabels, login: "admin", hasAccess: false},
				{server: serverWorker, login: "root", hasAccess: false},
				{server: serverWorker, login: "admin", hasAccess: true},
				{server: serverDB, login: "root", hasAccess: false},
				{server: serverDB, login: "admin", hasAccess: false},
			},
		},
		{
			name: "role matches any label out of multiple labels",
			roles: []RoleV3{
				newRole(func(r *RoleV3) {
					r.Spec.Allow.Logins = []string{"admin"}
					r.Spec.Allow.NodeLabels = Labels{"role": []string{"worker2", "worker"}}
				}),
			},
			checks: []check{
				{server: serverNoLabels, login: "root", hasAccess: false},
				{server: serverNoLabels, login: "admin", hasAccess: false},
				{server: serverWorker, login: "root", hasAccess: false},
				{server: serverWorker, login: "admin", hasAccess: true},
				{server: serverDB, login: "root", hasAccess: false},
				{server: serverDB, login: "admin", hasAccess: false},
			},
		},
		{
			name: "node_labels with empty list value matches nothing",
			roles: []RoleV3{
				newRole(func(r *RoleV3) {
					r.Spec.Allow.Logins = []string{"admin"}
					r.Spec.Allow.NodeLabels = Labels{"role": []string{}}
				}),
			},
			checks: []check{
				{server: serverNoLabels, login: "root", hasAccess: false},
				{server: serverNoLabels, login: "admin", hasAccess: false},
				{server: serverWorker, login: "root", hasAccess: false},
				{server: serverWorker, login: "admin", hasAccess: false},
				{server: serverDB, login: "root", hasAccess: false},
				{server: serverDB, login: "admin", hasAccess: false},
			},
		},
		{
			name: "one role is more permissive than another",
			roles: []RoleV3{
				newRole(func(r *RoleV3) {
					r.Spec.Allow.Logins = []string{"admin"}
					r.Spec.Allow.Namespaces = []string{defaults.Namespace}
					r.Spec.Allow.NodeLabels = Labels{"role": []string{"worker"}}
				}),
				newRole(func(r *RoleV3) {
					r.Spec.Allow.Logins = []string{"root", "admin"}
				}),
			},
			checks: []check{
				{server: serverNoLabels, login: "root", hasAccess: true},
				{server: serverNoLabels, login: "admin", hasAccess: true},
				{server: serverWorker, login: "root", hasAccess: true},
				{server: serverWorker, login: "admin", hasAccess: true},
				{server: serverDB, login: "root", hasAccess: true},
				{server: serverDB, login: "admin", hasAccess: true},
			},
		},
		{
			name: "one role needs to access servers sharing the partially same label value",
			roles: []RoleV3{
				newRole(func(r *RoleV3) {
					r.Spec.Allow.Logins = []string{"admin"}
					r.Spec.Allow.NodeLabels = Labels{"role": []string{"^db(.*)$"}, "status": []string{"follow*"}}
					r.Spec.Allow.Namespaces = []string{namespaceC}
				}),
			},
			checks: []check{
				{server: serverNoLabels, login: "root", hasAccess: false},
				{server: serverNoLabels, login: "admin", hasAccess: false},
				{server: serverWorker, login: "root", hasAccess: false},
				{server: serverWorker, login: "admin", hasAccess: false},
				{server: serverDB, login: "root", hasAccess: false},
				{server: serverDB, login: "admin", hasAccess: true},
				{server: serverDBWithSuffix, login: "root", hasAccess: false},
				{server: serverDBWithSuffix, login: "admin", hasAccess: true},
			},
		},
		{
			name: "no logins means no access",
			roles: []RoleV3{
				newRole(func(r *RoleV3) {
					r.Spec.Allow.Logins = nil
				}),
			},
			checks: []check{
				{server: serverNoLabels, login: "root", hasAccess: false},
				{server: serverNoLabels, login: "admin", hasAccess: false},
				{server: serverWorker, login: "root", hasAccess: false},
				{server: serverWorker, login: "admin", hasAccess: false},
				{server: serverDB, login: "root", hasAccess: false},
				{server: serverDB, login: "admin", hasAccess: false},
			},
		},
		{
			name: "one role requires MFA but MFA was not verified",
			roles: []RoleV3{
				newRole(func(r *RoleV3) {
					r.Spec.Allow.Logins = []string{"root"}
					r.Spec.Allow.NodeLabels = Labels{"role": []string{"worker"}}
					r.Spec.Options.RequireSessionMFA = true
				}),
				newRole(func(r *RoleV3) {
					r.Spec.Allow.Logins = []string{"root"}
					r.Spec.Options.RequireSessionMFA = false
				}),
			},
			mfaParams: AccessMFAParams{Verified: false},
			checks: []check{
				{server: serverNoLabels, login: "root", hasAccess: true},
				{server: serverWorker, login: "root", hasAccess: false},
				{server: serverDB, login: "root", hasAccess: true},
			},
		},
		{
			name: "one role requires MFA and MFA was verified",
			roles: []RoleV3{
				newRole(func(r *RoleV3) {
					r.Spec.Allow.Logins = []string{"root"}
					r.Spec.Allow.NodeLabels = Labels{"role": []string{"worker"}}
					r.Spec.Options.RequireSessionMFA = true
				}),
				newRole(func(r *RoleV3) {
					r.Spec.Allow.Logins = []string{"root"}
					r.Spec.Options.RequireSessionMFA = false
				}),
			},
			mfaParams: AccessMFAParams{Verified: true},
			checks: []check{
				{server: serverNoLabels, login: "root", hasAccess: true},
				{server: serverWorker, login: "root", hasAccess: true},
				{server: serverDB, login: "root", hasAccess: true},
			},
		},
		{
			name: "cluster requires MFA but MFA was not verified",
			roles: []RoleV3{
				newRole(func(r *RoleV3) {
					r.Spec.Allow.Logins = []string{"root"}
				}),
			},
			mfaParams: AccessMFAParams{Verified: false, AlwaysRequired: true},
			checks: []check{
				{server: serverNoLabels, login: "root", hasAccess: false},
				{server: serverWorker, login: "root", hasAccess: false},
				{server: serverDB, login: "root", hasAccess: false},
			},
		},
		{
			name: "cluster requires MFA and MFA was verified",
			roles: []RoleV3{
				newRole(func(r *RoleV3) {
					r.Spec.Allow.Logins = []string{"root"}
				}),
			},
			mfaParams: AccessMFAParams{Verified: true, AlwaysRequired: true},
			checks: []check{
				{server: serverNoLabels, login: "root", hasAccess: true},
				{server: serverWorker, login: "root", hasAccess: true},
				{server: serverDB, login: "root", hasAccess: true},
			},
		},
	}
	for i, tc := range testCases {

		var set RoleSet
		for i := range tc.roles {
			set = append(set, &tc.roles[i])
		}
		for j, check := range tc.checks {
			comment := fmt.Sprintf("test case %v '%v', check %v", i, tc.name, j)
			result := set.CheckAccessToServer(check.login, check.server, tc.mfaParams)
			if check.hasAccess {
				require.NoError(t, result, comment)
			} else {
				require.True(t, trace.IsAccessDenied(result), comment)
			}
		}
	}
}

func TestCheckAccessToRemoteCluster(t *testing.T) {
	type check struct {
		rc        RemoteCluster
		hasAccess bool
	}
	rcA := &RemoteClusterV3{
		Metadata: Metadata{
			Name: "a",
		},
	}
	rcB := &RemoteClusterV3{
		Metadata: Metadata{
			Name:   "b",
			Labels: map[string]string{"role": "worker", "status": "follower"},
		},
	}
	rcC := &RemoteClusterV3{
		Metadata: Metadata{
			Name:   "c",
			Labels: map[string]string{"role": "db", "status": "follower"},
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
				{rc: rcA, hasAccess: false},
				{rc: rcB, hasAccess: false},
				{rc: rcC, hasAccess: false},
			},
		},
		{
			name: "role matches any label out of multiple labels",
			roles: []RoleV3{
				{
					Metadata: Metadata{
						Name:      "name1",
						Namespace: defaults.Namespace,
					},
					Spec: RoleSpecV3{
						Options: RoleOptions{
							MaxSessionTTL: Duration(20 * time.Hour),
						},
						Allow: RoleConditions{
							Logins:        []string{"admin"},
							ClusterLabels: Labels{"role": []string{"worker2", "worker"}},
							Namespaces:    []string{defaults.Namespace},
						},
					},
				},
			},
			checks: []check{
				{rc: rcA, hasAccess: false},
				{rc: rcB, hasAccess: true},
				{rc: rcC, hasAccess: false},
			},
		},
		{
			name: "wildcard matches anything",
			roles: []RoleV3{
				{
					Metadata: Metadata{
						Name:      "name1",
						Namespace: defaults.Namespace,
					},
					Spec: RoleSpecV3{
						Options: RoleOptions{
							MaxSessionTTL: Duration(20 * time.Hour),
						},
						Allow: RoleConditions{
							Logins:        []string{"admin"},
							ClusterLabels: Labels{Wildcard: []string{Wildcard}},
							Namespaces:    []string{defaults.Namespace},
						},
					},
				},
			},
			checks: []check{
				{rc: rcA, hasAccess: true},
				{rc: rcB, hasAccess: true},
				{rc: rcC, hasAccess: true},
			},
		},
		{
			name: "role with no labels will match clusters with no labels, but no others",
			roles: []RoleV3{
				{
					Metadata: Metadata{
						Name:      "name1",
						Namespace: defaults.Namespace,
					},
					Spec: RoleSpecV3{
						Options: RoleOptions{
							MaxSessionTTL: Duration(20 * time.Hour),
						},
						Allow: RoleConditions{
							Namespaces: []string{defaults.Namespace},
						},
					},
				},
			},
			checks: []check{
				{rc: rcA, hasAccess: true},
				{rc: rcB, hasAccess: false},
				{rc: rcC, hasAccess: false},
			},
		},
		{
			name: "any role in the set with labels in the set makes the set to match labels",
			roles: []RoleV3{
				{
					Metadata: Metadata{
						Name:      "name1",
						Namespace: defaults.Namespace,
					},
					Spec: RoleSpecV3{
						Options: RoleOptions{
							MaxSessionTTL: Duration(20 * time.Hour),
						},
						Allow: RoleConditions{
							ClusterLabels: Labels{"role": []string{"worker"}},
							Namespaces:    []string{defaults.Namespace},
						},
					},
				},
				{
					Metadata: Metadata{
						Name:      "name2",
						Namespace: defaults.Namespace,
					},
					Spec: RoleSpecV3{
						Options: RoleOptions{
							MaxSessionTTL: Duration(20 * time.Hour),
						},
						Allow: RoleConditions{
							Namespaces: []string{defaults.Namespace},
						},
					},
				},
			},
			checks: []check{
				{rc: rcA, hasAccess: false},
				{rc: rcB, hasAccess: true},
				{rc: rcC, hasAccess: false},
			},
		},
		{
			name: "cluster_labels with empty list value matches nothing",
			roles: []RoleV3{
				{
					Metadata: Metadata{
						Name:      "name1",
						Namespace: defaults.Namespace,
					},
					Spec: RoleSpecV3{
						Options: RoleOptions{
							MaxSessionTTL: Duration(20 * time.Hour),
						},
						Allow: RoleConditions{
							Logins:        []string{"admin"},
							ClusterLabels: Labels{"role": []string{}},
							Namespaces:    []string{defaults.Namespace},
						},
					},
				},
			},
			checks: []check{
				{rc: rcA, hasAccess: false},
				{rc: rcB, hasAccess: false},
				{rc: rcC, hasAccess: false},
			},
		},
		{
			name: "one role is more permissive than another",
			roles: []RoleV3{
				{
					Metadata: Metadata{
						Name:      "name1",
						Namespace: defaults.Namespace,
					},
					Spec: RoleSpecV3{
						Options: RoleOptions{
							MaxSessionTTL: Duration(20 * time.Hour),
						},
						Allow: RoleConditions{
							Logins:        []string{"admin"},
							ClusterLabels: Labels{"role": []string{"worker"}},
							Namespaces:    []string{defaults.Namespace},
						},
					},
				},
				{
					Metadata: Metadata{
						Name:      "name2",
						Namespace: defaults.Namespace,
					},
					Spec: RoleSpecV3{
						Options: RoleOptions{
							MaxSessionTTL: Duration(20 * time.Hour),
						},
						Allow: RoleConditions{
							Logins:        []string{"root", "admin"},
							ClusterLabels: Labels{Wildcard: []string{Wildcard}},
							Namespaces:    []string{Wildcard},
						},
					},
				},
			},
			checks: []check{
				{rc: rcA, hasAccess: true},
				{rc: rcB, hasAccess: true},
				{rc: rcC, hasAccess: true},
			},
		},
		{
			name: "regexp label match",
			roles: []RoleV3{
				{
					Metadata: Metadata{
						Name:      "name1",
						Namespace: defaults.Namespace,
					},
					Spec: RoleSpecV3{
						Options: RoleOptions{
							MaxSessionTTL: Duration(20 * time.Hour),
						},
						Allow: RoleConditions{
							Logins:        []string{"admin"},
							ClusterLabels: Labels{"role": []string{"^db(.*)$"}, "status": []string{"follow*"}},
							Namespaces:    []string{defaults.Namespace},
						},
					},
				},
			},
			checks: []check{
				{rc: rcA, hasAccess: false},
				{rc: rcB, hasAccess: false},
				{rc: rcC, hasAccess: true},
			},
		},
	}
	for i, tc := range testCases {
		var set RoleSet
		for i := range tc.roles {
			set = append(set, &tc.roles[i])
		}
		for j, check := range tc.checks {
			comment := fmt.Sprintf("test case %v '%v', check %v", i, tc.name, j)
			result := set.CheckAccessToRemoteCluster(check.rc)
			if check.hasAccess {
				require.NoError(t, result, comment)
			} else {
				require.True(t, trace.IsAccessDenied(result), fmt.Sprintf("%v: %v", comment, result))
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

func TestCheckRuleAccess(t *testing.T) {
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
				{
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
				{
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
				{
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
				{
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
				{
					Metadata: Metadata{
						Name:      "name1",
						Namespace: defaults.Namespace,
					},
					Spec: RoleSpecV3{
						Allow: RoleConditions{
							Namespaces: []string{defaults.Namespace},
							Rules: []Rule{
								{
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
										"group": {"dev", "prod"},
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
										"group": {"dev"},
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
				{
					Metadata: Metadata{
						Name:      "name1",
						Namespace: defaults.Namespace,
					},
					Spec: RoleSpecV3{
						Allow: RoleConditions{
							Namespaces: []string{defaults.Namespace},
							Rules: []Rule{
								{
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
							Resource: &RoleV3{
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
				{
					Metadata: Metadata{
						Name:      "name1",
						Namespace: defaults.Namespace,
					},
					Spec: RoleSpecV3{
						Allow: RoleConditions{
							Namespaces: []string{defaults.Namespace},
							Rules: []Rule{
								{
									Resources: []string{Wildcard},
									Verbs:     []string{Wildcard},
								},
								{
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
							Resource: &RoleV3{
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
					matchBuffer: "more specific rule",
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
			comment := fmt.Sprintf("test case %v '%v', check %v", i, tc.name, j)
			result := set.CheckAccessToRule(&check.context, check.namespace, check.rule, check.verb, false)
			if check.hasAccess {
				require.NoError(t, result, comment)
			} else {
				require.True(t, trace.IsAccessDenied(result), comment)
			}
			if check.matchBuffer != "" {
				require.Contains(t, check.context.buffer.String(), check.matchBuffer, comment)
			}
		}
	}
}

func TestCheckRuleSorting(t *testing.T) {
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
		comment := fmt.Sprintf("test case %v '%v'", i, tc.name)
		out := MakeRuleSet(tc.rules)
		require.Equal(t, tc.set, out, comment)
	}
}

func TestApplyTraits(t *testing.T) {
	type rule struct {
		inLogins       []string
		outLogins      []string
		inLabels       Labels
		outLabels      Labels
		inKubeLabels   Labels
		outKubeLabels  Labels
		inKubeGroups   []string
		outKubeGroups  []string
		inKubeUsers    []string
		outKubeUsers   []string
		inAppLabels    Labels
		outAppLabels   Labels
		inDBLabels     Labels
		outDBLabels    Labels
		inDBNames      []string
		outDBNames     []string
		inDBUsers      []string
		outDBUsers     []string
		inImpersonate  types.ImpersonateConditions
		outImpersonate types.ImpersonateConditions
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
				"foo": {"bar"},
			},
			allow: rule{
				inLogins:  []string{`{{external.foo}}`, "root"},
				outLogins: []string{"bar", "root"},
			},
		},
		{
			comment: "logins substitute in allow rule with function",
			inTraits: map[string][]string{
				"foo": {"Bar <bar@example.com>"},
			},
			allow: rule{
				inLogins:  []string{`{{email.local(external.foo)}}`, "root"},
				outLogins: []string{"bar", "root"},
			},
		},
		{
			comment: "logins substitute in deny rule",
			inTraits: map[string][]string{
				"foo": {"bar"},
			},
			deny: rule{
				inLogins:  []string{`{{external.foo}}`},
				outLogins: []string{"bar"},
			},
		},
		{
			comment: "kube group substitute in allow rule",
			inTraits: map[string][]string{
				"foo": {"bar"},
			},
			allow: rule{
				inKubeGroups:  []string{`{{external.foo}}`, "root"},
				outKubeGroups: []string{"bar", "root"},
			},
		},
		{
			comment: "kube group substitute in deny rule",
			inTraits: map[string][]string{
				"foo": {"bar"},
			},
			deny: rule{
				inKubeGroups:  []string{`{{external.foo}}`, "root"},
				outKubeGroups: []string{"bar", "root"},
			},
		},
		{
			comment: "kube user interpolation in allow rule",
			inTraits: map[string][]string{
				"foo": {"bar"},
			},
			allow: rule{
				inKubeUsers:  []string{`IAM#{{external.foo}};`},
				outKubeUsers: []string{"IAM#bar;"},
			},
		},
		{
			comment: "kube users interpolation in deny rule",
			inTraits: map[string][]string{
				"foo": {"bar"},
			},
			deny: rule{
				inKubeUsers:  []string{`IAM#{{external.foo}};`},
				outKubeUsers: []string{"IAM#bar;"},
			},
		},
		{
			comment: "database name/user external vars in allow rule",
			inTraits: map[string][]string{
				"foo": {"bar"},
			},
			allow: rule{
				inDBNames:  []string{"{{external.foo}}", "{{external.baz}}", "postgres"},
				outDBNames: []string{"bar", "postgres"},
				inDBUsers:  []string{"{{external.foo}}", "{{external.baz}}", "postgres"},
				outDBUsers: []string{"bar", "postgres"},
			},
		},
		{
			comment: "database name/user external vars in deny rule",
			inTraits: map[string][]string{
				"foo": {"bar"},
			},
			deny: rule{
				inDBNames:  []string{"{{external.foo}}", "{{external.baz}}", "postgres"},
				outDBNames: []string{"bar", "postgres"},
				inDBUsers:  []string{"{{external.foo}}", "{{external.baz}}", "postgres"},
				outDBUsers: []string{"bar", "postgres"},
			},
		},
		{
			comment: "database name/user internal vars in allow rule",
			inTraits: map[string][]string{
				"db_names": {"db1", "db2"},
				"db_users": {"alice"},
			},
			allow: rule{
				inDBNames:  []string{"{{internal.db_names}}", "{{internal.foo}}", "postgres"},
				outDBNames: []string{"db1", "db2", "postgres"},
				inDBUsers:  []string{"{{internal.db_users}}", "{{internal.foo}}", "postgres"},
				outDBUsers: []string{"alice", "postgres"},
			},
		},
		{
			comment: "database name/user internal vars in deny rule",
			inTraits: map[string][]string{
				"db_names": {"db1", "db2"},
				"db_users": {"alice"},
			},
			deny: rule{
				inDBNames:  []string{"{{internal.db_names}}", "{{internal.foo}}", "postgres"},
				outDBNames: []string{"db1", "db2", "postgres"},
				inDBUsers:  []string{"{{internal.db_users}}", "{{internal.foo}}", "postgres"},
				outDBUsers: []string{"alice", "postgres"},
			},
		},
		{
			comment: "no variable in logins",
			inTraits: map[string][]string{
				"foo": {"bar"},
			},
			allow: rule{
				inLogins:  []string{"root"},
				outLogins: []string{"root"},
			},
		},
		{
			comment: "invalid variable in logins does not get passed along",
			inTraits: map[string][]string{
				"foo": {"bar"},
			},
			allow: rule{
				inLogins: []string{`external.foo}}`},
			},
		},
		{
			comment: "invalid function call in logins does not get passed along",
			inTraits: map[string][]string{
				"foo": {"bar"},
			},
			allow: rule{
				inLogins: []string{`{{email.local(external.foo, 1)}}`},
			},
		},
		{
			comment: "invalid function call in logins does not get passed along",
			inTraits: map[string][]string{
				"foo": {"bar"},
			},
			allow: rule{
				inLogins: []string{`{{email.local()}}`},
			},
		},
		{
			comment: "invalid function call in logins does not get passed along",
			inTraits: map[string][]string{
				"foo": {"bar"},
			},
			allow: rule{
				inLogins: []string{`{{email.local(email.local)}}`, `{{email.local(email.local())}}`},
			},
		},
		{
			comment: "variable in logins, none in traits",
			inTraits: map[string][]string{
				"foo": {"bar"},
			},
			allow: rule{
				inLogins:  []string{`{{internal.bar}}`, "root"},
				outLogins: []string{"root"},
			},
		},
		{
			comment: "multiple variables in traits",
			inTraits: map[string][]string{
				"logins": {"bar", "baz"},
			},
			allow: rule{
				inLogins:  []string{`{{internal.logins}}`, "root"},
				outLogins: []string{"bar", "baz", "root"},
			},
		},
		{
			comment: "deduplicate",
			inTraits: map[string][]string{
				"foo": {"bar"},
			},
			allow: rule{
				inLogins:  []string{`{{external.foo}}`, "bar"},
				outLogins: []string{"bar"},
			},
		},
		{
			comment: "invalid unix login",
			inTraits: map[string][]string{
				"foo": {"-foo"},
			},
			allow: rule{
				inLogins:  []string{`{{external.foo}}`, "bar"},
				outLogins: []string{"bar"},
			},
		},
		{
			comment: "label substitute in allow and deny rule",
			inTraits: map[string][]string{
				"foo":   {"bar"},
				"hello": {"there"},
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
				"foo": {"bar"},
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
				"foo": {"bar", "baz"},
			},
			allow: rule{
				inLabels:  Labels{`{{external.foo}}`: []string{"value"}},
				outLabels: Labels{`bar`: []string{"value"}},
			},
		},

		{
			comment: "all values are expanded for label values",
			inTraits: map[string][]string{
				"foo": {"bar", "baz"},
			},
			allow: rule{
				inLabels:  Labels{`key`: []string{`{{external.foo}}`}},
				outLabels: Labels{`key`: []string{"bar", "baz"}},
			},
		},
		{
			comment: "values are expanded in kube labels",
			inTraits: map[string][]string{
				"foo": {"bar", "baz"},
			},
			allow: rule{
				inKubeLabels:  Labels{`key`: []string{`{{external.foo}}`}},
				outKubeLabels: Labels{`key`: []string{"bar", "baz"}},
			},
		},
		{
			comment: "values are expanded in app labels",
			inTraits: map[string][]string{
				"foo": {"bar", "baz"},
			},
			allow: rule{
				inAppLabels:  Labels{`key`: []string{`{{external.foo}}`}},
				outAppLabels: Labels{`key`: []string{"bar", "baz"}},
			},
		},
		{
			comment: "values are expanded in database labels",
			inTraits: map[string][]string{
				"foo": {"bar", "baz"},
			},
			allow: rule{
				inDBLabels:  Labels{`key`: []string{`{{external.foo}}`}},
				outDBLabels: Labels{`key`: []string{"bar", "baz"}},
			},
		},

		{
			comment: "impersonate roles",
			inTraits: map[string][]string{
				"teams":         {"devs"},
				"users":         {"alice", "bob"},
				"blocked_users": {"root"},
				"blocked_teams": {"admins"},
			},
			allow: rule{
				inImpersonate: types.ImpersonateConditions{
					Users: []string{"{{external.users}}"},
					Roles: []string{"{{external.teams}}"},
					Where: `contains(user.spec.traits, "hello")`,
				},
				outImpersonate: types.ImpersonateConditions{
					Users: []string{"alice", "bob"},
					Roles: []string{"devs"},
					Where: `contains(user.spec.traits, "hello")`,
				},
			},
			deny: rule{
				inImpersonate: types.ImpersonateConditions{
					Users: []string{"{{external.blocked_users}}"},
					Roles: []string{"{{external.blocked_teams}}"},
				},
				outImpersonate: types.ImpersonateConditions{
					Users: []string{"root"},
					Roles: []string{"admins"},
				},
			},
		},
	}

	for i, tt := range tests {
		comment := fmt.Sprintf("Test %v %v", i, tt.comment)

		role := &RoleV3{
			Kind:    KindRole,
			Version: V3,
			Metadata: Metadata{
				Name:      "name1",
				Namespace: defaults.Namespace,
			},
			Spec: RoleSpecV3{
				Allow: RoleConditions{
					Logins:           tt.allow.inLogins,
					NodeLabels:       tt.allow.inLabels,
					ClusterLabels:    tt.allow.inLabels,
					KubernetesLabels: tt.allow.inKubeLabels,
					KubeGroups:       tt.allow.inKubeGroups,
					KubeUsers:        tt.allow.inKubeUsers,
					AppLabels:        tt.allow.inAppLabels,
					DatabaseLabels:   tt.allow.inDBLabels,
					DatabaseNames:    tt.allow.inDBNames,
					DatabaseUsers:    tt.allow.inDBUsers,
					Impersonate:      &tt.allow.inImpersonate,
				},
				Deny: RoleConditions{
					Logins:           tt.deny.inLogins,
					NodeLabels:       tt.deny.inLabels,
					ClusterLabels:    tt.deny.inLabels,
					KubernetesLabels: tt.deny.inKubeLabels,
					KubeGroups:       tt.deny.inKubeGroups,
					KubeUsers:        tt.deny.inKubeUsers,
					AppLabels:        tt.deny.inAppLabels,
					DatabaseLabels:   tt.deny.inDBLabels,
					DatabaseNames:    tt.deny.inDBNames,
					DatabaseUsers:    tt.deny.inDBUsers,
					Impersonate:      &tt.deny.inImpersonate,
				},
			},
		}

		outRole := ApplyTraits(role, tt.inTraits)
		require.Equal(t, outRole.GetLogins(Allow), tt.allow.outLogins, comment)
		require.Equal(t, outRole.GetNodeLabels(Allow), tt.allow.outLabels, comment)
		require.Equal(t, outRole.GetClusterLabels(Allow), tt.allow.outLabels, comment)
		require.Equal(t, outRole.GetKubernetesLabels(Allow), tt.allow.outKubeLabels, comment)
		require.Equal(t, outRole.GetKubeGroups(Allow), tt.allow.outKubeGroups, comment)
		require.Equal(t, outRole.GetKubeUsers(Allow), tt.allow.outKubeUsers, comment)
		require.Equal(t, outRole.GetAppLabels(Allow), tt.allow.outAppLabels, comment)
		require.Equal(t, outRole.GetDatabaseLabels(Allow), tt.allow.outDBLabels, comment)
		require.Equal(t, outRole.GetDatabaseNames(Allow), tt.allow.outDBNames, comment)
		require.Equal(t, outRole.GetDatabaseUsers(Allow), tt.allow.outDBUsers, comment)
		require.Equal(t, outRole.GetImpersonateConditions(Allow), tt.allow.outImpersonate, comment)

		require.Equal(t, outRole.GetLogins(Deny), tt.deny.outLogins, comment)
		require.Equal(t, outRole.GetNodeLabels(Deny), tt.deny.outLabels, comment)
		require.Equal(t, outRole.GetClusterLabels(Deny), tt.deny.outLabels, comment)
		require.Equal(t, outRole.GetKubernetesLabels(Deny), tt.deny.outKubeLabels, comment)
		require.Equal(t, outRole.GetKubeGroups(Deny), tt.deny.outKubeGroups, comment)
		require.Equal(t, outRole.GetKubeUsers(Deny), tt.deny.outKubeUsers, comment)
		require.Equal(t, outRole.GetAppLabels(Deny), tt.deny.outAppLabels, comment)
		require.Equal(t, outRole.GetDatabaseLabels(Deny), tt.deny.outDBLabels, comment)
		require.Equal(t, outRole.GetDatabaseNames(Deny), tt.deny.outDBNames, comment)
		require.Equal(t, outRole.GetDatabaseUsers(Deny), tt.deny.outDBUsers, comment)
		require.Equal(t, outRole.GetImpersonateConditions(Deny), tt.deny.outImpersonate, comment)
	}
}

// TestBoolOptions makes sure that bool options (like agent forwarding and
// port forwarding) can be disabled in a role.
func TestBoolOptions(t *testing.T) {
	var tests = []struct {
		inOptions           RoleOptions
		outCanPortForward   bool
		outCanForwardAgents bool
	}{
		// Setting options explicitly off should remain off.
		{
			inOptions: RoleOptions{
				ForwardAgent:   NewBool(false),
				PortForwarding: NewBoolOption(false),
			},
			outCanPortForward:   false,
			outCanForwardAgents: false,
		},
		// Not setting options should set port forwarding to true (default enabled)
		// and agent forwarding false (default disabled).
		{
			inOptions:           RoleOptions{},
			outCanPortForward:   true,
			outCanForwardAgents: false,
		},
		// Explicitly enabling should enable them.
		{
			inOptions: RoleOptions{
				ForwardAgent:   NewBool(true),
				PortForwarding: NewBoolOption(true),
			},
			outCanPortForward:   true,
			outCanForwardAgents: true,
		},
	}
	for _, tt := range tests {
		set := NewRoleSet(&RoleV3{
			Kind:    KindRole,
			Version: V3,
			Metadata: Metadata{
				Name:      "role-name",
				Namespace: defaults.Namespace,
			},
			Spec: RoleSpecV3{
				Options: tt.inOptions,
			},
		})
		require.Equal(t, tt.outCanPortForward, set.CanPortForward())
		require.Equal(t, tt.outCanForwardAgents, set.CanForwardAgents())
	}
}

func TestCheckAccessToDatabase(t *testing.T) {
	dbStage := types.NewDatabaseServerV3("stage",
		map[string]string{"env": "stage"},
		types.DatabaseServerSpecV3{})
	dbProd := types.NewDatabaseServerV3("prod",
		map[string]string{"env": "prod"},
		types.DatabaseServerSpecV3{})
	roleDevStage := &RoleV3{
		Metadata: Metadata{Name: "dev-stage", Namespace: defaults.Namespace},
		Spec: RoleSpecV3{
			Allow: RoleConditions{
				Namespaces:     []string{defaults.Namespace},
				DatabaseLabels: Labels{"env": []string{"stage"}},
				DatabaseNames:  []string{Wildcard},
				DatabaseUsers:  []string{Wildcard},
			},
			Deny: RoleConditions{
				Namespaces:    []string{defaults.Namespace},
				DatabaseNames: []string{"supersecret"},
			},
		},
	}
	roleDevProd := &RoleV3{
		Metadata: Metadata{Name: "dev-prod", Namespace: defaults.Namespace},
		Spec: RoleSpecV3{
			Allow: RoleConditions{
				Namespaces:     []string{defaults.Namespace},
				DatabaseLabels: Labels{"env": []string{"prod"}},
				DatabaseNames:  []string{"test"},
				DatabaseUsers:  []string{"dev"},
			},
		},
	}
	roleDevProdWithMFA := &RoleV3{
		Metadata: Metadata{Name: "dev-prod", Namespace: defaults.Namespace},
		Spec: RoleSpecV3{
			Options: types.RoleOptions{
				RequireSessionMFA: true,
			},
			Allow: RoleConditions{
				Namespaces:     []string{defaults.Namespace},
				DatabaseLabels: Labels{"env": []string{"prod"}},
				DatabaseNames:  []string{"test"},
				DatabaseUsers:  []string{"dev"},
			},
		},
	}
	// Database labels are not set in allow/deny rules on purpose to test
	// that they're set during check and set defaults below.
	roleDeny := &types.RoleV3{
		Metadata: Metadata{Name: "deny", Namespace: defaults.Namespace},
		Spec: RoleSpecV3{
			Allow: RoleConditions{
				Namespaces:    []string{defaults.Namespace},
				DatabaseNames: []string{Wildcard},
				DatabaseUsers: []string{Wildcard},
			},
			Deny: RoleConditions{
				Namespaces:    []string{defaults.Namespace},
				DatabaseNames: []string{"postgres"},
				DatabaseUsers: []string{"postgres"},
			},
		},
	}
	require.NoError(t, roleDeny.CheckAndSetDefaults())
	type access struct {
		server types.DatabaseServer
		dbName string
		dbUser string
		access bool
	}
	testCases := []struct {
		name      string
		roles     RoleSet
		access    []access
		mfaParams AccessMFAParams
	}{
		{
			name:  "developer allowed any username/database in stage database except one database",
			roles: RoleSet{roleDevStage, roleDevProd},
			access: []access{
				{server: dbStage, dbName: "superdb", dbUser: "superuser", access: true},
				{server: dbStage, dbName: "test", dbUser: "dev", access: true},
				{server: dbStage, dbName: "supersecret", dbUser: "dev", access: false},
			},
		},
		{
			name:  "developer allowed only specific username/database in prod database",
			roles: RoleSet{roleDevStage, roleDevProd},
			access: []access{
				{server: dbProd, dbName: "superdb", dbUser: "superuser", access: false},
				{server: dbProd, dbName: "test", dbUser: "dev", access: true},
				{server: dbProd, dbName: "superdb", dbUser: "dev", access: false},
				{server: dbProd, dbName: "test", dbUser: "superuser", access: false},
			},
		},
		{
			name:  "deny role denies access to specific database and user",
			roles: RoleSet{roleDeny},
			access: []access{
				{server: dbProd, dbName: "test", dbUser: "test", access: true},
				{server: dbProd, dbName: "postgres", dbUser: "test", access: false},
				{server: dbProd, dbName: "test", dbUser: "postgres", access: false},
			},
		},
		{
			name:      "prod database requires MFA, no MFA provided",
			roles:     RoleSet{roleDevStage, roleDevProdWithMFA, roleDevProd},
			mfaParams: AccessMFAParams{Verified: false},
			access: []access{
				{server: dbStage, dbName: "test", dbUser: "dev", access: true},
				{server: dbProd, dbName: "test", dbUser: "dev", access: false},
			},
		},
		{
			name:      "prod database requires MFA, MFA provided",
			roles:     RoleSet{roleDevStage, roleDevProdWithMFA, roleDevProd},
			mfaParams: AccessMFAParams{Verified: true},
			access: []access{
				{server: dbStage, dbName: "test", dbUser: "dev", access: true},
				{server: dbProd, dbName: "test", dbUser: "dev", access: true},
			},
		},
		{
			name:      "cluster requires MFA, no MFA provided",
			roles:     RoleSet{roleDevStage, roleDevProdWithMFA, roleDevProd},
			mfaParams: AccessMFAParams{Verified: false, AlwaysRequired: true},
			access:    []access{},
		},
		{
			name:      "cluster requires MFA, MFA provided",
			roles:     RoleSet{roleDevStage, roleDevProdWithMFA, roleDevProd},
			mfaParams: AccessMFAParams{Verified: true, AlwaysRequired: true},
			access: []access{
				{server: dbStage, dbName: "test", dbUser: "dev", access: true},
				{server: dbProd, dbName: "test", dbUser: "dev", access: true},
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			for _, access := range tc.access {
				err := tc.roles.CheckAccessToDatabase(access.server, tc.mfaParams,
					&DatabaseLabelsMatcher{Labels: access.server.GetAllLabels()},
					&DatabaseUserMatcher{User: access.dbUser},
					&DatabaseNameMatcher{Name: access.dbName})
				if access.access {
					require.NoError(t, err)
				} else {
					require.Error(t, err)
					require.True(t, trace.IsAccessDenied(err))
				}
			}
		})
	}
}

func TestCheckAccessToDatabaseUser(t *testing.T) {
	dbStage := types.NewDatabaseServerV3("stage",
		map[string]string{"env": "stage"},
		types.DatabaseServerSpecV3{})
	dbProd := types.NewDatabaseServerV3("prod",
		map[string]string{"env": "prod"},
		types.DatabaseServerSpecV3{})
	roleDevStage := &RoleV3{
		Metadata: Metadata{Name: "dev-stage", Namespace: defaults.Namespace},
		Spec: RoleSpecV3{
			Allow: RoleConditions{
				Namespaces:     []string{defaults.Namespace},
				DatabaseLabels: Labels{"env": []string{"stage"}},
				DatabaseUsers:  []string{Wildcard},
			},
			Deny: RoleConditions{
				Namespaces:    []string{defaults.Namespace},
				DatabaseUsers: []string{"superuser"},
			},
		},
	}
	roleDevProd := &RoleV3{
		Metadata: Metadata{Name: "dev-prod", Namespace: defaults.Namespace},
		Spec: RoleSpecV3{
			Allow: RoleConditions{
				Namespaces:     []string{defaults.Namespace},
				DatabaseLabels: Labels{"env": []string{"prod"}},
				DatabaseUsers:  []string{"dev"},
			},
		},
	}
	type access struct {
		server types.DatabaseServer
		dbUser string
		access bool
	}
	testCases := []struct {
		name   string
		roles  RoleSet
		access []access
	}{
		{
			name:  "developer allowed any username in stage database except superuser",
			roles: RoleSet{roleDevStage, roleDevProd},
			access: []access{
				{server: dbStage, dbUser: "superuser", access: false},
				{server: dbStage, dbUser: "dev", access: true},
				{server: dbStage, dbUser: "test", access: true},
			},
		},
		{
			name:  "developer allowed only specific username/database in prod database",
			roles: RoleSet{roleDevStage, roleDevProd},
			access: []access{
				{server: dbProd, dbUser: "superuser", access: false},
				{server: dbProd, dbUser: "dev", access: true},
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			for _, access := range tc.access {
				err := tc.roles.CheckAccessToDatabase(access.server, AccessMFAParams{},
					&DatabaseLabelsMatcher{Labels: access.server.GetAllLabels()},
					&DatabaseUserMatcher{User: access.dbUser})
				if access.access {
					require.NoError(t, err)
				} else {
					require.Error(t, err)
					require.True(t, trace.IsAccessDenied(err))
				}
			}
		})
	}
}

func TestCheckDatabaseNamesAndUsers(t *testing.T) {
	roleEmpty := &RoleV3{
		Metadata: Metadata{Name: "roleA", Namespace: defaults.Namespace},
		Spec: RoleSpecV3{
			Options: RoleOptions{MaxSessionTTL: Duration(time.Hour)},
			Allow: RoleConditions{
				Namespaces: []string{defaults.Namespace},
			},
		},
	}
	roleA := &RoleV3{
		Metadata: Metadata{Name: "roleA", Namespace: defaults.Namespace},
		Spec: RoleSpecV3{
			Options: RoleOptions{MaxSessionTTL: Duration(2 * time.Hour)},
			Allow: RoleConditions{
				Namespaces:    []string{defaults.Namespace},
				DatabaseNames: []string{"postgres", "main"},
				DatabaseUsers: []string{"postgres", "alice"},
			},
		},
	}
	roleB := &RoleV3{
		Metadata: Metadata{Name: "roleB", Namespace: defaults.Namespace},
		Spec: RoleSpecV3{
			Options: RoleOptions{MaxSessionTTL: Duration(time.Hour)},
			Allow: RoleConditions{
				Namespaces:    []string{defaults.Namespace},
				DatabaseNames: []string{"metrics"},
				DatabaseUsers: []string{"bob"},
			},
			Deny: RoleConditions{
				Namespaces:    []string{defaults.Namespace},
				DatabaseNames: []string{"postgres"},
				DatabaseUsers: []string{"postgres"},
			},
		},
	}
	testCases := []struct {
		name         string
		roles        RoleSet
		ttl          time.Duration
		overrideTTL  bool
		namesOut     []string
		usersOut     []string
		accessDenied bool
		notFound     bool
	}{
		{
			name:     "single role",
			roles:    RoleSet{roleA},
			ttl:      time.Hour,
			namesOut: []string{"postgres", "main"},
			usersOut: []string{"postgres", "alice"},
		},
		{
			name:     "combined roles",
			roles:    RoleSet{roleA, roleB},
			ttl:      time.Hour,
			namesOut: []string{"main", "metrics"},
			usersOut: []string{"alice", "bob"},
		},
		{
			name:         "ttl doesn't match",
			roles:        RoleSet{roleA},
			ttl:          5 * time.Hour,
			accessDenied: true,
		},
		{
			name:     "empty role",
			roles:    RoleSet{roleEmpty},
			ttl:      time.Hour,
			notFound: true,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			names, users, err := tc.roles.CheckDatabaseNamesAndUsers(tc.ttl, tc.overrideTTL)
			if tc.accessDenied {
				require.Error(t, err)
				require.True(t, trace.IsAccessDenied(err))
			} else if tc.notFound {
				require.Error(t, err)
				require.True(t, trace.IsNotFound(err))
			} else {
				require.NoError(t, err)
				require.ElementsMatch(t, tc.namesOut, names)
				require.ElementsMatch(t, tc.usersOut, users)
			}
		})
	}
}

func TestCheckAccessToDatabaseService(t *testing.T) {
	dbNoLabels := types.NewDatabaseServerV3("test",
		nil,
		types.DatabaseServerSpecV3{})
	dbStage := types.NewDatabaseServerV3("stage",
		map[string]string{"env": "stage"},
		types.DatabaseServerSpecV3{
			DynamicLabels: map[string]CommandLabelV2{"arch": {Result: "x86"}},
		})
	dbStage2 := types.NewDatabaseServerV3("stage2",
		map[string]string{"env": "stage"},
		types.DatabaseServerSpecV3{
			DynamicLabels: map[string]CommandLabelV2{"arch": {Result: "amd64"}},
		})
	dbProd := types.NewDatabaseServerV3("prod",
		map[string]string{"env": "prod"},
		types.DatabaseServerSpecV3{})
	roleAdmin := &RoleV3{
		Metadata: Metadata{Name: "admin", Namespace: defaults.Namespace},
		Spec: RoleSpecV3{
			Allow: RoleConditions{
				Namespaces:     []string{defaults.Namespace},
				DatabaseLabels: Labels{Wildcard: []string{Wildcard}},
			},
		},
	}
	roleDev := &RoleV3{
		Metadata: Metadata{Name: "dev", Namespace: defaults.Namespace},
		Spec: RoleSpecV3{
			Allow: RoleConditions{
				Namespaces:     []string{defaults.Namespace},
				DatabaseLabels: Labels{"env": []string{"stage"}},
			},
			Deny: RoleConditions{
				Namespaces:     []string{defaults.Namespace},
				DatabaseLabels: Labels{"arch": []string{"amd64"}},
			},
		},
	}
	roleIntern := &RoleV3{
		Metadata: Metadata{Name: "intern", Namespace: defaults.Namespace},
		Spec: RoleSpecV3{
			Allow: RoleConditions{
				Namespaces: []string{defaults.Namespace},
			},
		},
	}
	type access struct {
		server types.DatabaseServer
		access bool
	}
	testCases := []struct {
		name   string
		roles  RoleSet
		access []access
	}{
		{
			name:  "empty role doesn't have access to any databases",
			roles: nil,
			access: []access{
				{server: dbNoLabels, access: false},
				{server: dbStage, access: false},
				{server: dbStage2, access: false},
				{server: dbProd, access: false},
			},
		},
		{
			name:  "intern doesn't have access to any databases",
			roles: RoleSet{roleIntern},
			access: []access{
				{server: dbNoLabels, access: false},
				{server: dbStage, access: false},
				{server: dbStage2, access: false},
				{server: dbProd, access: false},
			},
		},
		{
			name:  "developer only has access to one of stage database",
			roles: RoleSet{roleDev},
			access: []access{
				{server: dbNoLabels, access: false},
				{server: dbStage, access: true},
				{server: dbStage2, access: false},
				{server: dbProd, access: false},
			},
		},
		{
			name:  "admin has access to all databases",
			roles: RoleSet{roleAdmin},
			access: []access{
				{server: dbNoLabels, access: true},
				{server: dbStage, access: true},
				{server: dbStage2, access: true},
				{server: dbProd, access: true},
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			for _, access := range tc.access {
				err := tc.roles.CheckAccessToDatabase(access.server, AccessMFAParams{},
					&DatabaseLabelsMatcher{Labels: access.server.GetAllLabels()})
				if access.access {
					require.NoError(t, err)
				} else {
					require.Error(t, err)
					require.True(t, trace.IsAccessDenied(err))
				}
			}
		})
	}
}

func TestCheckAccessToKubernetes(t *testing.T) {
	clusterNoLabels := &KubernetesCluster{
		Name: "no-labels",
	}
	clusterWithLabels := &KubernetesCluster{
		Name:          "no-labels",
		StaticLabels:  map[string]string{"foo": "bar"},
		DynamicLabels: map[string]CommandLabelV2{"baz": {Result: "qux"}},
	}
	wildcardRole := &RoleV3{
		Metadata: Metadata{
			Name:      "wildcard-labels",
			Namespace: defaults.Namespace,
		},
		Spec: RoleSpecV3{
			Allow: RoleConditions{
				Namespaces:       []string{defaults.Namespace},
				KubernetesLabels: Labels{Wildcard: []string{Wildcard}},
			},
		},
	}
	matchingLabelsRole := &RoleV3{
		Metadata: Metadata{
			Name:      "matching-labels",
			Namespace: defaults.Namespace,
		},
		Spec: RoleSpecV3{
			Allow: RoleConditions{
				Namespaces: []string{defaults.Namespace},
				KubernetesLabels: Labels{
					"foo": utils.Strings{"bar"},
					"baz": utils.Strings{"qux"},
				},
			},
		},
	}
	matchingLabelsRoleWithMFA := &RoleV3{
		Metadata: Metadata{
			Name:      "matching-labels",
			Namespace: defaults.Namespace,
		},
		Spec: RoleSpecV3{
			Options: types.RoleOptions{
				RequireSessionMFA: true,
			},
			Allow: RoleConditions{
				Namespaces: []string{defaults.Namespace},
				KubernetesLabels: Labels{
					"foo": utils.Strings{"bar"},
					"baz": utils.Strings{"qux"},
				},
			},
		},
	}
	noLabelsRole := &RoleV3{
		Metadata: Metadata{
			Name:      "no-labels",
			Namespace: defaults.Namespace,
		},
		Spec: RoleSpecV3{
			Allow: RoleConditions{
				Namespaces: []string{defaults.Namespace},
			},
		},
	}
	mismatchingLabelsRole := &RoleV3{
		Metadata: Metadata{
			Name:      "mismatching-labels",
			Namespace: defaults.Namespace,
		},
		Spec: RoleSpecV3{
			Allow: RoleConditions{
				Namespaces: []string{defaults.Namespace},
				KubernetesLabels: Labels{
					"qux": utils.Strings{"baz"},
					"bar": utils.Strings{"foo"},
				},
			},
		},
	}
	testCases := []struct {
		name      string
		roles     []*RoleV3
		cluster   *KubernetesCluster
		mfaParams AccessMFAParams
		hasAccess bool
	}{
		{
			name:      "empty role set has access to nothing",
			roles:     nil,
			cluster:   clusterNoLabels,
			hasAccess: false,
		},
		{
			name:      "role with no labels has access to nothing",
			roles:     []*RoleV3{noLabelsRole},
			cluster:   clusterNoLabels,
			hasAccess: false,
		},
		{
			name:      "role with wildcard labels matches cluster without labels",
			roles:     []*RoleV3{wildcardRole},
			cluster:   clusterNoLabels,
			hasAccess: true,
		},
		{
			name:      "role with wildcard labels matches cluster with labels",
			roles:     []*RoleV3{wildcardRole},
			cluster:   clusterWithLabels,
			hasAccess: true,
		},
		{
			name:      "role with labels does not match cluster with no labels",
			roles:     []*RoleV3{matchingLabelsRole},
			cluster:   clusterNoLabels,
			hasAccess: false,
		},
		{
			name:      "role with labels matches cluster with labels",
			roles:     []*RoleV3{matchingLabelsRole},
			cluster:   clusterWithLabels,
			hasAccess: true,
		},
		{
			name:      "role with mismatched labels does not match cluster with labels",
			roles:     []*RoleV3{mismatchingLabelsRole},
			cluster:   clusterWithLabels,
			hasAccess: false,
		},
		{
			name:      "one role in the roleset matches",
			roles:     []*RoleV3{mismatchingLabelsRole, noLabelsRole, matchingLabelsRole},
			cluster:   clusterWithLabels,
			hasAccess: true,
		},
		{
			name:      "role requires MFA but MFA not verified",
			roles:     []*RoleV3{matchingLabelsRole, matchingLabelsRoleWithMFA},
			cluster:   clusterWithLabels,
			mfaParams: AccessMFAParams{Verified: false},
			hasAccess: false,
		},
		{
			name:      "role requires MFA and MFA verified",
			roles:     []*RoleV3{matchingLabelsRole, matchingLabelsRoleWithMFA},
			cluster:   clusterWithLabels,
			mfaParams: AccessMFAParams{Verified: true},
			hasAccess: true,
		},
		{
			name:      "cluster requires MFA but MFA not verified",
			roles:     []*RoleV3{matchingLabelsRole},
			cluster:   clusterWithLabels,
			mfaParams: AccessMFAParams{Verified: false, AlwaysRequired: true},
			hasAccess: false,
		},
		{
			name:      "role requires MFA and MFA verified",
			roles:     []*RoleV3{matchingLabelsRole},
			cluster:   clusterWithLabels,
			mfaParams: AccessMFAParams{Verified: true, AlwaysRequired: true},
			hasAccess: true,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var set RoleSet
			for _, r := range tc.roles {
				set = append(set, r)
			}
			err := set.CheckAccessToKubernetes(defaults.Namespace, tc.cluster, tc.mfaParams)
			if tc.hasAccess {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				require.True(t, trace.IsAccessDenied(err))
			}
		})
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
			for login := range allowLogins {
				if err := set.CheckAccessToServer(login, servers[i], AccessMFAParams{}); err != nil {
					b.Error(err)
				}
			}
		}
	}
}

func TestRoleParsing(t *testing.T) {
	testCases := []struct {
		roleMap RoleMap
		err     error
	}{
		{
			roleMap: nil,
		},
		{
			roleMap: RoleMap{
				{Remote: Wildcard, Local: []string{"local-devs", "local-admins"}},
			},
		},
		{
			roleMap: RoleMap{
				{Remote: "remote-devs", Local: []string{"local-devs"}},
			},
		},
		{
			roleMap: RoleMap{
				{Remote: "remote-devs", Local: []string{"local-devs"}},
				{Remote: "remote-devs", Local: []string{"local-devs"}},
			},
			err: trace.BadParameter(""),
		},
		{
			roleMap: RoleMap{
				{Remote: Wildcard, Local: []string{"local-devs"}},
				{Remote: Wildcard, Local: []string{"local-devs"}},
			},
			err: trace.BadParameter(""),
		},
	}

	for i, tc := range testCases {
		comment := fmt.Sprintf("test case '%v'", i)
		t.Run(comment, func(t *testing.T) {
			_, err := parseRoleMap(tc.roleMap)
			if tc.err != nil {
				require.Error(t, err)
				require.IsType(t, tc.err, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestRoleMap(t *testing.T) {
	testCases := []struct {
		remote  []string
		local   []string
		roleMap RoleMap
		name    string
		err     error
	}{
		{
			name:    "all empty",
			remote:  nil,
			local:   nil,
			roleMap: nil,
		},
		{
			name:   "wildcard matches empty as well",
			remote: nil,
			local:  []string{"local-devs", "local-admins"},
			roleMap: RoleMap{
				{Remote: Wildcard, Local: []string{"local-devs", "local-admins"}},
			},
		},
		{
			name:   "direct match",
			remote: []string{"remote-devs"},
			local:  []string{"local-devs"},
			roleMap: RoleMap{
				{Remote: "remote-devs", Local: []string{"local-devs"}},
			},
		},
		{
			name:   "direct match for multiple roles",
			remote: []string{"remote-devs", "remote-logs"},
			local:  []string{"local-devs", "local-logs"},
			roleMap: RoleMap{
				{Remote: "remote-devs", Local: []string{"local-devs"}},
				{Remote: "remote-logs", Local: []string{"local-logs"}},
			},
		},
		{
			name:   "direct match and wildcard",
			remote: []string{"remote-devs"},
			local:  []string{"local-devs", "local-logs"},
			roleMap: RoleMap{
				{Remote: "remote-devs", Local: []string{"local-devs"}},
				{Remote: Wildcard, Local: []string{"local-logs"}},
			},
		},
		{
			name:   "glob capture match",
			remote: []string{"remote-devs"},
			local:  []string{"local-devs"},
			roleMap: RoleMap{
				{Remote: "remote-*", Local: []string{"local-$1"}},
			},
		},
		{
			name:   "passthrough match",
			remote: []string{"remote-devs"},
			local:  []string{"remote-devs"},
			roleMap: RoleMap{
				{Remote: "^(.*)$", Local: []string{"$1"}},
			},
		},
		{
			name:   "passthrough match ignores implicit role",
			remote: []string{"remote-devs", teleport.DefaultImplicitRole},
			local:  []string{"remote-devs"},
			roleMap: RoleMap{
				{Remote: "^(.*)$", Local: []string{"$1"}},
			},
		},
		{
			name:   "partial match",
			remote: []string{"remote-devs", "something-else"},
			local:  []string{"remote-devs"},
			roleMap: RoleMap{
				{Remote: "^(remote-.*)$", Local: []string{"$1"}},
			},
		},
		{
			name:   "partial empty expand section is removed",
			remote: []string{"remote-devs"},
			local:  []string{"remote-devs", "remote-"},
			roleMap: RoleMap{
				{Remote: "^(remote-.*)$", Local: []string{"$1", "remote-$2", "$2"}},
			},
		},
		{
			name:   "multiple matches yield different results",
			remote: []string{"remote-devs"},
			local:  []string{"remote-devs", "test"},
			roleMap: RoleMap{
				{Remote: "^(remote-.*)$", Local: []string{"$1"}},
				{Remote: `^\Aremote-.*$`, Local: []string{"test"}},
			},
		},
		{
			name:   "different expand groups can be referred",
			remote: []string{"remote-devs"},
			local:  []string{"remote-devs", "devs"},
			roleMap: RoleMap{
				{Remote: "^(remote-(.*))$", Local: []string{"$1", "$2"}},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			local, err := MapRoles(tc.roleMap, tc.remote)
			if tc.err != nil {
				require.Error(t, err)
				require.IsType(t, tc.err, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.local, local)
			}
		})
	}
}
