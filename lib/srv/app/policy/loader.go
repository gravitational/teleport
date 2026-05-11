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
	"strings"

	"github.com/gravitational/trace"
)

// Spec is the raw, pre-compile shape of a policy, as parsed from a YAML
// config or constructed by callers. Strings are unparsed.
type Spec struct {
	Name  string
	Allow []RuleSpec
	Deny  []RuleSpec
}

// RuleSpec is the raw, pre-compile shape of a rule.
type RuleSpec struct {
	Paths      []string
	Methods    []string
	Where      string
	ReasonCode string
	Reason     string
}

// Ref is one entry in an app's policies list: either a name reference or
// an inline policy spec.
type Ref struct {
	Name   string
	Inline *Spec
}

// Compile produces a Policy from a Spec, compiling path patterns and the
// where: expression. Invariants are enforced via ValidatePolicy.
func Compile(s Spec) (Policy, error) {
	p := Policy{Name: s.Name}
	for i, r := range s.Allow {
		compiled, err := compileRule(r)
		if err != nil {
			return Policy{}, trace.Wrap(err, "policy %q allow rule %d", s.Name, i)
		}
		p.Allow = append(p.Allow, compiled)
	}
	for i, r := range s.Deny {
		compiled, err := compileRule(r)
		if err != nil {
			return Policy{}, trace.Wrap(err, "policy %q deny rule %d", s.Name, i)
		}
		p.Deny = append(p.Deny, compiled)
	}
	if err := ValidatePolicy(p); err != nil {
		return Policy{}, trace.Wrap(err)
	}
	FillDefaults(&p)
	return p, nil
}

func compileRule(r RuleSpec) (Rule, error) {
	methods := make([]string, len(r.Methods))
	for i, m := range r.Methods {
		methods[i] = strings.ToUpper(m)
	}
	out := Rule{
		Methods:    methods,
		ReasonCode: r.ReasonCode,
		Reason:     r.Reason,
	}
	for _, pat := range r.Paths {
		m, err := CompilePath(pat)
		if err != nil {
			return Rule{}, trace.Wrap(err)
		}
		out.Paths = append(out.Paths, m)
	}
	if r.Where != "" {
		pred, err := CompilePredicate(r.Where)
		if err != nil {
			return Rule{}, trace.Wrap(err)
		}
		out.Where = pred
	}
	return out, nil
}

// CompileAll compiles a slice of Specs in order. Duplicate names are
// rejected via ValidatePolicies after compilation.
func CompileAll(specs []Spec) ([]Policy, error) {
	out := make([]Policy, 0, len(specs))
	for _, s := range specs {
		p, err := Compile(s)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		out = append(out, p)
	}
	if err := ValidatePolicies(out); err != nil {
		return nil, trace.Wrap(err)
	}
	return out, nil
}

// Resolve maps a list of policy refs against the named library and
// returns the compiled set for one app. Names not found in library are
// an error.
func Resolve(library []Policy, refs []Ref) ([]Policy, error) {
	byName := map[string]Policy{}
	for _, p := range library {
		byName[p.Name] = p
	}
	out := make([]Policy, 0, len(refs))
	for _, r := range refs {
		switch {
		case r.Name != "" && r.Inline != nil:
			return nil, trace.BadParameter("policy ref %q has both name and inline spec", r.Name)
		case r.Name != "":
			p, ok := byName[r.Name]
			if !ok {
				return nil, trace.NotFound("unknown policy %q", r.Name)
			}
			out = append(out, p)
		case r.Inline != nil:
			p, err := Compile(*r.Inline)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			out = append(out, p)
		default:
			return nil, trace.BadParameter("policy ref must set either name or inline")
		}
	}
	return out, nil
}
