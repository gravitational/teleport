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

package cli

// CopyBinariesCommand includes fields for `tbot copy-binaries`
type CopyBinariesCommand struct {
	*genericExecutorHandler[CopyBinariesCommand]

	// IncludeFDPass specifies whether or not `fdpass-teleport` should be
	// copied.
	IncludeFDPass bool

	// DestinationDir is the directory into which the tbot binary (and
	// optionally fdpass) should be written.
	DestinationDir string
}

// NewCopyBinariesCommand initializes the `tbot copy-binaries` subcommand and
// its fields.
func NewCopyBinariesCommand(app KingpinClause, action func(*CopyBinariesCommand) error) *CopyBinariesCommand {
	cmd := app.Command("copy-binaries", "Copies this tbot binary to a given destination")
	cmd.Interspersed(true)

	c := &CopyBinariesCommand{}
	c.genericExecutorHandler = newGenericExecutorHandler(cmd, c, action)

	cmd.Flag("include-fdpass", "If set, also copy `fdpass-teleport`. It must be available in the same path as `tbot`.").BoolVar(&c.IncludeFDPass)
	cmd.Arg("destination-dir", "The destination path to write the copy of the tbot binary").Required().StringVar(&c.DestinationDir)

	return c
}
