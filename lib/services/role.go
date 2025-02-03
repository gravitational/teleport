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
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"path"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	jsoniter "github.com/json-iterator/go"
	"github.com/vulcand/predicate"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/wrappers"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/api/utils/keys"
	dtauthz "github.com/gravitational/teleport/lib/devicetrust/authz"
	"github.com/gravitational/teleport/lib/services/readonly"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
	awsutils "github.com/gravitational/teleport/lib/utils/aws"
	logutils "github.com/gravitational/teleport/lib/utils/log"
	"github.com/gravitational/teleport/lib/utils/parse"
)

// DefaultImplicitRules provides access to the default set of implicit rules
// assigned to all roles.
var DefaultImplicitRules = []types.Rule{
	types.NewRule(types.KindNode, RO()),
	types.NewRule(types.KindProxy, RO()),
	types.NewRule(types.KindAuthServer, RO()),
	types.NewRule(types.KindReverseTunnel, RO()),
	types.NewRule(types.KindCertAuthority, ReadNoSecrets()),
	types.NewRule(types.KindClusterAuthPreference, RO()),
	types.NewRule(types.KindClusterName, RO()),
	types.NewRule(types.KindSSHSession, RO()),
	types.NewRule(types.KindAppServer, RO()),
	types.NewRule(types.KindRemoteCluster, RO()),
	types.NewRule(types.KindKubeServer, RO()),
	types.NewRule(types.KindDatabaseServer, RO()),
	types.NewRule(types.KindDatabase, RO()),
	types.NewRule(types.KindApp, RO()),
	types.NewRule(types.KindWindowsDesktopService, RO()),
	types.NewRule(types.KindWindowsDesktop, RO()),
	types.NewRule(types.KindDynamicWindowsDesktop, RO()),
	types.NewRule(types.KindKubernetesCluster, RO()),
	types.NewRule(types.KindUsageEvent, []string{types.VerbCreate}),
	types.NewRule(types.KindVnetConfig, RO()),
	types.NewRule(types.KindSPIFFEFederation, RO()),
	types.NewRule(types.KindSAMLIdPServiceProvider, RO()),
	types.NewRule(types.KindIdentityCenter, RO()),
	types.NewRule(types.KindGitServer, RO()),
}

// DefaultCertAuthorityRules provides access the minimal set of resources
// needed for a certificate authority to function.
var DefaultCertAuthorityRules = []types.Rule{
	types.NewRule(types.KindSession, RO()),
	types.NewRule(types.KindNode, RO()),
	types.NewRule(types.KindAuthServer, RO()),
	types.NewRule(types.KindReverseTunnel, RO()),
	types.NewRule(types.KindCertAuthority, ReadNoSecrets()),
}

// ErrTrustedDeviceRequired is returned by AccessChecker when access to a
// resource requires a trusted device.
// It's an alias to [dtauthz.ErrTrustedDeviceRequired].
var ErrTrustedDeviceRequired = dtauthz.ErrTrustedDeviceRequired

// ErrSessionMFARequired is returned by AccessChecker when access to a resource
// requires an MFA check.
var ErrSessionMFARequired = &trace.AccessDeniedError{
	Message: "access to resource requires MFA",
}

