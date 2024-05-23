/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package services

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	gocmp "github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/constants"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/wrappers"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/api/utils/sshutils"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/tlsca"
)

const matchAllExpression = `"" == ""`

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
			role := &types.RoleV6{
				Kind:    types.KindRole,
				Version: types.V3,
				Metadata: types.Metadata{
					Name:      fmt.Sprintf("role-%d", i),
					Namespace: apidefaults.Namespace,
				},
				Spec: types.RoleSpecV6{
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
		role         types.RoleV6
		error        error
		matchMessage string
	}{
		{
			name:  "no input, should not parse",
			in:    ``,
			role:  types.RoleV6{},
			error: trace.BadParameter("empty input"),
		},
		{
			name:  "validation error, no name",
			in:    `{}`,
			role:  types.RoleV6{},
			error: trace.BadParameter("failed to validate: name: name is required"),
		},
		{
			name:  "validation error, no name",
			in:    `{"kind": "role"}`,
			role:  types.RoleV6{},
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
			name: "validation error, missing namespace in pod names",
			in: `{
					"kind": "role",
					"version": "v6",
					"metadata": {"name": "name1"},
					"spec": {
						"allow": {
							"kubernetes_labels": {"a": "b"},
							"kubernetes_resources": [{"kind":"pod","name": "*","namespace": ""}]
						}
					}
				}`,
			error:        trace.BadParameter(""),
			matchMessage: "KubernetesResource must include Namespace",
		},
		{
			name: "validation error, missing podname in pod names",
			in: `{
					"kind": "role",
					"version": "v6",
					"metadata": {"name": "name1"},
					"spec": {
						"allow": {
							"kubernetes_labels": {"a": "b"},
							"kubernetes_resources": [{"kind":"pod","name": "","namespace": "*"}]
						}
					}
				}`,
			error:        trace.BadParameter(""),
			matchMessage: "KubernetesResource must include Name",
		},
		{
			name:         "validation error, no version",
			in:           `{"kind": "role", "metadata": {"name": "defrole"}, "spec": {}}`,
			role:         types.RoleV6{},
			error:        trace.BadParameter(""),
			matchMessage: `role version "" is not supported`,
		},
		{
			name:         "validation error, bad version",
			in:           `{"kind": "role", "version": "v2", "metadata": {"name": "defrole"}, "spec": {}}`,
			role:         types.RoleV6{},
			error:        trace.BadParameter(""),
			matchMessage: `role version "v2" is not supported`,
		},
		{
			name: "v3 role with no spec gets v3 defaults",
			in:   `{"kind": "role", "version": "v3", "metadata": {"name": "defrole"}, "spec": {}}`,
			role: types.RoleV6{
				Kind:    types.KindRole,
				Version: types.V3,
				Metadata: types.Metadata{
					Name:      "defrole",
					Namespace: apidefaults.Namespace,
				},
				Spec: types.RoleSpecV6{
					Options: types.RoleOptions{
						CertificateFormat: constants.CertificateFormatStandard,
						MaxSessionTTL:     types.NewDuration(apidefaults.MaxCertDuration),
						PortForwarding:    types.NewBoolOption(true),
						RecordSession: &types.RecordSession{
							Default: constants.SessionRecordingModeBestEffort,
							Desktop: types.NewBoolOption(true),
						},
						BPF:                     apidefaults.EnhancedEvents(),
						DesktopClipboard:        types.NewBoolOption(true),
						DesktopDirectorySharing: types.NewBoolOption(true),
						CreateDesktopUser:       types.NewBoolOption(false),
						CreateHostUser:          nil,
						CreateDatabaseUser:      types.NewBoolOption(false),
						SSHFileCopy:             types.NewBoolOption(true),
						IDP: &types.IdPOptions{
							SAML: &types.IdPSAMLOptions{
								Enabled: types.NewBoolOption(true),
							},
						},
					},
					Allow: types.RoleConditions{
						NodeLabels:       types.Labels{},
						AppLabels:        types.Labels{types.Wildcard: []string{types.Wildcard}},
						KubernetesLabels: types.Labels{types.Wildcard: []string{types.Wildcard}},
						DatabaseLabels:   types.Labels{types.Wildcard: []string{types.Wildcard}},
						Namespaces:       []string{apidefaults.Namespace},
						KubernetesResources: []types.KubernetesResource{
							{
								Kind:      types.KindKubePod,
								Namespace: types.Wildcard,
								Name:      types.Wildcard,
								Verbs:     []string{types.Wildcard},
							},
						},
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
			role: types.RoleV6{
				Kind:    types.KindRole,
				Version: types.V4,
				Metadata: types.Metadata{
					Name:      "defrole",
					Namespace: apidefaults.Namespace,
				},
				Spec: types.RoleSpecV6{
					Options: types.RoleOptions{
						CertificateFormat: constants.CertificateFormatStandard,
						MaxSessionTTL:     types.NewDuration(apidefaults.MaxCertDuration),
						PortForwarding:    types.NewBoolOption(true),
						RecordSession: &types.RecordSession{
							Default: constants.SessionRecordingModeBestEffort,
							Desktop: types.NewBoolOption(true),
						},
						BPF:                     apidefaults.EnhancedEvents(),
						DesktopClipboard:        types.NewBoolOption(true),
						DesktopDirectorySharing: types.NewBoolOption(true),
						CreateDesktopUser:       types.NewBoolOption(false),
						CreateHostUser:          nil,
						CreateDatabaseUser:      types.NewBoolOption(false),
						SSHFileCopy:             types.NewBoolOption(true),
						IDP: &types.IdPOptions{
							SAML: &types.IdPSAMLOptions{
								Enabled: types.NewBoolOption(true),
							},
						},
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
			name: "full valid role v6",
			in: `{
					"kind": "role",
					"version": "v6",
					"metadata": {"name": "name1", "labels": {"a-b": "c"}},
					"spec": {
						"options": {
							"cert_format": "standard",
							"max_session_ttl": "20h",
							"port_forwarding": true,
							"client_idle_timeout": "17m",
							"disconnect_expired_cert": "yes",
							"enhanced_recording": ["command", "network"],
							"desktop_clipboard": true,
							"desktop_directory_sharing": true,
							"ssh_file_copy" : false
						},
						"allow": {
							"node_labels": {"a": "b", "c-d": "e"},
							"app_labels": {"a": "b", "c-d": "e"},
							"group_labels": {"a": "b", "c-d": "e"},
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
			role: types.RoleV6{
				Kind:    types.KindRole,
				Version: types.V6,
				Metadata: types.Metadata{
					Name:      "name1",
					Namespace: apidefaults.Namespace,
					Labels:    map[string]string{"a-b": "c"},
				},
				Spec: types.RoleSpecV6{
					Options: types.RoleOptions{
						CertificateFormat: constants.CertificateFormatStandard,
						MaxSessionTTL:     types.NewDuration(20 * time.Hour),
						PortForwarding:    types.NewBoolOption(true),
						RecordSession: &types.RecordSession{
							Default: constants.SessionRecordingModeBestEffort,
							Desktop: types.NewBoolOption(true),
						},
						ClientIdleTimeout:       types.NewDuration(17 * time.Minute),
						DisconnectExpiredCert:   types.NewBool(true),
						BPF:                     apidefaults.EnhancedEvents(),
						DesktopClipboard:        types.NewBoolOption(true),
						DesktopDirectorySharing: types.NewBoolOption(true),
						CreateDesktopUser:       types.NewBoolOption(false),
						CreateDatabaseUser:      types.NewBoolOption(false),
						CreateHostUser:          nil,
						SSHFileCopy:             types.NewBoolOption(false),
						IDP: &types.IdPOptions{
							SAML: &types.IdPSAMLOptions{
								Enabled: types.NewBoolOption(true),
							},
						},
					},
					Allow: types.RoleConditions{
						NodeLabels:       types.Labels{"a": []string{"b"}, "c-d": []string{"e"}},
						AppLabels:        types.Labels{"a": []string{"b"}, "c-d": []string{"e"}},
						GroupLabels:      types.Labels{"a": []string{"b"}, "c-d": []string{"e"}},
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
							"enhanced_recording": ["command", "network"],
							"desktop_clipboard": true,
							"desktop_directory_sharing": true,
							"ssh_file_copy" : false
						},
						"allow": {
							"node_labels": {"a": "b", "c-d": "e"},
							"app_labels": {"a": "b", "c-d": "e"},
							"group_labels": {"a": "b", "c-d": "e"},
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
			role: types.RoleV6{
				Kind:    types.KindRole,
				Version: types.V3,
				Metadata: types.Metadata{
					Name:      "name1",
					Namespace: apidefaults.Namespace,
					Labels:    map[string]string{"a-b": "c"},
				},
				Spec: types.RoleSpecV6{
					Options: types.RoleOptions{
						CertificateFormat: constants.CertificateFormatStandard,
						MaxSessionTTL:     types.NewDuration(20 * time.Hour),
						PortForwarding:    types.NewBoolOption(true),
						RecordSession: &types.RecordSession{
							Default: constants.SessionRecordingModeBestEffort,
							Desktop: types.NewBoolOption(true),
						},
						ClientIdleTimeout:       types.NewDuration(17 * time.Minute),
						DisconnectExpiredCert:   types.NewBool(true),
						BPF:                     apidefaults.EnhancedEvents(),
						DesktopClipboard:        types.NewBoolOption(true),
						DesktopDirectorySharing: types.NewBoolOption(true),
						CreateDesktopUser:       types.NewBoolOption(false),
						CreateHostUser:          nil,
						CreateDatabaseUser:      types.NewBoolOption(false),
						SSHFileCopy:             types.NewBoolOption(false),
						IDP: &types.IdPOptions{
							SAML: &types.IdPSAMLOptions{
								Enabled: types.NewBoolOption(true),
							},
						},
					},
					Allow: types.RoleConditions{
						NodeLabels:       types.Labels{"a": []string{"b"}, "c-d": []string{"e"}},
						AppLabels:        types.Labels{"a": []string{"b"}, "c-d": []string{"e"}},
						GroupLabels:      types.Labels{"a": []string{"b"}, "c-d": []string{"e"}},
						KubernetesLabels: types.Labels{"a": []string{"b"}, "c-d": []string{"e"}},
						DatabaseLabels:   types.Labels{"a": []string{"b"}, "c-d": []string{"e"}},
						DatabaseNames:    []string{"postgres"},
						DatabaseUsers:    []string{"postgres"},
						KubernetesResources: []types.KubernetesResource{
							{
								Kind:      types.KindKubePod,
								Namespace: types.Wildcard,
								Name:      types.Wildcard,
								Verbs:     []string{types.Wildcard},
							},
						},
						Namespaces: []string{"default"},
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
							  "enhanced_recording": ["command", "network"],
							  "desktop_clipboard": true,
							  "desktop_directory_sharing": true,
							  "ssh_file_copy" : true
							},
							"allow": {
							  "node_labels": {"a": "b"},
							  "app_labels": {"a": "b"},
							  "group_labels": {"a": "b"},
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
			role: types.RoleV6{
				Kind:    types.KindRole,
				Version: types.V3,
				Metadata: types.Metadata{
					Name:      "name1",
					Namespace: apidefaults.Namespace,
				},
				Spec: types.RoleSpecV6{
					Options: types.RoleOptions{
						CertificateFormat: constants.CertificateFormatStandard,
						ForwardAgent:      types.NewBool(true),
						MaxSessionTTL:     types.NewDuration(20 * time.Hour),
						PortForwarding:    types.NewBoolOption(true),
						RecordSession: &types.RecordSession{
							Default: constants.SessionRecordingModeBestEffort,
							Desktop: types.NewBoolOption(true),
						},
						ClientIdleTimeout:       types.NewDuration(0),
						DisconnectExpiredCert:   types.NewBool(false),
						BPF:                     apidefaults.EnhancedEvents(),
						DesktopClipboard:        types.NewBoolOption(true),
						DesktopDirectorySharing: types.NewBoolOption(true),
						CreateDesktopUser:       types.NewBoolOption(false),
						CreateHostUser:          nil,
						CreateDatabaseUser:      types.NewBoolOption(false),
						SSHFileCopy:             types.NewBoolOption(true),
						IDP: &types.IdPOptions{
							SAML: &types.IdPSAMLOptions{
								Enabled: types.NewBoolOption(true),
							},
						},
					},
					Allow: types.RoleConditions{
						NodeLabels:       types.Labels{"a": []string{"b"}},
						AppLabels:        types.Labels{"a": []string{"b"}},
						GroupLabels:      types.Labels{"a": []string{"b"}},
						KubernetesLabels: types.Labels{"c": []string{"d"}},
						DatabaseLabels:   types.Labels{"e": []string{"f"}},
						Namespaces:       []string{"default"},
						KubernetesResources: []types.KubernetesResource{
							{
								Kind:      types.KindKubePod,
								Namespace: types.Wildcard,
								Name:      types.Wildcard,
								Verbs:     []string{types.Wildcard},
							},
						},
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
							  "enhanced_recording": ["command", "network"],
							  "desktop_clipboard": true,
							  "desktop_directory_sharing": true,
							  "ssh_file_copy" : true
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
			role: types.RoleV6{
				Kind:    types.KindRole,
				Version: types.V3,
				Metadata: types.Metadata{
					Name:      "name1",
					Namespace: apidefaults.Namespace,
				},
				Spec: types.RoleSpecV6{
					Options: types.RoleOptions{
						CertificateFormat: constants.CertificateFormatStandard,
						ForwardAgent:      types.NewBool(true),
						MaxSessionTTL:     types.NewDuration(20 * time.Hour),
						PortForwarding:    types.NewBoolOption(true),
						RecordSession: &types.RecordSession{
							Default: constants.SessionRecordingModeBestEffort,
							Desktop: types.NewBoolOption(true),
						},
						ClientIdleTimeout:       types.NewDuration(0),
						DisconnectExpiredCert:   types.NewBool(false),
						BPF:                     apidefaults.EnhancedEvents(),
						DesktopClipboard:        types.NewBoolOption(true),
						DesktopDirectorySharing: types.NewBoolOption(true),
						CreateDesktopUser:       types.NewBoolOption(false),
						CreateHostUser:          nil,
						CreateDatabaseUser:      types.NewBoolOption(false),
						SSHFileCopy:             types.NewBoolOption(true),
						IDP: &types.IdPOptions{
							SAML: &types.IdPSAMLOptions{
								Enabled: types.NewBoolOption(true),
							},
						},
					},
					Allow: types.RoleConditions{
						KubernetesResources: []types.KubernetesResource{
							{
								Kind:      types.KindKubePod,
								Namespace: types.Wildcard,
								Name:      types.Wildcard,
								Verbs:     []string{types.Wildcard},
							},
						},
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
				require.Equal(t, &tc.role, role2)
			}
		})
	}
}

func TestValidateRole(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		spec           types.RoleSpecV6
		expectError    error
		expectWarnings []string
	}{
		{
			name: "valid syntax",
			spec: types.RoleSpecV6{
				Allow: types.RoleConditions{
					Logins: []string{`{{external["http://schemas.microsoft.com/ws/2008/06/identity/claims/windowsaccountname"]}}`},
				},
			},
		},
		{
			name: "invalid role condition login syntax",
			spec: types.RoleSpecV6{
				Allow: types.RoleConditions{
					Logins: []string{"{{foo"},
				},
			},
			expectWarnings: []string{
				"parsing allow.logins expression",
				`"{{foo" is using template brackets '{{' or '}}', however expression does not parse`,
			},
		},
		{
			name: "unsupported function in actions",
			spec: types.RoleSpecV6{
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
			expectWarnings: []string{
				"parsing allow rule",
				"could not parse 'where' rule",
				"unsupported function: containz",
			},
		},
		{
			name: "unsupported function in where",
			spec: types.RoleSpecV6{
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
			expectWarnings: []string{
				"parsing allow rule",
				"could not parse action",
				"unsupported function: zzz",
			},
		},
		{
			name: "wildcard not allowed in database_roles",
			spec: types.RoleSpecV6{
				Allow: types.RoleConditions{
					DatabaseRoles: []string{types.Wildcard},
				},
			},
			expectError: trace.BadParameter("wildcard is not allowed in allow.database_roles"),
		},
		{
			name: "unsupported function in labels",
			spec: types.RoleSpecV6{
				Allow: types.RoleConditions{
					Logins: []string{"test"},
					NodeLabels: types.Labels{
						"owner": {"{{email.localz(external.email)}}"},
					},
					AppLabels: types.Labels{
						"owner": {"{{email.localz(external.email)}}"},
					},
					KubernetesLabels: types.Labels{
						"owner": {"{{email.localz(external.email)}}"},
					},
					DatabaseLabels: types.Labels{
						"owner": {"{{email.localz(external.email)}}"},
					},
					WindowsDesktopLabels: types.Labels{
						"owner": {"{{email.localz(external.email)}}"},
					},
					ClusterLabels: types.Labels{
						"owner": {"{{email.localz(external.email)}}"},
					},
				},
				Deny: types.RoleConditions{
					Logins: []string{"test"},
					NodeLabels: types.Labels{
						"owner": {"{{email.localz(external.email)}}"},
					},
					AppLabels: types.Labels{
						"owner": {"{{email.localz(external.email)}}"},
					},
					KubernetesLabels: types.Labels{
						"owner": {"{{email.localz(external.email)}}"},
					},
					DatabaseLabels: types.Labels{
						"owner": {"{{email.localz(external.email)}}"},
					},
					WindowsDesktopLabels: types.Labels{
						"owner": {"{{email.localz(external.email)}}"},
					},
					ClusterLabels: types.Labels{
						"owner": {"{{email.localz(external.email)}}"},
					},
				},
			},
			expectWarnings: []string{
				"parsing allow.node_labels template expression",
				"parsing allow.app_labels template expression",
				"parsing allow.kubernetes_labels template expression",
				"parsing allow.db_labels template expression",
				"parsing allow.windows_desktop_labels template expression",
				"parsing allow.cluster_labels template expression",
				"parsing deny.node_labels template expression",
				"parsing deny.app_labels template expression",
				"parsing deny.kubernetes_labels template expression",
				"parsing deny.db_labels template expression",
				"parsing deny.windows_desktop_labels template expression",
				"parsing deny.cluster_labels template expression",
				"unsupported function: email.localz",
			},
		},
		{
			name: "unsupported function in labels expression",
			spec: types.RoleSpecV6{
				Allow: types.RoleConditions{
					ClusterLabelsExpression:         `containz(labels["env"], "staging")`,
					NodeLabelsExpression:            `containz(labels["env"], "staging")`,
					AppLabelsExpression:             `containz(labels["env"], "staging")`,
					KubernetesLabelsExpression:      `containz(labels["env"], "staging")`,
					DatabaseLabelsExpression:        `containz(labels["env"], "staging")`,
					DatabaseServiceLabelsExpression: `containz(labels["env"], "staging")`,
					WindowsDesktopLabelsExpression:  `containz(labels["env"], "staging")`,
					GroupLabelsExpression:           `containz(labels["env"], "staging")`,
				},
				Deny: types.RoleConditions{
					ClusterLabelsExpression:         `containz(labels["env"], "staging")`,
					NodeLabelsExpression:            `containz(labels["env"], "staging")`,
					AppLabelsExpression:             `containz(labels["env"], "staging")`,
					KubernetesLabelsExpression:      `containz(labels["env"], "staging")`,
					DatabaseLabelsExpression:        `containz(labels["env"], "staging")`,
					DatabaseServiceLabelsExpression: `containz(labels["env"], "staging")`,
					WindowsDesktopLabelsExpression:  `containz(labels["env"], "staging")`,
					GroupLabelsExpression:           `containz(labels["env"], "staging")`,
				},
			},
			expectWarnings: []string{
				"parsing allow.node_labels_expression",
				"parsing allow.app_labels_expression",
				"parsing allow.kubernetes_labels_expression",
				"parsing allow.db_labels_expression",
				"parsing allow.windows_desktop_labels_expression",
				"parsing allow.cluster_labels_expression",
				"parsing deny.node_labels_expression",
				"parsing deny.app_labels_expression",
				"parsing deny.kubernetes_labels_expression",
				"parsing deny.db_labels_expression",
				"parsing deny.windows_desktop_labels_expression",
				"parsing deny.cluster_labels_expression",
				"unsupported function: containz",
			},
		},
		{
			name: "unsupported function in slice fields",
			spec: types.RoleSpecV6{
				Allow: types.RoleConditions{
					Logins:               []string{"{{email.localz(external.email)}}"},
					WindowsDesktopLogins: []string{"{{email.localz(external.email)}}"},
					AWSRoleARNs:          []string{"{{email.localz(external.email)}}"},
					AzureIdentities:      []string{"{{email.localz(external.email)}}"},
					GCPServiceAccounts:   []string{"{{email.localz(external.email)}}"},
					KubeGroups:           []string{"{{email.localz(external.email)}}"},
					KubeUsers:            []string{"{{email.localz(external.email)}}"},
					DatabaseNames:        []string{"{{email.localz(external.email)}}"},
					DatabaseUsers:        []string{"{{email.localz(external.email)}}"},
					HostGroups:           []string{"{{email.localz(external.email)}}"},
					HostSudoers:          []string{"{{email.localz(external.email)}}"},
					DesktopGroups:        []string{"{{email.localz(external.email)}}"},
					Impersonate: &types.ImpersonateConditions{
						Users: []string{"{{email.localz(external.email)}}"},
						Roles: []string{"{{email.localz(external.email)}}"},
					},
				},
				Deny: types.RoleConditions{
					Logins:               []string{"{{email.localz(external.email)}}"},
					WindowsDesktopLogins: []string{"{{email.localz(external.email)}}"},
					AWSRoleARNs:          []string{"{{email.localz(external.email)}}"},
					AzureIdentities:      []string{"{{email.localz(external.email)}}"},
					GCPServiceAccounts:   []string{"{{email.localz(external.email)}}"},
					KubeGroups:           []string{"{{email.localz(external.email)}}"},
					KubeUsers:            []string{"{{email.localz(external.email)}}"},
					DatabaseNames:        []string{"{{email.localz(external.email)}}"},
					DatabaseUsers:        []string{"{{email.localz(external.email)}}"},
					HostGroups:           []string{"{{email.localz(external.email)}}"},
					HostSudoers:          []string{"{{email.localz(external.email)}}"},
					DesktopGroups:        []string{"{{email.localz(external.email)}}"},
					Impersonate: &types.ImpersonateConditions{
						Users: []string{"{{email.localz(external.email)}}"},
						Roles: []string{"{{email.localz(external.email)}}"},
					},
				},
			},
			expectWarnings: []string{
				"parsing allow.logins expression",
				"parsing allow.windows_desktop_logins expression",
				"parsing allow.aws_role_arns expression",
				"parsing allow.azure_identities expression",
				"parsing allow.gcp_service_accounts expression",
				"parsing allow.kubernetes_groups expression",
				"parsing allow.kubernetes_users expression",
				"parsing allow.db_names expression",
				"parsing allow.db_users expression",
				"parsing allow.host_groups expression",
				"parsing allow.host_sudeoers expression",
				"parsing allow.desktop_groups expression",
				"parsing allow.impersonate.users expression",
				"parsing allow.impersonate.roles expression",
				"parsing deny.logins expression",
				"parsing deny.windows_desktop_logins expression",
				"parsing deny.aws_role_arns expression",
				"parsing deny.azure_identities expression",
				"parsing deny.gcp_service_accounts expression",
				"parsing deny.kubernetes_groups expression",
				"parsing deny.kubernetes_users expression",
				"parsing deny.db_names expression",
				"parsing deny.db_users expression",
				"parsing deny.host_groups expression",
				"parsing deny.host_sudeoers expression",
				"parsing deny.desktop_groups expression",
				"parsing deny.impersonate.users expression",
				"parsing deny.impersonate.roles expression",
				"unsupported function: email.localz",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var warning error
			err := ValidateRole(&types.RoleV6{
				Metadata: types.Metadata{
					Name:      "name1",
					Namespace: apidefaults.Namespace,
				},
				Version: types.V3,
				Spec:    tc.spec,
			}, withWarningReporter(func(err error) {
				warning = err
			}))
			if tc.expectError != nil {
				require.ErrorIs(t, err, tc.expectError)
				return
			}
			require.NoError(t, err, trace.DebugReport(err))

			if len(tc.expectWarnings) == 0 {
				require.Empty(t, warning)
			}
			for _, msg := range tc.expectWarnings {
				require.ErrorContains(t, warning, msg)
			}
		})
	}
}

// BenchmarkValidateRole benchmarks the performance of ValidateRole.
//
// $ go test ./lib/services -bench BenchmarkValidateRole -v -run xxx
// goos: darwin
// goarch: amd64
// pkg: github.com/gravitational/teleport/lib/services
// cpu: Intel(R) Core(TM) i9-9880H CPU @ 2.30GHz
// BenchmarkValidateRole
// BenchmarkValidateRole-16           14630             80205 ns/op
// PASS
// ok      github.com/gravitational/teleport/lib/services  3.030s
func BenchmarkValidateRole(b *testing.B) {
	role, err := types.NewRole("test", types.RoleSpecV6{
		Allow: types.RoleConditions{
			Logins:               []string{"{{email.local(external.email)}}"},
			WindowsDesktopLogins: []string{"{{email.local(external.email)}}"},
			AWSRoleARNs:          []string{"{{email.local(external.email)}}"},
			AzureIdentities:      []string{"{{email.local(external.email)}}"},
			GCPServiceAccounts:   []string{"{{email.local(external.email)}}"},
			KubeGroups:           []string{"{{email.local(external.email)}}"},
			KubeUsers:            []string{"{{email.local(external.email)}}"},
			DatabaseNames:        []string{"{{email.local(external.email)}}"},
			DatabaseUsers:        []string{"{{email.local(external.email)}}"},
			HostGroups:           []string{"{{email.local(external.email)}}"},
			HostSudoers:          []string{"{{email.local(external.email)}}"},
			DesktopGroups:        []string{"{{email.local(external.email)}}"},
			Impersonate: &types.ImpersonateConditions{
				Users: []string{"{{email.local(external.email)}}"},
				Roles: []string{"{{email.local(external.email)}}"},
			},
			NodeLabels:           types.Labels{"env": {`{{regexp.replace(external["allow-envs"], "^env-(.*)$", "$1")}}`}},
			AppLabels:            types.Labels{"env": {`{{regexp.replace(external["allow-envs"], "^env-(.*)$", "$1")}}`}},
			KubernetesLabels:     types.Labels{"env": {`{{regexp.replace(external["allow-envs"], "^env-(.*)$", "$1")}}`}},
			DatabaseLabels:       types.Labels{"env": {`{{regexp.replace(external["allow-envs"], "^env-(.*)$", "$1")}}`}},
			WindowsDesktopLabels: types.Labels{"env": {`{{regexp.replace(external["allow-envs"], "^env-(.*)$", "$1")}}`}},
			ClusterLabels:        types.Labels{"env": {`{{regexp.replace(external["allow-envs"], "^env-(.*)$", "$1")}}`}},
			Rules: []types.Rule{
				{
					Resources: []string{types.KindRole},
					Verbs:     []string{types.VerbRead, types.VerbList},
					Where:     `contains(user.spec.traits["groups"], "prod")`,
				},
				{
					Resources: []string{types.KindSession},
					Verbs:     []string{types.VerbRead, types.VerbList},
					Where:     "contains(session.participants, user.metadata.name)",
				},
			},
		},
	})
	require.NoError(b, err)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		require.NoError(b, ValidateRole(role))
	}
}

func TestValidateRoleName(t *testing.T) {
	tests := []struct {
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
		err := ValidateRoleName(&types.RoleV6{Metadata: types.Metadata{
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

func newRole(mut func(*types.RoleV6)) *types.RoleV6 {
	r := &types.RoleV6{
		Metadata: types.Metadata{
			Name:      "name",
			Namespace: apidefaults.Namespace,
		},
		Spec: types.RoleSpecV6{
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
	mut(r)
	r.CheckAndSetDefaults()
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
		name                        string
		roles                       []*types.RoleV6
		checks                      []check
		authSpec                    types.AuthPreferenceSpecV2
		enableDeviceVerification    bool
		mfaVerified, deviceVerified bool
	}{
		{
			name:  "empty role set has access to nothing",
			roles: []*types.RoleV6{},
			checks: []check{
				{server: serverNoLabels, login: "root", hasAccess: false},
				{server: serverWorker, login: "root", hasAccess: false},
				{server: serverDB, login: "root", hasAccess: false},
			},
		},
		{
			name: "role is limited to default namespace",
			roles: []*types.RoleV6{
				newRole(func(r *types.RoleV6) {
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
			roles: []*types.RoleV6{
				newRole(func(r *types.RoleV6) {
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
			roles: []*types.RoleV6{
				newRole(func(r *types.RoleV6) {
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
			roles: []*types.RoleV6{
				newRole(func(r *types.RoleV6) {
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
			roles: []*types.RoleV6{
				newRole(func(r *types.RoleV6) {
					r.Spec.Allow.Logins = []string{"admin"}
					r.Spec.Allow.Namespaces = []string{apidefaults.Namespace}
					r.Spec.Allow.NodeLabels = types.Labels{"role": []string{"worker"}}
				}),
				newRole(func(r *types.RoleV6) {
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
			roles: []*types.RoleV6{
				newRole(func(r *types.RoleV6) {
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
			roles: []*types.RoleV6{
				newRole(func(r *types.RoleV6) {
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
		// MFA.
		{
			name: "one role requires MFA but MFA was not verified",
			roles: []*types.RoleV6{
				newRole(func(r *types.RoleV6) {
					r.Spec.Allow.Logins = []string{"root"}
					r.Spec.Allow.NodeLabels = types.Labels{"role": []string{"worker"}}
					r.Spec.Options.RequireMFAType = types.RequireMFAType_SESSION
				}),
				newRole(func(r *types.RoleV6) {
					r.Spec.Allow.Logins = []string{"root"}
					r.Spec.Options.RequireMFAType = types.RequireMFAType_OFF
				}),
			},
			checks: []check{
				{server: serverNoLabels, login: "root", hasAccess: true},
				{server: serverWorker, login: "root", hasAccess: false},
				{server: serverDB, login: "root", hasAccess: true},
			},
		},
		{
			name: "one role requires MFA and MFA was verified",
			roles: []*types.RoleV6{
				newRole(func(r *types.RoleV6) {
					r.Spec.Allow.Logins = []string{"root"}
					r.Spec.Allow.NodeLabels = types.Labels{"role": []string{"worker"}}
					r.Spec.Options.RequireMFAType = types.RequireMFAType_SESSION
				}),
				newRole(func(r *types.RoleV6) {
					r.Spec.Allow.Logins = []string{"root"}
					r.Spec.Options.RequireMFAType = types.RequireMFAType_OFF
				}),
			},
			mfaVerified: true,
			checks: []check{
				{server: serverNoLabels, login: "root", hasAccess: true},
				{server: serverWorker, login: "root", hasAccess: true},
				{server: serverDB, login: "root", hasAccess: true},
			},
		},
		{
			name: "cluster requires MFA but MFA was not verified",
			roles: []*types.RoleV6{
				newRole(func(r *types.RoleV6) {
					r.Spec.Allow.Logins = []string{"root"}
				}),
			},
			authSpec: types.AuthPreferenceSpecV2{
				RequireMFAType: types.RequireMFAType_SESSION,
			},
			checks: []check{
				{server: serverNoLabels, login: "root", hasAccess: false},
				{server: serverWorker, login: "root", hasAccess: false},
				{server: serverDB, login: "root", hasAccess: false},
			},
		},
		{
			name: "cluster requires MFA and MFA was verified",
			roles: []*types.RoleV6{
				newRole(func(r *types.RoleV6) {
					r.Spec.Allow.Logins = []string{"root"}
				}),
			},
			authSpec: types.AuthPreferenceSpecV2{
				RequireMFAType: types.RequireMFAType_SESSION,
			},
			mfaVerified: true,
			checks: []check{
				{server: serverNoLabels, login: "root", hasAccess: true},
				{server: serverWorker, login: "root", hasAccess: true},
				{server: serverDB, login: "root", hasAccess: true},
			},
		},
		// MFA with private key policy.
		{
			name: "cluster requires session+hardware key, MFA not verified",
			roles: []*types.RoleV6{
				newRole(func(r *types.RoleV6) {
					r.Spec.Allow.Logins = []string{"root"}
				}),
			},
			authSpec: types.AuthPreferenceSpecV2{
				// Functionally equivalent to "session".
				RequireMFAType: types.RequireMFAType_SESSION_AND_HARDWARE_KEY,
			},
			checks: []check{
				{server: serverNoLabels, login: "root", hasAccess: false},
				{server: serverWorker, login: "root", hasAccess: false},
				{server: serverDB, login: "root", hasAccess: false},
			},
		},
		{
			name: "cluster requires session+hardware key, MFA verified",
			roles: []*types.RoleV6{
				newRole(func(r *types.RoleV6) {
					r.Spec.Allow.Logins = []string{"root"}
				}),
			},
			authSpec: types.AuthPreferenceSpecV2{
				// Functionally equivalent to "session".
				RequireMFAType: types.RequireMFAType_SESSION_AND_HARDWARE_KEY,
			},
			mfaVerified: true,
			checks: []check{
				{server: serverNoLabels, login: "root", hasAccess: true},
				{server: serverWorker, login: "root", hasAccess: true},
				{server: serverDB, login: "root", hasAccess: true},
			},
		},
		{
			name: "cluster requires hardware key touch, MFA not verified",
			roles: []*types.RoleV6{
				newRole(func(r *types.RoleV6) {
					r.Spec.Allow.Logins = []string{"root"}
				}),
			},
			authSpec: types.AuthPreferenceSpecV2{
				// Functionally equivalent to "session".
				RequireMFAType: types.RequireMFAType_HARDWARE_KEY_TOUCH,
			},
			checks: []check{
				{server: serverNoLabels, login: "root", hasAccess: false},
				{server: serverWorker, login: "root", hasAccess: false},
				{server: serverDB, login: "root", hasAccess: false},
			},
		},
		{
			name: "cluster requires hardware key touch, MFA verified",
			roles: []*types.RoleV6{
				newRole(func(r *types.RoleV6) {
					r.Spec.Allow.Logins = []string{"root"}
				}),
			},
			authSpec: types.AuthPreferenceSpecV2{
				// Functionally equivalent to "session".
				RequireMFAType: types.RequireMFAType_HARDWARE_KEY_TOUCH,
			},
			mfaVerified: true,
			checks: []check{
				{server: serverNoLabels, login: "root", hasAccess: true},
				{server: serverWorker, login: "root", hasAccess: true},
				{server: serverDB, login: "root", hasAccess: true},
			},
		},
		{
			name: "cluster requires hardware key pin, MFA not verified",
			roles: []*types.RoleV6{
				newRole(func(r *types.RoleV6) {
					r.Spec.Allow.Logins = []string{"root"}
				}),
			},
			authSpec: types.AuthPreferenceSpecV2{
				// Functionally equivalent to "session".
				RequireMFAType: types.RequireMFAType_HARDWARE_KEY_PIN,
			},
			checks: []check{
				{server: serverNoLabels, login: "root", hasAccess: false},
				{server: serverWorker, login: "root", hasAccess: false},
				{server: serverDB, login: "root", hasAccess: false},
			},
		},
		{
			name: "cluster requires hardware key pin, MFA verified",
			roles: []*types.RoleV6{
				newRole(func(r *types.RoleV6) {
					r.Spec.Allow.Logins = []string{"root"}
				}),
			},
			authSpec: types.AuthPreferenceSpecV2{
				// Functionally equivalent to "session".
				RequireMFAType: types.RequireMFAType_HARDWARE_KEY_PIN,
			},
			mfaVerified: true,
			checks: []check{
				{server: serverNoLabels, login: "root", hasAccess: true},
				{server: serverWorker, login: "root", hasAccess: true},
				{server: serverDB, login: "root", hasAccess: true},
			},
		},
		{
			name: "cluster requires hardware key touch and pin, MFA not verified",
			roles: []*types.RoleV6{
				newRole(func(r *types.RoleV6) {
					r.Spec.Allow.Logins = []string{"root"}
				}),
			},
			authSpec: types.AuthPreferenceSpecV2{
				// Functionally equivalent to "session".
				RequireMFAType: types.RequireMFAType_HARDWARE_KEY_TOUCH_AND_PIN,
			},
			checks: []check{
				{server: serverNoLabels, login: "root", hasAccess: false},
				{server: serverWorker, login: "root", hasAccess: false},
				{server: serverDB, login: "root", hasAccess: false},
			},
		},
		{
			name: "cluster requires hardware key touch and pin, MFA verified",
			roles: []*types.RoleV6{
				newRole(func(r *types.RoleV6) {
					r.Spec.Allow.Logins = []string{"root"}
				}),
			},
			authSpec: types.AuthPreferenceSpecV2{
				// Functionally equivalent to "session".
				RequireMFAType: types.RequireMFAType_HARDWARE_KEY_TOUCH_AND_PIN,
			},
			mfaVerified: true,
			checks: []check{
				{server: serverNoLabels, login: "root", hasAccess: true},
				{server: serverWorker, login: "root", hasAccess: true},
				{server: serverDB, login: "root", hasAccess: true},
			},
		},
		// Device Trust.
		{
			name: "role requires trusted device, device not verified",
			roles: []*types.RoleV6{
				newRole(func(r *types.RoleV6) {
					r.Spec.Allow.Logins = []string{"root"}
					r.Spec.Options.DeviceTrustMode = constants.DeviceTrustModeRequired
				}),
			},
			enableDeviceVerification: true,
			deviceVerified:           false,
			checks: []check{
				{server: serverNoLabels, login: "root", hasAccess: false},
				{server: serverWorker, login: "root", hasAccess: false},
				{server: serverDB, login: "root", hasAccess: false},
			},
		},
		{
			name: "role requires trusted device, device verified",
			roles: []*types.RoleV6{
				newRole(func(r *types.RoleV6) {
					r.Spec.Allow.Logins = []string{"root"}
					r.Spec.Options.DeviceTrustMode = constants.DeviceTrustModeRequired
				}),
			},
			enableDeviceVerification: true,
			deviceVerified:           true,
			checks: []check{
				{server: serverNoLabels, login: "root", hasAccess: true},
				{server: serverWorker, login: "root", hasAccess: true},
				{server: serverDB, login: "root", hasAccess: true},
			},
		},
		{
			name: "role requires trusted device for specific label, device not verified",
			roles: []*types.RoleV6{
				newRole(func(r *types.RoleV6) {
					r.Spec.Allow.Logins = []string{"root"}
				}),
				newRole(func(r *types.RoleV6) {
					r.Spec.Allow.Logins = []string{"root"}
					r.Spec.Allow.NodeLabels = types.Labels{"role": []string{"worker"}}
					r.Spec.Options.DeviceTrustMode = constants.DeviceTrustModeRequired
				}),
			},
			enableDeviceVerification: true,
			deviceVerified:           false,
			checks: []check{
				{server: serverNoLabels, login: "root", hasAccess: true},
				{server: serverWorker, login: "root", hasAccess: false}, // NOK, device not verified
				{server: serverDB, login: "root", hasAccess: true},
			},
		},
		{
			name: "role requires trusted device for specific label, device verified",
			roles: []*types.RoleV6{
				newRole(func(r *types.RoleV6) {
					r.Spec.Allow.Logins = []string{"root"}
				}),
				newRole(func(r *types.RoleV6) {
					r.Spec.Allow.Logins = []string{"root"}
					r.Spec.Allow.NodeLabels = types.Labels{"role": []string{"worker"}}
					r.Spec.Options.DeviceTrustMode = constants.DeviceTrustModeRequired
				}),
			},
			enableDeviceVerification: true,
			deviceVerified:           true,
			checks: []check{
				{server: serverNoLabels, login: "root", hasAccess: true},
				{server: serverWorker, login: "root", hasAccess: true}, // OK, device verified
				{server: serverDB, login: "root", hasAccess: true},
			},
		},
		{
			name: "device verification disabled",
			roles: []*types.RoleV6{
				newRole(func(r *types.RoleV6) {
					r.Spec.Allow.Logins = []string{"root"}
					r.Spec.Options.DeviceTrustMode = constants.DeviceTrustModeRequired
				}),
			},
			enableDeviceVerification: false,
			deviceVerified:           false,
			checks: []check{
				{server: serverNoLabels, login: "root", hasAccess: true},
				{server: serverWorker, login: "root", hasAccess: true},
				{server: serverDB, login: "root", hasAccess: true},
			},
		},
		{
			name: "restrictive role device mode takes precedence",
			roles: []*types.RoleV6{
				newRole(func(r *types.RoleV6) {
					r.Spec.Allow.Logins = []string{"root"}
					r.Spec.Options.DeviceTrustMode = constants.DeviceTrustModeOff
				}),
				newRole(func(r *types.RoleV6) {
					r.Spec.Allow.Logins = []string{"root"}
					r.Spec.Options.DeviceTrustMode = constants.DeviceTrustModeOptional
				}),
				newRole(func(r *types.RoleV6) {
					r.Spec.Allow.Logins = []string{"root"}
					r.Spec.Options.DeviceTrustMode = constants.DeviceTrustModeRequired // wins
				}),
			},
			enableDeviceVerification: true,
			deviceVerified:           false,
			checks: []check{
				{server: serverNoLabels, login: "root", hasAccess: false},
				{server: serverWorker, login: "root", hasAccess: false},
				{server: serverDB, login: "root", hasAccess: false},
			},
		},
		// MFA + Device verification.
		{
			name: "MFA and device required, fails MFA",
			roles: []*types.RoleV6{
				newRole(func(r *types.RoleV6) {
					r.Spec.Allow.Logins = []string{"root"}
					r.Spec.Options.RequireMFAType = types.RequireMFAType_SESSION
				}),
				newRole(func(r *types.RoleV6) {
					r.Spec.Allow.Logins = []string{"root"}
					r.Spec.Options.DeviceTrustMode = constants.DeviceTrustModeRequired
				}),
			},
			enableDeviceVerification: true,
			mfaVerified:              false,
			deviceVerified:           true,
			checks: []check{
				{server: serverNoLabels, login: "root", hasAccess: false},
				{server: serverWorker, login: "root", hasAccess: false},
				{server: serverDB, login: "root", hasAccess: false},
			},
		},
		{
			name: "MFA and device required, fails device",
			roles: []*types.RoleV6{
				newRole(func(r *types.RoleV6) {
					r.Spec.Allow.Logins = []string{"root"}
					r.Spec.Options.RequireMFAType = types.RequireMFAType_SESSION
				}),
				newRole(func(r *types.RoleV6) {
					r.Spec.Allow.Logins = []string{"root"}
					r.Spec.Options.DeviceTrustMode = constants.DeviceTrustModeRequired
				}),
			},
			enableDeviceVerification: true,
			mfaVerified:              true,
			deviceVerified:           false,
			checks: []check{
				{server: serverNoLabels, login: "root", hasAccess: false},
				{server: serverWorker, login: "root", hasAccess: false},
				{server: serverDB, login: "root", hasAccess: false},
			},
		},
		{
			name: "MFA and device required, passes all",
			roles: []*types.RoleV6{
				newRole(func(r *types.RoleV6) {
					r.Spec.Allow.Logins = []string{"root"}
					r.Spec.Options.RequireMFAType = types.RequireMFAType_SESSION
				}),
				newRole(func(r *types.RoleV6) {
					r.Spec.Allow.Logins = []string{"root"}
					r.Spec.Options.DeviceTrustMode = constants.DeviceTrustModeRequired
				}),
			},
			enableDeviceVerification: true,
			mfaVerified:              true,
			deviceVerified:           true,
			checks: []check{
				{server: serverNoLabels, login: "root", hasAccess: true},
				{server: serverWorker, login: "root", hasAccess: true},
				{server: serverDB, login: "root", hasAccess: true},
			},
		},
		{
			name: "label expressions",
			roles: []*types.RoleV6{
				newRole(func(r *types.RoleV6) {
					r.Spec.Allow.NodeLabels = nil
					r.Spec.Allow.NodeLabelsExpression = `labels.role == "worker" && labels.status == "follower"`
					r.Spec.Allow.Logins = []string{"root"}
				}),
			},
			checks: []check{
				{server: serverNoLabels, login: "root", hasAccess: false},
				{server: serverWorker, login: "root", hasAccess: true},
				{server: serverDB, login: "root", hasAccess: false},
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			authPref, err := types.NewAuthPreference(tc.authSpec)
			require.NoError(t, err, "NewAuthPreference failed")

			accessChecker := makeAccessCheckerWithRolePointers(tc.roles)
			for j, check := range tc.checks {
				comment := fmt.Sprintf("check #%v: user: %v, server: %v, should access: %v", j, check.login, check.server.GetName(), check.hasAccess)
				state := accessChecker.GetAccessState(authPref)
				state.MFAVerified = tc.mfaVerified
				state.EnableDeviceVerification = tc.enableDeviceVerification
				state.DeviceVerified = tc.deviceVerified
				err := accessChecker.CheckAccess(
					check.server,
					state,
					NewLoginMatcher(check.login))
				if check.hasAccess {
					require.NoError(t, err, comment)
				} else {
					require.True(t, trace.IsAccessDenied(err), "Got err = %v/%T, wanted AccessDenied. %v", err, err, comment)
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
	require.NoError(t, rcA.CheckAndSetDefaults())
	require.NoError(t, rcB.CheckAndSetDefaults())
	require.NoError(t, rcC.CheckAndSetDefaults())
	testCases := []struct {
		name   string
		roles  []types.RoleV6
		checks []check
	}{
		{
			name:  "empty role set has access to nothing",
			roles: []types.RoleV6{},
			checks: []check{
				{rc: rcA, hasAccess: false},
				{rc: rcB, hasAccess: false},
				{rc: rcC, hasAccess: false},
			},
		},
		{
			name: "role matches any label out of multiple labels",
			roles: []types.RoleV6{
				{
					Metadata: types.Metadata{
						Name:      "name1",
						Namespace: apidefaults.Namespace,
					},
					Spec: types.RoleSpecV6{
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
			roles: []types.RoleV6{
				{
					Metadata: types.Metadata{
						Name:      "name1",
						Namespace: apidefaults.Namespace,
					},
					Spec: types.RoleSpecV6{
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
			roles: []types.RoleV6{
				{
					Metadata: types.Metadata{
						Name:      "name1",
						Namespace: apidefaults.Namespace,
					},
					Spec: types.RoleSpecV6{
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
			roles: []types.RoleV6{
				{
					Metadata: types.Metadata{
						Name:      "name1",
						Namespace: apidefaults.Namespace,
					},
					Spec: types.RoleSpecV6{
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
					Spec: types.RoleSpecV6{
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
			roles: []types.RoleV6{
				{
					Metadata: types.Metadata{
						Name:      "name1",
						Namespace: apidefaults.Namespace,
					},
					Spec: types.RoleSpecV6{
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
			roles: []types.RoleV6{
				{
					Metadata: types.Metadata{
						Name:      "name1",
						Namespace: apidefaults.Namespace,
					},
					Spec: types.RoleSpecV6{
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
					Spec: types.RoleSpecV6{
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
			roles: []types.RoleV6{
				{
					Metadata: types.Metadata{
						Name:      "name1",
						Namespace: apidefaults.Namespace,
					},
					Spec: types.RoleSpecV6{
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
		{
			name: "label expressions",
			roles: []types.RoleV6{
				*newRole(func(r *types.RoleV6) {
					r.Spec.Allow.ClusterLabelsExpression = `labels.role == "worker" && labels.status == "follower"`
				}),
			},
			checks: []check{
				{rc: rcA, hasAccess: false},
				{rc: rcB, hasAccess: true},
				{rc: rcC, hasAccess: false},
			},
		},
	}
	for i, tc := range testCases {
		accessChecker := makeAccessCheckerWithRoles(tc.roles)
		for j, check := range tc.checks {
			comment := fmt.Sprintf("test case %v '%v', check %v", i, tc.name, j)
			result := accessChecker.CheckAccessToRemoteCluster(check.rc)
			if check.hasAccess {
				require.NoError(t, result, comment)
			} else {
				require.True(t, trace.IsAccessDenied(result), fmt.Sprintf("%v: %v", comment, result))
			}
		}
	}
}

func makeAccessCheckerWithRoles(roles []types.RoleV6) AccessChecker {
	roleSet := make(RoleSet, len(roles))
	for i := range roles {
		roleSet[i] = &roles[i]
	}
	return makeAccessCheckerWithRoleSet(roleSet)
}

func makeAccessCheckerWithRolePointers(roles []*types.RoleV6) AccessChecker {
	roleSet := make(RoleSet, len(roles))
	for i := range roles {
		roleSet[i] = roles[i]
	}
	return makeAccessCheckerWithRoleSet(roleSet)
}

func makeAccessCheckerWithRoleSet(roleSet RoleSet) AccessChecker {
	roleNames := make([]string, len(roleSet))
	for i, role := range roleSet {
		roleNames[i] = role.GetName()
	}
	accessInfo := &AccessInfo{
		Username:           "alice",
		Roles:              roleNames,
		Traits:             nil,
		AllowedResourceIDs: nil,
	}
	return NewAccessCheckerWithRoleSet(accessInfo, "clustername", roleSet)
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
		roles  []types.RoleV6
		checks []check
	}{
		{
			name:  "0 - empty role set has access to nothing",
			roles: []types.RoleV6{},
			checks: []check{
				{rule: types.KindUser, verb: types.ActionWrite, namespace: apidefaults.Namespace, hasAccess: false},
			},
		},
		{
			name: "1 - user can read session but can't list in default namespace",
			roles: []types.RoleV6{
				{
					Metadata: types.Metadata{
						Name:      "name1",
						Namespace: apidefaults.Namespace,
					},
					Spec: types.RoleSpecV6{
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
			roles: []types.RoleV6{
				{
					Metadata: types.Metadata{
						Name:      "name1",
						Namespace: apidefaults.Namespace,
					},
					Spec: types.RoleSpecV6{
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
					Spec: types.RoleSpecV6{
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
			roles: []types.RoleV6{
				{
					Metadata: types.Metadata{
						Name:      "name1",
						Namespace: apidefaults.Namespace,
					},
					Spec: types.RoleSpecV6{
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
			roles: []types.RoleV6{
				{
					Metadata: types.Metadata{
						Name:      "name1",
						Namespace: apidefaults.Namespace,
					},
					Spec: types.RoleSpecV6{
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
			roles: []types.RoleV6{
				{
					Metadata: types.Metadata{
						Name:      "name1",
						Namespace: apidefaults.Namespace,
					},
					Spec: types.RoleSpecV6{
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
							Resource: &types.RoleV6{
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
			roles: []types.RoleV6{
				{
					Metadata: types.Metadata{
						Name:      "name1",
						Namespace: apidefaults.Namespace,
					},
					Spec: types.RoleSpecV6{
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
							Resource: &types.RoleV6{
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
			result := set.CheckAccessToRule(&check.context, check.namespace, check.rule, check.verb)
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
	ownSessions, err := types.NewRole("own-sessions", types.RoleSpecV6{
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
	ownSSHSessions, err := types.NewRole("own-ssh-sessions", types.RoleSpecV6{
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
	readAllSessions, err := types.NewRole("all-sessions", types.RoleSpecV6{
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
	allowSSHSessions, err := types.NewRole("all-ssh-sessions", types.RoleSpecV6{
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
	denySSHSessions, err := types.NewRole("deny-ssh-sessions", types.RoleSpecV6{
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
			for _, verb := range params.verbs {
				err := test.roles.CheckAccessToRule(&params.ctx, params.namespace, params.resource, verb)
				if gotAccess, wantAccess := err == nil, test.wantRuleAccess; gotAccess != wantAccess {
					t.Errorf("CheckAccessToRule(verb=%q) returned err = %v=q, wantAccess = %v", verb, err, wantAccess)
				}

				err = test.roles.GuessIfAccessIsPossible(&params.ctx, params.namespace, params.resource, verb)
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
		inLogins                []string
		outLogins               []string
		inWindowsLogins         []string
		outWindowsLogins        []string
		inRoleARNs              []string
		outRoleARNs             []string
		inAzureIdentities       []string
		outAzureIdentities      []string
		inGCPServiceAccounts    []string
		outGCPServiceAccounts   []string
		inLabels                types.Labels
		outLabels               types.Labels
		inKubeLabels            types.Labels
		outKubeLabels           types.Labels
		inKubeGroups            []string
		outKubeGroups           []string
		inKubeUsers             []string
		outKubeUsers            []string
		inAppLabels             types.Labels
		outAppLabels            types.Labels
		inGroupLabels           types.Labels
		outGroupLabels          types.Labels
		inDBLabels              types.Labels
		outDBLabels             types.Labels
		inWindowsDesktopLabels  types.Labels
		outWindowsDesktopLabels types.Labels
		inDBNames               []string
		outDBNames              []string
		inDBUsers               []string
		outDBUsers              []string
		inDBRoles               []string
		outDBRoles              []string
		inImpersonate           types.ImpersonateConditions
		outImpersonate          types.ImpersonateConditions
		inSudoers               []string
		outSudoers              []string
	}
	tests := []struct {
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
			comment: "logins substitute in allow rule with multiple functions and regexps",
			inTraits: map[string][]string{
				"email": {"ab_cd@example.com"},
			},
			allow: rule{
				inLogins: []string{
					`{{regexp.replace(external.email, "_", "")}}`,
					`{{email.local(external.email)}}`,
					`{{regexp.replace(email.local(external.email), "_", "")}}`,
					`{{regexp.replace(external.email, "d", "e")}}`,
					`{{email.local(regexp.replace(external.email, "d", "e"))}}`,
					`{{regexp.replace(regexp.replace(email.local(regexp.replace(external.email, "cd", "z")), "ab", "xy"), "_", "")}}`,
					"root",
				},
				outLogins: []string{
					"abcd@example.com",
					"ab_cd",
					"abcd",
					"ab_ce@example.com",
					"ab_ce",
					"xyz",
					"root",
				},
			},
		},
		{
			comment:  "logins substitute in allow rule can have constant expressions",
			inTraits: map[string][]string{},
			allow: rule{
				inLogins: []string{
					`{{regexp.replace("vitor@gravitational.com", "gravitational", "goteleport")}}`,
					`{{email.local("vitor@goteleport.com")}}`,
					`{{email.local(regexp.replace("vitor.enes@gravitational.com", "gravitational", "goteleport"))}}`,
					"root",
				},
				outLogins: []string{
					"vitor@goteleport.com",
					"vitor",
					"vitor.enes",
					"root",
				},
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
				"foo":                      {"bar"},
				constants.TraitAWSRoleARNs: {"baz"},
			},
			allow: rule{
				inRoleARNs:  []string{"{{external.foo}}", teleport.TraitInternalAWSRoleARNs},
				outRoleARNs: []string{"bar", "baz"},
			},
		},
		{
			comment: "AWS role ARN substitute in deny rule",
			inTraits: map[string][]string{
				"foo":                      {"bar"},
				constants.TraitAWSRoleARNs: {"baz"},
			},
			deny: rule{
				inRoleARNs:  []string{"{{external.foo}}", teleport.TraitInternalAWSRoleARNs},
				outRoleARNs: []string{"bar", "baz"},
			},
		},
		{
			comment: "Azure identity substitute in allow rule",
			inTraits: map[string][]string{
				"foo":                          {"/subscriptions/1020304050607-cafe-8090-a0b0c0d0e0f0/resourceGroups/external-foo/providers/Microsoft.ManagedIdentity/userAssignedIdentities/teleport-azure"},
				constants.TraitAzureIdentities: {"/subscriptions/1020304050607-cafe-8090-a0b0c0d0e0f0/resourceGroups/internal-azure-identities/providers/Microsoft.ManagedIdentity/userAssignedIdentities/teleport-azure"},
			},
			deny: rule{
				inAzureIdentities: []string{"{{external.foo}}", teleport.TraitInternalAzureIdentities},
				outAzureIdentities: []string{
					"/subscriptions/1020304050607-cafe-8090-a0b0c0d0e0f0/resourceGroups/external-foo/providers/Microsoft.ManagedIdentity/userAssignedIdentities/teleport-azure",
					"/subscriptions/1020304050607-cafe-8090-a0b0c0d0e0f0/resourceGroups/internal-azure-identities/providers/Microsoft.ManagedIdentity/userAssignedIdentities/teleport-azure",
				},
			},
		},
		{
			comment: "Azure identity substitute in deny rule",
			inTraits: map[string][]string{
				"foo":                          {"/subscriptions/1020304050607-cafe-8090-a0b0c0d0e0f0/resourceGroups/external-foo/providers/Microsoft.ManagedIdentity/userAssignedIdentities/teleport-azure"},
				constants.TraitAzureIdentities: {"/subscriptions/1020304050607-cafe-8090-a0b0c0d0e0f0/resourceGroups/internal-azure-identities/providers/Microsoft.ManagedIdentity/userAssignedIdentities/teleport-azure"},
			},
			deny: rule{
				inAzureIdentities: []string{"{{external.foo}}", teleport.TraitInternalAzureIdentities},
				outAzureIdentities: []string{
					"/subscriptions/1020304050607-cafe-8090-a0b0c0d0e0f0/resourceGroups/external-foo/providers/Microsoft.ManagedIdentity/userAssignedIdentities/teleport-azure",
					"/subscriptions/1020304050607-cafe-8090-a0b0c0d0e0f0/resourceGroups/internal-azure-identities/providers/Microsoft.ManagedIdentity/userAssignedIdentities/teleport-azure",
				},
			},
		},
		{
			comment: "GCP service account substitute in allow rule",
			inTraits: map[string][]string{
				"foo":                             {"bar"},
				constants.TraitGCPServiceAccounts: {"baz"},
			},
			allow: rule{
				inGCPServiceAccounts:  []string{"{{external.foo}}", teleport.TraitInternalGCPServiceAccounts},
				outGCPServiceAccounts: []string{"bar", "baz"},
			},
		},
		{
			comment: "GCP service account substitute in deny rule",
			inTraits: map[string][]string{
				"foo":                             {"bar"},
				constants.TraitGCPServiceAccounts: {"baz"},
			},
			deny: rule{
				inGCPServiceAccounts:  []string{"{{external.foo}}", teleport.TraitInternalGCPServiceAccounts},
				outGCPServiceAccounts: []string{"bar", "baz"},
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
			comment: "database name/user/role external vars in allow rule",
			inTraits: map[string][]string{
				"foo": {"bar"},
			},
			allow: rule{
				inDBNames:  []string{"{{external.foo}}", "{{external.baz}}", "postgres"},
				outDBNames: []string{"bar", "postgres"},
				inDBUsers:  []string{"{{external.foo}}", "{{external.baz}}", "postgres"},
				outDBUsers: []string{"bar", "postgres"},
				inDBRoles:  []string{"{{external.foo}}", "{{external.baz}}", "postgres"},
				outDBRoles: []string{"bar", "postgres"},
			},
		},
		{
			comment: "database name/user/role external vars in deny rule",
			inTraits: map[string][]string{
				"foo": {"bar"},
			},
			deny: rule{
				inDBNames:  []string{"{{external.foo}}", "{{external.baz}}", "postgres"},
				outDBNames: []string{"bar", "postgres"},
				inDBUsers:  []string{"{{external.foo}}", "{{external.baz}}", "postgres"},
				outDBUsers: []string{"bar", "postgres"},
				inDBRoles:  []string{"{{external.foo}}", "{{external.baz}}", "postgres"},
				outDBRoles: []string{"bar", "postgres"},
			},
		},
		{
			comment: "database name/user/role internal vars in allow rule",
			inTraits: map[string][]string{
				"db_names": {"db1", "db2"},
				"db_users": {"alice"},
			},
			allow: rule{
				inDBNames:  []string{"{{internal.db_names}}", "{{internal.foo}}", "postgres"},
				outDBNames: []string{"db1", "db2", "postgres"},
				inDBUsers:  []string{"{{internal.db_users}}", "{{internal.foo}}", "postgres"},
				outDBUsers: []string{"alice", "postgres"},
				inDBRoles:  []string{"{{internal.db_roles}}", "{{internal.foo}}", "postgres"},
				outDBRoles: []string{"alice", "postgres"},
			},
		},
		{
			comment: "database name/user/role internal vars in deny rule",
			inTraits: map[string][]string{
				"db_names": {"db1", "db2"},
				"db_users": {"alice"},
			},
			deny: rule{
				inDBNames:  []string{"{{internal.db_names}}", "{{internal.foo}}", "postgres"},
				outDBNames: []string{"db1", "db2", "postgres"},
				inDBUsers:  []string{"{{internal.db_users}}", "{{internal.foo}}", "postgres"},
				outDBUsers: []string{"alice", "postgres"},
				inDBRoles:  []string{"{{internal.db_roles}}", "{{internal.foo}}", "postgres"},
				outDBRoles: []string{"alice", "postgres"},
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
			comment: "values are expanded in group labels",
			inTraits: map[string][]string{
				"foo": {"bar", "baz"},
			},
			allow: rule{
				inGroupLabels:  types.Labels{`key`: []string{`{{external.foo}}`}},
				outGroupLabels: types.Labels{`key`: []string{"bar", "baz"}},
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
			comment: "values are expanded in windows desktop labels",
			inTraits: map[string][]string{
				"foo": {"bar", "baz"},
			},
			allow: rule{
				inWindowsDesktopLabels:  types.Labels{`key`: []string{`{{external.foo}}`}},
				outWindowsDesktopLabels: types.Labels{`key`: []string{"bar", "baz"}},
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
		{
			comment: "sudoers substitution roles",
			inTraits: map[string][]string{
				"users": {"alice", "bob"},
			},
			allow: rule{
				inSudoers: []string{"{{external.users}} ALL=(ALL) ALL"},
				outSudoers: []string{
					"alice ALL=(ALL) ALL",
					"bob ALL=(ALL) ALL",
				},
			},
		},
		{
			comment: "sudoers substitution not found trait",
			inTraits: map[string][]string{
				"users": {"alice", "bob"},
			},
			allow: rule{
				inSudoers: []string{
					"{{external.hello}} ALL=(ALL) ALL",
					"{{external.users}} ALL=(test) ALL",
				},
				outSudoers: []string{
					"alice ALL=(test) ALL",
					"bob ALL=(test) ALL",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.comment, func(t *testing.T) {
			role := &types.RoleV6{
				Kind:    types.KindRole,
				Version: types.V3,
				Metadata: types.Metadata{
					Name:      "name1",
					Namespace: apidefaults.Namespace,
				},
				Spec: types.RoleSpecV6{
					Allow: types.RoleConditions{
						Logins:               tt.allow.inLogins,
						WindowsDesktopLogins: tt.allow.inWindowsLogins,
						NodeLabels:           tt.allow.inLabels,
						ClusterLabels:        tt.allow.inLabels,
						KubernetesLabels:     tt.allow.inKubeLabels,
						KubeGroups:           tt.allow.inKubeGroups,
						KubeUsers:            tt.allow.inKubeUsers,
						AppLabels:            tt.allow.inAppLabels,
						GroupLabels:          tt.allow.inGroupLabels,
						AWSRoleARNs:          tt.allow.inRoleARNs,
						AzureIdentities:      tt.allow.inAzureIdentities,
						GCPServiceAccounts:   tt.allow.inGCPServiceAccounts,
						DatabaseLabels:       tt.allow.inDBLabels,
						DatabaseNames:        tt.allow.inDBNames,
						DatabaseUsers:        tt.allow.inDBUsers,
						WindowsDesktopLabels: tt.allow.inWindowsDesktopLabels,
						Impersonate:          &tt.allow.inImpersonate,
						HostSudoers:          tt.allow.inSudoers,
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
						GroupLabels:          tt.deny.inGroupLabels,
						AWSRoleARNs:          tt.deny.inRoleARNs,
						AzureIdentities:      tt.deny.inAzureIdentities,
						GCPServiceAccounts:   tt.deny.inGCPServiceAccounts,
						DatabaseLabels:       tt.deny.inDBLabels,
						DatabaseNames:        tt.deny.inDBNames,
						DatabaseUsers:        tt.deny.inDBUsers,
						WindowsDesktopLabels: tt.deny.inWindowsDesktopLabels,
						Impersonate:          &tt.deny.inImpersonate,
						HostSudoers:          tt.deny.outSudoers,
					},
				},
			}

			outRole, err := ApplyTraits(role, tt.inTraits)
			require.NoError(t, err)
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
				require.Equal(t, rule.spec.outGroupLabels, outRole.GetGroupLabels(rule.condition))
				require.Equal(t, rule.spec.outRoleARNs, outRole.GetAWSRoleARNs(rule.condition))
				require.Equal(t, rule.spec.outAzureIdentities, outRole.GetAzureIdentities(rule.condition))
				require.Equal(t, rule.spec.outGCPServiceAccounts, outRole.GetGCPServiceAccounts(rule.condition))
				require.Equal(t, rule.spec.outDBLabels, outRole.GetDatabaseLabels(rule.condition))
				require.Equal(t, rule.spec.outDBNames, outRole.GetDatabaseNames(rule.condition))
				require.Equal(t, rule.spec.outDBUsers, outRole.GetDatabaseUsers(rule.condition))
				require.Equal(t, rule.spec.outWindowsDesktopLabels, outRole.GetWindowsDesktopLabels(rule.condition))
				require.Equal(t, rule.spec.outImpersonate, outRole.GetImpersonateConditions(rule.condition))
				require.Equal(t, rule.spec.outSudoers, outRole.GetHostSudoers(rule.condition))
			}
		})
	}
}

// TestExtractFrom makes sure roles and traits are extracted from SSH and TLS
// certificates not services.User.
func TestExtractFrom(t *testing.T) {
	ctx := context.Background()
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

	roles, traits, err = ExtractFromIdentity(ctx, &userGetter{
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

	roles, traits, err = ExtractFromIdentity(ctx, &userGetter{
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
	tests := []struct {
		inOptions                  types.RoleOptions
		outCanPortForward          bool
		outCanForwardAgents        bool
		outRecordDesktopSessions   bool
		outDesktopClipboard        bool
		outDesktopDirectorySharing bool
	}{
		// Setting options explicitly off should remain off.
		{
			inOptions: types.RoleOptions{
				ForwardAgent:            types.NewBool(false),
				PortForwarding:          types.NewBoolOption(false),
				RecordSession:           &types.RecordSession{Desktop: types.NewBoolOption(false)},
				DesktopClipboard:        types.NewBoolOption(false),
				DesktopDirectorySharing: types.NewBoolOption(false),
			},
			outCanPortForward:          false,
			outCanForwardAgents:        false,
			outRecordDesktopSessions:   false,
			outDesktopClipboard:        false,
			outDesktopDirectorySharing: false,
		},
		// Not setting options should set port forwarding to true (default enabled),
		// agent forwarding false (default disabled),
		// desktop session recording to true (default enabled),
		// desktop clipboard sharing to true (default enabled),
		// and desktop directory sharing to true (default enabled).
		{
			inOptions:                  types.RoleOptions{},
			outCanPortForward:          true,
			outCanForwardAgents:        false,
			outRecordDesktopSessions:   true,
			outDesktopClipboard:        true,
			outDesktopDirectorySharing: true,
		},
		// Explicitly enabling should enable them.
		{
			inOptions: types.RoleOptions{
				ForwardAgent:            types.NewBool(true),
				PortForwarding:          types.NewBoolOption(true),
				RecordSession:           &types.RecordSession{Desktop: types.NewBoolOption(true)},
				DesktopClipboard:        types.NewBoolOption(true),
				DesktopDirectorySharing: types.NewBoolOption(true),
			},
			outCanPortForward:          true,
			outCanForwardAgents:        true,
			outRecordDesktopSessions:   true,
			outDesktopClipboard:        true,
			outDesktopDirectorySharing: true,
		},
	}
	for _, tt := range tests {
		set := NewRoleSet(&types.RoleV6{
			Kind:    types.KindRole,
			Version: types.V3,
			Metadata: types.Metadata{
				Name:      "role-name",
				Namespace: apidefaults.Namespace,
			},
			Spec: types.RoleSpecV6{
				Options: tt.inOptions,
			},
		})
		require.Equal(t, tt.outCanPortForward, set.CanPortForward())
		require.Equal(t, tt.outCanForwardAgents, set.CanForwardAgents())
		require.Equal(t, tt.outRecordDesktopSessions, set.RecordDesktopSession())
		require.Equal(t, tt.outDesktopClipboard, set.DesktopClipboard())
		require.Equal(t, tt.outDesktopDirectorySharing, set.DesktopDirectorySharing())
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
	roleDevStage := &types.RoleV6{
		Metadata: types.Metadata{Name: "dev-stage", Namespace: apidefaults.Namespace},
		Version:  types.V3,
		Spec: types.RoleSpecV6{
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
	roleDevProd := &types.RoleV6{
		Metadata: types.Metadata{Name: "dev-prod", Namespace: apidefaults.Namespace},
		Version:  types.V3,
		Spec: types.RoleSpecV6{
			Allow: types.RoleConditions{
				Namespaces:               []string{apidefaults.Namespace},
				DatabaseLabelsExpression: `labels["env"] == "prod"`,
				DatabaseNames:            []string{"test"},
				DatabaseUsers:            []string{"dev"},
			},
		},
	}
	require.NoError(t, roleDevProd.CheckAndSetDefaults())
	roleDevProdWithMFA := &types.RoleV6{
		Metadata: types.Metadata{Name: "dev-prod-mfa", Namespace: apidefaults.Namespace},
		Version:  types.V3,
		Spec: types.RoleSpecV6{
			Options: types.RoleOptions{
				RequireMFAType: types.RequireMFAType_SESSION,
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
	roleDevProdWithDeviceTrust := &types.RoleV6{
		Metadata: types.Metadata{Name: "dev-prod-devicetrust", Namespace: apidefaults.Namespace},
		Version:  types.V3,
		Spec: types.RoleSpecV6{
			Options: types.RoleOptions{
				DeviceTrustMode: constants.DeviceTrustModeRequired,
			},
			Allow: types.RoleConditions{
				Namespaces:     []string{apidefaults.Namespace},
				DatabaseLabels: types.Labels{"env": []string{"prod"}},
				DatabaseNames:  []string{"test"},
				DatabaseUsers:  []string{"dev"},
			},
		},
	}
	require.NoError(t, roleDevProdWithDeviceTrust.CheckAndSetDefaults())

	// Database labels are not set in allow/deny rules on purpose to test
	// that they're set during check and set defaults below.
	roleDeny := &types.RoleV6{
		Metadata: types.Metadata{Name: "deny", Namespace: apidefaults.Namespace},
		Version:  types.V3,
		Spec: types.RoleSpecV6{
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
		name   string
		roles  RoleSet
		access []access
		state  AccessState
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
			name:  "prod database requires MFA, no MFA provided",
			roles: RoleSet{roleDevStage, roleDevProdWithMFA, roleDevProd},
			state: AccessState{MFAVerified: false},
			access: []access{
				{server: dbStage, dbName: "test", dbUser: "dev", access: true},
				{server: dbProd, dbName: "test", dbUser: "dev", access: false},
			},
		},
		{
			name:  "prod database requires MFA, MFA provided",
			roles: RoleSet{roleDevStage, roleDevProdWithMFA, roleDevProd},
			state: AccessState{MFAVerified: true},
			access: []access{
				{server: dbStage, dbName: "test", dbUser: "dev", access: true},
				{server: dbProd, dbName: "test", dbUser: "dev", access: true},
			},
		},
		{
			name:   "cluster requires MFA, no MFA provided",
			roles:  RoleSet{roleDevStage, roleDevProdWithMFA, roleDevProd},
			state:  AccessState{MFAVerified: false, MFARequired: MFARequiredAlways},
			access: []access{},
		},
		{
			name:  "cluster requires MFA, MFA provided",
			roles: RoleSet{roleDevStage, roleDevProdWithMFA, roleDevProd},
			state: AccessState{MFAVerified: true, MFARequired: MFARequiredAlways},
			access: []access{
				{server: dbStage, dbName: "test", dbUser: "dev", access: true},
				{server: dbProd, dbName: "test", dbUser: "dev", access: true},
			},
		},
		{
			name:  "roles requires trusted device, device not verified",
			roles: RoleSet{roleDevStage, roleDevProd, roleDevProdWithDeviceTrust},
			state: AccessState{
				EnableDeviceVerification: true,
				DeviceVerified:           false,
			},
			access: []access{
				{server: dbStage, dbName: "test", dbUser: "dev", access: true},
				{server: dbProd, dbName: "test", dbUser: "dev", access: false},
			},
		},
		{
			name:  "roles requires trusted device, device verified",
			roles: RoleSet{roleDevStage, roleDevProd, roleDevProdWithDeviceTrust},
			state: AccessState{
				EnableDeviceVerification: true,
				DeviceVerified:           true,
			},
			access: []access{
				{server: dbStage, dbName: "test", dbUser: "dev", access: true},
				{server: dbProd, dbName: "test", dbUser: "dev", access: true},
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			for _, access := range tc.access {
				err := tc.roles.checkAccess(access.server, wrappers.Traits{}, tc.state,
					NewDatabaseUserMatcher(access.server, access.dbUser),
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
	roleDevStage := &types.RoleV6{
		Metadata: types.Metadata{Name: "dev-stage", Namespace: apidefaults.Namespace},
		Spec: types.RoleSpecV6{
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
	roleDevProd := &types.RoleV6{
		Metadata: types.Metadata{Name: "dev-prod", Namespace: apidefaults.Namespace},
		Spec: types.RoleSpecV6{
			Allow: types.RoleConditions{
				Namespaces:               []string{apidefaults.Namespace},
				DatabaseLabelsExpression: `labels["env"] == "prod"`,
				DatabaseUsers:            []string{"dev"},
			},
		},
	}

	dbRequireAWSRoles, err := types.NewDatabaseV3(types.Metadata{
		Name: "dynamodb",
	}, types.DatabaseSpecV3{
		Protocol: "dynamodb",
		URI:      "test.xxxxxxx.mongodb.net",
		AWS: types.AWS{
			AccountID: "123456789012",
			Region:    "us-east-1",
		},
	})
	require.NoError(t, err)
	require.True(t, dbRequireAWSRoles.RequireAWSIAMRolesAsUsers())
	roleWithAWSRoles := &types.RoleV6{
		Metadata: types.Metadata{Name: "aws-roles", Namespace: apidefaults.Namespace},
		Spec: types.RoleSpecV6{
			Allow: types.RoleConditions{
				Namespaces:     []string{apidefaults.Namespace},
				DatabaseLabels: types.Labels{types.Wildcard: []string{types.Wildcard}},
				DatabaseUsers: []string{
					"allow-role-with-short-name",
					"arn:aws:iam::123456789012:role/allow-role-with-full-arn",
				},
			},
		},
	}

	roleWithUsersAndAWSRoles := &types.RoleV6{
		Metadata: types.Metadata{Name: "users-and-aws-roles", Namespace: apidefaults.Namespace},
		Spec: types.RoleSpecV6{
			Allow: types.RoleConditions{
				Namespaces:     []string{apidefaults.Namespace},
				DatabaseLabels: types.Labels{types.Wildcard: []string{types.Wildcard}},
				DatabaseUsers: []string{
					"regular-user",
					"role/allow-role-with-partial-arn",
					"arn:aws:iam::123456789012:role/allow-role-with-full-arn",
				},
			},
		},
	}
	dbSupportAWSRoles, err := types.NewDatabaseV3(types.Metadata{
		Name: "mongo-atlas",
	}, types.DatabaseSpecV3{
		Protocol: "mongodb",
		URI:      "test.xxxxxxx.mongodb.net",
		MongoAtlas: types.MongoAtlas{
			Name: "instance",
		},
		AWS: types.AWS{
			AccountID: "123456789012",
			Region:    "us-east-1",
		},
	})
	require.NoError(t, err)
	require.True(t, dbSupportAWSRoles.SupportAWSIAMRoleARNAsUsers())

	dbCockroachStage, err := types.NewDatabaseV3(types.Metadata{
		Name:   "cockroachdb",
		Labels: map[string]string{"env": "stage"},
	}, types.DatabaseSpecV3{
		Protocol: "cockroachdb",
		URI:      "cockroachdb:26257",
	})
	require.NoError(t, err)
	dbCockroachProd, err := types.NewDatabaseV3(types.Metadata{
		Name:   "cockroachdb",
		Labels: map[string]string{"env": "prod"},
	}, types.DatabaseSpecV3{
		Protocol: "cockroachdb",
		URI:      "cockroachdb:26257",
	})
	require.NoError(t, err)

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
				{server: dbStage, dbUser: "SUPERUSER", access: true},
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
		{
			name:  "database types require AWS roles as database users",
			roles: RoleSet{roleWithAWSRoles},
			access: []access{
				{server: dbRequireAWSRoles, dbUser: "allow-role-with-short-name", access: true},
				{server: dbRequireAWSRoles, dbUser: "allow-role-with-full-arn", access: true},
				{server: dbRequireAWSRoles, dbUser: "arn:aws:iam::123456789012:role/allow-role-with-full-arn", access: true},
				{server: dbRequireAWSRoles, dbUser: "arn:aws:iam::123456789012:role/allow-role-with-short-name", access: true},
				{server: dbRequireAWSRoles, dbUser: "unknown-role-name", access: false},
				{server: dbRequireAWSRoles, dbUser: "arn:aws:iam::123456789012:role/unknown-role-name", access: false},
				{server: dbRequireAWSRoles, dbUser: "arn:aws:iam::123456789012:user/username", access: false},
				{server: dbRequireAWSRoles, dbUser: "arn:aws-cn:iam::123456789012:role/allow-role-with-short-name", access: false},
			},
		},
		{
			name:  "database types support AWS roles and regular users",
			roles: RoleSet{roleWithUsersAndAWSRoles},
			access: []access{
				{server: dbSupportAWSRoles, dbUser: "role/allow-role-with-partial-arn", access: true},
				{server: dbSupportAWSRoles, dbUser: "arn:aws:iam::123456789012:role/allow-role-with-partial-arn", access: true},
				{server: dbSupportAWSRoles, dbUser: "role/unknown-role", access: false},
				{server: dbSupportAWSRoles, dbUser: "allow-role-with-partial-arn", access: false},
				{server: dbSupportAWSRoles, dbUser: "arn:aws:iam::123456789012:role/allow-role-with-full-arn", access: true},
				{server: dbSupportAWSRoles, dbUser: "role/allow-role-with-full-arn", access: true},
				{server: dbSupportAWSRoles, dbUser: "arn:aws:iam::123456789012:role/unknown-role", access: false},
				{server: dbSupportAWSRoles, dbUser: "regular-user", access: true},
				{server: dbSupportAWSRoles, dbUser: "role/regular-user", access: false},
				{server: dbSupportAWSRoles, dbUser: "arn:aws:iam::123456789012:role/regular-user", access: false},
				{server: dbSupportAWSRoles, dbUser: "unknown-user", access: false},
			},
		},
		{
			name:  "(case-insensitive db) developer allowed any username in stage except superuser",
			roles: RoleSet{roleDevStage, roleDevProd},
			access: []access{
				{server: dbCockroachStage, dbUser: "dev", access: true},
				{server: dbCockroachStage, dbUser: "DEV", access: true},
				{server: dbCockroachStage, dbUser: "test", access: true},
				{server: dbCockroachStage, dbUser: "superuser", access: false},
				{server: dbCockroachStage, dbUser: "SUPERUSER", access: false},
			},
		},
		{
			name:  "(case-insensitive db) developer allowed only specific username/database in prod database",
			roles: RoleSet{roleDevStage, roleDevProd},
			access: []access{
				{server: dbCockroachProd, dbUser: "dev", access: true},
				{server: dbCockroachProd, dbUser: "DEV", access: true},
				{server: dbCockroachProd, dbUser: "superuser", access: false},
				{server: dbCockroachProd, dbUser: "Superuser", access: false},
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			for _, access := range tc.access {
				err := tc.roles.checkAccess(access.server, wrappers.Traits{}, AccessState{}, NewDatabaseUserMatcher(access.server, access.dbUser))
				if access.access {
					require.NoError(t, err, "access check shouldn't have failed for username %q", access.dbUser)
				} else {
					require.Error(t, err, "access check should have failed for username %q", access.dbUser)
					require.True(t, trace.IsAccessDenied(err))
				}
			}
		})
	}
}

func TestRoleSetEnumerateDatabaseUsersAndNames(t *testing.T) {
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
	dbAutoUser, err := types.NewDatabaseV3(types.Metadata{
		Name:   "auto-user",
		Labels: map[string]string{"env": "prod"},
	}, types.DatabaseSpecV3{
		Protocol: "postgres",
		URI:      "localhost:5432",
		AdminUser: &types.DatabaseAdminUser{
			Name: "teleport-admin",
		},
	})
	require.NoError(t, err)
	roleDevStage := &types.RoleV6{
		Metadata: types.Metadata{Name: "dev-stage", Namespace: apidefaults.Namespace},
		Spec: types.RoleSpecV6{
			Allow: types.RoleConditions{
				Namespaces:     []string{apidefaults.Namespace},
				DatabaseLabels: types.Labels{"env": []string{"stage"}},
				DatabaseUsers:  []string{types.Wildcard},
				DatabaseNames:  []string{types.Wildcard},
			},
			Deny: types.RoleConditions{
				Namespaces:    []string{apidefaults.Namespace},
				DatabaseUsers: []string{"root"},
				DatabaseNames: []string{"root"},
			},
		},
	}
	roleDevProd := &types.RoleV6{
		Metadata: types.Metadata{Name: "dev-prod", Namespace: apidefaults.Namespace},
		Spec: types.RoleSpecV6{
			Allow: types.RoleConditions{
				Namespaces:     []string{apidefaults.Namespace},
				DatabaseLabels: types.Labels{"env": []string{"prod"}},
				DatabaseUsers:  []string{"dev"},
				DatabaseNames:  []string{"dev"},
			},
		},
	}

	roleNoDBAccess := &types.RoleV6{
		Metadata: types.Metadata{Name: "no_db_access", Namespace: apidefaults.Namespace},
		Spec: types.RoleSpecV6{
			Deny: types.RoleConditions{
				Namespaces:    []string{apidefaults.Namespace},
				DatabaseUsers: []string{"*"},
				DatabaseNames: []string{"*"},
			},
		},
	}

	roleAllowDenySame := &types.RoleV6{
		Metadata: types.Metadata{Name: "allow_deny_same", Namespace: apidefaults.Namespace},
		Spec: types.RoleSpecV6{
			Allow: types.RoleConditions{
				Namespaces:    []string{apidefaults.Namespace},
				DatabaseUsers: []string{"root"},
				DatabaseNames: []string{"root"},
			},
			Deny: types.RoleConditions{
				Namespaces:    []string{apidefaults.Namespace},
				DatabaseUsers: []string{"root"},
				DatabaseNames: []string{"root"},
			},
		},
	}

	roleAutoUser := &types.RoleV6{
		Metadata: types.Metadata{Name: "auto-user", Namespace: apidefaults.Namespace},
		Spec: types.RoleSpecV6{
			Options: types.RoleOptions{
				CreateDatabaseUserMode: types.CreateDatabaseUserMode_DB_USER_MODE_KEEP,
			},
			Allow: types.RoleConditions{
				Namespaces:     []string{apidefaults.Namespace},
				DatabaseLabels: types.Labels{"env": []string{"prod"}},
				DatabaseRoles:  []string{"dev"},
				DatabaseNames:  []string{"*"},
				DatabaseUsers:  []string{types.Wildcard},
			},
		},
	}

	testCases := []struct {
		name             string
		roles            RoleSet
		server           types.Database
		enumDBUserResult EnumerationResult
		enumDBNameResult EnumerationResult
	}{
		{
			name:   "deny overrides allow",
			roles:  RoleSet{roleAllowDenySame},
			server: dbStage,
			enumDBUserResult: EnumerationResult{
				allowedDeniedMap: map[string]bool{"root": false},
				wildcardAllowed:  false,
				wildcardDenied:   false,
			},
			enumDBNameResult: EnumerationResult{
				allowedDeniedMap: map[string]bool{"root": false},
				wildcardAllowed:  false,
				wildcardDenied:   false,
			},
		},
		{
			name:   "developer allowed any username in stage database except root",
			roles:  RoleSet{roleDevStage, roleDevProd},
			server: dbStage,
			enumDBUserResult: EnumerationResult{
				allowedDeniedMap: map[string]bool{"dev": true, "root": false},
				wildcardAllowed:  true,
				wildcardDenied:   false,
			},
			enumDBNameResult: EnumerationResult{
				allowedDeniedMap: map[string]bool{"dev": true, "root": false},
				wildcardAllowed:  true,
				wildcardDenied:   false,
			},
		},
		{
			name:   "developer allowed only specific username/database in prod database",
			roles:  RoleSet{roleDevStage, roleDevProd},
			server: dbProd,
			enumDBUserResult: EnumerationResult{
				allowedDeniedMap: map[string]bool{"dev": true, "root": false},
				wildcardAllowed:  false,
				wildcardDenied:   false,
			},
			enumDBNameResult: EnumerationResult{
				allowedDeniedMap: map[string]bool{"dev": true, "root": false},
				wildcardAllowed:  false,
				wildcardDenied:   false,
			},
		},
		{
			name:   "there may be users disallowed from all users",
			roles:  RoleSet{roleDevStage, roleDevProd, roleNoDBAccess},
			server: dbProd,
			enumDBUserResult: EnumerationResult{
				allowedDeniedMap: map[string]bool{"dev": false, "root": false},
				wildcardAllowed:  false,
				wildcardDenied:   true,
			},
			enumDBNameResult: EnumerationResult{
				allowedDeniedMap: map[string]bool{"dev": false, "root": false},
				wildcardAllowed:  false,
				wildcardDenied:   true,
			},
		},
		{
			name:   "auto-user provisioning enabled",
			roles:  RoleSet{roleAutoUser},
			server: dbAutoUser,
			enumDBUserResult: EnumerationResult{
				allowedDeniedMap: map[string]bool{"alice": true},
				wildcardAllowed:  false,
				wildcardDenied:   false,
			},
			enumDBNameResult: EnumerationResult{
				allowedDeniedMap: map[string]bool{},
				wildcardAllowed:  true,
				wildcardDenied:   false,
			},
		},
	}
	for _, tc := range testCases {
		accessChecker := makeAccessCheckerWithRoleSet(tc.roles)
		t.Run(tc.name, func(t *testing.T) {
			enumResult, err := accessChecker.EnumerateDatabaseUsers(tc.server)
			require.NoError(t, err)
			require.Equal(t, tc.enumDBUserResult, enumResult)
			enumResult = accessChecker.EnumerateDatabaseNames(tc.server)
			require.Equal(t, tc.enumDBNameResult, enumResult)
		})
	}
}

func TestGetAllowedLoginsForResource(t *testing.T) {
	newRole := func(
		allowLogins []string,
		allowLabels types.Labels,
		denyLogins []string,
		denyLabels types.Labels,
	) *types.RoleV6 {
		return &types.RoleV6{
			Spec: types.RoleSpecV6{
				Allow: types.RoleConditions{
					Namespaces:           []string{apidefaults.Namespace},
					Logins:               allowLogins,
					WindowsDesktopLogins: allowLogins,
					AWSRoleARNs:          allowLogins,
					NodeLabels:           allowLabels,
					WindowsDesktopLabels: allowLabels,
					AppLabels:            allowLabels,
				},
				Deny: types.RoleConditions{
					Namespaces:           []string{apidefaults.Namespace},
					Logins:               denyLogins,
					WindowsDesktopLogins: denyLogins,
					AWSRoleARNs:          denyLogins,
					NodeLabels:           denyLabels,
					WindowsDesktopLabels: denyLabels,
					AppLabels:            denyLabels,
				},
			},
		}
	}

	devEnvRole := newRole([]string{"devuser"}, types.Labels{"env": []string{"dev"}}, []string{}, types.Labels{})
	prodEnvRole := newRole([]string{"produser"}, types.Labels{"env": []string{"prod"}}, []string{}, types.Labels{})
	anyEnvRole := newRole([]string{"anyenvrole"}, types.Labels{"env": []string{"*"}}, []string{}, types.Labels{})
	rootUser := newRole([]string{"root"}, types.Labels{"*": []string{"*"}}, []string{}, types.Labels{})
	roleWithMultipleLabels := newRole([]string{"multiplelabelsuser"},
		types.Labels{
			"region": []string{"*"},
			"env":    []string{"dev"},
		}, []string{}, types.Labels{})

	tt := []struct {
		name           string
		labels         map[string]string
		roleSet        RoleSet
		expectedLogins []string
	}{
		{
			name: "env dev login is added",
			labels: map[string]string{
				"env": "dev",
			},
			roleSet:        NewRoleSet(devEnvRole),
			expectedLogins: []string{"devuser"},
		},
		{
			name: "env prod login is added",
			labels: map[string]string{
				"env": "prod",
			},
			roleSet:        NewRoleSet(prodEnvRole),
			expectedLogins: []string{"produser"},
		},
		{
			name: "only the correct login is added",
			labels: map[string]string{
				"env": "prod",
			},
			roleSet:        NewRoleSet(prodEnvRole, devEnvRole),
			expectedLogins: []string{"produser"},
		},
		{
			name: "logins from role not authorizeds are not added",
			labels: map[string]string{
				"env": "staging",
			},
			roleSet:        NewRoleSet(devEnvRole, prodEnvRole),
			expectedLogins: nil,
		},
		{
			name: "role with wildcard get its logins",
			labels: map[string]string{
				"env": "prod",
			},
			roleSet:        NewRoleSet(anyEnvRole),
			expectedLogins: []string{"anyenvrole"},
		},
		{
			name: "can return multiple logins",
			labels: map[string]string{
				"env": "prod",
			},
			roleSet:        NewRoleSet(anyEnvRole, prodEnvRole),
			expectedLogins: []string{"anyenvrole", "produser"},
		},
		{
			name: "can return multiple logins from same role",
			labels: map[string]string{
				"env": "prod",
			},
			roleSet: NewRoleSet(
				newRole(
					[]string{"role1", "role2", "role3"},
					types.Labels{"env": []string{"*"}}, []string{}, types.Labels{}),
			),
			expectedLogins: []string{"role1", "role2", "role3"},
		},
		{
			name: "works with user with full access",
			labels: map[string]string{
				"env": "prod",
			},
			roleSet:        NewRoleSet(rootUser),
			expectedLogins: []string{"root"},
		},
		{
			name: "works with server with multiple labels",
			labels: map[string]string{
				"env":    "prod",
				"region": "us-east-1",
			},
			roleSet:        NewRoleSet(prodEnvRole),
			expectedLogins: []string{"produser"},
		},
		{
			name: "don't add login from unrelated labels",
			labels: map[string]string{
				"env": "dev",
			},
			roleSet: NewRoleSet(
				newRole(
					[]string{"anyregionuser"},
					types.Labels{"region": []string{"*"}}, []string{}, types.Labels{}),
			),
			expectedLogins: nil,
		},
		{
			name: "works with roles with multiple labels that role shouldn't access",
			labels: map[string]string{
				"env": "dev",
			},
			roleSet:        NewRoleSet(roleWithMultipleLabels),
			expectedLogins: nil,
		},
		{
			name: "works with roles with multiple labels that role shouldn't access",
			labels: map[string]string{
				"env":    "dev",
				"region": "us-west-1",
			},
			roleSet:        NewRoleSet(roleWithMultipleLabels),
			expectedLogins: []string{"multiplelabelsuser"},
		},
		{
			name: "works with roles with regular expressions",
			labels: map[string]string{
				"region": "us-west-1",
			},
			roleSet: NewRoleSet(
				newRole(
					[]string{"rolewithregexpuser"},
					types.Labels{"region": []string{"^us-west-1|eu-central-1$"}}, []string{}, types.Labels{}),
			),
			expectedLogins: []string{"rolewithregexpuser"},
		},
		{
			name: "works with denied roles",
			labels: map[string]string{
				"env": "dev",
			},
			roleSet: NewRoleSet(
				newRole(
					[]string{}, types.Labels{}, []string{"devuser"}, types.Labels{"env": []string{"*"}}),
			),
			expectedLogins: nil,
		},
		{
			name: "works with denied roles of unrelated labels",
			labels: map[string]string{
				"env": "dev",
			},
			roleSet: NewRoleSet(
				newRole(
					[]string{}, types.Labels{}, []string{"devuser"}, types.Labels{"region": []string{"*"}}),
			),
			expectedLogins: nil,
		},
	}
	for _, tc := range tt {
		accessChecker := makeAccessCheckerWithRoleSet(tc.roleSet)
		t.Run(tc.name, func(t *testing.T) {
			server := mustMakeTestServer(tc.labels)
			desktop := mustMakeTestWindowsDesktop(tc.labels)
			app := mustMakeTestAWSApp(tc.labels)

			serverLogins, err := accessChecker.GetAllowedLoginsForResource(server)
			require.NoError(t, err)
			desktopLogins, err := accessChecker.GetAllowedLoginsForResource(desktop)
			require.NoError(t, err)
			awsARNLogins, err := accessChecker.GetAllowedLoginsForResource(app)
			require.NoError(t, err)

			require.ElementsMatch(t, tc.expectedLogins, serverLogins)
			require.ElementsMatch(t, tc.expectedLogins, desktopLogins)
			require.ElementsMatch(t, tc.expectedLogins, awsARNLogins)
		})
	}
}

// mustMakeTestServer creates a server with labels and an empty spec.
// It panics in case of an error. Used only for testing
func mustMakeTestServer(labels map[string]string) types.Server {
	s, err := types.NewServerWithLabels("server", types.KindNode, types.ServerSpecV2{}, labels)
	if err != nil {
		panic(err)
	}
	return s
}

func mustMakeTestWindowsDesktop(labels map[string]string) types.WindowsDesktop {
	d, err := types.NewWindowsDesktopV3("desktop", labels, types.WindowsDesktopSpecV3{Addr: "addr"})
	if err != nil {
		panic(err)
	}
	return d
}

func mustMakeTestAWSApp(labels map[string]string) types.Application {
	app, err := types.NewAppV3(types.Metadata{
		Name:   "my-app",
		Labels: labels,
	},
		types.AppSpecV3{
			URI:   "https://some-addr.com",
			Cloud: "AWS",
		},
	)
	if err != nil {
		panic(err)
	}
	return app
}

func TestCheckDatabaseRoles(t *testing.T) {
	// roleA just allows access to all databases without auto-provisioning.
	roleA := &types.RoleV6{
		Metadata: types.Metadata{Name: "roleA", Namespace: apidefaults.Namespace},
		Spec: types.RoleSpecV6{
			Allow: types.RoleConditions{
				DatabaseLabels: types.Labels{types.Wildcard: []string{types.Wildcard}},
			},
		},
	}

	// roleB allows auto-user provisioning for production database and uses
	// label expressions.
	roleB := &types.RoleV6{
		Metadata: types.Metadata{Name: "roleB", Namespace: apidefaults.Namespace},
		Spec: types.RoleSpecV6{
			Options: types.RoleOptions{
				CreateDatabaseUserMode: types.CreateDatabaseUserMode_DB_USER_MODE_KEEP,
			},
			Allow: types.RoleConditions{
				DatabaseLabelsExpression: `labels["env"] == "prod"`,
				DatabaseRoles:            []string{"reader"},
			},
			Deny: types.RoleConditions{
				DatabaseLabelsExpression: `labels["env"] == "prod"`,
				DatabaseRoles:            []string{"writer"},
			},
		},
	}

	// roleC allows auto-user provisioning for metrics database and uses label
	// expressions.
	roleC := &types.RoleV6{
		Metadata: types.Metadata{Name: "roleC", Namespace: apidefaults.Namespace},
		Spec: types.RoleSpecV6{
			Options: types.RoleOptions{
				CreateDatabaseUserMode: types.CreateDatabaseUserMode_DB_USER_MODE_KEEP,
			},
			Allow: types.RoleConditions{
				DatabaseLabels: types.Labels{"app": []string{"metrics"}},
				DatabaseRoles:  []string{"reader", "writer"},
			},
		},
	}

	// roleD has a bad label expression.
	roleD := &types.RoleV6{
		Metadata: types.Metadata{Name: "roleD", Namespace: apidefaults.Namespace},
		Spec: types.RoleSpecV6{
			Options: types.RoleOptions{
				CreateDatabaseUserMode: types.CreateDatabaseUserMode_DB_USER_MODE_KEEP,
			},
			Allow: types.RoleConditions{
				DatabaseLabelsExpression: `a bad expression`,
				DatabaseRoles:            []string{"reader"},
			},
		},
	}

	// roleE is like roleB, allows auto-user provisioning for production database,
	// but uses database permissions instead of roles.
	roleE := &types.RoleV6{
		Metadata: types.Metadata{Name: "roleB", Namespace: apidefaults.Namespace},
		Spec: types.RoleSpecV6{
			Options: types.RoleOptions{
				CreateDatabaseUserMode: types.CreateDatabaseUserMode_DB_USER_MODE_KEEP,
			},
			Allow: types.RoleConditions{
				DatabaseLabelsExpression: `labels["env"] == "prod"`,
				DatabasePermissions: []types.DatabasePermission{
					{
						Permissions: []string{"SELECT"},
						Match:       map[string]apiutils.Strings{"*": []string{"*"}},
					},
				},
			},
			Deny: types.RoleConditions{
				DatabaseLabelsExpression: `labels["env"] == "prod"`,
				DatabasePermissions: []types.DatabasePermission{
					{
						Permissions: []string{"UPDATE", "INSERT", "DELETE"},
						Match:       map[string]apiutils.Strings{"*": []string{"*"}},
					},
				},
			},
		},
	}

	// roleF is like roleC, allows auto-user provisioning for metrics database,
	// but uses database permissions instead of roles.
	roleF := &types.RoleV6{
		Metadata: types.Metadata{Name: "roleC", Namespace: apidefaults.Namespace},
		Spec: types.RoleSpecV6{
			Options: types.RoleOptions{
				CreateDatabaseUserMode: types.CreateDatabaseUserMode_DB_USER_MODE_KEEP,
			},
			Allow: types.RoleConditions{
				DatabaseLabels: types.Labels{"app": []string{"metrics"}},
				DatabasePermissions: []types.DatabasePermission{
					{
						Permissions: []string{"SELECT", "UPDATE", "INSERT", "DELETE"},
						Match:       map[string]apiutils.Strings{"*": []string{"*"}},
					},
				},
			},
		},
	}

	tests := []struct {
		name                string
		roleSet             RoleSet
		inDatabaseLabels    map[string]string
		inRequestedRoles    []string
		outModeError        bool
		outRolesError       bool
		outCreateUser       bool
		outRoles            []string
		outPermissionsError bool
		outAllowPermissions types.DatabasePermissions
		outDenyPermissions  types.DatabasePermissions
	}{
		{
			name:             "no auto-provision roles assigned",
			roleSet:          RoleSet{roleA},
			inDatabaseLabels: map[string]string{"app": "metrics"},
			outCreateUser:    false,
			outRoles:         []string{},
		},
		{
			name:             "database doesn't match",
			roleSet:          RoleSet{roleB},
			inDatabaseLabels: map[string]string{"env": "test"},
			outCreateUser:    false,
			outRoles:         []string{},
		},
		{
			name:             "connect to test database, no auto-provisioning",
			roleSet:          RoleSet{roleA, roleB, roleC},
			inDatabaseLabels: map[string]string{"env": "test"},
			outCreateUser:    false,
			outRoles:         []string{},
		},
		{
			name:             "connect to metrics database, get reader/writer role",
			roleSet:          RoleSet{roleA, roleB, roleC},
			inDatabaseLabels: map[string]string{"app": "metrics"},
			outCreateUser:    true,
			outRoles:         []string{"reader", "writer"},
		},
		{
			name:             "connect to metrics database, get reader/writer permissions",
			roleSet:          RoleSet{roleA, roleE, roleF},
			inDatabaseLabels: map[string]string{"app": "metrics"},
			outCreateUser:    true,
			outRoles:         []string{},
			outAllowPermissions: types.DatabasePermissions{
				types.DatabasePermission{Permissions: []string{"SELECT", "UPDATE", "INSERT", "DELETE"}, Match: types.Labels{"*": apiutils.Strings{"*"}}},
			},
		},
		{
			name:             "connect to prod database, get reader role",
			roleSet:          RoleSet{roleA, roleB, roleC},
			inDatabaseLabels: map[string]string{"app": "metrics", "env": "prod"},
			outCreateUser:    true,
			outRoles:         []string{"reader"},
		},
		{
			name:             "connect to prod database, get reader permissions",
			roleSet:          RoleSet{roleA, roleE, roleF},
			inDatabaseLabels: map[string]string{"app": "metrics", "env": "prod"},
			outCreateUser:    true,
			outRoles:         []string{},
			// the overlap between outAllowPermissions and outDenyPermissions is expected.
			// the permission arithmetic (e.g. removing denied permissions) will be done ba a downstream function.
			outAllowPermissions: types.DatabasePermissions{
				types.DatabasePermission{Permissions: []string{"SELECT"}, Match: types.Labels{"*": apiutils.Strings{"*"}}},
				types.DatabasePermission{Permissions: []string{"SELECT", "UPDATE", "INSERT", "DELETE"}, Match: types.Labels{"*": apiutils.Strings{"*"}}},
			},
			outDenyPermissions: types.DatabasePermissions{
				types.DatabasePermission{Permissions: []string{"UPDATE", "INSERT", "DELETE"}, Match: types.Labels{"*": apiutils.Strings{"*"}}},
			},
		},
		{
			name:             "connect to metrics database, requested writer role",
			roleSet:          RoleSet{roleA, roleB, roleC},
			inDatabaseLabels: map[string]string{"app": "metrics"},
			inRequestedRoles: []string{"writer"},
			outCreateUser:    true,
			outRoles:         []string{"writer"},
		},
		{
			name:             "requested role denied",
			roleSet:          RoleSet{roleA, roleB, roleC},
			inDatabaseLabels: map[string]string{"app": "metrics", "env": "prod"},
			inRequestedRoles: []string{"writer"},
			outCreateUser:    true,
			outRolesError:    true,
		},
		{
			name:                "check fails",
			roleSet:             RoleSet{roleD},
			inDatabaseLabels:    map[string]string{"app": "metrics"},
			outModeError:        true,
			outRolesError:       true,
			outPermissionsError: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			accessChecker := makeAccessCheckerWithRoleSet(test.roleSet)
			database, err := types.NewDatabaseV3(types.Metadata{
				Name:   "test",
				Labels: test.inDatabaseLabels,
			}, types.DatabaseSpecV3{
				Protocol: "protocol",
				URI:      "uri",
			})
			require.NoError(t, err)

			create, err := accessChecker.DatabaseAutoUserMode(database)
			if test.outModeError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, test.outCreateUser, create.IsEnabled())
			}

			roles, err := accessChecker.CheckDatabaseRoles(database, test.inRequestedRoles)
			if test.outRolesError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, test.outRoles, roles)
			}

			allow, deny, err := accessChecker.GetDatabasePermissions(database)
			if test.outPermissionsError {
				require.Error(t, err)
				require.Empty(t, allow)
				require.Empty(t, deny)
			} else {
				require.NoError(t, err)
				require.Equal(t, test.outAllowPermissions, allow)
				require.Equal(t, test.outDenyPermissions, deny)
			}
		})
	}
}

func TestGetCreateDatabaseCreateMode(t *testing.T) {
	for name, tc := range map[string]struct {
		roleSet      RoleSet
		expectedMode types.CreateDatabaseUserMode
	}{
		"disabled": {
			roleSet: RoleSet{
				&types.RoleV6{
					Spec: types.RoleSpecV6{
						Options: types.RoleOptions{
							CreateDatabaseUserMode: types.CreateDatabaseUserMode_DB_USER_MODE_OFF,
						},
					},
				},
			},
			expectedMode: types.CreateDatabaseUserMode_DB_USER_MODE_OFF,
		},
		"enabled mode take precedence": {
			roleSet: RoleSet{
				&types.RoleV6{
					Spec: types.RoleSpecV6{
						Options: types.RoleOptions{
							CreateDatabaseUserMode: types.CreateDatabaseUserMode_DB_USER_MODE_OFF,
						},
					},
				},
				&types.RoleV6{
					Spec: types.RoleSpecV6{
						Options: types.RoleOptions{
							CreateDatabaseUserMode: types.CreateDatabaseUserMode_DB_USER_MODE_KEEP,
						},
					},
				},
			},
			expectedMode: types.CreateDatabaseUserMode_DB_USER_MODE_KEEP,
		},
		"delete mode take precedence": {
			roleSet: RoleSet{
				&types.RoleV6{
					Spec: types.RoleSpecV6{
						Options: types.RoleOptions{
							CreateDatabaseUserMode: types.CreateDatabaseUserMode_DB_USER_MODE_BEST_EFFORT_DROP,
						},
					},
				},
				&types.RoleV6{
					Spec: types.RoleSpecV6{
						Options: types.RoleOptions{
							CreateDatabaseUserMode: types.CreateDatabaseUserMode_DB_USER_MODE_OFF,
						},
					},
				},
				&types.RoleV6{
					Spec: types.RoleSpecV6{
						Options: types.RoleOptions{
							CreateDatabaseUserMode: types.CreateDatabaseUserMode_DB_USER_MODE_KEEP,
						},
					},
				},
			},
			expectedMode: types.CreateDatabaseUserMode_DB_USER_MODE_BEST_EFFORT_DROP,
		},
	} {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, tc.expectedMode, tc.roleSet.GetCreateDatabaseUserMode())
		})
	}
}

// TestEncodeCreateDatabaseUserMode guarantees all modes are implemented in the
// encoder/decoder.
func TestEncodeDecodeCreateDatabaseUserMode(t *testing.T) {
	for name, rawMode := range types.CreateDatabaseUserMode_value {
		t.Run(name, func(t *testing.T) {
			mode := types.CreateDatabaseUserMode(rawMode)

			t.Run("YAML", func(t *testing.T) {
				encoded, err := yaml.Marshal(&mode)
				require.NoError(t, err)

				var decodedMode types.CreateDatabaseUserMode
				require.NoError(t, yaml.Unmarshal(encoded, &decodedMode))
				require.Equal(t, mode, decodedMode)
			})

			t.Run("JSON", func(t *testing.T) {
				encoded, err := mode.MarshalJSON()
				require.NoError(t, err)

				var decodedMode types.CreateDatabaseUserMode
				require.NoError(t, decodedMode.UnmarshalJSON(encoded))
				require.Equal(t, mode, decodedMode)
			})
		})
	}
}

func TestCheckDatabaseNamesAndUsers(t *testing.T) {
	roleEmpty := &types.RoleV6{
		Metadata: types.Metadata{Name: "roleA", Namespace: apidefaults.Namespace},
		Spec: types.RoleSpecV6{
			Options: types.RoleOptions{MaxSessionTTL: types.Duration(time.Hour)},
			Allow: types.RoleConditions{
				Namespaces: []string{apidefaults.Namespace},
			},
		},
	}
	roleA := &types.RoleV6{
		Metadata: types.Metadata{Name: "roleA", Namespace: apidefaults.Namespace},
		Spec: types.RoleSpecV6{
			Options: types.RoleOptions{MaxSessionTTL: types.Duration(2 * time.Hour)},
			Allow: types.RoleConditions{
				Namespaces:    []string{apidefaults.Namespace},
				DatabaseNames: []string{"postgres", "main"},
				DatabaseUsers: []string{"postgres", "alice"},
			},
		},
	}
	roleB := &types.RoleV6{
		Metadata: types.Metadata{Name: "roleB", Namespace: apidefaults.Namespace},
		Spec: types.RoleSpecV6{
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
	roleAdmin := &types.RoleV6{
		Metadata: types.Metadata{Name: "admin", Namespace: apidefaults.Namespace},
		Spec: types.RoleSpecV6{
			Allow: types.RoleConditions{
				Namespaces:     []string{apidefaults.Namespace},
				DatabaseLabels: types.Labels{types.Wildcard: []string{types.Wildcard}},
			},
		},
	}
	roleDev := &types.RoleV6{
		Metadata: types.Metadata{Name: "dev", Namespace: apidefaults.Namespace},
		Spec: types.RoleSpecV6{
			Allow: types.RoleConditions{
				Namespaces:               []string{apidefaults.Namespace},
				DatabaseLabelsExpression: `contains(user.spec.traits["allow-env"], labels["env"])`,
			},
			Deny: types.RoleConditions{
				Namespaces:     []string{apidefaults.Namespace},
				DatabaseLabels: types.Labels{"arch": []string{"amd64"}},
			},
		},
	}
	roleIntern := &types.RoleV6{
		Metadata: types.Metadata{Name: "intern", Namespace: apidefaults.Namespace},
		Spec: types.RoleSpecV6{
			Allow: types.RoleConditions{
				Namespaces: []string{apidefaults.Namespace},
			},
		},
	}
	userTraits := wrappers.Traits{
		"allow-env": {"stage"},
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
				err := tc.roles.checkAccess(access.server, userTraits, AccessState{})
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
	roleNoAccess := &types.RoleV6{
		Metadata: types.Metadata{
			Name:      "noaccess",
			Namespace: apidefaults.Namespace,
		},
		Spec: types.RoleSpecV6{
			Allow: types.RoleConditions{
				Namespaces:  []string{apidefaults.Namespace},
				AppLabels:   types.Labels{types.Wildcard: []string{types.Wildcard}},
				AWSRoleARNs: []string{},
			},
		},
	}
	roleReadOnly := &types.RoleV6{
		Metadata: types.Metadata{
			Name:      "readonly",
			Namespace: apidefaults.Namespace,
		},
		Spec: types.RoleSpecV6{
			Allow: types.RoleConditions{
				Namespaces:          []string{apidefaults.Namespace},
				AppLabelsExpression: matchAllExpression,
				AWSRoleARNs:         []string{readOnlyARN},
			},
		},
	}
	roleFullAccess := &types.RoleV6{
		Metadata: types.Metadata{
			Name:      "fullaccess",
			Namespace: apidefaults.Namespace,
		},
		Spec: types.RoleSpecV6{
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
				err := test.roles.checkAccess(
					app,
					wrappers.Traits{},
					AccessState{},
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

// TestCheckAccessToAzureCloud verifies Azure identities access checker.
func TestCheckAccessToAzureCloud(t *testing.T) {
	app, err := types.NewAppV3(types.Metadata{Name: "azureapp"}, types.AppSpecV3{Cloud: types.CloudAzure})
	require.NoError(t, err)
	readOnlyIdentity := "readonly"
	fullAccessIdentity := "fullaccess"
	roleNoAccess := &types.RoleV6{
		Metadata: types.Metadata{
			Name:      "noaccess",
			Namespace: apidefaults.Namespace,
		},
		Spec: types.RoleSpecV6{
			Allow: types.RoleConditions{
				Namespaces:      []string{apidefaults.Namespace},
				AppLabels:       types.Labels{types.Wildcard: []string{types.Wildcard}},
				AzureIdentities: []string{},
			},
		},
	}
	roleReadOnly := &types.RoleV6{
		Metadata: types.Metadata{
			Name:      "readonly",
			Namespace: apidefaults.Namespace,
		},
		Spec: types.RoleSpecV6{
			Allow: types.RoleConditions{
				Namespaces:          []string{apidefaults.Namespace},
				AppLabelsExpression: matchAllExpression,
				AzureIdentities:     []string{readOnlyIdentity},
			},
		},
	}
	roleFullAccess := &types.RoleV6{
		Metadata: types.Metadata{
			Name:      "fullaccess",
			Namespace: apidefaults.Namespace,
		},
		Spec: types.RoleSpecV6{
			Allow: types.RoleConditions{
				Namespaces:      []string{apidefaults.Namespace},
				AppLabels:       types.Labels{types.Wildcard: []string{types.Wildcard}},
				AzureIdentities: []string{readOnlyIdentity, fullAccessIdentity},
			},
		},
	}
	tests := []struct {
		name   string
		roles  RoleSet
		access map[string]bool
	}{
		{
			name:  "empty role set",
			roles: nil,
			access: map[string]bool{
				readOnlyIdentity:   false,
				fullAccessIdentity: false,
			},
		},
		{
			name:  "no access role",
			roles: RoleSet{roleNoAccess},
			access: map[string]bool{
				readOnlyIdentity:   false,
				fullAccessIdentity: false,
			},
		},
		{
			name:  "readonly role",
			roles: RoleSet{roleReadOnly},
			access: map[string]bool{
				readOnlyIdentity:   true,
				fullAccessIdentity: false,
			},
		},
		{
			name:  "full access role",
			roles: RoleSet{roleFullAccess},
			access: map[string]bool{
				readOnlyIdentity:   true,
				fullAccessIdentity: true,
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			for identity, hasAccess := range test.access {
				err := test.roles.checkAccess(app, wrappers.Traits{}, AccessState{}, &AzureIdentityMatcher{Identity: identity})
				if hasAccess {
					require.NoError(t, err)
				} else {
					require.Error(t, err)
					require.True(t, trace.IsAccessDenied(err))
				}
			}
		})
	}
}

// TestCheckAccessToGCP verifies GCP account access checker.
func TestCheckAccessToGCP(t *testing.T) {
	app, err := types.NewAppV3(types.Metadata{Name: "azureapp"}, types.AppSpecV3{Cloud: types.CloudAzure})
	require.NoError(t, err)
	readOnlyAccount := "readonly@example-123456.iam.gserviceaccount.com"
	fullAccessAccount := "fullaccess@example-123456.iam.gserviceaccount.com"
	roleNoAccess := &types.RoleV6{
		Metadata: types.Metadata{
			Name:      "noaccess",
			Namespace: apidefaults.Namespace,
		},
		Spec: types.RoleSpecV6{
			Allow: types.RoleConditions{
				Namespaces:         []string{apidefaults.Namespace},
				AppLabels:          types.Labels{types.Wildcard: []string{types.Wildcard}},
				GCPServiceAccounts: []string{},
			},
		},
	}
	roleReadOnly := &types.RoleV6{
		Metadata: types.Metadata{
			Name:      "readonly",
			Namespace: apidefaults.Namespace,
		},
		Spec: types.RoleSpecV6{
			Allow: types.RoleConditions{
				Namespaces:         []string{apidefaults.Namespace},
				AppLabels:          types.Labels{types.Wildcard: []string{types.Wildcard}},
				GCPServiceAccounts: []string{readOnlyAccount},
			},
		},
	}
	roleFullAccess := &types.RoleV6{
		Metadata: types.Metadata{
			Name:      "fullaccess",
			Namespace: apidefaults.Namespace,
		},
		Spec: types.RoleSpecV6{
			Allow: types.RoleConditions{
				Namespaces:          []string{apidefaults.Namespace},
				AppLabelsExpression: matchAllExpression,
				GCPServiceAccounts:  []string{readOnlyAccount, fullAccessAccount},
			},
		},
	}
	tests := []struct {
		name   string
		roles  RoleSet
		access map[string]bool
	}{
		{
			name:  "empty role set",
			roles: nil,
			access: map[string]bool{
				readOnlyAccount:   false,
				fullAccessAccount: false,
			},
		},
		{
			name:  "no access role",
			roles: RoleSet{roleNoAccess},
			access: map[string]bool{
				readOnlyAccount:   false,
				fullAccessAccount: false,
			},
		},
		{
			name:  "readonly role",
			roles: RoleSet{roleReadOnly},
			access: map[string]bool{
				readOnlyAccount:   true,
				fullAccessAccount: false,
			},
		},
		{
			name:  "full access role",
			roles: RoleSet{roleFullAccess},
			access: map[string]bool{
				readOnlyAccount:   true,
				fullAccessAccount: true,
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			for account, hasAccess := range test.access {
				err := test.roles.checkAccess(app, wrappers.Traits{}, AccessState{}, &GCPServiceAccountMatcher{ServiceAccount: account})
				if hasAccess {
					require.NoError(t, err)
				} else {
					require.Error(t, err)
					require.True(t, trace.IsAccessDenied(err))
				}

			}
		})
	}
}

func TestCheckAzureIdentities(t *testing.T) {
	readOnlyIdentity := "readonly"
	fullAccessIdentity := "fullaccess"

	maxSessionTTL := time.Hour * 2
	sessionShort := time.Hour * 1
	sessionLong := time.Hour * 3

	roleNoAccess := &types.RoleV6{
		Metadata: types.Metadata{
			Name:      "noaccess",
			Namespace: apidefaults.Namespace,
		},
		Spec: types.RoleSpecV6{
			Options: types.RoleOptions{
				MaxSessionTTL: types.NewDuration(maxSessionTTL),
			},
			Allow: types.RoleConditions{
				Namespaces:      []string{apidefaults.Namespace},
				AppLabels:       types.Labels{types.Wildcard: []string{types.Wildcard}},
				AzureIdentities: []string{},
			},
		},
	}
	roleReadOnly := &types.RoleV6{
		Metadata: types.Metadata{
			Name:      "readonly",
			Namespace: apidefaults.Namespace,
		},
		Spec: types.RoleSpecV6{
			Options: types.RoleOptions{
				MaxSessionTTL: types.NewDuration(maxSessionTTL),
			},
			Allow: types.RoleConditions{
				Namespaces:      []string{apidefaults.Namespace},
				AppLabels:       types.Labels{types.Wildcard: []string{types.Wildcard}},
				AzureIdentities: []string{readOnlyIdentity},
			},
		},
	}
	roleFullAccess := &types.RoleV6{
		Metadata: types.Metadata{
			Name:      "fullaccess",
			Namespace: apidefaults.Namespace,
		},
		Spec: types.RoleSpecV6{
			Options: types.RoleOptions{
				MaxSessionTTL: types.NewDuration(maxSessionTTL),
			},
			Allow: types.RoleConditions{
				Namespaces:      []string{apidefaults.Namespace},
				AppLabels:       types.Labels{types.Wildcard: []string{types.Wildcard}},
				AzureIdentities: []string{readOnlyIdentity, fullAccessIdentity},
			},
		},
	}

	roleDenyReadOnlyIdentity := &types.RoleV6{
		Metadata: types.Metadata{
			Name:      "deny-identity",
			Namespace: apidefaults.Namespace,
		},
		Spec: types.RoleSpecV6{
			Options: types.RoleOptions{
				MaxSessionTTL: types.NewDuration(maxSessionTTL),
			},
			Deny: types.RoleConditions{
				Namespaces:      []string{apidefaults.Namespace},
				AzureIdentities: []string{readOnlyIdentity},
			},
		},
	}

	roleDenyReadOnlyIdentityUppercase := &types.RoleV6{
		Metadata: types.Metadata{
			Name:      "deny-identity-upper",
			Namespace: apidefaults.Namespace,
		},
		Spec: types.RoleSpecV6{
			Options: types.RoleOptions{
				MaxSessionTTL: types.NewDuration(maxSessionTTL),
			},
			Deny: types.RoleConditions{
				Namespaces:      []string{apidefaults.Namespace},
				AzureIdentities: []string{strings.ToUpper(readOnlyIdentity)},
			},
		},
	}

	roleDenyAll := &types.RoleV6{
		Metadata: types.Metadata{
			Name:      "deny-all",
			Namespace: apidefaults.Namespace,
		},
		Spec: types.RoleSpecV6{
			Options: types.RoleOptions{
				MaxSessionTTL: types.NewDuration(maxSessionTTL),
			},
			Deny: types.RoleConditions{
				Namespaces:      []string{apidefaults.Namespace},
				AzureIdentities: []string{types.Wildcard},
			},
		},
	}

	tests := []struct {
		name           string
		roles          RoleSet
		ttl            time.Duration
		overrideTTL    bool
		wantIdentities []string
		wantError      require.ErrorAssertionFunc
	}{
		{
			name:      "empty role set",
			roles:     nil,
			wantError: require.Error,
		},
		{
			name:        "no access role",
			overrideTTL: true,
			roles:       RoleSet{roleNoAccess},
			wantError: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, "this user cannot access Azure API, has no assigned identities")
			},
		},
		{
			name:           "readonly role, short session",
			overrideTTL:    false,
			ttl:            sessionShort,
			roles:          RoleSet{roleReadOnly},
			wantIdentities: []string{"readonly"},
			wantError:      require.NoError,
		},
		{
			name:        "readonly role, long session",
			overrideTTL: false,
			ttl:         sessionLong,
			roles:       RoleSet{roleReadOnly},
			wantError: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, "this user cannot access Azure API for 3h0m0s")
			},
		},
		{
			name:           "readonly role, override TTL",
			overrideTTL:    true,
			roles:          RoleSet{roleReadOnly},
			wantIdentities: []string{"readonly"},
			wantError:      require.NoError,
		},
		{
			name:           "full access role",
			overrideTTL:    true,
			roles:          RoleSet{roleFullAccess},
			wantIdentities: []string{"fullaccess", "readonly"},
			wantError:      require.NoError,
		},
		{
			name:           "denying a role works",
			overrideTTL:    true,
			roles:          RoleSet{roleFullAccess, roleDenyReadOnlyIdentity},
			wantIdentities: []string{"fullaccess"},
			wantError:      require.NoError,
		},
		{
			name:           "denying an uppercase role works",
			overrideTTL:    true,
			roles:          RoleSet{roleFullAccess, roleDenyReadOnlyIdentityUppercase},
			wantIdentities: []string{"fullaccess"},
			wantError:      require.NoError,
		},
		{
			name:           "denying wildcard works",
			overrideTTL:    true,
			roles:          RoleSet{roleFullAccess, roleDenyAll},
			wantIdentities: nil,
			wantError: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, "this user cannot access Azure API, has no assigned identities")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			identities, err := tt.roles.CheckAzureIdentities(tt.ttl, tt.overrideTTL)
			require.Equal(t, tt.wantIdentities, identities)
			tt.wantError(t, err)
		})
	}
}

func TestCheckGCPServiceAccounts(t *testing.T) {
	readOnlyAccount := "readonly@example-123456.iam.gserviceaccount.com"
	fullAccessAccount := "fullaccess@example-123456.iam.gserviceaccount.com"

	maxSessionTTL := time.Hour * 2
	sessionShort := time.Hour * 1
	sessionLong := time.Hour * 3

	roleNoAccess := &types.RoleV6{
		Metadata: types.Metadata{
			Name:      "noaccess",
			Namespace: apidefaults.Namespace,
		},
		Spec: types.RoleSpecV6{
			Options: types.RoleOptions{
				MaxSessionTTL: types.NewDuration(maxSessionTTL),
			},
			Allow: types.RoleConditions{
				Namespaces:         []string{apidefaults.Namespace},
				AppLabels:          types.Labels{types.Wildcard: []string{types.Wildcard}},
				GCPServiceAccounts: []string{},
			},
		},
	}
	roleReadOnly := &types.RoleV6{
		Metadata: types.Metadata{
			Name:      "readonly",
			Namespace: apidefaults.Namespace,
		},
		Spec: types.RoleSpecV6{
			Options: types.RoleOptions{
				MaxSessionTTL: types.NewDuration(maxSessionTTL),
			},
			Allow: types.RoleConditions{
				Namespaces:         []string{apidefaults.Namespace},
				AppLabels:          types.Labels{types.Wildcard: []string{types.Wildcard}},
				GCPServiceAccounts: []string{readOnlyAccount},
			},
		},
	}
	roleFullAccess := &types.RoleV6{
		Metadata: types.Metadata{
			Name:      "fullaccess",
			Namespace: apidefaults.Namespace,
		},
		Spec: types.RoleSpecV6{
			Options: types.RoleOptions{
				MaxSessionTTL: types.NewDuration(maxSessionTTL),
			},
			Allow: types.RoleConditions{
				Namespaces:         []string{apidefaults.Namespace},
				AppLabels:          types.Labels{types.Wildcard: []string{types.Wildcard}},
				GCPServiceAccounts: []string{readOnlyAccount, fullAccessAccount},
			},
		},
	}

	roleDenyReadOnlyAccount := &types.RoleV6{
		Metadata: types.Metadata{
			Name:      "deny-account",
			Namespace: apidefaults.Namespace,
		},
		Spec: types.RoleSpecV6{
			Options: types.RoleOptions{
				MaxSessionTTL: types.NewDuration(maxSessionTTL),
			},
			Deny: types.RoleConditions{
				Namespaces:         []string{apidefaults.Namespace},
				GCPServiceAccounts: []string{readOnlyAccount},
			},
		},
	}

	roleDenyReadOnlyAccountUppercase := &types.RoleV6{
		Metadata: types.Metadata{
			Name:      "deny-account-upper",
			Namespace: apidefaults.Namespace,
		},
		Spec: types.RoleSpecV6{
			Options: types.RoleOptions{
				MaxSessionTTL: types.NewDuration(maxSessionTTL),
			},
			Deny: types.RoleConditions{
				Namespaces:         []string{apidefaults.Namespace},
				GCPServiceAccounts: []string{strings.ToUpper(readOnlyAccount)},
			},
		},
	}

	roleDenyAll := &types.RoleV6{
		Metadata: types.Metadata{
			Name:      "deny-all",
			Namespace: apidefaults.Namespace,
		},
		Spec: types.RoleSpecV6{
			Options: types.RoleOptions{
				MaxSessionTTL: types.NewDuration(maxSessionTTL),
			},
			Deny: types.RoleConditions{
				Namespaces:         []string{apidefaults.Namespace},
				GCPServiceAccounts: []string{types.Wildcard},
			},
		},
	}

	tests := []struct {
		name         string
		roles        RoleSet
		ttl          time.Duration
		overrideTTL  bool
		wantAccounts []string
		wantError    require.ErrorAssertionFunc
	}{
		{
			name:      "empty role set",
			roles:     nil,
			wantError: require.Error,
		},
		{
			name:        "no access role",
			overrideTTL: true,
			roles:       RoleSet{roleNoAccess},
			wantError: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, "this user cannot request GCP API access, has no assigned service accounts")
			},
		},
		{
			name:         "readonly role, short session",
			overrideTTL:  false,
			ttl:          sessionShort,
			roles:        RoleSet{roleReadOnly},
			wantAccounts: []string{"readonly@example-123456.iam.gserviceaccount.com"},
			wantError:    require.NoError,
		},
		{
			name:        "readonly role, long session",
			overrideTTL: false,
			ttl:         sessionLong,
			roles:       RoleSet{roleReadOnly},
			wantError: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, "this user cannot request GCP API access for 3h0m0s")
			},
		},
		{
			name:         "readonly role, override TTL",
			overrideTTL:  true,
			roles:        RoleSet{roleReadOnly},
			wantAccounts: []string{"readonly@example-123456.iam.gserviceaccount.com"},
			wantError:    require.NoError,
		},
		{
			name:         "full access role",
			overrideTTL:  true,
			roles:        RoleSet{roleFullAccess},
			wantAccounts: []string{"fullaccess@example-123456.iam.gserviceaccount.com", "readonly@example-123456.iam.gserviceaccount.com"},
			wantError:    require.NoError,
		},
		{
			name:         "denying a role works",
			overrideTTL:  true,
			roles:        RoleSet{roleFullAccess, roleDenyReadOnlyAccount},
			wantAccounts: []string{"fullaccess@example-123456.iam.gserviceaccount.com"},
			wantError:    require.NoError,
		},
		{
			name:         "denying an uppercase role works",
			overrideTTL:  true,
			roles:        RoleSet{roleFullAccess, roleDenyReadOnlyAccountUppercase},
			wantAccounts: []string{"fullaccess@example-123456.iam.gserviceaccount.com"},
			wantError:    require.NoError,
		},
		{
			name:         "denying wildcard works",
			overrideTTL:  true,
			roles:        RoleSet{roleFullAccess, roleDenyAll},
			wantAccounts: nil,
			wantError: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, "this user cannot request GCP API access, has no assigned service accounts")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			accounts, err := tt.roles.CheckGCPServiceAccounts(tt.ttl, tt.overrideTTL)
			require.Equal(t, tt.wantAccounts, accounts)
			tt.wantError(t, err)
		})
	}
}

func TestCheckAccessToSAMLIdP(t *testing.T) {
	roleNoSAMLOptions := &types.RoleV6{
		Metadata: types.Metadata{Name: "roleNoSAMLOptions", Namespace: apidefaults.Namespace},
		Spec: types.RoleSpecV6{
			Options: types.RoleOptions{MaxSessionTTL: types.Duration(time.Hour)},
			Allow: types.RoleConditions{
				Namespaces: []string{apidefaults.Namespace},
			},
		},
	}
	//nolint:revive // Because we want this to be IdP.
	roleOnlyIdPBlock := &types.RoleV6{
		Metadata: types.Metadata{Name: "roleOnlyIdPBlock", Namespace: apidefaults.Namespace},
		Spec: types.RoleSpecV6{
			Options: types.RoleOptions{
				MaxSessionTTL: types.Duration(time.Hour),
				IDP:           &types.IdPOptions{},
			},
			Allow: types.RoleConditions{
				Namespaces: []string{apidefaults.Namespace},
			},
		},
	}
	roleOnlySAMLBlock := &types.RoleV6{
		Metadata: types.Metadata{Name: "roleOnlySAMLBlock", Namespace: apidefaults.Namespace},
		Spec: types.RoleSpecV6{
			Options: types.RoleOptions{
				MaxSessionTTL: types.Duration(time.Hour),
				IDP: &types.IdPOptions{
					SAML: &types.IdPSAMLOptions{},
				},
			},
			Allow: types.RoleConditions{
				Namespaces: []string{apidefaults.Namespace},
			},
		},
	}
	roleSAMLAllowed := &types.RoleV6{
		Metadata: types.Metadata{Name: "roleSAMLAllowed", Namespace: apidefaults.Namespace},
		Spec: types.RoleSpecV6{
			Options: types.RoleOptions{
				MaxSessionTTL: types.Duration(2 * time.Hour),
				IDP: &types.IdPOptions{
					SAML: &types.IdPSAMLOptions{
						Enabled: types.NewBoolOption(true),
					},
				},
			},
		},
	}
	roleSAMLNotAllowed := &types.RoleV6{
		Metadata: types.Metadata{Name: "roleSAMLNotAllowed", Namespace: apidefaults.Namespace},
		Spec: types.RoleSpecV6{
			Options: types.RoleOptions{
				MaxSessionTTL: types.Duration(2 * time.Hour),
				IDP: &types.IdPOptions{
					SAML: &types.IdPSAMLOptions{
						Enabled: types.NewBoolOption(false),
					},
				},
			},
		},
	}

	testCases := []struct {
		name                string
		roles               RoleSet
		authPrefSamlEnabled bool
		errAssertionFunc    require.ErrorAssertionFunc
	}{
		{
			name:                "role with no IdP block",
			roles:               RoleSet{roleNoSAMLOptions},
			authPrefSamlEnabled: true,
			errAssertionFunc:    require.NoError,
		},
		{
			name:                "role with only IdP block",
			roles:               RoleSet{roleOnlyIdPBlock},
			authPrefSamlEnabled: true,
			errAssertionFunc:    require.NoError,
		},
		{
			name:                "role with only SAML block",
			roles:               RoleSet{roleOnlySAMLBlock},
			authPrefSamlEnabled: true,
			errAssertionFunc:    require.NoError,
		},
		{
			name:                "only allowed role",
			roles:               RoleSet{roleSAMLAllowed},
			authPrefSamlEnabled: true,
			errAssertionFunc:    require.NoError,
		},
		{
			name:                "only denied role",
			roles:               RoleSet{roleSAMLNotAllowed},
			authPrefSamlEnabled: true,
			errAssertionFunc: func(tt require.TestingT, err error, i ...interface{}) {
				require.ErrorIs(t, err, trace.AccessDenied("user has been denied access to the SAML IdP by role roleSAMLNotAllowed"))
			},
		},
		{
			name:                "allowed and denied role",
			roles:               RoleSet{roleSAMLAllowed, roleSAMLNotAllowed},
			authPrefSamlEnabled: true,
			errAssertionFunc: func(tt require.TestingT, err error, i ...interface{}) {
				require.ErrorIs(t, err, trace.AccessDenied("user has been denied access to the SAML IdP by role roleSAMLNotAllowed"))
			},
		},
		{
			name:                "allowed role, but denied at cluster level",
			roles:               RoleSet{roleSAMLAllowed},
			authPrefSamlEnabled: false,
			errAssertionFunc: func(tt require.TestingT, err error, i ...interface{}) {
				require.ErrorIs(t, err, trace.AccessDenied("SAML IdP is disabled at the cluster level"))
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			authPref, err := types.NewAuthPreference(types.AuthPreferenceSpecV2{
				IDP: &types.IdPOptions{
					SAML: &types.IdPSAMLOptions{
						Enabled: types.NewBoolOption(tc.authPrefSamlEnabled),
					},
				},
			})
			require.NoError(t, err)
			tc.errAssertionFunc(t, tc.roles.CheckAccessToSAMLIdP(authPref))
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
	wildcardRole := &types.RoleV6{
		Metadata: types.Metadata{
			Name:      "wildcard-labels",
			Namespace: apidefaults.Namespace,
		},
		Spec: types.RoleSpecV6{
			Allow: types.RoleConditions{
				Namespaces:       []string{apidefaults.Namespace},
				KubernetesLabels: types.Labels{types.Wildcard: []string{types.Wildcard}},
			},
		},
	}
	matchingLabelsRole := &types.RoleV6{
		Metadata: types.Metadata{
			Name:      "matching-labels",
			Namespace: apidefaults.Namespace,
		},
		Spec: types.RoleSpecV6{
			Allow: types.RoleConditions{
				Namespaces: []string{apidefaults.Namespace},
				KubernetesLabels: types.Labels{
					"foo": apiutils.Strings{"bar"},
					"baz": apiutils.Strings{"qux"},
				},
			},
		},
	}
	matchingLabelsRoleWithMFA := &types.RoleV6{
		Metadata: types.Metadata{
			Name:      "matching-labels-mfa",
			Namespace: apidefaults.Namespace,
		},
		Spec: types.RoleSpecV6{
			Options: types.RoleOptions{
				RequireMFAType: types.RequireMFAType_SESSION,
			},
			Allow: types.RoleConditions{
				Namespaces:                 []string{apidefaults.Namespace},
				KubernetesLabelsExpression: `labels.foo == "bar" && labels.baz == "qux"`,
			},
		},
	}
	matchingLabelsRoleWithDeviceTrust := &types.RoleV6{
		Metadata: types.Metadata{
			Name:      "matching-labels-devicetrust",
			Namespace: apidefaults.Namespace,
		},
		Spec: types.RoleSpecV6{
			Options: types.RoleOptions{
				DeviceTrustMode: constants.DeviceTrustModeRequired,
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
	noLabelsRole := &types.RoleV6{
		Metadata: types.Metadata{
			Name:      "no-labels",
			Namespace: apidefaults.Namespace,
		},
		Spec: types.RoleSpecV6{
			Allow: types.RoleConditions{
				Namespaces: []string{apidefaults.Namespace},
			},
		},
	}
	mismatchingLabelsRole := &types.RoleV6{
		Metadata: types.Metadata{
			Name:      "mismatching-labels",
			Namespace: apidefaults.Namespace,
		},
		Spec: types.RoleSpecV6{
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
		roles     []*types.RoleV6
		cluster   *types.KubernetesCluster
		state     AccessState
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
			roles:     []*types.RoleV6{noLabelsRole},
			cluster:   clusterNoLabels,
			hasAccess: false,
		},
		{
			name:      "role with wildcard labels matches cluster without labels",
			roles:     []*types.RoleV6{wildcardRole},
			cluster:   clusterNoLabels,
			hasAccess: true,
		},
		{
			name:      "role with wildcard labels matches cluster with labels",
			roles:     []*types.RoleV6{wildcardRole},
			cluster:   clusterWithLabels,
			hasAccess: true,
		},
		{
			name:      "role with labels does not match cluster with no labels",
			roles:     []*types.RoleV6{matchingLabelsRole},
			cluster:   clusterNoLabels,
			hasAccess: false,
		},
		{
			name:      "role with labels matches cluster with labels",
			roles:     []*types.RoleV6{matchingLabelsRole},
			cluster:   clusterWithLabels,
			hasAccess: true,
		},
		{
			name:      "role with mismatched labels does not match cluster with labels",
			roles:     []*types.RoleV6{mismatchingLabelsRole},
			cluster:   clusterWithLabels,
			hasAccess: false,
		},
		{
			name:      "one role in the roleset matches",
			roles:     []*types.RoleV6{mismatchingLabelsRole, noLabelsRole, matchingLabelsRole},
			cluster:   clusterWithLabels,
			hasAccess: true,
		},
		{
			name:      "role requires MFA but MFA not verified",
			roles:     []*types.RoleV6{matchingLabelsRole, matchingLabelsRoleWithMFA},
			cluster:   clusterWithLabels,
			state:     AccessState{MFAVerified: false},
			hasAccess: false,
		},
		{
			name:      "role requires MFA and MFA verified",
			roles:     []*types.RoleV6{matchingLabelsRole, matchingLabelsRoleWithMFA},
			cluster:   clusterWithLabels,
			state:     AccessState{MFAVerified: true},
			hasAccess: true,
		},
		{
			name:      "cluster requires MFA but MFA not verified",
			roles:     []*types.RoleV6{matchingLabelsRole},
			cluster:   clusterWithLabels,
			state:     AccessState{MFAVerified: false, MFARequired: MFARequiredAlways},
			hasAccess: false,
		},
		{
			name:      "role requires MFA and MFA verified",
			roles:     []*types.RoleV6{matchingLabelsRole},
			cluster:   clusterWithLabels,
			state:     AccessState{MFAVerified: true, MFARequired: MFARequiredAlways},
			hasAccess: true,
		},
		{
			name:    "role requires device trust, device not verified",
			roles:   []*types.RoleV6{wildcardRole, matchingLabelsRole, matchingLabelsRoleWithDeviceTrust},
			cluster: clusterWithLabels,
			state: AccessState{
				EnableDeviceVerification: true,
				DeviceVerified:           false,
			},
			hasAccess: false,
		},
		{
			name:    "role requires device trust, device verified",
			roles:   []*types.RoleV6{wildcardRole, matchingLabelsRole, matchingLabelsRoleWithDeviceTrust},
			cluster: clusterWithLabels,
			state: AccessState{
				EnableDeviceVerification: true,
				DeviceVerified:           true,
			},
			hasAccess: true,
		},
		{
			name:    "role requires device trust, resource doesn't match",
			roles:   []*types.RoleV6{wildcardRole, matchingLabelsRole, matchingLabelsRoleWithDeviceTrust},
			cluster: clusterNoLabels,
			state: AccessState{
				EnableDeviceVerification: true,
				DeviceVerified:           false,
			},
			hasAccess: true, // doesn't match device trust role
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			k8sV3, err := types.NewKubernetesClusterV3FromLegacyCluster(apidefaults.Namespace, tc.cluster)
			require.NoError(t, err)

			accessChecker := makeAccessCheckerWithRolePointers(tc.roles)
			err = accessChecker.CheckAccess(k8sV3, tc.state)
			if tc.hasAccess {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				require.True(t, trace.IsAccessDenied(err))
			}
		})
	}
}

func TestDesktopRecordingEnabled(t *testing.T) {
	for _, test := range []struct {
		desc         string
		roleSet      RoleSet
		shouldRecord bool
	}{
		{
			desc: "single role recording disabled",
			roleSet: NewRoleSet(
				newRole(func(r *types.RoleV6) {
					r.SetName("no-record")
					r.SetOptions(types.RoleOptions{
						RecordSession: &types.RecordSession{Desktop: types.NewBoolOption(false)},
					})
				}),
			),
			shouldRecord: false,
		},
		{
			desc: "multiple roles, one requires recording",
			roleSet: NewRoleSet(
				newRole(func(r *types.RoleV6) {
					r.SetOptions(types.RoleOptions{
						RecordSession: &types.RecordSession{Desktop: types.NewBoolOption(false)},
					})
				}),
				newRole(func(r *types.RoleV6) {
					r.SetOptions(types.RoleOptions{
						RecordSession: &types.RecordSession{Desktop: types.NewBoolOption(false)},
					})
				}),
				// recording defaults to true, so a default role should force recording
				newRole(func(r *types.RoleV6) {}),
			),
			shouldRecord: true,
		},
	} {
		t.Run(test.desc, func(t *testing.T) {
			require.Equal(t, test.shouldRecord, test.roleSet.RecordDesktopSession())
		})
	}
}

func TestDesktopClipboard(t *testing.T) {
	for _, test := range []struct {
		desc         string
		roleSet      RoleSet
		hasClipboard bool
	}{
		{
			desc: "single role, unspecified, defaults true",
			roleSet: NewRoleSet(
				newRole(func(r *types.RoleV6) {}),
			),
			hasClipboard: true,
		},
		{
			desc: "single role, explicitly disabled",
			roleSet: NewRoleSet(
				newRole(func(r *types.RoleV6) {
					r.SetOptions(types.RoleOptions{
						DesktopClipboard: types.NewBoolOption(false),
					})
				}),
			),
			hasClipboard: false,
		},
		{
			desc: "multiple conflicting roles, disable wins",
			roleSet: NewRoleSet(
				newRole(func(r *types.RoleV6) {}),
				newRole(func(r *types.RoleV6) {
					r.SetOptions(types.RoleOptions{
						DesktopClipboard: types.NewBoolOption(false),
					})
				}),
			),
			hasClipboard: false,
		},
	} {
		t.Run(test.desc, func(t *testing.T) {
			require.Equal(t, test.hasClipboard, test.roleSet.DesktopClipboard())
		})
	}
}

func TestDesktopDirectorySharing(t *testing.T) {
	for _, test := range []struct {
		desc                string
		roleSet             RoleSet
		hasDirectorySharing bool
	}{
		{
			desc: "single role, unspecified, defaults true",
			roleSet: NewRoleSet(
				newRole(func(r *types.RoleV6) {}),
			),
			hasDirectorySharing: true,
		},
		{
			desc: "single role, explicitly disabled",
			roleSet: NewRoleSet(
				newRole(func(r *types.RoleV6) {
					r.SetOptions(types.RoleOptions{
						DesktopDirectorySharing: types.NewBoolOption(false),
					})
				}),
			),
			hasDirectorySharing: false,
		},
		{
			desc: "multiple conflicting roles, disable wins",
			roleSet: NewRoleSet(
				newRole(func(r *types.RoleV6) {
					r.SetOptions(types.RoleOptions{
						DesktopDirectorySharing: types.NewBoolOption(false),
					})
				}),
				newRole(func(r *types.RoleV6) {
					r.SetOptions(types.RoleOptions{
						DesktopDirectorySharing: types.NewBoolOption(true),
					})
				}),
			),
			hasDirectorySharing: false,
		},
	} {
		t.Run(test.desc, func(t *testing.T) {
			require.Equal(t, test.hasDirectorySharing, test.roleSet.DesktopDirectorySharing())
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
		name    string
		roleSet RoleSet
		checks  []check
	}{
		{
			name:    "no roles, no access",
			roleSet: RoleSet{},
			checks: []check{
				{desktop: desktopNoLabels, login: "admin", hasAccess: false},
				{desktop: desktop2012, login: "admin", hasAccess: false},
			},
		},
		{
			name: "empty label, no access",
			roleSet: RoleSet{
				newRole(func(r *types.RoleV6) {
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
			roleSet: RoleSet{
				newRole(func(r *types.RoleV6) {
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
			roleSet: RoleSet{
				newRole(func(r *types.RoleV6) {
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
			roleSet: RoleSet{
				newRole(func(r *types.RoleV6) {
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
			roleSet: RoleSet{
				newRole(func(r *types.RoleV6) {
					r.Spec.Allow.WindowsDesktopLogins = []string{"admin"}
					r.Spec.Allow.NodeLabels = types.Labels{"win_version": []string{"2012"}}
				}),
				newRole(func(r *types.RoleV6) {
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
		{
			name: "labels expression more permissive than another role",
			roleSet: RoleSet{
				newRole(func(r *types.RoleV6) {
					r.Spec.Allow.WindowsDesktopLogins = []string{"admin"}
					r.Spec.Allow.NodeLabels = types.Labels{"win_version": []string{"2012"}}
				}),
				newRole(func(r *types.RoleV6) {
					r.Spec.Allow.WindowsDesktopLabels = nil
					r.Spec.Allow.WindowsDesktopLabelsExpression = matchAllExpression
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
			for i, check := range test.checks {
				msg := fmt.Sprintf("check=%d, user=%v, server=%v, should_have_access=%v",
					i, check.login, check.desktop.GetName(), check.hasAccess)
				err := test.roleSet.checkAccess(check.desktop, wrappers.Traits{}, AccessState{}, NewWindowsLoginMatcher(check.login))
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

func TestCheckAccessToUserGroups(t *testing.T) {
	userGroupNoLabels := &types.UserGroupV1{
		ResourceHeader: types.ResourceHeader{
			Kind:     types.KindUserGroup,
			Metadata: types.Metadata{Name: "no-labels"},
		},
	}
	userGroupLabels := &types.UserGroupV1{
		ResourceHeader: types.ResourceHeader{
			Kind: types.KindUserGroup,
			Metadata: types.Metadata{
				Name: "labels",
				Labels: map[string]string{
					"a": "b",
				},
			},
		},
	}

	type check struct {
		userGroup *types.UserGroupV1
		hasAccess bool
	}

	for _, test := range []struct {
		name    string
		roleSet RoleSet
		checks  []check
	}{
		{
			name:    "no roles, no access",
			roleSet: RoleSet{},
			checks: []check{
				{userGroup: userGroupNoLabels, hasAccess: false},
				{userGroup: userGroupLabels, hasAccess: false},
			},
		},
		{
			name: "no matching labels, no access",
			roleSet: RoleSet{
				newRole(func(r *types.RoleV6) {
					r.Spec.Deny.Namespaces = []string{apidefaults.Namespace}
					r.Spec.Allow.GroupLabels = types.Labels{"a": []string{"c"}}
				}),
			},
			checks: []check{
				{userGroup: userGroupNoLabels, hasAccess: false},
				{userGroup: userGroupLabels, hasAccess: false},
			},
		},
		{
			name: "deny labels, no access",
			roleSet: RoleSet{
				newRole(func(r *types.RoleV6) {
					r.Spec.Deny.Namespaces = []string{apidefaults.Namespace}
					r.Spec.Allow.GroupLabels = types.Labels{"a": []string{"b"}}
					r.Spec.Deny.GroupLabels = types.Labels{"a": []string{"b"}}
				}),
			},
			checks: []check{
				{userGroup: userGroupNoLabels, hasAccess: false},
				{userGroup: userGroupLabels, hasAccess: false},
			},
		},
		{
			name: "wild card, access",
			roleSet: RoleSet{
				newRole(func(r *types.RoleV6) {
					r.Spec.Deny.Namespaces = []string{apidefaults.Namespace}
					r.Spec.Allow.GroupLabels = types.Labels{types.Wildcard: []string{types.Wildcard}}
				}),
			},
			checks: []check{
				{userGroup: userGroupNoLabels, hasAccess: true},
				{userGroup: userGroupLabels, hasAccess: true},
			},
		},
		{
			name: "labels expression, access",
			roleSet: RoleSet{
				newRole(func(r *types.RoleV6) {
					r.Spec.Deny.Namespaces = []string{apidefaults.Namespace}
					r.Spec.Allow.GroupLabelsExpression = matchAllExpression
				}),
			},
			checks: []check{
				{userGroup: userGroupNoLabels, hasAccess: true},
				{userGroup: userGroupLabels, hasAccess: true},
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			for i, check := range test.checks {
				msg := fmt.Sprintf("check=%d, userGroup=%v, should_have_access=%v",
					i, check.userGroup.GetName(), check.hasAccess)
				err := test.roleSet.checkAccess(check.userGroup, wrappers.Traits{}, AccessState{})
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
//	go test -bench=.
//
// To run benchmark and obtain CPU and memory profiling:
//
//	go test -bench=. -cpuprofile=cpu.prof -memprofile=mem.prof
//
// To use the command line tool to read the profile:
//
//	go tool pprof cpu.prof
//	go tool pprof cpu.prof
//
// To generate a graph:
//
//	go tool pprof --pdf cpu.prof > cpu.pdf
//	go tool pprof --pdf mem.prof > mem.pdf
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
		set = append(set, &types.RoleV6{
			Kind:    types.KindRole,
			Version: types.V3,
			Metadata: types.Metadata{
				Name:      strconv.Itoa(i),
				Namespace: apidefaults.Namespace,
			},
			Spec: types.RoleSpecV6{
				Allow: types.RoleConditions{
					Logins:     []string{"admin", "one", "two", "three", "four"},
					NodeLabels: types.Labels{"a": []string{"b"}},
				},
			},
		})
	}
	userTraits := wrappers.Traits{}

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
				_ = set.checkAccess(
					servers[i],
					userTraits,
					AccessState{},
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

func (f *userGetter) GetUser(ctx context.Context, name string, _ bool) (types.User, error) {
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
		role, err := types.NewRole(uuid.New().String(), types.RoleSpecV6{Options: types.RoleOptions{Lock: mode}})
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
		role, err := types.NewRole(uuid.New().String(), types.RoleSpecV6{Allow: types.RoleConditions{
			Rules: []types.Rule{{Resources: []string{types.KindSession}, Verbs: []string{types.VerbList}, Where: where}},
		}})
		require.NoError(t, err)
		return role
	}
	denyWhere := func(where string) types.Role {
		role, err := types.NewRole(uuid.New().String(), types.RoleSpecV6{Deny: types.RoleConditions{
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
	require.Empty(t, gocmp.Diff(cond, guestParticipantCond))

	// Add a role allowing access to the user's own session recordings.
	role = allowWhere(`contains(session.participants, user.metadata.name)`)
	userParticipantCond := func(user types.User) *types.WhereExpr {
		return &types.WhereExpr{Contains: types.WhereExpr2{L: &types.WhereExpr{Field: "participants"}, R: &types.WhereExpr{Literal: user.GetName()}}}
	}
	set = append(set, role)

	cond, err = set.ExtractConditionForIdentifier(&Context{}, apidefaults.Namespace, types.KindSession, types.VerbList, SessionIdentifier)
	require.NoError(t, err)
	require.Empty(t, gocmp.Diff(cond, userParticipantCond(emptyUser)))
	cond, err = set.ExtractConditionForIdentifier(&Context{User: user}, apidefaults.Namespace, types.KindSession, types.VerbList, SessionIdentifier)
	require.NoError(t, err)
	require.Empty(t, gocmp.Diff(cond, userParticipantCond(user)))
	cond, err = set.ExtractConditionForIdentifier(&Context{User: user2}, apidefaults.Namespace, types.KindSession, types.VerbList, SessionIdentifier)
	require.NoError(t, err)
	require.Empty(t, gocmp.Diff(cond, &types.WhereExpr{Or: types.WhereExpr2{L: guestParticipantCond, R: userParticipantCond(user2)}}))

	// Add a role denying access to sessions with root login.
	role = denyWhere(`equals(session.login, "root")`)
	noRootLoginCond := &types.WhereExpr{Not: &types.WhereExpr{Equals: types.WhereExpr2{L: &types.WhereExpr{Field: "login"}, R: &types.WhereExpr{Literal: "root"}}}}
	set = append(set, role)

	cond, err = set.ExtractConditionForIdentifier(&Context{}, apidefaults.Namespace, types.KindSession, types.VerbList, SessionIdentifier)
	require.NoError(t, err)
	require.Empty(t, gocmp.Diff(cond, &types.WhereExpr{And: types.WhereExpr2{L: noRootLoginCond, R: userParticipantCond(emptyUser)}}))
	cond, err = set.ExtractConditionForIdentifier(&Context{User: user}, apidefaults.Namespace, types.KindSession, types.VerbList, SessionIdentifier)
	require.NoError(t, err)
	require.Empty(t, gocmp.Diff(cond, &types.WhereExpr{And: types.WhereExpr2{L: noRootLoginCond, R: userParticipantCond(user)}}))
	cond, err = set.ExtractConditionForIdentifier(&Context{User: user2}, apidefaults.Namespace, types.KindSession, types.VerbList, SessionIdentifier)
	require.NoError(t, err)
	require.Empty(t, gocmp.Diff(cond, &types.WhereExpr{And: types.WhereExpr2{L: noRootLoginCond, R: &types.WhereExpr{Or: types.WhereExpr2{L: guestParticipantCond, R: userParticipantCond(user2)}}}}))

	// Add a role denying access for user2.
	role = denyWhere(fmt.Sprintf(`equals(user.metadata.name, "%s")`, user2.GetName()))
	set = append(set, role)

	cond, err = set.ExtractConditionForIdentifier(&Context{}, apidefaults.Namespace, types.KindSession, types.VerbList, SessionIdentifier)
	require.NoError(t, err)
	require.Empty(t, gocmp.Diff(cond, &types.WhereExpr{And: types.WhereExpr2{L: noRootLoginCond, R: userParticipantCond(emptyUser)}}))
	cond, err = set.ExtractConditionForIdentifier(&Context{User: user}, apidefaults.Namespace, types.KindSession, types.VerbList, SessionIdentifier)
	require.NoError(t, err)
	require.Empty(t, gocmp.Diff(cond, &types.WhereExpr{And: types.WhereExpr2{L: noRootLoginCond, R: userParticipantCond(user)}}))
	_, err = set.ExtractConditionForIdentifier(&Context{User: user2}, apidefaults.Namespace, types.KindSession, types.VerbList, SessionIdentifier)
	require.True(t, trace.IsAccessDenied(err))

	// Add a role allowing access to all sessions.
	// This should cause all the other allow rules' conditions to be dropped.
	role = allowWhere(``)
	set = append(set, role)

	cond, err = set.ExtractConditionForIdentifier(&Context{}, apidefaults.Namespace, types.KindSession, types.VerbList, SessionIdentifier)
	require.NoError(t, err)
	require.Empty(t, gocmp.Diff(cond, noRootLoginCond))
	cond, err = set.ExtractConditionForIdentifier(&Context{User: user}, apidefaults.Namespace, types.KindSession, types.VerbList, SessionIdentifier)
	require.NoError(t, err)
	require.Empty(t, gocmp.Diff(cond, noRootLoginCond))
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
	roleA := &types.RoleV6{
		Metadata: types.Metadata{Name: "roleA", Namespace: apidefaults.Namespace},
		Spec: types.RoleSpecV6{
			Allow: types.RoleConditions{
				KubeGroups:                 []string{"system:masters"},
				KubeUsers:                  []string{"dev-user"},
				KubernetesLabelsExpression: `labels["env"] == "dev"`,
			},
		},
	}
	roleB := &types.RoleV6{
		Metadata: types.Metadata{Name: "roleB", Namespace: apidefaults.Namespace},
		Spec: types.RoleSpecV6{
			Allow: types.RoleConditions{
				KubeGroups: []string{"limited"},
				KubeUsers:  []string{"limited-user"},
				KubernetesLabels: map[string]apiutils.Strings{
					"env": []string{"prod"},
				},
			},
		},
	}
	roleC := &types.RoleV6{
		Metadata: types.Metadata{Name: "roleC", Namespace: apidefaults.Namespace},
		Spec: types.RoleSpecV6{
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
		errorFunc     func(t *testing.T, err error)
	}{
		{
			name: "empty kube labels should returns all kube users/groups",
			roles: RoleSet{
				&types.RoleV6{
					Metadata: types.Metadata{Name: "role-dev", Namespace: apidefaults.Namespace},
					Spec: types.RoleSpecV6{
						Allow: types.RoleConditions{
							KubeUsers:  []string{"dev-user"},
							KubeGroups: []string{"system:masters"},
							KubernetesLabels: map[string]apiutils.Strings{
								"*": []string{"*"},
							},
						},
					},
				},
				&types.RoleV6{
					Metadata: types.Metadata{Name: "role-prod", Namespace: apidefaults.Namespace},
					Spec: types.RoleSpecV6{
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
				&types.RoleV6{
					Metadata: types.Metadata{Name: "roleD", Namespace: apidefaults.Namespace},
					Spec: types.RoleSpecV6{
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
				&types.RoleV6{
					Metadata: types.Metadata{Name: "roleA", Namespace: apidefaults.Namespace},
					Spec: types.RoleSpecV6{
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
				&types.RoleV6{
					Metadata: types.Metadata{Name: "deny", Namespace: apidefaults.Namespace},
					Spec: types.RoleSpecV6{
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
		{
			name:          "v5 role empty deny.kubernetes_labels",
			kubeResLabels: nil,
			roles: RoleSet{
				&types.RoleV6{
					Version:  types.V5,
					Metadata: types.Metadata{Name: "roleV5A", Namespace: apidefaults.Namespace},
					Spec: types.RoleSpecV6{
						Allow: types.RoleConditions{
							KubernetesLabels: map[string]apiutils.Strings{types.Wildcard: []string{types.Wildcard}},
							KubeGroups:       []string{"system:masters"},
							KubeUsers:        []string{"dev-user"},
						},
						Deny: types.RoleConditions{
							KubeGroups: []string{"system:masters"},
							KubeUsers:  []string{"dev-user"},
						},
					},
				},
			},
			errorFunc: func(t *testing.T, err error) {
				require.IsType(t, trace.AccessDenied(""), err)
			},
		},
		{
			name:          "v5 role with wildcard deny.kubernetes_labels",
			kubeResLabels: nil,
			roles: RoleSet{
				&types.RoleV6{
					Version:  types.V5,
					Metadata: types.Metadata{Name: "roleV5A", Namespace: apidefaults.Namespace},
					Spec: types.RoleSpecV6{
						Allow: types.RoleConditions{
							KubernetesLabels: map[string]apiutils.Strings{types.Wildcard: []string{types.Wildcard}},
							KubeGroups:       []string{"system:masters"},
							KubeUsers:        []string{"dev-user"},
						},
						Deny: types.RoleConditions{
							KubernetesLabels: map[string]apiutils.Strings{types.Wildcard: []string{types.Wildcard}},
							KubeGroups:       []string{"system:masters"},
							KubeUsers:        []string{"dev-user"},
						},
					},
				},
			},
			errorFunc: func(t *testing.T, err error) {
				require.IsType(t, trace.AccessDenied(""), err)
			},
		},
		{
			name:          "v3 role with empty deny.kubernetes_labels",
			kubeResLabels: nil,
			roles: RoleSet{
				&types.RoleV6{
					Version:  types.V3,
					Metadata: types.Metadata{Name: "roleV3A", Namespace: apidefaults.Namespace},
					Spec: types.RoleSpecV6{
						Allow: types.RoleConditions{
							KubernetesLabels: map[string]apiutils.Strings{types.Wildcard: []string{types.Wildcard}},
							KubeGroups:       []string{"system:masters"},
							KubeUsers:        []string{"dev-user"},
						},
						Deny: types.RoleConditions{
							KubeGroups: []string{"system:masters"},
							KubeUsers:  []string{"dev-user"},
						},
					},
				},
			},
			errorFunc: func(t *testing.T, err error) {
				require.IsType(t, trace.AccessDenied(""), err)
			},
		},
		{
			name:          "v3 with wildcard deny.kubernetes_labels",
			kubeResLabels: nil,
			roles: RoleSet{
				&types.RoleV6{
					Version:  types.V3,
					Metadata: types.Metadata{Name: "roleV3A", Namespace: apidefaults.Namespace},
					Spec: types.RoleSpecV6{
						Allow: types.RoleConditions{
							KubernetesLabels: map[string]apiutils.Strings{types.Wildcard: []string{types.Wildcard}},
							KubeGroups:       []string{"system:masters"},
							KubeUsers:        []string{"dev-user"},
						},
						Deny: types.RoleConditions{
							KubernetesLabels: map[string]apiutils.Strings{types.Wildcard: []string{types.Wildcard}},
							KubeGroups:       []string{"system:masters"},
							KubeUsers:        []string{"dev-user"},
						},
					},
				},
			},
			errorFunc: func(t *testing.T, err error) {
				require.IsType(t, trace.AccessDenied(""), err)
			},
		},
		{
			name:          "v3 role with custom deny.kubernetes_labels",
			kubeResLabels: nil,
			roles: RoleSet{
				&types.RoleV6{
					Version:  types.V3,
					Metadata: types.Metadata{Name: "roleV3A", Namespace: apidefaults.Namespace},
					Spec: types.RoleSpecV6{
						Allow: types.RoleConditions{
							KubernetesLabels: map[string]apiutils.Strings{types.Wildcard: []string{types.Wildcard}},
							KubeGroups:       []string{"system:masters"},
							KubeUsers:        []string{"dev-user"},
						},
						Deny: types.RoleConditions{
							KubernetesLabels: map[string]apiutils.Strings{"env": []string{"env"}},
							KubeGroups:       []string{"system:masters"},
							KubeUsers:        []string{"dev-user"},
						},
					},
				},
			},
			wantUsers:  []string{"dev-user"},
			wantGroups: []string{"system:masters"},
		},
	}

	var userTraits wrappers.Traits

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			matcher := NewKubernetesClusterLabelMatcher(tc.kubeResLabels, userTraits)
			gotGroups, gotUsers, err := tc.roles.CheckKubeGroupsAndUsers(time.Hour, true, matcher)
			if tc.errorFunc == nil {
				require.NoError(t, err)
			} else {
				tc.errorFunc(t, err)
			}

			require.ElementsMatch(t, tc.wantUsers, gotUsers)
			require.ElementsMatch(t, tc.wantGroups, gotGroups)
		})
	}
}

func TestWindowsDesktopGroups(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		desc          string
		roles         []types.Role
		desktopLabels map[string]string
		expectDenied  bool
		expectGroups  []string
	}{
		{
			desc: "allow labels",
			roles: []types.Role{
				newRole(func(r *types.RoleV6) {
					r.Spec.Allow.WindowsDesktopLabels = types.Labels{"env": {"prod"}}
					r.Spec.Allow.DesktopGroups = []string{"a", "b", "c"}
					r.Spec.Options.CreateDesktopUser = types.NewBoolOption(true)
				}),
			},
			desktopLabels: map[string]string{"env": "prod"},
			expectGroups:  []string{"a", "b", "c"},
		},
		{
			desc: "allow expression",
			roles: []types.Role{
				newRole(func(r *types.RoleV6) {
					r.Spec.Allow.WindowsDesktopLabels = nil
					r.Spec.Allow.WindowsDesktopLabelsExpression = `labels["env"] == "prod"`
					r.Spec.Allow.DesktopGroups = []string{"a", "b", "c"}
					r.Spec.Options.CreateDesktopUser = types.NewBoolOption(true)
				}),
			},
			desktopLabels: map[string]string{"env": "prod"},
			expectGroups:  []string{"a", "b", "c"},
		},
		{
			desc: "option denied",
			roles: []types.Role{
				newRole(func(r *types.RoleV6) {
					r.Spec.Allow.WindowsDesktopLabels = types.Labels{"*": {"*"}}
					r.Spec.Allow.DesktopGroups = []string{"a", "b", "c"}
					r.Spec.Options.CreateDesktopUser = types.NewBoolOption(true)
				}),
				newRole(func(r *types.RoleV6) {
					r.Spec.Allow.WindowsDesktopLabels = types.Labels{"env": {"prod"}}
					r.Spec.Options.CreateDesktopUser = types.NewBoolOption(false)
				}),
			},
			desktopLabels: map[string]string{"env": "prod"},
			expectDenied:  true,
		},
		{
			desc: "irrelevant deny",
			roles: []types.Role{
				newRole(func(r *types.RoleV6) {
					r.Spec.Allow.WindowsDesktopLabels = types.Labels{"*": {"*"}}
					r.Spec.Allow.DesktopGroups = []string{"a", "b", "c"}
					r.Spec.Options.CreateDesktopUser = types.NewBoolOption(true)
				}),
				newRole(func(r *types.RoleV6) {
					r.Spec.Allow.WindowsDesktopLabels = types.Labels{"env": {"prod"}}
					r.Spec.Options.CreateDesktopUser = types.NewBoolOption(false)
				}),
			},
			desktopLabels: map[string]string{"env": "staging"},
			expectGroups:  []string{"a", "b", "c"},
		},
		{
			desc: "one group denied",
			roles: []types.Role{
				newRole(func(r *types.RoleV6) {
					r.Spec.Allow.WindowsDesktopLabels = types.Labels{"env": {"prod"}}
					r.Spec.Allow.DesktopGroups = []string{"a", "b", "c"}
					r.Spec.Options.CreateDesktopUser = types.NewBoolOption(true)
				}),
				newRole(func(r *types.RoleV6) {
					r.Spec.Deny.WindowsDesktopLabels = nil
					r.Spec.Deny.WindowsDesktopLabelsExpression = matchAllExpression
					r.Spec.Deny.DesktopGroups = []string{"b"}
					r.Spec.Options.CreateDesktopUser = types.NewBoolOption(true)
				}),
			},
			desktopLabels: map[string]string{"env": "prod"},
			expectGroups:  []string{"a", "c"},
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			desktop := mustMakeTestWindowsDesktop(tc.desktopLabels)
			set := NewRoleSet(tc.roles...)
			accessChecker := makeAccessCheckerWithRoleSet(set)
			groups, err := accessChecker.DesktopGroups(desktop)
			if tc.expectDenied {
				require.True(t, trace.IsAccessDenied(err), "expected access denied error, got %v", err)
				return
			}
			require.NoError(t, err, trace.DebugReport(err))
			require.ElementsMatch(t, tc.expectGroups, groups)
		})
	}
}

func TestGetKubeResources(t *testing.T) {
	t.Parallel()
	podA := types.KubernetesResource{
		Kind:      types.KindKubePod,
		Namespace: "test",
		Name:      "podA",
	}
	podB := types.KubernetesResource{
		Kind:      types.KindKubePod,
		Namespace: "test",
		Name:      "podB",
	}
	for _, tc := range []struct {
		desc                        string
		roles                       []types.Role
		clusterLabels               map[string]string
		expectAllowed, expectDenied []types.KubernetesResource
	}{
		{
			desc: "labels allow",
			roles: []types.Role{
				newRole(func(r *types.RoleV6) {
					r.Spec.Allow.KubernetesLabels = types.Labels{"env": {"prod"}}
					r.Spec.Allow.KubernetesResources = []types.KubernetesResource{podA, podB}
				}),
			},
			clusterLabels: map[string]string{"env": "prod"},
			expectAllowed: []types.KubernetesResource{podA, podB},
		},
		{
			desc: "labels expression allow",
			roles: []types.Role{
				newRole(func(r *types.RoleV6) {
					r.Spec.Allow.KubernetesLabelsExpression = `labels["env"] == "prod"`
					r.Spec.Allow.KubernetesResources = []types.KubernetesResource{podA, podB}
				}),
			},
			clusterLabels: map[string]string{"env": "prod"},
			expectAllowed: []types.KubernetesResource{podA, podB},
		},
		{
			desc: "one denied",
			roles: []types.Role{
				newRole(func(r *types.RoleV6) {
					r.Spec.Allow.KubernetesLabelsExpression = `labels["env"] == "prod"`
					r.Spec.Allow.KubernetesResources = []types.KubernetesResource{podA, podB}
				}),
				newRole(func(r *types.RoleV6) {
					r.Spec.Deny.KubernetesLabels = types.Labels{"env": {"prod"}}
					r.Spec.Deny.KubernetesResources = []types.KubernetesResource{podA}
				}),
			},
			clusterLabels: map[string]string{"env": "prod"},
			expectAllowed: []types.KubernetesResource{podA, podB},
			expectDenied:  []types.KubernetesResource{podA},
		},
		{
			desc: "irrelevant deny",
			roles: []types.Role{
				newRole(func(r *types.RoleV6) {
					r.Spec.Allow.KubernetesLabelsExpression = `labels["env"] == "staging"`
					r.Spec.Allow.KubernetesResources = []types.KubernetesResource{podA, podB}
				}),
				newRole(func(r *types.RoleV6) {
					r.Spec.Deny.KubernetesLabels = types.Labels{"env": {"prod"}}
					r.Spec.Deny.KubernetesResources = []types.KubernetesResource{podA}
				}),
			},
			clusterLabels: map[string]string{"env": "staging"},
			expectAllowed: []types.KubernetesResource{podA, podB},
			expectDenied:  []types.KubernetesResource{podA},
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			cluster, err := types.NewKubernetesClusterV3(types.Metadata{
				Name:   "testcluster",
				Labels: tc.clusterLabels,
			}, types.KubernetesClusterSpecV3{})
			require.NoError(t, err)
			set := NewRoleSet(tc.roles...)
			accessChecker := makeAccessCheckerWithRoleSet(set)
			allowed, denied := accessChecker.GetKubeResources(cluster)
			require.ElementsMatch(t, tc.expectAllowed, allowed, "allow list mismatch")
			require.ElementsMatch(t, tc.expectDenied, denied, "deny list mismatch")
		})
	}
}

func TestSessionRecordingMode(t *testing.T) {
	tests := map[string]struct {
		service      constants.SessionRecordingService
		expectedMode constants.SessionRecordingMode
		rolesOptions []types.RoleOptions
	}{
		"service-specific option": {
			expectedMode: constants.SessionRecordingModeBestEffort,
			service:      constants.SessionRecordingServiceSSH,
			rolesOptions: []types.RoleOptions{
				{RecordSession: &types.RecordSession{SSH: constants.SessionRecordingModeBestEffort}},
			},
		},
		"service-specific multiple roles": {
			expectedMode: constants.SessionRecordingModeBestEffort,
			service:      constants.SessionRecordingServiceSSH,
			rolesOptions: []types.RoleOptions{
				{RecordSession: &types.RecordSession{Default: constants.SessionRecordingModeStrict}},
				{RecordSession: &types.RecordSession{SSH: constants.SessionRecordingModeBestEffort}},
			},
		},
		"strict service-specific multiple roles": {
			expectedMode: constants.SessionRecordingModeStrict,
			service:      constants.SessionRecordingServiceSSH,
			rolesOptions: []types.RoleOptions{
				{RecordSession: &types.RecordSession{Default: constants.SessionRecordingModeStrict}},
				{RecordSession: &types.RecordSession{SSH: constants.SessionRecordingModeBestEffort}},
				{RecordSession: &types.RecordSession{SSH: constants.SessionRecordingModeStrict}},
			},
		},
		"strict default multiple roles": {
			expectedMode: constants.SessionRecordingModeStrict,
			service:      constants.SessionRecordingServiceSSH,
			rolesOptions: []types.RoleOptions{
				{RecordSession: &types.RecordSession{Default: constants.SessionRecordingModeBestEffort}},
				{RecordSession: &types.RecordSession{Default: constants.SessionRecordingModeBestEffort}},
				{RecordSession: &types.RecordSession{Default: constants.SessionRecordingModeStrict}},
			},
		},
		"default multiple roles": {
			expectedMode: constants.SessionRecordingModeBestEffort,
			service:      constants.SessionRecordingServiceSSH,
			rolesOptions: []types.RoleOptions{
				{RecordSession: &types.RecordSession{Default: constants.SessionRecordingModeBestEffort}},
				{RecordSession: &types.RecordSession{Default: constants.SessionRecordingModeBestEffort}},
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			roles := make([]types.Role, len(test.rolesOptions))
			for i := range roles {
				roles[i] = &types.RoleV6{
					Spec: types.RoleSpecV6{Options: test.rolesOptions[i]},
				}
			}

			roleSet := RoleSet(roles)
			require.Equal(t, test.expectedMode, roleSet.SessionRecordingMode(test.service))
		})
	}
}

func TestHostUsers_getGroups(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		test   string
		groups []string
		roles  RoleSet
		server types.Server
	}{
		{
			test:   "test exact match, one group, one role",
			groups: []string{"group"},
			roles: NewRoleSet(&types.RoleV6{
				Spec: types.RoleSpecV6{
					Options: types.RoleOptions{
						CreateHostUser: types.NewBoolOption(true),
					},
					Allow: types.RoleConditions{
						NodeLabels: types.Labels{"success": []string{"abc"}},
						HostGroups: []string{"group"},
					},
				},
			}),
			server: &types.ServerV2{
				Kind: types.KindNode,
				Metadata: types.Metadata{
					Labels: map[string]string{
						"success": "abc",
					},
				},
			},
		},
		{
			test:   "test deny on group entry",
			groups: []string{"group"},
			roles: NewRoleSet(&types.RoleV6{
				Spec: types.RoleSpecV6{
					Options: types.RoleOptions{
						CreateHostUser: types.NewBoolOption(true),
					},
					Allow: types.RoleConditions{
						NodeLabels: types.Labels{"success": []string{"abc"}},
						HostGroups: []string{"group", "groupdel"},
					},
				},
			}, &types.RoleV6{
				Spec: types.RoleSpecV6{
					Options: types.RoleOptions{
						CreateHostUser: types.NewBoolOption(true),
					},
					Deny: types.RoleConditions{
						NodeLabels: types.Labels{"success": []string{"abc"}},
						HostGroups: []string{"groupdel"},
					},
				},
			}),
			server: &types.ServerV2{
				Kind: types.KindNode,
				Metadata: types.Metadata{
					Labels: map[string]string{
						"success": "abc",
					},
				},
			},
		},
		{
			test:   "multiple roles, one no match",
			groups: []string{"group1", "group2"},
			roles: NewRoleSet(&types.RoleV6{
				Spec: types.RoleSpecV6{
					Options: types.RoleOptions{
						CreateHostUser: types.NewBoolOption(true),
					},
					Allow: types.RoleConditions{
						NodeLabels: types.Labels{"success": []string{"abc"}},
						HostGroups: []string{"group1"},
					},
				},
			}, &types.RoleV6{
				Spec: types.RoleSpecV6{
					Options: types.RoleOptions{
						CreateHostUser: types.NewBoolOption(true),
					},
					Allow: types.RoleConditions{
						NodeLabels: types.Labels{types.Wildcard: []string{types.Wildcard}},
						HostGroups: []string{"group2"},
					},
				},
			}, &types.RoleV6{
				Spec: types.RoleSpecV6{
					Allow: types.RoleConditions{
						NodeLabels: types.Labels{"fail": []string{"abc"}},
						HostGroups: []string{"notpresentgroup"},
					},
				},
			}),
			server: &types.ServerV2{
				Kind: types.KindNode,
				Metadata: types.Metadata{
					Labels: map[string]string{
						"success": "abc",
					},
				},
			},
		},
	} {
		t.Run(tc.test, func(t *testing.T) {
			accessChecker := makeAccessCheckerWithRoleSet(tc.roles)
			info, err := accessChecker.HostUsers(tc.server)
			require.NoError(t, err)
			require.Equal(t, tc.groups, info.Groups)
		})
	}
}

func TestHostUsers_HostSudoers(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		test    string
		sudoers []string
		roles   RoleSet
		server  types.Server
	}{
		{
			test:    "test exact match, one sudoer entry, one role",
			sudoers: []string{"%sudo	ALL=(ALL) ALL"},
			roles: NewRoleSet(&types.RoleV6{
				Spec: types.RoleSpecV6{
					Allow: types.RoleConditions{
						NodeLabels:  types.Labels{"success": []string{"abc"}},
						HostSudoers: []string{"%sudo	ALL=(ALL) ALL"},
					},
				},
			}),
			server: &types.ServerV2{
				Kind: types.KindNode,
				Metadata: types.Metadata{
					Labels: map[string]string{
						"success": "abc",
					},
				},
			},
		},
		{
			test:    "multiple roles, one not matching",
			sudoers: []string{"sudoers entry 1", "sudoers entry 2"},
			roles: NewRoleSet(&types.RoleV6{
				Metadata: types.Metadata{
					Name: "a",
				},
				Spec: types.RoleSpecV6{
					Allow: types.RoleConditions{
						NodeLabels:  types.Labels{"success": []string{"abc"}},
						HostSudoers: []string{"sudoers entry 1"},
					},
				},
			}, &types.RoleV6{
				Metadata: types.Metadata{
					Name: "b",
				},
				Spec: types.RoleSpecV6{
					Allow: types.RoleConditions{
						NodeLabels:  types.Labels{types.Wildcard: []string{types.Wildcard}},
						HostSudoers: []string{"sudoers entry 2"},
					},
				},
			}, &types.RoleV6{
				Spec: types.RoleSpecV6{
					Allow: types.RoleConditions{
						NodeLabels:  types.Labels{"fail": []string{"abc"}},
						HostSudoers: []string{"not present sudoers entry"},
					},
				},
			}),
			server: &types.ServerV2{
				Kind: types.KindNode,
				Metadata: types.Metadata{
					Labels: map[string]string{
						"success": "abc",
					},
				},
			},
		},
		{
			test:    "glob deny",
			sudoers: nil,
			roles: NewRoleSet(&types.RoleV6{
				Spec: types.RoleSpecV6{
					Allow: types.RoleConditions{
						NodeLabels:  types.Labels{"success": []string{"abc"}},
						HostSudoers: []string{"%sudo	ALL=(ALL) ALL"},
					},
				},
			}, &types.RoleV6{
				Spec: types.RoleSpecV6{
					Deny: types.RoleConditions{
						NodeLabels:  types.Labels{"success": []string{"abc"}},
						HostSudoers: []string{"*"},
					},
				},
			}),
			server: &types.ServerV2{
				Kind: types.KindNode,
				Metadata: types.Metadata{
					Labels: map[string]string{
						"success": "abc",
					},
				},
			},
		},
		{
			test:    "line deny",
			sudoers: []string{"%sudo	ALL=(ALL) ALL"},
			roles: NewRoleSet(&types.RoleV6{
				Spec: types.RoleSpecV6{
					Allow: types.RoleConditions{
						NodeLabels: types.Labels{"success": []string{"abc"}},
						HostSudoers: []string{
							"%sudo	ALL=(ALL) ALL",
							"removed entry",
						},
					},
				},
			}, &types.RoleV6{
				Spec: types.RoleSpecV6{
					Deny: types.RoleConditions{
						NodeLabels:  types.Labels{"success": []string{"abc"}},
						HostSudoers: []string{"removed entry"},
					},
				},
			}),
			server: &types.ServerV2{
				Kind: types.KindNode,
				Metadata: types.Metadata{
					Labels: map[string]string{
						"success": "abc",
					},
				},
			},
		},
		{
			test:    "multiple roles, order preserved by role name",
			sudoers: []string{"sudoers entry 1", "sudoers entry 2", "sudoers entry 3", "sudoers entry 4"},
			roles: NewRoleSet(&types.RoleV6{
				Metadata: types.Metadata{
					Name: "a",
				},
				Spec: types.RoleSpecV6{
					Allow: types.RoleConditions{
						NodeLabels:  types.Labels{"success": []string{"abc"}},
						HostSudoers: []string{"sudoers entry 1"},
					},
				},
			}, &types.RoleV6{
				Metadata: types.Metadata{
					Name: "c",
				},
				Spec: types.RoleSpecV6{
					Allow: types.RoleConditions{
						NodeLabels:  types.Labels{types.Wildcard: []string{types.Wildcard}},
						HostSudoers: []string{"sudoers entry 4", "sudoers entry 1"},
					},
				},
			}, &types.RoleV6{
				Metadata: types.Metadata{
					Name: "b",
				},
				Spec: types.RoleSpecV6{
					Allow: types.RoleConditions{
						NodeLabels:  types.Labels{types.Wildcard: []string{types.Wildcard}},
						HostSudoers: []string{"sudoers entry 2", "sudoers entry 3"},
					},
				},
			}),
			server: &types.ServerV2{
				Kind: types.KindNode,
				Metadata: types.Metadata{
					Labels: map[string]string{
						"success": "abc",
					},
				},
			},
		},
		{
			test:    "duplication handled",
			sudoers: []string{"sudoers entry 2"},
			roles: NewRoleSet(&types.RoleV6{
				Metadata: types.Metadata{
					Name: "a",
				},
				Spec: types.RoleSpecV6{
					Allow: types.RoleConditions{
						NodeLabels:  types.Labels{"success": []string{"abc"}},
						HostSudoers: []string{"sudoers entry 1"},
					},
				},
			}, &types.RoleV6{ // DENY sudoers entry 1
				Metadata: types.Metadata{
					Name: "d",
				},
				Spec: types.RoleSpecV6{
					Deny: types.RoleConditions{
						NodeLabels:  types.Labels{"success": []string{"abc"}},
						HostSudoers: []string{"sudoers entry 1"},
					},
				},
			}, &types.RoleV6{ // duplicate sudoers entry 1 case also gets removed
				Metadata: types.Metadata{
					Name: "c",
				},
				Spec: types.RoleSpecV6{
					Allow: types.RoleConditions{
						NodeLabels:  types.Labels{types.Wildcard: []string{types.Wildcard}},
						HostSudoers: []string{"sudoers entry 1", "sudoers entry 2"},
					},
				},
			}),
			server: &types.ServerV2{
				Kind: types.KindNode,
				Metadata: types.Metadata{
					Labels: map[string]string{
						"success": "abc",
					},
				},
			},
		},
	} {
		t.Run(tc.test, func(t *testing.T) {
			accessChecker := makeAccessCheckerWithRoleSet(tc.roles)
			info, err := accessChecker.HostSudoers(tc.server)
			require.NoError(t, err)
			require.Equal(t, tc.sudoers, info)
		})
	}
}

func TestHostUsers_CanCreateHostUser(t *testing.T) {
	t.Parallel()
	type testCase struct {
		test         string
		canCreate    bool
		roles        RoleSet
		server       types.Server
		expectedMode types.CreateHostUserMode
	}

	createDefaultTCWithMode := func(name string, canCreate bool, mode types.CreateHostUserMode) testCase {
		return testCase{
			test:         name,
			canCreate:    canCreate,
			expectedMode: mode,
			roles: NewRoleSet(&types.RoleV6{
				Spec: types.RoleSpecV6{
					Options: types.RoleOptions{
						CreateHostUserMode: mode,
					},
					Allow: types.RoleConditions{
						NodeLabels: types.Labels{"success": []string{"abc"}},
					},
				},
			}),
			server: &types.ServerV2{
				Kind: types.KindNode,
				Metadata: types.Metadata{
					Labels: map[string]string{
						"success": "abc",
					},
				},
			},
		}
	}

	for _, tc := range []testCase{
		{
			test:         "test exact match, one role, can create",
			canCreate:    true,
			expectedMode: types.CreateHostUserMode_HOST_USER_MODE_KEEP,
			roles: NewRoleSet(&types.RoleV6{
				Spec: types.RoleSpecV6{
					Options: types.RoleOptions{
						CreateHostUser: types.NewBoolOption(true),
					},
					Allow: types.RoleConditions{
						NodeLabels: types.Labels{"success": []string{"abc"}},
					},
				},
			}),
			server: &types.ServerV2{
				Kind: types.KindNode,
				Metadata: types.Metadata{
					Labels: map[string]string{
						"success": "abc",
					},
				},
			},
		},
		{
			test:         "test two roles, 1 exact match, one can create",
			canCreate:    false,
			expectedMode: types.CreateHostUserMode_HOST_USER_MODE_KEEP,
			roles: NewRoleSet(&types.RoleV6{
				Spec: types.RoleSpecV6{
					Options: types.RoleOptions{
						CreateHostUser: types.NewBoolOption(true),
					},
					Allow: types.RoleConditions{
						NodeLabels: types.Labels{"success": []string{"abc"}},
					},
				},
			}, &types.RoleV6{
				Spec: types.RoleSpecV6{
					Options: types.RoleOptions{
						CreateHostUser: types.NewBoolOption(false),
					},
					Allow: types.RoleConditions{
						NodeLabelsExpression: `labels["success"] == "abc"`,
					},
				},
			}),
			server: &types.ServerV2{
				Kind: types.KindNode,
				Metadata: types.Metadata{
					Labels: map[string]string{
						"success": "abc",
					},
				},
			},
		},
		{
			test:         "test three roles, 2 exact match, both can create",
			canCreate:    true,
			expectedMode: types.CreateHostUserMode_HOST_USER_MODE_KEEP,
			roles: NewRoleSet(&types.RoleV6{
				Spec: types.RoleSpecV6{
					Options: types.RoleOptions{
						CreateHostUser: types.NewBoolOption(true),
					},
					Allow: types.RoleConditions{
						NodeLabels: types.Labels{"success": []string{"abc"}},
					},
				},
			}, &types.RoleV6{
				Spec: types.RoleSpecV6{
					Options: types.RoleOptions{
						CreateHostUser: types.NewBoolOption(true),
					},
					Allow: types.RoleConditions{
						NodeLabelsExpression: `labels["success"] == "abc"`,
					},
				},
			}, &types.RoleV6{
				Spec: types.RoleSpecV6{
					Options: types.RoleOptions{
						CreateHostUser: types.NewBoolOption(false),
					},
					Allow: types.RoleConditions{
						NodeLabels: types.Labels{"unmatched": []string{"abc"}},
					},
				},
			}),
			server: &types.ServerV2{
				Kind: types.KindNode,
				Metadata: types.Metadata{
					Labels: map[string]string{
						"success": "abc",
					},
				},
			},
		},
		{
			test:      "test cant create when create host user is nil",
			canCreate: false,
			roles: NewRoleSet(&types.RoleV6{
				Spec: types.RoleSpecV6{
					Options: types.RoleOptions{
						CreateHostUser: nil,
					},
					Allow: types.RoleConditions{
						NodeLabels: types.Labels{"success": []string{"abc"}},
					},
				},
			}),
			server: &types.ServerV2{
				Kind: types.KindNode,
				Metadata: types.Metadata{
					Labels: map[string]string{
						"success": "abc",
					},
				},
			},
		},
		createDefaultTCWithMode(
			"test cant create when create host user mode is off",
			false,
			types.CreateHostUserMode_HOST_USER_MODE_OFF,
		),
		createDefaultTCWithMode(
			"test can create when create host user mode is insecure-drop",
			true,
			types.CreateHostUserMode_HOST_USER_MODE_INSECURE_DROP,
		),
		createDefaultTCWithMode(
			"test can create when create host user mode is keep",
			true,
			types.CreateHostUserMode_HOST_USER_MODE_KEEP,
		),
		{
			test:      "test three roles, 3 exact match, one off",
			canCreate: false,
			roles: NewRoleSet(&types.RoleV6{
				Spec: types.RoleSpecV6{
					Options: types.RoleOptions{
						CreateHostUserMode: types.CreateHostUserMode_HOST_USER_MODE_KEEP,
					},
					Allow: types.RoleConditions{
						NodeLabels: types.Labels{"success": []string{"abc"}},
					},
				},
			}, &types.RoleV6{
				Spec: types.RoleSpecV6{
					Options: types.RoleOptions{
						CreateHostUserMode: types.CreateHostUserMode_HOST_USER_MODE_OFF,
					},
					Allow: types.RoleConditions{
						NodeLabels: types.Labels{"success": []string{"abc"}},
					},
				},
			}, &types.RoleV6{
				Spec: types.RoleSpecV6{
					Options: types.RoleOptions{
						CreateHostUserMode: types.CreateHostUserMode_HOST_USER_MODE_INSECURE_DROP,
					},
					Allow: types.RoleConditions{
						NodeLabels: types.Labels{"success": []string{"abc"}},
					},
				},
			}),
			server: &types.ServerV2{
				Metadata: types.Metadata{
					Labels: map[string]string{
						"success": "abc",
					},
				},
			},
		},
		{
			test:         "test three roles, 3 exact match, mode defaults to keep",
			canCreate:    true,
			expectedMode: types.CreateHostUserMode_HOST_USER_MODE_KEEP,
			roles: NewRoleSet(&types.RoleV6{
				Spec: types.RoleSpecV6{
					Options: types.RoleOptions{
						CreateHostUserMode: types.CreateHostUserMode_HOST_USER_MODE_KEEP,
					},
					Allow: types.RoleConditions{
						NodeLabels: types.Labels{"success": []string{"abc"}},
					},
				},
			}, &types.RoleV6{
				Spec: types.RoleSpecV6{
					Options: types.RoleOptions{
						CreateHostUserMode: types.CreateHostUserMode_HOST_USER_MODE_KEEP,
					},
					Allow: types.RoleConditions{
						NodeLabels: types.Labels{"success": []string{"abc"}},
					},
				},
			}, &types.RoleV6{
				Spec: types.RoleSpecV6{
					Options: types.RoleOptions{
						CreateHostUserMode: types.CreateHostUserMode_HOST_USER_MODE_KEEP,
					},
					Allow: types.RoleConditions{
						NodeLabels: types.Labels{"success": []string{"abc"}},
					},
				},
			}),
			server: &types.ServerV2{
				Kind: types.KindNode,
				Metadata: types.Metadata{
					Labels: map[string]string{
						"success": "abc",
					},
				},
			},
		},
	} {
		t.Run(tc.test, func(t *testing.T) {
			accessChecker := makeAccessCheckerWithRoleSet(tc.roles)
			info, err := accessChecker.HostUsers(tc.server)
			require.Equal(t, tc.canCreate, err == nil && info != nil)
			if tc.canCreate {
				require.Equal(t, tc.expectedMode, info.Mode)
			}
		})
	}
}

type mockCurrentUserRoleGetter struct {
	getCurrentUserError error
	currentUser         types.User
	nameToRole          map[string]types.Role
}

func (m mockCurrentUserRoleGetter) GetCurrentUser(ctx context.Context) (types.User, error) {
	if m.getCurrentUserError != nil {
		return nil, trace.Wrap(m.getCurrentUserError)
	}
	if m.currentUser != nil {
		return m.currentUser, nil
	}
	return nil, trace.NotFound("currentUser not set")
}

func (m mockCurrentUserRoleGetter) GetCurrentUserRoles(ctx context.Context) ([]types.Role, error) {
	var roles []types.Role
	for _, role := range m.nameToRole {
		roles = append(roles, role)
	}
	return roles, nil
}

func (m mockCurrentUserRoleGetter) GetRole(ctx context.Context, name string) (types.Role, error) {
	if role, ok := m.nameToRole[name]; ok {
		return role, nil
	}
	return nil, trace.NotFound("role not found: %v", name)
}

type mockCurrentUser struct {
	types.User
	roles  []string
	traits wrappers.Traits
}

func (u mockCurrentUser) GetRoles() []string {
	return u.roles
}

func (u mockCurrentUser) GetTraits() map[string][]string {
	return u.traits
}

func (u mockCurrentUser) GetName() string {
	return "mockCurrentUser"
}

func (u mockCurrentUser) toRemoteUserFromCluster(localClusterName string) types.User {
	return mockRemoteUser{
		mockCurrentUser:  u,
		localClusterName: localClusterName,
	}
}

type mockRemoteUser struct {
	mockCurrentUser
	localClusterName string
}

// GetName returns the username from the remote cluster's view.
func (u mockRemoteUser) GetName() string {
	return UsernameForRemoteCluster(u.mockCurrentUser.GetName(), u.localClusterName)
}

func TestNewAccessCheckerForRemoteCluster(t *testing.T) {
	user := mockCurrentUser{
		roles: []string{"dev", "admin"},
		traits: map[string][]string{
			"logins": {"currentUserTraitLogin"},
		},
	}

	devRole := newRole(func(r *types.RoleV6) {
		r.Metadata.Name = "dev"
		r.Spec.Allow.Logins = []string{"{{internal.logins}}"}
	})
	adminRole := newRole(func(r *types.RoleV6) {
		r.Metadata.Name = "admin"
	})

	currentUserRoleGetter := mockCurrentUserRoleGetter{
		nameToRole: map[string]types.Role{
			"dev":   devRole,
			"admin": adminRole,
		},
		currentUser: user.toRemoteUserFromCluster("localCluster"),
	}

	localAccessInfo := AccessInfoFromUserState(user)
	require.Equal(t, "mockCurrentUser", localAccessInfo.Username)
	accessChecker, err := NewAccessCheckerForRemoteCluster(context.Background(), localAccessInfo, "remoteCluster", currentUserRoleGetter)
	require.NoError(t, err)

	// After sort: "admin","default-implicit-role","dev"
	roles := accessChecker.Roles()
	slices.SortFunc(roles, func(a, b types.Role) int {
		return cmp.Compare(a.GetName(), b.GetName())
	})
	require.Len(t, roles, 3)
	require.Contains(t, roles, devRole, "devRole not found in roleSet")
	require.Contains(t, roles, adminRole, "adminRole not found in roleSet")
	require.Equal(t, []string{"currentUserTraitLogin"}, roles[2].GetLogins(types.Allow))

	mustHaveUsername(t, accessChecker, "remote-mockCurrentUser-localCluster")
}

func mustHaveUsername(t *testing.T, access AccessChecker, wantUsername string) {
	t.Helper()

	accessImpl, ok := access.(*accessChecker)
	require.True(t, ok)
	require.Equal(t, wantUsername, accessImpl.info.Username)
}

func TestRoleSet_GetAccessState(t *testing.T) {
	testCases := []struct {
		name                   string
		roleMFARequireTypes    []types.RequireMFAType
		authPrefMFARequireType types.RequireMFAType
		expectState            AccessState
	}{
		{
			name: "empty role set and auth pref requirement",
			expectState: AccessState{
				MFARequired: MFARequiredNever,
			},
		},
		{
			name: "no roles require mfa, auth pref doesn't require mfa",
			roleMFARequireTypes: []types.RequireMFAType{
				types.RequireMFAType_OFF,
				types.RequireMFAType_OFF,
			},
			authPrefMFARequireType: types.RequireMFAType_OFF,
			expectState: AccessState{
				MFARequired: MFARequiredNever,
			},
		},
		{
			name: "no roles require mfa, auth pref requires mfa",
			roleMFARequireTypes: []types.RequireMFAType{
				types.RequireMFAType_OFF,
				types.RequireMFAType_OFF,
			},
			authPrefMFARequireType: types.RequireMFAType_SESSION,
			expectState: AccessState{
				MFARequired: MFARequiredAlways,
			},
		},
		{
			name: "some roles require mfa, auth pref doesn't require mfa",
			roleMFARequireTypes: []types.RequireMFAType{
				types.RequireMFAType_OFF,
				types.RequireMFAType_SESSION,
			},
			authPrefMFARequireType: types.RequireMFAType_OFF,
			expectState: AccessState{
				MFARequired: MFARequiredPerRole,
			},
		},
		{
			name: "some roles require mfa, auth pref requires mfa",
			roleMFARequireTypes: []types.RequireMFAType{
				types.RequireMFAType_OFF,
				types.RequireMFAType_SESSION,
			},
			authPrefMFARequireType: types.RequireMFAType_SESSION,
			expectState: AccessState{
				MFARequired: MFARequiredAlways,
			},
		},
		{
			name: "all roles require mfa, auth pref requires mfa",
			roleMFARequireTypes: []types.RequireMFAType{
				types.RequireMFAType_SESSION,
				types.RequireMFAType_SESSION,
			},
			authPrefMFARequireType: types.RequireMFAType_SESSION,
			expectState: AccessState{
				MFARequired: MFARequiredAlways,
			},
		},
		{
			name: "all roles require mfa, auth pref doesn't require mfa",
			roleMFARequireTypes: []types.RequireMFAType{
				types.RequireMFAType_SESSION,
				types.RequireMFAType_SESSION,
			},
			authPrefMFARequireType: types.RequireMFAType_OFF,
			expectState: AccessState{
				MFARequired: MFARequiredAlways,
			},
		},
		{
			name: "auth pref requires hardware key",
			roleMFARequireTypes: []types.RequireMFAType{
				types.RequireMFAType_OFF,
			},
			authPrefMFARequireType: types.RequireMFAType_SESSION_AND_HARDWARE_KEY,
			expectState: AccessState{
				MFARequired: MFARequiredAlways,
			},
		},
		{
			name: "auth pref requires hardware key touch",
			roleMFARequireTypes: []types.RequireMFAType{
				types.RequireMFAType_OFF,
			},
			authPrefMFARequireType: types.RequireMFAType_HARDWARE_KEY_TOUCH,
			expectState: AccessState{
				MFARequired: MFARequiredAlways,
			},
		},
		{
			name: "role requires hardware key",
			roleMFARequireTypes: []types.RequireMFAType{
				types.RequireMFAType_SESSION_AND_HARDWARE_KEY,
			},
			authPrefMFARequireType: types.RequireMFAType_OFF,
			expectState: AccessState{
				MFARequired: MFARequiredAlways,
			},
		},
		{
			name: "role requires hardware key touch",
			roleMFARequireTypes: []types.RequireMFAType{
				types.RequireMFAType_HARDWARE_KEY_TOUCH,
			},
			authPrefMFARequireType: types.RequireMFAType_OFF,
			expectState: AccessState{
				MFARequired: MFARequiredAlways,
			},
		},
	}
	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			authPref, err := types.NewAuthPreference(types.AuthPreferenceSpecV2{
				RequireMFAType: tc.authPrefMFARequireType,
			})
			require.NoError(t, err, "NewAuthPreference failed")

			var set RoleSet
			for _, roleRequirement := range tc.roleMFARequireTypes {
				set = append(set, newRole(func(r *types.RoleV6) {
					r.Spec.Options.RequireMFAType = roleRequirement
				}))
			}
			require.Equal(t, tc.expectState, set.GetAccessState(authPref))
		})
	}
}

func TestAzureIdentityMatcher_Match(t *testing.T) {
	tests := []struct {
		name       string
		identities []string

		role      types.Role
		matchType types.RoleConditionType

		wantMatched []string
	}{
		{
			name:       "allow ignores wildcard",
			identities: []string{"foo", "BAR", "baz"},
			role: newRole(func(r *types.RoleV6) {
				r.Spec.Allow.AzureIdentities = []string{"*", "bar", "baz"}
			}),
			matchType:   types.Allow,
			wantMatched: []string{"BAR", "baz"},
		},
		{
			name:       "deny matches wildcard",
			identities: []string{"FoO", "BAr", "baz"},
			role: newRole(func(r *types.RoleV6) {
				r.Spec.Deny.AzureIdentities = []string{"*", "bar", "baz"}
			}),
			matchType:   types.Deny,
			wantMatched: []string{"FoO", "BAr", "baz"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var matched []string
			for _, identity := range tt.identities {
				m := &AzureIdentityMatcher{Identity: identity}
				if ok, _ := m.Match(tt.role, tt.matchType); ok {
					matched = append(matched, identity)
				}
			}
			require.Equal(t, tt.wantMatched, matched)
		})
	}
}

func TestMatchAzureIdentity(t *testing.T) {
	tests := []struct {
		name string

		identities    []string
		identity      string
		matchWildcard bool

		wantMatch     bool
		wantMatchType string
	}{
		{
			name: "allow exact match",

			identities:    []string{"foo", "bar", "baz"},
			identity:      "bar",
			matchWildcard: false,

			wantMatch:     true,
			wantMatchType: "element matched",
		},
		{
			name: "allow case insensitive match",

			identities:    []string{"foo", "bar", "baz"},
			identity:      "BAR",
			matchWildcard: false,

			wantMatch:     true,
			wantMatchType: "element matched",
		},
		{
			name: "allow wildcard mismatch",

			identities:    []string{"foo", "bar", "*"},
			identity:      "baz",
			matchWildcard: false,

			wantMatch:     false,
			wantMatchType: "no match, role selectors [foo bar *], identity: baz",
		},
		{
			name: "deny exact match",

			identities:    []string{"foo", "bar", "baz"},
			identity:      "bar",
			matchWildcard: true,

			wantMatch:     true,
			wantMatchType: "element matched",
		},
		{
			name: "deny case insensitive match",

			identities:    []string{"foo", "bar", "baz"},
			identity:      "BAZ",
			matchWildcard: true,

			wantMatch:     true,
			wantMatchType: "element matched",
		},
		{
			name: "deny wildcard match",

			identities:    []string{"foo", "bar", "*"},
			identity:      "baz",
			matchWildcard: true,

			wantMatch:     true,
			wantMatchType: "wildcard matched",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotMatch, gotMatchType := MatchAzureIdentity(tt.identities, tt.identity, tt.matchWildcard)
			require.Equal(t, tt.wantMatch, gotMatch)
			require.Equal(t, tt.wantMatchType, gotMatchType)
		})
	}
}

func TestMatchValidAzureIdentity(t *testing.T) {
	tests := []struct {
		name                  string
		identity              string
		valid                 bool
		ignoreParseResourceID bool
	}{
		{
			name:                  "wildcard",
			identity:              "*",
			valid:                 true,
			ignoreParseResourceID: true,
		},
		{
			name:     "correct format",
			identity: "/subscriptions/1020304050607-cafe-8090-a0b0c0d0e0f0/resourceGroups/example-resource-group/providers/Microsoft.ManagedIdentity/userAssignedIdentities/teleport-azure",
			valid:    true,
		},
		{
			name:     "correct format with underscore",
			identity: "/subscriptions/1020304050607-cafe-8090-a0b0c0d0e0f0/resourceGroups/az-cli-access_group/providers/Microsoft.ManagedIdentity/userAssignedIdentities/teleport-azure_under",
			valid:    true,
		},
		{
			name:     "correct format, case insensitive match",
			identity: "/SUBscriptions/0000000000000-0000-CAFE-A0B0C0D0E0F0/RESOURCEGroups/EXAMPLE-resource-group/provIders/microsoft.managedidentity/userassignedidentities/Tele10329azure",
			valid:    true,
		},
		{
			name:     "invalid format # 1",
			identity: "/subscriptions/0000000000000-XXXX-XXXX-000000000000/resourceGroups/example-resource-group/providers/Microsoft.ManagedIdentity/userAssignedIdentities/teleport-azure",
			valid:    false,
		},
		{
			name:     "invalid format # 2",
			identity: "/subscriptions/0000000000000-0000-0000-000000000000/resourceGroups/example resource group/providers/Microsoft.ManagedIdentity/userAssignedIdentities/teleport-azure",
			valid:    false,
		},
		{
			name:     "invalid format # 3",
			identity: "/subscriptions/0000000000000-0000-0000-000000000000/resourceGroups/example-resource-group/providers/Microsoft.ManagedIdentity/userAssignedIdentities/teleport azure",
			valid:    false,
		},
		{
			name:     "invalid format # 4",
			identity: "/subscriptions/0000000000000-0000-0000-000000000000/resourceGroups/example-resource-group/providers/Microsoft.ManagedIdentity/userAssignedIdentities/",
			valid:    false,
		},
		{
			name:     "invalid format # 5",
			identity: "/subscriptions/0000000000000-0000-0000-000000000000/resourceGroups/example-resource-group/providers/Microsoft.ManagedIdentity/userAssignedIdentities",
			valid:    false,
		},
		{
			name:     "invalid format # 6",
			identity: "whatever /subscriptions/0000000000000-0000-0000-000000000000/resourceGroups/example-resource-group/providers/Microsoft.ManagedIdentity/userAssignedIdentities/foo",
			valid:    false,
		},
		{
			name:     "invalid format # 7",
			identity: "///subscriptions///1020304050607-cafe-8090-a0b0c0d0e0f0///resourceGroups/example-resource-group/providers/Microsoft.ManagedIdentity/userAssignedIdentities/teleport-azure",
			valid:    false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.valid, MatchValidAzureIdentity(tt.identity))

			if tt.ignoreParseResourceID == false {
				// if it ParseResourceID returns an error, we expect MatchValidAzureIdentity to do the same.
				_, err := arm.ParseResourceID(tt.identity)
				if err != nil {
					require.False(t, MatchValidAzureIdentity(tt.identity))
				}
			}
		})
	}
}

func TestGCPServiceAccountMatcher_Match(t *testing.T) {
	tests := []struct {
		name     string
		accounts []string

		role      types.Role
		matchType types.RoleConditionType

		wantMatched []string
	}{
		{
			name:     "allow ignores wildcard",
			accounts: []string{"foo", "bar", "baz"},
			role: newRole(func(r *types.RoleV6) {
				r.Spec.Allow.GCPServiceAccounts = []string{"*", "bar", "baz"}
			}),
			matchType:   types.Allow,
			wantMatched: []string{"bar", "baz"},
		},
		{
			name:     "deny matches wildcard",
			accounts: []string{"FoO", "BAr", "baz"},
			role: newRole(func(r *types.RoleV6) {
				r.Spec.Deny.GCPServiceAccounts = []string{"*"}
			}),
			matchType:   types.Deny,
			wantMatched: []string{"FoO", "BAr", "baz"},
		},
		{
			name:     "non-wildcard deny matches",
			accounts: []string{"foo", "bar", "baz"},
			role: newRole(func(r *types.RoleV6) {
				r.Spec.Deny.GCPServiceAccounts = []string{"foo", "bar", "admin"}
			}),
			matchType:   types.Deny,
			wantMatched: []string{"foo", "bar"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var matched []string
			for _, account := range tt.accounts {
				m := &GCPServiceAccountMatcher{ServiceAccount: account}
				if ok, _ := m.Match(tt.role, tt.matchType); ok {
					matched = append(matched, account)
				}
			}
			require.Equal(t, tt.wantMatched, matched)
		})
	}
}

func TestMatchGCPServiceAccount(t *testing.T) {
	tests := []struct {
		name string

		accounts      []string
		account       string
		matchWildcard bool

		wantMatch     bool
		wantMatchType string
	}{
		{
			name: "allow exact match",

			accounts:      []string{"foo", "bar", "baz"},
			account:       "bar",
			matchWildcard: false,

			wantMatch:     true,
			wantMatchType: "element matched",
		},
		{
			name: "wildcard in allow doesn't work",

			accounts:      []string{"foo", "bar", "*"},
			account:       "baz",
			matchWildcard: false,

			wantMatch:     false,
			wantMatchType: "no match, role selectors [foo bar *], identity: baz",
		},
		{
			name: "deny exact match",

			accounts:      []string{"foo", "bar", "baz"},
			account:       "bar",
			matchWildcard: true,

			wantMatch:     true,
			wantMatchType: "element matched",
		},
		{
			name: "wildcard in deny works",

			accounts:      []string{"foo", "bar", "*"},
			account:       "baz",
			matchWildcard: true,

			wantMatch:     true,
			wantMatchType: "wildcard matched",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotMatch, gotMatchType := MatchGCPServiceAccount(tt.accounts, tt.account, tt.matchWildcard)
			require.Equal(t, tt.wantMatch, gotMatch)
			require.Equal(t, tt.wantMatchType, gotMatchType)
		})
	}
}

func TestKubeResourcesMatcher(t *testing.T) {
	defaultRole, err := types.NewRole("kube",
		types.RoleSpecV6{
			Allow: types.RoleConditions{
				KubernetesResources: []types.KubernetesResource{
					{
						Kind:      types.KindKubePod,
						Namespace: "dev",
						Name:      types.Wildcard,
					},
					{
						Kind:      types.KindKubePod,
						Namespace: "default",
						Name:      "nginx-*",
					},
				},
			},
			Deny: types.RoleConditions{
				KubernetesResources: []types.KubernetesResource{
					{
						Kind:      types.KindKubePod,
						Namespace: "default",
						Name:      "restricted",
					},
				},
			},
		},
	)
	require.NoError(t, err)

	prodRole, err := types.NewRole("kube2",
		types.RoleSpecV6{
			Allow: types.RoleConditions{
				KubernetesResources: []types.KubernetesResource{
					{
						Kind:      types.KindKubePod,
						Namespace: "prod",
						Name:      "pod",
					},
				},
			},
		},
	)
	require.NoError(t, err)

	invalidRole, err := types.NewRole("kube3",
		types.RoleSpecV6{
			Allow: types.RoleConditions{
				KubernetesResources: []types.KubernetesResource{
					{
						Kind:      types.KindKubePod,
						Namespace: `^[($`,
						Name:      `^[($`,
					},
				},
			},
		},
	)
	require.NoError(t, err)

	type args struct {
		resources []types.KubernetesResource
		roles     []types.Role
		cond      types.RoleConditionType
	}
	tests := []struct {
		name               string
		args               args
		wantMatch          []bool
		assertErr          require.ErrorAssertionFunc
		unmatchedResources []string
	}{
		{
			name: "user requests a valid subset of pods for defaultRole",
			args: args{
				roles: []types.Role{defaultRole},
				cond:  types.Allow,
				resources: []types.KubernetesResource{
					{
						Kind:      types.KindKubePod,
						Namespace: "dev",
						Name:      "pod",
					},
					{
						Kind:      types.KindKubePod,
						Namespace: "default",
						Name:      "nginx-*",
					},
				},
			},
			wantMatch:          boolsToSlice(true),
			assertErr:          require.NoError,
			unmatchedResources: []string{},
		},
		{
			name: "user requests a valid and invalid pod for role defaultRole",
			args: args{
				roles: []types.Role{defaultRole},
				cond:  types.Allow,
				resources: []types.KubernetesResource{
					{
						Kind:      types.KindKubePod,
						Namespace: "dev",
						Name:      "pod",
					},
					{
						Kind:      types.KindKubePod,
						Namespace: "default",
						Name:      "nginx*",
					},
				},
			},
			// returns true because the first resource matched but the request will fail
			// because unmatchedResources is not empty.
			wantMatch:          boolsToSlice(true),
			assertErr:          require.NoError,
			unmatchedResources: []string{"pod/default/nginx*"},
		},
		{
			name: "user requests a valid subset of pods but distributed across two roles",
			args: args{
				roles: []types.Role{defaultRole, prodRole},
				cond:  types.Allow,
				resources: []types.KubernetesResource{
					{
						Kind:      types.KindKubePod,
						Namespace: "dev",
						Name:      "pod",
					},
					{
						Kind:      types.KindKubePod,
						Namespace: "prod",
						Name:      "pod",
					},
				},
			},
			wantMatch:          boolsToSlice(true, true),
			assertErr:          require.NoError,
			unmatchedResources: []string{},
		},
		{
			name: "user requests a pod that does not match any role",
			args: args{
				roles: []types.Role{defaultRole, prodRole},
				cond:  types.Allow,
				resources: []types.KubernetesResource{
					{
						Kind:      types.KindKubePod,
						Namespace: "default",
						Name:      "pod",
					},
				},
			},
			wantMatch:          boolsToSlice(false, false),
			assertErr:          require.NoError,
			unmatchedResources: []string{"pod/default/pod"},
		},
		{
			name: "user requests a denied pod",
			args: args{
				roles: []types.Role{defaultRole},
				cond:  types.Deny,
				resources: []types.KubernetesResource{
					{
						Kind:      types.KindKubePod,
						Namespace: "default",
						Name:      "restricted",
					},
				},
			},
			wantMatch:          boolsToSlice(true),
			assertErr:          require.NoError,
			unmatchedResources: []string{},
		},
		{
			name: "user requests a role with wrong regex",
			args: args{
				roles: []types.Role{invalidRole},
				cond:  types.Allow,
				resources: []types.KubernetesResource{
					{
						Kind:      types.KindKubePod,
						Namespace: "default",
						Name:      "restricted",
					},
				},
			},
			wantMatch:          boolsToSlice(false),
			assertErr:          require.Error,
			unmatchedResources: []string{"pod/default/restricted"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matcher := NewKubeResourcesMatcher(tt.args.resources)
			// Verify each role independently. If a resource matches the role, matched
			// is true. Later, after analyzing all roles we verify the resources that
			// missed the match.
			for i, role := range tt.args.roles {
				matched, err := matcher.Match(role, tt.args.cond)
				require.Equal(t, tt.wantMatch[i], matched)
				tt.assertErr(t, err)
			}
			unmatched := matcher.Unmatched()
			sort.Strings(unmatched)
			require.Equal(t, tt.unmatchedResources, unmatched)
		})
	}
}

func boolsToSlice(v ...bool) []bool {
	return v
}

func TestCheckAccessWithLabelExpressions(t *testing.T) {
	t.Parallel()

	resources := []types.ResourceWithLabels{
		&types.ServerV2{Kind: types.KindNode},
		&types.KubernetesClusterV3{Kind: types.KindKubernetesCluster},
		&types.AppV3{Kind: types.KindApp},
		&types.DatabaseV3{Kind: types.KindDatabase},
		&types.DatabaseServiceV1{ResourceHeader: types.ResourceHeader{Kind: types.KindDatabaseService}},
		&types.WindowsDesktopV3{ResourceHeader: types.ResourceHeader{Kind: types.KindWindowsDesktop}},
		&types.WindowsDesktopServiceV3{ResourceHeader: types.ResourceHeader{Kind: types.KindWindowsDesktopService}},
		&types.UserGroupV1{ResourceHeader: types.ResourceHeader{Kind: types.KindUserGroup}},
	}
	for _, r := range resources {
		r.SetStaticLabels(map[string]string{"env": "prod"})
	}
	// remoteCluster doesn't implement ResourceWithLabels and access is checked
	// with CheckAccessToRemoteCluster instead of checkAccess
	remoteCluster := &types.RemoteClusterV3{
		Kind: types.KindRemoteCluster,
		Metadata: types.Metadata{
			Labels: map[string]string{"env": "prod"},
		},
	}

	type option string
	const (
		unset   option = "unset"
		match   option = "match"
		nomatch option = "nomatch"
	)
	allOptions := []option{unset, match, nomatch}

	type testcase struct {
		allowLabels, allowLabelsExpression, denyLabels, denyLabelsExpression option
	}
	var testcases []testcase
	for _, al := range allOptions {
		for _, ae := range allOptions {
			for _, dl := range allOptions {
				for _, de := range allOptions {
					testcases = append(testcases, testcase{al, ae, dl, de})
				}
			}
		}
	}

	matchLabels := types.Labels{"env": {"prod"}}
	noMatchLabels := types.Labels{"env": {"staging"}}
	matchExpression := `contains(user.spec.traits["allow-env"], labels["env"])`
	noMatchExpression := `!contains(user.spec.traits["allow-env"], labels["env"])`
	labelsForOption := func(o option) types.Labels {
		switch o {
		case match:
			return matchLabels
		case nomatch:
			return noMatchLabels
		}
		return nil
	}
	expressionForOption := func(o option) string {
		switch o {
		case match:
			return matchExpression
		case nomatch:
			return noMatchExpression
		}
		return ""
	}
	makeRole := func(tc testcase, kind string) types.Role {
		role, err := types.NewRole("rolename", types.RoleSpecV6{})
		require.NoError(t, err)
		require.NoError(t, role.SetLabelMatchers(types.Allow, kind, types.LabelMatchers{
			Labels:     labelsForOption(tc.allowLabels),
			Expression: expressionForOption(tc.allowLabelsExpression),
		}))
		require.NoError(t, role.SetLabelMatchers(types.Deny, kind, types.LabelMatchers{
			Labels:     labelsForOption(tc.denyLabels),
			Expression: expressionForOption(tc.denyLabelsExpression),
		}))
		return role
	}

	expectDenied := func(tc testcase) bool {
		return tc.denyLabels == match ||
			tc.denyLabelsExpression == match ||
			tc.allowLabels == nomatch ||
			tc.allowLabelsExpression == nomatch ||
			(tc.allowLabels == unset && tc.allowLabelsExpression == unset)
	}

	for _, resource := range resources {
		resource := resource
		t.Run(resource.GetKind(), func(t *testing.T) {
			t.Parallel()
			for _, tc := range testcases {
				t.Run(fmt.Sprint(tc), func(t *testing.T) {
					role := makeRole(tc, resource.GetKind())
					rs := NewRoleSet(role)
					accessInfo := &AccessInfo{
						Roles: []string{role.GetName()},
						Traits: wrappers.Traits{
							"allow-env": {"prod"},
						},
					}
					accessChecker := NewAccessCheckerWithRoleSet(accessInfo, "testcluster", rs)
					err := accessChecker.CheckAccess(resource, AccessState{})
					if expectDenied(tc) {
						require.True(t, trace.IsAccessDenied(err),
							"expected AccessDenied error, got: %v", err)
						return
					}
					require.NoError(t, err, trace.DebugReport(err))
				})
			}
		})
	}
	t.Run("remote cluster", func(t *testing.T) {
		t.Parallel()
		for _, tc := range testcases {
			t.Run(fmt.Sprint(tc), func(t *testing.T) {
				role := makeRole(tc, types.KindRemoteCluster)
				rs := NewRoleSet(role)
				accessInfo := &AccessInfo{
					Roles: []string{role.GetName()},
					Traits: wrappers.Traits{
						"allow-env": {"prod"},
					},
				}
				accessChecker := NewAccessCheckerWithRoleSet(accessInfo, "testcluster", rs)
				err := accessChecker.CheckAccessToRemoteCluster(remoteCluster)
				if expectDenied(tc) {
					require.True(t, trace.IsAccessDenied(err),
						"expected AccessDenied error, got: %v", err)
					return
				}
				require.NoError(t, err, trace.DebugReport(err))
			})
		}
	})
}

func TestCheckSPIFFESVID(t *testing.T) {
	t.Parallel()

	makeRole := func(allow []*types.SPIFFERoleCondition, deny []*types.SPIFFERoleCondition) types.Role {
		role, err := types.NewRole(uuid.NewString(), types.RoleSpecV6{
			Allow: types.RoleConditions{
				SPIFFE: allow,
			},
			Deny: types.RoleConditions{
				SPIFFE: deny,
			},
		})
		require.NoError(t, err)
		return role
	}
	tests := []struct {
		name  string
		roles []types.Role

		spiffeIDPath string
		dnsSANs      []string
		ipSANs       []net.IP

		requireErr require.ErrorAssertionFunc
	}{
		{
			name: "simple success",

			spiffeIDPath: "/foo/bar",
			dnsSANs: []string{
				"foo.example.com",
				"foo.example.net",
			},
			ipSANs: []net.IP{
				{10, 0, 0, 32},
			},

			roles: []types.Role{
				makeRole([]*types.SPIFFERoleCondition{
					{
						// Non-matching condition.
						Path:    "/bar/boo",
						DNSSANs: []string{},
						IPSANs:  []string{},
					},
					{
						Path: "/foo/*",
						DNSSANs: []string{
							"foo.example.com",
							"*.example.net",
						},
						IPSANs: []string{
							"10.0.0.1/8",
						},
					},
				}, []*types.SPIFFERoleCondition{}),
			},

			requireErr: require.NoError,
		},
		{
			name: "regex success",

			spiffeIDPath: "/foo/bar",
			dnsSANs: []string{
				"foo.example.com",
				"foo.example.net",
			},
			ipSANs: []net.IP{
				{10, 0, 0, 32},
			},

			roles: []types.Role{
				makeRole([]*types.SPIFFERoleCondition{
					{
						// Non-matching condition.
						Path:    "/bar/boo",
						DNSSANs: []string{},
						IPSANs:  []string{},
					},
					{
						Path: `^\/foo\/.*$`,
						DNSSANs: []string{
							"foo.example.com",
							"*.example.net",
						},
						IPSANs: []string{
							"10.0.0.1/8",
						},
					},
				}, []*types.SPIFFERoleCondition{}),
			},

			requireErr: require.NoError,
		},
		{
			name: "explicit deny - id path",

			spiffeIDPath: "/foo/bar",
			dnsSANs:      []string{},
			ipSANs:       []net.IP{},

			roles: []types.Role{
				makeRole([]*types.SPIFFERoleCondition{
					{
						Path:    "/foo/*",
						DNSSANs: []string{},
						IPSANs:  []string{},
					},
				}, []*types.SPIFFERoleCondition{
					{
						Path: "/foo/bar",
					},
				}),
			},

			requireErr: requireAccessDenied,
		},
		{
			name: "explicit deny - id path regex",

			spiffeIDPath: "/foo/bar",
			dnsSANs:      []string{},
			ipSANs:       []net.IP{},

			roles: []types.Role{
				makeRole([]*types.SPIFFERoleCondition{
					{
						Path:    "/foo/*",
						DNSSANs: []string{},
						IPSANs:  []string{},
					},
				}, []*types.SPIFFERoleCondition{
					{
						Path: `^\/foo\/bar$`,
					},
				}),
			},

			requireErr: requireAccessDenied,
		},
		{
			name: "explicit deny - ip san",

			spiffeIDPath: "/foo/bar",
			dnsSANs:      []string{},
			ipSANs: []net.IP{
				{10, 0, 0, 42},
			},

			roles: []types.Role{
				makeRole([]*types.SPIFFERoleCondition{
					{
						Path:    "/foo/*",
						DNSSANs: []string{},
						IPSANs:  []string{"10.0.0.1/8"},
					},
				}, []*types.SPIFFERoleCondition{
					{
						Path: "/*",
						IPSANs: []string{
							"10.0.0.42/32",
						},
					},
				}),
			},

			requireErr: requireAccessDenied,
		},
		{
			name: "explicit deny - dns san",

			spiffeIDPath: "/foo/bar",
			dnsSANs: []string{
				"foo.example.com",
			},
			ipSANs: []net.IP{},

			roles: []types.Role{
				makeRole([]*types.SPIFFERoleCondition{
					{
						Path: "/foo/*",
						DNSSANs: []string{
							"*",
						},
						IPSANs: []string{},
					},
				}, []*types.SPIFFERoleCondition{
					{
						Path: "/*",
						DNSSANs: []string{
							"foo.example.com",
						},
					},
				}),
			},

			requireErr: requireAccessDenied,
		},
		{
			name: "implicit deny - no match",

			spiffeIDPath: "/foo/bar",
			dnsSANs:      []string{},
			ipSANs:       []net.IP{},

			roles: []types.Role{
				makeRole([]*types.SPIFFERoleCondition{
					{
						Path: "/bar/*",
					},
				}, []*types.SPIFFERoleCondition{}),
			},

			requireErr: requireAccessDenied,
		},
		{
			name: "implicit deny - no match regex",

			spiffeIDPath: "/foo/bar",
			dnsSANs:      []string{},
			ipSANs:       []net.IP{},

			roles: []types.Role{
				makeRole([]*types.SPIFFERoleCondition{
					{
						Path: `^\/bar\/.*$`,
					},
				}, []*types.SPIFFERoleCondition{}),
			},

			requireErr: requireAccessDenied,
		},
		{
			name:  "no roles",
			roles: []types.Role{},

			spiffeIDPath: "/foo/bar",
			dnsSANs:      []string{},
			ipSANs:       []net.IP{},

			requireErr: requireAccessDenied,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			accessChecker := makeAccessCheckerWithRoleSet(tt.roles)
			err := accessChecker.CheckSPIFFESVID(tt.spiffeIDPath, tt.dnsSANs, tt.ipSANs)
			tt.requireErr(t, err)
		})
	}
}
