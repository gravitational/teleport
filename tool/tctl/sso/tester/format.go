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

package tester

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ghodss/yaml"

	"github.com/gravitational/teleport/api/types"
)

func FormatString(description string, msg string) string {
	return fmt.Sprintf("%v:\n%v\n", description, msg)
}

func FormatYAML(description string, object interface{}) string {
	output, err := yaml.Marshal(object)
	if err != nil {
		return formatError(description, err)
	}
	return fmt.Sprintf("%v:\n%v", description, string(output))
}

func FormatJSON(description string, object interface{}) string {
	output, err := json.MarshalIndent(object, "", "    ")
	if err != nil {
		return formatError(description, err)
	}
	return fmt.Sprintf("%v:\n%v\n", description, string(output))
}

func formatUserDetails(description string, info *types.CreateUserParams) string {
	if info == nil {
		return ""
	}

	// Skip fields: connector_name, session_ttl
	info.ConnectorName = ""
	info.SessionTTL = 0

	output, err := yaml.Marshal(info)
	if err != nil {
		return formatError(description, err)
	}
	return fmt.Sprintf("%v:\n%v", description, Indent(string(output), 3))
}

func formatError(fieldDesc string, err error) string {
	return fmt.Sprintf("%v: error rendering field: %v\n", fieldDesc, err)
}

func formatSSOWarnings(description string, info *types.SSOWarnings) string {
	if info == nil {
		return ""
	}

	if len(info.Warnings) > 0 {
		return fmt.Sprintf("%v: %v. Warnings:\n%v\n", description, info.Message, Indent(strings.Join(info.Warnings, "\n"), 2))
	}

	return fmt.Sprintf("%v: %v\n", description, info.Message)
}
