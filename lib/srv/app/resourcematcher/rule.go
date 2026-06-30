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
	"slices"
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
// app_resources_expressions field, a list of predicate strings.
//
// AllowCode and AllowReason lower to an allow_code call wrapping the rule, and
// DenyCodeHint and DenyReasonHint to a deny_hint call wrapping the where, so
// each sugared field and its expression primitive share one representation. A
// deny hint explains a near-miss, when the rule's path and method matched but
// it did not allow.
type Rule struct {
	// Paths are declarative path patterns, OR-ed. A request matches the rule's
	// path territory if any pattern matches. A rule must set either Paths or
	// UnsafeAllowAll, since a rule that scopes nothing by path would grant
	// unrestricted access by accident; UnsafeAllowAll is the explicit way to
	// ask for that.
	Paths []string `yaml:"paths,omitempty"`
	// Methods are HTTP methods that further scope a Paths rule, OR-ed. If
	// Methods is empty, any method is permitted. Otherwise the request method
	// must appear in the list. Names are case-insensitive and validated
	// against the standard HTTP methods (GET, HEAD, POST, PUT, PATCH, DELETE,
	// OPTIONS, TRACE); an unknown method is a load error, so a typo fails
	// loudly rather than silently never matching. CONNECT is not a member: a
	// CONNECT request targets an authority, not a slash-path, so the tokenizer
	// rejects it and a rule listing CONNECT could never match.
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
	// DenyCodeHint is the structured code emitted on the deny audit event when
	// the rule's path and method matched but the rule did not allow. It explains
	// the near-miss within the rule's path-and-method scope.
	DenyCodeHint string `yaml:"deny_code_hint,omitempty"`
	// DenyReasonHint is the human-readable explanation emitted alongside
	// DenyCodeHint.
	DenyReasonHint string `yaml:"deny_reason_hint,omitempty"`
	// UnsafeAllowAll grants unrestricted access to every path and method,
	// restoring the pre-v9 behavior where a role that granted an app granted
	// all of it. It is the deliberate escape hatch for when the safe path
	// surface is too restrictive, such as traffic that carries percent-encoding
	// the matcher would otherwise reject. It bypasses every safety layer,
	// including the request tokenizer, so a path that would be rejected as
	// malformed or unsafe is forwarded as-is. Because it is all-or-nothing, it
	// cannot be combined with any other field; setting it alongside one is a
	// load error.
	UnsafeAllowAll bool `yaml:"unsafe_allow_all,omitempty"`
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
// or from an app_resources_expressions predicate string. The allow code and
// reason ride in the predicate as an allow_code wrapper, and the deny hints as
// deny_hint wrappers, read off the evaluation state on a match or a deny, so
// neither is a field here. One predicate captures the whole rule, so the
// sugared and the expression surfaces are exactly 1:1.
type CompiledRule struct {
	pred predicate
	// unsafeAllowAll marks a rule compiled from UnsafeAllowAll. The rule's
	// predicate is the constant true, so it allows every request, and the flag
	// lets the rule set skip the request tokenizer's pre-rule rejection so a
	// path that would be rejected as malformed still reaches this allow.
	unsafeAllowAll bool
}

// Compile validates a sugared rule and lowers it to one internal predicate. A
// rule must scope its access through paths, or opt into unrestricted access
// through unsafe_allow_all; setting neither is a load error. Methods and where
// further narrow a paths rule. unsafe_allow_all is all-or-nothing, so it cannot
// be combined with any other field.
func (r Rule) Compile() (*CompiledRule, error) {
	if err := r.validate(); err != nil {
		return nil, trace.Wrap(err)
	}
	if r.UnsafeAllowAll {
		// unsafe_allow_all lowers to the constant true: it allows every request
		// outright. The rule set reads unsafeAllowAll to skip the tokenizer's
		// pre-rule rejection too, so even a path the tokenizer would reject as
		// malformed reaches this allow.
		pred, err := compilePredicate("true")
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &CompiledRule{pred: pred, unsafeAllowAll: true}, nil
	}

	expr, err := r.desugar()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	pred, err := compilePredicate(expr)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &CompiledRule{pred: pred}, nil
}

// compileExpression compiles one app_resources_expressions entry, a bare
// predicate string. Unlike a sugared rule's where clause, it may call
// path.match and use the full matcher language directly. It may also wrap an
// inner condition in deny_hint to contribute a near-miss hint, the same
// primitive the sugared deny_code_hint lowers to, so an expression rule and a
// sugared rule share one deny mechanism.
func compileExpression(expr string) (*CompiledRule, error) {
	if strings.TrimSpace(expr) == "" {
		return nil, trace.BadParameter("an app_resources_expressions entry cannot be empty")
	}
	if len(expr) > maxExpressionBytes {
		return nil, trace.BadParameter(
			"app_resources_expressions entry is %d bytes, over the %d byte cap", len(expr), maxExpressionBytes)
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
	if err := validateLiterals(expr); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := validateEncodedSets(expr); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := validateAllowCodes(expr); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := validateDenyHintCodes(expr); err != nil {
		return nil, trace.Wrap(err)
	}
	return pred, nil
}

// desugar lowers the declarative fields to an equivalent predicate string,
// ANDing the path, method, and where-with-deny clauses, then wrapping the whole
// rule in an allow_code call when an allow code is set. The path clause
// compiles each pattern to the canonical matcher tree and emits the matcher
// constructors as source, so the desugared declarative form parses to exactly
// the tree a hand-written predicate would. The allow code wraps the rule
// because it must record on a match alone, and the deny code wraps the where
// because it must record on a near-miss alone; both audit fields share one
// representation with the expression surface and the lowering is exactly 1:1.
// unsafe_allow_all lowers to the constant true, which allows every request;
// its bypass of the request tokenizer is a property of the rule set, not the
// predicate, so the lowered form alone does not reproduce it on a malformed
// path.
func (r Rule) desugar() (string, error) {
	if err := r.validate(); err != nil {
		return "", trace.Wrap(err)
	}
	if r.UnsafeAllowAll {
		return "true", nil
	}
	path, err := r.pathClause()
	if err != nil {
		return "", trace.Wrap(err)
	}
	body := joinClauses(path, r.methodClause(), r.whereDenyClause())
	return r.wrapAllowCode(body), nil
}

// whereDenyClause renders the where condition, wrapped in a deny_hint call
// when a deny code is set. deny_hint records its hint when the wrapped
// expression is false and returns that value, so wrapping the where makes the
// hint fire on the near-miss: the path and method on its left of the && gate
// it to the rule's territory, and the wrapped where on a false reading records
// the hint and denies. validate rejects a deny code with no where, so a hint
// always has a near-miss condition to wrap; the empty-where branch here only
// renders an unhinted where.
func (r Rule) whereDenyClause() string {
	if r.Where == "" {
		return ""
	}
	if r.DenyCodeHint == "" {
		return r.Where
	}
	return fmt.Sprintf("deny_hint(%s, %s, %s)",
		strconv.Quote(r.DenyCodeHint),
		strconv.Quote(r.DenyReasonHint),
		r.Where)
}

// wrapAllowCode wraps the rule body in an allow_code call when an allow code
// is set, the predicate primitive the sugared fields lower to. allow_code
// records its code when the wrapped expression is true and returns that value,
// so wrapping the whole rule records the code only on a match. It returns the
// body unchanged when no allow code is set.
func (r Rule) wrapAllowCode(body string) string {
	if r.AllowCode == "" {
		return body
	}
	return fmt.Sprintf("allow_code(%s, %s, %s)",
		strconv.Quote(r.AllowCode),
		strconv.Quote(r.AllowReason),
		body)
}

// pathClause renders the Paths as one path.match over a root() of the compiled
// patterns. A lone pattern passes its tree straight through, since a root() that
// wraps a single child is redundant. It is empty when no paths are set. An
// encoded char is opted into per segment by an encoded node in the pattern,
// such as "{name:/}", so there is no rule-wide encoding flag to render.
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

// standardMethods is the set of HTTP methods a rule may name, the request
// methods defined by RFC 9110 less CONNECT. A rule that names anything else has
// a typo, so validateMethods rejects it at load rather than compiling a rule
// that can never match. CONNECT is excluded deliberately: it targets an
// authority rather than a slash-path, so the tokenizer rejects a CONNECT
// request and a rule listing it could only ever be a dead rule.
var standardMethods = map[string]bool{
	"GET":     true,
	"HEAD":    true,
	"POST":    true,
	"PUT":     true,
	"PATCH":   true,
	"DELETE":  true,
	"OPTIONS": true,
	"TRACE":   true,
}

// validateMethods rejects a method that is not a standard HTTP method. The
// comparison is case-insensitive, matching how methodClause upper-cases before
// the membership test, so "get" and "GET" are the same method and a typo such
// as "GTE" fails loudly at load rather than silently never matching. CONNECT is
// rejected here too, since a CONNECT request never reaches a rule.
func validateMethods(methods []string) error {
	for _, m := range methods {
		if !standardMethods[strings.ToUpper(m)] {
			return trace.BadParameter(
				"method %q is not a standard HTTP method (GET, HEAD, POST, PUT, PATCH, DELETE, OPTIONS, TRACE)", m)
		}
	}
	return nil
}

// validate checks a rule's structural constraints, the ones that decide whether
// the rule may be lowered at all. Both Compile and desugar call it, so the
// sugared and desugared surfaces agree on exactly which rules are valid: a rule
// that fails here errors the same way whether it is compiled or merely lowered
// for display. Value-level checks that the desugared predicate would itself
// reject, such as a malformed path pattern, are left to compilation.
func (r Rule) validate() error {
	if r.UnsafeAllowAll {
		return r.validateUnsafeAllowAllStandsAlone()
	}
	if len(r.Paths) == 0 {
		// A present-but-empty list (paths: []) lands here too: it is treated as
		// unset rather than silently allowed, so an author who clears the list
		// gets a load error instead of an accidental unrestricted grant.
		return trace.BadParameter("a rule must set paths or unsafe_allow_all")
	}
	if err := validateMethods(r.Methods); err != nil {
		return trace.Wrap(err)
	}
	if err := validateWhereLen(r.Where); err != nil {
		return trace.Wrap(err)
	}
	if err := validateWhereNoPathMatch(r.Where); err != nil {
		return trace.Wrap(err)
	}
	if r.AllowReason != "" && r.AllowCode == "" {
		// A reason explains a code, so a reason with no code has nothing to
		// qualify and would be silently dropped by wrapAllowCode. Reject it at
		// load, the same as the deny side rejects a reason hint with no code.
		return trace.BadParameter("allow_reason set without allow_code")
	}
	if r.AllowCode != "" {
		if err := validateCode(r.AllowCode); err != nil {
			return trace.Wrap(err, "invalid allow_code")
		}
	}
	if r.DenyReasonHint != "" && r.DenyCodeHint == "" {
		return trace.BadParameter("deny_reason_hint set without deny_code_hint")
	}
	if r.DenyCodeHint != "" {
		if err := validateCode(r.DenyCodeHint); err != nil {
			return trace.Wrap(err, "invalid deny_code_hint")
		}
		// A deny hint explains a near-miss, when the rule's path and method
		// matched but the where condition did not. With no where there is no
		// near-miss condition to wrap, so the hint could never fire. Reject it
		// at load rather than silently dropping it, so an author who sets a hint
		// on a path-only rule learns the hint does nothing.
		if r.Where == "" {
			return trace.BadParameter("deny_code_hint set without a where clause for the hint to qualify")
		}
	}
	return nil
}

// maxWhereBytes caps a sugared rule's where clause. The where compiles to one
// predicate evaluated per request, so an unbounded clause is both a parse cost
// and an audit-log hazard; the cap keeps an authored condition to a reviewable
// size.
const maxWhereBytes = 1 << 10 // 1 KiB

// maxExpressionBytes caps one app_resources_expressions entry, the bare
// predicate surface. It is wider than the where cap because an expression
// carries the whole rule, path match included, where the sugared where carries
// only the identity and request condition.
const maxExpressionBytes = 4 << 10 // 4 KiB

// validateWhereLen rejects a where clause over the byte cap.
func validateWhereLen(where string) error {
	if len(where) > maxWhereBytes {
		return trace.BadParameter("where clause is %d bytes, over the %d byte cap", len(where), maxWhereBytes)
	}
	return nil
}

// validateUnsafeAllowAllStandsAlone rejects an unsafe_allow_all rule that also
// sets another field. unsafe_allow_all grants everything, so any companion
// field is either redundant or a contradiction, and silently ignoring it would
// hide an authoring mistake.
func (r Rule) validateUnsafeAllowAllStandsAlone() error {
	if len(r.Paths) > 0 || len(r.Methods) > 0 || r.Where != "" ||
		r.AllowCode != "" || r.AllowReason != "" ||
		r.DenyCodeHint != "" || r.DenyReasonHint != "" {
		return trace.BadParameter("unsafe_allow_all cannot be combined with any other field")
	}
	return nil
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
		// A glob carrying exclusions is a glob_without: render the excluded set
		// as its leading argument so the round-trip keeps the carve-out rather
		// than dropping it back to a plain glob.
		if len(n.globExclude) > 0 {
			return constructorSource("glob_without", setSource(n.globExclude), n.children)
		}
		return constructorSource("glob", "", n.children)
	case kindGlobEncoded:
		return constructorSource("glob_encoded", setSource(n.allowedEncoded), n.children)
	case kindCaptureEncoded:
		// capture_encoded carries two leading args, the bound name and the
		// allowed-encoded set, so render the lead as the comma-joined pair.
		lead := strconv.Quote(n.text) + ", " + setSource(n.allowedEncoded)
		return constructorSource("capture_encoded", lead, n.children)
	case kindEncodedLiteral:
		// encoded_literal carries the decoded value and the allowed-encoded set
		// as its two leading args, the same pair form as capture_encoded.
		lead := strconv.Quote(n.text) + ", " + setSource(n.allowedEncoded)
		return constructorSource("encoded_literal", lead, n.children)
	case kindRoot:
		return constructorSource("root", "", n.children)
	case kindSlash:
		return "slash()"
	case kindOptional:
		return constructorSource("optional", "", n.children)
	case kindGreedy:
		// A greedy carrying exclusions is a greedy_except over the rendered
		// exclusion subtrees, the general form greedy_without also lowers to, so
		// the round-trip keeps the carve-out rather than dropping it back to a
		// plain greedy. A greedy never has children, so the exclusions are the
		// only arguments.
		if len(n.greedyExcept) > 0 {
			return constructorSource("greedy_except", "", n.greedyExcept)
		}
		return "greedy()"
	default:
		return ""
	}
}

// setSource renders a list of strings as a set(...) call, the form the encoded
// nodes take for their allowed-encoded chars and glob_without takes for its
// excluded values, so the rendered source parses back to the same values.
func setSource(values []string) string {
	quoted := make([]string, 0, len(values))
	for _, v := range values {
		quoted = append(quoted, strconv.Quote(v))
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
	// A read of a capture the matcher did not bind, or a path the tokenizer
	// rejected, forces the rule to no-match. Both can only deny, never widen, no
	// matter which operator read them, so an unbound capture or an unreadable
	// path fails closed even behind a negation.
	if allowed && !e.state.unboundRead && !e.state.tokenizeFailed {
		return Decision{
			Allowed: true,
			Allow:   &AllowDetails{Vars: e.vars, Code: e.state.allowCode, Reason: e.state.allowReason},
		}, nil
	}

	// On a deny, surface any near-miss hints the predicate recorded. A
	// deny_hint call records its hint only when the inner expression it wraps
	// is false, and the path on its left of the && gates it to the rule's
	// territory, so the hints are exactly the ones whose path and method
	// matched but whose inner condition failed.
	return Decision{Deny: &DenyDetails{Kind: DenyNotAllowed, Hints: e.state.denyHints}}, nil
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

// Role is one role's app_resources and app_resources_expressions, the unit the
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
	// Expressions are the role's app_resources_expressions entries, bare
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

// CompileRoles compiles the roles a caller holds into a RoleSet. The roles are
// evaluated in a deterministic order: by name lexicographically, then within
// each role the sugared Resources compile first and the Expressions second, so
// a matching sugared rule's captures and allow code surface ahead of an
// expression rule's. The order never changes whether a request is allowed,
// since rules are allow-only, only which allow code or hint order a caller sees;
// sorting by name pins that detail so it does not vary with the input slice
// order, which matters for the playground and for reproducible audit output.
func CompileRoles(roles []Role) (RoleSet, error) {
	sorted := make([]Role, len(roles))
	copy(sorted, roles)
	slices.SortFunc(sorted, func(a, b Role) int {
		return strings.Compare(a.Name, b.Name)
	})
	set := make(RoleSet, 0, len(sorted))
	for _, role := range sorted {
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
				return nil, trace.Wrap(err, "role %q app_resources_expressions %d", role.Name, i)
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
// rule-independent, so a single check at the top serves the whole set. An
// unsafe_allow_all rule anywhere in the set turns this floor off, since it
// grants everything by design, including a path the tokenizer would reject.
func (s RoleSet) Evaluate(request Request, identity Identity) (Decision, error) {
	roles := s.EvaluatedRoles()
	if hasRules, ok := s.canTokenize(request.Path); hasRules && !ok && !s.hasUnsafeAllowAll() {
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

// hasUnsafeAllowAll reports whether any rule in the set was compiled from
// unsafe_allow_all. Such a rule allows every request outright, so its presence
// turns off the tokenizer floor for the whole set: a path the tokenizer would
// reject as malformed is still forwarded, restoring the pre-v9 behavior the
// field opts into.
func (s RoleSet) hasUnsafeAllowAll() bool {
	for _, role := range s {
		for _, rule := range role.rules {
			if rule.unsafeAllowAll {
				return true
			}
		}
	}
	return false
}
