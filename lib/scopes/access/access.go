// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package access

import (
	"iter"
	"slices"
	"strings"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/constants"
	scopedaccessv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/access/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/scopes"
	"github.com/gravitational/teleport/lib/services/label"
)

const (
	// MaxRolesPerAssignment is the maximum number of roles@scope assignments that a given scoped role assignment
	// resource may contain. This value is so low because our backend limits the number of keys that can be associated
	// with a single atomic operation. Any significant increase to this value would necessitate a change to the
	// scoped role backend model.
	MaxRolesPerAssignment = 16

	// KindScopedRole is the kind of a scoped role resource.
	KindScopedRole = "scoped_role"

	// KindScopedRoleAssignment is the kind of a scoped role assignment resource.
	KindScopedRoleAssignment = "scoped_role_assignment"

	// KindScopedToken is the kind of a scoped token resource.
	KindScopedToken = "scoped_token"

	// SubKindDynamic is the sub kind of a scoped role assignment created via the API.
	SubKindDynamic = "dynamic"

	// SubKindMaterialized is the sub kind of a scoped role assignment that has been materialized.
	SubKindMaterialized = "materialized"

	// CreatorKindAccessList indicates that the creator is an access list, for
	// scoped role assignments materialized from access list membership.
	CreatorKindAccessList = "access_list"

	// maxAssignableScopes is the maximum number of assignable scopes that a given scoped role resource may contain. Note that
	// unlike MaxRolesPerAssignment, this is a fairly arbitrary limit and there isn't a strong reason to keep it low other than
	// to avoid excess resource size and to keep our options open for the future.
	maxAssignableScopes = 16

	// invalidChars are the special characters that should not be allowed in certain keys or values.
	invalidChars = "{}^$*"
	// invalidLabelChars are the special characters that should not be allowed in label keys or values.
	invalidLabelChars = "{}^$"
)

// RoleIsAssignableToScopeOfEffect checks if the given role is assignable to the given scope of effect. For example,
// a given assignment can attempt to assign a role at any given scope, but the role's resource scope and assignable
// scope globs must permit such an assignment for privileges to be effective.
func RoleIsAssignableToScopeOfEffect(role *scopedaccessv1.ScopedRole, scopeOfEffect string) bool {
	if scopes.WeakValidate(role.GetScope()) != nil {
		return false
	}

	// The scope of effect must be assignable from the role's origin scope (cannot reach up)
	if !scopes.ScopeOfOrigin(role.GetScope()).IsAssignableToScopeOfEffect(scopeOfEffect) {
		return false
	}

	// The scope of effect must match one of the role's assignable scope globs
	for assignableScope := range WeakValidatedAssignableScopes(role) {
		if scopes.ScopeOfEffectGlob(assignableScope).MatchesScopeOfEffectLiteral(scopeOfEffect) {
			return true
		}
	}

	return false
}

// RoleIsAssignableFromScopeOfOrigin checks if the given role is assignable from the given scope of origin. For example,
// assignment resources at a given scope can only assign roles that are assignable *from* that scope. In such a scenario,
// the resource scope of the assignment resource is the origin scope of the actual assignment.
func RoleIsAssignableFromScopeOfOrigin(role *scopedaccessv1.ScopedRole, scopeOfOrigin string) bool {
	if scopes.WeakValidate(role.GetScope()) != nil {
		return false
	}

	// conceptually, we think of the role and assignment scopes as both being policy resource scopes. when dealing
	// with interdependence between policy resources, we need to ensure that the dependence does not open a hole by
	// which edits can cause changes to policies outside of the editing admin's scope of authority.
	return scopes.PolicyResourceScope(scopeOfOrigin).CanDependOnStateFromPolicyResourceAtScope(role.GetScope())
}

// RoleIsEnforceableAt reports whether the given role is validly assigned at the specified
// enforcement point. This is the authoritative combined check for whether a cross-resource role
// assignment is allowable via scoping rules. This check *must* be performed prior to the policies
// and privileges of a role being considered for enforcement in any context. Assignments that do not
// pass this check must have no effect.
func RoleIsEnforceableAt(role *scopedaccessv1.ScopedRole, point scopes.EnforcementPoint) bool {
	return RoleIsAssignableFromScopeOfOrigin(role, point.ScopeOfOrigin) &&
		RoleIsAssignableToScopeOfEffect(role, point.ScopeOfEffect)
}

