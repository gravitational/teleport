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
	"io"
	"net"

	"github.com/gravitational/trace"

	clientproto "github.com/gravitational/teleport/api/client/proto"
)

// NewREPLConfig represents the database REPL constructor config.
type NewREPLConfig struct {
	// Client is the user terminal client.
	Client io.ReadWriteCloser
	// ServerConn is the database server connection.
	ServerConn net.Conn
	// Route is the session routing information.
	Route clientproto.RouteToDatabase
}

// REPLNewFunc defines the constructor function for database REPL
// sessions.
type REPLNewFunc func(context.Context, *NewREPLConfig) (REPLInstance, error)

// REPLInstance represents a REPL instance.
type REPLInstance interface {
	// Run executes the REPL. This is a blocking operation.
	Run(context.Context) error
}

// REPLRegistry is an interface for initializing REPL instances and checking
// if the database protocol is supported.
type REPLRegistry interface {
	// IsSupported returns if a database protocol is supported by any REPL.
	IsSupported(protocol string) bool
	// NewInstance initializes a new REPL instance given the configuration.
	NewInstance(context.Context, *NewREPLConfig) (REPLInstance, error)
}

// NewREPLGetter creates a new REPL getter given the list of supported REPLs.
func NewREPLGetter(replNewFuncs map[string]REPLNewFunc) REPLRegistry {
	return &replRegistry{m: replNewFuncs}
}

type replRegistry struct {
	m map[string]REPLNewFunc
}

// IsSupported implements REPLGetter.
func (r *replRegistry) IsSupported(protocol string) bool {
	_, supported := r.m[protocol]
	return supported
}

// NewInstance implements REPLGetter.
func (r *replRegistry) NewInstance(ctx context.Context, cfg *NewREPLConfig) (REPLInstance, error) {
	if newFunc, ok := r.m[cfg.Route.Protocol]; ok {
		return newFunc(ctx, cfg)
	}

	return nil, trace.NotImplemented("REPL not supported for protocol %q", cfg.Route.Protocol)
}
