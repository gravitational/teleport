/*
Copyright 2023 Gravitational, Inc.

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

package alpnproxy

import (
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	v4 "github.com/aws/aws-sdk-go/aws/signer/v4"
	"github.com/aws/aws-sdk-go/private/protocol"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/stretchr/testify/require"

	awsutils "github.com/gravitational/teleport/lib/utils/aws"
)

func TestAWSAccessMiddleware(t *testing.T) {
	t.Parallel()

	assumedRoleARN := "arn:aws:sts::123456789012:assumed-role/role-name/role-session-name"
	localProxyCred := credentials.NewStaticCredentials("local-proxy", "local-proxy-secret", "")
	assumedRoleCred := credentials.NewStaticCredentials("assumed-role", "assumed-role-secret", "assumed-role-token")

	stsRequestByLocalProxyCred := httptest.NewRequest(http.MethodPost, "http://sts.us-east-2.amazonaws.com", nil)
	v4.NewSigner(localProxyCred).Sign(stsRequestByLocalProxyCred, nil, "sts", "us-west-1", time.Now())

	requestByAssumedRole := httptest.NewRequest(http.MethodGet, "http://s3.amazonaws.com", nil)
	v4.NewSigner(assumedRoleCred).Sign(requestByAssumedRole, nil, "s3", "us-west-1", time.Now())

	m := &AWSAccessMiddleware{
		AWSCredentials: localProxyCred,
	}
	require.NoError(t, m.CheckAndSetDefaults())

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

func assumeRoleResponse(t *testing.T, roleARN string, cred *credentials.Credentials) *http.Response {
	t.Helper()

	credValue, err := cred.Get()
	require.NoError(t, err)

	body, err := awsutils.MarshalXML(
		xml.Name{
			Local: "AssumeRoleResponse",
			Space: "https://sts.amazonaws.com/doc/2011-06-15/",
		},
		map[string]any{
			"AssumeRoleResult": sts.AssumeRoleOutput{
				AssumedRoleUser: &sts.AssumedRoleUser{
					Arn: aws.String(roleARN),
				},
				Credentials: &sts.Credentials{
					AccessKeyId:     aws.String(credValue.AccessKeyID),
					SecretAccessKey: aws.String(credValue.SecretAccessKey),
					SessionToken:    aws.String(credValue.SessionToken),
				},
			},
			"ResponseMetadata": protocol.ResponseMetadata{
				StatusCode: http.StatusOK,
				RequestID:  "22222222-3333-3333-3333-333333333333",
			},
		},
	)
	require.NoError(t, err)
	return fakeHTTPResponse(http.StatusOK, body)
}

func getCallerIdentityResponse(t *testing.T, roleARN string) *http.Response {
	t.Helper()

	body, err := awsutils.MarshalXML(
		xml.Name{
			Local: "GetCallerIdentityResponse",
			Space: "https://sts.amazonaws.com/doc/2011-06-15/",
		},
		map[string]any{
			"GetCallerIdentityResult": sts.GetCallerIdentityOutput{
				Arn: aws.String(roleARN),
			},
			"ResponseMetadata": protocol.ResponseMetadata{
				StatusCode: http.StatusOK,
				RequestID:  "22222222-3333-3333-3333-333333333333",
			},
		},
	)
	require.NoError(t, err)
	return fakeHTTPResponse(http.StatusOK, body)
}

func fakeHTTPResponse(code int, body []byte) *http.Response {
	recorder := httptest.NewRecorder()
	recorder.Write(body)
	recorder.WriteHeader(code)
	return recorder.Result()
}
