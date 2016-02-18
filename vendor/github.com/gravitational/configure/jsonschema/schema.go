/*
Copyright 2015 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package jsonschema

import (
	"encoding/json"
	"reflect"
	"strings"

	"github.com/gravitational/trace"
	"github.com/xeipuuv/gojsonschema"
)

// JSONSchema is a wrapper around gojsonschema that supports
// default variables
type JSONSchema struct {
	// schema specifies site-specific provisioning and installation
	// instructions expressed as JSON schema
	schema *gojsonschema.Schema
	// rawSchema is a parsed JSON schema, so we can set up
	// default variables
	rawSchema map[string]interface{}
}

// New returns JSON schema created from JSON byte string
// returns a valid schema or error if schema is invalid
func New(data []byte) (*JSONSchema, error) {
	j := JSONSchema{}
	err := json.Unmarshal(data, &j.rawSchema)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	loader := gojsonschema.NewGoLoader(j.rawSchema)
	j.schema, err = gojsonschema.NewSchema(loader)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &j, nil
}

// ProcessObject checks if the object is valid from this schema's standpoint
// and returns an object with defaults set up according to schema's spec
func (j *JSONSchema) ProcessObject(in interface{}) (interface{}, error) {
	result, err := j.schema.Validate(gojsonschema.NewGoLoader(in))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if !result.Valid() {
		return nil, trace.Wrap(trace.Errorf("errors: %v", schemaErrors(result.Errors())))
	}
	return setDefaults(j.rawSchema, in), nil
}

func setDefaults(ischema interface{}, ivars interface{}) interface{} {
	if ischema == nil {
		return ivars
	}
	schema, ok := ischema.(map[string]interface{})
	if !ok {
		return ivars
	}
	tp := getStringProp(schema, "type")
	switch tp {
	case "object":
		vars, ok := ivars.(map[string]interface{})
		if !ok {
			return ivars
		}
		if len(vars) == 0 {
			vars = make(map[string]interface{})
		}
		props, ok := getProperties(schema, "properties")
		if !ok {
			return ivars
		}
		out := make(map[string]interface{})
		for key, prop := range props {
			_, have := vars[key]
			defval := setDefaults(prop, vars[key])
			// only set default value if the property
			// is missing and retunred default value is not empty
			// otherwise we will return a bunch of nils
			if !have && isEmpty(defval) {
				continue
			}
			out[key] = defval
		}
		return out
	case "array":
		vars, ok := ivars.([]interface{})
		if !ok {
			return ivars
		}
		if len(vars) == 0 {
			return ivars
		}
		// we currently do not support tuples
		itemSchema, ok := getProperties(schema, "items")
		if !ok {
			return ivars
		}
		out := make([]interface{}, len(vars))
		for i, val := range vars {
			out[i] = setDefaults(itemSchema, val)
		}
		return out
	default:
		if isEmpty(ivars) {
			defval := schema["default"]
			if !isEmpty(defval) {
				return defval
			}
		}
		return ivars
	}
	return ivars
}

func isEmpty(x interface{}) bool {
	return x == nil || reflect.DeepEqual(x, reflect.Zero(reflect.TypeOf(x)).Interface())
}

func getStringProp(iobj interface{}, name string) string {
	obj, ok := iobj.(map[string]interface{})
	if !ok {
		return ""
	}
	i, ok := obj[name]
	if !ok {
		return ""
	}
	v, _ := i.(string)
	return v
}

func getProperties(schema map[string]interface{}, name string) (map[string]interface{}, bool) {
	i, ok := schema[name]
	if !ok {
		return nil, false
	}
	v, ok := i.(map[string]interface{})
	if !ok {
		return nil, false
	}
	if len(v) == 0 || v == nil {
		return nil, false
	}
	return v, true
}

func schemaErrors(errors []gojsonschema.ResultError) string {
	out := make([]string, len(errors))
	for i, err := range errors {
		out[i] = err.Description()
	}
	return strings.Join(out, ",")
}
