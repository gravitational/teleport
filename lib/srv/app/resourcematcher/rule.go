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

package resourcematcher

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/gravitational/trace"
)

// Rule is one app_resources entry. It has two authoring surfaces, and a rule
// uses one of them, never both:
//
//   - The declarative form: Paths, Methods, and Where, which read compactly
//     for the common HTTP case and desugar to one predicate.
//   - The predicate form: a single Pred holding one true/false predicate. This
//     is the foundation; the declarative form compiles to it.
//
// Both surfaces compile to one internal predicate and evaluate through one
// engine, so the declarative form is sugar over the predicate form rather than
// a parallel mechanism.
type Rule struct {
	// Paths are declarative path patterns, OR-ed. A request matches the rule's
	// path territory if any pattern matches. Omit to match any path.
	Paths []string `yaml:"paths,omitempty"`
	// Methods are HTTP methods, OR-ed and upper-cased. Omit to match any
	// method.
	Methods []string `yaml:"methods,omitempty"`
	// Where is a condition over the caller identity, the request, and this
	// rule's captures. It does not match paths.
	Where string `yaml:"where,omitempty"`
	// Pred is the bare predicate form. It is mutually exclusive with the
	// declarative fields.
	Pred string `yaml:"pred,omitempty"`
	// URLDecoding controls path normalization before matching. It defaults to
	// the strict, reject-everything zero value.
	URLDecoding DecodeConfig `yaml:"url_decoding,omitempty"`
}

// Decision is the outcome of evaluating a rule or rule set against a request.
// It is a struct rather than a bool so a test can assert on the captures a
// match bound, not just allow or deny.
type Decision struct {
	// Allowed reports whether any rule matched.
	Allowed bool
	// Vars holds the segments the matching rule's captures bound. It is nil on
	// a deny.
	Vars map[string]string
}

// CompiledRule is a parsed, ready-to-evaluate rule. The compiled predicate
// holds the matcher tree, the method set, and the identity condition as one
// expression, so the declarative and predicate forms cannot diverge once
// compiled.
type CompiledRule struct {
	pred   predicate
	decode DecodeConfig
}

// Compile validates a rule and compiles its authoring surface to one internal
// predicate. Setting both Pred and any declarative field, or setting neither
// surface, is a load error: a rule must constrain something through exactly
// one surface.
func (r Rule) Compile() (*CompiledRule, error) {
	hasDeclarative := len(r.Paths) > 0 || len(r.Methods) > 0 || r.Where != ""
	if r.Pred != "" && hasDeclarative {
		return nil, trace.BadParameter("a rule sets either pred or the declarative fields, not both")
	}
	if r.Pred == "" && !hasDeclarative {
		return nil, trace.BadParameter("a rule must set pred or at least one of paths, methods, where")
	}

	expr := r.Pred
	if hasDeclarative {
		var err error
		expr, err = r.desugar()
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	pred, err := parsePredicate(expr)
	if err != nil {
		return nil, trace.Wrap(err, "compiling rule predicate %q", expr)
	}
	if err := validateCaptures(expr); err != nil {
		return nil, trace.Wrap(err)
	}
	return &CompiledRule{pred: pred, decode: r.URLDecoding}, nil
}

// desugar lowers the declarative fields to an equivalent predicate string. It
// compiles each path pattern to the canonical matcher tree and emits the
// matcher constructors as source, so the desugared declarative form parses to
// exactly the tree a hand-written predicate would. The clauses are ANDed:
// path territory, then method, then the identity condition.
func (r Rule) desugar() (string, error) {
	var clauses []string

	if len(r.Paths) > 0 {
		roots := make([]string, 0, len(r.Paths))
		for _, p := range r.Paths {
			node, err := Compile(p)
			if err != nil {
				return "", trace.Wrap(err)
			}
			roots = append(roots, nodeToSource(node))
		}
		clauses = append(clauses, fmt.Sprintf("path.match(%s)", strings.Join(roots, ", ")))
	}

	if len(r.Methods) > 0 {
		quoted := make([]string, 0, len(r.Methods))
		for _, m := range r.Methods {
			quoted = append(quoted, strconv.Quote(strings.ToUpper(m)))
		}
		clauses = append(clauses, fmt.Sprintf("contains(set(%s), request.method)", strings.Join(quoted, ", ")))
	}

	if r.Where != "" {
		clauses = append(clauses, "("+r.Where+")")
	}

	return strings.Join(clauses, " && "), nil
}

// nodeToSource renders a matcher tree as the constructor source the predicate
// parser accepts. A compiled tree carries single-segment literals, so the
// rendered literal("seg", ...) round-trips back to the same tree.
func nodeToSource(n *Node) string {
	childArgs := func(prefix string) string {
		parts := make([]string, 0, len(n.children))
		for _, c := range n.children {
			parts = append(parts, nodeToSource(c))
		}
		if len(parts) == 0 {
			return prefix + ")"
		}
		return prefix + ", " + strings.Join(parts, ", ") + ")"
	}
	switch n.kind {
	case kindLiteral:
		return childArgs("literal(" + strconv.Quote(n.text))
	case kindCapture:
		return childArgs("capture(" + strconv.Quote(n.text))
	case kindGlob:
		return childArgs("glob(")
	case kindGreedy:
		return "greedy()"
	default:
		return ""
	}
}

// Evaluate runs the rule against a request and identity. A fresh environment,
// and therefore a fresh capture map, is built per call.
func (c *CompiledRule) Evaluate(request Request, identity Identity) (Decision, error) {
	e := env{
		request: request,
		user:    identity,
		decode:  c.decode,
		vars:    map[string]string{},
		state:   &evalState{},
	}
	allowed, err := c.pred.Evaluate(e)
	if err != nil {
		return Decision{}, trace.Wrap(err)
	}
	// A read of a capture the matcher did not bind on this request forces the
	// rule to no-match, so an unbound capture can only deny, never widen, no
	// matter which operator read it.
	if !allowed || e.state.unboundRead {
		return Decision{}, nil
	}
	return Decision{Allowed: true, Vars: e.vars}, nil
}

// RuleSet is the additive union of rules, the effective access an app grants a
// caller across every matching role. A request is allowed if any rule matches.
type RuleSet []*CompiledRule

// CompileRules compiles a slice of rules into a RuleSet.
func CompileRules(rules []Rule) (RuleSet, error) {
	set := make(RuleSet, 0, len(rules))
	for i, r := range rules {
		c, err := r.Compile()
		if err != nil {
			return nil, trace.Wrap(err, "rule %d", i)
		}
		set = append(set, c)
	}
	return set, nil
}

// Evaluate returns the first matching rule's decision, or a deny if none
// match. Because rules are allow-only, the order of evaluation does not affect
// whether the request is allowed, only which captures a caller sees on the
// returned decision.
func (s RuleSet) Evaluate(request Request, identity Identity) (Decision, error) {
	for _, rule := range s {
		decision, err := rule.Evaluate(request, identity)
		if err != nil {
			return Decision{}, trace.Wrap(err)
		}
		if decision.Allowed {
			return decision, nil
		}
	}
	return Decision{}, nil
}
