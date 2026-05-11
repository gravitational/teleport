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

import (
	"fmt"
)

// Decision is the result of evaluating a request against a policy set.
type Decision struct {
	Allow      bool
	ReasonCode string
	Reason     string
	PolicyRef  string
	BoundVars  map[string]string

	EvalSummary EvalSummary
}

// EvalSummary captures which policies were considered, for audit.
type EvalSummary struct {
	PoliciesEvaluated []string
}

// Request is the per-request input to Evaluate.
type Request struct {
	Method        string
	Path          string
	NormalizeCode string // teleport_* code if normalization failed
	NormalizeErr  string // message if normalization failed
	UserName      string
	UserRoles     []string
}

// Evaluate applies the deny-first / allow-second decision model. If
// policies is empty the request is allowed (backwards-compatibility
// contract). A normalization failure short-circuits to a synthetic deny.
func Evaluate(policies []Policy, req Request) Decision {
	summary := EvalSummary{}
	for _, p := range policies {
		summary.PoliciesEvaluated = append(summary.PoliciesEvaluated, p.Name)
	}

	if req.NormalizeCode != "" {
		return Decision{
			ReasonCode:  req.NormalizeCode,
			Reason:      req.NormalizeErr,
			EvalSummary: summary,
		}
	}

	if len(policies) == 0 {
		return Decision{Allow: true, EvalSummary: summary}
	}

	for _, p := range policies {
		for _, rule := range p.Deny {
			match, caps, err := matchRule(rule, req)
			if err != nil {
				return Decision{
					ReasonCode:  ReasonPredicateError,
					Reason:      err.Error(),
					PolicyRef:   policyRef(p.Name, rule.ReasonCode),
					BoundVars:   caps,
					EvalSummary: summary,
				}
			}
			if !match {
				continue
			}
			return Decision{
				ReasonCode:  rule.ReasonCode,
				Reason:      rule.Reason,
				PolicyRef:   policyRef(p.Name, rule.ReasonCode),
				BoundVars:   caps,
				EvalSummary: summary,
			}
		}
	}

	hasAllowRules := false
	for _, p := range policies {
		if len(p.Allow) > 0 {
			hasAllowRules = true
			break
		}
	}
	if !hasAllowRules {
		return Decision{Allow: true, EvalSummary: summary}
	}

	for _, p := range policies {
		for _, rule := range p.Allow {
			// Predicate errors on allow rules are treated as no-match.
			match, caps, err := matchRule(rule, req)
			if err != nil || !match {
				continue
			}
			return Decision{
				Allow:       true,
				ReasonCode:  rule.ReasonCode,
				Reason:      rule.Reason,
				PolicyRef:   policyRef(p.Name, rule.ReasonCode),
				BoundVars:   caps,
				EvalSummary: summary,
			}
		}
	}

	return Decision{
		ReasonCode:  ReasonNoMatchingAllow,
		Reason:      "no allow rule matched the request",
		EvalSummary: summary,
	}
}

// matchRule reports whether rule matches the request and returns any
// captured path variables. A non-nil error means the where: clause
// evaluation failed.
func matchRule(rule Rule, req Request) (bool, map[string]string, error) {
	if !rule.MatchesMethod(req.Method) {
		return false, nil, nil
	}
	caps, ok := rule.MatchesPath(req.Path)
	if !ok {
		return false, nil, nil
	}
	if rule.Where == nil {
		return true, caps, nil
	}
	env := Env{
		UserName:      req.UserName,
		UserRoles:     req.UserRoles,
		Path:          caps,
		RequestMethod: req.Method,
		RequestPath:   req.Path,
	}
	v, err := rule.Where.Evaluate(env)
	if err != nil {
		return false, caps, err
	}
	return v, caps, nil
}

func policyRef(policyName, reasonCode string) string {
	if reasonCode == "" {
		return policyName
	}
	return fmt.Sprintf("%s/%s", policyName, reasonCode)
}
