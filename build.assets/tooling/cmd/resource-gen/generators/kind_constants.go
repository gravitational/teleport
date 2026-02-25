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
	"sort"

	"github.com/gravitational/teleport/build.assets/tooling/cmd/resource-gen/spec"
	"github.com/gravitational/trace"
)

type kindEntry struct {
	ExportedName string
	Lower        string
}

type kindConstantsData struct {
	Kinds []kindEntry
}

var kindConstantsTmpl = mustReadTemplate("kind_constants.go.tmpl")

// GenerateKindConstants renders a single file with Kind* constants for all
// generated resources.
func GenerateKindConstants(specs []spec.ResourceSpec) (string, error) {
	entries := make([]kindEntry, 0, len(specs))
	for _, rs := range specs {
		entries = append(entries, kindEntry{
			ExportedName: rs.KindPascal,
			Lower:        rs.Kind,
		})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Lower < entries[j].Lower
	})

	data := kindConstantsData{Kinds: entries}
	out, err := render("kindConstants", kindConstantsTmpl, data)
	if err != nil {
		return "", trace.Wrap(err)
	}
	return out, nil
}