// WeakValidatedAssignableScopes is a helper for iterating all well formed assignable scopes for a given role.
func WeakValidatedAssignableScopes(role *scopedaccessv1.ScopedRole) iter.Seq[string] {
	return func(yield func(string) bool) {
		for _, assignableScope := range role.GetSpec().GetAssignableScopes() {
			if err := scopes.WeakValidateGlob(assignableScope); err != nil {
				// ignore invalid assignable scopes
				continue
			}

			if !scopes.ScopeOfEffectGlob(assignableScope).IsAlwaysAssignableFromScopeOfOrigin(role.GetScope()) {
				// ignore assignable scopes that do not conform to assignment subjugation rules
				continue
			}

			if !yield(assignableScope) {
				return
			}
		}
	}
}

// WeakValidateRole valides a role to ensure it is free of obvious issues that would render it unusable and/or
// induce serious unintended behavior. Prefer using this function for validating roles loaded from "internal" sources
// (e.g. backend/control-plane), and [StrongValidateRole] for validating roles loaded from "external" sources (e.g. user input).
func WeakValidateRole(role *scopedaccessv1.ScopedRole) error {
	if err := commonValidateRole(role); err != nil {
		return trace.Wrap(err)
	}

	if err := scopes.WeakValidate(role.GetScope()); err != nil {
		return trace.BadParameter("scoped role %q has invalid scope: %v", role.GetMetadata().GetName(), err)
	}

	// NOTE: in strong validation, this is where we'd check that the assignable scopes are valid. In weak validation
	// we don't do that and instead rely on invalid assignable scopes being filtered out
	// and excluded during runtime assignment validation checks. This helps us ensure that outdated agents continue
	// to be able to understand and process the subset of assignments that they are able to reason about.

	return nil
}

