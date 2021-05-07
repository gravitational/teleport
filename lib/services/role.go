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

package services

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"golang.org/x/crypto/ssh"

	"github.com/gravitational/configure/cstrings"
	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/wrappers"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/services/accesschecker"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/parse"

	"github.com/gravitational/trace"

	"github.com/pborman/uuid"
	log "github.com/sirupsen/logrus"
)

// getExtendedAdminUserRules provides access to the default set of rules assigned to
// all users.
func getExtendedAdminUserRules(features modules.Features) []Rule {
	rules := []Rule{
		NewRule(KindRole, RW()),
		NewRule(KindAuthConnector, RW()),
		NewRule(KindSession, RO()),
		NewRule(KindTrustedCluster, RW()),
		NewRule(KindEvent, RO()),
		NewRule(KindUser, RW()),
		NewRule(KindToken, RW()),
	}

	if features.Cloud {
		rules = append(rules, NewRule(KindBilling, RW()))
	}

	return rules
}

// DefaultImplicitRules provides access to the default set of implicit rules
// assigned to all roles.
var DefaultImplicitRules = []Rule{
	NewRule(KindNode, RO()),
	NewRule(KindProxy, RO()),
	NewRule(KindAuthServer, RO()),
	NewRule(KindReverseTunnel, RO()),
	NewRule(KindCertAuthority, ReadNoSecrets()),
	NewRule(KindClusterAuthPreference, RO()),
	NewRule(KindClusterName, RO()),
	NewRule(KindSSHSession, RO()),
	NewRule(KindAppServer, RO()),
	NewRule(KindRemoteCluster, RO()),
	NewRule(KindKubeService, RO()),
	NewRule(types.KindDatabaseServer, RO()),
}

// DefaultCertAuthorityRules provides access the minimal set of resources
// needed for a certificate authority to function.
var DefaultCertAuthorityRules = []Rule{
	NewRule(KindSession, RO()),
	NewRule(KindNode, RO()),
	NewRule(KindAuthServer, RO()),
	NewRule(KindReverseTunnel, RO()),
	NewRule(KindCertAuthority, ReadNoSecrets()),
}

// RoleNameForUser returns role name associated with a user.
func RoleNameForUser(name string) string {
	return "user:" + name
}

// RoleNameForCertAuthority returns role name associated with a certificate
// authority.
func RoleNameForCertAuthority(name string) string {
	return "ca:" + name
}

// NewAdminRole is the default admin role for all local users if another role
// is not explicitly assigned (this role applies to all users in OSS version).
func NewAdminRole() Role {
	adminRules := getExtendedAdminUserRules(modules.GetModules().Features())
	role := &RoleV3{
		Kind:    KindRole,
		Version: V3,
		Metadata: Metadata{
			Name:      teleport.AdminRoleName,
			Namespace: defaults.Namespace,
		},
		Spec: RoleSpecV3{
			Options: RoleOptions{
				CertificateFormat: teleport.CertificateFormatStandard,
				MaxSessionTTL:     NewDuration(defaults.MaxCertDuration),
				PortForwarding:    NewBoolOption(true),
				ForwardAgent:      NewBool(true),
				BPF:               defaults.EnhancedEvents(),
			},
			Allow: RoleConditions{
				Namespaces:       []string{defaults.Namespace},
				NodeLabels:       Labels{Wildcard: []string{Wildcard}},
				AppLabels:        Labels{Wildcard: []string{Wildcard}},
				KubernetesLabels: Labels{Wildcard: []string{Wildcard}},
				DatabaseLabels:   Labels{Wildcard: []string{Wildcard}},
				DatabaseNames:    []string{teleport.TraitInternalDBNamesVariable},
				DatabaseUsers:    []string{teleport.TraitInternalDBUsersVariable},
				Rules:            adminRules,
			},
		},
	}
	role.SetLogins(Allow, []string{teleport.TraitInternalLoginsVariable, teleport.Root})
	role.SetKubeUsers(Allow, []string{teleport.TraitInternalKubeUsersVariable})
	role.SetKubeGroups(Allow, []string{teleport.TraitInternalKubeGroupsVariable})
	return role
}

