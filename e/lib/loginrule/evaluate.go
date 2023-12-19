package loginrule

import (
	"context"
	"errors"
	"slices"
	"strings"

	"github.com/gravitational/trace"

	loginrulepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/loginrule/v1"
	"github.com/gravitational/teleport/api/types/wrappers"
	"github.com/gravitational/teleport/e/lib/loginrule/storage"
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
	traits := dictFromStringSliceMap(input.Traits)
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
			traits, err = evaluateTraitsMap(env, rule.TraitsMap)
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
		Traits:       stringSliceMapFromDict(traits),
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

func evaluateTraitsMap(env evaluationEnv, traitsMap map[string]*wrappers.StringValues) (dict, error) {
	d, err := newDict()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for key, values := range traitsMap {
		for _, expr := range values.Values {
			e, err := parseExpr(expr)
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
						d[key] = union(d[key], newSet(id))
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

func traitsMapResultToSet(result any, expr string) (set, error) {
	switch v := result.(type) {
	case string:
		return newSet(v), nil
	case set:
		return v, nil
	default:
		return nil, trace.BadParameter("traits_map expression must evaluate to type string or set, the following expression evaluates to %T: %q", result, expr)
	}
}

func evaluateTraitsExpression(env evaluationEnv, traitsExpression string) (dict, error) {
	expr, err := parseExpr(traitsExpression)
	if err != nil {
		return nil, trace.Wrap(err, "error parsing expression: %q", traitsExpression)
	}
	result, err := expr.Evaluate(env)
	if err != nil {
		return nil, trace.Wrap(err, "error evaluating expression: %q", traitsExpression)
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
