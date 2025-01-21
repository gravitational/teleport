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

package vnet

import (
	"context"

	"github.com/gravitational/trace"
)

// runPlatformUserProcess launches a Windows service in the background that will
// handle all networking and OS configuration. The user process exposes a gRPC
// interface that the admin process uses to query application names and get user
// certificates for apps. It returns a [ProcessManager] which controls the
// lifecycle of both the user and admin processes.
func runPlatformUserProcess(ctx context.Context, config *UserProcessConfig) (pm *ProcessManager, err error) {
	// Make sure to close the process manager if returning a non-nil error.
	defer func() {
		if pm != nil && err != nil {
			pm.Close()
		}
	}()

	pm, processCtx := newProcessManager()
	pm.AddCriticalBackgroundTask("VNet Windows service", func() error {
		return trace.Wrap(runService(processCtx), "running VNet Windows service in the background")
	})
	// TODO(nklaassen): run user process gRPC service.
	return pm, nil
}
