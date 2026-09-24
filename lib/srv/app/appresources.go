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
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

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

// denyKindInternalAccessError is the deny kind a client reads for a denial
// only an admin can act on, by updating a role or upgrading the agent. The
// real kind goes to the audit event, not to the client.
const denyKindInternalAccessError appresource.DenyKind = "teleport_internal_access_error"

// contentTypeJSON is the Content-Type of every deny response body.
const contentTypeJSON = "application/json"

// The kinds of v9 enforcement warning. Each kind is logged once per user and
// app.
const (
	warnAppType  = "apptype"
	warnDropped  = "drop"
	warnDenyRule = "deny"
	warnSkew     = "skew"
	warnEvaluate = "evaluate"
	warnCORS     = "cors"
)

// denyResponse is the HTTP response for one denied request.
type denyResponse struct {
	// body is the HTTP response body sent to the client.
	body string
	// status is the HTTP status code sent to the client.
	status int
}

// newDenyResponse returns the response for a deny decision. Every body is the
// JSON of [appresource.Decision.ClientJSON]. A denial only an admin can act on
// (DenyRoleVersionUnsupported, DenyTooManyRules) is a 500 whose body names
// denyKindInternalAccessError instead of the real kind. A malformed path
// (DenyInvalidRequest) is a 400, and any other denial is a 403.
func newDenyResponse(decision appresource.Decision) denyResponse {
	switch decision.Deny.Kind {
	case appresource.DenyRoleVersionUnsupported, appresource.DenyTooManyRules:
		redacted := appresource.Decision{Deny: &appresource.DenyDetails{Kind: denyKindInternalAccessError}}
		return denyResponse{body: string(redacted.ClientJSON()), status: http.StatusInternalServerError}
	case appresource.DenyInvalidRequest:
		return denyResponse{body: string(decision.ClientJSON()), status: http.StatusBadRequest}
	}
	return denyResponse{body: string(decision.ClientJSON()), status: http.StatusForbidden}
}

// write sends the response.
func (d denyResponse) write(w http.ResponseWriter) {
	body := d.body + "\n"
	w.Header().Set("Content-Type", contentTypeJSON)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	w.WriteHeader(d.status)
	fmt.Fprint(w, body)
}

// versionSkewDecision returns the deny decision for a request this version
// of Teleport cannot evaluate.
func versionSkewDecision(roles ...string) *appresource.Decision {
	return &appresource.Decision{Deny: &appresource.DenyDetails{Kind: appresource.DenyRoleVersionUnsupported}, Roles: roles}
}

// stripRequestOverrideHeaders removes the request method and path override
// headers, except on a request an allow_all rule allowed.
func stripRequestOverrideHeaders(r *http.Request, decision appresource.Decision) {
	if decision.Allow != nil && decision.Allow.AllowAll {
		return
	}
	for name := range r.Header {
		if common.IsRequestOverrideHeader(name) {
			delete(r.Header, name)
		}
	}
}

// appResourcesDenial applies the app_resources rules of the caller's roles to
// a plain HTTP app request. It returns nil when the request is allowed, and
// the response to write when it is denied. The app kinds
// [appresource.GovernsApp] excludes should never reach this path but they are
// defensively skipped.
func (c *ConnectionsHandler) appResourcesDenial(r *http.Request, authCtx *authz.Context, app types.Application) *denyResponse {
	identity := authCtx.Identity.GetIdentity()
	log := c.log.With("app", app.GetName(), "user", identity.Username)
	if !appresource.GovernsApp(app) {
		// Skip rather than deny, because v9 does not restrict these app types
		// and denying would break a working app on a routing bug.
		if c.v9WarnOnce(warnAppType, identity.Username, app.GetName()) {
			log.WarnContext(r.Context(), "Skipped app_resources enforcement because RoleV9 does not govern this app type.", "app_sub_kind", app.GetSubKind())
		}
		return nil
	}

	decision := c.decideAppResources(r, log, authCtx, app)
	if decision == nil {
		// Only v8 or older roles grant this app, so v8 pass-through applies.
		return nil
	}
	if decision.Allowed {
		stripRequestOverrideHeaders(r, *decision)
		c.emitRequestAllowed(r, log, &identity, app, *decision)
		return nil
	}

	if isCORSPreflight(r) && c.v9WarnOnce(warnCORS, identity.Username, app.GetName()) {
		log.WarnContext(r.Context(), "Denied CORS preflight: no rule allows the OPTIONS method, so the preflight is denied by default.", "missing_method", http.MethodOptions)
	}

	deny := newDenyResponse(*decision)
	c.emitRequestDenied(r, log, &identity, app, *decision)
	return &deny
}

