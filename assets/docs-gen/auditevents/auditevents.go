package auditevents

import (
	"fmt"
	"go/ast"
	"io"
	"os"
	"reflect"
	"sort"
	"strings"
	"text/template"

	"github.com/gravitational/teleport/api/types/events"
	"golang.org/x/tools/go/ast/astutil"
)

type EventData struct {
	Name    string
	Comment string
}

type AuditTableData struct {
	SortedNames []string
	// Each key is an event type name. Each value is its comment.
	NamesToComments map[string]string
}

// To be executed with an AuditTableData
var tmpl string = `|Event Type|Description|
|---|---|
{{- $m := .NamesToComments -}}
{{- range .SortedNames }}
|**{{.}}**|{{index $m . }}|
{{- end }}
`

// auditEventTypeMap maps the names of emitted audit events to an empty struct,
// letting us check whether a given type corresponds to an emitted audit event.
type auditEventTypeMap map[string]struct{}

// getEmittedAuditEventsFromFile returns a slice of the type names of audit
// events emitted within a given file, e.g., "cert.create".
func getEmittedAuditEventsFromFile(f *ast.File) []string {
	ft := make(map[string]struct{})
	mf := reflect.VisibleFields(reflect.ValueOf(events.Metadata{}).Type())
	for _, sf := range mf {
		ft[sf.Name] = struct{}{}
	}

	t := []string{}
	for _, d := range f.Decls {
		astutil.Apply(d, func(c *astutil.Cursor) bool {
			// While different struct types implement the AuditEvent
			// interface, all of these types include a Metadata
			// field with the apiEvents.Metadata type.
			//
			// We're looking for a KeyValueExpr
			// "Metadata: apievents.Metadata{}"
			kv, ok := c.Node().(*ast.KeyValueExpr)
			if !ok {
				return true
			}

			if ki, ok := kv.Key.(*ast.Ident); !ok || ki.Name != "Metadata" {
				// This can't be the Metadata field of an audit
				// event, since it's not an identifier named "Metadata"
				return true
			}

			// The value of the KeyValueExpression must be an
			// apievents.Metadata struct
			vl, ok := kv.Value.(*ast.CompositeLit)
			if !ok {
				return true
			}

			vt, ok := vl.Type.(*ast.SelectorExpr)
			if !ok {
				return true
			}

			if vt.Sel.Name != "Metadata" {
				return true
			}

			// Current best candidate for the Metadata type name
			var tn string
			// Make sure the value of the KeyValueExpression is an
			// audit event Metadata type by inspecting its fields
			for _, el := range vl.Elts {
				elkv, ok := el.(*ast.KeyValueExpr)
				if !ok {
					tn = ""
					continue
				}
				elkvk, ok := elkv.Key.(*ast.Ident)
				if !ok {
					tn = ""
					continue
				}

				// The Metadata has a field that doesn't belong
				if _, ok := ft[elkvk.Name]; !ok {
					tn = ""
					continue
				}

				if elkvk.Name == "Type" {
					elkvv, ok := elkv.Value.(*ast.SelectorExpr)
					if !ok {
						tn = ""
						continue
					}
					// This seems like an audit event type,
					// so record it temporarily in case the
					// other fields are invalid.
					tn = elkvv.Sel.Name
				}
			}
			if tn != "" {
				t = append(t, tn)
			}

			return true
		}, nil)
	}
	return t
}

// getDataForEventTypes takes a map of audit event type names and extracts
// information from the provided file for those audit event types.
func getDataForEventTypes(f *ast.File, m auditEventTypeMap) []EventData {
	eventData := []EventData{}
	for _, d := range f.Decls {
		astutil.Apply(d, func(c *astutil.Cursor) bool {
			// Look through all declarations and find those that match
			// the identifiers we have collected.
			val, ok := c.Node().(*ast.ValueSpec)
			if !ok {
				return true
			}
			for _, n := range val.Names {
				if _, y := m[n.Name]; y {
					tx := strings.ReplaceAll(val.Doc.Text(), "\n", " ")
					s := val.Values[0].(*ast.BasicLit).Value
					nm := strings.ReplaceAll(s, "\"", "")
					typ := strings.ReplaceAll(s, "\"", "`")
					if strings.HasPrefix(tx, n.Name) {
						tx = strings.ReplaceAll(tx, n.Name, typ)
					}
					eventData = append(eventData, EventData{
						Name:    nm,
						Comment: tx,
					})
				}
			}
			return true
		}, nil)
	}
	return eventData
}

// GenerateAuditEventsTable writes a table of Teleport audit events to out
// based on the parsed Go source files in gofiles.
func GenerateAuditEventsTable(out io.Writer, gofiles []*ast.File) error {
	eventTypes := make(auditEventTypeMap)
	eventData := []EventData{}
	tableData := AuditTableData{
		SortedNames:     []string{},
		NamesToComments: make(map[string]string),
	}

	// We will traverse the AST of each Go file twice: once to collect types of
	// audit events that are used in apievents.Metadata declarations, and
	// again to see where those audit event types are declared. In the second
	// traversal, we'll collect the string values of those event types along
	// their godoc comments.

	// First walk through the AST: collect types of audit events.
	// We identify audit event types by instances where a field named
	// "Metadata" is assigned to a composite literal with type
	// "Metadata". Further, that Metadata composite literal has a
	// field called "Type".
	for _, f := range gofiles {
		for _, t := range getEmittedAuditEventsFromFile(f) {
			eventTypes[t] = struct{}{}
		}
	}

	// Second walk through the AST: find definitions of audit event
	// types by comparing them to the audit event types we collected
	// in the first walk. Gather the comments.
	for _, f := range gofiles {
		eventData = append(eventData, getDataForEventTypes(f, eventTypes)...)
	}

	for _, d := range eventData {
		tableData.SortedNames = append(tableData.SortedNames, d.Name)
		tableData.NamesToComments[d.Name] = d.Comment
	}

	sort.Strings(tableData.SortedNames)

	tt, err := template.New("table").Parse(tmpl)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error parsing the audit event reference template: %v", err)
		os.Exit(1)
	}
	if err := tt.Execute(out, tableData); err != nil {
		return fmt.Errorf("error executing the audit event reference template: %v", err)
	}
	return nil
}