// RoleForUser creates an admin role for a services.User.
func RoleForUser(u User) Role {
	return &RoleV3{
		Kind:    KindRole,
		Version: V3,
		Metadata: Metadata{
			Name:      RoleNameForUser(u.GetName()),
			Namespace: defaults.Namespace,
		},
		Spec: RoleSpecV3{
			Options: RoleOptions{
				CertificateFormat: teleport.CertificateFormatStandard,
				MaxSessionTTL:     NewDuration(defaults.MaxCertDuration),
				PortForwarding:    NewBoolOption(true),
				ForwardAgent:      NewBool(true),
				BPF:               defaults.EnhancedEvents(),
			},
			Allow: RoleConditions{
				Namespaces:       []string{defaults.Namespace},
				NodeLabels:       Labels{Wildcard: []string{Wildcard}},
				AppLabels:        Labels{Wildcard: []string{Wildcard}},
				KubernetesLabels: Labels{Wildcard: []string{Wildcard}},
				DatabaseLabels:   Labels{Wildcard: []string{Wildcard}},
				Rules: []Rule{
					NewRule(KindRole, RW()),
					NewRule(KindAuthConnector, RW()),
					NewRule(KindSession, RO()),
					NewRule(KindTrustedCluster, RW()),
					NewRule(KindEvent, RO()),
				},
			},
		},
	}
}

// NewDowngradedOSSAdminRole is a role for enabling RBAC for open source users.
// This role overrides built in OSS "admin" role to have less privileges.
// DELETE IN (7.x)
func NewDowngradedOSSAdminRole() Role {
	role := &RoleV3{
		Kind:    KindRole,
		Version: V3,
		Metadata: Metadata{
			Name:      teleport.AdminRoleName,
			Namespace: defaults.Namespace,
			Labels:    map[string]string{teleport.OSSMigratedV6: types.True},
		},
		Spec: RoleSpecV3{
			Options: RoleOptions{
				CertificateFormat: teleport.CertificateFormatStandard,
				MaxSessionTTL:     NewDuration(defaults.MaxCertDuration),
				PortForwarding:    NewBoolOption(true),
				ForwardAgent:      NewBool(true),
				BPF:               defaults.EnhancedEvents(),
			},
			Allow: RoleConditions{
				Namespaces:       []string{defaults.Namespace},
				NodeLabels:       Labels{Wildcard: []string{Wildcard}},
				AppLabels:        Labels{Wildcard: []string{Wildcard}},
				KubernetesLabels: Labels{Wildcard: []string{Wildcard}},
				DatabaseLabels:   Labels{Wildcard: []string{Wildcard}},
				DatabaseNames:    []string{teleport.TraitInternalDBNamesVariable},
				DatabaseUsers:    []string{teleport.TraitInternalDBUsersVariable},
				Rules: []Rule{
					NewRule(KindEvent, RO()),
					NewRule(KindSession, RO()),
				},
			},
		},
	}
	role.SetLogins(Allow, []string{teleport.TraitInternalLoginsVariable})
	role.SetKubeUsers(Allow, []string{teleport.TraitInternalKubeUsersVariable})
	role.SetKubeGroups(Allow, []string{teleport.TraitInternalKubeGroupsVariable})
	return role
}

