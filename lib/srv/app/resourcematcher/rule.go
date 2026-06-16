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
// a parallel mechanism. AllowCode and DenyHints are metadata that attach to
// either surface.
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
	// declarative path, method, and where fields.
	Pred string `yaml:"pred,omitempty"`
	// AllowCode is the structured code emitted on the allow audit event when
	// this rule matches. Without it, an allow emits no event.
	AllowCode string `yaml:"allow_code,omitempty"`
	// AllowReason is the human-readable explanation emitted alongside
	// AllowCode on an allow.
	AllowReason string `yaml:"allow_reason,omitempty"`
	// DenyHints explain a deny. On a deny, every hint whose On predicate
	// matches contributes its DenyCode and DenyReason to the decision.
	DenyHints []DenyHint `yaml:"deny_hint,omitempty"`
	// URLDecoding controls path normalization before matching. It defaults to
	// the strict, reject-everything zero value.
	URLDecoding DecodeConfig `yaml:"url_decoding,omitempty"`
}

// DenyHint is one near-miss explanation. On a deny, the hint fires when its On
// predicate is true. On is the territory the hint speaks for: when omitted in
// the declarative form it defaults to "the rule's path and method matched", so
// the hint fires exactly when the path and method matched but where failed. In
// the predicate form there is no separate path and method to default from, so
// On is required.
type DenyHint struct {
	// On is the predicate that decides whether the hint fires on a deny. Omit
	// in the declarative form to default to the rule's path and method
	// clauses.
	On string `yaml:"on,omitempty"`
	// DenyCode is the structured code emitted on the deny audit event when the
	// hint fires.
	DenyCode string `yaml:"deny_code"`
	// DenyReason is the human-readable explanation emitted alongside DenyCode.
	DenyReason string `yaml:"deny_reason,omitempty"`
}

// Decision is the outcome of evaluating a rule or rule set against a request.
// It is a struct rather than a bool so a test can assert on the captures a
// match bound and the codes a match or near-miss emitted, not just allow or
// deny.
type Decision struct {
	// Allowed reports whether any rule matched.
	Allowed bool
	// Vars holds the segments the matching rule's captures bound. It is nil on
	// a deny.
	Vars map[string]string
	// AllowCode is the matching rule's allow_code, set only on an allow.
	AllowCode string
	// AllowReason is the matching rule's allow_reason, set only on an allow.
	AllowReason string
	// DenyHints lists every hint that fired on a deny, in rule order. It is
	// nil on an allow.
	DenyHints []FiredHint
}

// FiredHint is a deny hint that matched on a deny.
type FiredHint struct {
	DenyCode   string
	DenyReason string
}

// CompiledRule is a parsed, ready-to-evaluate rule.
type CompiledRule struct {
	pred        predicate
	decode      DecodeConfig
	allowCode   string
	allowReason string
	denyHints   []compiledHint
}

// compiledHint is a parsed deny hint.
type compiledHint struct {
	on         predicate
	denyCode   string
	denyReason string
}

// Compile validates a rule and compiles its authoring surface to one internal
// predicate. Setting both Pred and any declarative path, method, or where
// field, or setting none of them, is a load error: a rule must constrain
// something through exactly one surface.
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

	if r.AllowCode != "" {
		if err := validateCode(r.AllowCode); err != nil {
			return nil, trace.Wrap(err, "invalid allow_code")
		}
	}

	hints, err := r.compileDenyHints()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &CompiledRule{
		pred:        pred,
		decode:      r.URLDecoding,
		allowCode:   r.AllowCode,
		allowReason: r.AllowReason,
		denyHints:   hints,
	}, nil
}