// StrongValidateRole performs robust validation of a role to ensure it complies with all expected constraints. Prefer
// using this function for validating roles loaded from "external" sources (e.g. user input), and [WeakValidateRole] for
// validating roles loaded from "internal" sources (e.g. backend/control-plane).
func StrongValidateRole(role *scopedaccessv1.ScopedRole) error {
	if err := commonValidateRole(role); err != nil {
		return trace.Wrap(err)
	}

	if err := validateRoleName(role.GetMetadata().GetName()); err != nil {
		return trace.BadParameter("scoped role name %q does not conform to segment naming rules: %v", role.GetMetadata().GetName(), err)
	}

	if err := scopes.StrongValidate(role.GetScope()); err != nil {
		return trace.BadParameter("scoped role %q has invalid scope: %v", role.GetMetadata().GetName(), err)
	}

	if len(role.GetSpec().GetAssignableScopes()) == 0 {
		return trace.BadParameter("scoped role %q does not have any assignable scopes", role.GetMetadata().GetName())
	}

	if len(role.GetSpec().GetAssignableScopes()) > maxAssignableScopes {
		return trace.BadParameter("scoped role %q has too many assignable scopes (max %d)", role.GetMetadata().GetName(), maxAssignableScopes)
	}

	for _, scopeGlob := range role.GetSpec().GetAssignableScopes() {
		if err := scopes.StrongValidateGlob(scopeGlob); err != nil {
			return trace.BadParameter("scoped role %q has invalid assignable scope %q: %v", role.GetMetadata().GetName(), scopeGlob, err)
		}

		if scopes.Compare(scopeGlob, scopes.Root) == scopes.Equivalent {
			return trace.BadParameter("scoped role %q has root scope as an assignable scope, which is not permitted (use '/**' to allow assignment to all non-root scopes)", role.GetMetadata().GetName())
		}

		if !scopes.ScopeOfEffectGlob(scopeGlob).IsAlwaysAssignableFromScopeOfOrigin(role.GetScope()) {
			return trace.BadParameter("scoped role %q has assignable scope %q that is not a sub-scope of the role's scope %q", role.GetMetadata().GetName(), scopeGlob, role.GetScope())
		}
	}

	// verify that all rules are allowed for scoped roles
	for _, rule := range role.GetSpec().GetRules() {
		for _, resource := range rule.GetResources() {
			for _, verb := range rule.GetVerbs() {
				if !isAllowedScopedRule(resource, verb) {
					if verb == types.VerbRead && isAllowedScopedRule(resource, types.VerbReadNoSecrets) {
						return trace.BadParameter("scoped role %q has rule with verb %q that is too permissive for resource %q, use %q instead", role.GetMetadata().GetName(), verb, resource, types.VerbReadNoSecrets)
					}
					return trace.BadParameter("scoped role %q has rule with unsupported resource/verb combination: %q/%q", role.GetMetadata().GetName(), resource, verb)
				}
			}
		}
	}

	// verify that ssh logins are well-formed
	if login := validateDoesNotContain(role.GetSpec().GetSsh().GetLogins(), invalidChars); login != "" {
		// we currently don't support any form of wildcard/regex/substitution in scoped role
		// logins. we likely will support substitution in the future, but its best to disallow
		// it until that has landed.
		return trace.BadParameter("scoped role %q has invalid login %q", role.GetMetadata().GetName(), login)
	}

	// verify that at least one label entry is defined when SSH block is given - scoped roles are deny by default, so a wildcard entry for
	// labels must be explicitly provided if the role is meant to grant full access
	if role.GetSpec().GetSsh() != nil && len(role.GetSpec().GetSsh().GetLabels()) == 0 {
		return trace.BadParameter("scoped role %q has no spec.ssh.labels defined, please add at least one entry", role.GetMetadata().GetName())
	}

	// verify that ssh node labels are well-formed
	for _, label := range role.GetSpec().GetSsh().GetLabels() {
		// we currently don't support any form of wildcard/regex/substitution in scoped role
		// node labels. we likely will support such things in the future, but its best to disallow
		// them until that has landed.

		if strings.ContainsAny(label.GetName(), invalidLabelChars) {
			return trace.BadParameter("scoped role %q has invalid node label name %q", role.GetMetadata().GetName(), label.GetName())
		}
		if value := validateDoesNotContain(label.GetValues(), invalidLabelChars); value != "" {
			return trace.BadParameter("scoped role %q has invalid node label value %q for label %q", role.GetMetadata().GetName(), value, label.GetName())
		}
	}

	// verify that client_idle_timeout fields are valid Go duration strings
	if s := role.GetSpec().GetSsh().GetClientIdleTimeout(); s != "" {
		if _, err := time.ParseDuration(s); err != nil {
			return trace.BadParameter("scoped role %q has invalid ssh.client_idle_timeout %q: %v", role.GetMetadata().GetName(), s, err)
		}
	}
	if s := role.GetSpec().GetKube().GetClientIdleTimeout(); s != "" {
		if _, err := time.ParseDuration(s); err != nil {
			return trace.BadParameter("scoped role %q has invalid kube.client_idle_timeout %q: %v", role.GetMetadata().GetName(), s, err)
		}
	}
	if s := role.GetSpec().GetDefaults().GetClientIdleTimeout(); s != "" {
		if _, err := time.ParseDuration(s); err != nil {
			return trace.BadParameter("scoped role %q has invalid defaults.client_idle_timeout %q: %v", role.GetMetadata().GetName(), s, err)
		}
	}

	// verify that create_host_user_mode is a recognized value
	if mode := role.GetSpec().GetSsh().GetHostUserCreation().GetMode(); mode != "" {
		var hostUserMode types.CreateHostUserMode
		if err := hostUserMode.UnmarshalText([]byte(mode)); err != nil {
			return trace.BadParameter("scoped role %q has invalid ssh.host_user_creation.create_host_user_mode %q", role.GetMetadata().GetName(), mode)
		}
	}

	// verify that max_sessions is non-negative
	if ms := role.GetSpec().GetSsh().GetMaxSessions(); ms < 0 {
		return trace.BadParameter("scoped role %q has invalid ssh.max_sessions %d: must be non-negative", role.GetMetadata().GetName(), ms)
	}

	// verify that session_recording_mode fields are recognized values
	if mode := role.GetSpec().GetSsh().GetSessionRecording().GetMode(); mode != "" {
		switch constants.SessionRecordingMode(mode) {
		case constants.SessionRecordingModeStrict, constants.SessionRecordingModeBestEffort:
		default:
			return trace.BadParameter("scoped role %q has invalid ssh.session_recording_mode. %q: must be %q or %q",
				role.GetMetadata().GetName(), mode, constants.SessionRecordingModeStrict, constants.SessionRecordingModeBestEffort)
		}
	}

	if mode := role.GetSpec().GetDefaults().GetSessionRecording().GetMode(); mode != "" {
		switch constants.SessionRecordingMode(mode) {
		case constants.SessionRecordingModeStrict, constants.SessionRecordingModeBestEffort:
		default:
			return trace.BadParameter("scoped role %q has invalid defaults.session_recording_mode %q: must be %q or %q",
				role.GetMetadata().GetName(), mode, constants.SessionRecordingModeStrict, constants.SessionRecordingModeBestEffort)
		}
	}

	// verify that the lock.Mode is a recognized value for Defaults
	if lock := role.GetSpec().GetDefaults().GetLock(); lock != nil {
		if err := validateLock(lock); err != nil {
			return trace.BadParameter("scoped role %q has invalid defaults.lock.mode %q", role.GetMetadata().GetName(), lock.GetMode())
		}
	}
	// verify that lock.Mode is a recognized value for SSH
	if lock := role.GetSpec().GetSsh().GetLock(); lock != nil {
		if err := validateLock(lock); err != nil {
			return trace.BadParameter("scoped role %q has invalid ssh.lock.mode %q", role.GetMetadata().GetName(), lock.GetMode())
		}
	}

	if err := validateAppBlock(role.GetSpec().GetApp()); err != nil {
		return trace.BadParameter("scoped role %q %s", role.GetMetadata().GetName(), err)
	}

	// verify that kube block is well-formed
	if err := validateKubeBlock(role.GetSpec().GetKube()); err != nil {
		return trace.BadParameter("scoped role %q has %s", role.GetMetadata().GetName(), err)
	}

	// verify that workload_identity labels are well-formed
	for _, label := range role.GetSpec().GetWorkloadIdentity().GetLabels() {
		// we currently don't support any form of wildcard/regex/substitution in scoped role
		// workload identity labels. we likely will support such things in the future, but its
		// best to disallow them until that has landed.

		if strings.ContainsAny(label.GetName(), invalidLabelChars) {
			return trace.BadParameter("scoped role %q has invalid workload_identity label name %q", role.GetMetadata().GetName(), label.GetName())
		}
		if value := validateDoesNotContain(label.GetValues(), invalidLabelChars); value != "" {
			return trace.BadParameter("scoped role %q has invalid workload_identity label value %q for label %q", role.GetMetadata().GetName(), value, label.GetName())
		}
	}

	// verify that scoped role converts to a valid unscoped role
	if _, err := ScopedRoleToRole(role, role.GetScope()); err != nil {
		return trace.BadParameter("scoped role %q is malformed: %v", role.GetMetadata().GetName(), err)
	}

	return nil
}

