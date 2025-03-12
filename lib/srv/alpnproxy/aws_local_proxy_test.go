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
	"net/http/httptest"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	ststypes "github.com/aws/aws-sdk-go-v2/service/sts/types"
	"github.com/stretchr/testify/require"

	awsutils "github.com/gravitational/teleport/lib/utils/aws"
)

func TestAWSAccessMiddleware(t *testing.T) {
	t.Parallel()

	assumedRoleARN := "arn:aws:sts::123456789012:assumed-role/role-name/role-session-name"
	localCred := aws.Credentials{AccessKeyID: "local-proxy", SecretAccessKey: "local-proxy-secret"}
	assumedRoleCred := aws.Credentials{AccessKeyID: "assumed-role", SecretAccessKey: "assumed-role-secret", SessionToken: "assumed-role-token"}

	m := &AWSAccessMiddleware{
		AWSCredentialsProvider: credentials.NewStaticCredentialsProvider("local-proxy", "local-proxy-secret", ""),
	}
	require.NoError(t, m.CheckAndSetDefaults())

	stsRequestByLocalProxyCred := httptest.NewRequest(http.MethodPost, "http://sts.us-east-2.amazonaws.com", nil)
	awsutils.NewSignerV2("sts").SignHTTP(t.Context(), localCred, stsRequestByLocalProxyCred, awsutils.EmptyPayloadHash, "sts", "us-west-1", time.Now())

	requestByAssumedRole := httptest.NewRequest(http.MethodGet, "http://s3.amazonaws.com", nil)
	awsutils.NewSignerV2("s3").SignHTTP(t.Context(), assumedRoleCred, requestByAssumedRole, awsutils.EmptyPayloadHash, "s3", "us-west-1", time.Now())

	t.Run("request no authorization", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		require.True(t, m.HandleRequest(recorder, httptest.NewRequest("", "http://localhost", nil)))
		require.Equal(t, http.StatusForbidden, recorder.Code)
	})

	t.Run("request signed by unknown credentials", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		require.True(t, m.HandleRequest(recorder, requestByAssumedRole))
		require.Equal(t, http.StatusForbidden, recorder.Code)
	})

	t.Run("request signed by local proxy credentials", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		require.False(t, m.HandleRequest(recorder, stsRequestByLocalProxyCred))
		require.Equal(t, http.StatusOK, recorder.Code)
	})

	// Verifies sts:AssumeRole output can be handled successfully. The
	// credentials should be saved afterwards.
	t.Run("handle sts:AssumeRole response", func(t *testing.T) {
		response := assumeRoleResponse(t, assumedRoleARN, assumedRoleCred)
		response.Request = stsRequestByLocalProxyCred
		defer response.Body.Close()
		require.NoError(t, m.HandleResponse(response))
	})

	// This is the same request as the "unknown credentials" test above. But at
	// this point, the assumed role credentials should have been saved by the
	// middleware so the request can be handled successfully now.
	t.Run("request signed by assumed role", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		require.False(t, m.HandleRequest(recorder, requestByAssumedRole))
		require.Equal(t, http.StatusOK, recorder.Code)
	})

	// Verifies non sts:AssumeRole responses do not give errors.
	t.Run("handle sts:GetCallerIdentity response", func(t *testing.T) {
		response := getCallerIdentityResponse(t, assumedRoleARN)
		response.Request = stsRequestByLocalProxyCred
		defer response.Body.Close()
		require.NoError(t, m.HandleResponse(response))
	})
}

// IdentityResult represents the identitiy result of an AWS response.
type IdentityResult struct {
	ARN string `xml:"Arn"`
}

// ResponseMetadata contains the metadata of a AWS response.
type ResponseMetadata struct {
	RequestID  string `xml:"RequestID"`
	StatusCode int    `xml:"StatusCode"`
}

// AssumeRoleResult contains the assume role result.
type AssumeRoleResult struct {
	// AssumedRoleUser is the assumed user.
	AssumedRoleUser IdentityResult `xml:"AssumedRoleUser"`
	// Credentials is the generated credentials.
	Credentials ststypes.Credentials `xml:"Credentials"`
}

// AssumeRoleResponse is the response of assume role.
type AssumeRoleResponse struct {
	// AssumeRoleResult is the resulting response from assume role.
	AssumeRoleResult AssumeRoleResult `xml:"AssumeRoleResult"`
	// Response is the response metadata.
	Response ResponseMetadata `xml:"ResponseMetadata"`
}

// GetCallerIdentityResponse is the response of get caller identity call.
type GetCallerIdentityResponse struct {
	// AssumeRoleResult is the resulting response from assume role.
	GetCallerIdentityResult IdentityResult `xml:"GetCallerIdentityResult"`
	// Response is the response metadata.
	Response ResponseMetadata `xml:"ResponseMetadata"`
}

func assumeRoleResponse(t *testing.T, roleARN string, creds aws.Credentials) *http.Response {
	t.Helper()

	body, err := awsutils.MarshalXML("AssumeRoleResponse", "https://sts.amazonaws.com/doc/2011-06-15/", AssumeRoleResponse{
		AssumeRoleResult: AssumeRoleResult{
			AssumedRoleUser: IdentityResult{
				ARN: roleARN,
			},
			Credentials: ststypes.Credentials{
				AccessKeyId:     aws.String(creds.AccessKeyID),
				SecretAccessKey: aws.String(creds.SecretAccessKey),
				SessionToken:    aws.String(creds.SessionToken),
			},
		},
		Response: ResponseMetadata{
			StatusCode: http.StatusOK,
			RequestID:  "22222222-3333-3333-3333-333333333333",
		},
	})
	require.NoError(t, err)
	return fakeHTTPResponse(http.StatusOK, body)
}

func getCallerIdentityResponse(t *testing.T, roleARN string) *http.Response {
	t.Helper()

	body, err := awsutils.MarshalXML("GetCallerIdentityResponse", "https://sts.amazonaws.com/doc/2011-06-15/", GetCallerIdentityResponse{
		GetCallerIdentityResult: IdentityResult{
			ARN: roleARN,
		},
		Response: ResponseMetadata{
			StatusCode: http.StatusOK,
			RequestID:  "22222222-3333-3333-3333-333333333333",
		},
	})
	require.NoError(t, err)
	return fakeHTTPResponse(http.StatusOK, body)
}

func fakeHTTPResponse(code int, body []byte) *http.Response {
	recorder := httptest.NewRecorder()
	recorder.Write(body)
	recorder.WriteHeader(code)
	return recorder.Result()
}
