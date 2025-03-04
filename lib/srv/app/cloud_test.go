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

package app

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials/ec2rolecreds"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/lib/cloud/awsconfig"
	"github.com/gravitational/teleport/lib/cloud/mocks"
	"github.com/gravitational/teleport/lib/tlsca"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

func TestIsSessionUsingTemporaryCredentials(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name        string
		credentials aws.CredentialsProvider
		expectBool  bool
		expectError func(error) bool
	}{
		{
			name: "ec2 role",
			credentials: &mockCredentialsProvider{
				retrieveValue: aws.Credentials{
					AccessKeyID:     "id",
					SecretAccessKey: "secret",
					Source:          ec2rolecreds.ProviderName,
				},
			},
			expectBool: false,
		},
		{
			name: "web identity",
			credentials: &mockCredentialsProvider{
				retrieveValue: aws.Credentials{
					AccessKeyID:     "id",
					SecretAccessKey: "secret",
					SessionToken:    "token",
					Source:          stscreds.WebIdentityProviderName,
				},
			},
			expectBool: true,
		},
		{
			name: "session token exists",
			credentials: &mockCredentialsProvider{
				retrieveValue: aws.Credentials{
					AccessKeyID:     "id",
					SecretAccessKey: "secret",
					SessionToken:    "token",
					Source:          "SharedConfigCredentials",
				},
			},
			expectBool: true,
		},
		{
			name:        "bad config",
			credentials: nil,
			expectError: trace.IsNotFound,
		},
		{
			name: "failed to get credentials",
			credentials: &mockCredentialsProvider{
				retrieveError: trace.AccessDenied(""),
			},
			expectError: trace.IsAccessDenied,
		},
	}

	for _, test := range tests {
		test := test // capture range variable
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			isTemporary, err := isSessionUsingTemporaryCredentials(ctx, aws.Config{Credentials: test.credentials})

			if test.expectError != nil {
				require.True(t, test.expectError(err))
			} else {
				require.Equal(t, test.expectBool, isTemporary)
			}
		})
	}
}

func TestCloudGetFederationDuration(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name             string
		expiresAt        time.Time
		temporarySession bool
		expectedDuration time.Duration
		expectedErrorIs  func(error) bool
	}{
		{
			name:             "max session",
			expiresAt:        now.Add(100 * time.Hour),
			temporarySession: false,
			expectedDuration: 12 * time.Hour,
		},
		{
			name:             "max temporary session",
			expiresAt:        now.Add(100 * time.Hour),
			temporarySession: true,
			expectedDuration: time.Hour,
		},
		{
			name:             "expires in 2 hours",
			expiresAt:        now.Add(2 * time.Hour),
			temporarySession: false,
			expectedDuration: 2 * time.Hour,
		},
		{
			name:            "minimum session",
			expiresAt:       now.Add(time.Minute),
			expectedErrorIs: trace.IsAccessDenied,
		},
	}

	for _, test := range tests {
		test := test // capture range variable
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			c, err := NewCloud(CloudConfig{
				AWSConfigOptions: []awsconfig.OptionsFn{
					awsconfig.WithSTSClientProvider(func(_ aws.Config) awsconfig.STSClient {
						return &mocks.STSClient{}
					}),
				},
				Clock:  clockwork.NewFakeClockAt(now),
				Logger: slog.New(logutils.DiscardHandler{}),
			})
			require.NoError(t, err)

			cloud, ok := c.(*cloud)
			require.True(t, ok)

			req := &AWSSigninRequest{
				Identity: &tlsca.Identity{
					RouteToApp: tlsca.RouteToApp{
						AWSRoleARN: "arn:aws:iam::123456789012:role/test",
					},
					Expires: test.expiresAt,
				},
				Issuer: "test",
			}

			actualDuration, err := cloud.getFederationDuration(req, test.temporarySession)
			if test.expectedErrorIs != nil {
				require.True(t, test.expectedErrorIs(err))
			} else {
				require.NoError(t, err)
				require.Equal(t, test.expectedDuration, actualDuration)
			}
		})
	}
}