// decideAppResources evaluates the request against the app_resources rules of
// the caller's roles that grant app access. It returns nil when no role of v9
// or newer grants app access, so the request keeps full v8 behavior.
func (c *ConnectionsHandler) decideAppResources(r *http.Request, log *slog.Logger, authCtx *authz.Context, app types.Application) *appresource.Decision {
	identity := authCtx.Identity.GetIdentity()
	roles := authCtx.Checker.Roles()
	traits := authCtx.Checker.Traits()

	partition := appresource.PartitionRoles(grantingRoles(roles, app, identity.Username, traits))
	if !partition.Enforced() {
		return nil
	}
	// Deny overrides allow, so a rule in the deny block this version of
	// Teleport cannot evaluate denies the request, regardless of labels or
	// other roles.
	if name, ok := appresource.HasDenyRules(roles); ok {
		if c.v9WarnOnce(warnDenyRule, identity.Username, app.GetName()) {
			log.WarnContext(r.Context(), "Denied app request: a role sets app_resources or app_resources_expressions under deny, which this Teleport version does not implement, and unimplemented rules deny by default. Remove the deny rule to restore access.", "role", name)
		}
		return versionSkewDecision(name)
	}
	if len(partition.IgnoredRoleNames) > 0 && c.v9WarnOnce(warnDropped, identity.Username, app.GetName()) {
		log.WarnContext(r.Context(), "Dropped v8-or-older roles that grant a v9-governed app; v8 roles cannot re-open unrestricted access.", "dropped_roles", partition.IgnoredRoleNames)
	}
	if len(partition.Errors) > 0 && c.v9WarnOnce(warnSkew, identity.Username, app.GetName()) {
		errs := make([]error, 0, len(partition.Errors))
		for _, err := range partition.Errors {
			errs = append(errs, err)
		}
		log.WarnContext(r.Context(), "Dropped roles with app_resources rules or role versions this Teleport version does not implement. Unimplemented rules deny by default. Upgrade this app agent to enforce the intended rules.", "error", errors.Join(errs...))
	}

	request := appresource.Request{Method: r.Method, Path: r.URL.EscapedPath()}
	caller := appresource.Identity{Name: identity.Username, Roles: authCtx.Checker.RoleNames(), Traits: traits}
	decision, err := partition.Evaluate(request, caller)
	if err != nil {
		if c.v9WarnOnce(warnEvaluate, identity.Username, app.GetName()) {
			log.WarnContext(r.Context(), "Denied app request: a rule could not be evaluated.", "error", err)
		}
		return versionSkewDecision(partition.RoleNames()...)
	}
	return &decision
}

// grantingRoles returns the roles that grant app to the user.
func grantingRoles(roles []types.Role, app types.Application, username string, traits wrappers.Traits) []types.Role {
	var granting []types.Role
	for _, role := range roles {
		if services.RoleGrantsResource(role, app, username, traits) {
			granting = append(granting, role)
		}
	}
	return granting
}

