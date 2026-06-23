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

// DenyKind is the structured reason a request was denied. Its values are the
// top-level reason_code emitted on the app.session.request.deny audit event, so
// the type reads as a category in Go while it serializes straight to the audit
// string in JSON.
type DenyKind string

const (
	// DenyNotAllowed is the kind for a well-formed request that no allow rule
	// matched. A fired hint explains the near-miss, and an empty EvaluatedRoles
	// marks a misconfigured default-deny, where no role carried any
	// app_resources, as opposed to a request a granting role did not match.
	DenyNotAllowed DenyKind = "teleport_request_not_allowed"
	// DenyInvalidRequest is the kind for a request denied before any rule
	// evaluated, because its path was rejected as malformed or unsafe, such as
	// a "." or ".." segment, consecutive slashes, an illegal byte, or a
	// percent-escape other than the encoded separator %2F.
	DenyInvalidRequest DenyKind = "teleport_invalid_request"
)

// Decision is the outcome of evaluating a rule or role set against a request.
// Allowed is the verdict; exactly one of Allow or Deny carries the matching
// detail, so allow-only and deny-only fields cannot be read on the wrong
// outcome. EvaluatedRoles rides both, since the audit event emits it either
// way.
type Decision struct {
	// Allowed reports whether any rule matched.
	Allowed bool
	// Allow carries the captures and codes of the matching rule. It is non-nil
	// if and only if Allowed.
	Allow *AllowDetails
	// Deny carries the deny kind and any fired hints. It is non-nil if and only
	// if not Allowed.
	Deny *DenyDetails
	// EvaluatedRoles lists the roles that carried app_resources for the app, in
	// the order they were evaluated. An empty list on a deny marks a
	// misconfigured default-deny, where no role granted any app_resources, as
	// opposed to a request that a granting role did not match. The RoleSet
	// derives this from the roles it was built from.
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

// Hint is a deny hint that matched on a deny.
type Hint struct {
	Code   string
	Reason string
}

// CompiledRule is a parsed, ready-to-evaluate rule.
type CompiledRule struct {
	pred        predicate
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
	if err := validateExclusions(expr); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := validateRoot(expr); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := validateLiterals(expr); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := validateEncodedSets(expr); err != nil {
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
		if err := validateExclusions(on); err != nil {
			return nil, trace.Wrap(err, "deny_hint %d", i)
		}
		if err := validateRoot(on); err != nil {
			return nil, trace.Wrap(err, "deny_hint %d", i)
		}
		if err := validateLiterals(on); err != nil {
			return nil, trace.Wrap(err, "deny_hint %d", i)
		}
		if err := validateEncodedSets(on); err != nil {
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

// pathClause renders the Paths as one path.match over a root() of the compiled
// patterns. A lone pattern passes its tree straight through, since a root() that
// wraps a single child is redundant. It is empty when no paths are set.
func (r Rule) pathClause() (string, error) {
	if len(r.Paths) == 0 {
		return "", nil
	}
	nodes := make([]*Node, 0, len(r.Paths))
	for _, p := range r.Paths {
		node, err := Compile(p)
		if err != nil {
			return "", trace.Wrap(err)
		}
		nodes = append(nodes, node)
	}
	// Emit a single path.match. Several patterns merge into one tree that shares
	// any common prefix, so paths branch only where they diverge and the prefix
	// is never duplicated. Patterns that share no first segment stay distinct
	// roots, OR-ed through one root() node, the one place an alternation needs a
	// synthetic parent. A lone pattern, or several that merge to one root, passes
	// its tree straight through, since a root() wrapping a single child is
	// redundant.
	merged := mergeAlternatives(nodes)
	tree := merged[0]
	if len(merged) > 1 {
		tree = &Node{kind: kindRoot, children: merged}
	}
	return fmt.Sprintf("path.match(%s)", nodeToSource(tree)), nil
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
	case kindGlobEncoded:
		return constructorSource("glob_encoded", encodedSetSource(n.allowedEncoded), n.children)
	case kindCaptureEncoded:
		// capture_encoded carries two leading args, the bound name and the
		// allowed-encoded set, so render the lead as the comma-joined pair.
		lead := strconv.Quote(n.text) + ", " + encodedSetSource(n.allowedEncoded)
		return constructorSource("capture_encoded", lead, n.children)
	case kindEncodedLiteral:
		// encoded_literal carries the decoded value and the allowed-encoded set
		// as its two leading args, the same pair form as capture_encoded.
		lead := strconv.Quote(n.text) + ", " + encodedSetSource(n.allowedEncoded)
		return constructorSource("encoded_literal", lead, n.children)
	case kindRoot:
		return constructorSource("root", "", n.children)
	case kindSlash:
		return "slash()"
	case kindOptional:
		return constructorSource("optional", "", n.children)
	case kindGreedy:
		return "greedy()"
	default:
		return ""
	}
}

// encodedSetSource renders the allowed-encoded chars of an encoded node as a
// set(...) call, the form glob_encoded and capture_encoded take as their
// leading argument, so the rendered source parses back to the same node.
func encodedSetSource(allowed []string) string {
	quoted := make([]string, 0, len(allowed))
	for _, c := range allowed {
		quoted = append(quoted, strconv.Quote(c))
	}
	return "set(" + strings.Join(quoted, ", ") + ")"
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
	e := newEnv(request, identity)
	allowed, err := c.pred.Evaluate(e)
	if err != nil {
		return Decision{}, trace.Wrap(err)
	}
	// A read of a capture the matcher did not bind, a path the tokenizer
	// rejected, or an encoded char in a match that did not opt in, forces the
	// rule to no-match. All three can only deny, never widen, no matter which
	// operator read them, so an unbound capture, an unreadable path, or an
	// unexpected encoded segment fails closed even behind a negation.
	if allowed && !e.state.unboundRead && !e.state.tokenizeFailed && !e.state.encodedNotAllowed {
		return Decision{
			Allowed: true,
			Allow:   &AllowDetails{Vars: e.vars, Code: c.allowCode, Reason: c.allowReason},
		}, nil
	}

	var fired []Hint
	for _, h := range c.denyHints {
		ok, err := evalPredicate(h.on, request, identity)
		if err != nil {
			return Decision{}, trace.Wrap(err)
		}
		if ok {
			fired = append(fired, Hint{Code: h.denyCode, Reason: h.denyReason})
		}
	}
	return Decision{Deny: &DenyDetails{Kind: DenyNotAllowed, Hints: fired}}, nil
}

// newEnv builds a fresh evaluation environment for one request. The first
// path.match tokenizes the path lazily into the shared state.
func newEnv(request Request, identity Identity) env {
	return env{
		request: request,
		user:    identity,
		vars:    map[string]string{},
		state:   &evalState{},
	}
}

// evalPredicate evaluates a predicate against a fresh environment and applies
// the no-match guards, returning whether it matched. An unbound capture read, a
// path the tokenizer rejected, or an encoded char in a match that did not opt
// in forces a no-match.
func evalPredicate(p predicate, request Request, identity Identity) (bool, error) {
	e := newEnv(request, identity)
	ok, err := p.Evaluate(e)
	if err != nil {
		return false, trace.Wrap(err)
	}
	return ok && !e.state.unboundRead && !e.state.tokenizeFailed && !e.state.encodedNotAllowed, nil
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

// Role is one role's app_resources, the unit the union is built from. It
// mirrors how a Teleport role carries app_resources under spec.allow, the same
// way services.RoleSet gathers its per-role matchers from each role.
type Role struct {
	// Name is the role name, surfaced as an evaluated role on a decision.
	Name string
	// Rules are the role's app_resources entries, OR-ed within the role.
	Rules []Rule
}

// RoleSet is the additive-OR union of the app_resources a caller holds, built
// from one or more roles. A request is allowed if any rule in any role matches.
// The set remembers its role names, so a decision reports the evaluated roles
// without a caller-supplied list, the way iterating a services.RoleSet reveals
// which role granted access.
type RoleSet []compiledRole

// compiledRole is one role's compiled rules.
type compiledRole struct {
	name  string
	rules []*CompiledRule
}

// CompileRoles compiles the roles a caller holds into a RoleSet.
func CompileRoles(roles []Role) (RoleSet, error) {
	set := make(RoleSet, 0, len(roles))
	for _, role := range roles {
		cr := compiledRole{name: role.Name, rules: make([]*CompiledRule, 0, len(role.Rules))}
		for i, r := range role.Rules {
			c, err := r.Compile()
			if err != nil {
				return nil, trace.Wrap(err, "role %q rule %d", role.Name, i)
			}
			cr.rules = append(cr.rules, c)
		}
		set = append(set, cr)
	}
	return set, nil
}

// EvaluatedRoles returns the names of the roles in the set, the roles that
// carried app_resources for the app. An empty result marks the misconfigured
// default-deny, where no role granted any app_resources.
func (s RoleSet) EvaluatedRoles() []string {
	names := make([]string, 0, len(s))
	for _, role := range s {
		names = append(names, role.name)
	}
	return names
}

// Evaluate returns the first matching rule's decision, or a deny if none match.
// Because rules are allow-only, the order of evaluation does not affect whether
// the request is allowed, only which captures and allow_code a caller sees. On
// a full deny the decision carries every deny hint that fired across all roles.
// Every decision carries the set's evaluated roles, so the audit log can tell a
// misconfigured default-deny, an empty set, from a request that a granting role
// did not match.
//
// A request whose path the tokenizer rejects is malformed or unsafe (an illegal
// byte, a percent-escape other than the encoded separator %2F, a "." or ".."
// segment, or consecutive slashes), so it is denied with DenyInvalidRequest
// before any rule runs, mirroring the agent's pre-rule rejection. Tokenizing is
// rule-independent, so a single check at the top serves the whole set.
func (s RoleSet) Evaluate(request Request, identity Identity) (Decision, error) {
	roles := s.EvaluatedRoles()
	if hasRules, ok := s.canTokenize(request.Path); hasRules && !ok {
		return Decision{
			Deny:           &DenyDetails{Kind: DenyInvalidRequest},
			EvaluatedRoles: roles,
		}, nil
	}

	var hints []Hint
	for _, role := range s {
		for _, rule := range role.rules {
			decision, err := rule.Evaluate(request, identity)
			if err != nil {
				return Decision{}, trace.Wrap(err)
			}
			if decision.Allowed {
				decision.EvaluatedRoles = roles
				return decision, nil
			}
			hints = append(hints, decision.Deny.Hints...)
		}
	}
	return Decision{
		Deny:           &DenyDetails{Kind: DenyNotAllowed, Hints: hints},
		EvaluatedRoles: roles,
	}, nil
}

// canTokenize reports whether the set holds any rule, and whether path
// tokenizes. A path the tokenizer rejects is malformed; an empty set has no
// rules and is a misconfigured default-deny rather than an invalid request, so
// hasRules guards the invalid-request verdict.
func (s RoleSet) canTokenize(path string) (hasRules, ok bool) {
	for _, role := range s {
		if len(role.rules) > 0 {
			hasRules = true
			break
		}
	}
	if !hasRules {
		return false, false
	}
	_, err := Tokenize(path)
	return true, err == nil
}
