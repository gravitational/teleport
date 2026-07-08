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

package oauth

import (
	"net"
	"net/url"
	"strings"

	"github.com/gravitational/trace"
)

// validateOAuthEndpoint rejects plaintext direct OAuth endpoints. HTTP is
// allowed only for loopback/dev endpoints.
func validateOAuthEndpoint(rawURL, field string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return trace.Wrap(err, "parsing OAuth %s", field)
	}
	if u.Scheme == "https" {
		return nil
	}
	if u.Scheme == "http" && isLoopbackHost(u.Host) {
		return nil
	}
	return trace.BadParameter("OAuth %s must use HTTPS unless it is loopback", field)
}

func isLoopbackHost(hostport string) bool {
	host := hostport
	if splitHost, _, err := net.SplitHostPort(hostport); err == nil {
		host = splitHost
	}
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
