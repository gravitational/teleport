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

// DenyKind is the structured reason a request was denied. Its values are
// the deny_kind emitted on the app.session.request.denied audit event, so
// the type reads as a category in Go while it serializes straight to the
// audit string in JSON.
type DenyKind string

const (
	// DenyNotAllowed is the kind for a well-formed request that no allow
	// rule matched. A fired hint explains the near-miss.
	DenyNotAllowed DenyKind = "teleport_request_not_allowed"
	// DenyInvalidRequest is the kind for a request denied before any rule
	// evaluated, because its path was rejected as malformed or unsafe,
	// such as a "." or ".." segment, consecutive slashes, an illegal
	// byte, or a percent-escape other than the encoded separator %2F.
	DenyInvalidRequest DenyKind = "teleport_invalid_request"
)

// Decision is the outcome of evaluating a rule or role set against a
// request. Allowed is the verdict. Exactly one of Allow or Deny carries
// the matching detail, so allow-only and deny-only fields cannot be read
// on the wrong outcome. EvaluatedRoles rides both, since the audit event
// emits it either way.
type Decision struct {
	// Allowed reports whether any rule matched.
	Allowed bool
	// Allow carries the captures and codes of the matching rule. It is
	// non-nil if and only if Allowed.
	Allow *AllowDetails
	// Deny carries the deny kind and any fired hints. It is non-nil if
	// and only if not Allowed.
	Deny *DenyDetails
	// EvaluatedRoles lists the roles that carried app_resources for the
	// app, in the order they were evaluated. An empty list on a deny
	// marks a misconfigured default-deny, where no role granted any
	// app_resources, as opposed to a request that a granting role did not
	// match.
	EvaluatedRoles []string
}

// AllowDetails carries the detail of an allow.
type AllowDetails struct {
	// Vars holds the segments the matching rule's captures bound.
	Vars map[string]string
	// Code is the matching rule's allow_code.
	Code string
	// Reason is the matching rule's allow_reason.
	Reason string
}

// DenyDetails carries the detail of a deny.
type DenyDetails struct {
	// Kind is the structured reason for the deny.
	Kind DenyKind
	// Hints lists every hint that fired, in rule order.
	Hints []Hint
}

// decisionJSON is the flat wire form of a Decision: the Allow and Deny
// details become top-level keys, matching the payload the audit event
// and the tctl evaluate output carry.
type decisionJSON struct {
	Allowed        bool              `json:"allowed"`
	EvaluatedRoles []string          `json:"evaluated_roles,omitempty"`
	Vars           map[string]string `json:"vars,omitempty"`
	AllowCode      string            `json:"allow_code,omitempty"`
	AllowReason    string            `json:"allow_reason,omitempty"`
	DenyKind       DenyKind          `json:"deny_kind,omitempty"`
	Hints          []hintJSON        `json:"hints,omitempty"`
}

// hintJSON is the wire form of one fired deny hint.
type hintJSON struct {
	Code   string `json:"code"`
	Reason string `json:"reason,omitempty"`
}

// MarshalJSON encodes the decision in its flat wire form, with unset
// fields omitted. It reads the detail matching the verdict, so a
// malformed decision that sets the wrong side cannot emit allow keys on
// a deny or deny keys on an allow.
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
		for _, h := range d.Deny.Hints {
			out.Hints = append(out.Hints, hintJSON(h))
		}
	}
	return json.Marshal(out)
}
