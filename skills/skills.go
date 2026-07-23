/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

// Package skills embeds the published Teleport agent skills so tools like tsh
// can list and install them without network access.
//
// Only the installable payload of each skill is embedded: its SKILL.md and
// references/ directory. Development-only artifacts such as evals/ (test
// fixtures and mock binaries) are deliberately excluded.
package skills

import "embed"

//go:embed teleport-*/SKILL.md
//go:embed teleport-*/references
var fsys embed.FS

// FS returns the embedded skills filesystem. Its top-level entries are skill
// directories (e.g. "teleport-discovery"), each containing a SKILL.md and a
// references/ directory.
func FS() embed.FS {
	return fsys
}