// NewOSSGithubRole creates a role for enabling RBAC for open source Github users
func NewOSSGithubRole(logins []string, kubeUsers []string, kubeGroups []string) Role {
	role := &RoleV3{
		Kind:    KindRole,
		Version: V3,
		Metadata: Metadata{
			Name:      "github-" + uuid.New(),
			Namespace: defaults.Namespace,
		},
		Spec: RoleSpecV3{
			Options: RoleOptions{
				CertificateFormat: teleport.CertificateFormatStandard,
				MaxSessionTTL:     NewDuration(defaults.MaxCertDuration),
				PortForwarding:    NewBoolOption(true),
				ForwardAgent:      NewBool(true),
				BPF:               defaults.EnhancedEvents(),
			},
			Allow: RoleConditions{
				Namespaces:       []string{defaults.Namespace},
				NodeLabels:       Labels{Wildcard: []string{Wildcard}},
				AppLabels:        Labels{Wildcard: []string{Wildcard}},
				KubernetesLabels: Labels{Wildcard: []string{Wildcard}},
				DatabaseLabels:   Labels{Wildcard: []string{Wildcard}},
				DatabaseNames:    []string{teleport.TraitInternalDBNamesVariable},
				DatabaseUsers:    []string{teleport.TraitInternalDBUsersVariable},
				Rules: []Rule{
					NewRule(KindEvent, RO()),
				},
			},
		},
	}
	role.SetLogins(Allow, logins)
	role.SetKubeUsers(Allow, kubeUsers)
	role.SetKubeGroups(Allow, kubeGroups)
	return role
}

// RoleForCertAuthority creates role using services.CertAuthority.
func RoleForCertAuthority(ca CertAuthority) Role {
	return &RoleV3{
		Kind:    KindRole,
		Version: V3,
		Metadata: Metadata{
			Name:      RoleNameForCertAuthority(ca.GetClusterName()),
			Namespace: defaults.Namespace,
		},
		Spec: RoleSpecV3{
			Options: RoleOptions{
				MaxSessionTTL: NewDuration(defaults.MaxCertDuration),
			},
			Allow: RoleConditions{
				Namespaces:       []string{defaults.Namespace},
				NodeLabels:       Labels{Wildcard: []string{Wildcard}},
				AppLabels:        Labels{Wildcard: []string{Wildcard}},
				KubernetesLabels: Labels{Wildcard: []string{Wildcard}},
				DatabaseLabels:   Labels{Wildcard: []string{Wildcard}},
				Rules:            CopyRulesSlice(DefaultCertAuthorityRules),
			},
		},
	}
}

// Access service manages roles and permissions
type Access interface {
	// GetRoles returns a list of roles
	GetRoles(ctx context.Context) ([]Role, error)

	// CreateRole creates a role
	CreateRole(role Role) error

	// UpsertRole creates or updates role
	UpsertRole(ctx context.Context, role Role) error

	// DeleteAllRoles deletes all roles
	DeleteAllRoles() error

	// GetRole returns role by name
	GetRole(ctx context.Context, name string) (Role, error)

	// DeleteRole deletes role by name
	DeleteRole(ctx context.Context, name string) error
}

const (
	// Allow is the set of conditions that allow access.
	Allow RoleConditionType = true
	// Deny is the set of conditions that prevent access.
	Deny RoleConditionType = false
)