// ErrSessionMFANotRequired indicates that per session mfa will not grant
// access to a resource.
var ErrSessionMFANotRequired = &trace.AccessDeniedError{
	Message: "MFA is not required to access resource",
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

// NewImplicitRole is the default implicit role that gets added to all
// RoleSets.
func NewImplicitRole() types.Role {
	return &types.RoleV6{
		Kind:    types.KindRole,
		Version: types.V3,
		Metadata: types.Metadata{
			Name:      constants.DefaultImplicitRole,
			Namespace: defaults.Namespace,
		},
		Spec: types.RoleSpecV6{
			Options: types.RoleOptions{
				MaxSessionTTL: types.MaxDuration(),
				RecordSession: &types.RecordSession{
					Desktop: types.NewBoolOption(false),
				},
			},
			Allow: types.RoleConditions{
				Namespaces: []string{defaults.Namespace},
				Rules:      types.CopyRulesSlice(DefaultImplicitRules),
			},
		},
	}
}

// RoleForUser creates an admin role for a services.User.
//
// Used in tests only.
func RoleForUser(u types.User) types.Role {
	role, _ := types.NewRole(RoleNameForUser(u.GetName()), types.RoleSpecV6{
		Options: types.RoleOptions{
			CertificateFormat: constants.CertificateFormatStandard,
			MaxSessionTTL:     types.NewDuration(defaults.MaxCertDuration),
			PortForwarding:    types.NewBoolOption(true),
			ForwardAgent:      types.NewBool(true),
			BPF:               defaults.EnhancedEvents(),
		},
		Allow: types.RoleConditions{
			Namespaces:            []string{defaults.Namespace},
			NodeLabels:            types.Labels{types.Wildcard: []string{types.Wildcard}},
			AppLabels:             types.Labels{types.Wildcard: []string{types.Wildcard}},
			GroupLabels:           types.Labels{types.Wildcard: []string{types.Wildcard}},
			KubernetesLabels:      types.Labels{types.Wildcard: []string{types.Wildcard}},
			DatabaseServiceLabels: types.Labels{types.Wildcard: []string{types.Wildcard}},
			DatabaseLabels:        types.Labels{types.Wildcard: []string{types.Wildcard}},
			Rules: []types.Rule{
				types.NewRule(types.KindRole, RW()),
				types.NewRule(types.KindAuthConnector, RW()),
				types.NewRule(types.KindSession, RO()),
				types.NewRule(types.KindTrustedCluster, RW()),
				types.NewRule(types.KindEvent, RO()),
				types.NewRule(types.KindClusterAuthPreference, RW()),
				types.NewRule(types.KindClusterNetworkingConfig, RW()),
				types.NewRule(types.KindSessionRecordingConfig, RW()),
				types.NewRule(types.KindUIConfig, RW()),
				types.NewRule(types.KindApp, RW()),
				types.NewRule(types.KindDatabase, RW()),
				types.NewRule(types.KindLock, RW()),
				types.NewRule(types.KindToken, RW()),
				types.NewRule(types.KindConnectionDiagnostic, RW()),
				types.NewRule(types.KindKubernetesCluster, RW()),
				types.NewRule(types.KindSessionTracker, RO()),
				types.NewRule(types.KindUserGroup, RW()),
				types.NewRule(types.KindSAMLIdPServiceProvider, RW()),
			},
			JoinSessions: []*types.SessionJoinPolicy{
				{
					Name:  "foo",
					Roles: []string{"*"},
					Kinds: []string{string(types.SSHSessionKind)},
					Modes: []string{string(types.SessionPeerMode)},
				},
			},
		},
	})
	return role
}

// RoleForCertAuthority creates role using types.CertAuthority.
func RoleForCertAuthority(ca types.CertAuthority) types.Role {
	role, _ := types.NewRole(RoleNameForCertAuthority(ca.GetClusterName()), types.RoleSpecV6{
		Options: types.RoleOptions{
			MaxSessionTTL: types.NewDuration(defaults.MaxCertDuration),
		},
		Allow: types.RoleConditions{
			Namespaces:       []string{defaults.Namespace},
			NodeLabels:       types.Labels{types.Wildcard: []string{types.Wildcard}},
			AppLabels:        types.Labels{types.Wildcard: []string{types.Wildcard}},
			KubernetesLabels: types.Labels{types.Wildcard: []string{types.Wildcard}},
			DatabaseLabels:   types.Labels{types.Wildcard: []string{types.Wildcard}},
			Rules:            types.CopyRulesSlice(DefaultCertAuthorityRules),
		},
	})
	return role
}

// ValidateRoleName checks that the role name is allowed to be created.
func ValidateRoleName(role types.Role) error {
	// System role names are not allowed.
	systemRoles := types.SystemRoles([]types.SystemRole{
		types.SystemRole(role.GetMetadata().Name),
	})
	if err := systemRoles.Check(); err == nil {
		return trace.BadParameter("reserved role: %s", role.GetMetadata().Name)
	}
	return nil
}

type validateRoleOptions struct {
	warningReporter func(error)
}

func defaultValidateRoleOptions() validateRoleOptions {
	return validateRoleOptions{
		warningReporter: func(error) {},
	}
}

type validateRoleOption func(*validateRoleOptions)

// withWarningReporter is meant for tests to assert the presence of expected
// warnings.
func withWarningReporter(f func(error)) validateRoleOption {
	return func(opts *validateRoleOptions) {
		opts.warningReporter = f
	}
}

// ValidateRole parses, validates, and sets default values on a role.
func ValidateRole(r types.Role, opts ...validateRoleOption) error {
	options := defaultValidateRoleOptions()
	for _, opt := range opts {
		opt(&options)
	}

	if err := CheckAndSetDefaults(r); err != nil {
		return trace.Wrap(err)
	}

	// Expression parsers in new versions sometimes get smarter/more strict and
	// catch more syntax or type errors that previously would only be caught
	// when they were evaluated. To avoid any possibility of bricking a cluster
	// by making all roles invalid due to a buggy expression that may not even
	// be used, only log expression parse errors as a warning.
	if err := validateRoleExpressions(r); err != nil {
		options.warningReporter(err)
		slog.WarnContext(context.Background(), "Detected invalid role", "role", r.GetName(), "error", err)
	}
	return nil
}

func validateRoleExpressions(r types.Role) error {
	var errs []error
	for _, condition := range []struct {
		name      string
		condition types.RoleConditionType
	}{
		{"allow", types.Allow},
		{"deny", types.Deny},
	} {
		for _, rule := range r.GetRules(condition.condition) {
			if err := validateRule(rule); err != nil {
				err = trace.BadParameter("parsing %s rule: %v", condition.name, err)
				errs = append(errs, err)
			}
		}

		for _, values := range []struct {
			name   string
			values []string
		}{
			{"logins", r.GetLogins(condition.condition)},
			{"windows_desktop_logins", r.GetWindowsLogins(condition.condition)},
			{"aws_role_arns", r.GetAWSRoleARNs(condition.condition)},
			{"azure_identities", r.GetAzureIdentities(condition.condition)},
			{"gcp_service_accounts", r.GetGCPServiceAccounts(condition.condition)},
			{"kubernetes_groups", r.GetKubeGroups(condition.condition)},
			{"kubernetes_users", r.GetKubeUsers(condition.condition)},
			{"db_names", r.GetDatabaseNames(condition.condition)},
			{"db_users", r.GetDatabaseUsers(condition.condition)},
			{"host_groups", r.GetHostGroups(condition.condition)},
			{"host_sudoers", r.GetHostSudoers(condition.condition)},
			{"desktop_groups", r.GetDesktopGroups(condition.condition)},
			{"impersonate.users", r.GetImpersonateConditions(condition.condition).Users},
			{"impersonate.roles", r.GetImpersonateConditions(condition.condition).Roles},
		} {
			for _, value := range values.values {
				_, err := parse.NewTraitsTemplateExpression(value)
				if err != nil {
					err = trace.BadParameter("parsing %s.%s expression: %v", condition.name, values.name, err)
					errs = append(errs, err)
				}
			}
		}

		for _, ks := range r.GetKubeResources(condition.condition) {
			_, err := parse.NewTraitsTemplateExpression(ks.Namespace)
			if err != nil {
				err = trace.BadParameter("parsing %s.kubernetes_resources.namespace expression: %v", condition.name, err)
				errs = append(errs, err)
			}
			_, err = parse.NewTraitsTemplateExpression(ks.Name)
			if err != nil {
				err = trace.BadParameter("parsing %s.kubernetes_resources.name expression: %v", condition.name, err)
				errs = append(errs, err)
			}
			for _, verb := range ks.Verbs {
				_, err = parse.NewTraitsTemplateExpression(verb)
				if err != nil {
					err = trace.BadParameter("parsing %s.kubernetes_resources.verbs expression: %v", condition.name, err)
					errs = append(errs, err)
				}
			}
		}

		for _, labels := range []struct {
			name string
			kind string
		}{
			{"cluster_labels", types.KindRemoteCluster},
			{"node_labels", types.KindNode},
			{"kubernetes_labels", types.KindKubernetesCluster},
			{"app_labels", types.KindApp},
			{"db_labels", types.KindDatabase},
			{"db_service_labels", types.KindDatabaseService},
			{"windows_desktop_labels", types.KindWindowsDesktop},
			{"windows_desktop_labels", types.KindDynamicWindowsDesktop},
			{"group_labels", types.KindUserGroup},
		} {
			labelMatchers, err := r.GetLabelMatchers(condition.condition, labels.kind)
			if err != nil {
				return trace.Wrap(err)
			}
			for _, labelValues := range labelMatchers.Labels {
				for _, label := range labelValues {
					_, err := parse.NewTraitsTemplateExpression(label)
					if err != nil {
						err = trace.BadParameter("parsing %s.%s template expression: %v", condition.name, labels.name, err)
						errs = append(errs, err)
					}
				}
			}
			if len(labelMatchers.Expression) > 0 {
				if _, err := parseLabelExpression(labelMatchers.Expression); err != nil {
					err = trace.BadParameter("parsing %s.%s_expression: %v", condition.name, labels.name, err)
					errs = append(errs, err)
				}
			}
		}
	}
	return trace.NewAggregate(errs...)
}

// validateRule parses the where and action fields to validate the rule.
func validateRule(r types.Rule) error {
	if len(r.Where) != 0 {
		parser, err := NewWhereParser(&Context{})
		if err != nil {
			return trace.Wrap(err)
		}
		_, err = parser.Parse(r.Where)
		if err != nil {
			return trace.BadParameter("could not parse 'where' rule: %q, error: %v", r.Where, err)
		}
	}

	if len(r.Actions) != 0 {
		parser, err := NewActionsParser(&Context{})
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

func filterInvalidUnixLogins(candidates []string) []string {
	var output []string

	for _, candidate := range candidates {
		if utils.IsValidUnixUser(candidate) {
			// A valid variable was found in the traits, append it to the list of logins.
			output = append(output, candidate)
			continue
		}

		// Log any invalid logins which were added by a user but ignore any
		// Teleport internal logins which are known to be invalid.
		if candidate != teleport.SSHSessionJoinPrincipal && !strings.HasPrefix(candidate, "no-login-") {
			slog.DebugContext(context.Background(), "Skipping invalid Unix login.", "login", candidate)
		}
	}
	return output
}

func filterInvalidWindowsLogins(candidates []string) []string {
	var output []string

	// https://docs.microsoft.com/en-us/previous-versions/windows/it-pro/windows-2000-server/bb726984(v=technet.10)
	const invalidChars = `"/\[]:;|=,+*?<>`

	for _, candidate := range candidates {
		if strings.ContainsAny(candidate, invalidChars) {
			slog.DebugContext(context.Background(), "Skipping invalid Windows login.", "login", candidate)
			continue
		}

		output = append(output, candidate)
	}

	return output
}

func warnInvalidAzureIdentities(candidates []string) {
	for _, candidate := range candidates {
		if !MatchValidAzureIdentity(candidate) {
			slog.WarnContext(context.Background(), "Invalid format of Azure identity", "identity", candidate)
		}
	}
}

// ParseResourceID from Azure SDK is too lenient; we use a strict regexp instead.
var azureIdentityPattern = regexp.MustCompile(`(?i)^/subscriptions/([a-fA-F0-9-]+)/resourceGroups/([0-9a-zA-Z-_]+)/providers/Microsoft\.ManagedIdentity/userAssignedIdentities/([0-9a-zA-Z-_]+)$`)

func MatchValidAzureIdentity(identity string) bool {
	if identity == types.Wildcard {
		return true
	}

	return azureIdentityPattern.MatchString(identity)
}

// ApplyTraits applies the passed in traits to any variables within the role
// and returns itself.
func ApplyTraits(r types.Role, traits map[string][]string) (types.Role, error) {
	for _, condition := range []types.RoleConditionType{types.Allow, types.Deny} {

		inLogins := r.GetLogins(condition)
		outLogins := applyValueTraitsSlice(inLogins, traits, "login")
		outLogins = filterInvalidUnixLogins(outLogins)
		r.SetLogins(condition, apiutils.Deduplicate(outLogins))

		inWindowsLogins := r.GetWindowsLogins(condition)
		outWindowsLogins := applyValueTraitsSlice(inWindowsLogins, traits, "windows_login")
		outWindowsLogins = filterInvalidWindowsLogins(outWindowsLogins)
		r.SetWindowsLogins(condition, apiutils.Deduplicate(outWindowsLogins))

		inRoleARNs := r.GetAWSRoleARNs(condition)
		outRoleARNs := applyValueTraitsSlice(inRoleARNs, traits, "AWS role ARN")
		r.SetAWSRoleARNs(condition, apiutils.Deduplicate(outRoleARNs))

		inAzureIdentities := r.GetAzureIdentities(condition)
		outAzureIdentities := applyValueTraitsSlice(inAzureIdentities, traits, "Azure identity")
		warnInvalidAzureIdentities(outAzureIdentities)
		r.SetAzureIdentities(condition, apiutils.Deduplicate(outAzureIdentities))

		inGCPAccounts := r.GetGCPServiceAccounts(condition)
		outGCPAccounts := applyValueTraitsSlice(inGCPAccounts, traits, "GCP service account")
		r.SetGCPServiceAccounts(condition, apiutils.Deduplicate(outGCPAccounts))

		// apply templates to kubernetes groups
		inKubeGroups := r.GetKubeGroups(condition)
		outKubeGroups := applyValueTraitsSlice(inKubeGroups, traits, "kube group")
		r.SetKubeGroups(condition, apiutils.Deduplicate(outKubeGroups))

		// apply templates to kubernetes users
		inKubeUsers := r.GetKubeUsers(condition)
		outKubeUsers := applyValueTraitsSlice(inKubeUsers, traits, "kube user")
		r.SetKubeUsers(condition, apiutils.Deduplicate(outKubeUsers))

		// apply templates to database names
		inDbNames := r.GetDatabaseNames(condition)
		outDbNames := applyValueTraitsSlice(inDbNames, traits, "database name")
		r.SetDatabaseNames(condition, apiutils.Deduplicate(outDbNames))

		// apply templates to database users
		inDbUsers := r.GetDatabaseUsers(condition)
		outDbUsers := applyValueTraitsSlice(inDbUsers, traits, "database user")
		r.SetDatabaseUsers(condition, apiutils.Deduplicate(outDbUsers))

		// apply templates to database roles
		inDbRoles := r.GetDatabaseRoles(condition)
		outDbRoles := applyValueTraitsSlice(inDbRoles, traits, "database role")
		r.SetDatabaseRoles(condition, apiutils.Deduplicate(outDbRoles))

		githubPermissions := r.GetGitHubPermissions(condition)
		for i, perm := range githubPermissions {
			githubPermissions[i].Organizations = applyValueTraitsSlice(perm.Organizations, traits, "github organizations")
		}
		r.SetGitHubPermissions(condition, githubPermissions)

		var out []types.KubernetesResource
		// we access the resources in the role using the role conditions
		// to avoid receiving the compatibility resources added in GetKubernetesResources
		// for roles <v7
		for _, rec := range r.GetRoleConditions(condition).KubernetesResources {
			namespaces := applyValueTraitsSlice([]string{rec.Namespace}, traits, "kubernetes resource namespace")
			if rec.Namespace == "" {
				namespaces = []string{""}
			}
			names := applyValueTraitsSlice([]string{rec.Name}, traits, "kubernetes resource name")
			if rec.Name == "" {
				names = []string{""}
			}
			verbs := applyValueTraitsSlice(rec.Verbs, traits, "kubernetes resource verb")
			for _, namespace := range namespaces {
				for _, name := range names {
					out = append(out, types.KubernetesResource{
						Kind:      rec.Kind,
						Namespace: namespace,
						Name:      name,
						Verbs:     verbs,
					})
				}
			}
		}
		r.SetKubeResources(condition, out)

		for _, kind := range []string{
			types.KindRemoteCluster,
			types.KindNode,
			types.KindKubernetesCluster,
			types.KindApp,
			types.KindDatabase,
			types.KindDatabaseService,
			types.KindWindowsDesktop,
			types.KindUserGroup,
		} {
			labelMatchers, err := r.GetLabelMatchers(condition, kind)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			// Only labelMatchers.Labels is templated, if empty we can skip
			// these label matchers. labelMatchers.Expression can reference user
			// traits later during the access check through the expression
			// environment, they are not templated in here.
			if len(labelMatchers.Labels) == 0 {
				continue
			}
			labelMatchers.Labels = applyLabelsTraits(labelMatchers.Labels, traits)
			if err := r.SetLabelMatchers(condition, kind, labelMatchers); err != nil {
				return nil, trace.Wrap(err)
			}
		}

		r.SetHostGroups(condition,
			applyValueTraitsSlice(r.GetHostGroups(condition), traits, "host_groups"))

		r.SetHostSudoers(condition,
			applyValueTraitsSlice(r.GetHostSudoers(condition), traits, "host_sudoers"))

		r.SetDesktopGroups(condition,
			applyValueTraitsSlice(r.GetDesktopGroups(condition), traits, "desktop_groups"))

		options := r.GetOptions()
		for i, ext := range options.CertExtensions {
			vals, err := ApplyValueTraits(ext.Value, traits)
			if err != nil && !trace.IsNotFound(err) {
				slog.WarnContext(context.Background(), "Failed to apply trait to cert_extensions.value", "error", err)
				continue
			}
			if len(vals) != 0 {
				options.CertExtensions[i].Value = vals[0]
			}
		}

		// apply templates to impersonation conditions
		inCond := r.GetImpersonateConditions(condition)
		var outCond types.ImpersonateConditions
		outCond.Users = applyValueTraitsSlice(inCond.Users, traits, "impersonate user")
		outCond.Roles = applyValueTraitsSlice(inCond.Roles, traits, "impersonate role")
		outCond.Users = apiutils.Deduplicate(outCond.Users)
		outCond.Roles = apiutils.Deduplicate(outCond.Roles)
		outCond.Where = inCond.Where
		r.SetImpersonateConditions(condition, outCond)
	}

	return r, nil
}

// applyValueTraitsSlice iterates over a slice of input strings, calling
// ApplyValueTraits on each.
func applyValueTraitsSlice(inputs []string, traits map[string][]string, fieldName string) []string {
	var output []string
	for _, value := range inputs {
		outputs, err := ApplyValueTraits(value, traits)
		if err != nil {
			if !trace.IsNotFound(err) {
				slog.DebugContext(context.Background(), "Skipping trait value.", "field", fieldName, "value", value, "error", err)
			}
			continue
		}
		output = append(output, outputs...)
	}
	return output
}

// applyLabelsTraits interpolates variables based on the templates
// and traits from identity provider. For example:
//
// cluster_labels:
//
//	env: ['{{external.groups}}']
//
// and groups: ['admins', 'devs']
//
// will be interpolated to:
//
// cluster_labels:
//
//	env: ['admins', 'devs']
func applyLabelsTraits(inLabels types.Labels, traits map[string][]string) types.Labels {
	outLabels := make(types.Labels, len(inLabels))
	// every key will be mapped to the first value
	for key, vals := range inLabels {
		keyVars, err := ApplyValueTraits(key, traits)
		if err != nil {
			// empty key will not match anything
			slog.DebugContext(context.Background(), "Setting empty node label pair", "key", key, "values", vals, "error", err)
			keyVars = []string{""}
		}

		var values []string
		for _, val := range vals {
			valVars, err := ApplyValueTraits(val, traits)
			if err != nil {
				slog.DebugContext(context.Background(), "Setting empty node label value", "key", key, "value", val, "error", err)
				// empty value will not match anything
				valVars = []string{""}
			}
			values = append(values, valVars...)
		}
		outLabels[keyVars[0]] = apiutils.Deduplicate(values)
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
	expr, err := parse.NewTraitsTemplateExpression(val)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	varValidation := func(namespace string, name string) error {
		// verify that internal traits match the supported variables
		if namespace == teleport.TraitInternalPrefix {
			switch name {
			case constants.TraitLogins, constants.TraitWindowsLogins,
				constants.TraitKubeGroups, constants.TraitKubeUsers,
				constants.TraitDBNames, constants.TraitDBUsers, constants.TraitDBRoles,
				constants.TraitAWSRoleARNs, constants.TraitAzureIdentities,
				constants.TraitGCPServiceAccounts, constants.TraitJWT,
				constants.TraitGitHubOrgs:
			default:
				return trace.BadParameter("unsupported variable %q", name)
			}
		}
		// The "external" trait namespace is explicitly allowed to reference
		// "internal" traits listed above. This is for multiple reasons:
		// - back compat, it's always been this way
		// - IdPs are allowed to set those trait names so it wouldn't make
		//   sense to block them when referenced via "external"
		// - The user resource spec.traits can include the "internal" trait
		//   names listed above, as well as any other trait name - but other
		//   trait names must be referenced in the "external" namespace. It
		//   wouldn't make a lot of sense to change the namespace
		//   based only on the trait name, especially given that we tend to
		//   expand the list of "internal" traits, and that would be a breaking
		//   change if someone already referred to one of the "new" internal
		//   traits in the "external" namespace.
		return nil
	}
	interpolated, err := expr.Interpolate(varValidation, traits)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if len(interpolated) == 0 {
		return nil, trace.NotFound("variable interpolation result is empty")
	}
	return interpolated, nil
}

// ruleScore is a sorting score of the rule, the larger the score, the more
// specific the rule is
func ruleScore(r *types.Rule) int {
	score := 0
	// wildcard rules are less specific
	if slices.Contains(r.Resources, types.Wildcard) {
		score -= 4
	} else if len(r.Resources) == 1 {
		// rules that match specific resource are more specific than
		// fields that match several resources
		score += 2
	}
	// rules that have wildcard verbs are less specific
	if slices.Contains(r.Verbs, types.Wildcard) {
		score -= 2
	}
	// rules that supply 'where' or 'actions' are more specific
	// having 'where' or 'actions' is more important than
	// whether the rules are wildcard or not, so here we have +8 vs
	// -4 and -2 score penalty for wildcards in resources and verbs
	if len(r.Where) > 0 {
		score += 8
	}
	// rules featuring actions are more specific
	if len(r.Actions) > 0 {
		score += 8
	}
	return score
}

// CompareRuleScore returns true if the first rule is more specific than the other.
//
// * nRule matching wildcard resource is less specific
// than same rule matching specific resource.
// * Rule that has wildcard verbs is less specific
// than the same rules matching specific verb.
// * Rule that has where section is more specific
// than the same rule without where section.
// * Rule that has actions list is more specific than
// rule without actions list.
func CompareRuleScore(r *types.Rule, o *types.Rule) bool {
	return ruleScore(r) > ruleScore(o)
}

// RuleSet maps resource to a set of rules defined for it
type RuleSet map[string][]types.Rule

// MakeRuleSet creates a new rule set from a list
func MakeRuleSet(rules []types.Rule) RuleSet {
	set := make(RuleSet)
	for _, rule := range rules {
		for _, resource := range rule.Resources {
			set[resource] = append(set[resource], rule)
		}
	}
	for resource := range set {
		rules := set[resource]
		// sort rules by most specific rule, the rule that has actions
		// is more specific than the one that has no actions
		sort.Slice(rules, func(i, j int) bool {
			return CompareRuleScore(&rules[i], &rules[j])
		})
		set[resource] = rules
	}
	return set
}

// Match tests if the resource name and verb are in a given list of rules.
// More specific rules will be matched first. See Rule.IsMoreSpecificThan
// for exact specs on whether the rule is more or less specific.
//
// Specifying order solves the problem on having multiple rules, e.g. one wildcard
// rule can override more specific rules with 'where' sections that can have
// 'actions' lists with side effects that will not be triggered otherwise.
func (set RuleSet) Match(whereParser predicate.Parser, actionsParser predicate.Parser, resource string, verb string) (bool, error) {
	// empty set matches nothing
	if len(set) == 0 {
		return false, nil
	}

	// check for matching resource by name
	// the most specific rule should win
	rules := set[resource]
	for _, rule := range rules {
		match, err := matchesWhere(&rule, whereParser)
		if err != nil {
			return false, trace.Wrap(err)
		}
		if match && (rule.HasVerb(types.Wildcard) || rule.HasVerb(verb)) {
			if err := processActions(&rule, actionsParser); err != nil {
				return true, trace.Wrap(err)
			}
			return true, nil
		}
	}

	// check for wildcard resource matcher
	for _, rule := range set[types.Wildcard] {
		match, err := matchesWhere(&rule, whereParser)
		if err != nil {
			return false, trace.Wrap(err)
		}
		if match && (rule.HasVerb(types.Wildcard) || rule.HasVerb(verb)) {
			if err := processActions(&rule, actionsParser); err != nil {
				return true, trace.Wrap(err)
			}
			return true, nil
		}
	}

	return false, nil
}

// matchesWhere returns true if Where rule matches.
// Empty Where block always matches.
func matchesWhere(r *types.Rule, parser predicate.Parser) (bool, error) {
	if r.Where == "" {
		return true, nil
	}
	ifn, err := parser.Parse(r.Where)
	if err != nil {
		return false, trace.Wrap(err)
	}
	fn, ok := ifn.(predicate.BoolPredicate)
	if !ok {
		return false, trace.BadParameter("invalid predicate type for where expression: %v", r.Where)
	}
	return fn(), nil
}

// processActions processes actions specified for this rule
func processActions(r *types.Rule, parser predicate.Parser) error {
	for _, action := range r.Actions {
		ifn, err := parser.Parse(action)
		if err != nil {
			return trace.Wrap(err)
		}
		fn, ok := ifn.(predicate.BoolPredicate)
		if !ok {
			return trace.BadParameter("invalid predicate type for action expression: %v", action)
		}
		fn()
	}
	return nil
}

// Slice returns slice from a set
func (set RuleSet) Slice() []types.Rule {
	var out []types.Rule
	for _, rules := range set {
		out = append(out, rules...)
	}
	return out
}

// RoleFromSpec returns new Role created from spec
func RoleFromSpec(name string, spec types.RoleSpecV6) (types.Role, error) {
	role, err := types.NewRole(name, spec)
	return role, trace.Wrap(err)
}

// RoleSetFromSpec returns a new RoleSet from spec
func RoleSetFromSpec(name string, spec types.RoleSpecV6) (RoleSet, error) {
	role, err := RoleFromSpec(name, spec)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return NewRoleSet(role), nil
}

// RW is a shortcut that returns all CRUD verbs.
func RW() []string {
	return []string{types.VerbList, types.VerbCreate, types.VerbRead, types.VerbUpdate, types.VerbDelete}
}

// RO is a shortcut that returns read only verbs that provide access to secrets.
func RO() []string {
	return []string{types.VerbList, types.VerbRead}
}

// ReadNoSecrets is a shortcut that returns read only verbs that do not
// provide access to secrets.
func ReadNoSecrets() []string {
	return []string{types.VerbList, types.VerbReadNoSecrets}
}

// RoleGetter is an interface that defines GetRole method
type RoleGetter interface {
	// GetRole returns role by name
	GetRole(ctx context.Context, name string) (types.Role, error)
}

// ExtractFromCertificate will extract roles and traits from a *ssh.Certificate.
func ExtractFromCertificate(cert *ssh.Certificate) ([]string, wrappers.Traits, error) {
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
func ExtractFromIdentity(ctx context.Context, access UserGetter, identity tlsca.Identity) ([]string, wrappers.Traits, error) {
	// Legacy certs are not encoded with roles or traits,
	// so we fallback to the traits and roles in the backend.
	// empty traits are a valid use case in standard certs,
	// so we only check for whether roles are empty.
	if len(identity.Groups) == 0 {
		u, err := access.GetUser(ctx, identity.Username, false)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}

		const msg = "Failed to find roles in x509 identity. Fetching " +
			"from backend. If the identity provider allows username changes, this can " +
			"potentially allow an attacker to change the role of the existing user."
		slog.WarnContext(ctx, msg, "username", identity.Username)
		return u.GetRoles(), u.GetTraits(), nil
	}

	return identity.Groups, identity.Traits, nil
}

// FetchRoleList fetches roles by their names, applies the traits to role
// variables, and returns the list
func FetchRoleList(roleNames []string, access RoleGetter, traits map[string][]string) (RoleSet, error) {
	var roles []types.Role

	for _, roleName := range roleNames {
		role, err := access.GetRole(context.TODO(), roleName)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		role, err = ApplyTraits(role, traits)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		roles = append(roles, role)
	}

	return roles, nil
}

// FetchRoles fetches roles by their names, applies the traits to role
// variables, and returns the RoleSet. Adds runtime roles like the default
// implicit role to RoleSet.
func FetchRoles(roleNames []string, access RoleGetter, traits map[string][]string) (RoleSet, error) {
	roles, err := FetchRoleList(roleNames, access, traits)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return NewRoleSet(roles...), nil
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

func ExtractAllowedResourcesFromCert(cert *ssh.Certificate) ([]types.ResourceID, error) {
	allowedResourcesStr, ok := cert.Extensions[teleport.CertExtensionAllowedResources]
	if !ok {
		// if not present in the cert, there are no resource-based restrictions
		return nil, nil
	}
	allowedResources, err := types.ResourceIDsFromString(allowedResourcesStr)
	return allowedResources, trace.Wrap(err)
}

// NewRoleSet returns new RoleSet based on the roles
func NewRoleSet(roles ...types.Role) RoleSet {
	// unauthenticated Nop role should not have any privileges
	// by default, otherwise it is too permissive
	if len(roles) == 1 && roles[0].GetName() == string(types.RoleNop) {
		return roles
	}
	return append(roles, NewImplicitRole())
}

// RoleSet is a set of roles that implements access control functionality
type RoleSet []types.Role

// EnumerationResult is a result of enumerating a role set against some property, e.g. allowed names or logins.
type EnumerationResult struct {
	allowedDeniedMap map[string]bool
	wildcardAllowed  bool
	wildcardDenied   bool
}

func (result *EnumerationResult) filtered(value bool) []string {
	var filtered []string

	for entity, allow := range result.allowedDeniedMap {
		if allow == value {
			filtered = append(filtered, entity)
		}
	}

	sort.Strings(filtered)

	return filtered
}

// Denied returns all explicitly denied entities.
func (result *EnumerationResult) Denied() []string {
	return result.filtered(false)
}

// Allowed returns all known allowed entities.
func (result *EnumerationResult) Allowed() []string {
	if result.WildcardDenied() {
		return nil
	}
	return result.filtered(true)
}

// WildcardAllowed is true if the * entity is allowed for a given rule set.
func (result *EnumerationResult) WildcardAllowed() bool {
	return result.wildcardAllowed && !result.wildcardDenied
}

// WildcardDenied is true if the * entity is denied for a given rule set.
func (result *EnumerationResult) WildcardDenied() bool {
	return result.wildcardDenied
}

// NewEnumerationResult returns new EnumerationResult.
func NewEnumerationResult() EnumerationResult {
	return EnumerationResult{
		allowedDeniedMap: map[string]bool{},
		wildcardAllowed:  false,
		wildcardDenied:   false,
	}
}

// MatchNamespace returns true if given list of namespace matches
// target namespace, wildcard matches everything.
func MatchNamespace(selectors []string, namespace string) (bool, string) {
	for _, n := range selectors {
		if n == namespace || n == types.Wildcard {
			return true, "matched"
		}
	}
	return false, fmt.Sprintf("no match, role selectors %v, server namespace: %v", selectors, namespace)
}

// MatchAWSRoleARN returns true if provided role ARN matches selectors.
func MatchAWSRoleARN(selectors []string, roleARN string) (bool, string) {
	for _, l := range selectors {
		if l == roleARN {
			return true, "matched"
		}
	}
	return false, fmt.Sprintf("no match, role selectors %v, role ARN: %v", selectors, roleARN)
}

// MatchAzureIdentity returns true if provided Azure identity matches selectors.
func MatchAzureIdentity(selectors []string, identity string, matchWildcard bool) (bool, string) {
	identity = strings.ToLower(identity)
	for _, l := range selectors {
		if strings.ToLower(l) == identity {
			return true, "element matched"
		}
		if matchWildcard && l == types.Wildcard {
			return true, "wildcard matched"
		}
	}
	return false, fmt.Sprintf("no match, role selectors %v, identity: %v", selectors, identity)
}

// MatchGCPServiceAccount returns true if provided GCP service account matches selectors.
func MatchGCPServiceAccount(selectors []string, account string, matchWildcard bool) (bool, string) {
	for _, l := range selectors {
		if l == account {
			return true, "element matched"
		}
		if matchWildcard && l == types.Wildcard {
			return true, "wildcard matched"
		}
	}
	return false, fmt.Sprintf("no match, role selectors %v, identity: %v", selectors, account)
}

// MatchDatabaseName returns true if provided database name matches selectors.
func MatchDatabaseName(selectors []string, name string) (bool, string) {
	for _, n := range selectors {
		if n == name || n == types.Wildcard {
			return true, "matched"
		}
	}
	return false, fmt.Sprintf("no match, role selectors %v, database name: %v", selectors, name)
}

// MatchDatabaseUser returns true if provided database user matches selectors.
func MatchDatabaseUser(selectors []string, user string, matchWildcard, caseFold bool) (bool, string) {
	for _, u := range selectors {
		if caseFold {
			if strings.EqualFold(u, user) {
				return true, "matched"
			}
		} else if u == user {
			return true, "matched"
		}
		if matchWildcard && u == types.Wildcard {
			return true, "matched"
		}
	}
	return false, fmt.Sprintf("no match, role selectors %v, database user: %v", selectors, user)
}

// MatchLabels matches selector against target. Empty selector matches
// nothing, wildcard matches everything.
func MatchLabels(selector types.Labels, target map[string]string) (bool, string, error) {
	return MatchLabelGetter(selector, mapLabelGetter(target))
}

// LabelGetter allows retrieving a particular label by name or retreiving all
// labels at once. Prefer to use GetLabel when possible to avoid unnecessary
// copies.
type LabelGetter interface {
	GetLabel(key string) (value string, ok bool)
	GetAllLabels() map[string]string
}

type mapLabelGetter map[string]string

func (m mapLabelGetter) GetLabel(key string) (value string, ok bool) {
	v, ok := m[key]
	return v, ok
}

func (m mapLabelGetter) GetAllLabels() map[string]string {
	return map[string]string(m)
}

// MatchLabelGetter matches selector against labelGetter. Empty selector matches
// nothing, wildcard matches everything.
func MatchLabelGetter(selector types.Labels, labelGetter LabelGetter) (bool, string, error) {
	// Empty selector matches nothing.
	if len(selector) == 0 {
		return false, "no match, empty selector", nil
	}

	// *: * matches everything even empty target set.
	selectorValues := selector[types.Wildcard]
	if len(selectorValues) == 1 && selectorValues[0] == types.Wildcard {
		return true, "matched", nil
	}

	// Perform full match.
	for key, selectorValues := range selector {
		targetVal, hasKey := labelGetter.GetLabel(key)
		if !hasKey {
			return false, fmt.Sprintf("no key match: '%v'", key), nil
		}

		if slices.Contains(selectorValues, types.Wildcard) {
			continue
		}

		result, err := utils.SliceMatchesRegex(targetVal, selectorValues)
		if err != nil {
			return false, "", trace.Wrap(err)
		} else if !result {
			return false, fmt.Sprintf("no value match: got '%v' want: '%v'", targetVal, selectorValues), nil
		}
	}

	return true, "matched", nil
}

// RoleNames returns a slice with role names. Removes runtime roles like
// the default implicit role.
func (set RoleSet) RoleNames() []string {
	out := make([]string, 0, len(set))
	for _, r := range set {
		if r.GetName() == constants.DefaultImplicitRole {
			continue
		}
		out = append(out, r.GetName())
	}
	return out
}

// Roles returns the list underlying roles this RoleSet is based on.
func (set RoleSet) Roles() []types.Role {
	return append([]types.Role{}, set...)
}

// HasRole checks if the role set has the role
func (set RoleSet) HasRole(role string) bool {
	for _, r := range set {
		if r.GetName() == role {
			return true
		}
	}
	return false
}

// WithoutImplicit returns this role set with default implicit role filtered out.
func (set RoleSet) WithoutImplicit() (out RoleSet) {
	for _, r := range set {
		if r.GetName() == constants.DefaultImplicitRole {
			continue
		}
		out = append(out, r)
	}
	return out
}

// PinSourceIP determines if the role set should use source IP pinning.
// If one or more roles in the set requires IP pinning then it will be enabled.
func (set RoleSet) PinSourceIP() bool {
	for _, role := range set {
		if role.GetOptions().PinSourceIP {
			return true
		}
	}
	return false
}

// GetAccessState returns the AccessState, setting [AccessState.MFARequired]
// according to the user's roles and cluster auth preference.
func (set RoleSet) GetAccessState(authPref readonly.AuthPreference) AccessState {
	return AccessState{
		MFARequired: set.getMFARequired(authPref.GetRequireMFAType()),
		// We don't set EnableDeviceVerification here, as both it and DeviceVerified
		// should be set in tandem.
	}
}

func (set RoleSet) getMFARequired(clusterRequireMFAType types.RequireMFAType) MFARequired {
	// MFA is always required according to the cluster auth pref.
	if clusterRequireMFAType.IsSessionMFARequired() {
		return MFARequiredAlways
	}

	// If MFA requirement is the same across all roles, we can skip the per-role check.
	// Set mfaRequired to the first role's requirement, then check if all other roles match.
	if len(set) > 0 {
		rolesMFARequired := set[0].GetOptions().RequireMFAType.IsSessionMFARequired()
		for _, role := range set[1:] {
			if role.GetOptions().RequireMFAType.IsSessionMFARequired() != rolesMFARequired {
				// This role differs from the MFA requirement of the other roles, return per-role.
				return MFARequiredPerRole
			}
		}

		if rolesMFARequired {
			return MFARequiredAlways
		}
	}

	// No roles to check or no roles require MFA.
	return MFARequiredNever
}

// PrivateKeyPolicy returns the enforced private key policy for this role set.
func (set RoleSet) PrivateKeyPolicy(authPreferencePolicy keys.PrivateKeyPolicy) (keys.PrivateKeyPolicy, error) {
	policySet := []keys.PrivateKeyPolicy{authPreferencePolicy}
	for _, role := range set {
		policySet = append(policySet, role.GetPrivateKeyPolicy())
	}

	return keys.PolicyThatSatisfiesSet(policySet)
}

// AdjustSessionTTL will reduce the requested ttl to the lowest max allowed TTL
// for this role set, otherwise it returns ttl unchanged
func (set RoleSet) AdjustSessionTTL(ttl time.Duration) time.Duration {
	for _, role := range set {
		maxSessionTTL := role.GetOptions().MaxSessionTTL.Value()
		if maxSessionTTL != 0 && ttl > maxSessionTTL {
			ttl = maxSessionTTL
		}
	}
	return ttl
}

// AdjustMFAVerificationInterval will reduce the requested ttl to the lowest mfa verification interval
// for this role set if the role forces MFA tap, otherwise it returns ttl unchanged
func (set RoleSet) AdjustMFAVerificationInterval(ttl time.Duration, enforce bool) time.Duration {
	for _, role := range set {
		mfaVerificationInterval := role.GetOptions().MFAVerificationInterval
		if role.GetOptions().RequireMFAType == types.RequireMFAType_OFF && !enforce {
			continue
		}
		if mfaVerificationInterval != 0 && ttl > mfaVerificationInterval {
			ttl = mfaVerificationInterval
		}
	}
	return ttl
}

// MaxConnections returns the maximum number of concurrent ssh connections
// allowed.  If MaxConnections is zero then no maximum was defined
// and the number of concurrent connections is unconstrained.
func (set RoleSet) MaxConnections() int64 {
	var mcs int64
	for _, role := range set {
		if m := role.GetOptions().MaxConnections; m != 0 && (m < mcs || mcs == 0) {
			mcs = m
		}
	}
	return mcs
}

// MaxSessions returns the maximum number of concurrent ssh sessions
// per connection.  If MaxSessions is zero then no maximum was defined
// and the number of sessions is unconstrained.
func (set RoleSet) MaxSessions() int64 {
	var ms int64
	for _, role := range set {
		if m := role.GetOptions().MaxSessions; m != 0 && (m < ms || ms == 0) {
			ms = m
		}
	}
	return ms
}

// MaxConnections returns the maximum number of concurrent Kubernetes connections
// allowed.  If MaxConnections is zero then no maximum was defined
// and the number of concurrent connections is unconstrained.
func (set RoleSet) MaxKubernetesConnections() int64 {
	var mcs int64
	for _, role := range set {
		if m := role.GetOptions().MaxKubernetesConnections; m != 0 && (m < mcs || mcs == 0) {
			mcs = m
		}
	}
	return mcs
}

// SessionPolicySets returns the list of SessionPolicySets for all roles.
func (set RoleSet) SessionPolicySets() []*types.SessionTrackerPolicySet {
	var policySets []*types.SessionTrackerPolicySet
	for _, role := range set {
		policySet := role.GetSessionPolicySet()
		policySets = append(policySets, &policySet)
	}
	return policySets
}

// AdjustClientIdleTimeout adjusts requested idle timeout
// to the lowest max allowed timeout, the most restrictive
// option will be picked, negative values will be assumed as 0
func (set RoleSet) AdjustClientIdleTimeout(timeout time.Duration) time.Duration {
	if timeout < 0 {
		timeout = 0
	}
	for _, role := range set {
		roleTimeout := role.GetOptions().ClientIdleTimeout
		// 0 means not set, so it can't be most restrictive, disregard it too
		if roleTimeout.Duration() <= 0 {
			continue
		}
		switch {
		// in case if timeout is 0, means that incoming value
		// does not restrict the idle timeout, pick any other value
		// set by the role
		case timeout == 0:
			timeout = roleTimeout.Duration()
		case roleTimeout.Duration() < timeout:
			timeout = roleTimeout.Duration()
		}
	}
	return timeout
}

// AdjustDisconnectExpiredCert adjusts the value based on the role set
// the most restrictive option will be picked
func (set RoleSet) AdjustDisconnectExpiredCert(disconnect bool) bool {
	for _, role := range set {
		if role.GetOptions().DisconnectExpiredCert.Value() {
			disconnect = true
		}
	}
	return disconnect
}

// CheckKubeGroupsAndUsers check if role can login into kubernetes
// and returns two lists of allowed groups and users
func (set RoleSet) CheckKubeGroupsAndUsers(ttl time.Duration, overrideTTL bool, matchers ...RoleMatcher) ([]string, []string, error) {
	groups := make(map[string]struct{})
	users := make(map[string]struct{})
	var matchedTTL bool
	for _, role := range set {
		ok, err := RoleMatchers(matchers).MatchAll(role, types.Allow)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}
		if !ok {
			continue
		}

		maxSessionTTL := role.GetOptions().MaxSessionTTL.Value()
		if overrideTTL || (ttl <= maxSessionTTL && maxSessionTTL != 0) {
			matchedTTL = true
			for _, group := range role.GetKubeGroups(types.Allow) {
				groups[group] = struct{}{}
			}
			for _, user := range role.GetKubeUsers(types.Allow) {
				users[user] = struct{}{}
			}
		}
	}
	for _, role := range set {
		ok, _, err := RoleMatchers(matchers).MatchAny(role, types.Deny)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}
		if !ok {
			continue
		}
		for _, group := range role.GetKubeGroups(types.Deny) {
			delete(groups, group)
		}
		for _, user := range role.GetKubeUsers(types.Deny) {
			delete(users, user)
		}
	}
	if !matchedTTL {
		return nil, nil, trace.AccessDenied("this user cannot request kubernetes access for %v", ttl)
	}
	if len(groups) == 0 && len(users) == 0 {
		return nil, nil, trace.NotFound("this user cannot request kubernetes access, has no assigned groups or users")
	}
	return utils.StringsSliceFromSet(groups), utils.StringsSliceFromSet(users), nil
}

// CheckDatabaseNamesAndUsers checks if the role has any allowed database
// names or users.
func (set RoleSet) CheckDatabaseNamesAndUsers(ttl time.Duration, overrideTTL bool) ([]string, []string, error) {
	names := make(map[string]struct{})
	users := make(map[string]struct{})
	var matchedTTL bool
	for _, role := range set {
		maxSessionTTL := role.GetOptions().MaxSessionTTL.Value()
		if overrideTTL || (ttl <= maxSessionTTL && maxSessionTTL != 0) {
			matchedTTL = true
			for _, name := range role.GetDatabaseNames(types.Allow) {
				names[name] = struct{}{}
			}
			for _, user := range role.GetDatabaseUsers(types.Allow) {
				users[user] = struct{}{}
			}
		}
	}
	for _, role := range set {
		for _, name := range role.GetDatabaseNames(types.Deny) {
			delete(names, name)
		}
		for _, user := range role.GetDatabaseUsers(types.Deny) {
			delete(users, user)
		}
	}
	if !matchedTTL {
		return nil, nil, trace.AccessDenied("this user cannot request database access for %v", ttl)
	}
	if len(names) == 0 && len(users) == 0 {
		return nil, nil, trace.NotFound("this user cannot request database access, has no assigned database names or users")
	}
	return utils.StringsSliceFromSet(names), utils.StringsSliceFromSet(users), nil
}

// CheckAWSRoleARNs returns a list of AWS role ARNs this role set is allowed to assume.
func (set RoleSet) CheckAWSRoleARNs(ttl time.Duration, overrideTTL bool) ([]string, error) {
	arns := make(map[string]struct{})
	var matchedTTL bool
	for _, role := range set {
		maxSessionTTL := role.GetOptions().MaxSessionTTL.Value()
		if overrideTTL || (ttl <= maxSessionTTL && maxSessionTTL != 0) {
			matchedTTL = true
			for _, arn := range role.GetAWSRoleARNs(types.Allow) {
				arns[arn] = struct{}{}
			}
		}
	}
	for _, role := range set {
		for _, arn := range role.GetAWSRoleARNs(types.Deny) {
			delete(arns, arn)
		}
	}
	if !matchedTTL {
		return nil, trace.AccessDenied("this user cannot request AWS management console access for %v", ttl)
	}
	if len(arns) == 0 {
		return nil, trace.NotFound("this user cannot request AWS management console, has no assigned role ARNs")
	}
	return utils.StringsSliceFromSet(arns), nil
}

// CheckAzureIdentities returns a list of Azure identities the user is allowed to assume.
func (set RoleSet) CheckAzureIdentities(ttl time.Duration, overrideTTL bool) ([]string, error) {
	identities := make(map[string]string)
	var matchedTTL bool
	for _, role := range set {
		maxSessionTTL := role.GetOptions().MaxSessionTTL.Value()
		if overrideTTL || (ttl <= maxSessionTTL && maxSessionTTL != 0) {
			matchedTTL = true
			for _, identity := range role.GetAzureIdentities(types.Allow) {
				identities[strings.ToLower(identity)] = identity
			}
		}
	}
	for _, role := range set {
		for _, identity := range role.GetAzureIdentities(types.Deny) {
			// deny * cleans options
			if identity == types.Wildcard {
				identities = make(map[string]string)
			}
			// remove particular identity
			delete(identities, strings.ToLower(identity))
		}
	}
	if !matchedTTL {
		return nil, trace.AccessDenied("this user cannot access Azure API for %v", ttl)
	}
	if len(identities) == 0 {
		return nil, trace.NotFound("this user cannot access Azure API, has no assigned identities")
	}

	out := make([]string, 0, len(identities))
	for _, identity := range identities {
		out = append(out, identity)
	}
	sort.Strings(out)
	return out, nil
}

// CheckGCPServiceAccounts returns a list of GCP service accounts this role set is allowed to assume.
func (set RoleSet) CheckGCPServiceAccounts(ttl time.Duration, overrideTTL bool) ([]string, error) {
	accounts := make(map[string]struct{})
	var matchedTTL bool
	for _, role := range set {
		maxSessionTTL := role.GetOptions().MaxSessionTTL.Value()
		if overrideTTL || (ttl <= maxSessionTTL && maxSessionTTL != 0) {
			matchedTTL = true
			for _, account := range role.GetGCPServiceAccounts(types.Allow) {
				accounts[strings.ToLower(account)] = struct{}{}
			}
		}
	}
	for _, role := range set {
		for _, account := range role.GetGCPServiceAccounts(types.Deny) {
			// deny * removes all accounts
			if account == types.Wildcard {
				accounts = make(map[string]struct{})
			}
			// remove particular account
			delete(accounts, strings.ToLower(account))
		}
	}
	if !matchedTTL {
		return nil, trace.AccessDenied("this user cannot request GCP API access for %v", ttl)
	}
	if len(accounts) == 0 {
		return nil, trace.NotFound("this user cannot request GCP API access, has no assigned service accounts")
	}
	return utils.StringsSliceFromSet(accounts), nil
}

// CheckAccessToSAMLIdP checks access to the SAML IdP.
//
//nolint:revive // Because we want this to be IdP.
func (set RoleSet) CheckAccessToSAMLIdP(authPref readonly.AuthPreference, state AccessState) error {
	ctx := context.Background()

	if authPref != nil {
		if !authPref.IsSAMLIdPEnabled() {
			return trace.AccessDenied("SAML IdP is disabled at the cluster level")
		}
	}

	if state.MFARequired == MFARequiredAlways && !state.MFAVerified {
		rbacLogger.LogAttrs(ctx, logutils.TraceLevel, "Access to SAML IdP denied, cluster requires per-session MFA")
		return trace.Wrap(ErrSessionMFARequired)
	}

	mfaAllowed := state.MFAVerified || state.MFARequired == MFARequiredNever

	for _, role := range set {
		options := role.GetOptions()

		// This should never happen, but we should make sure that we don't get a nil pointer error here.
		if options.IDP == nil || options.IDP.SAML == nil || options.IDP.SAML.Enabled == nil {
			continue
		}

		// If any role specifically denies access to the IdP, we'll return AccessDenied.
		if !options.IDP.SAML.Enabled.Value {
			return trace.AccessDenied("user has been denied access to the SAML IdP by role %s", role.GetName())
		}

		if !mfaAllowed && options.RequireMFAType.IsSessionMFARequired() {
			rbacLogger.LogAttrs(ctx, logutils.TraceLevel, "Access to SAML IdP denied, role requires per-session MFA",
				slog.String("role", role.GetName()),
			)

			return trace.Wrap(ErrSessionMFARequired)
		}
	}

	return nil
}

// CheckLoginDuration checks if role set can login up to given duration and
// returns a combined list of allowed logins.
func (set RoleSet) CheckLoginDuration(ttl time.Duration) ([]string, error) {
	logins, matchedTTL := set.GetLoginsForTTL(ttl)
	if !matchedTTL {
		return nil, trace.AccessDenied("this user cannot request a certificate for %v", ttl)
	}

	if len(logins) == 0 && !set.hasPossibleLogins() {
		// user was deliberately configured to have no login capability,
		// but ssh certificates must contain at least one valid principal.
		// we add a single distinctive value which should be unique, and
		// will never be a valid unix login (due to leading '-').
		logins = []string{constants.NoLoginPrefix + uuid.New().String()}
	}

	if len(logins) == 0 {
		return nil, trace.AccessDenied("this user cannot create SSH sessions, has no allowed logins")
	}

	return logins, nil
}

// GetAllLogins returns all valid unix logins for the RoleSet.
func (set RoleSet) GetAllLogins() []string {
	logins, _ := set.GetLoginsForTTL(0)
	return logins
}

// GetLoginsForTTL collects all logins that are valid for the given TTL.  The matchedTTL
// value indicates whether the TTL is within scope of *any* role.  This helps to distinguish
// between TTLs which are categorically invalid, and TTLs which are theoretically valid
// but happen to grant no logins.
func (set RoleSet) GetLoginsForTTL(ttl time.Duration) (logins []string, matchedTTL bool) {
	for _, role := range set {
		maxSessionTTL := role.GetOptions().MaxSessionTTL.Value()
		if ttl <= maxSessionTTL && maxSessionTTL != 0 {
			matchedTTL = true
			logins = append(logins, role.GetLogins(types.Allow)...)
		}
	}
	return apiutils.Deduplicate(logins), matchedTTL
}

func (set RoleSet) hasPossibleLogins() bool {
	for _, role := range set {
		if role.GetName() == constants.DefaultImplicitRole {
			continue
		}
		if len(role.GetLogins(types.Allow)) != 0 {
			return true
		}
	}
	return false
}

// AWSRoleARNMatcher matches a role against AWS role ARN.
type AWSRoleARNMatcher struct {
	RoleARN string
}

// Match matches AWS role ARN against provided role and condition.
func (m *AWSRoleARNMatcher) Match(role types.Role, condition types.RoleConditionType) (bool, error) {
	match, _ := MatchAWSRoleARN(role.GetAWSRoleARNs(condition), m.RoleARN)
	return match, nil
}

// String returns the matcher's string representation.
func (m *AWSRoleARNMatcher) String() string {
	return fmt.Sprintf("AWSRoleARNMatcher(RoleARN=%v)", m.RoleARN)
}

// AzureIdentityMatcher matches a role against Azure identity.
type AzureIdentityMatcher struct {
	Identity string
}

// Match matches Azure identity against provided role and condition.
func (m *AzureIdentityMatcher) Match(role types.Role, condition types.RoleConditionType) (bool, error) {
	match, _ := MatchAzureIdentity(role.GetAzureIdentities(condition), m.Identity, condition == types.Deny)
	return match, nil
}

// String returns the matcher's string representation.
func (m *AzureIdentityMatcher) String() string {
	return fmt.Sprintf("AzureIdentityMatcher(Identity=%v)", m.Identity)
}

// GCPServiceAccountMatcher matches a role against GCP service account.
type GCPServiceAccountMatcher struct {
	// ServiceAccount is a GCP service account to match, e.g. teleport@example-123456.iam.gserviceaccount.com.
	// It can also be a wildcard *, but that is only respected for Deny rules.
	ServiceAccount string
}

// Match matches GCP ServiceAccount against provided role and condition.
func (m *GCPServiceAccountMatcher) Match(role types.Role, condition types.RoleConditionType) (bool, error) {
	match, _ := MatchGCPServiceAccount(role.GetGCPServiceAccounts(condition), m.ServiceAccount, condition == types.Deny)
	return match, nil
}

// String returns the matcher's string representation.
func (m *GCPServiceAccountMatcher) String() string {
	return fmt.Sprintf("GCPServiceAccountMatcher(ServiceAccount=%v)", m.ServiceAccount)
}

// CanImpersonateSomeone returns true if this checker has any impersonation rules
func (set RoleSet) CanImpersonateSomeone() bool {
	for _, role := range set {
		cond := role.GetImpersonateConditions(types.Allow)
		if !cond.IsEmpty() {
			return true
		}
	}
	return false
}

// CheckImpersonate returns nil if this role set can impersonate
// a user and their roles, returns AccessDenied otherwise
// CheckImpersonate checks whether current user is allowed to impersonate
// users and roles
func (set RoleSet) CheckImpersonate(currentUser, impersonateUser types.User, impersonateRoles []types.Role) error {
	ctx := &impersonateContext{
		user:            currentUser,
		impersonateUser: impersonateUser,
	}
	whereParser, err := newImpersonateWhereParser(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	// check deny: a single match on a deny rule prohibits access
	for _, role := range set {
		cond := role.GetImpersonateConditions(types.Deny)
		matched, err := matchDenyImpersonateCondition(cond, impersonateUser, impersonateRoles)
		if err != nil {
			return trace.Wrap(err)
		}
		if matched {
			return trace.AccessDenied("access denied to '%s' to impersonate user '%s' and roles '%s'", currentUser.GetName(), impersonateUser.GetName(), roleNames(impersonateRoles))
		}
	}

	// check allow: if matches, allow to impersonate
	for _, role := range set {
		cond := role.GetImpersonateConditions(types.Allow)
		matched, err := matchAllowImpersonateCondition(ctx, whereParser, cond, impersonateUser, impersonateRoles)
		if err != nil {
			return trace.Wrap(err)
		}
		if matched {
			return nil
		}
	}

	return trace.AccessDenied("access denied to '%s' to impersonate user '%s' and roles '%s'", currentUser.GetName(), impersonateUser.GetName(), roleNames(impersonateRoles))
}

// CheckImpersonateRoles validates that the current user can perform role-only impersonation
// of the given roles. Role-only impersonation requires an allow rule with
// roles but no users (and no user-less deny rules). All requested roles must
// be allowed for the check to succeed.
func (set RoleSet) CheckImpersonateRoles(currentUser types.User, impersonateRoles []types.Role) error {
	ctx := &impersonateContext{
		user: currentUser,
	}
	whereParser, err := newImpersonateWhereParser(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	// TODO: Unlike regular impersonation where all requested roles must be
	// granted by a single impersonation role, it would be reasonable to
	// request several roles whose `allow` conditions are split between
	// several roles. Our initial use-case doesn't require this, so for now
	// we'll assume all requested roles must be granted by a single `allow`.

	// check deny: a single match on a deny rule prohibits access
	for _, role := range set {
		cond := role.GetImpersonateConditions(types.Deny)
		matched, err := matchDenyRoleImpersonateCondition(cond, impersonateRoles)
		if err != nil {
			return trace.Wrap(err)
		}
		if matched {
			return trace.AccessDenied("access denied to '%s' to impersonate roles '%s'", currentUser.GetName(), roleNames(impersonateRoles))
		}
	}

	// check allow: if any one Role satisfies all the role requests, allow impersonation
	for _, role := range set {
		cond := role.GetImpersonateConditions(types.Allow)
		matched, err := matchAllowRoleImpersonateCondition(ctx, whereParser, cond, impersonateRoles)
		if err != nil {
			return trace.Wrap(err)
		}
		if matched {
			return nil
		}
	}

	return trace.AccessDenied("access denied to '%s' to impersonate roles '%s'", currentUser.GetName(), roleNames(impersonateRoles))
}

// LockingMode returns the locking mode to apply with this RoleSet.
func (set RoleSet) LockingMode(defaultMode constants.LockingMode) constants.LockingMode {
	mode := defaultMode
	for _, role := range set {
		options := role.GetOptions()
		if options.Lock == constants.LockingModeStrict {
			return constants.LockingModeStrict
		}
		if options.Lock != "" {
			mode = options.Lock
		}
	}
	return mode
}

// CertificateExtensions returns the list of extensions for each role in the RoleSet
func (set RoleSet) CertificateExtensions() []*types.CertExtension {
	var exts []*types.CertExtension
	for _, role := range set {
		exts = append(exts, role.GetOptions().CertExtensions...)
	}
	return exts
}

// SessionRecordingMode returns the recording mode for a specific service.
func (set RoleSet) SessionRecordingMode(service constants.SessionRecordingService) constants.SessionRecordingMode {
	defaultValue := constants.SessionRecordingModeBestEffort
	useDefault := true

	for _, role := range set {
		recordSession := role.GetOptions().RecordSession

		// If one of the default values is "strict", set it as the value.
		if recordSession.Default == constants.SessionRecordingModeStrict {
			defaultValue = constants.SessionRecordingModeStrict
		}

		var roleMode constants.SessionRecordingMode
		switch service {
		case constants.SessionRecordingServiceSSH:
			roleMode = recordSession.SSH
		}

		switch roleMode {
		case constants.SessionRecordingModeStrict:
			// Early return as "strict" since it is the strictest value.
			return constants.SessionRecordingModeStrict
		case constants.SessionRecordingModeBestEffort:
			useDefault = false
		}
	}

	// Return the strictest default value.
	if useDefault {
		return defaultValue
	}

	return constants.SessionRecordingModeBestEffort
}

func contains[S ~[]E, E any](s S, f func(E) (bool, error)) (bool, error) {
	for i := range s {
		match, err := f(s[i])
		if err != nil {
			return false, err
		}
		if match {
			return true, nil
		}
	}
	return false, nil
}

// matchSPIFFESVIDConditions compares a slice of SPIFFE Role Conditions against
// a requested SPIFFE SVID generation. All fields within a condition must match,
// but any condition in the slice can match for the function to return true.
func matchSPIFFESVIDConditions(
	conds []*types.SPIFFERoleCondition,
	spiffeIDPath string,
	dnsSANs []string,
	ipSANs []net.IP,
) (bool, error) {
	return contains(conds, func(cond *types.SPIFFERoleCondition) (bool, error) {
		// Match SPIFFE ID path.
		match, err := utils.MatchString(spiffeIDPath, cond.Path)
		if err != nil {
			return false, trace.Wrap(err)
		}
		if !match {
			// No match - skip to next condition.
			return false, nil
		}

		// All DNS SANs requested must match one of the DNS SAN matchers in the
		// condition.
		for _, dnsSAN := range dnsSANs {
			match, err := contains(cond.DNSSANs, func(s string) (bool, error) {
				match, err := utils.MatchString(dnsSAN, s)
				if err != nil {
					return false, trace.Wrap(err)
				}
				return match, nil
			})
			if err != nil {
				return false, trace.Wrap(err)
			}
			if !match {
				return false, nil
			}
		}

		// All IP SANs requested must match one of the IP SAN matchers in the
		// condition.
		for _, ipSAN := range ipSANs {
			match, err := contains(cond.IPSANs, func(s string) (bool, error) {
				_, cidr, err := net.ParseCIDR(s)
				if err != nil {
					return false, trace.Wrap(err, "parsing cidr")
				}

				return cidr.Contains(ipSAN), nil
			})
			if err != nil {
				return false, trace.Wrap(err)
			}
			if !match {
				return false, nil
			}
		}

		// All condition fields matched.
		return true, nil
	})
}

// CheckSPIFFESVID checks if the role set has access to generating the
// requested SPIFFE ID. Returns an error if the role set does not have the
// ability to generate the requested SVID.
func (set RoleSet) CheckSPIFFESVID(spiffeIDPath string, dnsSANs []string, ipSANs []net.IP) error {
	accessDenied := trace.AccessDenied("access denied to generate SVID %q", spiffeIDPath)

	// check deny: a single match on a deny rule prohibits generation
	for _, role := range set {
		cond := role.GetSPIFFEConditions(types.Deny)
		matched, err := matchSPIFFESVIDConditions(cond, spiffeIDPath, dnsSANs, ipSANs)
		if err != nil {
			return trace.Wrap(err)
		}
		if matched {
			return accessDenied
		}
	}

	// check allow: if a single condition matches, allow generation
	for _, role := range set {
		cond := role.GetSPIFFEConditions(types.Allow)
		matched, err := matchSPIFFESVIDConditions(cond, spiffeIDPath, dnsSANs, ipSANs)
		if err != nil {
			return trace.Wrap(err)
		}
		if matched {
			return nil
		}
	}

	return accessDenied
}

func roleNames(roles []types.Role) string {
	out := make([]string, len(roles))
	for i := range roles {
		out[i] = roles[i].GetName()
	}
	return strings.Join(out, ", ")
}

// matchAllowImpersonateCondition matches impersonate condition,
// both user, role and where condition has to match
func matchAllowImpersonateCondition(ctx *impersonateContext, whereParser predicate.Parser, cond types.ImpersonateConditions, impersonateUser types.User, impersonateRoles []types.Role) (bool, error) {
	// User impersonation requires both users and roles. Roles with no users
	// must use RoleRequests instead; however, we can't treat this as an error
	// since this function is tested against all roles regardless of how
	// they'll be used.
	if len(cond.Users) == 0 || len(cond.Roles) == 0 {
		return false, nil
	}

	anyUser, err := parse.NewAnyMatcher(cond.Users)
	if err != nil {
		return false, trace.Wrap(err)
	}

	if !anyUser.Match(impersonateUser.GetName()) {
		return false, nil
	}

	anyRole, err := parse.NewAnyMatcher(cond.Roles)
	if err != nil {
		return false, trace.Wrap(err)
	}

	for _, impersonateRole := range impersonateRoles {
		if !anyRole.Match(impersonateRole.GetName()) {
			return false, nil
		}
		// TODO:
		// This set impersonateRole inside the ctx that is in turn used inside whereParser
		// which is created in CheckImpersonate above but is being used right below.
		// This is unfortunate interface of the parser, instead
		// parser should accept additional context as a first argument.
		ctx.impersonateRole = impersonateRole
		match, err := matchesImpersonateWhere(cond, whereParser)
		if err != nil {
			return false, trace.Wrap(err)
		}
		if !match {
			return false, nil
		}
	}

	return true, nil
}

// matchDenyImpersonateCondition matches impersonate condition,
// greedy is used for deny type rules, where any user or role can match
func matchDenyImpersonateCondition(cond types.ImpersonateConditions, impersonateUser types.User, impersonateRoles []types.Role) (bool, error) {
	// As above, user impersonation requires both users and roles. We can't
	// return an error to ensure role impersonation rules are allowed to exist
	// in the system.
	if len(cond.Users) == 0 || len(cond.Roles) == 0 {
		return false, nil
	}

	anyUser, err := parse.NewAnyMatcher(cond.Users)
	if err != nil {
		return false, trace.Wrap(err)
	}

	if anyUser.Match(impersonateUser.GetName()) {
		return true, nil
	}

	anyRole, err := parse.NewAnyMatcher(cond.Roles)
	if err != nil {
		return false, trace.Wrap(err)
	}

	for _, impersonateRole := range impersonateRoles {
		if anyRole.Match(impersonateRole.GetName()) {
			return true, nil
		}
	}

	return false, nil
}

// matchAllowRoleImpersonateCondition matches an allow impersonate condition
// specifically for role-only impersonation, where only roles are matched.
func matchAllowRoleImpersonateCondition(ctx *impersonateContext, whereParser predicate.Parser, cond types.ImpersonateConditions, impersonateRoles []types.Role) (bool, error) {
	// an empty set matches nothing
	if len(cond.Users) == 0 && len(cond.Roles) == 0 {
		return false, nil
	}

	// Role impersonation can never apply to users.
	if len(cond.Users) != 0 {
		return false, nil
	}

	// By this point, at least 1 role is guaranteed.
	anyRole, err := parse.NewAnyMatcher(cond.Roles)
	if err != nil {
		return false, trace.Wrap(err)
	}

	for _, impersonateRole := range impersonateRoles {
		if !anyRole.Match(impersonateRole.GetName()) {
			return false, nil
		}
		// TODO:
		// This set impersonateRole inside the ctx that is in turn used inside whereParser
		// which is created in CheckImpersonate above but is being used right below.
		// This is unfortunate interface of the parser, instead
		// parser should accept additional context as a first argument.
		ctx.impersonateRole = impersonateRole
		match, err := matchesImpersonateWhere(cond, whereParser)
		if err != nil {
			return false, trace.Wrap(err)
		}
		if !match {
			return false, nil
		}
	}

	return true, nil
}

// matchDenyRoleImpersonateCondition matches a deny impersonate condition
// specifically for role impersonation, where only roles are matched.
func matchDenyRoleImpersonateCondition(cond types.ImpersonateConditions, impersonateRoles []types.Role) (bool, error) {
	// an empty set matches nothing
	if len(cond.Users) == 0 && len(cond.Roles) == 0 {
		return false, nil
	}

	// Role impersonation can never apply to users.
	if len(cond.Users) != 0 {
		return false, nil
	}

	// By this point, at least 1 role is guaranteed.
	anyRole, err := parse.NewAnyMatcher(cond.Roles)
	if err != nil {
		return false, trace.Wrap(err)
	}

	for _, impersonateRole := range impersonateRoles {
		if anyRole.Match(impersonateRole.GetName()) {
			return true, nil
		}
	}

	return false, nil
}

// RoleMatcherFunc is a convenience type for creating a role matcher from a function.
type RoleMatcherFunc func(types.Role, types.RoleConditionType) (bool, error)

func (f RoleMatcherFunc) Match(role types.Role, condition types.RoleConditionType) (bool, error) {
	return f(role, condition)
}

// RoleMatcher defines an interface for a generic role matcher.
type RoleMatcher interface {
	Match(types.Role, types.RoleConditionType) (bool, error)
}

// RoleMatchers defines a list of matchers.
type RoleMatchers []RoleMatcher

// MatchAll returns true if all matchers in the set match.
func (m RoleMatchers) MatchAll(role types.Role, condition types.RoleConditionType) (bool, error) {
	for _, matcher := range m {
		match, err := matcher.Match(role, condition)
		if err != nil {
			return false, trace.Wrap(err)
		}
		if !match {
			return false, nil
		}
	}
	return true, nil
}

// MatchAny returns true if at least one of the matchers in the set matches.
//
// If the result is true, returns matcher that matched.
func (m RoleMatchers) MatchAny(role types.Role, condition types.RoleConditionType) (bool, RoleMatcher, error) {
	for _, matcher := range m {
		match, err := matcher.Match(role, condition)
		if err != nil {
			return false, nil, trace.Wrap(err)
		}
		if match {
			return true, matcher, nil
		}
	}
	return false, nil, nil
}

// databaseUserMatcher matches a role against database account name.
type databaseUserMatcher struct {
	// user is the name of the database user.
	user string
	// alternativeNames is a list of alternative names for the database user.
	alternativeNames []string
	// caseInsensitive specifies if the username is case insensitive.
	caseInsensitive bool
}

// NewDatabaseUserMatcher creates a RoleMatcher that checks whether the role's
// database users match the specified condition.
func NewDatabaseUserMatcher(db types.Database, user string) RoleMatcher {
	if db.SupportAWSIAMRoleARNAsUsers() {
		return &databaseUserMatcher{
			user:             user,
			alternativeNames: makeUsernamesForAWSRoleARN(db, user),
			caseInsensitive:  db.IsUsernameCaseInsensitive(),
		}
	}

	if db.RequireAWSIAMRolesAsUsers() {
		return &databaseUserMatcher{
			user:             user,
			alternativeNames: makeAlternativeNamesForAWSRole(db, user),
			caseInsensitive:  db.IsUsernameCaseInsensitive(),
		}
	}

	return &databaseUserMatcher{
		user:            user,
		caseInsensitive: db.IsUsernameCaseInsensitive(),
	}
}

// Match matches database account name against provided role and condition.
func (m *databaseUserMatcher) Match(role types.Role, condition types.RoleConditionType) (bool, error) {
	selectors := role.GetDatabaseUsers(condition)

	if match, _ := MatchDatabaseUser(selectors, m.user, true /*matchWildcard*/, m.caseInsensitive); match {
		return true, nil
	}

	for _, altName := range m.alternativeNames {
		if match, _ := MatchDatabaseUser(selectors, altName, false /*matchWildcard*/, m.caseInsensitive); match {
			return true, nil
		}
	}
	return false, nil
}

// String returns the matcher's string representation.
func (m *databaseUserMatcher) String() string {
	return fmt.Sprintf("databaseUserMatcher(user=%v, alternativeNames=%v)", m.user, m.alternativeNames)
}

func makeAlternativeNamesForAWSRole(db types.Database, user string) []string {
	metadata := db.GetAWS()
	if metadata.Region == "" || metadata.AccountID == "" {
		return nil
	}

	// If input database user is a role ARN, try the short role name.
	// The input role ARN must have matching partition and account ID in
	// order to try the short role name.
	if arn.IsARN(user) {
		roleName, err := awsutils.ValidateRoleARNAndExtractRoleName(user, metadata.Partition(), metadata.AccountID)
		if err != nil {
			return nil
		}
		return []string{roleName}
	}

	// If input database user is the short role name, try the full ARN.
	roleARN, err := awsutils.BuildRoleARN(user, metadata.Region, metadata.AccountID)
	if err != nil {
		return nil
	}
	return []string{roleARN}
}

// makeUsernamesForAWSRoleARN builds ARN alternatives for database users who are full or
// partial ARN.
func makeUsernamesForAWSRoleARN(db types.Database, user string) []string {
	if !awsutils.IsRoleARN(user) {
		return nil
	}

	metadata := db.GetAWS()
	if metadata.Region != "" && metadata.AccountID != "" && awsutils.IsPartialRoleARN(user) {
		roleARN, err := awsutils.BuildRoleARN(user, metadata.Region, metadata.AccountID)
		if err != nil {
			return nil
		}

		return []string{roleARN}
	}

	roleARN, err := awsutils.ParseRoleARN(user)
	if err != nil {
		return nil
	}

	return []string{roleARN.Resource}
}

// DatabaseNameMatcher matches a role against database name.
type DatabaseNameMatcher struct {
	Name string
}

// Match matches database name against provided role and condition.
func (m *DatabaseNameMatcher) Match(role types.Role, condition types.RoleConditionType) (bool, error) {
	match, _ := MatchDatabaseName(role.GetDatabaseNames(condition), m.Name)
	return match, nil
}

// String returns the matcher's string representation.
func (m *DatabaseNameMatcher) String() string {
	return fmt.Sprintf("DatabaseNameMatcher(Name=%v)", m.Name)
}

type loginMatcher struct {
	login string
}

// NewLoginMatcher creates a RoleMatcher that checks whether the role's logins
// match the specified condition.
func NewLoginMatcher(login string) RoleMatcher {
	return &loginMatcher{login: login}
}

// Match matches a login against a role.
func (l *loginMatcher) Match(role types.Role, typ types.RoleConditionType) (bool, error) {
	logins := role.GetLogins(typ)
	for _, login := range logins {
		if l.login == login {
			return true, nil
		}
	}
	return false, nil
}

type windowsLoginMatcher struct {
	login string
}

// NewWindowsLoginMatcher creates a RoleMatcher that checks whether the role's
// Windows desktop logins match the specified condition.
func NewWindowsLoginMatcher(login string) RoleMatcher {
	return &windowsLoginMatcher{login: login}
}

// Match matches a Windows Desktop login against a role.
func (l *windowsLoginMatcher) Match(role types.Role, typ types.RoleConditionType) (bool, error) {
	logins := role.GetWindowsLogins(typ)
	for _, login := range logins {
		if l.login == login {
			return true, nil
		}
	}
	return false, nil
}

type awsAppLoginMatcher struct {
	awsRole string
}

// NewAppAWSLoginMatcher creates a RoleMatcher that checks whether the role's
// AWS Role ARN match the specified condition.
func NewAppAWSLoginMatcher(awsRole string) RoleMatcher {
	return &awsAppLoginMatcher{awsRole: awsRole}
}

// Match matches an AWS Role ARN login against a role.
func (l *awsAppLoginMatcher) Match(role types.Role, typ types.RoleConditionType) (bool, error) {
	awsRoles := role.GetAWSRoleARNs(typ)
	for _, awsRole := range awsRoles {
		if l.awsRole == awsRole {
			return true, nil
		}
	}
	return false, nil
}

type kubernetesClusterLabelMatcher struct {
	clusterLabels map[string]string
	userTraits    wrappers.Traits
}

// NewKubeResourcesMatcher creates a new KubeResourcesMatcher matcher that
// matches a role against any Kubernetes Resource specified.
// It also keeps track of the resources that did not match any of user's roles and
// that shouldn't be included in the resource ids because the user is not allowed
// to request them.
func NewKubeResourcesMatcher(resources []types.KubernetesResource) *KubeResourcesMatcher {
	matcher := &KubeResourcesMatcher{
		resources:     resources,
		unmatchedReqs: map[string]struct{}{},
	}
	for _, r := range resources {
		matcher.unmatchedReqs[unmatchedKey(r)] = struct{}{}
	}
	return matcher
}

// unmatchedKey returns a unique key for a Kubernetes resource.
// It is used to keep track of the resources that did not match any of user's roles.
// Format: <kind>/<namespace>/<name>
func unmatchedKey(r types.KubernetesResource) string {
	return path.Join(r.Kind, r.ClusterResource())
}

// KubeResourcesMatcher matches a role against any Kubernetes Resource specified.
// It also keeps track of the resources that did not match any of user's roles and
// that shouldn't be included in the resource ids because the user is not allowed
// to request them.
type KubeResourcesMatcher struct {
	resources     []types.KubernetesResource
	unmatchedReqs map[string]struct{}
}

// Match matches a Kubernetes resource against provided role and condition.
func (m *KubeResourcesMatcher) Match(role types.Role, condition types.RoleConditionType) (bool, error) {
	var finalResult bool
	for _, resource := range m.resources {
		// We use utils.KubeResourceMatchesRegexWithVerbsCollector instead of utils.KubeResourceMatchesRegex
		// because KubeResourcesMatcher is used to match access request resources at creation time against
		// the roles specified in the `search_as_roles` field. This means that we don't have the request verb
		// at this point and we need to match the resource against all the verbs specified in the role.
		// If the resource matches any of the verbs, we consider the resource as matched.
		// Verbs are enforced at the request time when the user is trying to access the Kubernetes Pod.
		result, _, err := utils.KubeResourceMatchesRegexWithVerbsCollector(resource, role.GetKubeResources(condition))
		if err != nil {
			return false, trace.Wrap(err)
		}

		if result {
			delete(m.unmatchedReqs, unmatchedKey(resource))
			finalResult = true
		}
	}
	return finalResult, nil
}

// String returns the matcher's string representation.
func (m *KubeResourcesMatcher) String() string {
	return fmt.Sprintf("KubeResourcesMatcher(Resources=%v)", m.resources)
}

// Unmatched returns the Kubernetes Resource request access that that didn't
// match with any `search_as_roles` kubernetes resources.
func (m *KubeResourcesMatcher) Unmatched() []string {
	unmatched := make([]string, 0, len(m.unmatchedReqs))
	for k := range m.unmatchedReqs {
		unmatched = append(unmatched, k)
	}
	return unmatched
}

// KubernetesResourceMatcher matches a role against a Kubernetes Resource.
// Kind is must be stricly equal but namespace and name allow wildcards.
type KubernetesResourceMatcher struct {
	resource types.KubernetesResource
}

// NewKubernetesResourceMatcher creates a KubernetesResourceMatcher that checks
// whether the role's KubeResources match the specified condition.
func NewKubernetesResourceMatcher(resource types.KubernetesResource) *KubernetesResourceMatcher {
	return &KubernetesResourceMatcher{
		resource: resource,
	}
}

// Match matches a Kubernetes Resource against provided role and condition.
func (m *KubernetesResourceMatcher) Match(role types.Role, condition types.RoleConditionType) (bool, error) {
	result, err := utils.KubeResourceMatchesRegex(m.resource, role.GetKubeResources(condition), condition)

	return result, trace.Wrap(err)
}

// String returns the matcher's string representation.
func (m *KubernetesResourceMatcher) String() string {
	return fmt.Sprintf("KubernetesResourceMatcher(Resource=%v)", m.resource)
}

// NewKubernetesClusterLabelMatcher creates a RoleMatcher that checks whether a role's
// Kubernetes service labels match.
func NewKubernetesClusterLabelMatcher(clustersLabels map[string]string, userTraits wrappers.Traits) RoleMatcher {
	return &kubernetesClusterLabelMatcher{clusterLabels: clustersLabels, userTraits: userTraits}
}

// Match matches a Kubernetes cluster labels against a role.
func (l *kubernetesClusterLabelMatcher) Match(role types.Role, typ types.RoleConditionType) (bool, error) {
	labelMatchers, err := l.getKubeLabelMatchers(role, typ)
	if err != nil {
		return false, trace.Wrap(err)
	}
	ok, _, err := CheckLabelsMatch(typ, labelMatchers, l.userTraits, mapLabelGetter(l.clusterLabels), false)
	return ok, trace.Wrap(err)
}

// getKubeLabelMatchers returns kubernetes_labels based on resource version and role type.
func (l kubernetesClusterLabelMatcher) getKubeLabelMatchers(role types.Role, typ types.RoleConditionType) (types.LabelMatchers, error) {
	labelMatchers, err := role.GetLabelMatchers(typ, types.KindKubernetesCluster)
	if err != nil {
		return types.LabelMatchers{}, trace.Wrap(err)
	}

	// After the introduction of https://github.com/gravitational/teleport/pull/9759 the
	// kubernetes_labels started to be respected. Former role behavior evaluated deny rules
	// even if the kubernetes_labels was empty. To preserve this behavior after respecting kubernetes label the label
	// logic needs to be aligned.
	// Default wildcard rules should be added to  deny.kubernetes_labels if
	// deny.kubernetes_labels is empty to ensure that deny rule will be evaluated
	// even if kubernetes_labels are empty.
	if labelMatchers.Empty() && typ == types.Deny {
		labelMatchers.Labels = types.Labels{types.Wildcard: []string{types.Wildcard}}
	}
	return labelMatchers, nil
}

// AccessCheckable is the subset of types.Resource required for the RBAC checks.
type AccessCheckable interface {
	GetKind() string
	GetName() string
	GetMetadata() types.Metadata
	GetLabel(key string) (value string, ok bool)
	GetAllLabels() map[string]string
}

var rbacLogger = logutils.NewPackageLogger(teleport.ComponentKey, teleport.ComponentRBAC)

// resourceRequiresLabelMatching decides if a resource requires lapel matching
// when making RBAC access decisions.
func resourceRequiresLabelMatching(r AccessCheckable) bool {
	// Some resources do not need label matching when assessing whether the user
	// should be granted access. Enable it by default, but turn it off in the
	// special cases.
	switch r.GetKind() {
	case types.KindIdentityCenterAccount, types.KindIdentityCenterAccountAssignment:
		return false
	}
	return true
}

func (set RoleSet) checkAccess(r AccessCheckable, traits wrappers.Traits, state AccessState, matchers ...RoleMatcher) error {
	// Note: logging in this function only happens in trace mode. This is because
	// adding logging to this function (which is called on every resource returned
	// by the backend) can slow down this function by 50x for large clusters!
	ctx := context.Background()
	logger := rbacLogger
	isLoggingEnabled := logger.Handler().Enabled(ctx, logutils.TraceLevel)
	if isLoggingEnabled {
		logger.With("resource_kind", r.GetKind(), "resource_name", r.GetName())
	}

	if !state.MFAVerified && state.MFARequired == MFARequiredAlways {
		logger.LogAttrs(ctx, logutils.TraceLevel, "Access to resource denied, cluster requires per-session MFA")
		return ErrSessionMFARequired
	}

	requiresLabelMatching := resourceRequiresLabelMatching(r)
	namespace := types.ProcessNamespace(r.GetMetadata().Namespace)

	// Additional message depending on kind of resource
	// so there's more context on why the user might not have access.
	additionalDeniedMessage := ""
	switch r.GetKind() {
	case types.KindDatabase:
		additionalDeniedMessage = "Confirm database user and name."
	case types.KindNode:
		additionalDeniedMessage = "Confirm SSH login."
	case types.KindKubernetesCluster:
		additionalDeniedMessage = "Confirm Kubernetes user or group."
	case types.KindWindowsDesktop:
		additionalDeniedMessage = "Confirm Windows user."
	}

	// Check deny rules.
	for _, role := range set {
		matchNamespace, namespaceMessage := MatchNamespace(role.GetNamespaces(types.Deny), namespace)
		if !matchNamespace {
			continue
		}
		if requiresLabelMatching {
			matchLabels, labelsMessage, err := checkRoleLabelsMatch(types.Deny, role, traits, r, isLoggingEnabled)
			if err != nil {
				return trace.Wrap(err)
			}
			if matchLabels {
				logger.LogAttrs(ctx, logutils.TraceLevel, "Access to resource denied, deny rule in role matched",
					slog.String("role", role.GetName()),
					slog.String("namespace_message", namespaceMessage),
					slog.String("label_message", labelsMessage),
				)
				return trace.AccessDenied("access to %v denied. User does not have permissions. %v",
					r.GetKind(), additionalDeniedMessage)
			}
		} else {
			logger.LogAttrs(ctx, logutils.TraceLevel, "Role label matching skipped")
		}
		// Deny rules are greedy on purpose. They will always match if
		// at least one of the matchers returns true.
		matchMatchers, matchersMessage, err := RoleMatchers(matchers).MatchAny(role, types.Deny)
		if err != nil {
			return trace.Wrap(err)
		}
		if matchMatchers {
			logger.LogAttrs(ctx, logutils.TraceLevel, "Access to resource denied, deny rule in role matched",
				slog.String("role", role.GetName()),
				slog.Any("matcher_message", matchersMessage),
			)
			return trace.AccessDenied("access to %v denied. User does not have permissions. %v",
				r.GetKind(), additionalDeniedMessage)
		}
	}

	mfaAllowed := state.MFAVerified || state.MFARequired == MFARequiredNever

	// TODO(codingllama): Consider making EnableDeviceVerification opt-out instead
	//  of opt-in.
	deviceAllowed := !state.EnableDeviceVerification || state.DeviceVerified

	var errs []error
	allowed := false
	// Check allow rules.
	for _, role := range set {
		matchNamespace, namespaceMessage := MatchNamespace(role.GetNamespaces(types.Allow), namespace)
		if !matchNamespace {
			if isLoggingEnabled {
				errs = append(errs, trace.AccessDenied("role=%v, match(namespace=%v)",
					role.GetName(), namespaceMessage))
			}
			continue
		}

		if requiresLabelMatching {
			matchLabels, labelsMessage, err := checkRoleLabelsMatch(types.Allow, role, traits, r, isLoggingEnabled)
			if err != nil {
				return trace.Wrap(err)
			}

			if !matchLabels {
				if isLoggingEnabled {
					errs = append(errs, trace.AccessDenied("role=%v, match(%s)",
						role.GetName(), labelsMessage))
				}
				continue
			}
		} else {
			logger.LogAttrs(ctx, logutils.TraceLevel, "Role label matching skipped for resource")
		}

		// Allow rules are not greedy. They will match only if all of the
		// matchers return true.
		matchMatchers, err := RoleMatchers(matchers).MatchAll(role, types.Allow)
		if err != nil {
			return trace.Wrap(err)
		}
		if !matchMatchers {
			if isLoggingEnabled {
				errs = append(errs, fmt.Errorf("role=%v, match(matchers=%v)",
					role.GetName(), matchers))
			}
			continue
		}

		// If we've reached this point, namespace, labels, and matchers all match.
		//
		// The following checks remain:
		// 1. MFA verification (aka require_session_mfa)
		// 2. Device verification (aka device_trust_mode)
		//
		// The more restrictive setting applies, so either the caller passes all
		// (and gets an early exit) or we need to check every applicable role to
		// ensure the access is permitted.

		if mfaAllowed && deviceAllowed {
			logger.LogAttrs(ctx, logutils.TraceLevel, "Access to resource granted, allow rule in role matched",
				slog.String("role", role.GetName()),
			)
			return nil
		}

		// MFA verification.
		if !mfaAllowed && role.GetOptions().RequireMFAType.IsSessionMFARequired() {
			logger.LogAttrs(ctx, logutils.TraceLevel, "Access to resource denied, role requires per-session MFA",
				slog.String("role", role.GetName()),
			)
			return ErrSessionMFARequired
		}

		// Device verification.
		if !deviceAllowed && role.GetOptions().DeviceTrustMode == constants.DeviceTrustModeRequired {
			logger.LogAttrs(ctx, logutils.TraceLevel, "Access to resource denied, role requires a trusted device",
				slog.String("role", role.GetName()),
			)
			return ErrTrustedDeviceRequired
		}

		// Current role allows access, but keep looking for a more restrictive
		// setting.
		allowed = true
		logger.LogAttrs(ctx, logutils.TraceLevel, "Access to resource granted, allow rule in role matched",
			slog.String("role", role.GetName()),
		)
	}

	if allowed {
		return nil
	}

	logger.LogAttrs(ctx, logutils.TraceLevel, "Access to resource denied, no allow rule matched",
		slog.Any("errors", errs),
	)
	return trace.AccessDenied("access to %v denied. User does not have permissions. %v",
		r.GetKind(), additionalDeniedMessage)
}

// checkRoleLabelsMatch checks if the [role] matches the labels of [resource]
// for [condition].
// It considers both the role labels (<kind>_labels) and label expression
// (<kind>_labels_expression).
//
// Returns a match boolean, a debug message, and any unexpected error.
//
// If [condition] is types.Deny, the match is greedy, if either one matches it's
// considered a match.
//
// If [condition] is types.Allow, the match is not greedy, if either doesn't
// match it's not considered a match.
//
// If neither is set, it's not a match in either case.
func checkRoleLabelsMatch(
	condition types.RoleConditionType,
	role types.Role,
	userTraits wrappers.Traits,
	resource AccessCheckable,
	debug bool,
) (bool, string, error) {
	labelMatchers, err := role.GetLabelMatchers(condition, resource.GetKind())
	if err != nil {
		return false, "", trace.Wrap(err)
	}
	return CheckLabelsMatch(condition, labelMatchers, userTraits, resource, debug)
}

// CheckLabelsMatch checks if the [labelMatchers] match the labels of [resource]
// for [condition].
// It considers both [labelMatchers.Labels] and [labelMatchers.Expression].
//
// Returns a match boolean, a debug message, and any unexpected error.
//
// If [condition] is types.Deny, the match is greedy, if either one matches it's
// considered a match.
//
// If [condition] is types.Allow, the match is not greedy, if either doesn't
// match it's not considered a match.
//
// If neither is set, it's not a match in either case.
func CheckLabelsMatch(
	condition types.RoleConditionType,
	labelMatchers types.LabelMatchers,
	userTraits wrappers.Traits,
	resource LabelGetter,
	debug bool,
) (bool, string, error) {
	if labelMatchers.Empty() {
		return false, "no label matchers or label expression", nil
	}

	var message string
	labelsUnsetOrMatch, expressionUnsetOrMatch := true, true

	if len(labelMatchers.Labels) > 0 {
		match, msg, err := MatchLabelGetter(labelMatchers.Labels, resource)
		if err != nil {
			return false, "", trace.Wrap(err)
		}
		if debug {
			message += "label=" + msg
		}
		// Deny rules are greedy, if either matches, it's a match.
		if condition == types.Deny && match {
			return true, message, nil
		}
		labelsUnsetOrMatch = match
	}

	if len(labelMatchers.Expression) > 0 {
		match, msg, err := matchLabelExpression(labelMatchers.Expression, resource, userTraits)
		if err != nil {
			return false, "", trace.Wrap(err)
		}
		if debug {
			message = strings.Join([]string{message, "expression=" + msg}, ", ")
		}
		// Deny rules are greedy, if either matches, it's a match.
		if condition == types.Deny {
			return match, message, nil
		}
		expressionUnsetOrMatch = match
	}

	if condition == types.Deny {
		// Either branch would have returned if it was a match.
		return false, message, nil
	}

	// Allow rules are not greedy, both must match if they are set.
	return labelsUnsetOrMatch && expressionUnsetOrMatch, message, nil
}

func matchLabelExpression(labelExpression string, resource LabelGetter, userTraits wrappers.Traits) (bool, string, error) {
	parsedExpr, err := parseLabelExpression(labelExpression)
	if err != nil {
		return false, "", trace.Wrap(err)
	}
	match, err := parsedExpr.Evaluate(labelExpressionEnv{
		resourceLabelGetter: resource,
		userTraits:          userTraits,
	})
	if err != nil {
		return false, "", trace.Wrap(err, "evaluating label expression %q", labelExpression)
	}
	if match {
		return true, "matched", nil
	}
	return false, "no match", nil
}

// CanForwardAgents returns true if role set allows forwarding agents.
func (set RoleSet) CanForwardAgents() bool {
	for _, role := range set {
		if role.GetOptions().ForwardAgent.Value() {
			return true
		}
	}
	return false
}

// SSHPortForwardMode enumerates the possible SSH port forwarding modes available at a given time.
type SSHPortForwardMode int

const (
	// SSHPortForwardModeOn is the default mode, both remote and local port forwarding is allowed
	SSHPortForwardModeOn SSHPortForwardMode = iota
	// SSHPortForwardModeOff disallows any port forwarding.
	SSHPortForwardModeOff
	// SSHPortForwardModeRemote allows remote port forwarding.
	SSHPortForwardModeRemote
	// SSHPortForwardModeLocal allows local port forwarding.
	SSHPortForwardModeLocal
)

// String implements the Stringer interface for SSHPortForwardMode
func (m SSHPortForwardMode) String() string {
	switch m {
	case SSHPortForwardModeOff:
		return "off"
	case SSHPortForwardModeLocal:
		return "local"
	case SSHPortForwardModeRemote:
		return "remote"
	default:
		return "on"
	}
}

// SSHPortForwardMode returns the SSHPortForwardMode permitted by a RoleSet. Port forwarding is implicitly allowed, but explicit denies take
// precedence of explicit allows when using SSHPortForwarding. The legacy PortForwarding field prefers explicit allows for backwards
// compatibility reasons, but is only evaluated in the absence of an SSHPortForwarding config on the same role.
func (set RoleSet) SSHPortForwardMode() SSHPortForwardMode {
	var denyRemote, denyLocal, legacyDeny bool
	legacyCanDeny := true

	for _, role := range set {
		config := role.GetOptions().SSHPortForwarding
		// only consider legacy allows when config isn't provided on the same role
		if config == nil {
			//nolint:staticcheck // this field is preserved for backwards compatibility, but shouldn't be used going forward
			if legacy := role.GetOptions().PortForwarding; legacy != nil {
				if legacy.Value {
					return SSHPortForwardModeOn
				}
				legacyDeny = true
			}

			continue
		}

		if config.Remote != nil && config.Remote.Enabled != nil {
			if !config.Remote.Enabled.Value {
				denyRemote = true
			}

			// an explicit legacy deny is only possible if no explicit SSHPortForwarding config has been provided
			legacyCanDeny = false
		}

		if config.Local != nil && config.Local.Enabled != nil {
			if !config.Local.Enabled.Value {
				denyLocal = true
			}

			// an explicit legacy deny is only possible if no explicit SSHPortForwarding config has been provided
			legacyCanDeny = false
		}
	}

	// enforcing implicit allow and preferring allow over explicit deny
	switch {
	case denyRemote && denyLocal:
		return SSHPortForwardModeOff
	case legacyDeny && legacyCanDeny:
		return SSHPortForwardModeOff
	case denyRemote:
		return SSHPortForwardModeLocal
	case denyLocal:
		return SSHPortForwardModeRemote
	default:
		return SSHPortForwardModeOn
	}
}

// CanPortForward returns true if the RoleSet allows both local and remote port forwarding.
func (set RoleSet) CanPortForward() bool {
	return set.SSHPortForwardMode() == SSHPortForwardModeOn
}

// RecordDesktopSession returns true if the role set has enabled desktop
// session recording. Recording is considered enabled if at least one
// role in the set has enabled it.
func (set RoleSet) RecordDesktopSession() bool {
	for _, role := range set {
		var bo *types.BoolOption
		if role.GetOptions().RecordSession != nil {
			bo = role.GetOptions().RecordSession.Desktop
		}
		if types.BoolDefaultTrue(bo) {
			return true
		}
	}
	return false
}

// DesktopClipboard returns true if the role set has enabled shared
// clipboard for desktop sessions. Clipboard sharing is disabled if
// one or more of the roles in the set has disabled it.
func (set RoleSet) DesktopClipboard() bool {
	for _, role := range set {
		if !types.BoolDefaultTrue(role.GetOptions().DesktopClipboard) {
			return false
		}
	}
	return true
}

// DesktopDirectorySharing returns true if the role set has directory sharing
// enabled. This setting is disabled if one or more of the roles in the set has
// disabled it.
func (set RoleSet) DesktopDirectorySharing() bool {
	for _, role := range set {
		if !types.BoolDefaultTrue(role.GetOptions().DesktopDirectorySharing) {
			return false
		}
	}
	return true
}

// MaybeCanReviewRequests attempts to guess if this RoleSet belongs
// to a user who should be submitting access reviews.  Because not all rolesets
// are derived from statically assigned roles, this may return false positives.
func (set RoleSet) MaybeCanReviewRequests() bool {
	for _, role := range set {
		if !role.GetAccessReviewConditions(types.Allow).IsZero() {
			// at least one nonzero allow directive exists for
			// review submission.
			return true
		}
	}
	return false
}

// PermitX11Forwarding returns true if this RoleSet allows X11 Forwarding.
func (set RoleSet) PermitX11Forwarding() bool {
	for _, role := range set {
		if role.GetOptions().PermitX11Forwarding.Value() {
			return true
		}
	}
	return false
}

// CanCopyFiles returns true if the role set has enabled remote file
// operations via SCP or SFTP. Remote file operations are disabled if
// one or more of the roles in the set has disabled it.
func (set RoleSet) CanCopyFiles() bool {
	for _, role := range set {
		if !types.BoolDefaultTrue(role.GetOptions().SSHFileCopy) {
			return false
		}
	}
	return true
}

// CanJoinSessions returns true if at least one role in the role set
// allows the user to join active sessions.
func (set RoleSet) CanJoinSessions() bool {
	return slices.ContainsFunc(set, func(r types.Role) bool {
		return len(r.GetSessionJoinPolicies()) > 0
	})
}

// CertificateFormat returns the most permissive certificate format in a
// RoleSet.
func (set RoleSet) CertificateFormat() string {
	var formats []string

	for _, role := range set {
		// get the certificate format for each individual role. if a role does not
		// have a certificate format (like implicit roles) skip over it
		certificateFormat := role.GetOptions().CertificateFormat
		if certificateFormat == "" {
			continue
		}

		formats = append(formats, certificateFormat)
	}

	// if no formats were found, return standard
	if len(formats) == 0 {
		return constants.CertificateFormatStandard
	}

	// sort the slice so the most permissive is the first element
	sort.Slice(formats, func(i, j int) bool {
		return certificatePriority(formats[i]) < certificatePriority(formats[j])
	})

	return formats[0]
}

// EnhancedRecordingSet returns the set of enhanced session recording
// events to capture for thi role set.
func (set RoleSet) EnhancedRecordingSet() map[string]bool {
	m := make(map[string]bool)

	// Loop over all roles and create a set of all options.
	for _, role := range set {
		for _, opt := range role.GetOptions().BPF {
			m[opt] = true
		}
	}

	return m
}

// certificatePriority returns the priority of the certificate format. The
// most permissive has lowest value.
func certificatePriority(s string) int {
	switch s {
	case teleport.CertificateFormatOldSSH:
		return 0
	case constants.CertificateFormatStandard:
		return 1
	default:
		return 2
	}
}

// CheckAgentForward checks if the role can request to forward the SSH agent
// for this user.
func (set RoleSet) CheckAgentForward(login string) error {
	// check if we have permission to login and forward agent. we don't check
	// for deny rules because if you can't forward an agent if you can't login
	// in the first place.
	for _, role := range set {
		for _, l := range role.GetLogins(types.Allow) {
			if role.GetOptions().ForwardAgent.Value() && l == login {
				return nil
			}
		}
	}
	return trace.AccessDenied("%v can not forward agent for %v", set, login)
}

func (set RoleSet) String() string {
	if len(set) == 0 {
		return "user without assigned roles"
	}
	roleNames := make([]string, len(set))
	for i, role := range set {
		roleNames[i] = role.GetName()
	}
	return fmt.Sprintf("roles %v", strings.Join(roleNames, ","))
}

// GuessIfAccessIsPossible guesses if access is possible for an entire category
// of resources.
// It responds the question: "is it possible that there is a resource of this
// kind that the current user can access?".
// GuessIfAccessIsPossible is used, mainly, for UI decisions ("should the tab
// for resource X appear"?). Most callers should use CheckAccessToRule instead.
func (set RoleSet) GuessIfAccessIsPossible(ctx RuleContext, namespace string, resource string, verb string) error {
	// "Where" clause are handled differently by the method:
	// - "allow" rules have their "where" clause always match, as it's assumed
	//   that there could be a resource that matches it.
	// - "deny" rules have their "where" clause always fail, as it's assumed that
	//   there could be a resource that passes it.
	return set.checkAccessToRuleImpl(checkAccessParams{
		ctx:        ctx,
		namespace:  namespace,
		resource:   resource,
		verb:       verb,
		allowWhere: boolParser(true),  // always matches
		denyWhere:  boolParser(false), // never matches
	})
}

type boolParser bool

func (p boolParser) Parse(string) (interface{}, error) {
	return predicate.BoolPredicate(func() bool {
		return bool(p)
	}), nil
}

// CheckAccessToRule checks if the RoleSet provides access in the given
// namespace to the specified resource and verb.
// silent controls whether the access violations are logged.
func (set RoleSet) CheckAccessToRule(ctx RuleContext, namespace string, resource string, verb string) error {
	whereParser, err := NewWhereParser(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	return set.checkAccessToRuleImpl(checkAccessParams{
		ctx:        ctx,
		namespace:  namespace,
		resource:   resource,
		verb:       verb,
		allowWhere: whereParser,
		denyWhere:  whereParser,
	})
}

// GetKubeResources returns allowed and denied list of Kubernetes Resources configured in the RoleSet.
func (set RoleSet) GetKubeResources(cluster types.KubeCluster, userTraits wrappers.Traits) (allowed, denied []types.KubernetesResource) {
	for _, role := range set {
		matchLabels, _, err := checkRoleLabelsMatch(types.Allow, role, userTraits, cluster, false)
		if err != nil || !matchLabels {
			continue
		}
		allowed = append(allowed, role.GetKubeResources(types.Allow)...)
	}

	for _, role := range set {
		// deny rules are not checked for labels because they are greedy. It means that
		// if there is a deny rule for a cluster, it will deny access to all resources
		// in that cluster, regardless of kubernetes_resources (i.e. making them irrelevant).
		// If the goal is to deny access to a specific resource, it should be done by collecting
		// all kube resources in deny rules and ignoring if the role matches or not
		// the cluster (i.e. no labels check).
		denied = append(denied, role.GetKubeResources(types.Deny)...)
	}

	return deduplicateKubeResources(allowed), deduplicateKubeResources(denied)
}

func deduplicateKubeResources(resources []types.KubernetesResource) []types.KubernetesResource {
	allKeys := make(map[string]struct{})
	copy := make([]types.KubernetesResource, 0, len(resources))
	for _, item := range resources {
		key := item.String()
		if _, value := allKeys[key]; !value {
			allKeys[key] = struct{}{}
			copy = append(copy, item)
		}
	}
	return copy
}

type checkAccessParams struct {
	ctx                   RuleContext
	namespace             string
	resource              string
	verb                  string
	allowWhere, denyWhere predicate.Parser
}

type accessExplicitlyDenied struct {
	inner error
}

// AccessExplicitlyDenied is an error type that indicates an AccessDenied error
// where a deny rule matched and access is explicitly denied, in contrast to
// cases where there is no matching deny or allow rule and access is only
// implicitly denied.
func AccessExplicitlyDenied(inner error) error {
	return &accessExplicitlyDenied{inner}
}

// IsAccessExplicitlyDenied returns true if any of the errors in err's chain is
// an AccessExplicitlyDenied error.
func IsAccessExplicitlyDenied(err error) bool {
	var target *accessExplicitlyDenied
	return errors.As(err, &target)
}

func (a *accessExplicitlyDenied) Error() string {
	return a.inner.Error()
}

func (a *accessExplicitlyDenied) Unwrap() error {
	return a.inner
}

func (set RoleSet) checkAccessToRuleImpl(p checkAccessParams) (err error) {
	ctx := context.Background()

	// Every unknown error, which could be due to a bad role or an expression
	// that can't parse, should be considered an explicit denial.
	explicitDeny := true
	defer func() {
		if explicitDeny && err != nil {
			err = AccessExplicitlyDenied(err)
		}
	}()

	actionsParser, err := NewActionsParser(p.ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	// check deny: a single match on a deny rule prohibits access
	for _, role := range set {
		matchNamespace, _ := MatchNamespace(role.GetNamespaces(types.Deny), types.ProcessNamespace(p.namespace))
		if matchNamespace {
			matched, err := MakeRuleSet(role.GetRules(types.Deny)).Match(p.denyWhere, actionsParser, p.resource, p.verb)
			if err != nil {
				return trace.Wrap(err)
			}
			if matched {
				rbacLogger.LogAttrs(ctx, logutils.TraceLevel, "Access denied, deny rule matched",
					slog.String("verb", p.verb),
					slog.String("resource", p.resource),
					slog.String("namespace", p.namespace),
					slog.String("role", role.GetName()),
				)

				return trace.AccessDenied("access denied to perform action %q on %q", p.verb, p.resource)
			}
		}
	}

	// check allow: if rule matches, grant access to resource
	for _, role := range set {
		matchNamespace, _ := MatchNamespace(role.GetNamespaces(types.Allow), types.ProcessNamespace(p.namespace))
		if matchNamespace {
			match, err := MakeRuleSet(role.GetRules(types.Allow)).Match(p.allowWhere, actionsParser, p.resource, p.verb)
			if err != nil {
				return trace.Wrap(err)
			}
			if match {
				return nil
			}
		}
	}

	rbacLogger.LogAttrs(ctx, logutils.TraceLevel, "Access denied, no allow rule matched",
		slog.String("verb", p.verb),
		slog.String("resource", p.resource),
		slog.String("namespace", p.namespace),
		slog.Any("set", set),
	)

	// At this point no deny rule has matched and there are no more unknown
	// errors, so this is only an implicit denial.
	explicitDeny = false
	return trace.AccessDenied("access denied to perform action %q on %q", p.verb, p.resource)
}

// ExtractConditionForIdentifier returns a restrictive filter expression
// for list queries based on the rules' `where` conditions.
func (set RoleSet) ExtractConditionForIdentifier(ctx RuleContext, namespace, resource, verb, identifier string) (*types.WhereExpr, error) {
	parser, err := newParserForIdentifierSubcondition(ctx, identifier)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	parseWhere := func(rule types.Rule) (types.WhereExpr, error) {
		if rule.Where == "" {
			return types.WhereExpr{Literal: true}, nil
		}
		out, err := parser.Parse(rule.Where)
		if err != nil {
			return types.WhereExpr{}, trace.Wrap(err)
		}
		expr, ok := out.(types.WhereExpr)
		if !ok {
			return types.WhereExpr{}, trace.BadParameter("invalid type %T when extracting identifier subcondition from %q", out, rule.Where)
		}
		return expr, nil
	}

	// Gather identifier-related subconditions from the deny rules
	// and concatenate their negations by AND.
	var denyCond *types.WhereExpr
	for _, role := range set {
		matchNamespace, _ := MatchNamespace(role.GetNamespaces(types.Deny), types.ProcessNamespace(namespace))
		if !matchNamespace {
			continue
		}
		rules := MakeRuleSet(role.GetRules(types.Deny))
		for _, rule := range rules[resource] {
			if !rule.HasVerb(verb) && !rule.HasVerb(types.Wildcard) {
				continue
			}
			expr, err := parseWhere(rule)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			if b, ok := expr.Literal.(bool); ok {
				if b {
					return nil, trace.AccessDenied("access denied to perform action %q on %q", verb, resource)
				}
				continue
			}
			negated := types.WhereExpr{Not: &expr}
			if denyCond == nil {
				denyCond = &negated
			} else {
				denyCond = &types.WhereExpr{And: types.WhereExpr2{L: denyCond, R: &negated}}
			}
		}
	}

	// Gather identifier-related subconditions from the allow rules
	// and concatenate by OR.
	var allowCond *types.WhereExpr
	for _, role := range set {
		matchNamespace, _ := MatchNamespace(role.GetNamespaces(types.Allow), types.ProcessNamespace(namespace))
		if !matchNamespace {
			continue
		}
		rules := MakeRuleSet(role.GetRules(types.Allow))
		for _, rule := range rules[resource] {
			if !rule.HasVerb(verb) && !rule.HasVerb(types.Wildcard) {
				continue
			}
			expr, err := parseWhere(rule)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			if b, ok := expr.Literal.(bool); ok {
				if b {
					return denyCond, nil
				}
				continue
			}
			if allowCond == nil {
				allowCond = &expr
			} else {
				allowCond = &types.WhereExpr{Or: types.WhereExpr2{L: allowCond, R: &expr}}
			}
		}
	}

	if denyCond == nil {
		if allowCond == nil {
			return nil, trace.AccessDenied("access denied to perform action %q on %q", verb, resource)
		}
		return allowCond, nil
	}
	return &types.WhereExpr{And: types.WhereExpr2{L: denyCond, R: allowCond}}, nil
}

// SearchAsRolesOption is a functional option for filtering SearchAsRoles.
type SearchAsRolesOption func(role types.Role) bool

// GetSearchAsRoles returns all SearchAsRoles for this RoleSet.
func (set RoleSet) GetAllowedSearchAsRoles(allowFilters ...SearchAsRolesOption) []string {
	denied := make(map[string]struct{})
	var allowed []string
	for _, role := range set {
		for _, d := range role.GetSearchAsRoles(types.Deny) {
			denied[d] = struct{}{}
		}
	}
	for _, role := range set {
		if slices.ContainsFunc(allowFilters, func(filter SearchAsRolesOption) bool {
			return !filter(role)
		}) {
			// Don't consider this base role if it's filtered out.
			continue
		}
		for _, a := range role.GetSearchAsRoles(types.Allow) {
			if _, isDenied := denied[a]; isDenied {
				continue
			}
			allowed = append(allowed, a)
		}
	}
	return apiutils.Deduplicate(allowed)
}

// GetAllowedSearchAsRolesForKubeResourceKind returns all of the allowed SearchAsRoles
// that allowed requesting to the requested Kubernetes resource kind.
func (set RoleSet) GetAllowedSearchAsRolesForKubeResourceKind(requestedKubeResourceKind string) []string {
	// Return no results if encountering any denies since its globally matched.
	for _, role := range set {
		for _, kr := range role.GetAccessRequestConditions(types.Deny).KubernetesResources {
			if kr.Kind == types.Wildcard || kr.Kind == requestedKubeResourceKind {
				return nil
			}
		}
	}
	return set.GetAllowedSearchAsRoles(WithAllowedKubernetesResourceKindFilter(requestedKubeResourceKind))
}

// WithAllowedKubernetesResourceKindFilter returns a SearchAsRolesOption func
// that will check that the requestedKubeResourceKind exists in the allow list
// for the current role.
func WithAllowedKubernetesResourceKindFilter(requestedKubeResourceKind string) SearchAsRolesOption {
	return func(role types.Role) bool {
		allowed := role.GetAccessRequestConditions(types.Allow).KubernetesResources
		// any kind is allowed if nothing was configured.
		if len(allowed) == 0 {
			return true
		}
		for _, kr := range role.GetAccessRequestConditions(types.Allow).KubernetesResources {
			if kr.Kind == types.Wildcard || kr.Kind == requestedKubeResourceKind {
				return true
			}
		}
		return false
	}
}

// GetAllowedPreviewAsRoles returns all PreviewAsRoles for this RoleSet.
func (set RoleSet) GetAllowedPreviewAsRoles() []string {
	denied := make(map[string]struct{})
	var allowed []string
	for _, role := range set {
		for _, d := range role.GetPreviewAsRoles(types.Deny) {
			denied[d] = struct{}{}
		}
	}
	for _, role := range set {
		for _, a := range role.GetPreviewAsRoles(types.Allow) {
			if _, ok := denied[a]; !ok {
				allowed = append(allowed, a)
			}
		}
	}
	return apiutils.Deduplicate(allowed)
}

// GetCreateDatabaseUserMode returns the create database user mode of the rule
// set.
func (set RoleSet) GetCreateDatabaseUserMode() types.CreateDatabaseUserMode {
	var mode types.CreateDatabaseUserMode
	for _, r := range set {
		if roleMode := r.GetCreateDatabaseUserMode(); roleMode > mode {
			mode = roleMode
		}
	}

	return mode
}

// AccessState holds state for the present access attempt, including both
// cluster settings and user state (MFA, device trust, etc).
type AccessState struct {
	// MFARequired determines whether a user's MFA requirement dynamically changes
	// based on their active role (per-role), or is static across all roles
	// (always/never).
	MFARequired MFARequired
	// MFAVerified is set when MFA has been verified by the caller.
	MFAVerified bool
	// EnableDeviceVerification enables device verification in access checks.
	// It's recommended to set this in tandem with DeviceVerified, so device
	// checks are easier to reason about and have a proper chance of succeeding.
	// Used for role-based device mode checks.
	// Defaults to false for backwards compatibility.
	EnableDeviceVerification bool
	// DeviceVerified is true if the user certificate contains all required
	// device extensions.
	// A value of true enables the caller to clear device trust checks.
	// It's recommended to set this in tandem with EnableDeviceVerification.
	// See [dtauthz.IsTLSDeviceVerified] and [dtauthz.IsSSHDeviceVerified].
	DeviceVerified bool
}

// MFARequired determines when MFA is required for a user to access a resource.
type MFARequired string

const (
	// MFARequiredNever means that MFA is never required for any sessions started by this user.
	// This means that it is not required by the cluster auth preference or any of the user's roles.
	MFARequiredNever MFARequired = "never"
	// MFARequiredAlways means that MFA is required for all sessions started by a user. This either
	// means that the cluster auth preference requires per-session MFA, or all of the user's roles require
	// per-session MFA
	MFARequiredAlways MFARequired = "always"
	// MFARequiredPerRole means that MFA requirement is based on which of the user's roles
	// provides access to the session in question.
	MFARequiredPerRole MFARequired = "per-role"
)

// UnmarshalRole unmarshals the Role resource from JSON.
func UnmarshalRole(bytes []byte, opts ...MarshalOption) (types.Role, error) {
	return UnmarshalRoleV6(bytes, opts...)
}

// UnmarshalRoleV6 unmarshals the RoleV6 resource from JSON.
func UnmarshalRoleV6(bytes []byte, opts ...MarshalOption) (*types.RoleV6, error) {
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	version := jsoniter.Get(bytes, "version").ToString()
	switch version {
	// these are all backed by the same shape of data, they just have different semantics and defaults
	case types.V3, types.V4, types.V5, types.V6, types.V7:
	default:
		return nil, trace.BadParameter("role version %q is not supported", version)
	}

	var role types.RoleV6
	if err := utils.FastUnmarshal(bytes, &role); err != nil {
		return nil, trace.BadParameter("%s", err)
	}
	if role.Version != version {
		return nil, trace.BadParameter("inconsistent version in role data, got %q and %q", role.Version, version)
	}

	if err := ValidateRole(&role); err != nil {
		return nil, trace.Wrap(err)
	}

	if cfg.Revision != "" {
		role.SetRevision(cfg.Revision)
	}
	if !cfg.Expires.IsZero() {
		role.SetExpiry(cfg.Expires)
	}
	return &role, nil
}

// MarshalRole marshals the Role resource to JSON.
func MarshalRole(role types.Role, opts ...MarshalOption) ([]byte, error) {
	if err := ValidateRole(role); err != nil {
		return nil, trace.Wrap(err)
	}

	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch role := role.(type) {
	case *types.RoleV6:
		return utils.FastMarshal(maybeResetProtoRevision(cfg.PreserveRevision, role))
	default:
		return nil, trace.BadParameter("unrecognized role version %T", role)
	}
}

// AuthPreferenceGetter defines an interface for getting the authentication
// preferences.
type AuthPreferenceGetter interface {
	// GetAuthPreference fetches the cluster authentication preferences.
	GetAuthPreference(ctx context.Context) (types.AuthPreference, error)
}

// AccessStateFromSSHCertificate populates access state based on user's SSH
// certificate and auth preference.
func AccessStateFromSSHCertificate(ctx context.Context, cert *ssh.Certificate, checker AccessChecker, authPrefGetter AuthPreferenceGetter) (AccessState, error) {
	authPref, err := authPrefGetter.GetAuthPreference(ctx)
	if err != nil {
		return AccessState{}, trace.Wrap(err)
	}
	state := checker.GetAccessState(authPref)
	_, state.MFAVerified = cert.Extensions[teleport.CertExtensionMFAVerified]
	// Certain hardware-key based private key policies are treated as MFA verification.
	if policyString, ok := cert.Extensions[teleport.CertExtensionPrivateKeyPolicy]; ok {
		if keys.PrivateKeyPolicy(policyString).MFAVerified() {
			state.MFAVerified = true
		}
	}
	state.EnableDeviceVerification = true
	state.DeviceVerified = dtauthz.IsSSHDeviceVerified(cert)
	return state, nil
}
