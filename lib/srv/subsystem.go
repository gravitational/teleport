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

package srv

import (
	"context"

	"golang.org/x/crypto/ssh"
)

// SubsystemResult is a result of execution of the subsystem.
type SubsystemResult struct {
	// Name holds the name of the subsystem that was executed.
	Name string

	// Err holds the result of execution of the subsystem.
	Err error
}

// Subsystem represents SSH subsystem - special command executed
// in the context of the session.
type Subsystem interface {
	// Start starts subsystem
	Start(context.Context, *ssh.ServerConn, ssh.Channel, *ssh.Request, *ServerContext) error

	// Wait is returned by subsystem when it's completed
	Wait() error
}
