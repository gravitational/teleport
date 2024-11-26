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

package cli

// MigrateCommand contains fields parsed for `tbot migrate`
type MigrateCommand struct {
	*genericExecutorHandler[MigrateCommand]

	// ConfigureOutput is the path to write the file. If empty, writes to
	// stdout.
	ConfigureOutput string
}

// NewMigrateCommand initializes the `tbot migrate` command and its flags.
func NewMigrateCommand(app KingpinClause, action func(*MigrateCommand) error) *MigrateCommand {
	cmd := app.Command("migrate", "Migrates a config file from an older version to the newest version. Outputs to stdout by default.")

	c := &MigrateCommand{}
	c.genericExecutorHandler = newGenericExecutorHandler(cmd, c, action)

	cmd.Flag("output", "Path to write the generated configuration file to rather than write to stdout.").Short('o').StringVar(&c.ConfigureOutput)

	return c
}
