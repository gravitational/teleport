/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package database

import (
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/defaults"
)

// PingParams contains the required fields necessary to test a Database Connection.
type PingParams struct {
	// Host is the hostname of the Database (does not include port).
	Host string
	// Port is the port where the Database is accepting connections.
	Port int
	// Username is the user to be used to login into the database.
	Username string
	// DatabaseName is an optional database name to be used to login into the database.
	DatabaseName string
}

// CheckAndSetDefaults validates and set the default values for the Ping.
func (p *PingParams) CheckAndSetDefaults(protocol string) error {
	if protocol != defaults.ProtocolMySQL && p.DatabaseName == "" {
		return trace.BadParameter("missing required parameter DatabaseName")
	}

	if p.Username == "" {
		return trace.BadParameter("missing required parameter Username")
	}

	if p.Port == 0 {
		return trace.BadParameter("missing required parameter Port")
	}

	if p.Host == "" {
		p.Host = "localhost"
	}

	return nil
}
