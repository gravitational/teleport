//go:build windows

/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package tbot

import (
	"context"
	"os"

	"github.com/gravitational/trace"
)

// ConnectToSSHMultiplex connects to the SSH multiplexer and sends the target
// to the multiplexer. It then returns the connection to the SSH multiplexer
// over stdout.
func ConnectToSSHMultiplex(ctx context.Context, socketPath string, target string, stdout *os.File) error {
	return trace.NotImplemented("SSH Multiplexing not supported on Windows.")
}
