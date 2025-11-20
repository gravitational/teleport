// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package main

import (
	"fmt"
	"os"
	"regexp"
	"slices"
	"strings"
	"text/template"

	"github.com/gravitational/teleport/gen/go/eventschema"
)

// colNameList prints an example of columns from a given Access Monitoring event
// view to include an example command. It only prints up to the first three
// columns.
func colNameList(cols []*eventschema.ColumnSchemaDetails) string {
	var names []string
	for i := 0; i < len(cols) && i < 3; i++ {
		names = append(names, cols[i].NameSQL())
	}
	return strings.Join(names, ",")
}

var descPredicate = regexp.MustCompile(`^(is|are) `)

// prepareDescription returns a description of the column data provided in col.
func prepareDescription(col *eventschema.ColumnSchemaDetails) string {
	// Remove the initial verb, since there is no subject in the sentence.
	desc := descPredicate.ReplaceAllString(
		col.Description,
		"",
	)

	// Capitalize the first word in the description.
	if len(desc) > 1 {
		desc = strings.ToUpper(string(desc[0])) + desc[1:]
	}

	return desc
}

// docTempl is the template that represents an Access Monitoring event reference
// docs page. The assumption is that "@" characters are replaced with backticks
// before rendering the template.
const docTempl = `{/*generated file. DO NOT EDIT.*/}
{/*To generate, run make access-monitoring-reference*/}
{/*vale messaging.capitalization = NO*/}
{/*vale messaging.consistent-terms = NO*/}

{{ range . -}}
## {{ .Name }}

@{{ .Name }}@ {{ .Description }}.

Example query:

@@@code
$ tctl audit query exec \
  'select {{ ColNameList .Columns }} from {{ .SQLViewName }} limit 1'
@@@

Columns:

|SQL Name|Type|Description|
|---|---|---|
{{- range .Columns }}
|{{ .NameSQL }}|{{ .Type }}|{{ PrepareDescription . }}|
{{- end }}

{{ end }}
{/*vale messaging.capitalization = YES*/}
{/*vale messaging.consistent-terms = YES*/}
`

func main() {
	data, err := eventschema.GetViewsDetails()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot generate an Access Monitoring schema reference: %v\n", err)
		os.Exit(1)
	}

	slices.SortFunc(data, func(a, b *eventschema.TableSchemaDetails) int {
		return strings.Compare(a.Name, b.Name)
	})

	template.Must(template.New("event-reference").Funcs(
		template.FuncMap{
			"ColNameList":        colNameList,
			"PrepareDescription": prepareDescription,
		},
	).Parse(strings.ReplaceAll(docTempl, "@", "`"))).Execute(os.Stdout, data)
}