// validateDoestNotContain checks that a given slice of string values do not contain any of the characters in the given
// invalid set. The first invalid value is returned to be included in any error messages.
func validateDoesNotContain(values []string, invalidSet string) string {
	for _, val := range values {
		if strings.ContainsAny(val, invalidSet) {
			return val
		}
	}

	return ""
}

func validateAppBlock(app *scopedaccessv1.ScopedRoleApp) error {
	if app == nil {
		return nil
	}

	labels := app.GetLabels()
	if len(labels) == 0 && app.GetLabelExpression() == "" {
		return trace.BadParameter("must define at least one app.labels entry or app.label expressions")
	}

	if expr := app.GetLabelExpression(); expr != "" {
		if _, err := label.ParseExpression(expr); err != nil {
			return trace.BadParameter("has invalid app.label_expression: %v", err)
		}
	}

	// verify that app labels are well formed
	for _, label := range labels {
		// we currently don't support any form of wildcard/regex/substitution in scoped role
		// app labels. we likely will support such things in the future, but its best to disallow
		// them until that has landed.

		if strings.ContainsAny(label.GetName(), invalidLabelChars) {
			return trace.BadParameter("has invalid app label name %q", label.GetName())
		}
		if value := validateDoesNotContain(label.GetValues(), invalidLabelChars); value != "" {
			return trace.BadParameter("has invalid app label value %q for label %q", value, label.GetName())
		}
	}

	// verify that lock.Mode is a recognized value for App
	if lock := app.GetLock(); lock != nil {
		if err := validateLock(lock); err != nil {
			return trace.BadParameter("has invalid app.lock.mode %q", lock.GetMode())
		}
	}

	if s := app.GetClientIdleTimeout(); s != "" {
		if _, err := time.ParseDuration(s); err != nil {
			return trace.BadParameter("has invalid app.client_idle_timeout %q: %v", s, err)
		}
	}
	return nil
}

func validateLock(lock *scopedaccessv1.Lock) error {
	mode := lock.GetMode()
	switch constants.LockingMode(mode) {
	// Allow for empty string - we will fall back to the cluster defaults - or best_effort if not set.
	// This matches current behavior for unscoped lock mode checks.
	case "":
	case constants.LockingModeBestEffort, constants.LockingModeStrict:
	default:
		return trace.Errorf("invalid lock mode")
	}
	return nil
}