func TestCheckAndSetDefaults(t *testing.T) {
	t.Run("AWS config provider is required", func(t *testing.T) {
		err := (&CloudConfig{}).CheckAndSetDefaults()
		require.True(t, trace.IsBadParameter(err))
	})
}

func TestCloudGetAWSSigninToken(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name                    string
		federationServerHandler http.HandlerFunc
		expectedToken           string
		expectedErrorIs         func(error) bool
		expectedError           bool
	}{
		{
			name: "get failed",
			federationServerHandler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusBadRequest)
			}),
			expectedErrorIs: trace.IsBadParameter,
		},
		{
			name: "bad response",
			federationServerHandler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte("not valid json"))
			}),
			expectedError: true,
		},
		{
			name: "validate URL parameters",
			federationServerHandler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				values := r.URL.Query()
				require.Equal(t, "getSigninToken", values.Get("Action"))
				require.Equal(t, `{"sessionId":"FAKEACCESSKEYID","sessionKey":"secret","sessionToken":"token"}`, values.Get("Session"))
				w.Write([]byte(`{"SigninToken":"generated-token"}`))
			}),
			expectedToken: "generated-token",
		},
		{
			name: "validate URL parameters temporary session",
			federationServerHandler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				values := r.URL.Query()
				require.Equal(t, "getSigninToken", values.Get("Action"))
				require.Equal(t, `{"sessionId":"FAKEACCESSKEYID","sessionKey":"secret","sessionToken":"token"}`, values.Get("Session"))
				require.Equal(t, "", values.Get("SessionDuration"))
				w.Write([]byte(`{"SigninToken":"generated-token"}`))
			}),
			expectedToken: "generated-token",
		},
	}

	for _, test := range tests {
		test := test // capture range variable
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			mockFederationServer := httptest.NewServer(test.federationServerHandler)
			t.Cleanup(mockFederationServer.Close)

			c, err := NewCloud(CloudConfig{
				AWSConfigOptions: []awsconfig.OptionsFn{
					awsconfig.WithSTSClientProvider(func(_ aws.Config) awsconfig.STSClient {
						return &mocks.STSClient{}
					}),
				},
				Logger: slog.New(logutils.DiscardHandler{}),
			})
			require.NoError(t, err)

			cloud, ok := c.(*cloud)
			require.True(t, ok)

			req := &AWSSigninRequest{
				Identity: &tlsca.Identity{
					RouteToApp: tlsca.RouteToApp{
						AWSRoleARN: "arn:aws:iam::123456789012:role/test",
					},
					Expires: time.Now().Add(24 * time.Hour),
				},
				Issuer: "test",
			}

			actualToken, err := cloud.getAWSSigninToken(ctx, req, mockFederationServer.URL)
			if test.expectedErrorIs != nil {
				require.True(t, test.expectedErrorIs(err))
			} else if test.expectedError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, test.expectedToken, actualToken)
			}
		})
	}
}

func TestCloudGetFederationURL(t *testing.T) {
	tests := []struct {
		name                  string
		inputTargetURL        string
		expectedFederationURL string
	}{
		{
			name:                  "AWS GovCloud (US)",
			inputTargetURL:        constants.AWSUSGovConsoleURL,
			expectedFederationURL: "https://signin.amazonaws-us-gov.com/federation",
		},
		{
			name:                  "AWS China",
			inputTargetURL:        constants.AWSCNConsoleURL,
			expectedFederationURL: "https://signin.amazonaws.cn/federation",
		},
		{
			name:                  "AWS Standard",
			inputTargetURL:        constants.AWSConsoleURL,
			expectedFederationURL: "https://signin.aws.amazon.com/federation",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.Equal(t, test.expectedFederationURL, getFederationURL(test.inputTargetURL))
		})
	}
}
