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

package postgres

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"

	"github.com/gravitational/teleport/lib/client/db/profile"
)

// GetConnString returns formatted Postgres connection string for the profile.
func GetConnString(c *profile.ConnectProfile, noTLS bool, printFormat bool) string {
	connStr := "postgres://"
	if c.User != "" {
		// Username may contain special characters in which case it should
		// be percent-encoded. For example, when connecting to a Postgres
		// instance on GCP user looks like "name@project-id.iam".
		connStr += url.QueryEscape(c.User) + "@"
	}
	connStr += net.JoinHostPort(c.Host, strconv.Itoa(c.Port))
	if c.Database != "" {
		connStr += "/" + c.Database
	}
	if noTLS {
		return connStr
	}
	params := []string{
		fmt.Sprintf("sslrootcert=%v", c.CACertPath),
		fmt.Sprintf("sslcert=%v", c.CertPath),
		fmt.Sprintf("sslkey=%v", c.KeyPath),
	}
	if c.Insecure {
		params = append(params,
			fmt.Sprintf("sslmode=%v", SSLModeVerifyCA))
	} else {
		params = append(params,
			fmt.Sprintf("sslmode=%v", SSLModeVerifyFull))
	}
	connStr = fmt.Sprintf("%v?%v", connStr, strings.Join(params, "&"))

	// The printed connection string may get copy-pasted for execution. Add
	// quotes to avoid "&" getting interpreted by terminals.
	if printFormat {
		connStr = fmt.Sprintf(`"%s"`, connStr)
	}
	return connStr
}
