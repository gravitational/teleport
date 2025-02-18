// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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

package identitycenterutils

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/gravitational/trace"

	awsutils "github.com/gravitational/teleport/lib/utils/aws"
)

// matchAWSICEndpointIDField matches an alphanumeric value separated by a hyphen.
var matchAWSICEndpointIDField = regexp.MustCompile(`^[a-zA-Z0-9-]*$`).MatchString

// EnsureSCIMEndpoint validates dynamic fields of SCIM base URL and returns
// a new base URL constructed from the validated fields.
//
// E.g. valid SCIM base URL:
// "https://scim.ca-central-1.amazonaws.com/bdh6a5e3698-0fc6-4232-a028-fea1a99ff77a/scim/v2".
// Dynamic field includes the AWS region and a random ID field:
// "https://scim.<aws-region>.amazonaws.com/<random-id>/scim/v2"
// Region value is validated against known AWS regions and the random ID field is
// validated against an alphanumeric with hyphen regexp.
// Note: The random ID field looks like a UUID field but does not confirm to
// standard UUID format defined in RFC 4122.
func EnsureSCIMEndpoint(u string) (string, error) {
	baseURL, err := url.ParseRequestURI(u)
	if err != nil {
		return "", trace.BadParameter("invalid SCIM endpoint format: %s", err.Error())
	}
	if baseURL.Scheme != "https" {
		return "", trace.BadParameter("url scheme must be https")
	}

	domainParts := strings.Split(baseURL.Hostname(), ".")
	if len(domainParts) != 4 {
		return "", trace.BadParameter("invalid SCIM endpoint format")
	}
	if domainParts[0] != "scim" {
		return "", trace.BadParameter("unrecognized SCIM endpoint")
	}
	region := domainParts[1]
	if !awsutils.IsKnownRegion(region) {
		return "", trace.BadParameter("region %q is invalid", region)
	}
	if domainParts[2] != "amazonaws" || domainParts[3] != "com" {
		return "", trace.BadParameter("SCIM endpoint must be of 'amazonaws.com' domain")
	}

	pathParts := strings.Split(baseURL.Path, "/")
	if len(pathParts) != 4 {
		return "", trace.BadParameter("invalid SCIM endpoint format")
	}
	if !matchAWSICEndpointIDField(pathParts[1]) {
		return "", trace.BadParameter("invalid SCIM endpoint format")
	}
	if pathParts[2] != "scim" {
		return "", trace.BadParameter("unrecognized SCIM endpoint")
	}
	if pathParts[3] != "v2" {
		return "", trace.BadParameter("only v2 SCIM endpoint is supported")
	}

	newBaseURL := url.URL{
		Scheme: "https",
		Host:   fmt.Sprintf("scim.%s.amazonaws.com", region),
		Path:   fmt.Sprintf("%s/scim/v2", pathParts[1]),
	}
	return newBaseURL.String(), nil
}
