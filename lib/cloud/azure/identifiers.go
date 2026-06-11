/*
 * Teleport
 * Copyright (C) 2026 Gravitational, Inc.
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

package azure

import (
	"strings"
	"unicode"

	"github.com/gravitational/trace"
)

// IsValidResourceGroupName validates that the provided name is a valid Azure Resource Group name.
// Azure defines the following rules for resource group names:
// https://learn.microsoft.com/en-us/rest/api/resources/resource-groups/create-or-update
//
// > The name of the resource group to create or update.
// > Can include alphanumeric, underscore, parentheses, hyphen, period (except at end), and Unicode characters that match the allowed characters.
// > minLength: 1
// > maxLength: 90
// > pattern: ^[-\w\._\(\)]+$
//
// In this scenario, \w includes characters like á, à, ã, ꣲ, ...
func IsValidResourceGroupName(name string) error {
	const allowedSymbols = "-._()"
	if name == "" || len(name) > 90 {
		return trace.BadParameter("invalid resource group name")
	}

	for i, r := range name {
		// Period is not allowed at the end of the name.
		if r == '.' && i == len(name)-1 {
			return trace.BadParameter("invalid resource group name")
		}

		if unicode.IsLetter(r) || unicode.IsNumber(r) || strings.ContainsRune(allowedSymbols, r) {
			continue
		}

		return trace.BadParameter("invalid resource group name")
	}

	return nil
}

// IsValidLocationNameWeak validates that the provided name looks to be a valid Azure Location name.
// Currently active regions are listed here:
// https://learn.microsoft.com/en-us/azure/reliability/regions-list?tabs=all
// This is a weak validation that only checks for well formedness of the name, not that it corresponds to an actual Azure region.
func IsValidLocationNameWeak(name string) error {
	if name == "" {
		return trace.BadParameter("invalid location name")
	}

	for _, r := range name {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' {
			continue
		}

		return trace.BadParameter("invalid location name")
	}

	return nil
}