// compileDenyHints parses each deny hint. A hint with no On defaults to the
// rule's path and method clauses, which is only possible in the declarative
// form. In the predicate form an omitted On is a load error, since there is no
// separate path and method clause to default from.
func (r Rule) compileDenyHints() ([]compiledHint, error) {
	if len(r.DenyHints) == 0 {
		return nil, nil
	}

	defaultOn, err := r.defaultHintOn()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	hints := make([]compiledHint, 0, len(r.DenyHints))
	for i, h := range r.DenyHints {
		if h.DenyCode == "" {
			return nil, trace.BadParameter("deny_hint %d must set deny_code", i)
		}
		if err := validateCode(h.DenyCode); err != nil {
			return nil, trace.Wrap(err, "deny_hint %d has an invalid deny_code", i)
		}

		on := h.On
		if on == "" {
			if defaultOn == "" {
				return nil, trace.BadParameter(
					"deny_hint %d must set on: a predicate-form rule has no path or method to default the hint territory from", i)
			}
			on = defaultOn
		}

		onPred, err := parsePredicate(on)
		if err != nil {
			return nil, trace.Wrap(err, "compiling deny_hint %d on %q", i, on)
		}
		if err := validateCaptures(on); err != nil {
			return nil, trace.Wrap(err, "deny_hint %d", i)
		}
		hints = append(hints, compiledHint{on: onPred, denyCode: h.DenyCode, denyReason: h.DenyReason})
	}
	return hints, nil
}

// defaultHintOn returns the predicate a hint with no On falls back to: the
// rule's path and method clauses ANDed. It is empty for a rule with neither, so
// the predicate form cannot default a hint.
func (r Rule) defaultHintOn() (string, error) {
	path, err := r.pathClause()
	if err != nil {
		return "", trace.Wrap(err)
	}
	return joinClauses(path, r.methodClause()), nil
}

// desugar lowers the declarative fields to an equivalent predicate string,
// ANDing the path, method, and where clauses. The path clause compiles each
// pattern to the canonical matcher tree and emits the matcher constructors as
// source, so the desugared declarative form parses to exactly the tree a
// hand-written predicate would.
func (r Rule) desugar() (string, error) {
	path, err := r.pathClause()
	if err != nil {
		return "", trace.Wrap(err)
	}
	var where string
	if r.Where != "" {
		// Wrap the where in parentheses only when it is ANDed with a path or
		// method clause, where the grouping matters. When the where is the
		// whole predicate, the parentheses add nothing.
		where = r.Where
		if path != "" || r.methodClause() != "" {
			where = "(" + where + ")"
		}
	}
	return joinClauses(path, r.methodClause(), where), nil
}

// pathClause renders the Paths as a single path.match call whose arguments are
// the per-pattern matcher trees, OR-ed. It is empty when no paths are set.
func (r Rule) pathClause() (string, error) {
	if len(r.Paths) == 0 {
		return "", nil
	}
	roots := make([]string, 0, len(r.Paths))
	for _, p := range r.Paths {
		node, err := Compile(p)
		if err != nil {
			return "", trace.Wrap(err)
		}
		roots = append(roots, nodeToSource(node))
	}
	return fmt.Sprintf("path.match(%s)", strings.Join(roots, ", ")), nil
}

// methodClause renders the Methods as a membership test against the request
// method. It is empty when no methods are set.
func (r Rule) methodClause() string {
	if len(r.Methods) == 0 {
		return ""
	}
	quoted := make([]string, 0, len(r.Methods))
	for _, m := range r.Methods {
		quoted = append(quoted, strconv.Quote(strings.ToUpper(m)))
	}
	return fmt.Sprintf("contains(set(%s), request.method)", strings.Join(quoted, ", "))
}

// joinClauses ANDs the non-empty clauses.
func joinClauses(clauses ...string) string {
	nonEmpty := make([]string, 0, len(clauses))
	for _, c := range clauses {
		if c != "" {
			nonEmpty = append(nonEmpty, c)
		}
	}
	return strings.Join(nonEmpty, " && ")
}

// nodeToSource renders a matcher tree as the constructor source the predicate
// parser accepts. A run of single-child literals contracts into one
// slash-joined literal, since the Literal constructor splits the text back on
// "/" and so parses to the same tree; this keeps the rendered source readable.
// A glob, capture, greedy, or a branch ends the run.
func nodeToSource(n *Node) string {
	switch n.kind {
	case kindLiteral:
		texts := []string{n.text}
		cur := n
		for len(cur.children) == 1 && cur.children[0].kind == kindLiteral {
			cur = cur.children[0]
			texts = append(texts, cur.text)
		}
		return constructorSource("literal", strconv.Quote(strings.Join(texts, "/")), cur.children)
	case kindCapture:
		return constructorSource("capture", strconv.Quote(n.text), n.children)
	case kindGlob:
		return constructorSource("glob", "", n.children)
	case kindGreedy:
		return "greedy()"
	default:
		return ""
	}
}

