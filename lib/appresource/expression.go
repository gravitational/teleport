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
	"go/ast"
	"go/parser"
	"maps"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/utils/typical"
)

// expressionParser parses app_resources_expressions entries.
var expressionParser = mustNewParser(expressionSpec())

// nodeParser and optionParser parse the arguments of a path.match call. The
// matcher constructors take only string literals, so the result does not
// depend on the request.
var (
	nodeParser   = mustNewConstantParser[Node]()
	optionParser = mustNewConstantParser[CompileOption]()
)

// compiledEntry is a compiled app_resources_expressions entry.
type compiledEntry struct {
	expression typical.Expression[Env, bool]
	// patterns are the compiled path.match calls of an entry with the
	// allow_unsafe_encoded option. It is nil for an entry without the option.
	patterns []*Pattern
}

// Evaluate returns the value of the entry for the given Env.
func (c *compiledEntry) Evaluate(e Env) (bool, error) {
	// Before the expression, because a negation or an || branch would turn a
	// failed path.match into an allow.
	if e.encodedSlash && !c.coversEncodedSlash(e.tokens) {
		return recordDenyHint(e, unsafeEncodedHint.Code, unsafeEncodedHint.Reason, false)
	}
	value, err := c.expression.Evaluate(e)
	return value, trace.Wrap(err)
}

// coversEncodedSlash returns true when one of the entry's patterns matches
// tokens. A pattern matches a token with an encoded slash only in a capture
// named by allow_unsafe_encoded, so a match means every encoded slash sits
// where the entry allows it.
func (c *compiledEntry) coversEncodedSlash(tokens []Token) bool {
	return slices.ContainsFunc(c.patterns, func(p *Pattern) bool {
		matched, _ := Match(p, tokens)
		return matched
	})
}

// expressionSpec returns the parser spec for an app_resources_expressions
// entry. expressionSpec is a clean superset of whereSpec. Therefore a
// compiled and valid "where" expression can always be evaluated by
// [evaluateExpression].
func expressionSpec() typical.ParserSpec[Env] {
	spec := whereSpec()
	spec.Functions["allow_code"] = typical.TernaryFunctionWithEnv(recordAllowCode)
	spec.Functions["deny_hint"] = typical.TernaryFunctionWithEnv(recordDenyHint)
	// path.match walks the request path tokens against the matcher root
	// and records the bound segments for later vars.<name> reads.
	spec.Functions["path.match"] = typical.BinaryVariadicFunctionWithEnv(func(e Env, root Node, options ...CompileOption) (bool, error) {
		if e.result == nil {
			return false, trace.BadParameter("internal error: evaluating path.match without an evaluation result")
		}
		tokens := e.tokens
		if tokens == nil {
			return false, trace.BadParameter("internal error: evaluating path.match without a tokenized path")
		}
		pattern, err := Compile(root, options...)
		if err != nil {
			return false, trace.Wrap(err)
		}
		if matched, captures := Match(pattern, tokens); matched {
			if e.result.vars == nil {
				e.result.vars = map[string]string{}
			}
			for _, k := range slices.Sorted(maps.Keys(captures)) {
				// A runtime check that should never trigger,
				// validateNoCaptureOverwrites covers it at compile time.
				if _, ok := e.result.vars[k]; ok {
					return false, trace.BadParameter("path.match binds capture %q a second time in one evaluation", k)
				}
				e.result.vars[k] = captures[k]
			}
			return true, nil
		}
		return false, nil
	})
	// The matcher constructors build the tree used by path.match.
	spec.Functions["literal"] = typical.BinaryVariadicFunction[Env](func(s string, children ...Node) (Node, error) {
		return Literal(s, children...), nil
	})
	spec.Functions["capture"] = typical.BinaryVariadicFunction[Env](func(name string, children ...Node) (Node, error) {
		return Capture(name, children...), nil
	})
	spec.Functions["glob"] = typical.UnaryVariadicFunction[Env](func(children ...Node) (Node, error) {
		return Glob(children...), nil
	})
	spec.Functions["glob_without"] = typical.BinaryVariadicFunction[Env](func(excludes []string, children ...Node) (Node, error) {
		return GlobWithout(excludes, children...), nil
	})
	spec.Functions["greedy"] = typical.NullaryFunction[Env, Node](func() (Node, error) {
		return Greedy(), nil
	})
	spec.Functions["greedy_except"] = typical.UnaryVariadicFunction[Env](func(exclusions ...Node) (Node, error) {
		return GreedyExcept(exclusions...), nil
	})
	spec.Functions["slash"] = typical.NullaryFunction[Env, Node](func() (Node, error) {
		return Slash(), nil
	})
	spec.Functions["optional"] = typical.UnaryVariadicFunction[Env](func(children ...Node) (Node, error) {
		return Optional(children...), nil
	})
	spec.Functions["root"] = typical.UnaryVariadicFunction[Env](func(children ...Node) (Node, error) {
		return Root(children...), nil
	})
	// allow_unsafe_encoded is the desugared form of one allow_unsafe.encoded
	// key.
	spec.Functions["allow_unsafe_encoded"] = typical.BinaryFunction[Env](func(name, chars string) (CompileOption, error) {
		return AllowUnsafeEncoded(name, chars), nil
	})
	return spec
}

