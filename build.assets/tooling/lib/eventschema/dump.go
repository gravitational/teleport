/*
Copyright 2023 Gravitational, Inc.
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

package eventschema

import (
	"bytes"
	"fmt"
	"go/format"
	"sort"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	"github.com/gravitational/trace"
	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

const mainTemplate = `
{{- define "events" -}}
// Events is a map containing the description and schema for all Teleport events
var events = map[string]*Event{
    {{- range $_, $schema := orderSchemaMap .Roots }}
    {{- include "event" $schema | nindent 4 }}
    {{- end }}
}
{{- end -}}

{{- define "event" -}}
"{{ .Name }}": {
    Description: {{ .Description | quote }},
    {{- include "fields" . | nindent 4 }}
},
{{- end -}}

{{- define "fields" -}}
Fields: []*EventField{
{{- range $_, $prop := orderSchemaMap .Properties }}
  {{- if not (eq $prop.Type "null") }}
    {
        {{- include "namedField" $prop | nindent 8 }}
    },
  {{- end }}
{{- end }}
},
{{- end -}}

{{- define "namedField" -}}
Name: {{ .Name | quote }},
{{- include "field" . }}
{{- end -}}

{{- define "field" -}}
{{- if .Description }}
Description: {{ .Description | quote }},
{{- end }}
Type: "{{ .Type }}",
{{- if .Properties }}
{{ include "fields" . }}
{{- end }}
{{- if .Items }}
Items: &EventField{
    {{- include "field" .Items.Schema | indent 4 }}
},
{{- end }}
{{- end -}}

// Copyright {{ now | date "2006" }} Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package eventschema
{{ "// Code generated from protobuf, DO NOT EDIT." }}
// To re-generate the file, go into build.assets/ and run "make generate-eventschema".

type Event struct {
  Description string
  Fields      []*EventField
}

type EventField struct {
  Name        string
  Type        string
  Description string
  Fields      []*EventField
  Items       *EventField
}

{{ include "events" . }}`

const recursionMaxNums = 10

func (generator *SchemaGenerator) Render() (string, error) {
	t := template.New("*")

	funcMap := sprig.FuncMap()
	includedNames := make(map[string]int)

	// The template function doesn't output a string, so it cannot be piped into
	// another function (and we need to do this for indentation).
	// Helm solved this issue by implementing an "include" function that renders
	// the template and returns a string.
	funcMap["include"] = func(name string, data interface{}) (string, error) {
		var buf strings.Builder
		if v, ok := includedNames[name]; ok {
			if v > recursionMaxNums {
				return "", trace.WrapWithMessage(fmt.Errorf("unable to execute template"), "rendering template has a nested reference name: %s", name)
			}
			includedNames[name]++
		} else {
			includedNames[name] = 1
		}
		err := t.ExecuteTemplate(&buf, name, data)
		includedNames[name]--
		return buf.String(), err
	}

	type NamedSchema struct {
		Name string
		v1.JSONSchemaProps
	}

	// the orderSchemaMap iterates over a schema map and returns an order list
	// of all schemas wrapped in the NamedSchema type to store their name.
	// This ensures the template orders the events and their fields consistently.
	funcMap["orderSchemaMap"] = func(input map[string]v1.JSONSchemaProps) ([]NamedSchema, error) {
		keys := make([]string, 0, len(input))
		for key := range input {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		output := make([]NamedSchema, len(keys))
		for i, key := range keys {
			output[i] = NamedSchema{key, input[key]}
		}
		return output, nil
	}

	t = t.Funcs(funcMap)
	t = template.Must(t.Parse(mainTemplate))

	buf := &bytes.Buffer{}
	roots := make(map[string]v1.JSONSchemaProps, len(generator.roots))
	for key, root := range generator.roots {
		roots[key] = root.JSONSchemaProps
	}

	err := t.Execute(buf, struct {
		Roots map[string]v1.JSONSchemaProps
	}{roots})
	if err != nil {
		return "", trace.Wrap(err)
	}

	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		return "", trace.WrapWithMessage(err, "failed to format generated code")
	}

	return string(formatted), nil
}
