/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

// Package editor launches the user's configured text editor against a file,
// the mechanism shared by tctl commands that collect free-form input (resource
// edits, detection status-change reasons).
package editor

import (
	"cmp"
	"context"
	"os"
	"os/exec"
	"strings"

	"github.com/gravitational/trace"
)

// command returns the editor command line from TELEPORT_EDITOR, VISUAL, or EDITOR, defaulting to vi.
func command() string {
	return cmp.Or(os.Getenv("TELEPORT_EDITOR"), os.Getenv("VISUAL"), os.Getenv("EDITOR"), "vi")
}

// Run opens filename in the user's configured editor and blocks until it exits.
func Run(ctx context.Context, filename string) error {
	editor := command()
	args := strings.Fields(editor)
	cmd := exec.CommandContext(ctx, args[0], append(args[1:], filename)...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return trace.BadParameter("could not start editor %v: %v", editor, err)
	}
	if err := cmd.Wait(); err != nil {
		return trace.BadParameter("editor did not complete successfully: %v", err)
	}
	return nil
}
