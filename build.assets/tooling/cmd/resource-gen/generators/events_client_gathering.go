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

type eventsClientEntry struct {
	Kind       string // PascalCase, e.g. "Cookie"
	Lower      string // snake_case, e.g. "cookie"
	PkgAlias   string // e.g. "cookiev1"
	ImportPath string // e.g. "github.com/.../cookie/v1"
}

type eventsClientGatheringData struct {
	Module    string
	Resources []eventsClientEntry
}

var eventsClientGatheringTmpl = mustReadTemplate("events_client_gathering.go.tmpl")

// GenerateEventsClientGathering renders api/client/events_generated.gen.go
// with generatedEventToGRPC and generatedEventFromGRPC dispatch functions
// for all cache-enabled generated resources.
func GenerateEventsClientGathering(specs []spec.ResourceSpec, module string) (string, error) {
	// Filter to cache-enabled resources only.
	var entries []eventsClientEntry
	for _, rs := range specs {
		if !rs.Cache.Enabled {
			continue
		}
		entries = append(entries, eventsClientEntry{
			Kind:       rs.KindPascal,
			Lower:      rs.Kind,
			PkgAlias:   protoPackageAlias(rs.ServiceName),
			ImportPath: protoGoImportPath(rs.ServiceName, module),
		})
	}
	if len(entries) == 0 {
		// Produce no-op stubs so events.go compiles even with zero resources.
		return `package client

import (
	"` + module + `/api/client/proto"
	"` + module + `/api/types"
)

func generatedEventToGRPC(_ *proto.Event, _ types.Resource) bool { return false }

func generatedEventFromGRPC(_ *proto.Event) (types.Resource, bool) { return nil, false }
`, nil
	}
	data := eventsClientGatheringData{
		Module:    module,
		Resources: entries,
	}
	out, err := render("eventsClientGathering", eventsClientGatheringTmpl, data)
	if err != nil {
		return "", trace.Wrap(err)
	}
	return out, nil
}
