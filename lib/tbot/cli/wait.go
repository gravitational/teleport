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

import (
	"net/http"
	"time"
)

// WaitCommand supports `tbot wait`
type WaitCommand struct {
	*genericExecutorHandler[WaitCommand]

	DiagAddr string
	Service  string
	Timeout  time.Duration

	// Client is an optional alternative HTTP client implementation for use in
	// tests. If unspecified, a standard client is used.
	Client *http.Client
}

// NewWaitCommand initializes the subcommand for `tbot wait`
func NewWaitCommand(app KingpinClause, action func(*WaitCommand) error) *WaitCommand {
	cmd := app.Command("wait", "Waits for a running tbot to become ready.")

	c := &WaitCommand{}
	c.genericExecutorHandler = newGenericExecutorHandler(cmd, c, action)

	cmd.Flag("diag-addr", "The configured --diag-addr of a running bot, in host:port form.").Required().StringVar(&c.DiagAddr)
	cmd.Flag(
		"service",
		"An optional name. If set, waits for only the named service to "+
			"become healthy. If unset, waits for all services.",
	).StringVar(&c.Service)
	cmd.Flag(
		"timeout",
		"An optional timeout. If set, returns an error if all specified "+
			"services have reported healthy by the timeout.",
	).DurationVar(&c.Timeout)

	return c
}
