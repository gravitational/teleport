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
	"bytes"
	"fmt"
	"go/format"
	"go/parser"
	"go/token"
	"strconv"
	"strings"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/gravitational/teleport/api/types"
)

// The segment kinds drawPattern draws from.
const (
	segLiteral = iota
	segGlob
	segGlobWithout
	segCapture
)

// drawRule draws a valid sugared rule. Every path binds the same capture
// names, so a where clause may read them.
func drawRule(t *rapid.T) types.AppResource {
	captures := rapid.SliceOfNDistinct(rapid.SampledFrom([]string{"p", "q"}), 0, 2, rapid.ID[string]).Draw(t, "captures")
	var rule types.AppResource
	for range rapid.IntRange(1, 3).Draw(t, "pathCount") {
		pattern, _ := drawPattern(t, captures)
		rule.Paths = append(rule.Paths, pattern)
	}
	if rapid.Bool().Draw(t, "hasMethods") {
		for _, m := range rapid.SliceOfNDistinct(rapid.SampledFrom(validMethods), 1, 3, rapid.ID[string]).Draw(t, "methods") {
			if rapid.Bool().Draw(t, "lowerCase") {
				m = strings.ToLower(m)
			}
			rule.Methods = append(rule.Methods, m)
		}
	}
	wheres := []string{`user.name == "alice"`, `contains(user.roles, "dev")`, "false", `user.name == "alice" || user.name == "bob"`}
	for _, name := range captures {
		wheres = append(wheres, "vars."+name+` == "a"`)
	}
	if rapid.Bool().Draw(t, "hasWhere") {
		rule.Where = rapid.SampledFrom(wheres).Draw(t, "where")
		if rapid.Bool().Draw(t, "hasDenyHint") {
			rule.DenyCodeHint = "denied"
			rule.DenyReasonHint = "Denied."
		}
	}
	if rapid.Bool().Draw(t, "hasAllowCode") {
		rule.AllowCode = "allowed"
		rule.AllowReason = "Allowed."
	}
	return rule
}

// drawPattern draws one path pattern that binds every name in captures once,
// and returns it with the matcher tree it denotes, built from the constructors.
func drawPattern(t *rapid.T, captures []string) (string, Node) {
	type segment struct {
		kind int
		text string
	}
	segments := make([]segment, 0, 6)
	for _, name := range captures {
		segments = append(segments, segment{segCapture, name})
	}
	for range rapid.IntRange(0, 3).Draw(t, "plainCount") {
		kind := rapid.SampledFrom([]int{segLiteral, segGlob, segGlobWithout}).Draw(t, "segmentKind")
		segments = append(segments, segment{kind, smallText.Draw(t, "text")})
	}
	segments = rapid.Permutation(segments).Draw(t, "order")

	var node Node
	var tailText []string
	switch {
	case len(segments) == 0 && rapid.Bool().Draw(t, "bareSlash"):
		node = Slash()
	case len(segments) == 0 || rapid.Bool().Draw(t, "greedy"):
		node = Greedy()
		tailText = []string{"**"}
	case rapid.Bool().Draw(t, "trailingSlash"):
		node = Slash()
		tailText = []string{""}
	}
	for i := len(segments) - 1; i >= 0; i-- {
		var children []Node
		if node != nil {
			children = []Node{node}
		}
		seg := segments[i]
		switch seg.kind {
		case segLiteral:
			node = Literal(seg.text, children...)
		case segGlob:
			node = Glob(children...)
		case segGlobWithout:
			node = GlobWithout([]string{seg.text}, children...)
		default:
			node = Capture(seg.text, children...)
		}
	}
	texts := make([]string, 0, len(segments)+1)
	for _, seg := range segments {
		switch seg.kind {
		case segLiteral:
			texts = append(texts, seg.text)
		case segGlob:
			texts = append(texts, "*")
		case segGlobWithout:
			texts = append(texts, "!"+seg.text)
		default:
			texts = append(texts, "{"+seg.text+"}")
		}
	}
	texts = append(texts, tailText...)
	return "/" + strings.Join(texts, "/"), node
}

// TestPathTreeProperty checks that pathTree parses a drawn pattern into the
// tree the pattern was drawn from.
func TestPathTreeProperty(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		captures := rapid.SliceOfNDistinct(rapid.SampledFrom([]string{"p", "q"}), 0, 2, rapid.ID[string]).Draw(t, "captures")
		pattern, want := drawPattern(t, captures)
		got, err := pathTree(pattern)
		require.NoError(t, err, "pathTree(%q)", pattern)
		require.Equal(t, nodeToSource(want), nodeToSource(got), "pathTree(%q)", pattern)
	})
}

