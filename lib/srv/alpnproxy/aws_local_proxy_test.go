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
	"context"
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	ststypes "github.com/aws/aws-sdk-go-v2/service/sts/types"
	"github.com/stretchr/testify/require"

	awsutils "github.com/gravitational/teleport/lib/utils/aws"
)

func TestAWSAccessMiddleware(t *testing.T) {
	t.Parallel()

	assumedRoleARN := "arn:aws:sts::123456789012:assumed-role/role-name/role-session-name"
	localProxyCred := credentials.NewStaticCredentialsProvider("local-proxy", "local-proxy-secret", "")
	assumedRoleCred := credentials.NewStaticCredentialsProvider("assumed-role", "assumed-role-secret", "assumed-role-token")

	m := &AWSAccessMiddleware{
		AWSCredentialsProvider: credentials.NewStaticCredentialsProvider("local-proxy", "local-proxy-secret", ""),
	}
	require.NoError(t, m.CheckAndSetDefaults())

	stsRequestByLocalProxyCred := httptest.NewRequest(http.MethodPost, "http://sts.us-east-2.amazonaws.com", nil)

	awsutils.NewSignerV2(localProxyCred, "sts").Sign(stsRequestByLocalProxyCred, nil, "sts", "us-west-1", time.Now())

	requestByAssumedRole := httptest.NewRequest(http.MethodGet, "http://s3.amazonaws.com", nil)
	awsutils.NewSignerV2(assumedRoleCred, "s3").Sign(requestByAssumedRole, nil, "s3", "us-west-1", time.Now())

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

func assumeRoleResponse(t *testing.T, roleARN string, provider aws.CredentialsProvider) *http.Response {
	t.Helper()

	credValue, err := provider.Retrieve(context.Background())
	require.NoError(t, err)

	body, err := awsutils.MarshalXML(
		xml.Name{
			Local: "AssumeRoleResponse",
			Space: "https://sts.amazonaws.com/doc/2011-06-15/",
		},
		map[string]any{
			"AssumeRoleResult": sts.AssumeRoleOutput{
				AssumedRoleUser: &ststypes.AssumedRoleUser{
					Arn: aws.String(roleARN),
				},
				Credentials: &ststypes.Credentials{
					AccessKeyId:     aws.String(credValue.AccessKeyID),
					SecretAccessKey: aws.String(credValue.SecretAccessKey),
					SessionToken:    aws.String(credValue.SessionToken),
				},
			},
			"ResponseMetadata": map[string]any{
				"StatusCode": http.StatusOK,
				"RequestID":  "22222222-3333-3333-3333-333333333333",
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
			"ResponseMetadata": map[string]any{
				"StatusCode": http.StatusOK,
				"RequestID":  "22222222-3333-3333-3333-333333333333",
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
