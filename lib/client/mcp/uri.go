// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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

package mcp

import (
	"net/url"

	"github.com/gravitational/trace"
	"github.com/ucarion/urlpath"
)

// databaseURITemplate template used to parse database resource URIs.
var databaseURITemplate = urlpath.New("/databases/:name")

const (
	// resourceSchema schema used by Teleport MCP resources.
	resourceSchema = "teleport"

	// databaseNameQueryParamName is the query param name used for database
	// name.
	databaseNameQueryParamName = "dbName"
	// databaseUserQueryParamName is the query param name used for database
	// user.
	databaseUserQueryParamName = "dbUser"
)

// ResourceURI is a Teleport MCP resource URI.
type ResourceURI struct {
	url *url.URL
}

// ParseResourceURI parses a raw resource URI into a Teleport URI.
func ParseResourceURI(uri string) (*ResourceURI, error) {
	parsedURL, err := url.Parse(uri)
	if err != nil {
		return nil, trace.BadParameter("invalid resource URI: %s", err)
	}

	if parsedURL.Scheme != resourceSchema {
		return nil, trace.BadParameter("invalid URI schema")
	}

	return &ResourceURI{url: parsedURL}, nil
}

// GetDatabaseServiceName returns the database service name of the resource.
// Returns empty if the resource is not a database.
func (u ResourceURI) GetDatabaseServiceName() string {
	if match, ok := databaseURITemplate.Match(u.path()); ok {
		return match.Params["name"]
	}

	return ""
}

// GetDatabaseUser returns the database username param of the resource.
// Returns empty if the resource is not a database.
func (u ResourceURI) GetDatabaseUser() string {
	return u.url.Query().Get(databaseUserQueryParamName)
}

// GetDatabaseName returns the database name param of the resource.
// Returns empty if the resource is not a database.
func (u ResourceURI) GetDatabaseName() string {
	return u.url.Query().Get(databaseNameQueryParamName)
}

// IsDatabase returns true if the resource is a database.
func (u ResourceURI) IsDatabase() bool {
	return u.GetDatabaseServiceName() != ""
}

// path returns the resoruce URI full path. For resources, we must include the
// hostname as it indicates the resource type.
func (u ResourceURI) path() string {
	return "/" + u.url.Hostname() + u.url.Path
}
