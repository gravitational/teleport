package filter

import (
	"strings"

	"github.com/gravitational/trace"
	"github.com/scim2/filter-parser/v2"
)

// ParseFilter parses a SCIM filter expression. This implementation is
// specifically limited to what is needed for Okta support, which is a single
// `eq` expression.
func ParseFilter(text string) (filter.Expression, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, nil
	}

	f, err := filter.ParseFilter([]byte(text))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := validateFilter(f); err != nil {
		return nil, trace.Wrap(err)
	}

	return f, nil
}

func validateFilter(exp filter.Expression) error {
	switch exp := exp.(type) {
	case *filter.ValuePath:
		if err := validateAttributePath(exp.AttributePath); err != nil {
			return trace.Wrap(err)
		}
		return nil

	case *filter.AttributeExpression:
		if err := validateAttributePath(exp.AttributePath); err != nil {
			return trace.Wrap(err)
		}
		if exp.Operator != filter.EQ {
			return trace.BadParameter("unsupported operator %q", exp.Operator)
		}
		return nil

	case *filter.LogicalExpression:
		return trace.BadParameter("logical operations are unsupported: %q", exp.Operator)

	case *filter.NotExpression:
		return trace.BadParameter("logical negation is unsupported")

	default:
		return trace.BadParameter("invalid expression element %q", exp)
	}
}

func validateAttributePath(attr filter.AttributePath) error {
	if attr.SubAttribute != nil {
		return trace.BadParameter("attribute paths are not supported: %v", attr)
	}

	return nil
}

// EvaluateFilter evaluates a SCIM filter expression against a set of
// attributes.
func EvaluateFilter(exp filter.Expression, attribs map[string]string) error {
	if exp == nil {
		return nil
	}

	switch exp := exp.(type) {
	case *filter.AttributeExpression:
		if exp.Operator != filter.EQ {
			return trace.BadParameter("unsupported operator %q", exp.Operator)
		}

		value, present := attribs[exp.AttributePath.AttributeName]
		if !present {
			return trace.BadParameter("no such attribute: %s", exp.AttributePath.AttributeName)
		}

		if value == exp.CompareValue {
			return nil
		}
	}

	return trace.NotFound("")
}
