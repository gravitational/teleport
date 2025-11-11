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

package eventschema

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/gravitational/trace"

	events2 "github.com/gravitational/teleport/lib/events"
)

// GetEventSchemaFromType takes an event type, looks up the corresponding
// protobuf message, and returns the message schema.
func GetEventSchemaFromType(eventType string) (*Event, error) {
	fields := events2.EventFields{"event": eventType}
	eventStruct, err := events2.FromEventFields(fields)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	messageName := getMessageName(eventStruct)
	event, ok := events[messageName]
	if !ok {
		return nil, trace.NotFound("message %s unknown", messageName)
	}
	return event, nil
}

// getMessageName takes a message struct and returns its name.
// The struct name is also the protobuf message name.
func getMessageName(eventStruct interface{}) string {
	if t := reflect.TypeOf(eventStruct); t.Kind() == reflect.Ptr {
		return t.Elem().Name()
	} else {
		return t.Name()
	}
}

// QueryableEventList generates a CSV list of all user-exposed event types that can be
// queried in Athena and their descriptions
// The list looks like
//
//	table_name, description
//	user.login, records a successfully or failed user login event
//	database.session.query, is emitted when a user executes a database query
//	...
func QueryableEventList() (string, error) {
	sb := strings.Builder{}
	sb.WriteString("table_name, description\n")
	for _, name := range eventTypes {
		eventSchema, err := GetEventSchemaFromType(name)
		if err != nil {
			return "", trace.Wrap(err)
		}
		sb.WriteString(fmt.Sprintf("%s, %s\n", name, eventSchema.Description))
	}
	return sb.String(), nil
}

// TableSchemaDetails describe Athena view schema.
type TableSchemaDetails struct {
	// Name is view name
	Name string
	// SQLViewName is SQL compatible table name.
	SQLViewName string
	// Description is Athena view description.
	Description string
	// Columns contains information about columns schema.
	Columns []*ColumnSchemaDetails
}

// CreateView returns a SQL statement to create an Athena view.
// This view is used during a user query execution to extract data from the
// raw event_data column and make data representation more user-friendly.
// CreateView result will be used as in a user SQL query as a query header
// to create dynamic tables.
// Access Monitoring allow users to run any arbitrary Athena SQL query where
// the query scope is limited to by dedicated IAM Role that allows
// only read access to Athena audit event table.
func (d *TableSchemaDetails) CreateView() string {
	sb := strings.Builder{}
	sb.WriteString("SELECT\n")
	sb.WriteString(" event_date, event_time\n")
	for _, v := range d.Columns {
		sb.WriteString(viewSchemaLine(v.NameJSON(), v.NameSQL(), v.Type))
	}
	return sb.String()
}

// SQLViewNameForEvent returns a SQL compatible view name for a given event.
// [event code]  -> [athena view name]
// session.start -> session_start
func SQLViewNameForEvent(eventName string) string {
	viewName := strings.ReplaceAll(eventName, ".", "_")
	return strings.ReplaceAll(viewName, "-", "_")
}

// ColumnSchemaDetails describe Athena view column schema.
type ColumnSchemaDetails struct {
	// Path is column field path.
	Path []string
	// Type is the column type.
	Type string
	// Description is the column description.
	Description string
}

func (c *ColumnSchemaDetails) viewSchema() string {
	return viewSchemaLine(c.NameJSON(), c.NameSQL(), c.Type)
}

func (c *ColumnSchemaDetails) tableSchema() string {
	return viewSchemaLine(c.NameJSON(), c.NameSQL(), c.Type)
}

// NameJSON returns a JSON compatible column name.
func (c *ColumnSchemaDetails) NameJSON() string {
	sb := strings.Builder{}
	sb.WriteString("$")
	for _, item := range c.Path {
		sb.WriteString(fmt.Sprintf(`["%s"]`, item))
	}
	return sb.String()
}

// NameSQL returns a SQL compatible column name.
func (c *ColumnSchemaDetails) NameSQL() string {
	return strings.ReplaceAll(strings.Join(c.Path, "_"), ".", "_")
}

// GetViewsDetails returns a list of Athena view schema.
func GetViewsDetails() ([]*TableSchemaDetails, error) {
	var out []*TableSchemaDetails
	for _, eventName := range eventTypes {
		es, err := GetEventSchemaFromType(eventName)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		tb, err := es.TableSchemaDetails()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		tb.Name = eventName
		tb.SQLViewName = SQLViewNameForEvent(eventName)
		out = append(out, tb)
	}
	return out, nil
}

