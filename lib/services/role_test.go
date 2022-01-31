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

package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/gravitational/teleport/api/constants"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/wrappers"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/api/utils/sshutils"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/tlsca"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
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
			role := &types.RoleV4{
				Kind:    types.KindRole,
				Version: types.V3,
				Metadata: types.Metadata{
					Name:      fmt.Sprintf("role-%d", i),
					Namespace: apidefaults.Namespace,
				},
				Spec: types.RoleSpecV4{
					Options: types.RoleOptions{
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

func TestRoleParse(t *testing.T) {
	testCases := []struct {
		name         string
		in           string
		role         types.RoleV4
		error        error
		matchMessage string
	}{
		{
			name:  "no input, should not parse",
			in:    ``,
			role:  types.RoleV4{},
			error: trace.BadParameter("empty input"),
		},
		{
			name:  "validation error, no name",
			in:    `{}`,
			role:  types.RoleV4{},
			error: trace.BadParameter("failed to validate: name: name is required"),
		},
		{
			name:  "validation error, no name",
			in:    `{"kind": "role"}`,
			role:  types.RoleV4{},
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
			matchMessage: "missing resources",
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
			matchMessage: "missing verbs",
		},
		{
			name:         "validation error, no version",
			in:           `{"kind": "role", "metadata": {"name": "defrole"}, "spec": {}}`,
			role:         types.RoleV4{},
			error:        trace.BadParameter(""),
			matchMessage: `role version "" is not supported`,
		},
		{
			name:         "validation error, bad version",
			in:           `{"kind": "role", "version": "v2", "metadata": {"name": "defrole"}, "spec": {}}`,
			role:         types.RoleV4{},
			error:        trace.BadParameter(""),
			matchMessage: `role version "v2" is not supported`,
		},
		{
			name: "v3 role with no spec gets v3 defaults",
			in:   `{"kind": "role", "version": "v3", "metadata": {"name": "defrole"}, "spec": {}}`,
			role: types.RoleV4{
				Kind:    types.KindRole,
				Version: types.V3,
				Metadata: types.Metadata{
					Name:      "defrole",
					Namespace: apidefaults.Namespace,
				},
				Spec: types.RoleSpecV4{
					Options: types.RoleOptions{
						CertificateFormat: constants.CertificateFormatStandard,
						MaxSessionTTL:     types.NewDuration(apidefaults.MaxCertDuration),
						PortForwarding:    types.NewBoolOption(true),
						BPF:               apidefaults.EnhancedEvents(),
					},
					Allow: types.RoleConditions{
						NodeLabels:       types.Labels{},
						AppLabels:        types.Labels{types.Wildcard: []string{types.Wildcard}},
						KubernetesLabels: types.Labels{types.Wildcard: []string{types.Wildcard}},
						DatabaseLabels:   types.Labels{types.Wildcard: []string{types.Wildcard}},
						Namespaces:       []string{apidefaults.Namespace},
					},
					Deny: types.RoleConditions{
						Namespaces: []string{apidefaults.Namespace},
					},
				},
			},
			error: nil,
		},
		{
			name: "v4 role gets v4 defaults",
			in:   `{"kind": "role", "version": "v4", "metadata": {"name": "defrole"}, "spec": {}}`,
			role: types.RoleV4{
				Kind:    types.KindRole,
				Version: types.V4,
				Metadata: types.Metadata{
					Name:      "defrole",
					Namespace: apidefaults.Namespace,
				},
				Spec: types.RoleSpecV4{
					Options: types.RoleOptions{
						CertificateFormat: constants.CertificateFormatStandard,
						MaxSessionTTL:     types.NewDuration(apidefaults.MaxCertDuration),
						PortForwarding:    types.NewBoolOption(true),
						BPF:               apidefaults.EnhancedEvents(),
					},
					Allow: types.RoleConditions{
						Namespaces: []string{apidefaults.Namespace},
					},
					Deny: types.RoleConditions{
						Namespaces: []string{apidefaults.Namespace},
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
					   		      "metadata": {"name": "name1", "labels": {"a-b": "c"}},
					   		      "spec": {
					                    "options": {
					                      "cert_format": "standard",
					                      "max_session_ttl": "20h",
					                      "port_forwarding": true,
					                      "client_idle_timeout": "17m",
					                      "disconnect_expired_cert": "yes",
			                              "enhanced_recording": ["command", "network"]
					                    },
					                    "allow": {
					                      "node_labels": {"a": "b", "c-d": "e"},
					                      "app_labels": {"a": "b", "c-d": "e"},
					                      "kubernetes_labels": {"a": "b", "c-d": "e"},
										  "db_labels": {"a": "b", "c-d": "e"},
										  "db_names": ["postgres"],
										  "db_users": ["postgres"],
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
			role: types.RoleV4{
				Kind:    types.KindRole,
				Version: types.V3,
				Metadata: types.Metadata{
					Name:      "name1",
					Namespace: apidefaults.Namespace,
					Labels:    map[string]string{"a-b": "c"},
				},
				Spec: types.RoleSpecV4{
					Options: types.RoleOptions{
						CertificateFormat:     constants.CertificateFormatStandard,
						MaxSessionTTL:         types.NewDuration(20 * time.Hour),
						PortForwarding:        types.NewBoolOption(true),
						ClientIdleTimeout:     types.NewDuration(17 * time.Minute),
						DisconnectExpiredCert: types.NewBool(true),
						BPF:                   apidefaults.EnhancedEvents(),
					},
					Allow: types.RoleConditions{
						NodeLabels:       types.Labels{"a": []string{"b"}, "c-d": []string{"e"}},
						AppLabels:        types.Labels{"a": []string{"b"}, "c-d": []string{"e"}},
						KubernetesLabels: types.Labels{"a": []string{"b"}, "c-d": []string{"e"}},
						DatabaseLabels:   types.Labels{"a": []string{"b"}, "c-d": []string{"e"}},
						DatabaseNames:    []string{"postgres"},
						DatabaseUsers:    []string{"postgres"},
						Namespaces:       []string{"default"},
						Rules: []types.Rule{
							{
								Resources: []string{types.KindRole},
								Verbs:     []string{types.VerbRead, types.VerbList},
								Where:     "contains(user.spec.traits[\"groups\"], \"prod\")",
								Actions: []string{
									"log(\"info\", \"log entry\")",
								},
							},
						},
					},
					Deny: types.RoleConditions{
						Namespaces: []string{apidefaults.Namespace},
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
		                      "disconnect_expired_cert": "no",
		                      "enhanced_recording": ["command", "network"]
		                    },
		                    "allow": {
		                      "node_labels": {"a": "b"},
		                      "app_labels": {"a": "b"},
		                      "kubernetes_labels": {"c": "d"},
		                      "db_labels": {"e": "f"},
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
			role: types.RoleV4{
				Kind:    types.KindRole,
				Version: types.V3,
				Metadata: types.Metadata{
					Name:      "name1",
					Namespace: apidefaults.Namespace,
				},
				Spec: types.RoleSpecV4{
					Options: types.RoleOptions{
						CertificateFormat:     constants.CertificateFormatStandard,
						ForwardAgent:          types.NewBool(true),
						MaxSessionTTL:         types.NewDuration(20 * time.Hour),
						PortForwarding:        types.NewBoolOption(true),
						ClientIdleTimeout:     types.NewDuration(0),
						DisconnectExpiredCert: types.NewBool(false),
						BPF:                   apidefaults.EnhancedEvents(),
					},
					Allow: types.RoleConditions{
						NodeLabels:       types.Labels{"a": []string{"b"}},
						AppLabels:        types.Labels{"a": []string{"b"}},
						KubernetesLabels: types.Labels{"c": []string{"d"}},
						DatabaseLabels:   types.Labels{"e": []string{"f"}},
						Namespaces:       []string{"default"},
						Rules: []types.Rule{
							{
								Resources: []string{types.KindRole},
								Verbs:     []string{types.VerbRead, types.VerbList},
								Where:     "contains(user.spec.traits[\"groups\"], \"prod\")",
								Actions: []string{
									"log(\"info\", \"log entry\")",
								},
							},
						},
					},
					Deny: types.RoleConditions{
						Namespaces: []string{apidefaults.Namespace},
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
		                      "disconnect_expired_cert": "no",
		                      "enhanced_recording": ["command", "network"]
		                    },
		                    "allow": {
		                      "node_labels": {"a": "b", "key": ["val"], "key2": ["val2", "val3"]},
		                      "app_labels": {"a": "b", "key": ["val"], "key2": ["val2", "val3"]},
		                      "kubernetes_labels": {"a": "b", "key": ["val"], "key2": ["val2", "val3"]},
		                      "db_labels": {"a": "b", "key": ["val"], "key2": ["val2", "val3"]}
		                    },
		                    "deny": {
		                      "logins": ["c"]
		                    }
		   		      }
		   		    }`,
			role: types.RoleV4{
				Kind:    types.KindRole,
				Version: types.V3,
				Metadata: types.Metadata{
					Name:      "name1",
					Namespace: apidefaults.Namespace,
				},
				Spec: types.RoleSpecV4{
					Options: types.RoleOptions{
						CertificateFormat:     constants.CertificateFormatStandard,
						ForwardAgent:          types.NewBool(true),
						MaxSessionTTL:         types.NewDuration(20 * time.Hour),
						PortForwarding:        types.NewBoolOption(true),
						ClientIdleTimeout:     types.NewDuration(0),
						DisconnectExpiredCert: types.NewBool(false),
						BPF:                   apidefaults.EnhancedEvents(),
					},
					Allow: types.RoleConditions{
						NodeLabels: types.Labels{
							"a":    []string{"b"},
							"key":  []string{"val"},
							"key2": []string{"val2", "val3"},
						},
						AppLabels: types.Labels{
							"a":    []string{"b"},
							"key":  []string{"val"},
							"key2": []string{"val2", "val3"},
						},
						KubernetesLabels: types.Labels{
							"a":    []string{"b"},
							"key":  []string{"val"},
							"key2": []string{"val2", "val3"},
						},
						DatabaseLabels: types.Labels{
							"a":    []string{"b"},
							"key":  []string{"val"},
							"key2": []string{"val2", "val3"},
						},
						Namespaces: []string{"default"},
					},
					Deny: types.RoleConditions{
						Namespaces: []string{apidefaults.Namespace},
						Logins:     []string{"c"},
					},
				},
			},
			error: nil,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			role, err := UnmarshalRole([]byte(tc.in))
			if tc.error != nil {
				require.Error(t, err)
				if tc.matchMessage != "" {
					require.Contains(t, err.Error(), tc.matchMessage)
				}
			} else {
				require.NoError(t, err)
				require.Equal(t, &tc.role, role)

				err := ValidateRole(role)
				require.NoError(t, err)

				out, err := json.Marshal(role)
				require.NoError(t, err)

				role2, err := UnmarshalRole(out)
				require.NoError(t, err)
				require.Equal(t, role2, &tc.role)
			}
		})
	}
}

func TestValidateRole(t *testing.T) {
	var tests = []struct {
		name         string
		spec         types.RoleSpecV4
		err          error
		matchMessage string
	}{
		{
			name: "valid syntax",
			spec: types.RoleSpecV4{
				Allow: types.RoleConditions{
					Logins: []string{`{{external["http://schemas.microsoft.com/ws/2008/06/identity/claims/windowsaccountname"]}}`},
				},
			},
		},
		{
			name: "invalid role condition login syntax",
			spec: types.RoleSpecV4{
				Allow: types.RoleConditions{
					Logins: []string{"{{foo"},
				},
			},
			err:          trace.BadParameter(""),
			matchMessage: "invalid login found",
		},
		{
			name: "unsupported function in actions",
			spec: types.RoleSpecV4{
				Allow: types.RoleConditions{
					Logins: []string{`{{external["http://schemas.microsoft.com/ws/2008/06/identity/claims/windowsaccountname"]}}`},
					Rules: []types.Rule{
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
			spec: types.RoleSpecV4{
				Allow: types.RoleConditions{
					Logins: []string{`{{external["http://schemas.microsoft.com/ws/2008/06/identity/claims/windowsaccountname"]}}`},
					Rules: []types.Rule{
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
		err := ValidateRole(&types.RoleV4{
			Metadata: types.Metadata{
				Name:      "name1",
				Namespace: apidefaults.Namespace,
			},
			Version: types.V3,
			Spec:    tc.spec,
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

func TestValidateRoleName(t *testing.T) {
	var tests = []struct {
		name         string
		roleName     string
		err          error
		matchMessage string
	}{
		{
			name:         "reserved role name proxy",
			roleName:     string(types.RoleProxy),
			err:          trace.BadParameter(""),
			matchMessage: fmt.Sprintf("reserved role: %s", types.RoleProxy),
		},
		{
			name:     "valid role name test-1",
			roleName: "test-1",
		},
	}

	for _, tc := range tests {
		err := ValidateRoleName(&types.RoleV4{Metadata: types.Metadata{
			Name: tc.roleName,
		}})
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
	labels := types.Labels{
		"key": []string{"val"},
	}
	data, err := json.Marshal(labels)
	require.NoError(t, err)

	var out map[string]string
	err = json.Unmarshal(data, &out)
	require.NoError(t, err)
	require.Equal(t, map[string]string{"key": "val"}, out)
}

func newRole(mut func(*types.RoleV4)) types.RoleV4 {
	r := types.RoleV4{
		Metadata: types.Metadata{
			Name:      "name",
			Namespace: apidefaults.Namespace,
		},
		Spec: types.RoleSpecV4{
			Options: types.RoleOptions{
				MaxSessionTTL: types.Duration(20 * time.Hour),
			},
			Allow: types.RoleConditions{
				NodeLabels:           types.Labels{types.Wildcard: []string{types.Wildcard}},
				WindowsDesktopLabels: types.Labels{types.Wildcard: []string{types.Wildcard}},
				Namespaces:           []string{types.Wildcard},
			},
		},
	}
	mut(&r)
	return r
}

func TestCheckAccessToServer(t *testing.T) {
	type check struct {
		server    types.Server
		hasAccess bool
		login     string
	}
	serverNoLabels := &types.ServerV2{
		Kind: types.KindNode,
		Metadata: types.Metadata{
			Name: "a",
		},
	}
	serverWorker := &types.ServerV2{
		Kind: types.KindNode,
		Metadata: types.Metadata{
			Name:      "b",
			Namespace: apidefaults.Namespace,
			Labels:    map[string]string{"role": "worker", "status": "follower"},
		},
	}
	namespaceC := "namespace-c"
	serverDB := &types.ServerV2{
		Kind: types.KindNode,
		Metadata: types.Metadata{
			Name:      "c",
			Namespace: namespaceC,
			Labels:    map[string]string{"role": "db", "status": "follower"},
		},
	}
	serverDBWithSuffix := &types.ServerV2{
		Kind: types.KindNode,
		Metadata: types.Metadata{
			Name:      "c2",
			Namespace: namespaceC,
			Labels:    map[string]string{"role": "db01", "status": "follower01"},
		},
	}
	testCases := []struct {
		name      string
		roles     []types.RoleV4
		checks    []check
		mfaParams AccessMFAParams
	}{
		{
			name:  "empty role set has access to nothing",
			roles: []types.RoleV4{},
			checks: []check{
				{server: serverNoLabels, login: "root", hasAccess: false},
				{server: serverWorker, login: "root", hasAccess: false},
				{server: serverDB, login: "root", hasAccess: false},
			},
		},
		{
			name: "role is limited to default namespace",
			roles: []types.RoleV4{
				newRole(func(r *types.RoleV4) {
					r.Spec.Allow.Logins = []string{"admin"}
					r.Spec.Allow.Namespaces = []string{apidefaults.Namespace}
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
			roles: []types.RoleV4{
				newRole(func(r *types.RoleV4) {
					r.Spec.Allow.Logins = []string{"admin"}
					r.Spec.Allow.NodeLabels = types.Labels{"role": []string{"worker"}}
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
			roles: []types.RoleV4{
				newRole(func(r *types.RoleV4) {
					r.Spec.Allow.Logins = []string{"admin"}
					r.Spec.Allow.NodeLabels = types.Labels{"role": []string{"worker2", "worker"}}
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
			roles: []types.RoleV4{
				newRole(func(r *types.RoleV4) {
					r.Spec.Allow.Logins = []string{"admin"}
					r.Spec.Allow.NodeLabels = types.Labels{"role": []string{}}
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
			roles: []types.RoleV4{
				newRole(func(r *types.RoleV4) {
					r.Spec.Allow.Logins = []string{"admin"}
					r.Spec.Allow.Namespaces = []string{apidefaults.Namespace}
					r.Spec.Allow.NodeLabels = types.Labels{"role": []string{"worker"}}
				}),
				newRole(func(r *types.RoleV4) {
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
			roles: []types.RoleV4{
				newRole(func(r *types.RoleV4) {
					r.Spec.Allow.Logins = []string{"admin"}
					r.Spec.Allow.NodeLabels = types.Labels{"role": []string{"^db(.*)$"}, "status": []string{"follow*"}}
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
			roles: []types.RoleV4{
				newRole(func(r *types.RoleV4) {
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
			roles: []types.RoleV4{
				newRole(func(r *types.RoleV4) {
					r.Spec.Allow.Logins = []string{"root"}
					r.Spec.Allow.NodeLabels = types.Labels{"role": []string{"worker"}}
					r.Spec.Options.RequireSessionMFA = true
				}),
				newRole(func(r *types.RoleV4) {
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
			roles: []types.RoleV4{
				newRole(func(r *types.RoleV4) {
					r.Spec.Allow.Logins = []string{"root"}
					r.Spec.Allow.NodeLabels = types.Labels{"role": []string{"worker"}}
					r.Spec.Options.RequireSessionMFA = true
				}),
				newRole(func(r *types.RoleV4) {
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
			roles: []types.RoleV4{
				newRole(func(r *types.RoleV4) {
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
			roles: []types.RoleV4{
				newRole(func(r *types.RoleV4) {
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
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var set RoleSet
			for i := range tc.roles {
				set = append(set, &tc.roles[i])
			}
			for j, check := range tc.checks {
				comment := fmt.Sprintf("check %v: user: %v, server: %v, should access: %v", j, check.login, check.server.GetName(), check.hasAccess)
				err := set.CheckAccess(
					check.server,
					tc.mfaParams,
					NewLoginMatcher(check.login))
				if check.hasAccess {
					require.NoError(t, err, comment)
				} else {
					require.True(t, trace.IsAccessDenied(err), comment)
				}
			}
		})
	}
}

func TestCheckAccessToRemoteCluster(t *testing.T) {
	type check struct {
		rc        types.RemoteCluster
		hasAccess bool
	}
	rcA := &types.RemoteClusterV3{
		Metadata: types.Metadata{
			Name: "a",
		},
	}
	rcB := &types.RemoteClusterV3{
		Metadata: types.Metadata{
			Name:   "b",
			Labels: map[string]string{"role": "worker", "status": "follower"},
		},
	}
	rcC := &types.RemoteClusterV3{
		Metadata: types.Metadata{
			Name:   "c",
			Labels: map[string]string{"role": "db", "status": "follower"},
		},
	}
	testCases := []struct {
		name   string
		roles  []types.RoleV4
		checks []check
	}{
		{
			name:  "empty role set has access to nothing",
			roles: []types.RoleV4{},
			checks: []check{
				{rc: rcA, hasAccess: false},
				{rc: rcB, hasAccess: false},
				{rc: rcC, hasAccess: false},
			},
		},
		{
			name: "role matches any label out of multiple labels",
			roles: []types.RoleV4{
				{
					Metadata: types.Metadata{
						Name:      "name1",
						Namespace: apidefaults.Namespace,
					},
					Spec: types.RoleSpecV4{
						Options: types.RoleOptions{
							MaxSessionTTL: types.Duration(20 * time.Hour),
						},
						Allow: types.RoleConditions{
							Logins:        []string{"admin"},
							ClusterLabels: types.Labels{"role": []string{"worker2", "worker"}},
							Namespaces:    []string{apidefaults.Namespace},
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
			roles: []types.RoleV4{
				{
					Metadata: types.Metadata{
						Name:      "name1",
						Namespace: apidefaults.Namespace,
					},
					Spec: types.RoleSpecV4{
						Options: types.RoleOptions{
							MaxSessionTTL: types.Duration(20 * time.Hour),
						},
						Allow: types.RoleConditions{
							Logins:        []string{"admin"},
							ClusterLabels: types.Labels{types.Wildcard: []string{types.Wildcard}},
							Namespaces:    []string{apidefaults.Namespace},
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
			roles: []types.RoleV4{
				{
					Metadata: types.Metadata{
						Name:      "name1",
						Namespace: apidefaults.Namespace,
					},
					Spec: types.RoleSpecV4{
						Options: types.RoleOptions{
							MaxSessionTTL: types.Duration(20 * time.Hour),
						},
						Allow: types.RoleConditions{
							Namespaces: []string{apidefaults.Namespace},
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
			roles: []types.RoleV4{
				{
					Metadata: types.Metadata{
						Name:      "name1",
						Namespace: apidefaults.Namespace,
					},
					Spec: types.RoleSpecV4{
						Options: types.RoleOptions{
							MaxSessionTTL: types.Duration(20 * time.Hour),
						},
						Allow: types.RoleConditions{
							ClusterLabels: types.Labels{"role": []string{"worker"}},
							Namespaces:    []string{apidefaults.Namespace},
						},
					},
				},
				{
					Metadata: types.Metadata{
						Name:      "name2",
						Namespace: apidefaults.Namespace,
					},
					Spec: types.RoleSpecV4{
						Options: types.RoleOptions{
							MaxSessionTTL: types.Duration(20 * time.Hour),
						},
						Allow: types.RoleConditions{
							Namespaces: []string{apidefaults.Namespace},
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
			roles: []types.RoleV4{
				{
					Metadata: types.Metadata{
						Name:      "name1",
						Namespace: apidefaults.Namespace,
					},
					Spec: types.RoleSpecV4{
						Options: types.RoleOptions{
							MaxSessionTTL: types.Duration(20 * time.Hour),
						},
						Allow: types.RoleConditions{
							Logins:        []string{"admin"},
							ClusterLabels: types.Labels{"role": []string{}},
							Namespaces:    []string{apidefaults.Namespace},
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
			roles: []types.RoleV4{
				{
					Metadata: types.Metadata{
						Name:      "name1",
						Namespace: apidefaults.Namespace,
					},
					Spec: types.RoleSpecV4{
						Options: types.RoleOptions{
							MaxSessionTTL: types.Duration(20 * time.Hour),
						},
						Allow: types.RoleConditions{
							Logins:        []string{"admin"},
							ClusterLabels: types.Labels{"role": []string{"worker"}},
							Namespaces:    []string{apidefaults.Namespace},
						},
					},
				},
				{
					Metadata: types.Metadata{
						Name:      "name2",
						Namespace: apidefaults.Namespace,
					},
					Spec: types.RoleSpecV4{
						Options: types.RoleOptions{
							MaxSessionTTL: types.Duration(20 * time.Hour),
						},
						Allow: types.RoleConditions{
							Logins:        []string{"root", "admin"},
							ClusterLabels: types.Labels{types.Wildcard: []string{types.Wildcard}},
							Namespaces:    []string{types.Wildcard},
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
			roles: []types.RoleV4{
				{
					Metadata: types.Metadata{
						Name:      "name1",
						Namespace: apidefaults.Namespace,
					},
					Spec: types.RoleSpecV4{
						Options: types.RoleOptions{
							MaxSessionTTL: types.Duration(20 * time.Hour),
						},
						Allow: types.RoleConditions{
							Logins:        []string{"admin"},
							ClusterLabels: types.Labels{"role": []string{"^db(.*)$"}, "status": []string{"follow*"}},
							Namespaces:    []string{apidefaults.Namespace},
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
		roles  []types.RoleV4
		checks []check
	}{
		{
			name:  "0 - empty role set has access to nothing",
			roles: []types.RoleV4{},
			checks: []check{
				{rule: types.KindUser, verb: types.ActionWrite, namespace: apidefaults.Namespace, hasAccess: false},
			},
		},
		{
			name: "1 - user can read session but can't list in default namespace",
			roles: []types.RoleV4{
				{
					Metadata: types.Metadata{
						Name:      "name1",
						Namespace: apidefaults.Namespace,
					},
					Spec: types.RoleSpecV4{
						Allow: types.RoleConditions{
							Namespaces: []string{apidefaults.Namespace},
							Rules: []types.Rule{
								types.NewRule(types.KindSSHSession, []string{types.VerbRead}),
							},
						},
					},
				},
			},
			checks: []check{
				{rule: types.KindSSHSession, verb: types.VerbRead, namespace: apidefaults.Namespace, hasAccess: true},
				{rule: types.KindSSHSession, verb: types.VerbList, namespace: apidefaults.Namespace, hasAccess: false},
			},
		},
		{
			name: "2 - user can read sessions in system namespace and create stuff in default namespace",
			roles: []types.RoleV4{
				{
					Metadata: types.Metadata{
						Name:      "name1",
						Namespace: apidefaults.Namespace,
					},
					Spec: types.RoleSpecV4{
						Allow: types.RoleConditions{
							Namespaces: []string{"system"},
							Rules: []types.Rule{
								types.NewRule(types.KindSSHSession, []string{types.VerbRead}),
							},
						},
					},
				},
				{
					Metadata: types.Metadata{
						Name:      "name2",
						Namespace: apidefaults.Namespace,
					},
					Spec: types.RoleSpecV4{
						Allow: types.RoleConditions{
							Namespaces: []string{apidefaults.Namespace},
							Rules: []types.Rule{
								types.NewRule(types.KindSSHSession, []string{types.VerbCreate, types.VerbRead}),
							},
						},
					},
				},
			},
			checks: []check{
				{rule: types.KindSSHSession, verb: types.VerbRead, namespace: apidefaults.Namespace, hasAccess: true},
				{rule: types.KindSSHSession, verb: types.VerbCreate, namespace: apidefaults.Namespace, hasAccess: true},
				{rule: types.KindSSHSession, verb: types.VerbCreate, namespace: "system", hasAccess: false},
				{rule: types.KindRole, verb: types.VerbRead, namespace: apidefaults.Namespace, hasAccess: false},
			},
		},
		{
			name: "3 - deny rules override allow rules",
			roles: []types.RoleV4{
				{
					Metadata: types.Metadata{
						Name:      "name1",
						Namespace: apidefaults.Namespace,
					},
					Spec: types.RoleSpecV4{
						Deny: types.RoleConditions{
							Namespaces: []string{apidefaults.Namespace},
							Rules: []types.Rule{
								types.NewRule(types.KindSSHSession, []string{types.VerbCreate}),
							},
						},
						Allow: types.RoleConditions{
							Namespaces: []string{apidefaults.Namespace},
							Rules: []types.Rule{
								types.NewRule(types.KindSSHSession, []string{types.VerbCreate}),
							},
						},
					},
				},
			},
			checks: []check{
				{rule: types.KindSSHSession, verb: types.VerbCreate, namespace: apidefaults.Namespace, hasAccess: false},
			},
		},
		{
			name: "4 - user can read sessions if trait matches",
			roles: []types.RoleV4{
				{
					Metadata: types.Metadata{
						Name:      "name1",
						Namespace: apidefaults.Namespace,
					},
					Spec: types.RoleSpecV4{
						Allow: types.RoleConditions{
							Namespaces: []string{apidefaults.Namespace},
							Rules: []types.Rule{
								{
									Resources: []string{types.KindSession},
									Verbs:     []string{types.VerbRead},
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
				{rule: types.KindSession, verb: types.VerbRead, namespace: apidefaults.Namespace, hasAccess: false},
				{rule: types.KindSession, verb: types.VerbList, namespace: apidefaults.Namespace, hasAccess: false},
				{
					context: testContext{
						buffer: &bytes.Buffer{},
						Context: Context{
							User: &types.UserV2{
								Metadata: types.Metadata{
									Name: "bob",
								},
								Spec: types.UserSpecV2{
									Traits: map[string][]string{
										"group": {"dev", "prod"},
									},
								},
							},
						},
					},
					rule:      types.KindSession,
					verb:      types.VerbRead,
					namespace: apidefaults.Namespace,
					hasAccess: true,
				},
				{
					context: testContext{
						buffer: &bytes.Buffer{},
						Context: Context{
							User: &types.UserV2{
								Spec: types.UserSpecV2{
									Traits: map[string][]string{
										"group": {"dev"},
									},
								},
							},
						},
					},
					rule:      types.KindSession,
					verb:      types.VerbRead,
					namespace: apidefaults.Namespace,
					hasAccess: false,
				},
			},
		},
		{
			name: "5 - user can read role if role has label",
			roles: []types.RoleV4{
				{
					Metadata: types.Metadata{
						Name:      "name1",
						Namespace: apidefaults.Namespace,
					},
					Spec: types.RoleSpecV4{
						Allow: types.RoleConditions{
							Namespaces: []string{apidefaults.Namespace},
							Rules: []types.Rule{
								{
									Resources: []string{types.KindRole},
									Verbs:     []string{types.VerbRead},
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
				{rule: types.KindRole, verb: types.VerbRead, namespace: apidefaults.Namespace, hasAccess: false},
				{rule: types.KindRole, verb: types.VerbList, namespace: apidefaults.Namespace, hasAccess: false},
				{
					context: testContext{
						buffer: &bytes.Buffer{},
						Context: Context{
							Resource: &types.RoleV4{
								Metadata: types.Metadata{
									Labels: map[string]string{"team": "dev"},
								},
							},
						},
					},
					rule:      types.KindRole,
					verb:      types.VerbRead,
					namespace: apidefaults.Namespace,
					hasAccess: true,
				},
			},
		},
		{
			name: "More specific rule wins",
			roles: []types.RoleV4{
				{
					Metadata: types.Metadata{
						Name:      "name1",
						Namespace: apidefaults.Namespace,
					},
					Spec: types.RoleSpecV4{
						Allow: types.RoleConditions{
							Namespaces: []string{apidefaults.Namespace},
							Rules: []types.Rule{
								{
									Resources: []string{types.Wildcard},
									Verbs:     []string{types.Wildcard},
								},
								{
									Resources: []string{types.KindRole},
									Verbs:     []string{types.VerbRead},
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
							Resource: &types.RoleV4{
								Metadata: types.Metadata{
									Labels: map[string]string{"team": "dev"},
								},
							},
						},
					},
					rule:        types.KindRole,
					verb:        types.VerbRead,
					namespace:   apidefaults.Namespace,
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

func TestGuessIfAccessIsPossible(t *testing.T) {
	// Examples from https://goteleport.com/docs/access-controls/reference/#rbac-for-sessions.
	ownSessions, err := types.NewRole("own-sessions", types.RoleSpecV4{
		Allow: types.RoleConditions{
			Rules: []types.Rule{
				{
					Resources: []string{types.KindSession},
					Verbs:     []string{types.VerbList, types.VerbRead},
					Where:     "contains(session.participants, user.metadata.name)",
				},
			},
		},
	})
	require.NoError(t, err)
	ownSSHSessions, err := types.NewRole("own-ssh-sessions", types.RoleSpecV4{
		Allow: types.RoleConditions{
			Rules: []types.Rule{
				{
					Resources: []string{types.KindSSHSession},
					Verbs:     []string{types.Wildcard},
				},
			},
		},
		Deny: types.RoleConditions{
			Rules: []types.Rule{
				{
					Resources: []string{types.KindSSHSession},
					Verbs:     []string{types.VerbList, types.VerbRead, types.VerbUpdate, types.VerbDelete},
					Where:     "!contains(ssh_session.participants, user.metadata.name)",
				},
			},
		},
	})
	require.NoError(t, err)

	// Simple, all-or-nothing roles.
	readAllSessions, err := types.NewRole("all-sessions", types.RoleSpecV4{
		Allow: types.RoleConditions{
			Rules: []types.Rule{
				{
					Resources: []string{types.KindSession},
					Verbs:     []string{types.VerbList, types.VerbRead},
				},
			},
		},
	})
	require.NoError(t, err)
	allowSSHSessions, err := types.NewRole("all-ssh-sessions", types.RoleSpecV4{
		Allow: types.RoleConditions{
			Rules: []types.Rule{
				{
					Resources: []string{types.KindSSHSession},
					Verbs:     []string{types.Wildcard},
				},
			},
		},
	})
	require.NoError(t, err)
	denySSHSessions, err := types.NewRole("deny-ssh-sessions", types.RoleSpecV4{
		Deny: types.RoleConditions{
			Rules: []types.Rule{
				{
					Resources: []string{types.KindSSHSession},
					Verbs:     []string{types.Wildcard},
				},
			},
		},
	})
	require.NoError(t, err)

	type checkAccessParams struct {
		ctx       Context
		namespace string
		resource  string
		verbs     []string
	}

	tests := []struct {
		name   string
		roles  RoleSet
		params *checkAccessParams
		// wantRuleAccess fully evaluates where conditions, used to determine access
		// to specific resources.
		wantRuleAccess bool
		// wantGuessAccess doesn't evaluate where conditions, used to determine
		// access to a category of resources.
		wantGuessAccess bool
	}{
		{
			name:  "global session list/read allowed",
			roles: RoleSet{readAllSessions},
			params: &checkAccessParams{
				namespace: apidefaults.Namespace,
				resource:  types.KindSession,
				verbs:     []string{types.VerbList, types.VerbRead},
			},
			wantRuleAccess:  true,
			wantGuessAccess: true,
		},
		{
			name:  "own session list/read allowed",
			roles: RoleSet{ownSessions}, // allowed despite "where" clause in allow rules
			params: &checkAccessParams{
				namespace: apidefaults.Namespace,
				resource:  types.KindSession,
				verbs:     []string{types.VerbList, types.VerbRead},
			},
			wantRuleAccess:  false, // where condition needs specific resource
			wantGuessAccess: true,
		},
		{
			name:  "session list/read denied",
			roles: RoleSet{allowSSHSessions, denySSHSessions}, // none mention "session"
			params: &checkAccessParams{
				namespace: apidefaults.Namespace,
				resource:  types.KindSession,
				verbs:     []string{types.VerbList, types.VerbRead},
			},
		},
		{
			name: "session write denied",
			roles: RoleSet{
				readAllSessions,                   // readonly
				allowSSHSessions, denySSHSessions, // none mention "session"
			},
			params: &checkAccessParams{
				namespace: apidefaults.Namespace,
				resource:  types.KindSession,
				verbs:     []string{types.VerbUpdate, types.VerbDelete},
			},
		},
		{
			name:  "global SSH session list/read allowed",
			roles: RoleSet{allowSSHSessions},
			params: &checkAccessParams{
				namespace: apidefaults.Namespace,
				resource:  types.KindSSHSession,
				verbs:     []string{types.VerbList, types.VerbRead},
			},
			wantRuleAccess:  true,
			wantGuessAccess: true,
		},
		{
			name:  "own SSH session list/read allowed",
			roles: RoleSet{ownSSHSessions}, // allowed despite "where" clause in deny rules
			params: &checkAccessParams{
				namespace: apidefaults.Namespace,
				resource:  types.KindSSHSession,
				verbs:     []string{types.VerbList, types.VerbRead},
			},
			wantRuleAccess:  false, // where condition needs specific resource
			wantGuessAccess: true,
		},
		{
			name: "SSH session list/read denied",
			roles: RoleSet{
				allowSSHSessions, ownSSHSessions,
				denySSHSessions, // unconditional deny, takes precedence
			},
			params: &checkAccessParams{
				namespace: apidefaults.Namespace,
				resource:  types.KindSSHSession,
				verbs:     []string{types.VerbCreate, types.VerbList, types.VerbRead, types.VerbUpdate, types.VerbDelete},
			},
		},
		{
			name: "SSH session list/read denied - different role ordering",
			roles: RoleSet{
				allowSSHSessions,
				denySSHSessions, // unconditional deny, takes precedence
				ownSSHSessions,
			},
			params: &checkAccessParams{
				namespace: apidefaults.Namespace,
				resource:  types.KindSSHSession,
				verbs:     []string{types.VerbCreate, types.VerbList, types.VerbRead, types.VerbUpdate, types.VerbDelete},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			params := test.params
			const silent = true
			for _, verb := range params.verbs {
				err := test.roles.CheckAccessToRule(&params.ctx, params.namespace, params.resource, verb, silent)
				if gotAccess, wantAccess := err == nil, test.wantRuleAccess; gotAccess != wantAccess {
					t.Errorf("CheckAccessToRule(verb=%q) returned err = %v=q, wantAccess = %v", verb, err, wantAccess)
				}

				err = test.roles.GuessIfAccessIsPossible(&params.ctx, params.namespace, params.resource, verb, silent)
				if gotAccess, wantAccess := err == nil, test.wantGuessAccess; gotAccess != wantAccess {
					t.Errorf("GuessIfAccessIsPossible(verb=%q) returned err = %q, wantAccess = %v", verb, err, wantAccess)
				}
			}
		})
	}
}

func TestCheckRuleSorting(t *testing.T) {
	testCases := []struct {
		name  string
		rules []types.Rule
		set   RuleSet
	}{
		{
			name: "single rule set sorts OK",
			rules: []types.Rule{
				{
					Resources: []string{types.KindUser},
					Verbs:     []string{types.VerbCreate},
				},
			},
			set: RuleSet{
				types.KindUser: []types.Rule{
					{
						Resources: []string{types.KindUser},
						Verbs:     []string{types.VerbCreate},
					},
				},
			},
		},
		{
			name: "rule with where section is more specific",
			rules: []types.Rule{
				{
					Resources: []string{types.KindUser},
					Verbs:     []string{types.VerbCreate},
				},
				{
					Resources: []string{types.KindUser},
					Verbs:     []string{types.VerbCreate},
					Where:     "contains(user.spec.traits[\"groups\"], \"prod\")",
				},
			},
			set: RuleSet{
				types.KindUser: []types.Rule{
					{
						Resources: []string{types.KindUser},
						Verbs:     []string{types.VerbCreate},
						Where:     "contains(user.spec.traits[\"groups\"], \"prod\")",
					},
					{
						Resources: []string{types.KindUser},
						Verbs:     []string{types.VerbCreate},
					},
				},
			},
		},
		{
			name: "rule with action is more specific",
			rules: []types.Rule{
				{
					Resources: []string{types.KindUser},
					Verbs:     []string{types.VerbCreate},

					Where: "contains(user.spec.traits[\"groups\"], \"prod\")",
				},
				{
					Resources: []string{types.KindUser},
					Verbs:     []string{types.VerbCreate},
					Where:     "contains(user.spec.traits[\"groups\"], \"prod\")",
					Actions: []string{
						"log(\"info\", \"log entry\")",
					},
				},
			},
			set: RuleSet{
				types.KindUser: []types.Rule{
					{
						Resources: []string{types.KindUser},
						Verbs:     []string{types.VerbCreate},
						Where:     "contains(user.spec.traits[\"groups\"], \"prod\")",
						Actions: []string{
							"log(\"info\", \"log entry\")",
						},
					},
					{
						Resources: []string{types.KindUser},
						Verbs:     []string{types.VerbCreate},
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
		inLogins         []string
		outLogins        []string
		inWindowsLogins  []string
		outWindowsLogins []string
		inRoleARNs       []string
		outRoleARNs      []string
		inLabels         types.Labels
		outLabels        types.Labels
		inKubeLabels     types.Labels
		outKubeLabels    types.Labels
		inKubeGroups     []string
		outKubeGroups    []string
		inKubeUsers      []string
		outKubeUsers     []string
		inAppLabels      types.Labels
		outAppLabels     types.Labels
		inDBLabels       types.Labels
		outDBLabels      types.Labels
		inDBNames        []string
		outDBNames       []string
		inDBUsers        []string
		outDBUsers       []string
		inImpersonate    types.ImpersonateConditions
		outImpersonate   types.ImpersonateConditions
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
			comment: "logins substitute in allow rule with regexp",
			inTraits: map[string][]string{
				"foo": {"bar-baz"},
			},
			allow: rule{
				inLogins:  []string{`{{regexp.replace(external.foo, "^bar-(.*)$", "$1")}}`, "root"},
				outLogins: []string{"baz", "root"},
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
			comment: "windows logins substitute",
			inTraits: map[string][]string{
				"windows_logins": {"user"},
				"foo":            {"bar"},
			},
			allow: rule{
				inWindowsLogins:  []string{"{{internal.windows_logins}}"},
				outWindowsLogins: []string{"user"},
			},
			deny: rule{
				inWindowsLogins:  []string{"{{external.foo}}"},
				outWindowsLogins: []string{"bar"},
			},
		},
		{
			comment: "invalid windows login",
			inTraits: map[string][]string{
				"windows_logins": {"test;"},
			},
			allow: rule{
				inWindowsLogins:  []string{"Administrator", "{{internal.windows_logins}}"},
				outWindowsLogins: []string{"Administrator"},
			},
		},
		{
			comment: "AWS role ARN substitute in allow rule",
			inTraits: map[string][]string{
				"foo": {"bar"},
			},
			allow: rule{
				inRoleARNs:  []string{"{{external.foo}}"},
				outRoleARNs: []string{"bar"},
			},
		},
		{
			comment: "AWS role ARN substitute in deny rule",
			inTraits: map[string][]string{
				"foo": {"bar"},
			},
			deny: rule{
				inRoleARNs:  []string{"{{external.foo}}"},
				outRoleARNs: []string{"bar"},
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
			comment: "kube user regexp interpolation in allow rule",
			inTraits: map[string][]string{
				"foo": {"bar-baz"},
			},
			allow: rule{
				inKubeUsers:  []string{`IAM#{{regexp.replace(external.foo, "^bar-(.*)$", "$1")}};`},
				outKubeUsers: []string{"IAM#baz;"},
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
			comment: "invalid regexp in logins does not get passed along",
			inTraits: map[string][]string{
				"foo": {"bar"},
			},
			allow: rule{
				inLogins: []string{`{{regexp.replace(external.foo, "(()", "baz")}}`},
			},
		},
		{
			comment: "logins which to not match regexp get filtered out",
			inTraits: map[string][]string{
				"foo": {"dev-alice", "dev-bob", "prod-charlie"},
			},
			allow: rule{
				inLogins:  []string{`{{regexp.replace(external.foo, "^dev-([a-zA-Z]+)$", "$1-admin")}}`},
				outLogins: []string{"alice-admin", "bob-admin"},
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
				inLabels:  types.Labels{`{{external.foo}}`: []string{"{{external.hello}}"}},
				outLabels: types.Labels{`bar`: []string{"there"}},
			},
			deny: rule{
				inLabels:  types.Labels{`{{external.hello}}`: []string{"{{external.foo}}"}},
				outLabels: types.Labels{`there`: []string{"bar"}},
			},
		},

		{
			comment: "missing node variables are set to empty during substitution",
			inTraits: map[string][]string{
				"foo": {"bar"},
			},
			allow: rule{
				inLabels: types.Labels{
					`{{external.foo}}`:     []string{"value"},
					`{{external.missing}}`: []string{"missing"},
					`missing`:              []string{"{{external.missing}}", "othervalue"},
				},
				outLabels: types.Labels{
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
				inLabels:  types.Labels{`{{external.foo}}`: []string{"value"}},
				outLabels: types.Labels{`bar`: []string{"value"}},
			},
		},

		{
			comment: "all values are expanded for label values",
			inTraits: map[string][]string{
				"foo": {"bar", "baz"},
			},
			allow: rule{
				inLabels:  types.Labels{`key`: []string{`{{external.foo}}`}},
				outLabels: types.Labels{`key`: []string{"bar", "baz"}},
			},
		},
		{
			comment: "values are expanded in kube labels",
			inTraits: map[string][]string{
				"foo": {"bar", "baz"},
			},
			allow: rule{
				inKubeLabels:  types.Labels{`key`: []string{`{{external.foo}}`}},
				outKubeLabels: types.Labels{`key`: []string{"bar", "baz"}},
			},
		},
		{
			comment: "values are expanded in app labels",
			inTraits: map[string][]string{
				"foo": {"bar", "baz"},
			},
			allow: rule{
				inAppLabels:  types.Labels{`key`: []string{`{{external.foo}}`}},
				outAppLabels: types.Labels{`key`: []string{"bar", "baz"}},
			},
		},
		{
			comment: "values are expanded in database labels",
			inTraits: map[string][]string{
				"foo": {"bar", "baz"},
			},
			allow: rule{
				inDBLabels:  types.Labels{`key`: []string{`{{external.foo}}`}},
				outDBLabels: types.Labels{`key`: []string{"bar", "baz"}},
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

	for _, tt := range tests {
		t.Run(tt.comment, func(t *testing.T) {
			role := &types.RoleV4{
				Kind:    types.KindRole,
				Version: types.V3,
				Metadata: types.Metadata{
					Name:      "name1",
					Namespace: apidefaults.Namespace,
				},
				Spec: types.RoleSpecV4{
					Allow: types.RoleConditions{
						Logins:               tt.allow.inLogins,
						WindowsDesktopLogins: tt.allow.inWindowsLogins,
						NodeLabels:           tt.allow.inLabels,
						ClusterLabels:        tt.allow.inLabels,
						KubernetesLabels:     tt.allow.inKubeLabels,
						KubeGroups:           tt.allow.inKubeGroups,
						KubeUsers:            tt.allow.inKubeUsers,
						AppLabels:            tt.allow.inAppLabels,
						DatabaseLabels:       tt.allow.inDBLabels,
						DatabaseNames:        tt.allow.inDBNames,
						DatabaseUsers:        tt.allow.inDBUsers,
						Impersonate:          &tt.allow.inImpersonate,
					},
					Deny: types.RoleConditions{
						Logins:               tt.deny.inLogins,
						WindowsDesktopLogins: tt.deny.inWindowsLogins,
						NodeLabels:           tt.deny.inLabels,
						ClusterLabels:        tt.deny.inLabels,
						KubernetesLabels:     tt.deny.inKubeLabels,
						KubeGroups:           tt.deny.inKubeGroups,
						KubeUsers:            tt.deny.inKubeUsers,
						AppLabels:            tt.deny.inAppLabels,
						DatabaseLabels:       tt.deny.inDBLabels,
						DatabaseNames:        tt.deny.inDBNames,
						DatabaseUsers:        tt.deny.inDBUsers,
						Impersonate:          &tt.deny.inImpersonate,
					},
				},
			}

			outRole := ApplyTraits(role, tt.inTraits)
			rules := []struct {
				condition types.RoleConditionType
				spec      *rule
			}{
				{types.Allow, &tt.allow},
				{types.Deny, &tt.deny},
			}
			for _, rule := range rules {
				require.Equal(t, rule.spec.outLogins, outRole.GetLogins(rule.condition))
				require.Equal(t, rule.spec.outWindowsLogins, outRole.GetWindowsLogins(rule.condition))
				require.Equal(t, rule.spec.outLabels, outRole.GetNodeLabels(rule.condition))
				require.Equal(t, rule.spec.outLabels, outRole.GetClusterLabels(rule.condition))
				require.Equal(t, rule.spec.outKubeLabels, outRole.GetKubernetesLabels(rule.condition))
				require.Equal(t, rule.spec.outKubeGroups, outRole.GetKubeGroups(rule.condition))
				require.Equal(t, rule.spec.outKubeUsers, outRole.GetKubeUsers(rule.condition))
				require.Equal(t, rule.spec.outAppLabels, outRole.GetAppLabels(rule.condition))
				require.Equal(t, rule.spec.outDBLabels, outRole.GetDatabaseLabels(rule.condition))
				require.Equal(t, rule.spec.outDBNames, outRole.GetDatabaseNames(rule.condition))
				require.Equal(t, rule.spec.outDBUsers, outRole.GetDatabaseUsers(rule.condition))
				require.Equal(t, rule.spec.outImpersonate, outRole.GetImpersonateConditions(rule.condition))
			}
		})
	}
}

// TestExtractFrom makes sure roles and traits are extracted from SSH and TLS
// certificates not services.User.
func TestExtractFrom(t *testing.T) {
	origRoles := []string{"admin"}
	origTraits := wrappers.Traits(map[string][]string{
		"login": {"foo"},
	})

	// Create a SSH certificate.
	cert, err := sshutils.ParseCertificate([]byte(fixtures.UserCertificateStandard))
	require.NoError(t, err)

	// Create a TLS identity.
	identity := &tlsca.Identity{
		Username: "foo",
		Groups:   origRoles,
		Traits:   origTraits,
	}

	// At this point, services.User and the certificate/identity are still in
	// sync. The roles and traits returned should be the same as the original.
	roles, traits, err := ExtractFromCertificate(cert)
	require.NoError(t, err)
	require.Equal(t, roles, origRoles)
	require.Equal(t, traits, origTraits)

	roles, traits, err = ExtractFromIdentity(&userGetter{
		roles:  origRoles,
		traits: origTraits,
	}, *identity)
	require.NoError(t, err)
	require.Equal(t, roles, origRoles)
	require.Equal(t, traits, origTraits)

	// The backend now returns new roles and traits, however because the roles
	// and traits are extracted from the certificate/identity, the original
	// roles and traits will be returned.
	roles, traits, err = ExtractFromCertificate(cert)
	require.NoError(t, err)
	require.Equal(t, roles, origRoles)
	require.Equal(t, traits, origTraits)

	roles, traits, err = ExtractFromIdentity(&userGetter{
		roles:  origRoles,
		traits: origTraits,
	}, *identity)
	require.NoError(t, err)
	require.Equal(t, roles, origRoles)
	require.Equal(t, traits, origTraits)
}

// TestBoolOptions makes sure that bool options (like agent forwarding and
// port forwarding) can be disabled in a role.
func TestBoolOptions(t *testing.T) {
	var tests = []struct {
		inOptions           types.RoleOptions
		outCanPortForward   bool
		outCanForwardAgents bool
	}{
		// Setting options explicitly off should remain off.
		{
			inOptions: types.RoleOptions{
				ForwardAgent:   types.NewBool(false),
				PortForwarding: types.NewBoolOption(false),
			},
			outCanPortForward:   false,
			outCanForwardAgents: false,
		},
		// Not setting options should set port forwarding to true (default enabled)
		// and agent forwarding false (default disabled).
		{
			inOptions:           types.RoleOptions{},
			outCanPortForward:   true,
			outCanForwardAgents: false,
		},
		// Explicitly enabling should enable them.
		{
			inOptions: types.RoleOptions{
				ForwardAgent:   types.NewBool(true),
				PortForwarding: types.NewBoolOption(true),
			},
			outCanPortForward:   true,
			outCanForwardAgents: true,
		},
	}
	for _, tt := range tests {
		set := NewRoleSet(&types.RoleV4{
			Kind:    types.KindRole,
			Version: types.V3,
			Metadata: types.Metadata{
				Name:      "role-name",
				Namespace: apidefaults.Namespace,
			},
			Spec: types.RoleSpecV4{
				Options: tt.inOptions,
			},
		})
		require.Equal(t, tt.outCanPortForward, set.CanPortForward())
		require.Equal(t, tt.outCanForwardAgents, set.CanForwardAgents())
	}
}

func TestCheckAccessToDatabase(t *testing.T) {
	dbStage, err := types.NewDatabaseV3(types.Metadata{
		Name:   "stage",
		Labels: map[string]string{"env": "stage"},
	}, types.DatabaseSpecV3{
		Protocol: "protocol",
		URI:      "uri",
	})
	require.NoError(t, err)
	dbProd, err := types.NewDatabaseV3(types.Metadata{
		Name:   "prod",
		Labels: map[string]string{"env": "prod"},
	}, types.DatabaseSpecV3{
		Protocol: "protocol",
		URI:      "uri",
	})
	require.NoError(t, err)
	roleDevStage := &types.RoleV4{
		Metadata: types.Metadata{Name: "dev-stage", Namespace: apidefaults.Namespace},
		Version:  types.V3,
		Spec: types.RoleSpecV4{
			Allow: types.RoleConditions{
				Namespaces:     []string{apidefaults.Namespace},
				DatabaseLabels: types.Labels{"env": []string{"stage"}},
				DatabaseNames:  []string{types.Wildcard},
				DatabaseUsers:  []string{types.Wildcard},
			},
			Deny: types.RoleConditions{
				Namespaces:    []string{apidefaults.Namespace},
				DatabaseNames: []string{"supersecret"},
			},
		},
	}
	require.NoError(t, roleDevStage.CheckAndSetDefaults())
	roleDevProd := &types.RoleV4{
		Metadata: types.Metadata{Name: "dev-prod", Namespace: apidefaults.Namespace},
		Version:  types.V3,
		Spec: types.RoleSpecV4{
			Allow: types.RoleConditions{
				Namespaces:     []string{apidefaults.Namespace},
				DatabaseLabels: types.Labels{"env": []string{"prod"}},
				DatabaseNames:  []string{"test"},
				DatabaseUsers:  []string{"dev"},
			},
		},
	}
	require.NoError(t, roleDevProd.CheckAndSetDefaults())
	roleDevProdWithMFA := &types.RoleV4{
		Metadata: types.Metadata{Name: "dev-prod", Namespace: apidefaults.Namespace},
		Version:  types.V3,
		Spec: types.RoleSpecV4{
			Options: types.RoleOptions{
				RequireSessionMFA: true,
			},
			Allow: types.RoleConditions{
				Namespaces:     []string{apidefaults.Namespace},
				DatabaseLabels: types.Labels{"env": []string{"prod"}},
				DatabaseNames:  []string{"test"},
				DatabaseUsers:  []string{"dev"},
			},
		},
	}
	require.NoError(t, roleDevProdWithMFA.CheckAndSetDefaults())
	// Database labels are not set in allow/deny rules on purpose to test
	// that they're set during check and set defaults below.
	roleDeny := &types.RoleV4{
		Metadata: types.Metadata{Name: "deny", Namespace: apidefaults.Namespace},
		Version:  types.V3,
		Spec: types.RoleSpecV4{
			Allow: types.RoleConditions{
				Namespaces:    []string{apidefaults.Namespace},
				DatabaseNames: []string{types.Wildcard},
				DatabaseUsers: []string{types.Wildcard},
			},
			Deny: types.RoleConditions{
				Namespaces:    []string{apidefaults.Namespace},
				DatabaseNames: []string{"postgres"},
				DatabaseUsers: []string{"postgres"},
			},
		},
	}
	require.NoError(t, roleDeny.CheckAndSetDefaults())
	type access struct {
		server types.Database
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
				err := tc.roles.CheckAccess(access.server, tc.mfaParams,
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
	dbStage, err := types.NewDatabaseV3(types.Metadata{
		Name:   "stage",
		Labels: map[string]string{"env": "stage"},
	}, types.DatabaseSpecV3{
		Protocol: "protocol",
		URI:      "uri",
	})
	require.NoError(t, err)
	dbProd, err := types.NewDatabaseV3(types.Metadata{
		Name:   "prod",
		Labels: map[string]string{"env": "prod"},
	}, types.DatabaseSpecV3{
		Protocol: "protocol",
		URI:      "uri",
	})
	require.NoError(t, err)
	roleDevStage := &types.RoleV4{
		Metadata: types.Metadata{Name: "dev-stage", Namespace: apidefaults.Namespace},
		Spec: types.RoleSpecV4{
			Allow: types.RoleConditions{
				Namespaces:     []string{apidefaults.Namespace},
				DatabaseLabels: types.Labels{"env": []string{"stage"}},
				DatabaseUsers:  []string{types.Wildcard},
			},
			Deny: types.RoleConditions{
				Namespaces:    []string{apidefaults.Namespace},
				DatabaseUsers: []string{"superuser"},
			},
		},
	}
	roleDevProd := &types.RoleV4{
		Metadata: types.Metadata{Name: "dev-prod", Namespace: apidefaults.Namespace},
		Spec: types.RoleSpecV4{
			Allow: types.RoleConditions{
				Namespaces:     []string{apidefaults.Namespace},
				DatabaseLabels: types.Labels{"env": []string{"prod"}},
				DatabaseUsers:  []string{"dev"},
			},
		},
	}
	type access struct {
		server types.Database
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
				err := tc.roles.CheckAccess(access.server, AccessMFAParams{}, &DatabaseUserMatcher{User: access.dbUser})
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
	roleEmpty := &types.RoleV4{
		Metadata: types.Metadata{Name: "roleA", Namespace: apidefaults.Namespace},
		Spec: types.RoleSpecV4{
			Options: types.RoleOptions{MaxSessionTTL: types.Duration(time.Hour)},
			Allow: types.RoleConditions{
				Namespaces: []string{apidefaults.Namespace},
			},
		},
	}
	roleA := &types.RoleV4{
		Metadata: types.Metadata{Name: "roleA", Namespace: apidefaults.Namespace},
		Spec: types.RoleSpecV4{
			Options: types.RoleOptions{MaxSessionTTL: types.Duration(2 * time.Hour)},
			Allow: types.RoleConditions{
				Namespaces:    []string{apidefaults.Namespace},
				DatabaseNames: []string{"postgres", "main"},
				DatabaseUsers: []string{"postgres", "alice"},
			},
		},
	}
	roleB := &types.RoleV4{
		Metadata: types.Metadata{Name: "roleB", Namespace: apidefaults.Namespace},
		Spec: types.RoleSpecV4{
			Options: types.RoleOptions{MaxSessionTTL: types.Duration(time.Hour)},
			Allow: types.RoleConditions{
				Namespaces:    []string{apidefaults.Namespace},
				DatabaseNames: []string{"metrics"},
				DatabaseUsers: []string{"bob"},
			},
			Deny: types.RoleConditions{
				Namespaces:    []string{apidefaults.Namespace},
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
	dbNoLabels, err := types.NewDatabaseV3(types.Metadata{
		Name: "test",
	}, types.DatabaseSpecV3{
		Protocol: "protocol",
		URI:      "uri",
	})
	require.NoError(t, err)
	dbStage, err := types.NewDatabaseV3(types.Metadata{
		Name:   "stage",
		Labels: map[string]string{"env": "stage"},
	}, types.DatabaseSpecV3{
		Protocol:      "protocol",
		URI:           "uri",
		DynamicLabels: map[string]types.CommandLabelV2{"arch": {Result: "x86"}},
	})
	require.NoError(t, err)
	dbStage2, err := types.NewDatabaseV3(types.Metadata{
		Name:   "stage2",
		Labels: map[string]string{"env": "stage"},
	}, types.DatabaseSpecV3{
		Protocol:      "protocol",
		URI:           "uri",
		DynamicLabels: map[string]types.CommandLabelV2{"arch": {Result: "amd64"}},
	})
	require.NoError(t, err)
	dbProd, err := types.NewDatabaseV3(types.Metadata{
		Name:   "prod",
		Labels: map[string]string{"env": "prod"},
	}, types.DatabaseSpecV3{
		Protocol: "protocol",
		URI:      "uri",
	})
	require.NoError(t, err)
	roleAdmin := &types.RoleV4{
		Metadata: types.Metadata{Name: "admin", Namespace: apidefaults.Namespace},
		Spec: types.RoleSpecV4{
			Allow: types.RoleConditions{
				Namespaces:     []string{apidefaults.Namespace},
				DatabaseLabels: types.Labels{types.Wildcard: []string{types.Wildcard}},
			},
		},
	}
	roleDev := &types.RoleV4{
		Metadata: types.Metadata{Name: "dev", Namespace: apidefaults.Namespace},
		Spec: types.RoleSpecV4{
			Allow: types.RoleConditions{
				Namespaces:     []string{apidefaults.Namespace},
				DatabaseLabels: types.Labels{"env": []string{"stage"}},
			},
			Deny: types.RoleConditions{
				Namespaces:     []string{apidefaults.Namespace},
				DatabaseLabels: types.Labels{"arch": []string{"amd64"}},
			},
		},
	}
	roleIntern := &types.RoleV4{
		Metadata: types.Metadata{Name: "intern", Namespace: apidefaults.Namespace},
		Spec: types.RoleSpecV4{
			Allow: types.RoleConditions{
				Namespaces: []string{apidefaults.Namespace},
			},
		},
	}
	type access struct {
		server types.Database
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
				err := tc.roles.CheckAccess(access.server, AccessMFAParams{})
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

// TestCheckAccessToAWSConsole verifies AWS role ARNs access checker.
func TestCheckAccessToAWSConsole(t *testing.T) {
	app, err := types.NewAppV3(types.Metadata{
		Name: "awsconsole",
	}, types.AppSpecV3{
		URI: constants.AWSConsoleURL,
	})
	require.NoError(t, err)
	readOnlyARN := "readonly"
	fullAccessARN := "fullaccess"
	roleNoAccess := &types.RoleV4{
		Metadata: types.Metadata{
			Name:      "noaccess",
			Namespace: apidefaults.Namespace,
		},
		Spec: types.RoleSpecV4{
			Allow: types.RoleConditions{
				Namespaces:  []string{apidefaults.Namespace},
				AppLabels:   types.Labels{types.Wildcard: []string{types.Wildcard}},
				AWSRoleARNs: []string{},
			},
		},
	}
	roleReadOnly := &types.RoleV4{
		Metadata: types.Metadata{
			Name:      "readonly",
			Namespace: apidefaults.Namespace,
		},
		Spec: types.RoleSpecV4{
			Allow: types.RoleConditions{
				Namespaces:  []string{apidefaults.Namespace},
				AppLabels:   types.Labels{types.Wildcard: []string{types.Wildcard}},
				AWSRoleARNs: []string{readOnlyARN},
			},
		},
	}
	roleFullAccess := &types.RoleV4{
		Metadata: types.Metadata{
			Name:      "fullaccess",
			Namespace: apidefaults.Namespace,
		},
		Spec: types.RoleSpecV4{
			Allow: types.RoleConditions{
				Namespaces:  []string{apidefaults.Namespace},
				AppLabels:   types.Labels{types.Wildcard: []string{types.Wildcard}},
				AWSRoleARNs: []string{readOnlyARN, fullAccessARN},
			},
		},
	}
	type access struct {
		roleARN   string
		hasAccess bool
	}
	tests := []struct {
		name   string
		roles  RoleSet
		access []access
	}{
		{
			name:  "empty role set",
			roles: nil,
			access: []access{
				{roleARN: readOnlyARN, hasAccess: false},
				{roleARN: fullAccessARN, hasAccess: false},
			},
		},
		{
			name:  "no access role",
			roles: RoleSet{roleNoAccess},
			access: []access{
				{roleARN: readOnlyARN, hasAccess: false},
				{roleARN: fullAccessARN, hasAccess: false},
			},
		},
		{
			name:  "readonly role",
			roles: RoleSet{roleReadOnly},
			access: []access{
				{roleARN: readOnlyARN, hasAccess: true},
				{roleARN: fullAccessARN, hasAccess: false},
			},
		},
		{
			name:  "full access role",
			roles: RoleSet{roleFullAccess},
			access: []access{
				{roleARN: readOnlyARN, hasAccess: true},
				{roleARN: fullAccessARN, hasAccess: true},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			for _, access := range test.access {
				err := test.roles.CheckAccess(
					app,
					AccessMFAParams{},
					&AWSRoleARNMatcher{RoleARN: access.roleARN})
				if access.hasAccess {
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
	clusterNoLabels := &types.KubernetesCluster{
		Name: "no-labels",
	}
	clusterWithLabels := &types.KubernetesCluster{
		Name:          "no-labels",
		StaticLabels:  map[string]string{"foo": "bar"},
		DynamicLabels: map[string]types.CommandLabelV2{"baz": {Result: "qux"}},
	}
	wildcardRole := &types.RoleV4{
		Metadata: types.Metadata{
			Name:      "wildcard-labels",
			Namespace: apidefaults.Namespace,
		},
		Spec: types.RoleSpecV4{
			Allow: types.RoleConditions{
				Namespaces:       []string{apidefaults.Namespace},
				KubernetesLabels: types.Labels{types.Wildcard: []string{types.Wildcard}},
			},
		},
	}
	matchingLabelsRole := &types.RoleV4{
		Metadata: types.Metadata{
			Name:      "matching-labels",
			Namespace: apidefaults.Namespace,
		},
		Spec: types.RoleSpecV4{
			Allow: types.RoleConditions{
				Namespaces: []string{apidefaults.Namespace},
				KubernetesLabels: types.Labels{
					"foo": apiutils.Strings{"bar"},
					"baz": apiutils.Strings{"qux"},
				},
			},
		},
	}
	matchingLabelsRoleWithMFA := &types.RoleV4{
		Metadata: types.Metadata{
			Name:      "matching-labels",
			Namespace: apidefaults.Namespace,
		},
		Spec: types.RoleSpecV4{
			Options: types.RoleOptions{
				RequireSessionMFA: true,
			},
			Allow: types.RoleConditions{
				Namespaces: []string{apidefaults.Namespace},
				KubernetesLabels: types.Labels{
					"foo": apiutils.Strings{"bar"},
					"baz": apiutils.Strings{"qux"},
				},
			},
		},
	}
	noLabelsRole := &types.RoleV4{
		Metadata: types.Metadata{
			Name:      "no-labels",
			Namespace: apidefaults.Namespace,
		},
		Spec: types.RoleSpecV4{
			Allow: types.RoleConditions{
				Namespaces: []string{apidefaults.Namespace},
			},
		},
	}
	mismatchingLabelsRole := &types.RoleV4{
		Metadata: types.Metadata{
			Name:      "mismatching-labels",
			Namespace: apidefaults.Namespace,
		},
		Spec: types.RoleSpecV4{
			Allow: types.RoleConditions{
				Namespaces: []string{apidefaults.Namespace},
				KubernetesLabels: types.Labels{
					"qux": apiutils.Strings{"baz"},
					"bar": apiutils.Strings{"foo"},
				},
			},
		},
	}
	testCases := []struct {
		name      string
		roles     []*types.RoleV4
		cluster   *types.KubernetesCluster
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
			roles:     []*types.RoleV4{noLabelsRole},
			cluster:   clusterNoLabels,
			hasAccess: false,
		},
		{
			name:      "role with wildcard labels matches cluster without labels",
			roles:     []*types.RoleV4{wildcardRole},
			cluster:   clusterNoLabels,
			hasAccess: true,
		},
		{
			name:      "role with wildcard labels matches cluster with labels",
			roles:     []*types.RoleV4{wildcardRole},
			cluster:   clusterWithLabels,
			hasAccess: true,
		},
		{
			name:      "role with labels does not match cluster with no labels",
			roles:     []*types.RoleV4{matchingLabelsRole},
			cluster:   clusterNoLabels,
			hasAccess: false,
		},
		{
			name:      "role with labels matches cluster with labels",
			roles:     []*types.RoleV4{matchingLabelsRole},
			cluster:   clusterWithLabels,
			hasAccess: true,
		},
		{
			name:      "role with mismatched labels does not match cluster with labels",
			roles:     []*types.RoleV4{mismatchingLabelsRole},
			cluster:   clusterWithLabels,
			hasAccess: false,
		},
		{
			name:      "one role in the roleset matches",
			roles:     []*types.RoleV4{mismatchingLabelsRole, noLabelsRole, matchingLabelsRole},
			cluster:   clusterWithLabels,
			hasAccess: true,
		},
		{
			name:      "role requires MFA but MFA not verified",
			roles:     []*types.RoleV4{matchingLabelsRole, matchingLabelsRoleWithMFA},
			cluster:   clusterWithLabels,
			mfaParams: AccessMFAParams{Verified: false},
			hasAccess: false,
		},
		{
			name:      "role requires MFA and MFA verified",
			roles:     []*types.RoleV4{matchingLabelsRole, matchingLabelsRoleWithMFA},
			cluster:   clusterWithLabels,
			mfaParams: AccessMFAParams{Verified: true},
			hasAccess: true,
		},
		{
			name:      "cluster requires MFA but MFA not verified",
			roles:     []*types.RoleV4{matchingLabelsRole},
			cluster:   clusterWithLabels,
			mfaParams: AccessMFAParams{Verified: false, AlwaysRequired: true},
			hasAccess: false,
		},
		{
			name:      "role requires MFA and MFA verified",
			roles:     []*types.RoleV4{matchingLabelsRole},
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
			k8sV3, err := types.NewKubernetesClusterV3FromLegacyCluster(apidefaults.Namespace, tc.cluster)
			require.NoError(t, err)

			err = set.CheckAccess(k8sV3, tc.mfaParams)
			if tc.hasAccess {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				require.True(t, trace.IsAccessDenied(err))
			}
		})
	}
}

func TestCheckAccessToWindowsDesktop(t *testing.T) {
	desktopNoLabels := &types.WindowsDesktopV3{
		ResourceHeader: types.ResourceHeader{
			Kind:     types.KindWindowsDesktop,
			Metadata: types.Metadata{Name: "no-labels"},
		},
	}
	desktop2012 := &types.WindowsDesktopV3{
		ResourceHeader: types.ResourceHeader{
			Kind: types.KindWindowsDesktop,
			Metadata: types.Metadata{
				Name:   "win2012",
				Labels: map[string]string{"win_version": "2012"},
			},
		},
	}

	type check struct {
		desktop   *types.WindowsDesktopV3
		login     string
		hasAccess bool
	}

	for _, test := range []struct {
		name      string
		roles     []types.RoleV4
		mfaParams AccessMFAParams
		checks    []check
	}{
		{
			name:  "no roles, no access",
			roles: []types.RoleV4{},
			checks: []check{
				{desktop: desktopNoLabels, login: "admin", hasAccess: false},
				{desktop: desktop2012, login: "admin", hasAccess: false},
			},
		},
		{
			name: "empty label, no access",
			roles: []types.RoleV4{
				newRole(func(r *types.RoleV4) {
					r.Spec.Allow.WindowsDesktopLogins = []string{"admin"}
					r.Spec.Allow.WindowsDesktopLabels = types.Labels{"role": []string{}}
				}),
			},
			checks: []check{
				{desktop: desktopNoLabels, login: "admin", hasAccess: false},
				{desktop: desktop2012, login: "admin", hasAccess: false},
			},
		},
		{
			name: "single role allows a single login",
			roles: []types.RoleV4{
				newRole(func(r *types.RoleV4) {
					r.Spec.Allow.WindowsDesktopLogins = []string{"admin"}
				}),
			},
			checks: []check{
				{desktop: desktopNoLabels, login: "admin", hasAccess: true},
				{desktop: desktop2012, login: "admin", hasAccess: true},
				{desktop: desktopNoLabels, login: "foo", hasAccess: false},
				{desktop: desktop2012, login: "foo", hasAccess: false},
			},
		},
		{
			name: "single role with allowed labels",
			roles: []types.RoleV4{
				newRole(func(r *types.RoleV4) {
					r.Spec.Allow.WindowsDesktopLogins = []string{"admin"}
					r.Spec.Allow.WindowsDesktopLabels = types.Labels{"win_version": []string{"2012"}}
				}),
			},
			checks: []check{
				{desktop: desktopNoLabels, login: "admin", hasAccess: false},
				{desktop: desktop2012, login: "admin", hasAccess: true},
			},
		},
		{
			name: "single role with deny labels",
			roles: []types.RoleV4{
				newRole(func(r *types.RoleV4) {
					r.Spec.Allow.WindowsDesktopLogins = []string{"admin"}
					r.Spec.Deny.Namespaces = []string{apidefaults.Namespace}
					r.Spec.Deny.WindowsDesktopLabels = types.Labels{"win_version": []string{"2012"}}
				}),
			},
			checks: []check{
				{desktop: desktopNoLabels, login: "admin", hasAccess: true},
				{desktop: desktop2012, login: "admin", hasAccess: false},
			},
		},
		{
			name: "one role more permissive than another",
			roles: []types.RoleV4{
				newRole(func(r *types.RoleV4) {
					r.Spec.Allow.WindowsDesktopLogins = []string{"admin"}
					r.Spec.Allow.NodeLabels = types.Labels{"win_version": []string{"2012"}}
				}),
				newRole(func(r *types.RoleV4) {
					r.Spec.Allow.WindowsDesktopLogins = []string{"root", "admin"}
				}),
			},
			checks: []check{
				{desktop: desktopNoLabels, login: "root", hasAccess: true},
				{desktop: desktopNoLabels, login: "admin", hasAccess: true},
				{desktop: desktop2012, login: "root", hasAccess: true},
				{desktop: desktop2012, login: "admin", hasAccess: true},
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			var set RoleSet
			for _, r := range test.roles {
				set = append(set, &r)
			}

			for i, check := range test.checks {
				msg := fmt.Sprintf("check=%d, user=%v, server=%v, should_have_access=%v",
					i, check.login, check.desktop.GetName(), check.hasAccess)
				err := set.CheckAccess(check.desktop, test.mfaParams, NewWindowsLoginMatcher(check.login))
				if check.hasAccess {
					require.NoError(t, err, msg)
				} else {
					require.Error(t, err, msg)
					require.True(t, trace.IsAccessDenied(err), "expected access denied error, got %v", err)
				}
			}
		})
	}
}

// BenchmarkCheckAccessToServer tests how long it takes to run
// CheckAccess for servers across 4,000 nodes for 5 roles each with 5 logins each.
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
	servers := make([]*types.ServerV2, 0, 4000)

	// Create 4,000 servers with random IDs.
	for i := 0; i < 4000; i++ {
		hostname := uuid.New().String()
		servers = append(servers, &types.ServerV2{
			Kind:    types.KindNode,
			Version: types.V2,
			Metadata: types.Metadata{
				Name:      hostname,
				Namespace: apidefaults.Namespace,
			},
			Spec: types.ServerSpecV2{
				Addr:     "127.0.0.1:3022",
				Hostname: hostname,
			},
		})
	}

	// Create RoleSet with four generic roles that have five logins
	// each and only have access to the a:b label.
	var set RoleSet
	for i := 0; i < 4; i++ {
		set = append(set, &types.RoleV4{
			Kind:    types.KindRole,
			Version: types.V3,
			Metadata: types.Metadata{
				Name:      strconv.Itoa(i),
				Namespace: apidefaults.Namespace,
			},
			Spec: types.RoleSpecV4{
				Allow: types.RoleConditions{
					Logins:     []string{"admin", "one", "two", "three", "four"},
					NodeLabels: types.Labels{"a": []string{"b"}},
				},
			},
		})
	}

	// Initialization is complete, start the benchmark timer.
	b.ResetTimer()

	// Build a map of all allowed logins.
	allowLogins := map[string]bool{}
	for _, role := range set {
		for _, login := range role.GetLogins(types.Allow) {
			allowLogins[login] = true
		}
	}

	// Check access to all 4,000 nodes.
	for n := 0; n < b.N; n++ {
		for i := 0; i < 4000; i++ {
			for login := range allowLogins {
				// note: we don't check the error here because this benchmark
				// is testing the performance of failed RBAC checks
				_ = set.CheckAccess(
					servers[i],
					AccessMFAParams{},
					NewLoginMatcher(login),
				)
			}
		}
	}
}

// userGetter is used in tests to return a user with the specified roles and
// traits.
type userGetter struct {
	roles  []string
	traits map[string][]string
}

func (f *userGetter) GetUser(name string, _ bool) (types.User, error) {
	user, err := types.NewUser(name)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	user.SetRoles(f.roles)
	user.SetTraits(f.traits)
	return user, nil
}

func TestRoleSetLockingMode(t *testing.T) {
	t.Parallel()
	t.Run("empty RoleSet gives default LockingMode", func(t *testing.T) {
		t.Parallel()
		set := RoleSet{}
		for _, defaultMode := range []constants.LockingMode{constants.LockingModeBestEffort, constants.LockingModeStrict} {
			require.Equal(t, defaultMode, set.LockingMode(defaultMode))
		}
	})

	missingMode := constants.LockingMode("")
	newRoleWithLockingMode := func(t *testing.T, mode constants.LockingMode) types.Role {
		role, err := types.NewRole(uuid.New().String(), types.RoleSpecV4{Options: types.RoleOptions{Lock: mode}})
		require.NoError(t, err)
		return role
	}

	t.Run("RoleSet with missing LockingMode gives default LockingMode", func(t *testing.T) {
		t.Parallel()
		set := RoleSet{newRoleWithLockingMode(t, missingMode), newRoleWithLockingMode(t, missingMode)}
		for _, defaultMode := range []constants.LockingMode{constants.LockingModeBestEffort, constants.LockingModeStrict} {
			require.Equal(t, defaultMode, set.LockingMode(defaultMode))
		}
	})
	t.Run("RoleSet with a set LockingMode gives the set LockingMode", func(t *testing.T) {
		t.Parallel()
		role1 := newRoleWithLockingMode(t, missingMode)
		for _, mode := range []constants.LockingMode{constants.LockingModeBestEffort, constants.LockingModeStrict} {
			role2 := newRoleWithLockingMode(t, mode)
			set := RoleSet{role1, role2}
			require.Equal(t, mode, set.LockingMode(mode))
		}
	})
	t.Run("RoleSet featuring LockingModeStrict gives LockingModeStrict", func(t *testing.T) {
		t.Parallel()
		role1 := newRoleWithLockingMode(t, constants.LockingModeBestEffort)
		for _, mode := range []constants.LockingMode{constants.LockingModeBestEffort, constants.LockingModeStrict} {
			role2 := newRoleWithLockingMode(t, mode)
			set := RoleSet{role1, role2}
			require.Equal(t, mode, set.LockingMode(mode))
		}
	})
}

func TestExtractConditionForIdentifier(t *testing.T) {
	t.Parallel()
	set := RoleSet{}
	_, err := set.ExtractConditionForIdentifier(&Context{}, apidefaults.Namespace, types.KindSession, types.VerbList, SessionIdentifier)
	require.True(t, trace.IsAccessDenied(err))

	allowWhere := func(where string) types.Role {
		role, err := types.NewRole(uuid.New().String(), types.RoleSpecV4{Allow: types.RoleConditions{
			Rules: []types.Rule{{Resources: []string{types.KindSession}, Verbs: []string{types.VerbList}, Where: where}},
		}})
		require.NoError(t, err)
		return role
	}
	denyWhere := func(where string) types.Role {
		role, err := types.NewRole(uuid.New().String(), types.RoleSpecV4{Deny: types.RoleConditions{
			Rules: []types.Rule{{Resources: []string{types.KindSession}, Verbs: []string{types.VerbList}, Where: where}},
		}})
		require.NoError(t, err)
		return role
	}

	user, err := types.NewUser("test-user")
	require.NoError(t, err)
	user2, err := types.NewUser("test-user2")
	require.NoError(t, err)
	user2Meta := user2.GetMetadata()
	user2Meta.Labels = map[string]string{"can-audit-guest": "yes"}
	user2.SetMetadata(user2Meta)

	// Add a role allowing access to guest session recordings if the user has a set label.
	role := allowWhere(`contains(session.participants, "guest") && equals(user.metadata.labels["can-audit-guest"], "yes")`)
	guestParticipantCond := &types.WhereExpr{Contains: types.WhereExpr2{L: &types.WhereExpr{Field: "participants"}, R: &types.WhereExpr{Literal: "guest"}}}
	set = append(set, role)

	_, err = set.ExtractConditionForIdentifier(&Context{}, apidefaults.Namespace, types.KindSession, types.VerbList, SessionIdentifier)
	require.True(t, trace.IsAccessDenied(err))
	_, err = set.ExtractConditionForIdentifier(&Context{User: user}, apidefaults.Namespace, types.KindSession, types.VerbList, SessionIdentifier)
	require.True(t, trace.IsAccessDenied(err))
	cond, err := set.ExtractConditionForIdentifier(&Context{User: user2}, apidefaults.Namespace, types.KindSession, types.VerbList, SessionIdentifier)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(cond, guestParticipantCond))

	// Add a role allowing access to the user's own session recordings.
	role = allowWhere(`contains(session.participants, user.metadata.name)`)
	userParticipantCond := func(user types.User) *types.WhereExpr {
		return &types.WhereExpr{Contains: types.WhereExpr2{L: &types.WhereExpr{Field: "participants"}, R: &types.WhereExpr{Literal: user.GetName()}}}
	}
	set = append(set, role)

	cond, err = set.ExtractConditionForIdentifier(&Context{}, apidefaults.Namespace, types.KindSession, types.VerbList, SessionIdentifier)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(cond, userParticipantCond(emptyUser)))
	cond, err = set.ExtractConditionForIdentifier(&Context{User: user}, apidefaults.Namespace, types.KindSession, types.VerbList, SessionIdentifier)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(cond, userParticipantCond(user)))
	cond, err = set.ExtractConditionForIdentifier(&Context{User: user2}, apidefaults.Namespace, types.KindSession, types.VerbList, SessionIdentifier)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(cond, &types.WhereExpr{Or: types.WhereExpr2{L: guestParticipantCond, R: userParticipantCond(user2)}}))

	// Add a role denying access to sessions with root login.
	role = denyWhere(`equals(session.login, "root")`)
	noRootLoginCond := &types.WhereExpr{Not: &types.WhereExpr{Equals: types.WhereExpr2{L: &types.WhereExpr{Field: "login"}, R: &types.WhereExpr{Literal: "root"}}}}
	set = append(set, role)

	cond, err = set.ExtractConditionForIdentifier(&Context{}, apidefaults.Namespace, types.KindSession, types.VerbList, SessionIdentifier)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(cond, &types.WhereExpr{And: types.WhereExpr2{L: noRootLoginCond, R: userParticipantCond(emptyUser)}}))
	cond, err = set.ExtractConditionForIdentifier(&Context{User: user}, apidefaults.Namespace, types.KindSession, types.VerbList, SessionIdentifier)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(cond, &types.WhereExpr{And: types.WhereExpr2{L: noRootLoginCond, R: userParticipantCond(user)}}))
	cond, err = set.ExtractConditionForIdentifier(&Context{User: user2}, apidefaults.Namespace, types.KindSession, types.VerbList, SessionIdentifier)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(cond, &types.WhereExpr{And: types.WhereExpr2{L: noRootLoginCond, R: &types.WhereExpr{Or: types.WhereExpr2{L: guestParticipantCond, R: userParticipantCond(user2)}}}}))

	// Add a role denying access for user2.
	role = denyWhere(fmt.Sprintf(`equals(user.metadata.name, "%s")`, user2.GetName()))
	set = append(set, role)

	cond, err = set.ExtractConditionForIdentifier(&Context{}, apidefaults.Namespace, types.KindSession, types.VerbList, SessionIdentifier)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(cond, &types.WhereExpr{And: types.WhereExpr2{L: noRootLoginCond, R: userParticipantCond(emptyUser)}}))
	cond, err = set.ExtractConditionForIdentifier(&Context{User: user}, apidefaults.Namespace, types.KindSession, types.VerbList, SessionIdentifier)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(cond, &types.WhereExpr{And: types.WhereExpr2{L: noRootLoginCond, R: userParticipantCond(user)}}))
	_, err = set.ExtractConditionForIdentifier(&Context{User: user2}, apidefaults.Namespace, types.KindSession, types.VerbList, SessionIdentifier)
	require.True(t, trace.IsAccessDenied(err))

	// Add a role allowing access to all sessions.
	// This should cause all the other allow rules' conditions to be dropped.
	role = allowWhere(``)
	set = append(set, role)

	cond, err = set.ExtractConditionForIdentifier(&Context{}, apidefaults.Namespace, types.KindSession, types.VerbList, SessionIdentifier)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(cond, noRootLoginCond))
	cond, err = set.ExtractConditionForIdentifier(&Context{User: user}, apidefaults.Namespace, types.KindSession, types.VerbList, SessionIdentifier)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(cond, noRootLoginCond))
	_, err = set.ExtractConditionForIdentifier(&Context{User: user2}, apidefaults.Namespace, types.KindSession, types.VerbList, SessionIdentifier)
	require.True(t, trace.IsAccessDenied(err))

	// Add a role denying access to all sessions.
	// This should make all calls return an AccessDenied.
	role = denyWhere(``)
	set = append(set, role)

	_, err = set.ExtractConditionForIdentifier(&Context{}, apidefaults.Namespace, types.KindSession, types.VerbList, SessionIdentifier)
	require.True(t, trace.IsAccessDenied(err))
	_, err = set.ExtractConditionForIdentifier(&Context{User: user}, apidefaults.Namespace, types.KindSession, types.VerbList, SessionIdentifier)
	require.True(t, trace.IsAccessDenied(err))
	_, err = set.ExtractConditionForIdentifier(&Context{User: user2}, apidefaults.Namespace, types.KindSession, types.VerbList, SessionIdentifier)
	require.True(t, trace.IsAccessDenied(err))
}

func TestCheckKubeGroupsAndUsers(t *testing.T) {
	roleA := &types.RoleV4{
		Metadata: types.Metadata{Name: "roleA", Namespace: apidefaults.Namespace},
		Spec: types.RoleSpecV4{
			Allow: types.RoleConditions{
				KubeGroups: []string{"system:masters"},
				KubeUsers:  []string{"dev-user"},
				KubernetesLabels: map[string]apiutils.Strings{
					"env": []string{"dev"},
				},
			},
		},
	}
	roleB := &types.RoleV4{
		Metadata: types.Metadata{Name: "roleB", Namespace: apidefaults.Namespace},
		Spec: types.RoleSpecV4{
			Allow: types.RoleConditions{
				KubeGroups: []string{"limited"},
				KubeUsers:  []string{"limited-user"},
				KubernetesLabels: map[string]apiutils.Strings{
					"env": []string{"prod"},
				},
			},
		},
	}
	roleC := &types.RoleV4{
		Metadata: types.Metadata{Name: "roleC", Namespace: apidefaults.Namespace},
		Spec: types.RoleSpecV4{
			Allow: types.RoleConditions{
				KubeGroups: []string{"system:masters", "groupB"},
				KubeUsers:  []string{"user"},
				KubernetesLabels: map[string]apiutils.Strings{
					"env": []string{"dev", "prod"},
				},
			},
		},
	}

	testCases := []struct {
		name          string
		kubeResLabels map[string]string
		roles         RoleSet
		wantUsers     []string
		wantGroups    []string
	}{
		{
			name: "empty kube labels should returns all kube users/groups",
			roles: RoleSet{
				&types.RoleV4{
					Metadata: types.Metadata{Name: "role-dev", Namespace: apidefaults.Namespace},
					Spec: types.RoleSpecV4{
						Allow: types.RoleConditions{
							KubeUsers:  []string{"dev-user"},
							KubeGroups: []string{"system:masters"},
							KubernetesLabels: map[string]apiutils.Strings{
								"*": []string{"*"},
							},
						},
					},
				},
				&types.RoleV4{
					Metadata: types.Metadata{Name: "role-prod", Namespace: apidefaults.Namespace},
					Spec: types.RoleSpecV4{
						Allow: types.RoleConditions{
							KubeUsers:  []string{"limited-user"},
							KubeGroups: []string{"limited"},
							KubernetesLabels: map[string]apiutils.Strings{
								"*": []string{"*"},
							},
						},
					},
				},
			},
			wantUsers:  []string{"dev-user", "limited-user"},
			wantGroups: []string{"limited", "system:masters"},
		},
		{
			name:  "dev accesses should allow system:masters roles",
			roles: RoleSet{roleA, roleB},
			kubeResLabels: map[string]string{
				"env": "dev",
			},
			wantUsers:  []string{"dev-user"},
			wantGroups: []string{"system:masters"},
		},
		{
			name:  "prod limited access",
			roles: RoleSet{roleA, roleB},
			kubeResLabels: map[string]string{
				"env": "prod",
			},
			wantUsers:  []string{"limited-user"},
			wantGroups: []string{"limited"},
		},
		{
			name:  "combine kube users/groups for different roles",
			roles: RoleSet{roleA, roleB, roleC},
			kubeResLabels: map[string]string{
				"env": "prod",
			},
			wantUsers:  []string{"limited-user", "user"},
			wantGroups: []string{"groupB", "limited", "system:masters"},
		},
		{
			name:  "all prod group",
			roles: RoleSet{roleC},
			kubeResLabels: map[string]string{
				"env": "prod",
			},
			wantUsers:  []string{"user"},
			wantGroups: []string{"system:masters", "groupB"},
		},
		{
			name: "deny system:masters prod kube group",
			roles: RoleSet{
				roleC,
				&types.RoleV4{
					Metadata: types.Metadata{Name: "roleD", Namespace: apidefaults.Namespace},
					Spec: types.RoleSpecV4{
						Deny: types.RoleConditions{
							KubeGroups: []string{"system:masters"},
							KubernetesLabels: map[string]apiutils.Strings{
								"env": []string{"prod", "dev"},
							},
						},
					},
				},
			},
			kubeResLabels: map[string]string{
				"env": "prod",
			},
			wantUsers:  []string{"user"},
			wantGroups: []string{"groupB"},
		},
		{
			name: "deny access with system:masters kube group",
			kubeResLabels: map[string]string{
				"env":     "prod",
				"release": "test",
			},
			roles: RoleSet{
				&types.RoleV4{
					Metadata: types.Metadata{Name: "roleA", Namespace: apidefaults.Namespace},
					Spec: types.RoleSpecV4{
						Allow: types.RoleConditions{
							KubeGroups: []string{"system:masters", "groupA"},
							KubeUsers:  []string{"dev-user"},
							KubernetesLabels: map[string]apiutils.Strings{
								"env":     []string{"prod"},
								"release": []string{"test"},
							},
						},
					},
				},
				&types.RoleV4{
					Metadata: types.Metadata{Name: "deny", Namespace: apidefaults.Namespace},
					Spec: types.RoleSpecV4{
						Deny: types.RoleConditions{
							KubeGroups: []string{"system:masters"},
							KubernetesLabels: map[string]apiutils.Strings{
								"env": []string{"prod"},
							},
						},
					},
				},
			},
			wantUsers:  []string{"dev-user"},
			wantGroups: []string{"groupA"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			matcher := NewKubernetesClusterLabelMatcher(tc.kubeResLabels)
			gotGroups, gotUsers, err := tc.roles.CheckKubeGroupsAndUsers(time.Hour, true, matcher)
			require.NoError(t, err)

			require.ElementsMatch(t, tc.wantUsers, gotUsers)
			require.ElementsMatch(t, tc.wantGroups, gotGroups)
		})
	}
}