// ValidateRole parses validates the role, and sets default values.
func ValidateRole(r Role) error {
	if err := r.CheckAndSetDefaults(); err != nil {
		return err
	}

	// if we find {{ or }} but the syntax is invalid, the role is invalid
	for _, condition := range []RoleConditionType{Allow, Deny} {
		for _, login := range r.GetLogins(condition) {
			if strings.Contains(login, "{{") || strings.Contains(login, "}}") {
				_, err := parse.NewExpression(login)
				if err != nil {
					return trace.BadParameter("invalid login found: %v", login)
				}
			}
		}
	}

	rules := append(r.GetRules(types.Allow), r.GetRules(types.Deny)...)
	for _, rule := range rules {
		if err := validateRule(rule); err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}

// validateRule parses the where and action fields to validate the rule.
func validateRule(r Rule) error {
	if len(r.Where) != 0 {
		parser, err := accesschecker.NewWhereParser(&Context{})
		if err != nil {
			return trace.Wrap(err)
		}
		_, err = parser.Parse(r.Where)
		if err != nil {
			return trace.BadParameter("could not parse 'where' rule: %q, error: %v", r.Where, err)
		}
	}

	if len(r.Actions) != 0 {
		parser, err := accesschecker.NewActionsParser(&Context{})
		if err != nil {
			return trace.Wrap(err)
		}
		for i, action := range r.Actions {
			_, err = parser.Parse(action)
			if err != nil {
				return trace.BadParameter("could not parse action %v %q, error: %v", i, action, err)
			}
		}
	}
	return nil
}

// FromSpec returns new RoleSet created from spec
func FromSpec(name string, spec RoleSpecV3) (RoleSet, error) {
	role, err := NewRole(name, spec)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return accesschecker.NewRoleSet(role), nil
}

// RW is a shortcut that returns all verbs.
func RW() []string {
	return []string{VerbList, VerbCreate, VerbRead, VerbUpdate, VerbDelete}
}

// RO is a shortcut that returns read only verbs that provide access to secrets.
func RO() []string {
	return []string{VerbList, VerbRead}
}

// ReadNoSecrets is a shortcut that returns read only verbs that do not
// provide access to secrets.
func ReadNoSecrets() []string {
	return []string{VerbList, VerbReadNoSecrets}
}

// RoleGetter is an interface that defines GetRole method
type RoleGetter interface {
	// GetRole returns role by name
	GetRole(ctx context.Context, name string) (Role, error)
}

// ExtractFromCertificate will extract roles and traits from a *ssh.Certificate
// or from the backend if they do not exist in the certificate.
func ExtractFromCertificate(access UserGetter, cert *ssh.Certificate) ([]string, wrappers.Traits, error) {
	// For legacy certificates, fetch roles and traits from the services.User
	// object in the backend.
	if isFormatOld(cert) {
		u, err := access.GetUser(cert.KeyId, false)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}
		log.Warnf("User %v using old style SSH certificate, fetching roles and traits "+
			"from backend. If the identity provider allows username changes, this can "+
			"potentially allow an attacker to change the role of the existing user. "+
			"It's recommended to upgrade to standard SSH certificates.", cert.KeyId)
		return u.GetRoles(), u.GetTraits(), nil
	}

	// Standard certificates have the roles and traits embedded in them.
	roles, err := ExtractRolesFromCert(cert)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	traits, err := ExtractTraitsFromCert(cert)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	return roles, traits, nil
}

// ExtractFromIdentity will extract roles and traits from the *x509.Certificate
// which Teleport passes along as a *tlsca.Identity. If roles and traits do not
// exist in the certificates, they are extracted from the backend.
func ExtractFromIdentity(access UserGetter, identity tlsca.Identity) ([]string, wrappers.Traits, error) {
	// For legacy certificates, fetch roles and traits from the services.User
	// object in the backend.
	if missingIdentity(identity) {
		u, err := access.GetUser(identity.Username, false)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}

		log.Warnf("Failed to find roles or traits in x509 identity for %v. Fetching	"+
			"from backend. If the identity provider allows username changes, this can "+
			"potentially allow an attacker to change the role of the existing user.",
			identity.Username)
		return u.GetRoles(), u.GetTraits(), nil
	}

	return identity.Groups, identity.Traits, nil
}

// FetchRoleList fetches roles by their names, applies the traits to role
// variables, and returns the list
func FetchRoleList(roleNames []string, access RoleGetter, traits map[string][]string) (RoleSet, error) {
	var roles []Role

	for _, roleName := range roleNames {
		role, err := access.GetRole(context.TODO(), roleName)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		roles = append(roles, ApplyTraits(role, traits))
	}

	return roles, nil
}

