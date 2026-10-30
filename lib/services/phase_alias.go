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

package services

import (
	"regexp"

	"github.com/gravitational/trace"
	"k8s.io/apimachinery/pkg/util/validation"
)

// MaxPhaseNameLength leaves room for the hyphen and eight-character build
// alias suffix appended to a Phase name in a hostname.
const MaxPhaseNameLength = 54

var buildBeamAliasRegexp = regexp.MustCompile(`^(.+)-[a-f0-9]{8}$`)

// BuildBeamPhase extracts the target Phase from a build Beam alias.
// Callers must also verify the Phase's active build ID before acting on it.
func BuildBeamPhase(alias string) (string, error) {
	parts := buildBeamAliasRegexp.FindStringSubmatch(alias)
	if len(parts) != 2 || len(parts[1]) > MaxPhaseNameLength || len(validation.IsDNS1123Label(parts[1])) != 0 {
		return "", trace.BadParameter("invalid build Beam alias %q", alias)
	}
	return parts[1], nil
}
