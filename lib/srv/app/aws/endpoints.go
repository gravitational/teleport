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
	"net/http"
	"strings"

	awsv2 "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go/aws/endpoints"
	"github.com/aws/aws-sdk-go/service/appregistry"
	"github.com/aws/aws-sdk-go/service/appstream"
	"github.com/aws/aws-sdk-go/service/detective"
	"github.com/aws/aws-sdk-go/service/ecr"
	"github.com/aws/aws-sdk-go/service/elasticinference"
	"github.com/aws/aws-sdk-go/service/iot"
	"github.com/aws/aws-sdk-go/service/iot1clickprojects"
	"github.com/aws/aws-sdk-go/service/iotfleethub"
	"github.com/aws/aws-sdk-go/service/iotjobsdataplane"
	"github.com/aws/aws-sdk-go/service/iotwireless"
	"github.com/aws/aws-sdk-go/service/lexmodelsv2"
	"github.com/aws/aws-sdk-go/service/marketplaceentitlementservice"
	"github.com/aws/aws-sdk-go/service/mediastoredata"
	"github.com/aws/aws-sdk-go/service/mediatailor"
	"github.com/aws/aws-sdk-go/service/pinpoint"
	"github.com/aws/aws-sdk-go/service/pricing"
	"github.com/aws/aws-sdk-go/service/sagemaker"
	"github.com/aws/aws-sdk-go/service/ses"
	"github.com/aws/aws-sdk-go/service/sso"
	"github.com/aws/aws-sdk-go/service/ssooidc"

	awsutils "github.com/gravitational/teleport/lib/utils/aws"

	"github.com/gravitational/trace"
)

// resolveEndpoint extracts the aws-service on and aws-region from the request authorization header
// and resolves the aws-service and aws-region to AWS endpoint.
func resolveEndpoint(r *http.Request) (*endpoints.ResolvedEndpoint, error) {
	awsAuthHeader, err := awsutils.ParseSigV4(r.Header.Get(awsutils.AuthorizationHeader))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Some clients may sign with some upper case letters (e.g.
	// "IoTSecuredTunneling") where other clients may sign with the same key
	// but in all lower cases. We use all lower cases as keys in our mappings.
	signingName := strings.ToLower(awsAuthHeader.Service)

	// EndpointFor from aws-sdk-go does not support all services. Use endpoint
	// resolvers from aws-sdk-go-v2 for those exceptions.
	endpointV2Resolver, found := endpointsV2Resolvers[signingName]
	if found {
		endpointV2, err := endpointV2Resolver(awsAuthHeader.Region)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		// SigningName can be empty using the resolver. Set it back to what is
		// received from the header.
		endpointV2.SigningName = awsAuthHeader.Service
		return endpointV2ToEndpointV1(&endpointV2), nil
	}

	// EndpointFor expects an endpoints ID which can be different from the
	// signing name.
	endpointsID := endpointsIDFromSigningName(signingName)
	resolvedEndpoint, err := endpoints.DefaultResolver().EndpointFor(endpointsID, awsAuthHeader.Region)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// SigningName can be derived from the endpoint ID which may not be the
	// correct signing name. Set it back to what is received from the header.
	resolvedEndpoint.SigningName = awsAuthHeader.Service
	return &resolvedEndpoint, nil
}

// endpointV2ToEndpointV1 converts endpoint object from aws-sdk-go-v2 to
// v1's endpoints.ResolvedEndpoint.
func endpointV2ToEndpointV1(endpointV2 *awsv2.Endpoint) *endpoints.ResolvedEndpoint {
	return &endpoints.ResolvedEndpoint{
		URL:           endpointV2.URL,
		PartitionID:   endpointV2.PartitionID,
		SigningRegion: endpointV2.SigningRegion,
		SigningName:   endpointV2.SigningName,
		SigningMethod: endpointV2.SigningMethod,
	}
}

// endpointsIDFromSigningName returns the endpoints ID used for endpoint
// lookups by calling endpoints.DefaultResolver().EndpointFor.
//
// A "services" map with supported endpoints IDs as the keys can be found at:
// https://github.com/aws/aws-sdk-go/blob/v1.43.2/aws/endpoints/defaults.go
func endpointsIDFromSigningName(signingName string) string {
	if endpointsID, ok := endpointsIDMapping[signingName]; ok {
		return endpointsID
	}

	// If not found in mapping, endpoints ID is expected to be the same as the
	// signing name.
	return signingName
}

// endpointsIDMapping is a mapping of services' signing names to their
// endpoints IDs.
var endpointsIDMapping = map[string]string{
	"appstream":         appstream.EndpointsID,
	"aws-marketplace":   marketplaceentitlementservice.EndpointsID,
	"awsssooidc":        ssooidc.EndpointsID,
	"awsssoportal":      sso.EndpointsID,
	"detective":         detective.EndpointsID,
	"ecr":               ecr.EndpointsID,
	"elastic-inference": elasticinference.EndpointsID,
	"execute-api":       iot.EndpointsID,
	"iot-jobs-data":     iotjobsdataplane.EndpointsID,
	"iot1click":         iot1clickprojects.EndpointsID,
	"iotfleethub":       iotfleethub.EndpointsID,
	"iotwireless":       iotwireless.EndpointsID,
	"lex":               lexmodelsv2.EndpointsID,
	"mediastore":        mediastoredata.EndpointsID,
	"mediatailor":       mediatailor.EndpointsID,
	"mobiletargeting":   pinpoint.EndpointsID,
	"pricing":           pricing.EndpointsID,
	"sagemaker":         sagemaker.EndpointsID,
	"servicecatalog":    appregistry.EndpointsID,
	"ses":               ses.EndpointsID,
}

// TODO Many services may sign with same names but use different hostnames.
// Will need a way to differentiate them. For now, either make the best guess
// in the mapping above or use the default signing name. Here is an incomplete
// list:
//
// "apigateway"          : apigateway, apigatewaymanagementapi, apigatewayv2
// "appconfig"           : appconfig, appconfigdata
// "aws-marketplace"     : marketplacecatalog, marketplaceentitlementservice, marketplacemetering
// "aws-marketplace"     : marketplaceentitlementservice, marketplacemetering, marketplacecatalog
// "chime"               : chime, chimesdkmeetings, chimesdkmessaging, chimesdkidentity
// "cloudhsm"            : cloudhsm, cloudhsmv2
// "cloudsearch"         : cloudsearch, cloudsearchdomain
// "elasticloadbalancing": elasticloadbalancing, elasticloadbalancingv2
// "es"                  : elasticsearchservice, opensearch
// "forecast"            : forecast, forecastquery
// "greengrass"          : greengrass, greengrassv2
// "connect"             : connect, connectcontactlens, connectparticipant
// "dynamodb"            : dynamodb, dynamodbstreams
// "forecast"            : forecast, forecastquery
// "iot1click"           : iot1clickprojects, iot1clickdevicesservice.
// "lex"                 : lexmodelbuildingservice, lexruntimeservice, lexmodelsv2, lexruntimev2.
// "migrationhub"        : migrationhub, migrationhubconfig
// "personalize"         : personalize, personalizeruntime, personalizeevents
// "qldb"                : qldb, qldbsession
// "rds"                 : rds, neptune, docdb
// "s3"                  : s3, s3control
// "sagemaker"           : sagemaker, sagemakerruntime, sagemakeredgemanager, sagemakerfeaturestoreruntime, augmentedairuntime.
// "ses"                 : ses, sesv2, pinpointemail
// "timestream"          : timestreamquery, timestreamwrite
// "transcribe"          : transcribe, transcribestreamingservice