// ApplyTraits applies the passed in traits to any variables within the role
// and returns itself.
func ApplyTraits(r Role, traits map[string][]string) Role {
	for _, condition := range []RoleConditionType{Allow, Deny} {
		inLogins := r.GetLogins(condition)

		var outLogins []string
		for _, login := range inLogins {
			variableValues, err := ApplyValueTraits(login, traits)
			if err != nil {
				if !trace.IsNotFound(err) {
					log.Debugf("Skipping login %v: %v.", login, err)
				}
				continue
			}

			// Filter out logins that come from variables that are not valid Unix logins.
			for _, variableValue := range variableValues {
				if !cstrings.IsValidUnixUser(variableValue) {
					log.Debugf("Skipping login %v, not a valid Unix login.", variableValue)
					continue
				}

				// A valid variable was found in the traits, append it to the list of logins.
				outLogins = append(outLogins, variableValue)
			}
		}

		r.SetLogins(condition, utils.Deduplicate(outLogins))

		// apply templates to kubernetes groups
		inKubeGroups := r.GetKubeGroups(condition)
		var outKubeGroups []string
		for _, group := range inKubeGroups {
			variableValues, err := ApplyValueTraits(group, traits)
			if err != nil {
				if !trace.IsNotFound(err) {
					log.Debugf("Skipping kube group %v: %v.", group, err)
				}
				continue
			}
			outKubeGroups = append(outKubeGroups, variableValues...)
		}
		r.SetKubeGroups(condition, utils.Deduplicate(outKubeGroups))

		// apply templates to kubernetes users
		inKubeUsers := r.GetKubeUsers(condition)
		var outKubeUsers []string
		for _, user := range inKubeUsers {
			variableValues, err := ApplyValueTraits(user, traits)
			if err != nil {
				if !trace.IsNotFound(err) {
					log.Debugf("Skipping kube user %v: %v.", user, err)
				}
				continue
			}
			outKubeUsers = append(outKubeUsers, variableValues...)
		}
		r.SetKubeUsers(condition, utils.Deduplicate(outKubeUsers))

		// apply templates to database names
		inDbNames := r.GetDatabaseNames(condition)
		var outDbNames []string
		for _, name := range inDbNames {
			variableValues, err := ApplyValueTraits(name, traits)
			if err != nil {
				if !trace.IsNotFound(err) {
					log.Debugf("Skipping database name %q: %v.", name, err)
				}
				continue
			}
			outDbNames = append(outDbNames, variableValues...)
		}
		r.SetDatabaseNames(condition, utils.Deduplicate(outDbNames))

		// apply templates to database users
		inDbUsers := r.GetDatabaseUsers(condition)
		var outDbUsers []string
		for _, user := range inDbUsers {
			variableValues, err := ApplyValueTraits(user, traits)
			if err != nil {
				if !trace.IsNotFound(err) {
					log.Debugf("Skipping database user %q: %v.", user, err)
				}
				continue
			}
			outDbUsers = append(outDbUsers, variableValues...)
		}
		r.SetDatabaseUsers(condition, utils.Deduplicate(outDbUsers))

		// apply templates to node labels
		inLabels := r.GetNodeLabels(condition)
		if inLabels != nil {
			r.SetNodeLabels(condition, applyLabelsTraits(inLabels, traits))
		}

		// apply templates to cluster labels
		inLabels = r.GetClusterLabels(condition)
		if inLabels != nil {
			r.SetClusterLabels(condition, applyLabelsTraits(inLabels, traits))
		}

		// apply templates to kube labels
		inLabels = r.GetKubernetesLabels(condition)
		if inLabels != nil {
			r.SetKubernetesLabels(condition, applyLabelsTraits(inLabels, traits))
		}

		// apply templates to app labels
		inLabels = r.GetAppLabels(condition)
		if inLabels != nil {
			r.SetAppLabels(condition, applyLabelsTraits(inLabels, traits))
		}

		// apply templates to database labels
		inLabels = r.GetDatabaseLabels(condition)
		if inLabels != nil {
			r.SetDatabaseLabels(condition, applyLabelsTraits(inLabels, traits))
		}

		// apply templates to impersonation conditions
		inCond := r.GetImpersonateConditions(condition)
		var outCond types.ImpersonateConditions
		for _, user := range inCond.Users {
			variableValues, err := ApplyValueTraits(user, traits)
			if err != nil {
				if !trace.IsNotFound(err) {
					log.WithError(err).Debugf("Skipping impersonate user %q.", user)
				}
				continue
			}
			outCond.Users = append(outCond.Users, variableValues...)
		}
		for _, role := range inCond.Roles {
			variableValues, err := ApplyValueTraits(role, traits)
			if err != nil {
				if !trace.IsNotFound(err) {
					log.WithError(err).Debugf("Skipping impersonate role %q.", role)
				}
				continue
			}
			outCond.Roles = append(outCond.Roles, variableValues...)
		}
		outCond.Users = utils.Deduplicate(outCond.Users)
		outCond.Roles = utils.Deduplicate(outCond.Roles)
		outCond.Where = inCond.Where
		r.SetImpersonateConditions(condition, outCond)
	}

	return r
}

