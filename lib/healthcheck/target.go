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

package healthcheck

import (
	"net/url"
	"strings"

	"github.com/gravitational/trace"
)

// normalizeAddress removes the "http://" or "https://" scheme from the input
// address and infers a default port if necessary. If a scheme is present but no
// port is specified, it defaults to 80 for "http" or 443 for "https". If there
// is no scheme and no port, it defaults to https port 443.
func normalizeAddress(addr string) (string, error) {
	if !strings.Contains(addr, "://") {
		addr = "https://" + addr
	}
	u, err := url.Parse(addr)
	if err != nil {
		return "", trace.Wrap(err)
	}
	host := u.Hostname()
	if host == "" {
		return "", trace.BadParameter("%s is missing host name", addr)
	}
	port := u.Port()
	if port == "" {
		switch u.Scheme {
		case "http":
			port = "80"
		case "https", "aws": // we sometimes use "aws" as a fake scheme.
			port = "443"
		case "postgres":
			port = "5432"
		case "mysql":
			port = "3306"
		case "clickhouse":
			port = "9440"
		case "clickhouse-http":
			port = "8443"
		default:
			return "", trace.BadParameter("%s is missing a port", addr)
		}
	}
	hostPort := host + ":" + port
	return hostPort, nil
}
