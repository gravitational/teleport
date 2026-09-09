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
	"cmp"
	"go/ast"
	"go/parser"
	"slices"
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
)

// maxWhereBytes is the maximum length in bytes of one where clause, the sugared
// form.
const maxWhereBytes = 1 << 10 // 1 KiB

// maxExpressionBytes is the maximum length in bytes of one
// app_resources_expressions entry, the desugared form.
const maxExpressionBytes = 1 << 12 // 4 KiB

// maxReasonBytes is the maximum length in bytes of an allow_reason or
// deny_reason_hint.
const maxReasonBytes = 1 << 10 // 1 KiB

// maxHints is the maximum number of hints one evaluation can record.
const maxHints = 16

// maxAuditCodeBytes is the maximum length in bytes of an allow_code or
// deny_code_hint.
const maxAuditCodeBytes = 256

// maxPathBytes is the maximum length in bytes of one path pattern.
const maxPathBytes = 1 << 10 // 1 KiB

// maxPaths is the maximum number of path patterns in one rule.
const maxPaths = 64

// validateAuditCode checks an allow or deny code. A valid code is 1 to 256 bytes of
// [a-z0-9_] and does not start with the reserved teleport_ prefix.
func validateAuditCode(code string) error {
	if len(code) < 1 || len(code) > maxAuditCodeBytes {
		return trace.BadParameter("code %q must be 1 to %d bytes", code, maxAuditCodeBytes)
	}
	for _, r := range code {
		legal := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_'
		if !legal {
			return trace.BadParameter("code %q must contain only [a-z0-9_]", code)
		}
	}
	if strings.HasPrefix(code, "teleport_") {
		return trace.BadParameter("code %q must not start with the reserved teleport_ prefix", code)
	}
	return nil
}

// validateReason rejects a reason over maxReasonBytes.
func validateReason(reason string) error {
	if len(reason) > maxReasonBytes {
		return trace.BadParameter("reason is %d bytes, over the %d byte maximum", len(reason), maxReasonBytes)
	}
	return nil
}

// validateRule checks a rule's structural constraints, e.g. that AllowAll cannot be
// combined with another field. Path pattern checks are left to compile time.
func validateRule(r types.AppResource) error {
	if r.AllowAll {
		if !r.IsAllowAllOnly() {
			return trace.BadParameter("allow_all cannot be combined with any other field")
		}
		return nil
	}
	if len(r.Paths) == 0 {
		return trace.BadParameter("a rule must set paths or allow_all")
	}
	if err := validatePaths(r.Paths); err != nil {
		return trace.Wrap(err)
	}
	if err := validateMethods(r.Methods); err != nil {
		return trace.Wrap(err)
	}
	if err := validateWhere(r.Where); err != nil {
		return trace.Wrap(err)
	}
	for _, e := range r.AllowEncoded {
		if e != "/" {
			return trace.BadParameter("allow_encoded allows only the separator %q, got %q", "/", e)
		}
	}
	if r.AllowReason != "" && r.AllowCode == "" {
		return trace.BadParameter("allow_reason set without allow_code")
	}
	if r.AllowCode != "" {
		if err := validateAuditCode(r.AllowCode); err != nil {
			return trace.Wrap(err, "invalid allow_code")
		}
	}
	if err := validateReason(r.AllowReason); err != nil {
		return trace.Wrap(err, "invalid allow_reason")
	}
	if r.DenyReasonHint != "" && r.DenyCodeHint == "" {
		return trace.BadParameter("deny_reason_hint set without deny_code_hint")
	}
	if r.DenyCodeHint != "" {
		if err := validateAuditCode(r.DenyCodeHint); err != nil {
			return trace.Wrap(err, "invalid deny_code_hint")
		}
		if strings.TrimSpace(r.Where) == "" {
			return trace.BadParameter("deny_code_hint set without a where clause")
		}
	}
	if err := validateReason(r.DenyReasonHint); err != nil {
		return trace.Wrap(err, "invalid deny_reason_hint")
	}
	return nil
}

