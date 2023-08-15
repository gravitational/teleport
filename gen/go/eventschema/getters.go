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
	"sort"
	"strings"

	"github.com/gravitational/trace"

	events2 "github.com/gravitational/teleport/lib/events"
)

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

func getMessageName(eventStruct interface{}) string {
	if t := reflect.TypeOf(eventStruct); t.Kind() == reflect.Ptr {
		return t.Elem().Name()
	} else {
		return t.Name()
	}
}

func TableList() (string, error) {
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

func (event *Event) TableSchema() (string, error) {
	sb := strings.Builder{}
	sb.WriteString("column_name, column_type, description\n")
	sb.WriteString("event_date, date, is the event date\n")
	sb.WriteString("event_time, timestamp, is the event time\n")
	err := iterateOverFields(event.Fields, func(propName string, prop *EventField) error {
		line, err := prop.TableSchema([]string{propName})
		if err != nil {
			return trace.Wrap(err)
		}
		sb.WriteString(line)
		return nil
	})
	return sb.String(), trace.Wrap(err)
}

func (field *EventField) TableSchema(path []string) (string, error) {
	sb := strings.Builder{}
	switch field.Type {
	case "object":
		err := iterateOverFields(field.Fields, func(propName string, prop *EventField) error {
			line, err := prop.TableSchema(append(path, propName))
			if err != nil {
				return trace.Wrap(err)
			}
			sb.WriteString(line)
			return nil
		})
		if err != nil {
			return "", trace.Wrap(err)
		}
	case "string", "integer", "boolean", "date-time", "array":
		sb.WriteString(tableSchemaLine(viewFieldName(path), field.dmlType(), field.Description))
	default:
		return "", trace.NotImplemented("field type '%s' not supported", field.Type)
	}
	return sb.String(), nil
}

func (event *Event) ViewSchema() (string, error) {
	sb := strings.Builder{}
	sb.WriteString("SELECT\n")
	sb.WriteString("  event_date, event_time\n")
	err := iterateOverFields(event.Fields, func(propName string, prop *EventField) error {
		line, err := prop.ViewSchema([]string{propName})
		if err != nil {
			return trace.Wrap(err)
		}
		sb.WriteString(line)
		return nil
	})
	return sb.String(), trace.Wrap(err)
}

func (field *EventField) ViewSchema(path []string) (string, error) {
	sb := strings.Builder{}
	switch field.Type {
	case "object":
		err := iterateOverFields(field.Fields, func(propName string, prop *EventField) error {
			line, err := prop.ViewSchema(append(path, propName))
			if err != nil {
				return trace.Wrap(err)
			}
			sb.WriteString(line)
			return nil
		})
		if err != nil {
			return "", trace.Wrap(err)
		}
	case "string", "integer", "boolean", "date-time", "array":
		sb.WriteString(viewSchemaLine(jsonFieldName(path), viewFieldName(path), field.dmlType()))
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
		for name, subField := range field.Fields {
			rowTypes = append(rowTypes, fmt.Sprintf("%s %s", name, subField.dmlType()))
		}
		return fmt.Sprintf("row(%s)", strings.Join(rowTypes, ", "))
	default:
		// If all else fails, we cast as a string, at last this is usable
		return "varchar"
	}
}

// We use the $["foo"]["bar"] syntax instead of the $.foo.bar syntax because
// foo and bar can contain dots and the second syntax would break (and we do
// have event fields with got in their name)
func jsonFieldName(path []string) string {
	sb := strings.Builder{}
	sb.WriteString("$")
	for _, item := range path {
		sb.WriteString(fmt.Sprintf(`["%s"]`, item))
	}
	return sb.String()
}

func viewFieldName(path []string) string {
	return strings.ReplaceAll(strings.Join(path, "_"), ".", "_")
}

func viewSchemaLine(jsonField, viewField, fieldType string) string {
	return fmt.Sprintf("  , CAST(json_extract(event_data, '%s') AS %s) as %s\n", jsonField, fieldType, viewField)
}

// iterateOverFields iterates over Event or EventField fields while ensuring
// the field order is consistent.
func iterateOverFields(fields map[string]*EventField, fn func(name string, prop *EventField) error) error {
	fieldNames := make([]string, 0, len(fields))
	for name, _ := range fields {
		fieldNames = append(fieldNames, name)
	}

	sort.Strings(fieldNames)
	var err error

	for _, name := range fieldNames {
		err = fn(name, fields[name])
		if err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}
