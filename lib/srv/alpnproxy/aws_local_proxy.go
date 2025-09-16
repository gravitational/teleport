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

package alpnproxy

import (
	"net/http"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport"
	awsapiutils "github.com/gravitational/teleport/api/utils/aws"
	appcommon "github.com/gravitational/teleport/lib/srv/app/common"
	"github.com/gravitational/teleport/lib/utils"
	awsutils "github.com/gravitational/teleport/lib/utils/aws"
)

// AWSAccessMiddleware verifies the requests to AWS proxy are properly signed.
type AWSAccessMiddleware struct {
	DefaultLocalProxyHTTPMiddleware

	// AWSCredentials are AWS Credentials used by LocalProxy for request's signature verification.
	AWSCredentials *credentials.Credentials

	Log logrus.FieldLogger

	assumedRoles utils.SyncMap[string, *sts.AssumeRoleOutput]
}

var _ LocalProxyHTTPMiddleware = &AWSAccessMiddleware{}

func (m *AWSAccessMiddleware) CheckAndSetDefaults() error {
	if m.Log == nil {
		m.Log = logrus.WithField(teleport.ComponentKey, "aws_access")
	}

	if m.AWSCredentials == nil {
		return trace.BadParameter("missing AWSCredentials")
	}

	return nil
}

// HandleRequest handles a request from the AWS client.
//
// Normally, the requests are signed with the local-proxy-generated credentials.
// We verify the signatures of these requests using the local-proxy-generated
// credentials then forward them to the proxy. The app agent will re-sign these
// requests with real credentials before sending them to AWS.
//
// When this AWS middleware receives a valid AssumeRole output (through
// HandleResponse), the middleware caches the credentials.
//
// When the middleware receives requests signed with these assumed-roles'
// credentials, in addition to verifying the signatures using the cached
// credentials, the middleware also rewrites the headers to indicate that these
// requests are signed by assumed roles. Upon receiving requests by assumed
// roles, the app agent restore the headers without re-signing before sending
// them to AWS.
//
// Here's a sample sequence for request by assumed role:
//
// client                   tsh                teleport                 AWS
// |                         |                    |                       |
// │ sts:AssumeRole          │                    │                       │
// ├────────────────────────►│ forward            │                       │
// │                         ├───────────────────►│ re-sign               │
// │                         │                    ├──────────────────────►│
// │                         │                    │ sts:AssumeRole output │
// │                         │                    │◄──────────────────────┤
// │                         │◄───────────────────┤                       │
// │                         │                    │                       │
// │                         ├────┐ cache         │                       │
// │                         │    │ sts:AssumeRole│                       │
// │ sts:AssuemRole output   │◄───┘ output        │                       │
// │◄────────────────────────┤                    │                       │
// │                         │                    │                       │
// │                         │                    │                       │
// │                         │                    │                       │
// │ request by assumed role │                    │                       │
// ├────────────────────────►│ rewrite headers    │                       │
// │                         ├───────────────────►│ restore headers       │
// │                         │                    ├──────────────────────►│
// │                         │                    │                       │
// │                         │                    │◄──────────────────────┤
// │                         │◄───────────────────┤                       │
// │◄────────────────────────┤                    │                       │
//
// Note that the first sts:AssumeRole should be signed with the
// local-proxy-generated credentials by the AWS client, while the second
// request is signed with real credentials of the assumed role.
func (m *AWSAccessMiddleware) HandleRequest(rw http.ResponseWriter, req *http.Request) bool {
	sigV4, err := awsutils.ParseSigV4(req.Header.Get(awsutils.AuthorizationHeader))
	if err != nil {
		m.Log.WithError(err).Error("Failed to parse AWS request authorization header.")
		rw.WriteHeader(http.StatusForbidden)
		return true
	}

	// Handle requests signed with real credentials of assumed roles by the AWS
	// client. These credentials were captured in previous HandleResponse.
	//
	// Note that currently this is only supported in HTTPS proxy mode where the
	// Host is a valid AWS endpoint.
	if awsapiutils.IsAWSEndpoint(req.Host) {
		if assumedRole, found := m.assumedRoles.Load(sigV4.KeyID); found {
			return m.handleRequestByAssumedRole(rw, req, assumedRole)
		}
	}

	// Handle requests signed with the default local proxy credentials.
	return m.handleCommonRequest(rw, req)
}

