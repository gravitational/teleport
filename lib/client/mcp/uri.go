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
	"strings"

	"github.com/gravitational/trace"
	"github.com/ucarion/urlpath"
)

var (
	// clusterURITemplate is the base cluster template.
	clusterURITemplate = urlpath.New("/clusters/:cluster/*")
	// databaseURITemplate template used to parse database resource URIs.
	databaseURITemplate = urlpath.New("/clusters/:cluster/databases/:dbName")
)

const (
	// resourceScheme scheme used by Teleport MCP resources.
	resourceScheme = "teleport"

	// databaseNameQueryParamName is the query param name used for database
	// name.
	databaseNameQueryParamName = "dbName"
	// databaseUserQueryParamName is the query param name used for database
	// user.
	databaseUserQueryParamName = "dbUser"
)

// ResourceURI is a Teleport MCP resource URI.
//
// Query parameters are not covered on the MCP spec but we use them to provide
// additional information about the resource connection. For example, if the
// resource requires a "username" value, this value is provided using the query
// params.
//
// https://modelcontextprotocol.io/docs/concepts/resources#resource-uris
type ResourceURI struct {
	url url.URL
}

// ParseResourceURI parses a raw resource URI into a Teleport URI.
func ParseResourceURI(uri string) (*ResourceURI, error) {
	parsedURL, err := url.Parse(uri)
	if err != nil {
		return nil, trace.BadParameter("invalid resource URI format: %s", err)
	}

	if parsedURL.Scheme != resourceScheme {
		return nil, trace.BadParameter("invalid URI scheme, must be %q", resourceScheme)
	}

	return &ResourceURI{url: *parsedURL}, nil
}

// databaseParams represents the connect params for the database resource.
type databaseParams struct {
	// user is the user to log in as.
	user string
	// name is the name to log in to.
	name string
}

// databaseParam is a param function used for setting database connect params.
type databaseParam func(*databaseParams)

// WithDatabaseUser configures database params with database user.
func WithDatabaseUser(user string) databaseParam {
	return func(dp *databaseParams) {
		dp.user = user
	}
}

// WithDatabaseUser configures database params with database name.
func WithDatabaseName(name string) databaseParam {
	return func(dp *databaseParams) {
		dp.name = name
	}
}

// NewDatabaseResourceURI creates a new database resource URI with connect
// params.
func NewDatabaseResourceURI(cluster string, databaseName string, opts ...databaseParam) ResourceURI {
	params := &databaseParams{}
	for _, opt := range opts {
		opt(params)
	}

	pathWithHost, _ := databaseURITemplate.Build(urlpath.Match{
		Params: map[string]string{
			"cluster": cluster,
			"dbName":  databaseName,
		},
	})

	values := url.Values{}
	if params.user != "" {
		values.Add(databaseUserQueryParamName, params.user)
	}
	if params.name != "" {
		values.Add(databaseNameQueryParamName, params.name)
	}

	return ResourceURI{
		url: url.URL{
			Scheme:   resourceScheme,
			Path:     strings.TrimPrefix(pathWithHost, "/"),
			RawQuery: values.Encode(),
		},
	}
}

// GetDatabaseServiceName returns the Teleport cluster name.
func (u ResourceURI) GetClusterName() string {
	if match, ok := clusterURITemplate.Match(u.path()); ok {
		return match.Params["cluster"]
	}

	return ""
}

// GetDatabaseServiceName returns the database service name of the resource.
// Returns empty if the resource is not a database.
func (u ResourceURI) GetDatabaseServiceName() string {
	if match, ok := databaseURITemplate.Match(u.path()); ok {
		return match.Params["dbName"]
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

// String returns the string representation of the resource URI.
func (u ResourceURI) String() string {
	return u.url.String()
}

// WithoutParams returns a copy of the resource without additional parameters.
func (u ResourceURI) WithoutParams() ResourceURI {
	copyURL := u.url
	copyURL.RawQuery = ""
	return ResourceURI{url: copyURL}
}

// Equal returns true if both resources represent the same Teleport resource.
func (u ResourceURI) Equal(b ResourceURI) bool {
	return u.String() == b.String()
}

// path returns the resource URI full path. We must include the hostname as the
// templates will also include them on the matching.
func (u ResourceURI) path() string {
	return "/" + u.url.Hostname() + u.url.Path
}

// IsDatabase returns true if the URI is a database resource.
func IsDatabaseResourceURI(uri string) bool {
	parsed, err := ParseResourceURI(uri)
	if err != nil {
		return false
	}

	return parsed.IsDatabase()
}

var (
	// SampleDatabaseResource contains a sample full resource URI. This can be
	// used on descriptions to show how a database resource URI looks like.
	SampleDatabaseResource = NewDatabaseResourceURI("example-cluster", "myDatabase")
)
