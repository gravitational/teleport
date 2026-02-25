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

type shortcutEntry struct {
	Alias string // e.g. "webhooks"
	Kind  string // e.g. "webhook"
}

type shortcutGatheringData struct {
	Shortcuts []shortcutEntry
}

var shortcutGatheringTmpl = mustReadTemplate("shortcut_gathering.go.tmpl")

// GenerateShortcutGathering renders lib/services/shortcuts.gen.go with a map
// of auto-derived aliases for all generated resources. Each resource gets its
// kind name and a pluralized form as aliases.
func GenerateShortcutGathering(specs []spec.ResourceSpec) (string, error) {
	entries := make([]shortcutEntry, 0, len(specs)*2)
	for _, rs := range specs {
		plural := spec.PascalToSnake(pluralize(rs.KindPascal))
		entries = append(entries,
			shortcutEntry{Alias: rs.Kind, Kind: rs.Kind},
			shortcutEntry{Alias: plural, Kind: rs.Kind},
		)
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Alias < entries[j].Alias
	})

	data := shortcutGatheringData{Shortcuts: entries}
	out, err := render("shortcutGathering", shortcutGatheringTmpl, data)
	if err != nil {
		return "", trace.Wrap(err)
	}
	return out, nil
}
