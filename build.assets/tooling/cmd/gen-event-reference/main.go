package main

import (
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"regexp"
	"sort"
	"strings"
	"text/template"

	"github.com/gravitational/teleport/gen/go/eventschema"
)

// EventEntryCollection represents data to include in the audit event reference.
type EventEntryCollection []EventEntry

// EventEntry is a section of the audit event reference about a single event.
type EventEntry struct {
	Name   string
	Code   string
	Type   string
	Schema eventschema.Event
}

// Len implements sort.Interface.
func (c EventEntryCollection) Len() int {
	return len(c)
}

// Less implements sort.Interface.
func (c EventEntryCollection) Less(i, j int) bool {
	return strings.Compare(c[i].Name, c[j].Name) == -1
}

// Swap implements sort.Interface.
func (c EventEntryCollection) Swap(i, j int) {
	tmp := c[i]
	c[i] = c[j]
	c[j] = tmp
}

// makeEventSectionName determines how to name a section of the audit event
// reference based on the name of the event's code constant (e.g.,
// UserLoginCode) and type (e.g., user.login). If the event has "Failure" in the
// code name, we add "(failure)" after the section name.
func makeEventSectionName(codeName, typeValue string) string {
	if strings.Contains(codeName, "Failure") {
		return typeValue + " (failure)"
	}
	return typeValue
}

// stringConstantDecls takes a Go source file and returns a mapping from all
// string constant declaration names to their values.
func stringConstantDecls(r io.Reader) (map[string]string, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(
		fset,
		"",
		r,
		parser.ParseComments,
	)
	if err != nil {
		return nil, err
	}

	res := make(map[string]string)
	for _, d := range f.Decls {
		g, ok := d.(*ast.GenDecl)
		if !ok {
			continue
		}

		if g.Tok != token.CONST {
			continue
		}

		for _, s := range g.Specs {
			v := s.(*ast.ValueSpec)
			if len(v.Names) != 1 || len(v.Values) != 1 {
				continue
			}

			val, ok := v.Values[0].(*ast.BasicLit)
			if !ok {
				continue
			}
			if val.Kind != token.STRING {
				continue
			}

			res[v.Names[0].Name] = strings.Trim(val.Value, `"`)
		}
	}
	return res, nil
}

// codeNamePattern represents the naming convention we expect event code
// constants to have, e.g., "UserLoginCode", "UserLoginSuccessCode", or
// "UserLoginFailureCode".
var codeNamePattern = regexp.MustCompile(`(Failure|Success)?Code$`)

func isEventCode(s string) bool {
	return codeNamePattern.MatchString(s)
}

// typeNameFromCodeName helps us find the constant that declares an event type
// from the corresponding constant that declares an event code. We expect type
// constants to share the same prefix as event code constants, but to end with
// "Event" rather than "Code".
func typeNameFromCodeName(codeName string) string {
	return codeNamePattern.ReplaceAllString(codeName, "Event")
}

// schemaNameFromCodeName helps us find the schema declaration that corresponds
// to an event code declaration. We expect schema names to have the same prefix
// as code names after removing "Code", "SuccessCode", or "FailureCode".
func schemaNameFromCodeName(codeName string) string {
	return codeNamePattern.ReplaceAllString(codeName, "")
}

type eventDataError struct {
	messages []string
}

func (e eventDataError) Error() string {
	return "- " + strings.Join(e.messages, "\n- ")
}

