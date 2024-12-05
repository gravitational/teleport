// Teleport
// Copyright (C) 2024 Gravitational, Inc.
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

package repl

import (
	"context"
	"crypto/tls"
	"io"

	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

// NewREPLConfig represents the database REPL constructor config.
type NewREPLConfig struct {
	// Client returns the user terminal client.
	Client io.ReadWriter
	// Addr returns the address the REPL should connect to start the database
	// connection.
	Addr utils.NetAddr
	// TLSConfig returns the TLS configuration used to connect to the Addr().
	TLSConfig *tls.Config
	// Route returns the session routing information.
	Route tlsca.RouteToDatabase
}

// REPLNewFunc defines the constructor function for database REPL
// sessions.
type REPLNewFunc func(context.Context, *NewREPLConfig) (REPLInstance, error)

// REPLInstance represents a REPL instance.
type REPLInstance interface {
	// Run executes the REPL. This is a blocking operation.
	Run(context.Context) error
}

// REPLGetter is an interface for retrieving REPL constructor functions given
// the database protocol.
type REPLGetter interface {
	// GetREPL returns a start function for the specified protocol.
	GetREPL(ctx context.Context, dbProtocol string) (REPLNewFunc, error)
}
