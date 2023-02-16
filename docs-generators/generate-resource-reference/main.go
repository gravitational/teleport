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
	"strings"

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

type VersionPropertyRow struct {
	Name        string
	Type        string
	Description string
}

// To be executed with a VersionDAta
var versionTableTemplate string = strings.ReplaceAll(
	`|Property|Description|Type|
|---|---|---|
{{ range .Rows }}
| ~{{ .Name }}~|{{.Description}}|{{.Type}}|
{{ end }} 

{{ .ExampleYAML }}
    `, "~", "`")

// To be executed with a TabbedVersionData
var tabbedVersionTableTemplate string = fmt.Sprintf(`<Tabs>
{{ range .Versions }}
<TabItem label={{ .VersionName }}>
%v
</TabItem>
{{ end }}
</Tabs>
`, versionTableTemplate)

// registerProperties recursively descends through the properties in props, adding
// these to the example YAML document in yml and the VersionPropertyRows in
// rows.
func registerProperties(
	yml map[string]interface{},
	rows []VersionPropertyRow,
	props *apiextv1.JSONSchemaProps,
	// If this JSONSchemaProps is a child of another property
	parent *VersionPropertyRow,
) {
	for k, v := range props.Properties {
		n := k
		if parent != nil {
			n = parent.Name + "." + k
		}

		r := VersionPropertyRow{
			Name:        n,
			Type:        v.Type,
			Description: v.Description,
		}
		rows = append(rows, r)
		// TODO: Assign new key to the example yml
		registerProperties(yml, rows, &v, &r)
	}
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
		var rows []VersionPropertyRow

		registerProperties(
			ex,
			rows,
			&v.Schema.JSONSchemaProps,
			nil,
		)

		vd.Rows = rows
		var buf bytes.Buffer
		if err := yaml.NewEncoder(&buf).Encode(ex); err != nil {
			return nil, trace.Wrap(err)
		}
		vd.ExampleYAML = buf.String()
		td.Versions = append(td.Versions, vd)
	}

	return nil, nil
}

func main() {
	log.SetLevel(log.DebugLevel)
	log.SetOutput(os.Stderr)

	if err := schemagen.RunPlugin(config, generateTable); err != nil {
		log.WithError(err).Error("Failed to generate schema")
		os.Exit(-1)
	}
}