func validateRoleName(name string) error {
	// note: having the scope name be validated as a segment name is a bit of an arbitrary choice, but its basically
	// equivalent to what we would want from a standalone name requirement, and there may even be some future benefit
	// if we ever need to encode a role assignment as a scope-like name.
	return trace.Wrap(scopes.StrongValidateSegment(name))
}

// commonValidateRole performs the subset of role validation common to both weak and strong validation.
func commonValidateRole(role *scopedaccessv1.ScopedRole) error {
	if role.GetMetadata().GetName() == "" {
		return trace.BadParameter("scoped role is missing metadata.name")
	}

	if role.GetKind() == "" {
		return trace.BadParameter("scoped role %q is missing kind", role.GetMetadata().GetName())
	}

	if role.GetKind() != KindScopedRole {
		return trace.BadParameter("scoped role %q has invalid kind %q, expected %q", role.GetMetadata().GetName(), role.GetKind(), KindScopedRole)
	}

	if role.GetSubKind() != "" {
		return trace.BadParameter("scoped role %q has unknown sub_kind %q", role.GetMetadata().GetName(), role.GetSubKind())
	}

	if role.GetVersion() == "" {
		return trace.BadParameter("scoped role %q is missing version", role.GetMetadata().GetName())
	}

	if role.GetVersion() != types.V1 {
		return trace.BadParameter("scoped role %q has unsupported version %q (expected %q)", role.GetMetadata().GetName(), role.GetVersion(), types.V1)
	}

	if role.GetScope() == "" {
		return trace.BadParameter("scoped role %q is missing scope", role.GetMetadata().GetName())
	}

	return nil
}

// WeakValidatedSubAssignments is a helper for iterating all well formed sub-assignments within a given assignment. Note that the concept
// of a well-formed sub-assignment is distinct from whether or not an assignment is "boken/invalidated" in the sense used when
// deciding whether or not an access-control check can be performed for a given scope. The only thing that is being filtered out
// by this iterator is sub-assignments that are so obviously misconfigured that we can't reason about them at all. Sub-assignments
// returned by this iterator may still be broken because they assign a nonexistent role, or to a scope that the target role is not
// assignable to.
func WeakValidatedSubAssignments(assignment *scopedaccessv1.ScopedRoleAssignment) iter.Seq[*scopedaccessv1.Assignment] {
	return func(yield func(*scopedaccessv1.Assignment) bool) {
		for _, subAssignment := range assignment.GetSpec().GetAssignments() {
			if subAssignment.GetRole() == "" {
				// ignore sub-assignments with missing role
				continue
			}

			if err := scopes.WeakValidate(subAssignment.GetScope()); err != nil {
				// ignore sub-assignments with invalid scopes
				continue
			}

			if !scopes.ScopeOfOrigin(assignment.GetScope()).IsAssignableToScopeOfEffect(subAssignment.GetScope()) {
				// ignore sub-assignments with scopes that do not conform to assignment subjugation rules
				continue
			}

			if !yield(subAssignment) {
				return
			}
		}
	}
}

// WeakValidateAssignment validates an assignment to ensure it is free of obvious issues that would render it unusable and/or
// induce serious unintended behavior. Prefer using this function for validating assignments loaded from "internal" sources
// (e.g. backend/control-plane), and [StrongValidateAssignment] for validating assignments loaded from "external" sources (e.g. user input).
func WeakValidateAssignment(assignment *scopedaccessv1.ScopedRoleAssignment) error {
	if err := commonValidateAssignment(assignment); err != nil {
		return trace.Wrap(err)
	}

	if err := scopes.WeakValidate(assignment.GetScope()); err != nil {
		return trace.BadParameter("scoped role assignment %q has invalid scope: %v", assignment.GetMetadata().GetName(), err)
	}

	// NOTE: in strong validation, this is where we'd check that the sub-assignments are valid. In weak validation
	// we don't do that and instead rely on invalid sub-assignments being filtered out and excluded during runtime
	// assignment resolution.

	return nil
}