// compileExpression parses and type-checks one app_resources_expressions
// entry.
func compileExpression(expr string) (*compiledEntry, error) {
	if strings.TrimSpace(expr) == "" {
		return nil, trace.BadParameter("an app_resources_expressions entry cannot be empty")
	}
	if len(expr) > maxExpressionBytes {
		return nil, trace.BadParameter("expression is %d bytes, over the %d byte maximum", len(expr), maxExpressionBytes)
	}
	parsed, err := parser.ParseExpr(expr)
	if err != nil {
		return nil, trace.NewAggregate(trace.BadParameter("compiling expression %q", expr), err)
	}
	expression, err := expressionParser.ParseAST(parsed)
	if err != nil {
		return nil, trace.NewAggregate(trace.BadParameter("compiling expression %q", expr), err)
	}
	validators := []func(ast.Expr) error{
		validateAuditLiterals,
		validateMatcherConstructors,
		validateVars,
		validateNoCaptureOverwrites,
		validateEncodedOptions,
	}
	for _, validate := range validators {
		if err := validate(parsed); err != nil {
			return nil, trace.Wrap(err, "compiling expression %q", expr)
		}
	}
	entry := &compiledEntry{expression: expression}
	if hasEncodedOption(parsed) {
		if entry.patterns, err = entryPatterns(parsed); err != nil {
			return nil, trace.Wrap(err, "compiling expression %q", expr)
		}
	}
	return entry, nil
}

// entryPatterns compiles the pattern of every path.match in parsed.
func entryPatterns(parsed ast.Expr) ([]*Pattern, error) {
	var patterns []*Pattern
	var err error
	ast.Inspect(parsed, func(n ast.Node) bool {
		if err != nil {
			return false
		}
		call, ok := n.(*ast.CallExpr)
		if !ok || !isPathMatch(call) {
			return true
		}
		var pattern *Pattern
		pattern, err = compilePathMatch(call)
		if err != nil {
			return false
		}
		patterns = append(patterns, pattern)
		return false
	})
	return patterns, trace.Wrap(err)
}

// compilePathMatch compiles the matcher tree and options of one path.match
// call.
func compilePathMatch(call *ast.CallExpr) (*Pattern, error) {
	root, err := evaluateConstant(nodeParser, call.Args[0])
	if err != nil {
		return nil, trace.Wrap(err)
	}
	options := make([]CompileOption, 0, len(call.Args)-1)
	for _, arg := range call.Args[1:] {
		option, err := evaluateConstant(optionParser, arg)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		options = append(options, option)
	}
	pattern, err := Compile(root, options...)
	return pattern, trace.Wrap(err)
}

