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
	"fmt"
	"strings"
	"text/template"
	"unicode"

	"github.com/gravitational/teleport/build.assets/tooling/cmd/resource-gen/spec"
	"github.com/gravitational/trace"
)

type webEventTSEntry struct {
	UpperSnakeName string
	Code           string
	EventType      string
	DisplayName    string
	Article        string
	PastVerb       string
	Lower          string
	OpLower        string
	FixtureUID     string
}

type webEventsTSData struct {
	Events []webEventTSEntry
}

var webEventsTSTmpl = mustReadTemplate("web_events_ts.ts.tmpl")

// GenerateWebEventsTS renders the TypeScript file containing event codes,
// formatters, icons, and fixtures for all resources that have audit events.
func GenerateWebEventsTS(specs []spec.ResourceSpec) (string, error) {
	entries := buildWebEventEntries(specs)
	data := webEventsTSData{Events: entries}
	out, err := renderTS("webEventsTS", webEventsTSTmpl, data)
	if err != nil {
		return "", trace.Wrap(err)
	}
	return out, nil
}

func buildWebEventEntries(specs []spec.ResourceSpec) []webEventTSEntry {
	goEntries := buildEventEntries(specs)
	tsEntries := make([]webEventTSEntry, 0, len(goEntries))
	pascalByKind := map[string]string{}
	for _, rs := range specs {
		pascalByKind[rs.Kind] = rs.KindPascal
	}
	for i, e := range goEntries {
		pascal := pascalByKind[e.Lower]
		display := pascalToDisplayName(pascal)
		tsEntries = append(tsEntries, webEventTSEntry{
			UpperSnakeName: pascalToUpperSnake(e.ConstName),
			Code:           e.Code,
			EventType:      fmt.Sprintf("resource.%s.%s", e.Lower, e.OpLower),
			DisplayName:    display,
			Article:        article(display),
			PastVerb:       pastVerb(e.OpLower),
			Lower:          e.Lower,
			OpLower:        e.OpLower,
			FixtureUID:     fmt.Sprintf("00000000-0000-0000-0000-%012d", i+1),
		})
	}
	return tsEntries
}

func pascalToUpperSnake(s string) string {
	var result strings.Builder
	for i, r := range s {
		if unicode.IsUpper(r) && i > 0 {
			prev := rune(s[i-1])
			if unicode.IsLower(prev) {
				result.WriteRune('_')
			}
		}
		result.WriteRune(unicode.ToUpper(r))
	}
	return result.String()
}

func pascalToDisplayName(s string) string {
	var result strings.Builder
	for i, r := range s {
		if unicode.IsUpper(r) && i > 0 {
			prev := rune(s[i-1])
			if unicode.IsLower(prev) {
				result.WriteRune(' ')
			}
		}
		result.WriteRune(r)
	}
	return result.String()
}

func article(displayName string) string {
	if len(displayName) == 0 {
		return "a"
	}
	switch unicode.ToLower(rune(displayName[0])) {
	case 'a', 'e', 'i', 'o', 'u':
		return "an"
	default:
		return "a"
	}
}

func pastVerb(op string) string {
	switch op {
	case "create":
		return "created"
	case "update":
		return "updated"
	case "delete":
		return "deleted"
	case "get":
		return "read"
	default:
		return op + "d"
	}
}

func renderTS(name string, tmpl string, data any) (string, error) {
	t, err := template.New(name).Funcs(template.FuncMap{
		"lower": strings.ToLower,
		"title": func(s string) string {
			if len(s) == 0 {
				return s
			}
			return strings.ToUpper(s[:1]) + s[1:]
		},
	}).Parse(tmpl)
	if err != nil {
		return "", trace.Wrap(err)
	}
	var b strings.Builder
	if err := t.Execute(&b, data); err != nil {
		return "", trace.Wrap(err)
	}
	return b.String(), nil
}