// StrongValidateAssignment performs robust validation of an assignment to ensure it complies with all expected constraints. Prefer
// using this function for validating assignments loaded from "external" sources (e.g. user input), and [WeakValidateAssignment] for
// validating assignments loaded from "internal" sources (e.g. backend/control-plane).
func StrongValidateAssignment(assignment *scopedaccessv1.ScopedRoleAssignment) error {
	if err := commonValidateAssignment(assignment); err != nil {
		return trace.Wrap(err)
	}

	switch assignment.GetSubKind() {
	case SubKindDynamic, SubKindMaterialized:
	case "":
		return trace.BadParameter("scoped role assignment %q has empty sub_kind", assignment.GetMetadata().GetName())
	default:
		return trace.BadParameter("scoped role assignment %q has invalid sub_kind %q", assignment.GetMetadata().GetName(), assignment.GetSubKind())
	}

	if err := scopes.StrongValidateSegment(assignment.GetMetadata().GetName()); err != nil {
		return trace.BadParameter("scoped role assignment name %q does not conform to segment naming rules: %v", assignment.GetMetadata().GetName(), err)
	}

	if err := scopes.StrongValidate(assignment.GetScope()); err != nil {
		return trace.BadParameter("scoped role assignment %q has invalid scope: %v", assignment.GetMetadata().GetName(), err)
	}

	if len(assignment.GetSpec().GetAssignments()) == 0 {
		return trace.BadParameter("scoped role assignment %q does not assign any roles", assignment.GetMetadata().GetName())
	}

	if len(assignment.GetSpec().GetAssignments()) > MaxRolesPerAssignment {
		return trace.BadParameter("scoped role assignment %q contains too many sub-assignments (max %d)", assignment.GetMetadata().GetName(), MaxRolesPerAssignment)
	}

	// Assigning to Bot is mutually exclusive with assigning to User.
	botSet := assignment.GetSpec().GetBot() != ""
	var botScope string
	if botSet {
		if assignment.GetSpec().GetUser() != "" {
			return trace.BadParameter("scoped role assignment %q cannot have both spec.bot and spec.user set", assignment.GetMetadata().GetName())
		}
		bot, err := scopes.ParseQualifiedName(assignment.GetSpec().GetBot())
		if err != nil {
			return trace.BadParameter("scoped role assignment %q has invalid spec.bot: %v", assignment.GetMetadata().GetName(), err)
		}
		if err := bot.StrongValidate(); err != nil {
			return trace.BadParameter("scoped role assignment %q has invalid spec.bot: %v", assignment.GetMetadata().GetName(), err)
		}
		botScope = bot.Scope
	}

	for i, subAssignment := range assignment.GetSpec().GetAssignments() {
		if subAssignment.GetRole() == "" {
			return trace.BadParameter("scoped role assignment %q is missing role in sub-assignment %d", assignment.GetMetadata().GetName(), i)
		}

		// the role is referenced by scope-qualified name (`<roleScope>::<roleName>`); validate both components.
		role, err := scopes.ParseQualifiedName(subAssignment.GetRole())
		if err != nil {
			return trace.BadParameter("scoped role assignment %q has invalid role reference in sub-assignment %d: %v", assignment.GetMetadata().GetName(), i, err)
		}
		if err := role.StrongValidate(); err != nil {
			return trace.BadParameter("scoped role assignment %q has invalid role reference in sub-assignment %d: %v", assignment.GetMetadata().GetName(), i, err)
		}

		if err := scopes.StrongValidate(subAssignment.GetScope()); err != nil {
			return trace.BadParameter("scoped role assignment %q has invalid scope in sub-assignment %d: %v", assignment.GetMetadata().GetName(), i, err)
		}

		if !scopes.ScopeOfOrigin(role.Scope).IsAssignableToScopeOfEffect(subAssignment.GetScope()) {
			return trace.BadParameter("scoped role assignment %q sub-assignment %d references role %q whose scope is not equivalent or ancestral to the scope of effect %q", assignment.GetMetadata().GetName(), i, subAssignment.GetRole(), subAssignment.GetScope())
		}

		if scopes.Compare(subAssignment.GetScope(), scopes.Root) == scopes.Equivalent {
			return trace.BadParameter("scoped role assignment %q has sub-assignment %d with root scope, which is not permitted (root scope cannot be used as a scope of effect)", assignment.GetMetadata().GetName(), i)
		}

		if !scopes.ScopeOfOrigin(assignment.GetScope()).IsAssignableToScopeOfEffect(subAssignment.GetScope()) {
			return trace.BadParameter("scoped role assignment %q has sub-assignment %d with scope %q that is not a sub-scope of the assignment's scope %q", assignment.GetMetadata().GetName(), i, subAssignment.GetScope(), assignment.GetScope())
		}

		// As per the MWI Scopes RFD, we enforce a special requirement for Bot
		// assignments. Bot's can only be assigned privileges in scopes
		// equivalent or descendent to their scope.
		if botSet {
			assignmentScope := subAssignment.GetScope()
			if !scopes.ScopeOfOrigin(botScope).IsAssignableToScopeOfEffect(assignmentScope) {
				return trace.BadParameter(
					"scoped role assignment %q has sub-assignment %d with scope %q that is not a sub-scope of the bot's declared scope %q",
					assignment.GetMetadata().GetName(),
					i,
					subAssignment.GetScope(),
					botScope,
				)
			}
		}
	}

	return nil
}

