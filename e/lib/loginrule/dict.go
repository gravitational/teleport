package loginrule

import "github.com/gravitational/trace"

type dict map[string]set

// newDict returns a dict initialized with the key-value pairs as specified in
// [pairs].
func newDict(pairs ...pair) (dict, error) {
	d := make(dict, len(pairs))
	for _, p := range pairs {
		k, ok := p.first.(string)
		if !ok {
			return nil, trace.BadParameter("dict keys must have type string, got %T", p.first)
		}
		v, ok := p.second.(set)
		if !ok {
			return nil, trace.BadParameter("dict values must have type set, got %T", p.second)
		}
		d[k] = v
	}
	return d, nil
}

func (d dict) addValues(key string, values ...string) dict {
	out := d.clone()
	s := out[key]
	if s == nil {
		out[key] = newSet(values...)
		return out
	}
	// Calling set.add would do an unnecessary extra copy, add the values
	// "manually".
	for _, value := range values {
		s[value] = struct{}{}
	}
	return out
}

func (d dict) put(key string, value set) dict {
	out := d.clone()
	out[key] = value
	return out
}

func (d dict) remove(keys ...string) any {
	out := d.clone()
	for _, key := range keys {
		delete(out, key)
	}
	return out
}

func (d dict) clone() dict {
	out := make(dict, len(d))
	for key, set := range d {
		out[key] = set.clone()
	}
	return out
}

type pair struct {
	first, second any
}

func buildNewDictExpr(pairs ...any) (expr, error) {
	pairExprs, err := validateExprs[pair](pairs...)
	if err != nil {
		return nil, trace.Wrap(err, "failed to parse argument to dict constructor")
	}
	return func(env *evaluationEnv) (any, error) {
		pairs, err := validateExprResults[pair](env, pairExprs...)
		if err != nil {
			return nil, trace.Wrap(err, "failed to evaluate argument to dict constructor")
		}
		return newDict(pairs...)
	}, nil
}

func buildDictAddValuesExpr(recv expr, key any, values ...any) (expr, error) {
	keyExpr, err := validateExpr[string](key)
	if err != nil {
		return nil, trace.Wrap(err, "failed to parse first argument (key) to dict.add_values method")
	}
	valuesExprs, err := validateExprs[string](values...)
	if err != nil {
		return nil, trace.Wrap(err, "failed to parse variadic argument (values) to dict.add_values method")
	}
	return func(env *evaluationEnv) (any, error) {
		d, err := validateExprResult[dict](env, recv)
		if err != nil {
			return nil, trace.Wrap(err, "failed to evaluate receiver for dict.add_values method")
		}
		key, err := validateExprResult[string](env, keyExpr)
		if err != nil {
			return nil, trace.Wrap(err, "failed to evaluate first argument (key) to dict.add_values method")
		}
		values, err := validateExprResults[string](env, valuesExprs...)
		if err != nil {
			return nil, trace.Wrap(err, "failed to evaluate variadic argument (values) to dict.add_values method")
		}
		return d.addValues(key, values...), nil
	}, nil
}

func buildDictPutExpr(recv expr, key any, value any) (expr, error) {
	keyExpr, err := validateExpr[string](key)
	if err != nil {
		return nil, trace.Wrap(err, "failed to parse first argument (key) to dict.put method")
	}
	valueExpr, err := validateExpr[set](value)
	if err != nil {
		return nil, trace.Wrap(err, "failed to parse second argument (value) to dict.put method")
	}
	return func(env *evaluationEnv) (any, error) {
		d, err := validateExprResult[dict](env, recv)
		if err != nil {
			return nil, trace.Wrap(err, "failed to evaluate receiver for dict.put method")
		}
		key, err := validateExprResult[string](env, keyExpr)
		if err != nil {
			return nil, trace.Wrap(err, "failed to evaluate first argument (key) to dict.put method")
		}
		value, err := validateExprResult[set](env, valueExpr)
		if err != nil {
			return nil, trace.Wrap(err, "failed to evaluate second argument (value) to dict.put method")
		}
		return d.put(key, value), nil
	}, nil
}

func buildNewPairExpr(a, b any) (expr, error) {
	aExpr, ok := a.(expr)
	if !ok {
		aExpr = buildLiteralExpr(a)
	}
	bExpr, ok := b.(expr)
	if !ok {
		bExpr = buildLiteralExpr(b)
	}

	return func(env *evaluationEnv) (any, error) {
		a, err := aExpr(env)
		if err != nil {
			return nil, trace.Wrap(err, "failed to evaluate first argument to pair constructor")
		}
		b, err := bExpr(env)
		if err != nil {
			return nil, trace.Wrap(err, "failed to evaluate second argument to pair constructor")
		}
		return pair{a, b}, nil
	}, nil
}
