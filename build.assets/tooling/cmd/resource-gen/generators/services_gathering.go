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

type gatheringEntry struct {
	Kind   string // PascalCase, e.g. "Foo"
	Lower  string // lowercase, e.g. "foo"
	Plural string // e.g. "Foos"
}

type servicesGatheringData struct {
	Module         string
	Resources      []gatheringEntry
	CacheResources []gatheringEntry
}

var servicesGatheringTmpl = mustReadTemplate("services_gathering.go.tmpl")

// GenerateServicesGathering renders lib/auth/services.gen.go with the
// servicesGenerated struct and its constructor for all generated resources.
func GenerateServicesGathering(specs []spec.ResourceSpec, module string) (string, error) {
	entries := buildGatheringEntries(specs)
	var cacheSpecs []spec.ResourceSpec
	for _, rs := range specs {
		if rs.Cache.Enabled {
			cacheSpecs = append(cacheSpecs, rs)
		}
	}
	cacheEntries := buildGatheringEntries(cacheSpecs)
	data := servicesGatheringData{
		Module:         module,
		Resources:      entries,
		CacheResources: cacheEntries,
	}
	out, err := render("servicesGathering", servicesGatheringTmpl, data)
	if err != nil {
		return "", trace.Wrap(err)
	}
	return out, nil
}

func buildGatheringEntries(specs []spec.ResourceSpec) []gatheringEntry {
	entries := make([]gatheringEntry, 0, len(specs))
	for _, rs := range specs {
		kind := rs.KindPascal
		entries = append(entries, gatheringEntry{
			Kind:   kind,
			Lower:  rs.Kind,
			Plural: pluralize(kind),
		})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Lower < entries[j].Lower
	})
	return entries
}
