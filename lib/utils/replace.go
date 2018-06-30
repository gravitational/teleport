package utils

import (
	"regexp"
	"strings"

	"github.com/gravitational/trace"
)

// ContainsExpansion returns true if value contains
// expansion syntax, e.g. $1 or ${10}
func ContainsExpansion(val string) bool {
	return reExpansion.FindAllStringIndex(val, -1) != nil
}

// ReplaceGlobWildcard replaces glob-style standalone wildcard values
// with real .* regexp-friendly values, does not modify regexp-compatible values
func ReplaceGlobWildcard(in string) string {
	return replaceWildcard.ReplaceAllString(in, "$1.*")
}

// ReplaceRegexp replaces value in string, accepts regular expression and simplified
// wildcard syntax, it has several important differeneces with standard lib
// regexp replacer:
// * Wildcard globs '*' are treated as regular expression .* expressions
// * Full match is expected, partial replacements ignored
// * If there is no match, returns not found error
func ReplaceRegexp(expression string, replaceWith string, input string) (string, error) {
	// replace glob-style wildcards with regexp wildcards
	converted := ReplaceGlobWildcard(expression)
	// treat string as full match
	if !strings.HasPrefix(converted, "^") && !strings.HasPrefix(converted, `\A`) {
		converted = `^` + converted
	}
	if !strings.HasSuffix(converted, "$") && !strings.HasSuffix(converted, `\z`) {
		converted = converted + `$`
	}
	expr, err := regexp.Compile(converted)
	if err != nil {
		return "", trace.BadParameter(err.Error())
	}
	// if there is no match, return NotFound error
	index := expr.FindAllStringIndex(input, -1)
	if len(index) == 0 {
		return "", trace.NotFound("no match found")
	}
	return expr.ReplaceAllString(input, replaceWith), nil
}

var replaceWildcard = regexp.MustCompile(`([^\.]|\A)(\*)`)
var reExpansion = regexp.MustCompile(`\$[^\$]+`)
