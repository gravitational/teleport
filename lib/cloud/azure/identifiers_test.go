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
	"testing"
)

func TestIsValidResourceGroupName(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		valid bool
		name  string
	}{
		{true, "valid"},
		{true, "valid-"},
		{true, "valid.name"},
		{true, "valid_"},
		{true, "valid("},
		{true, "valid)"},
		{true, "válíd"},
		{true, "VALIDNAME"},
		{true, "ꣲ"},

		{false, ""},
		{false, strings.Repeat("a", 91)},
		{false, "invalid."},
		{false, "invalid name"},
		{false, "invalid\tname"},
		{false, "invalid\nname"},
		{false, "invalid\\name"},
		{false, "invalid*name"},
		{false, "invalid~name"},
		{false, "invalid`name"},
		{false, "invalid'name"},
		{false, "invalid'name"},
		{false, "invalid\"name"},
		{false, "invalid@name"},
		{false, "invalid$name"},
	} {
		if err := IsValidResourceGroupName(tc.name); (err == nil) != tc.valid {
			t.Errorf("IsValidResourceGroupName(%q) = %v, want valid=%v", tc.name, err, tc.valid)
		}
	}
}

// Azure UI has the following javascript in the page's source code to validate resource group names:
// const c = ["\\s", "~", "!", "@", "#", "$", "%", "^", "&", "*", "+", "=", "<", ">", ",", "`", "\\?", "/", "\\\\", "\\:", ";", "'", '"', "\\[", "\\]", "\\{", "\\}", "\\|"].join("")
// u = new RegExp(`^[^${c}]*[^.${c}]$`,"i")
// This test ensures that our IsValidResourceGroupName function rejects the same characters as the Azure UI, and thus behaves consistently with it.
// Note that this regex is not exactly the same as the one in IsValidResourceGroupName, which also checks for length and non-empty string, but this test focuses on the character restrictions.
func TestIsValidResourceGroupName_usingUIRegex(t *testing.T) {
	t.Parallel()
	const invalidChars = " \t\n~!@#$%^&*+=<>,`?/\\:;'\"[]{}|"
	for _, char := range invalidChars {
		name := "invalid" + string(char) + "name"
		if err := IsValidResourceGroupName(name); err == nil {
			t.Errorf("IsValidResourceGroupName(%q) = nil, want error", name)
		}
	}
}

func TestIsValidLocationNameWeak(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		valid bool
		name  string
	}{
		{true, "eastus"},
		{true, "eastus2"},
		{true, "westeurope"},
		{true, "uksouth"},

		{false, ""},
		{false, "invalid location"},
	} {
		if err := IsValidLocationNameWeak(tc.name); (err == nil) != tc.valid {
			t.Errorf("IsValidLocationNameWeak(%q) = %v, want valid=%v", tc.name, err, tc.valid)
		}
	}
}
