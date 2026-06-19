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
	"slices"
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/utils/typical"
)

// Request is the HTTP request subject a predicate evaluates against.
type Request struct {
	Method string
	Path   string
}

// Identity is the caller subject a predicate evaluates against. It is a plain
// struct so a predicate can be exercised in a test with no cluster.
type Identity struct {
	Name   string
	Roles  []string
	Traits map[string][]string
}

// env is the evaluation environment threaded through one predicate evaluation.
// The vars map is the channel between a matcher and a later identity condition:
// a path.match call writes the segments its captures bind, and a vars.<name>
// read pulls them back out. Because typical evaluates the left side of && to
// completion before the right side, a vars read guarded behind its match by &&
// always observes the bound value. A fresh env, and therefore a fresh vars map
// and state, is built per request, so concurrent evaluations never share
// captures.
type env struct {
	request Request
	user    Identity
	vars    map[string]string
	state   *evalState
}

// Opt configures URL decoding for a single path.match call. The opt builders
// decode_iterations(n) and allow_percent() each return one Opt, and a
// path.match call applies the opts it carries to build its DecodeConfig. The
// options ride on the call rather than on the rule so a path.match expression
// is self-contained: it decodes exactly as written, with nothing pulled from a
// surrounding YAML field. A rule's Compile step checks every path.match in the
// rule carries identical options, so a carve-out's negated match cannot decode
// the subject differently from its positive match.
type Opt func(*DecodeConfig)

// evalState carries side effects of one evaluation back to the caller. It is
// held by pointer so the same instance is observed across the whole expression
// tree, even though env is passed by value.
type evalState struct {
	// unboundRead records that the predicate read a vars.<name> the matcher
	// never bound on this request. A bound capture is always a non-empty
	// segment, so an absent name can only mean unbound. The caller forces the
	// rule to deny when this is set, which makes an unbound read fail closed
	// regardless of the operator that read it (a bare vars.x != "admin" cannot
	// widen).
	unboundRead bool
}

// predicate is a parsed, type-checked app-access predicate ready to evaluate.
type predicate = typical.Expression[env, bool]

// parser is the shared, cached predicate parser. It registers the matcher
// constructors and the app-access bindings on top of the generic engine; the
// lexer, type checker, caching, and evaluator are reused unchanged.
var parser = mustNewParser()

func mustNewParser() *typical.CachedParser[env, bool] {
	p, err := newParser()
	if err != nil {
		panic(trace.Wrap(err, "building app-access predicate parser (this is a bug)"))
	}
	return p
}

