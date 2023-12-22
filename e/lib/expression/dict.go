package expression

import "github.com/gravitational/trace"

// Dict is a map of type string key and Set values.
type Dict map[string]Set

// NewDict returns a dict initialized with the key-value pairs as specified in
// [pairs].
func NewDict(pairs ...pair) (Dict, error) {
	d := make(Dict, len(pairs))
	for _, p := range pairs {
		k, ok := p.first.(string)
		if !ok {
			return nil, trace.BadParameter("dict keys must have type string, got %T", p.first)
		}
		v, ok := p.second.(Set)
		if !ok {
			return nil, trace.BadParameter("dict values must have type set, got %T", p.second)
		}
		d[k] = v
	}
	return d, nil
}

func (d Dict) addValues(key string, values ...string) Dict {
	out := d.clone()
	s := out[key]
	if s == nil {
		out[key] = NewSet(values...)
		return out
	}
	// Calling set.add would do an unnecessary extra copy, add the values
	// "manually".
	for _, value := range values {
		s[value] = struct{}{}
	}
	return out
}

func (d Dict) put(key string, value Set) Dict {
	out := d.clone()
	out[key] = value
	return out
}

func (d Dict) remove(keys ...string) any {
	out := d.clone()
	for _, key := range keys {
		delete(out, key)
	}
	return out
}

func (d Dict) clone() Dict {
	out := make(Dict, len(d))
	for key, set := range d {
		out[key] = set.clone()
	}
	return out
}

// Get implements typical.Getter[set]
func (d Dict) Get(key string) (Set, error) {
	return d[key], nil
}

type pair struct {
	first, second any
}
