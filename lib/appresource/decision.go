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

package appresource

import "encoding/json"

// Hint explains a near-miss on one [Rule], where its path and method matched but its
// Where did not. It contains the rule's DenyCodeHint and DenyReasonHint.
type Hint struct {
	Code   string `json:"code"`
	Reason string `json:"reason,omitempty"`
}

// DenyKind is the category of a denial, emitted as deny_kind on the
// app.session.request.denied audit event.
type DenyKind string

const (
	// DenyNotAllowed is the kind for a well-formed request that no allow
	// rule matched.
	DenyNotAllowed DenyKind = "teleport_request_not_allowed"
	// DenyRoleVersionUnsupported is the kind for a request denied because a
	// role or a rule is a version this agent cannot evaluate, as in a
	// mixed-version cluster.
	DenyRoleVersionUnsupported DenyKind = "teleport_role_version_unsupported"
	// DenyInvalidRequest is the denial category for a malformed request
	// path, e.g. containing a ".." segment. No rule is evaluated.
	DenyInvalidRequest DenyKind = "teleport_invalid_request"
)

// Decision is the aggregated result of evaluating one request against the
// app_resources rules on the caller's roles.
type Decision struct {
	// Allowed is true if any rule matched.
	Allowed bool
	// Allow contains details iff Allowed is true.
	Allow *AllowDetails
	// Deny contains details iff Allowed is false.
	Deny *DenyDetails
	// EvaluatedRoles lists the roles evaluated, in evaluation order.
	EvaluatedRoles []string
}

// AllowDetails is an allow decision record derived from the matching rule.
type AllowDetails struct {
	// Vars contains the path segments the matching rule captured.
	Vars map[string]string
	// Code is the matching rule's allow_code.
	Code string
	// Reason is the matching rule's allow_reason.
	Reason string
}

// DenyDetails is a deny decision record.
type DenyDetails struct {
	// Kind is the structured reason for the deny.
	Kind DenyKind
	// Hints lists every hint that fired, in rule order.
	Hints []Hint
}

// decisionJSON is the flat wire form of a Decision.
type decisionJSON struct {
	Allowed        bool              `json:"allowed"`
	EvaluatedRoles []string          `json:"evaluated_roles,omitempty"`
	Vars           map[string]string `json:"vars,omitempty"`
	AllowCode      string            `json:"allow_code,omitempty"`
	AllowReason    string            `json:"allow_reason,omitempty"`
	DenyKind       DenyKind          `json:"deny_kind,omitempty"`
	Hints          []Hint            `json:"hints,omitempty"`
}

// MarshalJSON encodes the decision in its flat wire form, with unset fields
// omitted. Detail on the side that does not match Allowed is dropped.
func (d Decision) MarshalJSON() ([]byte, error) {
	out := decisionJSON{
		Allowed:        d.Allowed,
		EvaluatedRoles: d.EvaluatedRoles,
	}
	if d.Allowed && d.Allow != nil {
		out.Vars = d.Allow.Vars
		out.AllowCode = d.Allow.Code
		out.AllowReason = d.Allow.Reason
	}
	if !d.Allowed && d.Deny != nil {
		out.DenyKind = d.Deny.Kind
		out.Hints = d.Deny.Hints
	}
	return json.Marshal(out)
}
