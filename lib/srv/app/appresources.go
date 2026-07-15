/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package app

import (
	"net/http"
	"slices"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/wrappers"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/services"
)

// denyKindRequestNotAllowed is the deny kind reported to the client when a v9
// role denies an HTTP app request by default.
const denyKindRequestNotAllowed = "teleport_request_not_allowed"

// minimalV9Decision is the outcome of the minimal v9 default-deny check for
// one HTTP app request. Path, method, and where matchers land later, so this
// release honors only unsafe_allow_all.
type minimalV9Decision struct {
	// enforced is true when at least one v9 role grants the app, so v9
	// default-deny governs the request instead of v8 pass-through.
	enforced bool
	// allowed is true when a granting v9 role sets unsafe_allow_all, which
	// forwards the request untouched, exactly as v8 did.
	allowed bool
	// droppedRoles names the v8-or-older roles dropped because a v9 role grants
	// the same app. They are logged, never allowed to re-open access.
	droppedRoles []string
}

// preV9RoleVersions lists the role versions that predate v9 default-deny.
var preV9RoleVersions = []string{types.V1, types.V2, types.V3, types.V4, types.V5, types.V6, types.V7, types.V8}

// decideMinimalV9 applies the minimal v9 policy to the caller's roles that
// grant app. If no v9-or-above role grants it, the request keeps full v8
// behavior. If one does, any pre-v9 role granting the same app is dropped,
// and the request is allowed only when a granting v9-or-above role sets
// unsafe_allow_all and carries no deny-side app rules.
//
// A version above v9 enforces default-deny here exactly like v9, so a role
// written by a later release fails closed rather than open when it reaches
// an agent of this release unstripped.
func decideMinimalV9(roles []types.Role, app types.Application, username string, traits wrappers.Traits) (minimalV9Decision, error) {
	var decision minimalV9Decision
	for _, role := range roles {
		granted, err := roleGrantsApp(role, app, username, traits)
		if err != nil {
			return minimalV9Decision{}, trace.Wrap(err)
		}
		if !granted {
			continue
		}
		if slices.Contains(preV9RoleVersions, role.GetVersion()) {
			decision.droppedRoles = append(decision.droppedRoles, role.GetName())
			continue
		}
		decision.enforced = true
		allowsAll := slices.ContainsFunc(role.GetAppResources(types.Allow), types.AppResource.IsPureUnsafeAllowAll)
		// Deny-side app rules are rejected at write time in this release, but
		// a later release may give them restricting semantics. A role carrying
		// any is not plain unsafe_allow_all and fails closed.
		hasDenyAppRules := len(role.GetAppResources(types.Deny)) > 0 || len(role.GetAppResourcesExpressions(types.Deny)) > 0
		if allowsAll && !hasDenyAppRules {
			decision.allowed = true
		}
	}
	if !decision.enforced {
		decision.droppedRoles = nil
	}
	return decision, nil
}

// roleGrantsApp reports whether role grants access to app through its allow
// app_labels while its deny app_labels do not exclude it. It mirrors the
// namespace and label stages of [services.RoleSet] access checking: a deny or
// allow stanza applies only when the role's namespaces for that condition
// match the app's namespace, exactly as in checkAccess.
func roleGrantsApp(role types.Role, app types.Application, username string, traits wrappers.Traits) (bool, error) {
	namespace := types.ProcessNamespace(app.GetMetadata().Namespace)
	if matched, _ := services.MatchNamespace(role.GetNamespaces(types.Deny), namespace); matched {
		denyMatchers, err := role.GetLabelMatchers(types.Deny, types.KindApp)
		if err != nil {
			return false, trace.Wrap(err)
		}
		denied, _, err := services.CheckLabelsMatch(types.Deny, denyMatchers, username, traits, app, false)
		if err != nil {
			return false, trace.Wrap(err)
		}
		if denied {
			return false, nil
		}
	}
	if matched, _ := services.MatchNamespace(role.GetNamespaces(types.Allow), namespace); !matched {
		return false, nil
	}
	allowMatchers, err := role.GetLabelMatchers(types.Allow, types.KindApp)
	if err != nil {
		return false, trace.Wrap(err)
	}
	allowed, _, err := services.CheckLabelsMatch(types.Allow, allowMatchers, username, traits, app, false)
	return allowed, trace.Wrap(err)
}

// enforceMinimalV9 applies v9 default-deny to a plain HTTP app request. It
// returns true when the request is denied, after writing a 403 response naming
// the deny kind. A granting v9 role with unsafe_allow_all, or the absence of
// any granting v9 role, lets the request proceed.
//
// Cloud apps (AWS console, Azure, GCP) and LLM apps are exempt from v9
// enforcement and never reach this path; they are handled by earlier cases in
// serveHTTP.
func (c *ConnectionsHandler) enforceMinimalV9(w http.ResponseWriter, r *http.Request, authCtx *authz.Context, app types.Application) (bool, error) {
	identity := authCtx.Identity.GetIdentity()
	decision, err := decideMinimalV9(authCtx.Checker.Roles(), app, identity.Username, authCtx.Checker.Traits())
	if err != nil {
		return false, trace.Wrap(err)
	}

	if len(decision.droppedRoles) > 0 {
		dropKey := identity.Username + "\x00" + app.GetName()
		if _, warned := c.v9DropWarned.LoadOrStore(dropKey, struct{}{}); !warned {
			c.log.WarnContext(r.Context(), "Dropped v8-or-older roles that grant a v9-governed app; v8 roles cannot re-open unrestricted access.",
				"app", app.GetName(),
				"user", identity.Username,
				"dropped_roles", decision.droppedRoles,
			)
		}
	}

	if !decision.enforced || decision.allowed {
		return false, nil
	}

	if isCORSPreflight(r) {
		c.log.WarnContext(r.Context(), "Denied CORS preflight: the app denies requests by default and no v9 rule allows OPTIONS.",
			"app", app.GetName(),
			"user", identity.Username,
		)
	}

	http.Error(w, denyKindRequestNotAllowed, http.StatusForbidden)
	return true, nil
}

// isCORSPreflight reports whether r is a CORS preflight request: an OPTIONS
// request carrying both the Origin and Access-Control-Request-Method headers.
func isCORSPreflight(r *http.Request) bool {
	return r.Method == http.MethodOptions &&
		r.Header.Get("Origin") != "" &&
		r.Header.Get("Access-Control-Request-Method") != ""
}