// applyLabelsTraits interpolates variables based on the templates
// and traits from identity provider. For example:
//
// cluster_labels:
//   env: ['{{external.groups}}']
//
// and groups: ['admins', 'devs']
//
// will be interpolated to:
//
// cluster_labels:
//   env: ['admins', 'devs']
//
func applyLabelsTraits(inLabels Labels, traits map[string][]string) Labels {
	outLabels := make(Labels, len(inLabels))
	// every key will be mapped to the first value
	for key, vals := range inLabels {
		keyVars, err := ApplyValueTraits(key, traits)
		if err != nil {
			// empty key will not match anything
			log.Debugf("Setting empty node label pair %q -> %q: %v", key, vals, err)
			keyVars = []string{""}
		}

		var values []string
		for _, val := range vals {
			valVars, err := ApplyValueTraits(val, traits)
			if err != nil {
				log.Debugf("Setting empty node label value %q -> %q: %v", key, val, err)
				// empty value will not match anything
				valVars = []string{""}
			}
			values = append(values, valVars...)
		}
		outLabels[keyVars[0]] = utils.Deduplicate(values)
	}
	return outLabels
}

// ApplyValueTraits applies the passed in traits to the variable,
// returns BadParameter in case if referenced variable is unsupported,
// returns NotFound in case if referenced trait is missing,
// mapped list of values otherwise, the function guarantees to return
// at least one value in case if return value is nil
func ApplyValueTraits(val string, traits map[string][]string) ([]string, error) {
	// Extract the variable from the role variable.
	variable, err := parse.NewExpression(val)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// For internal traits, only internal.logins, internal.kubernetes_users and
	// internal.kubernetes_groups are supported at the moment.
	if variable.Namespace() == teleport.TraitInternalPrefix {
		switch variable.Name() {
		case teleport.TraitLogins, teleport.TraitKubeGroups, teleport.TraitKubeUsers, teleport.TraitDBNames, teleport.TraitDBUsers:
		default:
			return nil, trace.BadParameter("unsupported variable %q", variable.Name())
		}
	}

	// If the variable is not found in the traits, skip it.
	interpolated, err := variable.Interpolate(traits)
	if trace.IsNotFound(err) || len(interpolated) == 0 {
		return nil, trace.NotFound("variable %q not found in traits", variable.Name())
	}
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return interpolated, nil
}

// FetchRoles fetches roles by their names, applies the traits to role
// variables, and returns the RoleSet. Adds runtime roles like the default
// implicit role to RoleSet.
func FetchRoles(roleNames []string, access RoleGetter, traits map[string][]string) (RoleSet, error) {
	roleList, err := FetchRoleList(roleNames, access, traits)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return accesschecker.NewRoleSet(roleList...), nil
}

// isFormatOld returns true if roles and traits were not found in the
// *ssh.Certificate.
func isFormatOld(cert *ssh.Certificate) bool {
	_, hasRoles := cert.Extensions[teleport.CertExtensionTeleportRoles]
	_, hasTraits := cert.Extensions[teleport.CertExtensionTeleportTraits]

	if hasRoles || hasTraits {
		return false
	}
	return true
}

// missingIdentity returns true if the identity is missing or the identity
// has no roles or traits.
func missingIdentity(identity tlsca.Identity) bool {
	if len(identity.Groups) == 0 || len(identity.Traits) == 0 {
		return true
	}
	return false
}

