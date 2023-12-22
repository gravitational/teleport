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

package webauthn

import (
	"net/url"
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/utils"
)

func validateOrigin(origin, rpID string) error {
	parsedOrigin, err := url.Parse(origin)
	if err != nil {
		return trace.BadParameter("origin is not a valid URL: %v", err)
	}
	host, err := utils.Host(parsedOrigin.Host)
	if err != nil {
		return trace.BadParameter("extracting host from origin: %v", err)
	}

	// TODO(codingllama): Check origin against the public addresses of Proxies and
	//  Auth Servers

	// Accept origins whose host matches the RPID.
	if host == rpID {
		return nil
	}

	// Accept origins whose host is a subdomain of RPID.
	originParts := strings.Split(host, ".")
	rpParts := strings.Split(rpID, ".")
	if len(originParts) <= len(rpParts) {
		return trace.BadParameter("origin doesn't match RPID")
	}
	i := len(originParts) - 1
	j := len(rpParts) - 1
	for j >= 0 {
		if originParts[i] != rpParts[j] {
			return trace.BadParameter("origin doesn't match RPID")
		}
		i--
		j--
	}
	return nil
}
