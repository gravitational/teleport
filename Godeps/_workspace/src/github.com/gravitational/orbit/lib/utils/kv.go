package utils

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/trace"
	"github.com/gravitational/teleport/Godeps/_workspace/src/gopkg.in/alecthomas/kingpin.v2"
)

// KeyVal is a key value flag-compatible data structure
type KeyVal map[string]string

// Set accepts string with arguments in the form "key:val,key2:val2"
func (kv *KeyVal) Set(v string) error {
	for _, i := range SplitComma(v) {
		vals := strings.SplitN(i, ":", 2)
		if len(vals) != 2 {
			return trace.Errorf("extra options should be defined like KEY:VAL")
		}
		(*kv)[vals[0]] = vals[1]
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

// KeyValParam accepts a kingpin setting parameter and returns
// kingpin-compatible value
func KeyValParam(s kingpin.Settings) *KeyVal {
	kv := make(KeyVal)
	s.SetValue(&kv)
	return &kv
}
