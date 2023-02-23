/*
Copyright 2022 Gravitational, Inc.

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

package aws

import (
	"context"
	"net/http"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/endpoints"
	"github.com/aws/aws-sdk-go/aws/session"
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
	"github.com/aws/aws-sdk-go/service/mobile"
	"github.com/aws/aws-sdk-go/service/pinpoint"
	"github.com/aws/aws-sdk-go/service/pinpointsmsvoice"
	"github.com/aws/aws-sdk-go/service/pricing"
	"github.com/aws/aws-sdk-go/service/proton"
	"github.com/aws/aws-sdk-go/service/sagemaker"
	"github.com/aws/aws-sdk-go/service/ses"
	"github.com/aws/aws-sdk-go/service/sso"
	"github.com/aws/aws-sdk-go/service/ssooidc"
	"github.com/aws/aws-sdk-go/service/timestreamquery"
	"github.com/aws/aws-sdk-go/service/timestreamwrite"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"

	awsapiutils "github.com/gravitational/teleport/api/utils/aws"
	"github.com/gravitational/teleport/lib/srv/app/common"
	"github.com/gravitational/teleport/lib/utils"
	awsutils "github.com/gravitational/teleport/lib/utils/aws"
)

// resolveEndpoint extracts the aws-service and aws-region from the request
// authorization header and resolves the aws-service and aws-region to AWS
// endpoint.
func (s *signerHandler) resolveEndpoint(r *http.Request) (*endpoints.ResolvedEndpoint, error) {
	switch {
	// Use X-Forwarded-Host header if it is a valid AWS endpoint.
	case awsapiutils.IsAWSEndpoint(r.Header.Get("X-Forwarded-Host")):
		re, err := resolveEndpointByXForwardedHost(r, awsutils.AuthorizationHeader)
		return re, trace.Wrap(err)

	// Special handling for Timestream when tsh local proxy is in Endpoint mode.
	case shouldDiscoverTimestreamEndpoint(r):
		re, err := s.resolveTimestreamEndpoint(r)
		return re, trace.Wrap(err)

	// tsh local proxy in Endpoint mode.
	default:
		re, err := resolveEndpointBySDKResolver(r)
		return re, trace.Wrap(err)
	}
}

// resolveEndpointBySDKResolver resolves the endpoint by using AWS SDK's
// default resolver.
func resolveEndpointBySDKResolver(r *http.Request) (*endpoints.ResolvedEndpoint, error) {
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

// shouldDiscoverTimestreamEndpoint returns true if Timestream endpoint
// discovery is required.
//
// Common Timestream operations require an "endpoint discovery" prior to the
// actual API call. The "endpoint discovery" is done by making a
// "DescribeEndpoints" call using the user credentials. However, in Endpoint
// mode (e.g. `--endpoint-url` for AWS CLI or `Endpoint=` for JDBC), the
// "endpoint discovery" is skipped, and the endpoint passed to the client is
// expected to be an URL equivalent to what "DescribeEndpoints" returns. Thus
// the app agent has to do the "endpoint discovery" first (using the user
// credentials) to figure out where to send the actual APIs.
//
// Sample AWS SDK reference for the discovery flow:
// https://github.com/aws/aws-sdk-go/blob/41717ba2c04d3fd03f94d09ea984a10899574935/service/timestreamquery/api.go#L1295-L1319
func shouldDiscoverTimestreamEndpoint(r *http.Request) bool {
	target := r.Header.Get("X-Amz-Target")
	return strings.HasPrefix(target, timestreamOpPrefix) && !strings.HasSuffix(target, "DescribeEndpoints")
}

// resolveTimestreamEndpoint performs a Timestream endpoint discover to resolve
// the endpoint.
func (s *signerHandler) resolveTimestreamEndpoint(r *http.Request) (*endpoints.ResolvedEndpoint, error) {
	sessCtx, err := common.GetSessionContext(r)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	awsAuthHeader, err := awsutils.ParseSigV4(r.Header.Get(awsutils.AuthorizationHeader))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	credentials, err := s.CredentialsGetter.Get(s.closeContext, awsutils.GetCredentialsRequest{
		Provider:    s.Session,
		Expiry:      sessCtx.Identity.Expires,
		SessionName: sessCtx.Identity.Username,
		RoleARN:     sessCtx.Identity.RouteToApp.AWSRoleARN,
		ExternalID:  sessCtx.App.GetAWSExternalID(),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	key := resolveTimestreamEndpointCacheKey{
		credentials:              credentials,
		region:                   awsAuthHeader.Region,
		isTimestreamWriteRequest: isTimestreamWriteRequest(r),
	}
	re, err := utils.FnCacheGet(s.closeContext, s.cache, key, func(ctx context.Context) (*endpoints.ResolvedEndpoint, error) {
		session, err := session.NewSession(aws.NewConfig().WithCredentials(key.credentials).WithRegion(key.region))
		if err != nil {
			return nil, trace.Wrap(err)
		}

		getEndpoint := getTimestreamQueryEndpoint
		if key.isTimestreamWriteRequest {
			getEndpoint = getTimestreamWriteEndpoint
		}
		endpoint, err := getEndpoint(ctx, session)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		return &endpoints.ResolvedEndpoint{
			URL:           "https://" + endpoint,
			SigningRegion: key.region,
			SigningName:   "timestream",
		}, nil
	})
	return re, trace.Wrap(err)
}

type resolveTimestreamEndpointCacheKey struct {
	credentials              *credentials.Credentials
	region                   string
	isTimestreamWriteRequest bool
}

func isTimestreamWriteRequest(r *http.Request) bool {
	switch strings.TrimPrefix(r.Header.Get("X-Amz-Target"), timestreamOpPrefix) {
	// AWS SDK reference:
	// https://github.com/aws/aws-sdk-go/blob/main/service/timestreamwrite/api.go
	case "CreateDatabase", "CreateTable",
		"DeleteDatabase", "DeleteTable",
		"DescribeDatabase", "DescribeTable",
		"ListDatabases", "ListTables",
		"UpdateDatabase", "UpdateTable",
		"WriteRecords":
		return true
	// AWS SDK reference:
	// https://github.com/aws/aws-sdk-go/blob/main/service/timestreamquery/api.go
	case "CancelQuery", "CreateScheduledQuery", "DeleteScheduledQuery",
		"DescribeEndpoints", "DescribeScheduledQuery", "ExecuteScheduledQuery",
		"ListScheduledQueries", "PrepareQuery", "Query", "UpdateScheduledQuery":
		return false
	// Note that both timestream-query and timestream-write support these tag operations.
	// For now, we assume they are used for timestream-query.
	case "ListTagsForResource", "TagResource", "UntagResource":
		return false
	default:
		logrus.Warnf("Unknown Timestream operation %q. Assuming the request is a Timestream Query request.", r.Header.Get("X-Amz-Target"))
		return false
	}
}

func getTimestreamWriteEndpoint(ctx context.Context, session client.ConfigProvider) (string, error) {
	output, err := timestreamwrite.New(session).DescribeEndpointsWithContext(ctx, nil)
	if err != nil {
		return "", trace.Wrap(err)
	}
	if output == nil || len(output.Endpoints) == 0 {
		return "", trace.NotFound("no timestream-write endpoints found")
	}
	return aws.StringValue(output.Endpoints[0].Address), nil
}
func getTimestreamQueryEndpoint(ctx context.Context, session client.ConfigProvider) (string, error) {
	output, err := timestreamquery.New(session).DescribeEndpointsWithContext(ctx, nil)
	if err != nil {
		return "", trace.Wrap(err)
	}
	if output == nil || len(output.Endpoints) == 0 {
		return "", trace.NotFound("no timestream-query endpoints found")
	}
	return aws.StringValue(output.Endpoints[0].Address), nil
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
	"awsmobilehubservice":                   mobile.EndpointsID,
	"awsproton20200720":                     proton.EndpointsID,
	"awsssooidc":                            ssooidc.EndpointsID,
	"awsssoportal":                          sso.EndpointsID,
	"detective":                             detective.EndpointsID,
	"ecr":                                   ecr.EndpointsID,
	"ecr-public":                            ecrpublic.EndpointsID,
	"elastic-inference":                     elasticinference.EndpointsID,
	"iot-jobs-data":                         iotjobsdataplane.EndpointsID,
	"iot1click":                             iot1clickdevicesservice.EndpointsID,
	"iotdata":                               iotdataplane.EndpointsID,
	"iotdeviceadvisor":                      iotdeviceadvisor.EndpointsID,
	"ioteventsdata":                         ioteventsdata.EndpointsID,
	"iotfleethub":                           iotfleethub.EndpointsID,
	"iotsecuredtunneling":                   iotsecuretunneling.EndpointsID,
	"iotwireless":                           iotwireless.EndpointsID,
	"lex":                                   lexmodelsv2.EndpointsID,
	"mediatailor":                           mediatailor.EndpointsID,
	"memorydb":                              memorydb.EndpointsID,
	"mobiletargeting":                       pinpoint.EndpointsID,
	"pricing":                               pricing.EndpointsID,
	"sagemaker":                             sagemaker.EndpointsID,
	"ses":                                   ses.EndpointsID,
	"sms-voice":                             pinpointsmsvoice.EndpointsID,
	"timestream":                            timestreamquery.EndpointsID,
}

// timestreamOpPrefix is the prefix used for all timestream-write and
// timestream-query operations in the header "X-Amz-Target".
const timestreamOpPrefix = "Timestream_20181101."
