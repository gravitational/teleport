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

type apiClientData struct {
	resourceBase
	ClientCtor string
}

// GenerateAPIClient renders API client helper methods for a resource.
func GenerateAPIClient(rs spec.ResourceSpec, module string) (string, error) {
	if err := rs.Validate(); err != nil {
		return "", trace.Wrap(err)
	}

	d := apiClientData{
		resourceBase: newResourceBase(rs, module),
		ClientCtor:   serviceShortName(rs.ServiceName) + "Client",
	}
	return render("api-client", apiClientTmpl, d)
}

var apiClientTmpl = mustReadTemplate("api_client.go.tmpl")
