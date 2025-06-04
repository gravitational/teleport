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
	"errors"
	"io"
	"strings"

	"github.com/gravitational/trace"

	apiclient "github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/lib/client/mcp"
)

// ExtenralErrorRetriever returns an external error that might have happened.
//
// MCP servers don't have knowledge of other processes that might fail during
// their execution, such as authentication failures. This provider can be used
// to give them the necessary context to provide more accurate user messages.
type ExternalErrorRetriever interface {
	// RetrieveError retrieves the error if any.
	RetrieveError() error
}

// FormatErrorMessage formats the database MCP error messages.
// format.
func FormatErrorMessage(retreiver ExternalErrorRetriever, err error) error {
	if retreiver != nil {
		err = trace.NewAggregate(retreiver.RetrieveError(), err)
	}

	switch {
	case errors.Is(err, apiclient.ErrClientCredentialsHaveExpired):
		return trace.BadParameter(ReloginRequiredErrorMessage)
	case strings.Contains(err.Error(), "connection reset by peer") || errors.Is(err, io.ErrClosedPipe):
		return trace.BadParameter(LocalProxyConnectionErrorMessage)
	}

	return err
}

const (
	// ReloginRequiredErrorMessage is the message returned to the MCP client
	// when the tsh session expired.
	ReloginRequiredErrorMessage = `It looks like your Teleport session expired,
you must relogin (using "tsh login" on a terminal) before continue using this
tool. After that, there is no need to update or relaunch the MCP client - just
try using it again.`
	// LocalProxyConnectionErrorMessage is the message returned to the MCP client when
	// the database client cannot connect to the local proxy.
	LocalProxyConnectionErrorMessage = `Teleport MCP server is having issue while
establishing the database connection. You can verify the MCP logs for more
details on what is causing this issue. After identifying and fixing the issue
a restart on the MCP client might be necessary.`
	// EmptyDatabasesListErrorMessage is the message returned to the MCP client when
	// the started database server is serving no databases.
	EmptyDatabasesListErrorMessage = `There are no active Teleport databases available
for use on the MCP server. You can check the MCP server logs to see if any
database was not included due to an error. You can also verify that the list
of databases on the MCP command is correct.`
)

var (
	// WrongDatabaseURIFormatError is the message returned to the MCP client
	// when it sends a malformed database resource URI.
	WrongDatabaseURIFormatError = trace.BadParameter("Malformed database resource URI. Database resources must follow the format: %q", mcp.SampleDatabaseResource)
	// DatabaseNotFoundError is the message returned to the MCP client when the
	// requested database is not available as MCP resource.
	DatabaseNotFoundError = trace.NotFound(`Database not found. Only registered databases
can be used. Ask the user to attach the database resource or list the available
resources with %q tool`, listDatabasesToolName)
)
