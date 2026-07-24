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

import (
	"fmt"
	"go/ast"
	goparser "go/parser"
	"strconv"
	"strings"

	"github.com/gravitational/trace"
)

// maxWhereBytes caps a sugared rule's where clause. The where compiles to
// one predicate evaluated per request, so an unbounded clause is both a
// parse cost and an audit-log hazard. The cap keeps an authored condition
// to a reviewable size.
const maxWhereBytes = 1 << 10 // 1 KiB

// maxExpressionBytes caps one app_resources_expressions entry, the bare
// predicate surface. It is wider than the where cap because an expression
// carries the whole rule, path match included, where the sugared where
// carries only the identity and request condition.
const maxExpressionBytes = 4 << 10 // 4 KiB

// maxReasonBytes caps an allow_reason or deny_reason_hint. A reason rides
// on every matching audit event, so the cap bounds the per-event payload.
const maxReasonBytes = 1 << 10 // 1 KiB

// Rule is one app_resources entry, the sugared declarative authoring
// surface. It reads compactly for the common HTTP case and lowers to one
// predicate: Paths matches the request path, Methods the request method,
// and Where the caller identity and request. Where holds conditions over
// the caller identity, the request, and the rule's captures, and may not
// call path.match, so all path matching flows through Paths. A full
// predicate that needs the matcher language directly is the separate
// app_resources_expressions field, a list of predicate strings.
//
// AllowCode and AllowReason lower to an allow_code call wrapping the
// rule, and DenyCodeHint and DenyReasonHint to a deny_hint call wrapping
// the where, so each sugared field and its expression primitive share one
// representation. A deny hint explains a near-miss, when the rule's path
// and method matched but it did not allow.
type Rule struct {
	// Paths are declarative path patterns, OR-ed. A request matches on
	// path if any pattern matches. A rule must set either Paths or
	// UnsafeAllowAll, since a rule that scopes nothing by path would
	// grant unrestricted access by accident. UnsafeAllowAll is the
	// explicit way to ask for that.
	Paths []string `yaml:"paths,omitempty"`
	// Methods are HTTP methods that further scope a Paths rule, OR-ed. If
	// Methods is empty, any method is permitted. Otherwise the request
	// method must appear in the list, matched case-insensitively: both the
	// listed names and the request method are folded to upper case before
	// the membership test, so a request sent as "get" matches a rule
	// listing "GET". Names are validated against the standard HTTP methods
	// (GET, HEAD, POST, PUT, PATCH, DELETE, OPTIONS, TRACE). An unknown
	// method is a load error, so a typo fails loudly rather than silently
	// never matching. CONNECT is not a member: a CONNECT request targets
	// an authority, not a slash-path, so the tokenizer rejects it and a
	// rule listing CONNECT could never match.
	Methods []string `yaml:"methods,omitempty"`
	// Where is a condition over the caller identity, the request, and this
	// rule's captures. It does not match paths and may not call path.match.
	Where string `yaml:"where,omitempty"`
	// AllowEncoded opts the rule's path match into the named encoded
	// chars, lowering to the allow_encoded option on path.match. In this
	// version only the separator "/" is admitted, so it is written
	// allow_encoded: ["/"]. It applies to the whole match and pairs with
	// an encoded node in a path, such as "{name:/}" or "{:/}". Without it
	// an encoded node never matches.
	AllowEncoded []string `yaml:"allow_encoded,omitempty"`
	// AllowCode is the structured code emitted on the allow audit event
	// when this rule matches. Without it, an allow emits no code.
	AllowCode string `yaml:"allow_code,omitempty"`
	// AllowReason is the human-readable explanation emitted alongside
	// AllowCode on an allow.
	AllowReason string `yaml:"allow_reason,omitempty"`
	// DenyCodeHint is the structured code emitted on the deny audit event
	// when the rule's path and method matched but the rule did not allow.
	// It explains the near-miss within the rule's path-and-method scope.
	DenyCodeHint string `yaml:"deny_code_hint,omitempty"`
	// DenyReasonHint is the human-readable explanation emitted alongside
	// DenyCodeHint.
	DenyReasonHint string `yaml:"deny_reason_hint,omitempty"`
	// UnsafeAllowAll grants unrestricted access to every path and method,
	// restoring the pre-v9 behavior where a role that granted an app
	// granted all of it. It is the deliberate opt-out for when the safe
	// path surface is too restrictive, such as traffic that carries
	// percent-encoding the matcher would otherwise reject. It bypasses
	// every safety layer, including the request tokenizer, so a path that
	// would be rejected as malformed or unsafe is forwarded as-is. Because
	// it is all-or-nothing, it cannot be combined with any other field.
	// Setting it alongside one is a load error.
	UnsafeAllowAll bool `yaml:"unsafe_allow_all,omitempty"`
}

