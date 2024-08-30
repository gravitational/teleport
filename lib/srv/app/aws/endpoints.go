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

	"github.com/aws/aws-sdk-go/aws/endpoints"
	"github.com/aws/aws-sdk-go/service/appstream"
	"github.com/aws/aws-sdk-go/service/detective"
	"github.com/aws/aws-sdk-go/service/ecr"
	"github.com/aws/aws-sdk-go/service/ecrpublic"
	"github.com/aws/aws-sdk-go/service/elasticinference"
	"github.com/aws/aws-sdk-go/service/iot1clickdevicesservice"
	"github.com/aws/aws-sdk-go/service/iotdataplane"
	"github.com/aws/aws-sdk-go/service/iotdeviceadvisor"
	"github.com/aws/aws-sdk-go/service/ioteventsdata"
	"github.com/aws/aws-sdk-go/service/iotfleethub"
	"github.com/aws/aws-sdk-go/service/iotjobsdataplane"
	"github.com/aws/aws-sdk-go/service/iotsecuretunneling"
	"github.com/aws/aws-sdk-go/service/iottwinmaker"
	"github.com/aws/aws-sdk-go/service/iotwireless"
	"github.com/aws/aws-sdk-go/service/lexmodelsv2"
	"github.com/aws/aws-sdk-go/service/marketplacecatalog"
	"github.com/aws/aws-sdk-go/service/mediatailor"
	"github.com/aws/aws-sdk-go/service/memorydb"
	"github.com/aws/aws-sdk-go/service/migrationhubstrategyrecommendations"
	"github.com/aws/aws-sdk-go/service/pinpoint"
	"github.com/aws/aws-sdk-go/service/pinpointsmsvoice"
	"github.com/aws/aws-sdk-go/service/pricing"
	"github.com/aws/aws-sdk-go/service/proton"
	"github.com/aws/aws-sdk-go/service/sagemaker"
	"github.com/aws/aws-sdk-go/service/ses"
	"github.com/aws/aws-sdk-go/service/sso"
	"github.com/aws/aws-sdk-go/service/ssooidc"
	"github.com/aws/aws-sdk-go/service/timestreamquery"
	"github.com/gravitational/trace"

	awsapiutils "github.com/gravitational/teleport/api/utils/aws"
	libutils "github.com/gravitational/teleport/lib/utils"
	awsutils "github.com/gravitational/teleport/lib/utils/aws"
)

