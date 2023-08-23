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

package aws

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/google/uuid"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
)

type fakeCredentialsGetter struct {
}

func (f *fakeCredentialsGetter) Get(ctx context.Context, request GetCredentialsRequest) (*credentials.Credentials, error) {
	return credentials.NewStaticCredentials(
		fmt.Sprintf("%s-%s-%s", request.SessionName, request.RoleARN, request.ExternalID),
		uuid.NewString(),
		uuid.NewString(),
	), nil
}

func TestCachedCredentialsGetter(t *testing.T) {
	t.Parallel()

	hostSession := session.Must(session.NewSession(&aws.Config{
		Credentials: credentials.AnonymousCredentials,
		Region:      aws.String("us-west-2"),
	}))
	fakeClock := clockwork.NewFakeClock()

	getter, err := NewCachedCredentialsGetter(CachedCredentialsGetterConfig{
		Getter:   &fakeCredentialsGetter{},
		CacheTTL: time.Minute,
		Clock:    fakeClock,
	})
	require.NoError(t, err)

	cred1, err := getter.Get(context.Background(), GetCredentialsRequest{
		Provider:    hostSession,
		Expiry:      fakeClock.Now().Add(time.Hour),
		SessionName: "test-session",
		RoleARN:     "test-role",
	})
	require.NoError(t, err)
	checkCredentialsAccessKeyID(t, cred1, "test-session-test-role-")

	tests := []struct {
		name         string
		sessionName  string
		roleARN      string
		externalID   string
		advanceClock time.Duration
		compareCred1 require.ComparisonAssertionFunc
	}{
		{
			name:         "cached",
			sessionName:  "test-session",
			roleARN:      "test-role",
			compareCred1: require.Same,
		},
		{
			name:         "different session name",
			sessionName:  "test-session-2",
			roleARN:      "test-role",
			compareCred1: require.NotSame,
		},
		{
			name:         "different role ARN",
			sessionName:  "test-session",
			roleARN:      "test-role-2",
			compareCred1: require.NotSame,
		},
		{
			name:         "different external ID",
			sessionName:  "test-session",
			roleARN:      "test-role",
			externalID:   "test-id",
			compareCred1: require.NotSame,
		},
		{
			name:         "cache expired",
			sessionName:  "test-session",
			roleARN:      "test-role",
			advanceClock: time.Hour,
			compareCred1: require.NotSame,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fakeClock.Advance(test.advanceClock)

			cred, err := getter.Get(context.Background(), GetCredentialsRequest{
				Provider:    hostSession,
				Expiry:      fakeClock.Now().Add(time.Hour),
				SessionName: test.sessionName,
				RoleARN:     test.roleARN,
				ExternalID:  test.externalID,
			})
			require.NoError(t, err)
			checkCredentialsAccessKeyID(t, cred, fmt.Sprintf("%s-%s-%s", test.sessionName, test.roleARN, test.externalID))
			test.compareCred1(t, cred1, cred)
		})
	}
}

func checkCredentialsAccessKeyID(t *testing.T, cred *credentials.Credentials, wantAccessKeyID string) {
	t.Helper()
	value, err := cred.Get()
	require.NoError(t, err)
	require.Equal(t, wantAccessKeyID, value.AccessKeyID)
}
