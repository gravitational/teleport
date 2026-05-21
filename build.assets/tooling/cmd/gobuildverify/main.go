// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

// Command gobuildverify verifies that a Go binary has required build
// settings. See spec.md for details.
package main

import (
	"debug/buildinfo"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
)

var usageText = `usage: gobuildverify [-h] <binary> <expressions...>

Verify that a Go binary has required build settings.

Each expression has the form A[=B] where:
  A          Check that setting A is present
  A=B        Check that setting A has exact value B
  A=/B/      Check that setting A matches regexp B
  A=(B)      Check that setting A (comma-separated list) contains
             an element matching B, where B uses the rules above

A -- argument terminates parsing of flags. All subsequent arguments are
treated as non-flags, allowing them to start with a hyphen.`

func main() {
	binaryPath, exprs := parseArgs(os.Args)

	info, err := buildinfo.ReadFile(binaryPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading build info from %s: %v\n", binaryPath, err)
		os.Exit(3)
	}

	errs := verify(info, exprs)
	for _, e := range errs {
		fmt.Fprintln(os.Stderr, e)
	}
	if len(errs) > 0 {
		os.Exit(1)
	}
}

func parseArgs(clargs []string) (string, []string) {
	flag.CommandLine.Init(clargs[0], flag.ContinueOnError)
	flag.CommandLine.SetOutput(io.Discard)
	if err := flag.CommandLine.Parse(clargs[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fmt.Println(usageText)
			os.Exit(0)
		}
		fmt.Fprintln(os.Stderr, err)
		fmt.Fprintln(os.Stderr, usageText)
		os.Exit(2)
	}

	args := flag.Args()
	if len(args) < 2 {
		fmt.Fprintln(os.Stderr, usageText)
		os.Exit(2)
	}
	return args[0], args[1:]
}

// verify parses the expression strings and checks them against the build
// info, returning an error for each expression that does not match.
func verify(info *buildinfo.BuildInfo, exprStrs []string) []error {
	settings := make(map[string]string)
	for _, s := range info.Settings {
		settings[s.Key] = s.Value
	}

	var errs []error
	for _, s := range exprStrs {
		expr, err := parseExpr(s)
		if err != nil {
			errs = append(errs, err)
			continue
		}

		value, ok := settings[expr.key]
		if !ok {
			errs = append(errs, &matchError{key: expr.key})
			continue
		}
		if expr.matcher != nil && !expr.matcher.match(value) {
			err := &matchError{key: expr.key, actual: value, matcher: expr.matcher}
			errs = append(errs, err)
		}
	}
	return errs
}

// matchError is returned when a build setting does not satisfy an expression.
type matchError struct {
	key     string
	actual  string
	matcher matcher
}

func (e *matchError) Error() string {
	switch m := e.matcher.(type) {
	case nil:
		return fmt.Sprintf("Build setting not present: %s", e.key)
	case *listMatcher:
		return fmt.Sprintf("Build setting does not contain value: %s: %s not in %s", e.key, m.inner, e.actual)
	default:
		return fmt.Sprintf("Build setting value does not match: %s: %s != %s", e.key, e.actual, e.matcher)
	}
}

// expression represents a parsed check expression.
type expression struct {
	key     string
	matcher matcher // nil means presence check only
}

// matcher defines how a build setting value is checked.
type matcher interface {
	match(value string) bool
	String() string
}

type exactMatcher struct {
	want string
}

func (m *exactMatcher) match(value string) bool {
	return value == m.want
}

func (m *exactMatcher) String() string {
	return m.want
}

type regexpMatcher struct {
	re   *regexp.Regexp
	expr string
}

func (m *regexpMatcher) match(value string) bool {
	return m.re.MatchString(value)
}

func (m *regexpMatcher) String() string {
	return "/" + m.expr + "/"
}

type listMatcher struct {
	inner matcher
}

func (m *listMatcher) match(value string) bool {
	for _, elem := range strings.Split(value, ",") {
		if m.inner.match(elem) {
			return true
		}
	}
	return false
}

func (m *listMatcher) String() string {
	return "(" + m.inner.String() + ")"
}

// parseExpr parses an expression string of the form "A" or "A=B".
func parseExpr(s string) (expression, error) {
	key, value, hasValue := strings.Cut(s, "=")
	if key == "" {
		return expression{}, fmt.Errorf("no setting name in expression: %q", s)
	}
	if !hasValue {
		return expression{key: key}, nil
	}
	m, err := parseMatch(value)
	if err != nil {
		return expression{}, err
	}
	return expression{key: key, matcher: m}, nil
}

// parseMatch parses a match value: a bare string, /regexp/, or (list).
func parseMatch(s string) (matcher, error) {
	if strings.HasPrefix(s, "/") && strings.HasSuffix(s, "/") && len(s) > 1 {
		pattern := s[1 : len(s)-1]
		re, err := regexp.Compile(pattern)
		if err != nil {
			return nil, fmt.Errorf("invalid regexp %s: %w", s, err)
		}
		return &regexpMatcher{re: re, expr: pattern}, nil
	}
	if strings.HasPrefix(s, "(") && strings.HasSuffix(s, ")") && len(s) > 1 {
		inner, err := parseMatch(s[1 : len(s)-1])
		if err != nil {
			return nil, err
		}
		return &listMatcher{inner: inner}, nil
	}
	return &exactMatcher{want: s}, nil
}
