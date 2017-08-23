// Copyright 2015 xeipuuv ( https://github.com/xeipuuv )
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// author  			xeipuuv
// author-github 	https://github.com/xeipuuv
// author-mail		xeipuuv@gmail.com
// 
// repository-name	gojsonpointer
// repository-desc	An implementation of JSON Pointer - Go language
// 
// description		Automated tests on package.
// 
// created      	03-03-2013

package gojsonpointer

import (
	"encoding/json"
	"testing"
)

const (
	TEST_DOCUMENT_NB_ELEMENTS = 11
	TEST_NODE_OBJ_NB_ELEMENTS = 4
	TEST_DOCUMENT_STRING      = `{
"foo": ["bar", "baz"],
"obj": { "a":1, "b":2, "c":[3,4], "d":[ {"e":9}, {"f":[50,51]} ] },
"": 0,
"a/b": 1,
"c%d": 2,
"e^f": 3,
"g|h": 4,
"i\\j": 5,
"k\"l": 6,
" ": 7,
"m~n": 8
}`
)

var testDocumentJson interface{}

func init() {
	json.Unmarshal([]byte(TEST_DOCUMENT_STRING), &testDocumentJson)
}

func TestEscaping(t *testing.T) {

	ins := []string{`/`, `/`, `/a~1b`, `/a~1b`, `/c%d`, `/e^f`, `/g|h`, `/i\j`, `/k"l`, `/ `, `/m~0n`}
	outs := []float64{0, 0, 1, 1, 2, 3, 4, 5, 6, 7, 8}

	for i := range ins {

		p, err := NewJsonPointer(ins[i])
		if err != nil {
			t.Errorf("NewJsonPointer(%v) error %v", ins[i], err.Error())
		}

		result, _, err := p.Get(testDocumentJson)
		if err != nil {
			t.Errorf("Get(%v) error %v", ins[i], err.Error())
		}

		if result != outs[i] {
			t.Errorf("Get(%v) = %v, expect %v", ins[i], result, outs[i])
		}
	}

}

func TestFullDocument(t *testing.T) {

	in := ``

	p, err := NewJsonPointer(in)
	if err != nil {
		t.Errorf("NewJsonPointer(%v) error %v", in, err.Error())
	}

	result, _, err := p.Get(testDocumentJson)
	if err != nil {
		t.Errorf("Get(%v) error %v", in, err.Error())
	}

	if len(result.(map[string]interface{})) != TEST_DOCUMENT_NB_ELEMENTS {
		t.Errorf("Get(%v) = %v, expect full document", in, result)
	}
}

func TestGetNode(t *testing.T) {

	in := `/obj`

	p, err := NewJsonPointer(in)
	if err != nil {
		t.Errorf("NewJsonPointer(%v) error %v", in, err.Error())
	}

	result, _, err := p.Get(testDocumentJson)
	if err != nil {
		t.Errorf("Get(%v) error %v", in, err.Error())
	}

	if len(result.(map[string]interface{})) != TEST_NODE_OBJ_NB_ELEMENTS {
		t.Errorf("Get(%v) = %v, expect full document", in, result)
	}
}

func TestArray(t *testing.T) {

	ins := []string{`/foo/0`, `/foo/0`, `/foo/1`}
	outs := []string{"bar", "bar", "baz"}

	for i := range ins {

		p, err := NewJsonPointer(ins[i])
		if err != nil {
			t.Errorf("NewJsonPointer(%v) error %v", ins[i], err.Error())
		}

		result, _, err := p.Get(testDocumentJson)
		if err != nil {
			t.Errorf("Get(%v) error %v", ins[i], err.Error())
		}

		if result != outs[i] {
			t.Errorf("Get(%v) = %v, expect %v", ins[i], result, outs[i])
		}
	}

}

func TestObject(t *testing.T) {

	ins := []string{`/obj/a`, `/obj/b`, `/obj/c/0`, `/obj/c/1`, `/obj/c/1`, `/obj/d/1/f/0`}
	outs := []float64{1, 2, 3, 4, 4, 50}

	for i := range ins {

		p, err := NewJsonPointer(ins[i])
		if err != nil {
			t.Errorf("NewJsonPointer(%v) error %v", ins[i], err.Error())
		}

		result, _, err := p.Get(testDocumentJson)
		if err != nil {
			t.Errorf("Get(%v) error %v", ins[i], err.Error())
		}

		if result != outs[i] {
			t.Errorf("Get(%v) = %v, expect %v", ins[i], result, outs[i])
		}
	}

}

func TestSetNode(t *testing.T) {

	jsonText := `{"a":[{"b": 1, "c": 2}], "d": 3}`

	var jsonDocument interface{}
	json.Unmarshal([]byte(jsonText), &jsonDocument)

	in := "/a/0/c"

	p, err := NewJsonPointer(in)
	if err != nil {
		t.Errorf("NewJsonPointer(%v) error %v", in, err.Error())
	}

	_, err = p.Set(jsonDocument, 999)
	if err != nil {
		t.Errorf("Set(%v) error %v", in, err.Error())
	}

	firstNode := jsonDocument.(map[string]interface{})
	if len(firstNode) != 2 {
		t.Errorf("Set(%s) failed", in)
	}

	sliceNode := firstNode["a"].([]interface{})
	if len(sliceNode) != 1 {
		t.Errorf("Set(%s) failed", in)
	}

	changedNode := sliceNode[0].(map[string]interface{})
	changedNodeValue := changedNode["c"].(int)

	if changedNodeValue != 999 {
		if len(sliceNode) != 1 {
			t.Errorf("Set(%s) failed", in)
		}
	}

}
