// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

// A tool that dumps preset roles to standard output. It is used in TypeScript
// tests to make sure that the standard role editor can unambiguously represent
// a preset role.

package main

import (
	"encoding/json"
	"log"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/services"
)

func main() {
	roles := auth.GetPresetRoles()
	rolesByName := make(map[string]types.Role)
	for _, role := range roles {
		services.CheckAndSetDefaults(role)
		rolesByName[role.GetName()] = role
	}

	rolesJSON, err := json.Marshal(rolesByName)
	if err != nil {
		log.Fatalf("Could not marshal preset roles as JSON: %s", err)
	}

	println(string(rolesJSON))
}