// TestRuleEvaluateProperty compares the compiled rule against the desugared
// entry on the whole Result, for arbitrary rules and requests.
func TestRuleEvaluateProperty(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		rule := drawRule(t)
		compiled, err := newCompiledRule(rule)
		require.NoError(t, err, "compile %+v", rule)
		entry, err := desugar(rule)
		require.NoError(t, err, "desugar %+v", rule)
		expression, err := compileExpression(entry)
		require.NoError(t, err, "compile entry %q", entry)

		segmentText := rapid.SampledFrom([]string{"a", "b", "c", "x", "a%20b", "caf%C3%A9"})
		segments := rapid.SliceOfN(segmentText, 0, 5).Draw(t, "segments")
		path := "/" + strings.Join(segments, "/")
		if rapid.Bool().Draw(t, "requestTrailingSlash") && len(segments) > 0 {
			path += "/"
		}
		request := Request{Method: rapid.SampledFrom(validMethods).Draw(t, "method"), Path: path}
		identity := Identity{
			Name:  rapid.SampledFrom([]string{"alice", "bob"}).Draw(t, "name"),
			Roles: []string{rapid.SampledFrom([]string{"dev", "ops"}).Draw(t, "role")},
		}
		env, err := NewEnv(request, identity)
		require.NoError(t, err, "NewEnv(%q)", path)

		got, err := evaluateExpression(compiled, env)
		require.NoError(t, err, "evaluate %+v against %s %q", rule, request.Method, path)
		want, err := evaluateExpression(expression, env)
		require.NoError(t, err, "evaluate %q against %s %q", entry, request.Method, path)
		require.Equal(t, want, got, "rule %+v, desugared %q, on %s %q", rule, entry, request.Method, path)
	})
}

// desugar returns the app_resources_expressions entry equivalent to the rule.
// For example, the rule
//
//	app_resources:
//	  - paths: ["/api/v4/projects/{project}/**"]
//	    methods: [GET, HEAD]
//	    where: contains(user.traits["projects"], vars.project)
//	    deny_code_hint: not_in_projects
//	    deny_reason_hint: Project not allowed.
//
// desugars to
//
//	app_resources_expressions:
//	  - |
//	    path.match(literal("api/v4/projects", capture("project", greedy()))) &&
//	    contains(set("GET", "HEAD"), request.method) &&
//	    deny_hint("not_in_projects", "Project not allowed.",
//	      contains(user.traits["projects"], vars.project))
//
// The returned entry is on one line. An allow_all rule desugars to "true".
// A rule near the caps on paths desugars to an entry over the 4 KiB cap on an
// app_resources_expressions entry, which compileExpression rejects.
// desugar returns an error for an invalid rule, such as a vars.<name> read
// that some path does not bind.
func desugar(r types.AppResource) (string, error) {
	if err := validateRule(r); err != nil {
		return "", trace.Wrap(err)
	}
	if r.AllowAll {
		return "true", nil
	}
	where := strings.TrimSpace(r.Where)
	var reads []string
	if where != "" {
		if _, err := CompileWhere(where); err != nil {
			return "", trace.Wrap(err)
		}
		parsed, err := parser.ParseExpr(where)
		if err != nil {
			return "", trace.Wrap(err)
		}
		// Printing the parsed clause drops comments, so a trailing line
		// comment in where cannot comment out the closing parenthesis of
		// the deny_hint call.
		var buf bytes.Buffer
		if err := format.Node(&buf, token.NewFileSet(), parsed); err != nil {
			return "", trace.Wrap(err)
		}
		where = buf.String()
		if reads, err = varsReads(where); err != nil {
			return "", trace.Wrap(err)
		}
	}
	pattern, err := compilePaths(r.Paths, reads)
	if err != nil {
		return "", trace.Wrap(err)
	}
	clauses := []string{"path.match(" + nodeToSource(pattern.root) + ")"}
	if len(r.Methods) > 0 {
		clauses = append(clauses, methodClause(r.Methods))
	}
	// An unbound vars.<name> read is an evaluation error, not false, so the
	// where clause must stay after path.match.
	if where != "" {
		clauses = append(clauses, whereClause(where, r.DenyCodeHint, r.DenyReasonHint))
	}
	entry := strings.Join(clauses, " && ")
	if r.AllowCode != "" {
		entry = fmt.Sprintf("allow_code(%s, %s, %s)", strconv.Quote(r.AllowCode), strconv.Quote(r.AllowReason), entry)
	}
	return entry, nil
}

// methodClause returns the membership test of the request method in the
// upper-cased methods.
func methodClause(methods []string) string {
	return "contains(" + setSource(upperCase(methods)) + ", request.method)"
}

// whereClause returns the parenthesized where clause, or the where clause
// wrapped in a deny_hint call when code is set.
func whereClause(where, code, reason string) string {
	if code == "" {
		return "(" + where + ")"
	}
	return fmt.Sprintf("deny_hint(%s, %s, %s)", strconv.Quote(code), strconv.Quote(reason), where)
}

// nodeToSource returns the tree as source the expression parser accepts. A
// chain of single-child literals renders as one literal.
func nodeToSource(node Node) string {
	children := node.children()
	var name string
	var args []string
	switch n := node.(type) {
	case *literalNode:
		texts := []string{n.text}
		for len(children) == 1 {
			child, ok := children[0].(*literalNode)
			if !ok {
				break
			}
			texts = append(texts, child.text)
			children = child.childNodes
		}
		name, args = "literal", []string{strconv.Quote(strings.Join(texts, "/"))}
	case *captureNode:
		name, args = "capture", []string{strconv.Quote(n.name)}
	case *globWithoutNode:
		name, args = "glob_without", []string{setSource(n.excludes)}
	case *globNode:
		name = "glob"
	case *optionalNode:
		name = "optional"
	case *rootNode:
		name = "root"
	default:
		return node.String() // greedy() and slash() have no arguments.
	}
	for _, child := range children {
		args = append(args, nodeToSource(child))
	}
	return name + "(" + strings.Join(args, ", ") + ")"
}