func commonValidateAssignment(assignment *scopedaccessv1.ScopedRoleAssignment) error {
	if assignment.GetMetadata().GetName() == "" {
		return trace.BadParameter("scoped role assignment is missing metadata.name")
	}

	if assignment.GetKind() == "" {
		return trace.BadParameter("scoped role assignment %q is missing kind", assignment.GetMetadata().GetName())
	}

	if assignment.GetKind() != KindScopedRoleAssignment {
		return trace.BadParameter("scoped role assignment %q has invalid kind %q, expected %q", assignment.GetMetadata().GetName(), assignment.GetKind(), KindScopedRoleAssignment)
	}

	if assignment.GetVersion() == "" {
		return trace.BadParameter("scoped role assignment %q is missing version", assignment.GetMetadata().GetName())
	}

	if assignment.GetVersion() != types.V1 {
		return trace.BadParameter("scoped role assignment %q has unsupported version %q (expected %q)", assignment.GetMetadata().GetName(), assignment.GetVersion(), types.V1)
	}

	if assignment.GetScope() == "" {
		return trace.BadParameter("scoped role assignment %q is missing scope", assignment.GetMetadata().GetName())
	}

	if assignment.GetSpec().GetUser() == "" && assignment.GetSpec().GetBot() == "" {
		return trace.BadParameter("scoped role assignment %q is missing spec.user or spec.bot", assignment.GetMetadata().GetName())
	}

	return nil
}

// getNamespacedResourceAPIGroup maps the list of known Kubernetes resource kinds
// that are namespaced to their respective API group. It is copied from the kubernetesNamespacesResourceKinds
// map found in api/types/constants.go to prevent exporting it from the public API. Any changes should also be
// made in the original.
//
// Generated from `kubectl api-resources --namespaced=true -o name --sort-by=name` (kind k8s v1.32.2).
// The format is "<plural>.<apigroup>".
//
// Only used to validate the api_group field.
// If we have a match, we know we need a namespaced value, if we don't
// have a match, we don't know we don't. Best effort basis.
//
// Key: resource kind, value: api group.
func getNamespacedResourceAPIGroup(resource string) (string, bool) {
	switch resource {
	case "bindings", "services", "serviceaccounts", "secrets", "resourcequotas",
		"replicationcontrollers", "podtemplates", "pods", "limitranges", "endpoints",
		"configmaps", "persistentvolumeclaims":
		return "", true
	case "controllerrevisions":
		return "apps", true
	case "cronjobs":
		return "batch", true
	case "csistoragecapacities":
		return "storage.k8s.io", true
	case "daemonsets":
		return "apps", true
	case "deployments":
		return "apps", true
	case "endpointslices":
		return "discovery.k8s.io", true
	case "events":
		return "events.k8s.io", true
	case "horizontalpodautoscalers":
		return "autoscaling", true
	case "ingresses":
		return "networking.k8s.io", true
	case "jobs":
		return "batch", true
	case "leases":
		return "coordination.k8s.io", true
	case "localsubjectaccessreviews":
		return "authorization.k8s.io", true
	case "networkpolicies":
		return "networking.k8s.io", true
	case "poddisruptionbudgets":
		return "policy", true
	case "replicasets":
		return "apps", true
	case "rolebindings":
		return "rbac.authorization.k8s.io", true
	case "roles":
		return "rbac.authorization.k8s.io", true
	case "statefulsets":
		return "apps", true
	default:
		return "", false
	}
}

