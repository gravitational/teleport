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

	// EndpointFor expects an endpoints ID which can be different from the
	// signing name.
	endpointsID := endpointsIDFromSigningName(awsAuthHeader.Service)

	// Allow ResolveUnknownService to resolve services not in the defaults mapping.
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

// endpointsIDFromSigningName returns the endpoints ID used for endpoint
// lookups by calling endpoints.DefaultResolver().EndpointFor.
//
// A "services" map with supported endpoints IDs as the keys can be found at:
// https://github.com/aws/aws-sdk-go/blob/v1.43.2/aws/endpoints/defaults.go
func endpointsIDFromSigningName(signingName string) string {
	if endpointsID, ok := endpointsIDMapping[strings.ToLower(signingName)]; ok {
		return endpointsID
	}

	// If not found in mapping, endpoints ID is expected to be the same as the
	// signing name.
	return signingName
}

// endpointsIDMapping is a mapping of services' signing names to their
// endpoints IDs.
var endpointsIDMapping = map[string]string{
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

// TODO(greedy52) Many services may sign with same names but use different
// hostnames. Will need a way to differentiate them. For now, either make the
// best guess in the mapping above or use the default signing names. See
// signingNameToHostnames in endpoints_test.go for conflicting services.
