package main

import (
	"bytes"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	"github.com/gravitational/trace"
)

const referenceTemplate = `
{{- range .Values }}
#{{- range splitList "." .Name -}}#{{- end }} ` + "`" + `{{ .Name }}` + "`" + `

{{- if and .Kind .Default }}

| Type | Default |
|------|---------|
| ` + "`" + `{{.Kind}}` + "`" + ` | ` + "`" + `{{.Default}}` + "`" + ` |
{{- end }}

` + "`" + `{{.Name}}` + "`" + ` {{ .Description }}
{{- end -}}`

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
