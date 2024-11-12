/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package main

import (
	"bytes"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	"github.com/gravitational/trace"
)

const referenceTemplate = `
{/* Generated file. Do not edit.*/}
{/* Generate this file by navigating to examples/chart and running  make render-chart-ref*/}

{{- range .Values }}
#{{- range splitList "." .Name -}}#{{- end }} ` + "`" + `{{ .Name }}` + "`" + `

{{- if and .Kind .Default }}

| Type | Default |
|------|---------|
| ` + "`" + `{{.Kind}}` + "`" + ` | ` + "`" + `{{.Default}}` + "`" + ` |
{{- end }}

{{- if .Description }}

` + "`" + `{{.Name}}` + "`" + ` {{ .Description }}
{{- end }}
{{ end -}}`

func renderTemplate(values []*Value) ([]byte, error) {
	t := template.Must(template.New("reference").Funcs(sprig.FuncMap()).Parse(referenceTemplate))
	params := struct {
		Values []*Value
	}{
		values,
	}
	buf := &bytes.Buffer{}
	err := t.Execute(buf, params)
	return buf.Bytes(), trace.Wrap(err)
}