// ExtractRolesFromCert extracts roles from certificate metadata extensions.
func ExtractRolesFromCert(cert *ssh.Certificate) ([]string, error) {
	data, ok := cert.Extensions[teleport.CertExtensionTeleportRoles]
	if !ok {
		return nil, trace.NotFound("no roles found")
	}
	return UnmarshalCertRoles(data)
}

// ExtractTraitsFromCert extracts traits from the certificate extensions.
func ExtractTraitsFromCert(cert *ssh.Certificate) (wrappers.Traits, error) {
	rawTraits, ok := cert.Extensions[teleport.CertExtensionTeleportTraits]
	if !ok {
		return nil, trace.NotFound("no traits found")
	}
	var traits wrappers.Traits
	err := wrappers.UnmarshalTraits([]byte(rawTraits), &traits)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return traits, nil
}

// SortedRoles sorts roles by name
type SortedRoles []Role

// Len returns length of a role list
func (s SortedRoles) Len() int {
	return len(s)
}

// Less compares roles by name
func (s SortedRoles) Less(i, j int) bool {
	return s[i].GetName() < s[j].GetName()
}

// Swap swaps two roles in a list
func (s SortedRoles) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

// RoleSpecV3SchemaTemplate is JSON schema for RoleSpecV3
const RoleSpecV3SchemaTemplate = `{
	"type": "object",
	"additionalProperties": false,
	"properties": {
	  "max_session_ttl": { "type": "string" },
	  "options": {
		"type": "object",
		"additionalProperties": false,
		"properties": {
		  "forward_agent": { "type": ["boolean", "string"] },
		  "permit_x11_forwarding": { "type": ["boolean", "string"] },
		  "max_session_ttl": { "type": "string" },
		  "port_forwarding": { "type": ["boolean", "string"] },
		  "cert_format": { "type": "string" },
		  "client_idle_timeout": { "type": "string" },
		  "disconnect_expired_cert": { "type": ["boolean", "string"] },
		  "enhanced_recording": {
			"type": "array",
			"items": { "type": "string" }
		  },
		  "max_connections": { "type": "number" },
		  "max_sessions": {"type": "number"},
		  "request_access": { "type": "string" },
		  "request_prompt": { "type": "string" },
		  "require_session_mfa": { "type": ["boolean", "string"] }
		}
	  },
	  "allow": { "$ref": "#/definitions/role_condition" },
	  "deny": { "$ref": "#/definitions/role_condition" }%v
	}
  }`