// constructorSource renders one matcher constructor call. lead is the quoted
// leading argument for literal and capture, or empty for glob. The children
// follow, so an argument-less constructor with children does not emit a stray
// leading comma.
func constructorSource(name, lead string, children []*Node) string {
	var args []string
	if lead != "" {
		args = append(args, lead)
	}
	for _, c := range children {
		args = append(args, nodeToSource(c))
	}
	return name + "(" + strings.Join(args, ", ") + ")"
}

// Evaluate runs the rule against a request and identity. A fresh environment,
// and therefore a fresh capture map, is built per call. On an allow it carries
// the rule's allow_code; on a deny it carries every deny hint whose territory
// matched.
func (c *CompiledRule) Evaluate(request Request, identity Identity) (Decision, error) {
	e := newEnv(request, identity, c.decode)
	allowed, err := c.pred.Evaluate(e)
	if err != nil {
		return Decision{}, trace.Wrap(err)
	}
	// A read of a capture the matcher did not bind on this request forces the
	// rule to no-match, so an unbound capture can only deny, never widen, no
	// matter which operator read it.
	if allowed && !e.state.unboundRead {
		return Decision{Allowed: true, Vars: e.vars, AllowCode: c.allowCode, AllowReason: c.allowReason}, nil
	}

	var fired []FiredHint
	for _, h := range c.denyHints {
		ok, err := evalPredicate(h.on, request, identity, c.decode)
		if err != nil {
			return Decision{}, trace.Wrap(err)
		}
		if ok {
			fired = append(fired, FiredHint{DenyCode: h.denyCode, DenyReason: h.denyReason})
		}
	}
	return Decision{DenyHints: fired}, nil
}

// newEnv builds a fresh evaluation environment for one request.
func newEnv(request Request, identity Identity, decode DecodeConfig) env {
	return env{
		request: request,
		user:    identity,
		decode:  decode,
		vars:    map[string]string{},
		state:   &evalState{},
	}
}

// evalPredicate evaluates a predicate against a fresh environment and applies
// the unbound-capture guard, returning whether it matched.
func evalPredicate(p predicate, request Request, identity Identity, decode DecodeConfig) (bool, error) {
	e := newEnv(request, identity, decode)
	ok, err := p.Evaluate(e)
	if err != nil {
		return false, trace.Wrap(err)
	}
	return ok && !e.state.unboundRead, nil
}

// validateCode checks an allow or deny code: 1 to 64 of [a-z0-9_], and not the
// reserved teleport_ prefix.
func validateCode(code string) error {
	if len(code) < 1 || len(code) > 64 {
		return trace.BadParameter("code %q must be 1 to 64 characters", code)
	}
	if strings.HasPrefix(code, "teleport_") {
		return trace.BadParameter("code %q must not start with the reserved teleport_ prefix", code)
	}
	for _, r := range code {
		if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '_') {
			return trace.BadParameter("code %q must contain only [a-z0-9_]", code)
		}
	}
	return nil
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

// Evaluate returns the first matching rule's decision, or a deny if none match.
// Because rules are allow-only, the order of evaluation does not affect whether
// the request is allowed, only which captures and allow_code a caller sees. On
// a full deny the decision carries every deny hint that fired across all rules.
func (s RuleSet) Evaluate(request Request, identity Identity) (Decision, error) {
	var hints []FiredHint
	for _, rule := range s {
		decision, err := rule.Evaluate(request, identity)
		if err != nil {
			return Decision{}, trace.Wrap(err)
		}
		if decision.Allowed {
			return decision, nil
		}
		hints = append(hints, decision.DenyHints...)
	}
	return Decision{DenyHints: hints}, nil
}