// validatePaths checks the count and byte caps on a rule's path patterns. The
// pattern syntax is checked when the rule compiles.
func validatePaths(paths []string) error {
	if len(paths) > maxPaths {
		return trace.BadParameter("a rule holds %d paths, over the cap of %d", len(paths), maxPaths)
	}
	for _, p := range paths {
		if len(p) > maxPathBytes {
			return trace.BadParameter("path is %d bytes, over the %d byte cap", len(p), maxPathBytes)
		}
	}
	return nil
}

// validateMethods rejects a name outside validMethods, folded to upper case, so
// "get" passes and "GTE" fails.
func validateMethods(methods []string) error {
	for _, m := range methods {
		if !slices.Contains(validMethods, strings.ToUpper(m)) {
			return trace.BadParameter("method %q is not one of %s", m, strings.Join(validMethods, ", "))
		}
	}
	return nil
}

// validateWhere checks the byte cap on a sugared rule's where clause. The where
// language itself is checked when the clause compiles.
func validateWhere(where string) error {
	if where == "" {
		return nil
	}
	if len(where) > maxWhereBytes {
		return trace.BadParameter("where clause is %d bytes, over the %d byte cap", len(where), maxWhereBytes)
	}
	return nil
}

// ruleEvaluator is one compiled app_resources or app_resources_expressions
// entry. It has the single Evaluate method of typical.Expression[Env, bool], so
// a compiled expression entry satisfies it as it is. compiledRule implements it
// for a sugared rule.
type ruleEvaluator interface {
	Evaluate(Env) (bool, error)
}

// compiledRule is a sugared rule ready to evaluate. It implements
// ruleEvaluator. Only newCompiledRule returns a usable value. Evaluation writes
// nothing back, so one compiledRule can serve concurrent requests.
type compiledRule struct {
	allowAll    bool
	pattern     *Pattern // the paths as one tree, nil when allowAll is set
	methods     []string // upper case, empty allows every method
	where       *Where   // nil when the rule has no where clause
	allowCode   string
	allowReason string
	denyCode    string
	denyReason  string
}

