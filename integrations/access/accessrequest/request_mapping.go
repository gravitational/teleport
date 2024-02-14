package accessrequest

import (
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/expression"
	"github.com/gravitational/teleport/lib/utils/typical"
	"github.com/gravitational/trace"
)

// mappableRequestSpec holds user details that can be mapped in an
// access request condition assertion.
type mappableRequestSpec struct {
	// e.g access_request.spec.roles.contains('prod-rw') && !access_request.status.notified
	Roles       []string
	Notified    string
	Annotations map[string][]string
}

// evaluationEnv defines mappable attrbutes for predicate expression evaluation context.
type evaluationEnv struct {
	roles       expression.Set
	notified    expression.Set
	annotations expression.Dict
}

func newRequestConditionParser() *typical.Parser[evaluationEnv, any] {
	typicalEnvVar := map[string]typical.Variable{
		"true":  true,
		"false": false,
		"access_request.spec.roles": typical.DynamicVariable[evaluationEnv](func(env evaluationEnv) (expression.Set, error) {
			return env.roles, nil
		}),
		"access_request.status.notified": typical.DynamicVariable[evaluationEnv](func(env evaluationEnv) (expression.Set, error) {
			return env.notified, nil
		}),
		"access_request.spec.system_annotations": typical.DynamicMap[evaluationEnv, expression.Set](func(env evaluationEnv) (expression.Dict, error) {
			return env.annotations, nil
		}),
	}
	// TODO: Replace defaultParserSpec in new traits expression parser with more limited one
	requestConditionParser, err := expression.NewTraitsExpressionParser[evaluationEnv](typicalEnvVar)
	if err != nil {
		panic(trace.Wrap(err, "creating request condition parser (this is a bug)"))
	}
	return requestConditionParser
}

func newEvaluationEnv(mappableRequest mappableRequestSpec) evaluationEnv {
	return evaluationEnv{
		roles:       expression.NewSet(mappableRequest.Roles...),
		notified:    expression.NewSet(mappableRequest.Notified),
		annotations: expression.DictFromStringSliceMap(mappableRequest.Annotations),
	}
}

func evaluateRequest(condition string, req types.AccessRequest) (bool, error) {
	// mrs := mappableRequestSpec{
	// 	Roles: req.GetRoles(),
	// 	Annotations: req.GetSystemAnnotations(),
	// }
	// evalEnv := newEvaluationEnv(mrs)

	return false, nil
}
