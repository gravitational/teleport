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

package lib

import (
	"net/url"
	"strings"

	"github.com/gravitational/trace"
)

// AddrToURL transforms an address string that may or may not contain
// a leading protocol or trailing port number into a well-formed URL
func AddrToURL(addr string) (*url.URL, error) {
	var (
		result *url.URL
		err    error
	)
	if !strings.HasPrefix(addr, "http://") && !strings.HasPrefix(addr, "https://") {
		addr = "https://" + addr
	}
	if result, err = url.Parse(addr); err != nil {
		return nil, trace.Wrap(err)
	}
	if result.Scheme == "https" && result.Port() == "443" {
		// Cut off redundant :443
		result.Host = result.Hostname()
	}
	return result, nil
}
