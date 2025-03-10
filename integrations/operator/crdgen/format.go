/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package crdgen

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
	"text/template"

	"github.com/gravitational/trace"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"sigs.k8s.io/yaml"
)

// crdFormatFunc formats the given CRD into a document. It returns the document
// as a byte slice, plus the file name for the document. The file name is based
// on the CRD's API group name and plural name.
type crdFormatFunc func(crd apiextv1.CustomResourceDefinition, groupName, pluralName string) ([]byte, string, error)

func formatAsCRD(crd apiextv1.CustomResourceDefinition, groupName, pluralName string) ([]byte, string, error) {
	doc, err := yaml.Marshal(crd)
	if err != nil {
		return nil, "", err
	}
	filename := fmt.Sprintf("%s_%s.%v", groupName, pluralName, "yaml")
	return doc, filename, nil
}

var crdDocTmpl string = `---
title: {{.Title}}
description: {{.Description}}
tocDepth: 3
---

{/*Auto-generated file. Do not edit.*/}
{/*To regenerate, navigate to integrations/operator and run "make crd-docs".*/}

{{.Intro}}

{{ range .Sections}}
## {{.APIVersion}}

**apiVersion:** {{.APIVersion}}

{{- range .Subsections }}
{{- if ne .Name "" }}
### {{.Name}}
{{- end }}

|Field|Type|Description|
|---|---|---|
{{- range .Fields }}
|{{.Name}}|{{.Type}}|{{.Description}}|
{{- end }}
{{ end }}

{{- end}}
`

type ResourcePage struct {
	Title       string
	Description string
	Intro       string
	Sections    []VersionSection
}

func (r ResourcePage) Len() int {
	return len(r.Sections)
}

func (r ResourcePage) Swap(i, j int) {
	r.Sections[i], r.Sections[j] = r.Sections[j], r.Sections[i]
}

func (r ResourcePage) Less(i, j int) bool {
	return strings.Compare(r.Sections[i].APIVersion, r.Sections[j].APIVersion) == -1
}

func (v VersionSection) Len() int {
	return len(v.Subsections)
}

func (v VersionSection) Swap(i, j int) {
	v.Subsections[i], v.Subsections[j] = v.Subsections[j], v.Subsections[i]
}

func (v VersionSection) Less(i, j int) bool {
	return strings.Compare(v.Subsections[i].Name, v.Subsections[j].Name) == -1
}

type VersionSection struct {
	APIVersion  string
	Subsections []PropertyTable
}

type PropertyTable struct {
	Name   string
	Fields []PropertyTableField
}

type PropertyTableField struct {
	Name        string
	Type        string
	Description string
}

func (t PropertyTable) Len() int {
	return len(t.Fields)
}

func (t PropertyTable) Swap(i, j int) {
	t.Fields[i], t.Fields[j] = t.Fields[j], t.Fields[i]
}

func (t PropertyTable) Less(i, j int) bool {
	return strings.Compare(t.Fields[i].Name, t.Fields[j].Name) == -1
}

// Skipping the "status" field, since it is not available for users to
// configure.
const statusDescription = "Status defines the observed state of the Teleport resource"
const statusName = "status"

func propertyTable(currentFieldName string, props *apiextv1.JSONSchemaProps) ([]PropertyTable, error) {
	// Only create a property table for an object field. For other types, we can
	// describe the type within a table row.
	if props.Type != "object" {
		return nil, nil
	}
	tab := PropertyTable{
		Name: currentFieldName,
	}
	fields := []PropertyTableField{}
	tables := []PropertyTable{}
	var i int
	for k, v := range props.Properties {
		// Don't document the Status field, which is for
		// internal use.
		if k == statusName && strings.HasPrefix(v.Description, statusDescription) {
			continue
		}
		// Name the table after the hierarchy of
		// field names to avoid duplication.
		var tableName string
		if currentFieldName != "" {
			tableName = currentFieldName + "." + k
		} else {
			tableName = k
		}

		var fieldType string
		var fieldDesc string
		switch v.Type {
		case "object":
			fieldType = "object"
			if len(v.Properties) == 0 {
				break
			}

			extra, err := propertyTable(
				tableName,
				&v,
			)
			if err != nil {
				return nil, err
			}
			fieldType = fmt.Sprintf("[object](#%v)", strings.ReplaceAll(strings.ReplaceAll(tableName, ".", ""), " ", "-"))
			tables = append(tables, extra...)
		case "array":
			var subtp string
			if v.Items.Schema.Type == "object" {
				extra, err := propertyTable(
					fmt.Sprintf("%v items", tableName),
					v.Items.Schema,
				)
				if err != nil {
					return nil, err
				}
				tables = append(tables, extra...)
				subtp = fmt.Sprintf("[object](#%v-items)", strings.ReplaceAll(strings.ReplaceAll(tableName, ".", ""), " ", "-"))
			} else {
				subtp = v.Items.Schema.Type
			}
			fieldType = fmt.Sprintf("[]%v", subtp)
		case "":
			if !v.XIntOrString {
				fieldType = v.Type
				break
			}
			fieldType = "string or integer"
			fieldDesc = strings.TrimSuffix(v.Description, ".") + ". " + "Can be either the string or the integer representation of each option."
		default:
			fieldType = v.Type
		}

		if fieldDesc == "" {
			fieldDesc = v.Description
		}

		fields = append(fields, PropertyTableField{
			Name:        k,
			Type:        fieldType,
			Description: fieldDesc,
		})
		i++
	}
	tab.Fields = fields
	sort.Sort(tab)
	tables = append([]PropertyTable{tab}, tables...)
	return tables, nil
}

func formatAsDocsPage(crd apiextv1.CustomResourceDefinition, groupName, pluralName string) ([]byte, string, error) {
	var buf bytes.Buffer
	rp := ResourcePage{
		Title:       crd.Spec.Names.Kind,
		Description: fmt.Sprintf("Provides a comprehensive list of fields in the %v resource available through the Teleport Kubernetes operator", crd.Spec.Names.Kind),
		Intro: strings.ReplaceAll(fmt.Sprintf(
			`This guide is a comprehensive reference to the fields in the BACKTICK%vBACKTICK
resource, which you can apply after installing the Teleport Kubernetes operator.`,
			crd.Spec.Names.Kind), "BACKTICK", "`"),
	}

	vs := make([]VersionSection, len(crd.Spec.Versions))
	for i, v := range crd.Spec.Versions {
		props, err := propertyTable("", v.Schema.OpenAPIV3Schema)
		if err != nil {
			return nil, "", err
		}
		n := VersionSection{
			APIVersion:  fmt.Sprintf("%v/%v", crd.Spec.Group, v.Name),
			Subsections: props,
		}
		sort.Sort(n)
		vs[i] = n
	}
	rp.Sections = vs

	templ := template.New("docs")
	templ, err := templ.Parse(crdDocTmpl)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	err = templ.Execute(&buf, rp)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	filename := fmt.Sprintf(
		"%s-%s.%v",
		strings.ReplaceAll(groupName, ".", "-"),
		pluralName,
		"mdx",
	)
	return buf.Bytes(), filename, nil
}
