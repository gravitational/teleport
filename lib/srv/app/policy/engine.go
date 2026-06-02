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

package policy

import "slices"

// Decision is the result of evaluating one request against an app's
// attached policies. The tctl evaluate output and the audit event both
// project from it via JSON.
type Decision struct {
	// Allow is true when the request is permitted.
	Allow bool

	// Method, Path, and App echo the request. Path is the wire-form path
	// the rules matched against.
	Method string
	Path   string
	App    string

	// PolicyRef identifies what drove the decision: the matched rule's
	// reference as the loader assembled it on an allow, the reason code on
	// a deny.
	PolicyRef string
	// AuditCode is the matched rule's audit_code and gates allow-event
	// emission. Empty on a deny or when the rule sets none.
	AuditCode string
	// Binding names the binding the matched rule came through. Empty for an
	// inline match or a deny.
	Binding string
	// PathCaptures holds the {name} captures from the matched path. Nil
	// when there are none; it projects to {}.
	PathCaptures map[string]string

	// ReasonCode and Reason describe a deny and are empty on an allow.
	// Reason is the 403 body text.
	ReasonCode string
	Reason     string

	// AttachedPolicies is the app's full attached set, identical on allow
	// and deny; first-match short-circuiting never truncates it.
	AttachedPolicies []string
}

// Rule is one resolved allow rule: match criteria plus the audit
// provenance the loader computed. The engine copies provenance into a
// matching Decision unchanged and never inspects where the rule
// originated. Predicate (where:) evaluation comes later.
type Rule struct {
	// Paths are the compiled patterns; any one matching satisfies the
	// rule. Empty matches any path.
	Paths []*Matcher
	// Methods are the upper-case methods the rule permits. Empty matches
	// any method.
	Methods []string

	// PolicyRef, AuditCode, and Binding are the loader-assembled provenance
	// copied into a matching Decision.
	PolicyRef string
	AuditCode string
	Binding   string
}

// Request carries the per-request inputs. Path is the wire-form path
// (r.URL.EscapedPath()) the caller already cleared with ValidateWireform;
// Method is upper-case.
type Request struct {
	App    string
	Method string
	Path   string
}

// Input bundles the resolved inputs to Evaluate.
type Input struct {
	// Request is the per-request input.
	Request Request
	// Rules are the allow rules in evaluation order. Evaluate walks them
	// and stops at the first match.
	Rules []Rule
	// AttachedPolicies is the attached set, and its emptiness is the "no
	// policies attached" signal: empty selects the permissive branch (or a
	// deny under RequirePolicy); non-empty makes an unmatched request
	// default-deny.
	AttachedPolicies []string
	// RequirePolicy denies, rather than permits, a request to an app with
	// no attached policy. Maps to the cluster app_access_requires_policy
	// setting.
	RequirePolicy bool
}

// Audit event types emitted per request, one per decision outcome.
const (
	EventAllow = "app.session.policy.allow"
	EventDeny  = "app.session.policy.deny"
)

// Teleport-generated reason codes set on deny decisions. Customer
// audit_code values may not begin with the teleport_ prefix.
const (
	// ReasonNoMatchingAllow is the deny code when policies are attached but
	// no allow rule matched.
	ReasonNoMatchingAllow = "teleport_no_matching_allow"
	// ReasonNoPolicyAttached is the deny code when RequirePolicy is set and
	// the app has none attached.
	ReasonNoPolicyAttached = "teleport_no_policy_attached"

	// DefaultDenyReason is the reason text on every deny; allow-only rules
	// have no custom message.
	DefaultDenyReason = "Access denied by policy."
)

// Evaluate runs the policy decision model and returns a Decision. It is
// pure: no I/O, no clock, no audit emission, and it never errors. The path
// must already have cleared ValidateWireform.
//
// With no policies attached it permits, unless RequirePolicy denies with
// teleport_no_policy_attached. With policies attached it allows on the
// first rule whose method and path match, else denies with
// teleport_no_matching_allow.
func Evaluate(in Input) Decision {
	dec := Decision{
		Method:           in.Request.Method,
		Path:             in.Request.Path,
		App:              in.Request.App,
		AttachedPolicies: in.AttachedPolicies,
	}

	if len(in.AttachedPolicies) == 0 {
		if in.RequirePolicy {
			return deny(dec, ReasonNoPolicyAttached)
		}
		dec.Allow = true
		return dec
	}

	for _, rule := range in.Rules {
		if !matchMethod(rule.Methods, in.Request.Method) {
			continue
		}
		captures, ok := matchPath(rule.Paths, in.Request.Path)
		if !ok {
			continue
		}
		dec.Allow = true
		dec.PolicyRef = rule.PolicyRef
		dec.AuditCode = rule.AuditCode
		dec.Binding = rule.Binding
		dec.PathCaptures = captures
		return dec
	}

	return deny(dec, ReasonNoMatchingAllow)
}

// deny marks dec denied with code. PolicyRef carries the code to match the
// audit-event projection.
func deny(dec Decision, code string) Decision {
	dec.Allow = false
	dec.ReasonCode = code
	dec.Reason = DefaultDenyReason
	dec.PolicyRef = code
	return dec
}

// matchMethod reports whether method satisfies the filter. Empty matches
// any; otherwise an exact, case-sensitive compare.
func matchMethod(methods []string, method string) bool {
	return len(methods) == 0 || slices.Contains(methods, method)
}

// matchPath returns the captures from the first pattern that matches path.
// Empty matches any path with no captures; the bool reports whether any
// matched.
func matchPath(paths []*Matcher, path string) (map[string]string, bool) {
	if len(paths) == 0 {
		return nil, true
	}
	for _, m := range paths {
		if captures, ok := m.Match(path); ok {
			return captures, true
		}
	}
	return nil, false
}

// DecisionJSON is the deterministic subset of a Decision that both tctl
// evaluate and the audit event serialize, so the two cannot drift. The
// audit event adds metadata (time, resource metadata) on top.
type DecisionJSON struct {
	Event            string            `json:"event"`
	Method           string            `json:"method"`
	Path             string            `json:"path"`
	App              string            `json:"app"`
	Binding          string            `json:"binding,omitempty"`
	AuditCode        string            `json:"audit_code,omitempty"`
	ReasonCode       string            `json:"reason_code,omitempty"`
	Reason           string            `json:"reason,omitempty"`
	PolicyRef        string            `json:"policy_ref"`
	PathCaptures     map[string]string `json:"path_captures"`
	AttachedPolicies []string          `json:"attached_policies"`
}

// JSON projects the Decision to DecisionJSON, normalizing PathCaptures to
// {} and AttachedPolicies to [] so neither serializes as null.
func (d Decision) JSON() DecisionJSON {
	event := EventDeny
	if d.Allow {
		event = EventAllow
	}
	captures := d.PathCaptures
	if captures == nil {
		captures = map[string]string{}
	}
	attached := d.AttachedPolicies
	if attached == nil {
		attached = []string{}
	}
	return DecisionJSON{
		Event:            event,
		Method:           d.Method,
		Path:             d.Path,
		App:              d.App,
		Binding:          d.Binding,
		AuditCode:        d.AuditCode,
		ReasonCode:       d.ReasonCode,
		Reason:           d.Reason,
		PolicyRef:        d.PolicyRef,
		PathCaptures:     captures,
		AttachedPolicies: attached,
	}
}
