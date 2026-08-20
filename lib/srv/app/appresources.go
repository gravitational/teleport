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
	"context"
	"net/http"
	"slices"

	"github.com/gravitational/teleport"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/types/wrappers"
	"github.com/gravitational/teleport/lib/appresource"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/app/common"
	"github.com/gravitational/teleport/lib/tlsca"
)

// denyBodyRequestNotAllowed is the 403 response body for a request no v9 rule
// allows.
const denyBodyRequestNotAllowed = string(appresource.DenyNotAllowed)

// denyBodyInternalAccessError is the 500 response body for a denial the
// caller cannot act on, where an admin must update a role or upgrade the
// agent. The audit event contains the reason. The client does not
// receive it.
const denyBodyInternalAccessError = "teleport_internal_access_error"

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
	// versionSkew is true when the roles set app rules, predicates, or
	// versions a newer Teleport wrote and this version cannot evaluate.
	// The cases are deny-side rules or predicates, allow rules beyond a
	// single pure allow_all, allow-side predicates, and a role version
	// above v9.
	versionSkew bool
}

// denyResponse is what the agent sends and records for one denied request.
type denyResponse struct {
	// kind is the deny kind recorded in the audit event.
	kind appresource.DenyKind
	// body is the HTTP response body sent to the client.
	body string
	// status is the HTTP status code sent to the client.
	status int
}

// newDenyResponse builds the response for a denied request. Version skew
// returns a 500 with a generic body, because only an admin can act on it.
func newDenyResponse(decision minimalV9Decision) denyResponse {
	if decision.versionSkew {
		return denyResponse{
			kind:   appresource.DenyRoleVersionUnsupported,
			body:   denyBodyInternalAccessError,
			status: http.StatusInternalServerError,
		}
	}
	return denyResponse{
		kind:   appresource.DenyNotAllowed,
		body:   denyBodyRequestNotAllowed,
		status: http.StatusForbidden,
	}
}

// roleVersionPredatesV9 reports whether the role version predates v9
// default-deny.
func roleVersionPredatesV9(version string) bool {
	switch version {
	case types.V1, types.V2, types.V3, types.V4, types.V5, types.V6, types.V7, types.V8:
		return true
	}
	return false
}

// decideMinimalV9 applies the minimal v9 policy to the caller's roles that
// grant app. If only pre-v9 roles grant it, the request keeps full v8
// behavior. Otherwise pre-v9 roles granting the app are dropped and the
// request is denied unless a granting v9 role holds a single allow_all rule,
// sets no app_resources_expressions, and no role sets a deny-side rule or
// predicate. A role newer than v9 still enforces but never allows, since it
// may set restrictions this version cannot evaluate.
//
// TODO(@juliaogris): Replace with per-request rule matching from the
// upcoming lib/appresource engine package.
func decideMinimalV9(roles []types.Role, app types.Application, username string, traits wrappers.Traits) minimalV9Decision {
	// This version cannot evaluate deny-side app rules or predicates, which
	// could only occur in roles from newer versions. Deny beats allow across
	// the whole role set, so any role that sets either blocks allow_all and
	// the request is denied.
	denyAppRules := slices.ContainsFunc(roles, func(role types.Role) bool {
		return len(role.GetAppResources(types.Deny)) > 0 ||
			len(role.GetAppResourcesExpressions(types.Deny)) > 0
	})

	decision := minimalV9Decision{versionSkew: denyAppRules}
	for _, role := range roles {
		if !services.RoleGrantsResource(role, app, username, traits) {
			continue
		}
		if roleVersionPredatesV9(role.GetVersion()) {
			decision.droppedRoles = append(decision.droppedRoles, role.GetName())
			continue
		}
		decision.enforced = true
		if role.GetVersion() != types.V9 {
			// A newer version may add restrictions this agent cannot
			// evaluate, so a role above v9 denies rather than allows, even
			// when its known fields look like a plain allow_all.
			decision.versionSkew = true
			continue
		}
		allow := role.GetAppResources(types.Allow)
		allowExpressions := role.GetAppResourcesExpressions(types.Allow)
		switch {
		case len(allowExpressions) > 0:
			// A predicate restricts the rules it accompanies, and this
			// version cannot evaluate one, so the role denies whether or
			// not its declarative rules read as allow_all.
			decision.versionSkew = true
		case types.AppResourcesAllowAll(allow, role.GetAppResources(types.Deny)):
			decision.allowed = !denyAppRules
		case len(allow) > 0:
			// This version can only write a single pure allow_all rule, so
			// any other non-empty rule set must come from a newer version.
			decision.versionSkew = true
		}
	}
	if !decision.enforced {
		decision.droppedRoles = nil
		decision.versionSkew = false
	}
	return decision
}

