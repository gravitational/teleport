package saml

import (
	"github.com/crewjam/saml"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/e/lib/expression"
	"github.com/gravitational/teleport/lib/utils/typical"
)

// samlMappableUserSpec holds user details that can be mapped in
// SAML assertion.
type samlMappableUserSpec struct {
	Username string
	Roles    []string
	Traits   map[string][]string
}

// evaluationEnv defines mappable attrbutes for predicate expression evaluation context.
type evaluationEnv struct {
	userTraits expression.Dict
	userRoles  expression.Set
	username   expression.Set
}

var attributeMappingParser = newAttributeMappingParser()

func newAttributeMappingParser() *typical.Parser[evaluationEnv, any] {
	typicalEnvVar := map[string]typical.Variable{
		"uid": typical.DynamicVariable[evaluationEnv](func(env evaluationEnv) (expression.Set, error) {
			return env.username, nil
		}),
		"user.metadata.name": typical.DynamicVariable[evaluationEnv](func(env evaluationEnv) (expression.Set, error) {
			return env.username, nil
		}),
		"eduPersonAffiliation": typical.DynamicVariable[evaluationEnv](func(env evaluationEnv) (expression.Set, error) {
			return env.userRoles, nil
		}),
		"user.spec.roles": typical.DynamicVariable[evaluationEnv](func(env evaluationEnv) (expression.Set, error) {
			return env.userRoles, nil
		}),
		"user.spec.traits": typical.DynamicMap[evaluationEnv, expression.Set](func(env evaluationEnv) (expression.Dict, error) {
			return env.userTraits, nil
		}),
	}

	attributeParser, err := expression.NewTraitsExpressionParser[evaluationEnv](typicalEnvVar)
	if err != nil {
		panic(trace.Wrap(err, "creating attribute mapping parser (this is a bug)"))
	}

	return attributeParser
}

func evaluateAttributes(requestedAttributes []saml.RequestedAttribute, mappableAttributes samlMappableUserSpec) ([]saml.Attribute, error) {
	var attributes []saml.Attribute
	for _, reqAttrs := range requestedAttributes {
		traitsMap := map[string][]string{
			// there will only be one expression value available.
			reqAttrs.Name: {reqAttrs.Values[0].Value},
		}
		evalEnv := newEvaluationEnv(mappableAttributes)
		result, err := expression.EvaluateTraitsMap(
			evalEnv,
			traitsMap,
			func(input string) (typical.Expression[evaluationEnv, any], error) {
				expr, err := attributeMappingParser.Parse(input)
				return expr, trace.Wrap(err)
			})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		pv := stringSliceFromDict(result)
		attributes = addAttributeWithFormat(attributes, reqAttrs.Name, reqAttrs.Name, reqAttrs.NameFormat, pv...)
	}
	return attributes, nil
}

func newEvaluationEnv(mappableAttributes samlMappableUserSpec) evaluationEnv {
	return evaluationEnv{
		userTraits: expression.DictFromStringSliceMap(mappableAttributes.Traits),
		userRoles:  expression.NewSet(mappableAttributes.Roles...),
		username:   expression.NewSet(mappableAttributes.Username),
	}
}

func stringSliceFromDict(d expression.Dict) []string {
	ssm := expression.StringSliceMapFromDict(d)
	m := make([]string, 0, len(ssm))
	for _, s := range ssm {
		m = append(m, s...)
	}
	return m
}