// TableSchemaDetails returns a CSV description of the event table schema.
// This explains to the query writer which field are available.
// This must not be confused with ViewSchema which returns the Athena SQL
// statements used to build a view extracting content from raw event_data.
func (event *Event) TableSchemaDetails() (*TableSchemaDetails, error) {
	out := TableSchemaDetails{
		Description: event.Description,
	}
	for _, prop := range event.Fields {
		columns, err := prop.TableSchemaDetails([]string{prop.Name})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if columns == nil {
			continue
		}
		out.Columns = append(out.Columns, columns...)
	}
	return &out, nil
}

// TableSchemaDetails returns a CSV description of the EventField schema.
// This explains to the query writer which field are available.
func (field *EventField) TableSchemaDetails(path []string) ([]*ColumnSchemaDetails, error) {
	switch field.Type {
	case "object":
		var out []*ColumnSchemaDetails
		for _, prop := range field.Fields {
			r, err := prop.TableSchemaDetails(append(path, prop.Name))
			if err != nil {
				return nil, trace.Wrap(err)
			}
			out = append(out, r...)
		}
		return out, nil
	case "string", "integer", "boolean", "date-time", "array":
		return []*ColumnSchemaDetails{
			{
				Path:        path,
				Type:        field.dmlType(),
				Description: field.Description,
			},
		}, nil
	default:
		return nil, trace.NotImplemented("field type '%s' not supported", field.Type)
	}
	return nil, nil
}

func (field *EventField) dmlType() string {
	switch field.Type {
	case "string":
		return "varchar"
	case "integer":
		return "integer"
	case "boolean":
		return "boolean"
	case "date-time":
		return "timestamp"
	case "array":
		if field.Items == nil {
			return "array(varchar)"
		}
		return fmt.Sprintf("array(%s)", field.Items.dmlType())
	case "object":
		if field.Fields == nil || len(field.Fields) == 0 {
			return "varchar"
		}
		rowTypes := make([]string, 0, len(field.Fields))
		for _, subField := range field.Fields {
			rowTypes = append(rowTypes, fmt.Sprintf("%s %s", subField.Name, subField.dmlType()))
		}
		return fmt.Sprintf("row(%s)", strings.Join(rowTypes, ", "))
	default:
		// If all else fails, we cast as a string, at last this is usable
		return "varchar"
	}
}

// We use the $["foo"]["bar"] syntax instead of the $.foo.bar syntax because
// foo and bar can contain dots and the second syntax would break (and we do
// have event field with got in their name)
func jsonFieldName(path []string) string {
	sb := strings.Builder{}
	sb.WriteString("$")
	for _, item := range path {
		sb.WriteString(fmt.Sprintf(`["%s"]`, item))
	}
	return sb.String()
}

// sqlFieldName builds the field name from its path. Path components are
// joined with `_` and dots are replaced with `_`. Technically we could face a
// conflict if we had both a field named "addr.local" and nested fields "addr"
// and "local".
func sqlFieldName(path []string) string {
	return strings.ReplaceAll(strings.Join(path, "_"), ".", "_")
}

func viewSchemaLine(jsonField, viewField, fieldType string) string {
	return fmt.Sprintf("  , CAST(json_extract(event_data, '%s') AS %s) as %s\n", jsonField, fieldType, viewField)
}

// IsValidEventType takes a string and returns whether it represents a valid event type.
func IsValidEventType(input string) bool {
	for _, eventType := range eventTypes {
		if input == eventType {
			return true
		}
	}
	return false
}

// TableSchema returns a CSV description of the event table schema.
// This explains to the query writer which field are available.
// This must not be confused with ViewSchema which returns the Athena SQL
// statements used to build a view extracting content from raw event_data.
func (event *Event) TableSchema() (string, error) {
	sb := strings.Builder{}
	sb.WriteString("column_name, column_type, description\n")
	sb.WriteString("event_date, date, is the event date\n")
	sb.WriteString("event_time, timestamp, is the event time\n")
	for _, prop := range event.Fields {
		line, err := prop.TableSchema([]string{prop.Name})
		if err != nil {
			return "", trace.Wrap(err)
		}
		sb.WriteString(line)
	}
	return sb.String(), nil
}

// TableSchema returns a CSV description of the EventField schema.
// This explains to the query writer which field are available.
func (field *EventField) TableSchema(path []string) (string, error) {
	sb := strings.Builder{}
	switch field.Type {
	case "object":
		for _, prop := range field.Fields {
			line, err := prop.TableSchema(append(path, prop.Name))
			if err != nil {
				return "", trace.Wrap(err)
			}
			sb.WriteString(line)
		}
	case "string", "integer", "boolean", "date-time", "array":
		sb.WriteString(tableSchemaLine(sqlFieldName(path), field.dmlType(), field.Description))
	default:
		return "", trace.NotImplemented("field type '%s' not supported", field.Type)
	}
	return sb.String(), nil
}

func tableSchemaLine(columnName, columnType, description string) string {
	return fmt.Sprintf("%s, %s, %s\n", columnName, columnType, description)
}