// newCompiledRule checks the rule and returns it ready to evaluate. It returns
// an error for an invalid rule, such as a vars.<name> read that some path does
// not bind.
func newCompiledRule(r types.AppResource) (*compiledRule, error) {
	if err := validateRule(r); err != nil {
		return nil, trace.Wrap(err)
	}
	if r.AllowAll {
		return &compiledRule{allowAll: true}, nil
	}
	var where *Where
	var reads []string // capture names the where clause reads, e.g. vars.project
	if clause := strings.TrimSpace(r.Where); clause != "" {
		var err error
		if where, err = CompileWhere(clause); err != nil {
			return nil, trace.Wrap(err)
		}
		if reads, err = varsReads(clause); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	pattern, err := compilePaths(r.Paths, reads)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &compiledRule{
		pattern:     pattern,
		methods:     upperCase(r.Methods),
		where:       where,
		allowCode:   r.AllowCode,
		allowReason: r.AllowReason,
		denyCode:    r.DenyCodeHint,
		denyReason:  r.DenyReasonHint,
	}, nil
}

// Evaluate returns true when environment e matches the rule, and records the
// allow code or deny hint on the evaluation result.
func (c *compiledRule) Evaluate(e Env) (bool, error) {
	if c.allowAll {
		return true, nil
	}
	if e.result == nil {
		return false, trace.BadParameter("internal error: evaluating a rule without an evaluation result")
	}
	if e.tokens == nil {
		return false, trace.BadParameter("internal error: evaluating a rule without a tokenized path")
	}
	if len(c.methods) > 0 && !slices.Contains(c.methods, e.Request.Method) {
		return false, nil
	}
	matched, captures := Match(c.pattern, e.tokens)
	if !matched {
		return false, nil
	}
	e.result.vars = captures
	// An unbound vars.<name> read is an evaluation error, not false, so the
	// where clause runs only after the path matched.
	value := true
	if c.where != nil {
		var err error
		if value, err = c.where.expression.Evaluate(e); err != nil {
			return false, trace.Wrap(err)
		}
	}
	if !value && c.denyCode != "" {
		return recordDenyHint(e, c.denyCode, c.denyReason, value)
	}
	if value && c.allowCode != "" {
		return recordAllowCode(e, c.allowCode, c.allowReason, value)
	}
	return value, nil
}

// compilePaths returns the path patterns compiled as one matcher tree. Several
// patterns become the alternatives of one Root. Every pattern must bind each
// name in requiredCaptures. compilePaths returns an error for an invalid
// pattern and for a pattern that lacks a required capture.
func compilePaths(patterns []string, requiredCaptures []string) (*Pattern, error) {
	trees := make([]Node, 0, len(patterns))
	for _, p := range patterns {
		tree, err := pathTree(p)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		names := captureNames(tree)
		for _, name := range requiredCaptures {
			if !slices.Contains(names, name) {
				return nil, trace.BadParameter("where reads vars.%s, but path %q has no {%s} capture", name, p, name)
			}
		}
		trees = append(trees, tree)
	}
	if len(trees) == 1 {
		return Compile(trees[0])
	}
	return Compile(Root(trees...))
}

// upperCase returns a copy of s with every element in upper case.
func upperCase(s []string) []string {
	upper := make([]string, 0, len(s))
	for _, e := range s {
		upper = append(upper, strings.ToUpper(e))
	}
	return upper
}

// varsReads returns the names a where clause reads as vars.<name>. It returns
// an error when the clause does not parse.
func varsReads(where string) ([]string, error) {
	parsed, err := parser.ParseExpr(where)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var names []string
	ast.Inspect(parsed, func(n ast.Node) bool {
		if sel, ok := n.(*ast.SelectorExpr); ok {
			if id, ok := sel.X.(*ast.Ident); ok && id.Name == "vars" {
				names = append(names, sel.Sel.Name)
			}
		}
		return true
	})
	return names, nil
}

// pathTree returns the matcher tree of one path pattern. "*" is Glob, "**"
// as the last segment is Greedy, "{name}" is Capture, "!seg" is GlobWithout, a
// trailing "/" is Slash, and other text is Literal.
func pathTree(pattern string) (Node, error) {
	if !strings.HasPrefix(pattern, "/") {
		return nil, trace.BadParameter("path pattern %q must start with /", pattern)
	}
	segments := strings.Split(pattern[1:], "/")
	last := len(segments) - 1
	node, err := lastSegmentNode(segments[last])
	for i := last - 1; i >= 0; i-- {
		var children []Node
		if node != nil {
			children = []Node{node}
		}
		var segErr error
		node, segErr = segmentNode(segments[i], children)
		// Report the leftmost bad segment.
		err = cmp.Or(segErr, err)
	}
	if err != nil {
		return nil, trace.Wrap(err, "path pattern %q", pattern)
	}
	// Compile the tree on its own so that an error from a constructor
	// is wrapped with the pattern.
	if _, err := Compile(node); err != nil {
		return nil, trace.Wrap(err, "path pattern %q", pattern)
	}
	return node, nil
}

// lastSegmentNode returns the node of the last pattern segment. A trailing
// "/" is Slash and "**" is Greedy. Any other segment is a segmentNode.
func lastSegmentNode(seg string) (Node, error) {
	switch seg {
	case "":
		return Slash(), nil
	case "**":
		return Greedy(), nil
	}
	return segmentNode(seg, nil)
}

// segmentNode returns the node of one pattern segment with children as
// its continuation. It returns an error for an empty segment and for "**",
// which are allowed only as the last segment.
func segmentNode(seg string, children []Node) (Node, error) {
	switch {
	case seg == "":
		return nil, trace.BadParameter("empty segment")
	case seg == "**":
		return nil, trace.BadParameter("** must be the last segment")
	case seg == "*":
		return Glob(children...), nil
	case strings.HasPrefix(seg, "{") && strings.HasSuffix(seg, "}"):
		return Capture(seg[1:len(seg)-1], children...), nil
	case strings.ContainsAny(seg, "*{}"):
		return nil, trace.BadParameter("segment %q is not a literal, *, **, {name}, or !seg", seg)
	case strings.HasPrefix(seg, "!"):
		return GlobWithout([]string{seg[1:]}, children...), nil
	default:
		return Literal(seg, children...), nil
	}
}
