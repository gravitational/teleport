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

package common

import (
	"encoding/json"
	"fmt"

	"github.com/gravitational/trace"
)

// onPing does a ping test against the Proxy.
//
// Some notes on this command:
// - user profiles will NOT be updated after the test.
// - set "--proxy" flag to test a different Teleport Proxy.
// - use "--debug" flag to see ALPN handshake test, port resolver, etc.
// - the command can run without being logged in as long as `--proxy` is set.
// - newer fields in webapi/ping may not show up if `tsh` is old.
func onPing(cf *CLIConf) error {
	tc, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}

	resp, err := tc.Ping(cf.Context)
	if err != nil {
		return trace.Wrap(err)
	}

	json, err := json.MarshalIndent(resp, "", "  ")
	if err != nil {
		return trace.Wrap(err)
	}

	fmt.Fprintln(cf.Stdout(), string(json))
	return nil
}
