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

package aws

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/gravitational/trace"

	awsapiutils "github.com/gravitational/teleport/api/utils/aws"
	"github.com/gravitational/teleport/lib/srv/app/common"
	libutils "github.com/gravitational/teleport/lib/utils"
	awsutils "github.com/gravitational/teleport/lib/utils/aws"
)

// resolveEndpoint resolves the AWS endpoint from a valid "X-Forwarded-Host"
// header and extracts the AWS service and region from the authorization header.
// The forwarded host is accepted only if it parses as the exact HTTPS URL shape
// we later dial: no userinfo, path, query, or fragment, an AWS endpoint
// hostname, and either no port or the default HTTPS port. The returned endpoint
// URL is built from the same parsed URL that passed validation so validation and
// forwarding cannot diverge.
func resolveEndpoint(r *http.Request, authHeader string) (*common.AWSResolvedEndpoint, error) {
	forwardedHost, err := libutils.GetSingleHeader(r.Header, "X-Forwarded-Host")
	if err != nil {
		return nil, trace.BadParameter("proxied requests must include X-Forwarded-Host header")
	}

	// Parse the host once, with the scheme we actually dial, so validation and
	// forwarding can never disagree. Validating the raw header separately from
	// the "https://"+host we forward to allows a parser differential: e.g.
	// "attacker.example.com://s3.amazonaws.com" validates as host
	// s3.amazonaws.com but dials attacker.example.com (an SSRF primitive).
	u, err := url.Parse("https://" + forwardedHost)
	if err != nil || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
		return nil, trace.BadParameter("invalid AWS endpoint %q", forwardedHost)
	}
	switch u.Port() {
	case "", "443":
	default:
		return nil, trace.BadParameter("invalid AWS endpoint %q", forwardedHost)
	}
	if !awsapiutils.IsAWSEndpoint(u.Hostname()) {
		return nil, trace.BadParameter("invalid AWS endpoint %q", forwardedHost)
	}

	awsAuthHeader, err := awsutils.ParseSigV4(r.Header.Get(authHeader))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &common.AWSResolvedEndpoint{
		// Build from the validated parse, not the raw header, so the dialed
		// host is exactly the one validated above.
		URL:           (&url.URL{Scheme: "https", Host: u.Host}).String(),
		SigningRegion: awsAuthHeader.Region,
		SigningName:   awsAuthHeader.Service,
	}, nil
}

func isDynamoDBEndpoint(re *common.AWSResolvedEndpoint) bool {
	// Some clients may sign some services with upper case letters. We use all
	// lower cases in our mapping.
	signingName := strings.ToLower(re.SigningName)
	_, ok := dynamoDBSigningNames[signingName]
	return ok
}

// dynamoDBSigningNames is a set of signing names used for DynamoDB APIs.
var dynamoDBSigningNames = map[string]struct{}{
	// signing name for dynamodb and dynamodbstreams API.
	"dynamodb": {},
	// signing name for dynamodb accelerator API.
	"dax": {},
}
