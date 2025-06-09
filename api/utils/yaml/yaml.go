package yaml

import (
	"bytes"
	"encoding/json"
	"io"
	"reflect"

	"github.com/goccy/go-yaml"
	"github.com/gravitational/trace"
)

func requiresJSONToYAML(vType reflect.Type) bool {
	switch vType.Kind() {
	case reflect.Struct:
		// continue below
	case reflect.Pointer, reflect.Array, reflect.Chan, reflect.Slice:
		return requiresJSONToYAML(vType.Elem())
	case reflect.Map:
		return requiresJSONToYAML(vType.Key()) || requiresJSONToYAML(vType.Elem())
	default:
		return false
	}

	for i := range vType.NumField() {
		field := vType.Field(i)
		if field.Anonymous && field.Tag.Get("json") == "" {
			return true
		}
	}
	return false
}

func Marshal(v any) ([]byte, error) {
	if requiresJSONToYAML(reflect.TypeOf(v)) {
		jsonData, err := json.Marshal(v)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		yamlData, err := yaml.JSONToYAML(jsonData)
		return yamlData, trace.Wrap(err)
	}
	buf := &bytes.Buffer{}
	err := yaml.NewEncoder(buf).Encode(v)
	return buf.Bytes(), trace.Wrap(err)
}

func Unmarshal(data []byte, v any) error {
	return trace.Wrap(NewDecoder(bytes.NewReader(data)).Decode(v))
}

func NewDecoder(r io.Reader, opts ...yaml.DecodeOption) *yaml.Decoder {
	return yaml.NewDecoder(r, append([]yaml.DecodeOption{yaml.UseJSONUnmarshaler()}, opts...)...)
}

func NewEncoder(w io.Writer, opts ...yaml.EncodeOption) *yaml.Encoder {
	return yaml.NewEncoder(w, append([]yaml.EncodeOption{yaml.UseJSONMarshaler()}, opts...)...)
}
