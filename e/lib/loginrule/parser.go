package loginrule

import (
	"github.com/gravitational/trace"

	etypical "github.com/gravitational/teleport/e/lib/typical"
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
	external etypical.Dict
}

type loginRuleExpr typical.Expression[evaluationEnv, any]

func parseExpr(input string) (loginRuleExpr, error) {
	expr, err := loginRuleParser.Parse(input)
	return expr, trace.Wrap(err)
}

var loginRuleParser = mustLoginRuleParser()

func mustLoginRuleParser() *typical.Parser[evaluationEnv, any] {

	envVar := map[string]typical.Variable{
		"true":  true,
		"false": false,
		"external": typical.DynamicMap[evaluationEnv, etypical.Set](func(env evaluationEnv) (etypical.Dict, error) {
			return env.external, nil
		}),
	}
	parser, err := etypical.NewTypicalParser[evaluationEnv](envVar)
	if err != nil {
		panic(trace.Wrap(err, "creating login rule parser (this is a bug)"))
	}
	return parser
}
