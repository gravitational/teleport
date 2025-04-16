/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package expression

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/gravitational/trace"

	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/lib/utils/typical"
)

// RenderTemplate parses the given template and renders a string using the given
// attributes.
func RenderTemplate(tmpl string, attrs *workloadidentityv1pb.Attrs) (string, error) {
	t, err := NewTemplate(tmpl)
	if err != nil {
		return "", err
	}
	return t.Render(attrs)
}

// Template represents a string template with typical/predicate expressions in
// curly braces.
//
// Expressions can refer to fields on the workloadidentityv1pb.Attrs protobuf
// message by their "text" name, and use common functions such as `strings.lower`
// and `regex.replace`.
type Template struct {
	fragments []fragment
}

// NewTemplate parses the given template string and any interpolated expressions.
func NewTemplate(tmpl string) (*Template, error) {
	matches := reInterpolation.FindAllStringIndex(tmpl, -1)

	// If there were no expressions in curly braces, treat the whole template
	// string as a literal.
	if len(matches) == 0 {
		return &Template{
			fragments: []fragment{
				{
					kind: kindLiteral,
					text: tmpl,
				},
			},
		}, nil
	}

	t := &Template{}

	var pos int
	for _, m := range matches {
		start, end := m[0], m[1]

		// Treat everything between the previous expression and this one as a
		// literal value.
		if start != 0 {
			t.fragments = append(t.fragments, fragment{
				kind: kindLiteral,
				text: tmpl[pos:start],
			})
		}

		// Chop off the curly braces.
		text := tmpl[start+2 : end-2]

		// Special case: curly braces only contain spaces.
		if len(strings.TrimSpace(text)) == 0 {
			pos = end
			continue
		}

		// Parse the expression using the typical/predicate library.
		expr, err := templateExpressionParser.Parse(text)
		if err != nil {
			return nil, trace.Wrap(err, "parsing expression [%d:%d]: %s", start+2, end-2, text)
		}
		t.fragments = append(t.fragments, fragment{
			kind:       kindExpression,
			text:       tmpl[start+2 : end-2],
			expression: expr,
		})

		pos = end
	}

	// Catch any literal text after the final expression.
	if pos < len(tmpl) {
		t.fragments = append(t.fragments, fragment{
			kind: kindLiteral,
			text: tmpl[pos:],
		})
	}

	return t, nil
}

// Render a string from the template with the given attributes.
func (t *Template) Render(attrs *workloadidentityv1pb.Attrs) (string, error) {
	b := new(strings.Builder)

	for _, frag := range t.fragments {
		switch frag.kind {
		case kindLiteral:
			_, _ = b.WriteString(frag.text)
		case kindExpression:
			result, err := frag.expression.Evaluate(attrs)
			if err != nil {
				return "", trace.Wrap(err, "evaluating expression: %s", frag.text)
			}

			switch t := result.(type) {
			case string:
				_, _ = b.WriteString(t)
			case bool:
				_, _ = b.WriteString(strconv.FormatBool(t))
			case int:
				_, _ = b.WriteString(strconv.Itoa(t))
			default:
				return "", trace.Errorf("expression did not evaluate to a string: %s", frag.text)
			}
		}
	}

	return b.String(), nil
}

type fragmentKind int

const (
	kindLiteral fragmentKind = iota
	kindExpression
)

type fragment struct {
	kind       fragmentKind
	text       string
	expression typical.Expression[*workloadidentityv1pb.Attrs, any]
}

// reInterpolation matches anything between curly braces that isn't a curly
// brace. The downside of matching interpolations like this is that it means
// users cannot use curly braces in their expressions (e.g. in regex.replace)
// so we should eventually replace this with a real parser which supports
// escape sequences.
var reInterpolation = regexp.MustCompile(`\{\{([^{}]+?)\}\}`)
