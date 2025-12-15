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

// DBCommand contains fields for `tbot db`
type DBCommand struct {
	*genericExecutorHandler[DBCommand]

	DestinationDir string
	Cluster        string
	ProxyServer    string

	RemainingArgs *[]string
}

// NewDBCommand initializes flags for `tbot db`
func NewDBCommand(app KingpinClause, action func(*DBCommand) error) *DBCommand {
	cmd := app.Command("db", "Execute database commands through tsh.")

	c := &DBCommand{}
	c.genericExecutorHandler = newGenericExecutorHandler(cmd, c, action)

	cmd.Flag("proxy-server", "The Teleport proxy server to use, in host:port form.").StringVar(&c.ProxyServer)
	cmd.Flag("destination-dir", "The destination directory with which to authenticate tsh").StringVar(&c.DestinationDir)
	cmd.Flag("cluster", "The cluster name. Extracted from the certificate if unset.").StringVar(&c.Cluster)
	c.RemainingArgs = RemainingArgs(cmd.Arg(
		"args",
		"Arguments to `tsh db ...`; prefix with `-- ` to ensure flags are passed correctly.",
	))

	return c
}