// makeEventEntries assembles data for an audit event reference. It expects one
// Go source file to contain constants that declare event codes; one source file
// to declare constants for event types; and one mapping of event schema message
// declaration names to their JSON schema representations.
//
// To associate event types, names, and schemas, we start with each event code,
// and use the name of the code's constant declaration to find the corresponding
// type and schema using a naming convention.
func makeEventEntries(
	codeDecls io.Reader,
	typeDecls io.Reader,
	schemas map[string]*eventschema.Event,
) ([]EventEntry, error) {
	var e []EventEntry
	codes, err := stringConstantDecls(codeDecls)
	if err != nil {
		return nil, err
	}

	types, err := stringConstantDecls(typeDecls)
	if err != nil {
		return nil, err
	}

	// At this point, we have all the data we need to generate the
	// EventEntryCollection. Start an eventDataError so we can report event
	// code and type constants that go against the naming convention while
	// generating a reference of all the events we can.
	ederr := eventDataError{
		messages: []string{},
	}

	for codeName, codeVal := range codes {
		if !isEventCode(codeName) {
			continue
		}

		tn := typeNameFromCodeName(codeName)
		t, ok := types[tn]
		if !ok {
			ederr.messages = append(
				ederr.messages,
				fmt.Sprintf(
					"could not find an event type for event code constant %v; expected a constant called %v",
					codeName,
					tn,
				))
			continue
		}
		sn := schemaNameFromCodeName(codeName)
		s, ok := schemas[sn]
		if !ok {
			ederr.messages = append(
				ederr.messages,
				fmt.Sprintf(
					"could not find a schema for event code const %v; expected a constant called %v",
					codeName,
					sn,
				))
			continue
		}
		e = append(
			e,
			EventEntry{
				Name:   makeEventSectionName(codeName, t),
				Code:   codeVal,
				Type:   t,
				Schema: *s,
			},
		)
	}
	if len(ederr.messages) == 0 {
		return e, nil
	}
	return e, ederr
}

const tableTempl = `{{ range . }}
### {{ .Name }}

{{ .Schema.Description }}

**Event:** {{ .Type }}

**Code:** {{ .Code }}

|Field name|Type|Description|
|---|---|---|
{{- range .Schema.Fields }}
|{{.Name}}|{{.Type}}|{{.Description}}|
{{- end }}
{{ end}}`

// makeReferenceTables writes the final audit event reference to out using the
// provided data. The reference is a series of H3 headings that we expect to be
// imported as a partial within an H2 section of an existing docs page.
func makeReferenceTables(
	out io.Writer,
	data EventEntryCollection,
) error {
	fmt.Fprint(
		out,
		`{/* 
AUTOMATICALLY GENERATED FILE. Edit at:
build.assets/tooling/cmd/gen-event-reference
*/}
`)
	sort.Sort(data)
	for i, d := range data {
		d.Schema = flattenEvent(d.Schema)
		data[i] = d
	}
	return template.Must(template.New("").Parse(tableTempl)).Execute(out, data)
}

func flattenEvent(e eventschema.Event) eventschema.Event {
	fs := flattenFields("", e.Fields)
	return eventschema.Event{
		Description: e.Description,
		Fields:      fs,
	}
}

// flattenFields turns all nested fields within the provided slice into elements
// of that slice so we can include them easily into a single table. Child fields
// include the names of their parent fields, separated by dots.
func flattenFields(parentName string, f []*eventschema.EventField) []*eventschema.EventField {
	if len(f) <= 1 {
		return f
	}

	var res []*eventschema.EventField
	for _, l := range f {
		var prefix string
		if parentName != "" {
			prefix = parentName + "."
		}
		fs := flattenFields(prefix+l.Name, l.Fields)
		res = append(res, &eventschema.EventField{
			Name:        prefix + l.Name,
			Type:        l.Type,
			Description: l.Description,
			Fields:      []*eventschema.EventField{},
		})
		res = append(res, fs...)
	}
	return res
}

func main() {
	types := flag.String("types", "", "path to a file containing event type constant declarations")
	codes := flag.String("codes", "", "path to a file containing event code constant declarations")
	out := flag.String("out", "", "path to the output file")
	flag.Parse()

	tf, err := os.Open(*types)
	if err != nil {
		fmt.Fprintf(os.Stderr, "could not open %v: %v\n", *types, err)
		os.Exit(1)
	}

	cf, err := os.Open(*codes)
	if err != nil {
		fmt.Fprintf(os.Stderr, "could not open %v: %v\n", *codes, err)
		os.Exit(1)
	}

	es := eventschema.AllEventSchemas()

	of, err := os.Create(*out)
	if err != nil {
		fmt.Fprintf(os.Stderr, "could not create an output file: %v\n", err)
		os.Exit(1)
	}

	e, err := makeEventEntries(cf, tf, es)
	_, ok := err.(eventDataError)
	if err != nil && !ok {
		fmt.Fprintf(os.Stderr, "could not assemble audit event data: %v\n", err)
		os.Exit(1)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "found the following issues generating an audit event reference: %v\n", err)
	}

	if err := makeReferenceTables(of, e); err != nil {
		fmt.Fprintf(os.Stderr, "could not produce an audit event reference: %v\n", err)
		os.Exit(1)
	}
}
