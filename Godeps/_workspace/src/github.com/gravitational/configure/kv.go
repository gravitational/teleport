package configure

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/trace"
)

// KeyVal is map that can parse itself from string, represented as a
// comma-separated list of keys and values "key:val,key:val"
type KeyVal map[string]string

// Set accepts string with arguments in the form "key:val,key2:val2"
func (kv *KeyVal) Set(v string) error {
	if len(*kv) == 0 {
		*kv = make(map[string]string)
	}
	for _, i := range SplitComma(v) {
		vals := strings.SplitN(i, ":", 2)
		if len(vals) != 2 {
			return trace.Errorf("extra options should be defined like KEY:VAL")
		}
		(*kv)[vals[0]] = vals[1]
	}
	return nil
}

// SetEnv sets the value from environment variable using json encoding
func (kv *KeyVal) SetEnv(v string) error {
	if err := json.Unmarshal([]byte(v), &kv); err != nil {
		return trace.Wrap(
			err, "failed to parse environment variable, expected JSON map")
	}
	return nil
}

// String returns a string with comma separated key-values: "key:val,key2:val2"
func (kv *KeyVal) String() string {
	b := &bytes.Buffer{}
	for k, v := range *kv {
		fmt.Fprintf(b, "%v:%v", k, v)
		fmt.Fprintf(b, " ")
	}
	return b.String()
}

// KeyValSlice is a list of key value strings
type KeyValSlice []map[string]string

// Set accepts string with arguments in the form "key:val,key2:val2"
func (kv *KeyValSlice) Set(v string) error {
	if len(*kv) == 0 {
		*kv = make([]map[string]string, 0)
	}
	var i KeyVal
	if err := i.Set(v); err != nil {
		return trace.Wrap(err)
	}
	*kv = append(*kv, i)
	return nil
}

// SetEnv sets the value from environment variable using json encoding
func (kv *KeyValSlice) SetEnv(v string) error {
	if err := json.Unmarshal([]byte(v), &kv); err != nil {
		return trace.Wrap(
			err, "failed to parse environment variable, expected JSON map")
	}
	return nil
}

// String returns a string with comma separated key-values: "key:val,key2:val2"
func (kv *KeyValSlice) String() string {
	b := &bytes.Buffer{}
	for k, v := range *kv {
		fmt.Fprintf(b, "%v:%v", k, v)
		fmt.Fprintf(b, " ")
	}
	return b.String()
}
