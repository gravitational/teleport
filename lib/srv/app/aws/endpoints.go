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

	"github.com/aws/aws-sdk-go/aws/endpoints"
	"github.com/aws/aws-sdk-go/service/ecr"

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

	endpointsID := endpointsIDFromServiceName(awsAuthHeader.Service)
	resolvedEndpoint, err := endpoints.DefaultResolver().EndpointFor(endpointsID, awsAuthHeader.Region)

	// Signing name can be derived from the endpoint ID which may not be the
	// correct signing name. In that case, set the signing name back to what is
	// received from the header.
	if resolvedEndpoint.SigningNameDerived && resolvedEndpoint.SigningName != awsAuthHeader.Service {
		resolvedEndpoint.SigningName = awsAuthHeader.Service
	}
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &resolvedEndpoint, nil
}

// endpointsIDFromServiceName returns the endpoint ID used for endpoint lookups.
func endpointsIDFromServiceName(service string) string {
	if endpointsID, ok := endpointsIDMapping[service]; ok {
		return endpointsID
	}
	return service
}

// endpointsIDMapping is a mapping of services' signing names to their
// endpoints IDs, for services using different signing names from their
// endpoints IDs.
//
// The following awk script is run for each service.go in aws-sdk-go repo
// to find the mismatches.
//
//	BEGIN { EndpointsID = ""; SigningName = ""; }
//	/ServiceName =/ { ServiceName=$3; }
//	/EndpointsID =/ { EndpointsID=($3 == "ServiceName") ? ServiceName : $3; }
//	/SigningName =/ { SigningName=($3 == "EndpointsID") ? EndpointsID : $3; }
//	END {
//	    if (EndpointsID != SigningName)
//	       print FILENAME ": EndpointsID (" EndpointsID ") vs SigningName is (" SigningName ")"
//	}
//
// A "services" map with endpoints ID as the keys can be found at:
// https://github.com/aws/aws-sdk-go/blob/v1.43.2/aws/endpoints/defaults.go
//
// The results of the awk runs are validated against the service map before
// getting added to the mapping below.
//
// For aws-sdk-go@v1.43.2, 58 of 300 services are translated below.
var endpointsIDMapping = map[string]string{
	ecr.ServiceName: ecr.EndpointsID,
	/* TODO
	service/iotdataplane/service.go: EndpointsID ("data.iot") vs SigningName is ("iotdata")
	service/iotwireless/service.go: EndpointsID ("api.iotwireless") vs SigningName is ("iotwireless")
	service/timestreamwrite/service.go: EndpointsID ("ingest.timestream") vs SigningName is ("timestream")
	service/iotdeviceadvisor/service.go: EndpointsID ("api.iotdeviceadvisor") vs SigningName is ("iotdeviceadvisor")
	service/lexmodelbuildingservice/service.go: EndpointsID ("models.lex") vs SigningName is ("lex")
	service/pinpoint/service.go: EndpointsID ("pinpoint") vs SigningName is ("mobiletargeting")
	service/mediastoredata/service.go: EndpointsID ("data.mediastore") vs SigningName is ("mediastore")
	service/sagemakerruntime/service.go: EndpointsID ("runtime.sagemaker") vs SigningName is ("sagemaker")
	service/appregistry/service.go: EndpointsID ("servicecatalog-appregistry") vs SigningName is ("servicecatalog")
	service/elasticinference/service.go: EndpointsID ("api.elastic-inference") vs SigningName is ("elastic-inference")
	service/pinpointsmsvoice/service.go: EndpointsID ("sms-voice.pinpoint") vs SigningName is ("sms-voice")
	service/marketplaceentitlementservice/service.go: EndpointsID ("entitlement.marketplace") vs SigningName is ("aws-marketplace")
	service/sesv2/service.go: EndpointsID ("email") vs SigningName is ("ses")
	service/iotfleethub/service.go: EndpointsID ("api.fleethub.iot") vs SigningName is ("iotfleethub")
	service/pinpointemail/service.go: EndpointsID ("email") vs SigningName is ("ses")
	service/ecrpublic/service.go: EndpointsID ("api.ecr-public") vs SigningName is ("ecr-public")
	service/memorydb/service.go: EndpointsID ("memory-db") vs SigningName is ("memorydb")
	service/s3control/service.go: EndpointsID ("s3-control") vs SigningName is ("s3")
	service/iotsecuretunneling/service.go: EndpointsID ("api.tunneling.iot") vs SigningName is ("IoTSecuredTunneling")
	service/sagemakeredgemanager/service.go: EndpointsID ("edge.sagemaker") vs SigningName is ("sagemaker")
	service/cloudsearchdomain/service.go: EndpointsID ("cloudsearchdomain") vs SigningName is ("cloudsearch")
	service/ecr/service.go: EndpointsID ("api.ecr") vs SigningName is ("ecr")
	service/qldbsession/service.go: EndpointsID ("session.qldb") vs SigningName is ("qldb")
	service/chimesdkmeetings/service.go: EndpointsID ("meetings-chime") vs SigningName is ("chime")
	service/ioteventsdata/service.go: EndpointsID ("data.iotevents") vs SigningName is ("ioteventsdata")
	service/connectcontactlens/service.go: EndpointsID ("contact-lens") vs SigningName is ("connect")
	service/iot1clickdevicesservice/service.go: EndpointsID ("devices.iot1click") vs SigningName is ("iot1click")
	service/iot/service.go: EndpointsID ("iot") vs SigningName is ("execute-api")
	service/forecastqueryservice/service.go: EndpointsID ("forecastquery") vs SigningName is ("forecast")
	service/chimesdkmessaging/service.go: EndpointsID ("messaging-chime") vs SigningName is ("chime")
	service/mediatailor/service.go: EndpointsID ("api.mediatailor") vs SigningName is ("mediatailor")
	service/lexruntimeservice/service.go: EndpointsID ("runtime.lex") vs SigningName is ("lex")
	service/sagemakerfeaturestoreruntime/service.go: EndpointsID ("featurestore-runtime.sagemaker") vs SigningName is ("sagemaker")
	service/chimesdkidentity/service.go: EndpointsID ("identity-chime") vs SigningName is ("chime")
	service/connectparticipant/service.go: EndpointsID ("participant.connect") vs SigningName is ("execute-api")
	service/sso/service.go: EndpointsID ("portal.sso") vs SigningName is ("awsssoportal")
	service/migrationhubconfig/service.go: EndpointsID ("migrationhub-config") vs SigningName is ("mgh")
	service/mobile/service.go: EndpointsID ("mobile") vs SigningName is ("AWSMobileHubService")
	service/cloudhsmv2/service.go: EndpointsID ("cloudhsmv2") vs SigningName is ("cloudhsm")
	service/iot1clickprojects/service.go: EndpointsID ("projects.iot1click") vs SigningName is ("iot1click")
	service/detective/service.go: EndpointsID ("api.detective") vs SigningName is ("detective")
	service/ssooidc/service.go: EndpointsID ("oidc") vs SigningName is ("awsssooidc")
	service/lexmodelsv2/service.go: EndpointsID ("models-v2-lex") vs SigningName is ("lex")
	service/lexruntimev2/service.go: EndpointsID ("runtime-v2-lex") vs SigningName is ("lex")
	service/dynamodbstreams/service.go: EndpointsID ("streams.dynamodb") vs SigningName is ("dynamodb")
	service/ses/service.go: EndpointsID ("email") vs SigningName is ("ses")
	service/iotjobsdataplane/service.go: EndpointsID ("data.jobs.iot") vs SigningName is ("iot-jobs-data")
	service/appconfigdata/service.go: EndpointsID ("appconfigdata") vs SigningName is ("appconfig")
	service/augmentedairuntime/service.go: EndpointsID ("a2i-runtime.sagemaker") vs SigningName is ("sagemaker")
	service/timestreamquery/service.go: EndpointsID ("query.timestream") vs SigningName is ("timestream")
	service/marketplacemetering/service.go: EndpointsID ("metering.marketplace") vs SigningName is ("aws-marketplace")
	service/appstream/service.go: EndpointsID ("appstream2") vs SigningName is ("appstream")
	service/pricing/service.go: EndpointsID ("api.pricing") vs SigningName is ("pricing")
	service/transcribestreamingservice/service.go: EndpointsID ("transcribestreaming") vs SigningName is ("transcribe")
	service/personalizeruntime/service.go: EndpointsID ("personalize-runtime") vs SigningName is ("personalize")
	service/personalizeevents/service.go: EndpointsID ("personalize-events") vs SigningName is ("personalize")
	service/sagemaker/service.go: EndpointsID ("api.sagemaker") vs SigningName is ("sagemaker")
	service/marketplacecatalog/service.go: EndpointsID ("catalog.marketplace") vs SigningName is ("aws-marketplace")
	*/
}
