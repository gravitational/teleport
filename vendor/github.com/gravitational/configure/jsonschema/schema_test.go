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
	"testing"

	. "gopkg.in/check.v1"
)

func TestSchema(t *testing.T) { TestingT(t) }

type SchemaSuite struct {
}

var _ = Suite(&SchemaSuite{})

func (s *SchemaSuite) TestDefaults(c *C) {
	tcs := []struct {
		schema string
		input  interface{}
		output interface{}
	}{
		{
			schema: `{"type": "string", "default": "hello"}`,
			input:  "",
			output: "hello",
		},
		{
			schema: `{"type": "number", "default": -1}`,
			input:  0,
			output: float64(-1),
		},
		{
			schema: `{
                 "type": "object",
                 "properties": {
                    "value": {"type": "string", "default": "hello"},
                    "value2": {"type": "string"}
                 }
            }`,
			input: map[string]interface{}{},
			output: map[string]interface{}{
				"value": "hello",
			},
		},
		{
			schema: `{
                 "type": "array",
                 "items": {
                    "type": "string",
                    "default": "hello"
                 }
            }`,
			input:  []interface{}{""},
			output: []interface{}{"hello"},
		},
		{
			schema: `{
                 "type": "array",
                 "items": {
                    "type": "object",
                    "properties": {
                       "val": {"type": "string", "default": "hello"}
                    }
                 }
            }`,
			input:  []interface{}{map[string]interface{}{}},
			output: []interface{}{map[string]interface{}{"val": "hello"}},
		},
		{
			schema: `{
                 "type": "object",
                 "default": {},
                 "properties": {
                     "values": {
                        "type": "object",
                        "default": {},
                        "properties": {
                           "val": {"type": "string", "default": "hellothere"}
                        }
                     }
                 }
            }`,
			input: map[string]interface{}{},
			output: map[string]interface{}{
				"values": map[string]interface{}{
					"val": "hellothere",
				},
			},
		},
		{
			schema: `{
                 "type": "object",
                 "properties": {
                     "values": {
                        "type": "array",
                         "items": {
                             "type": "object",
                             "properties": {
                                "val": {"type": "string", "default": "hello"}
                             }
                          }
                     }
                 }
            }`,
			input: map[string]interface{}{
				"values": []interface{}{
					map[string]interface{}{},
				},
			},
			output: map[string]interface{}{
				"values": []interface{}{
					map[string]interface{}{
						"val": "hello",
					},
				},
			},
		},
		{
			schema: `{
                 "type": "object",
                 "properties": {
                     "value": {"type": "string"}
                 }
            }`,
			input:  map[string]interface{}{},
			output: map[string]interface{}{},
		},
	}
	for i, tc := range tcs {
		comment := Commentf("test #%d", i+1)
		j, err := New([]byte(tc.schema))
		c.Assert(err, IsNil)
		c.Assert(j, NotNil)

		out, err := j.ProcessObject(tc.input)
		c.Assert(err, IsNil, comment)
		c.Assert(out, DeepEquals, tc.output, comment)
	}
}
