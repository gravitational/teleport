package resource

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types/wrappers"
	"github.com/gravitational/teleport/api/utils/sshutils"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/tlsca"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/trace"
)

func TestRoleParse(t *testing.T) {
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
						BPF:               defaults.EnhancedEvents(),
					},
					Allow: RoleConditions{
						NodeLabels:       Labels{},
						AppLabels:        Labels{Wildcard: []string{Wildcard}},
						KubernetesLabels: Labels{Wildcard: []string{Wildcard}},
						DatabaseLabels:   Labels{Wildcard: []string{Wildcard}},
						Namespaces:       []string{defaults.Namespace},
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
			role: RoleV3{
				Kind:    KindRole,
				Version: V3,
				Metadata: Metadata{
					Name:      "name1",
					Namespace: defaults.Namespace,
					Labels:    map[string]string{"a-b": "c"},
				},
				Spec: RoleSpecV3{
					Options: RoleOptions{
						CertificateFormat:     teleport.CertificateFormatStandard,
						MaxSessionTTL:         NewDuration(20 * time.Hour),
						PortForwarding:        NewBoolOption(true),
						ClientIdleTimeout:     NewDuration(17 * time.Minute),
						DisconnectExpiredCert: NewBool(true),
						BPF:                   defaults.EnhancedEvents(),
					},
					Allow: RoleConditions{
						NodeLabels:       Labels{"a": []string{"b"}, "c-d": []string{"e"}},
						AppLabels:        Labels{"a": []string{"b"}, "c-d": []string{"e"}},
						KubernetesLabels: Labels{"a": []string{"b"}, "c-d": []string{"e"}},
						DatabaseLabels:   Labels{"a": []string{"b"}, "c-d": []string{"e"}},
						DatabaseNames:    []string{"postgres"},
						DatabaseUsers:    []string{"postgres"},
						Namespaces:       []string{"default"},
						Rules: []Rule{
							{
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
						BPF:                   defaults.EnhancedEvents(),
					},
					Allow: RoleConditions{
						NodeLabels:       Labels{"a": []string{"b"}},
						AppLabels:        Labels{"a": []string{"b"}},
						KubernetesLabels: Labels{"c": []string{"d"}},
						DatabaseLabels:   Labels{"e": []string{"f"}},
						Namespaces:       []string{"default"},
						Rules: []Rule{
							{
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
						BPF:                   defaults.EnhancedEvents(),
					},
					Allow: RoleConditions{
						NodeLabels: Labels{
							"a":    []string{"b"},
							"key":  []string{"val"},
							"key2": []string{"val2", "val3"},
						},
						AppLabels: Labels{
							"a":    []string{"b"},
							"key":  []string{"val"},
							"key2": []string{"val2", "val3"},
						},
						KubernetesLabels: Labels{
							"a":    []string{"b"},
							"key":  []string{"val"},
							"key2": []string{"val2", "val3"},
						},
						DatabaseLabels: Labels{
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
				require.True(t, role.Equals(&tc.role))

				err := auth.ValidateRole(role)
				require.NoError(t, err)

				out, err := json.Marshal(role)
				require.NoError(t, err)

				role2, err := UnmarshalRole(out)
				require.NoError(t, err)
				require.True(t, role2.Equals(&tc.role))
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
	roles, traits, err := ExtractFromCertificate(&userGetter{
		roles:  origRoles,
		traits: origTraits,
	}, cert)
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
	roles, traits, err = ExtractFromCertificate(&userGetter{
		roles: []string{"intern"},
		traits: wrappers.Traits(map[string][]string{
			"login": {"bar"},
		}),
	}, cert)
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

// TestExtractFromLegacy verifies that roles and traits are fetched
// from services.User for SSH certificates is the legacy format and TLS
// certificates that don't contain traits.
func TestExtractFromLegacy(t *testing.T) {
	origRoles := []string{"admin"}
	origTraits := wrappers.Traits(map[string][]string{
		"login": {"foo"},
	})

	// Create a SSH certificate in the legacy format.
	cert, err := sshutils.ParseCertificate([]byte(fixtures.UserCertificateLegacy))
	require.NoError(t, err)

	// Create a TLS identity with only roles.
	identity := &tlsca.Identity{
		Username: "foo",
		Groups:   origRoles,
	}

	// At this point, services.User and the certificate/identity are still in
	// sync. The roles and traits returned should be the same as the original.
	roles, traits, err := ExtractFromCertificate(&userGetter{
		roles:  origRoles,
		traits: origTraits,
	}, cert)
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

	// The backend now returns new roles and traits, because the SSH certificate
	// is in the old standard format and the TLS identity is missing traits.
	newRoles := []string{"intern"}
	newTraits := wrappers.Traits(map[string][]string{
		"login": {"bar"},
	})
	roles, traits, err = ExtractFromCertificate(&userGetter{
		roles:  newRoles,
		traits: newTraits,
	}, cert)
	require.NoError(t, err)
	require.Equal(t, roles, newRoles)
	require.Equal(t, traits, newTraits)
	roles, traits, err = ExtractFromIdentity(&userGetter{
		roles:  newRoles,
		traits: newTraits,
	}, *identity)
	require.NoError(t, err)
	require.Equal(t, roles, newRoles)
	require.Equal(t, traits, newTraits)
}

// userGetter is used in tests to return a user with the specified roles and
// traits.
type userGetter struct {
	roles  []string
	traits map[string][]string
}

func (f *userGetter) GetUser(name string, _ bool) (User, error) {
	user, err := NewUser(name)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	user.SetRoles(f.roles)
	user.SetTraits(f.traits)
	return user, nil
}