func (m *AWSAccessMiddleware) handleCommonRequest(rw http.ResponseWriter, req *http.Request) bool {
	if err := awsutils.VerifyAWSSignature(req, m.AWSCredentials); err != nil {
		m.Log.WithError(err).Error("AWS signature verification failed.")
		rw.WriteHeader(http.StatusForbidden)
		return true
	}
	return false
}

func (m *AWSAccessMiddleware) handleRequestByAssumedRole(rw http.ResponseWriter, req *http.Request, assumedRole *sts.AssumeRoleOutput) bool {
	credentials := credentials.NewStaticCredentials(
		aws.StringValue(assumedRole.Credentials.AccessKeyId),
		aws.StringValue(assumedRole.Credentials.SecretAccessKey),
		aws.StringValue(assumedRole.Credentials.SessionToken),
	)

	if err := awsutils.VerifyAWSSignature(req, credentials); err != nil {
		m.Log.WithError(err).Error("AWS signature verification failed.")
		rw.WriteHeader(http.StatusForbidden)
		return true
	}

	m.Log.Debugf("Rewriting headers for AWS request by assumed role %q.", aws.StringValue(assumedRole.AssumedRoleUser.Arn))

	// Add a custom header for marking the special request.
	req.Header.Add(appcommon.TeleportAWSAssumedRole, aws.StringValue(assumedRole.AssumedRoleUser.Arn))

	// Rename the original authorization header to ensure older app agents
	// (that don't support the requests by assumed roles) will fail.
	utils.RenameHeader(req.Header, awsutils.AuthorizationHeader, appcommon.TeleportAWSAssumedRoleAuthorization)
	return false
}

func (m *AWSAccessMiddleware) HandleResponse(response *http.Response) error {
	if response == nil || response.Request == nil {
		return nil
	}

	authHeader := utils.GetAnyHeader(response.Request.Header,
		awsutils.AuthorizationHeader,
		appcommon.TeleportAWSAssumedRoleAuthorization,
	)

	sigV4, err := awsutils.ParseSigV4(authHeader)
	if err != nil {
		m.Log.WithError(err).Error("Failed to parse AWS request authorization header.")
		return nil
	}

	if strings.EqualFold(sigV4.Service, sts.EndpointsID) {
		return trace.Wrap(m.handleSTSResponse(response))
	}
	return nil
}

func (m *AWSAccessMiddleware) handleSTSResponse(response *http.Response) error {
	// Only looking for successful sts:AssumeRole calls.
	if response.Request.Method != http.MethodPost ||
		response.StatusCode != http.StatusOK {
		return nil
	}

	// In case something goes wrong when draining the body, return an error.
	body, err := utils.GetAndReplaceResponseBody(response)
	if err != nil {
		return trace.Wrap(err)
	}

	// Save the credentials if valid AssumeRoleResponse is found.
	assumedRole, err := unmarshalAssumeRoleResponse(body)
	if err != nil {
		if !trace.IsNotFound(err) {
			m.Log.Warnf("Failed to unmarshal AssumeRoleResponse: %v.", err)
		}
		return nil
	}

	m.assumedRoles.Store(aws.StringValue(assumedRole.Credentials.AccessKeyId), assumedRole)
	m.Log.Debugf("Saved credentials for assumed role %q.", aws.StringValue(assumedRole.AssumedRoleUser.Arn))
	return nil
}

func unmarshalAssumeRoleResponse(body []byte) (*sts.AssumeRoleOutput, error) {
	if !awsutils.IsXMLOfLocalName(body, "AssumeRoleResponse") {
		return nil, trace.NotFound("not AssumeRoleResponse")
	}

	var assumedRole sts.AssumeRoleOutput
	if err := awsutils.UnmarshalXMLChildNode(&assumedRole, body, "AssumeRoleResult"); err != nil {
		return nil, trace.Wrap(err)
	}
	if assumedRole.AssumedRoleUser == nil {
		return nil, trace.BadParameter("missing AssumedRoleUser in AssumeRoleResponse %v", string(body))
	}
	if assumedRole.Credentials == nil {
		return nil, trace.BadParameter("missing Credentials in AssumeRoleResponse %v", string(body))
	}
	return &assumedRole, nil
}