// enforceMinimalV9 applies role v9 fine-grained app access rules to a plain
// HTTP app request.
//
// It returns true in the denial case, when a response has already been
// written, and false otherwise. Every denial emits one audit event with the
// reason. Cloud apps (AWS console, Azure, GCP) and LLM apps should never
// reach this path but they are defensively skipped.
//
// TODO(@juliaogris): Replace with per-request rule matching from the
// upcoming lib/appresource engine package.
func (c *ConnectionsHandler) enforceMinimalV9(w http.ResponseWriter, r *http.Request, authCtx *authz.Context, app types.Application) (bool, error) {
	identity := authCtx.Identity.GetIdentity()
	log := c.log.With("app", app.GetName(), "user", identity.Username)
	if !isGovernedByAppResources(app) {
		// Skip rather than deny, because v9 does not restrict these app types
		// and denying would break a working app on a routing bug.
		if c.v9WarnOnce("apptype", identity.Username, app.GetName()) {
			log.WarnContext(r.Context(), "Skipped app_resources enforcement because RoleV9 does not govern this app type.", "app_sub_kind", app.GetSubKind())
		}
		return false, nil
	}

	decision := decideMinimalV9(authCtx.Checker.Roles(), app, identity.Username, authCtx.Checker.Traits())

	if !decision.enforced {
		// Only v8 or older roles grant this app, so v8 pass-through applies.
		return false, nil
	}

	if len(decision.droppedRoles) > 0 && c.v9WarnOnce("drop", identity.Username, app.GetName()) {
		log.WarnContext(r.Context(), "Dropped v8-or-older roles that grant a v9-governed app; v8 roles cannot re-open unrestricted access.", "dropped_roles", decision.droppedRoles)
	}

	if decision.allowed {
		return false, nil
	}

	if decision.versionSkew && c.v9WarnOnce("skew", identity.Username, app.GetName()) {
		log.WarnContext(r.Context(), "Denied app request: the user's roles carry app_resources rules or role versions that this Teleport version does not implement, and unimplemented rules deny by default. Upgrade this app agent to enforce the intended rules.")
	}

	if isCORSPreflight(r) && c.v9WarnOnce("cors", identity.Username, app.GetName()) {
		log.WarnContext(r.Context(), "Denied CORS preflight: the app denies requests by default and no v9 rule allows OPTIONS.")
	}

	deny := newDenyResponse(decision)
	c.emitRequestDenied(r, &identity, app, deny.kind)
	http.Error(w, deny.body, deny.status)
	return true, nil
}

// isGovernedByAppResources reports whether v9 app_resources rules govern this
// app type.
func isGovernedByAppResources(app types.Application) bool {
	return !app.IsAWSConsole() && !app.IsAzureCloud() && !app.IsGCP() && !app.IsLLM() &&
		app.GetSubKind() != types.KindIdentityCenterAccount
}

// emitRequestDenied emits one audit event for a request denied under
// fine-grained app access roles. The event is not rate limited.
func (c *ConnectionsHandler) emitRequestDenied(r *http.Request, identity *tlsca.Identity, app types.Application, denyKind appresource.DenyKind) {
	event := &apievents.AppSessionRequestDenied{
		Metadata: apievents.Metadata{
			Type:        events.AppSessionRequestDeniedEvent,
			Code:        events.AppSessionRequestDeniedCode,
			ClusterName: identity.RouteToApp.ClusterName,
		},
		UserMetadata: identity.GetUserMetadata(),
		SessionMetadata: apievents.SessionMetadata{
			WithMFA:          identity.MFAVerified,
			PrivateKeyPolicy: string(identity.PrivateKeyPolicy),
		},
		ServerMetadata: apievents.ServerMetadata{
			ServerVersion:   teleport.Version,
			ServerID:        c.cfg.HostID,
			ServerNamespace: apidefaults.Namespace,
		},
		ConnectionMetadata: apievents.ConnectionMetadata{RemoteAddr: identity.LoginIP},
		AppMetadata:        *common.MakeAppMetadata(app),
		Method:             r.Method,
		Path:               r.URL.Path,
		DenyKind:           string(denyKind),
		AppSessionId:       identity.RouteToApp.SessionID,
	}
	// Detach from ctx cancellation: a client disconnecting as the request is
	// denied would otherwise cause the emitter to drop this record.
	emitCtx := context.WithoutCancel(r.Context())
	err := c.cfg.Emitter.EmitAuditEvent(emitCtx, event)
	if err != nil {
		c.log.WarnContext(emitCtx, "Failed to emit audit event for a denied app request.", "error", err, "app", app.GetName(), "user", identity.Username)
	}
}

// isCORSPreflight reports whether r is a CORS preflight request, an
// OPTIONS request carrying both the Origin and
// Access-Control-Request-Method headers.
func isCORSPreflight(r *http.Request) bool {
	return r.Method == http.MethodOptions &&
		r.Header.Get("Origin") != "" &&
		r.Header.Get("Access-Control-Request-Method") != ""
}
