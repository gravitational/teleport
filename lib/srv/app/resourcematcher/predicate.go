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

// Option is a per-match setting passed to path.match after the matcher tree,
// such as allow_encoded. By default a match admits no percent encoding at
// all: any encoded char in the path forces the rule to no-match, fail-closed,
// so a negated path.match cannot turn an encoded segment into an allow. An
// option relaxes that default for the one match it sits on, leaving every other
// match strict.
type Option struct {
	// allowEncoded lists the encoded chars this match admits in the path. It is
	// empty unless allow_encoded set it. Today only the separator "/" is
	// supported.
	allowEncoded []string
}

// allowsEncodedSlash reports whether any option opts this match into the
// encoded separator. When false, a path carrying any encoded char fails the
// match closed.
func allowsEncodedSlash(opts []Option) bool {
	for _, opt := range opts {
		for _, c := range opt.allowEncoded {
			if c == "/" {
				return true
			}
		}
	}
	return false
}

// encodedBlocked reports whether the path carries an encoded char that no opt
// admits, in which case a match must fail closed. It is the one gate both
// path.match and Match consult, so the predicate surface and the Go surface
// cannot drift on which paths an encoded char rejects.
func encodedBlocked(tokens []string, opts []Option) bool {
	if allowsEncodedSlash(opts) {
		return false
	}
	for _, tok := range tokens {
		if strings.ContainsRune(tok, '%') {
			return true
		}
	}
	return false
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

// pathTokens lazily tokenizes the request path and caches the result in the
// shared state, so several path.match calls in one evaluation tokenize the path
// once and a rule with no path.match never tokenizes at all. It reports
// ok=false when the path is not tokenizable and records tokenizeFailed, which
// the caller treats as a forced no-match. Returning a flag rather than an error
// keeps a negated path.match from inverting a tokenize-failure into an allow:
// tokenizeFailed overrides the boolean no matter which operator read it.
func (e env) pathTokens() ([]string, bool) {
	s := e.state
	if !s.tokenized {
		s.tokenized = true
		tokens, err := Tokenize(e.request.Path)
		if err != nil {
			s.tokenizeFailed = true
			return nil, false
		}
		s.tokens = tokens
	}
	if s.tokenizeFailed {
		return nil, false
	}
	return s.tokens, true
}

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
	// tokens caches the tokenized request path, filled lazily on the first
	// path.match.
	tokens []string
	// tokenized records that the lazy tokenize has run, so it runs at most once
	// per evaluation even when it produced no tokens.
	tokenized bool
	// tokenizeFailed records that the path could not be tokenized. The caller
	// forces the rule to no-match when this is set, so a path the matcher cannot
	// read fails closed even behind a negation.
	tokenizeFailed bool
	// encodedNotAllowed records that a path.match without the allow_encoded
	// option met a path carrying an encoded char. By default a match admits no
	// encoding, so the caller forces the rule to no-match, which makes a negated
	// path.match fail closed on an encoded segment instead of inverting the miss
	// into an allow.
	encodedNotAllowed bool
	// allowCode and allowReason hold the audit code and reason an allow_code
	// call recorded. allow_code wraps a rule's predicate and commits its code
	// only when the wrapped expression is true, so a non-matching rule records
	// nothing. The rule set keeps the recorded code only on the matching rule,
	// so the committed code is always the one the matching rule set. When
	// several allow_code calls run on the same evaluation, the last one wins.
	allowCode   string
	allowReason string
	// denyHints holds the near-miss hints a deny_hint call recorded. deny_hint
	// wraps an inner condition reached only after the path on its left matched,
	// and commits its hint only when the wrapped expression is false, so it
	// fires exactly when the path and method matched but the inner condition
	// failed. They are read only when the rule denies; on an allow the rule
	// set discards them.
	denyHints []Hint
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
			// true is the constant that unsafe_allow_all lowers to: a rule that
			// allows every request outright. It is a real boolean literal in the
			// language, so the lowered predicate "true" parses and evaluates like
			// any other expression rather than failing as an unknown identifier.
			"true": true,
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
			// Matcher entry point. path.match walks the request path, tokenized
			// lazily, against the matcher root. On a match it records the bound
			// segments into the environment's vars map so a later vars.<name>
			// read can see them, then returns true. A path the tokenizer rejects
			// records tokenizeFailed and returns false, and the caller forces the
			// rule to no-match, so a negated path.match cannot turn an unreadable
			// path into an allow.
			//
			// By default the match admits no percent encoding: when no
			// allow_encoded option is given and the path carries an encoded
			// char, it records encodedNotAllowed and returns false, and the caller
			// forces the rule to no-match. This makes a negated path.match fail
			// closed on an encoded segment rather than inverting the miss into an
			// allow. The allow_encoded option relaxes this for the one match
			// it sits on, where a glob_encoded or capture_encoded node then matches
			// the encoded segment explicitly.
			"path.match": typical.BinaryVariadicFunctionWithEnv(func(e env, root *Node, opts ...Option) (bool, error) {
				tokens, ok := e.pathTokens()
				if !ok {
					return false, nil
				}
				if encodedBlocked(tokens, opts) {
					e.state.encodedNotAllowed = true
					return false, nil
				}
				if matched, caps := Eval(tokens, root); matched {
					for k, v := range caps {
						e.vars[k] = v
					}
					return true, nil
				}
				return false, nil
			}),
			// allow_encoded opts one path.match into admitting the named
			// encoded chars, paired with a glob_encoded or capture_encoded node at
			// each position that carries one. Today only the separator "/" is
			// supported. Without it a match admits no encoding and an encoded path
			// fails closed.
			"allow_encoded": typical.UnaryFunction[env](func(allowed []string) (Option, error) {
				if err := validateEncodedChars(allowed); err != nil {
					return Option{}, trace.Wrap(err)
				}
				return Option{allowEncoded: allowed}, nil
			}),
			// allow_code wraps a rule's predicate. It records the structured audit
			// code, and the human reason, emitted on the allow event when the rule
			// matches, and returns the value of the wrapped expression so the
			// wrapper is transparent to the boolean result. The code is committed
			// only when the wrapped expression is true, so a non-matching rule
			// records nothing, and the rule set keeps the recorded code only on
			// the matching rule. The wrapper sits at the top of the rule, as in
			// allow_code("code", "reason", path.match(...) && where), so both
			// authoring surfaces share one representation: the sugared allow_code
			// and allow_reason fields lower to this call.
			"allow_code": typical.TernaryFunctionWithEnv(func(e env, code, reason string, expr bool) (bool, error) {
				if err := validateCode(code); err != nil {
					return false, trace.Wrap(err)
				}
				if expr {
					e.state.allowCode = code
					e.state.allowReason = reason
				}
				return expr, nil
			}),
			// deny_hint wraps an inner condition reached only after a path on its
			// left matched. It records a near-miss hint, the structured code and
			// human reason emitted on the deny event when the rule's path and
			// method matched but the inner condition failed, and returns the
			// value of the wrapped expression so the wrapper is transparent to
			// the boolean result. The hint is committed only when the wrapped
			// expression is false, so an allowed rule records nothing, and the
			// rule set keeps the hints only when the request is denied. The
			// wrapper sits on an inner condition, gated by the path and method on
			// its left, so it fires exactly on the near-miss: the sugared
			// deny_code_hint and deny_reason_hint fields lower to this call.
			"deny_hint": typical.TernaryFunctionWithEnv(func(e env, code, reason string, expr bool) (bool, error) {
				if err := validateCode(code); err != nil {
					return false, trace.Wrap(err)
				}
				if !expr {
					e.state.denyHints = append(e.state.denyHints, Hint{Code: code, Reason: reason})
				}
				return expr, nil
			}),
			// Matcher constructors. Each returns one Node, so they nest and
			// type-check at parse time: every child argument must itself
			// evaluate to a *Node.
			"literal": typical.BinaryVariadicFunction[env](func(s string, children ...*Node) (*Node, error) {
				if err := validateLiteral(s); err != nil {
					return nil, trace.Wrap(err)
				}
				return Literal(s, children...), nil
			}),
			"capture": typical.BinaryVariadicFunction[env](func(name string, children ...*Node) (*Node, error) {
				return Capture(name, children...), nil
			}),
			"glob": typical.UnaryVariadicFunction[env](func(children ...*Node) (*Node, error) {
				return Glob(children...), nil
			}),
			// Encoded-char constructors, the explicit per-position opt-in for an
			// encoded char. glob and capture are safe-only and reject any percent
			// byte; these admit a segment that is plain or carries only an
			// admitted encoded char (today the separator "/", as set("/")), kept
			// raw and forwarded byte-faithfully. They pair with the
			// allow_encoded option on path.match, which gates the match.
			"glob_encoded": typical.BinaryVariadicFunction[env](func(allowed []string, children ...*Node) (*Node, error) {
				return GlobEncoded(allowed, children...)
			}),
			"capture_encoded": typical.TernaryVariadicFunction[env](func(name string, allowed []string, children ...*Node) (*Node, error) {
				return CaptureEncoded(name, allowed, children...)
			}),
			// encoded_literal matches one segment by its decoded value, so it
			// admits either hex case (%2F or %2f) of an admitted encoded char. The
			// value is the decoded form with the encoded chars written plain, for
			// example encoded_literal("mygroup/myproject", set("/")) to pin a
			// GitLab id. The "/" in the value is content, not a separator: the
			// node is one segment and never splits.
			"encoded_literal": typical.TernaryVariadicFunction[env](func(value string, allowed []string, children ...*Node) (*Node, error) {
				return EncodedLiteral(value, allowed, children...)
			}),
			"greedy": typical.NullaryFunction[env](func() (*Node, error) {
				return Greedy(), nil
			}),
			// slash() matches the empty segment a final "/" produces. It replaces
			// the empty-literal pun, so a literal never carries empty text.
			"slash": typical.NullaryFunction[env](func() (*Node, error) {
				return Slash(), nil
			}),
			// optional() makes a trailing subtree optional: the path may end at
			// this node, or one of the children matches the remainder. So
			// optional(slash()) matches "/foo" and "/foo/" alike, and
			// optional(literal("reports")) matches "/files" and "/files/reports"
			// from one tree with no duplicated prefix. The skip branch binds
			// nothing, so a capture inside an optional is never guaranteed.
			"optional": typical.UnaryVariadicFunction[env](func(children ...*Node) (*Node, error) {
				return Optional(children...)
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
			// String helpers for case-insensitive comparisons in a where clause,
			// such as contains(set("get", "head"), lower(request.method)). They
			// fold ASCII and Unicode case the way strings.ToLower and ToUpper do.
			"lower": typical.UnaryFunction[env](func(s string) (string, error) {
				return strings.ToLower(s), nil
			}),
			"upper": typical.UnaryFunction[env](func(s string) (string, error) {
				return strings.ToUpper(s), nil
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
