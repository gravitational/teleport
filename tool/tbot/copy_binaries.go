/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package main

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/tbot/cli"
)

const (
	// fdpassBinaryName is the name of the fdpass binary
	fdpassBinaryName = "fdpass-teleport"

	// tbotBinaryName is the name of the tbot binary
	tbotBinaryName = "tbot"
)

// copyBinary copies a binary from the source to the destination. It assumes
// 0755 permissions.
func copyBinary(src, dest string) error {
	inputFile, err := os.Open(src)
	if err != nil {
		return trace.Wrap(err, "opening source file: %s", src)
	}
	defer inputFile.Close()

	destFile, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return trace.Wrap(err, "opening destination for writing: %s", dest)
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, inputFile)
	if err != nil {
		return trace.Wrap(err, "copying file contents: %s", src)
	}

	return nil
}

// onCopyBinariesCommand runs `tbot copy-binaries`
func onCopyBinariesCommand(ctx context.Context, cmd *cli.CopyBinariesCommand) error {
	selfPath, err := os.Executable()
	if err != nil {
		return trace.Wrap(err, "determining current executable")
	}

	stat, err := os.Stat(cmd.DestinationDir)
	if errors.Is(err, os.ErrNotExist) {
		if err := os.MkdirAll(cmd.DestinationDir, 0755); err != nil {
			return trace.Wrap(err, "creating destination directory: %s", cmd.DestinationDir)
		}
	} else if err != nil {
		return trace.Wrap(err, "could not resolve destination directory: %s", cmd.DestinationDir)
	} else if !stat.IsDir() {
		return trace.BadParameter("invalid destination directory: %s", cmd.DestinationDir)
	}

	tbotDest := filepath.Join(cmd.DestinationDir, tbotBinaryName)
	if err := copyBinary(selfPath, tbotDest); err != nil {
		return trace.Wrap(err, "copying %s binary", tbotBinaryName)
	}
	log.InfoContext(ctx, "Copied tbot to destination", "path", tbotDest)

	if cmd.IncludeFDPass {
		fdpassPath := filepath.Join(filepath.Dir(selfPath), fdpassBinaryName)
		fdpassDest := filepath.Join(cmd.DestinationDir, fdpassBinaryName)
		if err := copyBinary(fdpassPath, fdpassDest); err != nil {
			return trace.Wrap(err, "copying %s binary", fdpassBinaryName)
		}

		log.InfoContext(ctx, "Copied fdpass-teleport to destination", "path", fdpassDest)
	}

	log.InfoContext(ctx, "Binaries have been copied successfully", "destination", cmd.DestinationDir)

	return nil
}
