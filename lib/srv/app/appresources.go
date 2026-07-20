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

// minimalV9Decision is the outcome of the minimal v9 default-deny check
// for one HTTP app request. Only allow_all is honored.
//
// TODO(@juliaogris): Replace with per-request rule matching from the
// upcoming lib/appresource engine package.
type minimalV9Decision struct {
	// enforced is true when a v9 role grants the app, so v9 default-deny
	// governs the request instead of v8 pass-through.
	enforced bool
	// allowed is true when a granting v9 role sets allow_all, which
	// forwards the request untouched, exactly as v8 did.
	allowed bool
	// droppedRoles names the pre-v9 roles dropped because a v9 role grants
	// the same app. They are logged, never allowed to re-open access.
	droppedRoles []string
}

// preV9RoleVersions lists the role versions that predate v9 default-deny.
var preV9RoleVersions = []string{types.V1, types.V2, types.V3, types.V4, types.V5, types.V6, types.V7, types.V8}

// decideMinimalV9 applies the minimal v9 policy to the caller's roles that
// grant app. If no v9-or-above role grants it, the request keeps full v8
// behavior. If one does, pre-v9 roles granting the same app are dropped,
// and the request is allowed only when a granting v9-or-above role sets a
// single allow_all rule and no role carries deny-side app rules. A version
// above v9 is enforced like v9, so a role from a newer version denies
// rather than allows.
//
// TODO(@juliaogris): Replace with per-request rule matching from the
// upcoming lib/appresource engine package.
func decideMinimalV9(roles []types.Role, app types.Application, username string, traits wrappers.Traits) (minimalV9Decision, error) {
	// This version cannot evaluate deny-side app rules, which could only
	// occur in roles from newer versions. Deny beats allow across the
	// whole role set, so any role carrying them blocks allow_all
	// and the request is denied.
	denyAppRules := slices.ContainsFunc(roles, func(role types.Role) bool {
		return len(role.GetAppResources(types.Deny)) > 0
	})

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
		if !denyAppRules && types.AppResourcesAllowAll(role.GetAppResources(types.Allow), role.GetAppResources(types.Deny)) {
			decision.allowed = true
		}
	}
	if !decision.enforced {
		decision.droppedRoles = nil
	}
	return decision, nil
}

// roleGrantsApp reports whether role grants access to app through its
// allow app_labels while its deny app_labels do not exclude it. It mirrors
// the namespace and label stages of [services.RoleSet] access checking.
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
// returns true when the request is denied, after writing a 403 response
// naming the deny kind. Cloud apps (AWS console, Azure, GCP) and LLM apps
// never reach this path. Earlier cases in serveHTTP handle them.
//
// TODO(@juliaogris): Replace with per-request rule matching from the
// upcoming lib/appresource engine package.
func (c *ConnectionsHandler) enforceMinimalV9(w http.ResponseWriter, r *http.Request, authCtx *authz.Context, app types.Application) (bool, error) {
	identity := authCtx.Identity.GetIdentity()
	decision, err := decideMinimalV9(authCtx.Checker.Roles(), app, identity.Username, authCtx.Checker.Traits())
	if err != nil {
		return false, trace.Wrap(err)
	}

	if len(decision.droppedRoles) > 0 && c.v9WarnOnce("drop", identity.Username, app.GetName()) {
		c.log.WarnContext(r.Context(), "Dropped v8-or-older roles that grant a v9-governed app; v8 roles cannot re-open unrestricted access.",
			"app", app.GetName(),
			"user", identity.Username,
			"dropped_roles", decision.droppedRoles,
		)
	}

	if !decision.enforced || decision.allowed {
		return false, nil
	}

	if isCORSPreflight(r) && c.v9WarnOnce("cors", identity.Username, app.GetName()) {
		c.log.WarnContext(r.Context(), "Denied CORS preflight: the app denies requests by default and no v9 rule allows OPTIONS.",
			"app", app.GetName(),
			"user", identity.Username,
		)
	}

	http.Error(w, denyKindRequestNotAllowed, http.StatusForbidden)
	return true, nil
}

// isCORSPreflight reports whether r is a CORS preflight request, an
// OPTIONS request carrying both the Origin and
// Access-Control-Request-Method headers.
func isCORSPreflight(r *http.Request) bool {
	return r.Method == http.MethodOptions &&
		r.Header.Get("Origin") != "" &&
		r.Header.Get("Access-Control-Request-Method") != ""
}
