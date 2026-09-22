// Teleport
// Copyright (C) 2026 Gravitational, Inc.
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

//go:build unix

package service

import (
	"os"
	"syscall"
)

const (
	SIGUSR1 = syscall.SIGUSR1
	SIGUSR2 = syscall.SIGUSR2
)

// teleportSignals contains all the signals that
// [TeleportProcess.WaitForSignals] cares about.
var teleportSignals = []os.Signal{
	// Note: SIGKILL can't be trapped.
	syscall.SIGQUIT, // graceful shutdown
	syscall.SIGTERM, // fast shutdown
	syscall.SIGINT,  // fast shutdown
	SIGUSR1,         // log process diagnostic info
	SIGUSR2,         // initiate process restart procedure
	syscall.SIGHUP,  // graceful restart procedure
}
