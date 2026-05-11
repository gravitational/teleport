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

// Package policy implements policy-based per-request authorization for
// Teleport App Access. See RFD on policy-based App Access for the design.
package policy

import (
	"strings"

	"github.com/gravitational/trace"
)

// Reserved Teleport-emitted reason codes and policy-name prefix.
const (
	ReservedPrefix = "teleport_"

	ReasonExplicitDeny          = "teleport_explicit_deny"
	ReasonNoMatchingAllow       = "teleport_no_matching_allow"
	ReasonDoubleEncodedSlash    = "teleport_double_encoded_slash"
	ReasonEncodedSlashInSegment = "teleport_encoded_slash_in_segment"
	ReasonPathDecodeFailed      = "teleport_path_decode_failed"
	ReasonPredicateError        = "teleport_predicate_error"
)

const methodAny = "*"

// Policy is a named bundle of rules. A policy may hold any combination
// of allow and deny rules; at least one of the two must be set.
type Policy struct {
	Name  string
	Allow []Rule
	Deny  []Rule
}

// Rule is a single match clause inside a policy.
type Rule struct {
	// Paths are the compiled path patterns. Required on allow rules;
	// optional on deny rules.
	Paths []*PathMatcher

	// Methods is the list of HTTP methods, upper-case. Empty or
	// containing "*" means any method.
	Methods []string

	// Where is the optional compiled predicate. May be nil.
	Where Predicate

	// ReasonCode is the machine-readable identifier surfaced in
	// audit and 403 responses. Defaults to the policy name on allow
	// rules and to ReasonExplicitDeny on deny rules.
	ReasonCode string

	// Reason is the human-readable string returned to the user.
	// Defaults to ReasonCode.
	Reason string
}

// Predicate is a compiled where: expression. Returns the boolean result,
// or an error to be surfaced as teleport_predicate_error.
type Predicate interface {
	Evaluate(env Env) (bool, error)
}

// MatchesMethod reports whether r matches the given method.
func (r *Rule) MatchesMethod(method string) bool {
	if len(r.Methods) == 0 {
		return true
	}
	method = strings.ToUpper(method)
	for _, m := range r.Methods {
		if m == methodAny || m == method {
			return true
		}
	}
	return false
}

// MatchesPath returns the captured variables if any path in r matches the
// given normalized path. The second return is false if no path matches.
// A rule with no paths set matches any path with no captures.
func (r *Rule) MatchesPath(path string) (map[string]string, bool) {
	if len(r.Paths) == 0 {
		return nil, true
	}
	for _, p := range r.Paths {
		if caps, ok := p.Match(path); ok {
			return caps, true
		}
	}
	return nil, false
}

// ValidatePolicy enforces the name, reserved-prefix, and reason-code
// uniqueness invariants. Run before FillDefaults: uniqueness applies
// only to operator-set codes because multiple deny rules legitimately
// share the synthetic default.
func ValidatePolicy(p Policy) error {
	if p.Name == "" {
		return trace.BadParameter("policy name is required")
	}
	if strings.HasPrefix(p.Name, ReservedPrefix) {
		return trace.BadParameter("policy %q: name must not start with %q", p.Name, ReservedPrefix)
	}
	if len(p.Allow) == 0 && len(p.Deny) == 0 {
		return trace.BadParameter("policy %q: must hold at least one allow or deny rule", p.Name)
	}
	seen := map[string]bool{}
	for i, r := range p.Allow {
		if err := validateAllowRule(r); err != nil {
			return trace.Wrap(err, "policy %q allow rule %d", p.Name, i)
		}
		if err := checkOperatorReasonCode(r.ReasonCode, p.Name, seen, "allow", i); err != nil {
			return err
		}
	}
	for i, r := range p.Deny {
		if err := validateDenyRule(r); err != nil {
			return trace.Wrap(err, "policy %q deny rule %d", p.Name, i)
		}
		if err := checkOperatorReasonCode(r.ReasonCode, p.Name, seen, "deny", i); err != nil {
			return err
		}
	}
	return nil
}

func checkOperatorReasonCode(code, policyName string, seen map[string]bool, kind string, idx int) error {
	if code == "" {
		return nil
	}
	if strings.HasPrefix(code, ReservedPrefix) {
		return trace.BadParameter("policy %q %s rule %d: reason_code %q must not start with %q", policyName, kind, idx, code, ReservedPrefix)
	}
	if seen[code] {
		return trace.BadParameter("policy %q %s rule %d: duplicate reason_code %q", policyName, kind, idx, code)
	}
	seen[code] = true
	return nil
}

func validateAllowRule(r Rule) error {
	if len(r.Paths) == 0 {
		// A where-only allow rule matches every request and decides on
		// the predicate alone. A typo there fail-opens the app.
		return trace.BadParameter("allow rule must set paths")
	}
	return nil
}

func validateDenyRule(r Rule) error {
	if len(r.Paths) == 0 && len(r.Methods) == 0 && r.Where == nil {
		return trace.BadParameter("deny rule must set at least one of paths, methods, or where")
	}
	return nil
}

// FillDefaults populates ReasonCode and Reason on each rule. Allow
// rules default to the policy name; deny rules default to the
// synthetic ReasonExplicitDeny code. Run after ValidatePolicy.
func FillDefaults(p *Policy) {
	for i := range p.Allow {
		fillRuleDefaults(&p.Allow[i], p.Name)
	}
	for i := range p.Deny {
		fillRuleDefaults(&p.Deny[i], ReasonExplicitDeny)
	}
}

func fillRuleDefaults(r *Rule, defaultCode string) {
	if r.ReasonCode == "" {
		r.ReasonCode = defaultCode
	}
	if r.Reason == "" {
		r.Reason = r.ReasonCode
	}
}

// ValidatePolicies rejects duplicate names. Per-policy invariants are
// checked in Compile; ValidatePolicies expects already-compiled inputs
// (post-FillDefaults), so it does not re-run ValidatePolicy.
func ValidatePolicies(policies []Policy) error {
	seen := map[string]bool{}
	for _, p := range policies {
		if seen[p.Name] {
			return trace.BadParameter("duplicate policy name %q", p.Name)
		}
		seen[p.Name] = true
	}
	return nil
}
