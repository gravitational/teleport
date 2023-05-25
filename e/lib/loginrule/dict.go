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

// Get implements typical.Getter[set]
func (d dict) Get(key string) (set, error) {
	return d[key], nil
}

type pair struct {
	first, second any
}
