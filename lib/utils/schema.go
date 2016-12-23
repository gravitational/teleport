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

package utils

import (
	"encoding/json"

	"github.com/gravitational/configure/jsonschema"
	"github.com/gravitational/trace"
)

// UnmarshalWithSchema processes YAML or JSON encoded object with JSON schema, sets defaults
// and unmarshals resulting object into given struct
func UnmarshalWithSchema(schemaDefinition string, object interface{}, data []byte) error {
	schema, err := jsonschema.New([]byte(schemaDefinition))
	if err != nil {
		return trace.Wrap(err)
	}
	jsonData, err := ToJSON(data)
	if err != nil {
		return trace.Wrap(err)
	}

	raw := map[string]interface{}{}
	if err := json.Unmarshal(jsonData, &raw); err != nil {
		return trace.Wrap(err)
	}
	// schema will check format and set defaults
	processed, err := schema.ProcessObject(raw)
	if err != nil {
		return trace.Wrap(err)
	}
	// since ProcessObject works with unstructured data, the
	// data needs to be re-interpreted in structured form
	bytes, err := json.Marshal(processed)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := json.Unmarshal(bytes, object); err != nil {
		return trace.Wrap(err)
	}
	return nil
}
