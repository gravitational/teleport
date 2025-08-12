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

package winpki

import (
	"context"
	"net"

	"github.com/gravitational/trace"
)

// locateLDAPServer looks up the LDAP server in an Active Directory
// environment by implementing the DNS-based discovery DC locator
// process.
//
// See https://learn.microsoft.com/en-us/windows-server/identity/ad-ds/manage/dc-locator?tabs=dns-based-discovery
func locateLDAPServer(ctx context.Context, domain string, site string, resolver *net.Resolver) ([]string, error) {
	tryDomain := domain
	if site != "" {
		tryDomain = site + "._sites." + domain
	}

	_, records, err := resolver.LookupSRV(ctx, "ldap", "tcp", tryDomain)
	if err != nil && site != "" {
		// If the site lookup fails, try the domain directly.
		_, records, err = resolver.LookupSRV(ctx, "ldap", "tcp", domain)
	}

	if err != nil {
		return nil, trace.Wrap(err, "looking up SRV records for %v", domain)
	}

	// note: LookupSRV already returns records sorted by priority and takes in to account weights
	var result []string
	for _, record := range records {
		// SRV records will likely return the insecure LDAP port,
		// so we ignore it and hard code the LDAPS port.
		result = append(result, net.JoinHostPort(record.Target, "636"))
	}

	return result, nil
}
