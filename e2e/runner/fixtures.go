/**
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

package main

import (
	"flag"
)

var allFixtures []*fixture

type fixture struct {
	name    string
	usage   string
	enabled bool
}

// registerFixture declares a new optional piece of test infrastructure
// (like an SSH node) and adds it to the global registry so it gets a --with-<name> flag.
func registerFixture(name, usage string) *fixture {
	f := &fixture{name: name, usage: usage}

	allFixtures = append(allFixtures, f)

	return f
}

func bindFixtureFlags(fs *flag.FlagSet) {
	for _, f := range allFixtures {
		fs.BoolVar(&f.enabled, "with-"+f.name, false, f.usage)
	}
}

func enableAllFixtures() {
	for _, f := range allFixtures {
		f.enabled = true
	}
}