// emitRequestDenied emits one audit event for a request denied under
// fine-grained app access roles. The event is not rate limited.
func (c *ConnectionsHandler) emitRequestDenied(r *http.Request, log *slog.Logger, identity *tlsca.Identity, app types.Application, decision appresource.Decision) {
	event := &apievents.AppSessionRequestDenied{
		Metadata: apievents.Metadata{
			Type:        events.AppSessionRequestDeniedEvent,
			Code:        events.AppSessionRequestDeniedCode,
			ClusterName: identity.RouteToApp.ClusterName,
		},
		UserMetadata:       identity.GetUserMetadata(),
		SessionMetadata:    requestSessionMetadata(identity),
		ServerMetadata:     c.serverMetadata(),
		ConnectionMetadata: apievents.ConnectionMetadata{RemoteAddr: identity.LoginIP},
		AppMetadata:        *common.MakeAppMetadata(app),
		Method:             r.Method,
		Path:               r.URL.EscapedPath(),
		DenyKind:           string(decision.Deny.Kind),
		AppSessionId:       identity.RouteToApp.SessionID,
		EvaluatedRoles:     decision.Roles,
		Hints:              denyHints(decision.Deny.Hints),
	}
	c.emitRequestEvent(r.Context(), log, event)
}

// emitRequestAllowed emits one audit event for a request allowed by a rule
// that sets allow_code. The event is not rate limited.
func (c *ConnectionsHandler) emitRequestAllowed(r *http.Request, log *slog.Logger, identity *tlsca.Identity, app types.Application, decision appresource.Decision) {
	if decision.Allow == nil || decision.Allow.Code == "" {
		return
	}
	event := &apievents.AppSessionRequestAllowed{
		Metadata: apievents.Metadata{
			Type:        events.AppSessionRequestAllowedEvent,
			Code:        events.AppSessionRequestAllowedCode,
			ClusterName: identity.RouteToApp.ClusterName,
		},
		UserMetadata:       identity.GetUserMetadata(),
		SessionMetadata:    requestSessionMetadata(identity),
		ServerMetadata:     c.serverMetadata(),
		ConnectionMetadata: apievents.ConnectionMetadata{RemoteAddr: identity.LoginIP},
		AppMetadata:        *common.MakeAppMetadata(app),
		Method:             r.Method,
		Path:               r.URL.EscapedPath(),
		AppSessionId:       identity.RouteToApp.SessionID,
		AllowRole:          decision.Allow.Role,
		AllowCode:          decision.Allow.Code,
		AllowReason:        decision.Allow.Reason,
	}
	c.emitRequestEvent(r.Context(), log, event)
}

// emitRequestEvent emits event, detached from ctx cancellation.
func (c *ConnectionsHandler) emitRequestEvent(ctx context.Context, log *slog.Logger, event apievents.AuditEvent) {
	// A client disconnecting as the request is decided would otherwise cause
	// the emitter to drop this record.
	emitCtx := context.WithoutCancel(ctx)
	err := c.cfg.Emitter.EmitAuditEvent(emitCtx, event)
	if err != nil {
		log.WarnContext(emitCtx, "Failed to emit audit event for an app request.", "error", err, "event", event.GetType())
	}
}

// requestSessionMetadata returns the session metadata of a request audit
// event. The session ID is unset, so each event gets its own audit key.
func requestSessionMetadata(identity *tlsca.Identity) apievents.SessionMetadata {
	return apievents.SessionMetadata{
		WithMFA:          identity.MFAVerified,
		PrivateKeyPolicy: string(identity.PrivateKeyPolicy),
	}
}

// serverMetadata returns the server metadata of this app agent.
func (c *ConnectionsHandler) serverMetadata() apievents.ServerMetadata {
	return apievents.ServerMetadata{
		ServerVersion:   teleport.Version,
		ServerID:        c.cfg.HostID,
		ServerNamespace: apidefaults.Namespace,
	}
}

// denyHints converts a decision's hints to the audit event type.
func denyHints(hints []appresource.Hint) []*apievents.AppSessionDenyHint {
	eventHints := make([]*apievents.AppSessionDenyHint, 0, len(hints))
	for _, hint := range hints {
		eventHints = append(eventHints, &apievents.AppSessionDenyHint{Code: hint.Code, Reason: hint.Reason})
	}
	return eventHints
}

// isCORSPreflight reports whether r is a CORS preflight request, an
// OPTIONS request carrying both the Origin and
// Access-Control-Request-Method headers.
func isCORSPreflight(r *http.Request) bool {
	return r.Method == http.MethodOptions &&
		r.Header.Get("Origin") != "" &&
		r.Header.Get("Access-Control-Request-Method") != ""
}
