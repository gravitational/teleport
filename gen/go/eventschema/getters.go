// Copyright 2023 Gravitational, Inc
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

// ViewSchema returns the AthenaSQL statement used to build a view extracting
// content from the raw event data.
func (event *Event) ViewSchema() (string, error) {
	sb := strings.Builder{}
	sb.WriteString("SELECT\n")
	sb.WriteString("  event_date, event_time\n")

	for _, prop := range event.Fields {
		line, err := prop.ViewSchema([]string{prop.Name})
		if err != nil {
			return "", trace.Wrap(err)
		}
		sb.WriteString(line)
	}
	return sb.String(), nil
}

// ViewSchema returns the AthenaSQL statement used to build a view extracting
// content from the raw event data.
func (field *EventField) ViewSchema(path []string) (string, error) {
	sb := strings.Builder{}
	switch field.Type {
	case "object":
		for _, prop := range field.Fields {
			line, err := prop.ViewSchema(append(path, prop.Name))
			if err != nil {
				return "", trace.Wrap(err)
			}
			sb.WriteString(line)
		}

	case "string", "integer", "boolean", "date-time", "array":
		sb.WriteString(viewSchemaLine(jsonFieldName(path), sqlFieldName(path), field.dmlType()))
	default:
		return "", trace.NotImplemented("field type '%s' not supported", field.Type)
	}
	return sb.String(), nil
}

func tableSchemaLine(columnName, columnType, description string) string {
	return fmt.Sprintf("%s, %s, %s\n", columnName, columnType, description)
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
