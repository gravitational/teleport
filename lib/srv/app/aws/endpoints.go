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
	"strings"

	"github.com/gravitational/trace"

	awsapiutils "github.com/gravitational/teleport/api/utils/aws"
	"github.com/gravitational/teleport/lib/srv/app/common"
	libutils "github.com/gravitational/teleport/lib/utils"
	awsutils "github.com/gravitational/teleport/lib/utils/aws"
)

// resolveEndpoint resolves the endpoint by creating the URL
// from valid "X-Forwarded-Host" header and extracting aws-service and
// aws-region from the authorization header.
func resolveEndpoint(r *http.Request, authHeader string) (*common.AWSResolvedEndpoint, error) {
	forwardedHost, err := libutils.GetSingleHeader(r.Header, "X-Forwarded-Host")
	if err != nil {
		return nil, trace.BadParameter("proxied requests must include X-Forwarded-Host header")
	}

	if !awsapiutils.IsAWSEndpoint(forwardedHost) {
		return nil, trace.BadParameter("invalid AWS endpoint %v", forwardedHost)
	}

	awsAuthHeader, err := awsutils.ParseSigV4(r.Header.Get(authHeader))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &common.AWSResolvedEndpoint{
		URL:           "https://" + forwardedHost,
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
