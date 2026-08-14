package loginrule

import (
	"encoding/json"
	"maps"

	"github.com/gravitational/trace"
	"github.com/ohler55/ojg/jp"

	"github.com/gravitational/teleport/lib/expression"
	"github.com/gravitational/teleport/lib/utils/typical"
)

// evaluationEnv holds the "environment" including all identifiers which will be
// available to predicate expressions.
type evaluationEnv struct {
	// external holds the input traits which are referred to by the name
	// "external", in keeping with the syntax from role templates. For the
	// lowest priority login rule these will be the external traits coming from
	// the identity provider, for subsequent login rules this should be set to
	// the output traits of the previous login rule.
	external expression.Dict
	// claims holds the original, unparsed provider claims. Each claim may be
	// a standard string/list, or an arbitrary json object. These claims are
	// currently only used with the jsonpath expression function and cannot be
	// referenced directly.
	claims map[string]any
}

type loginRuleExpr typical.Expression[evaluationEnv, any]

func parseExpr(input string) (loginRuleExpr, error) {
	expr, err := loginRuleParser.Parse(input)
	return expr, trace.Wrap(err)
}

var loginRuleParser = mustLoginRuleParser()

func mustLoginRuleParser() *typical.Parser[evaluationEnv, any] {
	spec := expression.DefaultParserSpec[evaluationEnv]()
	spec.Variables = map[string]typical.Variable{
		"true":  true,
		"false": false,
		"external": typical.DynamicMap(func(env evaluationEnv) (expression.Dict, error) {
			return env.external, nil
		}),
	}

	// Add jsonpath as an additional function beyond the default parser.
	maps.Copy(spec.Functions, map[string]typical.Function{
		"jsonpath": typical.UnaryFunctionWithEnv(
			func(env evaluationEnv, path string) (expression.Set, error) {
				result, err := JSONPath(env.claims, path)
				if err != nil {
					return expression.Set{}, trace.Wrap(err)
				}
				return expression.NewSet(result...), nil
			}),
	})

	parser, err := typical.NewParser[evaluationEnv, any](spec)
	if err != nil {
		panic(trace.Wrap(err, "creating login rule parser (this is a bug)"))
	}
	return parser
}

// JSONPath takes an unmarshaled json blob and uses the provided jsonpath query on it.
// If the query results in a list of strings, it will be returned. If the query results
// are empty, an empty list is returned. Any other query results will result in an error.
func JSONPath(input map[string]any, path string) ([]string, error) {
	jpExpr, err := jp.ParseString(path)
	if err != nil {
		return nil, trace.Wrap(err, "failed to parse jsonpath path argument")
	}

	var out []string
	results := jpExpr.Get(input)
	for _, result := range results {
		switch r := result.(type) {
		case string:
			out = append(out, r)
		default:
			// Attempt to marshal the results to provide a more descriptive error message.
			resultsJSON, err := json.Marshal(results)
			if err != nil {
				return nil, trace.BadParameter("jsonpath interpolation must result in a string or list of strings, but resulted in type %T", results)
			}
			return nil, trace.BadParameter("jsonpath interpolation must result in a string or list of strings, but resulted in %v", string(resultsJSON))
		}
	}

	return out, nil
}
