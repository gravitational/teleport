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

// Rule is one app_resources entry, the sugared declarative authoring surface.
// It reads compactly for the common HTTP case and lowers to one predicate:
// Paths matches the request path, Methods the request method, and Where the
// caller identity and request. Where holds identity and request conditions
// only and may not call path.match, so all path matching flows through Paths;
// a full predicate that needs the matcher language directly is the separate
// app_resources_expression field, a list of predicate strings.
//
// AllowCode and AllowReason lower to a set_allow_code call appended to the
// predicate, so the sugared field and the expression primitive share one
// representation. DenyCode and DenyReason are a sugar-only audit feature: they
// explain a near-miss, when the rule's path and method matched but it did not
// allow, and have no expression form, so they do not appear in the desugared
// predicate.
type Rule struct {
	// Paths are declarative path patterns, OR-ed. A request matches the rule's
	// path territory if any pattern matches. Omit to match any path.
	Paths []string `yaml:"paths,omitempty"`
	// Methods are HTTP methods, OR-ed and upper-cased. Omit to match any
	// method.
	Methods []string `yaml:"methods,omitempty"`
	// Where is a condition over the caller identity, the request, and this
	// rule's captures. It does not match paths and may not call path.match.
	Where string `yaml:"where,omitempty"`
	// AllowCode is the structured code emitted on the allow audit event when
	// this rule matches. Without it, an allow emits no code.
	AllowCode string `yaml:"allow_code,omitempty"`
	// AllowReason is the human-readable explanation emitted alongside
	// AllowCode on an allow.
	AllowReason string `yaml:"allow_reason,omitempty"`
	// DenyCode is the structured code emitted on the deny audit event when the
	// rule's path and method matched but the rule did not allow. A where-only
	// rule, with no path or method to scope the near-miss, emits it on any deny
	// in the rule's scope.
	DenyCode string `yaml:"deny_code,omitempty"`
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

// CompiledRule is a parsed, ready-to-evaluate rule, built from a sugared Rule
// or from an app_resources_expression predicate string. The allow code and
// reason ride in the predicate as a set_allow_code call, read off the
// evaluation state on a match, so they are not fields here.
type CompiledRule struct {
	pred predicate
	// denyOn is the near-miss territory: the rule's path and method clauses. On
	// a deny the rule contributes its deny code when denyOn matches the request.
	// It is nil when the rule sets no deny code, when denyAlways fires the code
	// unconditionally, and for every expression rule, which carries no deny
	// mechanism.
	denyOn predicate
	// denyAlways fires the deny code on every deny in the rule's scope. It is
	// set for a where-only sugared rule, which has no path or method clause to
	// scope the near-miss.
	denyAlways bool
	denyCode   string
	denyReason string
}

// Compile validates a sugared rule and lowers it to one internal predicate. A
// rule must constrain something through at least one of paths, methods, or
// where; setting none is a load error.
func (r Rule) Compile() (*CompiledRule, error) {
	if len(r.Paths) == 0 && len(r.Methods) == 0 && r.Where == "" {
		return nil, trace.BadParameter("a rule must set at least one of paths, methods, where")
	}
	if err := validateWhereNoPathMatch(r.Where); err != nil {
		return nil, trace.Wrap(err)
	}
	if r.AllowCode != "" {
		if err := validateCode(r.AllowCode); err != nil {
			return nil, trace.Wrap(err, "invalid allow_code")
		}
	}

	expr, err := r.desugar()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	pred, err := compilePredicate(expr)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	denyOn, denyAlways, err := r.compileDenyOn()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &CompiledRule{
		pred:       pred,
		denyOn:     denyOn,
		denyAlways: denyAlways,
		denyCode:   r.DenyCode,
		denyReason: r.DenyReason,
	}, nil
}

// compileExpression compiles one app_resources_expression entry, a bare
// predicate string. Unlike a sugared rule's where clause, it may call
// path.match and use the full matcher language directly. It carries no deny
// mechanism: the deny code is a sugar-only feature, so an expression rule never
// contributes a deny hint.
func compileExpression(expr string) (*CompiledRule, error) {
	if strings.TrimSpace(expr) == "" {
		return nil, trace.BadParameter("an app_resources_expression entry cannot be empty")
	}
	pred, err := compilePredicate(expr)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &CompiledRule{pred: pred}, nil
}

// compilePredicate parses a predicate string and runs the load-time validators
// every predicate must pass, whether it came from a desugared sugar rule or a
// bare expression entry.
func compilePredicate(expr string) (predicate, error) {
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
	if err := validateAllowCodes(expr); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := validateAllowCodePlacement(expr); err != nil {
		return nil, trace.Wrap(err)
	}
	return pred, nil
}

// compileDenyOn parses the near-miss territory for a rule's deny code, the
// rule's path and method clauses ANDed. It returns a nil predicate and false
// when the rule sets no deny code. It returns denyAlways=true for a where-only
// rule, which has no path or method to scope the near-miss, so the deny code
// fires on any deny in the rule's scope.
func (r Rule) compileDenyOn() (predicate, bool, error) {
	if r.DenyCode == "" {
		if r.DenyReason != "" {
			return nil, false, trace.BadParameter("deny_reason set without deny_code")
		}
		return nil, false, nil
	}
	if err := validateCode(r.DenyCode); err != nil {
		return nil, false, trace.Wrap(err, "invalid deny_code")
	}
	path, err := r.pathClause()
	if err != nil {
		return nil, false, trace.Wrap(err)
	}
	on := joinClauses(path, r.methodClause())
	if on == "" {
		return nil, true, nil
	}
	onPred, err := parsePredicate(on)
	if err != nil {
		return nil, false, trace.Wrap(err, "compiling deny territory %q", on)
	}
	return onPred, false, nil
}

// desugar lowers the declarative fields to an equivalent predicate string,
// ANDing the path, method, where, and allow-code clauses. The path clause
// compiles each pattern to the canonical matcher tree and emits the matcher
// constructors as source, so the desugared declarative form parses to exactly
// the tree a hand-written predicate would. The allow code, when set, lowers to
// a trailing set_allow_code call, so the sugared field and the expression
// primitive share one representation. The deny code does not appear: it is a
// sugar-only audit feature with no expression form.
func (r Rule) desugar() (string, error) {
	path, err := r.pathClause()
	if err != nil {
		return "", trace.Wrap(err)
	}
	method := r.methodClause()
	allowCode := r.allowCodeClause()
	var where string
	if r.Where != "" {
		// Wrap the where in parentheses only when it is ANDed with another
		// clause, where the grouping matters. When the where is the whole
		// predicate, the parentheses add nothing.
		where = r.Where
		if path != "" || method != "" || allowCode != "" {
			where = "(" + where + ")"
		}
	}
	return joinClauses(path, method, where, allowCode), nil
}

// allowCodeClause renders the allow code and optional reason as a
// set_allow_code call, the predicate primitive the sugared fields lower to. It
// is empty when no allow code is set.
func (r Rule) allowCodeClause() string {
	if r.AllowCode == "" {
		return ""
	}
	if r.AllowReason != "" {
		return fmt.Sprintf("set_allow_code(%s, %s)", strconv.Quote(r.AllowCode), strconv.Quote(r.AllowReason))
	}
	return fmt.Sprintf("set_allow_code(%s)", strconv.Quote(r.AllowCode))
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
			Allow:   &AllowDetails{Vars: e.vars, Code: e.state.allowCode, Reason: e.state.allowReason},
		}, nil
	}

	var fired []Hint
	if c.denyCode != "" {
		matched := c.denyAlways
		if !matched && c.denyOn != nil {
			ok, err := evalPredicate(c.denyOn, request, identity)
			if err != nil {
				return Decision{}, trace.Wrap(err)
			}
			matched = ok
		}
		if matched {
			fired = append(fired, Hint{Code: c.denyCode, Reason: c.denyReason})
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

// Role is one role's app_resources and app_resources_expression, the unit the
// union is built from. It mirrors how a Teleport role carries its matchers
// under spec.allow, the same way services.RoleSet gathers its per-role matchers
// from each role. The two fields parallel the node_labels and
// node_labels_expression pair: Resources is the sugared declarative surface and
// Expressions is the bare predicate surface. A request is allowed if any rule
// in either field matches, so the two are an additive union, not a conjunction.
type Role struct {
	// Name is the role name, surfaced as an evaluated role on a decision.
	Name string
	// Resources are the role's app_resources entries, the sugared rules, OR-ed
	// within the role.
	Resources []Rule
	// Expressions are the role's app_resources_expression entries, bare
	// predicate strings, OR-ed within the role and with Resources.
	Expressions []string
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

// CompileRoles compiles the roles a caller holds into a RoleSet. Within each
// role the sugared Resources compile first, then the Expressions, so a
// matching sugared rule's captures and allow code surface ahead of an
// expression rule's. The order is cosmetic: rules are allow-only, so it never
// changes whether a request is allowed, only which detail a caller sees.
func CompileRoles(roles []Role) (RoleSet, error) {
	set := make(RoleSet, 0, len(roles))
	for _, role := range roles {
		cr := compiledRole{name: role.Name, rules: make([]*CompiledRule, 0, len(role.Resources)+len(role.Expressions))}
		for i, r := range role.Resources {
			c, err := r.Compile()
			if err != nil {
				return nil, trace.Wrap(err, "role %q app_resources %d", role.Name, i)
			}
			cr.rules = append(cr.rules, c)
		}
		for i, expr := range role.Expressions {
			c, err := compileExpression(expr)
			if err != nil {
				return nil, trace.Wrap(err, "role %q app_resources_expression %d", role.Name, i)
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
