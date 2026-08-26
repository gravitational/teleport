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

package utils

import (
	"regexp"
	"strings"

	"github.com/gravitational/trace"
)

var reProxyJump = regexp.MustCompile(
	// optional username, note that outside group
	`(?:(?P<username>[^\:]+)@)?(?P<hostport>[^\@]+)`,
)

// JumpHost is a target jump host
type JumpHost struct {
	// Username to login as
	Username string
	// Addr is a target addr
	Addr NetAddr
}

// ParseProxyJump parses strings like user@host:port,bob@host:port
func ParseProxyJump(in string) ([]JumpHost, error) {
	if in == "" {
		return nil, trace.BadParameter("missing proxyjump")
	}
	parts := strings.Split(in, ",")
	out := make([]JumpHost, 0, len(parts))
	for _, part := range parts {
		match := reProxyJump.FindStringSubmatch(strings.TrimSpace(part))
		if len(match) == 0 {
			return nil, trace.BadParameter("could not parse %q, expected format user@host:port,user@host:port", in)
		}
		addr, err := ParseAddr(match[2])
		if err != nil {
			return nil, trace.Wrap(err)
		}
		out = append(out, JumpHost{Username: match[1], Addr: *addr})
	}
	return out, nil
}