// validateKubeResources validates the following rules for each spec.kube.resources entry:
// - Kind doesn't map to an API group
// - Name is not empty
// - Namespace is not empty
// - ApiGroup not empty
// This validator is largely copied from the role v8 case of validateKubeResources found in api/types/role.go.
// It is ported to support scoped roles and the original should considered as well when making any changes.
func validateKubeResource(resource *scopedaccessv1.KubeResource) error {
	// NOTE(eriktate): errors must start with the field name producing the error so the final reported error
	// renders correctly.
	// (e.g. `scoped role "/my/scope::my-role" has invalid kube resource: spec.kube.resources[0].kind is required`)
	for _, verb := range resource.GetVerbs() {
		if !slices.Contains(types.KubernetesVerbs, verb) && verb != types.Wildcard {
			return trace.BadParameter("verbs contain invalid or unsupported %q; Supported: %v", verb, types.KubernetesVerbs)
		}
		if verb == types.Wildcard && len(resource.GetVerbs()) > 1 {
			return trace.BadParameter("verbs contain %q which cannot be used with other verbs", verb)
		}
	}

	if resource.GetKind() == "" {
		return trace.BadParameter("kind is required")
	}
	// If we have a kind in singular form for a known resource kind, check the api group.
	if slices.Contains(types.KubernetesResourcesKinds, resource.GetKind()) {
		// If the api group is a wildcard or matches a legacy group, then it is most definitely a mistake. Reject the role.
		if resource.GetApiGroup() == types.Wildcard || resource.GetApiGroup() == types.KubernetesResourcesV7KindGroups[resource.GetKind()] {
			return trace.BadParameter("kind %q is invalid. Please use plural name", resource.GetKind())
		}
	}
	// Only allow empty string for known core resources.
	if resource.GetApiGroup() == "" {
		if _, ok := types.KubernetesCoreResourceKinds[resource.GetKind()]; !ok {
			return trace.BadParameter("api_group is required for resource %q", resource.GetKind())
		}
	}
	// Best effort attempt to validate if the namespace field is needed.
	if resource.GetNamespace() == "" {
		if apiGroup, ok := getNamespacedResourceAPIGroup(resource.GetKind()); ok && apiGroup == resource.GetApiGroup() {
			return trace.BadParameter("namespace must be included for kind %q", resource.GetKind())
		}
	}

	return nil
}

// validateKubeBlock validates the given kube configuration in a scoped role spec.
func validateKubeBlock(kube *scopedaccessv1.ScopedRoleKube) error {
	// the kube block is not required for all scoped roles, so nil is
	// not an error
	if kube == nil {
		return nil
	}

	// verify that lock.Mode is a recognized value for Kube
	if lock := kube.GetLock(); lock != nil {
		if err := validateLock(lock); err != nil {
			return trace.BadParameter("invalid kube.lock.mode %q", lock.GetMode())
		}
	}

	// verify that at least one label entry is defined - scoped roles are deny by default, so a wildcard entry for
	// labels must be explicitly provided if the role is meant to grant full access
	if len(kube.GetLabels()) == 0 {
		return trace.BadParameter("no spec.kube.labels defined, please add at least one entry")
	}

	// verify that kube labels are well-formed
	for _, label := range kube.GetLabels() {
		if strings.ContainsAny(label.GetName(), invalidLabelChars) {
			return trace.BadParameter("invalid kube label name %q", label.GetName())
		}
		if value := validateDoesNotContain(label.GetValues(), invalidLabelChars); value != "" {
			return trace.BadParameter("invalid kube label value %q for label %q", value, label.GetName())
		}
	}

	// verify that at least one resource is defined - scoped roles are deny by default, so a wildcard entry for
	// resources must be explicitly provided when resource-based RBAC is not desired
	if len(kube.GetResources()) == 0 {
		return trace.BadParameter("no spec.kube.resources defined, if resource-based RBAC is not required please configure explicit wildcard access")
	}

	// verify that kube resources are well-formed
	for idx, resource := range kube.GetResources() {
		if err := validateKubeResource(resource); err != nil {
			return trace.BadParameter("invalid kube resource: spec.kube.resources[%d].%s", idx, err)
		}
	}

	// verify that kube groups are well-formed
	if group := validateDoesNotContain(kube.GetGroups(), invalidChars); group != "" {
		// we currently don't support any form of wildcard/regex/substitution in scoped role
		// kube groups. we likely will support substitution in the future, but its best to disallow
		// it until that has landed.
		return trace.BadParameter("invalid kube group %q", group)
	}

	// verify that kube users are well-formed
	if user := validateDoesNotContain(kube.GetUsers(), invalidChars); user != "" {
		// we currently don't support any form of wildcard/regex/substitution in scoped role
		// kube users. we likely will support substitution in the future, but its best to disallow
		// it until that has landed.
		return trace.BadParameter("invalid kube user %q", user)
	}

	return nil
}
