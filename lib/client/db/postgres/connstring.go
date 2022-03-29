/*
Copyright 2021 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
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
func GetConnString(c *profile.ConnectProfile) string {
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
	return fmt.Sprintf("%v?%v", connStr, strings.Join(params, "&"))
}
