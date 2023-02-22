package loginrule

import (
	"github.com/gravitational/trace"
	"golang.org/x/exp/maps"
)

type set map[string]struct{}

func newSet(values ...string) set {
	s := make(set, len(values))
	for _, value := range values {
		s[value] = struct{}{}
	}
	return s
}

func (s set) add(values ...string) set {
	out := s.clone()
	for _, value := range values {
		out[value] = struct{}{}
	}
	return out
}

func (s set) contains(str string) bool {
	_, ok := s[str]
	return ok
}

func (s set) remove(values ...string) any {
	out := s.clone()
	for _, value := range values {
		delete(out, value)
	}
	return out
}

func (s set) transform(f func(string) string) set {
	out := make(set, len(s))
	for str := range s {
		out[f(str)] = struct{}{}
	}
	return out
}

func (s set) clone() set {
	return maps.Clone(s)
}

func (s set) items() []string {
	return maps.Keys(s)
}

func union(sets ...set) set {
	result := make(set)
	for _, s := range sets {
		for v := range s {
			result[v] = struct{}{}
		}
	}
	return result
}

func buildNewSetExpr(items ...any) (expr, error) {
	itemExprs, err := validateExprs[string](items...)
	if err != nil {
		return nil, trace.Wrap(err, "failed to parse argument to set constructor")
	}
	return func(env *evaluationEnv) (any, error) {
		items, err := validateExprResults[string](env, itemExprs...)
		if err != nil {
			return nil, trace.Wrap(err, "failed to evaluate argument to set constructor")
		}
		return newSet(items...), nil
	}, nil
}

func buildSetAddExpr(recv expr, items ...any) (expr, error) {
	itemExprs, err := validateExprs[string](items...)
	if err != nil {
		return nil, trace.Wrap(err, "failed to parse arguments to set.add method")
	}
	return func(env *evaluationEnv) (any, error) {
		s, err := validateExprResult[set](env, recv)
		if err != nil {
			return nil, trace.Wrap(err, "failed to evaluate receiver for set.add method")
		}
		items, err := validateExprResults[string](env, itemExprs...)
		if err != nil {
			return nil, trace.Wrap(err, "failed to evaluate argument to set.add method")
		}
		return s.add(items...), nil
	}, nil
}

func buildSetContainsExpr(recv expr, arg any) (expr, error) {
	argExpr, err := validateExpr[string](arg)
	if err != nil {
		return nil, trace.Wrap(err, "failed to parse argument to set.contains method")
	}
	return func(env *evaluationEnv) (any, error) {
		s, err := validateExprResult[set](env, recv)
		if err != nil {
			return nil, trace.Wrap(err, "failed to evaluate receiver for set.contains method")
		}
		arg, err := validateExprResult[string](env, argExpr)
		if err != nil {
			return nil, trace.Wrap(err, "failed to evaluate argument to set.contains method")
		}
		return s.contains(arg), nil
	}, nil
}

func buildUnionExpr(sets ...any) (expr, error) {
	setExprs, err := validateExprs[set](sets...)
	if err != nil {
		return nil, trace.Wrap(err, "failed to parse argument to union")
	}
	return func(env *evaluationEnv) (any, error) {
		sets, err := validateExprResults[set](env, setExprs...)
		if err != nil {
			return nil, trace.Wrap(err, "failed to evaluate argument to union")
		}
		return union(sets...), nil
	}, nil
}