func newParser() (*typical.CachedParser[env, bool], error) {
	p, err := typical.NewCachedParser[env, bool](typical.ParserSpec[env]{
		Variables: map[string]typical.Variable{
			"user.name": typical.DynamicVariable(func(e env) (string, error) {
				return e.user.Name, nil
			}),
			"user.roles": typical.DynamicVariable(func(e env) ([]string, error) {
				return e.user.Roles, nil
			}),
			"user.traits": typical.DynamicMapFunction(func(e env, key string) ([]string, error) {
				return e.user.Traits[key], nil
			}),
			"request.method": typical.DynamicVariable(func(e env) (string, error) {
				return e.request.Method, nil
			}),
			"request.path": typical.DynamicVariable(func(e env) (string, error) {
				return e.request.Path, nil
			}),
		},
		Functions: map[string]typical.Function{
			// Matcher entry point. path.match reads the request path from the
			// environment, tokenizes it under the decode config its opts build,
			// and walks it against the matcher root. On a match it records the
			// bound segments into the environment's vars map so a later
			// var.<name> read can see them, then returns true. The first
			// argument is the matcher root; any further arguments are decode
			// options. Several patterns OR together as several path.match calls
			// joined by ||, so each call carries one root.
			"path.match": typical.BinaryVariadicFunctionWithEnv(func(e env, root *Node, opts ...Opt) (bool, error) {
				var cfg DecodeConfig
				for _, o := range opts {
					o(&cfg)
				}
				tokens, err := Tokenize(e.request.Path, cfg)
				if err != nil {
					// An unsafe path is not this predicate's concern to report;
					// it simply cannot match. The agent rejects such requests
					// before any rule runs.
					return false, nil
				}
				if ok, caps := Eval(tokens, root); ok {
					for k, v := range caps {
						e.vars[k] = v
					}
					return true, nil
				}
				return false, nil
			}),
			// Decode options for a path.match call. decode_iterations(n) sets
			// the number of percent-decode passes, and allow_percent() admits a
			// residual percent byte that survives those passes. Both default to
			// the strict zero value when omitted: no decode, and reject any
			// percent byte.
			"decode_iterations": typical.UnaryFunction[env](func(n int) (Opt, error) {
				if n < 0 || n > maxDecodeIterations {
					return nil, trace.BadParameter("decode_iterations must be between 0 and %d", maxDecodeIterations)
				}
				return func(c *DecodeConfig) { c.DecodeIterations = n }, nil
			}),
			"allow_percent": typical.UnaryVariadicFunction[env](func(_ ...*Node) (Opt, error) {
				return func(c *DecodeConfig) { c.AllowPercent = true }, nil
			}),
			// Matcher constructors. Each returns one Node, so they nest and
			// type-check at parse time: every child argument must itself
			// evaluate to a *Node.
			"literal": typical.BinaryVariadicFunction[env](func(s string, children ...*Node) (*Node, error) {
				return Literal(s, children...), nil
			}),
			"capture": typical.BinaryVariadicFunction[env](func(name string, children ...*Node) (*Node, error) {
				return Capture(name, children...), nil
			}),
			"glob": typical.UnaryVariadicFunction[env](func(children ...*Node) (*Node, error) {
				return Glob(children...), nil
			}),
			"greedy": typical.UnaryVariadicFunction[env](func(_ ...*Node) (*Node, error) {
				return Greedy(), nil
			}),
			// root is the synthetic top node, the one way to give a tree several
			// first segments. It folds several root paths into one path.match,
			// so the call carries the decode options once instead of repeating
			// them across an || of separate matches. It is valid only as the
			// matcher argument of path.match; validateRoot rejects it elsewhere.
			"root": typical.UnaryVariadicFunction[env](func(children ...*Node) (*Node, error) {
				return Root(children...)
			}),
			// Carve-out constructors. Each folds an exclusion into one matcher
			// tree so a deny needs no separate negated path.match, which keeps
			// the whole rule on one decode policy and avoids the fail-open
			// inversion a negated match carries. glob_without excludes single
			// segment values and continues to children; greedy_without excludes
			// the `<value>/**` subtrees by first segment; greedy_except excludes
			// arbitrary matcher subtrees, with the exclusion's own terminal-ness
			// choosing exact-segment versus whole-subtree.
			"glob_without": typical.BinaryVariadicFunction[env](func(excludes []string, children ...*Node) (*Node, error) {
				return GlobWithout(excludes, children...)
			}),
			"greedy_without": typical.UnaryVariadicFunction[env](func(excludes ...string) (*Node, error) {
				return GreedyWithout(excludes...)
			}),
			"greedy_except": typical.UnaryVariadicFunction[env](func(excludes ...*Node) (*Node, error) {
				return GreedyExcept(excludes...)
			}),
			// Identity helpers, matching the existing predicate language.
			"set": typical.UnaryVariadicFunction[env](func(args ...string) ([]string, error) {
				return args, nil
			}),
			"contains": typical.BinaryFunction[env](func(list []string, item string) (bool, error) {
				return slices.Contains(list, item), nil
			}),
		},
		// vars.<name> reads a capture bound by this evaluation's matcher. Names
		// are dynamic (each rule's {captures} define them), so they cannot be
		// enumerated in Variables. An unbound name reads as the empty string,
		// which forces any comparison to fail, so an unbound capture can only
		// deny, never widen.
		//
		// The namespace is "vars", not "var": the engine parses Go expression
		// syntax, and "var" is a reserved Go keyword the parser rejects as an
		// operand.
		GetUnknownIdentifier: func(e env, fields []string) (any, error) {
			if len(fields) == 2 && fields[0] == "vars" {
				v, ok := e.vars[fields[1]]
				if !ok {
					e.state.unboundRead = true
				}
				return v, nil
			}
			return nil, trace.NotFound("unknown identifier %q", strings.Join(fields, "."))
		},
	})
	return p, trace.Wrap(err)
}

// parsePredicate parses and type-checks a predicate string. Parsing is cached
// per string, so a repeated rule is parsed once.
func parsePredicate(expr string) (predicate, error) {
	p, err := parser.Parse(expr)
	return p, trace.Wrap(err)
}
