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
	"github.com/gravitational/teleport/build.assets/tooling/cmd/resource-gen/spec"
	"github.com/gravitational/trace"
)

type authclientGatheringData struct {
	Module    string
	Resources []gatheringEntry
}

var authclientGatheringTmpl = mustReadTemplate("authclient_gathering.go.tmpl")

// GenerateAuthclientGathering renders lib/auth/authclient/api.gen.go with the
// cacheGeneratedServices interface for all generated resources.
func GenerateAuthclientGathering(specs []spec.ResourceSpec, module string) (string, error) {
	var cacheSpecs []spec.ResourceSpec
	for _, rs := range specs {
		if rs.Cache.Enabled {
			cacheSpecs = append(cacheSpecs, rs)
		}
	}
	entries := buildGatheringEntries(cacheSpecs)
	data := authclientGatheringData{
		Module:    module,
		Resources: entries,
	}
	out, err := render("authclientGathering", authclientGatheringTmpl, data)
	if err != nil {
		return "", trace.Wrap(err)
	}
	return out, nil
}
