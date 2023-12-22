package expression

import (
	"errors"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/utils/typical"
)

// EvaluateTraitsMap evaluates expression that must evaluate to either string or Set.
// traitsMap: key is name of the trait and values are list of predicate expressions.
func EvaluateTraitsMap[TEnv any](env TEnv, traitsMap map[string][]string, parseExpression func(input string) (typical.Expression[TEnv, any], error)) (Dict, error) {
	d, err := NewDict()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for key, values := range traitsMap {
		for _, expr := range values {
			e, err := parseExpression(expr)
			if err != nil {
				var u typical.UnknownIdentifierError
				if errors.As(err, &u) {
					id := u.Identifier()
					if id == expr {
						// If the entire expression evaluates to a single unknown
						// identifier, treat it as a string. This is to support rules like
						//   groups: [devs]
						// instead of requiring extra quotes like
						//   groups: ['"devs"']
						d[key] = union(d[key], NewSet(id))
						continue
					}
				}
				return nil, trace.Wrap(err, "error parsing expression: %q", expr)
			}

			result, err := e.Evaluate(env)
			if err != nil {
				return nil, trace.Wrap(err, "error evaluating expression: %q", expr)
			}

			s, err := traitsMapResultToSet(result, expr)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			d[key] = union(d[key], s)
		}
	}
	return d, nil
}
