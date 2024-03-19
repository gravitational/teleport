/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/awalterschulze/goderive/derive"

	"github.com/gravitational/teleport/build.assets/tooling/cmd/goderive/plugin/teleportequal"
)

func main() {
	// Establish Teleport derive plugins of interest.
	plugins := []derive.Plugin{
		teleportequal.NewPlugin(),
	}

	// Parse args, which are just paths at the moment..
	flag.Parse()
	paths := derive.ImportPaths(flag.Args())

	// Load the given paths into the generator.
	g, err := derive.NewPlugins(plugins, false, false).Load(paths)
	if err != nil {
		fmt.Printf("Error creating new plugins: %v\n", err)
		os.Exit(1)
	}

	// Generate the derived code.
	if err := g.Generate(); err != nil {
		fmt.Printf("Error generating code: %v\n", err)
		os.Exit(1)
	}
}
