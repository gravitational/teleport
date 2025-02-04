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

// A tool that dumps preset roles in a JSON file that will later be used in
// TypeScript tests to make sure that the standard role editor can
// unambiguously represent a preset role.

package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
)

const filePath = "gen/preset-roles.json"

func main() {
	access := services.NewPresetAccessRole()
	editor := services.NewPresetEditorRole()
	auditor := services.NewPresetAuditorRole()

	rolesByName := map[string]types.Role{
		access.GetName():  access,
		editor.GetName():  editor,
		auditor.GetName(): auditor,
	}

	for _, r := range rolesByName {
		err := services.CheckAndSetDefaults(r)
		if err != nil {
			log.Fatalf("Could not set default values: %s", err)
		}
	}

	rolesJSON, err := json.Marshal(rolesByName)
	if err != nil {
		log.Fatalf("Could not marshal preset roles as JSON: %s", err)
	}

	if err = os.WriteFile(filePath, rolesJSON, 0744); err != nil {
		log.Fatalf("Could not write JSON for preset roles: %s", err)
	}

	fmt.Printf("Successfully recreated %s\n", filePath)
}