// RoleSpecV3SchemaDefinitions is JSON schema for RoleSpecV3 definitions
const RoleSpecV3SchemaDefinitions = `
	  "definitions": {
		"role_condition": {
		  "namespaces": {
			"type": "array",
			"items": { "type": "string" }
		  },
		  "node_labels": {
			"type": "object",
			"additionalProperties": false,
			"patternProperties": {
			  "^[a-zA-Z/.0-9_*-]+$": { "anyOf": [{"type": "string"}, { "type": "array", "items": {"type": "string"}}]}
			}
		  },
		  "cluster_labels": {
			"type": "object",
			"additionalProperties": false,
			"patternProperties": {
			  "^[a-zA-Z/.0-9_*-]+$": { "anyOf": [{"type": "string"}, { "type": "array", "items": {"type": "string"}}]}
			}
		  },
		  "logins": {
			"type": "array",
			"items": { "type": "string" }
		  },
		  "kubernetes_groups": {
			"type": "array",
			"items": { "type": "string" }
		  },
		  "db_labels": {
			"type": "object",
			"additionalProperties": false,
			"patternProperties": {
			  "^[a-zA-Z/.0-9_*-]+$": {"anyOf": [{"type": "string"}, {"type": "array", "items": {"type": "string"}}]}
			}
		  },
		  "kubernetes_labels": {
			"type": "object",
			"additionalProperties": false,
			"patternProperties": {
			  "^[a-zA-Z/.0-9_*-]+$": {"anyOf": [{"type": "string"}, {"type": "array", "items": {"type": "string"}}]}
			}
		  },
		  "db_names": {
			"type": "array",
			"items": {"type": "string"}
		  },
		  "db_users": {
			"type": "array",
			"items": {"type": "string"}
		  },
		  "request": {
			"type": "object",
			"additionalProperties": false,
			"properties": {
			  "roles": {
				"type": "array",
				"items": { "type": "string" }
			  },
			  "claims_to_roles": {
				"type": "object",
				"additionalProperties": false,
				"properties": {
				  "claim": {"type": "string"},
				  "value": {"type": "string"},
				  "roles": {
					"type": "array",
					"items": {
					  "type": "string"
					}
				  }
				}
			  },
			  "thresholds": {
			    "type": "array",
				"items": { "type": "object" }
			  }
			}
		  },
		  "impersonate": {
			"type": "object",
			"additionalProperties": false,
			"properties": {
			  "users": {
				"type": "array",
				"items": { "type": "string" }
			  },
			  "roles": {
				"type": "array",
				"items": { "type": "string" }
			  },
			  "where": {
			    "type": "string"
			  }
			}
		  },
		  "review_requests": {
		    "type": "object"
		  },
		  "rules": {
			"type": "array",
			"items": {
			  "type": "object",
			  "additionalProperties": false,
			  "properties": {
				"resources": {
				  "type": "array",
				  "items": { "type": "string" }
				},
				"verbs": {
				  "type": "array",
				  "items": { "type": "string" }
				},
				"where": {
				   "type": "string"
				},
				"actions": {
				  "type": "array",
				  "items": { "type": "string" }
				}
			  }
			}
		  }
		}
	  }
	`

// GetRoleSchema returns role schema for the version requested with optionally
// injected schema for extensions.
func GetRoleSchema(version string, extensionSchema string) string {
	schemaDefinitions := "," + RoleSpecV3SchemaDefinitions
	schemaTemplate := RoleSpecV3SchemaTemplate

	schema := fmt.Sprintf(schemaTemplate, ``)
	if extensionSchema != "" {
		schema = fmt.Sprintf(schemaTemplate, ","+extensionSchema)
	}

	return fmt.Sprintf(V2SchemaTemplate, MetadataSchema, schema, schemaDefinitions)
}

// UnmarshalRole unmarshals the Role resource from JSON.
func UnmarshalRole(bytes []byte, opts ...MarshalOption) (Role, error) {
	var h ResourceHeader
	err := json.Unmarshal(bytes, &h)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch h.Version {
	case V3:
		var role RoleV3
		if cfg.SkipValidation {
			if err := utils.FastUnmarshal(bytes, &role); err != nil {
				return nil, trace.BadParameter(err.Error())
			}
		} else {
			if err := utils.UnmarshalWithSchema(GetRoleSchema(V3, ""), &role, bytes); err != nil {
				return nil, trace.BadParameter(err.Error())
			}
		}

		if err := ValidateRole(&role); err != nil {
			return nil, trace.Wrap(err)
		}

		if cfg.ID != 0 {
			role.SetResourceID(cfg.ID)
		}
		if !cfg.Expires.IsZero() {
			role.SetExpiry(cfg.Expires)
		}
		return &role, nil
	}

	return nil, trace.BadParameter("role version %q is not supported", h.Version)
}

// MarshalRole marshals the Role resource to JSON.
func MarshalRole(role Role, opts ...MarshalOption) ([]byte, error) {
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch role := role.(type) {
	case *RoleV3:
		if version := role.GetVersion(); version != V3 {
			return nil, trace.BadParameter("mismatched role version %v and type %T", version, role)
		}
		if !cfg.PreserveResourceID {
			// avoid modifying the original object
			// to prevent unexpected data races
			copy := *role
			copy.SetResourceID(0)
			role = &copy
		}
		return utils.FastMarshal(role)
	default:
		return nil, trace.BadParameter("unrecognized role version %T", role)
	}
}
