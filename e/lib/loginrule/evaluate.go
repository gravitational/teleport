package loginrule

import (
	"context"
	"slices"
	"strings"

	"github.com/gravitational/trace"

	loginrulepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/loginrule/v1"
	"github.com/gravitational/teleport/api/types/wrappers"
	"github.com/gravitational/teleport/e/lib/loginrule/storage"
	etypical "github.com/gravitational/teleport/e/lib/typical"
	oss "github.com/gravitational/teleport/lib/loginrule"
	"github.com/gravitational/teleport/lib/utils/typical"
)

// Evaluator can be used to evaluate login rules for given inputs.
type Evaluator struct {
	storage *storage.S
}

// NewEvaluator returns a new Evaluator which will fetch login rules from the
// given [storage.S]
func NewEvaluator(storage *storage.S) *Evaluator {
	return &Evaluator{
		storage: storage,
	}
}

// Evaluate fetches all login rules currently present in the backend and
// evaluates them with the given input, returning the output or any error
// encountered.
func (e *Evaluator) Evaluate(ctx context.Context, input *oss.EvaluationInput) (*oss.EvaluationOutput, error) {
	allRules, nextPageToken, err := e.storage.ListLoginRules(ctx, 0 /*pageSize*/, "")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for nextPageToken != "" {
		var rules []*loginrulepb.LoginRule
		rules, nextPageToken, err = e.storage.ListLoginRules(ctx, 0 /*pageSize*/, nextPageToken)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		allRules = append(allRules, rules...)
	}

	output, err := Evaluate(allRules, input)
	return output, trace.Wrap(err)
}

// Evaluate evaluates a list of login rules with the given inputs.
func Evaluate(rules []*loginrulepb.LoginRule, input *oss.EvaluationInput) (*oss.EvaluationOutput, error) {
	if len(rules) == 0 {
		// If there are no rules, return the input traits unmodified.
		return &oss.EvaluationOutput{
			Traits: input.Traits,
		}, nil
	}
	sortLoginRules(rules)

	appliedRules := make([]string, 0, len(rules))
	traits := etypical.DictFromStringSliceMap(input.Traits)
	for _, rule := range rules {
		appliedRules = append(appliedRules, rule.Metadata.Name)
		// Every rule gets the output of the previous rule as input.
		env := evaluationEnv{
			external: traits,
		}
		// Each rule should only have one of TraitsMap or TraitsExpression set,
		// this should be checked when the rule is parsed from a file or from
		// storage, no need to check again here.
		if len(rule.TraitsMap) > 0 {
			var err error
			traits, err = etypical.EvaluateTraitsMap(
				env,
				wrapperStringValuesMapToStringSliceMap(rule.TraitsMap),
				func(input string) (typical.Expression[evaluationEnv, any], error) {
					expr, err := loginRuleParser.Parse(input)
					return expr, trace.Wrap(err)
				},
			)
			if err != nil {
				return nil, trace.Wrap(err)
			}
		}
		if len(rule.TraitsExpression) > 0 {
			var err error
			traits, err = evaluateTraitsExpression(env, rule.TraitsExpression)
			if err != nil {
				return nil, trace.Wrap(err)
			}
		}
	}
	return &oss.EvaluationOutput{
		Traits:       etypical.StringSliceMapFromDict(traits),
		AppliedRules: appliedRules,
	}, nil
}

// sortLoginRules sorts a slice of login rules in increasing order of Priority,
// with ties broken by sorting in increasing string order by Name.
func sortLoginRules(rules []*loginrulepb.LoginRule) {
	slices.SortFunc(rules, func(a, b *loginrulepb.LoginRule) int {
		if a.Priority != b.Priority {
			return int(a.Priority - b.Priority)
		}
		return strings.Compare(a.Metadata.Name, b.Metadata.Name)
	})
}

func evaluateTraitsExpression(env evaluationEnv, traitsExpression string) (etypical.Dict, error) {
	expr, err := parseExpr(traitsExpression)
	if err != nil {
		return nil, trace.Wrap(err, "error parsing expression: %q", traitsExpression)
	}
	result, err := expr.Evaluate(env)
	if err != nil {
		return nil, trace.Wrap(err, "error evaluating expression: %q", traitsExpression)
	}
	d, ok := result.(etypical.Dict)
	if !ok {
		return nil, trace.BadParameter("traits_expression must evaluate to type dict, the following expression evaluates to %T: %q", result, traitsExpression)
	}
	return d, nil
}

func wrapperStringValuesMapToStringSliceMap(ws map[string]*wrappers.StringValues) map[string][]string {
	s := make(map[string][]string, len(ws))
	for k, v := range ws {
		s[k] = append(s[k], v.Values...)
	}
	return s
}