// validate checks a rule's structural constraints, the ones that decide
// whether the rule may be lowered at all: field presence and combination
// rules, method names, the where clause, byte caps, and audit codes.
// Value-level checks that the lowered predicate would itself reject, such
// as a malformed path pattern, are left to compilation.
func (r Rule) validate() error {
	if r.UnsafeAllowAll {
		return r.validateUnsafeAllowAllStandsAlone()
	}
	if len(r.Paths) == 0 {
		// A present-but-empty list (paths: []) is treated as unset rather
		// than silently allowed, so an author who clears the list gets a
		// load error instead of an accidental unrestricted grant.
		return trace.BadParameter("a rule must set paths or unsafe_allow_all")
	}
	if err := validateMethods(r.Methods); err != nil {
		return trace.Wrap(err)
	}
	if err := validateWhere(r.Where); err != nil {
		return trace.Wrap(err)
	}
	for _, e := range r.AllowEncoded {
		if e != "/" {
			return trace.BadParameter("allow_encoded admits only the separator %q, got %q", "/", e)
		}
	}
	if r.AllowReason != "" && r.AllowCode == "" {
		// A reason explains a code, so a reason with no code has nothing
		// to qualify and would be silently dropped by the lowering.
		// Reject it at load.
		return trace.BadParameter("allow_reason set without allow_code")
	}
	if r.AllowCode != "" {
		if err := validateAuditCode(r.AllowCode); err != nil {
			return trace.Wrap(err, "invalid allow_code")
		}
	}
	if len(r.AllowReason) > maxReasonBytes {
		return trace.BadParameter("allow_reason is %d bytes, over the %d byte cap", len(r.AllowReason), maxReasonBytes)
	}
	if r.DenyReasonHint != "" && r.DenyCodeHint == "" {
		return trace.BadParameter("deny_reason_hint set without deny_code_hint")
	}
	if r.DenyCodeHint != "" {
		if err := validateAuditCode(r.DenyCodeHint); err != nil {
			return trace.Wrap(err, "invalid deny_code_hint")
		}
		// A deny hint qualifies the where condition. With no where there
		// is no near-miss to explain and the hint could never fire, so
		// reject it at load rather than silently dropping it.
		if r.Where == "" {
			return trace.BadParameter("deny_code_hint set without a where clause for the hint to qualify")
		}
	}
	if len(r.DenyReasonHint) > maxReasonBytes {
		return trace.BadParameter("deny_reason_hint is %d bytes, over the %d byte cap", len(r.DenyReasonHint), maxReasonBytes)
	}
	return nil
}

// validateUnsafeAllowAllStandsAlone rejects an unsafe_allow_all rule that
// also sets another field. unsafe_allow_all grants everything, so any
// companion field is either redundant or a contradiction, and silently
// ignoring it would hide an authoring mistake.
func (r Rule) validateUnsafeAllowAllStandsAlone() error {
	if len(r.Paths) > 0 || len(r.Methods) > 0 || r.Where != "" ||
		len(r.AllowEncoded) > 0 || r.AllowCode != "" || r.AllowReason != "" ||
		r.DenyCodeHint != "" || r.DenyReasonHint != "" {
		return trace.BadParameter("unsafe_allow_all cannot be combined with any other field")
	}
	return nil
}

