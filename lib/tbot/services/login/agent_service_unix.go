//go:build unix

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
package login

import (
	"context"
	"os"

	"github.com/gravitational/trace"
)

func (s *AgentService) verifyDirectoryPermissions(ctx context.Context, path string) error {
	// Directory.Init creates new directories with correct permissions but does
	// not fix or validate the permissions on existing/user-created directories.
	//
	// There's no good reason for the directory to ever be open to the world, so
	// we assume it's an accidental misconfiguration, and check that the "other"
	// bits are zero.
	stat, err := os.Stat(path)
	if err != nil {
		return trace.Wrap(err, "stating directory")
	}
	perm := stat.Mode().Perm()
	if perm&07 != 0 {
		return trace.BadParameter(
			"Directory is open to the world (%04o) allowing any arbitrary "+
				"user to obtain credentials from the login agent. Fix this with "+
				"chmod or remove the directory and allow tbot to recreate it "+
				"with the correct permissions.",
			perm,
		)
	}
	return nil
}
