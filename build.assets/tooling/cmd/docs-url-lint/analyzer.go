// Teleport
// Copyright (C) 2026 Gravitational, Inc.
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

package main

import "golang.org/x/tools/go/analysis"

// Analyzer reports errors, constructed in a curated set of CLI functions
// (see Targets), whose message doesn't reference Teleport documentation.
var Analyzer = &analysis.Analyzer{
	Name: "docsurllint",
	Doc:  "reports errors in scoped CLI functions that don't reference Teleport documentation",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, f := range checkTargets(pass, Targets(), ErrorConstructors()) {
		pass.Reportf(f.Pos, "%s", f.Message)
	}
	return nil, nil
}