// standardMethods is the set of HTTP methods a rule may name, the request
// methods defined by RFC 9110 less CONNECT. CONNECT is excluded
// deliberately: it targets an authority rather than a slash-path, so the
// tokenizer rejects a CONNECT request and a rule listing it could only
// ever be a dead rule.
var standardMethods = map[string]struct{}{
	"GET":     {},
	"HEAD":    {},
	"POST":    {},
	"PUT":     {},
	"PATCH":   {},
	"DELETE":  {},
	"OPTIONS": {},
	"TRACE":   {},
}

// validateMethods rejects a method that is not a standard HTTP method.
// The comparison folds to upper case, matching methodClause, so "get" and
// "GET" are the same method and a typo such as "GTE" fails at load rather
// than compiling a rule that never matches.
func validateMethods(methods []string) error {
	for _, m := range methods {
		if _, ok := standardMethods[strings.ToUpper(m)]; !ok {
			return trace.BadParameter("method %q is not a standard HTTP method (GET, HEAD, POST, PUT, PATCH, DELETE, OPTIONS, TRACE)", m)
		}
	}
	return nil
}

// methodClause renders the Methods as a membership test against the
// request method. It is empty when no methods are set, so an empty
// Methods list constrains nothing and any method is permitted. The listed
// methods are upper-cased and the request method is folded with upper()
// before the membership test, so matching is case-insensitive on both
// sides. Callers must validate the rule first: an unvalidated method
// renders a clause that never matches.
func (r Rule) methodClause() string {
	if len(r.Methods) == 0 {
		return ""
	}
	quoted := make([]string, 0, len(r.Methods))
	for _, m := range r.Methods {
		quoted = append(quoted, strconv.Quote(strings.ToUpper(m)))
	}
	return fmt.Sprintf("contains(set(%s), upper(request.method))", strings.Join(quoted, ", "))
}

// validateWhere checks a sugared rule's where clause: the byte cap, that
// the clause parses as one self-contained expression, and that it does
// not call path.match.
//
// The where uses the same Go expression syntax the engine parses, so this
// reuses go/parser to walk the AST. A clause that does not parse on its
// own is rejected rather than left to the engine, because the lowering
// splices the clause into a larger predicate string, and an unbalanced
// fragment that fails alone can parse inside the composite and change the
// rule's structure.
//
// Path matching in the sugared form flows through paths, so the where
// holds identity, request, and capture conditions only. A predicate that
// needs to call path.match directly belongs in
// app_resources_expressions.
func validateWhere(where string) error {
	if where == "" {
		return nil
	}
	if len(where) > maxWhereBytes {
		return trace.BadParameter("where clause is %d bytes, over the %d byte cap", len(where), maxWhereBytes)
	}
	parsed, err := goparser.ParseExpr(where)
	if err != nil {
		return trace.BadParameter("where clause does not parse: %v", err)
	}
	var bad bool
	ast.Inspect(parsed, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok && isPathMatch(call) {
			bad = true
		}
		return true
	})
	if bad {
		const msg = "where may not call path.match: express path matching through paths, or move the whole rule to app_resources_expressions"
		return trace.BadParameter(msg)
	}
	return nil
}

// isPathMatch reports whether call is a path.match(...) call.
func isPathMatch(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "match" {
		return false
	}
	id, ok := sel.X.(*ast.Ident)
	return ok && id.Name == "path"
}

// compileExpression compiles one app_resources_expressions entry, a bare
// predicate string. Unlike a sugared rule's where clause, it may use the
// full predicate language directly. It may also wrap an inner condition
// in deny_hint to contribute a near-miss hint, the same primitive the
// sugared deny_code_hint lowers to, so an expression rule and a sugared
// rule share one deny mechanism.
func compileExpression(expr string) (predicate, error) {
	if strings.TrimSpace(expr) == "" {
		return nil, trace.BadParameter("an app_resources_expressions entry cannot be empty")
	}
	if len(expr) > maxExpressionBytes {
		return nil, trace.BadParameter("app_resources_expressions entry is %d bytes, over the %d byte cap", len(expr), maxExpressionBytes)
	}
	pred, err := compilePredicate(expr)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return pred, nil
}
