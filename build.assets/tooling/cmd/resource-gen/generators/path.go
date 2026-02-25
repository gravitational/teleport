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
	"strings"

	"github.com/gravitational/teleport/build.assets/tooling/cmd/resource-gen/spec"
)

// ServicePathParts returns path components used for auth service code generation.
// For "teleport.widget.v1.WidgetService" it returns ("widget", "widgetv1").
// Underscores in proto package names are stripped to match Go naming conventions
// (e.g. "access_policy" → "accesspolicy").
func ServicePathParts(rs spec.ResourceSpec) (resource string, pkgDir string) {
	parts := strings.Split(rs.ServiceName, ".")
	if len(parts) >= 4 {
		resource = stripUnderscores(strings.ToLower(parts[len(parts)-3]))
		version := strings.ToLower(parts[len(parts)-2])
		return resource, resource + version
	}

	return strings.ToLower(rs.Kind), strings.ToLower(rs.Kind) + "v1"
}

// protoGoImportPath returns the Go import path for the proto-generated package.
// Underscores in proto package segments are stripped to match Go conventions
// (e.g. "access_policy" → "accesspolicy").
func protoGoImportPath(serviceName, module string) string {
	parts := strings.Split(serviceName, ".")
	if len(parts) < 2 {
		return ""
	}
	pkgParts := parts[:len(parts)-1]
	for i, p := range pkgParts {
		pkgParts[i] = stripUnderscores(p)
	}
	return module + "/api/gen/proto/go/" + strings.Join(pkgParts, "/")
}

// protoPackageAlias returns the Go package alias for proto imports.
// Underscores are stripped (e.g. "access_policy" → "accesspolicyv1").
func protoPackageAlias(serviceName string) string {
	parts := strings.Split(serviceName, ".")
	if len(parts) >= 4 {
		resource := stripUnderscores(strings.ToLower(parts[len(parts)-3]))
		version := strings.ToLower(parts[len(parts)-2])
		return resource + version
	}
	return ""
}

// stripUnderscores removes all underscores from a string.
func stripUnderscores(s string) string {
	return strings.ReplaceAll(s, "_", "")
}

// serviceShortName extracts the service type name from a fully qualified service name.
func serviceShortName(serviceName string) string {
	parts := strings.Split(serviceName, ".")
	return parts[len(parts)-1]
}

func exportedName(kind string) string {
	if kind == "" {
		return ""
	}
	return strings.ToUpper(kind[:1]) + kind[1:]
}

func pluralize(s string) string {
	lower := strings.ToLower(s)
	if strings.HasSuffix(lower, "s") || strings.HasSuffix(lower, "x") || strings.HasSuffix(lower, "z") {
		return s + "es"
	}
	if strings.HasSuffix(lower, "ch") || strings.HasSuffix(lower, "sh") {
		return s + "es"
	}
	if strings.HasSuffix(lower, "y") && len(s) > 1 {
		c := lower[len(lower)-2]
		if c != 'a' && c != 'e' && c != 'i' && c != 'o' && c != 'u' {
			return s[:len(s)-1] + "ies"
		}
	}
	return s + "s"
}

// snakeToCamel converts a snake_case string to CamelCase (e.g. "power_level" → "PowerLevel").
func snakeToCamel(s string) string {
	parts := strings.Split(s, "_")
	var b strings.Builder
	for _, p := range parts {
		if p == "" {
			continue
		}
		b.WriteString(strings.ToUpper(p[:1]) + p[1:])
	}
	return b.String()
}

// snakeToTitle converts a snake_case string to a human-readable title.
func snakeToTitle(s string) string {
	parts := strings.Split(s, "_")
	for i, p := range parts {
		if p != "" {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, " ")
}

// columnDef describes a tctl table column.
type columnDef struct {
	Header      string
	Getter      string
	IsTimestamp bool
}

// resolveColumns converts dot-separated field paths to column definitions.
// timestampSet contains full dot-paths (e.g. "status.last_delivery") that should
// be formatted as timestamps beyond the built-in "expires" detection.
func resolveColumns(paths []string, timestampSet map[string]bool) []columnDef {
	cols := make([]columnDef, 0, len(paths))
	for _, path := range paths {
		segments := strings.Split(path, ".")
		last := segments[len(segments)-1]
		header := snakeToTitle(last)

		var getterParts []string
		for _, seg := range segments {
			getterParts = append(getterParts, "Get"+snakeToCamel(seg)+"()")
		}
		getter := "r." + strings.Join(getterParts, ".")

		isTS := last == "expires" || timestampSet[path]
		cols = append(cols, columnDef{Header: header, Getter: getter, IsTimestamp: isTS})
	}
	return cols
}
