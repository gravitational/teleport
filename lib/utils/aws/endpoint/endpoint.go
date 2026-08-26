// Teleport
// Copyright (C) 2024 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package endpoint

import (
	"fmt"
	"regexp"
)

var schemeRegex = regexp.MustCompile("^([^:]+)://")

// CreateURI ensures that the provided endpoint is a valid
// URI and contains a scheme. This is primarily to preserve
// backward compatible behavior when changing between
// aws-sdk-go and aws-sdk-go/v2. The legacy sdk automatically
// applied the scheme to custom endpoints, while the new sdk
// does not, and will return an error if the URI is invalid.
// To allow existing configurations to continue to work with
// a custom service endpoint, this performs applies the same
// behavior that the legacy sdk did.
func CreateURI(endpoint string, insecure bool) string {
	if endpoint != "" && !schemeRegex.MatchString(endpoint) {
		scheme := "https"
		if insecure {
			scheme = "http"
		}
		return fmt.Sprintf("%s://%s", scheme, endpoint)
	}

	return endpoint
}
