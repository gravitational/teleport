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
package app

import (
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	awssession "github.com/aws/aws-sdk-go/aws/session"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

// mockCredentialsProvider mocks aws credentials.Provider
type mockCredentialsProvider struct {
	retrieveValue credentials.Value
	retrieveError error
}

func (m mockCredentialsProvider) Retrieve() (credentials.Value, error) {
	return m.retrieveValue, m.retrieveError
}
func (m mockCredentialsProvider) IsExpired() bool {
	return false
}

func TestIsSessionUsingTemporaryCredentials(t *testing.T) {
	tests := []struct {
		name        string
		credentials *credentials.Credentials
		expectBool  bool
		expectError func(error) bool
	}{
		{
			name: "not temporary",
			credentials: credentials.NewCredentials(&mockCredentialsProvider{
				retrieveValue: credentials.Value{
					AccessKeyID:     "id",
					SecretAccessKey: "secret",
					ProviderName:    "mock",
				},
			}),
			expectBool: false,
		},
		{
			name: "is temporary with session token",
			credentials: credentials.NewCredentials(&mockCredentialsProvider{
				retrieveValue: credentials.Value{
					AccessKeyID:     "id",
					SecretAccessKey: "secret",
					SessionToken:    "token",
					ProviderName:    "mock",
				},
			}),
			expectBool: true,
		},
		{
			name:        "bad config",
			credentials: nil,
			expectError: trace.IsNotFound,
		},
		{
			name: "failed to get credentials",
			credentials: credentials.NewCredentials(&mockCredentialsProvider{
				retrieveError: trace.AccessDenied(""),
			}),
			expectError: trace.IsAccessDenied,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			c, err := NewCloud(CloudConfig{
				Session: &awssession.Session{
					Config: &aws.Config{
						Credentials: test.credentials,
					},
				},
			})
			require.NoError(t, err)

			isTemporary, err := c.(*cloud).isSessionUsingTemporaryCredentials()

			if test.expectError != nil {
				require.True(t, test.expectError(err))
			} else {
				require.Equal(t, test.expectBool, isTemporary)
			}
		})

	}
}