// evaluateConstant evaluates expr, which must not read the request or the
// identity.
func evaluateConstant[T any](parser *typical.Parser[Env, T], expr ast.Expr) (T, error) {
	var zero T
	parsed, err := parser.ParseAST(expr)
	if err != nil {
		return zero, trace.Wrap(err)
	}
	value, err := parsed.Evaluate(Env{})
	if err != nil {
		return zero, trace.Wrap(err)
	}
	return value, nil
}

// mustNewConstantParser returns a parser for the expressionSpec values of
// type T, such as a matcher tree.
func mustNewConstantParser[T any]() *typical.Parser[Env, T] {
	p, err := typical.NewParser[Env, T](expressionSpec())
	if err != nil {
		panic(trace.Wrap(err, "building an app resource parser (this is a bug)"))
	}
	return p
}

// hasEncodedOption returns true when parsed contains an allow_unsafe_encoded
// call. validateEncodedOptions checks that every path.match in one entry has
// the same options.
func hasEncodedOption(parsed ast.Expr) bool {
	found := false
	ast.Inspect(parsed, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok && isIdentCall(call, "allow_unsafe_encoded") {
			found = true
		}
		return !found
	})
	return found
}

// recordAllowCode records code and reason on the evaluation result if value
// is true, and returns value unchanged. A later true call overrides an
// earlier one.
func recordAllowCode(e Env, code, reason string, value bool) (bool, error) {
	if e.result == nil {
		return false, trace.BadParameter("internal error: recording an allow code without an evaluation result")
	}
	if value {
		e.result.AuditRecord.AllowCode = code
		e.result.AuditRecord.AllowReason = clamp(reason, maxReasonBytes)
	}
	return value, nil
}

// recordDenyHint appends a hint to the evaluation result if value is false,
// and returns value unchanged. Hints past maxHints are dropped.
func recordDenyHint(e Env, code, reason string, value bool) (bool, error) {
	if e.result == nil {
		return false, trace.BadParameter("internal error: recording a deny hint without an evaluation result")
	}
	if !value && len(e.result.AuditRecord.DenyHints) < maxHints {
		hint := Hint{Code: code, Reason: clamp(reason, maxReasonBytes)}
		e.result.AuditRecord.DenyHints = append(e.result.AuditRecord.DenyHints, hint)
	}
	return value, nil
}

// evaluateExpression evaluates one compiled expression against env. The
// returned Result keeps the allow code and reason when the expression
// evaluated to true and the deny hints otherwise. evaluateExpression is
// used for desugared expressions and sugared "where" clauses.
func evaluateExpression(expression typical.Expression[Env, bool], env Env) (Result, error) {
	if err := validateMethod(env.Request.Method); err != nil {
		return Result{}, trace.Wrap(err)
	}
	var result Result
	env.result = &result
	value, err := expression.Evaluate(env)
	if err != nil {
		return Result{}, trace.Wrap(err)
	}
	result.Value = value
	if result.Value {
		result.AuditRecord.DenyHints = nil
	} else {
		result.AuditRecord.AllowCode = ""
		result.AuditRecord.AllowReason = ""
		result.vars = nil
	}
	return result, nil
}

// validateAuditLiterals checks the code and any literal reason of every
// allow_code and deny_hint call in the parsed expression. The code must be
// a string literal and pass validateAuditCode.
func validateAuditLiterals(parsed ast.Expr) error {
	var err error
	ast.Inspect(parsed, func(n ast.Node) bool {
		if err != nil {
			return false
		}
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		fnName := auditCallName(call)
		if fnName == "" {
			return true // not an allow_code or deny_hint call
		}
		code, ok := stringLiteral(call.Args[0])
		if !ok {
			err = trace.BadParameter("the code argument of %s must be a string literal", fnName)
			return false
		}
		err = validateAuditCode(code)
		if err != nil {
			return false
		}
		if reason, ok := stringLiteral(call.Args[1]); ok {
			err = validateReason(reason)
			if err != nil {
				return false
			}
		}
		return true
	})
	return trace.Wrap(err)
}

// clamp truncates s to at most limit bytes, cutting on a rune boundary.
func clamp(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	cut := limit
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut]
}
