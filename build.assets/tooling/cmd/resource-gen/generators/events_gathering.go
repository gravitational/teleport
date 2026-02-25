/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package generators

import (
	"sort"

	"github.com/gravitational/teleport/build.assets/tooling/cmd/resource-gen/spec"
	"github.com/gravitational/trace"
)

type eventEntry struct {
	ConstName string // e.g. "CookieCreate"
	Lower     string // e.g. "cookie"
	OpLower   string // e.g. "create"
	Code      string // e.g. "CK001I"
}

type eventsAPIData struct {
	Events []eventEntry
}

var eventsAPITmpl = mustReadTemplate("events_api.go.tmpl")

// GenerateEventsAPI renders lib/events/api.gen.go with event type string
// constants for all resources that have audit events enabled.
func GenerateEventsAPI(specs []spec.ResourceSpec) (string, error) {
	entries := buildEventEntries(specs)
	if len(entries) == 0 {
		return "package events\n", nil
	}
	data := eventsAPIData{Events: entries}
	out, err := render("eventsAPI", eventsAPITmpl, data)
	if err != nil {
		return "", trace.Wrap(err)
	}
	return out, nil
}

type eventsCodesData struct {
	Events []eventEntry
}

var eventsCodesTmpl = mustReadTemplate("events_codes.go.tmpl")

// GenerateEventsCodes renders lib/events/codes.gen.go with event code
// constants for all resources that have audit events enabled.
func GenerateEventsCodes(specs []spec.ResourceSpec) (string, error) {
	entries := buildEventEntries(specs)
	if len(entries) == 0 {
		return "package events\n", nil
	}
	data := eventsCodesData{Events: entries}
	out, err := render("eventsCodes", eventsCodesTmpl, data)
	if err != nil {
		return "", trace.Wrap(err)
	}
	return out, nil
}

type eventsDynamicData struct {
	Module string
	Events []eventEntry
}

var eventsDynamicTmpl = mustReadTemplate("events_dynamic.go.tmpl")

// GenerateEventsDynamic renders lib/events/dynamic.gen.go with init()
// registrations mapping event type strings to empty struct constructors.
func GenerateEventsDynamic(specs []spec.ResourceSpec, module string) (string, error) {
	entries := buildEventEntries(specs)
	if len(entries) == 0 {
		return "package events\n", nil
	}
	data := eventsDynamicData{Module: module, Events: entries}
	out, err := render("eventsDynamic", eventsDynamicTmpl, data)
	if err != nil {
		return "", trace.Wrap(err)
	}
	return out, nil
}

type eventsTestData struct {
	Module string
	Events []eventEntry
}

var eventsTestTmpl = mustReadTemplate("events_test.go.tmpl")

// GenerateEventsTest renders lib/events/events_test.gen.go with init()
// registrations for the test event coverage map.
func GenerateEventsTest(specs []spec.ResourceSpec, module string) (string, error) {
	entries := buildEventEntries(specs)
	if len(entries) == 0 {
		return "package events\n", nil
	}
	data := eventsTestData{Module: module, Events: entries}
	out, err := render("eventsTest", eventsTestTmpl, data)
	if err != nil {
		return "", trace.Wrap(err)
	}
	return out, nil
}

type eventsOneOfData struct {
	Events []eventEntry
}

var eventsOneOfTmpl = mustReadTemplate("events_oneof.go.tmpl")

// GenerateEventsOneOf renders api/types/events/oneof.gen.go with init()
// registrations for the ToOneOf converter.
func GenerateEventsOneOf(specs []spec.ResourceSpec) (string, error) {
	entries := buildEventEntries(specs)
	if len(entries) == 0 {
		return "package events\n", nil
	}
	data := eventsOneOfData{Events: entries}
	out, err := render("eventsOneOf", eventsOneOfTmpl, data)
	if err != nil {
		return "", trace.Wrap(err)
	}
	return out, nil
}

var eventsTrimTmpl = mustReadTemplate("events_trim.go.tmpl")

// GenerateEventsTrim renders api/types/events/trim.gen.go with TrimToMaxSize
// implementations for all resources that have audit events enabled.
func GenerateEventsTrim(specs []spec.ResourceSpec) (string, error) {
	entries := buildEventEntries(specs)
	if len(entries) == 0 {
		return "package events\n", nil
	}
	data := struct {
		Entries []eventEntry
	}{
		Entries: entries,
	}
	out, err := render("eventsTrim", eventsTrimTmpl, data)
	if err != nil {
		return "", trace.Wrap(err)
	}
	return out, nil
}

// buildEventEntries collects all enabled audit events across all resources,
// sorted by ConstName for deterministic output.
func buildEventEntries(specs []spec.ResourceSpec) []eventEntry {
	var entries []eventEntry
	for _, rs := range specs {
		kind := rs.KindPascal
		lower := rs.Kind
		prefix := rs.Audit.CodePrefix

		if rs.Audit.EmitOnCreate && rs.Operations.Create {
			entries = append(entries, eventEntry{ConstName: kind + "Create", Lower: lower, OpLower: "create", Code: prefix + "001I"})
		}
		if rs.Audit.EmitOnUpdate && (rs.Operations.Update || rs.Operations.Upsert) {
			entries = append(entries, eventEntry{ConstName: kind + "Update", Lower: lower, OpLower: "update", Code: prefix + "002I"})
		}
		if rs.Audit.EmitOnDelete && rs.Operations.Delete {
			entries = append(entries, eventEntry{ConstName: kind + "Delete", Lower: lower, OpLower: "delete", Code: prefix + "003I"})
		}
		if rs.Audit.EmitOnGet && rs.Operations.Get {
			entries = append(entries, eventEntry{ConstName: kind + "Get", Lower: lower, OpLower: "get", Code: prefix + "004I"})
		}
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].ConstName < entries[j].ConstName
	})
	return entries
}
