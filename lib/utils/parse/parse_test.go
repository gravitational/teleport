/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package parse

import (
	"regexp"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

// TestVariable tests variable parsing
func TestVariable(t *testing.T) {
	t.Parallel()
	traits := map[string][]string{
		"foo":   {"foovalue"},
		"bar":   {"barvalue"},
		"email": {"user@example.com"},
	}
	var tests = []struct {
		title string
		in    string
		err   error
		out   string
	}{
		{
			title: "no curly bracket prefix",
			in:    "external.foo}}",
			err:   trace.BadParameter(""),
		},
		{
			title: "invalid syntax",
			in:    `{{external.foo("bar")`,
			err:   trace.BadParameter(""),
		},
		{
			title: "invalid variable syntax",
			in:    "{{internal.}}",
			err:   trace.BadParameter(""),
		},
		{
			title: "invalid dot syntax",
			in:    "{{external..foo}}",
			err:   trace.BadParameter(""),
		},
		{
			title: "empty variable",
			in:    "{{}}",
			err:   trace.BadParameter(""),
		},
		{
			title: "string variable",
			in:    `{{"asdf"}}`,
			out:   "asdf",
		},
		{
			title: "invalid int variable",
			in:    `{{123}}`,
			err:   trace.BadParameter(""),
		},
		{
			title: "incomplete variables are not allowed",
			in:    `{{internal}}`,
			err:   trace.BadParameter(""),
		},
		{
			title: "no curly bracket suffix",
			in:    "{{internal.foo",
			err:   trace.BadParameter(""),
		},
		{
			title: "too many levels of nesting in the variable",
			in:    "{{internal.foo.bar}}",
			err:   trace.BadParameter(""),
		},
		{
			title: "too many levels of nesting in the variable with property",
			in:    `{{internal.foo["bar"]}}`,
			err:   trace.BadParameter(""),
		},
		{
			title: "regexp function call not allowed",
			in:    `{{regexp.match(".*")}}`,
			err:   trace.BadParameter(""),
		},
		{
			title: "valid with brackets",
			in:    `{{internal["foo"]}}`,
			out:   "foovalue",
		},
		{
			title: "string literal",
			in:    `foo`,
			out:   "foo",
		},
		{
			title: "external with no brackets",
			in:    "{{external.foo}}",
			out:   "foovalue",
		},
		{
			title: "invalid namespaces are allowed",
			in:    "{{foo.bar}}",
			out:   "barvalue",
		},
		{
			title: "internal with no brackets",
			in:    "{{internal.bar}}",
			out:   "barvalue",
		},
		{
			title: "internal with spaces removed",
			in:    "  {{  internal.bar  }}  ",
			out:   "barvalue",
		},
		{
			title: "variable with prefix and suffix",
			in:    "  hello,  {{  internal.bar  }}  there! ",
			out:   "hello,  barvalue  there!",
		},
		{
			title: "variable with local function",
			in:    "{{email.local(internal.email)}}",
			out:   "user",
		},
		{
			title: "regexp replace",
			in:    `{{regexp.replace(internal.foo, "^(.*)value$", "$1")}}`,
			out:   "foo",
		},
		{
			title: "regexp replace with variable expression",
			in:    `{{regexp.replace(internal.foo, internal.bar, "baz")}}`,
			err:   trace.BadParameter(""),
		},
		{
			title: "regexp replace with variable replacement",
			in:    `{{regexp.replace(internal.foo, "bar", internal.baz)}}`,
			err:   trace.BadParameter(""),
		},
		{
			title: "regexp replace constant expression",
			in:    `{{regexp.replace("abc", "c", "z")}}`,
			out:   "abz",
		},
		{
			title: "non existing function",
			in:    `{{regexp.replac("abc", "c", "z")}}`,
			err:   trace.BadParameter(""),
		},
		{
			title: "missing args",
			in:    `{{regexp.replace("abc", "c")}}`,
			err:   trace.BadParameter(""),
		},
		{
			title: "no args",
			in:    `{{regexp.replace()}}`,
			err:   trace.BadParameter(""),
		},
		{
			title: "extra args",
			in:    `{{regexp.replace("abc", "c", "x", "z")}}`,
			err:   trace.BadParameter(""),
		},
		{
			title: "invalid arg type",
			in:    `{{regexp.replace(regexp.match("a"), "c", "x")}}`,
			err:   trace.BadParameter(""),
		},
	}

	for _, tt := range tests {
		t.Run(tt.title, func(t *testing.T) {
			expr, err := NewTraitsTemplateExpression(tt.in)
			if tt.err != nil {
				require.IsType(t, tt.err, err)
				return
			}
			require.NoError(t, err, trace.DebugReport(err))

			result, err := expr.Interpolate(func(namespace, trait string) error {
				return nil
			}, traits)
			require.NoError(t, err, trace.DebugReport(err))

			require.Len(t, result, 1)
			require.Equal(t, tt.out, result[0])
		})
	}
}

// TestInterpolate tests variable interpolation
func TestInterpolate(t *testing.T) {
	t.Parallel()

	errCheckIsNotFound := func(tt require.TestingT, err error, i ...any) {
		require.True(tt, trace.IsNotFound(err), "expected not found error, got %v", err)
	}
	errCheckIsBadParameter := func(tt require.TestingT, err error, i ...any) {
		require.True(tt, trace.IsBadParameter(err), "expected bad parameter error, got %v", err)
	}
	type result struct {
		values   []string
		errCheck require.ErrorAssertionFunc
	}
	var tests = []struct {
		title  string
		in     string
		traits map[string][]string
		res    result
	}{
		{
			title:  "mapped traits",
			in:     "{{external.foo}}",
			traits: map[string][]string{"foo": {"a", "b"}, "bar": {"c"}},
			res:    result{values: []string{"a", "b"}},
		},
		{
			title:  "mapped traits with email.local",
			in:     "{{email.local(external.foo)}}",
			traits: map[string][]string{"foo": {"Alice <alice@example.com>", "bob@example.com"}, "bar": {"c"}},
			res:    result{values: []string{"alice", "bob"}},
		},
		{
			title:  "missed traits",
			in:     "{{external.baz}}",
			traits: map[string][]string{"foo": {"a", "b"}, "bar": {"c"}},
			res:    result{errCheck: errCheckIsNotFound, values: []string{}},
		},
		{
			title:  "traits with prefix and suffix",
			in:     "IAM#{{external.foo}};",
			traits: map[string][]string{"foo": {"a", "b"}, "bar": {"c"}},
			res:    result{values: []string{"IAM#a;", "IAM#b;"}},
		},
		{
			title:  "error in mapping traits",
			in:     "{{email.local(external.foo)}}",
			traits: map[string][]string{"foo": {"Alice <alice"}},
			res:    result{errCheck: errCheckIsBadParameter},
		},
		{
			title:  "literal expression",
			in:     "foo",
			traits: map[string][]string{"foo": {"a", "b"}, "bar": {"c"}},
			res:    result{values: []string{"foo"}},
		},
		{
			title:  "regexp replacement with numeric match",
			in:     `{{regexp.replace(internal.foo, "bar-(.*)", "$1")}}`,
			traits: map[string][]string{"foo": {"bar-baz"}},
			res:    result{values: []string{"baz"}},
		},
		{
			title:  "regexp replacement with named match",
			in:     `{{regexp.replace(internal.foo, "bar-(?P<suffix>.*)", "$suffix")}}`,
			traits: map[string][]string{"foo": {"bar-baz"}},
			res:    result{values: []string{"baz"}},
		},
		{
			title:  "regexp replacement with multiple matches",
			in:     `{{regexp.replace(internal.foo, "foo-(.*)-(.*)", "$1.$2")}}`,
			traits: map[string][]string{"foo": {"foo-bar-baz"}},
			res:    result{values: []string{"bar.baz"}},
		},
		{
			title:  "regexp replacement with no match",
			in:     `{{regexp.replace(internal.foo, "^bar-(.*)$", "$1-matched")}}`,
			traits: map[string][]string{"foo": {"foo-test1", "bar-test2"}},
			res:    result{values: []string{"test2-matched"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.title, func(t *testing.T) {
			expr, err := NewTraitsTemplateExpression(tt.in)
			require.NoError(t, err)
			noVarValidation := func(string, string) error {
				return nil
			}
			values, err := expr.Interpolate(noVarValidation, tt.traits)
			if tt.res.errCheck != nil {
				tt.res.errCheck(t, err)
				require.Empty(t, values)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.res.values, values)
		})
	}
}

// TestVarValidation tests that vars are validated during interpolation.
func TestVarValidation(t *testing.T) {
	t.Parallel()
	var tests = []struct {
		title         string
		in            string
		traits        map[string][]string
		varValidation func(string, string) error
		assertErr     require.ErrorAssertionFunc
	}{
		{
			title:  "no validation",
			in:     "{{external.foo}}",
			traits: map[string][]string{"foo": {"bar"}},
			varValidation: func(namespace, name string) error {
				return nil
			},
			assertErr: require.NoError,
		},
		{
			title:  "validate namespace ok",
			in:     "{{external.foo}}",
			traits: map[string][]string{"foo": {"bar"}},
			varValidation: func(namespace, name string) error {
				if namespace != "external" {
					return trace.BadParameter("")
				}
				return nil
			},
			assertErr: require.NoError,
		},
		{
			title:  "validate namespace error",
			in:     "{{internal.foo}}",
			traits: map[string][]string{"foo": {"bar"}},
			varValidation: func(namespace, name string) error {
				if namespace != "external" {
					return trace.BadParameter("")
				}
				return nil
			},
			assertErr: require.Error,
		},
		{
			title:  "variable found",
			in:     "{{external.foo}}",
			traits: map[string][]string{"foo": {"bar"}},
			varValidation: func(namespace, name string) error {
				return nil
			},
			assertErr: require.NoError,
		},
		{
			title:  "variable not found",
			in:     "{{external.baz}}",
			traits: map[string][]string{"foo": {"bar"}},
			varValidation: func(namespace, name string) error {
				return nil
			},
			assertErr: require.Error,
		},
	}

	for _, tt := range tests {
		t.Run(tt.title, func(t *testing.T) {
			expr, err := NewTraitsTemplateExpression(tt.in)
			require.NoError(t, err)
			_, err = expr.Interpolate(tt.varValidation, tt.traits)
			tt.assertErr(t, err)
		})
	}
}

func TestMatch(t *testing.T) {
	t.Parallel()
	tests := []struct {
		title string
		in    string
		err   error
		out   MatchExpression
	}{
		{
			title: "no curly bracket prefix",
			in:    `regexp.match(".*")}}`,
			err:   trace.BadParameter(""),
		},
		{
			title: "no curly bracket suffix",
			in:    `{{regexp.match(".*")`,
			err:   trace.BadParameter(""),
		},
		{
			title: "unknown function",
			in:    `{{regexp.surprise(".*")}}`,
			err:   trace.BadParameter(""),
		},
		{
			title: "bad regexp",
			in:    `{{regexp.match("+foo")}}`,
			err:   trace.BadParameter(""),
		},
		{
			title: "unknown namespace",
			in:    `{{surprise.match(".*")}}`,
			err:   trace.BadParameter(""),
		},
		{
			title: "not a boolean expression",
			in:    `{{email.local(external.email)}}`,
			err:   trace.BadParameter(""),
		},
		{
			title: "not a boolean variable",
			in:    `{{external.email}}`,
			err:   trace.BadParameter(""),
		},
		{
			title: "string literal",
			in:    `foo`,
			out: MatchExpression{
				matcher: regexpMatcher(`^foo$`),
			},
		},
		{
			title: "wildcard",
			in:    `foo*`,
			out: MatchExpression{
				matcher: regexpMatcher(`^foo(.*)$`),
			},
		},
		{
			title: "raw regexp",
			in:    `^foo.*$`,
			out: MatchExpression{
				matcher: regexpMatcher(`^foo.*$`),
			},
		},
		{
			title: "regexp.match simple call",
			in:    `{{regexp.match("foo")}}`,
			out: MatchExpression{
				matcher: regexpMatcher(`foo`),
			},
		},
		{
			title: "regexp.match call",
			in:    `foo-{{regexp.match("bar")}}-baz`,
			out: MatchExpression{
				prefix:  "foo-",
				matcher: regexpMatcher(`bar`),
				suffix:  "-baz",
			},
		},
		{
			title: "regexp.not_match call",
			in:    `foo-{{regexp.not_match("bar")}}-baz`,
			out: MatchExpression{
				prefix:  "foo-",
				matcher: regexpNotMatcher(`bar`),
				suffix:  "-baz",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.title, func(t *testing.T) {
			matcher, err := NewMatcher(tt.in)
			if tt.err != nil {
				require.IsType(t, tt.err, err, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.out, *matcher)
		})
	}
}

func TestMatchers(t *testing.T) {
	t.Parallel()
	tests := []struct {
		title   string
		matcher string
		in      string
		want    bool
	}{
		{
			title:   "regexp matcher positive",
			matcher: `{{regexp.match("foo")}}`,
			in:      "foo",
			want:    true,
		},
		{
			title:   "regexp matcher negative",
			matcher: `{{regexp.match("bar")}}`,
			in:      "foo",
			want:    false,
		},
		{
			title:   "not matcher",
			matcher: `{{regexp.not_match("bar")}}`,
			in:      "foo",
			want:    true,
		},
		{
			title:   "prefix/suffix matcher positive",
			matcher: `foo-{{regexp.match("bar")}}-baz`,
			in:      "foo-bar-baz",
			want:    true,
		},
		{
			title:   "prefix/suffix matcher negative",
			matcher: `foo-{{regexp.match("bar")}}-baz`,
			in:      "foo-foo-baz",
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.title, func(t *testing.T) {
			matcher, err := NewMatcher(tt.matcher)
			require.NoError(t, err)
			got := matcher.Match(tt.in)
			require.Equal(t, tt.want, got)
		})
	}
}

func regexpMatcher(match string) Matcher {
	return matcher{regexp.MustCompile(match)}
}

func regexpNotMatcher(match string) Matcher {
	return notMatcher{regexp.MustCompile(match)}
}
