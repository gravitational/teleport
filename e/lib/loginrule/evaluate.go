package loginrule

import (
	"sort"

	"github.com/gravitational/trace"
	"github.com/vulcand/predicate"

	loginrulepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/loginrule/v1"
	"github.com/gravitational/teleport/api/types/wrappers"
)

// EvaluationInput holds the inputs to a login rule evaluation.
type EvaluationInput struct {
	// Traits should be set to the external IDP-provided traits which will be
	// input to the login rule evaluation.
	Traits map[string][]string
}

// EvaluationOutput holds the output of a login rule evaluation.
type EvaluationOutput struct {
	// Traits holds the final output traits.
	Traits map[string][]string
}

// Evaluate evaluates a list of login rules with the given inputs.
func Evaluate(rules []*loginrulepb.LoginRule, input *EvaluationInput) (*EvaluationOutput, error) {
	if len(rules) == 0 {
		// If there are no rules, return the input traits unmodified.
		return &EvaluationOutput{
			Traits: input.Traits,
		}, nil
	}
	sortLoginRules(rules)

	traits := dictFromStringSliceMap(input.Traits)
	for _, rule := range rules {
		// Every rule gets the output of the previous rule as input.
		env := &parseEnv{
			external: traits,
		}
		// Parsers are cheap to create, make a new one with the new env rather
		// than modifying the env on an existing parser.
		parser, err := newParser(env)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		// Each rule should only have one of TraitsMap or TraitsExpression set,
		// this should be checked when the rule is parsed from a file or from
		// storage, no need to check again here.
		if len(rule.TraitsMap) > 0 {
			traits, err = evaluateTraitsMap(parser, rule.TraitsMap)
			if err != nil {
				return nil, trace.Wrap(err)
			}
		}
		if len(rule.TraitsExpression) > 0 {
			traits, err = evaluateTraitsExpression(parser, rule.TraitsExpression)
			if err != nil {
				return nil, trace.Wrap(err)
			}
		}
	}
	return &EvaluationOutput{
		Traits: stringSliceMapFromDict(traits),
	}, nil
}

// sortLoginRules sorts a slice of login rules in increasing order of Priority,
// with ties broken by sorting in increasing string order by Name.
func sortLoginRules(rules []*loginrulepb.LoginRule) {
	sort.Slice(rules, func(i, j int) bool {
		if rules[i].Priority != rules[j].Priority {
			return rules[i].Priority < rules[j].Priority
		}
		return rules[i].Metadata.Name < rules[j].Metadata.Name
	})
}

func evaluateTraitsMap(p predicate.Parser, traitsMap map[string]*wrappers.StringValues) (dict, error) {
	d, err := newDict()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for key, values := range traitsMap {
		for _, expr := range values.Values {
			result, err := p.Parse(expr)
			if err != nil {
				return nil, trace.Wrap(err, "error parsing expression: %q", expr)
			}

			s, err := traitsMapResultToSet(result, expr)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			d[key], err = union(d[key], s)
			if err != nil {
				return nil, trace.Wrap(err)
			}
		}
	}
	return d, nil
}

func traitsMapResultToSet(result any, expr string) (set, error) {
	switch v := result.(type) {
	case unknownIdentifier:
		// If the entire expression evaluates to a single unknown
		// identifier, treat it as a string. This is to support rules like
		//   groups: [devs]
		// instead of requiring extra quotes like
		//   groups: ['"devs"']
		return newSet(string(v)), nil
	case string:
		return newSet(v), nil
	case set:
		return v, nil
	default:
		return nil, trace.BadParameter("traits_map expression must evaluate to type string or set, the following expression evaluates to %T: %q", result, expr)
	}
}

func evaluateTraitsExpression(p predicate.Parser, traitsExpression string) (dict, error) {
	result, err := p.Parse(traitsExpression)
	if err != nil {
		return nil, trace.Wrap(err, "error parsing expression: %q", traitsExpression)
	}
	d, ok := result.(dict)
	if !ok {
		return nil, trace.BadParameter("traits_expression must evaluate to type dict, the following expression evaluates to %T: %q", result, traitsExpression)
	}
	return d, nil
}

func stringSliceMapFromDict(d dict) map[string][]string {
	m := make(map[string][]string, len(d))
	for key, s := range d {
		m[key] = s.items()
	}
	return m
}

func dictFromStringSliceMap(m map[string][]string) dict {
	d := make(dict, len(m))
	for key, values := range m {
		d[key] = newSet(values...)
	}
	return d
}
