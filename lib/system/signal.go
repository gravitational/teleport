//go:build !windows

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

package system

/*
#include <signal.h>
int resetInterruptSignalHandler() {
	struct sigaction act;
	int result;
	if ((result = sigaction(SIGINT, 0, &act)) != 0) {
		return result;
	}
	if (act.sa_handler == SIG_IGN) {
		// Reset the handler for SIGINT to system default.
		// FIXME: Note, this will also overwrite runtime's signal handler
		signal(SIGINT, SIG_DFL);
	}
	return 0;
}
*/
import "C"

import (
	"context"
	"log/slog"
)

// ResetInterruptSignal will reset the handler for SIGINT back to the default
// handler. We need to do this because when sysvinit launches Teleport on some
// operating systems (like CentOS 6.8) it configures Teleport to ignore SIGINT
// signals. See the following for more details:
//
// http://garethrees.org/2015/08/07/ping/
// https://github.com/openssh/openssh-portable/commit/4e0f5e1ec9b6318ef251180dbca50eaa01f74536
func ResetInterruptSignalHandler() {
	_, err := C.resetInterruptSignalHandler()
	if err != nil {
		slog.WarnContext(context.Background(), "Failed to reset interrupt signal handler", "error", err)
	}
}
