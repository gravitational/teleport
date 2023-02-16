/*
Copyright 2021-2022 Gravitational, Inc.

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

package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/template"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/schemagen"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

var config = []schemagen.ParseResourceOptions{
	schemagen.ParseResourceOptions{
		Name: "UserV2",
		IgnoredFields: []string{
			"LocalAuth",
			"Expires",
			"CreatedBy",
			"Status",
		},
	},
	// Use RoleV6 spec but override the version to V5. This will generate
	// crd based on RoleV6 but with resource version for v5.
	schemagen.ParseResourceOptions{
		Name:            "RoleV6",
		VersionOverride: types.V5,
	},
	schemagen.ParseResourceOptions{
		Name: "RoleV6",
	},
	schemagen.ParseResourceOptions{
		Name: "SAMLConnectorV2",
	},
	schemagen.ParseResourceOptions{
		Name: "OIDCConnectorV3",
	},
	schemagen.ParseResourceOptions{
		Name:          "GithubConnectorV3",
		IgnoredFields: []string{"TeamsToLogins"}, // Deprecated field, removed since v11
	},
}

type TabbedVersionData struct {
	Versions []VersionData
}

type VersionData struct {
	VersionName string
	Rows        []VersionPropertyRow
	ExampleYAML string
}

// Number of Rows in the VersionData. Required for sorting VersionData rows.
func (d VersionData) Len() int {
	return len(d.Rows)
}

// Less is used to sort the rows in a VersionData by name in lexigraphic order.
// i and j are indices within d.Rows.
func (d VersionData) Less(i, j int) bool {
	return d.Rows[i].Name < d.Rows[j].Name
}

// Swap swaps the order of rows with indices i and j.
func (d VersionData) Swap(i, j int) {
	tmp := d.Rows[i]
	d.Rows[i] = d.Rows[j]
	d.Rows[j] = tmp
}

type VersionPropertyRow struct {
	Name        string
	Type        string
	Description string
}

// To be executed with a VersionData
var versionTableTemplate string = strings.ReplaceAll(
	`|Property|Description|Type|
|---|---|---|
{{- range .Rows }}
|~{{.Name}}~|{{.Description}}|{{.Type -}}|
{{- end }}

{{ .ExampleYAML }}`, "~", "`")

// To be executed with a TabbedVersionData
var tabbedVersionTableTemplate string = fmt.Sprintf(`<Tabs>
{{ range .Versions -}}
<TabItem label="{{ .VersionName }}">
%v
</TabItem>
{{ end -}}
</Tabs>`, versionTableTemplate)

// insertExampleValue uses the provided path, e.g., "key1.key2.key3", to insert the
// value val into map doc. Returns an error if the path is malformed.
func insertExampleValue(path string, doc map[string]interface{}, val interface{}) error {
	keyPath := strings.Split(path, ".")

	if len(keyPath) == 0 {
		return trace.Wrap(errors.New("no keys found to use for inserting an example value"))
	}

	var i map[string]interface{} = doc
	for j, k := range keyPath {
		v, ok := i[k]
		if !ok && j != len(keyPath)-1 {
			return trace.Wrap(fmt.Errorf("key path has more keys than will fit in the example YAML doc: %v", path))
		}
		// We've found a leaf node of the map
		if !ok {
			i[k] = val
			break
		}

		if m, ok := v.(map[string]interface{}); ok {
			i = m
			continue
		}
		return trace.Wrap(errors.New("unexpected value found while inserting example YAML"))
	}
	return nil
}

// generateExampleValue returns a value to insert into an example YAML document
// based on the type of props. It returns an error if it is not possible to
// generate an example.
// TODO: make this work recursively with arrays and maps
func generateExampleValue(props *apiextv1.JSONSchemaProps) (interface{}, error) {
	switch props.Type {
	case "string":
		return "string", nil
	case "number":
		return 0, nil

	// TODO: Detect if there are more properties to document examples for.
	// If not, populate the map with dummy values.
	case "object":
		return map[string]interface{}{}, nil
	case "array":
		if props.Items.Schema == nil {
			return nil, errors.New("the items of a JSON Schema array must include their own schema")
		}

		switch props.Items.Schema.Type {
		case "string":
			return []string{"string1", "string2", "string3"}, nil
		default:
			return nil, fmt.Errorf("unsupported array item type: %v", props.Items.Schema.Type)
		}
	default:
		return nil, trace.Wrap(fmt.Errorf(
			"unsupported property type: %v",
			props.Type,
		))
	}
}

// registerProperties recursively descends through the properties in props, adding
// these to the example YAML document in yml and the VersionPropertyRows in
// rows. It returns an error if props is malformed.
func registerProperties(
	yml map[string]interface{},
	props *apiextv1.JSONSchemaProps,
	// If this JSONSchemaProps is a child of another property
	parent *VersionPropertyRow,
) ([]VersionPropertyRow, error) {
	rows := []VersionPropertyRow{}
	for k, v := range props.Properties {
		n := k
		if parent != nil {
			n = parent.Name + "." + k
		}

		// TODO: For composite types, recursively descend through the
		// type's Items or AdditionalProperties to get the type of the
		// values

		r := VersionPropertyRow{
			Name:        n,
			Type:        v.Type,
			Description: v.Description,
		}
		rows = append(rows, r)

		val, err := generateExampleValue(&v)

		if err != nil {
			return nil, trace.Wrap(err)
		}

		err = insertExampleValue(n, yml, val)

		if err != nil {
			return nil, trace.Wrap(err)
		}

		nr, err := registerProperties(yml, &v, &r)

		rows = append(rows, nr...)

		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return rows, nil
}

func generateTable(c *schemagen.RootSchema) (*schemagen.TransformedFile, error) {
	if c == nil || c.Versions == nil || len(c.Versions) == 0 {
		return nil, trace.Wrap(errors.New("no schema version available to parse"))
	}

	td := TabbedVersionData{}

	for _, v := range c.Versions {
		vd := VersionData{
			VersionName: v.Version,
		}

		// We'll use this to populate the example YAML document
		ex := make(map[string]interface{})

		rows, err := registerProperties(
			ex,
			&v.Schema.JSONSchemaProps,
			nil,
		)

		if err != nil {
			return nil, trace.Wrap(err)
		}

		vd.Rows = rows
		var buf bytes.Buffer
		if err := yaml.NewEncoder(&buf).Encode(ex); err != nil {
			return nil, trace.Wrap(err)
		}
		vd.ExampleYAML = buf.String()

		sort.Sort(vd)

		td.Versions = append(td.Versions, vd)
	}

	var buf bytes.Buffer

	if len(td.Versions) > 1 {
		tmpl, err := template.New("Tabbed version data").Parse(tabbedVersionTableTemplate)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		err = tmpl.Execute(&buf, td)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	} else {
		tmpl, err := template.New("Version data").Parse(versionTableTemplate)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		err = tmpl.Execute(&buf, td.Versions[0])
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	return &schemagen.TransformedFile{
		Name:    c.Name + ".yaml",
		Content: buf.String(),
	}, nil
}

func main() {
	log.SetLevel(log.DebugLevel)
	log.SetOutput(os.Stderr)

	if err := schemagen.RunPlugin(config, generateTable); err != nil {
		log.WithError(err).Error("Failed to generate schema")
		os.Exit(-1)
	}
}