// resolveEndpoint extracts the aws-service and aws-region from the request
// authorization header and resolves the aws-service and aws-region to AWS
// endpoint.
func resolveEndpoint(r *http.Request) (*endpoints.ResolvedEndpoint, error) {
	// Use X-Forwarded-Host header if it is a valid AWS endpoint.
	forwardedHost, headErr := libutils.GetSingleHeader(r.Header, "X-Forwarded-Host")
	if headErr == nil && awsapiutils.IsAWSEndpoint(forwardedHost) {
		re, err := resolveEndpointByXForwardedHost(r, awsutils.AuthorizationHeader)
		return re, trace.Wrap(err)
	}

	awsAuthHeader, err := awsutils.ParseSigV4(r.Header.Get(awsutils.AuthorizationHeader))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// aws-sdk-go maintains a mapping of service endpoints which can be looked
	// up by calling `endpoints.DefaultResolver().EndpointFor`. This mapping
	// can be found at:
	// https://github.com/aws/aws-sdk-go/blob/main/aws/endpoints/defaults.go
	//
	// The json equivalent can be found in botocore source code at:
	// https://github.com/boto/botocore/blob/develop/botocore/data/endpoints.json
	//
	// The keys used for lookups are endpoints IDs, which can be different from
	// the signing names. We have to translate the signing name received from
	// the header back to the endpoints ID.
	//
	// In addition, many services are NOT found in aws-sdk-go's endpoints
	// mapping. How aws-sdk-go resolves endpoints for these services is to
	// allow ResolveUnknownService when creating the client sessions, which in
	// turn generates the endpoint by using the endpoints ID and some default
	// suffixes. We allow ResolveUnknownService here for the same purpose.
	endpointsID := endpointsIDFromSigningName(awsAuthHeader.Service)
	opts := func(opts *endpoints.Options) {
		opts.ResolveUnknownService = true
	}

	resolvedEndpoint, err := endpoints.DefaultResolver().EndpointFor(endpointsID, awsAuthHeader.Region, opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// SigningName can be derived from the endpoint ID which may not be the
	// correct signing name. Set it back to what is received from the header.
	resolvedEndpoint.SigningName = awsAuthHeader.Service
	return &resolvedEndpoint, nil
}

// resolveEndpointByXForwardedHost resolves the endpoint by creating the URL
// from valid "X-Forwarded-Host" header and extracting aws-service and
// aws-region from the authorization header.
func resolveEndpointByXForwardedHost(r *http.Request, headerKey string) (*endpoints.ResolvedEndpoint, error) {
	forwardedHost := r.Header.Get("X-Forwarded-Host")
	if forwardedHost == "" {
		return nil, trace.BadParameter("missing X-Forwarded-Host")
	}
	if !awsapiutils.IsAWSEndpoint(forwardedHost) {
		return nil, trace.BadParameter("invalid AWS endpoint %v", forwardedHost)
	}

	awsAuthHeader, err := awsutils.ParseSigV4(r.Header.Get(headerKey))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &endpoints.ResolvedEndpoint{
		URL:           "https://" + forwardedHost,
		SigningRegion: awsAuthHeader.Region,
		SigningName:   awsAuthHeader.Service,
	}, nil
}

// endpointsIDFromSigningName returns the endpoints ID used for endpoint
// lookups when calling endpoints.DefaultResolver().EndpointFor.
func endpointsIDFromSigningName(signingName string) string {
	// Some clients may sign some services with upper case letters. We use all
	// lower cases in our mapping.
	signingName = strings.ToLower(signingName)

	if endpointsID, ok := signingNameToEndpointsID[signingName]; ok {
		return endpointsID
	}

	// If not found in the mapping, endpoints ID is expected to be the same as
	// the signing name.
	return signingName
}

func isDynamoDBEndpoint(re *endpoints.ResolvedEndpoint) bool {
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

// signingNameToEndpointsID is a map of AWS services' signing names to their
// endpoints IDs.
//
// This mapping was created by the following process:
// 1. Compiled a mapping of all signing names to their hostnames (e.g. grep/awk
// keywords in "aws-sdk-go-v2/services/")
// 2. Created unit test "TestResolveEndpoints" to test each signing name.
// 3. Investigated the test failures, and updated this mapping to fix them.
//
// TODO Many services may sign with same names but use different hostnames.
// Will need a way to differentiate them. For now, either make the best guess
// in this mapping or use the default signing names. See signingNameToHostname
// in endpoints_test.go for conflicting services.
var signingNameToEndpointsID = map[string]string{
	"appstream":                             appstream.EndpointsID,
	"aws-marketplace":                       marketplacecatalog.EndpointsID,
	"awsiottwinmaker":                       iottwinmaker.EndpointsID,
	"awsmigrationhubstrategyrecommendation": migrationhubstrategyrecommendations.EndpointsID,
	// AWS mobile service deprecated since v1.55.0.
	// Constant copied from mobile.EndpointsID:
	// https://github.com/aws/aws-sdk-go/blob/019bed03fa64f3edad98bba262d41d58eb2b9fee/service/mobile/service.go#L33-L34
	"awsmobilehubservice": "mobile",
	"awsproton20200720":   proton.EndpointsID,
	"awsssooidc":          ssooidc.EndpointsID,
	"awsssoportal":        sso.EndpointsID,
	"detective":           detective.EndpointsID,
	"ecr":                 ecr.EndpointsID,
	"ecr-public":          ecrpublic.EndpointsID,
	"elastic-inference":   elasticinference.EndpointsID,
	"iot-jobs-data":       iotjobsdataplane.EndpointsID,
	"iot1click":           iot1clickdevicesservice.EndpointsID,
	"iotdata":             iotdataplane.EndpointsID,
	"iotdeviceadvisor":    iotdeviceadvisor.EndpointsID,
	"ioteventsdata":       ioteventsdata.EndpointsID,
	"iotfleethub":         iotfleethub.EndpointsID,
	"iotsecuredtunneling": iotsecuretunneling.EndpointsID,
	"iotwireless":         iotwireless.EndpointsID,
	"lex":                 lexmodelsv2.EndpointsID,
	"mediatailor":         mediatailor.EndpointsID,
	"memorydb":            memorydb.EndpointsID,
	"mobiletargeting":     pinpoint.EndpointsID,
	"pricing":             pricing.EndpointsID,
	"sagemaker":           sagemaker.EndpointsID,
	"ses":                 ses.EndpointsID,
	"sms-voice":           pinpointsmsvoice.EndpointsID,
	"timestream":          timestreamquery.EndpointsID,
}
